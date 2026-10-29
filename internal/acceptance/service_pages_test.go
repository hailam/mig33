package acceptance

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/protocol"
)

func serviceTestForm(s *fixture, sid, op, target, body string) url.Values {
	return url.Values{"csrf": {s.session(sid).csrf}, "nonce": {token()}, "op": {op}, "target": {hex.EncodeToString([]byte(target))}, "body": {body}}
}

func TestFullHistoricalPageCatalogHasWorkingHandlers(t *testing.T) {
	s := newTestServer(t)
	sid := login(t, s, "alice")
	raw, err := os.ReadFile("../pages/routes.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Routes []struct{ ID, Template, Controller, Action string }
	}
	if err = json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, entry := range catalog.Routes {
		t.Run(entry.ID, func(t *testing.T) {
			template := strings.ReplaceAll(entry.Template, "%s", "")
			u, _ := url.Parse(template)
			q := u.Query()
			switch q.Get("c") + "/" + q.Get("a") {
			case "report/report_abuse":
				joinRoom(t, s, sid, "Lobby")
				q.Set("subject", "Chatroom")
				q.Set("subject_id", "Lobby")
			case "chatroom/setup", "group/add_to_group":
				q.Set("roomName", "Lobby")
			case "im/setup_im":
				q.Set("im", "msn")
			case "call/home":
				q.Set("number_to_call", "+9607771234")
			case "sms/home":
				q.Set("to_number", "+9607771234")
			}
			u.RawQuery = q.Encode()
			w := fetchReport(t, s, sid, "GET", s.base.String()+u.String(), "")
			if w.Code == 501 || w.Code == 404 || w.Code >= 500 {
				t.Fatalf("catalog page %s not handled: %d %s", entry.Template, w.Code, w.Body.String())
			}
		})
	}
}

func TestNativeServiceTextPersistenceAndAuthorization(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	b := login(t, s, "bob")
	u := s.featureURL("buzz", "home")
	f := serviceTestForm(s, a, "publish", "buzz", "")
	// The native serializer leaves punctuation raw; body must be the final tail.
	body := "Hello &op=delete + 50% ✓ <script>"
	raw := "csrf=" + f.Get("csrf") + "&nonce=" + f.Get("nonce") + "&op=publish&target=" + f.Get("target") + "&body=" + body
	for i := 0; i < 2; i++ {
		r, _ := s.Handle(t.Context(), a, protocol.New(922, 90).SetByte(1, 2).SetText(2, u).SetText(3, raw))
		code, _ := r[0].Int(1)
		if code != 200 {
			t.Fatalf("native post %d: %+v", code, r[0])
		}
	}
	entries, err := s.store.ServiceEntries(t.Context(), "buzz", "alice", "")
	if err != nil || len(entries) != 1 || entries[0].Body != body {
		t.Fatalf("native body or idempotency: %+v %v", entries, err)
	}
	w := fetchReport(t, s, b, "GET", u, "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "<script>") || !strings.Contains(w.Body.String(), "&lt;script&gt;") {
		t.Fatal("publication escaping")
	}
	f = serviceTestForm(s, b, "delete", strconv.FormatInt(entries[0].ID, 10), "")
	if w = fetchReport(t, s, b, "POST", u, f.Encode()); w.Code != 404 {
		t.Fatalf("another user's delete: %d", w.Code)
	}
	f = serviceTestForm(s, a, "publish", "buzz", "forged")
	f.Set("csrf", "wrong")
	if w = fetchReport(t, s, a, "POST", u, f.Encode()); w.Code != 403 {
		t.Fatal("forged mutation accepted")
	}
	if w = fetchReport(t, s, "", "GET", u, ""); w.Code != 401 {
		t.Fatal("anonymous feed accepted")
	}
}

func TestProviderRequestsAreHonestAndLocalMailIsDelivered(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	b := login(t, s, "bob")
	for _, test := range []struct{ controller, action, target, body, state, kind string }{
		{"email", "compose", "bob", "Local hello", "local_delivered", "mail"},
		{"email", "compose", "person@example.test", "External hello", "pending_provider", "mail"},
		{"sms", "home", "+9607771234", "SMS draft", "pending_provider", "sms"},
		{"call", "home", "+9607771234", "+9607775678", "pending_provider", "call"},
		{"help", "contact_us", "support", "Bug report", "open", "support"},
		{"im", "setup_im", "msn", "old-handle", "disconnected", "im"},
	} {
		f := serviceTestForm(s, a, "send", test.target, test.body)
		w := fetchReport(t, s, a, "POST", s.featureURL(test.controller, test.action), f.Encode())
		if w.Code != 200 || !strings.Contains(w.Body.String(), test.state) {
			t.Fatalf("%s: %d %s", test.kind, w.Code, w.Body.String())
		}
		entry, e := s.store.ServiceEntryByNonce(t.Context(), "alice", f.Get("nonce"))
		if e != nil || entry.State != test.state {
			t.Fatalf("%+v %v", entry, e)
		}
	}
	w := fetchReport(t, s, b, "GET", s.featureURL("email", "compose"), "")
	if !strings.Contains(w.Body.String(), "Local hello") || strings.Contains(w.Body.String(), "External hello") {
		t.Fatal("mail inbox isolation")
	}
}

func TestGameReplayKeepsOriginalResult(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	f := serviceTestForm(s, a, "play", "roll", "")
	u := s.featureURL("bot", "bot_list")
	for i := 0; i < 2; i++ {
		if w := fetchReport(t, s, a, "POST", u, f.Encode()); w.Code != 200 {
			t.Fatalf("game %d %s", w.Code, w.Body.String())
		}
	}
	rows, e := s.store.ServiceEntries(t.Context(), "game", "alice", "")
	if e != nil || len(rows) != 1 {
		t.Fatalf("game duplicate %+v %v", rows, e)
	}
	f.Set("target", hex.EncodeToString([]byte("8ball")))
	if w := fetchReport(t, s, a, "POST", u, f.Encode()); w.Code != 409 {
		t.Fatalf("nonce game switch accepted: %d", w.Code)
	}
}

func TestPhotoUploadReadAndOwnerActions(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	b := login(t, s, "bob")
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	req := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/photos?caption=Test", bytes.NewReader(data.Bytes()))
	req.Header.Set("sid", a)
	req.Header.Set("X-CSRF-Token", s.session(a).csrf)
	req.Header.Set("Idempotency-Key", token())
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload %d %s", w.Code, w.Body.String())
	}
	var reply struct {
		ID  int64
		URL string
	}
	json.Unmarshal(w.Body.Bytes(), &reply)
	get := httptest.NewRequest("GET", reply.URL, nil)
	get.Header.Set("sid", b)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, get)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), data.Bytes()) {
		t.Fatal("photo bytes")
	}
	u := s.featureURL("avatar", "home")
	f := serviceTestForm(s, b, "avatar", strconv.FormatInt(reply.ID, 10), "")
	if w = fetchReport(t, s, b, "POST", u, f.Encode()); w.Code != 403 {
		t.Fatalf("stolen avatar accepted %d", w.Code)
	}
	f = serviceTestForm(s, a, "avatar", strconv.FormatInt(reply.ID, 10), "")
	if w = fetchReport(t, s, a, "POST", u, f.Encode()); w.Code != 200 {
		t.Fatal("own avatar failed")
	}
	id, e := s.store.PhotoAvatar(t.Context(), "alice")
	if e != nil || id != reply.ID {
		t.Fatal("avatar not saved")
	}
}
