package engine

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/output"
	"github.com/firehol/update-ipsets/pkg/runreason"

	"go.opentelemetry.io/otel/attribute"
)

func (e *Engine) RunOnce(ctx context.Context, opts RunOptions) (*Report, error) {
	if e == nil || e.engineLane == nil {
		return nil, errors.New("engine work lane is not initialized")
	}
	ctx = nonNilContext(ctx)
	var report *Report
	var finalization *runFinalization
	runReason := normalizeRunReason(opts)
	err := e.engineLane.Run(ctx, LaneWork{
		Kind:      LaneWorkEngineRun,
		Component: LaneComponentEngineRun,
		Name:      "engine.run",
		Trigger:   runReason.String(),
	}, func(laneCtx context.Context) error {
		var runErr error
		report, finalization, runErr = e.runOnceAdmitted(laneCtx, opts)
		if finalization != nil {
			finalization.ctx = ctx
		}
		return runErr
	})
	var finalizationErr error
	if finalization != nil {
		finalizationErr = e.completeRunFinalization(finalization)
		err = errors.Join(err, finalizationErr)
	}
	if report == nil {
		report = &Report{
			StartedAt: e.now().UTC(),
			EndedAt:   e.now().UTC(),
			Messages:  map[string]string{},
			Statuses:  map[string]string{},
		}
	}
	return report, err
}

type runFinalization struct {
	ctx                   context.Context
	snapshot              operationSnapshot
	report                *Report
	runErr                error
	diagnostics           engineRunDiagnostics
	cacheSnapshot         *cache.State
	asyncCachePersistence bool
	opts                  RunOptions
	plan                  pipelineRunPlan
	generated             []output.GeneratedFile
	webBatch              *webPublishBatch
	entityBatch           *entityPublishBatch
	metrics               RunMetricsSnapshot
	activeFeeds           []ActiveFeed
	activeOperations      []ActiveOperation
	phase                 RunPhase
}

var (
	runFinalizationBeforeCacheSubmitHookMu sync.Mutex
	runFinalizationBeforeCacheSubmitHook   func()
	runOnceAfterStartHook                  func()
	runOnceBeforeMarkFinalizingHook        func()
)

func setRunFinalizationBeforeCacheSubmitHookForTest(fn func()) func() {
	runFinalizationBeforeCacheSubmitHookMu.Lock()
	old := runFinalizationBeforeCacheSubmitHook
	runFinalizationBeforeCacheSubmitHook = fn
	runFinalizationBeforeCacheSubmitHookMu.Unlock()
	return func() {
		runFinalizationBeforeCacheSubmitHookMu.Lock()
		runFinalizationBeforeCacheSubmitHook = old
		runFinalizationBeforeCacheSubmitHookMu.Unlock()
	}
}

func runFinalizationBeforeCacheSubmitHookSnapshot() func() {
	runFinalizationBeforeCacheSubmitHookMu.Lock()
	defer runFinalizationBeforeCacheSubmitHookMu.Unlock()
	return runFinalizationBeforeCacheSubmitHook
}

func setRunOnceAfterStartHookForTest(fn func()) func() {
	runFinalizationBeforeCacheSubmitHookMu.Lock()
	old := runOnceAfterStartHook
	runOnceAfterStartHook = fn
	runFinalizationBeforeCacheSubmitHookMu.Unlock()
	return func() {
		runFinalizationBeforeCacheSubmitHookMu.Lock()
		runOnceAfterStartHook = old
		runFinalizationBeforeCacheSubmitHookMu.Unlock()
	}
}

func runOnceAfterStartHookForTest() func() {
	runFinalizationBeforeCacheSubmitHookMu.Lock()
	defer runFinalizationBeforeCacheSubmitHookMu.Unlock()
	return runOnceAfterStartHook
}

func setRunOnceBeforeMarkFinalizingHookForTest(fn func()) func() {
	runFinalizationBeforeCacheSubmitHookMu.Lock()
	old := runOnceBeforeMarkFinalizingHook
	runOnceBeforeMarkFinalizingHook = fn
	runFinalizationBeforeCacheSubmitHookMu.Unlock()
	return func() {
		runFinalizationBeforeCacheSubmitHookMu.Lock()
		runOnceBeforeMarkFinalizingHook = old
		runFinalizationBeforeCacheSubmitHookMu.Unlock()
	}
}

