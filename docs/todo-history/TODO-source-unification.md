# Source Unification — One Mechanism for All Feeds

## TL;DR

- **Goal**: collapse `sources:`, `geolocation:`, `asn:`, and `bogons:` into a single `sources:` block where every feed is described by the same struct, processed by the same pipeline, and routed by a new `use:` field.
- **New fields on `Source`**: `Use []string` (engine roles), `Hidden bool` (no public page), `Format string` (data format hint, replaces per-block `type:`).
- **Three top-level config blocks deleted**: `geolocation:`, `asn:`, `bogons:`.
- **Three Go structs deleted**: `GeolocationFeed`, `ASNFeed`, `BogonProvider`.
- **rfc_reserved becomes a synthetic source** with `url: internal://rfc_reserved`, `use: [bogons]`, `hidden: true`. The downloader recognizes the `internal://` scheme and returns hardcoded data instead of doing HTTP.
- **Selective trust**: `unroutable` stays as a display category; only sources marked `use: [bogons]` participate in bogon attribution. Untrusted unroutable feeds keep their category but get no `use:` marker.
- **Single processing pipeline**: one processSource function branches on `use` to choose ipset / asn / geoip handling. The current `processASNFeeds`, `processGeolocationFeeds`, `loadBogonProviders` paths all collapse into this.

## Decisions locked by Costa

| # | Decision | Choice |
|---|---|---|
| 1 | Full unification now (vs. phase) | **Full** — phasing would leave a confusing half-state |
| 2 | Field name for format hint | **`format:`** (not `type:`) — clearer that it's about wire format |
| 3 | `hidden: true` semantics | **Removes from BOTH** the public catalog (`all-ipsets.json`) AND the per-source page |
| 4 | Empty `use:` vs absent `use:` | **Same meaning** — both default to "regular ipset". Convention: omit `use:` entirely for plain ipsets. |
| 5 | `category` vs `use` separation | **Separate** — `category` is display only, `use` is engine role. The "untrusted unroutable" example is the killer argument: a source can be `category: unroutable` without `use: [bogons]`. |
| 6 | rfc_reserved representation | **Synthetic source** with magic URL `internal://rfc_reserved`. Engine treats it as a regular source for cache/scheduler/admin purposes. |
| 7 | Multi-valued `use` | **Yes** — `Use []string`. A source can have `use: [bogons, critical_infrastructure]` if ever justified. |
| 8 | Trust `cidr_report_bogons` / `iblocklist_cidr_report_bogons` for bogon attribution | **No** — they stay as untrusted unroutable feeds. No `use: [bogons]` marker. |
| 9 | Rename `asn.maxmind_geolite2` → `sources.maxmind_geolite2_asn` | **Yes** — disambiguates from `geolite2_country` once both live in `sources:` |

## Current state — facts (read from the code)

### Top-level config blocks today

| Block | Struct | Entries | Purpose |
|---|---|---|---|
| `sources:` | `Source` | ~200 | Downloadable IP feeds → `.ipset` files |
| `merges:` | `Merge` | ~10 | Composite feeds (firehol_levelN, etc.) |
| `geolocation:` | `GeolocationFeed` | 5 | Country databases (mmdb/csv) → per-feed `<feed>_<provider>_country.json` |
| `asn:` | `ASNFeed` | 4 | ASN databases → per-feed `<feed>_asn_<provider>.json` |
| `bogons:` | `BogonProvider` | 4 (rfc_reserved + 3 feed_reference) | Bogon classifiers → per-feed `<feed>_bogons_<provider>.json` |
| `infrastructure_asns:` | `[]InfrastructureASN` | ~20 | Curated list of ASN numbers — stays as-is, not a "source" |

### Engine pipeline today (`pkg/engine/run.go:13-246`)

