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

	"github.com/a-h/templ"

	"github.com/hailam/mig33/internal/community"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
	roommodel "github.com/hailam/mig33/internal/rooms"
)

func communityCommerceRoute(u *url.URL) bool {
	if u == nil || u.Path != "/sites/index.php" {
		return false
	}

	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return false
	}

	for _, key := range []string{"c", "a", "v"} {
		if len(q[key]) != 1 {
			return false
		}
	}
	if q.Get("v") != "midlet" {
		return false
	}

	switch q.Get("c") {
	case "group", "account", "store", "merchant":
		return true
	}

	return false
}

// Native HTML forms only encode spaces. Every free-text field in these pages
// is last; fixed fields use restricted alphabets or hex and are parsed first.
func parseCommunityCommerceForm(u *url.URL, body string) (url.Values, error) {
	if len(body) > 8192 || !utf8.ValidString(body) {
		return nil, faults.ErrInvalid
	}

	field := ""
	switch u.Query().Get("op") {
	case "create":
		field = "name"
	case "create-details":
		field = "description"
	case "post":
		field = "body"
	case "recipient", "member":
		field = "username"
	case "purchase":
		if u.Query().Get("c") == "store" {
			field = "note"
		}
	}
	if field == "" {
		return url.ParseQuery(body)
	}

	prefix, value, found := strings.Cut(body, "&"+field+"=")
	if strings.HasPrefix(body, field+"=") {
		prefix = ""
		value = strings.TrimPrefix(body, field+"=")
		found = true
	}
	if !found {
		if field != "description" && field != "note" {
			return nil, faults.ErrInvalid
		}

		prefix = body
		value = ""
	}

	f, err := url.ParseQuery(prefix)
	if err != nil {
		return nil, err
	}
	if _, ok := f[field]; ok {
		return nil, faults.ErrInvalid
	}

	f.Set(field, strings.ReplaceAll(value, "%20", " "))
	return f, nil
}

func ccPage(status int, title string, body templ.Component) pages.Result {
	return pages.Render(status, title, pages.Stack(pages.Heading(title), body))
}
func ccText(text string) templ.Component          { return pages.Paragraph(text) }
func ccLink(target, label string) templ.Component { return pages.Link(target, label) }
func (s *Adapter) ccURL(controller, action string) string {
	return s.publicURL + "/sites/index.php?c=" + controller + "&a=" + action + "&v=midlet"
}

func ccForm(action, csrf string, fields [][2]string, control templ.Component, label string) templ.Component {
	hidden := []pages.HiddenField{{Name: "csrf", Value: csrf}}
	for _, field := range fields {
		hidden = append(hidden, pages.HiddenField{Name: field[0], Value: field[1]})
	}

	return pages.Form(action, hidden, control, label)
}

func ccValidPost(x *actor, form url.Values) bool {
	return x != nil && x.csrf != "" && len(form["csrf"]) == 1 && subtle.ConstantTimeCompare([]byte(x.csrf), []byte(form.Get("csrf"))) == 1
}

func ccError(title string, err error) pages.Result {
	status, message := 503, "The service is temporarily unavailable. Please try again."
	switch {
	case errors.Is(err, faults.ErrInvalid):
		status, message = 400, "Check the form values and try again."
	case errors.Is(err, faults.ErrForbidden):
		status, message = 403, "You do not have permission for this action."
	case errors.Is(err, faults.ErrNotFound):
		status, message = 404, "The requested item was not found."
	case errors.Is(err, faults.ErrConflict):
		status, message = 409, "This request conflicts with saved information. Open the form again."
	case errors.Is(err, faults.ErrLimit):
		status, message = 409, "The account or group limit has been reached."
	}

	return ccPage(status, title, ccText(message))
}

func ccID(u *url.URL) (int64, error) {
	q := u.Query()
	key := ""
	for _, candidate := range []string{"cid", "group_id", "id"} {
		if len(q[candidate]) != 0 {
			if key != "" || len(q[candidate]) != 1 {
				return 0, faults.ErrInvalid
			}

			key = candidate
		}
	}
	if key == "" {
		return 0, faults.ErrInvalid
	}

	id, err := strconv.ParseInt(q.Get(key), 10, 64)
	if err != nil || id <= 0 {
		return 0, faults.ErrInvalid
	}

	return id, nil
}

