package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
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
	counters = map[string]otelmetric.Int64Counter{}
	histograms = map[string]otelmetric.Float64Histogram{}
	gauges = map[string]otelmetric.Int64Gauge{}
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
