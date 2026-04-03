package scheduler

import (
	"github.com/firehol/update-ipsets/pkg/runreason"
	"time"
)

func (r *Runner) enqueueAutomaticDue(snapshot Snapshot, now time.Time) {
	engineRunning := false
	if r != nil && r.eng != nil {
		engineRunning = r.eng.StatusSnapshot().Running
	}
	for _, item := range snapshot.Items {
		if !item.Enabled {
			continue
		}
		if !item.NextDue.IsZero() && item.NextDue.After(now) {
			continue
		}
		src := r.eng.Config().Sources[item.Name]
		if src == nil || r.eng.IsHistoryDerivative(item.Name) || src.ArtifactParent != "" {
			continue
		}
		if !shouldEnqueueAutomaticDue(item, engineRunning) {
			continue
		}
		if r.eng.IsDownloadable(item.Name) {
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
