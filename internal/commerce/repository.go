package commerce

import (
	"context"
)

// Repository is the persistence capability required by commerce.
// Implementations own transactions and must not expose driver or generated query types.
type Repository interface {
	CreditBalance(ctx context.Context, user string) (int64, error)
	CreditOperation(ctx context.Context, actor, recipient, kind, item, nonce string, amount int64) (CreditReceipt, error)
	// CreditOperationWithNote commits the replay receipt, ledger, inventory and
	// durable gift message/audience together. A returned receipt is committed;
	// delivery wakeups cannot decide whether the purchase occurred.
	CreditOperationWithNote(ctx context.Context, actor, recipient, kind, item, nonce string, amount int64, note string) (CreditReceipt, error)
	CreditHistory(ctx context.Context, user string) ([]CreditEntry, error)
	OwnedStoreItems(ctx context.Context, user string) ([]OwnedStoreItem, error)
	SelectedStoreTheme(ctx context.Context, user string) (string, error)
	SelectStoreTheme(ctx context.Context, user, theme string) error
	GiftNote(ctx context.Context, operationID int64, viewer string) (string, error)
	GiftRating(ctx context.Context, item, owner string) (GiftRatingSummary, error)
	RateGift(ctx context.Context, owner, item string, rating int) (GiftRatingSummary, error)
	ResetStoreTheme(ctx context.Context, user string) error
	HasStoreItem(ctx context.Context, owner, item string) (bool, error)
}
