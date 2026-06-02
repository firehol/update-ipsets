package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/pkg/cache"
	"github.com/firehol/update-ipsets/pkg/config"
)

func (e *Engine) processGeoIPDatabases(ctx context.Context, opts RunOptions) (geoPreparedProviders, error) {
	geoSources := e.cfg.SourcesWithUse(config.UseGeoIP)
	if len(geoSources) == 0 {
		return nil, nil
	}
	reason := normalizeRunReason(opts)
	sourceDir := filepath.Join(e.runtime.LibDir, "geolocation")
	if err := os.MkdirAll(sourceDir, 0o700); err != nil {
		return nil, err
	}

	datasets := make(geoPreparedProviders, len(geoSources))
	for _, src := range geoSources {
		if err := contextErr(ctx); err != nil {
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
				DefaultCategory:   "geolocation",
				Info:              src.Info,
				Maintainer:        src.Maintainer,
				MaintainerURL:     src.MaintainerURL,
				Frequency:         src.Frequency,
				URL:               src.URL,
				Downloader:        src.Downloader,
				DownloaderOptions: src.DownloaderOptions,
			})

			spec, ok := lookupFormat(src.Format)
			if !ok || spec.role != formatRoleGeoIP {
				e.logger.Error("geoip source has unknown or wrong-role format", "name", name, "format", src.Format)
				entry.MarkProviderConfigError("unknown geoip format " + src.Format)
				return
			}

			sourcePath := filepath.Join(sourceDir, name+".source")
			processingPath := preferStagedPath(sourcePath)
			archiveTime := time.Time{}
			if archiveTime.IsZero() {
				if info, err := os.Stat(processingPath); err == nil {
					archiveTime = info.ModTime().UTC()
				}
			}
			if !fileExists(processingPath) {
				e.logger.Warn("geolocation source file not available, skipping source", "name", name, "path", processingPath)
				entry.MarkProviderUnavailable("source file not found at " + processingPath)
				return
			}
			entry.MarkProviderProcessing()
			prepared, err := e.geoProviders.LoadOrParse(name, src.Format, processingPath)
			if err != nil {
				e.logger.Error("geolocation parse failed", "name", name, "format", src.Format, "path", processingPath, "error", err)
				entry.MarkProviderParseFailed(err.Error())
				loopErr = fmt.Errorf("geolocation parse %s: %w", name, err)
				return
			}
			datasets[name] = prepared
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
				Entries:          prepared.totalEntries,
				UniqueIPs:        prepared.totalIPs,
			}, src.Frequency, processingPath != sourcePath)
			e.logger.Info("geolocation source loaded", "name", name, "countries", prepared.countryCount, "entries", prepared.totalEntries, "unique_ips", prepared.totalIPs)
			if stale {
				e.logger.Warn("geolocation using stale data after download failure", "name", name, "failures", entry.DownloadFailures)
			}
		}()
		if loopErr != nil {
			return nil, loopErr
		}
	}
	return datasets, nil
}

// writeCountryComparisonFiles computes geolocation overlap for ipsets.
// When updatedNames is non-empty, only those sources are compared against
// geo providers — the rest keep their existing _*_country.json files on disk.
// When updatedNames contains a provider name (e.g. a geolocation
// provider just updated), every output feed is re-compared against the
// fresh provider data. When updatedNames is empty, all sources are
// compared (initial run).
func (e *Engine) writeCountryComparisonFiles(ctx context.Context, datasets geoPreparedProviders, updatedNames []string, outDir string, setCache *latestSetCache) error {
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

	targetNames := targetFeedsForFanOut(e.cfg, updatedNames, e.publicOutputNames(), config.UseGeoIP)
	if len(targetNames) == 0 {
		return nil
	}

	numWorkers := e.runtime.HeavyPhaseWorkers()
	if numWorkers < 1 {
		numWorkers = 1
	}

	for provider, dataset := range datasets {
		if dataset == nil || len(dataset.segments) == 0 {
			continue
		}

		type geoResult struct {
			name        string
			values      []CountryValue
			totalMapped uint64
		}
		var geoResults []geoResult
		var geoResultsMu sync.Mutex
		if err := runBoundedNameJobs(ctx, numWorkers, targetNames, func(ctx context.Context, name string) error {
			if err := contextErr(ctx); err != nil {
				return err
			}
			src, err := setCache.Open(name)
			if err != nil {
				e.logger.Warn("geolocation comparison skipped: cannot open set", "set", name, "provider", provider, "error", err)
				return nil
			}
			values, totalMapped := dataset.CountSource(src.RangeSource)
			if ioErr := checkFileSetErr(src.RangeSource, name, e.logger); ioErr != nil {
				return nil
			}
			if err := contextErr(ctx); err != nil {
				return err
			}
			geoResultsMu.Lock()
			geoResults = append(geoResults, geoResult{name: name, values: values, totalMapped: totalMapped})
			geoResultsMu.Unlock()
			return nil
		}); err != nil {
			return err
		}

		for _, r := range geoResults {
			if err := contextErr(ctx); err != nil {
				return err
			}
			// Wrap the country array in an object with the flattened
			// union-based total_mapped count. Summing the per-country
			// values is unreliable when provider buckets overlap; the
			// prepared flattened segments de-duplicate the union while
			// still attributing each segment to every matching code.
			payload := struct {
				TotalMapped uint64         `json:"total_mapped"`
				Countries   []CountryValue `json:"countries"`
			}{
				TotalMapped: r.totalMapped,
				Countries:   r.values,
			}
			data, err := jsonMarshalTabIndent(payload)
			if err != nil {
				return err
			}
			if err := writeFileAtomicAt(filepath.Join(outDir, r.name+"_"+provider+".json"), append(data, '\n'), 0o600, e.feedProcessingTimestamp(r.name)); err != nil {
				return err
			}
		}
	}
	return nil
}
