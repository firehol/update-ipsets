package engine

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/internal/runtimeinfo"
	"github.com/firehol/update-ipsets/pkg/iprange"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

func TestRunDiagnosticSummaryIncludesOperationsCountersAndActiveWork(t *testing.T) {
	var logs bytes.Buffer
	eng := newEngineFixture(t)
	eng.logger = slog.New(slog.NewJSONHandler(&logs, nil))

	started := time.Now().UTC().Add(-2 * time.Second)
	if !eng.tryMarkRunStart(started, runreason.ReasonScheduledDue) {
		t.Fatal("tryMarkRunStart returned false")
	}
	eng.setRunPhase(RunPhaseSources)
	eng.observeRunOperation("sources.parse_feed_body", 10*time.Millisecond)
	eng.observeRunCounter("retention.reconcile.cohorts_processed", 2, 0)
	op := eng.beginActiveOperation("sources.update_retention", "sample", "update", "items", 10)
	op.Update(4, 10, map[string]int64{"processed_cohorts": 2})

	report := &Report{
		StartedAt: started,
		EndedAt:   started.Add(2 * time.Second),
		Updated:   []string{"sample"},
	}
	diag := eng.newEngineRunDiagnostics(runreason.ReasonScheduledDue, RunOptions{}, started)
	eng.logRunDiagnosticSummary(report, nil, diag)

	entry := findJSONLogByMessage(t, logs.String(), "engine run diagnostic summary")
	if got := entry["updated"]; got != float64(1) {
		t.Fatalf("updated = %v, want 1", got)
	}
	if got := entry["phase"]; got != string(RunPhaseSources) {
		t.Fatalf("phase = %v, want %q", got, RunPhaseSources)
	}
	assertJSONStatName(t, entry["operations"], "sources.parse_feed_body")
	assertJSONStatName(t, entry["counters"], "retention.reconcile.cohorts_processed")
	assertJSONOperation(t, entry["active_operations"], "sources.update_retention", "sample")
	assertJSONStatName(t, entry["phases"], "sources")
	if _, ok := entry["runtime"].(map[string]any); !ok {
		t.Fatalf("runtime field = %T, want object", entry["runtime"])
	}
	if _, ok := entry["runtime_delta"].(map[string]any); !ok {
		t.Fatalf("runtime_delta field = %T, want object", entry["runtime_delta"])
	}
}

func TestRunDiagnosticSummaryIncludesPhaseActiveOperation(t *testing.T) {
	var logs bytes.Buffer
	eng := newEngineFixture(t)
	eng.logger = slog.New(slog.NewJSONHandler(&logs, nil))

	started := time.Now().UTC().Add(-2 * time.Second)
	if !eng.tryMarkRunStart(started, runreason.ReasonScheduledDue) {
		t.Fatal("tryMarkRunStart returned false")
	}
	eng.setRunPhase(RunPhaseMetadata)
	op := eng.beginActiveOperation("metadata.write_per_feed_outputs", "", "write", "feeds", 10)
	op.Add(5, 10, nil)

	report := &Report{
		StartedAt: started,
		EndedAt:   started.Add(2 * time.Second),
	}
	diag := eng.newEngineRunDiagnostics(runreason.ReasonScheduledDue, RunOptions{}, started)
	eng.logRunDiagnosticSummary(report, nil, diag)

	entry := findJSONLogByMessage(t, logs.String(), "engine run diagnostic summary")
	assertJSONOperation(t, entry["active_operations"], "metadata.write_per_feed_outputs", "")
}

func TestRunDiagnosticsUseCachedRuntimeStats(t *testing.T) {
	eng := newEngineFixture(t)
	eng.runtimeStatsMu.Lock()
	eng.runtimeStats = engineRuntimeStats{
		Goroutines: 123,
		GoMemLimit: 456,
	}
	eng.runtimeStatsSampledAt = time.Now().UTC()
	eng.runtimeStatsMu.Unlock()

	diag := eng.newEngineRunDiagnostics(runreason.ReasonScheduledDue, RunOptions{}, time.Now().UTC())
	if diag.startStats.Goroutines != 123 {
		t.Fatalf("diagnostic goroutines = %d, want cached value 123", diag.startStats.Goroutines)
	}
	if diag.startStats.GoMemLimit != 456 {
		t.Fatalf("diagnostic GoMemLimit = %d, want cached value 456", diag.startStats.GoMemLimit)
	}
}

