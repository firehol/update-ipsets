package iprange

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

var rangeOverlapFilterSink RangeOverlapFilter

func TestCollectAndCountIterContext(t *testing.T) {
	src := setFromRanges("src",
		Range{Lo: 10, Hi: 12},
		Range{Lo: 20, Hi: 25},
	)

	got, err := CollectIterContext(t.Context(), "collected", src.Iter())
	if err != nil {
		t.Fatalf("CollectIterContext() error = %v", err)
	}
	expectRangeSlice(t, "CollectIterContext", got.Ranges, src.Ranges)

	count, err := CountIterContext(t.Context(), "counted", src.Iter())
	if err != nil {
		t.Fatalf("CountIterContext() error = %v", err)
	}
	if count != src.UniqueCount() {
		t.Fatalf("CountIterContext() = %d, want %d", count, src.UniqueCount())
	}
}

func TestCollectIterContextHonorsCancellation(t *testing.T) {
	src := setFromRanges("src")
	for i := uint32(0); i < 10_000; i++ {
		if err := src.AddRange(Range{Lo: i * 4, Hi: i*4 + 1}); err != nil {
			t.Fatalf("AddRange() error = %v", err)
		}
	}
	src.Optimize()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := CollectIterContext(ctx, "cancelled", src.Iter())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CollectIterContext() error = %v, want context.Canceled", err)
	}
}

func TestRangeSourcesEqualContextInMemoryAndFileSet(t *testing.T) {
	left := setFromRanges("left",
		Range{Lo: 1, Hi: 5},
		Range{Lo: 10, Hi: 20},
	)
	right := setFromRanges("right",
		Range{Lo: 1, Hi: 5},
		Range{Lo: 10, Hi: 20},
	)
	different := setFromRanges("different",
		Range{Lo: 1, Hi: 5},
		Range{Lo: 11, Hi: 20},
	)

	leftFile := writeRangeSourceTestFileSet(t, left)
	defer func() { _ = leftFile.Close() }()

	equal, err := RangeSourcesEqualContext(t.Context(), leftFile, right)
	if err != nil {
		t.Fatalf("RangeSourcesEqualContext(file, memory) error = %v", err)
	}
	if !equal {
		t.Fatal("RangeSourcesEqualContext(file, memory) = false, want true")
	}

	equal, err = RangeSourcesEqualContext(t.Context(), leftFile, different)
	if err != nil {
		t.Fatalf("RangeSourcesEqualContext(different) error = %v", err)
	}
	if equal {
		t.Fatal("RangeSourcesEqualContext(different) = true, want false")
	}

	rightFile := writeRangeSourceTestFileSet(t, right)
	defer func() { _ = rightFile.Close() }()
	differentFile := writeRangeSourceTestFileSet(t, different)
	defer func() { _ = differentFile.Close() }()

	equal, err = RangeSourcesEqualContext(t.Context(), leftFile, rightFile)
	if err != nil {
		t.Fatalf("RangeSourcesEqualContext(file, file) error = %v", err)
	}
	if !equal {
		t.Fatal("RangeSourcesEqualContext(file, file) = false, want true")
	}

	equal, err = RangeSourcesEqualContext(t.Context(), leftFile, differentFile)
	if err != nil {
		t.Fatalf("RangeSourcesEqualContext(file, different file) error = %v", err)
	}
	if equal {
		t.Fatal("RangeSourcesEqualContext(file, different file) = true, want false")
	}
}

func TestRangeSourcesEqualContextHonorsCancellation(t *testing.T) {
	left := setFromRanges("left", Range{Lo: 1, Hi: 5})
	right := setFromRanges("right", Range{Lo: 1, Hi: 5})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	equal, err := RangeSourcesEqualContext(ctx, left, right)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RangeSourcesEqualContext() error = %v, want context.Canceled", err)
	}
	if equal {
		t.Fatal("RangeSourcesEqualContext() equal = true after cancellation")
	}
}

func TestRangeSourceBoundsContext(t *testing.T) {
	src := setFromRanges("bounds",
		Range{Lo: 100, Hi: 120},
		Range{Lo: 200, Hi: 230},
	)
	fs := writeRangeSourceTestFileSet(t, src)
	defer func() { _ = fs.Close() }()

	for _, source := range []RangeSource{src, fs} {
		bounds, ok, err := RangeSourceBoundsContext(t.Context(), source)
		if err != nil {
			t.Fatalf("RangeSourceBoundsContext(%T) error = %v", source, err)
		}
		if !ok {
			t.Fatalf("RangeSourceBoundsContext(%T) ok = false, want true", source)
		}
		if bounds != (Range{Lo: 100, Hi: 230}) {
			t.Fatalf("RangeSourceBoundsContext(%T) = %+v, want 100..230", source, bounds)
		}
	}
}

