package dronebl

import (
	"fmt"
	"io"
	"math/bits"
	"net/netip"
	"sort"
	"strconv"
)

type Range struct {
	Lo uint32
	Hi uint32
}

type RangeSet struct {
	ranges []Range
}

func NewRangeSet() *RangeSet {
	return &RangeSet{}
}

func (s *RangeSet) AddRange(r Range) error {
	if r.Hi < r.Lo {
		return fmt.Errorf("invalid range: hi before lo")
	}
	s.ranges = append(s.ranges, r)
	return nil
}

func (s *RangeSet) Merge(other *RangeSet) error {
	if other == nil || len(other.ranges) == 0 {
		return nil
	}
	s.ranges = append(s.ranges, other.ranges...)
	return nil
}

func (s *RangeSet) Optimize() {
	if s == nil || len(s.ranges) < 2 {
		return
	}
	sort.Slice(s.ranges, func(i, j int) bool {
		if s.ranges[i].Lo == s.ranges[j].Lo {
			return s.ranges[i].Hi < s.ranges[j].Hi
		}
		return s.ranges[i].Lo < s.ranges[j].Lo
	})

	out := s.ranges[:0]
	for _, current := range s.ranges {
		if len(out) == 0 {
			out = append(out, current)
			continue
		}
		last := &out[len(out)-1]
		if current.Lo <= last.Hi || (last.Hi != ^uint32(0) && current.Lo == last.Hi+1) {
			if current.Hi > last.Hi {
				last.Hi = current.Hi
			}
			continue
		}
		out = append(out, current)
	}
	s.ranges = out
}

func (s *RangeSet) Entries() int {
	clone := s.clone()
	clone.Optimize()
	return len(clone.ranges)
}

func (s *RangeSet) UniqueCount() uint64 {
	clone := s.clone()
	clone.Optimize()
	var total uint64
	for _, r := range clone.ranges {
		total += uint64(r.Hi) - uint64(r.Lo) + 1
	}
	return total
}

func (s *RangeSet) WriteCIDR(w io.Writer) error {
	clone := s.clone()
	clone.Optimize()
	for _, r := range clone.ranges {
		if err := writeRangeCIDR(w, r.Lo, r.Hi); err != nil {
			return err
		}
	}
	return nil
}

func (s *RangeSet) clone() *RangeSet {
	if s == nil || len(s.ranges) == 0 {
		return NewRangeSet()
	}
	return &RangeSet{ranges: append([]Range(nil), s.ranges...)}
}

func Exclude(include, exclude *RangeSet) *RangeSet {
	inc := include.clone()
	exc := exclude.clone()
	inc.Optimize()
	exc.Optimize()

	out := NewRangeSet()
	j := 0
	for _, in := range inc.ranges {
		start := uint64(in.Lo)
		end := uint64(in.Hi)
		for j < len(exc.ranges) && uint64(exc.ranges[j].Hi) < start {
			j++
		}
		for k := j; k < len(exc.ranges) && uint64(exc.ranges[k].Lo) <= end; k++ {
			exLo := uint64(exc.ranges[k].Lo)
			exHi := uint64(exc.ranges[k].Hi)
			if exHi < start {
				continue
			}
			if exLo > start {
				_ = out.AddRange(Range{Lo: uint32(start), Hi: uint32(exLo - 1)})
			}
			if exHi >= end {
				start = end + 1
				break
			}
			start = exHi + 1
		}
		if start <= end {
			_ = out.AddRange(Range{Lo: uint32(start), Hi: uint32(end)})
		}
	}
	out.Optimize()
	return out
}

func ParseIPv4Token(token string) (uint32, error) {
	addr, err := netip.ParseAddr(token)
	if err != nil {
		return 0, err
	}
	if !addr.Is4() {
		return 0, fmt.Errorf("not an IPv4 address")
	}
	parts := addr.As4()
	return uint32(parts[0])<<24 | uint32(parts[1])<<16 | uint32(parts[2])<<8 | uint32(parts[3]), nil
}

func ParsePrefix(value string) (int, error) {
	prefix, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if prefix < 0 || prefix > 32 {
		return 0, fmt.Errorf("invalid prefix length %d", prefix)
	}
	return prefix, nil
}

func Network(addr uint32, prefix int) uint32 {
	if prefix == 0 {
		return 0
	}
	return addr & (^uint32(0) << uint(32-prefix))
}

func Broadcast(network uint32, prefix int) uint32 {
	if prefix == 0 {
		return ^uint32(0)
	}
	return network | (^uint32(0) >> uint(prefix))
}

func writeRangeCIDR(w io.Writer, lo, hi uint32) error {
	for current := uint64(lo); current <= uint64(hi); {
		size := largestCIDRBlock(uint32(current), hi)
		prefix := 32 - bits.TrailingZeros64(size)
		if prefix == 32 {
			if _, err := fmt.Fprintf(w, "%s\n", ipv4String(uint32(current))); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, "%s/%d\n", ipv4String(uint32(current)), prefix); err != nil {
				return err
			}
		}
		current += size
	}
	return nil
}

func largestCIDRBlock(lo, hi uint32) uint64 {
	remaining := uint64(hi) - uint64(lo) + 1
	var alignment uint64
	if lo == 0 {
		alignment = uint64(1) << 32
	} else {
		alignment = uint64(1) << bits.TrailingZeros32(lo)
	}
	for alignment > remaining {
		alignment >>= 1
	}
	return alignment
}

func ipv4String(value uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
}
