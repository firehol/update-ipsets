package engine

import (
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/enrichment"
)

func TestPublicFeedSummariesExposeEmbeddedEnrichment(t *testing.T) {
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", Enrichment: testEnrichment()}

	state := cache.New()
	state.ReplaceEntry("sample", cache.Entry{Name: "sample"})
	eng := newEngineFixture(t, withConfig(cfg), withState(state), withNow(func() time.Time {
		return time.Unix(1000, 0).UTC()
	}))

	summaries := summariesByName(eng.PublicFeedSummaries())
	summary := summaries["sample"]
	if summary.OfficialName != "Example Feed" {
		t.Fatalf("official_name = %q, want Example Feed", summary.OfficialName)
	}
	if summary.ShortDescription != "Short researched description." {
		t.Fatalf("short_description = %q", summary.ShortDescription)
	}
	if summary.CurrentStatusState != "active" {
		t.Fatalf("current_status_state = %q, want active", summary.CurrentStatusState)
	}
}

func TestPublicMetadataExposesEmbeddedEnrichment(t *testing.T) {
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", Enrichment: testEnrichment()}

	state := cache.New()
	state.ReplaceEntry("sample", cache.Entry{Name: "sample"})
	eng := newEngineFixture(t, withConfig(cfg), withState(state))

	payload, err := eng.MetadataWithEnableAll("sample", true)
	if err != nil {
		t.Fatalf("MetadataWithEnableAll: %v", err)
	}
	meta := payload.(setMetadata)
	if meta.Enrichment == nil {
		t.Fatal("metadata enrichment is nil")
	}
	if meta.CurrentStatus == nil || meta.CurrentStatus.State != "active" {
		t.Fatalf("metadata current_status = %#v, want active", meta.CurrentStatus)
	}
	if meta.OfficialName != "Example Feed" || meta.ShortDescription == "" {
		t.Fatalf("metadata display fields = official %q short %q", meta.OfficialName, meta.ShortDescription)
	}
	if got := enrichment.StringValue(meta.Enrichment.OfficialName); got != "Example Feed" {
		t.Fatalf("metadata enrichment official_name = %q", got)
	}
}

func testEnrichment() *enrichment.Feed {
	officialName := "Example Feed"
	shortDescription := "Short researched description."
	return &enrichment.Feed{
		EnrichmentSchemaVersion: enrichment.SchemaVersion,
		RunAt:                   "2026-05-26T00:00:00Z",
		OfficialName:            &officialName,
		ShortDescription:        &shortDescription,
		Derivation: enrichment.Derivation{
			Type:        "original",
			Description: "Original upstream feed.",
		},
		DetectionClassification: enrichment.DetectionClassification{
			PrimaryMethod: "unknown",
			Description:   "Not published.",
		},
		CurrentStatus: enrichment.CurrentStatus{
			State:       "active",
			Description: "Active.",
		},
		SourcesConsulted: []enrichment.SourceConsulted{
			{URL: "https://example.test/source"},
		},
	}
}
