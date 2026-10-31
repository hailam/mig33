package httptransport

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/testtls"
)

func startHTTPS(t *testing.T, timeout time.Duration, handler http.Handler) (*Server, *http.Client, <-chan struct{}, <-chan error) {
	t.Helper()
	identity := testtls.New(t)
	accepted := make(chan struct{}, 8)
	server, err := NewTLSServer(&http.Server{
		Addr: "127.0.0.1:0", Handler: handler,
		ConnState: func(_ net.Conn, state http.ConnState) {
			if state == http.StateNew {
				accepted <- struct{}{}
			}
		},
	}, identity.Server, timeout, httpTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{TLSClientConfig: identity.Client}
	client := &http.Client{Transport: transport, Timeout: time.Second}
	done := make(chan error, 1)
	go func() { done <- server.Run(t.Context()) }()
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	return server, client, accepted, done
}

func TestHTTPSRequestAndGracefulDrain(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	server, client, _, done := startHTTPS(t, time.Second, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || r.TLS.Version < tls.VersionTLS12 {
			t.Error("request did not use modern TLS")
		}
		close(entered)
		select {
		case <-release:
			io.WriteString(w, "drained over TLS")
		case <-r.Context().Done():
			t.Error("request canceled before drain")
		}
	}))
	response := make(chan error, 1)
	go func() {
		r, err := client.Get("https://" + server.Addr().String())
		if err == nil {
			defer r.Body.Close()
			body, readErr := io.ReadAll(r.Body)
			err = readErr
			if string(body) != "drained over TLS" {
				err = errors.New("response was not drained")
			}
		}
		response <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("HTTPS request did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- server.Stop(ctx) }()
	select {
	case err := <-stopped:
		t.Fatalf("Stop returned before handler: %v", err)
	case <-time.After(15 * time.Millisecond):
	}
	close(release)
	if err := <-response; err != nil {
		t.Fatal(err)
	}
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestHTTPSStalledHandshakeTimeoutAndShutdown(t *testing.T) {
	for _, stop := range []bool{false, true} {
		name, timeout := "timeout", 40*time.Millisecond
		if stop {
			name, timeout = "shutdown", time.Minute
		}
		t.Run(name, func(t *testing.T) {
			server, _, accepted, done := startHTTPS(t, timeout, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("stalled handshake reached HTTP handler") }))
			connection, err := net.DialTimeout("tcp", server.Addr().String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			select {
			case <-accepted:
			case <-time.After(time.Second):
				t.Fatal("connection not accepted")
			}
			if stop {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if err := server.Stop(ctx); err != nil {
					t.Fatal(err)
				}
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			}
			connection.SetReadDeadline(time.Now().Add(time.Second))
			_, err = connection.Read(make([]byte, 1))
			var networkError net.Error
			if err == nil || (errors.As(err, &networkError) && networkError.Timeout()) {
				t.Fatalf("stalled handshake not closed: %v", err)
			}
		})
	}
}
