package markdown_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/enrichment"
	"github.com/firehol/update-ipsets/pkg/markdown"
)

func TestFeedTemplateParses(t *testing.T) {
	t.Parallel()

	templateDir := filepath.Join("..", "..", "configs", "templates", "markdown")
	s := markdown.NewTemplateStore(templateDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	_, err := s.Execute("feed.md.tmpl", &markdown.FeedPageContext{
		Name:        "test-feed",
		Category:    "attacks",
		Maintainer:  "Test Maintainer",
		IPs:         1234567,
		Entries:     1234567,
		Frequency:   60,
		HealthClass: "healthy",
		Started:     1714646400000,
		Updated:     1714732800000,
		Format:      "ipset",
		IPV:         "4",
		Hash:        "abc123",
		Aggregation: 1,
		Info:        "This is a test feed for malicious IPs.",
	})
	if err != nil {
		t.Fatalf("Execute feed.md.tmpl: %v", err)
	}
}

func TestFeedTemplateWithAllSections(t *testing.T) {
	t.Parallel()

	templateDir := filepath.Join("..", "..", "configs", "templates", "markdown")
	s := markdown.NewTemplateStore(templateDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	ctx := &markdown.FeedPageContext{
		Name:             "dshield",
		Category:         "attacks",
		Maintainer:       "DShield",
		MaintainerURL:    "https://dshield.org",
		OfficialName:     "DShield Test Feed",
		ShortDescription: "Research-backed context for DShield.",
		IPs:              543210,
		Entries:          543210,
		Frequency:        10,
		HealthClass:      "healthy",
		Started:          1609459200000,
		Updated:          1714732800000,
		Output:           "ipset",
		IPV:              "4",
		Hash:             "deadbeef",
		Aggregation:      1,
		AvgUpdateMins:    15,
		MinUpdateMins:    5,
		MaxUpdateMins:    120,
		Source:           "https://dshield.org/block.txt",
		Info:             "Top attacking IPs from DShield honeypots.",
		Processor:        "generic",
		Downloader:       "http",
		License:          "CC-BY-SA",
		UsedFor:          []string{"attacks", "blocklist"},
		IPsMin:           400000,
		IPsMax:           600000,
		EntriesMin:       400000,
		EntriesMax:       600000,
		Insights: []markdown.InsightContext{
			{Code: "high_churn", Section: "trends", Headline: "Churn rate exceeds 40%"},
			{Code: "stale_entry", Section: "freshness", Headline: "Some IPs have been listed for >90 days"},
		},
		Critical: &markdown.CriticalContext{
			FeedIPs:     543210,
			CriticalIPs: 12345,
			Percent:     2.3,
			Complete:    true,
			Tiers: []markdown.CriticalTierContext{
				{Tier: "tier1", CriticalIPs: 5000, Percent: 1.0, Providers: 3},
			},
		},
		ASN: []markdown.ASNProviderContext{
			{
				Provider:      "geolite2",
				FeedIPs:       543210,
				AttributedIPs: 500000,
				BogonIPs:      10000,
				UnknownIPs:    33210,
				TopASNs: markdown.CappedRow{
					Rows: []markdown.CappedEntry{
						{Name: "AS13335 Cloudflare", Value: 50000},
						{Name: "AS15169 Google", Value: 30000},
					},
					Other: 420000,
				},
			},
		},
		GEO: []markdown.GEOProviderContext{
			{
				Provider:    "geolite2_country",
				TotalMapped: 500000,
				TopCountries: markdown.CappedRow{
					Rows: []markdown.CappedEntry{
						{Name: "United States (US)", Value: 200000},
						{Name: "China (CN)", Value: 100000},
					},
					Other: 200000,
				},
			},
		},
		Retention: &markdown.RetentionContext{
			CurrentRetentionDays: 1.5,
			PastRetentionDays:    2.5,
			CurrentSeries: []markdown.RetentionBucket{
				{Day: 1, Label: "1", IPs: 100},
				{Day: 366, Label: ">365 days", IPs: 5},
			},
			PastSeries: []markdown.RetentionBucket{
				{Day: 1, Label: "1", IPs: 200},
				{Day: 366, Label: ">365 days", IPs: 10},
			},
		},
		Comparison: []markdown.CompareRowContext{
			{Name: "alienvault", Category: "attacks", IPs: 300000, Common: 50000, ThisPercent: 50, TheirPercent: 16.6667},
			{Name: "blocklist-de", Category: "attacks", IPs: 200000, Common: 30000, ThisPercent: 30, TheirPercent: 15, Related: true},
		},
		Activity: &markdown.ActivityContext{
			Resolution:     "observed update",
			LatestIPs:      543210,
			RawChanges:     1,
			HistorySamples: 2,
			WindowNote:     "Rows describe content-changing updates in the published changeset artifact. The artifact is bounded to the retained public window, so it is not a full lifetime ledger and must not be used to infer first publication. Use Tracked since for monitoring start.",
			Rows: []markdown.ActivityRow{
				{Timestamp: time.Unix(1714732800, 0).UTC(), IPs: 543210, Entries: 543210, Added: 100, Removed: 50, ChurnPct: 0.03},
			},
		},
		Cadence:    []markdown.CadenceBin{{Interval: "<1h", Count: 5}},
		Enrichment: testFeedTemplateEnrichment(),
	}

	got, err := s.Execute("feed.md.tmpl", ctx)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(got) < 200 {
		t.Fatalf("output too short (%d bytes), template may not be rendering", len(got))
	}
	if !strings.Contains(got, "| Age (days) | Current IPs |") {
		t.Fatalf("rendered feed markdown missing daily retention table:\n%s", got)
	}
	if strings.Contains(got, "Age (hours)") {
		t.Fatalf("rendered feed markdown still contains hourly retention table:\n%s", got)
	}
	if !strings.Contains(got, "Days missing have zero IP count.") {
		t.Fatalf("rendered feed markdown missing zero-count day note:\n%s", got)
	}
	if !strings.Contains(got, "| >365 days | 5 |") {
		t.Fatalf("rendered feed markdown missing >365 days retention bucket:\n%s", got)
	}
	if !strings.Contains(got, "| Format | ipset |") {
		t.Fatalf("rendered feed markdown missing output-backed format row:\n%s", got)
	}
	if strings.Contains(got, "| Format |  |") {
		t.Fatalf("rendered feed markdown contains blank format row:\n%s", got)
	}
	if strings.Contains(got, "| Format | ipset |\n\n| IP Version |") {
		t.Fatalf("rendered feed markdown contains a blank line inside technical specs after format:\n%s", got)
	}
	if !strings.Contains(got, "| Avg Update | 15m |") || !strings.Contains(got, "| Min Update | 5m |") || !strings.Contains(got, "| Max Update | 2h |") {
		t.Fatalf("rendered feed markdown missing formatted update timing rows:\n%s", got)
	}
	if strings.Contains(got, "| Max Update | 2h |\n\n| Entries Range |") {
		t.Fatalf("rendered feed markdown contains a blank line inside technical specs after update timings:\n%s", got)
	}
	if !strings.Contains(got, "`This %` is the percentage of common IPs compared to the total IPs in this feed.") {
		t.Fatalf("rendered feed markdown missing overlap percentage explanation:\n%s", got)
	}
	if !strings.Contains(got, "| Feed | Category | IPs | Common | This % | Their % | Related |") {
		t.Fatalf("rendered feed markdown missing overlap percentage columns:\n%s", got)
	}
	if !strings.Contains(got, "| alienvault | attacks | 300,000 | 50,000 | 50.0% | 16.7% |") {
		t.Fatalf("rendered feed markdown missing formatted overlap percentages:\n%s", got)
	}
	if !strings.Contains(got, "## Behavior") {
		t.Fatalf("rendered feed markdown missing behavior section:\n%s", got)
	}
	if strings.Contains(got, "Initial publication") || strings.Contains(got, "post-baseline") {
		t.Fatalf("rendered feed markdown contains unsafe lifecycle wording:\n%s", got)
	}
	if !strings.Contains(got, "Resolution: **observed update**") {
		t.Fatalf("rendered feed markdown missing activity resolution:\n%s", got)
	}
	if !strings.Contains(got, "not a full lifetime ledger") || !strings.Contains(got, "Use **Tracked since** above for monitoring start") {
		t.Fatalf("rendered feed markdown missing bounded behavior explanation:\n%s", got)
	}
	if !strings.Contains(got, "Cadence is measured from published history timestamps.") {
		t.Fatalf("rendered feed markdown missing cadence explanation:\n%s", got)
	}
	if !strings.Contains(got, "DShield Test Feed") || !strings.Contains(got, "Research-backed context for DShield.") {
		t.Fatalf("rendered feed markdown missing enrichment title/summary:\n%s", got)
	}
	if !strings.Contains(got, "- `sensor_feed` (input) - Primary input.") {
		t.Fatalf("rendered feed markdown missing enrichment source feed cross-reference:\n%s", got)
	}
	// Sources consulted appears at the very end of the markdown as
	// auditable evidence behind the researched context above.
	if !strings.Contains(got, "## Sources consulted") {
		t.Fatalf("rendered feed markdown missing trailing sources_consulted section:\n%s", got)
	}
	if !strings.Contains(got, "Last researched: 2026-05-26T00:00:00Z") {
		t.Fatalf("rendered feed markdown missing enrichment research timestamp:\n%s", got)
	}
	// not_intended_for stays out of the public markdown surface.
	if strings.Contains(got, "Not intended for:") {
		t.Fatalf("rendered feed markdown should not include not_intended_for list:\n%s", got)
	}

	outPath := filepath.Join(t.TempDir(), "dshield.md")
	if err := os.WriteFile(outPath, []byte(got), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Logf("rendered %d bytes to %s", len(got), outPath)
	t.Log("first 500 chars:\n" + truncate(got, 500))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func strptr(s string) *string {
	return &s
}

func testFeedTemplateEnrichment() *enrichment.Feed {
	return &enrichment.Feed{
		RunAt:           "2026-05-26T00:00:00Z",
		LongDescription: strptr("Researched long description."),
		Derivation: enrichment.Derivation{
			Description: "Built from maintainer observations.",
			SourceFeeds: []enrichment.SourceFeed{
				{
					Identifier:   "sensor_feed",
					Relationship: strptr("input"),
					Notes:        strptr("Primary input."),
				},
			},
		},
		DetectionClassification: enrichment.DetectionClassification{
			PrimaryMethod: "passive_sensor",
			Description:   "Detection uses passive observations.",
		},
		ListingPolicy:   &enrichment.Policy{Summary: "Listed after observation."},
		UnlistingPolicy: &enrichment.Policy{Summary: "Removed after the signal stops."},
		CurrentStatus:   enrichment.CurrentStatus{State: "active", Description: "Active."},
	}
}
