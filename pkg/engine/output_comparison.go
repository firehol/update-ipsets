package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

type comparisonSetInfo struct {
	name        string
	ips         uint64
	category    string
	filter      iprange.RangeOverlapFilter
	contentHash iprange.RangeContentHash
}

type comparisonUpdateFilter map[string]struct{}

type comparisonPair struct {
	i int
	j int
}

type comparisonPairResult struct {
	i      int
	j      int
	common uint64
}

type comparisonPairStats struct {
	candidates    int64
	ledgerLookup  atomic.Int64
	ledgerHit     atomic.Int64
	ledgerMiss    atomic.Int64
	ledgerSkipped atomic.Int64
	overlapCount  atomic.Int64
	overlapTotal  atomic.Int64
	overlapMax    atomic.Int64
	skipped       atomic.Int64
	skippedEmpty  atomic.Int64
	skippedRange  atomic.Int64
	skippedPrefix atomic.Int64
	skippedSparse atomic.Int64
	identical     atomic.Int64
}

type comparisonMergeStats struct {
	count int64
	total time.Duration
	max   time.Duration
}

// writeComparisonFiles computes pairwise overlap between ipsets.
// When updatedNames is non-empty, only those sources are compared against
// all others — the rest keep their existing _comparison.json files on disk.
// When updatedNames is empty, all sources are compared (initial run).
func (e *Engine) writeComparisonFiles(ctx context.Context, updatedNames []string, outDir string, setCache *latestSetCache) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}

	names := e.publicOutputNames()
	if len(names) < 2 {
		return nil
	}
	if setCache == nil {
		setCache = newLatestSetCache(e)
		defer setCache.CloseAll(e.logger)
	}

	infos, err := e.prepareComparisonSetInfos(ctx, names, setCache)
	if err != nil {
		return err
	}
	if len(infos) < 2 {
		return nil
	}

	updated := newComparisonUpdateFilter(updatedNames)
	var ledger *comparisonPairLedgerSnapshot
	if len(updated) > 0 {
		var entries int
		var bytes int64
		var loadErr error
		ledger, entries, bytes, loadErr = e.loadComparisonPairLedger()
		if loadErr != nil {
			if !errors.Is(loadErr, errComparisonPairLedgerMissing) {
				e.logger.Warn("comparison pair ledger ignored", "error", loadErr)
				e.observeRunCounter("metadata.comparison_pair_ledger_ignored", 1, bytes)
			}
			e.observeRunCounter("metadata.comparison_pair_ledger_full_rebuild", 1, bytes)
			ledger = nil
			updated = nil
		} else if entries > 0 {
			e.observeRunCounter("metadata.comparison_pair_ledger_read", 1, bytes)
			e.observeRunCounter("metadata.comparison_pair_ledger_entries_read", int64(entries), 0)
		}
	}

	pairResults, pairStats := e.runComparisonPairs(ctx, infos, updated, setCache, ledger)
	if err := contextErr(ctx); err != nil {
		return err
	}
	e.observeComparisonPairStats(pairStats)

	grouped := e.groupComparisonRows(infos, pairResults)
	if err := e.writeMergedComparisonRows(outDir, grouped); err != nil {
		return err
	}
	entries, bytes, err := e.writeComparisonPairLedger(infos, pairResults)
	if err != nil {
		e.logger.Warn("comparison pair ledger write failed", "error", err)
		e.observeRunCounter("metadata.comparison_pair_ledger_write_failed", 1, bytes)
		return nil
	}
	if entries > 0 {
		e.observeRunCounter("metadata.comparison_pair_ledger_write", 1, bytes)
		e.observeRunCounter("metadata.comparison_pair_ledger_entries_write", int64(entries), 0)
	}
	return nil
}

func (e *Engine) prepareComparisonSetInfos(ctx context.Context, names []string, setCache *latestSetCache) ([]comparisonSetInfo, error) {
	ctx = nonNilContext(ctx)
	prepareStarted := time.Now()
	progress := e.beginActiveOperation("metadata.prepare_comparison_sets", "", "prepare", "feeds", int64(len(names)))
	defer progress.Finish()
	infos := make([]comparisonSetInfo, 0, len(names))
	for _, name := range names {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		snap := e.state.EntrySnapshot(name)
		if snap == nil || !e.hasUsableSet(name) {
			progress.Add(1, int64(len(names)), nil)
			continue
		}
		src, err := setCache.Open(name)
		if err != nil {
			e.logger.Warn("comparison skipped: cannot open set", "set", name, "error", err)
			progress.Add(1, int64(len(names)), nil)
			continue
		}
		summary, err := setCache.Summary(ctx, name)
		if err != nil {
			progress.Add(1, int64(len(names)), nil)
			return nil, err
		}
		if ioErr := checkFileSetErr(src.RangeSource, name, e.logger); ioErr != nil {
			progress.Add(1, int64(len(names)), nil)
			continue
		}
		infos = append(infos, comparisonSetInfo{
			name:        name,
			ips:         src.UniqueIPs(),
			category:    snap.Category,
			filter:      summary.OverlapFilter(),
			contentHash: summary.ContentHash,
		})
		progress.Add(1, int64(len(names)), nil)
	}
	e.observeRunOperation("metadata.comparison_prepare_sets", time.Since(prepareStarted))
	return infos, nil
}

