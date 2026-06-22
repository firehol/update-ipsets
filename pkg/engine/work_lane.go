package engine

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"

	"go.opentelemetry.io/otel/attribute"
)

var (
	ErrLaneShuttingDown         = errors.New("engine work lane is shutting down")
	ErrLaneMissingCoalescingKey = errors.New("engine work lane missing coalescing key")
	ErrLaneReentrantRun         = errors.New("engine work lane run called from active worker")
	ErrLanePanic                = errors.New("engine work lane callback panicked")
)

type LaneWorkKind string

const (
	LaneWorkEngineRun          LaneWorkKind = "engine_run"
	LaneWorkEntityRebuild      LaneWorkKind = "entity_rebuild"
	LaneWorkEntityRefresh      LaneWorkKind = "entity_refresh"
	LaneWorkEntityRepair       LaneWorkKind = "entity_repair"
	LaneWorkIntegrityRefresh   LaneWorkKind = "integrity_refresh"
	LaneWorkIntegrityReprocess LaneWorkKind = "integrity_reprocess"
	LaneWorkCleanup            LaneWorkKind = "cleanup"
)

type LaneWorkComponent string

const (
	LaneComponentEngineRun              LaneWorkComponent = "engine_run"
	LaneComponentEntityArtifacts        LaneWorkComponent = "entity_artifacts"
	LaneComponentEntityArtifactsHealth  LaneWorkComponent = "entity_artifacts_health"
	LaneComponentEntityIntegrity        LaneWorkComponent = "entity_integrity"
	LaneComponentPipelineIntegrity      LaneWorkComponent = "pipeline_integrity"
	LaneComponentCriticalInfrastructure LaneWorkComponent = "critical_infrastructure"
	LaneComponentPublishStages          LaneWorkComponent = "publish_stages"
)

type LaneWorkState string

const (
	LaneWorkQueued    LaneWorkState = "queued"
	LaneWorkActive    LaneWorkState = "active"
	LaneWorkCompleted LaneWorkState = "completed"
	LaneWorkFailed    LaneWorkState = "failed"
	LaneWorkCanceled  LaneWorkState = "canceled"
	LaneWorkSkipped   LaneWorkState = "skipped"
)

type LaneWork struct {
	ID            string            `json:"id,omitempty"`
	Kind          LaneWorkKind      `json:"kind,omitempty"`
	Component     LaneWorkComponent `json:"component,omitempty"`
	Name          string            `json:"name,omitempty"`
	Trigger       string            `json:"trigger,omitempty"`
	Phase         string            `json:"phase,omitempty"`
	Stage         string            `json:"stage,omitempty"`
	Detail        string            `json:"detail,omitempty"`
	CoalescingKey string            `json:"-"`
	QueuedAt      time.Time         `json:"queued_at,omitempty"`
}

type LaneTicket struct {
	ID        string            `json:"id"`
	Kind      LaneWorkKind      `json:"kind,omitempty"`
	Component LaneWorkComponent `json:"component,omitempty"`
	Queued    bool              `json:"queued"`
	Coalesced bool              `json:"coalesced"`
	State     LaneWorkState     `json:"state"`
}

type LaneSnapshot struct {
	Limit        int                `json:"limit"`
	ActiveCount  int                `json:"active_count"`
	WaitingCount int                `json:"waiting_count"`
	Active       []LaneWorkSnapshot `json:"active,omitempty"`
	Waiting      []LaneWorkSnapshot `json:"waiting,omitempty"`
}

type LaneWorkSnapshot struct {
	ID        string            `json:"id"`
	Kind      LaneWorkKind      `json:"kind,omitempty"`
	Component LaneWorkComponent `json:"component,omitempty"`
	Name      string            `json:"name,omitempty"`
	Trigger   string            `json:"trigger,omitempty"`
	Phase     string            `json:"phase,omitempty"`
	Stage     string            `json:"stage,omitempty"`
	Detail    string            `json:"detail,omitempty"`
	State     LaneWorkState     `json:"state"`
	QueuedAt  time.Time         `json:"queued_at,omitempty"`
	StartedAt time.Time         `json:"started_at,omitempty"`
	WaitMS    int64             `json:"wait_ms"`
	ElapsedMS int64             `json:"elapsed_ms"`
}

