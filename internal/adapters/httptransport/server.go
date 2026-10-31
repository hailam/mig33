package httptransport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// Server owns the listener and supplied HTTP server. Request contexts remain
// independent of Run's context so Stop can drain accepted work.
type Server struct {
	server   *http.Server
	logger   *slog.Logger
	mu       sync.Mutex
	listener net.Listener
	stopped  bool
	active   int
	idle     chan struct{}
	tls      bool
	pending  map[net.Conn]struct{}
}

func NewServer(server *http.Server, logger *slog.Logger) (*Server, error) {
	if server == nil || server.Handler == nil {
		return nil, fmt.Errorf("HTTP server and handler are required")
	}
	if logger == nil {
		return nil, fmt.Errorf("HTTP logger is required")
	}

	idle := make(chan struct{})
	close(idle)
	s := &Server{server: server, logger: logger, idle: idle}

	handler := server.Handler
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		if s.stopped {
			s.mu.Unlock()
			http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
			return
		}
		if s.active == 0 {
			s.idle = make(chan struct{})
		}

		s.active++
		s.mu.Unlock()

		defer func() {
			s.mu.Lock()
			s.active--
			if s.active == 0 {
				close(s.idle)
			}

			s.mu.Unlock()
		}()

		handler.ServeHTTP(w, r)
	})
	return s, nil
}

// NewTLSServer serves a separately configured HTTPS listener. Pending TLS
// handshakes are closed on shutdown; accepted HTTP requests still drain.
func NewTLSServer(server *http.Server, configuration *tls.Config, handshakeTimeout time.Duration, logger *slog.Logger) (*Server, error) {
	if configuration == nil || len(configuration.Certificates) == 0 {
		return nil, fmt.Errorf("HTTPS certificate is required")
	}
	if handshakeTimeout <= 0 {
		return nil, fmt.Errorf("HTTPS handshake timeout must be positive")
	}
	s, err := NewServer(server, logger)
	if err != nil {
		return nil, err
	}
	s.tls = true
	s.pending = make(map[net.Conn]struct{})
	server.TLSConfig = configuration.Clone()
	server.TLSConfig.MinVersion = max(tls.VersionTLS12, server.TLSConfig.MinVersion)
	// net/http bounds TLS handshakes by the smallest positive read-header,
	// read or write timeout. This also caps the HTTPS initial-header budget.
	if server.ReadHeaderTimeout <= 0 || server.ReadHeaderTimeout > handshakeTimeout {
		server.ReadHeaderTimeout = handshakeTimeout
	}
	previous := server.ConnState
	server.ConnState = func(conn net.Conn, state http.ConnState) {
		s.mu.Lock()
		closePending := state == http.StateNew && s.stopped
		if state == http.StateNew && !s.stopped {
			s.pending[conn] = struct{}{}
		} else {
			delete(s.pending, conn)
		}
		s.mu.Unlock()
		if closePending {
			conn.Close()
		}
		if previous != nil {
			previous(conn, state)
		}
	}
	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil || s.stopped {
		return fmt.Errorf("HTTP server cannot be started again")
	}

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", s.server.Addr)
	if err != nil {
		return err
	}

	s.listener = listener
	s.logger.Info("HTTP listening", "address", listener.Addr().String(), "tls", s.tls)
	return nil
}

func (s *Server) Run(context.Context) error {
	s.mu.Lock()
	listener, stopped := s.listener, s.stopped
	s.mu.Unlock()
	if stopped {
		return nil
	}
	if listener == nil {
		return fmt.Errorf("HTTP server was not started")
	}

	var err error
	if s.tls {
		err = s.server.ServeTLS(listener, "", "")
	} else {
		err = s.server.Serve(listener)
	}
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}

	return err
}

// Stop reserves the final tenth of its remaining budget for forced connection
// closure and handler joining. Handlers must observe their request contexts;
// arbitrary context-ignoring Go code cannot be forcibly terminated.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	s.stopped = true
	listener, idle := s.listener, s.idle
	pending := make([]net.Conn, 0, len(s.pending))
	for conn := range s.pending {
		pending = append(pending, conn)
	}
	s.mu.Unlock()
	if listener == nil {
		return nil
	}
	for _, conn := range pending {
		conn.Close()
	}

	drain := ctx
	cancel := func() {}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 {
			drain, cancel = context.WithDeadline(ctx, time.Now().Add(remaining*9/10))
		}
	}
	defer cancel()

	err := s.server.Shutdown(drain)
	closeErr := listener.Close()
	if errors.Is(closeErr, net.ErrClosed) {
		closeErr = nil
	}
	if err != nil {
		err = errors.Join(err, s.server.Close())
	}

	select {
	case <-idle:
	case <-ctx.Done():
		err = errors.Join(err, fmt.Errorf("HTTP handlers shutdown: %w", ctx.Err()))
	}

	return errors.Join(err, closeErr)
}

func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return nil
	}

	return s.listener.Addr()
}
