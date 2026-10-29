package realtime

import (
	"context"
	"time"

	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
)

type activityDelivery struct {
	ID    int64
	Owner string
}

func (s *Coordinator) activityPacket(ctx context.Context, entry activity.ServiceEntry) *events.Event {
	action, label := "posted an update", "Read update"
	if entry.Kind == "checkin" {
		action, label = "checked in", "View check-in"
	}
	payload := events.Activity{ID: entry.ID, Timestamp: entry.CreatedAt, Actor: entry.Owner, Display: action, Text: label}
	if avatar, err := s.mediaService.Avatar(ctx, entry.Owner); err == nil {
		payload.ImageID = avatar
	}
	return events.New(0, payload)
}

func (s *Coordinator) rememberActivityPacket(x *session, entry activity.ServiceEntry, event *events.Event) {
	if x.activityPackets == nil {
		x.activityPackets = make(map[events.Handle]activityDelivery)
	}
	if x.activitySeen == nil {
		x.activitySeen = make(map[int64]bool)
	}
	if event.Handle == 0 {
		s.reserveEvent(event)
	}
	x.activityPackets[event.Handle] = activityDelivery{ID: entry.ID, Owner: entry.Owner}
	x.activitySeen[entry.ID] = true
}

func (s *Coordinator) subscribeActivity(ctx context.Context, x *session, request *commands.Request) []*events.Event {
	mode, ok := request.Input.(commands.SubscribeActivity).Mode.Get()
	if !ok || mode > 1 {
		return []*events.Event{events.Failure(request.Correlation, "Choose whether to show activity updates")}
	}
	ack := events.Ack(request.Correlation)
	if mode == 0 {
		x.activities = false
		return []*events.Event{ack}
	}
	if x.activities {
		return []*events.Event{ack}
	}
	entries, err := s.activityService.ActivityEntries(ctx, x.username, "")
	if err != nil {
		return []*events.Event{events.Failure(request.Correlation, "Activity updates are temporarily unavailable")}
	}
	x.activities = true
	out := []*events.Event{ack}
	for _, entry := range entries {
		if x.activitySeen[entry.ID] {
			continue
		}
		event := s.activityPacket(ctx, entry)
		s.rememberActivityPacket(x, entry, event)
		out = append(out, event)
		if len(out) >= 21 {
			break
		}
	}
	return out
}

func (s *Coordinator) broadcastActivity(ctx context.Context, entry activity.ServiceEntry) {
	for _, x := range s.sessions {
		if !x.authenticated || !x.activities || x.activitySeen[entry.ID] || time.Since(x.touched) >= sessionTTL {
			continue
		}
		allowed, err := s.privacy.CanViewActivity(ctx, x.username, entry.Owner)
		if err != nil || !allowed {
			continue
		}
		event := s.activityPacket(ctx, entry)
		s.reserveEvent(event)
		select {
		case x.push <- event:
			s.rememberActivityPacket(x, entry, event)
		default:
		}
	}
}

// Recheck subscription, existence and privacy after reservation and before write.
func (s *Coordinator) allowActivityPacket(ctx context.Context, id string, event *events.Event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	x := s.active(ctx, id)
	if x == nil || !x.authenticated {
		s.releaseActivityPacket(event)
		return false
	}
	meta, ok := x.activityPackets[event.Handle]
	if !ok {
		return false
	}
	if !x.activities {
		delete(x.activityPackets, event.Handle)
		return false
	}
	entry, err := s.activityService.ActivityForDelivery(ctx, x.username, meta.ID)
	if err != nil || entry.Owner != meta.Owner {
		delete(x.activityPackets, event.Handle)
		return false
	}
	return true
}

func (s *Coordinator) releaseActivityPacket(event *events.Event) {
	for _, x := range s.sessions {
		delete(x.activityPackets, event.Handle)
	}
}
