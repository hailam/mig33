package realtime

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func joinTestRoom(t *testing.T, s *Application, id, name string, transaction int16) []*protocol.Packet {
	t.Helper()
	r, _ := s.Handle(t.Context(), id, protocol.New(703, transaction).SetText(1, name))
	if len(r) < 2 || r[0].Opcode != 701 || r[1].Opcode != 1 || r[0].Transaction != transaction || r[1].Transaction != transaction {
		t.Fatalf("join must return701 before completing703 with ACK: %v", r)
	}
	return r
}

func assertRoomNotice(t *testing.T, p *protocol.Packet, name string) {
	t.Helper()
	channel, _ := p.Byte(1)
	sender, _ := p.Text(2)
	kind, _ := p.Byte(3)
	destination, _ := p.Text(4)
	content, _ := p.Short(6)
	body, _ := p.Text(8)
	if p.Opcode != 500 || p.Transaction != 0 || channel != 1 || sender != name || kind != 3 || destination != name || content != 1 || body == "" {
		t.Fatalf("invalid room system packet: %+v", p)
	}
}

func onlyRoomNotices(packets []*protocol.Packet) []*protocol.Packet {
	var out []*protocol.Packet
	for _, p := range packets {
		if kind, _ := p.Byte(3); p.Opcode == 500 && kind == 3 {
			out = append(out, p)
		}
	}
	return out
}

func TestRoomCategoriesAndListingWire(t *testing.T) {
	s := newTestServer(t)
	alice := groupLogin(t, s, "alice")
	r, _ := s.Handle(t.Context(), alice, protocol.New(713, 100))
	if len(r) != 3 || r[0].Opcode != 714 || r[1].Opcode != 714 || r[2].Opcode != 715 {
		t.Fatalf("room categories: %v", r)
	}
	if category, _ := r[0].Short(1); category != 1 {
		t.Fatal("missing native category ID")
	}
	if label, _ := r[0].Text(2); label == "" {
		t.Fatal("missing category label")
	}
	for _, tag := range []int16{3, 4, 5} {
		value, ok := r[0].Byte(tag)
		if !ok || (tag == 5 && value != 0) || (tag != 5 && value != 1) {
			t.Fatalf("missing/unexpected category flag%d", tag)
		}
	}
	for _, p := range r {
		if p.Transaction != 100 {
			t.Fatal("category response lost pending713 transaction")
		}
	}

	r, _ = s.Handle(t.Context(), alice, protocol.New(716, 101).SetShort(1, 1).SetByte(2, 1))
	if len(r) != 4 || r[3].Opcode != 717 {
		t.Fatalf("room list requires three701 entries then717: %v", r)
	}
	seen := make(map[string]bool)
	for _, p := range r[:3] {
		name, _ := p.Text(1)
		capacity, capOK := p.Int(3)
		occupancy, occOK := p.Int(4)
		category, catOK := p.Short(6)
		if p.Opcode != 701 || p.Transaction != 101 || !capOK || capacity != 50 || !occOK || occupancy != 0 || !catOK || category != 1 {
			t.Fatalf("room list entry lacks required client fields: %+v", p)
		}
		seen[name] = true
	}
	if !reflect.DeepEqual(seen, map[string]bool{"Lobby": true, "Gaming": true, "Help": true}) {
		t.Fatalf("seeded rooms: %v", seen)
	}
	if footer, ok := r[3].Text(1); !ok || footer != "Refresh" || r[3].Transaction != 101 {
		t.Fatal("category completion must carry its pending716 transaction and footer")
	}
}

