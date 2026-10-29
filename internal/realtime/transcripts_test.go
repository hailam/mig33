package realtime

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOwnMirrorBackpressureDoesNotLoseMessages(t *testing.T) {
	s := newTestServer(t)
	origin := login(t, s, "alice")
	other := login(t, s, "alice")
	for i := 0; i < 150; i++ {
		r, _ := s.Handle(t.Context(), origin, privateMessage("bob", fmt.Sprintf("mirror-%03d", i)))
		if r[0].Opcode != 1 {
			t.Fatal("send failed")
		}
	}
	var bodies []string
	for attempts := 0; attempts < 5; attempts++ {
		for _, p := range requestPacket(t, s, other, nil) {
			if p.Opcode == 500 {
				body, _ := p.Text(8)
				bodies = append(bodies, body)
			}
		}
	}
	if len(bodies) != 150 {
		t.Fatalf("got %d mirrors", len(bodies))
	}
	for i, body := range bodies {
		if body != fmt.Sprintf("alice: mirror-%03d", i) {
			t.Fatalf("order at %d: %s", i, body)
		}
	}
	if len(s.sessions[origin].ownSkipped) != 0 {
		t.Fatal("origin skip markers leaked")
	}
}

func transcriptHTTP(t *testing.T, s *Application, id, method, path, body, key string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
	r.Header.Set("sid", id)
	if x := s.sessions[id]; x != nil {
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
