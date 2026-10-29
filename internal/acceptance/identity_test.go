package acceptance

import (
	"bytes"
	"image/png"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/protocol"
)

func loginIdentity(t *testing.T, s *fixture, user, password string) (string, *protocol.Packet) {
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

func registerIdentityPacket(t *testing.T, s *fixture, user, password, mobile string) {
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

func identityHTTP(t *testing.T, s *fixture, sid, method, path string, form url.Values) *httptest.ResponseRecorder {
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

func TestPersistentRegistrationActivationProfileAndCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.sqlite")
	s, err := Open(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}

	password := "MiXeD &+=% 😀"
	registerIdentityPacket(t, s, "new_person", password, "+9607771234")
	id, p := loginIdentity(t, s, "NEW_PERSON", password)
	if v, _ := p.Byte(2); v != 0 {
		t.Fatal("registration skipped native activation")
	}
	if mobile, _ := p.Text(1); mobile != "+9607771234" {
		t.Fatal(mobile)
	}

	r, _ := s.Handle(t.Context(), id, protocol.New(204, 3).SetText(1, "wrong"))
	if r[0].Opcode != 0 {
		t.Fatal("bad OTP accepted")
	}

	r, _ = s.Handle(t.Context(), id, protocol.New(204, 4).SetText(1, DevelopmentOTP))
	if r[0].Opcode != 1 {
		t.Fatal("activation rejected")
	}

	r, _ = s.Handle(t.Context(), id, protocol.New(601, 5).SetText(1, "persistent <status>"))
	if r[0].Opcode != 1 {
		t.Fatal("status rejected")
	}

	if err = s.store.UpdateIdentityField(t.Context(), "new_person", "bio", "About & me"); err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetIdentityPreference(t.Context(), "new_person", "chat_privacy", "friends"); err != nil {
		t.Fatal(err)
	}

	if err = s.Close(); err != nil {
		t.Fatal(err)
	}

	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(file, []byte(password)) || bytes.Contains(file, []byte(strings.ToLower(password))) {
		t.Fatal("plaintext password found in SQLite")
	}

	s, err = Open(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id, p = loginIdentity(t, s, "new_person", strings.ToUpper(password))
	if v, _ := p.Byte(2); v != 1 {
		t.Fatal("activation not persisted")
	}
	if v, _ := p.Text(28); v != "persistent <status>" {
		t.Fatal("status not persisted")
	}

	a, err := s.store.Identity(t.Context(), "new_person")
	if err != nil {
		t.Fatal(err)
	}
	if a.Bio != "About & me" || !verifyPassword(a, password) || verifyPassword(a, "not the password") {
		t.Fatal("persistent profile/credentials incorrect")
	}
	if a.LegacyHash != protocol.JavaHash(strings.ToLower(password)) {
		t.Fatal("legacy password hash")
	}
	if got, err := s.store.Contacts(t.Context(), "new_person"); err != nil || len(got.Contacts) != 0 {
		t.Fatalf("new account got seed friends: %+v %v", got, err)
	}

	w := identityHTTP(t, s, id, "GET", "/sites/index.php?c=profile&a=home&v=midlet", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "persistent &lt;status&gt;") || !strings.Contains(w.Body.String(), "About &amp; me") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestPublicRegistrationAndRecoverPasswordSingleUse(t *testing.T) {
	s := newTestServer(t)
	path := "/sites/index.php?c=registration&a=home&v=midlet"
	w := identityHTTP(t, s, "", "GET", path, nil)
	csrf := identityHiddenValue(t, w.Body.String(), "csrf")
	form := url.Values{
		"csrf":     {csrf},
		"username": {"webuser"},
		"mobile":   {"+9608881234"},
		"code":     {DevelopmentOTP},
		"agree":    {"on"},
		"password": {"Web &+= password"},
	}

	w = identityHTTP(t, s, "", "POST", path, form)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}

	w = identityHTTP(t, s, "", "POST", path, form)
	if w.Code == 201 {
		t.Fatal("signup token replayed")
	}

	id, _ := loginIdentity(t, s, "webuser", "Web &+= password")
	w = identityHTTP(t, s, "", "GET", "/sites/index.php?c=registration_api&a=check_username&v=midlet&username=webuser", nil)
	if w.Code != 200 || string(w.Body.Bytes()) != "[webuser1,webuser2,webuser3]" {
		t.Fatal(w.Code, w.Body.String())
	}

	w = identityHTTP(t, s, "", "GET", "/sites/index.php?c=profile&a=home&v=midlet", nil)
	if w.Code != 401 {
		t.Fatal("private page became public")
	}

	reset := "/sites/index.php?c=account&a=recover&v=midlet"
	w = identityHTTP(t, s, "", "GET", reset, nil)
	csrf = identityHiddenValue(t, w.Body.String(), "csrf")
	w = identityHTTP(t, s, "", "POST", reset, url.Values{"csrf": {csrf}, "mobile": {"+9608881234"}})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}

	csrf = identityHiddenValue(t, w.Body.String(), "csrf")
	form = url.Values{"csrf": {csrf}, "code": {DevelopmentOTP}, "password": {"Reset + & = password"}}
	w = identityHTTP(t, s, "", "POST", reset+"&stage=complete", form)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if s.session(id) != nil {
		t.Fatal("old session survived recovery")
	}
	loginIdentity(t, s, "webuser", "Reset + & = password")
	w = identityHTTP(t, s, "", "POST", reset+"&stage=complete", form)
	if w.Code == 200 {
		t.Fatal("reset token replayed")
	}

	r, _ := s.Handle(t.Context(), "", protocol.New(904, 10).SetText(2, "+9608881234"))
	if r[0].Opcode != 0 {
		t.Fatal("904 ACK would falsely claim SMS was sent")
	}
}

func TestNativePasswordFormPreservesRawTailAndInvalidatesSessions(t *testing.T) {
	s := newTestServer(t)
	id, _ := loginIdentity(t, s, "alice", "alice-demo")
	other, _ := loginIdentity(t, s, "alice", "alice-demo")
	u := "http://127.0.0.1:8080/sites/index.php?c=settings&a=change_password&v=midlet"
	csrf := s.session(id).csrf
	r, _ := s.Handle(t.Context(), id, protocol.New(922, 10).SetByte(1, 2).SetText(2, u).SetText(3, "csrf="+csrf+"&password=alice-demo"))
	if status, _ := r[0].Int(1); status != 200 {
		t.Fatalf("stage1 %v", r)
	}
	challenge := identityHiddenValue(t, string(r[0].Fields[2]), "challenge")
	password := "new &admin=true+%ZZ=😀"
	r, _ = s.Handle(t.Context(), id, protocol.New(922, 11).
		SetByte(1, 2).
		SetText(2, u+"&stage=new").
		SetText(3, "csrf="+csrf+"&challenge="+challenge+"&password="+strings.ReplaceAll(password, " ", "%20")))
	if status, _ := r[0].Int(1); status != 200 {
		t.Fatalf("stage2 %v %s", r, string(r[0].Fields[2]))
	}
	if s.session(other) != nil {
		t.Fatal("other session survived password change")
	}
	if s.session(id) == nil {
		t.Fatal("current session was lost")
	}
	if !verifyPassword(s.identity("alice"), password) {
		t.Fatal("native password was URL decoded or split")
	}
	loginIdentity(t, s, "alice", password)
}

func TestIdentityPrivacyEnforcesProfileAndMessageAndCSRF(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	path := "/sites/index.php?c=settings&a=chat_privacy&v=midlet"
	w := identityHTTP(t, s, bob, "POST", path, url.Values{"csrf": {"forged"}, "value": {"nobody"}})
	if w.Code != 403 {
		t.Fatal("forged preference update")
	}

	w = identityHTTP(t, s, bob, "POST", path, url.Values{"csrf": {s.session(bob).csrf}, "value": {"nobody"}})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}

	r, _ := s.Handle(t.Context(), alice, protocol.New(500, 14).SetByte(1, 1).SetByte(3, 1).SetShort(6, 1).SetText(4, "bob").SetText(8, "blocked by privacy"))
	if r[0].Opcode != 0 {
		t.Fatal("chat privacy ignored")
	}

	r, _ = s.Handle(t.Context(), alice, protocol.New(751, 15).SetText(1, "bob").SetText(2, "carol").SetByte(3, 1))
	if r[0].Opcode != 0 {
		t.Fatal("chat privacy ignored for new group")
	}

	r, _ = s.Handle(t.Context(), alice, protocol.New(751, 16).SetText(1, "carol").SetText(2, "dave").SetByte(3, 1))
	if r[0].Opcode != 750 {
		t.Fatal("allowed group rejected")
	}
	group, _ := r[0].Text(1)
	r, _ = s.Handle(t.Context(), alice, protocol.New(752, 17).SetText(1, group).SetText(2, "bob").SetByte(3, 1))
	if r[0].Opcode != 0 {
		t.Fatal("chat privacy ignored for existing group invitation")
	}
	if err := s.store.SetIdentityPreference(t.Context(), "bob", "profile_privacy_settings", "private"); err != nil {
		t.Fatal(err)
	}

	w = identityHTTP(t, s, alice, "GET", "/sites/index.php?c=profile&a=home&v=midlet&username=bob", nil)
	if w.Code != 403 {
		t.Fatal("private profile leaked")
	}

	w = identityHTTP(t, s, bob, "GET", "/sites/index.php?c=profile&a=home&v=midlet&username=bob", nil)
	if w.Code != 200 {
		t.Fatal("own profile hidden")
	}
	if err := s.store.SetIdentityPreference(t.Context(), "bob", "chat_privacy", "friends"); err != nil {
		t.Fatal(err)
	}
	if !s.canContact(t.Context(), "alice", "bob") {
		t.Fatal("existing contact rejected")
	}
	registerIdentityPacket(t, s, "stranger", "test-password", "+9601234567")
	if s.canContact(t.Context(), "stranger", "bob") {
		t.Fatal("stranger allowed")
	}
}

func TestRegistrationChallengeAttemptLimitAndExpiry(t *testing.T) {
	s := newTestServer(t)
	c, err := s.identityChallenge(t.Context(), "registration", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := s.registerIdentity(t.Context(), c.Token, "attempts", "password", "+9601234567", "wrong", false); err == nil {
			t.Fatal("bad OTP accepted")
		}
	}
	if _, err = s.registerIdentity(t.Context(), c.Token, "attempts", "password", "+9601234567", DevelopmentOTP, false); err == nil {
		t.Fatal("exhausted challenge accepted")
	}
	c, err = s.identityChallenge(t.Context(), "registration", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.ConsumeIdentityChallenge(t.Context(), c.Token, "recovery"); err == nil {
		t.Fatal("wrong purpose accepted")
	}
	if _, err = s.store.IdentityChallenge(t.Context(), c.Token, "registration"); err != nil {
		t.Fatal("wrong purpose consumed registration")
	}
	c.ExpiresAt = time.Now().Add(-time.Minute).UnixMilli()
	c.Token = token()
	if err = s.store.CreateIdentityChallenge(t.Context(), c); err == nil {
		t.Fatal("expired challenge created")
	}
}
