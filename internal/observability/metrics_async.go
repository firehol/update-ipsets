package observability

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const asyncMetricQueueSize = 8192

type asyncMetricKind uint8

const (
	asyncMetricCount asyncMetricKind = iota
	asyncMetricDuration
	asyncMetricGauge
	asyncMetricHTTPServerRequest
)

type asyncMetric struct {
	kind   asyncMetricKind
	name   string
	count  int64
	value  int64
	dur    time.Duration
	route  string
	method string
	status int
	attrs  []attribute.KeyValue
}

var (
	asyncMetrics        = make(chan asyncMetric, asyncMetricQueueSize)
	asyncMetricsStarted atomic.Bool
)

// TryCount records a counter from latency-sensitive paths without waiting for
// OpenTelemetry SDK/exporter work. Samples are dropped when the queue is full.
func TryCount(name string, count int64, attrs ...attribute.KeyValue) {
	if count == 0 || !designedCounterMetricInstrument(name) {
		return
	}
	enqueueAsyncMetric(asyncMetric{
		kind:  asyncMetricCount,
		name:  name,
		count: count,
		attrs: copyMetricAttrs(attrs),
	})
}

func TryBytes(name string, bytes int64, attrs ...attribute.KeyValue) {
	if bytes == 0 || name == "" {
		return
	}
	TryCount(name+".bytes", bytes, attrs...)
}

// TryDuration records a duration from latency-sensitive paths without waiting
// for OpenTelemetry SDK/exporter work. Samples are dropped when the queue is full.
func TryDuration(name string, dur time.Duration, attrs ...attribute.KeyValue) {
	if dur <= 0 || !designedDurationMetricInstrument(name) {
		return
	}
	enqueueAsyncMetric(asyncMetric{
		kind:  asyncMetricDuration,
		name:  name,
		dur:   dur,
		attrs: copyMetricAttrs(attrs),
	})
}

func TryObserve(name string, count, bytes int64, dur time.Duration, attrs ...attribute.KeyValue) {
	TryCount(name, count, attrs...)
	TryBytes(name, bytes, attrs...)
	TryDuration(name, dur, attrs...)
}

// TryGauge records a gauge from latency-sensitive paths without waiting for
// OpenTelemetry SDK/exporter work. Samples are dropped when the queue is full.
func TryGauge(name string, value int64, attrs ...attribute.KeyValue) {
	if !designedGaugeMetricInstrument(name) {
		return
	}
	enqueueAsyncMetric(asyncMetric{
		kind:  asyncMetricGauge,
		name:  name,
		value: value,
		attrs: copyMetricAttrs(attrs),
	})
}

func TryAPIRecalculation(surface, action, result string, targets int64) {
	attrs := []attribute.KeyValue{
		attribute.String("api.surface", boundedMetricLabel(surface, apiRecalculationSurfaces)),
		attribute.String("api.action", boundedMetricLabel(action, apiRecalculationActions)),
		attribute.String("api.result", boundedMetricLabel(result, apiRecalculationResults)),
	}
	TryCount("api.recalculation.requests", 1, attrs...)
	if targets > 0 {
		TryCount("api.recalculation.targets", targets, attrs...)
	}
}

func TryHTTPServerRequest(route, method string, status int, dur time.Duration) {
	if dur <= 0 {
		return
	}
	enqueueAsyncMetric(asyncMetric{
		kind:   asyncMetricHTTPServerRequest,
		dur:    dur,
		route:  route,
		method: method,
		status: status,
	})
}

func designedCounterMetricInstrument(name string) bool {
	if name == "" || designedGaugeMetricInstrument(name) || strings.HasSuffix(name, ".duration_ms") || name == "http.server.request.duration" {
		return false
	}
	return designedMetricInstrument(name)
}

func designedDurationMetricInstrument(name string) bool {
	return designedMetricInstrument(name + ".duration_ms")
}

func copyMetricAttrs(attrs []attribute.KeyValue) []attribute.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]attribute.KeyValue, len(attrs))
	copy(out, attrs)
	return out
}

func enqueueAsyncMetric(event asyncMetric) {
	if asyncMetricsStarted.CompareAndSwap(false, true) {
		go asyncMetricWorker()
	}
	select {
	case asyncMetrics <- event:
	default:
	}
}

func asyncMetricWorker() {
	for event := range asyncMetrics {
		recordAsyncMetric(event)
	}
}

func recordAsyncMetric(event asyncMetric) {
	ctx := context.Background()
	switch event.kind {
	case asyncMetricCount:
		Count(ctx, event.name, event.count, event.attrs...)
	case asyncMetricDuration:
		Duration(ctx, event.name, event.dur, event.attrs...)
	case asyncMetricGauge:
		Gauge(ctx, event.name, event.value, event.attrs...)
	case asyncMetricHTTPServerRequest:
		recordHTTPServerRequest(ctx, event.route, event.method, event.status, event.dur)
	}
}

func recordHTTPServerRequest(ctx context.Context, route, method string, status int, dur time.Duration) {
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
	histogram, ok := histogram("http.server.request.duration")
	if !ok {
		return
	}
	histogram.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.String("http.route", route),
		attribute.String("http.request.method", method),
		attribute.Int("http.response.status_code", status),
	))
}
