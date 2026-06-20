package iprange

import (
	"container/heap"
	"iter"
)

type RangeSource6 interface {
	Len() int
	Iter() func(yield func(Range6) bool)
}

func CountUniqueIter6(src RangeSource6) Uint128 {
	var total Uint128
	for r := range src.Iter() {
		total = total.Add(r.Size())
	}
	return total
}

func OverlapCountIter6(a, b RangeSource6) Uint128 {
	var count Uint128
	for r := range IntersectIter6(a, b) {
		count = count.Add(r.Size())
	}
	return count
}

func IntersectIter6(a, b RangeSource6) func(yield func(Range6) bool) {
	return func(yield func(Range6) bool) {
		nextA, stopA := iter.Pull(a.Iter())
		defer stopA()
		nextB, stopB := iter.Pull(b.Iter())
		defer stopB()

		ra, okA := nextA()
		rb, okB := nextB()

		for okA && okB {
			if ra.Hi.LessThan(rb.Lo) {
				ra, okA = nextA()
				continue
			}
			if rb.Hi.LessThan(ra.Lo) {
				rb, okB = nextB()
				continue
			}

			lo := ra.Lo
			if rb.Lo.GreaterThan(lo) {
				lo = rb.Lo
			}
			hi := ra.Hi
			if rb.Hi.LessThan(hi) {
				hi = rb.Hi
			}

			if !yield(Range6{Lo: lo, Hi: hi}) {
				return
			}

			if ra.Hi.LessThan(rb.Hi) {
				ra, okA = nextA()
			} else if rb.Hi.LessThan(ra.Hi) {
				rb, okB = nextB()
			} else {
				ra, okA = nextA()
				rb, okB = nextB()
			}
		}
	}
}

func ExcludeIter6(a, b RangeSource6) func(yield func(Range6) bool) {
	return func(yield func(Range6) bool) {
		nextA, stopA := iter.Pull(a.Iter())
		defer stopA()
		nextB, stopB := iter.Pull(b.Iter())
		defer stopB()

		ra, okA := nextA()
		rb, okB := nextB()

		for okA {
			if !okB {
				if !yield(Range6{Lo: ra.Lo, Hi: ra.Hi}) {
					return
				}
				ra, okA = nextA()
				continue
			}

			if ra.Hi.LessThan(rb.Lo) {
				if !yield(Range6{Lo: ra.Lo, Hi: ra.Hi}) {
					return
				}
				ra, okA = nextA()
				continue
			}

			if rb.Hi.LessThan(ra.Lo) {
				rb, okB = nextB()
				continue
			}

			if ra.Lo.LessThan(rb.Lo) {
				if !yield(Range6{Lo: ra.Lo, Hi: rb.Lo.Sub64(1)}) {
					return
				}
			}

			if !ra.Hi.GreaterThan(rb.Hi) {
				ra, okA = nextA()
			} else {
				ra.Lo = rb.Hi.Incr()
				rb, okB = nextB()
			}
		}
	}
}

func DiffIter6(a, b RangeSource6) func(yield func(Range6) bool) {
	return func(yield func(Range6) bool) {
		nextA, stopA := iter.Pull(a.Iter())
		defer stopA()
		nextB, stopB := iter.Pull(b.Iter())
		defer stopB()

		ra, okA := nextA()
		rb, okB := nextB()

		var pending Range6
		havePending := false

		emit := func(r Range6) bool {
			if !havePending {
				pending = r
				havePending = true
				return true
			}
			if canMerge6(pending, r) {
				if r.Hi.GreaterThan(pending.Hi) {
					pending.Hi = r.Hi
				}
				return true
			}
			if !yield(pending) {
				return false
			}
			pending = r
			return true
		}

		for okA && okB {
			if ra.Hi.LessThan(rb.Lo) {
				if !emit(Range6{Lo: ra.Lo, Hi: ra.Hi}) {
					return
				}
				ra, okA = nextA()
				continue
			}

			if rb.Hi.LessThan(ra.Lo) {
				if !emit(Range6{Lo: rb.Lo, Hi: rb.Hi}) {
					return
				}
				rb, okB = nextB()
				continue
			}

			if ra.Lo.LessThan(rb.Lo) {
				if !emit(Range6{Lo: ra.Lo, Hi: rb.Lo.Sub64(1)}) {
					return
				}
			} else if rb.Lo.LessThan(ra.Lo) {
				if !emit(Range6{Lo: rb.Lo, Hi: ra.Lo.Sub64(1)}) {
					return
				}
			}

			switch {
			case ra.Hi.LessThan(rb.Hi):
				rb.Lo = ra.Hi.Incr()
				ra, okA = nextA()
			case rb.Hi.LessThan(ra.Hi):
				ra.Lo = rb.Hi.Incr()
				rb, okB = nextB()
			default:
				ra, okA = nextA()
				rb, okB = nextB()
			}
		}

		for okA {
			if !emit(Range6{Lo: ra.Lo, Hi: ra.Hi}) {
				return
			}
			ra, okA = nextA()
		}
		for okB {
			if !emit(Range6{Lo: rb.Lo, Hi: rb.Hi}) {
				return
			}
			rb, okB = nextB()
		}

		if havePending {
			yield(pending)
		}
	}
}

