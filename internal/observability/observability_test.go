package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
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

func TestProtocolAndSignalEnvironmentParsing(t *testing.T) {
	for _, key := range []string{
		"UPDATE_IPSETS_OTEL",
		"UPDATE_IPSETS_OTEL_PROTOCOL",
		"UPDATE_IPSETS_OTEL_METRIC_INTERVAL",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_PROTOCOL",
		"OTEL_METRIC_EXPORT_INTERVAL",
	} {
		t.Setenv(key, "")
	}
	if enabledFromEnv() {
		t.Fatal("enabledFromEnv() = true without opt-in env")
	}
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://localhost:4318")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "http://localhost:4318")
	if enabledFromEnv() {
		t.Fatal("enabledFromEnv() = true for trace/log-only endpoints; only metric export is supported")
	}
	t.Setenv("UPDATE_IPSETS_OTEL", "enabled")
	if !enabledFromEnv() {
		t.Fatal("enabledFromEnv() ignored UPDATE_IPSETS_OTEL=enabled")
	}
	t.Setenv("UPDATE_IPSETS_OTEL_PROTOCOL", "grpc")
	if got, err := protocolFromEnv(); err != nil || got != otlpProtocolGRPC {
		t.Fatalf("protocolFromEnv() = %q, %v; want grpc, nil", got, err)
	}
	t.Setenv("UPDATE_IPSETS_OTEL_PROTOCOL", "invalid")
	if _, err := protocolFromEnv(); err == nil {
		t.Fatal("protocolFromEnv() error = nil, want invalid protocol error")
	}
	t.Setenv("UPDATE_IPSETS_OTEL_METRIC_INTERVAL", "250ms")
	opts, err := metricReaderOptionsFromEnv()
	if err != nil {
		t.Fatalf("metricReaderOptionsFromEnv() error = %v", err)
	}
	if len(opts) != 1 || opts[0] != 250*time.Millisecond {
		t.Fatalf("metricReaderOptionsFromEnv() = %#v, want 250ms", opts)
	}
}

func TestTelemetryBufferBudgetEnvironmentParsing(t *testing.T) {
	t.Setenv("UPDATE_IPSETS_TELEMETRY_BUFFER_BYTES", "50MiB")
	logBytes, traceBytes, err := telemetryBufferBudgetsFromEnv()
	if err != nil {
		t.Fatalf("telemetryBufferBudgetsFromEnv() error = %v", err)
	}
	if want := int64(25 * 1024 * 1024); logBytes != want || traceBytes != want {
		t.Fatalf("split budgets = log %d trace %d, want %d each", logBytes, traceBytes, want)
	}

	t.Setenv("UPDATE_IPSETS_LOG_BUFFER_BYTES", "4KB")
	t.Setenv("UPDATE_IPSETS_TRACE_BUFFER_BYTES", "8KB")
	logBytes, traceBytes, err = telemetryBufferBudgetsFromEnv()
	if err != nil {
		t.Fatalf("telemetryBufferBudgetsFromEnv() with overrides error = %v", err)
	}
	if logBytes != 4*1024 || traceBytes != 8*1024 {
		t.Fatalf("override budgets = log %d trace %d, want 4096 and 8192", logBytes, traceBytes)
	}

	t.Setenv("UPDATE_IPSETS_TRACE_BUFFER_BYTES", "0")
	if _, _, err := telemetryBufferBudgetsFromEnv(); err == nil {
		t.Fatal("telemetryBufferBudgetsFromEnv() error = nil for zero trace buffer")
	}
}

func TestInitFailsOpenWhenMetricExporterConfigurationIsInvalid(t *testing.T) {
	resetMetricsForTest()
	t.Setenv("UPDATE_IPSETS_OTEL", "1")
	t.Setenv("UPDATE_IPSETS_OTEL_PROTOCOL", "invalid")

	var logs bytes.Buffer
	setup, err := Init(t.Context(), "test-service", "test-version", slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatalf("Init() error = %v, want fail-open nil error", err)
	}
	if setup.Enabled {
		t.Fatal("Setup.Enabled = true after invalid exporter configuration")
	}
	if setup.PrometheusHandler == nil {
		t.Fatal("Setup.PrometheusHandler = nil after invalid exporter configuration")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := setup.Shutdown(ctx); err != nil {
		t.Fatalf("Setup.Shutdown() error = %v", err)
	}
	if !strings.Contains(logs.String(), "opentelemetry export disabled") {
		t.Fatalf("Init() did not log exporter disable warning:\n%s", logs.String())
	}
	TryCount("download.fetches", 1, String("download.status", "ok"))
	if got := counterValue(t, "download.fetches", map[string]string{"download.status": "ok"}); got != 1 {
		t.Fatalf("local metrics after disabled exporter = %d, want exact 1", got)
	}
}

func TestBlockedMetricExporterDoesNotDelayLocalMetricProducers(t *testing.T) {
	resetMetricsForTest()
	blocked := make(chan struct{})
	collectorSeen := make(chan struct{}, 1)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case collectorSeen <- struct{}{}:
		default:
		}
		<-blocked
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(collector.Close)
	t.Setenv("UPDATE_IPSETS_OTEL", "1")
	t.Setenv("UPDATE_IPSETS_OTEL_PROTOCOL", "http/protobuf")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)
	t.Setenv("UPDATE_IPSETS_OTEL_METRIC_INTERVAL", "1ms")

	setup, err := Init(t.Context(), "test-service", "test-version", slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if !setup.Enabled {
		t.Fatal("Init() did not enable metric export")
	}
	t.Cleanup(func() {
		close(blocked)
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		_ = setup.Shutdown(ctx)
	})
	select {
	case <-collectorSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("OTLP collector did not receive a metric export request")
	}

	producersDone := make(chan struct{})
	go func() {
		defer close(producersDone)
		for i := 0; i < 1000; i++ {
			TryCount("download.fetches", 1, String("download.status", "ok"))
		}
	}()
	select {
	case <-producersDone:
	case <-time.After(time.Second):
		t.Fatal("local metric producers blocked behind stalled OTLP export")
	}
	if got := counterValue(t, "download.fetches", map[string]string{"download.status": "ok"}); got != 1000 {
		t.Fatalf("download.fetches while exporter blocked = %d, want 1000", got)
	}
}

