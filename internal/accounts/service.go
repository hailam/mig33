package accounts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hailam/mig33/internal/faults"
)

// Service owns durable identity operations. A successful mutation returns the
// committed identity so its caller can update realtime state without another
// cancellable read. It does not own sessions or transport delivery locks.
type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

// Failure identifies the stage of a multi-step account operation independently
// of how a caller renders or transports the error.
type Failure struct {
	Code, Message string
	Cause         error
}

func (e *Failure) Error() string                     { return e.Message }
func (e *Failure) Unwrap() error                     { return e.Cause }
func failed(code, message string, cause error) error { return &Failure{code, message, cause} }

var seeds struct {
	sync.Once
	identities []Identity
	err        error
}

func (s *Service) Initialize(ctx context.Context) ([]Identity, error) {
	seeds.Do(func() {
		for i, name := range []string{"alice", "bob", "carol", "dave", "hailam"} {
			password := name + "-demo"
			if name == "hailam" {
				password = name
			}
			a, err := PasswordIdentity(name, password)
			if err != nil {
				seeds.err = err
				return
			}
			a.ID, a.Display, a.Status, a.Activated = int32(i+1), strings.ToUpper(name[:1])+name[1:], "Local test account", true
			seeds.identities = append(seeds.identities, a)
		}
	})
	if seeds.err != nil {
		return nil, seeds.err
	}

	for _, a := range seeds.identities {
		if err := s.repository.SeedIdentity(ctx, a); err != nil {
			return nil, err
		}
	}
	return s.repository.Identities(ctx)
}

func (s *Service) Identity(ctx context.Context, name string) (Identity, error) {
	return s.repository.Identity(ctx, name)
}

func (s *Service) Identities(ctx context.Context) ([]Identity, error) {
	return s.repository.Identities(ctx)
}

func (s *Service) Exists(ctx context.Context, name string) bool {
	_, err := s.repository.Identity(ctx, name)
	return err == nil
}

func (s *Service) Suggestions(ctx context.Context, value string) ([]string, error) {
	name, err := NormalizeName(value)
	if err != nil {
		return nil, err
	}

	_, err = s.repository.Identity(ctx, name)
	if errors.Is(err, faults.ErrNotFound) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(name) > 28 {
		name = name[:28]
	}

	var result []string
	for i := 1; len(result) < 3; i++ {
		candidate := fmt.Sprintf("%s%d", name, i)
		_, err := s.repository.Identity(ctx, candidate)
		if errors.Is(err, faults.ErrNotFound) {
			result = append(result, candidate)
		} else if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Service) Challenge(ctx context.Context, purpose, user, mobile string) (IdentityChallenge, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return IdentityChallenge{}, err
	}
	c := IdentityChallenge{
		Token:     hex.EncodeToString(raw[:]),
		Purpose:   purpose,
		Username:  user,
		Mobile:    mobile,
		ExpiresAt: time.Now().Add(15 * time.Minute).UnixMilli(),
	}
	return c, s.repository.CreateIdentityChallenge(ctx, c)
}

type Registration struct {
	Challenge, Username, Password, Mobile, Code string
	Activated                                   bool
}

func (s *Service) Register(ctx context.Context, input Registration) (Identity, error) {
	if _, err := s.repository.IdentityChallenge(ctx, input.Challenge, "registration"); err != nil {
		return Identity{}, failed("registration_expired", "Registration expired; start again", err)
	}
	if input.Code != DevelopmentOTP {
		_ = s.repository.FailIdentityChallenge(ctx, input.Challenge)
		return Identity{}, failed("code", "Use the development security code "+DevelopmentOTP, nil)
	}

	name, err := NormalizeName(input.Username)
	if err != nil {
		return Identity{}, err
	}
	phone, err := NormalizeMobile(input.Mobile)
	if err != nil {
		return Identity{}, err
	}

	_, err = s.repository.Identity(ctx, name)
	if err == nil {
		return Identity{}, failed("username_used", "Username is already in use", nil)
	}
	if !errors.Is(err, faults.ErrNotFound) {
		return Identity{}, err
	}

	a, err := PasswordIdentity(name, input.Password)
	if err != nil {
		return Identity{}, err
	}
	a.Display, a.Mobile, a.Activated = name, phone, input.Activated

	created, err := s.repository.RegisterIdentity(ctx, input.Challenge, a)
	if errors.Is(err, faults.ErrConflict) {
		return Identity{}, failed("registration_conflict", "Username or mobile number is already registered", err)
	}
	return created, err
}

func (s *Service) RequestRecovery(ctx context.Context, challenge, mobile string) (IdentityChallenge, error) {
	if err := s.repository.ConsumeIdentityChallenge(ctx, challenge, "recovery-request"); err != nil {
		return IdentityChallenge{}, failed("recovery_request_expired", "Form expired; start again", err)
	}
	phone, err := NormalizeMobile(mobile)
	if err != nil {
		return IdentityChallenge{}, err
	}
	a, err := s.repository.IdentityByMobile(ctx, phone)
	if err != nil {
		return IdentityChallenge{}, failed("recovery_user_missing", "No local account uses that mobile number", err)
	}
	c, err := s.Challenge(ctx, "recovery", a.Username, phone)
	if err != nil {
		return c, failed("unavailable", "Try again later", err)
	}
	return c, nil
}

