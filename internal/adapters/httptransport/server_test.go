package httptransport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

func httpTestLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestHTTPStopDrainsAcceptedRequests(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	server, err := NewServer(&http.Server{Addr: "127.0.0.1:0", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
			_, _ = io.WriteString(w, "drained")
		case <-r.Context().Done():
			t.Error("accepted request canceled before graceful drain")
		}
	})}, httpTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	running, cancel := context.WithCancel(t.Context())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- server.Run(running) }()
	response := make(chan error, 1)
	go func() {
		r, err := (&http.Client{Timeout: time.Second}).Get("http://" + server.Addr().String())
		if err == nil {
			defer r.Body.Close()
			body, readErr := io.ReadAll(r.Body)
			err = readErr
			if string(body) != "drained" {
				err = errors.New("response was not drained")
			}
		}
		response <- err
	}()
	<-entered
	cancel()
	shutdown, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	stopped := make(chan error, 1)
	go func() { stopped <- server.Stop(shutdown) }()
	select {
	case err := <-stopped:
		t.Fatalf("Stop returned before handler completed: %v", err)
	case <-time.After(15 * time.Millisecond):
	}
	close(release)
	if err := <-response; err != nil {
		t.Fatal(err)
	}
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	if err := <-served; err != nil {
		t.Fatal(err)
	}
}

func TestHTTPDeadlineClosesConnectionsAndJoinsHandlers(t *testing.T) {
	entered, exited := make(chan struct{}), make(chan struct{})
	server, err := NewServer(&http.Server{Addr: "127.0.0.1:0", Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done(); close(exited) })}, httpTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() { served <- server.Run(t.Context()) }()
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		r, err := (&http.Client{Timeout: time.Second}).Get("http://" + server.Addr().String())
		if err == nil {
			r.Body.Close()
		}
	}()
	<-entered
	shutdown, stop := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer stop()
	if err := server.Stop(shutdown); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline error lost: %v", err)
	}
	select {
	case <-exited:
	default:
		t.Fatal("Stop returned while canceled handler still running")
	}
	if err := <-served; err != nil {
		t.Fatal(err)
	}
	<-clientDone
}
