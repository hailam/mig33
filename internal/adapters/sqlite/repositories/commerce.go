package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/commerce"
)

// Credits are integer hundredths of local test credits, never a fiat balance.
type StoreItem = commerce.StoreItem

func StoreCatalog() []StoreItem                   { return catalog.Products() }
func LookupStoreItem(id string) (StoreItem, bool) { return catalog.Lookup(id) }

type (
	CreditReceipt  = commerce.CreditReceipt
	CreditEntry    = commerce.CreditEntry
	OwnedStoreItem = commerce.OwnedStoreItem
)

func validCommerceNonce(nonce string) bool {
	if len(nonce) != 48 {
		return false
	}

	for _, c := range nonce {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}

	return true
}

func (s *Store) CreditBalance(ctx context.Context, user string) (int64, error) {
	n, err := queries(s.db).GetCreditBalance(ctx, user)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}

	return n, queryError(

		// CreditOperation atomically posts a balanced transfer, a store purchase, or an
		// explicitly local test recharge. The same nonce returns the same receipt;
		// changing its payload is a conflict. Callers validate recipient existence.
		ctx, err)
}

func (s *Store) CreditOperation(ctx context.Context, actor, recipient, kind, item, nonce string, amount int64) (CreditReceipt, error) {
	return s.CreditOperationWithNote(ctx, actor, recipient, kind, item, nonce, amount, "")
}

