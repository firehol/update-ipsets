package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	logglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const scopeName = "github.com/firehol/update-ipsets"

type otlpProtocol string

const (
	otlpProtocolHTTP otlpProtocol = "http/protobuf"
	otlpProtocolGRPC otlpProtocol = "grpc"
)

const otlpStartupTimeout = 2 * time.Second

var ephemeralMetricAttributeKeys = []attribute.Key{
	attribute.Key("engine.batch.size"),
	attribute.Key("file.bytes"),
	attribute.Key("iprange.sources"),
	attribute.Key("processor.input.bytes"),
	attribute.Key("processor.steps"),
	attribute.Key("run.selected"),
	attribute.Key("scheduler.waiting"),
}

var httpServerMetricAttributeKeys = []attribute.Key{
	attribute.Key("http.route"),
	attribute.Key("http.request.method"),
	attribute.Key("http.response.status_code"),
}

var designedMetricNames = map[string]struct{}{
	"api.recalculation.requests":          {},
	"api.recalculation.targets":           {},
	"background.tasks":                    {},
	"background.worker.wait.duration_ms":  {},
	"background.workers.active":           {},
	"background.workers.limit":            {},
	"config.load.duration_ms":             {},
	"config.loads":                        {},
	"daemon.goroutine.panics":             {},
	"daemon.up":                           {},
	"daemon.watchdog.diagnostics":         {},
	"download.errors":                     {},
	"download.fetch.bytes":                {},
	"download.fetch.duration_ms":          {},
	"download.fetches":                    {},
	"engine.phase.current":                {},
	"engine.phase.duration_ms":            {},
	"engine.run.duration_ms":              {},
	"engine.running":                      {},
	"engine.runs":                         {},
	"feed.entries":                        {},
	"feed.errors":                         {},
	"feed.freshness.seconds":              {},
	"feed.health.state":                   {},
	"feed.last_success.timestamp":         {},
	"feed.state":                          {},
	"feed.unique_ips":                     {},
	"http.server.request.duration":        {},
	"integrity.check.duration_ms":         {},
	"integrity.checks":                    {},
	"integrity.findings":                  {},
	"integrity.recovery.targets":          {},
	"iprange.operations":                  {},
	"iprange.operation.duration_ms":       {},
	"processor.run.duration_ms":           {},
	"processor.runs":                      {},
	"processor.temp.write.duration_ms":    {},
	"processor.temp.writes":               {},
	"runtime.cache.operation.duration_ms": {},
	"runtime.cache.operations":            {},
	"scheduler.action.admission_failures": {},
	"scheduler.batch.duration_ms":         {},
	"scheduler.batch.items":               {},
	"scheduler.queue.admissions":          {},
	"scheduler.queue.depth":               {},
	"scheduler.recovered_panics":          {},
	"scheduler.work.completed":            {},
	"scheduler.work.started":              {},
	"systemd.notify.failures":             {},
	"web.artifact.cache.bytes":            {},
	"web.artifact.cache.entries":          {},
	"web.artifact.cache.evictions":        {},
	"web.artifact.cache.lookups":          {},
}

var apiRecalculationSurfaces = map[string]struct{}{
	"admin":  {},
	"public": {},
}

var apiRecalculationActions = map[string]struct{}{
	"artifact_recheck":    {},
	"compose":             {},
	"entity_rebuild":      {},
	"feed_recheck":        {},
	"feed_reprocess":      {},
	"feed_search":         {},
	"integrity_reprocess": {},
	"run_due":             {},
	"run_recheck":         {},
	"run_reprocess":       {},
	"search":              {},
}

var apiRecalculationResults = map[string]struct{}{
	"clean":       {},
	"conflict":    {},
	"error":       {},
	"in_progress": {},
	"ok":          {},
	"rejected":    {},
	"scheduled":   {},
}

