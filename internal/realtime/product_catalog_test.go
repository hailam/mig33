package realtime

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func TestConfiguredDefaultThemeRestoreIsFreeAndPreservesFields(t *testing.T) {
	s := newTestServer(t)
	x := s.sessions[login(t, s, "alice")]
	if _, err := s.store.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", token(), 1000); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.CreditOperation(t.Context(), "alice", "alice", "purchase", "theme-blue", token(), 300); err != nil {
		t.Fatal(err)
	}
	if err := s.applyStoreTheme(t.Context(), "alice", "theme-blue"); err != nil {
		t.Fatal(err)
	}
	if err := s.applyStoreTheme(t.Context(), "alice", "default"); err != nil {
		t.Fatal(err)
	}
	if selected, err := s.store.SelectedStoreTheme(t.Context(), "alice"); err != nil || selected != "default" {
		t.Fatal(selected, err)
	}
	if balance, _ := s.store.CreditBalance(t.Context(), "alice"); balance != 700 {
		t.Fatal("default reset charged credits", balance)
	}

	configured, err := protocol.Decode(bytes.NewReader(fixtureAsset(t, "default_v2")))
	if err != nil {
		t.Fatal(err)
	}
	var received *protocol.Packet
	for len(x.push) > 0 {
		p := fixturePacket(<-x.push)
		if p.Opcode == 1000 {
			received = p
		}
	}
	if received == nil || !reflect.DeepEqual(received.Fields, configured.Fields) {
		t.Fatal("default theme bytes changed")
	}

	challengeReply, otherID := s.Handle(t.Context(), "", protocol.New(200, 21).SetText(5, "alice"))
	challenge, _ := challengeReply[0].Text(1)
	loginReply, _ := s.Handle(t.Context(), otherID, protocol.New(202, 22).SetInt(1, protocol.JavaHash(challenge+"alice-demo")))
	if loginReply[0].Opcode != 203 {
		t.Fatal("login failed", loginReply)
	}

	rosterReply, _ := s.Handle(t.Context(), otherID, protocol.New(400, 23))
	received = nil
	for _, p := range rosterReply {
		if p.Opcode == 1000 {
			received = p
		}
	}
	if received == nil || !reflect.DeepEqual(received.Fields, configured.Fields) {
		t.Fatal("returning device did not receive configured default theme")
	}
}

func TestCatalogPublicMetadataAndPlaceholderCompleteness(t *testing.T) {
	s := newTestServer(t)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", "/api/catalog", nil))
	var response struct {
		Products  []catalog.Product     `json:"products"`
		Games     []catalog.CatalogGame `json:"games"`
		Providers map[string]string     `json:"providers"`
	}
	if w.Code != 200 ||
		json.Unmarshal(w.Body.Bytes(), &response) != nil ||
		len(response.Products) != 78 ||
		len(response.Games) != 25 ||
		response.Providers["external_im"] != "disconnected_tbd" {
		t.Fatalf("catalog response: %d %s", w.Code, w.Body.String())
	}

	seen := map[string]bool{}
	for _, item := range response.Products {
		if seen[item.ID] || item.Price <= 0 || item.PriceEvidence != "local_test_price" || item.ArtworkEvidence == "" {
			t.Fatalf("invalid product evidence: %+v", item)
		}
		seen[item.ID] = true
		if item.NameEvidence == "recovered_name" && len(item.SourceURLs) == 0 {
			t.Fatal("missing source", item.ID)
		}
		if item.Kind == "gift" || item.Kind == "emoticon_pack" {
			found := false
			for _, emote := range catalog.Emoticons() {
				if (item.Kind == "gift" && emote.Token == "[gift:"+item.ID+"]") || emote.Pack == item.ID {
					found = true
					if packet := fixturePacket(events.New(0, events.EmoticonResource{Token: emote.Token})); packet.Opcode != 914 || len(packet.Fields[4]) == 0 {
						t.Fatal(item.ID, "missing encoded emoticon content")
					}
				}
			}
			if !found {
				t.Fatal("product has no usable representation", item.ID)
			}
		}
	}

	for _, game := range response.Games {
		if game.Availability == "reference_placeholder" && game.PlayTarget != "" {
			t.Fatal("reference pretends to be playable", game.ID)
		}
	}

	for _, c := range []struct {
		method, path string
		status       int
	}{{"POST", "/api/catalog", 405}, {"GET", "/api/catalog?username=alice", 400}} {
		w = httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(c.method, c.path, nil))
		if w.Code != c.status {
			t.Fatal(c, w.Code)
		}
	}
}

