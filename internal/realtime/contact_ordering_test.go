package realtime

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	chatstore "github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func TestContactQueuedAddPrecedesNewDeleteResponse(t *testing.T) {
	s := newTestServer(t)
	a, b := loginHailam(t, s), loginHailam(t, s)
	contactRequest(t, s, a, protocol.New(405, 21).SetText(12, "bob").SetInt(2, 1), 402)
	r := requestPacket(t, s, b, protocol.New(406, 22).SetInt(1, 2))
	if len(r) != 4 || r[0].Opcode != 402 || r[1].Opcode != 420 || r[2].Opcode != 1 || r[2].Transaction != 22 || r[3].Opcode != 420 {
		t.Fatalf("old add must precede delete ACK/checkpoint: %v", opcodes(r))
	}
	oldRevision, _ := r[1].Int(1)
	newRevision, _ := r[3].Int(1)
	oldCheckpoint, _ := r[1].Long(2)
	newCheckpoint, _ := r[3].Long(2)
	if newRevision <= oldRevision || newCheckpoint <= oldCheckpoint {
		t.Fatalf("checkpoint regressed: %d/%d -> %d/%d", oldRevision, oldCheckpoint, newRevision, newCheckpoint)
	}
	if _, err := s.store.Contact(t.Context(), "hailam", 2); !errors.Is(err, chatstore.ErrNotFound) {
		t.Fatalf("delete not persisted: %v", err)
	}
	if r := requestPacket(t, s, b, nil); len(r) != 0 {
		t.Fatalf("older add remained queued after delete: %v", opcodes(r))
	}
}

func TestContactFullSnapshotSupersedesQueuedRosterEvents(t *testing.T) {
	for _, kind := range []string{"contact", "group"} {
		t.Run(kind, func(t *testing.T) {
			s := newTestServer(t)
			a, b := loginHailam(t, s), loginHailam(t, s)
			var deletedID int32
			if kind == "contact" {
				contactRequest(t, s, a, protocol.New(405, 21).SetText(12, "bob").SetInt(2, 1), 402)
				deletedID = 2
			} else {
				r := contactRequest(t, s, a, protocol.New(408, 21).SetText(2, "Temporary"), 401)
				deletedID, _ = r[0].Int(1)
			}
			// Overflow prevents the following delete from being pushed. Older
			// add/checkpoint packets are still queued, requiring a full refresh.
			s.mu.Lock()
			fillers := cap(s.sessions[b].push) - len(s.sessions[b].push)
			for i := 0; i < fillers; i++ {
				s.sessions[b].push <- events.New(int16(i+100), events.Pong{})
			}
			s.mu.Unlock()
			opcode := int16(406)
			if kind == "group" {
				opcode = 409
			}
			contactRequest(t, s, a, protocol.New(opcode, 22).SetInt(1, deletedID), 1)
			s.mu.Lock()
			stale := s.sessions[b].rosterStale
			s.mu.Unlock()
			if !stale {
				t.Fatal("overflow/group change must require a roster refresh")
			}
			r := requestPacket(t, s, b, protocol.New(400, 23))
			preserved := 0
			checkpoints := 0
			for _, p := range r {
				if p.Opcode == 3 {
					if int(p.Transaction) != preserved+100 {
						t.Fatal("non-roster prefix order changed")
					}
					preserved++
				}
				if p.Opcode == 402 {
					t.Fatal("queued old contact add resurrected a contact absent from full snapshot")
				}
				if p.Opcode == 401 {
					if id, _ := p.Int(1); id != 1 {
						t.Fatalf("queued old group add resurrected group %d", id)
					}
				}
				if p.Opcode == 420 {
					checkpoints++
					if p.Transaction != 23 {
						t.Fatal("superseded checkpoint retained in full response")
					}
				}
			}
			if preserved != fillers || checkpoints != 1 || r[len(r)-1].Opcode != 403 {
				t.Fatalf("full response/prefix incomplete: fillers=%d/%d checkpoints=%d", preserved, fillers, checkpoints)
			}
			if r := requestPacket(t, s, b, nil); len(r) != 0 {
				t.Fatalf("stale events remain after full snapshot: %v", opcodes(r))
			}
		})
	}
}

func opcodes(packets []*protocol.Packet) []int16 {
	result := make([]int16, len(packets))
	for i, p := range packets {
		result[i] = p.Opcode
	}
	return result
}

type pausedContactResponse struct {
	*httptest.ResponseRecorder
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *pausedContactResponse) Write(p []byte) (int, error) {
	close(w.started)
	<-w.release
	return w.ResponseRecorder.Write(p)
}

func (w *pausedContactResponse) resume() { w.once.Do(func() { close(w.release) }) }