var (
	meter  = otel.Meter(scopeName)
	tracer = otel.Tracer(scopeName)

	instrumentsMu sync.RWMutex
	counters      = map[string]metric.Int64Counter{}
	histograms    = map[string]metric.Float64Histogram{}
	gauges        = map[string]metric.Int64Gauge{}
)

var designedGaugeMetricNames = map[string]struct{}{
	"background.workers.active":   {},
	"background.workers.limit":    {},
	"daemon.up":                   {},
	"engine.phase.current":        {},
	"engine.running":              {},
	"feed.entries":                {},
	"feed.errors":                 {},
	"feed.freshness.seconds":      {},
	"feed.health.state":           {},
	"feed.last_success.timestamp": {},
	"feed.state":                  {},
	"feed.unique_ips":             {},
	"integrity.findings":          {},
	"scheduler.batch.items":       {},
	"scheduler.queue.depth":       {},
	"web.artifact.cache.bytes":    {},
	"web.artifact.cache.entries":  {},
}

type Setup struct {
	Enabled           bool
	Logger            *slog.Logger
	PrometheusHandler http.Handler
	shutdown          func(context.Context) error
}

func Init(ctx context.Context, serviceName, version string, baseLogger *slog.Logger) (*Setup, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	setupCtx, cancel := boundedSetupContext(ctx, otlpStartupTimeout)
	defer cancel()

	if serviceName == "" {
		serviceName = "update-ipsets"
	}
	if version == "" {
		version = "dev"
	}
	if baseLogger == nil {
		baseLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	otlpEnabled := enabledFromEnv()
	otlpActive := false

	metricRes, err := newResource(setupCtx, serviceName, version, false, false)
	if err != nil {
		baseLogger.Warn("opentelemetry resource setup failed; using local metrics resource", "error", err)
		metricRes = fallbackResource(serviceName, version, false)
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	var shutdowns []func(context.Context) error
	var richRes *resource.Resource
	if otlpEnabled && (signalEnabled("traces", true) || signalEnabled("logs", true)) {
		richRes, err = newResource(setupCtx, serviceName, version, true, true)
		if err != nil {
			baseLogger.Warn("opentelemetry rich resource setup failed; using fallback resource", "error", err)
			richRes = fallbackResource(serviceName, version, true)
		}
	}

	prometheusReader, prometheusHandler, err := newPrometheusMetrics()
	if err != nil {
		baseLogger.Warn("prometheus metrics setup failed; /metrics disabled", "error", err)
	}
	metricProviderOpts := []sdkmetric.Option{
		sdkmetric.WithResource(metricRes),
		sdkmetric.WithView(metricCardinalityView()),
	}
	if prometheusReader != nil {
		metricProviderOpts = append(metricProviderOpts, sdkmetric.WithReader(prometheusReader))
	}

	var protocol otlpProtocol
	if otlpEnabled {
		protocol, err = protocolFromEnv()
		if err != nil {
			baseLogger.Warn("opentelemetry protocol setup failed; OTLP export disabled", "error", err)
			otlpEnabled = false
		}
	}
	if otlpEnabled {
		if err := setupCtx.Err(); err != nil {
			baseLogger.Warn("opentelemetry startup budget expired; OTLP export disabled", "error", err)
			otlpEnabled = false
		}
	}

	if otlpEnabled {
		if signalEnabled("traces", true) {
			if err := setupCtx.Err(); err != nil {
				baseLogger.Warn("opentelemetry trace export disabled", "error", err)
			} else {
				traceExporter, err := newTraceExporter(setupCtx, protocol)
				if err != nil {
					baseLogger.Warn("opentelemetry trace export disabled", "error", err)
				} else {
					tracerProvider := sdktrace.NewTracerProvider(
						sdktrace.WithResource(richRes),
						sdktrace.WithBatcher(traceExporter),
					)
					otel.SetTracerProvider(tracerProvider)
					tracer = otel.Tracer(scopeName)
					shutdowns = append(shutdowns, tracerProvider.Shutdown)
					otlpActive = true
				}
			}
		}

		if signalEnabled("metrics", true) {
			if err := setupCtx.Err(); err != nil {
				baseLogger.Warn("opentelemetry metric export disabled", "error", err)
			} else {
				metricReaderOpts, err := metricReaderOptionsFromEnv()
				if err != nil {
					baseLogger.Warn("opentelemetry metric export disabled", "error", err)
				} else {
					metricExporter, err := newMetricExporter(setupCtx, protocol)
					if err != nil {
						baseLogger.Warn("opentelemetry metric export disabled", "error", err)
					} else {
						metricProviderOpts = append(metricProviderOpts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, metricReaderOpts...)))
						otlpActive = true
					}
				}
			}
		}
	}
	meterProvider := sdkmetric.NewMeterProvider(metricProviderOpts...)
	otel.SetMeterProvider(meterProvider)
	instrumentsMu.Lock()
	meter = otel.Meter(scopeName)
	counters, histograms, gauges = precreateMetricInstruments(meter)
	instrumentsMu.Unlock()
	shutdowns = append(shutdowns, meterProvider.Shutdown)
	TryGauge("daemon.up", 1)

	logger := baseLogger
	if otlpEnabled && signalEnabled("logs", true) {
		if err := setupCtx.Err(); err != nil {
			baseLogger.Warn("opentelemetry log export disabled", "error", err)
		} else {
			logExporter, err := newLogExporter(setupCtx, protocol)
			if err != nil {
				baseLogger.Warn("opentelemetry log export disabled", "error", err)
			} else {
				loggerProvider := sdklog.NewLoggerProvider(
					sdklog.WithResource(richRes),
					sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
				)
				logglobal.SetLoggerProvider(loggerProvider)
				otelHandler := newAsyncLogHandler(otelslog.NewHandler(scopeName, otelslog.WithLoggerProvider(loggerProvider)), asyncLogQueueSize)
				logger = slog.New(newTeeHandler(baseLogger.Handler(), otelHandler))
				shutdowns = append(shutdowns, loggerProvider.Shutdown)
				shutdowns = append(shutdowns, otelHandler.Shutdown)
				otlpActive = true
			}
		}
	}

	return &Setup{
		Enabled:           otlpActive,
		Logger:            logger,
		PrometheusHandler: prometheusHandler,
		shutdown: func(ctx context.Context) error {
			return shutdownAll(ctx, shutdowns)
		},
	}, nil
}

