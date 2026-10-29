package acceptance

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/adapters/legacyweb"
	"github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/protocol"
	"github.com/hailam/mig33/internal/review"
	"github.com/hailam/mig33/internal/testassets"
	"github.com/hailam/mig33/internal/testsupport"
)

type fixture struct {
	*testsupport.Fixture
	t     *testing.T
	store *repositories.Store
	base  *url.URL
}

func Open(t *testing.T, address, path string) (*fixture, error) {
	t.Helper()
	f := testsupport.Open(t, address, path)
	base, err := url.Parse(address)
	return &fixture{f, t, f.Repository, base}, err
}

func New(t *testing.T, address string) (*fixture, error) { return Open(t, address, ":memory:") }

func newTestServer(t *testing.T) *fixture {
	t.Helper()
	f, err := New(t, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) ServeHTTP(w http.ResponseWriter, r *http.Request) { f.Handler.ServeHTTP(w, r) }

func (f *fixture) Handle(ctx context.Context, id string, p *protocol.Packet) ([]*protocol.Packet, string) {
	return f.Native.Handle(ctx, id, p)
}

func (f *fixture) DropSession(ctx context.Context, id string) { f.Native.DropSession(ctx, id) }

func (f *fixture) SetRoomModerator(ctx context.Context, room, user string, enabled bool) error {
	return f.Core.SetRoomModerator(ctx, room, user, enabled)
}

func (f *fixture) ListReports(ctx context.Context, status string, limit int) ([]review.AbuseReport, error) {
	return f.Review.ListReports(ctx, status, limit)
}

func (f *fixture) ReviewReport(ctx context.Context, id int64, status, note string) error {
	return f.Review.ReviewReport(ctx, id, status, note)
}

func (f *fixture) ReportDetails(ctx context.Context, id int64) (review.AbuseReport, []review.ReportReview, error) {
	return f.Review.ReportDetails(ctx, id)
}

func (f *fixture) featureURL(controller, action string) string {
	return f.base.String() + "/sites/index.php?c=" + controller + "&a=" + action + "&v=midlet"
}

func (f *fixture) roomPageAction(action string) string { return f.featureURL("chatroom", action) }

func (f *fixture) presence(name string) byte { return f.Core.Presence(f.t.Context(), name) }

func (f *fixture) roomParticipants(name string) []string {
	return f.Core.Participants(f.t.Context(), name)
}

type sessionSnapshot struct {
	id, username, csrf string
	webOnly            bool
	rooms              map[string]bool
}

func (f *fixture) session(id string) *sessionSnapshot {
	v, joined, err := f.Core.WebSession(f.t.Context(), id)
	if err != nil {
		return nil
	}
	x := &sessionSnapshot{id: v.ID, username: v.Username, csrf: v.CSRF, webOnly: v.WebOnly, rooms: map[string]bool{}}
	for _, room := range joined {
		x.rooms[room] = true
	}
	return x
}

func (f *fixture) identity(name string) accounts.Identity {
	a, err := f.Repository.Identity(f.t.Context(), name)
	if err != nil {
		f.t.Fatal(err)
	}
	return a
}

func (f *fixture) canContact(ctx context.Context, sender, target string) bool {
	v, err := accounts.NewPrivacy(f.Repository, f.Repository).CanContact(ctx, sender, target)
	return err == nil && v
}

func (f *fixture) identityChallenge(ctx context.Context, purpose, user, mobile string) (accounts.IdentityChallenge, error) {
	return accounts.NewService(f.Repository).Challenge(ctx, purpose, user, mobile)
}

func (f *fixture) registerIdentity(ctx context.Context, challenge, user, password, mobile, code string, activated bool) (accounts.Profile, error) {
	return f.Core.RegisterIdentity(ctx, accounts.Registration{Challenge: challenge, Username: user, Password: password, Mobile: mobile, Code: code, Activated: activated})
}

func verifyPassword(identity accounts.Identity, password string) bool {
	return accounts.VerifyPassword(identity, password)
}

const (
	DevelopmentOTP = accounts.DevelopmentOTP
	wapChatPath    = "/member2/t.php"
)

func token() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

var originalNeutralIcon = testassets.Bytes("default-avatar.png")

func login(t *testing.T, s *fixture, user string) string {
	t.Helper()
	r, id := s.Handle(t.Context(), "", protocol.New(200, 10).SetText(5, user))
	if len(r) != 1 || r[0].Opcode != 201 {
		t.Fatalf("challenge: %v", r)
	}
	challenge, _ := r[0].Text(1)
	r, id = s.Handle(t.Context(), id, protocol.New(202, 11).SetInt(1, protocol.JavaHash(challenge+user+"-demo")))
	if len(r) != 1 || r[0].Opcode != 203 {
		t.Fatalf("login: %v", r)
	}
	if v, ok := r[0].Short(31); !ok || v != 0 {
		t.Fatal("required legacy short missing")
	}
	s.Handle(t.Context(), id, protocol.New(400, 12))
	return id
}

func joinRoom(t *testing.T, s *fixture, id, name string) {
	t.Helper()
	r, _ := s.Handle(t.Context(), id, protocol.New(703, 20).SetText(1, name))
	if len(r) < 2 || r[0].Opcode != 701 || r[1].Opcode != 1 {
		t.Fatalf("join: %v", r)
	}
}

func containsText(packets []*protocol.Packet, text string) bool {
	for _, p := range packets {
		if p.Opcode == 500 {
			if body, _ := p.Text(8); body == text {
				return true
			}
		}
	}
	return false
}

func roomHiddenFields(body string) url.Values {
	fields := url.Values{}
	for _, match := range regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`).FindAllStringSubmatch(body, -1) {
		fields.Set(match[1], match[2])
	}
	return fields
}

func requestPacket(t *testing.T, h http.Handler, id string, p *protocol.Packet) []*protocol.Packet {
	t.Helper()
	var body bytes.Buffer
	if p != nil {
		if err := protocol.Encode(&body, p); err != nil {
			t.Fatal(err)
		}
	}
	method := "POST"
	if p == nil {
		method = "GET"
	}
	r := httptest.NewRequest(method, "http://127.0.0.1:8080/"+id, &body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "image/gif" {
		t.Fatal("legacy content type")
	}
	var out []*protocol.Packet
	for w.Body.Len() > 0 {
		p, err := protocol.Decode(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

func parseNativeWAPForm(body string) (url.Values, error) {
	u, _ := url.Parse(wapChatPath)
	return legacyweb.ParseNativeForm(u, body)
}

func loginHailam(t *testing.T, s *fixture) string {
	t.Helper()
	r, id := s.Handle(t.Context(), "", protocol.New(200, 1).SetText(5, "hailam"))
	challenge, _ := r[0].Text(1)
	r, id = s.Handle(t.Context(), id, protocol.New(202, 2).SetInt(1, protocol.JavaHash(challenge+"hailam")))
	if len(r) != 1 || r[0].Opcode != 203 {
		t.Fatalf("Hailam login failed: %v", r)
	}
	s.Handle(t.Context(), id, protocol.New(400, 3))
	return id
}

func parseNativeReportForm(body string) (url.Values, error) {
	u, _ := url.Parse("/sites/index.php?c=report&a=report_abuse&v=midlet")
	return legacyweb.ParseNativeForm(u, body)
}
