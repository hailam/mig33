package acceptance

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/protocol"
)

const (
	formattingEditPath = "/sites/index.php?c=profile&a=edit&v=midlet&field=customization"
	formattingViewPath = "/sites/index.php?c=profile&a=home&v=midlet&username=alice"
)

func TestProfileFormattingOwnershipPrivacyAndReset(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	b := login(t, s, "bob")
	photo, err := s.store.AddPhoto(t.Context(), "alice", "An owned photo", token(), originalNeutralIcon)
	if err != nil {
		t.Fatal(err)
	}

	source := fmt.Sprintf(`<body bgcolor="#fff0cc" background="photo:%d"><p style="color:red"><b>Hello &amp; welcome</b></p><img src="photo:%d" width="24" height="24"><a href="mig33:privateChat(bob)">Chat</a></body>`, photo.ID, photo.ID)
	post := func(sid, csrf, value, op string) int {
		return identityHTTP(t, s, sid, "POST", formattingEditPath, url.Values{"csrf": {csrf}, "op": {op}, "value": {value}}).Code
	}
	if code := post(a, "forged", source, "save"); code != 403 {
		t.Fatalf("CSRF=%d", code)
	}
	if code := post(b, s.session(b).csrf, source, "save"); code != 400 {
		t.Fatalf("foreign image=%d", code)
	}
	if code := post(a, s.session(a).csrf, source, "save"); code != 200 {
		t.Fatalf("save=%d", code)
	}

	w := identityHTTP(t, s, b, "GET", formattingViewPath, nil)
	if w.Code != 200 ||
		!strings.Contains(w.Body.String(), `bgcolor="#fff0cc"`) ||
		!strings.Contains(w.Body.String(), `<b>Hello &amp; welcome</b>`) ||
		!strings.Contains(w.Body.String(), fmt.Sprintf(`/assets/photos/%d`, photo.ID)) {
		t.Fatalf("profile=%d %s", w.Code, w.Body.String())
	}
	if err = s.store.SetIdentityPreference(t.Context(), "alice", "profile_privacy_settings", "private"); err != nil {
		t.Fatal(err)
	}
	if w = identityHTTP(t, s, b, "GET", formattingViewPath, nil); w.Code != 403 || strings.Contains(w.Body.String(), "Hello") {
		t.Fatal("private profile leaked")
	}
	if w = identityHTTP(t, s, b, "GET", fmt.Sprintf("/assets/photos/%d", photo.ID), nil); w.Code != 403 {
		t.Fatalf("private image=%d", w.Code)
	}
	if code := post(a, s.session(a).csrf, `<script>bad</script>`, "save"); code != 400 {
		t.Fatalf("bad style=%d", code)
	}
	if stored, _ := s.store.ProfileFormatting(t.Context(), "alice"); stored != source {
		t.Fatal("rejected formatting mutated profile")
	}
	if err = s.store.DeletePhoto(t.Context(), "alice", photo.ID); err != nil {
		t.Fatal(err)
	}

	w = identityHTTP(t, s, a, "GET", formattingViewPath, nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "/assets/photos/") {
		t.Fatalf("deleted photo broke profile %d %s", w.Code, w.Body.String())
	}
	if code := post(a, s.session(a).csrf, "", "reset"); code != 200 {
		t.Fatalf("reset=%d", code)
	}

	w = identityHTTP(t, s, a, "GET", formattingViewPath, nil)
	if strings.Contains(w.Body.String(), "bgcolor") || strings.Contains(w.Body.String(), "Hello &amp;") {
		t.Fatal("reset retained styling")
	}
}

func TestNativeProfileFormattingSerializationAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.sqlite")
	s, err := Open(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	a := login(t, s, "alice")
	source := `<p style="color:yellow">A &amp; B + 50% = ✓</p>`
	raw := "csrf=" + s.session(a).csrf + "&op=save&value=" + strings.ReplaceAll(source, " ", "%20")
	r, _ := s.Handle(t.Context(), a, protocol.New(922, 70).SetByte(1, 2).SetText(2, s.base.String()+formattingEditPath).SetText(3, raw))
	status, _ := r[0].Int(1)
	if status != 200 {
		t.Fatalf("native save=%d %s", status, r[0].Fields[2])
	}
	if stored, _ := s.store.ProfileFormatting(t.Context(), "alice"); stored != source {
		t.Fatalf("source corrupted %q", stored)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = Open(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a = login(t, s, "alice")
	w := identityHTTP(t, s, a, "GET", formattingViewPath, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `style="color:#ffff00"`) || !strings.Contains(w.Body.String(), `A &amp; B + 50% = ✓`) {
		t.Fatalf("restart=%d %s", w.Code, w.Body.String())
	}
}
