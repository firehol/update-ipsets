package engine

import (
	"context"
	"errors"
	"testing"
)

func TestBackgroundLimiterAcquireContextCancelledWhileWaiting(t *testing.T) {
	limiter := newBackgroundLimiter(1)
	if err := limiter.AcquireContext(t.Context()); err != nil {
		t.Fatalf("AcquireContext() initial error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := limiter.AcquireContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("AcquireContext() waiting error = %v, want context.Canceled", err)
	}
	limit, running := limiter.Snapshot()
	if limit != 1 || running != 1 {
		t.Fatalf("Snapshot() = limit %d running %d, want 1/1", limit, running)
	}

	limiter.Release()
	_, running = limiter.Snapshot()
	if running != 0 {
		t.Fatalf("running after release = %d, want 0", running)
	}
}

func TestWithBackgroundTaskCancelledWaitDoesNotLeakTaskOrWorker(t *testing.T) {
	eng := newEngineFixture(t)
	eng.backgroundLimiter = newBackgroundLimiter(1)
	if err := eng.backgroundLimiter.AcquireContext(t.Context()); err != nil {
		t.Fatalf("AcquireContext() initial error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := eng.withBackgroundTask(ctx, "Entity artifacts refresh", "test", "running", "test", 0, 0, func(*BackgroundTaskHandle) error {
		t.Fatal("background task body should not run after cancelled wait")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("withBackgroundTask() error = %v, want context.Canceled", err)
	}
	if tasks := eng.snapshotBackgroundTasksLocked(); len(tasks) != 0 {
		t.Fatalf("background tasks leaked after cancelled wait: %v", tasks)
	}
	_, running := eng.backgroundLimiter.Snapshot()
	if running != 1 {
		t.Fatalf("running workers = %d, want 1", running)
	}
	eng.backgroundLimiter.Release()
}
