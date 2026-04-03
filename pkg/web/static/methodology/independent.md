# Independent feed

When the majority of a feed's IPs do not appear in any other feed we track, this rule reports the unique share.

## How we calculate this

The pairwise comparison step produces one comparison row per other tracked
feed: the other feed's size, and the number of IPs the two feeds have in
common. For every row the insight computes
`our_share = common / this.ips` — the fraction of *this* feed that also
appears in the other.

It then takes the maximum of `our_share` across every row. If that maximum is below 10%, the feed is "independent": no single other feed contains more than 10% of this feed's IPs.

## Threshold

`max(our_share) < 0.10` across at least 5 compared feeds.

## Sample size

Requires at least 100 IPs in this feed and at least 5 other feeds compared against.

## When this rule would be wrong

- The rule only considers the single feed with the largest overlap. A feed whose top overlap is 9% but whose 10 next overlaps are each 8% is still reported as "independent" even though its IPs are heavily shared across many feeds.
- The unique fraction reported in the headline is `1 - max(our_share)`, not the true set-theoretic unique fraction. A more precise "IPs in this feed that appear in NO other feed" would require computing the union of every other feed, which is expensive. The conservative max-based estimate is good enough and always an upper bound on the real unique share.
