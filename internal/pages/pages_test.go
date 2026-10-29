package pages

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestProfileLookupAndEscaping(t *testing.T) {
	viewer := User{Username: "bob", DisplayName: "Bob", Online: true}
	users := []User{{
		Username:    "alice",
		DisplayName: `<script>alert("name")</script> & Alice`,
		Status:      `I'm <b>here</b> & "ready"`,
		Online:      true,
	}}
	result := Handle(http.MethodGet, parseURL(t, "/sites/index.php?c=profile&a=home&v=midlet&username=ALICE"), viewer, users)
	if result.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", result.Status, result.Body)
	}
	if result.ContentType != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type %q", result.ContentType)
	}
	body := string(result.Body)
	for _, want := range []string{
		"<title>User Profile</title>",
		"&lt;script&gt;alert(&#34;name&#34;)&lt;/script&gt; &amp; Alice",
		"Username: alice",
		"Presence: Online",
		"I&#39;m &lt;b&gt;here&lt;/b&gt; &amp; &#34;ready&#34;",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("profile lacks %q: %s", want, body)
		}
	}
	if strings.Contains(body, "<script>") || strings.Contains(body, "<b>here</b>") || strings.Contains(body, "Username: bob") {
		t.Fatalf("unescaped content or wrong profile: %s", body)
	}
}

func TestSelfProfileAndUnknownUser(t *testing.T) {
	viewer := User{Username: "me", Status: "Available"}
	for _, suffix := range []string{"", "&username=", "&username=ME"} {
		result := Handle(http.MethodGet, parseURL(t, "/sites/index.php?c=profile&a=home&v=midlet"+suffix), viewer, nil)
		if result.Status != http.StatusOK || !strings.Contains(string(result.Body), "<b>me</b>") || !strings.Contains(string(result.Body), "Presence: Offline") {
			t.Errorf("self profile %q: status %d, body %s", suffix, result.Status, result.Body)
		}
	}
	result := Handle(http.MethodGet, parseURL(t, "/sites/index.php?c=profile&a=home&v=midlet&username=missing"), viewer, nil)
	if result.Status != http.StatusNotFound {
		t.Errorf("missing user status = %d, want 404", result.Status)
	}
}

func TestProfileNativeActionsDoNotEmbedUnsafeCommandArguments(t *testing.T) {
	viewer := User{Username: "viewer"}
	for _, username := range []string{`bob)"><a href="mig33:auth(`, "bob smith", "bob%20smith", "bob,alice"} {
		address := "/sites/index.php?c=profile&a=home&v=midlet&username=" + url.QueryEscape(username)
		result := Handle(http.MethodGet, parseURL(t, address), viewer, []User{{Username: username}})
		if result.Status != 200 || strings.Contains(string(result.Body), `<a href=`) {
			t.Fatalf("unsafe command argument emitted: %q %s", username, result.Body)
		}
	}
	result := Handle(http.MethodGet, parseURL(t, "/sites/index.php?c=profile&a=home&v=midlet"), viewer, nil)
	if strings.Contains(string(result.Body), "mig33:") {
		t.Fatal("self profile offers self-chat/contact action")
	}
}

func TestEveryPreservedTemplateIsRecognized(t *testing.T) {
	var catalog struct {
		Routes []route `json:"routes"`
	}
	if err := json.Unmarshal(catalogJSON, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Routes) != 52 {
		t.Fatalf("catalog contains %d templates, want all 52", len(catalog.Routes))
	}
	for _, entry := range catalog.Routes {
		t.Run(entry.ID, func(t *testing.T) {
			// A trailing %s after midlet represents an entire optional query
			// suffix, while the other placeholders are parameter values.
			path := strings.ReplaceAll(entry.Template, "midlet%s", "midlet")
			path = strings.ReplaceAll(path, "%s", "alice")
			result := Handle(http.MethodGet, parseURL(t, path), User{Username: "alice"}, nil)
			want := http.StatusNotImplemented
			if (entry.Controller == "profile" && entry.Action == "home") || (entry.Controller == "help" && entry.Action == "main") {
				want = http.StatusOK
			}
			if result.Status != want {
				t.Errorf("%s: status = %d, want %d", path, result.Status, want)
			}
			if want == http.StatusNotImplemented && !strings.Contains(string(result.Body), "not implemented") {
				t.Errorf("unavailable feature should explain its status: %s", result.Body)
			}
		})
	}
}

func TestUnknownRoutesAndMethods(t *testing.T) {
	for _, path := range []string{
		"/", "/sites/other.php?c=profile&a=home&v=midlet",
		"/sites/index.php?c=made_up&a=home&v=midlet",
		"/sites/index.php?c=profile&a=made_up&v=midlet",
		"/sites/index.php?c=profile&a=home&v=wap",
		"/sites/index.php?c=profile&a=home",
		"/sites/index.php?c=profile&c=help&a=home&v=midlet",
	} {
		result := Handle(http.MethodGet, parseURL(t, path), User{}, nil)
		if result.Status != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", path, result.Status)
		}
	}
	if result := Handle(http.MethodGet, nil, User{}, nil); result.Status != http.StatusNotFound {
		t.Errorf("nil URL: status = %d, want 404", result.Status)
	}
	for _, path := range []string{
		"/sites/index.php?c=profile&a=home&v=midlet",
		"/sites/index.php?c=account&a=home&v=midlet",
	} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodHead, http.MethodDelete} {
			result := Handle(method, parseURL(t, path), User{}, nil)
			if result.Status != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: status = %d, want 405", method, path, result.Status)
			}
		}
	}
}

func parseURL(t *testing.T, value string) *url.URL {
	t.Helper()
	u, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