func runOnceBeforeMarkFinalizingHookForTest() func() {
	runFinalizationBeforeCacheSubmitHookMu.Lock()
	defer runFinalizationBeforeCacheSubmitHookMu.Unlock()
	return runOnceBeforeMarkFinalizingHook
}

func (e *Engine) runOnceAdmitted(ctx context.Context, opts RunOptions) (report *Report, finalization *runFinalization, runErr error) {
	runReason := normalizeRunReason(opts)
	runStarted := time.Now()
	runHasStarted := false
	var runDiagnostics engineRunDiagnostics
	ctx, span := observability.Start(ctx, "engine.run",
		attribute.String("run.reason", runReason.String()),
		attribute.Bool("run.recheck", opts.Recheck),
		attribute.Bool("run.reprocess", opts.Reprocess),
		attribute.Bool("run.manual", opts.Manual),
		attribute.Int("run.selected", len(opts.Selected)),
	)
	defer func() {
		observability.End(span, runErr)
		status := "ok"
		if runErr != nil {
			status = "error"
		}
		attrs := []attribute.KeyValue{
			attribute.String("run.reason", runReason.String()),
			attribute.String("run.status", status),
		}
		observability.TryCount("engine.runs", 1, attrs...)
		observability.TryDuration("engine.run", time.Since(runStarted), attrs...)
	}()
	defer func() {
		if recovered := recover(); recovered != nil {
			runErr = fmt.Errorf("%w: engine run panicked: %v", ErrLanePanic, recovered)
		}
		if !runHasStarted {
			return
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				runErr = errors.Join(runErr, fmt.Errorf("%w: engine run finalizing state panicked: %v", ErrLanePanic, recovered))
				if report == nil {
					report = &Report{
						StartedAt: runStarted.UTC(),
						Messages:  map[string]string{},
						Statuses:  map[string]string{},
					}
				}
				if finalization == nil {
					finalization = &runFinalization{
						diagnostics:           runDiagnostics,
						asyncCachePersistence: opts.AsyncCachePersistence,
						opts:                  opts,
					}
				}
				report.EndedAt = e.now().UTC()
				finalization.report = report
				finalization.runErr = runErr
			}
		}()
		if report == nil {
			report = &Report{
				StartedAt: runStarted.UTC(),
				Messages:  map[string]string{},
				Statuses:  map[string]string{},
			}
		}
		if finalization == nil {
			finalization = &runFinalization{
				diagnostics:           runDiagnostics,
				asyncCachePersistence: opts.AsyncCachePersistence,
				opts:                  opts,
			}
		}
		report.EndedAt = e.now().UTC()
		finalization.report = report
		finalization.runErr = runErr
		if hook := runOnceBeforeMarkFinalizingHookForTest(); hook != nil {
			hook()
		}
		finalization.metrics, finalization.activeFeeds, finalization.activeOperations, finalization.phase = e.markRunFinalizing(report, runErr)
	}()
	report = &Report{
		StartedAt: e.now().UTC(),
		Messages:  map[string]string{},
		Statuses:  map[string]string{},
	}
	if !e.tryMarkRunStart(report.StartedAt, runReason) {
		if e.logger != nil {
			e.logger.Warn("engine run admitted but start marker was already active", "reason", runReason)
		}
		runErr = fmt.Errorf("run already in progress")
		return report, nil, runErr
	}
	runHasStarted = true
	runSnapshot := e.operationSnapshot()
	observability.TryGauge("engine.running", 1)
	runDiagnostics = e.newEngineRunDiagnostics(runReason, opts, runStarted)
	stopProgressLogging := e.startRunProgressLogger(ctx, runDiagnostics)
	defer stopProgressLogging()
	finalization = &runFinalization{
		snapshot:              runSnapshot,
		diagnostics:           runDiagnostics,
		asyncCachePersistence: opts.AsyncCachePersistence,
		opts:                  opts,
	}
	if hook := runOnceAfterStartHookForTest(); hook != nil {
		hook()
	}

	if err := ensureDirectoriesForRuntime(runSnapshot.runtime); err != nil {
		runErr = err
		return report, finalization, err
	}
	if opts.CleanupOld {
		if err := e.applyRenamesAndDeletesWithSnapshot(runSnapshot); err != nil {
			runErr = err
			return report, finalization, err
		}
		e.MarkIntegrityCachesStale()
	}
	e.setRunPhase(RunPhasePreflight)

	// Processing consumes a fixed batch of already-prepared feed bodies.
	// Downloader-stage work is complete before RunOnce starts, so the
	// engine never synthesizes derivatives or merges here.
	workers := runSnapshot.runtime.MaxProcessingWorkers
	if workers < 1 {
		workers = 1
	}
	e.setRunPhase(RunPhaseSources)
	e.processRunSources(ctx, runSnapshot, opts, runReason, report, workers)

	if ctx.Err() != nil {
		runErr = ctx.Err()
		return report, finalization, runErr
	}

	plan := e.buildPipelineRunPlanWithSnapshot(runSnapshot, report, opts)
	e.setRunPhasePlan(plannedRunPhases(plan), true)
	if !plan.shouldPublish {
		e.logger.Info("not publishing web files: no feeds updated in this run")
		return report, finalization, nil
	}
	webBatch, err := newWebPublishBatchForRuntime(runSnapshot.runtime)
	if err != nil {
		runErr = err
		return report, finalization, err
	}
	finalization.webBatch = webBatch
	webOutDir := webBatch.stageDir
	heavySetCache := newLatestSetCacheForSnapshot(e, runSnapshot)
	defer heavySetCache.CloseAll(e.logger)

	entityBatch, err := e.runHeavyPhases(ctx, runSnapshot, opts, report, plan, webOutDir, webBatch.stagedPublishBatch, heavySetCache)
	if err != nil {
		runErr = err
		return report, finalization, err
	}
	if entityBatch != nil {
		finalization.entityBatch = entityBatch
	}

	generated, err := e.writeRunMetadataAndInsights(ctx, runSnapshot, opts, report, plan, webOutDir, heavySetCache)
	if err != nil {
		runErr = err
		return report, finalization, err
	}
	finalization.plan = plan
	finalization.generated = generated
	return report, finalization, nil
}

