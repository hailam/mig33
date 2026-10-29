package repositories

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestActionMessagesPersistAndDeliverToTheirSender(t *testing.T) {
	path := filepath.Join(t.TempDir(), "actions.sqlite")
	s, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if _, err := s.CreateGroup(t.Context(), "actions", "alice", []string{"bob", "carol"}); err != nil {
		t.Fatal(err)
	}
	ordinary := save(t, s, Message{Kind: 1, Conversation: "bob", Sender: "alice", Body: "Ordinary chat"}, []string{"bob"})
	private := save(t, s, Message{Kind: 1, Conversation: "bob", Sender: "alice", Body: "alice waves to bob.", Action: true}, []string{"bob"})
	group := save(t, s, Message{Kind: 2, Conversation: "actions", Sender: "alice", Body: "alice smiles.", Action: true}, nil)
	room := save(t, s, Message{Kind: 3, Conversation: "lobby", Sender: "alice", Body: "alice laughs.", Action: true}, []string{"alice", "bob"})
	self := save(t, s, Message{Kind: 1, Conversation: "alice", Sender: "alice", Body: "ordinary self message"}, []string{"alice"})
	assertIDs(t, pending(t, s, "alice"), private.ID, group.ID, self.ID)
	assertIDs(t, pending(t, s, "bob"), ordinary.ID, private.ID, group.ID)
	assertIDs(t, pending(t, s, "carol"), group.ID)
	if err := s.MarkDelivered(t.Context(), "alice", ordinary.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ordinary sender acquired delivery: %v", err)
	}
	if err := s.MarkDelivered(t.Context(), "alice", private.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, pending(t, s, "alice"), group.ID, self.ID)
	assertIDs(t, pending(t, s, "bob"), ordinary.ID, private.ID, group.ID)
	for _, test := range []struct {
		viewer string
		kind   byte
		target string
		want   []Message
	}{
		{"alice", 1, "bob", []Message{ordinary, private}},
		{"bob", 1, "alice", []Message{ordinary, private}},
		{"alice", 2, "actions", []Message{group}},
		{"alice", 3, "Lobby", []Message{room}},
		{"bob", 3, "Lobby", []Message{room}},
	} {
		if got := history(t, s, test.viewer, test.kind, test.target); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("history %s/%d/%s: %+v, want %+v", test.viewer, test.kind, test.target, got, test.want)
		}
	}
	if err := s.MarkDelivered(t.Context(), "bob", private.ID); err != nil {
		t.Fatal(err)
	}
	assertIDs(t, pending(t, s, "bob"), ordinary.ID, group.ID)
}

func TestActionBlocksPreserveActorEchoAndFilterRecipients(t *testing.T) {
	s := testStore(t)
	if _, err := s.CreateGroup(t.Context(), "actions", "alice", []string{"bob", "carol"}); err != nil {
		t.Fatal(err)
	}
	first := save(t, s, Message{Kind: 1, Conversation: "bob", Sender: "alice", Body: "alice waves to bob.", Action: true}, []string{"bob"})
	if err := s.SetBlocked(t.Context(), "bob", "alice", true); err != nil {
		t.Fatal(err)
	}
	second := save(t, s, Message{Kind: 1, Conversation: "bob", Sender: "alice", Body: "alice smiles at bob.", Action: true}, []string{"bob"})
	group := save(t, s, Message{Kind: 2, Conversation: "actions", Sender: "alice", Body: "alice laughs.", Action: true}, nil)
	assertIDs(t, pending(t, s, "alice"), first.ID, second.ID, group.ID)
	assertIDs(t, pending(t, s, "bob"))
	assertIDs(t, pending(t, s, "carol"), group.ID)
	assertIDs(t, history(t, s, "bob", 1, "alice"), first.ID)
	assertIDs(t, history(t, s, "bob", 2, "actions"))
	if err := s.MarkDelivered(t.Context(), "bob", first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("blocked action delivery was not canceled: %v", err)
	}
	if err := s.SetBlocked(t.Context(), "bob", "alice", false); err != nil {
		t.Fatal(err)
	}
	assertIDs(t, pending(t, s, "bob"))
	if err := s.MarkDelivered(t.Context(), "alice", first.ID); err != nil {
		t.Fatalf("recipient's block canceled actor echo: %v", err)
	}
}