func newResource(ctx context.Context, serviceName, version string, includeProcess, includeVersion bool) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName),
		semconv.ServiceNamespace("firehol"),
	}
	if includeVersion {
		attrs = append(attrs, semconv.ServiceVersion(version))
	}
	options := []resource.Option{
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	}
	if includeProcess {
		options = append(options,
			resource.WithHost(),
			resource.WithOS(),
		)
		options = append(options, resource.WithProcess())
	}
	options = append(options, resource.WithAttributes(attrs...))
	return resource.New(ctx, options...)
}

func fallbackResource(serviceName, version string, includeVersion bool) *resource.Resource {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName),
		semconv.ServiceNamespace("firehol"),
	}
	if includeVersion {
		attrs = append(attrs, semconv.ServiceVersion(version))
	}
	return resource.NewSchemaless(attrs...)
}

func boundedSetupContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		return ctx, func() {}
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func metricCardinalityView() sdkmetric.View {
	return metricPolicyView
}

func metricPolicyView(inst sdkmetric.Instrument) (sdkmetric.Stream, bool) {
	switch {
	case !designedMetricInstrument(inst.Name):
		return metricStream(inst, nil, sdkmetric.AggregationDrop{}), true
	case droppedMetricInstrument(inst.Name):
		return metricStream(inst, nil, sdkmetric.AggregationDrop{}), true
	case inst.Name == "http.server.request.duration":
		return metricStream(inst, httpServerMetricAttributeFilter(), nil), true
	default:
		return metricStream(inst, metricAttributeFilterForInstrument(inst.Name), nil), true
	}
}

