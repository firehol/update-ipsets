package scheduler

import (
	"context"
	"errors"
	"fmt"
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

const (
	DefaultActionAdmissionTimeout = 5 * time.Second
	LaneActionAdmissionTimeout    = 250 * time.Millisecond
)

var (
	ErrActionQueueUnavailable = errors.New("scheduler action queue unavailable")
	ErrActionQueueSaturated   = errors.New("scheduler action queue saturated")
)

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

	stateMu            sync.RWMutex
	download           downloadLoopState
	downloadEnqueueSeq uint64
	processing         processingLoopState
	metrics            metricsState

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
	_ = r.TriggerSourcesWithin(context.Background(), DefaultActionAdmissionTimeout, action)
}

func (r *Runner) TriggerSourcesWithin(ctx context.Context, timeout time.Duration, action PendingAction) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return r.TriggerSourcesContext(ctx, action)
}

func (r *Runner) TriggerSourcesContext(ctx context.Context, action PendingAction) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.actionCh == nil {
		return ErrActionQueueUnavailable
	}
	select {
	case r.actionCh <- action:
		r.wakeDownloadLoop()
		r.wakeProcessLoop()
		return nil
	case <-ctx.Done():
		err := fmt.Errorf("%w: %w", ErrActionQueueSaturated, ctx.Err())
		r.recordActionAdmissionFailure(err)
		return err
	}
}

func (r *Runner) TryTriggerSources(action PendingAction) bool {
	if r == nil || r.actionCh == nil {
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

// TriggerQueuedAction queues an action only if the scheduler wake-up channel
// accepts it immediately. Used by endpoints that historically returned a
// conflict when a duplicate trigger was already pending.
func (r *Runner) TriggerQueuedAction(action PendingAction) bool {
	if r == nil || r.actionCh == nil {
		return false
	}
	if len(r.actionCh) > 0 {
		return false
	}
	return r.TryTriggerSources(action)
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
	cached := r.CachedSnapshot()
	if r.eng == nil {
		return cached
	}
	if !cached.GeneratedAt.IsZero() && now.Sub(cached.GeneratedAt) <= snapshotReadMaxAge {
		return cached
	}
	cfg, rt, policy := r.eng.ConfigRuntimePolicySnapshot()
	snapshot := BuildSnapshotWithPolicy(
		cfg,
		rt,
		policy,
		r.eng.EntriesSnapshotForConfig(cfg),
		r.enableAll,
		now,
	)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshot = snapshot
	return snapshot
}

// CachedSnapshot returns the last scheduler snapshot without rebuilding it.
//
// High-frequency status paths use this so an HTTP request cannot synchronously
// walk every cache entry just because the normal scheduler snapshot is stale.
func (r *Runner) CachedSnapshot() Snapshot {
	if r == nil {
		return Snapshot{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return Snapshot{
		GeneratedAt: r.snapshot.GeneratedAt,
		Items:       append([]Item(nil), r.snapshot.Items...),
	}
}

func (r *Runner) EnableAll() bool {
	if r == nil {
		return false
	}
	return r.enableAll
}

func (r *Runner) ActivitySnapshot() ActivitySnapshot {
	configSnapshot := r.schedulerConfigSnapshot()
	includeProcessing := func(name string) bool {
		return !configSnapshot.IsProviderDatabase(name)
	}
	lookup := r.queueStatusLookup()
	r.stateMu.RLock()
	downloadWaiting := queueSnapshotFromMap(r.download.waiting, lookup)
	for i := range downloadWaiting {
		if !r.downloadInputsSettledLocked(configSnapshot.DerivedFrom(downloadWaiting[i].Name)) {
			downloadWaiting[i].Blocked = true
			downloadWaiting[i].BlockedParents = configSnapshot.DerivedFrom(downloadWaiting[i].Name)
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

// ActivitySnapshotLight returns queue state without cache-entry status lookups
// or engine active-feed snapshots. It is intentionally limited to scheduler
// owned state so frequent admin polling cannot block on engine/cache work.
func (r *Runner) ActivitySnapshotLight() ActivitySnapshot {
	if r == nil {
		return ActivitySnapshot{}
	}
	configSnapshot := r.schedulerConfigSnapshot()
	includeProcessing := func(name string) bool {
		return !configSnapshot.IsProviderDatabase(name)
	}
	r.stateMu.RLock()
	downloadWaiting := queueSnapshotFromMap(r.download.waiting, nil)
	for i := range downloadWaiting {
		if !r.downloadInputsSettledLocked(configSnapshot.DerivedFrom(downloadWaiting[i].Name)) {
			downloadWaiting[i].Blocked = true
			downloadWaiting[i].BlockedParents = configSnapshot.DerivedFrom(downloadWaiting[i].Name)
		}
	}
	snap := ActivitySnapshot{
		DownloadWaiting:         downloadWaiting,
		DownloadActive:          activeSnapshotFromMap(r.download.active, nil),
		DownloadRefetchPending:  queueSnapshotFromMap(r.download.refetchPending, nil),
		ProcessingWaiting:       queueSnapshotFromMapFiltered(r.processing.waiting, includeProcessing, nil),
		ProcessingActive:        activeSnapshotFromMapFiltered(r.processing.active, includeProcessing, nil),
		ProcessingDeferred:      queueSnapshotFromMapFiltered(r.processing.deferred, includeProcessing, nil),
		RecentHealthTransitions: append([]HealthTransition(nil), r.recentHealthTransitions...),
	}
	r.stateMu.RUnlock()
	return snap
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
	cfg, _ := r.eng.ConfigRuntimeSnapshot()
	configSnapshot := engine.SchedulerConfigSnapshotForConfig(cfg)
	entries := r.eng.EntriesSnapshotForConfig(cfg)
	index := make(map[string]cache.Entry, len(entries))
	for _, entry := range entries {
		index[entry.Name] = entry
	}
	return func(name string) queueStatusView {
		entry, ok := index[name]
		if !ok {
			return queueStatusView{}
		}
		isFeed := !configSnapshot.IsArtifact(name)
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

func (r *Runner) schedulerConfigSnapshot() engine.SchedulerConfigSnapshot {
	if r == nil || r.eng == nil {
		return engine.SchedulerConfigSnapshot{}
	}
	return r.eng.SchedulerConfigSnapshot()
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
	r.recoverStagedWork(runCtx)
	wg.Go(func() {
		r.runRecoverableLoop(runCtx, "fetch_loop", func() {
			r.runFetchLoop(runCtx, &wg)
		})
	})
	wg.Go(func() {
		r.runRecoverableLoop(runCtx, "processing_loop", func() {
			r.runProcessingLoop(runCtx)
		})
	})
	if r.hasProcessingQueueWork() {
		r.wakeProcessLoop()
	}
	if r.hasDownloadQueueWork() {
		r.wakeDownloadLoop()
	}

	for {
		select {
		case <-runCtx.Done():
			wg.Wait()
			return
		case action := <-r.actionCh:
			r.handleActionRecovered(runCtx, action)
		}
	}
}

func (r *Runner) hasProcessingQueueWork() bool {
	if r == nil {
		return false
	}
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()
	return len(r.processing.waiting) > 0 || len(r.processing.deferred) > 0
}

func (r *Runner) hasDownloadQueueWork() bool {
	if r == nil {
		return false
	}
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()
	return len(r.download.waiting) > 0 || len(r.download.refetchPending) > 0
}