func TestEngineRuntimeStatsSamplerRecoversPanic(t *testing.T) {
	eng := newEngineFixture(t)
	var calls atomic.Int32
	restore := setEngineRuntimeStatsCaptureForTest(func() runtimeinfo.Snapshot {
		if calls.Add(1) == 1 {
			panic("forced engine runtime stats panic")
		}
		return runtimeinfo.Snapshot{
			Goroutines: 77,
			GoMemLimit: 99,
		}
	})
	t.Cleanup(restore)

	eng.refreshEngineRuntimeStatsSafely()
	eng.refreshEngineRuntimeStatsSafely()

	stats := eng.cachedEngineRuntimeStats()
	if stats.Goroutines != 77 || stats.GoMemLimit != 99 {
		t.Fatalf("cached engine runtime stats = %+v, want recovered second sample", stats)
	}
}

func TestReconcileRetentionCohortsLogsExactAccounting(t *testing.T) {
	var logs bytes.Buffer
	root := t.TempDir()
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.LibDir = filepath.Join(root, "lib")
	}))
	eng.logger = slog.New(slog.NewJSONHandler(&logs, nil))
	paths, err := eng.prepareRetentionUpdatePaths("sample")
	if err != nil {
		t.Fatalf("prepareRetentionUpdatePaths() error = %v", err)
	}
	addedAt := int64(1_700_000_000)
	writeRetentionTestCohort(t, paths.newDir, addedAt, []iprange.Range{{Lo: 10, Hi: 19}})
	writeRetentionTestCohort(t, paths.newDir, addedAt+3600, []iprange.Range{{Lo: 30, Hi: 39}})
	if err := writeFileAtomic(filepath.Join(paths.newDir, ".tmp-123"), []byte("partial"), generatedFileMode); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := writeFileAtomic(filepath.Join(paths.newDir, "not-a-timestamp"), []byte("bad"), generatedFileMode); err != nil {
		t.Fatalf("write malformed file: %v", err)
	}

	current := iprange.New("sample")
	if err := current.AddRange(iprange.Range{Lo: 15, Hi: 19}); err != nil {
		t.Fatalf("current AddRange() error = %v", err)
	}
	current.Optimize()

	result, err := eng.reconcileRetentionCohorts(t.Context(), "sample", paths, addedAt-3600, addedAt+7200, current, map[int]uint64{})
	if err != nil {
		t.Fatalf("reconcileRetentionCohorts() error = %v", err)
	}
	if got, want := result.cohorts[addedAt], uint64(5); got != want {
		t.Fatalf("kept cohort count = %d, want %d", got, want)
	}
	if _, ok := result.cohorts[addedAt+3600]; ok {
		t.Fatalf("fully removed cohort still present in result: %+v", result.cohorts)
	}

	entry := findJSONLogByMessage(t, logs.String(), "retention reconcile summary")
	assertJSONNumber(t, entry, "work_size", 4)
	assertJSONNumber(t, entry, "work_completed", 4)
	assertJSONNumber(t, entry, "completion_pct", 100)
	if got := entry["work_unit"]; got != "files" {
		t.Fatalf("work_unit = %v, want files", got)
	}
	if got, ok := entry["rate_per_second"].(float64); !ok || got <= 0 {
		t.Fatalf("rate_per_second = %v, want positive number", entry["rate_per_second"])
	}
	assertJSONNumber(t, entry, "total_entries", 4)
	assertJSONNumber(t, entry, "scanned_entries", 4)
	assertJSONNumber(t, entry, "skipped_entries", 2)
	assertJSONNumber(t, entry, "malformed_entries", 1)
	assertJSONNumber(t, entry, "processed_cohorts", 2)
	assertJSONNumber(t, entry, "rewritten_cohorts", 1)
	assertJSONNumber(t, entry, "deleted_cohorts", 1)
	assertJSONNumber(t, entry, "input_ips", 20)
	assertJSONNumber(t, entry, "kept_ips", 5)
	assertJSONNumber(t, entry, "removed_ips", 15)
}

