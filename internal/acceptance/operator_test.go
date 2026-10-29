package acceptance

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	chatstore "github.com/hailam/mig33/internal/adapters/sqlite/repositories"
)

func operatorRequest(s *fixture, method, path, sid string, f url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var body string
	if f != nil {
		body = f.Encode()
	}
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(body))
	r.Header.Set("sid", sid)
	if method == "POST" {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestOperatorAccessLoginAndLogout(t *testing.T) {
	s := newTestServer(t)
	if r := operatorRequest(s, "GET", "/operator/login", "", nil); r.Code != 404 {
		t.Fatalf("disabled operator: %d", r.Code)
	}
	if err := s.Core.ConfigureOperators([]string{"missing"}); err == nil {
		t.Fatal("unknown operator accepted")
	}
	if err := s.Core.ConfigureOperators([]string{"alice"}); err != nil {
		t.Fatal(err)
	}

	bob := login(t, s, "bob")
	if r := operatorRequest(s, "GET", "/operator", bob, nil); r.Code != 403 {
		t.Fatalf("ordinary user access: %d", r.Code)
	}

	r := operatorRequest(s, "GET", "/operator/login", "", nil)
	cookies := r.Result().Cookies()
	if r.Code != 200 || len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("login cookie policy")
	}

	f := url.Values{"username": {"alice"}, "password": {"alice-demo"}, "csrf": {cookies[0].Value}}
	if r := operatorRequest(s, "POST", "/operator/login", "", f); r.Code != 403 {
		t.Fatal("login accepted without binding cookie")
	}

	r = operatorRequest(s, "POST", "/operator/login", "", f, cookies...)
	if r.Code != 303 {
		t.Fatalf("operator login: %d %s", r.Code, r.Body.String())
	}

	var auth *http.Cookie
	for _, c := range r.Result().Cookies() {
		if c.Name == "mig33_operator" {
			auth = c
		}
	}
	if auth == nil || auth.Path != "/operator" || !auth.HttpOnly {
		t.Fatal("missing operator cookie")
	}

	r = operatorRequest(s, "GET", "/operator", "", nil, auth)
	if r.Code != 200 || !strings.Contains(r.Body.String(), "Room categories") {
		t.Fatalf("dashboard: %d %s", r.Code, r.Body.String())
	}

	x := s.session(auth.Value)
	if r := operatorRequest(s, "POST", "/operator/logout", "", url.Values{"csrf": {"wrong"}}, auth); r.Code != 403 {
		t.Fatal("logout accepted without CSRF")
	}
	if r := operatorRequest(s, "POST", "/operator/logout", "", url.Values{"csrf": {x.csrf}}, auth); r.Code != 303 {
		t.Fatal("logout failed")
	}
	if s.session(auth.Value) != nil {
		t.Fatal("operator session retained after logout")
	}
}

func TestOperatorReviewAtomicHistoryAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operator.sqlite")
	s, err := Open(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if s != nil {
			s.Close()
		}
	}()
	if err := s.Core.ConfigureOperators([]string{"alice"}); err != nil {
		t.Fatal(err)
	}

	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	report, err := s.store.SubmitReport(t.Context(), chatstore.ReportSubmission{
		Reporter: "bob",
		Kind:     "room",
		Context:  "Lobby",
		Target:   "carol",
		Category: "spam",
		Details:  "Private evidence <script>alert(1)</script>",
		Nonce:    strings.Repeat("1", 48),
	})
	if err != nil {
		t.Fatal(err)
	}

	support, err := s.store.AddServiceEntry(t.Context(), chatstore.ServiceEntry{
		Kind:  "support",
		Owner: "bob",
		Body:  "Help with my account",
		State: "open",
		Nonce: strings.Repeat("2", 48),
	})
	if err != nil {
		t.Fatal(err)
	}

	f := url.Values{
		"csrf":     {s.session(alice).csrf},
		"nonce":    {strings.Repeat("3", 48)},
		"kind":     {"report"},
		"id":       {"1"},
		"status":   {"reviewed"},
		"expected": {"pending"},
		"note":     {"Checked <b>evidence</b>"},
	}
	if r := operatorRequest(s, "POST", "/operator/review", bob, f); r.Code != 403 {
		t.Fatal("ordinary user reviewed report")
	}

	bad := maps.Clone(f)
	bad.Set("csrf", "wrong")
	if r := operatorRequest(s, "POST", "/operator/review", alice, bad); r.Code != 403 {
		t.Fatal("missing review CSRF")
	}

	for i := 0; i < 2; i++ {
		if r := operatorRequest(s, "POST", "/operator/review", alice, f); r.Code != 303 {
			t.Fatalf("review/retry: %d %s", r.Code, r.Body.String())
		}
	}

	reviews, err := s.store.OperatorReviews(t.Context(), "report", report.ID)
	if err != nil || len(reviews) != 1 || reviews[0].Actor != "alice" {
		t.Fatalf("review actor/idempotency: %+v %v", reviews, err)
	}
	legacy, _ := s.store.ReportReviews(t.Context(), report.ID)
	if len(legacy) != 1 {
		t.Fatal("legacy history did not commit together")
	}

	bad = maps.Clone(f)
	bad.Set("nonce", strings.Repeat("4", 48))
	bad.Set("status", "dismissed")
	if r := operatorRequest(s, "POST", "/operator/review", alice, bad); r.Code != 409 {
		t.Fatal("stale operator form overwrote decision")
	}

	r := operatorRequest(s, "GET", "/operator?kind=report&id=1", alice, nil)
	if r.Code != 200 ||
		strings.Contains(r.Body.String(), "<script>") ||
		!strings.Contains(r.Body.String(), "&lt;script&gt;") ||
		!strings.Contains(r.Body.String(), "alice — reviewed") {
		t.Fatalf("detail/escaped audit: %d %s", r.Code, r.Body.String())
	}

	f.Set("kind", "support")
	f.Set("id", "1")
	f.Set("status", "completed")
	f.Set("expected", "open")
	f.Set("nonce", strings.Repeat("5", 48))
	f.Set("note", "Resolved locally")
	if support.ID != 1 {
		t.Fatal("test requires first service entry")
	}
	if r := operatorRequest(s, "POST", "/operator/review", alice, f); r.Code != 303 {
		t.Fatalf("support review: %d %s", r.Code, r.Body.String())
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s = nil
	s, err = Open(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}

	restored, _ := s.store.Report(t.Context(), report.ID)
	item, _ := s.store.ServiceEntryByID(t.Context(), support.ID)
	reviews, _ = s.store.OperatorReviews(t.Context(), "support", support.ID)
	if restored.Status != "reviewed" || item.State != "completed" || len(reviews) != 1 || reviews[0].Actor != "alice" {
		t.Fatal("operator state/history did not persist")
	}

	if err := s.store.ReviewWorkItem(t.Context(), chatstore.OperatorReview{
		Kind:   "report",
		ItemID: report.ID,
		Actor:  "alice",
		Status: "pending",
		Note:   "Further investigation",
		Nonce:  strings.Repeat("6", 48),
	}, "reviewed"); err != nil {
		t.Fatalf("reopen report: %v", err)
	}

	reopened, _ := s.store.Report(t.Context(), report.ID)
	history, _ := s.store.ReportReviews(t.Context(), report.ID)
	if reopened.Status != "pending" || len(history) != 2 || history[1].Status != "pending" {
		t.Fatal("reopen status/audit not atomic")
	}
	if r := operatorRequest(s, "GET", "/operator/login", "", nil); r.Code != 404 {
		t.Fatal("operator access must remain opt-in each startup")
	}
}

func TestOperatorCategoryPermissions(t *testing.T) {
	s := newTestServer(t)
	if err := s.Core.ConfigureOperators([]string{"alice"}); err != nil {
		t.Fatal(err)
	}
	alice, bob := login(t, s, "alice"), login(t, s, "bob")
	f := url.Values{"csrf": {s.session(alice).csrf}, "op": {"save"}, "id": {"30"}, "name": {"Local languages"}}
	if r := operatorRequest(s, "POST", "/operator/categories", bob, f); r.Code != 403 {
		t.Fatal("nonoperator category mutation")
	}
	if r := operatorRequest(s, "POST", "/operator/categories", alice, f); r.Code != 303 {
		t.Fatalf("category mutation: %d %s", r.Code, r.Body.String())
	}
	categories, _ := s.store.RoomCategories(t.Context())
	found := false
	for _, c := range categories {
		found = found || c.ID == 30 && c.Name == "Local languages"
	}
	if !found {
		t.Fatal("category did not persist")
	}
	f.Set("op", "assign")
	f.Set("room", "Lobby")
	if r := operatorRequest(s, "POST", "/operator/categories", alice, f); r.Code != 303 {
		t.Fatalf("assign category: %d %s", r.Code, r.Body.String())
	}
	room, _ := s.store.Room(t.Context(), "Lobby")
	if room.Category != 30 {
		t.Fatal("room category assignment not saved")
	}
	f.Set("op", "delete")
	if r := operatorRequest(s, "POST", "/operator/categories", alice, f); r.Code != 409 {
		t.Fatal("deleted occupied category")
	}
	f.Set("op", "assign")
	f.Set("id", "1")
	operatorRequest(s, "POST", "/operator/categories", alice, f)
	f.Set("id", "30")
	f.Set("op", "delete")
	if r := operatorRequest(s, "POST", "/operator/categories", alice, f); r.Code != 303 {
		t.Fatal("empty category delete")
	}
}
