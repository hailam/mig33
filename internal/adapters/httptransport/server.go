package httptransport

import (
	"context"
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
	s.logger.Info("HTTP listening", "address", listener.Addr().String())
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

	err := s.server.Serve(listener)
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
	s.mu.Unlock()
	if listener == nil {
		return nil
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
