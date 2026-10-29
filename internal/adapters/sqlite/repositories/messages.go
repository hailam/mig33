package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/formatting"
)

// Message stores mig33 text and the original conversation selector. Action
// distinguishes server-expanded action text from ordinary chat; Body already
// contains the trusted actor's name and complete display text in that case.
// Kind 1 addresses a private recipient username; Kind 2 addresses a group ID;
// Kind 3 addresses a public room's canonical name.
// ID and CreatedAt are assigned by SaveMessage. Color is optional.
type Message = chat.Message

// SaveMessage commits the message and its immutable audience before returning.
// For a private message, recipients must contain exactly its destination after
// case normalization and deduplication. For a group message, recipients is
// ignored: current membership and sender authorization are read in this same
// transaction. The sender always receives historical access. An Action also
// gives the sender pending delivery, since the native client suppresses local
// echo for slash commands. Other group sends do not echo to the sender.
// Room recipients are
// a snapshot of joined accounts supplied by the caller, which must authorize
// the sender's live session membership. Room messages never enter Pending.
// Recipients blocking the sender get neither delivery nor a new audience grant;
// sender history remains and saving succeeds even if every recipient blocks.
func (s *Store) SaveMessage(ctx context.Context, message Message, recipients []string) (Message, error) {
	var err error
	message.Sender, err = username(message.Sender)
	if err != nil {
		return Message{}, queryError(ctx, err)
	}
	if message.Body == "" || !utf8.ValidString(message.Body) {
		return Message{}, fmt.Errorf("%w: empty or invalid UTF-8 message", ErrInvalid)
	}
	if len(message.Body) > 64*1024 {
		return Message{}, fmt.Errorf("%w: message exceeds 65536 bytes", ErrLimit)
	}
	if message.Markup != "" {
		doc, err := formatting.ParseChat(message.Markup)
		if err != nil || message.Action || message.PhotoID != 0 || formatting.PlainText(doc) != message.Body {
			return Message{}, fmt.Errorf("%w: invalid formatted message", ErrInvalid)
		}

		message.Color = formatting.UniformColor(doc)
	}

	var names []string
	switch message.Kind {
	case 1:
		message.Conversation, err = username(message.Conversation)
		if err != nil {
			return Message{}, queryError(ctx, err)
		}

		names, err = memberNames(recipients)
		if err != nil {
			return Message{}, queryError(ctx, err)
		}
		if len(names) != 1 || names[0] != message.Conversation {
			return Message{}, fmt.Errorf("%w: private recipients must match destination", ErrInvalid)
		}
	case 2:
		if err := groupID(message.Conversation); err != nil {
			return Message{}, queryError(ctx, err)
		}
	case 3:
		names, err = memberNames(recipients)
		if err != nil {
			return Message{}, queryError(ctx, err)
		}
	default:
		return Message{}, fmt.Errorf("%w: unsupported conversation kind", ErrInvalid)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Message{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	if message.RequestKey != "" {
		if len(message.RequestKey) > 128 || len(message.RequestHash) != 64 {
			return Message{}, ErrInvalid
		}

		previous, err := requestedMessage(ctx, tx, message.Sender, message.RequestKey, message.RequestHash)
		if err == nil {
			return previous, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Message{}, queryError(ctx, err)
		}
	}
	if message.PhotoID != 0 {
		if message.Action || message.PhotoID < 0 {
			return Message{}, ErrInvalid
		}

		owner, err := queries(tx).MessagePhotoOwner(ctx, message.PhotoID)
		if err != nil {
			return Message{}, queryError(ctx, err)
		}
		if owner != message.Sender {
			return Message{}, ErrForbidden
		}
	}
	if message.Kind == 2 {
		if err := authorizeGroup(ctx, tx, message.Conversation, message.Sender); err != nil {
			return Message{}, queryError(ctx, err)
		}

		group, err := readGroup(ctx, tx, message.Conversation)
		if err != nil {
			return Message{}, queryError(ctx, err)
		}

		names = group.Members
	}
	if message.Kind == 3 {
		room, err := readRoom(ctx, tx, message.Conversation)
		if err != nil {
			return Message{}, queryError(ctx, err)
		}

		message.Conversation = room.Name
		muted, err := roomAccountFlag(ctx, tx, "room_mutes", room.Name, message.Sender)
		if err != nil {
			return Message{}, queryError(ctx, err)
		}
		if muted {
			return Message{}, ErrMuted
		}
	}

	message.CreatedAt = time.Now().UTC().Truncate(time.Millisecond)
	var color sql.NullInt64
	if message.Color != nil {
		color = sql.NullInt64{Int64: int64(*message.Color), Valid: true}
	}

	message.ID, err = queries(tx).CreateMessage(ctx, sqlc.CreateMessageParams{
		Kind:         int64(message.Kind),
		Conversation: message.Conversation,
		Sender:       message.Sender,
		Body:         message.Body,
		Color:        color,
		CreatedAt:    message.CreatedAt.UnixMilli(),
		Action:       boolInt64(message.Action),
		PhotoID:      message.PhotoID,
		Markup:       message.Markup,
	})
	if err != nil {
		return Message{}, queryError(ctx, err)
	}
	if err := queries(tx).AddMessageAuthor(ctx, sqlc.AddMessageAuthorParams{MessageID: message.ID, Username: message.Sender, Recipient: boolInt64(message.Action)}); err != nil {
		return Message{}, queryError(ctx, err)
	}

	for _, name := range names {
		if message.Kind != 1 && name == message.Sender {
			continue
		}

		blocked, err := isBlocked(ctx, tx, name, message.Sender)
		if err != nil {
			return Message{}, queryError(ctx, err)
		}
		if blocked {
			continue
		}
		if err := queries(tx).AddMessageRecipient(ctx, sqlc.AddMessageRecipientParams{MessageID: message.ID, Username: name}); err != nil {
			return Message{}, queryError(ctx, err)
		}
	}
	if message.RequestKey != "" {
		if err := queries(tx).AddMessageRequest(ctx, sqlc.AddMessageRequestParams{
			Username:    message.Sender,
			RequestKey:  message.RequestKey,
			RequestHash: message.RequestHash,
			MessageID:   message.ID,
		}); err != nil {
			return Message{}, queryError(ctx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Message{}, queryError(ctx, err)
	}

	message.RequestKey, message.RequestHash = "", ""
	return message, nil
}

// Pending returns messages not yet confirmed written by the caller's transport,
// oldest first. Reading or putting a message into an in-memory queue does not
// acknowledge delivery. Reconnects can therefore replay an uncertain private
// or group write. Public room traffic is live-only and always excluded.
func (s *Store) Pending(ctx context.Context, viewer string, limit int) ([]Message, error) {
	viewer, err := username(viewer)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	rows, err := queries(s.db).PendingMessages(ctx, sqlc.PendingMessagesParams{Viewer: viewer, Count: int64(boundedLimit(limit))})
	if err != nil {
		return nil, queryError(ctx, err)
	}

	return messageModels(rows), nil
}

// MarkDelivered records a successful transport write for one recipient. It is
// idempotent. It does not imply the user read the message. A sender-only audience
// entry or an unrelated username cannot acknowledge another user's delivery.
func (s *Store) MarkDelivered(ctx context.Context, viewer string, id int64) error {
	viewer, err := username(viewer)
	if err != nil {
		return queryError(ctx, err)
	}
	if id <= 0 {
		return fmt.Errorf("%w: message ID", ErrInvalid)
	}

	count, err := queries(s.db).MarkMessageDelivered(ctx, sqlc.MarkMessageDeliveredParams{
		Now:       sql.NullInt64{Int64: time.Now().UnixMilli(), Valid: true},
		MessageID: id,
		Username:  viewer,
	})
	if err != nil {
		return queryError(ctx, err)
	}
	if count == 0 {
		return ErrNotFound
	}

	return nil
}

// History returns messages whose IDs exceed afterID in ascending order. Private
// conversation is the other user's username. Group history requires current
// membership or at least one historical audience entry, and each returned row
// still requires its own audience grant. Leaving/rejoining never grants access
// to messages sent while absent. Room history also filters each row by audience;
// the caller must enforce current session membership before requesting it.
// Limits default to 100 and are capped at 1000.
func (s *Store) History(ctx context.Context, viewer string, kind byte, conversation string, afterID int64, limit int) ([]Message, error) {
	viewer, err := username(viewer)
	if err != nil {
		return nil, queryError(ctx, err)
	}
	if afterID < 0 {
		return nil, fmt.Errorf("%w: history cursor", ErrInvalid)
	}
	if kind == 1 {
		conversation, err = username(conversation)
		if err != nil {
			return nil, queryError(ctx, err)
		}

		rows, err := queries(s.db).PrivateMessageHistory(ctx, sqlc.PrivateMessageHistoryParams{
			Viewer:  viewer,
			AfterID: afterID,
			Peer:    conversation,
			Count:   int64(boundedLimit(limit)),
		})
		if err != nil {
			return nil, queryError(ctx, err)
		}

		return messageModels(rows), nil
	}
	if kind == 3 {
		room, err := s.Room(ctx, conversation)
		if err != nil {
			return nil, queryError(ctx, err)
		}

		rows, err := queries(s.db).RoomMessageHistory(ctx, sqlc.RoomMessageHistoryParams{Viewer: viewer, Room: room.Name, AfterID: afterID, Count: int64(boundedLimit(limit))})
		if err != nil {
			return nil, queryError(ctx, err)
		}

		return messageModels(rows), nil
	}
	if kind != 2 {
		return nil, fmt.Errorf("%w: unsupported conversation kind", ErrInvalid)
	}
	if err := groupID(conversation); err != nil {
		return nil, queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, queryError(ctx, err)
	}
	defer tx.Rollback()

	if err := authorizeGroup(ctx, tx, conversation, viewer); err != nil {
		if !errors.Is(err, ErrNotMember) {
			return nil, queryError(ctx, err)
		}

		historical, err := queries(tx).HasGroupMessageHistory(ctx, sqlc.HasGroupMessageHistoryParams{GroupID: conversation, Viewer: viewer})
		if err != nil {
			return nil, queryError(ctx, err)
		}
		if !historical {
			return nil, ErrNotMember
		}
	}

	rows, err := queries(tx).GroupMessageHistory(ctx, sqlc.GroupMessageHistoryParams{
		Viewer:  viewer,
		GroupID: conversation,
		AfterID: afterID,
		Count:   int64(boundedLimit(limit)),
	})
	if err != nil {
		return nil, queryError(ctx, err)
	}

	messages := messageModels(rows)

	if err := tx.Commit(); err != nil {
		return nil, queryError(ctx, err)
	}

	return messages, nil
}

func messageModel(row sqlc.Message) Message {
	m := Message{
		ID:           row.ID,
		Kind:         byte(row.Kind),
		Conversation: row.Conversation,
		Sender:       row.Sender,
		Body:         row.Body,
		Markup:       row.Markup,
		Action:       row.Action != 0,
		PhotoID:      row.PhotoID,
		CreatedAt:    time.UnixMilli(row.CreatedAt).UTC(),
	}
	if row.Color.Valid {
		v := int32(row.Color.Int64)
		m.Color = &v
	}

	return m
}

func messageModels(rows []sqlc.Message) []Message {
	result := make([]Message, 0, len(rows))
	for _, row := range rows {
		result = append(result, messageModel(row))
	}

	return result
}
