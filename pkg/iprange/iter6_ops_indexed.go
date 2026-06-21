package iprange

import "container/heap"

func intersect6IndexedIter(left, right indexedRangeSource6, yield func(Range6) bool) {
	i, j := 0, 0
	for i < left.len() && j < right.len() {
		ra, err := left.at(i)
		if err != nil {
			return
		}
		rb, err := right.at(j)
		if err != nil {
			return
		}
		if ra.Hi.LessThan(rb.Lo) {
			i++
			continue
		}
		if rb.Hi.LessThan(ra.Lo) {
			j++
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

		switch {
		case ra.Hi.LessThan(rb.Hi):
			i++
		case rb.Hi.LessThan(ra.Hi):
			j++
		default:
			i++
			j++
		}
	}
}

func exclude6IndexedIter(left, right indexedRangeSource6, yield func(Range6) bool) {
	i, j := 0, 0
	for i < left.len() {
		ra, err := left.at(i)
		if err != nil {
			return
		}
		for j < right.len() {
			rb, err := right.at(j)
			if err != nil {
				return
			}
			if !rb.Hi.LessThan(ra.Lo) {
				break
			}
			j++
		}

		consumed := false
		for j < right.len() {
			rb, err := right.at(j)
			if err != nil {
				return
			}
			if ra.Hi.LessThan(rb.Lo) {
				break
			}
			if ra.Lo.LessThan(rb.Lo) {
				if !yield(Range6{Lo: ra.Lo, Hi: rb.Lo.Sub64(1)}) {
					return
				}
			}
			if !ra.Hi.GreaterThan(rb.Hi) {
				consumed = true
				break
			}
			ra.Lo = rb.Hi.Incr()
			j++
		}
		if !consumed {
			if !yield(ra) {
				return
			}
		}
		i++
	}
}

func diff6IndexedIter(left, right indexedRangeSource6, yield func(Range6) bool) {
	i, j := 0, 0
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

	var ra, rb Range6
	okA, okB := false, false
	if i < left.len() {
		var err error
		ra, err = left.at(i)
		if err != nil {
			return
		}
		okA = true
	}
	if j < right.len() {
		var err error
		rb, err = right.at(j)
		if err != nil {
			return
		}
		okB = true
	}

	for okA && okB {
		if ra.Hi.LessThan(rb.Lo) {
			if !emit(ra) {
				return
			}
			i++
			okA = i < left.len()
			if okA {
				var err error
				ra, err = left.at(i)
				if err != nil {
					return
				}
			}
			continue
		}
		if rb.Hi.LessThan(ra.Lo) {
			if !emit(rb) {
				return
			}
			j++
			okB = j < right.len()
			if okB {
				var err error
				rb, err = right.at(j)
				if err != nil {
					return
				}
			}
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
			i++
			okA = i < left.len()
			if okA {
				var err error
				ra, err = left.at(i)
				if err != nil {
					return
				}
			}
		case rb.Hi.LessThan(ra.Hi):
			ra.Lo = rb.Hi.Incr()
			j++
			okB = j < right.len()
			if okB {
				var err error
				rb, err = right.at(j)
				if err != nil {
					return
				}
			}
		default:
			i++
			j++
			okA = i < left.len()
			okB = j < right.len()
			if okA {
				var err error
				ra, err = left.at(i)
				if err != nil {
					return
				}
			}
			if okB {
				var err error
				rb, err = right.at(j)
				if err != nil {
					return
				}
			}
		}
	}
	for okA {
		if !emit(ra) {
			return
		}
		i++
		okA = i < left.len()
		if okA {
			var err error
			ra, err = left.at(i)
			if err != nil {
				return
			}
		}
	}
	for okB {
		if !emit(rb) {
			return
		}
		j++
		okB = j < right.len()
		if okB {
			var err error
			rb, err = right.at(j)
			if err != nil {
				return
			}
		}
	}
	if havePending {
		yield(pending)
	}
}

func unionTwo6IndexedIter(a, b indexedRangeSource6, yield func(Range6) bool) {
	i, j := 0, 0
	var cur Range6
	haveCur := false
	for i < a.len() || j < b.len() {
		next, err := nextUnion6IndexedRange(a, b, &i, &j)
		if err != nil {
			return
		}
		if !haveCur {
			cur = next
			haveCur = true
			continue
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
	if haveCur {
		yield(cur)
	}
}

func nextUnion6IndexedRange(a, b indexedRangeSource6, i, j *int) (Range6, error) {
	if *i >= a.len() {
		r, err := b.at(*j)
		*j = *j + 1
		return r, err
	}
	if *j >= b.len() {
		r, err := a.at(*i)
		*i = *i + 1
		return r, err
	}
	ra, err := a.at(*i)
	if err != nil {
		return Range6{}, err
	}
	rb, err := b.at(*j)
	if err != nil {
		return Range6{}, err
	}
	if !ra.Lo.GreaterThan(rb.Lo) {
		*i = *i + 1
		return ra, nil
	}
	*j = *j + 1
	return rb, nil
}

type indexedMergeHeap6 []indexedMergeEntry6

type indexedMergeEntry6 struct {
	r         Range6
	sourceID  int
	nextIndex int
}

func (h indexedMergeHeap6) Len() int { return len(h) }

func (h indexedMergeHeap6) Less(i, j int) bool {
	cmp := h[i].r.Lo.Cmp(h[j].r.Lo)
	if cmp != 0 {
		return cmp < 0
	}
	return h[i].r.Hi.LessThan(h[j].r.Hi)
}

func (h indexedMergeHeap6) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *indexedMergeHeap6) Push(x any) {
	*h = append(*h, x.(indexedMergeEntry6))
}

func (h *indexedMergeHeap6) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func unionKWay6IndexedIter(sources []indexedRangeSource6, yield func(Range6) bool) {
	h := make(indexedMergeHeap6, 0, len(sources))
	for sourceID, src := range sources {
		if src.len() == 0 {
			continue
		}
		r, err := src.at(0)
		if err != nil {
			return
		}
		h = append(h, indexedMergeEntry6{r: r, sourceID: sourceID, nextIndex: 1})
	}
	heap.Init(&h)
	var cur Range6
	haveCur := false
	for h.Len() > 0 {
		entry := heap.Pop(&h).(indexedMergeEntry6)
		next := entry.r
		if !haveCur {
			cur = next
			haveCur = true
		} else if canMerge6(cur, next) {
			if next.Hi.GreaterThan(cur.Hi) {
				cur.Hi = next.Hi
			}
		} else {
			if !yield(cur) {
				return
			}
			cur = next
		}

		src := sources[entry.sourceID]
		if entry.nextIndex < src.len() {
			r, err := src.at(entry.nextIndex)
			if err != nil {
				return
			}
			heap.Push(&h, indexedMergeEntry6{
				r:         r,
				sourceID:  entry.sourceID,
				nextIndex: entry.nextIndex + 1,
			})
		}
	}
	if haveCur {
		yield(cur)
	}
}
