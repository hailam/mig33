package legacyweb

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/hailam/mig33/internal/formatting"
	"github.com/hailam/mig33/internal/pages"
)

const wapChatPath = "/member2/t.php"

// The native serializer escapes only spaces, so arbitrary markup must be the
// last field. Browser forms use the standard net/http decoder instead.
func parseNativeWAPForm(body string) (url.Values, error) {
	return parseNativeServiceForm(body)
}

func (s *Adapter) wapURL(kind, target string) string {
	return s.publicURL + wapChatPath + "?" + (url.Values{"kind": {kind}, "target": {target}}).Encode()
}

func (s *Adapter) wapPage(ctx context.Context, x *actor, method string, u *url.URL, f url.Values) pages.Result {
	if x == nil || !x.authenticated {
		return pages.RoomNotice(401, "Chat", "Sign in to chat")
	}
	if method != "GET" && method != "POST" {
		return pages.RoomNotice(405, "Chat", "Use GET or POST")
	}

	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return pages.RoomNotice(400, "Chat", "Invalid conversation")
	}

	for key, values := range q {
		if len(values) != 1 || (key != "kind" && key != "target" && key != "after_id") {
			return pages.RoomNotice(400, "Chat", "Invalid conversation")
		}
	}
	if method == "POST" && !identityCSRF(x, f) {
		return pages.RoomNotice(403, "Chat", "Reopen the form")
	}
	if q.Get("target") == "" {
		if method != "GET" || len(q) != 0 {
			return pages.RoomNotice(400, "Chat", "Choose a conversation")
		}

		return s.wapDirectory(ctx, x)
	}

	kind := transcriptKind(q.Get("kind"))
	target := q.Get("target")
	if kind == 0 {
		return pages.RoomNotice(400, "Chat", "Choose private, group or room")
	}
	if kind == 1 {
		target = strings.ToLower(target)
		if s.account(ctx, target) == nil {
			return pages.RoomNotice(404, "Chat", "User not found")
		}
	}
	if kind == 2 {
		if _, err := s.chat.Group(ctx, target, x.username); err != nil {
			return pages.RoomNotice(403, "Chat", "Join this group conversation first")
		}
	}
	if kind == 3 {
		room, err := s.rooms.Room(ctx, target)
		if err != nil {
			return pages.RoomNotice(404, "Chat", "Room not found")
		}

		target = room.Name
	}

	after := int64(0)
	if q.Get("after_id") != "" {
		after, err = strconv.ParseInt(q.Get("after_id"), 10, 64)
		if err != nil || after < 0 {
			return pages.RoomNotice(400, "Chat", "Invalid history cursor")
		}
	}

	notice := ""
	if method == "POST" {
		switch f.Get("op") {
		case "join", "leave":
			if kind != 3 || !identityFields(f, []string{"csrf", "op"}) {
				return pages.RoomNotice(400, "Chat", "Invalid room action")
			}

			var err error
			if f.Get("op") == "leave" {
				err = s.workflow.LeaveRoom(ctx, x.id, x.csrf, target)
				delete(x.rooms, target)
			} else {
				err = s.workflow.JoinRoom(ctx, x.id, x.csrf, target)
				if err == nil {
					x.rooms[target] = true
				}
			}
			if err != nil {
				return pages.RoomNotice(400, "Chat", err.Error())
			}

		case "send":
			if !identityFields(f, []string{"csrf", "op", "nonce", "format"}, "body") || !validToken(f.Get("nonce")) {
				return pages.RoomNotice(400, "Chat", "Reopen the message form")
			}
			if err := s.wapSend(ctx, x, kind, target, f.Get("body"), f.Get("format"), f.Get("nonce")); err != nil {
				return pages.RoomNotice(400, "Chat", err.Error())
			}

			notice = "Message sent."
		default:
			return pages.RoomNotice(400, "Chat", "Unknown action")
		}
	}

	action := s.wapURL(q.Get("kind"), target)
	if after > 0 {
		action += "&after_id=" + strconv.FormatInt(after, 10)
	}

	data := pages.WAPChatData{
		Action:         action,
		CSRF:           x.csrf,
		Nonce:          token(),
		User:           x.username,
		Kind:           q.Get("kind"),
		Target:         target,
		Notice:         notice,
		IsRoom:         kind == 3,
		Joined:         x.rooms[target],
		BrowserSession: x.webOnly,
	}
	if kind != 3 || data.Joined {
		messages, err := s.chat.History(ctx, x.username, kind, target, after, 51)
		if err != nil {
			return serviceError("Chat", err)
		}
		if len(messages) > 50 {
			messages = messages[:50]
			data.NextURL = s.wapURL(q.Get("kind"), target) + "&after_id=" + strconv.FormatInt(messages[49].ID, 10)
		}

		for _, m := range messages {
			view := pages.WAPMessage{Sender: m.Sender, Body: m.Body, Action: m.Action, Time: m.CreatedAt.UTC().Format("15:04 UTC")}
			if m.Markup != "" {
				if doc, err := formatting.ParseChat(m.Markup); err == nil {
					view.Formatted = true
					view.Document = doc
				}
			}
			if m.Color != nil && *m.Color >= 0 && *m.Color <= 0xffffff {
				view.Color = fmt.Sprintf("#%06x", *m.Color)
			}

			data.Messages = append(data.Messages, view)
		}
	}

	return pages.Render(200, "Chat", pages.WAPChat(data))
}

func (s *Adapter) wapSend(ctx context.Context, x *actor, kind byte, target, source, format, key string) error {
	return s.workflow.SendFormatted(ctx, x.id, x.csrf, kind, target, source, format, key)
}

func (s *Adapter) wapDirectory(ctx context.Context, x *actor) pages.Result {
	body := pages.Stack(pages.Heading("Conversations"), pages.Paragraph("Signed in as "+x.username))
	for _, name := range s.accountNames(ctx) {
		if name != x.username && s.canViewProfile(ctx, x.username, name) && !s.blocksIncoming(ctx, x.username, name) {
			body = pages.Stack(body, pages.Link(s.wapURL("private", name), "Chat with "+name))
		}
	}

	groups, err := s.chat.Groups(ctx, x.username)
	if err != nil {
		return serviceError("Chat", err)
	}

	for _, g := range groups {
		body = pages.Stack(body, pages.Link(s.wapURL("group", g.ID), "Group: "+strings.Join(g.Members, ", ")))
	}

	rooms, err := s.rooms.Rooms(ctx, 0)
	if err != nil {
		return serviceError("Chat", err)
	}

	for _, r := range rooms {
		body = pages.Stack(body, pages.Link(s.wapURL("room", r.Name), "Room: "+r.Name))
	}

	body = pages.Stack(body, pages.Link(s.identityURL("profile", "home"), "My Profile"))
	if x.webOnly {
		body = pages.Stack(body, pages.Form(s.publicURL+wapChatPath, []pages.HiddenField{{Name: "csrf", Value: x.csrf}, {Name: "op", Value: "logout"}}, pages.Stack(), "Sign out"))
	}

	return pages.Render(200, "Chat", body)
}
