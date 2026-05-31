# Bogons as a first-class special kind + three-bucket ASN split

## TL;DR

Introduce bogons as a new special kind alongside geolocation and asn. Each bogon provider is either an existing feed we already process (e.g. `bogons`, `fullbogons`, `iblocklist_bogons`) or a hardcoded RFC baseline that is always present. All ASN providers now produce a three-bucket split: `attributed_ips + bogon_ips + unknown_ips = feed_ips`, where `unknown_ips` means "truly unrecognized — neither in any ASN database nor in any configured bogon reference". Also fix a latent fan-out bug where updates to geo/asn providers did not trigger recomputation of existing source comparisons.

## Status

### Phase 1 — Backend (engine)
- [x] B1 — Add `BogonProvider` config struct and `Bogons` map to `pkg/config/config.go`
- [x] B2 — Add hardcoded RFC reserved baseline constants
- [x] B3 — Add config validation for `feed_reference` bogon providers (named feeds must exist)
- [x] B4 — Create `pkg/engine/bogons.go` with `loadBogonProviders`, `buildBogonUnion`, `writeBogonComparisonFiles`
- [x] B5 — Wire bogon pipeline into `pkg/engine/run.go` (runs BEFORE asn processing)
- [x] B6 — Pass `bogonUnion` into ASN counting so unknown is split into bogon + unknown
- [x] B7 — Fix fan-out bug: provider updates must trigger full re-comparison in geo/asn/bogons
- [x] B8 — Include bogon provider names in `configuredNames()`

### Phase 2 — API + admin
- [x] B9 — Admin feeds endpoint enumerates bogon providers (new `kind: "bogon"`)
- [x] B10 — Scheduler snapshot includes bogon providers
- [x] B11 — Admin feeds detail handler builds detail for bogon providers
- [x] B12 — New API endpoints: `/api/v1/sets/{name}/bogons` and `/api/v1/sets/{name}/bogons/{provider}`

### Phase 3 — Frontend
- [x] B13 — New bogon section in `index.html` with tabbed providers + red callout above infrastructure callout
- [x] B14 — `app.js` loaders: `loadBogons`, `setBogonProvider`, `computeBogonHeadline`
- [x] B15 — `app.css` styling for the bogon callout (red left-border, same structure as infrastructure)
- [x] B16 — Admin frontend filter dropdown adds `bogon` option + badge CSS class

### Phase 4 — Config + docs
- [x] B17 — Seed `bogons:` block in `configs/firehol.yaml` with `rfc_reserved`, `cymru_bogons`, `cymru_fullbogons`, `iblocklist`
- [x] B18 — Methodology page `/methodology/bogon-classification`
- [x] B19 — Update `/methodology/asn-attribution` to document the three-bucket split

### Phase 5 — Validation
- [x] B20 — `go build ./...` passes
- [x] B21 — `go test ./...` passes
- [x] B22 — Single commit with all changes

**Assigned to**: background agent (general-purpose)
**Status**: completed

## Background

Current state of the engine processing pipeline (relevant extracts):

- `pkg/engine/run.go` — heavy block calls `processGeolocationFeeds` → `writeCountryComparisonFiles` → `processASNFeeds` → `writeASNComparisonFiles`
- `pkg/engine/geoloc.go` — handles geolocation providers (MaxMind, IPDeny, IP2Location, IPIP, DB-IP)
- `pkg/engine/asn.go` — handles ASN providers (currently only maxmind_geolite2, but config already supports multiple)
- `pkg/asnloc/asnloc.go` — MMDB reader with `CountFeed(src iprange.RangeSource)` that walks feed ranges and returns `(counts, names, err)` — counts map is `ASN → IP count`, with ASN 0 meaning "no record in DB"
- `pkg/engine/helpers.go` — `applyEntryStatsUpdate(entry, frequency)` post-update bookkeeping
- `pkg/config/config.go` — `Config` struct with `Sources`, `Merges`, `Geolocation`, `ASN` maps, plus `InfrastructureASNs` list

**The latent fan-out bug** (lines in `pkg/engine/geoloc.go` and `pkg/engine/asn.go`):

```go
updatedSet := make(map[string]struct{}, len(updatedNames))
for _, name := range updatedNames {
    updatedSet[name] = struct{}{}
}

var targetNames []string
if len(updatedSet) == 0 {
    targetNames = e.outputNames()
} else {
    // filter targetNames to only updated sources
}
```

If ONLY a provider updated (no regular sources), `updatedSet` contains the provider name (e.g. `maxmind_geolite2`), which is NOT in `outputNames()`, so `targetNames` ends up empty and nothing gets recomputed. The provider's fresh data is loaded but never used against existing sources.

