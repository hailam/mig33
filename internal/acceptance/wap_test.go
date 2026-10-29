package acceptance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	chatstore "github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/protocol"
)

func wapRequest(s *fixture, id, method, path string, f url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(f.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("sid", id)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func wapSendForm(s *fixture, id, source string) url.Values {
	return url.Values{"csrf": {s.session(id).csrf}, "nonce": {token()}, "op": {"send"}, "format": {"html"}, "body": {source}}
}

func TestWAPFormattingDeliveryAndExactRetry(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	source := `<p style="color:red"><b>Hello &amp; café</b></p>`
	f := wapSendForm(s, alice, source)
	path := wapChatPath + "?kind=private&target=bob"
	for i := 0; i < 2; i++ {
		w := wapRequest(s, alice, "POST", path, f)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `<p style="color:#ff0000"><b>Hello &amp; café</b></p>`) {
			t.Fatalf("formatted send %d: %d %s", i, w.Code, w.Body.String())
		}
	}
	history, err := s.store.History(t.Context(), "alice", 1, "bob", 0, 100)
	if err != nil || len(history) != 1 || history[0].Body != "Hello & café" || history[0].Markup != source {
		t.Fatalf("history=%+v %v", history, err)
	}
	packets := requestPacket(t, s, bob, nil)
	if len(packets) != 1 {
		t.Fatalf("received %d packets", len(packets))
	}
	p := packets[0]
	body, _ := p.Text(8)
	sender, _ := p.Text(2)
	rgb, ok := p.Int(13)
	if body != "Hello & café" || sender != "alice" || !ok || rgb != 0xff0000 {
		t.Fatalf("native fallback %+v", p)
	}
	f.Set("body", `<b>changed</b>`)
	if w := wapRequest(s, alice, "POST", path, f); w.Code != 400 {
		t.Fatal("nonce conflict accepted")
	}
	f = wapSendForm(s, alice, `<span style="color:red">Red</span> <span style="color:blue">blue</span>`)
	if w := wapRequest(s, alice, "POST", path, f); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	p = requestPacket(t, s, bob, nil)[0]
	if _, ok := p.Int(13); ok {
		t.Fatal("mixed colours falsely collapsed into one")
	}
	if body, _ := p.Text(8); body != "Red blue" {
		t.Fatalf("plain fallback %q", body)
	}
	if w := wapRequest(s, bob, "GET", wapChatPath+"?kind=private&target=alice", nil); !strings.Contains(w.Body.String(), `color:#0000ff`) {
		t.Fatal("recipient lost rich history")
	}
}

func TestWAPRejectsInvalidMarkupAndPreservesPlainNativeText(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	path := wapChatPath + "?kind=private&target=bob"
	for _, source := range []string{
		`<script>evil</script>`,
		`<b onclick="x">hello</b>`,
		`<body bgcolor="red">x</body>`,
		`<img src="photo:1">`,
		`<span style="color:red">unclosed`,
		`<b></b>`,
	} {
		if w := wapRequest(s, alice, "POST", path, wapSendForm(s, alice, source)); w.Code != 400 {
			t.Fatalf("accepted %s", source)
		}
	}
	f := wapSendForm(s, alice, `<b>hello</b>`)
	f.Set("csrf", "wrong")
	if w := wapRequest(s, alice, "POST", path, f); w.Code != 403 {
		t.Fatal("invalid csrf accepted")
	}
	history, _ := s.store.History(t.Context(), "alice", 1, "bob", 0, 100)
	if len(history) != 0 {
		t.Fatal("invalid sends mutated history")
	}
	f = wapSendForm(s, alice, `<b>/ban bob</b>`)
	if w := wapRequest(s, alice, "POST", path, f); w.Code != 200 {
		t.Fatal("literal formatted slash failed", w.Body.String())
	}
	history, _ = s.store.History(t.Context(), "alice", 1, "bob", 0, 100)
	if len(history) != 1 || history[0].Action || history[0].Body != "/ban bob" {
		t.Fatal("formatted slash became command")
	}
	requestPacket(t, s, bob, nil)
	raw := `[color=red]literal</color> <b>text</b>`
	s.Handle(t.Context(), alice, privateMessage("bob", raw))
	packets := requestPacket(t, s, bob, nil)
	body, _ := packets[0].Text(8)
	if body != raw {
		t.Fatal("ordinary native input was silently reinterpreted")
	}
}

func TestWAPNativeFormAndPrivacy(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	source := `<p style="color:green">A & B + 50% = ✓</p>`
	form := "csrf=" + s.session(alice).csrf + "&nonce=" + token() + "&op=send&format=html&body=" + strings.ReplaceAll(source, " ", "%20")
	u := "http://127.0.0.1:8080" + wapChatPath + "?kind=private&target=bob"
	r, _ := s.Handle(t.Context(), alice, protocol.New(922, 100).SetByte(1, 2).SetText(2, u).SetText(3, form).SetInt(4, 1))
	status, _ := r[0].Int(1)
	if status != 200 {
		t.Fatalf("native form %+v", r[0])
	}
	history, _ := s.store.History(t.Context(), "bob", 1, "alice", 0, 100)
	if len(history) != 1 || history[0].Body != "A & B + 50% = ✓" || history[0].Markup != source {
		t.Fatal("native punctuation lost", history)
	}
	if err := s.store.SetIdentityPreference(t.Context(), "bob", "chat_privacy", "nobody"); err != nil {
		t.Fatal(err)
	}
	if w := wapRequest(s, alice, "POST", wapChatPath+"?kind=private&target=bob", wapSendForm(s, alice, "<b>no</b>")); w.Code != 400 {
		t.Fatal("privacy bypass")
	}
	s.store.SetIdentityPreference(t.Context(), "bob", "chat_privacy", "everyone")
	s.store.SetBlocked(t.Context(), "bob", "alice", true)
	w := wapRequest(s, alice, "POST", wapChatPath+"?kind=private&target=bob", wapSendForm(s, alice, "<b>blocked</b>"))
	if w.Code != 200 {
		t.Fatal("block should not reveal recipient state")
	}
	history, _ = s.store.History(t.Context(), "bob", 1, "alice", 0, 100)
	if len(history) != 1 {
		t.Fatal("blocked rich message leaked")
	}
	_ = bob
}

func TestWAPBrowserLoginRoomQueuesAndLogout(t *testing.T) {
	s := newTestServer(t)
	w := wapRequest(s, "", "GET", wapChatPath, nil)
	cookies := w.Result().Cookies()
	if w.Code != 200 || len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatal("sign in form")
	}
	f := url.Values{"op": {"login"}, "csrf": {cookies[0].Value}, "username": {"bob"}, "password": {"bob-demo"}}
	if w := wapRequest(s, "", "POST", wapChatPath, f); w.Code != 403 {
		t.Fatal("login csrf accepted")
	}

	w = wapRequest(s, "", "POST", wapChatPath, f, cookies...)
	if w.Code != 303 {
		t.Fatalf("login=%d %s", w.Code, w.Body.String())
	}
	var browserCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "mig33_wap" {
			browserCookie = c
		}
	}
	if browserCookie == nil {
		t.Fatal("browser cookie missing")
	}
	x := s.session(browserCookie.Value)
	if x == nil || !x.webOnly {
		t.Fatal("browser session missing")
	}
	roomPath := wapChatPath + "?kind=room&target=Lobby"
	f = url.Values{"csrf": {x.csrf}, "op": {"join"}}
	w = wapRequest(s, "", "POST", roomPath, f, browserCookie)
	if w.Code != 200 || !s.session(x.id).rooms["Lobby"] {
		t.Fatal("join failed", w.Body.String())
	}
	alice := login(t, s, "alice")
	s.Handle(t.Context(), alice, protocol.New(703, 44).SetText(1, "Lobby"))
	for i := 0; i < 135; i++ {
		p := protocol.New(500, 51).SetByte(1, 1).SetByte(3, 3).SetText(4, "Lobby").SetShort(6, 1).SetText(8, fmt.Sprintf("web room %d", i))
		r, _ := s.Handle(t.Context(), alice, p)
		if r[0].Opcode != 1 {
			t.Fatal("browser history-only participant filled native queue", i, r[0])
		}
	}
	if len(s.Native.Poll(t.Context(), x.id, 256)) != 0 {
		t.Fatal("browser accumulated native packets")
	}

	w = wapRequest(s, "", "GET", roomPath, nil, browserCookie)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Next messages") {
		t.Fatal("browser history pagination missing")
	}

	w = wapRequest(s, "", "GET", "/sites/index.php?c=profile&a=home&v=midlet", nil, browserCookie)
	if w.Code != 200 {
		t.Fatal("browser profile cookie not authorized")
	}

	w = wapRequest(s, "", "POST", wapChatPath, url.Values{"csrf": {x.csrf}, "op": {"logout"}}, browserCookie)
	if w.Code != 303 || s.session(x.id) != nil || len(s.roomParticipants("Lobby")) != 1 {
		t.Fatal("logout did not remove browser room membership")
	}
}

func TestFormattedAPIHistorySurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "formatted.sqlite")
	s, err := Open(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	id := login(t, s, "alice")
	source := `<font color="blue"><u>Saved</u></font>`
	payload, _ := json.Marshal(map[string]any{"kind": "private", "target": "bob", "markup": source})
	key := strings.Repeat("a", 48)
	w := transcriptHTTP(t, s, id, "POST", "/api/chat/messages", string(payload), key)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	s.Close()
	s, err = Open(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id = login(t, s, "alice")
	w = transcriptHTTP(t, s, id, "POST", "/api/chat/messages", string(payload), key)
	if w.Code != 200 {
		t.Fatal("retry across restart", w.Code, w.Body.String())
	}
	var result struct {
		Message  chatstore.Message
		Replayed bool
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil ||
		!result.Replayed ||
		result.Message.Markup != source ||
		result.Message.Body != "Saved" ||
		result.Message.Color == nil ||
		*result.Message.Color != 255 {
		t.Fatal("stored markup/colour lost", result, err)
	}
	bob := login(t, s, "bob")
	w = transcriptHTTP(t, s, bob, "GET", "/api/chat/sync", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"markup"`) {
		t.Fatal("sync lost formatting")
	}
}

func TestWAPDirectoryPrivacyAndCommandBoundaries(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	if err := s.store.SetIdentityPreference(t.Context(), "bob", "profile_privacy_settings", "private"); err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetBlocked(t.Context(), "alice", "carol", true); err != nil {
		t.Fatal(err)
	}
	w := wapRequest(s, alice, "GET", wapChatPath, nil)
	if strings.Contains(w.Body.String(), "Chat with bob") || strings.Contains(w.Body.String(), "Chat with carol") || !strings.Contains(w.Body.String(), "Chat with dave") {
		t.Fatal("directory bypassed discovery visibility")
	}
	for _, command := range []string{"/play", "/scores", "/announce hello", "/invite bob", "/bot dice", "/ban bob"} {
		f := wapSendForm(s, alice, command)
		f.Set("format", "plain")
		if w := wapRequest(s, alice, "POST", wapChatPath+"?kind=room&target=Lobby", f); w.Code != 400 || !strings.Contains(w.Body.String(), "room menu") {
			t.Fatalf("non-idempotent command accepted: %s %d %s", command, w.Code, w.Body.String())
		}
	}
	f := wapSendForm(s, alice, "/roll")
	f.Set("format", "plain")
	for i := 0; i < 2; i++ {
		if w := wapRequest(s, alice, "POST", wapChatPath+"?kind=private&target=dave", f); w.Code != 200 {
			t.Fatal("core action failed", w.Body.String())
		}
	}
	history, _ := s.store.History(t.Context(), "alice", 1, "dave", 0, 100)
	if len(history) != 1 || !history[0].Action {
		t.Fatal("WAP action replay was not idempotent")
	}
}
