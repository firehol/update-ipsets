package engine

import (
	"sort"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"github.com/firehol/update-ipsets/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
)

const maxSlowFeedSnapshots = 12

type RunPhaseTimingSnapshot struct {
	Phase      RunPhase `json:"phase"`
	DurationMS int64    `json:"duration_ms"`
}

type FeedTimingSnapshot struct {
	Name       string                         `json:"name"`
	TotalMS    int64                          `json:"total_ms"`
	Operations []telemetry.TimingStatSnapshot `json:"operations,omitempty"`
}

type RunMetricsSnapshot struct {
	StartedAt  time.Time                      `json:"started_at,omitempty"`
	Current    bool                           `json:"current"`
	PhaseTimes []RunPhaseTimingSnapshot       `json:"phase_times,omitempty"`
	Operations []telemetry.TimingStatSnapshot `json:"operations,omitempty"`
	SlowFeeds  []FeedTimingSnapshot           `json:"slow_feeds,omitempty"`
}

type feedRunMetrics struct {
	total      time.Duration
	operations telemetry.TimingBook
}

type runMetrics struct {
	mu             sync.Mutex
	startedAt      time.Time
	currentPhase   RunPhase
	phaseStartedAt time.Time
	phaseTotals    map[RunPhase]time.Duration
	operations     telemetry.TimingBook
	feeds          map[string]*feedRunMetrics
	completed      bool
}

func newRunMetrics(startedAt time.Time, phase RunPhase) *runMetrics {
	now := time.Now()
	return &runMetrics{
		startedAt:      startedAt,
		currentPhase:   phase,
		phaseStartedAt: now,
		phaseTotals:    make(map[RunPhase]time.Duration),
		feeds:          make(map[string]*feedRunMetrics),
	}
}

func (m *runMetrics) setPhase(phase RunPhase) {
	if m == nil {
		return
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.advancePhaseLocked(now, phase)
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
}

func (m *runMetrics) observeOperationAggregate(name string, count int64, total, max time.Duration) {
	if m == nil || name == "" {
		return
	}
	m.operations.ObserveAggregate(name, count, total, max)
}

func (m *runMetrics) observeFeedOperation(feedName, operation string, dur time.Duration) {
	if m == nil || feedName == "" || operation == "" {
		return
	}
	observability.Duration(observability.BackgroundContext(), operation, dur, attribute.String("feed.name", feedName))
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
	for _, phase := range phaseNames {
		phaseSnapshots = append(phaseSnapshots, RunPhaseTimingSnapshot{
			Phase:      phase,
			DurationMS: telemetryDurationMillis(phaseTotals[phase]),
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

	return RunMetricsSnapshot{
		StartedAt:  startedAt,
		Current:    current,
		PhaseTimes: phaseSnapshots,
		Operations: m.operations.Snapshot(),
		SlowFeeds:  feedSnaps,
	}
}

func (m *runMetrics) advancePhaseLocked(now time.Time, next RunPhase) {
	if m.currentPhase.Valid() && !m.phaseStartedAt.IsZero() {
		dur := now.Sub(m.phaseStartedAt)
		m.phaseTotals[m.currentPhase] += dur
		observability.Duration(observability.BackgroundContext(), "engine.phase", dur, attribute.String("engine.phase", string(m.currentPhase)))
	}
	m.currentPhase = next
	m.phaseStartedAt = now
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
