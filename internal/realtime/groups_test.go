package realtime

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/protocol"
)

func groupLogin(t *testing.T, s *Application, user string) string {
	t.Helper()
	// login already initializes the roster and queues restored group notices.
	return login(t, s, user)
}

func drainGroupQueue(s *Application, id string) []*protocol.Packet {
	s.mu.Lock()
	defer s.mu.Unlock()
	var packets []*protocol.Packet
	if x := s.sessions[id]; x != nil {
		for {
			select {
			case p := <-x.push:
				packets = append(packets, fixturePacket(p))
			default:
				return packets
			}
		}
	}
	return packets
}

func clearGroupQueues(s *Application, ids ...string) {
	for _, id := range ids {
		drainGroupQueue(s, id)
	}
}

func createTestGroup(t *testing.T, s *Application, alice string) (string, []*protocol.Packet) {
	t.Helper()
	r, _ := s.Handle(t.Context(), alice, protocol.New(751, 31).SetText(1, "bob").SetText(2, "carol").SetByte(3, 1))
	if len(r) != 2 || r[0].Opcode != 750 || r[0].Transaction != 31 {
		t.Fatalf("create: %v", r)
	}
	id, ok := r[0].Text(1)
	if !ok || !strings.HasPrefix(id, "g") || len(id) != 49 {
		t.Fatalf("group ID: %q", id)
	}
	return id, r
}

func assertGroupNotice(t *testing.T, p *protocol.Packet, groupID string) {
	t.Helper()
	channel, _ := p.Byte(1)
	sender, _ := p.Text(2)
	kind, _ := p.Byte(3)
	destination, _ := p.Text(4)
	content, _ := p.Short(6)
	body, _ := p.Text(8)
	if p.Opcode != 500 || p.Transaction != 0 || channel != 1 || sender != groupID || kind != 2 || destination != groupID || content != 1 || body == "" {
		t.Fatalf("invalid group system packet: %+v", p)
	}
}

func TestGroupCreationConvertsPeerBeforeNotice(t *testing.T) {
	s := newTestServer(t)
	alice := groupLogin(t, s, "alice")
	otherAlice := groupLogin(t, s, "alice")
	bob := groupLogin(t, s, "bob")
	carol := groupLogin(t, s, "carol")
	dave := groupLogin(t, s, "dave")
	clearGroupQueues(s, alice, otherAlice, bob, carol, dave)
	id, replies := createTestGroup(t, s, alice)
	if service, _ := replies[0].Byte(3); service != 1 {
		t.Fatal("conversion service missing")
	}
	assertGroupNotice(t, replies[1], id)
	if q := drainGroupQueue(s, alice); len(q) != 0 {
		t.Fatal("creator notice must follow 750 in its direct response, not a concurrent poll")
	}
	q := drainGroupQueue(s, bob)
	if len(q) != 2 || q[0].Opcode != 750 || q[0].Transaction != 0 {
		t.Fatalf("peer conversion must precede group notice: %v", q)
	}
	if peer, _ := q[0].Text(2); peer != "alice" {
		t.Fatal("unsolicited conversion must name the creator's private conversation")
	}
	if destination, _ := q[0].Text(1); destination != id {
		t.Fatal("conversion changed the group ID")
	}
	assertGroupNotice(t, q[1], id)
	for _, sessionID := range []string{carol, otherAlice} {
		q = drainGroupQueue(s, sessionID)
		if len(q) != 1 {
			t.Fatalf("new invitee/other creator session needs one notice: %v", q)
		}
		assertGroupNotice(t, q[0], id)
	}
	if q := drainGroupQueue(s, dave); len(q) != 0 {
		t.Fatal("outsider received private group state")
	}
	g, err := s.store.Group(t.Context(), id, "alice")
	if err != nil || !reflect.DeepEqual(g.Members, []string{"alice", "bob", "carol"}) {
		t.Fatalf("persisted membership: %+v, %v", g, err)
	}
}

