package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

type comparisonSetInfo struct {
	name         string
	ips          uint64
	category     string
	lo           uint32
	hi           uint32
	hasRange     bool
	prefixBitmap *comparisonPrefixBitmap
	sparsePrefix *comparisonSparsePrefixSet
	contentHash  comparisonContentHash
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

	infos := e.prepareComparisonSetInfos(names, setCache)
	if len(infos) < 2 {
		return nil
	}

	pairResults, pairStats := e.runComparisonPairs(ctx, infos, newComparisonUpdateFilter(updatedNames), setCache)
	if err := contextErr(ctx); err != nil {
		return err
	}
	e.observeComparisonPairStats(pairStats)

	grouped := e.groupComparisonRows(infos, pairResults)
	if err := e.writeMergedComparisonRows(outDir, grouped); err != nil {
		return err
	}
	return e.sanitizeComparisonArtifacts(outDir)
}

func (e *Engine) prepareComparisonSetInfos(names []string, setCache *latestSetCache) []comparisonSetInfo {
	prepareStarted := time.Now()
	infos := make([]comparisonSetInfo, 0, len(names))
	for _, name := range names {
		snap := e.state.EntrySnapshot(name)
		if snap == nil || !e.hasUsableSet(name) {
			continue
		}
		src, err := setCache.Open(name)
		if err != nil {
			e.logger.Warn("comparison skipped: cannot open set", "set", name, "error", err)
			continue
		}
		signature := buildComparisonSetSignature(src.RangeSource)
		if ioErr := checkFileSetErr(src.RangeSource, name, e.logger); ioErr != nil {
			continue
		}
		lo, hi, hasRange := rangeBounds(src)
		infos = append(infos, comparisonSetInfo{
			name:         name,
			ips:          src.UniqueIPs(),
			category:     snap.Category,
			lo:           lo,
			hi:           hi,
			hasRange:     hasRange,
			prefixBitmap: signature.prefixBitmap,
			sparsePrefix: signature.sparsePrefix,
			contentHash:  signature.contentHash,
		})
	}
	e.observeRunOperation("metadata.comparison_prepare_sets", time.Since(prepareStarted))
	return infos
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

func (e *Engine) runComparisonPairs(ctx context.Context, infos []comparisonSetInfo, updated comparisonUpdateFilter, setCache *latestSetCache) ([]comparisonPairResult, *comparisonPairStats) {
	numWorkers := e.runtime.HeavyPhaseWorkers()
	if numWorkers < 1 {
		numWorkers = 1
	}

	pairCh := make(chan comparisonPair)
	var pairMu sync.Mutex
	var pairResults []comparisonPairResult
	var wg sync.WaitGroup
	stats := &comparisonPairStats{}

	for range numWorkers {
		wg.Go(func() {
			for pair := range pairCh {
				result, ok := e.compareSetPair(ctx, pair, infos, setCache, stats)
				if !ok {
					continue
				}
				pairMu.Lock()
				pairResults = append(pairResults, result)
				pairMu.Unlock()
			}
		})
	}

sendPairs:
	for i := 0; i < len(infos); i++ {
		for j := i + 1; j < len(infos); j++ {
			if !updated.includes(infos[i].name) && !updated.includes(infos[j].name) {
				continue
			}
			stats.candidates++
			select {
			case <-ctx.Done():
				break sendPairs
			case pairCh <- comparisonPair{i: i, j: j}:
			}
		}
	}
	close(pairCh)
	wg.Wait()
	return pairResults, stats
}

func (e *Engine) compareSetPair(ctx context.Context, pair comparisonPair, infos []comparisonSetInfo, setCache *latestSetCache, stats *comparisonPairStats) (comparisonPairResult, bool) {
	if err := contextErr(ctx); err != nil {
		return comparisonPairResult{}, false
	}
	if infos[pair.i].ips == 0 || infos[pair.j].ips == 0 {
		stats.recordSkippedEmpty()
		return comparisonPairResult{i: pair.i, j: pair.j, common: 0}, true
	}
	if comparisonRangesDisjoint(infos[pair.i], infos[pair.j]) {
		stats.recordSkippedRange()
		return comparisonPairResult{i: pair.i, j: pair.j, common: 0}, true
	}
	if comparisonSetsIdentical(infos[pair.i], infos[pair.j]) {
		stats.recordIdentical()
		return comparisonPairResult{i: pair.i, j: pair.j, common: infos[pair.i].ips}, true
	}
	if !comparisonSparsePrefixOverlap(infos[pair.i].sparsePrefix, infos[pair.j].sparsePrefix) {
		stats.recordSkippedSparse()
		return comparisonPairResult{i: pair.i, j: pair.j, common: 0}, true
	}
	if !comparisonPrefixOverlap(infos[pair.i].prefixBitmap, infos[pair.j].prefixBitmap) {
		stats.recordSkippedPrefix()
		return comparisonPairResult{i: pair.i, j: pair.j, common: 0}, true
	}

	srcA, err := setCache.Open(infos[pair.i].name)
	if err != nil {
		e.logger.Warn("pairwise comparison skipped: cannot open set", "set", infos[pair.i].name, "error", err)
		return comparisonPairResult{}, false
	}
	srcB, err := setCache.Open(infos[pair.j].name)
	if err != nil {
		e.logger.Warn("pairwise comparison skipped: cannot open set", "set", infos[pair.j].name, "error", err)
		return comparisonPairResult{}, false
	}

	started := time.Now()
	common := iprange.OverlapCountIter(srcA, srcB)
	recordAtomicDuration(&stats.overlapCount, &stats.overlapTotal, &stats.overlapMax, time.Since(started))
	ioErrA := checkFileSetErr(srcA.RangeSource, infos[pair.i].name, e.logger)
	ioErrB := checkFileSetErr(srcB.RangeSource, infos[pair.j].name, e.logger)
	if ioErrA != nil || ioErrB != nil {
		return comparisonPairResult{}, false
	}
	return comparisonPairResult{i: pair.i, j: pair.j, common: common}, true
}

func comparisonRangesDisjoint(a, b comparisonSetInfo) bool {
	return a.hasRange && b.hasRange && (a.hi < b.lo || b.hi < a.lo)
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
	for name, group := range grouped {
		started := time.Now()
		if err := e.writeMergedComparisonRowsForFeed(outDir, name, group); err != nil {
			return err
		}
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
	path := filepath.Join(outDir, name+"_comparison.json")
	return writeFileAtomicAt(path, append(data, '\n'), generatedFileMode, e.feedProcessingTimestamp(name))
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
