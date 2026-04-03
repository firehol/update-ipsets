package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func (e *Engine) QueryIP(ctx context.Context, ipText string) ([]QueryMatch, error) {
	ctx = nonNilContext(ctx)
	ipv4, err := parseQueryIPv4(ipText)
	if err != nil {
		return nil, err
	}

	entries := e.EntriesSnapshot()
	matches := make([]QueryMatch, 0)
	for i := range entries {
		snap := &entries[i]
		name := snap.Name
		if !e.isPublicFeedName(name) {
			continue
		}
		if !e.hasUsableSet(name) {
			continue
		}
		match, found := e.queryNamedIPv4(ctx, name, snap, ipv4)
		if found {
			matches = append(matches, match)
		}
	}
	return matches, nil
}

func (e *Engine) QueryFeedIP(ctx context.Context, name, ipText string) (*QueryMatch, bool, error) {
	ctx = nonNilContext(ctx)
	if _, err := e.Entry(name); err != nil {
		return nil, false, err
	}
	ipv4, err := parseQueryIPv4(ipText)
	if err != nil {
		return nil, false, err
	}
	snap := e.EntrySnapshot(name)
	if snap == nil || !e.hasUsableSet(name) {
		return nil, false, fmt.Errorf("set %q has no usable data", name)
	}
	match, found := e.queryNamedIPv4(ctx, name, snap, ipv4)
	return &match, found, nil
}

func parseQueryIPv4(ipText string) (uint32, error) {
	ip := net.ParseIP(strings.TrimSpace(ipText)).To4()
	if ip == nil {
		return 0, fmt.Errorf("invalid IPv4 address %q", ipText)
	}
	return iprange.ParseIPv4Token(ip.String())
}

func (e *Engine) queryNamedIPv4(ctx context.Context, name string, snap *cache.Entry, ipv4 uint32) (QueryMatch, bool) {
	match := e.buildQueryMatch(name, snap)
	src, release, err := e.openLatestSetForQuery(ctx, name)
	if err != nil {
		e.logger.Warn("query: cannot open set for IP lookup", "set", name, "error", err)
		match.Error = fmt.Sprintf("failed to open set: %v", err)
		return match, false
	}
	defer release()
	found := src.Contains(ipv4)
	if ioErr := checkFileSetErr(src.RangeSource, name, e.logger); ioErr != nil {
		match.Error = ioErr.Error()
		return match, false
	}
	if found {
		e.populateQueryMatchTiming(ctx, &match, name, snap, ipv4)
	}
	return match, found
}

func (e *Engine) openLatestSetForQuery(ctx context.Context, name string) (*closableSource, func(), error) {
	if e != nil && e.querySetCache != nil {
		src, release, err := e.querySetCache.Acquire(name)
		if src != nil || err != nil {
			return src, release, err
		}
	}
	src, err := e.openLatestSet(ctx, name)
	if err != nil {
		return nil, nil, err
	}
	return src, func() {
		if src != nil {
			_ = src.Close()
		}
	}, nil
}

func (e *Engine) buildQueryMatch(name string, snap *cache.Entry) QueryMatch {
	if snap == nil {
		snap = e.EntrySnapshot(name)
	}
	src := e.lookupSource(name)
	match := QueryMatch{
		Name:       name,
		Provenance: string(publicProvenance(src)),
	}
	if snap == nil {
		return match
	}
	match.File = snap.File
	match.Category = snap.Category
	match.Info = snap.Info
	match.Maintainer = snap.Maintainer
	if snap.SourceDate > 0 {
		match.LastSeen = snap.SourceDate
	}
	if src != nil {
		match.Health = e.classifyEffectiveEntryHealth(name, snap)
	}
	return match
}

func (e *Engine) populateQueryMatchTiming(ctx context.Context, match *QueryMatch, name string, snap *cache.Entry, ipv4 uint32) {
	if match == nil || e == nil {
		return
	}
	firstSeen := e.queryMatchFirstSeen(ctx, name, ipv4)
	if firstSeen > 0 {
		match.FirstSeen = firstSeen
	}
}

