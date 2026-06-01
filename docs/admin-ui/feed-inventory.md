# Feed Inventory

You will learn how to use the main feed table, what each column means, and how filters work to narrow down what you see.

## Main table

The feed inventory is the primary operator view. It lists every feed in the catalog in a single table.

### Columns

| Column | Meaning |
|---|---|
| **Feed** | Feed identifier, current health dot, kind, version, archived marker, and maintainer when available. |
| **Category** | Configured feed category. |
| **Vis** | Public-visibility and redistribution indicator. |
| **Sched** | Configured frequency. |
| **Actual** | Observed average update interval and cadence ratio. |
| **Next** | Next scheduled check or input-triggered state. |
| **Processed** | When processing last finalized for this feed. |
| **Why** | Why the feed last ran. |
| **Took** | Processing duration for the last run attempt. |
| **Upstream** | Upstream `Last-Modified` timestamp when available. |
| **IPs** | Unique IP count in the current output. |
| **Entries** | Entry count in the current output. |
| **State** | Scheduler state, latest settled status, problem class, and latest error text. |
| **Fail** | Consecutive download failures. |
| **Files** | Current integrity-file state for required and derived files. |
| **Public page** | Opens the public feed page when a public URL is available. |

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

- Identity fields: kind, roles, URLs, maintainer, license, redistribution, visibility, downloader, processor, output, and source-file information.
- Schedule fields: configured frequency, observed cadence, health thresholds, next check, scheduler state, and retry/backoff state.
- Timeline fields: first tracking time, last check, last upstream change, last processed time, next check, and upstream clock skew when detected.
- Content fields: unique IPs, entries, observed ranges, and version.
- Merge details: included inputs, configured subtracted inputs, excluded inputs, and why each excluded input was skipped.
- File manifest: required and optional local files, provider ownership, size, modified time, missing files, and stale files.
- Diagnostics: current health, health detail, last reason, processing time, last status, problem class, download failures, latest error, and integrity recovery plan.

## See also

- [Artifact Inventory](artifact-inventory.md) — managing artifact parents separately
- [Feed Status Reference](../pipeline/feed-status-reference.md) — what each status value means
- [Health Classes](../pipeline/health-classes.md) — how health is computed
