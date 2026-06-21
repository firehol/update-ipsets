package iprange

import (
	"context"
	"errors"
	"testing"
)

func TestCompareSourcePairsMatchesSelectedCompareRows(t *testing.T) {
	a := newOptimizedSet("a", Range{Lo: 1, Hi: 10})
	b := newOptimizedSet("b", Range{Lo: 5, Hi: 15})
	c := newOptimizedSet("c", Range{Lo: 20, Hi: 30})
	d := newOptimizedSet("d", Range{Lo: 8, Hi: 8})

	got, err := CompareSourcePairs(t.Context(),
		[]CompareSource{
			{Name: "a", Source: a},
			{Name: "b", Source: b},
			{Name: "c", Source: c},
			{Name: "d", Source: d},
		},
		[]ComparePair{
			{Left: 1, Right: 3},
			{Left: 0, Right: 2},
			{Left: 0, Right: 1},
		},
	)
	if err != nil {
		t.Fatalf("CompareSourcePairs() error = %v", err)
	}
	want := []CompareRow{
		{Name1: "b", Name2: "d", Entries1: 1, Entries2: 1, Unique1: 11, Unique2: 1, CombinedIPs: 11, CommonIPs: 1},
		{Name1: "a", Name2: "c", Entries1: 1, Entries2: 1, Unique1: 10, Unique2: 11, CombinedIPs: 21, CommonIPs: 0},
		{Name1: "a", Name2: "b", Entries1: 1, Entries2: 1, Unique1: 10, Unique2: 11, CombinedIPs: 15, CommonIPs: 6},
	}
	expectCompareRows(t, got, want)
}

func TestCompareSourcePairsWithFileSetsAndRepeatedLeft(t *testing.T) {
	left := newOptimizedSet("left",
		Range{Lo: 1, Hi: 10},
		Range{Lo: 100, Hi: 110},
	)
	rightA := newOptimizedSet("right-a",
		Range{Lo: 5, Hi: 7},
		Range{Lo: 105, Hi: 120},
	)
	rightB := newOptimizedSet("right-b", Range{Lo: 200, Hi: 210})

	leftFS, err := OpenFileSet(writeTempSet(t, left))
	if err != nil {
		t.Fatalf("OpenFileSet(left) error = %v", err)
	}
	t.Cleanup(func() { _ = leftFS.Close() })
	rightAFS, err := OpenFileSet(writeTempSet(t, rightA))
	if err != nil {
		t.Fatalf("OpenFileSet(right-a) error = %v", err)
	}
	t.Cleanup(func() { _ = rightAFS.Close() })
	rightBFS, err := OpenFileSet(writeTempSet(t, rightB))
	if err != nil {
		t.Fatalf("OpenFileSet(right-b) error = %v", err)
	}
	t.Cleanup(func() { _ = rightBFS.Close() })

	got, err := CompareSourcePairs(t.Context(),
		[]CompareSource{
			{Name: "left", Source: leftFS},
			{Name: "right-a", Source: rightAFS},
			{Name: "right-b", Source: rightBFS},
		},
		[]ComparePair{
			{Left: 0, Right: 1},
			{Left: 0, Right: 2},
		},
	)
	if err != nil {
		t.Fatalf("CompareSourcePairs() error = %v", err)
	}
	want := []CompareRow{
		{Name1: "left", Name2: "right-a", Entries1: 2, Entries2: 2, Unique1: 21, Unique2: 19, CombinedIPs: 31, CommonIPs: 9},
		{Name1: "left", Name2: "right-b", Entries1: 2, Entries2: 1, Unique1: 21, Unique2: 11, CombinedIPs: 32, CommonIPs: 0},
	}
	expectCompareRows(t, got, want)
}

