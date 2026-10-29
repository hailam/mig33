package commerce

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/faults"
)

const (
	giftCommandLimit     = 20
	GiftCommandTextLimit = 3500
)

var giftNoteOption = regexp.MustCompile(`(?:^|\s)-m(?:\s|$)`)

func ParseGiftSend(arg string) (name, note string, err error) {
	name = strings.TrimSpace(arg)
	if marker := giftNoteOption.FindStringIndex(arg); marker != nil {
		name, note = strings.TrimSpace(arg[:marker[0]]), strings.TrimSpace(arg[marker[1]:])
		if note == "" {
			return "", "", faults.ErrInvalid
		}
	}
	if name == "" || len(name) > 120 {
		return "", "", faults.ErrInvalid
	}
	return name, note, ValidateGiftNote(note)
}

func FindGift(name string) (StoreItem, bool) {
	for _, item := range catalog.Products() {
		if item.Kind == "gift" && (strings.EqualFold(item.Name, name) || strings.EqualFold(item.ID, name)) {
			return item, true
		}
	}
	return StoreItem{}, false
}

func GiftRatingText(rating GiftRatingSummary) string {
	return fmt.Sprintf("Your rating: %d/5. Local average: %.1f/5 (%d rating(s)). One current vote per account; 0 means not yet rated.", rating.OwnRating, rating.Average, rating.Count)
}

func GiftCatalogItems() []StoreItem {
	items := []StoreItem{}
	for _, item := range catalog.Products() {
		if item.Kind == "gift" {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
	return items
}

func GiftCategory(item StoreItem) string {
	if item.Category == "" {
		return "General"
	}
	return item.Category
}

func GiftItemLines(items []StoreItem) []string {
	lines := []string{}
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s - %s test credits", item.Name, FormatAmount(item.Price)))
	}
	if len(lines) == 0 {
		lines = append(lines, "No matching gifts. Try a different search or open Store.")
	}
	return lines
}

func GiftCommandText(heading string, lines []string) string {
	more := len(lines) > giftCommandLimit
	if more {
		lines = lines[:giftCommandLimit]
	}
	// The original Canvas renderer consumes line breaks without spacing.
	// Visible separators must therefore contain ordinary spaces/punctuation.
	text := heading + ": " + strings.Join(lines, "; ")
	if more {
		text += "; More items in Menu > Store; use /gift search TEXT to narrow the list."
	}
	runes := []rune(text)
	if len(runes) > GiftCommandTextLimit {
		suffix := "; More details in Menu > Store."
		text = string(runes[:GiftCommandTextLimit-len([]rune(suffix))]) + suffix
	}
	return text
}
