package community

import (
	"context"

	"github.com/hailam/mig33/internal/rooms"
)

// Repository is the persistence capability required by community.
// Implementations own transactions and must not expose driver or generated query types.
type Repository interface {
	Community(ctx context.Context, id int64) (Community, error)
	Communities(ctx context.Context, viewer string, mine bool) ([]Community, error)
	CommunityRole(ctx context.Context, id int64, user string) (string, error)
	CreateCommunity(ctx context.Context, owner, name, description string, private bool) (Community, error)
	CommunityMembership(ctx context.Context, id int64, actor, target, operation string) error
	CommunityMembers(ctx context.Context, id int64, actor string) ([]CommunityMember, error)
	LinkCommunityRoom(ctx context.Context, id int64, actor, room string, remove bool) error
	CommunityRooms(ctx context.Context, id int64, actor string) ([]rooms.Room, error)
	AddCommunityPost(ctx context.Context, id, parent int64, actor, body, nonce string) (int64, error)
	CommunityPosts(ctx context.Context, id int64, actor string) ([]CommunityPost, error)
}