func (s *Service) Recover(ctx context.Context, challenge, code, password string) (Identity, error) {
	c, err := s.repository.IdentityChallenge(ctx, challenge, "recovery")
	if err != nil {
		return Identity{}, failed("recovery_expired", "Reset expired; start again", err)
	}
	if code != DevelopmentOTP {
		_ = s.repository.FailIdentityChallenge(ctx, c.Token)
		return Identity{}, failed("code", "Incorrect development code", nil)
	}

	credentials, err := PasswordIdentity(c.Username, password)
	if err != nil {
		return Identity{}, err
	}

	current, err := s.repository.Identity(ctx, c.Username)
	if err != nil {
		return Identity{}, failed("identity_unavailable", "Account could not be loaded; try again", err)
	}

	if err = s.repository.ReplaceIdentityPassword(ctx, c.Token, "recovery", c.Username, credentials); err != nil {
		return Identity{}, failed("recovery_changed", "Reset expired or the mobile number changed; start again", err)
	}
	return WithPassword(current, credentials), nil
}

func (s *Service) BeginPasswordChange(ctx context.Context, current Identity, csrf, password string) (IdentityChallenge, error) {
	if !VerifyPassword(current, password) {
		return IdentityChallenge{}, failed("password_incorrect", "Current password is incorrect", nil)
	}
	c, err := s.Challenge(ctx, "password:"+csrf, current.Username, "")
	if err != nil {
		return c, failed("unavailable", "Try again later", err)
	}
	return c, nil
}

func (s *Service) ChangePassword(ctx context.Context, current Identity, csrf, challenge, password string) (Identity, error) {
	credentials, err := PasswordIdentity(current.Username, password)
	if err != nil {
		return Identity{}, err
	}

	if err = s.repository.ReplaceIdentityPassword(ctx, challenge, "password:"+csrf, current.Username, credentials); err != nil {
		return Identity{}, failed("password_expired", "Password form expired; enter your current password again", err)
	}
	return WithPassword(current, credentials), nil
}

func (s *Service) UpdateProfile(ctx context.Context, current Identity, field, value string) (Identity, error) {
	if err := s.repository.UpdateIdentityField(ctx, current.Username, field, value); err != nil {
		return Identity{}, err
	}
	switch field {
	case "display":
		current.Display = value
	case "status":
		current.Status = value
	case "bio":
		current.Bio = value
	case "country":
		current.Country = value
	}
	return current, nil
}

func (s *Service) ChangeMobile(ctx context.Context, current Identity, mobile string, activated bool) (Identity, error) {
	phone, err := NormalizeMobile(mobile)
	if err != nil {
		return Identity{}, err
	}
	if err = s.repository.SetIdentityMobile(ctx, current.Username, phone, activated); err != nil {
		return Identity{}, failed("mobile_conflict", "This mobile number is already registered", err)
	}
	current.Mobile, current.Activated = phone, activated
	return current, nil
}

func (s *Service) Activate(ctx context.Context, current Identity) (Identity, error) {
	if err := s.repository.ActivateIdentity(ctx, current.Username); err != nil {
		return Identity{}, err
	}
	current.Activated = true
	return current, nil
}

func (s *Service) Preference(ctx context.Context, owner, name string) (string, error) {
	return s.repository.IdentityPreference(ctx, owner, name)
}

func (s *Service) SetPreference(ctx context.Context, owner, name, value string) error {
	return s.repository.SetIdentityPreference(ctx, owner, name, value)
}

// Profile is the non-credential account information exposed to feature views.
type Profile struct {
	ID                                              int32
	Username, Display, Status, Mobile, Bio, Country string
	Activated                                       bool
}

func PublicProfile(a Identity) Profile {
	return Profile{a.ID, a.Username, a.Display, a.Status, a.Mobile, a.Bio, a.Country, a.Activated}
}

func (s *Service) Profile(ctx context.Context, name string) (Profile, error) {
	a, err := s.repository.Identity(ctx, name)
	return PublicProfile(a), err
}

func (s *Service) Profiles(ctx context.Context) ([]Profile, error) {
	identities, err := s.repository.Identities(ctx)
	if err != nil {
		return nil, err
	}
	profiles := make([]Profile, 0, len(identities))
	for _, a := range identities {
		profiles = append(profiles, PublicProfile(a))
	}
	return profiles, nil
}

func (s *Service) SetLanguage(ctx context.Context, owner, code string) error {
	if code != "en-US" {
		return faults.ErrInvalid
	}
	return s.repository.SetLanguage(ctx, owner, code)
}

func (s *Service) Language(ctx context.Context, owner string) (string, error) {
	return s.repository.Language(ctx, owner)
}
