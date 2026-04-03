# TODO-ARTIFACT-SCHEDULER — Fix artifact retry loop and per-artifact size limits

> **Purpose**: Artifact-backed feeds must behave like normal downloader inputs from the scheduler's point of view: bounded retries, truthful operator state, and no hot-loop retry storms. DroneBL specifically must be able to complete within an explicit artifact-owned size cap instead of being forced through the generic source download limit.

## TL;DR

Costa reported that `dronebl` keeps downloading again immediately after it finishes.

Verified root cause from the live service:

- `dronebl` is not succeeding; it is failing with `local file exceeds max download size (104857600 bytes)`.
- The scheduler is also buggy for artifact parents: it does not see the artifact's updated cache entry, so it keeps treating `dronebl` as `never checked` and immediately requeues it.
- The correct fix is twofold:
  - include artifact-parent entries in the scheduler's state view so artifact retry/backoff works correctly
  - add an explicit per-artifact max download size override so DroneBL can use a larger cap than ordinary sources

## Analysis

### Live evidence

- Journal spam from the running service shows repeated failures for `dronebl`:
  - `download loop failed` with `local file exceeds max download size (104857600 bytes)`
- Live files on disk confirm the current DroneBL buildzone exceeds the generic cap:
  - `/opt/update-ipsets/lib/artifacts/dronebl/fetch/buildzone` = `131635013` bytes
- Live admin artifact state is inconsistent with the journal:
  - `/api/v1/admin/artifacts` for `dronebl` shows:
    - `last_check: 0`
    - `download_failures: 0`
    - `scheduler_detail: "never checked"`
  - this proves the scheduler's artifact view is not seeing the artifact-parent cache entry updates

### Verified code path: size failure

- DroneBL artifact download path:
  - `pkg/engine/artifact_stage.go`
  - `fetchAndStageDroneBLArtifact()` sets `entry.CheckedDate` and then calls the downloader with `MaxDownloadSize: e.runtime.MaxDownloadSize`
- Generic local-copy size guard:
  - `pkg/downloader/downloader.go`
  - local file fetch fails when the copied file is larger than `MaxDownloadSize`
- Current config:
  - `configs/firehol.yaml`
  - `artifacts.dronebl.frequency: 1`
  - there is no artifact-specific size override today

### Verified code path: scheduler bug

- Scheduler builds artifact due-items from `EntriesSnapshot()`:
  - `pkg/scheduler/scheduler.go`
  - `runFetchLoop()` -> `BuildArtifactItems(..., r.eng.EntriesSnapshot(), ...)`
- `BuildArtifactItems()` indexes artifact state by name from the passed entries slice:
  - `pkg/scheduler/scheduler.go`
- `EntriesSnapshot()` filters names through `configuredNames()`:
  - `pkg/engine/query.go`
- `configuredNames()` currently includes only sources and merges, not artifacts:
  - `pkg/engine/public.go`
- Result:
  - artifact parents like `dronebl` are absent from `EntriesSnapshot()`
  - `BuildArtifactItems()` sees a zero-value cache entry for `dronebl`
  - `nextDue()` sees `CheckedDate == 0` and returns `never checked`
  - the artifact parent is considered due again immediately, regardless of the actual artifact failure state/backoff

## Decisions

### Costa decisions already made

- Fix the scheduler bug.
- Raise the limit for DroneBL.
- Add per-artifact cap size support if needed.
- 2026-04-24: change the DroneBL artifact fetch cadence from 1 minute to 60
  minutes.

### Implied implementation decisions for this pass

- The artifact size override should be configuration-owned, not hardcoded in code.
- The artifact override should fall back to the existing runtime/global max when unset.
- Artifact-parent cache state must remain visible to artifact scheduling/admin status, but must not leak into public feed inventory.
- Existing source behavior must remain unchanged.

## Plan

1. Extend the artifact config/runtime contract with an optional per-artifact max download size override.
2. Wire artifact fetch paths to use the artifact override when present, otherwise the runtime default.
3. Fix the scheduler/state path so artifact parents are included in the entries snapshot used for artifact scheduling and admin artifact status.
4. Add regression tests for:
   - artifact parents are visible to artifact scheduling state
   - artifact retry/backoff respects recorded `CheckedDate` / `DownloadFailures`
   - per-artifact max size override is honored