type WorkLane struct {
	mu         sync.Mutex
	limit      int
	nextSeq    uint64
	queue      []*laneItem
	active     map[string]*laneItem
	coalescing map[string]*laneItem
	idle       chan struct{}
	shutdown   bool
}

type laneContextKey struct{}

type laneStart struct {
	ctx context.Context
	err error
}

type laneItem struct {
	seq       uint64
	work      LaneWork
	fn        func(context.Context) error
	syncStart chan laneStart
	ctx       context.Context
	cancel    context.CancelFunc
	state     LaneWorkState
	startedAt time.Time
}

func NewWorkLane(limit int) *WorkLane {
	if limit < 1 {
		limit = 1
	}
	idle := make(chan struct{})
	close(idle)
	return &WorkLane{
		limit:      limit,
		active:     make(map[string]*laneItem),
		coalescing: make(map[string]*laneItem),
		idle:       idle,
	}
}

func (l *WorkLane) Run(ctx context.Context, work LaneWork, fn func(context.Context) error) error {
	ctx = nonNilContext(ctx)
	if ctx.Value(laneContextKey{}) == l {
		return ErrLaneReentrantRun
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	if fn == nil {
		return errors.New("engine work lane run requires callback")
	}
	item := l.newItem(ctx, work, fn)
	item.syncStart = make(chan laneStart, 1)

	l.mu.Lock()
	if l.shutdown {
		l.mu.Unlock()
		return ErrLaneShuttingDown
	}
	l.queue = append(l.queue, item)
	starts := l.scheduleLocked(time.Now().UTC())
	l.mu.Unlock()
	l.startAsync(starts)

	for {
		select {
		case start := <-item.syncStart:
			if start.err != nil {
				return start.err
			}
			err := runLaneCallback(start.ctx, fn)
			l.finishItem(item, err)
			if item.cancel != nil {
				item.cancel()
			}
			return err
		case <-ctx.Done():
			l.mu.Lock()
			if item.state == LaneWorkQueued {
				l.removeQueuedLocked(item)
				item.state = LaneWorkCanceled
				l.mu.Unlock()
				return ctx.Err()
			}
			l.mu.Unlock()
		}
	}
}

func (l *WorkLane) Submit(ctx context.Context, work LaneWork, fn func(context.Context) error) (LaneTicket, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return LaneTicket{}, err
	}
	if fn == nil {
		return LaneTicket{}, errors.New("engine work lane submit requires callback")
	}
	if strings.TrimSpace(work.CoalescingKey) == "" {
		return LaneTicket{}, ErrLaneMissingCoalescingKey
	}
	item := l.newItem(ctx, work, fn)

	l.mu.Lock()
	if l.shutdown {
		l.mu.Unlock()
		return LaneTicket{}, ErrLaneShuttingDown
	}
	if existing := l.coalescing[work.CoalescingKey]; existing != nil {
		ticket := existing.ticketLocked(false, true)
		l.mu.Unlock()
		return ticket, nil
	}
	l.coalescing[work.CoalescingKey] = item
	l.queue = append(l.queue, item)
	starts := l.scheduleLocked(time.Now().UTC())
	ticket := item.ticketLocked(item.state == LaneWorkQueued, false)
	l.mu.Unlock()
	l.startAsync(starts)
	return ticket, nil
}

func (l *WorkLane) TryRun(ctx context.Context, work LaneWork, fn func(context.Context) error) (bool, error) {
	ctx = nonNilContext(ctx)
	if ctx.Value(laneContextKey{}) == l {
		return false, ErrLaneReentrantRun
	}
	if err := contextErr(ctx); err != nil {
		return false, err
	}
	if fn == nil {
		return false, errors.New("engine work lane try-run requires callback")
	}
	item := l.newItem(ctx, work, fn)

	l.mu.Lock()
	if l.shutdown {
		l.mu.Unlock()
		return false, ErrLaneShuttingDown
	}
	if len(l.queue) > 0 || len(l.active) >= l.limit {
		l.mu.Unlock()
		return false, nil
	}
	now := time.Now().UTC()
	l.activateLocked(item, now)
	l.observeWorkerStartLocked(item, now)
	l.mu.Unlock()
	err := runLaneCallback(item.ctx, fn)
	l.finishItem(item, err)
	if item.cancel != nil {
		item.cancel()
	}
	return true, err
}

