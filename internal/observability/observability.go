package observability

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/firehol/update-ipsets/internal/observability/otelexporter"
)

const DefaultLogTraceBufferBytes int64 = 50 * 1024 * 1024
const otlpStartupTimeout = 2 * time.Second

type otlpProtocol string

const (
	otlpProtocolHTTP otlpProtocol = "http/protobuf"
	otlpProtocolGRPC otlpProtocol = "grpc"
)

type attrKind uint8

const (
	attrString attrKind = iota
	attrInt
	attrBool
)

type Attr struct {
	Key string

	kind attrKind
	s    string
	i    int64
	b    bool
}

func String(key, value string) Attr {
	return Attr{Key: key, kind: attrString, s: value}
}

func Int(key string, value int) Attr {
	return Attr{Key: key, kind: attrInt, i: int64(value)}
}

func Int64(key string, value int64) Attr {
	return Attr{Key: key, kind: attrInt, i: value}
}

func Bool(key string, value bool) Attr {
	return Attr{Key: key, kind: attrBool, b: value}
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
	if serviceName == "" {
		serviceName = "update-ipsets"
	}
	if version == "" {
		version = "dev"
	}
	if baseLogger == nil {
		baseLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	logBufferBytes, traceBufferBytes, err := telemetryBufferBudgetsFromEnv()
	if err != nil {
		baseLogger.Warn("local telemetry buffer configuration invalid; using default", "error", err)
		logBufferBytes = DefaultLogTraceBufferBytes
		traceBufferBytes = 0
	}
	configureTraceQueue(traceBufferBytes)
	traceQueue := activeTraceQueue.Load()
	logHandler := newAsyncLogHandler(baseLogger.Handler(), asyncLogQueueCapacity(logBufferBytes))
	logger := slog.New(logHandler)
	var shutdowns []func(context.Context) error
	shutdowns = append(shutdowns, logHandler.Shutdown)
	shutdowns = append(shutdowns, startRuntimeMetricSampler(runtimeMetricsSampleInterval))
	if traceQueue != nil {
		shutdowns = append(shutdowns, func(context.Context) error {
			traceQueue.stopQueue()
			return nil
		})
	}
	exportEnabled := false
	if enabledFromEnv() && signalEnabled("metrics", true) {
		protocol, err := protocolFromEnv()
		if err != nil {
			logger.Warn("opentelemetry export disabled", "error", err)
		} else {
			intervals, err := metricReaderOptionsFromEnv()
			if err != nil {
				logger.Warn("opentelemetry metric export disabled", "error", err)
			} else {
				setupCtx, cancel := boundedSetupContext(ctx, otlpStartupTimeout)
				defer cancel()
				var interval time.Duration
				if len(intervals) > 0 {
					interval = intervals[0]
				}
				exporter, err := otelexporter.Start(setupCtx, otelexporter.Options{
					ServiceName: serviceName,
					Version:     version,
					Protocol:    otelexporter.Protocol(protocol),
					Interval:    interval,
					Descriptors: metricExportDescriptors(),
					Snapshot:    metricExportSamples,
					Logger:      logger,
				})
				if err != nil {
					logger.Warn("opentelemetry metric export disabled", "error", err)
				} else {
					shutdowns = append(shutdowns, exporter.Shutdown)
					exportEnabled = true
				}
			}
		}
	}
	TryGauge("daemon.up", 1)
	return &Setup{
		Enabled:           exportEnabled,
		Logger:            logger,
		PrometheusHandler: newPrometheusMetrics(),
		shutdown:          func(ctx context.Context) error { return shutdownAll(ctx, shutdowns) },
	}, nil
}

func (s *Setup) Shutdown(ctx context.Context) error {
	if s == nil || s.shutdown == nil {
		return nil
	}
	TryGauge("daemon.up", 0)
	return s.shutdown(ctx)
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

type Span struct {
	name    string
	started time.Time
	id      uint64
	attrs   [maxTraceAttrs]Attr
	nattrs  uint8
}

var nextSpanID atomic.Uint64

func Start(ctx context.Context, name string, attrs ...Attr) (context.Context, Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	if activeTraceQueue.Load() == nil {
		return ctx, Span{}
	}
	if name == "" {
		name = "operation"
	}
	name = boundedTraceString(name)
	span := Span{name: name, started: time.Now(), id: nextSpanID.Add(1)}
	span.nattrs = copyTraceAttrs(&span.attrs, attrs)
	enqueueTraceEvent(TraceEvent{
		ID:     span.id,
		Kind:   traceEventStart,
		Time:   span.started,
		Name:   span.name,
		Attrs:  span.attrs,
		NAttrs: span.nattrs,
	})
	return ctx, span
}

func End(span Span, err error) {
	if span.id == 0 {
		return
	}
	enqueueTraceEvent(TraceEvent{
		ID:       span.id,
		Kind:     traceEventEnd,
		Time:     time.Now(),
		Name:     span.name,
		Duration: time.Since(span.started),
		Error:    err != nil,
		Attrs:    span.attrs,
		NAttrs:   span.nattrs,
	})
}

func BackgroundContext() context.Context {
	return context.Background()
}

func Count(ctx context.Context, name string, count int64, attrs ...Attr) {
	_ = ctx
	TryCount(name, count, attrs...)
}

func Bytes(ctx context.Context, name string, bytes int64, attrs ...Attr) {
	_ = ctx
	TryBytes(name, bytes, attrs...)
}

func Duration(ctx context.Context, name string, dur time.Duration, attrs ...Attr) {
	_ = ctx
	TryDuration(name, dur, attrs...)
}

func Observe(ctx context.Context, name string, count, bytes int64, dur time.Duration, attrs ...Attr) {
	_ = ctx
	TryObserve(name, count, bytes, dur, attrs...)
}

func Gauge(ctx context.Context, name string, value int64, attrs ...Attr) {
	_ = ctx
	TryGauge(name, value, attrs...)
}

func APIRecalculation(ctx context.Context, surface, action, result string, targets int64) {
	_ = ctx
	TryAPIRecalculation(surface, action, result, targets)
}

func TryCount(name string, count int64, attrs ...Attr) {
	if count <= 0 || name == "" {
		return
	}
	defaultRegistry.addCounter(name, count, attrs...)
}

func TryBytes(name string, bytes int64, attrs ...Attr) {
	if bytes <= 0 || name == "" {
		return
	}
	TryCount(name+".bytes", bytes, attrs...)
}

func TryDuration(name string, dur time.Duration, attrs ...Attr) {
	if dur <= 0 || name == "" {
		return
	}
	defaultRegistry.observeDuration(name+".duration_ms", dur, attrs...)
}

func TryObserve(name string, count, bytes int64, dur time.Duration, attrs ...Attr) {
	TryCount(name, count, attrs...)
	TryBytes(name, bytes, attrs...)
	TryDuration(name, dur, attrs...)
}

func TryGauge(name string, value int64, attrs ...Attr) {
	if name == "" {
		return
	}
	defaultRegistry.setGauge(name, value, attrs...)
}

func TryAPIRecalculation(surface, action, result string, targets int64) {
	attrs := [3]Attr{
		String("api.surface", surface),
		String("api.action", action),
		String("api.result", result),
	}
	TryCount("api.recalculation.requests", 1, attrs[:]...)
	if targets > 0 {
		TryCount("api.recalculation.targets", targets, attrs[:]...)
	}
}

func TryHTTPServerRequest(route, method string, status int, dur time.Duration) {
	if dur <= 0 {
		return
	}
	if route == "" {
		route = "/*"
	}
	if method == "" {
		method = "UNKNOWN"
	}
	if status <= 0 {
		status = http.StatusOK
	}
	attrs := [3]Attr{
		String("http.route", route),
		String("http.request.method", method),
		Int("http.response.status_code", status),
	}
	defaultRegistry.observeDuration("http.server.request.duration", dur, attrs[:]...)
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

func resetMetricsForTest() {
	for _, series := range defaultRegistry.series {
		series.touched.Store(false)
		series.counter.Store(0)
		series.gauge.Store(0)
		series.count.Store(0)
		series.sumMicros.Store(0)
		series.maxMicros.Store(0)
	}
	nextSpanID.Store(0)
	configureTraceQueue(0)
}
