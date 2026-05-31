# Glossary

You will learn the key terms used throughout the update-ipsets documentation.

## A

**archived** — A health class for feeds that have been continuously unavailable beyond the configured archival threshold. Archived feeds stop automatic retry scheduling but remain visible in the admin UI and public reference surfaces.

**artifact parent** — An upstream artifact that is not itself a public feed. It materializes one or more child feeds from a single download. Examples include DNSBL-style zone downloads.

## B

**bogons** — IP addresses that should not appear on the public internet: reserved, private, and unallocated ranges. Detected by comparing feeds against configured bogon providers.

## C

**cache-first** — The serving model where public requests read precomputed artifacts from disk. Public requests do not trigger downloads, processing, or recomputation.

**cadence** — The configured update interval for a feed. Expressed in minutes. Frequency 0 means the feed does not own an independent wall-clock cadence.

**canonical feed body** — The normalized, deduplicated IP set produced by the downloader from raw upstream material. The processing engine consumes only canonical feed bodies.

**category** — A configuration-defined group for classifying feeds, such as `intrusion`, `malware_infrastructure`, or `provider_infrastructure`. Categories have labels, descriptions, colors, ordering, and public visibility.

**child feed** — A feed derived from an artifact parent or history derivative. Not an independent upstream download.

**committed feed body** — The authoritative on-disk feed body after successful processing. Used as the baseline for comparisons and as the fallback input for reprocessing.

**comparison** — Pairwise overlap analysis between two feeds. Produces metrics like shared IPs, overlap percentage, and inclusion relationships.

**compose** — Build a new IP set from multiple feeds using union and exclusion operations. Available via the CLI `query --set` command and the `/api/v1/compose` endpoint.

**config catalog** — The directory of YAML files that defines all feeds, merges, artifacts, categories, and runtime settings. Lives at `configs/firehol/` in the repository and `/opt/update-ipsets/etc/config/` when installed.

**critical infrastructure** — Networks and services whose compromise would cause widespread harm (DNS root, CDNs, cloud control planes, etc.). Detected by comparing feeds against configured reference feeds.

## D

**derivative** — A feed derived from another feed. Includes history derivatives and artifact-backed children.

**disabled** — A feed that is not enabled for scheduling or processing. The feed's data remains on disk but is not refreshed.

**downloader loop** — The daemon subsystem responsible for fetching, composing, and staging feed bodies. Runs independently from the processing loop.

## E

**empty** — A valid download result that produces zero IPs. Empty is not failure. The feed is published as an empty set.

**enable marker** — A durable on-disk flag that marks a feed as operationally enabled. Separate from the feed body and configuration.

**entity artifact** — Published country or ASN detail pages derived from per-feed geographic and ASN attribution data.

## F

**feed body** — The canonical IP set content for a feed. Can exist in three states: staged (`.new`), processing (`.processing`), or committed.

**feed family** — The classification of how a feed obtains its data: plain source, history derivative, merge, or artifact-backed child.

## G

**geolocation** — Country-level IP attribution derived from configured GEO providers.

**grace period** — A single-observation window where a feed is not immediately penalized for missing an update. Prevents false health degradation for infrequently updated feeds.

## H

**health** — A backend classification of feed update behavior. Classes: healthy, delayed, risky, unmaintained, empty, unavailable, archived.

**health class** — The specific health value assigned to a feed. Derived from observed cadence, failure streaks, and configured thresholds.

**heavy phase** — The processing stage that performs global enrichment: pairwise comparisons, GEO fan-out, ASN fan-out, and bogon analysis.

**hidden** — A feed excluded from public browsing but still visible in admin and still processed unless disabled.

**history derivative** — A feed that represents the union of IPs observed in a parent feed over a configured time window. Updated when the parent updates, not on its own schedule.

## I

**integrity** — Verification that local on-disk state matches the last successful publication. Integrity is separate from health. Integrity answers: is our local data self-consistent? Health answers: is the upstream source behaving normally?

**ipset** — An output format where each IP address appears on its own line. CIDR inputs are expanded into individual IPs.

## L

**latest set** — The file-backed binary set snapshot stored at `lib/{name}/latest`. Used for fast lookups via mmap or pread.

**lib directory** — The working directory where the daemon stores binary range snapshots, history ledgers, retention data, provider databases, entity sidecars, and other analysis state. Default install path: `/opt/update-ipsets/lib/`.

## M

