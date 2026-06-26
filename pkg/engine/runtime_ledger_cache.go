package engine

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/firehol/update-ipsets/pkg/cache"
)

type historyLedgerStats struct {
	firstTS       int64
	lastTS        int64
	count         int
	totalGapSecs  int64
	minGapSecs    int64
	maxGapSecs    int64
	entriesMin    int
	entriesMax    int
	ipsMin        uint64
	ipsMax        uint64
	lastEntries   int
	lastUniqueIPs uint64
}

type runtimeLedgerSnapshot struct {
	libDir           string
	webChartsEntries int
	ledger           *runtimeLedgerCache
}

func (e *Engine) runtimeLedgerSnapshot() runtimeLedgerSnapshot {
	if e == nil {
		return runtimeLedgerSnapshot{}
	}
	e.mu.RLock()
	rt := e.runtime
	ledger := e.ledgerCache
	e.mu.RUnlock()
	return runtimeLedgerSnapshot{
		libDir:           rt.LibDir,
		webChartsEntries: webChartsEntriesFromRuntime(rt),
		ledger:           ledger,
	}
}

func (s *historyLedgerStats) observe(point HistoryPoint) {
	if !validHistoryTimestamp(point.Timestamp) {
		return
	}
	if s.count == 0 {
		s.firstTS = point.Timestamp
		s.lastTS = point.Timestamp
		s.count = 1
		s.entriesMin = point.Entries
		s.entriesMax = point.Entries
		s.ipsMin = point.UniqueIPs
		s.ipsMax = point.UniqueIPs
		s.lastEntries = point.Entries
		s.lastUniqueIPs = point.UniqueIPs
		return
	}
	if point.Timestamp <= s.lastTS {
		return
	}
	gap := point.Timestamp - s.lastTS
	s.lastTS = point.Timestamp
	s.count++
	s.totalGapSecs += gap
	if s.minGapSecs == 0 || gap < s.minGapSecs {
		s.minGapSecs = gap
	}
	if gap > s.maxGapSecs {
		s.maxGapSecs = gap
	}
	if point.Entries < s.entriesMin {
		s.entriesMin = point.Entries
	}
	if point.Entries > s.entriesMax {
		s.entriesMax = point.Entries
	}
	if point.UniqueIPs < s.ipsMin {
		s.ipsMin = point.UniqueIPs
	}
	if point.UniqueIPs > s.ipsMax {
		s.ipsMax = point.UniqueIPs
	}
	s.lastEntries = point.Entries
	s.lastUniqueIPs = point.UniqueIPs
}

func (s historyLedgerStats) apply(entry *cache.Entry, frequency int) bool {
	if entry == nil || s.count == 0 {
		return false
	}
	snapshot := cache.HistoryLedgerStatsSnapshot{
		Version:             s.count,
		StartedUnix:         s.firstTS,
		Entries:             s.lastEntries,
		UniqueIPs:           s.lastUniqueIPs,
		EntriesMin:          s.entriesMin,
		EntriesMax:          s.entriesMax,
		IPsMin:              s.ipsMin,
		IPsMax:              s.ipsMax,
		HistoryTotalGapSecs: s.totalGapSecs,
		HistoryMinGapSecs:   s.minGapSecs,
		HistoryMaxGapSecs:   s.maxGapSecs,
	}
	if s.count <= 1 {
		snapshot.HistoryTotalGapSecs = 0
		snapshot.HistoryMinGapSecs = 0
		snapshot.HistoryMaxGapSecs = 0
		if frequency > 0 {
			snapshot.AverageUpdateMinutes = frequency
			snapshot.MinUpdateMinutes = frequency
			snapshot.MaxUpdateMinutes = frequency
		}
		return entry.ApplyHistoryLedgerStats(snapshot)
	}
	snapshot.AverageUpdateMinutes = roundSecondsToMinutes(s.totalGapSecs / int64(s.count-1))
	snapshot.MinUpdateMinutes = roundSecondsToMinutes(s.minGapSecs)
	snapshot.MaxUpdateMinutes = roundSecondsToMinutes(s.maxGapSecs)
	return entry.ApplyHistoryLedgerStats(snapshot)
}

