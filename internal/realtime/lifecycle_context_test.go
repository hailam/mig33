package realtime

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/faults"
)

func discardedLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func initializedApplication(t *testing.T) (*Application, *repositories.Store) {
	t.Helper()
	repo, err := repositories.Open(t.Context(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewWithRepository("http://127.0.0.1:8080", repo, discardedLogger())
	if err != nil {
		repo.Close()
		t.Fatal(err)
	}
	startup, cancel := context.WithCancel(t.Context())
	if err := app.Initialize(startup); err != nil {
		cancel()
		app.Close()
		repo.Close()
		t.Fatal(err)
	}
	cancel() // A finished startup lifetime must not poison later operations.
	t.Cleanup(func() {
		app.Close()
		if err := repo.Close(); err != nil {
			t.Error(err)
		}
	})
	return app, repo
}

func waitApplication(t *testing.T, app *Application, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		app.mu.Lock()
		ready := condition()
		app.mu.Unlock()
		if ready {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("application condition did not become true")
}

func TestMaintenanceCancellationKeepsApplicationAndRepositoryOpen(t *testing.T) {
	app, repo := initializedApplication(t)
	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan error, 1)
	go func() { finished <- app.Run(ctx) }()
	waitApplication(t, app, func() bool { return app.running })
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("maintenance worker did not join")
	}
	id := login(t, app, "alice")
	if _, err := app.ChatGroups(t.Context(), id); err != nil {
		t.Fatalf("draining request cannot use application: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Identity(t.Context(), "alice"); err != nil {
		t.Fatalf("application closed caller-owned repository: %v", err)
	}
}

func TestMaintenanceExpiresVoteWithoutIncomingTraffic(t *testing.T) {
	app, repo := initializedApplication(t)
	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan error, 1)
	go func() { finished <- app.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-finished; err != nil {
			t.Error(err)
		}
	})
	waitApplication(t, app, func() bool { return app.running })
	app.mu.Lock()
	app.kickVotes["Lobby"] = &kickVote{Target: "bob", Initiator: "alice", Voters: map[string]bool{"alice": true}, Required: 3, ExpiresAt: time.Now().Add(20 * time.Millisecond)}
	app.wakeMaintenance()
	app.mu.Unlock()
	waitApplication(t, app, func() bool { return app.kickVotes["Lobby"] == nil })
	// The application remains usable after timer-owned work finishes.
	if _, err := repo.Room(t.Context(), "Lobby"); err != nil {
		t.Fatal(err)
	}
}

type contextRepository struct {
	Repository
	groups func(context.Context, string) ([]chat.Group, error)
}

func (repo *contextRepository) Groups(ctx context.Context, user string) ([]chat.Group, error) {
	return repo.groups(ctx, user)
}

func TestServicePreservesContextAndStorageCancellationCause(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		repo := &contextRepository{groups: func(received context.Context, user string) ([]chat.Group, error) {
			if received != ctx || user != "alice" {
				t.Fatalf("context/actor changed: %v %q", received, user)
			}
			return nil, cause
		}}
		app := applicationWithRepositoryPort(repo)
		_, err := app.ChatGroups(ctx, "alice-session")
		if !errors.Is(err, cause) {
			t.Fatalf("storage cancellation cause lost: %v", err)
		}
		var failure *faults.ServiceError
		if !errors.As(err, &failure) || failure.Message != "Chat storage unavailable" {
			t.Fatalf("public error changed: %v", err)
		}
	}
}

type admissionContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (ctx *admissionContext) Err() error {
	err := ctx.Context.Err()
	ctx.once.Do(func() { close(ctx.checked) })
	return err
}

func TestCancellationWhileWaitingForStateLockDoesNotReachRepository(t *testing.T) {
	app := applicationWithRepositoryPort(&contextRepository{groups: func(context.Context, string) ([]chat.Group, error) {
		t.Error("canceled operation reached storage after lock admission")
		return nil, nil
	}})
	parent, cancel := context.WithCancel(t.Context())
	defer cancel()
	ctx := &admissionContext{Context: parent, checked: make(chan struct{})}
	app.mu.Lock()
	finished := make(chan error, 1)
	go func() { _, err := app.ChatGroups(ctx, "alice-session"); finished <- err }()
	<-ctx.checked // The initial check observed a live context before the lock.
	cancel()
	app.mu.Unlock()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled waiter did not return")
	}
}
