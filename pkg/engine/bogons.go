package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

// bogonProviderSet is one loaded bogon source, ready to be unioned
// into the bogon_union or used directly to compute per-feed overlap.
// Format mirrors Source.Format and is used to special-case the RFC
// reserved baseline (which has known per-range labels). Sources holds
// the closable backing sources (FileSet handles or in-memory IPSets)
// so the caller can release them after the run.
type bogonProviderSet struct {
	Name          string
	Format        string
	Set           iprange.RangeSource
	overlapFilter rangeOverlapFilter
	sources       []*closableSource
}

// bogonDatasets is the result of loadBogonProviders, keyed by provider
// name. Iterate over Names for deterministic ordering — Go map order
// is non-deterministic and the JSON output sorts on the same key.
type bogonDatasets struct {
	Providers map[string]*bogonProviderSet
	Names     []string
}

// closeAll releases the underlying file handles for every loaded bogon
// provider. Must be called when the run is done with the dataset.
func (b *bogonDatasets) closeAll() {
	if b == nil {
		return
	}
	for _, p := range b.Providers {
		if p == nil {
			continue
		}
		for _, s := range p.sources {
			if s != nil {
				_ = s.Close()
			}
		}
	}
}

// loadBogonSources walks every source in cfg with use:[bogons] and
// materializes its committed on-disk binary set into an iprange.RangeSource.
// Synthetic bogon feeds such as rfc_reserved flow through the exact same
// committed-feed-body path as any real source; the comparison phase does not
// special-case them at load time.
//
// Missing feeds (e.g. a bogon source whose first download has not happened
// yet) log a warning and are skipped instead of failing the run.
func (e *Engine) loadBogonSources(ctx context.Context) (*bogonDatasets, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	bogonSources := e.cfg.SourcesWithUse(config.UseBogons)
	if len(bogonSources) == 0 {
		return &bogonDatasets{Providers: map[string]*bogonProviderSet{}}, nil
	}

	out := &bogonDatasets{Providers: make(map[string]*bogonProviderSet, len(bogonSources))}
	loadOp := e.beginActiveOperation("bogons.load_providers", "", "load", "providers", int64(len(bogonSources)))
	defer loadOp.Finish()
	for _, src := range bogonSources {
		if err := contextErr(ctx); err != nil {
			out.closeAll()
			return nil, err
		}
		func() {
			defer loadOp.Add(1, int64(len(bogonSources)), nil)
			name := src.Name
			latest, err := e.openLatestSet(ctx, name)
			if err != nil {
				e.logger.Warn("bogon source skipped: latest set not available",
					"source", name, "error", err)
				return
			}
			out.Providers[name] = &bogonProviderSet{
				Name:          name,
				Format:        src.Format,
				Set:           latest.RangeSource,
				overlapFilter: buildRangeOverlapFilter(latest.RangeSource),
				sources:       []*closableSource{latest},
			}
			out.Names = append(out.Names, name)
		}()
	}
	return out, nil
}

// buildBogonUnion materializes the union of every loaded bogon
// provider into a single in-memory *iprange.IPSet. The result is
// passed to ASN counting so each feed's unknown bucket can be split
// into bogon vs truly-unknown.
//
// Returns nil when there are zero providers (the dataset has no
// bogon coverage). Callers must treat nil as "no bogon split".
func buildBogonUnion(datasets *bogonDatasets) (*iprange.IPSet, error) {
	if datasets == nil || len(datasets.Providers) == 0 {
		return nil, nil
	}
	sources := make([]iprange.RangeSource, 0, len(datasets.Providers))
	for _, name := range datasets.Names {
		p := datasets.Providers[name]
		if p == nil || p.Set == nil {
			continue
		}
		sources = append(sources, p.Set)
	}
	if len(sources) == 0 {
		return nil, nil
	}
	union := iprange.New("bogon_union")
	for r := range iprange.UnionIter(sources...) {
		if err := union.AddRange(r); err != nil {
			return nil, fmt.Errorf("bogon union add: %w", err)
		}
	}
	union.Optimize()
	return union, nil
}

// bogonRangeJSON is one entry in the optional by_range breakdown of
// the per-feed bogon JSON. Only populated for the rfc_reserved
// provider, where each range has a known label and CIDR.
type bogonRangeJSON struct {
	CIDR  string `json:"cidr"`
	Name  string `json:"name"`
	RFC   string `json:"rfc,omitempty"`
	Count uint64 `json:"count"`
}

// bogonFeedJSON is the schema of <feed>_bogons_<provider>.json. The
// invariant is that BogonIPs is the total IP overlap between the feed
// and the provider's bogon set; ByRange is optional and only populated
// for providers with known per-range labels (currently only the RFC
// reserved baseline).
type bogonFeedJSON struct {
	Provider string           `json:"provider"`
	FeedIPs  uint64           `json:"feed_ips"`
	BogonIPs uint64           `json:"bogon_ips"`
	Percent  float64          `json:"percent"`
	ByRange  []bogonRangeJSON `json:"by_range,omitempty"`
}

