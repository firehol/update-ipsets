package scheduler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

func TestTriggerQueuedActionRejectsDuplicatePendingAction(t *testing.T) {
	runner := &Runner{
		actionCh: make(chan PendingAction, 1),
		download: downloadLoopState{
			wake: make(chan struct{}, 1),
		},
		processing: processingLoopState{
			wake: make(chan struct{}, 1),
		},
	}

	if !runner.TriggerQueuedAction(PendingAction{Names: []string{"sample"}}) {
		t.Fatal("expected first queued action to be accepted")
	}
	if runner.TriggerQueuedAction(PendingAction{Names: []string{"sample"}}) {
		t.Fatal("expected duplicate queued action to be rejected while first action is pending")
	}
}

func TestTriggerSourcesContextTimesOutWhenActionQueueFull(t *testing.T) {
	runner := &Runner{
		actionCh: make(chan PendingAction, 1),
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		download: downloadLoopState{
			wake: make(chan struct{}, 1),
		},
		processing: processingLoopState{
			wake: make(chan struct{}, 1),
		},
	}
	runner.actionCh <- PendingAction{Names: []string{"first"}}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := runner.TriggerSourcesContext(ctx, PendingAction{Names: []string{"second"}})
	elapsed := time.Since(started)

	if !errors.Is(err, ErrActionQueueSaturated) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("TriggerSourcesContext() error = %v, want saturated deadline", err)
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("TriggerSourcesContext() elapsed = %s, want bounded by caller context", elapsed)
	}
	metrics := runner.MetricsSnapshot()
	if metrics.ActionAdmissionFailures != 1 || !metrics.Degraded {
		t.Fatalf("scheduler metrics after admission failure = %+v", metrics)
	}
}

func TestTryTriggerSourcesDoesNotBlockWhenActionQueueFull(t *testing.T) {
	runner := &Runner{
		actionCh: make(chan PendingAction, 1),
		download: downloadLoopState{
			wake: make(chan struct{}, 1),
		},
		processing: processingLoopState{
			wake: make(chan struct{}, 1),
		},
	}
	runner.actionCh <- PendingAction{Names: []string{"first"}}

	started := time.Now()
	if runner.TryTriggerSources(PendingAction{Names: []string{"second"}}) {
		t.Fatal("TryTriggerSources() accepted action despite full queue")
	}
	if elapsed := time.Since(started); elapsed > 50*time.Millisecond {
		t.Fatalf("TryTriggerSources() elapsed = %s, want non-blocking", elapsed)
	}
}

func TestHandleActionRecoveredRecordsPanicAndContinues(t *testing.T) {
	runner := &Runner{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		download: downloadLoopState{
			wake: make(chan struct{}, 1),
		},
		processing: processingLoopState{
			wake: make(chan struct{}, 1),
		},
	}

	runner.handleActionRecovered(t.Context(), PendingAction{Reprocess: true})
	metrics := runner.MetricsSnapshot()
	if metrics.RecoveredPanics != 1 || !metrics.Degraded {
		t.Fatalf("scheduler metrics after recovered action panic = %+v", metrics)
	}

	runner.handleActionRecovered(t.Context(), PendingAction{RunDue: true})
	select {
	case <-runner.download.wake:
	case <-time.After(2 * time.Second):
		t.Fatal("RunDue action did not wake download loop after recovered panic")
	}
}

func TestRunDueActionDoesNotNeedEngineSnapshot(t *testing.T) {
	runner := &Runner{
		download: downloadLoopState{
			wake: make(chan struct{}, 1),
		},
		processing: processingLoopState{
			wake: make(chan struct{}, 1),
		},
	}

	runner.handleActionRecovered(t.Context(), PendingAction{RunDue: true})
	if metrics := runner.MetricsSnapshot(); metrics.RecoveredPanics != 0 {
		t.Fatalf("RunDue action recovered panics = %d, want 0", metrics.RecoveredPanics)
	}
	select {
	case <-runner.download.wake:
	case <-time.After(2 * time.Second):
		t.Fatal("RunDue action did not wake download loop")
	}
}

