package events

import "github.com/hailam/mig33/internal/rooms"

type RoomCategory struct {
	ID       int16
	Name     string
	Favorite bool
}

type (
	RoomCategoriesComplete struct{}
	RoomListingComplete    struct{ Action string }
	Room                   struct {
		Room         rooms.Room
		Participants int
		Category     int16
	}
)

type (
	RoomParticipants struct{ Participants, Moderators, Muted []string }
	RoomKicked       struct{ Room, Actor string }
)
