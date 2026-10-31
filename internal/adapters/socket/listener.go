package socket

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
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
	tlsConfig        *tls.Config
	handshakeTimeout time.Duration
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

// NewTLSListener adds TLS around the unchanged native packet stream. The
// certificate must already be loaded before any listener is acquired.
func NewTLSListener(server *Server, address string, configuration *tls.Config, handshakeTimeout time.Duration, logger *slog.Logger) (*Listener, error) {
	if configuration == nil || len(configuration.Certificates) == 0 {
		return nil, fmt.Errorf("socket TLS certificate is required")
	}
	if handshakeTimeout <= 0 {
		return nil, fmt.Errorf("socket TLS handshake timeout must be positive")
	}
	listener, err := NewListener(server, address, logger)
	if err != nil {
		return nil, err
	}
	listener.tlsConfig = configuration.Clone()
	listener.tlsConfig.MinVersion = max(tls.VersionTLS12, listener.tlsConfig.MinVersion)
	listener.handshakeTimeout = handshakeTimeout
	return listener, nil
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
	s.logger.Info("Socket listening", "address", listener.Addr().String(), "tls", s.tlsConfig != nil)
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
	var err error
	if s.tlsConfig != nil {
		err = s.server.serve(running, tls.NewListener(listener, s.tlsConfig), s.handshakeTimeout)
	} else {
		err = s.server.Serve(running, listener)
	}
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
