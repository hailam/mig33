package socket

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
)

// Listener owns the network lifecycle around the native packet server.
type Listener struct {
	server           *Server
	address          string
	logger           *slog.Logger
	mu               sync.Mutex
	listener         net.Listener
	cancel           context.CancelFunc
	done             chan struct{}
	running, stopped bool
}

func NewListener(server *Server, address string, logger *slog.Logger) (*Listener, error) {
	if server == nil {
		return nil, fmt.Errorf("socket server is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("socket logger is required")
	}

	return &Listener{server: server, address: address, logger: logger, done: make(chan struct{})}, nil
}

func (s *Listener) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return fmt.Errorf("socket server is required")
	}
	if s.listener != nil || s.stopped {
		return fmt.Errorf("socket listener cannot be started again")
	}

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", s.address)
	if err != nil {
		return err
	}

	s.listener = listener
	s.logger.Info("Socket listening", "address", listener.Addr().String())
	return nil
}

func (s *Listener) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	if s.listener == nil {
		s.mu.Unlock()
		return fmt.Errorf("socket listener was not started")
	}
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("socket listener already running")
	}

	running, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true
	listener := s.listener
	s.mu.Unlock()
	defer cancel()
	defer close(s.done)
	err := s.server.Serve(running, listener)
	if errors.Is(err, net.ErrClosed) {
		return nil
	}

	return err
}

func (s *Listener) Stop(ctx context.Context) error {
	s.mu.Lock()
	s.stopped = true
	listener, cancel, running := s.listener, s.cancel, s.running
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	var err error
	if listener != nil {
		err = listener.Close()
		if errors.Is(err, net.ErrClosed) {
			err = nil
		}
	}
	if running {
		select {
		case <-s.done:
		case <-ctx.Done():
			err = errors.Join(err, ctx.Err())
		}
	}

	return err
}

func (s *Listener) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return nil
	}

	return s.listener.Addr()
}
