package engine

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

// ErrMaintainerNotFound reports that no public eligible feed exists for a
// maintainer slug.
var ErrMaintainerNotFound = errors.New("maintainer not found")

// CountryDetailFeed is one row in the country detail payload.
type CountryDetailFeed struct {
	Name          string `json:"name"`
	Category      string `json:"category"`
	Provenance    string `json:"provenance,omitempty"`
	Maintainer    string `json:"maintainer,omitempty"`
	AttributedIPs uint64 `json:"attributed_ips"`
	UniqueIPs     uint64 `json:"unique_ips"`
	HealthClass   string `json:"health_class"`
	LastChangeTS  int64  `json:"last_change_ts,omitempty"`
}

// CountryDetailASN is one row in the country's top-ASNs block.
type CountryDetailASN struct {
	ASN           uint32 `json:"asn"`
	Name          string `json:"name,omitempty"`
	FeedCount     int    `json:"feed_count"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

// DetailCategorySummary is one grouped composition row for a detail page.
type DetailCategorySummary struct {
	Category      string `json:"category"`
	FeedCount     int    `json:"feed_count"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

// DetailMaintainerSummary is one grouped maintainer row for a detail page.
type DetailMaintainerSummary struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	URL           string `json:"url,omitempty"`
	FeedCount     int    `json:"feed_count"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

// CountryDetailTotals aggregates cross-feed totals for the country.
type CountryDetailTotals struct {
	FeedsMatching       int    `json:"feeds_matching"`
	AttributedIPsInFeed uint64 `json:"attributed_ips_in_feeds"`
	Categories          int    `json:"categories"`
	Maintainers         int    `json:"maintainers"`
	ASNs                int    `json:"asns"`
}

// CountryDetailPayload is the response shape for /api/v1/countries/{code}.
type CountryDetailPayload struct {
	Code            string                         `json:"code"`
	Provider        HomeSummaryProvider            `json:"provider"`
	Totals          CountryDetailTotals            `json:"totals"`
	Feeds           []CountryDetailFeed            `json:"feeds"`
	FeedsByCategory map[string][]CountryDetailFeed `json:"feeds_by_category,omitempty"`
	TopCategories   []DetailCategorySummary        `json:"top_categories,omitempty"`
	TopMaintainers  []DetailMaintainerSummary      `json:"top_maintainers,omitempty"`
	TopASNs         []CountryDetailASN             `json:"top_asns_in_country,omitempty"`
	ASNProvider     HomeSummaryProvider            `json:"asn_provider,omitempty"`
}

// ASNDetailFeed is one row in the ASN detail payload.
type ASNDetailFeed struct {
	Name          string `json:"name"`
	Category      string `json:"category"`
	Provenance    string `json:"provenance,omitempty"`
	Maintainer    string `json:"maintainer,omitempty"`
	AttributedIPs uint64 `json:"attributed_ips"`
	UniqueIPs     uint64 `json:"unique_ips"`
	HealthClass   string `json:"health_class"`
	LastChangeTS  int64  `json:"last_change_ts,omitempty"`
}

// ASNDetailTotals aggregates cross-feed totals for this ASN.
type ASNDetailTotals struct {
	FeedsMatching int    `json:"feeds_matching"`
	AttributedIPs uint64 `json:"attributed_ips"`
	Categories    int    `json:"categories"`
	Maintainers   int    `json:"maintainers"`
	Countries     int    `json:"countries"`
}

// ASNDetailCountry is one country row in an ASN detail payload.
type ASNDetailCountry struct {
	Code          string `json:"code"`
	FeedCount     int    `json:"feed_count"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

// ASNDetailPayload is the response shape for /api/v1/asns/{asn}.
type ASNDetailPayload struct {
	ASN                 uint32                     `json:"asn"`
	Name                string                     `json:"name,omitempty"`
	Description         string                     `json:"description,omitempty"`
	Provider            HomeSummaryProvider        `json:"provider"`
	GeoProvider         HomeSummaryProvider        `json:"geo_provider,omitempty"`
	Totals              ASNDetailTotals            `json:"totals"`
	Feeds               []ASNDetailFeed            `json:"feeds"`
	FeedsByCategory     map[string][]ASNDetailFeed `json:"feeds_by_category,omitempty"`
	TopCategories       []DetailCategorySummary    `json:"top_categories,omitempty"`
	TopMaintainers      []DetailMaintainerSummary  `json:"top_maintainers,omitempty"`
	TopCountries        []ASNDetailCountry         `json:"top_countries,omitempty"`
	CountryDistribution *CountryComparisonPayload  `json:"country_distribution,omitempty"`
}

// MaintainerDetailFeed is one row in the maintainer detail payload.
type MaintainerDetailFeed struct {
	Name         string `json:"name"`
	Category     string `json:"category"`
	Provenance   string `json:"provenance,omitempty"`
	UniqueIPs    uint64 `json:"unique_ips"`
	HealthClass  string `json:"health_class"`
	LastChangeTS int64  `json:"last_change_ts,omitempty"`
}

// MaintainerDetailTotals aggregates the maintainer's cross-feed totals.
type MaintainerDetailTotals struct {
	Feeds      int    `json:"feeds"`
	UniqueIPs  uint64 `json:"unique_ips"`
	Categories int    `json:"categories"`
}

// MaintainerDetailPayload is the response shape for
// /api/v1/maintainers/{slug}.
type MaintainerDetailPayload struct {
	Slug            string                            `json:"slug"`
	Name            string                            `json:"name"`
	URL             string                            `json:"url,omitempty"`
	Totals          MaintainerDetailTotals            `json:"totals"`
	FeedsByCategory map[string][]MaintainerDetailFeed `json:"feeds_by_category"`
}

// MaintainerDetail returns every feed a maintainer publishes, grouped
// by category. The slug identity is computed by maintainerSlugify.
// Rich maintainer records (description, history, credibility) are
// not yet part of this payload — they land with a future maintainer
// registry spec.
func (e *Engine) MaintainerDetail(slug string) (*MaintainerDetailPayload, error) {
	return e.MaintainerDetailWithSnapshot(e.operationSnapshot(), slug)
}

func (e *Engine) MaintainerDetailWithSnapshot(snap operationSnapshot, slug string) (*MaintainerDetailPayload, error) {
	if e == nil || snap.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	normalized := strings.TrimSpace(slug)
	if normalized == "" {
		return nil, fmt.Errorf("maintainer slug is required")
	}
	now := e.now().UTC()

	grouped := map[string][]MaintainerDetailFeed{}
	categories := map[string]struct{}{}
	var canonicalName, canonicalURL string
	var totalIPs uint64
	feedCount := 0

	for _, entry := range e.entriesSnapshot(snap.cfg, configuredNamesForConfig(snap.cfg)) {
		src := lookupSourceForConfig(snap.cfg, entry.Name)
		if !homeSummaryEligible(snap.cfg, src, nil) {
			continue
		}
		health := feedhealth.Classify(&entry, src, snap.feedHealthPolicy, now)
		if !homeGlobeHealthEligible(health.Class) {
			continue
		}
		maintainerName := strings.TrimSpace(entry.Maintainer)
		if maintainerName == "" {
			continue
		}
		if maintainerSlugify(maintainerName) != normalized {
			continue
		}
		if canonicalName == "" {
			canonicalName = maintainerName
		}
		if canonicalURL == "" && entry.MaintainerURL != "" {
			canonicalURL = entry.MaintainerURL
		}

		category := src.Category
		categories[category] = struct{}{}
		feedCount++
		totalIPs += entry.UniqueIPs

		grouped[category] = append(grouped[category], MaintainerDetailFeed{
			Name:         entry.Name,
			Category:     category,
			Provenance:   string(publicProvenance(src)),
			UniqueIPs:    entry.UniqueIPs,
			HealthClass:  string(health.Class),
			LastChangeTS: entry.SourceDate,
		})
	}

	if canonicalName == "" {
		return nil, ErrMaintainerNotFound
	}

	for category, rows := range grouped {
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].UniqueIPs > rows[j].UniqueIPs
		})
		grouped[category] = rows
	}

	return &MaintainerDetailPayload{
		Slug: normalized,
		Name: canonicalName,
		URL:  canonicalURL,
		Totals: MaintainerDetailTotals{
			Feeds:      feedCount,
			UniqueIPs:  totalIPs,
			Categories: len(categories),
		},
		FeedsByCategory: grouped,
	}, nil
}

