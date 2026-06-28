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
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestEnabledFromEnv(t *testing.T) {
	if enabledFromEnv() {
		t.Fatal("enabledFromEnv() = true without opt-in env")
	}

	t.Setenv("UPDATE_IPSETS_OTEL", "disabled")
	if enabledFromEnv() {
		t.Fatal("enabledFromEnv() ignored disabled local env")
	}

	t.Setenv("UPDATE_IPSETS_OTEL", "enabled")
	if !enabledFromEnv() {
		t.Fatal("enabledFromEnv() ignored enabled local env")
	}

	t.Setenv("UPDATE_IPSETS_OTEL", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318")
	if !enabledFromEnv() {
		t.Fatal("enabledFromEnv() ignored OTLP endpoint")
	}
}

func TestMetricReaderOptionsFromEnv(t *testing.T) {
	opts, err := metricReaderOptionsFromEnv()
	if err != nil {
		t.Fatalf("metricReaderOptionsFromEnv() error = %v", err)
	}
	if len(opts) != 0 {
		t.Fatalf("metricReaderOptionsFromEnv() len = %d, want 0", len(opts))
	}

	t.Setenv("UPDATE_IPSETS_OTEL_METRIC_INTERVAL", "250ms")
	opts, err = metricReaderOptionsFromEnv()
	if err != nil {
		t.Fatalf("metricReaderOptionsFromEnv() error = %v", err)
	}
	if len(opts) != 1 {
		t.Fatalf("metricReaderOptionsFromEnv() len = %d, want 1", len(opts))
	}

	t.Setenv("UPDATE_IPSETS_OTEL_METRIC_INTERVAL", "0")
	if _, err := metricReaderOptionsFromEnv(); err == nil {
		t.Fatal("metricReaderOptionsFromEnv() error = nil, want error")
	}
}

func TestShutdownAllAndSetupShutdown(t *testing.T) {
	if err := ((*Setup)(nil)).Shutdown(t.Context()); err != nil {
		t.Fatalf("nil Setup.Shutdown() error = %v", err)
	}
	if err := (&Setup{}).Shutdown(t.Context()); err != nil {
		t.Fatalf("empty Setup.Shutdown() error = %v", err)
	}

	var called bool
	setup := &Setup{shutdown: func(context.Context) error {
		called = true
		return nil
	}}
	if err := setup.Shutdown(t.Context()); err != nil {
		t.Fatalf("Setup.Shutdown() error = %v", err)
	}
	if !called {
		t.Fatal("Setup.Shutdown() did not invoke shutdown function")
	}

	errFirst := errors.New("first")
	errSecond := errors.New("second")
	order := make([]int, 0, 2)
	err := shutdownAll(t.Context(), []func(context.Context) error{
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

func TestInitFailsOpenWhenOTLPProtocolInvalid(t *testing.T) {
	t.Setenv("UPDATE_IPSETS_OTEL", "1")
	t.Setenv("UPDATE_IPSETS_OTEL_PROTOCOL", "invalid")

	var logs bytes.Buffer
	setup, err := Init(t.Context(), "test-service", "test-version", slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatalf("Init() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := setup.Shutdown(t.Context()); err != nil {
			t.Fatalf("Setup.Shutdown() error = %v", err)
		}
	})
	if setup.Enabled {
		t.Fatal("Setup.Enabled = true, want false when OTLP protocol is invalid")
	}
	if setup.PrometheusHandler == nil {
		t.Fatal("PrometheusHandler = nil, want local metrics after OTLP setup failure")
	}
	Gauge(t.Context(), "daemon.up", 1)
	assertMetricsHandlerContains(t, setup.PrometheusHandler, "daemon_up")
	if !strings.Contains(logs.String(), "OTLP export disabled") {
		t.Fatalf("Init() logs did not report OTLP degradation:\n%s", logs.String())
	}
}

func TestInitFailsOpenWhenOTLPMetricExporterCannotStart(t *testing.T) {
	t.Setenv("UPDATE_IPSETS_OTEL", "1")
	t.Setenv("UPDATE_IPSETS_OTEL_PROTOCOL", "grpc")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_LOGS_EXPORTER", "none")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var logs bytes.Buffer
	setup, err := Init(ctx, "test-service", "test-version", slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatalf("Init() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := setup.Shutdown(t.Context()); err != nil {
			t.Fatalf("Setup.Shutdown() error = %v", err)
		}
	})
	if setup.Enabled {
		t.Fatal("Setup.Enabled = true, want false when the only OTLP signal cannot start")
	}
	if setup.PrometheusHandler == nil {
		t.Fatal("PrometheusHandler = nil, want local metrics after OTLP exporter failure")
	}
	Gauge(t.Context(), "daemon.up", 1)
	assertMetricsHandlerContains(t, setup.PrometheusHandler, "daemon_up")
	if !strings.Contains(logs.String(), "startup budget expired") {
		t.Fatalf("Init() logs did not report startup budget degradation:\n%s", logs.String())
	}
}

func TestTracingHelpersHandleDefaults(t *testing.T) {
	ctx, span := Start(context.Background(), "", attribute.String("component", "test"))
	if ctx == nil {
		t.Fatal("Start(nil) returned nil context")
	}
	End(span, nil)
	End(nil, errors.New("ignored"))
	if BackgroundContext() == nil {
		t.Fatal("BackgroundContext() returned nil")
	}
}

func TestMetricHelpersRecordDesignedInstruments(t *testing.T) {
	handler, restore := installTestMeter(t)
	defer restore()

	ctx := context.Background()
	Count(ctx, "", 1)
	Count(ctx, "download.fetches", 1, attribute.String("download.status", "ok"))
	Bytes(ctx, "download.fetch", 128, attribute.String("download.status", "ok"))
	Duration(ctx, "download.fetch", 2*time.Millisecond, attribute.String("download.status", "ok"))
	Observe(ctx, "processor.runs", 1, 64, 3*time.Millisecond, attribute.String("processor.mode", "stream"))
	Gauge(ctx, "daemon.up", 1)
	Gauge(ctx, "", 1)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"download_fetches_total",
		"download_fetch_bytes_total",
		"download_fetch_duration_ms_milliseconds",
		"processor_runs_total",
		"daemon_up",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("/metrics body missing %q:\n%s", want, body)
		}
	}
}

