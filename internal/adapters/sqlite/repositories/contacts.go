package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
	"github.com/hailam/mig33/internal/contacts"
)

// Contact is a directed address-book entry, not a reciprocal friendship.
// IDs are supplied by the account adapter and scoped to the owner. GroupID -1
// means ungrouped. Editing a contact never changes its username identity.
type Contact = contacts.Contact

// ContactIM is an offline external identity, never a credential or a claim
// that the historical provider remains connected.
type ContactIM = contacts.ContactIM

type ContactGroup = contacts.ContactGroup

// ContactRoster is either a complete snapshot or a delta. Deltas contain final
// current rows changed after the requested checkpoint and deleted IDs which
// remain absent. Group deletions require the client adapter's refresh policy.
type ContactRoster = contacts.ContactRoster

// EnsureContacts initializes an owner's address book exactly once. Later calls
// never restore deleted contacts or groups, even when seed data changes.
func (s *Store) EnsureContacts(ctx context.Context, owner string, seeds []Contact) error {
	owner, err := username(owner)
	if err != nil {
		return queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	exists, err := queries(tx).HasContactRoster(ctx, owner)
	if err != nil {
		return queryError(ctx, err)
	}
	if exists {
		return queryError(ctx, tx.Commit())
	}
	if len(seeds) > 1000 {
		return ErrLimit
	}

	now := time.Now().UnixMilli()
	// Version 4 differs from the old fixed five-account roster revision 3.
	if err := queries(tx).CreateContactRoster(ctx, sqlc.CreateContactRosterParams{Owner: owner, Now: now}); err != nil {
		return queryError(ctx, err)
	}
	if err := queries(tx).CreateDefaultContactGroup(ctx, sqlc.CreateDefaultContactGroupParams{Owner: owner, Now: now}); err != nil {
		return queryError(ctx, err)
	}

	for _, seed := range seeds {
		seed.GroupID = 1
		seed, err = validateContact(owner, seed)
		if err != nil {
			return queryError(ctx, err)
		}
		if err := insertContact(ctx, tx, owner, seed, now); err != nil {
			return queryError(ctx, err)
		}
	}

	return queryError(ctx, tx.Commit())
}

func (s *Store) Contacts(ctx context.Context, owner string) (ContactRoster, error) {
	return s.contactSnapshot(ctx, owner, -1)
}

func (s *Store) ContactChanges(ctx context.Context, owner string, after int64) (ContactRoster, error) {
	if after < 0 {
		return ContactRoster{}, ErrInvalid
	}

	return s.contactSnapshot(ctx, owner, after)
}

func (s *Store) contactSnapshot(ctx context.Context, owner string, after int64) (ContactRoster, error) {
	owner, err := username(owner)
	if err != nil {
		return ContactRoster{}, queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ContactRoster{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	r := ContactRoster{Groups: []ContactGroup{}, Contacts: []Contact{}, Deleted: []int32{}, DeletedGroups: []int32{}}
	roster, err := queries(tx).GetContactRoster(ctx, owner)
	if err != nil {
		return r, contactError(ctx, err)
	}

	r.Revision, r.Checkpoint = int32(roster.Revision), roster.Checkpoint
	if after > r.Checkpoint {
		return r, fmt.Errorf("%w: contact checkpoint is in the future", ErrInvalid)
	}

	groups, err := queries(tx).ChangedContactGroups(ctx, sqlc.ChangedContactGroupsParams{Owner: owner, AfterID: after})
	if err != nil {
		return r, queryError(ctx, err)
	}

	for _, g := range groups {
		r.Groups = append(r.Groups, ContactGroup{ID: int32(g.ID), Name: g.Name})
	}

	contacts, err := queries(tx).ChangedContacts(ctx, sqlc.ChangedContactsParams{Owner: owner, AfterID: after})
	if err != nil {
		return r, queryError(ctx, err)
	}

	for _, row := range contacts {
		contact, err := contactModel(row)
		if err != nil {
			return r, queryError(ctx, err)
		}

		r.Contacts = append(r.Contacts, contact)
	}
	if after >= 0 {
		for _, kind := range []string{"contact", "group"} {
			ids, err := queries(tx).DeletedContacts(ctx, sqlc.DeletedContactsParams{Owner: owner, Kind: kind, AfterID: after})
			if err != nil {
				return r, queryError(ctx, err)
			}

			for _, id := range ids {
				if kind == "contact" {
					r.Deleted = append(r.Deleted, int32(id))
				} else {
					r.DeletedGroups = append(r.DeletedGroups, int32(id))
				}
			}
		}
	}

	return r, queryError(ctx, tx.Commit())
}

func (s *Store) Contact(ctx context.Context, owner string, id int32) (Contact, error) {
	owner, err := username(owner)
	if err != nil || id <= 0 {
		return Contact{}, ErrInvalid
	}

	return readContact(ctx, s.db, owner, id)
}

func (s *Store) ContactByUsername(ctx context.Context, owner, target string) (Contact, error) {
	owner, target, err := blockUsers(owner, target)
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}

	return readContactIdentity(ctx, s.db, owner, target)
}

func (s *Store) AddContact(ctx context.Context, owner string, contact Contact) (Contact, error) {
	owner, err := username(owner)
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}

	contact, err = validateContact(owner, contact)
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	if contact.Kind != "mig33" && contact.ID == 0 {
		roster, err := queries(tx).GetContactRoster(ctx, owner)
		if err != nil {
			return Contact{}, contactError(ctx, err)
		}

		next := roster.NextLocalID
		for next > 0 {
			occupied, err := queries(tx).IsLocalContactIDOccupied(ctx, sqlc.IsLocalContactIDOccupiedParams{Owner: owner, ID: next})
			if err != nil {
				return Contact{}, queryError(ctx, err)
			}
			if !occupied.Bool {
				break
			}

			next--
		}
		if next <= 0 {
			return Contact{}, ErrLimit
		}

		contact.ID = int32(next)
		if err := queries(tx).SetNextLocalContactID(ctx, sqlc.SetNextLocalContactIDParams{NextID: next - 1, Owner: owner}); err != nil {
			return Contact{}, queryError(ctx, err)
		}
	}

	exists, err := queries(tx).HasConflictingContact(ctx, sqlc.HasConflictingContactParams{Owner: owner, ID: int64(contact.ID), Username: contactStorageIdentity(contact)})
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}
	if exists {
		return Contact{}, ErrConflict
	}

	count, err := queries(tx).CountContacts(ctx, owner)
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}
	if count >= 1000 {
		return Contact{}, ErrLimit
	}
	if err := requireContactGroup(ctx, tx, owner, contact.GroupID); err != nil {
		return Contact{}, queryError(ctx, err)
	}

	now, err := advanceContacts(ctx, tx, owner)
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}
	if err := insertContact(ctx, tx, owner, contact, now); err != nil {
		return Contact{}, queryError(ctx, err)
	}
	if err := queries(tx).ClearContactDeletion(ctx, sqlc.ClearContactDeletionParams{Owner: owner, ID: int64(contact.ID)}); err != nil {
		return Contact{}, queryError(ctx, err)
	}

	return contact, queryError(ctx, tx.Commit())
}

