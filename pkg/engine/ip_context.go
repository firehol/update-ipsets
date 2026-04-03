package engine

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/firehol/update-ipsets/pkg/asnloc"
)

// fileSizeModKey returns a cheap composite key derived from a file's
// size and modification time. Used as a cache-busting fingerprint for
// lazy-loaded provider databases: when the key changes, the caller
// reopens the file.
func fileSizeModKey(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.ModTime().UnixNano() ^ info.Size(), nil
}

// IPContext captures what the engine knows about a single IP beyond
// its feed membership. Every field is "best-effort": when a provider
// is not configured, not loadable, or the IP has no record, the
// corresponding fields are simply omitted. The search endpoint fills
// feed matches separately; this struct covers geography + ASN only.
type IPContext struct {
	IP               string `json:"ip"`
	CountryCode      string `json:"country_code,omitempty"`
	GeoProvider      string `json:"geo_provider,omitempty"`
	GeoProviderLabel string `json:"geo_provider_label,omitempty"`
	ASN              uint32 `json:"asn,omitempty"`
	ASNName          string `json:"asn_name,omitempty"`
	ASNProvider      string `json:"asn_provider,omitempty"`
	ASNProviderLabel string `json:"asn_provider_label,omitempty"`
}

// asnDatabaseCache is a minimal lazy-load cache for ad-hoc single-IP
// ASN lookups via the public search endpoint. Unlike the per-run
// asnDatasets map populated by processASNDatabases, this cache is
// scoped to the engine lifetime and rebuilt on demand, which lets the
// search endpoint answer immediately after daemon start without
// waiting for the first batch run.
type asnDatabaseCache struct {
	mu  sync.Mutex
	dbs map[string]*asnDatabaseCacheEntry
}

type asnDatabaseCacheEntry struct {
	db         *asnloc.Database
	path       string
	sizeModKey int64
}

func newASNDatabaseCache() *asnDatabaseCache {
	return &asnDatabaseCache{dbs: make(map[string]*asnDatabaseCacheEntry)}
}

// LookupIPContext returns the best-effort geography + ASN attribution
// for a single IPv4 address. The IP string must parse to a valid IPv4
// address; IPv6 is not yet supported by the downstream providers.
func (e *Engine) LookupIPContext(ipStr string) (*IPContext, error) {
	if e == nil || e.cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	ipv4, err := parseIPv4ForLookup(ipStr)
	if err != nil {
		return nil, err
	}
	ctx := &IPContext{IP: ipStr}
	if provider := e.preferredGeoProvider(); provider != "" {
		if prepared := e.loadGeoProviderForLookup(provider); prepared != nil {
			ctx.CountryCode = lookupCountryInPreparedProvider(prepared, ipv4)
			ctx.GeoProvider = provider
			ctx.GeoProviderLabel = providerDisplayLabel(e.lookupSource(provider))
		}
	}
	if provider := e.preferredASNProvider(); provider != "" {
		if db := e.loadASNProviderForLookup(provider); db != nil {
			record, _, lookupErr := db.Lookup(ipv4)
			if lookupErr == nil && record.ASN != 0 {
				ctx.ASN = record.ASN
				ctx.ASNName = record.Name
				ctx.ASNProvider = provider
				ctx.ASNProviderLabel = providerDisplayLabel(e.lookupSource(provider))
			}
		}
	}
	return ctx, nil
}

// loadGeoProviderForLookup returns the engine's cached geo dataset
// for the given provider, loading it from disk if the cache is cold.
// The returned value is nil when no on-disk copy exists (e.g. daemon
// has never completed its first run).
func (e *Engine) loadGeoProviderForLookup(provider string) *geoPreparedProvider {
	if e == nil || e.cfg == nil || e.geoProviders == nil {
		return nil
	}
	src := e.lookupSource(provider)
	if src == nil {
		return nil
	}
	spec, ok := lookupFormat(src.Format)
	if !ok || spec.role != formatRoleGeoIP {
		return nil
	}
	path := filepath.Join(e.runtime.LibDir, "geolocation", provider+".source")
	if !fileExists(path) {
		return nil
	}
	prepared, err := e.geoProviders.LoadOrParse(provider, src.Format, path)
	if err != nil {
		return nil
	}
	return prepared
}

// loadASNProviderForLookup returns the engine's cached ASN database
// for the given provider, opening it from disk when cold. The cache
// is invalidated when the underlying file's size or modification
// time changes so a provider refresh automatically becomes visible.
func (e *Engine) loadASNProviderForLookup(provider string) *asnloc.Database {
	if e == nil || e.cfg == nil {
		return nil
	}
	src := e.lookupSource(provider)
	if src == nil {
		return nil
	}
	spec, ok := lookupFormat(src.Format)
	if !ok || spec.role != formatRoleASN {
		return nil
	}
	path := filepath.Join(e.runtime.LibDir, "asn", provider, spec.dataFile)
	if !fileExists(path) {
		return nil
	}
	key, err := fileSizeModKey(path)
	if err != nil {
		return nil
	}

	e.asnLookupCache.mu.Lock()
	defer e.asnLookupCache.mu.Unlock()
	if cached, ok := e.asnLookupCache.dbs[provider]; ok && cached.db != nil && cached.path == path && cached.sizeModKey == key {
		return cached.db
	}

	// File changed or never loaded — open fresh.
	db, err := asnloc.Open(src.Format, path)
	if err != nil {
		return nil
	}
	if existing, ok := e.asnLookupCache.dbs[provider]; ok && existing != nil && existing.db != nil {
		_ = existing.db.Close()
	}
	e.asnLookupCache.dbs[provider] = &asnDatabaseCacheEntry{
		db:         db,
		path:       path,
		sizeModKey: key,
	}
	return db
}

// lookupCountryInPreparedProvider binary-searches the provider's
// flattened segment table and returns the first matching code in the
// provider's stable alphabetical order.
func lookupCountryInPreparedProvider(prepared *geoPreparedProvider, ipv4 uint32) string {
	if prepared == nil || len(prepared.segments) == 0 || len(prepared.countryCodes) == 0 {
		return ""
	}
	index := sort.Search(len(prepared.segments), func(i int) bool {
		return prepared.segments[i].rng.Hi >= ipv4
	})
	if index >= len(prepared.segments) {
		return ""
	}
	segment := prepared.segments[index]
	if ipv4 < segment.rng.Lo || len(segment.codes) == 0 {
		return ""
	}
	return prepared.countryCodes[int(segment.codes[0])]
}

// parseIPv4ForLookup parses a string form of an IPv4 address into a
// 32-bit number. Returns an error for IPv6, hostnames, or otherwise
// unparseable values.
func parseIPv4ForLookup(s string) (uint32, error) {
	ip := net.ParseIP(s)
	if ip == nil {
		return 0, fmt.Errorf("invalid IP address: %s", s)
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return 0, fmt.Errorf("only IPv4 is supported for IP lookup: %s", s)
	}
	return uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3]), nil
}
