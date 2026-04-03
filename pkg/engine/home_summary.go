package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
)

// HomeSummaryProvider identifies a preferred backend provider used in
// the homepage summary aggregation. A public summary is honest about
// the provider it attributes its numbers to.
type HomeSummaryProvider struct {
	Name  string `json:"name,omitempty"`
	Label string `json:"label,omitempty"`
}

// HomeSummaryProviders bundles the geo and ASN providers chosen for
// the summary aggregation.
type HomeSummaryProviders struct {
	Geo HomeSummaryProvider `json:"geo"`
	ASN HomeSummaryProvider `json:"asn"`
}

// HomeSummaryTotals aggregates the cross-feed totals under the active
// category filter. UniqueIPs is a sum across feeds, not a deduplicated
// union — documented as such in the methodology page.
type HomeSummaryTotals struct {
	Feeds      int    `json:"feeds"`
	UniqueIPs  uint64 `json:"unique_ips"`
	Categories int    `json:"categories"`
}

// HomeSummaryCountry is one row in the top_countries aggregation.
type HomeSummaryCountry struct {
	Code          string `json:"code"`
	FeedCount     int    `json:"feed_count"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

// HomeSummaryASN is one row in the top_asns aggregation.
type HomeSummaryASN struct {
	ASN           uint32 `json:"asn"`
	Name          string `json:"name,omitempty"`
	FeedCount     int    `json:"feed_count"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

// HomeSummaryMaintainer is one row in the top_maintainers aggregation.
type HomeSummaryMaintainer struct {
	Slug              string         `json:"slug"`
	Name              string         `json:"name"`
	URL               string         `json:"url,omitempty"`
	FeedCount         int            `json:"feed_count"`
	UniqueIPs         uint64         `json:"unique_ips"`
	CategoryBreakdown map[string]int `json:"category_breakdown"`
}

// HomeSummaryPayload is the response shape for /api/v1/home/summary.
type HomeSummaryPayload struct {
	Categories        []string                `json:"categories"`
	EligibleFeeds     int                     `json:"eligible_feeds"`
	ContributingFeeds int                     `json:"contributing_feeds"`
	Totals            HomeSummaryTotals       `json:"totals"`
	Providers         HomeSummaryProviders    `json:"providers"`
	TopCountries      []HomeSummaryCountry    `json:"top_countries"`
	TopASNs           []HomeSummaryASN        `json:"top_asns"`
	TopMaintainers    []HomeSummaryMaintainer `json:"top_maintainers"`
}

// HomeSummary aggregates cross-feed totals and top-N rankings for the
// homepage surface under the given category filter. Only feeds in the
// selected categories with health in {healthy, delayed} and provenance in
// {primary, secondary_upstream} contribute to the aggregations. When
// the filter is empty, every public (non-system) category is included.
//
// The limit applies independently to top_countries, top_asns, and
// top_maintainers. A value of 0 falls back to the default of 20.
func (e *Engine) HomeSummary(categories []string, limit int) (*HomeSummaryPayload, error) {
	outputDir := ""
	if e != nil {
		outputDir = e.outputDir()
	}
	return e.HomeSummaryInDir(categories, limit, outputDir)
}

// HomeSummaryInDir composes homepage summary data from the precomputed
// homepage aggregate artifact in a specific published artifact directory.
// Public web handlers pass the resolved Options.WebDir so requests only read
// the artifact tree they serve.
func (e *Engine) HomeSummaryInDir(categories []string, limit int, outputDir string) (*HomeSummaryPayload, error) {
	if e == nil || e.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	start := time.Now()
	e.observeRunCounter("http.home_summary.requests", 1, 0)
	defer func() {
		e.observeRunOperation("http.home_summary.request", time.Since(start))
	}()
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	filter := normalizeCategoryFilter(categories)
	payload, err := e.loadHomeAggregatesInDir(outputDir)
	if err != nil {
		return nil, err
	}
	out := composeHomeSummaryFromAggregates(payload, filter, limit)
	e.observeRunCounter("http.home_summary.eligible_feeds", int64(out.EligibleFeeds), 0)
	e.observeRunCounter("http.home_summary.contributing_feeds", int64(out.ContributingFeeds), 0)
	return out, nil
}

// homeSummaryEligible applies the spec's aggregation filter policy:
// exclude system provider sources, hidden, and role-only sources;
// keep only primary + secondary_upstream provenance; require
// a category match when a filter is provided, otherwise accept any
// public category.
func homeSummaryEligible(cfg *config.Config, src *config.Source, filter map[string]struct{}) bool {
	if src == nil || src.Hidden {
		return false
	}
	if src.HasUse(config.UseGeoIP) || src.HasUse(config.UseASN) {
		return false
	}
	if cfg == nil || !cfg.CategoryIsPublic(src.Category) {
		return false
	}
	provenance := publicProvenance(src)
	if provenance != config.ProvenancePrimary && provenance != config.ProvenanceSecondaryUpstream {
		return false
	}
	if len(filter) == 0 {
		return true
	}
	_, ok := filter[src.Category]
	return ok
}

// topASNRow is the subset of asnFeedJSON needed for the summary
// aggregation. Keeps the summary decoupled from insight-specific
// sampling structs.
type topASNRow struct {
	ASN   uint32
	Name  string
	Count uint64
}

// maintainerSlugify produces a stable, URL-safe slug for a maintainer
// display name. Multiple feeds published by the same maintainer must
// agree on this slug so the homepage → maintainer detail pages line
// up. The rules are intentionally simple: lowercase, collapse
// non-alphanumeric runs to a single dash, strip leading/trailing
// dashes. Empty input becomes "unknown".
func maintainerSlugify(name string) string {
	trimmed := strings.TrimSpace(strings.ToLower(name))
	if trimmed == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(trimmed))
	lastDash := true
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	return out
}
