package engine

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

var updateGoldenFiles = flag.Bool("update", false, "rewrite golden files")

func TestBuildSetMetadataUsesEnableAllForMergeComposition(t *testing.T) {
	baseDir := t.TempDir()
	cfg := config.New()
	cfg.Sources["included"] = &config.Source{Name: "included", Frequency: 60, IPV: "ipv4", Output: "netset"}
	cfg.Sources["subtracted"] = &config.Source{Name: "subtracted", Frequency: 60, IPV: "ipv4", Output: "netset"}
	cfg.Sources["merged"] = &config.Source{
		Name:         "merged",
		Frequency:    60,
		IPV:          "ipv4",
		Output:       "netset",
		DerivedFrom:  []string{"included", "subtracted"},
		MergeSources: []string{"included"},
		MergeExclude: []string{"subtracted"},
		Provenance:   config.ProvenanceSecondaryMerge,
	}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
	}))
	if err := os.WriteFile(eng.feedBodyPath("included"), []byte("10.0.0.0/24\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eng.feedBodyPath("subtracted"), []byte("10.0.0.0/25\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	meta := eng.buildSetMetadataWithEnableAll("merged", &cache.Entry{Name: "merged"}, true)

	if len(meta.MergeIncluded) != 1 || meta.MergeIncluded[0].Name != "included" {
		t.Fatalf("merge included = %+v, want included", meta.MergeIncluded)
	}
	if len(meta.MergeSubtracted) != 1 || meta.MergeSubtracted[0].Name != "subtracted" {
		t.Fatalf("merge subtracted = %+v, want subtracted", meta.MergeSubtracted)
	}
	if len(meta.MergeExcluded) != 0 {
		t.Fatalf("merge excluded = %+v, want none", meta.MergeExcluded)
	}
}

func TestRedistributableDerivedFeedInheritsParents(t *testing.T) {
	redistributable := false
	cfg := config.New()
	cfg.Sources["private"] = &config.Source{
		Name:            "private",
		Redistributable: &redistributable,
	}
	cfg.Sources["derived"] = &config.Source{
		Name:        "derived",
		DerivedFrom: []string{"private"},
	}
	eng := newEngineFixture(t, withConfig(cfg))

	if eng.IsRedistributable("derived") {
		t.Fatal("derived feed should be non-redistributable when any parent is non-redistributable")
	}
	eng.runtime.LocalCopyURL = "https://files.example.test"
	eng.runtime.GitHubChangesURL = "https://history.example.test"
	eng.state.ReplaceEntry("derived", cache.Entry{
		Name:      "derived",
		File:      "derived.ipset",
		PublicURL: "https://public.example.test/derived.ipset",
	})
	metaPayload, err := eng.Metadata("derived")
	if err != nil {
		t.Fatal(err)
	}
	metaData, err := json.Marshal(metaPayload)
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		DontRedistribute bool   `json:"dont_redistribute"`
		Source           string `json:"source"`
		File             string `json:"file"`
		FileLocal        string `json:"file_local"`
		CommitHistory    string `json:"commit_history"`
	}
	if err := json.Unmarshal(metaData, &meta); err != nil {
		t.Fatal(err)
	}
	if !meta.DontRedistribute {
		t.Fatal("derived metadata should expose dont_redistribute when any parent is non-redistributable")
	}
	if meta.Source != "" || meta.File != "" || meta.FileLocal != "" || meta.CommitHistory != "" {
		t.Fatalf("non-redistributable metadata exposed raw/source fields: %+v", meta)
	}
	eng.state.ReplaceEntry("derived", cache.Entry{
		Name:      "derived",
		URL:       "https://example.test/private.txt",
		PublicURL: "/derived.ipset",
		File:      "derived.ipset",
		Source:    "private.txt",
	})
	summary := summariesByName(eng.PublicFeedSummaries())["derived"]
	if summary.Redistributable {
		t.Fatal("derived public summary should be non-redistributable when any parent is non-redistributable")
	}
	if summary.URL != "" || summary.PublicURL != "" || summary.File != "" || summary.Source != "" {
		t.Fatalf("non-redistributable public summary exposed raw/source fields: %+v", summary)
	}
}

