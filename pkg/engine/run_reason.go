package engine

import (
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

func normalizeRunReason(opts RunOptions) runreason.Reason {
	if opts.Reason.Valid() && opts.Reason != runreason.ReasonUnknown {
		return opts.Reason
	}
	switch {
	case opts.Reprocess:
		return runreason.ReasonManualReprocess
	case opts.Recheck:
		return runreason.ReasonManualRecheck
	case opts.Manual || len(opts.Selected) > 0:
		return runreason.ReasonManualRun
	default:
		return runreason.ReasonScheduledDue
	}
}

type feedAttempt struct {
	e       *Engine
	entry   *cache.Entry
	name    string
	started time.Time
}

func (e *Engine) beginFeedAttempt(entry *cache.Entry, reason runreason.Reason) *feedAttempt {
	started := e.now().UTC()
	name := ""
	if entry != nil {
		name = entry.Snapshot().Name
		entry.MarkRunStarted(reason)
	}
	e.markFeedStart(name, ActiveFeed{
		Name:      name,
		Reason:    reason,
		StartedAt: started,
	})
	return &feedAttempt{
		e:       e,
		entry:   entry,
		name:    name,
		started: started,
	}
}

func (a *feedAttempt) finish() {
	if a == nil || a.e == nil {
		return
	}
	defer a.e.markFeedEnd(a.name)
	if a.entry == nil {
		return
	}
	elapsed := a.e.now().UTC().Sub(a.started)
	if elapsed < 0 {
		elapsed = 0
	}
	ms := elapsed.Milliseconds()
	if elapsed > 0 && ms == 0 {
		ms = 1
	}
	a.entry.RecordProcessingDuration(ms)
}
