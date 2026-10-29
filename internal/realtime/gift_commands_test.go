package realtime

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	chatstore "github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func giftTestRequest(t *testing.T, s *Application, sid string, kind byte, target, body string, want int16) {
	t.Helper()
	packet := protocol.New(500, 170).SetByte(1, 1).SetByte(3, kind).SetText(4, target).SetShort(6, 1).SetText(8, body).SetText(2, "forged-author")
	replies, _ := s.Handle(t.Context(), sid, packet)
	if len(replies) != 1 || replies[0].Opcode != want || replies[0].Transaction != 170 {
		t.Fatalf("%s: replies %+v, expected %d", body, replies, want)
	}
}

func giftTestResponse(t *testing.T, s *Application, sid string, kind byte, target string) string {
	t.Helper()
	packets := requestPacket(t, s, sid, nil)
	var body string
	count := 0
	for _, packet := range packets {
		if packet.Opcode != 500 {
			continue
		}
		count++
		gotKind, _ := packet.Byte(3)
		gotTarget, _ := packet.Text(4)
		gotContent, _ := packet.Short(6)
		if gotKind != kind || gotTarget != target || gotContent != 6 {
			t.Fatalf("wrong discovery route: %+v", packet)
		}
		if _, exists := packet.Fields[13]; exists {
			t.Fatal("discovery response disables native action rendering")
		}
		if kind == 1 {
			peer, _ := packet.Text(2)
			if peer != target {
				t.Fatalf("private gift response opened wrong tab: %s", peer)
			}
		}
		body, _ = packet.Text(8)
	}
	if count != 1 || !utf8.ValidString(body) || len([]rune(body)) > commerce.GiftCommandTextLimit || strings.ContainsAny(body, "\r\n") {
		t.Fatalf("response missing, duplicated or unbounded: %d %q", count, body)
	}
	return body
}

func TestGiftDiscoveryIsPrivateAcrossChatKindsAndDoesNotMutateHistory(t *testing.T) {
	s := newTestServer(t)
	a, a2, b, c := login(t, s, "alice"), login(t, s, "alice"), login(t, s, "bob"), login(t, s, "carol")
	group, _ := createTestGroup(t, s, a)
	for _, sid := range []string{a, a2, b, c} {
		joinRoom(t, s, sid, "Lobby")
	}
	for _, sid := range []string{a, a2, b, c} {
		requestPacket(t, s, sid, nil)
	}
	for _, conversation := range []struct {
		kind   byte
		target string
	}{{1, "bob"}, {2, group}, {3, "Lobby"}} {
		giftTestRequest(t, s, a, conversation.kind, conversation.target, "/gift list", 1)
		body := giftTestResponse(t, s, a, conversation.kind, conversation.target)
		if !strings.Contains(body, "local alphabetical order") || !strings.Contains(body, "test credits") || !strings.Contains(body, "More items") {
			t.Fatalf("catalog limits or local-price label lost: %s", body)
		}
		for _, sid := range []string{a2, b, c} {
			for _, packet := range requestPacket(t, s, sid, nil) {
				if packet.Opcode == 500 {
					t.Fatal("gift discovery leaked to another session")
				}
			}
		}
		history, err := s.store.History(t.Context(), "alice", conversation.kind, conversation.target, 0, 100)
		if err != nil || len(history) != 0 {
			t.Fatalf("catalog discovery stored as conversation: %+v %v", history, err)
		}
	}
	for _, user := range []string{"alice", "bob", "carol"} {
		balance, err := s.store.CreditBalance(t.Context(), user)
		if err != nil || balance != 0 {
			t.Fatalf("discovery changed wallet: %s %d %v", user, balance, err)
		}
		owned, err := s.store.OwnedStoreItems(t.Context(), user)
		if err != nil || len(owned) != 0 {
			t.Fatalf("discovery created inventory: %s %+v %v", user, owned, err)
		}
	}
}

