package realtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
)

func (s *Coordinator) recipientAllowed(ctx context.Context, x *session, kind byte, target string) error {
	if kind == 1 && s.accounts[target] == nil {
		return fmt.Errorf("Unknown recipient")
	}
	if kind == 1 && !s.canContact(ctx, x.username, target) {
		return fmt.Errorf("Recipient privacy settings do not allow this direct message")
	}
	return nil
}

// message adapts the already decoded native composer command to the shared send
// workflow. Packet bytes and response encoding remain outside this package.
func (s *Coordinator) message(ctx context.Context, x *session, p *commands.Request) (reply *events.Event) {
	if response, handled := s.nativeProviderMessage(ctx, x, p); handled {
		return response
	}

	input := p.Input.(commands.SendMessage)
	channel, _ := input.Channel.Get()
	kind, _ := input.Kind.Get()
	content, _ := input.Content.Get()
	if channel != 1 || (kind != 1 && kind != 2 && kind != 3) || (content != 1 && content != 2 && content != 5) {
		return failure(p, "Use mig33 text or a scrapbook photo")
	}

	target := input.Target.Value
	if kind == 1 {
		target = strings.ToLower(target)
	}
	if err := s.recipientAllowed(ctx, x, kind, target); err != nil {
		return failure(p, err.Error())
	}

	body, ok := input.Body.Get()
	var photoID int64
	if content == 2 || content == 5 {
		var err error
		photoID, err = s.prepareChatPhoto(ctx, x, p, content)
		if err != nil {
			return failure(p, "Photo unavailable; select one of your scrapbook photos")
		}
		if content == 2 {
			defer func() {
				if !events.Successful(reply) {
					if err := s.mediaService.DeletePhoto(ctx, x.username, photoID); err != nil {
						s.logError(ctx, "remove rejected photo upload: %v", err)
					}
				}
			}()
		}
		body, ok = "Photo "+photoToken(photoID), true
	}
	if !ok {
		return failure(p, "Message must contain 1 to 65536 UTF-8 bytes")
	}

	message := chat.Message{
		Kind:         kind,
		Conversation: target,
		Sender:       x.username,
		Body:         body,
		PhotoID:      photoID,
		Markup:       input.Markup,
		RequestKey:   input.RequestKey,
		RequestHash:  input.RequestHash,
	}
	if color, ok := input.Color.Get(); ok {
		message.Color = &color
	}
	if _, err := s.sendPrepared(ctx, x, p, message); err != nil {
		return failure(p, err.Error())
	}
	return events.Ack(p.Correlation)
}

// sendPrepared is the shared typed application operation used by native, JSON
// and WAP composers. Its result is a committed message, not a native ACK.
func (s *Coordinator) sendPrepared(ctx context.Context, x *session, request *commands.Request, message chat.Message) (chat.Message, error) {
	if err := chat.ValidateMessage(&message); err != nil {
		return chat.Message{}, err
	}
	if err := s.catalogService.ValidateEmoticons(ctx, x.username, message.Body); err != nil {
		return chat.Message{}, err
	}

	if message.Markup == "" {
		if result, handled := s.giftCommand(ctx, x, request, message); handled {
			if !events.Successful(result) {
				return chat.Message{}, fmt.Errorf("%s", events.Error(result))
			}
			return chat.Message{}, nil
		}
		if result, handled := s.roomControlCommand(ctx, x, request, message); handled {
			if !events.Successful(result) {
				return chat.Message{}, fmt.Errorf("%s", events.Error(result))
			}
			return chat.Message{}, nil
		}
		if err := s.expandCommand(ctx, x, &message); err != nil {
			return chat.Message{}, err
		}
	}

	if message.Kind == 3 {
		return s.roomMessage(ctx, x, message)
	}

	saved, err := s.chatService.Accept(ctx, message, []string{message.Conversation})
	if err != nil {
		if errors.Is(err, faults.ErrNotMember) || errors.Is(err, faults.ErrNotFound) {
			return chat.Message{}, fmt.Errorf("Group chat unavailable or you are not a member")
		}
		s.logError(ctx, "save chat message: %v", err)
		return chat.Message{}, fmt.Errorf("Message could not be saved; try again")
	}

	if !saved.Action && !x.webOnly {
		if x.ownSkipped == nil {
			x.ownSkipped = map[int64]bool{}
		}
		x.ownSkipped[saved.ID] = true
	}

	for _, other := range s.sessions {
		s.flushPending(ctx, other)
	}
	return saved, nil
}

func (s *Coordinator) expandCommand(ctx context.Context, x *session, m *chat.Message) error {
	if !strings.HasPrefix(strings.TrimSpace(m.Body), "/") {
		return nil
	}

	var members []string
	switch m.Kind {
	case 1:
		members = []string{x.username, m.Conversation}
	case 2:
		g, err := s.chatService.Group(ctx, m.Conversation, x.username)
		if err != nil {
			return fmt.Errorf("Group chat unavailable or you are not a member")
		}
		members = g.Members
	case 3:
		room, err := s.roomsService.Room(ctx, m.Conversation)
		if err != nil || !x.rooms[room.Name] {
			return fmt.Errorf("Join the chat room before using commands")
		}
		m.Conversation = room.Name
		muted, err := s.roomsService.IsRoomMuted(ctx, room.Name, x.username)
		if err != nil {
			return fmt.Errorf("Room permissions could not be checked; try again")
		}
		if muted {
			return fmt.Errorf("You are muted in this chat room")
		}
		members = s.roomParticipants(room.Name)
	}
	return chat.ExpandCommand(m, members, s.accountNames())
}