func TestRedistributableMergeInheritsSubtractiveParents(t *testing.T) {
	redistributable := false
	cfg := config.New()
	cfg.Sources["included"] = &config.Source{Name: "included"}
	cfg.Sources["subtracted"] = &config.Source{
		Name:            "subtracted",
		Redistributable: &redistributable,
	}
	cfg.Sources["merged"] = &config.Source{
		Name:         "merged",
		DerivedFrom:  []string{"included", "subtracted"},
		MergeSources: []string{"included"},
		MergeExclude: []string{"subtracted"},
		Provenance:   config.ProvenanceSecondaryMerge,
	}
	eng := newEngineFixture(t, withConfig(cfg))

	if eng.IsRedistributable("merged") {
		t.Fatal("merge should be non-redistributable when a subtractive parent is non-redistributable")
	}
}

func TestLeafAncestorsIgnoreSubtractiveMergeParents(t *testing.T) {
	cfg := config.New()
	cfg.Sources["included"] = &config.Source{Name: "included"}
	cfg.Sources["subtracted"] = &config.Source{Name: "subtracted"}
	cfg.Sources["merged"] = &config.Source{
		Name:         "merged",
		DerivedFrom:  []string{"included", "subtracted"},
		MergeSources: []string{"included"},
		MergeExclude: []string{"subtracted"},
		Provenance:   config.ProvenanceSecondaryMerge,
	}

	ancestors := leafAncestors(cfg, "merged")
	if !ancestors["included"] {
		t.Fatalf("positive ancestors = %v, want included", ancestors)
	}
	if ancestors["subtracted"] {
		t.Fatalf("positive ancestors = %v, must not include subtractive parent", ancestors)
	}
}

