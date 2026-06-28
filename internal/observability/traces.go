package observability

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

const maxTraceAttrs = 8
const maxTraceStringBytes = 256
const traceFlushTimeout = 5 * time.Second
const truncatedTraceValue = "[truncated]"

type traceEventKind uint8

const (
	traceEventStart traceEventKind = iota + 1
	traceEventEnd
)

type TraceEvent struct {
	ID       uint64
	Kind     traceEventKind
	Time     time.Time
	Name     string
	Duration time.Duration
	Error    bool
	Attrs    [maxTraceAttrs]Attr
	NAttrs   uint8
}

type traceQueue struct {
	queue  chan TraceEvent
	flush  chan chan struct{}
	stopC  chan struct{}
	stop   sync.Once
	closed atomic.Bool

	mu      sync.Mutex
	records []TraceEvent
	next    int
	count   int
}

var activeTraceQueue atomic.Pointer[traceQueue]

func newTraceQueue(bufferBytes int64) *traceQueue {
	capacity := traceQueueCapacity(bufferBytes)
	q := &traceQueue{
		queue:   make(chan TraceEvent, capacity),
		flush:   make(chan chan struct{}),
		stopC:   make(chan struct{}),
		records: make([]TraceEvent, capacity),
	}
	go q.run()
	return q
}

func traceQueueCapacity(bufferBytes int64) int {
	if bufferBytes <= 0 {
		return 0
	}
	recordSize := int64(unsafe.Sizeof(TraceEvent{}))
	if recordSize <= 0 {
		return 1
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

func configureTraceQueue(bufferBytes int64) {
	var next *traceQueue
	if bufferBytes > 0 {
		next = newTraceQueue(bufferBytes)
	}
	if previous := activeTraceQueue.Swap(next); previous != nil {
		previous.stopQueue()
	}
}

func enqueueTraceEvent(event TraceEvent) {
	queue := activeTraceQueue.Load()
	if queue == nil {
		return
	}
	queue.tryAppend(event)
}

func (q *traceQueue) tryAppend(event TraceEvent) {
	if q == nil || q.closed.Load() || q.queue == nil {
		return
	}
	select {
	case q.queue <- event:
	default:
		TryCount("telemetry.traces.dropped", 1)
	}
}

func (q *traceQueue) run() {
	defer func() {
		if recover() != nil {
			q.stop.Do(func() {
				q.closed.Store(true)
				close(q.stopC)
			})
		}
	}()
	for {
		select {
		case event := <-q.queue:
			q.appendRing(event)
		case ack := <-q.flush:
			q.drain()
			close(ack)
		case <-q.stopC:
			q.drain()
			return
		}
	}
}

func (q *traceQueue) stopQueue() {
	q.stop.Do(func() {
		q.closed.Store(true)
		close(q.stopC)
	})
}

func (q *traceQueue) drain() {
	for {
		select {
		case event := <-q.queue:
			q.appendRing(event)
		default:
			return
		}
	}
}

func (q *traceQueue) appendRing(event TraceEvent) {
	if q == nil || len(q.records) == 0 {
		TryCount("telemetry.traces.dropped", 1)
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.count < len(q.records) {
		idx := (q.next + q.count) % len(q.records)
		q.records[idx] = event
		q.count++
		return
	}
	q.records[q.next] = event
	q.next = (q.next + 1) % len(q.records)
	TryCount("telemetry.traces.dropped", 1)
}

func (q *traceQueue) snapshot() []TraceEvent {
	if q == nil {
		return nil
	}
	q.flushQueuedEvents()
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]TraceEvent, 0, q.count)
	for i := 0; i < q.count; i++ {
		idx := (q.next + i) % len(q.records)
		out = append(out, q.records[idx])
	}
	return out
}

func (q *traceQueue) flushQueuedEvents() {
	ack := make(chan struct{})
	select {
	case q.flush <- ack:
		select {
		case <-ack:
		case <-q.stopC:
			q.drain()
		case <-time.After(traceFlushTimeout):
		}
	case <-q.stopC:
		q.drain()
	}
}

func SnapshotTraceEvents() []TraceEvent {
	queue := activeTraceQueue.Load()
	if queue == nil {
		return nil
	}
	return queue.snapshot()
}

func copyTraceAttrs(dst *[maxTraceAttrs]Attr, attrs []Attr) uint8 {
	limit := len(attrs)
	if limit > len(dst) {
		limit = len(dst)
	}
	for i := 0; i < limit; i++ {
		dst[i] = boundedTraceAttr(attrs[i])
	}
	return uint8(limit)
}

func boundedTraceAttr(attr Attr) Attr {
	attr.Key = boundedTraceString(attr.Key)
	if attr.kind == attrString {
		attr.s = boundedTraceString(attr.s)
	}
	return attr
}

func boundedTraceString(value string) string {
	if len(value) <= maxTraceStringBytes {
		return value
	}
	return truncatedTraceValue
}