func (e *Engine) processingBatchNames(selected []string) []string {
	return e.processingBatchNamesForSnapshot(e.operationSnapshot(), selected)
}

func (e *Engine) processingBatchNamesForSnapshot(snap operationSnapshot, selected []string) []string {
	if snap.cfg == nil {
		return nil
	}
	names := make([]string, 0)
	if len(selected) == 0 {
		names = append(names, config.SortedSourceNames(snap.cfg)...)
	} else {
		seen := make(map[string]struct{}, len(selected))
		for _, name := range selected {
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	slices.SortStableFunc(names, func(a, b string) int {
		return processingOrderCompareForConfig(snap.cfg, a, b)
	})
	return names
}

func (e *Engine) onlyCriticalProviderSetChangedRun(providerSetChanged bool, updated []string, databaseSelected bool, opts RunOptions) bool {
	return onlyCriticalProviderSetChangedRunForConfig(e.Config(), providerSetChanged, updated, databaseSelected, opts)
}

func onlyCriticalProviderSetChangedRunForConfig(cfg *config.Config, providerSetChanged bool, updated []string, databaseSelected bool, opts RunOptions) bool {
	if !providerSetChanged || databaseSelected || opts.Recheck {
		return false
	}
	if opts.Reprocess && len(opts.Selected) == 0 {
		return false
	}
	for _, name := range updated {
		if cfg == nil {
			return false
		}
		src := cfg.Sources[name]
		if src == nil || !src.HasUse(config.UseCriticalInfrastructure) {
			return false
		}
	}
	return true
}

func (e *Engine) processingOrderCompare(a, b string) int {
	return processingOrderCompareForConfig(e.Config(), a, b)
}

func processingOrderCompareForConfig(cfg *config.Config, a, b string) int {
	rankA := processingOrderRankForConfig(cfg, a)
	rankB := processingOrderRankForConfig(cfg, b)
	if rankA != rankB {
		return cmp.Compare(rankA, rankB)
	}
	if rankA == 2 {
		lenA := 0
		lenB := 0
		if cfg != nil {
			if src := cfg.Sources[a]; src != nil {
				lenA = len(src.DerivedFrom)
			}
			if src := cfg.Sources[b]; src != nil {
				lenB = len(src.DerivedFrom)
			}
		}
		if lenA != lenB {
			return cmp.Compare(lenA, lenB)
		}
	}
	return cmp.Compare(a, b)
}

func (e *Engine) processingOrderRank(name string) int {
	return processingOrderRankForConfig(e.Config(), name)
}

func processingOrderRankForConfig(cfg *config.Config, name string) int {
	if cfg == nil {
		return 0
	}
	src := cfg.Sources[name]
	switch {
	case src != nil && src.Provenance == config.ProvenanceSecondaryMerge:
		return 2
	case src != nil && src.Provenance == config.ProvenanceSecondaryRetention:
		return 1
	default:
		return 0
	}
}

func (e *Engine) tryMarkRunStart(t time.Time, reason runreason.Reason) bool {
	metrics := newRunMetrics(t, RunPhasePreflight)
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return false
	}
	e.running = true
	e.runState = RunStateRunning
	e.lastStarted = t
	e.currentReason = reason
	e.currentPhase = RunPhasePreflight
	e.currentBatch = nil
	e.currentPhasePlan = initialRunPhasePlan()
	e.currentPhasePlanFinal = false
	e.currentMetrics = metrics
	e.currentMetricsPtr.Store(metrics)
	e.mu.Unlock()
	e.activeFeedsMu.Lock()
	e.activeFeeds = make(map[string]ActiveFeed)
	e.activeFeedsMu.Unlock()
	e.activeOperationsMu.Lock()
	e.activeOperations = make(map[string]ActiveOperation)
	e.activeOperationsMu.Unlock()
	return true
}

func (e *Engine) completeRunFinalization(finalization *runFinalization) (finalizationErr error) {
	if e == nil || finalization == nil || finalization.report == nil {
		return nil
	}
	report := finalization.report
	defer func() {
		if recovered := recover(); recovered != nil {
			finalizationErr = errors.Join(finalizationErr, fmt.Errorf("%w: run finalization panicked: %v", ErrLanePanic, recovered))
		}
		e.markRunIdleAfterFinalization(report, errors.Join(finalization.runErr, finalizationErr))
		observability.TryGauge("engine.running", 0)
	}()
	finalizationErr = e.completeRunPublication(finalization)
	finalization.cacheSnapshot = e.state.SnapshotState()
	elapsed := report.EndedAt.Sub(report.StartedAt)
	e.logger.Info("run finished",
		"updated", len(report.Updated),
		"skipped", len(report.Skipped),
		"failed", len(report.Failed),
		"elapsed", elapsed.Round(time.Millisecond).String(),
	)
	e.logRunDiagnosticSummarySnapshot(
		report,
		finalization.runErr,
		finalization.diagnostics,
		finalization.metrics,
		finalization.activeFeeds,
		finalization.activeOperations,
		finalization.phase,
	)
	if hook := runFinalizationBeforeCacheSubmitHookSnapshot(); hook != nil {
		hook()
	}
	seq, err := e.submitCachePersistence(finalization.cacheSnapshot)
	if err != nil {
		e.logger.Error("failed to submit cache persistence on run exit", "error", err)
		e.recordCachePersistenceSubmitError(err)
		finalizationErr = errors.Join(finalizationErr, err)
	} else if !finalization.asyncCachePersistence {
		finalizationErr = errors.Join(finalizationErr, e.waitForSynchronousCachePersistence(finalization.ctx, seq))
	}
	return finalizationErr
}

func (e *Engine) completeRunPublication(finalization *runFinalization) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%w: final publication panicked: %v", ErrLanePanic, recovered)
		}
	}()
	if e == nil || finalization == nil {
		return nil
	}
	if finalization.webBatch != nil {
		defer finalization.webBatch.cleanup()
	}
	if finalization.entityBatch != nil {
		defer finalization.entityBatch.cleanup()
	}
	if finalization.runErr != nil || finalization.webBatch == nil {
		return nil
	}
	ctx := nonNilContext(finalization.ctx)
	if err := e.publishRunArtifactsWithSnapshot(
		ctx,
		finalization.snapshot,
		finalization.opts,
		finalization.report,
		finalization.plan,
		finalization.generated,
		finalization.webBatch,
		finalization.entityBatch,
	); err != nil {
		return err
	}
	e.MarkIntegrityCachesStale()
	if listenerErr := e.dispatchReloadPublication(ReloadPublication{Runtime: finalization.snapshot.runtime}); listenerErr != nil && e.logger != nil {
		e.logger.Error("public serving publication listener failed", "error", listenerErr)
	}
	return nil
}

