package config

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestLoadFireholCatalogCounts verifies the full FireHOL directory catalog loads
// with the expected number of sources and merges. After source
// unification the geolocation/asn/bogons blocks are gone; the catalog
// now includes the current live feeds plus the historical feeds restored
// from synced bash production data. After the MISP feed audit and first
// critical-infrastructure references there are 341
// plain sources, 64 retention variants, 16 merges moved into Sources by
// ExpandDerivatives, and 2 hidden synthetic GeoIP-derived feeds.
func TestLoadFireholCatalogCounts(t *testing.T) {
	cfg := loadCatalog(t)

	// After ExpandDerivatives runs during LoadYAML, the in-memory
	// config contains every plain source PLUS the retention variants
	// that used to be sugar on the parent (`history: [...]`) PLUS
	// the merges that used to live in the top-level merges: block.
	// Current layout: 341 plain sources + 64 retention variants
	// + 16 merges moved in from the merges block + 2 hidden
	// synthetic GeoIP-derived feeds = 423 sources.
	// Recount from configs/firehol/
	// when this fails — the number drifts with catalog edits.
	if got := len(cfg.Sources); got != 423 {
		t.Fatalf("expected 423 sources after ExpandDerivatives and synthetic injection, got %d", got)
	}
	if got := len(cfg.Artifacts); got != 1 {
		t.Fatalf("expected 1 artifact after load, got %d", got)
	}
	// cfg.Merges is cleared by ExpandDerivatives; every merge now
	// exists as a Source entry with an internal://merge URL.
	if got := len(cfg.Merges); got != 0 {
		t.Fatalf("expected 0 merges (merges moved into Sources), got %d", got)
	}

	// Every source must have Name populated after load.
	for name, src := range cfg.Sources {
		if src.Name != name {
			t.Fatalf("source %q has mismatched Name field: %q", name, src.Name)
		}
	}
	for name, merge := range cfg.Merges {
		if merge.Name != name {
			t.Fatalf("merge %q has mismatched Name field: %q", name, merge.Name)
		}
	}
}

// TestLoadFireholCatalogAllSourcesHaveRequiredFields ensures every source
// in the FireHOL catalog that produces an ipset has the mandatory fields
// populated. ASN and GeoIP sources legitimately omit IPV/Output.
func TestLoadFireholCatalogAllSourcesHaveRequiredFields(t *testing.T) {
	cfg := loadCatalog(t)

	for name, src := range cfg.Sources {
		if src.HasUse(UseASN) || src.HasUse(UseGeoIP) {
			// Database sources do not produce ipsets and skip these checks.
			continue
		}
		if src.IPV == "" {
			t.Errorf("source %q missing ipv", name)
		}
		if src.Output == "" {
			t.Errorf("source %q missing output", name)
		}
		// Internal, static, and artifact-backed children have
		// frequency 0 because they have no direct download cadence.
		if strings.HasPrefix(src.URL, "internal://") || strings.HasPrefix(src.URL, ArtifactScheme+"://") || len(src.Static) > 0 {
			continue
		}
		if src.Frequency <= 0 {
			t.Errorf("source %q has invalid frequency %d", name, src.Frequency)
		}
	}
}

// TestValidateNilConfig verifies that Validate returns an error for nil.
func TestValidateNilConfig(t *testing.T) {
	if err := Validate(nil); err == nil {
		t.Fatal("expected error for nil config")
	}
}

// TestValidateNilSource checks that a nil source pointer is rejected.
func TestValidateNilSource(t *testing.T) {
	cfg := New()
	cfg.Sources["bad"] = nil
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for nil source")
	}
}

func TestLoadYAMLRejectsNilSourceWithoutPanic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("sources:\n  bad:\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadYAML(path)
	if err == nil || !strings.Contains(err.Error(), `source "bad" is nil`) {
		t.Fatalf("expected nil source validation error, got %v", err)
	}
}

