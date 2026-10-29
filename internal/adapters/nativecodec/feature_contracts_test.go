package nativecodec

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
	"github.com/hailam/mig33/internal/rooms"
)

func TestContactDecodePreservesOptionalFieldsAndExternalFraming(t *testing.T) {
	request, _ := DecodeContact(protocol.New(407, 11).SetBytes(1, []byte{0, 0, 1}).SetBytes(24, nil))
	input := request.Input.(commands.Contact)
	if request.Correlation != 11 || !input.ID.Present || input.ID.Valid || !input.ShareMobile.Present || input.ShareMobile.Valid || input.GroupID.Present {
		t.Fatalf("field state lost: %+v", input)
	}
	packet := protocol.New(407, 12).SetText(30, "alice\x00bob").SetBytes(31, []byte{2, 4})
	request, _ = DecodeContact(packet)
	input = request.Input.(commands.Contact)
	if !input.External.Valid || !reflect.DeepEqual(input.External.Value, []contacts.ContactIM{{Service: 2, Username: "alice"}, {Service: 4, Username: "bob"}}) {
		t.Fatalf("external identities: %+v", input.External)
	}
	packet.Fields[30][0] = 'x'
	packet.Fields[31][0] = 6
	if input.External.Value[0].Username != "alice" || input.External.Value[0].Service != 2 {
		t.Fatal("typed input aliases mutable packet")
	}
	for _, packet := range []*protocol.Packet{
		protocol.New(405, 1).SetText(30, "alice\x00bob").SetBytes(31, []byte{2, 4}),
		protocol.New(407, 1).SetText(30, "alice"),
		protocol.New(407, 1).SetBytes(31, []byte{2}),
	} {
		request, _ := DecodeContact(packet)
		if input := request.Input.(commands.Contact); !input.External.Present || input.External.Valid {
			t.Fatalf("malformed external field accepted: %+v", input)
		}
	}
}

func TestNativeContactDetailOptionalFieldsAndExternalPresence(t *testing.T) {
	empty := ""
	packet, ok := EncodeContact(events.New(-3, events.Contact{Contact: contacts.Contact{ID: 7, GroupID: -1, Kind: "im", Display: "Directory", External: []contacts.ContactIM{{Service: 2, Username: "alice"}, {Service: 4, Username: "bob"}}}, Status: &empty}))
	if !ok || packet.Opcode != 402 || packet.Transaction != -3 {
		t.Fatal(packet)
	}
	if _, present := packet.Fields[2]; present {
		t.Fatal("ungrouped contact gained a group field")
	}
	if _, present := packet.Fields[12]; present {
		t.Fatal("external contact gained a mig33 username")
	}
	if status, present := packet.Text(28); !present || status != "" {
		t.Fatal("present empty status was lost")
	}
	if names, _ := packet.Text(33); names != "alice\x00bob" {
		t.Fatal(names)
	}
	if !bytes.Equal(packet.Fields[31], []byte{2, 4}) || !bytes.Equal(packet.Fields[32], []byte{99, 99}) {
		t.Fatal("external identity/presence arrays differ")
	}
}

func TestRoomAndGroupListWireTerminatorsAndCategories(t *testing.T) {
	packet, _ := EncodeRoom(events.New(19, events.Room{Room: rooms.Room{Name: "Lobby", Category: 1, Capacity: 50}, Participants: 7, Category: 2}))
	if packet.Opcode != 701 || packet.Transaction != 19 {
		t.Fatal(packet)
	}
	if category, _ := packet.Short(6); category != 2 {
		t.Fatal("favorite used global category", category)
	}
	packet, _ = EncodeRoom(events.New(20, events.RoomParticipants{Participants: []string{"alice", "bob"}, Moderators: []string{"alice"}}))
	if members, _ := packet.Text(2); members != "alice;bob;" {
		t.Fatal(members)
	}
	if roles, _ := packet.Text(3); roles != "alice;" {
		t.Fatal(roles)
	}
	if muted, present := packet.Text(4); !present || muted != "" {
		t.Fatal("empty role field changed")
	}
	packet, _ = EncodeGroup(events.New(21, events.GroupParticipants{Members: []string{"alice", "bob"}}))
	if packet.Opcode != 755 {
		t.Fatal(packet.Opcode)
	}
	if members, _ := packet.Text(2); members != "alice;bob;" {
		t.Fatal(members)
	}
	for _, peer := range []string{"", "alice"} {
		packet, _ = EncodeGroup(events.New(22, events.GroupCreated{ID: "g-one", Peer: peer}))
		if _, present := packet.Fields[2]; present != (peer != "") {
			t.Fatalf("group conversion peer presence changed: %q", peer)
		}
	}
}
