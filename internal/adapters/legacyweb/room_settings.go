package legacyweb

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
)

func (s *Adapter) roomSettingsPage(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) pages.Result {
	room, err := s.rooms.Room(ctx, u.Query().Get("roomName"))
	if err != nil {
		return roomPageFailure(err)
	}

	mod, err := s.rooms.IsRoomModerator(ctx, room.Name, x.username)
	if err != nil {
		return roomPageFailure(err)
	}
	if room.Owner != x.username && (!mod || !x.rooms[room.Name]) {
		return serviceError("Room Settings", faults.ErrForbidden)
	}

	action := s.roomPageAction("setup") + "&manage=1&roomName=" + url.QueryEscape(room.Name)
	if method == "POST" {
		op, target, value, e := serviceMutation(x, form)
		if e != nil {
			return serviceError("Room Settings", e)
		}
		if target != room.Name {
			return serviceError("Room Settings", faults.ErrInvalid)
		}
		if op == "delete" {
			if value != room.Name {
				return serviceResult(400, "Delete Room", serviceText("Type the exact room name to confirm deletion."))
			}
			if err := s.workflow.ChangeRoom(ctx, x.id, x.csrf, room.Name, "delete", value); err != nil {
				return serviceError("Delete Room", err)
			}

			return serviceResult(200, "Room Deleted", serviceText(room.Name+" was deleted. Its history and moderation audit were retained."))
		}
		if e = s.workflow.ChangeRoom(ctx, x.id, x.csrf, room.Name, op, value); e != nil {
			return serviceError("Room Settings", e)
		}

		if op == "owner" {
			return serviceResult(
				200,
				"Room Ownership",
				pages.Stack(serviceText("Ownership transferred to "+value), serviceLink(s.roomPageAction("setup")+"&roomName="+url.QueryEscape(room.Name), "Room Info")),
			)
		}

		room, err = s.rooms.Room(ctx, room.Name)
		if err != nil {
			return roomPageFailure(err)
		}
	} else if method != "GET" {
		return serviceResult(405, "Room Settings", serviceText("Use GET or POST."))
	}

	settings, err := s.rooms.RoomSettings(ctx, room.Name)
	if err != nil {
		return roomPageFailure(err)
	}

	body := pages.Stack(
		serviceText(room.Name),
		serviceText("Language: "+settings.Language),
		serviceText("Keywords: "+settings.Keywords),
		serviceText("Description: "+room.Description),
		serviceText("Capacity: "+strconv.Itoa(room.Capacity)),
		serviceText("User vote-kicking enabled: "+strconv.FormatBool(settings.UserKicks)),
	)
	if room.Owner == x.username {
		for _, field := range [][2]string{
			{"language", "Primary Language"},
			{"description", "Description"},
			{"keywords", "Keywords"},
			{"capacity", "Capacity (2 to 100)"},
			{"kicks", "Allow user vote-kicking (1=yes, 0=no)"},
			{"moderator-add", "Add moderator username"},
			{"moderator-remove", "Remove moderator username"},
			{"owner", "Transfer ownership to username"},
		} {
			body = pages.Stack(body, serviceForm(action, x.csrf, field[0], room.Name, field[1], "Save "+field[1]))
		}
	}
	if room.Owner == x.username {
		body = pages.Stack(body, serviceForm(action, x.csrf, "delete", room.Name, "Type exact room name to delete", "Delete Room"))
	}

	body = pages.Stack(body,
		serviceForm(action, x.csrf, "ban", room.Name, "Ban username", "Ban User"),
		serviceForm(action, x.csrf, "unban", room.Name, "Unban username", "Unban User"),
	)

	bans, err := s.rooms.RoomBans(ctx, room.Name)
	if err != nil {
		return roomPageFailure(err)
	}

	body = pages.Stack(body, serviceText("Banned users: "+strings.Join(bans, ", ")))

	changes, err := s.rooms.RoomChanges(ctx, room.Name)
	if err != nil {
		return roomPageFailure(err)
	}

	for _, change := range changes {
		body = pages.Stack(body, serviceText(change.Actor+": "+change.Operation+" = "+change.Value))
	}

	if controls, e := s.rooms.GroupRoomControls(ctx, room.Name); e == nil {
		body = pages.Stack(
			body,
			serviceText("Group room locked: "+strconv.FormatBool(controls.Locked)+"; silenced: "+strconv.FormatBool(controls.Silenced)),
			serviceText("Bot: "+controls.Bot+"; rounds: "+strconv.Itoa(controls.BotRounds)+" / 20"),
		)
		scores, e := s.rooms.RoomBotScores(ctx, room.Name)
		if e != nil {
			return roomPageFailure(e)
		}

		for _, score := range scores {
			body = pages.Stack(body, serviceText(score.Username+": "+strconv.Itoa(score.Points)+" dice points in "+strconv.Itoa(score.Rounds)+" rounds"))
		}
	}

	return serviceResult(200, "Room Settings", body)
}
