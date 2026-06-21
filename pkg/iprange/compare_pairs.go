package iprange

import (
	"context"
	"fmt"
	"sort"
)

// ComparePair selects two sources from the CompareSource slice passed to
// CompareSourcePairs.
type ComparePair struct {
	Left  int
	Right int
}

// CompareSourcePairs compares the selected source pairs in order. It accepts
// streaming RangeSource inputs and uses an indexed one-to-many path for IPSet
// and FileSet sources so callers can batch many comparisons through pkg/iprange
// instead of running engine-local pair loops.
func CompareSourcePairs(ctx context.Context, sources []CompareSource, pairs []ComparePair) ([]CompareRow, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(pairs) == 0 {
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	prepared, err := prepareCompareSources(ctx, sources)
	if err != nil {
		return nil, err
	}
	if err := validateComparePairs(len(prepared), pairs); err != nil {
		return nil, err
	}

	rangeSources := make([]RangeSource, len(prepared))
	for i := range prepared {
		rangeSources[i] = prepared[i].source
	}
	indexed, unlock, ok, err := indexedRangeSources(rangeSources)
	if err != nil {
		return nil, err
	}
	if ok {
		defer unlock()
		rows, err := compareSourcePairsIndexed(ctx, prepared, indexed, pairs)
		if err != nil {
			return nil, err
		}
		return rows, compareSourcePairErrors(prepared)
	}
	return compareSourcePairsIter(ctx, prepared, pairs)
}

func validateComparePairs(sourceCount int, pairs []ComparePair) error {
	for i, pair := range pairs {
		if pair.Left < 0 || pair.Left >= sourceCount {
			return fmt.Errorf("compare pair %d left index %d out of range [0, %d)", i, pair.Left, sourceCount)
		}
		if pair.Right < 0 || pair.Right >= sourceCount {
			return fmt.Errorf("compare pair %d right index %d out of range [0, %d)", i, pair.Right, sourceCount)
		}
		if pair.Left == pair.Right {
			return fmt.Errorf("compare pair %d compares source %d with itself", i, pair.Left)
		}
	}
	return nil
}

func compareSourcePairsIter(ctx context.Context, sources []compareSourceMeta, pairs []ComparePair) ([]CompareRow, error) {
	rows := make([]CompareRow, len(pairs))
	for i, pair := range pairs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		left := sources[pair.Left]
		right := sources[pair.Right]
		common, err := OverlapCountIterContext(ctx, left.source, right.source)
		if err != nil {
			return nil, err
		}
		if err := RangeSourceErr(left.source); err != nil {
			return nil, fmt.Errorf("compare %s: %w", left.name, err)
		}
		if err := RangeSourceErr(right.source); err != nil {
			return nil, fmt.Errorf("compare %s: %w", right.name, err)
		}
		rows[i] = compareRowFromMeta(left, right, common)
	}
	return rows, nil
}

func compareSourcePairsIndexed(ctx context.Context, sources []compareSourceMeta, indexed []indexedRangeSource, pairs []ComparePair) ([]CompareRow, error) {
	rows := make([]CompareRow, len(pairs))
	for i, pair := range pairs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		common, err := overlapCountIndexedSources(ctx, indexed[pair.Left], indexed[pair.Right])
		if err != nil {
			return nil, err
		}
		rows[i] = compareRowFromMeta(sources[pair.Left], sources[pair.Right], common)
	}
	return rows, nil
}

func overlapCountIndexedSources(ctx context.Context, left, right indexedRangeSource) (uint64, error) {
	switch {
	case left.ranges != nil && right.ranges != nil:
		return overlapCountRangesContext(ctx, left.ranges, right.ranges)
	case left.ranges != nil && right.bytes != nil:
		return overlapCountRangesAndBytesContext(ctx, left.ranges, right.bytes)
	case left.bytes != nil && right.ranges != nil:
		return overlapCountBytesAndRangesContext(ctx, left.bytes, right.ranges)
	case left.bytes != nil && right.bytes != nil:
		return overlapCountRangeBytesContext(ctx, left.bytes, right.bytes)
	default:
		return overlapCountIndexedAtSources(ctx, left, right)
	}
}