func TestGroupCreationHTTPWireOrder(t *testing.T) {
	s := newTestServer(t)
	alice := httpLogin(t, s, "alice")
	requestPacket(t, s, alice, protocol.New(400, 13))
	replies := requestPacket(t, s, alice, protocol.New(751, 42).
		SetText(1, "bob").SetText(2, "carol").SetByte(3, 1))
	if len(replies) != 2 || replies[0].Opcode != 750 || replies[0].Transaction != 42 {
		t.Fatalf("HTTP must serialize conversion before creation notice: %v", replies)
	}
	id, _ := replies[0].Text(1)
	assertGroupNotice(t, replies[1], id)
	if q := requestPacket(t, s, alice, nil); len(q) != 0 {
		t.Fatalf("HTTP poll repeated creation notice: %v", q)
	}
}

func TestGroupParticipantsAndMemberInvites(t *testing.T) {
	s := newTestServer(t)
	alice := groupLogin(t, s, "alice")
	bob := groupLogin(t, s, "bob")
	dave := groupLogin(t, s, "dave")
	id, _ := createTestGroup(t, s, alice) // Carol can be invited while offline.
	clearGroupQueues(s, alice, bob, dave)
	r, _ := s.Handle(t.Context(), alice, protocol.New(754, 50).SetText(1, id).SetByte(2, 1))
	participants, _ := r[0].Text(2)
	service, _ := r[0].Byte(5)
	if len(r) != 1 || r[0].Opcode != 755 || r[0].Transaction != 50 || participants != "alice;bob;carol;" || service != 1 {
		t.Fatalf("participant response: %+v", r)
	}
	if _, ok := r[0].Bytes(3); ok {
		t.Fatal("group members must not be invented as room administrators")
	}
	for _, opcode := range []int16{752, 753, 754} {
		p := protocol.New(opcode, 51).SetText(1, id).SetByte(2, 1)
		if opcode == 752 {
			p.SetText(2, "bob").SetByte(3, 1)
		}
		r, _ = s.Handle(t.Context(), dave, p)
		if len(r) != 1 || r[0].Opcode != 0 || r[0].Transaction != 51 {
			t.Fatalf("outsider operation %d accepted", opcode)
		}
	}
	// A member other than the creator can invite, and names are canonicalized.
	p := protocol.New(752, 52).SetText(1, id).SetText(2, "DAVE").SetByte(3, 1)
	r, _ = s.Handle(t.Context(), bob, p)
	if len(r) != 1 || r[0].Opcode != 1 || r[0].Transaction != 52 {
		t.Fatalf("member invite: %v", r)
	}
	for _, sessionID := range []string{alice, bob, dave} {
		q := drainGroupQueue(s, sessionID)
		if len(q) != 1 {
			t.Fatalf("invite should emit one group notice and no 750/755: %v", q)
		}
		assertGroupNotice(t, q[0], id)
	}

	r, _ = s.Handle(t.Context(), bob, p)
	if r[0].Opcode != 1 {
		t.Fatal("repeated invite should be idempotent")
	}
	for _, sessionID := range []string{alice, bob, dave} {
		if q := drainGroupQueue(s, sessionID); len(q) != 0 {
			t.Fatal("repeated invite duplicated a join notification")
		}
	}

	r, _ = s.Handle(t.Context(), dave, protocol.New(754, 53).SetText(1, id).SetByte(2, 1))
	participants, _ = r[0].Text(2)
	if r[0].Opcode != 755 || participants != "alice;bob;carol;dave;" {
		t.Fatal("new member cannot read current membership")
	}
}

