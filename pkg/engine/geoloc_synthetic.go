package engine

import (
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/downloader"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func (e *Engine) registerSyntheticInternalSources() {
	if e == nil {
		return
	}
	downloader.RegisterInternal(config.GeoAnonymousSourceName, e.syntheticGeoBucketProvider("ANONYMOUS"))
	downloader.RegisterInternal(config.GeoSatelliteSourceName, e.syntheticGeoBucketProvider("SATELLITE"))
}

func (e *Engine) syntheticGeoBucketProvider(bucket string) downloader.InternalProvider {
	return func(referencePath string) ([]byte, error) {
		providers, newestSource, err := e.availableGeoSyntheticProviders()
		if err != nil {
			return nil, err
		}
		if referencePath != "" {
			if info, statErr := os.Stat(referencePath); statErr == nil && !newestSource.After(info.ModTime().UTC()) {
				return nil, downloader.ErrInternalNotModified
			}
		}
		set, err := geoBucketUnionSet(bucket, providers)
		if err != nil {
			return nil, err
		}
		return renderCanonicalFeedBody(set)
	}
}

func (e *Engine) availableGeoSyntheticProviders() (geoPreparedProviders, time.Time, error) {
	if e == nil || e.cfg == nil {
		return nil, time.Time{}, fmt.Errorf("engine is not initialized")
	}
	providers := make(geoPreparedProviders)
	missing := make([]string, 0)
	newest := time.Time{}
	for _, src := range e.cfg.SourcesWithUse(config.UseGeoIP) {
		if src == nil {
			continue
		}
		path := preferStagedPath(e.providerArchivePath(src.Name, src))
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, src.Name)
				continue
			}
			return nil, time.Time{}, fmt.Errorf("stat geolocation provider %s: %w", src.Name, err)
		}
		prepared, err := e.geoProviders.LoadOrParse(src.Name, src.Format, path)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("load geolocation provider %s: %w", src.Name, err)
		}
		providers[src.Name] = prepared
		if mod := info.ModTime().UTC(); mod.After(newest) {
			newest = mod
		}
	}
	if len(providers) == 0 {
		slices.Sort(missing)
		if len(missing) == 0 {
			return nil, time.Time{}, fmt.Errorf("no geolocation providers are configured")
		}
		return nil, time.Time{}, fmt.Errorf("no geolocation provider archives are available (missing: %v)", missing)
	}
	return providers, newest, nil
}

func geoBucketUnionSet(bucket string, providers geoPreparedProviders) (*iprange.IPSet, error) {
	set := iprange.New(bucket)
	for _, name := range sortedPreparedProviderNames(providers) {
		prepared := providers[name]
		if prepared == nil {
			continue
		}
		codeIndex := geoPreparedCodeIndex(prepared, bucket)
		if codeIndex < 0 {
			continue
		}
		for _, segment := range prepared.segments {
			if !geoPreparedSegmentHasCode(segment, uint16(codeIndex)) {
				continue
			}
			if err := set.AddRange(segment.rng); err != nil {
				return nil, fmt.Errorf("add %s segment from provider %s: %w", bucket, name, err)
			}
		}
	}
	set.Optimize()
	return set, nil
}

func sortedPreparedProviderNames(providers geoPreparedProviders) []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func geoPreparedCodeIndex(prepared *geoPreparedProvider, code string) int {
	if prepared == nil {
		return -1
	}
	for idx, candidate := range prepared.countryCodes {
		if candidate == code {
			return idx
		}
	}
	return -1
}

func geoPreparedSegmentHasCode(segment geoPreparedSegment, code uint16) bool {
	for _, candidate := range segment.codes {
		if candidate == code {
			return true
		}
	}
	return false
}
