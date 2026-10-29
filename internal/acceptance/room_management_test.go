package acceptance

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/protocol"
)

const roomInfoURL = "http://127.0.0.1:8080/sites/index.php?c=chatroom&a=setup&v=midlet&roomName=Lobby"

func TestRoomInfoMultipleTargetsAndPromotionAfterRendering(t *testing.T) {
	s := newTestServer(t)
	a, b, c := login(t, s, "alice"), login(t, s, "bob"), login(t, s, "carol")
	for _, sid := range []string{a, b, c} {
		joinRoom(t, s, sid, "Lobby")
	}
	w := fetchReport(t, s, a, "GET", roomInfoURL, "")
	forms := regexp.MustCompile(`<form\b[^>]*>.*?</form>`).FindAllString(w.Body.String(), -1)
	if len(forms) != 2 {
		t.Fatalf("expected separate member forms, got %d", len(forms))
	}
	second := roomHiddenFields(forms[1])
	if second.Get("target") != "carol" {
		t.Fatalf("second form targets %q", second.Get("target"))
	}
	r, _ := s.Handle(t.Context(), a, protocol.New(922, 95).SetByte(1, 2).SetText(2, roomInfoURL).SetText(3, second.Encode()))
	if status, _ := r[0].Int(1); status != 200 {
		t.Fatal("second target form failed")
	}
	if muted, _ := s.store.IsRoomMuted(t.Context(), "Lobby", "carol"); !muted {
		t.Fatal("selected second target not muted")
	}
	if muted, _ := s.store.IsRoomMuted(t.Context(), "Lobby", "bob"); muted {
		t.Fatal("second form mutated first target")
	}
	if err := s.store.SetRoomModerator(t.Context(), "Lobby", "carol", true); err != nil {
		t.Fatal(err)
	}
	if w := fetchReport(t, s, a, "POST", roomInfoURL, second.Encode()); w.Code != 409 {
		t.Fatal("old form bypassed newly promoted target's protection")
	}
	if muted, _ := s.store.IsRoomMuted(t.Context(), "Lobby", "carol"); muted {
		t.Fatal("promoted moderator was muted")
	}
}

func TestRoomInfoModeratorControlsUseRoomMuteService(t *testing.T) {
	s := newTestServer(t)
	a, b := login(t, s, "alice"), login(t, s, "bob")
	joinRoom(t, s, a, "Lobby")
	joinRoom(t, s, b, "Lobby")
	w := fetchReport(t, s, a, "GET", roomInfoURL, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `value="Mute bob"`) {
		t.Fatalf("missing moderator controls: %s", w.Body.String())
	}
	form := roomHiddenFields(w.Body.String())
	if len(form) != 5 || form.Get("target") != "bob" || form.Get("operation") != "mute" {
		t.Fatal("unexpected moderator form")
	}
	if muted, _ := s.store.IsRoomMuted(t.Context(), "Lobby", "bob"); muted {
		t.Fatal("GET mutated state")
	}
	requestPacket(t, s, a, nil)
	requestPacket(t, s, b, nil)
	// Exercise the original 922 POST path, not just ordinary HTTP forms.
	r, _ := s.Handle(t.Context(), a, protocol.New(922, 80).SetByte(1, 2).SetText(2, roomInfoURL).SetText(3, form.Encode()))
	status, _ := r[0].Int(1)
	body, _ := r[0].Text(2)
	if status != 200 || !strings.Contains(body, `value="Unmute bob"`) || !strings.Contains(body, "bob has been muted") {
		t.Fatalf("mute result: %d %s", status, body)
	}
	if !containsText(requestPacket(t, s, b, nil), "bob was muted by alice.") {
		t.Fatal("member not notified")
	}
	// A repeated form submission is idempotent, including its notification.
	if w := fetchReport(t, s, a, "POST", roomInfoURL, form.Encode()); w.Code != 200 {
		t.Fatal("duplicate mute failed")
	}
	if containsText(requestPacket(t, s, b, nil), "bob was muted by alice.") {
		t.Fatal("duplicate mute repeated notification")
	}
	r, _ = s.Handle(t.Context(), b, privateMessage("Lobby", "muted text").SetByte(3, 3))
	if r[0].Opcode != 0 {
		t.Fatal("page mute did not enforce room send rejection")
	}
	r, _ = s.Handle(t.Context(), b, protocol.New(707, 81).SetText(1, "Lobby"))
	if muted, _ := r[0].Text(4); muted != "bob;" {
		t.Fatal("page mute not represented in native participant list")
	}
	r, _ = s.Handle(t.Context(), b, privateMessage("alice", "private text while muted"))
	if r[0].Opcode != 1 {
		t.Fatal("room mute blocked private messages")
	}
	form = roomHiddenFields(body)
	w = fetchReport(t, s, a, "POST", roomInfoURL, form.Encode())
	if w.Code != 200 || !strings.Contains(w.Body.String(), `value="Mute bob"`) {
		t.Fatal("page unmute failed")
	}
	r, _ = s.Handle(t.Context(), b, privateMessage("Lobby", "after page unmute").SetByte(3, 3))
	if r[0].Opcode != 1 {
		t.Fatal("unmute did not restore room sending")
	}
}

