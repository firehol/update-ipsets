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

func BenchmarkParseIPsWithProgress(b *testing.B) {
	var input bytes.Buffer
	for i := 0; i < 10000; i++ {
		fmt.Fprintf(&input, "10.0.%d.%d\n", (i/256)%256, i%256)
	}
	data := input.Bytes()

	opts := DefaultParseOptions()
	var callbacks int
	opts.Progress = func(ParseProgress) {
		callbacks++
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		callbacks = 0
		if _, err := ParseReader(b.Context(), "bench", bytes.NewReader(data), opts); err != nil {
			b.Fatal(err)
		}
		if callbacks == 0 {
			b.Fatal("expected progress callbacks")
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

func BenchmarkCompareSourcePairsRepeatedLeftFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			for _, targetCount := range []int{8, 64} {
				b.Run(fmt.Sprintf("targets=%d", targetCount), func(b *testing.B) {
					left, err := OpenFileSet(benchFileSetFromOffsetRanges(b, size, 0))
					if err != nil {
						b.Fatal(err)
					}
					defer func() { _ = left.Close() }()

					sources := make([]CompareSource, 0, targetCount+1)
					pairs := make([]ComparePair, 0, targetCount)
					sources = append(sources, CompareSource{Name: "left", Source: left})
					for i := 0; i < targetCount; i++ {
						right, err := OpenFileSet(benchFileSetFromOffsetRanges(b, size, uint32(i%4)))
						if err != nil {
							b.Fatal(err)
						}
						defer func() { _ = right.Close() }()
						sources = append(sources, CompareSource{Name: fmt.Sprintf("right-%d", i), Source: right})
						pairs = append(pairs, ComparePair{Left: 0, Right: len(sources) - 1})
					}

					b.ReportAllocs()
					b.ResetTimer()
					for b.Loop() {
						rows, err := CompareSourcePairs(b.Context(), sources, pairs)
						if err != nil {
							b.Fatal(err)
						}
						if len(rows) != targetCount {
							b.Fatalf("CompareSourcePairs() returned %d rows, want %d", len(rows), targetCount)
						}
					}
				})
			}
		})
	}
}

func BenchmarkCompareSourcePairsPartitionedOneToManyFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			for _, targetCount := range []int{8, 64} {
				b.Run(fmt.Sprintf("targets=%d", targetCount), func(b *testing.B) {
					left, err := OpenFileSet(benchFileSetFromOffsetRanges(b, size, 0))
					if err != nil {
						b.Fatal(err)
					}
					defer func() { _ = left.Close() }()

					sources := make([]CompareSource, 0, targetCount+1)
					pairs := make([]ComparePair, 0, targetCount)
					sources = append(sources, CompareSource{Name: "left", Source: left})
					for i := 0; i < targetCount; i++ {
						right, err := OpenFileSet(benchPartitionFileSet(b, size, targetCount, i))
						if err != nil {
							b.Fatal(err)
						}
						defer func() { _ = right.Close() }()
						sources = append(sources, CompareSource{Name: fmt.Sprintf("right-%d", i), Source: right})
						pairs = append(pairs, ComparePair{Left: 0, Right: len(sources) - 1})
					}

					b.ReportAllocs()
					b.ResetTimer()
					for b.Loop() {
						rows, err := CompareSourcePairs(b.Context(), sources, pairs)
						if err != nil {
							b.Fatal(err)
						}
						if len(rows) != targetCount {
							b.Fatalf("CompareSourcePairs() returned %d rows, want %d", len(rows), targetCount)
						}
					}
				})
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
	return benchFileSetFromOffsetRanges(b, numRanges, 0)
}

// benchFileSetFromOffsetRanges creates a binary .set file with numRanges ranges.
func benchFileSetFromOffsetRanges(b *testing.B, numRanges int, offset uint32) string {
	b.Helper()
	set := New("bench")
	for i := 0; i < numRanges; i++ {
		lo := offset + uint32(i*4)
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

func benchPartitionFileSet(b *testing.B, numRanges, partitions, partition int) string {
	b.Helper()
	set := New("bench")
	for i := partition; i < numRanges; i += partitions {
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

func BenchmarkReadFileSetMetadata(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			path := benchFileSetFromRanges(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				meta, err := ReadFileSetMetadata(path)
				if err != nil {
					b.Fatal(err)
				}
				if meta.Records == 0 {
					b.Fatal("empty metadata")
				}
			}
		})
	}
}

func BenchmarkOpenFileSetStrict(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			path := benchFileSetFromRanges(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				fs, err := OpenFileSet(path)
				if err != nil {
					b.Fatal(err)
				}
				if fs.Len() == 0 {
					b.Fatal("empty fileset")
				}
				if err := fs.Close(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkOpenFileSetTrusted(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			path := benchFileSetFromRanges(b, size)
			opts := FileSetOpenOptions{TrustOptimizedPayload: true}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				fs, err := OpenFileSetWithOptions(path, opts)
				if err != nil {
					b.Fatal(err)
				}
				if fs.Len() == 0 {
					b.Fatal("empty fileset")
				}
				if err := fs.Close(); err != nil {
					b.Fatal(err)
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
