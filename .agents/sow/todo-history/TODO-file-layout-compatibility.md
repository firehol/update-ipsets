# TODO — file layout and bash compatibility specification

## Purpose

Define the complete on-disk contract of the product:

- every file and directory the Go implementation maintains
- which subsystem owns each file family
- which files are public outputs, local state, transient staging, or integrity
  signals
- which legacy bash files and layouts remain compatible
- how migration/synchronization from the bash production tree is expected to
  work

The fit-for-purpose target is operational reproducibility:

- an operator should be able to understand the full filesystem model from the
  specs alone
- a reviewer should be able to verify whether the Go implementation preserves
  the intended bash compatibility surface
- future maintainers should know which files are normative product contracts and
  which are merely implementation conveniences

## TL;DR

user requested one more authoritative spec focused on:

1. full file layout for all maintained files
2. explicit backward compatibility with the legacy bash version
3. verification against the real Go implementation and the bash-era files
4. coverage of the `sync-from-d1.sh` / production import path
5. renaming that script to `sync-from-bash-version.sh`
6. supporting either:
   - a hostname to sync from over ssh/rsync/scp
   - `localhost` to migrate from the old local bash layout into the new Go
     layout

This work is documentation-first, but it must be evidence-backed by the current
code and legacy tree rather than inferred from earlier specs.

## Analysis

Facts known before the audit:

- `specs/compatibility.md` already exists, so the new work must build on it
  instead of creating a contradictory second compatibility contract.
- `docs/migration-from-bash.md` already exists, so migration procedure and
  normative compatibility may need a cleaner split.
- `pkg/engine/file_contract_test.go` exists, which likely already encodes part
  of the filesystem contract and must be used as evidence.
- `pkg/web/admin_manifest.go` exists, which likely enumerates expected files per
  feed and is useful for the maintained-files inventory.
- user explicitly called out the legacy production sync path
  `sync-from-d1.sh`, so that script and any equivalent current helper must be
  audited directly.

Facts confirmed by code and legacy audit:

- `pkg/engine/file_contract_test.go` already verifies important bash-compatible
  outputs:
  - `lib/{feed}/latest` is the canonical binary snapshot
  - `lib/{feed}/latest.set` must not be created anymore
  - `lib/{feed}/new/{unix_ts}` is the canonical retention cohort snapshot
  - `lib/{feed}/new/{unix_ts}.set` must not be created anymore
  - `lib/{feed}/history.csv`, `lib/{feed}/changesets.csv`,
    `lib/{feed}/histogram`, `web/{feed}_history.csv`,
    `web/{feed}_changesets.csv`, and `web/{feed}_retention.json` are all part
    of the maintained contract
- `pkg/web/admin_manifest.go` is a strong implementation inventory for
  maintained feed files. It enumerates:
  - committed feed bodies
  - committed outputs (`.ipset` / `.netset`)
  - `.setinfo`
  - public metadata/history/changesets/retention/comparison/insights files
  - per-provider geo/asn/bogons JSONs
  - `lib/{feed}/latest`
  - downloader-owned history rollups under `history/{parent}/*.set`
- `pkg/engine/finalize.go`, `retention.go`, `public_series.go`,
  `metadata.go`, and `output/sync.go` show additional maintained files not
  obvious from the manifest:
  - `lib/{feed}/retention.csv`
  - `lib/{feed}/retention.json`
  - `lib/{feed}/retention_cohorts.csv`
  - root-level `README.md`, `.gitignore`, `set_file_timestamps.sh` when
    BaseDir is a git repo
  - `web/index.json` and `web/all-ipsets.json`
- The current migration helper is still d1-specific:
  - script name: `sync-from-d1.sh`
  - staging dir: `import-d1`
  - backup naming: `pre-d1-sync-*`
  - engine helper `pkg/engine/legacy_failure_bootstrap.go` also still looks for
    `import-d1/merged-cache.json`
- The runtime/cache layout is split and must be documented accurately:
  - engine state cache: `BaseDir/.cache.json`
  - legacy bash cache import: `BaseDir/.cache`
  - scheduler runtime state: `CacheDir/scheduler-state.json`
- `install.sh` comments are stale in at least one important place:
  - it still describes `lib/.../latest.set`, while the implementation and tests
    require `lib/{feed}/latest`
- Legacy bash evidence in `/home/user/src/firehol/firehol/sbin/update-ipsets`
  confirms the historical filenames/directories we still care about:
  - `BASE_DIR/.cache`
  - `LIB_DIR/{feed}/latest`
  - `LIB_DIR/{feed}/new/{unix_ts}`
  - `LIB_DIR/{feed}/changesets.csv`
  - `LIB_DIR/{feed}/retention.csv`
  - `LIB_DIR/{feed}/histogram`
  - `RUN_DIR/all-ipsets.json`
  - `RUN_DIR/{feed}_history.csv`
  - `RUN_DIR/{feed}_changesets.csv`
  - `RUN_DIR/{feed}_comparison.json`
  - `BASE_DIR/{feed}.setinfo`
- Direct inspection of `d1` confirmed the live bash history layout:
  - `/etc/firehol/ipsets/history/{feed}/{unix_timestamp}.set`
  - this is per-update timestamped snapshot history
  - it is not the same as the Go downloader's daily rollups
    (`data/history/{parent}/{YYYY-MM-DD}.set`)
  - migration tooling therefore needs translation, not blind copy, if it is to
    preserve semantic continuity for history derivatives

