package engine

import (
	"sort"
	"strings"

	"github.com/firehol/update-ipsets/pkg/asnloc"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

type countryDetailASNAggregate struct {
	name          string
	feedCount     int
	attributedIPs uint64
}

type countryDetailBuilder struct {
	code            string
	feeds           []countryDetailFeedBase
	facets          detailFacetAccumulator
	asnTotals       map[uint32]*countryDetailASNAggregate
	totalAttributed uint64
}

func newCountryDetailBuilder(code string) *countryDetailBuilder {
	return &countryDetailBuilder{
		code:      strings.ToUpper(strings.TrimSpace(code)),
		facets:    newDetailFacetAccumulator(),
		asnTotals: make(map[uint32]*countryDetailASNAggregate),
	}
}

func (b *countryDetailBuilder) addFeed(row countryDetailFeedBase, maintainerURL string) {
	b.feeds = append(b.feeds, row)
	b.totalAttributed += row.AttributedIPs
	b.facets.add(row.Category, row.Maintainer, maintainerURL, row.AttributedIPs)
}

func (b *countryDetailBuilder) addASN(asn uint32, name string, count uint64) {
	if asn == 0 || count == 0 {
		return
	}
	agg := b.asnTotals[asn]
	if agg == nil {
		agg = &countryDetailASNAggregate{}
		b.asnTotals[asn] = agg
	}
	if agg.name == "" && name != "" {
		agg.name = name
	}
	agg.feedCount++
	agg.attributedIPs += count
}

func (b *countryDetailBuilder) build(geoProvider, asnProvider HomeSummaryProvider) *countryDetailSidecar {
	if b == nil || len(b.feeds) == 0 {
		return nil
	}
	sort.Slice(b.feeds, func(i, j int) bool {
		if b.feeds[i].AttributedIPs != b.feeds[j].AttributedIPs {
			return b.feeds[i].AttributedIPs > b.feeds[j].AttributedIPs
		}
		return b.feeds[i].Name < b.feeds[j].Name
	})

	topCategories := b.facets.topCategories()
	topMaintainers := b.facets.topMaintainers()

	topASNs := make([]CountryDetailASN, 0, len(b.asnTotals))
	for asn, agg := range b.asnTotals {
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
		Code:        b.code,
		Provider:    geoProvider,
		ASNProvider: asnProvider,
		Totals: CountryDetailTotals{
			FeedsMatching:       len(b.feeds),
			AttributedIPsInFeed: b.totalAttributed,
			Categories:          b.facets.categoryCount(),
			Maintainers:         b.facets.maintainerCount(),
			ASNs:                len(b.asnTotals),
		},
		Feeds:          b.feeds,
		TopCategories:  topCategories,
		TopMaintainers: topMaintainers,
		TopASNs:        topASNs,
	}
}

type asnDetailCountryAggregate struct {
	feedCount     int
	attributedIPs uint64
}

type asnDetailBuilder struct {
	asn                uint32
	name               string
	description        string
	feeds              []asnDetailFeedBase
	facets             detailFacetAccumulator
	countryTotals      map[string]*asnDetailCountryAggregate
	distributionCounts map[string]uint64
	totalAttributed    uint64
	totalMapped        uint64
}

func newASNDetailBuilder(asn uint32) *asnDetailBuilder {
	return &asnDetailBuilder{
		asn:                asn,
		facets:             newDetailFacetAccumulator(),
		countryTotals:      make(map[string]*asnDetailCountryAggregate),
		distributionCounts: make(map[string]uint64),
	}
}

func (b *asnDetailBuilder) addFeed(row asnDetailFeedBase, maintainerURL, observedName string) {
	b.feeds = append(b.feeds, row)
	b.totalAttributed += row.AttributedIPs
	if b.name == "" && observedName != "" {
		b.name = observedName
	}
	b.facets.add(row.Category, row.Maintainer, maintainerURL, row.AttributedIPs)
}

func (b *asnDetailBuilder) addCountry(code string, count uint64) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" || count == 0 {
		return
	}
	agg := b.countryTotals[code]
	if agg == nil {
		agg = &asnDetailCountryAggregate{}
		b.countryTotals[code] = agg
	}
	agg.feedCount++
	agg.attributedIPs += count
	b.distributionCounts[code] += count
	b.totalMapped += count
}

