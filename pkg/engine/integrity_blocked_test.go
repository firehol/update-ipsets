package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestCheckIntegrityReportsBlockedMergeInputs(t *testing.T) {
	eng := newIntegrityTestEngine(t)
	eng.cfg.Sources["bogons"] = &config.Source{
		Name:      "bogons",
		URL:       "https://example.test/bogons.txt",
		Frequency: 60,
		IPV:       "ipv4",
		Output:    "netset",
		Use:       []string{config.UseBogons},
	}
	eng.cfg.Sources["cymru_unassigned"] = &config.Source{
		Name:         "cymru_unassigned",
		URL:          "internal://merge?exclude=bogons&inputs=fullbogons",
		Frequency:    1440,
		IPV:          "ipv4",
		Output:       "netset",
		Use:          []string{config.UseBogons},
		DerivedFrom:  []string{"fullbogons", "bogons"},
		MergeSources: []string{"fullbogons"},
		MergeExclude: []string{"bogons"},
		Provenance:   config.ProvenanceSecondaryMerge,
	}

	processedAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	writeFileForIntegrity(t, eng.feedBodyPath("fullbogons"), processedAt)
	writeFileForIntegrity(t, eng.feedBodyPath("cymru_unassigned"), processedAt)
	markProcessed(t, eng, "cymru_unassigned", processedAt)

	findings := eng.CheckIntegrityWithOptions(IntegrityOptions{EnableAll: true})
	if len(findings) != 1 {
		t.Fatalf("expected 1 blocked merge finding, got %d: %+v", len(findings), findings)
	}
	finding := findings[0]
	if finding.Feed != "cymru_unassigned" {
		t.Fatalf("finding feed = %q, want cymru_unassigned", finding.Feed)
	}
	if len(finding.BlockedFeeds) != 1 || finding.BlockedFeeds[0] != "bogons" {
		t.Fatalf("blocked feeds = %v, want [bogons]", finding.BlockedFeeds)
	}
	if !strings.Contains(finding.Reason, "blocked by unavailable input feeds") {
		t.Fatalf("reason = %q, want blocked reason", finding.Reason)
	}
	if !finding.ProcessedAt.Equal(processedAt) {
		t.Fatalf("processed_at = %s, want %s", finding.ProcessedAt, processedAt)
	}
	if !finding.SourceFileMTime.Equal(processedAt) {
		t.Fatalf("source_file_mtime = %s, want %s", finding.SourceFileMTime, processedAt)
	}
}
