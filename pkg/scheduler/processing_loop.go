package scheduler

import (
	"context"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

func (r *Runner) runProcessingLoop(ctx context.Context) {
	timer := time.NewTimer(r.processingInterval())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			r.runQueuedProcessing(ctx)
			resetProcessingTimer(timer, r.processingInterval())
		case <-r.processing.wake:
			r.runQueuedProcessing(ctx)
			resetProcessingTimer(timer, r.processingInterval())
		}
	}
}

func (r *Runner) processingInterval() time.Duration {
	_, rt := r.eng.ConfigRuntimeSnapshot()
	interval := time.Duration(rt.ProcessingIntervalMinutes) * time.Minute
	if interval <= 0 {
		return 10 * time.Minute
	}
	return interval
}

func resetProcessingTimer(timer *time.Timer, interval time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(interval)
}

func (r *Runner) runQueuedProcessing(ctx context.Context) {
	var activeItems []queuedWork
	defer func() {
		if recovered := recover(); recovered != nil {
			r.recoverProcessingBatchPanic(activeItems, recovered)
		}
	}()
	items := r.drainProcessingQueue()
	if len(items) == 0 {
		return
	}
	r.metrics.recordBatchStart(len(items))
	names := queuedProcessingNames(items)
	reason := combineReasons(items)
	reprocess := queuedProcessingReprocess(items)
	r.markProcessingActive(items)
	activeItems = items
	batchStarted := time.Now()
	runOnceStarted := time.Now()
	report, err := r.eng.RunOnce(ctx, engine.RunOptions{
		Selected:              names,
		EnableAll:             r.enableAll,
		Reprocess:             reprocess,
		Manual:                reason != runreason.ReasonScheduledDue,
		CleanupOld:            true,
		Reason:                reason,
		AsyncCachePersistence: true,
		BeforePublish: func(report *engine.Report) error {
			successItems, _ := splitProcessingItemsByFailure(items, report)
			promoteNames := r.promoteNamesForBatch(successItems, queuedProcessingNames(successItems))
			if len(promoteNames) == 0 {
				return nil
			}
			promoteStarted := time.Now()
			err := r.eng.PromoteCommittedDownloads(promoteNames)
			r.metrics.observeOperation("scheduler.promote_committed_downloads", time.Since(promoteStarted))
			if err != nil {
				r.logger.Error("failed to promote staged downloads", "names", promoteNames, "error", err)
			}
			return err
		},
	})
	r.metrics.observeOperation("scheduler.run_once", time.Since(runOnceStarted))
	if err != nil {
		r.logger.Error("processing batch failed", "error", err)
		released := r.finishProcessing(items, true)
		batchDur := time.Since(batchStarted)
		r.metrics.recordBatchComplete(len(items), batchDur)
		r.metrics.observeOperation("scheduler.processing_batch_total", batchDur)
		if released {
			r.wakeProcessLoop()
		}
		return
	}
	successItems, failedItems := splitProcessingItemsByFailure(items, report)
	released := false
	if len(successItems) > 0 {
		if r.finishProcessing(successItems, false) {
			released = true
		}
	}
	if len(failedItems) > 0 {
		r.logProcessingFailuresForRetry(failedItems, report)
		if r.finishProcessing(failedItems, true) {
			released = true
		}
	}
	r.requeuePendingDownloads(items)
	r.wakeDownloadLoop()
	if released {
		r.wakeProcessLoop()
	}
	batchDur := time.Since(batchStarted)
	r.metrics.recordBatchComplete(len(items), batchDur)
	r.metrics.observeOperation("scheduler.processing_batch_total", batchDur)
	r.logger.Info("processing batch completed", "updated", len(report.Updated), "skipped", len(report.Skipped), "failed", len(report.Failed))
	entityTargets := report.EntityRefreshTargets
	if len(entityTargets) == 0 {
		entityTargets = report.Updated
	}
	if len(entityTargets) > 0 {
		if _, err := r.eng.QueueEntityArtifactsRefreshForFeedUpdates(ctx, entityTargets, reason.String()); err != nil {
			r.logger.Error("failed to queue entity artifact refresh", "feeds", len(entityTargets), "trigger", reason.String(), "error", err)
		}
	}
	activeItems = nil
}

func (r *Runner) recoverProcessingBatchPanic(activeItems []queuedWork, recovered any) {
	r.recordRecoveredPanic("processing_batch", recovered)
	if len(activeItems) == 0 {
		return
	}
	if r.finishProcessing(activeItems, true) {
		r.wakeProcessLoop()
	}
}

