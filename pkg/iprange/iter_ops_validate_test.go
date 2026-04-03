package iprange

import (
	"math/rand/v2"
	"runtime"
	"testing"
	"testing/quick"
)

// --- Cross-validation against in-memory ops ----------------------------------

func TestIterMatchesInMemoryIntersect(t *testing.T) {
	fn := func(a1, a2, b1, b2, c1, c2, d1, d2 uint8) bool {
		aRanges := sortedPairs(a1, a2, c1, c2)
		bRanges := sortedPairs(b1, b2, d1, d2)
		a := setFromRanges("a", aRanges...)
		b := setFromRanges("b", bRanges...)

		memResult := Intersect(a, b)
		memResult.Optimize()

		iterRanges := collectIter(IntersectIter(a, b))
		return rangeSlicesEqual(memResult.Ranges, iterRanges)
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 2048}); err != nil {
		t.Fatal(err)
	}
}

func TestIterMatchesInMemoryExclude(t *testing.T) {
	fn := func(a1, a2, b1, b2, c1, c2, d1, d2 uint8) bool {
		aRanges := sortedPairs(a1, a2, c1, c2)
		bRanges := sortedPairs(b1, b2, d1, d2)
		a := setFromRanges("a", aRanges...)
		b := setFromRanges("b", bRanges...)

		memResult := Exclude(a, b)
		memResult.Optimize()

		iterRanges := collectIter(ExcludeIter(a, b))
		return rangeSlicesEqual(memResult.Ranges, iterRanges)
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 2048}); err != nil {
		t.Fatal(err)
	}
}

func TestIterMatchesInMemoryDiff(t *testing.T) {
	fn := func(a1, a2, b1, b2, c1, c2, d1, d2 uint8) bool {
		aRanges := sortedPairs(a1, a2, c1, c2)
		bRanges := sortedPairs(b1, b2, d1, d2)
		a := setFromRanges("a", aRanges...)
		b := setFromRanges("b", bRanges...)

		memResult := Diff(a, b)
		memResult.Optimize()

		iterRanges := collectIter(DiffIter(a, b))
		return rangeSlicesEqual(memResult.Ranges, iterRanges)
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 2048}); err != nil {
		t.Fatal(err)
	}
}

func TestIterMatchesInMemoryOverlapCount(t *testing.T) {
	fn := func(a1, a2, b1, b2, c1, c2, d1, d2 uint8) bool {
		aRanges := sortedPairs(a1, a2, c1, c2)
		bRanges := sortedPairs(b1, b2, d1, d2)
		a := setFromRanges("a", aRanges...)
		b := setFromRanges("b", bRanges...)

		memIntersect := Intersect(a, b)
		memIntersect.Optimize()

		iterCount := OverlapCountIter(a, b)
		return memIntersect.UniqueIPs == iterCount
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 2048}); err != nil {
		t.Fatal(err)
	}
}

// --- Large random set tests --------------------------------------------------

func TestIterOpsLargeRandom(t *testing.T) {
	const numRanges = 10_000
	rng := rand.New(rand.NewPCG(12345, 0))

	a := randomSet(rng, "a", numRanges)
	b := randomSet(rng, "b", numRanges)

	// Intersect.
	memIntersect := Intersect(a, b)
	memIntersect.Optimize()
	iterOverlap := OverlapCountIter(a, b)
	if memIntersect.UniqueIPs != iterOverlap {
		t.Fatalf("large intersect mismatch: mem=%d iter=%d", memIntersect.UniqueIPs, iterOverlap)
	}

	// Exclude.
	memExclude := Exclude(a, b)
	memExclude.Optimize()
	iterExcludeCount := countIterIPs(ExcludeIter(a, b))
	if memExclude.UniqueIPs != iterExcludeCount {
		t.Fatalf("large exclude mismatch: mem=%d iter=%d", memExclude.UniqueIPs, iterExcludeCount)
	}

	// Diff.
	memDiff := Diff(a, b)
	memDiff.Optimize()
	iterDiffCount := countIterIPs(DiffIter(a, b))
	if memDiff.UniqueIPs != iterDiffCount {
		t.Fatalf("large diff mismatch: mem=%d iter=%d", memDiff.UniqueIPs, iterDiffCount)
	}

	// Union.
	memUnion := Combine(a, b)
	memUnion.Optimize()
	iterUnionCount := countIterIPs(UnionIter(a, b))
	if memUnion.UniqueIPs != iterUnionCount {
		t.Fatalf("large union mismatch: mem=%d iter=%d", memUnion.UniqueIPs, iterUnionCount)
	}
}

