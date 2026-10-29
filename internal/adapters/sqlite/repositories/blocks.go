package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
)

// SetBlocked changes a directed personal block. Blocking atomically cancels
// existing undelivered incoming messages from target in every conversation kind.
// Audience rows remain for history. Unblocking never restores canceled delivery.
// The caller authenticates owner and validates that target is an existing account.
func (s *Store) SetBlocked(ctx context.Context, owner, target string, blocked bool) error {
	owner, target, err := blockUsers(owner, target)
	if err != nil {
		return queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	if blocked {
		if err := queries(tx).AddPersonalBlock(ctx, sqlc.AddPersonalBlockParams{Owner: owner, Target: target}); err != nil {
			return queryError(ctx, err)
		}
		if err := queries(tx).CancelBlockedDeliveries(ctx, sqlc.CancelBlockedDeliveriesParams{
			Now:    sql.NullInt64{Int64: time.Now().UnixMilli(), Valid: true},
			Owner:  owner,
			Target: target,
		}); err != nil {
			return queryError(ctx, err)
		}
	} else {
		if err := queries(tx).RemovePersonalBlock(ctx, sqlc.RemovePersonalBlockParams{Owner: owner, Target: target}); err != nil {
			return queryError(ctx, err)
		}
	}

	return queryError(ctx, tx.Commit())
}

// IsBlocked reports whether owner blocks target, independently of the reverse
// direction. Usernames are normalized; requesting a self-block is invalid.
func (s *Store) IsBlocked(ctx context.Context, owner, target string) (bool, error) {
	owner, target, err := blockUsers(owner, target)
	if err != nil {
		return false, queryError(ctx, err)
	}

	return isBlocked(ctx, s.db, owner, target)
}

// BlockedUsers returns owner's blocked usernames in sorted order.
func (s *Store) BlockedUsers(ctx context.Context, owner string) ([]string, error) {
	owner, err := username(owner)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	value, err := queries(s.db).ListPersonalBlocks(ctx, owner)
	return value, queryError(ctx, err)
}

func isBlocked(ctx context.Context, db sqlc.DBTX, owner, target string) (bool, error) {
	value, err := queries(db).IsPersonalBlocked(ctx, sqlc.IsPersonalBlockedParams{Owner: owner, Target: target})
	return value, queryError(ctx, err)
}

func blockUsers(owner, target string) (string, string, error) {
	owner, err := username(owner)
	if err != nil {
		return "", "", err
	}

	target, err = username(target)
	if err != nil {
		return "", "", err
	}
	if owner == target {
		return "", "", fmt.Errorf("%w: cannot block yourself", ErrInvalid)
	}

	return owner, target, nil
}
