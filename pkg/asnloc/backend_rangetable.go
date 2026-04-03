package asnloc

import (
	"fmt"
	"sort"
)

// asnRange is one entry in a range-table backend: a contiguous IPv4
// range attributed to a single Autonomous System with a (possibly empty)
// organization name.
type asnRange struct {
	lo   uint32
	hi   uint32
	asn  uint32
	name string
}

// rangeTableBackend serves lookups out of a sorted in-memory slice of
// asnRange entries. Used by the iptoasn TSV and CAIDA prefix2as text
// formats — both of which deliver (range, ASN) tuples that we read once
// at startup and binary-search at lookup time.
//
// The slice MUST be sorted by lo ascending and have no overlapping ranges
// (newRangeTableBackend takes care of this). Touching ranges with the
// same ASN are merged so a Lookup that lands inside a contiguous
// attribution returns the full extent of that attribution in one step —
// this is what makes the range walker fast.
type rangeTableBackend struct {
	ranges []asnRange
	// totalIPs is precomputed at load time so stats() does not have to
	// iterate the slice on every call.
	totalIPs uint64
}

func newRangeTableBackend(ranges []asnRange) *rangeTableBackend {
	merged := sortMergeRanges(ranges)
	var total uint64
	for _, r := range merged {
		total += uint64(r.hi-r.lo) + 1
	}
	return &rangeTableBackend{ranges: merged, totalIPs: total}
}

// lookup is the contract documented on the backend interface. It returns
// either:
//   - a hit: ASN/name from the matching range and the range bounds
//   - a miss: ASN=0, with bounds covering the entire gap from `cur` to
//     just before the next range (so the caller can skip the gap in one
//     step)
//
// The miss case is what makes the range walker O(distinct attributions
// in feed) rather than O(distinct IPs in feed).
func (b *rangeTableBackend) lookup(ipv4 uint32) (Record, Network, error) {
	if b == nil || len(b.ranges) == 0 {
		return Record{}, Network{Lo: ipv4, Hi: ^uint32(0)}, nil
	}
	// Binary search for the first range with lo > ipv4. The hit candidate
	// is the one immediately before it.
	idx := sort.Search(len(b.ranges), func(i int) bool {
		return b.ranges[i].lo > ipv4
	})
	if idx > 0 {
		candidate := b.ranges[idx-1]
		if ipv4 >= candidate.lo && ipv4 <= candidate.hi {
			return Record{ASN: candidate.asn, Name: candidate.name},
				Network{Lo: candidate.lo, Hi: candidate.hi}, nil
		}
	}
	// Miss: the IP falls in a gap between two attributed ranges (or
	// before the first / after the last). Compute the gap bounds so
	// the caller can skip the entire gap in one step.
	//
	// gapLo is the IP immediately after the previous attributed range
	// (or 0 if there is no previous range). gapHi is the IP immediately
	// before the next attributed range (or 2^32-1 if there is none).
	var gapLo uint32 = 0
	if idx > 0 {
		prev := b.ranges[idx-1]
		gapLo = prev.hi + 1
	}
	gapHi := ^uint32(0)
	if idx < len(b.ranges) {
		next := b.ranges[idx]
		if next.lo > 0 {
			gapHi = next.lo - 1
		}
	}
	if gapHi < gapLo || ipv4 < gapLo || ipv4 > gapHi {
		// Defensive: should not happen with a sorted, non-overlapping
		// slice. Fall back to advancing one IP at a time.
		return Record{}, Network{Lo: ipv4, Hi: ipv4}, nil
	}
	return Record{}, Network{Lo: gapLo, Hi: gapHi}, nil
}

func (b *rangeTableBackend) stats() (networks int, ipv4Covered uint64, err error) {
	if b == nil {
		return 0, 0, fmt.Errorf("nil range table")
	}
	return len(b.ranges), b.totalIPs, nil
}

func (b *rangeTableBackend) close() error {
	// Nothing to release — the slice is owned by the backend and will
	// be garbage collected when the Database is dropped.
	return nil
}

// sortMergeRanges sorts the input slice in place by lo ascending, then
// merges adjacent ranges that share the same ASN. The merge step is
// important: iptoasn and CAIDA both deliver many small adjacent records
// (frequently /24s of the same prefix) and merging them keeps the range
// walker close to one lookup per distinct attribution rather than one
// per source row.
//
// Overlapping ranges with different ASNs are shifted so the later range
// starts after the earlier one ends — well-formed inputs from iptoasn
// and CAIDA do not produce such overlaps, but the guard keeps the slice
// valid even if a future provider does.
//
// Returns the merged slice (which may be shorter than the input).
func sortMergeRanges(ranges []asnRange) []asnRange {
	if len(ranges) == 0 {
		return ranges
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].lo != ranges[j].lo {
			return ranges[i].lo < ranges[j].lo
		}
		return ranges[i].hi < ranges[j].hi
	})
	if len(ranges) == 1 {
		return ranges
	}
	out := ranges[:1]
	for i := 1; i < len(ranges); i++ {
		cur := ranges[i]
		last := &out[len(out)-1]
		// Adjacent or overlapping with the same ASN — extend in place.
		if cur.lo <= last.hi+1 && cur.asn == last.asn {
			if cur.hi > last.hi {
				last.hi = cur.hi
			}
			if last.name == "" && cur.name != "" {
				last.name = cur.name
			}
			continue
		}
		// Overlap with a different ASN — shift the new range so it
		// starts after the existing one. Drop if that empties it.
		if cur.lo <= last.hi {
			if cur.hi <= last.hi {
				continue
			}
			cur.lo = last.hi + 1
		}
		out = append(out, cur)
	}
	return out
}
