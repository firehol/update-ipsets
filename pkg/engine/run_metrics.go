package engine

import (
	"sort"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"github.com/firehol/update-ipsets/internal/telemetry"
)

const maxSlowFeedSnapshots = 12

type WorkRateSnapshot struct {
	Unit          string  `json:"unit"`
	Completed     int64   `json:"completed"`
	Total         int64   `json:"total"`
	CompletionPct int     `json:"completion_pct"`
	RatePerSecond float64 `json:"rate_per_second"`
	ElapsedMS     int64   `json:"elapsed_ms"`
}

type RunPhaseTimingSnapshot struct {
	Phase      RunPhase `json:"phase"`
	DurationMS int64    `json:"duration_ms"`
}

type RunPhaseMetricsSnapshot struct {
	Phase      RunPhase                        `json:"phase"`
	DurationMS int64                           `json:"duration_ms"`
	Work       WorkRateSnapshot                `json:"work"`
	Operations []telemetry.TimingStatSnapshot  `json:"operations,omitempty"`
	Counters   []telemetry.CounterStatSnapshot `json:"counters,omitempty"`
}

type FeedTimingSnapshot struct {
	Name       string                         `json:"name"`
	TotalMS    int64                          `json:"total_ms"`
	Operations []telemetry.TimingStatSnapshot `json:"operations,omitempty"`
}

type FeedWorkSnapshot struct {
	Name                string                         `json:"name"`
	Status              string                         `json:"status"`
	Processed           bool                           `json:"processed"`
	InputBytes          int64                          `json:"input_bytes"`
	Entries             int64                          `json:"entries"`
	UniqueIPs           int64                          `json:"unique_ips"`
	ElapsedMS           int64                          `json:"elapsed_ms"`
	InputBytesPerSecond float64                        `json:"input_bytes_per_second"`
	EntriesPerSecond    float64                        `json:"entries_per_second"`
	UniqueIPsPerSecond  float64                        `json:"unique_ips_per_second"`
	Operations          []telemetry.TimingStatSnapshot `json:"operations,omitempty"`
}

type RunMetricsSnapshot struct {
	StartedAt  time.Time                       `json:"started_at,omitempty"`
	Current    bool                            `json:"current"`
	PhaseTimes []RunPhaseTimingSnapshot        `json:"phase_times,omitempty"`
	Phases     []RunPhaseMetricsSnapshot       `json:"phases,omitempty"`
	Feeds      []FeedWorkSnapshot              `json:"feeds,omitempty"`
	Operations []telemetry.TimingStatSnapshot  `json:"operations,omitempty"`
	Counters   []telemetry.CounterStatSnapshot `json:"counters,omitempty"`
	SlowFeeds  []FeedTimingSnapshot            `json:"slow_feeds,omitempty"`
}

type feedRunMetrics struct {
	total      time.Duration
	operations telemetry.TimingBook
}

type runMetrics struct {
	mu              sync.Mutex
	startedAt       time.Time
	currentPhase    RunPhase
	phaseStartedAt  time.Time
	phaseTotals     map[RunPhase]time.Duration
	operations      telemetry.TimingBook
	counters        telemetry.CounterBook
	phaseOperations map[RunPhase]*telemetry.TimingBook
	phaseCounters   map[RunPhase]*telemetry.CounterBook
	feeds           map[string]*feedRunMetrics
	feedWork        map[string]FeedWorkSnapshot
	completed       bool
}

func newRunMetrics(startedAt time.Time, phase RunPhase) *runMetrics {
	now := time.Now()
	return &runMetrics{
		startedAt:       startedAt,
		currentPhase:    phase,
		phaseStartedAt:  now,
		phaseTotals:     make(map[RunPhase]time.Duration),
		phaseOperations: make(map[RunPhase]*telemetry.TimingBook),
		phaseCounters:   make(map[RunPhase]*telemetry.CounterBook),
		feeds:           make(map[string]*feedRunMetrics),
		feedWork:        make(map[string]FeedWorkSnapshot),
	}
}