func TestMetricHelpersDoNotCreateInstrumentsOnRecord(t *testing.T) {
	_, restore := installTestMeter(t)
	defer restore()

	instrumentsMu.RLock()
	counterCount := len(counters)
	histogramCount := len(histograms)
	gaugeCount := len(gauges)
	instrumentsMu.RUnlock()

	Count(t.Context(), "download.fetches", 1)
	Duration(t.Context(), "download.fetch", time.Millisecond)
	Gauge(t.Context(), "daemon.up", 1)
	Count(t.Context(), "not.designed", 1)
	Duration(t.Context(), "not.designed", time.Millisecond)
	Gauge(t.Context(), "not.designed", 1)

	instrumentsMu.RLock()
	defer instrumentsMu.RUnlock()
	if len(counters) != counterCount || len(histograms) != histogramCount || len(gauges) != gaugeCount {
		t.Fatalf("metric recording mutated instrument registry: counters %d->%d histograms %d->%d gauges %d->%d",
			counterCount, len(counters), histogramCount, len(histograms), gaugeCount, len(gauges))
	}
}

func TestAPIRecalculationMetricsExposeBoundedLabels(t *testing.T) {
	handler, restore := installTestMeter(t)
	defer restore()

	APIRecalculation(t.Context(), "public", "compose", "ok", 3)
	APIRecalculation(t.Context(), "feed-name", "feed/example", "200", 4)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, tt := range []struct {
		name   string
		labels []string
		value  string
	}{
		{name: "api_recalculation_requests_total", labels: []string{`api_action="compose"`, `api_result="ok"`, `api_surface="public"`}, value: "1"},
		{name: "api_recalculation_targets_total", labels: []string{`api_action="compose"`, `api_result="ok"`, `api_surface="public"`}, value: "3"},
		{name: "api_recalculation_requests_total", labels: []string{`api_action="other"`, `api_result="other"`, `api_surface="other"`}, value: "1"},
		{name: "api_recalculation_targets_total", labels: []string{`api_action="other"`, `api_result="other"`, `api_surface="other"`}, value: "4"},
	} {
		if !metricLineContains(body, tt.name, tt.labels, tt.value) {
			t.Fatalf("/metrics body missing %s with labels %v and value %s:\n%s", tt.name, tt.labels, tt.value, body)
		}
	}
	for _, forbidden := range []string{"feed-name", "feed/example", `api_result="200"`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("/metrics body exposed unbounded recalculation label %q:\n%s", forbidden, body)
		}
	}
}

