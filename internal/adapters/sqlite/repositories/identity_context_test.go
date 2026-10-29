package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"

	"github.com/hailam/mig33/internal/adapters/sqlite/migrations"
)

func registrationIdentityFixture() Identity {
	return Identity{Username: "registrant", Display: "Registrant", Mobile: "+9607771234", Activated: true, PasswordSalt: make([]byte, 16), PasswordKey: make([]byte, 32), PasswordIterations: 100000, LegacyUnits: 4}
}

func registrationChallengeFixture(t *testing.T, store *Store) string {
	t.Helper()
	token := strings.Repeat("d", 48)
	if err := store.CreateIdentityChallenge(t.Context(), IdentityChallenge{Token: token, Purpose: "registration", ExpiresAt: time.Now().Add(time.Minute).UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	return token
}

func TestRegistrationRollsBackIdentityRosterAndChallengeTogether(t *testing.T) {
	store := testStore(t)
	token := registrationChallengeFixture(t, store)
	if _, err := store.db.ExecContext(t.Context(), `CREATE TRIGGER fail_registration_group BEFORE INSERT ON contact_groups
WHEN NEW.owner='registrant' BEGIN SELECT RAISE(ABORT,'injected registration group failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RegisterIdentity(t.Context(), token, registrationIdentityFixture()); err == nil {
		t.Fatal("registration ignored failure of its initial Friends group")
	}
	if _, err := store.Identity(t.Context(), "registrant"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("failed roster setup left an identity: %v", err)
	}
	if _, err := store.Contacts(t.Context(), "registrant"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("failed group setup left a roster: %v", err)
	}
	if _, err := store.IdentityChallenge(t.Context(), token, "registration"); err != nil {
		t.Fatalf("failed registration consumed its challenge: %v", err)
	}
	if _, err := store.db.ExecContext(t.Context(), "DROP TRIGGER fail_registration_group"); err != nil {
		t.Fatal(err)
	}
	created, err := store.RegisterIdentity(t.Context(), token, registrationIdentityFixture())
	if err != nil || created.ID == 0 {
		t.Fatalf("retry after rollback: identity=%+v error=%v", created, err)
	}
	roster, err := store.Contacts(t.Context(), created.Username)
	if err != nil || len(roster.Contacts) != 0 || len(roster.Groups) != 1 || roster.Groups[0].ID != 1 || roster.Groups[0].Name != "Friends" {
		t.Fatalf("initial registration roster: %+v error=%v", roster, err)
	}
}

func TestRegistrationCommitSurvivesConcurrentCancellationWithRoster(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var armed atomic.Bool
	name := fmt.Sprintf("mig33-registration-commit-%d", cancellationDriverSequence.Add(1))
	sql.Register(name, &sqlite3.SQLiteDriver{ConnectHook: func(conn *sqlite3.SQLiteConn) error {
		conn.RegisterCommitHook(func() int {
			if armed.Load() {
				cancel()
			}
			return 0
		})
		return nil
	}})
	db, err := sql.Open(name, filepath.Join(t.TempDir(), "registration.sqlite")+"?_foreign_keys=on&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if err := migrations.Apply(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	store := &Store{db: db}
	token := registrationChallengeFixture(t, store)
	armed.Store(true)
	created, err := store.RegisterIdentity(ctx, token, registrationIdentityFixture())
	if err != nil || created.ID == 0 || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("committed registration: identity=%+v error=%v context=%v", created, err, ctx.Err())
	}
	if _, err := store.Identity(t.Context(), created.Username); err != nil {
		t.Fatalf("committed identity unavailable: %v", err)
	}
	roster, err := store.Contacts(t.Context(), created.Username)
	if err != nil || len(roster.Groups) != 1 || roster.Groups[0].Name != "Friends" {
		t.Fatalf("committed registration is not ready for login: %+v error=%v", roster, err)
	}
}
