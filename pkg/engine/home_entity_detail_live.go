package engine

import (
	"strings"

	"github.com/firehol/update-ipsets/pkg/asnloc"
)

type entityDetailProviderSet struct {
	geo         HomeSummaryProvider
	asn         HomeSummaryProvider
	geoPrepared *geoPreparedProvider
	asnDB       *asnloc.Database
	asnLease    *asnDatabaseLease
}

func (e *Engine) entityDetailProviders() entityDetailProviderSet {
	geoProvider := e.preferredGeoProvider()
	asnProvider := e.preferredASNProvider()
	asnLease := e.loadASNProviderForLookup(asnProvider)
	var asnDB *asnloc.Database
	if asnLease != nil {
		asnDB = asnLease.Database()
	}
	return entityDetailProviderSet{
		geo: HomeSummaryProvider{
			Name:  geoProvider,
			Label: providerDisplayLabel(e.lookupSource(geoProvider)),
		},
		asn: HomeSummaryProvider{
			Name:  asnProvider,
			Label: providerDisplayLabel(e.lookupSource(asnProvider)),
		},
		geoPrepared: e.loadGeoProviderForLookup(geoProvider),
		asnDB:       asnDB,
		asnLease:    asnLease,
	}
}

func (p entityDetailProviderSet) Close() {
	if p.asnLease != nil {
		p.asnLease.Close()
	}
}

func (b *countryDetailBuilder) buildAllowEmpty(geoProvider, asnProvider HomeSummaryProvider) *countryDetailSidecar {
	if sidecar := b.build(geoProvider, asnProvider); sidecar != nil {
		return sidecar
	}
	code := ""
	if b != nil {
		code = b.code
	}
	return &countryDetailSidecar{
		Code:        code,
		Provider:    geoProvider,
		ASNProvider: asnProvider,
		Feeds:       make([]countryDetailFeedBase, 0),
	}
}

func (b *asnDetailBuilder) buildAllowEmpty(asnProvider, geoProvider HomeSummaryProvider) *asnDetailSidecar {
	if sidecar := b.build(asnProvider, geoProvider); sidecar != nil {
		return sidecar
	}
	var asn uint32
	name := ""
	description := ""
	if b != nil {
		asn = b.asn
		name = b.name
		description = b.description
	}
	return &asnDetailSidecar{
		ASN:         asn,
		Name:        name,
		Description: description,
		Provider:    asnProvider,
		GeoProvider: geoProvider,
		Feeds:       make([]asnDetailFeedBase, 0),
	}
}

func (e *Engine) addCountryDetailFeedMatches(builder *countryDetailBuilder, provider string, view entityOutputView) []string {
	matches := make([]string, 0, 64)
	if builder == nil {
		return matches
	}
	for _, entry := range e.EntriesSnapshot() {
		if !e.isPublicFeedName(entry.Name) {
			continue
		}
		src := e.lookupSource(entry.Name)
		if !detailSurfaceEligible(e.cfg, src) {
			continue
		}
		attributed := countryComparisonValue(view, entry.Name, provider, builder.code)
		if attributed == 0 {
			continue
		}
		builder.addFeed(countryDetailFeedBase{
			Name:          entry.Name,
			Category:      src.Category,
			Provenance:    string(publicProvenance(src)),
			Maintainer:    entry.Maintainer,
			AttributedIPs: attributed,
			UniqueIPs:     entry.UniqueIPs,
			LastChangeTS:  entry.SourceDate,
		}, entry.MaintainerURL)
		matches = append(matches, entry.Name)
	}
	return matches
}

func countryComparisonValue(view entityOutputView, feedName, provider, code string) uint64 {
	if provider == "" {
		return 0
	}
	payload, err := view.countryComparison(feedName, provider)
	if err != nil || payload == nil {
		return 0
	}
	for _, country := range payload.Countries {
		if strings.ToUpper(strings.TrimSpace(country.Code)) == code {
			return country.Value
		}
	}
	return 0
}

func (e *Engine) addCountryDetailASNMatches(builder *countryDetailBuilder, matches []string, providers entityDetailProviderSet) {
	if builder == nil || providers.geoPrepared == nil || providers.asnDB == nil || len(matches) == 0 {
		return
	}
	setCache := newLatestSetCache(e)
	defer setCache.CloseAll(e.logger)
	for _, name := range matches {
		latest, err := setCache.Open(name)
		if err != nil || latest == nil || latest.RangeSource == nil {
			continue
		}
		counts, names, err := providers.asnDB.CountFeed(countryFilteredRangeSource(latest.RangeSource, providers.geoPrepared, builder.code))
		if err != nil {
			continue
		}
		for asn, count := range counts {
			builder.addASN(asn, names[asn], count)
		}
	}
}

func (e *Engine) addASNDetailFeedMatches(builder *asnDetailBuilder, provider string, view entityOutputView) []string {
	matches := make([]string, 0, 64)
	if builder == nil || provider == "" {
		return matches
	}
	for _, entry := range e.EntriesSnapshot() {
		if !e.isPublicFeedName(entry.Name) {
			continue
		}
		src := e.lookupSource(entry.Name)
		if !detailSurfaceEligible(e.cfg, src) {
			continue
		}
		attributed, observedName := asnComparisonValue(view, entry.Name, provider, builder.asn)
		if attributed == 0 {
			continue
		}
		builder.addFeed(asnDetailFeedBase{
			Name:          entry.Name,
			Category:      src.Category,
			Provenance:    string(publicProvenance(src)),
			Maintainer:    entry.Maintainer,
			AttributedIPs: attributed,
			UniqueIPs:     entry.UniqueIPs,
			LastChangeTS:  entry.SourceDate,
		}, entry.MaintainerURL, observedName)
		matches = append(matches, entry.Name)
	}
	return matches
}

func asnComparisonValue(view entityOutputView, feedName, provider string, asn uint32) (uint64, string) {
	for _, row := range view.topASNs(feedName, provider) {
		if row.ASN == asn {
			return row.Count, row.Name
		}
	}
	return 0, ""
}

func (e *Engine) addASNDetailCountryMatches(builder *asnDetailBuilder, matches []string, providers entityDetailProviderSet) {
	if builder == nil || providers.asnDB == nil || providers.geoPrepared == nil || len(matches) == 0 {
		return
	}
	setCache := newLatestSetCache(e)
	defer setCache.CloseAll(e.logger)
	for _, name := range matches {
		latest, err := setCache.Open(name)
		if err != nil || latest == nil || latest.RangeSource == nil {
			continue
		}
		values, totalMapped, err := countCountriesForASNSource(latest.RangeSource, providers.asnDB, providers.geoPrepared, builder.asn)
		if err != nil || totalMapped == 0 {
			continue
		}
		builder.addCountryDistribution(values, totalMapped)
	}
}

func (b *asnDetailBuilder) addCountryDistribution(values []CountryValue, totalMapped uint64) {
	if b == nil || totalMapped == 0 {
		return
	}
	b.totalMapped += totalMapped
	for _, value := range values {
		code := strings.ToUpper(strings.TrimSpace(value.Code))
		if code == "" || value.Value == 0 {
			continue
		}
		agg := b.countryTotals[code]
		if agg == nil {
			agg = &asnDetailCountryAggregate{}
			b.countryTotals[code] = agg
		}
		agg.feedCount++
		agg.attributedIPs += value.Value
		b.distributionCounts[code] += value.Value
	}
}