func TestAsyncLogHandlerDropsInsteadOfBlocking(t *testing.T) {
	resetMetricsForTest()

	blocking := newBlockingSlogHandler()
	handler := newAsyncLogHandler(blocking, 1)
	logger := slog.New(handler)

	logger.Info("first")
	<-blocking.entered

	logger.Info("second")
	logger.Info("third")

	if got := counterValue(t, "telemetry.logs.dropped", nil); got == 0 {
		t.Fatal("telemetry.logs.dropped = 0, want a dropped log record")
	}
	close(blocking.release)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := handler.Shutdown(ctx); err != nil {
		t.Fatalf("async log handler shutdown: %v", err)
	}
}

func TestAsyncLogHandlerCountsRecordsAfterShutdownAsDropped(t *testing.T) {
	resetMetricsForTest()

	handler := newAsyncLogHandler(slog.DiscardHandler, 1)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := handler.Shutdown(ctx); err != nil {
		t.Fatalf("async log handler shutdown: %v", err)
	}
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "late message", 0)
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if got := counterValue(t, "telemetry.logs.dropped", nil); got != 1 {
		t.Fatalf("telemetry.logs.dropped after shutdown = %d, want 1", got)
	}
}

func TestAsyncLogHandlerTruncatesOversizedStringPayloads(t *testing.T) {
	resetMetricsForTest()

	capture := newCaptureSlogHandler()
	handler := newAsyncLogHandler(capture, 4)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		_ = handler.Shutdown(ctx)
	})
	record := slog.NewRecord(time.Now(), slog.LevelInfo, strings.Repeat("m", maxAsyncLogStringBytes+1), 0)
	record.AddAttrs(slog.String("oversized", strings.Repeat("v", maxAsyncLogStringBytes+1)))
	record.AddAttrs(slog.String(strings.Repeat("k", maxAsyncLogStringBytes+1), "value"))
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	got := capture.wait(t)
	if got.Message != truncatedLogValue {
		t.Fatalf("logged message = %q, want truncated marker", got.Message)
	}
	var value string
	got.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "oversized" {
			value = attr.Value.String()
		}
		if attr.Value.String() == "value" {
			if attr.Key != truncatedLogValue {
				t.Fatalf("logged attr key = %q, want truncated marker", attr.Key)
			}
		}
		return true
	})
	if value != truncatedLogValue {
		t.Fatalf("logged attr value = %q, want truncated marker", value)
	}
}

func TestAsyncLogHandlerRecoversFromDownstreamPanic(t *testing.T) {
	resetMetricsForTest()

	sink := newPanicOnceSlogHandler()
	handler := newAsyncLogHandler(sink, 4)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		_ = handler.Shutdown(ctx)
	})
	logger := slog.New(handler)

	logger.Info("first")
	waitForCounterAtLeast(t, "telemetry.logs.dropped", nil, 1)
	logger.Info("second")

	got := sink.wait(t)
	if got.Message != "second" {
		t.Fatalf("captured log after panic = %q, want second", got.Message)
	}
}

func TestTraceQueueDropsInsteadOfBlocking(t *testing.T) {
	resetMetricsForTest()
	configureTraceQueue(1)

	_, span := Start(context.Background(), "overflow.trace")
	End(span, nil)

	if got := counterValue(t, "telemetry.traces.dropped", nil); got == 0 {
		t.Fatal("telemetry.traces.dropped = 0, want a dropped trace event")
	}
	events := SnapshotTraceEvents()
	if len(events) != 1 {
		t.Fatalf("SnapshotTraceEvents() length = %d, want bounded ring length 1", len(events))
	}
}

