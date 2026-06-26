package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/asnloc"
	"github.com/firehol/update-ipsets/pkg/cache"
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
	if got, want := rt.PushToGitTimeout, 600*time.Second; got != want {
		t.Fatalf("expected push-to-git timeout %s, got %s", want, got)
	}

	cfg.Runtime.MaxBackgroundWorkers = 3
	cfg.Runtime.MaxEngineLaneWorkers = 4
	cfg.Runtime.PushToGitTimeout = 42
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
	if got, want := rt.PushToGitTimeout, 42*time.Second; got != want {
		t.Fatalf("expected explicit push-to-git timeout %s, got %s", want, got)
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

func TestReloadContextHonorsCanceledContextBeforeWork(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	writeRuntimeReloadConfig(t, cfgPath, root, 2)

	eng, err := New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	before := eng.StatusSnapshotLight().ConfigReloadCount
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err = eng.ReloadContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ReloadContext() error = %v, want context.Canceled", err)
	}
	after := eng.StatusSnapshotLight().ConfigReloadCount
	if after != before {
		t.Fatalf("reload count changed from %d to %d after canceled reload", before, after)
	}
}

func TestReloadContextDoesNotHoldEngineMutexDuringDirectoryCreation(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	writeRuntimeReloadConfig(t, cfgPath, root, 2)

	eng, err := New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	newWebDir := filepath.Join(root, "web-reloaded")
	writeRuntimeReloadConfigWithWebDir(t, cfgPath, root, newWebDir, 2)

	entered := make(chan struct{})
	release := make(chan struct{})
	var blocked atomic.Bool
	ensureRuntimeDirectoryHook = func(dir string) {
		if dir == newWebDir && blocked.CompareAndSwap(false, true) {
			close(entered)
			<-release
		}
	}
	t.Cleanup(func() { ensureRuntimeDirectoryHook = nil })

	done := make(chan error, 1)
	go func() {
		done <- eng.ReloadContext(t.Context())
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("reload did not reach directory creation")
	}

	statusDone := make(chan struct{})
	go func() {
		_ = eng.StatusSnapshotLight()
		close(statusDone)
	}()
	select {
	case <-statusDone:
	case <-time.After(250 * time.Millisecond):
		close(release)
		t.Fatal("light status blocked while reload was creating directories")
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
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

func TestReloadConcurrentPublicRuntimeReadersRace(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	writeRuntimeReloadConfigWithProviders(t, cfgPath, root)

	eng, err := New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	rt := eng.Runtime()
	if err := os.MkdirAll(filepath.Join(rt.LibDir, "sample", "new"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(rt.BaseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rt.BaseDir, "sample.netset"), []byte("10.0.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := eng.state.Entry("sample")
	entry.Name = "sample"
	entry.File = "sample.netset"
	entry.IPV = "ipv4"
	entry.Hash = "net"
	entry.StartedDate = time.Now().Add(-time.Hour).Unix()
	entry.SourceDate = time.Now().Unix()
	entry.ProcessedDate = entry.SourceDate
	entry.CheckedDate = entry.SourceDate
	entry.FrequencyMinutes = 60

	ctx := t.Context()
	stop := make(chan struct{})
	var stopOnce sync.Once
	closeStop := func() {
		stopOnce.Do(func() { close(stop) })
	}

	errCh := make(chan error, 1)
	recordErr := func(err error) {
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		select {
		case errCh <- err:
		default:
		}
		closeStop()
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer closeStop()
		for i := 0; i < 20; i++ {
			recordErr(eng.ReloadContext(ctx))
			select {
			case <-stop:
				return
			default:
			}
		}
	}()

	type raceReader struct {
		name   string
		fn     func()
		active atomic.Int64
		done   atomic.Int64
	}
	readers := []*raceReader{
		{name: "LookupIPContext", fn: func() { _, _ = eng.LookupIPContext("10.0.0.1") }},
		{name: "availableGeoSyntheticProviders", fn: func() { _, _, _ = eng.availableGeoSyntheticProviders() }},
		{name: "historyTailFromRuntime", fn: func() { _ = eng.historyTailFromRuntime("sample") }},
		{name: "changesetTailFromRuntime", fn: func() { _ = eng.changesetTailFromRuntime("sample") }},
		{name: "retentionPastFromRuntime", fn: func() { _ = eng.retentionPastFromRuntime("sample", time.Now().Unix()-3600) }},
		{name: "retentionCohortsFromRuntime", fn: func() { _ = eng.retentionCohortsFromRuntime(ctx, "sample") }},
		{name: "historyStatsFromRuntime", fn: func() { _ = eng.historyStatsFromRuntime("sample", &cache.Entry{Name: "sample"}, 60) }},
		{name: "observeHistoryPoint", fn: func() {
			_ = eng.observeHistoryPoint("sample", HistoryPoint{Timestamp: time.Now().Unix(), Name: "sample", Entries: 1, UniqueIPs: 1}, &cache.Entry{Name: "sample"}, nil, 60)
		}},
		{name: "observeChangesetPoint", fn: func() { eng.observeChangesetPoint("sample", ChangesetPoint{Timestamp: time.Now().Unix(), Added: 1}) }},
		{name: "observeRetentionPast", fn: func() { eng.observeRetentionPast("sample", time.Now().Unix()-3600, 1, 1) }},
		{name: "observeRetentionCohort", fn: func() { eng.observeRetentionCohort("sample", time.Now().Unix(), 1) }},
		{name: "IsPublicFeedName", fn: func() { _ = eng.IsPublicFeedName("sample") }},
		{name: "IsRedistributable", fn: func() { _ = eng.IsRedistributable("sample") }},
		{name: "PublicRawFeedAllowed", fn: func() { _ = eng.PublicRawFeedAllowed("sample") }},
		{name: "Entry", fn: func() { _, _ = eng.Entry("sample") }},
		{name: "SetData", fn: func() { _, _, _ = eng.SetData("sample") }},
		{name: "Metadata", fn: func() { _, _ = eng.Metadata("sample") }},
		{name: "PublicCategories", fn: func() { _ = eng.PublicCategories() }},
		{name: "PublicFeedSummaries", fn: func() { _ = eng.PublicFeedSummaries() }},
		{name: "BogonProviders", fn: func() { _ = eng.BogonProviders() }},
		{name: "ASNProviders", fn: func() { _ = eng.ASNProviders() }},
		{name: "GeoProviders", fn: func() { _ = eng.GeoProviders() }},
		{name: "CriticalInfrastructureProviders", fn: func() { _ = eng.CriticalInfrastructureProviders() }},
		{name: "IsCriticalInfrastructureTarget", fn: func() { _ = eng.IsCriticalInfrastructureTarget("sample") }},
		{name: "QueryIP", fn: func() { _, _ = eng.QueryIP(ctx, "10.0.0.1") }},
		{name: "QueryFeedIP", fn: func() { _, _, _ = eng.QueryFeedIP(ctx, "sample", "10.0.0.1") }},
		{name: "HistorySeries", fn: func() { _, _ = eng.HistorySeries("sample") }},
		{name: "ChangesetSeries", fn: func() { _, _ = eng.ChangesetSeries("sample") }},
		{name: "Retention", fn: func() { _, _ = eng.Retention("sample") }},
		{name: "CompareSet", fn: func() { _, _ = eng.CompareSet(ctx, "sample") }},
		{name: "StatusSnapshotLight", fn: func() { _ = eng.StatusSnapshotLight() }},
		{name: "StatusSnapshot", fn: func() { _ = eng.StatusSnapshot() }},
	}
	for _, reader := range readers {
		reader := reader
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				select {
				case <-stop:
					return
				default:
				}
				reader.active.Add(1)
				reader.fn()
				reader.active.Add(-1)
				reader.done.Add(1)
				time.Sleep(time.Millisecond)
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		closeStop()
		for _, reader := range readers {
			t.Logf("reader %s active=%d done=%d", reader.name, reader.active.Load(), reader.done.Load())
		}
		t.Fatal("concurrent reload/readers did not stop")
	}
	select {
	case err := <-errCh:
		t.Fatalf("concurrent reload/readers error: %v", err)
	default:
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

func writeRuntimeReloadConfigWithProviders(t *testing.T, path, root string) {
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
  max_ingest_workers: 2
  parallel_downloads: 2
  parallel_dns_queries: 2
  max_processing_workers: 2
  max_heavy_phase_workers: 2
  max_background_workers: 2
  max_engine_lane_workers: 2
sources:
  sample:
    static:
      - 10.0.0.1
    frequency: 60
    ipv: ipv4
    output: netset
    processor:
      - passthrough
  dbip_country:
    url: https://example.test/dbip.csv.gz
    frequency: 1440
    use: [geoip]
    format: dbip_country_csv
    label: DB-IP Country
  iptoasn:
    url: https://example.test/iptoasn.tsv
    frequency: 1440
    use: [asn]
    format: iptoasn_combined_tsv
    label: IPtoASN
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), filepath.Join(root, "tmp"))
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
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
