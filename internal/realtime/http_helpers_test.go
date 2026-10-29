package realtime

import (
	"bytes"
	"encoding/hex"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/protocol"
)

const (
	reportURL   = "http://127.0.0.1:8080/sites/index.php?c=report&a=report_abuse&v=midlet&type=4&subject=Chatroom&subject_id=Lobby"
	wapChatPath = "/member2/t.php"
)

func (s *Application) featureURL(controller, action string) string {
	return s.base.String() + "/sites/index.php?c=" + controller + "&a=" + action + "&v=midlet"
}

func (s *Application) ccURL(controller, action string) string {
	return s.featureURL(controller, action)
}

func fetchReport(t *testing.T, s *Application, sid, method, address, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, address, strings.NewReader(body))
	if sid != "" {
		r.Header.Set("sid", sid)
	}
	if method == http.MethodPost {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func reportFields(t *testing.T, s *Application, sid, address string) url.Values {
	t.Helper()
	w := fetchReport(t, s, sid, "GET", address, "")
	if w.Code != 200 {
		t.Fatalf("report form: %d %s", w.Code, w.Body.String())
	}
	form := url.Values{}
	for _, match := range regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`).FindAllStringSubmatch(w.Body.String(), -1) {
		form.Set(match[1], match[2])
	}
	if len(form) != 3 {
		t.Fatalf("hidden fields: %v", form)
	}
	form.Set("category", "spam")
	form.Set("target", "bob")
	form.Set("details", "Repeated spam & flooding + 50% noise = test ✓")
	return form
}

func wapRequest(s *Application, id, method, path string, f url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
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

func wapSendForm(s *Application, id, source string) url.Values {
	return url.Values{"csrf": {s.sessions[id].csrf}, "nonce": {token()}, "op": {"send"}, "format": {"html"}, "body": {source}}
}

func loginIdentity(t *testing.T, s *Application, user, password string) (string, *protocol.Packet) {
	t.Helper()
	r, id := s.Handle(t.Context(), "", protocol.New(200, 1).SetText(5, user))
	if len(r) != 1 || r[0].Opcode != 201 {
		t.Fatalf("challenge=%v", r)
	}
	challenge, _ := r[0].Text(1)
	r, id = s.Handle(t.Context(), id, protocol.New(202, 2).SetInt(1, protocol.JavaHash(challenge+strings.ToLower(password))))
	if len(r) != 1 || r[0].Opcode != 203 {
		t.Fatalf("login=%v", r)
	}
	return id, r[0]
}

func registerIdentityPacket(t *testing.T, s *Application, user, password, mobile string) {
	t.Helper()
	r, _ := s.Handle(t.Context(), "", protocol.New(101, 1))
	if len(r) != 1 || r[0].Opcode != 102 {
		t.Fatalf("registration=%v", r)
	}
	challenge, _ := r[0].Text(2)
	if _, err := png.Decode(bytes.NewReader(r[0].Fields[3])); err != nil {
		t.Fatal(err)
	}
	r, _ = s.Handle(t.Context(), "", protocol.New(103, 2).SetText(1, user).SetText(2, password).SetText(3, mobile).SetText(4, challenge).SetText(5, DevelopmentOTP))
	if len(r) != 1 || r[0].Opcode != 1 {
		t.Fatalf("register=%v", r)
	}
}

func identityHTTP(t *testing.T, s *Application, sid, method, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	if method == "POST" {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if sid != "" {
		r.Header.Set("sid", sid)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func identityHiddenValue(t *testing.T, body, name string) string {
	t.Helper()
	m := regexp.MustCompile(`name="` + name + `" value="([^"]+)"`).FindStringSubmatch(body)
	if len(m) != 2 {
		t.Fatalf("missing %s in %s", name, body)
	}
	return m[1]
}

func serviceTestForm(s *Application, sid, op, target, body string) url.Values {
	return url.Values{"csrf": {s.sessions[sid].csrf}, "nonce": {token()}, "op": {op}, "target": {hex.EncodeToString([]byte(target))}, "body": {body}}
}

func (s *Application) roomPageAction(action string) string { return s.featureURL("chatroom", action) }

func (s *Application) identityURL(controller, action string) string {
	return s.featureURL(controller, action)
}
