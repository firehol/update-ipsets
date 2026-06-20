package iprange

import (
	"container/heap"
	"context"
)

const materializeContextCheckEvery = 4096

// UnionSourcesContext materializes the union of sorted range sources as an
// optimized IPSet. Known package-owned source types use direct indexed scans;
// arbitrary RangeSource implementations fall back to UnionIter.
func UnionSourcesContext(ctx context.Context, name string, sources ...RangeSource) (*IPSet, error) {
	set, _, err := UnionSourcesWithStatsContext(ctx, name, sources...)
	return set, err
}

func UnionSourcesWithStatsContext(ctx context.Context, name string, sources ...RangeSource) (*IPSet, OperationStats, error) {
	ctx = rangeSourceContext(ctx)
	stats := sourceInputStats(sources...)
	if set, ok, err := unionSourcesIndexed(ctx, name, sources); ok {
		if set != nil {
			stats.RangesEmitted = int64(len(set.Ranges))
		}
		return set, stats, err
	}
	set, err := CollectIterContext(ctx, name, UnionIter(sources...))
	if set != nil {
		stats.RangesEmitted = int64(len(set.Ranges))
	}
	return set, stats, err
}

// IntersectSourcesContext materializes the intersection of two sorted range
// sources as an optimized IPSet. Known package-owned source types use direct
// indexed scans; arbitrary RangeSource implementations fall back to IntersectIter.
func IntersectSourcesContext(ctx context.Context, name string, a, b RangeSource) (*IPSet, error) {
	set, _, err := IntersectSourcesWithStatsContext(ctx, name, a, b)
	return set, err
}

func IntersectSourcesWithStatsContext(ctx context.Context, name string, a, b RangeSource) (*IPSet, OperationStats, error) {
	ctx = rangeSourceContext(ctx)
	stats := sourceInputStats(a, b)
	if set, ok, err := intersectSourcesIndexed(ctx, name, a, b); ok {
		if set != nil {
			stats.RangesEmitted = int64(len(set.Ranges))
		}
		return set, stats, err
	}
	set, err := CollectIterContext(ctx, name, IntersectIter(a, b))
	if set != nil {
		stats.RangesEmitted = int64(len(set.Ranges))
	}
	return set, stats, err
}

// ExcludeSourcesContext materializes a\b as an optimized IPSet. Known
// package-owned source types use direct indexed scans; arbitrary RangeSource
// implementations fall back to ExcludeIter.
func ExcludeSourcesContext(ctx context.Context, name string, a, b RangeSource) (*IPSet, error) {
	set, _, err := ExcludeSourcesWithStatsContext(ctx, name, a, b)
	return set, err
}

func ExcludeSourcesWithStatsContext(ctx context.Context, name string, a, b RangeSource) (*IPSet, OperationStats, error) {
	ctx = rangeSourceContext(ctx)
	stats := sourceInputStats(a, b)
	if set, ok, err := excludeSourcesIndexed(ctx, name, a, b); ok {
		if set != nil {
			stats.RangesEmitted = int64(len(set.Ranges))
		}
		return set, stats, err
	}
	set, err := CollectIterContext(ctx, name, ExcludeIter(a, b))
	if set != nil {
		stats.RangesEmitted = int64(len(set.Ranges))
	}
	return set, stats, err
}

// ExcludeCountContext counts unique IPs in a\b without materializing the
// resulting ranges.
func ExcludeCountContext(ctx context.Context, a, b RangeSource) (uint64, error) {
	count, _, err := ExcludeCountWithStatsContext(ctx, a, b)
	return count, err
}

func ExcludeCountWithStatsContext(ctx context.Context, a, b RangeSource) (uint64, OperationStats, error) {
	ctx = rangeSourceContext(ctx)
	stats := sourceInputStats(a, b)
	var total uint64
	addRange := func(r Range) bool {
		total += r.Size()
		stats.RangesEmitted++
		return true
	}
	if ok, err := excludeRangesIndexed(ctx, a, b, addRange); ok {
		return total, stats, err
	}
	for r := range ExcludeIter(a, b) {
		if err := ctx.Err(); err != nil {
			return total, stats, err
		}
		total += r.Size()
		stats.RangesEmitted++
	}
	if err := firstRangeSourceErr(a, b); err != nil {
		return total, stats, err
	}
	return total, stats, ctx.Err()
}

// ExcludeRangesContext walks ranges in a\b without forcing callers through
// iter.Pull. Returning false from yield stops the scan early.
func ExcludeRangesContext(ctx context.Context, a, b RangeSource, yield func(Range) bool) error {
	ctx = rangeSourceContext(ctx)
	if yield == nil {
		return ctx.Err()
	}
	if ok, err := excludeRangesIndexed(ctx, a, b, yield); ok {
		return err
	}
	for r := range ExcludeIter(a, b) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !yield(r) {
			break
		}
	}
	if err := firstRangeSourceErr(a, b); err != nil {
		return err
	}
	return ctx.Err()
}

