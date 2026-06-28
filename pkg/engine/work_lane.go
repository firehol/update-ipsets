package engine

import (
	"context"
	"errors"
	"fmt"
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
	LaneWorkGitSync            LaneWorkKind = "git_sync"
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
	Limit           int                  `json:"limit"`
	ActiveCount     int                  `json:"active_count"`
	WaitingCount    int                  `json:"waiting_count"`
	Active          []LaneWorkSnapshot   `json:"active,omitempty"`
	Waiting         []LaneWorkSnapshot   `json:"waiting,omitempty"`
	LongHoldWarning *LaneLongHoldWarning `json:"long_hold_warning,omitempty"`
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
	idleOpen   bool
	shutdown   bool

	attachStarted         bool
	attachDuplicateCount  uint64
	attachCtx             context.Context
	startNotificationHook func()
	finishPanicHook       func()
	finishAfterLockHook   func()
}

type laneContextKey struct{}

type laneStart struct {
	ctx context.Context
	err error
}

type laneStartNotification struct {
	ch    chan laneStart
	start laneStart
}

type laneWorkerMetric struct {
	ctx         context.Context
	work        LaneWork
	limit       int
	active      int
	wait        time.Duration
	observeWait bool
}

type laneItem struct {
	seq       uint64
	activeKey string
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
	var item *laneItem

	l.mu.Lock()
	if l.shutdown {
		l.mu.Unlock()
		return ErrLaneShuttingDown
	}
	item = l.newItemLocked(ctx, work, fn)
	item.syncStart = make(chan laneStart, 1)
	l.queue = append(l.queue, item)
	starts, notifications, metrics := l.scheduleLocked(time.Now().UTC())
	l.mu.Unlock()
	l.notifyStarts(notifications)
	l.startAsync(starts)
	observeLaneWorkerMetrics(metrics)

	for {
		select {
		case start := <-item.syncStart:
			return l.runStartedSyncItem(item, start, fn)
		case <-ctx.Done():
			l.mu.Lock()
			if item.state == LaneWorkQueued {
				l.removeQueuedLocked(item)
				item.state = LaneWorkCanceled
				l.mu.Unlock()
				return ctx.Err()
			}
			l.mu.Unlock()
			start := <-item.syncStart
			return l.runStartedSyncItem(item, start, fn)
		}
	}
}

func (l *WorkLane) runStartedSyncItem(item *laneItem, start laneStart, fn func(context.Context) error) error {
	if start.err != nil {
		return start.err
	}
	if err := contextErr(start.ctx); err != nil {
		finishErr := l.finishItemSafely(item, nil)
		if item.cancel != nil {
			item.cancel()
		}
		if finishErr != nil {
			return finishErr
		}
		return err
	}
	err := runLaneCallback(start.ctx, fn)
	finishErr := l.finishItemSafely(item, err)
	if item.cancel != nil {
		item.cancel()
	}
	if finishErr != nil {
		return finishErr
	}
	return err
}