func TestProcessingIntervalUsesReloadedRuntimeSnapshot(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	writeSchedulerProcessingIntervalConfig(t, cfgPath, root, 1)

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)
	if got, want := runner.processingInterval(), time.Minute; got != want {
		t.Fatalf("initial processing interval = %s, want %s", got, want)
	}

	writeSchedulerProcessingIntervalConfig(t, cfgPath, root, 3)
	if err := eng.ReloadContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got, want := runner.processingInterval(), 3*time.Minute; got != want {
		t.Fatalf("reloaded processing interval = %s, want %s", got, want)
	}
}

func writeSchedulerProcessingIntervalConfig(t *testing.T, cfgPath, root string, intervalMinutes int) {
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
  processing_interval_minutes: %d
sources:
  sample:
    static:
      - 10.0.0.1
    frequency: 60
    ipv: ipv4
    output: ipset
    processor: [passthrough]
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), filepath.Join(root, "tmp"), intervalMinutes)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadWorkerPanicClearsActiveQueue(t *testing.T) {
	runner := &Runner{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		download: downloadLoopState{
			wake:           make(chan struct{}, 1),
			waiting:        nil,
			active:         map[string]ActiveQueueFeed{"sample": {Name: "sample"}},
			refetchPending: map[string]queuedWork{"sample": {Name: "sample"}},
		},
	}

	runner.runDownload(t.Context(), queuedWork{Name: "sample"})

	if _, ok := runner.download.active["sample"]; ok {
		t.Fatalf("download panic left sample active: %#v", runner.download.active)
	}
	if metrics := runner.MetricsSnapshot(); metrics.RecoveredPanics != 1 || !metrics.Degraded {
		t.Fatalf("scheduler metrics after download panic = %+v", metrics)
	}
}

func TestProcessingBatchPanicRequeuesActiveWork(t *testing.T) {
	queued := queuedWork{Name: "sample", Reason: runreason.ReasonManualRun, QueuedAt: time.Unix(1_700_000_000, 0).UTC()}
	runner := &Runner{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		processing: processingLoopState{
			waiting:  map[string]queuedWork{},
			active:   map[string]ActiveQueueFeed{"sample": {Name: "sample"}},
			deferred: map[string]queuedWork{},
			wake:     make(chan struct{}, 1),
		},
	}

	runner.recoverProcessingBatchPanic([]queuedWork{queued}, "processing panic")

	if _, ok := runner.processing.active["sample"]; ok {
		t.Fatalf("processing panic left sample active: %#v", runner.processing.active)
	}
	if _, ok := runner.processing.waiting["sample"]; !ok {
		t.Fatalf("processing panic did not requeue sample: %#v", runner.processing.waiting)
	}
	if metrics := runner.MetricsSnapshot(); metrics.RecoveredPanics != 1 || !metrics.Degraded {
		t.Fatalf("scheduler metrics after processing panic = %+v", metrics)
	}
}

