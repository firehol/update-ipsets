package engine

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestExpandTemplateShellDefaults(t *testing.T) {
	t.Setenv("HOME", "/tmp/home")
	got := expandTemplate("${BASE_DIR-${HOME}/ipsets}", map[string]string{
		"HOME": "/tmp/home",
	}, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	if got != "/tmp/home/ipsets" {
		t.Fatalf("unexpected expansion: got %q", got)
	}

	got = expandTemplate("${TMP_DIR-/tmp}", map[string]string{
		"HOME": os.Getenv("HOME"),
	}, time.Now())
	if got != "/tmp" {
		t.Fatalf("unexpected fallback expansion: got %q", got)
	}
}

func TestResolveRuntimeDisablesKernelApplyForUserMode(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("user-mode defaults are only meaningful for non-root test runs")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.New()
	rt, err := resolveRuntime(cfg, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("resolveRuntime returned error: %v", err)
	}
	if rt.IPSetsApply {
		t.Fatal("expected kernel ipsets to be disabled in user mode")
	}
	if rt.BaseDir != filepath.Join(home, ".update-ipsets", "ipsets") {
		t.Fatalf("unexpected user-mode base dir: %q", rt.BaseDir)
	}
}

func TestResolveRuntimeUsesNewQueueDefaults(t *testing.T) {
	cfg := config.New()
	rt, err := resolveRuntime(cfg, time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("resolveRuntime returned error: %v", err)
	}
	if rt.MaxIngestWorkers != 0 {
		t.Fatalf("expected max ingest workers default 0, got %d", rt.MaxIngestWorkers)
	}
	if rt.ParallelDownloads != 5 {
		t.Fatalf("expected parallel downloads default 5, got %d", rt.ParallelDownloads)
	}
	if rt.ProcessingIntervalMinutes != 5 {
		t.Fatalf("expected processing interval default 5 minutes, got %d", rt.ProcessingIntervalMinutes)
	}
}

func TestResolveRuntimeCarriesWebArtifactCacheControls(t *testing.T) {
	cfg := config.New()
	cfg.Runtime.WebArtifactCacheMaxEntries = 12
	cfg.Runtime.WebArtifactCacheMaxBytes = 34
	cfg.Runtime.WebArtifactCacheMaxFileBytes = 5

	rt, err := resolveRuntime(cfg, time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("resolveRuntime returned error: %v", err)
	}
	if got, want := rt.WebArtifactCacheMaxEntries, 12; got != want {
		t.Fatalf("web artifact cache max entries = %d, want %d", got, want)
	}
	if got, want := rt.WebArtifactCacheMaxBytes, int64(34); got != want {
		t.Fatalf("web artifact cache max bytes = %d, want %d", got, want)
	}
	if got, want := rt.WebArtifactCacheMaxFileBytes, int64(5); got != want {
		t.Fatalf("web artifact cache max file bytes = %d, want %d", got, want)
	}
}

func TestResolveRuntimeSeparatesHeavyPhaseWorkers(t *testing.T) {
	cfg := config.New()
	cfg.Runtime.MaxProcessingWorkers = 2

	rt, err := resolveRuntime(cfg, time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("resolveRuntime returned error: %v", err)
	}
	if rt.MaxProcessingWorkers != 2 {
		t.Fatalf("expected processing workers to remain 2, got %d", rt.MaxProcessingWorkers)
	}
	if rt.HeavyPhaseWorkers() < rt.MaxProcessingWorkers {
		t.Fatalf("expected heavy-phase workers >= processing workers, got heavy=%d processing=%d", rt.HeavyPhaseWorkers(), rt.MaxProcessingWorkers)
	}
	if rt.HeavyPhaseWorkers() > 8 {
		t.Fatalf("expected automatic heavy-phase workers to be capped at 8, got %d", rt.HeavyPhaseWorkers())
	}

	cfg.Runtime.MaxHeavyPhaseWorkers = 5
	rt, err = resolveRuntime(cfg, time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("resolveRuntime returned error with explicit heavy workers: %v", err)
	}
	if got, want := rt.HeavyPhaseWorkers(), 5; got != want {
		t.Fatalf("expected explicit heavy-phase workers %d, got %d", want, got)
	}
}

func TestResolveRuntimeDefaultsBackgroundWorkersToOne(t *testing.T) {
	cfg := config.New()

	rt, err := resolveRuntime(cfg, time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("resolveRuntime returned error: %v", err)
	}
	if got, want := rt.BackgroundWorkers(), 1; got != want {
		t.Fatalf("expected background workers %d, got %d", want, got)
	}

	cfg.Runtime.MaxBackgroundWorkers = 3
	rt, err = resolveRuntime(cfg, time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("resolveRuntime returned error with explicit background workers: %v", err)
	}
	if got, want := rt.BackgroundWorkers(), 3; got != want {
		t.Fatalf("expected explicit background workers %d, got %d", want, got)
	}
}

func TestResolveRuntimeAppliesIngestWorkerCeiling(t *testing.T) {
	cfg := config.New()
	cfg.Runtime.MaxIngestWorkers = 2
	cfg.Runtime.ParallelDownloads = 7
	cfg.Runtime.ParallelDNSQueries = 9
	cfg.Runtime.MaxProcessingWorkers = 4
	cfg.Runtime.MaxHeavyPhaseWorkers = 6
	cfg.Runtime.MaxBackgroundWorkers = 3

	rt, err := resolveRuntime(cfg, time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("resolveRuntime returned error: %v", err)
	}
	if got, want := rt.ParallelDownloads, 2; got != want {
		t.Fatalf("parallel downloads = %d, want %d", got, want)
	}
	if got, want := rt.ParallelDNSQueries, 2; got != want {
		t.Fatalf("parallel DNS queries = %d, want %d", got, want)
	}
	if got, want := rt.MaxProcessingWorkers, 2; got != want {
		t.Fatalf("processing workers = %d, want %d", got, want)
	}
	if got, want := rt.HeavyPhaseWorkers(), 2; got != want {
		t.Fatalf("heavy phase workers = %d, want %d", got, want)
	}
	if got, want := rt.BackgroundWorkers(), 2; got != want {
		t.Fatalf("background workers = %d, want %d", got, want)
	}
}

func TestResolveRuntimeAllowsUnlimitedIngestWorkerCeiling(t *testing.T) {
	cfg := config.New()
	cfg.Runtime.MaxIngestWorkers = 0
	cfg.Runtime.ParallelDownloads = 7
	cfg.Runtime.ParallelDNSQueries = 9
	cfg.Runtime.MaxProcessingWorkers = 4
	cfg.Runtime.MaxHeavyPhaseWorkers = 6
	cfg.Runtime.MaxBackgroundWorkers = 3

	rt, err := resolveRuntime(cfg, time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("resolveRuntime returned error: %v", err)
	}
	if got, want := rt.ParallelDownloads, 7; got != want {
		t.Fatalf("parallel downloads = %d, want %d", got, want)
	}
	if got, want := rt.ParallelDNSQueries, 9; got != want {
		t.Fatalf("parallel DNS queries = %d, want %d", got, want)
	}
	if got, want := rt.MaxProcessingWorkers, 4; got != want {
		t.Fatalf("processing workers = %d, want %d", got, want)
	}
	if got, want := rt.HeavyPhaseWorkers(), 6; got != want {
		t.Fatalf("heavy phase workers = %d, want %d", got, want)
	}
	if got, want := rt.BackgroundWorkers(), 3; got != want {
		t.Fatalf("background workers = %d, want %d", got, want)
	}
}

func TestReloadAppliesChangedIngestWorkerCeiling(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	writeRuntimeReloadConfig(t, cfgPath, root, 2)

	eng, err := New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := eng.Runtime().MaxIngestWorkers, 2; got != want {
		t.Fatalf("initial max ingest workers = %d, want %d", got, want)
	}
	if got, want := eng.Runtime().ParallelDownloads, 2; got != want {
		t.Fatalf("initial parallel downloads = %d, want %d", got, want)
	}

	writeRuntimeReloadConfig(t, cfgPath, root, 1)
	if err := eng.Reload(); err != nil {
		t.Fatal(err)
	}
	snap := eng.StatusSnapshot()
	if got, want := snap.MaxIngestWorkers, 1; got != want {
		t.Fatalf("reloaded max ingest workers = %d, want %d", got, want)
	}
	if got, want := snap.ParallelDownloads, 1; got != want {
		t.Fatalf("reloaded parallel downloads = %d, want %d", got, want)
	}
	if got, want := snap.ParallelDNSQueries, 1; got != want {
		t.Fatalf("reloaded parallel DNS queries = %d, want %d", got, want)
	}
	if got, want := snap.MaxProcessingWorkers, 1; got != want {
		t.Fatalf("reloaded processing workers = %d, want %d", got, want)
	}
	if got, want := snap.MaxHeavyPhaseWorkers, 1; got != want {
		t.Fatalf("reloaded heavy-phase workers = %d, want %d", got, want)
	}
	if got, want := snap.MaxBackgroundWorkers, 1; got != want {
		t.Fatalf("reloaded background workers = %d, want %d", got, want)
	}
	if got, want := snap.BackgroundLimit, 1; got != want {
		t.Fatalf("reloaded background limiter = %d, want %d", got, want)
	}
}

func TestStatusSnapshotReportsEffectiveRuntimeWorkers(t *testing.T) {
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.MaxIngestWorkers = 2
		rt.ParallelDownloads = 2
		rt.ParallelDNSQueries = 2
		rt.MaxProcessingWorkers = 2
		rt.MaxHeavyPhaseWorkers = 2
		rt.MaxBackgroundWorkers = 2
	}))

	snap := eng.StatusSnapshot()
	if got, want := snap.MaxIngestWorkers, 2; got != want {
		t.Fatalf("status max ingest workers = %d, want %d", got, want)
	}
	if got, want := snap.ParallelDownloads, 2; got != want {
		t.Fatalf("status parallel downloads = %d, want %d", got, want)
	}
	if got, want := snap.ParallelDNSQueries, 2; got != want {
		t.Fatalf("status parallel DNS queries = %d, want %d", got, want)
	}
	if got, want := snap.MaxProcessingWorkers, 2; got != want {
		t.Fatalf("status processing workers = %d, want %d", got, want)
	}
	if got, want := snap.MaxHeavyPhaseWorkers, 2; got != want {
		t.Fatalf("status heavy phase workers = %d, want %d", got, want)
	}
	if got, want := snap.MaxBackgroundWorkers, 2; got != want {
		t.Fatalf("status background workers = %d, want %d", got, want)
	}
}

