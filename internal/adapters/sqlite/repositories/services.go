package repositories

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
	"github.com/hailam/mig33/internal/media"
)

// ServiceEntry is a durable local publication, game result or provider request.
// A provider request remains pending_provider; recording it never means sent.
type ServiceEntry = activity.ServiceEntry

type Photo = media.Photo

func (s *Store) AddServiceEntry(ctx context.Context, e ServiceEntry) (ServiceEntry, error) {
	switch e.Kind {
	case "buzz", "blog", "checkin", "game", "mail", "sms", "call", "invite", "support", "im":
	default:
		return e, ErrInvalid
	}
	if _, err := username(e.Owner); err != nil {
		return e, queryError(ctx, err)
	}
	if len(e.Nonce) != 48 || len(e.Body) > 4000 || !utf8.ValidString(e.Body) || strings.ContainsRune(e.Body, 0) || len(e.Target) > 320 || strings.TrimSpace(e.Body) == "" {
		return e, ErrInvalid
	}
	if e.State != "published" && e.State != "local_delivered" && e.State != "pending_provider" && e.State != "open" && e.State != "completed" && e.State != "disconnected" {
		return e, ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return e, queryError(ctx, err)
	}
	defer tx.Rollback()

	record, err := queries(tx).ServiceEntryByNonce(ctx, sqlc.ServiceEntryByNonceParams{Owner: e.Owner, Nonce: e.Nonce})
	old := ServiceEntry(record)
	if err == nil {
		if old.Kind != e.Kind || old.Target != e.Target || old.Body != e.Body || old.State != e.State {
			return e, ErrConflict
		}

		return old, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return e, queryError(ctx, err)
	}

	e.CreatedAt = time.Now().UnixMilli()
	e.ID, err = queries(tx).CreateServiceEntry(ctx, sqlc.CreateServiceEntryParams{
		Kind:      e.Kind,
		Owner:     e.Owner,
		Target:    e.Target,
		Body:      e.Body,
		State:     e.State,
		Nonce:     e.Nonce,
		CreatedAt: e.CreatedAt,
	})
	if err != nil {
		return e, queryError(ctx, err)
	}

	return e, queryError(ctx, tx.Commit())
}

func (s *Store) ServiceEntryByNonce(ctx context.Context, owner, nonce string) (ServiceEntry, error) {
	row, err := queries(s.db).ServiceEntryByNonce(ctx, sqlc.ServiceEntryByNonceParams{Owner: owner, Nonce: nonce})
	return ServiceEntry(row), queryError(ctx, err)
}

func (s *Store) ServiceEntryByID(ctx context.Context, id int64) (ServiceEntry, error) {
	row, err := queries(s.db).ServiceEntryByID(ctx, id)
	return ServiceEntry(row), queryError(ctx, err)
}

// ActivityEntries combines authored updates and check-ins in one bounded feed.
// Viewer privacy and block policy are enforced by the authenticated caller.
func (s *Store) ActivityEntries(ctx context.Context, owner string) ([]ServiceEntry, error) {
	rows, err := queries(s.db).ListActivityEntries(ctx, owner)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	var out []ServiceEntry
	for _, r := range rows {
		out = append(out, ServiceEntry(r))
	}

	return out, nil
}

// ServiceEntries has bounded results; authorization of a public feed is the caller's job.
func (s *Store) ServiceEntries(ctx context.Context, kind, owner, target string) ([]ServiceEntry, error) {
	rows, err := queries(s.db).ListServiceEntries(ctx, sqlc.ListServiceEntriesParams{Kind: kind, Owner: owner, Target: target})
	if err != nil {
		return nil, queryError(ctx, err)
	}

	out := make([]ServiceEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, ServiceEntry(r))
	}

	return out, nil
}

func (s *Store) DeleteServiceEntry(ctx context.Context, owner string, id int64) error {
	n, err := queries(s.db).DeleteServiceEntry(ctx, sqlc.DeleteServiceEntryParams{Owner: owner, ID: id})
	if err == nil && n == 0 {
		return ErrNotFound
	}

	return queryError(ctx, err)
}

