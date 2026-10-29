package repositories

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoomSearchCategoriesAndDeletionPreserveAudit(t *testing.T) {
	s := testStore(t)
	r, err := s.CreateRoom(t.Context(), "alice", "Island People", "Friendly chat")
	if err != nil {
		t.Fatal(err)
	}
	for op, v := range map[string]string{"language": "Dhivehi", "keywords": "Maldives & reefs_%"} {
		if err = s.ManageRoom(t.Context(), r.Name, "alice", op, v); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		f    RoomSearchFilter
		want int
	}{
		{RoomSearchFilter{Language: "dhivehi", Keywords: "reefs_%"}, 1},
		{RoomSearchFilter{Query: "Friendly", Language: "Dhivehi", Keywords: "Maldives"}, 1},
		{RoomSearchFilter{Language: "English", Keywords: "Maldives"}, 0},
		{RoomSearchFilter{Query: "%"}, 0},
	} {
		got, e := s.SearchRoomsFiltered(t.Context(), tc.f, 100)
		if e != nil || len(got) != tc.want {
			t.Fatalf("search %+v: %+v %v", tc.f, got, e)
		}
	}
	if err = s.ManageRoomCategory(t.Context(), "operator", 2, "Forbidden", false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("reserved favorite category: %v", err)
	}
	if err = s.ManageRoomCategory(t.Context(), "operator", 3, "Islands", false); err != nil {
		t.Fatal(err)
	}
	if err = s.SetRoomCategory(t.Context(), "operator", r.Name, 3); err != nil {
		t.Fatal(err)
	}
	if err = s.ManageRoomCategory(t.Context(), "operator", 3, "", true); !errors.Is(err, ErrConflict) {
		t.Fatalf("deleted nonempty category: %v", err)
	}
	if _, err = s.AddRoomFavorite(t.Context(), "bob", r.Name); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteOwnedRoom(t.Context(), r.Name, "bob"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("nonowner deletion: %v", err)
	}
	if err = s.DeleteOwnedRoom(t.Context(), r.Name, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Room(t.Context(), r.Name); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted room still present")
	}
	favorites, err := s.FavoriteRooms(t.Context(), "bob")
	if err != nil || len(favorites) != 0 {
		t.Fatalf("dangling favorite: %v %v", favorites, err)
	}
	changes, err := s.RoomChanges(t.Context(), r.Name)
	if err != nil || len(changes) != 4 || changes[0].Operation != "delete" {
		t.Fatalf("retained changes: %+v %v", changes, err)
	}
	if _, err = s.CreateRoom(t.Context(), "alice", strings.ToLower(r.Name), "Fresh"); !errors.Is(err, ErrConflict) {
		t.Fatalf("deleted name could inherit history: %v", err)
	}
	if err = s.ManageRoomCategory(t.Context(), "operator", 3, "", true); err != nil {
		t.Fatal(err)
	}
}

func TestGroupRoomControlsPersistenceAndUnlinkIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rooms.sqlite")
	s, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	g, err := s.CreateCommunity(t.Context(), "alice", "Island Group", "", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"One", "Two"} {
		if _, err = s.CreateRoom(t.Context(), "alice", name, ""); err != nil {
			t.Fatal(err)
		}
		if err = s.LinkCommunityRoom(t.Context(), g.ID, "alice", name, false); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.ManageGroupRoom(t.Context(), "One", "bob", "lock", "1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("outsider command: %v", err)
	}
	for op, v := range map[string]string{"lock": "1", "silence": "1", "ban": "carol", "bot": "dice"} {
		if _, err = s.ManageGroupRoom(t.Context(), "One", "alice", op, v); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	join, speak, err := s.GroupRoomAccess(t.Context(), "One", "bob")
	if err != nil || join || speak {
		t.Fatalf("visitor access %v/%v %v", join, speak, err)
	}
	if err = s.CommunityMembership(t.Context(), g.ID, "bob", "bob", "join"); err != nil {
		t.Fatal(err)
	}
	join, speak, err = s.GroupRoomAccess(t.Context(), "One", "bob")
	if err != nil || !join || speak {
		t.Fatalf("member access %v/%v %v", join, speak, err)
	}
	join, _, err = s.GroupRoomAccess(t.Context(), "Two", "carol")
	if err != nil || join {
		t.Fatalf("ban did not cover other linked room %v %v", join, err)
	}
	for i := 0; i < 20; i++ {
		if err = s.PlayRoomBot(t.Context(), "One", "alice", "dice", "roll", 6); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.PlayRoomBot(t.Context(), "One", "alice", "dice", "roll", 6); !errors.Is(err, ErrConflict) {
		t.Fatalf("unbounded bot: %v", err)
	}
	scores, err := s.RoomBotScores(t.Context(), "One")
	if err != nil || len(scores) != 1 || scores[0].Points != 120 || scores[0].Rounds != 20 {
		t.Fatalf("scores %+v %v", scores, err)
	}
	if err = s.LinkCommunityRoom(t.Context(), g.ID, "alice", "One", true); err != nil {
		t.Fatal(err)
	}
	join, speak, err = s.GroupRoomAccess(t.Context(), "One", "bob")
	if err != nil || !join || !speak {
		t.Fatal("unlinked room retained old Group policy")
	}
	if err = s.LinkCommunityRoom(t.Context(), g.ID, "alice", "One", false); err != nil {
		t.Fatal(err)
	}
	controls, err := s.GroupRoomControls(t.Context(), "One")
	if err != nil || controls.Locked || controls.Silenced || controls.Bot != "" {
		t.Fatalf("relinked room inherited flags %+v %v", controls, err)
	}
}
