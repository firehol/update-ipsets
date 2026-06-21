package engine

import (
	"bytes"
	"compress/gzip"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/geoloc"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestLatestSetCacheReusesOpenSets(t *testing.T) {
	eng, _ := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	cache := newLatestSetCache(eng)
	defer cache.CloseAll(eng.logger)

	first, err := cache.Open("alpha")
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Open("alpha")
	if err != nil {
		t.Fatal(err)
	}

	if first != second {
		t.Fatalf("expected cached latest set pointer reuse, got %p and %p", first, second)
	}
	if got := first.UniqueIPs(); got == 0 {
		t.Fatalf("expected cached latest set to stay usable, got %d unique IPs", got)
	}
}

func TestLatestSetCacheReusesSummaries(t *testing.T) {
	eng, _ := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	cache := newLatestSetCache(eng)
	defer cache.CloseAll(eng.logger)

	first, err := cache.Summary(t.Context(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	filter, err := cache.OverlapFilter(t.Context(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Summary(t.Context(), "alpha")
	if err != nil {
		t.Fatal(err)
	}

	if !first.ContentHash.Equal(second.ContentHash) {
		t.Fatal("expected cached summaries to keep the same content hash")
	}
	if !filter.Valid() || !filter.HasRange() {
		t.Fatalf("cached overlap filter = %+v, want valid non-empty filter", filter)
	}
	if len(cache.summaries) != 1 {
		t.Fatalf("cached summaries = %d, want 1", len(cache.summaries))
	}
}

func TestLatestSetCacheOverlapFilterDoesNotBuildSummary(t *testing.T) {
	eng, _ := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	cache := newLatestSetCache(eng)
	defer cache.CloseAll(eng.logger)

	filter, err := cache.OverlapFilter(t.Context(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if !filter.Valid() || !filter.HasRange() {
		t.Fatalf("cached overlap filter = %+v, want valid non-empty filter", filter)
	}
	if len(cache.summaries) != 0 {
		t.Fatalf("cached summaries after filter-only lookup = %d, want 0", len(cache.summaries))
	}
	if len(cache.filters) != 1 {
		t.Fatalf("cached filters = %d, want 1", len(cache.filters))
	}

	summary, err := cache.Summary(t.Context(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if !summary.ContentHash.Valid {
		t.Fatal("expected summary lookup to build a valid content hash")
	}
	filter, err = cache.OverlapFilter(t.Context(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if filter.Disjoint(summary.OverlapFilter()) {
		t.Fatal("filter-only and summary-derived filters for same source must not be disjoint")
	}
}

func TestLatestSetCacheDoesNotReuseTextFallbackSets(t *testing.T) {
	eng, root := newTestEngine(t, "1.2.3.4\n5.6.7.0/30\n")
	runOnce(t, eng)

	latestPath := filepath.Join(root, "lib", "alpha", "latest")
	if err := os.Remove(latestPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	cache := newLatestSetCache(eng)
	defer cache.CloseAll(eng.logger)

	first, err := cache.Open("alpha")
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Open("alpha")
	if err != nil {
		t.Fatal(err)
	}

	if first == second {
		t.Fatalf("expected text fallback opens to stay uncached, got shared pointer %p", first)
	}
}

func TestGeoProviderCacheReloadsOnFreshnessChange(t *testing.T) {
	cache := newGeoProviderCache()
	path := filepath.Join(t.TempDir(), "dbip.csv.gz")

	writeDBIPGeoArchive(t, path,
		"1.0.0.0,1.0.0.0,US",
		"2.0.0.0,2.0.0.1,DE",
	)

	first, err := cache.LoadOrParse("dbip_country", "dbip_country_csv", path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.LoadOrParse("dbip_country", "dbip_country_csv", path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("expected unchanged provider freshness to reuse cached prepared dataset")
	}

	writeDBIPGeoArchive(t, path,
		"1.0.0.0,1.0.0.3,US",
		"2.0.0.0,2.0.0.1,DE",
		"3.0.0.0,3.0.0.0,FR",
	)
	newMTime := time.Now().UTC().Add(2 * time.Second)
	if err := os.Chtimes(path, newMTime, newMTime); err != nil {
		t.Fatal(err)
	}

	third, err := cache.LoadOrParse("dbip_country", "dbip_country_csv", path)
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatal("expected changed provider freshness to rebuild prepared dataset")
	}
	if third.totalIPs <= first.totalIPs {
		t.Fatalf("expected rebuilt dataset to reflect larger payload, got first=%d third=%d", first.totalIPs, third.totalIPs)
	}
}

func TestGeoProviderFreshnessDetectsContentChangeWithStableMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider.source")
	bodyA := []byte("same-size-payload-A")
	bodyB := []byte("same-size-payload-B")
	if len(bodyA) != len(bodyB) {
		t.Fatalf("test setup bug: expected same-length payloads, got %d and %d", len(bodyA), len(bodyB))
	}
	if err := os.WriteFile(path, bodyA, 0o600); err != nil {
		t.Fatal(err)
	}
	stableTime := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, stableTime, stableTime); err != nil {
		t.Fatal(err)
	}
	first, err := currentGeoProviderFreshness("dbip_country_csv", path)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, bodyB, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, stableTime, stableTime); err != nil {
		t.Fatal(err)
	}
	second, err := currentGeoProviderFreshness("dbip_country_csv", path)
	if err != nil {
		t.Fatal(err)
	}

	if first.bodyHash == second.bodyHash {
		t.Fatal("expected freshness hash to change when file content changes")
	}
}

func TestPreparedGeoProviderCountSourceMatchesLegacySemantics(t *testing.T) {
	dataset := &geoloc.Dataset{
		Provider: "synthetic",
		Sets: map[string]*iprange.IPSet{
			"US": mustSet(t, "us", iprange.Range{Lo: 10, Hi: 13}),
			"DE": mustSet(t, "de", iprange.Range{Lo: 12, Hi: 15}),
			"FR": mustSet(t, "fr", iprange.Range{Lo: 30, Hi: 31}),
		},
	}
	prepared, err := prepareGeoProvider("synthetic", dataset)
	if err != nil {
		t.Fatal(err)
	}

	feed := mustSet(t, "feed", iprange.Range{Lo: 11, Hi: 14})

	values, totalMapped := prepared.CountSource(feed)
	if totalMapped != 4 {
		t.Fatalf("expected total_mapped 4, got %d", totalMapped)
	}

	got := make(map[string]uint64, len(values))
	for _, row := range values {
		got[row.Code] = row.Value
	}
	want := map[string]uint64{
		"DE": 3,
		"US": 3,
	}
	if !maps.Equal(got, want) {
		t.Fatalf("unexpected country counts: got %+v want %+v", got, want)
	}
}

func TestPreparedGeoProviderCountSourcePropagatesSourceErrors(t *testing.T) {
	dataset := &geoloc.Dataset{
		Provider: "synthetic",
		Sets: map[string]*iprange.IPSet{
			"US": mustSet(t, "us", iprange.Range{Lo: 10, Hi: 13}),
		},
	}
	prepared, err := prepareGeoProvider("synthetic", dataset)
	if err != nil {
		t.Fatal(err)
	}

	sentinel := errors.New("range source failed")
	src := iprange.RangeSourceFromIterErr(func(yield func(iprange.Range) bool) error {
		if !yield(iprange.Range{Lo: 11, Hi: 12}) {
			return nil
		}
		return sentinel
	}, -1)

	_, _, err = prepared.CountSourceContext(t.Context(), src)
	if !errors.Is(err, sentinel) {
		t.Fatalf("CountSourceContext() error = %v, want %v", err, sentinel)
	}
}

func TestCountryFilteredRangeSourcePropagatesSourceErrors(t *testing.T) {
	dataset := &geoloc.Dataset{
		Provider: "synthetic",
		Sets: map[string]*iprange.IPSet{
			"US": mustSet(t, "us", iprange.Range{Lo: 10, Hi: 13}),
		},
	}
	prepared, err := prepareGeoProvider("synthetic", dataset)
	if err != nil {
		t.Fatal(err)
	}

	sentinel := errors.New("range source failed")
	src := iprange.RangeSourceFromIterErr(func(yield func(iprange.Range) bool) error {
		if !yield(iprange.Range{Lo: 11, Hi: 12}) {
			return nil
		}
		return sentinel
	}, -1)

	filtered := countryFilteredRangeSource(src, prepared, "US")
	err = iprange.WalkRangesContext(t.Context(), filtered, func(iprange.Range) bool {
		return true
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WalkRangesContext(countryFilteredRangeSource) error = %v, want %v", err, sentinel)
	}
}

func TestLookupCountryInPreparedProviderMatchesAlphabeticalLegacyChoice(t *testing.T) {
	dataset := &geoloc.Dataset{
		Provider: "synthetic",
		Sets: map[string]*iprange.IPSet{
			"US":        mustSet(t, "us", iprange.Range{Lo: 100, Hi: 120}),
			"ANONYMOUS": mustSet(t, "anonymous", iprange.Range{Lo: 110, Hi: 115}),
		},
	}
	prepared, err := prepareGeoProvider("synthetic", dataset)
	if err != nil {
		t.Fatal(err)
	}

	if got := lookupCountryInPreparedProvider(prepared, 112); got != "ANONYMOUS" {
		t.Fatalf("expected alphabetical first matching code ANONYMOUS, got %q", got)
	}
	if got := lookupCountryInPreparedProvider(prepared, 121); got != "" {
		t.Fatalf("expected no match outside provider ranges, got %q", got)
	}
}

func writeDBIPGeoArchive(t *testing.T, path string, rows ...string) {
	t.Helper()

	var payload bytes.Buffer
	zw := gzip.NewWriter(&payload)
	for _, row := range rows {
		if _, err := zw.Write([]byte(row + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustSet(t *testing.T, name string, ranges ...iprange.Range) *iprange.IPSet {
	t.Helper()

	set := iprange.New(name)
	for _, r := range ranges {
		if err := set.AddRange(r); err != nil {
			t.Fatal(err)
		}
	}
	set.Optimize()
	return set
}
