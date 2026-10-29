package accounts_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/adapters/sqlite/repositories"
)

type cancelAtCommit struct {
	accounts.Repository
	cancel                 context.CancelFunc
	registration, password bool
}

func (r *cancelAtCommit) RegisterIdentity(ctx context.Context, challenge string, identity accounts.Identity) (accounts.Identity, error) {
	value, err := r.Repository.RegisterIdentity(ctx, challenge, identity)
	if err == nil && r.registration {
		r.cancel()
	}
	return value, err
}

func (r *cancelAtCommit) ReplaceIdentityPassword(ctx context.Context, challenge, purpose, owner string, identity accounts.Identity) error {
	err := r.Repository.ReplaceIdentityPassword(ctx, challenge, purpose, owner, identity)
	if err == nil && r.password {
		r.cancel()
	}
	return err
}

func accountFixture(t *testing.T) (*repositories.Store, *accounts.Service) {
	t.Helper()
	repository, err := repositories.Open(t.Context(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := repository.Close(); err != nil {
			t.Error(err)
		}
	})
	return repository, accounts.NewService(repository)
}

func TestRegistrationReturnsCommittedIdentityAfterCallerCancellation(t *testing.T) {
	repository, base := accountFixture(t)
	challenge, err := base.Challenge(t.Context(), "registration", "", "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	service := accounts.NewService(&cancelAtCommit{Repository: repository, cancel: cancel, registration: true})
	identity, err := service.Register(ctx, accounts.Registration{
		Challenge: challenge.Token,
		Username:  "new_user",
		Password:  "PaSs 😀",
		Mobile:    "+9607771234",
		Code:      accounts.DevelopmentOTP,
		Activated: true,
	})
	if err != nil || identity.Username != "new_user" || identity.ID == 0 {
		t.Fatalf("committed identity lost: %+v %v", identity, err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("fixture did not cancel at commit")
	}
	if !accounts.VerifyPassword(identity, "pass 😀") {
		t.Fatal("returned credentials differ from accepted credentials")
	}
	roster, err := repository.Contacts(t.Context(), identity.Username)
	if err != nil {
		t.Fatal(err)
	}
	if len(roster.Groups) != 1 || roster.Groups[0].Name != "Friends" {
		t.Fatalf("registration roster not committed with identity: %+v", roster)
	}
}

func TestRecoveryReturnsFullCommittedIdentityWithoutPostCommitRead(t *testing.T) {
	repository, base := accountFixture(t)
	registration, err := base.Challenge(t.Context(), "registration", "", "")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := base.Register(t.Context(), accounts.Registration{
		Challenge: registration.Token,
		Username:  "recover_me",
		Password:  "before",
		Mobile:    "+9607771235",
		Code:      accounts.DevelopmentOTP,
		Activated: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	identity, err = base.UpdateProfile(t.Context(), identity, "bio", "retained profile")
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := base.Challenge(t.Context(), "recovery", identity.Username, identity.Mobile)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	service := accounts.NewService(&cancelAtCommit{Repository: repository, cancel: cancel, password: true})
	recovered, err := service.Recover(ctx, challenge.Token, accounts.DevelopmentOTP, "after")
	if err != nil || recovered.Bio != "retained profile" || !accounts.VerifyPassword(recovered, "after") {
		t.Fatalf("committed recovery lost: %+v %v", accounts.PublicProfile(recovered), err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("fixture did not cancel at commit")
	}
	stored, err := repository.Identity(t.Context(), identity.Username)
	if err != nil || !accounts.VerifyPassword(stored, "after") {
		t.Fatal("new credentials not committed", err)
	}
}

func TestLegacyProofKeepsUTF16AndLowercaseCompatibility(t *testing.T) {
	identity, err := accounts.PasswordIdentity("alice", "MiXeD 😀")
	if err != nil {
		t.Fatal(err)
	}
	// Java hashes UTF-16 code units, including both halves of a surrogate pair.
	if got := accounts.JavaHash("😀"); got != 1772899 {
		t.Fatalf("Java surrogate hash=%d", got)
	}
	if got, want := accounts.LegacyProof(identity, "challenge"), accounts.JavaHash("challenge"+"mixed 😀"); got != want {
		t.Fatalf("proof=%d want=%d", got, want)
	}
	if !accounts.VerifyPassword(identity, "MIXED 😀") {
		t.Fatal("legacy lowercase password contract changed")
	}
}
