package commerce

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/faults"
)

type StoreItem = catalog.Product

type CreditReceipt struct {
	ID, Amount, CreatedAt        int64
	Actor, Recipient, Kind, Item string
}

type CreditEntry struct {
	ID, OperationID, Delta, BalanceAfter, CreatedAt int64
	Kind, Actor, Recipient, Item                    string
}

type OwnedStoreItem struct {
	ID, OperationID, CreatedAt int64
	Owner, Sender, Item        string
}

func ValidateGiftNote(note string) error {
	if len(note) > 320 || !utf8.ValidString(note) {
		return faults.ErrInvalid
	}
	for _, r := range note {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return faults.ErrInvalid
		}
	}
	for _, prefix := range []string{"[gift:", "[photo:", "[pack:"} {
		if strings.Contains(strings.ToLower(note), prefix) {
			return faults.ErrInvalid
		}
	}
	return nil
}

type GiftRatingSummary struct {
	Count     int     `json:"count"`
	Average   float64 `json:"average"`
	OwnRating int     `json:"own_rating"`
}
