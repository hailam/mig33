package community

import (
	"context"
	"testing"
)

type communityStore struct {
	Repository
	role                     string
	actor, target, operation string
}

func (r *communityStore) Community(context.Context, int64) (Community, error) {
	return Community{Private: true}, nil
}

func (r *communityStore) CommunityRole(context.Context, int64, string) (string, error) {
	return r.role, nil
}

func (r *communityStore) CommunityMembership(_ context.Context, _ int64, actor, target, operation string) error {
	r.actor, r.target, r.operation = actor, target, operation
	return nil
}

func TestPendingCommunityMemberCannotReadPrivateContent(t *testing.T) {
	repo := &communityStore{role: "pending"}
	service := New(repo, nil, nil)
	view, err := service.View(context.Background(), "alice", 1)
	if err != nil || view.CanRead {
		t.Fatalf("pending role gained access: %+v %v", view, err)
	}
	repo.role = "member"
	view, err = service.View(context.Background(), "alice", 1)
	if err != nil || !view.CanRead {
		t.Fatalf("member cannot read: %+v %v", view, err)
	}
}

func TestSelfMembershipIgnoresInjectedTarget(t *testing.T) {
	repo := &communityStore{}
	err := New(repo, nil, nil).Membership(context.Background(), MembershipCommand{ID: 1, Actor: "alice", Target: "bob", Operation: "leave"})
	if err != nil || repo.actor != "alice" || repo.target != "alice" || repo.operation != "leave" {
		t.Fatalf("membership authority: %+v %v", repo, err)
	}
}
