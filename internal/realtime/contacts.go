package realtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
)

// Existing demo identities keep their starter lists once. Hailam starts with
// an empty address book. Subsequent starts never recreate removed contacts.
func (s *Coordinator) initializeContacts(ctx context.Context) error {
	for _, owner := range s.accountNames() {
		var seeds []contacts.Contact
		if owner == "alice" || owner == "bob" || owner == "carol" || owner == "dave" {
			for _, target := range s.accountNames() {
				if target == owner || (target != "alice" && target != "bob" && target != "carol" && target != "dave" && target != "hailam") {
					continue
				}
				a := s.accounts[target]
				seeds = append(seeds, contacts.Contact{ID: a.id, GroupID: 1, Username: target, Display: a.display})
			}
		}
		if err := s.contactsService.EnsureContacts(ctx, owner, seeds); err != nil {
			return fmt.Errorf("initialize contacts for %s: %w", owner, err)
		}
	}
	return nil
}

func (s *Coordinator) contactPacket(ctx context.Context, c contacts.Contact, correlation int16) *events.Event {
	detail := events.Contact{Contact: c, Presence: s.presence(c.Username)}
	if a := s.accounts[c.Username]; a != nil {
		status := a.status
		detail.Status = &status
	}
	if id, err := s.mediaService.Avatar(ctx, c.Username); err == nil && id != 0 {
		detail.Avatar = fmt.Sprintf("photos/%d", id)
	}
	return events.New(correlation, detail)
}

func rosterCheckpoint(roster contacts.ContactRoster, correlation int16) *events.Event {
	return events.New(correlation, events.ContactCheckpoint{Revision: roster.Revision, Checkpoint: roster.Checkpoint})
}

func (s *Coordinator) contactSync(ctx context.Context, x *session, p *commands.Request) (replies []*events.Event) {
	input := p.Input.(commands.Contact)
	prefix := s.contactPrefix(x)
	if input.Operation == commands.ContactSnapshot {
		// Native400 clears its cache before sending the request. Older roster
		// pushes must not become extra rows in the new snapshot (absence from
		// a snapshot cannot remove an old queued contact or group header).
		kept := prefix[:0]
		for _, packet := range prefix {
			if events.IsContactSnapshotEvent(packet) {
				continue
			}
			kept = append(kept, packet)
		}
		prefix = kept
	}
	defer func() { replies = append(prefix, replies...) }()
	var roster contacts.ContactRoster
	var err error
	if input.Operation == commands.ContactChanges {
		after, ok := input.Checkpoint.Get()
		if !ok || after < 0 || x.rosterStale {
			return []*events.Event{failure(p, "Contact groups changed or checkpoint is invalid; sign in again")}
		}
		roster, err = s.contactsService.ContactChanges(ctx, x.username, after)
		// No native incoming group rename/delete exists. A changed login
		// revision normally makes the original client request full400 instead.
		if err == nil && (len(roster.Groups) != 0 || len(roster.DeletedGroups) != 0) {
			return []*events.Event{failure(p, "Contact groups changed; sign in again to refresh")}
		}
	} else {
		roster, err = s.contactsService.Contacts(ctx, x.username)
	}
	if err != nil {
		return []*events.Event{s.contactFailure(ctx, p, err)}
	}
	out := make([]*events.Event, 0, len(roster.Groups)+len(roster.Contacts)+len(roster.Deleted)+2)
	if !x.ready {
		out = append(out, s.initialNativeServices(ctx, x)...)
	}
	for _, group := range roster.Groups {
		out = append(out, events.New(p.Correlation, events.ContactGroup{Group: group}))
	}
	for _, id := range roster.Deleted {
		out = append(out, events.New(p.Correlation, events.ContactRemoved{ID: id}))
	}
	for _, c := range roster.Contacts {
		out = append(out, s.contactPacket(ctx, c, p.Correlation))
	}
	if input.Operation == commands.ContactChanges {
		// Presence/status are live account state, not address-book edits. A
		// reconnect must refresh them even when the contact checkpoint matches.
		current, err := s.contactsService.Contacts(ctx, x.username)
		if err != nil {
			return []*events.Event{s.contactFailure(ctx, p, err)}
		}
		updated := make(map[int32]bool, len(roster.Contacts))
		for _, c := range roster.Contacts {
			updated[c.ID] = true
		}
		for _, c := range current.Contacts {
			if updated[c.ID] {
				continue
			}
			out = append(out, events.New(p.Correlation, events.ContactPresence{ID: c.ID, Presence: s.presence(c.Username)}))
			if a := s.accounts[c.Username]; a != nil {
				out = append(out, events.New(p.Correlation, events.ContactStatus{ID: c.ID, Status: a.status}))
			}
		}
	}
	if !x.ready {
		if id, err := s.mediaService.Avatar(ctx, x.username); err == nil && id != 0 {
			out = append(out, events.New(0, events.Profile{Avatar: contactStringPointer(fmt.Sprintf("photos/%d", id))}))
		}
		out = append(out, colorPalette())
	}
	out = append(out, rosterCheckpoint(roster, p.Correlation), events.New(p.Correlation, events.ContactsComplete{}))
	x.rosterStale = false
	if !x.ready {
		x.ready = true
		s.restoreGroups(ctx, x)
	}
	s.flushPending(ctx, x)
	return out
}

