package engine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

func TestRunPersistsRunReasonAndProcessingDuration(t *testing.T) {
	modified := time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer server.Close()

	eng := newRunReasonTestEngine(t, server.URL, false)
	installAdvancingClock(eng, modified)

	if _, err := runSchedulerStyleOnce(t, eng, RunOptions{
		EnableAll:  true,
		Manual:     true,
		Reason:     runreason.ReasonManualReprocess,
		CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}

	entry := eng.state.EntrySnapshot("sample")
	if entry == nil {
		t.Fatal("expected sample cache entry")
	}
	if entry.LastRunReason != runreason.ReasonManualReprocess {
		t.Fatalf("unexpected last run reason: got %q", entry.LastRunReason)
	}
	if entry.LastProcessingMS <= 0 {
		t.Fatalf("expected positive processing duration, got %d", entry.LastProcessingMS)
	}

	status := eng.StatusSnapshot()
	if status.LastReason != runreason.ReasonManualReprocess {
		t.Fatalf("unexpected engine last reason: got %q", status.LastReason)
	}
	if status.CurrentReason != runreason.ReasonUnknown {
		t.Fatalf("expected no current run reason after completion, got %q", status.CurrentReason)
	}
}

func TestBeginFeedAttemptConcurrentEntryConfigUpdate(t *testing.T) {
	eng := newEngineFixture(t)
	entry := cache.New().Entry("initial")
	start := make(chan struct{})
	done := make(chan struct{})
	var stop atomic.Bool

	go func() {
		defer close(done)
		<-start
		names := []string{"initial", "renamed"}
		for i := 0; !stop.Load(); i++ {
			entry.ApplyProcessingSourceConfig(cache.ProcessingSourceConfigSnapshot{Name: names[i%len(names)]})
		}
	}()
	close(start)

	for range 5000 {
		attempt := eng.beginFeedAttempt(entry, runreason.ReasonScheduledDue)
		if attempt.name != "initial" && attempt.name != "renamed" {
			stop.Store(true)
			<-done
			t.Fatalf("attempt name = %q, want stable configured name", attempt.name)
		}
		attempt.finish()
	}
	stop.Store(true)
	<-done
}

func TestStatusSnapshotCountsExpandedMergeSources(t *testing.T) {
	cfg := config.New()
	cfg.Sources["plain"] = &config.Source{Name: "plain", URL: "https://example.test/plain.txt", Frequency: 60, IPV: "ipv4", Output: "ipset"}
	cfg.Sources["merged"] = &config.Source{Name: "merged", Frequency: 60, IPV: "ipv4", Output: "ipset", Provenance: config.ProvenanceSecondaryMerge}
	eng := newEngineFixture(t, withConfig(cfg))

	status := eng.StatusSnapshot()
	if status.MergeCount != 1 {
		t.Fatalf("merge_count = %d, want 1", status.MergeCount)
	}
}

func TestStatusSnapshotLightOmitsMetricsButKeepsLiveProgress(t *testing.T) {
	started := time.Date(2026, 6, 21, 15, 48, 41, 0, time.UTC)
	eng := newEngineFixture(t)
	eng.mu.Lock()
	eng.running = true
	eng.currentPhase = RunPhaseMetadata
	metrics := newRunMetrics(started, RunPhaseMetadata)
	eng.currentMetrics = metrics
	eng.currentMetricsPtr.Store(metrics)
	eng.lastMetrics = &RunMetricsSnapshot{StartedAt: started.Add(-time.Hour)}
	eng.mu.Unlock()
	eng.ObserveOperation("metadata.comparison_pair_overlap", time.Second)
	op := eng.beginActiveOperation("metadata.compare_pairs", "", "compare", "candidate_pairs", 10)
	defer op.Finish()
	op.Update(3, 10, nil)

	light := eng.StatusSnapshotLight()
	if !light.Running {
		t.Fatal("light status lost running state")
	}
	if light.CurrentPhase != RunPhaseMetadata {
		t.Fatalf("light status current phase = %q, want metadata", light.CurrentPhase)
	}
	if len(light.ActiveOperations) != 1 {
		t.Fatalf("light status active operations = %d, want 1", len(light.ActiveOperations))
	}
	if light.ActiveOperations[0].Current != 3 || light.ActiveOperations[0].Total != 10 {
		t.Fatalf("light status active operation progress = %d/%d, want 3/10", light.ActiveOperations[0].Current, light.ActiveOperations[0].Total)
	}
	encoded, err := json.Marshal(light)
	if err != nil {
		t.Fatalf("marshal light status: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode light status fields: %v", err)
	}
	for _, field := range []string{"current_metrics", "last_metrics", "lifetime_metrics"} {
		if _, ok := fields[field]; ok {
			t.Fatalf("light status JSON included %s", field)
		}
	}

	full := eng.StatusSnapshot()
	if full.CurrentMetrics == nil {
		t.Fatal("full status lost current metrics")
	}
	if full.LastMetrics == nil {
		t.Fatal("full status lost last metrics")
	}
	if full.LifetimeMetrics == nil {
		t.Fatal("full status lost lifetime metrics")
	}
}

func TestObserverMetricsDoNotTakeEngineMutex(t *testing.T) {
	eng := newEngineFixture(t)
	metrics := newRunMetrics(time.Now().UTC(), RunPhaseMetadata)
	eng.currentMetricsPtr.Store(metrics)
	t.Cleanup(func() { eng.currentMetricsPtr.Store(nil) })

	eng.mu.Lock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		eng.ObserveOperation("http.admin_status.build", time.Millisecond)
		eng.ObserveCounter("http.admin_status", 1, 128)
		eng.observeRunOperationAggregate("http.admin_status.aggregate", 2, 2*time.Millisecond, time.Millisecond)
		eng.observeFeedOperation("sample", "sources.finalize", time.Millisecond)
	}()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		eng.mu.Unlock()
		t.Fatal("observer metrics blocked on engine mutex")
	}
	eng.mu.Unlock()
}

