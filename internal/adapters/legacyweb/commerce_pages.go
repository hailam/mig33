package legacyweb

import (
	"context"
	"crypto/subtle"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
)

func (s *Adapter) commercePage(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) (pages.Result, bool) {
	if !communityCommerceRoute(u) {
		return pages.Result{}, false
	}

	q := u.Query()
	controller, action := q.Get("c"), q.Get("a")
	known := controller == "account" && (action == "home" || action == "get_history" || action == "recharge_credit" || action == "transfer_credit") ||
		controller == "store" && (action == "home" || action == "main") ||
		controller == "merchant" && action == "dashboard"
	if !known {
		return pages.Result{}, false
	}
	if method != "GET" && method != "POST" {
		return ccPage(405, "Account", ccText("Use GET or POST.")), true
	}
	if method == "POST" && !ccValidPost(x, form) {
		return ccPage(403, "Account", ccText("Open this page again before submitting.")), true
	}
	if controller == "store" {
		return s.storePage(ctx, x, method, u, form), true
	}

	notice := ""
	recipient := strings.ToLower(strings.TrimSpace(q.Get("recipient_username")))
	if len(q["recipient_username"]) > 1 {
		return ccError("Transfer Credit", faults.ErrInvalid), true
	}
	if method == "POST" {
		switch {
		case action == "transfer_credit" && q.Get("op") == "recipient":
			if !roomFormFields(form, "csrf", "username") {
				return ccError("Transfer Credit", faults.ErrInvalid), true
			}

			recipient = strings.ToLower(strings.TrimSpace(form.Get("username")))
		case action == "transfer_credit" && q.Get("op") == "transfer":
			recipient = form.Get("recipient")
			binding := reportBinding(x.csrf, "credit-transfer", "recipient", recipient)
			if !roomFormFields(form, "csrf", "recipient", "binding", "nonce", "amount") || subtle.ConstantTimeCompare([]byte(binding), []byte(form.Get("binding"))) != 1 || s.account(ctx, recipient) == nil {
				return ccError("Transfer Credit", faults.ErrInvalid), true
			}

			amount, err := commerce.ParseAmount(form.Get("amount"))
			if err != nil {
				return ccError("Transfer Credit", err), true
			}

			receipt, err := s.commerce.Transfer(ctx, x.username, recipient, form.Get("nonce"), amount)
			if err != nil {
				if err == faults.ErrLimit {
					return ccPage(409, "Transfer Credit", ccText("Insufficient test credits, or a wallet limit was reached.")), true
				}

				return ccError("Transfer Credit", err), true
			}

			notice = "Transferred " + commerce.FormatAmount(receipt.Amount) + " test credits to " + recipient + ". Receipt #" + strconv.FormatInt(receipt.ID, 10) + "."
		case action == "recharge_credit" && q.Get("op") == "recharge":
			if !roomFormFields(form, "csrf", "nonce", "amount") {
				return ccError("Recharge Credit", faults.ErrInvalid), true
			}

			amount, err := strconv.ParseInt(form.Get("amount"), 10, 64)
			if err != nil || (amount != 1000 && amount != 5000 && amount != 10000) {
				return ccError("Recharge Credit", faults.ErrInvalid), true
			}

			receipt, err := s.commerce.Recharge(ctx, x.username, form.Get("nonce"), amount)
			if err != nil {
				return ccError("Recharge Credit", err), true
			}

			notice = "Added " + commerce.FormatAmount(receipt.Amount) + " local test credits. Receipt #" + strconv.FormatInt(receipt.ID, 10) + ". No payment was taken."
		default:
			return ccError("Account", faults.ErrInvalid), true
		}
	}

	balance, err := s.commerce.Balance(ctx, x.username)
	if err != nil {
		return ccError("Account", err), true
	}

	body := pages.Stack(ccText("Balance: "+commerce.FormatAmount(balance)+" test credits"), ccText("Local test credits have no cash value. Payment and withdrawal services are not connected."))
	if notice != "" {
		body = pages.Stack(body, ccText(notice))
	}

	title := "My Account"
	switch action {
	case "transfer_credit":
		title = "Transfer Credit"
		if recipient != "" {
			if s.account(ctx, recipient) == nil || recipient == x.username {
				return ccError(title, faults.ErrInvalid), true
			}

			body = pages.Stack(body, pages.Stack(ccText("Recipient: "+recipient), ccForm(s.ccURL("account", "transfer_credit")+"&op=transfer", x.csrf, [][2]string{
				{"recipient", recipient},
				{"binding", reportBinding(x.csrf, "credit-transfer", "recipient", recipient)},
				{"nonce", token()},
			}, pages.Input("amount", "Amount (test credits)", "text", "", 10), "Transfer")))
		}

		body = pages.Stack(body, ccForm(s.ccURL("account", "transfer_credit")+"&op=recipient", x.csrf, nil, pages.Input("username", "Recipient username", "text", "", 10), "Choose Recipient"))
	case "recharge_credit":
		title = "Recharge Credit"
		for _, amount := range []int64{1000, 5000, 10000} {
			body = pages.Stack(body, ccForm(
				s.ccURL("account", "recharge_credit")+"&op=recharge",
				x.csrf,
				[][2]string{{"nonce", token()}, {"amount", strconv.FormatInt(amount, 10)}},
				pages.Stack(),
				"Add "+commerce.FormatAmount(amount)+" test credits",
			))
		}
	case "get_history", "dashboard":
		title = "Account History"
		if action == "dashboard" {
			title = "Merchant Dashboard"
			body = pages.Stack(body, ccText("Local transfers and gifts use the same account ledger. Merchant commissions and real settlement are not enabled."))
		}

		entries, err := s.commerce.History(ctx, x.username)
		if err != nil {
			return ccError(title, err), true
		}
		if len(entries) == 0 {
			body = pages.Stack(body, ccText("No transactions yet."))
		}

		for _, e := range entries {
			body = pages.Stack(body, pages.Stack(
				pages.Stack(
					ccText("Receipt #"+strconv.FormatInt(e.OperationID, 10)+" · "+e.Kind+" · "+commerce.FormatAmount(e.Delta)),
					ccText(e.Actor+" → "+e.Recipient+" · Balance "+commerce.FormatAmount(e.BalanceAfter)),
				),
				ccText(time.UnixMilli(e.CreatedAt).UTC().Format("2006-01-02 15:04 UTC")),
			))
		}
		if len(entries) == 100 {
			body = pages.Stack(body, ccText("Showing the latest 100 transactions."))
		}
	}

	for _, link := range [][3]string{
		{"account", "home", "My Account"},
		{"account", "recharge_credit", "Recharge Credit"},
		{"account", "transfer_credit", "Transfer Credit"},
		{"account", "get_history", "Account History"},
		{"store", "home", "Store"},
		{"merchant", "dashboard", "Merchant Dashboard"},
	} {
		body = pages.Stack(body, ccLink(s.ccURL(link[0], link[1]), link[2]))
	}

	return ccPage(200, title, body), true
}

