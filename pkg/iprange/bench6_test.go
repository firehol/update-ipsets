package iprange

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

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

func benchPairFileSets6(b *testing.B, size int) (FileSet6, FileSet6) {
	b.Helper()
	a, bb := benchPairSets6(b, size)
	open := func(name string, set *IPSet6) FileSet6 {
		b.Helper()
		path := filepath.Join(b.TempDir(), name+".set")
		f, err := os.Create(path)
		if err != nil {
			b.Fatal(err)
		}
		if err := WriteBinary6(f, set); err != nil {
			_ = f.Close()
			b.Fatal(err)
		}
		if err := f.Close(); err != nil {
			b.Fatal(err)
		}
		fs, err := OpenFileSet6(path)
		if err != nil {
			b.Fatal(err)
		}
		b.Cleanup(func() { _ = fs.Close() })
		return fs
	}
	return open("a", a), open("b", bb)
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

func BenchmarkIntersectIter6FileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			a, bb := benchPairFileSets6(b, size)
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

func BenchmarkUnionIter6FileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			a, bb := benchPairFileSets6(b, size)
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

func BenchmarkExcludeIter6FileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			a, bb := benchPairFileSets6(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for range ExcludeIter6(a, bb) {
				}
			}
		})
	}
}
