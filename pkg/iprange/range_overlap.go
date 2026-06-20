package iprange

import (
	"context"
	"fmt"
)

// RangeIndex provides indexed access to sorted, non-overlapping IPv4 ranges.
type RangeIndex interface {
	Len() int
	Range(int) (Range, error)
}

// RangeList adapts a sorted range slice to RangeIndex.
type RangeList []Range

func (r RangeList) Len() int {
	return len(r)
}

func (r RangeList) Range(i int) (Range, error) {
	if i < 0 || i >= len(r) {
		return Range{}, fmt.Errorf("range index %d out of bounds [0, %d)", i, len(r))
	}
	return r[i], nil
}

// RangeOverlap describes one overlap between a left RangeSource range and an
// indexed right-side range.
type RangeOverlap struct {
	Left       Range
	Right      Range
	RightIndex int
	Overlap    Range
}

// WalkRangesContext walks src ranges with context and source-error handling.
func WalkRangesContext(ctx context.Context, src RangeSource, yield func(Range) bool) error {
	ctx = rangeSourceContext(ctx)
	if src == nil || yield == nil {
		return ctx.Err()
	}
	if indexed, unlock, ok, err := indexedRangeSources([]RangeSource{src}); ok {
		if unlock != nil {
			defer unlock()
		}
		if err != nil {
			return err
		}
		return walkIndexedRangesContext(ctx, indexed[0], src, yield)
	}

	steps := 0
	for r := range src.Iter() {
		steps++
		if steps&(materializeContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		if !yield(r) {
			break
		}
	}
	if err := RangeSourceErr(src); err != nil {
		return err
	}
	return ctx.Err()
}

func walkIndexedRangesContext(ctx context.Context, src indexedRangeSource, original RangeSource, yield func(Range) bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for i := 0; i < src.len(); i++ {
		if i&(materializeContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		r, err := src.at(i)
		if err != nil {
			return err
		}
		if !yield(r) {
			break
		}
	}
	if err := RangeSourceErr(original); err != nil {
		return err
	}
	return ctx.Err()
}

// WalkRangeOverlapsContext walks overlaps between sorted src ranges and sorted
// indexed right-side ranges. The right index is returned so callers can keep
// domain metadata outside pkg/iprange.
func WalkRangeOverlapsContext(ctx context.Context, src RangeSource, right RangeIndex, yield func(RangeOverlap) bool) error {
	ctx = rangeSourceContext(ctx)
	if src == nil || right == nil || yield == nil || right.Len() == 0 {
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if indexed, unlock, ok, err := indexedRangeSources([]RangeSource{src}); ok {
		if unlock != nil {
			defer unlock()
		}
		if err != nil {
			return err
		}
		return walkIndexedRangeOverlapsContext(ctx, indexed[0], src, right, yield)
	}

	rightIndex := 0
	rightLen := right.Len()
	steps := 0
	for left := range src.Iter() {
		steps++
		if steps&(materializeContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		nextIndex, keepGoing, err := walkOneRangeOverlaps(left, right, rightLen, rightIndex, yield)
		if err != nil {
			return err
		}
		rightIndex = nextIndex
		if !keepGoing || rightIndex >= rightLen {
			break
		}
	}
	if err := RangeSourceErr(src); err != nil {
		return err
	}
	return ctx.Err()
}

func walkIndexedRangeOverlapsContext(ctx context.Context, left indexedRangeSource, original RangeSource, right RangeIndex, yield func(RangeOverlap) bool) error {
	rightIndex := 0
	rightLen := right.Len()
	for i := 0; i < left.len() && rightIndex < rightLen; i++ {
		if i&(materializeContextCheckEvery-1) == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		leftRange, err := left.at(i)
		if err != nil {
			return err
		}
		nextIndex, keepGoing, err := walkOneRangeOverlaps(leftRange, right, rightLen, rightIndex, yield)
		if err != nil {
			return err
		}
		rightIndex = nextIndex
		if !keepGoing {
			break
		}
	}
	if err := RangeSourceErr(original); err != nil {
		return err
	}
	return ctx.Err()
}

func walkOneRangeOverlaps(left Range, right RangeIndex, rightLen, startIndex int, yield func(RangeOverlap) bool) (int, bool, error) {
	for startIndex < rightLen {
		rightRange, err := right.Range(startIndex)
		if err != nil {
			return startIndex, false, err
		}
		if rightRange.Hi >= left.Lo {
			break
		}
		startIndex++
	}

	idx := startIndex
	for idx < rightLen {
		rightRange, err := right.Range(idx)
		if err != nil {
			return startIndex, false, err
		}
		if left.Hi < rightRange.Lo {
			break
		}
		overlap := Range{Lo: max(left.Lo, rightRange.Lo), Hi: min(left.Hi, rightRange.Hi)}
		if !yield(RangeOverlap{
			Left:       left,
			Right:      rightRange,
			RightIndex: idx,
			Overlap:    overlap,
		}) {
			return startIndex, false, nil
		}
		if left.Hi <= rightRange.Hi {
			break
		}
		idx++
	}
	return idx, true, nil
}
