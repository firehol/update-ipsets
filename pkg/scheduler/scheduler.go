package scheduler

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/engine"
)

const maxJSONUnixSeconds int64 = 253402300799
const snapshotReadMaxAge = 2 * time.Second
const detailCriticalProviderSetChanged = "critical infrastructure provider set changed"

type downloadLoopState struct {
	wake           chan struct{}
	waiting        map[string]queuedWork
	active         map[string]ActiveQueueFeed
	refetchPending map[string]queuedWork
}

type processingLoopState struct {
	wake     chan struct{}
	waiting  map[string]queuedWork
	active   map[string]ActiveQueueFeed
	deferred map[string]queuedWork
}

type Runner struct {
	eng       *engine.Engine
	enableAll bool
	logger    *slog.Logger
	now       func() time.Time
	statePath string

	activeFeedSnapshot func() []engine.ActiveFeed

	mu       sync.RWMutex
	snapshot Snapshot

	actionCh chan PendingAction

	stateMu    sync.RWMutex
	download   downloadLoopState
	processing processingLoopState
	metrics    metricsState

	recentHealthTransitions []HealthTransition
}

func New(eng *engine.Engine, enableAll bool, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	runner := &Runner{
		eng:                eng,
		enableAll:          enableAll,
		logger:             logger,
		now:                func() time.Time { return time.Now().UTC() },
		statePath:          filepath.Join(eng.Runtime().CacheDir, "scheduler-state.json"),
		activeFeedSnapshot: eng.ActiveFeedsSnapshot,
		actionCh:           make(chan PendingAction, 64),
		download: downloadLoopState{
			wake:           make(chan struct{}, 1),
			waiting:        make(map[string]queuedWork),
			active:         make(map[string]ActiveQueueFeed),
			refetchPending: make(map[string]queuedWork),
		},
		processing: processingLoopState{
			wake:     make(chan struct{}, 1),
			waiting:  make(map[string]queuedWork),
			active:   make(map[string]ActiveQueueFeed),
			deferred: make(map[string]queuedWork),
		},
	}
	if snapshot, err := LoadSnapshot(runner.statePath); err == nil && len(snapshot.Items) > 0 {
		runner.snapshot = snapshot
	}
	return runner
}

func (r *Runner) wakeDownloadLoop() bool {
	select {
	case r.download.wake <- struct{}{}:
		return true
	default:
		return false
	}
}

func (r *Runner) wakeProcessLoop() bool {
	select {
	case r.processing.wake <- struct{}{}:
		return true
	default:
		return false
	}
}

func (r *Runner) Trigger() bool {
	wokeDownload := r.wakeDownloadLoop()
	wokeProcess := r.wakeProcessLoop()
	return wokeDownload || wokeProcess
}

func (r *Runner) TriggerSources(action PendingAction) {
	r.actionCh <- action
	r.wakeDownloadLoop()
	r.wakeProcessLoop()
}

// TriggerQueuedAction queues an action only if the scheduler wake-up channel
// accepts it immediately. Used by endpoints that historically returned a
// conflict when a duplicate trigger was already pending.
func (r *Runner) TriggerQueuedAction(action PendingAction) bool {
	if len(r.actionCh) > 0 {
		return false
	}
	select {
	case r.actionCh <- action:
		r.wakeDownloadLoop()
		r.wakeProcessLoop()
		return true
	default:
		return false
	}
}

// Snapshot returns a recent scheduler snapshot for admin reads.
//
// The fetch loop persists fresh snapshots for scheduling decisions,
// but admin reads also need reasonably current next-due data after
// processing changes cache entries. To avoid rebuilding on every
// poll while also avoiding long-lived stale data, Snapshot serves a
// short-lived cached copy and rebuilds only when that cache has aged
// past snapshotReadMaxAge.
func (r *Runner) Snapshot() Snapshot {
	now := r.now().UTC()
	r.mu.RLock()
	cached := Snapshot{
		GeneratedAt: r.snapshot.GeneratedAt,
		Items:       append([]Item(nil), r.snapshot.Items...),
	}
	r.mu.RUnlock()
	if r.eng == nil {
		return cached
	}
	if !cached.GeneratedAt.IsZero() && now.Sub(cached.GeneratedAt) <= snapshotReadMaxAge {
		return cached
	}
	snapshot := BuildSnapshot(
		r.eng.Config(),
		r.eng.Runtime(),
		r.eng.EntriesSnapshot(),
		r.enableAll,
		now,
	)
	r.mu.Lock()
	r.snapshot = snapshot
	r.mu.Unlock()
	return snapshot
}