func TestTraceQueueTruncatesOversizedStringPayloads(t *testing.T) {
	resetMetricsForTest()
	configureTraceQueue(1 << 20)

	_, span := Start(
		context.Background(),
		strings.Repeat("n", maxTraceStringBytes+1),
		String("oversized", strings.Repeat("v", maxTraceStringBytes+1)),
	)
	End(span, nil)
	events := SnapshotTraceEvents()
	if len(events) != 2 {
		t.Fatalf("SnapshotTraceEvents() length = %d, want 2", len(events))
	}
	if events[0].Name != truncatedTraceValue {
		t.Fatalf("trace name = %q, want truncated marker", events[0].Name)
	}
	if got := events[0].Attrs[0].s; got != truncatedTraceValue {
		t.Fatalf("trace attr value = %q, want truncated marker", got)
	}
}

func TestTraceQueueDoesNotDropBecauseProducersContend(t *testing.T) {
	resetMetricsForTest()
	configureTraceQueue(1 << 20)

	const goroutines = 8
	const perGoroutine = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for n := 0; n < perGoroutine; n++ {
				_, span := Start(context.Background(), "concurrent.trace", String("download.status", "ok"))
				End(span, nil)
			}
		}()
	}
	wg.Wait()
	events := SnapshotTraceEvents()
	wantEvents := goroutines * perGoroutine * 2
	if len(events) != wantEvents {
		t.Fatalf("SnapshotTraceEvents() length = %d, want %d", len(events), wantEvents)
	}
	if got := optionalCounterValue("telemetry.traces.dropped", nil); got != 0 {
		t.Fatalf("telemetry.traces.dropped = %d, want 0 with sufficient queue capacity", got)
	}
}

func TestSetupUsesLocalMetricsAndShutdownOrder(t *testing.T) {
	resetMetricsForTest()

	var logs bytes.Buffer
	setup, err := Init(t.Context(), "test-service", "test-version", slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if setup.Enabled {
		t.Fatal("Setup.Enabled = true before isolated exporter is attached")
	}
	if setup.Logger == nil {
		t.Fatal("Setup.Logger = nil")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		_ = setup.Shutdown(ctx)
	})
	if setup.PrometheusHandler == nil {
		t.Fatal("Setup.PrometheusHandler = nil")
	}
	assertMetricsHandlerContains(t, setup.PrometheusHandler, "daemon_up 1")
	assertMetricsHandlerContains(t, setup.PrometheusHandler, "runtime_go_goroutines")

	errFirst := errors.New("first")
	errSecond := errors.New("second")
	order := make([]int, 0, 2)
	err = shutdownAll(t.Context(), []func(context.Context) error{
		func(context.Context) error {
			order = append(order, 1)
			return errFirst
		},
		nil,
		func(context.Context) error {
			order = append(order, 2)
			return errSecond
		},
	})
	if !errors.Is(err, errFirst) || !errors.Is(err, errSecond) {
		t.Fatalf("shutdownAll() error = %v, want joined errors", err)
	}
	if want := []int{2, 1}; len(order) != len(want) || order[0] != want[0] || order[1] != want[1] {
		t.Fatalf("shutdown order = %#v, want %#v", order, want)
	}
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

type blockingSlogHandler struct {
	entered     chan struct{}
	release     chan struct{}
	enteredOnce sync.Once
}

func newBlockingSlogHandler() *blockingSlogHandler {
	return &blockingSlogHandler{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (h *blockingSlogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *blockingSlogHandler) Handle(context.Context, slog.Record) error {
	h.enteredOnce.Do(func() { close(h.entered) })
	<-h.release
	return nil
}

func (h *blockingSlogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *blockingSlogHandler) WithGroup(string) slog.Handler {
	return h
}

type captureSlogHandler struct {
	records chan slog.Record
}

func newCaptureSlogHandler() *captureSlogHandler {
	return &captureSlogHandler{records: make(chan slog.Record, 1)}
}

func (h *captureSlogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureSlogHandler) Handle(_ context.Context, record slog.Record) error {
	h.records <- record
	return nil
}

func (h *captureSlogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *captureSlogHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *captureSlogHandler) wait(t *testing.T) slog.Record {
	t.Helper()
	select {
	case record := <-h.records:
		return record
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for captured slog record")
		return slog.Record{}
	}
}

type panicOnceSlogHandler struct {
	panicked atomic.Bool
	records  chan slog.Record
}

func newPanicOnceSlogHandler() *panicOnceSlogHandler {
	return &panicOnceSlogHandler{records: make(chan slog.Record, 1)}
}

func (h *panicOnceSlogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *panicOnceSlogHandler) Handle(_ context.Context, record slog.Record) error {
	if !h.panicked.Swap(true) {
		panic("test slog handler panic")
	}
	h.records <- record
	return nil
}

func (h *panicOnceSlogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *panicOnceSlogHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *panicOnceSlogHandler) wait(t *testing.T) slog.Record {
	t.Helper()
	select {
	case record := <-h.records:
		return record
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for recovered slog handler record")
		return slog.Record{}
	}
}
