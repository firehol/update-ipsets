# Entity Integrity

You will learn how integrity works for country and ASN artifacts, why entity integrity is separate from feed integrity, and what repair options exist.

## What entity artifacts are

Country and ASN artifacts are global published outputs. They are not feeds. They are produced by aggregating data from all public feeds and enriching it with geolocation and ASN provider data.

These artifacts power the country detail pages, ASN detail pages, country index, and ASN index on the public website.

## Separate from feed integrity

Entity integrity is a separate panel from feed integrity in the admin UI. This separation exists because:

- Countries and ASNs span across all feeds. A single feed update can affect many country and ASN artifacts.
- The entity refresh path is a background coalesced process, not inline feed processing.
- Recovery work for entities has different concurrency controls than feed recovery.

## What entity integrity checks

For each published entity artifact, integrity verifies:

- **Missing:** the country or ASN JSON file does not exist in the published tree.
- **Stale:** the public artifact is older than the per-feed entity sidecars it should reflect.
- **Malformed:** the JSON exists but cannot be parsed, or its structure does not match the expected schema.

Entity integrity also checks health-transition drift. Country and ASN payloads embed feed health classes. Integrity compares the currently rendered health in the published payload against what should be rendered now — not just file timestamps.

## Startup behavior

Startup entity-artifact integrity repair is conservative. The daemon does not automatically rebuild everything when existing artifacts are usable. Broad startup findings are surfaced to operators and left for:

- bounded ordinary refreshes that happen during normal processing
- explicit operator-triggered repair via the admin UI

The daemon does perform full entity bootstrap when the artifact tree is missing, version-incompatible, or otherwise unusable.

## Full rebuild

A full entity rebuild path exists for operators who need to regenerate everything from scratch. This path:

- is separate from the ordinary incremental feed-update refresh
- processes all feeds through the entity pipeline
- rewrites all country and ASN artifacts

Use full rebuild when:

- the entity artifact tree is corrupted beyond surgical repair
- a major provider change invalidated many artifacts at once
- you want to guarantee a clean baseline after operational incidents

Trigger full rebuild from the admin UI entity integrity panel.

## Operator API

The entity integrity panel is backed by authenticated admin API endpoints:

| Endpoint | Purpose |
|---|---|
| `GET /api/v1/admin/integrity/entities` | Return cached country/ASN entity findings, or queue a refresh and report `in_progress` when the cache is cold or stale. |
| `POST /api/v1/admin/integrity/entities/refresh` | Queue a fresh entity-integrity refresh in the engine lane. |
| `POST /api/v1/admin/integrity/entities/rebuild` | Queue a full country and ASN entity artifact rebuild. |

Refresh and rebuild endpoints require `POST`. If an equivalent refresh, rebuild,
or entity refresh is already queued or running, the response reports
`in_progress` instead of queuing duplicate work.
