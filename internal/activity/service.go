package activity

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"sort"
	"strings"
	"sync"

	"github.com/hailam/mig33/internal/faults"
)

type Privacy interface {
	Exists(context.Context, string) (bool, error)
	CanContact(context.Context, string, string) (bool, error)
	CanViewProfile(context.Context, string, string) (bool, error)
	CanViewActivity(context.Context, string, string) (bool, error)
	BlocksIncoming(context.Context, string, string) (bool, error)
}

type Preferences interface {
	IdentityPreference(context.Context, string, string) (string, error)
}

type Service struct {
	repo        Repository
	privacy     Privacy
	preferences Preferences
	// Serializes local random-result creation with nonce replay. The previous
	// coordinated application provided this ordering with its shared mutex.
	gameMu sync.Mutex
}

func New(repo Repository, privacy Privacy, preferences Preferences) *Service {
	return &Service{repo: repo, privacy: privacy, preferences: preferences}
}

func (s *Service) Publish(ctx context.Context, owner, kind, body, nonce string) (ServiceEntry, error) {
	if kind != "buzz" && kind != "blog" && kind != "checkin" {
		return ServiceEntry{}, faults.ErrInvalid
	}
	return s.repo.AddServiceEntry(ctx, ServiceEntry{Kind: kind, Owner: owner, Body: body, State: "published", Nonce: nonce})
}

func (s *Service) Delete(ctx context.Context, owner string, id int64) error {
	return s.repo.DeleteServiceEntry(ctx, owner, id)
}

func (s *Service) validateOwner(ctx context.Context, viewer, owner string) error {
	if owner == "" {
		return nil
	}
	exists, err := s.privacy.Exists(ctx, owner)
	if err != nil {
		return err
	}
	if !exists {
		return faults.ErrNotFound
	}
	allowed, err := s.privacy.CanViewProfile(ctx, viewer, owner)
	if err != nil {
		return err
	}
	if !allowed {
		return faults.ErrNotFound
	}
	return nil
}

func (s *Service) ActivityEntries(ctx context.Context, viewer, owner string) ([]ServiceEntry, error) {
	if err := s.validateOwner(ctx, viewer, owner); err != nil {
		return nil, err
	}

	entries, err := s.repo.ActivityEntries(ctx, owner)
	if err != nil {
		return nil, err
	}
	return s.visible(ctx, viewer, entries, true)
}

func (s *Service) Entries(ctx context.Context, viewer, kind, owner string) ([]ServiceEntry, error) {
	if kind != "buzz" && kind != "blog" && kind != "checkin" && kind != "game" {
		return nil, faults.ErrInvalid
	}
	if kind == "game" && owner != viewer {
		return nil, faults.ErrForbidden
	}
	if err := s.validateOwner(ctx, viewer, owner); err != nil {
		return nil, err
	}

	entries, err := s.repo.ServiceEntries(ctx, kind, owner, "")
	if err != nil {
		return nil, err
	}
	return s.visible(ctx, viewer, entries, kind == "buzz" || kind == "checkin")
}

func (s *Service) visible(ctx context.Context, viewer string, entries []ServiceEntry, activity bool) ([]ServiceEntry, error) {
	visible := make([]ServiceEntry, 0, len(entries))
	for _, entry := range entries {
		allowed, err := s.privacy.CanViewProfile(ctx, viewer, entry.Owner)
		if err != nil {
			return nil, err
		}
		if !allowed {
			continue
		}
		if activity {
			allowed, err = s.privacy.CanViewActivity(ctx, viewer, entry.Owner)
			if err != nil {
				return nil, err
			}
			if !allowed {
				continue
			}
		}
		visible = append(visible, entry)
	}
	return visible, nil
}

type Ranking struct {
	Username string
	Games    int
}

func (s *Service) Leaderboard(ctx context.Context, viewer string) ([]Ranking, error) {
	entries, err := s.repo.ServiceEntries(ctx, "game", "", "")
	if err != nil {
		return nil, err
	}

	counts := map[string]int{}
	for _, entry := range entries {
		allowed, err := s.privacy.CanViewActivity(ctx, viewer, entry.Owner)
		if err != nil {
			return nil, err
		}
		if allowed {
			counts[entry.Owner]++
		}
	}

	ranking := make([]Ranking, 0, len(counts))
	for username, games := range counts {
		ranking = append(ranking, Ranking{Username: username, Games: games})
	}
	sort.Slice(ranking, func(i, j int) bool {
		if ranking[i].Games == ranking[j].Games {
			return ranking[i].Username < ranking[j].Username
		}
		return ranking[i].Games > ranking[j].Games
	})
	return ranking, nil
}

func (s *Service) PlayGame(ctx context.Context, owner, target, nonce string) (ServiceEntry, error) {
	if target != "roll" && target != "8ball" {
		return ServiceEntry{}, faults.ErrInvalid
	}

	s.gameMu.Lock()
	defer s.gameMu.Unlock()
	if err := ctx.Err(); err != nil {
		return ServiceEntry{}, err
	}

	previous, err := s.repo.ServiceEntryByNonce(ctx, owner, nonce)
	if err != nil && !errors.Is(err, faults.ErrNotFound) {
		return ServiceEntry{}, err
	}
	if err == nil {
		if previous.Kind != "game" || previous.Target != target {
			return ServiceEntry{}, faults.ErrConflict
		}
		if previous.Body != "" {
			return previous, nil
		}
	}

	n, err := rand.Int(rand.Reader, big.NewInt(100))
	if err != nil {
		return ServiceEntry{}, err
	}
	result := fmt.Sprintf("%s rolled %d (1 to 100).", owner, n.Int64()+1)
	if target == "8ball" {
		answers := []string{"Yes.", "No.", "Ask again later.", "It looks promising."}
		result = owner + ": " + answers[n.Int64()%4]
	}
	return s.repo.AddServiceEntry(ctx, ServiceEntry{Kind: "game", Owner: owner, Target: target, Body: result, State: "completed", Nonce: nonce})
}

