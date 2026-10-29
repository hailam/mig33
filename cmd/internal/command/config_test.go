package command

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func clearConfigEnvironment(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "MIG33_") {
			// Setenv records the previous value for cleanup; remove it while the
			// test runs because an explicitly empty environment value is significant.
			t.Setenv(name, "")
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func assertDatabaseAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("configuration parsing created database %s: %v", path, err)
	}
}

func TestExplicitEmptyFlagsOverrideEnvironment(t *testing.T) {
	clearConfigEnvironment(t)
	directory := t.TempDir()
	t.Chdir(directory)
	database := filepath.Join(directory, "must-not-be-created.sqlite")
	t.Setenv("MIG33_CONFIG_DIR", filepath.Join(directory, "missing-configs"))
	t.Setenv("MIG33_ENV", "missing")
	t.Setenv("MIG33_PUBLIC_URL", "invalid")
	t.Setenv("MIG33_DATABASE_PATH", "")
	t.Setenv("MIG33_OPERATORS", "alice")
	flags := NewFlags("test", io.Discard)
	if err := flags.Parse([]string{"-config-dir=", "-env=", "-operators=", "-public-url=http://127.0.0.1:8080", "-db", database}); err != nil {
		t.Fatal(err)
	}
	configuration, err := flags.Load()
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Environment != "" || len(configuration.Operators) != 0 || configuration.PublicURL != "http://127.0.0.1:8080" || configuration.Database.Path != database {
		t.Fatalf("explicit overrides lost: %+v", configuration)
	}
	assertDatabaseAbsent(t, database)
}

func TestOnlySelectedListenersAreValidated(t *testing.T) {
	clearConfigEnvironment(t)
	for _, tc := range []struct {
		name, httpAddress, socketAddress, failure string
		httpEnabled, socketEnabled                bool
	}{
		{"no servers", "invalid", "invalid", "", false, false},
		{"http ignores unused socket", "127.0.0.1:0", "invalid", "", true, false},
		{"socket ignores unused http", "invalid", "127.0.0.1:0", "", false, true},
		{"http rejects selected address", "invalid", "127.0.0.1:0", "http.listen", true, false},
		{"socket rejects selected address", "127.0.0.1:0", "invalid", "socket.listen", false, true},
		{"both selected", "127.0.0.1:0", "127.0.0.1:0", "", true, true},
		{"second selected address invalid", "127.0.0.1:0", "invalid", "socket.listen", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			database := filepath.Join(directory, "unopened.sqlite")
			data := fmt.Sprintf("database:\n  path: %q\nhttp:\n  listen: %q\nsocket:\n  listen: %q\n", database, tc.httpAddress, tc.socketAddress)
			if err := os.WriteFile(filepath.Join(directory, "app.yaml"), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			flags := NewFlags("test", io.Discard)
			if tc.httpEnabled {
				flags.HTTP()
			}
			if tc.socketEnabled {
				flags.Socket()
			}
			if err := flags.Parse([]string{"-config-dir", directory}); err != nil {
				t.Fatal(err)
			}
			_, err := flags.Load()
			if tc.failure == "" && err != nil {
				t.Fatal(err)
			}
			if tc.failure != "" && (err == nil || !strings.Contains(err.Error(), tc.failure)) {
				t.Fatalf("wanted %s validation failure, got %v", tc.failure, err)
			}
			assertDatabaseAbsent(t, database)
		})
	}
}

func TestParseAndHelpDoNotLoadConfiguration(t *testing.T) {
	clearConfigEnvironment(t)
	directory := t.TempDir()
	database := filepath.Join(directory, "unopened.sqlite")
	missing := filepath.Join(directory, "missing")
	t.Setenv("MIG33_ENV", "missing")
	t.Setenv("MIG33_PUBLIC_URL", "invalid")
	for _, tc := range []struct {
		name string
		help bool
	}{{"parse only", false}, {"help", true}} {
		t.Run(tc.name, func(t *testing.T) {
			flags := NewFlags("test", io.Discard)
			arguments := []string{"-config-dir", missing, "-db", database}
			if tc.help {
				arguments = append(arguments, "-help")
			}
			err := flags.Parse(arguments)
			if tc.help {
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("help error = %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("Parse loaded configuration before entrypoint validation: %v", err)
				}
				if _, err := flags.Load(); err == nil || !strings.Contains(err.Error(), "app.yaml") {
					t.Fatalf("Load must report missing config: %v", err)
				}
			}
			assertDatabaseAbsent(t, database)
		})
	}
}

func TestUnselectedCapabilityFlagsAreRejected(t *testing.T) {
	clearConfigEnvironment(t)
	for _, tc := range []struct {
		name      string
		configure func(*Flags)
		arguments []string
	}{
		{"http rejects socket", (*Flags).HTTP, []string{"-tcp", "127.0.0.1:0"}},
		{"socket rejects http", (*Flags).Socket, []string{"-http", "127.0.0.1:0"}},
		{"common helper has no operator actions", func(*Flags) {}, []string{"-reports"}},
		{"positional arguments", func(*Flags) {}, []string{"unexpected"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags := NewFlags("test", io.Discard)
			tc.configure(flags)
			if err := flags.Parse(tc.arguments); err == nil {
				t.Fatalf("accepted unsupported arguments: %v", tc.arguments)
			}
		})
	}
}
