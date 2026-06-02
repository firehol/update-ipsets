package scheduler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
)

// TestContextCancellationStopsRun verifies that cancelling the context
// passed to Runner.Run causes it to return promptly, stopping any
// in-progress wait for the next scheduled run.
func TestContextCancellationStopsRun(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer sourceServer.Close()

	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	cfg := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  ipsets_apply: false
sources:
  cancel_test:
    url: %q
    frequency: 9999
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: cancel test feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"),
		filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"),
		sourceServer.URL)

	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Run the engine once so the scheduler has something to schedule.
	if _, err := runSchedulerStyleOnce(t, eng, engine.RunOptions{
		EnableAll: true, Manual: true, CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}

	runner := New(eng, true, nil)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		runner.Run(ctx)
		close(done)
	}()

	stopRunner(t, runner, cancel, done)
}

// TestTriggerWakesUpWaitingRunner verifies that Trigger() wakes a runner
// that is waiting for the next scheduled time.
func TestTriggerWakesUpWaitingRunner(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("2.3.4.5\n"))
	}))
	defer sourceServer.Close()

	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	cfg := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  ipsets_apply: false
sources:
  trigger_test:
    url: %q
    frequency: 9999
    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: trigger test feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"),
		filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"),
		sourceServer.URL)

	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runSchedulerStyleOnce(t, eng, engine.RunOptions{
		EnableAll: true, Manual: true, CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}

	runner := New(eng, true, nil)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		runner.Run(ctx)
		close(done)
	}()
	defer stopRunner(t, runner, cancel, done)

	// Trigger should succeed even if the runner has not yet reached
	// its wait loop; the queued trigger is the public contract.
	if !runner.Trigger() {
		t.Fatal("expected Trigger() to return true")
	}

	// Second trigger while the first hasn't been consumed: may return false.
	// This is expected behavior — the channel has capacity 1.

	waitForSchedulerSnapshot(t, runner, 2*time.Second, func(snapshot Snapshot) bool {
		return len(snapshot.Items) > 0
	})
}

func stopRunner(t *testing.T, runner *Runner, cancel context.CancelFunc, done <-chan struct{}) {
	t.Helper()

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for scheduler.Run to stop")
	}
	assertRunnerHasNoActiveWork(t, runner)
}

func assertRunnerHasNoActiveWork(t *testing.T, runner *Runner) {
	t.Helper()
	activity := runner.ActivitySnapshot()
	if len(activity.DownloadActive) != 0 || len(activity.ProcessingActive) != 0 {
		t.Fatalf("runner stopped with active work: downloads=%#v processing=%#v", activity.DownloadActive, activity.ProcessingActive)
	}
}
