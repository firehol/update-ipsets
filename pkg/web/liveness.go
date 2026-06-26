package web

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"github.com/firehol/update-ipsets/pkg/systemd"
	"go.opentelemetry.io/otel/attribute"
)

const (
	systemdNotifyFailureLogInterval = time.Minute
	watchdogSelfHealthMinTick       = time.Second
	watchdogSelfHealthMaxTick       = 15 * time.Second
)

var (
	watchdogDiagnosticMaxGoroutines = 100
	watchdogDiagnosticMaxBytes      = 64 * 1024
	daemonPanicDiagnosticMaxBytes   = 16 * 1024
)

var (
	systemdReadyNotify    = systemd.Ready
	systemdStoppingNotify = systemd.Stopping
	systemdWatchdogNotify = systemd.Watchdog

	systemdNotifyLimiters sync.Map

	diagnosticCredentialPattern     = regexp.MustCompile(`(?i)\b(password|passwd|secret|token|authorization|api[_-]?key|rsync_password|cookie|set-cookie|request_body|body|payload)\s*[:=]\s*("[^"]*"|'[^']*'|\S+)`)
	diagnosticJSONCredentialPattern = regexp.MustCompile(`(?i)"(password|passwd|secret|token|authorization|api[_-]?key|rsync_password|cookie|request_body|body|payload)"\s*:\s*"[^"]*"`)
	diagnosticBearerPattern         = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]+`)
	diagnosticIPv4Pattern           = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	diagnosticLongPathPattern       = regexp.MustCompile(`(?:/[^\s:/]+){5,}[^\s:]*`)
)

type rateLimitedLogCounter struct {
	mu         sync.Mutex
	last       time.Time
	suppressed uint64
}

func watchdogSelfHealthTick(watchdogInterval time.Duration) time.Duration {
	if watchdogInterval <= 0 {
		return watchdogSelfHealthMaxTick
	}
	tick := watchdogInterval / 4
	if tick < watchdogSelfHealthMinTick {
		return watchdogSelfHealthMinTick
	}
	if tick > watchdogSelfHealthMaxTick {
		return watchdogSelfHealthMaxTick
	}
	return tick
}

func watchdogSelfHealthThreshold(watchdogInterval, notifyDeadline time.Duration) time.Duration {
	if watchdogInterval <= 0 {
		return watchdogSelfHealthMaxTick
	}
	threshold := watchdogInterval + notifyDeadline
	if alt := watchdogInterval * 3 / 2; alt > threshold {
		threshold = alt
	}
	return threshold
}

func startWatchdogSelfHealth(ctx context.Context, logger *slog.Logger, watchdogInterval, notifyDeadline time.Duration, lastBeat *atomic.Int64) <-chan struct{} {
	done := make(chan struct{})
	if ctx == nil || watchdogInterval <= 0 || lastBeat == nil {
		close(done)
		return done
	}
	logger = nonNilLogger(logger)
	tick := watchdogSelfHealthTick(watchdogInterval)
	threshold := watchdogSelfHealthThreshold(watchdogInterval, notifyDeadline)
	go func() {
		defer close(done)
		defer recoverDaemonControlPanic(logger, "watchdog_self_health")
		ticker := time.NewTicker(tick)
		defer ticker.Stop()
		var lastDiagnostic time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				last := time.Unix(0, lastBeat.Load())
				elapsed := now.Sub(last)
				if elapsed < threshold {
					continue
				}
				if !lastDiagnostic.IsZero() && now.Sub(lastDiagnostic) < threshold {
					continue
				}
				lastDiagnostic = now
				observability.Count(ctx, "daemon.watchdog.diagnostics", 1)
				logger.Error("watchdog heartbeat stalled",
					"elapsed_ms", elapsed.Milliseconds(),
					"threshold_ms", threshold.Milliseconds(),
					"goroutines", sanitizedGoroutineSample(watchdogDiagnosticMaxGoroutines, watchdogDiagnosticMaxBytes))
			}
		}
	}()
	return done
}

func reportSystemdNotifyError(ctx context.Context, logger *slog.Logger, kind string, err error) {
	if err == nil {
		return
	}
	logger = nonNilLogger(logger)
	if kind == "" {
		kind = "unknown"
	}
	observability.Count(ctx, "systemd.notify.failures", 1, attribute.String("systemd.notify.kind", kind))
	limiter := systemdNotifyLimiter(kind)
	if shouldLog, suppressed := limiter.allow(time.Now()); shouldLog {
		args := []any{"kind", kind, "error", err}
		if suppressed > 0 {
			args = append(args, "suppressed", suppressed)
		}
		logger.Warn("systemd notification failed", args...)
	}
}

func systemdNotifyLimiter(kind string) *rateLimitedLogCounter {
	actual, _ := systemdNotifyLimiters.LoadOrStore(kind, &rateLimitedLogCounter{})
	return actual.(*rateLimitedLogCounter)
}

func (l *rateLimitedLogCounter) allow(now time.Time) (bool, uint64) {
	if l == nil {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.last.IsZero() || now.Sub(l.last) >= systemdNotifyFailureLogInterval {
		suppressed := l.suppressed
		l.last = now
		l.suppressed = 0
		return true, suppressed
	}
	l.suppressed++
	return false, 0
}

func recoverDaemonControlPanic(logger *slog.Logger, name string) {
	if recovered := recover(); recovered != nil {
		reportDaemonControlPanic(logger, name, recovered)
	}
}

func reportDaemonControlPanic(logger *slog.Logger, name string, recovered any) {
	logger = nonNilLogger(logger)
	if name == "" {
		name = "unknown"
	}
	observability.Count(context.Background(), "daemon.goroutine.panics", 1, attribute.String("daemon.goroutine", name))
	logger.Error("daemon control goroutine panic recovered",
		"goroutine", name,
		"panic", recovered,
		"goroutines", sanitizedGoroutineSample(1, daemonPanicDiagnosticMaxBytes))
}

func sanitizedGoroutineSample(maxGoroutines, maxBytes int) string {
	if maxGoroutines <= 0 || maxBytes <= 0 {
		return ""
	}
	buf := make([]byte, maxBytes)
	n := runtime.Stack(buf, true)
	if n > len(buf) {
		n = len(buf)
	}
	text := limitGoroutineBlocks(string(buf[:n]), maxGoroutines)
	return SanitizeDiagnosticText(text, maxBytes)
}

func limitGoroutineBlocks(text string, maxGoroutines int) string {
	if maxGoroutines <= 0 || text == "" {
		return ""
	}
	const marker = "\ngoroutine "
	parts := strings.Split(text, marker)
	if len(parts) <= maxGoroutines {
		return text
	}
	var b strings.Builder
	for i := 0; i < maxGoroutines; i++ {
		if i == 0 {
			b.WriteString(parts[i])
			continue
		}
		b.WriteString(marker)
		b.WriteString(parts[i])
	}
	fmt.Fprintf(&b, "\n... %d goroutines omitted ...", len(parts)-maxGoroutines)
	return b.String()
}

func sanitizeDiagnosticText(text string) string {
	text = diagnosticJSONCredentialPattern.ReplaceAllString(text, `"$1":"[REDACTED]"`)
	text = diagnosticBearerPattern.ReplaceAllString(text, "Bearer [REDACTED]")
	text = diagnosticCredentialPattern.ReplaceAllString(text, "$1=[REDACTED]")
	text = diagnosticIPv4Pattern.ReplaceAllString(text, "[REDACTED_IP]")
	text = diagnosticLongPathPattern.ReplaceAllStringFunc(text, shortenDiagnosticPath)
	return text
}

func SanitizeDiagnosticText(text string, maxBytes int) string {
	text = sanitizeDiagnosticText(text)
	if maxBytes <= 0 || len(text) <= maxBytes {
		return text
	}
	const suffix = "\n... diagnostic truncated ..."
	if maxBytes > len(suffix) {
		return text[:maxBytes-len(suffix)] + suffix
	}
	return text[:maxBytes]
}

func shortenDiagnosticPath(path string) string {
	const maxPathBytes = 64
	if len(path) <= maxPathBytes {
		return path
	}
	parts := strings.Split(path, "/")
	if len(parts) <= 4 {
		return path
	}
	tail := strings.Join(parts[len(parts)-3:], "/")
	return ".../" + tail
}

func nonNilLogger(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}
