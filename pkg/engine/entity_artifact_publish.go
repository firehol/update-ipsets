package engine

import (
	"context"
	"errors"
	"time"

	"github.com/firehol/update-ipsets/pkg/output"
)

const entityArtifactMutationMaxAttempts = 3

var errEntityArtifactStageStale = errors.New("entity artifact staged mutation is stale")

type entityArtifactMutationPlan struct {
	web       *webPublishBatch
	entity    *entityPublishBatch
	generated []output.GeneratedFile

	publishStage   string
	publishDetail  string
	publishCurrent int
	publishTotal   int
}

func (p *entityArtifactMutationPlan) cleanup() {
	if p == nil {
		return
	}
	if p.web != nil {
		p.web.cleanup()
	}
	if p.entity != nil {
		p.entity.cleanup()
	}
}

func (e *Engine) entityArtifactGenerationSnapshot() uint64 {
	if e == nil {
		return 0
	}
	e.entityArtifactsMu.Lock()
	defer e.entityArtifactsMu.Unlock()
	return e.entityArtifactsGeneration
}

func (e *Engine) bumpEntityArtifactGenerationLocked() {
	if e == nil {
		return
	}
	e.entityArtifactsGeneration++
}

func (e *Engine) runOptimisticEntityArtifactMutation(ctx context.Context, task *BackgroundTaskHandle, detail string, stage func() (*entityArtifactMutationPlan, error)) error {
	ctx = nonNilContext(ctx)
	for attempt := 1; attempt <= entityArtifactMutationMaxAttempts; attempt++ {
		if err := contextErr(ctx); err != nil {
			return err
		}
		expectedGeneration := e.entityArtifactGenerationSnapshot()
		plan, err := stage()
		if err != nil {
			return err
		}
		if plan == nil {
			return nil
		}
		err = e.publishEntityArtifactMutationPlan(ctx, task, detail, expectedGeneration, plan)
		plan.cleanup()
		if err == nil {
			return nil
		}
		if !errors.Is(err, errEntityArtifactStageStale) {
			return err
		}
		e.observeRunCounter("entity.writer_stale_stage_retry", 1, 0)
	}
	return errEntityArtifactStageStale
}

func (e *Engine) publishEntityArtifactMutationPlan(ctx context.Context, task *BackgroundTaskHandle, detail string, expectedGeneration uint64, plan *entityArtifactMutationPlan) error {
	ctx = nonNilContext(ctx)
	if plan == nil {
		return nil
	}
	if plan.web != nil {
		if err := plan.web.applyGeneratedFileTimestampsContext(ctx, plan.generated); err != nil {
			return err
		}
	}
	if task != nil {
		task.Update("waiting for entity artifact writer", detail, 0, 0)
	}
	waitStarted := time.Now()
	e.entityArtifactsMu.Lock()
	e.observeRunOperation("entity.writer_lock_wait", time.Since(waitStarted))
	holdStarted := time.Now()
	defer e.entityArtifactsMu.Unlock()
	defer func() {
		e.observeRunOperation("entity.writer_lock_hold", time.Since(holdStarted))
	}()
	if e.entityArtifactsGeneration != expectedGeneration {
		return errEntityArtifactStageStale
	}
	if task != nil && plan.publishStage != "" {
		task.Update(plan.publishStage, plan.publishDetail, plan.publishCurrent, plan.publishTotal)
	}
	mutatesLive := plan.entity != nil || plan.web != nil
	defer func() {
		if mutatesLive {
			e.bumpEntityArtifactGenerationLocked()
		}
	}()
	var published []string
	if plan.entity != nil {
		if _, err := plan.entity.publishContext(ctx); err != nil {
			return err
		}
	}
	if plan.web != nil {
		var err error
		published, err = plan.web.publishContext(ctx)
		if err != nil {
			return err
		}
	}
	if err := e.syncGeneratedFiles(plan.generated, published); err != nil {
		return err
	}
	return nil
}
