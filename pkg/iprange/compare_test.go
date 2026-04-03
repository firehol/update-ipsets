package iprange

import "testing"

func TestCompareModes(t *testing.T) {
	a := newOptimizedSet("a", Range{Lo: 1, Hi: 10})
	b := newOptimizedSet("b", Range{Lo: 5, Hi: 15})
	c := newOptimizedSet("c", Range{Lo: 20, Hi: 30})

	all, err := CompareAll([]*IPSet{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(all))
	}
	if all[0].CommonIPs != 6 {
		t.Fatalf("unexpected common IP count: %d", all[0].CommonIPs)
	}

	first, err := CompareFirst([]*IPSet{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || first[0].CommonIPs != 6 || first[1].CommonIPs != 0 {
		t.Fatalf("unexpected compare-first rows: %#v", first)
	}

	next, err := CompareNext([]*IPSet{a}, []*IPSet{b, c})
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 2 {
		t.Fatalf("unexpected compare-next row count: %d", len(next))
	}

	merged, err := CountUniqueMerged([]*IPSet{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if merged.Entries != 1 || merged.UniqueIPs != 15 {
		t.Fatalf("unexpected merged counts: %#v", merged)
	}

	rows, err := CountUniqueAll([]*IPSet{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Entries != 1 || rows[0].UniqueIPs != 10 {
		t.Fatalf("unexpected count rows: %#v", rows)
	}
}
