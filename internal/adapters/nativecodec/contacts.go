package nativecodec

import (
	"strings"

	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func decodeFeatureField[T any](p *protocol.Packet, tag int16, read func(int16) (T, bool)) commands.Field[T] {
	value, valid := read(tag)
	_, present := p.Fields[tag]
	return commands.Field[T]{Value: value, Present: present, Valid: valid}
}

func DecodeContact(p *protocol.Packet) (*commands.Request, bool) {
	if p == nil {
		return nil, false
	}

	operation, ok := map[int16]commands.ContactOperation{
		400: commands.ContactSnapshot,
		419: commands.ContactChanges,
		405: commands.ContactAdd,
		407: commands.ContactUpdate,
		406: commands.ContactDelete,
		408: commands.ContactGroupAdd,
		409: commands.ContactGroupDelete,
		410: commands.ContactGroupRename,
		411: commands.ContactMove,
	}[p.Opcode]
	if !ok {
		return nil, false
	}

	input := commands.Contact{
		Operation: operation,
		ID:        decodeFeatureField(p, 1, p.Int), GroupID: decodeFeatureField(p, 2, p.Int),
		Username: decodeFeatureField(p, 12, p.Text), Display: decodeFeatureField(p, 3, p.Text), Mobile: decodeFeatureField(p, 8, p.Text), GroupName: decodeFeatureField(p, 2, p.Text),
		ShareMobile: decodeFeatureField(p, 24, p.Byte), Checkpoint: decodeFeatureField(p, 1, p.Long),
	}
	names, namesPresent := p.Fields[30]
	services, servicesPresent := p.Fields[31]
	input.ExternalRequested = namesPresent
	input.External.Present = namesPresent || servicesPresent
	if input.External.Present {
		usernames := strings.Split(string(names), "\x00")
		valid := namesPresent && servicesPresent && len(usernames) == len(services) && len(services) > 0 && len(services) <= 4
		if operation == commands.ContactAdd {
			valid = valid && len(services) == 1 && !strings.ContainsRune(string(names), 0)
		}

		input.External.Valid = valid
		if valid {
			input.External.Value = make([]contacts.ContactIM, len(services))
			for i, service := range services {
				input.External.Value[i] = contacts.ContactIM{Service: service, Username: usernames[i]}
			}
		}
	}

	return &commands.Request{Correlation: p.Transaction, Input: input}, true
}

func EncodeContact(event *events.Event) (*protocol.Packet, bool) {
	switch data := event.Data.(type) {
	case events.Contact:
		c := data.Contact
		p := protocol.New(402, event.Correlation).SetInt(1, c.ID).SetText(3, c.Display)
		if c.Kind == "" || c.Kind == "mig33" {
			p.SetText(12, c.Username).SetByte(13, data.Presence)
		} else if c.Kind == "im" {
			services, presences := make([]byte, len(c.External)), make([]byte, len(c.External))
			names := make([]string, len(c.External))
			for i, e := range c.External {
				services[i] = e.Service
				presences[i] = 99
				names[i] = e.Username
			}

			p.SetBytes(31, services).SetBytes(32, presences).SetText(33, strings.Join(names, "\x00"))
		}
		if c.GroupID != -1 {
			p.SetInt(2, c.GroupID)
		}
		if c.Mobile != "" {
			p.SetText(8, c.Mobile)
		}
		if data.Status != nil {
			// Original v4 bc402: tag27 -> t.a(2) -> t.h, painted as text.
			p.SetText(27, *data.Status)
		}
		if data.Avatar != "" {
			// Tag28 -> t.a(91) -> t.p, consumed by the avatar image loader.
			p.SetText(28, data.Avatar)
		}

		return p, true
	case events.ContactGroup:
		return protocol.New(401, event.Correlation).SetInt(1, data.Group.ID).SetText(2, data.Group.Name), true
	case events.ContactRemoved:
		return protocol.New(406, event.Correlation).SetInt(1, data.ID), true
	case events.ContactPresence:
		return protocol.New(404, event.Correlation).SetInt(1, data.ID).SetByte(2, data.Presence), true
	case events.ContactStatus:
		return protocol.New(421, event.Correlation).SetInt(1, data.ID).SetText(2, data.Status), true
	case events.ContactCheckpoint:
		return protocol.New(420, event.Correlation).SetInt(1, data.Revision).SetLong(2, data.Checkpoint), true
	case events.ContactsComplete:
		return protocol.New(403, event.Correlation), true
	}

	return nil, false
}
