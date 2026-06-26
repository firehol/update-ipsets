package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRunOnceFinalPublicationUsesRunContext(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
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
  sample:
    url: https://example.test/list.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "sample.ipset.new"), []byte("2.2.2.2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	report, err := eng.RunOnce(ctx, RunOptions{
		Selected:              []string{"sample"},
		EnableAll:             true,
		Manual:                true,
		CleanupOld:            true,
		AsyncCachePersistence: true,
		BeforePublish: func(report *Report) error {
			cancel()
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunOnce error = %v, want context.Canceled", err)
	}
	if report == nil || len(report.Updated) != 1 || report.Updated[0] != "sample" {
		t.Fatalf("expected processed report before canceled publication, got %#v", report)
	}
	status := eng.StatusSnapshotLight()
	if status.Running || status.RunState != RunStateIdle {
		t.Fatalf("run state after canceled finalization = running:%v state:%q, want idle", status.Running, status.RunState)
	}
	if _, err := os.Stat(filepath.Join(webDir, "index.json")); !os.IsNotExist(err) {
		t.Fatalf("canceled final publication created index.json, stat err=%v", err)
	}
	stopCachePersistenceForTest(t, eng)
}
