package pages

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func TestBlockListFormsRetainNativePOSTContract(t *testing.T) {
	const token = "0123456789abcdef0123456789abcdef0123456789abcdef"
	result := BlockList([]string{"bob", "carol"}, token, "", "http://127.0.0.1:8080")
	if result.Status != http.StatusOK || result.ContentType != "text/html; charset=utf-8" {
		t.Fatalf("block-list response: %+v", result)
	}
	body := string(result.Body)
	forms := regexp.MustCompile(`<form\s+method="([^"]+)"\s+action="([^"]+)">(.*?)</form>`).FindAllStringSubmatch(body, -1)
	if len(forms) != 2 {
		t.Fatalf("each blocked user needs an independent form: %s", body)
	}
	for i, form := range forms {
		if form[1] != "post" {
			t.Fatal("unblocking must not be a GET link")
		}
		// The native parser passes attributes through without entity decoding.
		// Parsing the raw action catches an incompatible &amp; substitution.
		action, err := url.Parse(form[2])
		if err != nil || action.Scheme != "http" || action.Host != "127.0.0.1:8080" || action.Path != "/sites/index.php" || action.Query().Get("c") != "settings" || action.Query().Get("a") != "block_list" || action.Query().Get("v") != "midlet" || len(action.Query()) != 3 {
			t.Fatalf("native form action changed: %q (%v)", form[2], err)
		}
		base, _ := url.Parse("http://127.0.0.1:8080/sites/index.php?c=settings&a=block_list&v=midlet")
		if resolved := base.ResolveReference(action); resolved.Path != "/sites/index.php" {
			t.Fatalf("browser form action resolves to the wrong path: %s", resolved)
		}
		inputs := regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`).FindAllStringSubmatch(form[3], -1)
		values := make(url.Values)
		for _, input := range inputs {
			values.Add(input[1], html.UnescapeString(input[2]))
		}
		if len(inputs) != 3 || values.Get("action") != "unblock" || values.Get("username") != []string{"bob", "carol"}[i] || values.Get("csrf") != token {
			t.Fatalf("incorrect unblock form payload: %v", values)
		}
		if !strings.Contains(form[3], `<input type="submit" value="Unblock">`) {
			t.Fatal("native parser needs a submit input, not a button element or script")
		}
	}
	if strings.Contains(body, "<script") || strings.Contains(body, "<button") || strings.Contains(body, "onclick") {
		t.Fatal("block-list form depends on unsupported browser features")
	}
}

func TestBlockListEscapesUntrustedValues(t *testing.T) {
	username := `bob"><input name="injected" value="yes"> & 'friend'`
	token := `token"&<test>`
	notice := `<script>alert("bad")</script> & done`
	origin := `http://example.test/" onsubmit="injected`
	body := string(BlockList([]string{username}, token, notice, origin).Body)
	for _, expected := range []string{
		"<b>" + html.EscapeString(username) + "</b>",
		`name="username" value="` + html.EscapeString(username) + `"`,
		`name="csrf" value="` + html.EscapeString(token) + `"`,
		"<p>" + html.EscapeString(notice) + "</p>",
		// The shared URL boundary rejects malformed command/URL attributes;
		// ordinary text and fixed form values still use template escaping.
		`action="about:invalid#invalid-native-url"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("unescaped or missing value: %q in %s", expected, body)
		}
	}
	if strings.Contains(body, `<input name="injected"`) || strings.Contains(body, "<script>") || strings.Contains(body, ` onsubmit="injected`) {
		t.Fatal("untrusted values injected markup")
	}
}

func TestBlockListEmptyStateAndNotice(t *testing.T) {
	body := string(BlockList(nil, "token", "bob was unblocked.", "http://127.0.0.1:8080").Body)
	if !strings.Contains(body, "You have no blocked users.") || !strings.Contains(body, "bob was unblocked.") {
		t.Fatalf("empty state or action result missing: %s", body)
	}
	if strings.Contains(body, "<form") || strings.Contains(body, "<input") {
		t.Fatal("empty block list must not offer an action without a target")
	}
}

func TestBlockListDoesNotFallBackToRelativeAction(t *testing.T) {
	result := BlockList([]string{"bob"}, "token", "", "")
	if result.Status != http.StatusServiceUnavailable || strings.Contains(string(result.Body), "<form") {
		t.Fatal("missing origin must not generate a relative form action")
	}
}