func TestRangeSourceSummaryAndOverlapFilter(t *testing.T) {
	left := setFromRanges("left", Range{Lo: 0x0A000001, Hi: 0x0A000001})
	sameCoarseDifferentSparse := setFromRanges("same-coarse", Range{Lo: 0x0A000201, Hi: 0x0A000201})
	differentBounds := setFromRanges("different-bounds", Range{Lo: 0xC0000201, Hi: 0xC0000201})
	overlapping := setFromRanges("overlap", Range{Lo: 0x0A000001, Hi: 0x0A000002})

	leftSummary, err := BuildRangeSourceSummaryContext(t.Context(), left)
	if err != nil {
		t.Fatalf("BuildRangeSourceSummaryContext(left) error = %v", err)
	}
	leftFile := writeRangeSourceTestFileSet(t, left)
	defer func() { _ = leftFile.Close() }()
	fileSummary, err := BuildRangeSourceSummaryContext(t.Context(), leftFile)
	if err != nil {
		t.Fatalf("BuildRangeSourceSummaryContext(file) error = %v", err)
	}
	if !leftSummary.ContentHash.Equal(fileSummary.ContentHash) {
		t.Fatal("in-memory and file-backed summaries should have the same content hash")
	}

	sameCoarseFilter := mustRangeSourceTestFilter(t, sameCoarseDifferentSparse)
	if leftSummary.OverlapFilter().PrefixesDisjoint(sameCoarseFilter) {
		t.Fatal("same coarse prefix should not be a coarse-prefix disjoint proof")
	}
	if !leftSummary.OverlapFilter().SparsePrefixesDisjoint(sameCoarseFilter) {
		t.Fatal("different sparse prefixes should prove zero overlap")
	}

	if !leftSummary.OverlapFilter().BoundsDisjoint(mustRangeSourceTestFilter(t, differentBounds)) {
		t.Fatal("non-overlapping bounds should prove zero overlap")
	}
	if leftSummary.OverlapFilter().Disjoint(mustRangeSourceTestFilter(t, overlapping)) {
		t.Fatal("overlapping ranges must not be reported disjoint")
	}
	if (RangeOverlapFilter{}).Disjoint(leftSummary.OverlapFilter()) {
		t.Fatal("unknown filters must fall through to exact overlap")
	}

	filterOnly, err := BuildRangeOverlapFilterContext(t.Context(), leftFile)
	if err != nil {
		t.Fatalf("BuildRangeOverlapFilterContext(file) error = %v", err)
	}
	if filterOnly.Disjoint(leftSummary.OverlapFilter()) {
		t.Fatal("filter-only and summary-derived filters for the same source must not be disjoint")
	}
	if !filterOnly.BoundsDisjoint(mustRangeSourceTestFilter(t, differentBounds)) {
		t.Fatal("filter-only bounds should prove zero overlap")
	}
}

func TestRangeOverlapFilterBroadSourceAllocationShape(t *testing.T) {
	broad := New("broad")
	for i := range rangeSummarySparsePrefixLimit + 512 {
		prefix := uint32(i) << rangeSummarySparsePrefixShift
		if err := broad.AddRange(Range{Lo: prefix, Hi: prefix}); err != nil {
			t.Fatalf("AddRange() error = %v", err)
		}
	}
	broad.Optimize()

	filter, err := BuildRangeOverlapFilterContext(context.Background(), broad)
	if err != nil {
		t.Fatalf("BuildRangeOverlapFilterContext() error = %v", err)
	}
	if !filter.Valid() || !filter.HasRange() {
		t.Fatalf("BuildRangeOverlapFilterContext() = %+v, want valid non-empty filter", filter)
	}
	if filter.SparsePrefixesDisjoint(mustRangeSourceTestFilter(t, setFromRanges("narrow", Range{Lo: 1, Hi: 1}))) {
		t.Fatal("overflowed sparse prefixes must stay conservative")
	}

	allocs := testing.AllocsPerRun(20, func() {
		got, err := BuildRangeOverlapFilterContext(context.Background(), broad)
		if err != nil {
			panic(err)
		}
		rangeOverlapFilterSink = got
	})
	if allocs > 6 {
		t.Fatalf("BuildRangeOverlapFilterContext() broad-source allocations = %.0f, want <= 6", allocs)
	}
}

