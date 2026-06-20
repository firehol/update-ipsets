package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/firehol/update-ipsets/pkg/asnloc"
	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

// asnDatasets holds the open ASN providers for the current run, keyed by
// provider name. Each Database wraps one decoded MMDB. Callers must Close
// every Database after use.
type asnDatasets map[string]*asnloc.Database

// closeAll releases the underlying file handles for every loaded ASN
// provider. Errors from individual closes are logged but not returned.
func (a asnDatasets) closeAll(logger interface{ Warn(string, ...any) }) {
	for name, db := range a {
		if db == nil {
			continue
		}
		if err := db.Close(); err != nil && logger != nil {
			logger.Warn("ASN database close failed", "provider", name, "error", err)
		}
	}
}

// processASNDatabases downloads, extracts, and opens every configured
// ASN source (cfg.SourcesWithUse("asn")). The result is a map keyed by
// the source name whose values are open asnloc.Database instances.
// Mirrors processGeoIPDatabases in shape and lifecycle.
func (e *Engine) processASNDatabases(ctx context.Context, opts RunOptions) (asnDatasets, error) {
	asnSources := e.cfg.SourcesWithUse(config.UseASN)
	if len(asnSources) == 0 {
		return nil, nil
	}
	reason := normalizeRunReason(opts)
	asnDir := filepath.Join(e.runtime.LibDir, "asn")
	if err := os.MkdirAll(asnDir, generatedDirMode); err != nil {
		return nil, err
	}

	datasets := asnDatasets{}
	loadOp := e.beginActiveOperation("asn.load_providers", "", "load", "providers", int64(len(asnSources)))
	defer loadOp.Finish()
	for _, src := range asnSources {
		if err := contextErr(ctx); err != nil {
			datasets.closeAll(e.logger)
			return nil, err
		}
		if err := func() error {
			defer loadOp.Add(1, int64(len(asnSources)), nil)
			db, err := e.processASNProvider(src, asnDir, reason)
			if err != nil {
				return err
			}
			if db != nil {
				datasets[src.Name] = db
			}
			return nil
		}(); err != nil {
			datasets.closeAll(e.logger)
			return nil, err
		}
	}
	return datasets, nil
}

// asnEntryJSON is one row in the per-feed ASN output file. Sorted desc
// by Count when serialized.
type asnEntryJSON struct {
	ASN     uint32  `json:"asn"`
	Name    string  `json:"name"`
	Count   uint64  `json:"count"`
	Percent float64 `json:"percent"`
}

// asnFeedJSON is the schema of <feed>_asn_<provider>.json. The four IP
// counts add up: FeedIPs == AttributedIPs + BogonIPs + UnknownIPs.
// AttributedIPs is the total represented by every row in by_asn (real
// ASN records). BogonIPs is the count of feed IPs that fell into the
// configured bogon union (RFC reserved + any feed_reference providers).
// UnknownIPs is the residual: IPs that are NOT bogon AND have no MMDB
// record.
//
// When no bogon providers are configured, BogonIPs is 0 and UnknownIPs
// keeps its previous semantics ("no MMDB record"). The invariant
// AttributedIPs + BogonIPs + UnknownIPs == FeedIPs always holds.
type asnFeedJSON struct {
	Provider      string         `json:"provider"`
	FeedIPs       uint64         `json:"feed_ips"`
	AttributedIPs uint64         `json:"attributed_ips"`
	BogonIPs      uint64         `json:"bogon_ips"`
	UnknownIPs    uint64         `json:"unknown_ips"`
	ByASN         []asnEntryJSON `json:"by_asn"`
}

