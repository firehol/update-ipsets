package observability

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
)

func newPrometheusMetrics() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		writePrometheusMetrics(w, SnapshotMetrics())
	})
}

func writePrometheusMetrics(w io.Writer, snapshots []MetricSnapshot) {
	for _, snap := range snapshots {
		switch snap.Kind {
		case metricCounter:
			name := prometheusName(snap.Name) + "_total"
			fmt.Fprintf(w, "# TYPE %s counter\n", name)
			writePrometheusSample(w, name, snap.Labels, float64(snap.Value))
		case metricGauge:
			name := prometheusName(snap.Name)
			fmt.Fprintf(w, "# TYPE %s gauge\n", name)
			writePrometheusSample(w, name, snap.Labels, float64(snap.Value))
		case metricDuration:
			name := prometheusName(snap.Name)
			fmt.Fprintf(w, "# TYPE %s untyped\n", name)
			scale := durationPrometheusScale(snap.Unit)
			writePrometheusSample(w, name+"_sum", snap.Labels, float64(snap.SumMicros)*scale)
			writePrometheusSample(w, name+"_count", snap.Labels, float64(snap.Count))
			writePrometheusSample(w, name+"_max", snap.Labels, float64(snap.MaxMicros)*scale)
		}
	}
}

func writePrometheusSample(w io.Writer, name string, labels []MetricLabelSnapshot, value float64) {
	fmt.Fprint(w, name)
	if len(labels) > 0 {
		fmt.Fprint(w, "{")
		for i, label := range labels {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, `%s="%s"`, prometheusLabelName(label.Key), escapePrometheusLabelValue(label.Value))
		}
		fmt.Fprint(w, "}")
	}
	if math.Trunc(value) == value {
		fmt.Fprintf(w, " %.0f\n", value)
		return
	}
	fmt.Fprintf(w, " %g\n", value)
}

func durationPrometheusScale(unit string) float64 {
	if unit == "s" {
		return 1.0 / 1_000_000.0
	}
	return 1.0 / 1000.0
}

func prometheusName(name string) string {
	return prometheusLabelName(name)
}

func prometheusLabelName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

func escapePrometheusLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}