func metricStream(inst sdkmetric.Instrument, filter attribute.Filter, aggregation sdkmetric.Aggregation) sdkmetric.Stream {
	return sdkmetric.Stream{
		Name:            inst.Name,
		Description:     inst.Description,
		Unit:            inst.Unit,
		Aggregation:     aggregation,
		AttributeFilter: filter,
	}
}

func droppedMetricInstrument(name string) bool {
	switch name {
	case "http.server.request.body.size", "http.server.response.body.size":
		return true
	}
	return apiAdHocMetricName(name)
}

func designedMetricInstrument(name string) bool {
	_, ok := designedMetricNames[name]
	return ok
}

func apiAdHocMetricName(name string) bool {
	for _, prefix := range []string{
		"http.admin_",
		"http.compare_set.",
		"http.entity_artifact.",
		"http.home_",
	} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func metricAttributeFilter() attribute.Filter {
	return attribute.NewDenyKeysFilter(ephemeralMetricAttributeKeys...)
}

func metricAttributeFilterForInstrument(name string) attribute.Filter {
	switch {
	case strings.HasPrefix(name, "api.recalculation."):
		return attribute.NewAllowKeysFilter(attribute.Key("api.surface"), attribute.Key("api.action"), attribute.Key("api.result"))
	case strings.HasPrefix(name, "web.artifact.cache."):
		return attribute.NewAllowKeysFilter(attribute.Key("cache.result"), attribute.Key("cache.reason"))
	case strings.HasPrefix(name, "feed."):
		return attribute.NewAllowKeysFilter(attribute.Key("feed.name"))
	case strings.HasPrefix(name, "scheduler."):
		return attribute.NewAllowKeysFilter(attribute.Key("scheduler.queue"), attribute.Key("scheduler.result"))
	case strings.HasPrefix(name, "download."):
		return attribute.NewAllowKeysFilter(attribute.Key("download.downloader"), attribute.Key("download.status"))
	case strings.HasPrefix(name, "processor."):
		return attribute.NewAllowKeysFilter(attribute.Key("processor.mode"), attribute.Key("processor.status"), attribute.Key("processor.temp.kind"))
	case strings.HasPrefix(name, "engine."):
		return attribute.NewAllowKeysFilter(attribute.Key("run.reason"), attribute.Key("run.status"), attribute.Key("engine.phase"))
	case strings.HasPrefix(name, "integrity."):
		return attribute.NewAllowKeysFilter(attribute.Key("integrity.kind"), attribute.Key("integrity.result"), attribute.Key("integrity.action"))
	case strings.HasPrefix(name, "background."):
		return attribute.NewAllowKeysFilter(attribute.Key("background.component"), attribute.Key("background.result"))
	case strings.HasPrefix(name, "config."):
		return attribute.NewAllowKeysFilter(attribute.Key("config.result"))
	case strings.HasPrefix(name, "runtime.cache."):
		return attribute.NewAllowKeysFilter(attribute.Key("cache.operation"), attribute.Key("cache.result"))
	case strings.HasPrefix(name, "iprange."):
		return attribute.NewAllowKeysFilter(attribute.Key("ip.version"), attribute.Key("iprange.operation"))
	default:
		return metricAttributeFilter()
	}
}

func httpServerMetricAttributeFilter() attribute.Filter {
	return attribute.NewAllowKeysFilter(httpServerMetricAttributeKeys...)
}

func newPrometheusMetrics() (*otelprom.Exporter, http.Handler, error) {
	registry := prometheus.NewRegistry()
	exporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		return nil, nil, err
	}
	return exporter, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), nil
}

func (s *Setup) Shutdown(ctx context.Context) error {
	if s == nil || s.shutdown == nil {
		return nil
	}
	return s.shutdown(ctx)
}