func TestRoomJoinHTTPWireOrderAndIdempotence(t *testing.T) {
	s := newTestServer(t)
	alice := httpLogin(t, s, "alice")
	requestPacket(t, s, alice, protocol.New(400, 102))
	r := requestPacket(t, s, alice, protocol.New(703, 103).SetText(1, "lObBy"))
	if len(r) != 3 || r[0].Opcode != 701 || r[1].Opcode != 1 || r[0].Transaction != 103 || r[1].Transaction != 103 {
		t.Fatalf("join wire order must be701,ACK,welcome: %v", r)
	}
	if name, _ := r[0].Text(1); name != "Lobby" {
		t.Fatal("join did not return canonical room name")
	}
	if occupancy, _ := r[0].Int(4); occupancy != 1 {
		t.Fatal("joining user missing from room occupancy")
	}
	assertRoomNotice(t, r[2], "Lobby")
	if q := requestPacket(t, s, alice, nil); len(q) != 0 {
		t.Fatal("welcome should be in direct response, not another poll")
	}
	r = requestPacket(t, s, alice, protocol.New(703, 104).SetText(1, "Lobby"))
	if len(r) != 2 || r[0].Opcode != 701 || r[1].Opcode != 1 {
		t.Fatalf("repeated join duplicated notice or failed to complete request: %v", r)
	}
	r = requestPacket(t, s, alice, protocol.New(707, 105).SetText(1, "Lobby"))
	if len(r) != 1 || r[0].Opcode != 708 || r[0].Transaction != 105 {
		t.Fatalf("participants: %v", r)
	}
	if names, _ := r[0].Text(2); names != "alice;" {
		t.Fatal("participants need a delimiter after the final name")
	}
	if service, _ := r[0].Byte(5); service != 1 {
		t.Fatal("participant response must select native service")
	}
	if admins, _ := r[0].Text(3); admins != "alice;" {
		t.Fatal("participant list must identify the configured room moderator")
	}
}

func TestRoomPresenceIsPerSessionAndCountsUniqueUsers(t *testing.T) {
	s := newTestServer(t)
	alice := groupLogin(t, s, "alice")
	otherAlice := groupLogin(t, s, "alice")
	bob := groupLogin(t, s, "bob")
	carol := groupLogin(t, s, "carol")
	clearGroupQueues(s, alice, otherAlice, bob, carol)
	joinTestRoom(t, s, alice, "Lobby", 110)
	if q := drainGroupQueue(s, otherAlice); len(q) != 0 {
		t.Fatal("unjoined session received a room notification")
	}
	joinTestRoom(t, s, otherAlice, "Lobby", 111)
	if q := drainGroupQueue(s, alice); len(q) != 0 {
		t.Fatal("second session must not announce a duplicate participant")
	}
	joinTestRoom(t, s, bob, "Lobby", 112)
	for _, id := range []string{alice, otherAlice} {
		q := drainGroupQueue(s, id)
		if len(q) != 1 {
			t.Fatalf("joined session did not receive Bob's join notice: %v", q)
		}
		assertRoomNotice(t, q[0], "Lobby")
	}
	joinTestRoom(t, s, carol, "Gaming", 113)
	if participants := s.roomParticipants("Lobby"); !reflect.DeepEqual(participants, []string{"alice", "bob"}) {
		t.Fatalf("session count/other room leaked into participants: %v", participants)
	}
	if q := drainGroupQueue(s, alice); len(q) != 0 {
		t.Fatal("Gaming join broadcast to Lobby")
	}

	r, _ := s.Handle(t.Context(), alice, protocol.New(704, 114).SetText(1, "Lobby"))
	if len(r) != 1 || r[0].Opcode != 1 || r[0].Transaction != 114 {
		t.Fatalf("leave: %v", r)
	}
	for _, id := range []string{alice, otherAlice, bob} {
		if q := drainGroupQueue(s, id); len(q) != 0 {
			t.Fatalf("first session leave incorrectly announced user departure: %v", q)
		}
	}
	if participants := s.roomParticipants("Lobby"); !reflect.DeepEqual(participants, []string{"alice", "bob"}) {
		t.Fatal("another session should retain Alice's room presence")
	}
	s.DropSession(t.Context(), otherAlice)
	q := onlyRoomNotices(drainGroupQueue(s, bob))
	if len(q) != 1 {
		t.Fatalf("last-session disconnect must announce departure once: %v", q)
	}
	assertRoomNotice(t, q[0], "Lobby")
	if q := onlyRoomNotices(drainGroupQueue(s, alice)); len(q) != 0 {
		t.Fatal("leaving connection received a room notice that would reopen its tab")
	}
	if q := onlyRoomNotices(drainGroupQueue(s, carol)); len(q) != 0 {
		t.Fatal("unrelated room received departure notice")
	}
	if participants := s.roomParticipants("Lobby"); !reflect.DeepEqual(participants, []string{"bob"}) {
		t.Fatalf("disconnected user retained room presence: %v", participants)
	}
}

