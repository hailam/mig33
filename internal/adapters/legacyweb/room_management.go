package legacyweb

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"net/url"

	"github.com/hailam/mig33/internal/pages"
	"github.com/hailam/mig33/internal/rooms"
)

// Room Info's moderator controls reuse the packet service's authorization and
// persistence. They are reconstructed HTML; no historical form was recovered.
func (s *Adapter) roomInfoPage(ctx context.Context, x *actor, method string, room rooms.Room, form url.Values) pages.Result {
	moderator, err := s.rooms.IsRoomModerator(ctx, room.Name, x.username)
	if err != nil {
		return roomPageFailure(err)
	}

	canModerate := moderator && x.rooms[room.Name]
	notice := ""
	if method == "POST" {
		if !canModerate {
			return pages.RoomNotice(403, "Room Moderation", "Join this room as a moderator to manage its members.")
		}
		if !roomFormFields(form, "csrf", "room", "binding", "target", "operation") {
			return pages.RoomNotice(400, "Room Moderation", "Invalid moderation form. Open Room Info again.")
		}
		if x.csrf == "" || subtle.ConstantTimeCompare([]byte(x.csrf), []byte(form.Get("csrf"))) != 1 ||
			form.Get("room") != hex.EncodeToString([]byte(room.Name)) ||
			subtle.ConstantTimeCompare([]byte(reportBinding(x.csrf, "room-moderation", "room", room.Name)), []byte(form.Get("binding"))) != 1 {
			return pages.RoomNotice(403, "Room Moderation", "This form has expired or changed. Open Room Info again.")
		}

		muted := false
		switch form.Get("operation") {
		case "mute":
			muted = true
		case "unmute":
		default:
			return pages.RoomNotice(400, "Room Moderation", "Choose Mute or Unmute.")
		}

		var err error
		notice, err = s.workflow.ModerateRoom(ctx, x.id, x.csrf, room.Name, form.Get("target"), muted)
		if err != nil {
			return pages.RoomNotice(409, "Room Moderation", err.Error())
		}

	}

	view := s.roomView(ctx, room)
	view.Notice = notice
	view.Moderators, err = s.rooms.RoomModerators(ctx, room.Name)
	if err != nil {
		return roomPageFailure(err)
	}
	if canModerate {
		muted, err := s.rooms.RoomMuted(ctx, room.Name)
		if err != nil {
			return roomPageFailure(err)
		}

		management := &pages.RoomManagement{
			Action: view.InfoURL, CSRF: x.csrf,
			Binding: reportBinding(x.csrf, "room-moderation", "room", room.Name),
		}
		for _, target := range s.workflow.Participants(ctx, room.Name) {
			if target != x.username && !roomHasUser(view.Moderators, target) {
				management.Targets = append(management.Targets, pages.RoomMuteTarget{Username: target, Muted: roomHasUser(muted, target)})
			}
		}

		view.Management = management
	}
	if room.Owner == x.username || canModerate {
		view.SettingsURL = view.InfoURL + "&manage=1"
	}

	return pages.RoomInfo(view)
}
