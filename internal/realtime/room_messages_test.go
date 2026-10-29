package realtime

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func joinRoom(t *testing.T, s *Application, id, name string) {
	t.Helper()
	r, _ := s.Handle(t.Context(), id, protocol.New(703, 20).SetText(1, name))
	if len(r) < 2 || r[0].Opcode != 701 || r[1].Opcode != 1 {
		t.Fatalf("join: %v", r)
	}
}

func containsText(packets []*protocol.Packet, text string) bool {
	for _, p := range packets {
		if p.Opcode == 500 {
			if body, _ := p.Text(8); body == text {
				return true
			}
		}
	}
	return false
}

func TestRoomMessagesStayWithinJoinedSessions(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	bobOther := login(t, s, "bob")
	carol := login(t, s, "carol")
	joinRoom(t, s, alice, "lobby")
	joinRoom(t, s, bob, "Lobby")
	joinRoom(t, s, carol, "Gaming")
	r, _ := s.Handle(t.Context(), alice, privateMessage("LOBBY", "Room ✓ 😀").SetByte(3, 3).SetInt(13, 123))
	if r[0].Opcode != 1 {
		t.Fatal("room message rejected")
	}
	packets := requestPacket(t, s, bob, nil)
	if !containsText(packets, "Room ✓ 😀") {
		t.Fatal("joined recipient missed message")
	}
	for _, p := range packets {
		if body, _ := p.Text(8); body == "Room ✓ 😀" {
			if sender, _ := p.Text(2); sender != "alice" {
				t.Fatal("room sender spoofed")
			}
			if room, _ := p.Text(4); room != "Lobby" {
				t.Fatal("room canonicalization")
			}
			if color, _ := p.Int(13); color != 123 {
				t.Fatal("room color")
			}
		}
	}
	if containsText(requestPacket(t, s, bobOther, nil), "Room ✓ 😀") {
		t.Fatal("room message leaked to nonjoined session of same account")
	}
	if containsText(requestPacket(t, s, carol, nil), "Room ✓ 😀") {
		t.Fatal("room message leaked across rooms")
	}
	if containsText(requestPacket(t, s, alice, nil), "Room ✓ 😀") {
		t.Fatal("room outgoing echo duplicated")
	}
	r, _ = s.Handle(t.Context(), carol, privateMessage("Lobby", "outsider").SetByte(3, 3))
	if r[0].Opcode != 0 {
		t.Fatal("outsider sent room text")
	}
}

func TestRoomLeaveCancelsBufferedMessageEvenAfterRejoin(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	joinRoom(t, s, alice, "Lobby")
	joinRoom(t, s, bob, "Lobby")
	s.Handle(t.Context(), alice, privateMessage("Lobby", "before leaving").SetByte(3, 3))
	s.Handle(t.Context(), bob, protocol.New(704, 22).SetText(1, "Lobby"))
	joinRoom(t, s, bob, "Lobby")
	if containsText(requestPacket(t, s, bob, nil), "before leaving") {
		t.Fatal("old room message survived leave/rejoin")
	}
	s.Handle(t.Context(), alice, privateMessage("Lobby", "after rejoining").SetByte(3, 3))
	if !containsText(requestPacket(t, s, bob, nil), "after rejoining") {
		t.Fatal("rejoined member cannot receive")
	}
}

func TestRoomDisconnectDoesNotReplayOfflineTraffic(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	joinRoom(t, s, alice, "Lobby")
	joinRoom(t, s, bob, "Lobby")
	s.Handle(t.Context(), alice, privateMessage("Lobby", "before disconnect").SetByte(3, 3))
	s.DropSession(t.Context(), bob)
	s.Handle(t.Context(), alice, privateMessage("Lobby", "while absent").SetByte(3, 3))
	bob = login(t, s, "bob")
	if len(s.sessions[bob].rooms) != 0 {
		t.Fatal("rooms automatically rejoined")
	}
	joinRoom(t, s, bob, "Lobby")
	packets := requestPacket(t, s, bob, nil)
	if containsText(packets, "before disconnect") || containsText(packets, "while absent") {
		t.Fatal("room offline traffic replayed")
	}
	history, err := s.store.History(t.Context(), "bob", 3, "Lobby", 0, 50)
	if err != nil || len(history) != 1 || history[0].Body != "before disconnect" {
		t.Fatalf("room audience history: %v %v", history, err)
	}
}

func TestRoomHistoryRequiresJoinedSession(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	bobOther := login(t, s, "bob")
	joinRoom(t, s, alice, "Lobby")
	joinRoom(t, s, bob, "Lobby")
	s.Handle(t.Context(), alice, privateMessage("Lobby", "room history").SetByte(3, 3))
	for _, tc := range []struct {
		id   string
		code int
	}{{bob, 200}, {bobOther, 403}, {"", 401}} {
		r := httptest.NewRequest("GET", "http://localhost/api/chat/history?kind=room&target=lobby", nil)
		r.Header.Set("sid", tc.id)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("history status %d want%d", w.Code, tc.code)
		}
		if w.Code == 200 && !strings.Contains(w.Body.String(), "room history") {
			t.Fatal("missing room history")
		}
	}
}