func queuedProcessingReprocess(items []queuedWork) bool {
	for _, item := range items {
		if queuedProcessingReasonReprocess(item.Reason) {
			return true
		}
	}
	return false
}

func queuedProcessingReasonReprocess(reason runreason.Reason) bool {
	switch reason {
	case runreason.ReasonManualReprocess,
		runreason.ReasonIntegrityReprocess,
		runreason.ReasonStartupIntegrityReprocess,
		runreason.ReasonProviderDefaults:
		return true
	default:
		return false
	}
}

func (r *Runner) drainProcessingQueue() []queuedWork {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	if len(r.processing.waiting) == 0 {
		return nil
	}
	out := make([]queuedWork, 0, len(r.processing.waiting))
	for _, item := range r.processing.waiting {
		out = append(out, item)
	}
	r.processing.waiting = make(map[string]queuedWork)
	return out
}

func (r *Runner) markProcessingActive(items []queuedWork) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	startedAt := r.now().UTC()
	for _, item := range items {
		r.processing.active[item.Name] = ActiveQueueFeed{
			Name:      item.Name,
			Reason:    item.Reason,
			StartedAt: startedAt,
		}
	}
}

func (r *Runner) finishProcessing(items []queuedWork, requeue bool) bool {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	released := false
	for _, item := range items {
		delete(r.processing.active, item.Name)
		if requeue {
			if pending, ok := r.processing.deferred[item.Name]; ok {
				delete(r.processing.deferred, item.Name)
				item = mergeQueuedWork(item, pending)
				released = true
			}
			r.processing.waiting[item.Name] = mergeQueuedWork(r.processing.waiting[item.Name], item)
			r.metrics.recordProcessingRequeue(len(r.processing.waiting))
			continue
		}
		if pending, ok := r.processing.deferred[item.Name]; ok {
			delete(r.processing.deferred, item.Name)
			r.processing.waiting[item.Name] = mergeQueuedWork(r.processing.waiting[item.Name], pending)
			r.metrics.recordProcessingEnqueue(len(r.processing.waiting))
			released = true
		}
	}
	return released
}
func queuedProcessingNames(items []queuedWork) []string {
	if len(items) == 0 {
		return nil
	}
	names := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.Name == "" {
			continue
		}
		if _, ok := seen[item.Name]; ok {
			continue
		}
		seen[item.Name] = struct{}{}
		names = append(names, item.Name)
	}
	return names
}
func (r *Runner) promoteNamesForBatch(items []queuedWork, fallback []string) []string {
	out := make([]string, 0, len(fallback))
	seen := make(map[string]bool, len(fallback))
	for _, item := range items {
		for _, name := range item.Promote {
			if name == "" || seen[name] {
				continue
			}
			out = append(out, name)
			seen[name] = true
		}
	}
	for _, name := range fallback {
		if name == "" || seen[name] {
			continue
		}
		out = append(out, name)
		seen[name] = true
	}
	return out
}

func combineReasons(items []queuedWork) runreason.Reason {
	if len(items) == 0 {
		return runreason.ReasonScheduledDue
	}
	reason := items[0].Reason
	for _, item := range items[1:] {
		if item.Reason != reason {
			if item.Reason == runreason.ReasonManualReprocess || reason == runreason.ReasonManualReprocess {
				return runreason.ReasonManualReprocess
			}
			if item.Reason == runreason.ReasonManualRecheck || reason == runreason.ReasonManualRecheck {
				return runreason.ReasonManualRecheck
			}
			return runreason.ReasonManualRun
		}
	}
	if reason == "" {
		return runreason.ReasonScheduledDue
	}
	return reason
}

func splitProcessingItemsByFailure(items []queuedWork, report *engine.Report) (success []queuedWork, failed []queuedWork) {
	if len(items) == 0 {
		return nil, nil
	}
	failedSet := make(map[string]struct{}, len(items))
	if report != nil {
		for _, name := range report.Failed {
			if name != "" {
				failedSet[name] = struct{}{}
			}
		}
	}
	success = make([]queuedWork, 0, len(items))
	failed = make([]queuedWork, 0, len(failedSet))
	for _, item := range items {
		if _, ok := failedSet[item.Name]; ok {
			failed = append(failed, item)
			continue
		}
		success = append(success, item)
	}
	return success, failed
}

func (r *Runner) logProcessingFailuresForRetry(items []queuedWork, report *engine.Report) {
	if r == nil || report == nil {
		return
	}
	for _, item := range items {
		r.logger.Error("processing item failed; staged input retained and retry scheduled",
			"name", item.Name,
			"reason", item.Reason,
			"status", report.Statuses[item.Name],
			"message", report.Messages[item.Name],
			"retry_scheduled", true,
		)
	}
}