```
1. prefetchSources                                  (sources only)
2. for each source: processSource                   (parallel, ipset path)
3. for each merge: processMerge                     (parallel, composes ipsets)
4. heavy block (skipped if no updates):
   a. processGeolocationFeeds → writeCountryComparisonFiles
   b. loadBogonProviders → writeBogonComparisonFiles → buildBogonUnion
   c. processASNFeeds → writeASNComparisonFiles (uses bogonUnion)
5. writeMetadataFiles
```

Each of (a), (b), (c) has its own download/cache/parse path despite doing semantically the same thing: download → extract → load into a queryable in-memory representation → walk every feed's IPs against it.

### File-level inventory

| File | LoC | Purpose |
|---|---|---|
| `pkg/config/config.go` | 494 | Source/Merge/Geo/ASN/Bogon structs + LoadYAML/Merge |
| `pkg/config/validate.go` | (read) | Per-block validation |
| `pkg/engine/run.go` | 270 | RunOnce orchestration with the 5-step pipeline |
| `pkg/engine/process.go` | 627 | processSource + helpers (ipset path) |
| `pkg/engine/asn.go` | 464 | processASNFeeds + writeASNComparisonFiles + buildASNFeedJSON |
| `pkg/engine/geoloc.go` | 278 | processGeolocationFeeds + writeCountryComparisonFiles |
| `pkg/engine/bogons.go` | 290 | loadBogonProviders + buildBogonUnion + writeBogonComparisonFiles |
| `pkg/engine/bogons_rfc.go` | (read) | Hardcoded 15 RFC reserved ranges + parser |
| `pkg/engine/public.go` | 280 | configuredNames + Entry + ASNProviders + BogonProviders APIs |
| `pkg/web/admin.go` | (~600) | adminFeed builders for sources/merges/geo/asn/bogons |
| `pkg/web/server.go` | (read) | API endpoints, including `/api/v1/sets/{name}/{geo,asn,bogons}/{provider}` |
| `pkg/scheduler/scheduler.go` | 350 | Snapshot lists with kind=source/merge/geo/asn/bogon |
| `pkg/asnloc/...` | (refactored 998b2d4) | MMDB + range-table backends behind unified `Database` |
| `pkg/geoloc/...` | (read) | Country DB parsers |
| `configs/firehol.yaml` | 2477 | The 4 blocks above + ~200 source entries |

## The new model

### Unified `Source` struct

```go
type Source struct {
    // identity & display (unchanged + new Hidden flag)
    Name          string
    URL           string
    Frequency     int
    Category      string
    Info          string
    Maintainer    string
    MaintainerURL string

    // role marker (NEW)
    Use    []string  // engine roles: [], [bogons], [asn], [geoip], [critical_infrastructure]
    Hidden bool      // exclude from public catalog and per-source page

    // ipset-specific fields (existing, only meaningful when Use is empty/has bogons/has critical_infrastructure)
    IPV          string
    Output       string
    Processor    []ProcessorStep
    ProcessorRaw string
    Attributes   map[string]string
    EnabledByAll bool
    AcceptEmpty  bool
    History      []int

    // database-specific fields (existing on ASNFeed/GeolocationFeed, promoted)
    Format            string  // wire format: maxmind_asn_mmdb_tar_gz, iptoasn_combined_tsv, caida_prefix2as, dbip_asn_lite_mmdb, maxmind_country_csv, ip2location_country_zip, ipdeny_country_tar_gz, ipip_country_zip, dbip_country_csv, rfc_reserved_baseline
    Downloader        string
    DownloaderOptions string
    License           string
    Attribution       string
    Redistributable   *bool   // promoted from ASNFeed; defaults to true when omitted
}
```

### Engine roles

