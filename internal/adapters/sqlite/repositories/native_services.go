package repositories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
)

// SetLanguage records a selection of a complete, recovered language pack.
// Additional codes must only be enabled when their complete packs are present.
func (s *Store) SetLanguage(ctx context.Context, owner, code string) error {
	if code != "en-US" {
		return ErrInvalid
	}
	if _, err := username(owner); err != nil {
		return queryError(ctx, err)
	}

	err := queries(s.db).SetIdentityPreference(ctx, sqlc.SetIdentityPreferenceParams{Username: owner, Name: "language", Value: code})
	return queryError(ctx, err)
}

func (s *Store) Language(ctx context.Context, owner string) (string, error) {
	code, err := queries(s.db).GetIdentityPreference(ctx, sqlc.GetIdentityPreferenceParams{Username: owner, Name: "language"})
	if errors.Is(err, sql.ErrNoRows) {
		return "en-US", nil
	}

	return code, queryError(ctx, err)
}

func (s *Store) ClearPhotoAvatar(ctx context.Context, owner string) error {
	err := queries(s.db).ClearPhotoAvatar(ctx, owner)
	return queryError(

		// RemoveIMLabels deliberately removes only saved labels, never credentials.
		ctx, err)
}

func (s *Store) RemoveIMLabels(ctx context.Context, owner, service string) error {
	err := queries(s.db).RemoveIMLabels(ctx, sqlc.RemoveIMLabelsParams{Owner: owner, Service: service})
	return queryError(

		// HasStoreItem checks the complete inventory, not its bounded display page.
		ctx, err)
}

func (s *Store) HasStoreItem(ctx context.Context, owner, item string) (bool, error) {
	value, err := queries(s.db).HasStoreItem(ctx, sqlc.HasStoreItemParams{Owner: owner, Item: item})
	return value, queryError(ctx, err)
}

// The gift message and receipt share one transaction. A repeated purchase
// nonce returns the prior receipt without creating another message or charge.
func saveGiftMessage(ctx context.Context, tx *sql.Tx, actor, recipient string, item StoreItem, note string, now int64) error {
	body := actor + " sent " + recipient + " a " + item.Name + " [gift:" + item.ID + "]"
	if note != "" {
		body += " Message: " + note
	}

	id, err := queries(tx).CreateMessage(ctx, sqlc.CreateMessageParams{Kind: 1, Conversation: recipient, Sender: actor, Body: body, CreatedAt: now, Action: 1})
	if err != nil {
		return queryError(ctx, err)
	}
	if err = queries(tx).AddMessageAuthor(ctx, sqlc.AddMessageAuthorParams{MessageID: id, Username: actor, Recipient: 1}); err != nil {
		return queryError(ctx, err)
	}

	blocked, err := isBlocked(ctx, tx, recipient, actor)
	if err != nil {
		return queryError(ctx, err)
	}
	if !blocked {
		err = queries(tx).AddMessageRecipient(ctx, sqlc.AddMessageRecipientParams{MessageID: id, Username: recipient})
	}

	return queryError(ctx, err)
}
