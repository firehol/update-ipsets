package iprange

import (
	"context"
	"fmt"
)

// optimizedView returns s directly if already optimized, avoiding a
// clone+optimize allocation. The caller must not mutate the result.
func optimizedView(s *IPSet) *IPSet {
	if s.Optimized {
		return s
	}
	return s.Clone().EnsureOptimized()
}

// Combine returns the union of a and b without optimization. Call
// Optimize() on the result to merge overlapping ranges.
func Combine(a, b *IPSet) *IPSet {
	out := New("combined")
	out.Lines = a.Lines + b.Lines
	out.Ranges = make([]Range, 0, len(a.Ranges)+len(b.Ranges))
	out.Ranges = append(out.Ranges, a.Ranges...)
	out.Ranges = append(out.Ranges, b.Ranges...)
	return out
}

// Exclude returns the set difference a \ b (IPs in a but not in b).
func Exclude(a, b *IPSet) *IPSet {
	left := optimizedView(a)
	right := optimizedView(b)
	out := New(a.Name)
	out.Lines = left.Lines + right.Lines

	if len(left.Ranges) == 0 {
		out.Optimized = true
		return out
	}
	if len(right.Ranges) == 0 {
		out.Ranges = append(out.Ranges, left.Ranges...)
		out.Optimize()
		out.Lines = left.Lines + right.Lines
		return out
	}

	i, j := 0, 0
	lo1, hi1 := left.Ranges[0].Lo, left.Ranges[0].Hi
	lo2, hi2 := right.Ranges[0].Lo, right.Ranges[0].Hi

	for i < len(left.Ranges) && j < len(right.Ranges) {
		if lo1 > hi2 {
			j++
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
			continue
		}
		if lo2 > hi1 {
			out.Ranges = append(out.Ranges, Range{Lo: lo1, Hi: hi1})
			i++
			if i < len(left.Ranges) {
				lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
			}
			continue
		}

		if lo1 < lo2 {
			out.Ranges = append(out.Ranges, Range{Lo: lo1, Hi: lo2 - 1})
			lo1 = lo2
		}

		switch {
		case hi1 == hi2:
			i++
			j++
			if i < len(left.Ranges) {
				lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
			}
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
		case hi1 < hi2:
			i++
			if i < len(left.Ranges) {
				lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
			}
		default:
			lo1 = hi2 + 1
			j++
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
		}
	}

	if i < len(left.Ranges) {
		out.Ranges = append(out.Ranges, Range{Lo: lo1, Hi: hi1})
		i++
		for ; i < len(left.Ranges); i++ {
			out.Ranges = append(out.Ranges, left.Ranges[i])
		}
	}

	out.Optimize()
	out.Lines = left.Lines + right.Lines
	return out
}

// Intersect returns the IPs common to both a and b.
func Intersect(a, b *IPSet) *IPSet {
	left := optimizedView(a)
	right := optimizedView(b)
	out := New("common")
	out.Lines = left.Lines + right.Lines

	if len(left.Ranges) == 0 || len(right.Ranges) == 0 {
		out.Optimized = true
		return out
	}

	i, j := 0, 0
	lo1, hi1 := left.Ranges[0].Lo, left.Ranges[0].Hi
	lo2, hi2 := right.Ranges[0].Lo, right.Ranges[0].Hi
	for i < len(left.Ranges) && j < len(right.Ranges) {
		if lo1 > hi2 {
			j++
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
			continue
		}
		if lo2 > hi1 {
			i++
			if i < len(left.Ranges) {
				lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
			}
			continue
		}

		lo := lo1
		if lo2 > lo {
			lo = lo2
		}
		hi := hi2
		if hi1 < hi {
			hi = hi1
			i++
			if i < len(left.Ranges) {
				lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
			}
		} else {
			j++
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
		}
		out.Ranges = append(out.Ranges, Range{Lo: lo, Hi: hi})
	}

	out.Optimize()
	out.Lines = left.Lines + right.Lines
	return out
}

