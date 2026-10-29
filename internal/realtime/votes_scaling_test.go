package realtime

import (
	"fmt"
	"strings"
	"testing"
	"time"

	chatstore "github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/protocol"
)

// Extra identities exist only in an isolated test server, not the demo DB.
func addVoteTestUser(t *testing.T, s *Application, name, room string) string {
	t.Helper()
	s.mu.Lock()
	s.accounts[name] = &account{
		id:       int32(len(s.accounts) + 1),
		name:     name,
		display:  name,
		identity: chatstore.Identity{LegacyHash: protocol.JavaHash(name + "-demo"), LegacyUnits: len(name + "-demo"), Activated: true},
	}
	err := s.store.EnsureContacts(t.Context(), name, nil)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	id := login(t, s, name)
	joinRoom(t, s, id, room)
	return id
}

func voteOccupancyRoom(t *testing.T, occupancy int) (*Application, string, []string) {
	t.Helper()
	s := newTestServer(t)
	target := login(t, s, "bob")
	joinRoom(t, s, target, "Lobby")
	voters := make([]string, occupancy-1)
	for i := range voters {
		voters[i] = addVoteTestUser(t, s, fmt.Sprintf("voteuser%d", i), "Lobby")
	}
	for _, id := range append([]string{target}, voters...) {
		drainGroupQueue(s, id)
	}
	return s, target, voters
}

func assertVoteTargetPresent(t *testing.T, s *Application, target string, want bool) {
	t.Helper()
	s.mu.Lock()
	present := s.sessions[target].rooms["Lobby"]
	s.mu.Unlock()
	if present != want {
		t.Fatalf("target room membership = %t, want %t", present, want)
	}
}

func assertVoteNotice(t *testing.T, s *Application, id, fragment string) {
	t.Helper()
	for _, p := range drainGroupQueue(s, id) {
		if body, _ := p.Text(8); p.Opcode == 500 && strings.Contains(body, fragment) {
			return
		}
	}
	t.Fatalf("missing room notice containing %q", fragment)
}

func TestVoteKickOccupancyThresholds(t *testing.T) {
	for _, test := range []struct{ users, required int }{
		{4, 3},
		{8, 3},
		{9, 4},
		{11, 4},
		{12, 5},
		{14, 5},
		{15, 6},
		{17, 6},
		{18, 7},
		{20, 7},
		{21, 8},
		{24, 8},
		{25, 9},
		{26, 9},
		{30, 9},
		{50, 9},
	} {
		t.Run(fmt.Sprintf("%d_members_need_%d_votes", test.users, test.required), func(t *testing.T) {
			s, target, voters := voteOccupancyRoom(t, test.users)
			for i := 0; i < test.required; i++ {
				castVote(t, s, voters[i], "Lobby", "bob", 1)
				assertVoteTargetPresent(t, s, target, i+1 < test.required)
				assertVoteNotice(t, s, voters[0], fmt.Sprintf("%d/%d", i+1, test.required))
			}
			s.mu.Lock()
			active := s.kickVotes["Lobby"] != nil
			s.mu.Unlock()
			if active {
				t.Fatal("completed vote remains active")
			}
		})
	}
}

func TestVoteKickUsesUniqueActiveRoomOccupancy(t *testing.T) {
	s, target, voters := voteOccupancyRoom(t, 8)
	duplicate := login(t, s, "voteuser0")
	joinRoom(t, s, duplicate, "Lobby")
	addVoteTestUser(t, s, "outsider", "Gaming")
	expired := addVoteTestUser(t, s, "expireduser", "Lobby")
	_, challenge := s.Handle(t.Context(), "", protocol.New(200, 1).SetText(5, "unverified"))
	s.mu.Lock()
	s.sessions[expired].touched = time.Now().Add(-sessionTTL - time.Second)
	// Even a malformed session fixture must not count before authentication.
	s.sessions[challenge].rooms["Lobby"] = true
	s.mu.Unlock()
	castVote(t, s, voters[0], "Lobby", "bob", 1)
	assertVoteNotice(t, s, voters[0], "1/3")
	replies := castVote(t, s, duplicate, "Lobby", "bob", 1)
	if body, _ := replies[1].Text(8); !strings.Contains(body, "1/3") {
		t.Fatalf("duplicate vote denominator changed: %q", body)
	}
	castVote(t, s, voters[1], "Lobby", "bob", 1)
	assertVoteTargetPresent(t, s, target, true)
	castVote(t, s, voters[2], "Lobby", "bob", 1)
	assertVoteTargetPresent(t, s, target, false)
}

