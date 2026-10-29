package accounts

import (
	"context"
)

// Repository is the persistence capability required by accounts.
// Implementations own transactions and must not expose driver or generated query types.
type Repository interface {
	Identity(ctx context.Context, name string) (Identity, error)
	IdentityByMobile(ctx context.Context, mobile string) (Identity, error)
	Identities(ctx context.Context) ([]Identity, error)
	SeedIdentity(ctx context.Context, a Identity) error
	CreateIdentityChallenge(ctx context.Context, c IdentityChallenge) error
	IdentityChallenge(ctx context.Context, value, purpose string) (IdentityChallenge, error)
	FailIdentityChallenge(ctx context.Context, value string) error
	ConsumeIdentityChallenge(ctx context.Context, value, purpose string) error
	// RegisterIdentity atomically consumes the registration challenge and creates
	// the identity, its empty contact roster and default Friends group. A returned
	// identity is committed and ready for login without additional setup calls.
	RegisterIdentity(ctx context.Context, value string, a Identity) (Identity, error)
	UpdateIdentityField(ctx context.Context, owner, field, value string) error
	SetIdentityMobile(ctx context.Context, owner, mobile string, activated bool) error
	ActivateIdentity(ctx context.Context, owner string) error
	ReplaceIdentityPassword(ctx context.Context, value, purpose, owner string, a Identity) error
	SetIdentityPreference(ctx context.Context, owner, name, value string) error
	IdentityPreference(ctx context.Context, owner, name string) (string, error)
	ProfileFormatting(ctx context.Context, owner string) (string, error)
	SetProfileFormatting(ctx context.Context, owner, source string) error
	SetLanguage(ctx context.Context, owner, code string) error
	Language(ctx context.Context, owner string) (string, error)
}
