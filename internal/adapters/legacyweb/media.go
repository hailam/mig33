package legacyweb

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/media"
	"github.com/hailam/mig33/internal/pages"
)

func (s *Adapter) photoPage(ctx context.Context, x *actor, method string, u *url.URL, f url.Values, op, target string) pages.Result {
	key := featureKey(u)
	title := serviceRoutes[key]
	owner := strings.ToLower(u.Query().Get("username"))
	if owner == "" && key != "photos/wall" {
		owner = x.username
	}
	if owner != "" && (s.account(ctx, owner) == nil || !s.canViewProfile(ctx, x.username, owner)) {
		return serviceError(title, faults.ErrForbidden)
	}
	if method == "POST" {
		id, e := strconv.ParseInt(target, 10, 64)
		if e != nil {
			return serviceError(title, faults.ErrInvalid)
		}

		switch op {
		case "avatar":
			e = s.workflow.SetAvatar(ctx, x.id, x.csrf, id)
		case "delete":
			e = s.media.DeletePhoto(ctx, x.username, id)
		default:
			e = faults.ErrInvalid
		}
		if e != nil {
			return serviceError(title, e)
		}
	}

	photos, e := s.media.PhotosForViewer(ctx, x.username, owner)
	if e != nil {
		return serviceError(title, e)
	}

	action := s.publicURL + u.EscapedPath() + "?" + u.Query().Encode()
	body := pages.Stack()
	if owner == x.username {
		body = pages.Stack(body, serviceLink("mig33:uploadPhotoToScrapbook()", "Upload Photo"))
	}

	for _, photo := range photos {
		if !s.canViewProfile(ctx, x.username, photo.Owner) {
			continue
		}

		id := strconv.FormatInt(photo.ID, 10)
		body = pages.Stack(body, serviceText(photo.Owner+": "+photo.Caption), pages.Picture(s.publicURL+"/assets/photos/"+id+"?w=96&h=96", 96, 96))
		if u.Query().Get("f") == "sendfromphoto" {
			body = pages.Stack(body, serviceLink("mig33:sendScrapbookPhoto("+id+")", "Send Photo"))
		}
		if photo.Owner == x.username {
			body = pages.Stack(body, serviceForm(action, x.csrf, "avatar", id, "", "Use as Display Picture"), serviceForm(action, x.csrf, "delete", id, "", "Delete Photo"))
		}
	}
	if len(photos) == 0 {
		body = pages.Stack(body, serviceText("No photos yet."))
	}

	return serviceResult(200, title, body)
}

func (s *Adapter) photoResult(ctx context.Context, x *actor, u *url.URL) pages.Result {
	path := u.Path
	id, e := strconv.ParseInt(strings.TrimPrefix(path, "/assets/photos/"), 10, 64)
	if e != nil {
		return serviceError("Photo", faults.ErrNotFound)
	}

	p, e := s.media.PhotoForViewer(ctx, x.username, id)
	if e != nil {
		return serviceError("Photo", e)
	}
	if size := u.Query().Get("w"); size != "" {
		max, err := strconv.Atoi(size)
		if err != nil || max < 1 || max > 240 {
			return serviceError("Photo", faults.ErrInvalid)
		}

		preview, err := media.Preview(p.Data, max)
		if err != nil {
			return serviceError("Photo", err)
		}

		return pages.Result{Status: 200, ContentType: "image/png", Body: preview}
	}

	return pages.Result{Status: 200, ContentType: p.MIME, Body: p.Data}
}

func mediaNotice(status int, message string) pages.Result {
	return pages.Result{Status: status, ContentType: "text/plain; charset=utf-8", Body: []byte(message + "\n")}
}

type MediaRequest struct {
	SessionID, Method, CSRF, IdempotencyKey string
	URL                                     *url.URL
	Data                                    []byte
}
type PhotoReceipt struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}
type MediaResult struct {
	Page  Result
	Photo *PhotoReceipt
}

func (s *Adapter) Media(ctx context.Context, input MediaRequest) (MediaResult, error) {
	if err := ctx.Err(); err != nil {
		return MediaResult{}, err
	}

	page := func(result pages.Result) (MediaResult, error) { return MediaResult{Page: result}, nil }
	x, err := s.actor(ctx, input.SessionID)
	if err != nil || x == nil || !x.authenticated {
		return page(mediaNotice(401, "Login required"))
	}
	if strings.HasPrefix(input.URL.Path, "/assets/photos/") {
		if input.Method != "GET" {
			return page(mediaNotice(405, "Use GET"))
		}

		return page(s.photoResult(ctx, x, input.URL))
	}
	if input.Method != "POST" {
		return page(mediaNotice(405, "Use POST with raw PNG/JPEG bytes"))
	}
	if input.CSRF != x.csrf || !validToken(input.IdempotencyKey) {
		return page(mediaNotice(403, "CSRF token and 48-hex Idempotency-Key required"))
	}

	photo, err := s.media.AddPhoto(ctx, x.username, input.URL.Query().Get("caption"), input.IdempotencyKey, input.Data)
	if err != nil {
		return page(serviceError("Upload Photo", err))
	}

	return MediaResult{Photo: &PhotoReceipt{ID: photo.ID, URL: fmt.Sprintf("%s/assets/photos/%d", s.publicURL, photo.ID)}}, nil
}
