package bootstrap

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestReadinessBeforeWorkersAndReverseStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var starts, stops []string
	ready := make(chan struct{}, 2)
	workerFinished := make(chan struct{}, 2)
	component := func(name string) Component {
		return Component{
			Name:  name,
			Start: func(context.Context) error { starts = append(starts, name); return nil },
			Run: func(ctx context.Context) error {
				if len(starts) != 2 {
					t.Error("worker ran before every component was ready")
				}
				ready <- struct{}{}
				<-ctx.Done()
				workerFinished <- struct{}{}
				return ctx.Err()
			},
			Stop: func(ctx context.Context) error {
				if ctx.Err() != nil {
					t.Error("shutdown inherited canceled worker context")
				}
				stops = append(stops, name)
				return nil
			},
		}
	}
	done := make(chan error, 1)
	go func() { done <- Run(ctx, time.Second, testLogger(), component("first"), component("second")) }()
	for range 2 {
		select {
		case <-ready:
		case <-time.After(time.Second):
			t.Fatal("readiness timed out")
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown timed out")
	}
	if !reflect.DeepEqual(starts, []string{"first", "second"}) || !reflect.DeepEqual(stops, []string{"second", "first"}) {
		t.Fatalf("ordering: starts=%v stops=%v", starts, stops)
	}
	if len(workerFinished) != 2 {
		t.Fatal("returned before workers finished")
	}
}

func TestStartupFailureRollsBackPartialAcquisition(t *testing.T) {
	failure, cleanup := errors.New("startup failed"), errors.New("cleanup failed")
	var order []string
	ran := false
	err := Run(context.Background(), time.Second, testLogger(),
		Component{
			Name:  "first",
			Start: func(context.Context) error { order = append(order, "start first"); return nil },
			Run:   func(context.Context) error { ran = true; return nil },
			Stop:  func(context.Context) error { order = append(order, "stop first"); return cleanup },
		},
		Component{
			Name:  "partial",
			Start: func(context.Context) error { order = append(order, "start partial"); return failure },
			Stop:  func(context.Context) error { order = append(order, "stop partial"); return nil },
		},
		Component{Name: "unselected", Start: func(context.Context) error { t.Error("later component started"); return nil }})
	if !errors.Is(err, failure) || !errors.Is(err, cleanup) {
		t.Fatalf("failure lost: %v", err)
	}
	if ran {
		t.Fatal("worker launched before startup completed")
	}
	if !reflect.DeepEqual(order, []string{"start first", "start partial", "stop partial", "stop first"}) {
		t.Fatal(order)
	}
}

func TestWorkerFailureCancelsPeerAndJoinsEveryError(t *testing.T) {
	first, second, cleanup := errors.New("first failure"), errors.New("second failure"), errors.New("cleanup failure")
	ready := make(chan struct{})
	err := Run(context.Background(), time.Second, testLogger(),
		Component{
			Name: "first",
			Run:  func(context.Context) error { <-ready; return first },
			Stop: func(context.Context) error { return cleanup },
		},
		Component{Name: "second", Run: func(ctx context.Context) error { close(ready); <-ctx.Done(); return second }})
	for _, want := range []error{first, second, cleanup} {
		if !errors.Is(err, want) {
			t.Fatalf("missing %v in %v", want, err)
		}
	}
}

func TestUnexpectedWorkerExitStopsPeers(t *testing.T) {
	err := Run(context.Background(), time.Second, testLogger(), Component{Name: "early", Run: func(context.Context) error { return nil }})
	if err == nil || !strings.Contains(err.Error(), "stopped unexpectedly") {
		t.Fatalf("early exit: %v", err)
	}
}

func TestCancellationDoesNotHideJoinedWorkerFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	want := errors.New("independent worker failure")
	err := Run(ctx, time.Second, testLogger(), Component{Name: "worker", Run: func(context.Context) error {
		cancel()
		return errors.Join(context.Canceled, want)
	}})
	if !errors.Is(err, want) {
		t.Fatalf("cancellation hid independent failure: %v", err)
	}
}

func TestCanceledStartupDoesNotAcquireResources(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Run(ctx, time.Second, testLogger(), Component{Name: "unused", Start: func(context.Context) error { t.Fatal("started after cancellation"); return nil }})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestStopBudgetBoundsUncooperativeHook(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	running, stopEntered, release, stopExited := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, 25*time.Millisecond, testLogger(), Component{
			Name: "blocked",
			Run:  func(ctx context.Context) error { close(running); <-ctx.Done(); return nil },
			Stop: func(ctx context.Context) error {
				if ctx.Err() != nil {
					t.Error("shutdown did not receive fresh context")
				}
				close(stopEntered)
				<-release
				close(stopExited)
				return nil
			},
		})
	}()
	<-running
	cancel()
	<-stopEntered
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("missing deadline: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("unbounded Stop")
	}
	close(release)
	<-stopExited
}

func TestWorkersAreJoinedWithinShutdownBudget(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	running, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, 25*time.Millisecond, testLogger(), Component{Name: "stubborn", Run: func(context.Context) error { close(running); <-release; close(finished); return nil }})
	}()
	<-running
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("missing deadline: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("unbounded worker join")
	}
	close(release)
	<-finished
}

func TestInvalidComponentsDoNotStart(t *testing.T) {
	valid := Component{Name: "valid", Start: func(context.Context) error { t.Error("invalid graph started"); return nil }}
	for _, test := range []struct {
		name       string
		timeout    time.Duration
		components []Component
	}{
		{"none", time.Second, nil},
		{"timeout", 0, []Component{valid}},
		{"empty name", time.Second, []Component{{Run: func(context.Context) error { return nil }}}},
		{"duplicate", time.Second, []Component{valid, valid}},
		{"no hooks", time.Second, []Component{{Name: "empty"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := Run(context.Background(), test.timeout, testLogger(), test.components...); err == nil {
				t.Fatal("invalid components accepted")
			}
		})
	}
}