func TestVoteKickThresholdRemainsFixedDuringBallot(t *testing.T) {
	t.Run("joining does not increase threshold", func(t *testing.T) {
		s, target, voters := voteOccupancyRoom(t, 8)
		castVote(t, s, voters[0], "Lobby", "bob", 1)
		joined := addVoteTestUser(t, s, "newvoter", "Lobby")
		castVote(t, s, joined, "Lobby", "bob", 1)
		assertVoteNotice(t, s, voters[0], "2/3")
		castVote(t, s, voters[1], "Lobby", "bob", 1)
		assertVoteTargetPresent(t, s, target, false)
	})
	t.Run("nonvoter departure does not reduce threshold", func(t *testing.T) {
		s, target, voters := voteOccupancyRoom(t, 9)
		castVote(t, s, voters[0], "Lobby", "bob", 1)
		s.Handle(t.Context(), voters[7], protocol.New(704, 1).SetText(1, "Lobby"))
		castVote(t, s, voters[1], "Lobby", "bob", 1)
		castVote(t, s, voters[2], "Lobby", "bob", 1)
		assertVoteNotice(t, s, voters[0], "3/4")
		assertVoteTargetPresent(t, s, target, true)
		castVote(t, s, voters[3], "Lobby", "bob", 1)
		assertVoteTargetPresent(t, s, target, false)
	})
	t.Run("departing voter loses vote but threshold stays", func(t *testing.T) {
		s, target, voters := voteOccupancyRoom(t, 9)
		castVote(t, s, voters[0], "Lobby", "bob", 1)
		castVote(t, s, voters[1], "Lobby", "bob", 1)
		drainGroupQueue(s, voters[0])
		s.Handle(t.Context(), voters[1], protocol.New(704, 1).SetText(1, "Lobby"))
		assertVoteNotice(t, s, voters[0], "1/4 votes; voteuser1 left")
		castVote(t, s, voters[2], "Lobby", "bob", 1)
		castVote(t, s, voters[3], "Lobby", "bob", 1)
		assertVoteTargetPresent(t, s, target, true)
		castVote(t, s, voters[4], "Lobby", "bob", 1)
		assertVoteTargetPresent(t, s, target, false)
	})
	t.Run("new ballot recalculates occupancy", func(t *testing.T) {
		s, target, voters := voteOccupancyRoom(t, 9)
		castVote(t, s, voters[0], "Lobby", "bob", 1)
		s.Handle(t.Context(), target, protocol.New(704, 1).SetText(1, "Lobby"))
		s.Handle(t.Context(), voters[7], protocol.New(704, 2).SetText(1, "Lobby"))
		joinRoom(t, s, target, "Lobby")
		drainGroupQueue(s, voters[1])
		castVote(t, s, voters[1], "Lobby", "bob", 1)
		assertVoteNotice(t, s, voters[1], "1/3")
		castVote(t, s, voters[2], "Lobby", "bob", 1)
		castVote(t, s, voters[3], "Lobby", "bob", 1)
		assertVoteTargetPresent(t, s, target, false)
	})
}

func TestModeratorCountsInOccupancyAndStillKicksImmediately(t *testing.T) {
	s, target, voters := voteOccupancyRoom(t, 8)
	moderator := login(t, s, "alice")
	joinRoom(t, s, moderator, "Lobby")
	castVote(t, s, voters[0], "Lobby", "bob", 1)
	assertVoteNotice(t, s, voters[0], "1/4")
	castVote(t, s, moderator, "Lobby", "bob", 1)
	assertVoteTargetPresent(t, s, target, false)
	s.mu.Lock()
	active := s.kickVotes["Lobby"] != nil
	s.mu.Unlock()
	if active {
		t.Fatal("moderator kick left ballot active")
	}
}