// MaintainerIndexEntry is one row in the maintainers index.
type MaintainerIndexEntry struct {
	Slug       string   `json:"slug"`
	Name       string   `json:"name"`
	URL        string   `json:"url,omitempty"`
	FeedCount  int      `json:"feed_count"`
	UniqueIPs  uint64   `json:"unique_ips"`
	Categories []string `json:"categories"`
}

// MaintainerIndexPayload is the response shape for /api/v1/maintainers.
type MaintainerIndexPayload struct {
	Maintainers []MaintainerIndexEntry `json:"maintainers"`
}

// MaintainerIndex returns a sorted list of every maintainer tracked
// by the catalog. Two feeds published under the same display name
// share a slug and appear as one row. System-role sources (ASN,
// geolocation, bogon-only) are excluded. The optional category filter
// narrows the list to maintainers publishing at least one feed in the
// selected categories.
func (e *Engine) MaintainerIndex(categories []string) (*MaintainerIndexPayload, error) {
	cfg, _, policy := e.configRuntimePolicySnapshot()
	return e.maintainerIndexWithConfigPolicy(cfg, policy, categories)
}

func (e *Engine) MaintainerIndexWithSnapshot(snap operationSnapshot, categories []string) (*MaintainerIndexPayload, error) {
	return e.maintainerIndexWithConfigPolicy(snap.cfg, snap.feedHealthPolicy, categories)
}