func TestRoomAuthorizationAndMalformedRequests(t *testing.T) {
	s := newTestServer(t)
	r, _ := s.Handle(t.Context(), "", protocol.New(703, 120).SetText(1, "Lobby"))
	if len(r) != 1 || r[0].Opcode != 0 {
		t.Fatal("anonymous room join accepted")
	}
	alice := groupLogin(t, s, "alice")
	requests := []*protocol.Packet{
		protocol.New(703, 121),
		protocol.New(703, 121).SetText(1, "Unknown"),
		protocol.New(703, 121).SetText(1, "Lobby;Gaming"),
		protocol.New(704, 121).SetText(1, "Lobby"),
		protocol.New(707, 121).SetText(1, "Lobby"),
		protocol.New(716, 121),
		protocol.New(716, 121).SetShort(1, 3).SetByte(2, 1),
		protocol.New(716, 121).SetShort(1, 1),
		protocol.New(716, 121).SetShort(1, 1).SetByte(2, 2),
	}
	for i, p := range requests {
		r, _ := s.Handle(t.Context(), alice, p)
		if len(r) != 1 || r[0].Opcode != 0 || r[0].Transaction != 121 {
			t.Fatalf("invalid/unjoined room request%d accepted: %v", i, r)
		}
		if text, _ := r[0].Text(2); text == "" {
			t.Fatal("room error lacks client-readable text")
		}
	}
	joinTestRoom(t, s, alice, "Lobby", 122)
	s.Handle(t.Context(), alice, protocol.New(704, 123).SetText(1, "Lobby"))
	r, _ = s.Handle(t.Context(), alice, protocol.New(707, 124).SetText(1, "Lobby"))
	if r[0].Opcode != 0 {
		t.Fatal("departed session retained access to live participants")
	}
}

func TestRoomCapacityAndExpiredPresence(t *testing.T) {
	s := newTestServer(t)
	alice := groupLogin(t, s, "alice")
	// Model additional authenticated accounts to exercise the seeded50-user
	// capacity without weakening production account or room definitions.
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("capacity-%d", i)
		s.sessions[id] = &session{
			id: id, username: id, authenticated: true,
			touched: time.Now(), rooms: map[string]bool{"Lobby": true}, push: make(chan *events.Event, 4),
		}
	}

	r, _ := s.Handle(t.Context(), alice, protocol.New(703, 130).SetText(1, "Lobby"))
	if len(r) != 1 || r[0].Opcode != 0 || s.sessions[alice].rooms["Lobby"] {
		t.Fatal("full room accepted a new user")
	}
	s.sessions["capacity-0"].touched = time.Now().Add(-sessionTTL - time.Second)
	r = joinTestRoom(t, s, alice, "Lobby", 131)
	if occupancy, _ := r[0].Int(4); occupancy != 50 {
		t.Fatalf("expired session counted toward room capacity: %d", occupancy)
	}
	// Additional sessions of an already present user consume no extra slot.
	id := groupLogin(t, s, "alice")
	r = joinTestRoom(t, s, id, "Lobby", 132)
	if occupancy, _ := r[0].Int(4); occupancy != 50 {
		t.Fatal("second session increased occupancy")
	}
}

