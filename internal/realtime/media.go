package realtime

import (
	"context"
	"strconv"
	"strings"

	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/media"
)

func photoToken(id int64) string { return media.PhotoToken(id) }

func (s *Coordinator) prepareChatPhoto(ctx context.Context, x *session, request *commands.Request, content int16) (int64, error) {
	input := request.Input.(commands.SendMessage)
	if content == 2 {
		photo, err := s.mediaService.AddPhoto(ctx, x.username, "", token(), input.Photo)
		return photo.ID, err
	}
	ref, ok := input.Body.Get()
	if !ok {
		return 0, faults.ErrInvalid
	}

	id, err := strconv.ParseInt(ref, 10, 64)
	if err != nil || id <= 0 {
		return 0, faults.ErrInvalid
	}

	photo, err := s.mediaService.OwnedPhoto(ctx, x.username, id)
	return photo.ID, err
}

func (s *Coordinator) canReadPhoto(ctx context.Context, x *session, photo media.Photo) bool {
	allowed, err := s.mediaService.CanReadPhoto(ctx, x.username, photo)
	return err == nil && allowed
}

func (s *Coordinator) mediaToken(ctx context.Context, x *session, request *commands.Request) *events.Event {
	name, ok := request.Input.(commands.MediaToken).Name.Get()
	name = strings.TrimSpace(name)
	if !ok || !strings.HasPrefix(name, "[photo:") || !strings.HasSuffix(name, "]") {
		return s.catalogEmoticon(request, name)
	}

	id, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(name, "[photo:"), "]"), 10, 64)
	if err != nil {
		return events.Failure(request.Correlation, "Photo unavailable")
	}

	photo, err := s.mediaService.PhotoForViewer(ctx, x.username, id)
	if err != nil {
		return events.Failure(request.Correlation, "Photo unavailable")
	}
	preview, err := media.Preview(photo.Data, 96)
	if err != nil {
		return events.Failure(request.Correlation, "Photo preview unavailable")
	}
	return events.New(request.Correlation, events.Resource{Token: name, Label: "Photo", Type: 1, Data: preview})
}

func (s *Coordinator) setPhotoAvatar(ctx context.Context, x *session, id int64) error {
	change, err := s.mediaService.SetAvatar(ctx, x.username, id)
	if err != nil {
		return err
	}
	s.broadcastAvatar(ctx, change.Owner, change.Key)
	return nil
}

func (s *Coordinator) broadcastAvatar(ctx context.Context, owner, key string) {
	s.enqueue(owner, events.New(0, events.Profile{Avatar: &key}))
	for _, other := range s.sessions {
		if !other.authenticated || other.webOnly || other.username == owner {
			continue
		}
		contact, err := s.contactsService.ContactByUsername(ctx, other.username, owner)
		if err != nil {
			continue
		}
		select {
		case other.push <- events.New(0, events.Avatar{ContactID: contact.ID, Key: key}):
		default:
		}
	}
}

func (s *Coordinator) uploadPhoto(ctx context.Context, x *session, request *commands.Request) *events.Event {
	input := request.Input.(commands.UploadPhoto)
	purpose, ok := input.Purpose.Get()
	if !ok || purpose > 2 {
		return events.Failure(request.Correlation, "Invalid photo purpose")
	}
	if _, err := s.mediaService.AddPhoto(ctx, x.username, "", token(), input.Data); err != nil {
		return events.Failure(request.Correlation, "Use a PNG or JPEG photo up to 1 MiB and 2048 by 2048 pixels")
	}
	return events.New(request.Correlation, events.OpenFeature{Controller: "photo", Action: "home"})
}