func TestActionGroupLeaveCancelsActorEchoWithoutRemovingHistory(t *testing.T) {
	s := testStore(t)
	if _, err := s.CreateGroup(t.Context(), "actions", "alice", []string{"bob", "carol"}); err != nil {
		t.Fatal(err)
	}
	action := save(t, s, Message{Kind: 2, Conversation: "actions", Sender: "alice", Body: "alice waves.", Action: true}, nil)
	if _, err := s.Leave(t.Context(), "actions", "alice"); err != nil {
		t.Fatal(err)
	}
	assertIDs(t, pending(t, s, "alice"))
	assertIDs(t, pending(t, s, "bob"), action.ID)
	if err := s.MarkDelivered(t.Context(), "alice", action.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("left-group actor echo survived: %v", err)
	}
	if got := history(t, s, "alice", 2, "actions"); len(got) != 1 || !reflect.DeepEqual(got[0], action) {
		t.Fatalf("left-group action history changed: %+v", got)
	}
	if _, err := s.Invite(t.Context(), "actions", "bob", []string{"alice"}); err != nil {
		t.Fatal(err)
	}
	assertIDs(t, pending(t, s, "alice"))
	newAction := save(t, s, Message{Kind: 2, Conversation: "actions", Sender: "alice", Body: "alice returns.", Action: true}, nil)
	assertIDs(t, pending(t, s, "alice"), newAction.ID)
}

func TestActionMigrationPreservesV7MessagesAndAudience(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v7.sqlite")
	createV6DiscoveryFixture(t, path)
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(schemaV7 + `
UPDATE chat_rooms SET owner = 'bob', created_at = 500 WHERE name = 'Old Room';
INSERT INTO room_favorites(owner,room) VALUES ('alice','Old Room');
`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 18 {
		t.Fatalf("version: %d %v", version, err)
	}
	old := history(t, s, "alice", 1, "bob")
	if len(old) != 1 || old[0].ID != 7 || old[0].Action || old[0].Body != "Old private 世界" || old[0].Color == nil || *old[0].Color != 1193046 || old[0].CreatedAt.UnixMilli() != 1234000 {
		t.Fatalf("old message changed: %+v", old)
	}
	assertIDs(t, pending(t, s, "alice"), 10)
	assertIDs(t, pending(t, s, "bob"))
	var recipient int
	if err := s.db.QueryRow("SELECT recipient FROM message_audience WHERE message_id=7 AND username='alice'").Scan(&recipient); err != nil || recipient != 0 {
		t.Fatalf("old sender acquired echo: %d %v", recipient, err)
	}
	favorites, err := s.FavoriteRooms(t.Context(), "alice")
	if err != nil || len(favorites) != 1 || favorites[0].Name != "Old Room" || favorites[0].Owner != "bob" || favorites[0].Capacity != 30 {
		t.Fatalf("v7 room/favorite changed: %+v %v", favorites, err)
	}
	action := save(t, s, Message{Kind: 1, Conversation: "bob", Sender: "alice", Body: "alice waves to bob.", Action: true}, []string{"bob"})
	if action.ID != 91 {
		t.Fatalf("migration changed message sequence: %d", action.ID)
	}
	assertIDs(t, pending(t, s, "alice"), 10, action.ID)
	assertIDs(t, pending(t, s, "bob"), action.ID)
	for _, value := range []string{"2", "-1", "NULL"} {
		if _, err := s.db.Exec("UPDATE messages SET action = " + value + " WHERE id=7"); err == nil {
			t.Fatalf("invalid action value accepted: %s", value)
		}
	}
}
