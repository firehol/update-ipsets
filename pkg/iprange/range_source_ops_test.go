package iprange

import (
	"context"
	"errors"
	"testing"
)

func TestOverlapCountIterSourceCombinations(t *testing.T) {
	leftSet := setFromRanges("left",
		Range{Lo: 1, Hi: 10},
		Range{Lo: 20, Hi: 25},
		Range{Lo: 40, Hi: 50},
	)
	rightSet := setFromRanges("right",
		Range{Lo: 5, Hi: 22},
		Range{Lo: 30, Hi: 35},
		Range{Lo: 45, Hi: 55},
	)
	leftFile, err := OpenFileSet(writeTempSet(t, leftSet))
	if err != nil {
		t.Fatalf("OpenFileSet(left): %v", err)
	}
	defer func() { _ = leftFile.Close() }()
	rightFile, err := OpenFileSet(writeTempSet(t, rightSet))
	if err != nil {
		t.Fatalf("OpenFileSet(right): %v", err)
	}
	defer func() { _ = rightFile.Close() }()

	leftGeneric := RangeSourceFromIter(leftSet.Iter(), leftSet.Len())
	rightGeneric := RangeSourceFromIter(rightSet.Iter(), rightSet.Len())

	cases := []struct {
		name  string
		left  RangeSource
		right RangeSource
	}{
		{"memory_memory", leftSet, rightSet},
		{"fileset_fileset", leftFile, rightFile},
		{"memory_fileset", leftSet, rightFile},
		{"fileset_memory", leftFile, rightSet},
		{"generic_generic", leftGeneric, rightGeneric},
	}

	const want uint64 = 15
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := OverlapCountIterContext(t.Context(), tc.left, tc.right)
			if err != nil {
				t.Fatalf("OverlapCountIterContext() error = %v", err)
			}
			if got != want {
				t.Fatalf("OverlapCountIterContext() = %d, want %d", got, want)
			}
		})
	}
}

func TestMaterializedSourceOperationCombinations(t *testing.T) {
	leftSet := setFromRanges("left",
		Range{Lo: 1, Hi: 10},
		Range{Lo: 20, Hi: 25},
		Range{Lo: 40, Hi: 50},
	)
	rightSet := setFromRanges("right",
		Range{Lo: 5, Hi: 22},
		Range{Lo: 30, Hi: 35},
		Range{Lo: 45, Hi: 55},
	)
	leftFile, err := OpenFileSet(writeTempSet(t, leftSet))
	if err != nil {
		t.Fatalf("OpenFileSet(left): %v", err)
	}
	defer func() { _ = leftFile.Close() }()
	rightFile, err := OpenFileSet(writeTempSet(t, rightSet))
	if err != nil {
		t.Fatalf("OpenFileSet(right): %v", err)
	}
	defer func() { _ = rightFile.Close() }()

	leftGeneric := RangeSourceFromIter(leftSet.Iter(), leftSet.Len())
	rightGeneric := RangeSourceFromIter(rightSet.Iter(), rightSet.Len())

	cases := []struct {
		name  string
		left  RangeSource
		right RangeSource
	}{
		{"memory_memory", leftSet, rightSet},
		{"fileset_fileset", leftFile, rightFile},
		{"memory_fileset", leftSet, rightFile},
		{"fileset_memory", leftFile, rightSet},
		{"generic_generic", leftGeneric, rightGeneric},
	}

	wantUnion := []Range{{Lo: 1, Hi: 25}, {Lo: 30, Hi: 35}, {Lo: 40, Hi: 55}}
	wantIntersect := []Range{{Lo: 5, Hi: 10}, {Lo: 20, Hi: 22}, {Lo: 45, Hi: 50}}
	wantExclude := []Range{{Lo: 1, Hi: 4}, {Lo: 23, Hi: 25}, {Lo: 40, Hi: 44}}
	const wantExcludeCount uint64 = 12

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			union, err := UnionSourcesContext(t.Context(), "union", tc.left, tc.right)
			if err != nil {
				t.Fatalf("UnionSourcesContext() error = %v", err)
			}
			expectRanges(t, union, wantUnion)

			intersect, err := IntersectSourcesContext(t.Context(), "intersect", tc.left, tc.right)
			if err != nil {
				t.Fatalf("IntersectSourcesContext() error = %v", err)
			}
			expectRanges(t, intersect, wantIntersect)

			exclude, err := ExcludeSourcesContext(t.Context(), "exclude", tc.left, tc.right)
			if err != nil {
				t.Fatalf("ExcludeSourcesContext() error = %v", err)
			}
			expectRanges(t, exclude, wantExclude)

			excludeCount, err := ExcludeCountContext(t.Context(), tc.left, tc.right)
			if err != nil {
				t.Fatalf("ExcludeCountContext() error = %v", err)
			}
			if excludeCount != wantExcludeCount {
				t.Fatalf("ExcludeCountContext() = %d, want %d", excludeCount, wantExcludeCount)
			}

			var walked []Range
			if err := ExcludeRangesContext(t.Context(), tc.left, tc.right, func(r Range) bool {
				walked = append(walked, r)
				return true
			}); err != nil {
				t.Fatalf("ExcludeRangesContext() error = %v", err)
			}
			gotWalked := setFromRanges("walked", walked...)
			expectRanges(t, gotWalked, wantExclude)
		})
	}
}

func TestUnionSourcesContextKWay(t *testing.T) {
	a := setFromRanges("a", Range{Lo: 1, Hi: 10}, Range{Lo: 40, Hi: 50})
	b := setFromRanges("b", Range{Lo: 5, Hi: 22}, Range{Lo: 45, Hi: 55})
	c := setFromRanges("c", Range{Lo: 12, Hi: 18}, Range{Lo: 60, Hi: 70})

	got, err := UnionSourcesContext(t.Context(), "kway", a, b, c)
	if err != nil {
		t.Fatalf("UnionSourcesContext() error = %v", err)
	}
	expectRanges(t, got, []Range{{Lo: 1, Hi: 22}, {Lo: 40, Hi: 55}, {Lo: 60, Hi: 70}})
}

func TestMaterializedSourceOperationsHonorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	left := setFromRanges("left", Range{Lo: 1, Hi: 10})
	right := setFromRanges("right", Range{Lo: 5, Hi: 15})

	if _, err := UnionSourcesContext(ctx, "union", left, right); !errors.Is(err, context.Canceled) {
		t.Fatalf("UnionSourcesContext() error = %v, want context.Canceled", err)
	}
	if _, err := IntersectSourcesContext(ctx, "intersect", left, right); !errors.Is(err, context.Canceled) {
		t.Fatalf("IntersectSourcesContext() error = %v, want context.Canceled", err)
	}
	if _, err := ExcludeSourcesContext(ctx, "exclude", left, right); !errors.Is(err, context.Canceled) {
		t.Fatalf("ExcludeSourcesContext() error = %v, want context.Canceled", err)
	}
	if _, err := ExcludeCountContext(ctx, left, right); !errors.Is(err, context.Canceled) {
		t.Fatalf("ExcludeCountContext() error = %v, want context.Canceled", err)
	}
	if err := ExcludeRangesContext(ctx, left, right, func(Range) bool { return true }); !errors.Is(err, context.Canceled) {
		t.Fatalf("ExcludeRangesContext() error = %v, want context.Canceled", err)
	}
}
