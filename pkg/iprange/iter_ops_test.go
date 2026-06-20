package iprange

import (
	"context"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

// collectIter drains a push iterator into a slice.
func collectIter(it func(yield func(Range) bool)) []Range {
	var out []Range
	for r := range it {
		out = append(out, r)
	}
	return out
}

// collectIter6 drains an IPv6 push iterator into a slice.
func collectIter6(it func(yield func(Range6) bool)) []Range6 {
	var out []Range6
	for r := range it {
		out = append(out, r)
	}
	return out
}

// setFromRanges is a convenience to build an optimized IPSet from ranges.
func setFromRanges(name string, ranges ...Range) *IPSet {
	return newOptimizedSet(name, ranges...)
}

// --- RangeSource conformance -------------------------------------------------

func TestIPSetSatisfiesRangeSource(t *testing.T) {
	var _ RangeSource = setFromRanges("test", Range{Lo: 1, Hi: 2})
}

func TestFileSetSatisfiesRangeSource(t *testing.T) {
	set := setFromRanges("test", Range{Lo: 1, Hi: 2})
	path := writeTempSet(t, set)
	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()
	var _ RangeSource = fs
}

// --- CountUniqueIter ---------------------------------------------------------

func TestCountUniqueIter(t *testing.T) {
	cases := []struct {
		name   string
		ranges []Range
		want   uint64
	}{
		{"empty", nil, 0},
		{"single_ip", []Range{{Lo: 5, Hi: 5}}, 1},
		{"single_range", []Range{{Lo: 10, Hi: 20}}, 11},
		{"multiple", []Range{{Lo: 1, Hi: 3}, {Lo: 10, Hi: 12}}, 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := setFromRanges(tc.name, tc.ranges...)
			got := CountUniqueIter(s)
			if got != tc.want {
				t.Fatalf("CountUniqueIter: got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCountUniqueIterUsesKnownCount(t *testing.T) {
	src := panicIterCountSource{count: 42}
	if got := CountUniqueIter(src); got != 42 {
		t.Fatalf("CountUniqueIter() = %d, want known count 42", got)
	}
}

func TestOverlapCountIter6InMemoryAndFileSet(t *testing.T) {
	left := makeTestSet6(
		Range6{Lo: u128FromUint64(10), Hi: u128FromUint64(20)},
		Range6{Lo: u128FromUint64(40), Hi: u128FromUint64(50)},
	)
	right := makeTestSet6(
		Range6{Lo: u128FromUint64(15), Hi: u128FromUint64(25)},
		Range6{Lo: u128FromUint64(45), Hi: u128FromUint64(45)},
	)
	leftFile, err := OpenFileSet6(writeTestBinary6File(t, left))
	if err != nil {
		t.Fatalf("OpenFileSet6(left): %v", err)
	}
	defer func() { _ = leftFile.Close() }()
	rightFile, err := OpenFileSet6(writeTestBinary6File(t, right))
	if err != nil {
		t.Fatalf("OpenFileSet6(right): %v", err)
	}
	defer func() { _ = rightFile.Close() }()

	want := u128FromUint64(7)
	for _, tc := range []struct {
		name  string
		left  RangeSource6
		right RangeSource6
	}{
		{name: "memory_memory", left: left, right: right},
		{name: "fileset_fileset", left: leftFile, right: rightFile},
		{name: "memory_fileset", left: left, right: rightFile},
		{name: "fileset_memory", left: leftFile, right: right},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := OverlapCountIter6(tc.left, tc.right); !got.Equals(want) {
				t.Fatalf("OverlapCountIter6() = %s, want %s", got.String(), want.String())
			}
		})
	}
}

func TestIPv6IteratorsInMemory(t *testing.T) {
	leftRanges := []Range6{
		{Lo: u128FromUint64(1), Hi: u128FromUint64(5)},
		{Lo: u128FromUint64(10), Hi: u128FromUint64(15)},
		{Lo: u128FromUint64(20), Hi: u128FromUint64(25)},
	}
	left := makeTestSet6(
		leftRanges...,
	)
	right := makeTestSet6(
		Range6{Lo: u128FromUint64(3), Hi: u128FromUint64(12)},
		Range6{Lo: u128FromUint64(22), Hi: u128FromUint64(30)},
	)

	expectRange6Slice(t, "IntersectIter6", collectIter6(IntersectIter6(left, right)), []Range6{
		{Lo: u128FromUint64(3), Hi: u128FromUint64(5)},
		{Lo: u128FromUint64(10), Hi: u128FromUint64(12)},
		{Lo: u128FromUint64(22), Hi: u128FromUint64(25)},
	})
	expectRange6Slice(t, "ExcludeIter6", collectIter6(ExcludeIter6(left, right)), []Range6{
		{Lo: u128FromUint64(1), Hi: u128FromUint64(2)},
		{Lo: u128FromUint64(13), Hi: u128FromUint64(15)},
		{Lo: u128FromUint64(20), Hi: u128FromUint64(21)},
	})
	expectRange6Slice(t, "UnionIter6", collectIter6(UnionIter6(left, right)), []Range6{
		{Lo: u128FromUint64(1), Hi: u128FromUint64(15)},
		{Lo: u128FromUint64(20), Hi: u128FromUint64(30)},
	})
	expectRange6Slice(t, "source ranges after iteration", left.Ranges, leftRanges)
}

func TestIPv4IteratorsInMemoryDoNotMutateInputs(t *testing.T) {
	leftRanges := []Range{
		{Lo: 1, Hi: 5},
		{Lo: 10, Hi: 15},
		{Lo: 20, Hi: 25},
	}
	left := setFromRanges("left", leftRanges...)
	right := setFromRanges("right",
		Range{Lo: 3, Hi: 12},
		Range{Lo: 22, Hi: 30},
	)

	_ = collectIter(IntersectIter(left, right))
	_ = collectIter(ExcludeIter(left, right))
	_ = collectIter(DiffIter(left, right))
	_ = collectIter(UnionIter(left, right))

	expectRangeSlice(t, "source ranges after iteration", left.Ranges, leftRanges)
}

// --- IntersectIter -----------------------------------------------------------

func TestIntersectIter(t *testing.T) {
	cases := []struct {
		name string
		a, b []Range
		want []Range
	}{
		{
			"disjoint",
			[]Range{{Lo: 1, Hi: 5}},
			[]Range{{Lo: 10, Hi: 15}},
			nil,
		},
		{
			"identical",
			[]Range{{Lo: 1, Hi: 10}},
			[]Range{{Lo: 1, Hi: 10}},
			[]Range{{Lo: 1, Hi: 10}},
		},
		{
			"a_subset_of_b",
			[]Range{{Lo: 3, Hi: 7}},
			[]Range{{Lo: 1, Hi: 10}},
			[]Range{{Lo: 3, Hi: 7}},
		},
		{
			"b_subset_of_a",
			[]Range{{Lo: 1, Hi: 10}},
			[]Range{{Lo: 3, Hi: 7}},
			[]Range{{Lo: 3, Hi: 7}},
		},
		{
			"partial_overlap",
			[]Range{{Lo: 1, Hi: 10}},
			[]Range{{Lo: 5, Hi: 15}},
			[]Range{{Lo: 5, Hi: 10}},
		},
		{
			"empty_a",
			nil,
			[]Range{{Lo: 1, Hi: 10}},
			nil,
		},
		{
			"empty_b",
			[]Range{{Lo: 1, Hi: 10}},
			nil,
			nil,
		},
		{
			"both_empty",
			nil,
			nil,
			nil,
		},
		{
			"multiple_ranges_overlap",
			[]Range{{Lo: 1, Hi: 5}, {Lo: 10, Hi: 15}, {Lo: 20, Hi: 25}},
			[]Range{{Lo: 3, Hi: 12}, {Lo: 22, Hi: 30}},
			[]Range{{Lo: 3, Hi: 5}, {Lo: 10, Hi: 12}, {Lo: 22, Hi: 25}},
		},
		{
			"single_ip",
			[]Range{{Lo: 5, Hi: 5}},
			[]Range{{Lo: 5, Hi: 5}},
			[]Range{{Lo: 5, Hi: 5}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := setFromRanges("a", tc.a...)
			b := setFromRanges("b", tc.b...)
			got := collectIter(IntersectIter(a, b))
			expectRangeSlice(t, "IntersectIter", got, tc.want)
		})
	}
}

// --- ExcludeIter -------------------------------------------------------------

func TestExcludeIter(t *testing.T) {
	cases := []struct {
		name string
		a, b []Range
		want []Range
	}{
		{
			"disjoint",
			[]Range{{Lo: 1, Hi: 5}},
			[]Range{{Lo: 10, Hi: 15}},
			[]Range{{Lo: 1, Hi: 5}},
		},
		{
			"identical",
			[]Range{{Lo: 1, Hi: 10}},
			[]Range{{Lo: 1, Hi: 10}},
			nil,
		},
		{
			"a_subset_of_b",
			[]Range{{Lo: 3, Hi: 7}},
			[]Range{{Lo: 1, Hi: 10}},
			nil,
		},
		{
			"b_subset_of_a",
			[]Range{{Lo: 1, Hi: 10}},
			[]Range{{Lo: 3, Hi: 7}},
			[]Range{{Lo: 1, Hi: 2}, {Lo: 8, Hi: 10}},
		},
		{
			"partial_overlap",
			[]Range{{Lo: 1, Hi: 10}},
			[]Range{{Lo: 5, Hi: 15}},
			[]Range{{Lo: 1, Hi: 4}},
		},
		{
			"empty_a",
			nil,
			[]Range{{Lo: 1, Hi: 10}},
			nil,
		},
		{
			"empty_b",
			[]Range{{Lo: 1, Hi: 10}},
			nil,
			[]Range{{Lo: 1, Hi: 10}},
		},
		{
			"both_empty",
			nil,
			nil,
			nil,
		},
		{
			"multiple_ranges",
			[]Range{{Lo: 1, Hi: 5}, {Lo: 10, Hi: 15}, {Lo: 20, Hi: 25}},
			[]Range{{Lo: 3, Hi: 12}},
			[]Range{{Lo: 1, Hi: 2}, {Lo: 13, Hi: 15}, {Lo: 20, Hi: 25}},
		},
		{
			"single_ip_excluded",
			[]Range{{Lo: 5, Hi: 5}},
			[]Range{{Lo: 5, Hi: 5}},
			nil,
		},
		{
			"b_covers_middle_of_a",
			[]Range{{Lo: 1, Hi: 100}},
			[]Range{{Lo: 20, Hi: 30}, {Lo: 50, Hi: 60}},
			[]Range{{Lo: 1, Hi: 19}, {Lo: 31, Hi: 49}, {Lo: 61, Hi: 100}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := setFromRanges("a", tc.a...)
			b := setFromRanges("b", tc.b...)
			got := collectIter(ExcludeIter(a, b))
			expectRangeSlice(t, "ExcludeIter", got, tc.want)
		})
	}
}

// --- DiffIter ----------------------------------------------------------------

func TestDiffIter(t *testing.T) {
	cases := []struct {
		name string
		a, b []Range
		want []Range
	}{
		{
			"disjoint",
			[]Range{{Lo: 1, Hi: 5}},
			[]Range{{Lo: 10, Hi: 15}},
			[]Range{{Lo: 1, Hi: 5}, {Lo: 10, Hi: 15}},
		},
		{
			"identical",
			[]Range{{Lo: 1, Hi: 10}},
			[]Range{{Lo: 1, Hi: 10}},
			nil,
		},
		{
			"partial_overlap",
			[]Range{{Lo: 1, Hi: 10}},
			[]Range{{Lo: 5, Hi: 15}},
			[]Range{{Lo: 1, Hi: 4}, {Lo: 11, Hi: 15}},
		},
		{
			"empty_a",
			nil,
			[]Range{{Lo: 1, Hi: 10}},
			[]Range{{Lo: 1, Hi: 10}},
		},
		{
			"empty_b",
			[]Range{{Lo: 1, Hi: 10}},
			nil,
			[]Range{{Lo: 1, Hi: 10}},
		},
		{
			"both_empty",
			nil,
			nil,
			nil,
		},
		{
			"b_subset_of_a",
			[]Range{{Lo: 1, Hi: 10}},
			[]Range{{Lo: 3, Hi: 7}},
			[]Range{{Lo: 1, Hi: 2}, {Lo: 8, Hi: 10}},
		},
		{
			"a_subset_of_b",
			[]Range{{Lo: 3, Hi: 7}},
			[]Range{{Lo: 1, Hi: 10}},
			[]Range{{Lo: 1, Hi: 2}, {Lo: 8, Hi: 10}},
		},
		{
			"multiple_ranges",
			[]Range{{Lo: 1, Hi: 5}, {Lo: 20, Hi: 25}},
			[]Range{{Lo: 3, Hi: 22}},
			[]Range{{Lo: 1, Hi: 2}, {Lo: 6, Hi: 19}, {Lo: 23, Hi: 25}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := setFromRanges("a", tc.a...)
			b := setFromRanges("b", tc.b...)
			got := collectIter(DiffIter(a, b))
			expectRangeSlice(t, "DiffIter", got, tc.want)
		})
	}
}

// --- UnionIter ---------------------------------------------------------------

func TestUnionIter(t *testing.T) {
	cases := []struct {
		name    string
		sources [][]Range
		want    []Range
	}{
		{
			"zero_sources",
			nil,
			nil,
		},
		{
			"single_source",
			[][]Range{{{Lo: 1, Hi: 5}, {Lo: 10, Hi: 15}}},
			[]Range{{Lo: 1, Hi: 5}, {Lo: 10, Hi: 15}},
		},
		{
			"two_disjoint",
			[][]Range{
				{{Lo: 1, Hi: 5}},
				{{Lo: 10, Hi: 15}},
			},
			[]Range{{Lo: 1, Hi: 5}, {Lo: 10, Hi: 15}},
		},
		{
			"two_overlapping",
			[][]Range{
				{{Lo: 1, Hi: 10}},
				{{Lo: 5, Hi: 15}},
			},
			[]Range{{Lo: 1, Hi: 15}},
		},
		{
			"two_identical",
			[][]Range{
				{{Lo: 1, Hi: 10}},
				{{Lo: 1, Hi: 10}},
			},
			[]Range{{Lo: 1, Hi: 10}},
		},
		{
			"two_adjacent",
			[][]Range{
				{{Lo: 1, Hi: 5}},
				{{Lo: 6, Hi: 10}},
			},
			[]Range{{Lo: 1, Hi: 10}},
		},
		{
			"three_sources",
			[][]Range{
				{{Lo: 1, Hi: 3}},
				{{Lo: 5, Hi: 7}},
				{{Lo: 2, Hi: 6}},
			},
			[]Range{{Lo: 1, Hi: 7}},
		},
		{
			"four_sources_disjoint",
			[][]Range{
				{{Lo: 1, Hi: 2}},
				{{Lo: 10, Hi: 11}},
				{{Lo: 20, Hi: 21}},
				{{Lo: 30, Hi: 31}},
			},
			[]Range{{Lo: 1, Hi: 2}, {Lo: 10, Hi: 11}, {Lo: 20, Hi: 21}, {Lo: 30, Hi: 31}},
		},
		{
			"all_empty",
			[][]Range{{}, {}, {}},
			nil,
		},
		{
			"one_empty_one_not",
			[][]Range{
				{},
				{{Lo: 1, Hi: 5}},
			},
			[]Range{{Lo: 1, Hi: 5}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sources := make([]RangeSource, len(tc.sources))
			for i, rs := range tc.sources {
				sources[i] = setFromRanges("s", rs...)
			}
			got := collectIter(UnionIter(sources...))
			expectRangeSlice(t, "UnionIter", got, tc.want)
		})
	}
}

// --- OverlapCountIter --------------------------------------------------------

func TestOverlapCountIter(t *testing.T) {
	cases := []struct {
		name string
		a, b []Range
		want uint64
	}{
		{"disjoint", []Range{{Lo: 1, Hi: 5}}, []Range{{Lo: 10, Hi: 15}}, 0},
		{"identical", []Range{{Lo: 1, Hi: 10}}, []Range{{Lo: 1, Hi: 10}}, 10},
		{"partial", []Range{{Lo: 1, Hi: 10}}, []Range{{Lo: 5, Hi: 15}}, 6},
		{"subset", []Range{{Lo: 3, Hi: 7}}, []Range{{Lo: 1, Hi: 10}}, 5},
		{"empty_a", nil, []Range{{Lo: 1, Hi: 10}}, 0},
		{"empty_b", []Range{{Lo: 1, Hi: 10}}, nil, 0},
		{"both_empty", nil, nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := setFromRanges("a", tc.a...)
			b := setFromRanges("b", tc.b...)
			got := OverlapCountIter(a, b)
			if got != tc.want {
				t.Fatalf("OverlapCountIter: got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestOverlapCountIterContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	a := setFromRanges("a", Range{Lo: 1, Hi: 10})
	b := setFromRanges("b", Range{Lo: 1, Hi: 10})
	got, err := OverlapCountIterContext(ctx, a, b)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("OverlapCountIterContext() error = %v, want context.Canceled", err)
	}
	if got != 0 {
		t.Fatalf("OverlapCountIterContext() = %d, want 0 after cancellation", got)
	}
}

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

// --- Early termination tests -------------------------------------------------

func TestIterEarlyBreak(t *testing.T) {
	a := setFromRanges("a",
		Range{Lo: 1, Hi: 5},
		Range{Lo: 10, Hi: 15},
		Range{Lo: 20, Hi: 25},
		Range{Lo: 30, Hi: 35},
	)
	b := setFromRanges("b",
		Range{Lo: 3, Hi: 22},
	)

	ops := []struct {
		name string
		iter func(yield func(Range) bool)
	}{
		{"IntersectIter", IntersectIter(a, b)},
		{"ExcludeIter", ExcludeIter(a, b)},
		{"DiffIter", DiffIter(a, b)},
		{"UnionIter", UnionIter(a, b)},
	}
	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			count := 0
			for range op.iter {
				count++
				if count >= 1 {
					break
				}
			}
			if count != 1 {
				t.Fatalf("expected exactly 1 iteration, got %d", count)
			}
		})
	}
}

// --- Helpers -----------------------------------------------------------------

func expectRangeSlice(t *testing.T, label string, got, want []Range) {
	t.Helper()
	if !rangeSlicesEqual(got, want) {
		t.Fatalf("%s mismatch:\n  got  = %v\n  want = %v", label, got, want)
	}
}

func expectRange6Slice(t *testing.T, label string, got, want []Range6) {
	t.Helper()
	if !range6SlicesEqual(got, want) {
		t.Fatalf("%s mismatch:\n  got  = %v\n  want = %v", label, got, want)
	}
}

func rangeSlicesEqual(a, b []Range) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func range6SlicesEqual(a, b []Range6) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Lo.Equals(b[i].Lo) || !a[i].Hi.Equals(b[i].Hi) {
			return false
		}
	}
	return true
}

func countIterIPs(it func(yield func(Range) bool)) uint64 {
	var total uint64
	for r := range it {
		total += r.Size()
	}
	return total
}

type panicIterCountSource struct {
	count uint64
}

func (s panicIterCountSource) Len() int {
	return 1
}

func (s panicIterCountSource) UniqueIPs() uint64 {
	return s.count
}

func (s panicIterCountSource) Iter() func(func(Range) bool) {
	return func(func(Range) bool) {
		panic("CountUniqueIter should use the known count")
	}
}

func randomSet(rng *rand.Rand, name string, numRanges int) *IPSet {
	set := New(name)
	for range numRanges {
		lo := rng.Uint32N(0xFFFFF000)
		span := rng.Uint32N(256)
		if err := set.Add(lo, lo+span); err != nil {
			panic(err)
		}
	}
	set.Optimize()
	return set
}

// sortedPairs creates two ranges from four uint8 values, ensuring valid ranges.
func sortedPairs(a1, a2, b1, b2 uint8) []Range {
	lo1, hi1 := ordered(uint32(a1), uint32(a2))
	lo2, hi2 := ordered(uint32(b1), uint32(b2))
	s := New("tmp")
	_ = s.Add(lo1, hi1)
	_ = s.Add(lo2, hi2)
	s.Optimize()
	return s.Ranges
}

// setFromIter builds an optimized IPSet from an iterator.
func setFromIter(name string, it func(yield func(Range) bool)) *IPSet {
	s := New(name)
	for r := range it {
		s.Ranges = append(s.Ranges, r)
		s.Lines++
	}
	s.Optimize()
	return s
}

// writeTempSetBench writes an IPSet to a temp .set file for benchmarks.
func writeTempSetBench(b *testing.B, set *IPSet) string {
	b.Helper()
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.set")
	f, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	if err := WriteBinary(f, set); err != nil {
		_ = f.Close()
		b.Fatal(err)
	}
	if err := f.Close(); err != nil {
		b.Fatal(err)
	}
	return path
}
