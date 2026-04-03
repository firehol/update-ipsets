# Country concentration

When a small number of countries hold most of a feed's IPs, this rule reports the share.

## How we calculate this

The product computes one country-attribution breakdown per available
geolocation provider.

The insight does **not** average across providers. It uses the site's preferred
GeoIP provider and computes country share against that provider's mapped total.

The rule sums the shares of the top three countries. If that sum exceeds 70%, and the top country alone does not exceed 95% (which would be the single-country rule instead), the rule fires.

## Threshold

`share(top1) + share(top2) + share(top3) > 0.70` and `share(top1) <= 0.95`

## Sample size

Requires at least 100 IPs in the feed and at least 3 countries with any attributed IPs.

## When this rule would be wrong

- Geolocation providers disagree about where IPs are physically located,
  especially for cloud and CDN infrastructure. The rule intentionally uses a
  single preferred provider instead of averaging contradictory answers.
- The fraction is computed against attributed IPs only; IPs the provider cannot place (unknown country) are excluded from the denominator.
- A feed that happens to list three countries each at 25% but leaves the remaining 25% spread across many small countries will fire this rule. That is a correct observation — the top 3 really do hold 75% of the list.
