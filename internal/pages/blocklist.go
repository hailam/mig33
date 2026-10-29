package pages

import (
	"net/http"
	"strings"
)

type blockListView struct {
	Action, CSRF, Notice string
	Blocked              []string
}

// BlockList renders a replacement page at the recovered block-list URL. The
// caller supplies its CSRF token and validated absolute server origin. The
// fields are reconstruction choices, not recovered historical payloads.
func BlockList(blocked []string, csrfToken, notice, origin string) Result {
	if origin == "" {
		return Render(http.StatusServiceUnavailable, "Manage Block List",
			Paragraph("Your block list is temporarily unavailable."))
	}
	return Render(http.StatusOK, "Manage Block List", legacyBlockListView(blockListView{
		Action:  strings.TrimRight(origin, "/") + "/sites/index.php?c=settings&a=block_list&v=midlet",
		CSRF:    csrfToken,
		Notice:  notice,
		Blocked: blocked,
	}))
}

func unblockFields(view blockListView, username string) []HiddenField {
	return []HiddenField{{Name: "action", Value: "unblock"}, {Name: "username", Value: username}, {Name: "csrf", Value: view.CSRF}}
}
