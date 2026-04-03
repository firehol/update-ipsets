# High churn

When a feed changes more than half its IPs on every update, this rule reports the median churn ratio.

## How we calculate this

Every observed update records how many IPs were added, removed, and kept. The
insight reads the last up-to-500 observed changes and computes, for each
change, the churn ratio: `(added + removed) / size`.

The rule takes the median of those ratios across the window. If the median exceeds 50%, the feed has high churn.

## Threshold

`median(churn) > 0.50`

## Sample size

Requires at least 50 churn points.

## When this rule would be wrong

- A feed that publishes cumulative data (same full list every update, no deltas) would show very low churn. A feed that publishes "only the last hour" data will show very high churn. Both are correct observations about the publication cadence, not the underlying threat.
- Size-zero points are skipped so a single empty snapshot cannot divide by zero. Real data rarely has size-zero points.
- Median is sensitive to the distribution's centre, not its extremes. A feed with a pattern of "99% churn on Sunday, 10% churn every other day" may report a median around 10% even though one day a week is explosive.
