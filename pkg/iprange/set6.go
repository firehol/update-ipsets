package iprange

import (
	"errors"
	"sort"
)

var errNilIPSet = errors.New("nil ipset")

// IPSet6 holds an in-memory collection of IPv6 ranges.
type IPSet6 struct {
	Name      string
	Ranges    []Range6
	Lines     int
	UniqueIPs Uint128
	Optimized bool
}

func New6(name string) *IPSet6 {
	if name == "" {
		name = DefaultName
	}
	return &IPSet6{Name: name}
}

func (s *IPSet6) Family() AddressFamily {
	return FamilyIPv6
}

func (s *IPSet6) Clone() *IPSet6 {
	out := &IPSet6{
		Name:      s.Name,
		Lines:     s.Lines,
		UniqueIPs: s.UniqueIPs,
		Optimized: s.Optimized,
	}
	out.Ranges = append(out.Ranges, s.Ranges...)
	return out
}

func (s *IPSet6) AddRange6(r Range6) error {
	if !r.Valid() {
		return ErrInvalidRange
	}
	s.Ranges = append(s.Ranges, r)
	s.Lines++
	s.UniqueIPs = s.UniqueIPs.Add(r.Size())
	s.Optimized = false
	return nil
}

func (s *IPSet6) Add6(lo, hi Uint128) error {
	return s.AddRange6(Range6{Lo: lo, Hi: hi})
}

func (s *IPSet6) Optimize() {
	if s.Optimized {
		return
	}
	if len(s.Ranges) == 0 {
		s.UniqueIPs = uint128Zero
		s.Optimized = true
		return
	}

	originalLines := s.Lines
	sort.Slice(s.Ranges, func(i, j int) bool {
		cmp := s.Ranges[i].Lo.Cmp(s.Ranges[j].Lo)
		if cmp != 0 {
			return cmp < 0
		}
		return s.Ranges[i].Hi.GreaterThan(s.Ranges[j].Hi)
	})

	write := 0
	current := s.Ranges[0]
	for _, next := range s.Ranges[1:] {
		if !next.Hi.GreaterThan(current.Hi) {
			continue
		}
		if !current.Hi.IsMax() && next.Lo.LessOrEqual(current.Hi.Incr()) {
			current.Hi = next.Hi
			continue
		}
		if current.Hi.IsMax() && next.Lo.LessOrEqual(current.Hi) {
			current.Hi = next.Hi
			continue
		}
		s.Ranges[write] = current
		write++
		current = next
	}
	s.Ranges[write] = current
	write++
	s.Ranges = s.Ranges[:write]

	s.Lines = originalLines
	s.UniqueIPs = uint128Zero
	for _, r := range s.Ranges {
		s.UniqueIPs = s.UniqueIPs.Add(r.Size())
	}
	s.Optimized = true
}

func (s *IPSet6) EnsureOptimized() *IPSet6 {
	s.Optimize()
	return s
}

func (s *IPSet6) Entries() int {
	s.Optimize()
	return len(s.Ranges)
}

func (s *IPSet6) UniqueCount() Uint128 {
	s.Optimize()
	return s.UniqueIPs
}

func (s *IPSet6) Merge(other *IPSet6) error {
	if s == nil || other == nil {
		return errNilIPSet
	}
	s.Ranges = append(s.Ranges, other.Ranges...)
	s.Lines += other.Lines
	s.Optimized = false
	s.UniqueIPs = uint128Zero
	return nil
}

func (s *IPSet6) Len() int {
	s.Optimize()
	return len(s.Ranges)
}

func (s *IPSet6) Iter() func(yield func(Range6) bool) {
	s.Optimize()
	return func(yield func(Range6) bool) {
		for _, r := range s.Ranges {
			if !yield(r) {
				return
			}
		}
	}
}

func (s *IPSet6) Contains(ip Uint128) bool {
	ok, _ := s.containsWithStats(ip)
	return ok
}

func (s *IPSet6) containsWithStats(ip Uint128) (bool, OperationStats) {
	s.Optimize()
	stats := OperationStats{Lookups: 1, BinarySearches: 1}
	i := sort.Search(len(s.Ranges), func(i int) bool {
		stats.Comparisons++
		return !s.Ranges[i].Hi.LessThan(ip)
	})
	if i >= len(s.Ranges) {
		return false, stats
	}
	return !s.Ranges[i].Lo.GreaterThan(ip), stats
}
