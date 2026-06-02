package markdown_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/markdown"
)

func TestFeedArtifactReaderFiltersProvidersAndRollsUpRetention(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeJSON(t, dir, "sample.json", map[string]any{
		"name":              "sample",
		"category":          "intrusion",
		"entries":           36,
		"ips":               36,
		"output":            "ipset",
		"frequency":         1440,
		"average_update":    30,
		"min_update":        15,
		"max_update":        60,
		"official_name":     "Example Feed",
		"short_description": "Short researched description.",
		"current_status": map[string]any{
			"state":       "active",
			"description": "Active.",
		},
		"enrichment": map[string]any{
			"enrichment_schema_version": 2,
			"run_at":                    "2026-05-26T00:00:00Z",
			"official_name":             "Example Feed",
			"short_description":         "Short researched description.",
			"roles":                     []any{},
			"derivation": map[string]any{
				"type":         "original",
				"description":  "Original upstream feed.",
				"source_feeds": []any{},
			},
			"detection_classification": map[string]any{
				"primary_method":    "unknown",
				"secondary_methods": []any{},
				"description":       "Not published.",
			},
			"redistribution": map[string]any{},
			"current_status": map[string]any{
				"state":       "active",
				"description": "Active.",
			},
			"community":         map[string]any{},
			"sources_consulted": []any{map[string]any{"url": "https://example.test/source"}},
		},
		"geo": map[string]string{
			"dbip_country":     "sample_dbip_country.json",
			"geolite2_country": "sample_geolite2_country.json",
		},
	})
	writeJSON(t, dir, "sample_asn_iptoasn.json", asnPayload("iptoasn", 64500, "Example ASN", 21))
	writeJSON(t, dir, "sample_asn_caida_prefix2as.json", asnPayload("caida_prefix2as", 64501, "Other ASN", 15))
	writeJSON(t, dir, "sample_dbip_country.json", geoPayload("US", "United States", 30))
	writeJSON(t, dir, "sample_geolite2_country.json", geoPayload("GB", "United Kingdom", 6))
	writeJSON(t, dir, "sample_critical_infrastructure.json", map[string]any{
		"feed_ips":     36,
		"critical_ips": 2,
		"percent":      5.56,
		"complete":     true,
		"providers": []map[string]any{
			{
				"provider": map[string]any{
					"name":       "critical_context_github_hosted_compute",
					"label":      "GitHub hosted compute ranges",
					"maintainer": "GitHub",
				},
				"feed_ips":     36,
				"critical_ips": 2,
				"percent":      5.56,
			},
		},
	})
	writeJSON(t, dir, "sample_retention.json", map[string]any{
		"current": map[string]any{
			"hours": []int{0, 1, 23, 24, 25, 8759, 8760, 9000},
			"ips":   []int{1, 2, 3, 4, 5, 6, 7, 8},
		},
		"past": map[string]any{
			"hours": []int{47, 48, 8760},
			"ips":   []int{10, 11, 12},
		},
	})
	writeJSON(t, dir, "sample_comparison.json", []map[string]any{
		{
			"name":     "other_feed",
			"category": "intrusion",
			"ips":      72,
			"common":   18,
			"related":  true,
		},
	})
	writeText(t, dir, "sample_history.csv", "DateTime,Entries,UniqueIPs\n1000,30,30\n2000,36,36\n")
	writeText(t, dir, "sample_changesets.csv", "DateTime,AddedIPs,RemovedIPs\n2000,6,0\n")

	reader := markdown.NewFeedArtifactReader(
		dir,
		markdown.WithPreferredASNProvider("iptoasn"),
		markdown.WithPreferredGEOProvider("dbip_country"),
	)
	ctx, err := reader.BuildFeedContext("sample")
	if err != nil {
		t.Fatalf("BuildFeedContext: %v", err)
	}

	if len(ctx.ASN) != 1 || ctx.ASN[0].Provider != "iptoasn" {
		t.Fatalf("ASN providers = %#v, want only iptoasn", ctx.ASN)
	}
	if len(ctx.GEO) != 1 || ctx.GEO[0].Provider != "dbip_country" {
		t.Fatalf("GEO providers = %#v, want only dbip_country", ctx.GEO)
	}
	if len(ctx.Comparison) != 1 {
		t.Fatalf("comparison rows = %#v, want one row", ctx.Comparison)
	}
	if got := ctx.DisplayFormat(); got != "ipset" {
		t.Fatalf("display format = %q, want ipset", got)
	}
	if ctx.AvgUpdateMins != 30 || ctx.MinUpdateMins != 15 || ctx.MaxUpdateMins != 60 {
		t.Fatalf("update timings = avg %d min %d max %d, want 30/15/60", ctx.AvgUpdateMins, ctx.MinUpdateMins, ctx.MaxUpdateMins)
	}
	if ctx.OfficialName != "Example Feed" || ctx.ShortDescription != "Short researched description." {
		t.Fatalf("enrichment display fields = official %q short %q", ctx.OfficialName, ctx.ShortDescription)
	}
	if ctx.CurrentStatus == nil || ctx.CurrentStatus.State != "active" {
		t.Fatalf("current status = %#v, want active", ctx.CurrentStatus)
	}
	if ctx.Enrichment == nil || ctx.Enrichment.OfficialName == nil || *ctx.Enrichment.OfficialName != "Example Feed" {
		t.Fatalf("enrichment context = %#v, want Example Feed", ctx.Enrichment)
	}
	if got := ctx.Comparison[0].ThisPercent; got != 50 {
		t.Fatalf("this overlap percent = %.1f, want 50.0", got)
	}
	if got := ctx.Comparison[0].TheirPercent; got != 25 {
		t.Fatalf("their overlap percent = %.1f, want 25.0", got)
	}
	if ctx.Activity == nil || len(ctx.Activity.Rows) != 1 {
		t.Fatalf("activity rows = %#v, want one retained changeset row", ctx.Activity)
	}
	if ctx.Activity.Resolution != "observed update" {
		t.Fatalf("activity resolution = %q, want observed update", ctx.Activity.Resolution)
	}
	if ctx.Activity.HistorySamples != 2 || ctx.Activity.RawChanges != 1 {
		t.Fatalf("activity samples = history %d changes %d, want 2/1", ctx.Activity.HistorySamples, ctx.Activity.RawChanges)
	}

	if len(ctx.Critical.Providers) != 1 {
		t.Fatalf("critical providers = %#v, want one provider", ctx.Critical.Providers)
	}
	if got := ctx.Critical.Providers[0].Name; got != "GitHub hosted compute ranges" || strings.Contains(got, "map[") {
		t.Fatalf("critical provider name = %q, want readable label", got)
	}

	if ctx.Retention == nil {
		t.Fatal("Retention is nil")
	}
	current := ctx.Retention.CurrentSeries
	if len(current) != 4 {
		t.Fatalf("current retention buckets = %d, want only non-zero days plus >365 days", len(current))
	}
	assertRetentionBucket(t, current[0], "1", 6)
	assertRetentionBucket(t, current[1], "2", 9)
	assertRetentionBucket(t, current[2], "365", 6)
	assertRetentionBucket(t, current[3], ">365 days", 15)

	past := ctx.Retention.PastSeries
	if len(past) != 3 {
		t.Fatalf("past retention buckets = %d, want only non-zero days plus >365 days", len(past))
	}
	assertRetentionBucket(t, past[0], "2", 10)
	assertRetentionBucket(t, past[1], "3", 11)
	assertRetentionBucket(t, past[2], ">365 days", 12)
}

func asnPayload(provider string, asn int, name string, count int) map[string]any {
	return map[string]any{
		"provider":       provider,
		"feed_ips":       count,
		"attributed_ips": count,
		"bogon_ips":      0,
		"unknown_ips":    0,
		"by_asn": []map[string]any{
			{
				"asn":   asn,
				"name":  name,
				"count": count,
			},
		},
	}
}

func geoPayload(code, name string, count int) map[string]any {
	return map[string]any{
		"total_mapped": count,
		"countries": []map[string]any{
			{
				"code":  code,
				"name":  name,
				"value": count,
			},
		},
	}
}

func assertRetentionBucket(t *testing.T, bucket markdown.RetentionBucket, label string, ips uint64) {
	t.Helper()
	if bucket.Label != label || bucket.IPs != ips {
		t.Fatalf("retention bucket = %#v, want label %q and %d IPs", bucket, label, ips)
	}
}

func writeJSON(t *testing.T, dir, name string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", name, err)
	}
}

func writeText(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", name, err)
	}
}
