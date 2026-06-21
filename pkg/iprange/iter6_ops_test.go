package iprange

import "testing"

// collectIter6 drains an IPv6 push iterator into a slice.
func collectIter6(it func(yield func(Range6) bool)) []Range6 {
	var out []Range6
	for r := range it {
		out = append(out, r)
	}
	return out
}

func TestOverlapCountIter6InMemoryAndFileSet(t *testing.T) {
	left := makeTestSet6(
		Range6{Lo: u128FromUint64(10), Hi: u128FromUint64(20)},
		Range6{Lo: u128FromUint64(40), Hi: u128FromUint64(50)},
	)
	right := makeTestSet6(
		Range6{Lo: u128FromUint64(15), Hi: u128FromUint64(25)},
		Range6{Lo: u128FromUint64(45), Hi: u128FromUint64(45)},
	)
	leftFile, err := OpenFileSet6(writeTestBinary6File(t, left))
	if err != nil {
		t.Fatalf("OpenFileSet6(left): %v", err)
	}
	defer func() { _ = leftFile.Close() }()
	rightFile, err := OpenFileSet6(writeTestBinary6File(t, right))
	if err != nil {
		t.Fatalf("OpenFileSet6(right): %v", err)
	}
	defer func() { _ = rightFile.Close() }()

	want := u128FromUint64(7)
	for _, tc := range []struct {
		name  string
		left  RangeSource6
		right RangeSource6
	}{
		{name: "memory_memory", left: left, right: right},
		{name: "fileset_fileset", left: leftFile, right: rightFile},
		{name: "memory_fileset", left: left, right: rightFile},
		{name: "fileset_memory", left: leftFile, right: right},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := OverlapCountIter6(tc.left, tc.right); !got.Equals(want) {
				t.Fatalf("OverlapCountIter6() = %s, want %s", got.String(), want.String())
			}
		})
	}
}

func TestIPv6IteratorsInMemory(t *testing.T) {
	leftRanges := []Range6{
		{Lo: u128FromUint64(1), Hi: u128FromUint64(5)},
		{Lo: u128FromUint64(10), Hi: u128FromUint64(15)},
		{Lo: u128FromUint64(20), Hi: u128FromUint64(25)},
	}
	left := makeTestSet6(
		leftRanges...,
	)
	right := makeTestSet6(
		Range6{Lo: u128FromUint64(3), Hi: u128FromUint64(12)},
		Range6{Lo: u128FromUint64(22), Hi: u128FromUint64(30)},
	)

	expectRange6Slice(t, "IntersectIter6", collectIter6(IntersectIter6(left, right)), []Range6{
		{Lo: u128FromUint64(3), Hi: u128FromUint64(5)},
		{Lo: u128FromUint64(10), Hi: u128FromUint64(12)},
		{Lo: u128FromUint64(22), Hi: u128FromUint64(25)},
	})
	expectRange6Slice(t, "ExcludeIter6", collectIter6(ExcludeIter6(left, right)), []Range6{
		{Lo: u128FromUint64(1), Hi: u128FromUint64(2)},
		{Lo: u128FromUint64(13), Hi: u128FromUint64(15)},
		{Lo: u128FromUint64(20), Hi: u128FromUint64(21)},
	})
	expectRange6Slice(t, "UnionIter6", collectIter6(UnionIter6(left, right)), []Range6{
		{Lo: u128FromUint64(1), Hi: u128FromUint64(15)},
		{Lo: u128FromUint64(20), Hi: u128FromUint64(30)},
	})
	expectRange6Slice(t, "source ranges after iteration", left.Ranges, leftRanges)
}

