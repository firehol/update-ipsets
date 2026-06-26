package engine

import (
	"context"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/output"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

type sourceResult struct {
	name   string
	result FeedProcessingResult
}

type pipelineRunPlan struct {
	hasUpdates                 bool
	databaseSelected           bool
	criticalProviderSetChanged bool
	// criticalProviderSetID is the catalog identity captured exactly once at
	// plan time. It is the single source of truth for the provider_set_id
	// stamped into every critical-overlap artifact during this run and for
	// the runtime marker written at the end of the run. Capturing it once
	// and threading it explicitly closes the TOCTOU window between artifact
	// stamping and marker writing.
	criticalProviderSetID   string
	providerDefaultsChanged bool
	shouldPublish           bool
	skipHeavy               bool
	onlyCriticalProviderSet bool
	fanOutUpdated           []string
	criticalFanOutUpdated   []string
	comparisonUpdated       []string
	perFeedNames            []string
	insightUpdated          []string
}

func (e *Engine) processRunSources(ctx context.Context, snap operationSnapshot, opts RunOptions, reason runreason.Reason, report *Report, workers int) {
	if workers < 1 {
		workers = 1
	}
	cfg := snap.cfg
	if cfg == nil {
		return
	}
	explicitHistory := make([]string, 0)
	explicitMerges := make([]string, 0)
	var (
		mu      sync.Mutex
		results = make(map[string]*sourceResult, len(cfg.Sources))
		wg      sync.WaitGroup
		sem     = make(chan struct{}, workers)
	)
	enqueue := func(name string, reason runreason.Reason) {
		wg.Go(func() {
			defer e.markRunBatchCompleted(name)
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				mu.Lock()
				results[name] = &sourceResult{
					name: name,
					result: processingException(
						ProcessingExceptionCancelled,
						"processing cancelled before worker slot was available",
						ctx.Err(),
					),
				}
				mu.Unlock()
				return
			}
			defer func() { <-sem }()

			src := cfg.Sources[name]
			if src == nil {
				mu.Lock()
				results[name] = &sourceResult{name: name, result: processingOK("unknown source", false)}
				mu.Unlock()
				return
			}
			started := time.Now()
			result := e.processSourceWithSnapshot(ctx, snap, src, opts, reason)
			elapsed := time.Since(started)
			e.observeFeedWork(name, result, elapsed)
			e.logFeedProcessingSummary(name, elapsed, result)
			mu.Lock()
			results[name] = &sourceResult{name: name, result: result}
			mu.Unlock()
		})
	}

	batchNames := e.processingBatchNamesForSnapshot(snap, opts.Selected)
	e.startRunBatch(batchNames)
	e.observeRunCounter("sources.feeds_expected", int64(len(batchNames)), 0)
	for _, name := range batchNames {
		src := cfg.Sources[name]
		if src == nil {
			continue
		}
		switch {
		case snap.isHistoryDerivative(name):
			explicitHistory = append(explicitHistory, name)
		case snap.isMerge(name):
			explicitMerges = append(explicitMerges, name)
		default:
			enqueue(name, reason)
		}
	}
	wg.Wait()
	for _, name := range explicitHistory {
		enqueue(name, reason)
	}
	wg.Wait()
	for _, name := range explicitMerges {
		enqueue(name, reason)
	}
	wg.Wait()

	for _, name := range batchNames {
		r, ok := results[name]
		if !ok {
			continue
		}
		report.Statuses[r.name] = r.result.StatusString()
		report.Messages[r.name] = r.result.Message
		switch {
		case r.result.Err != nil:
			report.Failed = append(report.Failed, r.name)
			e.logger.Error("source failed", "source", r.name, "message", r.result.Message, "exception", r.result.Exception)
		case r.result.Processed:
			report.Updated = append(report.Updated, r.name)
		default:
			report.Skipped = append(report.Skipped, r.name)
		}
	}
}

func (e *Engine) buildPipelineRunPlan(report *Report, opts RunOptions) pipelineRunPlan {
	return e.buildPipelineRunPlanWithSnapshot(e.operationSnapshot(), report, opts)
}

