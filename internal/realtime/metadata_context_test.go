package realtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/rooms"
)

type voteEligibilityRepository struct {
	Repository
	moderatorError error
	muteError      error
}

func (repo *voteEligibilityRepository) RoomSettings(context.Context, string) (rooms.RoomSettings, error) {
	return rooms.RoomSettings{UserKicks: true}, nil
}

func (repo *voteEligibilityRepository) IsRoomModerator(context.Context, string, string) (bool, error) {
	return false, repo.moderatorError
}

func (repo *voteEligibilityRepository) IsRoomMuted(context.Context, string, string) (bool, error) {
	return false, repo.muteError
}

func TestVoteEligibilityLookupFailurePreservesBallotAndRejectsAnotherVote(t *testing.T) {
	for _, field := range []string{"moderator", "mute"} {
		for name, cause := range map[string]error{"canceled": context.Canceled, "deadline": context.DeadlineExceeded, "temporary": errors.New("storage temporarily unavailable")} {
			t.Run(field+"_"+name, func(t *testing.T) {
				repo := &voteEligibilityRepository{}
				if field == "moderator" {
					repo.moderatorError = cause
				} else {
					repo.muteError = cause
				}
				app := applicationWithRepositoryPort(repo)
				app.sessions["alice-session"].rooms = map[string]bool{"Lobby": true}
				app.sessions["bob-session"].rooms = map[string]bool{"Lobby": true}
				vote := &kickVote{Target: "bob", Initiator: "alice", Voters: map[string]bool{"alice": true}, Required: 3, ExpiresAt: time.Now().Add(time.Minute)}
				app.kickVotes = map[string]*kickVote{"Lobby": vote}
				app.mu.Lock()
				defer app.mu.Unlock()
				if err := app.pruneKickVote(t.Context(), "Lobby", time.Now()); !errors.Is(err, cause) {
					t.Fatalf("eligibility error lost: %v", err)
				}
				if app.kickVotes["Lobby"] != vote || !vote.Voters["alice"] || len(vote.Voters) != 1 {
					t.Fatal("failed lookup fabricated a ballot cancellation or withdrawal")
				}
				response := app.roomKickVote(t.Context(), app.sessions["alice-session"], &commands.Request{Correlation: 1, Input: commands.Room{Operation: commands.RoomKick}}, "Lobby", "bob")
				if len(response) != 1 || fixturePacket(response[0]).Opcode != 0 {
					t.Fatal("vote was accepted with unknown eligibility")
				}
				if app.kickVotes["Lobby"] != vote || !vote.Voters["alice"] || len(vote.Voters) != 1 {
					t.Fatal("failed vote changed existing ballots")
				}
			})
		}
	}
}

type emoticonOwnershipRepository struct {
	Repository
	failure error
}

func (repo *emoticonOwnershipRepository) HasStoreItem(context.Context, string, string) (bool, error) {
	return true, repo.failure
}

func TestFailedEmoticonOwnershipLookupPreservesCompleteCatalogRetry(t *testing.T) {
	for name, cause := range map[string]error{"canceled": context.Canceled, "deadline": context.DeadlineExceeded, "temporary": errors.New("storage temporarily unavailable")} {
		t.Run(name, func(t *testing.T) {
			repo := &emoticonOwnershipRepository{failure: cause}
			app := applicationWithRepositoryPort(repo)
			x := app.sessions["alice-session"]
			x.ready = true
			x.emoticonsStale = true
			x.push = make(chan *events.Event, 8)
			app.mu.Lock()
			defer app.mu.Unlock()
			app.flushEmoticonCatalog(t.Context(), x)
			if !x.emoticonsStale || len(x.push) != 0 {
				t.Fatal("failed refresh published a truncated catalog or suppressed retry")
			}
			initial := app.initialNativeServices(t.Context(), x)
			if len(initial) != len(nativeIMServices) || !x.emoticonsStale {
				t.Fatal("initial failure lost provider descriptors or catalog retry")
			}
			for _, packet := range initial {
				if fixturePacket(packet).Opcode != 208 {
					t.Fatal("initial failure published an incomplete emoticon catalog")
				}
			}
			repo.failure = nil
			app.flushEmoticonCatalog(t.Context(), x)
			if x.emoticonsStale || len(x.push) != 1 {
				t.Fatal("catalog refresh did not retry")
			}
			packet := <-x.push
			aliases, _ := fixturePacket(packet).Text(1)
			if fixturePacket(packet).Opcode != 916 || !strings.Contains(aliases, "[buzz]") || !strings.Contains(aliases, "[smile]") {
				t.Fatal("retry omitted owned or default emoticons")
			}
		})
	}
}
