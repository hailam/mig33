package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// This exercises the actual generated adapter next to the compatibility
// fallback. Application authorization and message validation are separate.
type contractProbe struct {
	ServerInterface
	t *testing.T
}

func (p contractProbe) ReadHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (p contractProbe) GetChatHistory(w http.ResponseWriter, _ *http.Request, params GetChatHistoryParams) {
	if params.Kind != Private || params.Target != "bob" || params.AfterId == nil || *params.AfterId != 2 || params.Limit == nil || *params.Limit != 10 {
		p.t.Errorf("history parameter binding: %+v", params)
	}
	w.WriteHeader(http.StatusAccepted)
}

func (p contractProbe) SendMessage(w http.ResponseWriter, r *http.Request, params SendMessageParams) {
	if params.XCSRFToken != strings.Repeat("a", 48) || params.IdempotencyKey != strings.Repeat("b", 48) {
		p.t.Errorf("message header binding: %+v", params)
	}
	data, err := io.ReadAll(r.Body)
	if err != nil || string(data) != `{"kind":"private","target":"bob","body":"hello"}` {
		p.t.Errorf("generated adapter changed request body: %q, %v", data, err)
	}
	w.WriteHeader(http.StatusCreated)
}

func TestGeneratedContractRouting(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(299) })
	handler := HandlerFromMux(contractProbe{t: t}, mux)
	for _, tc := range []struct {
		name, method, target string
		headers              bool
		status               int
	}{
		{"health", "GET", "/healthz", false, 200},
		{"history", "GET", "/api/chat/history?kind=private&target=bob&after_id=2&limit=10", false, 202},
		{"missing required target", "GET", "/api/chat/history?kind=private", false, 400},
		{"malformed integer", "GET", "/api/chat/history?kind=private&target=bob&limit=oops", false, 400},
		{"message", "POST", "/api/chat/messages", true, 201},
		{"missing required header", "POST", "/api/chat/messages", false, 400},
		{"legacy page remains fallback", "GET", "/sites/index.php?c=profile&a=home&v=midlet", false, 299},
		{"native gateway remains fallback", "POST", "/" + strings.Repeat("a", 48), false, 299},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.target, strings.NewReader(`{"kind":"private","target":"bob","body":"hello"}`))
			if tc.headers {
				r.Header.Set("X-CSRF-Token", strings.Repeat("a", 48))
				r.Header.Set("Idempotency-Key", strings.Repeat("b", 48))
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}
