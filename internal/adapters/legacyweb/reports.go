package legacyweb

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
	"github.com/hailam/mig33/internal/review"
)

func isReportURL(u *url.URL) bool {
	if u == nil || u.Path != "/sites/index.php" {
		return false
	}

	q, err := url.ParseQuery(u.RawQuery)
	return err == nil && len(q["c"]) == 1 && len(q["a"]) == 1 && len(q["v"]) == 1 && q.Get("c") == "report" && q.Get("a") == "report_abuse" && q.Get("v") == "midlet"
}

func parseNativeReportForm(body string) (url.Values, error) {
	if len(body) > 8192 || !utf8.ValidString(body) {
		return nil, faults.ErrInvalid
	}

	before, details, found := strings.Cut(body, "&details=")
	if !found {
		return nil, faults.ErrInvalid
	}

	form, err := url.ParseQuery(before)
	if err != nil {
		return nil, err
	}
	if _, exists := form["details"]; exists {
		return nil, faults.ErrInvalid
	}

	form.Set("details", strings.ReplaceAll(details, "%20", " "))
	return form, nil
}

func reportBinding(secret, nonce, kind, context string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(nonce + "\x00" + kind + "\x00" + context))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Adapter) reportContext(ctx context.Context, x *actor, u *url.URL) (kind, conversation string, targets []string, status int) {
	q, parseErr := url.ParseQuery(u.RawQuery)
	if parseErr != nil {
		return "", "", nil, 400
	}
	if len(q["type"]) != 1 || q.Get("type") != "4" || len(q["subject"]) != 1 || len(q["subject_id"]) != 1 {
		return "", "", nil, 400
	}

	conversation = q.Get("subject_id")
	if conversation == "" || len(conversation) > 256 || !utf8.ValidString(conversation) || strings.ContainsRune(conversation, '\x00') {
		return "", "", nil, 400
	}

	switch q.Get("subject") {
	case "Chatroom":
		kind = "room"
	case "Group Chat":
		kind = "group"
	default:
		return "", "", nil, 400
	}

	canonical, targets, err := s.workflow.ReportTargets(ctx, x.id, kind, conversation)
	if err != nil {
		if errors.Is(err, faults.ErrNotFound) {
			return kind, conversation, nil, 404
		}
		if errors.Is(err, faults.ErrForbidden) || errors.Is(err, faults.ErrNotMember) {
			return kind, conversation, nil, 403
		}

		return kind, conversation, nil, 503
	}

	return kind, canonical, targets, 200
}

func (s *Adapter) reportPage(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) pages.Result {
	if method != "GET" && method != "POST" {
		return pages.ReportNotice(405, "Use GET to open a report form or POST to submit it.")
	}

	kind, context, targets, status := s.reportContext(ctx, x, u)
	if status != 200 {
		text := "Invalid report conversation."
		if status == 403 {
			text = "Join this conversation before opening or submitting a report."
		}
		if status == 404 {
			text = "This chat room was not found."
		}

		return pages.ReportNotice(status, text)
	}
	if method == "GET" {
		nonce := token()
		subject := "Chatroom"
		if kind == "group" {
			subject = "Group Chat"
		}
		// Values are URL-escaped; fixed query separators deliberately remain
		// literal because the native parser does not decode attribute entities.
		action := s.publicURL + "/sites/index.php?c=report&a=report_abuse&v=midlet&type=4&subject=" + url.QueryEscape(subject) + "&subject_id=" + url.QueryEscape(context)
		return pages.ReportForm(action, context, x.csrf, nonce, reportBinding(x.csrf, nonce, kind, context), targets)
	}

	for key, values := range form {
		if len(values) != 1 {
			return pages.ReportNotice(400, "Invalid report form fields.")
		}

		switch key {
		case "csrf", "nonce", "binding", "category", "target", "details":
		default:
			return pages.ReportNotice(400, "Invalid report form fields.")
		}
	}
	if len(form["csrf"]) != 1 || x.csrf == "" || subtle.ConstantTimeCompare([]byte(form.Get("csrf")), []byte(x.csrf)) != 1 {
		return pages.ReportNotice(403, "This report form has expired. Open Report Abuse again.")
	}

	nonce := form.Get("nonce")
	if !validToken(nonce) || len(form["binding"]) != 1 || subtle.ConstantTimeCompare([]byte(form.Get("binding")), []byte(reportBinding(x.csrf, nonce, kind, context))) != 1 {
		return pages.ReportNotice(403, "This report form does not match the conversation. Open Report Abuse again.")
	}

	target := form.Get("target")
	if target != "" && !roomHasUser(targets, target) {
		return pages.ReportNotice(400, "Choose a user who is currently in this conversation.")
	}
	if len(form["category"]) != 1 || !review.ValidReportCategory(form.Get("category")) || len(form["details"]) != 1 {
		return pages.ReportNotice(400, "Choose a reason and describe what happened.")
	}

	report, err := s.workflow.SubmitReport(ctx, x.id, x.csrf, review.ReportSubmission{
		Reporter: x.username,
		Kind:     kind,
		Context:  context,
		Target:   target,
		Category: form.Get("category"),
		Details:  form.Get("details"),
		Nonce:    nonce,
	})
	if err != nil {
		if errors.Is(err, faults.ErrInvalid) {
			return pages.ReportNotice(400, "Enter a description within the report length limit.")
		}
		if errors.Is(err, faults.ErrConflict) {
			return pages.ReportNotice(409, "This form was already submitted. Open a new report form to submit different details.")
		}
		if errors.Is(err, faults.ErrNotMember) {
			return pages.ReportNotice(403, "You are no longer a member of this conversation.")
		}

		return pages.ReportNotice(503, "Your report could not be saved. Please try again.")
	}

	return pages.ReportSaved(report.ID)
}
