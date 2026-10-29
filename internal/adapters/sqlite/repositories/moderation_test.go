package repositories

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRoomModerationAuthorizationAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "moderation.sqlite")
	store, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	for _, room := range []string{"Lobby", "Gaming", "Help"} {
		moderators, err := store.RoomModerators(t.Context(), room)
		if err != nil || !reflect.DeepEqual(moderators, []string{"alice"}) {
			t.Fatalf("seed moderators for %s: %v %v", room, moderators, err)
		}
	}
	if moderator, err := store.IsRoomModerator(t.Context(), "lObBy", "ALICE"); err != nil || !moderator {
		t.Fatalf("canonical moderator lookup: %v %v", moderator, err)
	}
	for _, request := range []struct {
		actor, target string
		muted         bool
	}{
		{"bob", "carol", true}, {"alice", "ALICE", true}, {"alice", "ALICE", false},
	} {
		if err := store.SetRoomMuted(t.Context(), "Lobby", request.actor, request.target, request.muted); !errors.Is(err, ErrForbidden) {
			t.Fatalf("unauthorized mute %+v: %v", request, err)
		}
	}
	if err := store.SetRoomMuted(t.Context(), "lobby", "ALICE", "BOB", true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRoomMuted(t.Context(), "Lobby", "alice", "bob", true); err != nil {
		t.Fatalf("idempotent mute: %v", err)
	}
	muted, err := store.RoomMuted(t.Context(), "LOBBY")
	if err != nil || !reflect.DeepEqual(muted, []string{"bob"}) {
		t.Fatalf("mute list: %v %v", muted, err)
	}
	if _, err := store.SaveMessage(t.Context(), Message{Kind: 3, Conversation: "lobby", Sender: "BOB", Body: "Muted message"}, []string{"alice"}); !errors.Is(err, ErrMuted) {
		t.Fatalf("muted send: %v", err)
	}
	assertIDs(t, history(t, store, "bob", 3, "Lobby"))
	assertIDs(t, history(t, store, "alice", 3, "Lobby"))
	// A mute is local to its room and does not prevent private messaging.
	save(t, store, Message{Kind: 3, Conversation: "Help", Sender: "bob", Body: "Other room"}, nil)
	save(t, store, Message{Kind: 1, Conversation: "alice", Sender: "bob", Body: "Private"}, []string{"alice"})
	if err := store.SetRoomModerator(t.Context(), "Lobby", "carol", true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRoomMuted(t.Context(), "Lobby", "alice", "carol", true); !errors.Is(err, ErrForbidden) {
		t.Fatalf("target moderator: %v", err)
	}
	if err := store.SetRoomMuted(t.Context(), "Lobby", "carol", "alice", false); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unmute moderator: %v", err)
	}
	if err := store.SetRoomMuted(t.Context(), "Lobby", "carol", "bob", false); err != nil {
		t.Fatal(err)
	}
	if flag, err := store.IsRoomMuted(t.Context(), "lobby", "BOB"); err != nil || flag {
		t.Fatalf("unmute failed: %v %v", flag, err)
	}
	if err := store.SetRoomMuted(t.Context(), "Lobby", "carol", "bob", true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRoomModerator(t.Context(), "Lobby", "alice", false); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if flag, err := store.IsRoomMuted(t.Context(), "Lobby", "bob"); err != nil || !flag {
		t.Fatalf("mute did not persist: %v %v", flag, err)
	}
	if flag, err := store.IsRoomModerator(t.Context(), "Lobby", "alice"); err != nil || flag {
		t.Fatalf("revocation did not persist: %v %v", flag, err)
	}
	if flag, err := store.IsRoomModerator(t.Context(), "Gaming", "alice"); err != nil || !flag {
		t.Fatalf("other room role changed: %v %v", flag, err)
	}
	if err := store.SetRoomMuted(t.Context(), "Lobby", "alice", "dave", true); !errors.Is(err, ErrForbidden) {
		t.Fatalf("revoked actor: %v", err)
	}
	if err := store.SetRoomModerator(t.Context(), "LOBBY", "BOB", true); err != nil {
		t.Fatal(err)
	}
	if flag, err := store.IsRoomMuted(t.Context(), "Lobby", "bob"); err != nil || flag {
		t.Fatalf("promotion did not clear mute: %v %v", flag, err)
	}
	save(t, store, Message{Kind: 3, Conversation: "Lobby", Sender: "bob", Body: "Promoted"}, nil)
	if err := store.SetRoomMuted(t.Context(), "Lobby", "carol", "bob", true); !errors.Is(err, ErrForbidden) {
		t.Fatalf("new moderator not protected: %v", err)
	}
	if _, err := store.IsRoomMuted(t.Context(), "unknown", "bob"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown room: %v", err)
	}
	if err := store.SetRoomModerator(t.Context(), "Lobby", "", true); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty role username: %v", err)
	}
}

func TestModeratorPromotionAndUnmuteAreAtomic(t *testing.T) {
	store := testStore(t)
	if err := store.SetRoomMuted(t.Context(), "Lobby", "alice", "bob", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`CREATE TRIGGER fail_unmute BEFORE DELETE ON room_mutes BEGIN SELECT RAISE(ABORT, 'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRoomModerator(t.Context(), "Lobby", "bob", true); err == nil {
		t.Fatal("promotion should fail when unmute fails")
	}
	if flag, err := store.IsRoomModerator(t.Context(), "Lobby", "bob"); err != nil || flag {
		t.Fatalf("partial promotion survived rollback: %v %v", flag, err)
	}
	if flag, err := store.IsRoomMuted(t.Context(), "Lobby", "bob"); err != nil || !flag {
		t.Fatalf("mute lost on rollback: %v %v", flag, err)
	}
}

func TestMigrationV2PreservesAllChatKinds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version2.sqlite")
	createV1Fixture(t, path)
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := migrateV2(t.Context(), tx); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO messages(id, kind, conversation, sender, body, created_at) VALUES (91, 3, 'Lobby', 'bob', 'Old room history', 1238000);
INSERT INTO message_audience(message_id, username, recipient, delivered_at) VALUES (91, 'bob', 0, NULL), (91, 'alice', 1, 333);`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	assertIDs(t, history(t, store, "alice", 1, "bob"), 7)
	assertIDs(t, history(t, store, "bob", 2, "existing"), 9, 10)
	assertIDs(t, history(t, store, "alice", 3, "Lobby"), 91)
	assertIDs(t, pending(t, store, "alice"), 10)
	group, err := store.Group(t.Context(), "existing", "bob")
	if err != nil || !reflect.DeepEqual(group.Members, []string{"alice", "bob"}) {
		t.Fatalf("group changed: %+v %v", group, err)
	}
	var delivered, version int
	if err := store.db.QueryRow("SELECT delivered_at FROM message_audience WHERE message_id = 91 AND username = 'alice'").Scan(&delivered); err != nil || delivered != 333 {
		t.Fatalf("room delivery changed: %d %v", delivered, err)
	}
	if err := store.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 18 {
		t.Fatalf("version: %d %v", version, err)
	}
	if flag, err := store.IsRoomModerator(t.Context(), "Lobby", "alice"); err != nil || !flag {
		t.Fatalf("moderator not seeded: %v %v", flag, err)
	}
	message := save(t, store, Message{Kind: 3, Conversation: "Lobby", Sender: "bob", Body: "After migration"}, nil)
	if message.ID != 92 {
		t.Fatalf("sequence changed: got %d, want92", message.ID)
	}
}
