package markdown

import "slices"

type CappedRow struct {
	Name  string
	Value uint64
	Other uint64
	Rows  []CappedEntry
}

type CappedEntry struct {
	Name  string
	Value uint64
}

func TopN(entries []CappedEntry, n int) CappedRow {
	if n < 1 {
		n = 1
	}
	sorted := make([]CappedEntry, len(entries))
	copy(sorted, entries)
	slices.SortFunc(sorted, func(a, b CappedEntry) int {
		if a.Value != b.Value {
			if a.Value > b.Value {
				return -1
			}
			return 1
		}
		return slices.Compare([]string{a.Name}, []string{b.Name})
	})

	if len(sorted) <= n {
		return CappedRow{Rows: sorted}
	}

	var other uint64
	for _, e := range sorted[n:] {
		other += e.Value
	}

	return CappedRow{
		Other: other,
		Rows:  sorted[:n],
	}
}

func TopNFromMap(m map[string]uint64, n int) CappedRow {
	entries := make([]CappedEntry, 0, len(m))
	for k, v := range m {
		entries = append(entries, CappedEntry{Name: k, Value: v})
	}
	return TopN(entries, n)
}
