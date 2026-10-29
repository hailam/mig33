package realtime

import (
	"database/sql"
	"errors"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	chatstore "github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/protocol"
)

const (
	roomCreateURL = "http://127.0.0.1:8080/sites/index.php?c=chatroom&a=create&v=midlet"
	roomSearchURL = "http://127.0.0.1:8080/sites/index.php?c=chatroom&a=search&v=midlet"
)

func TestRoomCreationUsesHistoricalNameLimit(t *testing.T) {
	s := newTestServer(t)
	id := login(t, s, "alice")
	w := fetchReport(t, s, id, "GET", roomCreateURL, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `maxlength="15"`) || !strings.Contains(w.Body.String(), "1 to 15") {
		t.Fatal("creation form does not describe the recovered limit", w.Code, w.Body.String())
	}
	f := roomHiddenFields(w.Body.String())
	f.Set("name", strings.Repeat("A", 16))
	w = fetchReport(t, s, id, "POST", roomCreateURL+"&stage=name", f.Encode())
	if w.Code != 400 {
		t.Fatal("16-character new room accepted", w.Code)
	}
	if _, err := s.store.Room(t.Context(), f.Get("name")); !errors.Is(err, chatstore.ErrNotFound) {
		t.Fatal("rejected room was stored", err)
	}
	name := strings.Repeat("A", 15)
	f = prepareRoomCreation(t, s, id, name)
	f.Set("description", "At the recovered name limit")
	w = fetchReport(t, s, id, "POST", roomCreateURL+"&stage=details", f.Encode())
	if w.Code != 200 {
		t.Fatal("15-character new room rejected", w.Code, w.Body.String())
	}
	if room, err := s.store.Room(t.Context(), name); err != nil || room.Name != name {
		t.Fatal("maximum-length room was not stored", room, err)
	}
}

func TestExistingLongRoomKeepsLookupEditingAndNativeJoining(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing-rooms.sqlite")
	s, err := newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	// This definition represents a room created under the earlier 40-character
	// policy. The new creation rule must not rewrite or strand existing data.
	name := strings.Repeat("Legacy", 6) + "Room"
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO chat_rooms(name,category,capacity,description,owner) VALUES(?,1,50,?,?)", name, "Old description", "alice")
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	s, err = newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if normalized, err := chatstore.NormalizeRoomName(name); err != nil || normalized != name {
		t.Fatal("existing-name compatibility was narrowed", normalized, err)
	}
	if room, err := s.store.Room(t.Context(), strings.ToLower(name)); err != nil || room.Name != name || room.Owner != "alice" {
		t.Fatal("stored room lookup changed", room, err)
	}
	if _, err := s.store.CreateRoom(t.Context(), "alice", name, "Old description"); err != nil {
		t.Fatal("identical existing-room retry failed", err)
	}
	alice, bob := login(t, s, "alice"), login(t, s, "bob")
	info := "http://127.0.0.1:8080/sites/index.php?c=chatroom&a=setup&v=midlet" + "&roomName=" + url.QueryEscape(name)
	w := fetchReport(t, s, bob, "GET", info, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "mig33:joinChatroom("+name+")") {
		t.Fatal("existing room lost its native join link", w.Code, w.Body.String())
	}
	f := serviceTestForm(s, alice, "description", name, "Updated legacy room")
	w = fetchReport(t, s, alice, "POST", info+"&manage=1", f.Encode())
	if w.Code != 200 {
		t.Fatal("existing room cannot be edited", w.Code, w.Body.String())
	}

	joinRoom(t, s, bob, name)
	if !s.sessions[bob].rooms[name] {
		t.Fatal("native join did not retain the canonical existing name")
	}
	if room, err := s.store.Room(t.Context(), name); err != nil || room.Name != name || room.Description != "Updated legacy room" {
		t.Fatal("legacy room was renamed or edit was lost", room, err)
	}
}

