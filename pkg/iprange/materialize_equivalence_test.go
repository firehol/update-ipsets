package iprange

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

func TestMaterializedSourceOpsRandomizedEquivalence(t *testing.T) {
	cases := materializedEquivalenceRangeCases()
	rng := rand.New(rand.NewPCG(0x5109, 0x20260620))
	for i := range 64 {
		cases = append(cases, materializedRangeCase{
			name:  fmt.Sprintf("random_%02d", i),
			left:  randomizedEquivalenceRanges(rng, "left", 96).Ranges,
			right: randomizedEquivalenceRanges(rng, "right", 96).Ranges,
		})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			leftSet := setFromRanges("left", tc.left...)
			rightSet := setFromRanges("right", tc.right...)

			wantUnion := mustCollectIter(t, "want_union", UnionIter(leftSet, rightSet))
			wantIntersect := mustCollectIter(t, "want_intersect", IntersectIter(leftSet, rightSet))
			wantExclude := mustCollectIter(t, "want_exclude", ExcludeIter(leftSet, rightSet))
			wantExcludeCount := countIterIPs(ExcludeIter(leftSet, rightSet))

			for _, sc := range materializedPairSourceCases(t, leftSet, rightSet) {
				t.Run(sc.name, func(t *testing.T) {
					gotUnion, err := UnionSourcesContext(t.Context(), "got_union", sc.left, sc.right)
					if err != nil {
						t.Fatalf("UnionSourcesContext() error = %v", err)
					}
					expectSetEquivalent(t, "union", gotUnion, wantUnion)

					gotIntersect, err := IntersectSourcesContext(t.Context(), "got_intersect", sc.left, sc.right)
					if err != nil {
						t.Fatalf("IntersectSourcesContext() error = %v", err)
					}
					expectSetEquivalent(t, "intersect", gotIntersect, wantIntersect)

					gotExclude, err := ExcludeSourcesContext(t.Context(), "got_exclude", sc.left, sc.right)
					if err != nil {
						t.Fatalf("ExcludeSourcesContext() error = %v", err)
					}
					expectSetEquivalent(t, "exclude", gotExclude, wantExclude)

					gotExcludeCount, err := ExcludeCountContext(t.Context(), sc.left, sc.right)
					if err != nil {
						t.Fatalf("ExcludeCountContext() error = %v", err)
					}
					if gotExcludeCount != wantExcludeCount {
						t.Fatalf("ExcludeCountContext() = %d, want %d", gotExcludeCount, wantExcludeCount)
					}

					var walked []Range
					if err := ExcludeRangesContext(t.Context(), sc.left, sc.right, func(r Range) bool {
						walked = append(walked, r)
						return true
					}); err != nil {
						t.Fatalf("ExcludeRangesContext() error = %v", err)
					}
					expectSetEquivalent(t, "exclude scanner", setFromRanges("walked", walked...), wantExclude)
				})
			}
		})
	}
}

func TestUnionSourcesContextRandomizedKWayEquivalence(t *testing.T) {
	rng := rand.New(rand.NewPCG(0x5110, 0x20260620))
	for i := range 32 {
		t.Run(fmt.Sprintf("random_%02d", i), func(t *testing.T) {
			sets := []*IPSet{
				randomizedEquivalenceRanges(rng, "a", 64),
				randomizedEquivalenceRanges(rng, "b", 64),
				randomizedEquivalenceRanges(rng, "c", 64),
				randomizedEquivalenceRanges(rng, "d", 64),
			}
			want := mustCollectIter(t, "want_union", UnionIter(
				sets[0],
				sets[1],
				sets[2],
				sets[3],
			))

			for _, sc := range materializedKWaySourceCases(t, sets) {
				t.Run(sc.name, func(t *testing.T) {
					got, err := UnionSourcesContext(t.Context(), "got_union", sc.sources...)
					if err != nil {
						t.Fatalf("UnionSourcesContext() error = %v", err)
					}
					expectSetEquivalent(t, "k-way union", got, want)
				})
			}
		})
	}
}

type materializedRangeCase struct {
	name        string
	left, right []Range
}

func materializedEquivalenceRangeCases() []materializedRangeCase {
	max := ^uint32(0)
	return []materializedRangeCase{
		{name: "both_empty"},
		{name: "left_empty", right: []Range{{Lo: 10, Hi: 20}}},
		{name: "right_empty", left: []Range{{Lo: 10, Hi: 20}}},
		{name: "same_single", left: []Range{{Lo: 42, Hi: 42}}, right: []Range{{Lo: 42, Hi: 42}}},
		{name: "disjoint", left: []Range{{Lo: 1, Hi: 10}}, right: []Range{{Lo: 20, Hi: 30}}},
		{name: "full_left", left: []Range{{Lo: 0, Hi: max}}, right: []Range{{Lo: 10, Hi: 20}, {Lo: max - 10, Hi: max}}},
		{name: "full_right", left: []Range{{Lo: 10, Hi: 20}, {Lo: max - 10, Hi: max}}, right: []Range{{Lo: 0, Hi: max}}},
		{name: "max_edge", left: []Range{{Lo: max - 20, Hi: max - 10}, {Lo: max - 5, Hi: max}}, right: []Range{{Lo: max - 15, Hi: max}}},
		{name: "alternating", left: alternatingRanges(0, 2, 64), right: alternatingRanges(1, 2, 64)},
		{name: "dense_overlap", left: denseRanges(1000, 7, 80), right: denseRanges(1020, 9, 80)},
	}
}

