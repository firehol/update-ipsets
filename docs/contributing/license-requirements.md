# License Requirements

You will learn when a feed can be redistributed, how to handle attribution, and how license policy works for merges.

## Before submitting

Check the upstream feed's terms of service or license page. Look for:

- An explicit license statement
- Terms of use on the download page
- Any restriction on copying or redistributing the data

## Redistributable defaults

By default, feeds are `redistributable: true`. You only need to mark a feed as non-redistributable when the terms explicitly forbid redistribution.

The following do **not** make a feed non-redistributable:

- Attribution requirements (you must give credit)
- Non-commercial use restrictions (the catalog is not commercial)
- Warranty disclaimers ("use at your own risk")
- Unknown license with no explicit anti-redistribution language

If the terms say "free for any use" or "public domain" or have no restrictions at all, the feed is redistributable.

## When to mark non-redistributable

Set `redistributable: false` only when the terms explicitly say:

- "You may not redistribute this data"
- "Redistribution is prohibited"
- "For personal use only — no republication"
- The license requires a separate agreement for redistribution

## Attribution

When the upstream requires attribution, include it in the `attribution` field:

```yaml
attribution: |
  Data provided by Example Corp under CC-BY-4.0.
  Source: https://example.com/feed
  License: https://example.com/terms
```

The attribution text accompanies the published feed data wherever it is served.

## SPDX license identifiers

Use standard SPDX identifiers when the upstream license is recognized:

- `MIT`
- `CC-BY-4.0`
- `CC-BY-SA-4.0`
- `Apache-2.0`
- `BSD-3-Clause`

For non-standard licenses, use a short descriptive string:

```yaml
license: "Custom - free for non-commercial use with attribution"
```

## Merges and license inheritance

Merges inherit license constraints from all their parents — including subtractive parents. If any parent is non-redistributable, the merge is also non-redistributable.

This is conservative by design. Subtractive parents influence the final set (by removing IPs), so their license terms apply to the derived output.

When submitting a merge, verify that all parents are redistributable. If any are not, mark the merge as non-redistributable.

## Critical infrastructure feeds

Critical infrastructure reference feeds in the shipped catalog default to `redistributable: false`. These feeds are curated reference data and get a source-specific redistribution review before any change.

## Quick reference

| Situation | Action |
|-----------|--------|
| Upstream says "public domain" | `redistributable: true` |
| Upstream says "CC-BY" with attribution | `redistributable: true`, include `attribution` |
| Upstream says "free for any use" | `redistributable: true` |
| Upstream says "no redistribution" | `redistributable: false` |
| Upstream says "personal use only" | `redistributable: false` |
| No license mentioned, no restrictions | `redistributable: true` |
| Unsure after reading terms | Ask in the pull request — reviewers will help |
