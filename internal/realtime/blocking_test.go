package realtime

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/protocol"
)

const blockListURL = "http://127.0.0.1:8080/sites/index.php?c=settings&a=block_list&v=midlet"

func block(t *testing.T, s *Application, id, target string) {
	t.Helper()
	r, _ := s.Handle(t.Context(), id, protocol.New(417, 40).SetText(1, target).SetByte(2, 2))
	if len(r) != 1 || r[0].Opcode != 1 || r[0].Transaction != 40 {
		t.Fatalf("block: %v", r)
	}
}

func unblockThroughPage(t *testing.T, s *Application, id, target string) {
	t.Helper()
	form := url.Values{"action": {"unblock"}, "username": {target}, "csrf": {s.sessions[id].csrf}}
	p := protocol.New(922, 41).SetByte(1, 2).SetText(2, blockListURL).SetText(3, form.Encode()).SetInt(4, 1)
	r, _ := s.Handle(t.Context(), id, p)
	if r[0].Opcode != 923 {
		t.Fatal("unblock page response")
	}
	if status, _ := r[0].Int(1); status != 200 {
		t.Fatalf("unblock status %d", status)
	}
}

func TestPersonalBlockIsDirectedAcrossConversationTypes(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	carol := login(t, s, "carol")
	created, _ := s.Handle(t.Context(), bob, protocol.New(751, 1).SetText(1, "alice").SetText(2, "carol").SetByte(3, 1))
	group, _ := created[0].Text(1)
	joinRoom(t, s, alice, "Lobby")
	joinRoom(t, s, bob, "Lobby")
	joinRoom(t, s, carol, "Lobby")
	block(t, s, alice, "BOB")
	for _, tc := range []struct {
		target, body string
		kind         byte
	}{{"alice", "blocked direct", 1}, {group, "blocked group", 2}, {"Lobby", "blocked room", 3}} {
		r, _ := s.Handle(t.Context(), bob, privateMessage(tc.target, tc.body).SetByte(3, tc.kind))
		if r[0].Opcode != 1 {
			t.Fatal("block should not disclose itself via sender error")
		}
	}
	if packets := requestPacket(t, s, alice, nil); containsText(packets, "blocked direct") || containsText(packets, "blocked group") || containsText(packets, "blocked room") {
		t.Fatal("blocked incoming text delivered")
	}
	packets := requestPacket(t, s, carol, nil)
	if !containsText(packets, "blocked group") || !containsText(packets, "blocked room") {
		t.Fatal("personal block affected other users")
	}
	for _, tc := range []struct {
		target string
		kind   byte
	}{{"bob", 1}, {group, 2}, {"Lobby", 3}} {
		history, err := s.store.History(t.Context(), "alice", tc.kind, tc.target, 0, 50)
		if err != nil || len(history) != 0 {
			t.Fatalf("blocked new text entered recipient history: %v %v", history, err)
		}
	}
	s.Handle(t.Context(), alice, privateMessage("bob", "my outgoing message"))
	if !containsText(requestPacket(t, s, bob, nil), "my outgoing message") {
		t.Fatal("directed block incorrectly blocked own outgoing text")
	}
	if _, err := s.store.Group(t.Context(), group, "alice"); err != nil || !s.sessions[alice].rooms["Lobby"] {
		t.Fatal("block changed memberships")
	}
	muted, err := s.store.IsRoomMuted(t.Context(), "Lobby", "bob")
	if err != nil || muted {
		t.Fatal("personal block became moderator mute")
	}
}

func TestBlockCancelsQueuedTextAcrossSessionsAndUnblockDoesNotReplay(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	aliceOther := login(t, s, "alice")
	bob := login(t, s, "bob")
	s.Handle(t.Context(), bob, privateMessage("alice", "queued before block"))
	block(t, s, alice, "bob")
	unblockThroughPage(t, s, alice, "bob")
	for _, id := range []string{alice, aliceOther} {
		if containsText(requestPacket(t, s, id, nil), "queued before block") {
			t.Fatal("canceled text replayed after unblock")
		}
	}
	s.Handle(t.Context(), bob, privateMessage("alice", "new after unblock"))
	if !containsText(requestPacket(t, s, alice, nil), "new after unblock") {
		t.Fatal("unblock did not restore new delivery")
	}
}

