package engine

import (
	"encoding/json"
	"testing"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
)

func TestBuildSetMetadataConvertsMarkdownLinksAndUsesMillis(t *testing.T) {
	eng := newEngineFixture(t)
	eng.cfg.Sources["sample"] = &config.Source{Name: "sample", Output: "ipset"}
	entry := &cache.Entry{
		Info:             "see [docs](https://example.test/docs)",
		PublicURL:        "https://example.test/feed",
		File:             "sample.ipset",
		SourceDate:       1_774_997_253,
		ProcessedDate:    1_774_997_253,
		CheckedDate:      1_774_997_373,
		StartedDate:      1_774_995_588,
		FrequencyMinutes: 60,
	}
	eng.state.ReplaceEntry("sample", *entry)

	payload, err := eng.Metadata("sample")
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Info      string `json:"info"`
		Updated   int64  `json:"updated"`
		Processed int64  `json:"processed"`
		Checked   int64  `json:"checked"`
		Output    string `json:"output"`
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatal(err)
	}
	// Preserve bash-era markdown conversion, including the trailing space.
	if got, want := meta.Info, `see <a href="https://example.test/docs">docs</a> `; got != want {
		t.Fatalf("unexpected info html: got %q want %q", got, want)
	}
	if meta.Updated <= 1_000_000_000_000 || meta.Processed <= 1_000_000_000_000 || meta.Checked <= 1_000_000_000_000 {
		t.Fatalf("expected millisecond timestamps, got updated=%d processed=%d checked=%d", meta.Updated, meta.Processed, meta.Checked)
	}
	if got, want := meta.Output, "ipset"; got != want {
		t.Fatalf("metadata output = %q, want %q", got, want)
	}
}