// Diff returns the symmetric difference of a and b (IPs in either but not both).
func Diff(a, b *IPSet) *IPSet {
	left := optimizedView(a)
	right := optimizedView(b)
	out := New("diff")
	out.Lines = left.Lines + right.Lines

	if len(left.Ranges) == 0 && len(right.Ranges) == 0 {
		out.Optimized = true
		return out
	}
	if len(left.Ranges) == 0 {
		out.Ranges = append(out.Ranges, right.Ranges...)
		out.Optimize()
		out.Lines = left.Lines + right.Lines
		return out
	}
	if len(right.Ranges) == 0 {
		out.Ranges = append(out.Ranges, left.Ranges...)
		out.Optimize()
		out.Lines = left.Lines + right.Lines
		return out
	}

	i, j := 0, 0
	lo1, hi1 := left.Ranges[0].Lo, left.Ranges[0].Hi
	lo2, hi2 := right.Ranges[0].Lo, right.Ranges[0].Hi

	for i < len(left.Ranges) && j < len(right.Ranges) {
		if lo1 > hi2 {
			out.Ranges = append(out.Ranges, Range{Lo: lo2, Hi: hi2})
			j++
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
			continue
		}
		if lo2 > hi1 {
			out.Ranges = append(out.Ranges, Range{Lo: lo1, Hi: hi1})
			i++
			if i < len(left.Ranges) {
				lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
			}
			continue
		}

		if lo1 > lo2 {
			out.Ranges = append(out.Ranges, Range{Lo: lo2, Hi: lo1 - 1})
		} else if lo2 > lo1 {
			out.Ranges = append(out.Ranges, Range{Lo: lo1, Hi: lo2 - 1})
		}

		switch {
		case hi1 > hi2:
			lo1 = hi2 + 1
			j++
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
		case hi2 > hi1:
			lo2 = hi1 + 1
			i++
			if i < len(left.Ranges) {
				lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
			}
		default:
			i++
			j++
			if i < len(left.Ranges) {
				lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
			}
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
		}
	}

	for i < len(left.Ranges) {
		out.Ranges = append(out.Ranges, Range{Lo: lo1, Hi: hi1})
		i++
		if i < len(left.Ranges) {
			lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
		}
	}
	for j < len(right.Ranges) {
		out.Ranges = append(out.Ranges, Range{Lo: lo2, Hi: hi2})
		j++
		if j < len(right.Ranges) {
			lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
		}
	}

	out.Optimize()
	out.Lines = left.Lines + right.Lines
	return out
}

type CompareRow struct {
	Name1       string
	Name2       string
	Entries1    int
	Entries2    int
	Unique1     uint64
	Unique2     uint64
	CombinedIPs uint64
	CommonIPs   uint64
}

// CompareSource names a range source for comparison output. Source may be an
// in-memory *IPSet or a file-backed FileSet.
type CompareSource struct {
	Name   string
	Source RangeSource
}

type CompareFirstRow struct {
	Name      string
	Entries   int
	UniqueIPs uint64
	CommonIPs uint64
}

type CountRow struct {
	Name      string
	Entries   int
	UniqueIPs uint64
}

func CompareAll(sets []*IPSet) ([]CompareRow, error) {
	if len(sets) < 2 {
		return nil, fmt.Errorf("compare requires at least two ipsets")
	}
	rows := make([]CompareRow, 0, len(sets)*(len(sets)-1)/2)
	for i := 0; i < len(sets); i++ {
		sets[i].Optimize()
		for j := i + 1; j < len(sets); j++ {
			sets[j].Optimize()
			common, err := OverlapCountIterContext(context.Background(), sets[i], sets[j])
			if err != nil {
				return nil, err
			}
			rows = append(rows, CompareRow{
				Name1:       sets[i].Name,
				Name2:       sets[j].Name,
				Entries1:    len(sets[i].Ranges),
				Entries2:    len(sets[j].Ranges),
				Unique1:     sets[i].UniqueIPs,
				Unique2:     sets[j].UniqueIPs,
				CombinedIPs: sets[i].UniqueIPs + sets[j].UniqueIPs - common,
				CommonIPs:   common,
			})
		}
	}
	return rows, nil
}

func CompareNext(before, after []*IPSet) ([]CompareRow, error) {
	if len(before) == 0 || len(after) == 0 {
		return nil, fmt.Errorf("compare-next requires inputs on both sides")
	}
	rows := make([]CompareRow, 0, len(before)*len(after))
	for _, left := range before {
		left.Optimize()
		for _, right := range after {
			right.Optimize()
			common, err := OverlapCountIterContext(context.Background(), left, right)
			if err != nil {
				return nil, err
			}
			rows = append(rows, CompareRow{
				Name1:       left.Name,
				Name2:       right.Name,
				Entries1:    len(left.Ranges),
				Entries2:    len(right.Ranges),
				Unique1:     left.UniqueIPs,
				Unique2:     right.UniqueIPs,
				CombinedIPs: left.UniqueIPs + right.UniqueIPs - common,
				CommonIPs:   common,
			})
		}
	}
	return rows, nil
}