func (s *Store) AddPhoto(ctx context.Context, owner, caption, nonce string, data []byte) (Photo, error) {
	p := Photo{Owner: owner, Caption: caption, Data: data, CreatedAt: time.Now().UnixMilli()}
	if _, err := username(owner); err != nil {
		return p, queryError(ctx, err)
	}
	if len(nonce) != 48 || len(data) == 0 || len(data) > 1024*1024 || len(caption) > 320 || !utf8.ValidString(caption) {
		return p, ErrInvalid
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 2048 || config.Height > 2048 || (format != "jpeg" && format != "png") {
		return p, fmt.Errorf("%w: PNG/JPEG, at most 2048x2048 and 1 MiB", ErrInvalid)
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return p, ErrInvalid
	}

	p.MIME = "image/" + format
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return p, queryError(ctx, err)
	}
	defer tx.Rollback()

	record, err := queries(tx).PhotoByNonce(ctx, sqlc.PhotoByNonceParams{Owner: owner, Nonce: nonce})
	old := photoModel(record)
	if err == nil {
		if old.Caption != caption || !bytes.Equal(old.Data, data) {
			return p, ErrConflict
		}

		return old, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return p, queryError(ctx, err)
	}

	count, err := queries(tx).CountOwnerPhotos(ctx, owner)
	if err != nil {
		return p, queryError(ctx, err)
	}
	if count >= 100 {
		return p, ErrLimit
	}

	p.ID, err = queries(tx).CreatePhoto(ctx, sqlc.CreatePhotoParams{Owner: owner, Caption: caption, Mime: p.MIME, Data: data, Nonce: nonce, CreatedAt: p.CreatedAt})
	if err != nil {
		return p, queryError(ctx, err)
	}

	return p, queryError(ctx, tx.Commit())
}

func (s *Store) Photos(ctx context.Context, owner string) ([]Photo, error) {
	rows, err := queries(s.db).ListPhotos(ctx, owner)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	out := make([]Photo, 0, len(rows))
	for _, r := range rows {
		out = append(out, Photo{ID: r.ID, Owner: r.Owner, Caption: r.Caption, MIME: r.Mime, CreatedAt: r.CreatedAt})
	}

	return out, nil
}

func (s *Store) Photo(ctx context.Context, id int64) (Photo, error) {
	row, err := queries(s.db).GetPhoto(ctx, id)
	return photoModel(row), queryError(ctx, err)
}

func photoModel(r sqlc.ServicePhoto) Photo {
	return Photo{ID: r.ID, Owner: r.Owner, Caption: r.Caption, MIME: r.Mime, Data: r.Data, CreatedAt: r.CreatedAt}
}

// PhotoSharedWith checks the immutable chat audience, including block cancellation.
func (s *Store) PhotoSharedWith(ctx context.Context, id int64, viewer string) (bool, error) {
	value, err := queries(s.db).PhotoSharedWith(ctx, sqlc.PhotoSharedWithParams{PhotoID: id, Username: viewer})
	return value, queryError(ctx, err)
}

func (s *Store) SetPhotoAvatar(ctx context.Context, owner string, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	actual, err := queries(tx).GetPhotoOwner(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return queryError(ctx, err)
	}
	if actual != owner {
		return ErrForbidden
	}

	err = queries(tx).SetPhotoAvatar(ctx, sqlc.SetPhotoAvatarParams{Owner: owner, PhotoID: sql.NullInt64{Int64: id, Valid: true}})
	if err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func (s *Store) PhotoAvatar(ctx context.Context, owner string) (int64, error) {
	id, err := queries(s.db).GetPhotoAvatar(ctx, owner)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}

	return id.Int64, queryError(ctx, err)
}

func (s *Store) DeletePhoto(ctx context.Context, owner string, id int64) error {
	n, err := queries(s.db).DeletePhoto(ctx, sqlc.DeletePhotoParams{Owner: owner, ID: id})
	if err == nil && n == 0 {
		return ErrNotFound
	}

	return queryError(ctx, err)
}
