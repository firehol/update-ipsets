# Feed Status Reference

You will learn every status value a feed can show, what it means in plain terms, and how to tell whether a status indicates a problem.

## Terminal statuses

These are settled outcomes. The feed is no longer in progress.

| Status | Meaning | Action needed? |
|---|---|---|
| `downloaded` | Content was downloaded successfully and has IPs/CIDRs. | No. |
| `not_modified` | Upstream confirmed the source has not changed since last check. | No. |
| `same` | Downloaded successfully, but the normalized content matches the current local version. | No. |
| `empty` | Downloaded successfully, but no IPs/CIDRs were found in the source. | Check upstream if unexpected. |
| `skipped` | Not checked this cycle. The feed is not due, wrong cadence, or not applicable. | No. |
| `failed` | The download or composition attempt failed. | Check logs. May auto-retry. |
| `disabled` | The feed is operationally disabled and was not checked. | No. Expected if you disabled it. |

## In-progress statuses

These mean the feed is actively being worked on right now.

| Status | Meaning |
|---|---|
| `downloading` | The downloader is actively fetching or composing the feed body. |
| `materializing` | An artifact parent is being processed and child feed bodies are being generated from it. |

## Error statuses

These indicate a specific failure during download or composition.

| Status | Meaning | Typical cause |
|---|---|---|
| `missing_env` | A required environment variable is not set. | The URL template references a variable that is undefined. |
| `url_resolve_failed` | The URL or reference could not be resolved into a usable source. | Bad template, missing variable, or invalid local path. |
| `download_failed` | The HTTP fetch or local acquisition failed. | HTTP 404/410/5xx, timeout, DNS failure, missing merge input, missing artifact child input. |
| `prepare_failed` | Source material was obtained, but normalization into a canonical feed body failed. | Corrupt archive, unexpected format, processor pipeline error. |
| `history_snapshot_failed` | A history-derivative parent snapshot could not be retained. | Disk full, permission error, or corrupt parent state. |

## Processing-stage statuses

These are set by the processing engine during or after processing runs.

| Status | Meaning |
|---|---|
| `updated` | Processing completed successfully. Artifacts were published. |
| `processing` | The feed is currently being processed. |
| `cancelled` | Processing was cancelled before completion (e.g. daemon shutdown). |
| `running` | A processing run attempt has started. |
| `missing_input` | No feed body was found to process. Staged and committed bodies are both absent. |
| `parse_failed` | Source parse failure during processing. |
| `finalize_failed` | Finalization step failed during processing. |
| `retention_failed` | Retention history update failed during processing. |

## How to read status in the admin UI

The admin UI shows an operator-facing label, not a raw status code. When something is wrong, the UI also shows which subsystem caused the problem:

- **downloader** — the issue happened during fetch or composition.
- **processing** — the issue happened during analysis or publication.

This tells you where to start debugging without reading logs.

## See also

- [Download Lifecycle](download-lifecycle.md) — how statuses are produced
- [Health Classes](health-classes.md) — how repeated status outcomes determine feed health
