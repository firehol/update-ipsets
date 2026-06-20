package engine

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestComparisonSparsePrefixOverlap(t *testing.T) {
	left := mustRangeSummary(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000001}, // 10.0.0.1
	)
	right := mustRangeSummary(t,
		iprange.Range{Lo: 0x0A000201, Hi: 0x0A000201}, // 10.0.2.1, same /20 as left
	)
	if left.OverlapFilter().PrefixesDisjoint(right.OverlapFilter()) {
		t.Fatal("test setup expected coarse /20 prefixes to overlap")
	}
	if !left.OverlapFilter().SparsePrefixesDisjoint(right.OverlapFilter()) {
		t.Fatal("expected disjoint sparse /24 occupancy to skip the pair")
	}

	overlapping := mustRangeSummary(t,
		iprange.Range{Lo: 0x0A0000FE, Hi: 0x0A000101}, // spans 10.0.0.0/24 and 10.0.1.0/24
	)
	if left.OverlapFilter().SparsePrefixesDisjoint(overlapping.OverlapFilter()) {
		t.Fatal("expected shared sparse /24 occupancy to keep the pair")
	}
}

func TestBuildComparisonSetSignatureContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	signature, err := iprange.BuildRangeSourceSummaryContext(ctx, mustBitmapSet(t,
		iprange.Range{Lo: 1, Hi: 10},
	))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("BuildRangeSourceSummaryContext() error = %v, want context.Canceled", err)
	}
	if signature.ContentHash.Valid {
		t.Fatal("cancelled signature unexpectedly has a content hash")
	}
}

func TestComparisonSparsePrefixOverflowFallsBack(t *testing.T) {
	broad := mustRangeSummary(t,
		iprange.Range{Lo: 0, Hi: 0xFFFFFFFF},
	)
	narrow := mustRangeSummary(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000001},
	)
	if broad.OverlapFilter().SparsePrefixesDisjoint(narrow.OverlapFilter()) {
		t.Fatal("overflowed sparse index must fall back to possible overlap")
	}
}

func TestRangeOverlapFiltersDisjoint(t *testing.T) {
	left := mustRangeOverlapFilter(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000001},
	)
	rightSameCoarsePrefix := mustRangeOverlapFilter(t,
		iprange.Range{Lo: 0x0A000201, Hi: 0x0A000201},
	)
	if !left.Disjoint(rightSameCoarsePrefix) {
		t.Fatal("same-/20, different-/24 filters should prove disjoint")
	}
	if (iprange.RangeOverlapFilter{}).Disjoint(left) {
		t.Fatal("unknown filters must fall through to full counting")
	}

	rightDifferentBounds := mustRangeOverlapFilter(t,
		iprange.Range{Lo: 0xC0000201, Hi: 0xC0000201},
	)
	if !left.Disjoint(rightDifferentBounds) {
		t.Fatal("non-overlapping bounds should prove disjoint")
	}

	overlapping := mustRangeOverlapFilter(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000002},
	)
	if left.Disjoint(overlapping) {
		t.Fatal("overlapping ranges must fall through to full counting")
	}
}

func TestComparisonSetsIdentical(t *testing.T) {
	left := mustRangeSummary(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000001},
		iprange.Range{Lo: 0x0A000010, Hi: 0x0A000020},
	)
	right := mustRangeSummary(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000001},
		iprange.Range{Lo: 0x0A000010, Hi: 0x0A000020},
	)
	changed := mustRangeSummary(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A000001},
		iprange.Range{Lo: 0x0A000011, Hi: 0x0A000020},
	)
	sameCountChanged := mustRangeSummary(t,
		iprange.Range{Lo: 0x0A000100, Hi: 0x0A000110},
		iprange.Range{Lo: 0x0A000200, Hi: 0x0A000200},
	)
	if !comparisonSetsIdentical(comparisonSetInfo{ips: 18, contentHash: left.ContentHash}, comparisonSetInfo{ips: 18, contentHash: right.ContentHash}) {
		t.Fatal("same range content should be detected as identical")
	}
	if comparisonSetsIdentical(comparisonSetInfo{ips: 18, contentHash: left.ContentHash}, comparisonSetInfo{ips: 17, contentHash: changed.ContentHash}) {
		t.Fatal("different range content should not be detected as identical")
	}
	if comparisonSetsIdentical(comparisonSetInfo{ips: 18, contentHash: left.ContentHash}, comparisonSetInfo{ips: 18, contentHash: sameCountChanged.ContentHash}) {
		t.Fatal("different range content with same IP count should not be detected as identical")
	}
}

