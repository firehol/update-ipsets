package engine

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const entityArtifactsVersion = "4"

const entityPublishStagePattern = ".update-ipsets-entities-*"
const entityPublishStagePrefix = ".update-ipsets-entities-"

type entityPublishBatch struct {
	*stagedPublishBatch
	expectedGeneration uint64
}

func (e *Engine) newEntityPublishBatch() (*entityPublishBatch, error) {
	return newEntityPublishBatchForRuntime(e.Runtime())
}

func newEntityPublishBatchForRuntime(rt Runtime) (*entityPublishBatch, error) {
	batch, err := newStagedPublishBatch(entitiesDirForRuntime(rt), "", entityPublishStagePattern)
	if err != nil {
		return nil, err
	}
	return &entityPublishBatch{stagedPublishBatch: batch}, nil
}

func (e *Engine) entitiesDir() string {
	return entitiesDirForRuntime(e.Runtime())
}

func entitiesDirForRuntime(rt Runtime) string {
	return filepath.Join(rt.LibDir, "entities")
}

func (e *Engine) entityFeedsDir() string {
	return entityFeedsDirForRuntime(e.Runtime())
}

func entityFeedsDirForRuntime(rt Runtime) string {
	return filepath.Join(entitiesDirForRuntime(rt), "feeds")
}

func (e *Engine) entityFeedPendingDir() string {
	return entityFeedPendingDirForRuntime(e.Runtime())
}

func entityFeedPendingDirForRuntime(rt Runtime) string {
	return filepath.Join(entitiesDirForRuntime(rt), "feeds-pending")
}

func (e *Engine) entityCountriesDir() string {
	return entityCountriesDirForRuntime(e.Runtime())
}

func entityCountriesDirForRuntime(rt Runtime) string {
	return filepath.Join(entitiesDirForRuntime(rt), "countries")
}

func (e *Engine) entityASNsDir() string {
	return entityASNsDirForRuntime(e.Runtime())
}

func entityASNsDirForRuntime(rt Runtime) string {
	return filepath.Join(entitiesDirForRuntime(rt), "asns")
}

func (e *Engine) entityVersionPath() string {
	return entityVersionPathForRuntime(e.Runtime())
}

func entityVersionPathForRuntime(rt Runtime) string {
	return filepath.Join(entitiesDirForRuntime(rt), "version")
}

func (e *Engine) publicCountryIndexRelPath() string {
	return filepath.Join("countries", "index.json")
}

func (e *Engine) publicCountryDetailRelPath(code string) string {
	return filepath.Join("countries", strings.ToUpper(strings.TrimSpace(code))+".json")
}

func (e *Engine) publicASNIndexRelPath() string {
	return filepath.Join("asns", "index.json")
}

func (e *Engine) publicASNDetailRelPath(asn uint32) string {
	return filepath.Join("asns", strconv.FormatUint(uint64(asn), 10)+".json")
}

func (e *Engine) PublicCountryIndexPath() string {
	return publicCountryIndexPathForRuntime(e.Runtime())
}

func publicCountryIndexPathForRuntime(rt Runtime) string {
	return filepath.Join(outputDirForRuntime(rt), "countries", "index.json")
}

func (e *Engine) PublicCountryDetailPath(code string) string {
	return publicCountryDetailPathForRuntime(e.Runtime(), code)
}

func publicCountryDetailPathForRuntime(rt Runtime, code string) string {
	return filepath.Join(outputDirForRuntime(rt), "countries", strings.ToUpper(strings.TrimSpace(code))+".json")
}

func (e *Engine) PublicASNIndexPath() string {
	return publicASNIndexPathForRuntime(e.Runtime())
}

func publicASNIndexPathForRuntime(rt Runtime) string {
	return filepath.Join(outputDirForRuntime(rt), "asns", "index.json")
}

func (e *Engine) PublicASNDetailPath(asn uint32) string {
	return publicASNDetailPathForRuntime(e.Runtime(), asn)
}

func publicASNDetailPathForRuntime(rt Runtime, asn uint32) string {
	return filepath.Join(outputDirForRuntime(rt), "asns", strconv.FormatUint(uint64(asn), 10)+".json")
}

func (e *Engine) entityFeedSidecarRelPath(name string) string {
	return filepath.Join("feeds", name+".json")
}

func (e *Engine) entityFeedPendingRelPath(name string) string {
	return filepath.Join("feeds-pending", name+".json")
}

func (e *Engine) entityCountrySidecarRelPath(code string) string {
	return filepath.Join("countries", strings.ToUpper(strings.TrimSpace(code))+".json")
}

func (e *Engine) entityASNSidecarRelPath(asn uint32) string {
	return filepath.Join("asns", strconv.FormatUint(uint64(asn), 10)+".json")
}

func (e *Engine) entityArtifactsNeedBootstrapFast() bool {
	return e.entityArtifactsNeedBootstrapFastWithSnapshot(e.operationSnapshot())
}

