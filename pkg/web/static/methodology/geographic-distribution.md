# Geographic distribution

How we attribute the IPs in each feed to countries and publish the per-provider
country breakdown.

## What it shows

For every feed, the Geographic distribution section answers:

- **Which countries are most represented in this feed right now?**
- **How many countries does the feed touch in total?**

The current UI renders:

- provider tabs
- stat tiles for distinct countries, mapped IPs, RFC-reserved IPs, and unmapped
  IPs
- map and list views for the active provider

## Data source

Country attribution comes from supporting GeoIP datasets. They are supporting
reference datasets, not ordinary public threat feeds.

The feed-detail UI shows one tab per available provider. Tab order follows the
site's provider preference.

If the active provider has no per-feed breakdown, or maps zero countries for
the current feed, the tab stays visible and the section shows a provider-local
notice instead of silently hiding that provider.

| Current source name | Upstream |
|---|---|
| `dbip_country` | <https://db-ip.com/> |
| `geolite2_country` | <https://dev.maxmind.com/geoip/geolite2-free-geolocation-data> |
| `ip2location_country` | <https://lite.ip2location.com/> |
| `ipdeny_country` | <https://www.ipdeny.com/> |
| `ipip_country` | <https://en.ipip.net/> |

Different providers can disagree, especially around cloud, CDN, and
multi-country deployments. The product shows those disagreements explicitly
instead of averaging them away.

## How counts are computed

For each available provider, on every relevant processing run:

1. The feed's IP ranges are compared with the provider's country ranges.
2. IP counts are summed per country code.
3. The public page shows the provider's total mapped IP count and per-country
   counts.

Country names are shown from the standard country code labels.

## Current product usage

- The feed-detail stats row uses the provider's mapped count together with the
  authoritative bogon count to derive mapped / RFC-reserved / unmapped totals.
- The map/list views use the provider's country counts.
- The map includes a local legend explaining that colour intensity tracks
  attributed IP count on a square-root scale.
- The public country-detail page uses the active geo provider to decide which
  public feeds currently contribute to the selected country and to group those
  feeds into category/maintainer composition blocks.
- The country-detail page's `Top ASNs in this country` block is **not** copied
  from whole-feed ASN rankings. It is rebuilt from the canonical feed ranges
  that overlap the selected country and then attributed through the active ASN
  provider.
- The public ASN-detail page's country map reuses the same geo provider, but
  only after first isolating the IPs attributed to the selected ASN. It is
  therefore an ASN-specific country distribution, not a feed-level one.
- Geography insight rules use the site's preferred GeoIP provider.

## Edge cases

- The mapped total can be smaller than the sum of per-country rows when
  provider buckets overlap internally; the UI treats the mapped total as
  authoritative.
- the same caveat applies when the ASN-detail page renders ASN-country
  distributions: per-country row totals can exceed the authoritative
  mapped total when the underlying geo provider has overlapping country buckets
- Reserved/bogon space is normally not geolocatable and is broken out
  separately in the UI.
- A feed with zero mapped IPs can still exist and still be valid.