func overlapCountIndexedAtSources(ctx context.Context, left, right indexedRangeSource) (uint64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	var count uint64
	i, j := 0, 0
	steps := 0
	for i < left.len() && j < right.len() {
		steps++
		if steps&(overlapContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return count, err
			}
		}
		ra, err := left.at(i)
		if err != nil {
			return count, err
		}
		rb, err := right.at(j)
		if err != nil {
			return count, err
		}
		if ra.Hi < rb.Lo {
			i++
			continue
		}
		if rb.Hi < ra.Lo {
			j++
			continue
		}
		lo := ra.Lo
		if rb.Lo > lo {
			lo = rb.Lo
		}
		hi := ra.Hi
		if rb.Hi < hi {
			hi = rb.Hi
		}
		count += uint64(hi) - uint64(lo) + 1
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
	return count, ctx.Err()
}

type comparePairGroup struct {
	left    int
	targets []comparePairTarget
}

type comparePairTarget struct {
	right int
	row   int
}

func comparePairsByLeft(pairs []ComparePair) []comparePairGroup {
	byLeft := make(map[int][]comparePairTarget)
	for row, pair := range pairs {
		byLeft[pair.Left] = append(byLeft[pair.Left], comparePairTarget{right: pair.Right, row: row})
	}
	lefts := make([]int, 0, len(byLeft))
	for left := range byLeft {
		lefts = append(lefts, left)
	}
	sort.Ints(lefts)
	groups := make([]comparePairGroup, 0, len(lefts))
	for _, left := range lefts {
		groups = append(groups, comparePairGroup{left: left, targets: byLeft[left]})
	}
	return groups
}

type oneToManyCursor struct {
	target comparePairTarget
	source indexedRangeSource
	index  int
	r      Range
	ok     bool
}

type oneToManyCursorHeap []oneToManyCursor

func (h oneToManyCursorHeap) Len() int { return len(h) }
func (h oneToManyCursorHeap) less(i, j int) bool {
	if h[i].r.Lo != h[j].r.Lo {
		return h[i].r.Lo < h[j].r.Lo
	}
	if h[i].r.Hi != h[j].r.Hi {
		return h[i].r.Hi < h[j].r.Hi
	}
	return h[i].target.row < h[j].target.row
}

func (h oneToManyCursorHeap) swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *oneToManyCursorHeap) init() {
	for i := h.Len()/2 - 1; i >= 0; i-- {
		h.down(i)
	}
}

func (h *oneToManyCursorHeap) push(cursor oneToManyCursor) {
	*h = append(*h, cursor)
	h.up(h.Len() - 1)
}

func (h *oneToManyCursorHeap) pop() oneToManyCursor {
	old := *h
	n := len(old) - 1
	out := old[0]
	if n == 0 {
		old[0] = oneToManyCursor{}
		*h = old[:0]
		return out
	}
	old[0] = old[n]
	old[n] = oneToManyCursor{}
	*h = old[:n]
	h.down(0)
	return out
}

func (h oneToManyCursorHeap) up(j int) {
	for {
		i := (j - 1) / 2
		if i == j || !h.less(j, i) {
			return
		}
		h.swap(i, j)
		j = i
	}
}

func (h oneToManyCursorHeap) down(i int) {
	n := h.Len()
	for {
		left := 2*i + 1
		if left >= n {
			return
		}
		child := left
		if right := left + 1; right < n && h.less(right, left) {
			child = right
		}
		if !h.less(child, i) {
			return
		}
		h.swap(i, child)
		i = child
	}
}