func writeRuntimeReloadConfig(t *testing.T, path, root string, ceiling int) {
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
  max_ingest_workers: %d
  parallel_downloads: 7
  parallel_dns_queries: 9
  max_processing_workers: 4
  max_heavy_phase_workers: 6
  max_background_workers: 3
sources:
  sample:
    static:
      - 10.0.0.1
    frequency: 0
    ipv: ipv4
    output: netset
    processor:
      - passthrough
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), filepath.Join(root, "tmp"), ceiling)
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRuntimeOverridesAlignsEngineRuntime(t *testing.T) {
	root := t.TempDir()
	cfg := config.New()
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = filepath.Join(root, "base")
		rt.WebDir = filepath.Join(root, "configured-web")
		rt.WebDirForIPSets = filepath.Join(root, "configured-files")
	}))
	overrideWeb := filepath.Join(root, "served-web")
	overrideFiles := filepath.Join(root, "served-files")

	if err := eng.ApplyRuntimeOverrides(overrideWeb, overrideFiles); err != nil {
		t.Fatal(err)
	}
	if got := eng.Runtime().WebDir; got != overrideWeb {
		t.Fatalf("runtime web dir = %q, want %q", got, overrideWeb)
	}
	if got := eng.Runtime().WebDirForIPSets; got != overrideFiles {
		t.Fatalf("runtime files dir = %q, want %q", got, overrideFiles)
	}
	if cfg.Runtime.WebDir != overrideWeb {
		t.Fatalf("config runtime web dir = %q, want %q", cfg.Runtime.WebDir, overrideWeb)
	}
	if _, err := os.Stat(overrideWeb); err != nil {
		t.Fatalf("override web dir was not created: %v", err)
	}
	if _, err := os.Stat(overrideFiles); err != nil {
		t.Fatalf("override files dir was not created: %v", err)
	}
}
