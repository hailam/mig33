package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hailam/mig33/internal/adapters/httptransport"
)

func TestHealthPreservesMethodRestrictions(t *testing.T) {
	handler := httptransport.New(New(Dependencies{}))
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(method, "/healthz", nil))
			want := http.StatusMethodNotAllowed
			if method == http.MethodGet {
				want = http.StatusOK
			}
			if w.Code != want {
				t.Fatalf("status = %d, want %d; body = %s", w.Code, want, w.Body)
			}
		})
	}
}
