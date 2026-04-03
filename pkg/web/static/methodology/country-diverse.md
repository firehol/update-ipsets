# Country diversity

When no single country dominates a feed and the list spans many countries, this rule reports the diversity.

## How we calculate this

The rule reads the same preferred-provider country attribution that the
concentration rule uses. For every country with any attributed IPs, it checks
that the share is below 5%. If every country passes that test and the number of
distinct countries is at least 50, the rule fires.

## Threshold

`share(c) < 0.05 for every country c` and `count(countries) >= 50`

## Sample size

Requires at least 100 IPs in the feed.

## When this rule would be wrong

- A feed listing 80 countries each at 1% is the canonical diverse feed. A feed listing 49 countries each at 2% fails the count guard even though the pattern is basically the same. This is a known threshold effect we accept in exchange for a clean, defensible rule.
- Geolocation providers with small country coverage (e.g. country-level datasets that roll up sub-regions) may under-report the count. Using a widely-used provider mitigates this.
- Hidden geographic concentration can exist inside an ASN that spans multiple countries. This rule only sees country-level attribution; the ASN tab and the pairwise overlap comparison are the right place to look for that.
