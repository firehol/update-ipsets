package engine

import (
	"context"
)

func (e *Engine) RefreshEntityArtifactsForFeedUpdates(ctx context.Context, feedNames []string, trigger string) error {
	if e == nil || e.engineLane == nil {
		return nil
	}
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	feedNames = uniqueNonEmptyStrings(feedNames)
	if len(feedNames) == 0 {
		return nil
	}
	if e.preferredGeoProvider() == "" && e.preferredASNProvider() == "" {
		return nil
	}
	if trigger == "" {
		trigger = "feed_update"
	}
	return e.engineLane.Run(ctx, LaneWork{
		Kind:      LaneWorkEntityRefresh,
		Component: LaneComponentEntityArtifacts,
		Name:      "entity.refresh",
		Trigger:   trigger,
		Stage:     "planning",
		Detail:    backgroundEntityTaskDetail("feeds", len(feedNames)),
	}, func(laneCtx context.Context) error {
		return e.refreshEntityArtifactsForFeedUpdatesAdmitted(laneCtx, feedNames, trigger)
	})
}

func (e *Engine) refreshEntityArtifactsForFeedUpdatesAdmitted(ctx context.Context, feedNames []string, trigger string) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	feedNames = uniqueNonEmptyStrings(feedNames)
	if len(feedNames) == 0 {
		return nil
	}
	snap := e.operationSnapshot()
	if preferredGeoProviderForConfig(snap.cfg) == "" && preferredASNProviderForConfig(snap.cfg) == "" {
		return nil
	}
	if trigger == "" {
		trigger = "feed_update"
	}
	if e.entityArtifactsNeedBootstrapFastWithSnapshot(snap) {
		return e.rebuildEntityArtifactsWithTriggerAdmittedWithSnapshot(ctx, snap, trigger)
	}
	return e.withEngineLaneBackgroundTask(
		ctx,
		LaneWorkEntityRefresh,
		LaneComponentEntityArtifacts,
		backgroundTaskEntityArtifactsRefresh,
		trigger,
		"planning",
		backgroundEntityTaskDetail("feeds", len(feedNames)),
		0,
		len(feedNames),
		func(task *BackgroundTaskHandle) error {
			return e.refreshEntityArtifactsForFeedUpdatesWithSnapshot(ctx, snap, feedNames, task)
		},
	)
}
