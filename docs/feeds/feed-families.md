# Feed Families

You will learn the six feed families, when to use each one, and how to decide which family fits your use case.

## The six families

| Family | Description | Example |
|--------|-------------|---------|
| **Source feed** | Direct upstream feed from HTTP/HTTPS/local file | `dshield` — downloads DShield block list every 10 minutes |
| **Static feed** | IP/CIDR list defined directly in YAML | `critical_public_dns_core` — curated list of public DNS resolvers |
| **Artifact-backed child** | Feed derived from a downloaded artifact parent | `dronebl_anonymizers` — extracted from the DroneBL buildzone artifact |
| **History derivative** | Time-window feed from a parent's retained snapshots | `dshield_1d` — all IPs seen in DShield during the last 24 hours |
| **Merge** | Synthetic feed composed from other feeds | `firehol_level1` — union of dshield, feodo, fullbogons, spamhaus_drop |
| **Provider database** | ASN, GeoIP, or bogon enrichment source | `maxmind_geolite2_asn` — IP-to-ASN mapping database |

## Decision guide

**I have an upstream URL that publishes IPs or CIDRs.**
Use a source feed. Set `url`, `frequency`, `output`, and `processor`.

**I have a small curated list that operators should customize.**
Use a static feed. Set `static:` with the IP/CIDR list. Set `frequency: 0` for config-change-only updates.

**The upstream publishes a single large file containing multiple feed categories.**
Use an artifact parent with artifact-backed children. Define the artifact in `artifacts/`, then define child feeds that reference it with `artifact://<name>?parts=<parts>`.

**I want "all IPs seen in the last N days" for an existing feed.**
Use a history derivative. Add `history:` to the parent source or merge definition.

**I want to combine multiple feeds into one, with optional exclusions.**
Use a merge. Define it in `merges/` with `sources` and optional `exclude`.

**I have an ASN or GeoIP database that enriches other feeds.**
Use a provider database. Configure it as a normal source with `use: [asn]` or `use: [geoip]`.

## Detailed pages

- [Source Feeds](source-feeds.md) — direct upstream feeds
- [Static Feeds](static-feeds.md) — config-backed curated lists
- [Merge Feeds](merge-feeds.md) — composed feeds with union/exclude
- [Artifact Parents](artifact-parents.md) — downloadable artifacts that produce child feeds
- [History Derivatives](history-derivatives.md) — time-window feeds from parent history
- [Provider Databases](provider-databases.md) — ASN, GeoIP, and bogon enrichment sources