func TestEnqueueDownloadWhileActiveDefersRefetchUntilActiveFinishes(t *testing.T) {
	queuedAt := time.Unix(1_700_000_000, 0).UTC()
	runner := &Runner{
		download: downloadLoopState{
			waiting:        map[string]queuedWork{},
			active:         map[string]ActiveQueueFeed{"sample": {Name: "sample"}},
			refetchPending: map[string]queuedWork{},
		},
	}

	runner.enqueueDownload(queuedWork{
		Name:     "sample",
		Reason:   runreason.ReasonManualRecheck,
		QueuedAt: queuedAt,
		ForceRun: true,
	})

	if len(runner.download.waiting) != 0 {
		t.Fatalf("active download should not be requeued immediately, got waiting=%#v", runner.download.waiting)
	}
	pending, ok := runner.download.refetchPending["sample"]
	if !ok {
		t.Fatalf("expected active download to have deferred refetch state, got %#v", runner.download.refetchPending)
	}
	if pending.Reason != runreason.ReasonManualRecheck || !pending.ForceRun {
		t.Fatalf("deferred refetch = %#v, want manual forced recheck", pending)
	}

	runner.finishDownload("sample")
	runner.releaseDeferredDownload("sample")

	if _, ok := runner.download.refetchPending["sample"]; ok {
		t.Fatalf("expected deferred refetch to be released, got %#v", runner.download.refetchPending)
	}
	if got := runner.download.waiting["sample"]; got.Name != "sample" || got.Reason != runreason.ReasonManualRecheck || !got.ForceRun {
		t.Fatalf("released refetch = %#v, want waiting manual forced recheck", got)
	}
}

func TestDownloadQueueUsesEnqueueSequenceWhenQueuedAtTies(t *testing.T) {
	queuedAt := time.Unix(1_700_000_001, 0).UTC()
	runner := &Runner{
		now: func() time.Time { return queuedAt.Add(time.Minute) },
		download: downloadLoopState{
			waiting:        map[string]queuedWork{},
			active:         map[string]ActiveQueueFeed{},
			refetchPending: map[string]queuedWork{},
		},
	}

	for _, item := range []queuedWork{
		{Name: "third", QueuedAt: queuedAt, EnqueueSeq: 3},
		{Name: "first", QueuedAt: queuedAt, EnqueueSeq: 1},
		{Name: "second", QueuedAt: queuedAt, EnqueueSeq: 2},
	} {
		runner.enqueueDownload(item)
	}

	for _, want := range []string{"first", "second", "third"} {
		got, ok := runner.startNextDownload()
		if !ok {
			t.Fatalf("startNextDownload returned no item, want %s", want)
		}
		if got.Name != want {
			t.Fatalf("download start = %q, want %q", got.Name, want)
		}
		runner.finishDownload(got.Name)
	}
}

func TestDownloadQueueMergeKeepsRecoveredArtifactOwnershipAndEarliestSequence(t *testing.T) {
	queuedAt := time.Unix(1_700_000_002, 0).UTC()
	runner := &Runner{
		download: downloadLoopState{
			waiting:        map[string]queuedWork{},
			active:         map[string]ActiveQueueFeed{},
			refetchPending: map[string]queuedWork{},
		},
	}

	runner.enqueueDownload(queuedWork{
		Name:       "dronebl",
		Kind:       queuedWorkKindNormal,
		QueuedAt:   queuedAt,
		EnqueueSeq: 7,
	})
	runner.enqueueDownload(queuedWork{
		Name:       "dronebl",
		Kind:       queuedWorkKindRecoveredArtifact,
		QueuedAt:   queuedAt,
		EnqueueSeq: 3,
		ForceRun:   true,
		Immediate:  true,
	})

	got := runner.download.waiting["dronebl"]
	if got.Kind != queuedWorkKindRecoveredArtifact {
		t.Fatalf("merged DroneBL kind = %q, want recovered_artifact", got.Kind)
	}
	if got.EnqueueSeq != 3 {
		t.Fatalf("merged DroneBL enqueue seq = %d, want earliest 3", got.EnqueueSeq)
	}
	if !got.ForceRun || !got.Immediate {
		t.Fatalf("merged DroneBL flags = force:%v immediate:%v, want true/true", got.ForceRun, got.Immediate)
	}
}

