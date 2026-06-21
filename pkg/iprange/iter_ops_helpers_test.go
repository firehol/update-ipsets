package iprange

import (
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

// setFromRanges is a convenience to build an optimized IPSet from ranges.
func setFromRanges(name string, ranges ...Range) *IPSet {
	return newOptimizedSet(name, ranges...)
}

func expectRangeSlice(t *testing.T, label string, got, want []Range) {
	t.Helper()
	if !rangeSlicesEqual(got, want) {
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
