# SOW-0028 | 2026-04-30 | home-critical-provider-defaults

## Status

Status: completed

## Requirements

### Purpose

Make homepage exploration and derived entity pages reflect deliberate product
semantics instead of accidental catalog ordering. The homepage must let users
filter both critical-infrastructure reference feeds and feeds overlapping those
references. ASN and geolocation defaults must be explicit configuration so
generated pages, insights, IP lookup context, and feed-detail provider tabs all
agree on the same chosen providers.

### User request quoted verbatim

> the home page filter are missing 2 controls:
>
> 1. which are the critical infra: hard, soft, contextual
> 2. which overlap with critical infra: hard, soft, contextual
>
> Also, the tiles at the home page have a few issues:
>
> 1. When the name of the feed does not fit, there should be a tooltip (not browser tooltip) showing the full name
>
> I noticed that for ASNs the app is using by default `CAIDA prefix2as`. This is a problematic ASN provider because it does not supply names for the ASNs.
>
> The default ASN provider for ASN pages and the default GEO provider for the GEO pages should be selectable in the config, it should not be accidental. This should also enforce the ordering of the ASN and GEO section tabs in feed tabs.
>
> The default ASN provider: ip2asn.com
> The default GEO provider: DB-IP
>
> Changing the default ASN and GEO providers means all ASN and GEO pages and all feeds needs rebuilding.

### Assistant understanding

- Add two homepage filter axes:
  - feeds that are themselves critical-infrastructure reference feeds, grouped
    by `critical.tier` (`hard`, `soft`, `contextual`)
  - normal feed pages that overlap critical-infrastructure reference feeds,
    grouped by overlap tier (`hard`, `soft`, `contextual`)
- Add app-native tooltip behavior for truncated homepage feed-card names.
- Replace first-provider-wins defaulting with explicit config fields.
- Set the shipped config defaults to:
  - ASN default source: `iptoasn` (public label `iptoasn.com`; the request's
    `ip2asn.com` maps to this existing configured source)
  - GEO default source: `dbip_country` (public label `DB-IP`)
- Default providers must appear first in ASN/GEO provider lists so feed-detail
  tabs default to the configured provider without frontend hardcoding.
- A default-provider change must force regeneration of the artifacts whose
  content depends on preferred ASN/GEO providers.

### Acceptance criteria

- Homepage filter state, URL parameters, filter rail, and filter function
  support critical-reference tier and critical-overlap tier filters.
- `/api/v1/sets` feed summaries expose enough config/artifact-derived fields
  for the homepage to filter without name-pattern matching.
- Feed-card names use the shared `HoverTip` primitive and do not use browser
  `title` attributes.
- Config supports explicit default ASN and GEO provider source names.
- Config validation rejects default provider names that do not exist or do not
  carry the matching `use:` role.
- Provider list APIs return the configured default provider first, preserving
  remaining provider order after it.
- Engine preferred ASN/GEO provider selection uses the config defaults and
  falls back to existing source order only when no default is configured.
- Default provider changes are detected as pipeline drift and force rebuilding
  feed/entity artifacts that depend on preferred providers.
- Specs and tests are updated.

## Analysis

Facts from code:

- `pkg/engine/insights.go` currently implements `preferredGeoProvider()` and
  `preferredASNProvider()` by returning the first source with `use: [geoip]` or
  `use: [asn]`. This makes defaults depend on directory/catalog ordering.
- `pkg/engine/public.go` returns ASN/GEO provider API lists in the same
  `SourcesWithUse()` order. Feed-detail `SectionASN` and `SectionGeo` select
  `providers[0]` when the user has not chosen another tab.
- `configs/firehol/sources/asn/caida_prefix2as.yaml` sorts before
  `configs/firehol/sources/asn/iptoasn.yaml`, so CAIDA can become default
  without an explicit product decision.
- `pkg/asnloc/loader_caida.go` documents and implements CAIDA prefix2as as
  ASN-number-only data with empty organization names.
- `pkg/asnloc/loader_iptoasn.go` parses the iptoasn AS description column into
  ASN names.
- Critical-infrastructure reference tiers already exist in config metadata
  (`critical.tier`) and aggregate overlap artifacts already include `tiers`.
- Homepage filtering currently uses `ui/src/lib/explorer-state.ts` over
  `/api/v1/sets` rows. `FeedSummary` does not currently expose critical
  reference or overlap tier fields.
