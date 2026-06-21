package iprange

import (
	"context"
	"errors"
	"testing"
)

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