func TestBlockListPageAuthorizationAndOriginalPath(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	block(t, s, alice, "bob")
	fetch := func(method, id, query, body, contentType string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, blockListURL+query, strings.NewReader(body))
		r.Header.Set("sid", id)
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	if w := fetch("GET", "", "", "", ""); w.Code != 401 {
		t.Fatal("unauthenticated block list")
	}
	w := fetch("GET", alice, "", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `name="csrf"`) || !strings.Contains(w.Body.String(), "bob") || strings.Contains(w.Body.String(), "&amp;a=") {
		t.Fatalf("original-compatible form missing: %s", w.Body.String())
	}
	if w := fetch("GET", alice, "&action=unblock&username=bob", "", ""); w.Code != 200 {
		t.Fatal("GET block list")
	}
	if blocked, _ := s.store.IsBlocked(t.Context(), "alice", "bob"); !blocked {
		t.Fatal("GET mutated block state")
	}
	for _, form := range []url.Values{
		{"action": {"unblock"}, "username": {"bob"}},
		{"action": {"unblock"}, "username": {"bob"}, "csrf": {s.sessions[bob].csrf}},
		{"action": {"unblock"}, "username": {"bob"}, "csrf": {s.sessions[alice].csrf, s.sessions[alice].csrf}},
	} {
		if w := fetch("POST", alice, "", form.Encode(), "application/x-www-form-urlencoded"); w.Code != 403 {
			t.Fatal("invalid/cross-session token accepted")
		}
	}
	good := url.Values{"action": {"unblock"}, "username": {"bob"}, "csrf": {s.sessions[alice].csrf}}
	if w := fetch("POST", alice, "", good.Encode(), "application/json"); w.Code != 415 {
		t.Fatal("wrong content type accepted")
	}
	if w := fetch("POST", alice, "", good.Encode(), "application/x-www-form-urlencoded"); w.Code != 200 {
		t.Fatalf("direct unblock: %d", w.Code)
	}
	if blocked, _ := s.store.IsBlocked(t.Context(), "alice", "bob"); blocked {
		t.Fatal("direct HTTP unblock failed")
	}
}

func TestBlockListScopeAndInputValidation(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	for _, p := range []*protocol.Packet{
		protocol.New(417, 1).SetText(1, "alice").SetByte(2, 2),
		protocol.New(417, 2).SetText(1, "unknown").SetByte(2, 2),
		protocol.New(417, 3).SetText(1, "bob").SetByte(2, 0),
		protocol.New(417, 4).SetText(1, "bob"),
	} {
		r, _ := s.Handle(t.Context(), alice, p)
		if r[0].Opcode != 0 {
			t.Fatal("invalid block accepted")
		}
	}
	block(t, s, alice, "bob")
	r, _ := s.Handle(t.Context(), bob, protocol.New(922, 5).SetByte(1, 1).SetText(2, blockListURL).SetInt(4, 1))
	if body, _ := r[0].Text(2); strings.Contains(body, `name="username" value="bob"`) {
		t.Fatal("another account's block list leaked")
	}
	block(t, s, alice, "bob") // idempotent
	blocked, err := s.store.BlockedUsers(t.Context(), "alice")
	if err != nil || len(blocked) != 1 {
		t.Fatal("duplicate block")
	}
}

func TestBlockPersistsThroughRestartAndCancelsOfflineQueue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat.sqlite")
	s, err := newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	bob := login(t, s, "bob")
	s.Handle(t.Context(), bob, privateMessage("alice", "offline before block"))
	alice := login(t, s, "alice")
	block(t, s, alice, "bob")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	alice = login(t, s, "alice")
	bob = login(t, s, "bob")
	s.Handle(t.Context(), bob, privateMessage("alice", "blocked after restart"))
	packets := requestPacket(t, s, alice, nil)
	if containsText(packets, "offline before block") || containsText(packets, "blocked after restart") {
		t.Fatal("block did not survive restart")
	}
	unblockThroughPage(t, s, alice, "bob")
	if containsText(requestPacket(t, s, alice, nil), "offline before block") {
		t.Fatal("unblock replayed old offline message")
	}
}

