package legacyweb

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/a-h/templ"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
)

func identitySelectors(u *url.URL) (string, string, bool) {
	if u == nil || u.Path != "/sites/index.php" {
		return "", "", false
	}

	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(q["c"]) != 1 || len(q["a"]) != 1 || len(q["v"]) != 1 || q.Get("v") != "midlet" {
		return "", "", false
	}

	return q.Get("c"), q.Get("a"), true
}

func isPublicIdentityURL(u *url.URL) bool {
	route, ok := lookupPageRoute(u)
	return ok && route.public
}

func isIdentityURL(u *url.URL) bool {
	if isPublicIdentityURL(u) {
		return true
	}

	c, a, ok := identitySelectors(u)
	if !ok {
		return false
	}
	if c == "profile" && (a == "edit" || a == "home") {
		return true
	}
	if c != "settings" {
		return false
	}

	switch a {
	case "change_password", "change_mobile", "chat_privacy", "profile_privacy_settings", "view_update_settings", "call_by_name", "change_tab_order":
		return true
	}

	return false
}

// Native forms have one unrestricted text control, last. The old applet only
// replaces spaces with %20. Do not turn a literal &, + or = in a password into
// another form field. Browser HTTP posts use ordinary ParseForm instead.
func parseNativeIdentityForm(u *url.URL, body string) (url.Values, error) {
	if len(body) > 8192 || !utf8.ValidString(body) {
		return nil, faults.ErrInvalid
	}

	c, a, _ := identitySelectors(u)
	field := ""
	switch {
	case c == "registration":
		field = "password"
	case a == "recover":
		field = "mobile"
		if u.Query().Get("stage") == "complete" {
			field = "password"
		}
	case a == "edit":
		field = "value"
	case a == "change_password":
		field = "password"
	}
	if field == "" {
		return url.ParseQuery(body)
	}

	var before, value string
	var found bool
	if strings.HasPrefix(body, field+"=") {
		value = strings.TrimPrefix(body, field+"=")
		found = true
	} else {
		before, value, found = strings.Cut(body, "&"+field+"=")
	}
	if !found {
		before = body
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

func identityHTML(status int, title string, body templ.Component) pages.Result {
	return pages.Render(status, title, pages.Stack(pages.Heading(title), body))
}

func (s *Adapter) identityURL(c, a string) string {
	return s.publicURL + "/sites/index.php?c=" + c + "&a=" + a + "&v=midlet"
}

func identityCSRF(x *actor, form url.Values) bool {
	return x != nil && x.csrf != "" && len(form["csrf"]) == 1 && subtle.ConstantTimeCompare([]byte(form.Get("csrf")), []byte(x.csrf)) == 1
}

func identityFields(form url.Values, required []string, optional ...string) bool {
	allowed := map[string]bool{}
	for _, key := range required {
		allowed[key] = true
		if len(form[key]) != 1 {
			return false
		}
	}

	for _, key := range optional {
		allowed[key] = true
	}

	for key, value := range form {
		if !allowed[key] || len(value) != 1 {
			return false
		}
	}

	return true
}

func (s *Adapter) identityPage(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) pages.Result {
	c, a, _ := identitySelectors(u)
	if method != "GET" && method != "POST" {
		return pages.RoomNotice(405, "Account", "Use GET or POST")
	}
	if c == "registration_api" {
		return s.usernameAvailability(ctx, method, u)
	}
	if c == "registration" {
		return s.registrationPage(ctx, method, u, form)
	}
	if c == "account" && a == "recover" {
		return s.recoveryPage(ctx, method, u, form)
	}
	if x == nil || !x.authenticated {
		return pages.RoomNotice(401, "Account", "Log in to view this page")
	}
	if method == "POST" && !identityCSRF(x, form) {
		return pages.RoomNotice(403, "Account", "This form expired; open the page again")
	}
	if c == "profile" && a == "home" {
		return s.identityProfilePage(ctx, x, method, u)
	}
	if c == "profile" && a == "edit" {
		return s.editIdentityPage(ctx, x, method, u, form)
	}
	if a == "change_password" {
		return s.changePasswordPage(ctx, x, method, u, form)
	}
	if a == "change_mobile" {
		return s.changeMobilePage(ctx, x, method, u, form)
	}

	return s.identityPreferencePage(ctx, x, method, u, form)
}

func (s *Adapter) usernameAvailability(ctx context.Context, method string, u *url.URL) pages.Result {
	result := pages.Result{Status: 200, ContentType: "text/plain; charset=utf-8"}
	if method != "GET" {
		result.Status = 405
		result.Body = []byte("[]")
		return result
	}
	if len(u.Query()["username"]) != 1 {
		result.Status = 400
		result.Body = []byte("[]")
		return result
	}

	suggestions, err := s.accounts.Suggestions(ctx, u.Query().Get("username"))
	if err != nil {
		result.Status = 400
		result.Body = []byte("[]")
		return result
	}

	result.Body = []byte("[" + strings.Join(suggestions, ",") + "]")
	return result
}

func (s *Adapter) registrationPage(ctx context.Context, method string, u *url.URL, form url.Values) pages.Result {
	if method == "POST" {
		if !identityFields(form, []string{"csrf", "username", "mobile", "code", "password", "agree"}) || form.Get("agree") != "on" {
			return pages.RoomNotice(400, "Registration", "Complete every field and agree to the local test terms")
		}

		a, err := s.workflow.RegisterIdentity(ctx, accounts.Registration{
			Challenge: form.Get("csrf"),
			Username:  form.Get("username"),
			Password:  form.Get("password"),
			Mobile:    form.Get("mobile"),
			Code:      form.Get("code"),
			Activated: true,
		})
		if err != nil {
			return pages.RoomNotice(400, "Registration", err.Error())
		}

		return identityHTML(
			201,
			"Registration complete",
			pages.Paragraph("Created "+a.Username+". Log in to the original app using your chosen password. Your number was verified with a development code; no SMS was sent."),
		)
	}

	c, err := s.accounts.Challenge(ctx, "registration", "", "")
	if err != nil {
		return pages.RoomNotice(503, "Registration", "Registration is busy; try again later")
	}

	return identityHTML(200, "Registration", pages.IdentityRegistration(s.identityURL("registration", "home"), c.Token, DevelopmentOTP))
}

func (s *Adapter) recoveryPage(ctx context.Context, method string, u *url.URL, form url.Values) pages.Result {
	if method == "GET" {
		c, err := s.accounts.Challenge(ctx, "recovery-request", "", "")
		if err != nil {
			return pages.RoomNotice(503, "Recover Account", "Try again later")
		}

		return identityHTML(200, "Recover Account", pages.IdentityRecoveryStart(s.identityURL("account", "recover"), c.Token, DevelopmentOTP))
	}
	if u.Query().Get("stage") == "complete" {
		if !identityFields(form, []string{"csrf", "code", "password"}) {
			return pages.RoomNotice(400, "Recover Account", "Enter the code and new password")
		}

		a, err := s.workflow.RecoverIdentity(ctx, form.Get("csrf"), form.Get("code"), form.Get("password"))
		if err != nil {
			return accountFailure("Recover Account", err)
		}

		return identityHTML(
			200,
			"Password reset",
			pages.Stack(pages.Paragraph("Username: "+a.Username), pages.Paragraph("Log in with your new password. Existing sessions have been signed out.")),
		)
	}
	if !identityFields(form, []string{"csrf", "mobile"}) {
		return pages.RoomNotice(400, "Recover Account", "Enter your registered mobile number")
	}

	c, err := s.accounts.RequestRecovery(ctx, form.Get("csrf"), form.Get("mobile"))
	if err != nil {
		return accountFailure("Recover Account", err)
	}

	return identityHTML(200, "Choose a new password", pages.IdentityRecoveryPassword(s.identityURL("account", "recover")+"&stage=complete", c.Token, DevelopmentOTP))
}

func (s *Adapter) identityProfilePage(ctx context.Context, x *actor, method string, u *url.URL) pages.Result {
	if method != "GET" {
		return pages.RoomNotice(405, "User Profile", "Use GET to view profiles")
	}

	name := strings.ToLower(u.Query().Get("username"))
	if name == "" {
		name = x.username
	}

	a := s.account(ctx, name)
	if a == nil {
		return pages.RoomNotice(404, "User Not Found", "The requested user was not found.")
	}
	if !s.canViewProfile(ctx, x.username, name) {
		return pages.RoomNotice(403, "Private Profile", "This user's profile privacy settings do not allow you to view it.")
	}

	presence := "Offline"
	if s.workflow.Presence(ctx, name) != 99 {
		presence = "Online"
	}

	view := pages.IdentityProfileData{
		Username: name, Display: a.display, Presence: presence, Status: a.status,
		Bio: a.identity.Bio, Country: a.identity.Country, Own: name == x.username,
		EditURL: s.identityURL("profile", "edit"), ChatURL: "mig33:privateChat(" + name + ")", AddFriendURL: "mig33:inviteFriend(" + name + ")",
		PhotosURL: s.identityURL("photo", "home") + "&username=" + url.QueryEscape(name),
		BlogURL:   s.identityURL("profile", "user_blog_posts") + "&username=" + url.QueryEscape(name),
	}
	return s.formattedProfileResult(ctx, x, name, view)
}

func (s *Adapter) editIdentityPage(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) pages.Result {
	labels := map[string]string{"display": "Display name", "status": "Status message", "bio": "About me", "country": "Country"}
	field := u.Query().Get("field")
	if field == "customization" {
		return s.profileFormattingPage(ctx, x, method, u, form)
	}

	notice := ""
	var a accounts.Profile
	if method == "POST" {
		if labels[field] == "" || !identityFields(form, []string{"csrf"}, "value") {
			return pages.RoomNotice(400, "Edit Profile", "Choose one profile field")
		}

		var err error
		a, err = s.workflow.UpdateProfile(ctx, x.id, x.csrf, field, form.Get("value"))
		if err != nil {
			return pages.RoomNotice(400, "Edit Profile", "Value is empty, too long or contains invalid characters")
		}

		notice = labels[field] + " saved."
	} else {
		var err error
		a, err = s.accounts.Profile(ctx, x.username)
		if err != nil {
			return pages.RoomNotice(503, "Edit Profile", "Account could not be loaded; try again")
		}
	}

	values := map[string]string{"display": a.Display, "status": a.Status, "bio": a.Bio, "country": a.Country}
	fields := make([]pages.IdentityEditField, 0, 4)
	for _, key := range []string{"display", "status", "bio", "country"} {
		fields = append(fields, pages.IdentityEditField{
			Label:      labels[key],
			Value:      values[key],
			InputLabel: "New " + strings.ToLower(labels[key]),
			Action:     s.identityURL("profile", "edit") + "&field=" + key,
		})
	}

	return identityHTML(
		200,
		"Edit Profile",
		pages.Stack(pages.Link(s.identityURL("profile", "edit")+"&field=customization", "Customize Profile"), pages.IdentityProfileEditor(x.csrf, notice, fields)),
	)
}

func (s *Adapter) changePasswordPage(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) pages.Result {
	if method == "GET" {
		return identityHTML(
			200,
			"Change Password",
			pages.IdentityPasswordForm(s.identityURL("settings", "change_password"), []pages.HiddenField{{Name: "csrf", Value: x.csrf}}, "Current password", "Continue"),
		)
	}
	if u.Query().Get("stage") == "new" {
		if !identityFields(form, []string{"csrf", "challenge", "password"}) {
			return pages.RoomNotice(400, "Change Password", "Enter a new password")
		}
		if err := s.workflow.CompletePasswordChange(ctx, x.id, x.csrf, form.Get("challenge"), form.Get("password")); err != nil {
			return accountFailure("Change Password", err)
		}

		return pages.RoomNotice(200, "Password Changed", "Your password was changed. Other sessions were signed out. Update saved login details on your device.")
	}
	if !identityFields(form, []string{"csrf", "password"}) {
		return pages.RoomNotice(403, "Change Password", "Current password is incorrect")
	}

	c, err := s.workflow.BeginPasswordChange(ctx, x.id, x.csrf, form.Get("password"))
	if err != nil {
		return accountFailure("Change Password", err)
	}

	return identityHTML(
		200,
		"New Password",
		pages.IdentityPasswordForm(
			s.identityURL("settings", "change_password")+"&stage=new",
			[]pages.HiddenField{{Name: "csrf", Value: x.csrf}, {Name: "challenge", Value: c.Token}},
			"New password",
			"Change Password",
		),
	)
}

func (s *Adapter) changeMobilePage(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) pages.Result {
	notice := ""
	var a accounts.Profile
	if method == "POST" {
		if !identityFields(form, []string{"csrf", "mobile", "code"}) {
			return pages.RoomNotice(400, "Change Mobile Number", "Enter a mobile number and verification code")
		}
		if form.Get("code") != DevelopmentOTP {
			return pages.RoomNotice(400, "Change Mobile Number", "Incorrect development verification code")
		}

		var err error
		a, err = s.workflow.ChangeMobile(ctx, x.id, x.csrf, form.Get("mobile"), form.Get("code"))
		if err != nil {
			return accountFailure("Change Mobile Number", err)
		}

		notice = "Mobile number updated and verified locally. No SMS was sent."
	} else {
		var err error
		a, err = s.accounts.Profile(ctx, x.username)
		if err != nil {
			return pages.RoomNotice(503, "Change Mobile Number", "Account could not be loaded; try again")
		}
	}

	return identityHTML(200, "Change Mobile Number", pages.IdentityMobileSettings(s.identityURL("settings", "change_mobile"), x.csrf, a.Mobile, notice, DevelopmentOTP))
}

var identityPreferenceOptions = map[string][][2]string{
	"chat_privacy":             {{"everyone", "Everyone"}, {"friends", "My contacts only"}, {"nobody", "Nobody"}},
	"profile_privacy_settings": {{"everyone", "Everyone"}, {"friends", "My contacts only"}, {"private", "Only me"}},
	"view_update_settings":     {{"on", "Show my activity"}, {"off", "Hide my activity"}},
	"call_by_name":             {{"on", "Allow calls by username"}, {"off", "Disable calls by username"}},
	"change_tab_order":         {{"friends,chatrooms,migworld", "Friends, Chat Rooms, migWorld"}, {"migworld,friends,chatrooms", "migWorld, Friends, Chat Rooms"}},
}

var identityPreferenceTitles = map[string]string{
	"chat_privacy":             "Chat Privacy",
	"profile_privacy_settings": "Profile Privacy",
	"view_update_settings":     "Activity Updates",
	"call_by_name":             "Call By Name",
	"change_tab_order":         "Tab Order",
}

func (s *Adapter) identityPreferencePage(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) pages.Result {
	name := u.Query().Get("a")
	title := identityPreferenceTitles[name]
	if title == "" {
		return pages.RoomNotice(404, "Settings", "Unknown setting")
	}

	notice, applyURL := "", ""
	if method == "POST" {
		if !identityFields(form, []string{"csrf", "value"}) {
			return pages.RoomNotice(400, title, "Choose one setting")
		}
		if err := s.accounts.SetPreference(ctx, x.username, name, form.Get("value")); err != nil {
			return pages.RoomNotice(400, title, "Choose a listed setting")
		}

		notice = "Setting saved."
		if name == "change_tab_order" {
			command := "1"
			if form.Get("value") == "migworld,friends,chatrooms" {
				command = "0"
			}

			applyURL = "mig33:reorderHomeTab(" + command + ")"
		}
	}

	current, err := s.accounts.Preference(ctx, x.username, name)
	if err != nil {
		return pages.RoomNotice(500, title, "Setting could not be loaded")
	}

	return identityHTML(200, title, pages.IdentityPreferenceSettings(s.identityURL("settings", name), x.csrf, current, notice, applyURL, identityPreferenceOptions[name]))
}

func accountFailure(title string, err error) pages.Result {
	status := 400
	var failure *accounts.Failure
	if errors.As(err, &failure) {
		switch failure.Code {
		case "recovery_expired", "recovery_request_expired", "password_incorrect", "password_expired":
			status = 403
		case "recovery_user_missing":
			status = 404
		case "recovery_changed", "mobile_conflict":
			status = 409
		case "unavailable", "identity_unavailable":
			status = 503
		}
	}

	return pages.RoomNotice(status, title, err.Error())
}