func enabledFromEnv() bool {
	if disabled := strings.ToLower(strings.TrimSpace(os.Getenv("UPDATE_IPSETS_OTEL"))); disabled == "0" || disabled == "false" || disabled == "disabled" {
		return false
	}
	if enabled := strings.ToLower(strings.TrimSpace(os.Getenv("UPDATE_IPSETS_OTEL"))); enabled == "1" || enabled == "true" || enabled == "enabled" {
		return true
	}
	for _, key := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
	} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

func protocolFromEnv() (otlpProtocol, error) {
	raw := firstEnv("UPDATE_IPSETS_OTEL_PROTOCOL", "OTEL_EXPORTER_OTLP_PROTOCOL")
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "http/protobuf", "http/proto", "http":
		return otlpProtocolHTTP, nil
	case "grpc":
		return otlpProtocolGRPC, nil
	default:
		return "", fmt.Errorf("unsupported OpenTelemetry protocol %q (use grpc or http/protobuf)", raw)
	}
}

func newTraceExporter(ctx context.Context, protocol otlpProtocol) (sdktrace.SpanExporter, error) {
	if protocol == otlpProtocolGRPC {
		return otlptracegrpc.New(ctx)
	}
	return otlptracehttp.New(ctx)
}

func newMetricExporter(ctx context.Context, protocol otlpProtocol) (sdkmetric.Exporter, error) {
	if protocol == otlpProtocolGRPC {
		return otlpmetricgrpc.New(ctx)
	}
	return otlpmetrichttp.New(ctx)
}

func newLogExporter(ctx context.Context, protocol otlpProtocol) (sdklog.Exporter, error) {
	if protocol == otlpProtocolGRPC {
		return otlploggrpc.New(ctx)
	}
	return otlploghttp.New(ctx)
}

func metricReaderOptionsFromEnv() ([]sdkmetric.PeriodicReaderOption, error) {
	raw := firstEnv("UPDATE_IPSETS_OTEL_METRIC_INTERVAL", "OTEL_METRIC_EXPORT_INTERVAL")
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	interval, err := parseMetricExportInterval(raw)
	if err != nil {
		return nil, err
	}
	return []sdkmetric.PeriodicReaderOption{sdkmetric.WithInterval(interval)}, nil
}

func parseMetricExportInterval(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	if ms, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		if ms <= 0 {
			return 0, fmt.Errorf("OpenTelemetry metric export interval must be positive, got %q", raw)
		}
		return time.Duration(ms) * time.Millisecond, nil
	}
	interval, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid OpenTelemetry metric export interval %q: %w", raw, err)
	}
	if interval <= 0 {
		return 0, fmt.Errorf("OpenTelemetry metric export interval must be positive, got %q", raw)
	}
	return interval, nil
}

func signalEnabled(name string, defaultEnabled bool) bool {
	upper := strings.ToUpper(name)
	if value, ok := lookupBoolEnv("UPDATE_IPSETS_OTEL_" + upper); ok {
		return value
	}
	if raw := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_" + upper + "_EXPORTER"))); raw != "" {
		return raw != "none"
	}
	return defaultEnabled
}

func lookupBoolEnv(key string) (bool, bool) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return false, false
	case "1", "true", "yes", "enabled", "on":
		return true, true
	case "0", "false", "no", "disabled", "off", "none":
		return false, true
	default:
		return false, false
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func shutdownAll(ctx context.Context, shutdowns []func(context.Context) error) error {
	errs := make([]error, 0, len(shutdowns))
	for i := len(shutdowns) - 1; i >= 0; i-- {
		if shutdowns[i] == nil {
			continue
		}
		errs = append(errs, shutdowns[i](ctx))
	}
	return errors.Join(errs...)
}

