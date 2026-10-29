package accounts

import (
	"context"
	"strconv"
	"strings"

	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/formatting"
	"github.com/hailam/mig33/internal/media"
)

type FormattingRepository interface {
	ProfileFormatting(context.Context, string) (string, error)
	SetProfileFormatting(context.Context, string, string) error
}

type FormattingPhotos interface {
	OwnedPhoto(context.Context, string, int64) (media.Photo, error)
}

// Customization validates authored markup and image ownership before storing
// source. Public image URLs and template construction belong to the adapter.
type Customization struct {
	repository FormattingRepository
	photos     FormattingPhotos
}

func NewCustomization(repository FormattingRepository, photos FormattingPhotos) *Customization {
	return &Customization{repository, photos}
}

func (s *Customization) Source(ctx context.Context, owner string) (string, error) {
	return s.repository.ProfileFormatting(ctx, owner)
}

func (s *Customization) Save(ctx context.Context, owner, source string) error {
	doc, err := formatting.Parse(source)
	if err != nil {
		return failed("format_invalid", err.Error(), err)
	}
	check := func(reference string) error {
		if !formatting.PhotoReference(reference) {
			return faults.ErrInvalid
		}
		id, err := strconv.ParseInt(strings.TrimPrefix(reference, "photo:"), 10, 64)
		if err != nil {
			return faults.ErrInvalid
		}
		_, err = s.photos.OwnedPhoto(ctx, owner, id)
		return err
	}
	if doc.BackgroundImage != "" {
		if err = check(doc.BackgroundImage); err != nil {
			return failed("format_photo", "Use only photos from your own scrapbook", err)
		}
	}
	if err = formatting.Walk(&doc, func(node *formatting.Node) error {
		if node.Tag == "img" {
			return check(node.Src)
		}
		return nil
	}); err != nil {
		return failed("format_photo", "Use only photos from your own scrapbook", err)
	}
	if err = s.repository.SetProfileFormatting(ctx, owner, source); err != nil {
		return failed("format_unavailable", "Profile formatting could not be saved", err)
	}
	return nil
}
