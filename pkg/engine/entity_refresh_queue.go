package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	entityRefreshMaxLaneWaves = 2
	entityRefreshMaxLaneHold  = time.Minute
)

var (
	entityRefreshQueueHookMu        sync.Mutex
	entityArtifactRefreshAfterDrain func()
	entityHealthRefreshAfterDrain   func()
	entityArtifactRefreshAfterTask  func()
	entityHealthRefreshAfterTask    func()
)

func setEntityArtifactRefreshAfterDrainHookForTest(fn func()) func() {
	entityRefreshQueueHookMu.Lock()
	old := entityArtifactRefreshAfterDrain
	entityArtifactRefreshAfterDrain = fn
	entityRefreshQueueHookMu.Unlock()
	return func() {
		entityRefreshQueueHookMu.Lock()
		entityArtifactRefreshAfterDrain = old
		entityRefreshQueueHookMu.Unlock()
	}
}

func setEntityHealthRefreshAfterDrainHookForTest(fn func()) func() {
	entityRefreshQueueHookMu.Lock()
	old := entityHealthRefreshAfterDrain
	entityHealthRefreshAfterDrain = fn
	entityRefreshQueueHookMu.Unlock()
	return func() {
		entityRefreshQueueHookMu.Lock()
		entityHealthRefreshAfterDrain = old
		entityRefreshQueueHookMu.Unlock()
	}
}

func setEntityArtifactRefreshAfterTaskHookForTest(fn func()) func() {
	entityRefreshQueueHookMu.Lock()
	old := entityArtifactRefreshAfterTask
	entityArtifactRefreshAfterTask = fn
	entityRefreshQueueHookMu.Unlock()
	return func() {
		entityRefreshQueueHookMu.Lock()
		entityArtifactRefreshAfterTask = old
		entityRefreshQueueHookMu.Unlock()
	}
}

func setEntityHealthRefreshAfterTaskHookForTest(fn func()) func() {
	entityRefreshQueueHookMu.Lock()
	old := entityHealthRefreshAfterTask
	entityHealthRefreshAfterTask = fn
	entityRefreshQueueHookMu.Unlock()
	return func() {
		entityRefreshQueueHookMu.Lock()
		entityHealthRefreshAfterTask = old
		entityRefreshQueueHookMu.Unlock()
	}
}

func entityArtifactRefreshAfterDrainHookForTest() func() {
	entityRefreshQueueHookMu.Lock()
	defer entityRefreshQueueHookMu.Unlock()
	return entityArtifactRefreshAfterDrain
}

func entityHealthRefreshAfterDrainHookForTest() func() {
	entityRefreshQueueHookMu.Lock()
	defer entityRefreshQueueHookMu.Unlock()
	return entityHealthRefreshAfterDrain
}

func entityArtifactRefreshAfterTaskHookForTest() func() {
	entityRefreshQueueHookMu.Lock()
	defer entityRefreshQueueHookMu.Unlock()
	return entityArtifactRefreshAfterTask
}

func entityHealthRefreshAfterTaskHookForTest() func() {
	entityRefreshQueueHookMu.Lock()
	defer entityRefreshQueueHookMu.Unlock()
	return entityHealthRefreshAfterTask
}

type EntityArtifactQueueResult struct {
	Ticket    LaneTicket    `json:"ticket"`
	Queued    bool          `json:"queued"`
	Coalesced bool          `json:"coalesced"`
	State     LaneWorkState `json:"state"`
}

func (e *Engine) QueueEntityArtifactsRebuild(ctx context.Context, trigger string) (EntityArtifactQueueResult, error) {
	if e == nil {
		return EntityArtifactQueueResult{}, nil
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return EntityArtifactQueueResult{}, err
	}
	if trigger == "" {
		trigger = "operator_rebuild"
	}
	ticket, err := e.engineLane.Submit(ctx, LaneWork{
		Kind:          LaneWorkEntityRebuild,
		Component:     LaneComponentEntityArtifacts,
		Name:          "entity.rebuild",
		Trigger:       trigger,
		Stage:         "queued",
		Detail:        "full entity artifact rebuild",
		CoalescingKey: "entity:rebuild:full",
	}, func(laneCtx context.Context) error {
		return e.rebuildEntityArtifactsWithTriggerAdmitted(laneCtx, trigger)
	})
	return entityArtifactQueueResult(ticket), err
}

