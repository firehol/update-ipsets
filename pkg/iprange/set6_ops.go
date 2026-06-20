package iprange

import "fmt"

func optimizedView6(s *IPSet6) *IPSet6 {
	if s.Optimized {
		return s
	}
	return s.Clone().EnsureOptimized()
}

func Combine6(a, b *IPSet6) *IPSet6 {
	out := New6("combined")
	out.Lines = a.Lines + b.Lines
	out.Ranges = make([]Range6, 0, len(a.Ranges)+len(b.Ranges))
	out.Ranges = append(out.Ranges, a.Ranges...)
	out.Ranges = append(out.Ranges, b.Ranges...)
	return out
}

func Exclude6(a, b *IPSet6) *IPSet6 {
	left := optimizedView6(a)
	right := optimizedView6(b)
	out := New6(a.Name)
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
		if lo1.GreaterThan(hi2) {
			j++
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
			continue
		}
		if lo2.GreaterThan(hi1) {
			out.Ranges = append(out.Ranges, Range6{Lo: lo1, Hi: hi1})
			i++
			if i < len(left.Ranges) {
				lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
			}
			continue
		}

		if lo1.LessThan(lo2) {
			out.Ranges = append(out.Ranges, Range6{Lo: lo1, Hi: lo2.Sub64(1)})
			lo1 = lo2
		}

		switch {
		case hi1.Equals(hi2):
			i++
			j++
			if i < len(left.Ranges) {
				lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
			}
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
		case hi1.LessThan(hi2):
			i++
			if i < len(left.Ranges) {
				lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
			}
		default:
			if hi2.IsMax() {
				i++
				if i < len(left.Ranges) {
					lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
				}
			} else {
				lo1 = hi2.Incr()
			}
			j++
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
		}
	}

	if i < len(left.Ranges) {
		out.Ranges = append(out.Ranges, Range6{Lo: lo1, Hi: hi1})
		i++
		for ; i < len(left.Ranges); i++ {
			out.Ranges = append(out.Ranges, left.Ranges[i])
		}
	}

	out.Optimize()
	out.Lines = left.Lines + right.Lines
	return out
}

