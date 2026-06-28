package observability

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func enabledFromEnv() bool {
	updateIPSetsOTel := strings.ToLower(strings.TrimSpace(os.Getenv("UPDATE_IPSETS_OTEL")))
	if updateIPSetsOTel == "0" || updateIPSetsOTel == "false" || updateIPSetsOTel == "disabled" {
		return false
	}
	if updateIPSetsOTel == "1" || updateIPSetsOTel == "true" || updateIPSetsOTel == "enabled" {
		return true
	}
	for _, key := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
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

func metricReaderOptionsFromEnv() ([]time.Duration, error) {
	raw := firstEnv("UPDATE_IPSETS_OTEL_METRIC_INTERVAL", "OTEL_METRIC_EXPORT_INTERVAL")
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	interval, err := parseMetricExportInterval(raw)
	if err != nil {
		return nil, err
	}
	return []time.Duration{interval}, nil
}

func telemetryBufferBudgetsFromEnv() (int64, int64, error) {
	total := DefaultLogTraceBufferBytes
	if raw := firstEnv("UPDATE_IPSETS_TELEMETRY_BUFFER_BYTES"); raw != "" {
		parsed, err := parseBufferBytes(raw)
		if err != nil {
			return DefaultLogTraceBufferBytes, 0, err
		}
		total = parsed
	}
	logBytes := total
	traceBytes := int64(0)
	if raw := firstEnv("UPDATE_IPSETS_LOG_BUFFER_BYTES"); raw != "" {
		parsed, err := parseBufferBytes(raw)
		if err != nil {
			return DefaultLogTraceBufferBytes, 0, err
		}
		logBytes = parsed
	}
	if raw := firstEnv("UPDATE_IPSETS_TRACE_BUFFER_BYTES"); raw != "" {
		parsed, err := parseTraceBufferBytes(raw)
		if err != nil {
			return DefaultLogTraceBufferBytes, 0, err
		}
		traceBytes = parsed
	}
	return logBytes, traceBytes, nil
}

func parseBufferBytes(raw string) (int64, error) {
	return parseBufferBytesValue(raw, false)
}

func parseTraceBufferBytes(raw string) (int64, error) {
	return parseBufferBytesValue(raw, true)
}

func parseBufferBytesValue(raw string, allowZero bool) (int64, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return 0, nil
	}
	multiplier := int64(1)
	for _, suffix := range []struct {
		text string
		mul  int64
	}{
		{text: "gib", mul: 1024 * 1024 * 1024},
		{text: "gb", mul: 1024 * 1024 * 1024},
		{text: "mib", mul: 1024 * 1024},
		{text: "mb", mul: 1024 * 1024},
		{text: "kib", mul: 1024},
		{text: "kb", mul: 1024},
		{text: "b", mul: 1},
	} {
		if strings.HasSuffix(trimmed, suffix.text) {
			multiplier = suffix.mul
			trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, suffix.text))
			break
		}
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid telemetry buffer size %q: %w", raw, err)
	}
	if value == 0 && allowZero {
		return 0, nil
	}
	if value <= 0 {
		return 0, fmt.Errorf("telemetry buffer size must be positive, got %q", raw)
	}
	if value > (1<<63-1)/multiplier {
		return 0, fmt.Errorf("telemetry buffer size %q is too large", raw)
	}
	return value * multiplier, nil
}
