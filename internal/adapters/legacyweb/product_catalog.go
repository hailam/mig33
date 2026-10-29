package legacyweb

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/a-h/templ"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
)

type productSelection struct {
	Kind, Query, Item string
	Page              int
}

func selectProducts(q url.Values) (productSelection, []catalog.Product, error) {
	v := productSelection{Kind: q.Get("kind"), Query: q.Get("query"), Item: q.Get("item")}
	for _, key := range []string{"kind", "query", "item", "page", "ty"} {
		if len(q[key]) > 1 {
			return v, nil, faults.ErrInvalid
		}
	}
	if len(v.Query) > 120 || !utf8.ValidString(v.Query) {
		return v, nil, faults.ErrInvalid
	}
	if raw := q.Get("page"); raw != "" {
		var err error
		v.Page, err = strconv.Atoi(raw)
		if err != nil || v.Page < 0 || v.Page > 10000 {
			return v, nil, faults.ErrInvalid
		}
	}
	if v.Kind != "" && v.Kind != "gift" && v.Kind != "theme" && v.Kind != "emoticon_pack" {
		return v, nil, faults.ErrInvalid
	}

	legacyKind := map[string]string{"1": "gift", "5": "theme"}[q.Get("ty")]
	if legacyKind != "" {
		if v.Kind != "" && v.Kind != legacyKind {
			return v, nil, faults.ErrInvalid
		}

		v.Kind = legacyKind
	}

	items := []catalog.Product{}
	for _, item := range catalog.Products() {
		if (v.Kind == "" || item.Kind == v.Kind) && (v.Query == "" || strings.Contains(strings.ToLower(item.Name), strings.ToLower(v.Query))) {
			items = append(items, item)
		}
	}
	if v.Item != "" {
		item, ok := catalog.Lookup(v.Item)
		if !ok || (v.Kind != "" && item.Kind != v.Kind) {
			return v, nil, faults.ErrNotFound
		}

		items = []catalog.Product{item}
	}

	return v, items, nil
}

func (s *Adapter) productURL(v productSelection, recipient string) string {
	q := url.Values{"c": {"store"}, "a": {"home"}, "v": {"midlet"}, "username": {recipient}}
	if v.Kind != "" {
		q.Set("kind", v.Kind)
	}
	if v.Query != "" {
		q.Set("query", v.Query)
	}
	if v.Item != "" {
		q.Set("item", v.Item)
	}
	if v.Page > 0 {
		q.Set("page", strconv.Itoa(v.Page))
	}

	return s.publicURL + "/sites/index.php?" + q.Encode()
}

func (s *Adapter) productContent(ctx context.Context, x *actor, item catalog.Product, recipient string, details bool) templ.Component {
	target := recipient
	if item.Kind == "theme" {
		target = x.username
	}

	body := pages.Stack(pages.Heading(item.Name + " · " + commerce.FormatAmount(item.Price) + " test credits"))
	// Keep the existing six local fixture forms usable; recovered catalog names
	// open a detail/confirmation page before their placeholder can be bought.
	if !details && item.NameEvidence == "recovered_name" && item.ID != "rose" {
		return pages.Stack(body, ccLink(s.productURL(productSelection{Item: item.ID}, recipient), "View "+item.Name))
	}

	body = pages.Stack(body, ccText(item.Description))
	if details && item.Kind == "gift" {
		if rating, err := s.commerce.Rating(ctx, item.ID, x.username); err == nil {
			body = pages.Stack(body, ccText(giftRatingText(rating)))
			value := ""
			if rating.OwnRating != 0 {
				value = strconv.Itoa(rating.OwnRating)
			}

			body = pages.Stack(body, ccForm(s.productURL(productSelection{Item: item.ID}, recipient)+"&op=rate", x.csrf,
				[][2]string{{"item", item.ID}}, pages.Input("rating", "Your rating (local 1 to 5 scale)", "text", value, 5), "Rate gift"))
		}
	}
	if item.Kind == "emoticon_pack" && item.ID != "emotes-effects" {
		body = pages.Stack(body, ccText("Local placeholder: "+catalog.PackPlaceholderToken(item.ID)))
	}

	legacyType := map[string]string{"gift": "1", "theme": "5"}[item.Kind]
	controls := pages.Stack()
	if details && item.Kind == "gift" {
		controls = pages.TextArea("note", "Optional gift message (320 UTF-8 bytes)", "", 320)
	}

	body = pages.Stack(body, ccText("Recipient: "+target), ccForm(s.ccURL("store", "home")+"&op=purchase&ty="+legacyType, x.csrf,
		[][2]string{
			{"item", item.ID},
			{"recipient", target},
			{"binding", reportBinding(x.csrf, "store-purchase", item.ID, target)},
			{"nonce", token()},
		}, controls, "Buy "+item.Name))
	if details {
		body = pages.Stack(body, ccLink(s.productURL(productSelection{Kind: item.Kind}, recipient), "No, go back"))
	}

	return body
}

func (s *Adapter) productListing(ctx context.Context, x *actor, v productSelection, items []catalog.Product, recipient string) templ.Component {
	body := pages.Stack()
	for _, section := range []struct{ kind, label string }{
		{"", "All products"},
		{"gift", "Gifts"},
		{"emoticon_pack", "Emoticon Packs"},
		{"theme", "Themes"},
	} {
		body = pages.Stack(body, ccLink(s.productURL(productSelection{Kind: section.kind}, recipient), section.label))
	}
	if v.Item != "" {
		return pages.Stack(body, s.productContent(ctx, x, items[0], recipient, true))
	}

	const perPage = 8
	start := v.Page * perPage
	if start > len(items) {
		start = len(items)
	}

	end := min(start+perPage, len(items))
	body = pages.Stack(body, ccText(fmt.Sprintf("Products %d–%d of %d", min(start+1, len(items)), end, len(items))))
	for _, item := range items[start:end] {
		body = pages.Stack(body, s.productContent(ctx, x, item, recipient, false))
	}
	if v.Page > 0 {
		previous := v
		previous.Page--
		body = pages.Stack(body, ccLink(s.productURL(previous, recipient), "Previous products"))
	}
	if end < len(items) {
		next := v
		next.Page++
		body = pages.Stack(body, ccLink(s.productURL(next, recipient), "Next products"))
	}

	return body
}

// Catalog exposes public static metadata without balances, ownership or users.

func giftRatingText(rating commerce.GiftRatingSummary) string {
	return fmt.Sprintf("Your rating: %d/5. Local average: %.1f/5 (%d rating(s)). One current vote per account; 0 means not yet rated.", rating.OwnRating, rating.Average, rating.Count)
}