// QueueEntityArtifactsRefreshForFeedUpdates coalesces feed-update entity
// refresh requests before they become background tasks. The scheduler may
// complete several processing batches while a previous entity refresh is still
// running; repeated feed names must collapse into one later refresh wave.
func (e *Engine) QueueEntityArtifactsRefreshForFeedUpdates(ctx context.Context, feedNames []string, trigger string) (EntityArtifactQueueResult, error) {
	if e == nil {
		return EntityArtifactQueueResult{}, nil
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return EntityArtifactQueueResult{}, err
	}
	feedNames = uniqueNonEmptyStrings(feedNames)
	if len(feedNames) == 0 {
		return EntityArtifactQueueResult{}, nil
	}
	if e.preferredGeoProvider() == "" && e.preferredASNProvider() == "" {
		return EntityArtifactQueueResult{}, nil
	}
	if trigger == "" {
		trigger = "feed_update"
	}
	started, pending, coalescingKey := e.enqueueEntityArtifactRefresh(feedNames)
	if e.logger != nil {
		e.logger.Info("queued entity artifact refresh", "feeds", len(feedNames), "pending", pending, "trigger", trigger)
	}
	if !started {
		return coalescedEntityArtifactQueueResult(LaneWorkEntityRefresh, LaneComponentEntityArtifacts), nil
	}
	ticket, err := e.engineLane.Submit(ctx, LaneWork{
		Kind:          LaneWorkEntityRefresh,
		Component:     LaneComponentEntityArtifacts,
		Name:          "entity.refresh",
		Trigger:       trigger,
		Stage:         "queued",
		Detail:        backgroundEntityTaskDetail("feeds", pending),
		CoalescingKey: coalescingKey,
	}, func(laneCtx context.Context) error {
		return e.runQueuedEntityArtifactRefresh(laneCtx, trigger)
	})
	if err != nil {
		e.finishEntityArtifactRefreshQueue()
		return EntityArtifactQueueResult{}, err
	}
	return entityArtifactQueueResult(ticket), nil
}

func (e *Engine) QueueEntityArtifactsRefreshForHealthTransitions(ctx context.Context, feedNames []string) (EntityArtifactQueueResult, error) {
	if e == nil {
		return EntityArtifactQueueResult{}, nil
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return EntityArtifactQueueResult{}, err
	}
	feedNames = uniqueNonEmptyStrings(feedNames)
	if len(feedNames) == 0 {
		return EntityArtifactQueueResult{}, nil
	}
	if e.preferredGeoProvider() == "" && e.preferredASNProvider() == "" {
		return EntityArtifactQueueResult{}, nil
	}
	started, pending, coalescingKey := e.enqueueEntityHealthRefresh(feedNames)
	if e.logger != nil {
		e.logger.Info("queued entity health-transition refresh", "feeds", len(feedNames), "pending", pending, "trigger", "health_transition")
	}
	if !started {
		return coalescedEntityArtifactQueueResult(LaneWorkEntityRefresh, LaneComponentEntityArtifactsHealth), nil
	}
	ticket, err := e.engineLane.Submit(ctx, LaneWork{
		Kind:          LaneWorkEntityRefresh,
		Component:     LaneComponentEntityArtifactsHealth,
		Name:          "entity.health_refresh",
		Trigger:       "health_transition",
		Stage:         "queued",
		Detail:        backgroundEntityTaskDetail("health", pending),
		CoalescingKey: coalescingKey,
	}, func(laneCtx context.Context) error {
		return e.runQueuedEntityHealthRefresh(laneCtx)
	})
	if err != nil {
		e.finishEntityHealthRefreshQueue()
		return EntityArtifactQueueResult{}, err
	}
	return entityArtifactQueueResult(ticket), nil
}

func entityArtifactQueueResult(ticket LaneTicket) EntityArtifactQueueResult {
	return EntityArtifactQueueResult{
		Ticket:    ticket,
		Queued:    ticket.Queued,
		Coalesced: ticket.Coalesced,
		State:     ticket.State,
	}
}

func coalescedEntityArtifactQueueResult(kind LaneWorkKind, component LaneWorkComponent) EntityArtifactQueueResult {
	ticket := LaneTicket{
		Kind:      kind,
		Component: component,
		Coalesced: true,
		State:     LaneWorkActive,
	}
	return entityArtifactQueueResult(ticket)
}

