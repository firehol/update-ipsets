package engine

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/asnloc"
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
	if got, want := rt.EngineLaneWorkers(), 1; got != want {
		t.Fatalf("expected engine lane workers %d, got %d", want, got)
	}

	cfg.Runtime.MaxBackgroundWorkers = 3
	cfg.Runtime.MaxEngineLaneWorkers = 4
	rt, err = resolveRuntime(cfg, time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("resolveRuntime returned error with explicit background workers: %v", err)
	}
	if got, want := rt.BackgroundWorkers(), 3; got != want {
		t.Fatalf("expected explicit background workers %d, got %d", want, got)
	}
	if got, want := rt.EngineLaneWorkers(), 4; got != want {
		t.Fatalf("expected explicit engine lane workers %d, got %d", want, got)
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
	cfg.Runtime.MaxEngineLaneWorkers = 5

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
	if got, want := rt.EngineLaneWorkers(), 2; got != want {
		t.Fatalf("engine lane workers = %d, want %d", got, want)
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
	cfg.Runtime.MaxEngineLaneWorkers = 5

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
	if got, want := rt.EngineLaneWorkers(), 5; got != want {
		t.Fatalf("engine lane workers = %d, want %d", got, want)
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
	if got, want := snap.MaxEngineLaneWorkers, 1; got != want {
		t.Fatalf("reloaded engine lane workers = %d, want %d", got, want)
	}
	if got, want := snap.BackgroundLimit, 1; got != want {
		t.Fatalf("reloaded background limiter = %d, want %d", got, want)
	}
	if got, want := snap.EngineLane.Limit, 1; got != want {
		t.Fatalf("reloaded engine lane limit = %d, want %d", got, want)
	}
	waitForEngineLaneIdle(t, eng)
}

func TestReloadRetiresASNLookupCacheWithoutReplacingCache(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	writeRuntimeReloadConfig(t, cfgPath, root, 2)

	eng, err := New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	cache := eng.asnLookupCache
	lease, err := cache.acquire("asn", "/tmp/asn.source", 1, func() (*asnloc.Database, error) {
		return &asnloc.Database{Provider: "test"}, nil
	})
	if err != nil {
		t.Fatalf("acquire() error = %v", err)
	}
	entry := lease.entry

	writeRuntimeReloadConfig(t, cfgPath, root, 1)
	if err := eng.Reload(); err != nil {
		t.Fatal(err)
	}
	if eng.asnLookupCache != cache {
		t.Fatalf("reload replaced ASN lookup cache pointer")
	}
	if got := len(cache.dbs); got != 0 {
		t.Fatalf("ASN lookup cache retained %d entries after reload, want 0", got)
	}
	if !entry.retired {
		t.Fatalf("leased ASN lookup entry was not retired by reload")
	}
	if entry.closed {
		t.Fatalf("leased ASN lookup entry was closed before release")
	}

	lease.Close()
	if !entry.closed {
		t.Fatalf("retired ASN lookup entry was not closed after lease release")
	}
	waitForEngineLaneIdle(t, eng)
}

func TestStatusSnapshotReportsEffectiveRuntimeWorkers(t *testing.T) {
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.MaxIngestWorkers = 2
		rt.ParallelDownloads = 2
		rt.ParallelDNSQueries = 2
		rt.MaxProcessingWorkers = 2
		rt.MaxHeavyPhaseWorkers = 2
		rt.MaxBackgroundWorkers = 2
		rt.MaxEngineLaneWorkers = 2
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
	if got, want := snap.MaxEngineLaneWorkers, 2; got != want {
		t.Fatalf("status engine lane workers = %d, want %d", got, want)
	}
	if got, want := snap.EngineLane.Limit, 2; got != want {
		t.Fatalf("status engine lane limit = %d, want %d", got, want)
	}
}

func TestStatusSnapshotReportsIntegrityCacheSummaries(t *testing.T) {
	eng := newEngineFixture(t)
	eng.StorePipelineIntegrityFindings(IntegrityOptions{}, []IntegrityFinding{{Feed: "sample"}}, nil)
	eng.StoreEntityIntegrityFindings([]EntityIntegrityFinding{{Scope: "global", Kind: "version_missing", Subject: "entity_artifacts"}}, nil)

	snap := eng.StatusSnapshotLight()
	if got, want := snap.PipelineIntegrityCache.CacheState, IntegrityCacheFresh; got != want {
		t.Fatalf("pipeline integrity cache state = %q, want %q", got, want)
	}
	if got, want := snap.PipelineIntegrityCache.Count, 1; got != want {
		t.Fatalf("pipeline integrity cache count = %d, want %d", got, want)
	}
	if got, want := snap.EntityIntegrityCache.CacheState, IntegrityCacheFresh; got != want {
		t.Fatalf("entity integrity cache state = %q, want %q", got, want)
	}
	if got, want := snap.EntityIntegrityCache.Count, 1; got != want {
		t.Fatalf("entity integrity cache count = %d, want %d", got, want)
	}
}

func TestPipelineIntegrityCacheKeepsIndependentScopes(t *testing.T) {
	eng := newEngineFixture(t)
	optsA := IntegrityOptions{WebDir: filepath.Join(t.TempDir(), "web-a")}
	optsB := IntegrityOptions{WebDir: filepath.Join(t.TempDir(), "web-b"), IncludeArchived: true}

	eng.StorePipelineIntegrityFindings(optsA, []IntegrityFinding{{Feed: "alpha"}}, nil)
	eng.StorePipelineIntegrityFindings(optsB, []IntegrityFinding{{Feed: "beta"}, {Feed: "gamma"}}, nil)

	snapA := eng.PipelineIntegrityCacheSnapshot(optsA)
	if snapA.CacheState != IntegrityCacheFresh || len(snapA.Findings) != 1 || snapA.Findings[0].Feed != "alpha" {
		t.Fatalf("scope A snapshot = %+v, want fresh alpha only", snapA)
	}
	snapB := eng.PipelineIntegrityCacheSnapshot(optsB)
	if snapB.CacheState != IntegrityCacheFresh || len(snapB.Findings) != 2 || snapB.Findings[0].Feed != "beta" {
		t.Fatalf("scope B snapshot = %+v, want fresh beta/gamma", snapB)
	}
	cold := eng.PipelineIntegrityCacheSnapshot(IntegrityOptions{WebDir: filepath.Join(t.TempDir(), "web-c")})
	if cold.CacheState != IntegrityCacheCold || len(cold.Findings) != 0 {
		t.Fatalf("unknown scope snapshot = %+v, want cold empty", cold)
	}
}

func TestReloadStalesOldWebDirIntegrityScope(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	writeRuntimeReloadConfig(t, cfgPath, root, 2)

	eng, err := New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	oldWebDir := eng.Runtime().WebDir
	eng.StorePipelineIntegrityFindings(IntegrityOptions{}, []IntegrityFinding{{Feed: "sample"}}, nil)

	newWebDir := filepath.Join(root, "web-reloaded")
	writeRuntimeReloadConfigWithWebDir(t, cfgPath, root, newWebDir, 2)
	if err := eng.Reload(); err != nil {
		t.Fatal(err)
	}

	oldSnap := eng.PipelineIntegrityCacheSnapshot(IntegrityOptions{WebDir: oldWebDir})
	if oldSnap.CacheState != IntegrityCacheStale {
		t.Fatalf("old web dir cache state = %q, want stale", oldSnap.CacheState)
	}
	current := eng.StatusSnapshotLight().PipelineIntegrityCache
	if current.WebDir != newWebDir {
		t.Fatalf("current integrity web dir = %q, want %q", current.WebDir, newWebDir)
	}
	if current.CacheState != IntegrityCacheCold {
		t.Fatalf("current web dir cache state = %q, want cold", current.CacheState)
	}
	waitForEngineLaneIdle(t, eng)
}

func writeRuntimeReloadConfig(t *testing.T, path, root string, ceiling int) {
	t.Helper()
	writeRuntimeReloadConfigWithWebDir(t, path, root, filepath.Join(root, "web"), ceiling)
}

func writeRuntimeReloadConfigWithWebDir(t *testing.T, path, root, webDir string, ceiling int) {
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
  max_engine_lane_workers: 3
sources:
  sample:
    static:
      - 10.0.0.1
    frequency: 0
    ipv: ipv4
    output: netset
    processor:
      - passthrough
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), webDir, filepath.Join(root, "cache"), filepath.Join(root, "tmp"), ceiling)
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
