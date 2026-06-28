package engine

import (
	"slices"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"github.com/firehol/update-ipsets/pkg/config"

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

type runBatchSnapshotState struct {
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
	e.mu.Unlock()
	observeEnginePhaseCurrent(phase)
	if current := e.currentRunMetrics(); current != nil {
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
	snap := e.operationSnapshot()
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
		src := lookupSourceForConfig(snap.cfg, name)
		switch {
		case src != nil && src.Provenance == config.ProvenanceSecondaryRetention:
			kind = runBatchItemHistory
		case src != nil && src.Provenance == config.ProvenanceSecondaryMerge:
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

func (e *Engine) copyRunBatchLocked() *runBatchSnapshotState {
	if e == nil || e.currentBatch == nil || len(e.currentBatch.order) == 0 {
		return nil
	}
	items := make(map[string]runBatchItemState, len(e.currentBatch.items))
	for name, item := range e.currentBatch.items {
		items[name] = item
	}
	return &runBatchSnapshotState{
		startedAt: e.currentBatch.startedAt,
		order:     append([]string(nil), e.currentBatch.order...),
		items:     items,
	}
}

func snapshotRunBatch(batch *runBatchSnapshotState, activeFeeds []ActiveFeed) *RunBatchSnapshot {
	if batch == nil || len(batch.order) == 0 {
		return nil
	}
	activeNames := make(map[string]struct{}, len(activeFeeds))
	for _, feed := range activeFeeds {
		activeNames[feed.Name] = struct{}{}
	}
	snap := &RunBatchSnapshot{
		Names:     append([]string(nil), batch.order...),
		StartedAt: batch.startedAt,
	}
	for _, name := range batch.order {
		item, ok := batch.items[name]
		if !ok {
			continue
		}
		_, active := activeNames[name]
		snap.Total++
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
	for _, phase := range allRunPhases() {
		value := int64(0)
		if current == phase {
			value = 1
		}
		observability.TryGauge("engine.phase.current", value, attribute.String("engine.phase", string(phase)))
	}
}
