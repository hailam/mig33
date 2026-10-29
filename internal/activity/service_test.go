package activity

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/hailam/mig33/internal/faults"
)

type entryStore struct {
	Repository
	entry  ServiceEntry
	writes int
}

func (r *entryStore) ServiceEntryByNonce(context.Context, string, string) (ServiceEntry, error) {
	if r.entry.ID == 0 {
		return ServiceEntry{}, faults.ErrNotFound
	}
	return r.entry, nil
}

func (r *entryStore) AddServiceEntry(_ context.Context, entry ServiceEntry) (ServiceEntry, error) {
	r.writes++
	entry.ID = int64(r.writes)
	r.entry = entry
	return entry, nil
}

type activityPrivacy struct{ local, allowed, blocked bool }

func (p activityPrivacy) Exists(context.Context, string) (bool, error) { return p.local, nil }
func (p activityPrivacy) CanContact(context.Context, string, string) (bool, error) {
	return p.allowed, nil
}

func (p activityPrivacy) CanViewProfile(context.Context, string, string) (bool, error) {
	return p.allowed, nil
}

func (p activityPrivacy) CanViewActivity(context.Context, string, string) (bool, error) {
	return p.allowed, nil
}

func (p activityPrivacy) BlocksIncoming(context.Context, string, string) (bool, error) {
	return p.blocked, nil
}

type activityPreferences string

func (p activityPreferences) IdentityPreference(context.Context, string, string) (string, error) {
	return string(p), nil
}

func TestConcurrentGameRetriesReuseOneResult(t *testing.T) {
	repo := &entryStore{}
	service := New(repo, activityPrivacy{}, activityPreferences(""))
	results := make(chan ServiceEntry, 12)
	errs := make(chan error, 12)
	var work sync.WaitGroup
	for range 12 {
		work.Go(func() {
			result, err := service.PlayGame(context.Background(), "alice", "roll", "same-nonce")
			results <- result
			errs <- err
		})
	}
	work.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for result := range results {
		if result.ID != repo.entry.ID || result.Body != repo.entry.Body {
			t.Fatalf("replay redrew result: %+v != %+v", result, repo.entry)
		}
	}
	if repo.writes != 1 {
		t.Fatalf("writes=%d", repo.writes)
	}
	if _, err := service.PlayGame(context.Background(), "alice", "8ball", "same-nonce"); !errors.Is(err, faults.ErrConflict) {
		t.Fatalf("cross-game nonce reuse: %v", err)
	}
}

func TestProviderStateDistinguishesLocalDeliveryAndRecordedRequests(t *testing.T) {
	for _, tc := range []struct {
		name, kind, target, state string
		local                     bool
	}{
		{"local mail", "mail", "BOB", "local_delivered", true},
		{"external mail", "mail", "bob@example.test", "pending_provider", false},
		{"SMS", "sms", "+9607777777", "pending_provider", false},
		{"support", "support", "ignored", "open", false},
		{"IM", "im", "bob@example.test", "disconnected", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &entryStore{}
			result, err := New(repo, activityPrivacy{local: tc.local, allowed: true}, activityPreferences("on")).RecordProvider(context.Background(), ProviderCommand{Actor: "alice", Kind: tc.kind, Target: tc.target, Body: "hello"})
			if err != nil || result.State != tc.state {
				t.Fatalf("provider state: %+v %v", result, err)
			}
		})
	}
}

func TestBlockedLocalMailNeverRecorded(t *testing.T) {
	repo := &entryStore{}
	_, err := New(repo, activityPrivacy{local: true, allowed: true, blocked: true}, activityPreferences("on")).RecordProvider(context.Background(), ProviderCommand{Actor: "alice", Kind: "mail", Target: "bob", Body: "hello"})
	if !errors.Is(err, faults.ErrForbidden) || repo.writes != 0 {
		t.Fatalf("blocked mail: %v writes=%d", err, repo.writes)
	}
}
