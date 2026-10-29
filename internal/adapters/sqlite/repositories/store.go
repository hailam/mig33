// Package repositories implements application repository ports using SQLite and sqlc.
// The caller authenticates users and validates that usernames refer to accounts.
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hailam/mig33/internal/adapters/sqlite/migrations"
	"github.com/hailam/mig33/internal/faults"
)

var (
	ErrNotFound  = faults.ErrNotFound
	ErrNotMember = faults.ErrNotMember
	ErrForbidden = faults.ErrForbidden
	ErrConflict  = faults.ErrConflict
	ErrInvalid   = faults.ErrInvalid
	ErrLimit     = faults.ErrLimit
	ErrMuted     = faults.ErrMuted
	ErrBlocked   = faults.ErrBlocked
)

// Store supports concurrent callers. A single SQLite connection serializes
// writes and keeps in-memory databases usable without shared-cache settings.
type Store struct {
	db *sql.DB
}

// Open opens a database file and applies transactional schema migrations.
// The special path :memory: creates a private, non-durable database for tests.
// File paths are treated literally, including question marks and URI syntax.
func Open(ctx context.Context, path string) (*Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, queryError(ctx, err)
	}
	if path == "" {
		return nil, fmt.Errorf("%w: database path is empty", ErrInvalid)
	}

	var dsn string
	if path == ":memory:" {
		dsn = path
	} else {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("chatstore: database path: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
			return nil, fmt.Errorf("chatstore: database directory: %w", err)
		}

		file, err := os.OpenFile(absolute, os.O_RDWR|os.O_CREATE, 0o600)
		if err != nil {
			return nil, fmt.Errorf("chatstore: database file: %w", err)
		}
		if err := file.Close(); err != nil {
			return nil, queryError(ctx, err)
		}

		dsn = (&url.URL{Scheme: "file", Path: absolute}).String()
	}
	// Immediate transactions make membership checks and their mutations one
	// serialized operation even when another process opens the same database.
	dsn += "?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL&_synchronous=FULL&_txlock=immediate"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("chatstore: open: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db}
	if err := migrations.Apply(ctx, db); err != nil {
		db.Close()
		return nil, queryError(ctx, err)
	}

	return s, nil
}

// Close closes the store after outstanding database operations complete.
func (s *Store) Close() error { return s.db.Close() }

func username(value string) (string, error) {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value || !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("%w: username", ErrInvalid)
	}

	return strings.ToLower(value), nil
}

func groupID(value string) error {
	if value == "" || len(value) > 256 || strings.TrimSpace(value) != value || !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%w: group ID", ErrInvalid)
	}

	return nil
}

func boundedLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 1000 {
		return 1000
	}

	return limit
}
