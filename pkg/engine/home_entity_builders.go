package engine

import (
	"encoding/json"
	"fmt"
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
	pathGroups := make([][]rootedCandidatePath, 0, 2)
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
	pathGroups := make([][]rootedCandidatePath, 0, 2)
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
	providers := e.entityDetailProviders()
	builder := newCountryDetailBuilder(normalized)
	matches := e.addCountryDetailFeedMatches(builder, providers.geo.Name, view)
	e.addCountryDetailASNMatches(builder, matches, providers)
	return builder.buildAllowEmpty(providers.geo, providers.asn), nil
}

func (e *Engine) buildASNDetailSidecar(asn uint32, view entityOutputView) (*asnDetailSidecar, error) {
	if e == nil || e.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	if asn == 0 {
		return nil, fmt.Errorf("asn must be a positive integer")
	}
	providers := e.entityDetailProviders()
	builder := newASNDetailBuilder(asn)
	if providers.asn.Name == "" {
		return builder.buildAllowEmpty(providers.asn, providers.geo), nil
	}
	matches := e.addASNDetailFeedMatches(builder, providers.asn.Name, view)
	e.addASNDetailCountryMatches(builder, matches, providers)
	return builder.buildAllowEmpty(providers.asn, providers.geo), nil
}
