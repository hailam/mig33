package realtime

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/rooms"
)

// roomControlCommand handles read-only game discovery and the local, explicitly
// scoped Group-room command contract. Slash text is never forwarded and side effects never depend on
// treating a user-supplied message as a privileged server action.
func (s *Coordinator) roomControlCommand(ctx context.Context, x *session, p *commands.Request, m chat.Message) (*events.Event, bool) {
	name, args, handled := rooms.ParseControl(m.Body)
	if !handled {
		return nil, false
	}
	fail := func(message string) (*events.Event, bool) { return failure(p, message), true }
	if m.Kind != 3 {
		return fail("This command requires a community Group-linked chat room")
	}
	room, err := s.roomsService.Room(ctx, m.Conversation)
	if err != nil || !x.rooms[room.Name] {
		return fail("Join the chat room before using commands")
	}
	if name == "games" {
		if args != "" {
			return fail("Usage: /games")
		}
		if x.webOnly {
			return fail("Open migWorld > Games to browse the catalog")
		}
		if len(x.push) == cap(x.push) {
			return fail("Your connection is busy; try again")
		}
		// Discovery is private to the requesting session and read-only. It is
		// available in ordinary joined rooms without Group moderator powers.
		x.push <- roomSystemPacket(room.Name, catalog.GameNotice())
		return events.Ack(p.Correlation), true
	}
	controls, err := s.roomsService.GroupRoomControls(ctx, room.Name)
	if err != nil {
		return fail("This command requires a community Group-linked chat room")
	}
	muted, err := s.roomsService.IsRoomMuted(ctx, room.Name, x.username)
	if err != nil || muted {
		return fail("You are muted or room permissions are unavailable")
	}
	_, canSpeak, err := s.roomsService.GroupRoomAccess(ctx, room.Name, x.username)
	if err != nil {
		return fail("Room permissions could not be checked")
	}
	if name == "scores" {
		if args != "" {
			return fail("Usage: /scores")
		}
		if len(x.push) == cap(x.push) {
			return fail("Your connection is busy; try again")
		}
		scores, e := s.roomsService.RoomBotScores(ctx, room.Name)
		if e != nil {
			return fail("Scores could not be loaded")
		}
		body := "Local dice leaderboard:"
		for i, score := range scores {
			body += fmt.Sprintf(" %d. %s %d points (%d rounds);", i+1, score.Username, score.Points, score.Rounds)
		}
		if len(scores) == 0 {
			body += " no completed dice rounds."
		}
		x.push <- roomSystemPacket(room.Name, body)
		return events.Ack(p.Correlation), true
	}
	if name == "play" {
		if !canSpeak {
			return fail("This Group room is silenced")
		}
		if controls.Bot == "" || controls.BotExpires <= time.Now().UnixMilli() || controls.BotRounds >= 20 {
			return fail("No active bot. A Group moderator can start /bot dice or /bot 8ball")
		}
		if controls.Bot == "dice" && args != "" {
			return fail("Usage: /play (dice takes no arguments)")
		}
		if controls.Bot == "8ball" && (args == "" || len(args) > 320) {
			return fail("Usage: /play <question up to 320 bytes>")
		}
		if !s.roomControlQueuesReady([]rooms.Room{room}) {
			return fail("A room connection is busy; try again")
		}
		count := 6
		if controls.Bot == "8ball" {
			count = len(chat.EightBallResponses())
		}
		n, e := rand.Int(rand.Reader, big.NewInt(int64(count)))
		if e != nil {
			return fail("A game result could not be generated")
		}
		points := int(n.Int64()) + 1
		result := fmt.Sprintf("%s rolled %d on the local dice bot.", x.username, points)
		if controls.Bot == "8ball" {
			points = 0
			result = x.username + " asked " + args + " — " + chat.EightBallResponses()[n.Int64()]
		}
		if e = s.roomsService.PlayRoomBot(ctx, room.Name, x.username, controls.Bot, result, points); e != nil {
			return fail("Bot round could not be saved; it may have expired")
		}
		s.roomBroadcast(room.Name, events.New(0, events.Message{Message: chat.Message{Kind: 3, Conversation: room.Name, Sender: x.username, Body: result, Action: true}}), "")
		return events.Ack(p.Correlation), true
	}
	role, err := s.communityService.Role(ctx, controls.GroupID, x.username)
	if err != nil || (role != "owner" && role != "moderator") {
		return fail("Only this community Group's owner or moderators can use this command")
	}
	operation, value := name, args
	switch name {
	case "silence", "lock":
		switch strings.ToLower(args) {
		case "", "on":
			value = "1"
		case "off":
			value = "0"
		default:
			return fail("Usage: /" + name + " [on|off]")
		}
	case "unsilence", "unlock":
		if args != "" {
			return fail("Usage: /" + name)
		}
		value = "0"
		operation = "silence"
		if name == "unlock" {
			operation = "lock"
		}
	case "bot":
		if args != "dice" && args != "8ball" {
			return fail("Usage: /bot dice or /bot 8ball (20 rounds or 30 minutes)")
		}
	case "botstop":
		if args != "" {
			return fail("Usage: /botstop")
		}
	case "ban", "unban", "invite":
		value = strings.ToLower(args)
		if len(strings.Fields(args)) != 1 || s.accounts[value] == nil || value == x.username {
			return fail("Use one other registered username")
		}
		if name == "invite" && (!s.canContact(ctx, x.username, value) || s.blocksIncoming(ctx, value, x.username)) {
			return fail("Recipient privacy does not allow this invitation")
		}
	case "announce", "broadcast", "description":
		if args == "" || len(args) > 1000 {
			return fail("Use 1 to 1000 bytes of text")
		}
	}
	linkedRooms, err := s.communityService.Rooms(ctx, x.username, controls.GroupID)
	if err != nil {
		return fail("Linked rooms could not be loaded")
	}
	affected := []rooms.Room{room}
	if name == "broadcast" || name == "ban" || name == "unban" {
		affected = linkedRooms
	}
	if !s.roomControlQueuesReady(affected) {
		return fail("A room connection is busy; try again")
	}
	if _, err = s.roomsService.ManageGroupRoom(ctx, room.Name, x.username, operation, value); err != nil {
		if errors.Is(err, faults.ErrConflict) {
			return fail("A bot is already active; use /botstop first")
		}
		return fail("This Group-room operation is not permitted or could not be saved")
	}
	notice := fmt.Sprintf("%s set %s to %s.", x.username, operation, value)
	switch name {
	case "announce", "broadcast":
		notice = x.username + " announces: " + value
	case "ban":
		notice = value + " was banned from this Group's linked rooms by " + x.username + "."
		for _, linked := range linkedRooms {
			if vote := s.kickVotes[linked.Name]; vote != nil && vote.Target == value {
				s.cancelKickVote(ctx, linked.Name, "the target was banned")
			}
			s.performRoomKick(ctx, linked.Name, value, x.username)
		}
	case "unban":
		notice = value + " was unbanned from this Group's linked rooms by " + x.username + "."
	case "invite":
		notice = "An invitation for " + value + " is saved in local Mail. Joining still requires room access."
	case "bot":
		notice = x.username + " started the local " + value + " bot. Use /play; limit 20 rounds or 30 minutes. /scores shows local dice totals."
	case "botstop":
		notice = x.username + " stopped the room bot."
	}
	if operation == "lock" && value == "1" {
		// Lock only controls subsequent entry. Existing guests remain until they
		// leave; a ban is the explicit removal operation.
		notice = x.username + " locked new entry to approved Group members and moderators."
	}
	for _, linked := range affected {
		s.roomBroadcast(linked.Name, roomSystemPacket(linked.Name, notice), "")
	}
	return events.Ack(p.Correlation), true
}

func (s *Coordinator) roomControlQueuesReady(rooms []rooms.Room) bool {
	for _, room := range rooms {
		for _, other := range s.sessions {
			if other.authenticated && !other.webOnly && other.rooms[room.Name] && time.Since(other.touched) < sessionTTL && len(other.push) == cap(other.push) {
				return false
			}
		}
	}
	return true
}
