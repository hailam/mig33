package realtime

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/rooms"
)

// roomOperation handles public native chat rooms. Room presence belongs to a
// session; the room definitions and message history belong to the durable store.
// All helpers in this file run with Server.mu held.
func (s *Coordinator) roomOperation(ctx context.Context, x *session, p *commands.Request) []*events.Event {
	input := p.Input.(commands.Room)
	switch input.Operation {
	case commands.RoomCategories:
		x.knownFavorites = nil
		categories, err := s.roomsService.RoomCategories(ctx)
		if err != nil {
			return []*events.Event{roomFailure(p, err)}
		}

		out := make([]*events.Event, 0, len(categories)+2)
		for _, category := range categories {
			out = append(out, events.New(p.Correlation, events.RoomCategory{ID: category.ID, Name: category.Name}))
		}
		out = append(out, events.New(p.Correlation, events.RoomCategory{ID: 2, Name: "Favorites", Favorite: true}), events.New(p.Correlation, events.RoomCategoriesComplete{}))
		return out
	case commands.RoomList:
		category, ok := input.Category.Get()
		flag, flagOK := input.Refresh.Get()
		if !ok || category < 1 || !flagOK || flag > 1 {
			return []*events.Event{failure(p, "Invalid room category request")}
		}

		listed, err := s.roomsService.List(ctx, x.username, category)
		if err != nil {
			return []*events.Event{roomFailure(p, err)}
		}

		out := make([]*events.Event, 0, len(listed)+1)
		if category == 2 {
			x.knownFavorites = make(map[string]bool, len(listed))
		}
		for _, room := range listed {
			out = append(out, s.roomEntryCategory(room, p.Correlation, category))
			if category == 2 {
				x.knownFavorites[room.Name] = true
			}
		}
		// Both categories return full snapshots. Their flag3 makes the native
		// footer action clear previous rows before requesting another snapshot.
		return append(out, events.New(p.Correlation, events.RoomListingComplete{Action: "Refresh"}))
	case commands.RoomFavoriteAdd, commands.RoomFavoriteRemove:
		return s.roomFavorite(ctx, x, p)
	}

	name, ok := input.Name.Get()
	if !ok || name == "" {
		return []*events.Event{failure(p, "A room name is required")}
	}

	room, err := s.roomsService.Room(ctx, name)
	if err != nil {
		return []*events.Event{roomFailure(p, err)}
	}

	ack := events.Ack(p.Correlation)
	switch input.Operation {
	case commands.RoomJoin:
		participants := s.roomParticipants(room.Name)
		if err := s.roomsService.CheckJoin(ctx, x.username, room, x.rooms[room.Name], participants); err != nil {
			return []*events.Event{roomFailure(p, err)}
		}
		controls, err := s.roomsService.GroupRoomControls(ctx, room.Name)
		if err != nil && !errors.Is(err, faults.ErrNotFound) {
			return []*events.Event{roomFailure(p, err)}
		}
		if x.rooms[room.Name] {
			// 701 updates an existing native tab. The ACK still completes703.
			return []*events.Event{s.joinedRoomEntry(room, p.Correlation, controls.GroupID), ack}
		}

		alreadyPresent := roomHasUser(participants, x.username)

		if x.rooms == nil {
			x.rooms = make(map[string]bool)
		}
		x.rooms[room.Name] = true
		if !alreadyPresent {
			s.roomBroadcast(room.Name, roomSystemPacket(room.Name, x.username+" joined the room."), x.id)
		}

		welcome := "Welcome to " + room.Name + "."
		if room.Description != "" {
			welcome += " " + room.Description
		}
		// 701 does not remove the pending703. ACK must follow it, and the
		// welcome follows both in this response, never in a competing HTTP poll.
		return []*events.Event{
			s.joinedRoomEntry(room, p.Correlation, controls.GroupID), ack, roomSystemPacket(room.Name, welcome),
		}
	case commands.RoomLeave:
		if !x.rooms[room.Name] {
			return []*events.Event{failure(p, "Join the chat room first")}
		}
		s.roomLeave(ctx, x, room.Name)
		return []*events.Event{ack}
	case commands.RoomKick, commands.RoomMute, commands.RoomUnmute:
		return s.roomModeration(ctx, x, p, room.Name)
	case commands.RoomParticipants:
		if !x.rooms[room.Name] {
			return []*events.Event{failure(p, "Join the chat room first")}
		}

		moderators, err := s.roomsService.RoomModerators(ctx, room.Name)
		if err != nil {
			return []*events.Event{roomFailure(p, err)}
		}

		muted, err := s.roomsService.RoomMuted(ctx, room.Name)
		if err != nil {
			return []*events.Event{roomFailure(p, err)}
		}

		participants := s.roomParticipants(room.Name)
		return []*events.Event{events.New(p.Correlation, events.RoomParticipants{
			Participants: participants,
			Moderators:   roomRoleList(moderators, participants),
			Muted:        roomRoleList(muted, participants),
		})}
	default:
		return []*events.Event{failure(p, "Room operation is not implemented")}
	}
}

