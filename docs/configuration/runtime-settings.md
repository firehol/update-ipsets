# Runtime Settings

You will learn which knobs control daemon behavior, how to tune concurrency, set health thresholds, and configure web publishing.

## Where runtime settings live

All runtime settings go in `runtime.yaml` at the catalog root. Example from the shipped catalog:

```yaml
runtime:
  base_dir: ${BASE_DIR-${HOME}/ipsets}
  processing_interval_minutes: 5
  min_run_interval_seconds: 30
  max_processing_workers: 2
  max_background_workers: 1
  web_artifact_cache_max_entries: 2048
  web_artifact_cache_max_bytes: 67108864
  web_artifact_cache_max_file_bytes: 8388608
  feed_health_single_observation_grace_minutes: 14400
  feed_health_default_healthy_cadence_minutes: 10080
  feed_health_default_risky_cadence_minutes: 43200
  feed_health_archival_threshold_minutes: 86400
  web_url: https://iplists.firehol.org/ipsets/
  public_base_url: https://iplists.firehol.org
```

## Concurrency domains

The daemon separates work into four independent concurrency pools.

| Domain | Setting | Default | Controls |
|--------|---------|---------|----------|
| Download workers | managed by scheduler | — | Upstream HTTP/HTTPS acquisition and merge composition |
| Feed-processing workers | `max_processing_workers` | 2 | Turning staged downloads into committed feed outputs |
| Heavy-phase workers | `max_heavy_phase_workers` | auto (min(CPU, 8)) | Pairwise comparisons, GeoIP/ASN/bogon fan-out |
| Background workers | `max_background_workers` | 1 | Startup repair, health-transition refreshes, deferred maintenance |

Background work is intentionally low-priority. It prefers finishing later over competing with the main pipeline for CPU and memory.

## Processing cadence

- **`processing_interval_minutes`** — how often the processing queue drains automatically. Default: 5.
- **`min_run_interval_seconds`** — minimum time between scheduler runs. Prevents rapid re-scheduling. Default: 30.

## Cache limits for published artifacts

The public web server caches generated JSON and static artifacts in memory. Raw `.ipset`/`.netset` downloads are streamed separately and do not use this cache.

| Setting | Default | Purpose |
|---------|---------|---------|
| `web_artifact_cache_max_entries` | 2048 | Maximum number of cached files |
| `web_artifact_cache_max_bytes` | 64 MiB | Total cache size across all entries |
| `web_artifact_cache_max_file_bytes` | 8 MiB | Maximum single file size in cache |

## Health thresholds

Health states determine whether a feed is considered healthy, delayed, risky, unmaintained, or archived.

| Setting | Default | Purpose |
|---------|---------|---------|
| `feed_health_single_observation_grace_minutes` | 14400 (10 days) | Grace period before a feed with only one observation gets health-classified |
| `feed_health_default_healthy_cadence_minutes` | 10080 (7 days) | Default upper bound for "healthy" age |
| `feed_health_default_risky_cadence_minutes` | 43200 (30 days) | Default upper bound for "risky" age |
| `feed_health_archival_threshold_minutes` | 86400 (60 days) | Continuous unavailable duration before archival |

Category-specific overrides live in `feed_health_category_thresholds`. For example, `intrusion` feeds use tighter thresholds than `special_use` feeds because intrusion feeds update more frequently.

## Web publishing

| Setting | Purpose |
|---------|---------|
| `public_base_url` | Externally visible base URL of the public website. Used for admin-to-public navigation links. |
| `web_url` | Published feed-detail prefix used in generated metadata and output files. May include a path like `/ipsets/`. |
| `local_copy_url` | Base URL for raw file downloads. |
| `web_dir` | Local directory for published web files. Can be a separate git repository. |

`public_base_url` and `web_url` serve different purposes. `public_base_url` is the website root. `web_url` is the feed-detail path prefix. Do not use them interchangeably.

## Git push options

| Setting | Default | Purpose |
|---------|---------|---------|
| `push_to_git_merged` | true | Commit merged output after processing |
| `push_to_git_web` | — | Also commit the `web_dir` tree if it is a separate repository |

## Environment variable expansion

Runtime settings support `${VAR-default}` shell-style expansion. For example:

```yaml
base_dir: ${BASE_DIR-${HOME}/ipsets}
```

This resolves `$BASE_DIR` if set, otherwise falls back to `$HOME/ipsets`.
