# Merge Feeds

You will learn how to compose a feed from other feeds using union and exclude, how health-based exclusions work, and the safety rules that prevent accidental broadening.

## What a merge is

A merge is a first-class feed composed from other feeds. Its set expression is:

```
union(sources) - union(exclude)
```

Additive inputs are evaluated first, then subtractive inputs remove ranges from the result.

Merges do not fetch upstream content. They compose their output from the latest durable local canonical feed bodies of their inputs.

## Key fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes (YAML key) | Unique merge identifier |
| `frequency` | no | Minutes between composition attempts. If omitted or `0`, the merge uses `runtime.processing_interval_minutes`. |
| `sources` | yes | List of additive input feed names |
| `exclude` | no | List of subtractive input feed names |
| `history` | no | History-derivative windows, in minutes, generated from the merge output |
| `output` | yes | `ipset` or `netset` |
| `category` | yes | Category key for browsing |

## Simple merge example

```yaml
merges:
  firehol_level1:
    frequency: 5
    ipv: ipv4
    output: netset
    category: intrusion
    info: A firewall blacklist providing maximum protection with minimum false positives.
    maintainer: FireHOL
    maintainer_url: http://iplists.firehol.org/
    sources:
      - dshield
      - feodo
      - fullbogons
      - spamhaus_drop
```

This merge composes every 5 minutes from four inputs.

## Merge with exclusion example

```yaml
merges:
  cymru_unassigned:
    label: Team Cymru unassigned
    frequency: 1440
    ipv: ipv4
    output: netset
    category: special_use
    use: [bogons]
    license: Free — no restrictions stated
    info: Team Cymru fullbogons with traditional bogons subtracted, leaving RIR-allocated space not assigned to an ISP.
    maintainer: Team Cymru
    maintainer_url: http://www.team-cymru.org/
    sources:
      - fullbogons
    exclude:
      - bogons
```

This merge takes `fullbogons` and removes `bogons`, producing "unassigned but allocated" space.

## Health-based exclusions

Archived and unmaintained additive inputs are excluded from merge composition automatically. This prevents stale or dead feeds from polluting the merge output.

If an additive input feed transitions to one of these health states, the merge excludes it on the next composition cycle. If a subtractive input transitions to one of these health states, the merge fails instead; publishing without the exclusion would broaden the output.

## Safety: subtractive input missing

If any configured subtractive input is disabled, archived, unmaintained, or missing, the merge composition fails rather than publishes a broader-than-configured set. This is a deliberate safety measure — without the exclusion, the merge output would silently include ranges that the operator expected to be removed.

The merge waits for the next cadence tick or an explicit operator action (enable/reprocess) to retry.

## Safety: additive input missing body

If any eligible additive input lacks a durable local canonical feed body (never successfully downloaded), the merge composition attempt fails. The merge does not publish partial results.

## Safety: no eligible additive inputs

If no additive inputs remain eligible (all are disabled, archived, or unmaintained), the merge is operationally disabled for composition. It does not produce an empty set — it simply waits.

## Legal inheritance

Merges inherit `redistributable: false` conservatively from all transitive parents. This includes subtractive parents, because those parents influence the derived artifact even when their ranges are removed from the final set.

If any parent (additive or subtractive) is non-redistributable, the merge is also non-redistributable.

## Merge cadence

Merges have their own `frequency` and run on their own schedule. Re-enabling a previously disabled input does not force immediate recomposition — the merge waits for its cadence tick or an operator reprocess action.

## History windows on merges

A merge can declare `history:` when the window should apply to the merged output, not to each input independently.

```yaml
merges:
  cleantalk:
    frequency: 5
    history: [1440, 10080, 43200]
    sources:
      - cleantalk_new
      - cleantalk_updated
```

The base feed remains a merge. The suffixed feeds, such as `cleantalk_7d`, are generated as history derivatives of that merge.
