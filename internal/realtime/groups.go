package realtime

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
)

// groupOperation runs with Server.mu held. These operations implement private
// group conversations (500/3=2), not public chat rooms (500/3=3).
func (s *Coordinator) groupOperation(ctx context.Context, x *session, p *commands.Request) []*events.Event {
	input := p.Input.(commands.Group)
	service, ok := input.Service.Get()
	if !ok || service != 1 {
		return []*events.Event{failure(p, "Only mig33 group chats are supported")}
	}
	id, ok := input.Conversation.Get()
	if !ok || id == "" {
		return []*events.Event{failure(p, "A conversation is required")}
	}

	ack := events.Ack(p.Correlation)
	switch input.Operation {
	case commands.GroupCreate:
		peer := strings.ToLower(id)
		invitee, exists := input.Invitee.Get()
		invitee = strings.ToLower(invitee)
		if !exists || s.accounts[peer] == nil || s.accounts[invitee] == nil {
			return []*events.Event{failure(p, "Choose two registered users for the group chat")}
		}
		if peer == x.username || invitee == x.username || peer == invitee {
			return []*events.Event{failure(p, "Choose two different users other than yourself")}
		}
		if !s.canContact(ctx, x.username, peer) || !s.canContact(ctx, x.username, invitee) {
			return []*events.Event{failure(p, "Recipient privacy settings do not allow this group invitation")}
		}
		g, err := s.chatService.CreateGroup(ctx, "g"+token(), x.username, []string{peer, invitee})
		if err != nil {
			return []*events.Event{groupFailure(p, err)}
		}
		text := x.username + " started a group chat with " + peer + " and " + invitee + "."
		// An unsolicited 750 converts an existing private tab only. A following
		// group 500 creates the tab when there was no private conversation.
		s.enqueue(peer, events.New(0, events.GroupCreated{ID: g.ID, Peer: x.username}))
		s.groupSystem(g, text, x.username)
		// Other sessions of the creator need a group tab, but must not consume
		// the transaction belonging to this connection's private conversation.
		for _, other := range s.sessions {
			if other != x && other.authenticated && other.ready && other.username == x.username && time.Since(other.touched) < sessionTTL {
				select {
				case other.push <- groupSystemPacket(g.ID, text):
				default:
				}
			}
		}
		// Keep these together in the direct response. Enqueuing the second
		// packet would let a concurrent HTTP poll overtake the correlated 750.
		return []*events.Event{
			events.New(p.Correlation, events.GroupCreated{ID: g.ID}),
			groupSystemPacket(g.ID, text),
		}
	case commands.GroupInvite:
		g, err := s.chatService.Group(ctx, id, x.username)
		if err != nil {
			return []*events.Event{groupFailure(p, err)}
		}
		invitee, exists := input.Invitee.Get()
		invitee = strings.ToLower(invitee)
		if !exists || s.accounts[invitee] == nil {
			return []*events.Event{failure(p, "Choose a registered user to invite")}
		}
		if invitee == x.username {
			return []*events.Event{failure(p, "You are already in this group chat")}
		}
		for _, member := range g.Members {
			if member == invitee {
				return []*events.Event{ack}
			}
		}
		if !s.canContact(ctx, x.username, invitee) {
			return []*events.Event{failure(p, "Recipient privacy settings do not allow this group invitation")}
		}
		g, err = s.chatService.Invite(ctx, id, x.username, []string{invitee})
		if err != nil {
			return []*events.Event{groupFailure(p, err)}
		}
		s.groupSystem(g, x.username+" invited "+invitee+" to the group chat.", "")
		return []*events.Event{ack}
	case commands.GroupLeave:
		g, err := s.chatService.Leave(ctx, id, x.username)
		if err != nil {
			return []*events.Event{groupFailure(p, err)}
		}
		s.cancelGroupDeliveries(x.username, id)
		// The native client already closed the tab. Sending it a group 500
		// after this acknowledgment would reopen the conversation.
		s.groupSystem(g, x.username+" left the group chat.", x.username)
		return []*events.Event{ack}
	case commands.GroupParticipants:
		g, err := s.chatService.Group(ctx, id, x.username)
		if err != nil {
			return []*events.Event{groupFailure(p, err)}
		}
		// The original parser consumes only semicolon-terminated entries.
		return []*events.Event{events.New(p.Correlation, events.GroupParticipants{Members: append([]string(nil), g.Members...)})}
	default:
		return []*events.Event{failure(p, "Unknown group operation")}
	}
}

func groupFailure(p *commands.Request, err error) *events.Event {
	switch {
	case errors.Is(err, faults.ErrNotFound), errors.Is(err, faults.ErrForbidden):
		return failure(p, "Group chat unavailable or you are not a member")
	case errors.Is(err, faults.ErrLimit):
		return failure(p, "The group chat has reached its member limit")
	case errors.Is(err, faults.ErrBlocked):
		return failure(p, "Unable to invite this user")
	case errors.Is(err, faults.ErrInvalid):
		return failure(p, "Invalid group chat request")
	default:
		return failure(p, "The group chat could not be updated; try again")
	}
}

func groupSystemPacket(id, text string) *events.Event {
	return events.New(0, events.Notice{Kind: 2, Target: id, Text: text})
}

// groupSystem sends a transient membership notice to current members. Durable
// membership is recovered at login even if a disconnected/full queue misses it.
func (s *Coordinator) groupSystem(g chat.Group, text, excludeUsername string) {
	for _, member := range g.Members {
		if member != excludeUsername {
			s.enqueue(member, groupSystemPacket(g.ID, text))
		}
	}
}
