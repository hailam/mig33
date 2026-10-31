// Package socket adapts the native packet application port to TCP streams.
package socket

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/hailam/mig33/internal/ports"
	"github.com/hailam/mig33/internal/protocol"
)

type Server struct{ app ports.NativeService }

func New(app ports.NativeService) *Server { return &Server{app: app} }

// Serve accepts original binary-protocol streams until cancellation.
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	return s.serve(ctx, listener, 0)
}

func (s *Server) serve(ctx context.Context, listener net.Listener, handshakeTimeout time.Duration) error {
	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() {
		cancel()
		listener.Close()
		workers.Wait()
	}()

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}

			return err
		}

		workers.Add(1)
		go func() {
			defer workers.Done()
			if secure, ok := conn.(*tls.Conn); ok {
				// No application session or decoder exists until the TLS handshake
				// completes. Cancellation joins stalled handshakes as well as readers.
				handshake, stop := context.WithTimeout(ctx, handshakeTimeout)
				conn.SetDeadline(time.Now().Add(handshakeTimeout))
				err := secure.HandshakeContext(handshake)
				stop()
				if err != nil {
					conn.Close()
					return
				}
				conn.SetDeadline(time.Time{})
			}
			s.serveConnection(ctx, conn)
		}()
	}
}

func (s *Server) serveConnection(parent context.Context, conn net.Conn) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	defer conn.Close()

	// Cancellation must interrupt a blocked network write as well as the idle
	// select loop, so bootstrap can join workers within its shutdown deadline.
	stopClose := context.AfterFunc(ctx, func() { conn.Close() })
	defer stopClose()

	id := ""
	defer func() {
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		s.app.DropSession(cleanup, id)
	}()

	input := make(chan *protocol.Packet)
	done := make(chan struct{})
	readerDone := make(chan struct{})
	defer func() {
		close(done)
		conn.Close()
		<-readerDone
	}()

	go func() {
		defer close(input)
		defer close(readerDone)
		defer cancel()

		for {
			conn.SetReadDeadline(time.Now().Add(3 * time.Minute))
			p, err := protocol.Decode(conn)
			if err != nil {
				return
			}

			select {
			case input <- p:
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	write := func(operation context.Context, p *protocol.Packet) error {
		finish := s.app.BeginWrite(id)
		defer finish()

		if !s.app.AllowPacket(operation, id, p) {
			return nil
		}

		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		err := protocol.Encode(conn, p)

		completion, stop := context.WithTimeout(context.WithoutCancel(operation), 5*time.Second)
		s.app.FinishWrite(completion, []*protocol.Packet{p}, err == nil)
		stop()
		return err
	}

	respond := func(p *protocol.Packet) error {
		operation, stop := context.WithTimeout(ctx, 30*time.Second)
		defer stop()
		finishResponse := s.app.BeginResponse(id)
		defer finishResponse()

		var replies []*protocol.Packet
		replies, id = s.app.Handle(operation, id, p)
		for index, reply := range replies {
			if err := write(operation, reply); err != nil {
				if remaining := replies[index+1:]; len(remaining) > 0 {
					completion, cancel := context.WithTimeout(context.WithoutCancel(operation), 5*time.Second)
					s.app.FinishWrite(completion, remaining, false)
					cancel()
				}

				return err
			}
		}

		return nil
	}

	push := func() error {
		operation, stop := context.WithTimeout(ctx, 10*time.Second)
		defer stop()
		finishResponse, available := s.app.TryBeginResponse(id)
		if !available {
			return nil
		}
		defer finishResponse()

		packets := s.app.Poll(operation, id, 32)
		for index, packet := range packets {
			if err := write(operation, packet); err != nil {
				if remaining := packets[index+1:]; len(remaining) > 0 {
					completion, cancel := context.WithTimeout(context.WithoutCancel(operation), 5*time.Second)
					s.app.FinishWrite(completion, remaining, false)
					cancel()
				}

				return err
			}
		}

		return nil
	}

	// Receive from the queue only while holding responseGate. Selecting the
	// queue first could pop an old event, let a same-SID HTTP response overtake
	// it, then deliver the stale event afterward. A short idle poll avoids
	// holding the gate while waiting; requests and cancellation stay immediate.
	pushTick := time.NewTicker(20 * time.Millisecond)
	defer pushTick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case p, ok := <-input:
			if !ok {
				return
			}
			if respond(p) != nil {
				return
			}
		case <-pushTick.C:
			if push() != nil {
				return
			}
		}
	}
}
