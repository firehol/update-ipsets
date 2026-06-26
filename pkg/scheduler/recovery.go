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
	if recovered, err := r.eng.RecoverStagedArtifacts(ctx); err != nil {
		r.logger.Error("failed to recover staged artifacts", "error", err)
	} else {
		for _, artifactName := range recovered {
			if ctx.Err() != nil {
				return
			}
			r.enqueueDownload(queuedWork{
				Name:      artifactName,
				Kind:      queuedWorkKindRecoveredArtifact,
				Reason:    runreason.ReasonScheduledDue,
				QueuedAt:  r.now().UTC(),
				ForceRun:  true,
				Immediate: true,
			})
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
	if targets, ok := r.eng.ProviderReprocessTargetsForSource(name, r.enableAll); ok {
		r.enqueueProviderWaveTargets(runreason.ReasonScheduledDue, queuedAt, true, true, targets, []string{name})
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
	r.enqueueProviderWaveTargets(reason, queuedAt, forceRun, immediate, targets, promote)
}

func (r *Runner) enqueueProviderWaveTargets(reason runreason.Reason, queuedAt time.Time, forceRun, immediate bool, targets, promote []string) {
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
