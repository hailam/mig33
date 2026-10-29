package realtime

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/hailam/mig33/internal/protocol"
)

func privateMessage(target, body string) *protocol.Packet {
	return protocol.New(500, 50).SetByte(1, 1).SetText(2, "forged").SetByte(3, 1).SetText(4, target).SetShort(6, 1).SetText(8, body)
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

func TestQueuedGroupMessageCanceledAcrossLeaveAndRejoin(t *testing.T) {
	s := newTestServer(t)
	alice := login(t, s, "alice")
	bob := login(t, s, "bob")
	carol := login(t, s, "carol")
	r, _ := s.Handle(t.Context(), alice, protocol.New(751, 1).SetText(1, "bob").SetText(2, "carol").SetByte(3, 1))
	group, _ := r[0].Text(1)
	r, _ = s.Handle(t.Context(), alice, privateMessage(group, "sent before leave").SetByte(3, 2))
	if r[0].Opcode != 1 {
		t.Fatal("group send")
	}
	if len(s.sessions[carol].inflight) != 1 {
		t.Fatal("missing inflight message")
	}
	s.Handle(t.Context(), carol, protocol.New(753, 2).SetText(1, group).SetByte(2, 1))
	s.Handle(t.Context(), bob, protocol.New(752, 3).SetText(1, group).SetText(2, "carol").SetByte(3, 1))
	packets := requestPacket(t, s, carol, nil)
	for _, p := range packets {
		if body, _ := p.Text(8); body == "sent before leave" {
			t.Fatal("canceled queued message reopened after rejoin")
		}
	}
	history, err := s.store.History(t.Context(), "carol", 2, group, 0, 50)
	if err != nil || len(history) != 1 {
		t.Fatal("leave erased legitimate old history")
	}
}

type brokenResponse struct {
	header    http.Header
	failWrite bool
	body      bytes.Buffer
}
