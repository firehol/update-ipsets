package engine

import (
	"slices"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"

	"go.opentelemetry.io/otel/attribute"
)

const (
	runBatchItemSource  = "source"
	runBatchItemHistory = "history"
	runBatchItemMerge   = "merge"
)

type runBatchItemState struct {
	kind      string
	completed bool
}

type runBatchState struct {
	startedAt time.Time
	order     []string
	items     map[string]runBatchItemState
}

func (e *Engine) setRunPhase(phase RunPhase) {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.currentPhase = phase
	current := e.currentMetrics
	e.mu.Unlock()
	observeEnginePhaseCurrent(phase)
	if current != nil {
		if completed, ok := current.setPhase(phase); ok {
			if completed.Phase != phase {
				e.logRunPhaseSummary(completed)
			}
		}
	}
}

func initialRunPhasePlan() []RunPhase {
	return []RunPhase{RunPhasePreflight, RunPhaseSources}
}

func plannedRunPhases(plan pipelineRunPlan) []RunPhase {
	phases := initialRunPhasePlan()
	if !plan.shouldPublish {
		return phases
	}
	if plan.skipHeavy {
		return append(phases, RunPhaseMetadata, RunPhasePublish)
	}
	if plan.onlyCriticalProviderSet {
		return append(phases,
			RunPhaseCritical,
			RunPhaseMetadata,
			RunPhaseInsights,
			RunPhasePublish,
		)
	}
	return append(phases,
		RunPhaseGeoIP,
		RunPhaseBogons,
		RunPhaseASN,
		RunPhaseCritical,
		RunPhaseEntities,
		RunPhaseMetadata,
		RunPhaseInsights,
		RunPhasePublish,
	)
}

func (e *Engine) setRunPhasePlan(phases []RunPhase, final bool) {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.currentPhasePlan = append(e.currentPhasePlan[:0], phases...)
	e.currentPhasePlanFinal = final
	e.mu.Unlock()
}

func (e *Engine) startRunBatch(names []string) {
	if e == nil {
		return
	}
	items := make(map[string]runBatchItemState, len(names))
	order := make([]string, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, ok := items[name]; ok {
			continue
		}
		kind := runBatchItemSource
		switch {
		case e.IsHistoryDerivative(name):
			kind = runBatchItemHistory
		case e.IsMerge(name):
			kind = runBatchItemMerge
		}
		items[name] = runBatchItemState{kind: kind}
		order = append(order, name)
	}
	e.mu.Lock()
	e.currentBatch = &runBatchState{
		startedAt: e.now().UTC(),
		order:     order,
		items:     items,
	}
	e.mu.Unlock()
}

func (e *Engine) markRunBatchCompleted(name string) {
	if e == nil || name == "" {
		return
	}
	e.mu.Lock()
	if e.currentBatch != nil {
		item, ok := e.currentBatch.items[name]
		if ok {
			item.completed = true
			e.currentBatch.items[name] = item
		}
	}
	e.mu.Unlock()
}

func (e *Engine) snapshotRunBatchLocked() *RunBatchSnapshot {
	if e == nil || e.currentBatch == nil || len(e.currentBatch.order) == 0 {
		return nil
	}
	snap := &RunBatchSnapshot{
		Names:     append([]string(nil), e.currentBatch.order...),
		StartedAt: e.currentBatch.startedAt,
	}
	for _, name := range e.currentBatch.order {
		item, ok := e.currentBatch.items[name]
		if !ok {
			continue
		}
		snap.Total++
		active := false
		if e.activeFeeds != nil {
			_, active = e.activeFeeds[name]
		}
		switch item.kind {
		case runBatchItemHistory:
			snap.HistoryTotal++
		case runBatchItemMerge:
			snap.MergeTotal++
		default:
			snap.SourceTotal++
		}
		if item.completed {
			snap.Completed++
			snap.CompletedNames = append(snap.CompletedNames, name)
			switch item.kind {
			case runBatchItemHistory:
				snap.HistoryCompleted++
			case runBatchItemMerge:
				snap.MergeCompleted++
			default:
				snap.SourceCompleted++
			}
			continue
		}
		if active {
			snap.Active++
			snap.ActiveNames = append(snap.ActiveNames, name)
			continue
		}
		snap.Pending++
		snap.PendingNames = append(snap.PendingNames, name)
	}
	return snap
}

func (e *Engine) snapshotRunPhasePlanLocked() *RunPhasePlanSnapshot {
	if e == nil || len(e.currentPhasePlan) == 0 {
		return nil
	}
	phases := append([]RunPhase(nil), e.currentPhasePlan...)
	current := e.currentPhase
	position := 0
	if current.Valid() {
		idx := slices.Index(phases, current)
		if idx < 0 {
			phases = append(phases, current)
			idx = len(phases) - 1
		}
		position = idx + 1
	}
	return &RunPhasePlanSnapshot{
		Phases:          phases,
		Current:         current,
		CurrentPosition: position,
		Total:           len(phases),
		Final:           e.currentPhasePlanFinal,
	}
}

func observeEnginePhaseCurrent(current RunPhase) {
	ctx := observability.BackgroundContext()
	for _, phase := range allRunPhases() {
		value := int64(0)
		if current == phase {
			value = 1
		}
		observability.Gauge(ctx, "engine.phase.current", value, attribute.String("engine.phase", string(phase)))
	}
}
