package realtime

import (
	"context"
	"fmt"
	"time"

	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/rooms"
)

// The recovered client supplies706 for both regular users and moderators.
// The occupancy bands follow the 2010 community table in docs/rooms.md;
// ballot timing and snapshot behavior remain local reconstruction policy.
const (
	kickVoteWindow   = rooms.KickVoteWindow
	kickVoteCooldown = rooms.KickVoteCooldown
)

func kickVotesForOccupancy(users int) int { return rooms.KickVotesForOccupancy(users) }

type voteInitiator struct{ Room, Username string }

type kickVote struct {
	Target, Initiator string
	Voters            map[string]bool
	Required          int
	ExpiresAt         time.Time
}

// All helpers below run under Server.mu, including lifecycle-owned expiry work.
func (s *Coordinator) roomKickVote(ctx context.Context, x *session, request *commands.Request, room, target string) []*events.Event {
	settings, err := s.roomsService.RoomSettings(ctx, room)
	if err != nil {
		return []*events.Event{roomFailure(request, err)}
	}
	if !settings.UserKicks {
		return []*events.Event{failure(request, "User vote-kicking is disabled in this room")}
	}

	now := time.Now()
	if err := s.pruneKickVote(ctx, room, now); err != nil {
		return []*events.Event{roomFailure(request, err)}
	}

	muted, err := s.roomsService.IsRoomMuted(ctx, room, x.username)
	if err != nil {
		return []*events.Event{roomFailure(request, err)}
	}
	if muted {
		return []*events.Event{failure(request, "Muted users cannot start or join a kick vote")}
	}

	ack := events.Ack(request.Correlation)
	vote := s.kickVotes[room]
	if vote != nil {
		if vote.Target != target {
			return []*events.Event{failure(request, "A vote to kick "+vote.Target+" is already active in this room")}
		}
		if vote.Voters[x.username] {
			return []*events.Event{ack, roomSystemPacket(room, fmt.Sprintf("You have already voted to kick %s (%d/%d).", target, len(vote.Voters), vote.Required))}
		}

		event := "vote-cast"
		if len(vote.Voters)+1 >= vote.Required {
			event = "vote-kick"
		}
		if err := s.roomsService.RecordRoomEvent(ctx, room, x.username, event, fmt.Sprintf("%s %d/%d", target, len(vote.Voters)+1, vote.Required)); err != nil {
			return []*events.Event{roomFailure(request, err)}
		}

		vote.Voters[x.username] = true
		if len(vote.Voters) >= vote.Required {
			s.clearKickVote(room)
			s.roomBroadcast(room, roomSystemPacket(room, fmt.Sprintf("%s was kicked by a room vote (%d/%d).", target, len(vote.Voters), vote.Required)), "")
			s.performRoomKick(ctx, room, target, "a room vote")
		} else {
			s.roomBroadcast(room, roomSystemPacket(room, fmt.Sprintf("Vote to kick %s: %d/%d votes.", target, len(vote.Voters), vote.Required)), "")
		}
		return []*events.Event{ack}
	}

	for key, until := range s.voteCooldowns {
		if !now.Before(until) {
			delete(s.voteCooldowns, key)
		}
	}

	key := voteInitiator{room, x.username}
	if until := s.voteCooldowns[key]; now.Before(until) {
		return []*events.Event{failure(request, "Please wait before starting another kick vote")}
	}

	// Snapshot unique active room accounts, including target and initiator.
	// Later joins/leaves affect eligibility but never move this ballot's goal.
	vote = &kickVote{
		Target:    target,
		Initiator: x.username,
		Voters:    map[string]bool{x.username: true},
		Required:  kickVotesForOccupancy(len(s.roomParticipants(room))),
		ExpiresAt: now.Add(kickVoteWindow),
	}
	if err := s.roomsService.RecordRoomEvent(ctx, room, x.username, "vote-start", fmt.Sprintf("%s 1/%d", target, vote.Required)); err != nil {
		return []*events.Event{roomFailure(request, err)}
	}

	s.kickVotes[room] = vote
	s.voteCooldowns[key] = now.Add(kickVoteCooldown)
	s.wakeMaintenance()

	s.roomBroadcast(room, roomSystemPacket(room, fmt.Sprintf(
		"A vote to kick %s was started by %s: 1/%d votes. Use Menu > Kick User > %s to vote. Voting closes in 60 seconds.",
		target, x.username, vote.Required, target,
	)), "")
	return []*events.Event{ack}
}

