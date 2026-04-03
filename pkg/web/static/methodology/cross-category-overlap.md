# Cross-category overlap

When a feed in one category has significant overlap with feeds in a different category, this rule reports the relationship.

## How we calculate this

For every compared peer feed, the rule groups rows by the *other* feed's
category and computes the maximum `our_share` seen in each category. It also
counts the number of compared feeds in each category so a single specialist
feed cannot single-handedly trigger the rule.

If the maximum overlap in any non-self category exceeds 30% AND that category has at least 3 compared feeds, the rule fires with the highest-share category.

## Threshold

`max(our_share) > 0.30` in some category `c != own_category`, and `feed_count(c) >= 3`

## Sample size

Requires at least 100 IPs in this feed and at least 3 feeds in the target category.

## When this rule would be wrong

- Category boundaries are fuzzy. A feed labelled "abuse" that overlaps heavily with "attacks" feeds is a real cross-category observation — and also just a note that our taxonomy splits a single phenomenon into two labels. We report the fact; the interpretation is left to the reader.
- The rule reports the maximum share, not the average, so a feed with one outlier-overlap in another category (one "malware" feed at 35%, others at 5%) will fire. The sample-size guard of 3 feeds in the target category prevents a single outlier from being reported.
- `our_share = common / this.ips`, so the denominator is the feed whose page we are looking at. Swapping the perspective would change the number.
