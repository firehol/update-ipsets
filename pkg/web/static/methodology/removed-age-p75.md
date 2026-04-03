# Removed IP duration (p75)

How we summarise how long removed IPs were kept before being dropped.

## How we calculate this

Every time an observed update drops an IP that was previously listed, the
product records the observed lifetime in hours and aggregates those removals
into the removal-age histogram.

The rule computes the 75th percentile of that histogram: the lowest bucket `H`
such that the cumulative removed count at or below `H` is at least 75% of the
total.

## Threshold

The rule always fires once the sample-size guard is met.

If the observation window is short (30 days or less), the headline includes the
observation window explicitly so users do not mistake a short local tracking
window for the maintainer's full historical policy.

## Sample size

Requires at least 1000 removed IPs. The percentile of a retention histogram is sensitive to the shape of the distribution and noisy below this bound.

## When this rule would be wrong

- A feed that almost never removes IPs (permanent-ban list) will fail the sample-size guard and stay silent.
- A feed whose retention policy has changed twice during our observation window will produce a multimodal distribution; the p75 may land between two policy bands.
- Snapshot-to-snapshot granularity rounds every hours value to the nearest hour, so very short-lived IPs (e.g. 5-minute honeypot hits) show up as 0 h rather than their true lifetime.
