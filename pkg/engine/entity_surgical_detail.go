package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func (e *Engine) patchCountrySidecarForFeedDeltas(code string, deltas []feedEntityDelta) (*countryDetailSidecar, bool, error) {
	patchStart := time.Now()
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, false, nil
	}
	defer func() {
		e.observeRunOperation("entity.refresh.country_patch", time.Since(patchStart))
	}()
	path := filepath.Join(e.entityCountriesDir(), code+".json")
	start := time.Now()
	sidecar, err := loadCountryDetailSidecar(path)
	var original *countryDetailSidecar
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, false, err
		}
		sidecar = e.emptyCountryDetailSidecar(code)
	} else {
		original = sidecar
		var bytes int64
		if info, statErr := os.Stat(path); statErr == nil {
			bytes = info.Size()
		}
		e.observeRunCounter("entity.refresh.country_sidecar_read", 1, bytes)
		e.observeRunOperation("entity.refresh.country_sidecar_read", time.Since(start))
	}
	feeds := removeCountryFeedRows(sidecar.Feeds, deltas)
	asnTotals := countryASNAggregatesFromSidecar(sidecar)
	for _, delta := range deltas {
		if contribution, ok := delta.old.countryActorContribution(code, delta.oldIndex); ok {
			if err := applyCountryJointRows(asnTotals, contribution.asns, -1); err != nil {
				return nil, false, fmt.Errorf("subtract old %s contribution from country %s: %w", delta.name, code, err)
			}
		}
		if contribution, ok := delta.new.countryActorContribution(code, delta.newIndex); ok {
			feeds = append(feeds, contribution.feed)
			if err := applyCountryJointRows(asnTotals, contribution.asns, 1); err != nil {
				return nil, false, fmt.Errorf("add new %s contribution to country %s: %w", delta.name, code, err)
			}
		}
	}
	updated := e.rebuildCountrySidecarFromParts(code, sidecar, feeds, asnTotals)
	return updated, !reflect.DeepEqual(original, updated), nil
}

func (e *Engine) patchASNSidecarForFeedDeltas(asn uint32, deltas []feedEntityDelta) (*asnDetailSidecar, bool, error) {
	patchStart := time.Now()
	if asn == 0 {
		return nil, false, nil
	}
	defer func() {
		e.observeRunOperation("entity.refresh.asn_patch", time.Since(patchStart))
	}()
	path := filepath.Join(e.entityASNsDir(), strconv.FormatUint(uint64(asn), 10)+".json")
	start := time.Now()
	sidecar, err := loadASNDetailSidecar(path)
	var original *asnDetailSidecar
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, false, err
		}
		sidecar = e.emptyASNDetailSidecar(asn)
	} else {
		original = sidecar
		var bytes int64
		if info, statErr := os.Stat(path); statErr == nil {
			bytes = info.Size()
		}
		e.observeRunCounter("entity.refresh.asn_sidecar_read", 1, bytes)
		e.observeRunOperation("entity.refresh.asn_sidecar_read", time.Since(start))
	}
	feeds := removeASNFeedRows(sidecar.Feeds, deltas)
	countryTotals := asnCountryAggregatesFromSidecar(sidecar)
	for _, delta := range deltas {
		if contribution, ok := delta.old.asnActorContribution(asn, delta.oldIndex); ok {
			if err := applyASNCountryRows(countryTotals, contribution.countries, -1); err != nil {
				return nil, false, fmt.Errorf("subtract old %s contribution from ASN %d: %w", delta.name, asn, err)
			}
		}
		if contribution, ok := delta.new.asnActorContribution(asn, delta.newIndex); ok {
			feeds = append(feeds, contribution.feed)
			if sidecar.Name == "" && contribution.name != "" {
				sidecar.Name = contribution.name
			}
			if err := applyASNCountryRows(countryTotals, contribution.countries, 1); err != nil {
				return nil, false, fmt.Errorf("add new %s contribution to ASN %d: %w", delta.name, asn, err)
			}
		}
	}
	updated := e.rebuildASNSidecarFromParts(asn, sidecar, feeds, countryTotals)
	return updated, !reflect.DeepEqual(original, updated), nil
}

func (e *Engine) rebuildCountrySidecarFromParts(code string, base *countryDetailSidecar, feeds []countryDetailFeedBase, asnTotals map[uint32]*countryDetailASNAggregate) *countryDetailSidecar {
	if len(feeds) == 0 {
		return nil
	}
	if base == nil {
		base = e.emptyCountryDetailSidecar(code)
	}
	builder := newCountryDetailBuilder(code)
	for _, row := range feeds {
		builder.addFeed(row, sourceMaintainerURL(e.lookupSource(row.Name)))
	}
	builder.asnTotals = asnTotals
	return builder.build(base.Provider, base.ASNProvider)
}