func newComparisonUpdateFilter(updatedNames []string) comparisonUpdateFilter {
	updated := make(comparisonUpdateFilter, len(updatedNames))
	for _, name := range updatedNames {
		updated[name] = struct{}{}
	}
	return updated
}

func (f comparisonUpdateFilter) includes(name string) bool {
	if len(f) == 0 {
		return true
	}
	_, ok := f[name]
	return ok
}

func (f comparisonUpdateFilter) contains(name string) bool {
	_, ok := f[name]
	return ok
}

func (e *Engine) runComparisonPairs(ctx context.Context, infos []comparisonSetInfo, updated comparisonUpdateFilter, setCache *latestSetCache, ledger *comparisonPairLedgerSnapshot) ([]comparisonPairResult, *comparisonPairStats) {
	totalPairs := int64(len(infos) * (len(infos) - 1) / 2)
	scanOp := e.beginActiveOperation("metadata.scan_comparison_pairs", "", "scan", "pairs", totalPairs)
	defer scanOp.Finish()
	compareOp := e.beginActiveOperation("metadata.compare_pairs", "", "compare", "candidate_pairs", 0)
	defer compareOp.Finish()

	var pairResults []comparisonPairResult
	var ledgerResults []comparisonPairResult
	var exactPairs []comparisonPair
	stats := &comparisonPairStats{}

	for i := 0; i < len(infos); i++ {
		for j := i + 1; j < len(infos); j++ {
			scanOp.Add(1, totalPairs, nil)
			if ledger != nil {
				stats.ledgerLookup.Add(1)
				if common, ok := ledger.lookup(infos[i], infos[j]); ok {
					stats.ledgerHit.Add(1)
					ledgerResults = append(ledgerResults, comparisonPairResult{i: i, j: j, common: common})
					continue
				}
				stats.ledgerMiss.Add(1)
				if !updated.includes(infos[i].name) && !updated.includes(infos[j].name) {
					stats.ledgerSkipped.Add(1)
					continue
				}
			} else if !updated.includes(infos[i].name) && !updated.includes(infos[j].name) {
				continue
			}
			if err := contextErr(ctx); err != nil {
				return pairResults, stats
			}
			pair := comparisonPair{i: i, j: j}
			if result, ok := e.compareSetPairFast(pair, infos, stats); ok {
				pairResults = append(pairResults, result)
				continue
			}
			exactPairs = append(exactPairs, pair)
			stats.candidates++
		}
	}

	compareOp.Update(0, stats.candidates, nil)
	if len(exactPairs) > 0 {
		exactResults := e.compareSetPairBatches(ctx, exactPairs, infos, updated, setCache, stats, compareOp)
		pairResults = append(pairResults, exactResults...)
	}
	if len(ledgerResults) > 0 {
		pairResults = append(pairResults, ledgerResults...)
	}
	return pairResults, stats
}

func (e *Engine) compareSetPairFast(pair comparisonPair, infos []comparisonSetInfo, stats *comparisonPairStats) (comparisonPairResult, bool) {
	if infos[pair.i].ips == 0 || infos[pair.j].ips == 0 {
		stats.recordSkippedEmpty()
		return comparisonPairResult{i: pair.i, j: pair.j, common: 0}, true
	}
	if infos[pair.i].filter.BoundsDisjoint(infos[pair.j].filter) {
		stats.recordSkippedRange()
		return comparisonPairResult{i: pair.i, j: pair.j, common: 0}, true
	}
	if comparisonSetsIdentical(infos[pair.i], infos[pair.j]) {
		stats.recordIdentical()
		return comparisonPairResult{i: pair.i, j: pair.j, common: infos[pair.i].ips}, true
	}
	if infos[pair.i].filter.SparsePrefixesDisjoint(infos[pair.j].filter) {
		stats.recordSkippedSparse()
		return comparisonPairResult{i: pair.i, j: pair.j, common: 0}, true
	}
	if infos[pair.i].filter.PrefixesDisjoint(infos[pair.j].filter) {
		stats.recordSkippedPrefix()
		return comparisonPairResult{i: pair.i, j: pair.j, common: 0}, true
	}
	return comparisonPairResult{}, false
}

