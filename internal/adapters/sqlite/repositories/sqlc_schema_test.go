package repositories

import (
	"database/sql"
	"os"
	"reflect"
	"testing"
)

// sqlc compiles against a schema snapshot; verify it against real migrations
// so a column/index/constraint change cannot silently leave codegen behind.
func TestSQLCSchemaMatchesMigrations(t *testing.T) {
	store, err := Open(t.Context(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ddl, err := os.ReadFile("../../../../db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	snapshot.SetMaxOpenConns(1)
	if _, err := snapshot.Exec(string(ddl)); err != nil {
		t.Fatal(err)
	}
	definitions := func(db *sql.DB) map[string]string {
		t.Helper()
		rows, err := db.Query("SELECT name,sql FROM sqlite_schema WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY name")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		result := map[string]string{}
		for rows.Next() {
			var name, ddl string
			if err := rows.Scan(&name, &ddl); err != nil {
				t.Fatal(err)
			}
			result[name] = ddl
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return result
	}
	actual, generated := definitions(store.db), definitions(snapshot)
	if !reflect.DeepEqual(actual, generated) {
		for name, want := range actual {
			if generated[name] != want {
				t.Errorf("sqlc schema differs from migrated database for %s; regenerate db/schema.sql", name)
			}
		}
		for name := range generated {
			if _, ok := actual[name]; !ok {
				t.Errorf("sqlc schema contains obsolete object %s", name)
			}
		}
	}
}