func mustRangeSummary(t *testing.T, ranges ...iprange.Range) iprange.RangeSourceSummary {
	t.Helper()
	summary, err := iprange.BuildRangeSourceSummaryContext(t.Context(), mustBitmapSet(t, ranges...))
	if err != nil {
		t.Fatalf("BuildRangeSourceSummaryContext() error = %v", err)
	}
	return summary
}

func mustRangeOverlapFilter(t *testing.T, ranges ...iprange.Range) iprange.RangeOverlapFilter {
	t.Helper()
	filter, err := iprange.BuildRangeOverlapFilterContext(t.Context(), mustBitmapSet(t, ranges...))
	if err != nil {
		t.Fatalf("BuildRangeOverlapFilterContext() error = %v", err)
	}
	return filter
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

func TestWriteComparisonFilesDoesNotSanitizeUntouchedLiveArtifacts(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	stageDir := filepath.Join(root, "stage")
	for _, dir := range []string{baseDir, webDir, stageDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
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
		"alpha": "10.0.0.0/24\n",
		"beta":  "192.0.2.0/24\n",
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

	orphan := []byte(`[{"name":"zero","ips":1,"common":0}]` + "\n")
	if err := os.WriteFile(filepath.Join(webDir, "orphan_comparison.json"), orphan, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := eng.writeComparisonFiles(t.Context(), []string{"alpha"}, stageDir, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(stageDir, "beta_comparison.json")); err != nil {
		t.Fatalf("expected paired beta comparison artifact to be staged, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(stageDir, "orphan_comparison.json")); !os.IsNotExist(err) {
		t.Fatalf("untouched live comparison artifact was sanitized into stage, stat err=%v", err)
	}
	data, err := os.ReadFile(filepath.Join(webDir, "orphan_comparison.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(orphan) {
		t.Fatalf("untouched live comparison artifact changed to %q, want %q", data, orphan)
	}
}

func TestWriteComparisonFilesSkipsCurrentLiveComparisonArtifacts(t *testing.T) {
	eng, webDir, stageDir, logical := newCurrentComparisonLiveFixture(t, "")
	alphaLive := filepath.Join(webDir, "alpha_comparison.json")
	betaLive := filepath.Join(webDir, "beta_comparison.json")
	alphaBefore := statFile(t, alphaLive)
	betaBefore := statFile(t, betaLive)

	if err := eng.writeComparisonFiles(t.Context(), []string{"alpha"}, stageDir, nil); err != nil {
		t.Fatal(err)
	}

	assertMissingFile(t, filepath.Join(stageDir, "alpha_comparison.json"))
	assertMissingFile(t, filepath.Join(stageDir, "beta_comparison.json"))
	alphaAfter := statFile(t, alphaLive)
	betaAfter := statFile(t, betaLive)
	if !alphaAfter.ModTime().UTC().Equal(logical) || !betaAfter.ModTime().UTC().Equal(logical) {
		t.Fatalf("live mtimes changed or were not logical: alpha=%s beta=%s want=%s", alphaAfter.ModTime().UTC(), betaAfter.ModTime().UTC(), logical)
	}
	if alphaAfter.ModTime() != alphaBefore.ModTime() || betaAfter.ModTime() != betaBefore.ModTime() {
		t.Fatalf("current live artifacts were touched: alpha before=%s after=%s beta before=%s after=%s", alphaBefore.ModTime(), alphaAfter.ModTime(), betaBefore.ModTime(), betaAfter.ModTime())
	}
}

func TestWriteComparisonFilesStagesComparisonArtifactWithStaleMTime(t *testing.T) {
	eng, webDir, stageDir, logical := newCurrentComparisonLiveFixture(t, "")
	alphaLive := filepath.Join(webDir, "alpha_comparison.json")
	stale := logical.Add(-time.Hour)
	if err := os.Chtimes(alphaLive, stale, stale); err != nil {
		t.Fatalf("Chtimes(%q) error = %v", alphaLive, err)
	}

	if err := eng.writeComparisonFiles(t.Context(), []string{"alpha"}, stageDir, nil); err != nil {
		t.Fatal(err)
	}

	stageInfo := statFile(t, filepath.Join(stageDir, "alpha_comparison.json"))
	if got := stageInfo.ModTime().UTC(); !got.Equal(logical) {
		t.Fatalf("staged mtime = %s, want %s", got, logical)
	}
	assertMissingFile(t, filepath.Join(stageDir, "beta_comparison.json"))
}

func TestWriteComparisonFilesStagesComparisonArtifactWithWrongMode(t *testing.T) {
	eng, webDir, stageDir, logical := newCurrentComparisonLiveFixture(t, "")
	alphaLive := filepath.Join(webDir, "alpha_comparison.json")
	wrongMode := generatedFileMode ^ 0o040
	if err := os.Chmod(alphaLive, wrongMode); err != nil {
		t.Fatalf("Chmod(%q) error = %v", alphaLive, err)
	}

	if err := eng.writeComparisonFiles(t.Context(), []string{"alpha"}, stageDir, nil); err != nil {
		t.Fatal(err)
	}

	stageInfo := statFile(t, filepath.Join(stageDir, "alpha_comparison.json"))
	if got := stageInfo.Mode().Perm(); got != generatedFileMode {
		t.Fatalf("staged mode = %04o, want %04o", got, generatedFileMode)
	}
	if got := stageInfo.ModTime().UTC(); !got.Equal(logical) {
		t.Fatalf("staged mtime = %s, want %s", got, logical)
	}
	assertMissingFile(t, filepath.Join(stageDir, "beta_comparison.json"))
}

func TestWriteComparisonFilesStagesCurrentComparisonArtifactWhenOwnerConfigured(t *testing.T) {
	eng, _, stageDir, _ := newCurrentComparisonLiveFixture(t, "web-owner")

	if err := eng.writeComparisonFiles(t.Context(), []string{"alpha"}, stageDir, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(stageDir, "alpha_comparison.json")); err != nil {
		t.Fatalf("expected configured owner to force staging, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(stageDir, "beta_comparison.json")); err != nil {
		t.Fatalf("expected configured owner to force paired staging, stat err=%v", err)
	}
}

func TestWriteComparisonFilesReplacesSymlinkStageComparisonArtifact(t *testing.T) {
	eng, webDir, stageDir, logical := newCurrentComparisonLiveFixture(t, "")
	alphaLive := filepath.Join(webDir, "alpha_comparison.json")
	alphaStage := filepath.Join(stageDir, "alpha_comparison.json")
	if err := os.Symlink(alphaLive, alphaStage); err != nil {
		t.Skipf("symlink creation unsupported: %v", err)
	}

	if err := eng.writeComparisonFiles(t.Context(), []string{"alpha"}, stageDir, nil); err != nil {
		t.Fatal(err)
	}

	linkInfo, err := os.Lstat(alphaStage)
	if err != nil {
		t.Fatalf("Lstat(%q) error = %v", alphaStage, err)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("stage comparison artifact remained a symlink: mode=%s", linkInfo.Mode())
	}
	stageInfo := statFile(t, alphaStage)
	if got := stageInfo.ModTime().UTC(); !got.Equal(logical) {
		t.Fatalf("staged mtime = %s, want %s", got, logical)
	}
}

func newCurrentComparisonLiveFixture(t *testing.T, webOwner string) (*Engine, string, string, time.Time) {
	t.Helper()
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	webDir := filepath.Join(root, "web")
	stageDir := filepath.Join(root, "stage")
	for _, dir := range []string{baseDir, webDir, stageDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.New()
	cfg.Sources["alpha"] = &config.Source{Name: "alpha", Frequency: 60, IPV: "ipv4", Output: "ip", Category: "test"}
	cfg.Sources["beta"] = &config.Source{Name: "beta", Frequency: 60, IPV: "ipv4", Output: "ip", Category: "test"}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
		rt.WebDir = webDir
		rt.LibDir = filepath.Join(root, "lib")
		rt.MaxHeavyPhaseWorkers = 1
		rt.WebOwner = webOwner
	}))
	logical := time.Date(2026, 6, 19, 9, 30, 0, 0, time.UTC)
	for name, body := range map[string]string{
		"alpha": "10.0.0.1\n",
		"beta":  "10.0.0.1\n",
	} {
		file := name + ".ipset"
		if err := os.WriteFile(filepath.Join(baseDir, file), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		entry := eng.state.Entry(name)
		entry.Name = name
		entry.File = file
		entry.Category = "test"
		entry.ProcessedDate = logical.Unix()
		entry.CheckedDate = entry.ProcessedDate
		entry.SourceDate = entry.ProcessedDate
	}
	if err := eng.writeComparisonFiles(t.Context(), []string{"alpha"}, webDir, nil); err != nil {
		t.Fatal(err)
	}
	return eng, webDir, stageDir, logical
}

func statFile(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	return info
}

func assertMissingFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q to be missing, stat err=%v", path, err)
	}
}
