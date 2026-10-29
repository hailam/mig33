package pages

import (
	"encoding/hex"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// RoomView is display data supplied by the authenticated server adapter.
// InfoURL, when present, is an absolute URL on that adapter's own origin.
type RoomView struct {
	Name, Description, Owner, InfoURL, SettingsURL string
	Capacity, Occupancy                            int
	Moderators                                     []string
	Management                                     *RoomManagement
	Notice                                         string
}

// Native commands do not URL-decode arguments. Imported names outside the
// original command grammar must not become executable links.
func nativeRoomJoinURL(name string) string {
	if name == "" || len(name) > 40 || strings.TrimSpace(name) != name {
		return ""
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != ' ' && c != '_' && c != '-' {
			return ""
		}
	}
	return "mig33:joinChatroom(" + name + ")"
}

func roomOccupancy(view RoomView) string {
	return "People: " + strconv.Itoa(view.Occupancy) + " / " + strconv.Itoa(view.Capacity)
}

// RoomInfo is read-only; displaying a room does not join it or grant a role.
func RoomInfo(view RoomView) Result {
	if view.Description == "" {
		view.Description = "No description."
	}
	moderators := "None"
	if len(view.Moderators) != 0 {
		moderators = strings.Join(view.Moderators, ", ")
	}
	return Render(http.StatusOK, "Room Info", legacyRoomInfoView(view, moderators))
}

type roomSearchView struct {
	Action, Query                                      string
	Language, Keywords, LanguageAction, KeywordsAction string
	Results                                            []RoomView
	Searched                                           bool
}

// RoomSearch posts only its final free-text query. Results use native join
// commands and original PHP-shaped info URLs supplied by the adapter.
func RoomSearch(action, query string, results []RoomView, searched bool) Result {
	return RoomSearchFiltered(action, query, "", "", results, searched)
}

func RoomSearchFiltered(action, query, language, keywords string, results []RoomView, searched bool) Result {
	makeAction := func(field string) string {
		u, _ := url.Parse(action)
		q := u.Query()
		q.Set("query", query)
		q.Set("language", language)
		q.Set("keywords", keywords)
		q.Set("field", field)
		u.RawQuery = q.Encode()
		return u.String()
	}
	return Render(http.StatusOK, "Search Rooms", legacyRoomSearchView(roomSearchView{
		Action:         makeAction("query"),
		Query:          query,
		Language:       language,
		Keywords:       keywords,
		LanguageAction: makeAction("language"),
		KeywordsAction: makeAction("keywords"),
		Results:        results,
		Searched:       searched,
	}))
}

type roomCreateView struct {
	Action, Name string
	Hidden       []HiddenField
	NameLimit    int
}

// RoomCreate collects the name alone. The original serializer cannot safely
// delimit multiple unrestricted free-text values in one submitted form.
func RoomCreate(action, csrf string, nameLimit int) Result {
	return Render(http.StatusOK, "Create Room", legacyRoomCreateView(roomCreateView{
		Action: action, Hidden: []HiddenField{{Name: "csrf", Value: csrf}}, NameLimit: nameLimit,
	}))
}

// RoomCreateDetails carries the validated name as hex and binds it to the
// session. Description stays last to preserve the native raw-tail contract.
func RoomCreateDetails(action, csrf, name, binding string) Result {
	return Render(http.StatusOK, "Create Room", legacyRoomCreateDetailsView(roomCreateView{
		Action: action,
		Name:   name,
		Hidden: []HiddenField{
			{Name: "csrf", Value: csrf},
			{Name: "room", Value: hex.EncodeToString([]byte(name))},
			{Name: "binding", Value: binding},
		},
	}))
}

func RoomCreated(view RoomView) Result {
	return Render(http.StatusOK, "Room Created", legacyRoomCreatedView(view))
}

func RoomNotice(status int, title, text string) Result {
	return Render(status, title, Paragraph(text))
}
