package engine

import (
	"context"
	"sync"

	"github.com/firehol/update-ipsets/pkg/cache"
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

func (e *Engine) processRunSources(ctx context.Context, opts RunOptions, reason runreason.Reason, report *Report, workers int) {
	if workers < 1 {
		workers = 1
	}
	explicitHistory := make([]string, 0)
	explicitMerges := make([]string, 0)
	var (
		mu      sync.Mutex
		results = make(map[string]*sourceResult, len(e.cfg.Sources))
		wg      sync.WaitGroup
		sem     = make(chan struct{}, workers)
	)
	enqueue := func(name string, reason runreason.Reason) {
		wg.Go(func() {
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

			src := e.cfg.Sources[name]
			if src == nil {
				mu.Lock()
				results[name] = &sourceResult{name: name, result: processingOK("unknown source", false)}
				mu.Unlock()
				return
			}
			result := e.processSource(ctx, src, opts, reason)
			mu.Lock()
			results[name] = &sourceResult{name: name, result: result}
			mu.Unlock()
		})
	}

	batchNames := e.processingBatchNames(opts.Selected)
	for _, name := range batchNames {
		src := e.cfg.Sources[name]
		if src == nil {
			continue
		}
		switch {
		case e.IsHistoryDerivative(name):
			explicitHistory = append(explicitHistory, name)
		case e.IsMerge(name):
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
	// Capture the critical-infrastructure provider-set identity exactly once
	// per pipeline run. Every downstream consumer in this run (artifact
	// stamping, marker writing) MUST use this captured value, not a fresh
	// recomputation, so the on-disk marker and the per-feed artifacts always
	// agree within the same run.
	currentCriticalProviderSetID := CriticalInfrastructureProviderSetIDForSnapshot(e.cfg)
	e.criticalProviderSetMu.Lock()
	e.criticalProviderSetID = currentCriticalProviderSetID
	e.criticalProviderSetCached = true
	e.criticalProviderSetMu.Unlock()
	markerPath := CriticalInfrastructureProviderSetMarkerPath(e.runtime)
	criticalProviderSetChanged := markerPath != "" && readCriticalInfrastructureProviderSetMarker(e.runtime) != currentCriticalProviderSetID

	plan := pipelineRunPlan{
		hasUpdates:                 len(report.Updated) > 0,
		criticalProviderSetChanged: criticalProviderSetChanged,
		criticalProviderSetID:      currentCriticalProviderSetID,
		providerDefaultsChanged:    e.providerDefaultsChanged(),
	}
	plan.databaseSelected = e.databaseSourceSelected(opts.Selected)

	// Recheck and Reprocess are explicit operator intent: even when no
	// source has changed, they force the heavy comparison path so generated
	// per-feed artifacts can be rebuilt after code/config changes.
	plan.skipHeavy = !plan.hasUpdates &&
		!plan.databaseSelected &&
		!plan.criticalProviderSetChanged &&
		!plan.providerDefaultsChanged &&
		e.runtime.SkipComparisonIfNoUpdates &&
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
		e.onlyCriticalProviderSetChangedRun(plan.criticalProviderSetChanged, report.Updated, plan.databaseSelected, opts)

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
	plan.perFeedNames = e.perFeedPublicationNames(report.Updated, opts)
	if plan.providerDefaultsChanged {
		plan.perFeedNames = e.publicOutputNames()
	}
	plan.insightUpdated = report.Updated
	if plan.criticalProviderSetChanged || plan.providerDefaultsChanged || (opts.Reprocess && len(opts.Selected) == 0) {
		plan.insightUpdated = nil
	}
	return plan
}

func (e *Engine) databaseSourceSelected(selected []string) bool {
	for _, name := range selected {
		src := e.cfg.Sources[name]
		if src != nil && (src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP)) {
			return true
		}
	}
	return false
}

func (e *Engine) runHeavyPhases(ctx context.Context, opts RunOptions, report *Report, plan pipelineRunPlan, webOutDir string, publishBatch *stagedPublishBatch, setCache *latestSetCache) (*entityPublishBatch, error) {
	if plan.skipHeavy {
		return nil, nil
	}
	if plan.onlyCriticalProviderSet {
		return nil, e.runCriticalOnlyPhase(ctx, plan, webOutDir, publishBatch, setCache)
	}
	return e.runFullHeavyPhases(ctx, opts, report, plan, webOutDir, publishBatch, setCache)
}

func (e *Engine) runCriticalOnlyPhase(ctx context.Context, plan pipelineRunPlan, webOutDir string, publishBatch *stagedPublishBatch, setCache *latestSetCache) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	e.setRunPhase(RunPhaseCritical)
	criticalDS, err := e.loadCriticalInfrastructureSources(ctx, plan.criticalProviderSetID)
	if err != nil {
		return err
	}
	defer criticalDS.closeAll()
	if err := e.writeCriticalInfrastructureFiles(ctx, criticalDS, plan.criticalFanOutUpdated, webOutDir, setCache); err != nil {
		return err
	}
	return e.markStaleCriticalInfrastructureArtifactDeletes(publishBatch)
}

func (e *Engine) runFullHeavyPhases(ctx context.Context, opts RunOptions, report *Report, plan pipelineRunPlan, webOutDir string, publishBatch *stagedPublishBatch, setCache *latestSetCache) (*entityPublishBatch, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.setRunPhase(RunPhaseGeoIP)
	geoProviders, err := e.processGeoIPDatabases(ctx, opts)
	if err != nil {
		return nil, err
	}
	if err := e.writeCountryComparisonFiles(ctx, geoProviders, plan.fanOutUpdated, webOutDir, setCache); err != nil {
		return nil, err
	}

	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.setRunPhase(RunPhaseBogons)
	bogonDS, err := e.loadBogonSources(ctx)
	if err != nil {
		return nil, err
	}
	defer bogonDS.closeAll()
	if err := e.writeBogonComparisonFiles(ctx, bogonDS, plan.fanOutUpdated, webOutDir, setCache); err != nil {
		return nil, err
	}
	bogonUnion, err := buildBogonUnion(bogonDS)
	if err != nil {
		return nil, err
	}

	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.setRunPhase(RunPhaseASN)
	asnDBs, err := e.processASNDatabases(ctx, opts)
	if err != nil {
		return nil, err
	}
	defer asnDBs.closeAll(e.logger)
	if err := e.writeASNComparisonFiles(ctx, asnDBs, bogonUnion, plan.fanOutUpdated, webOutDir, setCache); err != nil {
		return nil, err
	}

	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.setRunPhase(RunPhaseCritical)
	criticalDS, err := e.loadCriticalInfrastructureSources(ctx, plan.criticalProviderSetID)
	if err != nil {
		return nil, err
	}
	defer criticalDS.closeAll()
	if err := e.writeCriticalInfrastructureFiles(ctx, criticalDS, plan.criticalFanOutUpdated, webOutDir, setCache); err != nil {
		return nil, err
	}
	if err := e.markStaleCriticalInfrastructureArtifactDeletes(publishBatch); err != nil {
		return nil, err
	}

	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.setRunPhase(RunPhaseEntities)
	entityBatch, err := e.newEntityPublishBatch()
	if err != nil {
		return nil, err
	}
	entityRefreshTargets, err := e.stageFeedEntitySidecarsFromLoadedProviders(ctx, geoProviders, asnDBs, plan.fanOutUpdated, webOutDir, entityBatch.stagedPublishBatch, setCache)
	if err != nil {
		entityBatch.cleanup()
		return nil, err
	}
	report.EntityRefreshTargets = entityRefreshTargets
	return entityBatch, nil
}

func (e *Engine) writeRunMetadataAndInsights(ctx context.Context, opts RunOptions, report *Report, plan pipelineRunPlan, webOutDir string, setCache *latestSetCache) ([]output.GeneratedFile, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	e.setRunPhase(RunPhaseMetadata)
	generated, err := e.writeMetadataFiles(ctx, plan.skipHeavy, plan.comparisonUpdated, plan.perFeedNames, webOutDir, setCache, opts.EnableAll)
	if err != nil {
		return nil, err
	}
	if !plan.skipHeavy && !plan.onlyCriticalProviderSet {
		homeAggregate, err := e.stageHomeAggregates(ctx, webOutDir, webOutDir)
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
		e.writeInsightsForFeeds(plan.insightUpdated, webOutDir)
	}
	mdGenerated, err := e.writeMarkdownFilesForFeeds(plan.perFeedNames, webOutDir)
	if err != nil {
		e.logger.Warn("markdown generation error", "error", err)
	} else {
		generated = append(generated, mdGenerated...)
	}
	return generated, nil
}

func (e *Engine) publishRunArtifacts(opts RunOptions, report *Report, plan pipelineRunPlan, generated []output.GeneratedFile, webBatch *webPublishBatch, entityBatch *entityPublishBatch) error {
	if opts.BeforePublish != nil {
		if err := opts.BeforePublish(report); err != nil {
			return err
		}
	}
	if err := webBatch.applyGeneratedFileTimestamps(generated); err != nil {
		return err
	}
	e.setRunPhase(RunPhasePublish)
	published, err := webBatch.publish()
	if err != nil {
		return err
	}
	if entityBatch != nil {
		e.entityArtifactsMu.Lock()
		_, err := entityBatch.publish()
		e.entityArtifactsMu.Unlock()
		if err != nil {
			return err
		}
	}
	copied, err := e.copyUpdatedIPSetsToWeb(report.Updated)
	if err != nil {
		return err
	}
	generated = append(generated, copied...)
	for _, file := range copied {
		published = append(published, file.Path)
	}
	if err := e.syncGeneratedFiles(generated, published); err != nil {
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
	if plan.providerDefaultsChanged || e.providerDefaultsChanged() {
		if err := e.writeProviderDefaultsMarker(); err != nil {
			return err
		}
	}
	return cache.Save(e.cachePath, e.state)
}
