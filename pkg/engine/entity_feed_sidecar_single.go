package engine

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/firehol/update-ipsets/pkg/asnloc"
)

func (e *Engine) buildSingleFeedEntitySidecar(name string, view entityOutputView, resolver *effectiveEntryResolver, geoProvider, asnProvider string, geoPrepared *geoPreparedProvider, asnDB *asnloc.Database, setCache *latestSetCache) (*feedEntitySidecar, error) {
	sidecar, ok := e.newFeedEntitySidecar(name, resolver, geoProvider, asnProvider)
	if !ok {
		return nil, nil
	}

	countriesByCode, err := feedEntityCountryRows(view, name, geoProvider)
	if err != nil {
		return nil, err
	}
	asnsByNumber := feedEntityASNRows(view, name, asnProvider)

	if err := e.enrichFeedEntityJointRows(name, countriesByCode, asnsByNumber, geoPrepared, asnDB, setCache); err != nil {
		return nil, err
	}

	sidecar.Countries = sortedFeedEntityCountries(countriesByCode)
	sidecar.ASNs = sortedFeedEntityASNs(asnsByNumber)
	return sidecar, nil
}

func (e *Engine) newFeedEntitySidecar(name string, resolver *effectiveEntryResolver, geoProvider, asnProvider string) (*feedEntitySidecar, bool) {
	entry := resolver.entryFromSnapshot(name)
	if entry == nil || !e.isPublicFeedName(name) {
		return nil, false
	}
	src := e.lookupSource(name)
	if !detailSurfaceEligible(e.cfg, src) {
		return nil, false
	}
	return &feedEntitySidecar{
		Feed:         name,
		Category:     src.Category,
		Provenance:   string(publicProvenance(src)),
		Maintainer:   entry.Maintainer,
		UniqueIPs:    entry.UniqueIPs,
		LastChangeTS: entry.SourceDate,
		GeoProvider:  geoProvider,
		ASNProvider:  asnProvider,
	}, true
}

func (e *Engine) feedEntitySidecarExpected(name, geoProvider, asnProvider string, resolver *effectiveEntryResolver) bool {
	if e == nil || e.cfg == nil {
		return false
	}
	if resolver == nil && e.state != nil {
		resolver = newEffectiveEntryResolver(e.cfg, e.state.SnapshotEntries())
	}
	if resolver == nil {
		return false
	}
	_, ok := e.newFeedEntitySidecar(name, resolver, geoProvider, asnProvider)
	return ok
}

func feedEntityCountryRows(view entityOutputView, name, geoProvider string) (map[string]*feedEntityCountryContribution, error) {
	out := map[string]*feedEntityCountryContribution{}
	if geoProvider == "" {
		return out, nil
	}
	payload, err := view.countryComparison(name, geoProvider)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if payload == nil {
		return out, nil
	}
	for _, country := range payload.Countries {
		code := strings.ToUpper(strings.TrimSpace(country.Code))
		if code == "" || country.Value == 0 {
			continue
		}
		out[code] = &feedEntityCountryContribution{
			Code:          code,
			AttributedIPs: country.Value,
		}
	}
	return out, nil
}

func feedEntityASNRows(view entityOutputView, name, asnProvider string) map[uint32]*feedEntityASNContribution {
	out := map[uint32]*feedEntityASNContribution{}
	if asnProvider == "" {
		return out
	}
	for _, row := range view.topASNs(name, asnProvider) {
		if row.ASN == 0 || row.Count == 0 {
			continue
		}
		out[row.ASN] = &feedEntityASNContribution{
			ASN:           row.ASN,
			Name:          row.Name,
			AttributedIPs: row.Count,
		}
	}
	return out
}

