package scheduler

import (
	"testing"
	"time"
)

func TestActivitySnapshotLightDoesNotWaitForSchedulerStateLock(t *testing.T) {
	runner := &Runner{}
	runner.stateMu.Lock()
	defer runner.stateMu.Unlock()

	done := make(chan ActivitySnapshot, 1)
	go func() {
		done <- runner.ActivitySnapshotLight()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ActivitySnapshotLight waited for scheduler state lock")
	}
}

func TestTryCachedSnapshotDoesNotWaitForSchedulerSnapshotLock(t *testing.T) {
	runner := &Runner{}
	runner.mu.Lock()
	defer runner.mu.Unlock()

	done := make(chan bool, 1)
	go func() {
		_, ok := runner.TryCachedSnapshot()
		done <- ok
	}()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("TryCachedSnapshot returned ok=true while scheduler snapshot lock was held")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("TryCachedSnapshot waited for scheduler snapshot lock")
	}
}