func TestLoadYAMLRejectsNilMergeWithoutDropping(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("merges:\n  bad:\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadYAML(path)
	if err == nil || !strings.Contains(err.Error(), `merge "bad" is nil`) {
		t.Fatalf("expected nil merge validation error, got %v", err)
	}
}

func TestLoadDirectoryRejectsNilFragmentEntries(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "artifact",
			content: "artifacts:\n  bad:\n",
			want:    `artifact "bad" is nil`,
		},
		{
			name:    "source",
			content: "sources:\n  bad:\n",
			want:    `source "bad" is nil`,
		},
		{
			name:    "merge",
			content: "merges:\n  bad:\n",
			want:    `merge "bad" is nil`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "fragment.yaml"), []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadDirectory(dir)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q validation error, got %v", tt.want, err)
			}
		})
	}
}

// TestValidateNilMerge checks that a nil merge pointer is rejected.
func TestValidateNilMerge(t *testing.T) {
	cfg := New()
	cfg.Merges["bad"] = nil
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for nil merge")
	}
}

// TestValidateInvalidFrequency checks negative frequency.
func TestValidateInvalidFrequency(t *testing.T) {
	cfg := New()
	cfg.Sources["bad"] = &Source{
		Name:      "bad",
		Frequency: -1,
		IPV:       "ipv4",
		Output:    "ipset",
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for negative frequency")
	}
}

// TestValidateInvalidIPV checks unsupported IP version.
func TestValidateInvalidIPV(t *testing.T) {
	cfg := New()
	cfg.Sources["bad"] = &Source{
		Name:      "bad",
		Frequency: 10,
		IPV:       "ipv5",
		Output:    "ipset",
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for invalid IP version")
	}
}

// TestValidateInvalidOutput checks unsupported output type.
func TestValidateInvalidOutput(t *testing.T) {
	cfg := New()
	cfg.Sources["bad"] = &Source{
		Name:      "bad",
		Frequency: 10,
		IPV:       "ipv4",
		Output:    "bogus",
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for invalid output type")
	}
}

// TestValidateInvalidURL checks malformed URL.
func TestValidateInvalidURL(t *testing.T) {
	cfg := New()
	cfg.Sources["bad"] = &Source{
		Name:      "bad",
		Frequency: 10,
		IPV:       "ipv4",
		Output:    "ipset",
		URL:       "http://[",
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

// TestValidateMergeMissingOutput checks empty merge output.
func TestValidateMergeMissingOutput(t *testing.T) {
	cfg := New()
	cfg.Merges["bad"] = &Merge{
		Name:    "bad",
		IPV:     "ipv4",
		Output:  "",
		Sources: []string{"a"},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for empty merge output")
	}
}

// TestValidateMergeInvalidOutput checks unsupported merge output type.
func TestValidateMergeInvalidOutput(t *testing.T) {
	cfg := New()
	cfg.Merges["bad"] = &Merge{
		Name:    "bad",
		IPV:     "ipv4",
		Output:  "bogus",
		Sources: []string{"a"},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for invalid merge output type")
	}
}

// TestValidateMergeInvalidIPV checks merge with bad IP version.
func TestValidateMergeInvalidIPV(t *testing.T) {
	cfg := New()
	cfg.Merges["bad"] = &Merge{
		Name:    "bad",
		IPV:     "ipv9",
		Output:  "ipset",
		Sources: []string{"a"},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for invalid merge IP version")
	}
}

// TestValidateMergeNoSources checks merge with empty sources.
func TestValidateMergeNoSources(t *testing.T) {
	cfg := New()
	cfg.Merges["bad"] = &Merge{
		Name:    "bad",
		IPV:     "ipv4",
		Output:  "ipset",
		Sources: []string{},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for merge with no sources")
	}
}

func TestValidateRejectsDuplicateMergeExclude(t *testing.T) {
	cfg := New()
	cfg.Merges["bad"] = &Merge{
		Name:    "bad",
		IPV:     "ipv4",
		Output:  "ipset",
		Sources: []string{"a"},
		Exclude: []string{"b", "b"},
	}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "duplicate exclude") {
		t.Fatalf("expected duplicate exclude error, got %v", err)
	}
}

func TestValidateRejectsDuplicateMergeSources(t *testing.T) {
	cfg := New()
	cfg.Merges["bad"] = &Merge{
		Name:    "bad",
		IPV:     "ipv4",
		Output:  "ipset",
		Sources: []string{"a", "a"},
	}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "duplicate source") {
		t.Fatalf("expected duplicate source error, got %v", err)
	}
}