func (s *Store) UpdateContact(ctx context.Context, owner string, contact Contact) (Contact, error) {
	owner, err := username(owner)
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}

	contact, err = validateContact(owner, contact)
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	old, err := readContact(ctx, tx, owner, contact.ID)
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}
	if old.Username != contact.Username || old.Kind != contact.Kind {
		return Contact{}, fmt.Errorf("%w: cannot change contact identity", ErrInvalid)
	}
	if err := requireContactGroup(ctx, tx, owner, contact.GroupID); err != nil {
		return Contact{}, queryError(ctx, err)
	}

	oldExternal, newExternal := old.External, contact.External
	old.External, contact.External = nil, nil
	unchanged := old.ID == contact.ID && old.GroupID == contact.GroupID &&
		old.Username == contact.Username && old.Display == contact.Display &&
		old.Mobile == contact.Mobile && old.ShareMobile == contact.ShareMobile &&
		old.Kind == contact.Kind && slices.Equal(oldExternal, newExternal)
	contact.External = newExternal
	if unchanged {
		return contact, queryError(ctx, tx.Commit())
	}

	now, err := advanceContacts(ctx, tx, owner)
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}

	duplicate, err := queries(tx).HasDuplicateContactIdentity(ctx, sqlc.HasDuplicateContactIdentityParams{Owner: owner, Username: contactStorageIdentity(contact), ID: int64(contact.ID)})
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}
	if duplicate {
		return Contact{}, ErrConflict
	}

	external, _ := json.Marshal(contact.External)
	if err := queries(tx).UpdateContact(ctx, sqlc.UpdateContactParams{
		GroupID:      nullableContactGroup(contact.GroupID),
		Username:     contactStorageIdentity(contact),
		Display:      contact.Display,
		Mobile:       contact.Mobile,
		ShareMobile:  boolInt64(contact.ShareMobile),
		ExternalJson: string(external),
		Now:          now,
		Owner:        owner,
		ID:           int64(contact.ID),
	}); err != nil {
		return Contact{}, queryError(ctx, err)
	}

	return contact, queryError(ctx, tx.Commit())
}

