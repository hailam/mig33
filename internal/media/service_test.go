package media

import (
	"context"
	"errors"
	"testing"

	"github.com/hailam/mig33/internal/faults"
)

type mediaStore struct {
	Repository
	photo     Photo
	shared    bool
	avatarErr error
}

func (r mediaStore) Photo(context.Context, int64) (Photo, error) { return r.photo, nil }
func (r mediaStore) PhotoSharedWith(context.Context, int64, string) (bool, error) {
	return r.shared, nil
}
func (r mediaStore) SetPhotoAvatar(context.Context, string, int64) error { return r.avatarErr }

type mediaPrivacy struct{ visible, blocked bool }

func (p mediaPrivacy) Exists(context.Context, string) (bool, error) { return true, nil }
func (p mediaPrivacy) CanViewProfile(context.Context, string, string) (bool, error) {
	return p.visible, nil
}

func (p mediaPrivacy) BlocksIncoming(context.Context, string, string) (bool, error) {
	return p.blocked, nil
}

func TestSharedPrivatePhotoRemainsReadableUnlessBlocked(t *testing.T) {
	repo := mediaStore{photo: Photo{ID: 9, Owner: "alice"}, shared: true}
	photo, err := New(repo, mediaPrivacy{}).PhotoForViewer(context.Background(), "bob", 9)
	if err != nil || photo.ID != 9 {
		t.Fatalf("shared photo: %+v %v", photo, err)
	}
	_, err = New(repo, mediaPrivacy{blocked: true}).PhotoForViewer(context.Background(), "bob", 9)
	if !errors.Is(err, faults.ErrForbidden) {
		t.Fatalf("block did not override shared access: %v", err)
	}
	_, err = New(repo, mediaPrivacy{visible: true}).OwnedPhoto(context.Background(), "bob", 9)
	if !errors.Is(err, faults.ErrForbidden) {
		t.Fatalf("visibility granted ownership: %v", err)
	}
}

func TestAvatarBroadcastResultRequiresCommit(t *testing.T) {
	failed := errors.New("commit failure")
	change, err := New(mediaStore{avatarErr: failed}, mediaPrivacy{}).SetAvatar(context.Background(), "alice", 9)
	if !errors.Is(err, failed) || change.Owner != "" || change.Key != "" {
		t.Fatalf("premature avatar effect: %+v %v", change, err)
	}
	change, err = New(mediaStore{}, mediaPrivacy{}).SetAvatar(context.Background(), "alice", 9)
	if err != nil || change.Owner != "alice" || change.Key != "photos/9" {
		t.Fatalf("avatar effect: %+v %v", change, err)
	}
}
