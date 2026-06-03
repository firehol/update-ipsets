package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func recordAtomicDuration(count, totalNS, maxNS *atomic.Int64, dur time.Duration) {
	if count == nil || totalNS == nil || maxNS == nil {
		return
	}
	if dur < 0 {
		dur = 0
	}
	ns := dur.Nanoseconds()
	count.Add(1)
	totalNS.Add(ns)
	for {
		current := maxNS.Load()
		if ns <= current {
			return
		}
		if maxNS.CompareAndSwap(current, ns) {
			return
		}
	}
}

func rangeBounds(src *closableSource) (uint32, uint32, bool) {
	if src == nil || src.RangeSource == nil {
		return 0, 0, false
	}
	switch set := src.RangeSource.(type) {
	case iprange.FileSet:
		if set.Len() == 0 {
			return 0, 0, false
		}
		first, err := set.Range(0)
		if err != nil {
			return 0, 0, false
		}
		last, err := set.Range(set.Len() - 1)
		if err != nil {
			return 0, 0, false
		}
		return first.Lo, last.Hi, true
	case *iprange.IPSet:
		set.Optimize()
		if len(set.Ranges) == 0 {
			return 0, 0, false
		}
		return set.Ranges[0].Lo, set.Ranges[len(set.Ranges)-1].Hi, true
	default:
		return 0, 0, false
	}
}

const (
	comparisonPrefixBits  = 20
	comparisonPrefixShift = 32 - comparisonPrefixBits
	comparisonPrefixWords = 1 << (comparisonPrefixBits - 6)
)

type comparisonPrefixBitmap [comparisonPrefixWords]uint64

func buildComparisonPrefixBitmap(src iprange.RangeSource) *comparisonPrefixBitmap {
	if src == nil {
		return nil
	}
	var bitmap comparisonPrefixBitmap
	hasRanges := false
	for r := range src.Iter() {
		hasRanges = true
		start := r.Lo >> comparisonPrefixShift
		end := r.Hi >> comparisonPrefixShift
		for prefix := start; prefix <= end; prefix++ {
			bitmap[prefix>>6] |= uint64(1) << (prefix & 63)
		}
	}
	if !hasRanges {
		return nil
	}
	return &bitmap
}

func comparisonPrefixOverlap(a, b *comparisonPrefixBitmap) bool {
	if a == nil || b == nil {
		return true
	}
	for i := range a {
		if a[i]&b[i] != 0 {
			return true
		}
	}
	return false
}

// leafAncestors returns the set of positive leaf (primary, non-derivative)
// source names that a given feed ultimately derives from. It walks positive
// ancestry until it hits sources with no positive parents and collects their
// names. For signed merges, only additive parents are positive ancestry;
// subtractive parents remain dependencies but do not make comparison rows
// related.
//
// For a plain HTTP source X:               leafAncestors(X) = {X}
// For a retention variant X_1d of X:       leafAncestors(X_1d) = {X}
// For a merge M with sources [A, B] - [C]: leafAncestors(M) = {A, B}
//
//	(or the leaves of A and B if either is itself a
//	derivative — merges-of-merges are legal in the DAG model
//	and this walks through transitively)
//
// Used by writeComparisonFiles to mark "related" pairs: two feeds whose
// positive leaf-ancestor sets intersect share at least one primary ancestor,
// which means any overlap between them is trivially explained by shared positive
// ancestry rather than by genuine agreement between independent sources. The
// public UI's uniqueness / inclusion tiles skip these rows so the numbers
// reflect real cross-feed signal instead of derivative echo.
func leafAncestors(cfg *config.Config, name string) map[string]bool {
	result := map[string]bool{}
	if cfg == nil {
		return result
	}
	visited := map[string]bool{}
	var walk func(n string)
	walk = func(n string) {
		if visited[n] {
			return
		}
		visited[n] = true
		src := cfg.Sources[n]
		if src == nil {
			return
		}
		parents := positiveLineageParents(src)
		if len(parents) == 0 {
			result[n] = true
			return
		}
		for _, parent := range parents {
			walk(parent)
		}
	}
	walk(name)
	return result
}

func positiveLineageParents(src *config.Source) []string {
	if src == nil {
		return nil
	}
	if src.Provenance == config.ProvenanceSecondaryMerge || len(src.MergeSources) > 0 || len(src.MergeExclude) > 0 {
		return mergeSourceNames(src)
	}
	return src.DerivedFrom
}

// mergeCompareRows merges fresh comparison rows into the existing rows
// loaded from disk. Fresh positive-overlap rows replace any existing entry with
// the same counterpart Name; rows that only exist on disk are preserved. Fresh
// zero-overlap rows delete matching existing rows because absence is the public
// representation of "no overlap".
func mergeCompareRows(existing, fresh []CompareRow) []CompareRow {
	byName := make(map[string]CompareRow, len(existing)+len(fresh))
	for _, r := range existing {
		if r.Common == 0 {
			continue
		}
		byName[r.Name] = r
	}
	for _, r := range fresh {
		if r.Common == 0 {
			delete(byName, r.Name)
			continue
		}
		byName[r.Name] = r
	}
	out := make([]CompareRow, 0, len(byName))
	for _, r := range byName {
		out = append(out, r)
	}
	return out
}

func (e *Engine) sanitizeComparisonArtifacts(outDir string) error {
	liveOutDir := e.outputDir()
	paths := map[string]string{}
	collect := func(dir string) {
		if dir == "" {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) && e.logger != nil {
				e.logger.Warn("comparison sanitize: cannot read directory", "dir", dir, "error", err)
			}
			return
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, "_comparison.json") {
				continue
			}
			paths[name] = dir
		}
	}
	collect(liveOutDir)
	// Staged output wins over live output when both contain the same artifact.
	collect(outDir)

	var cleaned int64
	for rel, rootDir := range paths {
		data, changed, err := sanitizedComparisonArtifactData(rootDir, rel)
		if err != nil {
			if e.logger != nil {
				e.logger.Warn("comparison sanitize: cannot parse artifact", "file", filepath.Join(rootDir, rel), "error", err)
			}
			continue
		}
		if !changed {
			continue
		}
		if err := writeFileAtomic(filepath.Join(outDir, rel), data, generatedFileMode); err != nil {
			return err
		}
		cleaned++
	}
	if cleaned > 0 {
		e.observeRunCounter("metadata.comparison_zero_rows_removed", cleaned, 0)
	}
	return nil
}

func sanitizedComparisonArtifactData(rootDir, rel string) ([]byte, bool, error) {
	data, err := readFileInRoot(rootDir, rel)
	if err != nil {
		return nil, false, err
	}
	var rows []CompareRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, false, err
	}
	filtered := rows[:0]
	changed := false
	for _, row := range rows {
		if row.Common == 0 {
			changed = true
			continue
		}
		filtered = append(filtered, row)
	}
	if !changed {
		return nil, false, nil
	}
	out, err := jsonMarshalTabIndent(filtered)
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}