func (e *Engine) maintainerIndexWithConfigPolicy(cfg *config.Config, policy feedhealth.Policy, categories []string) (*MaintainerIndexPayload, error) {
	if e == nil || cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	filter := normalizeCategoryFilter(categories)
	now := e.now().UTC()

	type maintainerRow struct {
		slug       string
		name       string
		url        string
		feedCount  int
		uniqueIPs  uint64
		categories map[string]struct{}
	}
	rows := map[string]*maintainerRow{}

	for _, entry := range e.entriesSnapshot(cfg, configuredNamesForConfig(cfg)) {
		src := lookupSourceForConfig(cfg, entry.Name)
		if !homeSummaryEligible(cfg, src, nil) {
			continue
		}
		health := feedhealth.Classify(&entry, src, policy, now)
		if !homeGlobeHealthEligible(health.Class) {
			continue
		}
		if len(filter) > 0 {
			if _, ok := filter[src.Category]; !ok {
				continue
			}
		}
		maintainerName := strings.TrimSpace(entry.Maintainer)
		if maintainerName == "" {
			continue
		}
		slug := maintainerSlugify(maintainerName)
		row := rows[slug]
		if row == nil {
			row = &maintainerRow{
				slug:       slug,
				name:       maintainerName,
				url:        entry.MaintainerURL,
				categories: map[string]struct{}{},
			}
			rows[slug] = row
		}
		if row.url == "" && entry.MaintainerURL != "" {
			row.url = entry.MaintainerURL
		}
		row.feedCount++
		row.uniqueIPs += entry.UniqueIPs
		row.categories[src.Category] = struct{}{}
	}

	out := make([]MaintainerIndexEntry, 0, len(rows))
	for _, row := range rows {
		cats := make([]string, 0, len(row.categories))
		for c := range row.categories {
			cats = append(cats, c)
		}
		slices.Sort(cats)
		out = append(out, MaintainerIndexEntry{
			Slug:       row.slug,
			Name:       row.name,
			URL:        row.url,
			FeedCount:  row.feedCount,
			UniqueIPs:  row.uniqueIPs,
			Categories: cats,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FeedCount != out[j].FeedCount {
			return out[i].FeedCount > out[j].FeedCount
		}
		return out[i].Name < out[j].Name
	})
	return &MaintainerIndexPayload{Maintainers: out}, nil
}

// allowHomeFilter is a small guard ensuring the filter never includes
// system-role categories. Exported as a sanity helper if other callers
// want the same semantics; unused internally today but referenced by
// the tests in this package.
var _ = func(cfg *config.Config, src *config.Source) bool { return homeSummaryEligible(cfg, src, nil) }