func (e *Engine) waitForSynchronousCachePersistence(ctx context.Context, seq uint64) error {
	if e == nil || seq == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(nonNilContext(ctx), 30*time.Second)
	defer cancel()
	e.cachePersistenceMu.Lock()
	worker := e.cachePersistence
	e.cachePersistenceMu.Unlock()
	if worker == nil {
		return nil
	}
	var waitErr error
	if err := worker.Wait(ctx, seq); err != nil {
		e.logger.Error("cache persistence did not finish before direct run returned", "error", err)
		waitErr = err
	}
	var stopErr error
	if err := worker.Stop(ctx); err != nil {
		e.logger.Warn("cache persistence worker did not stop after direct run", "error", err)
		stopErr = err
	}
	return errors.Join(waitErr, stopErr)
}

func (e *Engine) markRunFinalizing(report *Report, err error) (RunMetricsSnapshot, []ActiveFeed, []ActiveOperation, RunPhase) {
	current := e.currentRunMetrics()
	var metrics RunMetricsSnapshot
	var lastMetrics *RunMetricsSnapshot
	if current != nil {
		current.finish()
		snap := current.snapshot(false)
		metrics = snap
		lastMetrics = &snap
	}
	activeFeeds := e.snapshotActiveFeeds()
	activeOperations := e.snapshotActiveOperations(time.Now().UTC())
	var phase RunPhase
	e.currentMetricsPtr.Store(nil)
	e.activeFeedsMu.Lock()
	e.activeFeeds = nil
	e.activeFeedsMu.Unlock()
	e.activeOperationsMu.Lock()
	e.activeOperations = nil
	e.activeOperationsMu.Unlock()

	func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		phase = e.currentPhase
		e.runState = RunStateFinalizing
		e.lastEnded = e.now().UTC()
		e.lastReport = report
		e.lastReason = e.currentReason
		e.currentReason = runreason.ReasonUnknown
		e.currentPhase = RunPhaseUnknown
		e.currentBatch = nil
		e.currentPhasePlan = nil
		e.currentPhasePlanFinal = false
		e.currentMetrics = nil
		if lastMetrics != nil {
			e.lastMetrics = lastMetrics
		}
		if err != nil {
			e.lastError = err.Error()
		} else {
			e.lastError = ""
		}
	}()
	observeEnginePhaseCurrent(RunPhaseUnknown)
	return metrics, activeFeeds, activeOperations, phase
}

