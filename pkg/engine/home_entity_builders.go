package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/feedhealth"
)

type entityOutputView struct {
	e            *Engine
	stageDir     string
	mu           *sync.Mutex
	countryCache map[string]countryComparisonCacheEntry
	asnCache     map[string]asnRowsCacheEntry
}

type countryComparisonCacheEntry struct {
	payload *CountryComparisonPayload
	err     error
}

type asnRowsCacheEntry struct {
	rows []topASNRow
	err  error
}

func newEntityOutputView(e *Engine, stageDir string) entityOutputView {
	return entityOutputView{
		e:            e,
		stageDir:     strings.TrimSpace(stageDir),
		mu:           &sync.Mutex{},
		countryCache: map[string]countryComparisonCacheEntry{},
		asnCache:     map[string]asnRowsCacheEntry{},
	}
}

func (v entityOutputView) countryComparison(name, provider string) (*CountryComparisonPayload, error) {
	if v.e == nil || provider == "" {
		return nil, fmt.Errorf("country provider is not configured")
	}
	key := name + "\x00" + provider
	if v.mu != nil {
		v.mu.Lock()
		if cached, ok := v.countryCache[key]; ok {
			v.mu.Unlock()
			v.e.observeRunCounter("entity.output_view.country_json_cache_hit", 1, 0)
			return cached.payload, cached.err
		}
		v.mu.Unlock()
	}
	pathGroups := make([][]string, 0, 2)
	if v.stageDir != "" {
		pathGroups = append(pathGroups, geoCountryCandidatePaths(v.stageDir, name, provider))
	}
	pathGroups = append(pathGroups, geoCountryCandidatePaths(v.e.outputDir(), name, provider))
	data, err := readFirstExisting(pathGroups...)
	if err != nil {
		if v.mu != nil {
			v.mu.Lock()
			v.countryCache[key] = countryComparisonCacheEntry{err: err}
			v.mu.Unlock()
		}
		return nil, err
	}
	v.e.observeRunCounter("entity.output_view.country_json_read", 1, int64(len(data)))
	var payload CountryComparisonPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		if v.mu != nil {
			v.mu.Lock()
			v.countryCache[key] = countryComparisonCacheEntry{err: err}
			v.mu.Unlock()
		}
		return nil, err
	}
	v.e.observeRunCounter("entity.output_view.country_json_decode", 1, int64(len(data)))
	if v.mu != nil {
		v.mu.Lock()
		v.countryCache[key] = countryComparisonCacheEntry{payload: &payload}
		v.mu.Unlock()
	}
	return &payload, nil
}

func (v entityOutputView) topASNs(name, provider string) []topASNRow {
	rows, err := v.topASNsWithError(name, provider)
	if err != nil {
		return nil
	}
	return rows
}

