package realtime

import (
	"sync"

	"github.com/hailam/mig33/internal/events"
)

// responseGate returns a session's transport-ordering lock without retaining
// the server state lock. The lock order is responseGate, account deliveryGate,
// then s.mu; no caller waits for either outer gate while holding s.mu.
func (s *Coordinator) responseGate(id string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()

	if x := s.sessions[id]; x != nil {
		return &x.responseGate
	}
	return nil
}

// beginResponse serializes a gateway request, queue drain and transport write
// against other responses for the same session. Different sessions still run
// independently, including two sessions belonging to the same account.
func (s *Coordinator) beginResponse(id string) func() {
	if gate := s.responseGate(id); gate != nil {
		gate.Lock()
		return gate.Unlock
	}
	return func() {}
}

func (s *Coordinator) tryBeginResponse(id string) (func(), bool) {
	if gate := s.responseGate(id); gate != nil {
		if !gate.TryLock() {
			return nil, false
		}
		return gate.Unlock, true
	}
	return func() {}, true
}

// contactPrefix is called with s.mu held, before constructing a contact reply.
// Older queued events must precede the new mutation or snapshot: sending an old
// 402 after a newer delete ACK would recreate the deleted contact on the client.
// Preserve all queued traffic in FIFO order, including chat and its delivery
// bookkeeping. The transport still applies allowPacket before writing it.
func (s *Coordinator) contactPrefix(x *session) []*events.Event {
	var prefix []*events.Event
	for {
		select {
		case p := <-x.push:
			prefix = append(prefix, p)
		default:
			return prefix
		}
	}
}
