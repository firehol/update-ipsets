package observability

import (
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/firehol/update-ipsets/internal/observability/otelexporter"
)

type metricKind uint8

const (
	metricCounter metricKind = iota
	metricGauge
	metricDuration
)

const maxMetricLabels = 3

type metricDescriptor struct {
	name   string
	kind   metricKind
	unit   string
	labels []labelDescriptor
}

type labelDescriptor struct {
	key        string
	values     []string
	stringVals map[string]uint16
	intVals    map[int64]uint16
	boolVals   map[bool]uint16
}

type seriesKey struct {
	metric int
	values [maxMetricLabels]uint16
}

type metricSeries struct {
	desc   *metricDescriptor
	labels [maxMetricLabels]uint16

	touched atomic.Bool
	counter atomic.Int64
	gauge   atomic.Int64

	count     atomic.Uint64
	sumMicros atomic.Uint64
	maxMicros atomic.Uint64
}

type registry struct {
	descriptors []metricDescriptor
	byName      map[string]int
	series      []*metricSeries
	byKey       map[seriesKey]*metricSeries
}

var defaultRegistry = newRegistry(metricDescriptors())

func newRegistry(descriptors []metricDescriptor) *registry {
	r := &registry{
		descriptors: descriptors,
		byName:      make(map[string]int, len(descriptors)),
		byKey:       make(map[seriesKey]*metricSeries),
	}
	for i := range r.descriptors {
		desc := &r.descriptors[i]
		r.byName[desc.name] = i
		var values [maxMetricLabels]uint16
		r.addSeries(i, desc, 0, values)
	}
	return r
}

func (r *registry) addSeries(metric int, desc *metricDescriptor, label int, values [maxMetricLabels]uint16) {
	if label >= len(desc.labels) {
		series := &metricSeries{desc: desc, labels: values}
		r.series = append(r.series, series)
		r.byKey[seriesKey{metric: metric, values: values}] = series
		return
	}
	labelValues := desc.labels[label].values
	for i := range labelValues {
		next := values
		next[label] = uint16(i)
		r.addSeries(metric, desc, label+1, next)
	}
}

func (r *registry) addCounter(name string, count int64, attrs ...Attr) {
	series := r.lookup(name, attrs...)
	if series == nil || series.desc.kind != metricCounter {
		return
	}
	series.touched.Store(true)
	series.counter.Add(count)
}

func (r *registry) setGauge(name string, value int64, attrs ...Attr) {
	series := r.lookup(name, attrs...)
	if series == nil || series.desc.kind != metricGauge {
		return
	}
	series.gauge.Store(value)
	series.touched.Store(true)
}

func (r *registry) observeDuration(name string, dur time.Duration, attrs ...Attr) {
	series := r.lookup(name, attrs...)
	if series == nil || series.desc.kind != metricDuration {
		return
	}
	micros := uint64(dur.Microseconds())
	if micros == 0 {
		micros = 1
	}
	series.sumMicros.Add(micros)
	series.count.Add(1)
	atomicMaxUint64(&series.maxMicros, micros)
	series.touched.Store(true)
}

func (r *registry) lookup(name string, attrs ...Attr) *metricSeries {
	metric, ok := r.byName[name]
	if !ok {
		r.addUnknownMetric()
		return nil
	}
	desc := &r.descriptors[metric]
	var values [maxMetricLabels]uint16
	for i := range desc.labels {
		values[i] = desc.labels[i].valueFor(attrs)
	}
	return r.byKey[seriesKey{metric: metric, values: values}]
}

func (l labelDescriptor) valueFor(attrs []Attr) uint16 {
	for i := range attrs {
		attr := attrs[i]
		if attr.Key != l.key {
			continue
		}
		switch attr.kind {
		case attrInt:
			if idx, ok := l.intVals[attr.i]; ok {
				return idx
			}
		case attrBool:
			if idx, ok := l.boolVals[attr.b]; ok {
				return idx
			}
		default:
			value := strings.TrimSpace(attr.s)
			if idx, ok := l.stringVals[value]; ok {
				return idx
			}
			value = strings.ToLower(value)
			if idx, ok := l.stringVals[value]; ok {
				return idx
			}
		}
		return l.other()
	}
	return l.other()
}

