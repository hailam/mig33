package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, directory, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name+".yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultsAndOverlayPrecedence(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "app", "database:\n  path: base.sqlite\nhttp:\n  listen: 127.0.0.1:8000\noperators: [alice]\n")
	writeConfig(t, dir, "dev", "database:\n  path: dev.sqlite\nhttp:\n  read_timeout: 25s\noperators: [bob]\n")
	writeConfig(t, dir, "custom", "http:\n  listen: 127.0.0.1:8002\noperators: [carol]\n")
	configuration, err := Load(LoadOptions{Directory: dir, Overlays: []string{"custom"}, Environ: []string{
		"MIG33_ENV=dev", "MIG33_HTTP_LISTEN=127.0.0.1:8003", "MIG33_OPERATORS=", "MIG33_HTTP_WRITE_TIMEOUT=30s",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Environment != "dev" || configuration.Database.Path != "dev.sqlite" || configuration.HTTP.Listen != "127.0.0.1:8003" {
		t.Fatalf("overlay precedence: %+v", configuration)
	}
	if configuration.HTTP.ReadTimeout != 25*time.Second || configuration.HTTP.WriteTimeout != 30*time.Second || configuration.HTTP.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("nested values were not merged: %+v", configuration.HTTP)
	}
	if len(configuration.Operators) != 0 {
		t.Fatal("empty environment override must clear operator list")
	}
	if configuration.Socket.Listen != "127.0.0.1:9119" {
		t.Fatal("embedded defaults missing")
	}
}

func TestExplicitSelectionWinsOverEnvironment(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "app", "{}")
	writeConfig(t, dir, "local", "database:\n  path: local.sqlite\n")
	configuration, err := Load(LoadOptions{Directory: dir, Environment: "local", Environ: []string{"MIG33_CONFIG_DIR=/missing", "MIG33_ENV=missing"}})
	if err != nil || configuration.Database.Path != "local.sqlite" {
		t.Fatalf("selection: %+v %v", configuration, err)
	}
}

func TestResourceDirectoryCanBeOverriddenOrDisabled(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "app", "resources:\n  directory: local-assets\n")
	writeConfig(t, dir, "dev", "resources:\n  directory: dev-assets\n")
	for _, test := range []struct {
		name    string
		environ []string
		want    string
	}{
		{"overlay", []string{}, "dev-assets"},
		{"environment", []string{"MIG33_RESOURCES_DIRECTORY=/missing/optional-assets"}, "/missing/optional-assets"},
		{"disabled", []string{"MIG33_RESOURCES_DIRECTORY="}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := Load(LoadOptions{Directory: dir, Environment: "dev", Environ: test.environ})
			if err != nil || cfg.Resources.Directory != test.want {
				t.Fatalf("resources: %q, %v", cfg.Resources.Directory, err)
			}
		})
	}
}

func TestExplicitOverridesPrecedeValidation(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "app", "{}")
	publicURL, database := "http://127.0.0.1:8080", ":memory:"
	configuration, err := Load(LoadOptions{
		Directory:      dir,
		EnvironmentSet: true,
		Environ:        []string{"MIG33_ENV=missing", "MIG33_PUBLIC_URL=invalid", "MIG33_DATABASE_PATH="},
		Overrides:      Overrides{PublicURL: &publicURL, DatabasePath: &database},
	})
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Environment != "" || configuration.PublicURL != publicURL || configuration.Database.Path != database {
		t.Fatalf("explicit values lost: %+v", configuration)
	}
}

func TestInvalidConfigurationIsNotSilentlyIgnored(t *testing.T) {
	for _, test := range []struct{ name, yaml string }{
		{"unknown", "http:\n  lissten: 127.0.0.1:8000\n"},
		{"duplicate", "public_url: http://localhost\npublic_url: http://example.test\n"},
		{"null", "database:\n  path: null\n"},
		{"multiple documents", "{}\n---\n{}\n"},
		{"list", "- invalid\n"},
		{"empty", ""},
		{"bad duration", "http:\n  read_timeout: later\n"},
		{"zero duration", "shutdown_timeout: 0s\n"},
		{"empty database", "database:\n  path: ''\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, "app", test.yaml)
			if _, err := Load(LoadOptions{Directory: dir, Environ: []string{}}); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	for _, entry := range []string{
		"MIG33_HTTP_LISTENN=bad",
		"MIG33_HTTP_READ_TIMEOUT=later",
		"MIG33_HTTP_MAX_HEADER_BYTES=many",
		"MIG33_PUBLIC_URL=",
		"MIG33_OPERATORS=alice,,bob",
	} {
		t.Run(entry, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, "app", "{}")
			if _, err := Load(LoadOptions{Directory: dir, Environ: []string{entry}}); err == nil {
				t.Fatal("invalid environment accepted")
			}
		})
	}
}

func TestMissingAndUnsafeOverlayFails(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "app", "{}")
	for _, name := range []string{"missing", "../elsewhere", "/tmp/local", "app", "dev.yaml"} {
		if _, err := Load(LoadOptions{Directory: dir, Environment: name, Environ: []string{}}); err == nil {
			t.Fatalf("overlay %q accepted", name)
		}
	}
	if _, err := Load(LoadOptions{Directory: t.TempDir(), Environ: []string{}}); err == nil || !strings.Contains(err.Error(), "app.yaml") {
		t.Fatalf("missing explicit app.yaml: %v", err)
	}
}

func TestListenerValidation(t *testing.T) {
	for _, value := range []string{"127.0.0.1:8080", "[::1]:8080", ":0"} {
		if err := ValidateListener("http.listen", value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"", "localhost", "localhost:http", ":65536", "::-1"} {
		if err := ValidateListener("http.listen", value); err == nil {
			t.Fatalf("invalid address %q accepted", value)
		}
	}
}
