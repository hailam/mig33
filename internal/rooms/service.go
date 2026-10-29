package rooms

import (
	"context"

	"github.com/hailam/mig33/internal/faults"
)

// Service owns durable room policy. Its caller owns a synchronized snapshot of
// live participants and applies approved membership changes before releasing it.
type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context, user string, category int16) ([]Room, error) {
	if category == 2 {
		return s.repository.FavoriteRooms(ctx, user)
	}
	categories, err := s.repository.RoomCategories(ctx)
	if err != nil {
		return nil, err
	}
	for _, candidate := range categories {
		if candidate.ID == category {
			return s.repository.Rooms(ctx, category)
		}
	}
	return nil, faults.NewError(faults.Invalid, "Unknown room category")
}

// CheckJoin checks durable permissions even when the session is already joined.
// Multiple active sessions of the same account occupy one place in a room.
func (s *Service) CheckJoin(ctx context.Context, user string, room Room, alreadyJoined bool, participants []string) error {
	allowed, _, err := s.repository.GroupRoomAccess(ctx, room.Name, user)
	if err != nil {
		return err
	}
	if !allowed {
		return faults.NewError(faults.Forbidden, "This Group room is locked or you are banned from its Group rooms")
	}
	banned, err := s.repository.RoomBanned(ctx, room.Name, user)
	if err != nil {
		return err
	}
	if banned {
		return faults.NewError(faults.Forbidden, "You are banned from this chat room")
	}
	if !alreadyJoined && !HasUser(participants, user) && len(participants) >= room.Capacity {
		return faults.NewError(faults.Limited, "This chat room is full")
	}
	return nil
}

// CheckModeration returns whether the actor has a moderator role. A regular
// account may request a kick ballot; only moderators may mute or unmute.
func (s *Service) CheckModeration(ctx context.Context, actor, target, room string, actorJoined, targetPresent, kick bool) (bool, error) {
	if !actorJoined {
		return false, faults.NewError(faults.Forbidden, "Join the chat room first")
	}
	moderator, err := s.repository.IsRoomModerator(ctx, room, actor)
	if err != nil {
		return false, err
	}
	if !moderator && !kick {
		return false, faults.NewError(faults.Forbidden, "Only room moderators can perform this action")
	}
	if !targetPresent {
		return false, faults.NewError(faults.Invalid, "Choose a user who is in this chat room")
	}
	if target == actor {
		return false, faults.NewError(faults.Forbidden, "You cannot moderate yourself")
	}
	targetModerator, err := s.repository.IsRoomModerator(ctx, room, target)
	if err != nil {
		return false, err
	}
	if targetModerator {
		return false, faults.NewError(faults.Forbidden, "You cannot moderate another room moderator")
	}
	return moderator, nil
}

// SetMuted persists the state before its caller publishes a room notification.
// The returned flag distinguishes a transition from an idempotent request.
func (s *Service) SetMuted(ctx context.Context, room, actor, target string, muted bool) (bool, error) {
	previous, err := s.repository.IsRoomMuted(ctx, room, target)
	if err != nil {
		return false, err
	}
	if err := s.repository.SetRoomMuted(ctx, room, actor, target, muted); err != nil {
		return false, err
	}
	return previous != muted, nil
}
