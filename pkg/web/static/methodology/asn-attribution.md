# ASN attribution

How we attribute a feed's IPs to Autonomous Systems and publish the per-provider
ASN breakdown.

## What it shows

For every feed, the ASN section answers:

- **Which Autonomous Systems own the IPs in this feed?**
- **How much address space is attributed, bogon, or unknown for each ASN provider?**
- **How many distinct ASNs does the feed span?**

The current UI renders:

- provider tabs
- stat tiles for distinct ASNs, attributed IPs, RFC-reserved IPs, and unknown
  IPs
- treemap / bubble / list views for the ASN composition
- direct navigation from rendered ASN rows/nodes to the public ASN detail page

## Data source

ASN providers are supporting reference datasets, not ordinary public threat
feeds.

The public UI presents one tab per available provider. Tab order follows the
site's provider preference.

If the active provider has no per-feed breakdown, or attributes nothing for the
current feed, the tab stays visible and the section shows a provider-local
notice instead of silently hiding that provider.

### Configured ASN sources

| Source name | Upstream | License | Update cadence | Raw data published |
|---|---|---|---|---|
| `iptoasn` | <https://iptoasn.com/> | PDDL v1.0 (Public Domain) | Hourly | Yes |
| `caida_prefix2as` | <https://www.caida.org/catalog/datasets/routeviews-prefix2as/> | CAIDA Acceptable Use Agreement | Daily | No (derived statistics only) |
| `dbip_asn_lite` | <https://db-ip.com/db/download/ip-to-asn-lite> | CC BY 4.0 | Monthly | Yes |
| `maxmind_geolite2_asn` | <https://dev.maxmind.com/geoip/geolite2-free-geolocation-data> | GeoLite2 EULA + CC BY-SA 4.0 | Weekly | No |

### Attribution requirements

The following attribution notices are reproduced here in compliance with each provider's license:

- **iptoasn.com** — public domain (PDDL); no attribution required
- **CAIDA prefix2as** — `The CAIDA UCSD Routeviews Prefix-to-AS mappings (pfx2as), https://catalog.caida.org/dataset/routeviews_prefix_to_as_mappings`
- **DB-IP Lite ASN** — `IP Geolocation by DB-IP (https://db-ip.com)`
- **MaxMind GeoLite2 ASN** — `This product includes GeoLite Data created by MaxMind, available from https://www.maxmind.com`

### Why each provider differs

Different providers can disagree because they observe routing differently:

- `iptoasn` is the freshest available provider and is the site's default ASN
  provider for summary signals.
- `caida_prefix2as` reports BGP origin AS from RouteViews-derived data and can
  prefer upstream/transit ownership over end-operator branding.
- `dbip_asn_lite` is monthly and therefore the stalest available ASN provider.
- `maxmind_geolite2_asn` is weekly and includes organization naming that can
  differ from pure BGP-origin views.

## How counts are computed

Before ASN attribution, the product builds a **bogon union** from the
authoritative RFC-reserved baseline plus trusted bogon reference feeds.

IPs in that union are counted as bogons and are **not** looked up in the ASN
database. Only the remaining address space is attributed to ASNs.

For each available ASN provider, on every relevant processing run:

1. Open the provider's MMDB file
2. Compute the bogon overlap: the IP-level intersection between the feed and the bogon union. This count becomes `bogon_ips`
3. Walk every IPv4 range in the bogon-free residual of the feed
4. For each range, repeatedly look up the lower bound in the MMDB. The MMDB lookup returns both the ASN attribution and the network range over which that attribution is constant
5. Count the IPs from the current cursor up to the smaller of (range end, network end) and assign them to the returned ASN
6. Move the cursor past the end of the matched network and repeat until the range is exhausted
7. any residual address space with no provider record becomes `unknown_ips`

This range-walking design keeps large feeds tractable: the work scales with
distinct attribution regions, not with every individual IP.

### The three-bucket invariant

Every feed's ASN breakdown satisfies:

```
feed_ips == attributed_ips + bogon_ips + unknown_ips
```

| Bucket | Meaning |
|---|---|
| `attributed_ips` | IPs that the ASN database has a real record for. The sum of every row in `by_asn` |
| `bogon_ips` | IPs that fall in the trusted bogon union (RFC reserved baseline plus any external bogon feeds) |
| `unknown_ips` | IPs that are NOT bogon AND have no MMDB record. The residual |

The authoritative RFC-reserved baseline is always present, so `bogon_ips` is
never depends on a third-party feed being available.

| Field | Meaning |
|---|---|
| `feed_ips` | Total IPs in the feed (`attributed_ips + bogon_ips + unknown_ips`) |
| `attributed_ips` | IPs the provider's database has a record for |
| `bogon_ips` | IPs that fall in the trusted bogon union — see [Bogon classification](/methodology/bogon-classification) |
| `unknown_ips` | IPs that are NOT bogon AND have no MMDB record |
| `by_asn[].percent` | `count / feed_ips * 100` (percentage of the entire feed, including bogon and unknown) |

## Current product usage

The current UI reads the active provider breakdown directly:

- stat tiles use `by_asn`, `attributed_ips`, `bogon_ips`, and `unknown_ips`
- the list view uses the full `by_asn` array
- the treemap visual shows the top 80 ASNs by IP count for readability and
  labels that truncation locally
- the bubble visual shows the top 60 ASNs by IP count for readability and
  labels that truncation locally
- rendered ASN labels in the treemap, bubble chart, list, and
  tables navigate to `/asns/{asn}`

The public ASN-detail page also reuses ASN attribution, but with a different
entity-specific semantic:

- it includes every public feed that currently attributes IPs to the selected
  ASN; it is not restricted to the homepage's narrower aggregate subset
- it groups those feeds by category and keeps health / provenance visible on
  each row
- its country map and top-country summaries are built from the IPs that the
  page attributes to the selected ASN, intersected with the active geo
  provider
- it MUST NOT infer ASN-country geography from whole-feed country rankings or
  homepage aggregate summaries

The critical-infrastructure insight rule uses the separate
[critical infrastructure reference-feed overlap](/methodology/infrastructure-asns),
not the ASN attribution breakdown.

## Related

- [Bogon classification](/methodology/bogon-classification)
- [Critical infrastructure reference feeds](/methodology/infrastructure-asns)
- [How we attribute IPs to countries](/methodology/geographic-distribution)

## Edge cases

- `unknown_ips` is always reported explicitly; it is never merged into a real
  ASN row.
- Bogon IPs are never looked up in the provider database at all.
- Different providers can disagree legitimately; the product shows them side by
  side instead of pretending there is one universal answer.
