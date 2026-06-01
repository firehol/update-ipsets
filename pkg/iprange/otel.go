package iprange

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const iprangeTelemetryScope = "github.com/firehol/update-ipsets/pkg/iprange"

var (
	iprangeTelemetryMu sync.Mutex
	iprangeCounters    = map[string]metric.Int64Counter{}
	iprangeHistograms  = map[string]metric.Float64Histogram{}
)

func iprangeStart(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	return otel.Tracer(iprangeTelemetryScope).Start(ctx, name, trace.WithAttributes(attrs...))
}

func iprangeBackground() context.Context {
	return context.Background()
}

func iprangeEnd(span trace.Span, err error) {
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

func iprangeObserve(ctx context.Context, name string, count, bytes int64, dur time.Duration, attrs ...attribute.KeyValue) {
	if ctx == nil {
		ctx = context.Background()
	}
	metricAttrs := iprangeMetricAttributes(name, attrs...)
	if count != 0 {
		if counter, ok := iprangeCounter("iprange.operations"); ok {
			counter.Add(ctx, count, metric.WithAttributes(metricAttrs...))
		}
	}
	if bytes != 0 && count == 0 {
		if counter, ok := iprangeCounter("iprange.operations"); ok {
			counter.Add(ctx, 1, metric.WithAttributes(metricAttrs...))
		}
	}
	if dur > 0 {
		if histogram, ok := iprangeHistogram("iprange.operation.duration_ms"); ok {
			histogram.Record(ctx, float64(dur.Microseconds())/1000.0, metric.WithAttributes(metricAttrs...))
		}
	}
}

func iprangeCount(ctx context.Context, name string, count int64, attrs ...attribute.KeyValue) {
	iprangeObserve(ctx, name, count, 0, 0, attrs...)
}

func iprangeMetricAttributes(name string, attrs ...attribute.KeyValue) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, 2)
	for _, attr := range attrs {
		if attr.Key == attribute.Key("ip.version") {
			out = append(out, attr)
			break
		}
	}
	out = append(out, attribute.String("iprange.operation", iprangeMetricOperation(name)))
	return out
}

func iprangeMetricOperation(name string) string {
	op := strings.TrimSpace(name)
	op = strings.TrimPrefix(op, "iprange.")
	for _, suffix := range []string{".duration_ms", ".bytes", ".ops"} {
		op = strings.TrimSuffix(op, suffix)
	}
	op = strings.TrimSuffix(op, ".searches")
	op = strings.TrimSuffix(op, ".search")
	op = strings.Trim(op, ".")
	op = strings.ReplaceAll(op, ".", "_")
	if op == "" {
		return "unknown"
	}
	return op
}

func iprangeCounter(name string) (metric.Int64Counter, bool) {
	iprangeTelemetryMu.Lock()
	defer iprangeTelemetryMu.Unlock()
	if existing, ok := iprangeCounters[name]; ok {
		return existing, true
	}
	created, err := otel.Meter(iprangeTelemetryScope).Int64Counter(name)
	if err != nil {
		return nil, false
	}
	iprangeCounters[name] = created
	return created, true
}

func iprangeHistogram(name string) (metric.Float64Histogram, bool) {
	iprangeTelemetryMu.Lock()
	defer iprangeTelemetryMu.Unlock()
	if existing, ok := iprangeHistograms[name]; ok {
		return existing, true
	}
	created, err := otel.Meter(iprangeTelemetryScope).Float64Histogram(name, metric.WithUnit("ms"))
	if err != nil {
		return nil, false
	}
	iprangeHistograms[name] = created
	return created, true
}
