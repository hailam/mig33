// Package migrations applies embedded SQLite schema upgrades atomically.
package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	catalog "github.com/hailam/mig33/db/migrations"
)

// Apply upgrades a database to the current schema in one transaction. A failed
// step rolls back both data/schema changes and the recorded schema version.
// The caller configures its connection's foreign keys and transaction mode.
func Apply(ctx context.Context, db *sql.DB) (err error) {
	defer func() { err = operationError(ctx, err) }()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("chatstore: begin migration: %w", err)
	}
	defer tx.Rollback()

	versionSQL, err := catalog.Support("version")
	if err != nil {
		return err
	}

	var version int
	if err := tx.QueryRowContext(ctx, versionSQL).Scan(&version); err != nil {
		return fmt.Errorf("chatstore: schema version: %w", err)
	}
	if version < 0 || version > catalog.LatestVersion {
		return fmt.Errorf("chatstore: unsupported schema version %d", version)
	}

	for next := version + 1; next <= catalog.LatestVersion; next++ {
		if err := ApplyVersion(ctx, tx, next); err != nil {
			return fmt.Errorf("chatstore: migrate schema to version %d: %w", next, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("chatstore: commit migration: %w", err)
	}

	return nil
}

// ApplyVersion executes one migration within a caller-owned transaction. The
// caller must have applied its predecessor; Apply handles ordering for normal
// startup. Migration fixtures also use this to build genuine older schemas.
func ApplyVersion(ctx context.Context, tx *sql.Tx, version int) (err error) {
	defer func() { err = operationError(ctx, err) }()
	ddl, err := catalog.SQL(version)
	if err != nil {
		return err
	}
	if version == 2 {
		return applyV2(ctx, tx, ddl)
	}

	_, err = tx.ExecContext(ctx, ddl)
	return err
}

// Keep the driver's failure observable when a cancellation races its result.
// Successful work, including a successful commit, remains successful.
func operationError(ctx context.Context, err error) error {
	if err != nil {
		if cause := ctx.Err(); cause != nil && !errors.Is(err, cause) {
			return errors.Join(cause, err)
		}
	}

	return err
}

func applyV2(ctx context.Context, tx *sql.Tx, ddl string) error {
	// V2 rebuilds the message/audience tables while foreign keys remain enabled.
	// Preserve the AUTOINCREMENT high-water mark even if its row was deleted:
	// merely copying surviving IDs could otherwise reuse a historical ID.
	readSequence, err := catalog.Support("002_read_sequence")
	if err != nil {
		return err
	}

	var sequence int64
	if err := tx.QueryRowContext(ctx, readSequence).Scan(&sequence); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, ddl); err != nil {
		return err
	}

	for _, name := range []string{"002_insert_sequence", "002_restore_sequence"} {
		query, err := catalog.Support(name)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, query, sequence); err != nil {
			return err
		}
	}

	check, err := catalog.Support("foreign_key_check")
	if err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, check)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("foreign-key violation after table migration")
	}

	return rows.Err()
}
