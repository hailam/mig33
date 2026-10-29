package pages

import "encoding/hex"

type RoomMuteTarget struct {
	Username string
	Muted    bool
}

type RoomManagement struct {
	Action, CSRF, Binding string
	Targets               []RoomMuteTarget
}

func roomModerationFields(room string, management RoomManagement, target RoomMuteTarget) []HiddenField {
	operation := "mute"
	if target.Muted {
		operation = "unmute"
	}
	return []HiddenField{
		{Name: "csrf", Value: management.CSRF},
		{Name: "room", Value: hex.EncodeToString([]byte(room))},
		{Name: "binding", Value: management.Binding},
		{Name: "target", Value: target.Username},
		{Name: "operation", Value: operation},
	}
}

func roomModerationLabel(target RoomMuteTarget) string {
	if target.Muted {
		return "Unmute " + target.Username
	}
	return "Mute " + target.Username
}
