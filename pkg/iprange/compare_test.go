package iprange

import (
	"context"
	"errors"
	"testing"
)

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

func TestCompareNextSourcesMatchesCompareNext(t *testing.T) {
	a := newOptimizedSet("a", Range{Lo: 1, Hi: 10})
	b := newOptimizedSet("b", Range{Lo: 5, Hi: 15})
	c := newOptimizedSet("c", Range{Lo: 20, Hi: 30})

	want, err := CompareNext([]*IPSet{a}, []*IPSet{b, c})
	if err != nil {
		t.Fatalf("CompareNext() error = %v", err)
	}
	got, err := CompareNextSources(t.Context(),
		[]CompareSource{{Source: a}},
		[]CompareSource{{Source: b}, {Source: c}},
	)
	if err != nil {
		t.Fatalf("CompareNextSources() error = %v", err)
	}
	expectCompareRows(t, got, want)
}

func TestCompareNextSourcesWithFileSets(t *testing.T) {
	latest := newOptimizedSet("latest", Range{Lo: 1, Hi: 10})
	cohortA := newOptimizedSet("cohort-a", Range{Lo: 5, Hi: 15})
	cohortB := newOptimizedSet("cohort-b", Range{Lo: 20, Hi: 30})

	latestFS, err := OpenFileSet(writeTempSet(t, latest))
	if err != nil {
		t.Fatalf("OpenFileSet(latest) error = %v", err)
	}
	t.Cleanup(func() { _ = latestFS.Close() })
	cohortAFS, err := OpenFileSet(writeTempSet(t, cohortA))
	if err != nil {
		t.Fatalf("OpenFileSet(cohort-a) error = %v", err)
	}
	t.Cleanup(func() { _ = cohortAFS.Close() })
	cohortBFS, err := OpenFileSet(writeTempSet(t, cohortB))
	if err != nil {
		t.Fatalf("OpenFileSet(cohort-b) error = %v", err)
	}
	t.Cleanup(func() { _ = cohortBFS.Close() })

	got, err := CompareNextSources(t.Context(),
		[]CompareSource{{Name: "latest", Source: latestFS}},
		[]CompareSource{
			{Name: "cohort-a", Source: cohortAFS},
			{Name: "cohort-b", Source: cohortBFS},
		},
	)
	if err != nil {
		t.Fatalf("CompareNextSources() error = %v", err)
	}
	want := []CompareRow{
		{Name1: "latest", Name2: "cohort-a", Entries1: 1, Entries2: 1, Unique1: 10, Unique2: 11, CombinedIPs: 15, CommonIPs: 6},
		{Name1: "latest", Name2: "cohort-b", Entries1: 1, Entries2: 1, Unique1: 10, Unique2: 11, CombinedIPs: 21, CommonIPs: 0},
	}
	expectCompareRows(t, got, want)
}

func TestCompareNextSourcesHonorsContextCancellation(t *testing.T) {
	a := newOptimizedSet("a", Range{Lo: 1, Hi: 10})
	b := newOptimizedSet("b", Range{Lo: 5, Hi: 15})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := CompareNextSources(ctx,
		[]CompareSource{{Source: a}},
		[]CompareSource{{Source: b}},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CompareNextSources() error = %v, want context.Canceled", err)
	}
}

func expectCompareRows(t *testing.T, got, want []CompareRow) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("row count = %d, want %d: got %#v", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("row %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}
