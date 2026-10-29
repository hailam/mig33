package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
)

// Identity contains credential verifiers, never the original password. The
// legacy verifier is deliberately retained only for the original 202 protocol.
type Identity = accounts.Identity

func identityModel(a sqlc.Identity) Identity {
	return Identity{
		ID:                 int32(a.ID),
		Username:           a.Username,
		Display:            a.Display,
		Status:             a.Status,
		Mobile:             a.Mobile,
		Bio:                a.Bio,
		Country:            a.Country,
		Activated:          a.Activated != 0,
		PasswordSalt:       a.PasswordSalt,
		PasswordKey:        a.PasswordKey,
		PasswordIterations: int(a.PasswordIterations),
		LegacyHash:         int32(a.LegacyHash),
		LegacyUnits:        int(a.LegacyUnits),
	}
}

func (s *Store) Identity(ctx context.Context, name string) (Identity, error) {
	row, err := queries(s.db).GetIdentity(ctx, strings.ToLower(name))
	return identityModel(row), queryError(ctx, err)
}

func (s *Store) IdentityByMobile(ctx context.Context, mobile string) (Identity, error) {
	if mobile == "" {
		return Identity{}, ErrNotFound
	}

	row, err := queries(s.db).GetIdentityByMobile(ctx, mobile)
	return identityModel(row), queryError(ctx, err)
}

func (s *Store) Identities(ctx context.Context) ([]Identity, error) {
	rows, err := queries(s.db).ListIdentities(ctx)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	var result []Identity
	for _, row := range rows {
		result = append(result, identityModel(row))
	}

	return result, nil
}

func validateIdentity(a Identity) error {
	if len(a.Username) < 3 || len(a.Username) > 32 || a.Username != strings.ToLower(a.Username) {
		return ErrInvalid
	}

	for _, r := range a.Username {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '.' || r == '-') {
			return ErrInvalid
		}
	}
	if len(a.PasswordSalt) != 16 || len(a.PasswordKey) != 32 || a.PasswordIterations < 100000 || a.LegacyUnits < 1 || a.LegacyUnits > 640 {
		return ErrInvalid
	}

	return nil
}

// SeedIdentity preserves an existing account including changed credentials.
func (s *Store) SeedIdentity(ctx context.Context, a Identity) error {
	if err := validateIdentity(a); err != nil {
		return queryError(ctx, err)
	}

	err := queries(s.db).SeedIdentity(ctx, sqlc.SeedIdentityParams{
		ID:                 int64(a.ID),
		Username:           a.Username,
		Display:            a.Display,
		Status:             a.Status,
		Mobile:             a.Mobile,
		Activated:          boolInt64(a.Activated),
		PasswordSalt:       a.PasswordSalt,
		PasswordKey:        a.PasswordKey,
		PasswordIterations: int64(a.PasswordIterations),
		LegacyHash:         int64(a.LegacyHash),
		LegacyUnits:        int64(a.LegacyUnits),
		CreatedAt:          time.Now().UnixMilli(),
	})
	return queryError(ctx, err)
}

type IdentityChallenge = accounts.IdentityChallenge

