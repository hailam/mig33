package realtime

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func actionPackets(packets []*protocol.Packet) []*protocol.Packet {
	var actions []*protocol.Packet
	for _, p := range packets {
		if content, _ := p.Short(6); p.Opcode == 500 && content == 6 {
			actions = append(actions, p)
		}
	}
	return actions
}

func oneAction(t *testing.T, packets []*protocol.Packet, peer, expected string) *protocol.Packet {
	t.Helper()
	actions := actionPackets(packets)
	if len(actions) != 1 {
		t.Fatalf("action count %d, want one: %v", len(actions), packets)
	}
	p := actions[0]
	if sender, _ := p.Text(2); sender != peer {
		t.Fatalf("packet peer/actor=%q want %q", sender, peer)
	}
	if body, _ := p.Text(8); body != expected {
		t.Fatalf("body=%q want %q", body, expected)
	}
	if _, present := p.Int(13); present {
		t.Fatal("explicit color would bypass native action formatting")
	}
	return p
}

func TestSlashCoreCommandsEchoGeneratedActionsToPrivatePeers(t *testing.T) {
	s := newTestServer(t)
	a, b := login(t, s, "alice"), login(t, s, "bob")
	for _, command := range []string{" /Me  waves hello ✓  ", "/hug BOB", "/slap", "/wave alice", "/smile", "/laugh bob", "/cry bob", "/brb", "/roll", "/fortune", "/8ball"} {
		r, _ := s.Handle(t.Context(), a, privateMessage("bob", command).SetInt(13, 0xff0000))
		if r[0].Opcode != 1 {
			t.Fatalf("rejected %q: %+v", command, r)
		}
		history, err := s.store.History(t.Context(), "alice", 1, "bob", 0, 100)
		if err != nil || len(history) == 0 {
			t.Fatal("missing action history")
		}
		m := history[len(history)-1]
		if !m.Action || m.Sender != "alice" || m.Color != nil || !strings.HasPrefix(m.Body, "alice ") || strings.Contains(m.Body, command) {
			t.Fatalf("untrusted or unexpanded action: %+v", m)
		}
		if command == "/roll" {
			roll, err := strconv.Atoi(strings.TrimPrefix(m.Body, "alice rolls "))
			if err != nil || roll < 1 || roll > 100 {
				t.Fatalf("invalid local roll range: %q", m.Body)
			}
		}
		oneAction(t, requestPacket(t, s, a, nil), "bob", m.Body)
		oneAction(t, requestPacket(t, s, b, nil), "alice", m.Body)
		if len(actionPackets(requestPacket(t, s, a, nil))) != 0 {
			t.Fatal("successful echo was delivered twice")
		}
	}
	s.Handle(t.Context(), a, privateMessage("bob", "ordinary text"))
	if containsText(requestPacket(t, s, a, nil), "ordinary text") || !containsText(requestPacket(t, s, b, nil), "ordinary text") {
		t.Fatal("ordinary text local-echo contract changed")
	}
}

func TestSlashErrorsNeverPersistOrBroadcastAndCannotForgeContentSix(t *testing.T) {
	s := newTestServer(t)
	a, b := login(t, s, "alice"), login(t, s, "bob")
	for _, body := range []string{
		"/",
		"/unknown",
		"/me",
		"/roll 6",
		"/8ball question",
		"/fortune bob",
		"/hug absent",
		"/hug carol",
		"/wave bob carol",
		"/kick bob",
		"/mute bob",
		"/unmute bob",
		"/silence 30",
		"/ban bob reason",
		"/invite carol",
		"/botstop ! 120",
		"/me " + strings.Repeat("x", 65532),
	} {
		r, _ := s.Handle(t.Context(), a, privateMessage("bob", body))
		if len(r) != 1 || r[0].Opcode != 0 || r[0].Transaction != 50 {
			t.Fatalf("command not rejected with correlated error: %q %+v", body, r)
		}
	}
	r, _ := s.Handle(t.Context(), a, privateMessage("bob", "bob waves").SetShort(6, 6))
	if r[0].Opcode != 0 {
		t.Fatal("client forged a generated action")
	}
	history, _ := s.store.History(t.Context(), "alice", 1, "bob", 0, 100)
	if len(history) != 0 || len(actionPackets(requestPacket(t, s, a, nil))) != 0 || len(actionPackets(requestPacket(t, s, b, nil))) != 0 {
		t.Fatal("rejected command left message state")
	}
}