## Decisions

Made by user:

1. Add another spec dedicated to file layout for all maintained files plus bash
   backward compatibility.
2. The work must include compatibility verification against the old bash files.
3. The work must account for `sync-from-d1.sh`, which copies production files
   from the bash version into the Go-managed directories.
4. `sync-from-d1.sh` should become `sync-from-bash-version.sh`.
5. The migration script should accept either a remote hostname or `localhost`
   as the bash-source origin.
6. A design decision is now open for history-derivative retained state:
   - keep the current Go daily-rollup model
   - or switch to the bash per-update snapshot model
6. Verify how the bash version actually stores daily rollup-like historical
   state, including direct inspection of available files on `d1`.
7. The specs MUST NOT hardcode user's personal home directory. Paths in specs
   must be:
   - relative to the installation/runtime root
   - or explicit deployment/legacy absolutes such as `/opt/update-ipsets`,
     `/etc/firehol`, `/var/lib/update-ipsets`, `/var/www/blocklists`

Implied execution decisions unless contradicted:

1. Prefer extending the current spec set rather than duplicating
   `specs/compatibility.md` with overlapping language.
2. The likely deliverable is:
   - one new normative spec focused on filesystem layout and ownership
   - updates to `specs/compatibility.md` and/or `docs/migration-from-bash.md`
     where that split is currently blurry
3. Any new or updated compatibility statements must be grounded in direct code
   and legacy-tree evidence.

Decision pending:

1. History-derivative retained state model
   Evidence:
   - bash keeps one binary snapshot per successful update and unions snapshots
     newer than the requested time cutoff:
     - `/home/user/src/firehol/firehol/sbin/update-ipsets:973-1015`
- live `d1` confirms the same shape:
  - `/etc/firehol/ipsets/history/{feed}/{unix_timestamp}.set`
- current Go keeps one downloader-owned rollup per UTC day and unions the
  parent body plus the last `N` daily buckets:
  - `pkg/engine/feed_body_stage.go:128-289`
  - `specs/pipeline.md:120-133`
   Options:
   - A. Keep Go daily rollups
     - Pros: bounded file count, cheap pruning, simple model, lower metadata
       churn for high-frequency feeds
     - Cons: loses intra-day update granularity; weaker parity with bash;
       migration needs translation from bash snapshots
   - B. Switch Go to bash-style per-update snapshots
     - Pros: exact bash parity, more expressive history model, no information
       loss within a day, migration becomes mostly direct copy for this family
     - Cons: much larger file counts for chatty feeds, more inode pressure,
       more expensive unions unless we add indexing/limits, more maintenance
       work in integrity/pruning/specs
   Decision made by user:
   - B. Switch Go to bash-style per-update snapshots.
   Implications accepted:
   - more files are acceptable
   - migration should preserve the historical shape instead of translating it
   - specs and implementation must stop describing this state as daily rollups

## Plan

1. Read the current compatibility/migration/spec docs to understand present
   ownership and gaps.
2. Audit the Go implementation for all maintained files and directories:
   - downloader
   - engine
   - web publication
   - integrity/admin manifest
3. Audit bash compatibility evidence:
   - legacy bash tree
   - sync/import scripts, especially `sync-from-d1.sh`
   - any tests already asserting compatibility
4. Decide the cleanest spec ownership split:
   - new filesystem-layout spec vs updates to compatibility/migration docs
5. Replace the d1-specific migration helper with a generic
   `sync-from-bash-version.sh` entry point and make its source selection
   explicit (`hostname` or `localhost`).
6. Inspect `d1` directly to verify which historical/bash files actually exist,
   especially any daily-rollup or equivalent retention/history evidence.
7. Replace the current Go daily-rollup implementation with bash-style
   per-update history snapshots across:
   - downloader staging/composition
   - integrity validation and recovery expectations
   - admin manifest/reporting
   - migration helper behavior
   - tests and wording
8. Write/update the spec set.
9. Add or extend verification tests/checks if the current repo lacks explicit
   compatibility coverage for the documented files.

## Implied decisions

- Do not guess file families from memory; derive them from implementation and
  tests.
- Distinguish clearly between:
  - normative product file contracts
  - best-effort migration/import helpers
  - transient implementation details that are not operator contracts
- Keep subsystem ownership explicit:
  - downloader-owned files
  - engine-owned files
  - website/publication-owned files
- The renamed migration helper should be explicit about source selection and
  should no longer encode a hard-wired `d1` framing in its public name.
- The new file-layout spec must reflect the real historical bash storage model,
  not just what the current Go implementation does.
- Personal workstation-specific paths do not belong in the normative spec set.
- The previous plan to translate bash history snapshots into Go-native daily
  rollups is now invalid and must be removed from both the specs and the
  migration helper.

## Testing requirements

- Re-run any file-contract and compatibility-focused tests affected by the
  documentation or verification work.
- Exercise the migration helper in at least argument/behavior validation mode
  for both remote-host and `localhost` source selection.
- If new compatibility checks are added, they must be narrow, deterministic, and
  based on real existing artifact names/layouts.

## Documentation updates required

- likely new spec file under `specs/`
- likely updates to:
  - `specs/compatibility.md`
  - `docs/migration-from-bash.md`
  - `specs/README.md`
  - `AGENTS.md` documentation map if a new spec file is added
- migration helper documentation/name references anywhere that still mention
  `sync-from-d1.sh`
