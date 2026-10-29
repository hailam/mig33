package repositories

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestCreatedRoomsAndPersonalFavoritesSurviveRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "discovery.sqlite")
	s, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	room, err := s.CreateRoom(t.Context(), "HAILAM", "  Island Chat_1  ", "Welcome & enjoy + 50% = fun 世界")
	if err != nil || room.Name != "Island Chat_1" || room.Owner != "hailam" || room.Category != 1 || room.Capacity != 50 {
		t.Fatalf("create: %+v %v", room, err)
	}
	moderators, err := s.RoomModerators(t.Context(), room.Name)
	if err != nil || !reflect.DeepEqual(moderators, []string{"hailam"}) {
		t.Fatalf("creator role: %v %v", moderators, err)
	}
	replayed, err := s.CreateRoom(t.Context(), "hailam", "island chat_1", room.Description)
	if err != nil || replayed != room {
		t.Fatalf("identical replay: %+v %v", replayed, err)
	}
	if _, err := s.CreateRoom(t.Context(), "bob", "ISLAND CHAT_1", room.Description); !errors.Is(err, ErrConflict) {
		t.Fatalf("other creator took room: %v", err)
	}
	if _, err := s.CreateRoom(t.Context(), "hailam", room.Name, "Changed description"); !errors.Is(err, ErrConflict) {
		t.Fatalf("create edited existing room: %v", err)
	}
	if _, err := s.CreateRoom(t.Context(), "alice", "LOBBY", "Meet other users and chat."); !errors.Is(err, ErrConflict) {
		t.Fatalf("seeded room acquired owner: %v", err)
	}
	if got, err := s.AddRoomFavorite(t.Context(), "BOB", "ISLAND CHAT_1"); err != nil || got != room {
		t.Fatalf("favorite: %+v %v", got, err)
	}
	if _, err := s.AddRoomFavorite(t.Context(), "bob", "island chat_1"); err != nil {
		t.Fatalf("repeated favorite: %v", err)
	}
	if _, err := s.AddRoomFavorite(t.Context(), "alice", room.Name); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRoomModerator(t.Context(), room.Name, "hailam", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRoom(t.Context(), "hailam", room.Name, room.Description); err != nil {
		t.Fatal(err)
	}
	if isMod, err := s.IsRoomModerator(t.Context(), room.Name, "hailam"); err != nil || isMod {
		t.Fatalf("retry restored revoked role: %t %v", isMod, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Room(t.Context(), "ISLAND CHAT_1")
	if err != nil || got != room {
		t.Fatalf("room restart: %+v %v", got, err)
	}
	favorites, err := s.FavoriteRooms(t.Context(), "BOB")
	if err != nil || !reflect.DeepEqual(favorites, []Room{room}) {
		t.Fatalf("favorites restart: %+v %v", favorites, err)
	}
	if err := s.RemoveRoomFavorite(t.Context(), "bob", "island chat_1"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveRoomFavorite(t.Context(), "bob", room.Name); err != nil {
		t.Fatalf("remove replay: %v", err)
	}
	favorites, err = s.FavoriteRooms(t.Context(), "bob")
	if err != nil || len(favorites) != 0 {
		t.Fatalf("own removal failed: %+v %v", favorites, err)
	}
	favorites, err = s.FavoriteRooms(t.Context(), "alice")
	if err != nil || !reflect.DeepEqual(favorites, []Room{room}) {
		t.Fatalf("removed someone else's favorite: %+v %v", favorites, err)
	}
	if _, err := s.Room(t.Context(), room.Name); err != nil {
		t.Fatalf("removing favorite deleted room: %v", err)
	}
}

func TestRoomCreationValidationAndRoleRollback(t *testing.T) {
	s := testStore(t)
	for _, name := range []string{"", "   ", strings.Repeat("a", 16), strings.Repeat("a", 41), "room;name", "room&name", "room+name", "room%20name", "room/name", "room\"name", "世界", "room\nname", "room\x00name"} {
		if _, err := s.CreateRoom(t.Context(), "alice", name, ""); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid room name %q accepted: %v", name, err)
		}
	}
	for _, description := range []string{strings.Repeat("x", 1001), "bad\x00description", string([]byte{0xff})} {
		if _, err := s.CreateRoom(t.Context(), "alice", "Valid Name", description); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid description accepted: %v", err)
		}
	}
	if _, err := s.CreateRoom(t.Context(), "", "Valid Name", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing owner: %v", err)
	}
	if _, err := s.CreateRoom(t.Context(), "alice", strings.Repeat("A", 15), strings.Repeat("界", 333)+"x"); err != nil {
		t.Fatalf("maximum valid lengths: %v", err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_room_role BEFORE INSERT ON room_moderators
WHEN NEW.room = 'Rollback' BEGIN SELECT RAISE(ABORT, 'role insertion failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRoom(t.Context(), "alice", "Rollback", ""); err == nil {
		t.Fatal("failed role insertion should fail create")
	}
	if _, err := s.Room(t.Context(), "Rollback"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("room survived failed role insertion: %v", err)
	}
	if _, err := s.AddRoomFavorite(t.Context(), "alice", "Missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("favorite missing room: %v", err)
	}
	if err := s.RemoveRoomFavorite(t.Context(), "alice", "Missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("remove missing room: %v", err)
	}
	if _, err := s.FavoriteRooms(t.Context(), ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("favorites missing owner: %v", err)
	}
	if _, err := s.AddRoomFavorite(t.Context(), "", "Lobby"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("add missing owner: %v", err)
	}
	if err := s.RemoveRoomFavorite(t.Context(), "", "Lobby"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("remove missing owner: %v", err)
	}
}

func TestRoomSearchIsLiteralCaseInsensitiveAndBounded(t *testing.T) {
	s := testStore(t)
	for _, room := range []struct{ name, description string }{
		{"Blue Sky", "Ocean & beach"},
		{"Percent", "100% ready"},
		{"Under_score", "Words"},
		{"Unicode", "世界"},
		{"Slash", `a\b`},
	} {
		if _, err := s.CreateRoom(t.Context(), "alice", room.name, room.description); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		query string
		want  []string
	}{
		{"bLuE", []string{"Blue Sky"}},
		{" BEACH ", []string{"Blue Sky"}},
		{"%", []string{"Percent"}},
		{"_", []string{"Under_score"}},
		{"\\", []string{"Slash"}},
		{"世界", []string{"Unicode"}},
		{"' OR 1=1 --", []string{}},
		{"missing", []string{}},
	} {
		rooms, err := s.SearchRooms(t.Context(), test.query, 50)
		names := []string{}
		for _, room := range rooms {
			names = append(names, room.Name)
		}
		if err != nil || !reflect.DeepEqual(names, test.want) {
			t.Fatalf("search %q: %v %v, want %v", test.query, names, err, test.want)
		}
	}
	rooms, err := s.SearchRooms(t.Context(), "", 2)
	if err != nil || len(rooms) != 2 || rooms[0].Name != "Blue Sky" || rooms[1].Name != "Gaming" {
		t.Fatalf("bounded sorted all rooms: %+v %v", rooms, err)
	}
	for _, query := range []string{strings.Repeat("a", 129), "x\x00", string([]byte{0xff})} {
		if _, err := s.SearchRooms(t.Context(), query, 1); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid search %q: %v", query, err)
		}
	}
}

func TestRoomOwnershipLimitAndDuplicateCreationAreAtomic(t *testing.T) {
	s := testStore(t)
	for i := 0; i < 9; i++ {
		if _, err := s.CreateRoom(t.Context(), "alice", fmt.Sprintf("Owned%d", i), ""); err != nil {
			t.Fatal(err)
		}
	}
	errorsCh := make(chan error, 2)
	var wg sync.WaitGroup
	for _, name := range []string{"LastOne", "LastTwo"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			_, err := s.CreateRoom(t.Context(), "alice", name, "")
			errorsCh <- err
		}(name)
	}
	wg.Wait()
	close(errorsCh)
	success, limit := 0, 0
	for err := range errorsCh {
		if err == nil {
			success++
		} else if errors.Is(err, ErrLimit) {
			limit++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || limit != 1 {
		t.Fatalf("concurrent quota success=%d limit=%d", success, limit)
	}
	if _, err := s.CreateRoom(t.Context(), "alice", "owned0", ""); err != nil {
		t.Fatalf("idempotent create at limit: %v", err)
	}
	errorsCh = make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.CreateRoom(t.Context(), "bob", "One Room", "Same contents")
			errorsCh <- err
		}()
	}
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("concurrent duplicate: %v", err)
		}
	}
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM chat_rooms WHERE owner = 'bob'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate room count=%d err=%v", count, err)
	}
}

