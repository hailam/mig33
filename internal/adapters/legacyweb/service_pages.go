package legacyweb

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/a-h/templ"

	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
)

var serviceRoutes = map[string]string{
	"avatar/home": "Display Picture", "bot/bot_list": "Games", "buzz/home": "Buzz",
	"call/home": "Call", "email/compose": "Mail", "help/contact_us": "Contact Support",
	"im/setup_im": "External IM", "leaderboard/home": "Leaderboard", "location/checkin": "Check In",
	"lookout/home": "Lookout", "migworld/get_mobpage": "migWorld", "photo/home": "Scrapbook",
	"photos/wall": "Photo Wall", "profile/user_blog_posts": "Blog", "sms/home": "SMS",
}

func featureKey(u *url.URL) string {
	c, a, ok := identitySelectors(u)
	if !ok {
		return ""
	}

	return c + "/" + a
}

func parseNativeServiceForm(body string) (url.Values, error) {
	if len(body) > 8192 || !utf8.ValidString(body) {
		return nil, faults.ErrInvalid
	}

	before, value, found := strings.Cut(body, "&body=")
	if strings.HasPrefix(body, "body=") {
		before, value, found = "", strings.TrimPrefix(body, "body="), true
	}
	if !found {
		before = body
	}

	f, err := url.ParseQuery(before)
	if err != nil {
		return nil, err
	}
	if _, exists := f["body"]; exists {
		return nil, faults.ErrInvalid
	}
	if found {
		f.Set("body", strings.ReplaceAll(value, "%20", " "))
	}

	return f, nil
}

func serviceResult(status int, title string, body templ.Component) pages.Result {
	return pages.Render(status, title, pages.Stack(pages.Heading(title), body))
}
func serviceText(text string) templ.Component { return pages.Paragraph(text) }
func (s *Adapter) featureURL(controller, action string) string {
	return s.publicURL + "/sites/index.php?c=" + controller + "&a=" + action + "&v=midlet"
}
func serviceLink(href, label string) templ.Component { return pages.Link(href, label) }

// Every free-text form has a single final field. Hidden values use a safe alphabet.
func serviceForm(action, csrf, op, target, label, submit string) templ.Component {
	var controls templ.Component
	if label != "" {
		controls = pages.Input("body", label, "text", "", 10)
	}

	return pages.Form(action, []pages.HiddenField{
		{Name: "csrf", Value: csrf},
		{Name: "nonce", Value: token()},
		{Name: "op", Value: op},
		{Name: "target", Value: hex.EncodeToString([]byte(target))},
	}, controls, submit)
}

func serviceMutation(x *actor, f url.Values) (op, target, body string, err error) {
	if x == nil || x.csrf == "" || len(f["csrf"]) != 1 || subtle.ConstantTimeCompare([]byte(f.Get("csrf")), []byte(x.csrf)) != 1 {
		return "", "", "", faults.ErrForbidden
	}

	for key, values := range f {
		if len(values) != 1 || (key != "csrf" && key != "nonce" && key != "op" && key != "target" && key != "body") {
			return "", "", "", faults.ErrInvalid
		}
	}
	if !validToken(f.Get("nonce")) || len(f["op"]) != 1 {
		return "", "", "", faults.ErrInvalid
	}

	b, err := hex.DecodeString(f.Get("target"))
	if err != nil || !utf8.Valid(b) {
		return "", "", "", faults.ErrInvalid
	}

	body = f.Get("body")
	if len(body) > 4000 || !utf8.ValidString(body) || strings.ContainsRune(body, 0) {
		return "", "", "", faults.ErrInvalid
	}

	return f.Get("op"), string(b), body, nil
}

