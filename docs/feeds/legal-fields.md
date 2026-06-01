# Legal Fields

You will learn how to record license, attribution, and redistribution policy for each feed, when a feed is non-redistributable, and how merges inherit legal status.

## The three legal fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `license` | string | — | SPDX identifier or free-text license description |
| `attribution` | string | — | Required attribution text shown on public pages |
| `redistributable` | boolean | `true` | Whether the raw feed body can be redistributed |

## License

Record the license under which the upstream publishes the data. Use an SPDX identifier when one exists, otherwise use a short free-text description.

Examples:

```yaml
license: CC0 1.0
license: CC BY-NC-SA 4.0
license: GeoLite2 EULA + CC BY-SA 4.0
license: The Unlicense
license: DroneBL community data — software BSD-style
license: Free — no restrictions stated
```

## Attribution

Some licenses require attribution. Record the required text in the `attribution` field. The public website displays this text on the feed detail page.

```yaml
attribution: This product includes GeoLite Data created by MaxMind, available from https://www.maxmind.com
```

## Redistribution policy

### Default: redistributable

Feeds are redistributable by default. Only mark a feed `redistributable: false` when the source terms explicitly forbid copying, redistribution, or republication.

### What is NOT sufficient to mark non-redistributable

These conditions alone do not make a feed non-redistributable:

- Attribution requirements (use `attribution:` instead)
- Non-commercial restrictions
- Warranty disclaimers
- "Use at your own risk" language
- Unknown license with no explicit anti-redistribution language

### When to mark non-redistributable

Mark `redistributable: false` only when the upstream terms explicitly say "do not redistribute" or equivalent language.

### Critical infrastructure feeds

Critical-infrastructure reference feeds follow the same redistribution policy as any other source. The `critical_infrastructure` use role does not make a feed non-redistributable by itself.

Critical reference metadata and overlap results can be public even when the raw feed body is not. Raw-body download and compose routes still enforce the configured `redistributable` value.

## Merge inheritance

Merges inherit legal status conservatively from all transitive parents — including subtractive parents.

If any parent (additive or subtractive) is `redistributable: false`, the merge is also `redistributable: false`. This is because subtractive parents influence the derived artifact even when their ranges are removed from the final set.

```yaml
# If any of these are non-redistributable, firehol_level1 is also non-redistributable
merges:
  firehol_level1:
    sources:
      - dshield          # CC BY-NC-SA 4.0
      - feodo            # CC0 1.0
      - fullbogons       # Free
      - spamhaus_drop    # check license
```

Changing this legal model requires a separate explicit decision — it is not a casual configuration change.
