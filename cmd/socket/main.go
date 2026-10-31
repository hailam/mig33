package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"

	"github.com/hailam/mig33/cmd/internal/command"
	"github.com/hailam/mig33/cmd/internal/wiring"
	"github.com/hailam/mig33/internal/adapters/socket"
	"github.com/hailam/mig33/internal/bootstrap"
)

func main() { command.Main(run) }

func run(ctx context.Context, arguments []string, _ io.Writer, stderr io.Writer) (resultErr error) {
	flags := command.NewFlags("socket", stderr)
	flags.Socket()
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	cfg, err := flags.Load()
	if err != nil {
		return err
	}
	var secure *tls.Config
	if cfg.Socket.TLSListen != "" {
		secure, err = wiring.LoadTLS(cfg.TLS)
		if err != nil {
			return err
		}
	}
	logger := slog.New(slog.NewTextHandler(stderr, nil))
	graph, cleanup, err := wiring.Open(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, cleanup()) }()
	socketServer, err := socket.NewListener(socket.New(graph.Native), cfg.Socket.Listen, logger)
	if err != nil {
		return err
	}
	components := []bootstrap.Component{
		{Name: "application", Run: graph.Core.Run},
		{Name: "Socket", Start: socketServer.Start, Run: socketServer.Run, Stop: socketServer.Stop},
	}
	if cfg.Socket.TLSListen != "" {
		tlsServer, err := socket.NewTLSListener(socket.New(graph.Native), cfg.Socket.TLSListen, secure, cfg.TLS.HandshakeTimeout, logger)
		if err != nil {
			return err
		}
		components = append(components, bootstrap.Component{Name: "TLS socket", Start: tlsServer.Start, Run: tlsServer.Run, Stop: tlsServer.Stop})
	}
	return bootstrap.Run(ctx, cfg.ShutdownTimeout, logger, components...)
}