// writeBogonComparisonFiles computes the per-feed bogon overlap for
// every loaded provider and writes one <feed>_bogons_<provider>.json
// file per (feed, provider) pair. Mirrors writeASNComparisonFiles in
// shape, parallelism, and the updated-vs-all selection logic.
//
// When updatedNames is non-empty AND no provider name appears in it,
// only those feeds are recomputed. When a provider has updated, the
// caller must pass the provider name in updatedNames so all feeds get
// rebuilt against the fresh provider data.
func (e *Engine) writeBogonComparisonFiles(ctx context.Context, datasets *bogonDatasets, updatedNames []string, outDir string, setCache *latestSetCache) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if datasets == nil || len(datasets.Providers) == 0 {
		return nil
	}
	if setCache == nil {
		setCache = newLatestSetCache(e)
		defer setCache.CloseAll(e.logger)
	}

	targetNames := targetFeedsForFanOut(e.cfg, updatedNames, e.publicOutputNames(), config.UseBogons)
	if len(targetNames) == 0 {
		return nil
	}
	targets := e.bogonComparisonTargets(ctx, targetNames, setCache)
	if len(targets) == 0 {
		return nil
	}
	providerNames := make([]string, 0, len(datasets.Names))
	for _, providerName := range datasets.Names {
		provider := datasets.Providers[providerName]
		if provider == nil || provider.Set == nil {
			continue
		}
		providerNames = append(providerNames, providerName)
	}
	if len(providerNames) == 0 {
		return nil
	}
	totalPairs := int64(len(providerNames) * len(targetNames))
	compareOp := e.beginActiveOperation("bogons.write_comparisons", "", "compare", "feed_provider_pairs", totalPairs)
	defer compareOp.Finish()

	rfcRanges, err := getRFCReservedRanges()
	if err != nil {
		return err
	}

	numWorkers := e.runtime.HeavyPhaseWorkers()
	if numWorkers < 1 {
		numWorkers = 1
	}

	for _, providerName := range providerNames {
		provider := datasets.Providers[providerName]
		if err := runBoundedNameJobs(ctx, numWorkers, targetNames, func(ctx context.Context, name string) error {
			if err := contextErr(ctx); err != nil {
				return err
			}
			defer compareOp.Add(1, totalPairs, nil)
			target, ok := targets[name]
			if !ok {
				return nil
			}

			feedIPs := target.feedIPs
			var (
				src      *closableSource
				bogonIPs uint64
			)
			if rangeOverlapFiltersDisjoint(target.filter, provider.overlapFilter) {
				e.observeRunCounter("bogons.overlap_skipped_filter", 1, 0)
			} else {
				var err error
				src, err = setCache.Open(name)
				if err != nil {
					e.logger.Warn("bogon comparison skipped: cannot open set",
						"set", name, "provider", providerName, "error", err)
					return nil
				}
				bogonIPs = iprange.OverlapCountIter(src.RangeSource, provider.Set)
				if checkErr := checkFileSetErr(src.RangeSource, name, e.logger); checkErr != nil {
					return nil
				}
			}
			if err := contextErr(ctx); err != nil {
				return err
			}

			payload := bogonFeedJSON{
				Provider: providerName,
				FeedIPs:  feedIPs,
				BogonIPs: bogonIPs,
			}
			if feedIPs > 0 {
				payload.Percent = 100 * float64(bogonIPs) / float64(feedIPs)
			}
			if provider.Format == RFCReservedFormat && bogonIPs > 0 {
				if src == nil {
					// A zero-overlap prefilter does not need the feed body, but
					// RFC by-range output still needs the source when overlaps exist.
					var err error
					src, err = setCache.Open(name)
					if err != nil {
						e.logger.Warn("bogon comparison skipped: cannot open set",
							"set", name, "provider", providerName, "error", err)
						return nil
					}
				}
				payload.ByRange = computeRFCByRangeBreakdown(src.RangeSource, rfcRanges)
				if checkErr := checkFileSetErr(src.RangeSource, name, e.logger); checkErr != nil {
					return nil
				}
			}

			data, mErr := jsonMarshalTabIndent(payload)
			if mErr != nil {
				return mErr
			}
			outPath := filepath.Join(outDir, name+"_bogons_"+providerName+".json")
			if wErr := writeFileAtomicAt(outPath, append(data, '\n'), generatedFileMode, e.feedProcessingTimestamp(name)); wErr != nil {
				return wErr
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

type bogonComparisonTarget struct {
	feedIPs uint64
	filter  rangeOverlapFilter
}

func (e *Engine) bogonComparisonTargets(ctx context.Context, names []string, setCache *latestSetCache) map[string]bogonComparisonTarget {
	targets := make(map[string]bogonComparisonTarget, len(names))
	for _, name := range names {
		if err := contextErr(ctx); err != nil {
			return targets
		}
		src, err := setCache.Open(name)
		if err != nil {
			e.logger.Warn("bogon comparison skipped: cannot open set", "set", name, "error", err)
			continue
		}
		filter := buildRangeOverlapFilter(src.RangeSource)
		if checkErr := checkFileSetErr(src.RangeSource, name, e.logger); checkErr != nil {
			continue
		}
		targets[name] = bogonComparisonTarget{
			feedIPs: src.UniqueIPs(),
			filter:  filter,
		}
	}
	return targets
}

// computeRFCByRangeBreakdown computes the per-RFC-range IP count for
// a single feed against the hardcoded RFC reserved baseline. Only
// non-zero entries are returned, sorted by count descending.
func computeRFCByRangeBreakdown(src iprange.RangeSource, ranges []rfcReservedRange) []bogonRangeJSON {
	out := make([]bogonRangeJSON, 0, len(ranges))
	for _, r := range ranges {
		single := iprange.New(r.CIDR)
		if err := single.AddRange(iprange.Range{Lo: r.Lo, Hi: r.Hi}); err != nil {
			continue
		}
		count := iprange.OverlapCountIter(src, single)
		if count == 0 {
			continue
		}
		out = append(out, bogonRangeJSON{
			CIDR:  r.CIDR,
			Name:  r.Name,
			RFC:   r.RFC,
			Count: count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].CIDR < out[j].CIDR
	})
	return out
}