func materializedPairSourceCases(t *testing.T, leftSet, rightSet *IPSet) []struct {
	name        string
	left, right RangeSource
} {
	t.Helper()
	leftFile, err := OpenFileSet(writeTempSet(t, leftSet))
	if err != nil {
		t.Fatalf("OpenFileSet(left): %v", err)
	}
	t.Cleanup(func() { _ = leftFile.Close() })
	rightFile, err := OpenFileSet(writeTempSet(t, rightSet))
	if err != nil {
		t.Fatalf("OpenFileSet(right): %v", err)
	}
	t.Cleanup(func() { _ = rightFile.Close() })

	leftGeneric := RangeSourceFromIter(leftSet.Iter(), leftSet.Len())
	rightGeneric := RangeSourceFromIter(rightSet.Iter(), rightSet.Len())

	return []struct {
		name        string
		left, right RangeSource
	}{
		{name: "memory_memory", left: leftSet, right: rightSet},
		{name: "fileset_fileset", left: leftFile, right: rightFile},
		{name: "memory_fileset", left: leftSet, right: rightFile},
		{name: "fileset_memory", left: leftFile, right: rightSet},
		{name: "generic_generic", left: leftGeneric, right: rightGeneric},
		{name: "generic_fileset", left: leftGeneric, right: rightFile},
		{name: "fileset_generic", left: leftFile, right: rightGeneric},
	}
}

func materializedKWaySourceCases(t *testing.T, sets []*IPSet) []struct {
	name    string
	sources []RangeSource
} {
	t.Helper()
	files := make([]FileSet, 0, len(sets))
	for _, set := range sets {
		fs, err := OpenFileSet(writeTempSet(t, set))
		if err != nil {
			t.Fatalf("OpenFileSet(%s): %v", set.Name, err)
		}
		files = append(files, fs)
	}
	t.Cleanup(func() {
		for _, fs := range files {
			_ = fs.Close()
		}
	})

	memory := make([]RangeSource, len(sets))
	fileSources := make([]RangeSource, len(sets))
	generic := make([]RangeSource, len(sets))
	mixed := make([]RangeSource, len(sets))
	for i, set := range sets {
		memory[i] = set
		fileSources[i] = files[i]
		generic[i] = RangeSourceFromIter(set.Iter(), set.Len())
		switch i % 3 {
		case 0:
			mixed[i] = files[i]
		case 1:
			mixed[i] = set
		default:
			mixed[i] = generic[i]
		}
	}

	return []struct {
		name    string
		sources []RangeSource
	}{
		{name: "all_memory", sources: memory},
		{name: "all_fileset", sources: fileSources},
		{name: "all_generic", sources: generic},
		{name: "mixed", sources: mixed},
	}
}

func mustCollectIter(t *testing.T, name string, seq func(yield func(Range) bool)) *IPSet {
	t.Helper()
	set, err := CollectIterContext(t.Context(), name, seq)
	if err != nil {
		t.Fatalf("CollectIterContext(%s): %v", name, err)
	}
	return set
}

func expectSetEquivalent(t *testing.T, label string, got, want *IPSet) {
	t.Helper()
	got.Optimize()
	want.Optimize()
	expectRangeSlice(t, label, got.Ranges, want.Ranges)
	if got.UniqueIPs != want.UniqueIPs {
		t.Fatalf("%s UniqueIPs = %d, want %d", label, got.UniqueIPs, want.UniqueIPs)
	}
}

func randomizedEquivalenceRanges(rng *rand.Rand, name string, maxRanges int) *IPSet {
	set := New(name)
	count := int(rng.Uint32N(uint32(maxRanges + 1)))
	for range count {
		var lo, span uint32
		switch rng.Uint32N(6) {
		case 0:
			lo = rng.Uint32N(4096)
			span = rng.Uint32N(512)
		case 1:
			lo = 1_000_000 + rng.Uint32N(4096)
			span = 2048 + rng.Uint32N(4096)
		case 2:
			lo = ^uint32(0) - rng.Uint32N(4096)
			span = rng.Uint32N(4096)
		case 3:
			lo = rng.Uint32() &^ 1
			span = rng.Uint32N(1)
		case 4:
			lo = rng.Uint32()
			span = rng.Uint32N(64)
		default:
			lo = rng.Uint32()
			span = rng.Uint32N(4096)
		}
		hi := lo + span
		if hi < lo {
			hi = ^uint32(0)
		}
		if err := set.AddRange(Range{Lo: lo, Hi: hi}); err != nil {
			panic(err)
		}
	}
	set.Optimize()
	return set
}

func alternatingRanges(start, step, count uint32) []Range {
	out := make([]Range, 0, count)
	for i := uint32(0); i < count; i++ {
		lo := start + i*step
		out = append(out, Range{Lo: lo, Hi: lo})
	}
	return out
}

func denseRanges(start, step, count uint32) []Range {
	out := make([]Range, 0, count)
	for i := uint32(0); i < count; i++ {
		lo := start + i*step
		out = append(out, Range{Lo: lo, Hi: lo + step + 10})
	}
	return out
}
