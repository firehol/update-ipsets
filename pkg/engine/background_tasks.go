package engine

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"

	"go.opentelemetry.io/otel/attribute"
)

type BackgroundTaskSnapshot struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Trigger   string    `json:"trigger,omitempty"`
	Stage     string    `json:"stage,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Current   int       `json:"current,omitempty"`
	Total     int       `json:"total,omitempty"`
}

type backgroundTaskState struct {
	BackgroundTaskSnapshot
}

type BackgroundTaskHandle struct {
	engine *Engine
	id     string
}

type backgroundLimiter struct {
	mu      sync.Mutex
	cond    *sync.Cond
	limit   int
	running int
}

func newBackgroundLimiter(limit int) *backgroundLimiter {
	if limit < 1 {
		limit = 1
	}
	l := &backgroundLimiter{limit: limit}
	l.cond = sync.NewCond(&l.mu)
	return l
}

func (l *backgroundLimiter) Acquire() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for l.running >= l.limit {
		l.cond.Wait()
	}
	l.running++
}

func (l *backgroundLimiter) Release() {
	if l == nil {
		return
	}
	l.mu.Lock()
	if l.running > 0 {
		l.running--
	}
	l.mu.Unlock()
	l.cond.Signal()
}

func (l *backgroundLimiter) SetLimit(limit int) {
	if l == nil {
		return
	}
	if limit < 1 {
		limit = 1
	}
	l.mu.Lock()
	l.limit = limit
	l.mu.Unlock()
	l.cond.Broadcast()
}

func (l *backgroundLimiter) Snapshot() (int, int) {
	if l == nil {
		return 0, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.limit, l.running
}

func (e *Engine) beginBackgroundTask(name, trigger, stage, detail string, current, total int) *BackgroundTaskHandle {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	e.backgroundTaskSeq++
	id := name + "-" + strconv.FormatUint(e.backgroundTaskSeq, 10)
	now := time.Now().UTC()
	if e.now != nil {
		now = e.now().UTC()
	}
	if e.backgroundTasks == nil {
		e.backgroundTasks = make(map[string]backgroundTaskState)
	}
	e.backgroundTasks[id] = backgroundTaskState{
		BackgroundTaskSnapshot: BackgroundTaskSnapshot{
			ID:        id,
			Name:      name,
			Trigger:   trigger,
			Stage:     stage,
			Detail:    detail,
			StartedAt: now,
			UpdatedAt: now,
			Current:   current,
			Total:     total,
		},
	}
	return &BackgroundTaskHandle{engine: e, id: id}
}

func (h *BackgroundTaskHandle) Update(stage, detail string, current, total int) {
	if h == nil || h.engine == nil || h.id == "" {
		return
	}
	h.engine.mu.Lock()
	defer h.engine.mu.Unlock()

	task, ok := h.engine.backgroundTasks[h.id]
	if !ok {
		return
	}
	task.Stage = stage
	task.Detail = detail
	task.UpdatedAt = time.Now().UTC()
	if h.engine.now != nil {
		task.UpdatedAt = h.engine.now().UTC()
	}
	task.Current = current
	task.Total = total
	h.engine.backgroundTasks[h.id] = task
}

func (h *BackgroundTaskHandle) Finish() {
	if h == nil || h.engine == nil || h.id == "" {
		return
	}
	h.engine.mu.Lock()
	defer h.engine.mu.Unlock()
	delete(h.engine.backgroundTasks, h.id)
}

func (e *Engine) withBackgroundTask(name, trigger, stage, detail string, current, total int, fn func(task *BackgroundTaskHandle) error) error {
	component := backgroundMetricComponent(name)
	e.observeRunCounter("background.tasks.started", 1, 0)
	observeBackgroundTask(component, "started")
	task := e.beginBackgroundTask(name, trigger, "queued", "waiting for background worker", 0, 0)
	if task == nil {
		err := fn(nil)
		if err != nil {
			e.observeRunCounter("background.tasks.failed", 1, 0)
			observeBackgroundTask(component, "failed")
		} else {
			e.observeRunCounter("background.tasks.completed", 1, 0)
			observeBackgroundTask(component, "completed")
		}
		return err
	}
	defer task.Finish()

	if e.backgroundLimiter != nil {
		waitStarted := time.Now()
		e.backgroundLimiter.Acquire()
		wait := time.Since(waitStarted)
		e.observeRunOperation("background.worker_wait", wait)
		observeBackgroundWorkerWait(component, wait)
		e.observeBackgroundWorkerGauges(component)
		defer func() {
			e.backgroundLimiter.Release()
			e.observeBackgroundWorkerGauges(component)
		}()
	}

	task.Update(stage, detail, current, total)
	err := fn(task)
	if err != nil {
		e.observeRunCounter("background.tasks.failed", 1, 0)
		observeBackgroundTask(component, "failed")
	} else {
		e.observeRunCounter("background.tasks.completed", 1, 0)
		observeBackgroundTask(component, "completed")
	}
	return err
}

func backgroundMetricComponent(name string) string {
	if strings.HasPrefix(name, "Entity artifacts ") {
		return "entity_artifacts"
	}
	return "other"
}

func observeBackgroundTask(component, result string) {
	if component == "" {
		component = "other"
	}
	if result == "" {
		result = "unknown"
	}
	observability.Count(
		observability.BackgroundContext(),
		"background.tasks",
		1,
		attribute.String("background.component", component),
		attribute.String("background.result", result),
	)
}

func observeBackgroundWorkerWait(component string, wait time.Duration) {
	if component == "" {
		component = "other"
	}
	observability.Duration(
		observability.BackgroundContext(),
		"background.worker.wait",
		wait,
		attribute.String("background.component", component),
	)
}

func (e *Engine) observeBackgroundWorkerGauges(component string) {
	if e == nil || e.backgroundLimiter == nil {
		return
	}
	if component == "" {
		component = "other"
	}
	limit, running := e.backgroundLimiter.Snapshot()
	attr := attribute.String("background.component", component)
	ctx := observability.BackgroundContext()
	observability.Gauge(ctx, "background.workers.active", int64(running), attr)
	observability.Gauge(ctx, "background.workers.limit", int64(limit), attr)
}

func (e *Engine) withEntityArtifactMutation(task *BackgroundTaskHandle, detail string, fn func() error) error {
	if e == nil {
		return fn()
	}
	if task != nil {
		task.Update("waiting for entity artifact writer", detail, 0, 0)
	}
	waitStarted := time.Now()
	e.entityArtifactsMu.Lock()
	e.observeRunOperation("entity.writer_lock_wait", time.Since(waitStarted))
	holdStarted := time.Now()
	defer e.entityArtifactsMu.Unlock()
	defer func() {
		e.observeRunOperation("entity.writer_lock_hold", time.Since(holdStarted))
	}()
	return fn()
}

func (e *Engine) snapshotBackgroundTasksLocked() []BackgroundTaskSnapshot {
	if e == nil || len(e.backgroundTasks) == 0 {
		return nil
	}
	out := make([]BackgroundTaskSnapshot, 0, len(e.backgroundTasks))
	for _, task := range e.backgroundTasks {
		out = append(out, task.BackgroundTaskSnapshot)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartedAt.Equal(out[j].StartedAt) {
			return out[i].ID < out[j].ID
		}
		if out[i].StartedAt.IsZero() {
			return false
		}
		if out[j].StartedAt.IsZero() {
			return true
		}
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	return out
}

func backgroundEntityTaskDetail(kind string, count int) string {
	switch kind {
	case "full":
		return "building full country and ASN entity artifacts"
	case "integrity":
		return fmt.Sprintf("repairing %d stale country/ASN artifact targets", count)
	case "health":
		return fmt.Sprintf("refreshing country and ASN entity artifacts for %d health-changed feeds", count)
	case "feeds":
		return fmt.Sprintf("refreshing country and ASN entity artifacts for %d changed feeds", count)
	default:
		return "building country and ASN entity artifacts"
	}
}
