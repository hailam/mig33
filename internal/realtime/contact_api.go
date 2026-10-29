package realtime

import (
	"context"

	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/faults"
)

func (s *Coordinator) ContactsSnapshot(ctx context.Context, sid string) (contacts.ContactsSnapshot, error) {
	var result contacts.ContactsSnapshot
	if err := ctx.Err(); err != nil {
		return result, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireSession(ctx, sid)
	if err != nil {
		return result, err
	}

	roster, err := s.contactsService.Contacts(ctx, x.username)
	if err != nil {
		return result, faults.WrapError(faults.Unavailable, "Contacts unavailable", err)
	}

	result.Revision, result.Checkpoint = int64(roster.Revision), roster.Checkpoint
	result.Contacts = make([]contacts.ContactPresence, 0, len(roster.Contacts))
	result.Groups = make([]contacts.ContactGroupView, 0, len(roster.Groups))
	for _, c := range roster.Contacts {
		entry := contacts.ContactPresence{
			ID:          c.ID,
			GroupID:     c.GroupID,
			Kind:        c.Kind,
			Username:    c.Username,
			Display:     c.Display,
			Mobile:      c.Mobile,
			ShareMobile: c.ShareMobile,
			Presence:    99,
		}
		if a := s.accounts[c.Username]; a != nil {
			entry.Presence = s.presence(c.Username)
			entry.Status = a.status
		}
		for _, e := range c.External {
			entry.External = append(entry.External, contacts.ExternalPresence{Service: e.Service, Username: e.Username, Presence: 99})
		}
		result.Contacts = append(result.Contacts, entry)
	}
	for _, g := range roster.Groups {
		result.Groups = append(result.Groups, contacts.ContactGroupView{ID: g.ID, Name: g.Name})
	}
	return result, nil
}
