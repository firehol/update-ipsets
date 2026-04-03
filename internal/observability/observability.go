package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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

var (
	meter  = otel.Meter(scopeName)
	tracer = otel.Tracer(scopeName)

	instrumentsMu sync.Mutex
	counters      = map[string]metric.Int64Counter{}
	histograms    = map[string]metric.Float64Histogram{}
)

type Setup struct {
	Enabled  bool
	Logger   *slog.Logger
	shutdown func(context.Context) error
}

func Init(ctx context.Context, serviceName, version string, baseLogger *slog.Logger) (*Setup, error) {
	if serviceName == "" {
		serviceName = "update-ipsets"
	}
	if version == "" {
		version = "dev"
	}
	if baseLogger == nil {
		baseLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	if !enabledFromEnv() {
		return &Setup{
			Enabled: false,
			Logger:  baseLogger,
			shutdown: func(context.Context) error {
				return nil
			},
		}, nil
	}
	protocol, err := protocolFromEnv()
	if err != nil {
		return nil, err
	}
	metricReaderOpts, err := metricReaderOptionsFromEnv()
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceNamespace("firehol"),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, err
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	var shutdowns []func(context.Context) error
	if signalEnabled("traces", true) {
		traceExporter, err := newTraceExporter(ctx, protocol)
		if err != nil {
			return nil, errors.Join(err, shutdownAll(ctx, shutdowns))
		}
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithBatcher(traceExporter),
		)
		otel.SetTracerProvider(tracerProvider)
		tracer = otel.Tracer(scopeName)
		shutdowns = append(shutdowns, tracerProvider.Shutdown)
	}

	if signalEnabled("metrics", true) {
		metricExporter, err := newMetricExporter(ctx, protocol)
		if err != nil {
			return nil, errors.Join(err, shutdownAll(ctx, shutdowns))
		}
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, metricReaderOpts...)),
		)
		otel.SetMeterProvider(meterProvider)
		meter = otel.Meter(scopeName)
		shutdowns = append(shutdowns, meterProvider.Shutdown)
	}

	logger := baseLogger
	if signalEnabled("logs", true) {
		logExporter, err := newLogExporter(ctx, protocol)
		if err != nil {
			return nil, errors.Join(err, shutdownAll(ctx, shutdowns))
		}
		loggerProvider := sdklog.NewLoggerProvider(
			sdklog.WithResource(res),
			sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		)
		logglobal.SetLoggerProvider(loggerProvider)
		otelHandler := otelslog.NewHandler(scopeName, otelslog.WithLoggerProvider(loggerProvider))
		logger = slog.New(newTeeHandler(baseLogger.Handler(), otelHandler))
		shutdowns = append(shutdowns, loggerProvider.Shutdown)
	}

	return &Setup{
		Enabled: true,
		Logger:  logger,
		shutdown: func(ctx context.Context) error {
			return shutdownAll(ctx, shutdowns)
		},
	}, nil
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

func counter(name string) (metric.Int64Counter, bool) {
	instrumentsMu.Lock()
	defer instrumentsMu.Unlock()
	if existing, ok := counters[name]; ok {
		return existing, true
	}
	created, err := meter.Int64Counter(name)
	if err != nil {
		return nil, false
	}
	counters[name] = created
	return created, true
}

func histogram(name string) (metric.Float64Histogram, bool) {
	instrumentsMu.Lock()
	defer instrumentsMu.Unlock()
	if existing, ok := histograms[name]; ok {
		return existing, true
	}
	created, err := meter.Float64Histogram(name, metric.WithUnit("ms"))
	if err != nil {
		return nil, false
	}
	histograms[name] = created
	return created, true
}