// CreditOperationWithNote adds a literal gift note to the purchase transaction.
// Empty notes preserve pre-v18 receipts and their replay semantics. A caller
// must validate recipient existence; privacy and blocks are checked atomically.
func (s *Store) CreditOperationWithNote(ctx context.Context, actor, recipient, kind, item, nonce string, amount int64, note string) (CreditReceipt, error) {
	actor, err := username(actor)
	if err != nil {
		return CreditReceipt{}, queryError(ctx, err)
	}

	recipient, err = username(recipient)
	if err != nil {
		return CreditReceipt{}, queryError(ctx, err)
	}
	if !validCommerceNonce(nonce) || amount <= 0 || amount > 100000000 {
		return CreditReceipt{}, ErrInvalid
	}

	switch kind {
	case "test-recharge":
		if recipient != actor || item != "" {
			return CreditReceipt{}, ErrInvalid
		}
	case "transfer":
		if recipient == actor || item != "" {
			return CreditReceipt{}, ErrInvalid
		}
	case "purchase":
		p, ok := LookupStoreItem(item)
		if !ok || p.Price != amount || (p.Kind == "theme" && recipient != actor) {
			return CreditReceipt{}, ErrInvalid
		}
	default:
		return CreditReceipt{}, ErrInvalid
	}
	if err := ValidateGiftNote(note); err != nil {
		return CreditReceipt{}, queryError(ctx, err)
	}

	product, _ := LookupStoreItem(item)
	isGift := kind == "purchase" && product.Kind == "gift"
	if note != "" && !isGift {
		return CreditReceipt{}, ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CreditReceipt{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	record, err := queries(tx).GetCreditReceiptByNonce(ctx, sqlc.GetCreditReceiptByNonceParams{Actor: actor, Nonce: nonce})
	prior := CreditReceipt{
		ID:        record.ID,
		Actor:     record.Actor,
		Recipient: record.Recipient,
		Kind:      record.Kind,
		Item:      record.Item,
		Amount:    record.Amount,
		CreatedAt: record.CreatedAt,
	}
	if err == nil {
		priorNote, err := queries(tx).GetGiftNoteByReceipt(ctx, prior.ID)
		if err != nil {
			return CreditReceipt{}, queryError(ctx, err)
		}
		if prior.Recipient != recipient || prior.Kind != kind || prior.Item != item || prior.Amount != amount || priorNote != note {
			return CreditReceipt{}, ErrConflict
		}

		return prior, queryError(ctx, tx.Commit())
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return CreditReceipt{}, queryError(ctx, err)
	}
	if isGift && actor != recipient {
		if err = allowGift(ctx, tx, actor, recipient); err != nil {
			return CreditReceipt{}, queryError(ctx, err)
		}
	}

	for _, u := range []string{actor, recipient} {
		if err = queries(tx).EnsureCreditWallet(ctx, u); err != nil {
			return CreditReceipt{}, queryError(ctx, err)
		}
	}

	balance, err := queries(tx).GetCreditBalance(ctx, actor)
	if err != nil {
		return CreditReceipt{}, queryError(ctx, err)
	}
	if kind != "test-recharge" && balance < amount {
		return CreditReceipt{}, ErrLimit
	}

	now := time.Now().UnixMilli()
	id, err := queries(tx).CreateCreditReceipt(ctx, sqlc.CreateCreditReceiptParams{
		Actor:     actor,
		Nonce:     nonce,
		Kind:      kind,
		Recipient: recipient,
		Amount:    amount,
		Item:      item,
		CreatedAt: now,
	})
	if err != nil {
		return CreditReceipt{}, queryError(ctx, err)
	}
	if note != "" {
		if err = queries(tx).CreateGiftNote(ctx, sqlc.CreateGiftNoteParams{OperationID: id, Note: note}); err != nil {
			return CreditReceipt{}, queryError(ctx, err)
		}
	}

	entries := map[string]int64{}
	if kind == "test-recharge" {
		entries[actor] = amount
	} else {
		entries[actor] = -amount
		if kind == "transfer" {
			entries[recipient] = amount
		}
	}

	for u, delta := range entries {
		before, err := queries(tx).GetCreditBalance(ctx, u)
		if err != nil {
			return CreditReceipt{}, queryError(ctx, err)
		}

		after := before + delta
		if after < 0 || after > 1000000000000 {
			return CreditReceipt{}, ErrLimit
		}
		if err = queries(tx).SetCreditBalance(ctx, sqlc.SetCreditBalanceParams{Balance: after, Username: u}); err != nil {
			return CreditReceipt{}, queryError(ctx, err)
		}
		if err = queries(tx).CreateCreditEntry(ctx, sqlc.CreateCreditEntryParams{OperationID: id, Username: u, Delta: delta, BalanceAfter: after}); err != nil {
			return CreditReceipt{}, queryError(ctx, err)
		}
	}
	if kind == "purchase" {
		if err = queries(tx).CreateOwnedStoreItem(ctx, sqlc.CreateOwnedStoreItemParams{OperationID: id, Owner: recipient, Sender: actor, Item: item, CreatedAt: now}); err != nil {
			return CreditReceipt{}, queryError(ctx, err)
		}
		if product.Kind == "gift" && actor != recipient {
			if err = saveGiftMessage(ctx, tx, actor, recipient, product, note, now); err != nil {
				return CreditReceipt{}, queryError(ctx, err)
			}
		}
	}

	return CreditReceipt{ID: id, Actor: actor, Recipient: recipient, Kind: kind, Item: item, Amount: amount, CreatedAt: now}, queryError(ctx, tx.Commit())
}

func (s *Store) CreditHistory(ctx context.Context, user string) ([]CreditEntry, error) {
	rows, err := queries(s.db).ListCreditHistory(ctx, user)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	var out []CreditEntry
	for _, r := range rows {
		out = append(out, CreditEntry(r))
	}

	return out, nil
}

func (s *Store) OwnedStoreItems(ctx context.Context, user string) ([]OwnedStoreItem, error) {
	rows, err := queries(s.db).ListOwnedStoreItems(ctx, user)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	var out []OwnedStoreItem
	for _, r := range rows {
		out = append(out, OwnedStoreItem(r))
	}

	return out, nil
}

func (s *Store) SelectedStoreTheme(ctx context.Context, user string) (string, error) {
	theme, err := queries(s.db).GetStoreTheme(ctx, user)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}

	return theme, queryError(ctx, err)
}

func (s *Store) SelectStoreTheme(ctx context.Context, user, theme string) error {
	item, ok := LookupStoreItem(theme)
	if !ok || item.Kind != "theme" {
		return ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	owns, err := queries(tx).HasStoreItem(ctx, sqlc.HasStoreItemParams{Owner: user, Item: theme})
	if err != nil {
		return queryError(ctx, err)
	}
	if !owns {
		return ErrForbidden
	}
	if err = queries(tx).SetStoreTheme(ctx, sqlc.SetStoreThemeParams{Username: user, Theme: theme}); err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}
