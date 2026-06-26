package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEngineLaneBackgroundTaskCancelledContextDoesNotRunOrLeakTask(t *testing.T) {
	eng := newEngineFixture(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := eng.withEngineLaneBackgroundTask(ctx, LaneWorkEntityRefresh, LaneComponentEntityArtifacts, "Entity artifacts refresh", "test", "running", "test", 0, 0, func(*BackgroundTaskHandle) error {
		t.Fatal("background task body should not run after cancelled wait")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("withEngineLaneBackgroundTask() error = %v, want context.Canceled", err)
	}
	if tasks := eng.snapshotBackgroundTasks(); len(tasks) != 0 {
		t.Fatalf("background tasks leaked after cancelled context: %v", tasks)
	}
}

func TestEngineLaneBackgroundTaskPanicFinishesTaskAndReturnsError(t *testing.T) {
	eng := newEngineFixture(t)

	err := eng.withEngineLaneBackgroundTask(t.Context(), LaneWorkEntityRefresh, LaneComponentEntityArtifacts, "Entity artifacts refresh", "test", "running", "test", 0, 0, func(*BackgroundTaskHandle) error {
		panic("forced background task panic")
	})
	if !errors.Is(err, ErrLanePanic) {
		t.Fatalf("withEngineLaneBackgroundTask() error = %v, want ErrLanePanic", err)
	}
	if !strings.Contains(err.Error(), "forced background task panic") {
		t.Fatalf("withEngineLaneBackgroundTask() error = %v, want panic detail", err)
	}
	if tasks := eng.snapshotBackgroundTasks(); len(tasks) != 0 {
		t.Fatalf("background tasks leaked after panic: %v", tasks)
	}
}
