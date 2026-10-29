package acceptance

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/protocol"
)

const reportURL = "http://127.0.0.1:8080/sites/index.php?c=report&a=report_abuse&v=midlet&type=4&subject=Chatroom&subject_id=Lobby"

func fetchReport(t *testing.T, s *fixture, sid, method, address, body string) *httptest.ResponseRecorder {
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

func reportFields(t *testing.T, s *fixture, sid, address string) url.Values {
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

func TestReportSubmissionAuthenticationReplayAndPrivacy(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	b := login(t, s, "bob")
	c := login(t, s, "carol")
	if w := fetchReport(t, s, "", "GET", reportURL, ""); w.Code != 401 {
		t.Fatalf("anonymous form: %d", w.Code)
	}
	if w := fetchReport(t, s, h, "GET", reportURL, ""); w.Code != 403 {
		t.Fatalf("outsider form: %d", w.Code)
	}
	joinRoom(t, s, h, "Lobby")
	joinRoom(t, s, b, "Lobby")
	form := reportFields(t, s, h, reportURL)
	before, _ := s.ListReports(t.Context(), "", 100)
	if len(before) != 0 {
		t.Fatal("GET created a report")
	}
	if w := fetchReport(t, s, c, "POST", reportURL, form.Encode()); w.Code != 403 {
		t.Fatalf("outsider post: %d", w.Code)
	}
	joinRoom(t, s, c, "Lobby")
	if w := fetchReport(t, s, c, "POST", reportURL, form.Encode()); w.Code != 403 {
		t.Fatalf("cross-session CSRF: %d", w.Code)
	}
	w := fetchReport(t, s, h, "POST", reportURL, form.Encode())
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Report #1 saved for local review.") {
		t.Fatalf("submission: %d %s", w.Code, w.Body.String())
	}
	if w := fetchReport(t, s, h, "POST", reportURL, form.Encode()); w.Code != 200 {
		t.Fatalf("duplicate form: %d", w.Code)
	}
	reports, err := s.ListReports(t.Context(), "pending", 100)
	if err != nil || len(reports) != 1 || reports[0].Reporter != "hailam" || reports[0].Target != "bob" || reports[0].Details != form.Get("details") {
		t.Fatalf("stored report: %+v %v", reports, err)
	}
	form.Set("details", "Changed content")
	if w := fetchReport(t, s, h, "POST", reportURL, form.Encode()); w.Code != 409 {
		t.Fatalf("changed replay: %d", w.Code)
	}
	if !s.session(b).rooms["Lobby"] {
		t.Fatal("report automatically removed the target")
	}
	if w := fetchReport(t, s, b, "GET", reportURL+"&report_id=1", ""); w.Code != 200 || strings.Contains(w.Body.String(), "Repeated spam") {
		t.Fatal("user route disclosed another report")
	}
	if w := fetchReport(t, s, b, "GET", "http://127.0.0.1:8080/sites/index.php?c=report&a=list&v=midlet", ""); w.Code == 200 {
		t.Fatal("operator report queue exposed to client")
	}
}

func TestReportContextBindingValidationAndMembershipChange(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	b := login(t, s, "bob")
	joinRoom(t, s, h, "Lobby")
	joinRoom(t, s, b, "Lobby")
	joinRoom(t, s, h, "Gaming")
	form := reportFields(t, s, h, reportURL)
	if w := fetchReport(t, s, h, "POST", strings.Replace(reportURL, "Lobby", "Gaming", 1), form.Encode()); w.Code != 403 {
		t.Fatalf("context binding: %d", w.Code)
	}
	for _, mutate := range []func(url.Values){
		func(v url.Values) { v.Del("csrf") }, func(v url.Values) { v.Add("csrf", v.Get("csrf")) },
		func(v url.Values) { v.Set("reporter", "alice") }, func(v url.Values) { v.Set("target", "dave") },
		func(v url.Values) { v.Set("category", "unknown") }, func(v url.Values) { v.Set("details", " ") },
		func(v url.Values) { v.Set("details", strings.Repeat("x", 2001)) },
	} {
		copy := url.Values{}
		for key, values := range form {
			copy[key] = append([]string(nil), values...)
		}
		mutate(copy)
		if w := fetchReport(t, s, h, "POST", reportURL, copy.Encode()); w.Code != 400 && w.Code != 403 {
			t.Fatalf("invalid form %v: %d", copy, w.Code)
		}
	}
	r := httptest.NewRequest("POST", reportURL, strings.NewReader(form.Encode()))
	r.Header.Set("sid", h)
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 415 {
		t.Fatalf("wrong content type: %d", w.Code)
	}
	if w := fetchReport(t, s, h, "POST", reportURL, "details="+strings.Repeat("x", 8193)); w.Code != 400 {
		t.Fatalf("oversize form: %d", w.Code)
	}
	s.Handle(t.Context(), b, protocol.New(704, 40).SetText(1, "Lobby"))
	if w := fetchReport(t, s, h, "POST", reportURL, form.Encode()); w.Code != 400 {
		t.Fatalf("departed target: %d", w.Code)
	}
	form.Del("target")
	s.Handle(t.Context(), h, protocol.New(704, 41).SetText(1, "Lobby"))
	if w := fetchReport(t, s, h, "POST", reportURL, form.Encode()); w.Code != 403 {
		t.Fatalf("departed reporter: %d", w.Code)
	}
	all, _ := s.ListReports(t.Context(), "", 100)
	if len(all) != 0 {
		t.Fatal("invalid forms persisted")
	}
}

func TestOriginalReport922PreservesRawDetailsAndCannotInjectControls(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	b := login(t, s, "bob")
	joinRoom(t, s, h, "Lobby")
	joinRoom(t, s, b, "Lobby")
	form := reportFields(t, s, h, reportURL)
	details := "Spam &csrf=forged&target=dave + 50% = test ✓\nSecond line"
	body := "csrf=" + form.Get("csrf") + "&nonce=" + form.Get("nonce") + "&binding=" + form.Get("binding") + "&category=spam&target=bob&details=" + details
	body = strings.ReplaceAll(body, " ", "%20")
	r, _ := s.Handle(t.Context(), h, protocol.New(922, 55).SetByte(1, 2).SetText(2, reportURL).SetText(3, body))
	if status, _ := r[0].Int(1); status != 200 {
		t.Fatalf("native submit: %+v status=%d", r, status)
	}
	all, _ := s.ListReports(t.Context(), "", 100)
	if len(all) != 1 || all[0].Details != details || all[0].Target != "bob" || all[0].Reporter != "hailam" {
		t.Fatalf("native details corrupted: %+v", all)
	}
	if _, err := parseNativeReportForm("details=x&csrf=forged&details=y"); err == nil {
		t.Fatal("accepted unbounded leading details")
	}
	if _, err := parseNativeReportForm("csrf=x&nonce=y"); err == nil {
		t.Fatal("accepted absent details")
	}
}

func TestPrivateGroupReportAndOperatorReview(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	login(t, s, "bob")
	if _, err := s.store.CreateGroup(t.Context(), "report-group", "hailam", []string{"bob"}); err != nil {
		t.Fatal(err)
	}
	address := "http://127.0.0.1:8080/sites/index.php?c=report&a=report_abuse&v=midlet&type=4&subject=Group%20Chat&subject_id=report-group"
	form := reportFields(t, s, h, address)
	w := fetchReport(t, s, h, "POST", address, form.Encode())
	if w.Code != 200 {
		t.Fatalf("group report: %d", w.Code)
	}
	all, _ := s.ListReports(t.Context(), "pending", 100)
	if len(all) != 1 || all[0].Kind != "group" {
		t.Fatalf("group report: %+v", all)
	}
	if err := s.ReviewReport(t.Context(), all[0].ID, "reviewed", "Checked report "+strconv.FormatInt(all[0].ID, 10)); err != nil {
		t.Fatal(err)
	}
	report, reviews, err := s.ReportDetails(t.Context(), all[0].ID)
	if err != nil || report.Status != "reviewed" || len(reviews) != 1 {
		t.Fatalf("operator review: %+v %+v %v", report, reviews, err)
	}
	if _, err := s.store.Leave(t.Context(), "report-group", "hailam"); err != nil {
		t.Fatal(err)
	}
	if w := fetchReport(t, s, h, "POST", address, form.Encode()); w.Code != 403 {
		t.Fatalf("left group report: %d", w.Code)
	}
}
