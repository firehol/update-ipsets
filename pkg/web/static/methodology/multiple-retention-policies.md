# Multiple retention policies

When a list shows two distinct retention windows for removed IPs — a fast window for ephemeral entries and a slow window for confirmed bad actors — this rule reports it.

## How we calculate this

For each IP that was removed from the list, the product records how long it was
listed before removal and aggregates the distribution into the removal-age
histogram.

The rule computes the 50th and 90th percentiles of these durations (the lowest hour bucket at which the cumulative removed count reaches 50% and 90% of the total). If the 90th percentile is more than 5 times longer than the 50th, the list has two distinct retention windows.

## Threshold

`p90 / p50 > 5.0`

## Sample size

Requires at least 1000 removed IPs. Below this the percentiles are too noisy to trust.

## When this rule would be wrong

- A list with very few removals (most IPs are kept indefinitely) — mitigated by the 1000-removed minimum.
- A list with a smooth log-normal distribution of retention times can produce a ratio above 5 without a true bimodal split. We accept this false positive in exchange for catching the more common bimodal case.
- A feed whose maintainer changed retention policy once during our observation window may look bimodal because we are aggregating across the policy change. This is a real fact worth surfacing, even if the live maintainer has only one policy today.
