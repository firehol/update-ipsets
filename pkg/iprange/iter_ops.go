package iprange

import (
	"container/heap"
	"iter"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// RangeSource provides sequential access to sorted, non-overlapping ranges.
// Both *IPSet and FileSet satisfy this interface.
type RangeSource interface {
	Len() int
	Iter() func(yield func(Range) bool)
}

// CountUniqueIter counts the total unique IPs from a RangeSource without
// materializing the ranges into a slice.
func CountUniqueIter(src RangeSource) uint64 {
	started := time.Now()
	defer func() {
		iprangeObserve(iprangeBackground(), "iprange.count_unique.ops", 1, 0, time.Since(started), attribute.String("ip.version", "4"))
	}()
	var total uint64
	for r := range src.Iter() {
		total += r.Size()
	}
	return total
}

// OverlapCountIter counts the number of IPs in the intersection of a and b
// without materializing the intersection. Uses a two-pointer sweep over both
// iterators. O(n+m) time, O(1) memory beyond the iterators.
func OverlapCountIter(a, b RangeSource) uint64 {
	started := time.Now()
	defer func() {
		attrs := []attribute.KeyValue{attribute.String("ip.version", "4")}
		iprangeObserve(iprangeBackground(), "iprange.compare.ops", 1, 0, time.Since(started), attrs...)
		iprangeObserve(iprangeBackground(), "iprange.overlap.ops", 1, 0, time.Since(started), attrs...)
	}()
	var count uint64
	for r := range IntersectIter(a, b) {
		count += r.Size()
	}
	return count
}

// IntersectIter yields ranges representing the intersection of a and b.
// Both inputs must be sorted, non-overlapping range sequences.
// Output ranges are sorted and non-overlapping.
func IntersectIter(a, b RangeSource) func(yield func(Range) bool) {
	return func(yield func(Range) bool) {
		started := time.Now()
		defer func() {
			iprangeObserve(iprangeBackground(), "iprange.intersect.ops", 1, 0, time.Since(started), attribute.String("ip.version", "4"))
		}()
		nextA, stopA := iter.Pull(a.Iter())
		defer stopA()
		nextB, stopB := iter.Pull(b.Iter())
		defer stopB()

		ra, okA := nextA()
		rb, okB := nextB()

		for okA && okB {
			// No overlap — advance the one that ends first.
			if ra.Hi < rb.Lo {
				ra, okA = nextA()
				continue
			}
			if rb.Hi < ra.Lo {
				rb, okB = nextB()
				continue
			}

			// Overlap exists — compute intersection.
			lo := ra.Lo
			if rb.Lo > lo {
				lo = rb.Lo
			}
			hi := ra.Hi
			if rb.Hi < hi {
				hi = rb.Hi
			}

			if !yield(Range{Lo: lo, Hi: hi}) {
				return
			}

			// Advance the range that ends first (or both if equal).
			if ra.Hi < rb.Hi {
				ra, okA = nextA()
			} else if rb.Hi < ra.Hi {
				rb, okB = nextB()
			} else {
				ra, okA = nextA()
				rb, okB = nextB()
			}
		}
	}
}

// ExcludeIter yields ranges that are in a but not in b (set difference a \ b).
// Both inputs must be sorted, non-overlapping range sequences.
// Output ranges are sorted and non-overlapping.
func ExcludeIter(a, b RangeSource) func(yield func(Range) bool) {
	return func(yield func(Range) bool) {
		started := time.Now()
		defer func() {
			iprangeObserve(iprangeBackground(), "iprange.exclude.ops", 1, 0, time.Since(started), attribute.String("ip.version", "4"))
		}()
		nextA, stopA := iter.Pull(a.Iter())
		defer stopA()
		nextB, stopB := iter.Pull(b.Iter())
		defer stopB()

		ra, okA := nextA()
		rb, okB := nextB()

		for okA {
			// No more exclusions — emit the rest of a.
			if !okB {
				if !yield(Range{Lo: ra.Lo, Hi: ra.Hi}) {
					return
				}
				ra, okA = nextA()
				continue
			}

			// Current a range is entirely before current b range.
			if ra.Hi < rb.Lo {
				if !yield(Range{Lo: ra.Lo, Hi: ra.Hi}) {
					return
				}
				ra, okA = nextA()
				continue
			}

			// Current b range is entirely before current a range.
			if rb.Hi < ra.Lo {
				rb, okB = nextB()
				continue
			}

			// Overlap: emit the part of a before b starts.
			if ra.Lo < rb.Lo {
				if !yield(Range{Lo: ra.Lo, Hi: rb.Lo - 1}) {
					return
				}
			}

			// Trim a past the end of b.
			if ra.Hi <= rb.Hi {
				// a is fully consumed by this b range.
				ra, okA = nextA()
			} else {
				// a extends past b — adjust a's start and advance b.
				ra.Lo = rb.Hi + 1
				rb, okB = nextB()
			}
		}
	}
}

// DiffIter yields the symmetric difference of a and b (ranges in a or b but
// not both). Both inputs must be sorted, non-overlapping range sequences.
// Output ranges are sorted, non-overlapping, and coalesced (adjacent ranges
// from different sides of the diff are merged).
func DiffIter(a, b RangeSource) func(yield func(Range) bool) {
	return func(yield func(Range) bool) {
		started := time.Now()
		defer func() {
			iprangeObserve(iprangeBackground(), "iprange.diff.ops", 1, 0, time.Since(started), attribute.String("ip.version", "4"))
		}()
		nextA, stopA := iter.Pull(a.Iter())
		defer stopA()
		nextB, stopB := iter.Pull(b.Iter())
		defer stopB()

		ra, okA := nextA()
		rb, okB := nextB()

		// Coalescing emitter: buffers the last range to merge with adjacent ones.
		var pending Range
		havePending := false

		emit := func(r Range) bool {
			if !havePending {
				pending = r
				havePending = true
				return true
			}
			if canMerge(pending, r) {
				if r.Hi > pending.Hi {
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
			// a entirely before b.
			if ra.Hi < rb.Lo {
				if !emit(Range{Lo: ra.Lo, Hi: ra.Hi}) {
					return
				}
				ra, okA = nextA()
				continue
			}

			// b entirely before a.
			if rb.Hi < ra.Lo {
				if !emit(Range{Lo: rb.Lo, Hi: rb.Hi}) {
					return
				}
				rb, okB = nextB()
				continue
			}

			// Overlap exists. Emit parts that don't overlap.
			if ra.Lo < rb.Lo {
				if !emit(Range{Lo: ra.Lo, Hi: rb.Lo - 1}) {
					return
				}
			} else if rb.Lo < ra.Lo {
				if !emit(Range{Lo: rb.Lo, Hi: ra.Lo - 1}) {
					return
				}
			}

			// Advance past the overlapping region.
			switch {
			case ra.Hi < rb.Hi:
				rb.Lo = ra.Hi + 1
				ra, okA = nextA()
			case rb.Hi < ra.Hi:
				ra.Lo = rb.Hi + 1
				rb, okB = nextB()
			default:
				ra, okA = nextA()
				rb, okB = nextB()
			}
		}

		// Drain remaining.
		for okA {
			if !emit(Range{Lo: ra.Lo, Hi: ra.Hi}) {
				return
			}
			ra, okA = nextA()
		}
		for okB {
			if !emit(Range{Lo: rb.Lo, Hi: rb.Hi}) {
				return
			}
			rb, okB = nextB()
		}

		// Flush the pending range.
		if havePending {
			yield(pending)
		}
	}
}

// UnionIter yields the union of all sources as a merged, non-overlapping range
// stream. For two sources it uses a direct two-pointer merge. For three or more
// it uses a min-heap based k-way merge.
func UnionIter(sources ...RangeSource) func(yield func(Range) bool) {
	wrap := func(inner func(yield func(Range) bool)) func(yield func(Range) bool) {
		return func(yield func(Range) bool) {
			started := time.Now()
			defer func() {
				attrs := []attribute.KeyValue{
					attribute.String("ip.version", "4"),
				}
				iprangeObserve(iprangeBackground(), "iprange.union.ops", 1, 0, time.Since(started), attrs...)
				iprangeObserve(iprangeBackground(), "iprange.merge.ops", 1, 0, time.Since(started), attrs...)
			}()
			inner(yield)
		}
	}
	switch len(sources) {
	case 0:
		return wrap(func(yield func(Range) bool) {})
	case 1:
		return wrap(sources[0].Iter())
	case 2:
		return wrap(unionTwo(sources[0], sources[1]))
	default:
		return wrap(unionKWay(sources))
	}
}

// unionTwo merges two sorted range sources into a single non-overlapping stream.
func unionTwo(a, b RangeSource) func(yield func(Range) bool) {
	return func(yield func(Range) bool) {
		nextA, stopA := iter.Pull(a.Iter())
		defer stopA()
		nextB, stopB := iter.Pull(b.Iter())
		defer stopB()

		ra, okA := nextA()
		rb, okB := nextB()

		// pick returns the smaller of the two available ranges and advances
		// the corresponding iterator.
		pick := func() (Range, bool) {
			if okA && okB {
				if ra.Lo <= rb.Lo {
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
			return Range{}, false
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
			// Try to merge next into cur.
			if canMerge(cur, next) {
				if next.Hi > cur.Hi {
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

// canMerge reports whether next is adjacent to or overlaps with cur.
func canMerge(cur, next Range) bool {
	if cur.Hi == ^uint32(0) {
		return next.Lo <= cur.Hi
	}
	return next.Lo <= cur.Hi+1
}

// mergeHeap is a min-heap of (Range, pull-function) entries, ordered by Lo.
type mergeHeap []mergeEntry

type mergeEntry struct {
	r    Range
	next func() (Range, bool)
	stop func()
}

func (h mergeHeap) Len() int { return len(h) }
func (h mergeHeap) Less(i, j int) bool {
	if h[i].r.Lo != h[j].r.Lo {
		return h[i].r.Lo < h[j].r.Lo
	}
	return h[i].r.Hi < h[j].r.Hi
}
func (h mergeHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *mergeHeap) Push(x any)   { *h = append(*h, x.(mergeEntry)) }
func (h *mergeHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// unionKWay merges k sorted range sources using a min-heap.
func unionKWay(sources []RangeSource) func(yield func(Range) bool) {
	return func(yield func(Range) bool) {
		h := make(mergeHeap, 0, len(sources))
		var stops []func()

		// Seed the heap with the first range from each source.
		for _, src := range sources {
			next, stop := iter.Pull(src.Iter())
			stops = append(stops, stop)
			if r, ok := next(); ok {
				h = append(h, mergeEntry{r: r, next: next, stop: stop})
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

		// Pop the first entry to start the current merged range.
		first := heap.Pop(&h).(mergeEntry)
		cur := first.r
		if r, ok := first.next(); ok {
			heap.Push(&h, mergeEntry{r: r, next: first.next, stop: first.stop})
		}

		for h.Len() > 0 {
			top := heap.Pop(&h).(mergeEntry)
			next := top.r

			// Try to merge.
			if canMerge(cur, next) {
				if next.Hi > cur.Hi {
					cur.Hi = next.Hi
				}
			} else {
				if !yield(cur) {
					return
				}
				cur = next
			}

			// Advance this source.
			if r, ok := top.next(); ok {
				heap.Push(&h, mergeEntry{r: r, next: top.next, stop: top.stop})
			}
		}

		yield(cur)
	}
}
