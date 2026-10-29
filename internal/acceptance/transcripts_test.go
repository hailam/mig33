package acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	chatstore "github.com/hailam/mig33/internal/adapters/sqlite/repositories"
)

func TestOwnPrivateMirrorRoutesToPeerAndRetries(t *testing.T) {
	s := newTestServer(t)
	origin := login(t, s, "alice")
	other := login(t, s, "alice")
	bob := login(t, s, "bob")
	requestPacket(t, s, origin, nil)
	requestPacket(t, s, other, nil)
	r, _ := s.Handle(t.Context(), origin, privateMessage("bob", "my text").SetInt(13, 0x112233))
	if r[0].Opcode != 1 {
		t.Fatal("send failed")
	}
	if r := requestPacket(t, s, origin, nil); len(r) != 0 {
		t.Fatalf("origin local echo duplicated: %v", r)
	}
	broken := &brokenResponse{header: make(map[string][]string), failWrite: true}
	s.ServeHTTP(broken, httptest.NewRequest("GET", "http://localhost/"+other, nil))
	r = requestPacket(t, s, other, nil)
	if len(r) != 1 || r[0].Opcode != 500 {
		t.Fatalf("mirror retry: %v", r)
	}
	peer, _ := r[0].Text(2)
	body, _ := r[0].Text(8)
	kind, _ := r[0].Short(6)
	if peer != "bob" || body != "alice: my text" || kind != 6 {
		t.Fatalf("incorrect native own routing: %+v", r[0])
	}
	if r := requestPacket(t, s, other, nil); len(r) != 0 {
		t.Fatal("mirror repeated after successful delivery")
	}
	toBob := requestPacket(t, s, bob, nil)
	if len(toBob) != 1 {
		t.Fatalf("recipient delivery changed: %v", toBob)
	}
	if body, _ := toBob[0].Text(8); body != "my text" {
		t.Fatal("recipient text altered")
	}
	history, err := s.store.History(t.Context(), "alice", 1, "bob", 0, 10)
	if err != nil || len(history) != 1 || history[0].Action || history[0].Color == nil || *history[0].Color != 0x112233 {
		t.Fatalf("history altered by native mirror: %+v %v", history, err)
	}
	newSession := login(t, s, "alice")
	if r := requestPacket(t, s, newSession, nil); len(r) != 0 {
		t.Fatal("fresh login unexpectedly replays old own transcript")
	}
}

// TestOwnMirrorBackpressureDoesNotLoseMessages remains in realtime because
// it also verifies that private origin skip markers are reclaimed.

func transcriptHTTP(t *testing.T, s *fixture, id, method, path, body, key string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
	r.Header.Set("sid", id)
	if x := s.session(id); x != nil {
		r.Header.Set("X-CSRF-Token", x.csrf)
	}
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestTranscriptAPIRetryReadAndAudienceAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat.sqlite")
	s, err := Open(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	carol := login(t, s, "carol")
	key := strings.Repeat("a", 48)
	body := `{"kind":"private","target":"bob","body":"stable & +=% text","color":1193046}`
	response := transcriptHTTP(t, s, alice, "POST", "/api/chat/messages", body, key)
	var sent struct {
		Message  chatstore.Message `json:"message"`
		Replayed bool              `json:"replayed"`
	}
	if response.Code != 201 || json.Unmarshal(response.Body.Bytes(), &sent) != nil || sent.Message.ID <= 0 {
		t.Fatalf("send: %d %s", response.Code, response.Body.String())
	}
	id := sent.Message.ID
	response = transcriptHTTP(t, s, alice, "POST", "/api/chat/messages", body, key)
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &sent) != nil || !sent.Replayed || sent.Message.ID != id {
		t.Fatal("retry not deduplicated")
	}
	response = transcriptHTTP(t, s, alice, "POST", "/api/chat/messages", strings.Replace(body, "stable", "changed", 1), key)
	if response.Code != 409 {
		t.Fatal("changed nonce payload accepted")
	}
	response = transcriptHTTP(t, s, carol, "GET", "/api/chat/sync", "", "")
	var sync struct {
		Messages []chatstore.Message `json:"messages"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &sync) != nil || len(sync.Messages) != 0 {
		t.Fatal("private transcript disclosed")
	}
	read := fmt.Sprintf(`{"kind":"private","target":"alice","through_id":%d}`, id)
	if w := transcriptHTTP(t, s, carol, "POST", "/api/chat/read", read, ""); w.Code != 404 {
		t.Fatalf("outsider read pointer: %d", w.Code)
	}
	if w := transcriptHTTP(t, s, bob, "POST", "/api/chat/read", read, ""); w.Code != 200 {
		t.Fatalf("read pointer: %d %s", w.Code, w.Body.String())
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	alice = login(t, s, "alice")
	bob = login(t, s, "bob")
	response = transcriptHTTP(t, s, alice, "POST", "/api/chat/messages", body, key)
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &sent) != nil || !sent.Replayed || sent.Message.ID != id {
		t.Fatalf("restart retry: %d %s", response.Code, response.Body.String())
	}
	response = transcriptHTTP(t, s, bob, "GET", "/api/chat/history?kind=private&target=alice", "", "")
	if response.Code != 200 || !bytes.Contains(response.Body.Bytes(), []byte(fmt.Sprintf(`"read_through_id":%d`, id))) {
		t.Fatal("read state not persistent")
	}
	messages, err := s.store.History(t.Context(), "alice", 1, "bob", 0, 10)
	if err != nil || len(messages) != 1 {
		t.Fatalf("duplicate persisted: %v %v", messages, err)
	}
}

func TestTranscriptAPIPaginationCSRFAndReadMonotonicity(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	for _, body := range []string{"one", "two", "three"} {
		s.Handle(t.Context(), alice, privateMessage("bob", body))
	}
	response := transcriptHTTP(t, s, bob, "GET", "/api/chat/sync?limit=2", "", "")
	var page struct {
		Messages []chatstore.Message `json:"messages"`
		Next     int64               `json:"next_after_id"`
		More     bool                `json:"has_more"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil || len(page.Messages) != 2 || !page.More {
		t.Fatal("first sync page")
	}
	first, second := page.Messages[0].ID, page.Messages[1].ID
	response = transcriptHTTP(t, s, bob, "GET", fmt.Sprintf("/api/chat/sync?limit=2&after_id=%d", page.Next), "", "")
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil || len(page.Messages) != 1 || page.More || page.Messages[0].Body != "three" {
		t.Fatal("second sync page")
	}
	for _, id := range []int64{second, first} {
		if w := transcriptHTTP(t, s, bob, "POST", "/api/chat/read", fmt.Sprintf(`{"kind":"private","target":"alice","through_id":%d}`, id), ""); w.Code != 200 {
			t.Fatal("read failed")
		}
	}
	if id, err := s.store.ConversationRead(t.Context(), "bob", 1, "alice"); err != nil || id != second {
		t.Fatal("read checkpoint moved back")
	}
	r := httptest.NewRequest("POST", "http://localhost/api/chat/messages", strings.NewReader(`{"kind":"private","target":"bob","body":"no csrf"}`))
	r.Header.Set("sid", alice)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", strings.Repeat("b", 48))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("missing CSRF accepted")
	}
	if w := transcriptHTTP(t, s, "", "GET", "/api/chat/session", "", ""); w.Code != 401 {
		t.Fatal("unauthenticated session disclosed")
	}
}
