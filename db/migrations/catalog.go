// Package migrations embeds the ordered schema upgrades and their support SQL.
// Application binaries never depend on migration files in the working directory.
package migrations

import (
	"embed"
	"fmt"
	"strings"
)

const LatestVersion = 18

//go:embed *.sql support/*.sql
var files embed.FS

// SQL returns the original SQL for one numbered migration.
func SQL(version int) (string, error) {
	if version < 1 || version > LatestVersion {
		return "", fmt.Errorf("migration version %d is outside 1..%d", version, LatestVersion)
	}
	data, err := files.ReadFile(fmt.Sprintf("%03d.sql", version))
	return string(data), err
}

// Support returns a named migration metadata or V2 repair statement. Names are
// package-owned basenames without their .sql extension, never query text.
func Support(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, "/\\.") {
		return "", fmt.Errorf("invalid migration support name %q", name)
	}
	data, err := files.ReadFile("support/" + name + ".sql")
	return string(data), err
}
