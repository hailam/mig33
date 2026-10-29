// Package bootstrap coordinates caller-supplied component lifecycles.
// It has no knowledge of configuration, transports or application dependencies.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Component is a lifecycle owned and supplied by the caller. Start establishes
// readiness synchronously; every Start completes before any Run begins. Run
// blocks until canceled or stopped. Stop releases resources and joins any work
// the component owns; it must honor its context, including an expired context.
// Stop is also called after a failed Start, allowing partial acquisition cleanup.
// Hooks may be nil, but a component must supply at least one hook.
//
// Cancellation cannot forcibly terminate Go code. A hook that ignores its
// context can outlive Run's shutdown budget; Run reports that failure.
type Component struct {
	Name  string
	Start func(context.Context) error
	Run   func(context.Context) error
	Stop  func(context.Context) error
}

type completion struct {
	name string
	err  error
}

// Run starts supplied components in order and stops them in reverse order.
// Cancellation after startup is a normal stop; cancellation before startup is
// returned to the caller. A worker returning before cancellation stops its peers.
// Worker and cleanup errors are joined, preserving the original startup failure.
func Run(ctx context.Context, shutdownTimeout time.Duration, logger *slog.Logger, components ...Component) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if shutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive")
	}
	if len(components) == 0 {
		return fmt.Errorf("at least one component is required")
	}
	names := make(map[string]bool, len(components))
	for _, c := range components {
		if strings.TrimSpace(c.Name) == "" {
			return fmt.Errorf("component name must not be empty")
		}
		if names[c.Name] {
			return fmt.Errorf("duplicate component name %q", c.Name)
		}
		names[c.Name] = true
		if c.Start == nil && c.Run == nil && c.Stop == nil {
			return fmt.Errorf("component %q has no lifecycle hooks", c.Name)
		}
	}
	if logger == nil {
		return fmt.Errorf("logger is required")
	}
	started := 0
	for index, c := range components {
		if err := ctx.Err(); err != nil {
			return errors.Join(err, stopComponents(shutdownTimeout, logger, components[:started]))
		}
		started = index + 1
		if c.Start != nil {
			if err := c.Start(ctx); err != nil {
				return errors.Join(fmt.Errorf("%s start: %w", c.Name, err), stopComponents(shutdownTimeout, logger, components[:started]))
			}
		}
		logger.Debug("component ready", "component", c.Name)
	}
	running, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan completion, len(components))
	remaining := 0
	for _, c := range components {
		if c.Run == nil {
			continue
		}
		remaining++
		go func() { results <- completion{name: c.Name, err: c.Run(running)} }()
	}
	var runErr error
	select {
	case <-running.Done():
	case result := <-results:
		remaining--
		if result.err == nil && running.Err() == nil {
			runErr = fmt.Errorf("%s stopped unexpectedly", result.name)
		} else {
			runErr = workerError(result, running.Err() != nil)
		}
	}
	cancel()
	shutdown, stop := context.WithTimeout(context.Background(), shutdownTimeout)
	defer stop()
	runErr = errors.Join(runErr, stopWithContext(shutdown, logger, components[:started]))
	for remaining > 0 {
		select {
		case result := <-results:
			remaining--
			runErr = errors.Join(runErr, workerError(result, true))
		case <-shutdown.Done():
			// Retain every already-reported failure even when another worker is stuck.
			for remaining > 0 {
				select {
				case result := <-results:
					remaining--
					runErr = errors.Join(runErr, workerError(result, true))
				default:
					return errors.Join(runErr, fmt.Errorf("component workers shutdown: %w", shutdown.Err()))
				}
			}
		}
	}
	return runErr
}

func workerError(result completion, stopping bool) error {
	if result.err == nil {
		return nil
	}
	if stopping && (result.err == context.Canceled || result.err == context.DeadlineExceeded) {
		return nil
	}
	return fmt.Errorf("%s run: %w", result.name, result.err)
}

func stopComponents(timeout time.Duration, logger *slog.Logger, components []Component) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return stopWithContext(ctx, logger, components)
}

func stopWithContext(ctx context.Context, logger *slog.Logger, components []Component) error {
	var result error
	for index := len(components) - 1; index >= 0; index-- {
		c := components[index]
		if c.Stop == nil {
			continue
		}
		stopped := make(chan error, 1)
		go func() { stopped <- c.Stop(ctx) }()
		select {
		case err := <-stopped:
			if err != nil {
				result = errors.Join(result, fmt.Errorf("%s stop: %w", c.Name, err))
			}
		case <-ctx.Done():
			result = errors.Join(result, fmt.Errorf("%s stop: %w", c.Name, ctx.Err()))
		}
		logger.Debug("component stop requested", "component", c.Name)
	}
	return result
}
