package repositories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
)

// queries binds generated SQL to either the database or the caller's transaction.
// Transaction ownership and domain validation remain in the repository methods.
func queries(db sqlc.DBTX) *sqlc.Queries { return sqlc.New(db) }

func boolInt64(value bool) int64 {
	if value {
		return 1
	}

	return 0
}

// queryError maps missing rows to the domain contract while preserving wrapped
// cancellation, deadline and other storage errors for errors.Is at the caller.
func queryError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	// SQLite may finish a canceled lock wait with SQLITE_BUSY, and database/sql
	// can report ErrTxDone after its cancellation rollback wins the race. Keep
	// that cause available while also exposing the caller's cancellation.
	if cause := ctx.Err(); cause != nil && !errors.Is(err, cause) {
		return errors.Join(cause, err)
	}

	return err
}
