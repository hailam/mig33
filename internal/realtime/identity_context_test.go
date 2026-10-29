package realtime

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/adapters/legacyweb"
	"github.com/hailam/mig33/internal/protocol"
)

// A successful storage call can complete at the same moment its caller leaves.
// Keep the real adapter underneath so login also verifies its committed roster.
type identityCommitCancellationRepository struct {
	Repository
	cancel        context.CancelFunc
	canceledReads atomic.Int64
}

func (r *identityCommitCancellationRepository) Identity(ctx context.Context, name string) (accounts.Identity, error) {
	if err := ctx.Err(); err != nil {
		r.canceledReads.Add(1)
		return accounts.Identity{}, err
	}
	return r.Repository.Identity(ctx, name)
}

func (r *identityCommitCancellationRepository) ReplaceIdentityPassword(ctx context.Context, token, purpose, owner string, credentials accounts.Identity) error {
	err := r.Repository.ReplaceIdentityPassword(ctx, token, purpose, owner, credentials)
	if err == nil {
		r.cancel()
	}
	return err
}

func (r *identityCommitCancellationRepository) RegisterIdentity(ctx context.Context, token string, identity accounts.Identity) (accounts.Identity, error) {
	created, err := r.Repository.RegisterIdentity(ctx, token, identity)
	if err == nil {
		r.cancel()
	}
	return created, err
}

func (r *identityCommitCancellationRepository) UpdateIdentityField(ctx context.Context, owner, field, value string) error {
	err := r.Repository.UpdateIdentityField(ctx, owner, field, value)
	if err == nil {
		r.cancel()
	}
	return err
}

func (r *identityCommitCancellationRepository) SetIdentityMobile(ctx context.Context, owner, mobile string, activated bool) error {
	err := r.Repository.SetIdentityMobile(ctx, owner, mobile, activated)
	if err == nil {
		r.cancel()
	}
	return err
}

func (r *identityCommitCancellationRepository) ActivateIdentity(ctx context.Context, owner string) error {
	err := r.Repository.ActivateIdentity(ctx, owner)
	if err == nil {
		r.cancel()
	}
	return err
}

func cancelAfterIdentityCommit(t *testing.T, s *Application) (context.Context, *identityCommitCancellationRepository) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	s.mu.Lock()
	repository := &identityCommitCancellationRepository{Repository: s.store, cancel: cancel}
	s.useStore(repository)
	s.mu.Unlock()
	return ctx, repository
}

func TestCommittedPasswordRemainsUsableAfterCallerCancellation(t *testing.T) {
	for _, operation := range []string{"change", "recovery"} {
		t.Run(operation, func(t *testing.T) {
			s := newTestServer(t)
			sid, _ := loginIdentity(t, s, "alice", "alice-demo")
			other, _ := loginIdentity(t, s, "alice", "alice-demo")
			s.mu.Lock()
			original := s.accounts["alice"]
			before := original.identity
			csrf := s.sessions[sid].csrf
			s.mu.Unlock()
			purpose := "password:" + csrf
			if operation == "recovery" {
				purpose = "recovery"
			}
			challenge, err := s.identityChallenge(t.Context(), purpose, "alice", before.Mobile)
			if err != nil {
				t.Fatal(err)
			}
			password := "New & mixed Password"
			path := s.identityURL("settings", "change_password") + "&stage=new"
			form := url.Values{"csrf": {csrf}, "challenge": {challenge.Token}, "password": {password}}
			pageSID := sid
			if operation == "recovery" {
				path = s.identityURL("account", "recover") + "&stage=complete"
				form = url.Values{"csrf": {challenge.Token}, "code": {DevelopmentOTP}, "password": {password}}
				pageSID = ""
			}
			u, _ := url.Parse(path)
			ctx, repository := cancelAfterIdentityCommit(t, s)
			result, err := s.FeaturePage(ctx, pageSID, legacyweb.Request{Method: "POST", URL: u, Form: form})
			if err != nil || result.Status != 200 || !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatalf("committed password result: status=%d error=%v context=%v", result.Status, err, ctx.Err())
			}
			if repository.canceledReads.Load() != 0 {
				t.Fatal("password completion attempted a canceled identity refresh")
			}
			s.mu.Lock()
			current := s.accounts["alice"]
			after := current.identity
			otherPresent := s.sessions[other] != nil
			currentPresent := s.sessions[sid] != nil
			s.mu.Unlock()
			if current != original || &current.deliveryGate != &original.deliveryGate {
				t.Fatal("password commit replaced the account and its delivery gate")
			}
			if after.ID != before.ID ||
				after.Display != before.Display ||
				after.Status != before.Status ||
				after.Mobile != before.Mobile ||
				after.Activated != before.Activated ||
				after.Bio != before.Bio ||
				after.Country != before.Country {
				t.Fatal("password commit overwrote unrelated profile fields")
			}
			if verifyPassword(after, "alice-demo") || !verifyPassword(after, password) {
				t.Fatal("cached browser password verifier differs from the committed password")
			}
			if otherPresent || currentPresent != (operation == "change") {
				t.Fatal("password completion did not invalidate the intended sessions")
			}
			replies, challengeSID := s.Handle(t.Context(), "", protocol.New(200, 90).SetText(5, "alice"))
			loginChallenge, _ := replies[0].Text(1)
			replies, _ = s.Handle(t.Context(), challengeSID, protocol.New(202, 91).SetInt(1, protocol.JavaHash(loginChallenge+"alice-demo")))
			if len(replies) != 1 || replies[0].Opcode != 0 {
				t.Fatal("old native credentials still authenticate after committed password change")
			}
			loginIdentity(t, s, "alice", password)
		})
	}
}

