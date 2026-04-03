package scheduler

import (
	"context"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

// PendingAction describes an admin-triggered action for specific sources.
type PendingAction struct {
	Names     []string
	Recheck   bool
	Reprocess bool
	RunDue    bool
	Reason    runreason.Reason
}

func (r *Runner) handleAction(ctx context.Context, action PendingAction) {
	switch {
	case action.Recheck:
		names := action.Names
		if len(names) == 0 {
			r.logger.Warn("manual recheck requires explicit feed names")
			return
		}
		for _, name := range names {
			if !r.eng.IsDownloadable(name) {
				r.logger.Warn("recheck skipped for source without downloader-stage support", "name", name)
				continue
			}
			target := r.eng.ResolveRecheckTarget(ctx, name)
			r.enqueueDownload(queuedWork{
				Name:      target,
				Reason:    actionReason(action),
				QueuedAt:  r.now().UTC(),
				ForceRun:  true,
				Immediate: true,
			})
		}
		r.wakeDownloadLoop()
		r.wakeProcessLoop()
	case action.Reprocess:
		names := action.Names
		if len(names) == 0 {
			names = config.SortedSourceNames(r.eng.Config())
		}
		for _, name := range names {
			if !r.eng.HasLocalReprocessState(name) {
				r.logger.Warn("reprocess skipped without local staged or committed state", "name", name)
				continue
			}
			if r.eng.IsProviderDatabase(name) {
				r.enqueueProviderWave(actionReason(action), r.now().UTC(), true, true, r.promoteNamesForProviderReprocess(name))
				continue
			}
			r.enqueueProcessing(queuedWork{
				Name:      name,
				Reason:    actionReason(action),
				QueuedAt:  r.now().UTC(),
				ForceRun:  true,
				Immediate: true,
			})
		}
		r.wakeProcessLoop()
	case action.RunDue:
		now := r.now().UTC()
		snapshot := BuildSnapshot(r.eng.Config(), r.eng.Runtime(), r.eng.EntriesSnapshot(), r.enableAll, now)
		r.storeSnapshot(snapshot)
		r.enqueueAutomaticDue(snapshot, now)
		r.wakeDownloadLoop()
		r.wakeProcessLoop()
	default:
		names := action.Names
		if len(names) == 0 {
			names = config.SortedSourceNames(r.eng.Config())
		}
		for _, name := range names {
			if !r.eng.IsDownloadable(name) {
				r.logger.Warn("run skipped for source without downloader-stage support", "name", name)
				continue
			}
			r.enqueueDownload(queuedWork{
				Name:      name,
				Reason:    actionReason(action),
				QueuedAt:  r.now().UTC(),
				ForceRun:  false,
				Immediate: true,
			})
		}
		r.wakeDownloadLoop()
	}
}
func actionReason(action PendingAction) runreason.Reason {
	if action.Reason.Valid() && action.Reason != runreason.ReasonUnknown {
		return action.Reason
	}
	switch {
	case action.Reprocess:
		return runreason.ReasonManualReprocess
	case action.Recheck:
		return runreason.ReasonManualRecheck
	case len(action.Names) > 0 || action.RunDue:
		return runreason.ReasonManualRun
	default:
		return runreason.ReasonScheduledDue
	}
}
