package realtime

import (
	"strings"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/protocol"
)

func castVote(t *testing.T, s *Application, sid, room, target string, want int16) []*protocol.Packet {
	t.Helper()
	replies, _ := s.Handle(t.Context(), sid, protocol.New(706, 90).SetText(1, room).SetText(2, target))
	if len(replies) == 0 || replies[0].Opcode != want || replies[0].Transaction != 90 {
		t.Fatalf("kick vote %s/%s: %v", room, target, replies)
	}
	return replies
}

func voteRoom(t *testing.T, s *Application) map[string]string {
	t.Helper()
	ids := map[string]string{}
	for _, user := range []string{"alice", "bob", "carol", "dave"} {
		ids[user] = login(t, s, user)
		joinRoom(t, s, ids[user], "Lobby")
	}
	ids["hailam"] = loginHailam(t, s)
	joinRoom(t, s, ids["hailam"], "Lobby")
	for _, sid := range ids {
		drainGroupQueue(s, sid)
	}
	return ids
}

func TestVoteKickThreeUniqueUsersAndModeratorImmediateKick(t *testing.T) {
	s := newTestServer(t)
	ids := voteRoom(t, s)
	secondHailam := loginHailam(t, s)
	secondBob := login(t, s, "bob")
	joinRoom(t, s, secondHailam, "Lobby")
	joinRoom(t, s, secondBob, "Lobby")
	joinRoom(t, s, ids["bob"], "Gaming")
	castVote(t, s, ids["hailam"], "Lobby", "BOB", 1)
	if !s.sessions[ids["bob"]].rooms["Lobby"] || len(s.kickVotes["Lobby"].Voters) != 1 {
		t.Fatal("first vote kicked immediately")
	}
	r := castVote(t, s, secondHailam, "Lobby", "bob", 1)
	if len(s.kickVotes["Lobby"].Voters) != 1 || len(r) != 2 {
		t.Fatal("second session counted as a second voter")
	}
	castVote(t, s, ids["carol"], "Lobby", "bob", 1)
	if len(s.kickVotes["Lobby"].Voters) != 2 {
		t.Fatal("second distinct vote missing")
	}
	castVote(t, s, ids["dave"], "Lobby", "bob", 1)
	if s.kickVotes["Lobby"] != nil || s.sessions[ids["bob"]].rooms["Lobby"] || s.sessions[secondBob].rooms["Lobby"] || !s.sessions[ids["bob"]].rooms["Gaming"] {
		t.Fatal("vote did not remove only target's room memberships")
	}
	for _, sid := range []string{ids["bob"], secondBob} {
		found := false
		for _, p := range drainGroupQueue(s, sid) {
			if text, _ := p.Text(2); p.Opcode == 5 && strings.Contains(text, "a room vote") {
				found = true
			}
		}
		if !found {
			t.Fatal("kicked session did not receive native popup")
		}
	}

	joinRoom(t, s, ids["bob"], "Lobby")
	castVote(t, s, ids["alice"], "Lobby", "bob", 1)
	if s.sessions[ids["bob"]].rooms["Lobby"] {
		t.Fatal("moderator kick became a vote")
	}
}

func TestVoteKickExpiryCancellationAndCooldown(t *testing.T) {
	s := newTestServer(t)
	ids := voteRoom(t, s)
	castVote(t, s, ids["hailam"], "Lobby", "bob", 1)
	castVote(t, s, ids["carol"], "Lobby", "dave", 0) // One ballot per room.
	s.kickVotes["Lobby"].ExpiresAt = time.Now().Add(-time.Second)
	s.mu.Lock()
	s.pruneKickVote(t.Context(), "Lobby", time.Now())
	s.mu.Unlock()
	if s.kickVotes["Lobby"] != nil || !s.sessions[ids["bob"]].rooms["Lobby"] {
		t.Fatal("expired vote removed target")
	}
	castVote(t, s, ids["hailam"], "Lobby", "bob", 0) // Start cooldown survives expiry.
	castVote(t, s, ids["carol"], "Lobby", "bob", 1)
	s.Handle(t.Context(), ids["bob"], protocol.New(704, 1).SetText(1, "Lobby"))
	if s.kickVotes["Lobby"] != nil {
		t.Fatal("vote survived target departure")
	}

	joinRoom(t, s, ids["bob"], "Lobby")
	castVote(t, s, ids["dave"], "Lobby", "bob", 1)
	s.DropSession(t.Context(), ids["dave"])
	if s.kickVotes["Lobby"] != nil {
		t.Fatal("empty vote survived its last voter leaving")
	}
}

func TestVoteKickWithdrawsDepartedVoterAndProtectsModerators(t *testing.T) {
	s := newTestServer(t)
	ids := voteRoom(t, s)
	castVote(t, s, ids["hailam"], "Lobby", "alice", 0)
	castVote(t, s, ids["hailam"], "Lobby", "hailam", 0)
	castVote(t, s, ids["hailam"], "Lobby", "bob", 1)
	castVote(t, s, ids["carol"], "Lobby", "bob", 1)
	s.Handle(t.Context(), ids["carol"], protocol.New(704, 1).SetText(1, "Lobby"))
	castVote(t, s, ids["dave"], "Lobby", "bob", 1)
	if !s.sessions[ids["bob"]].rooms["Lobby"] || len(s.kickVotes["Lobby"].Voters) != 2 {
		t.Fatal("departed voter still counted")
	}
	if err := s.SetRoomModerator(t.Context(), "Lobby", "bob", true); err != nil {
		t.Fatal(err)
	}
	if s.kickVotes["Lobby"] != nil {
		t.Fatal("promotion did not cancel vote")
	}

	joinRoom(t, s, ids["carol"], "Lobby")
	castVote(t, s, ids["carol"], "Lobby", "bob", 0)
}

func TestMutedAndExpiredUsersCannotVote(t *testing.T) {
	s := newTestServer(t)
	ids := voteRoom(t, s)
	s.Handle(t.Context(), ids["alice"], protocol.New(709, 1).SetText(1, "Lobby").SetText(2, "hailam"))
	castVote(t, s, ids["hailam"], "Lobby", "bob", 0)
	castVote(t, s, ids["carol"], "Lobby", "bob", 1)
	s.sessions[ids["carol"]].touched = time.Now().Add(-sessionTTL - time.Second)
	castVote(t, s, ids["dave"], "Lobby", "bob", 1)
	if vote := s.kickVotes["Lobby"]; vote == nil || len(vote.Voters) != 1 || !vote.Voters["dave"] {
		t.Fatal("expired voter counted toward threshold")
	}
}