func TestRoomFavoriteLimitIsAtomicAndSearchLimitIsCapped(t *testing.T) {
	s := testStore(t)
	for i := 0; i < 101; i++ {
		name := fmt.Sprintf("Room%03d", i)
		if _, err := s.CreateRoom(t.Context(), fmt.Sprintf("owner%d", i/10), name, ""); err != nil {
			t.Fatal(err)
		}
		if i < 99 {
			if _, err := s.AddRoomFavorite(t.Context(), "alice", name); err != nil {
				t.Fatal(err)
			}
		}
	}
	errorsCh := make(chan error, 2)
	var wg sync.WaitGroup
	for _, name := range []string{"Room099", "Room100"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			_, err := s.AddRoomFavorite(t.Context(), "alice", name)
			errorsCh <- err
		}(name)
	}
	wg.Wait()
	close(errorsCh)
	success, limit := 0, 0
	for err := range errorsCh {
		if err == nil {
			success++
		} else if errors.Is(err, ErrLimit) {
			limit++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || limit != 1 {
		t.Fatalf("concurrent favorites success=%d limit=%d", success, limit)
	}
	if _, err := s.AddRoomFavorite(t.Context(), "alice", "room000"); err != nil {
		t.Fatalf("idempotent favorite at limit: %v", err)
	}
	favorites, err := s.FavoriteRooms(t.Context(), "alice")
	if err != nil || len(favorites) != 100 {
		t.Fatalf("favorite limit: %d %v", len(favorites), err)
	}
	if _, err := s.AddRoomFavorite(t.Context(), "bob", "Room100"); err != nil {
		t.Fatalf("another owner's independent limit: %v", err)
	}
	for _, test := range []struct{ limit, want int }{{0, 50}, {1000, 100}} {
		rooms, err := s.SearchRooms(t.Context(), "Room", test.limit)
		if err != nil || len(rooms) != test.want {
			t.Fatalf("search limit=%d got=%d err=%v", test.limit, len(rooms), err)
		}
	}
}

func createV6DiscoveryFixture(t *testing.T, path string) {
	t.Helper()
	createV1Fixture(t, path)
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := migrateV2(t.Context(), tx); err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{schemaV3, schemaV4, schemaV5, schemaV6} {
		if _, err := tx.Exec(schema); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec("INSERT INTO chat_rooms(name,category,capacity,description) VALUES ('Old Room',1,30,'Original definition')"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestRoomDiscoveryMigrationPreservesV6DataAndRollsBackFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v6.sqlite")
	createV6DiscoveryFixture(t, path)
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE room_favorites(conflict INTEGER)"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := Open(t.Context(), path); err == nil {
		s.Close()
		t.Fatal("conflicting migration should fail")
	}
	db, err = sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 6 {
		t.Fatalf("failed migration advanced version: %d %v", version, err)
	}
	if _, err := db.Exec("SELECT owner FROM chat_rooms"); err == nil {
		t.Fatal("failed migration retained added owner column")
	}
	if _, err := db.Exec("DROP TABLE room_favorites"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 18 {
		t.Fatalf("migration version: %d %v", version, err)
	}
	old, err := s.Room(t.Context(), "old room")
	if err != nil || old.Capacity != 30 || old.Description != "Original definition" || old.Owner != "" {
		t.Fatalf("old room changed: %+v %v", old, err)
	}
	assertIDs(t, history(t, s, "alice", 1, "bob"), 7)
	assertIDs(t, pending(t, s, "alice"), 10)
	if _, err := s.AddRoomFavorite(t.Context(), "alice", "OLD ROOM"); err != nil {
		t.Fatalf("favorite migrated room: %v", err)
	}
	if _, err := s.db.Exec("INSERT INTO room_favorites(owner,room) VALUES ('bob','Missing')"); err == nil {
		t.Fatal("favorites foreign key was not enforced")
	}
}
