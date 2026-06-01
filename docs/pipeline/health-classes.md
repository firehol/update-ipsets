# Health Classes

You will learn what each feed health class means, how the daemon computes health from observed behavior, and when a feed transitions between classes.

## What health measures

Health describes **upstream behavior**, not local pipeline state. A feed that downloads successfully on every attempt is healthy even if processing is slow. A feed that keeps failing downloads is unhealthy even if processing works fine.

## Health classes in order

| Class | Meaning |
|---|---|
| **healthy** | The feed updates on schedule. No problems observed. |
| **delayed** | The feed has a successful local publication, but the last observed change is older than the effective healthy cadence. No action needed yet. |
| **risky** | The feed has a successful local publication, but the last observed change is older than the risky cadence. It may be becoming unreliable. |
| **unmaintained** | The feed still has usable local data, but the last observed change is older than the unmaintained threshold. |
| **unavailable** | The feed has never published successfully, or it is in a current download/provider failure or stale-data state beyond the recovery threshold. |
| **empty** | The latest successful publication produced no IPs. This is not a failure — the upstream source may simply be empty. |
| **archived** | The feed stayed unavailable beyond the archival threshold. It stays in the catalog but stops automatic downloads. |

## How transitions work

Health is computed from observed behavior over time, not from a single check:

- **Single-observation grace** — a feed with only one observed publication stays `healthy` during the configured grace window instead of moving through the age ladder immediately.
- **Cadence floors** — each feed has configured healthy and risky cadence floors. The daemon compares the time since last observed change against those floors and the observed average update interval.
- **Category-specific thresholds** — some categories tolerate longer gaps than others. The daemon uses per-category thresholds when they are configured.
- **Failure streaks** — continuous download/provider failures contribute to `unavailable` independently of the age ladder.

## Special cases

- A newly enabled feed that has never been published starts as `unavailable`. This is expected and brief — new feeds are due immediately.
- A successful `empty` publication is not a download failure, but it has its own `empty` health class so operators can inspect it.
- ASN and GeoIP provider databases, provider-context feeds, critical-infrastructure reference feeds, and feeds with `exclude_from_unmaintained: true` skip the age-based freshness ladder (`delayed`, `risky`, `unmaintained`). They can still be `healthy`, `unavailable`, `empty`, or `archived`.
- History derivatives follow their parent's health, not their own rebuild timestamp.

## Archived feeds

Archived feeds are not removed from the catalog. They remain visible in the UI and API. They stop ordinary automatic downloads and retry scheduling.

You can manually recheck an archived feed. If the recheck succeeds, the feed leaves `archived` naturally.

Archived feeds disable their public download URLs (raw feed body, upstream source link) but keep their analytical and detail pages.

## Merges and health

When an additive merge input feed is `unmaintained` or `archived`, it is excluded from merge composition. This prevents stale inputs from poisoning the merged set.

Subtractive inputs that are disabled, archived, unmaintained, or missing cause the merge to **fail composition** rather than silently broadening the published set. This is a safety behavior.

## See also

- [Feed Status Reference](feed-status-reference.md) — the download statuses that feed into health
- [Enable and Disable](../admin-ui/enable-disable.md) — how enable/disable interacts with health
