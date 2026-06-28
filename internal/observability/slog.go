package observability

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
)

const asyncLogQueueSize = 8192

type teeHandler struct {
	handlers []slog.Handler
}

func newTeeHandler(handlers ...slog.Handler) slog.Handler {
	out := &teeHandler{}
	for _, h := range handlers {
		if h != nil {
			out.handlers = append(out.handlers, h)
		}
	}
	return out
}

func (h *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *teeHandler) Handle(ctx context.Context, record slog.Record) error {
	var first error
	for _, handler := range h.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}
		if err := handler.Handle(ctx, record.Clone()); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (h *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithAttrs(attrs))
	}
	return &teeHandler{handlers: handlers}
}

func (h *teeHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithGroup(name))
	}
	return &teeHandler{handlers: handlers}
}

type asyncLogRecord struct {
	handler slog.Handler
	record  slog.Record
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
		return nil
	}
	select {
	case h.state.queue <- asyncLogRecord{handler: h.handler, record: record.Clone()}:
	default:
	}
	return nil
}

func (h *asyncLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if h == nil || h.handler == nil || h.state == nil {
		return h
	}
	return &asyncLogHandler{
		handler: h.handler.WithAttrs(attrs),
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
			handleAsyncLogRecord(event)
		case <-s.stopC:
			for {
				select {
				case event := <-s.queue:
					handleAsyncLogRecord(event)
				default:
					return
				}
			}
		}
	}
}

func handleAsyncLogRecord(event asyncLogRecord) {
	if event.handler == nil {
		return
	}
	_ = event.handler.Handle(context.Background(), event.record)
}
