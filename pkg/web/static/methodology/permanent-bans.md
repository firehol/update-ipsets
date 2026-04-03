# Permanent bans

When a small fraction of IPs are kept in a feed dramatically longer than the bulk of removed IPs, this rule reports the long tail.

## How we calculate this

The rule reads the `past` retention series and computes two percentiles: the 90th (the lowest bucket containing the top 10% of removed IPs) and the 100th (the largest populated bucket — the longest-held IP the feed has ever removed). If p100 is more than 10 times p90, the feed has a permanent-ban tail.

## Threshold

`p100 / p90 > 10.0`

## Sample size

Requires at least 1000 removed IPs. Tail-sensitive rules are especially vulnerable to small samples.

## When this rule would be wrong

- A feed with a single unusually-old outlier (stale snapshot, clock skew at the source) can push p100 far above p90 without a real policy band. The 10x ratio is conservative enough that most single outliers are absorbed into noise, but this is the primary failure mode.
- A feed whose maintainer has been active for many years may show a p100 that reflects the total tracking window rather than an actual retention decision. We accept this because the raw number is still a fact about the observed data.
- A feed we only recently began tracking may not yet have observed its true tail. The rule will simply stay silent until enough removals accumulate.
