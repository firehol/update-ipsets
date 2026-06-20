package iprange

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestParseReaderReportsOperationStats(t *testing.T) {
	opts := DefaultParseOptions()
	opts.Resolver = staticResolver{
		"example.net": {mustIP(t, "203.0.113.10")},
	}
	var stats OperationStats
	opts.Stats = &stats

	set, err := ParseReader(t.Context(), "stats", strings.NewReader("1.2.3.4\nbad line\nexample.net\n"), opts)
	if err != nil {
		t.Fatal(err)
	}

	if set.UniqueCount() != 2 {
		t.Fatalf("unique IPs = %d, want 2", set.UniqueCount())
	}
	if stats.BytesRead != int64(len("1.2.3.4\nbad line\nexample.net\n")) {
		t.Fatalf("BytesRead = %d, want %d", stats.BytesRead, len("1.2.3.4\nbad line\nexample.net\n"))
	}
	if stats.LinesRead != 3 {
		t.Fatalf("LinesRead = %d, want 3", stats.LinesRead)
	}
	if stats.RangesAccepted != 1 {
		t.Fatalf("RangesAccepted = %d, want 1", stats.RangesAccepted)
	}
	if stats.HostnamesQueued != 1 || stats.HostnamesCompleted != 1 || stats.HostnamesResolved != 1 {
		t.Fatalf("hostname stats = queued:%d completed:%d resolved:%d, want 1/1/1", stats.HostnamesQueued, stats.HostnamesCompleted, stats.HostnamesResolved)
	}
}

