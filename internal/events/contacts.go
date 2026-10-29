package events

import "github.com/hailam/mig33/internal/contacts"

type Contact struct {
	Contact  contacts.Contact
	Presence byte
	Status   *string
	Avatar   string
}

type (
	ContactGroup    struct{ Group contacts.ContactGroup }
	ContactRemoved  struct{ ID int32 }
	ContactPresence struct {
		ID       int32
		Presence byte
	}
)

type ContactStatus struct {
	ID     int32
	Status string
}

type ContactCheckpoint struct {
	Revision   int32
	Checkpoint int64
}

type ContactsComplete struct{}

func IsContactSnapshotEvent(event *Event) bool {
	switch event.Data.(type) {
	case ContactGroup, Contact, ContactPresence, ContactRemoved, ContactCheckpoint, Avatar, ContactStatus:
		return true
	}
	return false
}