func TestWalkRangeOverlapsContext(t *testing.T) {
	src := setFromRanges("src",
		Range{Lo: 10, Hi: 20},
		Range{Lo: 30, Hi: 40},
	)
	targets := RangeList{
		{Lo: 5, Hi: 12},
		{Lo: 18, Hi: 32},
		{Lo: 50, Hi: 60},
	}

	var got []RangeOverlap
	if err := WalkRangeOverlapsContext(t.Context(), src, targets, func(overlap RangeOverlap) bool {
		got = append(got, overlap)
		return true
	}); err != nil {
		t.Fatalf("WalkRangeOverlapsContext() error = %v", err)
	}

	want := []RangeOverlap{
		{Left: Range{Lo: 10, Hi: 20}, Right: Range{Lo: 5, Hi: 12}, RightIndex: 0, Overlap: Range{Lo: 10, Hi: 12}},
		{Left: Range{Lo: 10, Hi: 20}, Right: Range{Lo: 18, Hi: 32}, RightIndex: 1, Overlap: Range{Lo: 18, Hi: 20}},
		{Left: Range{Lo: 30, Hi: 40}, Right: Range{Lo: 18, Hi: 32}, RightIndex: 1, Overlap: Range{Lo: 30, Hi: 32}},
	}
	if len(got) != len(want) {
		t.Fatalf("WalkRangeOverlapsContext() yielded %d overlaps, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("WalkRangeOverlapsContext()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestWalkRangeOverlapsContextHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := WalkRangeOverlapsContext(ctx,
		setFromRanges("src", Range{Lo: 1, Hi: 10}),
		RangeList{{Lo: 1, Hi: 10}},
		func(RangeOverlap) bool { return true },
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WalkRangeOverlapsContext() error = %v, want context.Canceled", err)
	}
}

func TestRangeSourceContentHashContext(t *testing.T) {
	src := setFromRanges("hash",
		Range{Lo: 10, Hi: 12},
		Range{Lo: 20, Hi: 25},
	)
	fs := writeRangeSourceTestFileSet(t, src)
	defer func() { _ = fs.Close() }()

	summary, err := BuildRangeSourceSummaryContext(t.Context(), src)
	if err != nil {
		t.Fatalf("BuildRangeSourceSummaryContext() error = %v", err)
	}
	hash, err := RangeSourceContentHashContext(t.Context(), fs)
	if err != nil {
		t.Fatalf("RangeSourceContentHashContext() error = %v", err)
	}
	if !hash.Equal(summary.ContentHash) {
		t.Fatalf("RangeSourceContentHashContext() = %s, want summary hash %s", hash.Hex(), summary.ContentHash.Hex())
	}
	if hash.Hex() == "" {
		t.Fatal("RangeSourceContentHashContext() returned an empty hex hash")
	}
}

func TestRangeSourceContentHashContextHonorsCancellation(t *testing.T) {
	src := setFromRanges("src", Range{Lo: 1, Hi: 10})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	hash, err := RangeSourceContentHashContext(ctx, src)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RangeSourceContentHashContext() error = %v, want context.Canceled", err)
	}
	if hash.Valid {
		t.Fatal("cancelled content hash unexpectedly valid")
	}
}

func TestRangeSourceSummaryHonorsCancellation(t *testing.T) {
	src := setFromRanges("src", Range{Lo: 1, Hi: 10})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	summary, err := BuildRangeSourceSummaryContext(ctx, src)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("BuildRangeSourceSummaryContext() error = %v, want context.Canceled", err)
	}
	if summary.ContentHash.Valid {
		t.Fatal("cancelled summary unexpectedly has a content hash")
	}
}

func mustRangeSourceTestFilter(t *testing.T, src RangeSource) RangeOverlapFilter {
	t.Helper()
	filter, err := BuildRangeOverlapFilterContext(t.Context(), src)
	if err != nil {
		t.Fatalf("BuildRangeOverlapFilterContext() error = %v", err)
	}
	return filter
}

func writeRangeSourceTestFileSet(t *testing.T, set *IPSet) FileSet {
	t.Helper()
	path := filepath.Join(t.TempDir(), set.Name+".set")
	writeSet(t, path, set)
	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatalf("OpenFileSet() error = %v", err)
	}
	return fs
}