func TestProviderDefaultsReprocessQueuesFullFeedTargets(t *testing.T) {
	eng, root := newSchedulerPolicyEngine(t, `
defaults:
  asn_provider: iptoasn
sources:
  iptoasn:
    url: https://example.test/asn.tsv
    frequency: 1440
    use: [asn]
    format: iptoasn_combined_tsv
    info: asn provider
    maintainer: test
    maintainer_url: https://example.test
  sample:
    url: https://example.test/sample.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`)
	if err := os.WriteFile(filepath.Join(root, "base", "sample.ipset"), []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)
	now := time.Unix(1_700_000_000, 0).UTC()

	runner.enqueueProviderDefaultsReprocess(now)

	got, ok := runner.processing.waiting["sample"]
	if !ok {
		t.Fatalf("expected provider-default drift to queue sample reprocess, got %#v", runner.processing.waiting)
	}
	if got.Reason != runreason.ReasonProviderDefaults || !got.ForceRun || !got.Immediate {
		t.Fatalf("queued provider-default reprocess = %#v, want provider_defaults forced immediate work", got)
	}
	if _, ok := runner.processing.waiting["iptoasn"]; ok {
		t.Fatalf("provider databases must not be full-feed reprocess targets, got %#v", runner.processing.waiting)
	}
}

func TestManualProviderReprocessQueuesTargetsWithPromotion(t *testing.T) {
	eng, root := newSchedulerPolicyEngine(t, `
defaults:
  asn_provider: iptoasn
sources:
  iptoasn:
    url: https://example.test/asn.tsv
    frequency: 1440
    use: [asn]
    format: iptoasn_combined_tsv
    info: asn provider
    maintainer: test
    maintainer_url: https://example.test
  sample:
    url: https://example.test/sample.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`)
	if err := os.WriteFile(filepath.Join(root, "base", "sample.ipset"), []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	providerDir := filepath.Join(root, "lib", "asn", "iptoasn")
	if err := os.MkdirAll(providerDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(providerDir, "source.new"), []byte("1.0.0.0\t1.0.0.255\t13335\tCF\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)
	now := time.Unix(1_700_000_000, 0).UTC()
	runner.now = func() time.Time { return now }

	runner.handleAction(t.Context(), PendingAction{
		Names:     []string{"iptoasn"},
		Reprocess: true,
		Reason:    runreason.ReasonManualReprocess,
	})

	got, ok := runner.processing.waiting["sample"]
	if !ok {
		t.Fatalf("expected provider reprocess to queue sample, got %#v", runner.processing.waiting)
	}
	if got.Reason != runreason.ReasonManualReprocess || !got.ForceRun || !got.Immediate {
		t.Fatalf("provider reprocess work = %#v, want forced immediate manual reprocess", got)
	}
	if len(got.Promote) != 1 || got.Promote[0] != "iptoasn" {
		t.Fatalf("provider reprocess promote list = %#v, want [iptoasn]", got.Promote)
	}
	if _, ok := runner.processing.waiting["iptoasn"]; ok {
		t.Fatalf("provider database must not be queued as a processing target, got %#v", runner.processing.waiting)
	}
}

func TestRecoverStagedWorkQueuesRecoveredSourceForProcessing(t *testing.T) {
	eng, root := newSchedulerPolicyEngine(t, `
sources:
  sample:
    url: https://example.test/sample.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`)
	if err := os.WriteFile(filepath.Join(root, "base", "sample.ipset.new"), []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)
	runner.now = func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }

	runner.recoverStagedWork(t.Context())

	got, ok := runner.processing.waiting["sample"]
	if !ok {
		t.Fatalf("expected recovered staged source to queue processing work, got %#v", runner.processing.waiting)
	}
	if got.Reason != runreason.ReasonScheduledDue || !got.ForceRun || !got.Immediate {
		t.Fatalf("recovered staged work = %#v, want scheduled forced immediate processing", got)
	}
	if _, err := os.Stat(filepath.Join(root, "base", "sample.ipset.processing")); err != nil {
		t.Fatalf("expected staged body to be claimed for processing: %v", err)
	}
}

func newSchedulerPolicyEngine(t *testing.T, body string) (*engine.Engine, string) {
	t.Helper()
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
%s
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), body)
	if err := os.MkdirAll(filepath.Join(root, "base"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	return eng, root
}
