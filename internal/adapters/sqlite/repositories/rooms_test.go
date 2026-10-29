package repositories

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRoomsAreSeededAndRoomTrafficNeverReplays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rooms.sqlite")
	store, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	rooms, err := store.Rooms(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(rooms))
	for i, room := range rooms {
		names[i] = room.Name
		if room.Category != 1 || room.Capacity != 50 || room.Description == "" {
			t.Fatalf("room definition: %+v", room)
		}
	}
	if !reflect.DeepEqual(names, []string{"Gaming", "Help", "Lobby"}) {
		t.Fatalf("room names: %v", names)
	}
	all, err := store.Rooms(t.Context(), 0)
	if err != nil || !reflect.DeepEqual(all, rooms) {
		t.Fatalf("all rooms: %+v, %v", all, err)
	}
	if unknown, err := store.Rooms(t.Context(), 2); err != nil || len(unknown) != 0 {
		t.Fatalf("unknown category: %v, %v", unknown, err)
	}
	if _, err := store.Rooms(t.Context(), -1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("negative category: %v", err)
	}
	room, err := store.Room(t.Context(), "lObBy")
	if err != nil || room.Name != "Lobby" {
		t.Fatalf("room lookup: %+v, %v", room, err)
	}
	if _, err := store.Room(t.Context(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown room: %v", err)
	}
	message := save(t, store, Message{Kind: 3, Conversation: "lobby", Sender: "ALICE", Body: "Hello room"}, []string{"alice", "BOB", "bob"})
	if message.Conversation != "Lobby" {
		t.Fatalf("noncanonical conversation %q", message.Conversation)
	}
	solo := save(t, store, Message{Kind: 3, Conversation: "Lobby", Sender: "alice", Body: "Alone now"}, nil)
	assertIDs(t, pending(t, store, "bob"))
	assertIDs(t, history(t, store, "bob", 3, "LOBBY"), message.ID)
	assertIDs(t, history(t, store, "alice", 3, "Lobby"), message.ID, solo.ID)
	assertIDs(t, history(t, store, "carol", 3, "Lobby"))
	if err := store.MarkDelivered(t.Context(), "alice", message.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sender echo: %v", err)
	}
	if err := store.MarkDelivered(t.Context(), "bob", message.ID); err != nil {
		t.Fatal(err)
	}
	undelivered := save(t, store, Message{Kind: 3, Conversation: "Lobby", Sender: "alice", Body: "Not yet written"}, []string{"bob"})
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, pending(t, store, "bob"))
	assertIDs(t, history(t, store, "bob", 3, "Lobby"), message.ID, undelivered.ID)
	for _, test := range []struct {
		message    Message
		recipients []string
		want       error
	}{
		{Message{Kind: 3, Conversation: "Missing", Sender: "alice", Body: "x"}, nil, ErrNotFound},
		{Message{Kind: 3, Conversation: "Lobby", Sender: "alice", Body: "x"}, []string{""}, ErrInvalid},
	} {
		if _, err := store.SaveMessage(t.Context(), test.message, test.recipients); !errors.Is(err, test.want) {
			t.Fatalf("room validation: %v, want %v", err, test.want)
		}
	}
}