func (e *Engine) enqueueEntityArtifactRefresh(feedNames []string) (bool, int, string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.entityRefreshPending == nil {
		e.entityRefreshPending = make(map[string]struct{}, len(feedNames))
	}
	for _, name := range uniqueNonEmptyStrings(feedNames) {
		e.entityRefreshPending[name] = struct{}{}
	}
	if e.entityRefreshRunning {
		return false, len(e.entityRefreshPending), ""
	}
	e.entityRefreshRunning = true
	e.entityRefreshContinuation ^= 1
	coalescingKey := entityRefreshContinuationCoalescingKey(e.entityRefreshContinuation)
	return true, len(e.entityRefreshPending), coalescingKey
}

func (e *Engine) enqueueEntityHealthRefresh(feedNames []string) (bool, int, string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.entityHealthPending == nil {
		e.entityHealthPending = make(map[string]struct{}, len(feedNames))
	}
	for _, name := range uniqueNonEmptyStrings(feedNames) {
		e.entityHealthPending[name] = struct{}{}
	}
	if e.entityHealthRunning {
		return false, len(e.entityHealthPending), ""
	}
	e.entityHealthRunning = true
	e.entityHealthContinuation ^= 1
	coalescingKey := entityHealthContinuationCoalescingKey(e.entityHealthContinuation)
	return true, len(e.entityHealthPending), coalescingKey
}

func (e *Engine) drainEntityArtifactRefreshPending() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.entityRefreshPending) == 0 {
		return nil
	}
	names := make([]string, 0, len(e.entityRefreshPending))
	for name := range e.entityRefreshPending {
		names = append(names, name)
	}
	e.entityRefreshPending = nil
	return uniqueNonEmptyStrings(names)
}

func (e *Engine) drainEntityHealthRefreshPending() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.entityHealthPending) == 0 {
		return nil
	}
	names := make([]string, 0, len(e.entityHealthPending))
	for name := range e.entityHealthPending {
		names = append(names, name)
	}
	e.entityHealthPending = nil
	return uniqueNonEmptyStrings(names)
}

func (e *Engine) runQueuedEntityArtifactRefresh(ctx context.Context, trigger string) (runErr error) {
	ctx = nonNilContext(ctx)
	defer func() {
		if recovered := recover(); recovered != nil {
			e.finishEntityArtifactRefreshQueue()
			panic(recovered)
		}
	}()
	if trigger == "" {
		trigger = "feed_update"
	}
	started := time.Now()
	waves := 0
	for {
		if err := contextErr(ctx); err != nil {
			e.finishEntityArtifactRefreshQueue()
			return errors.Join(runErr, err)
		}
		err := e.withEngineLaneBackgroundTask(
			ctx,
			LaneWorkEntityRefresh,
			LaneComponentEntityArtifacts,
			backgroundTaskEntityArtifactsRefresh,
			trigger,
			"coalescing",
			"coalescing changed feed entity refresh requests",
			0,
			0,
			func(task *BackgroundTaskHandle) error {
				return e.runEntityArtifactRefreshQueue(ctx, task)
			},
		)
		if err != nil && e.logger != nil {
			e.logger.Error("failed to refresh queued entity artifacts", "trigger", trigger, "error", err)
		}
		runErr = errors.Join(runErr, err)
		if hook := entityArtifactRefreshAfterTaskHookForTest(); hook != nil {
			hook()
		}
		waves++

		e.mu.Lock()
		pending := len(e.entityRefreshPending)
		hasPending := pending > 0
		if !hasPending {
			e.entityRefreshRunning = false
		}
		e.mu.Unlock()
		if !hasPending {
			return runErr
		}
		if waves >= entityRefreshMaxLaneWaves || time.Since(started) >= entityRefreshMaxLaneHold {
			coalescingKey, pending, ok := e.prepareEntityArtifactRefreshContinuation()
			if ok {
				e.submitEntityArtifactRefreshContinuation(ctx, trigger, pending, coalescingKey)
			}
			return runErr
		}
	}
}

