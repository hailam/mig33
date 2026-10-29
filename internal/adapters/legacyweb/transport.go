package legacyweb

import (
	"context"
	"net/url"
	"strings"

	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
	"github.com/hailam/mig33/internal/session"
)

func IsPublicURL(u *url.URL) bool { return isPublicIdentityURL(u) }

// FeaturePage is shared by browser forms and native embedded-page navigation.
func (s *Adapter) FeaturePage(ctx context.Context, id string, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	x, err := s.actor(ctx, id)
	if err != nil && !isPublicIdentityURL(request.URL) {
		return Result{}, faults.NewError(faults.Unauthenticated, "Login required: supply the gateway session in the sid header")
	}

	return s.pageForm(ctx, x, request.Method, request.URL, request.Form), nil
}

func (s *Adapter) BrowserPage(ctx context.Context, id string, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	x, err := s.actor(ctx, id)
	if err != nil {
		return Result{}, err
	}

	return s.wapPage(ctx, x, request.Method, request.URL, request.Form), nil
}

func (s *Adapter) OperatorDashboard(ctx context.Context, request session.OperatorDashboardRequest) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	view, joined, err := s.workflow.WebSession(ctx, request.SessionID)
	if err != nil {
		return Result{}, err
	}
	if !view.Operator {
		return Result{}, faults.NewError(faults.Forbidden, "Operator access required")
	}

	return s.operatorDashboard(ctx, actorView(view, joined), url.Values{
		"kind":           {request.Kind},
		"id":             {request.ItemID},
		"report_before":  {request.ReportBefore},
		"support_before": {request.SupportBefore},
	}), nil
}

func (s *Adapter) pageForm(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) pages.Result {
	if u != nil && u.Path == wapChatPath {
		return s.wapPage(ctx, x, method, u, form)
	}

	route, known := lookupPageRoute(u)
	if known && route.public {
		return route.handle(s, ctx, x, method, u, form)
	}
	if x == nil || !x.authenticated {
		return serviceResult(401, "Login Required", serviceText("Sign in to use this page."))
	}
	if u != nil && strings.HasPrefix(u.Path, "/assets/photos/") && method == "GET" {
		return s.photoResult(ctx, x, u)
	}
	if known {
		return route.handle(s, ctx, x, method, u, form)
	}

	return s.catalogPage(ctx, x, method, u)
}

type NativeRequest struct {
	URL               string
	Method            byte
	Body              string
	Purpose           int32
	HasURL, HasMethod bool
}
type NativeResult struct {
	Status int
	Body   []byte
	TTL    int64
}

// NativePage preserves opcode922's URL policy and incomplete form encoding.
// The native transport alone constructs the corresponding923 packet.
func (s *Adapter) NativePage(ctx context.Context, id string, input NativeRequest) NativeResult {
	response := func(status int, text string) NativeResult { return NativeResult{Status: status, Body: []byte(text)} }
	base, err := url.Parse(s.publicURL)
	if err != nil {
		return response(403, "Only this server's feature URLs are supported")
	}

	u, err := url.Parse(input.URL)
	if !input.HasURL || err != nil || u.User != nil || u.Fragment != "" || (u.IsAbs() && (u.Scheme != base.Scheme || u.Host != base.Host)) || (u.Host != "" && u.Host != base.Host) {
		return response(403, "Only this server's feature URLs are supported")
	}
	if !input.HasMethod || (input.Method != 1 && input.Method != 2) {
		return response(400, "Invalid page method")
	}

	method := "GET"
	var form url.Values
	if input.Method == 2 {
		method = "POST"
		if len(input.Body) > 8192 {
			return response(413, "Form is too large")
		}

		form, err = ParseNativeForm(u, input.Body)
		if err != nil {
			return response(400, "Invalid form")
		}
	}

	x, _ := s.actor(ctx, id)
	page := s.pageForm(ctx, x, method, u, form)
	result := NativeResult{Status: page.Status, Body: page.Body}
	if input.Purpose == 2 && method == "GET" && page.Status == 200 && strings.HasPrefix(page.ContentType, "image/") {
		result.TTL = 60000
	}

	return result
}

func ParseNativeForm(u *url.URL, body string) (url.Values, error) {
	switch {
	case u.Path == wapChatPath:
		return parseNativeWAPForm(body)
	case isReportURL(u):
		return parseNativeReportForm(body)
	case isIdentityURL(u):
		return parseNativeIdentityForm(u, body)
	case communityCommerceRoute(u):
		return parseCommunityCommerceForm(u, body)
	case serviceRoutes[featureKey(u)] != "":
		return parseNativeServiceForm(body)
	case isRoomPageURL(u) || isFriendSearchURL(u):
		return parseNativeRoomForm(u, body)
	default:
		return url.ParseQuery(body)
	}
}