func sourceInputStats(sources ...RangeSource) OperationStats {
	stats := OperationStats{Sources: int64(len(sources))}
	for _, src := range sources {
		if src == nil {
			continue
		}
		n := int64(src.Len())
		stats.RangesRead += n
		stats.RangesScanned += n
	}
	return stats
}

func unionSourcesIndexed(ctx context.Context, name string, sources []RangeSource) (*IPSet, bool, error) {
	indexed, unlock, ok, err := indexedRangeSources(sources)
	if !ok || err != nil {
		return nil, ok, err
	}
	defer unlock()

	if err := ctx.Err(); err != nil {
		return nil, true, err
	}
	switch len(indexed) {
	case 0:
		return newMaterializedSet(name, 0), true, nil
	case 1:
		return materializeSingleIndexed(ctx, name, indexed[0])
	case 2:
		return unionTwoIndexed(ctx, name, indexed[0], indexed[1])
	default:
		return unionKWayIndexed(ctx, name, indexed)
	}
}

func intersectSourcesIndexed(ctx context.Context, name string, a, b RangeSource) (*IPSet, bool, error) {
	indexed, unlock, ok, err := indexedRangeSources([]RangeSource{a, b})
	if !ok || err != nil {
		return nil, ok, err
	}
	defer unlock()

	if err := ctx.Err(); err != nil {
		return nil, true, err
	}
	left, right := indexed[0], indexed[1]
	out := newMaterializedSet(name, min(left.len(), right.len()))
	i, j := 0, 0
	steps := 0
	for i < left.len() && j < right.len() {
		steps++
		if steps&(materializeContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return nil, true, err
			}
		}
		ra, err := left.at(i)
		if err != nil {
			return nil, true, err
		}
		rb, err := right.at(j)
		if err != nil {
			return nil, true, err
		}
		if ra.Hi < rb.Lo {
			i++
			continue
		}
		if rb.Hi < ra.Lo {
			j++
			continue
		}
		appendMaterializedRange(out, Range{Lo: max(ra.Lo, rb.Lo), Hi: min(ra.Hi, rb.Hi)})
		switch {
		case ra.Hi < rb.Hi:
			i++
		case rb.Hi < ra.Hi:
			j++
		default:
			i++
			j++
		}
	}
	return out, true, ctx.Err()
}

func excludeSourcesIndexed(ctx context.Context, name string, a, b RangeSource) (*IPSet, bool, error) {
	indexed, unlock, ok, err := indexedRangeSources([]RangeSource{a, b})
	if !ok || err != nil {
		return nil, ok, err
	}
	defer unlock()

	out := newMaterializedSet(name, indexed[0].len())
	err = excludeIndexed(ctx, indexed[0], indexed[1], func(r Range) bool {
		appendMaterializedRange(out, r)
		return true
	})
	return out, true, err
}

func excludeRangesIndexed(ctx context.Context, a, b RangeSource, yield func(Range) bool) (bool, error) {
	indexed, unlock, ok, err := indexedRangeSources([]RangeSource{a, b})
	if !ok || err != nil {
		return ok, err
	}
	defer unlock()

	return true, excludeIndexed(ctx, indexed[0], indexed[1], yield)
}

func excludeIndexed(ctx context.Context, left, right indexedRangeSource, yield func(Range) bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	j := 0
	steps := 0
	for i := 0; i < left.len(); i++ {
		steps++
		if steps&(materializeContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		ra, err := left.at(i)
		if err != nil {
			return err
		}
		for j < right.len() {
			rb, err := right.at(j)
			if err != nil {
				return err
			}
			if rb.Hi >= ra.Lo {
				break
			}
			j++
		}
		curLo := ra.Lo
		consumed := false
		for j < right.len() {
			rb, err := right.at(j)
			if err != nil {
				return err
			}
			if rb.Lo > ra.Hi {
				break
			}
			if curLo < rb.Lo {
				if !yield(Range{Lo: curLo, Hi: rb.Lo - 1}) {
					return ctx.Err()
				}
			}
			if rb.Hi >= ra.Hi {
				consumed = true
				break
			}
			curLo = rb.Hi + 1
			j++
		}
		if !consumed {
			if !yield(Range{Lo: curLo, Hi: ra.Hi}) {
				return ctx.Err()
			}
		}
	}
	return ctx.Err()
}

func materializeSingleIndexed(ctx context.Context, name string, src indexedRangeSource) (*IPSet, bool, error) {
	out := newMaterializedSet(name, src.len())
	if unique, ok := src.uniqueCount(); ok {
		out.UniqueIPs = unique
	}
	for i := 0; i < src.len(); i++ {
		if i&(materializeContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return nil, true, err
			}
		}
		r, err := src.at(i)
		if err != nil {
			return nil, true, err
		}
		out.Ranges = append(out.Ranges, r)
		out.Lines++
		if !src.knownUnique {
			out.UniqueIPs += r.Size()
		}
	}
	return out, true, ctx.Err()
}