func TestFeedProcessingSummaryLogsWorkSizeAndRates(t *testing.T) {
	var logs bytes.Buffer
	eng := newEngineFixture(t)
	eng.logger = slog.New(slog.NewJSONHandler(&logs, nil))
	if !eng.tryMarkRunStart(time.Now().UTC(), runreason.ReasonScheduledDue) {
		t.Fatal("tryMarkRunStart returned false")
	}
	eng.setRunPhase(RunPhaseSources)
	eng.observeFeedOperation("sample", "sources.parse_feed_body", 25*time.Millisecond)
	result := processingOK("updated successfully", true).withWork(FeedProcessingWork{
		InputBytes: 1024,
		Entries:    20,
		UniqueIPs:  10,
	})
	elapsed := 2 * time.Second
	eng.observeFeedWork("sample", result, elapsed)
	eng.logFeedProcessingSummary("sample", elapsed, result)

	entry := findJSONLogByMessage(t, logs.String(), "feed processing summary")
	if got := entry["source"]; got != "sample" {
		t.Fatalf("source = %v, want sample", got)
	}
	assertJSONNumber(t, entry, "input_bytes", 1024)
	assertJSONNumber(t, entry, "entries", 20)
	assertJSONNumber(t, entry, "unique_ips", 10)
	assertJSONNumber(t, entry, "elapsed_ms", 2000)
	assertJSONNumber(t, entry, "input_bytes_per_second", 512)
	assertJSONNumber(t, entry, "entries_per_second", 10)
	assertJSONNumber(t, entry, "unique_ips_per_second", 5)
	assertJSONStatName(t, entry["operations"], "sources.parse_feed_body")
}

func writeRetentionTestCohort(t *testing.T, dir string, addedAt int64, ranges []iprange.Range) {
	t.Helper()
	set := iprange.New("sample")
	for _, r := range ranges {
		if err := set.AddRange(r); err != nil {
			t.Fatalf("AddRange(%v) error = %v", r, err)
		}
	}
	set.Optimize()
	if err := writeBinaryPath(filepath.Join(dir, strconvFormatInt(addedAt)), set, time.Unix(addedAt, 0).UTC()); err != nil {
		t.Fatalf("write cohort %d: %v", addedAt, err)
	}
}

func findJSONLogByMessage(t *testing.T, body, message string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		if entry["msg"] == message {
			return entry
		}
	}
	t.Fatalf("log message %q not found in:\n%s", message, body)
	return nil
}

func assertJSONStatName(t *testing.T, value any, want string) {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("stats field = %T, want array", value)
	}
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if ok && (obj["name"] == want || obj["phase"] == want) {
			return
		}
	}
	t.Fatalf("stat %q not found in %+v", want, value)
}

func assertJSONOperation(t *testing.T, value any, wantOperation, wantFeed string) {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("active operations field = %T, want array", value)
	}
	for _, item := range items {
		obj, ok := item.(map[string]any)
		feed, _ := obj["feed"].(string)
		if ok && obj["operation"] == wantOperation && feed == wantFeed {
			if obj["unit"] == "" {
				t.Fatalf("operation %q/%q missing unit in %+v", wantOperation, wantFeed, obj)
			}
			if _, ok := obj["completion_pct"]; !ok {
				t.Fatalf("operation %q/%q missing completion_pct in %+v", wantOperation, wantFeed, obj)
			}
			if _, ok := obj["rate_per_second"]; !ok {
				t.Fatalf("operation %q/%q missing rate_per_second in %+v", wantOperation, wantFeed, obj)
			}
			return
		}
	}
	t.Fatalf("operation %q/%q not found in %+v", wantOperation, wantFeed, value)
}

func assertJSONNumber(t *testing.T, entry map[string]any, key string, want float64) {
	t.Helper()
	if got := entry[key]; got != want {
		t.Fatalf("%s = %v, want %.0f", key, got, want)
	}
}

func strconvFormatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
