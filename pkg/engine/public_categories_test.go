package engine

import (
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestPublicCategoriesExcludeConfiguredNonPublicCategories(t *testing.T) {
	cfg := config.New()
	cfg.Categories["intrusion"] = config.CategoryDefinition{
		Label:       "Intrusion",
		Description: "Public intrusion category.",
	}
	cfg.Categories["provider_support"] = config.CategoryDefinition{
		Label:       "Provider Support",
		Description: "Internal provider datasets.",
		Public:      boolPtr(false),
	}

	eng := newEngineFixture(t, withConfig(cfg))
	got := eng.PublicCategories()
	if len(got) != 1 {
		t.Fatalf("public categories = %d, want 1 (%+v)", len(got), got)
	}
	if got[0].Name != "intrusion" {
		t.Fatalf("public category = %q, want intrusion", got[0].Name)
	}
}

func TestHomeSummaryExcludesConfiguredNonPublicCategories(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	cfg := config.New()
	cfg.Runtime.FeedHealthSingleObservationGraceMins = 60
	cfg.Runtime.FeedHealthDefaultHealthyCadenceMins = 60
	cfg.Runtime.FeedHealthDefaultRiskyCadenceMins = 120
	cfg.Categories["intrusion"] = config.CategoryDefinition{
		Label:       "Intrusion",
		Description: "Public intrusion category.",
	}
	cfg.Categories["provider_support"] = config.CategoryDefinition{
		Label:       "Provider Support",
		Description: "Internal provider datasets.",
		Public:      boolPtr(false),
	}
	cfg.Sources["alpha"] = &config.Source{
		Name:          "alpha",
		URL:           "https://example.test/alpha.txt",
		Frequency:     60,
		IPV:           "ipv4",
		Output:        "ip",
		Category:      "intrusion",
		Info:          "alpha",
		Maintainer:    "Example",
		MaintainerURL: "https://example.test",
	}
	cfg.Sources["beta"] = &config.Source{
		Name:          "beta",
		URL:           "https://example.test/beta.txt",
		Frequency:     60,
		IPV:           "ipv4",
		Output:        "ip",
		Category:      "provider_support",
		Info:          "beta",
		Maintainer:    "Example",
		MaintainerURL: "https://example.test",
	}

	eng := newEngineFixture(t, withConfig(cfg), withNow(func() time.Time { return now }))
	for _, name := range []string{"alpha", "beta"} {
		entry := eng.state.Entry(name)
		entry.Name = name
		entry.Category = cfg.Sources[name].Category
		entry.FrequencyMinutes = 60
		entry.AverageUpdateMins = 60
		entry.MinUpdateMins = 60
		entry.MaxUpdateMins = 60
		entry.Version = 2
		entry.Entries = 10
		entry.UniqueIPs = 10
		entry.StartedDate = now.Add(-7 * 24 * time.Hour).Unix()
		entry.SourceDate = now.Add(-30 * time.Minute).Unix()
		entry.ProcessedDate = now.Add(-30 * time.Minute).Unix()
		entry.CheckedDate = now.Add(-20 * time.Minute).Unix()
		entry.Maintainer = "Example"
		entry.MaintainerURL = "https://example.test"
	}
	if _, err := eng.stageHomeAggregates(t.Context(), eng.outputDir(), eng.outputDir()); err != nil {
		t.Fatal(err)
	}

	payload, err := eng.HomeSummary(nil, 20)
	if err != nil {
		t.Fatal(err)
	}
	if payload.EligibleFeeds != 1 {
		t.Fatalf("eligible feeds = %d, want 1", payload.EligibleFeeds)
	}
	if payload.Totals.Feeds != 1 {
		t.Fatalf("totals.feeds = %d, want 1", payload.Totals.Feeds)
	}
	if payload.Totals.Categories != 1 {
		t.Fatalf("totals.categories = %d, want 1", payload.Totals.Categories)
	}

	filtered, err := eng.HomeSummary([]string{"provider_support"}, 20)
	if err != nil {
		t.Fatal(err)
	}
	if filtered.EligibleFeeds != 0 || filtered.Totals.Feeds != 0 || filtered.Totals.Categories != 0 {
		t.Fatalf(
			"non-public filter should contribute nothing, got eligible=%d totals=%d categories=%d",
			filtered.EligibleFeeds,
			filtered.Totals.Feeds,
			filtered.Totals.Categories,
		)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
