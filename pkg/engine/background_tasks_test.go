package engine

import (
	"context"
	"errors"
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
	if tasks := eng.snapshotBackgroundTasksLocked(); len(tasks) != 0 {
		t.Fatalf("background tasks leaked after cancelled context: %v", tasks)
	}
}