func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	if name == "" {
		name = "operation"
	}
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func End(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

func BackgroundContext() context.Context {
	return context.Background()
}

func Count(ctx context.Context, name string, count int64, attrs ...attribute.KeyValue) {
	if count == 0 || name == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	counter, ok := counter(name)
	if !ok {
		return
	}
	counter.Add(ctx, count, metric.WithAttributes(attrs...))
}

func Bytes(ctx context.Context, name string, bytes int64, attrs ...attribute.KeyValue) {
	if bytes == 0 || name == "" {
		return
	}
	Count(ctx, name+".bytes", bytes, attrs...)
}

func Duration(ctx context.Context, name string, dur time.Duration, attrs ...attribute.KeyValue) {
	if dur <= 0 || name == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	histogram, ok := histogram(name + ".duration_ms")
	if !ok {
		return
	}
	histogram.Record(ctx, float64(dur.Microseconds())/1000.0, metric.WithAttributes(attrs...))
}

func Observe(ctx context.Context, name string, count, bytes int64, dur time.Duration, attrs ...attribute.KeyValue) {
	Count(ctx, name, count, attrs...)
	Bytes(ctx, name, bytes, attrs...)
	Duration(ctx, name, dur, attrs...)
}

func Gauge(ctx context.Context, name string, value int64, attrs ...attribute.KeyValue) {
	if name == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	gauge, ok := gauge(name)
	if !ok {
		return
	}
	gauge.Record(ctx, value, metric.WithAttributes(attrs...))
}

func APIRecalculation(ctx context.Context, surface, action, result string, targets int64) {
	attrs := []attribute.KeyValue{
		attribute.String("api.surface", boundedMetricLabel(surface, apiRecalculationSurfaces)),
		attribute.String("api.action", boundedMetricLabel(action, apiRecalculationActions)),
		attribute.String("api.result", boundedMetricLabel(result, apiRecalculationResults)),
	}
	Count(ctx, "api.recalculation.requests", 1, attrs...)
	if targets > 0 {
		Count(ctx, "api.recalculation.targets", targets, attrs...)
	}
}

func boundedMetricLabel(value string, allowed map[string]struct{}) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if _, ok := allowed[value]; ok {
		return value
	}
	return "other"
}

func precreateMetricInstruments(m metric.Meter) (map[string]metric.Int64Counter, map[string]metric.Float64Histogram, map[string]metric.Int64Gauge) {
	nextCounters := make(map[string]metric.Int64Counter, len(designedMetricNames))
	nextHistograms := make(map[string]metric.Float64Histogram, len(designedMetricNames))
	nextGauges := make(map[string]metric.Int64Gauge, len(designedGaugeMetricNames))
	for name := range designedMetricNames {
		switch {
		case designedGaugeMetricInstrument(name):
			created, err := m.Int64Gauge(name)
			if err == nil {
				nextGauges[name] = created
			}
		case name == "http.server.request.duration":
			created, err := m.Float64Histogram(name, metric.WithUnit("s"))
			if err == nil {
				nextHistograms[name] = created
			}
		case strings.HasSuffix(name, ".duration_ms"):
			created, err := m.Float64Histogram(name, metric.WithUnit("ms"))
			if err == nil {
				nextHistograms[name] = created
			}
		default:
			created, err := m.Int64Counter(name)
			if err == nil {
				nextCounters[name] = created
			}
		}
	}
	return nextCounters, nextHistograms, nextGauges
}

func designedGaugeMetricInstrument(name string) bool {
	_, ok := designedGaugeMetricNames[name]
	return ok
}

func counter(name string) (metric.Int64Counter, bool) {
	if !designedMetricInstrument(name) {
		return nil, false
	}
	instrumentsMu.RLock()
	existing, ok := counters[name]
	instrumentsMu.RUnlock()
	return existing, ok
}

func histogram(name string) (metric.Float64Histogram, bool) {
	if !designedMetricInstrument(name) {
		return nil, false
	}
	instrumentsMu.RLock()
	existing, ok := histograms[name]
	instrumentsMu.RUnlock()
	return existing, ok
}

func gauge(name string) (metric.Int64Gauge, bool) {
	if !designedMetricInstrument(name) {
		return nil, false
	}
	instrumentsMu.RLock()
	existing, ok := gauges[name]
	instrumentsMu.RUnlock()
	return existing, ok
}
