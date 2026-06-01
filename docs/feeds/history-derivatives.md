# History Derivatives

You will learn how to create "all IPs seen in the last N days" feeds from an existing parent source or merge, how the time window works, and what anchoring rules apply.

## What a history derivative is

A history derivative is a feed that contains the union of all IPs observed in a parent source or merge during a specified time window. For example: "all IPs that appeared in DShield during the last 24 hours."

Each window becomes its own feed identity with its own name, processed output, and artifacts.

## How to declare history derivatives

History derivatives are declared in the parent source or merge YAML using the `history:` field:

```yaml
sources:
  dshield:
    url: https://feeds.dshield.org/block.txt
    frequency: 10
    history:
      - 1440      # 1 day window
      - 10080     # 7 day window
      - 43200     # 30 day window
    output: netset
    category: intrusion
    ...
```

Each value in `history:` is a number of minutes. The example above creates three derivative feeds:

| Parent | Window (minutes) | Derivative name |
|--------|-------------------|-----------------|
| `dshield` | 1440 (1 day) | `dshield_1d` |
| `dshield` | 10080 (7 days) | `dshield_7d` |
| `dshield` | 43200 (30 days) | `dshield_30d` |

Use whole-hour or whole-day windows. The generated suffix is based on the configured minute value: exact day windows become `<parent>_<N>d`, exact hour windows under one day become `<parent>_<N>h`, and mixed day/hour windows become `<parent>_<D>d<H>h`.

Merges can also own history windows:

```yaml
merges:
  cleantalk:
    frequency: 5
    history:
      - 1440
      - 10080
      - 43200
    sources:
      - cleantalk_new
      - cleantalk_updated
```

This keeps `cleantalk` as the current merge and creates `cleantalk_1d`, `cleantalk_7d`, and `cleantalk_30d` as retention derivatives of that merge output.

## Window semantics

The window is **additive** — it contains the union of all IPs observed in the parent during the last X days of retained history snapshots.

An IP that appeared on day 1 and disappeared on day 3 is still in the 7-day window on day 5. The window is not "currently active IPs" — it is "all IPs seen during the period."

## Anchoring rules

History derivatives are anchored to the parent's successful update times — not to an independent schedule.

- The derivative does not have its own wall-clock cadence.
- The derivative follows the parent's downloader behavior.
- When the parent source or merge updates, all its derivatives are re-evaluated.

## Eligible parents

History derivatives can only be declared on parents that produce committed feed bodies. Provider databases (ASN and GeoIP sources with `use: [asn]` or `use: [geoip]`) are not valid history-derivative parents. Critical-infrastructure reference feeds also cannot declare history windows because they are reference providers, not retention variants.

## Using derivatives in merges

Derivative feeds are first-class feeds. You can reference them in merges:

```yaml
merges:
  firehol_level2:
    sources:
      - blocklist_de
      - dshield_1d
      - greensnow
```

Here `dshield_1d` is a 1-day history derivative of the `dshield` parent.
