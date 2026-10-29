package repositories

import (
	"context"
	"database/sql"

	catalog "github.com/hailam/mig33/db/migrations"
	"github.com/hailam/mig33/internal/adapters/sqlite/migrations"
)

// Existing upgrade fixtures use the same embedded migrations as startup. Keep
// these aliases test-only so feature repository code contains no schema SQL.
var (
	schemaV1 = migrationFixture(1)
	schemaV3 = migrationFixture(3)
	schemaV4 = migrationFixture(4)
	schemaV5 = migrationFixture(5)
	schemaV6 = migrationFixture(6)
	schemaV7 = migrationFixture(7)
)

func migrationFixture(version int) string {
	ddl, err := catalog.SQL(version)
	if err != nil {
		panic(err)
	}
	return ddl
}

func migrateV2(ctx context.Context, tx *sql.Tx) error { return migrations.ApplyVersion(ctx, tx, 2) }
