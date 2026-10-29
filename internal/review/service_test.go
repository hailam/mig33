package review

import (
	"context"
	"errors"
	"testing"

	"github.com/hailam/mig33/internal/faults"
)

type reviewStore struct {
	Repository
	calls      int
	submission ReportSubmission
}

func (r *reviewStore) SubmitReport(_ context.Context, input ReportSubmission) (AbuseReport, error) {
	r.calls++
	r.submission = input
	return AbuseReport{ID: 10}, nil
}

func TestOrdinaryActorCannotReadOrDecideOperatorWork(t *testing.T) {
	service := New(&reviewStore{}, nil, []string{"operator"})
	if _, err := service.Queue(context.Background(), "alice", 0, 0); !errors.Is(err, faults.ErrForbidden) {
		t.Fatal(err)
	}
	if _, err := service.Details(context.Background(), "alice", "report", 1); !errors.Is(err, faults.ErrForbidden) {
		t.Fatal(err)
	}
	if err := service.Decide(context.Background(), OperatorReview{Actor: "alice"}, "pending"); !errors.Is(err, faults.ErrForbidden) {
		t.Fatal(err)
	}
}

func TestReportTargetMustBelongToAuthorizedConversation(t *testing.T) {
	repo := &reviewStore{}
	service := New(repo, nil, nil)
	conversation := Conversation{Reporter: "alice", Kind: "room", Context: "Lobby", Targets: []string{"bob"}}
	command := SubmitCommand{Target: "mallory", Category: "spam", Details: "Repeated unwanted messages", Nonce: "nonce"}
	if _, err := service.Submit(context.Background(), conversation, command); !errors.Is(err, faults.ErrInvalid) || repo.calls != 0 {
		t.Fatalf("outside target: %v calls=%d", err, repo.calls)
	}
	command.Target = "bob"
	if _, err := service.Submit(context.Background(), conversation, command); err != nil {
		t.Fatal(err)
	}
	if repo.submission.Reporter != "alice" || repo.submission.Context != "Lobby" || repo.submission.Target != "bob" {
		t.Fatalf("wrong bound conversation: %+v", repo.submission)
	}
}
