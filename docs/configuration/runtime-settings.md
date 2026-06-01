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
  feed_health_category_thresholds:
    intrusion:
      healthy_cadence_minutes: 1440
      risky_cadence_minutes: 10080
  skip_comparison_if_no_updates: true
  web_url: https://iplists.firehol.org/ipsets/
```

## Concurrency domains

The daemon separates work into four independent concurrency pools.

| Domain | Setting | Default | Controls |
|--------|---------|---------|----------|
| Download workers | `parallel_downloads` | 5 | Upstream HTTP/HTTPS acquisition and merge composition |
| Feed-processing workers | `max_processing_workers` | 2 | Turning staged downloads into committed feed outputs |
| Heavy-phase workers | `max_heavy_phase_workers` | auto (min(CPU, 8)) | Pairwise comparisons, GeoIP/ASN/bogon fan-out |
| Background workers | `max_background_workers` | 1 | Startup repair, health-transition refreshes, deferred maintenance |

Background work is intentionally low-priority. It prefers finishing later over competing with the main pipeline for CPU and memory.

## Processing cadence

- **`processing_interval_minutes`** — how often the processing queue drains automatically. Default: 5.
- **`min_run_interval_seconds`** — minimum time between scheduler runs. Prevents rapid re-scheduling. Default: 30.
- **`skip_comparison_if_no_updates`** — accepted optimization flag for no-update processing runs. Default: `true`. Ordinary no-update runs do not publish public artifacts; use explicit recheck/reprocess or provider/default changes when you need to force regeneration.

## Download and parsing limits

These settings control acquisition and normalization cost.

| Setting | Default | Purpose |
|---------|---------|---------|
| `max_connect_time` | 10 seconds | Maximum time to establish an upstream connection |
| `max_download_time` | 300 seconds | Maximum time allowed for one download |
| `max_download_size` | 100 MiB when unset or `0` | Maximum response body size; `-1` disables the cap |
| `ignore_repeating_download_errors` | 10 | Accepted runtime field. Current retry timing is driven by failure count and health class, not this value. |
| `parallel_dns_queries` | 10 | DNS lookups allowed in parallel while parsing hostname-based input |
| `user_agent` | FireHOL default | User-Agent sent to upstream HTTP servers |

Artifact parents can override `max_download_size` in their own artifact definition.

## Cache limits for published artifacts

The public web server caches generated JSON and static artifacts in memory. Raw `.ipset`/`.netset` downloads are streamed separately and do not use this cache.

| Setting | Default | Purpose |
|---------|---------|---------|
| `web_artifact_cache_max_entries` | 2048 | Maximum number of cached files |
| `web_artifact_cache_max_bytes` | 64 MiB | Total cache size across all entries |
| `web_artifact_cache_max_file_bytes` | 8 MiB | Maximum single file size in cache |

## Output reduction and charts

| Setting | Default | Purpose |
|---------|---------|---------|
| `ipset_reduce_factor` | 20 | Accepted compatibility field. The current Go publishing pipeline does not use it for public outputs. |
| `ipset_reduce_entries` | 65536 | Accepted compatibility field. The current Go publishing pipeline does not use it for public outputs. |
| `web_charts_entries` | 500 | Number of historical points used for generated chart data |

## Health thresholds

Health states determine whether a feed is considered healthy, delayed, risky, unmaintained, or archived.

| Setting | Default | Purpose |
|---------|---------|---------|
| `feed_health_single_observation_grace_minutes` | 14400 (10 days) | Grace period before a feed with only one observation gets health-classified |
| `feed_health_default_healthy_cadence_minutes` | 10080 (7 days) | Default upper bound for "healthy" age |
| `feed_health_default_risky_cadence_minutes` | 43200 (30 days) | Default threshold for "risky" age; the unmaintained threshold is double this value |
| `feed_health_archival_threshold_minutes` | 86400 (60 days) | Continuous unavailable duration before archival |
| `feed_health_category_thresholds` | empty map | Per-category overrides for healthy and risky cadence thresholds |

Category-specific overrides live in `feed_health_category_thresholds`. For example, `intrusion` feeds use tighter thresholds than `special_use` feeds because intrusion feeds update more frequently.

Each category override has two required fields:

| Field | Purpose |
|---|---|
| `healthy_cadence_minutes` | Upper bound for "healthy" age in that category |
| `risky_cadence_minutes` | Threshold for "risky" age in that category; the unmaintained threshold is double this value |

Both values must be positive, and `healthy_cadence_minutes` must be lower than `risky_cadence_minutes`.

## Web publishing

| Setting | Purpose |
|---------|---------|
| `public_base_url` | Externally visible base URL of the public website. Used for admin-to-public navigation links. |
| `web_url` | Published feed-detail prefix used in generated metadata and output files. May include a path like `/ipsets/`. |
| `local_copy_url` | Base URL for raw file downloads. |
| `web_dir` | Local directory for published web files. Can be a separate git repository. |
| `web_dir_for_ipsets` | Local directory for downloadable `.ipset` and `.netset` files. |
| `web_owner` | Optional filesystem owner applied to published web files. |
| `github_changes_url` | URL prefix used in metadata links to feed-output commits. |
| `github_setinfo` | URL prefix used in metadata links to setinfo files. |

`public_base_url` and `web_url` serve different purposes. `public_base_url` is the website root. `web_url` is the feed-detail path prefix. Do not use them interchangeably.

## Git push options

| Setting | Default | Purpose |
|---------|---------|---------|
| `push_to_git` | false | Enable git commits/pushes for generated outputs |
| `push_to_git_merged` | true | Commit merged output after processing |
| `push_to_git_commit_options` | empty | Extra options passed to `git commit` |
| `push_to_git_push_options` | empty | Extra options passed to `git push` |
| `push_to_git_web` | false | Also commit the `web_dir` tree if it is a separate repository |

## Runtime paths

These settings control where the daemon stores state. The installed systemd unit normally sets matching environment variables instead of requiring YAML edits.

| Setting | Purpose |
|---------|---------|
| `base_dir` | Committed `.ipset`, `.netset`, `.source`, `.setinfo`, and enable-marker files |
| `config_file` | Legacy bash config file path used by compatibility loaders |
| `run_parent_dir` | Parent directory for the process lock |
| `lock_file` | Explicit lock-file path |
| `cache_dir` | Scheduler/runtime cache directory |
| `lib_dir` | Binary snapshots, history ledgers, retention data, provider state, and entity sidecars |
| `admin_supplied_ipsets` | Admin-managed supplemental catalog directory |
| `distribution_supplied_ipsets` | Distribution-packaged supplemental catalog directory |
| `user_supplied_ipsets` | User-managed supplemental catalog directory |
| `history_dir` | Feed history snapshot directory |
| `errors_dir` | Download error-log directory |
| `tmp_dir` | Temporary download, extraction, and staging directory |

See [Filesystem Layout](../installation/filesystem-layout.md) for the installed directory tree.

## Network trust and local apply

| Setting | Default | Purpose |
|---------|---------|---------|
| `trust_proxy_headers` | false | Trust `X-Forwarded-For` and `X-Real-IP` for client IP detection |
| `trust_cloudflare_headers` | false | Trust `CF-Connecting-IP` for client IP detection |
| `ipsets_apply` | true for root, false for non-root | Apply generated sets to kernel ipset when supported |

Only enable trusted-header settings when every request reaches the daemon through the trusted proxy. Direct client access with these enabled lets clients choose their apparent IP address.

## Environment variable expansion

Runtime settings support `${VAR-default}` shell-style expansion. For example:

```yaml
base_dir: ${BASE_DIR-${HOME}/ipsets}
```

This resolves `$BASE_DIR` if set, otherwise falls back to `$HOME/ipsets`.

That example shows the YAML template value. When the daemon runs as a non-root
user and the path settings are unset or still equal to the built-in defaults,
runtime resolution relocates the main state paths before expansion:

| Runtime path | Effective non-root default |
|---|---|
| `base_dir` | `$HOME/.update-ipsets/ipsets` |
| `run_parent_dir` | `$HOME/.update-ipsets/run` |
| `cache_dir` | `$HOME/.cache/update-ipsets` |
| `lib_dir` | `$HOME/.local/share/update-ipsets` |

Explicit YAML values or environment-variable overrides take priority over this
non-root relocation.
