package iprange

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkParseIPs(b *testing.B) {
	var input bytes.Buffer
	for i := 0; i < 10000; i++ {
		fmt.Fprintf(&input, "10.0.%d.%d\n", (i/256)%256, i%256)
	}
	data := input.Bytes()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := ParseReader(b.Context(), "bench", bytes.NewReader(data), DefaultParseOptions()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOptimize(b *testing.B) {
	template := New("bench")
	for i := 10000; i > 0; i-- {
		mustAddBench(b, template, uint32(i*2), uint32(i*2))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		set := template.Clone()
		set.Optimize()
	}
}

func BenchmarkCompare(b *testing.B) {
	sets := make([]*IPSet, 0, 25)
	for i := 0; i < 25; i++ {
		set := New(fmt.Sprintf("set-%d", i))
		for j := 0; j < 256; j++ {
			mustAddBench(b, set, uint32(i*1000+j*2), uint32(i*1000+j*2+1))
		}
		set.Optimize()
		sets = append(sets, set)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := CompareAll(sets); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompareNextSourcesFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			fsA, fsB := benchPairFileSets(b, size)
			before := []CompareSource{{Name: "before", Source: fsA}}
			after := []CompareSource{{Name: "after", Source: fsB}}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				rows, err := CompareNextSources(b.Context(), before, after)
				if err != nil {
					b.Fatal(err)
				}
				if len(rows) != 1 {
					b.Fatalf("CompareNextSources() returned %d rows, want 1", len(rows))
				}
			}
		})
	}
}

func BenchmarkBinaryRoundTrip(b *testing.B) {
	set := New("binary")
	for i := 0; i < 4096; i++ {
		mustAddBench(b, set, uint32(i*4), uint32(i*4+3))
	}
	set.Optimize()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var buf bytes.Buffer
		if err := WriteBinary(&buf, set); err != nil {
			b.Fatal(err)
		}
		if _, err := ReadBinary("binary", bytes.NewReader(buf.Bytes())); err != nil {
			b.Fatal(err)
		}
	}
}

func mustAddBench(tb testing.TB, set *IPSet, lo, hi uint32) {
	tb.Helper()
	if err := set.Add(lo, hi); err != nil {
		tb.Fatal(err)
	}
}

// benchFileSetFromRanges creates a binary .set file with numRanges ranges.
func benchFileSetFromRanges(b *testing.B, numRanges int) string {
	b.Helper()
	set := New("bench")
	for i := 0; i < numRanges; i++ {
		lo := uint32(i * 4)
		hi := lo + 1
		if err := set.Add(lo, hi); err != nil {
			b.Fatal(err)
		}
	}
	set.Optimize()

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
	_ = f.Close()
	return path
}

func BenchmarkFileSetContains(b *testing.B) {
	for _, size := range []int{1000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			path := benchFileSetFromRanges(b, size)
			fs, err := OpenFileSet(path)
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = fs.Close() }()

			rng := rand.New(rand.NewPCG(42, 0))
			maxIP := uint32(size * 4)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				fs.Contains(rng.Uint32N(maxIP))
			}
		})
	}
}

func BenchmarkSetContains(b *testing.B) {
	for _, size := range []int{1000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			set := New("bench")
			for i := 0; i < size; i++ {
				lo := uint32(i * 4)
				hi := lo + 1
				if err := set.Add(lo, hi); err != nil {
					b.Fatal(err)
				}
			}
			set.Optimize()

			rng := rand.New(rand.NewPCG(42, 0))
			maxIP := uint32(size * 4)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				set.Contains(rng.Uint32N(maxIP))
			}
		})
	}
}

func BenchmarkFileSetIter(b *testing.B) {
	for _, size := range []int{1000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			path := benchFileSetFromRanges(b, size)
			fs, err := OpenFileSet(path)
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = fs.Close() }()

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for range fs.Iter() {
				}
			}
		})
	}
}

func BenchmarkSetIter(b *testing.B) {
	for _, size := range []int{1000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			set := New("bench")
			for i := 0; i < size; i++ {
				lo := uint32(i * 4)
				hi := lo + 1
				if err := set.Add(lo, hi); err != nil {
					b.Fatal(err)
				}
			}
			set.Optimize()

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for range set.Ranges {
				}
			}
		})
	}
}

// --- Iterator-based operation benchmarks -------------------------------------

