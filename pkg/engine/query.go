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
	"github.com/firehol/update-ipsets/pkg/config"
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
		src, release, err := e.querySetCache.AcquireContext(ctx, name)
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
	if e == nil {
		return 0
	}
	rt := e.Runtime()
	if rt.LibDir == "" {
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
		path, rel, err := retentionCohortPathForRuntime(rt, name, addedAt)
		if err != nil {
			e.logger.Warn("query: cannot resolve retention cohort", "feed", name, "added_at", addedAt, "error", err)
			continue
		}
		src, err := openRetentionCohortSet(ctx, name, rt.LibDir, rel, path)
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

func retentionCohortPathForRuntime(rt Runtime, name string, addedAt int64) (string, string, error) {
	if rt.LibDir == "" || name == "" || addedAt <= 0 {
		return "", "", fmt.Errorf("invalid retention cohort lookup for %q at %d", name, addedAt)
	}
	dir := filepath.Join(rt.LibDir, name, "new")
	relDir := filepath.Join(name, "new")
	base := strconv.FormatInt(addedAt, 10)
	for _, filename := range []string{base, base + ".set"} {
		path := filepath.Join(dir, filename)
		if fileExists(path) {
			return path, filepath.Join(relDir, filename), nil
		}
	}
	return "", "", os.ErrNotExist
}

func openRetentionCohortSet(ctx context.Context, name, rootDir, rel, path string) (*closableSource, error) {
	fs, err := iprange.OpenFileSetWithOptions(path, iprange.FileSetOpenOptions{TrustOptimizedPayload: true})
	if err == nil {
		return &closableSource{RangeSource: fs, close: fs.Close}, nil
	}
	set, err := loadSnapshotSet(ctx, name, rootDir, rel)
	if err != nil {
		return nil, err
	}
	return &closableSource{RangeSource: set}, nil
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
	cfg := e.Config()
	targetFamily := leafAncestors(cfg, name)

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
		common, err := iprange.OverlapCountIterContext(ctx, targetSrc, candSrc)
		if err != nil {
			_ = candSrc.Close()
			return nil, err
		}
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
		related := familiesIntersect(targetFamily, leafAncestors(cfg, candidate))
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
	return e.RetentionContext(context.Background(), name)
}

func (e *Engine) RetentionContext(ctx context.Context, name string) (*RetentionData, error) {
	// Try pre-built JSON first (generated by updateRetention during processing).
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("unknown set %q", name)
	}
	rt := e.Runtime()
	data, err := readFileInRoot(rt.LibDir, filepath.Join(name, "retention.json"))
	if err != nil {
		// JSON doesn't exist yet (e.g. immediately after bash-state import) —
		// build it from the retained CSV evidence.
		return e.buildRetentionData(ctx, name, e.now().UTC().Unix())
	}
	var out RetentionData
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func mergeCountForConfig(cfg *config.Config) int {
	if cfg == nil {
		return 0
	}
	count := 0
	for _, src := range cfg.Sources {
		if src != nil && src.Provenance == config.ProvenanceSecondaryMerge {
			count++
		}
	}
	return count
}

func (e *Engine) EntriesSnapshot() []cache.Entry {
	cfg := e.Config()
	return e.EntriesSnapshotForConfig(cfg)
}

func (e *Engine) EntriesSnapshotWithArtifacts() []cache.Entry {
	cfg := e.Config()
	return e.EntriesSnapshotWithArtifactsForConfig(cfg)
}

func (e *Engine) EntriesSnapshotForConfig(cfg *config.Config) []cache.Entry {
	return e.entriesSnapshot(cfg, configuredNamesForConfig(cfg))
}

func (e *Engine) EntriesSnapshotWithArtifactsForConfig(cfg *config.Config) []cache.Entry {
	return e.entriesSnapshot(cfg, configuredNamesWithArtifactsForConfig(cfg))
}

func (e *Engine) entriesSnapshot(cfg *config.Config, configured map[string]bool) []cache.Entry {
	if cfg == nil {
		return nil
	}
	snapMap := e.state.SnapshotEntries()
	resolver := newEffectiveEntryResolver(cfg, snapMap)
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