func TestRoomInfoModerationRevalidatesAuthorityAndFormContext(t *testing.T) {
	s := newTestServer(t)
	a, b := login(t, s, "alice"), login(t, s, "bob")
	joinRoom(t, s, a, "Lobby")
	joinRoom(t, s, b, "Lobby")
	form := roomHiddenFields(fetchReport(t, s, a, "GET", roomInfoURL, "").Body.String())
	for _, sid := range []string{"", b} {
		w := fetchReport(t, s, sid, "POST", roomInfoURL, form.Encode())
		if w.Code != 401 && w.Code != 403 {
			t.Fatal("unauthorized moderation accepted")
		}
	}
	if strings.Contains(fetchReport(t, s, b, "GET", roomInfoURL, "").Body.String(), "name=\"csrf\"") {
		t.Fatal("ordinary participant sees moderator form")
	}
	for _, field := range []string{"csrf", "binding", "room"} {
		bad, _ := url.ParseQuery(form.Encode())
		bad.Set(field, "forged")
		if w := fetchReport(t, s, a, "POST", roomInfoURL, bad.Encode()); w.Code != 403 {
			t.Fatalf("forged %s accepted", field)
		}
	}
	for _, target := range []string{"alice", "absent", "carol"} {
		bad, _ := url.ParseQuery(form.Encode())
		bad.Set("target", target)
		if w := fetchReport(t, s, a, "POST", roomInfoURL, bad.Encode()); w.Code != 409 {
			t.Fatalf("invalid target %q accepted", target)
		}
	}
	if w := fetchReport(t, s, a, "POST", roomInfoURL, form.Encode()+"&target=carol"); w.Code != 400 {
		t.Fatal("duplicate target accepted")
	}
	// The form is not authority: revoke the moderator role after rendering it.
	if err := s.store.SetRoomModerator(t.Context(), "Lobby", "alice", false); err != nil {
		t.Fatal(err)
	}
	if w := fetchReport(t, s, a, "POST", roomInfoURL, form.Encode()); w.Code != 403 {
		t.Fatal("revoked role reused old form")
	}
	if err := s.store.SetRoomModerator(t.Context(), "Lobby", "alice", true); err != nil {
		t.Fatal(err)
	}
	s.Handle(t.Context(), a, protocol.New(704, 90).SetText(1, "Lobby"))
	if w := fetchReport(t, s, a, "POST", roomInfoURL, form.Encode()); w.Code != 403 {
		t.Fatal("departed moderator reused old form")
	}
	if muted, _ := s.store.IsRoomMuted(t.Context(), "Lobby", "bob"); muted {
		t.Fatal("rejected forms mutated mute state")
	}
}
