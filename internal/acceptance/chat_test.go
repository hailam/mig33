package acceptance

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	chatstore "github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/protocol"
)

func privateMessage(target, body string) *protocol.Packet {
	return protocol.New(500, 50).SetByte(1, 1).SetText(2, "forged").SetByte(3, 1).SetText(4, target).SetShort(6, 1).SetText(8, body)
}

func TestOfflinePrivateMessageSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat.sqlite")
	s, err := Open(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	alice := login(t, s, "alice")
	r, _ := s.Handle(t.Context(), alice, privateMessage("bob", "Stored ✓ 😀").SetInt(13, 0xff00))
	if r[0].Opcode != 1 {
		t.Fatal("message was not accepted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, _ = s.Handle(t.Context(), alice, protocol.New(400, 1))
	if r[0].Opcode != 0 {
		t.Fatal("restart retained old authentication")
	}
	bob := login(t, s, "bob")
	pending, err := s.store.Pending(t.Context(), "bob", 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("queueing incorrectly marked delivered: %v %v", pending, err)
	}
	r = requestPacket(t, s, bob, nil)
	if len(r) != 1 || r[0].Opcode != 500 {
		t.Fatalf("offline replay: %v", r)
	}
	if text, _ := r[0].Text(8); text != "Stored ✓ 😀" {
		t.Fatal("stored text changed")
	}
	if color, _ := r[0].Int(13); color != 0xff00 {
		t.Fatal("color lost")
	}
	if sender, _ := r[0].Text(2); sender != "alice" {
		t.Fatal("trusted sender lost")
	}
	pending, err = s.store.Pending(t.Context(), "bob", 10)
	if err != nil || len(pending) != 0 {
		t.Fatal("successful write did not mark delivery")
	}
	history, err := s.store.History(t.Context(), "bob", 1, "alice", 0, 10)
	if err != nil || len(history) != 1 {
		t.Fatalf("history missing after delivery: %v %v", history, err)
	}
}

type brokenResponse struct {
	header    http.Header
	failWrite bool
	body      bytes.Buffer
}

func (w *brokenResponse) Header() http.Header { return w.header }

func (w *brokenResponse) WriteHeader(int) {}

func (w *brokenResponse) Write(b []byte) (int, error) {
	if w.failWrite {
		return 0, io.ErrClosedPipe
	}
	return w.body.Write(b)
}

func (w *brokenResponse) FlushError() error { return io.ErrClosedPipe }

func TestFailedHTTPDeliveryRemainsPending(t *testing.T) {
	for _, failWrite := range []bool{true, false} {
		t.Run(map[bool]string{true: "write", false: "flush"}[failWrite], func(t *testing.T) {
			s := newTestServer(t)
			alice := login(t, s, "alice")
			bob := login(t, s, "bob")
			s.Handle(t.Context(), alice, privateMessage("bob", "retry after broken transport"))
			w := &brokenResponse{header: make(http.Header), failWrite: failWrite}
			s.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/"+bob, nil))
			pending, err := s.store.Pending(t.Context(), "bob", 10)
			if err != nil || len(pending) != 1 {
				t.Fatal("failed delivery was discarded")
			}
			r := requestPacket(t, s, bob, nil)
			if len(r) != 1 || r[0].Opcode != 500 {
				t.Fatal("failed delivery was not retried")
			}
		})
	}
}

func TestHistoryExtensionAuthorizationAndPagination(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	carol := login(t, s, "carol")
	s.Handle(t.Context(), alice, privateMessage("bob", "one"))
	s.Handle(t.Context(), bob, privateMessage("alice", "two"))
	fetch := func(id, path string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("GET", "http://localhost"+path, nil)
		r.Header.Set("sid", id)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	if w := fetch("", "/api/chat/history?kind=private&target=bob"); w.Code != 401 {
		t.Fatal("unauthenticated history")
	}
	w := fetch(alice, "/api/chat/history?kind=private&target=bob&limit=1")
	var first struct {
		Messages []chatstore.Message `json:"messages"`
		Next     int64               `json:"next_after_id"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &first) != nil || len(first.Messages) != 1 || first.Messages[0].Body != "one" || first.Next == 0 {
		t.Fatalf("history page: %s", w.Body.String())
	}
	w = fetch(carol, "/api/chat/history?kind=private&target=bob")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &first) != nil || len(first.Messages) != 0 {
		t.Fatal("third party could read private history")
	}
	for _, query := range []string{"kind=unknown&target=bob", "kind=private&target=bob&limit=1000", "kind=private&target=bob&after_id=-1", "kind=private"} {
		if w := fetch(alice, "/api/chat/history?"+query); w.Code != 400 {
			t.Fatalf("bad history query accepted: %s", query)
		}
	}
	if w := fetch(alice, "/api/chat/groups"); w.Code != 200 {
		t.Fatal("groups extension")
	}
}

// TestQueuedGroupMessageCanceledAcrossLeaveAndRejoin remains in realtime
// because it also verifies the private inflight reservation before leaving.

func TestStorageFailureDoesNotAcknowledgeMessage(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	if err := s.store.Close(); err != nil {
		t.Fatal(err)
	}
	r, _ := s.Handle(t.Context(), alice, privateMessage("bob", "must not disappear"))
	if r[0].Opcode != 0 {
		t.Fatal("storage failure falsely acknowledged")
	}
}

func TestQueuedMessageReplaysAfterSessionDisconnect(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	s.Handle(t.Context(), alice, privateMessage("bob", "disconnected before write"))
	s.DropSession(t.Context(), bob)
	bob = login(t, s, "bob")
	r := requestPacket(t, s, bob, nil)
	if len(r) != 1 || r[0].Opcode != 500 {
		t.Fatal("disconnected queue lost")
	}
	_, err := s.store.Group(t.Context(), "missing", "alice")
	if !errors.Is(err, chatstore.ErrNotFound) {
		t.Fatal("missing group")
	}
}
