package realtime

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/protocol"
)

func newTestServer(t *testing.T) *Application {
	t.Helper()
	s, err := newTestApp(t, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func login(t *testing.T, s *Application, user string) string {
	t.Helper()
	r, id := s.Handle(t.Context(), "", protocol.New(200, 10).SetText(5, user))
	if len(r) != 1 || r[0].Opcode != 201 {
		t.Fatalf("challenge: %v", r)
	}
	challenge, _ := r[0].Text(1)
	r, id = s.Handle(t.Context(), id, protocol.New(202, 11).SetInt(1, protocol.JavaHash(challenge+user+"-demo")))
	if len(r) != 1 || r[0].Opcode != 203 {
		t.Fatalf("login: %v", r)
	}
	if v, ok := r[0].Short(31); !ok || v != 0 {
		t.Fatal("required legacy short missing")
	}
	s.Handle(t.Context(), id, protocol.New(400, 12))
	return id
}

func TestAuthenticationAndSessionIsolation(t *testing.T) {
	s := newTestServer(t)
	r, _ := s.Handle(t.Context(), "", protocol.New(400, 1))
	if r[0].Opcode != 0 {
		t.Fatal("unauthenticated contacts")
	}
	r, id := s.Handle(t.Context(), "", protocol.New(200, 2).SetText(5, "alice"))
	challenge, _ := r[0].Text(1)
	r, _ = s.Handle(t.Context(), id, protocol.New(400, 3))
	if r[0].Opcode != 0 {
		t.Fatal("challenge grants access")
	}
	r, next := s.Handle(t.Context(), id, protocol.New(202, 4).SetInt(1, 0))
	if r[0].Opcode != 0 || next != "" {
		t.Fatal("bad password accepted")
	}
	r, _ = s.Handle(t.Context(), id, protocol.New(202, 5).SetInt(1, protocol.JavaHash(challenge+"alice-demo")))
	if r[0].Opcode != 0 {
		t.Fatal("failed challenge reused")
	}
	id = login(t, s, "alice")
	r, _ = s.Handle(t.Context(), id, protocol.New(400, 6))
	if len(r) != 7 || r[0].Opcode != 401 || r[1].Opcode != 402 || r[5].Opcode != 420 || r[6].Opcode != 403 {
		t.Fatalf("roster sequence: %v", r)
	}
	if name, _ := r[1].Text(12); name != "bob" {
		t.Fatal("wrong roster")
	}
	for _, p := range r {
		if p.Transaction != 6 {
			t.Fatal("uncorrelated contact reply")
		}
	}
	r, _ = s.Handle(t.Context(), id, protocol.New(713, 7))
	if len(r) != 3 || r[0].Opcode != 714 || r[1].Opcode != 714 || r[2].Opcode != 715 {
		t.Fatal("categories")
	}
	r, _ = s.Handle(t.Context(), id, protocol.New(999, 8))
	if r[0].Opcode != 0 {
		t.Fatal("unsupported opcode silently accepted")
	}
	s.Handle(t.Context(), id, protocol.New(300, 9))
	r, _ = s.Handle(t.Context(), id, protocol.New(400, 10))
	if r[0].Opcode != 0 {
		t.Fatal("logout retained authorization")
	}
}

func TestChallengeExpiration(t *testing.T) {
	s := newTestServer(t)
	r, id := s.Handle(t.Context(), "", protocol.New(200, 1).SetText(5, "alice"))
	challenge, _ := r[0].Text(1)
	s.sessions[id].created = time.Now().Add(-2 * time.Minute)
	r, _ = s.Handle(t.Context(), id, protocol.New(202, 2).SetInt(1, protocol.JavaHash(challenge+"alice-demo")))
	if r[0].Opcode != 0 {
		t.Fatal("expired challenge accepted")
	}
}

func TestTCPHeartbeatKeepsAuthenticatedSessionAlive(t *testing.T) {
	s := newTestServer(t)
	id := login(t, s, "alice")
	s.sessions[id].touched = time.Now().Add(-sessionTTL + time.Minute)
	before := time.Now()
	r, next := s.Handle(t.Context(), id, protocol.New(2, 20).SetInt(1, 1))
	if r[0].Opcode != 3 || next != id || s.sessions[id].touched.Before(before) {
		t.Fatal("TCP heartbeat failed to refresh session activity")
	}
	r, next = s.Handle(t.Context(), "", protocol.New(2, 21))
	if r[0].Opcode != 3 || next != "" {
		t.Fatal("unauthenticated connection test must remain available")
	}
}

func requestPacket(t *testing.T, h http.Handler, id string, p *protocol.Packet) []*protocol.Packet {
	t.Helper()
	var body bytes.Buffer
	if p != nil {
		if err := protocol.Encode(&body, p); err != nil {
			t.Fatal(err)
		}
	}
	method := "POST"
	if p == nil {
		method = "GET"
	}
	req := httptest.NewRequest(method, "http://127.0.0.1:8080/"+id, &body)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	if recorder.Code != 200 {
		t.Fatalf("HTTP %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Content-Type") != "image/gif" {
		t.Fatal("legacy content type")
	}
	var packets []*protocol.Packet
	for recorder.Body.Len() > 0 {
		p, err := protocol.Decode(recorder.Body)
		if err != nil {
			t.Fatal(err)
		}
		packets = append(packets, p)
	}
	return packets
}

func httpLogin(t *testing.T, s *Application, name string) string {
	t.Helper()
	r := requestPacket(t, s, "", protocol.New(200, 1).SetText(5, name))
	challenge, _ := r[0].Text(1)
	id, _ := r[0].Text(2)
	r = requestPacket(t, s, id, protocol.New(202, 2).SetInt(1, protocol.JavaHash(challenge+name+"-demo")))
	if r[0].Opcode != 203 {
		t.Fatal("HTTP login failed")
	}
	requestPacket(t, s, id, protocol.New(400, 3))
	return id
}

func TestHTTPProfileAndMalformedRequests(t *testing.T) {
	s := newTestServer(t)
	id := httpLogin(t, s, "alice")
	profile := "http://127.0.0.1:8080/sites/index.php?c=profile&a=home&v=midlet&username=bob"
	p := protocol.New(922, 3).SetByte(1, 1).SetText(2, profile).SetInt(4, 1)
	r := requestPacket(t, s, id, p)
	status, _ := r[0].Int(1)
	body, _ := r[0].Text(2)
	if r[0].Opcode != 923 || status != 200 || !strings.Contains(body, "Bob") {
		t.Fatalf("profile: %d %s", status, body)
	}
	p.SetText(2, "http://example.com/sites/index.php?c=profile&a=home&v=midlet")
	r = requestPacket(t, s, id, p)
	status, _ = r[0].Int(1)
	if status != 403 {
		t.Fatal("arbitrary outbound URL accepted")
	}
	for _, auth := range []bool{false, true} {
		req := httptest.NewRequest("GET", profile, nil)
		if auth {
			req.Header.Set("sid", id)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		want := 401
		if auth {
			want = 200
		}
		if w.Code != want {
			t.Fatalf("direct page: %d want%d", w.Code, want)
		}
	}
	req := httptest.NewRequest("POST", "http://127.0.0.1:8080/", bytes.NewReader([]byte{2, 0, 200}))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatal("truncated frame accepted")
	}
	req = httptest.NewRequest("GET", "http://127.0.0.1:8080/"+id, nil)
	req.Header.Set("sid", strings.Repeat("a", 48))
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatal("conflicting session selectors accepted")
	}
}

func TestTCPToHTTPMessageAndLogout(t *testing.T) {
	s := newTestServer(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- s.ServeTCP(ctx, listener) }()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	exchange := func(p *protocol.Packet) *protocol.Packet {
		t.Helper()
		if err := protocol.Encode(conn, p); err != nil {
			t.Fatal(err)
		}
		r, err := protocol.Decode(conn)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	r := exchange(protocol.New(2, 1))
	if r.Opcode != 3 {
		t.Fatal("ping")
	}
	r = exchange(protocol.New(200, 2).SetText(5, "alice"))
	challenge, _ := r.Text(1)
	r = exchange(protocol.New(202, 3).SetInt(1, protocol.JavaHash(challenge+"alice-demo")))
	if r.Opcode != 203 {
		t.Fatal("TCP login")
	}
	if err := protocol.Encode(conn, protocol.New(400, 30)); err != nil {
		t.Fatal(err)
	}
	for {
		r, err = protocol.Decode(conn)
		if err != nil {
			t.Fatal(err)
		}
		if r.Opcode == 403 {
			break
		}
	}
	bob := httpLogin(t, s, "bob")
	// Bob's login pushes presence to Alice before the next correlated reply.
	r, err = protocol.Decode(conn)
	if err != nil || r.Opcode != 404 {
		t.Fatalf("presence push: %v %v", r, err)
	}
	r = exchange(protocol.New(500, 4).SetByte(1, 1).SetText(2, "forged-sender").SetByte(3, 1).SetText(4, "bob").SetShort(6, 1).SetText(8, "Hello ✓ 😀"))
	if r.Opcode != 1 || r.Transaction != 4 {
		t.Fatal("message ack")
	}
	pushes := requestPacket(t, s, bob, nil)
	if len(pushes) != 1 || pushes[0].Opcode != 500 {
		t.Fatalf("message poll: %v", pushes)
	}
	if sender, _ := pushes[0].Text(2); sender != "alice" {
		t.Fatal("sender spoofing")
	}
	if contact, ok := pushes[0].Int(5); !ok || contact != 1 {
		t.Fatal("incoming message must refer to the sender's contact record")
	}
	if text, _ := pushes[0].Text(8); text != "Hello ✓ 😀" {
		t.Fatal("UTF-8 message changed")
	}
	// Reverse transport direction also delivers asynchronously.
	rp := requestPacket(t, s, bob, protocol.New(500, 5).SetByte(1, 1).SetByte(3, 1).SetText(4, "alice").SetShort(6, 1).SetText(8, "Reply"))
	if rp[0].Opcode != 1 {
		t.Fatal("HTTP message ack")
	}
	r, err = protocol.Decode(conn)
	if err != nil || r.Opcode != 500 {
		t.Fatalf("TCP push: %v %v", r, err)
	}
	conn.Close()
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("TCP shutdown hung")
	}
}

func TestDurableQueueBackpressureAndOfflineDelivery(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	message := protocol.New(500, 1).SetByte(1, 1).SetByte(3, 1).SetText(4, "bob").SetShort(6, 1).SetText(8, "test")
	r, _ := s.Handle(t.Context(), alice, message)
	if r[0].Opcode != 1 {
		t.Fatal("offline message was not durably accepted")
	}
	bob := login(t, s, "bob")
	for i := 0; i < 128; i++ {
		r, _ = s.Handle(t.Context(), alice, message)
		if r[0].Opcode != 1 {
			t.Fatal("full memory queue must spill to the durable outbox")
		}
	}
	r, _ = s.Handle(t.Context(), alice, message)
	if r[0].Opcode != 1 {
		t.Fatal("overflow message was not durably accepted")
	}
	if len(s.sessions[bob].push) != 128 {
		t.Fatal("unbounded queue")
	}
	first := requestPacket(t, s, bob, nil)
	second := requestPacket(t, s, bob, nil)
	if len(first) != 128 || len(second) != 2 {
		t.Fatalf("lost queued messages: %d + %d", len(first), len(second))
	}
	pending, err := s.store.Pending(t.Context(), "bob", 200)
	if err != nil || len(pending) != 0 {
		t.Fatalf("delivered messages stayed pending: %v %v", pending, err)
	}
}

func TestHTTPRejectsTrailingPartialPacketWithoutSideEffects(t *testing.T) {
	s := newTestServer(t)
	var b bytes.Buffer
	protocol.Encode(&b, protocol.New(200, 1).SetText(5, "alice"))
	b.WriteByte(2)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("POST", "http://localhost/", &b))
	if w.Code != 400 || len(s.sessions) != 0 {
		t.Fatal("malformed request created session")
	}
	_, err := io.Copy(io.Discard, w.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
}
