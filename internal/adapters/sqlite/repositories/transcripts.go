package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
)

// Reconstruction API metadata is separate from the legacy transaction number.
// An idempotency key belongs to one account and survives process restarts.

func (s *Store) LatestMessageID(ctx context.Context) (int64, error) {
	value, err := queries(s.db).LatestMessageID(ctx)
	return value, queryError(ctx, err)
}

// OwnMessagesAfter supplies live mirrors to another already connected native
// session. Actions already use the normal outbox; rooms use live subscriptions.
func (s *Store) OwnMessagesAfter(ctx context.Context, viewer string, after int64, limit int) ([]Message, error) {
	rows, err := queries(s.db).OwnMessagesAfter(ctx, sqlc.OwnMessagesAfterParams{Viewer: viewer, AfterID: after, Count: int64(boundedLimit(limit))})
	if err != nil {
		return nil, queryError(ctx, err)
	}

	return messageModels(rows), nil
}

// Transcript returns immutable audience-authorized records for frontend sync.
// Public room history uses its separate endpoint and current room membership.
func (s *Store) Transcript(ctx context.Context, viewer string, after int64, limit int) ([]Message, error) {
	viewer, err := username(viewer)
	if err != nil || after < 0 {
		return nil, ErrInvalid
	}

	rows, err := queries(s.db).MessageTranscript(ctx, sqlc.MessageTranscriptParams{Viewer: viewer, AfterID: after, Count: int64(boundedLimit(limit))})
	if err != nil {
		return nil, queryError(ctx, err)
	}

	return messageModels(rows), nil
}

func requestedMessage(ctx context.Context, db sqlc.DBTX, viewer, key, hash string) (Message, error) {
	row, err := queries(db).RequestedMessage(ctx, sqlc.RequestedMessageParams{Viewer: viewer, RequestKey: key})
	if err != nil {
		return Message{}, queryError(ctx, err)
	}
	if row.RequestHash != hash {
		return Message{}, ErrConflict
	}

	return messageModel(row.Message), nil
}

func (s *Store) RequestedMessage(ctx context.Context, viewer, key, hash string) (Message, error) {
	return requestedMessage(ctx, s.db, viewer, key, hash)
}

// MarkConversationRead is an account-private UI checkpoint, never a claim of
// native delivery or a receipt broadcast to another user. It cannot move back
// or point at a message outside the viewer's immutable conversation audience.
func (s *Store) MarkConversationRead(ctx context.Context, viewer string, kind byte, target string, id int64) error {
	viewer, err := username(viewer)
	if err != nil || id <= 0 || kind < 1 || kind > 3 {
		return ErrInvalid
	}
	if kind == 1 {
		target, err = username(target)
		if err != nil {
			return queryError(ctx, err)
		}
	}
	if kind == 3 {
		room, e := s.Room(ctx, target)
		if e != nil {
			return e
		}

		target = room.Name
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	audience, err := queries(tx).MessageReadAudience(ctx, sqlc.MessageReadAudienceParams{Viewer: viewer, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return queryError(ctx, err)
	}
	if audience.Kind != int64(kind) {
		return ErrNotFound
	}
	if kind == 1 {
		if !((audience.Sender == viewer && audience.Conversation == target) || (audience.Sender == target && audience.Conversation == viewer)) {
			return ErrNotFound
		}
	} else if audience.Conversation != target {
		return ErrNotFound
	}

	err = queries(tx).MarkConversationRead(ctx, sqlc.MarkConversationReadParams{
		Username:     viewer,
		Kind:         int64(kind),
		Conversation: target,
		MessageID:    id,
		UpdatedAt:    time.Now().UnixMilli(),
	})
	if err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func (s *Store) ConversationRead(ctx context.Context, viewer string, kind byte, target string) (int64, error) {
	if kind == 1 {
		target = strings.ToLower(target)
	}

	id, err := queries(s.db).ConversationRead(ctx, sqlc.ConversationReadParams{Username: viewer, Kind: int64(kind), Conversation: target})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}

	return id, queryError(ctx, err)
}