func Intersect6(a, b *IPSet6) *IPSet6 {
	left := optimizedView6(a)
	right := optimizedView6(b)
	out := New6("common")
	out.Lines = left.Lines + right.Lines

	if len(left.Ranges) == 0 || len(right.Ranges) == 0 {
		out.Optimized = true
		return out
	}

	i, j := 0, 0
	lo1, hi1 := left.Ranges[0].Lo, left.Ranges[0].Hi
	lo2, hi2 := right.Ranges[0].Lo, right.Ranges[0].Hi
	for i < len(left.Ranges) && j < len(right.Ranges) {
		if lo1.GreaterThan(hi2) {
			j++
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
			continue
		}
		if lo2.GreaterThan(hi1) {
			i++
			if i < len(left.Ranges) {
				lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
			}
			continue
		}

		lo := lo1
		if lo2.GreaterThan(lo) {
			lo = lo2
		}
		hi := hi2
		if hi1.LessThan(hi) {
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
		out.Ranges = append(out.Ranges, Range6{Lo: lo, Hi: hi})
	}

	out.Optimize()
	out.Lines = left.Lines + right.Lines
	return out
}

func Diff6(a, b *IPSet6) *IPSet6 {
	left := optimizedView6(a)
	right := optimizedView6(b)
	out := New6("diff")
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
		if lo1.GreaterThan(hi2) {
			out.Ranges = append(out.Ranges, Range6{Lo: lo2, Hi: hi2})
			j++
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
			continue
		}
		if lo2.GreaterThan(hi1) {
			out.Ranges = append(out.Ranges, Range6{Lo: lo1, Hi: hi1})
			i++
			if i < len(left.Ranges) {
				lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
			}
			continue
		}

		if lo1.GreaterThan(lo2) {
			out.Ranges = append(out.Ranges, Range6{Lo: lo2, Hi: lo1.Sub64(1)})
		} else if lo2.GreaterThan(lo1) {
			out.Ranges = append(out.Ranges, Range6{Lo: lo1, Hi: lo2.Sub64(1)})
		}

		switch {
		case hi1.GreaterThan(hi2):
			if hi2.IsMax() {
				i++
			} else {
				lo1 = hi2.Incr()
			}
			j++
			if j < len(right.Ranges) {
				lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
			}
		case hi2.GreaterThan(hi1):
			if hi1.IsMax() {
				j++
			} else {
				lo2 = hi1.Incr()
			}
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
		out.Ranges = append(out.Ranges, Range6{Lo: lo1, Hi: hi1})
		i++
		if i < len(left.Ranges) {
			lo1, hi1 = left.Ranges[i].Lo, left.Ranges[i].Hi
		}
	}
	for j < len(right.Ranges) {
		out.Ranges = append(out.Ranges, Range6{Lo: lo2, Hi: hi2})
		j++
		if j < len(right.Ranges) {
			lo2, hi2 = right.Ranges[j].Lo, right.Ranges[j].Hi
		}
	}

	out.Optimize()
	out.Lines = left.Lines + right.Lines
	return out
}

type CompareRow6 struct {
	Name1       string
	Name2       string
	Entries1    int
	Entries2    int
	Unique1     Uint128
	Unique2     Uint128
	CombinedIPs Uint128
	CommonIPs   Uint128
}

type CompareFirstRow6 struct {
	Name      string
	Entries   int
	UniqueIPs Uint128
	CommonIPs Uint128
}

type CountRow6 struct {
	Name      string
	Entries   int
	UniqueIPs Uint128
}

func CompareAll6(sets []*IPSet6) ([]CompareRow6, error) {
	if len(sets) < 2 {
		return nil, fmt.Errorf("compare requires at least two ipsets")
	}
	rows := make([]CompareRow6, 0, len(sets)*(len(sets)-1)/2)
	for i := 0; i < len(sets); i++ {
		sets[i].Optimize()
		for j := i + 1; j < len(sets); j++ {
			sets[j].Optimize()
			common := OverlapCountIter6(sets[i], sets[j])
			rows = append(rows, CompareRow6{
				Name1:       sets[i].Name,
				Name2:       sets[j].Name,
				Entries1:    len(sets[i].Ranges),
				Entries2:    len(sets[j].Ranges),
				Unique1:     sets[i].UniqueIPs,
				Unique2:     sets[j].UniqueIPs,
				CombinedIPs: sets[i].UniqueIPs.Add(sets[j].UniqueIPs).Sub(common),
				CommonIPs:   common,
			})
		}
	}
	return rows, nil
}

func CompareNext6(before, after []*IPSet6) ([]CompareRow6, error) {
	if len(before) == 0 || len(after) == 0 {
		return nil, fmt.Errorf("compare-next requires inputs on both sides")
	}
	rows := make([]CompareRow6, 0, len(before)*len(after))
	for _, left := range before {
		left.Optimize()
		for _, right := range after {
			right.Optimize()
			common := OverlapCountIter6(left, right)
			rows = append(rows, CompareRow6{
				Name1:       left.Name,
				Name2:       right.Name,
				Entries1:    len(left.Ranges),
				Entries2:    len(right.Ranges),
				Unique1:     left.UniqueIPs,
				Unique2:     right.UniqueIPs,
				CombinedIPs: left.UniqueIPs.Add(right.UniqueIPs).Sub(common),
				CommonIPs:   common,
			})
		}
	}
	return rows, nil
}

func CompareFirst6(sets []*IPSet6) ([]CompareFirstRow6, error) {
	if len(sets) < 2 {
		return nil, fmt.Errorf("compare-first requires at least two ipsets")
	}
	base := sets[0].Clone()
	base.Optimize()
	rows := make([]CompareFirstRow6, 0, len(sets)-1)
	for _, candidate := range sets[1:] {
		candidate.Optimize()
		common := OverlapCountIter6(base, candidate)
		rows = append(rows, CompareFirstRow6{
			Name:      candidate.Name,
			Entries:   len(candidate.Ranges),
			UniqueIPs: candidate.UniqueIPs,
			CommonIPs: common,
		})
	}
	return rows, nil
}

func CountUniqueMerged6(sets []*IPSet6) (CountRow6, error) {
	merged, err := mergeAll6(sets)
	if err != nil {
		return CountRow6{}, err
	}
	merged.Optimize()
	return CountRow6{
		Name:      merged.Name,
		Entries:   len(merged.Ranges),
		UniqueIPs: merged.UniqueIPs,
	}, nil
}

func CountUniqueAll6(sets []*IPSet6) ([]CountRow6, error) {
	if len(sets) == 0 {
		return nil, fmt.Errorf("no ipsets provided")
	}
	rows := make([]CountRow6, 0, len(sets))
	for _, set := range sets {
		set.Optimize()
		rows = append(rows, CountRow6{
			Name:      set.Name,
			Entries:   len(set.Ranges),
			UniqueIPs: set.UniqueIPs,
		})
	}
	return rows, nil
}
