package repositories

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func contactSeed(id int32, name string) Contact {
	return Contact{ID: id, Username: name, Display: name, GroupID: 1}
}

func roster(t *testing.T, s *Store, owner string) ContactRoster {
	t.Helper()
	r, err := s.Contacts(t.Context(), owner)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestContactsPersistWithoutReseedingAndRemainDirected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "contacts.sqlite")
	s, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	seeds := []Contact{contactSeed(2, "BOB"), contactSeed(3, "carol")}
	if err := s.EnsureContacts(t.Context(), "ALICE", seeds); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureContacts(t.Context(), "bob", nil); err != nil {
		t.Fatal(err)
	}
	before := roster(t, s, "alice")
	if before.Revision != 4 || len(before.Contacts) != 2 || before.Contacts[0].Username != "bob" || len(before.Groups) != 1 {
		t.Fatalf("initial roster: %+v", before)
	}
	if r := roster(t, s, "bob"); len(r.Contacts) != 0 || len(r.Groups) != 1 {
		t.Fatalf("contact became reciprocal: %+v", r)
	}
	if _, err := s.Contact(t.Context(), "bob", 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another owner's contact visible: %v", err)
	}
	if err := s.DeleteContact(t.Context(), "bob", 3); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another owner's contact deleted: %v", err)
	}
	if err := s.DeleteContact(t.Context(), "alice", 3); err != nil {
		t.Fatal(err)
	}
	group, err := s.AddContactGroup(t.Context(), "alice", "Family")
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.ContactByUsername(t.Context(), "alice", "BOB")
	if err != nil {
		t.Fatal(err)
	}
	c.GroupID, c.Display, c.Mobile, c.ShareMobile = group.ID, "Robert", "+9601234567", true
	if _, err := s.UpdateContact(t.Context(), "alice", c); err != nil {
		t.Fatal(err)
	}
	want := roster(t, s, "alice")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.EnsureContacts(t.Context(), "alice", seeds); err != nil {
		t.Fatal(err)
	}
	if got := roster(t, s, "alice"); !reflect.DeepEqual(got, want) {
		t.Fatalf("restart/reseed changed roster: got %+v want %+v", got, want)
	}
	if got, err := s.Contact(t.Context(), "alice", 2); err != nil || !reflect.DeepEqual(got, c) {
		t.Fatalf("contact metadata lost: %+v %v", got, err)
	}
	if _, err := s.ContactByUsername(t.Context(), "alice", "carol"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed contact returned: %v", err)
	}
}

func TestContactDeltaCheckpointsAndDeleteReadd(t *testing.T) {
	s := testStore(t)
	if err := s.EnsureContacts(t.Context(), "alice", nil); err != nil {
		t.Fatal(err)
	}
	initial := roster(t, s, "alice")
	// Simulate a clock rollback/future checkpoint; every mutation must still
	// produce an ordered checkpoint, including several changes in one millis.
	if _, err := s.db.Exec("UPDATE contact_rosters SET checkpoint = checkpoint + 100000 WHERE owner = 'alice'"); err != nil {
		t.Fatal(err)
	}
	base := roster(t, s, "alice")
	c, err := s.AddContact(t.Context(), "alice", contactSeed(2, "bob"))
	if err != nil {
		t.Fatal(err)
	}
	first := roster(t, s, "alice")
	if first.Checkpoint != base.Checkpoint+1 || first.Revision != initial.Revision+1 {
		t.Fatalf("nonmonotonic first change: %+v %+v", base, first)
	}
	if _, err := s.UpdateContact(t.Context(), "alice", c); err != nil {
		t.Fatal(err)
	}
	if got := roster(t, s, "alice"); got.Checkpoint != first.Checkpoint || got.Revision != first.Revision {
		t.Fatal("no-op edit changed checkpoint")
	}
	c.Display = "Bob renamed"
	if _, err := s.UpdateContact(t.Context(), "alice", c); err != nil {
		t.Fatal(err)
	}
	delta, err := s.ContactChanges(t.Context(), "alice", base.Checkpoint)
	if err != nil || len(delta.Contacts) != 1 || !reflect.DeepEqual(delta.Contacts[0], c) || len(delta.Groups) != 0 || len(delta.Deleted) != 0 {
		t.Fatalf("delta: %+v %v", delta, err)
	}
	if delta.Checkpoint != first.Checkpoint+1 {
		t.Fatal("second mutation checkpoint did not advance")
	}
	if err := s.DeleteContact(t.Context(), "alice", 2); err != nil {
		t.Fatal(err)
	}
	deleted, err := s.ContactChanges(t.Context(), "alice", delta.Checkpoint)
	if err != nil || len(deleted.Contacts) != 0 || !reflect.DeepEqual(deleted.Deleted, []int32{2}) {
		t.Fatalf("deletion delta: %+v %v", deleted, err)
	}
	if _, err := s.AddContact(t.Context(), "alice", c); err != nil {
		t.Fatal(err)
	}
	readded, err := s.ContactChanges(t.Context(), "alice", base.Checkpoint)
	if err != nil || len(readded.Contacts) != 1 || !reflect.DeepEqual(readded.Contacts[0], c) || len(readded.Deleted) != 0 {
		t.Fatalf("delete/readd delta: %+v %v", readded, err)
	}
	unchanged, err := s.ContactChanges(t.Context(), "alice", readded.Checkpoint)
	if err != nil || len(unchanged.Contacts)+len(unchanged.Groups)+len(unchanged.Deleted)+len(unchanged.DeletedGroups) != 0 {
		t.Fatalf("empty delta: %+v %v", unchanged, err)
	}
	for _, checkpoint := range []int64{-1, readded.Checkpoint + 1} {
		if _, err := s.ContactChanges(t.Context(), "alice", checkpoint); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid checkpoint accepted: %d %v", checkpoint, err)
		}
	}
}

