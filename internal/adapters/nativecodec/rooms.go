package nativecodec

import (
	"math"
	"strings"

	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func DecodeRoom(p *protocol.Packet) (*commands.Request, bool) {
	if p == nil {
		return nil, false
	}

	operation, ok := map[int16]commands.RoomOperation{
		713: commands.RoomCategories,
		716: commands.RoomList,
		703: commands.RoomJoin,
		704: commands.RoomLeave,
		706: commands.RoomKick,
		707: commands.RoomParticipants,
		709: commands.RoomMute,
		710: commands.RoomUnmute,
		711: commands.RoomFavoriteAdd,
		712: commands.RoomFavoriteRemove,
	}[p.Opcode]
	if !ok {
		return nil, false
	}

	nameTag := int16(1)
	if operation == commands.RoomFavoriteRemove {
		nameTag = 2
	}

	input := commands.Room{
		Operation: operation,
		Name:      decodeFeatureField(p, nameTag, p.Text),
		Target:    decodeFeatureField(p, 2, p.Text),
		Category:  decodeFeatureField(p, 1, p.Short),
		Refresh:   decodeFeatureField(p, 2, p.Byte),
	}
	return &commands.Request{Correlation: p.Transaction, Input: input}, true
}

func EncodeRoom(event *events.Event) (*protocol.Packet, bool) {
	switch data := event.Data.(type) {
	case events.RoomCategory:
		favorite := byte(0)
		if data.Favorite {
			favorite = 1
		}

		return protocol.New(714, event.Correlation).SetShort(1, data.ID).SetText(2, data.Name).SetByte(3, 1).SetByte(4, 1).SetByte(5, favorite), true
	case events.RoomCategoriesComplete:
		return protocol.New(715, event.Correlation), true
	case events.RoomListingComplete:
		return protocol.New(717, event.Correlation).SetText(1, data.Action), true
	case events.Room:
		packet := protocol.New(701, event.Correlation).SetText(1, data.Room.Name).SetInt(3, int32(data.Room.Capacity)).SetInt(4, int32(data.Participants)).SetShort(6, data.Category)
		// Original bc's703 reply passes tag8 to s.a; aj.U enables Group
		// Menu when positive and its pages append that value as community cid.
		if data.CommunityID > 0 && data.CommunityID <= math.MaxInt32 {
			packet.SetInt(8, int32(data.CommunityID))
		}
		return packet, true
	case events.RoomParticipants:
		return protocol.New(708, event.Correlation).SetText(2, strings.Join(data.Participants, ";")+";").SetText(3, terminatedNames(data.Moderators)).SetText(4, terminatedNames(data.Muted)).SetByte(5, 1), true
	case events.RoomKicked:
		return protocol.New(5, event.Correlation).SetByte(1, 0).SetText(2, "You were kicked from "+data.Room+" by "+data.Actor+".").SetByte(3, 1).SetText(4, "Chat room"), true
	}

	return nil, false
}

func terminatedNames(names []string) string {
	if len(names) == 0 {
		return ""
	}

	return strings.Join(names, ";") + ";"
}
