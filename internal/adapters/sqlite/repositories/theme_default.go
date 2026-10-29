package repositories

import (
	"context"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
)

// ResetStoreTheme restores the original bundled theme without a purchase.
func (s *Store) ResetStoreTheme(ctx context.Context, user string) error {
	name, err := username(user)
	if err != nil {
		return queryError(

			// Persist an explicit default selection so an offline device with an older
			// RMS theme also receives the original packet on its next login.
			ctx, err)
	}

	err = queries(s.db).SetStoreTheme(ctx, sqlc.SetStoreThemeParams{Username: name, Theme: "default"})
	return queryError(ctx, err)
}
