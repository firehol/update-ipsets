package observability

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAsyncLogHandlerDropsInsteadOfBlocking(t *testing.T) {
	resetMetricsForTest()

	blocking := newBlockingSlogHandler()
	handler := newAsyncLogHandler(blocking, 1)
	logger := slog.New(handler)

	logger.Info("first")
	<-blocking.entered

	logger.Info("second")
	logger.Info("third")

	if got := counterValue(t, "telemetry.logs.dropped", nil); got == 0 {
		t.Fatal("telemetry.logs.dropped = 0, want a dropped log record")
	}
	close(blocking.release)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := handler.Shutdown(ctx); err != nil {
		t.Fatalf("async log handler shutdown: %v", err)
	}
}

func TestAsyncLogHandlerCountsRecordsAfterShutdownAsDropped(t *testing.T) {
	resetMetricsForTest()

	handler := newAsyncLogHandler(slog.DiscardHandler, 1)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := handler.Shutdown(ctx); err != nil {
		t.Fatalf("async log handler shutdown: %v", err)
	}
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "late message", 0)
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if got := counterValue(t, "telemetry.logs.dropped", nil); got != 1 {
		t.Fatalf("telemetry.logs.dropped after shutdown = %d, want 1", got)
	}
}

func TestAsyncLogHandlerTruncatesOversizedStringPayloads(t *testing.T) {
	resetMetricsForTest()

	capture := newCaptureSlogHandler()
	handler := newAsyncLogHandler(capture, 4)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		_ = handler.Shutdown(ctx)
	})
	record := slog.NewRecord(time.Now(), slog.LevelInfo, strings.Repeat("m", maxAsyncLogStringBytes+1), 0)
	record.AddAttrs(slog.String("oversized", strings.Repeat("v", maxAsyncLogStringBytes+1)))
	record.AddAttrs(slog.String(strings.Repeat("k", maxAsyncLogStringBytes+1), "value"))
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	got := capture.wait(t)
	if got.Message != truncatedLogValue {
		t.Fatalf("logged message = %q, want truncated marker", got.Message)
	}
	var value string
	got.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "oversized" {
			value = attr.Value.String()
		}
		if attr.Value.String() == "value" {
			if attr.Key != truncatedLogValue {
				t.Fatalf("logged attr key = %q, want truncated marker", attr.Key)
			}
		}
		return true
	})
	if value != truncatedLogValue {
		t.Fatalf("logged attr value = %q, want truncated marker", value)
	}
}

func TestAsyncLogHandlerRecoversFromDownstreamPanic(t *testing.T) {
	resetMetricsForTest()

	sink := newPanicOnceSlogHandler()
	handler := newAsyncLogHandler(sink, 4)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		_ = handler.Shutdown(ctx)
	})
	logger := slog.New(handler)

	logger.Info("first")
	waitForCounterAtLeast(t, "telemetry.logs.dropped", nil, 1)
	logger.Info("second")

	got := sink.wait(t)
	if got.Message != "second" {
		t.Fatalf("captured log after panic = %q, want second", got.Message)
	}
}

type blockingSlogHandler struct {
	entered     chan struct{}
	release     chan struct{}
	enteredOnce sync.Once
}

func newBlockingSlogHandler() *blockingSlogHandler {
	return &blockingSlogHandler{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (h *blockingSlogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *blockingSlogHandler) Handle(context.Context, slog.Record) error {
	h.enteredOnce.Do(func() { close(h.entered) })
	<-h.release
	return nil
}

func (h *blockingSlogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *blockingSlogHandler) WithGroup(string) slog.Handler {
	return h
}

type captureSlogHandler struct {
	records chan slog.Record
}

func newCaptureSlogHandler() *captureSlogHandler {
	return &captureSlogHandler{records: make(chan slog.Record, 1)}
}

func (h *captureSlogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureSlogHandler) Handle(_ context.Context, record slog.Record) error {
	h.records <- record
	return nil
}

func (h *captureSlogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *captureSlogHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *captureSlogHandler) wait(t *testing.T) slog.Record {
	t.Helper()
	select {
	case record := <-h.records:
		return record
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for captured slog record")
		return slog.Record{}
	}
}

type panicOnceSlogHandler struct {
	panicked atomic.Bool
	records  chan slog.Record
}

func newPanicOnceSlogHandler() *panicOnceSlogHandler {
	return &panicOnceSlogHandler{records: make(chan slog.Record, 1)}
}

func (h *panicOnceSlogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *panicOnceSlogHandler) Handle(_ context.Context, record slog.Record) error {
	if !h.panicked.Swap(true) {
		panic("test slog handler panic")
	}
	h.records <- record
	return nil
}

func (h *panicOnceSlogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *panicOnceSlogHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *panicOnceSlogHandler) wait(t *testing.T) slog.Record {
	t.Helper()
	select {
	case record := <-h.records:
		return record
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for recovered slog handler record")
		return slog.Record{}
	}
}