func serviceError(title string, err error) pages.Result {
	var failure *faults.ServiceError
	if errors.As(err, &failure) {
		status := 503
		switch failure.Kind {
		case faults.Invalid:
			status = 400
		case faults.Unauthenticated:
			status = 401
		case faults.Forbidden:
			status = 403
		case faults.NotFound:
			status = 404
		case faults.Conflict:
			status = 409
		case faults.Limited:
			status = 429
		case faults.Internal:
			status = 500
		}

		return serviceResult(status, title, serviceText(failure.Message))
	}

	status, message := 503, "This service is temporarily unavailable. Try again."
	switch {
	case errors.Is(err, faults.ErrForbidden):
		status, message = 403, "This action is not allowed. Reopen the page and check your account."
	case errors.Is(err, faults.ErrNotFound):
		status, message = 404, "The requested item was not found."
	case errors.Is(err, faults.ErrInvalid):
		status, message = 400, "Check the submitted fields."
	case errors.Is(err, faults.ErrConflict):
		status, message = 409, "This request was already used with different details. Reopen the page."
	case errors.Is(err, faults.ErrLimit):
		status, message = 429, "The storage limit has been reached."
	}

	return serviceResult(status, title, serviceText(message))
}

// The local Activity Updates preference controls disclosure in activity feeds,
// not access to explicitly authored blog pages. Owners can still see their
// own records when their public activity is hidden.
func (s *Adapter) canViewActivity(ctx context.Context, viewer, owner string) bool {
	allowed, err := s.privacy.CanViewActivity(ctx, viewer, owner)
	return err == nil && allowed
}

