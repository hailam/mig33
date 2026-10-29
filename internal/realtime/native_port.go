package realtime

import (
	"context"

	"github.com/hailam/mig33/internal/events"
)

func (s *Coordinator) BeginWrite(id string) func() { return s.beginWrite(id) }

func (s *Coordinator) AllowEvent(ctx context.Context, id string, e *events.Event) bool {
	return s.allowPacket(ctx, id, e)
}

func (s *Coordinator) FinishWrite(ctx context.Context, out []*events.Event, written bool) {
	s.finishWrite(ctx, out, written)
}

func (s *Coordinator) BeginResponse(id string) func() { return s.beginResponse(id) }

func (s *Coordinator) TryBeginResponse(id string) (func(), bool) { return s.tryBeginResponse(id) }

// Poll reserves only the number of events the transport will actually process.
// Its caller holds the response gate, preserving shared-session ordering.
func (s *Coordinator) Poll(ctx context.Context, id string, limit int) []*events.Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	x := s.active(ctx, id)
	if x == nil || !x.authenticated {
		return nil
	}
	s.flushPending(ctx, x)
	out := make([]*events.Event, 0, min(limit, len(x.push)))
	for len(out) < limit {
		select {
		case e := <-x.push:
			out = append(out, e)
		default:
			return out
		}
	}
	return out
}