func (m *runMetrics) setPhase(phase RunPhase) (RunPhaseTimingSnapshot, bool) {
	if m == nil {
		return RunPhaseTimingSnapshot{}, false
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.advancePhaseLocked(now, phase)
}

func (m *runMetrics) finish() {
	if m == nil {
		return
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.completed {
		return
	}
	m.advancePhaseLocked(now, RunPhaseUnknown)
	m.completed = true
}

func (m *runMetrics) observeOperation(name string, dur time.Duration) {
	if m == nil || name == "" {
		return
	}
	m.operations.Observe(name, dur)
	m.mu.Lock()
	phaseBook := m.phaseOperationBookLocked(m.currentPhase)
	m.mu.Unlock()
	if phaseBook != nil {
		phaseBook.Observe(name, dur)
	}
}

func (m *runMetrics) tryObserveOperation(name string, dur time.Duration) {
	if m == nil || name == "" {
		return
	}
	m.operations.TryObserve(name, dur)
	if !m.mu.TryLock() {
		return
	}
	phaseBook := m.phaseOperationBookLocked(m.currentPhase)
	m.mu.Unlock()
	if phaseBook != nil {
		phaseBook.TryObserve(name, dur)
	}
}

func (m *runMetrics) observeOperationAggregate(name string, count int64, total, max time.Duration) {
	if m == nil || name == "" {
		return
	}
	m.operations.ObserveAggregate(name, count, total, max)
	m.mu.Lock()
	phaseBook := m.phaseOperationBookLocked(m.currentPhase)
	m.mu.Unlock()
	if phaseBook != nil {
		phaseBook.ObserveAggregate(name, count, total, max)
	}
}

func (m *runMetrics) observeCounter(name string, count, bytes int64) {
	if m == nil || name == "" {
		return
	}
	m.counters.Add(name, count, bytes)
	m.mu.Lock()
	phaseBook := m.phaseCounterBookLocked(m.currentPhase)
	m.mu.Unlock()
	if phaseBook != nil {
		phaseBook.Add(name, count, bytes)
	}
}

func (m *runMetrics) tryObserveCounter(name string, count, bytes int64) {
	if m == nil || name == "" {
		return
	}
	m.counters.TryAdd(name, count, bytes)
	if !m.mu.TryLock() {
		return
	}
	phaseBook := m.phaseCounterBookLocked(m.currentPhase)
	m.mu.Unlock()
	if phaseBook != nil {
		phaseBook.TryAdd(name, count, bytes)
	}
}

func (m *runMetrics) observeFeedOperation(feedName, operation string, dur time.Duration) {
	if m == nil || feedName == "" || operation == "" {
		return
	}
	m.mu.Lock()
	feed := m.feeds[feedName]
	if feed == nil {
		feed = &feedRunMetrics{}
		m.feeds[feedName] = feed
	}
	feed.total += dur
	m.mu.Unlock()
	feed.operations.Observe(operation, dur)
}

func (m *runMetrics) observeFeedWork(feedName string, result FeedProcessingResult, elapsed time.Duration) {
	if m == nil || feedName == "" {
		return
	}
	elapsedMS := telemetryDurationMillis(elapsed)
	work := result.Work
	snap := FeedWorkSnapshot{
		Name:                feedName,
		Status:              result.StatusString(),
		Processed:           result.Processed,
		InputBytes:          work.InputBytes,
		Entries:             work.Entries,
		UniqueIPs:           work.UniqueIPs,
		ElapsedMS:           elapsedMS,
		InputBytesPerSecond: ratePerSecond(work.InputBytes, elapsedMS),
		EntriesPerSecond:    ratePerSecond(work.Entries, elapsedMS),
		UniqueIPsPerSecond:  ratePerSecond(work.UniqueIPs, elapsedMS),
	}
	m.mu.Lock()
	feed := m.feeds[feedName]
	if feed != nil {
		snap.Operations = feed.operations.Snapshot()
	}
	m.feedWork[feedName] = snap
	m.mu.Unlock()
}

func (m *runMetrics) snapshot(current bool) RunMetricsSnapshot {
	if m == nil {
		return RunMetricsSnapshot{}
	}
	now := time.Now()
	m.mu.Lock()
	startedAt := m.startedAt
	phaseTotals := make(map[RunPhase]time.Duration, len(m.phaseTotals)+1)
	for phase, total := range m.phaseTotals {
		phaseTotals[phase] = total
	}
	currentPhase := m.currentPhase
	phaseStartedAt := m.phaseStartedAt
	feedSnaps := make([]FeedTimingSnapshot, 0, len(m.feeds))
	for name, feed := range m.feeds {
		if feed == nil {
			continue
		}
		feedSnaps = append(feedSnaps, FeedTimingSnapshot{
			Name:       name,
			TotalMS:    telemetryDurationMillis(feed.total),
			Operations: feed.operations.Snapshot(),
		})
	}
	feedWork := make([]FeedWorkSnapshot, 0, len(m.feedWork))
	for _, snap := range m.feedWork {
		feedWork = append(feedWork, snap)
	}
	phaseOperations := make(map[RunPhase][]telemetry.TimingStatSnapshot, len(m.phaseOperations))
	for phase, book := range m.phaseOperations {
		if book != nil {
			phaseOperations[phase] = book.Snapshot()
		}
	}
	phaseCounters := make(map[RunPhase][]telemetry.CounterStatSnapshot, len(m.phaseCounters))
	for phase, book := range m.phaseCounters {
		if book != nil {
			phaseCounters[phase] = book.Snapshot()
		}
	}
	m.mu.Unlock()

	if current && currentPhase.Valid() && !phaseStartedAt.IsZero() {
		phaseTotals[currentPhase] += now.Sub(phaseStartedAt)
	}

	phaseNames := make([]RunPhase, 0, len(phaseTotals))
	for phase := range phaseTotals {
		if !phase.Valid() {
			continue
		}
		phaseNames = append(phaseNames, phase)
	}
	sort.Slice(phaseNames, func(i, j int) bool {
		return phaseNames[i] < phaseNames[j]
	})
	phaseSnapshots := make([]RunPhaseTimingSnapshot, 0, len(phaseNames))
	phaseMetrics := make([]RunPhaseMetricsSnapshot, 0, len(phaseNames))
	for _, phase := range phaseNames {
		durationMS := telemetryDurationMillis(phaseTotals[phase])
		phaseSnapshots = append(phaseSnapshots, RunPhaseTimingSnapshot{
			Phase:      phase,
			DurationMS: durationMS,
		})
		phaseMetrics = append(phaseMetrics, RunPhaseMetricsSnapshot{
			Phase:      phase,
			DurationMS: durationMS,
			Work:       phaseWorkSnapshot(phase, durationMS, phaseOperations[phase], phaseCounters[phase]),
			Operations: phaseOperations[phase],
			Counters:   phaseCounters[phase],
		})
	}

	sort.Slice(feedSnaps, func(i, j int) bool {
		if feedSnaps[i].TotalMS != feedSnaps[j].TotalMS {
			return feedSnaps[i].TotalMS > feedSnaps[j].TotalMS
		}
		return feedSnaps[i].Name < feedSnaps[j].Name
	})
	if len(feedSnaps) > maxSlowFeedSnapshots {
		feedSnaps = feedSnaps[:maxSlowFeedSnapshots]
	}
	sort.Slice(feedWork, func(i, j int) bool {
		return feedWork[i].Name < feedWork[j].Name
	})

	return RunMetricsSnapshot{
		StartedAt:  startedAt,
		Current:    current,
		PhaseTimes: phaseSnapshots,
		Phases:     phaseMetrics,
		Feeds:      feedWork,
		Operations: m.operations.Snapshot(),
		Counters:   m.counters.Snapshot(),
		SlowFeeds:  feedSnaps,
	}
}

func (m *runMetrics) trySnapshot(current bool) (RunMetricsSnapshot, bool) {
	if m == nil {
		return RunMetricsSnapshot{}, true
	}
	now := time.Now()
	if !m.mu.TryLock() {
		return RunMetricsSnapshot{}, false
	}
	startedAt := m.startedAt
	phaseTotals := make(map[RunPhase]time.Duration, len(m.phaseTotals)+1)
	for phase, total := range m.phaseTotals {
		phaseTotals[phase] = total
	}
	currentPhase := m.currentPhase
	phaseStartedAt := m.phaseStartedAt
	feedSnaps := make([]FeedTimingSnapshot, 0, len(m.feeds))
	feedBooks := make(map[string]*telemetry.TimingBook, len(m.feeds))
	for name, feed := range m.feeds {
		if feed == nil {
			continue
		}
		feedSnaps = append(feedSnaps, FeedTimingSnapshot{
			Name:    name,
			TotalMS: telemetryDurationMillis(feed.total),
		})
		feedBooks[name] = &feed.operations
	}
	feedWork := make([]FeedWorkSnapshot, 0, len(m.feedWork))
	for _, snap := range m.feedWork {
		feedWork = append(feedWork, snap)
	}
	phaseOperationBooks := make(map[RunPhase]*telemetry.TimingBook, len(m.phaseOperations))
	for phase, book := range m.phaseOperations {
		if book != nil {
			phaseOperationBooks[phase] = book
		}
	}
	phaseCounterBooks := make(map[RunPhase]*telemetry.CounterBook, len(m.phaseCounters))
	for phase, book := range m.phaseCounters {
		if book != nil {
			phaseCounterBooks[phase] = book
		}
	}
	operationBook := &m.operations
	counterBook := &m.counters
	m.mu.Unlock()

	for i := range feedSnaps {
		if book := feedBooks[feedSnaps[i].Name]; book != nil {
			if ops, ok := book.TrySnapshot(); ok {
				feedSnaps[i].Operations = ops
			}
		}
	}
	phaseOperations := make(map[RunPhase][]telemetry.TimingStatSnapshot, len(phaseOperationBooks))
	for phase, book := range phaseOperationBooks {
		if ops, ok := book.TrySnapshot(); ok {
			phaseOperations[phase] = ops
		}
	}
	phaseCounters := make(map[RunPhase][]telemetry.CounterStatSnapshot, len(phaseCounterBooks))
	for phase, book := range phaseCounterBooks {
		if counters, ok := book.TrySnapshot(); ok {
			phaseCounters[phase] = counters
		}
	}

	if current && currentPhase.Valid() && !phaseStartedAt.IsZero() {
		phaseTotals[currentPhase] += now.Sub(phaseStartedAt)
	}

	phaseNames := make([]RunPhase, 0, len(phaseTotals))
	for phase := range phaseTotals {
		if !phase.Valid() {
			continue
		}
		phaseNames = append(phaseNames, phase)
	}
	sort.Slice(phaseNames, func(i, j int) bool {
		return phaseNames[i] < phaseNames[j]
	})
	phaseSnapshots := make([]RunPhaseTimingSnapshot, 0, len(phaseNames))
	phaseMetrics := make([]RunPhaseMetricsSnapshot, 0, len(phaseNames))
	for _, phase := range phaseNames {
		durationMS := telemetryDurationMillis(phaseTotals[phase])
		phaseSnapshots = append(phaseSnapshots, RunPhaseTimingSnapshot{
			Phase:      phase,
			DurationMS: durationMS,
		})
		phaseMetrics = append(phaseMetrics, RunPhaseMetricsSnapshot{
			Phase:      phase,
			DurationMS: durationMS,
			Work:       phaseWorkSnapshot(phase, durationMS, phaseOperations[phase], phaseCounters[phase]),
			Operations: phaseOperations[phase],
			Counters:   phaseCounters[phase],
		})
	}

	sort.Slice(feedSnaps, func(i, j int) bool {
		if feedSnaps[i].TotalMS != feedSnaps[j].TotalMS {
			return feedSnaps[i].TotalMS > feedSnaps[j].TotalMS
		}
		return feedSnaps[i].Name < feedSnaps[j].Name
	})
	if len(feedSnaps) > maxSlowFeedSnapshots {
		feedSnaps = feedSnaps[:maxSlowFeedSnapshots]
	}
	sort.Slice(feedWork, func(i, j int) bool {
		return feedWork[i].Name < feedWork[j].Name
	})

	operations, _ := operationBook.TrySnapshot()
	counters, _ := counterBook.TrySnapshot()
	return RunMetricsSnapshot{
		StartedAt:  startedAt,
		Current:    current,
		PhaseTimes: phaseSnapshots,
		Phases:     phaseMetrics,
		Feeds:      feedWork,
		Operations: operations,
		Counters:   counters,
		SlowFeeds:  feedSnaps,
	}, true
}

func (m *runMetrics) phaseSnapshot(phase RunPhase, durationMS int64) RunPhaseMetricsSnapshot {
	if m == nil || !phase.Valid() {
		return RunPhaseMetricsSnapshot{}
	}
	m.mu.Lock()
	ops := snapshotTimingBook(m.phaseOperations[phase])
	counters := snapshotCounterBook(m.phaseCounters[phase])
	m.mu.Unlock()
	return RunPhaseMetricsSnapshot{
		Phase:      phase,
		DurationMS: durationMS,
		Work:       phaseWorkSnapshot(phase, durationMS, ops, counters),
		Operations: ops,
		Counters:   counters,
	}
}

func (m *runMetrics) feedSnapshot(name string) (FeedTimingSnapshot, bool) {
	if m == nil || name == "" {
		return FeedTimingSnapshot{}, false
	}
	m.mu.Lock()
	feed := m.feeds[name]
	if feed == nil {
		m.mu.Unlock()
		return FeedTimingSnapshot{}, false
	}
	total := feed.total
	operations := feed.operations.Snapshot()
	m.mu.Unlock()
	return FeedTimingSnapshot{
		Name:       name,
		TotalMS:    telemetryDurationMillis(total),
		Operations: operations,
	}, true
}

func (m *runMetrics) phaseOperationBookLocked(phase RunPhase) *telemetry.TimingBook {
	if !phase.Valid() {
		return nil
	}
	book := m.phaseOperations[phase]
	if book == nil {
		book = &telemetry.TimingBook{}
		m.phaseOperations[phase] = book
	}
	return book
}

func (m *runMetrics) phaseCounterBookLocked(phase RunPhase) *telemetry.CounterBook {
	if !phase.Valid() {
		return nil
	}
	book := m.phaseCounters[phase]
	if book == nil {
		book = &telemetry.CounterBook{}
		m.phaseCounters[phase] = book
	}
	return book
}

func snapshotTimingBook(book *telemetry.TimingBook) []telemetry.TimingStatSnapshot {
	if book == nil {
		return nil
	}
	return book.Snapshot()
}

func snapshotCounterBook(book *telemetry.CounterBook) []telemetry.CounterStatSnapshot {
	if book == nil {
		return nil
	}
	return book.Snapshot()
}

func phaseWorkSnapshot(phase RunPhase, durationMS int64, operations []telemetry.TimingStatSnapshot, counters []telemetry.CounterStatSnapshot) WorkRateSnapshot {
	unit := "operations"
	completed := operationCount(operations)
	total := completed
	if phase == RunPhaseSources {
		unit = "feeds"
		completed = counterCount(counters, "sources.feeds_processed")
		total = counterCount(counters, "sources.feeds_expected")
		if total < completed {
			total = completed
		}
	}
	pct := 0
	if total > 0 {
		pct = completionPct(completed, total)
	}
	return WorkRateSnapshot{
		Unit:          unit,
		Completed:     completed,
		Total:         total,
		CompletionPct: pct,
		RatePerSecond: ratePerSecond(completed, durationMS),
		ElapsedMS:     durationMS,
	}
}

func operationCount(operations []telemetry.TimingStatSnapshot) int64 {
	var total int64
	for _, op := range operations {
		total += op.Count
	}
	return total
}

func counterCount(counters []telemetry.CounterStatSnapshot, name string) int64 {
	for _, counter := range counters {
		if counter.Name == name {
			return counter.Count
		}
	}
	return 0
}

func (m *runMetrics) advancePhaseLocked(now time.Time, next RunPhase) (RunPhaseTimingSnapshot, bool) {
	var completed RunPhaseTimingSnapshot
	ok := false
	if m.currentPhase.Valid() && !m.phaseStartedAt.IsZero() {
		dur := now.Sub(m.phaseStartedAt)
		m.phaseTotals[m.currentPhase] += dur
		completed = RunPhaseTimingSnapshot{
			Phase:      m.currentPhase,
			DurationMS: telemetryDurationMillis(dur),
		}
		ok = true
		observability.TryDuration("engine.phase", dur, observability.String("engine.phase", string(m.currentPhase)))
	}
	m.currentPhase = next
	m.phaseStartedAt = now
	return completed, ok
}

func telemetryDurationMillis(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	ms := d.Milliseconds()
	if ms == 0 {
		return 1
	}
	return ms
}