func TestValidateRejectsMergeSourceAlsoExcluded(t *testing.T) {
	cfg := New()
	cfg.Merges["bad"] = &Merge{
		Name:    "bad",
		IPV:     "ipv4",
		Output:  "ipset",
		Sources: []string{"a"},
		Exclude: []string{"a"},
	}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "both sources and exclude") {
		t.Fatalf("expected source/exclude overlap error, got %v", err)
	}
}

func TestValidateRejectsCommasInConfiguredNames(t *testing.T) {
	tests := []struct {
		name string
		cfg  func() *Config
	}{
		{
			name: "source",
			cfg: func() *Config {
				cfg := New()
				cfg.Sources["bad,name"] = &Source{Name: "bad,name", URL: "https://example.test/feed.txt", Frequency: 60, IPV: "ipv4", Output: "ipset"}
				return cfg
			},
		},
		{
			name: "merge",
			cfg: func() *Config {
				cfg := New()
				cfg.Merges["bad,name"] = &Merge{Name: "bad,name", Frequency: 60, IPV: "ipv4", Output: "ipset", Sources: []string{"a"}}
				return cfg
			},
		},
		{
			name: "artifact",
			cfg: func() *Config {
				cfg := New()
				cfg.Artifacts["bad,name"] = &Artifact{Name: "bad,name", Type: ArtifactTypeDroneBLBuildzone, Frequency: 60}
				return cfg
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.cfg()); err == nil || !strings.Contains(err.Error(), "commas") {
				t.Fatalf("expected comma validation error, got %v", err)
			}
		})
	}
}

func TestValidateRejectsMergeDatabaseUseRole(t *testing.T) {
	cfg := New()
	cfg.Merges["bad"] = &Merge{
		Name:    "bad",
		IPV:     "ipv4",
		Output:  "ipset",
		Sources: []string{"a"},
		Use:     []string{UseGeoIP},
	}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "unsupported use role") {
		t.Fatalf("expected unsupported merge use role error, got %v", err)
	}
}

