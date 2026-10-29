package realtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
)

func (s *Coordinator) deliveryGate(id string) *sync.RWMutex {
	s.mu.Lock()
	defer s.mu.Unlock()

	if x := s.sessions[id]; x != nil {
		if a := s.accounts[x.username]; a != nil {
			return &a.deliveryGate
		}
	}
	return nil
}

func (s *Coordinator) beginWrite(id string) func() {
	if gate := s.deliveryGate(id); gate != nil {
		gate.RLock()
		return gate.RUnlock
	}
	return func() {}
}

// These helpers are called with s.mu held, unless documented otherwise.
func (s *Coordinator) removeSession(ctx context.Context, id string) {
	if x := s.sessions[id]; x != nil {
		s.roomLeaveSession(ctx, x)
		for _, p := range x.inflight {
			delete(s.deliveries, p.Handle)
		}
		delete(s.sessions, id)
		if x.authenticated {
			s.broadcastPresence(ctx, x.username)
		}
	}
}

// enqueue distributes transient control notifications. User messages instead
// go through the durable outbox below, so a full queue never loses chat text.
func (s *Coordinator) enqueue(username string, p *events.Event) bool {
	queued := false
	for _, x := range s.sessions {
		if x.authenticated && !x.webOnly && x.ready && x.username == username && time.Since(x.touched) < sessionTTL {
			select {
			case x.push <- p:
				queued = true
			default:
			}
		}
	}
	return queued
}

func (s *Coordinator) restoreGroups(ctx context.Context, x *session) {
	if !x.groupRestoreLoaded {
		groups, err := s.chatService.Groups(ctx, x.username)
		if err != nil {
			s.logError(ctx, "load groups: %v", err)
			return
		}
		for _, g := range groups {
			x.pendingGroups = append(x.pendingGroups, g.ID)
		}
		x.groupRestoreLoaded = true
	}

	// The transport queue is bounded, while an account's group memberships
	// need not fit in one batch. Keep the unqueued IDs for the next poll/tick.
	for len(x.pendingGroups) > 0 && len(x.push) < cap(x.push) {
		// A different session may leave a group or change its members between
		// batches. Build the notice from current membership, not the snapshot.
		g, err := s.chatService.Group(ctx, x.pendingGroups[0], x.username)
		if err != nil && !errors.Is(err, faults.ErrNotFound) && !errors.Is(err, faults.ErrForbidden) {
			s.logError(ctx, "restore group: %v", err)
			return
		}
		if err == nil {
			x.push <- groupSystemPacket(g.ID, "Group chat: "+strings.Join(g.Members, ", "))
		}
		x.pendingGroups[0] = ""
		x.pendingGroups = x.pendingGroups[1:]
	}
	if len(x.pendingGroups) == 0 {
		x.pendingGroups = nil
	}
}

func (s *Coordinator) messagePacket(ctx context.Context, m chat.Message, viewer string) *events.Event {
	e := events.Message{Message: m, Viewer: viewer, Emoticons: catalog.ReferencedEmoticons(m.Body)}
	if !m.Action {
		if id, err := s.mediaService.Avatar(ctx, m.Sender); err == nil && id != 0 {
			e.Avatar = fmt.Sprintf("photos/%d", id)
		}
	}
	return events.New(0, e)
}

func (s *Coordinator) flushPending(ctx context.Context, x *session) {
	if !x.authenticated || x.webOnly || !x.ready || time.Since(x.touched) >= sessionTTL || len(x.push) == cap(x.push) {
		return
	}
	s.flushEmoticonCatalog(ctx, x)
	s.restoreGroups(ctx, x)
	s.flushOwnMessages(ctx, x)
	if len(x.push) == cap(x.push) {
		return
	}

	messages, err := s.chatService.Pending(ctx, x.username, 128)
	if err != nil {
		s.logError(ctx, "load pending messages: %v", err)
		return
	}

	for _, m := range messages {
		if _, exists := x.inflight[m.ID]; exists {
			continue
		}
		p := s.messagePacket(ctx, m, x.username)
		if m.Kind == 1 {
			peer := m.Sender
			if m.Action && m.Kind == 1 && x.username == m.Sender {
				peer = m.Conversation
			}
			if contact, err := s.contactsService.ContactByUsername(ctx, x.username, peer); err == nil {
				data := p.Data.(events.Message)
				data.ContactID = &contact.ID
				p.Data = data
			}
		}
		s.reserveEvent(p)
		select {
		case x.push <- p:
			x.inflight[m.ID] = p
			s.reserve(p, delivery{sessionID: x.id, username: x.username, actor: m.Sender, messageID: m.ID, kind: m.Kind, target: m.Conversation})
		default:
			return
		}
	}
}

