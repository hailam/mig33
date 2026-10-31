package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvalidTLSIdentityFailsBeforeDatabaseOrListeners(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "app.yaml"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(directory, "must-not-exist.sqlite")
	err := run(t.Context(), []string{
		"-config-dir", directory, "-env=", "-db", database,
		"-http=127.0.0.1:0", "-tcp=127.0.0.1:0", "-https=127.0.0.1:0", "-tls-tcp=127.0.0.1:0",
		"-tls-cert", filepath.Join(directory, "missing.pem"), "-tls-key", filepath.Join(directory, "missing-key.pem"),
	}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "load TLS certificate and key") {
		t.Fatalf("wanted TLS load failure, got %v", err)
	}
	if _, err := os.Stat(database); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid TLS identity opened application database: %v", err)
	}
}
