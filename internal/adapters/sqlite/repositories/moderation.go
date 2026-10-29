package repositories

import (
	"context"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
)

// IsRoomModerator reports a persisted role for a canonicalized room and user.
// It does not check live room membership, which belongs to the server.
func (s *Store) IsRoomModerator(ctx context.Context, room, user string) (bool, error) {
	return s.roomAccountFlag(ctx, "room_moderators", room, user)
}

// RoomModerators lists moderator usernames in sorted order.
func (s *Store) RoomModerators(ctx context.Context, room string) ([]string, error) {
	return s.roomAccounts(ctx, "room_moderators", room)
}

// IsRoomMuted reports a persistent room-specific mute. Mutes survive a session
// leaving or reconnecting; they do not prevent joining or reading the room.
func (s *Store) IsRoomMuted(ctx context.Context, room, user string) (bool, error) {
	return s.roomAccountFlag(ctx, "room_mutes", room, user)
}

// RoomMuted lists muted usernames in sorted order.
func (s *Store) RoomMuted(ctx context.Context, room string) ([]string, error) {
	return s.roomAccounts(ctx, "room_mutes", room)
}

// SetRoomMuted changes a mute only when actor is a persisted room moderator.
// Actors cannot target themselves or another moderator, including for unmute.
// Authorization and the mutation are atomic; caller must also enforce current
// room membership for the actor and target and validate target account existence.
func (s *Store) SetRoomMuted(ctx context.Context, room, actor, target string, muted bool) error {
	actor, err := username(actor)
	if err != nil {
		return queryError(ctx, err)
	}

	target, err = username(target)
	if err != nil {
		return queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	definition, err := readRoom(ctx, tx, room)
	if err != nil {
		return queryError(ctx, err)
	}
	if actor == target {
		return ErrForbidden
	}

	allowed, err := roomAccountFlag(ctx, tx, "room_moderators", definition.Name, actor)
	if err != nil {
		return queryError(ctx, err)
	}
	if !allowed {
		return ErrForbidden
	}

	protected, err := roomAccountFlag(ctx, tx, "room_moderators", definition.Name, target)
	if err != nil {
		return queryError(ctx, err)
	}
	if protected {
		return ErrForbidden
	}
	if muted {
		err = queries(tx).AddRoomMute(ctx, sqlc.AddRoomMuteParams{Room: definition.Name, Username: target})
	} else {
		err = queries(tx).RemoveRoomMute(ctx, sqlc.RemoveRoomMuteParams{Room: definition.Name, Username: target})
	}
	if err != nil {
		return queryError(ctx, err)
	}

	op := "unmute"
	if muted {
		op = "mute"
	}
	if err := roomAudit(ctx, tx, definition.Name, actor, op, target); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

// SetRoomModerator is a trusted operator operation, not a client permission
// grant. The caller must restrict access to server administration. Granting a
// role also clears an existing mute in the same transaction. Revocation is
// permitted even for the last moderator and survives subsequent restarts.
func (s *Store) SetRoomModerator(ctx context.Context, room, user string, enabled bool) error {
	user, err := username(user)
	if err != nil {
		return queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	definition, err := readRoom(ctx, tx, room)
	if err != nil {
		return queryError(ctx, err)
	}
	if enabled {
		if err := queries(tx).AddRoomModerator(ctx, sqlc.AddRoomModeratorParams{Room: definition.Name, Username: user}); err != nil {
			return queryError(ctx, err)
		}
		if err := queries(tx).RemoveRoomMute(ctx, sqlc.RemoveRoomMuteParams{Room: definition.Name, Username: user}); err != nil {
			return queryError(ctx, err)
		}
	} else {
		if err := queries(tx).RemoveRoomModerator(ctx, sqlc.RemoveRoomModeratorParams{Room: definition.Name, Username: user}); err != nil {
			return queryError(ctx, err)
		}
	}

	op := "moderator-remove"
	if enabled {
		op = "moderator-add"
	}
	if err := roomAudit(ctx, tx, definition.Name, "operator", op, user); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func (s *Store) roomAccountFlag(ctx context.Context, table, room, user string) (bool, error) {
	user, err := username(user)
	if err != nil {
		return false, queryError(ctx, err)
	}

	definition, err := s.Room(ctx, room)
	if err != nil {
		return false, queryError(ctx, err)
	}

	return roomAccountFlag(ctx, s.db, table, definition.Name, user)
}

// table is always a package-owned literal, never caller input.
func roomAccountFlag(ctx context.Context, db rowQuerier, table, room, user string) (bool, error) {
	q := queries(db)
	switch table {
	case "room_moderators":
		value, err := q.IsRoomModerator(ctx, sqlc.IsRoomModeratorParams{Room: room, Username: user})
		return value, queryError(ctx, err)
	case "room_mutes":
		value, err := q.IsRoomMuted(ctx, sqlc.IsRoomMutedParams{Room: room, Username: user})
		return value, queryError(ctx, err)
	case "room_bans":
		value, err := q.IsRoomBanned(ctx, sqlc.IsRoomBannedParams{Room: room, Username: user})
		return value, queryError(ctx, err)
	default:
		return false, ErrInvalid
	}
}

func (s *Store) roomAccounts(ctx context.Context, table, room string) ([]string, error) {
	definition, err := s.Room(ctx, room)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	q := queries(s.db)
	switch table {
	case "room_moderators":
		value, err := q.ListRoomModerators(ctx, definition.Name)
		return value, queryError(ctx, err)
	case "room_mutes":
		value, err := q.ListRoomMuted(ctx, definition.Name)
		return value, queryError(ctx, err)
	case "room_bans":
		value, err := q.ListRoomBanned(ctx, definition.Name)
		return value, queryError(ctx, err)
	default:
		return nil, ErrInvalid
	}
}