func (l *WorkLane) Submit(ctx context.Context, work LaneWork, fn func(context.Context) error) (LaneTicket, error) {
	callerCtx := nonNilContext(ctx)
	if err := contextErr(callerCtx); err != nil {
		return LaneTicket{}, err
	}
	if fn == nil {
		return LaneTicket{}, errors.New("engine work lane submit requires callback")
	}
	if strings.TrimSpace(work.CoalescingKey) == "" {
		return LaneTicket{}, ErrLaneMissingCoalescingKey
	}
	ctx = l.submitParentContext(callerCtx)
	if err := contextErr(ctx); err != nil {
		l.mu.Lock()
		shutdown := l.shutdown
		l.mu.Unlock()
		if shutdown {
			return LaneTicket{}, ErrLaneShuttingDown
		}
		return LaneTicket{}, err
	}
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
	item := l.newItemLocked(ctx, work, fn)
	l.coalescing[work.CoalescingKey] = item
	l.queue = append(l.queue, item)
	starts, notifications, metrics := l.scheduleLocked(time.Now().UTC())
	ticket := item.ticketLocked(item.state == LaneWorkQueued, false)
	l.mu.Unlock()
	l.notifyStarts(notifications)
	l.startAsync(starts)
	observeLaneWorkerMetrics(metrics)
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
	item := l.newItemLocked(ctx, work, fn)
	l.activateLocked(item, now)
	metric := l.workerStartMetricLocked(item, now)
	l.mu.Unlock()
	observeLaneWorkerMetrics([]laneWorkerMetric{metric})
	err := runLaneCallback(item.ctx, fn)
	finishErr := l.finishItemSafely(item, err)
	if item.cancel != nil {
		item.cancel()
	}
	if finishErr != nil {
		return true, finishErr
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
	starts, notifications, metrics := l.scheduleLocked(time.Now().UTC())
	metrics = append(metrics, l.workerGaugeMetricLocked(context.Background()))
	l.mu.Unlock()
	l.notifyStarts(notifications)
	l.startAsync(starts)
	observeLaneWorkerMetrics(metrics)
}

func (l *WorkLane) Snapshot() LaneSnapshot {
	return l.snapshotAt(time.Now().UTC())
}

func (l *WorkLane) Shutdown(grace time.Duration) {
	if l == nil {
		return
	}
	var cancels []context.CancelFunc
	var notifications []laneStartNotification
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
			notifications = append(notifications, laneStartNotification{
				ch:    item.syncStart,
				start: laneStart{err: ErrLaneShuttingDown},
			})
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

	l.notifyStarts(notifications)
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
	l.mu.Lock()
	if l.attachStarted {
		l.attachDuplicateCount++
		l.mu.Unlock()
		observability.TryCount("background.workers.attach_duplicate", 1)
		return
	}
	l.attachStarted = true
	l.attachCtx = ctx
	l.mu.Unlock()

	go func() {
		<-ctx.Done()
		l.Shutdown(grace)
	}()
}

func (l *WorkLane) submitParentContext(ctx context.Context) context.Context {
	ctx = nonNilContext(ctx)
	if l == nil {
		return ctx
	}
	l.mu.Lock()
	attached := l.attachCtx
	l.mu.Unlock()
	if attached != nil {
		return attached
	}
	if ctx.Value(laneContextKey{}) != l {
		return ctx
	}
	return context.Background()
}

func (e *Engine) AttachWorkLaneContext(ctx context.Context, grace time.Duration) {
	if e == nil || e.engineLane == nil {
		return
	}
	e.startEngineRuntimeStatsSampler(ctx)
	e.startEngineLaneDiagnostics(ctx)
	e.engineLane.AttachContext(ctx, grace)
	if e.gitLane != nil {
		e.gitLane.AttachContext(ctx, grace)
	}
}

func (l *WorkLane) newItem(ctx context.Context, work LaneWork, fn func(context.Context) error) *laneItem {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.newItemLocked(ctx, work, fn)
}

func (l *WorkLane) newItemLocked(ctx context.Context, work LaneWork, fn func(context.Context) error) *laneItem {
	l.nextSeq++
	seq := l.nextSeq
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
		ctx:       ctx,
		seq:       seq,
		activeKey: fmt.Sprintf("%d", seq),
		work:      work,
		fn:        fn,
		state:     LaneWorkQueued,
	}
}

func (l *WorkLane) scheduleLocked(now time.Time) ([]*laneItem, []laneStartNotification, []laneWorkerMetric) {
	if l == nil || l.shutdown {
		return nil, nil, nil
	}
	var starts []*laneItem
	var notifications []laneStartNotification
	var metrics []laneWorkerMetric
	for len(l.active) < l.limit && len(l.queue) > 0 {
		item := l.queue[0]
		l.queue = l.queue[1:]
		if item.state != LaneWorkQueued {
			continue
		}
		l.activateLocked(item, now)
		metrics = append(metrics, l.workerStartMetricLocked(item, now))
		if item.syncStart != nil {
			notifications = append(notifications, laneStartNotification{
				ch:    item.syncStart,
				start: laneStart{ctx: item.ctx},
			})
			continue
		}
		starts = append(starts, item)
	}
	return starts, notifications, metrics
}

func (l *WorkLane) activateLocked(item *laneItem, now time.Time) {
	if len(l.active) == 0 {
		l.idle = make(chan struct{})
		l.idleOpen = true
	}
	item.state = LaneWorkActive
	item.startedAt = now
	ctx, cancel := context.WithCancel(item.ctxOrBackground())
	item.ctx = context.WithValue(ctx, laneContextKey{}, l)
	item.cancel = cancel
	l.active[item.activeKey] = item
}

