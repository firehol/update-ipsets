# Provider Defaults

You will learn how to configure the canonical ASN and GeoIP providers, why this matters for the public site, and what happens when you change a default.

## What provider defaults are

Provider defaults tell the system which ASN and GeoIP source to use as the canonical provider for IP lookups, homepage summaries, insights, entity pages, and feed-detail pages.

```yaml
defaults:
  asn_provider: iptoasn
  geo_provider: dbip_country
```

## The values are source names — not labels

The `asn_provider` and `geo_provider` values reference configured source names (the key in the `sources:` map), not the human-readable `label` field. Validation rejects a default provider that does not exist or does not carry the matching `use:` role.

- `asn_provider` references a source with `use: [asn]`
- `geo_provider` references a source with `use: [geoip]`

Example — `iptoasn` is a valid ASN default because its source definition includes `use: [asn]`:

```yaml
sources:
  iptoasn:
    url: https://iptoasn.com/data/ip2asn-v4.tsv.gz
    use: [asn]
    ...
```

## Where defaults take effect

Changing the default provider affects:

- **IP lookup context** — when a visitor looks up an IP on the homepage, the ASN and country attribution use the default providers
- **Homepage summaries** — aggregate country and ASN breakdowns use the default providers
- **Insights and entity pages** — generated analysis uses the configured canonical providers
- **Feed-detail default tabs** — the first ASN and GeoIP tab selected on a feed detail page
- **Provider-list ordering** — the default provider appears first in provider-list API responses, followed by other providers in catalog order

If a default is omitted, the engine falls back to the first configured provider for that role in catalog order. The shipped catalog sets explicit defaults, so this fallback is mainly for custom or test configurations.

## Changing a default

Changing `asn_provider` or `geo_provider` is a pipeline-significant configuration change. The daemon detects the drift and rebuilds affected public feed and entity artifacts instead of waiting for the selected provider body to change upstream.

This means: after you change a default and reload, the daemon reprocesses the comparison, GeoIP, and ASN artifacts that depend on the old provider. You do not need to wait for the new provider to publish an update.

## Multiple providers, one default

You can configure multiple ASN sources (for example MaxMind GeoLite2 ASN, iptoasn, CAIDA) and multiple GeoIP sources (MaxMind GeoLite2, DB-IP, IPDeny). All of them produce per-feed artifacts. The default selects which one is shown first and used for homepage-level summaries.

The remaining providers are still available — users can switch tabs on feed-detail pages, and the API returns all providers.
