package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
	"github.com/hailam/mig33/internal/commerce"
)

// ValidateGiftNote keeps notes literal and prevents embedded gift, photo or
// catalog-pack receipt triggers. Native emoticon ownership is checked upstream.
func ValidateGiftNote(note string) error { return commerce.ValidateGiftNote(note) }

// allowGift uses the same everyone/friends/nobody and directional block policy
// as chat, under the debit transaction so a concurrent privacy change cannot
// leave a paid gift without its intended recipient audience.
func allowGift(ctx context.Context, tx *sql.Tx, actor, recipient string) error {
	blocked, err := isBlocked(ctx, tx, recipient, actor)
	if err != nil {
		return queryError(ctx, err)
	}
	if blocked {
		return ErrBlocked
	}

	privacy, err := queries(tx).GetIdentityPreference(ctx, sqlc.GetIdentityPreferenceParams{Username: recipient, Name: "chat_privacy"})
	if errors.Is(err, sql.ErrNoRows) || (err == nil && privacy == "everyone") {
		return nil
	}
	if err != nil {
		return queryError(ctx, err)
	}
	if privacy == "friends" {
		contact, err := queries(tx).IsGiftContact(ctx, sqlc.IsGiftContactParams{Owner: recipient, Username: actor})
		if err != nil {
			return queryError(ctx, err)
		}
		if contact {
			return nil
		}
	}

	return ErrForbidden
}

// GiftNote exposes notes only to the sender or recipient of the gift receipt.
func (s *Store) GiftNote(ctx context.Context, operationID int64, viewer string) (string, error) {
	note, err := queries(s.db).GetAuthorizedGiftNote(ctx, sqlc.GetAuthorizedGiftNoteParams{ID: operationID, Viewer: viewer})
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}

	return note, queryError(ctx, err)
}

type GiftRatingSummary = commerce.GiftRatingSummary

func giftRating(ctx context.Context, db sqlc.DBTX, item, owner string) (GiftRatingSummary, error) {
	row, err := queries(db).GetGiftRating(ctx, sqlc.GetGiftRatingParams{Username: owner, Item: item})
	return GiftRatingSummary{Count: int(row.Count), Average: row.Average, OwnRating: int(row.OwnRating)}, queryError(ctx, err)
}

func (s *Store) GiftRating(ctx context.Context, item, owner string) (GiftRatingSummary, error) {
	product, ok := LookupStoreItem(item)
	if !ok || product.Kind != "gift" {
		return GiftRatingSummary{}, ErrInvalid
	}

	return giftRating(ctx, s.db, item, owner)
}

// RateGift uses an explicit local 1–5 integer scale. One current vote per
// account/item makes retries harmless; a later rating replaces that vote.
func (s *Store) RateGift(ctx context.Context, owner, item string, rating int) (GiftRatingSummary, error) {
	owner, err := username(owner)
	if err != nil {
		return GiftRatingSummary{}, queryError(ctx, err)
	}

	product, ok := LookupStoreItem(item)
	if !ok || product.Kind != "gift" || rating < 1 || rating > 5 {
		return GiftRatingSummary{}, ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GiftRatingSummary{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	exists, err := queries(tx).GiftRatingOwnerExists(ctx, owner)
	if err != nil {
		return GiftRatingSummary{}, queryError(ctx, err)
	}
	if !exists {
		return GiftRatingSummary{}, ErrNotFound
	}
	if err = queries(tx).SetGiftRating(ctx, sqlc.SetGiftRatingParams{Username: owner, Item: item, Rating: int64(rating), UpdatedAt: time.Now().UnixMilli()}); err != nil {
		return GiftRatingSummary{}, queryError(ctx, err)
	}

	result, err := giftRating(ctx, tx, item, owner)
	if err != nil {
		return GiftRatingSummary{}, queryError(ctx, err)
	}

	return result, queryError(ctx, tx.Commit())
}
