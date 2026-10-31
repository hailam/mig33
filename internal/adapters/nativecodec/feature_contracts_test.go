package nativecodec

import (
	"bytes"
	"math"
	"reflect"
	"testing"

	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
	"github.com/hailam/mig33/internal/rooms"
)

func TestJoinedRoomCommunityIDUsesOriginalIntTag8(t *testing.T) {
	for _, id := range []int64{0, -1, 37, math.MaxInt32, int64(math.MaxInt32) + 1} {
		packet, _ := EncodeRoom(events.New(31, events.Room{Room: rooms.Room{Name: "Club Room"}, CommunityID: id}))
		value, ok := packet.Int(8)
		if id > 0 && id <= math.MaxInt32 {
			if !ok || int64(value) != id {
				t.Fatalf("community %d must survive original int tag8: %d, present=%v", id, value, ok)
			}
		} else if ok {
			t.Fatalf("unrepresentable community %d became a different native group: %d", id, value)
		}
	}
}

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
	if status, present := packet.Text(27); !present || status != "" {
		t.Fatal("present empty status was lost")
	}
	if names, _ := packet.Text(33); names != "alice\x00bob" {
		t.Fatal(names)
	}
	if !bytes.Equal(packet.Fields[31], []byte{2, 4}) || !bytes.Equal(packet.Fields[32], []byte{99, 99}) {
		t.Fatal("external identity/presence arrays differ")
	}
}

// The unchanged v4 consumer routes402 tag27 and421 through t.a(2) to
// t.h (rendered status); tag28 and423 go through t.a(91) to t.p (avatar).
// Distinct values prevent a mutually swapped producer/consumer from passing.
func TestContactStatusAndAvatarMatchOriginalV4Slots(t *testing.T) {
	status := "Status rendered beside Bob"
	avatar := "photos/42"
	packet, ok := EncodeContact(events.New(19, events.Contact{
		Contact: contacts.Contact{ID: 2, GroupID: -1, Username: "bob", Display: "Bob"},
		Status:  &status, Avatar: avatar,
	}))
	if !ok || packet.Opcode != 402 || packet.Transaction != 19 {
		t.Fatal(packet)
	}
	if value, _ := packet.Text(27); value != status {
		t.Fatalf("original status slot27: %q", value)
	}
	if value, _ := packet.Text(28); value != avatar {
		t.Fatalf("original avatar slot28: %q", value)
	}
	statusPush, _ := EncodeContact(events.New(20, events.ContactStatus{ID: 2, Status: status}))
	if value, _ := statusPush.Text(2); statusPush.Opcode != 421 || value != status {
		t.Fatalf("original status update421: %v", statusPush)
	}
	avatarPush := New(nil).Encode(t.Context(), events.New(21, events.Avatar{ContactID: 2, Key: avatar}), "")
	if value, _ := avatarPush.Text(2); avatarPush.Opcode != 423 || value != avatar {
		t.Fatalf("original avatar update423: %v", avatarPush)
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
