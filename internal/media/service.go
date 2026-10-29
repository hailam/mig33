package media

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hailam/mig33/internal/faults"
)

type Privacy interface {
	Exists(context.Context, string) (bool, error)
	CanViewProfile(context.Context, string, string) (bool, error)
	BlocksIncoming(context.Context, string, string) (bool, error)
}

type Service struct {
	repo    Repository
	privacy Privacy
}

func New(repo Repository, privacy Privacy) *Service { return &Service{repo: repo, privacy: privacy} }

type AvatarChange struct{ Owner, Key string }

func PhotoToken(id int64) string { return "[photo:" + strconv.FormatInt(id, 10) + "]" }

func (s *Service) AddPhoto(ctx context.Context, owner, caption, nonce string, data []byte) (Photo, error) {
	return s.repo.AddPhoto(ctx, owner, caption, nonce, data)
}

func (s *Service) OwnedPhoto(ctx context.Context, owner string, id int64) (Photo, error) {
	if id <= 0 {
		return Photo{}, faults.ErrInvalid
	}

	photo, err := s.repo.Photo(ctx, id)
	if err != nil {
		return Photo{}, err
	}
	if photo.Owner != owner {
		return Photo{}, faults.ErrForbidden
	}
	return photo, nil
}

func (s *Service) CanReadPhoto(ctx context.Context, viewer string, photo Photo) (bool, error) {
	blocked, err := s.privacy.BlocksIncoming(ctx, viewer, photo.Owner)
	if err != nil || blocked {
		return false, err
	}

	allowed, err := s.privacy.CanViewProfile(ctx, viewer, photo.Owner)
	if err != nil {
		return false, err
	}
	if allowed {
		return true, nil
	}
	return s.repo.PhotoSharedWith(ctx, photo.ID, viewer)
}

func (s *Service) PhotoForViewer(ctx context.Context, viewer string, id int64) (Photo, error) {
	photo, err := s.repo.Photo(ctx, id)
	if err != nil {
		return Photo{}, err
	}

	allowed, err := s.CanReadPhoto(ctx, viewer, photo)
	if err != nil {
		return Photo{}, err
	}
	if !allowed {
		return Photo{}, faults.ErrForbidden
	}
	return photo, nil
}

// PhotosForViewer preserves scrapbook/wall discovery policy. A direct shared
// photo can be readable without exposing the owner's entire private scrapbook.
func (s *Service) PhotosForViewer(ctx context.Context, viewer, owner string) ([]Photo, error) {
	if owner != "" {
		exists, err := s.privacy.Exists(ctx, owner)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, faults.ErrForbidden
		}
		allowed, err := s.privacy.CanViewProfile(ctx, viewer, owner)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, faults.ErrForbidden
		}
	}

	photos, err := s.repo.Photos(ctx, owner)
	if err != nil {
		return nil, err
	}

	visible := make([]Photo, 0, len(photos))
	for _, photo := range photos {
		allowed, err := s.privacy.CanViewProfile(ctx, viewer, photo.Owner)
		if err != nil {
			return nil, err
		}
		if allowed {
			visible = append(visible, photo)
		}
	}
	return visible, nil
}

func (s *Service) DeletePhoto(ctx context.Context, owner string, id int64) error {
	return s.repo.DeletePhoto(ctx, owner, id)
}

func (s *Service) SetAvatar(ctx context.Context, owner string, id int64) (AvatarChange, error) {
	if err := s.repo.SetPhotoAvatar(ctx, owner, id); err != nil {
		return AvatarChange{}, err
	}
	return AvatarChange{Owner: owner, Key: fmt.Sprintf("photos/%d", id)}, nil
}

func (s *Service) ClearAvatar(ctx context.Context, owner string) (AvatarChange, error) {
	if err := s.repo.ClearPhotoAvatar(ctx, owner); err != nil {
		return AvatarChange{}, err
	}
	return AvatarChange{Owner: owner}, nil
}

func (s *Service) Avatar(ctx context.Context, owner string) (int64, error) {
	return s.repo.PhotoAvatar(ctx, owner)
}
