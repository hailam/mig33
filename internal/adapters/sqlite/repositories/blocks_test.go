package repositories

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPersonalBlocksPersistAndUnblockDoesNotReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blocks.sqlite")
	store, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	send := func(from, to, body string) Message {
		return save(t, store, Message{Kind: 1, Conversation: to, Sender: from, Body: body}, []string{to})
	}
	delivered := send("alice", "bob", "Already delivered")
	if err := store.MarkDelivered(t.Context(), "bob", delivered.ID); err != nil {
		t.Fatal(err)
	}
	queued := send("alice", "bob", "Not delivered")
	reverse := send("bob", "alice", "Opposite direction")
	other := send("carol", "bob", "Other sender")
	if err := store.SetBlocked(t.Context(), "BOB", "ALICE", true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetBlocked(t.Context(), "bob", "alice", true); err != nil {
		t.Fatalf("repeated block: %v", err)
	}
	if err := store.SetBlocked(t.Context(), "bob", "dave", true); err != nil {
		t.Fatal(err)
	}
	if flag, err := store.IsBlocked(t.Context(), "Bob", "Alice"); err != nil || !flag {
		t.Fatalf("block lookup: %v %v", flag, err)
	}
	if flag, err := store.IsBlocked(t.Context(), "alice", "bob"); err != nil || flag {
		t.Fatalf("block must be directed: %v %v", flag, err)
	}
	assertIDs(t, pending(t, store, "bob"), other.ID)
	assertIDs(t, history(t, store, "bob", 1, "alice"), delivered.ID, queued.ID, reverse.ID)
	blocked := send("alice", "bob", "Saved but hidden from Bob")
	assertIDs(t, history(t, store, "alice", 1, "bob"), delivered.ID, queued.ID, reverse.ID, blocked.ID)
	assertIDs(t, history(t, store, "bob", 1, "alice"), delivered.ID, queued.ID, reverse.ID)
	for _, id := range []int64{queued.ID, blocked.ID} {
		if err := store.MarkDelivered(t.Context(), "bob", id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("ineligible delivery %d: %v", id, err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	users, err := store.BlockedUsers(t.Context(), "BOB")
	if err != nil || !reflect.DeepEqual(users, []string{"alice", "dave"}) {
		t.Fatalf("persistent sorted block list: %v %v", users, err)
	}
	assertIDs(t, pending(t, store, "bob"), other.ID)
	if err := store.SetBlocked(t.Context(), "bob", "alice", false); err != nil {
		t.Fatal(err)
	}
	if err := store.SetBlocked(t.Context(), "bob", "alice", false); err != nil {
		t.Fatalf("repeated unblock: %v", err)
	}
	assertIDs(t, pending(t, store, "bob"), other.ID)
	after := send("alice", "bob", "After unblock")
	assertIDs(t, pending(t, store, "bob"), other.ID, after.ID)
	assertIDs(t, history(t, store, "bob", 1, "alice"), delivered.ID, queued.ID, reverse.ID, after.ID)
	if flag, err := store.IsBlocked(t.Context(), "bob", "alice"); err != nil || flag {
		t.Fatalf("unblock: %v %v", flag, err)
	}
	if err := store.SetBlocked(t.Context(), "Alice", "ALICE", true); !errors.Is(err, ErrInvalid) {
		t.Fatalf("self block: %v", err)
	}
	if _, err := store.IsBlocked(t.Context(), "alice", "ALICE"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("self predicate: %v", err)
	}
	if err := store.SetBlocked(t.Context(), "", "bob", true); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty owner: %v", err)
	}
}

func TestPersonalBlockFiltersGroupAndRoomAudience(t *testing.T) {
	store := testStore(t)
	if _, err := store.CreateGroup(t.Context(), "g", "alice", []string{"bob", "carol"}); err != nil {
		t.Fatal(err)
	}
	groupSend := func(sender string) Message {
		return save(t, store, Message{Kind: 2, Conversation: "g", Sender: sender, Body: "Group message"}, nil)
	}
	roomSend := func() Message {
		return save(t, store, Message{Kind: 3, Conversation: "Lobby", Sender: "alice", Body: "Room message"}, []string{"alice", "bob", "carol"})
	}
	oldGroup := groupSend("alice")
	oldRoom := roomSend()
	if err := store.SetBlocked(t.Context(), "bob", "alice", true); err != nil {
		t.Fatal(err)
	}
	newGroup := groupSend("alice")
	newRoom := roomSend()
	assertIDs(t, pending(t, store, "bob"))
	assertIDs(t, pending(t, store, "carol"), oldGroup.ID, newGroup.ID)
	assertIDs(t, history(t, store, "bob", 2, "g"), oldGroup.ID)
	assertIDs(t, history(t, store, "carol", 2, "g"), oldGroup.ID, newGroup.ID)
	assertIDs(t, history(t, store, "bob", 3, "Lobby"), oldRoom.ID)
	assertIDs(t, history(t, store, "carol", 3, "Lobby"), oldRoom.ID, newRoom.ID)
	if err := store.MarkDelivered(t.Context(), "bob", oldRoom.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("queued room traffic was not canceled: %v", err)
	}
	other := groupSend("carol")
	assertIDs(t, pending(t, store, "bob"), other.ID)
	if err := store.SetBlocked(t.Context(), "carol", "alice", true); err != nil {
		t.Fatal(err)
	}
	senderOnlyGroup := groupSend("alice")
	senderOnlyRoom := roomSend()
	assertIDs(t, history(t, store, "alice", 2, "g"), oldGroup.ID, newGroup.ID, other.ID, senderOnlyGroup.ID)
	assertIDs(t, history(t, store, "alice", 3, "Lobby"), oldRoom.ID, newRoom.ID, senderOnlyRoom.ID)
	assertIDs(t, history(t, store, "bob", 2, "g"), oldGroup.ID, other.ID)
	assertIDs(t, history(t, store, "carol", 2, "g"), oldGroup.ID, newGroup.ID, other.ID)
	assertIDs(t, history(t, store, "bob", 3, "Lobby"), oldRoom.ID)
}

func TestBlockCancellationRollbackAndPendingDefense(t *testing.T) {
	store := testStore(t)
	message := save(t, store, Message{Kind: 1, Conversation: "bob", Sender: "alice", Body: "Pending"}, []string{"bob"})
	if _, err := store.db.Exec(`CREATE TRIGGER fail_block_cancel BEFORE UPDATE OF canceled_at ON message_audience BEGIN SELECT RAISE(ABORT, 'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := store.SetBlocked(t.Context(), "bob", "alice", true); err == nil {
		t.Fatal("block must fail if cancellation fails")
	}
	if flag, err := store.IsBlocked(t.Context(), "bob", "alice"); err != nil || flag {
		t.Fatalf("partial block survived: %v %v", flag, err)
	}
	assertIDs(t, pending(t, store, "bob"), message.ID)
	if _, err := store.db.Exec("DROP TRIGGER fail_block_cancel"); err != nil {
		t.Fatal(err)
	}
	// Simulate a row written outside the public API: Pending must still honor
	// active blocks even if the expected cancellation was not applied.
	if _, err := store.db.Exec("INSERT INTO personal_blocks(owner, target) VALUES ('bob', 'alice')"); err != nil {
		t.Fatal(err)
	}
	assertIDs(t, pending(t, store, "bob"))
	if err := store.SetBlocked(t.Context(), "bob", "alice", true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetBlocked(t.Context(), "bob", "alice", false); err != nil {
		t.Fatal(err)
	}
	assertIDs(t, pending(t, store, "bob"))
}

func TestBlocksPreventNewGroupInvitesWithoutChangingExistingMembership(t *testing.T) {
	store := testStore(t)
	if _, err := store.CreateGroup(t.Context(), "existing", "alice", []string{"bob"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetBlocked(t.Context(), "bob", "alice", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateGroup(t.Context(), "blocked", "alice", []string{"bob", "carol"}); !errors.Is(err, ErrBlocked) {
		t.Fatalf("blocked group create: %v", err)
	}
	if _, err := store.Group(t.Context(), "blocked", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("partial group creation survived: %v", err)
	}
	group, err := store.Invite(t.Context(), "existing", "alice", []string{"BOB"})
	if err != nil || !reflect.DeepEqual(group.Members, []string{"alice", "bob"}) {
		t.Fatalf("existing member invite changed group: %+v %v", group, err)
	}
	if err := store.SetBlocked(t.Context(), "dave", "alice", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Invite(t.Context(), "existing", "alice", []string{"carol", "dave"}); !errors.Is(err, ErrBlocked) {
		t.Fatalf("mixed invite should fail: %v", err)
	}
	group, err = store.Group(t.Context(), "existing", "bob")
	if err != nil || !reflect.DeepEqual(group.Members, []string{"alice", "bob"}) {
		t.Fatalf("mixed invitation was not rolled back: %+v %v", group, err)
	}
	group, err = store.Invite(t.Context(), "existing", "bob", []string{"dave"})
	if err != nil || !reflect.DeepEqual(group.Members, []string{"alice", "bob", "dave"}) {
		t.Fatalf("unblocked inviter should succeed: %+v %v", group, err)
	}
	if err := store.SetBlocked(t.Context(), "alice", "carol", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateGroup(t.Context(), "outgoing", "alice", []string{"carol"}); err != nil {
		t.Fatalf("actor's outgoing invitation should remain allowed: %v", err)
	}
}

func TestMigrationV3PreservesModerationAndChat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version3.sqlite")
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
	if _, err := tx.Exec(schemaV3); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM room_moderators WHERE room = 'Lobby' AND username = 'alice';
INSERT INTO room_moderators(room, username) VALUES ('Lobby', 'carol');
INSERT INTO room_mutes(room, username) VALUES ('Lobby', 'bob');
INSERT INTO messages(id, kind, conversation, sender, body, created_at) VALUES (91, 3, 'Lobby', 'alice', 'Old room message', 1238000);
INSERT INTO message_audience(message_id, username, recipient, delivered_at) VALUES (91, 'alice', 0, NULL), (91, 'bob', 1, 333);`); err != nil {
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
	assertIDs(t, history(t, store, "bob", 3, "Lobby"), 91)
	assertIDs(t, pending(t, store, "alice"), 10)
	group, err := store.Group(t.Context(), "existing", "alice")
	if err != nil || !reflect.DeepEqual(group.Members, []string{"alice", "bob"}) {
		t.Fatalf("group changed: %+v %v", group, err)
	}
	roles, err := store.RoomModerators(t.Context(), "Lobby")
	if err != nil || !reflect.DeepEqual(roles, []string{"carol"}) {
		t.Fatalf("roles changed: %v %v", roles, err)
	}
	if flag, err := store.IsRoomMuted(t.Context(), "Lobby", "bob"); err != nil || !flag {
		t.Fatalf("mute changed: %v %v", flag, err)
	}
	users, err := store.BlockedUsers(t.Context(), "bob")
	if err != nil || len(users) != 0 {
		t.Fatalf("new block list: %v %v", users, err)
	}
	var version, delivered, canceled int
	if err := store.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 18 {
		t.Fatalf("version: %d %v", version, err)
	}
	if err := store.db.QueryRow("SELECT delivered_at FROM message_audience WHERE message_id = 91 AND username = 'bob'").Scan(&delivered); err != nil || delivered != 333 {
		t.Fatalf("delivery changed: %d %v", delivered, err)
	}
	if err := store.db.QueryRow("SELECT canceled_at FROM message_audience WHERE message_id = 9 AND username = 'bob'").Scan(&canceled); err != nil || canceled != 222 {
		t.Fatalf("cancellation changed: %d %v", canceled, err)
	}
	message := save(t, store, Message{Kind: 1, Conversation: "bob", Sender: "alice", Body: "After migration"}, []string{"bob"})
	if message.ID != 92 {
		t.Fatalf("sequence changed: got %d, want92", message.ID)
	}
}