type comparisonOrientedPair struct {
	original comparisonPair
	left     int
	right    int
}

type comparisonOrientedPairBatch struct {
	left  int
	pairs []comparisonOrientedPair
}

const comparisonPairBatchMaxPairs = 256

func comparisonPairBatches(pairs []comparisonPair, infos []comparisonSetInfo, updated comparisonUpdateFilter) []comparisonOrientedPairBatch {
	if len(pairs) == 0 {
		return nil
	}
	groups := make([]comparisonOrientedPairBatch, 0, len(pairs))
	groupIndex := make(map[int]int, len(pairs))
	for _, pair := range pairs {
		oriented := orientComparisonPair(pair, infos, updated)
		idx, ok := groupIndex[oriented.left]
		if !ok || len(groups[idx].pairs) >= comparisonPairBatchMaxPairs {
			idx = len(groups)
			groupIndex[oriented.left] = idx
			groups = append(groups, comparisonOrientedPairBatch{left: oriented.left})
		}
		groups[idx].pairs = append(groups[idx].pairs, oriented)
	}
	return groups
}

func orientComparisonPair(pair comparisonPair, infos []comparisonSetInfo, updated comparisonUpdateFilter) comparisonOrientedPair {
	left, right := pair.i, pair.j
	if len(updated) > 0 && right < len(infos) && left < len(infos) {
		leftUpdated := updated.contains(infos[left].name)
		rightUpdated := updated.contains(infos[right].name)
		if rightUpdated && !leftUpdated {
			left, right = right, left
		}
	}
	return comparisonOrientedPair{
		original: pair,
		left:     left,
		right:    right,
	}
}

func (e *Engine) compareSetPairBatches(ctx context.Context, pairs []comparisonPair, infos []comparisonSetInfo, updated comparisonUpdateFilter, setCache *latestSetCache, stats *comparisonPairStats, progress *activeOperationHandle) []comparisonPairResult {
	batches := comparisonPairBatches(pairs, infos, updated)
	if len(batches) == 0 {
		return nil
	}
	results := make([]comparisonPairResult, 0, len(pairs))
	var completed int64
	for _, batch := range batches {
		if err := contextErr(ctx); err != nil {
			return results
		}
		results = append(results, e.compareSetPairBatch(ctx, batch.pairs, infos, setCache, stats)...)
		completed += int64(len(batch.pairs))
		if progress != nil {
			progress.Update(completed, int64(len(pairs)), nil)
		}
	}
	return results
}

func (e *Engine) compareSetPairBatch(ctx context.Context, pairs []comparisonOrientedPair, infos []comparisonSetInfo, setCache *latestSetCache, stats *comparisonPairStats) []comparisonPairResult {
	sources, iprangePairs, originalPairs := e.comparisonPairBatchSources(pairs, infos, setCache)
	if len(iprangePairs) == 0 {
		return nil
	}

	started := time.Now()
	rows, err := iprange.CompareSourcePairs(ctx, sources, iprangePairs)
	elapsed := time.Since(started)
	if stats != nil {
		stats.overlapCount.Add(int64(len(rows)))
		stats.overlapTotal.Add(elapsed.Nanoseconds())
		for {
			current := stats.overlapMax.Load()
			if elapsed.Nanoseconds() <= current {
				break
			}
			if stats.overlapMax.CompareAndSwap(current, elapsed.Nanoseconds()) {
				break
			}
		}
	}
	if err != nil {
		if !errors.Is(err, context.Canceled) && e.logger != nil {
			e.logger.Warn("pairwise comparison batch skipped", "error", err)
		}
		return nil
	}
	results := make([]comparisonPairResult, 0, len(rows))
	for i, row := range rows {
		pair := originalPairs[i]
		results = append(results, comparisonPairResult{i: pair.i, j: pair.j, common: row.CommonIPs})
	}
	return results
}