// TestValidateValidConfig passes validation for a correct config.
func TestValidateValidConfig(t *testing.T) {
	cfg := New()
	cfg.Sources["good"] = &Source{
		Name:      "good",
		Frequency: 10,
		IPV:       "ipv4",
		Output:    "ipset",
		URL:       "https://example.test/feed.txt",
	}
	cfg.Merges["combo"] = &Merge{
		Name:    "combo",
		IPV:     "ipv4",
		Output:  "ipset",
		Sources: []string{"good"},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// TestSortedSourceNames verifies alphabetical ordering.
func TestSortedSourceNames(t *testing.T) {
	cfg := New()
	cfg.Sources["zebra"] = &Source{Name: "zebra", IPV: "ipv4", Output: "ipset"}
	cfg.Sources["alpha"] = &Source{Name: "alpha", IPV: "ipv4", Output: "ipset"}
	cfg.Sources["middle"] = &Source{Name: "middle", IPV: "ipv4", Output: "ipset"}

	names := SortedSourceNames(cfg)
	if !slices.IsSorted(names) {
		t.Fatalf("names not sorted: %v", names)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "middle" || names[2] != "zebra" {
		t.Fatalf("unexpected order: %v", names)
	}
}

// TestSortedMergeNames verifies alphabetical ordering.
func TestSortedMergeNames(t *testing.T) {
	cfg := New()
	cfg.Merges["z_merge"] = &Merge{Name: "z_merge", IPV: "ipv4", Output: "ipset", Sources: []string{"a"}}
	cfg.Merges["a_merge"] = &Merge{Name: "a_merge", IPV: "ipv4", Output: "ipset", Sources: []string{"b"}}

	names := SortedMergeNames(cfg)
	if !slices.IsSorted(names) {
		t.Fatalf("names not sorted: %v", names)
	}
	if len(names) != 2 || names[0] != "a_merge" || names[1] != "z_merge" {
		t.Fatalf("unexpected order: %v", names)
	}
}

// TestSortedSourceNamesEmpty verifies empty config produces empty slice.
func TestSortedSourceNamesEmpty(t *testing.T) {
	cfg := New()
	names := SortedSourceNames(cfg)
	if len(names) != 0 {
		t.Fatalf("expected empty slice, got %v", names)
	}
}

// TestSortedMergeNamesEmpty verifies empty config produces empty slice.
func TestSortedMergeNamesEmpty(t *testing.T) {
	cfg := New()
	names := SortedMergeNames(cfg)
	if len(names) != 0 {
		t.Fatalf("expected empty slice, got %v", names)
	}
}

// TestEmptyConfig verifies a config with no sources, merges, or geoloc is valid.
func TestEmptyConfig(t *testing.T) {
	cfg := New()
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected validation error for empty config: %v", err)
	}
}

// TestConfigOnlyMerges verifies a config with only merges (no sources) validates.
func TestConfigOnlyMerges(t *testing.T) {
	cfg := New()
	cfg.Merges["only_merge"] = &Merge{
		Name:    "only_merge",
		IPV:     "ipv4",
		Output:  "netset",
		Sources: []string{"external"},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// TestConfigOnlyGeoloc verifies a config with only geolocation
// (database) sources validates. After source unification, geo databases
// live in cfg.Sources with use:[geoip] and do not need IPV/Output.
func TestConfigOnlyGeoloc(t *testing.T) {
	cfg := New()
	cfg.Sources["test_geo"] = &Source{
		Name:      "test_geo",
		Frequency: 1440,
		URL:       "https://example.test/geo.csv",
		Use:       []string{UseGeoIP},
		Format:    "dbip_country_csv",
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// TestValidateRejectsUnknownUseRole catches typos in the use: field
// before the engine silently routes the source to the default ipset
// path.
func TestValidateRejectsUnknownUseRole(t *testing.T) {
	cfg := New()
	cfg.Sources["bad"] = &Source{
		Name:      "bad",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "ipset",
		Use:       []string{"bognos"}, // typo
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for unknown use role")
	}
}

// TestValidateRejectsASNWithoutFormat ensures asn sources cannot omit
// the format wire-format identifier.
func TestValidateRejectsASNWithoutFormat(t *testing.T) {
	cfg := New()
	cfg.Sources["bad"] = &Source{
		Name:      "bad",
		Frequency: 60,
		Use:       []string{UseASN},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for asn source without format")
	}
}

// TestValidateRejectsGeoIPWithoutFormat ensures geoip sources cannot
// omit the format wire-format identifier.
func TestValidateRejectsGeoIPWithoutFormat(t *testing.T) {
	cfg := New()
	cfg.Sources["bad"] = &Source{
		Name:      "bad",
		Frequency: 60,
		Use:       []string{UseGeoIP},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for geoip source without format")
	}
}

// TestValidateRejectsLegacyTopLevelBlocks verifies the migration error
// fires when an operator tries to load YAML that still has the old
// geolocation/asn/bogons top-level blocks.
func TestValidateRejectsLegacyTopLevelBlocks(t *testing.T) {
	for _, block := range []string{"geolocation", "asn", "bogons"} {
		yamlText := "sources: {}\n" + block + ":\n  foo:\n    type: bar\n"
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(path, []byte(yamlText), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadYAML(path); err == nil {
			t.Errorf("expected legacy block %q to be rejected", block)
		}
	}
}

func TestLoadYAMLRejectsUnknownMergeReferences(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "source",
			yaml: `
sources:
  real:
    url: https://example.test/real.txt
    frequency: 60
    ipv: ipv4
    output: ipset
merges:
  bad:
    frequency: 60
    ipv: ipv4
    output: ipset
    sources: [missing]
`,
		},
		{
			name: "exclude",
			yaml: `
sources:
  real:
    url: https://example.test/real.txt
    frequency: 60
    ipv: ipv4
    output: ipset
merges:
  bad:
    frequency: 60
    ipv: ipv4
    output: ipset
    sources: [real]
    exclude: [missing]
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadYAML(path); err == nil {
				t.Fatal("expected LoadYAML to reject unknown merge reference")
			}
		})
	}
}

func TestLoadYAMLRejectsMergeWithoutAdditiveSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := `
sources:
  real:
    url: https://example.test/real.txt
    frequency: 60
    ipv: ipv4
    output: ipset
merges:
  bad:
    frequency: 60
    ipv: ipv4
    output: ipset
    exclude: [real]
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadYAML(path); err == nil || !strings.Contains(err.Error(), `merge "bad" has no sources`) {
		t.Fatalf("expected merge with no sources to be rejected, got %v", err)
	}
}

func TestExpandDerivativesRejectsMergeWithoutAdditiveSources(t *testing.T) {
	cfg := New()
	cfg.Sources["real"] = &Source{Name: "real", Frequency: 60, IPV: "ipv4", Output: "ipset"}
	cfg.Merges["bad"] = &Merge{
		Name:    "bad",
		IPV:     "ipv4",
		Output:  "ipset",
		Exclude: []string{"real"},
	}
	if err := ExpandDerivatives(cfg); err == nil || !strings.Contains(err.Error(), `expand: merge "bad" has no sources`) {
		t.Fatalf("expected ExpandDerivatives to reject merge with no sources, got %v", err)
	}
}

// TestSourceHasUseAndSortedSourceNamesWithUse exercises the helpers
// the engine uses to find sources by role.
func TestSourceHasUseAndSortedSourceNamesWithUse(t *testing.T) {
	cfg := New()
	cfg.Sources["a"] = &Source{Name: "a", Frequency: 1, IPV: "ipv4", Output: "ipset"}
	cfg.Sources["b"] = &Source{Name: "b", Frequency: 1, Use: []string{UseASN}, Format: "iptoasn_combined_tsv"}
	cfg.Sources["c"] = &Source{Name: "c", Frequency: 1, Use: []string{UseBogons}, IPV: "ipv4", Output: "netset"}
	cfg.Sources["d"] = &Source{Name: "d", Frequency: 1, Use: []string{UseGeoIP, UseBogons}, IPV: "ipv4", Output: "netset", Format: "dbip_country_csv"}

	if got := cfg.SortedSourceNamesWithUse(UseASN); len(got) != 1 || got[0] != "b" {
		t.Errorf("UseASN: got %v", got)
	}
	if got := cfg.SortedSourceNamesWithUse(UseBogons); len(got) != 2 || got[0] != "c" || got[1] != "d" {
		t.Errorf("UseBogons: got %v", got)
	}
	if got := cfg.SortedSourceNamesWithUse(UseGeoIP); len(got) != 1 || got[0] != "d" {
		t.Errorf("UseGeoIP: got %v", got)
	}
	if cfg.Sources["a"].HasUse(UseASN) {
		t.Error("plain ipset must not report any role")
	}
	if !cfg.Sources["d"].HasUse(UseBogons) || !cfg.Sources["d"].HasUse(UseGeoIP) {
		t.Error("multi-role source must report all of its roles")
	}
}

// TestRuntimeTemplateExpansionInFireholCatalog confirms the FireHOL catalog
// runtime section uses shell-style variable templates.
func TestRuntimeTemplateExpansionInFireholCatalog(t *testing.T) {
	cfg := loadCatalog(t)

	// The raw runtime fields should contain ${...} templates.
	if cfg.Runtime.BaseDir == "" {
		t.Fatal("expected non-empty base_dir template")
	}
	if cfg.Runtime.HistoryDir == "" {
		t.Fatal("expected non-empty history_dir template")
	}
}

// TestProcessorStepMarshalEmpty verifies error on empty name.
func TestProcessorStepMarshalEmpty(t *testing.T) {
	p := ProcessorStep{}
	_, err := p.MarshalYAML()
	if err == nil {
		t.Fatal("expected error for empty processor step name")
	}
}

// TestProcessorStepUnmarshalRoundTrip tests YAML encode/decode of processor steps.
func TestProcessorStepUnmarshalRoundTrip(t *testing.T) {
	cfg := New()
	cfg.Sources["test"] = &Source{
		Name:      "test",
		Frequency: 10,
		IPV:       "ipv4",
		Output:    "ipset",
		Processor: []ProcessorStep{
			{Name: "remove_comments"},
			{Name: "csv_column", Args: map[string]string{"index": "2"}},
		},
	}

	var buf bytes.Buffer
	if err := SaveYAML(&buf, cfg); err != nil {
		t.Fatal(err)
	}

	tmp := filepath.Join(t.TempDir(), "roundtrip.yaml")
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadYAML(tmp)
	if err != nil {
		t.Fatal(err)
	}

	procs := loaded.Sources["test"].Processor
	if len(procs) != 2 {
		t.Fatalf("expected 2 processors, got %d", len(procs))
	}
	if procs[0].Name != "remove_comments" {
		t.Fatalf("unexpected first processor: %q", procs[0].Name)
	}
	if procs[0].Args != nil {
		t.Fatalf("expected nil args for simple processor, got %v", procs[0].Args)
	}
	if procs[1].Name != "csv_column" {
		t.Fatalf("unexpected second processor: %q", procs[1].Name)
	}
	if procs[1].Args["index"] != "2" {
		t.Fatalf("unexpected csv_column index: %q", procs[1].Args["index"])
	}
}

// TestLoadUnsupportedFormat verifies the Load function rejects unknown extensions.
func TestLoadUnsupportedFormat(t *testing.T) {
	_, err := Load("/tmp/config.toml")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

// TestMergeConfigs verifies Config.Merge correctly combines two configs.
func TestMergeConfigs(t *testing.T) {
	base := New()
	base.Sources["a"] = &Source{Name: "a", IPV: "ipv4", Output: "ipset", Frequency: 10}

	other := New()
	other.Sources["b"] = &Source{Name: "b", IPV: "ipv4", Output: "netset", Frequency: 20}
	other.Merges["combo"] = &Merge{Name: "combo", IPV: "ipv4", Output: "netset", Sources: []string{"a", "b"}}
	other.Renames["old"] = "new"
	other.Deleted = []string{"deprecated"}

	base.Merge(other)

	if _, ok := base.Sources["a"]; !ok {
		t.Fatal("expected source 'a' to remain")
	}
	if _, ok := base.Sources["b"]; !ok {
		t.Fatal("expected source 'b' to be merged in")
	}
	if _, ok := base.Merges["combo"]; !ok {
		t.Fatal("expected merge 'combo' to be merged in")
	}
	if base.Renames["old"] != "new" {
		t.Fatalf("expected rename old->new, got %v", base.Renames)
	}
	if len(base.Deleted) != 1 || base.Deleted[0] != "deprecated" {
		t.Fatalf("expected deleted list, got %v", base.Deleted)
	}
}

// TestMergeNilOther verifies no panic on nil merge.
func TestMergeNilOther(t *testing.T) {
	base := New()
	base.Merge(nil) // should not panic
}

// TestMergePreservesNilEntriesForValidation verifies nil entries survive
// fragment merging so the final validation phase can reject malformed YAML.
func TestMergePreservesNilEntriesForValidation(t *testing.T) {
	base := New()
	other := New()
	other.Artifacts["nil_artifact"] = nil
	other.Sources["nil_src"] = nil
	other.Merges["nil_merge"] = nil

	base.Merge(other) // should not panic

	if _, ok := base.Artifacts["nil_artifact"]; !ok {
		t.Fatal("nil artifact should be preserved for validation")
	}
	if _, ok := base.Sources["nil_src"]; !ok {
		t.Fatal("nil source should be preserved for validation")
	}
	if _, ok := base.Merges["nil_merge"]; !ok {
		t.Fatal("nil merge should be preserved for validation")
	}
	if err := Validate(base); err == nil {
		t.Fatal("expected validation to reject preserved nil entries")
	}
}

// TestLoadDirectoryEmptyString returns empty config.
func TestLoadDirectoryEmptyString(t *testing.T) {
	cfg, err := LoadDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sources) != 0 {
		t.Fatalf("expected no sources for empty dir, got %d", len(cfg.Sources))
	}
}

// TestLoadDirectoryNonExistent returns empty config for missing dir.
func TestLoadDirectoryNonExistent(t *testing.T) {
	cfg, err := LoadDirectory("/tmp/does-not-exist-config-dir-12345")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sources) != 0 {
		t.Fatalf("expected no sources for missing dir, got %d", len(cfg.Sources))
	}
}

// TestLoadDirectoryRecursesSubdirectories verifies directory catalogs may
// organize per-feed files below nested folders.
func TestLoadDirectoryRecursesSubdirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sources", "policy_risk"), 0o700); err != nil {
		t.Fatal(err)
	}
	categories := []byte(`
categories:
  policy_risk:
    label: Policy / Risk
    description: test
    color: "#000000"
    sort_order: 1
`)
	source := []byte(`
sources:
  nested:
    url: https://example.test/nested.txt
    frequency: 10
    ipv: ipv4
    output: ipset
    category: policy_risk
`)
	if err := os.WriteFile(filepath.Join(dir, "categories.yaml"), categories, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sources", "policy_risk", "nested.yaml"), source, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Sources["nested"]; !ok {
		t.Fatalf("expected nested source to load from subdirectory")
	}
}

// TestValidateAllOutputTypes verifies the two authoritative
// output values ("ipset", "netset") pass Validate. Legacy
// aliases ("ip", "net", "both") are handled by
// canonicalizeOutput at LoadYAML time — this test exercises
// Validate directly, so it uses the post-canonicalisation
// values.
func TestValidateAllOutputTypes(t *testing.T) {
	for _, output := range []string{"ipset", "netset"} {
		cfg := New()
		cfg.Sources["test"] = &Source{
			Name:      "test",
			Frequency: 10,
			IPV:       "ipv4",
			Output:    output,
		}
		if err := Validate(cfg); err != nil {
			t.Errorf("output %q should be valid, got error: %v", output, err)
		}
	}
}

// TestCanonicalizeOutput pins the legacy-alias translation
// rules. If we ever drop one of the aliases we want to break
// this test first, not discover the regression at runtime when
// an old YAML deployment fails to load.
func TestCanonicalizeOutput(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"ipset", "ipset"},
		{"netset", "netset"},
		{"ip", "ipset"},
		{"ips", "ipset"},
		{"net", "netset"},
		{"nets", "netset"},
		{"both", "netset"},
		{"all", "netset"},
		{"", ""},               // empty stays empty — validator will catch it
		{"garbage", "garbage"}, // unknown stays unknown — validator rejects it
	}
	for _, c := range cases {
		if got := canonicalizeOutput(c.in); got != c.want {
			t.Errorf("canonicalizeOutput(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestValidateIPV6Source verifies ipv6 sources pass validation.
func TestValidateIPV6Source(t *testing.T) {
	cfg := New()
	cfg.Sources["v6"] = &Source{
		Name:      "v6",
		Frequency: 10,
		IPV:       "ipv6",
		Output:    "ipset",
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("ipv6 source should be valid: %v", err)
	}
}
