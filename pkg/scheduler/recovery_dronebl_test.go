package scheduler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

func TestDroneBLArtifactQueuesThroughDownloadLane(t *testing.T) {
	eng, root := newDroneBLSchedulerRecoveryEngine(t)
	stagedArtifactPath := filepath.Join(root, "lib", "artifacts", "dronebl", "source.new")
	if err := os.MkdirAll(filepath.Dir(stagedArtifactPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagedArtifactPath, []byte("staged dronebl buildzone\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)

	runner.recoverStagedWork(t.Context())

	activity := runner.ActivitySnapshot()
	if len(activity.DownloadWaiting) != 1 {
		t.Fatalf("expected recovered artifact in downloader queue, got %#v", activity.DownloadWaiting)
	}
	if activity.DownloadWaiting[0].Name != "dronebl" {
		t.Fatalf("expected dronebl artifact queued, got %#v", activity.DownloadWaiting)
	}
	if activity.DownloadWaiting[0].Kind != string(queuedWorkKindRecoveredArtifact) {
		t.Fatalf("download kind = %q, want %q", activity.DownloadWaiting[0].Kind, queuedWorkKindRecoveredArtifact)
	}
	if len(activity.ProcessingWaiting) != 0 {
		t.Fatalf("recovery must not enqueue child processing before downloader materialization, got %#v", activity.ProcessingWaiting)
	}
}

func TestDroneBLChildrenMaterializeInDownloadWorker(t *testing.T) {
	assertRecoveredDroneBLChildrenMaterializeInDownloadWorker(t)
}

func TestRecoveredDroneBLArtifactMaterializesInDownloadWorker(t *testing.T) {
	assertRecoveredDroneBLChildrenMaterializeInDownloadWorker(t)
}

func assertRecoveredDroneBLChildrenMaterializeInDownloadWorker(t *testing.T) {
	t.Helper()
	eng, root := newDroneBLSchedulerRecoveryEngine(t)
	stagedArtifactPath := filepath.Join(root, "lib", "artifacts", "dronebl", "source.new")
	if err := os.MkdirAll(filepath.Dir(stagedArtifactPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagedArtifactPath, []byte(":17:\n17.17.17.17\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)

	runner.runDownload(t.Context(), queuedWork{
		Name:      "dronebl",
		Kind:      queuedWorkKindRecoveredArtifact,
		Reason:    runreason.ReasonScheduledDue,
		QueuedAt:  time.Now().UTC(),
		ForceRun:  true,
		Immediate: true,
	})

	activity := runner.ActivitySnapshot()
	if len(activity.DownloadWaiting) != 0 {
		t.Fatalf("recovered artifact should be consumed by downloader worker, got waiting downloads %#v", activity.DownloadWaiting)
	}
	if len(activity.ProcessingWaiting) != 1 {
		t.Fatalf("expected materialized DroneBL child in processing queue, got %#v", activity.ProcessingWaiting)
	}
	if activity.ProcessingWaiting[0].Name != "child" {
		t.Fatalf("processing item name = %q, want child", activity.ProcessingWaiting[0].Name)
	}
}

func TestDroneBLDoesNotAcquireEngineLane(t *testing.T) {
	eng, root := newDroneBLSchedulerRecoveryEngine(t)
	eng.StorePipelineIntegrityFindings(engine.IntegrityOptions{}, []engine.IntegrityFinding{{Feed: "child"}}, nil)
	releaseLane := make(chan struct{})
	if _, err := eng.QueuePipelineIntegrityReprocess(t.Context(), engine.IntegrityOptions{}, "test_blocker", func(ctx context.Context, _ []engine.IntegrityFinding) error {
		select {
		case <-releaseLane:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}); err != nil {
		t.Fatalf("queue lane blocker: %v", err)
	}
	waitForEngineLaneActive(t, eng)
	defer func() {
		close(releaseLane)
		waitForEngineLaneIdle(t, eng)
	}()

	stagedArtifactPath := filepath.Join(root, "lib", "artifacts", "dronebl", "source.new")
	if err := os.MkdirAll(filepath.Dir(stagedArtifactPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagedArtifactPath, []byte(":17:\n17.17.17.17\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)
	done := make(chan struct{})
	go func() {
		runner.runDownload(t.Context(), queuedWork{
			Name:      "dronebl",
			Kind:      queuedWorkKindRecoveredArtifact,
			Reason:    runreason.ReasonScheduledDue,
			QueuedAt:  time.Now().UTC(),
			ForceRun:  true,
			Immediate: true,
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("DroneBL downloader work waited behind the busy engine lane")
	}
	if got := runner.ActivitySnapshot().ProcessingWaiting; len(got) != 1 || got[0].Name != "child" {
		t.Fatalf("DroneBL downloader work did not materialize child processing while engine lane was busy: %#v", got)
	}
}

func TestRecoveredCorruptDroneBLArtifactRequeuesNormalDownloaderFetch(t *testing.T) {
	eng, root := newDroneBLSchedulerRecoveryEngine(t)
	stagedArtifactPath := filepath.Join(root, "lib", "artifacts", "dronebl", "source.new")
	if err := os.MkdirAll(filepath.Dir(stagedArtifactPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagedArtifactPath, []byte(strings.Repeat("1", 2*1024*1024+1)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := New(eng, true, nil)

	runner.runDownload(t.Context(), queuedWork{
		Name:      "dronebl",
		Kind:      queuedWorkKindRecoveredArtifact,
		Reason:    runreason.ReasonScheduledDue,
		QueuedAt:  time.Now().UTC(),
		ForceRun:  true,
		Immediate: true,
	})

	activity := runner.ActivitySnapshot()
	if len(activity.DownloadWaiting) != 1 {
		t.Fatalf("expected normal downloader retry after corrupt recovery, got %#v", activity.DownloadWaiting)
	}
	if activity.DownloadWaiting[0].Name != "dronebl" {
		t.Fatalf("retry item name = %q, want dronebl", activity.DownloadWaiting[0].Name)
	}
	if activity.DownloadWaiting[0].Kind != string(queuedWorkKindNormal) {
		t.Fatalf("retry item kind = %q, want %q", activity.DownloadWaiting[0].Kind, queuedWorkKindNormal)
	}
	if len(activity.ProcessingWaiting) != 0 {
		t.Fatalf("corrupt recovery must not enqueue child processing, got %#v", activity.ProcessingWaiting)
	}
	if _, err := os.Stat(stagedArtifactPath + ".corrupt"); err != nil {
		t.Fatalf("corrupt recovered stage was not preserved as .corrupt: %v", err)
	}
	if _, err := os.Stat(stagedArtifactPath + ".corrupt.json"); err != nil {
		t.Fatalf("corrupt recovered stage sidecar was not written from downloader lane: %v", err)
	}
}

func newDroneBLSchedulerRecoveryEngine(t *testing.T) (*engine.Engine, string) {
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
artifacts:
  dronebl:
    type: dronebl_buildzone
    frequency: 60
    info: dronebl
    maintainer: dronebl
    maintainer_url: https://example.test
    rsync_url: rsync://example.test/dronebl/
sources:
  child:
    url: artifact://dronebl?parts=auto_botnets
    frequency: 0
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: child feed
    maintainer: test
    maintainer_url: https://example.test
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	eng, err := engine.New(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	return eng, root
}

func waitForEngineLaneActive(t *testing.T, eng *engine.Engine) {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if eng.StatusSnapshotLight().EngineLane.ActiveCount > 0 {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("engine lane did not become active")
		case <-ticker.C:
		}
	}
}

func waitForEngineLaneIdle(t *testing.T, eng *engine.Engine) {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		snap := eng.StatusSnapshotLight().EngineLane
		if snap.ActiveCount == 0 && snap.WaitingCount == 0 {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("engine lane did not become idle: %#v", snap)
		case <-ticker.C:
		}
	}
}
