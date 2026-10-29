package nativecodec

import (
	"strings"

	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func DecodeGroup(p *protocol.Packet) (*commands.Request, bool) {
	if p == nil {
		return nil, false
	}

	operation, ok := map[int16]commands.GroupOperation{
		751: commands.GroupCreate,
		752: commands.GroupInvite,
		753: commands.GroupLeave,
		754: commands.GroupParticipants,
	}[p.Opcode]
	if !ok {
		return nil, false
	}

	serviceTag := int16(2)
	if operation == commands.GroupCreate || operation == commands.GroupInvite {
		serviceTag = 3
	}

	input := commands.Group{
		Operation:    operation,
		Conversation: decodeFeatureField(p, 1, p.Text),
		Invitee:      decodeFeatureField(p, 2, p.Text),
		Service:      decodeFeatureField(p, serviceTag, p.Byte),
	}
	return &commands.Request{Correlation: p.Transaction, Input: input}, true
}

func EncodeGroup(event *events.Event) (*protocol.Packet, bool) {
	switch data := event.Data.(type) {
	case events.GroupCreated:
		p := protocol.New(750, event.Correlation).SetText(1, data.ID).SetByte(3, 1)
		if data.Peer != "" {
			p.SetText(2, data.Peer)
		}

		return p, true
	case events.GroupParticipants:
		return protocol.New(755, event.Correlation).SetText(2, strings.Join(data.Members, ";")+";").SetByte(5, 1), true
	}

	return nil, false
}
