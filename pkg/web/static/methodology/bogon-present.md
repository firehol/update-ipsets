# Bogon ranges present

When any fraction of a feed falls into a bogon range (RFC-reserved, private, or flagged by a bogon feed), this rule reports the count and share.

## How we calculate this

The product compares the feed with every available bogon provider: the
authoritative RFC-reserved baseline plus trusted bogon reference feeds. Each
comparison reports the feed size, the bogon overlap count, and the overlap
share.

The insight takes the maximum share across providers, not the union. A feed
counts as "bogon-present" as long as any single bogon dataset identifies an
overlap, without double-counting IPs that appear in multiple bogon datasets.

## Threshold

`bogon_share > 0`

## Sample size

Requires at least 100 IPs in the feed.

## When this rule would be wrong

- A bogon provider that mistakenly flags a real routable range as bogon will produce a false positive. The RFC reserved baseline is authoritative and does not have this failure mode.
- Share is computed against `feed_ips` (the raw count of IPs in the feed), not against some "non-bogon" denominator. This is deliberate: the headline reports "X% of this list is bogon".
- The maximum-share choice means a feed that overlaps provider A at 0.1% and provider B at 0.05% will report 0.1%, not 0.15%.