func (v entityOutputView) topASNsWithError(name, provider string) ([]topASNRow, error) {
	if v.e == nil || provider == "" {
		return nil, fmt.Errorf("ASN provider is not configured")
	}
	key := name + "\x00" + provider
	if v.mu != nil {
		v.mu.Lock()
		if cached, ok := v.asnCache[key]; ok {
			v.mu.Unlock()
			v.e.observeRunCounter("entity.output_view.asn_json_cache_hit", 1, 0)
			return append([]topASNRow(nil), cached.rows...), cached.err
		}
		v.mu.Unlock()
	}
	pathGroups := make([][]string, 0, 2)
	if v.stageDir != "" {
		pathGroups = append(pathGroups, asnCandidatePaths(v.stageDir, name, provider))
	}
	pathGroups = append(pathGroups, asnCandidatePaths(v.e.outputDir(), name, provider))
	data, err := readFirstExisting(pathGroups...)
	if err != nil {
		if v.mu != nil {
			v.mu.Lock()
			v.asnCache[key] = asnRowsCacheEntry{err: err}
			v.mu.Unlock()
		}
		return nil, err
	}
	v.e.observeRunCounter("entity.output_view.asn_json_read", 1, int64(len(data)))
	var payload struct {
		ByASN []struct {
			ASN   uint32 `json:"asn"`
			Name  string `json:"name"`
			Count uint64 `json:"count"`
		} `json:"by_asn"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		if v.mu != nil {
			v.mu.Lock()
			v.asnCache[key] = asnRowsCacheEntry{err: err}
			v.mu.Unlock()
		}
		return nil, err
	}
	v.e.observeRunCounter("entity.output_view.asn_json_decode", 1, int64(len(data)))
	rows := make([]topASNRow, 0, len(payload.ByASN))
	for _, row := range payload.ByASN {
		rows = append(rows, topASNRow{ASN: row.ASN, Name: row.Name, Count: row.Count})
	}
	if v.mu != nil {
		v.mu.Lock()
		v.asnCache[key] = asnRowsCacheEntry{rows: append([]topASNRow(nil), rows...)}
		v.mu.Unlock()
	}
	return rows, nil
}

type countryDetailFeedBase struct {
	Name          string `json:"name"`
	Category      string `json:"category"`
	Provenance    string `json:"provenance,omitempty"`
	Maintainer    string `json:"maintainer,omitempty"`
	AttributedIPs uint64 `json:"attributed_ips"`
	UniqueIPs     uint64 `json:"unique_ips"`
	LastChangeTS  int64  `json:"last_change_ts,omitempty"`
}

type countryDetailSidecar struct {
	Code           string                    `json:"code"`
	Provider       HomeSummaryProvider       `json:"provider"`
	ASNProvider    HomeSummaryProvider       `json:"asn_provider,omitempty"`
	Totals         CountryDetailTotals       `json:"totals"`
	Feeds          []countryDetailFeedBase   `json:"feeds"`
	TopCategories  []DetailCategorySummary   `json:"top_categories,omitempty"`
	TopMaintainers []DetailMaintainerSummary `json:"top_maintainers,omitempty"`
	TopASNs        []CountryDetailASN        `json:"top_asns_in_country,omitempty"`
}

type asnDetailFeedBase struct {
	Name          string `json:"name"`
	Category      string `json:"category"`
	Provenance    string `json:"provenance,omitempty"`
	Maintainer    string `json:"maintainer,omitempty"`
	AttributedIPs uint64 `json:"attributed_ips"`
	UniqueIPs     uint64 `json:"unique_ips"`
	LastChangeTS  int64  `json:"last_change_ts,omitempty"`
}

type asnDetailSidecar struct {
	ASN                 uint32                    `json:"asn"`
	Name                string                    `json:"name,omitempty"`
	Description         string                    `json:"description,omitempty"`
	Provider            HomeSummaryProvider       `json:"provider"`
	GeoProvider         HomeSummaryProvider       `json:"geo_provider,omitempty"`
	Totals              ASNDetailTotals           `json:"totals"`
	Feeds               []asnDetailFeedBase       `json:"feeds"`
	TopCategories       []DetailCategorySummary   `json:"top_categories,omitempty"`
	TopMaintainers      []DetailMaintainerSummary `json:"top_maintainers,omitempty"`
	TopCountries        []ASNDetailCountry        `json:"top_countries,omitempty"`
	CountryDistribution *CountryComparisonPayload `json:"country_distribution,omitempty"`
}

type feedHealthClassifier struct {
	e        *Engine
	entries  map[string]cache.Entry
	resolver *effectiveEntryResolver
	policy   feedhealth.Policy
	now      time.Time
}

func (e *Engine) newFeedHealthClassifier() *feedHealthClassifier {
	if e == nil || e.cfg == nil || e.state == nil {
		return nil
	}
	entries := e.state.SnapshotEntries()
	now := time.Now().UTC()
	if e.now != nil {
		now = e.now().UTC()
	}
	return &feedHealthClassifier{
		e:        e,
		entries:  entries,
		resolver: newEffectiveEntryResolver(e.cfg, entries),
		policy:   feedhealth.PolicyFromRuntime(e.cfg.Runtime),
		now:      now,
	}
}

func (c *feedHealthClassifier) class(name string) string {
	snap, ok := c.snapshot(name)
	if !ok {
		return ""
	}
	return string(snap.Class)
}

func (c *feedHealthClassifier) snapshot(name string) (feedhealth.Snapshot, bool) {
	if c == nil || c.e == nil || c.e.cfg == nil || c.resolver == nil {
		return feedhealth.Snapshot{}, false
	}
	src := c.e.lookupSource(name)
	if src == nil {
		return feedhealth.Snapshot{}, false
	}
	entry, ok := c.entries[name]
	if !ok {
		entry = cache.Entry{Name: name}
	}
	view := c.resolver.entry(name, &entry)
	if view == nil {
		return feedhealth.Snapshot{}, false
	}
	return feedhealth.Classify(view, src, c.policy, c.now), true
}

func (e *Engine) CountryIndex() (*CountryIndexPayload, error) {
	return e.buildCountryIndex(newEntityOutputView(e, ""))
}

func (e *Engine) ASNIndex() (*ASNIndexPayload, error) {
	return e.buildASNIndex(newEntityOutputView(e, ""))
}

func (e *Engine) CountryDetail(code string) (*CountryDetailPayload, error) {
	sidecar, err := e.buildCountryDetailSidecar(code, newEntityOutputView(e, ""))
	if err != nil {
		return nil, err
	}
	return e.materializeCountryDetail(sidecar), nil
}

func (e *Engine) ASNDetail(asn uint32) (*ASNDetailPayload, error) {
	sidecar, err := e.buildASNDetailSidecar(asn, newEntityOutputView(e, ""))
	if err != nil {
		return nil, err
	}
	return e.materializeASNDetail(sidecar), nil
}

func (e *Engine) buildCountryIndex(view entityOutputView) (*CountryIndexPayload, error) {
	if e == nil || e.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	geoProvider := e.preferredGeoProvider()
	geoSrc := e.lookupSource(geoProvider)
	payload := &CountryIndexPayload{
		Provider: HomeSummaryProvider{
			Name:  geoProvider,
			Label: providerDisplayLabel(geoSrc),
		},
	}
	if geoProvider == "" {
		return payload, nil
	}

	type aggregate struct {
		feedCount     int
		attributedIPs uint64
	}
	rows := map[string]*aggregate{}
	entries := e.EntriesSnapshot()
	for i := range entries {
		entry := &entries[i]
		if entry == nil || !e.isPublicFeedName(entry.Name) {
			continue
		}
		src := e.lookupSource(entry.Name)
		if !detailSurfaceEligible(e.cfg, src) {
			continue
		}
		countryPayload, err := view.countryComparison(entry.Name, geoProvider)
		if err != nil || countryPayload == nil {
			continue
		}
		seen := map[string]struct{}{}
		for _, country := range countryPayload.Countries {
			code := strings.ToUpper(strings.TrimSpace(country.Code))
			if code == "" {
				continue
			}
			agg := rows[code]
			if agg == nil {
				agg = &aggregate{}
				rows[code] = agg
			}
			agg.attributedIPs += country.Value
			if _, ok := seen[code]; !ok {
				agg.feedCount++
				seen[code] = struct{}{}
			}
		}
	}

	out := make([]CountryIndexEntry, 0, len(rows))
	for code, agg := range rows {
		out = append(out, CountryIndexEntry{
			Code:          code,
			FeedCount:     agg.feedCount,
			AttributedIPs: agg.attributedIPs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FeedCount != out[j].FeedCount {
			return out[i].FeedCount > out[j].FeedCount
		}
		if out[i].AttributedIPs != out[j].AttributedIPs {
			return out[i].AttributedIPs > out[j].AttributedIPs
		}
		return out[i].Code < out[j].Code
	})
	payload.Countries = out
	return payload, nil
}

func (e *Engine) buildASNIndex(view entityOutputView) (*ASNIndexPayload, error) {
	if e == nil || e.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	asnProvider := e.preferredASNProvider()
	asnSrc := e.lookupSource(asnProvider)
	payload := &ASNIndexPayload{
		Provider: HomeSummaryProvider{
			Name:  asnProvider,
			Label: providerDisplayLabel(asnSrc),
		},
	}
	if asnProvider == "" {
		return payload, nil
	}

	type aggregate struct {
		name          string
		feedCount     int
		attributedIPs uint64
	}
	rows := map[uint32]*aggregate{}
	entries := e.EntriesSnapshot()
	for i := range entries {
		entry := &entries[i]
		if entry == nil || !e.isPublicFeedName(entry.Name) {
			continue
		}
		src := e.lookupSource(entry.Name)
		if !detailSurfaceEligible(e.cfg, src) {
			continue
		}
		seen := map[uint32]struct{}{}
		for _, row := range view.topASNs(entry.Name, asnProvider) {
			if row.ASN == 0 {
				continue
			}
			agg := rows[row.ASN]
			if agg == nil {
				agg = &aggregate{name: row.Name}
				rows[row.ASN] = agg
			}
			if agg.name == "" && row.Name != "" {
				agg.name = row.Name
			}
			agg.attributedIPs += row.Count
			if _, ok := seen[row.ASN]; !ok {
				agg.feedCount++
				seen[row.ASN] = struct{}{}
			}
		}
	}

	out := make([]ASNIndexEntry, 0, len(rows))
	for asn, agg := range rows {
		out = append(out, ASNIndexEntry{
			ASN:           asn,
			Name:          agg.name,
			FeedCount:     agg.feedCount,
			AttributedIPs: agg.attributedIPs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FeedCount != out[j].FeedCount {
			return out[i].FeedCount > out[j].FeedCount
		}
		if out[i].AttributedIPs != out[j].AttributedIPs {
			return out[i].AttributedIPs > out[j].AttributedIPs
		}
		return out[i].ASN < out[j].ASN
	})
	payload.ASNs = out
	return payload, nil
}

func (e *Engine) buildCountryDetailSidecar(code string, view entityOutputView) (*countryDetailSidecar, error) {
	if e == nil || e.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if normalized == "" {
		return nil, fmt.Errorf("country code is required")
	}
	geoProvider := e.preferredGeoProvider()
	geoSrc := e.lookupSource(geoProvider)
	asnProvider := e.preferredASNProvider()
	asnSrc := e.lookupSource(asnProvider)
	geoPrepared := e.loadGeoProviderForLookup(geoProvider)
	asnDB := e.loadASNProviderForLookup(asnProvider)

	feeds := make([]countryDetailFeedBase, 0, 64)
	categoryTotals := map[string]*detailCategoryAggregate{}
	maintainerTotals := map[string]*detailMaintainerAggregate{}
	var totalAttributed uint64
	matches := make([]string, 0, 64)

	entries := e.EntriesSnapshot()
	for i := range entries {
		entry := &entries[i]
		if entry == nil || !e.isPublicFeedName(entry.Name) {
			continue
		}
		src := e.lookupSource(entry.Name)
		if !detailSurfaceEligible(e.cfg, src) {
			continue
		}

		var attributedInCountry uint64
		if geoProvider != "" {
			payload, err := view.countryComparison(entry.Name, geoProvider)
			if err != nil || payload == nil {
				continue
			}
			for _, country := range payload.Countries {
				if strings.ToUpper(strings.TrimSpace(country.Code)) == normalized {
					attributedInCountry = country.Value
					break
				}
			}
		}
		if attributedInCountry == 0 {
			continue
		}

		row := countryDetailFeedBase{
			Name:          entry.Name,
			Category:      src.Category,
			Provenance:    string(publicProvenance(src)),
			Maintainer:    entry.Maintainer,
			AttributedIPs: attributedInCountry,
			UniqueIPs:     entry.UniqueIPs,
			LastChangeTS:  entry.SourceDate,
		}
		feeds = append(feeds, row)
		totalAttributed += attributedInCountry
		matches = append(matches, entry.Name)

		categoryAgg := categoryTotals[src.Category]
		if categoryAgg == nil {
			categoryAgg = &detailCategoryAggregate{}
			categoryTotals[src.Category] = categoryAgg
		}
		categoryAgg.feedCount++
		categoryAgg.attributedIPs += attributedInCountry

		maintainerName := strings.TrimSpace(entry.Maintainer)
		if maintainerName != "" {
			slug := maintainerSlugify(maintainerName)
			maintainerAgg := maintainerTotals[slug]
			if maintainerAgg == nil {
				maintainerAgg = &detailMaintainerAggregate{
					slug: slug,
					name: maintainerName,
					url:  entry.MaintainerURL,
				}
				maintainerTotals[slug] = maintainerAgg
			}
			if maintainerAgg.url == "" && entry.MaintainerURL != "" {
				maintainerAgg.url = entry.MaintainerURL
			}
			maintainerAgg.feedCount++
			maintainerAgg.attributedIPs += attributedInCountry
		}
	}

	sort.Slice(feeds, func(i, j int) bool {
		if feeds[i].AttributedIPs != feeds[j].AttributedIPs {
			return feeds[i].AttributedIPs > feeds[j].AttributedIPs
		}
		return feeds[i].Name < feeds[j].Name
	})

	topCategories := make([]DetailCategorySummary, 0, len(categoryTotals))
	for category, agg := range categoryTotals {
		topCategories = append(topCategories, DetailCategorySummary{
			Category:      category,
			FeedCount:     agg.feedCount,
			AttributedIPs: agg.attributedIPs,
		})
	}
	sort.Slice(topCategories, func(i, j int) bool {
		if topCategories[i].AttributedIPs != topCategories[j].AttributedIPs {
			return topCategories[i].AttributedIPs > topCategories[j].AttributedIPs
		}
		if topCategories[i].FeedCount != topCategories[j].FeedCount {
			return topCategories[i].FeedCount > topCategories[j].FeedCount
		}
		return topCategories[i].Category < topCategories[j].Category
	})

	topMaintainers := make([]DetailMaintainerSummary, 0, len(maintainerTotals))
	for _, agg := range maintainerTotals {
		topMaintainers = append(topMaintainers, DetailMaintainerSummary{
			Slug:          agg.slug,
			Name:          agg.name,
			URL:           agg.url,
			FeedCount:     agg.feedCount,
			AttributedIPs: agg.attributedIPs,
		})
	}
	sort.Slice(topMaintainers, func(i, j int) bool {
		if topMaintainers[i].AttributedIPs != topMaintainers[j].AttributedIPs {
			return topMaintainers[i].AttributedIPs > topMaintainers[j].AttributedIPs
		}
		if topMaintainers[i].FeedCount != topMaintainers[j].FeedCount {
			return topMaintainers[i].FeedCount > topMaintainers[j].FeedCount
		}
		return topMaintainers[i].Name < topMaintainers[j].Name
	})

	type asnAggregate struct {
		name          string
		feedCount     int
		attributedIPs uint64
	}
	asnTotals := map[uint32]*asnAggregate{}
	if geoPrepared != nil && asnDB != nil && len(matches) > 0 {
		setCache := newLatestSetCache(e)
		defer setCache.CloseAll(e.logger)
		for _, name := range matches {
			latest, err := setCache.Open(name)
			if err != nil || latest == nil || latest.RangeSource == nil {
				continue
			}
			counts, names, err := asnDB.CountFeed(countryFilteredRangeSource(latest.RangeSource, geoPrepared, normalized))
			if err != nil {
				continue
			}
			for asn, count := range counts {
				if asn == 0 || count == 0 {
					continue
				}
				agg := asnTotals[asn]
				if agg == nil {
					agg = &asnAggregate{}
					asnTotals[asn] = agg
				}
				if agg.name == "" {
					agg.name = names[asn]
				}
				agg.feedCount++
				agg.attributedIPs += count
			}
		}
	}

	topASNs := make([]CountryDetailASN, 0, len(asnTotals))
	for asn, agg := range asnTotals {
		topASNs = append(topASNs, CountryDetailASN{
			ASN:           asn,
			Name:          agg.name,
			FeedCount:     agg.feedCount,
			AttributedIPs: agg.attributedIPs,
		})
	}
	sort.Slice(topASNs, func(i, j int) bool {
		if topASNs[i].AttributedIPs != topASNs[j].AttributedIPs {
			return topASNs[i].AttributedIPs > topASNs[j].AttributedIPs
		}
		if topASNs[i].FeedCount != topASNs[j].FeedCount {
			return topASNs[i].FeedCount > topASNs[j].FeedCount
		}
		return topASNs[i].ASN < topASNs[j].ASN
	})

	return &countryDetailSidecar{
		Code: normalized,
		Provider: HomeSummaryProvider{
			Name:  geoProvider,
			Label: providerDisplayLabel(geoSrc),
		},
		ASNProvider: HomeSummaryProvider{
			Name:  asnProvider,
			Label: providerDisplayLabel(asnSrc),
		},
		Totals: CountryDetailTotals{
			FeedsMatching:       len(feeds),
			AttributedIPsInFeed: totalAttributed,
			Categories:          len(categoryTotals),
			Maintainers:         len(maintainerTotals),
			ASNs:                len(asnTotals),
		},
		Feeds:          feeds,
		TopCategories:  topCategories,
		TopMaintainers: topMaintainers,
		TopASNs:        topASNs,
	}, nil
}

func (e *Engine) buildASNDetailSidecar(asn uint32, view entityOutputView) (*asnDetailSidecar, error) {
	if e == nil || e.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	if asn == 0 {
		return nil, fmt.Errorf("asn must be a positive integer")
	}
	asnProvider := e.preferredASNProvider()
	asnSrc := e.lookupSource(asnProvider)
	geoProvider := e.preferredGeoProvider()
	geoSrc := e.lookupSource(geoProvider)
	asnDB := e.loadASNProviderForLookup(asnProvider)
	geoPrepared := e.loadGeoProviderForLookup(geoProvider)

	sidecar := &asnDetailSidecar{
		ASN: asn,
		Provider: HomeSummaryProvider{
			Name:  asnProvider,
			Label: providerDisplayLabel(asnSrc),
		},
		GeoProvider: HomeSummaryProvider{
			Name:  geoProvider,
			Label: providerDisplayLabel(geoSrc),
		},
	}
	if asnProvider == "" {
		return sidecar, nil
	}

	feeds := make([]asnDetailFeedBase, 0, 64)
	categoryTotals := map[string]*detailCategoryAggregate{}
	maintainerTotals := map[string]*detailMaintainerAggregate{}
	var totalAttributed uint64
	var observedName string
	matches := make([]string, 0, 64)

	entries := e.EntriesSnapshot()
	for i := range entries {
		entry := &entries[i]
		if entry == nil || !e.isPublicFeedName(entry.Name) {
			continue
		}
		src := e.lookupSource(entry.Name)
		if !detailSurfaceEligible(e.cfg, src) {
			continue
		}

		var attributedInASN uint64
		for _, row := range view.topASNs(entry.Name, asnProvider) {
			if row.ASN == asn {
				attributedInASN = row.Count
				if observedName == "" && row.Name != "" {
					observedName = row.Name
				}
				break
			}
		}
		if attributedInASN == 0 {
			continue
		}

		row := asnDetailFeedBase{
			Name:          entry.Name,
			Category:      src.Category,
			Provenance:    string(publicProvenance(src)),
			Maintainer:    entry.Maintainer,
			AttributedIPs: attributedInASN,
			UniqueIPs:     entry.UniqueIPs,
			LastChangeTS:  entry.SourceDate,
		}
		feeds = append(feeds, row)
		totalAttributed += attributedInASN
		matches = append(matches, entry.Name)

		categoryAgg := categoryTotals[src.Category]
		if categoryAgg == nil {
			categoryAgg = &detailCategoryAggregate{}
			categoryTotals[src.Category] = categoryAgg
		}
		categoryAgg.feedCount++
		categoryAgg.attributedIPs += attributedInASN

		maintainerName := strings.TrimSpace(entry.Maintainer)
		if maintainerName != "" {
			slug := maintainerSlugify(maintainerName)
			maintainerAgg := maintainerTotals[slug]
			if maintainerAgg == nil {
				maintainerAgg = &detailMaintainerAggregate{
					slug: slug,
					name: maintainerName,
					url:  entry.MaintainerURL,
				}
				maintainerTotals[slug] = maintainerAgg
			}
			if maintainerAgg.url == "" && entry.MaintainerURL != "" {
				maintainerAgg.url = entry.MaintainerURL
			}
			maintainerAgg.feedCount++
			maintainerAgg.attributedIPs += attributedInASN
		}
	}

	sort.Slice(feeds, func(i, j int) bool {
		if feeds[i].AttributedIPs != feeds[j].AttributedIPs {
			return feeds[i].AttributedIPs > feeds[j].AttributedIPs
		}
		return feeds[i].Name < feeds[j].Name
	})

	topCategories := make([]DetailCategorySummary, 0, len(categoryTotals))
	for category, agg := range categoryTotals {
		topCategories = append(topCategories, DetailCategorySummary{
			Category:      category,
			FeedCount:     agg.feedCount,
			AttributedIPs: agg.attributedIPs,
		})
	}
	sort.Slice(topCategories, func(i, j int) bool {
		if topCategories[i].AttributedIPs != topCategories[j].AttributedIPs {
			return topCategories[i].AttributedIPs > topCategories[j].AttributedIPs
		}
		if topCategories[i].FeedCount != topCategories[j].FeedCount {
			return topCategories[i].FeedCount > topCategories[j].FeedCount
		}
		return topCategories[i].Category < topCategories[j].Category
	})

	topMaintainers := make([]DetailMaintainerSummary, 0, len(maintainerTotals))
	for _, agg := range maintainerTotals {
		topMaintainers = append(topMaintainers, DetailMaintainerSummary{
			Slug:          agg.slug,
			Name:          agg.name,
			URL:           agg.url,
			FeedCount:     agg.feedCount,
			AttributedIPs: agg.attributedIPs,
		})
	}
	sort.Slice(topMaintainers, func(i, j int) bool {
		if topMaintainers[i].AttributedIPs != topMaintainers[j].AttributedIPs {
			return topMaintainers[i].AttributedIPs > topMaintainers[j].AttributedIPs
		}
		if topMaintainers[i].FeedCount != topMaintainers[j].FeedCount {
			return topMaintainers[i].FeedCount > topMaintainers[j].FeedCount
		}
		return topMaintainers[i].Name < topMaintainers[j].Name
	})

	type countryAggregate struct {
		feedCount     int
		attributedIPs uint64
	}
	countryTotals := map[string]*countryAggregate{}
	distributionCounts := map[string]uint64{}
	var distributionTotalMapped uint64
	if asnDB != nil && geoPrepared != nil && len(matches) > 0 {
		setCache := newLatestSetCache(e)
		defer setCache.CloseAll(e.logger)
		for _, name := range matches {
			latest, err := setCache.Open(name)
			if err != nil || latest == nil || latest.RangeSource == nil {
				continue
			}
			values, totalMapped, err := countCountriesForASNSource(latest.RangeSource, asnDB, geoPrepared, asn)
			if err != nil || totalMapped == 0 {
				continue
			}
			distributionTotalMapped += totalMapped
			for _, value := range values {
				distributionCounts[value.Code] += value.Value
				agg := countryTotals[value.Code]
				if agg == nil {
					agg = &countryAggregate{}
					countryTotals[value.Code] = agg
				}
				agg.feedCount++
				agg.attributedIPs += value.Value
			}
		}
	}

	countryDistribution := make([]CountryValue, 0, len(distributionCounts))
	for code, value := range distributionCounts {
		countryDistribution = append(countryDistribution, CountryValue{Code: code, Value: value})
	}
	sort.Slice(countryDistribution, func(i, j int) bool {
		if countryDistribution[i].Code != countryDistribution[j].Code {
			return countryDistribution[i].Code < countryDistribution[j].Code
		}
		return countryDistribution[i].Value > countryDistribution[j].Value
	})

	topCountries := make([]ASNDetailCountry, 0, len(countryTotals))
	for code, agg := range countryTotals {
		topCountries = append(topCountries, ASNDetailCountry{
			Code:          code,
			FeedCount:     agg.feedCount,
			AttributedIPs: agg.attributedIPs,
		})
	}
	sort.Slice(topCountries, func(i, j int) bool {
		if topCountries[i].AttributedIPs != topCountries[j].AttributedIPs {
			return topCountries[i].AttributedIPs > topCountries[j].AttributedIPs
		}
		if topCountries[i].FeedCount != topCountries[j].FeedCount {
			return topCountries[i].FeedCount > topCountries[j].FeedCount
		}
		return topCountries[i].Code < topCountries[j].Code
	})

	if sidecar.Name == "" {
		sidecar.Name = observedName
	}
	sidecar.Totals = ASNDetailTotals{
		FeedsMatching: len(feeds),
		AttributedIPs: totalAttributed,
		Categories:    len(categoryTotals),
		Maintainers:   len(maintainerTotals),
		Countries:     len(countryTotals),
	}
	sidecar.Feeds = feeds
	sidecar.TopCategories = topCategories
	sidecar.TopMaintainers = topMaintainers
	sidecar.TopCountries = topCountries
	if len(countryDistribution) > 0 {
		sidecar.CountryDistribution = &CountryComparisonPayload{
			TotalMapped: distributionTotalMapped,
			Countries:   countryDistribution,
		}
	}
	return sidecar, nil
}

func (e *Engine) materializeCountryDetail(sidecar *countryDetailSidecar) *CountryDetailPayload {
	return e.materializeCountryDetailWithHealth(sidecar, e.newFeedHealthClassifier())
}

func (e *Engine) materializeCountryDetailWithHealth(sidecar *countryDetailSidecar, health *feedHealthClassifier) *CountryDetailPayload {
	if sidecar == nil {
		return &CountryDetailPayload{}
	}
	feeds := make([]CountryDetailFeed, 0, len(sidecar.Feeds))
	grouped := map[string][]CountryDetailFeed{}
	for _, base := range sidecar.Feeds {
		row := CountryDetailFeed{
			Name:          base.Name,
			Category:      base.Category,
			Provenance:    base.Provenance,
			Maintainer:    base.Maintainer,
			AttributedIPs: base.AttributedIPs,
			UniqueIPs:     base.UniqueIPs,
			HealthClass:   health.class(base.Name),
			LastChangeTS:  base.LastChangeTS,
		}
		feeds = append(feeds, row)
		grouped[row.Category] = append(grouped[row.Category], row)
	}
	for category, rows := range grouped {
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].AttributedIPs != rows[j].AttributedIPs {
				return rows[i].AttributedIPs > rows[j].AttributedIPs
			}
			return rows[i].Name < rows[j].Name
		})
		grouped[category] = rows
	}
	return &CountryDetailPayload{
		Code:            sidecar.Code,
		Provider:        sidecar.Provider,
		ASNProvider:     sidecar.ASNProvider,
		Totals:          sidecar.Totals,
		Feeds:           feeds,
		FeedsByCategory: grouped,
		TopCategories:   sidecar.TopCategories,
		TopMaintainers:  sidecar.TopMaintainers,
		TopASNs:         sidecar.TopASNs,
	}
}

func (e *Engine) materializeASNDetail(sidecar *asnDetailSidecar) *ASNDetailPayload {
	return e.materializeASNDetailWithHealth(sidecar, e.newFeedHealthClassifier())
}

func (e *Engine) materializeASNDetailWithHealth(sidecar *asnDetailSidecar, health *feedHealthClassifier) *ASNDetailPayload {
	if sidecar == nil {
		return &ASNDetailPayload{}
	}
	feeds := make([]ASNDetailFeed, 0, len(sidecar.Feeds))
	grouped := map[string][]ASNDetailFeed{}
	for _, base := range sidecar.Feeds {
		row := ASNDetailFeed{
			Name:          base.Name,
			Category:      base.Category,
			Provenance:    base.Provenance,
			Maintainer:    base.Maintainer,
			AttributedIPs: base.AttributedIPs,
			UniqueIPs:     base.UniqueIPs,
			HealthClass:   health.class(base.Name),
			LastChangeTS:  base.LastChangeTS,
		}
		feeds = append(feeds, row)
		grouped[row.Category] = append(grouped[row.Category], row)
	}
	for category, rows := range grouped {
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].AttributedIPs != rows[j].AttributedIPs {
				return rows[i].AttributedIPs > rows[j].AttributedIPs
			}
			return rows[i].Name < rows[j].Name
		})
		grouped[category] = rows
	}
	return &ASNDetailPayload{
		ASN:                 sidecar.ASN,
		Name:                sidecar.Name,
		Description:         sidecar.Description,
		Provider:            sidecar.Provider,
		GeoProvider:         sidecar.GeoProvider,
		Totals:              sidecar.Totals,
		Feeds:               feeds,
		FeedsByCategory:     grouped,
		TopCategories:       sidecar.TopCategories,
		TopMaintainers:      sidecar.TopMaintainers,
		TopCountries:        sidecar.TopCountries,
		CountryDistribution: sidecar.CountryDistribution,
	}
}

func loadCountryDetailSidecar(path string) (*countryDetailSidecar, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sidecar countryDetailSidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return nil, err
	}
	return &sidecar, nil
}

func loadASNDetailSidecar(path string) (*asnDetailSidecar, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sidecar asnDetailSidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return nil, err
	}
	return &sidecar, nil
}

func writeJSONFile(path string, value any) error {
	data, err := jsonMarshalTabIndent(value)
	if err != nil {
		return err
	}
	return writeFileAtomicNoSync(path, append(data, '\n'), 0o600)
}

func writeJSONFileAt(path string, value any, mod time.Time) error {
	if err := writeJSONFile(path, value); err != nil {
		return err
	}
	if mod.IsZero() {
		return nil
	}
	return os.Chtimes(path, mod.UTC(), mod.UTC())
}

func sortedJSONFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		out = append(out, filepath.Join(dir, entry.Name()))
	}
	slices.Sort(out)
	return out, nil
}