func (e *Engine) buildPipelineRunPlanWithSnapshot(snap operationSnapshot, report *Report, opts RunOptions) pipelineRunPlan {
	cfg := snap.cfg
	rt := snap.runtime
	// Capture the critical-infrastructure provider-set identity exactly once
	// per pipeline run. Every downstream consumer in this run (artifact
	// stamping, marker writing) MUST use this captured value, not a fresh
	// recomputation, so the on-disk marker and the per-feed artifacts always
	// agree within the same run.
	currentCriticalProviderSetID := CriticalInfrastructureProviderSetIDForSnapshot(cfg)
	e.criticalProviderSetMu.Lock()
	e.criticalProviderSetID = currentCriticalProviderSetID
	e.criticalProviderSetCached = true
	e.criticalProviderSetMu.Unlock()
	markerPath := CriticalInfrastructureProviderSetMarkerPath(rt)
	criticalProviderSetChanged := markerPath != "" && readCriticalInfrastructureProviderSetMarker(rt) != currentCriticalProviderSetID

	plan := pipelineRunPlan{
		hasUpdates:                 len(report.Updated) > 0,
		criticalProviderSetChanged: criticalProviderSetChanged,
		criticalProviderSetID:      currentCriticalProviderSetID,
		providerDefaultsChanged:    ProviderDefaultsChangedForConfig(cfg, rt),
	}
	plan.databaseSelected = databaseSourceSelectedForConfig(cfg, opts.Selected)

	// Recheck and Reprocess are explicit operator intent: even when no
	// source has changed, they force the heavy comparison path so generated
	// per-feed artifacts can be rebuilt after code/config changes.
	plan.skipHeavy = !plan.hasUpdates &&
		!plan.databaseSelected &&
		!plan.criticalProviderSetChanged &&
		!plan.providerDefaultsChanged &&
		rt.SkipComparisonIfNoUpdates &&
		!opts.Recheck &&
		!opts.Reprocess
	plan.shouldPublish = plan.hasUpdates ||
		plan.databaseSelected ||
		plan.criticalProviderSetChanged ||
		plan.providerDefaultsChanged ||
		opts.Reprocess
	if !plan.shouldPublish {
		return plan
	}

	plan.onlyCriticalProviderSet = !plan.skipHeavy &&
		!plan.providerDefaultsChanged &&
		onlyCriticalProviderSetChangedRunForConfig(cfg, plan.criticalProviderSetChanged, report.Updated, plan.databaseSelected, opts)

	plan.fanOutUpdated = report.Updated
	if opts.Reprocess && len(opts.Selected) == 0 {
		plan.fanOutUpdated = nil
	}
	plan.criticalFanOutUpdated = plan.fanOutUpdated
	if plan.criticalProviderSetChanged {
		plan.criticalFanOutUpdated = nil
	}
	if plan.providerDefaultsChanged {
		plan.fanOutUpdated = nil
		plan.criticalFanOutUpdated = nil
	}

	plan.comparisonUpdated = report.Updated
	if opts.Reprocess && len(opts.Selected) == 0 {
		plan.comparisonUpdated = nil
	}
	plan.perFeedNames = e.perFeedPublicationNamesForSnapshot(snap, report.Updated, opts)
	if plan.providerDefaultsChanged {
		plan.perFeedNames = e.publicOutputNamesForSnapshot(snap)
	}
	plan.insightUpdated = report.Updated
	if plan.criticalProviderSetChanged || plan.providerDefaultsChanged || (opts.Reprocess && len(opts.Selected) == 0) {
		plan.insightUpdated = nil
	}
	return plan
}

func (e *Engine) databaseSourceSelected(selected []string) bool {
	return databaseSourceSelectedForConfig(e.Config(), selected)
}

func databaseSourceSelectedForConfig(cfg *config.Config, selected []string) bool {
	for _, name := range selected {
		if cfg == nil {
			continue
		}
		src := cfg.Sources[name]
		if src != nil && (src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP)) {
			return true
		}
	}
	return false
}

func (e *Engine) runHeavyPhases(ctx context.Context, snap operationSnapshot, opts RunOptions, report *Report, plan pipelineRunPlan, webOutDir string, publishBatch *stagedPublishBatch, setCache *latestSetCache) (*entityPublishBatch, error) {
	if plan.skipHeavy {
		return nil, nil
	}
	if plan.onlyCriticalProviderSet {
		return nil, e.runCriticalOnlyPhase(ctx, snap, plan, webOutDir, publishBatch, setCache)
	}
	return e.runFullHeavyPhases(ctx, snap, opts, report, plan, webOutDir, publishBatch, setCache)
}

func (e *Engine) runCriticalOnlyPhase(ctx context.Context, snap operationSnapshot, plan pipelineRunPlan, webOutDir string, publishBatch *stagedPublishBatch, setCache *latestSetCache) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	e.setRunPhase(RunPhaseCritical)
	criticalDS, err := e.loadCriticalInfrastructureSourcesWithSnapshot(ctx, snap, plan.criticalProviderSetID)
	if err != nil {
		return err
	}
	defer criticalDS.closeAll()
	if err := e.writeCriticalInfrastructureFilesWithSnapshot(ctx, snap, criticalDS, plan.criticalFanOutUpdated, webOutDir, setCache); err != nil {
		return err
	}
	return markStaleCriticalInfrastructureArtifactDeletesForConfig(publishBatch, snap.cfg, outputDirForRuntime(snap.runtime))
}