func (s *Coordinator) contactFailure(ctx context.Context, p *commands.Request, err error) *events.Event {
	message := "Contacts could not be saved; try again"
	switch {
	case errors.Is(err, faults.ErrNotFound):
		message = "Contact or group was not found in your Friends list"
	case errors.Is(err, faults.ErrConflict):
		message = "Contact or group already exists, or the group is not empty"
	case errors.Is(err, faults.ErrInvalid):
		message = "Invalid contact or group details"
	case errors.Is(err, faults.ErrLimit):
		message = "Contact or group limit reached"
	default:
		s.logError(ctx, "contact operation: %v", err)
	}
	return failure(p, message)
}

func optionalContactGroup(field commands.Field[int32], fallback int32) (int32, bool) {
	if !field.Present {
		return fallback, true
	}
	id, ok := field.Get()
	return id, ok && (id > 0 || id == -1)
}

func optionalContactText(field commands.Field[string], current string, max int) (string, bool) {
	if !field.Present {
		return current, true
	}
	value, ok := field.Get()
	return value, ok && contacts.ValidText(value, max)
}

func (s *Coordinator) contactOperation(ctx context.Context, x *session, p *commands.Request) (replies []*events.Event) {
	input := p.Input.(commands.Contact)
	prefix := s.contactPrefix(x)
	defer func() { replies = append(prefix, replies...) }()
	var reply *events.Event
	var changed *events.Event
	structural := false
	var err error

	switch input.Operation {
	case commands.ContactAdd, commands.ContactUpdate:
		var c contacts.Contact
		if input.Operation == commands.ContactAdd {
			target, _ := input.Username.Get()
			target = strings.ToLower(strings.TrimSpace(target))
			if target != "" {
				a := s.accounts[target]
				if a == nil || target == x.username {
					return []*events.Event{failure(p, "Enter another registered mig33 username")}
				}
				c = contacts.Contact{ID: a.id, Username: target, Display: a.display, GroupID: -1, Kind: "mig33"}
			} else {
				mobile, _ := input.Mobile.Get()
				c = contacts.Contact{GroupID: -1, Kind: "phone", Mobile: mobile, Display: mobile}
				if input.ExternalRequested {
					c.Kind = "im"
				}
			}
		} else {
			id, ok := input.ID.Get()
			if !ok || id <= 0 {
				return []*events.Event{failure(p, "Contact ID is required")}
			}
			c, err = s.contactsService.Contact(ctx, x.username, id)
			if err != nil {
				return []*events.Event{s.contactFailure(ctx, p, err)}
			}
			if target, present := input.Username.Get(); present && strings.ToLower(strings.TrimSpace(target)) != c.Username {
				return []*events.Event{failure(p, "Remove and add a contact to change its username")}
			}
		}
		if err := parseContactExternal(input, &c); err != nil {
			return []*events.Event{failure(p, "Invalid external contact identities; supported service IDs are 2, 3, 4 and 6")}
		}
		var ok bool
		if c.GroupID, ok = optionalContactGroup(input.GroupID, c.GroupID); !ok {
			return []*events.Event{failure(p, "Invalid contact group")}
		}
		if c.Display, ok = optionalContactText(input.Display, c.Display, 128); !ok {
			return []*events.Event{failure(p, "Invalid display name")}
		}
		if c.Mobile, ok = optionalContactText(input.Mobile, c.Mobile, 64); !ok {
			return []*events.Event{failure(p, "Invalid mobile number")}
		}
		if input.ShareMobile.Present {
			share, ok := input.ShareMobile.Get()
			if !ok || share > 1 {
				return []*events.Event{failure(p, "Invalid share-mobile setting")}
			}
			c.ShareMobile = share == 1
		}
		if input.Operation == commands.ContactAdd {
			c, err = s.contactsService.Add(ctx, x.username, c)
		} else {
			c, err = s.contactsService.Update(ctx, x.username, c)
		}
		if err == nil {
			reply, changed = s.contactPacket(ctx, c, p.Correlation), s.contactPacket(ctx, c, 0)
		}
	case commands.ContactDelete:
		id, ok := input.ID.Get()
		if !ok || id <= 0 {
			return []*events.Event{failure(p, "Contact ID is required")}
		}
		err = s.contactsService.Delete(ctx, x.username, id)
		changed = events.New(0, events.ContactRemoved{ID: id})
	case commands.ContactGroupAdd:
		name, ok := optionalContactText(input.GroupName, "", 128)
		if !ok {
			return []*events.Event{failure(p, "Invalid group name")}
		}
		var group contacts.ContactGroup
		group, err = s.contactsService.AddGroup(ctx, x.username, name)
		if err == nil {
			reply = events.New(p.Correlation, events.ContactGroup{Group: group})
			changed = events.New(0, events.ContactGroup{Group: group})
		}
	case commands.ContactGroupDelete, commands.ContactGroupRename:
		id, ok := input.ID.Get()
		if !ok || id <= 0 {
			return []*events.Event{failure(p, "Group ID is required")}
		}
		if input.Operation == commands.ContactGroupDelete {
			err = s.contactsService.DeleteGroup(ctx, x.username, id)
		} else {
			name, ok := optionalContactText(input.GroupName, "", 128)
			if !ok {
				return []*events.Event{failure(p, "Invalid group name")}
			}
			err = s.contactsService.RenameGroup(ctx, x.username, id, name)
		}
		structural = true
	case commands.ContactMove:
		id, ok := input.ID.Get()
		if !ok || id <= 0 {
			return []*events.Event{failure(p, "Contact ID is required")}
		}
		var c contacts.Contact
		c, err = s.contactsService.Contact(ctx, x.username, id)
		if err == nil {
			if c.GroupID, ok = optionalContactGroup(input.GroupID, -1); !ok {
				return []*events.Event{failure(p, "Invalid contact group")}
			}
			c, err = s.contactsService.Update(ctx, x.username, c)
			changed = s.contactPacket(ctx, c, 0)
		}
	}
	if err != nil {
		return []*events.Event{s.contactFailure(ctx, p, err)}
	}
	if reply == nil {
		reply = events.Ack(p.Correlation)
	}
	roster, err := s.contactsService.Contacts(ctx, x.username)
	if err != nil {
		// The mutation committed; preserve its correct response and leave the
		// client's old revision so the next login requests a complete refresh.
		s.logError(ctx, "contact checkpoint: %v", err)
		return []*events.Event{reply}
	}
	s.publishContacts(x, changed, structural, roster)
	if x.rosterStale {
		return []*events.Event{reply}
	}
	return []*events.Event{reply, rosterCheckpoint(roster, 0)}
}

