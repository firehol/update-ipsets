package engine

import (
	"context"
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

func countryFilteredRangeSource(ctx context.Context, src iprange.RangeSource, prepared *geoPreparedProvider, code string) iprange.RangeSource {
	ctx = nonNilContext(ctx)
	targetIndex := geoPreparedCodeIndex(prepared, strings.ToUpper(strings.TrimSpace(code)))
	if src == nil || prepared == nil || targetIndex < 0 || len(prepared.segments) == 0 {
		return iprange.RangeSourceFromIter(nil, 0)
	}
	targetCode := uint16(targetIndex)
	return iprange.RangeSourceFromIterErr(
		func(yield func(iprange.Range) bool) error {
			return iprange.WalkRangeOverlapsContext(ctx, src, geoPreparedSegmentIndex(prepared.segments), func(overlap iprange.RangeOverlap) bool {
				segment := prepared.segments[overlap.RightIndex]
				if !geoPreparedSegmentHasCode(segment, targetCode) {
					return true
				}
				return yield(overlap.Overlap)
			})
		},
		-1,
	)
}

func countCountriesForASNSource(ctx context.Context, src iprange.RangeSource, db *asnloc.Database, prepared *geoPreparedProvider, targetASN uint32) ([]CountryValue, uint64, error) {
	ctx = nonNilContext(ctx)
	if src == nil || db == nil || prepared == nil || targetASN == 0 || len(prepared.segments) == 0 {
		return nil, 0, nil
	}

	counts := make(map[string]uint64)
	var totalMapped uint64

	var lookupErr error
	err := iprange.WalkRangeOverlapsContext(ctx, src, geoPreparedSegmentIndex(prepared.segments), func(overlap iprange.RangeOverlap) bool {
		segment := prepared.segments[overlap.RightIndex]
		cur := overlap.Overlap.Lo
		hi := overlap.Overlap.Hi
		for {
			rec, network, err := db.Lookup(cur)
			if err != nil {
				lookupErr = err
				return false
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
		return true
	})
	if lookupErr != nil {
		return nil, 0, lookupErr
	}
	if err != nil {
		return nil, 0, err
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
