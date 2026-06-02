package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadYAMLReadsEmbeddedEnrichment(t *testing.T) {
	path := writeEnrichmentConfig(t, validEnrichmentConfig("sample"))

	cfg, err := LoadYAML(path)
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	src := cfg.Sources["sample"]
	if src == nil || src.Enrichment == nil {
		t.Fatalf("sample enrichment = %#v, want populated", src)
	}
	if got := *src.Enrichment.ShortDescription; got != "Short researched description." {
		t.Fatalf("short description = %q", got)
	}
	if got := src.Enrichment.CurrentStatus.State; got != "active" {
		t.Fatalf("current status = %q, want active", got)
	}
}

func TestLoadYAMLRejectsMalformedEmbeddedEnrichment(t *testing.T) {
	cfgText := strings.Replace(validEnrichmentConfig("sample"), "enrichment_schema_version: 2", "enrichment_schema_version: 1", 1)

	_, err := LoadYAML(writeEnrichmentConfig(t, cfgText))
	if err == nil {
		t.Fatal("LoadYAML succeeded with malformed enrichment, want error")
	}
	if !strings.Contains(err.Error(), `source "sample" enrichment: schema version = 1`) {
		t.Fatalf("LoadYAML error = %v", err)
	}
}

func TestLoadYAMLToleratesMissingEmbeddedEnrichment(t *testing.T) {
	path := writeEnrichmentConfig(t, `
sources:
  sample:
    url: https://example.test/list.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
    - passthrough
`)

	cfg, err := LoadYAML(path)
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if cfg.Sources["sample"].Enrichment != nil {
		t.Fatalf("sample enrichment = %#v, want nil", cfg.Sources["sample"].Enrichment)
	}
}

func TestLoadYAMLStripsInternalEmbeddedEnrichmentFields(t *testing.T) {
	cfgText := strings.Replace(
		validEnrichmentConfig("sample"),
		"      sources_consulted:",
		"      assistant_reasoning: internal note\n      maintainer_quotes:\n      - internal quote\n      sources_consulted:",
		1,
	)
	path := writeEnrichmentConfig(t, cfgText)

	cfg, err := LoadYAML(path)
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	data, err := json.Marshal(cfg.Sources["sample"].Enrichment)
	if err != nil {
		t.Fatalf("Marshal enrichment: %v", err)
	}
	if strings.Contains(string(data), "assistant_reasoning") || strings.Contains(string(data), "maintainer_quotes") {
		t.Fatalf("internal enrichment fields leaked into JSON: %s", data)
	}
}

func TestExpandDerivativesCopiesEmbeddedEnrichment(t *testing.T) {
	cfgText := strings.Replace(validEnrichmentConfig("sample"), "    output: ipset", "    output: ipset\n    history: [1440]", 1)

	cfg, err := LoadYAML(writeEnrichmentConfig(t, cfgText))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	parent := cfg.Sources["sample"]
	variant := cfg.Sources["sample_1d"]
	if parent == nil || parent.Enrichment == nil || variant == nil || variant.Enrichment == nil {
		t.Fatalf("parent=%#v variant=%#v, want enrichment on both", parent, variant)
	}
	if parent.Enrichment == variant.Enrichment {
		t.Fatal("retention variant shares enrichment pointer with parent, want clone")
	}
}

func TestExpandDerivativesCopiesMergeEmbeddedEnrichment(t *testing.T) {
	cfgText := `
sources:
  a:
    url: https://example.test/a.txt
    frequency: 60
    ipv: ipv4
    output: ipset
  b:
    url: https://example.test/b.txt
    frequency: 60
    ipv: ipv4
    output: ipset
merges:
  combo:
    frequency: 60
    ipv: ipv4
    output: ipset
    sources: [a, b]
` + validEnrichmentBlock()

	cfg, err := LoadYAML(writeEnrichmentConfig(t, cfgText))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if len(cfg.Merges) != 0 {
		t.Fatalf("merges = %d, want expanded", len(cfg.Merges))
	}
	src := cfg.Sources["combo"]
	if src == nil || src.Enrichment == nil {
		t.Fatalf("combo source = %#v, want copied enrichment", src)
	}
}

func writeEnrichmentConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func validEnrichmentConfig(name string) string {
	return `
sources:
  ` + name + `:
    url: https://example.test/list.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
    - passthrough
` + validEnrichmentBlock()
}

func validEnrichmentBlock() string {
	return `
    enrichment:
      enrichment_schema_version: 2
      run_at: "2026-05-26T00:00:00Z"
      official_name: Example Feed
      official_url: https://example.test/
      short_description: Short researched description.
      long_description: Longer researched description.
      roles: []
      derivation:
        type: original
        description: Original upstream feed.
        source_feeds: []
      listing_policy:
      unlisting_policy:
      unlist_request:
      update_frequency:
        frequency: 1d
        human_readable: Daily.
      detection_classification:
        primary_method: unknown
        secondary_methods: []
        description: The maintainer does not publish the detection method.
      scope_and_intent:
      license:
      redistribution: {}
      current_status:
        state: active
        description: Active.
      community:
        awards:
        criticism:
        engagement:
      sources_consulted:
      - url: https://example.test/source
        document_date: "2026"
        validation_date: "2026-05-26"
`
}
