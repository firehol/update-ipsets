package engine

import "context"

func (e *Engine) repairEntityArtifactsWithPlan(ctx context.Context, trigger string, plan entityIntegrityPlan) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	if !plan.hasWork() {
		return nil
	}
	taskName := "Entity artifacts repair"
	if plan.full {
		taskName = "Entity artifacts rebuild"
	}
	return e.withBackgroundTask(
		taskName,
		trigger,
		"planning",
		backgroundEntityTaskDetail("integrity", plan.targetCount()),
		0,
		0,
		func(task *BackgroundTaskHandle) error {
			return e.withEntityArtifactMutation(task, backgroundEntityTaskDetail("integrity", plan.targetCount()), func() error {
				if err := contextErr(ctx); err != nil {
					return err
				}
				_, freshPlan, err := e.CheckEntityArtifactsIntegrity()
				if err != nil {
					return err
				}
				if !freshPlan.hasWork() {
					e.observeRunCounter("entity.integrity_repair.stale_plan_skipped", 1, 0)
					if task != nil {
						task.Update("clean", "entity artifacts are already current after queued repair revalidation", 0, 0)
					}
					return nil
				}
				if trigger == "startup" && freshPlan.shouldDeferStartupRepair() {
					e.observeRunCounter("entity.integrity_startup_repair_deferred_after_revalidation", int64(freshPlan.targetCount()), 0)
					if task != nil {
						task.Update("deferred", "startup entity repair is too broad for automatic background repair", 0, 0)
					}
					if e.logger != nil {
						e.logger.Warn("deferred broad startup entity artifact repair after revalidation",
							"targets", freshPlan.targetCount(),
							"limit", maxStartupEntityAutoRepairTargets)
					}
					return nil
				}
				plan = freshPlan
				if task != nil {
					task.Update("repairing", backgroundEntityTaskDetail("integrity", plan.targetCount()), 0, 0)
				}
				if plan.full {
					return e.rebuildEntityArtifactsFromLive(ctx, task)
				}
				homeRefreshed := false
				if len(plan.feedNames) > 0 {
					if err := contextErr(ctx); err != nil {
						return err
					}
					if err := e.rebuildEntityArtifactsForFeeds(ctx, plan.sortedFeeds(), task); err != nil {
						return err
					}
					homeRefreshed = true
				}
				if len(plan.countryCodes) > 0 || len(plan.asns) > 0 || plan.rebuildCountryIndex || plan.rebuildASNIndex {
					if err := contextErr(ctx); err != nil {
						return err
					}
					if err := e.rewriteSelectedEntityArtifacts(ctx, plan.countryCodes, plan.asns, plan.rebuildCountryIndex, plan.rebuildASNIndex, task); err != nil {
						return err
					}
					homeRefreshed = true
				}
				if len(plan.healthFeeds) > 0 {
					if err := e.refreshEntityArtifactsForHealthTransitions(ctx, plan.sortedHealthFeeds(), task); err != nil {
						return err
					}
					homeRefreshed = true
				}
				if plan.rebuildHomeAggregate && !homeRefreshed {
					if err := contextErr(ctx); err != nil {
						return err
					}
					if err := e.rewriteHomeAggregate(ctx, task); err != nil {
						return err
					}
				}
				return nil
			})
		},
	)
}
