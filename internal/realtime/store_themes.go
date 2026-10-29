package realtime

import (
	"context"

	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
)

func validStoreTheme(item string) bool {
	return item == "default" || item == "theme-blue" || item == "theme-orange"
}

// The caller holds the live-state lock through capacity checks and persistence.
// A full queue leaves both the saved selection and live themes unchanged.
func (s *Coordinator) applyStoreTheme(ctx context.Context, username, item string) error {
	if !validStoreTheme(item) {
		return faults.ErrInvalid
	}

	for _, x := range s.sessions {
		if x.authenticated && !x.webOnly && x.username == username && len(x.push) == cap(x.push) {
			return faults.ErrLimit
		}
	}
	if err := s.commerceService.SelectTheme(ctx, username, item); err != nil {
		return err
	}

	for _, x := range s.sessions {
		if x.authenticated && !x.webOnly && x.username == username {
			x.push <- events.New(0, events.Theme{ID: item})
		}
	}
	return nil
}

func (s *Coordinator) restoreStoreTheme(ctx context.Context, x *session) error {
	if x.webOnly {
		return nil
	}
	item, err := s.commerceService.SelectedTheme(ctx, x.username)
	if err != nil || item == "" {
		return err
	}
	if !validStoreTheme(item) {
		return faults.ErrInvalid
	}
	select {
	case x.push <- events.New(0, events.Theme{ID: item}):
		return nil
	default:
		return faults.ErrLimit
	}
}
