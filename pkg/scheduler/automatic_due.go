package scheduler

import (
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

func (r *Runner) enqueueAutomaticDue(cfg *config.Config, snapshot Snapshot, now time.Time) {
	engineRunning := false
	if r != nil && r.eng != nil {
		engineRunning = r.eng.StatusSnapshotLight().Running
	}
	for _, item := range snapshot.Items {
		if !item.Enabled {
			continue
		}
		if !item.NextDue.IsZero() && item.NextDue.After(now) {
			continue
		}
		var src *config.Source
		if cfg != nil {
			src = cfg.Sources[item.Name]
		}
		if src == nil || src.Provenance == config.ProvenanceSecondaryRetention || src.ArtifactParent != "" {
			continue
		}
		if !shouldEnqueueAutomaticDue(item, engineRunning) {
			continue
		}
		if src.URL != "" || len(src.Static) > 0 {
			r.considerAutomaticDownload(queuedWork{
				Name:     item.Name,
				Reason:   runreason.ReasonScheduledDue,
				QueuedAt: now,
				ForceRun: item.Detail == detailCriticalProviderSetChanged,
			})
			continue
		}
	}
}

func shouldEnqueueAutomaticDue(item Item, engineRunning bool) bool {
	return !engineRunning || item.Detail != detailCriticalProviderSetChanged
}

func (r *Runner) enqueueAutomaticArtifactDue(items []Item, now time.Time) {
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		if !item.NextDue.IsZero() && item.NextDue.After(now) {
			continue
		}
		r.considerAutomaticDownload(queuedWork{
			Name:     item.Name,
			Reason:   runreason.ReasonScheduledDue,
			QueuedAt: now,
		})
	}
}
