# Provider Databases

You will learn how ASN, GeoIP, and bogon sources are configured, how they differ from normal public feeds, and where they appear in the system.

## What a provider database is

Provider databases are enrichment sources that add context to other feeds. They are not threat intelligence — they provide ASN attribution, country attribution, or bogon reference data.

The three provider database roles:

| Role | `use:` tag | Purpose |
|------|-----------|---------|
| ASN | `use: [asn]` | Maps IPs to autonomous system numbers and names |
| GeoIP | `use: [geoip]` | Maps IPs to country codes |
| Bogons | `use: [bogons]` | Reference set of private, reserved, and non-routable addresses |

## How they are configured

Provider databases are configured as normal source feeds with an added `use:` role:

**ASN example:**

```yaml
sources:
  iptoasn:
    url: https://iptoasn.com/data/ip2asn-v4.tsv.gz
    frequency: 1440
    category: asn
    use: [asn]
    ...
```

**GeoIP example:**

```yaml
sources:
  geolite2_country:
    url: https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country-CSV&license_key=${MAXMIND_LICENSE_KEY}&suffix=zip
    frequency: 10080
    category: geolocation
    use: [geoip]
    hidden: true
    format: maxmind_country_csv
    ...
```

**Bogon example:**

```yaml
sources:
  bogons:
    url: https://team-cymru.org/Services/Bogons/bogon-bn-agg.txt
    frequency: 1440
    category: special_use
    use: [bogons]
    ...
```

## How they differ from normal feeds

Provider databases do not appear as normal public feeds in the browsing catalog. Their purpose is enrichment:

- **ASN databases** produce per-feed ASN breakdowns. When you visit a feed detail page and see "40% of IPs belong to AS12345," that attribution comes from the configured ASN provider.
- **GeoIP databases** produce per-feed country breakdowns and country-level comparison pages.
- **Bogon sources** produce per-feed bogon overlap reports, showing how many IPs in a feed are private or reserved.

## Multiple providers

You can configure multiple ASN sources and multiple GeoIP sources. Each produces independent per-feed artifacts. Users can switch between providers on feed-detail pages.

One provider is selected as the default via `defaults.yaml`:

```yaml
defaults:
  asn_provider: iptoasn
  geo_provider: dbip_country
```

The default provider is used for homepage summaries, IP lookup context, and the first tab shown on feed-detail pages.

## Hidden from public browsing

Most provider databases set `hidden: true`. They are not shown in the public feed list but are still active in the processing pipeline. The admin UI shows them with full status and metadata.

Bogon sources are typically not hidden — they appear in the `special_use` category as public feeds alongside their enrichment role.

## Download cadence

Provider databases have their own download cadence, independent of the feeds they enrich. GeoIP databases typically update weekly or monthly. ASN databases update daily. Bogon sources update daily.

When a provider database updates, the daemon reprocesses all enrichment artifacts that depend on it.