func (e *Engine) queryMatchFirstSeen(ctx context.Context, name string, ipv4 uint32) int64 {
	if e == nil || e.runtime.LibDir == "" {
		return 0
	}
	cohorts := e.retentionCohortsFromRuntime(ctx, name)
	if len(cohorts) == 0 {
		return 0
	}

	keys := make([]int64, 0, len(cohorts))
	for addedAt, ips := range cohorts {
		if addedAt <= 0 || ips == 0 {
			continue
		}
		keys = append(keys, addedAt)
	}
	if len(keys) == 0 {
		return 0
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	for _, addedAt := range keys {
		path, err := e.retentionCohortPath(name, addedAt)
		if err != nil {
			e.logger.Warn("query: cannot resolve retention cohort", "feed", name, "added_at", addedAt, "error", err)
			continue
		}
		src, err := openRetentionCohortSet(ctx, name, path)
		if err != nil {
			e.logger.Warn("query: cannot open retention cohort", "feed", name, "cohort", path, "error", err)
			continue
		}
		found := src.Contains(ipv4)
		if ioErr := checkFileSetErr(src.RangeSource, name, e.logger); ioErr != nil {
			e.logger.Warn("query: cannot scan retention cohort", "feed", name, "cohort", path, "error", ioErr)
			_ = src.Close()
			continue
		}
		_ = src.Close()
		if !found {
			continue
		}
		return addedAt
	}
	return 0
}

func (e *Engine) retentionCohortPath(name string, addedAt int64) (string, error) {
	if e == nil || e.runtime.LibDir == "" || name == "" || addedAt <= 0 {
		return "", fmt.Errorf("invalid retention cohort lookup for %q at %d", name, addedAt)
	}
	dir := filepath.Join(e.runtime.LibDir, name, "new")
	base := strconv.FormatInt(addedAt, 10)
	for _, filename := range []string{base, base + ".set"} {
		path := filepath.Join(dir, filename)
		if fileExists(path) {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

func openRetentionCohortSet(ctx context.Context, name, path string) (*closableSource, error) {
	fs, err := iprange.OpenFileSet(path)
	if err == nil {
		return &closableSource{RangeSource: fs, close: fs.Close}, nil
	}
	set, err := loadSnapshotSet(ctx, name, path)
	if err != nil {
		return nil, err
	}
	return &closableSource{RangeSource: set}, nil
}

func (e *Engine) HistorySeries(name string) ([]HistoryPoint, error) {
	// Read from the internal full ledger first. Bash keeps
	// LIB_DIR/<feed>/history.csv append-only and generates public
	// <feed>_history.csv as a last-N window from it.
	points := e.historyFromLedgerCSV(name)
	if len(points) == 0 {
		// Compatibility fallback for Go rewrite data written before the
		// internal ledger was restored.
		points = e.historyFromWebCSV(name)
	}

	if len(points) == 0 {
		return nil, nil
	}
	return points, nil
}

// ChangesetSeries returns the (timestamp, added, removed) tuples written to
// <LibDir>/<name>/changesets.csv by the retention step. Each tuple corresponds
// to exactly one successful update where the binary set changed. A missing file
// returns an empty slice and a nil error — young feeds or feeds that never
// changed since tracking started simply have no changesets yet.
//
// The results match the bash public changeset window: ignore historical
// zero-delta rows, drop the bootstrap row, then return the last
// WebChartsEntries real changes.
func (e *Engine) ChangesetSeries(name string) ([]ChangesetPoint, error) {
	out, err := e.readChangesetLedger(name)
	if err != nil {
		return nil, err
	}
	if len(out) > 0 {
		out = out[1:]
	}
	window := e.webChartsEntries()
	if len(out) > window {
		out = out[len(out)-window:]
	}
	return out, nil
}

// PublicChangesetSeries reads the already-published web changeset artifact.
// It intentionally does not fall back to the internal ledger because public
// requests must not regenerate missing artifacts.
func (e *Engine) PublicChangesetSeries(name string) ([]ChangesetPoint, error) {
	return e.PublicChangesetSeriesInDir(name, e.outputDir())
}

func (e *Engine) PublicChangesetSeriesInDir(name, dir string) ([]ChangesetPoint, error) {
	if _, err := e.Entry(name); err != nil {
		return nil, err
	}
	if dir == "" {
		dir = e.outputDir()
	}
	path := filepath.Join(dir, name+"_changesets.csv")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no changeset data for %q", name)
		}
		return nil, err
	}
	return parseChangesetCSVData(data), nil
}

func (e *Engine) readChangesetLedger(name string) ([]ChangesetPoint, error) {
	if e == nil || e.runtime.LibDir == "" {
		return nil, nil
	}
	path := filepath.Join(e.runtime.LibDir, name, "changesets.csv")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseChangesetCSVData(data), nil
}

// historyFromLedgerCSV reads the internal append-only history ledger.
func (e *Engine) historyFromLedgerCSV(name string) []HistoryPoint {
	if e == nil || e.runtime.LibDir == "" {
		return nil
	}
	return parseHistoryCSV(filepath.Join(e.runtime.LibDir, name, "history.csv"), name)
}

// historyFromWebCSV reads _history.csv from the web output dir. This preserves
// legacy Go-rewrite data that was written before the internal bash-compatible
// full ledger was restored.
func (e *Engine) historyFromWebCSV(name string) []HistoryPoint {
	dir := e.outputDir()
	if dir == "" {
		return nil
	}
	return parseHistoryCSV(filepath.Join(dir, name+"_history.csv"), name)
}

// parseHistoryCSV reads a CSV with header "DateTime,Entries,UniqueIPs".
func parseHistoryCSV(path, name string) []HistoryPoint {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseHistoryCSVData(data, name)
}

func parseHistoryCSVData(data []byte, name string) []HistoryPoint {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) <= 1 {
		return nil
	}
	points := make([]HistoryPoint, 0, len(lines)-1)
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			continue
		}
		ts, err1 := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		entries, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		ips, err3 := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		if !validHistoryTimestamp(ts) {
			continue
		}
		points = append(points, HistoryPoint{
			Timestamp: ts,
			Name:      name,
			Entries:   entries,
			UniqueIPs: ips,
		})
	}
	return points
}

