# Triggers and Reprocessing

You will learn what causes a feed to be downloaded, processed, or reprocessed, and what does not.

## Content change

The most common trigger. The downloader fetches upstream, normalizes the content, and finds the canonical feed body differs from the current local version.

Result: new `.new` file staged, processing queued.

## Provider change

When a supporting database updates (ASN, GeoIP, bogon, critical infrastructure), the daemon reprocesses all feeds that depend on that provider.

This reprocessing uses existing local feed bodies. The feeds themselves are not re-downloaded from upstream. The engine re-runs enrichment, comparison, and insights with the new provider data.

Result: broad reprocess wave for affected feeds.

## Admin action

Operators can trigger work manually:

- **Recheck** — fetch now, process even if unchanged.
- **Reprocess** — skip fetch, re-run processing from existing local data.

See [Operator Actions](../admin-ui/operator-actions.md) for when to use each.

## Integrity recovery

The daemon periodically checks published artifacts against expected state. If it finds missing or stale outputs, it queues repair work.

Repair uses existing local feed bodies when available. If the local body itself is missing, the daemon targets the parent feed or artifact for recheck.

Result: targeted reprocess or recheck depending on what is missing.

## Config reload

When the daemon reloads its configuration:

- Feed definitions that changed trigger reprocessing.
- Provider definitions that changed trigger provider-update reprocessing.
- New feeds are treated as never-checked and become due immediately.

Invalid reloads leave the previous configuration in place. No reprocessing happens from a failed reload.

## Critical infrastructure provider-set drift

When the configured critical-infrastructure reference set changes, the
downloader forces a refresh of all `critical_infrastructure` sources. This
includes additions, removals, and stable source-configuration changes that alter
the provider-set identity.

The provider-set identity deliberately excludes materialized feed content, entry
counts, and unique-IP counts. Ordinary content updates flow through the normal
download and processing paths.

## What is NOT a trigger

- **Public page view** — viewing a feed page, comparison page, or any public surface does not trigger downloads, reprocessing, or recomputation. All public pages serve precomputed artifacts.
- **Admin page view** — loading the admin UI does not trigger work. Admin API endpoints that read state are passive.
- **Scheduler tick with nothing due** — if no feeds are due for download and no processing is queued, the scheduler does nothing.

## See also

- [Operator Actions](../admin-ui/operator-actions.md) — manual triggers
- [Pipeline Overview](pipeline-overview.md) — how triggers flow through the pipeline