func (l labelDescriptor) other() uint16 {
	if idx, ok := l.stringVals["other"]; ok {
		return idx
	}
	return 0
}

func (r *registry) addUnknownMetric() {
	metric, ok := r.byName["telemetry.metrics.unknown"]
	if !ok {
		return
	}
	series := r.byKey[seriesKey{metric: metric}]
	if series == nil {
		return
	}
	series.touched.Store(true)
	series.counter.Add(1)
}

func atomicMaxUint64(v *atomic.Uint64, next uint64) {
	for {
		current := v.Load()
		if next <= current {
			return
		}
		if v.CompareAndSwap(current, next) {
			return
		}
	}
}

type MetricSnapshot struct {
	Name   string
	Kind   metricKind
	Unit   string
	Labels []MetricLabelSnapshot

	Value     int64
	Count     uint64
	SumMicros uint64
	MaxMicros uint64
}

type MetricLabelSnapshot struct {
	Key   string
	Value string
}

func SnapshotMetrics() []MetricSnapshot {
	return defaultRegistry.snapshot()
}

func metricExportDescriptors() []otelexporter.MetricDescriptor {
	descriptors := make([]otelexporter.MetricDescriptor, 0, len(defaultRegistry.descriptors))
	for _, desc := range defaultRegistry.descriptors {
		descriptors = append(descriptors, otelexporter.MetricDescriptor{
			Name: desc.name,
			Kind: exportMetricKind(desc.kind),
			Unit: desc.unit,
		})
	}
	return descriptors
}

func metricExportSamples() []otelexporter.MetricSample {
	snapshots := SnapshotMetrics()
	out := make([]otelexporter.MetricSample, 0, len(snapshots))
	for _, snap := range snapshots {
		sample := otelexporter.MetricSample{
			Name:      snap.Name,
			Kind:      exportMetricKind(snap.Kind),
			Unit:      snap.Unit,
			Value:     snap.Value,
			Count:     snap.Count,
			SumMicros: snap.SumMicros,
			MaxMicros: snap.MaxMicros,
		}
		if len(snap.Labels) > 0 {
			sample.Labels = make([]otelexporter.MetricLabel, 0, len(snap.Labels))
			for _, label := range snap.Labels {
				sample.Labels = append(sample.Labels, otelexporter.MetricLabel{
					Key:   label.Key,
					Value: label.Value,
				})
			}
		}
		out = append(out, sample)
	}
	return out
}

func exportMetricKind(kind metricKind) otelexporter.MetricKind {
	switch kind {
	case metricCounter:
		return otelexporter.MetricCounter
	case metricGauge:
		return otelexporter.MetricGauge
	default:
		return otelexporter.MetricDuration
	}
}

func (r *registry) snapshot() []MetricSnapshot {
	out := make([]MetricSnapshot, 0, len(r.series))
	for _, series := range r.series {
		if !series.touched.Load() {
			continue
		}
		desc := series.desc
		snap := MetricSnapshot{
			Name: desc.name,
			Kind: desc.kind,
			Unit: desc.unit,
		}
		if len(desc.labels) > 0 {
			snap.Labels = make([]MetricLabelSnapshot, 0, len(desc.labels))
			for i, label := range desc.labels {
				snap.Labels = append(snap.Labels, MetricLabelSnapshot{
					Key:   label.key,
					Value: label.values[series.labels[i]],
				})
			}
		}
		switch desc.kind {
		case metricCounter:
			snap.Value = series.counter.Load()
		case metricGauge:
			snap.Value = series.gauge.Load()
		case metricDuration:
			snap.Count = series.count.Load()
			snap.SumMicros = series.sumMicros.Load()
			snap.MaxMicros = series.maxMicros.Load()
		}
		out = append(out, snap)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return labelsLess(out[i].Labels, out[j].Labels)
	})
	return out
}

func labelsLess(a, b []MetricLabelSnapshot) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i].Key != b[i].Key {
			return a[i].Key < b[i].Key
		}
		if a[i].Value != b[i].Value {
			return a[i].Value < b[i].Value
		}
	}
	return len(a) < len(b)
}