// allowPacket checks queued group messages again at the transport boundary.
// A leave cancels stored pending messages; this also prevents already-buffered
// messages/notices from reopening the original client's closed group tab.
func (s *Coordinator) allowPacket(ctx context.Context, id string, p *events.Event) bool {
	if _, ok := p.Data.(events.Activity); ok {
		return s.allowActivityPacket(ctx, id, p)
	}
	switch p.Data.(type) {
	case events.Message, events.Notice, events.GroupCreated:
	default:
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if d, ok := s.deliveries[p.Handle]; ok && d.canceled {
		s.releasePacket(ctx, p, false)
		return false
	}
	x := s.active(ctx, id)
	if x == nil || !x.authenticated {
		s.releasePacket(ctx, p, false)
		return false
	}
	if invite, ok := p.Data.(events.GroupCreated); ok {
		_, err := s.chatService.Group(ctx, invite.ID, x.username)
		return err == nil
	}

	var sender, target string
	var kind byte
	switch data := p.Data.(type) {
	case events.Message:
		sender = data.Message.Sender
		kind = data.Message.Kind
		target = data.Message.Conversation
	case events.Notice:
		kind = data.Kind
		target = data.Target
	}
	if d, ok := s.deliveries[p.Handle]; ok && d.actor != "" {
		sender = d.actor
	}

	if s.blocksIncoming(ctx, x.username, sender) {
		s.releasePacket(ctx, p, false)
		return false
	}
	if kind == 2 {
		if _, err := s.chatService.Group(ctx, target, x.username); err != nil {
			s.releasePacket(ctx, p, false)
			return false
		}
	}
	if kind == 3 && !x.rooms[target] {
		s.releasePacket(ctx, p, false)
		return false
	}
	return true
}

func (s *Coordinator) cancelRoomDeliveries(sessionID, roomName string) {
	for handle, d := range s.deliveries {
		if d.sessionID == sessionID && d.kind == 3 && d.target == roomName {
			d.canceled = true
			s.deliveries[handle] = d
		}
	}
}

func (s *Coordinator) cancelGroupDeliveries(username, groupID string) {
	for handle, d := range s.deliveries {
		if d.username == username && d.kind == 2 && d.target == groupID {
			d.canceled = true
			s.deliveries[handle] = d
		}
	}
}

// releasePacket only marks a durable outbox row after a successful transport
// write. A crash after write but before marking can replay a message: the old
// protocol has no client delivery receipt to make this exactly once.
func (s *Coordinator) releasePacket(ctx context.Context, p *events.Event, written bool) {
	d, ok := s.deliveries[p.Handle]
	if !ok {
		return
	}

	if written && !d.ownMirror {
		if err := s.chatService.MarkDelivered(ctx, d.username, d.messageID); err != nil {
			s.logError(ctx, "mark message %d delivered: %v", d.messageID, err)
		}
	}

	if x := s.sessions[d.sessionID]; x != nil {
		delete(x.inflight, d.messageID)
		if d.ownMirror && (written || d.canceled) {
			delete(x.ownPending, d.messageID)
		}
	}
	delete(s.deliveries, p.Handle)
}

// The original private500 renderer uses sender2 as BOTH the tab address and
// sender label. It has no outgoing-message flag. An own-private live mirror
// therefore uses the recovered action renderer with an explicit trusted author
// in its body, and peer2 for correct routing. JSON/history retains exact text
// and RGB. Group and room packets can retain the ordinary sender and style.
func (s *Coordinator) ownMessagePacket(ctx context.Context, m chat.Message) *events.Event {
	p := s.messagePacket(ctx, m, m.Sender)
	data := p.Data.(events.Message)
	data.OwnMirror = true
	if m.Kind == 1 {
		data.Avatar = ""
		if c, err := s.contactsService.ContactByUsername(ctx, m.Sender, m.Conversation); err == nil {
			data.ContactID = &c.ID
		}
	}
	p.Data = data
	return p
}

func (s *Coordinator) flushOwnMessages(ctx context.Context, x *session) {
	if x.ownPending == nil {
		x.ownPending = make(map[int64]chat.Message)
	}

	queue := func(m chat.Message) bool {
		if m.Kind == 2 {
			if _, err := s.chatService.Group(ctx, m.Conversation, x.username); err != nil {
				if ctx.Err() == nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) && (errors.Is(err, faults.ErrNotFound) ||
					errors.Is(err, faults.ErrForbidden)) {
					delete(x.ownPending, m.ID)
					return true
				}
				// A failed lookup does not establish that membership ended.
				// Preserve the mirror and transcript cursor for the next poll.
				s.logError(ctx, "check own message group membership: %v", err)
				return false
			}
		}
		if _, ok := x.inflight[m.ID]; ok {
			return true
		}
		if len(x.push) == cap(x.push) {
			return false
		}
		p := s.ownMessagePacket(ctx, m)
		s.reserveEvent(p)
		x.push <- p
		x.inflight[m.ID] = p
		x.ownPending[m.ID] = m
		s.reserve(p, delivery{sessionID: x.id, username: x.username, actor: m.Sender, messageID: m.ID, ownMirror: true, kind: m.Kind, target: m.Conversation})
		return true
	}

	ids := make([]int64, 0, len(x.ownPending))
	for id := range x.ownPending {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		if !queue(x.ownPending[id]) {
			return
		}
	}
	if len(x.push) == cap(x.push) {
		return
	}

	messages, err := s.chatService.OwnMessagesAfter(ctx, x.username, x.ownAfter, 128)
	if err != nil {
		s.logError(ctx, "load own transcript: %v", err)
		return
	}

	for _, m := range messages {
		if x.ownSkipped[m.ID] {
			delete(x.ownSkipped, m.ID)
		} else if !queue(m) {
			return
		}
		x.ownAfter = m.ID
	}
}

func (s *Coordinator) finishWrite(ctx context.Context, packets []*events.Event, written bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, p := range packets {
		if _, ok := p.Data.(events.Activity); ok {
			s.releaseActivityPacket(p)
		}
		s.releasePacket(ctx, p, written)
	}
}
