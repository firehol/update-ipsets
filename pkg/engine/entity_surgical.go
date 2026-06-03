package engine

import (
	"context"
)

func (e *Engine) RefreshEntityArtifactsForFeedUpdates(ctx context.Context, feedNames []string, trigger string) error {
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
	if e.entityArtifactsNeedBootstrapFast() {
		return e.RebuildEntityArtifactsWithTrigger(ctx, trigger)
	}
	return e.withBackgroundTask(
		"Entity artifacts refresh",
		trigger,
		"planning",
		backgroundEntityTaskDetail("feeds", len(feedNames)),
		0,
		len(feedNames),
		func(task *BackgroundTaskHandle) error {
			return e.withEntityArtifactMutation(task, backgroundEntityTaskDetail("feeds", len(feedNames)), func() error {
				return e.refreshEntityArtifactsForFeedUpdates(ctx, feedNames, task)
			})
		},
	)
}
