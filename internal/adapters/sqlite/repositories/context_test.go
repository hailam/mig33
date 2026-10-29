package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"

	"github.com/hailam/mig33/internal/adapters/sqlite/migrations"
)

func TestOpenCanceledBeforeCreatingDatabase(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	path := filepath.Join(t.TempDir(), "uncreated", "chat.sqlite")
	store, err := Open(ctx, path)
	if store != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("Open: store=%v error=%v", store, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled open touched database directory: %v", err)
	}
}

func TestRepositoryCancellationWhileWaitingForConnection(t *testing.T) {
	operations := map[string]func(context.Context, *Store) error{
		"query": func(ctx context.Context, store *Store) error {
			_, err := store.Groups(ctx, "alice")
			return err
		},
		"transaction": func(ctx context.Context, store *Store) error {
			_, err := store.CreateGroup(ctx, "canceled", "alice", []string{"bob"})
			return err
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			store := testStore(t)
			held, err := store.db.Conn(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer held.Close()
			before := store.db.Stats().WaitCount
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			finished := make(chan error, 1)
			go func() { finished <- operation(ctx, store) }()
			waitDeadline := time.NewTimer(3 * time.Second)
			defer waitDeadline.Stop()
			tick := time.NewTicker(time.Millisecond)
			defer tick.Stop()
			for store.db.Stats().WaitCount == before {
				select {
				case err := <-finished:
					t.Fatalf("operation bypassed occupied pool: %v", err)
				case <-waitDeadline.C:
					t.Fatal("operation never waited for a connection")
				case <-tick.C:
				}
			}
			cancel()
			select {
			case err := <-finished:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("waiting operation lost cancellation: %v", err)
				}
			case <-waitDeadline.C:
				t.Fatal("canceled operation still waits for the occupied connection")
			}
			if err := held.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Group(t.Context(), "canceled", "alice"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("canceled transaction changed group state: %v", err)
			}
		})
	}
}

var cancellationDriverSequence atomic.Uint64

func TestCanceledMessageTransactionRollsBackEveryRecord(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var inserted atomic.Bool
	name := fmt.Sprintf("mig33-canceled-message-%d", cancellationDriverSequence.Add(1))
	sql.Register(name, &sqlite3.SQLiteDriver{ConnectHook: func(conn *sqlite3.SQLiteConn) error {
		return conn.RegisterFunc("cancel_message_insert", func() int {
			inserted.Store(true)
			cancel()
			return 0
		}, false)
	}})
	path := filepath.Join(t.TempDir(), "cancel.sqlite")
	db, err := sql.Open(name, path+"?_foreign_keys=on&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if err := migrations.Apply(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `CREATE TRIGGER cancel_after_message AFTER INSERT ON messages
BEGIN SELECT cancel_message_insert(); END;`); err != nil {
		t.Fatal(err)
	}
	store := &Store{db: db}
	_, err = store.SaveMessage(ctx, Message{Kind: 1, Conversation: "bob", Sender: "alice", Body: "must roll back", RequestKey: strings.Repeat("a", 48), RequestHash: strings.Repeat("b", 64)}, []string{"bob"})
	if !inserted.Load() {
		t.Fatal("fixture did not reach the message insert inside its transaction")
	}
	// The adapter preserves cancellation even if database/sql reports that
	// its automatic rollback already completed the transaction.
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled message transaction: %v", err)
	}
	for _, table := range []string{"messages", "message_audience", "message_requests"} {
		var count int
		if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("cancellation left %d rows in %s", count, table)
		}
	}
}

func TestWriterLockCancellationHonorsDriverWaitBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locked.sqlite")
	first, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	// SQLite's busy handler can delay interruption. Test a known busy timeout
	// with a generous completion bound, not a claim that deadlines preempt C.
	if _, err := second.db.ExecContext(t.Context(), "PRAGMA busy_timeout=150"); err != nil {
		t.Fatal(err)
	}
	held, err := first.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Rollback()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	finished := make(chan error, 1)
	started := time.Now()
	go func() {
		_, err := second.CreateGroup(ctx, "locked", "alice", []string{"bob"})
		finished <- err
	}()
	select {
	case err := <-finished:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("writer lock cancellation: %v", err)
		}
		var driverErr sqlite3.Error
		if !errors.As(err, &driverErr) || driverErr.Code != sqlite3.ErrBusy {
			t.Fatalf("cancellation lost SQLite lock error: %v", err)
		}
		t.Logf("writer-lock cancellation returned in %s (20ms context, 150ms SQLite busy timeout)", time.Since(started))
	case <-time.After(3 * time.Second):
		t.Fatal("writer lock cancellation exceeded the fixture's completion budget")
	}
	if err := held.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Group(t.Context(), "locked", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("timed-out write created a group: %v", err)
	}
}

func TestCommittedMessageReplaysAfterCallerCancellationAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.sqlite")
	store, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	message := Message{Kind: 1, Conversation: "bob", Sender: "alice", Body: "accepted before response loss", RequestKey: strings.Repeat("a", 48), RequestHash: strings.Repeat("b", 64)}
	accepted, err := store.SaveMessage(ctx, message, []string{"bob"})
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	// The database has accepted the command, but its caller loses the response.
	cancel()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	replayed, err := store.SaveMessage(t.Context(), message, []string{"bob"})
	if err != nil || replayed.ID != accepted.ID {
		t.Fatalf("durable replay: message=%+v error=%v, original=%+v", replayed, err, accepted)
	}
	assertIDs(t, pending(t, store, "bob"), accepted.ID)
	assertIDs(t, history(t, store, "alice", 1, "bob"), accepted.ID)
	message.RequestHash = strings.Repeat("c", 64)
	if _, err := store.SaveMessage(t.Context(), message, []string{"bob"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("same key accepted changed command: %v", err)
	}
}

func TestCancellationDuringSuccessfulCommitKeepsAcceptedMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var armed atomic.Bool
	name := fmt.Sprintf("mig33-commit-cancellation-%d", cancellationDriverSequence.Add(1))
	sql.Register(name, &sqlite3.SQLiteDriver{ConnectHook: func(conn *sqlite3.SQLiteConn) error {
		conn.RegisterCommitHook(func() int {
			if armed.Load() {
				cancel()
			}
			return 0 // Allow SQLite to complete its commit.
		})
		return nil
	}})
	db, err := sql.Open(name, filepath.Join(t.TempDir(), "commit.sqlite")+"?_foreign_keys=on&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if err := migrations.Apply(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	store := &Store{db: db}
	armed.Store(true)
	accepted, err := store.SaveMessage(ctx, Message{Kind: 1, Conversation: "bob", Sender: "alice", Body: "commit won the cancellation race"}, []string{"bob"})
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("commit hook did not cancel the caller")
	}
	if err != nil || accepted.ID == 0 {
		t.Fatalf("successful commit was changed into a cancellation failure: message=%+v error=%v", accepted, err)
	}
	assertIDs(t, pending(t, store, "bob"), accepted.ID)
	assertIDs(t, history(t, store, "alice", 1, "bob"), accepted.ID)
}