func TestRoomDisconnectDoesNotAutomaticallyRejoin(t *testing.T) {
	s := newTestServer(t)
	alice := groupLogin(t, s, "alice")
	joinTestRoom(t, s, alice, "Lobby", 140)
	joinTestRoom(t, s, alice, "Gaming", 141)
	s.DropSession(t.Context(), alice)
	alice = groupLogin(t, s, "alice")
	if len(s.sessions[alice].rooms) != 0 || len(s.roomParticipants("Lobby")) != 0 || len(s.roomParticipants("Gaming")) != 0 {
		t.Fatal("room presence survived a disconnected session")
	}
	if q := onlyRoomNotices(drainGroupQueue(s, alice)); len(q) != 0 {
		t.Fatal("room tabs should be rejoined by703, not pushed automatically at login")
	}
}

func TestRoomModerationRejectsUnauthorizedActions(t *testing.T) {
	s := newTestServer(t)
	alice := groupLogin(t, s, "alice")
	bob := groupLogin(t, s, "bob")
	carol := groupLogin(t, s, "carol")
	for _, id := range []string{alice, bob, carol} {
		joinTestRoom(t, s, id, "Lobby", 150)
	}
	clearGroupQueues(s, alice, bob, carol)
	for _, opcode := range []int16{706, 709, 710} {
		for _, request := range []struct {
			id, room, target string
		}{
			{bob, "Lobby", "carol"}, // A regular member cannot moderate.
			{bob, "Lobby", "alice"}, // Neither can they target a moderator.
			{alice, "Help", "bob"},  // A moderator must join this room first.
			{alice, "Lobby", "ALICE"},
			{alice, "Lobby", "dave"}, // Registered but absent target.
			{alice, "Lobby", "missing"},
			{alice, "Lobby", ""},
		} {
			if opcode == 706 && request.id == bob && request.target == "carol" {
				continue // Regular706 is a vote, covered by vote lifecycle tests.
			}
			r, _ := s.Handle(t.Context(), request.id, protocol.New(opcode, 151).
				SetText(1, request.room).SetText(2, request.target))
			if len(r) != 1 || r[0].Opcode != 0 || r[0].Transaction != 151 {
				t.Fatalf("unauthorized%d(%+v) accepted: %v", opcode, request, r)
			}
			if message, _ := r[0].Text(2); message == "" {
				t.Fatal("moderation rejection lacks readable error")
			}
		}
	}
	if participants := s.roomParticipants("Lobby"); !reflect.DeepEqual(participants, []string{"alice", "bob", "carol"}) {
		t.Fatal("rejected moderation changed room membership")
	}
	if muted, err := s.store.RoomMuted(t.Context(), "Lobby"); err != nil || len(muted) != 0 {
		t.Fatalf("rejected moderation changed mute state: %v %v", muted, err)
	}
	for _, id := range []string{alice, bob, carol} {
		if q := drainGroupQueue(s, id); len(q) != 0 {
			t.Fatal("rejected moderation broadcast a success notification")
		}
	}
}

