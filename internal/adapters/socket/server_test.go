package socket

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/protocol"
)

type blockedWriteGateway struct {
	started   chan struct{}
	delivered chan bool
}

func (g *blockedWriteGateway) Handle(ctx context.Context, _ string, p *protocol.Packet) ([]*protocol.Packet, string) {
	return []*protocol.Packet{protocol.New(3, p.Transaction)}, "session"
}
func (g *blockedWriteGateway) DropSession(context.Context, string) {}
func (g *blockedWriteGateway) BeginWrite(string) func()            { close(g.started); return func() {} }

func (g *blockedWriteGateway) AllowPacket(context.Context, string, *protocol.Packet) bool {
	return true
}

func (g *blockedWriteGateway) FinishWrite(ctx context.Context, _ []*protocol.Packet, written bool) {
	g.delivered <- written
}

func (g *blockedWriteGateway) BeginResponse(string) func() { return func() {} }

func (g *blockedWriteGateway) TryBeginResponse(string) (func(), bool)               { return func() {}, true }
func (g *blockedWriteGateway) Poll(context.Context, string, int) []*protocol.Packet { return nil }

func TestCancellationInterruptsBlockedWrite(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gateway := &blockedWriteGateway{started: make(chan struct{}), delivered: make(chan bool, 1)}
	done := make(chan struct{})
	go func() { New(gateway).serveConnection(ctx, server); close(done) }()
	client.SetWriteDeadline(time.Now().Add(time.Second))
	if err := protocol.Encode(client, protocol.New(2, 1)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-gateway.started:
	case <-time.After(time.Second):
		t.Fatal("response did not start")
	}
	// The peer deliberately never reads: network writes must unblock on shutdown.
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancellation left a blocked socket worker")
	}
	select {
	case written := <-gateway.delivered:
		if written {
			t.Fatal("canceled write acknowledged delivery")
		}
	default:
		t.Fatal("write result missing")
	}
}