func (e *Engine) runFullHeavyPhases(ctx context.Context, snap operationSnapshot, opts RunOptions, report *Report, plan pipelineRunPlan, webOutDir string, publishBatch *stagedPublishBatch, setCache *latestSetCache) (*entityPublishBatch, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.setRunPhase(RunPhaseGeoIP)
	geoProviders, err := e.processGeoIPDatabasesWithSnapshot(ctx, snap, opts)
	if err != nil {
		return nil, err
	}
	if err := e.writeCountryComparisonFilesWithSnapshot(ctx, snap, geoProviders, plan.fanOutUpdated, webOutDir, setCache); err != nil {
		return nil, err
	}

	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.setRunPhase(RunPhaseBogons)
	bogonDS, err := e.loadBogonSourcesWithSnapshot(ctx, snap)
	if err != nil {
		return nil, err
	}
	defer bogonDS.closeAll()
	if err := e.writeBogonComparisonFilesWithSnapshot(ctx, snap, bogonDS, plan.fanOutUpdated, webOutDir, setCache); err != nil {
		return nil, err
	}
	bogonProviderTotal := int64(0)
	if bogonDS != nil {
		bogonProviderTotal = int64(len(bogonDS.Names))
	}
	unionOp := e.beginActiveOperation("bogons.build_union", "", "union", "providers", bogonProviderTotal)
	bogonUnion, err := buildBogonUnion(ctx, bogonDS)
	unionOp.Update(bogonProviderTotal, bogonProviderTotal, nil)
	unionOp.Finish()
	if err != nil {
		return nil, err
	}

	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.setRunPhase(RunPhaseASN)
	asnDBs, err := e.processASNDatabasesWithSnapshot(ctx, snap, opts)
	if err != nil {
		return nil, err
	}
	defer asnDBs.closeAll(e.logger)
	if err := e.writeASNComparisonFilesWithSnapshot(ctx, snap, asnDBs, bogonUnion, plan.fanOutUpdated, webOutDir, setCache); err != nil {
		return nil, err
	}

	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.setRunPhase(RunPhaseCritical)
	criticalDS, err := e.loadCriticalInfrastructureSourcesWithSnapshot(ctx, snap, plan.criticalProviderSetID)
	if err != nil {
		return nil, err
	}
	defer criticalDS.closeAll()
	if err := e.writeCriticalInfrastructureFilesWithSnapshot(ctx, snap, criticalDS, plan.criticalFanOutUpdated, webOutDir, setCache); err != nil {
		return nil, err
	}
	if err := markStaleCriticalInfrastructureArtifactDeletesForConfig(publishBatch, snap.cfg, outputDirForRuntime(snap.runtime)); err != nil {
		return nil, err
	}

	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.setRunPhase(RunPhaseEntities)
	if e.entityArtifactFullRebuildQueuedOrRunning() {
		report.EntityRefreshTargets = targetFeedsForFanOut(snap.cfg, plan.fanOutUpdated, e.publicOutputNamesForSnapshot(snap), config.UseGeoIP, config.UseASN, config.UseBogons)
		e.observeRunCounter("entity.sidecar_stage.deferred_full_rebuild", int64(len(report.EntityRefreshTargets)), 0)
		return nil, nil
	}
	expectedEntityGeneration := e.entityArtifactGenerationSnapshot()
	entityBatch, err := newEntityPublishBatchForRuntime(snap.runtime)
	if err != nil {
		return nil, err
	}
	entityBatch.expectedGeneration = expectedEntityGeneration
	entityRefreshTargets, err := e.stageFeedEntitySidecarsFromLoadedProvidersWithSnapshot(ctx, snap, geoProviders, asnDBs, plan.fanOutUpdated, webOutDir, entityBatch.stagedPublishBatch, setCache)
	if err != nil {
		entityBatch.cleanup()
		return nil, err
	}
	report.EntityRefreshTargets = entityRefreshTargets
	return entityBatch, nil
}

