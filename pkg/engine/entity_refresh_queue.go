package engine

import (
	"context"
	"fmt"
	"strings"
)

func (e *Engine) QueueEntityArtifactsRebuild(ctx context.Context, trigger string) bool {
	if e == nil {
		return false
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return false
	}
	if trigger == "" {
		trigger = "operator_rebuild"
	}

	e.mu.Lock()
	if e.entityRebuildQueued || e.backgroundTaskNamedLocked("Entity artifacts rebuild") {
		e.mu.Unlock()
		return false
	}
	e.entityRebuildQueued = true
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			e.entityRebuildQueued = false
			e.mu.Unlock()
		}()
		if err := e.RebuildEntityArtifactsWithTrigger(ctx, trigger); err != nil && e.logger != nil {
			e.logger.Error("failed to queue entity artifacts rebuild", "trigger", trigger, "error", err)
		}
	}()
	return true
}

// QueueEntityArtifactsRefreshForFeedUpdates coalesces feed-update entity
// refresh requests before they become background tasks. The scheduler may
// complete several processing batches while a previous entity refresh is still
// running; repeated feed names must collapse into one later refresh wave.
func (e *Engine) QueueEntityArtifactsRefreshForFeedUpdates(ctx context.Context, feedNames []string, trigger string) {
	if e == nil {
		return
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return
	}
	feedNames = uniqueNonEmptyStrings(feedNames)
	if len(feedNames) == 0 {
		return
	}
	if e.preferredGeoProvider() == "" && e.preferredASNProvider() == "" {
		return
	}
	if trigger == "" {
		trigger = "feed_update"
	}
	shouldStart, pending := e.enqueueEntityArtifactRefresh(feedNames)
	if e.logger != nil {
		e.logger.Info("queued entity artifact refresh", "feeds", len(feedNames), "pending", pending, "trigger", trigger)
	}
	if shouldStart {
		go e.runQueuedEntityArtifactRefresh(ctx, trigger)
	}
}

func (e *Engine) QueueEntityArtifactsRefreshForHealthTransitions(ctx context.Context, feedNames []string) {
	if e == nil {
		return
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return
	}
	feedNames = uniqueNonEmptyStrings(feedNames)
	if len(feedNames) == 0 {
		return
	}
	if e.preferredGeoProvider() == "" && e.preferredASNProvider() == "" {
		return
	}
	shouldStart, pending := e.enqueueEntityHealthRefresh(feedNames)
	if e.logger != nil {
		e.logger.Info("queued entity health-transition refresh", "feeds", len(feedNames), "pending", pending, "trigger", "health_transition")
	}
	if shouldStart {
		go e.runQueuedEntityHealthRefresh(ctx)
	}
}

func (e *Engine) enqueueEntityArtifactRefresh(feedNames []string) (bool, int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.entityRefreshPending == nil {
		e.entityRefreshPending = make(map[string]struct{}, len(feedNames))
	}
	for _, name := range uniqueNonEmptyStrings(feedNames) {
		e.entityRefreshPending[name] = struct{}{}
	}
	if e.entityRefreshRunning {
		return false, len(e.entityRefreshPending)
	}
	e.entityRefreshRunning = true
	return true, len(e.entityRefreshPending)
}

func (e *Engine) enqueueEntityHealthRefresh(feedNames []string) (bool, int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.entityHealthPending == nil {
		e.entityHealthPending = make(map[string]struct{}, len(feedNames))
	}
	for _, name := range uniqueNonEmptyStrings(feedNames) {
		e.entityHealthPending[name] = struct{}{}
	}
	if e.entityHealthRunning {
		return false, len(e.entityHealthPending)
	}
	e.entityHealthRunning = true
	return true, len(e.entityHealthPending)
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

func (e *Engine) runQueuedEntityArtifactRefresh(ctx context.Context, trigger string) {
	ctx = nonNilContext(ctx)
	if trigger == "" {
		trigger = "feed_update"
	}
	for {
		if err := contextErr(ctx); err != nil {
			e.finishEntityArtifactRefreshQueue()
			return
		}
		err := e.withBackgroundTask(
			"Entity artifacts refresh",
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

		e.mu.Lock()
		hasPending := len(e.entityRefreshPending) > 0
		if !hasPending {
			e.entityRefreshRunning = false
		}
		e.mu.Unlock()
		if !hasPending {
			return
		}
	}
}

func (e *Engine) runQueuedEntityHealthRefresh(ctx context.Context) {
	ctx = nonNilContext(ctx)
	for {
		if err := contextErr(ctx); err != nil {
			e.finishEntityHealthRefreshQueue()
			return
		}
		err := e.withBackgroundTask(
			"Entity artifacts refresh",
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

		e.mu.Lock()
		hasPending := len(e.entityHealthPending) > 0
		if !hasPending {
			e.entityHealthRunning = false
		}
		e.mu.Unlock()
		if !hasPending {
			return
		}
	}
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

func (e *Engine) runEntityArtifactRefreshQueue(ctx context.Context, task *BackgroundTaskHandle) error {
	ctx = nonNilContext(ctx)
	for {
		if err := contextErr(ctx); err != nil {
			return err
		}
		names := e.drainEntityArtifactRefreshPending()
		if len(names) == 0 {
			return nil
		}
		if task != nil {
			task.Update(
				"coalescing",
				fmt.Sprintf("processing %d coalesced changed feeds", len(names)),
				0,
				len(names),
			)
		}
		if e.entityArtifactsNeedBootstrapFast() {
			if task != nil {
				task.Update("bootstrap", "entity artifacts are missing or stale; rebuilding full entity surface", 0, 0)
			}
			if err := e.withEntityArtifactMutation(task, backgroundEntityTaskDetail("full", 0), func() error {
				return e.rebuildEntityArtifactsFromLive(ctx, task)
			}); err != nil {
				return err
			}
			continue
		}
		if err := e.withEntityArtifactMutation(task, backgroundEntityTaskDetail("feeds", len(names)), func() error {
			return e.refreshEntityArtifactsForFeedUpdates(ctx, names, task)
		}); err != nil {
			return err
		}
	}
}

func (e *Engine) runEntityHealthRefreshQueue(ctx context.Context, task *BackgroundTaskHandle) error {
	ctx = nonNilContext(ctx)
	for {
		if err := contextErr(ctx); err != nil {
			return err
		}
		names := e.drainEntityHealthRefreshPending()
		if len(names) == 0 {
			return nil
		}
		if task != nil {
			task.Update(
				"coalescing",
				fmt.Sprintf("processing %d coalesced health-transition feeds", len(names)),
				0,
				len(names),
			)
		}
		if e.entityArtifactsNeedBootstrapFast() {
			if task != nil {
				task.Update("bootstrap", "entity artifacts are missing or stale; rebuilding full entity surface", 0, 0)
			}
			if err := e.withEntityArtifactMutation(task, backgroundEntityTaskDetail("full", 0), func() error {
				return e.rebuildEntityArtifactsFromLive(ctx, task)
			}); err != nil {
				return err
			}
			continue
		}
		if err := e.withEntityArtifactMutation(task, backgroundEntityTaskDetail("health", len(names)), func() error {
			return e.refreshEntityArtifactsForHealthTransitions(ctx, names, task)
		}); err != nil {
			return err
		}
	}
}

func (e *Engine) backgroundTaskNamedLocked(name string) bool {
	if e == nil || strings.TrimSpace(name) == "" {
		return false
	}
	for _, task := range e.backgroundTasks {
		if task.Name == name {
			return true
		}
	}
	return false
}
