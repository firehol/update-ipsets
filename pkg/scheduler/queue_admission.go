package scheduler

import (
	"context"
	"sync"
)

func (r *Runner) considerAutomaticDownload(item queuedWork) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.enqueueDownloadLocked(item)
}

func (r *Runner) enqueueProcessing(item queuedWork) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.enqueueProcessingLocked(item)
}

func mergeQueuedWork(current, incoming queuedWork) queuedWork {
	if current.Name == "" {
		current.Name = incoming.Name
	}
	if current.Kind == "" || current.Kind == queuedWorkKindNormal {
		current.Kind = incoming.Kind
	}
	if current.QueuedAt.IsZero() || (!incoming.QueuedAt.IsZero() && incoming.QueuedAt.Before(current.QueuedAt)) {
		current.QueuedAt = incoming.QueuedAt
	}
	if current.EnqueueSeq == 0 || (incoming.EnqueueSeq > 0 && incoming.EnqueueSeq < current.EnqueueSeq) {
		current.EnqueueSeq = incoming.EnqueueSeq
	}
	if incoming.ForceRun {
		current.ForceRun = true
	}
	if incoming.Immediate {
		current.Immediate = true
	}
	if incoming.Reason != "" {
		current.Reason = incoming.Reason
	}
	current.Promote = mergePromoteNames(current.Promote, incoming.Promote)
	return current
}

func mergePromoteNames(current, incoming []string) []string {
	if len(incoming) == 0 {
		return current
	}
	seen := make(map[string]bool, len(current))
	for _, name := range current {
		if name != "" {
			seen[name] = true
		}
	}
	for _, name := range incoming {
		if name == "" || seen[name] {
			continue
		}
		current = append(current, name)
		seen[name] = true
	}
	return current
}

func (r *Runner) enqueueDownload(item queuedWork) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.enqueueDownloadLocked(item)
}

func (r *Runner) enqueueDownloadLocked(item queuedWork) {
	if item.Name == "" {
		return
	}
	if item.Kind == "" {
		item.Kind = queuedWorkKindNormal
	}
	if item.EnqueueSeq == 0 {
		r.downloadEnqueueSeq++
		item.EnqueueSeq = r.downloadEnqueueSeq
	}
	if current, ok := r.download.waiting[item.Name]; ok {
		r.download.waiting[item.Name] = mergeQueuedWork(current, item)
		return
	}
	if _, ok := r.download.active[item.Name]; ok {
		r.metrics.recordDownloadDeferred()
		r.download.refetchPending[item.Name] = mergeQueuedWork(r.download.refetchPending[item.Name], item)
		return
	}
	r.download.waiting[item.Name] = item
	r.metrics.recordDownloadEnqueue(len(r.download.waiting))
}

func (r *Runner) enqueueProcessingLocked(item queuedWork) {
	if item.Name == "" {
		return
	}
	if _, ok := r.processing.waiting[item.Name]; ok {
		r.processing.waiting[item.Name] = mergeQueuedWork(r.processing.waiting[item.Name], item)
		return
	}
	if _, ok := r.processing.active[item.Name]; ok {
		if r.processing.deferred == nil {
			r.processing.deferred = make(map[string]queuedWork)
		}
		r.processing.deferred[item.Name] = mergeQueuedWork(r.processing.deferred[item.Name], item)
		return
	}
	r.processing.waiting[item.Name] = item
	r.metrics.recordProcessingEnqueue(len(r.processing.waiting))
}

func (r *Runner) dispatchDownloads(ctx context.Context, workers int, wg *sync.WaitGroup) bool {
	dispatched := false
	for {
		if r.activeDownloadCount() >= workers {
			return dispatched
		}
		item, ok := r.startNextDownload()
		if !ok {
			return dispatched
		}
		dispatched = true
		if wg == nil {
			go r.runDownload(ctx, item)
			continue
		}
		wg.Go(func() {
			r.runDownload(ctx, item)
		})
	}
}

func (r *Runner) activeDownloadCount() int {
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()
	return len(r.download.active)
}

func (r *Runner) startNextDownload() (queuedWork, bool) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	if len(r.download.waiting) == 0 {
		return queuedWork{}, false
	}
	var next queuedWork
	var blocked queuedWork
	first := true
	blockedFirst := true
	for _, item := range r.download.waiting {
		if blockedFirst || queuedWorkBefore(item, blocked) {
			blocked = item
			blockedFirst = false
		}
		if !r.downloadInputsSettledLocked(item.Name) {
			continue
		}
		if first || queuedWorkBefore(item, next) {
			next = item
			first = false
		}
	}
	if first {
		if len(r.download.active) > 0 || blockedFirst {
			return queuedWork{}, false
		}
		next = blocked
	}
	delete(r.download.waiting, next.Name)
	r.download.active[next.Name] = ActiveQueueFeed{
		Name:      next.Name,
		Kind:      string(next.Kind),
		Reason:    next.Reason,
		StartedAt: r.now().UTC(),
	}
	r.metrics.recordDownloadStart()
	return next, true
}

func (r *Runner) downloadInputsSettledLocked(name string) bool {
	if r == nil || r.eng == nil {
		return true
	}
	src := r.eng.Config().Sources[name]
	if src == nil || len(src.DerivedFrom) == 0 {
		return true
	}
	for _, parent := range src.DerivedFrom {
		if _, waiting := r.download.waiting[parent]; waiting {
			return false
		}
		if _, active := r.download.active[parent]; active {
			return false
		}
	}
	return true
}

func (r *Runner) finishDownload(name string) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	delete(r.download.active, name)
	r.metrics.recordDownloadFinish()
}
func (r *Runner) requeuePendingDownloads(items []queuedWork) {
	for _, item := range items {
		r.releaseDeferredDownload(item.Name)
	}
}

func (r *Runner) releaseDeferredDownload(name string) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	pending, ok := r.download.refetchPending[name]
	if !ok {
		return
	}
	if _, exists := r.download.waiting[name]; exists {
		return
	}
	if _, exists := r.download.active[name]; exists {
		return
	}
	delete(r.download.refetchPending, name)
	r.download.waiting[name] = pending
}

func queuedWorkBefore(a, b queuedWork) bool {
	if !a.QueuedAt.Equal(b.QueuedAt) {
		if a.QueuedAt.IsZero() {
			return false
		}
		if b.QueuedAt.IsZero() {
			return true
		}
		return a.QueuedAt.Before(b.QueuedAt)
	}
	if a.EnqueueSeq != b.EnqueueSeq {
		if a.EnqueueSeq == 0 {
			return false
		}
		if b.EnqueueSeq == 0 {
			return true
		}
		return a.EnqueueSeq < b.EnqueueSeq
	}
	return a.Name < b.Name
}