func TestContactGroupsMoveRenameDeleteAndOwnerIsolation(t *testing.T) {
	s := testStore(t)
	for _, owner := range []string{"alice", "bob"} {
		if err := s.EnsureContacts(t.Context(), owner, nil); err != nil {
			t.Fatal(err)
		}
	}
	initial := roster(t, s, "alice")
	group, err := s.AddContactGroup(t.Context(), "alice", "Work")
	if err != nil || group.ID != 2 {
		t.Fatalf("add group: %+v %v", group, err)
	}
	if _, err := s.AddContactGroup(t.Context(), "alice", "wOrK"); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate group name: %v", err)
	}
	if err := s.RenameContactGroup(t.Context(), "bob", group.ID, "Stolen"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner group rename: %v", err)
	}
	c := contactSeed(3, "carol")
	c.GroupID = group.ID
	if _, err := s.AddContact(t.Context(), "bob", c); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner group accepted: %v", err)
	}
	if _, err := s.AddContact(t.Context(), "alice", c); err != nil {
		t.Fatal(err)
	}
	before := roster(t, s, "alice")
	if err := s.DeleteContactGroup(t.Context(), "alice", group.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("nonempty group deleted: %v", err)
	}
	if got := roster(t, s, "alice"); !reflect.DeepEqual(got, before) {
		t.Fatal("failed group deletion altered roster")
	}
	if err := s.RenameContactGroup(t.Context(), "alice", group.ID, "Colleagues"); err != nil {
		t.Fatal(err)
	}
	delta, err := s.ContactChanges(t.Context(), "alice", initial.Checkpoint)
	if err != nil || !reflect.DeepEqual(delta.Groups, []ContactGroup{{ID: 2, Name: "Colleagues"}}) {
		t.Fatalf("renamed group delta: %+v %v", delta, err)
	}
	c.GroupID = -1
	if _, err := s.UpdateContact(t.Context(), "alice", c); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteContactGroup(t.Context(), "alice", group.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteContactGroup(t.Context(), "alice", 1); err != nil {
		t.Fatal(err)
	}
	delta, err = s.ContactChanges(t.Context(), "alice", initial.Checkpoint)
	if err != nil || len(delta.Groups) != 0 || !reflect.DeepEqual(delta.DeletedGroups, []int32{1, 2}) || len(delta.Contacts) != 1 || delta.Contacts[0].GroupID != -1 {
		t.Fatalf("group deletion delta: %+v %v", delta, err)
	}
	if err := s.EnsureContacts(t.Context(), "alice", nil); err != nil {
		t.Fatal(err)
	}
	if got := roster(t, s, "alice"); len(got.Groups) != 0 {
		t.Fatal("deleted Friends group was reseeded")
	}
	newGroup, err := s.AddContactGroup(t.Context(), "alice", "New")
	if err != nil || newGroup.ID != 3 {
		t.Fatalf("deleted group ID reused: %+v %v", newGroup, err)
	}
}

