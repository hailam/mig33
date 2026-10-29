package legacyweb

import (
	"context"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/pages"
)

func isFriendSearchURL(u *url.URL) bool {
	if u == nil || u.Path != "/sites/index.php" {
		return false
	}

	q, err := url.ParseQuery(u.RawQuery)
	return err == nil && len(q["c"]) == 1 && len(q["a"]) == 1 && len(q["v"]) == 1 &&
		q.Get("c") == "profile" && q.Get("a") == "search" && q.Get("v") == "midlet"
}

func (s *Adapter) friendSearchPage(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) pages.Result {
	if method != "GET" && method != "POST" {
		return pages.RoomNotice(405, "Search New Friends", "Use GET to open this page or POST to search.")
	}

	q := u.Query()
	query, searched := q.Get("query"), len(q["query"]) == 1
	if len(q["query"]) > 1 {
		return pages.RoomNotice(400, "Search New Friends", "Enter one search query.")
	}
	if method == "POST" {
		if !roomFormFields(form, "query") {
			return pages.RoomNotice(400, "Search New Friends", "Enter one search query.")
		}

		query, searched = form.Get("query"), true
	}
	if len(query) > 128 || !utf8.ValidString(query) || strings.ContainsRune(query, 0) {
		return pages.RoomNotice(400, "Search New Friends", "Use a search of at most 128 bytes without null characters.")
	}

	query = strings.TrimSpace(query)
	var matches []pages.User
	if searched {
		term := strings.ToLower(query)
		for _, user := range s.pageUsers(ctx) {
			if user.Username != x.username && s.canViewProfile(ctx, x.username, user.Username) &&
				(strings.Contains(strings.ToLower(user.Username), term) || strings.Contains(strings.ToLower(user.DisplayName), term)) {
				matches = append(matches, user)
			}
		}
	}

	base := s.publicURL + "/sites/index.php?c=profile&v=midlet&a="
	return pages.FriendSearch(base+"search", base+"home&username=", query, matches, searched)
}
