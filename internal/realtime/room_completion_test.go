package realtime

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func linkedRoomFixture(t *testing.T, s *Application) {
	t.Helper()
	g, err := s.store.CreateCommunity(t.Context(), "alice", "Local Club", "", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Club One", "Club Two"} {
		if _, err = s.store.CreateRoom(t.Context(), "alice", name, ""); err != nil {
			t.Fatal(err)
		}
		if err = s.store.LinkCommunityRoom(t.Context(), g.ID, "alice", name, false); err != nil {
			t.Fatal(err)
		}
	}
}

func TestJoinedRoomExposesCommunityIDAndRejoinClearsUnlinkedGroup(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	linkedRoomFixture(t, s)
	controls, err := s.store.GroupRoomControls(t.Context(), "Club One")
	if err != nil || controls.GroupID <= 0 {
		t.Fatalf("linked fixture: %+v, %v", controls, err)
	}
	joined := joinTestRoom(t, s, a, "Club One", 61)
	if id, ok := joined[0].Int(8); !ok || int64(id) != controls.GroupID {
		t.Fatalf("joined701 lost Community Group ID: %d, present=%v", id, ok)
	}
	// Listing/favorite701 has a different parser branch. It must not create
	// a group-backed conversation or inherit the previously joined metadata.
	listed, _ := s.Handle(t.Context(), a, protocol.New(716, 62).SetShort(1, 1).SetByte(2, 1))
	for _, packet := range listed {
		if packet.Opcode == 701 {
			if _, ok := packet.Int(8); ok {
				t.Fatal("category listing included joined-room-only community metadata")
			}
		}
	}
	if err := s.store.LinkCommunityRoom(t.Context(), controls.GroupID, "alice", "Club One", true); err != nil {
		t.Fatal(err)
	}
	// An already-joined703 is the original authoritative metadata refresh.
	rejoined := joinTestRoom(t, s, a, "Club One", 63)
	if _, ok := rejoined[0].Int(8); ok {
		t.Fatal("unlinked room retained its old Group Menu ID")
	}
	ordinary := joinTestRoom(t, s, a, "Lobby", 64)
	if _, ok := ordinary[0].Int(8); ok {
		t.Fatal("ordinary room unexpectedly advertises a Community Group")
	}
}

func roomCommand(t *testing.T, s *Application, sid, room, body string, want int16) {
	t.Helper()
	r, _ := s.Handle(t.Context(), sid, protocol.New(500, 50).SetByte(1, 1).SetByte(3, 3).SetText(4, room).SetShort(6, 1).SetText(8, body))
	if len(r) != 1 || r[0].Opcode != want {
		msg, _ := r[0].Text(2)
		t.Fatalf("%q opcode=%v want %d: %s", body, r[0].Opcode, want, msg)
	}
}

func TestGroupRoomCommandsRolesLockSilenceBanAndBots(t *testing.T) {
	s := newTestServer(t)
	a, b, c := login(t, s, "alice"), login(t, s, "bob"), login(t, s, "carol")
	linkedRoomFixture(t, s)
	for _, sid := range []string{a, b, c} {
		joinRoom(t, s, sid, "Club One")
		joinRoom(t, s, sid, "Club Two")
		requestPacket(t, s, sid, nil)
	}
	roomCommand(t, s, b, "Club One", "/silence on", 0)
	roomCommand(t, s, a, "Club One", "/silence on", 1)
	roomCommand(t, s, b, "Club One", "ordinary text", 0)
	roomCommand(t, s, a, "Club One", "/announce hello", 1)
	roomCommand(t, s, a, "Club One", "/silence off", 1)
	roomCommand(t, s, b, "Club One", "ordinary text", 1)
	roomCommand(t, s, a, "Club One", "/lock", 1)
	d := login(t, s, "dave")
	r, _ := s.Handle(t.Context(), d, protocol.New(703, 55).SetText(1, "Club One"))
	if r[0].Opcode != 0 {
		t.Fatal("locked room admitted visitor")
	}
	roomCommand(t, s, a, "Club One", "/unlock", 1)
	joinRoom(t, s, d, "Club One")
	roomCommand(t, s, a, "Club One", "/ban bob", 1)
	if s.sessions[b].rooms["Club One"] || s.sessions[b].rooms["Club Two"] {
		t.Fatal("Group ban left target in a linked room")
	}

	r, _ = s.Handle(t.Context(), b, protocol.New(703, 55).SetText(1, "Club Two"))
	if r[0].Opcode != 0 {
		t.Fatal("Group ban allowed rejoin")
	}
	roomCommand(t, s, a, "Club One", "/unban bob", 1)
	joinRoom(t, s, b, "Club Two")
	roomCommand(t, s, a, "Club One", "/bot dice", 1)
	roomCommand(t, s, a, "Club One", "/bot 8ball", 0)
	for i := 0; i < 20; i++ {
		roomCommand(t, s, c, "Club One", "/play", 1)
		for _, sid := range []string{a, c, d} {
			requestPacket(t, s, sid, nil)
		}
	}
	roomCommand(t, s, c, "Club One", "/play", 0)
	scores, e := s.store.RoomBotScores(t.Context(), "Club One")
	if e != nil || len(scores) != 1 || scores[0].Rounds != 20 || scores[0].Points < 20 || scores[0].Points > 120 {
		t.Fatalf("dice scoreboard %+v %v", scores, e)
	}
	roomCommand(t, s, a, "Club One", "/botstop", 1)
	roomCommand(t, s, a, "Club One", "/bot 8ball", 1)
	roomCommand(t, s, c, "Club One", "/play Will it rain?", 1)
	roomCommand(t, s, a, "Club One", "/description A new description", 1)
	roomCommand(t, s, a, "Club One", "/invite bob", 1)
	mail, e := s.store.ServiceEntries(t.Context(), "mail", "", "bob")
	if e != nil || len(mail) != 1 || !strings.Contains(mail[0].Body, "Club One") {
		t.Fatalf("invitation mail %+v %v", mail, e)
	}
	for _, sid := range []string{a, b, c, d} {
		requestPacket(t, s, sid, nil)
	}
	roomCommand(t, s, a, "Club One", "/broadcast club announcement", 1)
	if !containsText(requestPacket(t, s, b, nil), "alice announces: club announcement") {
		t.Fatal("broadcast did not reach second linked room")
	}
	roomCommand(t, s, a, "Lobby", "/lock", 0)
}

