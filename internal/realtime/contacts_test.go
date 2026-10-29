package realtime

import (
	"path/filepath"
	"testing"

	"github.com/hailam/mig33/internal/protocol"
)

func loginHailam(t *testing.T, s *Application) string {
	t.Helper()
	r, id := s.Handle(t.Context(), "", protocol.New(200, 1).SetText(5, "hailam"))
	challenge, _ := r[0].Text(1)
	r, id = s.Handle(t.Context(), id, protocol.New(202, 2).SetInt(1, protocol.JavaHash(challenge+"hailam")))
	if len(r) != 1 || r[0].Opcode != 203 {
		t.Fatalf("Hailam login failed: %v", r)
	}
	s.Handle(t.Context(), id, protocol.New(400, 3))
	return id
}

func contactRequest(t *testing.T, s *Application, sid string, request *protocol.Packet, want int16) []*protocol.Packet {
	t.Helper()
	r, _ := s.Handle(t.Context(), sid, request)
	if len(r) == 0 || r[0].Opcode != want || r[0].Transaction != request.Transaction {
		t.Fatalf("request %d: want %d, got %v", request.Opcode, want, r)
	}
	return r
}

func TestContactLifecycleAndOwnerIsolation(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	b := login(t, s, "bob")
	r := contactRequest(t, s, h, protocol.New(400, 10), 401)
	if len(r) != 3 {
		t.Fatal("Hailam must start with only an empty Friends group")
	}
	contactRequest(t, s, h, protocol.New(405, 11).SetText(12, "BOB").SetInt(2, 1).SetByte(24, 1), 402)
	r = contactRequest(t, s, h, protocol.New(408, 12).SetText(2, "Close friends"), 401)
	group, _ := r[0].Int(1)
	contactRequest(t, s, h, protocol.New(411, 13).SetInt(1, 2).SetInt(2, group), 1)
	contactRequest(t, s, h, protocol.New(409, 14).SetInt(1, group), 0) // Nonempty.
	contactRequest(t, s, b, protocol.New(410, 15).SetInt(1, group).SetText(2, "Forged"), 0)
	contactRequest(t, s, h, protocol.New(410, 16).SetInt(1, group).SetText(2, "Best friends"), 1)
	contactRequest(t, s, h, protocol.New(407, 17).SetInt(1, 2).SetText(3, "My Bob").SetText(8, "+960123"), 402)
	contactRequest(t, s, h, protocol.New(405, 18).SetText(12, "bob").SetText(3, "Reset attempt"), 402)
	c, err := s.store.Contact(t.Context(), "hailam", 2)
	if err != nil || c.Display != "My Bob" || c.Mobile != "+960123" || c.GroupID != group || !c.ShareMobile {
		t.Fatalf("saved metadata lost on duplicate add: %+v %v", c, err)
	}
	other, err := s.store.Contact(t.Context(), "alice", 2)
	if err != nil || other.Display != "Bob" || other.GroupID != 1 {
		t.Fatalf("another owner's contact was modified: %+v %v", other, err)
	}
	contactRequest(t, s, h, protocol.New(411, 19).SetInt(1, 2), 1) // Omitted2 means ungrouped.
	contactRequest(t, s, h, protocol.New(409, 20).SetInt(1, group), 1)
	c, _ = s.store.Contact(t.Context(), "hailam", 2)
	if c.GroupID != -1 {
		t.Fatal("move to ungrouped failed")
	}
	contactRequest(t, s, h, protocol.New(406, 21).SetInt(1, 2), 1)
	if _, err := s.store.Contact(t.Context(), "hailam", 2); err == nil {
		t.Fatal("removed contact remains")
	}
}

func TestContactValidationAndBlockedUserRemainIndependent(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	for _, request := range []*protocol.Packet{
		protocol.New(405, 10).SetText(12, "hailam"),
		protocol.New(405, 11).SetText(12, "missing"),
		protocol.New(405, 12).SetText(12, "bob").SetInt(2, 999),
		protocol.New(405, 13).SetText(12, "bob").SetByte(24, 2),
		protocol.New(405, 14).SetText(12, "bob").SetText(30, "external"),
		protocol.New(405, 15).SetText(8, "invalid phone"),
		protocol.New(407, 16).SetInt(1, 2).SetText(3, "Missing"),
		protocol.New(408, 17).SetText(2, "  "),
		protocol.New(408, 18).SetText(2, "Friends"),
	} {
		contactRequest(t, s, h, request, 0)
	}
	contactRequest(t, s, "", protocol.New(405, 19).SetText(12, "bob"), 0)
	contactRequest(t, s, h, protocol.New(405, 20).SetText(12, "bob").SetInt(2, 1), 402)
	contactRequest(t, s, h, protocol.New(407, 21).SetInt(1, 2).SetText(12, "carol"), 0)
	contactRequest(t, s, h, protocol.New(417, 22).SetText(1, "bob").SetByte(2, 2), 1)
	contactRequest(t, s, h, protocol.New(406, 23).SetInt(1, 2), 1)
	contactRequest(t, s, h, protocol.New(405, 24).SetText(12, "bob"), 402)
	blocked, err := s.store.IsBlocked(t.Context(), "hailam", "bob")
	if err != nil || !blocked {
		t.Fatal("contact remove/re-add must not remove personal block")
	}
}

