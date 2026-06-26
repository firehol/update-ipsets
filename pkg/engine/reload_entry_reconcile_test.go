package engine

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestReloadContextDoesNotHoldEngineMutexDuringEntryReconcile(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	writeRuntimeReloadCriticalConfig(t, cfgPath, root)

	eng, err := New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	entry := eng.state.Entry("critical_reference")
	entry.Name = "critical_reference"

	entered := make(chan struct{})
	var checked atomic.Bool
	var mutexAvailable atomic.Bool
	restore := setCurrentSetStatsBeforeOpenHookForTest(func(name string) {
		if name != "critical_reference" || !checked.CompareAndSwap(false, true) {
			return
		}
		if eng.mu.TryLock() {
			mutexAvailable.Store(true)
			eng.mu.Unlock()
		}
		close(entered)
	})
	t.Cleanup(restore)

	done := make(chan error, 1)
	go func() {
		done <- eng.ReloadContext(t.Context())
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("reload did not reach entry reconciliation stats scan")
	}
	if !mutexAvailable.Load() {
		t.Fatal("reload held engine mutex while reconciling entry stats from disk")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	waitForEngineLaneIdle(t, eng)
}

func writeRuntimeReloadCriticalConfig(t *testing.T, path, root string) {
	t.Helper()
	cfg := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  tmp_dir: %q
  ipsets_apply: false
  max_engine_lane_workers: 1
sources:
  critical_reference:
    static:
      - 1.1.1.1
    frequency: 0
    ipv: ipv4
    output: netset
    processor: [passthrough]
    category: provider_infrastructure
    info: Critical static test reference feed.
    maintainer: test
    maintainer_url: https://example.test
    use: [critical_infrastructure]
    critical:
      tier: hard
      role: public_dns_core
      source_type: curated_static
      source_quality: C
      rationale: test critical provider
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), filepath.Join(root, "tmp"))
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
}