func historyLedgerStatsFromEntry(entry *cache.Entry) (historyLedgerStats, bool) {
	if entry == nil || entry.Version <= 0 || entry.StartedDate <= 0 || entry.SourceDate <= 0 {
		return historyLedgerStats{}, false
	}

	stats := historyLedgerStats{
		firstTS:       entry.StartedDate,
		lastTS:        entry.SourceDate,
		count:         entry.Version,
		totalGapSecs:  entry.HistoryTotalGapSecs,
		minGapSecs:    entry.HistoryMinGapSecs,
		maxGapSecs:    entry.HistoryMaxGapSecs,
		entriesMin:    entry.EntriesMin,
		entriesMax:    entry.EntriesMax,
		ipsMin:        entry.IPsMin,
		ipsMax:        entry.IPsMax,
		lastEntries:   entry.Entries,
		lastUniqueIPs: entry.UniqueIPs,
	}
	if stats.count == 1 {
		stats.totalGapSecs = 0
		stats.minGapSecs = 0
		stats.maxGapSecs = 0
		return stats, true
	}
	if stats.totalGapSecs <= 0 || stats.minGapSecs <= 0 || stats.maxGapSecs <= 0 {
		if entry.AverageUpdateMins <= 0 || entry.MinUpdateMins <= 0 || entry.MaxUpdateMins <= 0 {
			return historyLedgerStats{}, false
		}
		stats.totalGapSecs = int64(entry.AverageUpdateMins) * 60 * int64(stats.count-1)
		stats.minGapSecs = int64(entry.MinUpdateMins) * 60
		stats.maxGapSecs = int64(entry.MaxUpdateMins) * 60
	}
	return stats, true
}

type feedLedgerState struct {
	mu sync.Mutex

	historyLoaded bool
	historyStats  historyLedgerStats
	historyTail   []HistoryPoint
	historyLimit  int

	changesLoaded bool
	changesTail   []ChangesetPoint
	changesLimit  int

	retentionLoaded  bool
	retentionStarted int64
	retentionPast    map[int]uint64

	cohortsLoaded bool
	cohorts       map[int64]uint64
}

type runtimeLedgerCache struct {
	mu    sync.Mutex
	feeds map[string]*feedLedgerState
}

func newRuntimeLedgerCache() *runtimeLedgerCache {
	return &runtimeLedgerCache{feeds: make(map[string]*feedLedgerState)}
}

func (c *runtimeLedgerCache) feed(name string) *feedLedgerState {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if st := c.feeds[name]; st != nil {
		return st
	}
	st := &feedLedgerState{}
	c.feeds[name] = st
	return st
}

func (e *Engine) ensureHistoryLedgerLoaded(ctx context.Context, name string, st *feedLedgerState, limit int, libDir string, baseline *cache.Entry) bool {
	ctx = nonNilContext(ctx)
	st.mu.Lock()
	if st.historyLoaded && st.historyLimit >= limit {
		st.mu.Unlock()
		return true
	}
	st.mu.Unlock()

	var (
		stats historyLedgerStats
		tail  []HistoryPoint
		err   error
	)
	if baselineStats, ok := historyLedgerStatsFromEntry(baseline); ok {
		stats = baselineStats
		tail = e.historyTailBootstrap(name)
	} else {
		stats, tail, err = loadHistoryLedgerStateContext(ctx, filepath.Join(libDir, name, "history.csv"), name, limit)
		if err != nil {
			return false
		}
	}

	st.mu.Lock()
	if !st.historyLoaded || st.historyLimit < limit {
		st.historyLoaded = true
		st.historyStats = stats
		st.historyTail = tail
		st.historyLimit = limit
	}
	st.mu.Unlock()
	return true
}

func (e *Engine) historyStatsFromRuntime(name string, entry *cache.Entry, frequency int) bool {
	return e.historyStatsFromRuntimeContext(context.Background(), name, entry, frequency)
}

