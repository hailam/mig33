package realtime

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/community"
	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/media"
	"github.com/hailam/mig33/internal/review"
	"github.com/hailam/mig33/internal/rooms"
)

// Nil persistence ports make any accidental constructor or canceled-startup I/O
// panic. This fixture deliberately has no concrete adapter or background worker.
func dependenciesWithoutStorage() Dependencies {
	return Dependencies{
		Accounts: accounts.NewService(nil), Contacts: contacts.New(nil), Chat: chat.New(nil), Rooms: rooms.New(nil),
		Community: community.New(nil, nil, nil), Catalog: catalog.New(nil), Commerce: commerce.New(nil, nil, nil),
		Media: media.New(nil, nil), Activity: activity.New(nil, nil, nil), Review: review.New(nil, nil, nil),
		Privacy: accounts.NewPrivacy(nil, nil), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestConstructorOnlyAssemblesDependencies(t *testing.T) {
	dependencies := dependenciesWithoutStorage()
	coordinator, err := New(dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if coordinator.initialized || coordinator.running || len(coordinator.accounts) != 0 {
		t.Fatal("constructor loaded application state or started workers")
	}
	if err := coordinator.Close(); err != nil {
		t.Fatal(err)
	}
	dependencies.Logger = nil
	if _, err := New(dependencies); err == nil {
		t.Fatal("required logger accepted nil")
	}
}

func TestInitializationCancellationDoesNotTouchStorage(t *testing.T) {
	coordinator, err := New(dependenciesWithoutStorage())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { coordinator.Close() })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := coordinator.Initialize(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Initialize: %v", err)
	}
	if coordinator.initialized {
		t.Fatal("canceled startup marked application initialized")
	}
	if err := coordinator.Run(t.Context()); err == nil {
		t.Fatal("worker started before initialization")
	}
}
