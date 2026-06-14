package engine

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

type criticalFeedWriter struct {
	e          *Engine
	name       string
	datasets   *criticalDatasets
	outDir     string
	src        *closableSource
	feedIPs    uint64
	feedFilter rangeOverlapFilter

	totalSet           *iprange.IPSet
	tierSets           map[string]*iprange.IPSet
	tierProviderCounts map[string]int
	providerPayloads   []criticalProviderOverlapJSON
}

func (e *Engine) writeCriticalInfrastructureForFeed(name string, datasets *criticalDatasets, outDir string, setCache *latestSetCache) ([]string, error) {
	writer, err := e.newCriticalFeedWriter(name, datasets, outDir, setCache)
	if err != nil || writer == nil {
		return nil, err
	}
	return writer.write()
}

func (e *Engine) newCriticalFeedWriter(name string, datasets *criticalDatasets, outDir string, setCache *latestSetCache) (*criticalFeedWriter, error) {
	src, err := setCache.Open(name)
	if err != nil {
		e.logger.Warn("critical infrastructure comparison skipped: cannot open set",
			"set", name, "error", err)
		return nil, nil
	}
	feedIPs := src.UniqueIPs()
	if checkErr := checkFileSetErr(src.RangeSource, name, e.logger); checkErr != nil {
		return nil, checkErr
	}
	return &criticalFeedWriter{
		e:                  e,
		name:               name,
		datasets:           datasets,
		outDir:             outDir,
		src:                src,
		feedIPs:            feedIPs,
		feedFilter:         buildRangeOverlapFilter(src.RangeSource),
		totalSet:           iprange.New(name + "_critical_infrastructure"),
		tierSets:           map[string]*iprange.IPSet{},
		tierProviderCounts: map[string]int{},
		providerPayloads:   make([]criticalProviderOverlapJSON, 0, len(datasets.Names)),
	}, nil
}

func (w *criticalFeedWriter) write() ([]string, error) {
	if err := w.writeProviderPayloads(); err != nil {
		return nil, err
	}
	sortCriticalProviderPayloads(w.providerPayloads)
	payload, err := w.aggregatePayload()
	if err != nil {
		return nil, err
	}
	tiers := criticalOverlapTiersFromAggregate(payload)
	if err := w.writeAggregatePayload(payload); err != nil {
		return nil, err
	}
	return tiers, nil
}

func (w *criticalFeedWriter) writeProviderPayloads() error {
	for _, providerName := range w.datasets.Names {
		provider := w.datasets.Providers[providerName]
		if provider == nil || provider.Set == nil {
			continue
		}
		if err := w.writeProviderPayload(providerName, provider); err != nil {
			return err
		}
	}
	return nil
}

func (w *criticalFeedWriter) writeProviderPayload(providerName string, provider *criticalProviderSet) error {
	if err := w.checkFeedSet(); err != nil {
		return err
	}
	if err := w.checkProviderSet(providerName, provider); err != nil {
		return err
	}
	criticalIPs, err := w.scanProviderOverlap(providerName, provider)
	if err != nil {
		return err
	}
	if err := w.checkFeedSet(); err != nil {
		return err
	}
	if err := w.checkProviderSet(providerName, provider); err != nil {
		return err
	}

	payload := w.providerPayload(provider, criticalIPs)
	if err := w.e.writeCriticalProviderPayload(w.outDir, w.name, providerName, payload); err != nil {
		return err
	}
	if criticalIPs > 0 {
		w.providerPayloads = append(w.providerPayloads, payload)
	}
	return nil
}

func (w *criticalFeedWriter) scanProviderOverlap(providerName string, provider *criticalProviderSet) (uint64, error) {
	if rangeOverlapFiltersDisjoint(w.feedFilter, provider.overlapFilter) {
		w.e.observeRunCounter("critical.overlap_skipped_filter", 1, 0)
		return 0, nil
	}
	var criticalIPs uint64
	countedProvider := false
	for r := range iprange.IntersectIter(w.src.RangeSource, provider.Set) {
		criticalIPs += r.Size()
		if !countedProvider {
			countedProvider = true
			w.ensureTierSet(provider.Meta.Tier)
		}
		if err := w.totalSet.AddRange(r); err != nil {
			return 0, fmt.Errorf("critical infrastructure aggregate add for %s/%s: %w", w.name, providerName, err)
		}
		if err := w.tierSets[provider.Meta.Tier].AddRange(r); err != nil {
			return 0, fmt.Errorf("critical infrastructure tier aggregate add for %s/%s: %w", w.name, providerName, err)
		}
	}
	return criticalIPs, nil
}

