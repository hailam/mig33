package repositories

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestRestartPersistsMessagesDeliveryAndGroups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "chat?mode=memory&x#1.sqlite")
	store, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	group, err := store.CreateGroup(t.Context(), "group-1", "ALICE", []string{"bob", "BOB"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(group.Members, []string{"alice", "bob"}) {
		t.Fatalf("members: %v", group.Members)
	}
	color := int32(0x123456)
	private := save(t, store, Message{Kind: 1, Conversation: "BOB", Sender: "ALICE", Body: "Hello, 世界", Color: &color}, []string{"bob"})
	groupMessage := save(t, store, Message{Kind: 2, Conversation: group.ID, Sender: "alice", Body: "Group hello"}, []string{"outsider"})
	assertIDs(t, pending(t, store, "bob"), private.ID, groupMessage.ID)
	assertIDs(t, pending(t, store, "outsider"))
	if err := store.MarkDelivered(t.Context(), "bob", private.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkDelivered(t.Context(), "BOB", private.ID); err != nil {
		t.Fatalf("idempotent delivery: %v", err)
	}
	if err := store.MarkDelivered(t.Context(), "carol", groupMessage.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("outsider delivery: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	reopened, err := store.Group(t.Context(), group.ID, "bob")
	if err != nil || !reflect.DeepEqual(reopened, group) {
		t.Fatalf("reopened group %+v: %v", reopened, err)
	}
	assertIDs(t, pending(t, store, "bob"), groupMessage.ID)
	messages := history(t, store, "alice", 1, "bob")
	if len(messages) != 1 || !reflect.DeepEqual(messages[0], private) {
		t.Fatalf("reopened message: %+v, want %+v", messages, private)
	}
	assertIDs(t, history(t, store, "bob", 1, "alice"), private.ID)
	assertIDs(t, history(t, store, "carol", 1, "bob"))
	groups, err := store.Groups(t.Context(), "bob")
	if err != nil || !reflect.DeepEqual(groups, []Group{group}) {
		t.Fatalf("groups: %+v, %v", groups, err)
	}
	for _, check := range []struct {
		path string
		mode os.FileMode
	}{{path, 0o600}, {filepath.Dir(path), 0o700}} {
		info, err := os.Stat(check.path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != check.mode {
			t.Errorf("%s permissions %o, want %o", check.path, info.Mode().Perm(), check.mode)
		}
	}
}

func TestMembershipSnapshotsAndLeaveCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.sqlite")
	store, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if _, err := store.CreateGroup(t.Context(), "g", "alice", []string{"bob"}); err != nil {
		t.Fatal(err)
	}
	send := func(body string) Message {
		return save(t, store, Message{Kind: 2, Conversation: "g", Sender: "alice", Body: body}, nil)
	}
	first := send("Before Carol joined")
	if _, err := store.Invite(t.Context(), "g", "bob", []string{"carol"}); err != nil {
		t.Fatal(err)
	}
	second := send("All three")
	if _, err := store.Leave(t.Context(), "g", "bob"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, pending(t, store, "bob"))
	assertIDs(t, history(t, store, "bob", 2, "g"), first.ID, second.ID)
	if err := store.MarkDelivered(t.Context(), "bob", first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("canceled delivery: %v", err)
	}
	third := send("While Bob is absent")
	if _, err := store.Group(t.Context(), "g", "bob"); !errors.Is(err, ErrNotMember) {
		t.Fatalf("former member group access: %v", err)
	}
	if _, err := store.Invite(t.Context(), "g", "bob", []string{"dave"}); !errors.Is(err, ErrNotMember) {
		t.Fatalf("former member invite: %v", err)
	}
	if _, err := store.SaveMessage(t.Context(), Message{Kind: 2, Conversation: "g", Sender: "bob", Body: "Forbidden"}, nil); !errors.Is(err, ErrNotMember) {
		t.Fatalf("former member send: %v", err)
	}
	if _, err := store.Invite(t.Context(), "g", "alice", []string{"bob"}); err != nil {
		t.Fatal(err)
	}
	fourth := send("Bob returned")
	assertIDs(t, pending(t, store, "bob"), fourth.ID)
	assertIDs(t, history(t, store, "bob", 2, "g"), first.ID, second.ID, fourth.ID)
	assertIDs(t, history(t, store, "carol", 2, "g"), second.ID, third.ID, fourth.ID)
	if _, err := store.History(t.Context(), "dave", 2, "g", 0, 100); !errors.Is(err, ErrNotMember) {
		t.Fatalf("outsider history: %v", err)
	}
	if _, err := store.Group(t.Context(), "missing", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing group: %v", err)
	}
}

func TestTransactionsRollBackPartialWork(t *testing.T) {
	store := testStore(t)
	exec := func(query string) {
		t.Helper()
		if _, err := store.db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TRIGGER fail_member BEFORE INSERT ON group_members WHEN NEW.username = 'dave' BEGIN SELECT RAISE(ABORT, 'test failure'); END`)
	if _, err := store.CreateGroup(t.Context(), "failed", "alice", []string{"bob", "dave"}); err == nil {
		t.Fatal("create should fail")
	}
	if _, err := store.Group(t.Context(), "failed", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("partial group survived: %v", err)
	}
	if _, err := store.CreateGroup(t.Context(), "g", "alice", []string{"bob"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Invite(t.Context(), "g", "alice", []string{"carol", "dave"}); err == nil {
		t.Fatal("invite should fail")
	}
	group, err := store.Group(t.Context(), "g", "alice")
	if err != nil || !reflect.DeepEqual(group.Members, []string{"alice", "bob"}) {
		t.Fatalf("partial invite survived: %+v %v", group, err)
	}
	exec(`CREATE TRIGGER fail_recipient BEFORE INSERT ON message_audience WHEN NEW.username = 'bob' BEGIN SELECT RAISE(ABORT, 'test failure'); END`)
	if _, err := store.SaveMessage(t.Context(), Message{Kind: 1, Conversation: "bob", Sender: "alice", Body: "Fail atomically"}, []string{"bob"}); err == nil {
		t.Fatal("save should fail")
	}
	assertIDs(t, history(t, store, "alice", 1, "bob"))
	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count); err != nil || count != 0 {
		t.Fatalf("orphan messages: %d, %v", count, err)
	}
	exec("DROP TRIGGER fail_recipient")
	message := save(t, store, Message{Kind: 2, Conversation: "g", Sender: "alice", Body: "Still pending"}, nil)
	exec(`CREATE TRIGGER fail_cancel BEFORE UPDATE OF canceled_at ON message_audience BEGIN SELECT RAISE(ABORT, 'test failure'); END`)
	if _, err := store.Leave(t.Context(), "g", "bob"); err == nil {
		t.Fatal("leave should fail")
	}
	if _, err := store.Group(t.Context(), "g", "bob"); err != nil {
		t.Fatalf("membership not restored: %v", err)
	}
	assertIDs(t, pending(t, store, "bob"), message.ID)
}

func TestGroupLimitIsTransactionalUnderConcurrency(t *testing.T) {
	store := testStore(t)
	names := make([]string, 48)
	for i := range names {
		names[i] = fmt.Sprintf("member%02d", i)
	}
	if _, err := store.CreateGroup(t.Context(), "g", "alice", names); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for _, name := range []string{"bob", "carol"} {
		workers.Add(1)
		go func(name string) {
			defer workers.Done()
			_, err := store.Invite(t.Context(), "g", "alice", []string{name})
			results <- err
		}(name)
	}
	workers.Wait()
	close(results)
	successes, limits := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrLimit) {
			limits++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || limits != 1 {
		t.Fatalf("successes %d, limits %d", successes, limits)
	}
	group, err := store.Group(t.Context(), "g", "alice")
	if err != nil || len(group.Members) != 50 {
		t.Fatalf("group after limit: %+v %v", group, err)
	}
	if _, err := store.Invite(t.Context(), "g", "alice", []string{"ALICE"}); err != nil {
		t.Fatalf("existing member should not exceed limit: %v", err)
	}
}

func TestValidationPaginationAndDeliveryAccess(t *testing.T) {
	store := testStore(t)
	valid := Message{Kind: 1, Conversation: "bob", Sender: "alice", Body: "Hi"}
	for _, change := range []func(*Message){
		func(m *Message) { m.Kind = 4 }, func(m *Message) { m.Sender = "" },
		func(m *Message) { m.Conversation = "" }, func(m *Message) { m.Body = "" },
		func(m *Message) { m.Body = string([]byte{255}) },
	} {
		message := valid
		change(&message)
		if _, err := store.SaveMessage(t.Context(), message, []string{"bob"}); !errors.Is(err, ErrInvalid) {
			t.Errorf("invalid message %+v: %v", message, err)
		}
	}
	tooBig := valid
	tooBig.Body = strings.Repeat("x", 65537)
	if _, err := store.SaveMessage(t.Context(), tooBig, []string{"bob"}); !errors.Is(err, ErrLimit) {
		t.Fatalf("large message: %v", err)
	}
	if _, err := store.SaveMessage(t.Context(), valid, []string{"carol"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("wrong private audience: %v", err)
	}
	first := save(t, store, valid, []string{"BOB", "bob"})
	second := save(t, store, Message{Kind: 1, Conversation: "alice", Sender: "bob", Body: "Reply"}, []string{"alice"})
	third := save(t, store, valid, []string{"bob"})
	page, err := store.History(t.Context(), "alice", 1, "bob", first.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, page, second.ID)
	page, err = store.Pending(t.Context(), "bob", 1)
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, page, first.ID)
	assertIDs(t, history(t, store, "alice", 1, "bob"), first.ID, second.ID, third.ID)
	if err := store.MarkDelivered(t.Context(), "alice", first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sender acknowledgment: %v", err)
	}
	if _, err := store.CreateGroup(t.Context(), "g", "alice", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateGroup(t.Context(), "g", "bob", nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate group: %v", err)
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(t.Context(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func save(t *testing.T, store *Store, message Message, recipients []string) Message {
	t.Helper()
	result, err := store.SaveMessage(t.Context(), message, recipients)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func pending(t *testing.T, store *Store, viewer string) []Message {
	t.Helper()
	messages, err := store.Pending(t.Context(), viewer, 1000)
	if err != nil {
		t.Fatal(err)
	}
	return messages
}

func history(t *testing.T, store *Store, viewer string, kind byte, conversation string) []Message {
	t.Helper()
	messages, err := store.History(t.Context(), viewer, kind, conversation, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	return messages
}

func assertIDs(t *testing.T, messages []Message, ids ...int64) {
	t.Helper()
	got := make([]int64, len(messages))
	for i := range messages {
		got[i] = messages[i].ID
	}
	if len(got) != len(ids) {
		t.Fatalf("message IDs: %v, want %v", got, ids)
	}
	for i := range got {
		if got[i] != ids[i] {
			t.Fatalf("message IDs: %v, want %v", got, ids)
		}
	}
}
