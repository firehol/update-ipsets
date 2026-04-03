# Feed Visibility and Lifecycle

You will learn how to control feed visibility, what each lifecycle flag does, and how use roles affect health classification.

## Visibility flags

### `hidden`

Set `hidden: true` to remove a feed from public browsing. The feed remains:

- Visible in the admin UI with full status
- Active in the processing pipeline
- Scheduled normally

Hidden feeds are useful for provider databases, internal references, and feeds that enrich other feeds but should not appear in the public catalog.

```yaml
sources:
  maxmind_geolite2_asn:
    hidden: true
    use: [asn]
    ...
```

### `disabled`

Disabled feeds are not scheduled and not processed. Their enable markers are removed. Use the admin API or CLI to disable/enable feeds.

Disabling a feed that participates in a merge affects the merge composition. See [Merge Feeds](merge-feeds.md) for safety rules.

## Health-related flags

### `exclude_from_unmaintained`

Set `exclude_from_unmaintained: true` to suppress age-based health states: `delayed`, `risky`, and `unmaintained`.

The feed can still receive `empty`, `unavailable`, and `archived` health states — this flag only affects age-based classification.

Use it for feeds with naturally slow update cadences that should not be flagged as stale.

```yaml
sources:
  bogons:
    exclude_from_unmaintained: true
    use: [bogons]
    ...
```

### Role-based health suppression

Feeds with these `use:` roles automatically suppress age-based health states:

- `critical_infrastructure`
- `provider_context`
- `asn`
- `geoip`

These feeds represent reference data, provider context, or enrichment databases. Their update cadence is naturally slow, and flagging them as "unmaintained" would be misleading.

This suppression uses the configured `use:` tags — not feed-name pattern matching.

The `bogons` role does not suppress age-based health. Bogon feeds are expected to update regularly.

## Health states

| State | Meaning |
|-------|---------|
| `healthy` | Feed updates within expected cadence |
| `delayed` | Feed is slower than the healthy cadence floor (age-based, suppressible) |
| `risky` | Feed is much slower than expected (age-based, suppressible) |
| `unmaintained` | Feed appears abandoned (age-based, suppressible) |
| `empty` | Feed body is empty (not suppressible) |
| `unavailable` | Feed consistently fails to download (not suppressible) |
| `archived` | Feed has been continuously unavailable beyond the archival threshold (not suppressible) |

## Archived feeds

Archived feeds have been unavailable for a long time (beyond `feed_health_archival_threshold_minutes`). They remain public catalog targets — their historical data and comparison artifacts persist — but raw downloads stop.

Archived inputs are excluded from merge composition automatically.

## Summary of what each flag controls

| Flag | Public browsing | Admin UI | Processing | Age-based health | Merge eligibility |
|------|----------------|----------|------------|-------------------|-------------------|
| (none) | visible | visible | active | normal | eligible |
| `hidden` | hidden | visible | active | normal | eligible |
| `disabled` | — | visible | stopped | — | excluded |
| `exclude_from_unmaintained` | visible | visible | active | suppressed | eligible |
| archived | visible (stale) | visible | stopped | archived | excluded |
