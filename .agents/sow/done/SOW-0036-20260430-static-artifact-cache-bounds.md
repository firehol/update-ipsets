# SOW-0036 | 2026-04-30 | static-artifact-cache-bounds

## Status

completed

## Requirements

### Purpose

Follow up the valid SOW-0031 static/public artifact cache concern with an
evidence-first audit of which public/static paths use the in-memory file cache,
whether any path can retain unbounded or high-cardinality data, and what
bounded serving policy is justified.

### Source

Created from the SOW-0031 follow-up/rejection ledger correction. SOW-0031 found
that raw feed routes already stream from disk, but the JSON/static cache path
still needs caller/path-space evidence before redesign.

### Acceptance criteria

- Every production caller of the static file cache is inventoried with route
  family and artifact type.
- Raw `.ipset`/`.netset` serving remains streaming/bounded and is not moved
  into a long-lived heap cache.
- If a cache bound, eviction policy, or streaming bypass is needed, it is
  implemented with tests that exercise observable HTTP behavior.
- If no change is justified, the SOW records the evidence and closes as a
  rejected/non-goal outcome rather than leaving the concern open.
- Any remaining non-implemented item is rejected with evidence or moved to a
  concrete pending SOW path before this SOW closes.

## Analysis

Seed finding from SOW-0031:

- A7 fileCache bounding is a valid concern, but was not proven to be an
  unbounded public path during SOW-0031.
- The next step is not a blind cache rewrite. The next step is caller/path
  evidence: which artifacts enter the cache, their size/cardinality, and
  whether cache lifetime is appropriate for the public-serving contract.

Caller inventory:

- `servePublicSetMetadata` caches feed metadata JSON under
  `/api/v1/sets/{feed}` and `/api/v1/ipsets/{feed}`.
- `servePublicSetFile` caches feed history CSV, comparison JSON, and retention
  JSON under feed-scoped API routes.
- `servePublicSetInsights` caches feed insight JSON.
- `servePublicSetProviderFile` caches feed/provider ASN and bogon JSON.
- `servePublicSetCriticalAggregate` and `servePublicSetCriticalProvider` cache
  critical-infrastructure aggregate/provider JSON after provider-set freshness
  validation.
- `handleCountryIndex`, `handleCountryDetail`, `handleASNIndex`, and
  `handleASNDetail` cache published country and ASN entity artifacts.
- `registerPublicArtifactsAndSPA` caches top-level public artifacts:
  `all-ipsets.json`, sitemap files, `robots.txt`, and `llms.txt`.
- `serveDirectPublishedArtifact` caches direct published `.json`, `.csv`,
  `.xml`, `.txt`, and `.html` requests under the configured published web tree.
- Raw `.ipset`/`.netset` routes do not use `fileCache`; they resolve a
  canonical raw feed path and stream through `serveRawFeedFile`.

Runtime artifact evidence from the installed development tree:

- `/opt/update-ipsets/web` contains 88,405 JSON files and 54,664 ASN detail
  JSON files.
- Largest observed cache-eligible files include:
  - `asns/index.json` at 5,900,092 bytes
  - `sitemap-asns-0001.xml` at 3,221,629 bytes
  - feed/provider ASN JSON files around 2-3.5 MiB
  - direct text files under `netdata-attacks/` at 11-13 MiB
- This proves the current cache is not safe to leave as an unbounded
  map-by-path. The direct published-artifact route plus entity detail pages can
  retain high-cardinality data indefinitely.

Implementation decision:

- Add a bounded LRU policy to `fileCache`, with operator-tunable runtime
  controls for max entries, total bytes, and per-file cache eligibility.
- Preserve cache-first behavior for normal JSON/static artifacts.
- Bypass the in-memory cache for files above the per-file limit and stream
  them from disk with normal cache headers.
- Keep raw `.ipset`/`.netset` routes streaming and outside this cache.
- Direct artifact path-space audit also found hidden publish staging
  directories can exist below the published web tree. Direct artifact serving
  now rejects hidden path segments so temporary publish directories do not
  become public/cache-eligible request targets.

## Plan

- Trace `fileCache` callers across `pkg/web`.
- Classify served artifacts by size, volatility, request path, and public value.
- Implement a bounded policy only if the audit shows a concrete unbounded or
  high-retention path.
- Add behavioral HTTP tests for the chosen policy.

## Validation

- Acceptance criteria evidence:
  - All production `fileCache` callers are inventoried in the Analysis section.
  - Raw `.ipset`/`.netset` route behavior is unchanged as a streaming path and
    has a regression test proving it does not populate the JSON/static cache.
  - `fileCache` now enforces LRU max entries, total byte cap, and per-file cache
    eligibility.
  - Oversized cache-eligible files are still served from disk without being
    retained.
  - Direct artifact routes reject hidden path segments under the published tree.
- Focused tests:
  - `go test ./pkg/web ./pkg/config ./pkg/engine`
  - `go test ./pkg/web`
- Full gates:
  - `make test`
  - `make race`
  - `make lint`
  - `make build`
  - `go test ./tools/archposture`
  - `git diff --check`
- Same-failure scan:
  - Searched all `fileCache`, `ServeFile`, direct artifact, entity artifact,
    raw feed, and static serving call paths under `pkg/web`.
  - Raw body routes remain outside `fileCache`; embedded static assets use
    `http.FileServer` over the embedded filesystem, not the generated-artifact
    cache.
- Specs updated:
  - `.agents/sow/specs/website.md`
  - `.agents/sow/specs/operating-principles.md`
  - `.agents/sow/specs/config.md`
- Skills updated:
  - `.agents/skills/project-coding/SKILL.md`
  - `.agents/skills/project-testing/SKILL.md`

## Outcome

Completed.

The generated public JSON/static artifact cache is now bounded by configured
runtime limits. Normal small artifacts remain cache-first; oversized eligible
artifacts stream from disk; raw feed files remain outside the cache; and hidden
publish-staging path segments are rejected by direct artifact routes.

## Execution log

- Added `web_artifact_cache_max_entries`,
  `web_artifact_cache_max_bytes`, and
  `web_artifact_cache_max_file_bytes` runtime settings with defaults in the
  authored catalog.
- Reworked `pkg/web/cache.go` from an unbounded map into an LRU cache with
  entry, byte, and per-file limits.
- Added disk streaming bypass for cache-eligible files above the per-file cache
  threshold.
- Updated surface route construction to use runtime cache limits.
- Added tests for LRU eviction, byte-limit eviction, oversized-file bypass,
  raw feed route cache isolation, runtime propagation, config validation, and
  hidden direct-artifact path rejection.
- Updated the website, operating, and config specs plus project coding/testing
  skills with the bounded-cache contract.

## Lessons extracted

- "Cache-first" does not mean "cache everything forever." Public generated
  artifact caches need explicit resource limits when the published tree can
  contain tens of thousands of route-addressable files.
- Route path-space audits should include temporary/staging directories, not
  only the intended public files. A safe-path join prevents traversal, but it
  does not by itself define which in-tree paths are public.
- Architecture posture failures should be handled by focused file splits, not
  baseline updates, unless the SOW explicitly accepts a larger file.
