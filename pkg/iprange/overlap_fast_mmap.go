//go:build linux || darwin

package iprange

import (
	"context"
	"unsafe"
)

func overlapCountFastPathPlatform(ctx context.Context, left, right RangeSource) (uint64, bool, error) {
	switch l := left.(type) {
	case *mmapFileSet:
		switch r := right.(type) {
		case *mmapFileSet:
			count, err := overlapCountMmapPair(ctx, l, r)
			return count, true, err
		case *IPSet:
			r.Optimize()
			count, err := overlapCountMmapAndRanges(ctx, l, r.Ranges)
			return count, true, err
		default:
			return 0, false, nil
		}
	case *IPSet:
		r, ok := right.(*mmapFileSet)
		if !ok {
			return 0, false, nil
		}
		l.Optimize()
		count, err := overlapCountRangesAndMmap(ctx, l.Ranges, r)
		return count, true, err
	default:
		return 0, false, nil
	}
}

func overlapCountRangesAndMmap(ctx context.Context, left []Range, right *mmapFileSet) (uint64, error) {
	if err := right.lockFastReader(); err != nil {
		return 0, err
	}
	defer right.mu.RUnlock()
	return overlapCountRangesAndBytesContext(ctx, left, right.rangesData)
}

func overlapCountMmapAndRanges(ctx context.Context, left *mmapFileSet, right []Range) (uint64, error) {
	if err := left.lockFastReader(); err != nil {
		return 0, err
	}
	defer left.mu.RUnlock()
	return overlapCountBytesAndRangesContext(ctx, left.rangesData, right)
}

func overlapCountMmapPair(ctx context.Context, left, right *mmapFileSet) (uint64, error) {
	unlock, err := lockMmapPair(left, right)
	if err != nil {
		return 0, err
	}
	defer unlock()
	return overlapCountRangeBytesContext(ctx, left.rangesData, right.rangesData)
}

func lockMmapPair(left, right *mmapFileSet) (func(), error) {
	if left == right {
		if err := left.lockFastReader(); err != nil {
			return nil, err
		}
		return func() { left.mu.RUnlock() }, nil
	}

	first, second := left, right
	if uintptr(unsafe.Pointer(second)) < uintptr(unsafe.Pointer(first)) {
		first, second = second, first
	}

	if err := first.lockFastReader(); err != nil {
		return nil, err
	}
	if err := second.lockFastReader(); err != nil {
		first.mu.RUnlock()
		return nil, err
	}
	return func() {
		second.mu.RUnlock()
		first.mu.RUnlock()
	}, nil
}
