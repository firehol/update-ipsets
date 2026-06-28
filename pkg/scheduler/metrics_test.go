package scheduler

import (
	"testing"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
)

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

func TestMetricsRecoveryAndAdmissionCountersExposeLocalMetrics(t *testing.T) {
	var metrics metricsState
	now := time.Unix(1, 0)
	recoveredBefore := schedulerMetricCounterValue("scheduler.recovered_panics", map[string]string{"scheduler.component": "download_worker"})
	admissionBefore := schedulerMetricCounterValue("scheduler.action.admission_failures", nil)

	metrics.recordRecoveredPanic("download_worker", now)
	metrics.recordActionAdmissionFailure(now)

	if got := schedulerMetricCounterValue("scheduler.recovered_panics", map[string]string{"scheduler.component": "download_worker"}); got <= recoveredBefore {
		t.Fatalf("scheduler.recovered_panics = %d, want above previous value %d", got, recoveredBefore)
	}
	if got := schedulerMetricCounterValue("scheduler.action.admission_failures", nil); got <= admissionBefore {
		t.Fatalf("scheduler.action.admission_failures = %d, want above previous value %d", got, admissionBefore)
	}
	snap := metrics.snapshot()
	if snap.RecoveredPanics != 1 || snap.ActionAdmissionFailures != 1 {
		t.Fatalf("metrics snapshot = %+v, want recovered/admission counters", snap)
	}
}

func schedulerMetricCounterValue(name string, labels map[string]string) int64 {
	for _, snap := range observability.SnapshotMetrics() {
		if snap.Name != name || snap.Value == 0 {
			continue
		}
		got := map[string]string{}
		for _, label := range snap.Labels {
			got[label.Key] = label.Value
		}
		matched := true
		for key, value := range labels {
			if got[key] != value {
				matched = false
				break
			}
		}
		if matched {
			return snap.Value
		}
	}
	return 0
}
