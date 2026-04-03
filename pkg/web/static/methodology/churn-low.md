# Low churn

When a feed barely changes from one update to the next, this rule reports the median churn ratio.

## How we calculate this

The same churn time series the high-churn rule uses: `(added + removed) / size` per recorded update, taken over the last up-to-500 points. The rule fires when the median falls below 5%.

## Threshold

`median(churn) < 0.05`

## Sample size

Requires at least 50 churn points.

## When this rule would be wrong

- A feed whose maintainer pushed an unchanged copy multiple times in a row (same input file, same output) will produce zero-churn entries that drag the median toward 0. We accept this because it is a real observation about how the feed is maintained.
- Both churn rules use the median, not the mean. A feed that swings wildly between zero churn and high churn may not trigger either rule because the median sits between the bands.
- Very small feeds (below 50 recorded points) cannot trigger this rule. The 50-point guard is the same as for size_variation.