func TestContactDeltaAndOtherSessions(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	other := loginHailam(t, s)
	before, _ := s.store.Contacts(t.Context(), "hailam")
	contactRequest(t, s, h, protocol.New(405, 10).SetText(12, "bob").SetInt(2, 1), 402)
	queued := requestPacket(t, s, other, nil)
	if len(queued) != 2 || queued[0].Opcode != 402 || queued[1].Opcode != 420 {
		t.Fatalf("other session add update: %v", queued)
	}
	r := contactRequest(t, s, other, protocol.New(419, 11).SetLong(1, before.Checkpoint), 402)
	if len(r) != 3 || r[1].Opcode != 420 || r[2].Opcode != 403 {
		t.Fatalf("incremental add: %v", r)
	}
	afterAdd, _ := s.store.Contacts(t.Context(), "hailam")
	contactRequest(t, s, h, protocol.New(406, 12).SetInt(1, 2), 1)
	r = contactRequest(t, s, h, protocol.New(419, 13).SetLong(1, afterAdd.Checkpoint), 406)
	if id, _ := r[0].Int(1); id != 2 || r[len(r)-1].Opcode != 403 {
		t.Fatal("deleted contact missing from delta")
	}
	requestPacket(t, s, other, nil)
	contactRequest(t, s, h, protocol.New(410, 14).SetInt(1, 1).SetText(2, "People"), 1)
	if !s.sessions[other].rosterStale || len(requestPacket(t, s, other, nil)) != 0 {
		t.Fatal("unsupported live group rename must retain old cache revision")
	}
	contactRequest(t, s, other, protocol.New(419, 15).SetLong(1, afterAdd.Checkpoint), 0)
	r = contactRequest(t, s, other, protocol.New(400, 16), 401)
	if name, _ := r[0].Text(2); name != "People" || s.sessions[other].rosterStale {
		t.Fatal("full refresh did not resolve stale group")
	}
	after, _ := s.store.Contacts(t.Context(), "hailam")
	r = contactRequest(t, s, h, protocol.New(419, 17).SetLong(1, after.Checkpoint), 420)
	if len(r) != 2 || r[1].Opcode != 403 {
		t.Fatal("unchanged delta should contain only checkpoint/end")
	}
	contactRequest(t, s, h, protocol.New(419, 18).SetLong(1, after.Checkpoint+1), 0)
}

func TestContactPersistenceAndNonContactMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat.sqlite")
	s, err := newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	h := loginHailam(t, s)
	b := login(t, s, "bob")
	// Address-book membership does not gate direct messages. Unknown contact
	// IDs must be absent so the original renderer uses the sender username.
	s.Handle(t.Context(), b, privateMessage("hailam", "before adding Bob"))
	packets := requestPacket(t, s, h, nil)
	if len(packets) != 1 || packets[0].Opcode != 500 {
		t.Fatalf("non-contact message: %v", packets)
	}
	if _, present := packets[0].Int(5); present {
		t.Fatal("non-contact message supplied nonexistent contact ID")
	}
	contactRequest(t, s, h, protocol.New(405, 10).SetText(12, "bob").SetInt(2, 1), 402)
	contactRequest(t, s, h, protocol.New(407, 11).SetInt(1, 2).SetText(3, "Bobby"), 402)
	contactRequest(t, s, b, protocol.New(406, 12).SetInt(1, 1), 1)
	s.Close()
	s, err = newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c, err := s.store.Contact(t.Context(), "hailam", 2)
	if err != nil || c.Display != "Bobby" || c.GroupID != 1 {
		t.Fatalf("contact not restored: %+v %v", c, err)
	}
	if _, err := s.store.Contact(t.Context(), "bob", 1); err == nil {
		t.Fatal("startup resurrected removed seed contact")
	}
}

func TestIncrementalContactsRefreshTransientPresenceAndStatus(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	b := login(t, s, "bob")
	contactRequest(t, s, h, protocol.New(405, 10).SetText(12, "bob").SetInt(2, 1), 402)
	checkpoint, _ := s.store.Contacts(t.Context(), "hailam")
	s.Handle(t.Context(), b, protocol.New(601, 11).SetText(1, "Changed while you were away"))
	s.Handle(t.Context(), b, protocol.New(300, 12))
	requestPacket(t, s, h, nil) // Discard transient pushes, simulating an absent cache.
	r, _ := s.Handle(t.Context(), h, protocol.New(419, 13).SetLong(1, checkpoint.Checkpoint))
	var presence, status bool
	for _, p := range r {
		switch p.Opcode {
		case 404:
			id, _ := p.Int(1)
			value, _ := p.Byte(2)
			presence = id == 2 && value == 99
		case 423:
			id, _ := p.Int(1)
			value, _ := p.Text(2)
			status = id == 2 && value == "Changed while you were away"
		}
	}
	if !presence || !status || r[len(r)-1].Opcode != 403 {
		t.Fatalf("incremental refresh omitted live state: %v", r)
	}
	unchanged, _ := s.store.Contacts(t.Context(), "hailam")
	if unchanged.Revision != checkpoint.Revision || unchanged.Checkpoint != checkpoint.Checkpoint {
		t.Fatal("transient state changed address-book checkpoint")
	}
}
