package legacyweb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/formatting"
	"github.com/hailam/mig33/internal/pages"
)

// resolveProfileFormatting converts visible authored photo references to
// bounded preview URLs. Missing or private photos are omitted from the view.
func (s *Adapter) resolveProfileFormatting(ctx context.Context, viewer *actor, owner string, doc *formatting.Document) error {
	resolve := func(reference string, size int) (string, error) {
		id, err := strconv.ParseInt(strings.TrimPrefix(reference, "photo:"), 10, 64)
		if err != nil || !formatting.PhotoReference(reference) {
			return "", faults.ErrInvalid
		}

		photo, err := s.media.PhotoForViewer(ctx, viewer.username, id)
		if err != nil {
			if errors.Is(err, faults.ErrNotFound) || errors.Is(err, faults.ErrForbidden) {
				return "", nil
			}

			return "", faults.ErrForbidden
		}
		if photo.Owner != owner {
			return "", nil
		}
		// The MIDlet does not scale image bytes to HTML width/height. Request
		// a bounded preview so an uploaded large photo fits the authored box.
		return fmt.Sprintf("%s/assets/photos/%d?w=%d", s.publicURL, id, size), nil
	}
	if doc.BackgroundImage != "" {
		value, err := resolve(doc.BackgroundImage, 240)
		if err != nil {
			return err
		}

		doc.BackgroundImage = value
	}

	return formatting.Walk(doc, func(n *formatting.Node) error {
		if n.Tag != "img" {
			return nil
		}

		value, err := resolve(n.Src, min(n.Width, n.Height))
		if err != nil {
			return err
		}
		if value == "" {
			*n = formatting.Node{}
			return nil
		}

		n.Src = value
		return nil
	})
}

func (s *Adapter) profileFormattingPage(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) pages.Result {
	const title = "Customize Profile"
	notice := ""
	if method == "POST" {
		if !identityFields(form, []string{"csrf"}, "op", "value") {
			return pages.RoomNotice(400, title, "Use the customization form")
		}

		source := form.Get("value")
		switch form.Get("op") {
		case "reset":
			if source != "" {
				return pages.RoomNotice(400, title, "Reset does not accept formatting")
			}

			source = ""
			notice = "Profile formatting reset."
		case "save", "":
			notice = "Profile formatting saved."
		default:
			return pages.RoomNotice(400, title, "Choose Save or Reset")
		}
		if err := s.customization.Save(ctx, x.username, source); err != nil {
			status := 400
			var failure *accounts.Failure
			if errors.As(err, &failure) && failure.Code == "format_unavailable" {
				status = 503
			}

			return pages.RoomNotice(status, title, err.Error())
		}

	}

	source, err := s.customization.Source(ctx, x.username)
	if err != nil {
		return pages.RoomNotice(503, title, "Profile formatting unavailable")
	}

	photos, err := s.media.PhotosForViewer(ctx, x.username, x.username)
	if err != nil {
		return pages.RoomNotice(503, title, "Scrapbook unavailable")
	}

	choices := make([]pages.ProfileFormattingPhoto, 0, len(photos))
	for _, photo := range photos {
		choices = append(choices, pages.ProfileFormattingPhoto{Reference: fmt.Sprintf("photo:%d", photo.ID), Caption: photo.Caption})
	}

	action := s.identityURL("profile", "edit") + "&field=customization"
	return identityHTML(200, title, pages.Stack(pages.Link(s.identityURL("profile", "home"), "View Profile"), pages.ProfileFormattingEditor(action, x.csrf, source, notice, choices)))
}

func (s *Adapter) formattedProfileResult(ctx context.Context, x *actor, name string, view pages.IdentityProfileData) pages.Result {
	source, err := s.customization.Source(ctx, name)
	if err != nil {
		return pages.RoomNotice(503, "User Profile", "Profile formatting unavailable")
	}

	doc, err := formatting.Parse(source)
	if err != nil {
		return pages.RoomNotice(503, "User Profile", "Profile formatting could not be read")
	}
	if err = s.resolveProfileFormatting(ctx, x, name, &doc); err != nil {
		return pages.RoomNotice(503, "User Profile", "Profile images unavailable")
	}

	return pages.RenderFormatted(200, "User Profile", doc, pages.Stack(pages.FormattedContent(doc), pages.Heading("User Profile"), pages.IdentityProfile(view)))
}