func (e *Engine) entityArtifactsNeedBootstrapFastWithSnapshot(snap operationSnapshot) bool {
	version, err := readFileInRoot(entitiesDirForRuntime(snap.runtime), "version")
	if err != nil {
		return true
	}
	if strings.TrimSpace(string(version)) != entityArtifactsVersion {
		return true
	}
	required := []string{
		filepath.Join(outputDirForRuntime(snap.runtime), e.publicCountryIndexRelPath()),
		filepath.Join(outputDirForRuntime(snap.runtime), e.publicASNIndexRelPath()),
		publicHomeAggregatesPathForRuntime(snap.runtime),
	}
	for _, path := range required {
		if _, err := os.Stat(path); err != nil {
			return true
		}
	}
	if _, _, err := loadEntityFeedPresenceIndexForRuntime(snap.runtime); err != nil {
		return true
	}
	return false
}

func (e *Engine) RebuildEntityArtifacts() error {
	return e.RebuildEntityArtifactsWithTrigger(context.Background(), "background")
}

func (e *Engine) RebuildEntityArtifactsWithTrigger(ctx context.Context, trigger string) error {
	if e == nil || e.engineLane == nil {
		return nil
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	if trigger == "" {
		trigger = "background"
	}
	return e.engineLane.Run(ctx, LaneWork{
		Kind:      LaneWorkEntityRebuild,
		Component: LaneComponentEntityArtifacts,
		Name:      "entity.rebuild",
		Trigger:   trigger,
		Stage:     "planning",
		Detail:    backgroundEntityTaskDetail("full", 0),
	}, func(laneCtx context.Context) error {
		return e.rebuildEntityArtifactsWithTriggerAdmitted(laneCtx, trigger)
	})
}

func (e *Engine) rebuildEntityArtifactsWithTriggerAdmitted(ctx context.Context, trigger string) error {
	return e.rebuildEntityArtifactsWithTriggerAdmittedWithSnapshot(ctx, e.operationSnapshot(), trigger)
}

func (e *Engine) rebuildEntityArtifactsWithTriggerAdmittedWithSnapshot(ctx context.Context, snap operationSnapshot, trigger string) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	rebuildMarked := e.tryMarkEntityArtifactFullRebuildQueued()
	if rebuildMarked {
		defer e.clearEntityArtifactFullRebuildQueued()
	}
	return e.withEngineLaneBackgroundTask(
		ctx,
		LaneWorkEntityRebuild,
		LaneComponentEntityArtifacts,
		backgroundTaskEntityArtifactsRebuild,
		trigger,
		"planning",
		backgroundEntityTaskDetail("full", 0),
		0,
		0,
		func(task *BackgroundTaskHandle) error {
			return e.rebuildEntityArtifactsFromLiveWithSnapshot(ctx, snap, task)
		},
	)
}

func (e *Engine) RefreshEntityArtifactsForHealthTransitions(ctx context.Context, feedNames []string) error {
	if e == nil || e.engineLane == nil {
		return nil
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	if len(feedNames) == 0 {
		return nil
	}
	return e.engineLane.Run(ctx, LaneWork{
		Kind:      LaneWorkEntityRefresh,
		Component: LaneComponentEntityArtifactsHealth,
		Name:      "entity.health_refresh",
		Trigger:   "health_transition",
		Stage:     "planning",
		Detail:    backgroundEntityTaskDetail("health", len(feedNames)),
	}, func(laneCtx context.Context) error {
		return e.refreshEntityArtifactsForHealthTransitionsAdmitted(laneCtx, feedNames)
	})
}

func (e *Engine) refreshEntityArtifactsForHealthTransitionsAdmitted(ctx context.Context, feedNames []string) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	if len(feedNames) == 0 {
		return nil
	}
	snap := e.operationSnapshot()
	if e.entityArtifactsNeedBootstrapFastWithSnapshot(snap) {
		return e.rebuildEntityArtifactsWithTriggerAdmittedWithSnapshot(ctx, snap, "health_transition")
	}
	return e.withEngineLaneBackgroundTask(
		ctx,
		LaneWorkEntityRefresh,
		LaneComponentEntityArtifactsHealth,
		backgroundTaskEntityArtifactsRefresh,
		"health_transition",
		"scanning memberships",
		backgroundEntityTaskDetail("health", len(feedNames)),
		0,
		len(feedNames),
		func(task *BackgroundTaskHandle) error {
			return e.refreshEntityArtifactsForHealthTransitionsWithSnapshot(ctx, snap, feedNames, task)
		},
	)
}

func (e *Engine) refreshEntityArtifactsForHealthTransitions(ctx context.Context, feedNames []string, task *BackgroundTaskHandle) error {
	return e.refreshEntityArtifactsForHealthTransitionsWithSnapshot(ctx, e.operationSnapshot(), feedNames, task)
}

func (e *Engine) refreshEntityArtifactsForHealthTransitionsWithSnapshot(ctx context.Context, snap operationSnapshot, feedNames []string, task *BackgroundTaskHandle) error {
	return e.runOptimisticEntityArtifactMutation(ctx, task, backgroundEntityTaskDetail("health", len(feedNames)), func() (*entityArtifactMutationPlan, error) {
		return e.stageEntityArtifactsForHealthTransitionsWithSnapshot(ctx, snap, feedNames, task)
	})
}

