package engine

import (
	"context"
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

func (e *Engine) withEngineLaneBackgroundTask(ctx context.Context, kind LaneWorkKind, component LaneWorkComponent, name, trigger, stage, detail string, current, total int, fn func(task *BackgroundTaskHandle) error) error {
	ctx = nonNilContext(ctx)
	metricComponent := backgroundMetricComponent(kind, component)
	e.observeRunCounter("background.tasks.started", 1, 0)
	observeBackgroundTask(metricComponent, "started")
	task := e.beginBackgroundTaskWithLaneWork(kind, component, name, trigger, stage, detail, current, total)
	if task == nil {
		err := fn(nil)
		if err != nil {
			e.observeRunCounter("background.tasks.failed", 1, 0)
			observeBackgroundTask(metricComponent, "failed")
		} else {
			e.observeRunCounter("background.tasks.completed", 1, 0)
			observeBackgroundTask(metricComponent, "completed")
		}
		return err
	}
	defer task.Finish()

	if err := contextErr(ctx); err != nil {
		e.observeRunCounter("background.tasks.failed", 1, 0)
		observeBackgroundTask(metricComponent, "failed")
		return err
	}
	err := fn(task)
	if err != nil {
		e.observeRunCounter("background.tasks.failed", 1, 0)
		observeBackgroundTask(metricComponent, "failed")
	} else {
		e.observeRunCounter("background.tasks.completed", 1, 0)
		observeBackgroundTask(metricComponent, "completed")
	}
	return err
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
	observability.Count(
		observability.BackgroundContext(),
		"background.tasks",
		1,
		attribute.String("background.component", component),
		attribute.String("background.result", result),
	)
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