func canMerge6(cur, next Range6) bool {
	if cur.Hi.IsMax() {
		return !next.Lo.GreaterThan(cur.Hi)
	}
	return !next.Lo.GreaterThan(cur.Hi.Incr())
}

func UnionIter6(sources ...RangeSource6) func(yield func(Range6) bool) {
	var union func(yield func(Range6) bool)
	switch len(sources) {
	case 0:
		union = func(yield func(Range6) bool) {}
	case 1:
		union = sources[0].Iter()
	case 2:
		union = unionTwo6(sources[0], sources[1])
	default:
		union = unionKWay6(sources)
	}
	return func(yield func(Range6) bool) {
		union(yield)
	}
}

func unionTwo6(a, b RangeSource6) func(yield func(Range6) bool) {
	return func(yield func(Range6) bool) {
		nextA, stopA := iter.Pull(a.Iter())
		defer stopA()
		nextB, stopB := iter.Pull(b.Iter())
		defer stopB()

		ra, okA := nextA()
		rb, okB := nextB()

		pick := func() (Range6, bool) {
			if okA && okB {
				if !ra.Lo.GreaterThan(rb.Lo) {
					r := ra
					ra, okA = nextA()
					return r, true
				}
				r := rb
				rb, okB = nextB()
				return r, true
			}
			if okA {
				r := ra
				ra, okA = nextA()
				return r, true
			}
			if okB {
				r := rb
				rb, okB = nextB()
				return r, true
			}
			return Range6{}, false
		}

		cur, ok := pick()
		if !ok {
			return
		}

		for {
			next, ok := pick()
			if !ok {
				break
			}
			if canMerge6(cur, next) {
				if next.Hi.GreaterThan(cur.Hi) {
					cur.Hi = next.Hi
				}
				continue
			}
			if !yield(cur) {
				return
			}
			cur = next
		}

		yield(cur)
	}
}

type mergeHeap6 []mergeEntry6

type mergeEntry6 struct {
	r    Range6
	next func() (Range6, bool)
	stop func()
}

func (h mergeHeap6) Len() int { return len(h) }

func (h mergeHeap6) Less(i, j int) bool {
	cmp := h[i].r.Lo.Cmp(h[j].r.Lo)
	if cmp != 0 {
		return cmp < 0
	}
	return h[i].r.Hi.LessThan(h[j].r.Hi)
}

func (h mergeHeap6) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *mergeHeap6) Push(x any) { *h = append(*h, x.(mergeEntry6)) }

func (h *mergeHeap6) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func unionKWay6(sources []RangeSource6) func(yield func(Range6) bool) {
	return func(yield func(Range6) bool) {
		h := make(mergeHeap6, 0, len(sources))
		var stops []func()

		for _, src := range sources {
			next, stop := iter.Pull(src.Iter())
			stops = append(stops, stop)
			if r, ok := next(); ok {
				h = append(h, mergeEntry6{r: r, next: next, stop: stop})
			}
		}
		defer func() {
			for _, stop := range stops {
				stop()
			}
		}()

		heap.Init(&h)
		if h.Len() == 0 {
			return
		}

		first := heap.Pop(&h).(mergeEntry6)
		cur := first.r
		if r, ok := first.next(); ok {
			heap.Push(&h, mergeEntry6{r: r, next: first.next, stop: first.stop})
		}

		for h.Len() > 0 {
			top := heap.Pop(&h).(mergeEntry6)
			next := top.r
			if canMerge6(cur, next) {
				if next.Hi.GreaterThan(cur.Hi) {
					cur.Hi = next.Hi
				}
			} else {
				if !yield(cur) {
					return
				}
				cur = next
			}
			if r, ok := top.next(); ok {
				heap.Push(&h, mergeEntry6{r: r, next: top.next, stop: top.stop})
			}
		}

		yield(cur)
	}
}
