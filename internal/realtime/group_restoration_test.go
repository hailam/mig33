package realtime

import (
	"fmt"
	"strings"
	"testing"

	chatstore "github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func seedRestorationGroups(t *testing.T, s *Application, count int) []string {
	t.Helper()
	ids := make([]string, count)
	for i := range ids {
		ids[i] = fmt.Sprintf("restore-%03d", i)
		if _, err := s.store.CreateGroup(t.Context(), ids[i], "alice", []string{"bob", "carol"}); err != nil {
			t.Fatal(err)
		}
	}
	return ids
}

func TestGroupRestorationResumesAcrossFullAndPartialQueues(t *testing.T) {
	// More than two normal 128-packet queues; existing presence/control traffic
	// must neither consume the restoration cursor nor cause a replay.
	for _, occupied := range []int{0, 1, 127, 128} {
		t.Run(fmt.Sprintf("occupied-%d", occupied), func(t *testing.T) {
			s := newTestServer(t)
			groups := seedRestorationGroups(t, s, 257)
			if _, err := s.store.SaveMessage(t.Context(), chatstore.Message{
				Kind: 2, Conversation: groups[256], Sender: "alice", Body: "offline group text",
			}, nil); err != nil {
				t.Fatal(err)
			}
			id := login(t, s, "bob")
			seen := make(map[string]int)
			controls, text := 0, 0
			record := func(packets []*protocol.Packet) {
				for _, p := range packets {
					if p.Opcode == 3 {
						controls++
						continue
					}
					if p.Opcode != 500 {
						continue // Contact-sync responses are checked elsewhere.
					}
					body, _ := p.Text(8)
					if body == "offline group text" {
						if len(seen) != len(groups) {
							t.Fatal("offline text overtook group restoration")
						}
						text++
						continue
					}
					groupID, _ := p.Text(4)
					assertGroupNotice(t, p, groupID)
					seen[groupID]++
				}
			}
			record(requestPacket(t, s, id, nil))
			if len(seen) != 128 {
				t.Fatalf("first restoration batch has %d groups, want 128", len(seen))
			}
			s.mu.Lock()
			for i := 0; i < occupied; i++ {
				s.sessions[id].push <- events.New(0, events.Pong{})
			}
			s.mu.Unlock()
			for poll := 0; poll < 6; poll++ {
				record(requestPacket(t, s, id, nil))
				// Repeating contact sync while restoration is incomplete must not
				// begin the snapshot again or discard its unqueued tail.
				record(requestPacket(t, s, id, protocol.New(400, 13)))
			}
			if controls != occupied || text != 1 || len(seen) != len(groups) {
				t.Fatalf("controls=%d/%d text=%d groups=%d/%d", controls, occupied, text, len(seen), len(groups))
			}
			for _, groupID := range groups {
				if seen[groupID] != 1 {
					t.Fatalf("group %s restored %d times", groupID, seen[groupID])
				}
			}
			if pending, err := s.store.Pending(t.Context(), "bob", 10); err != nil || len(pending) != 0 {
				t.Fatalf("durable message was not delivered: %+v, %v", pending, err)
			}
		})
	}
}

func TestGroupRestorationUsesCurrentMembershipAndSkipsLeaves(t *testing.T) {
	s := newTestServer(t)
	groups := seedRestorationGroups(t, s, 130)
	id := login(t, s, "bob")
	other := login(t, s, "bob")
	// One already-buffered notice and one unqueued notice become stale when
	// the account leaves from another session before the first transport poll.
	for _, groupID := range []string{groups[0], groups[129]} {
		r, _ := s.Handle(t.Context(), other, protocol.New(753, 21).SetText(1, groupID).SetByte(2, 1))
		if len(r) != 1 || r[0].Opcode != 1 {
			t.Fatalf("leave %s: %v", groupID, r)
		}
	}
	if _, err := s.store.Invite(t.Context(), groups[128], "alice", []string{"dave"}); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]int)
	for poll := 0; poll < 3; poll++ {
		for _, p := range requestPacket(t, s, id, nil) {
			groupID, _ := p.Text(4)
			assertGroupNotice(t, p, groupID)
			seen[groupID]++
			if groupID == groups[0] || groupID == groups[129] {
				t.Fatalf("restoration reopened departed group %s", groupID)
			}
			if groupID == groups[128] {
				body, _ := p.Text(8)
				if !strings.Contains(body, "dave") {
					t.Fatal("deferred restoration used stale membership")
				}
			}
		}
	}
	if len(seen) != 128 {
		t.Fatalf("restored %d groups, want 128 current memberships", len(seen))
	}
	for groupID, count := range seen {
		if count != 1 {
			t.Fatalf("group %s restored %d times", groupID, count)
		}
	}
}
