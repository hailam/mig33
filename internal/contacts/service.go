package contacts

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/faults"
)

// Service owns address-book mutations. Live presence and ordered notifications
// are supplied by the session owner after a mutation has committed.
type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

// Add makes a repeated add idempotent without replacing previously saved
// contact metadata. The repository still checks uniqueness transactionally.
func (s *Service) Add(ctx context.Context, owner string, input Contact) (Contact, error) {
	owner, err := NormalizeOwner(owner)
	if err != nil {
		return Contact{}, err
	}
	input, err = NormalizeContact(owner, input)
	if err != nil {
		return Contact{}, err
	}

	contact, err := s.repository.AddContact(ctx, owner, input)
	if errors.Is(err, faults.ErrConflict) {
		return s.repository.ContactByIdentity(ctx, owner, input)
	}
	return contact, err
}

func (s *Service) Update(ctx context.Context, owner string, input Contact) (Contact, error) {
	owner, err := NormalizeOwner(owner)
	if err != nil {
		return Contact{}, err
	}
	input, err = NormalizeContact(owner, input)
	if err != nil {
		return Contact{}, err
	}

	return s.repository.UpdateContact(ctx, owner, input)
}

func (s *Service) Delete(ctx context.Context, owner string, id int32) error {
	owner, err := NormalizeOwner(owner)
	if err != nil || id <= 0 {
		return faults.ErrInvalid
	}

	return s.repository.DeleteContact(ctx, owner, id)
}

func (s *Service) Move(ctx context.Context, owner string, id, groupID int32) (Contact, error) {
	owner, err := NormalizeOwner(owner)
	if err != nil || id <= 0 || (groupID <= 0 && groupID != -1) {
		return Contact{}, faults.ErrInvalid
	}

	contact, err := s.repository.Contact(ctx, owner, id)
	if err != nil {
		return Contact{}, err
	}

	contact.GroupID = groupID
	return s.repository.UpdateContact(ctx, owner, contact)
}

func (s *Service) AddGroup(ctx context.Context, owner, name string) (ContactGroup, error) {
	owner, err := NormalizeOwner(owner)
	if err != nil || !ValidGroupName(name) {
		return ContactGroup{}, faults.ErrInvalid
	}

	return s.repository.AddContactGroup(ctx, owner, name)
}

func (s *Service) RenameGroup(ctx context.Context, owner string, id int32, name string) error {
	owner, err := NormalizeOwner(owner)
	if err != nil || id <= 0 || !ValidGroupName(name) {
		return faults.ErrInvalid
	}

	return s.repository.RenameContactGroup(ctx, owner, id, name)
}

func (s *Service) DeleteGroup(ctx context.Context, owner string, id int32) error {
	owner, err := NormalizeOwner(owner)
	if err != nil || id <= 0 {
		return faults.ErrInvalid
	}

	return s.repository.DeleteContactGroup(ctx, owner, id)
}

func NormalizeOwner(owner string) (string, error) {
	if owner == "" || len(owner) > 128 || strings.TrimSpace(owner) != owner || !utf8.ValidString(owner) || strings.ContainsRune(owner, '\x00') {
		return "", faults.ErrInvalid
	}
	return strings.ToLower(owner), nil
}

func ValidText(value string, limit int) bool {
	return len(value) <= limit && utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func ValidGroupName(name string) bool {
	return name != "" && strings.TrimSpace(name) == name && ValidText(name, 128)
}