func (w *criticalFeedWriter) ensureTierSet(tier string) {
	w.tierProviderCounts[tier]++
	if w.tierSets[tier] == nil {
		w.tierSets[tier] = iprange.New(w.name + "_critical_" + tier)
	}
}

func (w *criticalFeedWriter) providerPayload(provider *criticalProviderSet, criticalIPs uint64) criticalProviderOverlapJSON {
	payload := criticalProviderOverlapJSON{
		Provider:      provider.Meta,
		ProviderSetID: w.datasets.ProviderSetID,
		FeedIPs:       w.feedIPs,
		CriticalIPs:   criticalIPs,
	}
	if w.feedIPs > 0 {
		payload.Percent = 100 * float64(criticalIPs) / float64(w.feedIPs)
	}
	return payload
}

func (w *criticalFeedWriter) aggregatePayload() (criticalAggregateJSON, error) {
	w.totalSet.Optimize()
	payload := criticalAggregateJSON{
		Feed:                w.name,
		Family:              criticalFeedFamily(w.e.cfg, w.name),
		FeedIPs:             w.feedIPs,
		CriticalIPs:         w.totalSet.UniqueCount(),
		Complete:            len(w.datasets.Missing) == 0,
		ProviderSetID:       w.datasets.ProviderSetID,
		ConfiguredProviders: append([]string(nil), w.datasets.Configured...),
		MissingProviders:    append([]criticalMissingProviderJSON(nil), w.datasets.Missing...),
		Providers:           w.providerPayloads,
	}
	if w.feedIPs > 0 {
		payload.Percent = 100 * float64(payload.CriticalIPs) / float64(w.feedIPs)
	}
	payload.Tiers = w.tierSummaries()
	asnContext, err := w.e.criticalASNContextForFeed(w.name, w.feedIPs, w.outDir)
	if err != nil {
		return criticalAggregateJSON{}, err
	}
	payload.ASNContext = asnContext
	return payload, nil
}

func (w *criticalFeedWriter) tierSummaries() []criticalTierSummaryJSON {
	out := make([]criticalTierSummaryJSON, 0, len(w.tierSets))
	for _, tier := range config.CriticalTiers() {
		tierSet := w.tierSets[tier]
		if tierSet == nil {
			continue
		}
		tierSet.Optimize()
		count := tierSet.UniqueCount()
		summary := criticalTierSummaryJSON{
			Tier:        tier,
			CriticalIPs: count,
			Providers:   w.tierProviderCounts[tier],
		}
		if w.feedIPs > 0 {
			summary.Percent = 100 * float64(count) / float64(w.feedIPs)
		}
		out = append(out, summary)
	}
	return out
}

func (w *criticalFeedWriter) writeAggregatePayload(payload criticalAggregateJSON) error {
	data, err := jsonMarshalTabIndent(payload)
	if err != nil {
		return err
	}
	outPath := filepath.Join(w.outDir, w.name+"_critical_infrastructure.json")
	return writeFileAtomicAt(outPath, append(data, '\n'), generatedFileMode, w.e.feedProcessingTimestamp(w.name))
}

func (w *criticalFeedWriter) checkFeedSet() error {
	return checkFileSetErr(w.src.RangeSource, w.name, w.e.logger)
}

func (w *criticalFeedWriter) checkProviderSet(providerName string, provider *criticalProviderSet) error {
	return checkFileSetErr(provider.Set, providerName, w.e.logger)
}

func sortCriticalProviderPayloads(payloads []criticalProviderOverlapJSON) {
	sort.Slice(payloads, func(i, j int) bool {
		if payloads[i].CriticalIPs != payloads[j].CriticalIPs {
			return payloads[i].CriticalIPs > payloads[j].CriticalIPs
		}
		return payloads[i].Provider.Name < payloads[j].Provider.Name
	})
}
