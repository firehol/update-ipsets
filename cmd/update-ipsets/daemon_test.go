package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"slices"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
)

type daemonContextKey string

func TestCompactCLIArgsDropsEmptyEntries(t *testing.T) {
	input := []string{
		"--config", "/opt/update-ipsets/etc/config",
		"--listen", ":18888",
		"",
		"--admin-auth-mode=disabled",
		"--allow-unauthenticated-admin",
	}

	got := compactCLIArgs(append([]string(nil), input...))
	want := []string{
		"--config", "/opt/update-ipsets/etc/config",
		"--listen", ":18888",
		"--admin-auth-mode=disabled",
		"--allow-unauthenticated-admin",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("compactCLIArgs() = %#v, want %#v", got, want)
	}
}

func TestReloadSignalLoopContinuesAfterReloadPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.WithValue(t.Context(), daemonContextKey("reload"), "daemon-context"))
	defer cancel()
	hup := make(chan os.Signal, 2)
	fake := &panicOnceReloadEngine{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	done := make(chan struct{})
	go func() {
		defer close(done)
		runReloadSignalLoop(ctx, logger, hup, fake)
	}()

	hup <- syscall.SIGHUP
	waitForCondition(t, func() bool { return fake.reloads.Load() >= 1 }, "first reload")
	hup <- syscall.SIGHUP
	waitForCondition(t, func() bool {
		return fake.reloads.Load() >= 2 && fake.queues.Load() == 1 && fake.reloadContextValues.Load() == 1 && fake.queueContextValues.Load() == 1
	}, "second reload after panic")

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("reload signal loop did not stop after context cancellation")
	}
}

type panicOnceReloadEngine struct {
	reloads             atomic.Int64
	queues              atomic.Int64
	reloadContextValues atomic.Int64
	queueContextValues  atomic.Int64
}

func (e *panicOnceReloadEngine) ReloadContext(ctx context.Context) error {
	if e.reloads.Add(1) == 1 {
		panic("reload panic")
	}
	if ctx.Value(daemonContextKey("reload")) == "daemon-context" {
		e.reloadContextValues.Add(1)
	}
	return nil
}

func (e *panicOnceReloadEngine) Runtime() engine.Runtime {
	return engine.Runtime{ConfigPath: "test-config.yaml"}
}

func (e *panicOnceReloadEngine) QueueEntityArtifactsEnsure(ctx context.Context, _ string) (engine.EntityArtifactQueueResult, error) {
	e.queues.Add(1)
	if ctx.Value(daemonContextKey("reload")) == "daemon-context" {
		e.queueContextValues.Add(1)
	}
	return engine.EntityArtifactQueueResult{}, nil
}

func waitForCondition(t *testing.T, fn func() bool, name string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if fn() {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s", name)
		case <-ticker.C:
		}
	}
}