func (s *Coordinator) clearKickVote(room string) {
	if vote := s.kickVotes[room]; vote != nil {
		delete(s.kickVotes, room)
	}
}

func (s *Coordinator) cancelKickVote(ctx context.Context, room, reason string) {
	if vote := s.kickVotes[room]; vote != nil {
		target := vote.Target
		if err := s.roomsService.RecordRoomEvent(ctx, room, "system", "vote-cancel", target+": "+reason); err != nil {
			s.logError(ctx, "record vote cancellation: %v", err)
		}
		s.clearKickVote(room)
		s.roomBroadcast(room, roomSystemPacket(room, "Vote to kick "+target+" cancelled: "+reason+"."), "")
	}
}

func (s *Coordinator) pruneKickVote(ctx context.Context, room string, now time.Time) error {
	vote := s.kickVotes[room]
	if vote == nil {
		return nil
	}
	if !now.Before(vote.ExpiresAt) {
		s.cancelKickVote(ctx, room, "the voting window expired")
		return nil
	}

	participants := s.roomParticipants(room)
	if !roomHasUser(participants, vote.Target) {
		s.cancelKickVote(ctx, room, "the target left the room")
		return nil
	}

	moderator, err := s.roomsService.IsRoomModerator(ctx, room, vote.Target)
	if err != nil {
		return err
	}
	if moderator {
		s.cancelKickVote(ctx, room, "the target cannot be voted out")
		return nil
	}

	for voter := range vote.Voters {
		eligible := roomHasUser(participants, voter)
		if eligible {
			muted, err := s.roomsService.IsRoomMuted(ctx, room, voter)
			if err != nil {
				return err
			}
			eligible = !muted
		}
		if !eligible {
			if err := s.roomsService.RecordRoomEvent(ctx, room, voter, "vote-withdraw", vote.Target+": no longer eligible"); err != nil {
				s.logError(ctx, "record vote withdrawal: %v", err)
			}
			delete(vote.Voters, voter)
		}
	}

	if len(vote.Voters) == 0 {
		s.cancelKickVote(ctx, room, "no voters remain in the room")
	}
	return nil
}

func (s *Coordinator) kickVoteMemberLeft(ctx context.Context, room, username string) {
	vote := s.kickVotes[room]
	if vote == nil {
		return
	}
	if vote.Target == username {
		s.cancelKickVote(ctx, room, "the target left the room")
		return
	}
	if vote.Voters[username] {
		if err := s.roomsService.RecordRoomEvent(ctx, room, username, "vote-withdraw", vote.Target); err != nil {
			s.logError(ctx, "record vote withdrawal: %v", err)
		}
		delete(vote.Voters, username)
		if len(vote.Voters) == 0 {
			s.cancelKickVote(ctx, room, "no voters remain in the room")
		} else {
			s.roomBroadcast(room, roomSystemPacket(room, fmt.Sprintf("Vote to kick %s: %d/%d votes; %s left the room.", vote.Target, len(vote.Voters), vote.Required, username)), "")
		}
	}
}

func (s *Coordinator) performRoomKick(ctx context.Context, room, target, actor string) {
	for _, other := range s.sessions {
		if other.username != target || !other.rooms[room] {
			continue
		}
		s.roomLeave(ctx, other, room)
		// The original client has a notification popup, but no recovered
		// room-only command to force its tab closed. Membership is enforced.
		if other.authenticated && !other.webOnly && time.Since(other.touched) < sessionTTL {
			popup := events.New(0, events.RoomKicked{Room: room, Actor: actor})
			select {
			case other.push <- popup:
			default:
			}
		}
	}
}
