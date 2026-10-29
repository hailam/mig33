package rooms

import (
	"strings"

	"github.com/hailam/mig33/internal/faults"
)

type Room struct {
	Name        string `json:"name"`
	Category    int16  `json:"category"`
	Capacity    int    `json:"capacity"`
	Description string `json:"description"`
	Owner       string `json:"owner,omitempty"`
}

const MaxCreatedRoomNameLength = 15

func NormalizeRoomName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if len(name) == 0 || len(name) > 40 {
		return "", faults.ErrInvalid
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != ' ' && c != '_' && c != '-' {
			return "", faults.ErrInvalid
		}
	}
	return name, nil
}

type RoomSearchFilter struct{ Query, Language, Keywords string }

type RoomSettings struct {
	Language, Keywords string
	UserKicks          bool
}

type RoomChange struct {
	ID, CreatedAt           int64
	Actor, Operation, Value string
}

type RoomCategory struct {
	ID   int16
	Name string
}

type GroupRoomControls struct {
	GroupID          int64
	Silenced, Locked bool
	Bot              string
	BotExpires       int64
	BotRounds        int
}

type RoomBotScore struct {
	Username       string
	Rounds, Points int
}