**The fix**: detect if any name in `updatedNames` is a **provider** (is a key in `cfg.Geolocation`, `cfg.ASN`, or `cfg.Bogons`). If yes, force `targetNames = e.outputNames()` regardless of what regular feeds updated. This must be applied in all three fan-out functions.

## Locked architectural decisions

1. **Bogons is a first-class special kind** — new top-level `bogons:` YAML block, new `cfg.Bogons map[string]*BogonProvider`, new engine pipeline that mirrors ASN
2. **Multi-provider** — every bogon provider in config produces its own `<feed>_bogons_<provider>.json` per regular feed, same pattern as geo/asn
3. **Hardcoded RFC baseline** — always present as a provider called `rfc_reserved` with type `rfc_reserved_baseline`; cannot be removed from config output even if the user omits it (if omitted, it's implicit). Actually: the YAML can be omitted entirely, in which case the engine injects `rfc_reserved` automatically. If the user lists their own bogon providers, `rfc_reserved` is still injected unless explicitly disabled.
4. **Three bogon provider types**: `rfc_reserved_baseline` (hardcoded), `feed_reference` (points to an existing source/merge in the same config), and nothing else for v1
5. **Union semantics for ASN three-bucket split** — the ASN processing computes `bogon_union = union(all configured bogon providers' latest sets, + hardcoded RFC ranges)` and uses it to subtract from the unknown bucket. This makes the ASN breakdown consistent across runs even if external bogon feeds disagree at the edges.
6. **Per-provider bogon breakdown is reported separately** — `<feed>_bogons_<provider>.json` shows each provider's individual overlap with the feed, so operators can see which bogon source is contributing what.
7. **Bogons run BEFORE ASN** in the heavy block — so the bogon_union is available when ASN processing starts.
8. **Fan-out bug is fixed for geo, asn, AND bogons** in the same commit, since bogons needs the fix to work and the other two have the same latent bug.
9. **Unknown semantics change** — `unknown_ips` in the ASN output now means "not in any ASN database AND not in any bogon reference set". The three-bucket invariant is `attributed + bogon + unknown = feed_ips`.

## Hardcoded RFC reserved baseline

Add to `pkg/engine/bogons.go` (or a separate `pkg/engine/bogons_rfc.go`) as a package-level constant slice. Each entry has CIDR, name, and description. These are the 15 RFC-defined reserved IPv4 ranges that should never appear in a public-facing blocklist:

```go
// rfcReservedBogons is the hardcoded baseline list of RFC-reserved
// IPv4 ranges. These are always included in the bogon classification
// regardless of what external bogon feeds are configured, so even if
// every external provider fails, RFC 1918 private space and the
// other reserved ranges are still correctly identified as bogus.
var rfcReservedBogons = []rfcReservedEntry{
    {CIDR: "0.0.0.0/8",          Name: "Current network",                   RFC: "RFC 1122 section 3.2.1.3"},
    {CIDR: "10.0.0.0/8",         Name: "RFC 1918 private (10/8)",           RFC: "RFC 1918"},
    {CIDR: "100.64.0.0/10",      Name: "Carrier-grade NAT",                 RFC: "RFC 6598"},
    {CIDR: "127.0.0.0/8",        Name: "Loopback",                          RFC: "RFC 1122 section 3.2.1.3"},
    {CIDR: "169.254.0.0/16",     Name: "Link-local",                        RFC: "RFC 3927"},
    {CIDR: "172.16.0.0/12",      Name: "RFC 1918 private (172.16/12)",      RFC: "RFC 1918"},
    {CIDR: "192.0.0.0/24",       Name: "IETF protocol assignments",         RFC: "RFC 6890"},
    {CIDR: "192.0.2.0/24",       Name: "TEST-NET-1",                        RFC: "RFC 5737"},
    {CIDR: "192.88.99.0/24",     Name: "6to4 relay anycast (deprecated)",   RFC: "RFC 7526"},
    {CIDR: "192.168.0.0/16",     Name: "RFC 1918 private (192.168/16)",     RFC: "RFC 1918"},
    {CIDR: "198.18.0.0/15",      Name: "Network benchmarking",              RFC: "RFC 2544"},
    {CIDR: "198.51.100.0/24",    Name: "TEST-NET-2",                        RFC: "RFC 5737"},
    {CIDR: "203.0.113.0/24",     Name: "TEST-NET-3",                        RFC: "RFC 5737"},
    {CIDR: "224.0.0.0/4",        Name: "IPv4 multicast",                    RFC: "RFC 5771"},
    {CIDR: "240.0.0.0/4",        Name: "Reserved for future use",           RFC: "RFC 1112"},
}
```

## Config schema

```go
// pkg/config/config.go — add after ASNFeed
type BogonProvider struct {
    Name        string `yaml:"-"`
    Type        string `yaml:"type"` // "rfc_reserved_baseline" or "feed_reference"
    Feed        string `yaml:"feed,omitempty"` // only for "feed_reference" — names an existing source or merge
    Info        string `yaml:"info,omitempty"`
    Maintainer  string `yaml:"maintainer,omitempty"`
    MaintainerURL string `yaml:"maintainer_url,omitempty"`
}

// Add to Config struct:
// Bogons map[string]*BogonProvider `yaml:"bogons,omitempty"`
```

Validation (in `pkg/config/validate.go`):
- Each bogon provider must have a valid `type`
- `type: feed_reference` requires `feed:` that names an existing source or merge in the same config
- `type: rfc_reserved_baseline` must not have `feed:` set
- No other types allowed (future providers will extend this)

If `cfg.Bogons` is nil or missing, the engine should still inject `rfc_reserved` as an implicit provider at load time so operators always get at least the baseline.

## JSON schema

### Per-feed main metadata `<feed>.json` — add new `bogons` summary field

```json
{
  ...existing fields...,
  "bogons": {
    "total_ips": 13527424,
    "percent": 90.14,
    "by_provider": [
      {"name": "rfc_reserved", "count": 13500000},
      {"name": "bogons",       "count": 13520000},
      {"name": "fullbogons",   "count": 13527424},
      {"name": "iblocklist_bogons", "count": 13510000}
    ]
  }
}
```

### New per-feed-per-bogon-provider file `<feed>_bogons_<provider>.json`

```json
{
  "provider": "fullbogons",
  "feed_ips": 15006976,
  "bogon_ips": 13527424,
  "percent": 90.14,
  "by_range": [
    {"cidr": "10.0.0.0/8",      "name": "RFC 1918 private (10/8)",       "count": 1234567},
    {"cidr": "192.168.0.0/16",  "name": "RFC 1918 private (192.168/16)", "count": 65536}
  ]
}
```

The `by_range` block is optional and only populated for `rfc_reserved` (where we know the per-range breakdown natively). For external feeds, the per-range breakdown is not available — the provider's set is just a union of opaque CIDRs with no human-readable labels.

### ASN file `<feed>_asn_<provider>.json` — add new `bogon_ips` field

Existing fields:
- `feed_ips` (unchanged — total IPs in the feed)
- `attributed_ips` (unchanged — IPs with a real ASN record)
- `unknown_ips` — **semantic change**: now means "not attributed AND not bogon"

New field:
- `bogon_ips` — IPs that fell into the bogon_union

Invariant: `feed_ips == attributed_ips + bogon_ips + unknown_ips`.

The `by_asn` list is unchanged. ASN 0 should NO LONGER be counted in the output — unknown is reported via the `unknown_ips` field. Bogon IPs are reported via `bogon_ips`.

The `infrastructure` summary is unchanged.

## New files to create

```
pkg/engine/bogons.go                                 NEW — bogon pipeline, mirrors asn.go structure
pkg/engine/bogons_rfc.go                             NEW — hardcoded RFC reserved baseline + parser
pkg/web/static/methodology/bogon-classification.md   NEW — methodology page
```

## Existing files to modify

```
pkg/config/config.go              + BogonProvider struct, Bogons map, auto-inject rfc_reserved
pkg/config/validate.go            + bogon provider validation
pkg/engine/engine.go              + include bogons in configuredNames()
pkg/engine/run.go                 + wire bogon pipeline (before ASN) + fan-out fix
pkg/engine/geoloc.go              + fan-out fix
pkg/engine/asn.go                 + accept bogon_union, split unknown into bogon+unknown
pkg/asnloc/asnloc.go              + CountFeed accepts optional bogon set, returns 3 counts
pkg/web/server.go                 + /bogons and /bogons/{provider} API endpoints
pkg/web/admin.go                  + include bogon providers in feed enumeration + detail builder
pkg/scheduler/scheduler.go        + scheduler snapshot includes bogon providers
pkg/web/static/index.html         + new bogon section above infrastructure callout
pkg/web/static/app.js             + bogon loaders, headline, render functions
pkg/web/static/app.css            + .bogon-callout styling (red accent)
pkg/web/static/admin.html         + bogon option in kind filter + .badge-bogon CSS
pkg/web/static/methodology/asn-attribution.md  + update to document 3-bucket split
configs/firehol.yaml              + bogons: block with 4 providers seeded
```

## Implementation order (for the agent)

Work phase by phase. Run `go build ./...` after each phase. Don't skip ahead.

**Phase 1: Backend**
1. Add `BogonProvider` struct and `Bogons` map to config — verify YAML round-trips
2. Add validation for bogon providers — write a small table-test
3. Add hardcoded RFC reserved baseline constant in `bogons_rfc.go`
4. Write `loadBogonProviders` that returns a map of provider name → iprange.IPSet, reading `latest.set` for `feed_reference` types and building the RFC set for `rfc_reserved_baseline`. Missing files (e.g. feed reference not yet processed) log a warning and are skipped.
5. Write `buildBogonUnion` that unions all loaded provider sets into one `*iprange.IPSet`
6. Write `writeBogonComparisonFiles(providers map, updatedNames []string)` mirroring `writeCountryComparisonFiles`: for each target feed, for each bogon provider, compute overlap and write `<feed>_bogons_<provider>.json`. Use `iprange.OverlapCountIter`.
7. Wire bogon pipeline into `run.go` BEFORE the ASN pipeline. Fix fan-out in all three fan-out functions.
8. Update `CountFeed` in `asnloc.go` to accept an optional `bogonSet iprange.RangeSource` parameter. When not nil, any IP that overlaps with bogonSet is counted as "bogon" rather than looked up in MMDB. Return a new counter struct or a third return value for bogon count.
9. Update `asn.go` to pass `bogonUnion` to CountFeed and produce the three-bucket split in the JSON output.
10. Update `configuredNames()` to include `cfg.Bogons` keys.

**Phase 2: API + admin**
11. Admin feeds enumeration and detail: add bogon providers (`kind: "bogon"`)
12. Scheduler snapshot: add bogon providers
13. API endpoints: `/api/v1/sets/{name}/bogons` and `/api/v1/sets/{name}/bogons/{provider}`

**Phase 3: Frontend**
14. New bogon section in `index.html` with tabbed providers + red callout
15. `app.js` loaders + headline computation
16. `app.css` styling
17. Admin frontend kind filter dropdown

**Phase 4: Config + docs**
18. Seed `bogons:` block in `configs/firehol.yaml`
19. New methodology page
20. Update asn-attribution methodology page

**Phase 5: Validation**
21. `go build ./...` passes
22. `go test ./...` passes
23. Single commit

## Constraints

- **Single commit** with all changes. Use a clear commit message summarizing the feature.
- **Do not install, do not restart** the daemon. user handles production deploys.
- **No emojis** anywhere in code, comments, or commit messages.
- **No mention of AI, Claude, assistants, or any AI product** in code, comments, or commit messages.
- **Never use `git add -A`** — add specific files.
- **Never bypass hooks** (`--no-verify`, `--no-gpg-sign`).
- **Do not touch unrelated files** (evolution chart, frontend globe, etc.).
- **Do not modify `pkg/cache/cache.go`** — the Entry struct stays the same.
- **Do not add new config fields to GeolocationFeed or ASNFeed** — they were just added and are stable.
- **Do not touch `pkg/engine/process.go`** — the merge path was just reworked.
- **Read pkg/engine/asn.go, pkg/engine/geoloc.go, pkg/asnloc/asnloc.go thoroughly before editing** — they have specific patterns you must mirror.
- **Update the TODO file** at the end to check off each completed item in the Status section and append the commit hash.

## Out of scope

- The spamhaus/firehol_level1 nearly-identical ASN table bug — investigate separately after this ships
- Adding new ASN providers (ip2asn, dbip) — future work
- Evolution chart improvements
- Luxury redesign (Phase 3 of the website)

## Reference patterns

- `pkg/engine/asn.go` `processASNFeeds` and `writeASNComparisonFiles` — the closest working example of what bogon pipeline should look like
- `pkg/engine/geoloc.go` `processGeolocationFeeds` and `writeCountryComparisonFiles` — similar pattern
- `pkg/engine/helpers.go` `applyEntryStatsUpdate` — the shared post-update bookkeeping (bogon providers should NOT use this; they have no cadence and their Entries/UniqueIPs come from the underlying feed)
- `pkg/web/admin.go` `buildAdminFeeds` and `buildAdminFeedDetail` — how to add a new kind to the admin enumerator
- `pkg/scheduler/scheduler.go` — how to add a new kind to the scheduler snapshot

## Implementation commit

`7498233f3f9e8926fd384d9cbcb74a0e9983f598` — Add bogons as a first-class special kind with three-bucket ASN split
