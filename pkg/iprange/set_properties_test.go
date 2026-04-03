package iprange

import (
	"testing"
	"testing/quick"
)

func TestSetAlgebraProperties(t *testing.T) {
	t.Parallel()

	cfg := &quick.Config{MaxCount: 1024}

	t.Run("union_idempotent", func(t *testing.T) {
		fn := func(raw [8]uint8) bool {
			a := quickSetFromBytes("a", raw)
			return sameSet(quickUnion(a, a), a)
		}
		if err := quick.Check(fn, cfg); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("union_commutative", func(t *testing.T) {
		fn := func(left, right [8]uint8) bool {
			a := quickSetFromBytes("a", left)
			b := quickSetFromBytes("b", right)
			return sameSet(quickUnion(a, b), quickUnion(b, a))
		}
		if err := quick.Check(fn, cfg); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("intersect_commutative", func(t *testing.T) {
		fn := func(left, right [8]uint8) bool {
			a := quickSetFromBytes("a", left)
			b := quickSetFromBytes("b", right)
			return sameSet(Intersect(a, b), Intersect(b, a))
		}
		if err := quick.Check(fn, cfg); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("exclude_idempotent", func(t *testing.T) {
		fn := func(left, right [8]uint8) bool {
			a := quickSetFromBytes("a", left)
			b := quickSetFromBytes("b", right)
			once := Exclude(a, b)
			return sameSet(Exclude(once, b), once)
		}
		if err := quick.Check(fn, cfg); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("exclude_and_intersect_partition_left", func(t *testing.T) {
		fn := func(left, right [8]uint8) bool {
			a := quickSetFromBytes("a", left)
			b := quickSetFromBytes("b", right)
			excluded := Exclude(a, b)
			common := Intersect(a, b)
			return excluded.UniqueCount()+common.UniqueCount() == a.UniqueCount()
		}
		if err := quick.Check(fn, cfg); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("pointwise_membership", func(t *testing.T) {
		fn := func(left, right [8]uint8) bool {
			a := quickSetFromBytes("a", left)
			b := quickSetFromBytes("b", right)
			union := quickUnion(a, b)
			common := Intersect(a, b)
			excluded := Exclude(a, b)
			for ip := range uint32(256) {
				inA := a.Contains(ip)
				inB := b.Contains(ip)
				if union.Contains(ip) != (inA || inB) {
					return false
				}
				if common.Contains(ip) != (inA && inB) {
					return false
				}
				if excluded.Contains(ip) != (inA && !inB) {
					return false
				}
			}
			return true
		}
		if err := quick.Check(fn, cfg); err != nil {
			t.Fatal(err)
		}
	})
}

func quickSetFromBytes(name string, raw [8]uint8) *IPSet {
	set := New(name)
	for i := 0; i < len(raw); i += 2 {
		lo, hi := ordered(uint32(raw[i]), uint32(raw[i+1]))
		_ = set.Add(lo, hi)
	}
	set.Optimize()
	return set
}

func quickUnion(a, b *IPSet) *IPSet {
	out := Combine(a, b)
	out.Optimize()
	return out
}

func sameSet(a, b *IPSet) bool {
	if a.UniqueCount() != b.UniqueCount() {
		return false
	}
	for ip := range uint32(256) {
		if a.Contains(ip) != b.Contains(ip) {
			return false
		}
	}
	return true
}
