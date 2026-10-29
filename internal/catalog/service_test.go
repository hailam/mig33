package catalog

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type ownership struct {
	owned bool
	err   error
}

func (o ownership) HasStoreItem(context.Context, string, string) (bool, error) { return o.owned, o.err }

func TestSendingRequiresPackButRenderingDoesNot(t *testing.T) {
	service := New(ownership{})
	if err := service.ValidateEmoticons(context.Background(), "alice", "Hello [smile]"); err != nil {
		t.Fatal(err)
	}
	if err := service.ValidateEmoticons(context.Background(), "alice", "[buzz]"); err == nil {
		t.Fatal("unowned paid effect accepted")
	}
	asset, ok := LookupEmoticon("[buzz]")
	if !ok || len(asset.Cells) == 0 || asset.Type != 2 {
		t.Fatalf("recipient definition: %+v %v", asset, ok)
	}
	tokens, err := service.OwnedEmoticons(context.Background(), "alice")
	if err != nil || slices.Contains(tokens, "[buzz]") || !slices.Contains(tokens, "[smile]") {
		t.Fatalf("free catalog: %v %v", tokens, err)
	}
	service = New(ownership{owned: true})
	if err := service.ValidateEmoticons(context.Background(), "alice", "[buzz]"); err != nil {
		t.Fatal(err)
	}
	if err := service.ValidateEmoticons(context.Background(), "alice", "[gift:rose]"); err == nil {
		t.Fatal("paid ownership allowed forged purchase animation")
	}
}

func TestCatalogOwnershipFailureAndCancellationFailClosed(t *testing.T) {
	failed := errors.New("inventory unavailable")
	if err := New(ownership{err: failed}).ValidateEmoticons(context.Background(), "alice", "[buzz]"); !errors.Is(err, failed) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Snapshot(ctx, "https://example.test"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestSnapshotPreservesHistoricalLanguageURL(t *testing.T) {
	result, err := Snapshot(context.Background(), "https://example.test")
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Languages[0]["url"]; got != "https://example.test/sites/resources/language/v4.6/en-US" {
		t.Fatal(got)
	}
	result.Products[0].SourceURLs = append(result.Products[0].SourceURLs, "changed")
	fresh, _ := Snapshot(context.Background(), "https://example.test")
	if slices.Contains(fresh.Products[0].SourceURLs, "changed") {
		t.Fatal("catalog exposes mutable source storage")
	}
}