func TestGiftDiscoverySubcommandsAndInvalidMutations(t *testing.T) {
	s := newTestServer(t)
	a, b := login(t, s, "alice"), login(t, s, "bob")
	requestPacket(t, s, a, nil)
	requestPacket(t, s, b, nil)
	for _, test := range []struct{ command, fragment string }{
		{"/gift", "/gift category [NAME]"},
		{"/gift new", "Historical release, featured and popularity rankings are unavailable"},
		{"/gift featured", "local alphabetical order"},
		{"/gift popular", "local alphabetical order"},
		{"/gift search ROSE", "Rose"},
		{"/gift search no-such-gift-zz", "No matching gifts"},
		{"/gift display Rose", "Price: 1.00 test credits (local price)"},
		{"/GIFT DISPLAY rose", "Gift: Rose"},
		{"/gift category", "Animals"},
		{"/gift category Animals", "Hug"},
		{"/gift gifts", "No received gifts"},
	} {
		giftTestRequest(t, s, a, 1, "bob", test.command, 1)
		body := giftTestResponse(t, s, a, 1, "bob")
		if !strings.Contains(body, test.fragment) {
			t.Fatalf("%s missing expected result: %s", test.command, body)
		}
	}

	for _, command := range []string{
		"/gift list extra",
		"/gift display",
		"/gift display not-a-gift",
		"/gift search",
		"/gift gifts bob",
		"/gift rate rose 6",
		"/gift bob Rose -m hello",
		"/gift " + strings.Repeat("x", 1100),
	} {
		giftTestRequest(t, s, a, 1, "bob", command, 0)
		if len(requestPacket(t, s, a, nil)) != 0 || len(requestPacket(t, s, b, nil)) != 0 {
			t.Fatalf("rejected gift command queued content: %s", command)
		}
	}
	history, err := s.store.History(t.Context(), "alice", 1, "bob", 0, 100)
	if err != nil || len(history) != 0 {
		t.Fatalf("gift discovery or unavailable send persisted: %+v %v", history, err)
	}
}

func TestGiftDiscoveryInventoryAuthorizationAndBackpressure(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.store.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", token(), 500); err != nil {
		t.Fatal(err)
	}

	if _, err := s.store.CreditOperation(t.Context(), "alice", "bob", "purchase", "rose", token(), 100); err != nil {
		t.Fatal(err)
	}
	a, b, d := login(t, s, "alice"), login(t, s, "bob"), login(t, s, "dave")
	requestPacket(t, s, a, nil)
	requestPacket(t, s, b, nil)
	requestPacket(t, s, d, nil)
	group, _ := createTestGroup(t, s, a)
	for _, sid := range []string{a, b, d} {
		requestPacket(t, s, sid, nil)
	}
	giftTestRequest(t, s, b, 1, "alice", "/gift gifts", 1)
	if body := giftTestResponse(t, s, b, 1, "alice"); !strings.Contains(body, "Rose from alice") {
		t.Fatalf("received ledger not reflected: %s", body)
	}
	giftTestRequest(t, s, a, 1, "bob", "/gift gifts", 1)
	if body := giftTestResponse(t, s, a, 1, "bob"); !strings.Contains(body, "No received gifts") {
		t.Fatalf("other user's inventory exposed: %s", body)
	}
	giftTestRequest(t, s, d, 2, group, "/gift list", 0)
	giftTestRequest(t, s, d, 3, "Lobby", "/gift list", 0)
	for len(s.sessions[a].push) < cap(s.sessions[a].push) {
		s.sessions[a].push <- events.New(0, events.Pong{})
	}
	giftTestRequest(t, s, a, 1, "bob", "/gift list", 0)
	clearGroupQueues(s, a)
	s.sessions[a].webOnly = true
	giftTestRequest(t, s, a, 1, "bob", "/gift list", 0)
	if len(s.sessions[a].push) != 0 {
		t.Fatal("browser-only session received native control traffic")
	}
	balance, err := s.store.CreditBalance(t.Context(), "alice")
	if err != nil || balance != 400 {
		t.Fatalf("discovery changed purchase balance: %d %v", balance, err)
	}
}

func TestGiftDiscoveryLongUnicodeDetailsRemainBounded(t *testing.T) {
	text := commerce.GiftCommandText("Gift", []string{strings.Repeat("🌹", 5000)})
	if !utf8.ValidString(text) || len([]rune(text)) > commerce.GiftCommandTextLimit || !strings.Contains(text, "More details in Menu > Store") {
		t.Fatal("long catalog details exceeded the output bound or broke Unicode")
	}
}