func TestMigrationV1PreservesMessagesAudienceGroupsAndSequence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.sqlite")
	createV1Fixture(t, path)
	store, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	group, err := store.Group(t.Context(), "existing", "bob")
	if err != nil || group.Creator != "alice" || !reflect.DeepEqual(group.Members, []string{"alice", "bob"}) {
		t.Fatalf("migrated group: %+v, %v", group, err)
	}
	assertIDs(t, history(t, store, "alice", 1, "bob"), 7)
	assertIDs(t, history(t, store, "bob", 1, "alice"), 7)
	assertIDs(t, history(t, store, "bob", 2, "existing"), 9, 10)
	assertIDs(t, pending(t, store, "bob"))
	assertIDs(t, pending(t, store, "alice"), 10)
	message := history(t, store, "alice", 1, "bob")[0]
	if message.Body != "Old private 世界" || message.Color == nil || *message.Color != 0x123456 || message.CreatedAt.UnixMilli() != 1234000 {
		t.Fatalf("message data changed: %+v", message)
	}
	var delivered, canceled int64
	if err := store.db.QueryRow("SELECT delivered_at FROM message_audience WHERE message_id = 7 AND username = 'bob'").Scan(&delivered); err != nil || delivered != 111 {
		t.Fatalf("delivery lost: %d %v", delivered, err)
	}
	if err := store.db.QueryRow("SELECT canceled_at FROM message_audience WHERE message_id = 9 AND username = 'bob'").Scan(&canceled); err != nil || canceled != 222 {
		t.Fatalf("cancellation lost: %d %v", canceled, err)
	}
	roomMessage := save(t, store, Message{Kind: 3, Conversation: "help", Sender: "alice", Body: "New room message"}, []string{"bob"})
	if roomMessage.ID != 91 {
		t.Fatalf("autoincrement high-water mark lost: id %d, want 91", roomMessage.ID)
	}
	assertIDs(t, pending(t, store, "bob"))
	assertIDs(t, history(t, store, "bob", 3, "Help"), 91)
	var version, foreignKeys int
	if err := store.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 18 {
		t.Fatalf("version: %d %v", version, err)
	}
	if err := store.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign keys: %d %v", foreignKeys, err)
	}
	if _, err := store.db.Exec("INSERT INTO message_audience(message_id, username, recipient) VALUES (9999, 'bob', 1)"); err == nil {
		t.Fatal("foreign key no longer enforced after migration")
	}
	if _, err := store.db.Exec("INSERT INTO messages(kind, conversation, sender, body, created_at) VALUES (4, 'x', 'alice', 'x', 1)"); err == nil {
		t.Fatal("kind constraint no longer enforced after migration")
	}
}

func TestMigrationFailureRollsBackBothRebuiltTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollback.sqlite")
	createV1Fixture(t, path)
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE chat_rooms(block_migration INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if store, err := Open(t.Context(), path); err == nil {
		store.Close()
		t.Fatal("migration should fail at conflicting room table")
	}
	db, err = sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, count, sequence int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 1 {
		t.Fatalf("rollback version: %d %v", version, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM message_audience").Scan(&count); err != nil || count != 6 {
		t.Fatalf("rollback audience: %d %v", count, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count); err != nil || count != 3 {
		t.Fatalf("rollback messages: %d %v", count, err)
	}
	if err := db.QueryRow("SELECT seq FROM sqlite_sequence WHERE name = 'messages'").Scan(&sequence); err != nil || sequence != 90 {
		t.Fatalf("rollback sequence: %d %v", sequence, err)
	}
	if _, err := db.Exec("INSERT INTO messages(kind, conversation, sender, body, created_at) VALUES (3, 'Lobby', 'alice', 'x', 1)"); err == nil {
		t.Fatal("v1 constraint changed despite rollback")
	}
	if _, err := db.Exec("DROP TABLE chat_rooms"); err != nil {
		t.Fatal(err)
	}
	store, err := Open(t.Context(), path)
	if err != nil {
		t.Fatalf("migration retry: %v", err)
	}
	defer store.Close()
	assertIDs(t, history(t, store, "alice", 1, "bob"), 7)
}

func createV1Fixture(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(schemaV1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO chat_groups(id, creator, created_at) VALUES ('existing', 'alice', 1);
INSERT INTO group_members(group_id, username, joined_at) VALUES ('existing', 'alice', 1), ('existing', 'bob', 1);
INSERT INTO messages(id, kind, conversation, sender, body, color, created_at) VALUES
    (7, 1, 'bob', 'alice', 'Old private 世界', 1193046, 1234000),
    (9, 2, 'existing', 'alice', 'Canceled old group', NULL, 1235000),
    (10, 2, 'existing', 'bob', 'Pending group reply', NULL, 1236000),
    (90, 1, 'bob', 'alice', 'Deleted high ID', NULL, 1237000);
DELETE FROM messages WHERE id = 90;
INSERT INTO message_audience(message_id, username, recipient, delivered_at, canceled_at) VALUES
    (7, 'alice', 0, NULL, NULL), (7, 'bob', 1, 111, NULL),
    (9, 'alice', 0, NULL, NULL), (9, 'bob', 1, NULL, 222),
    (10, 'bob', 0, NULL, NULL), (10, 'alice', 1, NULL, NULL);
`); err != nil {
		t.Fatal(err)
	}
}
