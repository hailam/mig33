package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/hailam/mig33/cmd/internal/command"
	"github.com/hailam/mig33/cmd/internal/wiring"
	"github.com/hailam/mig33/internal/adapters/httptransport"
	"github.com/hailam/mig33/internal/adapters/socket"
	"github.com/hailam/mig33/internal/bootstrap"
	"github.com/hailam/mig33/internal/controllers"
)

func main() { command.Main(run) }

func run(ctx context.Context, arguments []string, _ io.Writer, stderr io.Writer) (resultErr error) {
	flags := command.NewFlags("omni", stderr)
	flags.HTTP()
	flags.Socket()
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	cfg, err := flags.Load()
	if err != nil {
		return err
	}
	var secure *tls.Config
	if cfg.HTTP.TLSListen != "" || cfg.Socket.TLSListen != "" {
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
	httpServer, err := httptransport.NewServer(&http.Server{
		Addr:              cfg.HTTP.Listen,
		Handler:           httptransport.New(controllers.New(graph.HTTP)),
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}, logger)
	if err != nil {
		return err
	}
	socketServer, err := socket.NewListener(socket.New(graph.Native), cfg.Socket.Listen, logger)
	if err != nil {
		return err
	}
	components := []bootstrap.Component{
		{Name: "application", Run: graph.Core.Run},
		{Name: "HTTP", Start: httpServer.Start, Run: httpServer.Run, Stop: httpServer.Stop},
		{Name: "Socket", Start: socketServer.Start, Run: socketServer.Run, Stop: socketServer.Stop},
	}
	if cfg.HTTP.TLSListen != "" {
		httpsServer, err := httptransport.NewTLSServer(&http.Server{
			Addr:              cfg.HTTP.TLSListen,
			Handler:           httptransport.NewGateway(controllers.New(graph.HTTP)),
			ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
			ReadTimeout:       cfg.HTTP.ReadTimeout,
			WriteTimeout:      cfg.HTTP.WriteTimeout,
			IdleTimeout:       cfg.HTTP.IdleTimeout,
			MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
		}, secure, cfg.TLS.HandshakeTimeout, logger)
		if err != nil {
			return err
		}
		components = append(components, bootstrap.Component{Name: "HTTPS", Start: httpsServer.Start, Run: httpsServer.Run, Stop: httpsServer.Stop})
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
