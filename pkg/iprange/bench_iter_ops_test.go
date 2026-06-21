package iprange

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

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

func BenchmarkExcludeIterFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			fsA, fsB := benchPairFileSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for range ExcludeIter(fsA, fsB) {
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

func BenchmarkUnionSourcesContextFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			fsA, fsB := benchPairFileSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				set, err := UnionSourcesContext(b.Context(), "union", fsA, fsB)
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

func BenchmarkIntersectSourcesContextFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			fsA, fsB := benchPairFileSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				set, err := IntersectSourcesContext(b.Context(), "intersect", fsA, fsB)
				if err != nil {
					b.Fatal(err)
				}
				_ = set.Len()
			}
		})
	}
}

func BenchmarkExcludeSourcesContextFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			fsA, fsB := benchPairFileSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				set, err := ExcludeSourcesContext(b.Context(), "exclude", fsA, fsB)
				if err != nil {
					b.Fatal(err)
				}
				if set.Len() == 0 {
					b.Fatal("empty exclude")
				}
			}
		})
	}
}

func BenchmarkExcludeCountContextFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			fsA, fsB := benchPairFileSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				count, err := ExcludeCountContext(b.Context(), fsA, fsB)
				if err != nil {
					b.Fatal(err)
				}
				if count == 0 {
					b.Fatal("empty exclude count")
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

func BenchmarkBuildRangeOverlapFilterFileSet(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			fsA, _ := benchPairFileSets(b, size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				filter, err := BuildRangeOverlapFilterContext(b.Context(), fsA)
				if err != nil {
					b.Fatal(err)
				}
				if !filter.Valid() {
					b.Fatal("overlap filter invalid")
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
