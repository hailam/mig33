package repositories

import (
	"context"
	"fmt"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
	"github.com/hailam/mig33/internal/rooms"
)

// Room is a public room definition. Presence and membership are session-scoped
// and belong to the server; opening this store never rejoins a room.
type Room = rooms.Room

// Rooms returns room definitions sorted by name. Category zero selects all
// categories; a positive category filters the result. A negative value is invalid.
func (s *Store) Rooms(ctx context.Context, category int16) ([]Room, error) {
	if category < 0 {
		return nil, fmt.Errorf("%w: room category", ErrInvalid)
	}

	rows, err := queries(s.db).ListRooms(ctx, int64(category))
	if err != nil {
		return nil, queryError(ctx, err)
	}

	return roomsFromRows(rows), nil
}

func roomFromRow(r sqlc.ChatRoom) Room {
	return Room{Name: r.Name, Category: int16(r.Category), Capacity: int(r.Capacity), Description: r.Description, Owner: r.Owner}
}

func roomsFromRows(rows []sqlc.ChatRoom) []Room {
	out := make([]Room, 0, len(rows))
	for _, r := range rows {
		out = append(out, roomFromRow(r))
	}

	return out
}

// Room looks up a room without case sensitivity and returns its canonical name.
func (s *Store) Room(ctx context.Context, name string) (Room, error) {
	return readRoom(ctx, s.db, name)
}

type rowQuerier = sqlc.DBTX

func readRoom(ctx context.Context, db rowQuerier, name string) (Room, error) {
	if err := groupID(name); err != nil {
		return Room{}, fmt.Errorf("room name: %w", err)
	}

	r, err := queries(db).GetRoom(ctx, name)
	return roomFromRow(r), queryError(ctx, err)
}
