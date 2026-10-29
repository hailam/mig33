package realtime

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/rooms"
)

// An unimplemented capability panics through the nil embedded port. These
// tests therefore fail if a use case reaches storage before authorization or
// starts depending on an unrelated repository capability.
type serviceRepositoryStub struct {
	Repository
	groups   func(string) ([]chat.Group, error)
	history  func(string, byte, string, int64, int) ([]chat.Message, error)
	read     func(string, byte, string) (int64, error)
	room     func(string) (rooms.Room, error)
	contacts func(string) (contacts.ContactRoster, error)
}

func (r *serviceRepositoryStub) Groups(ctx context.Context, owner string) ([]chat.Group, error) {
	if r.groups == nil {
		panic("unexpected repository call: Groups")
	}
	return r.groups(owner)
}

func (r *serviceRepositoryStub) History(ctx context.Context, owner string, kind byte, target string, after int64, limit int) ([]chat.Message, error) {
	if r.history == nil {
		panic("unexpected repository call: History")
	}
	return r.history(owner, kind, target, after, limit)
}

func (r *serviceRepositoryStub) ConversationRead(ctx context.Context, owner string, kind byte, target string) (int64, error) {
	if r.read == nil {
		panic("unexpected repository call: ConversationRead")
	}
	return r.read(owner, kind, target)
}

func (r *serviceRepositoryStub) Room(ctx context.Context, name string) (rooms.Room, error) {
	if r.room == nil {
		panic("unexpected repository call: Room")
	}
	return r.room(name)
}

func (r *serviceRepositoryStub) Contacts(ctx context.Context, owner string) (contacts.ContactRoster, error) {
	if r.contacts == nil {
		panic("unexpected repository call: Contacts")
	}
	return r.contacts(owner)
}

// Deliberately construct only application state: no bootstrap, driver, SQL
// fixture or concrete adapter is involved in these service contracts.
func applicationWithRepositoryPort(repo Repository) *Application {
	app, err := NewWithRepository("http://local", repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		panic(err)
	}
	now := time.Now()
	app.accounts = map[string]*account{"alice": {name: "alice"}, "bob": {name: "bob", status: "Available for a chat"}}
	app.sessions = map[string]*session{
		"alice-session": {id: "alice-session", username: "alice", authenticated: true, created: now, touched: now, presence: 1, rooms: map[string]bool{}},
		"bob-session":   {id: "bob-session", username: "bob", authenticated: true, created: now, touched: now, presence: 3},
		"pending-login": {id: "pending-login", username: "alice", created: now, touched: now},
	}
	return app
}

