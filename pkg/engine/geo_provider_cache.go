package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"iter"
	"os"
	"slices"
	"sort"
	"sync"

	"github.com/firehol/update-ipsets/pkg/geoloc"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

type geoPreparedSegment struct {
	rng   iprange.Range
	codes []uint16
}

type geoPreparedProvider struct {
	provider     string
	countryCodes []string
	segments     []geoPreparedSegment
	countryCount int
	totalEntries int
	totalIPs     uint64
}

type geoPreparedProviders map[string]*geoPreparedProvider

type geoProviderFreshnessKey struct {
	path       string
	format     string
	sizeModKey int64
	bodyHash   string
}

type geoProviderCacheEntry struct {
	key      geoProviderFreshnessKey
	prepared *geoPreparedProvider
}

type geoProviderCache struct {
	mu      sync.Mutex
	entries map[string]*geoProviderCacheEntry
}

func newGeoProviderCache() *geoProviderCache {
	return &geoProviderCache{
		entries: make(map[string]*geoProviderCacheEntry),
	}
}

func (c *geoProviderCache) LoadOrParse(name, format, path string) (*geoPreparedProvider, error) {
	if c == nil {
		return nil, fmt.Errorf("nil geo provider cache")
	}

	statKey, err := currentGeoProviderStatFreshness(format, path)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	if entry := c.entries[name]; entry != nil && entry.prepared != nil && entry.key.path == statKey.path && entry.key.format == statKey.format && entry.key.sizeModKey == statKey.sizeModKey {
		prepared := entry.prepared
		c.mu.Unlock()
		return prepared, nil
	}
	c.mu.Unlock()

	key, err := currentGeoProviderFreshness(format, path)
	if err != nil {
		return nil, err
	}

	dataset, err := geoloc.ParseFile(format, path)
	if err != nil {
		return nil, err
	}
	prepared, err := prepareGeoProvider(name, dataset)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[name] = &geoProviderCacheEntry{
		key:      key,
		prepared: prepared,
	}
	return prepared, nil
}

func currentGeoProviderStatFreshness(format, path string) (geoProviderFreshnessKey, error) {
	key, err := fileSizeModKey(path)
	if err != nil {
		return geoProviderFreshnessKey{}, err
	}
	return geoProviderFreshnessKey{
		path:       path,
		format:     format,
		sizeModKey: key,
	}, nil
}

