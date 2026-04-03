package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/firehol/update-ipsets/pkg/asnloc"
	"github.com/firehol/update-ipsets/pkg/cache"
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
	if err := os.MkdirAll(asnDir, 0o755); err != nil {
		return nil, err
	}

	datasets := asnDatasets{}
	for _, src := range asnSources {
		if err := contextErr(ctx); err != nil {
			datasets.closeAll(e.logger)
			return nil, err
		}
		name := src.Name
		entry := e.state.Entry(name)
		attempt := e.beginFeedAttempt(entry, reason)
		var loopErr error
		func() {
			defer attempt.finish()

			entry.ApplyProviderSourceConfig(cache.ProviderSourceConfigSnapshot{
				Name:              name,
				Category:          src.Category,
				DefaultCategory:   "asn",
				Info:              src.Info,
				Maintainer:        src.Maintainer,
				MaintainerURL:     src.MaintainerURL,
				Frequency:         src.Frequency,
				URL:               src.URL,
				Downloader:        src.Downloader,
				DownloaderOptions: src.DownloaderOptions,
			})

			spec, ok := lookupFormat(src.Format)
			if !ok || spec.role != formatRoleASN {
				e.logger.Error("ASN source has unknown or wrong-role format", "name", name, "format", src.Format)
				entry.MarkProviderConfigError("unknown ASN format " + src.Format)
				return
			}

			providerDir := filepath.Join(asnDir, name)
			if err := os.MkdirAll(providerDir, 0o755); err != nil {
				entry.MarkProviderFilesystemFailure(err.Error())
				loopErr = err
				return
			}
			archivePath := filepath.Join(providerDir, "source")
			processingArchivePath := preferStagedPath(archivePath)
			dataPath := filepath.Join(providerDir, spec.dataFile)
			archiveTime := time.Time{}
			if archiveTime.IsZero() {
				if info, err := os.Stat(processingArchivePath); err == nil {
					archiveTime = info.ModTime().UTC()
				}
			}
			if processingArchivePath != archivePath && spec.extract != nil {
				entry.MarkProviderProcessing()
				if err := spec.extract(processingArchivePath, dataPath); err != nil {
					e.logger.Error("ASN staged extract failed", "name", name, "error", err)
					entry.MarkProviderExtractFailed(err.Error())
					return
				}
			} else if !fileExists(dataPath) && spec.extract != nil {
				entry.MarkProviderProcessing()
				if err := spec.extract(processingArchivePath, dataPath); err != nil {
					e.logger.Error("ASN extract failed", "name", name, "error", err)
					entry.MarkProviderExtractFailed(err.Error())
					return
				}
			}
			if !fileExists(dataPath) {
				e.logger.Warn("ASN database not available, skipping source", "name", name, "path", dataPath)
				entry.MarkProviderUnavailable("database file not found at " + dataPath)
				return
			}
			entry.MarkProviderProcessing()
			db, err := asnloc.Open(src.Format, dataPath)
			if err != nil {
				e.logger.Error("ASN open failed", "name", name, "format", src.Format, "path", dataPath, "error", err)
				entry.MarkProviderOpenFailed(err.Error())
				loopErr = fmt.Errorf("asn open %s: %w", name, err)
				return
			}
			datasets[name] = db
			entries := entry.Entries
			uniqueIPs := entry.UniqueIPs
			if networks, ipv4Covered, statsErr := db.Stats(); statsErr != nil {
				e.logger.Warn("ASN stats failed", "name", name, "error", statsErr)
			} else {
				entries = networks
				uniqueIPs = ipv4Covered
			}
			processedAt := e.now().UTC()
			now := e.now().UTC()
			clockSkewSeconds := int64(0)
			if archiveTime.After(now) {
				clockSkewSeconds = int64(archiveTime.Sub(now).Seconds())
			}
			stale := entry.RecordProviderLoaded(cache.ProviderLoadStats{
				SourceUnix:       archiveTime.Unix(),
				ProcessedUnix:    processedAt.Unix(),
				ClockSkewSeconds: clockSkewSeconds,
				Entries:          entries,
				UniqueIPs:        uniqueIPs,
			}, src.Frequency, processingArchivePath != archivePath)
			e.logger.Info("ASN source loaded", "name", name, "networks", entry.Entries, "ipv4_covered", entry.UniqueIPs)
			if stale {
				e.logger.Warn("ASN using stale data after download failure", "name", name, "failures", entry.DownloadFailures)
			}
		}()
		if loopErr != nil {
			datasets.closeAll(e.logger)
			return nil, loopErr
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

	numWorkers := e.runtime.HeavyPhaseWorkers()
	if numWorkers < 1 {
		numWorkers = 1
	}

	for provider, db := range datasets {
		if db == nil {
			continue
		}
		if err := runBoundedNameJobs(ctx, numWorkers, targetNames, func(ctx context.Context, name string) error {
			if err := contextErr(ctx); err != nil {
				return err
			}
			src, err := setCache.Open(name)
			if err != nil {
				e.logger.Warn("ASN comparison skipped: cannot open set", "set", name, "provider", provider, "error", err)
				return nil
			}

			var bogonRangeSrc iprange.RangeSource
			if bogonUnion != nil {
				bogonRangeSrc = bogonUnion
			}
			counts, names, bogonIPs, err := db.CountFeedWithBogons(src.RangeSource, bogonRangeSrc)
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
			if wErr := writeFileAtomicAt(outPath, append(data, '\n'), 0o644, e.feedProcessingTimestamp(name)); wErr != nil {
				return wErr
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
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
