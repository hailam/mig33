package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/formatting"
)

// Service owns durable conversation operations. The realtime workflow resolves
// the authenticated actor and checks live room membership before invoking it.
type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func Kind(value string) byte {
	switch value {
	case "private":
		return 1
	case "group":
		return 2
	case "room":
		return 3
	}
	return 0
}

// PrepareInput validates the JSON composer contract without constructing a
// native frame. Its canonical encoding is deliberately unchanged for retries.
func PrepareInput(input MessageInput) (Message, string, error) {
	kind := Kind(input.Kind)
	if kind == 0 || input.Target == "" || input.PhotoID < 0 || (input.PhotoID > 0 && input.Body != "") || (input.Color != nil && (*input.Color < 0 || *input.Color > 0xffffff)) || (input.Markup != "" && (input.Body != "" || input.Color != nil || input.PhotoID != 0)) {
		return Message{}, "", faults.NewError(faults.Invalid, "Invalid message fields")
	}
	if kind == 1 {
		input.Target = strings.ToLower(input.Target)
	}
	if !CoreInput(input.Body) {
		return Message{}, "", faults.NewError(faults.Invalid, "This API accepts ordinary messages and core action commands; use room controls for moderation")
	}

	encoded, _ := json.Marshal(input)
	sum := sha256.Sum256(encoded)
	m := Message{
		Kind:         kind,
		Conversation: input.Target,
		Body:         input.Body,
		Color:        input.Color,
		PhotoID:      input.PhotoID,
		Markup:       input.Markup,
	}
	return m, hex.EncodeToString(sum[:]), nil
}

// ValidateMessage is shared by native and modern composers after any uploaded
// photo has been resolved to its durable ID by the media capability.
func ValidateMessage(m *Message) error {
	if strings.TrimSpace(m.Body) == "" || len(m.Body) > 64*1024 || !utf8.ValidString(m.Body) {
		return faults.NewError(faults.Invalid, "Message must contain 1 to 65536 UTF-8 bytes")
	}
	if m.Markup != "" {
		doc, err := formatting.ParseChat(m.Markup)
		if err != nil || m.PhotoID != 0 || formatting.PlainText(doc) != m.Body {
			return faults.NewError(faults.Invalid, "Invalid formatted chat message")
		}
		m.Color = formatting.UniformColor(doc)
	}
	return nil
}

func (s *Service) Replay(ctx context.Context, actor, key, hash string) (MessageResult, bool, error) {
	m, err := s.repository.RequestedMessage(ctx, actor, key, hash)
	if err == nil {
		return MessageResult{Message: m, Replayed: true}, true, nil
	}
	if errors.Is(err, faults.ErrNotFound) {
		return MessageResult{}, false, nil
	}
	if errors.Is(err, faults.ErrConflict) {
		return MessageResult{}, false, faults.WrapError(faults.Conflict, "Idempotency key was already used for a different message", err)
	}
	return MessageResult{}, false, faults.WrapError(faults.Unavailable, "Chat storage unavailable", err)
}

// Accept persists message, audience, and request identity atomically. The
// adapter must retain its transactional authorization/integrity checks.
func (s *Service) Accept(ctx context.Context, message Message, audience []string) (Message, error) {
	return s.repository.SaveMessage(ctx, message, audience)
}

func (s *Service) Groups(ctx context.Context, actor string) ([]Group, error) {
	groups, err := s.repository.Groups(ctx, actor)
	if err != nil {
		return nil, conversationError(err)
	}
	if groups == nil {
		groups = []Group{}
	}
	return groups, nil
}

func (s *Service) HistoryPage(ctx context.Context, actor string, query HistoryQuery) (HistoryPage, error) {
	var result HistoryPage
	kind := Kind(query.Kind)
	if kind == 0 {
		return result, faults.NewError(faults.Invalid, "kind must be private, group or room")
	}
	if query.AfterID < 0 {
		return result, faults.NewError(faults.Invalid, "Invalid after_id")
	}
	if query.Limit < 1 || query.Limit > 200 || query.Target == "" {
		return result, faults.NewError(faults.Invalid, "target is required; limit must be 1 to 200")
	}

	messages, err := s.repository.History(ctx, actor, kind, query.Target, query.AfterID, query.Limit+1)
	if err != nil {
		return result, conversationError(err)
	}

	result.MessagePage = page(messages, query.PageQuery)
	result.ReadThroughID, err = s.repository.ConversationRead(ctx, actor, kind, query.Target)
	if err != nil {
		return HistoryPage{}, conversationError(err)
	}
	return result, nil
}

func (s *Service) Sync(ctx context.Context, actor string, query PageQuery) (MessagePage, error) {
	if query.AfterID < 0 {
		return MessagePage{}, faults.NewError(faults.Invalid, "Invalid after_id")
	}
	if query.Limit < 1 || query.Limit > 200 {
		return MessagePage{}, faults.NewError(faults.Invalid, "limit must be 1 to 200")
	}

	messages, err := s.repository.Transcript(ctx, actor, query.AfterID, query.Limit+1)
	if err != nil {
		return MessagePage{}, faults.WrapError(faults.Unavailable, "Chat storage unavailable", err)
	}
	return page(messages, query), nil
}

func (s *Service) MarkRead(ctx context.Context, actor string, input ReadInput) (int64, error) {
	kind := Kind(input.Kind)
	if kind == 0 || input.Target == "" || input.ThroughID <= 0 {
		return 0, faults.NewError(faults.Invalid, "Valid kind, target and through_id required")
	}

	if err := s.repository.MarkConversationRead(ctx, actor, kind, input.Target, input.ThroughID); err != nil {
		if errors.Is(err, faults.ErrNotFound) {
			return 0, faults.WrapError(faults.NotFound, "Message unavailable in this conversation", err)
		}
		return 0, faults.WrapError(faults.Invalid, "Read state could not be saved", err)
	}
	id, err := s.repository.ConversationRead(ctx, actor, kind, input.Target)
	if err != nil {
		return 0, faults.WrapError(faults.Unavailable, "Read state unavailable", err)
	}
	return id, nil
}

func conversationError(err error) error {
	switch {
	case errors.Is(err, faults.ErrNotFound), errors.Is(err, faults.ErrNotMember):
		return faults.WrapError(faults.NotFound, "Conversation unavailable", err)
	case errors.Is(err, faults.ErrInvalid):
		return faults.WrapError(faults.Invalid, "Invalid conversation", err)
	default:
		return faults.WrapError(faults.Unavailable, "Chat storage unavailable", err)
	}
}

func page(messages []Message, query PageQuery) MessagePage {
	result := MessagePage{NextAfterID: query.AfterID, HasMore: len(messages) > query.Limit}
	if result.HasMore {
		messages = messages[:query.Limit]
	}
	if messages == nil {
		messages = []Message{}
	}
	result.Messages = messages
	if len(messages) > 0 {
		result.NextAfterID = messages[len(messages)-1].ID
	}
	return result
}