func (e *Engine) runQueuedEntityHealthRefresh(ctx context.Context) (runErr error) {
	ctx = nonNilContext(ctx)
	defer func() {
		if recovered := recover(); recovered != nil {
			e.finishEntityHealthRefreshQueue()
			panic(recovered)
		}
	}()
	started := time.Now()
	waves := 0
	for {
		if err := contextErr(ctx); err != nil {
			e.finishEntityHealthRefreshQueue()
			return errors.Join(runErr, err)
		}
		err := e.withEngineLaneBackgroundTask(
			ctx,
			LaneWorkEntityRefresh,
			LaneComponentEntityArtifactsHealth,
			backgroundTaskEntityArtifactsRefresh,
			"health_transition",
			"coalescing",
			"coalescing health-transition entity refresh requests",
			0,
			0,
			func(task *BackgroundTaskHandle) error {
				return e.runEntityHealthRefreshQueue(ctx, task)
			},
		)
		if err != nil && e.logger != nil {
			e.logger.Error("failed to refresh queued entity health transitions", "error", err)
		}
		runErr = errors.Join(runErr, err)
		if hook := entityHealthRefreshAfterTaskHookForTest(); hook != nil {
			hook()
		}
		waves++

		e.mu.Lock()
		pending := len(e.entityHealthPending)
		hasPending := pending > 0
		if !hasPending {
			e.entityHealthRunning = false
		}
		e.mu.Unlock()
		if !hasPending {
			return runErr
		}
		if waves >= entityRefreshMaxLaneWaves || time.Since(started) >= entityRefreshMaxLaneHold {
			coalescingKey, pending, ok := e.prepareEntityHealthRefreshContinuation()
			if ok {
				e.submitEntityHealthRefreshContinuation(ctx, pending, coalescingKey)
			}
			return runErr
		}
	}
}

