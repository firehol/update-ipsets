package otelexporter

import (
	"context"
	"math"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestObservableInstrumentsCollectLocalSnapshot(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
	})
	meter := provider.Meter("test")
	instruments, err := buildObservableInstruments(meter, []MetricDescriptor{
		{Name: "download.fetches", Kind: MetricCounter},
		{Name: "daemon.up", Kind: MetricGauge},
		{Name: "engine.run.duration_ms", Kind: MetricDuration, Unit: "ms"},
	})
	if err != nil {
		t.Fatalf("buildObservableInstruments() error = %v", err)
	}
	registration, err := meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		instruments.observe(ctx, observer, []MetricSample{
			{
				Name:   "download.fetches",
				Kind:   MetricCounter,
				Value:  2,
				Labels: []MetricLabel{{Key: "download.status", Value: "ok"}},
			},
			{Name: "daemon.up", Kind: MetricGauge, Value: 1},
			{
				Name:      "engine.run.duration_ms",
				Kind:      MetricDuration,
				Unit:      "ms",
				Count:     1,
				SumMicros: 2500,
				MaxMicros: 2500,
			},
		})
		return nil
	}, instruments.observables...)
	if err != nil {
		t.Fatalf("RegisterCallback() error = %v", err)
	}
	t.Cleanup(func() {
		_ = registration.Unregister()
	})

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	assertInt64Sum(t, rm, "download.fetches", 2, map[string]string{"download.status": "ok"})
	assertInt64Gauge(t, rm, "daemon.up", 1, nil)
	assertFloat64Sum(t, rm, "engine.run.duration_ms.sum", 2.5, nil)
	assertInt64Sum(t, rm, "engine.run.duration_ms.count", 1, nil)
	assertFloat64Gauge(t, rm, "engine.run.duration_ms.max", 2.5, nil)
}

func TestStartRejectsUnsupportedProtocol(t *testing.T) {
	_, err := Start(context.Background(), Options{
		Protocol:    Protocol("invalid"),
		Descriptors: []MetricDescriptor{{Name: "daemon.up", Kind: MetricGauge}},
		Snapshot:    func() []MetricSample { return nil },
	})
	if err == nil {
		t.Fatal("Start() error = nil, want unsupported protocol error")
	}
}

func TestShutdownIsNilSafe(t *testing.T) {
	var exporter *Exporter
	if err := exporter.Shutdown(context.Background()); err != nil {
		t.Fatalf("nil exporter Shutdown() error = %v", err)
	}
}

func assertInt64Sum(t *testing.T, rm metricdata.ResourceMetrics, name string, want int64, labels map[string]string) {
	t.Helper()
	metricData := findMetric(t, rm, name)
	sum, ok := metricData.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("%s data type = %T, want metricdata.Sum[int64]", name, metricData.Data)
	}
	if got := findInt64Point(t, sum.DataPoints, labels); got != want {
		t.Fatalf("%s value = %d, want %d", name, got, want)
	}
}

func assertInt64Gauge(t *testing.T, rm metricdata.ResourceMetrics, name string, want int64, labels map[string]string) {
	t.Helper()
	metricData := findMetric(t, rm, name)
	gauge, ok := metricData.Data.(metricdata.Gauge[int64])
	if !ok {
		t.Fatalf("%s data type = %T, want metricdata.Gauge[int64]", name, metricData.Data)
	}
	if got := findInt64Point(t, gauge.DataPoints, labels); got != want {
		t.Fatalf("%s value = %d, want %d", name, got, want)
	}
}

func assertFloat64Sum(t *testing.T, rm metricdata.ResourceMetrics, name string, want float64, labels map[string]string) {
	t.Helper()
	metricData := findMetric(t, rm, name)
	sum, ok := metricData.Data.(metricdata.Sum[float64])
	if !ok {
		t.Fatalf("%s data type = %T, want metricdata.Sum[float64]", name, metricData.Data)
	}
	if got := findFloat64Point(t, sum.DataPoints, labels); math.Abs(got-want) > 0.000001 {
		t.Fatalf("%s value = %f, want %f", name, got, want)
	}
}

func assertFloat64Gauge(t *testing.T, rm metricdata.ResourceMetrics, name string, want float64, labels map[string]string) {
	t.Helper()
	metricData := findMetric(t, rm, name)
	gauge, ok := metricData.Data.(metricdata.Gauge[float64])
	if !ok {
		t.Fatalf("%s data type = %T, want metricdata.Gauge[float64]", name, metricData.Data)
	}
	if got := findFloat64Point(t, gauge.DataPoints, labels); math.Abs(got-want) > 0.000001 {
		t.Fatalf("%s value = %f, want %f", name, got, want)
	}
}

func findMetric(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Metrics {
	t.Helper()
	for _, scope := range rm.ScopeMetrics {
		for _, metricData := range scope.Metrics {
			if metricData.Name == name {
				return metricData
			}
		}
	}
	t.Fatalf("metric %q not found in %#v", name, rm)
	return metricdata.Metrics{}
}

func findInt64Point(t *testing.T, points []metricdata.DataPoint[int64], labels map[string]string) int64 {
	t.Helper()
	for _, point := range points {
		if labelsMatch(point.Attributes, labels) {
			return point.Value
		}
	}
	t.Fatalf("int64 point with labels %#v not found in %#v", labels, points)
	return 0
}

func findFloat64Point(t *testing.T, points []metricdata.DataPoint[float64], labels map[string]string) float64 {
	t.Helper()
	for _, point := range points {
		if labelsMatch(point.Attributes, labels) {
			return point.Value
		}
	}
	t.Fatalf("float64 point with labels %#v not found in %#v", labels, points)
	return 0
}

func labelsMatch(set attribute.Set, labels map[string]string) bool {
	if len(labels) == 0 {
		return set.Len() == 0
	}
	if set.Len() != len(labels) {
		return false
	}
	for key, want := range labels {
		got, ok := set.Value(attribute.Key(key))
		if !ok || got.AsString() != want {
			return false
		}
	}
	return true
}
