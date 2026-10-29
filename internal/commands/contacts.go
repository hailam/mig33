package commands

import "github.com/hailam/mig33/internal/contacts"

type ContactOperation string

const (
	ContactSnapshot    ContactOperation = "snapshot"
	ContactChanges     ContactOperation = "changes"
	ContactAdd         ContactOperation = "add"
	ContactUpdate      ContactOperation = "update"
	ContactDelete      ContactOperation = "delete"
	ContactGroupAdd    ContactOperation = "group_add"
	ContactGroupDelete ContactOperation = "group_delete"
	ContactGroupRename ContactOperation = "group_rename"
	ContactMove        ContactOperation = "move"
)

type Contact struct {
	Operation                            ContactOperation
	ID, GroupID                          Field[int32]
	Username, Display, Mobile, GroupName Field[string]
	ShareMobile                          Field[byte]
	Checkpoint                           Field[int64]
	External                             Field[[]contacts.ContactIM]
	ExternalRequested                    bool
}
