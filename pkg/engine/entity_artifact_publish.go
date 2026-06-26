package engine

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/pkg/output"
)

const entityArtifactMutationMaxAttempts = 3

var errEntityArtifactStageStale = errors.New("entity artifact staged mutation is stale")

var entityArtifactPublishHookMu sync.Mutex
var entityArtifactPublishAfterLeaseHook func()

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

type entityArtifactPublishLease struct {
	engine      *Engine
	holdStarted time.Time
	released    bool
}

func setEntityArtifactPublishAfterLeaseHookForTest(fn func()) func() {
	entityArtifactPublishHookMu.Lock()
	old := entityArtifactPublishAfterLeaseHook
	entityArtifactPublishAfterLeaseHook = fn
	entityArtifactPublishHookMu.Unlock()
	return func() {
		entityArtifactPublishHookMu.Lock()
		entityArtifactPublishAfterLeaseHook = old
		entityArtifactPublishHookMu.Unlock()
	}
}

func entityArtifactPublishAfterLeaseHookForTest() func() {
	entityArtifactPublishHookMu.Lock()
	defer entityArtifactPublishHookMu.Unlock()
	return entityArtifactPublishAfterLeaseHook
}

func (e *Engine) acquireEntityArtifactPublishLease(ctx context.Context, expectedGeneration uint64) (*entityArtifactPublishLease, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	waitStarted := time.Now()
	e.entityArtifactPublishMu.Lock()
	lease := &entityArtifactPublishLease{
		engine:      e,
		holdStarted: time.Now(),
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			lease.release(false)
			panic(recovered)
		}
	}()
	e.observeRunOperation("entity.writer_lock_wait", time.Since(waitStarted))
	e.entityArtifactsMu.Lock()
	stale := e.entityArtifactsGeneration != expectedGeneration
	e.entityArtifactsMu.Unlock()
	if stale {
		lease.release(false)
		return nil, errEntityArtifactStageStale
	}
	if hook := entityArtifactPublishAfterLeaseHookForTest(); hook != nil {
		hook()
	}
	return lease, nil
}

func (l *entityArtifactPublishLease) release(mutatesLive bool) {
	if l == nil || l.released {
		return
	}
	l.released = true
	e := l.engine
	if mutatesLive {
		e.entityArtifactsMu.Lock()
		e.bumpEntityArtifactGenerationLocked()
		e.entityArtifactsMu.Unlock()
	}
	e.observeRunOperation("entity.writer_lock_hold", time.Since(l.holdStarted))
	e.entityArtifactPublishMu.Unlock()
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
	lease, err := e.acquireEntityArtifactPublishLease(ctx, expectedGeneration)
	if err != nil {
		return err
	}
	if task != nil && plan.publishStage != "" {
		task.Update(plan.publishStage, plan.publishDetail, plan.publishCurrent, plan.publishTotal)
	}
	mutatesLive := plan.entity != nil || plan.web != nil
	defer lease.release(mutatesLive)
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
	// Release the entity publish lease before git work. The git lane may run
	// subprocesses, but it must still be awaited here because it stages live
	// file paths, not immutable content snapshots.
	lease.release(mutatesLive)
	if err := e.syncGeneratedFiles(ctx, plan.generated, published); err != nil {
		return err
	}
	if mutatesLive {
		e.MarkIntegrityCachesStale()
	}
	return nil
}