func TestRangeSourceContainsWithStatsMatchesContains(t *testing.T) {
	set := newOptimizedSet("contains_stats",
		Range{Lo: 10, Hi: 20},
		Range{Lo: 100, Hi: 200},
	)
	path := writeTempSet(t, set)
	fs, err := OpenFileSet(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()

	for _, src := range []RangeSource{set, fs} {
		for _, ip := range []uint32{0, 10, 15, 20, 99, 100, 150, 200, 201} {
			got, stats, err := RangeSourceContains(t.Context(), src, ip)
			if err != nil {
				t.Fatalf("RangeSourceContains(%d) error = %v", ip, err)
			}
			want := false
			if withContains, ok := src.(interface{ Contains(uint32) bool }); ok {
				want = withContains.Contains(ip)
			}
			if got != want {
				t.Fatalf("RangeSourceContains(%d) = %v, want %v", ip, got, want)
			}
			if stats.Lookups != 1 {
				t.Fatalf("Lookups = %d, want 1", stats.Lookups)
			}
			if stats.BinarySearches != 1 {
				t.Fatalf("BinarySearches = %d, want 1", stats.BinarySearches)
			}
			if stats.Comparisons == 0 && set.Len() > 0 {
				t.Fatalf("Comparisons = %d, want > 0", stats.Comparisons)
			}
		}
	}
}

func TestSourceAlgebraWithStatsMatchesPlainAPIs(t *testing.T) {
	left := newOptimizedSet("left",
		Range{Lo: 0, Hi: 10},
		Range{Lo: 20, Hi: 30},
		Range{Lo: ^uint32(0) - 1, Hi: ^uint32(0)},
	)
	right := newOptimizedSet("right",
		Range{Lo: 5, Hi: 15},
		Range{Lo: 30, Hi: 40},
		Range{Lo: ^uint32(0), Hi: ^uint32(0)},
	)

	wantUnion, err := UnionSourcesContext(t.Context(), "union", left, right)
	if err != nil {
		t.Fatal(err)
	}
	gotUnion, unionStats, err := UnionSourcesWithStatsContext(t.Context(), "union", left, right)
	if err != nil {
		t.Fatal(err)
	}
	expectRanges(t, gotUnion, wantUnion.Ranges)
	if unionStats.RangesRead == 0 || unionStats.RangesEmitted == 0 {
		t.Fatalf("union stats = %+v, want read and emitted ranges", unionStats)
	}

	wantIntersect, err := IntersectSourcesContext(t.Context(), "intersect", left, right)
	if err != nil {
		t.Fatal(err)
	}
	gotIntersect, intersectStats, err := IntersectSourcesWithStatsContext(t.Context(), "intersect", left, right)
	if err != nil {
		t.Fatal(err)
	}
	expectRanges(t, gotIntersect, wantIntersect.Ranges)
	if intersectStats.RangesScanned == 0 || intersectStats.RangesEmitted == 0 {
		t.Fatalf("intersect stats = %+v, want scanned and emitted ranges", intersectStats)
	}

	wantExclude, err := ExcludeSourcesContext(t.Context(), "exclude", left, right)
	if err != nil {
		t.Fatal(err)
	}
	gotExclude, excludeStats, err := ExcludeSourcesWithStatsContext(t.Context(), "exclude", left, right)
	if err != nil {
		t.Fatal(err)
	}
	expectRanges(t, gotExclude, wantExclude.Ranges)
	if excludeStats.RangesScanned == 0 || excludeStats.RangesEmitted == 0 {
		t.Fatalf("exclude stats = %+v, want scanned and emitted ranges", excludeStats)
	}

	wantCount, err := ExcludeCountContext(t.Context(), left, right)
	if err != nil {
		t.Fatal(err)
	}
	gotCount, countStats, err := ExcludeCountWithStatsContext(t.Context(), left, right)
	if err != nil {
		t.Fatal(err)
	}
	if gotCount != wantCount {
		t.Fatalf("ExcludeCountWithStatsContext() = %d, want %d", gotCount, wantCount)
	}
	if countStats.RangesScanned == 0 {
		t.Fatalf("count stats = %+v, want scanned ranges", countStats)
	}

	fallbackLeft := RangeSourceFromIter(left.Iter(), left.Len())
	fallbackRight := RangeSourceFromIter(right.Iter(), right.Len())
	fallbackCount, fallbackStats, err := ExcludeCountWithStatsContext(t.Context(), fallbackLeft, fallbackRight)
	if err != nil {
		t.Fatal(err)
	}
	if fallbackCount != wantCount {
		t.Fatalf("generic ExcludeCountWithStatsContext() = %d, want %d", fallbackCount, wantCount)
	}
	if fallbackStats.RangesEmitted == 0 {
		t.Fatalf("generic count stats = %+v, want emitted ranges", fallbackStats)
	}
}

func TestBinaryRoundTripWithStats(t *testing.T) {
	set := newOptimizedSet("binary_stats",
		Range{Lo: 10, Hi: 20},
		Range{Lo: 100, Hi: 200},
	)
	var buf bytes.Buffer

	writeStats, err := WriteBinaryWithStats(&buf, set)
	if err != nil {
		t.Fatal(err)
	}
	if writeStats.RangesWritten != int64(len(set.Ranges)) {
		t.Fatalf("RangesWritten = %d, want %d", writeStats.RangesWritten, len(set.Ranges))
	}
	if writeStats.BytesWritten != int64(buf.Len()) {
		t.Fatalf("BytesWritten = %d, want %d", writeStats.BytesWritten, buf.Len())
	}

	loaded, readStats, err := ReadBinaryWithStats("binary_stats", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	expectRanges(t, loaded, set.Ranges)
	if readStats.RangesRead != int64(len(set.Ranges)) {
		t.Fatalf("RangesRead = %d, want %d", readStats.RangesRead, len(set.Ranges))
	}
	if readStats.BytesRead != int64(buf.Len()) {
		t.Fatalf("BytesRead = %d, want %d", readStats.BytesRead, buf.Len())
	}
}

func TestOperationStatsAdd(t *testing.T) {
	var got OperationStats
	got.Add(OperationStats{BytesRead: 1, LinesRead: 2, RangesAccepted: 3})
	got.Add(OperationStats{BytesRead: 10, LinesRead: 20, RangesAccepted: 30})

	want := OperationStats{BytesRead: 11, LinesRead: 22, RangesAccepted: 33}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("OperationStats.Add() = %+v, want %+v", got, want)
	}
}
