package engine

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
)

type feedEntitySidecarVisitFunc func(name string, sidecar *feedEntitySidecar) error
type feedEntitySidecarWalker func(feedEntitySidecarVisitFunc) error

func (e *Engine) walkCommittedFeedEntitySidecarsWithRuntime(ctx context.Context, rt Runtime, visit feedEntitySidecarVisitFunc) error {
	ctx = nonNilContext(ctx)
	if visit == nil {
		return nil
	}
	paths, err := sortedJSONFiles(entityFeedsDirForRuntime(rt))
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := contextErr(ctx); err != nil {
			return err
		}
		sidecar, err := e.loadFeedEntitySidecar(path)
		if err != nil {
			return err
		}
		name := normalizedFeedEntitySidecarName(strings.TrimSuffix(filepath.Base(path), ".json"), sidecar)
		if name == "" {
			continue
		}
		if err := visit(name, sidecar); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) walkMergedFeedEntitySidecarsWithRuntime(ctx context.Context, rt Runtime, replacements map[string]*feedEntitySidecar, replaceAll bool, visit feedEntitySidecarVisitFunc) error {
	ctx = nonNilContext(ctx)
	if visit == nil {
		return nil
	}
	if replaceAll {
		return walkFeedEntitySidecarMap(ctx, replacements, visit)
	}

	paths, err := sortedJSONFiles(entityFeedsDirForRuntime(rt))
	if err != nil {
		return err
	}
	pathByName := make(map[string]string, len(paths))
	names := make([]string, 0, len(paths)+len(replacements))
	seen := map[string]struct{}{}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		if name == "" {
			continue
		}
		pathByName[name] = path
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	for name := range replacements {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	slices.Sort(names)

	for _, name := range names {
		if err := contextErr(ctx); err != nil {
			return err
		}
		if sidecar, ok := replacements[name]; ok {
			if sidecar == nil {
				continue
			}
			if err := visit(normalizedFeedEntitySidecarName(name, sidecar), sidecar); err != nil {
				return err
			}
			continue
		}
		path := pathByName[name]
		if path == "" {
			continue
		}
		sidecar, err := e.loadFeedEntitySidecar(path)
		if err != nil {
			return err
		}
		visitName := normalizedFeedEntitySidecarName(name, sidecar)
		if visitName == "" {
			continue
		}
		if err := visit(visitName, sidecar); err != nil {
			return err
		}
	}
	return nil
}

func walkFeedEntitySidecarMap(ctx context.Context, sidecars map[string]*feedEntitySidecar, visit feedEntitySidecarVisitFunc) error {
	ctx = nonNilContext(ctx)
	if visit == nil {
		return nil
	}
	for _, name := range sortedFeedEntitySidecarNames(sidecars) {
		if err := contextErr(ctx); err != nil {
			return err
		}
		sidecar := sidecars[name]
		visitName := normalizedFeedEntitySidecarName(name, sidecar)
		if visitName == "" {
			continue
		}
		if err := visit(visitName, sidecar); err != nil {
			return err
		}
	}
	return nil
}

func normalizedFeedEntitySidecarName(fallback string, sidecar *feedEntitySidecar) string {
	if sidecar != nil {
		if name := strings.TrimSpace(sidecar.Feed); name != "" {
			return name
		}
	}
	return strings.TrimSpace(fallback)
}

func (e *Engine) buildSelectedEntityDetailSidecarsFromFeedSidecarWalker(ctx context.Context, snap operationSnapshot, targetCountries map[string]struct{}, targetASNs map[uint32]struct{}, full bool, walk feedEntitySidecarWalker) (map[string]*countryDetailSidecar, map[uint32]*asnDetailSidecar, error) {
	builder, err := e.newSelectedEntityDetailSidecarStreamBuilder(snap, targetCountries, targetASNs, full)
	if err != nil {
		return nil, nil, err
	}
	return builder.buildFromWalker(ctx, walk)
}

func entityFeedPresenceNamesFromSidecarWalker(ctx context.Context, walk feedEntitySidecarWalker) ([]string, error) {
	ctx = nonNilContext(ctx)
	names := []string{}
	if walk == nil {
		return names, nil
	}
	err := walk(func(name string, sidecar *feedEntitySidecar) error {
		if err := contextErr(ctx); err != nil {
			return err
		}
		if !feedEntitySidecarHasEntityPresence(sidecar) {
			return nil
		}
		name = normalizedFeedEntitySidecarName(name, sidecar)
		if name != "" {
			names = append(names, name)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return uniqueNonEmptyStrings(names), nil
}

func stageEntityFeedPresenceIndexFromWalker(ctx context.Context, entityBatch *stagedPublishBatch, walk feedEntitySidecarWalker) error {
	names, err := entityFeedPresenceNamesFromSidecarWalker(ctx, walk)
	if err != nil {
		return err
	}
	return stageEntityFeedPresenceIndex(entityBatch, names)
}

func (e *Engine) buildEntityIndexesFromFeedSidecarWalkerWithSnapshot(ctx context.Context, snap operationSnapshot, walk feedEntitySidecarWalker, buildCountry, buildASN bool) (*CountryIndexPayload, *ASNIndexPayload, error) {
	ctx = nonNilContext(ctx)
	if !buildCountry && !buildASN {
		return nil, nil, nil
	}
	countryRows := map[string]*CountryIndexEntry{}
	asnRows := map[uint32]*ASNIndexEntry{}
	if walk != nil {
		err := walk(func(_ string, sidecar *feedEntitySidecar) error {
			if err := contextErr(ctx); err != nil {
				return err
			}
			if sidecar == nil {
				return nil
			}
			if buildCountry {
				accumulateCountryIndexRows(countryRows, sidecar)
			}
			if buildASN {
				accumulateASNIndexRows(asnRows, sidecar)
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	} else if err := contextErr(ctx); err != nil {
		return nil, nil, err
	}
	var countryIndex *CountryIndexPayload
	if buildCountry {
		countryIndex = e.countryIndexPayloadFromRows(snap, countryRows)
	}
	var asnIndex *ASNIndexPayload
	if buildASN {
		asnIndex = e.asnIndexPayloadFromRows(snap, asnRows)
	}
	return countryIndex, asnIndex, nil
}

func accumulateCountryIndexRows(rows map[string]*CountryIndexEntry, sidecar *feedEntitySidecar) {
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

func accumulateASNIndexRows(rows map[uint32]*ASNIndexEntry, sidecar *feedEntitySidecar) {
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

func (e *Engine) countryIndexPayloadFromRows(snap operationSnapshot, rows map[string]*CountryIndexEntry) *CountryIndexPayload {
	payload := e.emptyCountryIndexPayloadWithSnapshot(snap)
	payload.Countries = make([]CountryIndexEntry, 0, len(rows))
	for _, row := range rows {
		payload.Countries = append(payload.Countries, *row)
	}
	slices.SortFunc(payload.Countries, func(a, b CountryIndexEntry) int {
		if a.FeedCount != b.FeedCount {
			return b.FeedCount - a.FeedCount
		}
		if a.AttributedIPs != b.AttributedIPs {
			if a.AttributedIPs > b.AttributedIPs {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Code, b.Code)
	})
	return payload
}

func (e *Engine) asnIndexPayloadFromRows(snap operationSnapshot, rows map[uint32]*ASNIndexEntry) *ASNIndexPayload {
	payload := e.emptyASNIndexPayloadWithSnapshot(snap)
	payload.ASNs = make([]ASNIndexEntry, 0, len(rows))
	for _, row := range rows {
		payload.ASNs = append(payload.ASNs, *row)
	}
	slices.SortFunc(payload.ASNs, func(a, b ASNIndexEntry) int {
		if a.FeedCount != b.FeedCount {
			return b.FeedCount - a.FeedCount
		}
		if a.AttributedIPs != b.AttributedIPs {
			if a.AttributedIPs > b.AttributedIPs {
				return -1
			}
			return 1
		}
		return int(a.ASN) - int(b.ASN)
	})
	return payload
}

func committedFeedEntitySidecarNamesWithRuntime(rt Runtime) (map[string]struct{}, error) {
	paths, err := sortedJSONFiles(entityFeedsDirForRuntime(rt))
	if err != nil {
		return nil, err
	}
	names := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		if name != "" {
			names[name] = struct{}{}
		}
	}
	return names, nil
}
