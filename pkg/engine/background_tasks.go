package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"

	"go.opentelemetry.io/otel/attribute"
)

type BackgroundTaskSnapshot struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Kind      LaneWorkKind      `json:"kind,omitempty"`
	Component LaneWorkComponent `json:"component,omitempty"`
	Trigger   string            `json:"trigger,omitempty"`
	Stage     string            `json:"stage,omitempty"`
	Detail    string            `json:"detail,omitempty"`
	StartedAt time.Time         `json:"started_at,omitempty"`
	UpdatedAt time.Time         `json:"updated_at,omitempty"`
	Current   int               `json:"current,omitempty"`
	Total     int               `json:"total,omitempty"`
}

const (
	backgroundTaskEntityArtifactsRebuild = "Entity artifacts rebuild"
	backgroundTaskEntityArtifactsRefresh = "Entity artifacts refresh"
	backgroundTaskEntityArtifactsRepair  = "Entity artifacts repair"
)

type backgroundTaskState struct {
	BackgroundTaskSnapshot
}

type BackgroundTaskHandle struct {
	engine *Engine
	id     string
}

func (e *Engine) beginBackgroundTask(name, trigger, stage, detail string, current, total int) *BackgroundTaskHandle {
	return e.beginBackgroundTaskWithLaneWork("", "", name, trigger, stage, detail, current, total)
}

func (e *Engine) beginBackgroundTaskWithLaneWork(kind LaneWorkKind, component LaneWorkComponent, name, trigger, stage, detail string, current, total int) *BackgroundTaskHandle {
	if e == nil {
		return nil
	}
	e.backgroundTasksMu.Lock()
	defer e.backgroundTasksMu.Unlock()

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
			Kind:      kind,
			Component: component,
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
	h.engine.backgroundTasksMu.Lock()
	defer h.engine.backgroundTasksMu.Unlock()

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
	h.engine.backgroundTasksMu.Lock()
	defer h.engine.backgroundTasksMu.Unlock()
	delete(h.engine.backgroundTasks, h.id)
}

func (e *Engine) withEngineLaneBackgroundTask(ctx context.Context, kind LaneWorkKind, component LaneWorkComponent, name, trigger, stage, detail string, current, total int, fn func(task *BackgroundTaskHandle) error) (err error) {
	ctx = nonNilContext(ctx)
	metricComponent := backgroundMetricComponent(kind, component)
	e.observeRunCounter("background.tasks.started", 1, 0)
	observeBackgroundTask(metricComponent, "started")
	task := e.beginBackgroundTaskWithLaneWork(kind, component, name, trigger, stage, detail, current, total)
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%w: background task panicked: %v", ErrLanePanic, recovered)
		}
		if task != nil {
			task.Finish()
		}
		if err != nil {
			e.observeRunCounter("background.tasks.failed", 1, 0)
			observeBackgroundTask(metricComponent, "failed")
		} else {
			e.observeRunCounter("background.tasks.completed", 1, 0)
			observeBackgroundTask(metricComponent, "completed")
		}
	}()
	if fn == nil {
		return errors.New("background task requires callback")
	}

	if err := contextErr(ctx); err != nil {
		return err
	}
	return fn(task)
}

func backgroundMetricComponent(kind LaneWorkKind, component LaneWorkComponent) string {
	switch component {
	case LaneComponentEngineRun:
		return string(LaneComponentEngineRun)
	case LaneComponentEntityArtifacts:
		return string(LaneComponentEntityArtifacts)
	case LaneComponentEntityArtifactsHealth:
		return string(LaneComponentEntityArtifactsHealth)
	case LaneComponentEntityIntegrity, LaneComponentPipelineIntegrity:
		return "integrity"
	case LaneComponentCriticalInfrastructure, LaneComponentPublishStages:
		return "cleanup"
	}
	switch kind {
	case LaneWorkEntityRebuild, LaneWorkEntityRefresh, LaneWorkEntityRepair:
		return string(LaneComponentEntityArtifacts)
	case LaneWorkIntegrityRefresh, LaneWorkIntegrityReprocess:
		return "integrity"
	case LaneWorkCleanup:
		return "cleanup"
	default:
		return "other"
	}
}

func observeBackgroundTask(component, result string) {
	if component == "" {
		component = "other"
	}
	if result == "" {
		result = "unknown"
	}
	observability.TryCount(
		"background.tasks",
		1,
		attribute.String("background.component", component),
		attribute.String("background.result", result),
	)
}

func backgroundTasksFromMap(in map[string]backgroundTaskState) []BackgroundTaskSnapshot {
	if len(in) == 0 {
		return nil
	}
	out := make([]BackgroundTaskSnapshot, 0, len(in))
	for _, task := range in {
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

func (e *Engine) snapshotBackgroundTasks() []BackgroundTaskSnapshot {
	if e == nil {
		return nil
	}
	e.backgroundTasksMu.RLock()
	defer e.backgroundTasksMu.RUnlock()
	return backgroundTasksFromMap(e.backgroundTasks)
}

func (e *Engine) trySnapshotBackgroundTasks() ([]BackgroundTaskSnapshot, bool) {
	if e == nil {
		return nil, true
	}
	if !e.backgroundTasksMu.TryRLock() {
		return nil, false
	}
	defer e.backgroundTasksMu.RUnlock()
	return backgroundTasksFromMap(e.backgroundTasks), true
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