func benchPairSets(b *testing.B, size int) (*IPSet, *IPSet) {
	b.Helper()
	rng := rand.New(rand.NewPCG(42, 0))
	a := New("a")
	for i := 0; i < size; i++ {
		lo := rng.Uint32N(0xFFFFF000)
		span := rng.Uint32N(64)
		if err := a.Add(lo, lo+span); err != nil {
			b.Fatal(err)
		}
	}
	a.Optimize()

	bb := New("b")
	for i := 0; i < size; i++ {
		lo := rng.Uint32N(0xFFFFF000)
		span := rng.Uint32N(64)
		if err := bb.Add(lo, lo+span); err != nil {
			b.Fatal(err)
		}
	}
	bb.Optimize()
	return a, bb
}

func benchPairFileSets(b *testing.B, size int) (FileSet, FileSet) {
	b.Helper()
	a, bb := benchPairSets(b, size)
	pathA := writeTempSetBench(b, a)
	pathB := writeTempSetBench(b, bb)

	// Rename to avoid collision — TempDir is shared within sub-benchmark.
	dir := b.TempDir()
	newA := filepath.Join(dir, "a.set")
	newB := filepath.Join(dir, "b.set")
	copyFileBench(b, pathA, newA)
	copyFileBench(b, pathB, newB)

	fsA, err := OpenFileSet(newA)
	if err != nil {
		b.Fatal(err)
	}
	fsB, err := OpenFileSet(newB)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		_ = fsA.Close()
		_ = fsB.Close()
	})
	return fsA, fsB
}

func copyFileBench(b *testing.B, src, dst string) {
	b.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0600); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkOverlapCountInMemory(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			a, bb := benchPairSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				OverlapCountIter(a, bb)
			}
		})
	}
}

func BenchmarkOverlapCountFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			fsA, fsB := benchPairFileSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				OverlapCountIter(fsA, fsB)
			}
		})
	}
}

func BenchmarkIntersectIterInMemory(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			a, bb := benchPairSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for range IntersectIter(a, bb) {
				}
			}
		})
	}
}

func BenchmarkIntersectIterFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			fsA, fsB := benchPairFileSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for range IntersectIter(fsA, fsB) {
				}
			}
		})
	}
}

func BenchmarkUnionIterInMemory(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			a, bb := benchPairSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for range UnionIter(a, bb) {
				}
			}
		})
	}
}

func BenchmarkUnionIterFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			fsA, fsB := benchPairFileSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for range UnionIter(fsA, fsB) {
				}
			}
		})
	}
}

func BenchmarkExcludeIterInMemory(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			a, bb := benchPairSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for range ExcludeIter(a, bb) {
				}
			}
		})
	}
}

func BenchmarkCollectIterContextFileSetUnion(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			fsA, fsB := benchPairFileSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				set, err := CollectIterContext(b.Context(), "union", UnionIter(fsA, fsB))
				if err != nil {
					b.Fatal(err)
				}
				if set.Len() == 0 {
					b.Fatal("empty union")
				}
			}
		})
	}
}

func BenchmarkRangeSourcesEqualContextFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			path := benchFileSetFromRanges(b, size)
			fsA, err := OpenFileSet(path)
			if err != nil {
				b.Fatal(err)
			}
			fsB, err := OpenFileSet(path)
			if err != nil {
				_ = fsA.Close()
				b.Fatal(err)
			}
			b.Cleanup(func() {
				_ = fsA.Close()
				_ = fsB.Close()
			})
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				equal, err := RangeSourcesEqualContext(b.Context(), fsA, fsB)
				if err != nil {
					b.Fatal(err)
				}
				if !equal {
					b.Fatal("identical file-backed sets reported different")
				}
			}
		})
	}
}

func BenchmarkBuildRangeSourceSummaryFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			fsA, _ := benchPairFileSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				summary, err := BuildRangeSourceSummaryContext(b.Context(), fsA)
				if err != nil {
					b.Fatal(err)
				}
				if !summary.ContentHash.Valid {
					b.Fatal("summary missing content hash")
				}
			}
		})
	}
}

func BenchmarkRangeSourceContentHashFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			fsA, _ := benchPairFileSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				hash, err := RangeSourceContentHashContext(b.Context(), fsA)
				if err != nil {
					b.Fatal(err)
				}
				if !hash.Valid {
					b.Fatal("content hash invalid")
				}
			}
		})
	}
}

// --- IPv6 benchmarks ---------------------------------------------------------

