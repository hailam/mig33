package chat

import (
	"context"
)

// Repository is the persistence capability required by chat.
// Implementations own transactions and must not expose driver or generated query types.
type Repository interface {
	CreateGroup(ctx context.Context, id, creator string, members []string) (Group, error)
	Invite(ctx context.Context, id, actor string, members []string) (Group, error)
	Leave(ctx context.Context, id, actor string) (Group, error)
	Group(ctx context.Context, id, viewer string) (Group, error)
	Groups(ctx context.Context, viewer string) ([]Group, error)
	// SaveMessage atomically rechecks durable permissions and commits the
	// message, immutable audience and optional request-key/hash receipt.
	// Live room membership is coordinated by the caller before this operation.
	SaveMessage(ctx context.Context, message Message, recipients []string) (Message, error)
	Pending(ctx context.Context, viewer string, limit int) ([]Message, error)
	MarkDelivered(ctx context.Context, viewer string, id int64) error
	History(ctx context.Context, viewer string, kind byte, conversation string, afterID int64, limit int) ([]Message, error)
	LatestMessageID(ctx context.Context) (int64, error)
	OwnMessagesAfter(ctx context.Context, viewer string, after int64, limit int) ([]Message, error)
	Transcript(ctx context.Context, viewer string, after int64, limit int) ([]Message, error)
	RequestedMessage(ctx context.Context, viewer, key, hash string) (Message, error)
	MarkConversationRead(ctx context.Context, viewer string, kind byte, target string, id int64) error
	ConversationRead(ctx context.Context, viewer string, kind byte, target string) (int64, error)
}