func (s *Adapter) servicePage(ctx context.Context, x *actor, method string, u *url.URL, f url.Values) (pages.Result, bool) {
	key := featureKey(u)
	title, ok := serviceRoutes[key]
	if !ok {
		return pages.Result{}, false
	}
	if method != "GET" && method != "POST" {
		return serviceResult(405, title, serviceText("Use GET or POST.")), true
	}

	op, target, body := "", "", ""
	var err error
	if method == "POST" {
		op, target, body, err = serviceMutation(x, f)
		if err != nil {
			return serviceError(title, err), true
		}
	}

	action := s.publicURL + u.EscapedPath() + "?" + u.Query().Encode()
	notice := pages.Stack()
	q := u.Query()
	if key == "migworld/get_mobpage" {
		if method != "GET" {
			return serviceResult(405, title, serviceText("Open a service below.")), true
		}

		links := []struct{ c, a, label string }{
			{"profile", "home", "My Profile"},
			{"profile", "search", "Find Friends"},
			{"buzz", "home", "Buzz"},
			{"lookout", "home", "Lookout"},
			{"group", "list_groups_home", "Groups"},
			{"chatroom", "search", "Chat Rooms"},
			{"store", "main", "Store"},
			{"account", "home", "My Account"},
			{"bot", "bot_list", "Games"},
			{"photo", "home", "Scrapbook"},
			{"photos", "wall", "Photo Wall"},
			{"location", "checkin", "Check In"},
			{"leaderboard", "home", "Leaderboard"},
			{"email", "compose", "Mail"},
			{"sms", "home", "SMS"},
			{"call", "home", "Call"},
			{"merchant", "dashboard", "Merchant Dashboard"},
			{"help", "contact_us", "Contact Support"},
		}
		for _, l := range links {
			notice = pages.Stack(notice, serviceLink(s.featureURL(l.c, l.a), l.label))
		}

		notice = pages.Stack(notice, serviceLink(s.publicURL+wapChatPath, "Formatted Chat"))
		return serviceResult(200, title, notice), true
	}
	if key == "photo/home" || key == "photos/wall" || key == "avatar/home" {
		return s.photoPage(ctx, x, method, u, f, op, target), true
	}
	if key == "leaderboard/home" {
		if method != "GET" {
			return serviceResult(405, title, pages.Stack()), true
		}

		rankings, e := s.activity.Leaderboard(ctx, x.username)
		if e != nil {
			return serviceError(title, e), true
		}

		notice = serviceText("Local games played, using the latest 100 results. Historical ranking rules have not been recovered.")
		for i, rank := range rankings {
			notice = pages.Stack(notice, serviceText(fmt.Sprintf("%d. %s: %d games", i+1, rank.Username, rank.Games)))
		}

		return serviceResult(200, title, notice), true
	}
	if key == "bot/bot_list" {
		if len(q["game"]) > 1 {
			return serviceError(title, faults.ErrInvalid), true
		}
		if q.Get("game") != "" && method != "GET" {
			return serviceResult(405, title, serviceText("Game details are read-only. Open All games to play an available game.")), true
		}

		catalog, e := s.gameCatalogContent(x, q.Get("game"))
		if e != nil {
			return serviceError(title, e), true
		}
		if method == "POST" {
			if op != "play" || (target != "roll" && target != "8ball") {
				return serviceError(title, faults.ErrInvalid), true
			}

			entry, e := s.activity.PlayGame(ctx, x.username, target, f.Get("nonce"))
			if e != nil {
				return serviceError(title, e), true
			}

			notice = pages.Stack(notice, serviceText(entry.Body))

		}

		notice = pages.Stack(notice, catalog)
		entries, e := s.activity.Entries(ctx, x.username, "game", x.username)
		if e != nil {
			return serviceError(title, e), true
		}

		for _, entry := range entries {
			notice = pages.Stack(notice, serviceText(entry.Body))
		}

		return serviceResult(200, title, notice), true
	}
	if key == "email/compose" || key == "sms/home" || key == "call/home" || key == "help/contact_us" || key == "im/setup_im" {
		return s.providerPage(ctx, x, method, u, f, op, target, body), true
	}

	kind := "buzz"
	owner := ""
	switch key {
	case "profile/user_blog_posts":
		kind = "blog"
		owner = strings.ToLower(q.Get("username"))
		if owner == "" {
			owner = x.username
		}
	case "location/checkin":
		kind = "checkin"
		owner = x.username
	case "lookout/home":
		owner = strings.ToLower(q.Get("username"))
		if owner == "" {
			owner = x.username
		}
	}
	if key == "buzz/home" || key == "lookout/home" {
		owner, err = s.activityPageOwner(ctx, x, u, owner)
		if err != nil {
			return serviceError(title, err), true
		}
	}
	if owner != "" && (s.account(ctx, owner) == nil || !s.canViewProfile(ctx, x.username, owner)) {
		return serviceError(title, faults.ErrNotFound), true
	}
	if method == "POST" {
		if key == "lookout/home" || owner != "" && owner != x.username {
			return serviceError(title, faults.ErrForbidden), true
		}
		if op == "delete" {
			id, e := strconv.ParseInt(target, 10, 64)
			if e != nil {
				return serviceError(title, faults.ErrInvalid), true
			}

			err = s.activity.Delete(ctx, x.username, id)
		} else if op == "publish" && target == kind {
			var entry activity.ServiceEntry
			entry, err = s.workflow.PublishActivity(ctx, x.id, x.csrf, activity.ServiceEntry{Kind: kind, Owner: x.username, Body: body, State: "published", Nonce: f.Get("nonce")})
			_ = entry
		} else {
			err = faults.ErrInvalid
		}
		if err != nil {
			return serviceError(title, err), true
		}

		notice = pages.Stack(notice, serviceText("Saved."))
	}
	if key != "lookout/home" && (owner == "" || owner == x.username) {
		label := "Write an update"
		if kind == "checkin" {
			label = "Place name (shared with other users)"
		}

		notice = pages.Stack(notice, serviceForm(action, x.csrf, "publish", kind, label, "Publish"))
	}

	var entries []activity.ServiceEntry
	var e error
	if key == "buzz/home" || key == "lookout/home" {
		entries, e = s.activity.ActivityEntries(ctx, x.username, owner)
	} else {
		entries, e = s.activity.Entries(ctx, x.username, kind, owner)
	}
	if e != nil {
		return serviceError(title, e), true
	}

	visible := 0
	for _, entry := range entries {
		if !s.canViewProfile(ctx, x.username, entry.Owner) ||
			((kind == "buzz" || kind == "checkin") && !s.canViewActivity(ctx, x.username, entry.Owner)) {
			continue
		}

		visible++
		notice = pages.Stack(notice, serviceText(entry.Owner+": "+entry.Body))
		if entry.Owner == x.username && key != "lookout/home" {
			notice = pages.Stack(notice, serviceForm(action, x.csrf, "delete", strconv.FormatInt(entry.ID, 10), "", "Delete"))
		}
	}
	if visible == 0 {
		notice = pages.Stack(notice, serviceText("No posts yet."))
	}
	if key == "lookout/home" {
		notice = pages.Stack(notice, serviceLink(s.featureURL("profile", "home")+"&username="+url.QueryEscape(owner), "View Profile"))
	}

	return serviceResult(200, title, notice), true
}

