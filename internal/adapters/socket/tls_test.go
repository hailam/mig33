package socket

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/protocol"
	"github.com/hailam/mig33/internal/testtls"
)

type pingGateway struct{ handled atomic.Int32 }

func (g *pingGateway) Handle(_ context.Context, _ string, p *protocol.Packet) ([]*protocol.Packet, string) {
	g.handled.Add(1)
	return []*protocol.Packet{protocol.New(3, p.Transaction)}, ""
}
func (*pingGateway) DropSession(context.Context, string)                        {}
func (*pingGateway) BeginWrite(string) func()                                   { return func() {} }
func (*pingGateway) AllowPacket(context.Context, string, *protocol.Packet) bool { return true }
func (*pingGateway) FinishWrite(context.Context, []*protocol.Packet, bool)      {}
func (*pingGateway) BeginResponse(string) func()                                { return func() {} }
func (*pingGateway) TryBeginResponse(string) (func(), bool)                     { return func() {}, true }
func (*pingGateway) Poll(context.Context, string, int) []*protocol.Packet       { return nil }

func startTLS(t *testing.T, timeout time.Duration) (*Listener, *tls.Config, *pingGateway, <-chan error) {
	t.Helper()
	identity := testtls.New(t)
	gateway := &pingGateway{}
	server, err := NewTLSListener(New(gateway), "127.0.0.1:0", identity.Server, timeout, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Run(t.Context()) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	return server, identity.Client, gateway, done
}

func TestTLSNativePacketsAndIdentityVerification(t *testing.T) {
	server, trust, gateway, _ := startTLS(t, time.Second)
	connection, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", server.Addr().String(), trust)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	connection.SetDeadline(time.Now().Add(time.Second))
	if connection.ConnectionState().Version < tls.VersionTLS12 {
		t.Fatal("obsolete TLS version negotiated")
	}
	if err := protocol.Encode(connection, protocol.New(2, 7)); err != nil {
		t.Fatal(err)
	}
	packet, err := protocol.Decode(connection)
	if err != nil || packet.Opcode != 3 || packet.Transaction != 7 {
		t.Fatalf("native TLS ping: %v %v", packet, err)
	}
	for _, name := range []string{"untrusted", "wrong host", "TLS 1.1"} {
		t.Run(name, func(t *testing.T) {
			configuration := trust.Clone()
			switch name {
			case "untrusted":
				configuration.RootCAs = x509.NewCertPool()
			case "wrong host":
				configuration.ServerName = "wrong.example"
			case "TLS 1.1":
				configuration.MinVersion = tls.VersionTLS10
				configuration.MaxVersion = tls.VersionTLS11
			}
			client, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", server.Addr().String(), configuration)
			if err == nil {
				client.Close()
				t.Fatal("invalid TLS connection accepted")
			}
		})
	}
	if gateway.handled.Load() != 1 {
		t.Fatal("failed TLS client reached packet handler")
	}
}

func TestTLSHandshakeTimesOutBeforePacketHandling(t *testing.T) {
	server, _, gateway, _ := startTLS(t, 40*time.Millisecond)
	connection, err := net.DialTimeout("tcp", server.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	connection.SetReadDeadline(time.Now().Add(time.Second))
	_, err = connection.Read(make([]byte, 1))
	var networkError net.Error
	if err == nil || (errors.As(err, &networkError) && networkError.Timeout()) {
		t.Fatalf("stalled handshake not closed: %v", err)
	}
	if gateway.handled.Load() != 0 {
		t.Fatal("unhandshaken client reached packet handler")
	}
}

func TestTLSShutdownJoinsStalledHandshake(t *testing.T) {
	server, _, gateway, done := startTLS(t, time.Minute)
	connection, err := net.DialTimeout("tcp", server.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	// A partial TLS record keeps an accepted worker blocked in HandshakeContext.
	if _, err := connection.Write([]byte{22, 3, 1, 0, 100}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("TLS worker survived shutdown")
	}
	connection.SetReadDeadline(time.Now().Add(time.Second))
	_, err = connection.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("handshake connection survived shutdown")
	}
	if gateway.handled.Load() != 0 {
		t.Fatal("partial TLS reached packet handler")
	}
}
