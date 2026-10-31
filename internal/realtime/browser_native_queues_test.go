package realtime

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	chatstore "github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func browserQueueLogin(t *testing.T, s *Application, username string) (*session, *http.Cookie) {
	t.Helper()
	w := wapRequest(s, "", "GET", wapChatPath, nil)
	cookies := w.Result().Cookies()
	if w.Code != 200 || len(cookies) != 1 {
		t.Fatal("browser login form", w.Code)
	}
	w = wapRequest(s, "", "POST", wapChatPath, url.Values{
		"op": {"login"}, "csrf": {cookies[0].Value},
		"username": {username}, "password": {username + "-demo"},
	}, cookies...)
	if w.Code != 303 {
		t.Fatal("browser login", w.Code, w.Body.String())
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "mig33_wap" {
			x := s.sessions[cookie.Value]
			if x == nil || !x.webOnly {
				t.Fatal("browser session missing")
			}
			return x, cookie
		}
	}
	t.Fatal("browser session cookie missing")
	return nil, nil
}

func browserQueueJoin(t *testing.T, s *Application, x *session, cookie *http.Cookie, room string) {
	t.Helper()
	w := wapRequest(s, "", "POST", wapChatPath+"?kind=room&target="+url.QueryEscape(room), url.Values{"csrf": {x.csrf}, "op": {"join"}}, cookie)
	if w.Code != 200 || !x.rooms[room] {
		t.Fatal("browser room join", w.Code, w.Body.String())
	}
}

func TestBrowserThemeAndAvatarChangesKeepNativeQueuesIsolated(t *testing.T) {
	s := newTestServer(t)
	alice, bob := login(t, s, "alice"), login(t, s, "bob")
	browser, _ := browserQueueLogin(t, s, "bob")
	item, ok := chatstore.LookupStoreItem("theme-orange")
	if !ok {
		t.Fatal("theme fixture missing")
	}
	if _, err := s.store.CreditOperation(t.Context(), "bob", "bob", "test-recharge", "", token(), item.Price); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.CreditOperation(t.Context(), "bob", "bob", "purchase", item.ID, token(), item.Price); err != nil {
		t.Fatal(err)
	}
	requestPacket(t, s, alice, nil)
	requestPacket(t, s, bob, nil)
	// Each theme/avatar update must reach the native client without leaving
	// unread packets in the simultaneously signed-in browser session.
	for i := 0; i < 135; i++ {
		s.mu.Lock()
		err := s.applyStoreTheme(t.Context(), "bob", item.ID)
		s.broadcastAvatar(t.Context(), "alice", "")
		s.mu.Unlock()
		if err != nil {
			t.Fatalf("theme update %d blocked by browser: %v", i, err)
		}
		packets := requestPacket(t, s, bob, nil)
		seenTheme, seenAvatar := false, false
		for _, packet := range packets {
			seenTheme = seenTheme || packet.Opcode == 1000
			seenAvatar = seenAvatar || packet.Opcode == 423
		}
		if !seenTheme || !seenAvatar {
			t.Fatalf("native theme/avatar update lost at %d", i)
		}
		requestPacket(t, s, alice, nil)
	}
	if len(browser.push) != 0 {
		t.Fatalf("browser accumulated %d native packets", len(browser.push))
	}
	// A queue left by an older process path cannot block later selection.
	for len(browser.push) < cap(browser.push) {
		browser.push <- events.New(0, events.Pong{})
	}
	s.mu.Lock()
	err := s.applyStoreTheme(t.Context(), "bob", item.ID)
	if err == nil {
		err = s.restoreStoreTheme(t.Context(), browser)
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal("browser queue prevented persisted theme selection", err)
	}
	if selected, err := s.store.SelectedStoreTheme(t.Context(), "bob"); err != nil || selected != item.ID {
		t.Fatal("theme selection was not persisted", selected, err)
	}
}

func TestBrowserQueueCannotBlockRoomControlsOrOwnActions(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	browser, cookie := browserQueueLogin(t, s, "bob")
	linkedRoomFixture(t, s)
	joinRoom(t, s, alice, "Club One")
	browserQueueJoin(t, s, browser, cookie, "Club One")
	requestPacket(t, s, alice, nil)
	for len(browser.push) < cap(browser.push) {
		browser.push <- events.New(0, events.Pong{})
	}
	roomCommand(t, s, alice, "Club One", "/silence on", 1)
	controls, err := s.store.GroupRoomControls(t.Context(), "Club One")
	if err != nil || !controls.Silenced {
		t.Fatal("room control did not persist", controls, err)
	}
	roomCommand(t, s, alice, "Club One", "/silence off", 1)
	f := wapSendForm(s, browser.id, "/roll")
	f.Set("format", "plain")
	w := wapRequest(s, "", "POST", wapChatPath+"?kind=room&target=Club+One", f, cookie)
	if w.Code != 200 {
		t.Fatal("browser action blocked by unused native queue", w.Code, w.Body.String())
	}
	history, err := s.store.History(t.Context(), "bob", 3, "Club One", 0, 10)
	if err != nil || len(history) != 1 || !history[0].Action || history[0].Sender != "bob" {
		t.Fatal("browser action missing from authorized history", history, err)
	}
	if len(s.roomParticipants("Club One")) != 2 || !browser.rooms["Club One"] {
		t.Fatal("browser participation was changed by native queue handling")
	}
}

func TestBrowserKickAndRoomDeletionRemoveMembershipWithoutNativePopups(t *testing.T) {
	s := newTestServer(t)
	alice, bob := login(t, s, "alice"), login(t, s, "bob")
	browser, cookie := browserQueueLogin(t, s, "bob")
	hasPopup := func(packets []*protocol.Packet, text string) bool {
		for _, packet := range packets {
			body, _ := packet.Text(2)
			if packet.Opcode == 5 && strings.Contains(body, text) {
				return true
			}
		}
		return false
	}
	linkedRoomFixture(t, s)
	for _, room := range []string{"Club One", "Club Two"} {
		joinRoom(t, s, alice, room)
		joinRoom(t, s, bob, room)
		browserQueueJoin(t, s, browser, cookie, room)
	}
	requestPacket(t, s, bob, nil)
	replies, _ := s.Handle(t.Context(), alice, protocol.New(706, 51).SetText(1, "Club One").SetText(2, "bob"))
	if replies[0].Opcode != 1 || browser.rooms["Club One"] || s.sessions[bob].rooms["Club One"] {
		t.Fatal("kick did not remove both sessions", replies)
	}
	if len(browser.push) != 0 || !hasPopup(requestPacket(t, s, bob, nil), "You were kicked from Club One") {
		t.Fatal("kick popup did not stay on native transport")
	}
	f := serviceTestForm(s, alice, "delete", "Club Two", "Club Two")
	w := fetchReport(t, s, alice, "POST", s.roomPageAction("setup")+"&manage=1&roomName=Club+Two", f.Encode())
	if w.Code != 200 || browser.rooms["Club Two"] || s.sessions[bob].rooms["Club Two"] {
		t.Fatal("deletion did not remove both sessions", w.Code, w.Body.String())
	}
	if len(browser.push) != 0 || !hasPopup(requestPacket(t, s, bob, nil), "Club Two was deleted") {
		t.Fatal("deletion popup did not stay on native transport")
	}
}
