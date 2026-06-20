package engine

import (
	"sort"
	"strings"

	"github.com/firehol/update-ipsets/pkg/asnloc"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

type detailCategoryAggregate struct {
	feedCount     int
	attributedIPs uint64
}

type detailMaintainerAggregate struct {
	slug          string
	name          string
	url           string
	feedCount     int
	attributedIPs uint64
}

func detailSurfaceEligible(cfg *config.Config, src *config.Source) bool {
	if src == nil || src.Hidden {
		return false
	}
	if src.HasUse(config.UseGeoIP) || src.HasUse(config.UseASN) {
		return false
	}
	if cfg == nil || !cfg.CategoryIsPublic(src.Category) {
		return false
	}
	return true
}

func countryFilteredRangeSource(src iprange.RangeSource, prepared *geoPreparedProvider, code string) iprange.RangeSource {
	targetIndex := geoPreparedCodeIndex(prepared, strings.ToUpper(strings.TrimSpace(code)))
	if src == nil || prepared == nil || targetIndex < 0 || len(prepared.segments) == 0 {
		return iprange.RangeSourceFromIter(nil, 0)
	}
	targetCode := uint16(targetIndex)
	return iprange.RangeSourceFromIter(
		func(yield func(iprange.Range) bool) {
			segmentIndex := 0
			for sourceRange := range src.Iter() {
				for segmentIndex < len(prepared.segments) && prepared.segments[segmentIndex].rng.Hi < sourceRange.Lo {
					segmentIndex++
				}
				idx := segmentIndex
				for idx < len(prepared.segments) {
					segment := prepared.segments[idx]
					if sourceRange.Hi < segment.rng.Lo {
						break
					}
					if geoPreparedSegmentHasCode(segment, targetCode) {
						lo := max(sourceRange.Lo, segment.rng.Lo)
						hi := min(sourceRange.Hi, segment.rng.Hi)
						if lo <= hi && !yield(iprange.Range{Lo: lo, Hi: hi}) {
							return
						}
					}
					if sourceRange.Hi <= segment.rng.Hi {
						break
					}
					idx++
				}
				segmentIndex = idx
				if segmentIndex >= len(prepared.segments) {
					return
				}
			}
		},
		-1,
	)
}

func countCountriesForASNSource(src iprange.RangeSource, db *asnloc.Database, prepared *geoPreparedProvider, targetASN uint32) ([]CountryValue, uint64, error) {
	if src == nil || db == nil || prepared == nil || targetASN == 0 || len(prepared.segments) == 0 {
		return nil, 0, nil
	}

	counts := make(map[string]uint64)
	var totalMapped uint64
	segmentIndex := 0

	for sourceRange := range src.Iter() {
		for segmentIndex < len(prepared.segments) && prepared.segments[segmentIndex].rng.Hi < sourceRange.Lo {
			segmentIndex++
		}
		idx := segmentIndex
		for idx < len(prepared.segments) {
			segment := prepared.segments[idx]
			if sourceRange.Hi < segment.rng.Lo {
				break
			}

			lo := max(sourceRange.Lo, segment.rng.Lo)
			hi := min(sourceRange.Hi, segment.rng.Hi)
			cur := lo
			for {
				rec, network, err := db.Lookup(cur)
				if err != nil {
					return nil, 0, err
				}
				end := max(cur, min(network.Hi, hi))
				if rec.ASN == targetASN {
					span := uint64(end-cur) + 1
					totalMapped += span
					for _, codeIndex := range segment.codes {
						counts[prepared.countryCodes[codeIndex]] += span
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

	values := make([]CountryValue, 0, len(counts))
	for code, value := range counts {
		values = append(values, CountryValue{Code: code, Value: value})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Code != values[j].Code {
			return values[i].Code < values[j].Code
		}
		return values[i].Value > values[j].Value
	})
	return values, totalMapped, nil
}
