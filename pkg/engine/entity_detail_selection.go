package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/firehol/update-ipsets/pkg/config"
)

type selectedEntityDetailSidecarBuilder struct {
	e               *Engine
	snapshot        operationSnapshot
	sidecars        map[string]*feedEntitySidecar
	targetCountries map[string]struct{}
	targetASNs      map[uint32]struct{}
	full            bool

	geoProvider HomeSummaryProvider
	asnProvider HomeSummaryProvider

	countryBuilders map[string]*countryDetailBuilder
	asnBuilders     map[uint32]*asnDetailBuilder
}

func (e *Engine) newSelectedEntityDetailSidecarBuilder(sidecars map[string]*feedEntitySidecar, targetCountries map[string]struct{}, targetASNs map[uint32]struct{}, full bool) (*selectedEntityDetailSidecarBuilder, error) {
	snap := e.operationSnapshot()
	if e == nil || snap.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	builder := &selectedEntityDetailSidecarBuilder{
		e:               e,
		snapshot:        snap,
		sidecars:        sidecars,
		targetCountries: targetCountries,
		targetASNs:      targetASNs,
		full:            full,
		geoProvider:     homeSummaryProviderForConfig(snap.cfg, preferredGeoProviderForConfig(snap.cfg)),
		asnProvider:     homeSummaryProviderForConfig(snap.cfg, preferredASNProviderForConfig(snap.cfg)),
		countryBuilders: map[string]*countryDetailBuilder{},
		asnBuilders:     map[uint32]*asnDetailBuilder{},
	}
	builder.initTargetBuilders()
	return builder, nil
}

func (e *Engine) homeSummaryProvider(name string) HomeSummaryProvider {
	return homeSummaryProviderForConfig(e.Config(), name)
}

func homeSummaryProviderForConfig(cfg *config.Config, name string) HomeSummaryProvider {
	return HomeSummaryProvider{
		Name:  name,
		Label: providerDisplayLabel(lookupSourceForConfig(cfg, name)),
	}
}

func (b *selectedEntityDetailSidecarBuilder) initTargetBuilders() {
	if b.full {
		return
	}
	for code := range b.targetCountries {
		b.countryBuilders[code] = newCountryDetailBuilder(code)
	}
	for asn := range b.targetASNs {
		b.asnBuilders[asn] = newASNDetailBuilder(asn)
	}
}

func (b *selectedEntityDetailSidecarBuilder) build() (map[string]*countryDetailSidecar, map[uint32]*asnDetailSidecar, error) {
	for _, name := range sortedFeedEntitySidecarNames(b.sidecars) {
		b.addFeedSidecar(name, b.sidecars[name])
	}
	return b.countrySidecars(), b.asnSidecars(), nil
}

func sortedFeedEntitySidecarNames(sidecars map[string]*feedEntitySidecar) []string {
	names := make([]string, 0, len(sidecars))
	for name := range sidecars {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (b *selectedEntityDetailSidecarBuilder) addFeedSidecar(name string, sidecar *feedEntitySidecar) {
	if sidecar == nil {
		return
	}
	needCountryDetail := b.full || feedEntitySidecarHasCountries(sidecar, b.targetCountries)
	needASNDetail := b.full || feedEntitySidecarHasASNs(sidecar, b.targetASNs)
	if !needCountryDetail && !needASNDetail {
		return
	}
	index := indexFeedEntitySidecar(sidecar)
	maintainerURL := sourceMaintainerURL(lookupSourceForConfig(b.snapshot.cfg, name))
	if needCountryDetail {
		b.addCountryDetails(sidecar, maintainerURL)
	}
	if needASNDetail {
		b.addASNFeedDetails(sidecar, maintainerURL)
		b.addASNCountryDetails(sidecar, index)
	}
}

func (b *selectedEntityDetailSidecarBuilder) addCountryDetails(sidecar *feedEntitySidecar, maintainerURL string) {
	for _, country := range sidecar.Countries {
		code := strings.ToUpper(strings.TrimSpace(country.Code))
		if code == "" {
			continue
		}
		if !b.full {
			if _, ok := b.targetCountries[code]; !ok {
				continue
			}
		}
		builder := b.countryBuilder(code)
		builder.addFeed(sidecar.countryRow(country), maintainerURL)
		for _, row := range country.ASNs {
			builder.addASN(row.ASN, row.Name, row.Count)
		}
	}
}

func (b *selectedEntityDetailSidecarBuilder) addASNFeedDetails(sidecar *feedEntitySidecar, maintainerURL string) {
	for _, row := range sidecar.ASNs {
		if row.ASN == 0 || row.AttributedIPs == 0 {
			continue
		}
		if !b.full {
			if _, ok := b.targetASNs[row.ASN]; !ok {
				continue
			}
		}
		builder := b.asnBuilder(row.ASN)
		if builder.name == "" && row.Name != "" {
			builder.name = row.Name
		}
		builder.addFeed(sidecar.asnRow(row), maintainerURL, row.Name)
	}
}

func (b *selectedEntityDetailSidecarBuilder) addASNCountryDetails(sidecar *feedEntitySidecar, index feedEntitySidecarIndex) {
	for _, row := range sidecar.ASNs {
		if row.ASN == 0 {
			continue
		}
		if !b.full {
			if _, ok := b.targetASNs[row.ASN]; !ok {
				continue
			}
		}
		builder := b.asnBuilders[row.ASN]
		if builder == nil {
			continue
		}
		for _, country := range index.asnCountries(row.ASN) {
			builder.addCountry(country.code, country.count)
		}
	}
}

func (b *selectedEntityDetailSidecarBuilder) countryBuilder(code string) *countryDetailBuilder {
	builder := b.countryBuilders[code]
	if builder == nil {
		builder = newCountryDetailBuilder(code)
		b.countryBuilders[code] = builder
	}
	return builder
}

func (b *selectedEntityDetailSidecarBuilder) asnBuilder(asn uint32) *asnDetailBuilder {
	builder := b.asnBuilders[asn]
	if builder == nil {
		builder = newASNDetailBuilder(asn)
		b.asnBuilders[asn] = builder
	}
	return builder
}

func (b *selectedEntityDetailSidecarBuilder) countrySidecars() map[string]*countryDetailSidecar {
	sidecars := make(map[string]*countryDetailSidecar, len(b.countryBuilders))
	for code, builder := range b.countryBuilders {
		if sidecar := builder.build(b.geoProvider, b.asnProvider); sidecar != nil {
			sidecars[code] = sidecar
		}
	}
	return sidecars
}

func (b *selectedEntityDetailSidecarBuilder) asnSidecars() map[uint32]*asnDetailSidecar {
	sidecars := make(map[uint32]*asnDetailSidecar, len(b.asnBuilders))
	for asn, builder := range b.asnBuilders {
		if sidecar := builder.build(b.asnProvider, b.geoProvider); sidecar != nil {
			sidecars[asn] = sidecar
		}
	}
	return sidecars
}
