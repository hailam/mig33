package contacts

import "context"

func (s *Service) EnsureContacts(ctx context.Context, owner string, seeds []Contact) error {
	return s.repository.EnsureContacts(ctx, owner, seeds)
}

func (s *Service) Contacts(ctx context.Context, owner string) (ContactRoster, error) {
	return s.repository.Contacts(ctx, owner)
}

func (s *Service) ContactChanges(ctx context.Context, owner string, after int64) (ContactRoster, error) {
	return s.repository.ContactChanges(ctx, owner, after)
}

func (s *Service) Contact(ctx context.Context, owner string, id int32) (Contact, error) {
	return s.repository.Contact(ctx, owner, id)
}

func (s *Service) ContactByUsername(ctx context.Context, owner, target string) (Contact, error) {
	return s.repository.ContactByUsername(ctx, owner, target)
}

func (s *Service) ContactByIdentity(ctx context.Context, owner string, contact Contact) (Contact, error) {
	return s.repository.ContactByIdentity(ctx, owner, contact)
}

func (s *Service) IsBlocked(ctx context.Context, owner, target string) (bool, error) {
	return s.repository.IsBlocked(ctx, owner, target)
}

func (s *Service) SetBlocked(ctx context.Context, owner, target string, blocked bool) error {
	return s.repository.SetBlocked(ctx, owner, target, blocked)
}

func (s *Service) BlockedUsers(ctx context.Context, owner string) ([]string, error) {
	return s.repository.BlockedUsers(ctx, owner)
}

func (s *Service) RemoveIMLabels(ctx context.Context, owner, service string) error {
	return s.repository.RemoveIMLabels(ctx, owner, service)
}