func (e *Engine) recordCachePersistenceSubmitError(err error) {
	if e == nil || err == nil {
		return
	}
	e.cachePersistenceMu.Lock()
	worker := e.cachePersistence
	e.cachePersistenceMu.Unlock()
	if worker == nil {
		return
	}
	worker.mu.Lock()
	worker.lastError = err.Error()
	worker.failedSaves++
	worker.failedSeq = worker.acceptedSeq
	worker.cond.Broadcast()
	worker.mu.Unlock()
}

func (e *Engine) markRunIdleAfterFinalization(report *Report, err error) {
	if e == nil {
		return
	}
	cleared := false
	e.mu.Lock()
	sameFinalizingRun := e.runState == RunStateFinalizing && e.lastReport == report
	sameRunningRun := report != nil && e.runState == RunStateRunning && e.lastStarted.Equal(report.StartedAt)
	if sameFinalizingRun || sameRunningRun {
		e.running = false
		e.runState = RunStateIdle
		e.currentPhase = RunPhaseUnknown
		e.currentBatch = nil
		e.currentPhasePlan = nil
		e.currentPhasePlanFinal = false
		e.currentMetrics = nil
		e.lastReport = report
		e.lastEnded = e.now().UTC()
		if err != nil {
			e.lastError = err.Error()
		} else {
			e.lastError = ""
		}
		cleared = true
	}
	e.mu.Unlock()
	if cleared {
		observeEnginePhaseCurrent(RunPhaseUnknown)
		e.currentMetricsPtr.Store(nil)
		e.activeFeedsMu.Lock()
		e.activeFeeds = nil
		e.activeFeedsMu.Unlock()
		e.activeOperationsMu.Lock()
		e.activeOperations = nil
		e.activeOperationsMu.Unlock()
	}
}
