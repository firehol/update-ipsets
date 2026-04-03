package scheduler

import (
	"fmt"
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
	if err := os.WriteFile(filepath.Join(root, "base", "sample.ipset"), []byte("1.2.3.4\n"), 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(root, "base", "sample.ipset.new"), []byte("1.2.3.4\n"), 0o644); err != nil {
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
	if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	return eng, root
}