func TestActiveAndBackgroundUpdatesDoNotTakeEngineMutex(t *testing.T) {
	eng := newEngineFixture(t)
	op := eng.beginActiveOperation("metadata.compare_pairs", "", "compare", "pairs", 10)
	task := eng.beginBackgroundTask("Entity artifacts refresh", "test", "running", "test", 0, 10)
	t.Cleanup(func() {
		op.Finish()
		task.Finish()
	})

	eng.mu.Lock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		op.Update(5, 10, map[string]int64{"pairs": 5})
		op.Add(1, 10, nil)
		task.Update("running", "progress", 5, 10)
	}()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		eng.mu.Unlock()
		t.Fatal("active/background progress update blocked on engine mutex")
	}
	eng.mu.Unlock()
}

func TestStatusSnapshotDoesNotHoldEngineMutexWhileReadingActiveOperations(t *testing.T) {
	eng := newEngineFixture(t)
	op := eng.beginActiveOperation("metadata.compare_pairs", "", "compare", "pairs", 10)
	defer op.Finish()

	eng.activeOperationsMu.Lock()
	statusDone := make(chan struct{})
	go func() {
		defer close(statusDone)
		_ = eng.StatusSnapshotLight()
	}()

	lockDone := make(chan struct{})
	go func() {
		eng.mu.Lock()
		close(lockDone)
		eng.mu.Unlock()
	}()
	select {
	case <-lockDone:
	case <-time.After(250 * time.Millisecond):
		eng.activeOperationsMu.Unlock()
		t.Fatal("status snapshot held engine mutex while waiting on active operations")
	}
	eng.activeOperationsMu.Unlock()
	select {
	case <-statusDone:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("status snapshot did not finish after active operation lock was released")
	}
}

