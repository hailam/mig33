package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func generatedHTTPMetadata(t *testing.T, root string) nativeHTTPMetadata {
	t.Helper()
	generated, err := generateHTTP(root)
	if err != nil {
		t.Fatal(err)
	}
	_, payload, ok := strings.Cut(string(generated), "export const nativeHTTP = ")
	if !ok {
		t.Fatal("missing generated nativeHTTP declaration")
	}
	payload, ok = strings.CutSuffix(payload, " as const;\n")
	if !ok {
		t.Fatal("missing generated const assertion")
	}
	var result nativeHTTPMetadata
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func httpContractFixture(t *testing.T, change func(*httpSource)) string {
	t.Helper()
	raw, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var source httpSource
	if err := yaml.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	change(&source)
	raw, err = yaml.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "api/openapi.yaml"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestNativeHTTPMetadataPreservesOriginalRoutesAndBinaryMIME(t *testing.T) {
	metadata := generatedHTTPMetadata(t, "../..")
	if metadata.RootPath != "/" || metadata.SessionPath != "/{sid}" {
		t.Fatal("original HTTP paths changed")
	}
	if metadata.RequestContentType != "image/gif" || metadata.ResponseContentType != "image/gif" {
		t.Fatal("historical binary tunnel MIME changed")
	}
	if !slices.Equal(metadata.BasePathOpcodes, []int{200, 100, 2, 904}) {
		t.Fatalf("base-path exceptions = %v", metadata.BasePathOpcodes)
	}
	if metadata.Operations.RootGet != "PollNativeRoot" || metadata.Operations.RootPost != "PostNativeRoot" ||
		metadata.Operations.SessionGet != "PollNativeSession" || metadata.Operations.SessionPost != "PostNativeSession" {
		t.Fatalf("unexpected native operation IDs: %+v", metadata.Operations)
	}
	if !strings.Contains(metadata.MessageBoundary, "zero or more concatenated packets") ||
		!strings.Contains(metadata.SessionRouting, "Polls also use the session URL") {
		t.Fatal("source batching/session observations were lost")
	}
}

func TestNativeHTTPMetadataTracksSourceChanges(t *testing.T) {
	root := httpContractFixture(t, func(source *httpSource) {
		source.OriginalClient.Tunnel.BaseURLOpcodes = []int{200, 2}
		source.OriginalClient.Tunnel.SessionRouting = "Changed reviewed routing observation"
		source.OriginalClient.Tunnel.MessageBoundary = "Changed reviewed batch observation"
		operation := source.Paths["/"]["get"]
		operation.ID = "NewReviewedRootPollName"
		source.Paths["/"]["get"] = operation
	})
	metadata := generatedHTTPMetadata(t, root)
	if !slices.Equal(metadata.BasePathOpcodes, []int{200, 2}) ||
		metadata.Operations.RootGet != "NewReviewedRootPollName" ||
		metadata.SessionRouting != "Changed reviewed routing observation" ||
		metadata.MessageBoundary != "Changed reviewed batch observation" {
		t.Fatalf("HTTP projection retained duplicated constants instead of source data: %+v", metadata)
	}
}

func TestNativeHTTPMetadataRejectsIncompleteOrNonbinaryContracts(t *testing.T) {
	for name, change := range map[string]func(*httpSource){
		"missing session route": func(source *httpSource) { delete(source.Paths, "/{sid}") },
		"missing post":          func(source *httpSource) { delete(source.Paths["/"], "post") },
		"missing historical request MIME": func(source *httpSource) {
			operation := source.Paths["/"]["post"]
			delete(operation.RequestBody.Content, "image/gif")
			source.Paths["/"]["post"] = operation
		},
		"JSON response": func(source *httpSource) {
			operation := source.Paths["/{sid}"]["get"]
			media := operation.Responses["200"].Content["image/gif"]
			media.Schema.Type = "object"
			operation.Responses["200"].Content["image/gif"] = media
			source.Paths["/{sid}"]["get"] = operation
		},
		"missing response": func(source *httpSource) {
			operation := source.Paths["/"]["get"]
			delete(operation.Responses, "200")
			source.Paths["/"]["get"] = operation
		},
		"missing source routing":   func(source *httpSource) { source.OriginalClient.Tunnel.SessionRouting = "" },
		"duplicate base opcode":    func(source *httpSource) { source.OriginalClient.Tunnel.BaseURLOpcodes = []int{200, 200} },
		"out of range base opcode": func(source *httpSource) { source.OriginalClient.Tunnel.BaseURLOpcodes = []int{32768} },
	} {
		t.Run(name, func(t *testing.T) {
			root := httpContractFixture(t, change)
			if _, err := generateHTTP(root); err == nil {
				t.Fatal("incomplete or incompatible HTTP contract was accepted")
			}
		})
	}
}
