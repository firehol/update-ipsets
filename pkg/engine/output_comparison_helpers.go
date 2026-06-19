package engine

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
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
	return rangeSourceBounds(src.RangeSource)
}

const (
	comparisonPrefixBits        = 20
	comparisonPrefixShift       = 32 - comparisonPrefixBits
	comparisonPrefixWords       = 1 << (comparisonPrefixBits - 6)
	comparisonSparsePrefixBits  = 24
	comparisonSparsePrefixShift = 32 - comparisonSparsePrefixBits
	comparisonSparsePrefixLimit = 8192
)

type comparisonPrefixBitmap [comparisonPrefixWords]uint64

type comparisonContentHash struct {
	sum   [sha256.Size]byte
	valid bool
}

type comparisonSetSignature struct {
	prefixBitmap *comparisonPrefixBitmap
	sparsePrefix *comparisonSparsePrefixSet
	contentHash  comparisonContentHash
}

type comparisonSparsePrefixSet struct {
	prefixes []uint32
}

type comparisonSparsePrefixBuilder struct {
	prefixes []uint32
	last     uint32
	haveLast bool
	overflow bool
}

type rangeOverlapFilter struct {
	lo           uint32
	hi           uint32
	valid        bool
	hasRange     bool
	prefixBitmap *comparisonPrefixBitmap
	sparsePrefix *comparisonSparsePrefixSet
}

func buildRangeOverlapFilter(src iprange.RangeSource) rangeOverlapFilter {
	signature := buildComparisonSetSignature(src)
	lo, hi, hasRange := rangeSourceBounds(src)
	return rangeOverlapFilter{
		lo:           lo,
		hi:           hi,
		valid:        true,
		hasRange:     hasRange,
		prefixBitmap: signature.prefixBitmap,
		sparsePrefix: signature.sparsePrefix,
	}
}

func rangeSourceBounds(src iprange.RangeSource) (uint32, uint32, bool) {
	if src == nil {
		return 0, 0, false
	}
	switch set := src.(type) {
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
		first := true
		var lo, hi uint32
		for r := range set.Iter() {
			if first {
				lo = r.Lo
				first = false
			}
			hi = r.Hi
		}
		return lo, hi, !first
	}
}

func rangeOverlapFiltersDisjoint(a, b rangeOverlapFilter) bool {
	if !a.valid || !b.valid {
		return false
	}
	// Valid filters with no ranges are known-empty sources. Invalid filters are
	// handled above and must fall through to exact counting.
	if !a.hasRange || !b.hasRange {
		return true
	}
	if a.hi < b.lo || b.hi < a.lo {
		return true
	}
	if !comparisonSparsePrefixOverlap(a.sparsePrefix, b.sparsePrefix) {
		return true
	}
	return !comparisonPrefixOverlap(a.prefixBitmap, b.prefixBitmap)
}

func buildComparisonSetSignature(src iprange.RangeSource) comparisonSetSignature {
	signature, _ := buildComparisonSetSignatureContext(context.Background(), src)
	return signature
}

func buildComparisonSetSignatureContext(ctx context.Context, src iprange.RangeSource) (comparisonSetSignature, error) {
	ctx = nonNilContext(ctx)
	if src == nil {
		return comparisonSetSignature{}, nil
	}
	var bitmap comparisonPrefixBitmap
	hasRanges := false
	hasher := sha256.New()
	var hashBuf [8]byte
	sparse := comparisonSparsePrefixBuilder{}
	for r := range src.Iter() {
		if err := contextErr(ctx); err != nil {
			return comparisonSetSignature{}, err
		}
		hasRanges = true
		binary.BigEndian.PutUint32(hashBuf[0:4], r.Lo)
		binary.BigEndian.PutUint32(hashBuf[4:8], r.Hi)
		_, _ = hasher.Write(hashBuf[:])
		start := r.Lo >> comparisonPrefixShift
		end := r.Hi >> comparisonPrefixShift
		for prefix := start; prefix <= end; prefix++ {
			bitmap[prefix>>6] |= uint64(1) << (prefix & 63)
		}
		sparse.addRange(r.Lo>>comparisonSparsePrefixShift, r.Hi>>comparisonSparsePrefixShift)
	}
	if !hasRanges {
		return comparisonSetSignature{}, nil
	}
	sum := hasher.Sum(nil)
	var contentHash comparisonContentHash
	copy(contentHash.sum[:], sum)
	contentHash.valid = true
	return comparisonSetSignature{
		prefixBitmap: &bitmap,
		sparsePrefix: sparse.set(),
		contentHash:  contentHash,
	}, nil
}

func buildComparisonPrefixBitmap(src iprange.RangeSource) *comparisonPrefixBitmap {
	return buildComparisonSetSignature(src).prefixBitmap
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

func (b *comparisonSparsePrefixBuilder) addRange(start, end uint32) {
	if b.overflow {
		return
	}
	if b.haveLast && start <= b.last {
		if end <= b.last {
			return
		}
		start = b.last + 1
	}
	count := uint64(end) - uint64(start) + 1
	if uint64(len(b.prefixes))+count > comparisonSparsePrefixLimit {
		b.prefixes = nil
		b.overflow = true
		return
	}
	for prefix := start; prefix <= end; prefix++ {
		b.prefixes = append(b.prefixes, prefix)
		if prefix == end {
			break
		}
	}
	b.last = end
	b.haveLast = true
}

func (b *comparisonSparsePrefixBuilder) set() *comparisonSparsePrefixSet {
	if b == nil || b.overflow || len(b.prefixes) == 0 {
		return nil
	}
	return &comparisonSparsePrefixSet{prefixes: b.prefixes}
}

func comparisonSparsePrefixOverlap(a, b *comparisonSparsePrefixSet) bool {
	if a == nil || b == nil {
		return true
	}
	i, j := 0, 0
	for i < len(a.prefixes) && j < len(b.prefixes) {
		switch {
		case a.prefixes[i] == b.prefixes[j]:
			return true
		case a.prefixes[i] < b.prefixes[j]:
			i++
		default:
			j++
		}
	}
	return false
}

func comparisonSetsIdentical(a, b comparisonSetInfo) bool {
	return a.ips > 0 &&
		a.ips == b.ips &&
		a.contentHash.valid &&
		b.contentHash.valid &&
		a.contentHash.sum == b.contentHash.sum
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