func (r *Runner) EnableAll() bool {
	if r == nil {
		return false
	}
	return r.enableAll
}

func (r *Runner) ActivitySnapshot() ActivitySnapshot {
	includeProcessing := func(name string) bool {
		return r.eng == nil || !r.eng.IsProviderDatabase(name)
	}
	lookup := r.queueStatusLookup()
	r.stateMu.RLock()
	downloadWaiting := queueSnapshotFromMap(r.download.waiting, lookup)
	for i := range downloadWaiting {
		if !r.downloadInputsSettledLocked(downloadWaiting[i].Name) {
			downloadWaiting[i].Blocked = true
			if r.eng != nil {
				if src := r.eng.Config().Sources[downloadWaiting[i].Name]; src != nil {
					downloadWaiting[i].BlockedParents = src.DerivedFrom
				}
			}
		}
	}
	downloadActive := activeSnapshotFromMap(r.download.active, lookup)
	downloadRefetchPending := queueSnapshotFromMap(r.download.refetchPending, lookup)
	processingWaiting := queueSnapshotFromMapFiltered(r.processing.waiting, includeProcessing, lookup)
	processingDeferred := queueSnapshotFromMapFiltered(r.processing.deferred, includeProcessing, lookup)
	recentTransitions := append([]HealthTransition(nil), r.recentHealthTransitions...)
	r.stateMu.RUnlock()
	return ActivitySnapshot{
		DownloadWaiting:         downloadWaiting,
		DownloadActive:          downloadActive,
		DownloadRefetchPending:  downloadRefetchPending,
		ProcessingWaiting:       processingWaiting,
		ProcessingActive:        r.operatorProcessingActive(includeProcessing),
		ProcessingDeferred:      processingDeferred,
		RecentHealthTransitions: recentTransitions,
	}
}

func (r *Runner) operatorProcessingActive(include func(name string) bool) []ActiveQueueFeed {
	if r == nil || r.activeFeedSnapshot == nil {
		return nil
	}
	activeFeeds := r.activeFeedSnapshot()
	if len(activeFeeds) == 0 {
		return nil
	}
	items := make(map[string]ActiveQueueFeed, len(activeFeeds))
	for _, feed := range activeFeeds {
		if feed.Name == "" {
			continue
		}
		if include != nil && !include(feed.Name) {
			continue
		}
		items[feed.Name] = ActiveQueueFeed{
			Name:      feed.Name,
			Reason:    feed.Reason,
			StartedAt: feed.StartedAt,
		}
	}
	return activeSnapshotFromMap(items, r.queueStatusLookup())
}

func (r *Runner) queueStatusLookup() func(name string) queueStatusView {
	if r == nil || r.eng == nil {
		return nil
	}
	entries := r.eng.EntriesSnapshot()
	index := make(map[string]cache.Entry, len(entries))
	for _, entry := range entries {
		index[entry.Name] = entry
	}
	return func(name string) queueStatusView {
		entry, ok := index[name]
		if !ok {
			return queueStatusView{}
		}
		isFeed := true
		if cfg := r.eng.Config(); cfg != nil && cfg.ArtifactByName(name) != nil {
			isFeed = false
		}
		status := engine.OperatorStatusMeaning(entry.LastStatus, entry.DownloadFailures, isFeed)
		detail := entry.LastError
		if detail == "" {
			detail = status.Label
		}
		return queueStatusView{
			Status:       entry.LastStatus,
			StatusLabel:  status.Label,
			ProblemClass: status.ProblemClass,
			Detail:       detail,
		}
	}
}

func (r *Runner) MetricsSnapshot() MetricsSnapshot {
	if r == nil {
		return MetricsSnapshot{}
	}
	return r.metrics.snapshot()
}

func (r *Runner) Run(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Go(func() {
		r.runFetchLoop(runCtx, &wg)
	})
	wg.Go(func() {
		r.runProcessingLoop(runCtx)
	})
	wg.Go(func() {
		r.recoverStagedWork(runCtx)
	})
	if len(r.ActivitySnapshot().ProcessingWaiting) > 0 {
		r.wakeProcessLoop()
	}
	if len(r.ActivitySnapshot().DownloadWaiting) > 0 {
		r.wakeDownloadLoop()
	}

	for {
		select {
		case <-runCtx.Done():
			wg.Wait()
			return
		case action := <-r.actionCh:
			r.handleAction(runCtx, action)
		}
	}
}
