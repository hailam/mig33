package legacyweb

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
	"github.com/hailam/mig33/internal/rooms"
)

func isRoomPageURL(u *url.URL) bool {
	if u == nil || u.Path != "/sites/index.php" {
		return false
	}

	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(q["c"]) != 1 || len(q["a"]) != 1 || len(q["v"]) != 1 || q.Get("c") != "chatroom" || q.Get("v") != "midlet" {
		return false
	}

	return q.Get("a") == "setup" || q.Get("a") == "search" || q.Get("a") == "create"
}

// Each native form has exactly one free-text value, placed last. The old
// serializer only escapes spaces; preserve all other text as the final tail.
func parseNativeRoomForm(u *url.URL, body string) (url.Values, error) {
	if len(body) > 8192 || !utf8.ValidString(body) {
		return nil, faults.ErrInvalid
	}
	if u.Query().Get("a") == "setup" {
		if u.Query().Get("manage") == "1" {
			return parseNativeServiceForm(body)
		}
		// Moderator forms contain only fixed, safe hidden values, no free text.
		return url.ParseQuery(body)
	}

	field := "query"
	if u.Query().Get("a") == "create" {
		field = "name"
		if u.Query().Get("stage") == "details" {
			field = "description"
		}
	}

	var before, value string
	var found bool
	if strings.HasPrefix(body, field+"=") {
		value, found = strings.TrimPrefix(body, field+"="), true
	} else {
		before, value, found = strings.Cut(body, "&"+field+"=")
	}
	if !found {
		// Native forms omit empty text controls rather than writing key=.
		// Description is optional; a blank search intentionally lists rooms.
		if field == "description" || (field == "query" && body == "") {
			before = body
		} else {
			return nil, faults.ErrInvalid
		}
	}

	form, err := url.ParseQuery(before)
	if err != nil {
		return nil, err
	}
	if _, duplicate := form[field]; duplicate {
		return nil, faults.ErrInvalid
	}

	form.Set(field, strings.ReplaceAll(value, "%20", " "))
	return form, nil
}

func roomFormFields(form url.Values, names ...string) bool {
	if len(form) != len(names) {
		return false
	}

	for _, name := range names {
		if len(form[name]) != 1 {
			return false
		}
	}

	return true
}

func (s *Adapter) roomPageAction(action string) string {
	return s.publicURL + "/sites/index.php?c=chatroom&a=" + action + "&v=midlet"
}

func (s *Adapter) roomView(ctx context.Context, room rooms.Room) pages.RoomView {
	return pages.RoomView{
		Name: room.Name, Description: room.Description, Owner: room.Owner,
		Capacity: room.Capacity, Occupancy: len(s.workflow.Participants(ctx, room.Name)),
		InfoURL: s.roomPageAction("setup") + "&roomName=" + url.QueryEscape(room.Name),
	}
}