func TestSlashOwnDMResultUsesActorForBlockChecks(t *testing.T) {
	for _, blockBefore := range []bool{true, false} {
		s := newTestServer(t)
		a, b := login(t, s, "alice"), login(t, s, "bob")
		block := func() { s.Handle(t.Context(), a, protocol.New(417, 60).SetText(1, "bob").SetByte(2, 2)) }
		if blockBefore {
			block()
		}
		s.Handle(t.Context(), a, privateMessage("bob", "/wave bob"))
		s.Handle(t.Context(), b, privateMessage("alice", "/wave alice"))
		if !blockBefore {
			block() // Cancel Bob's buffered action, but retain Alice's own echo.
		}
		oneAction(t, requestPacket(t, s, a, nil), "bob", "alice waves to bob.")
		// Bob's block prevents new receipt, but does not suppress Alice's own result.
		s.Handle(t.Context(), b, protocol.New(417, 61).SetText(1, "alice").SetByte(2, 2))
		requestPacket(t, s, b, nil)
		s.Handle(t.Context(), a, privateMessage("bob", "/smile"))
		oneAction(t, requestPacket(t, s, a, nil), "bob", "alice smiles.")
		if len(actionPackets(requestPacket(t, s, b, nil))) != 0 {
			t.Fatal("blocked recipient received action")
		}
	}
}

func TestSlashGroupAndRoomMembershipMuteAndBackpressure(t *testing.T) {
	s := newTestServer(t)
	a, b, c, d := login(t, s, "alice"), login(t, s, "bob"), login(t, s, "carol"), login(t, s, "dave")
	group, _ := createTestGroup(t, s, a)
	for _, sid := range []string{a, b, c, d} {
		requestPacket(t, s, sid, nil)
	}
	command := func(sid, target, text string, kind byte) []*protocol.Packet {
		r, _ := s.Handle(t.Context(), sid, privateMessage(target, text).SetByte(3, kind))
		return r
	}
	if command(d, group, "/wave", 2)[0].Opcode != 0 || command(a, group, "/hug dave", 2)[0].Opcode != 0 {
		t.Fatal("group command bypassed membership/target checks")
	}
	if command(a, group, "/hug bob", 2)[0].Opcode != 1 {
		t.Fatal("group action rejected")
	}
	for _, sid := range []string{a, b, c} {
		oneAction(t, requestPacket(t, s, sid, nil), "alice", "alice hugs bob.")
	}
	if len(actionPackets(requestPacket(t, s, d, nil))) != 0 {
		t.Fatal("group action leaked")
	}
	joinRoom(t, s, a, "Lobby")
	joinRoom(t, s, b, "Lobby")
	for _, sid := range []string{a, b} {
		requestPacket(t, s, sid, nil)
	}
	if command(c, "Lobby", "/wave", 3)[0].Opcode != 0 || command(a, "Lobby", "/hug carol", 3)[0].Opcode != 0 {
		t.Fatal("room command bypassed membership/target checks")
	}
	s.Handle(t.Context(), a, protocol.New(709, 70).SetText(1, "Lobby").SetText(2, "bob"))
	if command(b, "Lobby", "/me speaks", 3)[0].Opcode != 0 {
		t.Fatal("muted user sent action")
	}
	if command(b, "alice", "/wave", 1)[0].Opcode != 1 {
		t.Fatal("room mute blocked DM action")
	}
	for _, sid := range []string{a, b} {
		requestPacket(t, s, sid, nil)
	}
	if command(a, "lobby", "/me welcomes everyone", 3)[0].Opcode != 1 {
		t.Fatal("room action rejected")
	}
	for _, sid := range []string{a, b} {
		oneAction(t, requestPacket(t, s, sid, nil), "alice", "alice welcomes everyone")
	}
	// A room result cannot be acknowledged if the invoking session cannot see it.
	otherAlice := login(t, s, "alice")
	joinRoom(t, s, otherAlice, "Lobby")
	requestPacket(t, s, otherAlice, nil)
	s.mu.Lock()
	for len(s.sessions[a].push) < cap(s.sessions[a].push) {
		s.sessions[a].push <- events.Ack(0)
	}
	s.mu.Unlock()
	if command(a, "Lobby", "/roll", 3)[0].Opcode != 0 {
		t.Fatal("full sender queue accepted room action")
	}
	history, _ := s.store.History(t.Context(), "alice", 3, "Lobby", 0, 100)
	if len(history) != 1 {
		t.Fatalf("rejected room action stored: %d", len(history))
	}
}

func TestSlashRandomResultSurvivesFailedWriteAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "actions.sqlite")
	s, err := newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	a := login(t, s, "alice")
	s.Handle(t.Context(), a, privateMessage("bob", "/roll"))
	pending, err := s.store.Pending(t.Context(), "alice", 10)
	if err != nil || len(pending) != 1 {
		t.Fatal("sender action is not durable")
	}
	result := pending[0]
	w := &brokenResponse{header: make(http.Header), failWrite: true}
	s.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/"+a, nil))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, b := login(t, s, "alice"), login(t, s, "bob")
	for _, who := range []string{"alice", "bob"} {
		pending, _ := s.store.Pending(t.Context(), who, 10)
		if len(pending) != 1 || pending[0].ID != result.ID || pending[0].Body != result.Body || !pending[0].Action {
			t.Fatal("action regenerated or lost on retry")
		}
	}
	oneAction(t, requestPacket(t, s, a, nil), "bob", result.Body)
	oneAction(t, requestPacket(t, s, b, nil), "alice", result.Body)
}