func (s *Store) DeleteContact(ctx context.Context, owner string, id int32) error {
	owner, err := username(owner)
	if err != nil || id <= 0 {
		return ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	n, err := queries(tx).DeleteContact(ctx, sqlc.DeleteContactParams{Owner: owner, ID: int64(id)})
	if err != nil {
		return queryError(ctx, err)
	}
	if n == 0 {
		return ErrNotFound
	}

	now, err := advanceContacts(ctx, tx, owner)
	if err != nil {
		return queryError(ctx, err)
	}
	if err := deleteContactMarker(ctx, tx, owner, "contact", id, now); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func (s *Store) AddContactGroup(ctx context.Context, owner, name string) (ContactGroup, error) {
	owner, err := username(owner)
	if err != nil || !validContactGroupName(name) {
		return ContactGroup{}, ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ContactGroup{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	roster, err := queries(tx).GetContactRoster(ctx, owner)
	if err != nil {
		return ContactGroup{}, contactError(ctx, err)
	}

	id := roster.NextGroupID
	count, err := queries(tx).CountContactGroups(ctx, owner)
	if err != nil {
		return ContactGroup{}, queryError(ctx, err)
	}
	if count >= 100 || id > math.MaxInt32 {
		return ContactGroup{}, ErrLimit
	}
	if err := uniqueContactGroupName(ctx, tx, owner, 0, name); err != nil {
		return ContactGroup{}, queryError(ctx, err)
	}

	now, err := advanceContacts(ctx, tx, owner)
	if err != nil {
		return ContactGroup{}, queryError(ctx, err)
	}
	if err := queries(tx).CreateContactGroup(ctx, sqlc.CreateContactGroupParams{Owner: owner, ID: id, Name: name, Now: now}); err != nil {
		return ContactGroup{}, queryError(ctx, err)
	}
	if err := queries(tx).AdvanceContactGroupID(ctx, owner); err != nil {
		return ContactGroup{}, queryError(ctx, err)
	}

	return ContactGroup{ID: int32(id), Name: name}, queryError(ctx, tx.Commit())
}

func (s *Store) RenameContactGroup(ctx context.Context, owner string, id int32, name string) error {
	owner, err := username(owner)
	if err != nil || id <= 0 || !validContactGroupName(name) {
		return ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	old, err := queries(tx).ContactGroupName(ctx, sqlc.ContactGroupNameParams{Owner: owner, ID: int64(id)})
	if err != nil {
		return contactError(ctx, err)
	}
	if old == name {
		return queryError(ctx, tx.Commit())
	}
	if err := uniqueContactGroupName(ctx, tx, owner, id, name); err != nil {
		return queryError(ctx, err)
	}

	now, err := advanceContacts(ctx, tx, owner)
	if err != nil {
		return queryError(ctx, err)
	}
	if err := queries(tx).RenameContactGroup(ctx, sqlc.RenameContactGroupParams{Name: name, Now: now, Owner: owner, ID: int64(id)}); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

// DeleteContactGroup rejects nonempty groups. Callers must move or remove their
// contacts explicitly before deleting a group, including the initial Friends.
func (s *Store) DeleteContactGroup(ctx context.Context, owner string, id int32) error {
	owner, err := username(owner)
	if err != nil || id <= 0 {
		return ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	if err := requireContactGroup(ctx, tx, owner, id); err != nil {
		return queryError(ctx, err)
	}

	occupied, err := queries(tx).ContactGroupOccupied(ctx, sqlc.ContactGroupOccupiedParams{Owner: owner, ID: sql.NullInt64{Int64: int64(id), Valid: true}})
	if err != nil {
		return queryError(ctx, err)
	}
	if occupied {
		return fmt.Errorf("%w: group still contains contacts", ErrConflict)
	}

	now, err := advanceContacts(ctx, tx, owner)
	if err != nil {
		return queryError(ctx, err)
	}
	if err := queries(tx).DeleteContactGroup(ctx, sqlc.DeleteContactGroupParams{Owner: owner, ID: int64(id)}); err != nil {
		return queryError(ctx, err)
	}
	if err := deleteContactMarker(ctx, tx, owner, "group", id, now); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func deleteContactMarker(ctx context.Context, tx *sql.Tx, owner, kind string, id int32, now int64) error {
	err := queries(tx).MarkContactDeleted(ctx, sqlc.MarkContactDeletedParams{Owner: owner, Kind: kind, ID: int64(id), Now: now})
	return queryError(ctx, err)
}

func advanceContacts(ctx context.Context, tx *sql.Tx, owner string) (int64, error) {
	roster, err := queries(tx).GetContactRoster(ctx, owner)
	if err != nil {
		return 0, contactError(ctx, err)
	}
	if roster.Revision >= math.MaxInt32 || roster.Checkpoint == math.MaxInt64 {
		return 0, ErrLimit
	}

	now := time.Now().UnixMilli()
	if now <= roster.Checkpoint {
		now = roster.Checkpoint + 1
	}

	err = queries(tx).AdvanceContacts(ctx, sqlc.AdvanceContactsParams{Now: now, Owner: owner})
	return now, queryError(ctx, err)
}

func requireContactGroup(ctx context.Context, tx *sql.Tx, owner string, id int32) error {
	if id == -1 {
		return nil
	}

	exists, err := queries(tx).HasContactGroup(ctx, sqlc.HasContactGroupParams{Owner: owner, ID: int64(id)})
	if err != nil {
		return queryError(ctx, err)
	}
	if !exists {
		return ErrNotFound
	}

	return nil
}

func uniqueContactGroupName(ctx context.Context, tx *sql.Tx, owner string, id int32, name string) error {
	exists, err := queries(tx).ContactGroupNameConflict(ctx, sqlc.ContactGroupNameConflictParams{Owner: owner, Name: name, ID: int64(id)})
	if err != nil {
		return queryError(ctx, err)
	}
	if exists {
		return ErrConflict
	}

	return nil
}

func validateContact(owner string, c Contact) (Contact, error) {
	return contacts.NormalizeContact(owner, c)
}

func ValidContactService(service byte) bool { return contacts.ValidContactService(service) }
func ValidContactMobile(mobile string) bool { return contacts.ValidContactMobile(mobile) }
func contactStorageIdentity(c Contact) string {
	if c.Kind == "phone" {
		return "phone:" + c.Mobile
	}
	if c.Kind == "im" {
		b, _ := json.Marshal(c.External)
		return "im:" + string(b)
	}

	return c.Username
}

func (s *Store) ContactByIdentity(ctx context.Context, owner string, c Contact) (Contact, error) {
	owner, err := username(owner)
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}

	c, err = validateContact(owner, c)
	if err != nil {
		return Contact{}, queryError(ctx, err)
	}

	return readContactIdentity(ctx, s.db, owner, contactStorageIdentity(c))
}

func validContactText(value string, limit int) bool { return contacts.ValidText(value, limit) }

func validContactGroupName(name string) bool { return contacts.ValidGroupName(name) }

func nullableContactGroup(id int32) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(id), Valid: id != -1}
}

func insertContact(ctx context.Context, tx *sql.Tx, owner string, c Contact, now int64) error {
	external, _ := json.Marshal(c.External)
	return queryError(ctx, queries(tx).CreateContact(ctx, sqlc.CreateContactParams{
		Owner:        owner,
		ID:           int64(c.ID),
		GroupID:      nullableContactGroup(c.GroupID),
		Username:     contactStorageIdentity(c),
		Display:      c.Display,
		Mobile:       c.Mobile,
		ShareMobile:  boolInt64(c.ShareMobile),
		Now:          now,
		ContactKind:  c.Kind,
		ExternalJson: string(external),
	}))
}

func contactModel(row sqlc.Contact) (Contact, error) {
	c := Contact{
		ID:          int32(row.ID),
		GroupID:     -1,
		Username:    row.Username,
		Display:     row.Display,
		Mobile:      row.Mobile,
		ShareMobile: row.ShareMobile != 0,
		Kind:        row.ContactKind,
	}
	if row.GroupID.Valid {
		c.GroupID = int32(row.GroupID.Int64)
	}

	err := json.Unmarshal([]byte(row.ExternalJson), &c.External)
	if c.Kind != "mig33" {
		c.Username = ""
	}

	return c, err
}

func readContact(ctx context.Context, db sqlc.DBTX, owner string, id int32) (Contact, error) {
	row, err := queries(db).GetContact(ctx, sqlc.GetContactParams{Owner: owner, ID: int64(id)})
	if err != nil {
		return Contact{}, contactError(ctx, err)
	}

	return contactModel(row)
}

func readContactIdentity(ctx context.Context, db sqlc.DBTX, owner, target string) (Contact, error) {
	row, err := queries(db).GetContactByIdentity(ctx, sqlc.GetContactByIdentityParams{Owner: owner, Username: target})
	if err != nil {
		return Contact{}, contactError(ctx, err)
	}

	return contactModel(row)
}
func contactError(ctx context.Context, err error) error { return queryError(ctx, err) }
