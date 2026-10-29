package realtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
)

// giftCommand implements recovered syntax against the local catalog and test
// credit ledger. Command replies belong only to the requesting native session;
// completed purchases create a separate durable private gift receipt.
func (s *Coordinator) giftCommand(ctx context.Context, x *session, p *commands.Request, m chat.Message) (*events.Event, bool) {
	text := strings.TrimSpace(m.Body)
	name, rest := text, ""
	if split := strings.IndexFunc(text, unicode.IsSpace); split >= 0 {
		name, rest = text[:split], strings.TrimSpace(text[split:])
	}
	if !strings.EqualFold(name, "/gift") {
		return nil, false
	}
	fail := func(message string) (*events.Event, bool) { return failure(p, message), true }
	if len(text) > 1024 {
		return fail("Gift commands must contain at most 1024 bytes")
	}
	if x.webOnly {
		return fail("Open Store to browse gifts")
	}
	switch m.Kind {
	case 1:
		// The shared handler has already checked the private peer and privacy.
	case 2:
		if _, err := s.chatService.Group(ctx, m.Conversation, x.username); err != nil {
			return fail("Join this group conversation before using gift commands")
		}
	case 3:
		room, err := s.roomsService.Room(ctx, m.Conversation)
		if err != nil || !x.rooms[room.Name] {
			return fail("Join the chat room before using gift commands")
		}
		m.Conversation = room.Name
	default:
		return fail("Open a private chat, group conversation or chat room")
	}
	if len(x.push) == cap(x.push) {
		return fail("Your connection is busy; try again")
	}
	subcommand, arg := rest, ""
	if split := strings.IndexFunc(rest, unicode.IsSpace); split >= 0 {
		subcommand, arg = rest[:split], strings.TrimSpace(rest[split:])
	}
	subcommand = strings.ToLower(subcommand)
	items := commerce.GiftCatalogItems()
	heading := "Gifts"
	lines := []string{}
	flushGifts := false
	switch subcommand {
	case "":
		lines = append(lines,
			"/gift list; /gift new; /gift featured; /gift popular; /gift search TEXT; /gift display NAME; /gift category [NAME]; /gift gifts.",
			"/gift USER NAME [-m MESSAGE] sends a gift using test credits. /gift rate NAME RATING uses a local 1 to 5 scale.",
		)
	case "list", "new", "featured", "popular":
		if arg != "" {
			return fail("Usage: /gift " + subcommand)
		}
		heading = "Gifts: " + subcommand + " (local alphabetical order)"
		if subcommand != "list" {
			lines = append(lines, "Historical release, featured and popularity rankings are unavailable; this is a local catalog view.")
		}
		lines = append(lines, commerce.GiftItemLines(items)...)
	case "search":
		if arg == "" || len(arg) > 120 {
			return fail("Usage: /gift search TEXT (1 to 120 bytes)")
		}
		found := []commerce.StoreItem{}
		for _, item := range items {
			if strings.Contains(strings.ToLower(item.Name+" "+item.Description), strings.ToLower(arg)) {
				found = append(found, item)
			}
		}
		heading = "Gift search: " + arg
		lines = append(lines, commerce.GiftItemLines(found)...)
	case "display":
		if arg == "" || len(arg) > 120 {
			return fail("Usage: /gift display NAME (or catalog ID)")
		}
		found := false
		for _, item := range items {
			if !strings.EqualFold(item.ID, arg) && !strings.EqualFold(item.Name, arg) {
				continue
			}
			found = true
			heading = "Gift: " + item.Name
			lines = append(lines, "Category: "+commerce.GiftCategory(item)+". Price: "+commerce.FormatAmount(item.Price)+" test credits (local price).", item.Description)
			lines = append(lines, "Artwork: "+strings.ReplaceAll(item.ArtworkEvidence, "_", " ")+". Open Menu > Store to preview and send.")
			rating, err := s.commerceService.Rating(ctx, item.ID, x.username)
			if err != nil {
				return fail("Gift ratings could not be loaded")
			}
			lines = append(lines, commerce.GiftRatingText(rating))
			break
		}
		if !found {
			return fail("Gift not found. Use /gift search TEXT or open Store")
		}
	case "category":
		if len(arg) > 120 {
			return fail("Usage: /gift category [NAME] (at most 120 bytes)")
		}
		if arg == "" {
			categories := map[string]bool{}
			for _, item := range items {
				categories[commerce.GiftCategory(item)] = true
			}
			for category := range categories {
				lines = append(lines, category)
			}
			sort.Strings(lines)
			heading = "Gift categories (local placement unless recovered)"
		} else {
			found := []commerce.StoreItem{}
			for _, item := range items {
				if strings.EqualFold(commerce.GiftCategory(item), arg) {
					found = append(found, item)
				}
			}
			heading = "Gift category: " + arg
			lines = append(lines, commerce.GiftItemLines(found)...)
		}
	case "gifts":
		if arg != "" {
			return fail("Usage: /gift gifts (your received gifts only)")
		}
		owned, err := s.commerceService.OwnedItems(ctx, x.username)
		if err != nil {
			return fail("Your received gifts could not be loaded")
		}
		for _, ownedItem := range owned {
			item, ok := catalog.Lookup(ownedItem.Item)
			if ok && item.Kind == "gift" {
				line := item.Name + " from " + ownedItem.Sender
				note, err := s.commerceService.GiftNote(ctx, ownedItem.OperationID, x.username)
				if err != nil {
					return fail("Your received gift notes could not be loaded")
				}
				if note != "" {
					line += " - Message: " + note
				}
				lines = append(lines, line)
			}
		}
		heading = "Your received gifts (newest first)"
		if len(lines) == 0 {
			lines = append(lines, "No received gifts.")
		}
	case "rate", "rating":
		split := strings.LastIndexFunc(arg, unicode.IsSpace)
		if split < 1 {
			return fail("Usage: /gift rate NAME RATING (local integer scale: 1 to 5)")
		}
		item, ok := commerce.FindGift(strings.TrimSpace(arg[:split]))
		rating, err := strconv.Atoi(strings.TrimSpace(arg[split:]))
		if !ok || err != nil || rating < 1 || rating > 5 {
			return fail("Use a known gift and an integer rating from 1 to 5 (local scale)")
		}
		summary, err := s.commerceService.Rate(ctx, x.username, item.ID, rating)
		if err != nil {
			return fail("Your gift rating could not be saved")
		}
		heading = "Gift rating: " + item.Name
		lines = append(lines, commerce.GiftRatingText(summary))
	default:
		recipient := subcommand
		if s.accounts[recipient] == nil {
			return fail("Gift recipient not found. Usage: /gift USER NAME [-m MESSAGE]")
		}
		giftName, note, err := commerce.ParseGiftSend(arg)
		item, ok := commerce.FindGift(giftName)
		if err != nil || !ok {
			return fail("Use /gift USER NAME [-m MESSAGE]. Notes must be at most 320 UTF-8 bytes, without controls or gift/photo/pack tokens")
		}
		if !s.canContact(ctx, x.username, recipient) || s.blocksIncoming(ctx, recipient, x.username) {
			return fail("This user does not accept gifts from you")
		}
		purchase, err := s.commerceService.Purchase(ctx, commerce.PurchaseCommand{Actor: x.username, Recipient: recipient, Item: item.ID, Nonce: p.Nonce, Note: note})
		if err != nil {
			switch {
			case errors.Is(err, faults.ErrLimit):
				return fail("Insufficient test credits. Open Menu > Recharge Credit")
			case errors.Is(err, faults.ErrForbidden), errors.Is(err, faults.ErrBlocked):
				return fail("This user does not accept gifts from you")
			case errors.Is(err, faults.ErrConflict):
				return fail("This gift request conflicts with its earlier receipt")
			default:
				var detail *faults.ServiceError
				if errors.As(err, &detail) {
					return fail(detail.Message)
				}
				return fail("Your gift could not be sent")
			}
		}
		heading = "Gift sent"
		lines = append(lines,
			fmt.Sprintf("%s delivered to %s. Receipt #%d.", item.Name, recipient, purchase.Receipt.ID),
			"Cost: "+commerce.FormatAmount(purchase.Receipt.Amount)+" local test credits. No payment was taken.",
		)
		flushGifts = purchase.DurableMessages
	}
	response := commerce.GiftCommandText(heading, lines)
	packet := s.messagePacket(ctx, chat.Message{Kind: m.Kind, Conversation: m.Conversation, Sender: x.username, Body: response, Action: true}, x.username)
	if m.Kind == 1 {
		if contact, err := s.contactsService.ContactByUsername(ctx, x.username, m.Conversation); err == nil {
			data := packet.Data.(events.Message)
			data.ContactID = &contact.ID
			packet.Data = data
		}
	}
	x.push <- packet
	if flushGifts {
		// Queue saturation/offline recipients do not invalidate a committed
		// gift. The durable audience delivers it on the next successful poll.
		for _, other := range s.sessions {
			s.flushPending(ctx, other)
		}
	}
	return events.Ack(p.Correlation), true
}
