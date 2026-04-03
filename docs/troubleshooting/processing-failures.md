# Processing Failures

You will learn what each processing exception means and how to recover from it.

## Exception classes

When the processing engine fails, it reports a structured exception class. These are severe runtime faults — they indicate something went wrong inside the engine, not just a transient download problem.

### parse_failed

The local feed body exists but cannot be parsed into a valid IP set.

**What it means:** The file on disk is corrupt, truncated, or contains content the parser cannot handle.

**What to do:** Trigger a recheck to download fresh input:

```bash
curl -X POST -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds/<name>/recheck
```

### finalize_failed

Downstream publication or promotion of the processing output failed.

**What it means:** The feed was processed successfully, but writing the results to disk failed. This often indicates disk space issues or permission problems.

**What to do:** Check disk space and permissions. Then reprocess:

```bash
curl -X POST -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds/<name>/reprocess
```

### integrity_failed

An integrity check after processing found that the outputs do not match what was expected.

**What it means:** Something went wrong during publication — a file was not written, or a secondary artifact is missing or malformed.

**What to do:** Use the integrity recovery action from the admin UI, or reprocess the feed.

### missing_input

No staged or committed local feed body exists for processing.

**What it means:** The feed has never been downloaded, or its local data was lost.

**What to do:** Trigger a recheck to fetch fresh data:

```bash
curl -X POST -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds/<name>/recheck
```

### cancelled

Processing was cancelled because the run context was closed — usually due to daemon shutdown.

**What it means:** The feed was in the processing queue when the daemon shut down or restarted.

**What to do:** No action needed. The daemon recovers staged input on restart and re-queues the feed.

### config_error

Configuration for this feed is invalid or inconsistent.

**What it means:** A config field references a missing provider, an invalid URL, or a broken dependency chain.

**What to do:** Check the feed's configuration in the YAML catalog. Fix the error and reload or restart the daemon.

### extract_failed

Source material was obtained but the extraction or normalization pipeline failed.

**What it means:** The download succeeded, but converting the raw content into a canonical IP set failed. This could be a malformed archive, an unexpected format change, or a processor bug.

**What to do:** Check the logs for the specific extraction error. If the upstream format changed, the processor pipeline may need updating. Trigger a recheck after the fix.

### open_failed

The engine could not open a required file — typically a provider dataset or a committed feed body.

**What it means:** A file that should exist on disk is missing or unreadable.

**What to do:** Check file permissions and disk health. For provider datasets, verify the provider is enabled and has been downloaded. Reprocess after the fix.

### unavailable

A supporting resource (provider dataset, artifact parent) is not available.

**What it means:** The feed depends on a provider or parent that has not been downloaded or is disabled.

**What to do:** Enable and recheck the missing dependency. Then reprocess the affected feed.

### stale

The input or supporting data is too old to be useful.

**What it means:** The feed body or a provider dataset has exceeded its useful lifetime.

**What to do:** Recheck the feed to get fresh input.

## Recovery actions

Most processing failures resolve with one of two actions:

| Action | When to use |
|--------|-------------|
| **Recheck** | The local input is corrupt, missing, or stale — you need fresh upstream data |
| **Reprocess** | The local input is valid but outputs are missing or corrupt — rebuild from existing data |

### Recheck (fetch fresh data)

```bash
curl -X POST -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds/<name>/recheck
```

### Reprocess (rebuild from local data)

```bash
curl -X POST -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds/<name>/reprocess
```

## When to investigate further

If the same feed fails repeatedly after recheck and reprocess, check the logs:

```bash
journalctl -u update-ipsets | grep '"feed":"<name>"' | grep '"level":"error"'
```

Recurring `parse_failed` or `finalize_failed` exceptions suggest a bug or a serious consistency problem. Report the issue with the feed name, error message, and relevant log excerpts.
