package observability

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLocalCountersRemainExactWithoutExporter(t *testing.T) {
	resetMetricsForTest()

	const goroutines = 8
	const perGoroutine = 1000
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for n := 0; n < perGoroutine; n++ {
				TryCount("download.fetches", 1, String("download.status", "ok"))
			}
		}()
	}
	wg.Wait()

	got := counterValue(t, "download.fetches", map[string]string{"download.status": "ok"})
	want := int64(goroutines * perGoroutine)
	if got != want {
		t.Fatalf("download.fetches counter = %d, want exact %d", got, want)
	}
}

func TestLocalMetricsMapRuntimeValuesToFiniteBuckets(t *testing.T) {
	resetMetricsForTest()

	TryAPIRecalculation("tenant-a", "dynamic-action", "200", 7)
	TryHTTPServerRequest("/runtime/path", "PATCH", 599, time.Second)
	TryCount("download.fetches", 1, String("download.status", "skipped"))
	TryCount("web.artifact.cache.lookups", 1, String("cache.result", "oversize"))
	TryCount("web.artifact.cache.evictions", 1, String("cache.reason", "max_bytes"))
	TryCount("web.artifact.cache.evictions", 1, String("cache.reason", "max_entries"))
	TryCount("background.tasks", 1, String("background.result", "unknown"))
	TryCount("config.loads", 1, String("config.result", "unknown"))
	TryCount("runtime.cache.operations", 1, String("cache.operation", "unknown"), String("cache.result", "unknown"))
	TryCount("integrity.checks", 1, String("integrity.kind", "unknown"), String("integrity.result", "unknown"))
	TryCount("integrity.recovery.targets", 1, String("integrity.kind", "unknown"), String("integrity.action", "unknown"))

	for _, tt := range []struct {
		name   string
		labels map[string]string
		want   int64
	}{
		{
			name: "api.recalculation.requests",
			labels: map[string]string{
				"api.surface": "other",
				"api.action":  "other",
				"api.result":  "other",
			},
			want: 1,
		},
		{
			name: "api.recalculation.targets",
			labels: map[string]string{
				"api.surface": "other",
				"api.action":  "other",
				"api.result":  "other",
			},
			want: 7,
		},
	} {
		if got := counterValue(t, tt.name, tt.labels); got != tt.want {
			t.Fatalf("%s = %d, want %d", tt.name, got, tt.want)
		}
	}

	duration := durationValue(t, "http.server.request.duration", map[string]string{
		"http.route":                "other",
		"http.request.method":       "other",
		"http.response.status_code": "other",
	})
	if duration.Count != 1 {
		t.Fatalf("HTTP runtime labels were not mapped to the finite other bucket: %#v", duration)
	}

	if got := counterValue(t, "download.fetches", map[string]string{"download.status": "skipped"}); got != 1 {
		t.Fatalf("download skipped status = %d, want first-class finite bucket count 1", got)
	}
	if got := counterValue(t, "web.artifact.cache.lookups", map[string]string{"cache.result": "oversize"}); got != 1 {
		t.Fatalf("cache oversize result = %d, want first-class finite bucket count 1", got)
	}
	for _, reason := range []string{"max_bytes", "max_entries"} {
		if got := counterValue(t, "web.artifact.cache.evictions", map[string]string{"cache.reason": reason}); got != 1 {
			t.Fatalf("cache eviction reason %q = %d, want first-class finite bucket count 1", reason, got)
		}
	}
	for _, tt := range []struct {
		name   string
		labels map[string]string
	}{
		{name: "background.tasks", labels: map[string]string{"background.component": "other", "background.result": "unknown"}},
		{name: "config.loads", labels: map[string]string{"config.result": "unknown"}},
		{name: "runtime.cache.operations", labels: map[string]string{"cache.operation": "unknown", "cache.result": "unknown"}},
		{name: "integrity.checks", labels: map[string]string{"integrity.kind": "unknown", "integrity.result": "unknown"}},
		{name: "integrity.recovery.targets", labels: map[string]string{"integrity.kind": "unknown", "integrity.action": "unknown"}},
	} {
		if got := counterValue(t, tt.name, tt.labels); got != 1 {
			t.Fatalf("%s labels %#v = %d, want first-class finite unknown bucket count 1", tt.name, tt.labels, got)
		}
	}
	for _, result := range []string{"issues", "in_progress", "scheduled"} {
		TryCount("integrity.checks", 1, String("integrity.kind", "pipeline"), String("integrity.result", result))
		if got := counterValue(t, "integrity.checks", map[string]string{"integrity.kind": "pipeline", "integrity.result": result}); got != 1 {
			t.Fatalf("integrity.checks result %q = %d, want first-class finite bucket count 1", result, got)
		}
	}
	TryCount("integrity.recovery.targets", 1, String("integrity.kind", "pipeline"), String("integrity.action", "recheck"))
	if got := counterValue(t, "integrity.recovery.targets", map[string]string{"integrity.kind": "pipeline", "integrity.action": "recheck"}); got != 1 {
		t.Fatalf("integrity.recovery.targets recheck action = %d, want first-class finite bucket count 1", got)
	}
}

