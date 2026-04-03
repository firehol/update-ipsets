# Use Roles

You will learn what `use:` roles exist, what each role means, and which combinations are allowed or rejected by validation.

## What use roles are

The `use:` field assigns a feed or merge to an engine role. The role controls how the system treats the feed — whether it appears as a normal public feed, a bogon reference, a critical infrastructure reference, a provider context source, or an enrichment database.

## Role reference

### No `use` value — normal public feed

A source or merge without a `use:` field is a normal public threat-intelligence feed. It appears in public browsing, gets comparisons, and follows standard health classification.

### `bogons`

IPset-compatible bogon reference. The feed produces a committed set and can be used as a bogon comparison provider. Bogon feeds appear in the public catalog and in per-feed bogon overlap reports.

### `critical_infrastructure`

IPset-compatible critical-infrastructure reference data. The feed produces a committed set and is used as infrastructure reference for overlap checks against other public feeds.

Requires a `critical:` metadata block with `tier`, `role`, `source_type`, `source_quality`, and `rationale`.

### `provider_context`

IPset-compatible broad provider/hosting context. The feed produces a public set but represents broad cloud, hosting, or CDN address space — not critical-infrastructure warning truth. Provider-context feeds help operators understand collateral risk.

Excluded from critical-overlap target generation so broad provider pages do not receive misleading critical-warning artifacts.

### `asn`

Provider-database role. The source provides IP-to-ASN mapping data. It enriches other feeds with ASN attribution but is not a normal public IP set.

### `geoip`

Provider-database role. The source provides IP-to-country mapping data. It enriches other feeds with country attribution but is not a normal public IP set.

## Allowed combinations

Merges can only declare ipset-compatible roles: `bogons`, `critical_infrastructure`, or `provider_context`. This is because merge outputs are set files — they cannot produce ASN or GeoIP databases.

## Forbidden combinations

Validation rejects these combinations:

| Combination | Reason |
|-------------|--------|
| `critical_infrastructure` + `bogons` | Different public artifact semantics and UI meaning |
| `critical_infrastructure` + `provider_context` | Exact reference warnings and broad provider context are separate signals |
| `critical_infrastructure` + `asn` | Critical references produce normal IP set artifacts, not ASN databases |
| `critical_infrastructure` + `geoip` | Critical references produce normal IP set artifacts, not GeoIP databases |
| Merges with `asn` or `geoip` | Merges publish IP sets, not databases |

## Role-based health suppression

Feeds with `use: [critical_infrastructure]`, `use: [provider_context]`, `use: [asn]`, or `use: [geoip]` suppress age-based health states (delayed, risky, unmaintained). This prevents reference and provider feeds from being flagged as stale when their update cadence is naturally slow.

The `bogons` role does not suppress age-based health — bogon feeds are expected to update regularly.

Role-based suppression uses the configured `use:` tags, not feed-name pattern matching.
