package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRunOncePanicDoesNotWedgeRunningFlag(t *testing.T) {
	eng := newEngineFixture(t)
	restore := setRunOnceAfterStartHookForTest(func() {
		panic("forced admitted run panic")
	})

	_, err := eng.RunOnce(context.Background(), RunOptions{CleanupOld: true, AsyncCachePersistence: true})
	restore()
	if !errors.Is(err, ErrLanePanic) {
		t.Fatalf("RunOnce panic error = %v, want ErrLanePanic", err)
	}

	status := eng.StatusSnapshotLight()
	if status.Running || status.RunState != RunStateIdle {
		t.Fatalf("status after recovered run panic running=%v run_state=%q, want idle", status.Running, status.RunState)
	}

	_, secondErr := eng.RunOnce(context.Background(), RunOptions{CleanupOld: true, AsyncCachePersistence: true})
	if secondErr != nil && strings.Contains(secondErr.Error(), "run already in progress") {
		t.Fatalf("second RunOnce was rejected by wedged running flag: %v", secondErr)
	}
}

func TestRunOnceFinalizingPanicDoesNotWedgeRunningFlag(t *testing.T) {
	eng := newEngineFixture(t)
	restore := setRunOnceBeforeMarkFinalizingHookForTest(func() {
		panic("forced finalizing panic")
	})

	_, err := eng.RunOnce(context.Background(), RunOptions{CleanupOld: true, AsyncCachePersistence: true})
	restore()
	if !errors.Is(err, ErrLanePanic) {
		t.Fatalf("RunOnce finalizing panic error = %v, want ErrLanePanic", err)
	}

	status := eng.StatusSnapshotLight()
	if status.Running || status.RunState != RunStateIdle {
		t.Fatalf("status after recovered finalizing panic running=%v run_state=%q, want idle", status.Running, status.RunState)
	}

	_, secondErr := eng.RunOnce(context.Background(), RunOptions{CleanupOld: true, AsyncCachePersistence: true})
	if secondErr != nil && strings.Contains(secondErr.Error(), "run already in progress") {
		t.Fatalf("second RunOnce was rejected by wedged running flag: %v", secondErr)
	}
}
