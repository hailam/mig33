package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCLIAndFlagOverrides(t *testing.T) {
	t.Setenv("MIG33_ENV", "missing")
	t.Setenv("MIG33_PUBLIC_URL", "invalid")
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "app.yaml"), []byte("operators: [missing_account]\ndatabase:\n  path: /this/path/must/not/be/opened.sqlite\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(context.Background(), []string{
		"-config-dir",
		directory,
		"-env=",
		"-public-url",
		"http://127.0.0.1:8080",
		"-db",
		":memory:",
		"-operators=",
		"-reports",
	}, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	if output.String() != "[]\n" {
		t.Fatalf("reports: %q", output.String())
	}
	for _, args := range [][]string{nil, {"-reports", "-report-id", "1"}, {"-revoke"}, {"-report-id", "-1"}} {
		if err := run(context.Background(), args, io.Discard, io.Discard); err == nil {
			t.Fatalf("invalid CLI args accepted: %v", args)
		}
	}
}
