package commerce

import (
	"context"
	"errors"
	"testing"

	"github.com/hailam/mig33/internal/faults"
)

type purchaseStore struct {
	Repository
	err                                       error
	calls                                     int
	actor, recipient, kind, item, nonce, note string
	amount                                    int64
}

func (r *purchaseStore) CreditOperationWithNote(_ context.Context, actor, recipient, kind, item, nonce string, amount int64, note string) (CreditReceipt, error) {
	r.calls++
	r.actor, r.recipient, r.kind, r.item, r.nonce, r.amount, r.note = actor, recipient, kind, item, nonce, amount, note
	if r.err != nil {
		return CreditReceipt{}, r.err
	}
	return CreditReceipt{ID: 42, Actor: actor, Recipient: recipient, Kind: kind, Item: item, Amount: amount}, nil
}

type purchasePrivacy struct{ allowed, blocked bool }

func (p purchasePrivacy) Exists(context.Context, string) (bool, error) { return true, nil }
func (p purchasePrivacy) CanContact(context.Context, string, string) (bool, error) {
	return p.allowed, nil
}

func (p purchasePrivacy) BlocksIncoming(context.Context, string, string) (bool, error) {
	return p.blocked, nil
}

type validEmoticons struct{}

func (validEmoticons) ValidateEmoticons(context.Context, string, string) error { return nil }

func TestPurchaseEffectsRequireCommittedWorkflow(t *testing.T) {
	command := PurchaseCommand{Actor: "alice", Recipient: "bob", Item: "rose", Nonce: "request", Note: "Hello"}
	repo := &purchaseStore{err: errors.New("commit failed")}
	service := New(repo, purchasePrivacy{allowed: true}, validEmoticons{})
	result, err := service.Purchase(context.Background(), command)
	if err == nil || result.DurableMessages || result.RefreshEmoticons || result.Receipt.ID != 0 {
		t.Fatalf("uncommitted effects: %+v, %v", result, err)
	}
	repo.err = nil
	result, err = service.Purchase(context.Background(), command)
	if err != nil || !result.DurableMessages || result.RefreshEmoticons || result.Receipt.ID != 42 {
		t.Fatalf("committed gift: %+v, %v", result, err)
	}
	if repo.actor != "alice" || repo.recipient != "bob" || repo.kind != "purchase" || repo.note != "Hello" || repo.amount != result.Product.Price {
		t.Fatalf("atomic purchase arguments: %+v", repo)
	}
}

func TestPurchasePrivacyAndSystemTokensPreventCharge(t *testing.T) {
	for _, tc := range []struct {
		name    string
		privacy purchasePrivacy
		note    string
	}{
		{"private", purchasePrivacy{}, "hello"},
		{"blocked", purchasePrivacy{allowed: true, blocked: true}, "hello"},
		{"forged receipt", purchasePrivacy{allowed: true}, "[gift:rose]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &purchaseStore{}
			_, err := New(repo, tc.privacy, validEmoticons{}).Purchase(context.Background(), PurchaseCommand{Actor: "alice", Recipient: "bob", Item: "rose", Note: tc.note})
			if err == nil || repo.calls != 0 {
				t.Fatalf("invalid purchase reached ledger: calls=%d error=%v", repo.calls, err)
			}
		})
	}
}

func TestRechargeRejectsArbitraryAmountsBeforeLedger(t *testing.T) {
	_, err := New(&purchaseStore{}, purchasePrivacy{}, validEmoticons{}).Recharge(context.Background(), "alice", "request", 1001)
	if !errors.Is(err, faults.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestPackPurchaseReturnsCatalogRefresh(t *testing.T) {
	result, err := New(&purchaseStore{}, purchasePrivacy{allowed: true}, validEmoticons{}).Purchase(context.Background(), PurchaseCommand{Actor: "alice", Recipient: "bob", Item: "emotes-effects"})
	if err != nil || !result.RefreshEmoticons || result.DurableMessages {
		t.Fatalf("pack effects: %+v %v", result, err)
	}
}