func (s *Coordinator) roomFavorite(ctx context.Context, x *session, p *commands.Request) []*events.Event {
	input := p.Input.(commands.Room)
	if input.Operation == commands.RoomFavoriteRemove {
		category, ok := input.Category.Get()
		if !ok || category != 2 {
			return []*events.Event{failure(p, "Choose a room from Favorites")}
		}
	}

	name, ok := input.Name.Get()
	if !ok || name == "" {
		return []*events.Event{failure(p, "A room name is required")}
	}

	room, err := s.roomsService.Room(ctx, name)
	if err != nil {
		return []*events.Event{roomFailure(p, err)}
	}
	if input.Operation == commands.RoomFavoriteRemove {
		if err := s.roomsService.RemoveRoomFavorite(ctx, x.username, room.Name); err != nil {
			return []*events.Event{roomFailure(p, err)}
		}
		delete(x.knownFavorites, room.Name)
		return []*events.Event{events.Ack(p.Correlation)}
	}

	room, err = s.roomsService.AddRoomFavorite(ctx, x.username, room.Name)
	if errors.Is(err, faults.ErrLimit) {
		return []*events.Event{failure(p, "Favorites is full; remove a room first")}
	}
	if err != nil {
		return []*events.Event{roomFailure(p, err)}
	}

	if x.knownFavorites[room.Name] {
		// The original711701 branch inserts without deduplicating loaded
		// rows. Its ordinary ACK branch also completes711, safely.
		return []*events.Event{events.Ack(p.Correlation)}
	}
	if x.knownFavorites == nil {
		x.knownFavorites = make(map[string]bool)
	}
	x.knownFavorites[room.Name] = true
	// 701 itself completes711 and inserts the favorite in the existing
	// category. This path must not use the global room category.
	return []*events.Event{s.roomEntryCategory(room, p.Correlation, 2)}
}

func roomRoleList(roles, participants []string) []string {
	var names []string
	for _, participant := range participants {
		if roomHasUser(roles, participant) {
			names = append(names, participant)
		}
	}
	return names
}