func TestRoomControlBusyQueueHasNoMutation(t *testing.T) {
	s := newTestServer(t)
	a, b := login(t, s, "alice"), login(t, s, "bob")
	linkedRoomFixture(t, s)
	joinRoom(t, s, a, "Club One")
	joinRoom(t, s, b, "Club One")
	for len(s.sessions[b].push) < cap(s.sessions[b].push) {
		s.sessions[b].push <- events.New(0, events.Pong{})
	}
	roomCommand(t, s, a, "Club One", "/silence on", 0)
	c, e := s.store.GroupRoomControls(t.Context(), "Club One")
	if e != nil || c.Silenced {
		t.Fatal("failed command mutated state")
	}
	changes, e := s.store.RoomChanges(t.Context(), "Club One")
	if e != nil || len(changes) != 0 {
		t.Fatalf("failed command audited success %+v %v", changes, e)
	}
}

func TestRoomDeleteCategoriesSearchAndOwnSessionSync(t *testing.T) {
	s := newTestServer(t)
	a, a2, b := login(t, s, "alice"), login(t, s, "alice"), login(t, s, "bob")
	room, err := s.store.CreateRoom(t.Context(), "alice", "Delete Me", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, sid := range []string{a, a2, b} {
		joinRoom(t, s, sid, room.Name)
		requestPacket(t, s, sid, nil)
	}
	roomCommand(t, s, a, room.Name, "own other session", 1)
	if containsText(requestPacket(t, s, a, nil), "own other session") {
		t.Fatal("origin received duplicate local echo")
	}
	if !containsText(requestPacket(t, s, a2, nil), "own other session") {
		t.Fatal("other author session missed text")
	}
	if !containsText(requestPacket(t, s, b, nil), "own other session") {
		t.Fatal("peer missed text")
	}
	if err = s.store.ManageRoomCategory(t.Context(), "test-operator", 3, "Islands", false); err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetRoomCategory(t.Context(), "test-operator", room.Name, 3); err != nil {
		t.Fatal(err)
	}

	r, _ := s.Handle(t.Context(), a, protocol.New(713, 50))
	if len(r) != 4 {
		t.Fatalf("native categories %+v", r)
	}

	r, _ = s.Handle(t.Context(), a, protocol.New(716, 51).SetShort(1, 3).SetByte(2, 1))
	if len(r) != 2 || r[0].Opcode != 701 {
		t.Fatal("custom category room missing")
	}
	for op, v := range map[string]string{"language": "Dhivehi", "keywords": "reef & fish"} {
		if err = s.store.ManageRoom(t.Context(), room.Name, "alice", op, v); err != nil {
			t.Fatal(err)
		}
	}
	u := roomSearchURL + "&language=Dhivehi&keywords=" + url.QueryEscape("reef & fish")
	w := fetchReport(t, s, a, "GET", u, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Delete Me") {
		t.Fatalf("filter page %d %s", w.Code, w.Body.String())
	}
	// The original form only escapes spaces; the final query value retains &.
	r, _ = s.Handle(t.Context(), a, protocol.New(922, 52).SetByte(1, 2).SetText(2, roomSearchURL+"&field=keywords&language=Dhivehi").SetText(3, "query=reef%20&%20fish"))
	html, _ := r[0].Text(2)
	code, _ := r[0].Int(1)
	if code != 200 || !strings.Contains(html, "Delete Me") {
		t.Fatalf("native filter %d %s", code, html)
	}
	u = "http://127.0.0.1:8080/sites/index.php?c=chatroom&a=setup&v=midlet" + "&manage=1&roomName=" + url.QueryEscape(room.Name)
	post := func(sid, value string) int {
		f := serviceTestForm(s, sid, "delete", room.Name, value)
		return fetchReport(t, s, sid, "POST", u, f.Encode()).Code
	}
	if got := post(b, room.Name); got != 403 {
		t.Fatalf("nonowner deletion %d", got)
	}
	if got := post(a, "wrong"); got != 400 {
		t.Fatalf("unconfirmed deletion %d", got)
	}
	if got := post(a, room.Name); got != 200 {
		t.Fatalf("owner deletion %d", got)
	}
	for _, sid := range []string{a, a2, b} {
		if s.sessions[sid].rooms[room.Name] {
			t.Fatal("deleted room has live members")
		}
		r, _ := s.Handle(t.Context(), sid, protocol.New(703, 56).SetText(1, room.Name))
		if r[0].Opcode != 0 {
			t.Fatal("deleted room rejoined")
		}
	}
	changes, e := s.store.RoomChanges(t.Context(), room.Name)
	if e != nil || len(changes) == 0 || changes[0].Operation != "delete" {
		t.Fatalf("deletion audit %v %v", changes, e)
	}
}

func TestNativeModerationEventsAreAudited(t *testing.T) {
	s := newTestServer(t)
	a, b, c, d := login(t, s, "alice"), login(t, s, "bob"), login(t, s, "carol"), login(t, s, "dave")
	for _, sid := range []string{a, b, c, d} {
		joinRoom(t, s, sid, "Lobby")
	}
	for _, opcode := range []int16{709, 710, 706} {
		r, _ := s.Handle(t.Context(), a, protocol.New(opcode, 70).SetText(1, "Lobby").SetText(2, "bob"))
		if r[0].Opcode != 1 {
			t.Fatalf("native %d failed", opcode)
		}
	}

	joinRoom(t, s, b, "Lobby")
	for _, sid := range []string{b, c} {
		r, _ := s.Handle(t.Context(), sid, protocol.New(706, 71).SetText(1, "Lobby").SetText(2, "dave"))
		if r[0].Opcode != 1 {
			t.Fatal("vote failed")
		}
	}
	changes, e := s.store.RoomChanges(t.Context(), "Lobby")
	if e != nil {
		t.Fatal(e)
	}
	ops := map[string]bool{}
	for _, change := range changes {
		ops[change.Operation] = true
	}
	for _, op := range []string{"mute", "unmute", "kick", "vote-start", "vote-cast"} {
		if !ops[op] {
			t.Fatal(fmt.Sprintf("missing audit %s: %+v", op, changes))
		}
	}
}

func TestGroupRoomInvitationAndBotRespectPersonalBlocks(t *testing.T) {
	s := newTestServer(t)
	a, b, c := login(t, s, "alice"), login(t, s, "bob"), login(t, s, "carol")
	linkedRoomFixture(t, s)
	for _, sid := range []string{a, b, c} {
		joinRoom(t, s, sid, "Club One")
		requestPacket(t, s, sid, nil)
	}
	s.Handle(t.Context(), b, protocol.New(417, 90).SetText(1, "alice").SetByte(2, 2))
	roomCommand(t, s, a, "Club One", "/invite bob", 0)
	mail, e := s.store.ServiceEntries(t.Context(), "mail", "", "bob")
	if e != nil || len(mail) != 0 {
		t.Fatalf("blocked invitation persisted %+v %v", mail, e)
	}
	roomCommand(t, s, a, "Club One", "/bot 8ball", 1)
	for _, sid := range []string{a, b, c} {
		requestPacket(t, s, sid, nil)
	}
	roomCommand(t, s, c, "Club One", "/play A blocked question?", 1)
	s.Handle(t.Context(), b, protocol.New(417, 91).SetText(1, "carol").SetByte(2, 2))
	if len(actionPackets(requestPacket(t, s, b, nil))) != 0 {
		t.Fatal("queued game result bypassed new block")
	}
	actions := actionPackets(requestPacket(t, s, a, nil))
	if len(actions) != 1 {
		t.Fatal("unblocked game recipient missed result")
	}
	body, _ := actions[0].Text(8)
	if !strings.Contains(body, "A blocked question?") {
		t.Fatal("game result lost question")
	}
}