func (e *Engine) historyStatsFromRuntimeContext(ctx context.Context, name string, entry *cache.Entry, frequency int) bool {
	ctx = nonNilContext(ctx)
	snap := e.runtimeLedgerSnapshot()
	if entry == nil || snap.libDir == "" || snap.ledger == nil {
		return false
	}
	st := snap.ledger.feed(name)
	if st == nil {
		return false
	}
	if !e.ensureHistoryLedgerLoaded(ctx, name, st, snap.webChartsEntries, snap.libDir, nil) {
		return false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.historyStats.apply(entry, frequency)
}

func (e *Engine) observeHistoryPoint(name string, point HistoryPoint, entry, baseline *cache.Entry, frequency int) bool {
	return e.observeHistoryPointContext(context.Background(), name, point, entry, baseline, frequency)
}

func (e *Engine) observeHistoryPointContext(ctx context.Context, name string, point HistoryPoint, entry, baseline *cache.Entry, frequency int) bool {
	ctx = nonNilContext(ctx)
	snap := e.runtimeLedgerSnapshot()
	if entry == nil || snap.libDir == "" || snap.ledger == nil {
		return false
	}
	st := snap.ledger.feed(name)
	if st == nil {
		return false
	}
	limit := snap.webChartsEntries
	if !e.ensureHistoryLedgerLoaded(ctx, name, st, limit, snap.libDir, baseline) {
		return false
	}

	st.mu.Lock()
	if point.Timestamp <= st.historyStats.lastTS {
		if point.Timestamp == st.historyStats.lastTS {
			if point.Entries == st.historyStats.lastEntries && point.UniqueIPs == st.historyStats.lastUniqueIPs {
				st.historyTail = appendObservedHistoryTail(st.historyTail, point, limit)
				st.historyLimit = limit
				e.observeRunCounter("sources.finalize.observe_history_same_timestamp_noop", 1, 0)
				applied := st.historyStats.apply(entry, frequency)
				st.mu.Unlock()
				return applied
			}
			expectedLastTS := st.historyStats.lastTS
			expectedLastEntries := st.historyStats.lastEntries
			expectedLastIPs := st.historyStats.lastUniqueIPs
			st.mu.Unlock()

			stats, tail, err := loadHistoryLedgerStateContext(ctx, filepath.Join(snap.libDir, name, "history.csv"), name, limit)
			st.mu.Lock()
			if err == nil && st.historyStats.lastTS == expectedLastTS && st.historyStats.lastEntries == expectedLastEntries && st.historyStats.lastUniqueIPs == expectedLastIPs {
				st.historyLoaded = true
				st.historyStats = stats
				st.historyTail = tail
				st.historyLimit = limit
			}
			applied := st.historyStats.apply(entry, frequency)
			st.mu.Unlock()
			return applied
		}
		applied := st.historyStats.apply(entry, frequency)
		st.mu.Unlock()
		return applied
	}
	st.historyStats.observe(point)
	st.historyTail = appendObservedHistoryTail(st.historyTail, point, limit)
	st.historyLimit = limit
	applied := st.historyStats.apply(entry, frequency)
	st.mu.Unlock()
	return applied
}

func (e *Engine) historyTailFromRuntime(name string) []HistoryPoint {
	return e.historyTailFromRuntimeContext(context.Background(), name)
}

func (e *Engine) historyTailFromRuntimeContext(ctx context.Context, name string) []HistoryPoint {
	ctx = nonNilContext(ctx)
	snap := e.runtimeLedgerSnapshot()
	if snap.libDir == "" || snap.ledger == nil {
		return nil
	}
	st := snap.ledger.feed(name)
	if st == nil {
		return nil
	}
	if !e.ensureHistoryLedgerLoaded(ctx, name, st, snap.webChartsEntries, snap.libDir, nil) {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return append([]HistoryPoint(nil), st.historyTail...)
}

func (e *Engine) ensureChangesetTailLoaded(ctx context.Context, name string, st *feedLedgerState, limit int, libDir string) (bool, bool) {
	ctx = nonNilContext(ctx)
	st.mu.Lock()
	loaded := st.changesLoaded && st.changesLimit >= limit
	st.mu.Unlock()
	if loaded {
		return true, true
	}

	points, err := loadChangesetTailContext(ctx, filepath.Join(libDir, name, "changesets.csv"), limit)
	if err != nil {
		return false, false
	}
	st.mu.Lock()
	if !st.changesLoaded || st.changesLimit < limit {
		st.changesLoaded = true
		st.changesTail = points
		st.changesLimit = limit
	}
	st.mu.Unlock()
	return false, true
}

func (e *Engine) changesetTailFromRuntime(name string) []ChangesetPoint {
	return e.changesetTailFromRuntimeContext(context.Background(), name)
}

func (e *Engine) changesetTailFromRuntimeContext(ctx context.Context, name string) []ChangesetPoint {
	ctx = nonNilContext(ctx)
	snap := e.runtimeLedgerSnapshot()
	if snap.libDir == "" || snap.ledger == nil {
		return nil
	}
	st := snap.ledger.feed(name)
	if st == nil {
		return nil
	}
	if _, ok := e.ensureChangesetTailLoaded(ctx, name, st, snap.webChartsEntries, snap.libDir); !ok {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return append([]ChangesetPoint(nil), st.changesTail...)
}

func (e *Engine) observeChangesetPoint(name string, point ChangesetPoint) {
	snap := e.runtimeLedgerSnapshot()
	if snap.libDir == "" || snap.ledger == nil || (point.Added == 0 && point.Removed == 0) {
		return
	}
	st := snap.ledger.feed(name)
	if st == nil {
		return
	}
	loadedBefore, ok := e.ensureChangesetTailLoaded(context.Background(), name, st, snap.webChartsEntries, snap.libDir)
	if !ok || !loadedBefore {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.changesTail = appendChangesTail(st.changesTail, point, snap.webChartsEntries)
	st.changesLimit = snap.webChartsEntries
}

func (e *Engine) ensureRetentionPastLoaded(ctx context.Context, name string, st *feedLedgerState, started int64, libDir string) (bool, bool) {
	ctx = nonNilContext(ctx)
	st.mu.Lock()
	loaded := st.retentionLoaded && st.retentionStarted == started
	st.mu.Unlock()
	if loaded {
		return true, true
	}

	past, err := loadRetentionPastContext(ctx, filepath.Join(libDir, name, "retention.csv"), started)
	if err != nil {
		return false, false
	}
	st.mu.Lock()
	if !st.retentionLoaded || st.retentionStarted != started {
		st.retentionLoaded = true
		st.retentionStarted = started
		st.retentionPast = past
	}
	st.mu.Unlock()
	return false, true
}

func (e *Engine) retentionPastFromRuntime(name string, started int64) map[int]uint64 {
	return e.retentionPastFromRuntimeContext(context.Background(), name, started)
}

func (e *Engine) retentionPastFromRuntimeContext(ctx context.Context, name string, started int64) map[int]uint64 {
	ctx = nonNilContext(ctx)
	snap := e.runtimeLedgerSnapshot()
	if snap.libDir == "" || snap.ledger == nil {
		return map[int]uint64{}
	}
	st := snap.ledger.feed(name)
	if st == nil {
		return map[int]uint64{}
	}
	if _, ok := e.ensureRetentionPastLoaded(ctx, name, st, started, snap.libDir); !ok {
		return map[int]uint64{}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return copyRetentionPast(st.retentionPast)
}

func (e *Engine) observeRetentionPast(name string, started int64, hours int, ips uint64) {
	snap := e.runtimeLedgerSnapshot()
	if snap.libDir == "" || snap.ledger == nil || ips == 0 {
		return
	}
	st := snap.ledger.feed(name)
	if st == nil {
		return
	}
	loadedBefore, ok := e.ensureRetentionPastLoaded(context.Background(), name, st, started, snap.libDir)
	if !ok || !loadedBefore {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.retentionPast == nil {
		st.retentionPast = make(map[int]uint64)
	}
	st.retentionPast[hours] += ips
}

func (e *Engine) retentionCohortsFromRuntime(ctx context.Context, name string) map[int64]uint64 {
	ctx = nonNilContext(ctx)
	snap := e.runtimeLedgerSnapshot()
	if snap.libDir == "" || snap.ledger == nil {
		return map[int64]uint64{}
	}
	st := snap.ledger.feed(name)
	if st == nil {
		return map[int64]uint64{}
	}

	st.mu.Lock()
	loaded := st.cohortsLoaded
	st.mu.Unlock()
	if !loaded {
		cohorts, err := loadRetentionCohorts(ctx, filepath.Join(snap.libDir, name))
		if err != nil {
			return map[int64]uint64{}
		}
		st.mu.Lock()
		if !st.cohortsLoaded {
			st.cohortsLoaded = true
			st.cohorts = cohorts
		}
		st.mu.Unlock()
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	out := make(map[int64]uint64, len(st.cohorts))
	for addedAt, ips := range st.cohorts {
		out[addedAt] = ips
	}
	return out
}

func (e *Engine) observeRetentionCohort(name string, addedAt int64, ips uint64) {
	snap := e.runtimeLedgerSnapshot()
	if snap.libDir == "" || snap.ledger == nil || ips == 0 {
		return
	}
	st := snap.ledger.feed(name)
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.cohortsLoaded {
		return
	}
	if st.cohorts == nil {
		st.cohorts = make(map[int64]uint64)
	}
	st.cohorts[addedAt] += ips
}

func (e *Engine) replaceRetentionCohorts(name string, cohorts map[int64]uint64) {
	snap := e.runtimeLedgerSnapshot()
	if snap.libDir == "" || snap.ledger == nil {
		return
	}
	st := snap.ledger.feed(name)
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.cohortsLoaded = true
	st.cohorts = make(map[int64]uint64, len(cohorts))
	for addedAt, ips := range cohorts {
		if ips == 0 {
			continue
		}
		st.cohorts[addedAt] = ips
	}
}
