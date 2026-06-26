package scheduler

import (
	"context"
	"time"

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
		for _, admission := range r.eng.ResolveRecheckAdmissions(ctx, names) {
			if !admission.Downloadable {
				r.logger.Warn("recheck skipped for source without downloader-stage support", "name", admission.Name)
				continue
			}
			r.enqueueDownload(queuedWork{
				Name:      admission.Target,
				Reason:    actionReason(action),
				QueuedAt:  r.now().UTC(),
				ForceRun:  true,
				Immediate: true,
			})
		}
		r.wakeDownloadLoop()
		r.wakeProcessLoop()
	case action.Reprocess:
		for _, admission := range r.eng.ReprocessAdmissions(action.Names, r.enableAll) {
			if !admission.HasLocalState {
				r.logger.Warn("reprocess skipped without local staged or committed state", "name", admission.Name)
				continue
			}
			if admission.ProviderDatabase {
				r.enqueueProviderWaveTargets(actionReason(action), r.now().UTC(), true, true, admission.ProcessingNames, admission.PromoteNames)
				continue
			}
			r.enqueueProcessingNames(actionReason(action), r.now().UTC(), true, true, admission.ProcessingNames, nil)
		}
		r.wakeProcessLoop()
	case action.RunDue:
		r.wakeDownloadLoop()
		r.wakeProcessLoop()
	default:
		for _, admission := range r.eng.DownloadAdmissions(action.Names) {
			if !admission.Downloadable {
				r.logger.Warn("run skipped for source without downloader-stage support", "name", admission.Name)
				continue
			}
			r.enqueueDownload(queuedWork{
				Name:      admission.Name,
				Reason:    actionReason(action),
				QueuedAt:  r.now().UTC(),
				ForceRun:  false,
				Immediate: true,
			})
		}
		r.wakeDownloadLoop()
	}
}

func (r *Runner) enqueueProcessingNames(reason runreason.Reason, queuedAt time.Time, forceRun, immediate bool, names, promote []string) {
	for _, name := range names {
		r.enqueueProcessing(queuedWork{
			Name:      name,
			Reason:    reason,
			QueuedAt:  queuedAt,
			ForceRun:  forceRun,
			Immediate: immediate,
			Promote:   append([]string(nil), promote...),
		})
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