| `Use` value | Role | Behavior |
|---|---|---|
| (empty / unset) | regular ipset | Download → parse → write `.ipset`, history, retention. The default. |
| `bogons` | bogon attribution | Source is loaded into the bogon union. Per-feed `<feed>_bogons_<source>.json` is computed. Source is ALSO an ipset unless `hidden: true`. |
| `critical_infrastructure` | infrastructure attribution | Source is loaded into the infrastructure union. Per-feed `<feed>_infra_<source>.json` is computed. Source is ALSO an ipset unless `hidden: true`. |
| `asn` | ASN database | Source produces an in-memory `asnloc.Database`. Per-feed `<feed>_asn_<source>.json` is computed. Source does NOT produce an `.ipset`. |
| `geoip` | GeoIP database | Source produces an in-memory `geoloc.Dataset`. Per-feed `<feed>_<source>_country.json` is computed. Source does NOT produce an `.ipset`. |

A source can have multiple roles. The engine processes each declared role; storage and outputs are additive.

### Processing pipeline after unification

```
1. prefetchSources                              (ALL sources, including asn/geoip/bogons)
2. for each source: processSource               (parallel)
                                                ├─ if Use contains "asn":   download → format-specific extract → asnloc.Database (in memory)
                                                ├─ if Use contains "geoip": download → format-specific extract → geoloc.Dataset (in memory)
                                                └─ if Use empty/[bogons]/[critical_infrastructure]: download → parse → write .ipset
3. for each merge: processMerge                 (unchanged)
4. heavy block (skipped if no updates):
   a. build bogonUnion from sources with Use containing "bogons"
   b. build infraUnion from sources with Use containing "critical_infrastructure"
   c. for each source with Use containing "geoip": writeCountryComparisonFiles
   d. for each source with Use containing "bogons": writeBogonComparisonFiles
   e. for each source with Use containing "asn":   writeASNComparisonFiles (uses bogonUnion + infraUnion)
5. writeMetadataFiles
```

The function names get a rename (no more "Feeds" suffix; they operate on sources):

| Old | New |
|---|---|
| `processASNFeeds` | `processASNDatabases` |
| `processGeolocationFeeds` | `processGeoIPDatabases` |
| `loadBogonProviders` | (deleted; replaced by `loadBogonSources` walking `cfg.Sources`) |
| `writeBogonComparisonFiles` | (kept, rewritten to walk sources by Use) |
| `e.cfg.ASN`, `e.cfg.Geolocation`, `e.cfg.Bogons` | (deleted; helper accessors `cfg.SourcesWithUse("asn")` etc.) |

### YAML migration table

#### Bogons block — DELETED

| Current entry | Action |
|---|---|
| `bogons.rfc_reserved` (hardcoded baseline) | Become source `rfc_reserved` with `url: internal://rfc_reserved`, `category: unroutable`, `use: [bogons]`, `hidden: true`, `format: rfc_reserved_baseline`, `frequency: 0` (engine treats 0 as "static") |
| `bogons.cymru_bogons` (feed_reference → source `bogons`) | Delete entry. Add `use: [bogons]` to existing `sources.bogons`. |
| `bogons.cymru_fullbogons` (feed_reference → source `fullbogons`) | Delete entry. Add `use: [bogons]` to existing `sources.fullbogons`. |
| `bogons.iblocklist` (feed_reference → source `iblocklist_bogons`) | Delete entry. Add `use: [bogons]` to existing `sources.iblocklist_bogons`. |
| (also consider) `cidr_report_bogons`, `iblocklist_cidr_report_bogons` | Costa to decide whether these get `use: [bogons]` or stay as untrusted unroutable feeds. **DEFAULT: leave them without `use: [bogons]`** until Costa explicitly opts them in. |

#### ASN block — DELETED, contents moved into sources

