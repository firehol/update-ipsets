package engine

import (
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