func (s *Adapter) storePage(ctx context.Context, x *actor, method string, u *url.URL, form url.Values) pages.Result {
	q := u.Query()
	selection, products, err := selectProducts(q)
	if err != nil {
		return ccError("Store", err)
	}

	recipient := strings.ToLower(strings.TrimSpace(q.Get("username")))
	if recipient == "" {
		recipient = x.username
	}
	if len(q["username"]) > 1 || s.account(ctx, recipient) == nil {
		return ccError("Store", faults.ErrNotFound)
	}

	notice := ""
	if method == "POST" {
		switch q.Get("op") {
		case "purchase":
			if !roomFormFields(form, "csrf", "item", "recipient", "binding", "nonce") && !roomFormFields(form, "csrf", "item", "recipient", "binding", "nonce", "note") {
				return ccError("Store", faults.ErrInvalid)
			}

			item, ok := catalog.Lookup(form.Get("item"))
			recipient = form.Get("recipient")
			binding := reportBinding(x.csrf, "store-purchase", item.ID, recipient)
			if !ok || s.account(ctx, recipient) == nil || subtle.ConstantTimeCompare([]byte(binding), []byte(form.Get("binding"))) != 1 {
				return ccError("Store", faults.ErrInvalid)
			}

			purchase, err := s.workflow.Purchase(ctx, x.id, x.csrf, commerce.PurchaseCommand{
				Actor:     x.username,
				Recipient: recipient,
				Item:      item.ID,
				Nonce:     form.Get("nonce"),
				Note:      form.Get("note"),
			})
			receipt := purchase.Receipt

			if err != nil {
				if err == faults.ErrLimit {
					return ccPage(409, "Store", pages.Stack(ccText("Insufficient test credits."), ccLink(s.ccURL("account", "recharge_credit"), "Recharge Credit")))
				}

				return ccError("Store", err)
			}

			notice = item.Name + " delivered to " + recipient + ". Receipt #" + strconv.FormatInt(receipt.ID, 10) + "."
		case "theme":
			if !roomFormFields(form, "csrf", "item") {
				return ccError("Store", faults.ErrInvalid)
			}
			if err := s.workflow.ApplyTheme(ctx, x.id, x.csrf, form.Get("item")); err != nil {
				return ccError("Store", err)
			}

			notice = "Theme selected."
		case "rate":
			if !roomFormFields(form, "csrf", "item", "rating") {
				return ccError("Store", faults.ErrInvalid)
			}

			rating, err := strconv.Atoi(form.Get("rating"))
			if err != nil {
				return ccError("Store", faults.ErrInvalid)
			}
			if _, err := s.commerce.Rate(ctx, x.username, form.Get("item"), rating); err != nil {
				return ccError("Store", err)
			}

			notice = "Gift rating saved."
		default:
			return ccError("Store", faults.ErrInvalid)
		}
	}

	balance, err := s.commerce.Balance(ctx, x.username)
	if err != nil {
		return ccError("Store", err)
	}

	body := pages.Stack(ccText("Balance: "+commerce.FormatAmount(balance)+" test credits"), ccText("This local catalog uses test credits. No payment is taken."))
	if notice != "" {
		body = pages.Stack(body, ccText(notice))
	}
	if q.Get("ty") == "1" {
		body = pages.Stack(body, ccText("Gifts for "+recipient))
	}

	body = pages.Stack(body, s.productListing(ctx, x, selection, products, recipient))
	if selection.Kind == "theme" || selection.Kind == "" {
		body = pages.Stack(body, ccForm(s.ccURL("store", "home")+"&ty=5&op=theme", x.csrf, [][2]string{{"item", "default"}}, pages.Stack(), "Use original default theme"))
	}

	inventory, err := s.commerce.Inventory(ctx, x.username)
	if err != nil {
		return ccError("Store", err)
	}

	items, selected := inventory.Items, inventory.SelectedTheme

	body = pages.Stack(
		body,
		ccText("Your gifts, themes and emoticons"),
		ccText("Free chat emoticons: [smile] [heart] [gift]. Choose them from the native emoticon picker or type their token."),
	)
	seen := map[string]bool{}
	for _, owned := range items {
		item, ok := catalog.Lookup(owned.Item)
		if !ok {
			continue
		}

		body = pages.Stack(body, ccText(item.Name+" from "+owned.Sender))
		if item.Kind == "gift" {
			note := inventory.GiftNotes[owned.OperationID]

			if note != "" {
				body = pages.Stack(body, ccText(note))
			}
		}
		if item.Kind == "theme" && !seen[item.ID] {
			seen[item.ID] = true
			if selected == item.ID {
				body = pages.Stack(body, ccText("Selected theme"))
			} else {
				body = pages.Stack(body, ccForm(s.ccURL("store", "home")+"&ty=5&op=theme", x.csrf, [][2]string{{"item", item.ID}}, pages.Stack(), "Use "+item.Name))
			}
		}
	}
	if len(items) == 0 {
		body = pages.Stack(body, ccText("No gifts, themes or emoticon packs yet."))
	}

	return ccPage(
		200,
		"Store",
		pages.Stack(pages.Stack(body, ccLink(s.ccURL("account", "recharge_credit"), "Recharge Credit")), ccLink(s.ccURL("account", "get_history"), "Account History")),
	)
}
