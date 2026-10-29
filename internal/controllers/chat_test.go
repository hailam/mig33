package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/adapters/httpapi"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/session"
)

type chatServiceStub struct {
	Chat
	Sessions
	sid, csrf, key string
	history        chat.HistoryQuery
	send           chat.MessageInput
	sendCalls      int
	sessionErr     error
}

func (s *chatServiceStub) Session(context.Context, string) (session.View, error) {
	return session.View{Username: "alice"}, s.sessionErr
}

func (s *chatServiceStub) ChatHistory(_ context.Context, sid string, query chat.HistoryQuery) (chat.HistoryPage, error) {
	s.sid, s.history = sid, query
	return chat.HistoryPage{MessagePage: chat.MessagePage{NextAfterID: query.AfterID}}, nil
}

func (s *chatServiceStub) SendChatMessage(_ context.Context, sid, csrf, key string, input chat.MessageInput) (chat.MessageResult, error) {
	s.sid, s.csrf, s.key, s.send = sid, csrf, key, input
	s.sendCalls++
	return chat.MessageResult{Message: chat.Message{Kind: 1, Body: input.Body, Color: input.Color}}, nil
}

// Generated parameters are the controller contract. A router or middleware may
// normalize them independently of the raw request's query and header values.
func TestChatControllersConsumeGeneratedParameters(t *testing.T) {
	service := &chatServiceStub{}
	controller := New(Dependencies{Chat: service, Sessions: service})
	after, limit := int64(7), int64(10)
	r := httptest.NewRequest(http.MethodGet, "/api/chat/history?kind=room&target=Ignored", nil)
	r.Header.Set("sid", "session")
	w := httptest.NewRecorder()
	controller.GetChatHistory(w, r, httpapi.GetChatHistoryParams{Kind: httpapi.Private, Target: "bob", AfterId: &after, Limit: &limit})
	if w.Code != http.StatusOK || service.sid != "session" || service.history.Kind != "private" || service.history.Target != "bob" || service.history.AfterID != 7 || service.history.Limit != 10 {
		t.Fatalf("history mapping: status=%d sid=%q query=%+v", w.Code, service.sid, service.history)
	}

	r = httptest.NewRequest(http.MethodPost, "/api/chat/messages", strings.NewReader(`{"kind":"private","target":"bob","body":"hello","color":0}`))
	r.Header.Set("sid", "session")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", "raw-value")
	w = httptest.NewRecorder()
	controller.SendMessage(w, r, httpapi.SendMessageParams{XCSRFToken: "bound-csrf", IdempotencyKey: "bound-key"})
	if w.Code != http.StatusCreated || service.csrf != "bound-csrf" || service.key != "bound-key" || service.send.Color == nil || *service.send.Color != 0 {
		t.Fatalf("send mapping: status=%d csrf=%q key=%q input=%+v", w.Code, service.csrf, service.key, service.send)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	var message map[string]json.RawMessage
	if err := json.Unmarshal(result["message"], &message); err != nil {
		t.Fatal(err)
	}
	if string(message["color"]) != "0" {
		t.Fatalf("explicit black color omitted: %s", w.Body)
	}
	for _, absent := range []string{"action", "markup", "photo_id", "RequestKey", "RequestHash"} {
		if _, ok := message[absent]; ok {
			t.Fatalf("unexpected %q in %s", absent, w.Body)
		}
	}
}

func TestChatRequestDecodingPrecedesServiceAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name, media, body string
		status            int
		message           string
	}{
		{"content type", "text/plain", `{}`, 415, "JSON content type required"},
		{"unknown field", "application/json", `{"unknown":true}`, 400, "Invalid JSON"},
		{"color overflow", "application/json", `{"color":2147483648}`, 400, "Invalid JSON"},
		{"earlier duplicate color overflow", "application/json", `{"color":2147483648,"color":0}`, 400, "Invalid JSON"},
		{"earlier duplicate color underflow", "application/json", `{"color":-2147483649,"color":0}`, 400, "Invalid JSON"},
		{"color overflow precedes trailing value", "application/json", `{"color":2147483648} {}`, 400, "Invalid JSON"},
		{"multiple values", "application/json", `{} {}`, 400, "One JSON object required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &chatServiceStub{}
			r := httptest.NewRequest(http.MethodPost, "/api/chat/messages", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.media)
			w := httptest.NewRecorder()
			New(Dependencies{Chat: service, Sessions: service}).ServeChatAPI(w, r)
			if w.Code != tc.status || strings.TrimSpace(w.Body.String()) != tc.message || service.sendCalls != 0 {
				t.Fatalf("status=%d body=%q service calls=%d", w.Code, w.Body, service.sendCalls)
			}
		})
	}
}

func TestChatBindingFallbackPreservesAuthenticationPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name       string
		sessionErr error
		status     int
		body       string
	}{
		{"unauthenticated", faults.NewError(faults.Unauthenticated, "Login required"), 401, "Login required"},
		{"authenticated", nil, 400, "Invalid after_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/chat/history?kind=private&target=bob&after_id=invalid", nil)
			New(Dependencies{Sessions: &chatServiceStub{sessionErr: tc.sessionErr}}).ServeChatAPI(w, r)
			if w.Code != tc.status || strings.TrimSpace(w.Body.String()) != tc.body {
				t.Fatalf("status=%d body=%q", w.Code, w.Body)
			}
		})
	}
}

// The existing decoder ignores null for scalar fields and clears pointers.
// Preserve those rules even when fields are repeated in one JSON object.
func TestMessageDecodingRetainsLegacyDuplicateNullSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, fields string
		want         chat.MessageInput
	}{
		{"body", `"body":"hello","body":null`, chat.MessageInput{Body: "hello"}},
		{"markup", `"markup":"<b>hello</b>","markup":null`, chat.MessageInput{Markup: "<b>hello</b>"}},
		{"photo", `"photo_id":42,"photo_id":null`, chat.MessageInput{PhotoID: 42}},
		{"color", `"color":0,"color":null,"body":"hello"`, chat.MessageInput{Body: "hello"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &chatServiceStub{}
			r := httptest.NewRequest(http.MethodPost, "/api/chat/messages", strings.NewReader(`{"kind":"private","target":"bob",`+tc.fields+`}`))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			New(Dependencies{Chat: service, Sessions: service}).ServeChatAPI(w, r)
			tc.want.Kind, tc.want.Target = "private", "bob"
			if w.Code != http.StatusCreated || service.sendCalls != 1 || !reflect.DeepEqual(service.send, tc.want) {
				t.Fatalf("status=%d calls=%d input=%+v, want %+v; body=%q", w.Code, service.sendCalls, service.send, tc.want, w.Body)
			}
		})
	}
}
