package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestComparisonSparsePrefixOverlap(t *testing.T) {
	left := buildComparisonSetSignature(mustBitmapSet(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000001}, // 10.0.0.1
	))
	right := buildComparisonSetSignature(mustBitmapSet(t,
		iprange.Range{Lo: 0x0A000201, Hi: 0x0A000201}, // 10.0.2.1, same /20 as left
	))
	if !comparisonPrefixOverlap(left.prefixBitmap, right.prefixBitmap) {
		t.Fatal("test setup expected coarse /20 prefixes to overlap")
	}
	if comparisonSparsePrefixOverlap(left.sparsePrefix, right.sparsePrefix) {
		t.Fatal("expected disjoint sparse /24 occupancy to skip the pair")
	}

	overlapping := buildComparisonSetSignature(mustBitmapSet(t,
		iprange.Range{Lo: 0x0A0000FE, Hi: 0x0A000101}, // spans 10.0.0.0/24 and 10.0.1.0/24
	))
	if !comparisonSparsePrefixOverlap(left.sparsePrefix, overlapping.sparsePrefix) {
		t.Fatal("expected shared sparse /24 occupancy to keep the pair")
	}
}

func TestComparisonSparsePrefixOverflowFallsBack(t *testing.T) {
	broad := buildComparisonSetSignature(mustBitmapSet(t,
		iprange.Range{Lo: 0, Hi: 0xFFFFFFFF},
	))
	narrow := buildComparisonSetSignature(mustBitmapSet(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000001},
	))
	if broad.sparsePrefix != nil {
		t.Fatal("broad set should overflow the sparse prefix index")
	}
	if !comparisonSparsePrefixOverlap(broad.sparsePrefix, narrow.sparsePrefix) {
		t.Fatal("overflowed sparse index must fall back to possible overlap")
	}
}

func TestRangeOverlapFiltersDisjoint(t *testing.T) {
	left := buildRangeOverlapFilter(mustBitmapSet(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000001},
	))
	rightSameCoarsePrefix := buildRangeOverlapFilter(mustBitmapSet(t,
		iprange.Range{Lo: 0x0A000201, Hi: 0x0A000201},
	))
	if !rangeOverlapFiltersDisjoint(left, rightSameCoarsePrefix) {
		t.Fatal("same-/20, different-/24 filters should prove disjoint")
	}
	if rangeOverlapFiltersDisjoint(rangeOverlapFilter{}, left) {
		t.Fatal("unknown filters must fall through to full counting")
	}

	rightDifferentBounds := buildRangeOverlapFilter(mustBitmapSet(t,
		iprange.Range{Lo: 0xC0000201, Hi: 0xC0000201},
	))
	if !rangeOverlapFiltersDisjoint(left, rightDifferentBounds) {
		t.Fatal("non-overlapping bounds should prove disjoint")
	}

	overlapping := buildRangeOverlapFilter(mustBitmapSet(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000002},
	))
	if rangeOverlapFiltersDisjoint(left, overlapping) {
		t.Fatal("overlapping ranges must fall through to full counting")
	}
}

func TestComparisonSetsIdentical(t *testing.T) {
	left := buildComparisonSetSignature(mustBitmapSet(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000001},
		iprange.Range{Lo: 0x0A000010, Hi: 0x0A000020},
	))
	right := buildComparisonSetSignature(mustBitmapSet(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000001},
		iprange.Range{Lo: 0x0A000010, Hi: 0x0A000020},
	))
	changed := buildComparisonSetSignature(mustBitmapSet(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000001},
		iprange.Range{Lo: 0x0A000011, Hi: 0x0A000020},
	))
	sameCountChanged := buildComparisonSetSignature(mustBitmapSet(t,
		iprange.Range{Lo: 0x0A000100, Hi: 0x0A000110},
		iprange.Range{Lo: 0x0A000200, Hi: 0x0A000200},
	))
	if !comparisonSetsIdentical(comparisonSetInfo{ips: 18, contentHash: left.contentHash}, comparisonSetInfo{ips: 18, contentHash: right.contentHash}) {
		t.Fatal("same range content should be detected as identical")
	}
	if comparisonSetsIdentical(comparisonSetInfo{ips: 18, contentHash: left.contentHash}, comparisonSetInfo{ips: 17, contentHash: changed.contentHash}) {
		t.Fatal("different range content should not be detected as identical")
	}
	if comparisonSetsIdentical(comparisonSetInfo{ips: 18, contentHash: left.contentHash}, comparisonSetInfo{ips: 18, contentHash: sameCountChanged.contentHash}) {
		t.Fatal("different range content with same IP count should not be detected as identical")
	}
}

func TestWriteComparisonFilesUsesSparsePrefixToRemoveSameCoarsePrefixStaleRows(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.New()
	cfg.Sources["alpha"] = &config.Source{Name: "alpha", Frequency: 60, IPV: "ipv4", Output: "ip", Category: "test"}
	cfg.Sources["beta"] = &config.Source{Name: "beta", Frequency: 60, IPV: "ipv4", Output: "ip", Category: "test"}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
		rt.WebDir = webDir
		rt.LibDir = filepath.Join(root, "lib")
		rt.MaxHeavyPhaseWorkers = 1
	}))
	for name, body := range map[string]string{
		"alpha": "10.0.0.1\n",
		"beta":  "10.0.2.1\n",
	} {
		file := name + ".ipset"
		if err := os.WriteFile(filepath.Join(baseDir, file), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		entry := eng.state.Entry(name)
		entry.Name = name
		entry.File = file
		entry.Category = "test"
		entry.ProcessedDate = time.Now().UTC().Unix()
		entry.CheckedDate = entry.ProcessedDate
		entry.SourceDate = entry.ProcessedDate
	}

	stale := []byte(`[{"name":"beta","category":"test","ips":1,"common":1}]` + "\n")
	if err := os.WriteFile(filepath.Join(webDir, "alpha_comparison.json"), stale, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := eng.writeComparisonFiles(t.Context(), []string{"alpha"}, webDir, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(webDir, "alpha_comparison.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []CompareRow
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected stale same-/20 overlap to be removed by exact sparse prefix skip, got %+v", rows)
	}
}