func (l *WorkLane) SetLimit(limit int) {
	if l == nil {
		return
	}
	if limit < 1 {
		limit = 1
	}
	l.mu.Lock()
	l.limit = limit
	starts := l.scheduleLocked(time.Now().UTC())
	l.observeWorkerGaugeLocked(context.Background())
	l.mu.Unlock()
	l.startAsync(starts)
}

func (l *WorkLane) Snapshot() LaneSnapshot {
	if l == nil {
		return LaneSnapshot{}
	}
	now := time.Now().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()
	active := make([]LaneWorkSnapshot, 0, len(l.active))
	for _, item := range l.active {
		active = append(active, item.snapshot(now))
	}
	slices.SortFunc(active, func(a, b LaneWorkSnapshot) int {
		return a.QueuedAt.Compare(b.QueuedAt)
	})
	waiting := make([]LaneWorkSnapshot, 0, len(l.queue))
	for _, item := range l.queue {
		if item.state == LaneWorkQueued {
			waiting = append(waiting, item.snapshot(now))
		}
	}
	return LaneSnapshot{
		Limit:        l.limit,
		ActiveCount:  len(active),
		WaitingCount: len(waiting),
		Active:       active,
		Waiting:      waiting,
	}
}

func (l *WorkLane) Shutdown(grace time.Duration) {
	if l == nil {
		return
	}
	var cancels []context.CancelFunc
	l.mu.Lock()
	if !l.shutdown {
		l.shutdown = true
	}
	for _, item := range l.queue {
		item.state = LaneWorkSkipped
		if item.cancel != nil {
			cancels = append(cancels, item.cancel)
		}
		if item.syncStart != nil {
			item.syncStart <- laneStart{err: ErrLaneShuttingDown}
		}
		if key := strings.TrimSpace(item.work.CoalescingKey); key != "" {
			delete(l.coalescing, key)
		}
	}
	l.queue = nil
	for _, item := range l.active {
		if item.cancel != nil {
			cancels = append(cancels, item.cancel)
		}
	}
	idle := l.idle
	l.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
	if grace <= 0 {
		return
	}
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-idle:
	case <-timer.C:
	}
}

func (l *WorkLane) AttachContext(ctx context.Context, grace time.Duration) {
	if l == nil || ctx == nil {
		return
	}
	go func() {
		<-ctx.Done()
		l.Shutdown(grace)
	}()
}

func (e *Engine) AttachWorkLaneContext(ctx context.Context, grace time.Duration) {
	if e == nil || e.engineLane == nil {
		return
	}
	e.engineLane.AttachContext(ctx, grace)
}

func (l *WorkLane) newItem(ctx context.Context, work LaneWork, fn func(context.Context) error) *laneItem {
	l.mu.Lock()
	l.nextSeq++
	seq := l.nextSeq
	l.mu.Unlock()
	now := time.Now().UTC()
	if work.QueuedAt.IsZero() {
		work.QueuedAt = now
	}
	if strings.TrimSpace(work.ID) == "" {
		prefix := string(work.Kind)
		if prefix == "" {
			prefix = "work"
		}
		work.ID = fmt.Sprintf("%s:%d", prefix, seq)
	}
	if strings.TrimSpace(work.Name) == "" {
		work.Name = string(work.Kind)
	}
	return &laneItem{
		ctx:   ctx,
		seq:   seq,
		work:  work,
		fn:    fn,
		state: LaneWorkQueued,
	}
}

func (l *WorkLane) scheduleLocked(now time.Time) []*laneItem {
	if l == nil || l.shutdown {
		return nil
	}
	var starts []*laneItem
	for len(l.active) < l.limit && len(l.queue) > 0 {
		item := l.queue[0]
		l.queue = l.queue[1:]
		if item.state != LaneWorkQueued {
			continue
		}
		l.activateLocked(item, now)
		l.observeWorkerStartLocked(item, now)
		if item.syncStart != nil {
			item.syncStart <- laneStart{ctx: item.ctx}
			continue
		}
		starts = append(starts, item)
	}
	return starts
}

