# Subset of another feed

When almost every IP in this feed also appears in an older feed, this rule reports the relationship.

## How we calculate this

For every compared peer feed, the rule computes
`our_share = common / this.ips`. If any peer has `our_share > 0.95` and that
peer was first observed before this feed, the rule fires with the highest-share
candidate.

The "older than this" requirement is important: without it, a brand-new feed that happens to include every IP in a small, specialist feed would be reported as a subset of the specialist. That is semantically backwards. We only claim X is a subset of Y when Y existed first.

## Threshold

`our_share > 0.95` for some row where `other.started_date < this.started_date`

## Sample size

Requires at least 100 IPs in this feed and at least 1 compared feed.

## When this rule would be wrong

- The age comparison uses when we first observed each feed, not when the
  maintainer created it. A feed that existed for years before we started
  tracking it may appear "younger" than a feed we tracked earlier.
- The 95% threshold is deliberately conservative. A feed that is a true 94% subset will stay silent here and instead be represented by the pairwise overlap chart.