func TestComparisonRowsDoNotMarkSubtractiveParentRelated(t *testing.T) {
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
	cfg.Sources["included"] = &config.Source{Name: "included", Frequency: 60, IPV: "ipv4", Output: "netset", Category: "test"}
	cfg.Sources["subtracted"] = &config.Source{Name: "subtracted", Frequency: 60, IPV: "ipv4", Output: "netset", Category: "test"}
	cfg.Sources["merged"] = &config.Source{
		Name:         "merged",
		Frequency:    60,
		IPV:          "ipv4",
		Output:       "netset",
		Category:     "test",
		DerivedFrom:  []string{"included", "subtracted"},
		MergeSources: []string{"included"},
		MergeExclude: []string{"subtracted"},
		Provenance:   config.ProvenanceSecondaryMerge,
	}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
		rt.WebDir = webDir
		rt.LibDir = filepath.Join(root, "lib")
		rt.MaxHeavyPhaseWorkers = 1
	}))
	for name, body := range map[string]string{
		"included":   "10.0.0.0/24\n",
		"subtracted": "10.0.0.0/24\n",
		"merged":     "10.0.0.0/25\n",
	} {
		file := name + ".netset"
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

	if err := eng.writeComparisonFiles(t.Context(), nil, webDir, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(webDir, "merged_comparison.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []CompareRow
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	var includedRow, subtractedRow *CompareRow
	for i := range rows {
		switch rows[i].Name {
		case "included":
			includedRow = &rows[i]
		case "subtracted":
			subtractedRow = &rows[i]
		}
	}
	if includedRow == nil || !includedRow.Related {
		t.Fatalf("included row = %+v, want related=true in rows %+v", includedRow, rows)
	}
	if subtractedRow == nil {
		t.Fatalf("missing subtracted row in rows %+v", rows)
	}
	if subtractedRow.Related {
		t.Fatalf("subtractive parent row = %+v, want related=false", subtractedRow)
	}
}

func TestComparisonPrefixOverlap(t *testing.T) {
	left := mustBitmapSet(t,
		iprange.Range{Lo: 0x0A000001, Hi: 0x0A00FFFF}, // 10.0.0.1 - 10.0.255.255
	)
	right := mustBitmapSet(t,
		iprange.Range{Lo: 0x0A020001, Hi: 0x0A02FFFF}, // 10.2.0.1 - 10.2.255.255
	)
	leftFilter, err := iprange.BuildRangeOverlapFilterContext(t.Context(), left)
	if err != nil {
		t.Fatalf("BuildRangeOverlapFilterContext(left) error = %v", err)
	}
	rightFilter, err := iprange.BuildRangeOverlapFilterContext(t.Context(), right)
	if err != nil {
		t.Fatalf("BuildRangeOverlapFilterContext(right) error = %v", err)
	}
	if !leftFilter.PrefixesDisjoint(rightFilter) {
		t.Fatal("expected disjoint prefix occupancy to skip the pair")
	}

	overlapping := mustBitmapSet(t,
		iprange.Range{Lo: 0x0A00F000, Hi: 0x0A0100FF}, // spans 10.0/16 and 10.1/16
	)
	overlapFilter, err := iprange.BuildRangeOverlapFilterContext(t.Context(), overlapping)
	if err != nil {
		t.Fatalf("BuildRangeOverlapFilterContext(overlapping) error = %v", err)
	}
	if leftFilter.PrefixesDisjoint(overlapFilter) {
		t.Fatal("expected shared prefix occupancy to keep the pair")
	}
}

func TestMergeCompareRowsDropsAndDeletesZeroOverlapRows(t *testing.T) {
	rows := mergeCompareRows(
		[]CompareRow{
			{Name: "keep", Category: "test", IPs: 10, Common: 3},
			{Name: "stale_zero", Category: "test", IPs: 10, Common: 0},
			{Name: "became_zero", Category: "test", IPs: 10, Common: 5},
		},
		[]CompareRow{
			{Name: "became_zero", Category: "test", IPs: 10, Common: 0},
			{Name: "fresh_zero", Category: "test", IPs: 10, Common: 0},
			{Name: "new_overlap", Category: "test", IPs: 10, Common: 2},
		},
	)

	byName := map[string]CompareRow{}
	for _, row := range rows {
		if row.Common == 0 {
			t.Fatalf("merged comparison rows contain zero overlap row: %+v in %+v", row, rows)
		}
		byName[row.Name] = row
	}
	if _, ok := byName["keep"]; !ok {
		t.Fatalf("positive existing row was not preserved: %+v", rows)
	}
	if _, ok := byName["new_overlap"]; !ok {
		t.Fatalf("positive fresh row was not added: %+v", rows)
	}
	for _, name := range []string{"stale_zero", "became_zero", "fresh_zero"} {
		if _, ok := byName[name]; ok {
			t.Fatalf("zero overlap row %q was preserved in %+v", name, rows)
		}
	}
}

func TestValidateComparisonPayloadRejectsZeroOverlapRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample_comparison.json")
	if err := os.WriteFile(path, []byte(`[{"name":"zero","ips":1,"common":0}]`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateComparisonPayload(path); err == nil {
		t.Fatal("expected zero-overlap comparison row to be invalid")
	}
}

func TestWriteComparisonFilesRemovesStaleZeroOverlapRows(t *testing.T) {
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

	stale := []byte(`[{"name":"beta","category":"test","ips":256,"common":10}]` + "\n")
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
		t.Fatalf("expected stale overlap to be removed when recomputed common=0, got %+v", rows)
	}
}

func TestRenderHeaderDropsRetiredToolNames(t *testing.T) {
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.BaseDir = "/tmp/update-ipsets-test"
		rt.WebURL = "https://iplists.firehol.org/"
	}), withNow(func() time.Time {
		return time.Unix(1_776_000_000, 0).UTC()
	}))
	eng.cfg.Sources["sample"] = &config.Source{Name: "sample"}
	eng.state.Entry("sample").Version = 6

	src := &config.Source{
		Name:          "sample",
		IPV:           "ipv4",
		Info:          "Example feed",
		Maintainer:    "FireHOL",
		MaintainerURL: "https://firehol.org/",
		URL:           "https://example.test/sample",
		Category:      "attacks",
		Frequency:     60,
	}
	set := iprange.New("sample")
	if err := set.Add(0x01020304, 0x01020304); err != nil {
		t.Fatal(err)
	}
	set.Optimize()

	header := eng.renderHeader("sample", src, "ip", set, time.Unix(1_775_999_000, 0).UTC())
	if !bytes.Contains(header, []byte("# Generated by FireHOL's update-ipsets\n")) {
		t.Fatalf("expected updated generator label, got:\n%s", header)
	}
	if bytes.Contains(header, []byte("update-ipsets.sh")) {
		t.Fatalf("unexpected legacy shell-wrapper mention in header:\n%s", header)
	}
	if bytes.Contains(header, []byte("Processed with FireHOL's iprange")) {
		t.Fatalf("unexpected obsolete iprange-binary mention in header:\n%s", header)
	}
}