// writeASNComparisonFiles computes the per-feed ASN breakdown for every
// loaded provider and writes one <feed>_asn_<provider>.json file per
// (feed, provider) pair. Mirrors writeCountryComparisonFiles in scope:
// when updatedNames is non-empty only those feeds are recomputed; when
// a provider has updated, every output feed is re-compared against it;
// when updatedNames is empty every feed is processed (initial run).
//
// When bogonUnion is non-nil, the unknown bucket of every per-feed ASN
// breakdown is split into bogon_ips (IPs that fall in the bogon union)
// and unknown_ips (IPs the MMDB has no record for AND are not bogon).
// The three buckets satisfy the invariant
//
//	feed_ips == attributed_ips + bogon_ips + unknown_ips
//
// regardless of which providers are configured.
func (e *Engine) writeASNComparisonFiles(ctx context.Context, datasets asnDatasets, bogonUnion *iprange.IPSet, updatedNames []string, outDir string, setCache *latestSetCache) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if len(datasets) == 0 {
		return nil
	}
	if setCache == nil {
		setCache = newLatestSetCache(e)
		defer setCache.CloseAll(e.logger)
	}

	targetNames := targetFeedsForFanOut(e.cfg, updatedNames, e.publicOutputNames(), config.UseASN, config.UseBogons)
	if len(targetNames) == 0 {
		return nil
	}
	bogonSplits := e.precomputeASNBogonSplits(ctx, targetNames, datasets, bogonUnion, setCache)
	providers := make([]string, 0, len(datasets))
	for provider, db := range datasets {
		if db == nil {
			continue
		}
		providers = append(providers, provider)
	}
	if len(providers) == 0 {
		return nil
	}
	totalPairs := int64(len(providers) * len(targetNames))
	compareOp := e.beginActiveOperation("asn.write_comparisons", "", "compare", "feed_provider_pairs", totalPairs)
	defer compareOp.Finish()

	numWorkers := e.runtime.HeavyPhaseWorkers()
	if numWorkers < 1 {
		numWorkers = 1
	}

	for _, provider := range providers {
		db := datasets[provider]
		if err := runBoundedNameJobs(ctx, numWorkers, targetNames, func(ctx context.Context, name string) error {
			if err := contextErr(ctx); err != nil {
				return err
			}
			defer compareOp.Add(1, totalPairs, nil)
			src, err := setCache.Open(name)
			if err != nil {
				e.logger.Warn("ASN comparison skipped: cannot open set", "set", name, "provider", provider, "error", err)
				return nil
			}

			counts, names, bogonIPs, err := countASNFeedWithBogonSplit(db, src.RangeSource, bogonUnion, bogonSplits, name)
			if checkErr := checkFileSetErr(src.RangeSource, name, e.logger); checkErr != nil {
				return nil
			}
			if err != nil {
				return fmt.Errorf("asn count %s/%s: %w", provider, name, err)
			}
			if err := contextErr(ctx); err != nil {
				return err
			}

			payload := buildASNFeedJSON(provider, counts, names, bogonIPs)
			data, mErr := jsonMarshalTabIndent(payload)
			if mErr != nil {
				return mErr
			}
			outPath := filepath.Join(outDir, name+"_asn_"+provider+".json")
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

func (e *Engine) precomputeASNBogonSplits(ctx context.Context, targetNames []string, datasets asnDatasets, bogonUnion *iprange.IPSet, setCache *latestSetCache) map[string]uint64 {
	if bogonUnion == nil || len(datasets) <= 1 || len(targetNames) == 0 || setCache == nil {
		return nil
	}
	progress := e.beginActiveOperation("asn.precompute_bogon_splits", "", "precompute", "feeds", int64(len(targetNames)))
	defer progress.Finish()
	splits := make(map[string]uint64, len(targetNames))
	bogonFilter, err := iprange.BuildRangeOverlapFilterContext(ctx, bogonUnion)
	if err != nil {
		e.logger.Warn("ASN bogon split precompute skipped: bogon overlap filter failed", "error", err)
		return splits
	}
	for _, name := range targetNames {
		if err := contextErr(ctx); err != nil {
			return splits
		}
		func() {
			defer progress.Add(1, int64(len(targetNames)), nil)
			src, err := setCache.Open(name)
			if err != nil {
				e.logger.Warn("ASN bogon split skipped: cannot open set", "set", name, "error", err)
				return
			}
			filter, err := iprange.BuildRangeOverlapFilterContext(ctx, src.RangeSource)
			if err != nil {
				e.logger.Warn("ASN bogon split skipped: overlap filter failed", "set", name, "error", err)
				return
			}
			if checkErr := checkFileSetErr(src.RangeSource, name, e.logger); checkErr != nil {
				return
			}
			if filter.Disjoint(bogonFilter) {
				splits[name] = 0
				e.observeRunCounter("asn.bogon_split_skipped_filter", 1, 0)
				return
			}
			bogonIPs, err := iprange.OverlapCountIterContext(ctx, src.RangeSource, bogonUnion)
			if err != nil {
				e.logger.Warn("ASN bogon split skipped: overlap count failed", "set", name, "error", err)
				return
			}
			splits[name] = bogonIPs
			if checkErr := checkFileSetErr(src.RangeSource, name, e.logger); checkErr != nil {
				delete(splits, name)
				return
			}
			e.observeRunCounter("asn.bogon_split_precomputed", 1, 0)
		}()
	}
	return splits
}

func countASNFeedWithBogonSplit(db *asnloc.Database, src iprange.RangeSource, bogonUnion *iprange.IPSet, bogonSplits map[string]uint64, name string) (map[uint32]uint64, map[uint32]string, uint64, error) {
	if bogonUnion != nil && bogonSplits != nil {
		if bogonIPs, ok := bogonSplits[name]; ok {
			counts, names, err := db.CountFeedExcluding(src, bogonUnion)
			return counts, names, bogonIPs, err
		}
	}

	var bogonRangeSrc iprange.RangeSource
	if bogonUnion != nil {
		bogonRangeSrc = bogonUnion
	}
	return db.CountFeedWithBogons(src, bogonRangeSrc)
}

// buildASNFeedJSON assembles the per-feed-per-provider JSON payload from
// the raw ASN counts. The output is sorted desc by count and tracks the
// unknown-ASN and bogon counts separately so neither is ever surfaced as a real
// ASN row.
func buildASNFeedJSON(provider string, counts map[uint32]uint64, names map[uint32]string, bogonIPs uint64) asnFeedJSON {
	out := asnFeedJSON{Provider: provider}
	out.UnknownIPs = counts[0]
	out.BogonIPs = bogonIPs
	for asn, count := range counts {
		if asn == 0 {
			continue
		}
		out.AttributedIPs += count
	}
	out.FeedIPs = out.AttributedIPs + out.BogonIPs + out.UnknownIPs
	denominator := out.FeedIPs
	if denominator == 0 {
		denominator = 1
	}

	out.ByASN = make([]asnEntryJSON, 0, len(counts))
	for asn, count := range counts {
		if asn == 0 {
			continue
		}
		entry := asnEntryJSON{
			ASN:     asn,
			Name:    names[asn],
			Count:   count,
			Percent: 100 * float64(count) / float64(denominator),
		}
		out.ByASN = append(out.ByASN, entry)
	}
	sort.Slice(out.ByASN, func(i, j int) bool {
		if out.ByASN[i].Count != out.ByASN[j].Count {
			return out.ByASN[i].Count > out.ByASN[j].Count
		}
		return out.ByASN[i].ASN < out.ByASN[j].ASN
	})

	return out
}
