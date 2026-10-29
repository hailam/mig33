package contacts

import (
	"context"
)

// Repository is the persistence capability required by contacts.
// Implementations own transactions and must not expose driver or generated query types.
type Repository interface {
	SetBlocked(ctx context.Context, owner, target string, blocked bool) error
	IsBlocked(ctx context.Context, owner, target string) (bool, error)
	BlockedUsers(ctx context.Context, owner string) ([]string, error)
	EnsureContacts(ctx context.Context, owner string, seeds []Contact) error
	Contacts(ctx context.Context, owner string) (ContactRoster, error)
	ContactChanges(ctx context.Context, owner string, after int64) (ContactRoster, error)
	Contact(ctx context.Context, owner string, id int32) (Contact, error)
	ContactByUsername(ctx context.Context, owner, target string) (Contact, error)
	AddContact(ctx context.Context, owner string, contact Contact) (Contact, error)
	UpdateContact(ctx context.Context, owner string, contact Contact) (Contact, error)
	DeleteContact(ctx context.Context, owner string, id int32) error
	AddContactGroup(ctx context.Context, owner, name string) (ContactGroup, error)
	RenameContactGroup(ctx context.Context, owner string, id int32, name string) error
	DeleteContactGroup(ctx context.Context, owner string, id int32) error
	ContactByIdentity(ctx context.Context, owner string, c Contact) (Contact, error)
	RemoveIMLabels(ctx context.Context, owner, service string) error
}
