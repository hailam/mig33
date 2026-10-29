package repositories

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
	"github.com/hailam/mig33/internal/rooms"
)

// These are local reconstruction limits, not recovered historical policy.
const (
	maxOwnedRooms       = 10
	maxRoomFavorites    = 100
	createdRoomCapacity = 50
)

// MaxCreatedRoomNameLength follows the recovered Create Chatroom page. Existing
// room names retain the earlier 40-character compatibility range.
const MaxCreatedRoomNameLength = rooms.MaxCreatedRoomNameLength

// NormalizeRoomName trims outer whitespace and validates the restricted name
// alphabet used by existing rooms and the original native action links.
func NormalizeRoomName(name string) (string, error) { return rooms.NormalizeRoomName(name) }

// CreateRoom creates a public category1 room and grants its creator a room
// moderator role atomically. Name lookup is case insensitive. An identical
// retry by the same owner returns the existing room without restoring roles
// that an operator may subsequently have revoked.
func (s *Store) CreateRoom(ctx context.Context, owner, name, description string) (Room, error) {
	owner, err := username(owner)
	if err != nil {
		return Room{}, queryError(ctx, err)
	}

	name, err = NormalizeRoomName(name)
	if err != nil || len(description) > 1000 || !utf8.ValidString(description) || strings.ContainsRune(description, '\x00') {
		return Room{}, ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Room{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	existing, err := readRoom(ctx, tx, name)
	if err == nil {
		if existing.Owner != owner || existing.Description != description {
			return Room{}, ErrConflict
		}

		return existing, queryError(ctx, tx.Commit())
	}
	if !errors.Is(err, ErrNotFound) {
		return Room{}, queryError(ctx, err)
	}
	if len(name) > MaxCreatedRoomNameLength {
		return Room{}, ErrInvalid
	}

	deleted, err := queries(tx).RoomTombstoned(ctx, name)
	if err != nil {
		return Room{}, queryError(ctx, err)
	}
	if deleted {
		return Room{}, ErrConflict
	}

	owned, err := queries(tx).CountOwnedRooms(ctx, owner)
	if err != nil {
		return Room{}, queryError(ctx, err)
	}
	if owned >= maxOwnedRooms {
		return Room{}, ErrLimit
	}
	if err := queries(tx).CreateRoomDefinition(ctx, sqlc.CreateRoomDefinitionParams{
		Name:        name,
		Capacity:    createdRoomCapacity,
		Description: description,
		Owner:       owner,
		CreatedAt:   time.Now().UnixMilli(),
	}); err != nil {
		return Room{}, queryError(ctx, err)
	}
	if err := queries(tx).AddRoomModerator(ctx, sqlc.AddRoomModeratorParams{Room: name, Username: owner}); err != nil {
		return Room{}, queryError(ctx, err)
	}

	room, err := readRoom(ctx, tx, name)
	if err != nil {
		return Room{}, queryError(ctx, err)
	}

	return room, queryError(ctx, tx.Commit())
}

// RoomSearchFilter keeps literal text, language and keyword matching independent.
// Query matches name/description; Keywords matches saved keywords; Language is
// an exact case-insensitive language name. Empty fields impose no restriction.
type RoomSearchFilter = rooms.RoomSearchFilter

func (s *Store) SearchRooms(ctx context.Context, query string, limit int) ([]Room, error) {
	return s.SearchRoomsFiltered(ctx, RoomSearchFilter{Query: query}, limit)
}

func (s *Store) SearchRoomsFiltered(ctx context.Context, filter RoomSearchFilter, limit int) ([]Room, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Language = strings.TrimSpace(filter.Language)
	filter.Keywords = strings.TrimSpace(filter.Keywords)
	for _, v := range []string{filter.Query, filter.Language, filter.Keywords} {
		if len(v) > 128 || !utf8.ValidString(v) || strings.ContainsRune(v, 0) {
			return nil, ErrInvalid
		}
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := queries(s.db).SearchRoomDefinitions(ctx, sqlc.SearchRoomDefinitionsParams{
		Query:       filter.Query,
		Language:    filter.Language,
		Keywords:    filter.Keywords,
		ResultLimit: int64(limit),
	})
	if err != nil {
		return nil, queryError(ctx, err)
	}

	return roomsFromRows(rows), nil
}

// FavoriteRooms returns only this account's saved rooms. Definitions retain
// their global category; the client adapter supplies the Favorites category.
func (s *Store) FavoriteRooms(ctx context.Context, owner string) ([]Room, error) {
	owner, err := username(owner)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	rows, err := queries(s.db).FavoriteRoomDefinitions(ctx, owner)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	return roomsFromRows(rows), nil
}

// AddRoomFavorite is idempotent, canonicalizes room names and applies the
// per-account limit within the same transaction as insertion.
func (s *Store) AddRoomFavorite(ctx context.Context, owner, room string) (Room, error) {
	owner, err := username(owner)
	if err != nil {
		return Room{}, queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Room{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	definition, err := readRoom(ctx, tx, room)
	if err != nil {
		return Room{}, queryError(ctx, err)
	}

	exists, err := queries(tx).IsFavoriteRoom(ctx, sqlc.IsFavoriteRoomParams{Owner: owner, Room: definition.Name})
	if err != nil {
		return Room{}, queryError(ctx, err)
	}
	if exists {
		return definition, queryError(ctx, tx.Commit())
	}

	count, err := queries(tx).CountFavoriteRooms(ctx, owner)
	if err != nil {
		return Room{}, queryError(ctx, err)
	}
	if count >= maxRoomFavorites {
		return Room{}, ErrLimit
	}
	if err := queries(tx).AddFavoriteRoom(ctx, sqlc.AddFavoriteRoomParams{Owner: owner, Room: definition.Name}); err != nil {
		return Room{}, queryError(ctx, err)
	}

	return definition, queryError(ctx, tx.Commit())
}

// RemoveRoomFavorite removes only the specified owner's bookmark. Repeating
// removal succeeds; an unknown room remains an ErrNotFound rather than a delete.
func (s *Store) RemoveRoomFavorite(ctx context.Context, owner, room string) error {
	owner, err := username(owner)
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
	if err := queries(tx).RemoveFavoriteRoom(ctx, sqlc.RemoveFavoriteRoomParams{Owner: owner, Room: definition.Name}); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}