func (l *WorkLane) activateLocked(item *laneItem, now time.Time) {
	if len(l.active) == 0 {
		l.idle = make(chan struct{})
	}
	item.state = LaneWorkActive
	item.startedAt = now
	ctx, cancel := context.WithCancel(item.ctxOrBackground())
	item.ctx = context.WithValue(ctx, laneContextKey{}, l)
	item.cancel = cancel
	l.active[item.work.ID] = item
}

func (l *WorkLane) observeWorkerStartLocked(item *laneItem, now time.Time) {
	if l == nil || item == nil {
		return
	}
	ctx := item.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	l.observeWorkerGaugeLocked(ctx)
	wait := now.Sub(item.work.QueuedAt)
	if wait > 0 {
		observability.Duration(ctx, "background.worker.wait", wait, laneWorkMetricAttrs(item.work)...)
	}
}

func (l *WorkLane) observeWorkerGaugeLocked(ctx context.Context) {
	if l == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	observability.Gauge(ctx, "background.workers.limit", int64(l.limit))
	observability.Gauge(ctx, "background.workers.active", int64(len(l.active)))
}

func laneWorkMetricAttrs(work LaneWork) []attribute.KeyValue {
	component := backgroundMetricComponent(work.Kind, work.Component)
	if component == "" {
		component = "other"
	}
	kind := string(work.Kind)
	if kind == "" {
		kind = "unknown"
	}
	return []attribute.KeyValue{
		attribute.String("background.component", component),
		attribute.String("engine.work.kind", kind),
	}
}

func (l *WorkLane) startAsync(items []*laneItem) {
	for _, item := range items {
		go func(item *laneItem) {
			err := runLaneCallback(item.ctx, item.fn)
			l.finishItem(item, err)
			if item.cancel != nil {
				item.cancel()
			}
		}(item)
	}
}

func runLaneCallback(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%w: %v", ErrLanePanic, recovered)
		}
	}()
	return fn(ctx)
}

func (l *WorkLane) finishItem(item *laneItem, err error) {
	if l == nil || item == nil {
		return
	}
	l.mu.Lock()
	if err != nil {
		item.state = LaneWorkFailed
	} else if contextErr(item.ctx) != nil {
		item.state = LaneWorkCanceled
	} else {
		item.state = LaneWorkCompleted
	}
	delete(l.active, item.work.ID)
	if key := strings.TrimSpace(item.work.CoalescingKey); key != "" {
		delete(l.coalescing, key)
	}
	if len(l.active) == 0 {
		close(l.idle)
	}
	starts := l.scheduleLocked(time.Now().UTC())
	l.observeWorkerGaugeLocked(item.ctx)
	l.mu.Unlock()
	l.startAsync(starts)
}

func (l *WorkLane) removeQueuedLocked(item *laneItem) {
	for i, queued := range l.queue {
		if queued == item {
			l.queue = append(l.queue[:i], l.queue[i+1:]...)
			break
		}
	}
	if key := strings.TrimSpace(item.work.CoalescingKey); key != "" {
		delete(l.coalescing, key)
	}
}

func (item *laneItem) ctxOrBackground() context.Context {
	if item.ctx != nil {
		return item.ctx
	}
	return context.Background()
}

func (item *laneItem) ticketLocked(queued, coalesced bool) LaneTicket {
	return LaneTicket{
		ID:        item.work.ID,
		Kind:      item.work.Kind,
		Component: item.work.Component,
		Queued:    queued,
		Coalesced: coalesced,
		State:     item.state,
	}
}

func (item *laneItem) snapshot(now time.Time) LaneWorkSnapshot {
	snap := LaneWorkSnapshot{
		ID:        item.work.ID,
		Kind:      item.work.Kind,
		Component: item.work.Component,
		Name:      item.work.Name,
		Trigger:   item.work.Trigger,
		Phase:     item.work.Phase,
		Stage:     item.work.Stage,
		Detail:    item.work.Detail,
		State:     item.state,
		QueuedAt:  item.work.QueuedAt,
		StartedAt: item.startedAt,
	}
	if !item.work.QueuedAt.IsZero() {
		until := now
		if !item.startedAt.IsZero() {
			until = item.startedAt
		}
		snap.WaitMS = until.Sub(item.work.QueuedAt).Milliseconds()
	}
	if !item.startedAt.IsZero() {
		snap.ElapsedMS = now.Sub(item.startedAt).Milliseconds()
	}
	return snap
}