func TestDerivativeRunReasonIsDependencyUpdate(t *testing.T) {
	modified := time.Date(2026, 4, 11, 11, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer server.Close()

	eng := newRunReasonTestEngine(t, server.URL, true)
	installAdvancingClock(eng, modified)

	if _, err := runSchedulerStyleOnce(t, eng, RunOptions{
		EnableAll:  true,
		Manual:     true,
		Reason:     runreason.ReasonManualRun,
		CleanupOld: true,
	}); err != nil {
		t.Fatal(err)
	}

	parent := eng.state.EntrySnapshot("sample")
	if parent == nil {
		t.Fatal("expected parent cache entry")
	}
	if parent.LastRunReason != runreason.ReasonManualRun {
		t.Fatalf("unexpected parent reason: got %q", parent.LastRunReason)
	}

	child := eng.state.EntrySnapshot("sample_1h")
	if child == nil {
		t.Fatal("expected derivative cache entry")
	}
	if child.LastRunReason != runreason.ReasonManualRun {
		t.Fatalf("unexpected derivative reason: got %q", child.LastRunReason)
	}
	if child.LastProcessingMS <= 0 {
		t.Fatalf("expected positive derivative processing duration, got %d", child.LastProcessingMS)
	}
}

func TestStatusSnapshotIncludesActiveFeeds(t *testing.T) {
	now := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	eng := newEngineFixture(t, withNow(func() time.Time { return now }))
	entry := &cache.Entry{Name: "sample"}

	attempt := eng.beginFeedAttempt(entry, runreason.ReasonManualRun)
	status := eng.StatusSnapshot()
	if len(status.ActiveFeeds) != 1 {
		t.Fatalf("expected 1 active feed, got %d", len(status.ActiveFeeds))
	}
	active := status.ActiveFeeds[0]
	if active.Name != "sample" {
		t.Fatalf("unexpected active feed name: got %q", active.Name)
	}
	if active.Reason != runreason.ReasonManualRun {
		t.Fatalf("unexpected active feed reason: got %q", active.Reason)
	}
	if !active.StartedAt.Equal(now) {
		t.Fatalf("unexpected active feed start: got %v want %v", active.StartedAt, now)
	}

	attempt.finish()
	status = eng.StatusSnapshot()
	if len(status.ActiveFeeds) != 0 {
		t.Fatalf("expected no active feeds after finish, got %d", len(status.ActiveFeeds))
	}
}

func TestStatusSnapshotIncludesActiveOperations(t *testing.T) {
	eng := newEngineFixture(t)

	op := eng.beginActiveOperation("retention.reconcile_cohorts", "sample", "scan", "files", 10)
	op.Update(4, 10, map[string]int64{"processed_cohorts": 4})
	op.Add(3, 10, map[string]int64{"processed_cohorts": 7})

	status := eng.StatusSnapshot()
	if len(status.ActiveOperations) != 1 {
		t.Fatalf("expected 1 active operation, got %d", len(status.ActiveOperations))
	}
	active := status.ActiveOperations[0]
	if active.Operation != "retention.reconcile_cohorts" || active.Feed != "sample" || active.Stage != "scan" {
		t.Fatalf("unexpected active operation: %+v", active)
	}
	if active.Current != 7 || active.Total != 10 {
		t.Fatalf("active operation progress = %d/%d, want 7/10", active.Current, active.Total)
	}
	if active.Unit != "files" {
		t.Fatalf("active operation unit = %q, want files", active.Unit)
	}
	if active.CompletionPct != 70 {
		t.Fatalf("active operation completion = %d, want 70", active.CompletionPct)
	}
	if active.RatePerSecond < 0 {
		t.Fatalf("active operation rate = %f, want non-negative", active.RatePerSecond)
	}
	if got := active.Counters["processed_cohorts"]; got != 7 {
		t.Fatalf("processed_cohorts counter = %d, want 7", got)
	}

	op.Finish()
	status = eng.StatusSnapshot()
	if len(status.ActiveOperations) != 0 {
		t.Fatalf("expected no active operations after finish, got %d", len(status.ActiveOperations))
	}
}

func TestStatusSnapshotIncludesCurrentBatchAndPhasePlan(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	cfg := config.New()
	cfg.Sources["plain"] = &config.Source{Name: "plain", URL: "https://example.test/plain.txt", Frequency: 60, IPV: "ipv4", Output: "ipset"}
	cfg.Sources["plain_1h"] = &config.Source{Name: "plain_1h", Frequency: 60, IPV: "ipv4", Output: "ipset", Provenance: config.ProvenanceSecondaryRetention}
	cfg.Sources["merged"] = &config.Source{Name: "merged", Frequency: 60, IPV: "ipv4", Output: "ipset", Provenance: config.ProvenanceSecondaryMerge}
	eng := newEngineFixture(t, withConfig(cfg), withNow(func() time.Time { return now }))

	if !eng.tryMarkRunStart(now, runreason.ReasonManualRun) {
		t.Fatal("expected run start")
	}
	defer eng.markRunIdleAfterFinalization(&Report{StartedAt: now}, nil)
	eng.startRunBatch([]string{"plain", "plain_1h", "merged"})
	eng.markRunBatchCompleted("plain")
	attempt := eng.beginFeedAttempt(&cache.Entry{Name: "plain_1h"}, runreason.ReasonDependencyUpdate)
	defer attempt.finish()
	eng.setRunPhase(RunPhaseSources)
	eng.setRunPhasePlan([]RunPhase{RunPhasePreflight, RunPhaseSources, RunPhaseMetadata, RunPhasePublish}, true)

	status := eng.StatusSnapshot()
	if status.CurrentBatch == nil {
		t.Fatal("expected current batch")
	}
	if status.CurrentBatch.Total != 3 || status.CurrentBatch.Completed != 1 || status.CurrentBatch.Active != 1 || status.CurrentBatch.Pending != 1 {
		t.Fatalf("unexpected batch counts: %+v", status.CurrentBatch)
	}
	if status.CurrentBatch.SourceTotal != 1 || status.CurrentBatch.SourceCompleted != 1 {
		t.Fatalf("unexpected source counts: %+v", status.CurrentBatch)
	}
	if status.CurrentBatch.HistoryTotal != 1 || status.CurrentBatch.HistoryCompleted != 0 {
		t.Fatalf("unexpected history counts: %+v", status.CurrentBatch)
	}
	if status.CurrentBatch.MergeTotal != 1 || status.CurrentBatch.MergeCompleted != 0 {
		t.Fatalf("unexpected merge counts: %+v", status.CurrentBatch)
	}
	if got, want := status.CurrentBatch.ActiveNames, []string{"plain_1h"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("active names = %v, want %v", got, want)
	}
	if status.PhasePlan == nil {
		t.Fatal("expected phase plan")
	}
	if status.PhasePlan.CurrentPosition != 2 || status.PhasePlan.Total != 4 || !status.PhasePlan.Final {
		t.Fatalf("unexpected phase plan: %+v", status.PhasePlan)
	}
}

func TestStatusSnapshotIncludesPhase(t *testing.T) {
	eng := newEngineFixture(t)

	eng.setRunPhase(RunPhaseASN)

	status := eng.StatusSnapshot()
	if status.CurrentPhase != RunPhaseASN {
		t.Fatalf("unexpected current phase: got %q", status.CurrentPhase)
	}
}

func TestStatusSnapshotIncludesBackgroundTasks(t *testing.T) {
	now := time.Date(2026, 4, 24, 13, 0, 0, 0, time.UTC)
	eng := newEngineFixture(t, withNow(func() time.Time { return now }))

	task := eng.beginBackgroundTask("Entity artifacts rebuild", "startup", "planning", "building full country and ASN entity artifacts", 0, 0)
	task.Update("building indexes", "building country and ASN index payloads", 1, 2)

	status := eng.StatusSnapshot()
	if len(status.BackgroundTasks) != 1 {
		t.Fatalf("expected 1 background task, got %d", len(status.BackgroundTasks))
	}
	bg := status.BackgroundTasks[0]
	if bg.Name != "Entity artifacts rebuild" {
		t.Fatalf("unexpected background task name: got %q", bg.Name)
	}
	if bg.Trigger != "startup" {
		t.Fatalf("unexpected background task trigger: got %q", bg.Trigger)
	}
	if bg.Stage != "building indexes" {
		t.Fatalf("unexpected background task stage: got %q", bg.Stage)
	}
	if bg.Current != 1 || bg.Total != 2 {
		t.Fatalf("unexpected background task progress: got %d/%d", bg.Current, bg.Total)
	}

	task.Finish()
	status = eng.StatusSnapshot()
	if len(status.BackgroundTasks) != 0 {
		t.Fatalf("expected no background tasks after finish, got %d", len(status.BackgroundTasks))
	}
}

func newRunReasonTestEngine(t *testing.T, sourceURL string, withHistory bool) *Engine {
	t.Helper()

	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	historyLine := ""
	if withHistory {
		historyLine = "    history: [60]\n"
	}
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
    url: %q
    frequency: 1
%s    ipv: ipv4
    output: ip
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), sourceURL, historyLine)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

func installAdvancingClock(eng *Engine, base time.Time) {
	var tick int64
	eng.now = func() time.Time {
		step := atomic.AddInt64(&tick, 1)
		return base.Add(time.Duration(step) * 10 * time.Millisecond)
	}
}
