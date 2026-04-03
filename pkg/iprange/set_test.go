package iprange

import (
	"testing"
	"testing/quick"
)

func TestOptimizeMergesRanges(t *testing.T) {
	set := New("test")
	mustAdd(t, set, 10, 20)
	mustAdd(t, set, 5, 8)
	mustAdd(t, set, 9, 9)
	mustAdd(t, set, 15, 18)

	set.Optimize()

	if len(set.Ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(set.Ranges))
	}
	if set.Ranges[0] != (Range{Lo: 5, Hi: 20}) {
		t.Fatalf("unexpected optimized range: %#v", set.Ranges[0])
	}
	if set.UniqueIPs != 16 {
		t.Fatalf("unexpected unique IP count: %d", set.UniqueIPs)
	}
}

func TestExcludeIntersectDiff(t *testing.T) {
	a := newOptimizedSet("a", Range{Lo: 1, Hi: 10})
	b := newOptimizedSet("b", Range{Lo: 5, Hi: 7})

	excluded := Exclude(a, b)
	expectRanges(t, excluded, []Range{{Lo: 1, Hi: 4}, {Lo: 8, Hi: 10}})

	common := Intersect(a, b)
	expectRanges(t, common, []Range{{Lo: 5, Hi: 7}})

	diff := Diff(a, b)
	expectRanges(t, diff, []Range{{Lo: 1, Hi: 4}, {Lo: 8, Hi: 10}})
}

func TestContains(t *testing.T) {
	set := newOptimizedSet("test", Range{Lo: 100, Hi: 120}, Range{Lo: 200, Hi: 220})
	for _, ip := range []uint32{100, 111, 220} {
		if !set.Contains(ip) {
			t.Fatalf("expected set to contain %d", ip)
		}
	}
	for _, ip := range []uint32{99, 121, 221} {
		if set.Contains(ip) {
			t.Fatalf("expected set not to contain %d", ip)
		}
	}
}

func TestSetIdentityQuick(t *testing.T) {
	fn := func(a1, a2, b1, b2 uint8) bool {
		aLo, aHi := ordered(uint32(a1), uint32(a2))
		bLo, bHi := ordered(uint32(b1), uint32(b2))
		a := newOptimizedSet("a", Range{Lo: aLo, Hi: aHi})
		b := newOptimizedSet("b", Range{Lo: bLo, Hi: bHi})

		excluded := Exclude(a, b)
		common := Intersect(a, b)
		return excluded.UniqueCount()+common.UniqueCount() == a.UniqueCount()
	}

	if err := quick.Check(fn, &quick.Config{MaxCount: 512}); err != nil {
		t.Fatal(err)
	}
}

func newOptimizedSet(name string, ranges ...Range) *IPSet {
	set := New(name)
	for _, r := range ranges {
		set.Ranges = append(set.Ranges, r)
		set.Lines++
	}
	set.Optimize()
	return set
}

func mustAdd(t *testing.T, set *IPSet, lo, hi uint32) {
	t.Helper()
	if err := set.Add(lo, hi); err != nil {
		t.Fatal(err)
	}
}

func expectRanges(t *testing.T, set *IPSet, want []Range) {
	t.Helper()
	set.Optimize()
	if len(set.Ranges) != len(want) {
		t.Fatalf("expected %d ranges, got %d: %#v", len(want), len(set.Ranges), set.Ranges)
	}
	for i := range want {
		if set.Ranges[i] != want[i] {
			t.Fatalf("range %d mismatch: got %#v want %#v", i, set.Ranges[i], want[i])
		}
	}
}

func ordered(a, b uint32) (uint32, uint32) {
	if a <= b {
		return a, b
	}
	return b, a
}
