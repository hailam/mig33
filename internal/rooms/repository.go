package rooms

import (
	"context"
)

// Repository is the persistence capability required by rooms.
// Implementations own transactions and must not expose driver or generated query types.
type Repository interface {
	IsRoomModerator(ctx context.Context, room, user string) (bool, error)
	RoomModerators(ctx context.Context, room string) ([]string, error)
	IsRoomMuted(ctx context.Context, room, user string) (bool, error)
	RoomMuted(ctx context.Context, room string) ([]string, error)
	SetRoomMuted(ctx context.Context, room, actor, target string, muted bool) error
	SetRoomModerator(ctx context.Context, room, user string, enabled bool) error
	RoomCategories(ctx context.Context) ([]RoomCategory, error)
	ManageRoomCategory(ctx context.Context, actor string, id int16, name string, remove bool) error
	SetRoomCategory(ctx context.Context, actor, room string, id int16) error
	DeleteOwnedRoom(ctx context.Context, room, actor string) error
	RecordRoomEvent(ctx context.Context, room, actor, operation, value string) error
	GroupRoomControls(ctx context.Context, room string) (GroupRoomControls, error)
	GroupRoomAccess(ctx context.Context, room, user string) (canJoin, canSpeak bool, err error)
	ManageGroupRoom(ctx context.Context, room, actor, operation, value string) ([]Room, error)
	PlayRoomBot(ctx context.Context, room, user, expectedGame, result string, points int) error
	RoomBotScores(ctx context.Context, room string) ([]RoomBotScore, error)
	CreateRoom(ctx context.Context, owner, name, description string) (Room, error)
	SearchRooms(ctx context.Context, query string, limit int) ([]Room, error)
	SearchRoomsFiltered(ctx context.Context, filter RoomSearchFilter, limit int) ([]Room, error)
	FavoriteRooms(ctx context.Context, owner string) ([]Room, error)
	AddRoomFavorite(ctx context.Context, owner, room string) (Room, error)
	RemoveRoomFavorite(ctx context.Context, owner, room string) error
	RoomSettings(ctx context.Context, room string) (RoomSettings, error)
	RoomBanned(ctx context.Context, room, user string) (bool, error)
	RoomBans(ctx context.Context, room string) ([]string, error)
	RoomChanges(ctx context.Context, room string) ([]RoomChange, error)
	ManageRoom(ctx context.Context, room, actor, operation, value string) error
	Rooms(ctx context.Context, category int16) ([]Room, error)
	Room(ctx context.Context, name string) (Room, error)
}
