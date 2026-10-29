package legacyweb

import (
	"net/url"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/commerce"
)

func TestNativeStoreNoteFormPreservesVerbatimTail(t *testing.T) {
	target, _ := url.Parse("http://127.0.0.1:8080/sites/index.php?c=store&a=home&v=midlet&op=purchase")
	form := url.Values{"csrf": {"session"}, "item": {"big-hug"}, "recipient": {"bob"}, "binding": {"binding"}, "nonce": {"request"}}
	const note = "<b>Hello & plus + equals = 50% ✓</b>"
	parsed, err := parseCommunityCommerceForm(target, form.Encode()+"&note="+strings.ReplaceAll(note, " ", "%20"))
	if err != nil || parsed.Get("note") != note {
		t.Fatal("native note serialization", parsed, err)
	}
}

func TestCommunityCommerceNativeFormTailAndRouteValidation(t *testing.T) {
	u, _ := url.Parse("http://127.0.0.1:8080/sites/index.php?c=group&a=view_forum&v=midlet&op=post")
	f, err := parseCommunityCommerceForm(u, "csrf=abc&nonce=def&parent=0&body=Hello%20A&B=1+50%✓")
	if err != nil || f.Get("body") != "Hello A&B=1+50%✓" || len(f) != 4 {
		t.Fatal(f, err)
	}
	u.RawQuery += "&c=account"
	if communityCommerceRoute(u) {
		t.Fatal("ambiguous selector accepted")
	}
	for _, raw := range []string{"", "1.001", "-1", "1e2", "1+2", ".1", "1.", "NaN", "1000000.01"} {
		if _, err := commerce.ParseAmount(raw); err == nil {
			t.Fatal("invalid credit amount", raw)
		}
	}
	for raw, want := range map[string]int64{"1": 100, "1.2": 120, "0.01": 1, "1000000.00": 100000000} {
		got, err := commerce.ParseAmount(raw)
		if err != nil || got != want {
			t.Fatal(raw, got, err)
		}
	}
	for _, query := range []string{"cid=1", "group_id=1", "id=1"} {
		got, err := ccID(&url.URL{RawQuery: query})
		if err != nil || got != 1 {
			t.Fatal(query, got, err)
		}
	}
	for _, query := range []string{"cid=1&cid=1", "cid=1&group_id=1", "cid=1&id=2", "cid=", "cid=-1"} {
		if _, err := ccID(&url.URL{RawQuery: query}); err == nil {
			t.Fatal("ambiguous Group selector", query)
		}
	}
}

func TestProductSelectionPageRangePreserved(t *testing.T) {
	for page := 0; page < 10; page++ {
		q := url.Values{"page": {string(rune('0' + page))}}
		selection, items, err := selectProducts(q)
		if err != nil || len(items) != 78 || selection.Page != page {
			t.Fatal(selection, len(items), err)
		}
	}
}
