package realtime

import (
	"context"

	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/faults"
)

func (s *Coordinator) ChatGroups(ctx context.Context, sid string) ([]chat.Group, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireSession(ctx, sid)
	if err != nil {
		return nil, err
	}
	groups, err := s.chatService.Groups(ctx, x.username)
	if err != nil {
		return nil, err
	}
	if groups == nil {
		groups = []chat.Group{}
	}
	return groups, nil
}

// ChatHistory requires active room membership, including when the room exists.
func (s *Coordinator) ChatHistory(ctx context.Context, sid string, query chat.HistoryQuery) (chat.HistoryPage, error) {
	var result chat.HistoryPage
	if err := ctx.Err(); err != nil {
		return result, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireSession(ctx, sid)
	if err != nil {
		return result, err
	}
	kind := chat.Kind(query.Kind)
	if kind == 0 {
		return result, faults.NewError(faults.Invalid, "kind must be private, group or room")
	}
	if query.AfterID < 0 {
		return result, faults.NewError(faults.Invalid, "Invalid after_id")
	}
	if query.Limit < 1 || query.Limit > 200 || query.Target == "" {
		return result, faults.NewError(faults.Invalid, "target is required; limit must be 1 to 200")
	}
	if kind == 3 {
		room, err := s.roomsService.Room(ctx, query.Target)
		if err != nil || !x.rooms[room.Name] {
			return result, faults.WrapError(faults.Forbidden, "Join the chat room to read its history", err)
		}
		query.Target = room.Name
	}
	return s.chatService.HistoryPage(ctx, x.username, query)
}