func parseContactExternal(input commands.Contact, c *contacts.Contact) error {
	if !input.External.Present {
		return nil
	}
	identities, valid := input.External.Get()
	if !valid || c.Kind != "im" {
		return faults.ErrInvalid
	}
	c.External = append([]contacts.ContactIM(nil), identities...)
	seen := map[byte]bool{}
	for _, identity := range c.External {
		if !contacts.ValidContactService(identity.Service) ||
			seen[identity.Service] ||
			identity.Username == "" ||
			len(identity.Username) > 320 ||
			strings.TrimSpace(identity.Username) != identity.Username ||
			!utf8.ValidString(identity.Username) {
			return faults.ErrInvalid
		}
		seen[identity.Service] = true
	}
	if c.Display == "" {
		c.Display = c.External[0].Username
	}
	return nil
}

func (s *Coordinator) publishContacts(origin *session, changed *events.Event, structural bool, roster contacts.ContactRoster) {
	for _, other := range s.sessions {
		if other == origin || other.username != origin.username || !other.authenticated || !other.ready || time.Since(other.touched) >= sessionTTL {
			continue
		}
		if structural || other.rosterStale || len(other.push)+2 > cap(other.push) {
			// There is no incoming legacy group rename/delete packet. Keep the
			// older checkpoint; the next login's revision will force full400.
			other.rosterStale = true
			continue
		}
		if changed != nil {
			other.push <- changed
		}
		other.push <- rosterCheckpoint(roster, 0)
	}
}

func (s *Coordinator) broadcastContactStatus(ctx context.Context, username string) {
	a := s.accounts[username]
	for _, other := range s.sessions {
		if !other.authenticated || !other.ready || other.username == username {
			continue
		}
		if _, err := s.contactsService.ContactByUsername(ctx, other.username, username); err != nil {
			continue
		}
		select {
		case other.push <- events.New(0, events.ContactStatus{ID: a.id, Status: a.status}):
		default:
		}
	}
}

func contactStringPointer(value string) *string { return &value }

func (s *Coordinator) broadcastPresence(ctx context.Context, username string) {
	a := s.accounts[username]
	if a == nil {
		return
	}
	event := events.New(0, events.ContactPresence{ID: a.id, Presence: s.presence(username)})
	for _, x := range s.sessions {
		if x.authenticated && !x.webOnly && x.username != username {
			if _, err := s.contactsService.ContactByUsername(ctx, x.username, username); err != nil {
				continue
			}
			select {
			case x.push <- event:
			default:
			}
		}
	}
}
