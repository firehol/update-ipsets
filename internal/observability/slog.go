package observability

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

const asyncLogQueueSize = 8192
const maxAsyncLogAttrs = 8
const maxAsyncLogStringBytes = 4096
const truncatedLogValue = "[truncated]"

type asyncLogRecord struct {
	handler slog.Handler
	time    time.Time
	level   slog.Level
	pc      uintptr
	message string
	attrs   [maxAsyncLogAttrs]slog.Attr
	nattrs  uint8
}

type asyncLogState struct {
	queue  chan asyncLogRecord
	done   chan struct{}
	stopC  chan struct{}
	stop   sync.Once
	closed atomic.Bool
}

type asyncLogHandler struct {
	handler slog.Handler
	state   *asyncLogState
}

func newAsyncLogHandler(handler slog.Handler, queueSize int) *asyncLogHandler {
	if queueSize <= 0 {
		queueSize = asyncLogQueueSize
	}
	state := &asyncLogState{
		queue: make(chan asyncLogRecord, queueSize),
		done:  make(chan struct{}),
		stopC: make(chan struct{}),
	}
	h := &asyncLogHandler{
		handler: handler,
		state:   state,
	}
	go state.run()
	return h
}

func (h *asyncLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h != nil && h.handler != nil && h.state != nil && h.handler.Enabled(ctx, level)
}

func (h *asyncLogHandler) Handle(_ context.Context, record slog.Record) error {
	if h == nil || h.handler == nil || h.state == nil {
		return nil
	}
	if h.state.closed.Load() {
		TryCount("telemetry.logs.dropped", 1)
		return nil
	}
	event := asyncLogRecord{
		handler: h.handler,
		time:    record.Time,
		level:   record.Level,
		pc:      record.PC,
		message: boundedLogString(record.Message),
	}
	record.Attrs(func(attr slog.Attr) bool {
		if int(event.nattrs) >= len(event.attrs) {
			return false
		}
		event.attrs[event.nattrs] = boundedLogAttr(attr)
		event.nattrs++
		return true
	})
	select {
	case h.state.queue <- event:
	default:
		TryCount("telemetry.logs.dropped", 1)
	}
	return nil
}

func (h *asyncLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if h == nil || h.handler == nil || h.state == nil {
		return h
	}
	bounded := make([]slog.Attr, len(attrs))
	for i := range attrs {
		bounded[i] = boundedLogAttr(attrs[i])
	}
	return &asyncLogHandler{
		handler: h.handler.WithAttrs(bounded),
		state:   h.state,
	}
}

func (h *asyncLogHandler) WithGroup(name string) slog.Handler {
	if h == nil || h.handler == nil || h.state == nil {
		return h
	}
	return &asyncLogHandler{
		handler: h.handler.WithGroup(name),
		state:   h.state,
	}
}

func (h *asyncLogHandler) Shutdown(ctx context.Context) error {
	if h == nil || h.state == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	h.state.stop.Do(func() {
		h.state.closed.Store(true)
		close(h.state.stopC)
	})
	select {
	case <-h.state.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *asyncLogState) run() {
	defer close(s.done)
	for {
		select {
		case event := <-s.queue:
			handleAsyncLogRecordSafely(event)
		case <-s.stopC:
			for {
				select {
				case event := <-s.queue:
					handleAsyncLogRecordSafely(event)
				default:
					return
				}
			}
		}
	}
}

func handleAsyncLogRecordSafely(event asyncLogRecord) {
	defer func() {
		if recover() != nil {
			TryCount("telemetry.logs.dropped", 1)
		}
	}()
	handleAsyncLogRecord(event)
}

func handleAsyncLogRecord(event asyncLogRecord) {
	if event.handler == nil {
		return
	}
	record := slog.NewRecord(event.time, event.level, event.message, event.pc)
	if event.nattrs > 0 {
		record.AddAttrs(event.attrs[:event.nattrs]...)
	}
	_ = event.handler.Handle(context.Background(), record)
}

func asyncLogQueueCapacity(bufferBytes int64) int {
	if bufferBytes <= 0 {
		return asyncLogQueueSize
	}
	recordSize := int64(unsafe.Sizeof(asyncLogRecord{}))
	if recordSize <= 0 {
		return asyncLogQueueSize
	}
	capacity := bufferBytes / recordSize
	if capacity < 1 {
		return 1
	}
	if capacity > int64(^uint(0)>>1) {
		return int(^uint(0) >> 1)
	}
	return int(capacity)
}

func boundedLogAttr(attr slog.Attr) slog.Attr {
	attr.Key = boundedLogString(attr.Key)
	switch attr.Value.Kind() {
	case slog.KindString:
		attr.Value = slog.StringValue(boundedLogString(attr.Value.String()))
	case slog.KindAny:
		if value, ok := attr.Value.Any().(string); ok {
			attr.Value = slog.StringValue(boundedLogString(value))
		}
	}
	return attr
}

func boundedLogString(value string) string {
	if len(value) <= maxAsyncLogStringBytes {
		return value
	}
	return truncatedLogValue
}