func TestUnknownMetricNamesIncrementSafetyCounter(t *testing.T) {
	resetMetricsForTest()

	TryCount("dynamic.metric.name", 1)

	if got := counterValue(t, "telemetry.metrics.unknown", nil); got != 1 {
		t.Fatalf("telemetry.metrics.unknown = %d, want 1", got)
	}
}

func TestPrometheusHandlerServesLocalSnapshot(t *testing.T) {
	resetMetricsForTest()

	TryCount("download.fetches", 2, String("download.status", "ok"))
	TryBytes("download.fetch", 512, String("download.status", "ok"))
	TryDuration("download.fetch", 1500*time.Microsecond, String("download.status", "ok"))
	TryGauge("daemon.up", 1)
	TryHTTPServerRequest("/healthz", http.MethodGet, http.StatusNoContent, 2*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	newPrometheusMetrics().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"download_fetches_total{download_status=\"ok\"} 2",
		"download_fetch_bytes_total{download_status=\"ok\"} 512",
		"download_fetch_duration_ms_count{download_status=\"ok\"} 1",
		"download_fetch_duration_ms_sum{download_status=\"ok\"} 1.5",
		"daemon_up 1",
		"http_server_request_duration_count{http_route=\"/healthz\",http_request_method=\"GET\",http_response_status_code=\"204\"} 1",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("/metrics body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "feed.name") || strings.Contains(body, "tenant-a") {
		t.Fatalf("/metrics exposed runtime identity:\n%s", body)
	}
}

func TestRepresentativeMetricHotPathsDoNotAllocate(t *testing.T) {
	resetMetricsForTest()

	assertZeroAllocs(t, "counter", func() {
		TryCount("download.fetches", 1, String("download.status", "ok"))
	})
	assertZeroAllocs(t, "gauge", func() {
		TryGauge("engine.running", 1)
	})
	assertZeroAllocs(t, "duration", func() {
		TryDuration("download.fetch", time.Millisecond, String("download.status", "ok"))
	})
	assertZeroAllocs(t, "prebuilt attr slice", func() {
		attrs := []Attr{
			String("integrity.kind", "pipeline"),
			String("integrity.result", "clean"),
		}
		TryCount("integrity.checks", 1, attrs...)
		TryDuration("integrity.check", time.Millisecond, attrs...)
	})
	assertZeroAllocs(t, "span", func() {
		_, span := Start(context.Background(), "test.operation")
		End(span, nil)
	})
	handler := newAsyncLogHandler(slog.DiscardHandler, 2048)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		_ = handler.Shutdown(ctx)
	})
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	assertZeroAllocs(t, "log enqueue", func() {
		_ = handler.Handle(context.Background(), record)
	})
	recordWithAttrs := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	recordWithAttrs.AddAttrs(
		slog.String("download.status", "ok"),
		slog.String("trace.value", "bounded"),
	)
	assertZeroAllocs(t, "log enqueue with attrs", func() {
		_ = handler.Handle(context.Background(), recordWithAttrs)
	})
	assertZeroAllocs(t, "trace with attr", func() {
		_, span := Start(context.Background(), "test.operation", String("download.status", "ok"))
		End(span, nil)
	})
	assertZeroAllocs(t, "trace with five attrs", func() {
		_, span := Start(context.Background(), "test.operation",
			String("run.reason", "manual_run"),
			Bool("run.recheck", true),
			Bool("run.reprocess", false),
			Bool("run.manual", true),
			Int("run.selected", 5),
		)
		End(span, nil)
	})
}

func assertZeroAllocs(t *testing.T, name string, fn func()) {
	t.Helper()
	if got := testing.AllocsPerRun(1000, fn); got != 0 {
		t.Fatalf("%s hot path allocations = %f, want 0", name, got)
	}
}

func counterValue(t *testing.T, name string, labels map[string]string) int64 {
	t.Helper()
	for _, snap := range SnapshotMetrics() {
		if snap.Name == name && matchLabels(snap.Labels, labels) {
			return snap.Value
		}
	}
	t.Fatalf("counter %s labels %#v not found in snapshot %#v", name, labels, SnapshotMetrics())
	return 0
}

func optionalCounterValue(name string, labels map[string]string) int64 {
	for _, snap := range SnapshotMetrics() {
		if snap.Name == name && matchLabels(snap.Labels, labels) {
			return snap.Value
		}
	}
	return 0
}

func waitForCounterAtLeast(t *testing.T, name string, labels map[string]string, want int64) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := optionalCounterValue(name, labels); got >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("counter %s labels %#v did not reach %d; got %d", name, labels, want, optionalCounterValue(name, labels))
}

func durationValue(t *testing.T, name string, labels map[string]string) MetricSnapshot {
	t.Helper()
	for _, snap := range SnapshotMetrics() {
		if snap.Name == name && matchLabels(snap.Labels, labels) {
			return snap
		}
	}
	t.Fatalf("duration %s labels %#v not found in snapshot %#v", name, labels, SnapshotMetrics())
	return MetricSnapshot{}
}

func matchLabels(got []MetricLabelSnapshot, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, label := range got {
		if want[label.Key] != label.Value {
			return false
		}
	}
	return true
}

func assertMetricsHandlerContains(t *testing.T, handler http.Handler, want string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, want) {
		t.Fatalf("/metrics body missing %q:\n%s", want, body)
	}
}