func TestGiftSendAcrossChatKindsAndRatingUpdates(t *testing.T) {
	s := newTestServer(t)
	a, b, c := login(t, s, "alice"), login(t, s, "bob"), login(t, s, "carol")
	if _, err := s.store.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", token(), 1000); err != nil {
		t.Fatal(err)
	}
	group, _ := createTestGroup(t, s, a)
	for _, sid := range []string{a, b, c} {
		joinRoom(t, s, sid, "Lobby")
		requestPacket(t, s, sid, nil)
	}
	for i, route := range []struct {
		kind   byte
		target string
	}{{1, "bob"}, {2, group}, {3, "Lobby"}} {
		for _, sid := range []string{a, b, c} {
			requestPacket(t, s, sid, nil)
		}
		note := fmt.Sprintf("Native gift %d & plus + equals = <b>literal</b>", i)
		giftTestRequest(t, s, a, route.kind, route.target, "/gift bob Rose -m "+note, 1)
		confirmations, receipts := 0, 0
		for _, packet := range requestPacket(t, s, a, nil) {
			body, _ := packet.Text(8)
			if strings.Contains(body, "Rose delivered to bob. Receipt #") {
				kind, _ := packet.Byte(3)
				target, _ := packet.Text(4)
				if kind != route.kind || target != route.target {
					t.Fatal("confirmation opened a different conversation", packet)
				}
				confirmations++
			} else if strings.Contains(body, "[gift:rose]") {
				receipts++
			}
		}
		if confirmations != 1 || receipts != 1 {
			t.Fatalf("requester confirmation/receipt count: %d/%d", confirmations, receipts)
		}
		got := 0
		for _, packet := range requestPacket(t, s, b, nil) {
			if packet.Opcode != 500 {
				continue
			}
			kind, _ := packet.Byte(3)
			body, _ := packet.Text(8)
			if kind != 1 || body != "alice sent bob a Rose [gift:rose] Message: "+note {
				t.Fatal("recipient gift was not a literal private receipt", packet)
			}
			got++
		}
		if got != 1 {
			t.Fatal("recipient gift missing or duplicated", got)
		}
		for _, packet := range requestPacket(t, s, c, nil) {
			if packet.Opcode == 500 {
				t.Fatal("gift command or note leaked to room/group observer", packet)
			}
		}
	}
	if balance, _ := s.store.CreditBalance(t.Context(), "alice"); balance != 700 {
		t.Fatal("unexpected local debit", balance)
	}
	if history, err := s.store.History(t.Context(), "alice", 2, group, 0, 100); err != nil || len(history) != 0 {
		t.Fatal("gift commands stored in group history", history, err)
	}
	for _, test := range []struct{ sid, target, command, want string }{
		{a, "bob", "/gift rate Rose 5", "Local average: 5.0/5 (1 rating(s))"},
		{a, "bob", "/gift rate Rose 5", "Local average: 5.0/5 (1 rating(s))"},
		{b, "alice", "/gift rate Rose 1", "Local average: 3.0/5 (2 rating(s))"},
		{a, "bob", "/gift rate Rose 3", "Your rating: 3/5. Local average: 2.0/5 (2 rating(s))"},
		{a, "bob", "/gift display rose", "Your rating: 3/5. Local average: 2.0/5 (2 rating(s))"},
		{b, "alice", "/gift gifts", "Message: Native gift 2 & plus + equals = <b>literal</b>"},
	} {
		giftTestRequest(t, s, test.sid, 1, test.target, test.command, 1)
		if body := giftTestResponse(t, s, test.sid, 1, test.target); !strings.Contains(body, test.want) {
			t.Fatalf("%s: %s", test.command, body)
		}
	}
}