func (e *Engine) enrichFeedEntityJointRows(name string, countriesByCode map[string]*feedEntityCountryContribution, asnsByNumber map[uint32]*feedEntityASNContribution, geoPrepared *geoPreparedProvider, asnDB *asnloc.Database, setCache *latestSetCache) error {
	if geoPrepared == nil || asnDB == nil || len(countriesByCode) == 0 || len(asnsByNumber) == 0 {
		return nil
	}
	if setCache == nil {
		setCache = newLatestSetCache(e)
		defer setCache.CloseAll(e.logger)
	}
	latest, err := setCache.Open(name)
	if err != nil {
		return fmt.Errorf("open latest set for entity sidecar %s: %w", name, err)
	}
	counts, namesByASN, stats, err := countCountryASNJointSource(latest.RangeSource, asnDB, geoPrepared)
	if err != nil {
		return fmt.Errorf("count country/asn attribution for %s: %w", name, err)
	}
	e.observeRunCounter("entity.sidecar_build.feed", 1, 0)
	e.observeRunCounter("entity.sidecar_build.source_ranges", stats.sourceRanges, 0)
	e.observeRunCounter("entity.sidecar_build.geo_segments", stats.geoSegments, 0)
	e.observeRunCounter("entity.sidecar_build.asn_lookups", stats.asnLookups, 0)
	e.observeRunCounter("entity.sidecar_build.country_asn_hits", stats.countryASNHits, 0)
	if ioErr := checkFileSetErr(latest.RangeSource, name, e.logger); ioErr != nil {
		return ioErr
	}

	applyCountryASNJointRows(countriesByCode, asnsByNumber, counts, namesByASN)
	backfillASNContributionRows(asnsByNumber, countriesByCode)
	return nil
}

func applyCountryASNJointRows(countriesByCode map[string]*feedEntityCountryContribution, asnsByNumber map[uint32]*feedEntityASNContribution, counts map[string]map[uint32]uint64, namesByASN map[uint32]string) {
	for code, countryCounts := range counts {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			continue
		}
		country := countriesByCode[code]
		if country == nil {
			country = &feedEntityCountryContribution{Code: code}
			countriesByCode[code] = country
		}
		rows, total := feedEntityJointASNRows(countryCounts, namesByASN, asnsByNumber)
		country.ASNs = rows
		if country.AttributedIPs == 0 {
			country.AttributedIPs = total
		}
	}
}

func feedEntityJointASNRows(countryCounts map[uint32]uint64, namesByASN map[uint32]string, asnsByNumber map[uint32]*feedEntityASNContribution) ([]feedEntityJointASN, uint64) {
	rows := make([]feedEntityJointASN, 0, len(countryCounts))
	var total uint64
	for asn, count := range countryCounts {
		if asn == 0 || count == 0 {
			continue
		}
		rows = append(rows, feedEntityJointASN{
			ASN:   asn,
			Name:  namesByASN[asn],
			Count: count,
		})
		total += count
		asnRow := asnsByNumber[asn]
		if asnRow == nil {
			asnRow = &feedEntityASNContribution{ASN: asn}
			asnsByNumber[asn] = asnRow
		}
		if asnRow.Name == "" {
			asnRow.Name = namesByASN[asn]
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].ASN < rows[j].ASN
	})
	return rows, total
}

func backfillASNContributionRows(asnsByNumber map[uint32]*feedEntityASNContribution, countriesByCode map[string]*feedEntityCountryContribution) {
	for asn, row := range asnsByNumber {
		if row.Name == "" {
			row.Name = feedEntityASNNameFromCountries(asn, countriesByCode)
		}
		if row.AttributedIPs == 0 {
			row.AttributedIPs = feedEntityASNJointTotal(asn, countriesByCode)
		}
	}
}

func feedEntityASNNameFromCountries(asn uint32, countriesByCode map[string]*feedEntityCountryContribution) string {
	for _, country := range countriesByCode {
		for _, joint := range country.ASNs {
			if joint.ASN == asn && joint.Name != "" {
				return joint.Name
			}
		}
	}
	return ""
}

func feedEntityASNJointTotal(asn uint32, countriesByCode map[string]*feedEntityCountryContribution) uint64 {
	var total uint64
	for _, country := range countriesByCode {
		for _, joint := range country.ASNs {
			if joint.ASN == asn {
				total += joint.Count
			}
		}
	}
	return total
}

func sortedFeedEntityCountries(countriesByCode map[string]*feedEntityCountryContribution) []feedEntityCountryContribution {
	out := make([]feedEntityCountryContribution, 0, len(countriesByCode))
	for _, row := range countriesByCode {
		if row == nil || row.AttributedIPs == 0 {
			continue
		}
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

func sortedFeedEntityASNs(asnsByNumber map[uint32]*feedEntityASNContribution) []feedEntityASNContribution {
	out := make([]feedEntityASNContribution, 0, len(asnsByNumber))
	for _, row := range asnsByNumber {
		if row == nil || row.ASN == 0 || row.AttributedIPs == 0 {
			continue
		}
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ASN < out[j].ASN })
	return out
}
