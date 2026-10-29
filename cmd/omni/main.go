package main

import (
	"context"
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
	return bootstrap.Run(ctx, cfg.ShutdownTimeout, logger,
		bootstrap.Component{Name: "application", Run: graph.Core.Run},
		bootstrap.Component{Name: "HTTP", Start: httpServer.Start, Run: httpServer.Run, Stop: httpServer.Stop},
		bootstrap.Component{Name: "Socket", Start: socketServer.Start, Run: socketServer.Run, Stop: socketServer.Stop},
	)
}