func pausedContactWriter(t *testing.T) *pausedContactResponse {
	t.Helper()
	w := &pausedContactResponse{ResponseRecorder: httptest.NewRecorder(), started: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(w.resume)
	return w
}

func waitContactEvent(t *testing.T, done <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal(message)
	}
}

func TestContactResponsesSerializePerSessionThroughWrite(t *testing.T) {
	s := newTestServer(t)
	a, b := loginHailam(t, s), loginHailam(t, s)
	contactRequest(t, s, a, protocol.New(405, 21).SetText(12, "bob").SetInt(2, 1), 402)
	w := pausedContactWriter(t)
	firstDone := make(chan struct{})
	go func() {
		s.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/"+b, nil))
		close(firstDone)
	}()
	waitContactEvent(t, w.started, "first poll never started writing")
	var body bytes.Buffer
	if err := protocol.Encode(&body, protocol.New(406, 22).SetInt(1, 2)); err != nil {
		t.Fatal(err)
	}
	second := httptest.NewRecorder()
	secondDone := make(chan struct{})
	go func() {
		s.ServeHTTP(second, httptest.NewRequest("POST", "http://localhost/"+b, &body))
		close(secondDone)
	}()
	select {
	case <-secondDone:
		t.Fatal("later same-session delete overtook unfinished add response")
	case <-time.After(30 * time.Millisecond):
	}
	if _, err := s.store.Contact(t.Context(), "hailam", 2); err != nil {
		t.Fatal("mutation ran before earlier response finished writing")
	}
	// A different session, even for the same owner, remains usable while one
	// response writer is slow. Its newer queued event will precede B's delete.
	independent := make(chan struct{})
	go func() {
		s.Handle(t.Context(), a, protocol.New(405, 24).SetText(12, "carol").SetInt(2, 1))
		close(independent)
	}()
	waitContactEvent(t, independent, "different session stalled behind response gate")
	w.resume()
	waitContactEvent(t, firstDone, "first response did not finish")
	waitContactEvent(t, secondDone, "second response did not resume")
	if second.Code != http.StatusOK {
		t.Fatalf("second HTTP status: %d", second.Code)
	}
	var packets []*protocol.Packet
	for second.Body.Len() > 0 {
		p, err := protocol.Decode(second.Body)
		if err != nil {
			t.Fatal(err)
		}
		packets = append(packets, p)
	}
	if len(packets) != 4 || packets[0].Opcode != 402 || packets[2].Opcode != 1 || packets[2].Transaction != 22 {
		t.Fatalf("newer pending Carol add not ordered before delete: %v", opcodes(packets))
	}
	if target, _ := packets[0].Text(12); target != "carol" {
		t.Fatalf("wrong pending contact update: %q", target)
	}
	if _, err := s.store.Contact(t.Context(), "hailam", 2); !errors.Is(err, chatstore.ErrNotFound) {
		t.Fatalf("delete failed after gated response: %v", err)
	}
}

func TestTCPDoesNotPopPushWhileSameSessionHTTPWriteIsBlocked(t *testing.T) {
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
	send := func(p *protocol.Packet) {
		t.Helper()
		if err := protocol.Encode(conn, p); err != nil {
			t.Fatal(err)
		}
	}
	receive := func() *protocol.Packet {
		t.Helper()
		p, err := protocol.Decode(conn)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	send(protocol.New(200, 1).SetText(5, "hailam"))
	r := receive()
	challenge, _ := r.Text(1)
	id, _ := r.Text(2)
	send(protocol.New(202, 2).SetInt(1, protocol.JavaHash(challenge+"hailam")))
	if p := receive(); p.Opcode != 203 {
		t.Fatal("TCP login failed")
	}
	send(protocol.New(400, 3))
	for receive().Opcode != 403 {
	}
	w := pausedContactWriter(t)
	httpDone := make(chan struct{})
	go func() {
		s.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/"+id, nil))
		close(httpDone)
	}()
	waitContactEvent(t, w.started, "shared-SID HTTP write never started")
	other := loginHailam(t, s)
	contactRequest(t, s, other, protocol.New(405, 21).SetText(12, "bob").SetInt(2, 1), 402)
	// Several TCP idle polls must occur while the response gate is occupied.
	// Taking a packet before acquiring that gate would permit it to be overtaken.
	time.Sleep(80 * time.Millisecond)
	s.mu.Lock()
	queued := len(s.sessions[id].push)
	s.mu.Unlock()
	if queued != 2 {
		t.Fatalf("TCP popped queued contact data before obtaining response gate: %d packets remain", queued)
	}
	w.resume()
	waitContactEvent(t, httpDone, "HTTP response did not finish")
	if receive().Opcode != 402 || receive().Opcode != 420 {
		t.Fatal("TCP did not deliver queued contact/checkpoint after HTTP completed")
	}
	conn.Close()
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("TCP server did not shut down")
	}
}