func (s *Adapter) roomPage(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) pages.Result {
	q := u.Query()
	action := q.Get("a")
	if method != "GET" && method != "POST" {
		return pages.RoomNotice(405, "Chat Rooms", "Use GET to open this page or POST to submit its form.")
	}

	switch action {
	case "setup":
		if q.Get("manage") == "1" {
			return s.roomSettingsPage(ctx, x, method, u, form)
		}
		if len(q["roomName"]) != 1 {
			return pages.RoomNotice(400, "Room Info", "Choose a room to view.")
		}

		room, err := s.rooms.Room(ctx, q.Get("roomName"))
		if err != nil {
			return roomPageFailure(err)
		}

		return s.roomInfoPage(ctx, x, method, room, form)
	case "search":
		query := q.Get("query")
		language, keywords := q.Get("language"), q.Get("keywords")
		searched := len(q["query"]) == 1 || len(q["language"]) == 1 || len(q["keywords"]) == 1
		if len(q["query"]) > 1 || len(q["language"]) > 1 || len(q["keywords"]) > 1 || len(q["field"]) > 1 {
			return pages.RoomNotice(400, "Search Rooms", "Enter one search query.")
		}
		if method == "POST" {
			if !roomFormFields(form, "query") {
				return pages.RoomNotice(400, "Search Rooms", "Enter one search query.")
			}

			switch q.Get("field") {
			case "", "query":
				query = form.Get("query")
			case "language":
				language = form.Get("query")
			case "keywords":
				keywords = form.Get("query")
			default:
				return pages.RoomNotice(400, "Search Rooms", "Unknown search filter.")
			}

			searched = true
		}

		var views []pages.RoomView
		if searched {
			rooms, err := s.rooms.SearchRoomsFiltered(ctx, rooms.RoomSearchFilter{Query: query, Language: language, Keywords: keywords}, 100)
			if err != nil {
				return roomPageFailure(err)
			}

			for _, room := range rooms {
				views = append(views, s.roomView(ctx, room))
			}
		}

		return pages.RoomSearchFiltered(s.roomPageAction("search"), query, language, keywords, views, searched)
	case "create":
		if len(q["stage"]) > 1 || (q.Get("stage") != "" && q.Get("stage") != "name" && q.Get("stage") != "details") {
			return pages.RoomNotice(400, "Create Room", "Open Create Room to start again.")
		}
		if method == "GET" {
			return pages.RoomCreate(s.roomPageAction("create")+"&stage=name", x.csrf, rooms.MaxCreatedRoomNameLength)
		}
		if len(form["csrf"]) != 1 || x.csrf == "" || subtle.ConstantTimeCompare([]byte(x.csrf), []byte(form.Get("csrf"))) != 1 {
			return pages.RoomNotice(403, "Create Room", "This form has expired. Open Create Room again.")
		}
		if q.Get("stage") != "details" {
			if !roomFormFields(form, "csrf", "name") {
				return pages.RoomNotice(400, "Create Room", "Enter a room name.")
			}

			name, err := rooms.NormalizeRoomName(form.Get("name"))
			if err != nil || len(name) > rooms.MaxCreatedRoomNameLength {
				return pages.RoomNotice(400, "Create Room", "Use 1 to "+strconv.Itoa(rooms.MaxCreatedRoomNameLength)+" letters, digits, spaces, underscores or hyphens for the room name.")
			}

			binding := reportBinding(x.csrf, "room-create", "room", name)
			return pages.RoomCreateDetails(s.roomPageAction("create")+"&stage=details", x.csrf, name, binding)
		}
		if !roomFormFields(form, "csrf", "room", "binding", "description") {
			return pages.RoomNotice(400, "Create Room", "Invalid room form. Open Create Room again.")
		}

		decoded, err := hex.DecodeString(form.Get("room"))
		if err != nil {
			return pages.RoomNotice(400, "Create Room", "Invalid room name.")
		}

		name, err := rooms.NormalizeRoomName(string(decoded))
		if err != nil || name != string(decoded) || len(name) > rooms.MaxCreatedRoomNameLength {
			return pages.RoomNotice(400, "Create Room", "Invalid room name.")
		}

		binding := reportBinding(x.csrf, "room-create", "room", name)
		if subtle.ConstantTimeCompare([]byte(binding), []byte(form.Get("binding"))) != 1 {
			return pages.RoomNotice(403, "Create Room", "The room name has changed. Open Create Room again.")
		}

		room, err := s.rooms.CreateRoom(ctx, x.username, name, form.Get("description"))
		if err != nil {
			return roomPageFailure(err)
		}

		return pages.RoomCreated(s.roomView(ctx, room))
	}

	return pages.RoomNotice(404, "Chat Rooms", "Page not found.")
}

func roomPageFailure(err error) pages.Result {
	switch {
	case errors.Is(err, faults.ErrNotFound):
		return pages.RoomNotice(404, "Chat Rooms", "This room was not found.")
	case errors.Is(err, faults.ErrInvalid):
		return pages.RoomNotice(400, "Chat Rooms", "Check the name, search text or description length and characters.")
	case errors.Is(err, faults.ErrConflict):
		return pages.RoomNotice(409, "Create Room", "That room name is already in use. Choose another name.")
	case errors.Is(err, faults.ErrLimit):
		return pages.RoomNotice(429, "Create Room", "You have reached the limit of 10 created rooms.")
	default:
		return pages.RoomNotice(503, "Chat Rooms", "Rooms could not be loaded or saved. Please try again.")
	}
}
