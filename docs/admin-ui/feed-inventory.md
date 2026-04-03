# Feed Inventory

You will learn how to use the main feed table, what each column means, and how filters work to narrow down what you see.

## Main table

The feed inventory is the primary operator view. It lists every feed in the catalog in a single table.

### Columns

| Column | Meaning |
|---|---|
| **Name** | Feed identifier from configuration. |
| **Health** | Current health class (healthy, delayed, risky, unavailable, empty, unmaintained, archived). |
| **Status** | Latest settled status from the downloader or processing engine. |
| **Last check** | When the daemon last attempted to refresh this feed. |
| **Last change** | When upstream content last changed. |
| **Last published** | When the daemon last successfully produced outputs for this feed. |
| **Next schedule** | When the next automatic action is expected, and why. |

### Feed kinds

Each feed has a kind label that tells you what family it belongs to:

| Kind | Feed family |
|---|---|
| `source` | Plain upstream feed. |
| `merge` | Synthetic feed composed from multiple inputs. |
| `retention` | History derivative built from a parent feed's retained snapshots. |
| `asn` | Feed surfaced primarily for ASN enrichment. |
| `geolocation` | Feed surfaced primarily for GeoIP enrichment. |
| `bogon` | Feed surfaced primarily for bogon analysis. |

## Filters

Filters are independent. Selecting a health filter does not change what the kind filter shows — the counts on each filter axis update based on the other active filters.

Available filter axes:

- **Health** — healthy, delayed, risky, unavailable, empty, unmaintained, archived.
- **Kind** — source, merge, retention, asn, geolocation, bogon.
- **Category** — the feed's configured category.
- **Hidden** — whether the feed is hidden from public browsing.
- **Disabled** — whether the feed is operationally disabled.

Filter counters are faceted: each count shows how many feeds match that value with all other filters applied.

## Feed detail

Click a feed row to see its detail view. The detail shows:

- Full status and error message (if any).
- Problem class — whether the issue is downloader-stage or processing-stage.
- Failure streak count and duration.
- Last run reason and processing duration.
- For merges: current included inputs, configured subtracted inputs, health-excluded inputs, and why each was excluded.
- File manifest — what files exist for this feed locally.

## See also

- [Artifact Inventory](artifact-inventory.md) — managing artifact parents separately
- [Feed Status Reference](../pipeline/feed-status-reference.md) — what each status value means
- [Health Classes](../pipeline/health-classes.md) — how health is computed
