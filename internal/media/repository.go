package media

import (
	"context"
)

// Repository is the persistence capability required by media.
// Implementations own transactions and must not expose driver or generated query types.
type Repository interface {
	ClearPhotoAvatar(ctx context.Context, owner string) error
	AddPhoto(ctx context.Context, owner, caption, nonce string, data []byte) (Photo, error)
	Photos(ctx context.Context, owner string) ([]Photo, error)
	Photo(ctx context.Context, id int64) (p Photo, err error)
	PhotoSharedWith(ctx context.Context, id int64, viewer string) (bool, error)
	SetPhotoAvatar(ctx context.Context, owner string, id int64) error
	PhotoAvatar(ctx context.Context, owner string) (int64, error)
	DeletePhoto(ctx context.Context, owner string, id int64) error
}