func unionTwoIndexed(ctx context.Context, name string, a, b indexedRangeSource) (*IPSet, bool, error) {
	out := newMaterializedSet(name, a.len()+b.len())
	i, j := 0, 0
	var cur Range
	haveCur := false
	steps := 0
	for i < a.len() || j < b.len() {
		steps++
		if steps&(materializeContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return nil, true, err
			}
		}
		next, err := nextUnionIndexedRange(a, b, &i, &j)
		if err != nil {
			return nil, true, err
		}
		if !haveCur {
			cur = next
			haveCur = true
			continue
		}
		if canMerge(cur, next) {
			if next.Hi > cur.Hi {
				cur.Hi = next.Hi
			}
			continue
		}
		appendMaterializedRange(out, cur)
		cur = next
	}
	if haveCur {
		appendMaterializedRange(out, cur)
	}
	return out, true, ctx.Err()
}

func nextUnionIndexedRange(a, b indexedRangeSource, i, j *int) (Range, error) {
	if *i < a.len() && *j < b.len() {
		ra, err := a.at(*i)
		if err != nil {
			return Range{}, err
		}
		rb, err := b.at(*j)
		if err != nil {
			return Range{}, err
		}
		if ra.Lo <= rb.Lo {
			(*i)++
			return ra, nil
		}
		(*j)++
		return rb, nil
	}
	if *i < a.len() {
		r, err := a.at(*i)
		(*i)++
		return r, err
	}
	r, err := b.at(*j)
	(*j)++
	return r, err
}

func unionKWayIndexed(ctx context.Context, name string, sources []indexedRangeSource) (*IPSet, bool, error) {
	totalRanges := 0
	for _, src := range sources {
		totalRanges += src.len()
	}
	out := newMaterializedSet(name, totalRanges)
	h := make(indexedMergeHeap, 0, len(sources))
	for sourceID, src := range sources {
		if src.len() == 0 {
			continue
		}
		r, err := src.at(0)
		if err != nil {
			return nil, true, err
		}
		h = append(h, indexedMergeEntry{r: r, sourceID: sourceID, nextIndex: 1})
	}
	heap.Init(&h)
	var cur Range
	haveCur := false
	steps := 0
	for h.Len() > 0 {
		steps++
		if steps&(materializeContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return nil, true, err
			}
		}
		entry := heap.Pop(&h).(indexedMergeEntry)
		next := entry.r
		if !haveCur {
			cur = next
			haveCur = true
		} else if canMerge(cur, next) {
			if next.Hi > cur.Hi {
				cur.Hi = next.Hi
			}
		} else {
			appendMaterializedRange(out, cur)
			cur = next
		}
		src := sources[entry.sourceID]
		if entry.nextIndex < src.len() {
			r, err := src.at(entry.nextIndex)
			if err != nil {
				return nil, true, err
			}
			heap.Push(&h, indexedMergeEntry{
				r:         r,
				sourceID:  entry.sourceID,
				nextIndex: entry.nextIndex + 1,
			})
		}
	}
	if haveCur {
		appendMaterializedRange(out, cur)
	}
	return out, true, ctx.Err()
}

type indexedMergeHeap []indexedMergeEntry

type indexedMergeEntry struct {
	r         Range
	sourceID  int
	nextIndex int
}

func (h indexedMergeHeap) Len() int { return len(h) }

func (h indexedMergeHeap) Less(i, j int) bool {
	if h[i].r.Lo != h[j].r.Lo {
		return h[i].r.Lo < h[j].r.Lo
	}
	return h[i].r.Hi < h[j].r.Hi
}

func (h indexedMergeHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *indexedMergeHeap) Push(x any)   { *h = append(*h, x.(indexedMergeEntry)) }

func (h *indexedMergeHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func newMaterializedSet(name string, capacity int) *IPSet {
	set := New(name)
	set.Ranges = make([]Range, 0, max(0, capacity))
	set.Optimized = true
	return set
}

func appendMaterializedRange(set *IPSet, r Range) {
	set.Ranges = append(set.Ranges, r)
	set.Lines++
	set.UniqueIPs += r.Size()
	set.Optimized = true
}
