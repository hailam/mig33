package realtime

import (
	"context"
	"sort"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
)

// cacheIdentity installs a committed value while retaining its delivery gate.
func (s *Coordinator) cacheIdentity(a accounts.Identity) *account {
	current := s.accounts[a.Username]
	if current == nil {
		current = &account{}
		s.accounts[a.Username] = current
	}

	current.id = a.ID
	current.name = a.Username
	current.display = a.Display
	current.status = a.Status
	current.identity = a
	return current
}

func (s *Coordinator) accountNames() []string {
	names := make([]string, 0, len(s.accounts))
	for name := range s.accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s *Coordinator) identityPublicCommand(ctx context.Context, p *commands.Request) ([]*events.Event, bool) {
	switch input := p.Input.(type) {
	case commands.Captcha:
		challenge, err := s.accountService.Challenge(ctx, "registration", "", "")
		if err != nil {
			return []*events.Event{failure(p, "Registration is busy; try again later")}, true
		}
		return []*events.Event{events.New(p.Correlation, events.Captcha{Challenge: challenge.Token, Image: accounts.CaptchaImage()})}, true
	case commands.Register:
		created, err := s.accountService.Register(ctx, accounts.Registration{
			Username:  input.Username.Value,
			Password:  input.Password.Value,
			Mobile:    input.Mobile.Value,
			Challenge: input.Challenge.Value,
			Code:      input.Code.Value,
		})
		if err != nil {
			return []*events.Event{failure(p, err.Error())}, true
		}

		s.cacheIdentity(created)
		message := "Account created. Log in, then activate with development code " + accounts.DevelopmentOTP + ". No SMS was sent."
		return []*events.Event{events.New(p.Correlation, events.Acknowledged{Message: &message})}, true
	}
	return nil, false
}

func (s *Coordinator) identityCommand(ctx context.Context, x *session, p *commands.Request) *events.Event {
	switch input := p.Input.(type) {
	case commands.Activate:
		if x.otpAttempts >= 5 {
			return failure(p, "Too many code attempts; log in again")
		}
		if input.Code.Value != accounts.DevelopmentOTP {
			x.otpAttempts++
			return failure(p, "Incorrect development code; use "+accounts.DevelopmentOTP)
		}

		a, err := s.accountService.Activate(ctx, s.accounts[x.username].identity)
		if err != nil {
			return failure(p, "Account could not be activated")
		}

		s.cacheIdentity(a)
		message := "Account activated locally. No SMS provider was used."
		return events.New(p.Correlation, events.Acknowledged{Message: &message})
	case commands.SetMobile:
		phone, err := accounts.NormalizeMobile(input.Mobile.Value)
		if err != nil {
			return failure(p, err.Error())
		}

		a, err := s.accountService.ChangeMobile(ctx, s.accounts[x.username].identity, phone, false)
		if err != nil {
			return failure(p, "Mobile number could not be changed; it may already be registered")
		}

		s.cacheIdentity(a)
		message := "Use development code " + accounts.DevelopmentOTP + ". No SMS was sent."
		return events.New(p.Correlation, events.Acknowledged{Message: &message})
	}
	return failure(p, "Unsupported identity operation")
}

func (s *Coordinator) canContact(ctx context.Context, sender, recipient string) bool {
	allowed, err := s.privacy.CanContact(ctx, sender, recipient)
	return err == nil && allowed
}

func (s *Coordinator) canViewProfile(ctx context.Context, viewer, target string) bool {
	allowed, err := s.privacy.CanViewProfile(ctx, viewer, target)
	return err == nil && allowed
}

func (s *Coordinator) invalidateIdentitySessions(ctx context.Context, username, except string) {
	for id, x := range s.sessions {
		if x.username == username && id != except {
			s.removeSession(ctx, id)
		}
	}
}