5. Update the relevant specs so artifact scheduling state and artifact-specific download caps are documented.
6. Install and verify live:
   - service healthy
   - `dronebl` no longer shows `never checked` after a failure
   - DroneBL uses the raised cap

2026-04-24 extension — config single-source-of-truth:

- Costa pointed out the deployment risk correctly: after adding the
  per-artifact cap to both repo config and live `/opt/.../config.yaml`, the
  product still had a two-place edit model.
- Verified installer behavior:
  - `install.sh` only writes `etc/config.yaml` on first install
  - on reinstall it preserves the old active file and only refreshes
    `etc/config.yaml.default`
  - this is the exact source of repo/runtime catalog drift
- Verified filesystem spec:
  - `specs/files-layout.md` currently defines `etc/config.yaml` as the
    canonical runtime config and operator-owned
- Practical fix direction for this pass:
  - make install flow update the active installed config from the repo catalog
    every time
  - create a timestamped backup before overwrite when the existing file differs
  - remove the false comfort of a refreshed `.default` copy while the daemon
    keeps using stale config
  - document that repo config is the deploy source of truth, while
    environment-specific runtime changes belong in systemd drop-ins / overlay
    config paths rather than in-place edits to the installed catalog file

## Implied decisions

- The scheduler bug should be fixed in the shared engine snapshot path rather than by special-casing DroneBL in scheduler code.
- The new config field should live under `artifacts.<name>` because the cap belongs to the artifact parent, not to every child feed.
- Documentation must explain that artifacts can carry their own download-size cap when their raw upstream blob is materially larger than ordinary feed bodies.

## Testing requirements

- `go test ./pkg/config`
- `go test ./pkg/engine`
- `go test ./pkg/scheduler`
- `go test ./pkg/web`
- `git diff --check`
- live install and health verification

## Verification notes

### 2026-04-24 live verification

- Repo config and installed active config now agree on the DroneBL cap:
  - `configs/firehol.yaml` -> `artifacts.dronebl.max_download_size = 268435456`
  - `/opt/update-ipsets/etc/config.yaml` -> `artifacts.dronebl.max_download_size = 268435456`
- Live service after reinstall:
  - `systemctl is-active update-ipsets` -> `active`
  - `curl http://localhost:18888/healthz` -> `ok`
- Live admin artifact state is now truthful for `dronebl`:
  - `/api/v1/admin/artifacts` shows:
    - non-zero `last_check`
    - non-zero `last_update`
    - `download_failures = 0`
    - scheduler detail like `next check in 0 mins (base 1 mins)`
  - the previous false `never checked` state is gone
- Live journal after the fix no longer shows the old size-cap error:
  - no new `local file exceeds max download size (104857600 bytes)` after reinstall
  - DroneBL child feeds are being materialized successfully:
    - `dronebl_dictionary_attacks`
    - `dronebl_worms_bots`
    - `dronebl_irc_drones`
    - `dronebl_auto_botnets`
    - `dronebl_anonymizers`
- The only DroneBL failure seen right before the successful run was
  `rsync DroneBL buildzone: signal: killed`, which aligns with the service
  restart window, not with the previous size-cap bug.

### 2026-04-24 installer/config verification

- `install.sh` now deploys `configs/firehol.yaml` directly to
  `/opt/update-ipsets/etc/config.yaml` as the active installed catalog.
- On reinstall, if the active file differs, install now creates a timestamped
  backup before overwrite instead of silently preserving stale runtime config
  and only refreshing `config.yaml.default`.
- This removes the two-place-edit operational trap Costa called out.

## Documentation updates required

- `specs/config.md`
- `specs/downloader.md`
- `specs/feeds.md` and/or `specs/files-layout.md` only if artifact-parent state visibility contract needs clarification
- `specs/operating-principles.md` if the bounded-work / retry semantics wording needs to mention artifact-parent backoff explicitly
