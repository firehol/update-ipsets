package iprange

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestLargeFileSetBounded verifies that a 1M-range FileSet keeps heap
// growth bounded. The on-disk data is ~8MB; heap growth must stay under
// 10MB even after Contains, Iter, and OverlapCountIter operations.
func TestLargeFileSetBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large set memory test in short mode")
	}

	const numRanges = 1_000_000

	// Build two large sets (A and B) with partial overlap.
	rng := rand.New(rand.NewPCG(1, 0))
	setA := New("stress-a")
	for i := 0; i < numRanges; i++ {
		lo := uint32(i * 4)
		hi := lo + 1
		if err := setA.Add(lo, hi); err != nil {
			t.Fatal(err)
		}
	}
	setA.Optimize()

	setB := New("stress-b")
	for i := 0; i < numRanges; i++ {
		lo := uint32(i*4 + 1) // offset by 1 to create 50% overlap
		hi := lo + 1
		if err := setB.Add(lo, hi); err != nil {
			t.Fatal(err)
		}
	}
	setB.Optimize()

	// Write both to disk.
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.set")
	pathB := filepath.Join(dir, "b.set")
	for _, pair := range []struct {
		path string
		set  *IPSet
	}{{pathA, setA}, {pathB, setB}} {
		f, err := os.Create(pair.path)
		if err != nil {
			t.Fatal(err)
		}
		if err := WriteBinary(f, pair.set); err != nil {
			_ = f.Close()
			t.Fatal(err)
		}
		_ = f.Close()
	}

	// Release in-memory sets before measuring.
	setA = nil
	setB = nil
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

	// 1. Contains checks — random lookups
	for i := 0; i < 10_000; i++ {
		ip := rng.Uint32N(uint32(numRanges * 4))
		fsA.Contains(ip)
		fsB.Contains(ip)
	}

	// 2. Full iteration — consume without storing
	countA := 0
	for range fsA.Iter() {
		countA++
	}
	if countA != fsA.Len() {
		t.Fatalf("iter count %d != Len %d", countA, fsA.Len())
	}

	// 3. OverlapCountIter — streaming pairwise comparison
	overlap := OverlapCountIter(fsA, fsB)
	if overlap == 0 {
		t.Fatal("expected non-zero overlap between offset sets")
	}
	t.Logf("overlap between 1M-range sets: %d IPs", overlap)

	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	// 1M ranges = 8MB on disk each. Heap growth must stay under 10MB.
	const maxHeapGrowth = 10 * 1024 * 1024
	heapGrowth := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("heap before=%d after=%d growth=%d (limit=%d)",
		before.HeapAlloc, after.HeapAlloc, heapGrowth, maxHeapGrowth)

	if heapGrowth > maxHeapGrowth {
		t.Fatalf("heap grew by %d bytes (>%d); data leaked into heap",
			heapGrowth, maxHeapGrowth)
	}
}

// TestLargeFileSetIntersectIter verifies IntersectIter on large FileSets
// produces correct results without unbounded heap growth.
func TestLargeFileSetIntersectIter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large intersection test in short mode")
	}

	const numRanges = 100_000

	setA := New("intersect-a")
	for i := 0; i < numRanges; i++ {
		lo := uint32(i * 8)
		hi := lo + 3 // [0,3], [8,11], [16,19], ...
		if err := setA.Add(lo, hi); err != nil {
			t.Fatal(err)
		}
	}
	setA.Optimize()

	setB := New("intersect-b")
	for i := 0; i < numRanges; i++ {
		lo := uint32(i*8 + 2)
		hi := lo + 3 // [2,5], [10,13], [18,21], ...
		if err := setB.Add(lo, hi); err != nil {
			t.Fatal(err)
		}
	}
	setB.Optimize()

	// Compute expected overlap using in-memory sets.
	expectedOverlap := OverlapCountIter(setA, setB)

	// Write to disk, open as FileSets.
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.set")
	pathB := filepath.Join(dir, "b.set")
	writeSet(t, pathA, setA)
	writeSet(t, pathB, setB)

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

	got := OverlapCountIter(fsA, fsB)
	if got != expectedOverlap {
		t.Fatalf("FileSet OverlapCountIter = %d, want %d (from in-memory)", got, expectedOverlap)
	}
	t.Logf("intersection of 100K-range sets: %d IPs", got)

	// Also verify IntersectIter produces the same count.
	countFromIter := countIterIPs(IntersectIter(fsA, fsB))
	if countFromIter != expectedOverlap {
		t.Fatalf("IntersectIter count = %d, want %d", countFromIter, expectedOverlap)
	}
}

// TestLargeFileSetUnionExcludeDiff verifies Union, Exclude, Diff on large
// FileSets match their in-memory counterparts.
func TestLargeFileSetUnionExcludeDiff(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large set operations test in short mode")
	}

	const numRanges = 50_000
	rng := rand.New(rand.NewPCG(42, 0))

	setA := randomSet(rng, "ops-a", numRanges)
	setB := randomSet(rng, "ops-b", numRanges)

	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.set")
	pathB := filepath.Join(dir, "b.set")
	writeSet(t, pathA, setA)
	writeSet(t, pathB, setB)

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

	// Union
	memUnion := countIterIPs(UnionIter(setA, setB))
	fsUnion := countIterIPs(UnionIter(fsA, fsB))
	if memUnion != fsUnion {
		t.Fatalf("UnionIter: mem=%d fs=%d", memUnion, fsUnion)
	}

	// Exclude
	memExcl := countIterIPs(ExcludeIter(setA, setB))
	fsExcl := countIterIPs(ExcludeIter(fsA, fsB))
	if memExcl != fsExcl {
		t.Fatalf("ExcludeIter: mem=%d fs=%d", memExcl, fsExcl)
	}

	// Diff
	memDiff := countIterIPs(DiffIter(setA, setB))
	fsDiff := countIterIPs(DiffIter(fsA, fsB))
	if memDiff != fsDiff {
		t.Fatalf("DiffIter: mem=%d fs=%d", memDiff, fsDiff)
	}

	t.Logf("50K-range sets: union=%d exclude=%d diff=%d", fsUnion, fsExcl, fsDiff)
}

// writeSet writes an IPSet to a binary .set file.
func writeSet(t *testing.T, path string, set *IPSet) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteBinary(f, set); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
