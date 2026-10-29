package wiring

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/adapters/httptransport"
	"github.com/hailam/mig33/internal/adapters/socket"
	"github.com/hailam/mig33/internal/bootstrap"
	"github.com/hailam/mig33/internal/config"
	"github.com/hailam/mig33/internal/controllers"
	"github.com/hailam/mig33/internal/protocol"
	"github.com/hailam/mig33/internal/testassets"
)

func compositionLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
func configuration(t *testing.T) config.Config {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "app.yaml"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(config.LoadOptions{Directory: directory, Environ: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Database.Path = ":memory:"
	cfg.HTTP.Listen = "127.0.0.1:0"
	cfg.Socket.Listen = "127.0.0.1:0"
	cfg.Resources.Directory = filepath.Join(t.TempDir(), "absent-assets")
	return cfg
}

func TestOptionalResourcesAreLoadedFromConfiguredDirectory(t *testing.T) {
	for _, present := range []bool{false, true} {
		t.Run(fmt.Sprintf("present=%t", present), func(t *testing.T) {
			cfg := configuration(t)
			if present {
				cfg.Resources.Directory = t.TempDir()
				if err := os.WriteFile(filepath.Join(cfg.Resources.Directory, "en-US"), testassets.Bytes("en-US"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			graph, cleanup, err := Open(t.Context(), cfg, compositionLogger())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := cleanup(); err != nil {
					t.Error(err)
				}
			})
			handler := httptransport.New(controllers.New(graph.HTTP))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", "/sites/resources/language/v4.6/en-US", nil))
			if !present {
				if response.Code != http.StatusServiceUnavailable {
					t.Fatalf("missing resource: %d %s", response.Code, response.Body)
				}
				return
			}
			if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), testassets.Bytes("en-US")) {
				t.Fatalf("configured resource: %d %s", response.Code, response.Body)
			}
		})
	}
}

func launch(t *testing.T, components ...bootstrap.Component) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- bootstrap.Run(ctx, time.Second, compositionLogger(), components...) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(2 * time.Second):
				t.Error("composition did not stop")
			}
		})
	}
	t.Cleanup(stop)
	return stop
}

