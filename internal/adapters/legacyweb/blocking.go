package legacyweb

import (
	"context"
	"crypto/subtle"
	"net/url"
	"strings"

	"github.com/hailam/mig33/internal/pages"
)

func isBlockListURL(u *url.URL) bool {
	if u == nil || u.Path != "/sites/index.php" {
		return false
	}

	q, err := url.ParseQuery(u.RawQuery)
	return err == nil && len(q["c"]) == 1 && len(q["a"]) == 1 && len(q["v"]) == 1 && q.Get("c") == "settings" && q.Get("a") == "block_list" && q.Get("v") == "midlet"
}

func (s *Adapter) blockListPage(ctx context.Context, x *actor, method string, form url.Values) pages.Result {
	status, notice := 200, ""
	switch method {
	case "GET":
	case "POST":
		if len(form["csrf"]) != 1 || x.csrf == "" || subtle.ConstantTimeCompare([]byte(form.Get("csrf")), []byte(x.csrf)) != 1 {
			status, notice = 403, "This form has expired. Open Manage Block List again."
		} else if len(form["action"]) != 1 || form.Get("action") != "unblock" || len(form["username"]) != 1 {
			status, notice = 400, "Invalid block list request."
		} else {
			target := strings.ToLower(form.Get("username"))
			if s.account(ctx, target) == nil || target == x.username {
				status, notice = 400, "Choose a valid blocked user."
			} else if err := s.workflow.Unblock(ctx, x.id, x.csrf, target); err != nil {
				s.logError(ctx, "remove personal block: %v", err)
				status, notice = 503, "Could not update your block list. Please try again."
			} else {
				notice = target + " has been unblocked."
			}
		}
	default:
		status, notice = 405, "Use GET to view your block list or POST to unblock a user."
	}

	blocked, err := s.contacts.BlockedUsers(ctx, x.username)
	if err != nil {
		status, notice = 503, "Your block list is temporarily unavailable."
	}

	result := pages.BlockList(blocked, x.csrf, notice, s.publicURL)
	result.Status = status
	return result
}