func TestPublicSiteBaseURLPrefersPublicBaseURL(t *testing.T) {
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.PublicBaseURL = "https://public.example.test/root/?ignored=true#fragment"
		rt.WebURL = "https://public.example.test/root/ipsets/"
	}))

	if got, want := eng.publicSiteBaseURL(), "https://public.example.test/root"; got != want {
		t.Fatalf("publicSiteBaseURL() = %q, want %q", got, want)
	}
	if got, want := eng.publicFeedURLPrefix(eng.publicSiteBaseURL()), "https://public.example.test/root/ipsets"; got != want {
		t.Fatalf("publicFeedURLPrefix() = %q, want %q", got, want)
	}
}

func TestPublicSiteBaseURLDerivesFromWebURL(t *testing.T) {
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.WebURL = "https://iplists.firehol.org/ipsets/"
	}))

	if got, want := eng.publicSiteBaseURL(), "https://iplists.firehol.org"; got != want {
		t.Fatalf("publicSiteBaseURL() = %q, want %q", got, want)
	}
}

func TestRenderLLMSTXTOmitsAdminPaths(t *testing.T) {
	body := renderLLMSTXT("https://iplists.firehol.org", "https://iplists.firehol.org/ipsets", []string{"sample"})

	for _, want := range []string{
		"# FireHOL IP Lists",
		"https://iplists.firehol.org/methodology",
		"https://iplists.firehol.org/api/v1/sets",
		"https://iplists.firehol.org/ipsets/sample",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("llms.txt missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "/admin") || strings.Contains(body, "/api/v1/admin") {
		t.Fatalf("llms.txt exposed admin path:\n%s", body)
	}
}

func TestWritePublicMetadataFilesBuildsSitemapIndexAndDetailShards(t *testing.T) {
	root := t.TempDir()
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_776_000_000, 0).UTC()
	eng := newEngineFixture(t, withRuntime(func(rt *Runtime) {
		rt.BaseDir = root
		rt.WebDir = webDir
		rt.WebURL = "https://iplists.firehol.org/ipsets/"
	}), withNow(func() time.Time {
		return now
	}))
	eng.cfg.Sources["geo"] = &config.Source{
		Name:   "geo",
		Use:    []string{config.UseGeoIP},
		Format: "dbip_country_csv",
	}
	eng.cfg.Sources["asn"] = &config.Source{
		Name:   "asn",
		Use:    []string{config.UseASN},
		Format: "iptoasn_combined_tsv",
	}
	eng.cfg.Sources["sample"] = &config.Source{
		Name:          "sample",
		Category:      "attacks",
		Frequency:     60,
		IPV:           "ipv4",
		Output:        "ip",
		Info:          "sample feed",
		Maintainer:    "Team One",
		MaintainerURL: "https://example.test/team-one",
	}
	entry := eng.state.Entry("sample")
	entry.Name = "sample"
	entry.Category = "attacks"
	entry.FrequencyMinutes = 60
	entry.Entries = 2
	entry.UniqueIPs = 10
	entry.SourceDate = now.Unix()
	entry.ProcessedDate = now.Unix()
	entry.CheckedDate = now.Unix()
	entry.StartedDate = now.Add(-time.Hour).Unix()
	entry.Maintainer = "Team One"
	entry.MaintainerURL = "https://example.test/team-one"

	if err := os.WriteFile(filepath.Join(webDir, "sample_geo.json"), []byte(`{"total_mapped":10,"countries":[{"code":"gr","value":4},{"code":"US","value":6}]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "sample_asn_asn.json"), []byte(`{"by_asn":[{"asn":13335,"name":"CLOUDFLARENET","count":6},{"asn":15169,"name":"GOOGLE","count":4}]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(webDir, "countries"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "countries", "index.json"), []byte(`{"countries":[{"code":"GR"},{"code":"US"}]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(webDir, "asns"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "asns", "index.json"), []byte(`{"asns":[{"asn":13335},{"asn":15169},{"asn":424242}]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	staleShard := filepath.Join(webDir, "sitemap-asns-0099.xml")
	if err := os.WriteFile(staleShard, []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, err := eng.writePublicMetadataFiles(webDir, []string{"sample"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staleShard); !os.IsNotExist(err) {
		t.Fatalf("expected stale sitemap shard to be removed, stat err=%v", err)
	}
	for _, name := range []string{
		"sitemap.xml",
		"sitemap-pages.xml",
		"sitemap-feeds.xml",
		"sitemap-countries.xml",
		"sitemap-maintainers.xml",
		"sitemap-asns-0001.xml",
		"robots.txt",
		"llms.txt",
	} {
		if !containsGeneratedFile(files, name) {
			t.Fatalf("generated file list missing %s: %v", name, files)
		}
		if _, err := os.Stat(filepath.Join(webDir, name)); err != nil {
			t.Fatalf("expected generated %s: %v", name, err)
		}
	}

	assertFileContains(t, filepath.Join(webDir, "sitemap.xml"),
		"<sitemapindex",
		"https://iplists.firehol.org/sitemap-pages.xml",
		"https://iplists.firehol.org/sitemap-feeds.xml",
		"https://iplists.firehol.org/sitemap-countries.xml",
		"https://iplists.firehol.org/sitemap-maintainers.xml",
		"https://iplists.firehol.org/sitemap-asns-0001.xml",
	)
	assertFileContains(t, filepath.Join(webDir, "sitemap-feeds.xml"), "https://iplists.firehol.org/ipsets/sample")
	assertFileContains(t, filepath.Join(webDir, "sitemap-countries.xml"),
		"https://iplists.firehol.org/countries/GR",
		"https://iplists.firehol.org/countries/US",
	)
	assertFileContains(t, filepath.Join(webDir, "sitemap-maintainers.xml"), "https://iplists.firehol.org/maintainers/team-one")
	assertFileContains(t, filepath.Join(webDir, "sitemap-asns-0001.xml"),
		"https://iplists.firehol.org/asns/13335",
		"https://iplists.firehol.org/asns/15169",
		"https://iplists.firehol.org/asns/424242",
	)
	assertFileContains(t, filepath.Join(webDir, "robots.txt"),
		"Disallow: /api/v1/search",
		"Disallow: /api/v1/query",
		"Disallow: /api/v1/compose",
		"Disallow: /api/v1/sets/*/search",
		"Disallow: /api/v1/ipsets/*/search",
	)
	assertGoldenFile(t, "robots.golden", filepath.Join(webDir, "robots.txt"))
	assertGoldenFile(t, "llms.golden", filepath.Join(webDir, "llms.txt"))
}

func containsGeneratedFile(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func assertFileContains(t *testing.T, path string, wants ...string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Fatalf("%s missing %q:\n%s", path, want, body)
		}
	}
}

func assertGoldenFile(t *testing.T, goldenName, actualPath string) {
	t.Helper()

	actual, err := os.ReadFile(actualPath)
	if err != nil {
		t.Fatalf("read actual artifact %s: %v", actualPath, err)
	}
	goldenPath := filepath.Join("testdata", goldenName)
	if *updateGoldenFiles {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o700); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, actual, 0o600); err != nil {
			t.Fatalf("update golden %s: %v", goldenPath, err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", goldenPath, err)
	}
	if !bytes.Equal(actual, want) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", goldenName, actual, want)
	}
}

func mustBitmapSet(t *testing.T, ranges ...iprange.Range) *iprange.IPSet {
	t.Helper()

	set := iprange.New("bitmap")
	for _, r := range ranges {
		if err := set.AddRange(r); err != nil {
			t.Fatal(err)
		}
	}
	set.Optimize()
	return set
}