func (e *Engine) writeRunMetadataAndInsights(ctx context.Context, snap operationSnapshot, opts RunOptions, report *Report, plan pipelineRunPlan, webOutDir string, setCache *latestSetCache) ([]output.GeneratedFile, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.setRunPhase(RunPhaseMetadata)
	generated, err := e.writeMetadataFilesWithSnapshot(ctx, snap, plan.skipHeavy, plan.comparisonUpdated, plan.perFeedNames, webOutDir, setCache, opts.EnableAll)
	if err != nil {
		return nil, err
	}
	if !plan.skipHeavy && !plan.onlyCriticalProviderSet {
		homeAggregate, err := e.stageHomeAggregatesWithSnapshot(ctx, snap, webOutDir, webOutDir)
		if err != nil {
			return nil, err
		}
		generated = append(generated, homeAggregate)
	}
	if !plan.skipHeavy {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		e.setRunPhase(RunPhaseInsights)
		if err := e.writeInsightsForFeedsWithSnapshot(ctx, snap, plan.insightUpdated, webOutDir); err != nil {
			return nil, err
		}
	}
	mdGenerated, err := e.writeMarkdownFilesForFeedsWithSnapshot(ctx, snap, plan.perFeedNames, webOutDir)
	if err != nil {
		e.logger.Warn("markdown generation error", "error", err)
	} else {
		generated = append(generated, mdGenerated...)
	}
	return generated, nil
}

func (e *Engine) publishRunArtifacts(ctx context.Context, opts RunOptions, report *Report, plan pipelineRunPlan, generated []output.GeneratedFile, webBatch *webPublishBatch, entityBatch *entityPublishBatch) error {
	return e.publishRunArtifactsWithSnapshot(ctx, e.operationSnapshot(), opts, report, plan, generated, webBatch, entityBatch)
}

func (e *Engine) publishRunArtifactsWithSnapshot(ctx context.Context, snap operationSnapshot, opts RunOptions, report *Report, plan pipelineRunPlan, generated []output.GeneratedFile, webBatch *webPublishBatch, entityBatch *entityPublishBatch) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	if opts.BeforePublish != nil {
		if err := opts.BeforePublish(report); err != nil {
			return err
		}
		if err := contextErr(ctx); err != nil {
			return err
		}
	}
	e.setRunPhase(RunPhasePublish)
	timestampOp := e.beginActiveOperation("publish.apply_timestamps", "", "timestamps", "files", int64(len(generated)))
	if err := webBatch.applyGeneratedFileTimestampsContext(ctx, generated, timestampOp); err != nil {
		timestampOp.Finish()
		return err
	}
	timestampOp.Finish()
	publishTotal, err := webBatch.publishWorkTotal(ctx)
	if err != nil {
		return err
	}
	publishOp := e.beginActiveOperation("publish.promote_web_artifacts", "", "publish", "files", publishTotal)
	published, err := webBatch.publishContext(ctx, publishOp)
	publishOp.Finish()
	if err != nil {
		return err
	}
	if entityBatch != nil {
		if err := contextErr(ctx); err != nil {
			return err
		}
		entityPublishOp := e.beginActiveOperation("publish.promote_entity_artifacts", "", "publish", "files", 0)
		entityPublishTotal, countErr := entityBatch.publishWorkTotal(ctx)
		if countErr == nil {
			entityPublishOp.Update(0, entityPublishTotal, nil)
			err = func() error {
				lease, err := e.acquireEntityArtifactPublishLease(ctx, entityBatch.expectedGeneration)
				if err != nil {
					return err
				}
				defer lease.release(true)
				_, err = entityBatch.publishContext(ctx, entityPublishOp)
				return err
			}()
		} else {
			err = countErr
		}
		entityPublishOp.Finish()
		if err != nil {
			return err
		}
	}
	copied, err := e.copyUpdatedIPSetsToWebContextWithSnapshot(ctx, snap, report.Updated)
	if err != nil {
		return err
	}
	generated = append(generated, copied...)
	for _, file := range copied {
		published = append(published, file.Path)
	}
	if err := e.syncGeneratedFiles(ctx, generated, published); err != nil {
		return err
	}
	// The runtime marker carries the SAME identity that was stamped into
	// every critical-overlap artifact written in this run. Re-reading from
	// engine state here would reopen the TOCTOU window we are explicitly
	// closing — and would let a single mutation between artifact write and
	// marker write make the integrity check report 285+ feeds as malformed.
	if plan.criticalProviderSetChanged {
		if err := e.writeCriticalInfrastructureProviderSetMarkerValue(plan.criticalProviderSetID); err != nil {
			return err
		}
	}
	if plan.providerDefaultsChanged {
		if err := writeProviderDefaultsMarkerForConfigRuntime(snap.cfg, snap.runtime); err != nil {
			return err
		}
	}
	return nil
}
