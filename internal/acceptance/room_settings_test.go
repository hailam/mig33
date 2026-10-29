package acceptance

import (
	"net/url"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/protocol"
)

func TestRoomSettingsPermissionsVotesBanAndOwnership(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	b := login(t, s, "bob")
	c := login(t, s, "carol")
	room, err := s.store.CreateRoom(t.Context(), "alice", "Owner Test", "Original")
	if err != nil {
		t.Fatal(err)
	}
	for _, sid := range []string{a, b, c} {
		joinRoom(t, s, sid, room.Name)
	}
	u := s.roomPageAction("setup") + "&manage=1&roomName=" + url.QueryEscape(room.Name)
	post := func(sid, op, value string) int {
		f := serviceTestForm(s, sid, op, room.Name, value)
		return fetchReport(t, s, sid, "POST", u, f.Encode()).Code
	}
	if code := post(b, "description", "Hacked"); code != 403 {
		t.Fatalf("nonowner edit %d", code)
	}

	for _, change := range [][2]string{{"language", "Dhivehi"}, {"keywords", "Maldives & islands"}, {"description", "Island chat"}, {"capacity", "70"}, {"kicks", "0"}} {
		if code := post(a, change[0], change[1]); code != 200 {
			t.Fatalf("%s=%s: %d", change[0], change[1], code)
		}
	}
	r, err := s.store.Room(t.Context(), room.Name)
	if err != nil || r.Capacity != 70 || r.Description != "Island chat" {
		t.Fatalf("settings %+v %v", r, err)
	}
	settings, err := s.store.RoomSettings(t.Context(), room.Name)
	if err != nil || settings.UserKicks || settings.Language != "Dhivehi" || settings.Keywords != "Maldives & islands" {
		t.Fatalf("extra settings %+v %v", settings, err)
	}
	replies, _ := s.Handle(t.Context(), b, protocol.New(706, 123).SetText(1, room.Name).SetText(2, "carol"))
	if replies[0].Opcode != 0 {
		t.Fatal("disabled vote accepted")
	}
	if code := post(a, "capacity", "2"); code != 409 {
		t.Fatalf("capacity below occupancy %d", code)
	}
	if code := post(a, "ban", "bob"); code != 200 {
		t.Fatalf("ban %d", code)
	}
	if s.session(b).rooms[room.Name] {
		t.Fatal("banned user retained room membership")
	}
	replies, _ = s.Handle(t.Context(), b, protocol.New(703, 124).SetText(1, room.Name))
	if replies[0].Opcode != 0 {
		t.Fatal("banned user rejoined")
	}
	if code := post(a, "unban", "bob"); code != 200 {
		t.Fatalf("unban %d", code)
	}
	joinRoom(t, s, b, room.Name)
	if code := post(a, "moderator-add", "carol"); code != 200 {
		t.Fatalf("grant %d", code)
	}
	if code := post(a, "ban", "carol"); code != 403 {
		t.Fatalf("moderator ban %d", code)
	}
	if code := post(c, "keywords", "Unauthorized"); code != 403 {
		t.Fatalf("moderator owner edit %d", code)
	}
	if code := post(a, "owner", "bob"); code != 200 {
		t.Fatalf("ownership %d", code)
	}
	if code := post(a, "description", "Old owner"); code != 403 {
		t.Fatalf("former owner retained rights %d", code)
	}
	if code := post(b, "description", "New owner"); code != 200 {
		t.Fatalf("new owner edit %d", code)
	}
	changes, err := s.store.RoomChanges(t.Context(), room.Name)
	if err != nil || len(changes) != 10 {
		t.Fatalf("audit changes=%d %v", len(changes), err)
	}
}

func TestRoomSettingsNativeTextTailAndReadOnlyViews(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	room, err := s.store.CreateRoom(t.Context(), "alice", "Native Settings", "")
	if err != nil {
		t.Fatal(err)
	}
	u := s.roomPageAction("setup") + "&manage=1&roomName=" + url.QueryEscape(room.Name)
	f := serviceTestForm(s, a, "keywords", room.Name, "")
	f.Del("body")
	raw := f.Encode() + "&body=a&b=1+50%✓"
	r, _ := s.Handle(t.Context(), a, protocol.New(922, 91).SetByte(1, 2).SetText(2, u).SetText(3, raw))
	code, _ := r[0].Int(1)
	if code != 200 {
		t.Fatalf("native settings %d", code)
	}
	settings, e := s.store.RoomSettings(t.Context(), room.Name)
	if e != nil || settings.Keywords != "a&b=1+50%✓" {
		t.Fatalf("native punctuation %+v %v", settings, e)
	}
	w := fetchReport(t, s, a, "GET", strings.Replace(u, "&manage=1", "", 1), "")
	if !strings.Contains(w.Body.String(), "&manage=1") {
		t.Fatal("room settings unreachable")
	}
	if s.session(a).rooms[room.Name] {
		t.Fatal("viewing settings joined room")
	}
}