// --- Identity properties -----------------------------------------------------

func TestIterIdentityExcludePlusIntersect(t *testing.T) {
	// For any a,b: |Exclude(a,b)| + |Intersect(a,b)| == |a|
	fn := func(a1, a2, b1, b2, c1, c2, d1, d2 uint8) bool {
		aRanges := sortedPairs(a1, a2, c1, c2)
		bRanges := sortedPairs(b1, b2, d1, d2)
		a := setFromRanges("a", aRanges...)
		b := setFromRanges("b", bRanges...)

		excludeCount := countIterIPs(ExcludeIter(a, b))
		intersectCount := OverlapCountIter(a, b)

		return excludeCount+intersectCount == a.UniqueIPs
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 2048}); err != nil {
		t.Fatal(err)
	}
}

func TestIterIdentityDiffEqualsSymmetric(t *testing.T) {
	// Diff(a,b) == Union(Exclude(a,b), Exclude(b,a))
	fn := func(a1, a2, b1, b2 uint8) bool {
		aLo, aHi := ordered(uint32(a1), uint32(a2))
		bLo, bHi := ordered(uint32(b1), uint32(b2))
		a := setFromRanges("a", Range{Lo: aLo, Hi: aHi})
		b := setFromRanges("b", Range{Lo: bLo, Hi: bHi})

		diffRanges := collectIter(DiffIter(a, b))

		excAB := setFromIter("excAB", ExcludeIter(a, b))
		excBA := setFromIter("excBA", ExcludeIter(b, a))
		unionRanges := collectIter(UnionIter(excAB, excBA))

		return rangeSlicesEqual(diffRanges, unionRanges)
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 2048}); err != nil {
		t.Fatal(err)
	}
}

// --- FileSet integration tests -----------------------------------------------

func TestIterOpsWithFileSet(t *testing.T) {
	a := setFromRanges("a",
		Range{Lo: 1, Hi: 10},
		Range{Lo: 20, Hi: 30},
		Range{Lo: 50, Hi: 60},
	)
	b := setFromRanges("b",
		Range{Lo: 5, Hi: 25},
		Range{Lo: 55, Hi: 70},
	)

	pathA := writeTempSet(t, a)
	pathB := writeTempSet(t, b)
	fsA, err := OpenFileSet(pathA)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fsA.Close() }()
	fsB, err := OpenFileSet(pathB)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fsB.Close() }()

	memIntersect := collectIter(IntersectIter(a, b))
	fsIntersect := collectIter(IntersectIter(fsA, fsB))
	expectRangeSlice(t, "FileSet IntersectIter", fsIntersect, memIntersect)

	memExclude := collectIter(ExcludeIter(a, b))
	fsExclude := collectIter(ExcludeIter(fsA, fsB))
	expectRangeSlice(t, "FileSet ExcludeIter", fsExclude, memExclude)

	memDiff := collectIter(DiffIter(a, b))
	fsDiff := collectIter(DiffIter(fsA, fsB))
	expectRangeSlice(t, "FileSet DiffIter", fsDiff, memDiff)

	memUnion := collectIter(UnionIter(a, b))
	fsUnion := collectIter(UnionIter(fsA, fsB))
	expectRangeSlice(t, "FileSet UnionIter", fsUnion, memUnion)

	memOvlp := OverlapCountIter(a, b)
	fsOvlp := OverlapCountIter(fsA, fsB)
	if memOvlp != fsOvlp {
		t.Fatalf("OverlapCountIter: mem=%d fs=%d", memOvlp, fsOvlp)
	}

	memCount := CountUniqueIter(a)
	fsCount := CountUniqueIter(fsA)
	if memCount != fsCount {
		t.Fatalf("CountUniqueIter: mem=%d fs=%d", memCount, fsCount)
	}
}

