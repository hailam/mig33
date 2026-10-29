package commands

type RoomOperation string

const (
	RoomCategories     RoomOperation = "categories"
	RoomList           RoomOperation = "list"
	RoomJoin           RoomOperation = "join"
	RoomLeave          RoomOperation = "leave"
	RoomKick           RoomOperation = "kick"
	RoomParticipants   RoomOperation = "participants"
	RoomMute           RoomOperation = "mute"
	RoomUnmute         RoomOperation = "unmute"
	RoomFavoriteAdd    RoomOperation = "favorite_add"
	RoomFavoriteRemove RoomOperation = "favorite_remove"
)

type Room struct {
	Operation    RoomOperation
	Name, Target Field[string]
	Category     Field[int16]
	Refresh      Field[byte]
}