type ProviderCommand struct{ Actor, Kind, Target, Body, Nonce string }

// RecordTelephone records already normalized phone-addressed callbacks/invites.
// Numeric addresses are never interpreted as a local account name.
func (s *Service) RecordTelephone(ctx context.Context, command ProviderCommand) (ServiceEntry, error) {
	if command.Kind != "call" && command.Kind != "invite" && command.Kind != "sms" {
		return ServiceEntry{}, faults.ErrInvalid
	}
	digits := strings.TrimPrefix(command.Target, "+")
	if len(digits) < 5 || len(digits) > 20 {
		return ServiceEntry{}, faults.ErrInvalid
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return ServiceEntry{}, faults.ErrInvalid
		}
	}
	return s.repo.AddServiceEntry(ctx, ServiceEntry{
		Kind:   command.Kind,
		Owner:  command.Actor,
		Target: command.Target,
		Body:   command.Body,
		State:  "pending_provider",
		Nonce:  command.Nonce,
	})
}

func (s *Service) ValidateProvider(ctx context.Context, kind, target string) error {
	if target == "" || len(target) > 320 {
		return faults.ErrInvalid
	}
	switch kind {
	case "mail", "sms", "call":
		exists, err := s.privacy.Exists(ctx, strings.ToLower(target))
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
		if kind == "mail" {
			address, err := mail.ParseAddress(target)
			if err != nil || address.Address != target {
				return faults.ErrInvalid
			}
			return nil
		}
		digits := strings.TrimPrefix(target, "+")
		if len(digits) < 5 || len(digits) > 20 {
			return faults.ErrInvalid
		}
		for _, c := range digits {
			if c < '0' || c > '9' {
				return faults.ErrInvalid
			}
		}
	case "im":
		for _, c := range target {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("@._+-", c)) {
				return faults.ErrInvalid
			}
		}
	case "support":
		if target != "support" {
			return faults.ErrInvalid
		}
	case "invite":
		digits := strings.TrimPrefix(target, "+")
		if len(digits) < 5 || len(digits) > 20 {
			return faults.ErrInvalid
		}
		for _, c := range digits {
			if c < '0' || c > '9' {
				return faults.ErrInvalid
			}
		}
	default:
		return faults.ErrInvalid
	}
	return nil
}

func (s *Service) RecordProvider(ctx context.Context, command ProviderCommand) (ServiceEntry, error) {
	state := "pending_provider"
	if command.Kind == "support" {
		command.Target, state = "support", "open"
	}
	if command.Kind == "im" {
		state = "disconnected"
	}
	if command.Kind == "mail" || command.Kind == "sms" || command.Kind == "call" {
		target := strings.ToLower(command.Target)
		exists, err := s.privacy.Exists(ctx, target)
		if err != nil {
			return ServiceEntry{}, err
		}
		if exists {
			command.Target = target
			allowed, err := s.privacy.CanContact(ctx, command.Actor, target)
			if err != nil {
				return ServiceEntry{}, err
			}
			if !allowed {
				return ServiceEntry{}, faults.ErrForbidden
			}
			blocked, err := s.privacy.BlocksIncoming(ctx, target, command.Actor)
			if err != nil {
				return ServiceEntry{}, err
			}
			if blocked {
				return ServiceEntry{}, faults.ErrForbidden
			}
			if command.Kind == "call" {
				allowed, err := s.preferences.IdentityPreference(ctx, target, "call_by_name")
				if err != nil {
					return ServiceEntry{}, err
				}
				if allowed != "on" {
					return ServiceEntry{}, faults.ErrForbidden
				}
			}
			if command.Kind == "mail" {
				state = "local_delivered"
			}
		}
	}
	if err := s.ValidateProvider(ctx, command.Kind, command.Target); err != nil {
		return ServiceEntry{}, err
	}
	return s.repo.AddServiceEntry(ctx, ServiceEntry{
		Kind:   command.Kind,
		Owner:  command.Actor,
		Target: command.Target,
		Body:   command.Body,
		State:  state,
		Nonce:  command.Nonce,
	})
}

func (s *Service) ProviderEntries(ctx context.Context, owner, kind string) ([]ServiceEntry, error) {
	switch kind {
	case "mail", "sms", "call", "invite", "support", "im":
	default:
		return nil, faults.ErrInvalid
	}
	return s.repo.ServiceEntries(ctx, kind, owner, "")
}

func (s *Service) Inbox(ctx context.Context, owner string) ([]ServiceEntry, error) {
	return s.repo.ServiceEntries(ctx, "mail", "", owner)
}

// ActivityForDelivery rechecks existence and privacy immediately before a live
// notification is handed to a transport. Deletion and privacy edits invalidate
// previously reserved notifications.
func (s *Service) ActivityForDelivery(ctx context.Context, viewer string, id int64) (ServiceEntry, error) {
	entry, err := s.repo.ServiceEntryByID(ctx, id)
	if err != nil {
		return ServiceEntry{}, err
	}
	if entry.Kind != "buzz" && entry.Kind != "checkin" {
		return ServiceEntry{}, faults.ErrNotFound
	}
	allowed, err := s.privacy.CanViewActivity(ctx, viewer, entry.Owner)
	if err != nil {
		return ServiceEntry{}, err
	}
	if !allowed {
		return ServiceEntry{}, faults.ErrForbidden
	}
	return entry, nil
}