func (e *Engine) comparisonPairBatchSources(pairs []comparisonOrientedPair, infos []comparisonSetInfo, setCache *latestSetCache) ([]iprange.CompareSource, []iprange.ComparePair, []comparisonPair) {
	if setCache == nil {
		return nil, nil, nil
	}
	sourceIndexes := make(map[int]int)
	badSources := make(map[int]struct{})
	sources := make([]iprange.CompareSource, 0, len(infos))
	ensureSource := func(idx int) (int, bool) {
		if compact, ok := sourceIndexes[idx]; ok {
			return compact, true
		}
		if _, bad := badSources[idx]; bad {
			return 0, false
		}
		src, err := setCache.Open(infos[idx].name)
		if err != nil {
			badSources[idx] = struct{}{}
			if e.logger != nil {
				e.logger.Warn("pairwise comparison skipped: cannot open set", "set", infos[idx].name, "error", err)
			}
			return 0, false
		}
		compact := len(sources)
		sourceIndexes[idx] = compact
		sources = append(sources, iprange.CompareSource{Name: infos[idx].name, Source: src.RangeSource})
		return compact, true
	}

	iprangePairs := make([]iprange.ComparePair, 0, len(pairs))
	originalPairs := make([]comparisonPair, 0, len(pairs))
	for _, pair := range pairs {
		left, ok := ensureSource(pair.left)
		if !ok {
			continue
		}
		right, ok := ensureSource(pair.right)
		if !ok {
			continue
		}
		iprangePairs = append(iprangePairs, iprange.ComparePair{Left: left, Right: right})
		originalPairs = append(originalPairs, pair.original)
	}
	return sources, iprangePairs, originalPairs
}

func comparisonSetsIdentical(a, b comparisonSetInfo) bool {
	return a.ips > 0 && a.ips == b.ips && a.contentHash.Equal(b.contentHash)
}

func (s *comparisonPairStats) recordSkippedEmpty() {
	s.skipped.Add(1)
	s.skippedEmpty.Add(1)
}

func (s *comparisonPairStats) recordSkippedRange() {
	s.skipped.Add(1)
	s.skippedRange.Add(1)
}

func (s *comparisonPairStats) recordSkippedPrefix() {
	s.skipped.Add(1)
	s.skippedPrefix.Add(1)
}

func (s *comparisonPairStats) recordSkippedSparse() {
	s.skipped.Add(1)
	s.skippedSparse.Add(1)
}

func (s *comparisonPairStats) recordIdentical() {
	s.identical.Add(1)
}

func (e *Engine) observeComparisonPairStats(stats *comparisonPairStats) {
	if stats == nil {
		return
	}
	if stats.candidates > 0 {
		e.observeRunCounter("metadata.comparison_pair_candidates", stats.candidates, 0)
	}
	if count := stats.ledgerLookup.Load(); count > 0 {
		e.observeRunCounter("metadata.comparison_pair_ledger_lookup", count, 0)
	}
	if count := stats.ledgerHit.Load(); count > 0 {
		e.observeRunCounter("metadata.comparison_pair_ledger_hit", count, 0)
	}
	if count := stats.ledgerMiss.Load(); count > 0 {
		e.observeRunCounter("metadata.comparison_pair_ledger_miss", count, 0)
	}
	if count := stats.ledgerSkipped.Load(); count > 0 {
		e.observeRunCounter("metadata.comparison_pair_ledger_miss_unchanged_skipped", count, 0)
	}
	if count := stats.overlapCount.Load(); count > 0 {
		e.observeRunCounter("metadata.comparison_pair_overlap", count, 0)
		e.observeRunOperationAggregate(
			"metadata.comparison_pair_overlap",
			count,
			time.Duration(stats.overlapTotal.Load()),
			time.Duration(stats.overlapMax.Load()),
		)
	}
	if skipped := stats.skipped.Load(); skipped > 0 {
		e.observeRunOperationAggregate("metadata.comparison_pair_skipped", skipped, 0, 0)
		e.observeRunCounter("metadata.comparison_pair_skipped", skipped, 0)
	}
	if skipped := stats.skippedEmpty.Load(); skipped > 0 {
		e.observeRunCounter("metadata.comparison_pair_skipped_empty", skipped, 0)
	}
	if skipped := stats.skippedRange.Load(); skipped > 0 {
		e.observeRunCounter("metadata.comparison_pair_skipped_range", skipped, 0)
	}
	if skipped := stats.skippedPrefix.Load(); skipped > 0 {
		e.observeRunCounter("metadata.comparison_pair_skipped_prefix", skipped, 0)
	}
	if skipped := stats.skippedSparse.Load(); skipped > 0 {
		e.observeRunCounter("metadata.comparison_pair_skipped_sparse_prefix", skipped, 0)
	}
	if identical := stats.identical.Load(); identical > 0 {
		e.observeRunCounter("metadata.comparison_pair_identical", identical, 0)
	}
}

