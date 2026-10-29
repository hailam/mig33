package repositories

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
	"github.com/hailam/mig33/internal/rooms"
)

type (
	RoomCategory      = rooms.RoomCategory
	GroupRoomControls = rooms.GroupRoomControls
	RoomBotScore      = rooms.RoomBotScore
)

func (s *Store) RoomCategories(ctx context.Context) ([]RoomCategory, error) {
	rows, err := queries(s.db).ListRoomCategories(ctx)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	out := make([]RoomCategory, 0, len(rows))
	for _, r := range rows {
		out = append(out, RoomCategory{ID: int16(r.ID), Name: r.Name})
	}

	return out, nil
}

// ManageRoomCategory is a trusted operator-only store API. ID 2 belongs to the
// client's per-user Favorites view and can never become a stored category.
func (s *Store) ManageRoomCategory(ctx context.Context, actor string, id int16, name string, remove bool) error {
	if actor == "" || id < 1 || id == 2 || !validCommunityText(name, 80) || (!remove && strings.TrimSpace(name) == "") {
		return ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	if remove {
		if id == 1 {
			return ErrForbidden
		}

		n, countErr := queries(tx).CountRoomsInCategory(ctx, int64(id))
		if countErr != nil {
			return queryError(ctx, countErr)
		}
		if n != 0 {
			return ErrConflict
		}

		err = queries(tx).RemoveRoomCategory(ctx, int64(id))
	} else {
		n, countErr := queries(tx).CountOtherRoomCategories(ctx, int64(id))
		if countErr != nil {
			return queryError(ctx, countErr)
		}
		if n >= 30 {
			return ErrLimit
		}

		err = queries(tx).SaveRoomCategory(ctx, sqlc.SaveRoomCategoryParams{ID: int64(id), Name: strings.TrimSpace(name)})
	}
	if err != nil {
		return queryError(ctx, err)
	}

	op := "category-save"
	if remove {
		op = "category-delete"
	}
	if err = roomAudit(ctx, tx, "", actor, op, strconv.Itoa(int(id))+":"+name); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func (s *Store) SetRoomCategory(ctx context.Context, actor, room string, id int16) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	r, err := readRoom(ctx, tx, room)
	if err != nil {
		return queryError(ctx, err)
	}

	found, err := queries(tx).RoomCategoryExists(ctx, int64(id))
	if err != nil {
		return queryError(ctx, err)
	}
	if !found {
		return ErrNotFound
	}
	if err = queries(tx).SetRoomCategory(ctx, sqlc.SetRoomCategoryParams{Category: int64(id), Name: r.Name}); err != nil {
		return queryError(ctx, err)
	}
	if err = roomAudit(ctx, tx, r.Name, actor, "category", strconv.Itoa(int(id))); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

// DeleteOwnedRoom removes live definitions and dependent roles/bookmarks while
// retaining history and audit. Tombstones prevent a new room inheriting an old
// room's name-based history. The server must evict live sessions after commit.
func (s *Store) DeleteOwnedRoom(ctx context.Context, room, actor string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	r, err := readRoom(ctx, tx, room)
	if err != nil {
		return queryError(ctx, err)
	}
	if r.Owner == "" || r.Owner != actor {
		return ErrForbidden
	}
	if err = roomAudit(ctx, tx, r.Name, actor, "delete", r.Description); err != nil {
		return queryError(ctx, err)
	}
	if err = queries(tx).AddRoomTombstone(ctx, sqlc.AddRoomTombstoneParams{Name: r.Name, Owner: actor, DeletedAt: time.Now().UnixMilli()}); err != nil {
		return queryError(ctx, err)
	}

	q := queries(tx)
	for _, remove := range []func(context.Context, string) error{
		q.RemoveRoomCommunityLinks,
		q.RemoveRoomFavorites,
		q.RemoveRoomModerators,
		q.RemoveRoomMutes,
		q.RemoveRoomBans,
		q.RemoveRoomSettings,
	} {
		if err = remove(ctx, r.Name); err != nil {
			return queryError(ctx, err)
		}
	}
	if err = q.RemoveRoomDefinition(ctx, r.Name); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func roomAudit(ctx context.Context, db sqlc.DBTX, room, actor, operation, value string) error {
	return queryError(ctx, queries(db).AddRoomChange(ctx, sqlc.AddRoomChangeParams{
		Room:      room,
		Actor:     actor,
		Operation: operation,
		Value:     value,
		CreatedAt: time.Now().UnixMilli(),
	}))
}

func (s *Store) RecordRoomEvent(ctx context.Context, room, actor, operation, value string) error {
	return roomAudit(ctx, s.db, room, actor, operation, value)
}

func groupRoomControls(ctx context.Context, db sqlc.DBTX, room string) (GroupRoomControls, error) {
	r, err := queries(db).GetGroupRoomControls(ctx, room)
	return GroupRoomControls{
		GroupID:    r.GroupID,
		Silenced:   r.Silenced != 0,
		Locked:     r.Locked != 0,
		Bot:        r.Bot,
		BotExpires: r.BotExpires,
		BotRounds:  int(r.BotRounds),
	}, queryError(ctx, err)
}

func (s *Store) GroupRoomControls(ctx context.Context, room string) (GroupRoomControls, error) {
	return groupRoomControls(ctx, s.db, room)
}

func groupRoomRole(ctx context.Context, db sqlc.DBTX, id int64, user string) (string, error) {
	role, err := queries(db).GetGroupRoomRole(ctx, sqlc.GetGroupRoomRoleParams{GroupID: id, Username: user})
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}

	return role, queryError(ctx, err)
}

// GroupRoomAccess applies only while the room is linked. Private Groups do not
// automatically lock public rooms: an explicit /lock is required.
func (s *Store) GroupRoomAccess(ctx context.Context, room, user string) (canJoin, canSpeak bool, err error) {
	c, err := s.GroupRoomControls(ctx, room)
	if errors.Is(err, ErrNotFound) {
		return true, true, nil
	}
	if err != nil {
		return false, false, queryError(ctx, err)
	}

	banned, err := queries(s.db).IsCommunityRoomBanned(ctx, sqlc.IsCommunityRoomBannedParams{GroupID: c.GroupID, Username: user})
	if err != nil {
		return false, false, queryError(ctx, err)
	}
	if banned {
		return false, false, nil
	}

	role, err := groupRoomRole(ctx, s.db, c.GroupID, user)
	if err != nil {
		return false, false, queryError(ctx, err)
	}

	mod, err := s.IsRoomModerator(ctx, room, user)
	if err != nil {
		return false, false, queryError(ctx, err)
	}

	privileged := mod || role == "owner" || role == "moderator"
	member := role != "" && role != "pending"
	return !c.Locked || member || privileged, !c.Silenced || privileged, nil
}

// ManageGroupRoom rechecks community role inside the transaction. Community
// moderation is deliberately scoped to linked rooms, not private group chats.
func (s *Store) ManageGroupRoom(ctx context.Context, room, actor, operation, value string) ([]Room, error) {
	if !validCommunityText(value, 1000) {
		return nil, ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, queryError(ctx, err)
	}
	defer tx.Rollback()

	r, err := readRoom(ctx, tx, room)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	c, err := groupRoomControls(ctx, tx, r.Name)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	role, err := groupRoomRole(ctx, tx, c.GroupID, actor)
	if err != nil {
		return nil, queryError(ctx, err)
	}
	if role != "owner" && role != "moderator" {
		return nil, ErrForbidden
	}

	rows, err := queries(tx).ListLinkedRoomDefinitions(ctx, c.GroupID)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	rooms := roomsFromRows(rows)
	if err = queries(tx).EnsureCommunityRoomControls(ctx, r.Name); err != nil {
		return nil, queryError(ctx, err)
	}

	switch operation {
	case "silence", "lock":
		if value != "0" && value != "1" {
			return nil, ErrInvalid
		}
		if operation == "lock" {
			err = queries(tx).SetCommunityRoomLocked(ctx, sqlc.SetCommunityRoomLockedParams{Locked: boolInt64(value == "1"), Room: r.Name})
		} else {
			err = queries(tx).SetCommunityRoomSilenced(ctx, sqlc.SetCommunityRoomSilencedParams{Silenced: boolInt64(value == "1"), Room: r.Name})
		}
	case "description":
		err = queries(tx).SetRoomDescription(ctx, sqlc.SetRoomDescriptionParams{Description: value, Name: r.Name})
	case "ban", "unban":
		value, err = username(value)
		if err != nil {
			return nil, queryError(ctx, err)
		}
		if value == actor {
			return nil, ErrForbidden
		}

		targetRole, e := groupRoomRole(ctx, tx, c.GroupID, value)
		if e != nil {
			return nil, e
		}
		if operation == "ban" && (targetRole == "owner" || targetRole == "moderator") {
			return nil, ErrForbidden
		}

		for _, linked := range rooms {
			mod, e := roomAccountFlag(ctx, tx, "room_moderators", linked.Name, value)
			if e != nil {
				return nil, e
			}
			if operation == "ban" && (linked.Owner == value || mod) {
				return nil, ErrForbidden
			}
		}
		if operation == "ban" {
			err = queries(tx).AddCommunityRoomBan(ctx, sqlc.AddCommunityRoomBanParams{GroupID: c.GroupID, Username: value, Actor: actor, CreatedAt: time.Now().UnixMilli()})
		} else {
			err = queries(tx).RemoveCommunityRoomBan(ctx, sqlc.RemoveCommunityRoomBanParams{GroupID: c.GroupID, Username: value})
		}
	case "bot":
		if value != "dice" && value != "8ball" {
			return nil, ErrInvalid
		}
		if c.Bot != "" && c.BotExpires > time.Now().UnixMilli() && c.BotRounds < 20 {
			return nil, ErrConflict
		}

		err = queries(tx).StartRoomBot(ctx, sqlc.StartRoomBotParams{Bot: value, BotExpires: time.Now().Add(30 * time.Minute).UnixMilli(), Room: r.Name})
	case "botstop":
		err = queries(tx).StopRoomBot(ctx, r.Name)
	case "invite":
		value, err = username(value)
		if err != nil {
			return nil, queryError(ctx, err)
		}

		var nonce [24]byte
		if _, err = rand.Read(nonce[:]); err != nil {
			return nil, queryError(ctx, err)
		}

		err = queries(tx).AddRoomInvitation(ctx, sqlc.AddRoomInvitationParams{
			Owner:     actor,
			Target:    value,
			Body:      "Invitation to join chat room " + r.Name + ". Open Chat Rooms and search for this name; existing membership, locks and bans still apply.",
			Nonce:     hex.EncodeToString(nonce[:]),
			CreatedAt: time.Now().UnixMilli(),
		})
	case "announce", "broadcast":
		if strings.TrimSpace(value) == "" {
			return nil, ErrInvalid
		}
	default:
		return nil, ErrInvalid
	}
	if err != nil {
		return nil, queryError(ctx, err)
	}

	auditRooms := []Room{r}
	if operation == "ban" || operation == "unban" || operation == "broadcast" {
		auditRooms = rooms
	}

	for _, linked := range auditRooms {
		if err = roomAudit(ctx, tx, linked.Name, actor, "group-"+operation, value); err != nil {
			return nil, queryError(ctx, err)
		}
	}

	return rooms, queryError(ctx, tx.Commit())
}

func (s *Store) PlayRoomBot(ctx context.Context, room, user, expectedGame, result string, points int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	c, err := groupRoomControls(ctx, tx, room)
	if err != nil {
		return queryError(ctx, err)
	}
	if c.Bot == "" || c.Bot != expectedGame || c.BotExpires <= time.Now().UnixMilli() || c.BotRounds >= 20 {
		return ErrConflict
	}
	if expectedGame == "dice" && (points < 1 || points > 6) || expectedGame == "8ball" && points != 0 || !validCommunityText(result, 1000) {
		return ErrInvalid
	}
	if err = queries(tx).AdvanceRoomBot(ctx, room); err != nil {
		return queryError(ctx, err)
	}
	if err = queries(tx).AddRoomBotResult(ctx, sqlc.AddRoomBotResultParams{
		Room:      room,
		Username:  user,
		Game:      expectedGame,
		Result:    result,
		Points:    int64(points),
		CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func (s *Store) RoomBotScores(ctx context.Context, room string) ([]RoomBotScore, error) {
	rows, err := queries(s.db).ListRoomBotScores(ctx, room)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	out := make([]RoomBotScore, 0, len(rows))
	for _, r := range rows {
		out = append(out, RoomBotScore{Username: r.Username, Rounds: int(r.Rounds), Points: int(r.Points)})
	}

	return out, nil
}
