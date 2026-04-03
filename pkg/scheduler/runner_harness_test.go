package scheduler

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func startSchedulerRunner(t *testing.T, runner *Runner) func() {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		runner.Run(ctx)
		close(done)
	}()

	stop := sync.OnceFunc(func() {
		t.Helper()
		stopRunner(t, runner, cancel, done)
	})
	t.Cleanup(stop)
	return stop
}

func waitForSchedulerActivity(t *testing.T, runner *Runner, timeout time.Duration, ready func(ActivitySnapshot) bool) ActivitySnapshot {
	t.Helper()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		activity := runner.ActivitySnapshot()
		if ready(activity) {
			return activity
		}
		select {
		case <-deadline.C:
			t.Fatal("timed out waiting for scheduler activity")
		case <-ticker.C:
		}
	}
}

func waitForSchedulerSnapshot(t *testing.T, runner *Runner, timeout time.Duration, ready func(Snapshot) bool) Snapshot {
	t.Helper()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		snapshot := runner.Snapshot()
		if ready(snapshot) {
			return snapshot
		}
		select {
		case <-deadline.C:
			t.Fatal("timed out waiting for scheduler snapshot")
		case <-ticker.C:
		}
	}
}

func waitForFileContent(t *testing.T, path string, timeout time.Duration, want string) []byte {
	t.Helper()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		body, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(body), want) {
			return body
		}
		select {
		case <-deadline.C:
			if err != nil {
				t.Fatalf("timed out waiting for %s: %v", path, err)
			}
			t.Fatalf("timed out waiting for %s to contain %q; got %q", path, want, string(body))
		case <-ticker.C:
		}
	}
}

func activityHasDownloadActive(activity ActivitySnapshot, name string) bool {
	for _, item := range activity.DownloadActive {
		if item.Name == name {
			return true
		}
	}
	return false
}
