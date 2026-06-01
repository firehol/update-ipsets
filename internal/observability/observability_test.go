package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestProtocolFromEnv(t *testing.T) {
	t.Setenv("UPDATE_IPSETS_OTEL_PROTOCOL", "grpc")
	got, err := protocolFromEnv()
	if err != nil {
		t.Fatalf("protocolFromEnv() error = %v", err)
	}
	if got != otlpProtocolGRPC {
		t.Fatalf("protocolFromEnv() = %q, want %q", got, otlpProtocolGRPC)
	}

	t.Setenv("UPDATE_IPSETS_OTEL_PROTOCOL", "")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	got, err = protocolFromEnv()
	if err != nil {
		t.Fatalf("protocolFromEnv() error = %v", err)
	}
	if got != otlpProtocolHTTP {
		t.Fatalf("protocolFromEnv() = %q, want %q", got, otlpProtocolHTTP)
	}

	t.Setenv("UPDATE_IPSETS_OTEL_PROTOCOL", "invalid")
	if _, err = protocolFromEnv(); err == nil {
		t.Fatal("protocolFromEnv() error = nil, want error")
	}
}

func TestParseMetricExportInterval(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{name: "milliseconds", raw: "1000", want: time.Second},
		{name: "duration", raw: "2s", want: 2 * time.Second},
		{name: "empty", raw: "", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMetricExportInterval(tt.raw)
			if err != nil {
				t.Fatalf("parseMetricExportInterval(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("parseMetricExportInterval(%q) = %s, want %s", tt.raw, got, tt.want)
			}
		})
	}

	for _, raw := range []string{"0", "-1", "bogus"} {
		if _, err := parseMetricExportInterval(raw); err == nil {
			t.Fatalf("parseMetricExportInterval(%q) error = nil, want error", raw)
		}
	}
}

func TestSignalEnabled(t *testing.T) {
	if !signalEnabled("metrics", true) {
		t.Fatal("signalEnabled() default true returned false")
	}

	t.Setenv("UPDATE_IPSETS_OTEL_METRICS", "false")
	if signalEnabled("metrics", true) {
		t.Fatal("signalEnabled() ignored UPDATE_IPSETS_OTEL_METRICS=false")
	}

	t.Setenv("UPDATE_IPSETS_OTEL_METRICS", "")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	if signalEnabled("metrics", true) {
		t.Fatal("signalEnabled() ignored OTEL_METRICS_EXPORTER=none")
	}

	t.Setenv("OTEL_METRICS_EXPORTER", "")
	t.Setenv("UPDATE_IPSETS_OTEL_METRICS", "not-a-bool")
	if !signalEnabled("metrics", true) {
		t.Fatal("signalEnabled() should ignore invalid local signal values")
	}
}

func TestMetricCardinalityViewDropsEphemeralAttributes(t *testing.T) {
	t.Parallel()

	stream, ok := metricCardinalityView()(sdkmetric.Instrument{Name: "download.fetches"})
	if !ok {
		t.Fatal("metricCardinalityView() did not match instrument")
	}
	if stream.AttributeFilter == nil {
		t.Fatal("metricCardinalityView() returned nil AttributeFilter")
	}

	for _, key := range ephemeralMetricAttributeKeys {
		t.Run(string(key), func(t *testing.T) {
			t.Parallel()
			if stream.AttributeFilter(key.Int(1)) {
				t.Fatalf("metric attribute %q was kept; want dropped", key)
			}
		})
	}

	for _, attr := range []attribute.KeyValue{
		attribute.String("download.downloader", "http"),
		attribute.String("download.status", "ok"),
	} {
		t.Run(string(attr.Key), func(t *testing.T) {
			t.Parallel()
			if !stream.AttributeFilter(attr) {
				t.Fatalf("metric attribute %q was dropped; want kept", attr.Key)
			}
		})
	}

	for _, attr := range []attribute.KeyValue{
		attribute.String("feed.name", "bounded-catalog-name"),
		attribute.Int("http.response.status_code", 200),
		attribute.String("processor.step", "csv_column"),
		attribute.String("engine.phase", "sources"),
	} {
		t.Run(string(attr.Key), func(t *testing.T) {
			t.Parallel()
			if stream.AttributeFilter(attr) {
				t.Fatalf("metric attribute %q was kept; want dropped for download metrics", attr.Key)
			}
		})
	}
}

func TestMetricPolicyDropsNoisyAPIMetrics(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"http.server.request.body.size",
		"http.server.response.body.size",
		"http.admin_status.total.duration_ms",
		"http.home_summary.request.duration_ms",
		"http.compare_set.request.duration_ms",
		"http.entity_artifact.country_detail_hit",
		"prometheus.endpoint.test",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stream, ok := metricCardinalityView()(sdkmetric.Instrument{Name: name})
			if !ok {
				t.Fatal("metricCardinalityView() did not match instrument")
			}
			if _, ok := stream.Aggregation.(sdkmetric.AggregationDrop); !ok {
				t.Fatalf("metric %q aggregation = %T, want AggregationDrop", name, stream.Aggregation)
			}
		})
	}
}

