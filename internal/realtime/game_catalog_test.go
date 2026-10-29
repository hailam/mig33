package realtime

import (
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/events"
)

func TestHistoricalGameReferencesCannotBePlayedOrCharged(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	u := s.featureURL("bot", "bot_list")
	index := fetchReport(t, s, a, "GET", u, "")
	if index.Code != 200 ||
		!strings.Contains(index.Body.String(), "Historical games") ||
		!strings.Contains(index.Body.String(), "Roll Dice") ||
		!strings.Contains(index.Body.String(), "Ask Magic 8 Ball") {
		t.Fatalf("catalog page: %d %s", index.Code, index.Body.String())
	}
	for _, item := range catalog.Games() {
		if item.Availability != "reference_placeholder" {
			continue
		}
		if item.PlayTarget != "" || item.SourceURL == "" || !strings.Contains(index.Body.String(), item.Name+" (reference)") {
			t.Fatalf("historical reference misrepresented as playable: %+v", item)
		}
		view := fetchReport(t, s, a, "GET", u+"&game="+item.ID, "")
		if view.Code != 200 || strings.Contains(view.Body.String(), "<form") || !strings.Contains(view.Body.String(), "Reference only") {
			t.Fatalf("reference has misleading action: %s %d %s", item.ID, view.Code, view.Body.String())
		}
		f := serviceTestForm(s, a, "play", item.ID, "")
		if got := fetchReport(t, s, a, "POST", u, f.Encode()); got.Code != 400 {
			t.Fatalf("reference game played: %s %d", item.ID, got.Code)
		}
		f = serviceTestForm(s, a, "play", "roll", "")
		if got := fetchReport(t, s, a, "POST", u+"&game="+item.ID, f.Encode()); got.Code != 405 {
			t.Fatalf("read-only reference URL accepted a play: %s %d", item.ID, got.Code)
		}
	}
	rows, err := s.store.ServiceEntries(t.Context(), "game", "alice", "")
	if err != nil || len(rows) != 0 {
		t.Fatalf("references created game results: %+v %v", rows, err)
	}
	for _, query := range []string{"&game=unknown", "&game=one&game=dice"} {
		view := fetchReport(t, s, a, "GET", u+query, "")
		if view.Code != 404 && view.Code != 400 {
			t.Fatalf("invalid catalog selection: %d", view.Code)
		}
	}
	if got := fetchReport(t, s, "", "GET", u, ""); got.Code != 401 {
		t.Fatal("unauthenticated Games page")
	}
}

func TestRecoveredGamesAndBotStopCommands(t *testing.T) {
	s := newTestServer(t)
	a, b := login(t, s, "alice"), login(t, s, "bob")
	joinRoom(t, s, a, "Lobby")
	joinRoom(t, s, b, "Lobby")
	requestPacket(t, s, a, nil)
	requestPacket(t, s, b, nil)
	roomCommand(t, s, b, "Lobby", "/games", 1)
	catalog := ""
	for _, packet := range requestPacket(t, s, b, nil) {
		if packet.Opcode == 500 {
			text, _ := packet.Text(8)
			catalog += text
		}
	}
	if !strings.Contains(catalog, "Historical references (not playable)") || !strings.Contains(catalog, "Moonbase") || !strings.Contains(catalog, "/bot stop") {
		t.Fatal("catalog was not sent to requester")
	}
	if containsText(requestPacket(t, s, a, nil), catalog) {
		t.Fatal("private catalog listing broadcast to room")
	}
	roomCommand(t, s, b, "Lobby", "/games extra", 0)
	roomCommand(t, s, b, "Gaming", "/games", 0)
	for len(s.sessions[b].push) < cap(s.sessions[b].push) {
		s.sessions[b].push <- events.New(0, events.Pong{})
	}
	roomCommand(t, s, b, "Lobby", "/games", 0)
	requestPacket(t, s, b, nil)
	linkedRoomFixture(t, s)
	joinRoom(t, s, a, "Club One")
	joinRoom(t, s, b, "Club One")
	requestPacket(t, s, a, nil)
	requestPacket(t, s, b, nil)
	roomCommand(t, s, a, "Club One", "/bot dice", 1)
	roomCommand(t, s, b, "Club One", "/bot stop", 0)
	state, err := s.store.GroupRoomControls(t.Context(), "Club One")
	if err != nil || state.Bot != "dice" {
		t.Fatalf("unauthorized stop changed bot: %+v %v", state, err)
	}
	roomCommand(t, s, a, "Club One", "/BOT STOP", 1)
	state, err = s.store.GroupRoomControls(t.Context(), "Club One")
	if err != nil || state.Bot != "" {
		t.Fatalf("authorized stop did not clear bot: %+v %v", state, err)
	}
	roomCommand(t, s, b, "Club One", "/play", 0)
}