- The app has a shared tooltip primitive at
  `ui/src/components/editorial/hover-tip.tsx`; its comments explicitly forbid
  browser-default `title=` tooltips.

Inference:

- The correct implementation is to promote provider defaults and critical filter
  facts into typed config/API fields, not to infer from feed names or UI-only
  heuristics.
- Default provider changes affect generated country/ASN pages, entity sidecars,
  homepage summaries, and insights because those surfaces use
  `preferredGeoProvider()` / `preferredASNProvider()`.

## Implications and decisions

No user decision is pending before implementation because the request already
selects the product choices:

- homepage filters must exist for critical-reference tier and critical-overlap
  tier
- feed-card truncation must use an app tooltip, not browser tooltip
- default ASN/GEO providers must be config-driven
- shipped defaults must be iptoasn/DB-IP
- default changes must trigger rebuilds

Implementation decision recorded:

- Use a top-level `defaults:` config block:
  - `defaults.asn_provider`
  - `defaults.geo_provider`

Reasoning:

- These defaults are product/catalog semantics, not runtime resource knobs.
- Source names remain the stable config identity. Public labels remain source
  labels (`iptoasn.com`, `DB-IP`).
- Keeping the defaults top-level avoids overloading per-source order or adding
  source-name pattern rules.

## Plan

1. `config-provider-defaults` - medium risk
   - Add config schema, merge, validation, shipped defaults, and provider-order
     helpers.
   - Update config specs.

2. `pipeline-default-drift` - high risk
   - Add a default-provider marker/fingerprint.
   - Scheduler queues a full feed reprocess when defaults change, without
     relying on upstream provider downloads.
   - RunOnce treats default-provider drift like a full preferred-provider
     artifact rebuild and writes the marker after publish.
   - Update pipeline/integrity specs.

3. `home-critical-filters` - medium risk
   - Add critical reference metadata and critical overlap tier summary to
     public feed summaries.
   - Extend frontend API types, URL state, filtering, and filter rail controls.

4. `home-card-tooltip` - low risk
   - Wrap the feed-card name link in `HoverTip` with the full feed name.

5. `validation-and-retrospection` - medium risk
   - Add/adjust Go tests and UI build/lint checks.
   - Update homepage/website specs and project skills if a durable lesson is
     found.

## Execution log

- Added top-level `defaults.asn_provider` and `defaults.geo_provider` config
  support, validation, catalog defaults, provider-default-first ordering, and
  engine preferred-provider selection.
- Added provider-default drift detection and scheduler-triggered full
  provider-derived rebuilds, with a successful marker written after publication.
- Added critical reference metadata and cached critical overlap tiers to public
  feed summaries.
- Added homepage critical-reference and critical-overlap tier filters, including
  URL state.
- Added app-native tooltip behavior for truncated homepage feed-card names.
- Updated specs and project skills so future work preserves explicit provider
  defaults and homepage critical-tier semantics.

## Validation

- `go test ./pkg/config ./pkg/engine ./pkg/scheduler -count=1`
- `make test`
- `make lint`
- `make race`
- `pnpm --dir ui lint`
- `pnpm --dir ui build`
- `./install.sh`
- Runtime smoke:
  - `GET /healthz` returned `ok`
  - installed daemon restarted as `update-ipsets.service`
  - provider-default rebuild completed and wrote
    `/opt/update-ipsets/lib/provider_defaults/provider_set_id`
  - `GET /api/v1/sets/abuseipdb_1d/asn` returns `iptoasn` first
  - `GET /api/v1/sets/abuseipdb_1d/countries` returns `dbip_country` first
  - `GET /api/v1/sets` exposes `critical` and `critical_overlap_tiers`
  - `GET /api/v1/admin/integrity` returned `clean`
  - `GET /api/v1/admin/integrity/entities` returned `clean`

## Outcome

Implemented and installed. Homepage exploration can filter critical reference
feeds and feeds overlapping critical infrastructure by hard/soft/contextual
tier. ASN/GEO canonical providers are now explicit config defaults, shipped as
`iptoasn` and `dbip_country`, and default changes trigger a provider-derived
artifact rebuild.

## Lessons extracted

- Provider default choice is product semantics. It belongs in configuration and
  specs, not in directory order or frontend assumptions.
- Cache mutations must not be added inside parallel artifact writers without a
  race review. The critical-overlap tier cache update is serialized after the
  parallel writer phase.
