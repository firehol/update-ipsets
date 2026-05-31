# License Requirements

You will learn when a feed can be redistributed, how to handle attribution, and how license policy works for source feeds and merges.

## Check the direct upstream

Use the terms of the URL the catalog actually downloads from. Look for:

- an explicit license statement
- terms of use on the download page
- restrictions on copying or redistributing the data
- attribution requirements

Terms from an upstream-of-upstream are useful context, but they do not change the catalog fields unless they apply to the direct download source.

## Redistributable defaults

By default, feeds are redistributable. You only need to set `redistributable: false` when the direct upstream terms explicitly forbid redistribution.

The following do not make a feed non-redistributable by themselves:

- attribution requirements
- non-commercial use restrictions
- warranty disclaimers
- no explicit license statement and no explicit anti-redistribution language

## When to mark non-redistributable

Set `redistributable: false` when the direct upstream terms say:

- redistribution is prohibited
- republication requires a separate agreement
- use is personal-only
- copying the data into a public mirror is not allowed

## Attribution

When the upstream requires attribution, include it in `attribution`:

```yaml
attribution: |
  Data provided by Example Corp under CC-BY-4.0.
  Source: https://example.com/feed
  License: https://example.com/terms
```

The attribution text is carried with public metadata and feed pages.

## SPDX license identifiers

Use standard SPDX identifiers when the upstream license is recognized:

- `MIT`
- `CC-BY-4.0`
- `CC-BY-SA-4.0`
- `Apache-2.0`
- `BSD-3-Clause`

For non-standard terms, use a short descriptive string:

```yaml
license: "Custom - free for non-commercial use with attribution"
```

## Merges

Merges inherit redistribution constraints from all transitive parents, including subtractive parents. If any parent is non-redistributable, the merge is also non-redistributable.

This is conservative by design. Subtractive parents influence the derived output because they remove ranges from it.

## Critical infrastructure feeds

Critical-infrastructure reference feeds in the shipped catalog default to `redistributable: false`. These feeds are curated reference data and need source-specific review before being republished.

## Quick reference

| Situation | Action |
|-----------|--------|
| Direct upstream says "public domain" | `redistributable: true` |
| Direct upstream says "CC-BY" with attribution | `redistributable: true`, include `attribution` |
| Direct upstream says "free for any use" | `redistributable: true` |
| Direct upstream says "no redistribution" | `redistributable: false` |
| Direct upstream says "personal use only" | `redistributable: false` |
| No license mentioned, no restrictions | `redistributable: true` |
