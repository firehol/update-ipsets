package iprange

import (
	"errors"
	"sort"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// IPSet holds an in-memory collection of IPv4 ranges. It is NOT safe for
// concurrent use; callers must synchronize access when sharing across
// goroutines.
type IPSet struct {
	Name      string
	Ranges    []Range
	Lines     int
	UniqueIPs uint64
	Optimized bool
}

func New(name string) *IPSet {
	if name == "" {
		name = DefaultName
	}
	return &IPSet{Name: name}
}

func (s *IPSet) Family() AddressFamily {
	return FamilyIPv4
}

func (s *IPSet) Clone() *IPSet {
	out := &IPSet{
		Name:      s.Name,
		Lines:     s.Lines,
		UniqueIPs: s.UniqueIPs,
		Optimized: s.Optimized,
	}
	out.Ranges = append(out.Ranges, s.Ranges...)
	return out
}

func (s *IPSet) AddRange(r Range) error {
	iprangeCount(iprangeBackground(), "iprange.add.ops", 1, attribute.String("ip.version", "4"))
	if !r.Valid() {
		return ErrInvalidRange
	}
	s.Ranges = append(s.Ranges, r)
	s.Lines++
	s.UniqueIPs += r.Size()
	s.Optimized = false
	return nil
}

func (s *IPSet) Add(lo, hi uint32) error {
	return s.AddRange(Range{Lo: lo, Hi: hi})
}

func (s *IPSet) Optimize() {
	if s.Optimized {
		return
	}
	started := time.Now()
	defer func() {
		iprangeObserve(iprangeBackground(), "iprange.optimize.ops", 1, int64(len(s.Ranges))*8, time.Since(started), attribute.String("ip.version", "4"))
	}()
	if len(s.Ranges) == 0 {
		s.UniqueIPs = 0
		s.Optimized = true
		return
	}

	originalLines := s.Lines
	sort.Slice(s.Ranges, func(i, j int) bool {
		if s.Ranges[i].Lo != s.Ranges[j].Lo {
			return s.Ranges[i].Lo < s.Ranges[j].Lo
		}
		return s.Ranges[i].Hi > s.Ranges[j].Hi
	})

	write := 0
	current := s.Ranges[0]
	for _, next := range s.Ranges[1:] {
		if next.Hi <= current.Hi {
			continue
		}
		if current.Hi != ^uint32(0) && next.Lo <= current.Hi+1 {
			current.Hi = next.Hi
			continue
		}
		if current.Hi == ^uint32(0) && next.Lo <= current.Hi {
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
	s.UniqueIPs = 0
	for _, r := range s.Ranges {
		s.UniqueIPs += r.Size()
	}
	s.Optimized = true
}

func (s *IPSet) EnsureOptimized() *IPSet {
	s.Optimize()
	return s
}

func (s *IPSet) Entries() int {
	s.Optimize()
	return len(s.Ranges)
}

func (s *IPSet) UniqueCount() uint64 {
	s.Optimize()
	return s.UniqueIPs
}

func (s *IPSet) Merge(other *IPSet) error {
	started := time.Now()
	defer func() {
		iprangeObserve(iprangeBackground(), "iprange.merge.ops", 1, 0, time.Since(started), attribute.String("ip.version", "4"))
	}()
	if s == nil || other == nil {
		return errors.New("nil ipset")
	}
	s.Ranges = append(s.Ranges, other.Ranges...)
	s.Lines += other.Lines
	s.Optimized = false
	s.UniqueIPs = 0
	return nil
}

// Len returns the number of ranges after optimization.
func (s *IPSet) Len() int {
	s.Optimize()
	return len(s.Ranges)
}

// Iter returns a Go 1.23-style push iterator over all ranges.
// The set is optimized before iteration begins.
func (s *IPSet) Iter() func(yield func(Range) bool) {
	s.Optimize()
	return func(yield func(Range) bool) {
		for _, r := range s.Ranges {
			if !yield(r) {
				return
			}
		}
	}
}

func (s *IPSet) Contains(ip uint32) bool {
	s.Optimize()
	iprangeCount(iprangeBackground(), "iprange.contains.ops", 1, attribute.String("ip.version", "4"))
	iprangeCount(iprangeBackground(), "iprange.binary.searches", 1, attribute.String("ip.version", "4"), attribute.String("iprange.source", "memory"))
	i := sort.Search(len(s.Ranges), func(i int) bool {
		return s.Ranges[i].Hi >= ip
	})
	if i >= len(s.Ranges) {
		return false
	}
	return s.Ranges[i].Lo <= ip
}