| Current entry | Become source | Notes |
|---|---|---|
| `asn.maxmind_geolite2` | `sources.maxmind_geolite2_asn` (renamed for clarity since there's also a `maxmind_geolite2_country`) | `use: [asn]`, `format: maxmind_asn_mmdb_tar_gz`, all license/attribution fields preserved. `hidden: false` (has its own page). |
| `asn.iptoasn` | `sources.iptoasn` | `use: [asn]`, `format: iptoasn_combined_tsv`, public domain. |
| `asn.dbip_asn_lite` | `sources.dbip_asn_lite` | `use: [asn]`, `format: dbip_asn_lite_mmdb`. |
| `asn.caida_prefix2as` | `sources.caida_prefix2as` | `use: [asn]`, `format: caida_prefix2as`, `redistributable: false`. |

**Naming conflict to resolve:** if an existing source already has the name `maxmind_geolite2`, the rename preserves uniqueness.

#### Geolocation block — DELETED, contents moved into sources

| Current entry | Become source | Notes |
|---|---|---|
| `geolocation.dbip_country` | `sources.dbip_country` | `use: [geoip]`, `format: dbip_country_csv` |
| `geolocation.geolite2_country` | `sources.geolite2_country` | `use: [geoip]`, `format: maxmind_country_csv` |
| `geolocation.ip2location_country` | `sources.ip2location_country` | `use: [geoip]`, `format: ip2location_country_zip` |
| `geolocation.ipdeny_country` | `sources.ipdeny_country` | `use: [geoip]`, `format: ipdeny_country_tar_gz` |
| `geolocation.ipip_country` | `sources.ipip_country` | `use: [geoip]`, `format: ipip_country_zip` |

**Naming conflict check:** verify none of these collide with existing source names. (Likely fine since these have descriptive names.)

#### `infrastructure_asns:` block — UNCHANGED

This is a curated list of ASN numbers, not a source. It stays where it is. Future evolution: pair it with sources that have `use: [critical_infrastructure]` for cross-validation.

### URL scheme: `internal://`

New convention for synthetic sources whose data is built into the binary:

- URL form: `internal://<name>`
- Recognized by the downloader as a no-op (no HTTP request)
- The engine has a registry mapping `internal://<name>` → bytes provider function
- Initial registry:
  - `internal://rfc_reserved` → returns the hardcoded RFC reserved baseline as text
- The "frequency" of internal sources is `0` (never expires) but they still pass through the same processing path (cache entry, parse, output). For `rfc_reserved`, the format `rfc_reserved_baseline` parser produces an in-memory IPSet with the 15 hardcoded ranges.

### `hidden: true` semantics

A source with `Hidden: true`:
- **Does** appear in admin UI (operator can monitor failures, adjust schedule)
- **Does** appear in scheduler snapshot (engine tracks it)
- **Does NOT** appear in `all-ipsets.json` (the public catalog)
- **Does NOT** have a per-source HTML page generated
- **Does NOT** appear in homepage category tables
- API endpoints `/api/v1/sets/{name}` return 404 for hidden sources (consistent with no public page)
- Admin endpoints `/api/v1/admin/feeds/{name}` still work

`rfc_reserved` is the canonical example: it has no real "source" to point users at, no maintainer to credit independently of FireHOL, no useful per-source page. But the admin should still see it as a row to confirm it's loaded.

## Plan

### Phase A — Config layer (foundation)

1. **`pkg/config/config.go`**:
   - Add `Use []string`, `Hidden bool`, `Format string`, `Downloader string`, `DownloaderOptions string`, `License string`, `Attribution string` to `Source`
   - Promote `Redistributable *bool` to use the same nullable pattern as ASNFeed
   - Delete `GeolocationFeed`, `ASNFeed`, `BogonProvider` structs
   - Delete `Geolocation`, `ASN`, `Bogons` map fields from `Config`
   - Delete the rfc_reserved auto-injection block in `LoadYAML` (it becomes a real source in the YAML)
   - Update `Merge()` to drop the deleted blocks
   - Add helper: `func (c *Config) SourcesWithUse(role string) []*Source`
   - Add helper: `func (c *Config) Source(name string) *Source` (already implicit)
   - Add helper: `func (s *Source) HasUse(role string) bool`
   - Add constants: `UseBogons = "bogons"`, `UseASN = "asn"`, `UseGeoIP = "geoip"`, `UseCriticalInfrastructure = "critical_infrastructure"`

2. **`pkg/config/validate.go`**:
   - Validate `Use` enum values (only known roles allowed)
   - Validate `Format` is set when `Use` includes `asn` or `geoip` (ipsets can omit it)
   - Validate `internal://` URLs map to a known synthetic name
   - Reject the old top-level blocks with a clear error: "geolocation/asn/bogons blocks are no longer supported; use sources with `use: [...]`. See migration notes."

3. **`pkg/config/legacy.go`** (the bash conf loader):
   - Old conf files don't have asn/geo/bogons blocks anyway, so no change here. Verify this assumption.

### Phase B — Downloader synthetic URL support

4. **`pkg/downloader/downloader.go`**:
   - Detect `internal://` scheme
   - Look up the name in a registry
   - Return the registered bytes as if they had been downloaded
   - Skip HTTP entirely; treat as "always fresh, never modified" (the download result reports `StatusOK` with the registered ModifiedTime — set to a fixed sentinel like Unix epoch so it never triggers re-comparison)

5. **`pkg/downloader/internal.go`** (new file):
   - Registry function: `RegisterInternal(name string, provider func() ([]byte, error))`
   - Init function called from engine startup that registers `rfc_reserved`

### Phase C — Engine routing

6. **`pkg/engine/process.go`** (`processSource`):
   - Branch on `src.Use`:
     - empty / `[bogons]` / `[critical_infrastructure]` → existing ipset path (write `.ipset`)
     - `[asn]` → ASN database loading path (no `.ipset` output)
     - `[geoip]` → GeoIP database loading path (no `.ipset` output)
     - mixed (e.g. `[bogons, asn]`) → both paths execute (rare but supported)
   - Refactor: extract the format-specific download/extract logic into per-format handlers (one for each `format:` value)

7. **`pkg/engine/asn.go`**:
   - Delete `processASNFeeds` (inline its body into the processSource asn branch)
   - Keep `writeASNComparisonFiles` but rename to `writeASNAttributionFiles` and have it walk `cfg.SourcesWithUse("asn")` to find providers
   - The buildASNFeedJSON three-bucket invariant stays exactly as it is

8. **`pkg/engine/geoloc.go`**:
   - Delete `processGeolocationFeeds` (inline into processSource geoip branch)
   - Keep `writeCountryComparisonFiles`, walk by `Use` instead of `cfg.Geolocation`

9. **`pkg/engine/bogons.go`**:
   - **Delete** `loadBogonProviders`, `bogonProviderSet`, `bogonDatasets`, the whole provider abstraction
   - **Replace** with `loadBogonUnion` that walks `cfg.SourcesWithUse("bogons")`, opens each source's already-processed `.ipset` file (or for `rfc_reserved` reads from the in-memory baseline), and unions them
   - **Rewrite** `writeBogonComparisonFiles` to iterate over `cfg.SourcesWithUse("bogons")` and produce `<feed>_bogons_<source>.json` for each one
   - **Keep** `computeRFCByRangeBreakdown` (for the `format: rfc_reserved_baseline` source's by_range output)

10. **`pkg/engine/bogons_rfc.go`**:
    - Keep the hardcoded 15 RFC ranges
    - Expose a `RFCReservedBytes()` function that returns text-format bytes (one CIDR per line) for the downloader registry
    - The `format: rfc_reserved_baseline` parser is just `remove_comments` over the registry output (or simpler: a special parser that returns the same hardcoded ranges directly)

11. **`pkg/engine/run.go`**:
    - The 5-step pipeline collapses:
      ```
      1. prefetchSources (all)
      2. for each source: processSource (parallel; routes by Use internally)
      3. for each merge: processMerge
      4. heavy block:
         a. bogonUnion := loadBogonUnion(cfg)
         b. infraUnion := loadInfrastructureUnion(cfg)  // skip if no critical_infrastructure sources
         c. for each geoip source: writeCountryComparisonFiles(source, ...)
         d. for each bogon source: writeBogonComparisonFiles(source, ...)
         e. for each asn source: writeASNComparisonFiles(source, bogonUnion, infraUnion, ...)
      5. writeMetadataFiles
      ```
    - Function names update to reflect that they iterate sources by Use, not separate config blocks

12. **`pkg/engine/public.go`**:
    - `configuredNames()` walks only `cfg.Sources` and `cfg.Merges` (no more `cfg.ASN`, `cfg.Geolocation`, `cfg.Bogons`)
    - `ASNProviders()` walks `cfg.SourcesWithUse("asn")` and builds the public DTO from Source fields
    - Same for `BogonProviders()` and a new `GeoIPProviders()` (currently the geo providers are queried elsewhere)
    - Apply `Hidden` filter wherever public-facing lists are built

13. **`pkg/engine/helpers.go`**:
    - `targetFeedsForFanOut`: update to recognize when an updated source has `Use containing "asn"` / `"geoip"` / `"bogons"` / `"critical_infrastructure"` and trigger fan-out to all consumer feeds (the same logic as before, just with the unified source iteration)

14. **`pkg/engine/asn_url_resolver.go` and `pkg/engine/asn_formats.go`**:
    - Rename to `format_resolvers.go` and `format_handlers.go` (since the same registry now handles ipset/asn/geoip formats)
    - The CAIDA URL resolver becomes one entry in the format registry, keyed by `format: caida_prefix2as`
    - Each handler returns either: (a) bytes-to-write for ipset role, (b) an `asnloc.Database` for asn role, (c) a `geoloc.Dataset` for geoip role

### Phase D — Web layer

15. **`pkg/web/admin.go`**:
    - **Delete** `buildBogonFeed`, `sortedBogonNames`
    - **Delete** the separate buildAdminFeed paths for asn/geo (they become regular sources)
    - The admin feeds list walks `cfg.Sources` and `cfg.Merges` only
    - Each source row gets a "Used for" badge derived from `Use` (e.g., "ASN database", "GeoIP database", "Bogon source", "Critical infrastructure")
    - Filter `Hidden: true` from the **public** catalog endpoint, but show in the **admin** endpoint
    - Detail builders: one source detail builder routes to format-specific sub-builders based on Use

16. **`pkg/web/server.go`**:
    - API endpoints:
      - `/api/v1/sets/{name}/asn/{provider}` → unchanged URL but `{provider}` is now a source name with `use: [asn]`
      - `/api/v1/sets/{name}/geo/{provider}` → same
      - `/api/v1/sets/{name}/bogons/{provider}` → same
    - Public set endpoints check `Hidden: true` and 404 hidden sources
    - Admin endpoints bypass the Hidden check

17. **`pkg/web/static/*` (frontend)**:
    - **No structural changes needed** if the API shape is preserved
    - Method labels in JS may say "providers" — keep them, since the user-facing concept is still "different ASN/GeoIP/Bogon providers", even if internally they're sources

### Phase E — Scheduler

18. **`pkg/scheduler/scheduler.go`**:
    - **Delete** the bogon provider snapshot path
    - **Delete** the asn/geo separate snapshot paths (if any)
    - The snapshot lists every source from `cfg.Sources` (not `cfg.ASN`, etc.) — already does this for regular sources
    - Each source's `Kind` field reflects its primary role: derived from `Use` (no use → "ipset"; first non-empty use → "asn"/"geoip"/"bogon"/"critical_infrastructure")
    - Filter `Hidden: true` from public scheduler API; show in admin

### Phase F — Documentation & methodology

19. **`pkg/web/static/methodology/bogon-classification.md`**:
    - Replace "providers" terminology with "trusted bogon sources"
    - Explain `use: [bogons]` as the trust marker
    - Mention rfc_reserved as the synthetic source

20. **`pkg/web/static/methodology/asn-attribution.md`**:
    - Replace "providers" with "ASN sources" or similar
    - Same for geographic-distribution

21. **`configs/firehol.yaml`**:
    - Migrate all entries per the migration tables above
    - Delete the three top-level blocks
    - Add header comment explaining the unified model

22. **`README.md` / `CLAUDE.md`** (project):
    - Document the unified config model
    - List the `use:` enum values

### Phase G — Tests

23. **`pkg/config/config_test.go`** (and any LoadYAML tests):
    - Test that the old `geolocation:`/`asn:`/`bogons:` blocks produce a clear error
    - Test that `use: [asn]` etc. parses correctly
    - Test `Hidden: true` round-trip
    - Test multi-valued `use`

24. **`pkg/config/validate_test.go`**:
    - Test invalid `use` values are rejected
    - Test missing `format` for asn/geoip is rejected
    - Test `internal://` URL with unregistered name is rejected

25. **`pkg/engine/bogons_test.go`**:
    - Update to use the new unified model: source with `Use: [bogons]` instead of provider entity
    - Three-bucket invariant test stays at the buildASNFeedJSON level (unchanged)
    - Add test: `unroutable` source WITHOUT `use: [bogons]` is NOT included in bogon union
    - Add test: rfc_reserved synthetic source loads from `internal://` registry

26. **`pkg/engine/asn_test.go`** / `geoloc_test.go`:
    - Update fixtures to use unified Source with `Use: [asn]` / `Use: [geoip]`

27. **`pkg/web/admin_test.go`**:
    - Test admin sees Hidden sources, public does not
    - Test the "Used for" badge appears

28. **`pkg/scheduler/scheduler_test.go`**:
    - Test that sources with `Use: [asn]` / etc. get the right Kind label

### Phase H — Build, test, commit

29. `go build ./...` clean
30. `go vet ./...` clean
31. `go test ./...` all packages pass
32. Verify the existing TestRebuildContinuesAfterNotModified and TestGeolocationCountryComparison still pass (they use the old API surface that's being rewritten)
33. Manual smoke test: load `configs/firehol.yaml`, verify it parses with no errors
34. Commit message: "Unify all feed kinds under sources with use: marker" (no AI mentions)

## Implied decisions (proceeding unless Costa vetoes)

- **`format` is required** when Use includes `asn` or `geoip`. For ipsets, format is optional (default = text). For `internal://` URLs, format is required (e.g., `rfc_reserved_baseline`).
- **`frequency: 0` means "never expires"** for synthetic sources (rfc_reserved). The cache entry still records a CheckedDate so the admin sees it as fresh.
- **Internal source registry is in `pkg/downloader/internal.go`**, populated at engine startup. New synthetic sources are added by registering them once and adding a yaml entry.
- **Format handlers live in `pkg/engine/format_handlers.go`** as a registry: `format: <name>` → handler that knows how to parse that wire format into the right output type.
- **`asn:` block migration verbiage** in the validation error is helpful (one line per old block) so operators know exactly what to do.
- **The methodology pages keep the word "providers"** when speaking to end users (it's the right user-facing term) but the code uses "sources". The methodology pages explain that "providers are sources marked with `use: [asn]` etc."
- **Renaming `asn.maxmind_geolite2` → `sources.maxmind_geolite2_asn`** is necessary to disambiguate from `geolocation.geolite2_country` (which becomes `sources.geolite2_country`). I'll do the rename in the migration. Other ASN/geo sources keep their names if they don't conflict.
- **`cidr_report_bogons` and `iblocklist_cidr_report_bogons` do NOT get `use: [bogons]`** by default. They're additional unroutable feeds but Costa hasn't explicitly trusted them for attribution. Easy to add later by appending to the yaml.
- **Critical infrastructure feeds**: none today. The yaml has the new `use: [critical_infrastructure]` value defined and the engine handles it, but no source declares it until Costa adds Cloudflare/Google/AWS feeds in a future PR.
- **TODO file lifecycle**: this file stays in the repo root until the refactor is verified by Costa, then deleted.
- **Commit strategy**: one big commit. The refactor is too interleaved to split cleanly without leaving the repo broken in intermediate states.

## Testing requirements

- All existing tests pass
- New tests for `use:` parsing, validation, and routing
- Three-bucket invariant test still passes (it's at the JSON-build layer which doesn't change)
- Test that rfc_reserved synthetic source produces the same bogon set as the old hardcoded path
- Visual smoke test: web UI still shows ASN provider tabs, geo tabs, bogon tabs (data flows through unchanged)
- Manual end-to-end: build, install locally, restart service, verify the admin UI shows feeds in the unified list and that ASN/geo/bogon comparisons still produce the right JSON files

## Documentation updates required

- `pkg/web/static/methodology/bogon-classification.md` — wording
- `pkg/web/static/methodology/asn-attribution.md` — wording
- `pkg/web/static/methodology/geographic-distribution.md` — wording
- `configs/firehol.yaml` — header comment explaining `use:` model
- `README.md` (project root, if relevant) — config model overview
- `CLAUDE.md` / `~/src/firehol/CLAUDE.md` — note the unified model so future agents don't get confused by old code references

## Known risks

1. **Yaml diff size**: ~70 lines of yaml move from 3 deleted blocks into the sources block. Visually large but mechanically simple. Mitigation: the migration tables in this TODO are exhaustive — no surprises.
2. **Test churn**: any test that constructs a `cfg.ASN[...]`, `cfg.Geolocation[...]`, or `cfg.Bogons[...]` map needs to switch to `cfg.Sources[...]` with `Use: [asn|geoip|bogons]`. Mitigation: grep for all uses, fix mechanically.
3. **Legacy bash config loader**: if `pkg/config/legacy.go` doesn't handle `use:`, that's fine since the legacy loader is for the OLD bash format which never had asn/geo/bogon blocks. The legacy loader stays as-is, only the YAML loader changes.
4. **External config files in `ipsets.d/`**: any user-supplied yaml files using the old blocks will break with the validation error. Acceptable: this is a one-time migration. The error message tells them what to do.
5. **Cache state**: existing cache entries keyed by source/asn/geo/bogon names will collide with new entries when the old asn/geo/bogon names become source names. Mitigation: the `Renames:` mechanism in `cfg` already handles renames. We may need to add entries like `Renames: { "rfc_reserved": "rfc_reserved" }` (no-op rename to confirm) — actually no, the names stay the same in most cases. Only `maxmind_geolite2` → `maxmind_geolite2_asn` needs a rename entry.
6. **Frontend assumption**: the frontend currently has `loadASN`, `loadGeo`, `loadBogons` paths that fetch `/api/v1/sets/{name}/{kind}/{provider}`. These continue to work because the URL shape is preserved. No frontend changes needed unless the data shape changes (it doesn't).
7. **The "format-specific extract" registry** is a new abstraction. Risk: bugs in moving format handlers from per-block code into the registry. Mitigation: the asn agent (commit 998b2d4) already built `pkg/engine/asn_formats.go` as a registry — we extend it rather than rebuild it.
8. **rfc_reserved as a "downloaded" source**: the engine will create a cache entry, history file, etc. for it. The admin will see "last checked: today" and "no failures". This is correct — it IS a tracked source — but operators might find it weird to see "synthetic" sources alongside real ones. Mitigation: the `Hidden: true` flag keeps it out of the public catalog; admin labels it clearly as synthetic.

## Out of scope

- **Folding `infrastructure_asns:` into sources**: this is a list of ASN numbers (not IP feeds). It belongs as a config entity, not a source.
- **Folding `merges:` into sources**: merges don't have a URL or download cycle; they compose other sources. Different processing model.
- **Renaming `unroutable` category**: we keep it as-is. The selective trust via `use:` is what matters.
- **Adding new critical infrastructure feeds**: the yaml block and engine support are added, but actual feeds (Cloudflare IPs, Google IPs, etc.) come in a separate PR.
- **PDF dossier export**: still Phase 4 of the website redesign, separate from this refactor.
- **Phase 3 luxury redesign**: separate, comes after this lands.
