package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/slashcommands"
)

func TestSlashCommandsProjectGoMetadata(t *testing.T) {
	output, err := generateSlashCommands()
	if err != nil {
		t.Fatal(err)
	}
	_, payload, ok := strings.Cut(string(output), "export const slashCommands = ")
	if !ok {
		t.Fatal("missing TypeScript catalog")
	}
	payload, ok = strings.CutSuffix(payload, " as const satisfies readonly SlashCommandDefinition[];\n")
	if !ok {
		t.Fatal("missing readonly literal schema check")
	}
	var got []slashcommands.Command
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, slashcommands.Catalog()) {
		t.Fatal("generated metadata differs from the Go catalog")
	}
}

func TestCheckDetectsStaleSlashCatalogWithoutWriting(t *testing.T) {
	c, err := loadContract("../..")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	write := func(path string, data []byte) {
		t.Helper()
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range append([]string{sourcePath, "api/openapi.yaml"}, evidencePaths(c)...) {
		data, err := os.ReadFile(filepath.Join("../..", path))
		if err != nil {
			t.Fatal(err)
		}
		write(path, data)
	}
	outputs, err := generate(root, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 5 || outputs[slashCommandsPath] == nil {
		t.Fatal("generation must include the separate fifth slash-command artifact")
	}
	for path, data := range outputs {
		write(path, data)
	}
	if err := run(root, true); err != nil {
		t.Fatalf("fresh output failed check: %v", err)
	}
	stale := []byte("// old catalog\n")
	write(slashCommandsPath, stale)
	if err := run(root, true); err == nil || !strings.Contains(err.Error(), slashCommandsPath) {
		t.Fatalf("stale suggestion catalog was not detected: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, slashCommandsPath))
	if err != nil || !bytes.Equal(got, stale) {
		t.Fatal("check mode changed the stale artifact")
	}
}
