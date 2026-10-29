package accounts

import (
	"context"
	"errors"

	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/faults"
)

type PrivacyAccounts interface {
	Identity(context.Context, string) (Identity, error)
	IdentityPreference(context.Context, string, string) (string, error)
}

type PrivacyContacts interface {
	ContactByUsername(context.Context, string, string) (contacts.Contact, error)
	IsBlocked(context.Context, string, string) (bool, error)
}

// Privacy reads current persisted preferences; it owns no copy of session or
// account state. Mutation workflows still recheck durable permissions atomically.
type Privacy struct {
	accounts PrivacyAccounts
	contacts PrivacyContacts
}

func NewPrivacy(accounts PrivacyAccounts, contacts PrivacyContacts) *Privacy {
	return &Privacy{accounts: accounts, contacts: contacts}
}

func (p *Privacy) Exists(ctx context.Context, name string) (bool, error) {
	_, err := p.accounts.Identity(ctx, name)
	if errors.Is(err, faults.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (p *Privacy) CanContact(ctx context.Context, sender, recipient string) (bool, error) {
	if sender == recipient {
		return true, nil
	}
	value, err := p.accounts.IdentityPreference(ctx, recipient, "chat_privacy")
	if err != nil {
		return false, err
	}
	if value == "everyone" {
		return true, nil
	}
	if value == "nobody" {
		return false, nil
	}
	return p.hasContact(ctx, recipient, sender)
}

func (p *Privacy) CanViewProfile(ctx context.Context, viewer, owner string) (bool, error) {
	if viewer == owner {
		return true, nil
	}
	value, err := p.accounts.IdentityPreference(ctx, owner, "profile_privacy_settings")
	if err != nil {
		return false, err
	}
	if value == "everyone" {
		return true, nil
	}
	if value == "private" {
		return false, nil
	}
	return p.hasContact(ctx, owner, viewer)
}

func (p *Privacy) hasContact(ctx context.Context, owner, target string) (bool, error) {
	_, err := p.contacts.ContactByUsername(ctx, owner, target)
	if errors.Is(err, faults.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (p *Privacy) BlocksIncoming(ctx context.Context, owner, sender string) (bool, error) {
	if owner == sender {
		return false, nil
	}
	exists, err := p.Exists(ctx, sender)
	if err != nil {
		return true, err
	}
	if !exists {
		return false, nil
	}
	blocked, err := p.contacts.IsBlocked(ctx, owner, sender)
	return blocked || err != nil, err
}

func (p *Privacy) CanViewActivity(ctx context.Context, viewer, owner string) (bool, error) {
	if viewer == owner {
		return true, nil
	}
	blocked, err := p.BlocksIncoming(ctx, viewer, owner)
	if err != nil || blocked {
		return false, err
	}
	allowed, err := p.CanViewProfile(ctx, viewer, owner)
	if err != nil || !allowed {
		return false, err
	}
	value, err := p.accounts.IdentityPreference(ctx, owner, "view_update_settings")
	return err == nil && value == "on", err
}
