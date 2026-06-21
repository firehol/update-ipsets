package engine

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const entityArtifactsVersion = "3"

const entityPublishStagePattern = ".update-ipsets-entities-*"
const entityPublishStagePrefix = ".update-ipsets-entities-"

type entityPublishBatch struct {
	*stagedPublishBatch
}

func (e *Engine) newEntityPublishBatch() (*entityPublishBatch, error) {
	batch, err := newStagedPublishBatch(e.entitiesDir(), "", entityPublishStagePattern)
	if err != nil {
		return nil, err
	}
	return &entityPublishBatch{stagedPublishBatch: batch}, nil
}

func (e *Engine) entitiesDir() string {
	return filepath.Join(e.runtime.LibDir, "entities")
}

func (e *Engine) entityFeedsDir() string {
	return filepath.Join(e.entitiesDir(), "feeds")
}

func (e *Engine) entityFeedPendingDir() string {
	return filepath.Join(e.entitiesDir(), "feeds-pending")
}

func (e *Engine) entityCountriesDir() string {
	return filepath.Join(e.entitiesDir(), "countries")
}

func (e *Engine) entityASNsDir() string {
	return filepath.Join(e.entitiesDir(), "asns")
}

func (e *Engine) entityVersionPath() string {
	return filepath.Join(e.entitiesDir(), "version")
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
	return filepath.Join(e.outputDir(), e.publicCountryIndexRelPath())
}

func (e *Engine) PublicCountryDetailPath(code string) string {
	return filepath.Join(e.outputDir(), e.publicCountryDetailRelPath(code))
}

func (e *Engine) PublicASNIndexPath() string {
	return filepath.Join(e.outputDir(), e.publicASNIndexRelPath())
}

func (e *Engine) PublicASNDetailPath(asn uint32) string {
	return filepath.Join(e.outputDir(), e.publicASNDetailRelPath(asn))
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
	version, err := readFileInRoot(e.entitiesDir(), "version")
	if err != nil {
		return true
	}
	if strings.TrimSpace(string(version)) != entityArtifactsVersion {
		return true
	}
	required := []string{
		filepath.Join(e.outputDir(), e.publicCountryIndexRelPath()),
		filepath.Join(e.outputDir(), e.publicASNIndexRelPath()),
		e.PublicHomeAggregatesPath(),
	}
	for _, path := range required {
		if _, err := os.Stat(path); err != nil {
			return true
		}
	}
	return false
}

func (e *Engine) RebuildEntityArtifacts() error {
	return e.RebuildEntityArtifactsWithTrigger(context.Background(), "background")
}

func (e *Engine) RebuildEntityArtifactsWithTrigger(ctx context.Context, trigger string) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	rebuildMarked := e.tryMarkEntityArtifactFullRebuildQueued()
	if rebuildMarked {
		defer e.clearEntityArtifactFullRebuildQueued()
	}
	return e.withBackgroundTask(
		ctx,
		"Entity artifacts rebuild",
		trigger,
		"planning",
		backgroundEntityTaskDetail("full", 0),
		0,
		0,
		func(task *BackgroundTaskHandle) error {
			return e.rebuildEntityArtifactsFromLive(ctx, task)
		},
	)
}

func (e *Engine) RefreshEntityArtifactsForHealthTransitions(ctx context.Context, feedNames []string) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	if len(feedNames) == 0 {
		return nil
	}
	if e.entityArtifactsNeedBootstrapFast() {
		return e.RebuildEntityArtifactsWithTrigger(ctx, "health_transition")
	}
	return e.withBackgroundTask(
		ctx,
		"Entity artifacts refresh",
		"health_transition",
		"scanning memberships",
		backgroundEntityTaskDetail("health", len(feedNames)),
		0,
		len(feedNames),
		func(task *BackgroundTaskHandle) error {
			return e.refreshEntityArtifactsForHealthTransitions(ctx, feedNames, task)
		},
	)
}

func (e *Engine) refreshEntityArtifactsForHealthTransitions(ctx context.Context, feedNames []string, task *BackgroundTaskHandle) error {
	return e.runOptimisticEntityArtifactMutation(ctx, task, backgroundEntityTaskDetail("health", len(feedNames)), func() (*entityArtifactMutationPlan, error) {
		return e.stageEntityArtifactsForHealthTransitions(ctx, feedNames, task)
	})
}

func (e *Engine) stageEntityArtifactsForHealthTransitions(ctx context.Context, feedNames []string, task *BackgroundTaskHandle) (*entityArtifactMutationPlan, error) {
	ctx = nonNilContext(ctx)
	affectedCountries := map[string]struct{}{}
	affectedASNs := map[uint32]struct{}{}
	if err := e.collectHealthTransitionAffectedEntities(ctx, feedNames, task, affectedCountries, affectedASNs); err != nil {
		return nil, err
	}
	if len(affectedCountries) == 0 && len(affectedASNs) == 0 {
		return e.stageHealthTransitionHomeAggregate(ctx)
	}
	return e.stageHealthTransitionEntityPayloads(ctx, affectedCountries, affectedASNs, task)
}

func (e *Engine) rebuildEntityArtifactsFromLive(ctx context.Context, task *BackgroundTaskHandle) error {
	return e.runOptimisticEntityArtifactMutation(ctx, task, backgroundEntityTaskDetail("full", 0), func() (*entityArtifactMutationPlan, error) {
		return e.stageRebuildEntityArtifactsFromLive(ctx, task)
	})
}

func (e *Engine) stageRebuildEntityArtifactsFromLive(ctx context.Context, task *BackgroundTaskHandle) (*entityArtifactMutationPlan, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	webBatch, err := e.newWebPublishBatch()
	if err != nil {
		return nil, err
	}
	entityBatch, err := e.newEntityPublishBatch()
	if err != nil {
		webBatch.cleanup()
		return nil, err
	}
	generated, err := e.writeEntityArtifacts(ctx, nil, true, webBatch.stagedPublishBatch, entityBatch.stagedPublishBatch, task)
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
	webBatch, err := e.newWebPublishBatch()
	if err != nil {
		return nil, err
	}
	entityBatch, err := e.newEntityPublishBatch()
	if err != nil {
		webBatch.cleanup()
		return nil, err
	}

	generated, err := e.writeEntityArtifacts(ctx, feedNames, false, webBatch.stagedPublishBatch, entityBatch.stagedPublishBatch, task)
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
