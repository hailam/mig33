package activity

import (
	"context"
)

// Repository is the persistence capability required by activity.
// Implementations own transactions and must not expose driver or generated query types.
type Repository interface {
	AddServiceEntry(ctx context.Context, e ServiceEntry) (ServiceEntry, error)
	ServiceEntryByNonce(ctx context.Context, owner, nonce string) (ServiceEntry, error)
	ServiceEntryByID(ctx context.Context, id int64) (ServiceEntry, error)
	ActivityEntries(ctx context.Context, owner string) ([]ServiceEntry, error)
	ServiceEntries(ctx context.Context, kind, owner, target string) ([]ServiceEntry, error)
	DeleteServiceEntry(ctx context.Context, owner string, id int64) error
}
