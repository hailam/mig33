package migrations_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	catalog "github.com/hailam/mig33/db/migrations"
	"github.com/hailam/mig33/internal/adapters/sqlite/migrations"
)

func TestCanceledMigrationDoesNotCreateSchema(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := migrations.Apply(ctx, db); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled migration: %v", err)
	}
	var tables, version int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM sqlite_schema").Scan(&tables); err != nil || tables != 0 {
		t.Fatalf("canceled migration changed schema: tables=%d error=%v", tables, err)
	}
	if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&version); err != nil || version != 0 {
		t.Fatalf("canceled migration changed version: version=%d error=%v", version, err)
	}
}

func TestUnsupportedVersionDoesNotTouchDatabase(t *testing.T) {
	for _, version := range []int{-1, catalog.LatestVersion + 1} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			db, err := sql.Open("sqlite3", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if _, err := db.Exec(fmt.Sprintf("CREATE TABLE marker(value TEXT); INSERT INTO marker VALUES('preserved'); PRAGMA user_version=%d;", version)); err != nil {
				t.Fatal(err)
			}
			if err := migrations.Apply(t.Context(), db); err == nil || !strings.Contains(err.Error(), "unsupported schema version") {
				t.Fatalf("unsupported version: %v", err)
			}
			var actual, tables int
			if err := db.QueryRow("PRAGMA user_version").Scan(&actual); err != nil || actual != version {
				t.Fatalf("version changed: %d, %v", actual, err)
			}
			if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil || tables != 1 {
				t.Fatalf("schema changed: %d tables, %v", tables, err)
			}
			var value string
			if err := db.QueryRow("SELECT value FROM marker").Scan(&value); err != nil || value != "preserved" {
				t.Fatalf("existing data changed: %q, %v", value, err)
			}
		})
	}
}