func TestBlockedUserCannotBypassWithGroupInvitations(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	login(t, s, "carol")
	block(t, s, alice, "bob")
	r, _ := s.Handle(t.Context(), bob, protocol.New(751, 1).SetText(1, "alice").SetText(2, "carol").SetByte(3, 1))
	if r[0].Opcode != 0 {
		t.Fatal("forced new group bypassed block")
	}
	created, _ := s.Handle(t.Context(), bob, protocol.New(751, 2).SetText(1, "dave").SetText(2, "carol").SetByte(3, 1))
	group, _ := created[0].Text(1)
	r, _ = s.Handle(t.Context(), bob, protocol.New(752, 3).SetText(1, group).SetText(2, "alice").SetByte(3, 1))
	if r[0].Opcode != 0 {
		t.Fatal("invitation bypassed block")
	}
}

func TestPersonalBlockDoesNotHideModeratorSystemMessages(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	joinRoom(t, s, alice, "Lobby")
	joinRoom(t, s, bob, "Lobby")
	block(t, s, bob, "alice")
	s.Handle(t.Context(), alice, protocol.New(709, 1).SetText(1, "Lobby").SetText(2, "bob"))
	if !containsText(requestPacket(t, s, bob, nil), "bob was muted by alice.") {
		t.Fatal("personal block hid server moderation notice")
	}
	s.Handle(t.Context(), alice, protocol.New(706, 2).SetText(1, "Lobby").SetText(2, "bob"))
	notice := false
	for _, p := range requestPacket(t, s, bob, nil) {
		if p.Opcode == 5 {
			notice = true
		}
	}
	if !notice || s.sessions[bob].rooms["Lobby"] {
		t.Fatal("personal block bypassed moderator kick")
	}
}

type pausedBlockResponse struct {
	header           http.Header
	started, release chan struct{}
}

func (w *pausedBlockResponse) Header() http.Header { return w.header }

func (w *pausedBlockResponse) WriteHeader(int) {}

func (w *pausedBlockResponse) Write(p []byte) (int, error) {
	close(w.started)
	<-w.release
	return len(p), nil
}

func TestBlockWaitsForAccountWritesWithoutStallingOtherUsers(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	carol := login(t, s, "carol")
	s.Handle(t.Context(), bob, privateMessage("alice", "already writing"))
	w := &pausedBlockResponse{header: make(http.Header), started: make(chan struct{}), release: make(chan struct{})}
	written := make(chan struct{})
	go func() { s.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/"+alice, nil)); close(written) }()
	<-w.started
	blocked := make(chan []*protocol.Packet, 1)
	go func() {
		r, _ := s.Handle(t.Context(), alice, protocol.New(417, 40).SetText(1, "bob").SetByte(2, 2))
		blocked <- r
	}()
	// A write belonging to Alice must not hold the whole server's state lock.
	other := make(chan struct{})
	go func() {
		s.Handle(t.Context(), carol, protocol.New(417, 41).SetText(1, "bob").SetByte(2, 2))
		close(other)
	}()
	select {
	case <-other:
	case <-time.After(time.Second):
		close(w.release)
		t.Fatal("unrelated account stalled")
	}
	select {
	case <-blocked:
		close(w.release)
		t.Fatal("block completed before earlier write")
	case <-time.After(20 * time.Millisecond):
	}
	close(w.release)
	select {
	case <-written:
	case <-time.After(time.Second):
		t.Fatal("write hung")
	}
	select {
	case r := <-blocked:
		if r[0].Opcode != 1 {
			t.Fatal("block failed")
		}
	case <-time.After(time.Second):
		t.Fatal("block hung")
	}
	s.Handle(t.Context(), bob, privateMessage("alice", "must stay blocked"))
	if containsText(requestPacket(t, s, alice, nil), "must stay blocked") {
		t.Fatal("later write bypassed block")
	}
}