func TestRoomBackpressureRejectsBeforeCommit(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	joinRoom(t, s, alice, "Lobby")
	joinRoom(t, s, bob, "Lobby")
	for len(s.sessions[bob].push) < cap(s.sessions[bob].push) {
		s.sessions[bob].push <- events.New(0, events.ContactPresence{})
	}
	r, _ := s.Handle(t.Context(), alice, privateMessage("Lobby", "not yet accepted").SetByte(3, 3))
	if r[0].Opcode != 0 {
		t.Fatal("full room queue falsely accepted")
	}
	history, err := s.store.History(t.Context(), "alice", 3, "Lobby", 0, 50)
	if err != nil || len(history) != 0 {
		t.Fatal("rejected room message was committed")
	}
	requestPacket(t, s, bob, nil)
	r, _ = s.Handle(t.Context(), alice, privateMessage("Lobby", "accepted after drain").SetByte(3, 3))
	if r[0].Opcode != 1 {
		t.Fatal("room queue failed to recover")
	}
}

func TestRoomMuteBlocksOnlyThatRoomAndUnmuteRestoresSending(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	joinRoom(t, s, alice, "Lobby")
	joinRoom(t, s, bob, "Lobby")
	joinRoom(t, s, bob, "Gaming")
	r, _ := s.Handle(t.Context(), alice, protocol.New(709, 30).SetText(1, "Lobby").SetText(2, "bob"))
	if r[0].Opcode != 1 {
		t.Fatal("moderator mute rejected")
	}
	r, _ = s.Handle(t.Context(), bob, privateMessage("Lobby", "blocked room text").SetByte(3, 3).SetText(2, "alice"))
	if r[0].Opcode != 0 {
		t.Fatal("muted sender bypassed mute")
	}
	history, err := s.store.History(t.Context(), "bob", 3, "Lobby", 0, 50)
	if err != nil || len(history) != 0 {
		t.Fatal("muted message entered history")
	}
	r, _ = s.Handle(t.Context(), bob, privateMessage("Gaming", "other room still works").SetByte(3, 3))
	if r[0].Opcode != 1 {
		t.Fatal("mute leaked into another room")
	}
	r, _ = s.Handle(t.Context(), bob, privateMessage("alice", "private message still works"))
	if r[0].Opcode != 1 {
		t.Fatal("mute leaked into direct messages")
	}
	r, _ = s.Handle(t.Context(), alice, protocol.New(710, 31).SetText(1, "Lobby").SetText(2, "bob"))
	if r[0].Opcode != 1 {
		t.Fatal("unmute")
	}
	r, _ = s.Handle(t.Context(), bob, privateMessage("Lobby", "unmuted room text").SetByte(3, 3))
	if r[0].Opcode != 1 {
		t.Fatal("unmuted sender still blocked")
	}
}

func TestRoomKickCancelsAllTargetSessionsBufferedMessages(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	bobOther := login(t, s, "bob")
	joinRoom(t, s, alice, "Lobby")
	joinRoom(t, s, bob, "Lobby")
	joinRoom(t, s, bobOther, "Lobby")
	joinRoom(t, s, bob, "Gaming")
	s.Handle(t.Context(), alice, privateMessage("Lobby", "queued before kick").SetByte(3, 3))
	r, _ := s.Handle(t.Context(), alice, protocol.New(706, 32).SetText(1, "Lobby").SetText(2, "bob"))
	if r[0].Opcode != 1 {
		t.Fatal("kick")
	}
	for _, id := range []string{bob, bobOther} {
		if s.sessions[id].rooms["Lobby"] {
			t.Fatal("target session still subscribed")
		}
		packets := requestPacket(t, s, id, nil)
		if containsText(packets, "queued before kick") {
			t.Fatal("kicked session received buffered text")
		}
		notice := false
		for _, p := range packets {
			if p.Opcode == 5 {
				notice = true
			}
		}
		if !notice {
			t.Fatal("kick notification missing")
		}
	}
	if !s.sessions[bob].rooms["Gaming"] {
		t.Fatal("kick removed unrelated room")
	}
	r, _ = s.Handle(t.Context(), bob, privateMessage("Lobby", "blocked after kick").SetByte(3, 3))
	if r[0].Opcode != 0 {
		t.Fatal("kicked user could send")
	}
	joinRoom(t, s, bob, "Lobby")
	r, _ = s.Handle(t.Context(), bob, privateMessage("Lobby", "after rejoin").SetByte(3, 3))
	if r[0].Opcode != 1 {
		t.Fatal("kick unexpectedly banned user")
	}
}