func (l *WorkLane) workerStartMetricLocked(item *laneItem, now time.Time) laneWorkerMetric {
	if l == nil || item == nil {
		return laneWorkerMetric{}
	}
	ctx := item.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	metric := l.workerGaugeMetricLocked(ctx)
	wait := now.Sub(item.work.QueuedAt)
	if wait > 0 {
		metric.work = item.work
		metric.wait = wait
		metric.observeWait = true
	}
	return metric
}

func (l *WorkLane) workerGaugeMetricLocked(ctx context.Context) laneWorkerMetric {
	if l == nil {
		return laneWorkerMetric{}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return laneWorkerMetric{
		ctx:    ctx,
		limit:  l.limit,
		active: len(l.active),
	}
}

func observeLaneWorkerMetrics(metrics []laneWorkerMetric) {
	for _, metric := range metrics {
		observability.TryGauge("background.workers.limit", int64(metric.limit))
		observability.TryGauge("background.workers.active", int64(metric.active))
		if metric.observeWait && metric.wait > 0 {
			observability.TryDuration("background.worker.wait", metric.wait, laneWorkMetricAttrs(metric.work)...)
		}
	}
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
			_ = l.finishItemSafely(item, err)
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

func notifyLaneStarts(notifications []laneStartNotification) {
	for _, notification := range notifications {
		if notification.ch == nil {
			continue
		}
		select {
		case notification.ch <- notification.start:
		default:
		}
	}
}

func (l *WorkLane) notifyStarts(notifications []laneStartNotification) {
	if l != nil && len(notifications) > 0 && l.startNotificationHook != nil {
		l.startNotificationHook()
	}
	notifyLaneStarts(notifications)
}

func (l *WorkLane) finishItemSafely(item *laneItem, err error) (finishErr error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			finishErr = l.recoverFinishPanic(item, recovered)
		}
	}()
	l.finishItem(item, err)
	return nil
}

func (l *WorkLane) recoverFinishPanic(item *laneItem, recovered any) (err error) {
	err = fmt.Errorf("%w: work lane finalization panicked: %v", ErrLanePanic, recovered)
	defer func() {
		if secondary := recover(); secondary != nil {
			err = fmt.Errorf("%w: work lane finalization panic recovery panicked: %v", ErrLanePanic, secondary)
		}
	}()
	var starts []*laneItem
	var notifications []laneStartNotification
	var metrics []laneWorkerMetric
	ctx := context.Background()
	cancel := func() {}
	func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if item != nil {
			if item.ctx != nil {
				ctx = item.ctx
			}
			if item.cancel != nil {
				cancel = item.cancel
			}
			item.state = LaneWorkFailed
			delete(l.active, item.activeKey)
			if key := strings.TrimSpace(item.work.CoalescingKey); key != "" {
				delete(l.coalescing, key)
			}
		}
		if len(l.active) == 0 {
			l.closeIdleLocked()
		}
		starts, notifications, metrics = l.scheduleLocked(time.Now().UTC())
		metrics = append(metrics, l.workerGaugeMetricLocked(ctx))
	}()

	cancel()
	observability.TryCount("background.workers.finalization_panic", 1)
	l.notifyStarts(notifications)
	l.startAsync(starts)
	observeLaneWorkerMetrics(metrics)
	return err
}

func (l *WorkLane) finishItem(item *laneItem, err error) {
	if l == nil || item == nil {
		return
	}
	if l.finishPanicHook != nil {
		l.finishPanicHook()
	}
	var starts []*laneItem
	var notifications []laneStartNotification
	var metrics []laneWorkerMetric
	func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.finishAfterLockHook != nil {
			l.finishAfterLockHook()
		}
		if err != nil {
			item.state = LaneWorkFailed
		} else if contextErr(item.ctx) != nil {
			item.state = LaneWorkCanceled
		} else {
			item.state = LaneWorkCompleted
		}
		delete(l.active, item.activeKey)
		if key := strings.TrimSpace(item.work.CoalescingKey); key != "" {
			delete(l.coalescing, key)
		}
		if len(l.active) == 0 {
			l.closeIdleLocked()
		}
		starts, notifications, metrics = l.scheduleLocked(time.Now().UTC())
		metrics = append(metrics, l.workerGaugeMetricLocked(item.ctx))
	}()
	l.notifyStarts(notifications)
	l.startAsync(starts)
	observeLaneWorkerMetrics(metrics)
}

func (l *WorkLane) closeIdleLocked() {
	if l == nil || !l.idleOpen {
		return
	}
	close(l.idle)
	l.idleOpen = false
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
