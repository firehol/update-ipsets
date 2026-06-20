package engine

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/runreason"

	"go.opentelemetry.io/otel/attribute"
)

func (e *Engine) RunOnce(ctx context.Context, opts RunOptions) (*Report, error) {
	var runErr error
	runReason := normalizeRunReason(opts)
	runStarted := time.Now()
	observability.Gauge(ctx, "engine.running", 1)
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
		observability.Count(ctx, "engine.runs", 1, attrs...)
		observability.Duration(ctx, "engine.run", time.Since(runStarted), attrs...)
		observability.Gauge(ctx, "engine.running", 0)
	}()
	report := &Report{
		StartedAt: e.now().UTC(),
		Messages:  map[string]string{},
		Statuses:  map[string]string{},
	}
	if !e.tryMarkRunStart(report.StartedAt, runReason) {
		runErr = fmt.Errorf("run already in progress")
		return report, runErr
	}
	runDiagnostics := newEngineRunDiagnostics(runReason, opts, runStarted)
	stopProgressLogging := e.startRunProgressLogger(ctx, runDiagnostics)
	defer stopProgressLogging()
	defer func() {
		report.EndedAt = e.now().UTC()
		// Persist cache state even on early abort so partial progress is not lost.
		if err := cache.Save(e.cachePath, e.state); err != nil {
			e.logger.Error("failed to persist cache on run exit", "error", err)
		}
		elapsed := report.EndedAt.Sub(report.StartedAt)
		e.logger.Info("run finished",
			"updated", len(report.Updated),
			"skipped", len(report.Skipped),
			"failed", len(report.Failed),
			"elapsed", elapsed.Round(time.Millisecond).String(),
		)
		e.logRunDiagnosticSummary(report, runErr, runDiagnostics)
		e.markRunEnd(report, runErr)
	}()

	if err := e.ensureDirectories(); err != nil {
		runErr = err
		return report, err
	}
	if opts.CleanupOld {
		if err := e.applyRenamesAndDeletes(); err != nil {
			runErr = err
			return report, err
		}
	}
	e.setRunPhase(RunPhasePreflight)

	// Processing consumes a fixed batch of already-prepared feed bodies.
	// Downloader-stage work is complete before RunOnce starts, so the
	// engine never synthesizes derivatives or merges here.
	workers := e.runtime.MaxProcessingWorkers
	if workers < 1 {
		workers = 1
	}
	e.setRunPhase(RunPhaseSources)
	e.processRunSources(ctx, opts, runReason, report, workers)

	if ctx.Err() != nil {
		runErr = ctx.Err()
		return report, runErr
	}

	plan := e.buildPipelineRunPlan(report, opts)
	e.setRunPhasePlan(plannedRunPhases(plan), true)
	if !plan.shouldPublish {
		e.logger.Info("not publishing web files: no feeds updated in this run")
		return report, nil
	}
	webBatch, err := e.newWebPublishBatch()
	if err != nil {
		runErr = err
		return report, err
	}
	defer webBatch.cleanup()
	webOutDir := webBatch.stageDir
	heavySetCache := newLatestSetCache(e)
	defer heavySetCache.CloseAll(e.logger)

	entityBatch, err := e.runHeavyPhases(ctx, opts, report, plan, webOutDir, webBatch.stagedPublishBatch, heavySetCache)
	if err != nil {
		runErr = err
		return report, err
	}
	if entityBatch != nil {
		defer entityBatch.cleanup()
	}

	generated, err := e.writeRunMetadataAndInsights(ctx, opts, report, plan, webOutDir, heavySetCache)
	if err != nil {
		runErr = err
		return report, err
	}
	if err := e.publishRunArtifacts(ctx, opts, report, plan, generated, webBatch, entityBatch); err != nil {
		runErr = err
		return report, err
	}
	return report, nil
}

func (e *Engine) processingBatchNames(selected []string) []string {
	if e == nil || e.cfg == nil {
		return nil
	}
	names := make([]string, 0)
	if len(selected) == 0 {
		names = append(names, config.SortedSourceNames(e.cfg)...)
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
	slices.SortStableFunc(names, e.processingOrderCompare)
	return names
}

func (e *Engine) onlyCriticalProviderSetChangedRun(providerSetChanged bool, updated []string, databaseSelected bool, opts RunOptions) bool {
	if !providerSetChanged || databaseSelected || opts.Recheck {
		return false
	}
	if opts.Reprocess && len(opts.Selected) == 0 {
		return false
	}
	for _, name := range updated {
		src := e.cfg.Sources[name]
		if src == nil || !src.HasUse(config.UseCriticalInfrastructure) {
			return false
		}
	}
	return true
}

func (e *Engine) processingOrderCompare(a, b string) int {
	rankA := e.processingOrderRank(a)
	rankB := e.processingOrderRank(b)
	if rankA != rankB {
		return cmp.Compare(rankA, rankB)
	}
	if rankA == 2 {
		lenA := len(e.cfg.Sources[a].DerivedFrom)
		lenB := len(e.cfg.Sources[b].DerivedFrom)
		if lenA != lenB {
			return cmp.Compare(lenA, lenB)
		}
	}
	return cmp.Compare(a, b)
}

func (e *Engine) processingOrderRank(name string) int {
	switch {
	case e.IsMerge(name):
		return 2
	case e.IsHistoryDerivative(name):
		return 1
	default:
		return 0
	}
}

func (e *Engine) tryMarkRunStart(t time.Time, reason runreason.Reason) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return false
	}
	e.running = true
	e.lastStarted = t
	e.currentReason = reason
	e.currentPhase = RunPhasePreflight
	e.currentBatch = nil
	e.currentPhasePlan = initialRunPhasePlan()
	e.currentPhasePlanFinal = false
	e.activeFeeds = make(map[string]ActiveFeed)
	e.activeOperations = make(map[string]ActiveOperation)
	e.currentMetrics = newRunMetrics(t, RunPhasePreflight)
	return true
}

func (e *Engine) markRunEnd(report *Report, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.running = false
	e.lastEnded = e.now().UTC()
	e.lastReport = report
	e.lastReason = e.currentReason
	e.currentReason = runreason.ReasonUnknown
	e.currentPhase = RunPhaseUnknown
	e.currentBatch = nil
	e.currentPhasePlan = nil
	e.currentPhasePlanFinal = false
	e.activeFeeds = nil
	e.activeOperations = nil
	observeEnginePhaseCurrent(RunPhaseUnknown)
	if e.currentMetrics != nil {
		e.currentMetrics.finish()
		snap := e.currentMetrics.snapshot(false)
		e.lastMetrics = &snap
		e.currentMetrics = nil
	}
	if err != nil {
		e.lastError = err.Error()
	} else {
		e.lastError = ""
	}
}
