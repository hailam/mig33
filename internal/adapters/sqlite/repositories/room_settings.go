package repositories

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
	"github.com/hailam/mig33/internal/rooms"
)

type (
	RoomSettings = rooms.RoomSettings
	RoomChange   = rooms.RoomChange
)

func (s *Store) RoomSettings(ctx context.Context, room string) (RoomSettings, error) {
	if _, err := s.Room(ctx, room); err != nil {
		return RoomSettings{}, queryError(ctx, err)
	}

	r, err := queries(s.db).GetRoomSettings(ctx, room)
	return RoomSettings{Language: r.Language, Keywords: r.Keywords, UserKicks: r.UserKicks != 0}, queryError(ctx, err)
}

func (s *Store) RoomBanned(ctx context.Context, room, user string) (bool, error) {
	value, err := queries(s.db).IsRoomBanned(ctx, sqlc.IsRoomBannedParams{Room: room, Username: user})
	return value, queryError(ctx, err)
}

func (s *Store) RoomBans(ctx context.Context, room string) ([]string, error) {
	value, err := queries(s.db).ListRoomBanned(ctx, room)
	return value, queryError(ctx, err)
}

func (s *Store) RoomChanges(ctx context.Context, room string) ([]RoomChange, error) {
	rows, err := queries(s.db).ListRoomChanges(ctx, room)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	out := make([]RoomChange, 0, len(rows))
	for _, r := range rows {
		out = append(out, RoomChange{ID: r.ID, Actor: r.Actor, Operation: r.Operation, Value: r.Value, CreatedAt: r.CreatedAt})
	}

	return out, nil
}

// ManageRoom rechecks ownership/roles in the mutation transaction. Session
// presence, account existence and capacity versus live occupancy are caller checks.
func (s *Store) ManageRoom(ctx context.Context, room, actor, operation, value string) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	r, err := readRoom(ctx, tx, room)
	if err != nil {
		return queryError(ctx, err)
	}

	mod, err := roomAccountFlag(ctx, tx, "room_moderators", r.Name, actor)
	if err != nil {
		return queryError(ctx, err)
	}
	if r.Owner != actor && !(mod && (operation == "ban" || operation == "unban")) {
		return ErrForbidden
	}
	if err = queries(tx).EnsureRoomSettings(ctx, r.Name); err != nil {
		return queryError(ctx, err)
	}

	switch operation {
	case "language":
		if len(value) < 1 || len(value) > 40 {
			return ErrInvalid
		}

		err = queries(tx).SetRoomLanguage(ctx, sqlc.SetRoomLanguageParams{Language: value, Room: r.Name})
	case "keywords":
		if len(value) > 320 {
			return ErrInvalid
		}

		err = queries(tx).SetRoomKeywords(ctx, sqlc.SetRoomKeywordsParams{Keywords: value, Room: r.Name})
	case "description":
		if len(value) > 1000 {
			return ErrInvalid
		}

		err = queries(tx).SetRoomDescription(ctx, sqlc.SetRoomDescriptionParams{Description: value, Name: r.Name})
	case "kicks":
		if value != "0" && value != "1" {
			return ErrInvalid
		}

		err = queries(tx).SetRoomUserKicks(ctx, sqlc.SetRoomUserKicksParams{UserKicks: boolInt64(value == "1"), Room: r.Name})
	case "capacity":
		n, e := strconv.Atoi(value)
		if e != nil || n < 2 || n > 100 {
			return ErrInvalid
		}

		err = queries(tx).SetRoomCapacity(ctx, sqlc.SetRoomCapacityParams{Capacity: int64(n), Name: r.Name})
	case "owner", "moderator-add", "moderator-remove", "ban", "unban":
		value, err = username(value)
		if err != nil {
			return queryError(ctx, err)
		}
		if value == actor {
			return ErrForbidden
		}

		protected, e := roomAccountFlag(ctx, tx, "room_moderators", r.Name, value)
		if e != nil {
			return e
		}
		if operation == "ban" && (protected || value == r.Owner) {
			return ErrForbidden
		}
		if operation == "moderator-remove" && value == r.Owner {
			return ErrForbidden
		}

		switch operation {
		case "owner":
			err = queries(tx).SetRoomOwner(ctx, sqlc.SetRoomOwnerParams{Owner: value, Name: r.Name})
			if err != nil {
				return queryError(ctx, err)
			}

			fallthrough
		case "moderator-add":
			err = queries(tx).AddRoomModerator(ctx, sqlc.AddRoomModeratorParams{Room: r.Name, Username: value})
			if err != nil {
				return queryError(ctx, err)
			}

			err = queries(tx).RemoveRoomBan(ctx, sqlc.RemoveRoomBanParams{Room: r.Name, Username: value})
			if err != nil {
				return queryError(ctx, err)
			}

			err = queries(tx).RemoveRoomMute(ctx, sqlc.RemoveRoomMuteParams{Room: r.Name, Username: value})
		case "moderator-remove":
			err = queries(tx).RemoveRoomModerator(ctx, sqlc.RemoveRoomModeratorParams{Room: r.Name, Username: value})
		case "ban":
			err = queries(tx).AddRoomBan(ctx, sqlc.AddRoomBanParams{Room: r.Name, Username: value, Actor: actor, CreatedAt: time.Now().UnixMilli()})
		case "unban":
			err = queries(tx).RemoveRoomBan(ctx, sqlc.RemoveRoomBanParams{Room: r.Name, Username: value})
		}
	default:
		return ErrInvalid
	}
	if err != nil {
		return queryError(ctx, err)
	}
	if err = roomAudit(ctx, tx, r.Name, actor, operation, value); err != nil {
		return fmt.Errorf("record room change: %w", err)
	}

	return queryError(ctx, tx.Commit())
}
