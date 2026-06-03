package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/firehol/update-ipsets/pkg/asnloc"
)

type feedEntityJointASN struct {
	ASN   uint32 `json:"asn"`
	Name  string `json:"name,omitempty"`
	Count uint64 `json:"count"`
}

type feedEntityCountryContribution struct {
	Code          string               `json:"code"`
	AttributedIPs uint64               `json:"attributed_ips"`
	ASNs          []feedEntityJointASN `json:"asns,omitempty"`
}

type feedEntityASNContribution struct {
	ASN           uint32 `json:"asn"`
	Name          string `json:"name,omitempty"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

type feedEntitySidecar struct {
	Feed         string                          `json:"feed"`
	Category     string                          `json:"category,omitempty"`
	Provenance   string                          `json:"provenance,omitempty"`
	Maintainer   string                          `json:"maintainer,omitempty"`
	UniqueIPs    uint64                          `json:"unique_ips"`
	LastChangeTS int64                           `json:"last_change_ts,omitempty"`
	GeoProvider  string                          `json:"geo_provider,omitempty"`
	ASNProvider  string                          `json:"asn_provider,omitempty"`
	Countries    []feedEntityCountryContribution `json:"countries,omitempty"`
	ASNs         []feedEntityASNContribution     `json:"asns,omitempty"`
	legacy       bool
}

func (s *feedEntitySidecar) UnmarshalJSON(data []byte) error {
	type rawFeedEntitySidecar struct {
		Feed         string          `json:"feed"`
		Category     string          `json:"category,omitempty"`
		Provenance   string          `json:"provenance,omitempty"`
		Maintainer   string          `json:"maintainer,omitempty"`
		UniqueIPs    uint64          `json:"unique_ips"`
		LastChangeTS int64           `json:"last_change_ts,omitempty"`
		GeoProvider  string          `json:"geo_provider,omitempty"`
		ASNProvider  string          `json:"asn_provider,omitempty"`
		Countries    json.RawMessage `json:"countries,omitempty"`
		ASNs         json.RawMessage `json:"asns,omitempty"`
	}

	var raw rawFeedEntitySidecar
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	countries, legacyCountries, err := decodeFeedEntityCountries(raw.Countries)
	if err != nil {
		return err
	}
	asns, legacyASNs, err := decodeFeedEntityASNs(raw.ASNs)
	if err != nil {
		return err
	}

	*s = feedEntitySidecar{
		Feed:         raw.Feed,
		Category:     raw.Category,
		Provenance:   raw.Provenance,
		Maintainer:   raw.Maintainer,
		UniqueIPs:    raw.UniqueIPs,
		LastChangeTS: raw.LastChangeTS,
		GeoProvider:  raw.GeoProvider,
		ASNProvider:  raw.ASNProvider,
		Countries:    countries,
		ASNs:         asns,
		legacy:       legacyCountries || legacyASNs,
	}
	return nil
}

func decodeFeedEntityCountries(raw json.RawMessage) ([]feedEntityCountryContribution, bool, error) {
	if emptyRawJSON(raw) {
		return nil, false, nil
	}
	var current []feedEntityCountryContribution
	if err := json.Unmarshal(raw, &current); err == nil {
		return current, false, nil
	} else {
		var legacy []string
		if legacyErr := json.Unmarshal(raw, &legacy); legacyErr != nil {
			return nil, false, fmt.Errorf("decode feed entity countries: %w", err)
		}
		rows := make([]feedEntityCountryContribution, 0, len(legacy))
		for _, code := range legacy {
			code = strings.ToUpper(strings.TrimSpace(code))
			if code == "" {
				continue
			}
			rows = append(rows, feedEntityCountryContribution{Code: code})
		}
		return rows, true, nil
	}
}

func decodeFeedEntityASNs(raw json.RawMessage) ([]feedEntityASNContribution, bool, error) {
	if emptyRawJSON(raw) {
		return nil, false, nil
	}
	var current []feedEntityASNContribution
	if err := json.Unmarshal(raw, &current); err == nil {
		return current, false, nil
	} else {
		var legacy []uint32
		if legacyErr := json.Unmarshal(raw, &legacy); legacyErr != nil {
			return nil, false, fmt.Errorf("decode feed entity ASNs: %w", err)
		}
		rows := make([]feedEntityASNContribution, 0, len(legacy))
		for _, asn := range legacy {
			if asn == 0 {
				continue
			}
			rows = append(rows, feedEntityASNContribution{ASN: asn})
		}
		return rows, true, nil
	}
}

func emptyRawJSON(raw json.RawMessage) bool {
	value := strings.TrimSpace(string(raw))
	return value == "" || value == "null"
}

type feedEntitySidecarIndex struct {
	countries map[string]feedEntityCountryContribution
	asns      map[uint32]feedEntityASNContribution
	byASN     map[uint32][]asnCountryDeltaRow
}

type countryActorContribution struct {
	feed countryDetailFeedBase
	asns []feedEntityJointASN
}

type asnActorContribution struct {
	feed      asnDetailFeedBase
	name      string
	countries []asnCountryDeltaRow
}

func (e *Engine) loadFeedEntitySidecar(path string) (*feedEntitySidecar, error) {
	data, err := readFilePathUnderRoot(filepath.Dir(path), path)
	if err != nil {
		return nil, err
	}
	var sidecar feedEntitySidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return nil, err
	}
	if sidecar.Feed == "" {
		sidecar.Feed = strings.TrimSuffix(filepath.Base(path), ".json")
	}
	return &sidecar, nil
}

func (e *Engine) loadCommittedFeedEntitySidecar(name string) (*feedEntitySidecar, error) {
	return e.loadFeedEntitySidecar(filepath.Join(e.entityFeedsDir(), name+".json"))
}

func (e *Engine) loadAllFeedEntitySidecars() (map[string]*feedEntitySidecar, error) {
	files, err := sortedJSONFiles(e.entityFeedsDir())
	if err != nil {
		return nil, err
	}
	out := make(map[string]*feedEntitySidecar, len(files))
	for _, path := range files {
		sidecar, err := e.loadFeedEntitySidecar(path)
		if err != nil {
			return nil, err
		}
		if sidecar == nil || strings.TrimSpace(sidecar.Feed) == "" {
			continue
		}
		out[sidecar.Feed] = sidecar
	}
	return out, nil
}

func indexFeedEntitySidecar(sidecar *feedEntitySidecar) feedEntitySidecarIndex {
	index := feedEntitySidecarIndex{
		countries: map[string]feedEntityCountryContribution{},
		asns:      map[uint32]feedEntityASNContribution{},
		byASN:     map[uint32][]asnCountryDeltaRow{},
	}
	if sidecar == nil {
		return index
	}
	for _, country := range sidecar.Countries {
		code := strings.ToUpper(strings.TrimSpace(country.Code))
		if code == "" {
			continue
		}
		country.Code = code
		rows := make([]feedEntityJointASN, 0, len(country.ASNs))
		for _, row := range country.ASNs {
			if row.ASN == 0 || row.Count == 0 {
				continue
			}
			rows = append(rows, row)
			index.byASN[row.ASN] = append(index.byASN[row.ASN], asnCountryDeltaRow{
				code:  code,
				count: row.Count,
			})
		}
		country.ASNs = rows
		index.countries[code] = country
	}
	for _, row := range sidecar.ASNs {
		if row.ASN == 0 || row.AttributedIPs == 0 {
			continue
		}
		index.asns[row.ASN] = row
	}
	return index
}

func (idx feedEntitySidecarIndex) countryContribution(code string) (feedEntityCountryContribution, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	row, ok := idx.countries[code]
	return row, ok
}

func (idx feedEntitySidecarIndex) asnContribution(asn uint32) (feedEntityASNContribution, bool) {
	row, ok := idx.asns[asn]
	return row, ok
}

func (idx feedEntitySidecarIndex) asnCountries(asn uint32) []asnCountryDeltaRow {
	if asn == 0 {
		return nil
	}
	return idx.byASN[asn]
}

func (s *feedEntitySidecar) countryRow(row feedEntityCountryContribution) countryDetailFeedBase {
	return countryDetailFeedBase{
		Name:          s.Feed,
		Category:      s.Category,
		Provenance:    s.Provenance,
		Maintainer:    s.Maintainer,
		AttributedIPs: row.AttributedIPs,
		UniqueIPs:     s.UniqueIPs,
		LastChangeTS:  s.LastChangeTS,
	}
}

func (s *feedEntitySidecar) asnRow(row feedEntityASNContribution) asnDetailFeedBase {
	return asnDetailFeedBase{
		Name:          s.Feed,
		Category:      s.Category,
		Provenance:    s.Provenance,
		Maintainer:    s.Maintainer,
		AttributedIPs: row.AttributedIPs,
		UniqueIPs:     s.UniqueIPs,
		LastChangeTS:  s.LastChangeTS,
	}
}

func (s *feedEntitySidecar) countryActorContribution(code string, idx feedEntitySidecarIndex) (countryActorContribution, bool) {
	if s == nil {
		return countryActorContribution{}, false
	}
	row, ok := idx.countryContribution(code)
	if !ok || row.AttributedIPs == 0 {
		return countryActorContribution{}, false
	}
	return countryActorContribution{
		feed: s.countryRow(row),
		asns: append([]feedEntityJointASN(nil), row.ASNs...),
	}, true
}

func (s *feedEntitySidecar) asnActorContribution(asn uint32, idx feedEntitySidecarIndex) (asnActorContribution, bool) {
	if s == nil || asn == 0 {
		return asnActorContribution{}, false
	}
	row, ok := idx.asnContribution(asn)
	if !ok || row.AttributedIPs == 0 {
		return asnActorContribution{}, false
	}
	return asnActorContribution{
		feed:      s.asnRow(row),
		name:      row.Name,
		countries: append([]asnCountryDeltaRow(nil), idx.asnCountries(asn)...),
	}, true
}

func (s *feedEntitySidecar) countryCodes() []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.Countries))
	for _, row := range s.Countries {
		code := strings.ToUpper(strings.TrimSpace(row.Code))
		if code != "" {
			out = append(out, code)
		}
	}
	slices.Sort(out)
	return uniqueNonEmptyStrings(out)
}

func (s *feedEntitySidecar) asnNumbers() []uint32 {
	if s == nil {
		return nil
	}
	out := make([]uint32, 0, len(s.ASNs))
	seen := map[uint32]struct{}{}
	for _, row := range s.ASNs {
		if row.ASN == 0 {
			continue
		}
		if _, ok := seen[row.ASN]; ok {
			continue
		}
		seen[row.ASN] = struct{}{}
		out = append(out, row.ASN)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func feedEntitySidecarHasCountries(sidecar *feedEntitySidecar, targets map[string]struct{}) bool {
	if sidecar == nil || len(sidecar.Countries) == 0 {
		return false
	}
	for _, row := range sidecar.Countries {
		code := strings.ToUpper(strings.TrimSpace(row.Code))
		if code == "" {
			continue
		}
		if _, ok := targets[code]; ok {
			return true
		}
	}
	return false
}

func feedEntitySidecarHasASNs(sidecar *feedEntitySidecar, targets map[uint32]struct{}) bool {
	if sidecar == nil || len(sidecar.ASNs) == 0 {
		return false
	}
	for _, row := range sidecar.ASNs {
		if row.ASN == 0 {
			continue
		}
		if _, ok := targets[row.ASN]; ok {
			return true
		}
	}
	return false
}

func (e *Engine) buildSingleFeedEntitySidecar(name string, view entityOutputView, resolver *effectiveEntryResolver, geoProvider, asnProvider string, geoPrepared *geoPreparedProvider, asnDB *asnloc.Database, setCache *latestSetCache) (*feedEntitySidecar, error) {
	entry := resolver.entryFromSnapshot(name)
	if entry == nil || !e.isPublicFeedName(name) {
		return nil, nil
	}
	src := e.lookupSource(name)
	if !detailSurfaceEligible(e.cfg, src) {
		return nil, nil
	}

	sidecar := &feedEntitySidecar{
		Feed:         name,
		Category:     src.Category,
		Provenance:   string(publicProvenance(src)),
		Maintainer:   entry.Maintainer,
		UniqueIPs:    entry.UniqueIPs,
		LastChangeTS: entry.SourceDate,
		GeoProvider:  geoProvider,
		ASNProvider:  asnProvider,
	}

	countriesByCode := map[string]*feedEntityCountryContribution{}
	if geoProvider != "" {
		payload, err := view.countryComparison(name, geoProvider)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if payload != nil {
			for _, country := range payload.Countries {
				code := strings.ToUpper(strings.TrimSpace(country.Code))
				if code == "" || country.Value == 0 {
					continue
				}
				row := &feedEntityCountryContribution{
					Code:          code,
					AttributedIPs: country.Value,
				}
				countriesByCode[code] = row
			}
		}
	}

	asnsByNumber := map[uint32]*feedEntityASNContribution{}
	if asnProvider != "" {
		for _, row := range view.topASNs(name, asnProvider) {
			if row.ASN == 0 || row.Count == 0 {
				continue
			}
			asnsByNumber[row.ASN] = &feedEntityASNContribution{
				ASN:           row.ASN,
				Name:          row.Name,
				AttributedIPs: row.Count,
			}
		}
	}

	if geoPrepared != nil && asnDB != nil && len(countriesByCode) > 0 && len(asnsByNumber) > 0 {
		if setCache == nil {
			setCache = newLatestSetCache(e)
			defer setCache.CloseAll(e.logger)
		}
		latest, err := setCache.Open(name)
		if err != nil {
			return nil, fmt.Errorf("open latest set for entity sidecar %s: %w", name, err)
		}
		counts, namesByASN, stats, err := countCountryASNJointSource(latest.RangeSource, asnDB, geoPrepared)
		if err != nil {
			return nil, fmt.Errorf("count country/asn attribution for %s: %w", name, err)
		}
		e.observeRunCounter("entity.sidecar_build.feed", 1, 0)
		e.observeRunCounter("entity.sidecar_build.source_ranges", stats.sourceRanges, 0)
		e.observeRunCounter("entity.sidecar_build.geo_segments", stats.geoSegments, 0)
		e.observeRunCounter("entity.sidecar_build.asn_lookups", stats.asnLookups, 0)
		e.observeRunCounter("entity.sidecar_build.country_asn_hits", stats.countryASNHits, 0)
		if ioErr := checkFileSetErr(latest.RangeSource, name, e.logger); ioErr != nil {
			return nil, ioErr
		}

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
			country.ASNs = rows
			if country.AttributedIPs == 0 {
				country.AttributedIPs = total
			}
		}
		for asn, row := range asnsByNumber {
			if row.Name == "" {
				for _, country := range countriesByCode {
					for _, joint := range country.ASNs {
						if joint.ASN == asn && joint.Name != "" {
							row.Name = joint.Name
							break
						}
					}
					if row.Name != "" {
						break
					}
				}
			}
			if row.AttributedIPs == 0 {
				var total uint64
				for _, country := range countriesByCode {
					for _, joint := range country.ASNs {
						if joint.ASN == asn {
							total += joint.Count
						}
					}
				}
				row.AttributedIPs = total
			}
		}
	}

	sidecar.Countries = make([]feedEntityCountryContribution, 0, len(countriesByCode))
	for _, row := range countriesByCode {
		if row == nil || row.AttributedIPs == 0 {
			continue
		}
		sidecar.Countries = append(sidecar.Countries, *row)
	}
	sort.Slice(sidecar.Countries, func(i, j int) bool { return sidecar.Countries[i].Code < sidecar.Countries[j].Code })

	sidecar.ASNs = make([]feedEntityASNContribution, 0, len(asnsByNumber))
	for _, row := range asnsByNumber {
		if row == nil || row.ASN == 0 || row.AttributedIPs == 0 {
			continue
		}
		sidecar.ASNs = append(sidecar.ASNs, *row)
	}
	sort.Slice(sidecar.ASNs, func(i, j int) bool { return sidecar.ASNs[i].ASN < sidecar.ASNs[j].ASN })

	if len(sidecar.Countries) == 0 && len(sidecar.ASNs) == 0 {
		return nil, nil
	}
	return sidecar, nil
}

func (e *Engine) buildSelectedEntityDetailSidecarsFromFeedSidecars(sidecars map[string]*feedEntitySidecar, targetCountries map[string]struct{}, targetASNs map[uint32]struct{}, full bool) (map[string]*countryDetailSidecar, map[uint32]*asnDetailSidecar, error) {
	if e == nil || e.cfg == nil {
		return nil, nil, fmt.Errorf("engine is not configured")
	}

	geoProvider := HomeSummaryProvider{
		Name:  e.preferredGeoProvider(),
		Label: providerDisplayLabel(e.lookupSource(e.preferredGeoProvider())),
	}
	asnProvider := HomeSummaryProvider{
		Name:  e.preferredASNProvider(),
		Label: providerDisplayLabel(e.lookupSource(e.preferredASNProvider())),
	}

	countryBuilders := map[string]*countryDetailBuilder{}
	asnBuilders := map[uint32]*asnDetailBuilder{}
	if !full {
		for code := range targetCountries {
			countryBuilders[code] = newCountryDetailBuilder(code)
		}
		for asn := range targetASNs {
			asnBuilders[asn] = newASNDetailBuilder(asn)
		}
	}

	feedNames := make([]string, 0, len(sidecars))
	for name := range sidecars {
		feedNames = append(feedNames, name)
	}
	slices.Sort(feedNames)

	for _, name := range feedNames {
		sidecar := sidecars[name]
		if sidecar == nil {
			continue
		}
		needCountryDetail := full || feedEntitySidecarHasCountries(sidecar, targetCountries)
		needASNDetail := full || feedEntitySidecarHasASNs(sidecar, targetASNs)
		if !needCountryDetail && !needASNDetail {
			continue
		}
		index := indexFeedEntitySidecar(sidecar)
		maintainerURL := sourceMaintainerURL(e.lookupSource(name))

		if needCountryDetail {
			for _, country := range sidecar.Countries {
				code := strings.ToUpper(strings.TrimSpace(country.Code))
				if code == "" {
					continue
				}
				if !full {
					if _, ok := targetCountries[code]; !ok {
						continue
					}
				}
				builder := countryBuilders[code]
				if builder == nil {
					builder = newCountryDetailBuilder(code)
					countryBuilders[code] = builder
				}
				builder.addFeed(sidecar.countryRow(country), maintainerURL)
				for _, row := range country.ASNs {
					builder.addASN(row.ASN, row.Name, row.Count)
				}
			}
		}

		if needASNDetail {
			for _, row := range sidecar.ASNs {
				if row.ASN == 0 || row.AttributedIPs == 0 {
					continue
				}
				if !full {
					if _, ok := targetASNs[row.ASN]; !ok {
						continue
					}
				}
				builder := asnBuilders[row.ASN]
				if builder == nil {
					builder = newASNDetailBuilder(row.ASN)
					asnBuilders[row.ASN] = builder
				}
				if builder.name == "" && row.Name != "" {
					builder.name = row.Name
				}
				builder.addFeed(sidecar.asnRow(row), maintainerURL, row.Name)
			}
			for _, row := range sidecar.ASNs {
				if row.ASN == 0 {
					continue
				}
				if !full {
					if _, ok := targetASNs[row.ASN]; !ok {
						continue
					}
				}
				builder := asnBuilders[row.ASN]
				if builder == nil {
					continue
				}
				for _, country := range index.asnCountries(row.ASN) {
					builder.addCountry(country.code, country.count)
				}
			}
		}
	}

	countrySidecars := make(map[string]*countryDetailSidecar, len(countryBuilders))
	for code, builder := range countryBuilders {
		if sidecar := builder.build(geoProvider, asnProvider); sidecar != nil {
			countrySidecars[code] = sidecar
		}
	}

	asnSidecars := make(map[uint32]*asnDetailSidecar, len(asnBuilders))
	for asn, builder := range asnBuilders {
		if sidecar := builder.build(asnProvider, geoProvider); sidecar != nil {
			asnSidecars[asn] = sidecar
		}
	}
	return countrySidecars, asnSidecars, nil
}

func (e *Engine) buildCountryIndexFromFeedSidecars(sidecars map[string]*feedEntitySidecar) *CountryIndexPayload {
	payload := e.emptyCountryIndexPayload()
	rows := map[string]*CountryIndexEntry{}
	for _, sidecar := range sidecars {
		if sidecar == nil {
			continue
		}
		for _, country := range sidecar.Countries {
			code := strings.ToUpper(strings.TrimSpace(country.Code))
			if code == "" || country.AttributedIPs == 0 {
				continue
			}
			row := rows[code]
			if row == nil {
				row = &CountryIndexEntry{Code: code}
				rows[code] = row
			}
			row.FeedCount++
			row.AttributedIPs += country.AttributedIPs
		}
	}
	payload.Countries = make([]CountryIndexEntry, 0, len(rows))
	for _, row := range rows {
		payload.Countries = append(payload.Countries, *row)
	}
	sort.Slice(payload.Countries, func(i, j int) bool {
		if payload.Countries[i].FeedCount != payload.Countries[j].FeedCount {
			return payload.Countries[i].FeedCount > payload.Countries[j].FeedCount
		}
		if payload.Countries[i].AttributedIPs != payload.Countries[j].AttributedIPs {
			return payload.Countries[i].AttributedIPs > payload.Countries[j].AttributedIPs
		}
		return payload.Countries[i].Code < payload.Countries[j].Code
	})
	return payload
}

func (e *Engine) buildASNIndexFromFeedSidecars(sidecars map[string]*feedEntitySidecar) *ASNIndexPayload {
	payload := e.emptyASNIndexPayload()
	rows := map[uint32]*ASNIndexEntry{}
	for _, sidecar := range sidecars {
		if sidecar == nil {
			continue
		}
		for _, asn := range sidecar.ASNs {
			if asn.ASN == 0 || asn.AttributedIPs == 0 {
				continue
			}
			row := rows[asn.ASN]
			if row == nil {
				row = &ASNIndexEntry{ASN: asn.ASN, Name: asn.Name}
				rows[asn.ASN] = row
			}
			if row.Name == "" && asn.Name != "" {
				row.Name = asn.Name
			}
			row.FeedCount++
			row.AttributedIPs += asn.AttributedIPs
		}
	}
	payload.ASNs = make([]ASNIndexEntry, 0, len(rows))
	for _, row := range rows {
		payload.ASNs = append(payload.ASNs, *row)
	}
	sort.Slice(payload.ASNs, func(i, j int) bool {
		if payload.ASNs[i].FeedCount != payload.ASNs[j].FeedCount {
			return payload.ASNs[i].FeedCount > payload.ASNs[j].FeedCount
		}
		if payload.ASNs[i].AttributedIPs != payload.ASNs[j].AttributedIPs {
			return payload.ASNs[i].AttributedIPs > payload.ASNs[j].AttributedIPs
		}
		return payload.ASNs[i].ASN < payload.ASNs[j].ASN
	})
	return payload
}