// CompareNextSources compares every source in before with every source in after.
// It produces the same row semantics as CompareNext while accepting streaming
// RangeSource inputs, so file-backed sets do not need to be materialized.
func CompareNextSources(ctx context.Context, before, after []CompareSource) ([]CompareRow, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(before) == 0 || len(after) == 0 {
		return nil, fmt.Errorf("compare-next requires inputs on both sides")
	}

	left, err := prepareCompareSources(ctx, before)
	if err != nil {
		return nil, err
	}
	right, err := prepareCompareSources(ctx, after)
	if err != nil {
		return nil, err
	}

	rows := make([]CompareRow, 0, len(left)*len(right))
	for _, l := range left {
		for _, r := range right {
			common, err := OverlapCountIterContext(ctx, l.source, r.source)
			if err != nil {
				return nil, err
			}
			if err := RangeSourceErr(l.source); err != nil {
				return nil, fmt.Errorf("compare %s: %w", l.name, err)
			}
			if err := RangeSourceErr(r.source); err != nil {
				return nil, fmt.Errorf("compare %s: %w", r.name, err)
			}
			rows = append(rows, CompareRow{
				Name1:       l.name,
				Name2:       r.name,
				Entries1:    l.entries,
				Entries2:    r.entries,
				Unique1:     l.uniqueIPs,
				Unique2:     r.uniqueIPs,
				CombinedIPs: l.uniqueIPs + r.uniqueIPs - common,
				CommonIPs:   common,
			})
		}
	}
	return rows, nil
}

type compareSourceMeta struct {
	name      string
	source    RangeSource
	entries   int
	uniqueIPs uint64
}

func prepareCompareSources(ctx context.Context, in []CompareSource) ([]compareSourceMeta, error) {
	out := make([]compareSourceMeta, 0, len(in))
	for i, src := range in {
		if src.Source == nil {
			return nil, fmt.Errorf("compare source %d has nil range source", i)
		}
		name := src.Name
		if name == "" {
			name = compareSourceDefaultName(src.Source)
		}
		uniqueIPs, err := RangeSourceUniqueIPs(ctx, src.Source)
		if err != nil {
			return nil, fmt.Errorf("compare %s: %w", name, err)
		}
		if err := RangeSourceErr(src.Source); err != nil {
			return nil, fmt.Errorf("compare %s: %w", name, err)
		}
		out = append(out, compareSourceMeta{
			name:      name,
			source:    src.Source,
			entries:   src.Source.Len(),
			uniqueIPs: uniqueIPs,
		})
	}
	return out, nil
}

func compareSourceDefaultName(src RangeSource) string {
	if set, ok := src.(*IPSet); ok && set.Name != "" {
		return set.Name
	}
	return DefaultName
}

func CompareFirst(sets []*IPSet) ([]CompareFirstRow, error) {
	if len(sets) < 2 {
		return nil, fmt.Errorf("compare-first requires at least two ipsets")
	}
	first := sets[0]
	first.Optimize()
	rows := make([]CompareFirstRow, 0, len(sets)-1)
	for _, set := range sets[1:] {
		set.Optimize()
		common, err := OverlapCountIterContext(context.Background(), first, set)
		if err != nil {
			return nil, err
		}
		rows = append(rows, CompareFirstRow{
			Name:      set.Name,
			Entries:   len(set.Ranges),
			UniqueIPs: set.UniqueIPs,
			CommonIPs: common,
		})
	}
	return rows, nil
}

func CountUniqueMerged(sets []*IPSet) (CountRow, error) {
	if len(sets) == 0 {
		return CountRow{}, fmt.Errorf("no ipsets to count")
	}
	merged := sets[0].Clone()
	for _, set := range sets[1:] {
		if err := merged.Merge(set); err != nil {
			return CountRow{}, err
		}
	}
	merged.Optimize()
	return CountRow{
		Entries:   len(merged.Ranges),
		UniqueIPs: merged.UniqueIPs,
	}, nil
}

func CountUniqueAll(sets []*IPSet) ([]CountRow, error) {
	rows := make([]CountRow, 0, len(sets))
	for _, set := range sets {
		set.Optimize()
		rows = append(rows, CountRow{
			Name:      set.Name,
			Entries:   len(set.Ranges),
			UniqueIPs: set.UniqueIPs,
		})
	}
	return rows, nil
}
