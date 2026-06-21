package scheduler

import (
	"context"
	"time"

	"github.com/firehol/update-ipsets/pkg/runreason"
)

func (r *Runner) recoverStagedWork(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return
	}
	if recovered, err := r.eng.RecoverStagedSources(); err != nil {
		r.logger.Error("failed to recover staged sources", "error", err)
	} else {
		for _, name := range recovered {
			if ctx.Err() != nil {
				return
			}
			r.enqueueRecoveredStagedSource(name)
		}
	}
	if ctx.Err() != nil {
		return
	}
	if recovered, err := r.eng.RecoverStagedArtifacts(ctx, r.enableAll); err != nil {
		r.logger.Error("failed to recover staged artifacts", "error", err)
	} else {
		for artifactName, children := range recovered {
			if ctx.Err() != nil {
				return
			}
			promote := append([]string{artifactName}, children...)
			for _, name := range children {
				if ctx.Err() != nil {
					return
				}
				r.enqueueProcessing(queuedWork{
					Name:      name,
					Reason:    runreason.ReasonScheduledDue,
					QueuedAt:  r.now().UTC(),
					ForceRun:  true,
					Immediate: true,
					Promote:   append([]string(nil), promote...),
				})
			}
		}
	}
	if len(r.ActivitySnapshot().ProcessingWaiting) > 0 {
		r.wakeProcessLoop()
	}
	if len(r.ActivitySnapshot().DownloadWaiting) > 0 {
		r.wakeDownloadLoop()
	}
}
func (r *Runner) enqueueProviderDefaultsReprocess(now time.Time) {
	if r == nil || r.eng == nil {
		return
	}
	if r.eng.StatusSnapshotLight().Running {
		return
	}
	if !r.eng.ProviderDefaultsChanged() {
		return
	}
	r.enqueueProviderWave(runreason.ReasonProviderDefaults, now, true, true, nil)
}
func (r *Runner) enqueueRecoveredStagedSource(name string) {
	queuedAt := r.now().UTC()
	if r.eng.IsProviderDatabase(name) {
		r.enqueueProviderWave(runreason.ReasonScheduledDue, queuedAt, true, true, []string{name})
		return
	}
	r.enqueueProcessing(queuedWork{
		Name:      name,
		Reason:    runreason.ReasonScheduledDue,
		QueuedAt:  queuedAt,
		ForceRun:  true,
		Immediate: true,
	})
}

func (r *Runner) enqueueProviderWave(reason runreason.Reason, queuedAt time.Time, forceRun, immediate bool, promote []string) {
	targets := r.eng.FullFeedReprocessTargets(r.enableAll)
	for _, name := range targets {
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

func (r *Runner) promoteNamesForProviderReprocess(name string) []string {
	if name == "" || !r.eng.HasStagedDownload(name) {
		return nil
	}
	return []string{name}
}