func TestCommittedRegistrationCanLoginAfterCallerCancellation(t *testing.T) {
	s := newTestServer(t)
	challenge, err := s.identityChallenge(t.Context(), "registration", "", "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, repository := cancelAfterIdentityCommit(t, s)
	u, _ := url.Parse(s.identityURL("registration", "home"))
	password := "New registration password"
	result, err := s.FeaturePage(ctx, "", legacyweb.Request{Method: "POST", URL: u, Form: url.Values{
		"csrf": {challenge.Token}, "username": {" Fresh_User "}, "mobile": {"9607771234"}, "password": {password}, "code": {DevelopmentOTP}, "agree": {"on"},
	}})
	if err != nil || result.Status != 201 || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("registration completion: status=%d error=%v context=%v", result.Status, err, ctx.Err())
	}
	if repository.canceledReads.Load() != 0 {
		t.Fatal("registration attempted a canceled identity refresh")
	}
	s.mu.Lock()
	created := s.accounts["fresh_user"]
	s.mu.Unlock()
	if created == nil || created.id == 0 || created.identity.Mobile != "+9607771234" || !created.identity.Activated {
		t.Fatal("committed registration was not installed in the account cache")
	}
	roster, err := repository.Contacts(t.Context(), "fresh_user")
	if err != nil || len(roster.Contacts) != 0 || len(roster.Groups) != 1 || roster.Groups[0].Name != "Friends" {
		t.Fatalf("committed registration lacks initial roster: %+v %v", roster, err)
	}
	loginIdentity(t, s, "fresh_user", password)
}

func TestCommittedIdentityFieldsUpdateCacheAfterCancellation(t *testing.T) {
	for _, operation := range []string{"display", "status", "bio", "country", "mobile", "native-mobile", "activate"} {
		t.Run(operation, func(t *testing.T) {
			s := newTestServer(t)
			sid, _ := loginIdentity(t, s, "alice", "alice-demo")
			s.mu.Lock()
			original := s.accounts["alice"]
			csrf := s.sessions[sid].csrf
			if operation == "activate" {
				original.identity.Activated = false
			}
			s.mu.Unlock()
			ctx, repository := cancelAfterIdentityCommit(t, s)
			value := "new " + operation
			switch operation {
			case "native-mobile", "activate":
				packet := protocol.New(205, 3).SetText(1, "9607771234")
				if operation == "activate" {
					packet = protocol.New(204, 3).SetText(1, DevelopmentOTP)
				}
				replies, _ := s.Handle(ctx, sid, packet)
				if len(replies) != 1 || replies[0].Opcode != 1 {
					t.Fatalf("native identity write failed: %v", replies)
				}
			default:
				path := s.identityURL("profile", "edit") + "&field=" + operation
				form := url.Values{"csrf": {csrf}, "value": {value}}
				if operation == "mobile" {
					path = s.identityURL("settings", "change_mobile")
					form = url.Values{"csrf": {csrf}, "mobile": {"9607771234"}, "code": {DevelopmentOTP}}
				}
				u, _ := url.Parse(path)
				result, err := s.FeaturePage(ctx, sid, legacyweb.Request{Method: "POST", URL: u, Form: form})
				if err != nil || result.Status != 200 {
					t.Fatalf("identity field write: status=%d error=%v", result.Status, err)
				}
			}
			if !errors.Is(ctx.Err(), context.Canceled) || repository.canceledReads.Load() != 0 {
				t.Fatal("committed identity write relied on a canceled read")
			}
			s.mu.Lock()
			current := s.accounts["alice"]
			a := current.identity
			display, status := current.display, current.status
			s.mu.Unlock()
			if current != original {
				t.Fatal("identity update replaced account ownership")
			}
			values := map[string]string{"display": a.Display, "status": a.Status, "bio": a.Bio, "country": a.Country}
			if strings.Contains(operation, "mobile") {
				if a.Mobile != "+9607771234" || a.Activated != (operation == "mobile") {
					t.Fatal("cached mobile/activation differs from committed canonical values")
				}
			} else if operation == "activate" {
				if !a.Activated {
					t.Fatal("committed activation was not cached")
				}
			} else if values[operation] != value || display != a.Display || status != a.Status {
				t.Fatal("committed profile value or native roster fields were not cached")
			}
		})
	}
}