func currentGeoProviderFreshness(format, path string) (geoProviderFreshnessKey, error) {
	statKey, err := currentGeoProviderStatFreshness(format, path)
	if err != nil {
		return geoProviderFreshnessKey{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return geoProviderFreshnessKey{}, err
	}
	defer func() { _ = f.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return geoProviderFreshnessKey{}, err
	}
	statKey.bodyHash = hex.EncodeToString(hasher.Sum(nil))
	return statKey, nil
}

func prepareGeoProvider(provider string, dataset *geoloc.Dataset) (*geoPreparedProvider, error) {
	if dataset == nil {
		return nil, fmt.Errorf("nil geolocation dataset")
	}

	sortedCodes := geoloc.SortedCodes(dataset)
	countryCodes := make([]string, 0, len(sortedCodes))
	events := make([]geoSegmentEvent, 0)
	var totalEntries int
	var totalIPs uint64

	for _, code := range sortedCodes {
		set := dataset.Sets[code]
		if set == nil {
			continue
		}
		totalEntries += set.Entries()
		totalIPs += set.UniqueCount()
		if set.UniqueCount() == 0 {
			continue
		}
		if len(countryCodes) >= 1<<16 {
			return nil, fmt.Errorf("geolocation provider %s has too many country buckets", provider)
		}
		codeIndex := uint16(len(countryCodes))
		countryCodes = append(countryCodes, code)
		for r := range set.Iter() {
			events = append(events, geoSegmentEvent{pos: r.Lo, code: codeIndex, delta: 1})
			if r.Hi < ^uint32(0) {
				events = append(events, geoSegmentEvent{pos: r.Hi + 1, code: codeIndex, delta: -1})
			}
		}
	}

	segments := buildGeoPreparedSegments(events, len(countryCodes))

	return &geoPreparedProvider{
		provider:     provider,
		countryCodes: countryCodes,
		segments:     segments,
		countryCount: len(dataset.Sets),
		totalEntries: totalEntries,
		totalIPs:     totalIPs,
	}, nil
}

type geoSegmentEvent struct {
	pos   uint32
	code  uint16
	delta int8
}

func buildGeoPreparedSegments(events []geoSegmentEvent, codeCount int) []geoPreparedSegment {
	if len(events) == 0 || codeCount == 0 {
		return nil
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].pos != events[j].pos {
			return events[i].pos < events[j].pos
		}
		if events[i].delta != events[j].delta {
			return events[i].delta < events[j].delta
		}
		return events[i].code < events[j].code
	})

	active := make([]uint16, codeCount)
	activeCount := 0
	segments := make([]geoPreparedSegment, 0, len(events)/2)

	for i := 0; i < len(events); {
		pos := events[i].pos
		for i < len(events) && events[i].pos == pos {
			evt := events[i]
			prev := active[evt.code]
			if evt.delta > 0 {
				active[evt.code]++
				if prev == 0 {
					activeCount++
				}
			} else if prev > 0 {
				active[evt.code]--
				if prev == 1 {
					activeCount--
				}
			}
			i++
		}
		if activeCount == 0 {
			continue
		}

		end := ^uint32(0)
		if i < len(events) {
			end = events[i].pos - 1
		}
		codes := make([]uint16, 0, activeCount)
		for idx, count := range active {
			if count > 0 {
				codes = append(codes, uint16(idx))
			}
		}
		segment := geoPreparedSegment{
			rng:   iprange.Range{Lo: pos, Hi: end},
			codes: codes,
		}
		if n := len(segments); n > 0 {
			prev := &segments[n-1]
			if prev.rng.Hi != ^uint32(0) && prev.rng.Hi+1 == segment.rng.Lo && slices.Equal(prev.codes, segment.codes) {
				prev.rng.Hi = segment.rng.Hi
				continue
			}
		}
		segments = append(segments, segment)
	}

	return segments
}

func (p *geoPreparedProvider) CountSource(src iprange.RangeSource) ([]CountryValue, uint64) {
	if p == nil || src == nil || len(p.segments) == 0 || len(p.countryCodes) == 0 {
		return nil, 0
	}

	counts := make([]uint64, len(p.countryCodes))
	var totalMapped uint64

	nextSource, stopSource := iter.Pull(src.Iter())
	defer stopSource()

	sourceRange, okSource := nextSource()
	segmentIndex := 0
	for okSource && segmentIndex < len(p.segments) {
		segment := p.segments[segmentIndex]
		if sourceRange.Hi < segment.rng.Lo {
			sourceRange, okSource = nextSource()
			continue
		}
		if segment.rng.Hi < sourceRange.Lo {
			segmentIndex++
			continue
		}

		lo := max(sourceRange.Lo, segment.rng.Lo)
		hi := min(sourceRange.Hi, segment.rng.Hi)
		span := uint64(hi-lo) + 1
		totalMapped += span
		for _, codeIndex := range segment.codes {
			counts[int(codeIndex)] += span
		}

		switch {
		case sourceRange.Hi < segment.rng.Hi:
			sourceRange, okSource = nextSource()
		case segment.rng.Hi < sourceRange.Hi:
			segmentIndex++
		default:
			sourceRange, okSource = nextSource()
			segmentIndex++
		}
	}

	values := make([]CountryValue, 0, len(counts))
	for idx, count := range counts {
		if count == 0 {
			continue
		}
		values = append(values, CountryValue{
			Code:  p.countryCodes[idx],
			Value: count,
		})
	}
	return values, totalMapped
}