func parseChangesetCSVData(data []byte) []ChangesetPoint {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) <= 1 {
		return nil
	}
	out := make([]ChangesetPoint, 0, len(lines)-1)
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			continue
		}
		ts, err := parseInt64(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		added, err := parseUint64(strings.TrimSpace(parts[1]))
		if err != nil {
			continue
		}
		removed, err := parseUint64(strings.TrimSpace(parts[2]))
		if err != nil {
			continue
		}
		if added == 0 && removed == 0 {
			continue
		}
		out = append(out, ChangesetPoint{
			Timestamp: ts,
			Added:     added,
			Removed:   removed,
		})
	}
	return out
}

func validHistoryTimestamp(ts int64) bool {
	const (
		minHistoryUnix = 946684800  // 2000-01-01T00:00:00Z
		maxHistoryUnix = 4102444800 // 2100-01-01T00:00:00Z
	)
	return ts >= minHistoryUnix && ts <= maxHistoryUnix
}

func (e *Engine) CompareSet(ctx context.Context, name string) ([]CompareRow, error) {
	start := time.Now()
	e.observeRunCounter("http.compare_set.requests", 1, 0)
	defer func() {
		e.observeRunOperation("http.compare_set.request", time.Since(start))
	}()
	if _, err := e.Entry(name); err != nil {
		return nil, err
	}
	targetSnap := e.state.EntrySnapshot(name)
	if targetSnap == nil || !e.hasUsableSet(name) {
		return nil, fmt.Errorf("unknown set %q", name)
	}

	targetSrc, err := e.openLatestSet(ctx, name)
	if err != nil {
		return nil, err
	}
	e.observeRunCounter("http.compare_set.target_open", 1, 0)
	defer func() { _ = targetSrc.Close() }()

	names := e.publicOutputNames()
	filtered := names[:0]
	for _, candidate := range names {
		if candidate != name {
			filtered = append(filtered, candidate)
		}
	}
	names = filtered
	e.observeRunCounter("http.compare_set.candidates", int64(len(names)), 0)
	targetFamily := leafAncestors(e.cfg, name)

	out := make([]CompareRow, 0, len(names))
	for _, candidate := range names {
		snap := e.state.EntrySnapshot(candidate)
		if snap == nil || !e.hasUsableSet(candidate) {
			continue
		}
		candSrc, err := e.openLatestSet(ctx, candidate)
		if err != nil {
			e.logger.Error("CompareSet: failed to open candidate set", "set", candidate, "err", err)
			continue
		}
		e.observeRunCounter("http.compare_set.candidate_open", 1, 0)
		common := iprange.OverlapCountIter(targetSrc, candSrc)
		if ioErr := checkFileSetErr(targetSrc.RangeSource, name, e.logger); ioErr != nil {
			_ = candSrc.Close()
			e.logger.Error("CompareSet: I/O error on target set", "set", name, "err", ioErr)
			continue
		}
		if ioErr := checkFileSetErr(candSrc.RangeSource, candidate, e.logger); ioErr != nil {
			_ = candSrc.Close()
			e.logger.Error("CompareSet: I/O error on candidate set", "set", candidate, "err", ioErr)
			continue
		}
		candIPs := candSrc.UniqueIPs()
		if err := candSrc.Close(); err != nil {
			e.logger.Error("CompareSet: failed to close candidate set", "set", candidate, "err", err)
			continue
		}

		category := snap.Category
		related := familiesIntersect(targetFamily, leafAncestors(e.cfg, candidate))
		out = append(out, CompareRow{
			Name:     candidate,
			Category: category,
			IPs:      candIPs,
			Common:   common,
			Related:  related,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Common != out[j].Common {
			return out[i].Common > out[j].Common
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func familiesIntersect(left, right map[string]bool) bool {
	if len(right) < len(left) {
		left, right = right, left
	}
	for key := range left {
		if right[key] {
			return true
		}
	}
	return false
}

func (e *Engine) Retention(name string) (*RetentionData, error) {
	// Try pre-built JSON first (generated by updateRetention during processing).
	path := filepath.Join(e.runtime.LibDir, name, "retention.json")
	data, err := os.ReadFile(path)
	if err != nil {
		// JSON doesn't exist yet (e.g. immediately after bash-state import) —
		// build it from the retained CSV evidence.
		return e.buildRetentionData(context.Background(), name, e.now().UTC().Unix())
	}
	var out RetentionData
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (e *Engine) StatusSnapshot() StatusSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var currentMetrics *RunMetricsSnapshot
	if e.currentMetrics != nil {
		snap := e.currentMetrics.snapshot(true)
		currentMetrics = &snap
	}
	var lastMetrics *RunMetricsSnapshot
	if e.lastMetrics != nil {
		snap := *e.lastMetrics
		lastMetrics = &snap
	}
	backgroundLimit, backgroundRunning := e.backgroundLimiter.Snapshot()
	return StatusSnapshot{
		Running:                      e.running,
		LastStarted:                  e.lastStarted,
		LastEnded:                    e.lastEnded,
		LastError:                    e.lastError,
		LastReport:                   e.lastReport,
		CurrentReason:                e.currentReason,
		LastReason:                   e.lastReason,
		CurrentPhase:                 e.currentPhase,
		ActiveFeeds:                  e.snapshotActiveFeedsLocked(),
		BackgroundTasks:              e.snapshotBackgroundTasksLocked(),
		BackgroundLimit:              backgroundLimit,
		BackgroundRunning:            backgroundRunning,
		CurrentMetrics:               currentMetrics,
		LastMetrics:                  lastMetrics,
		LifetimeMetrics:              e.lifetimeMetricsSnapshot(),
		ConfigPath:                   e.runtime.ConfigPath,
		BaseDir:                      e.runtime.BaseDir,
		SourceCount:                  len(e.cfg.Sources),
		MergeCount:                   e.mergeCount(),
		EntityRefreshPending:         len(e.entityRefreshPending),
		EntityHealthPending:          len(e.entityHealthPending),
		EntityRebuildPending:         e.entityRebuildQueued,
		LastConfigReload:             e.lastConfigReload,
		ConfigReloadCount:            e.configReloadCount,
		LastConfigReloadError:        e.lastConfigReloadError,
		StartupRepairDeferred:        e.startupRepairDeferred,
		StartupRepairDeferredTargets: e.startupRepairDeferredTargets,
	}
}

func (e *Engine) mergeCount() int {
	if e == nil || e.cfg == nil {
		return 0
	}
	count := 0
	for name := range e.cfg.Sources {
		if e.IsMerge(name) {
			count++
		}
	}
	return count
}

func (e *Engine) EntriesSnapshot() []cache.Entry {
	return e.entriesSnapshot(e.configuredNames())
}

func (e *Engine) EntriesSnapshotWithArtifacts() []cache.Entry {
	return e.entriesSnapshot(e.configuredNamesWithArtifacts())
}

func (e *Engine) entriesSnapshot(configured map[string]bool) []cache.Entry {
	snapMap := e.state.SnapshotEntries()
	resolver := newEffectiveEntryResolver(e.cfg, snapMap)
	names := make([]string, 0, len(snapMap))
	for name := range snapMap {
		if configured[name] {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	out := make([]cache.Entry, 0, len(names))
	for _, name := range names {
		entry := snapMap[name]
		if view := resolver.entry(name, &entry); view != nil {
			out = append(out, *view)
			continue
		}
		out = append(out, entry)
	}
	return out
}