func TestGroupRejectsMalformedAndUnsupportedOperations(t *testing.T) {
	s := newTestServer(t)
	alice := groupLogin(t, s, "alice")
	id, _ := createTestGroup(t, s, alice)
	requests := []*protocol.Packet{
		protocol.New(751, 70).SetText(1, "bob").SetText(2, "carol"),
		protocol.New(751, 70).SetText(1, "bob").SetText(2, "carol").SetByte(3, 2),
		protocol.New(751, 70).SetText(2, "carol").SetByte(3, 1),
		protocol.New(751, 70).SetText(1, "bob").SetByte(3, 1),
		protocol.New(751, 70).SetText(1, "bob").SetText(2, "BOB").SetByte(3, 1),
		protocol.New(751, 70).SetText(1, "alice").SetText(2, "carol").SetByte(3, 1),
		protocol.New(751, 70).SetText(1, "bob").SetText(2, "ALICE").SetByte(3, 1),
		protocol.New(751, 70).SetText(1, "missing").SetText(2, "carol").SetByte(3, 1),
		protocol.New(751, 70).SetText(1, "bob").SetText(2, "carol;").SetByte(3, 1),
		protocol.New(752, 70).SetText(1, id).SetText(2, "missing").SetByte(3, 1),
		protocol.New(752, 70).SetText(1, id).SetText(2, "alice").SetByte(3, 1),
		protocol.New(752, 70).SetText(1, id).SetByte(3, 1),
		protocol.New(752, 70).SetText(1, id).SetText(2, "dave").SetByte(3, 2),
		protocol.New(753, 70).SetText(1, id).SetByte(2, 2),
		protocol.New(754, 70).SetText(1, id),
		protocol.New(754, 70).SetText(1, "missing").SetByte(2, 1),
		protocol.New(754, 70).SetText(1, strings.Repeat("x", 257)).SetByte(2, 1),
	}
	for i, p := range requests {
		r, _ := s.Handle(t.Context(), alice, p)
		if len(r) != 1 || r[0].Opcode != 0 || r[0].Transaction != 70 {
			t.Fatalf("invalid request %d accepted: %+v", i, r)
		}
		if message, _ := r[0].Text(2); message == "" {
			t.Fatalf("invalid request %d returned no client-readable error", i)
		}
	}
	groups, err := s.store.Groups(t.Context(), "alice")
	if err != nil || len(groups) != 1 || len(groups[0].Members) != 3 {
		t.Fatalf("rejected request changed membership: %+v, %v", groups, err)
	}
}

func TestGroupLeaveDoesNotReopenLeaverConversation(t *testing.T) {
	s := newTestServer(t)
	alice := groupLogin(t, s, "alice")
	bob := groupLogin(t, s, "bob")
	carol := groupLogin(t, s, "carol")
	id, _ := createTestGroup(t, s, alice)
	clearGroupQueues(s, alice, bob, carol)
	r, _ := s.Handle(t.Context(), bob, protocol.New(753, 80).SetText(1, id).SetByte(2, 1))
	if len(r) != 1 || r[0].Opcode != 1 || r[0].Transaction != 80 {
		t.Fatalf("leave: %v", r)
	}
	if q := drainGroupQueue(s, bob); len(q) != 0 {
		t.Fatal("leave notification would reopen the leaver's closed tab")
	}
	for _, sessionID := range []string{alice, carol} {
		q := drainGroupQueue(s, sessionID)
		if len(q) != 1 {
			t.Fatalf("remaining member missed leave: %v", q)
		}
		assertGroupNotice(t, q[0], id)
	}
	for _, opcode := range []int16{752, 753, 754} {
		p := protocol.New(opcode, 81).SetText(1, id).SetByte(2, 1)
		if opcode == 752 {
			p.SetText(2, "dave").SetByte(3, 1)
		}
		r, _ = s.Handle(t.Context(), bob, p)
		if r[0].Opcode != 0 {
			t.Fatalf("former member retained operation %d", opcode)
		}
	}
	// Leaving does not destroy the group or grant special powers to its creator.
	r, _ = s.Handle(t.Context(), alice, protocol.New(753, 82).SetText(1, id).SetByte(2, 1))
	if r[0].Opcode != 1 {
		t.Fatal("creator cannot leave")
	}

	r, _ = s.Handle(t.Context(), carol, protocol.New(752, 83).SetText(1, id).SetText(2, "dave").SetByte(3, 1))
	if r[0].Opcode != 1 {
		t.Fatal("remaining member must retain invite rights after creator leaves")
	}
}

func TestOfflineGroupMembershipSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.sqlite")
	s, err := newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	alice := groupLogin(t, s, "alice")
	id, _ := createTestGroup(t, s, alice)
	s.DropSession(t.Context( // Disconnection must not mean leaving the group.
	), alice)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	carol := groupLogin(t, s, "carol")
	q := drainGroupQueue(s, carol)
	if len(q) != 1 {
		t.Fatalf("offline invitation did not restore a tab after restart: %v", q)
	}
	assertGroupNotice(t, q[0], id)
	r, _ := s.Handle(t.Context(), carol, protocol.New(754, 90).SetText(1, id).SetByte(2, 1))
	if members, _ := r[0].Text(2); r[0].Opcode != 755 || members != "alice;bob;carol;" {
		t.Fatal("offline membership changed across restart")
	}
}
