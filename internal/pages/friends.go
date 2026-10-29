package pages

import "net/http"

// Native commands do not URL-decode arguments. Restrict arguments rather than
// letting punctuation become part of the applet's command grammar.
func nativeUsernameAllowed(username string) bool {
	if username == "" || len(username) > 100 {
		return false
	}
	for _, c := range username {
		if !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != '_' && c != '-' && c != '.' {
			return false
		}
	}
	return true
}

type friendSearchView struct {
	Action, ProfileURL, Query string
	Users                     []User
	Searched                  bool
}

func userDisplayName(user User) string {
	if user.DisplayName == "" {
		return user.Username
	}
	return user.DisplayName
}

// FriendSearch uses the recovered route and native actions. The search form
// and matching policy are reconstructed; historical server HTML is absent.
func FriendSearch(action, profileURL, query string, users []User, searched bool) Result {
	return Render(http.StatusOK, "Search New Friends", legacyFriendSearchView(friendSearchView{
		Action:     action,
		ProfileURL: profileURL,
		Query:      query,
		Users:      users,
		Searched:   searched,
	}))
}