func (e *Engine) stageEntityArtifactsForHealthTransitions(ctx context.Context, feedNames []string, task *BackgroundTaskHandle) (*entityArtifactMutationPlan, error) {
	return e.stageEntityArtifactsForHealthTransitionsWithSnapshot(ctx, e.operationSnapshot(), feedNames, task)
}

func (e *Engine) stageEntityArtifactsForHealthTransitionsWithSnapshot(ctx context.Context, snap operationSnapshot, feedNames []string, task *BackgroundTaskHandle) (*entityArtifactMutationPlan, error) {
	ctx = nonNilContext(ctx)
	affectedCountries := map[string]struct{}{}
	affectedASNs := map[uint32]struct{}{}
	if err := e.collectHealthTransitionAffectedEntities(ctx, snap, feedNames, task, affectedCountries, affectedASNs); err != nil {
		return nil, err
	}
	if len(affectedCountries) == 0 && len(affectedASNs) == 0 {
		return e.stageHealthTransitionHomeAggregate(ctx, snap)
	}
	return e.stageHealthTransitionEntityPayloads(ctx, snap, affectedCountries, affectedASNs, task)
}

func (e *Engine) rebuildEntityArtifactsFromLive(ctx context.Context, task *BackgroundTaskHandle) error {
	return e.rebuildEntityArtifactsFromLiveWithSnapshot(ctx, e.operationSnapshot(), task)
}

func (e *Engine) rebuildEntityArtifactsFromLiveWithSnapshot(ctx context.Context, snap operationSnapshot, task *BackgroundTaskHandle) error {
	return e.runOptimisticEntityArtifactMutation(ctx, task, backgroundEntityTaskDetail("full", 0), func() (*entityArtifactMutationPlan, error) {
		return e.stageRebuildEntityArtifactsFromLiveWithSnapshot(ctx, snap, task)
	})
}

func (e *Engine) stageRebuildEntityArtifactsFromLive(ctx context.Context, task *BackgroundTaskHandle) (*entityArtifactMutationPlan, error) {
	return e.stageRebuildEntityArtifactsFromLiveWithSnapshot(ctx, e.operationSnapshot(), task)
}

func (e *Engine) stageRebuildEntityArtifactsFromLiveWithSnapshot(ctx context.Context, snap operationSnapshot, task *BackgroundTaskHandle) (*entityArtifactMutationPlan, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	webBatch, err := newWebPublishBatchForRuntime(snap.runtime)
	if err != nil {
		return nil, err
	}
	entityBatch, err := newEntityPublishBatchForRuntime(snap.runtime)
	if err != nil {
		webBatch.cleanup()
		return nil, err
	}
	generated, err := e.writeEntityArtifactsWithSnapshot(ctx, snap, nil, true, webBatch.stagedPublishBatch, entityBatch.stagedPublishBatch, task)
	if err != nil {
		webBatch.cleanup()
		entityBatch.cleanup()
		return nil, err
	}
	if err := contextErr(ctx); err != nil {
		webBatch.cleanup()
		entityBatch.cleanup()
		return nil, err
	}
	return &entityArtifactMutationPlan{
		web:            webBatch,
		entity:         entityBatch,
		generated:      generated,
		publishStage:   "publishing",
		publishDetail:  "publishing rebuilt entity artifacts",
		publishCurrent: 1,
		publishTotal:   1,
	}, nil
}

func (e *Engine) rebuildEntityArtifactsForFeeds(ctx context.Context, feedNames []string, task *BackgroundTaskHandle) error {
	return e.runOptimisticEntityArtifactMutation(ctx, task, backgroundEntityTaskDetail("integrity", len(feedNames)), func() (*entityArtifactMutationPlan, error) {
		return e.stageRebuildEntityArtifactsForFeeds(ctx, feedNames, task)
	})
}

func (e *Engine) stageRebuildEntityArtifactsForFeeds(ctx context.Context, feedNames []string, task *BackgroundTaskHandle) (*entityArtifactMutationPlan, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if len(feedNames) == 0 {
		return nil, nil
	}
	snap := e.operationSnapshot()
	webBatch, err := newWebPublishBatchForRuntime(snap.runtime)
	if err != nil {
		return nil, err
	}
	entityBatch, err := newEntityPublishBatchForRuntime(snap.runtime)
	if err != nil {
		webBatch.cleanup()
		return nil, err
	}

	generated, err := e.writeEntityArtifactsWithSnapshot(ctx, snap, feedNames, false, webBatch.stagedPublishBatch, entityBatch.stagedPublishBatch, task)
	if err != nil {
		webBatch.cleanup()
		entityBatch.cleanup()
		return nil, err
	}
	if err := contextErr(ctx); err != nil {
		webBatch.cleanup()
		entityBatch.cleanup()
		return nil, err
	}
	return &entityArtifactMutationPlan{
		web:            webBatch,
		entity:         entityBatch,
		generated:      generated,
		publishStage:   "publishing",
		publishDetail:  "publishing repaired entity artifacts",
		publishCurrent: 1,
		publishTotal:   1,
	}, nil
}