func TestExpandedCatalogPurchaseEntitlementReplayAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.sqlite")
	s, err := newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	x := s.sessions[login(t, s, "alice")]
	if _, err := s.store.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", token(), 1000); err != nil {
		t.Fatal(err)
	}
	if err := s.validateEmoticonMessage(t.Context(), "alice", "[pack:lovely-pack]"); err == nil {
		t.Fatal("unowned pack accepted")
	}

	for _, purchase := range []struct{ item, recipient string }{{"big-hug", "bob"}, {"emotes-lovely-pack", "alice"}} {
		item, _ := catalog.Lookup(purchase.item)
		u, _ := url.Parse(s.ccURL("store", "home") + "&item=" + item.ID + "&username=" + purchase.recipient)
		detail := ancillaryPage(t, s, x, "GET", u, nil)
		if detail.Status != 200 ||
			!bytes.Contains(detail.Body, []byte("Buy "+item.Name)) ||
			!bytes.Contains(detail.Body, []byte("No, go back")) ||
			!bytes.Contains(detail.Body, []byte("placeholder")) {
			t.Fatal("detail missing confirmation", string(detail.Body))
		}
		form := url.Values{
			"csrf":      {x.csrf},
			"item":      {item.ID},
			"recipient": {purchase.recipient},
			"binding":   {fixtureReportBinding(x.csrf, "store-purchase", item.ID, purchase.recipient)},
			"nonce":     {token()},
		}
		u, _ = url.Parse(s.ccURL("store", "home") + "&op=purchase")
		if item.ID == "big-hug" {
			const note = "<b>Hello & plus + equals = 50% ✓</b>"
			form.Set("note", note)
		}
		for i := 0; i < 2; i++ {
			result := ancillaryPage(t, s, x, "POST", u, form)
			if result.Status != 200 {
				t.Fatal(result.Status, string(result.Body))
			}
		}
	}
	if balance, _ := s.store.CreditBalance(t.Context(), "alice"); balance != 700 {
		t.Fatal("duplicate charge", balance)
	}
	history, err := s.store.History(t.Context(), "bob", 1, "alice", 0, 100)
	if err != nil || len(history) != 1 || !strings.Contains(history[0].Body, "[gift:big-hug]") {
		t.Fatal("gift receipt", history, err)
	}
	if !strings.Contains(history[0].Body, "<b>Hello & plus + equals = 50% ✓</b>") {
		t.Fatal("gift note lost", history[0].Body)
	}
	owned, err := s.store.OwnedStoreItems(t.Context(), "bob")
	if err != nil || len(owned) != 1 {
		t.Fatal(owned, err)
	}
	note, err := s.store.GiftNote(t.Context(), owned[0].OperationID, "bob")
	if err != nil || !strings.Contains(note, "50% ✓") {
		t.Fatal("stored note", note, err)
	}
	if err := s.validateEmoticonMessage(t.Context(), "alice", "[pack:lovely-pack]"); err != nil {
		t.Fatal(err)
	}
	if err := s.validateEmoticonMessage(t.Context(), "bob", "[pack:lovely-pack]"); err == nil {
		t.Fatal("ownership leaked")
	}
	if err := s.validateEmoticonMessage(t.Context(), "alice", "[gift:big-hug]"); err == nil {
		t.Fatal("forged gift accepted")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.validateEmoticonMessage(t.Context(), "alice", "[pack:lovely-pack]"); err != nil {
		t.Fatal("entitlement lost on restart", err)
	}
	packet, err := s.initialEmoticonCatalog(t.Context(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	aliases, _ := fixturePacket(packet).Text(1)
	if !strings.Contains(aliases, "[pack:lovely-pack]") {
		t.Fatal("missing picker alias", aliases)
	}
	for _, token := range []string{"[pack:lovely-pack]", "[gift:big-hug]"} {
		packet := fixturePacket(s.catalogEmoticon(&commands.Request{Correlation: 9}, token))
		if packet.Opcode != 914 {
			t.Fatal("download unavailable", token)
		}
	}
}

func TestStoreGiftRatingAndNoteVisibility(t *testing.T) {
	s := newTestServer(t)
	a := s.sessions[login(t, s, "alice")]
	b := s.sessions[login(t, s, "bob")]
	u, _ := url.Parse(s.ccURL("store", "home") + "&item=big-hug&username=bob&op=rate")
	detail := ancillaryPage(t, s, a, "GET", u, nil)
	if !bytes.Contains(detail.Body, []byte("item=big-hug&username=bob&v=midlet&op=rate")) {
		t.Fatal("rating action loses gift recipient", string(detail.Body))
	}
	form := url.Values{"csrf": {a.csrf}, "item": {"big-hug"}, "rating": {"4"}}
	for i := 0; i < 2; i++ {
		result := ancillaryPage(t, s, a, "POST", u, form)
		if result.Status != 200 || !bytes.Contains(result.Body, []byte("Gift rating saved")) {
			t.Fatal(result.Status, string(result.Body))
		}
		if !bytes.Contains(result.Body, []byte(`name="recipient" value="bob"`)) {
			t.Fatal("rating changed gift recipient", string(result.Body))
		}
	}
	rating, err := s.store.GiftRating(t.Context(), "big-hug", "alice")
	if err != nil || rating.Count != 1 || rating.Average != 4 || rating.OwnRating != 4 {
		t.Fatal(rating, err)
	}
	form.Set("csrf", "wrong")
	if result := ancillaryPage(t, s, a, "POST", u, form); result.Status != 403 {
		t.Fatal("rating CSRF", result.Status)
	}
	if _, err := s.store.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", token(), 1000); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.CreditOperationWithNote(t.Context(), "alice", "bob", "purchase", "big-hug", token(), 100, "<b>Hi & welcome</b>"); err != nil {
		t.Fatal(err)
	}
	u, _ = url.Parse(s.ccURL("store", "home"))
	result := ancillaryPage(t, s, b, "GET", u, nil)
	if result.Status != 200 || !bytes.Contains(result.Body, []byte("&lt;b&gt;Hi &amp; welcome&lt;/b&gt;")) {
		t.Fatal("gift note must render as escaped text", string(result.Body))
	}
}

func TestStoreCatalogNavigationAndMalformedSelection(t *testing.T) {
	s := newTestServer(t)
	x := s.sessions[login(t, s, "alice")]
	for _, query := range []string{"page=-1", "page=no", "page=0&page=1", "kind=gift&ty=5", "kind=no", "item=missing", "kind=theme&item=big-hug"} {
		u, _ := url.Parse(s.ccURL("store", "home") + "&" + query)
		result := ancillaryPage(t, s, x, "GET", u, nil)
		if result.Status != 400 && result.Status != 404 {
			t.Fatal(query, result.Status)
		}
	}
	u, _ := url.Parse(s.ccURL("store", "home") + "&kind=emoticon_pack&page=1&username=bob")
	result := ancillaryPage(t, s, x, "GET", u, nil)
	if result.Status != 200 ||
		!bytes.Contains(result.Body, []byte("Previous products")) ||
		!bytes.Contains(result.Body, []byte("Next products")) ||
		!bytes.Contains(result.Body, []byte("username=bob")) {
		t.Fatal("pagination lost selection", string(result.Body))
	}
}
