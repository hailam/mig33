package realtime

import (
	"testing"
	"time"

	"github.com/hailam/mig33/internal/protocol"
)

func TestPresenceAggregatesSessionsDeterministically(t *testing.T) {
	s := newTestServer(t)
	one := login(t, s, "bob")
	two := login(t, s, "bob")
	s.Handle(t.Context(), one, protocol.New(600, 1).SetByte(1, 4))
	s.Handle(t.Context(), two, protocol.New(600, 2).SetByte(1, 3))
	presence := func() byte {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.presence("bob")
	}
	for i := 0; i < 100; i++ {
		if presence() != 3 {
			t.Fatal("busy/away depends on map iteration")
		}
	}
	s.Handle(t.Context(), one, protocol.New(600, 3).SetByte(1, 1))
	if presence() != 1 {
		t.Fatal("online session should be visible")
	}
	s.Handle(t.Context(), one, protocol.New(600, 4).SetByte(1, 99))
	if presence() != 3 {
		t.Fatal("invisible session should not hide other active sessions")
	}
	s.DropSession(t.Context(), two)
	if presence() != 99 {
		t.Fatal("only invisible session must be invisible")
	}
}

func TestExpiredHTTPSessionAnnouncesOfflineAndLeavesRooms(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := httpLogin(t, s, "bob")
	joinRoom(t, s, alice, "Lobby")
	joinRoom(t, s, bob, "Lobby")
	drainGroupQueue(s, alice)
	s.mu.Lock()
	s.sessions[bob].touched = time.Now().Add(-sessionTTL - time.Second)
	s.expireSessions(t.Context())
	exists := s.sessions[bob] != nil
	present := roomHasUser(s.roomParticipants("Lobby"), "bob")
	s.mu.Unlock()
	if exists {
		t.Fatal("expired HTTP session remains")
	}
	offline := false
	for _, p := range drainGroupQueue(s, alice) {
		id, _ := p.Int(1)
		value, _ := p.Byte(2)
		if p.Opcode == 404 && id == 2 && value == 99 {
			offline = true
		}
	}
	if !offline || present {
		t.Fatal("expiry did not announce offline and remove room membership")
	}
}

func TestReauthenticationAnnouncesRemainingPresence(t *testing.T) {
	for _, test := range []struct {
		name      string
		remaining bool
		want      byte
	}{
		{name: "last session goes offline", want: 99},
		{name: "another busy session stays visible", remaining: true, want: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := newTestServer(t)
			alice := login(t, s, "alice")
			bob := login(t, s, "bob")
			if test.remaining {
				other := login(t, s, "bob")
				s.Handle(t.Context(), other, protocol.New(600, 20).SetByte(1, 3))
			}
			drainGroupQueue(s, alice)

			replies, next := s.Handle(t.Context(), bob, protocol.New(200, 21).SetText(5, "bob"))
			if len(replies) != 1 || replies[0].Opcode != 201 || next == bob {
				t.Fatal("reauthentication did not replace the authenticated session")
			}
			updates := 0
			for _, packet := range drainGroupQueue(s, alice) {
				id, _ := packet.Int(1)
				if packet.Opcode != 404 || id != 2 {
					continue
				}
				value, _ := packet.Byte(2)
				if value != test.want {
					t.Fatalf("presence after replacement = %d, want %d", value, test.want)
				}
				updates++
			}
			if updates != 1 {
				t.Fatalf("presence updates after replacement = %d, want 1", updates)
			}

			challenge, _ := replies[0].Text(1)
			wrongProof := protocol.JavaHash(challenge+"bob-demo") + 1
			replies, next = s.Handle(t.Context(), next, protocol.New(202, 22).SetInt(1, wrongProof))
			if len(replies) != 1 || replies[0].Opcode != 0 || next != "" {
				t.Fatal("incorrect replacement login succeeded")
			}
			for _, packet := range drainGroupQueue(s, alice) {
				if packet.Opcode == 404 {
					t.Fatal("failed unauthenticated challenge announced another presence change")
				}
			}
			s.mu.Lock()
			got := s.presence("bob")
			s.mu.Unlock()
			if got != test.want {
				t.Fatalf("presence after failed replacement login = %d, want %d", got, test.want)
			}
		})
	}
}
