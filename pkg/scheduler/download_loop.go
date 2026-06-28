package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
)

func (r *Runner) runFetchLoop(ctx context.Context, wg *sync.WaitGroup) {
	for {
		now := r.now().UTC()
		prev := r.currentSnapshot()
		cfg, rt, policy := r.eng.ConfigRuntimePolicySnapshot()
		workers := rt.ParallelDownloads
		if workers < 1 {
			workers = 1
		}
		entries := r.eng.EntriesSnapshotWithArtifactsForConfig(cfg)
		snapshot := BuildSnapshotWithPolicy(cfg, rt, policy, entries, r.enableAll, now)
		artifactItems := BuildArtifactItemsWithPolicy(cfg, rt, policy, entries, r.enableAll, now)
		snapshot.ArtifactItems = artifactItems
		r.storeSnapshot(snapshot)
		transitions := healthTransitionDetails(prev, snapshot)
		if len(transitions) > 0 {
			names := make([]string, 0, len(transitions))
			for _, t := range transitions {
				names = append(names, t.Feed)
			}
			if _, err := r.eng.QueueEntityArtifactsRefreshForHealthTransitions(ctx, names); err != nil {
				r.logger.Error("failed to queue entity health-transition refresh", "feeds", len(names), "error", err)
			}
			r.stateMu.Lock()
			const maxTransitions = 20
			r.recentHealthTransitions = append(r.recentHealthTransitions, transitions...)
			if len(r.recentHealthTransitions) > maxTransitions {
				r.recentHealthTransitions = r.recentHealthTransitions[len(r.recentHealthTransitions)-maxTransitions:]
			}
			r.stateMu.Unlock()
		}
		r.enqueueProviderDefaultsReprocess(now)
		r.enqueueAutomaticDue(cfg, snapshot, now)
		r.enqueueAutomaticArtifactDue(artifactItems, now)
		if r.dispatchDownloads(ctx, workers, wg) {
			continue
		}
		wait := nextWait(now, snapshot.Items, artifactItems)
		if wait < time.Second {
			wait = time.Second
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		case <-r.download.wake:
			timer.Stop()
		}
	}
}
func (r *Runner) runDownload(ctx context.Context, item queuedWork) {
	defer r.wakeDownloadLoop()
	finished := false
	defer func() {
		if recovered := recover(); recovered != nil {
			r.recordRecoveredPanic("download_worker", recovered)
			if !finished {
				r.finishDownload(item.Name)
			}
			r.releaseDeferredDownload(item.Name)
		}
	}()
	started := time.Now()
	decision, err := r.fetchQueuedDownload(ctx, item)
	r.metrics.observeOperation("scheduler.fetch_and_stage", time.Since(started))
	statusName := decision.Status.String()
	if statusName == "" {
		statusName = "unknown"
	}
	r.eng.TryObserveCounter("download.status."+statusName, 1, decision.BodySize)
	if decision.HTTPCode > 0 {
		r.eng.TryObserveCounter(fmt.Sprintf("download.http_status.%d", decision.HTTPCode), 1, decision.BodySize)
	}
	if len(decision.ProcessingNames) > 0 {
		r.eng.TryObserveCounter("download.processing_names", int64(len(decision.ProcessingNames)), 0)
	}
	r.finishDownload(item.Name)
	finished = true
	if err != nil {
		r.logger.Error("download loop failed", "name", item.Name, "error", err)
		if item.Kind == queuedWorkKindRecoveredArtifact && errors.Is(err, engine.ErrRecoveredArtifactCorrupt) {
			r.enqueueDownload(queuedWork{
				Name:      item.Name,
				Reason:    item.Reason,
				QueuedAt:  r.now().UTC(),
				ForceRun:  true,
				Immediate: true,
			})
		}
		r.releaseDeferredDownload(item.Name)
		return
	}
	for _, name := range decision.ProcessingNames {
		r.enqueueProcessing(queuedWork{
			Name:      name,
			Reason:    item.Reason,
			QueuedAt:  r.now().UTC(),
			ForceRun:  item.ForceRun,
			Immediate: item.Immediate,
			Promote:   append([]string(nil), decision.PromoteNames...),
		})
	}
	if len(decision.ProcessingNames) == 0 && len(decision.PromoteNames) > 0 {
		if err := r.eng.PromoteCommittedDownloads(decision.PromoteNames); err != nil {
			r.logger.Error("failed to promote staged downloads without processing batch", "names", decision.PromoteNames, "error", err)
		}
	}
	if len(decision.ProcessingNames) > 0 {
		r.wakeProcessLoop()
	}
	r.releaseDeferredDownload(item.Name)
}

func (r *Runner) fetchQueuedDownload(ctx context.Context, item queuedWork) (engine.DownloadDecision, error) {
	if item.Kind == queuedWorkKindRecoveredArtifact {
		return r.eng.RecoverStagedArtifact(ctx, item.Name, r.enableAll)
	}
	return r.eng.FetchAndStage(ctx, item.Name, item.ForceRun, r.enableAll)
}
