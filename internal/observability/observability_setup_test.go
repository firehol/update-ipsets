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
)

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
	if want := int64(50 * 1024 * 1024); logBytes != want || traceBytes != 0 {
		t.Fatalf("default budgets = log %d trace %d, want log %d and trace disabled", logBytes, traceBytes, want)
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
	logBytes, traceBytes, err = telemetryBufferBudgetsFromEnv()
	if err != nil {
		t.Fatalf("telemetryBufferBudgetsFromEnv() zero trace override error = %v", err)
	}
	if logBytes != 4*1024 || traceBytes != 0 {
		t.Fatalf("zero trace override budgets = log %d trace %d, want 4096 and 0", logBytes, traceBytes)
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
	_, span := Start(context.Background(), "default.trace.disabled")
	End(span, nil)
	if events := SnapshotTraceEvents(); len(events) != 0 {
		t.Fatalf("SnapshotTraceEvents() length = %d, want no default trace capture", len(events))
	}
	if got := optionalCounterValue("telemetry.traces.dropped", nil); got != 0 {
		t.Fatalf("telemetry.traces.dropped = %d, want no trace drops while default trace capture is disabled", got)
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

func TestInitEnablesTraceCaptureWithExplicitTraceBuffer(t *testing.T) {
	resetMetricsForTest()
	t.Setenv("UPDATE_IPSETS_TRACE_BUFFER_BYTES", "64KB")

	var logs bytes.Buffer
	setup, err := Init(t.Context(), "test-service", "test-version", slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		_ = setup.Shutdown(ctx)
	})

	_, span := Start(context.Background(), "enabled.trace", String("status", "ok"))
	End(span, nil)
	if events := SnapshotTraceEvents(); len(events) != 2 {
		t.Fatalf("SnapshotTraceEvents() length = %d, want start and end events", len(events))
	}
}