**maintenance mode** — Not a distinct mode. The daemon is always operational. Background work like entity rebuilds runs with bounded concurrency.

**maintainer** — The person or organization responsible for a feed's upstream source.

**merge** — A synthetic feed composed from multiple other feeds. Produces `union(sources) - union(exclusions)`. Has its own cadence but does not fetch upstream directly.

**merge exclusion** — The safety behavior where missing or unhealthy subtractive parents cause the merge to fail rather than publish a broader set.

**missing input** — A condition where a feed or merge cannot proceed because required local data is not available.

## N

**netset** — An output format where each CIDR prefix appears on its own line. Individual IPs render as `/32` prefixes.

**not modified** — A download result where upstream confirmed the content has not changed (HTTP 304 or equivalent).

## O

**operator** — The person deploying and running update-ipsets. The intended reader of this documentation.

**output family** — The canonical output format: `ipset` or `netset`.

## P

**pairwise comparison** — Overlap analysis between every pair of public feeds. Produces shared IP counts, overlap percentages, and inclusion metrics.

**pipeline** — The end-to-end flow from download through processing to publication. Consists of two loops: the downloader loop and the processing loop.

**processing engine** — The subsystem that turns canonical feed bodies into published artifacts: metadata, history, comparisons, enrichment, insights.

**processing loop** — The daemon subsystem that processes staged feed bodies. Runs independently from the downloader loop.

**processing timestamp** — The time when the application successfully produced and committed outputs. Used as the reference for integrity checks.

**provider** — A supporting dataset used for enrichment (ASN, geolocation, bogon) or critical infrastructure reference.

**provider context** — Broad cloud/hosting address ranges shown as collateral-risk context. Not critical infrastructure warnings.

**provider database** — A supporting dataset (ASN, geolocation, bogon) used for feed enrichment. Not a public feed.

**provenance** — The origin classification of a feed: primary, secondary_upstream, secondary_merge, or secondary_retention.

**published artifacts** — The public outputs generated by processing: metadata, history, comparisons, retention summaries, insights, and enrichment views.

## R

**redistribution** — Whether a feed's raw data may be republished. Defaults to allowed unless upstream terms explicitly forbid it.

**recheck** — An operator action that fetches fresh upstream data and queues processing, regardless of schedule or whether content changed.

**reprocess** — An operator action that reruns processing from existing local data without fetching new content.

**retention** — Analysis of how long IPs remain listed in a feed. Tracks current-membership duration and removed-entry lifetimes.

**retention window** — The configured time span for a history derivative (e.g., 7 days, 30 days).

## S

**same** — A download result where the fetched content is semantically identical to the current local version. No processing is needed.

**scheduled cadence** — The frequency at which the downloader checks a feed for updates.

**search** — The IP lookup function. Given an IP, returns which feeds contain it.

**set** — A collection of IP addresses or CIDR ranges. The fundamental data structure in update-ipsets.

**source** — A feed that downloads its data from an upstream URL or reads it from a local file.

**source timestamp** — The time when feed content is believed to have last changed. Derived from upstream metadata or local acquisition time.

**staged feed body** — A complete, validated feed body written to `.new`, waiting to be claimed by the processing engine.

**static feed** — A feed whose content comes from the `static:` YAML field in configuration, not from a URL. Used for small curated lists.

**status** — The current operational state of a feed: downloading, processing, updated, empty, disabled, failed, etc.

**subtractive** — A merge input listed in `exclude`. Its IPs are removed from the merge output. Missing subtractive inputs fail the merge as a safety measure.

**suppress** — To exclude a finding from integrity reporting because it is not actionable (e.g., an upstream feed that no longer exists).

## T

**timestamp** — A point in time used for tracking feed events. Three types: source timestamp (upstream change), processing timestamp (local publication), wallclock timestamp (runtime operations).

**two-loop model** — The daemon architecture with separate downloader and processing loops. Each loop has its own queue, concurrency control, and state.

## U

**unavailable** — A health class for feeds that have no successful local publication, or whose data is stale beyond recovery thresholds.

**unmaintained** — A health class for feeds that have been unhealthy long enough to consider them abandoned by their upstream provider.

**upstream** — The original source of a feed's data (a URL, file, or artifact).

**use role** — A configuration tag that assigns a feed to an engine role: bogons, critical_infrastructure, provider_context, asn, or geoip.

## W

**web directory** — The directory where published public artifacts are written for the web server to serve. Default: `/opt/update-ipsets/data/web/`.