func TestGiftSendPrivacyMembershipAndRecipientBackpressure(t *testing.T) {
	s := newTestServer(t)
	a, b, d := login(t, s, "alice"), login(t, s, "bob"), login(t, s, "dave")
	if _, err := s.store.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", token(), 1000); err != nil {
		t.Fatal(err)
	}
	group, _ := createTestGroup(t, s, a)
	joinRoom(t, s, a, "Lobby")
	for _, sid := range []string{a, b, d} {
		requestPacket(t, s, sid, nil)
	}

	for _, command := range []string{
		"/gift unknown-user Rose",
		"/gift bob theme-blue",
		"/gift bob Rose -m",
		"/gift bob Rose -m [gift:rose]",
		"/gift bob Rose -m [photo:1]",
		"/gift bob Rose -m [pack:cool]",
		"/gift bob Rose -m " + strings.Repeat("🌹", 81),
		"/gift bob Rose -m note\nnew line",
	} {
		giftTestRequest(t, s, a, 3, "Lobby", command, 0)
	}
	for _, route := range []struct {
		kind   byte
		target string
	}{{2, group}, {3, "Lobby"}} {
		giftTestRequest(t, s, d, route.kind, route.target, "/gift bob Rose", 0)
	}
	if err := s.store.SetIdentityPreference(t.Context(), "bob", "chat_privacy", "nobody"); err != nil {
		t.Fatal(err)
	}
	giftTestRequest(t, s, a, 3, "Lobby", "/gift bob Rose", 0)
	if err := s.store.SetIdentityPreference(t.Context(), "bob", "chat_privacy", "everyone"); err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetBlocked(t.Context(), "bob", "alice", true); err != nil {
		t.Fatal(err)
	}
	giftTestRequest(t, s, a, 3, "Lobby", "/gift bob Rose", 0)
	if err := s.store.SetBlocked(t.Context(), "bob", "alice", false); err != nil {
		t.Fatal(err)
	}
	if balance, _ := s.store.CreditBalance(t.Context(), "alice"); balance != 1000 {
		t.Fatal("rejected gift charged", balance)
	}
	for len(s.sessions[b].push) < cap(s.sessions[b].push) {
		s.sessions[b].push <- events.New(0, events.Pong{})
	}
	giftTestRequest(t, s, a, 3, "Lobby", "/gift bob Rose -m queued", 1)
	if messages, err := s.store.Pending(t.Context(), "bob", 100); err != nil || len(messages) != 1 || !strings.Contains(messages[0].Body, "Message: queued") {
		t.Fatal("full recipient queue lost durable gift", messages, err)
	}
	clearGroupQueues(s, b)
	gotGift := false
	for _, packet := range requestPacket(t, s, b, nil) {
		body, _ := packet.Text(8)
		if packet.Opcode == 500 && strings.Contains(body, "Message: queued") {
			peer, _ := packet.Text(2)
			if peer != "alice" {
				t.Fatal("recipient gift opened wrong peer", peer)
			}
			gotGift = true
		}
	}
	if !gotGift {
		t.Fatal("gift not delivered after recipient drains queue")
	}
	// Carol is offline; the purchase still completes and survives until login.
	requestPacket(t, s, a, nil)
	giftTestRequest(t, s, a, 3, "Lobby", "/gift carol Rose -m offline", 1)
	if messages, err := s.store.Pending(t.Context(), "carol", 100); err != nil || len(messages) != 1 || !strings.Contains(messages[0].Body, "Message: offline") {
		t.Fatal("offline recipient lost durable gift", messages, err)
	}
	if balance, _ := s.store.CreditBalance(t.Context(), "alice"); balance != 800 {
		t.Fatal("backpressure/offline delivery charged incorrectly", balance)
	}
}

func TestGiftNativeExactPacketConcurrentRetry(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	requestPacket(t, s, a, nil)
	if _, err := s.store.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", token(), 1000); err != nil {
		t.Fatal(err)
	}
	packet := protocol.New(500, 171).SetByte(1, 1).SetByte(3, 1).SetText(4, "bob").SetShort(6, 1).SetText(8, "/gift bob Rose -m once")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			replies, _ := s.Handle(t.Context(), a, packet)
			if len(replies) != 1 || replies[0].Opcode != 1 {
				t.Error("native retry failed", replies)
			}
		}()
	}
	wg.Wait()
	if balance, _ := s.store.CreditBalance(t.Context(), "alice"); balance != 900 {
		t.Fatal("exact packet retries charged again", balance)
	}
	if owned, _ := s.store.OwnedStoreItems(t.Context(), "bob"); len(owned) != 1 {
		t.Fatal("exact packet retries duplicated inventory", owned)
	}
	if messages, _ := s.store.Pending(t.Context(), "bob", 100); len(messages) != 1 || messages[0].Markup != "" {
		t.Fatal("exact packet retry duplicated or formatted gift note", messages)
	}
	rating, err := s.store.GiftRating(t.Context(), "rose", "alice")
	if err != nil || rating != (chatstore.GiftRatingSummary{}) {
		t.Fatal("sending implicitly rated a gift", rating, err)
	}
}