func TestServiceQueriesAuthorizeBeforeRepositoryAccess(t *testing.T) {
	app := applicationWithRepositoryPort(&serviceRepositoryStub{})
	operations := map[string]func(context.Context, string) error{
		"groups": func(ctx context.Context, sid string) error {
			_, err := app.ChatGroups(ctx, sid)
			return err
		},
		"history": func(ctx context.Context, sid string) error {
			_, err := app.ChatHistory(ctx, sid, chat.HistoryQuery{PageQuery: chat.PageQuery{Limit: 20}, Kind: "private", Target: "bob"})
			return err
		},
		"contacts": func(ctx context.Context, sid string) error {
			_, err := app.ContactsSnapshot(ctx, sid)
			return err
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			for _, sid := range []string{"unknown-session", "pending-login"} {
				var failure *faults.ServiceError
				if err := operation(context.Background(), sid); !errors.As(err, &failure) || failure.Kind != faults.Unauthenticated {
					t.Fatalf("session %s: expected authentication failure, got %v", sid, err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := operation(ctx, "alice-session"); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled use case reached storage or lost cancellation: %v", err)
			}
		})
	}
}

func TestChatServicesUseReplaceableRepositoryPorts(t *testing.T) {
	wantGroups := []chat.Group{{ID: "team", Creator: "bob", Members: []string{"alice", "bob"}}}
	repo := &serviceRepositoryStub{
		groups: func(owner string) ([]chat.Group, error) {
			if owner != "alice" {
				t.Fatalf("groups queried for %s instead of authenticated account", owner)
			}
			return wantGroups, nil
		},
		history: func(owner string, kind byte, target string, after int64, limit int) ([]chat.Message, error) {
			if owner != "alice" || kind != 1 || target != "bob" || after != 10 || limit != 2 {
				t.Fatalf("history scope: %s %d %s %d %d", owner, kind, target, after, limit)
			}
			return []chat.Message{{ID: 11, Sender: "bob", Body: "First"}, {ID: 12, Sender: "alice", Body: "Next"}}, nil
		},
		read: func(owner string, kind byte, target string) (int64, error) {
			if owner != "alice" || kind != 1 || target != "bob" {
				t.Fatalf("read-position scope: %s %d %s", owner, kind, target)
			}
			return 7, nil
		},
	}
	app := applicationWithRepositoryPort(repo)
	groups, err := app.ChatGroups(context.Background(), "alice-session")
	if err != nil || !reflect.DeepEqual(groups, wantGroups) {
		t.Fatalf("group service result: %+v, %v", groups, err)
	}
	page, err := app.ChatHistory(context.Background(), "alice-session", chat.HistoryQuery{
		PageQuery: chat.PageQuery{AfterID: 10, Limit: 1}, Kind: "private", Target: "bob",
	})
	if err != nil || len(page.Messages) != 1 || page.Messages[0].ID != 11 || !page.HasMore || page.NextAfterID != 11 || page.ReadThroughID != 7 {
		t.Fatalf("history service result: %+v, %v", page, err)
	}
}

func TestRoomHistoryRequiresLiveMembershipBeforeReadingMessages(t *testing.T) {
	repo := &serviceRepositoryStub{room: func(name string) (rooms.Room, error) {
		if name != "lobby" {
			t.Fatalf("unexpected room lookup %q", name)
		}
		return rooms.Room{Name: "Lobby"}, nil
	}}
	app := applicationWithRepositoryPort(repo)
	query := chat.HistoryQuery{PageQuery: chat.PageQuery{Limit: 20}, Kind: "room", Target: "lobby"}
	var failure *faults.ServiceError
	if _, err := app.ChatHistory(context.Background(), "alice-session", query); !errors.As(err, &failure) || failure.Kind != faults.Forbidden {
		t.Fatalf("non-member should not reach message history: %v", err)
	}
	// Membership belongs to the application; the repository only resolves the
	// canonical room and returns data scoped to the authorized account.
	app.sessions["alice-session"].rooms["Lobby"] = true
	repo.history = func(owner string, kind byte, target string, _ int64, _ int) ([]chat.Message, error) {
		if owner != "alice" || kind != 3 || target != "Lobby" {
			t.Fatalf("authorized room history scope: %s %d %s", owner, kind, target)
		}
		return []chat.Message{{ID: 31, Body: "Welcome"}}, nil
	}
	repo.read = func(string, byte, string) (int64, error) { return 31, nil }
	page, err := app.ChatHistory(context.Background(), "alice-session", query)
	if err != nil || len(page.Messages) != 1 || page.Messages[0].Body != "Welcome" {
		t.Fatalf("joined room result: %+v, %v", page, err)
	}
}

func TestContactsServiceCombinesRepositorySnapshotWithLivePresence(t *testing.T) {
	repo := &serviceRepositoryStub{contacts: func(owner string) (contacts.ContactRoster, error) {
		if owner != "alice" {
			t.Fatalf("contact snapshot requested for %q", owner)
		}
		return contacts.ContactRoster{
			Revision: 8, Checkpoint: 42,
			Groups: []contacts.ContactGroup{{ID: 2, Name: "Friends"}},
			Contacts: []contacts.Contact{{
				ID:       7,
				GroupID:  2,
				Kind:     "mig33",
				Username: "bob",
				Display:  "Robert",
				External: []contacts.ContactIM{{Service: 2, Username: "external-bob"}},
			}},
		}, nil
	}}
	snapshot, err := applicationWithRepositoryPort(repo).ContactsSnapshot(context.Background(), "alice-session")
	if err != nil || snapshot.Revision != 8 || snapshot.Checkpoint != 42 || len(snapshot.Groups) != 1 || len(snapshot.Contacts) != 1 {
		t.Fatalf("contact service result: %+v, %v", snapshot, err)
	}
	contact := snapshot.Contacts[0]
	if contact.ID != 7 ||
		contact.GroupID != 2 ||
		contact.Display != "Robert" ||
		contact.Presence != 3 ||
		contact.Status != "Available for a chat" ||
		len(contact.External) != 1 ||
		contact.External[0].Presence != 99 {
		t.Fatalf("repository data and application presence diverged: %+v", contact)
	}
}
