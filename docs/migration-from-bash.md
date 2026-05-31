# Migration from bash update-ipsets to Go update-ipsets

## Status

This document is procedural guidance.

It explains how an operator should migrate from the historical FireHOL bash
pipeline to the Go implementation while preserving state and minimizing
surprises. This page is procedural; use the API, feed, and filesystem docs for
the operator-visible compatibility surfaces.

## Purpose

The migration goal is to move the installation to the Go daemon without losing:

- catalog identity
- useful historical local state
- public/mirror consumers that still rely on compatibility artifacts

## Migration strategy

Prefer staged cut-over, not blind replacement.

Recommended high-level approach:

1. inventory what the bash deployment currently publishes and what consumes it
2. preserve the old state and cache
3. start the Go daemon against equivalent configuration and directories where
   required
4. validate feed count, public outputs, and admin/runtime behavior
5. switch traffic and downstream consumers only after verification

## Canonical migration helper

The canonical helper for importing a bash-era installation into the Go layout
is:

- `./scripts/sync-from-bash-version.sh <source-host|localhost> [INSTALL_DIR]`

Meaning:

- use `localhost` when the old bash layout is on the same machine
- use a hostname when importing over ssh/rsync from another machine

The older `scripts/sync-from-d1.sh` name may still exist as a compatibility wrapper
while operators transition, but the canonical documented interface is
`scripts/sync-from-bash-version.sh`.

## What to inventory first

Before migration, identify:

- the current bash catalog source
- whether downstreams consume:
  - raw feed outputs
  - history CSVs
  - changeset CSVs
  - feed metadata JSON
  - website/mirror files
- whether the old `.cache` file contains state you want to preserve
- whether the deployment uses legacy FireHOL directory conventions
- whether any custom automation expects specific filenames or paths
- whether the old installation uses `/etc/firehol/ipsets/history/{feed}/*.set`
  so those retained history snapshots can be copied into the Go installation's
  canonical `data/history/{feed}/{unix_timestamp}.set` layout

## Recommended migration steps

### 1. Back up the bash-era state

Preserve at least:

- the old configuration inputs
- the old `.cache`
- current published/public output directories
- any legacy directories still consumed by downstream tools

### 2. Decide the cut-over model

Choose one:

- **side-by-side migration**
  - safest
  - lets you validate the Go product before replacing the bash service
- **in-place migration**
  - simpler operationally
  - higher risk if hidden downstream dependencies exist

If there is any uncertainty about existing consumers, prefer side-by-side.

### 3. Establish the Go catalog/configuration

Use the canonical Go-era configuration model.

If migration depends on legacy catalog extraction, treat that as a migration
aid, not as the long-term source of truth.

### 4. Import or preserve legacy state where useful

If historical state continuity matters:

- use the canonical helper so it can import the legacy cache and preserve local-only Go cache entries
- preserve legacy committed outputs long enough to compare results

The helper uses the `cache-merge` subcommand internally when it finds a legacy `.cache` file. Operators normally run `scripts/sync-from-bash-version.sh` instead of invoking `cache-merge` directly.

### 5. Start the Go daemon

Bring up the Go daemon and confirm:

- service health
- downloader activity
- processing activity
- public artifact generation
- admin visibility

### 6. Validate compatibility outputs

Check the outputs that real consumers depend on.

At minimum validate:

- raw downloadable sets
- feed metadata artifacts
- history and changeset artifacts
- website/API consumers of public data
- any migrated bash history/retention evidence that was translated into the
  Go-native layout

### 7. Compare a representative feed set

Pick representative feeds across families:

- direct upstream feed
- history derivative
- merge
- artifact-backed feed
- provider-enriched feed page

Confirm they produce the expected public outcomes.

### 8. Cut over traffic and consumers

Only after validation:

- switch website/public routing if needed
- switch mirror/downstream consumers
- disable the bash updater

## Validation checklist

The migration should not be considered complete until you have confirmed:

- the expected catalog size is present
- expected feed names and categories are present
- downloader and processing queues behave correctly
- public feed pages render from current published artifacts
- pairwise comparisons are current
- provider enrichments are present where expected
- compatibility artifacts needed by legacy consumers still exist

## Common mistakes to avoid

- assuming no one consumes history or changeset artifacts
- migrating configuration but not state
- letting the public site depend on live runtime recomputation during cut-over
- removing bash-era compatibility files before confirming their consumers are
  gone
- treating side-by-side output differences as bugs before checking whether they
  are specification changes

## Rollback principle

Rollback is simplest when:

- the bash-era state was preserved
- the Go deployment was first validated side-by-side
- the cut-over point was explicit

If rollback may be needed, preserve the pre-cut-over published output tree and
legacy cache until the Go deployment is considered stable.
