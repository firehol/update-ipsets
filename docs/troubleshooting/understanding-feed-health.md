# Understanding Feed Health

You will learn how update-ipsets determines feed health, what each health class means, and how health affects the pipeline.

## How health is computed

Health is a backend classification derived from observed update behavior, not a UI label. The system measures how often a feed actually changes compared to its expected cadence.

The model considers:

- **Observed update cadence** — how often the feed produces new content in practice
- **Configured cadence floors** — the expected healthy and risky update intervals
- **Category-specific thresholds** — some categories tolerate slower updates than others
- **Failure streak** — continuous download or processing failures

## Health classes

| Class | Meaning |
|-------|---------|
| `healthy` | The feed updates within its expected cadence |
| `delayed` | The feed is slower than expected but not yet risky |
| `risky` | The feed is significantly overdue — may be abandoning upstream |
| `unmaintained` | The feed has been unhealthy long enough to consider it abandoned |
| `empty` | The feed downloads successfully but produces zero IPs |
| `unavailable` | The feed has no successful local publication yet, or its latest data is stale beyond recovery |
| `archived` | The feed has been continuously unavailable beyond the archival threshold |

## Grace period

The first observation of a feed does not immediately mark it as delayed. A single-observation grace period prevents false health degradation for newly added feeds or feeds that update infrequently by design.

## Cadence floors

Each feed has configurable thresholds for healthy and risky classification:

- If the observed update interval is below the healthy floor, the feed is `healthy`
- If the interval exceeds the risky floor, the feed is `risky`
- Between healthy and risky, the feed is `delayed`

Category-specific thresholds override the global defaults. For example, bogon and geolocation feeds update less frequently than threat lists — their cadence floors reflect that.

## The unavailable-to-archived transition

A feed becomes `archived` when it remains continuously `unavailable` beyond the configured archival threshold. The archival decision considers how long the product has gone without usable local data, not just the current failure streak.

Archived feeds:

- Stop retrying automatically
- Remain visible in the admin UI and public reference surfaces
- Can be manually rechecked by an operator
- May recover to a better health class if the manual recheck succeeds

## How health affects merges

Health directly influences merge composition:

- `archived` inputs are excluded from merge composition
- `unmaintained` inputs are excluded from merge composition
- A missing subtractive input fails the merge rather than broadening the output

This is a safety feature. If a subtractive input (exclusion list) becomes unavailable, the merge stops rather than publishing a set that is broader than configured.

## Checking health

The admin UI shows health for every feed. The admin API exposes health in the feed list:

```bash
curl -s -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds | jq '.[] | {name, health}'
```

## What to do about unhealthy feeds

- **delayed/risky** — monitor. The upstream may be temporarily slow.
- **unmaintained** — check if the upstream source has moved or shut down. Consider removing it from the catalog.
- **unavailable** — check download errors in the admin UI. The feed may need a URL update.
- **archived** — use the admin recheck action to test if the upstream has recovered.
- **empty** — verify the upstream still produces content. An empty result is valid if the feed genuinely has no IPs to report.
