package engine

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/firehol/update-ipsets/pkg/asnloc"
	"github.com/firehol/update-ipsets/pkg/config"
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
	mu         sync.Mutex
	dbs        map[string]*asnDatabaseCacheEntry
	loads      map[asnDatabaseCacheKey]*asnDatabaseLoad
	generation uint64
}

type asnDatabaseCacheEntry struct {
	db         *asnloc.Database
	path       string
	sizeModKey int64
	refs       int
	retired    bool
	closed     bool
}

type asnDatabaseCacheKey struct {
	provider   string
	path       string
	sizeModKey int64
}

type asnDatabaseLoad struct {
	done chan struct{}
}

type asnDatabaseLease struct {
	cache *asnDatabaseCache
	entry *asnDatabaseCacheEntry
	db    *asnloc.Database
	once  sync.Once
}

func newASNDatabaseCache() *asnDatabaseCache {
	return &asnDatabaseCache{
		dbs:   make(map[string]*asnDatabaseCacheEntry),
		loads: make(map[asnDatabaseCacheKey]*asnDatabaseLoad),
	}
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
	return c.acquireContext(context.Background(), provider, path, sizeModKey, open)
}

func (c *asnDatabaseCache) acquireContext(ctx context.Context, provider, path string, sizeModKey int64, open func() (*asnloc.Database, error)) (*asnDatabaseLease, error) {
	ctx = nonNilContext(ctx)
	if c == nil {
		return nil, fmt.Errorf("asn lookup cache is not initialized")
	}
	key := asnDatabaseCacheKey{provider: provider, path: path, sizeModKey: sizeModKey}
	for {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.ensureLocked()
		if cached, ok := c.dbs[provider]; ok && cached.matches(key) {
			cached.refs++
			lease := &asnDatabaseLease{cache: c, entry: cached, db: cached.db}
			c.mu.Unlock()
			return lease, nil
		}
		if load := c.loads[key]; load != nil {
			done := load.done
			c.mu.Unlock()
			select {
			case <-done:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}

		load := &asnDatabaseLoad{done: make(chan struct{})}
		loadGeneration := c.generation
		c.loads[key] = load
		c.mu.Unlock()

		db, err := open()
		ctxErr := contextErr(ctx)

		c.mu.Lock()
		if c.loads[key] == load {
			delete(c.loads, key)
			close(load.done)
		}
		if ctxErr != nil {
			c.mu.Unlock()
			if db != nil {
				_ = db.Close()
			}
			return nil, ctxErr
		}
		if err != nil {
			c.mu.Unlock()
			return nil, err
		}
		if loadGeneration != c.generation {
			c.mu.Unlock()
			_ = db.Close()
			continue
		}
		if cached, ok := c.dbs[provider]; ok && cached.matches(key) {
			cached.refs++
			lease := &asnDatabaseLease{cache: c, entry: cached, db: cached.db}
			c.mu.Unlock()
			_ = db.Close()
			return lease, nil
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
}

func (c *asnDatabaseCache) ensureLocked() {
	if c.dbs == nil {
		c.dbs = make(map[string]*asnDatabaseCacheEntry)
	}
	if c.loads == nil {
		c.loads = make(map[asnDatabaseCacheKey]*asnDatabaseLoad)
	}
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
		c.generation++
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
	c.generation++
	return dbs
}

func (e *asnDatabaseCacheEntry) matches(key asnDatabaseCacheKey) bool {
	return e != nil && e.db != nil && !e.retired && !e.closed && e.path == key.path && e.sizeModKey == key.sizeModKey
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
	return e.LookupIPContextContext(context.Background(), ipStr)
}

func (e *Engine) LookupIPContextContext(ctx context.Context, ipStr string) (*IPContext, error) {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	cfg, rt, geoProviders, asnLookupCache := e.lookupContextSnapshot()
	if cfg == nil {
		return nil, fmt.Errorf("engine is not configured")
	}
	ipv4, err := parseIPv4ForLookup(ipStr)
	if err != nil {
		return nil, err
	}
	result := &IPContext{IP: ipStr}
	if provider := preferredGeoProviderForConfig(cfg); provider != "" {
		if prepared := loadGeoProviderForLookupSnapshot(cfg, rt, geoProviders, provider); prepared != nil {
			result.CountryCode = lookupCountryInPreparedProvider(prepared, ipv4)
			result.GeoProvider = provider
			result.GeoProviderLabel = providerDisplayLabel(lookupSourceForConfig(cfg, provider))
		}
	}
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if provider := preferredASNProviderForConfig(cfg); provider != "" {
		if lease := loadASNProviderForLookupSnapshotContext(ctx, cfg, rt, asnLookupCache, provider); lease != nil {
			defer lease.Close()
			db := lease.Database()
			if db == nil {
				return result, nil
			}
			record, _, lookupErr := db.Lookup(ipv4)
			if lookupErr == nil && record.ASN != 0 {
				result.ASN = record.ASN
				result.ASNName = record.Name
				result.ASNProvider = provider
				result.ASNProviderLabel = providerDisplayLabel(lookupSourceForConfig(cfg, provider))
			}
		}
	}
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (e *Engine) lookupContextSnapshot() (*config.Config, Runtime, *geoProviderCache, *asnDatabaseCache) {
	if e == nil {
		return nil, Runtime{}, nil, nil
	}
	e.mu.RLock()
	cfg := e.cfg
	rt := e.runtime
	geoProviders := e.geoProviders
	asnLookupCache := e.asnLookupCache
	e.mu.RUnlock()
	return cfg, rt, geoProviders, asnLookupCache
}

// loadGeoProviderForLookup returns the engine's cached geo dataset
// for the given provider, loading it from disk if the cache is cold.
// The returned value is nil when no on-disk copy exists (e.g. daemon
// has never completed its first run).
func (e *Engine) loadGeoProviderForLookup(provider string) *geoPreparedProvider {
	cfg, rt, geoProviders, _ := e.lookupContextSnapshot()
	return loadGeoProviderForLookupSnapshot(cfg, rt, geoProviders, provider)
}

func loadGeoProviderForLookupSnapshot(cfg *config.Config, rt Runtime, geoProviders *geoProviderCache, provider string) *geoPreparedProvider {
	if cfg == nil || geoProviders == nil {
		return nil
	}
	src := lookupSourceForConfig(cfg, provider)
	if src == nil {
		return nil
	}
	spec, ok := lookupFormat(src.Format)
	if !ok || spec.role != formatRoleGeoIP {
		return nil
	}
	path := filepath.Join(rt.LibDir, "geolocation", provider+".source")
	if !fileExists(path) {
		return nil
	}
	prepared, err := geoProviders.LoadOrParse(provider, src.Format, path)
	if err != nil {
		return nil
	}
	return prepared
}

// loadASNProviderForLookup returns a lease for the engine's cached ASN
// database for the given provider, opening it from disk when cold. Callers
// must close the lease when the lookup/build operation is complete.
func (e *Engine) loadASNProviderForLookup(provider string) *asnDatabaseLease {
	return e.loadASNProviderForLookupContext(context.Background(), provider)
}

func (e *Engine) loadASNProviderForLookupContext(ctx context.Context, provider string) *asnDatabaseLease {
	cfg, rt, _, asnLookupCache := e.lookupContextSnapshot()
	return loadASNProviderForLookupSnapshotContext(ctx, cfg, rt, asnLookupCache, provider)
}

func loadASNProviderForLookupSnapshot(cfg *config.Config, rt Runtime, asnLookupCache *asnDatabaseCache, provider string) *asnDatabaseLease {
	return loadASNProviderForLookupSnapshotContext(context.Background(), cfg, rt, asnLookupCache, provider)
}

func loadASNProviderForLookupSnapshotContext(ctx context.Context, cfg *config.Config, rt Runtime, asnLookupCache *asnDatabaseCache, provider string) *asnDatabaseLease {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return nil
	}
	if cfg == nil || asnLookupCache == nil {
		return nil
	}
	src := lookupSourceForConfig(cfg, provider)
	if src == nil {
		return nil
	}
	spec, ok := lookupFormat(src.Format)
	if !ok || spec.role != formatRoleASN {
		return nil
	}
	path := filepath.Join(rt.LibDir, "asn", provider, spec.dataFile)
	if !fileExists(path) {
		return nil
	}
	key, err := fileSizeModKey(path)
	if err != nil {
		return nil
	}

	lease, err := asnLookupCache.acquireContext(ctx, provider, path, key, func() (*asnloc.Database, error) {
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
