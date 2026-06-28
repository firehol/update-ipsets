package otelexporter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

type Protocol string

const (
	ProtocolHTTP Protocol = "http/protobuf"
	ProtocolGRPC Protocol = "grpc"
)

type MetricKind uint8

const (
	MetricCounter MetricKind = iota
	MetricGauge
	MetricDuration
)

type MetricLabel struct {
	Key   string
	Value string
}

type MetricSample struct {
	Name   string
	Kind   MetricKind
	Unit   string
	Labels []MetricLabel

	Value     int64
	Count     uint64
	SumMicros uint64
	MaxMicros uint64
}

type MetricSnapshotFunc func() []MetricSample

type MetricDescriptor struct {
	Name string
	Kind MetricKind
	Unit string
}

type Options struct {
	ServiceName string
	Version     string
	Protocol    Protocol
	Interval    time.Duration
	Descriptors []MetricDescriptor
	Snapshot    MetricSnapshotFunc
	Logger      *slog.Logger
}

type Exporter struct {
	provider     *sdkmetric.MeterProvider
	registration metric.Registration
}

func Start(ctx context.Context, opts Options) (*Exporter, error) {
	if opts.Snapshot == nil {
		return nil, errors.New("metric snapshot function is required")
	}
	if opts.ServiceName == "" {
		opts.ServiceName = "update-ipsets"
	}
	if opts.Version == "" {
		opts.Version = "dev"
	}
	if opts.Interval <= 0 {
		opts.Interval = 10 * time.Second
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(opts.ServiceName),
			semconv.ServiceNamespace("firehol"),
			semconv.ServiceVersion(opts.Version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create OTel resource: %w", err)
	}

	exporter, err := newMetricExporter(ctx, opts.Protocol)
	if err != nil {
		return nil, err
	}

	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(opts.Interval))
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)
	meter := provider.Meter("github.com/firehol/update-ipsets/exporter")

	instruments, err := buildObservableInstruments(meter, opts.Descriptors)
	if err != nil {
		_ = provider.Shutdown(ctx)
		return nil, err
	}
	registration, err := meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		instruments.observe(ctx, observer, opts.Snapshot())
		return nil
	}, instruments.observables...)
	if err != nil {
		_ = provider.Shutdown(ctx)
		return nil, fmt.Errorf("register OTel metric snapshot callback: %w", err)
	}

	return &Exporter{
		provider:     provider,
		registration: registration,
	}, nil
}

func (e *Exporter) Shutdown(ctx context.Context) error {
	if e == nil {
		return nil
	}
	var errs []error
	if e.registration != nil {
		errs = append(errs, e.registration.Unregister())
	}
	if e.provider != nil {
		errs = append(errs, e.provider.Shutdown(ctx))
	}
	return errors.Join(errs...)
}

func newMetricExporter(ctx context.Context, protocol Protocol) (sdkmetric.Exporter, error) {
	switch protocol {
	case "", ProtocolHTTP:
		return otlpmetrichttp.New(ctx)
	case ProtocolGRPC:
		return otlpmetricgrpc.New(ctx)
	default:
		return nil, fmt.Errorf("unsupported OTel protocol %q", protocol)
	}
}

type observableInstruments struct {
	counters      map[string]metric.Int64ObservableCounter
	gauges        map[string]metric.Int64ObservableGauge
	durationSums  map[string]metric.Float64ObservableCounter
	durationCount map[string]metric.Int64ObservableCounter
	durationMax   map[string]metric.Float64ObservableGauge
	observables   []metric.Observable
}

func buildObservableInstruments(meter metric.Meter, descriptors []MetricDescriptor) (*observableInstruments, error) {
	out := &observableInstruments{
		counters:      make(map[string]metric.Int64ObservableCounter),
		gauges:        make(map[string]metric.Int64ObservableGauge),
		durationSums:  make(map[string]metric.Float64ObservableCounter),
		durationCount: make(map[string]metric.Int64ObservableCounter),
		durationMax:   make(map[string]metric.Float64ObservableGauge),
	}
	for _, desc := range descriptors {
		switch desc.Kind {
		case MetricCounter:
			counter, err := meter.Int64ObservableCounter(desc.Name)
			if err != nil {
				return nil, err
			}
			out.counters[desc.Name] = counter
			out.observables = append(out.observables, counter)
		case MetricGauge:
			gauge, err := meter.Int64ObservableGauge(desc.Name)
			if err != nil {
				return nil, err
			}
			out.gauges[desc.Name] = gauge
			out.observables = append(out.observables, gauge)
		case MetricDuration:
			sum, err := meter.Float64ObservableCounter(desc.Name+".sum", metric.WithUnit(desc.Unit))
			if err != nil {
				return nil, err
			}
			count, err := meter.Int64ObservableCounter(desc.Name + ".count")
			if err != nil {
				return nil, err
			}
			maximum, err := meter.Float64ObservableGauge(desc.Name+".max", metric.WithUnit(desc.Unit))
			if err != nil {
				return nil, err
			}
			out.durationSums[desc.Name] = sum
			out.durationCount[desc.Name] = count
			out.durationMax[desc.Name] = maximum
			out.observables = append(out.observables, sum, count, maximum)
		}
	}
	return out, nil
}

func (i *observableInstruments) observe(ctx context.Context, observer metric.Observer, samples []MetricSample) {
	for _, sample := range samples {
		attrs := attrsFor(sample.Labels)
		switch sample.Kind {
		case MetricCounter:
			counter := i.counters[sample.Name]
			if counter != nil {
				observer.ObserveInt64(counter, sample.Value, metric.WithAttributes(attrs...))
			}
		case MetricGauge:
			gauge := i.gauges[sample.Name]
			if gauge != nil {
				observer.ObserveInt64(gauge, sample.Value, metric.WithAttributes(attrs...))
			}
		case MetricDuration:
			scale := durationScale(sample.Unit)
			if sum := i.durationSums[sample.Name]; sum != nil {
				observer.ObserveFloat64(sum, float64(sample.SumMicros)*scale, metric.WithAttributes(attrs...))
			}
			if count := i.durationCount[sample.Name]; count != nil {
				observer.ObserveInt64(count, int64(sample.Count), metric.WithAttributes(attrs...))
			}
			if maximum := i.durationMax[sample.Name]; maximum != nil {
				observer.ObserveFloat64(maximum, float64(sample.MaxMicros)*scale, metric.WithAttributes(attrs...))
			}
		}
	}
	_ = ctx
}

func attrsFor(labels []MetricLabel) []attribute.KeyValue {
	if len(labels) == 0 {
		return nil
	}
	out := make([]attribute.KeyValue, 0, len(labels))
	for _, label := range labels {
		if strings.TrimSpace(label.Key) == "" {
			continue
		}
		out = append(out, attribute.String(label.Key, label.Value))
	}
	return out
}

func durationScale(unit string) float64 {
	if unit == "s" {
		return 1.0 / 1_000_000.0
	}
	return 1.0 / 1000.0
}