func (s *Store) CreateIdentityChallenge(ctx context.Context, c IdentityChallenge) error {
	if len(c.Token) != 48 || c.Purpose == "" || len(c.Purpose) > 100 || c.ExpiresAt <= time.Now().UnixMilli() {
		return ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	if err = queries(tx).DeleteExpiredIdentityChallenges(ctx, time.Now().UnixMilli()); err != nil {
		return queryError(ctx, err)
	}

	count, err := queries(tx).CountIdentityChallenges(ctx)
	if err != nil {
		return queryError(ctx, err)
	}
	if count >= 512 {
		return ErrLimit
	}
	if err = queries(tx).CreateIdentityChallenge(ctx, sqlc.CreateIdentityChallengeParams{
		Token:     c.Token,
		Purpose:   c.Purpose,
		Username:  c.Username,
		Mobile:    c.Mobile,
		ExpiresAt: c.ExpiresAt,
	}); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func (s *Store) IdentityChallenge(ctx context.Context, value, purpose string) (IdentityChallenge, error) {
	row, err := queries(s.db).GetIdentityChallenge(ctx, sqlc.GetIdentityChallengeParams{Token: value, Purpose: purpose, Now: time.Now().UnixMilli()})
	c := IdentityChallenge{
		Token:     row.Token,
		Purpose:   row.Purpose,
		Username:  row.Username,
		Mobile:    row.Mobile,
		ExpiresAt: row.ExpiresAt,
		Attempts:  int(row.Attempts),
	}
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}

	return c, queryError(ctx, err)
}

func (s *Store) FailIdentityChallenge(ctx context.Context, value string) error {
	err := queries(s.db).FailIdentityChallenge(ctx, value)
	return queryError(ctx, err)
}

func (s *Store) ConsumeIdentityChallenge(ctx context.Context, value, purpose string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	if err = consumeIdentityChallenge(ctx, tx, value, purpose); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func consumeIdentityChallenge(ctx context.Context, tx *sql.Tx, value, purpose string) error {
	n, err := queries(tx).ConsumeIdentityChallenge(ctx, sqlc.ConsumeIdentityChallengeParams{Token: value, Purpose: purpose, Now: time.Now().UnixMilli()})
	if err != nil {
		return queryError(ctx, err)
	}
	if n != 1 {
		return ErrNotFound
	}

	return nil
}

// RegisterIdentity consumes the registration challenge and creates the account,
// initial contact roster and Friends group in one transaction. A committed
// account is ready for login even if its registration request then disappears.
func (s *Store) RegisterIdentity(ctx context.Context, value string, a Identity) (Identity, error) {
	if err := validateIdentity(a); err != nil {
		return Identity{}, queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Identity{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	if err = consumeIdentityChallenge(ctx, tx, value, "registration"); err != nil {
		return Identity{}, queryError(ctx, err)
	}

	exists, err := queries(tx).CountConflictingIdentities(ctx, sqlc.CountConflictingIdentitiesParams{Username: a.Username, Mobile: a.Mobile})
	if err != nil {
		return Identity{}, queryError(ctx, err)
	}
	if exists != 0 {
		return Identity{}, ErrConflict
	}

	now := time.Now().UnixMilli()
	id, err := queries(tx).RegisterIdentity(ctx, sqlc.RegisterIdentityParams{
		Username:           a.Username,
		Display:            a.Display,
		Status:             a.Status,
		Mobile:             a.Mobile,
		Activated:          boolInt64(a.Activated),
		PasswordSalt:       a.PasswordSalt,
		PasswordKey:        a.PasswordKey,
		PasswordIterations: int64(a.PasswordIterations),
		LegacyHash:         int64(a.LegacyHash),
		LegacyUnits:        int64(a.LegacyUnits),
		CreatedAt:          now,
	})
	if err != nil {
		return Identity{}, queryError(ctx, err)
	}

	a.ID = int32(id)
	if err := queries(tx).CreateContactRoster(ctx, sqlc.CreateContactRosterParams{Owner: a.Username, Now: now}); err != nil {
		return Identity{}, queryError(ctx, err)
	}
	if err := queries(tx).CreateDefaultContactGroup(ctx, sqlc.CreateDefaultContactGroupParams{Owner: a.Username, Now: now}); err != nil {
		return Identity{}, queryError(ctx, err)
	}

	if err = tx.Commit(); err != nil {
		return Identity{}, queryError(ctx, err)
	}

	return a, nil
}

func (s *Store) UpdateIdentityField(ctx context.Context, owner, field, value string) error {
	limit := 320
	switch field {
	case "display":
		limit = 64
	case "status":
		limit = 512
	case "bio":
	case "country":
		limit = 64
	default:
		return ErrInvalid
	}
	if len(value) > limit || !utf8.ValidString(value) || strings.ContainsRune(value, 0) || (field == "display" && strings.TrimSpace(value) == "") {
		return ErrInvalid
	}

	var n int64
	var err error
	q := queries(s.db)
	switch field {
	case "display":
		n, err = q.UpdateIdentityDisplay(ctx, sqlc.UpdateIdentityDisplayParams{Value: value, Username: owner})
	case "status":
		n, err = q.UpdateIdentityStatus(ctx, sqlc.UpdateIdentityStatusParams{Value: value, Username: owner})
	case "bio":
		n, err = q.UpdateIdentityBio(ctx, sqlc.UpdateIdentityBioParams{Value: value, Username: owner})
	case "country":
		n, err = q.UpdateIdentityCountry(ctx, sqlc.UpdateIdentityCountryParams{Value: value, Username: owner})
	}
	if err != nil {
		return queryError(ctx, err)
	}
	if n != 1 {
		return ErrNotFound
	}

	return nil
}

func (s *Store) SetIdentityMobile(ctx context.Context, owner, mobile string, activated bool) error {
	if mobile == "" {
		return ErrInvalid
	}

	count, err := queries(s.db).CountMobileOwners(ctx, sqlc.CountMobileOwnersParams{Mobile: mobile, Username: owner})
	if err != nil {
		return queryError(ctx, err)
	}
	if count != 0 {
		return ErrConflict
	}

	n, err := queries(s.db).SetIdentityMobile(ctx, sqlc.SetIdentityMobileParams{Mobile: mobile, Activated: boolInt64(activated), Username: owner})
	if err != nil {
		return queryError(ctx, err)
	}
	if n != 1 {
		return ErrNotFound
	}

	return nil
}

func (s *Store) ActivateIdentity(ctx context.Context, owner string) error {
	n, err := queries(s.db).ActivateIdentity(ctx, owner)
	if err != nil {
		return queryError(ctx, err)
	}
	if n != 1 {
		return ErrNotFound
	}

	return nil
}

func (s *Store) ReplaceIdentityPassword(ctx context.Context, value, purpose, owner string, a Identity) error {
	if err := validateIdentity(a); err != nil {
		return queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	challenge, err := queries(tx).IdentityChallengeOwner(ctx, sqlc.IdentityChallengeOwnerParams{Token: value, Purpose: purpose})
	if err != nil {
		return queryError(ctx, err)
	}
	if challenge.Username != owner {
		return ErrForbidden
	}
	if purpose == "recovery" {
		actual, err := queries(tx).GetIdentityMobile(ctx, owner)
		if err != nil {
			return queryError(ctx, err)
		}
		if actual != challenge.Mobile {
			return ErrForbidden
		}
	}
	if err = consumeIdentityChallenge(ctx, tx, value, purpose); err != nil {
		return queryError(ctx, err)
	}

	n, err := queries(tx).ReplaceIdentityPassword(ctx, sqlc.ReplaceIdentityPasswordParams{
		PasswordSalt:       a.PasswordSalt,
		PasswordKey:        a.PasswordKey,
		PasswordIterations: int64(a.PasswordIterations),
		LegacyHash:         int64(a.LegacyHash),
		LegacyUnits:        int64(a.LegacyUnits),
		Username:           owner,
	})
	if err != nil {
		return queryError(ctx, err)
	}
	if n != 1 {
		return ErrNotFound
	}

	return queryError(ctx, tx.Commit())
}

var identityPreferenceValues = map[string][]string{
	"chat_privacy":             {"everyone", "friends", "nobody"},
	"profile_privacy_settings": {"everyone", "friends", "private"},
	"view_update_settings":     {"on", "off"},
	"call_by_name":             {"on", "off"},
	"change_tab_order":         {"friends,chatrooms,migworld", "migworld,friends,chatrooms"},
}

func (s *Store) SetIdentityPreference(ctx context.Context, owner, name, value string) error {
	allowed := false
	for _, v := range identityPreferenceValues[name] {
		if v == value {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("%w: preference", ErrInvalid)
	}

	err := queries(s.db).SetIdentityPreference(ctx, sqlc.SetIdentityPreferenceParams{Username: owner, Name: name, Value: value})
	return queryError(ctx, err)
}

func (s *Store) IdentityPreference(ctx context.Context, owner, name string) (string, error) {
	choices, ok := identityPreferenceValues[name]
	if !ok {
		return "", ErrInvalid
	}

	value, err := queries(s.db).GetIdentityPreference(ctx, sqlc.GetIdentityPreferenceParams{Username: owner, Name: name})
	if errors.Is(err, sql.ErrNoRows) {
		return choices[0], nil
	}

	return value, queryError(ctx, err)
}