func TestIPv6IteratorsFileSetParity(t *testing.T) {
	left := makeTestSet6(
		Range6{Lo: u128FromUint64(1), Hi: u128FromUint64(5)},
		Range6{Lo: u128FromUint64(10), Hi: u128FromUint64(15)},
		Range6{Lo: u128FromUint64(20), Hi: u128FromUint64(25)},
		Range6{Lo: u128FromHiLo(0x20010db800000000, 0), Hi: u128FromHiLo(0x20010db800000000, 0xffff)},
	)
	right := makeTestSet6(
		Range6{Lo: u128FromUint64(3), Hi: u128FromUint64(12)},
		Range6{Lo: u128FromUint64(22), Hi: u128FromUint64(30)},
		Range6{Lo: u128FromHiLo(0x20010db800000000, 0x10), Hi: u128FromHiLo(0x20010db800000000, 0x20)},
	)
	leftFile, err := OpenFileSet6(writeTestBinary6File(t, left))
	if err != nil {
		t.Fatalf("OpenFileSet6(left): %v", err)
	}
	defer func() { _ = leftFile.Close() }()
	rightFile, err := OpenFileSet6(writeTestBinary6File(t, right))
	if err != nil {
		t.Fatalf("OpenFileSet6(right): %v", err)
	}
	defer func() { _ = rightFile.Close() }()

	wantIntersect := collectIter6(IntersectIter6(left, right))
	wantExclude := collectIter6(ExcludeIter6(left, right))
	wantDiff := collectIter6(DiffIter6(left, right))
	wantUnion := collectIter6(UnionIter6(left, right))

	for _, tc := range []struct {
		name  string
		left  RangeSource6
		right RangeSource6
	}{
		{name: "fileset_fileset", left: leftFile, right: rightFile},
		{name: "memory_fileset", left: left, right: rightFile},
		{name: "fileset_memory", left: leftFile, right: right},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expectRange6Slice(t, "IntersectIter6", collectIter6(IntersectIter6(tc.left, tc.right)), wantIntersect)
			expectRange6Slice(t, "ExcludeIter6", collectIter6(ExcludeIter6(tc.left, tc.right)), wantExclude)
			expectRange6Slice(t, "DiffIter6", collectIter6(DiffIter6(tc.left, tc.right)), wantDiff)
			expectRange6Slice(t, "UnionIter6", collectIter6(UnionIter6(tc.left, tc.right)), wantUnion)
		})
	}
}

func TestIPv6FileSetIteratorsAllocationShape(t *testing.T) {
	left := makeTestSet6(
		Range6{Lo: u128FromUint64(1), Hi: u128FromUint64(5)},
		Range6{Lo: u128FromUint64(10), Hi: u128FromUint64(15)},
		Range6{Lo: u128FromUint64(20), Hi: u128FromUint64(25)},
	)
	right := makeTestSet6(
		Range6{Lo: u128FromUint64(3), Hi: u128FromUint64(12)},
		Range6{Lo: u128FromUint64(22), Hi: u128FromUint64(30)},
	)
	leftFile, err := OpenFileSet6(writeTestBinary6File(t, left))
	if err != nil {
		t.Fatalf("OpenFileSet6(left): %v", err)
	}
	defer func() { _ = leftFile.Close() }()
	rightFile, err := OpenFileSet6(writeTestBinary6File(t, right))
	if err != nil {
		t.Fatalf("OpenFileSet6(right): %v", err)
	}
	defer func() { _ = rightFile.Close() }()

	wantIntersect := len(collectIter6(IntersectIter6(left, right)))
	wantExclude := len(collectIter6(ExcludeIter6(left, right)))
	wantDiff := len(collectIter6(DiffIter6(left, right)))
	wantUnion := len(collectIter6(UnionIter6(left, right)))

	for _, tc := range []struct {
		name string
		run  func() int
		want int
		max  float64
	}{
		{name: "intersect", run: func() int { return countRange6Iter(IntersectIter6(leftFile, rightFile)) }, want: wantIntersect, max: 6},
		{name: "exclude", run: func() int { return countRange6Iter(ExcludeIter6(leftFile, rightFile)) }, want: wantExclude, max: 6},
		{name: "diff", run: func() int { return countRange6Iter(DiffIter6(leftFile, rightFile)) }, want: wantDiff, max: 8},
		{name: "union", run: func() int { return countRange6Iter(UnionIter6(leftFile, rightFile)) }, want: wantUnion, max: 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			allocs := testing.AllocsPerRun(20, func() {
				if got := tc.run(); got != tc.want {
					panic("unexpected range count")
				}
			})
			if allocs > tc.max {
				t.Fatalf("%s allocations = %.0f, want <= %.0f", tc.name, allocs, tc.max)
			}
		})
	}
}

func countRange6Iter(it func(yield func(Range6) bool)) int {
	count := 0
	for range it {
		count++
	}
	return count
}

func expectRange6Slice(t *testing.T, label string, got, want []Range6) {
	t.Helper()
	if !range6SlicesEqual(got, want) {
		t.Fatalf("%s mismatch:\n  got  = %v\n  want = %v", label, got, want)
	}
}

func range6SlicesEqual(a, b []Range6) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Lo.Equals(b[i].Lo) || !a[i].Hi.Equals(b[i].Hi) {
			return false
		}
	}
	return true
}