func TestHTTPServerRequestMetricUsesProjectOwnedInstrument(t *testing.T) {
	handler, restore := installTestMeter(t)
	defer restore()

	recordHTTPServerRequest(t.Context(), "/healthz", http.MethodGet, http.StatusNoContent, 2*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"http_server_request_duration_seconds",
		`http_request_method="GET"`,
		`http_response_status_code="204"`,
		`http_route="/healthz"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("/metrics body missing %q:\n%s", want, body)
		}
	}
}

func TestAsyncMetricTryHelpersDropWhenQueueFull(t *testing.T) {
	asyncMetrics = make(chan asyncMetric, 1)
	asyncMetrics <- asyncMetric{kind: asyncMetricCount, name: "download.fetches", count: 1}
	asyncMetricsStarted.Store(true)
	t.Cleanup(func() {
		asyncMetrics = make(chan asyncMetric, asyncMetricQueueSize)
		asyncMetricsStarted.Store(false)
	})

	done := make(chan struct{})
	go func() {
		TryCount("download.fetches", 1)
		TryDuration("download.fetch", time.Millisecond)
		TryGauge("daemon.up", 1)
		TryHTTPServerRequest("/healthz", http.MethodGet, http.StatusOK, time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("async metric Try helpers blocked behind a full telemetry queue")
	}
}

func TestTeeHandlerFanout(t *testing.T) {
	var infoOut bytes.Buffer
	var errorOut bytes.Buffer
	handler := newTeeHandler(
		nil,
		slog.NewTextHandler(&infoOut, &slog.HandlerOptions{Level: slog.LevelInfo}),
		slog.NewTextHandler(&errorOut, &slog.HandlerOptions{Level: slog.LevelError}),
	)

	if handler.Enabled(t.Context(), slog.LevelDebug) {
		t.Fatal("tee handler enabled debug unexpectedly")
	}
	if !handler.Enabled(t.Context(), slog.LevelInfo) {
		t.Fatal("tee handler disabled info unexpectedly")
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "message", 0)
	record.AddAttrs(slog.String("key", "value"))
	if err := handler.Handle(t.Context(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !strings.Contains(infoOut.String(), "message") || !strings.Contains(infoOut.String(), "key=value") {
		t.Fatalf("info handler output = %q", infoOut.String())
	}
	if errorOut.Len() != 0 {
		t.Fatalf("error handler received info record: %q", errorOut.String())
	}

	grouped := handler.WithAttrs([]slog.Attr{slog.String("component", "test")}).WithGroup("group")
	errorRecord := slog.NewRecord(time.Now(), slog.LevelError, "failed", 0)
	errorRecord.AddAttrs(slog.String("reason", "boom"))
	if err := grouped.Handle(t.Context(), errorRecord); err != nil {
		t.Fatalf("grouped Handle() error = %v", err)
	}
	if !strings.Contains(errorOut.String(), "failed") || !strings.Contains(errorOut.String(), "group.reason=boom") {
		t.Fatalf("error handler output = %q", errorOut.String())
	}
}

func TestAsyncLogHandlerDropsWhenExporterBlocked(t *testing.T) {
	blocking := newBlockingSlogHandler()
	handler := newAsyncLogHandler(blocking, 1)
	t.Cleanup(func() {
		blocking.release()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = handler.Shutdown(ctx)
	})

	done := make(chan struct{})
	go func() {
		for i := 0; i < 32; i++ {
			record := slog.NewRecord(time.Now(), slog.LevelInfo, "message", 0)
			_ = handler.Handle(t.Context(), record)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("async log handler blocked caller behind blocked exporter")
	}
}

func TestAsyncLogHandlerIgnoresRecordsAfterShutdown(t *testing.T) {
	var out bytes.Buffer
	handler := newAsyncLogHandler(slog.NewTextHandler(&out, nil), 1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := handler.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		for i := 0; i < 32; i++ {
			record := slog.NewRecord(time.Now(), slog.LevelInfo, "late", 0)
			_ = handler.Handle(t.Context(), record)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("async log handler blocked late records after shutdown")
	}
}

type blockingSlogHandler struct {
	releaseOnce sync.Once
	releaseCh   chan struct{}
}

func newBlockingSlogHandler() *blockingSlogHandler {
	return &blockingSlogHandler{
		releaseCh: make(chan struct{}),
	}
}

func (h *blockingSlogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *blockingSlogHandler) Handle(context.Context, slog.Record) error {
	<-h.releaseCh
	return nil
}

func (h *blockingSlogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *blockingSlogHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *blockingSlogHandler) release() {
	h.releaseOnce.Do(func() {
		close(h.releaseCh)
	})
}

func installTestMeter(t *testing.T) (http.Handler, func()) {
	t.Helper()
	exporter, handler, err := newPrometheusMetrics()
	if err != nil {
		t.Fatalf("newPrometheusMetrics() error = %v", err)
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithView(metricCardinalityView()),
	)
	t.Cleanup(func() {
		if err := provider.Shutdown(t.Context()); err != nil {
			t.Fatalf("shutdown meter provider: %v", err)
		}
	})

	oldMeter := meter
	instrumentsMu.Lock()
	meter = provider.Meter(scopeName)
	oldCounters := counters
	oldHistograms := histograms
	oldGauges := gauges
	counters, histograms, gauges = precreateMetricInstruments(meter)
	instrumentsMu.Unlock()

	restore := func() {
		instrumentsMu.Lock()
		meter = oldMeter
		counters = oldCounters
		histograms = oldHistograms
		gauges = oldGauges
		instrumentsMu.Unlock()
	}
	return handler, restore
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

func metricLineContains(body, name string, labels []string, value string) bool {
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, name+"{") || !strings.HasSuffix(line, " "+value) {
			continue
		}
		matched := true
		for _, label := range labels {
			if !strings.Contains(line, label) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