func (e *Engine) rebuildASNSidecarFromParts(asn uint32, base *asnDetailSidecar, feeds []asnDetailFeedBase, countryTotals map[string]*asnDetailCountryAggregate) *asnDetailSidecar {
	if len(feeds) == 0 {
		return nil
	}
	if base == nil {
		base = e.emptyASNDetailSidecar(asn)
	}
	builder := newASNDetailBuilder(asn)
	builder.name = base.Name
	builder.description = base.Description
	for _, row := range feeds {
		builder.addFeed(row, sourceMaintainerURL(e.lookupSource(row.Name)), builder.name)
	}
	builder.countryTotals = countryTotals
	builder.distributionCounts = make(map[string]uint64, len(countryTotals))
	for code, agg := range countryTotals {
		if agg == nil || agg.attributedIPs == 0 {
			continue
		}
		builder.distributionCounts[code] = agg.attributedIPs
		builder.totalMapped += agg.attributedIPs
	}
	return builder.build(base.Provider, base.GeoProvider)
}

func removeCountryFeedRows(rows []countryDetailFeedBase, deltas []feedEntityDelta) []countryDetailFeedBase {
	remove := make(map[string]struct{}, len(deltas))
	for _, delta := range deltas {
		remove[delta.name] = struct{}{}
	}
	out := make([]countryDetailFeedBase, 0, len(rows))
	for _, row := range rows {
		if _, ok := remove[row.Name]; ok {
			continue
		}
		out = append(out, row)
	}
	return out
}

func removeASNFeedRows(rows []asnDetailFeedBase, deltas []feedEntityDelta) []asnDetailFeedBase {
	remove := make(map[string]struct{}, len(deltas))
	for _, delta := range deltas {
		remove[delta.name] = struct{}{}
	}
	out := make([]asnDetailFeedBase, 0, len(rows))
	for _, row := range rows {
		if _, ok := remove[row.Name]; ok {
			continue
		}
		out = append(out, row)
	}
	return out
}

func countryASNAggregatesFromSidecar(sidecar *countryDetailSidecar) map[uint32]*countryDetailASNAggregate {
	out := map[uint32]*countryDetailASNAggregate{}
	if sidecar == nil {
		return out
	}
	for _, row := range sidecar.TopASNs {
		if row.ASN == 0 || row.AttributedIPs == 0 || row.FeedCount <= 0 {
			continue
		}
		out[row.ASN] = &countryDetailASNAggregate{
			name:          row.Name,
			feedCount:     row.FeedCount,
			attributedIPs: row.AttributedIPs,
		}
	}
	return out
}

func asnCountryAggregatesFromSidecar(sidecar *asnDetailSidecar) map[string]*asnDetailCountryAggregate {
	out := map[string]*asnDetailCountryAggregate{}
	if sidecar == nil {
		return out
	}
	for _, row := range sidecar.TopCountries {
		code := strings.ToUpper(strings.TrimSpace(row.Code))
		if code == "" || row.AttributedIPs == 0 || row.FeedCount <= 0 {
			continue
		}
		out[code] = &asnDetailCountryAggregate{
			feedCount:     row.FeedCount,
			attributedIPs: row.AttributedIPs,
		}
	}
	return out
}

func applyCountryJointRows(totals map[uint32]*countryDetailASNAggregate, rows []feedEntityJointASN, sign int) error {
	if len(rows) == 0 {
		return nil
	}
	for _, row := range rows {
		if row.ASN == 0 || row.Count == 0 {
			continue
		}
		agg := totals[row.ASN]
		if sign < 0 {
			if agg == nil || agg.feedCount <= 0 || agg.attributedIPs < row.Count {
				return fmt.Errorf("%w: aggregate underflow for ASN %d", errEntitySurgicalNeedsFullRebuild, row.ASN)
			}
			agg.feedCount--
			agg.attributedIPs -= row.Count
			if agg.feedCount == 0 || agg.attributedIPs == 0 {
				delete(totals, row.ASN)
			}
			continue
		}
		if agg == nil {
			agg = &countryDetailASNAggregate{}
			totals[row.ASN] = agg
		}
		if agg.name == "" && row.Name != "" {
			agg.name = row.Name
		}
		agg.feedCount++
		agg.attributedIPs += row.Count
	}
	return nil
}

func applyASNCountryRows(totals map[string]*asnDetailCountryAggregate, rows []asnCountryDeltaRow, sign int) error {
	for _, row := range rows {
		code := strings.ToUpper(strings.TrimSpace(row.code))
		if code == "" || row.count == 0 {
			continue
		}
		agg := totals[code]
		if sign < 0 {
			if agg == nil || agg.feedCount <= 0 || agg.attributedIPs < row.count {
				return fmt.Errorf("%w: aggregate underflow for country %s", errEntitySurgicalNeedsFullRebuild, code)
			}
			agg.feedCount--
			agg.attributedIPs -= row.count
			if agg.feedCount == 0 || agg.attributedIPs == 0 {
				delete(totals, code)
			}
			continue
		}
		if agg == nil {
			agg = &asnDetailCountryAggregate{}
			totals[code] = agg
		}
		agg.feedCount++
		agg.attributedIPs += row.count
	}
	return nil
}

type asnCountryDeltaRow struct {
	code  string
	count uint64
}