func (s *Adapter) communityPage(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) (pages.Result, bool) {
	if !communityCommerceRoute(u) || u.Query().Get("c") != "group" {
		return pages.Result{}, false
	}

	q := u.Query()
	action := q.Get("a")
	switch action {
	case "home", "add_to_group", "list_chatrooms", "list_groups_home", "list_my_groups", "members", "view_forum":
	default:
		return pages.Result{}, false
	}
	if method != "GET" && method != "POST" {
		return ccPage(405, "Groups", ccText("Use GET or POST.")), true
	}
	if method == "POST" && !ccValidPost(x, form) {
		return ccPage(403, "Groups", ccText("Open this page again before submitting.")), true
	}

	base := s.ccURL("group", "home")
	if method == "POST" && q.Get("op") == "create" {
		name, err := roommodel.NormalizeRoomName(form.Get("name"))
		private := form.Get("private")
		if err != nil || (private != "0" && private != "1") || !roomFormFields(form, "csrf", "name", "private") {
			return ccError("Create Group", faults.ErrInvalid), true
		}

		binding := reportBinding(x.csrf, "community-create", private, name)
		body := pages.Stack(ccText("Group: "+name), ccForm(
			base+"&op=create-details",
			x.csrf,
			[][2]string{{"name_hex", hex.EncodeToString([]byte(name))}, {"private", private}, {"binding", binding}},
			pages.TextArea("description", "Description", "", 1000),
			"Create Group",
		))
		return ccPage(200, "Create Group", body), true
	}
	if method == "POST" && q.Get("op") == "create-details" {
		name, err := hex.DecodeString(form.Get("name_hex"))
		private := form.Get("private")
		expected := reportBinding(x.csrf, "community-create", private, string(name))
		if err != nil || (private != "0" && private != "1") ||
			!roomFormFields(form, "csrf", "name_hex", "private", "binding", "description") ||
			subtle.ConstantTimeCompare([]byte(expected), []byte(form.Get("binding"))) != 1 {
			return ccError("Create Group", faults.ErrInvalid), true
		}

		g, err := s.community.Create(ctx, x.username, string(name), form.Get("description"), private == "1")
		if err != nil {
			return ccError("Create Group", err), true
		}

		return ccPage(200, "Group Created", pages.Stack(ccText(g.Name), ccLink(base+"&cid="+strconv.FormatInt(g.ID, 10), "Open Group"))), true
	}
	if action == "list_groups_home" || action == "list_my_groups" || (action == "home" && len(q["cid"]) == 0 && len(q["group_id"]) == 0 && len(q["id"]) == 0) {
		if method != "GET" {
			return ccError("Groups", faults.ErrInvalid), true
		}

		groups, err := s.community.List(ctx, x.username, action == "list_my_groups")
		if err != nil {
			return ccError("Groups", err), true
		}

		body := pages.Stack(ccLink(s.ccURL("group", "list_groups_home"), "Discover Groups"), ccLink(s.ccURL("group", "list_my_groups"), "My Groups"))
		for _, g := range groups {
			privacy := "Public"
			if g.Private {
				privacy = "Private"
			}

			body = pages.Stack(body, pages.Stack(ccLink(base+"&cid="+strconv.FormatInt(g.ID, 10), g.Name), ccText(privacy+" group · "+strconv.Itoa(g.Members)+" members")))
		}
		if len(groups) == 0 {
			body = pages.Stack(body, ccText("No groups yet."))
		}

		for _, priv := range []string{"0", "1"} {
			label := "Create Public Group"
			if priv == "1" {
				label = "Create Private Group"
			}

			body = pages.Stack(body, ccForm(base+"&op=create", x.csrf, [][2]string{{"private", priv}}, pages.Input("name", "Group name", "text", "", 10), label))
		}

		return ccPage(200, "Groups", body), true
	}
	if action == "add_to_group" && len(q["cid"]) == 0 && len(q["group_id"]) == 0 && len(q["id"]) == 0 {
		if method != "GET" || len(q["roomName"]) != 1 {
			return ccError("Link Room", faults.ErrInvalid), true
		}

		room, groups, err := s.community.LinkableGroups(ctx, x.username, q.Get("roomName"))
		if err != nil {
			return ccError("Link Room", err), true
		}

		body := ccText("Link " + room.Name + " to one of your Groups.")
		for _, g := range groups {
			body = pages.Stack(body, ccForm(
				s.ccURL("group", "add_to_group")+"&cid="+strconv.FormatInt(g.ID, 10)+"&op=link",
				x.csrf,
				[][2]string{{"room_hex", hex.EncodeToString([]byte(room.Name))}},
				pages.Stack(),
				g.Name,
			))
		}

		return ccPage(200, "Link Room", body), true
	}

	id, err := ccID(u)
	if err != nil {
		return ccError("Group", err), true
	}

	view, err := s.community.View(ctx, x.username, id)
	if err != nil {
		return ccError("Group", err), true
	}

	g, role := view.Community, view.Role

	groupQuery := "&cid=" + strconv.FormatInt(id, 10)
	notice := ""
	if method == "POST" {
		op := q.Get("op")
		switch op {
		case "join", "leave", "approve", "remove", "promote", "demote", "member":
			target := x.username
			if op != "join" && op != "leave" {
				target = strings.ToLower(strings.TrimSpace(form.Get("username")))
				if s.account(ctx, target) == nil {
					return ccError("Group Members", faults.ErrNotFound), true
				}
			}

			keys := []string{"csrf"}
			if op != "join" && op != "leave" {
				keys = append(keys, "username")
			}
			if !roomFormFields(form, keys...) {
				return ccError("Group Members", faults.ErrInvalid), true
			}
			if op == "member" {
				op = "add"
			}

			err = s.community.Membership(ctx, community.MembershipCommand{ID: id, Actor: x.username, Target: target, Operation: op})
			notice = "Membership updated."
		case "link", "unlink":
			var room []byte
			room, err = hex.DecodeString(form.Get("room_hex"))
			if err == nil && roomFormFields(form, "csrf", "room_hex") {
				err = s.community.LinkRoom(ctx, x.username, id, string(room), op == "unlink")
			} else {
				err = faults.ErrInvalid
			}

			notice = "Room links updated."
		case "post":
			var parent int64
			parent, err = strconv.ParseInt(form.Get("parent"), 10, 64)
			if err == nil && parent >= 0 && roomFormFields(form, "csrf", "parent", "nonce", "body") {
				_, err = s.community.AddPost(ctx, community.PostCommand{ID: id, Parent: parent, Actor: x.username, Body: form.Get("body"), Nonce: form.Get("nonce")})
			} else {
				err = faults.ErrInvalid
			}

			notice = "Your post was saved."
		default:
			err = faults.ErrInvalid
		}
		if err != nil {
			return ccError("Group", err), true
		}

		view, err = s.community.View(ctx, x.username, id)
		if err != nil {
			return ccError("Group", err), true
		}

		g, role = view.Community, view.Role

	}

	body := pages.Stack(pages.Stack(ccText(g.Description), ccText("Owner: "+g.Owner)), ccText("Members: "+strconv.Itoa(g.Members)))
	if notice != "" {
		body = pages.Stack(body, ccText(notice))
	}
	if role == "" {
		body = pages.Stack(body, ccForm(base+groupQuery+"&op=join", x.csrf, nil, pages.Stack(), "Join Group"))
	} else if role == "pending" {
		body = pages.Stack(body, pages.Stack(ccText("Your membership request is waiting for approval."), ccForm(base+groupQuery+"&op=leave", x.csrf, nil, pages.Stack(), "Cancel Request")))
	} else {
		body = pages.Stack(body, ccText("Your role: "+role))
		if role != "owner" {
			body = pages.Stack(body, ccForm(base+groupQuery+"&op=leave", x.csrf, nil, pages.Stack(), "Leave Group"))
		}
	}

	canRead := !g.Private || (role != "" && role != "pending")
	if canRead {
		for _, a := range [][2]string{{"home", "Group Home"}, {"members", "Members"}, {"list_chatrooms", "Chat Rooms"}, {"view_forum", "Forum"}} {
			body = pages.Stack(body, ccLink(s.ccURL("group", a[0])+groupQuery, a[1]))
		}
	}
	if !canRead && (action == "members" || action == "list_chatrooms" || action == "view_forum") {
		return ccError("Group", faults.ErrForbidden), true
	}

	switch action {
	case "members":
		members, err := s.community.Members(ctx, x.username, id)
		if err != nil {
			return ccError("Group Members", err), true
		}

		for _, m := range members {
			body = pages.Stack(body, ccText(m.Username+" ("+m.Role+")"))
			if role == "owner" || role == "moderator" {
				ops := []string{}
				if m.Role == "pending" {
					ops = append(ops, "approve", "remove")
				} else if m.Role == "member" {
					ops = append(ops, "remove")
					if role == "owner" {
						ops = append(ops, "promote")
					}
				} else if m.Role == "moderator" && role == "owner" {
					ops = append(ops, "demote", "remove")
				}

				for _, op := range ops {
					body = pages.Stack(body, ccForm(s.ccURL("group", "members")+groupQuery+"&op="+op, x.csrf, [][2]string{{"username", m.Username}}, pages.Stack(), op+" "+m.Username))
				}
			}
		}
		if role == "owner" || role == "moderator" {
			body = pages.Stack(body, ccForm(s.ccURL("group", "members")+groupQuery+"&op=member", x.csrf, nil, pages.Input("username", "Add a registered username", "text", "", 10), "Add Member"))
		}
	case "list_chatrooms", "add_to_group":
		rooms, err := s.community.Rooms(ctx, x.username, id)
		if err != nil {
			return ccError("Group Rooms", err), true
		}

		for _, room := range rooms {
			if safeName, err := roommodel.NormalizeRoomName(room.Name); err == nil && safeName == room.Name {
				body = pages.Stack(body, ccLink("mig33:joinChatroom("+room.Name+")", room.Name))
			} else {
				body = pages.Stack(body, ccText(room.Name))
			}
			if room.Owner == x.username && (role == "owner" || role == "moderator") {
				body = pages.Stack(body, ccForm(s.ccURL("group", "add_to_group")+groupQuery+"&op=unlink", x.csrf, [][2]string{{"room_hex", hex.EncodeToString([]byte(room.Name))}}, pages.Stack(), "Unlink Room"))
			}
		}
		if len(rooms) == 0 {
			body = pages.Stack(body, ccText("No linked chat rooms."))
		}
		if role == "owner" || role == "moderator" {
			rooms, err = s.community.LinkableRooms(ctx, x.username, id)
			if err != nil {
				return ccError("Group Rooms", err), true
			}

			for _, room := range rooms {
				if room.Owner == x.username {
					body = pages.Stack(body, ccForm(s.ccURL("group", "add_to_group")+groupQuery+"&op=link", x.csrf, [][2]string{{"room_hex", hex.EncodeToString([]byte(room.Name))}}, pages.Stack(), "Link "+room.Name))
				}
			}
		}
	case "view_forum":
		posts, err := s.community.Posts(ctx, x.username, id)
		if err != nil {
			return ccError("Group Forum", err), true
		}

		for _, p := range posts {
			label := "Topic #" + strconv.FormatInt(p.ID, 10)
			if p.ParentID != 0 {
				label = "Reply to #" + strconv.FormatInt(p.ParentID, 10)
			}

			body = pages.Stack(body, pages.Stack(ccText(label+" · "+p.Author), ccText(p.Body)))
			if p.ParentID == 0 && role != "" && role != "pending" {
				body = pages.Stack(body, ccLink(s.ccURL("group", "view_forum")+groupQuery+"&reply="+strconv.FormatInt(p.ID, 10), "Reply"))
			}
		}
		if len(posts) == 0 {
			body = pages.Stack(body, ccText("No forum posts yet."))
		}
		if role != "" && role != "pending" {
			parent := q.Get("reply")
			if parent == "" {
				parent = "0"
			}
			if n, e := strconv.ParseInt(parent, 10, 64); e != nil || n < 0 {
				return ccError("Group Forum", faults.ErrInvalid), true
			}

			body = pages.Stack(body, ccForm(
				s.ccURL("group", "view_forum")+groupQuery+"&op=post",
				x.csrf,
				[][2]string{{"parent", parent}, {"nonce", token()}},
				pages.TextArea("body", "Forum post", "", 2000),
				"Post",
			))
		}
	}

	return ccPage(200, g.Name, pages.Stack(body, ccLink(s.ccURL("group", "list_my_groups"), "My Groups"))), true
}