func (b *asnDetailBuilder) build(asnProvider, geoProvider HomeSummaryProvider) *asnDetailSidecar {
	if b == nil || len(b.feeds) == 0 {
		return nil
	}
	sort.Slice(b.feeds, func(i, j int) bool {
		if b.feeds[i].AttributedIPs != b.feeds[j].AttributedIPs {
			return b.feeds[i].AttributedIPs > b.feeds[j].AttributedIPs
		}
		return b.feeds[i].Name < b.feeds[j].Name
	})

	topCategories := b.facets.topCategories()
	topMaintainers := b.facets.topMaintainers()

	topCountries := make([]ASNDetailCountry, 0, len(b.countryTotals))
	for code, agg := range b.countryTotals {
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

	distribution := make([]CountryValue, 0, len(b.distributionCounts))
	for code, value := range b.distributionCounts {
		distribution = append(distribution, CountryValue{Code: code, Value: value})
	}
	sort.Slice(distribution, func(i, j int) bool {
		if distribution[i].Code != distribution[j].Code {
			return distribution[i].Code < distribution[j].Code
		}
		return distribution[i].Value > distribution[j].Value
	})

	sidecar := &asnDetailSidecar{
		ASN:         b.asn,
		Name:        b.name,
		Description: b.description,
		Provider:    asnProvider,
		GeoProvider: geoProvider,
		Totals: ASNDetailTotals{
			FeedsMatching: len(b.feeds),
			AttributedIPs: b.totalAttributed,
			Categories:    b.facets.categoryCount(),
			Maintainers:   b.facets.maintainerCount(),
			Countries:     len(b.countryTotals),
		},
		Feeds:          b.feeds,
		TopCategories:  topCategories,
		TopMaintainers: topMaintainers,
		TopCountries:   topCountries,
	}
	if len(distribution) > 0 {
		sidecar.CountryDistribution = &CountryComparisonPayload{
			TotalMapped: b.totalMapped,
			Countries:   distribution,
		}
	}
	return sidecar
}

type countryASNJointStats struct {
	sourceRanges   int64
	geoSegments    int64
	asnLookups     int64
	countryASNHits int64
}

func countCountryASNJointSource(src iprange.RangeSource, db *asnloc.Database, prepared *geoPreparedProvider) (map[string]map[uint32]uint64, map[uint32]string, countryASNJointStats, error) {
	var stats countryASNJointStats
	if src == nil || db == nil || prepared == nil || len(prepared.segments) == 0 || len(prepared.countryCodes) == 0 {
		return nil, nil, stats, nil
	}

	counts := make(map[string]map[uint32]uint64)
	names := make(map[uint32]string)
	segmentIndex := 0

	for sourceRange := range src.Iter() {
		stats.sourceRanges++
		for segmentIndex < len(prepared.segments) && prepared.segments[segmentIndex].rng.Hi < sourceRange.Lo {
			segmentIndex++
		}
		idx := segmentIndex
		for idx < len(prepared.segments) {
			segment := prepared.segments[idx]
			if sourceRange.Hi < segment.rng.Lo {
				break
			}
			stats.geoSegments++

			lo := max(sourceRange.Lo, segment.rng.Lo)
			hi := min(sourceRange.Hi, segment.rng.Hi)
			cur := lo
			for {
				stats.asnLookups++
				rec, network, err := db.Lookup(cur)
				if err != nil {
					return nil, nil, stats, err
				}
				end := max(cur, min(network.Hi, hi))
				if rec.ASN != 0 {
					span := uint64(end-cur) + 1
					if rec.Name != "" {
						names[rec.ASN] = rec.Name
					}
					for _, codeIndex := range segment.codes {
						code := prepared.countryCodes[int(codeIndex)]
						countryCounts := counts[code]
						if countryCounts == nil {
							countryCounts = make(map[uint32]uint64)
							counts[code] = countryCounts
						}
						countryCounts[rec.ASN] += span
						stats.countryASNHits++
					}
				}
				if end == ^uint32(0) || end >= hi {
					break
				}
				cur = end + 1
			}

			if sourceRange.Hi <= segment.rng.Hi {
				break
			}
			idx++
		}
		segmentIndex = idx
		if segmentIndex >= len(prepared.segments) {
			break
		}
	}

	return counts, names, stats, nil
}