func roomHiddenFields(body string) url.Values {
	fields := url.Values{}
	for _, match := range regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`).FindAllStringSubmatch(body, -1) {
		fields.Set(match[1], match[2])
	}
	return fields
}

func prepareRoomCreation(t *testing.T, s *Application, sid, name string) url.Values {
	t.Helper()
	w := fetchReport(t, s, sid, "GET", roomCreateURL, "")
	if w.Code != 200 {
		t.Fatalf("create form: %d %s", w.Code, w.Body.String())
	}
	form := roomHiddenFields(w.Body.String())
	form.Set("name", name)
	w = fetchReport(t, s, sid, "POST", roomCreateURL+"&stage=name", form.Encode())
	if w.Code != 200 || !strings.Contains(w.Body.String(), `name="description"`) {
		t.Fatalf("details form: %d %s", w.Code, w.Body.String())
	}
	return roomHiddenFields(w.Body.String())
}

func favoriteList(t *testing.T, s *Application, sid string) []*protocol.Packet {
	t.Helper()
	r, _ := s.Handle(t.Context(), sid, protocol.New(716, 40).SetShort(1, 2).SetByte(2, 1))
	if len(r) == 0 || r[len(r)-1].Opcode != 717 {
		t.Fatalf("favorites missing completion: %v", r)
	}
	for _, p := range r[:len(r)-1] {
		if category, _ := p.Short(6); p.Opcode != 701 || category != 2 || p.Transaction != 40 {
			t.Fatalf("favorite row: %+v", p)
		}
	}
	return r[:len(r)-1]
}

func TestRoomFavoritesNativeContractIsolationAndIdempotence(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	b := login(t, s, "bob")
	r, _ := s.Handle(t.Context(), h, protocol.New(713, 20))
	if len(r) != 3 {
		t.Fatal("missing Favorites category")
	}
	id, _ := r[1].Short(1)
	favorite, _ := r[1].Byte(5)
	clear, _ := r[1].Byte(3)
	if id != 2 || favorite != 1 || clear != 1 {
		t.Fatalf("wrong native favorite category flags: %+v", r[1])
	}
	if len(favoriteList(t, s, h)) != 0 {
		t.Fatal("new account has unsolicited favorites")
	}

	r, _ = s.Handle(t.Context(), h, protocol.New(711, 21).SetText(1, "lObBy"))
	if len(r) != 1 || r[0].Opcode != 701 || r[0].Transaction != 21 {
		t.Fatal("first add must complete711 with701")
	}
	if name, _ := r[0].Text(1); name != "Lobby" {
		t.Fatal("favorite is not canonical")
	}
	if category, _ := r[0].Short(6); category != 2 {
		t.Fatal("favorite inserted in global category")
	}

	r, _ = s.Handle(t.Context(), h, protocol.New(711, 22).SetText(1, "LOBBY"))
	if len(r) != 1 || r[0].Opcode != 1 || r[0].Transaction != 22 {
		t.Fatal("duplicate add would duplicate the original UI row")
	}
	if len(favoriteList(t, s, h)) != 1 || len(favoriteList(t, s, b)) != 0 {
		t.Fatal("favorites not idempotent and per-user")
	}
	s.mu.Lock()
	joined := s.sessions[h].rooms["Lobby"]
	s.mu.Unlock()
	if joined {
		t.Fatal("adding favorite joined the room")
	}
	for _, p := range []*protocol.Packet{
		protocol.New(711, 23), protocol.New(711, 23).SetText(1, "absent"),
		protocol.New(712, 23).SetShort(1, 1).SetText(2, "Lobby"),
		protocol.New(712, 23).SetShort(1, 2),
	} {
		r, _ = s.Handle(t.Context(), h, p)
		if len(r) != 1 || r[0].Opcode != 0 {
			t.Fatalf("invalid favorite operation accepted: %+v", p)
		}
	}
	newHailam := loginHailam(t, s)
	r, _ = s.Handle(t.Context(), newHailam, protocol.New(711, 26).SetText(1, "Lobby"))
	if len(r) != 1 || r[0].Opcode != 701 {
		t.Fatal("other session never received its favorite row")
	}

	r, _ = s.Handle(t.Context(), newHailam, protocol.New(711, 27).SetText(1, "Lobby"))
	if len(r) != 1 || r[0].Opcode != 1 {
		t.Fatal("other session repeated add duplicated its row")
	}
	if len(favoriteList(t, s, newHailam)) != 1 {
		t.Fatal("fresh session lost favorites")
	}

	joinRoom(t, s, h, "Lobby")
	for i := 0; i < 2; i++ {
		r, _ = s.Handle(t.Context(), newHailam, protocol.New(712, 24).SetShort(1, 2).SetText(2, "lobby"))
		if len(r) != 1 || r[0].Opcode != 1 {
			t.Fatal("remove favorite must be idempotent ACK")
		}
	}
	if len(favoriteList(t, s, h)) != 0 {
		t.Fatal("other session refresh kept removed favorite")
	}
	s.mu.Lock()
	joined = s.sessions[h].rooms["Lobby"]
	s.mu.Unlock()
	if !joined {
		t.Fatal("removing favorite left room")
	}
	if _, err := s.store.Room(t.Context(), "Lobby"); err != nil {
		t.Fatal("removing favorite deleted global room")
	}
}

func TestRoomPagesCreateInfoSearchAndOwnerAuthority(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	b := login(t, s, "bob")
	if w := fetchReport(t, s, "", "GET", roomCreateURL, ""); w.Code != 401 {
		t.Fatal("anonymous room creation form allowed")
	}
	form := prepareRoomCreation(t, s, h, "Island Chat")
	if _, err := s.store.Room(t.Context(), "Island Chat"); !errors.Is(err, chatstore.ErrNotFound) {
		t.Fatal("name step created room prematurely")
	}
	form.Set("description", "Island friends & games + 50% fun <welcome> ✓")
	w := fetchReport(t, s, h, "POST", roomCreateURL+"&stage=details", form.Encode())
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Room Created") || !strings.Contains(w.Body.String(), `mig33:joinChatroom(Island Chat)`) {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	room, err := s.store.Room(t.Context(), "island chat")
	if err != nil || room.Owner != "hailam" || room.Capacity != 50 || room.Description != form.Get("description") {
		t.Fatalf("created room: %+v %v", room, err)
	}
	if moderator, err := s.store.IsRoomModerator(t.Context(), room.Name, "hailam"); err != nil || !moderator {
		t.Fatal("creator lacks moderator role")
	}
	if len(favoriteList(t, s, h)) != 0 {
		t.Fatal("create implicitly saved favorite")
	}

	joinRoom(t, s, h, room.Name)
	joinRoom(t, s, b, room.Name)
	r, _ := s.Handle(t.Context(), h, protocol.New(706, 25).SetText(1, room.Name).SetText(2, "bob"))
	if r[0].Opcode != 1 {
		t.Fatal("creator cannot use moderator kick")
	}
	s.mu.Lock()
	present := s.sessions[b].rooms[room.Name]
	s.mu.Unlock()
	if present {
		t.Fatal("creator kick incorrectly became a vote")
	}
	info := "http://127.0.0.1:8080/sites/index.php?c=chatroom&a=setup&v=midlet&roomName=Island+Chat"
	w = fetchReport(t, s, b, "GET", info, "") // Viewing does not require joining.
	if w.Code != 200 || !strings.Contains(w.Body.String(), "hailam") || !strings.Contains(w.Body.String(), "&lt;welcome&gt;") || !strings.Contains(w.Body.String(), "1 / 50") {
		t.Fatalf("room info: %d %s", w.Code, w.Body.String())
	}
	w = fetchReport(t, s, b, "POST", roomSearchURL, url.Values{"query": {"FRIENDS & games"}}.Encode())
	if w.Code != 200 || !strings.Contains(w.Body.String(), `mig33:joinChatroom(Island Chat)`) || !strings.Contains(w.Body.String(), "roomName=Island+Chat") {
		t.Fatalf("room search: %d %s", w.Code, w.Body.String())
	}
	if w = fetchReport(t, s, h, "POST", roomCreateURL+"&stage=details", form.Encode()); w.Code != 200 {
		t.Fatal("identical creation replay failed")
	}
	form.Set("description", "different")
	if w = fetchReport(t, s, h, "POST", roomCreateURL+"&stage=details", form.Encode()); w.Code != 409 {
		t.Fatal("changed replay altered existing room")
	}
	other := prepareRoomCreation(t, s, b, "island chat")
	other.Set("description", "different")
	if w = fetchReport(t, s, b, "POST", roomCreateURL+"&stage=details", other.Encode()); w.Code != 409 {
		t.Fatal("another account claimed existing room")
	}
}

func TestRoomPageTokensAndInvalidFields(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	b := login(t, s, "bob")
	form := prepareRoomCreation(t, s, h, "Protected Room")
	form.Set("description", "safe")
	if w := fetchReport(t, s, b, "POST", roomCreateURL+"&stage=details", form.Encode()); w.Code != 403 {
		t.Fatal("cross-session creation token accepted")
	}
	for _, change := range []func(url.Values){
		func(f url.Values) { f.Set("room", "4f74686572") }, // Valid hex, altered signed name.
		func(f url.Values) { f.Set("binding", "invalid") },
		func(f url.Values) { f.Set("owner", "alice") },
		func(f url.Values) { f.Add("description", "second") },
		func(f url.Values) { f.Set("description", strings.Repeat("x", 1001)) },
	} {
		copy, _ := url.ParseQuery(form.Encode())
		change(copy)
		w := fetchReport(t, s, h, "POST", roomCreateURL+"&stage=details", copy.Encode())
		if w.Code < 400 {
			t.Fatalf("invalid creation accepted: %+v", copy)
		}
	}
	if _, err := s.store.Room(t.Context(), "Protected Room"); !errors.Is(err, chatstore.ErrNotFound) {
		t.Fatal("failed form created room")
	}
	for _, address := range []string{roomSearchURL, roomCreateURL} {
		r := httptest.NewRequest("POST", address, strings.NewReader("query=test"))
		r.Header.Set("sid", h)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 415 {
			t.Fatal("untyped form accepted")
		}
	}
	info := "http://127.0.0.1:8080/sites/index.php?c=chatroom&a=setup&v=midlet&roomName=Lobby"
	if w := fetchReport(t, s, h, "POST", info, ""); w.Code != 403 {
		t.Fatal("room info accepts unauthorized mutation")
	}
	if w := fetchReport(t, s, h, "GET", roomCreateURL+"&a=search", ""); w.Code != 404 {
		t.Fatal("ambiguous page selectors accepted")
	}
}

func TestNativeRoomFormsPreserveFinalTextAndRejectNameInjection(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	get, _ := s.Handle(t.Context(), h, protocol.New(922, 50).SetByte(1, 1).SetText(2, roomCreateURL))
	body, _ := get[0].Text(2)
	csrf := roomHiddenFields(body).Get("csrf")
	post := func(address, body string) *protocol.Packet {
		r, _ := s.Handle(t.Context(), h, protocol.New(922, 51).SetByte(1, 2).SetText(2, address).SetText(3, body))
		if len(r) != 1 || r[0].Opcode != 923 || r[0].Transaction != 51 {
			t.Fatal("native page response lost correlation")
		}
		return r[0]
	}
	bad := post(roomCreateURL+"&stage=name", "csrf="+csrf+"&name=Wrong&description=Injected")
	if status, _ := bad.Int(1); status != 400 {
		t.Fatal("raw name delimiter was interpreted as a second field")
	}
	first := post(roomCreateURL+"&stage=name", "csrf="+csrf+"&name=Native%20Room")
	if status, _ := first.Int(1); status != 200 {
		t.Fatal("native name step failed")
	}
	body, _ = first.Text(2)
	fields := roomHiddenFields(body)
	description := "Friends & fun + 50% = yes ✓ &csrf=ordinary text"
	second := post(roomCreateURL+"&stage=details",
		"csrf="+csrf+"&room="+fields.Get("room")+"&binding="+fields.Get("binding")+
			"&description="+strings.ReplaceAll(description, " ", "%20"))
	if status, _ := second.Int(1); status != 200 {
		t.Fatalf("native creation: %+v", second)
	}
	room, err := s.store.Room(t.Context(), "Native Room")
	if err != nil || room.Description != description {
		t.Fatalf("native description corrupted: %+v %v", room, err)
	}
	search := post(roomSearchURL, "query=+%2050%")
	body, _ = search.Text(2)
	if status, _ := search.Int(1); status != 200 || !strings.Contains(body, `mig33:joinChatroom(Native Room)`) {
		t.Fatalf("native search text corrupted: %d %s", status, body)
	}
	search = post(roomSearchURL, "query=%")
	body, _ = search.Text(2)
	if !strings.Contains(body, "Native Room") || strings.Contains(body, "mig33:joinChatroom(Lobby)") {
		t.Fatal("search interpreted percent as SQL wildcard")
	}
	search = post(roomSearchURL, "")
	body, _ = search.Text(2)
	if status, _ := search.Int(1); status != 200 || !strings.Contains(body, "mig33:joinChatroom(Lobby)") {
		t.Fatal("empty native search should list rooms")
	}
	first = post(roomCreateURL+"&stage=name", "csrf="+csrf+"&name=Blank%20Desc")
	if status, _ := first.Int(1); status != 200 {
		t.Fatal("blank-description fixture name was rejected")
	}
	body, _ = first.Text(2)
	fields = roomHiddenFields(body)
	second = post(roomCreateURL+"&stage=details", "csrf="+csrf+"&room="+fields.Get("room")+"&binding="+fields.Get("binding"))
	if status, _ := second.Int(1); status != 200 {
		t.Fatal("omitted optional native textarea was rejected")
	}
	room, err = s.store.Room(t.Context(), "Blank Desc")
	if err != nil || room.Description != "" {
		t.Fatal("omitted description not stored empty")
	}
}