func TestRoomMuteAndUnmuteUpdateNativeParticipantLists(t *testing.T) {
	s := newTestServer(t)
	alice := groupLogin(t, s, "alice")
	bob := groupLogin(t, s, "bob")
	joinTestRoom(t, s, alice, "Lobby", 160)
	joinTestRoom(t, s, bob, "Lobby", 161)
	clearGroupQueues(s, alice, bob)
	r, _ := s.Handle(t.Context(), alice, protocol.New(709, 162).SetText(1, "lobby").SetText(2, "BOB"))
	if len(r) != 1 || r[0].Opcode != 1 || r[0].Transaction != 162 {
		t.Fatalf("mute: %v", r)
	}
	if text, _ := r[0].Text(1); !strings.Contains(text, "muted") {
		t.Fatal("mute acknowledgment should populate the original success dialog")
	}
	for _, id := range []string{alice, bob} {
		q := drainGroupQueue(s, id)
		if len(q) != 1 {
			t.Fatalf("mute needs one room notice, not an unsolicited708: %v", q)
		}
		assertRoomNotice(t, q[0], "Lobby")
	}

	r, _ = s.Handle(t.Context(), bob, protocol.New(707, 163).SetText(1, "Lobby"))
	if admins, _ := r[0].Text(3); admins != "alice;" {
		t.Fatal("moderator list is not semicolon-terminated")
	}
	if muted, _ := r[0].Text(4); muted != "bob;" {
		t.Fatal("muted participant is missing the native Un-Mute marker")
	}
	message := protocol.New(500, 164).SetByte(1, 1).SetByte(3, 3).
		SetText(4, "Lobby").SetShort(6, 1).SetText(8, "Muted message")
	r, _ = s.Handle(t.Context(), bob, message)
	if r[0].Opcode != 0 {
		t.Fatal("muted user can still send to the room")
	}
	// Mute affects sending only. The user remains joined and receives text.
	r, _ = s.Handle(t.Context(), alice, message.SetText(8, "You can still read this"))
	if r[0].Opcode != 1 {
		t.Fatal("moderator message failed")
	}
	q := requestPacket(t, s, bob, nil)
	if len(q) != 1 || q[0].Opcode != 500 {
		t.Fatalf("muted member stopped receiving room messages: %v", q)
	}
	if sender, _ := q[0].Text(2); sender != "alice" {
		t.Fatal("muted reader received the wrong message")
	}
	s.Handle(t.Context(), alice, protocol.New(709, 165).SetText(1, "Lobby").SetText(2, "bob"))
	for _, id := range []string{alice, bob} {
		if q := drainGroupQueue(s, id); len(q) != 0 {
			t.Fatal("repeated mute duplicated its membership notice")
		}
	}

	r, _ = s.Handle(t.Context(), alice, protocol.New(710, 166).SetText(1, "Lobby").SetText(2, "bob"))
	if len(r) != 1 || r[0].Opcode != 1 || r[0].Transaction != 166 {
		t.Fatalf("unmute: %v", r)
	}

	r, _ = s.Handle(t.Context(), bob, protocol.New(707, 167).SetText(1, "Lobby"))
	if muted, _ := r[0].Text(4); muted != "" {
		t.Fatal("unmute retained the native participant marker")
	}

	r, _ = s.Handle(t.Context(), bob, message.SetText(8, "Allowed again"))
	if r[0].Opcode != 1 {
		t.Fatal("unmute did not restore sending")
	}
}

func TestRoomModeratorsCannotTargetOtherModerators(t *testing.T) {
	s := newTestServer(t)
	alice := groupLogin(t, s, "alice")
	carol := groupLogin(t, s, "carol")
	joinTestRoom(t, s, alice, "Lobby", 168)
	joinTestRoom(t, s, carol, "Lobby", 169)
	if err := s.store.SetRoomModerator(t.Context(), "Lobby", "carol", true); err != nil {
		t.Fatal(err)
	}
	for _, opcode := range []int16{706, 709, 710} {
		for _, request := range []struct{ actor, target string }{{alice, "carol"}, {carol, "alice"}} {
			r, _ := s.Handle(t.Context(), request.actor, protocol.New(opcode, 169).
				SetText(1, "Lobby").SetText(2, request.target))
			if len(r) != 1 || r[0].Opcode != 0 {
				t.Fatalf("moderator-to-moderator%d accepted", opcode)
			}
		}
	}

	r, _ := s.Handle(t.Context(), alice, protocol.New(707, 169).SetText(1, "Lobby"))
	if moderators, _ := r[0].Text(3); moderators != "alice;carol;" {
		t.Fatal("operator-granted role was not reflected in the native participant list")
	}
}

