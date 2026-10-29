package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
)

const profileFormattingPreference = "profile_formatting"

// ProfileFormatting is separate from the plain About me field. Original page
// effects are authored explicitly; historical URL injection is not reproduced.
func (s *Store) ProfileFormatting(ctx context.Context, owner string) (string, error) {
	source, err := queries(s.db).GetIdentityPreference(ctx, sqlc.GetIdentityPreferenceParams{Username: strings.ToLower(owner), Name: profileFormattingPreference})
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}

	return source, queryError(ctx, err)
}

func (s *Store) SetProfileFormatting(ctx context.Context, owner, source string) error {
	if len(source) > 4096 || !utf8.ValidString(source) || strings.ContainsRune(source, 0) {
		return fmt.Errorf("%w: profile formatting", ErrInvalid)
	}

	err := queries(s.db).SetIdentityPreference(ctx, sqlc.SetIdentityPreferenceParams{Username: strings.ToLower(owner), Name: profileFormattingPreference, Value: source})
	return queryError(ctx, err)
}
