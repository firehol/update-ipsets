# Single country

When a single country holds almost all of a feed's IPs, this rule reports the share.

## How we calculate this

The rule reads the top country from the preferred geolocation provider's
attribution. If the top country's share exceeds 95%, the rule fires.

## Threshold

`share(top1) > 0.95`

## Sample size

Requires at least 100 IPs in the feed.

## When this rule would be wrong

- Geolocation providers occasionally misattribute large chunks of an ASN to the ASN's registered country rather than the country where the IPs are actually deployed. When this happens the "single country" may be an accounting effect rather than a real geographic concentration.
- Country-specific feeds (e.g. "all IPs in country X") will always trigger this rule. That is the correct observation for them.
- The rule intentionally suppresses the country_concentrated rule when it fires, so the headline you see is always the more specific of the two.
