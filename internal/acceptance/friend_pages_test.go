package acceptance

import (
	"net/url"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/protocol"
)

const friendSearchURL = "http://127.0.0.1:8080/sites/index.php?c=profile&a=search&v=midlet"

func TestFriendDirectoryAndOfflineProfileActions(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	if w := fetchReport(t, s, "", "GET", friendSearchURL, ""); w.Code != 401 {
		t.Fatal("anonymous directory access")
	}
	w := fetchReport(t, s, h, "POST", friendSearchURL, "query=%20BO%20")
	if w.Code != 200 ||
		!strings.Contains(w.Body.String(), "mig33:privateChat(bob)") ||
		!strings.Contains(w.Body.String(), "mig33:inviteFriend(bob)") ||
		strings.Contains(w.Body.String(), "privateChat(alice)") {
		t.Fatalf("search result: %d %s", w.Code, w.Body.String())
	}

	w = fetchReport(t, s, h, "POST", friendSearchURL, "query=")
	if strings.Contains(w.Body.String(), "privateChat(hailam)") || strings.Count(w.Body.String(), "mig33:privateChat(") != 4 {
		t.Fatal("empty directory does not exclude viewer or include other accounts")
	}
	profile := "http://127.0.0.1:8080/sites/index.php?c=profile&v=midlet&a=home&username=bob"
	w = fetchReport(t, s, h, "GET", profile, "")
	if !strings.Contains(w.Body.String(), "Presence: Offline") || !strings.Contains(w.Body.String(), "mig33:privateChat(bob)") {
		t.Fatalf("offline profile cannot open chat: %s", w.Body.String())
	}
	// The page does not fake presence or add a contact as a side effect.
	if s.presence("bob") != 99 {
		t.Fatal("offline user became visible")
	}
	if _, err := s.store.ContactByUsername(t.Context(), "hailam", "bob"); err == nil {
		t.Fatal("viewing a search result added a contact")
	}
	r, _ := s.Handle(t.Context(), h, privateMessage("bob", "profile offline message"))
	if r[0].Opcode != 1 {
		t.Fatal("offline noncontact message rejected")
	}
	b := login(t, s, "bob")
	if !containsText(requestPacket(t, s, b, nil), "profile offline message") {
		t.Fatal("offline message not delivered on login")
	}
}

func TestFriendSearchNativeFormAndInvalidRequests(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	for _, query := range []string{"BO", "", "a&query=b+50%=✓"} {
		r, _ := s.Handle(t.Context(), h, protocol.New(922, 80).SetByte(1, 2).SetText(2, friendSearchURL).SetText(3, "query="+query))
		status, _ := r[0].Int(1)
		body, _ := r[0].Text(2)
		if status != 200 || (query == "BO" && !strings.Contains(body, "privateChat(bob)")) || (strings.Contains(query, "&") && !strings.Contains(body, "a&amp;query=b+50%=✓")) {
			t.Fatalf("native query %q: %d %s", query, status, body)
		}
	}
	// The applet omits empty text controls altogether.
	r, _ := s.Handle(t.Context(), h, protocol.New(922, 81).SetByte(1, 2).SetText(2, friendSearchURL).SetText(3, ""))
	body, _ := r[0].Text(2)
	if strings.Count(body, "mig33:privateChat(") != 4 {
		t.Fatal("omitted empty native query did not list accounts")
	}
	for _, body := range []string{"query=bob&query=alice", "query=bob&extra=1", "query=" + strings.Repeat("a", 129), "query=%00"} {
		if w := fetchReport(t, s, h, "POST", friendSearchURL, body); w.Code != 400 {
			t.Fatalf("bad form accepted %q: %d", body, w.Code)
		}
	}
	for _, test := range []struct {
		method, suffix string
		status         int
	}{
		{"PUT", "", 405}, {"GET", "&query=a&query=b", 400}, {"GET", "&c=profile", 404},
	} {
		if w := fetchReport(t, s, h, test.method, friendSearchURL+test.suffix, ""); w.Code != test.status {
			t.Fatalf("bad route %s %s: %d", test.method, test.suffix, w.Code)
		}
	}
	w := fetchReport(t, s, h, "POST", friendSearchURL, url.Values{"query": {`<script>"&`}}.Encode())
	if strings.Contains(w.Body.String(), "<script>") {
		t.Fatal("unescaped search text")
	}
}