func TestCompareSourcePairsTargetRangeSpansMultipleLeftRanges(t *testing.T) {
	left := newOptimizedSet("left",
		Range{Lo: 10, Hi: 20},
		Range{Lo: 30, Hi: 40},
		Range{Lo: 50, Hi: 60},
	)
	spanning := newOptimizedSet("spanning", Range{Lo: 15, Hi: 55})
	fragmented := newOptimizedSet("fragmented",
		Range{Lo: 1, Hi: 5},
		Range{Lo: 37, Hi: 38},
		Range{Lo: 59, Hi: 70},
	)

	got, err := CompareSourcePairs(t.Context(),
		[]CompareSource{
			{Name: "left", Source: left},
			{Name: "spanning", Source: spanning},
			{Name: "fragmented", Source: fragmented},
		},
		[]ComparePair{
			{Left: 0, Right: 1},
			{Left: 0, Right: 2},
		},
	)
	if err != nil {
		t.Fatalf("CompareSourcePairs() error = %v", err)
	}
	want := []CompareRow{
		{Name1: "left", Name2: "spanning", Entries1: 3, Entries2: 1, Unique1: 33, Unique2: 41, CombinedIPs: 51, CommonIPs: 23},
		{Name1: "left", Name2: "fragmented", Entries1: 3, Entries2: 3, Unique1: 33, Unique2: 19, CombinedIPs: 48, CommonIPs: 4},
	}
	expectCompareRows(t, got, want)
}

func TestCompareSourcePairsMatchesPairwiseOverlapForArbitraryPairs(t *testing.T) {
	sets := []*IPSet{
		newOptimizedSet("a", Range{Lo: 1, Hi: 3}, Range{Lo: 10, Hi: 20}, Range{Lo: 40, Hi: 45}),
		newOptimizedSet("b", Range{Lo: 2, Hi: 12}, Range{Lo: 30, Hi: 35}),
		newOptimizedSet("c", Range{Lo: 18, Hi: 22}, Range{Lo: 44, Hi: 50}),
		newOptimizedSet("d", Range{Lo: 0, Hi: 60}),
		newOptimizedSet("e", Range{Lo: 70, Hi: 80}),
	}
	sources := make([]CompareSource, len(sets))
	for i, set := range sets {
		sources[i] = CompareSource{Name: set.Name, Source: set}
	}
	pairs := []ComparePair{
		{Left: 0, Right: 1},
		{Left: 0, Right: 2},
		{Left: 3, Right: 1},
		{Left: 4, Right: 0},
		{Left: 2, Right: 4},
		{Left: 1, Right: 3},
	}

	got, err := CompareSourcePairs(t.Context(), sources, pairs)
	if err != nil {
		t.Fatalf("CompareSourcePairs() error = %v", err)
	}
	want := make([]CompareRow, len(pairs))
	for i, pair := range pairs {
		left := sets[pair.Left]
		right := sets[pair.Right]
		common, err := OverlapCountIterContext(t.Context(), left, right)
		if err != nil {
			t.Fatalf("OverlapCountIterContext(%s,%s) error = %v", left.Name, right.Name, err)
		}
		want[i] = CompareRow{
			Name1:       left.Name,
			Name2:       right.Name,
			Entries1:    len(left.Ranges),
			Entries2:    len(right.Ranges),
			Unique1:     left.UniqueIPs,
			Unique2:     right.UniqueIPs,
			CombinedIPs: left.UniqueIPs + right.UniqueIPs - common,
			CommonIPs:   common,
		}
	}
	expectCompareRows(t, got, want)
}

func TestCompareSourcePairsHonorsContextCancellation(t *testing.T) {
	a := newOptimizedSet("a", Range{Lo: 1, Hi: 10})
	b := newOptimizedSet("b", Range{Lo: 5, Hi: 15})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := CompareSourcePairs(ctx,
		[]CompareSource{{Source: a}, {Source: b}},
		[]ComparePair{{Left: 0, Right: 1}},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CompareSourcePairs() error = %v, want context.Canceled", err)
	}
}

func TestCompareSourcePairsRejectsInvalidPairIndexes(t *testing.T) {
	a := newOptimizedSet("a", Range{Lo: 1, Hi: 10})
	b := newOptimizedSet("b", Range{Lo: 5, Hi: 15})

	_, err := CompareSourcePairs(t.Context(),
		[]CompareSource{{Source: a}, {Source: b}},
		[]ComparePair{{Left: 0, Right: 2}},
	)
	if err == nil {
		t.Fatal("CompareSourcePairs() error = nil, want invalid pair error")
	}
}
