# Health Classes

You will learn what each feed health class means, how the daemon computes health from observed behavior, and when a feed transitions between classes.

## What health measures

Health describes **upstream behavior**, not local pipeline state. A feed that downloads successfully on every attempt is healthy even if processing is slow. A feed that keeps failing downloads is unhealthy even if processing works fine.

## Health classes in order

| Class | Meaning |
|---|---|
| **healthy** | The feed updates on schedule. No problems observed. |
| **delayed** | The feed is late, but still within the grace period. No action needed yet. |
| **risky** | The feed is significantly late. It may be becoming unreliable. |
| **unavailable** | No successful update for a long time. The feed is not producing usable data. |
| **empty** | The feed downloads successfully but consistently produces no IPs. This is not a failure — the upstream source may simply be empty. |
| **unmaintained** | The feed has been unavailable for so long that the upstream source is likely dead. |
| **archived** | The feed has been unavailable beyond the archival threshold. It stays in the catalog but stops automatic downloads. |

## How transitions work

Health is computed from observed behavior over time, not from a single check:

- **Grace period** — a single late check does not immediately move a feed to `delayed`. The daemon applies a single-observation grace before downgrading.
- **Cadence floors** — each feed has a configured expected update interval. The daemon compares the actual observed interval against this floor.
- **Category-specific thresholds** — some categories tolerate longer gaps than others. The daemon uses per-category thresholds when they are configured.
- **Failure streaks** — continuous download failures contribute to unavailability independently of the cadence check.

## Special cases

- A newly enabled feed that has never been published starts as `unavailable`. This is expected and brief — new feeds are due immediately.
- An `empty` feed is healthy from a download perspective. Empty is not failure.
- Provider databases (ASN, GeoIP, bogon) and critical-infrastructure reference feeds skip the age-based freshness ladder (`delayed`, `risky`, `unmaintained`). They can only be `healthy`, `unavailable`, `empty`, or `archived`.
- History derivatives follow their parent's health, not their own rebuild timestamp.

## Archived feeds

Archived feeds are not removed from the catalog. They remain visible in the UI and API. They stop automatic downloads and retry scheduling.

You can manually recheck an archived feed. If the recheck succeeds, the feed leaves `archived` naturally.

Archived feeds disable their public download URLs (raw feed body, upstream source link) but keep their analytical and detail pages.

## Merges and health

When a merge input feed is `unmaintained` or `archived`, it is excluded from merge composition. This prevents stale inputs from poisoning the merged set.

Subtractive inputs that are disabled, archived, unmaintained, or missing cause the merge to **fail composition** rather than silently broadening the published set. This is a safety behavior.

## See also

- [Feed Status Reference](feed-status-reference.md) — the download statuses that feed into health
- [Enable and Disable](../admin-ui/enable-disable.md) — how enable/disable interacts with health