func TestExplicitTransportSelectionsAndSharedAuthentication(t *testing.T) {
	for _, selection := range []struct {
		name         string
		http, socket bool
	}{{"HTTP", true, false}, {"socket", false, true}, {"both", true, true}} {
		t.Run(selection.name, func(t *testing.T) {
			cfg := configuration(t)
			logger := compositionLogger()
			graph, cleanup, err := Open(t.Context(), cfg, logger)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := cleanup(); err != nil {
					t.Error(err)
				}
			})
			components := []bootstrap.Component{{Name: "core", Run: graph.Core.Run}}
			ready := make(chan net.Addr, 2)
			var httpServer *httptransport.Server
			var socketServer *socket.Listener
			if selection.http {
				httpServer, err = httptransport.NewServer(&http.Server{Addr: cfg.HTTP.Listen, Handler: httptransport.New(controllers.New(graph.HTTP))}, logger)
				if err != nil {
					t.Fatal(err)
				}
				components = append(components, bootstrap.Component{
					Name: "HTTP",
					Start: func(ctx context.Context) error {
						if err := httpServer.Start(ctx); err != nil {
							return err
						}
						ready <- httpServer.Addr()
						return nil
					},
					Run:  httpServer.Run,
					Stop: httpServer.Stop,
				})
			}
			if selection.socket {
				socketServer, err = socket.NewListener(socket.New(graph.Native), cfg.Socket.Listen, logger)
				if err != nil {
					t.Fatal(err)
				}
				components = append(components, bootstrap.Component{
					Name: "socket",
					Start: func(ctx context.Context) error {
						if err := socketServer.Start(ctx); err != nil {
							return err
						}
						ready <- socketServer.Addr()
						return nil
					},
					Run:  socketServer.Run,
					Stop: socketServer.Stop,
				})
			}
			stop := launch(t, components...)
			var addresses []net.Addr
			for range len(components) - 1 {
				select {
				case addr := <-ready:
					addresses = append(addresses, addr)
				case <-time.After(time.Second):
					t.Fatal("transport did not bind")
				}
			}
			if selection.http {
				response, err := (&http.Client{Timeout: time.Second}).Get("http://" + httpServer.Addr().String() + "/healthz")
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				if response.StatusCode != 200 {
					t.Fatal(response.StatusCode)
				}
			}
			if selection.socket {
				connection, err := net.DialTimeout("tcp", socketServer.Addr().String(), time.Second)
				if err != nil {
					t.Fatal(err)
				}
				defer connection.Close()
				connection.SetDeadline(time.Now().Add(2 * time.Second))
				if err := protocol.Encode(connection, protocol.New(2, 7)); err != nil {
					t.Fatal(err)
				}
				packet, err := protocol.Decode(connection)
				if err != nil || packet.Opcode != 3 || packet.Transaction != 7 {
					t.Fatalf("socket ping: %v %v", packet, err)
				}
				if selection.http {
					if err := protocol.Encode(connection, protocol.New(200, 1).SetText(5, "alice")); err != nil {
						t.Fatal(err)
					}
					challenge, err := protocol.Decode(connection)
					if err != nil || challenge.Opcode != 201 {
						t.Fatalf("challenge: %v %v", challenge, err)
					}
					value, _ := challenge.Text(1)
					sid, _ := challenge.Text(2)
					if err := protocol.Encode(connection, protocol.New(202, 2).SetInt(1, protocol.JavaHash(value+"alice-demo"))); err != nil {
						t.Fatal(err)
					}
					login, err := protocol.Decode(connection)
					if err != nil || login.Opcode != 203 {
						t.Fatalf("login: %v %v", login, err)
					}
					request, _ := http.NewRequestWithContext(t.Context(), "GET", "http://"+httpServer.Addr().String()+"/api/chat/session", nil)
					request.Header.Set("sid", sid)
					response, err := (&http.Client{Timeout: time.Second}).Do(request)
					if err != nil {
						t.Fatal(err)
					}
					body, err := io.ReadAll(response.Body)
					response.Body.Close()
					if err != nil || response.StatusCode != 200 || !bytes.Contains(body, []byte(`"username":"alice"`)) {
						t.Fatalf("shared authentication: %d %s %v", response.StatusCode, body, err)
					}
				}
			}
			stop()
			if len(ready) != 0 {
				t.Fatal("unselected listener opened")
			}
			for _, addr := range addresses {
				conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
				if err == nil {
					conn.Close()
					t.Fatal("listener survived shutdown")
				}
			}
		})
	}
}

func TestListenerStartupRollback(t *testing.T) {
	server, err := httptransport.NewServer(&http.Server{Addr: "127.0.0.1:0", Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("served during partial startup") })}, compositionLogger())
	if err != nil {
		t.Fatal(err)
	}
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	second, err := httptransport.NewServer(&http.Server{Addr: occupied.Addr().String(), Handler: http.NotFoundHandler()}, compositionLogger())
	if err != nil {
		t.Fatal(err)
	}
	err = bootstrap.Run(t.Context(), time.Second, compositionLogger(), bootstrap.Component{Name: "first", Start: server.Start, Run: server.Run, Stop: server.Stop}, bootstrap.Component{Name: "second", Start: second.Start, Run: second.Run, Stop: second.Stop})
	if err == nil {
		t.Fatal("occupied listener accepted")
	}
	connection, dialErr := net.DialTimeout("tcp", server.Addr().String(), time.Second)
	if dialErr == nil {
		connection.Close()
		t.Fatal("first bound listener leaked")
	}
}

func TestOpenCanceledDoesNotCreateDatabase(t *testing.T) {
	cfg := configuration(t)
	cfg.Database.Path = filepath.Join(t.TempDir(), "must-not-exist.sqlite")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Open(ctx, cfg, compositionLogger()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled construction: %v", err)
	}
	if _, err := os.Stat(cfg.Database.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled construction touched database: %v", err)
	}
}
