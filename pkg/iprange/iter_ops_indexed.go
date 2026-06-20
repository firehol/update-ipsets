package iprange

import "container/heap"

func intersectIndexedIter(left, right indexedRangeSource, yield func(Range) bool) {
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
		if ra.Hi < rb.Lo {
			i++
			continue
		}
		if rb.Hi < ra.Lo {
			j++
			continue
		}
		if !yield(Range{Lo: max(ra.Lo, rb.Lo), Hi: min(ra.Hi, rb.Hi)}) {
			return
		}
		switch {
		case ra.Hi < rb.Hi:
			i++
		case rb.Hi < ra.Hi:
			j++
		default:
			i++
			j++
		}
	}
}

func diffIndexedIter(left, right indexedRangeSource, yield func(Range) bool) {
	i, j := 0, 0
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

	var ra, rb Range
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
		if ra.Hi < rb.Lo {
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
		if rb.Hi < ra.Lo {
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
		if ra.Lo < rb.Lo {
			if !emit(Range{Lo: ra.Lo, Hi: rb.Lo - 1}) {
				return
			}
		} else if rb.Lo < ra.Lo {
			if !emit(Range{Lo: rb.Lo, Hi: ra.Lo - 1}) {
				return
			}
		}
		switch {
		case ra.Hi < rb.Hi:
			rb.Lo = ra.Hi + 1
			i++
			okA = i < left.len()
			if okA {
				var err error
				ra, err = left.at(i)
				if err != nil {
					return
				}
			}
		case rb.Hi < ra.Hi:
			ra.Lo = rb.Hi + 1
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

func unionTwoIndexedIter(a, b indexedRangeSource, yield func(Range) bool) {
	i, j := 0, 0
	var cur Range
	haveCur := false
	for i < a.len() || j < b.len() {
		next, err := nextUnionIndexedRange(a, b, &i, &j)
		if err != nil {
			return
		}
		if !haveCur {
			cur = next
			haveCur = true
			continue
		}
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
	if haveCur {
		yield(cur)
	}
}

func unionKWayIndexedIter(sources []indexedRangeSource, yield func(Range) bool) {
	h := make(indexedMergeHeap, 0, len(sources))
	for sourceID, src := range sources {
		if src.len() == 0 {
			continue
		}
		r, err := src.at(0)
		if err != nil {
			return
		}
		h = append(h, indexedMergeEntry{r: r, sourceID: sourceID, nextIndex: 1})
	}
	heap.Init(&h)
	var cur Range
	haveCur := false
	for h.Len() > 0 {
		entry := heap.Pop(&h).(indexedMergeEntry)
		next := entry.r
		if !haveCur {
			cur = next
			haveCur = true
		} else if canMerge(cur, next) {
			if next.Hi > cur.Hi {
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
			heap.Push(&h, indexedMergeEntry{
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