func (s *Coordinator) roomModeration(ctx context.Context, x *session, p *commands.Request, name string) []*events.Event {
	input := p.Input.(commands.Room)
	target, valid := input.Target.Get()
	target = strings.ToLower(target)
	present := valid && s.accounts[target] != nil && roomHasUser(s.roomParticipants(name), target)
	moderator, err := s.roomsService.CheckModeration(ctx, x.username, target, name, x.rooms[name], present, input.Operation == commands.RoomKick)
	if err != nil {
		return []*events.Event{roomFailure(p, err)}
	}

	ack := events.Ack(p.Correlation)
	if input.Operation == commands.RoomKick {
		if !moderator {
			return s.roomKickVote(ctx, x, p, name, target)
		}
		if vote := s.kickVotes[name]; vote != nil && vote.Target == target {
			s.cancelKickVote(ctx, name, "a moderator removed the target")
		}
		if err := s.roomsService.RecordRoomEvent(ctx, name, x.username, "kick", target); err != nil {
			return []*events.Event{roomFailure(p, err)}
		}
		s.performRoomKick(ctx, name, target, x.username)
		return []*events.Event{ack}
	}

	muted := input.Operation == commands.RoomMute
	changed, err := s.roomsService.SetMuted(ctx, name, x.username, target, muted)
	if err != nil {
		if errors.Is(err, faults.ErrForbidden) {
			return []*events.Event{failure(p, "You are not permitted to moderate this user")}
		}
		return []*events.Event{roomFailure(p, err)}
	}

	verb := "unmuted"
	if muted {
		verb = "muted"
	}
	if changed {
		s.roomBroadcast(name, roomSystemPacket(name, target+" was "+verb+" by "+x.username+"."), "")
	}
	// 709/710 support success text in ACK tag1. They do not consume an
	// unsolicited708; the participant selector refreshes on its next707.
	return []*events.Event{events.New(p.Correlation, events.Acknowledged{Message: contactStringPointer(target + " has been " + verb + " in " + name + ".")})}
}

func roomFailure(p *commands.Request, err error) *events.Event {
	var serviceError *faults.ServiceError
	if errors.As(err, &serviceError) {
		return failure(p, serviceError.Message)
	}
	if errors.Is(err, faults.ErrNotFound) || errors.Is(err, faults.ErrInvalid) {
		return failure(p, "Unknown chat room")
	}
	return failure(p, "The chat room could not be loaded; try again")
}

func (s *Coordinator) joinedRoomEntry(room rooms.Room, correlation int16, communityID int64) *events.Event {
	return events.New(correlation, events.Room{Room: room, Participants: len(s.roomParticipants(room.Name)), Category: room.Category, CommunityID: communityID})
}

func (s *Coordinator) roomEntryCategory(room rooms.Room, correlation, category int16) *events.Event {
	return events.New(correlation, events.Room{Room: room, Participants: len(s.roomParticipants(room.Name)), Category: category})
}

func roomSystemPacket(name, text string) *events.Event {
	return events.New(0, events.Notice{Kind: 3, Target: name, Text: text})
}

func roomHasUser(users []string, username string) bool { return rooms.HasUser(users, username) }

// roomParticipants reports unique active users, not a count of their sessions.
func (s *Coordinator) roomParticipants(name string) []string {
	users := make(map[string]bool)
	for _, x := range s.sessions {
		if x.authenticated && x.rooms[name] && time.Since(x.touched) < sessionTTL {
			users[x.username] = true
		}
	}
	names := make([]string, 0, len(users))
	for name := range users {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// roomBroadcast is for transient room controls. Durable chat text uses the
// delivery receipts and history path in Server.message instead.
func (s *Coordinator) roomBroadcast(name string, p *events.Event, excludeSessionID string) {
	for _, x := range s.sessions {
		if x.id != excludeSessionID && x.authenticated && !x.webOnly && x.rooms[name] && time.Since(x.touched) < sessionTTL {
			select {
			case x.push <- p:
			default:
			}
		}
	}
}

func (s *Coordinator) roomLeave(ctx context.Context, x *session, name string) {
	delete(x.rooms, name)
	s.cancelRoomDeliveries(x.id, name)
	if !roomHasUser(s.roomParticipants(name), x.username) {
		s.roomBroadcast(name, roomSystemPacket(name, x.username+" left the room."), x.id)
		s.kickVoteMemberLeft(ctx, name, x.username)
	}
}

// roomLeaveSession is called before deleting a disconnected/expired session.
// Another session of the same user keeps that user present in each shared room.
func (s *Coordinator) roomLeaveSession(ctx context.Context, x *session) {
	for name := range x.rooms {
		s.roomLeave(ctx, x, name)
	}
}
