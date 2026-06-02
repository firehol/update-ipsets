package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDirectoryExpandsHistoryOnMerge(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`
sources:
  first:
    url: https://example.test/one.txt
    frequency: 10
    ipv: ipv4
    output: ipset
  second:
    url: https://example.test/two.txt
    frequency: 10
    ipv: ipv4
    output: ipset
merges:
  combined:
    frequency: 15
    history: [1440]
    ipv: ipv4
    output: ipset
    sources: [first, second]
`)
	if err := os.WriteFile(filepath.Join(dir, "merge-history.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	parent := cfg.Sources["combined"]
	if parent == nil {
		t.Fatal("expected combined merge source")
	}
	if parent.Provenance != ProvenanceSecondaryMerge {
		t.Fatalf("combined provenance = %q, want %q", parent.Provenance, ProvenanceSecondaryMerge)
	}
	if got, want := parent.MergeSources, []string{"first", "second"}; !equalStrings(got, want) {
		t.Fatalf("combined merge sources = %v, want %v", got, want)
	}
	if len(parent.History) != 0 {
		t.Fatalf("combined history sugar was not consumed: %v", parent.History)
	}

	child := cfg.Sources["combined_1d"]
	if child == nil {
		t.Fatal("expected combined_1d history derivative")
	}
	if child.Provenance != ProvenanceSecondaryRetention {
		t.Fatalf("combined_1d provenance = %q, want %q", child.Provenance, ProvenanceSecondaryRetention)
	}
	if got, want := child.DerivedFrom, []string{"combined"}; !equalStrings(got, want) {
		t.Fatalf("combined_1d derived_from = %v, want %v", got, want)
	}
	if len(child.MergeSources) != 0 || len(child.MergeExclude) != 0 {
		t.Fatalf("history derivative should not retain signed merge lists: sources=%v exclude=%v", child.MergeSources, child.MergeExclude)
	}
	if !strings.HasPrefix(child.URL, InternalRetentionWindowScheme) {
		t.Fatalf("combined_1d URL = %q, want retention-window URL", child.URL)
	}
}
