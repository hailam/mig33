package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"testing"
)

func TestGeneratedArtifactsAreCurrent(t *testing.T) {
	if err := run("../..", true); err != nil {
		t.Fatal(err)
	}
}

func TestEvidenceChangeRequiresReview(t *testing.T) {
	root := t.TempDir()
	c := &contract{
		Profile: "j2me-4.62", Version: "0.1.0",
		OriginalJARSHA256: "dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c",
		Evidence:          []evidence{{Path: "evidence.json", SHA256: "old-hash"}},
	}
	if err := os.WriteFile(filepath.Join(root, "evidence.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateContract(root, c); err == nil {
		t.Fatal("changed evidence must require a new source review")
	}
}

func TestProjectionTagRange(t *testing.T) {
	pattern := regexp.MustCompile(tagPattern)
	for tag := -32768; tag <= 32767; tag++ {
		if !pattern.MatchString(strconv.Itoa(tag)) {
			t.Fatalf("valid signed-short tag rejected: %d", tag)
		}
	}
	for _, invalid := range []string{"32768", "-32769", "99999", "-99999", "tag", "01", "-0", "+1", "1.0", ""} {
		if pattern.MatchString(invalid) {
			t.Errorf("invalid tag accepted: %q", invalid)
		}
	}
}

func TestTransportDirectionsAndChannelExclusivity(t *testing.T) {
	c, err := loadContract("../..")
	if err != nil {
		t.Fatal(err)
	}
	doc := asyncAPI(c, nil)
	channels := doc["channels"].(object)
	for _, pair := range [][2]string{{"tcpRequests", "httpRequests"}, {"tcpResponses", "httpResponses"}} {
		left := channels[pair[0]].(object)["messages"].(object)
		right := channels[pair[1]].(object)["messages"].(object)
		if len(left) != len(right) {
			t.Fatalf("%s/%s have different packet sets", pair[0], pair[1])
		}
		seen := map[int]bool{}
		for id := range left {
			if right[id] == nil {
				t.Errorf("%s missing %s", pair[1], id)
			}
			index := slices.IndexFunc(c.Messages, func(m message) bool { return m.ID == id })
			m := c.Messages[index]
			if seen[m.Opcode] {
				t.Errorf("overlapping opcode %d in channel %s", m.Opcode, pair[0])
			}
			seen[m.Opcode] = true
			if m.Direction == "local" || slices.Contains(m.Delivery, "http-resource") {
				t.Errorf("nongateway record %s is in channel %s", id, pair[0])
			}
		}
	}
	for _, value := range doc["operations"].(object) {
		op := value.(object)
		channel := op["channel"].(object)["$ref"].(string)
		want := "receive"
		if channel == "#/channels/tcpResponses" || channel == "#/channels/httpResponses" {
			want = "send"
		}
		if op["action"] != want {
			t.Errorf("%s action = %s, want %s from server perspective", channel, op["action"], want)
		}
	}
}

func TestGenerationIsDeterministic(t *testing.T) {
	c, err := loadContract("../..")
	if err != nil {
		t.Fatal(err)
	}
	first, err := generate("../..", c)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generate("../..", c)
	if err != nil {
		t.Fatal(err)
	}
	for path, expected := range first {
		if !bytes.Equal(expected, second[path]) {
			t.Errorf("nondeterministic output: %s", path)
		}
	}
}

func TestOriginalFixturesRetainRawPayloads(t *testing.T) {
	examples, err := fixtureExamples("../..")
	if err != nil {
		t.Fatal(err)
	}
	checks := map[int]map[string]string{
		101: {"1": "00000140"},
		602: {"1": "6f6c642d617661746172"},
		419: {"1": "0000011f71fb04cb"},
		407: {"30": "616c696365406578616d706c652e7465737400626f62", "31": "0204"},
	}
	for opcode, want := range checks {
		payload := examples[opcode][0]["payload"].(object)
		fields := payload["fields"].(object)
		for tag, expected := range want {
			if fields[tag] != expected {
				t.Errorf("opcode%d tag%s = %s, want %s", opcode, tag, fields[tag], expected)
			}
		}
	}
	// These fixtures are deliberately sparse; their examples must not invent
	// fields required by a prose interpretation or a UI rollback branch.
	avatar := examples[602][0]["payload"].(object)["fields"].(object)
	if len(avatar) != 1 {
		t.Fatal("duplicate avatar tag must retain only the final payload")
	}
}

func TestStaleOutputFailsWithoutWriting(t *testing.T) {
	// Check mode must detect missing generated outputs even before Git tracks
	// them; relying on git diff would silently miss this case.
	c, err := loadContract("../..")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, path := range append([]string{sourcePath, "api/openapi.yaml"}, evidencePaths(c)...) {
		data, err := os.ReadFile(filepath.Join("../..", path))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := run(root, true); err == nil {
		t.Fatal("missing output accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "api/asyncapi.yaml")); !os.IsNotExist(err) {
		t.Fatal("check mode wrote an output")
	}
}

func evidencePaths(c *contract) []string {
	result := []string{}
	for _, e := range c.Evidence {
		result = append(result, e.Path)
	}
	return result
}

func TestEvidencePointerMustResolve(t *testing.T) {
	data, err := json.Marshal(object{"operations": []object{{"opcode": 2}}})
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string][]byte{"audit.json": data}
	for _, pointer := range []string{"/operations/1", "/operations/0/missing", "/operations/-1"} {
		if err := checkEvidence(sources, []evidence{{Path: "audit.json", Pointer: pointer}}); err == nil {
			t.Errorf("unresolved evidence pointer %s accepted", pointer)
		}
	}
}
