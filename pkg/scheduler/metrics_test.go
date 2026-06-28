package scheduler

import "testing"

func TestMetricsSnapshotDoesNotWaitForMetricsLock(t *testing.T) {
	var metrics metricsState
	metrics.downloadEnqueued = 7

	metrics.mu.Lock()
	if got := metrics.snapshot(); got.DownloadEnqueued != 0 || len(got.Operations) != 0 {
		t.Fatalf("snapshot while locked = %+v, want empty best-effort snapshot", got)
	}
	metrics.mu.Unlock()

	got := metrics.snapshot()
	if got.DownloadEnqueued != 7 {
		t.Fatalf("snapshot after unlock download_enqueued = %d, want 7", got.DownloadEnqueued)
	}
}