func TestContactValidationAndRollback(t *testing.T) {
	s := testStore(t)
	if err := s.EnsureContacts(t.Context(), "alice", []Contact{contactSeed(2, "bob")}); err != nil {
		t.Fatal(err)
	}
	before := roster(t, s, "alice")
	bad := []Contact{contactSeed(1, "alice"), contactSeed(0, "bob"), contactSeed(3, " carol"), contactSeed(3, "carol")}
	bad[len(bad)-1].GroupID = 999
	for _, c := range bad {
		if _, err := s.AddContact(t.Context(), "alice", c); err == nil {
			t.Fatalf("invalid contact accepted: %+v", c)
		}
	}
	changed := contactSeed(2, "carol")
	if _, err := s.UpdateContact(t.Context(), "alice", changed); !errors.Is(err, ErrInvalid) {
		t.Fatalf("contact identity overwritten: %v", err)
	}
	for _, name := range []string{"", " Work", "Work\x00Bad"} {
		if _, err := s.AddContactGroup(t.Context(), "alice", name); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid group accepted: %q %v", name, err)
		}
	}
	if _, err := s.db.Exec(`CREATE TRIGGER reject_contact_insert BEFORE INSERT ON contacts WHEN NEW.username = 'carol' BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddContact(t.Context(), "alice", contactSeed(3, "carol")); err == nil {
		t.Fatal("injected insertion should fail")
	}
	if _, err := s.db.Exec(`CREATE TRIGGER reject_contact_tombstone BEFORE INSERT ON contact_deletions BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteContact(t.Context(), "alice", 2); err == nil {
		t.Fatal("injected deletion should fail")
	}
	if got := roster(t, s, "alice"); !reflect.DeepEqual(got, before) {
		t.Fatalf("failed write did not roll back roster/checkpoint: %+v %+v", got, before)
	}
	if err := s.EnsureContacts(t.Context(), "carol", []Contact{contactSeed(2, "bob"), contactSeed(2, "alice")}); err == nil {
		t.Fatal("duplicate seed accepted")
	}
	if _, err := s.Contacts(t.Context(), "carol"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("partially seeded roster survived: %v", err)
	}
}

func TestConcurrentContactAddDoesNotDuplicate(t *testing.T) {
	s := testStore(t)
	if err := s.EnsureContacts(t.Context(), "alice", nil); err != nil {
		t.Fatal(err)
	}
	before := roster(t, s, "alice")
	var workers sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := s.AddContact(t.Context(), "alice", contactSeed(2, "bob"))
			results <- err
		}()
	}
	workers.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	after := roster(t, s, "alice")
	if success != 1 || conflict != 1 || len(after.Contacts) != 1 || after.Revision != before.Revision+1 {
		t.Fatalf("duplicate add: success=%d conflict=%d roster=%+v", success, conflict, after)
	}
}

func TestContactMigrationV4PreservesPriorData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v4.sqlite")
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
		t.Fatal(err)
	}
	if _, err := tx.Exec(schemaV3 + schemaV4); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO personal_blocks(owner,target) VALUES ('alice','bob'); INSERT INTO room_mutes(room,username) VALUES ('Lobby','bob')`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
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
	if blocked, err := s.IsBlocked(t.Context(), "alice", "bob"); err != nil || !blocked {
		t.Fatalf("block lost: %v %v", blocked, err)
	}
	if muted, err := s.IsRoomMuted(t.Context(), "Lobby", "bob"); err != nil || !muted {
		t.Fatalf("mute lost: %v %v", muted, err)
	}
	if roles, err := s.RoomModerators(t.Context(), "Lobby"); err != nil || !reflect.DeepEqual(roles, []string{"alice"}) {
		t.Fatalf("moderator lost: %v %v", roles, err)
	}
	assertIDs(t, history(t, s, "alice", 1, "bob"), 7)
	assertIDs(t, history(t, s, "bob", 2, "existing"), 9, 10)
	if _, err := s.Contacts(t.Context(), "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("migration invented account data: %v", err)
	}
	if err := s.EnsureContacts(t.Context(), "alice", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddContact(t.Context(), "alice", contactSeed(2, "bob")); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 18 {
		t.Fatalf("schema version: %d %v", version, err)
	}
}