func TestMetricPolicyAllowListsHTTPServerDurationAttributes(t *testing.T) {
	t.Parallel()

	stream, ok := metricCardinalityView()(sdkmetric.Instrument{Name: "http.server.request.duration"})
	if !ok {
		t.Fatal("metricCardinalityView() did not match HTTP duration instrument")
	}
	if stream.AttributeFilter == nil {
		t.Fatal("HTTP duration stream returned nil AttributeFilter")
	}

	for _, attr := range []attribute.KeyValue{
		attribute.String("http.route", "/api/v1/sets/{name}/search"),
		attribute.String("http.request.method", "GET"),
		attribute.Int("http.response.status_code", 200),
	} {
		if !stream.AttributeFilter(attr) {
			t.Fatalf("HTTP duration attribute %q was dropped; want kept", attr.Key)
		}
	}

	for _, attr := range []attribute.KeyValue{
		attribute.String("url.path", "/api/v1/sets/example/search"),
		attribute.String("server.address", "127.0.0.1"),
		attribute.String("network.protocol.version", "1.1"),
		attribute.String("feed.name", "example"),
	} {
		if stream.AttributeFilter(attr) {
			t.Fatalf("HTTP duration attribute %q was kept; want dropped", attr.Key)
		}
	}
}

func TestMetricPolicyAllowListsDesignedComponentAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		keep attribute.KeyValue
		drop attribute.KeyValue
	}{
		{name: "feed.entries", keep: attribute.String("feed.name", "source-a"), drop: attribute.String("download.status", "ok")},
		{name: "engine.phase.duration_ms", keep: attribute.String("engine.phase", "publish"), drop: attribute.String("feed.name", "source-a")},
		{name: "processor.runs", keep: attribute.String("processor.mode", "stream"), drop: attribute.String("processor.step", "csv_column")},
		{name: "iprange.operations", keep: attribute.String("iprange.operation", "merge"), drop: attribute.String("iprange.source", "fileset")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stream, ok := metricCardinalityView()(sdkmetric.Instrument{Name: tt.name})
			if !ok {
				t.Fatal("metricCardinalityView() did not match instrument")
			}
			if stream.AttributeFilter == nil {
				t.Fatal("designed metric stream returned nil AttributeFilter")
			}
			if !stream.AttributeFilter(tt.keep) {
				t.Fatalf("metric %q dropped bounded attribute %q", tt.name, tt.keep.Key)
			}
			if stream.AttributeFilter(tt.drop) {
				t.Fatalf("metric %q kept unrelated attribute %q", tt.name, tt.drop.Key)
			}
		})
	}
}

func TestMetricPolicyAllowsOnlyDesignedInstruments(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"download.fetches",
		"feed.entries",
		"engine.runs",
		"integrity.checks",
	} {
		stream, ok := metricCardinalityView()(sdkmetric.Instrument{Name: name})
		if !ok {
			t.Fatalf("metricCardinalityView() did not match %q", name)
		}
		if _, ok := stream.Aggregation.(sdkmetric.AggregationDrop); ok {
			t.Fatalf("designed metric %q was dropped", name)
		}
	}

	stream, ok := metricCardinalityView()(sdkmetric.Instrument{Name: "engine.sources"})
	if !ok {
		t.Fatal("metricCardinalityView() did not match unknown instrument")
	}
	if _, ok := stream.Aggregation.(sdkmetric.AggregationDrop); !ok {
		t.Fatal("unknown metric was not dropped")
	}
}

func TestMetricResourceOmitsEphemeralProcessIdentity(t *testing.T) {
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")
	t.Setenv("OTEL_SERVICE_NAME", "")

	res, err := newResource(t.Context(), "update-ipsets", "test-version-dirty", false, false)
	if err != nil {
		t.Fatalf("newResource() error = %v", err)
	}

	for _, key := range []attribute.Key{
		attribute.Key("process.pid"),
		attribute.Key("process.parent_pid"),
		attribute.Key("process.executable.name"),
		attribute.Key("process.executable.path"),
		attribute.Key("process.command"),
		attribute.Key("process.command_line"),
		attribute.Key("process.command_args"),
		attribute.Key("process.owner"),
		attribute.Key("process.runtime.name"),
		attribute.Key("process.runtime.version"),
		attribute.Key("process.runtime.description"),
		attribute.Key("service.version"),
		attribute.Key("host.name"),
		attribute.Key("host.id"),
		attribute.Key("os.type"),
		attribute.Key("os.description"),
	} {
		if res.Set().HasValue(key) {
			t.Fatalf("metric resource contains ephemeral key %q", key)
		}
	}

	for _, key := range []attribute.Key{
		attribute.Key("service.name"),
		attribute.Key("service.namespace"),
	} {
		if !res.Set().HasValue(key) {
			t.Fatalf("metric resource missing stable key %q", key)
		}
	}
}

func TestPrometheusMetricsHandlerExposesFilteredMetrics(t *testing.T) {
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

	meter := provider.Meter(scopeName)
	counter, err := meter.Int64Counter("download.fetches")
	if err != nil {
		t.Fatalf("Int64Counter() error = %v", err)
	}
	counter.Add(t.Context(), 1,
		otelmetric.WithAttributes(
			attribute.String("download.status", "ok"),
			attribute.Int("scheduler.waiting", 42),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "download_fetches_total") {
		t.Fatalf("/metrics body missing recorded counter:\n%s", body)
	}
	if !strings.Contains(body, `download_status="ok"`) {
		t.Fatalf("/metrics body missing bounded label:\n%s", body)
	}
	if strings.Contains(body, "scheduler") {
		t.Fatalf("/metrics body exposed ephemeral scheduler label:\n%s", body)
	}
}

func TestAPIRecalculationMetricsExposeBoundedLabels(t *testing.T) {
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
	t.Cleanup(func() {
		instrumentsMu.Lock()
		meter = oldMeter
		counters = oldCounters
		histograms = oldHistograms
		gauges = oldGauges
		instrumentsMu.Unlock()
	})

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