func TestRoomKickRemovesAllTargetSessionsOnlyFromThatRoom(t *testing.T) {
	s := newTestServer(t)
	alice := groupLogin(t, s, "alice")
	bob := groupLogin(t, s, "bob")
	otherBob := groupLogin(t, s, "bob")
	unjoinedBob := groupLogin(t, s, "bob")
	carol := groupLogin(t, s, "carol")
	for _, id := range []string{alice, bob, otherBob, carol} {
		joinTestRoom(t, s, id, "Lobby", 170)
	}
	for _, id := range []string{bob, otherBob} {
		joinTestRoom(t, s, id, "Help", 171)
	}
	clearGroupQueues(s, alice, bob, otherBob, unjoinedBob, carol)
	message := protocol.New(500, 172).SetByte(1, 1).SetByte(3, 3).
		SetText(4, "Lobby").SetShort(6, 1).SetText(8, "Queued before the kick")
	r, _ := s.Handle(t.Context(), alice, message)
	if r[0].Opcode != 1 {
		t.Fatal("could not queue message before kick")
	}

	r, _ = s.Handle(t.Context(), alice, protocol.New(706, 173).SetText(1, "Lobby").SetText(2, "BOB"))
	if len(r) != 1 || r[0].Opcode != 1 || r[0].Transaction != 173 {
		t.Fatalf("kick: %v", r)
	}
	for _, id := range []string{bob, otherBob} {
		x := s.sessions[id]
		if x == nil || !x.authenticated || x.rooms["Lobby"] || !x.rooms["Help"] {
			t.Fatal("kick must remove only the target room, across all target sessions")
		}
		q := requestPacket(t, s, id, nil)
		if len(q) != 1 || q[0].Opcode != 5 || q[0].Transaction != 0 {
			t.Fatalf("kick must cancel queued room text and deliver a native popup: %v", q)
		}
		subtype, _ := q[0].Byte(1)
		text, _ := q[0].Text(2)
		presentation, _ := q[0].Byte(3)
		if subtype != 0 || presentation != 1 || !strings.Contains(text, "kicked from Lobby by alice") {
			t.Fatal("invalid original-client notification fields")
		}
	}
	if q := requestPacket(t, s, unjoinedBob, nil); len(q) != 0 {
		t.Fatal("an unjoined session should not receive a kick popup")
	}
	if participants := s.roomParticipants("Lobby"); !reflect.DeepEqual(participants, []string{"alice", "carol"}) {
		t.Fatalf("kick participant result: %v", participants)
	}
	for _, p := range []*protocol.Packet{
		protocol.New(707, 174).SetText(1, "Lobby"),
		message.SetText(8, "After kick"),
	} {
		r, _ = s.Handle(t.Context(), bob, p)
		if r[0].Opcode != 0 {
			t.Fatal("kicked session retained room privileges")
		}
	}
	// A kick is not a permanent ban. The existing native tab can be rejoined.
	joinTestRoom(t, s, bob, "Lobby", 175)
}

func TestRoomMuteSurvivesReconnectAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "moderation.sqlite")
	s, err := newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	alice := groupLogin(t, s, "alice")
	bob := groupLogin(t, s, "bob")
	joinTestRoom(t, s, alice, "Lobby", 180)
	joinTestRoom(t, s, bob, "Lobby", 181)
	r, _ := s.Handle(t.Context(), alice, protocol.New(709, 182).SetText(1, "Lobby").SetText(2, "bob"))
	if r[0].Opcode != 1 {
		t.Fatal("initial mute failed")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	bob = groupLogin(t, s, "bob")
	joinTestRoom(t, s, bob, "Lobby", 183)
	r, _ = s.Handle(t.Context(), bob, protocol.New(707, 184).SetText(1, "Lobby"))
	if muted, _ := r[0].Text(4); muted != "bob;" {
		t.Fatal("stored mute was lost after restart/rejoin")
	}
	if admins, _ := r[0].Text(3); admins != "" {
		t.Fatal("offline moderator should not appear among live participants")
	}
	message := protocol.New(500, 185).SetByte(1, 1).SetByte(3, 3).
		SetText(4, "Lobby").SetShort(6, 1).SetText(8, "After restart")
	r, _ = s.Handle(t.Context(), bob, message)
	if r[0].Opcode != 0 {
		t.Fatal("reconnect bypassed stored room mute")
	}
	joinTestRoom(t, s, bob, "Gaming", 186)
	r, _ = s.Handle(t.Context(), bob, message.SetText(4, "Gaming"))
	if r[0].Opcode != 1 {
		t.Fatal("room mute leaked into another room")
	}
}