func BenchmarkParseIPs6(b *testing.B) {
	var input bytes.Buffer
	for i := 0; i < 10000; i++ {
		fmt.Fprintf(&input, "2001:db8::%x\n", i)
	}
	data := input.Bytes()
	opts := DefaultParseOptions6()
	opts.DNSThreads = 1

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := ParseReader6(b.Context(), "bench", bytes.NewReader(data), opts); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOptimize6(b *testing.B) {
	template := New6("bench")
	for i := 10000; i > 0; i-- {
		v := u128FromUint64(uint64(i * 2))
		if err := template.Add6(v, v); err != nil {
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		set := template.Clone()
		set.Optimize()
	}
}

func BenchmarkBinary6RoundTrip(b *testing.B) {
	set := New6("binary")
	for i := 0; i < 4096; i++ {
		v := u128FromUint64(uint64(i * 4))
		if err := set.Add6(v, v.Add64(3)); err != nil {
			b.Fatal(err)
		}
	}
	set.Optimize()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var buf bytes.Buffer
		if err := WriteBinary6(&buf, set); err != nil {
			b.Fatal(err)
		}
		if _, err := ReadBinary6("binary", bytes.NewReader(buf.Bytes())); err != nil {
			b.Fatal(err)
		}
	}
}

func benchFileSet6FromRanges(b *testing.B, numRanges int) string {
	b.Helper()
	set := New6("bench")
	for i := 0; i < numRanges; i++ {
		lo := u128FromUint64(uint64(i * 4))
		if err := set.Add6(lo, lo.Add64(1)); err != nil {
			b.Fatal(err)
		}
	}
	set.Optimize()

	dir := b.TempDir()
	path := filepath.Join(dir, "bench6.set")
	f, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	if err := WriteBinary6(f, set); err != nil {
		_ = f.Close()
		b.Fatal(err)
	}
	_ = f.Close()
	return path
}

func BenchmarkFileSet6Contains(b *testing.B) {
	for _, size := range []int{1000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			path := benchFileSet6FromRanges(b, size)
			fs, err := OpenFileSet6(path)
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = fs.Close() }()

			rng := rand.New(rand.NewPCG(42, 0))
			maxIP := uint64(size * 4)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				fs.Contains(u128FromUint64(rng.Uint64N(maxIP)))
			}
		})
	}
}

func BenchmarkSet6Contains(b *testing.B) {
	for _, size := range []int{1000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			set := New6("bench")
			for i := 0; i < size; i++ {
				lo := u128FromUint64(uint64(i * 4))
				if err := set.Add6(lo, lo.Add64(1)); err != nil {
					b.Fatal(err)
				}
			}
			set.Optimize()

			rng := rand.New(rand.NewPCG(42, 0))
			maxIP := uint64(size * 4)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				set.Contains(u128FromUint64(rng.Uint64N(maxIP)))
			}
		})
	}
}

func benchPairSets6(b *testing.B, size int) (*IPSet6, *IPSet6) {
	b.Helper()
	rng := rand.New(rand.NewPCG(42, 0))
	a := New6("a")
	for i := 0; i < size; i++ {
		lo := u128FromUint64(rng.Uint64N(0xFFFFF000))
		span := u128FromUint64(rng.Uint64N(64))
		if err := a.Add6(lo, lo.Add(span)); err != nil {
			b.Fatal(err)
		}
	}
	a.Optimize()

	bb := New6("b")
	for i := 0; i < size; i++ {
		lo := u128FromUint64(rng.Uint64N(0xFFFFF000))
		span := u128FromUint64(rng.Uint64N(64))
		if err := bb.Add6(lo, lo.Add(span)); err != nil {
			b.Fatal(err)
		}
	}
	bb.Optimize()
	return a, bb
}

func BenchmarkOverlapCount6InMemory(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			a, bb := benchPairSets6(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				OverlapCountIter6(a, bb)
			}
		})
	}
}

func BenchmarkIntersectIter6InMemory(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			a, bb := benchPairSets6(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for range IntersectIter6(a, bb) {
				}
			}
		})
	}
}

func BenchmarkUnionIter6InMemory(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			a, bb := benchPairSets6(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for range UnionIter6(a, bb) {
				}
			}
		})
	}
}

func BenchmarkExcludeIter6InMemory(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			a, bb := benchPairSets6(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for range ExcludeIter6(a, bb) {
				}
			}
		})
	}
}
