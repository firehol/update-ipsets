package observability

import (
	"testing"
	"time"
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
