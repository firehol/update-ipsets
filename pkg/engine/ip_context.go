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
	refs       int
	retired    bool
	closed     bool
}

type asnDatabaseLease struct {
	cache *asnDatabaseCache
	entry *asnDatabaseCacheEntry
	db    *asnloc.Database
	once  sync.Once
}

func newASNDatabaseCache() *asnDatabaseCache {
	return &asnDatabaseCache{dbs: make(map[string]*asnDatabaseCacheEntry)}
}

func (l *asnDatabaseLease) Database() *asnloc.Database {
	if l == nil {
		return nil
	}
	return l.db
}

func (l *asnDatabaseLease) Close() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		if l.cache == nil || l.entry == nil {
			return
		}
		if db := l.cache.release(l.entry); db != nil {
			_ = db.Close()
		}
	})
}

func (c *asnDatabaseCache) acquire(provider, path string, sizeModKey int64, open func() (*asnloc.Database, error)) (*asnDatabaseLease, error) {
	if c == nil {
		return nil, fmt.Errorf("asn lookup cache is not initialized")
	}
	c.mu.Lock()
	if cached, ok := c.dbs[provider]; ok && cached.db != nil && !cached.retired && !cached.closed && cached.path == path && cached.sizeModKey == sizeModKey {
		cached.refs++
		lease := &asnDatabaseLease{cache: c, entry: cached, db: cached.db}
		c.mu.Unlock()
		return lease, nil
	}

	db, err := open()
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}

	var toClose *asnloc.Database
	if existing, ok := c.dbs[provider]; ok && existing != nil {
		toClose = existing.retireLocked()
	}
	entry := &asnDatabaseCacheEntry{
		db:         db,
		path:       path,
		sizeModKey: sizeModKey,
		refs:       1,
	}
	c.dbs[provider] = entry
	lease := &asnDatabaseLease{cache: c, entry: entry, db: db}
	c.mu.Unlock()

	if toClose != nil {
		_ = toClose.Close()
	}
	return lease, nil
}

func (c *asnDatabaseCache) release(entry *asnDatabaseCacheEntry) *asnloc.Database {
	if c == nil || entry == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry.refs > 0 {
		entry.refs--
	}
	if entry.refs == 0 && entry.retired && !entry.closed {
		entry.closed = true
		return entry.db
	}
	return nil
}

func (c *asnDatabaseCache) retireAll() map[string]*asnloc.Database {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.dbs) == 0 {
		return nil
	}
	dbs := make(map[string]*asnloc.Database, len(c.dbs))
	for provider, entry := range c.dbs {
		if entry == nil {
			continue
		}
		if db := entry.retireLocked(); db != nil {
			dbs[provider] = db
		}
	}
	c.dbs = make(map[string]*asnDatabaseCacheEntry)
	return dbs
}

func (e *asnDatabaseCacheEntry) retireLocked() *asnloc.Database {
	if e == nil || e.retired {
		return nil
	}
	e.retired = true
	if e.refs == 0 && !e.closed {
		e.closed = true
		return e.db
	}
	return nil
}

func closeASNLookupDatabases(dbs map[string]*asnloc.Database, logger interface{ Warn(string, ...any) }) {
	for provider, db := range dbs {
		if db == nil {
			continue
		}
		if err := db.Close(); err != nil && logger != nil {
			logger.Warn("ASN lookup database close failed", "provider", provider, "error", err)
		}
	}
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
		if lease := e.loadASNProviderForLookup(provider); lease != nil {
			defer lease.Close()
			db := lease.Database()
			if db == nil {
				return ctx, nil
			}
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

// loadASNProviderForLookup returns a lease for the engine's cached ASN
// database for the given provider, opening it from disk when cold. Callers
// must close the lease when the lookup/build operation is complete.
func (e *Engine) loadASNProviderForLookup(provider string) *asnDatabaseLease {
	if e == nil || e.cfg == nil || e.asnLookupCache == nil {
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

	lease, err := e.asnLookupCache.acquire(provider, path, key, func() (*asnloc.Database, error) {
		return asnloc.Open(src.Format, path)
	})
	if err != nil {
		return nil
	}
	return lease
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