func (s *Adapter) providerPage(ctx context.Context, x *actor, method string, u *url.URL, f url.Values, op, target, body string) pages.Result {
	key := featureKey(u)
	title := serviceRoutes[key]
	action := s.publicURL + u.EscapedPath() + "?" + u.Query().Encode()
	kind, label, initial := "mail", "Username or email address", u.Query().Get("username")
	if u.Query().Get("to") != "" {
		initial = u.Query().Get("to")
	}

	switch key {
	case "sms/home":
		kind, label, initial = "sms", "Mobile number", u.Query().Get("to_number")
	case "call/home":
		kind, label, initial = "call", "Number to call", u.Query().Get("number_to_call")
	case "help/contact_us":
		kind, initial = "support", "support"
	case "im/setup_im":
		kind, label, initial = "im", "External account username", u.Query().Get("im")
	}

	notice := pages.Stack()
	if method == "POST" {
		if op == "target" {
			target = strings.TrimSpace(body)
			op = "prepare"
		}
		if op == "send" {
			entry, e := s.activity.RecordProvider(ctx, activity.ProviderCommand{Actor: x.username, Kind: kind, Target: target, Body: body, Nonce: f.Get("nonce")})

			if e != nil {
				return serviceError(title, e)
			}

			notice = pages.Stack(notice, serviceText(fmt.Sprintf("Request #%d: %s.", entry.ID, entry.State)))
		} else if op != "prepare" {
			return serviceError(title, faults.ErrInvalid)
		}
		if op == "prepare" {
			if e := s.validateProviderTarget(ctx, kind, target); e != nil {
				return serviceError(title, e)
			}

			initial = target
		}
	}
	if kind == "support" {
		notice = pages.Stack(notice, serviceText("Support requests are saved locally for review."))
	} else if kind == "mail" {
		notice = pages.Stack(notice, serviceText("Mail to a local username is delivered to their local inbox. External mail waits for a configured provider."))
	} else if kind == "im" {
		notice = pages.Stack(notice, serviceText("External IM networks are not connected. Save an account label without a password; it remains disconnected."))
	} else {
		notice = pages.Stack(notice, serviceText("Local test service. Requests are recorded; no SMS is sent and no telephone call is placed."))
	}
	if initial == "" {
		notice = pages.Stack(notice, serviceForm(action, x.csrf, "target", "", label, "Continue"))
	} else {
		if e := s.validateProviderTarget(ctx, kind, initial); e != nil {
			return serviceError(title, e)
		}

		prompt := "Message"
		if kind == "call" {
			prompt = "Callback number"
		}
		if kind == "im" {
			prompt = "Account label (do not enter a password)"
		}

		notice = pages.Stack(notice, pages.Stack(serviceText("To: "+initial), serviceForm(action, x.csrf, "send", initial, prompt, "Save Request")))
	}

	entries, e := s.activity.ProviderEntries(ctx, x.username, kind)
	if e != nil {
		return serviceError(title, e)
	}

	for _, entry := range entries {
		notice = pages.Stack(notice, serviceText(fmt.Sprintf("#%d to %s [%s]: %s", entry.ID, entry.Target, entry.State, entry.Body)))
	}
	if kind == "mail" {
		inbox, e := s.activity.Inbox(ctx, x.username)
		if e != nil {
			return serviceError(title, e)
		}

		notice = pages.Stack(notice, serviceText("Inbox"))
		for _, entry := range inbox {
			notice = pages.Stack(notice, serviceText(entry.Owner+": "+entry.Body))
		}
	}

	return serviceResult(200, title, notice)
}

func (s *Adapter) validateProviderTarget(ctx context.Context, kind, target string) error {
	return s.activity.ValidateProvider(ctx, kind, target)
}
