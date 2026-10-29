package realtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/faults"
)

// Room traffic is live: joining a room subscribes only the current session.
// Messages still have durable history, but are never replayed as offline chat.
func (s *Coordinator) roomMessage(ctx context.Context, x *session, m chat.Message) (chat.Message, error) {
	room, err := s.roomsService.Room(ctx, m.Conversation)
	if err != nil || !x.rooms[room.Name] {
		return chat.Message{}, fmt.Errorf("Join the chat room before sending messages")
	}
	m.Conversation = room.Name

	_, canSpeak, err := s.roomsService.GroupRoomAccess(ctx, room.Name, x.username)
	if err != nil {
		return chat.Message{}, fmt.Errorf("Room permissions could not be checked")
	}
	if !canSpeak {
		return chat.Message{}, fmt.Errorf("This Group room is silenced")
	}

	muted, err := s.roomsService.IsRoomMuted(ctx, room.Name, x.username)
	if err != nil {
		return chat.Message{}, fmt.Errorf("Room permissions could not be checked; try again")
	}
	if muted {
		return chat.Message{}, fmt.Errorf("You are muted in this chat room")
	}

	participants := s.roomParticipants(room.Name)
	if m.Action && !x.webOnly && len(x.push) == cap(x.push) {
		return chat.Message{}, fmt.Errorf("Your connection is busy; try the command again")
	}

	// Avoid acknowledging a room broadcast when a member has no queue space.
	// The lock prevents competing producers taking these slots before enqueue;
	// readers can only free more space during the storage transaction.
	for _, username := range participants {
		if (username == x.username && !m.Action) || s.blocksIncoming(ctx, username, x.username) {
			continue
		}
		available := false
		for _, other := range s.sessions {
			if other.authenticated && other.username == username && other.rooms[room.Name] &&
				time.Since(other.touched) < sessionTTL && (other.webOnly || len(other.push) < cap(other.push)) {
				available = true
				break
			}
		}
		if !available {
			return chat.Message{}, fmt.Errorf("A room participant's connection is busy; try again")
		}
	}

	m, err = s.chatService.Accept(ctx, m, participants)
	if err != nil {
		if errors.Is(err, faults.ErrMuted) {
			return chat.Message{}, fmt.Errorf("You are muted in this chat room")
		}
		s.logError(ctx, "save room message: %v", err)
		return chat.Message{}, fmt.Errorf("Room message could not be saved; try again")
	}

	for _, other := range s.sessions {
		if !other.authenticated || other.webOnly || (other.id == x.id && !m.Action) ||
			!other.rooms[room.Name] || time.Since(other.touched) >= sessionTTL ||
			s.blocksIncoming(ctx, other.username, x.username) {
			continue
		}
		packet := s.messagePacket(ctx, m, other.username)
		s.reserveEvent(packet)
		select {
		case other.push <- packet:
			other.inflight[m.ID] = packet
			s.reserve(packet, delivery{
				sessionID: other.id,
				username:  other.username,
				actor:     m.Sender,
				messageID: m.ID,
				ownMirror: other.username == m.Sender && !m.Action,
				kind:      m.Kind,
				target:    m.Conversation,
			})
		default:
			// At least one session of this account was checked above. Additional
			// full sessions can retrieve the history once their queues drain.
		}
	}
	return m, nil
}