func overlapOneToManyIndexed(ctx context.Context, left indexedRangeSource, all []indexedRangeSource, targets []comparePairTarget, commonByRow []uint64) error {
	pending := make(oneToManyCursorHeap, 0, len(targets))
	for _, target := range targets {
		cursor, err := newOneToManyCursor(target, all[target.right])
		if err != nil {
			return err
		}
		if cursor.ok {
			pending = append(pending, cursor)
		}
	}
	pending.init()
	active := make([]oneToManyCursor, 0, len(targets))

	for i := 0; i < left.len(); i++ {
		if i&(overlapContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		leftRange, err := left.at(i)
		if err != nil {
			return err
		}
		var errActive error
		active = processActiveOneToManyCursors(ctx, active, leftRange, commonByRow, &pending, &errActive)
		if errActive != nil {
			return errActive
		}
		if err := processPendingOneToManyCursors(ctx, &pending, leftRange, commonByRow, &active); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func newOneToManyCursor(target comparePairTarget, source indexedRangeSource) (oneToManyCursor, error) {
	cursor := oneToManyCursor{target: target, source: source}
	if source.len() == 0 {
		return cursor, nil
	}
	r, err := source.at(0)
	if err != nil {
		return cursor, err
	}
	cursor.r = r
	cursor.ok = true
	return cursor, nil
}

func processActiveOneToManyCursors(ctx context.Context, active []oneToManyCursor, left Range, commonByRow []uint64, pending *oneToManyCursorHeap, firstErr *error) []oneToManyCursor {
	nextActive := active[:0]
	for _, cursor := range active {
		if *firstErr != nil {
			continue
		}
		var err error
		cursor, err = advanceCursorUntilPossibleOverlap(cursor, left.Lo)
		if err != nil {
			*firstErr = err
			continue
		}
		if !cursor.ok {
			continue
		}
		cursor, err = consumeCursorForLeftRange(ctx, cursor, left, commonByRow, pending, &nextActive)
		if err != nil {
			*firstErr = err
			continue
		}
	}
	return nextActive
}

func processPendingOneToManyCursors(ctx context.Context, pending *oneToManyCursorHeap, left Range, commonByRow []uint64, active *[]oneToManyCursor) error {
	for pending.Len() > 0 {
		cursor := pending.pop()
		var err error
		cursor, err = advanceCursorUntilPossibleOverlap(cursor, left.Lo)
		if err != nil {
			return err
		}
		if !cursor.ok {
			continue
		}
		if cursor.r.Lo > left.Hi {
			pending.push(cursor)
			if (*pending)[0].r.Lo > left.Hi {
				return nil
			}
			continue
		}
		if _, err := consumeCursorForLeftRange(ctx, cursor, left, commonByRow, pending, active); err != nil {
			return err
		}
	}
	return nil
}

func consumeCursorForLeftRange(ctx context.Context, cursor oneToManyCursor, left Range, commonByRow []uint64, pending *oneToManyCursorHeap, active *[]oneToManyCursor) (oneToManyCursor, error) {
	for cursor.ok && cursor.r.Lo <= left.Hi {
		if err := ctx.Err(); err != nil {
			return cursor, err
		}
		addOverlapToCompareRow(cursor.target.row, left, cursor.r, commonByRow)
		if cursor.r.Hi > left.Hi {
			*active = append(*active, cursor)
			return oneToManyCursor{}, nil
		}
		next, err := advanceOneToManyCursor(cursor)
		if err != nil {
			return cursor, err
		}
		cursor = next
		cursor, err = advanceCursorUntilPossibleOverlap(cursor, left.Lo)
		if err != nil {
			return cursor, err
		}
	}
	if cursor.ok {
		pending.push(cursor)
	}
	return cursor, nil
}

func advanceCursorUntilPossibleOverlap(cursor oneToManyCursor, minLo uint32) (oneToManyCursor, error) {
	for cursor.ok && cursor.r.Hi < minLo {
		next, err := advanceOneToManyCursor(cursor)
		if err != nil {
			return cursor, err
		}
		cursor = next
	}
	return cursor, nil
}

func advanceOneToManyCursor(cursor oneToManyCursor) (oneToManyCursor, error) {
	cursor.index++
	if cursor.index >= cursor.source.len() {
		cursor.ok = false
		return cursor, nil
	}
	r, err := cursor.source.at(cursor.index)
	if err != nil {
		return cursor, err
	}
	cursor.r = r
	cursor.ok = true
	return cursor, nil
}

func addOverlapToCompareRow(row int, left, right Range, commonByRow []uint64) {
	lo := left.Lo
	if right.Lo > lo {
		lo = right.Lo
	}
	hi := left.Hi
	if right.Hi < hi {
		hi = right.Hi
	}
	if lo <= hi {
		commonByRow[row] += uint64(hi) - uint64(lo) + 1
	}
}

func compareRowFromMeta(left, right compareSourceMeta, common uint64) CompareRow {
	return CompareRow{
		Name1:       left.name,
		Name2:       right.name,
		Entries1:    left.entries,
		Entries2:    right.entries,
		Unique1:     left.uniqueIPs,
		Unique2:     right.uniqueIPs,
		CombinedIPs: left.uniqueIPs + right.uniqueIPs - common,
		CommonIPs:   common,
	}
}

func compareSourcePairErrors(sources []compareSourceMeta) error {
	for _, src := range sources {
		if err := RangeSourceErr(src.source); err != nil {
			return fmt.Errorf("compare %s: %w", src.name, err)
		}
	}
	return nil
}