func (e *Engine) prepareEntityArtifactRefreshContinuation() (string, int, bool) {
	if e == nil {
		return "", 0, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	pending := len(e.entityRefreshPending)
	if pending == 0 {
		e.entityRefreshRunning = false
		return "", 0, false
	}
	e.entityRefreshRunning = true
	e.entityRefreshContinuation ^= 1
	return entityRefreshContinuationCoalescingKey(e.entityRefreshContinuation), pending, true
}

func (e *Engine) submitEntityArtifactRefreshContinuation(ctx context.Context, trigger string, pending int, coalescingKey string) {
	if e == nil || e.engineLane == nil || pending == 0 || coalescingKey == "" {
		e.finishEntityArtifactRefreshQueue()
		return
	}
	ctx = nonNilContext(ctx)
	ticket, err := e.engineLane.Submit(ctx, LaneWork{
		Kind:          LaneWorkEntityRefresh,
		Component:     LaneComponentEntityArtifacts,
		Name:          "entity.refresh",
		Trigger:       trigger,
		Stage:         "queued",
		Detail:        backgroundEntityTaskDetail("feeds", pending),
		CoalescingKey: coalescingKey,
	}, func(laneCtx context.Context) error {
		return e.runQueuedEntityArtifactRefresh(laneCtx, trigger)
	})
	if err != nil {
		e.finishEntityArtifactRefreshQueue()
		if e.logger != nil {
			e.logger.Error("failed to resubmit queued entity artifact refresh", "trigger", trigger, "error", err)
		}
		return
	}
	if e.logger != nil {
		e.logger.Info("resubmitted entity artifact refresh to release engine lane", "pending", pending, "ticket", ticket.ID)
	}
}

func (e *Engine) prepareEntityHealthRefreshContinuation() (string, int, bool) {
	if e == nil {
		return "", 0, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	pending := len(e.entityHealthPending)
	if pending == 0 {
		e.entityHealthRunning = false
		return "", 0, false
	}
	e.entityHealthRunning = true
	e.entityHealthContinuation ^= 1
	return entityHealthContinuationCoalescingKey(e.entityHealthContinuation), pending, true
}

func (e *Engine) submitEntityHealthRefreshContinuation(ctx context.Context, pending int, coalescingKey string) {
	if e == nil || e.engineLane == nil || pending == 0 || coalescingKey == "" {
		e.finishEntityHealthRefreshQueue()
		return
	}
	ctx = nonNilContext(ctx)
	ticket, err := e.engineLane.Submit(ctx, LaneWork{
		Kind:          LaneWorkEntityRefresh,
		Component:     LaneComponentEntityArtifactsHealth,
		Name:          "entity.health_refresh",
		Trigger:       "health_transition",
		Stage:         "queued",
		Detail:        backgroundEntityTaskDetail("health", pending),
		CoalescingKey: coalescingKey,
	}, func(laneCtx context.Context) error {
		return e.runQueuedEntityHealthRefresh(laneCtx)
	})
	if err != nil {
		e.finishEntityHealthRefreshQueue()
		if e.logger != nil {
			e.logger.Error("failed to resubmit queued entity health refresh", "error", err)
		}
		return
	}
	if e.logger != nil {
		e.logger.Info("resubmitted entity health refresh to release engine lane", "pending", pending, "ticket", ticket.ID)
	}
}

func entityRefreshContinuationCoalescingKey(parity int) string {
	if parity&1 == 0 {
		return "entity:refresh:feed_updates:continuation:0"
	}
	return "entity:refresh:feed_updates:continuation:1"
}

func entityHealthContinuationCoalescingKey(parity int) string {
	if parity&1 == 0 {
		return "entity:refresh:health:continuation:0"
	}
	return "entity:refresh:health:continuation:1"
}

func (e *Engine) finishEntityArtifactRefreshQueue() {
	e.mu.Lock()
	e.entityRefreshRunning = false
	e.mu.Unlock()
}

func (e *Engine) finishEntityHealthRefreshQueue() {
	e.mu.Lock()
	e.entityHealthRunning = false
	e.mu.Unlock()
}

func (e *Engine) entityArtifactFullRebuildQueuedOrRunning() bool {
	if e == nil {
		return false
	}
	e.mu.RLock()
	queued := e.entityRebuildQueued
	e.mu.RUnlock()
	e.backgroundTasksMu.RLock()
	for _, task := range e.backgroundTasks {
		if task.Kind == LaneWorkEntityRebuild {
			queued = true
			break
		}
	}
	e.backgroundTasksMu.RUnlock()
	if queued {
		return true
	}
	if e.engineLane == nil {
		return false
	}
	snap := e.engineLane.Snapshot()
	for _, item := range snap.Active {
		if item.Kind == LaneWorkEntityRebuild {
			return true
		}
	}
	for _, item := range snap.Waiting {
		if item.Kind == LaneWorkEntityRebuild {
			return true
		}
	}
	return false
}

func (e *Engine) tryMarkEntityArtifactFullRebuildQueued() bool {
	if e == nil {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.entityRebuildQueued {
		return false
	}
	e.entityRebuildQueued = true
	return true
}

func (e *Engine) clearEntityArtifactFullRebuildQueued() {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.entityRebuildQueued = false
	e.mu.Unlock()
}

func (e *Engine) runEntityArtifactRefreshQueue(ctx context.Context, task *BackgroundTaskHandle) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	names := e.drainEntityArtifactRefreshPending()
	if len(names) == 0 {
		return nil
	}
	if hook := entityArtifactRefreshAfterDrainHookForTest(); hook != nil {
		hook()
	}
	if task != nil {
		task.Update(
			"coalescing",
			fmt.Sprintf("processing %d coalesced changed feeds", len(names)),
			0,
			len(names),
		)
	}
	snap := e.operationSnapshot()
	if e.entityArtifactsNeedBootstrapFastWithSnapshot(snap) {
		if task != nil {
			task.Update("bootstrap", "entity artifacts are missing or stale; rebuilding full entity surface", 0, 0)
		}
		return e.rebuildEntityArtifactsFromLiveWithSnapshot(ctx, snap, task)
	}
	return e.refreshEntityArtifactsForFeedUpdatesWithSnapshot(ctx, snap, names, task)
}

func (e *Engine) runEntityHealthRefreshQueue(ctx context.Context, task *BackgroundTaskHandle) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	names := e.drainEntityHealthRefreshPending()
	if len(names) == 0 {
		return nil
	}
	if hook := entityHealthRefreshAfterDrainHookForTest(); hook != nil {
		hook()
	}
	if task != nil {
		task.Update(
			"coalescing",
			fmt.Sprintf("processing %d coalesced health-transition feeds", len(names)),
			0,
			len(names),
		)
	}
	snap := e.operationSnapshot()
	if e.entityArtifactsNeedBootstrapFastWithSnapshot(snap) {
		if task != nil {
			task.Update("bootstrap", "entity artifacts are missing or stale; rebuilding full entity surface", 0, 0)
		}
		return e.rebuildEntityArtifactsFromLiveWithSnapshot(ctx, snap, task)
	}
	return e.refreshEntityArtifactsForHealthTransitionsWithSnapshot(ctx, snap, names, task)
}
