package commerce

import (
	"context"
	"strings"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/faults"
)

type Privacy interface {
	Exists(context.Context, string) (bool, error)
	CanContact(context.Context, string, string) (bool, error)
	BlocksIncoming(context.Context, string, string) (bool, error)
}

type EmoticonValidator interface {
	ValidateEmoticons(context.Context, string, string) error
}

type Service struct {
	repo      Repository
	privacy   Privacy
	emoticons EmoticonValidator
}

func New(repo Repository, privacy Privacy, emoticons EmoticonValidator) *Service {
	return &Service{repo: repo, privacy: privacy, emoticons: emoticons}
}

type PurchaseCommand struct{ Actor, Recipient, Item, Nonce, Note string }

// PurchaseResult contains committed effects. Realtime delivery is a subsequent
// wakeup of durable messages and must never determine whether a purchase exists.
type PurchaseResult struct {
	Receipt          CreditReceipt
	Product          catalog.Product
	RefreshEmoticons bool
	DurableMessages  bool
}

type Inventory struct {
	Items         []OwnedStoreItem
	SelectedTheme string
	GiftNotes     map[int64]string
}

func (s *Service) Recipient(ctx context.Context, actor, recipient string, transfer bool) (string, error) {
	recipient = strings.ToLower(strings.TrimSpace(recipient))

	exists, err := s.privacy.Exists(ctx, recipient)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", faults.ErrNotFound
	}
	if transfer && actor == recipient {
		return "", faults.ErrInvalid
	}
	return recipient, nil
}

func (s *Service) Purchase(ctx context.Context, command PurchaseCommand) (PurchaseResult, error) {
	if err := ctx.Err(); err != nil {
		return PurchaseResult{}, err
	}

	item, ok := catalog.Lookup(command.Item)
	if !ok {
		return PurchaseResult{}, faults.ErrInvalid
	}

	exists, err := s.privacy.Exists(ctx, command.Recipient)
	if err != nil {
		return PurchaseResult{}, err
	}
	if !exists {
		return PurchaseResult{}, faults.ErrNotFound
	}
	if item.Kind == "theme" && command.Recipient != command.Actor {
		return PurchaseResult{}, faults.ErrInvalid
	}

	if command.Recipient != command.Actor {
		allowed, err := s.privacy.CanContact(ctx, command.Actor, command.Recipient)
		if err != nil {
			return PurchaseResult{}, err
		}
		if !allowed {
			return PurchaseResult{}, faults.ErrForbidden
		}
		blocked, err := s.privacy.BlocksIncoming(ctx, command.Recipient, command.Actor)
		if err != nil {
			return PurchaseResult{}, err
		}
		if blocked {
			return PurchaseResult{}, faults.ErrForbidden
		}
	}

	if err := ValidateGiftNote(command.Note); err != nil {
		return PurchaseResult{}, err
	}
	if command.Note != "" && item.Kind != "gift" {
		return PurchaseResult{}, faults.ErrInvalid
	}
	if err := s.emoticons.ValidateEmoticons(ctx, command.Actor, command.Note); err != nil {
		return PurchaseResult{}, faults.WrapError(faults.Invalid, err.Error(), err)
	}

	receipt, err := s.repo.CreditOperationWithNote(ctx, command.Actor, command.Recipient, "purchase", item.ID, command.Nonce, item.Price, command.Note)
	if err != nil {
		return PurchaseResult{}, err
	}
	return PurchaseResult{Receipt: receipt, Product: item, RefreshEmoticons: item.Kind == "emoticon_pack", DurableMessages: item.Kind == "gift"}, nil
}

func (s *Service) Transfer(ctx context.Context, actor, recipient, nonce string, amount int64) (CreditReceipt, error) {
	if err := ctx.Err(); err != nil {
		return CreditReceipt{}, err
	}
	if actor == recipient || amount <= 0 || amount > 100000000 {
		return CreditReceipt{}, faults.ErrInvalid
	}

	exists, err := s.privacy.Exists(ctx, recipient)
	if err != nil {
		return CreditReceipt{}, err
	}
	if !exists {
		return CreditReceipt{}, faults.ErrInvalid
	}

	return s.repo.CreditOperation(ctx, actor, recipient, "transfer", "", nonce, amount)
}

func (s *Service) Recharge(ctx context.Context, actor, nonce string, amount int64) (CreditReceipt, error) {
	if amount != 1000 && amount != 5000 && amount != 10000 {
		return CreditReceipt{}, faults.ErrInvalid
	}

	return s.repo.CreditOperation(ctx, actor, actor, "test-recharge", "", nonce, amount)
}

func (s *Service) Balance(ctx context.Context, actor string) (int64, error) {
	return s.repo.CreditBalance(ctx, actor)
}

func (s *Service) History(ctx context.Context, actor string) ([]CreditEntry, error) {
	return s.repo.CreditHistory(ctx, actor)
}

func (s *Service) Rating(ctx context.Context, item, actor string) (GiftRatingSummary, error) {
	return s.repo.GiftRating(ctx, item, actor)
}

func (s *Service) Rate(ctx context.Context, actor, item string, rating int) (GiftRatingSummary, error) {
	return s.repo.RateGift(ctx, actor, item, rating)
}

func (s *Service) SelectedTheme(ctx context.Context, actor string) (string, error) {
	return s.repo.SelectedStoreTheme(ctx, actor)
}

// SelectTheme is called only after the realtime workflow has reserved capacity
// for every affected native session. It owns persistence, not queue admission.
func (s *Service) SelectTheme(ctx context.Context, actor, item string) error {
	if item == "default" {
		return s.repo.ResetStoreTheme(ctx, actor)
	}
	return s.repo.SelectStoreTheme(ctx, actor, item)
}

func (s *Service) Inventory(ctx context.Context, actor string) (Inventory, error) {
	items, err := s.repo.OwnedStoreItems(ctx, actor)
	if err != nil {
		return Inventory{}, err
	}

	theme, err := s.repo.SelectedStoreTheme(ctx, actor)
	if err != nil {
		return Inventory{}, err
	}

	result := Inventory{Items: items, SelectedTheme: theme, GiftNotes: make(map[int64]string)}
	for _, owned := range items {
		item, ok := catalog.Lookup(owned.Item)
		if !ok || item.Kind != "gift" {
			continue
		}
		note, err := s.repo.GiftNote(ctx, owned.OperationID, actor)
		if err != nil {
			return Inventory{}, err
		}
		result.GiftNotes[owned.OperationID] = note
	}
	return result, nil
}