func (e *Engine) groupComparisonRows(infos []comparisonSetInfo, pairResults []comparisonPairResult) map[string][]CompareRow {
	relatedness := newComparisonRelatedness(e.cfg)
	grouped := make(map[string][]CompareRow)
	for _, result := range pairResults {
		left := infos[result.i]
		right := infos[result.j]
		related := relatedness.areRelated(left.name, right.name)
		grouped[left.name] = append(grouped[left.name], CompareRow{
			Name:     right.name,
			Category: right.category,
			IPs:      right.ips,
			Common:   result.common,
			Related:  related,
		})
		grouped[right.name] = append(grouped[right.name], CompareRow{
			Name:     left.name,
			Category: left.category,
			IPs:      left.ips,
			Common:   result.common,
			Related:  related,
		})
	}
	return grouped
}

type comparisonRelatedness struct {
	cfg         *config.Config
	familyCache map[string]map[string]bool
}

func newComparisonRelatedness(cfg *config.Config) *comparisonRelatedness {
	return &comparisonRelatedness{
		cfg:         cfg,
		familyCache: make(map[string]map[string]bool),
	}
}

func (r *comparisonRelatedness) areRelated(a, b string) bool {
	fa := r.familyFor(a)
	fb := r.familyFor(b)
	if len(fb) < len(fa) {
		fa, fb = fb, fa
	}
	for name := range fa {
		if fb[name] {
			return true
		}
	}
	return false
}

func (r *comparisonRelatedness) familyFor(name string) map[string]bool {
	if cached, ok := r.familyCache[name]; ok {
		return cached
	}
	family := leafAncestors(r.cfg, name)
	r.familyCache[name] = family
	return family
}

func (e *Engine) writeMergedComparisonRows(outDir string, grouped map[string][]CompareRow) error {
	var stats comparisonMergeStats
	progress := e.beginActiveOperation("metadata.write_comparison_rows", "", "write", "feeds", int64(len(grouped)))
	defer progress.Finish()
	for name, group := range grouped {
		started := time.Now()
		if err := e.writeMergedComparisonRowsForFeed(outDir, name, group); err != nil {
			progress.Add(1, int64(len(grouped)), nil)
			return err
		}
		progress.Add(1, int64(len(grouped)), nil)
		stats.record(time.Since(started))
	}
	if stats.count > 0 {
		e.observeRunOperationAggregate("metadata.comparison_merge_rows", stats.count, stats.total, stats.max)
	}
	return nil
}

func (s *comparisonMergeStats) record(dur time.Duration) {
	s.count++
	s.total += dur
	if dur > s.max {
		s.max = dur
	}
}

func (e *Engine) writeMergedComparisonRowsForFeed(outDir, name string, group []CompareRow) error {
	merged := mergeCompareRows(e.readExistingComparisonRows(name), group)
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Name < merged[j].Name
	})
	data, err := jsonMarshalTabIndent(merged)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	logicalTime := e.feedProcessingTimestamp(name)
	path := filepath.Join(outDir, name+"_comparison.json")
	if e.comparisonArtifactAlreadyCurrent(path, data, logicalTime) {
		return nil
	}
	if filepath.Clean(outDir) != filepath.Clean(e.outputDir()) && !fileExists(path) {
		livePath := filepath.Join(e.outputDir(), name+"_comparison.json")
		if e.comparisonArtifactAlreadyCurrent(livePath, data, logicalTime) {
			return nil
		}
	}
	return writeFileAtomicAt(path, data, generatedFileMode, logicalTime)
}

func (e *Engine) comparisonArtifactAlreadyCurrent(path string, data []byte, logicalTime time.Time) bool {
	if e != nil && e.runtime.WebOwner != "" {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	if info.Mode().Perm() != generatedFileMode {
		return false
	}
	if !logicalTime.IsZero() && !info.ModTime().UTC().Equal(logicalTime.UTC()) {
		return false
	}
	if info.Size() != int64(len(data)) {
		return false
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Equal(existing, data)
}

func (e *Engine) readExistingComparisonRows(name string) []CompareRow {
	var existing []CompareRow
	data, err := readFileInRoot(e.outputDir(), name+"_comparison.json")
	if err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			e.logger.Warn("comparison merge: ignoring unreadable existing file", "set", name, "error", err)
			return nil
		}
		return existing
	}
	if !os.IsNotExist(err) {
		e.logger.Warn("comparison merge: cannot read existing file", "set", name, "error", err)
	}
	return nil
}