func TestIterOpsWithFileSetLarge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large FileSet test in short mode")
	}

	const numRanges = 50_000
	rng := rand.New(rand.NewPCG(42, 0))
	a := randomSet(rng, "a", numRanges)
	b := randomSet(rng, "b", numRanges)

	pathA := writeTempSet(t, a)
	pathB := writeTempSet(t, b)
	fsA, err := OpenFileSet(pathA)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fsA.Close() }()
	fsB, err := OpenFileSet(pathB)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fsB.Close() }()

	memOverlap := OverlapCountIter(a, b)
	fsOverlap := OverlapCountIter(fsA, fsB)
	if memOverlap != fsOverlap {
		t.Fatalf("large OverlapCountIter: mem=%d fs=%d", memOverlap, fsOverlap)
	}

	memUnionCount := countIterIPs(UnionIter(a, b))
	fsUnionCount := countIterIPs(UnionIter(fsA, fsB))
	if memUnionCount != fsUnionCount {
		t.Fatalf("large UnionIter count: mem=%d fs=%d", memUnionCount, fsUnionCount)
	}
}

// --- Memory bound test -------------------------------------------------------

func TestIterOpsMemoryBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test in short mode")
	}

	const numRanges = 500_000
	rng := rand.New(rand.NewPCG(99, 0))
	var pathA, pathB string
	{
		a := randomSet(rng, "a", numRanges)
		b := randomSet(rng, "b", numRanges)
		pathA = writeTempSet(t, a)
		pathB = writeTempSet(t, b)
	}

	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	fsA, err := OpenFileSet(pathA)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fsA.Close() }()
	fsB, err := OpenFileSet(pathB)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fsB.Close() }()

	// Run all iter operations — none should materialize data.
	_ = OverlapCountIter(fsA, fsB)
	_ = countIterIPs(ExcludeIter(fsA, fsB))
	_ = countIterIPs(UnionIter(fsA, fsB))
	_ = countIterIPs(DiffIter(fsA, fsB))
	_ = CountUniqueIter(fsA)

	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	// 500K ranges = 4MB per set on disk. Allow up to 2MB heap growth for
	// iterator machinery, GC noise, test framework, etc.
	const maxHeapGrowth = 2 * 1024 * 1024
	heapGrowth := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("heap before=%d after=%d growth=%d (max allowed=%d)",
		before.HeapAlloc, after.HeapAlloc, heapGrowth, maxHeapGrowth)

	if heapGrowth > maxHeapGrowth {
		t.Fatalf("heap grew by %d bytes (>%d), iterator ops likely materialized data",
			heapGrowth, maxHeapGrowth)
	}
}

// --- Mixed RangeSource tests (IPSet + FileSet) -------------------------------

func TestIterOpsMixedSources(t *testing.T) {
	a := setFromRanges("a",
		Range{Lo: 1, Hi: 10},
		Range{Lo: 50, Hi: 60},
	)
	b := setFromRanges("b",
		Range{Lo: 5, Hi: 55},
	)

	pathB := writeTempSet(t, b)
	fsB, err := OpenFileSet(pathB)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fsB.Close() }()

	got := collectIter(IntersectIter(a, fsB))
	want := collectIter(IntersectIter(a, b))
	expectRangeSlice(t, "mixed IntersectIter", got, want)
}
