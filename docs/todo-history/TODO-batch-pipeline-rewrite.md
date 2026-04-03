# TODO: Batch-pipeline rewrite — decouple fetch from processing

## Purpose

Implement a feed pipeline that keeps strict separation of concerns:

- the downloader owns acquisition and full feed composition for every feed kind
- the processing engine owns only downstream analysis/publication work over an
  already prepared feed body
- operator behavior must stay predictable:
  - only downloader-stage outcomes decide whether a feed enters the processing
    queue
  - once queued, processing is mandatory and complete
  - reprocessing must refresh derived outputs even when the feed body itself is
    unchanged

## TL;DR

Split the runtime into two independent loops:

- **Step 1: fetch/change-detection loop**
  - runs continuously and independently
  - downloads source bodies to disk with up to **X workers** in parallel
  - default `X = 5`, configurable
  - decides only one thing: did this feed's source body actually change?
- **Step 2: processing loop**
  - runs every **Y minutes**
  - default `Y = 10`, configurable
  - processes whatever feeds are already queued as "changed"
  - must not wait for slow HTTP downloads, timeouts, or big remote responses

The key purpose is operational isolation:

- slow or stuck downloads must not delay parsing/finalize/retention/comparison/insights
- "no-change" feeds should be cheap and handled in the fetch loop only
- downloads must remain **disk-backed**, not RAM-backed

## Status — 2026-04-19

Implemented:

- Runtime split into two loops in `pkg/scheduler/scheduler.go`:
  - fetch loop
  - processing loop
- New runtime defaults:
  - `runtime.parallel_downloads = 5`
  - `runtime.processing_interval_minutes = 10`
- Download staging contract for downloadable artifacts:
  - write `{file}.tmp`
  - rename to `{file}.new`
  - process from `.new`
  - promote `.new` to final only after the processing batch succeeds
- Startup recovery:
  - lingering `.tmp` is discarded
  - lingering `.new` is re-queued for processing
- Processing batches now run with `SkipPrefetch=true`
  - the processing loop drains a stable queue snapshot
  - automatic downloads wait for the next processing tick
  - manual actions can force immediate processing
- Batch ordering implemented:
  1. normal feeds
  2. history derivatives
  3. merges, ordered by input count
- Manual actions implemented:
  - `recheck` = fetch now, queue processing even if downloader says same/not-modified
  - `reprocess` = skip fetch, queue processing from local committed/staged source
- Admin top panel rewritten to show only:
  - waiting to be downloaded
  - being downloaded now
  - waiting to be processed
  - being processed now
- Shared artifact-parent model implemented for bundle-style downloads:
  - new top-level `artifacts:` config section
  - child feeds use `artifact://<artifact>?parts=a,b,c`
  - artifact parents have their own enable/disable state
  - artifact-backed child feeds are not downloadable themselves
  - artifact refresh materializes child staged `.source.new` files and queues
    only the children that actually changed
- DroneBL moved into that artifact-parent model:
  - one `dronebl` artifact parent
  - all DroneBL child feeds now consume `artifact://dronebl?parts=...`
  - the old `dronebl_buildzone_class` / `dronebl_lists` source contract is gone
- Admin now exposes artifact parents separately from the feeds table:
  - `/api/v1/admin/status` includes `artifacts`
  - `/api/v1/admin/artifacts` and `/api/v1/admin/artifacts/{name}`
  - enable / disable / recheck actions for artifacts
- Local file sources now support operator-facing `file://` URLs
  - only absolute local paths are accepted
  - file URLs with a host component are rejected

Verified:

- `go test ./...`
- `npm --prefix ui run build`

## Status — 2026-04-21

Migration work completed against the current `specs/*.md` contract:

- downloader-stage ownership now covers:
  - plain feeds
  - history derivatives
  - merges
  - provider databases
  - artifact parents and artifact-backed children
- processing-stage execution now consumes only staged/committed feed bodies
  and no longer performs feed-family synthesis or upstream fetching
- legacy in-engine merge/retention provider code paths were removed
- scheduler/admin semantics were aligned with the spec:
  - provider databases follow normal enable/disable semantics
  - admin runtime kinds expose only the spec taxonomy
  - manifest source files reflect committed feed bodies / provider archives

Verification:

- `go test ./...`

## Remaining gaps to close — 2026-04-21T08:21:29Z

Facts verified against the current implementation:

1. History-derivative rollup integrity recovery is still incomplete.
   - `pkg/engine/integrity.go` currently checks public outputs and merge
     blocked inputs, but it does not detect missing/corrupt downloader-owned
     daily rollups for history derivatives.
   - `pkg/engine/integrity_recovery.go` already knows how to route recovery to
     a parent `recheck`, but that path is never reached for rollup loss because
     no integrity finding is emitted.

2. Scheduler ownership is still expressed through one combined `Runner`.
   - The downloader and processing loops are behaviorally split, but one type
     still owns both queue maps and both loops in `pkg/scheduler/scheduler.go`.
   - The remaining work is structural: make downloader/processing ownership
     explicit inside the scheduler package without changing operator-visible
     semantics.

3. Legacy `rebuild` compatibility still leaks through runtime/UI helpers.
   - `pkg/runreason/runreason.go` still accepts legacy rebuild reasons.
   - `ui/src/lib/admin-run-reason.ts` still maps legacy rebuild strings.
   - `pkg/engine/download_stage.go` still accepts legacy string statuses during
     staged-download promotion.

Plan:

1. Extend integrity to detect history-derivative rollup loss/corruption and
   route recovery to parent `recheck`.
2. Refactor scheduler internals so downloader/processing ownership is explicit
   and no longer expressed as one combined queue owner.
3. Remove legacy rebuild compatibility from runtime/UI paths that now have a
   stable `reprocess` contract.
4. Re-run `go test ./...` and re-audit the TODO/spec gaps after the code
   changes.

Closure status — 2026-04-21:

- Closed.
- Implemented in:
  - `pkg/engine/integrity.go`
  - `pkg/engine/integrity_recovery.go`
  - `pkg/engine/integrity_test.go`
  - `pkg/scheduler/scheduler.go`
  - `pkg/scheduler/scheduler_test.go`
  - `pkg/runreason/runreason.go`
  - `ui/src/lib/admin-run-reason.ts`
  - `pkg/engine/download_stage.go`
- Verification:
  - `go test ./...` passes
  - repo grep confirms no live `*_rebuild` reason handling remains outside
    historical/generated files
  - repo grep confirms no remaining stale flat scheduler field usage remains in
    live code

## Newly confirmed discrepancies — 2026-04-21

### 1. Failed staged downloads are re-entering the processing queue

Observed with:

- `firehol_anonymous`
- `firehol_level4`

Evidence:

- `pkg/scheduler/scheduler.go` requeues any feed that still has a staged
  download after a batch:
  - `requeueFailedStagedDownloads()` at lines around `756+`
- successful prepared-body staging currently records:
  - `entry.LastStatus = "downloaded"` in
    `pkg/engine/download_stage.go`
- promotion only accepts:
  - `"updated"`, `"same"`, `"empty"`, `"not_updated"`, `"not_modified"`
  - via `downloadPromotionStatusOK()`
- so a successful downloader-stage merge/plain staged body may keep its
  `.source.new` file after a successful processing batch
- the lingering `.new` then causes the scheduler to put the feed back into
  `waiting to be processed`

Impact:

- operator-facing queue semantics are wrong
- downloader-stage failures and successful staged downloads are conflated
- the same feed can appear to "finish" and then immediately reappear in
  `waiting to be processed`

### 2. Merge downloader failures are not surfaced as downloader-queue failures

Evidence:

- merge composition is downloader-stage work in
  `pkg/engine/download_stage.go`
- merge input-missing failures are recorded as:
  - `LastStatus = "download_failed"`
  - `LastError = <exact compose error>`
- but scheduler queue payloads only expose:
  - `name`
  - `reason`
  - `queued_at` / `started_at`
- see `pkg/scheduler/queue_state.go`
- the admin queue panel therefore cannot show the live downloader error

Impact:

- operators see a feed in downloader queues but cannot see the exact reason it
  is failing
- the state column later falls back to the latest settled cache state, which
  may already have overwritten the original downloader failure

### 3. Feed row status is built only from settled cache state, not live queue state

Evidence:

- the feeds table shows `last_error` if present, otherwise `last_status`
- see `ui/src/components/admin/feeds-table.tsx`
- downloader failures are cleared by later successful / `same` results in
  `pkg/engine/download_stage.go`
- admin feed status itself is derived without download queue membership in
  `pkg/web/admin.go`

Impact:

- the table can show `same` / `downloaded` / `ok` while the operator just saw a
  downloader-stage failure in the live pipeline

### 4. Integrity does not validate blocked synthetic composition inputs

Evidence:

- current integrity only checks:
  - committed primary outputs
  - public secondaries
  - structured secondary readability
- see `pkg/engine/integrity.go`
- it does not verify whether a merge's required committed feed bodies exist
- so `firehol_anonymous` can be operationally blocked by missing `anonymous`
  input without integrity reporting that blockage

Impact:

- integrity is silent on a real local inconsistency that prevents downloader
  composition

## Verified gap analysis — 2026-04-21T09:20:00Z

Purpose fit:

- close the remaining downloader/processing gaps so the implementation matches
  the normative `specs/*.md` contract
- fix correctness/spec violations before cleanup or micro-optimizations
- preserve the two-loop model:
  - downloader loop admits work
  - processing loop only consumes admitted work plus explicit admin/integrity
    reprocess work

Facts verified against code and specs:

1. Reprocess/provider-wave work can be silently dropped while a feed is already
   in `processing.active`.
   - code:
     - `pkg/scheduler/scheduler.go:567-580`
     - `pkg/scheduler/scheduler.go:879-890`
   - spec:
     - `specs/pipeline.md:519-522`

2. Parent-triggered history-derivative recomposition only runs for plain-feed
   parents.
   - code:
     - `pkg/engine/download_stage.go:262`
     - `pkg/engine/download_stage.go:284`
     - `pkg/engine/download_stage.go:606-627`
   - missing from:
     - artifact-child path
     - provider path
     - merge path

3. Same-content and merge composition paths still read whole committed bodies
   into heap, violating the memory contract.
   - code:
     - `pkg/engine/feed_body_stage.go:77-85`
     - `pkg/engine/download_stage.go:235-246`
     - `pkg/engine/feed_body_stage.go:244-275`
   - spec:
     - `specs/memory-management.md:20-24`
     - `specs/memory-management.md:36-50`

4. Parent `StatusSame` still rewrites daily rollups and recomposes history
   derivatives even when nothing changed inside the current UTC-day bucket.
   - code:
     - `pkg/engine/download_stage.go:235-263`
     - `pkg/engine/feed_body_stage.go:88-111`
   - impact:
     - avoidable downloader work and queue churn

5. Manual `recheck` on an artifact-backed child with missing local materialized
   input fails locally instead of redirecting recovery to the parent artifact.
   - code:
     - `pkg/engine/download_stage.go:362-370`
   - spec:
     - `specs/pipeline.md:542-553`
     - `specs/pipeline.md:783-788`

6. Manual `reprocess` is accepted even when neither staged nor committed local
   feed body exists, and then fails inside the processing engine.
   - code:
     - `pkg/scheduler/scheduler.go:395-413`
     - `pkg/engine/process.go:79-87`
   - spec:
     - `specs/pipeline.md:790-800`

7. `recheck` with no explicit names force-runs every downloadable source.
   - code:
     - `pkg/scheduler/scheduler.go:375-394`
   - spec tension:
     - `recheck` is a feed-level action
     - batch-level action is `run due work now`

8. Processing family ordering is implemented twice.
   - code:
     - `pkg/scheduler/scheduler.go:824-850`
     - `pkg/engine/run.go:310-349`

9. Scheduler snapshot is persisted on every fetch-loop iteration even when
   unchanged.
   - code:
     - `pkg/scheduler/scheduler.go:323-331`
     - `pkg/scheduler/scheduler.go:1199-1206`

10. Startup artifact recovery runs synchronously before the long-lived loops
    start.
    - code:
      - `pkg/scheduler/scheduler.go:280-311`
      - `pkg/engine/download_stage.go:108-128`
    - note:
      - current artifacts are small enough that this is primarily a fragility
        risk, not the highest-priority contract break

11. `BuildSnapshot()` is recomputed on every fetch-loop tick and every
    `Snapshot()` admin read.
    - code:
      - `pkg/scheduler/scheduler.go:184-199`
      - `pkg/scheduler/scheduler.go:323-331`
    - correction:
      - `ActivitySnapshot()` does not rebuild `BuildSnapshot()`
      - so the original review overstated this point

Decisions made:

- fix all verified gaps, not only the hard P0/P1 items
- preserve the current specs unless implementation work exposes another spec
  ambiguity
- for bulk operator behavior:
  - keep `run due work now` as the broad batch action
  - constrain `recheck` to explicit feed targets only
- for body selection:
  - staged local feed body always wins over committed local feed body
  - if neither exists, `reprocess` must be rejected before entering the engine

Implementation plan:

1. Add processing-side deferred reprocess bookkeeping so provider waves and
   manual reprocess requests are not dropped while a feed is already active.
2. Redirect invalid artifact-child rechecks and invalid bodyless reprocesses at
   scheduler/admin entry points instead of letting them become engine errors.
3. Make history-derivative parent-trigger expansion work for every legal parent
   family and avoid pointless same-day recomposition churn.
4. Replace heap-buffered same/merge body handling with streaming or file-backed
   comparisons/composition paths that satisfy the memory contract.
5. Remove global no-name `recheck` behavior and keep broad work under `run due`.
6. De-duplicate ordering logic and reduce unnecessary snapshot persistence/
   recomputation where safe.
7. Re-run tests, install, and verify the live queues/admin behavior.

Testing requirements:

- `go test ./...`
- focused scheduler tests for:
  - deferred reprocess while active
  - bodyless reprocess rejection
  - bulk recheck scope
- focused downloader/engine tests for:
  - artifact-child recheck fallback
  - history-derivative trigger coverage
  - same-day rollup no-op behavior
  - memory-safe comparison/composition helpers

Documentation updates required:

- update `specs/admin-ui.md` if bulk `recheck all` surface is removed or changed
- update `specs/pipeline.md` only if implementation uncovers a contract nuance
  not already expressed

## Implementation pass — 2026-04-21T10:25:00Z

Implemented in this pass:

1. Processing-stage deferred requeue for active feeds
   - second provider/manual reprocess requests are no longer dropped when a
     feed is already in `processing.active`
   - deferred work is released back to `waiting to be processed` after the
     active batch item finishes

2. Explicit recheck / reprocess validation
   - unnamed manual `recheck` is now rejected instead of force-running every
     downloadable source
   - `reprocess` now requires existing staged or committed local state
   - feed-level artifact-child `recheck` now redirects to the parent artifact
     when the child has no local materialized input

3. Downloader-side history-derivative trigger coverage
   - parent-triggered history-derivative recomposition now runs for additional
     feed-body-producing parent families, not only plain feeds
   - same-day identical rollup updates now short-circuit without rewriting the
     rollup or recomposing dependents

4. Memory-contract fixes
   - committed-vs-staged feed-body equality is now streaming/file-based instead
     of `os.ReadFile` on the committed body
   - plain-source `StatusSame` now parses the committed feed body from file
   - merge composition now streams parent feed bodies from open files instead
     of reading every enabled input fully into heap

5. Scheduler/admin cleanup
   - duplicate scheduler-side family ordering was removed; the engine remains
     the single authority for processing order
   - snapshot persistence now skips unchanged item sets
   - admin status/feed builders reuse a single scheduler snapshot per request
   - staged-recovery work now runs asynchronously inside the scheduler runtime
     instead of before the loops start

6. Config/spec tightening
   - history windows are now rejected for sources that do not produce feed
     bodies, which closes the invalid provider-database parent case
   - specs updated:
     - `specs/config.md`
     - `specs/feeds.md`

Verification:

- `go test ./...`
- new/updated tests cover:
  - artifact-child recheck redirect
  - unnamed recheck suppression
  - bodyless reprocess suppression
  - deferred processing requeue after active work
  - same-day daily-rollup no-op detection
  - artifact-child history-derivative propagation
  - invalid history windows on non-feed-body providers
- operators get no settled signal for "merge cannot be composed because one of
  its local required inputs does not exist"

### 5. Geolocation-derived `anonymous` / `satellite` remain reserved generated names, not first-class synthetic feeds

Evidence:

- config validation reserves:
  - `anonymous`
  - `satellite`
  - via `pkg/config/validate.go`
- but they are not real configured `Source` entries in `cfg.Sources`
- merges can still reference them because the validator pretends they exist
- by contrast, `rfc_reserved` is modeled as a hidden synthetic source and is
  visible in admin

## Decisions recorded — 2026-04-21 (explicit workflow contract)

Costa clarified the intended runtime contract and these decisions are now
binding for the spec update and the remaining implementation work:

### 1. Downloader admission is exclusive

- the only automatic path into `waiting to be processed` is:
  - `waiting to be downloaded`
  - `being downloaded now`
  - downloader decides the feed body is admitted
  - `waiting to be processed`
  - `being processed now`
- automatic runtime code MUST NOT enqueue feeds directly into
  `waiting to be processed`
- the downloader is the sole automatic owner of processing admission

### 2. Direct processing entry is admin-only

- direct enqueue into `waiting to be processed` is allowed only for explicit
  admin actions
- admin may request:
  - reprocess one feed
  - reprocess all feeds
- all non-admin direct-processing paths in runtime code must be removed

### 3. Downloader and processing are independent loops with independent state

- there are two distinct loops:
  - downloader loop
  - processing loop
- they are independent in behavior and in operator-facing state
- the runtime must not blur them into one combined scheduler abstraction that
  muddles queue ownership

### 4. Processing engine must not encode feed-family exceptions

- the processing engine consumes already prepared feed bodies only
- it must not contain hardcoded feed-family special cases to "handle" missing
  prerequisites or retry composition logic
- any such condition is a downloader/integrity consistency problem, not a
  normal engine branch

### 6. Migration scope

- the implementation work is not "fix the visible bug"
- it is a full migration to remove every discrepancy found against the current
  `specs/*.md` contract
- any code path still violating queue ownership, downloader/engine separation,
  integrity recovery rules, synthetic-feed modeling, or operator-visible fault
  reporting must be fixed in the same migration

### 5. Processing exceptions are severe faults

- unexpected processing exceptions mean a serious bug, data-consistency error,
  or operator-visible fault
- they must be:
  - logged clearly
  - surfaced in the admin UI
  - treated as requiring human intervention unless an explicit recovery path is
    defined in specs

## Spec work ordered — 2026-04-21

Before further code migration, the specs must be tightened to state
unambiguously:

- queue ownership and state transitions
- that downloader admission to processing is exclusive
- that admin actions are the only direct processing entry point
- that downloader and processing maintain independent state
- that the engine processes feed bodies only and does not synthesize feeds
- that unexpected engine-side exceptions are severe operator-visible faults

## Audit target — 2026-04-21

The next spec audit must verify that the documentation is clear enough that an
independent implementor would conclude:

- which loop owns each queue
- which stage is allowed to admit work into processing
- what restart recovery is allowed to do
- which admin actions bypass download
- which failures belong to downloader, integrity, or engine
- which conditions are considered severe consistency faults

## Specs audit result — 2026-04-21

The workflow contract is now explicitly stated in the specs for:

- downloader-loop versus processing-loop ownership
- queue admission rules
- admin-only direct `reprocess`
- restart recovery of already admitted staged work
- downloader-originated provider-refresh full reprocess waves
- severe processing exceptions as distinct operator-visible faults

### Decision resolved

- integrity recovery is allowed to enqueue direct `reprocess` work only when
  the problem is engine-side/local-integrity breakage and the product already
  has enough committed or staged local feed-body state
- this is a narrow exception to the "admin-only direct processing" rule
- automatic direct processing admission is therefore limited to:
  - downloader-originated admission
  - restart recovery of already admitted staged work
  - integrity-triggered local-only engine-repair reprocess recovery
  - explicit admin `reprocess`

This closes the remaining workflow ambiguity in the specs.

## Confirmed implementation discrepancies to fix — 2026-04-21

The current code still diverges from the specs in these confirmed ways:

1. One combined runtime coordinator still owns both loops and both queue-state
   maps in `pkg/scheduler/scheduler.go`, instead of exposing clearly separated
   downloader/processing ownership.

2. Manual `recheck` for non-downloadable feed families bypasses the downloader
   stage and enqueues processing directly in `pkg/scheduler/scheduler.go`.

3. Manual `run` also bypasses downloader-stage admission and enqueues
   processing directly in `pkg/scheduler/scheduler.go` and the admin run
   endpoints.

4. Successful staged downloads are not promotable when `LastStatus` is
   `downloaded`, and lingering `.new` files are then requeued directly into the
   processing queue.

5. Provider refresh currently admits the provider source itself into processing
   rather than enqueueing the public-feed reprocess wave as downloader-originated
   processing work.

6. Queue snapshots do not carry live downloader/processing error detail, so the
   admin queue panels cannot show the exact failure cause for an item.

7. Feed row state is still derived primarily from settled cache fields, with
   stale status labels and no strict downloader/integrity/severe-engine fault
   separation.

8. `anonymous` and `satellite` still exist as validator-only magic names, not
   as first-class hidden synthetic feed definitions with local feed bodies and
   admin visibility.

9. Integrity still checks only output-file consistency and does not validate
   blocked downloader composition prerequisites such as missing required local
   merge inputs.

10. The processing engine still contains hardcoded feed-specific behavior
    (`rfc_reserved` special casing) that must be removed from normal processing
    paths or reduced to ordinary synthetic-feed input handling.

11. Operator-facing and internal naming still uses `rebuild` in several paths
    where the contract now says `reprocess`, so code/API terminology must be
    aligned to the specs.

12. Tests need to be updated/expanded to cover all of the above before the
    migration can be considered closed.

13. Downloader outcome handling still relies on ad-hoc string literals rather
    than a strongly typed enum contract, so the downloader/engine/scheduler
    boundary remains too easy to drift or misuse.

14. Processing-stage per-feed results also still rely on string status values.
    The processing boundary must instead return `ok` for successful execution
    and a typed exception enum for every failure class.

Impact:

- `firehol_anonymous` can legally reference `anonymous` in config even though
  it is not a first-class feed body source
- admin cannot show this synthetic feed in the feed inventory
- the synthetic producer/consumer contract is inconsistent

## Fix sequence — 2026-04-21

### A. Repair downloader promotion semantics

- make successful staged feed-body downloads promotable after processing
- remove the false "staged therefore failed" loop
- verify that a successful batch removes `.source.new` for:
  - plain feeds
  - merges
  - history derivatives
  - artifact-backed child feeds

### B. Keep downloader failures in downloader queues

- ensure downloader failures never bounce directly into
  `waiting to be processed`
- failed downloader-stage work should remain:
  - visible in downloader status
  - retryable by cadence / manual recheck
  - absent from the processing queue unless there is a valid staged body

### C. Extend live queue payloads with per-item error/status detail

- add live downloader / processing status text to queue snapshots
- show the exact downloader error in:
  - `being downloaded now`
  - and, where relevant, `waiting to be downloaded`

### G. Replace stringly-typed downloader outcomes with a typed enum

- define one authoritative downloader outcome type
- use typed constants inside downloader / engine / scheduler logic
- convert to operator-facing strings only at persistence / API boundaries

### H. Replace stringly-typed processing results with typed success/exception contracts

- per-feed processing returns `ok` on success
- processing failures return a typed exception enum, not free-form status
- change/no-change semantics must become explicit fields, not overloaded status

### D. Align feed row state with live runtime state

- merge cache state with queue state when building admin feed rows
- a feed currently failing in downloader or processing should surface that
  active condition instead of only the last settled cache value

### E. Promote geolocation-derived synthetic feeds to first-class hidden sources

- model `anonymous` and `satellite` the same way the project models
  `rfc_reserved`
- give them:
  - stable feed identity
  - synthetic source URL / provenance
  - committed feed body semantics
  - admin visibility
- stop relying on validator-only reserved-name magic

### F. Extend integrity to validate blocked synthetic inputs

- for merges:
  - report missing required committed feed bodies of currently enabled inputs
- for other synthetic families:
  - verify the local prerequisites needed for downloader-side composition
- keep the existing rule that integrity is about settled local correctness, not
  transient in-flight work

## Testing additions required

- unit test: successful staged prepared-body downloads are promoted and do not
  remain `.new`
- unit test: merges with missing inputs stay downloader-failed and do not enter
  processing
- unit test: admin queue snapshot includes live downloader error detail
- unit test: admin feed row status prefers live queue failure over stale cache
  success
- unit test: `anonymous` / `satellite` appear as hidden synthetic feeds in
  admin feed inventory
- unit test: integrity flags merge blocked on missing required local input

## Regression discovered after install — 2026-04-19

### Symptom

- Multiple feeds fail during startup integrity recovery with errors like:
  - `source file does not exist at /opt/update-ipsets/data/blueliv_crimeserver_last.source`
- The failures affect feeds that the integrity check queued for
  `startup_integrity_rebuild`.

### Evidence

- `pkg/web/server.go` queues startup integrity findings with:
  - `runner.TriggerSources(... Rebuild: true, Reason: startup_integrity_rebuild)`
- `pkg/web/integrity.go` manual integrity reprocess does the same:
  - `runner.TriggerSources(... Rebuild: true, Reason: integrity_rebuild)`
- `pkg/engine/integrity.go` emits findings such as:
  - `source output file missing (cache says processed)`
- `pkg/engine/process.go` processing-only rebuild then expects a committed or
  staged `.source` file and fails if it does not exist.

### Root cause

- The new split pipeline uses `SkipPrefetch=true` for processing batches.
- Integrity recovery currently treats **missing source output** as a
  processing-only rebuild.
- That is wrong for downloadable feeds and their derivatives:
  - there is no committed `.source` to process
  - derivatives may also be queued directly even though their roots need
    re-download / re-materialization first

### Fix direction

- Integrity-triggered queueing must distinguish:
  - feeds that can be fixed by processing-only rebuild
  - feeds that first need a fresh fetch / parent recovery
- Startup/manual integrity recovery must not queue derivative names directly for
  missing-root-source cases.

### Additional regression discovered — provider databases in processing queue

- Operators now see ASN / GeoIP provider sources in the normal
  "currently being processed" queue.
- These providers are still handled by the heavy block, not by the normal feed
  processing pipeline.
- This suggests the new split scheduler is promoting provider downloads into the
  same queue semantics as ordinary feeds, which is misleading and may also keep
  the batch looking busy for a long time while database work runs.

### Additional regression discovered — processing wait list lacks queue age

- The admin "waiting to be processed" list currently shows only the trigger
  reason (for example `scheduled_due`).
- It does not show how long each feed has already been waiting in the queue.
- For operators, queue age is the important signal here; the reason alone is not
  enough to evaluate backlog or stuck work.

### Additional regression discovered — "currently being processed" shows full batch, including failures

- The admin "being processed now" panel currently renders the whole active batch
  membership from `engine.batch_feeds`.
- That means it shows terminal states like `failed`, `updated`, or `skipped`
  while the batch is still running.
- Costa wants this panel to show only feeds that are actually in flight right
  now, not the whole batch ledger.

### Decision recorded — 2026-04-19

- The "being processed now" panel must show only in-flight work.
- Terminal batch statuses (`failed`, `updated`, `skipped`) should not appear in
  that panel while the batch is still running.
- The only valid processing batch in the new runtime is the scheduler's
  drained `processingWaiting -> processingActive` snapshot for the current
  tick.
- The separate engine-side `batchFeeds` ledger is the wrong abstraction and
  must be removed.
- Admin/status views must derive "running" from the scheduler processing batch,
  not from an engine-local batch ledger.

### Additional regression discovered — same-batch derivatives are not reflected in `being processed now`

- Specs require history derivatives to execute in the same processing batch,
  after their parent outputs exist.
- The engine already does this:
  - `pkg/engine/run.go` dynamically injects retention/history derivatives into
    the current `RunOnce()` when a parent finishes with `updated`
  - `pkg/config/dependents.go` explicitly documents this as same-tick behavior
- The scheduler/admin exposure does not reflect it:
  - `pkg/scheduler/scheduler.go` marks `processingActive` only from the queue
    snapshot drained before `RunOnce()` starts
  - derivatives injected later inside `RunOnce()` are therefore absent from
    `queues.processing_active`
  - `ui/src/components/admin/current-run.tsx` renders `being processed now`
    from `queues.processing_active`, so injected derivatives appear stuck in
    `waiting to be processed` or disappear from the active list entirely

### Resolved direction — active processing must reflect the real in-flight set

- The backend source of truth for `being processed now` must be the real
  in-flight feed set, including dynamically injected derivatives.
- The existing `engine.active_feeds` map already tracks the real active
  per-feed work and should be used as the operator-facing active-processing set.
- The scheduler's `processingActive` list may remain internal bookkeeping for
  the drained queue snapshot, but it must not be exposed to operators as the
  full active batch if it omits same-run injected work.
- The admin UI should stop showing `"waiting turn"` entries inside
  `being processed now`; only feeds that are actually executing now belong
  there.

### Decision recorded — 2026-04-20: downloader owns final feed synthesis for every feed family

The intended architecture is now explicit:

- the downloader is responsible for producing the final pre-processed feed body
  for **all** feed families
- this includes:
  - remote URL feeds
  - local file feeds
  - history derivatives
  - merges
  - artifact-backed families such as DroneBL
- parsing / cleanup that turns the raw upstream or synthetic input into the
  feed body to be processed belongs to the downloader stage, not to the engine

Implications:

- the downloader decides and reports:
  - `updated`
  - `not_updated`
  - `same`
  - `failed`
  - `empty`
  - or equivalent downloader-stage outcomes
- only feeds whose downloader outcome means "this feed body should be processed
  now" enter `waiting to be processed`
- all other downloader outcomes do not enter the processing queue unless an
  explicit operator action forces reprocessing

### Decision recorded — 2026-04-20: queued feeds must always run the full engine pipeline

Once a feed enters `waiting to be processed` and the engine starts it:

- the engine MUST NOT refuse to process it because it appears unchanged
- the engine MUST run the full processing/publication pipeline for that feed
- the pipeline must be idempotent and graceful when reprocessing equivalent
  input

This is required so that reprocessing can refresh downstream artifacts when:

- ASN databases changed
- GeoIP databases changed
- critical infrastructure classification inputs changed
- website/public artifact contracts changed
- application behavior changed after deployment
- an operator explicitly requests reprocessing

### Decision recorded — 2026-04-20: strict downloader vs engine separation

The architecture is now explicitly split into two responsibilities:

- downloader responsibilities:
  - acquire upstream/local input
  - transform it into the final feed body for processing
  - decide whether the prepared body is `updated`, `same`, `not_updated`,
    `failed`, `empty`, or another downloader-stage outcome
  - queue only the feeds that should be processed
- processing engine responsibilities:
  - maintain feed-local artifacts such as age/retention/churn history
  - compare the feed against ASN, GEO, and other feeds
  - generate insights
  - update reverse comparisons/overlaps from the perspective of all other feeds

Feed-family implications:

- history derivatives:
  - downloader composes them immediately after the parent feed is updated by
    combining the new parent output with the previous derivative/history state
- merges:
  - downloader reconstructs them from processed source feeds
  - they are still cadence-driven, but composition belongs to downloader, not
    engine
- artifact-backed/custom families:
  - downloader-specific integrations may exist, but the output contract remains
    the same final prepared feed body

Reprocessing implications:

- if ASN providers or GEO providers are updated, the system MUST schedule a
  full reprocess of all feeds
- the engine must not refuse queued work because the prepared input matches the
  previous body
- unchanged input may still yield changed enrichment/insight/comparison output

### Decision recorded — 2026-04-20: peer comparison views must stay current on both sides

When feed `A` changes, the product must not refresh only `A`'s own comparison
view.

It MUST also refresh the peer-facing comparison artifacts that other feeds keep
about `A`, because:

- if `A` removes IPs, a peer feed that already overlapped with `A` now has a
  changed comparison row for `A`
- if `A` adds IPs, any peer may now overlap with `A` even if it did not before

Implications:

- a change in one feed requires re-evaluating its pairwise comparison against
  every other relevant feed
- the refreshed comparison result must be written to both sides' public
  comparison artifacts
- any peer-facing derived artifact that depends on those comparison rows must be
  refreshed too, otherwise the site becomes "current at peer update time"
  instead of "current now"

### Pending decision — 2026-04-20: overlap-triggered insights refresh scope

Need to define which insight rules must be rerun when only pairwise comparison
rows changed for a feed.

Evidence from the current rules:

- overlap-dependent rules:
  - `pkg/insights/rules_overlap.go`
  - `independent` reads `SignalSnapshot.Overlaps` but is rendered in
    `SectionOverview`
  - `subset_of` reads `SignalSnapshot.Overlaps`
  - `cross_category_overlap` reads `SignalSnapshot.OverlapsByCat`
- feed-local-only rules:
  - retention rules read `AgeOfListed` / `AgeOfRemoved`
  - trend rules read `SizeSeries` / `ChurnSeries`
  - composition rules read `TopCountries`, `BogonShare`, `InfraShare`

Design implication:

- refreshing only the UI section named `relationships` is not enough
- the contract should split insight recomputation by dependency family:
  - local-only insights
  - comparison-dependent insights

### Gap identified — current implementation violates this architecture

Current code still mixes responsibilities:

- internal feed synthesis (especially merges/retention) happens inside the
  engine processing path
- the engine short-circuits internal feeds when their generated body matches the
  previous reference file
- this causes queued feeds to be reported as `skipped` instead of being fully
  reprocessed through the pipeline

Additional concrete gaps found during the contract audit:

- the scheduler still enqueues cadence-driven merges straight into the
  processing queue instead of routing them through downloader-stage composition
- history derivatives are still injected dynamically from inside the engine run
  after a parent reports `updated`, instead of being composed by the downloader
- provider databases are still refreshed inside the heavy block and currently
  trigger fan-out writers directly, rather than queuing a full feed reprocess
  wave
- the engine still applies local mtime/content sameness checks after a feed has
  already entered processing, which violates the new "queued work is mandatory"
  rule

Resolved direction:

- move "is the final feed body updated/same/failed/empty?" decisions entirely
  into the downloader stage
- make the engine treat every queued feed as mandatory full processing work
- reserve engine-level skipping only for feeds that never entered the queue

### Additional regression discovered — admin integrity panel stays stuck in "waiting for the active run to finish"

- The admin integrity UI shows:
  - `Integrity check is waiting for the active run to finish.`
- Costa reports that this message never clears, even after the run view has
  otherwise moved on.
- Live verification showed the backend was already settled (`running: false`,
  real findings returned), so the real bug is in the admin UI refresh path.
- The integrity panel currently does not poll after it first sees
  `status: "in_progress"`, so it can stay stuck on the waiting message until
  the operator manually refreshes it.

### Additional regression discovered — startup blocks web availability while recomputing feed stats

- Live evidence:
  - startup log shows `cache loaded` at `23:15:08`
  - first `update-ipsets daemon listening` log appears at `23:16:39`
  - so about 92 seconds are spent before the web server binds the socket
- Code evidence:
  - `runDaemon()` calls `engine.New()` before `web.Run()`
  - `engine.New()` calls `reconcileEntriesFromSourceConfig()`
  - that function currently recalculates:
    - `refreshHistoryStatsFromLedger()`
    - `refreshRotationStatsFromLedger()`
  - rotation refresh still calls `HistorySeries()`, which scans
    `data/history/<feed>/*.set`
- Current live scale:
  - `data/history/*.set`: 21,937 files
  - `lib/*/history.csv`: 1,625 files
  - `lib/*/changesets.csv`: 597 files
  - `.cache.json` entries: 1,632
- Resolved fix direction:
  - startup reconciliation must be metadata-only
  - do not recompute history/rotation analytics in `engine.New()`
  - keep only cheap config-derived field sync for existing cached entries

### Additional regression discovered — integrity reports dead/unavailable feeds as broken local pipelines

- Costa reports that a missing local output/source paired with a feed that is
  no longer refreshable upstream produces persistent admin noise.
- Current code path:
  - `CheckIntegrity()` emits `source output file missing (cache says processed)`
    whenever a previously processed feed has no current local output file
  - it does this without checking whether the feed is already classified as
    `unavailable` / non-actionable
- Resolved fix direction:
  - missing local output should not be treated as an integrity failure when the
    feed is already unavailable and there is no actionable recovery path
  - integrity must remain focused on repairable local pipeline breakage, not
    permanent upstream disappearance

## Resolved direction — shared artifact parents

Facts:

- DroneBL feeds are not normal one-URL-per-feed downloads.
- `pkg/engine/dronebl.go` calls `dronebl.Update(...)` once and that helper writes multiple `dronebl_*.source` files from one downloaded `buildzone`.
- All DroneBL feeds currently share the same catalog cadence (`frequency: 1`) and same format (`dronebl_buildzone_class`) in `configs/firehol.yaml`.

Why this mattered:

- The new runtime is per-feed queued/staged.
- DroneBL is one remote fetch that can update many feeds at once.
- To move DroneBL into the single download loop, the scheduler needs an explicit bundling rule.

Original options considered:

1. Bundle fetch:
   - any due/manual DroneBL feed fetches the `buildzone` once
   - all enabled DroneBL outputs are staged together
   - processing queues the staged DroneBL feeds that were refreshed
2. Keep current exception:
   - leave DroneBL inside `RunOnce()` for now
   - finish the rest of the rewrite first
3. Bigger refactor:
   - introduce an explicit shared `dronebl_buildzone` source and make the per-class DroneBL feeds true derivatives of it

Initial recommendation at that stage:

- Option 1.
- This was the smaller short-term patch, but it was later superseded by the
  more general shared-parent design.

User challenge added on 2026-04-19:

- Costa questioned whether the repeatedly-valid design is not the ad-hoc
  DroneBL bundle fetch, but the more general model:
  - explicit shared upstream source
  - downstream derived feeds from that shared source
- This needs to be evaluated against the existing retention/merge
  derivative architecture before implementation continues.

Conclusion from code review:

- Yes: the repeatedly-valid design is the generalized shared-upstream
  model.
- In this codebase that means:
  - one explicit downloadable parent source for the shared remote artifact
  - child feeds as deterministic derivatives of that parent
  - child processing reads the parent's committed/staged artifact instead of
    downloading anything itself

Why this fits the current architecture:

- `ExpandDerivatives` already turns curator sugar into first-class sources
  with `DerivedFrom` plus an internal provider URL.
- `DetectCycles` and `Dependents` already exist to manage derivative graphs.
- `RunOnce` already supports parent-first processing with immediate
  derivative injection for retention windows.

Recommended pattern for future bundle-style providers:

- Introduce a new hidden parent source representing the shared downloaded
  artifact.
- Add a dedicated internal provider scheme for child extraction from that
  parent artifact.
- Treat each exposed child feed as a normal first-class source derived from
  the hidden parent.

Implication:

- This is a larger but cleaner refactor than the ad-hoc DroneBL bundle
  special-case.
- It is the right reusable pattern if more "one download -> many feeds"
  providers are expected.

User decision on 2026-04-19:

1. C
   - adopt the reusable shared-parent model
   - implement DroneBL as:
     - one hidden downloadable parent artifact source
     - child feeds as first-class derivatives of that parent

Implementation issues discovered after choosing C:

- The current runtime enables feeds by the presence of `BASE_DIR/<name>.source`
  (`pkg/engine/helpers.go`) and `update-ipsets enable` touches that exact path
  per feed (`pkg/engine/engine.go`).
- A hidden parent artifact source would therefore get its own enable bit unless
  we define different semantics.
- The first draft assumption was "child recheck fetches the shared parent and
  queues siblings".

User decisions / corrections on 2026-04-19:

- The shared parent must have its own explicit catalog entry and its own
  global enable/disable state.
- The parent is the master. A derivative must not control or trigger fetch of
  the parent during `recheck`.
- Disabling a feed must cascade through dependency structure:
  - disabling a parent disables derivatives that rely only on it
  - disabling a feed removes it from merges
  - if a merge is left with zero enabled inputs, that merge becomes disabled
- Rechecking a derivative is processing-only for that derivative; it must not
  recheck the parent/source.

Implication:

- The current assumption "child recheck fetches parent and queues siblings"
  was wrong and is now explicitly rejected.
- The shared-parent model is therefore not just a hidden downloadable parent;
  it introduces a new dependency-aware enablement contract for the whole
  catalog.

New design question raised by Costa on 2026-04-19:

- Do not mix artifact parents with normal IP feeds unless there is a strong
  reason.
- Administrators must be able to say "I want dnsbl" / "I do not want dnsbl"
  globally with one flag.
- Candidate designs:
  - a dedicated config section for downloadable artifacts / bundle parents
  - provider-specific hardcoded sections / schemes such as `dnsbl://...`
- Costa explicitly accepts provider-specific schemes if that is the cleanest
  fit for this codebase, because DroneBL support is already provider-specific
  and the system is not trying to be a general plugin platform.

Concrete design options to decide next:

1. Config shape for shared downloaded parents
   - A. New top-level `artifacts:` section.
     - Artifact parents live outside `sources:`.
     - Child IP feeds remain in `sources:` and explicitly reference the artifact parent.
     - This keeps normal feed inventory separate from non-feed downloadable inputs.
   - B. Keep artifact parents inside `sources:` with a new role / source kind.
     - Simpler in-memory reuse of current `cfg.Sources`.
     - But it mixes non-feed parents into feed counts, scheduler/admin inventories,
       enable semantics, and public/admin model code that currently assume
       `cfg.Sources` is the feed universe.
   - C. Provider-specific top-level section (for example `dronebl:` only).
     - Smallest change for one provider.
     - But becomes another ad-hoc branch when the next bundle-style provider arrives.

2. Parent URL / scheme style
   - A. Provider-specific URL schemes such as `dronebl://buildzone`.
     - Honest about the current codebase shape: provider-specific acquisition and
       parsing already exist.
   - B. Generic `artifact://...` / `bundle://...` scheme.
     - Looks more abstract, but the engine still needs provider-specific fetch and
       child-extraction logic behind it.

3. Admin visibility of artifact parents
   - A. Separate admin inventory for artifact parents, not part of the feeds table.
     - Fits Costa's "do not mix concerns" requirement.
   - B. Keep artifact parents in the existing all-feeds table as hidden/special rows.
     - Reuses current UI paths.
     - But administrators now see non-feeds in the feed inventory.

Current recommendation from code review:

- 1. A
- 2. A
- 3. A

Reason:

- `cfg.Sources` is currently treated as the feed universe by validation, enable
  state, scheduler snapshots, admin summaries, and admin feed-table builders.
- A dedicated `artifacts:` section keeps that model honest and avoids turning
  every current feed-oriented path into a mixed feed/artifact path.
- A provider-specific URL such as `dronebl://...` matches the code we already
  have better than pretending this is a generic plugin framework.

New framing raised by Costa on 2026-04-19:

- Treat bundle-style providers as "external producers" of local files:
  - an operator may run a cronjob outside update-ipsets
  - that cronjob downloads/extracts one or more feed files into a directory
  - update-ipsets then ingests those local files as normal feeds
- DroneBL can be framed as the same pattern, except its producer is shipped
  inside update-ipsets instead of being an external cronjob.

Facts discovered from code review:

- Raw `file://` URLs are currently rejected by both validation and the
  downloader fetch path:
  - `pkg/config/validate.go`
  - `pkg/downloader/downloader.go`
- But the downloader already supports local-file ingestion through the explicit
  override `downloader: copyfile` plus `downloader_options: <path>`:
  - `pkg/downloader/downloader.go`
- The engine fetch/stage loop already passes `Downloader` and
  `DownloaderOptions` through for normal sources:
  - `pkg/engine/download_stage.go`
- The catalog tests already recognize that some sources are populated by
  external scripts rather than direct URLs:
  - `pkg/config/catalog_verify_test.go`

Implication:

- The "external producer -> local files -> update-ipsets ingests them" model is
  already compatible with the current codebase.
- It does NOT require enabling raw `file://` URLs.
- The existing `copyfile` downloader is the safer and cleaner fit because it
  keeps local-file access explicit instead of weakening the URL-scheme guard.

Updated recommendation:

- The framing is better.
- Reusable pattern:
  1. external or internal producer writes local files into its own directory
  2. child feeds ingest those files through explicit local-file handling
  3. parent producer enablement stays separate from child feed processing
- That first recommendation was later rejected in favor of standard
  `file://...` syntax because backward compatibility is not a constraint.

User decision on 2026-04-19:

1. A
   - keep an explicit parent/artifact model
   - child feeds may be file-backed, but parent semantics remain first-class

Resolved design question:

2. Local-file ingestion syntax
   - Current code uses a custom downloader override:
     - `downloader: copyfile`
     - `downloader_options: /path/to/file`
   - Costa questioned whether this should be replaced by the standard-looking
     `file://...` URL scheme.

Facts relevant to decision 2:

- `copyfile` is not a protocol. It is an explicit branch in the downloader:
  - `pkg/downloader/downloader.go`
- `copyfile` streams the local file through the same temp-file + hash +
  size-limit path as HTTP downloads:
  - `pkg/downloader/downloader.go`
- The fetch/stage loop already forwards `Downloader` and `DownloaderOptions`
  for normal sources:
  - `pkg/engine/download_stage.go`
- Raw `file://` is rejected today by both validation and the downloader's URL
  scheme checks:
  - `pkg/config/validate.go`
  - `pkg/downloader/downloader.go`
- Admin already exposes `downloader` and `downloader_options` for feeds:
  - `pkg/web/admin.go`

Implication:

- Keeping `copyfile` means less engine churn; the local-file path already
  exists and already works with the new fetch/stage loop.
- Switching to `file://` gives a more obvious operator-facing syntax, but
  requires widening the scheme whitelist in both validation and downloader
  fetch, and deciding whether we keep `copyfile` for backward compatibility.

User clarification on 2026-04-19:

- Backward compatibility is NOT a constraint for this project yet.
- The project is still early and not released.

Updated implication:

- There is no reason to preserve `copyfile` as a public configuration surface
  just to avoid churn.
- If `file://` is the better model, we can switch fully to it and remove
  `copyfile` from the intended configuration contract.

User decision on 2026-04-19:

2. C
   - drop `copyfile` as the intended configuration surface
   - use `file://...` for local-file child feeds
   - update config/tests/docs accordingly if they use `copyfile`

Current impact check:

- The catalog/config does not currently use `downloader: copyfile`.
- Current `copyfile` usage is confined to implementation/tests/legacy-cache
  compatibility code:
  - `pkg/downloader/downloader.go`
  - downloader tests
  - `pkg/engine/engine_test.go`
  - `pkg/cache/legacy_test.go`

## Resolved design decisions before implementation continues

These are concrete shape decisions that were open during analysis and are now
explicitly decided by Costa. The behavior was already decided earlier:

- explicit parent/artifact model
- child feeds can be file-backed
- use `file://...` for local-file child feeds
- parent is the master for enable/disable semantics

## Remaining implementation design blocker

### How artifact parents hand child feeds to the existing processing pipeline

Facts from the current code:

- The processing loop runs `RunOnce(... SkipPrefetch=true)` and
  `processConcreteSource()` then expects to read a local committed/staged
  source file for the feed:
  - `pkg/scheduler/scheduler.go`
  - `pkg/engine/process.go`
- Feed enable/disable also depends on the existence of `BASE_DIR/<feed>.source`:
  - `pkg/engine/helpers.go`
  - `pkg/engine/engine.go`
- The current DroneBL helper already knows how to fetch one upstream artifact and
  materialize many child source files atomically:
  - `tools/dronebl2ipsets/main.go`
  - `tools/dronebl2ipsets/write.go`

Implication:

- If artifact refresh produces concrete child `.source.new` files, the current
  feed processing loop and enablement model stay mostly intact.
- If artifact refresh stores only the raw parent artifact, then artifact child
  feeds need a new on-demand extraction path during processing, and the current
  `.source`-based enable model no longer fits cleanly.

Options:

- A. Artifact refresh downloads the parent and materializes child staged source
  files.
  - Pros:
    - best fit with the current processing loop
    - keeps per-feed `.source` / `.source.new` semantics
    - matches Costa's "external cronjob writes files, update-ipsets ingests
      them" framing
    - reuses the existing DroneBL extraction code shape
  - Cons:
    - artifact refresh does download + extraction, not only raw download
  - Risks:
    - low; mostly localized artifact-parent orchestration

- B. Artifact refresh stores only the raw parent artifact; child feeds extract
  from it on demand during processing.
  - Pros:
    - cleaner separation between raw artifact storage and child-feed materialization
  - Cons:
    - requires a bigger refactor of processing
    - clashes with current `.source`-based enable/disable model
    - manual reprocess of a child becomes more complicated
  - Risks:
    - higher regression risk across scheduler, processing, and admin

Current recommendation:

- A

User decision on 2026-04-19:

- 1. A
  - artifact refresh downloads the parent and materializes child staged source
    files
  - child feeds continue to flow through the existing per-feed `.source` /
    `.source.new` processing contract

### Decision 1: top-level config shape for parents

Evidence:

- `Config` currently has only `Sources`, `Merges`, and other feed-oriented
  registries; there is no artifact registry:
  - `pkg/config/config.go`
- `cfg.Sources` is treated as the feed universe by:
  - validation
  - scheduler snapshots
  - admin feed summary and feed table
  - public catalog filtering

Options:

- A. Add a new top-level `artifacts:` section.
  - Pros:
    - clean separation of concerns
    - parent artifacts do not pollute feed inventory
    - matches Costa's "do not mix concerns" requirement
  - Cons:
    - more config/runtime plumbing now
  - Risks:
    - low; mostly mechanical refactor

- B. Put artifact parents into `sources:` with a new kind/role.
  - Pros:
    - reuses more existing loops immediately
  - Cons:
    - mixes non-feeds into feed counts/tables/scheduler/admin
  - Risks:
    - higher long-term code contamination

Recommendation:

- A

User decision on 2026-04-19:

- 1. A
  - add a new top-level `artifacts:` section
  - keep artifact parents out of `sources:`

### Decision 2: how a child feed points to its parent

Evidence:

- A `file://...` child path alone cannot express the master/child enablement
  rule Costa wants.
- Current `DerivedFrom` is loader-generated only (`yaml:\"-\"`), so child YAML
  needs a new explicit parent reference field if parent semantics matter:
  - `pkg/config/config.go`

Options:

- A. Child has both:
  - explicit parent field, e.g. `artifact: dronebl`
  - explicit local path URL, e.g. `url: file:///...`
  - Pros:
    - preserves parent semantics explicitly
    - keeps file location flexible for internal/external producers
    - best fit for the external-cronjob framing
  - Cons:
    - slightly more YAML
  - Risks:
    - low

- B. Child has only `url: file://...`, no explicit parent field.
  - Pros:
    - minimal YAML
  - Cons:
    - loses explicit parent relationship
    - parent enable/disable cascade becomes path inference or ad-hoc mapping
  - Risks:
    - wrong abstraction

- C. Child has explicit parent only; engine derives the file path from parent
  conventions.
  - Pros:
    - less YAML on child feeds
  - Cons:
    - more hidden conventions
    - weaker support for external producers choosing their own paths
  - Risks:
    - pushes provider/path rules into code

Recommendation:

- A

Additional UX constraint raised by Costa on 2026-04-19:

- Plain `file://...` is fine for external producers where the operator owns the
  delivery directory and already knows the path.
- For internal producers such as DroneBL, requiring child-feed config to know
  the internal delivery directory is undesirable UX.
- Costa's concern is configuration UX:
  - simple
  - predictable
  - clean

Implication:

- One child-reference syntax may not fit both cases cleanly.
- The likely clean split is:
  - external producers: explicit `file://...`
  - internal managed artifacts: parent-relative logical child reference that
    hides the delivery directory

Refined design question discovered from that UX constraint:

- The earlier `2. C` decision ("use `file://...` for local-file child feeds")
  is still valid for free-standing local files.
- But for artifact-backed children specifically, `file://...` may be the wrong
  UX because it leaks internal delivery directories into the catalog.

Refined options for artifact-backed child syntax:

- A. Explicit parent-relative fields, for example:
  - `artifact: dronebl`
  - `artifact_file: auto_botnets.txt`
  - Pros:
    - cleanest config UX
    - no leaked internal paths
    - explicit parent relationship
    - works for both internal and external artifact producers
  - Cons:
    - requires new schema fields
  - Risks:
    - low

- B. Provider-specific child URL, for example:
  - `url: dronebl://auto_botnets`
  - Pros:
    - hides the path
    - concise
  - Cons:
    - provider-specific child schemes proliferate
    - less explicit than dedicated parent/reference fields
  - Risks:
    - medium long-term sprawl

- C. Full `file://...` even for artifact-backed children
  - Pros:
    - one syntax everywhere
  - Cons:
    - leaks internal delivery layout
    - worse UX for managed internal artifacts
  - Risks:
    - config tied to runtime filesystem layout

Current recommendation:

- A

Note:

- This does not invalidate `file://...` support.
- It narrows its intended use:
  - use `file://...` for free-standing local-file feeds
  - use `artifact` + `artifact_file` for artifact-backed children

New design refinement raised by Costa on 2026-04-19:

- Costa wants one simple, predictable pattern for what a source URL may contain.
- Artifact-backed child sources need a URL that captures:
  1. this is artifact-derived
  2. which artifact parent it comes from
  3. which delivery / part of that artifact becomes the feed source

Candidate URL patterns:

1. `dronebl://auto_botnets`
   - provider-specific child scheme
   - artifact parent is implied by the scheme

2. `artifact://dronebl?part=auto_botnets`
   - generic artifact child scheme
   - artifact parent is the host / authority
   - requested delivery is an explicit query parameter

3. `artifact://dronebl/auto_botnets`
   - generic artifact child scheme
   - artifact parent is the host / authority
   - requested delivery is the path

Important correctness note:

- Reusing `merge=` as the query parameter name would be misleading.
- This is not merge semantics. It is selecting one delivery from one parent
  artifact.

Additional evidence on 2026-04-19:

- DroneBL child selection is already multi-valued today:
  - `OutputSpec.Lists []string`
  - `ParseListNames()` parses comma-separated names
  - `BuildOutputs()` unions all selected lists for that output
  - `dronebl_anonymizers` in the catalog already maps one feed to multiple
    DroneBL lists

Implication:

- The generic artifact child URL should not use singular `part=` if the
  selected delivery set can legitimately contain more than one logical part.
- To match both actual DroneBL semantics and the existing merge URL style, use
  plural `parts=` with a comma-separated list.

Current recommendation after Costa's UX clarification:

- Prefer a single generic artifact URL pattern over provider-specific child
  schemes.
- Best candidate:
  - `artifact://dronebl?parts=auto_botnets`

Why:

- It keeps one URL pattern for all artifact-backed children.
- It matches the existing "scheme + query parameters" style already used by:
  - `internal://merge?...`
  - `internal://retention_window?...`
- It avoids leaking internal filesystem paths.
- It keeps the provider identity explicit without inventing one scheme per
  provider.

User decision on 2026-04-19:

- Artifact-backed child source URLs use the generic artifact scheme with plural
  list selection:
  - `artifact://dronebl?parts=auto_botnets`
  - `artifact://dronebl?parts=http_proxies,socks_proxies,...`
- This replaces the earlier temporary thoughts about:
  - `artifact` + `artifact_file` fields
  - provider-specific child URLs like `dronebl://...`

### Decision 3: artifact config syntax inside `artifacts:`

Evidence:

- DroneBL already has provider-specific logic and fields:
  - buildzone rsync URL default
  - password from env
  - work dir / buildzone path
  - `tools/dronebl2ipsets`
- Once artifacts are separate from sources, they no longer need to pretend to
  be normal feed URLs unless we want them to.

Options:

- A. Artifact has explicit provider type, for example:
  - `type: dronebl_buildzone`
  - plus provider-specific fields like `rsync_url`
  - Pros:
    - clear config contract
    - does not fake artifact parents as feed URLs
    - better fit for a dedicated `artifacts:` section
  - Cons:
    - requires a new artifact dispatcher
  - Risks:
    - low

- B. Artifact uses provider-specific URL syntax, for example:
  - `url: dronebl://buildzone`
  - Pros:
    - reuses the URL-dispatch pattern conceptually
  - Cons:
    - artifacts are not really URLs users fetch directly
    - still mixes transport and provider identity a bit
  - Risks:
    - medium readability risk

- C. Special-case top-level `dronebl:` block only.
  - Pros:
    - smallest code now
  - Cons:
    - not reusable for the next provider
  - Risks:
    - guaranteed new special cases later

Recommendation:

- A

User decision on 2026-04-19:

- 3. A
  - artifact parents use explicit type-based config inside `artifacts:`
  - example shape:
    - `artifacts.dronebl.type: dronebl_buildzone`

### Decision 4: admin visibility of artifact parents

Evidence:

- Costa explicitly wants administrators to enable/disable the parent globally.
- Current admin feed table is explicitly requested to remain as it is today.

Options:

- A. Add a separate admin artifact inventory / section, not part of the feed
  table.
  - Pros:
    - respects "do not mix concerns"
    - keeps the current feed table intact
  - Cons:
    - more UI/API work now
  - Risks:
    - low

- B. Do not expose artifacts in admin yet; only CLI/API can manage them.
  - Pros:
    - less initial UI work
  - Cons:
    - weaker operator story than requested
  - Risks:
    - likely immediate follow-up work

Recommendation:

- A

User decision on 2026-04-19:

- 4. A
  - admin gets a separate artifact inventory / section
  - artifact parents do not appear in the existing all-feeds table

## Purpose

The current runtime still couples **download timing** to **processing timing** inside one `RunOnce()` execution. Even though there is a prefetch phase, the scheduler still waits for the whole fetch stage to complete before the processing stage starts. Costa wants these concerns separated completely:

- fetch/change detection keeps running and populates a "changed feeds" queue
- processing drains that queue on its own cadence
- the expensive pipeline only runs for feeds already proven changed

## Analysis — current state (facts only, no interpretation)

### Current runtime shape

- `RunOnce()` is still a **single batch execution** with ordered phases:
  - preflight
  - prefetch
  - sources
  - geoip
  - bogons
  - asn
  - metadata
  - insights
  - publish
- Evidence: `pkg/engine/run.go:15-252`, plus phase transitions later in the same file.

- The fetch path and processing path are not independent loops today.
  - `RunOnce()` calls `prefetchSources()` first at `pkg/engine/run.go:69-74`
  - only after `prefetchSources()` finishes does the source worker pool begin at `pkg/engine/run.go:81-234`

- This means a slow fetch phase still delays the whole processing batch, even if 90% of feeds are already known and ready to process.

### Data model

- `pkg/config/config.go:Source` has `History []int` (minute windows) and `Output string` (`"ip"`, `"net"`, `"both"`, `"split"`).
- `pkg/config/config.go:Merge` is a separate struct with `Sources []string`.
- Top-level YAML has both `sources:` and `merges:` blocks.
- `pkg/engine/public.go:configuredNames` walks `cfg.Sources` AND enumerates retention variants from `src.History` AND walks `cfg.Merges`. It produces a flat set of all "output names" the engine considers live.

### The pipeline today

`RunOnce` (`pkg/engine/run.go:15`) does:

1. `ensureDirectories` + `applyRenamesAndDeletes`
2. `prefetchSources` — parallel HTTP downloads, pool = `ParallelDownloads`
   - default is `8`, not `5`
   - configurable via runtime
   - evidence: `pkg/engine/runtime.go:174-176`
3. Phase 1: source processing — parallel pool = `MaxProcessingWorkers` (default 2). Each `processConcreteSource`:
   - applies the prefetched outcome,
   - parses + runs processors,
   - calls `finalize(name)` which writes `data/{name}.ipset`, `lib/{name}/latest.set`, the `.setinfo`, and bumps the cache entry
   - calls `updateRetention` (previous vs. current diff → `new/` directory, retention histogram)
   - **if `src.History` is non-empty, calls `updateHistoryVariants` inline** → for each window, computes `historyUnion`, clones the parent's `*config.Source`, sets `clone.Name = "viriback_1d"`, calls `finalize` again. The retention variant gets its own cache entry, `lib/viriback_1d/latest.set`, etc. — BUT it is NOT added to `report.Updated`.
4. Phase 2: merge processing — parallel pool, same size. Each `processMerge`:
   - concatenates all input files,
   - builds a synthetic `*config.Source` with `URL = ""`, `Frequency = 0`, `History = nil`,
   - calls `processConcreteSource` with that synthetic source.
   - The merge IS added to `report.Updated` as its own name.
5. `skipHeavy` check: if no sources updated AND no database selected AND `SkipComparisonIfNoUpdates = true` AND not recheck AND not rebuild → skip the heavy block.
6. Heavy block (`run.go:202-261`):
   - `processGeoIPDatabases` → loads all Geo provider data into memory (one provider at a time inside the per-provider loop inside `writeCountryComparisonFiles`, actually; but all providers are enumerated).
   - `writeCountryComparisonFiles(datasets, fanOutUpdated)` — fan-out targets decided by `targetFeedsForFanOut`.
   - `loadBogonSources` → **loads ALL bogon sources into memory at once** (6 providers today). Held for the rest of the heavy block.
   - `writeBogonComparisonFiles`
   - `buildBogonUnion` → materializes the bogon union as one in-memory `*iprange.IPSet` for the ASN three-bucket split.
   - `processASNDatabases` → loads ASN provider data.
   - `writeASNComparisonFiles` (uses bogon union).
   - Close bogons and ASN backings.
7. `writeMetadataFiles` → writes `{name}.json`, `index.json`, `all-ipsets.json`, `{name}.setinfo`, triggers `writeComparisonFiles` (pairwise overlap).
8. `writeInsightsForFeeds` — rule-driven insights per feed.
9. `cache.Save`.

### Where the bug manifests

`targetFeedsForFanOut` (my earlier fix at `helpers.go:486`) filters `outputNames` by `updatedNames`. Retention variants were NEVER in `updatedNames` before my fix, so the country/asn/bogon per-feed JSONs for `viriback_1d` were skipped every tick and the UI pages came up empty. My fix expands `["viriback"]` to include retention/split siblings but is a symptom patch.

### Scheduler model today

`pkg/scheduler/scheduler.go:BuildSnapshot` (`:154`) walks:

- `cfg.Sources` — builds one `Item` per source with `NextDue` computed from `frequency_minutes` + `entry.CheckedDate`
- `cfg.Merges` — builds one `Item` per merge with `mergeDue` (checks if any input file mtime > merge output mtime)

The runtime loop in `pkg/scheduler/scheduler.go:155-204` is:

- build a fresh snapshot from all feeds
- compute all feeds due **now**
- if any are due, call `RunOnce()` with all of them immediately
- then sleep at least `MinRunIntervalSeconds`

Important facts:

- there is no separate "fetch loop"
- there is no persisted "changed feeds" queue
- there is no fixed "process every Y minutes" cadence
- instead, processing runs whenever the scheduler finds due feeds, bounded by a minimum cooldown
- default cooldown is `30s`, not `10m`
  - evidence: `pkg/engine/runtime.go:183-184`

The CLI `daemon --interval` flag is not the scheduler cadence control for feed processing. The real runtime knobs are feed `frequency` plus `min_run_interval_seconds`.

### Downloader facts relevant to Costa's new requirement

- Downloads are already streamed to disk, not RAM.
  - the HTTP downloader writes the response body directly to a temp file in `TmpDir`
  - evidence: `pkg/downloader/downloader.go:213-242`

- Same-body detection is already disk-based:
  - after writing the temp file and hashing it, the downloader compares that hash to the on-disk reference file
  - evidence: `pkg/downloader/downloader.go:281-295`

- The fetch result already distinguishes:
  - `ok`
  - `not_modified`
  - `same`
  - `skipped`
  - `failed`
  - evidence: `pkg/downloader/downloader.go:17-25`

- The current prefetch worker pool already uses a concurrency semaphore:
  - `sem := make(chan struct{}, e.runtime.ParallelDownloads)`
  - evidence: `pkg/engine/process.go:103-120`

### Per-feed file map (current implementation)

#### 1. Source-feed input / source-of-truth files

- `BASE_DIR/<feed>.source`
  - meaning: downloaded source body currently used as input for processing
  - writer: `moveDownloadedBody(result, sourcePath)`
  - evidence: `pkg/engine/process.go:444-453`
  - current atomicity:
    - downloader writes to temp in `TmpDir`
    - then engine renames temp directly to final `.source`
    - there is **no** durable intermediate `.new` state today

- `BASE_DIR/<feed>.ipset` or `BASE_DIR/<feed>.netset`
  - meaning: final rendered output text file
  - writer: `finalize()`
  - evidence: `pkg/engine/finalize.go:17-53`
  - current atomicity:
    - `writeFileAtomic()` → temp in same directory, then rename to final

- `BASE_DIR/<feed>.setinfo`
  - meaning: sidecar metadata text used by the git/web sync path
  - writer: `writeMetadataFiles()`
  - evidence: `pkg/engine/metadata.go:69-76`
  - current atomicity:
    - `writeFileAtomic()`

#### 2. Internal lib files for every processed feed

- `LIB_DIR/<feed>/latest`
  - meaning: binary latest-set snapshot for comparisons/query
  - writer: `finalize()`
  - evidence: `pkg/engine/finalize.go:30-38`
  - current atomicity:
    - `writeBinaryPath()` → `writeFileAtomic()`

- `LIB_DIR/<feed>/history.csv`
  - meaning: append-only full history ledger
  - writer: `finalize()`
  - evidence: `pkg/engine/finalize.go:59-66`
  - current atomicity:
    - header creation is atomic
    - appended rows are **append + fsync**, not temp+rename

- `HISTORY_DIR/<feed>/<unix>.set`
  - meaning: primary-observation history snapshot used by history derivatives
  - writer: `keepHistorySnapshot()`
  - evidence: `pkg/engine/finalize.go:100-131`
  - current atomicity:
    - `writeFileAtomic()`
  - note:
    - only for non-`internal://` feeds
    - merges/history-derivatives are not snapshotted here

#### 3. Retention files for every processed feed

- `LIB_DIR/<feed>/changesets.csv`
  - meaning: append-only added/removed ledger
  - writer: `updateRetention()`
  - evidence: `pkg/engine/retention.go:67-75`
  - current atomicity:
    - header normalization is atomic
    - appended rows are **append + fsync**

- `LIB_DIR/<feed>/new/<unix>`
  - meaning: "added this run" binary set tracked for retention aging
  - writer: `updateRetention()`
  - evidence: `pkg/engine/retention.go:77-80`
  - current atomicity:
    - `writeBinaryPath()` → atomic

- `LIB_DIR/<feed>/retention.csv`
  - meaning: append-only removal/age ledger
  - writer: `updateRetention()`
  - evidence: `pkg/engine/retention.go:82-123`
  - current atomicity:
    - header creation atomic
    - appended rows are **append + fsync**

- `LIB_DIR/<feed>/retention.json`
  - meaning: internal retention summary JSON
  - writer: `updateRetention()`
  - evidence: `pkg/engine/retention.go:136-147`
  - current atomicity:
    - `writeFileAtomic()`

- `LIB_DIR/<feed>/histogram`
  - meaning: bash-compatible histogram cache
  - writer: `writeRetentionHistogramCache()`
  - evidence: `pkg/engine/retention.go:230-239`
  - current atomicity:
    - `writeFileAtomic()`

#### 4. Public/web per-feed artifacts

- `WEB_DIR/<feed>.json`
  - meaning: metadata/spec page payload
  - writer: `writeMetadataFiles()`
  - evidence: `pkg/engine/metadata.go:58-67`
  - current atomicity:
    - staged under web batch dir, then renamed into live dir

- `WEB_DIR/<feed>_history.csv`
  - writer: `writePublicHistoryCSV()`
  - evidence: `pkg/engine/public_series.go:37-47`
  - current atomicity:
    - staged write is atomic
    - publish to live dir is rename from stage dir

- `WEB_DIR/<feed>_changesets.csv`
  - writer: `writePublicChangesetsCSV()`
  - evidence: `pkg/engine/public_series.go:50-65`
  - current atomicity:
    - staged atomic write

- `WEB_DIR/<feed>_retention.json`
  - writer: `writePublicRetentionJSON()`
  - evidence: `pkg/engine/public_series.go:68-85`
  - current atomicity:
    - staged atomic write

- `WEB_DIR/<feed>_comparison.json`
  - writer: `writeComparisonFiles()`
  - evidence: `pkg/engine/output.go:447-480`
  - current atomicity:
    - staged atomic write

- `WEB_DIR/<feed>_insights.json`
  - writer: `writeInsights()`
  - evidence: `pkg/engine/insights.go:46-68`
  - current atomicity:
    - staged atomic write

- `WEB_DIR/<feed>_<geo_provider>.json`
  - writer: `writeCountryComparisonFiles()`
  - evidence: `pkg/engine/geoloc.go:333-345`
  - current atomicity:
    - staged atomic write

- `WEB_DIR/<feed>_asn_<asn_provider>.json`
  - writer: `writeASNComparisonFiles()`
  - evidence: `pkg/engine/asn.go:380-388`
  - current atomicity:
    - staged atomic write

- `WEB_DIR/<feed>_bogons_<bogon_provider>.json`
  - writer: `writeBogonComparisonFiles()`
  - evidence: `pkg/engine/bogons.go:241-248`
  - current atomicity:
    - staged atomic write

#### 5. Public/web global files (not per-feed, but same publish batch)

- `WEB_DIR/index.json`
- `WEB_DIR/all-ipsets.json`
- sitemap / robots files
  - writer: `writeMetadataFiles()`
  - evidence: `pkg/engine/metadata.go:22-24`, `pkg/engine/metadata.go:97-114`
  - current atomicity:
    - staged write + staged publish

#### 6. Web ipset mirror files

- `WEB_DIR_FOR_IPSETS/<feed>.ipset|.netset`
  - writer: `copyUpdatedIPSetsToWeb()`
  - evidence: `pkg/engine/web_ipsets.go:14-48`
  - current atomicity:
    - explicit `dst + ".new"` then rename to final

#### 7. Provider/database download artifacts (not regular feed outputs, but important)

- Geo providers:
  - `BASE_DIR/<provider>.source`
  - evidence: `pkg/engine/geoloc.go:78-83`, `pkg/engine/geoloc.go:117-131`
  - current atomicity:
    - download temp → direct rename to final `.source`

- ASN providers:
  - `LIB_DIR/asn/<provider>/source`
  - `LIB_DIR/asn/<provider>/<format-specific data file>`
  - evidence: `pkg/engine/asn.go:99-107`, `pkg/engine/asn.go:161-185`
  - current atomicity:
    - archive download temp → direct rename to final `source`
    - extracted data file atomicity depends on format extractor

#### 8. Publish semantics today

- Per-feed public files are first written into a staging dir under `WEB_DIR`
  - evidence: `pkg/engine/web_batch.go:14-23`
- Publish then renames each staged file into the live dir one by one
  - evidence: `pkg/engine/web_batch.go:33-59`
- Important consequence:
  - individual file replacement is atomic
  - **multi-file publish is not globally atomic**
  - a crash can leave a mixed generation across files, even though each single file is complete

### Memory story — what streams vs. what materializes

Good news (already streaming via mmap where it matters):

- `pkg/iprange/fileset_mmap.go` — Linux/Darwin mmap path. `unix.Mmap` + on-demand Range decoding. `pread` fallback at `fileset_pread.go`.
- `Engine.openLatestSet` goes through `iprange.OpenFileSet` → returns a streaming `FileSet` handle.
- All comparison code in `pkg/iprange/iter_ops.go` (`OverlapCountIter`, `IntersectIter`, `UnionIter`) walks the `FileSet` via streaming two-pointer iterators.
- `writeCountryComparisonFiles` iterates `countrySets` one provider at a time — the OTHER providers are not held in memory simultaneously.

Bad news (memory hot-spots that should be revisited during the rewrite):

- `loadBogonSources` holds ALL bogon providers in memory concurrently for the duration of Phase 3. Not per-provider like geo/asn.
- `buildBogonUnion` materializes the union as one in-memory `*iprange.IPSet`. Size is small today (~6 contributors) but grows linearly.
- `historyUnion` in `finalize.go:142` — loads snapshot files for the window. Uses mmap-backed FileSet per file, streams the union into a new `*iprange.IPSet` — should be memory-bounded but worth spot-checking.
- The comparison matrix writer (`writeComparisonFiles`) processes pairs one at a time — streaming.

## Decisions

### Already decided by Costa on 2026-04-19

1. Fetch/change detection and processing must be split into **two independent loops**.
2. Fetch/change detection must remain **disk-backed**.
3. Fetch loop parallelism defaults to **5 workers** and must be configurable.
4. Processing cadence defaults to **10 minutes** and must be configurable.
5. Slow downloads or timeouts must not block the processing pipeline.
6. Derivatives are split into two kinds:
   - **history derivatives**
     - depend on one feed only
     - enter the waiting queue together with their parent source
     - must be processed strictly **after** their parent
   - **merges**
     - may depend on multiple feeds
     - must not be trigger-driven from source updates
     - run on their own fixed cadence
7. Dirty-feed rule:
   - once a feed is queued or being processed, it is **dirty**
   - while dirty, no further download should happen for that feed
   - a refresh that becomes due while dirty is **postponed**, not lost
   - once dirty clears, the postponed refresh should run immediately
8. Source-feed download durability contract:
   - write download to `{file}.tmp`
   - on successful download completion, rename to `{file}.new`
   - `{file}.new` is a durable startup-recovery marker only
   - once processing completes successfully, promote to the source-of-truth file without suffix
9. Restart contract:
   - discard lingering `.tmp` files
   - if `{file}.new` exists on restart, requeue/reprocess that feed
10. Non-source outputs should also use `.tmp` writes before promotion to final paths.
11. Runtime dirty-state is **in-memory**, not file-based:
   - dirty feeds stay in the in-memory schedule/ordering model
   - while dirty they are skipped by fetch
   - once dirty clears, if they are already overdue they run immediately
   - no extra on-disk runtime marker is required beyond `.new` for startup recovery
12. Merge ordering/trigger policy:
   - merges can depend on anything
   - merges are cadence-driven only
   - merges use the normal per-feed scheduling model (`frequency`)
   - merges are treated as a normal "download/update-detection" kind in the unified pipeline
   - in a processing batch the order is:
     1. normal feeds
     2. history derivatives
     3. merges
   - merges within the batch are ordered by input/source count
   - known wrong-order exceptions are acceptable by design

### Older decisions that still align

1. **`history: [...]` is YAML sugar**. The config loader expands it into standalone source entries at load time. Curators keep the short syntax; the in-memory config has flat sources only.

2. **"Is this feed updated?" is answered at download time**. The downloader is the authority. Non-updated feeds are rejected in the download queue; their processing pipeline never runs for that tick.
   - For plain HTTP: standard If-Modified-Since / ETag / content-hash.
   - For history derivatives: parent update is the trigger, not an independent fetch loop.
   - For merges: their own fixed-cadence run decides whether outputs changed.

3. **History derivatives follow the parent in the same processing pass**. This preserves ordering without introducing a general dependency graph into the fetch queue.

4. **One PR, fully tested**. No incremental shipping.

5. **Avoid solving multi-input dependency graphs in the trigger queue**. Merges move to fixed cadence specifically to eliminate this complexity from the queue.

### Decisions made before implementation

1. **What exactly enters the changed-feeds queue?**
   - A. plain sources only
   - B. plain sources plus ASN/Geo downloaded artifacts
   - C. every downloaded artifact in the unified fetch loop
   - **Decision: Costa chose C**
   - The queue is fed by the single unified download process.

2. **What happens if a feed changes multiple times before the next processing tick?**
   - A. coalesce to one queue entry per feed
   - B. keep every change event separately
   - **Decision: implied by Costa's dirty-feed rule → A**
   - While dirty, no new download occurs; the missed refresh is postponed, not queued as another independent event.

3. **Should the changed-feeds queue survive daemon restart?**
   - A. yes, persist it on disk
   - B. no, rebuild from scratch after restart
   - **Decision: implied by Costa's `.new` startup-recovery rule → A**
   - Durable `.new` markers are the restart-survival mechanism.

4. **How should manual actions behave?**
   - A. keep both actions:
     - `recheck`: go through fetch now, then queue for processing even if the downloader reports `same` / `not_modified`
     - `reprocess`: skip fetch, queue immediate processing from the currently committed local source
   - B. collapse everything into one forced action
   - **Decision: Costa chose A**
   - Automatic fetches queue only real changes.
   - Manual actions may force queue admission even when there is no detected update.

5. **Does the fetch loop still obey each feed's existing `frequency` and backoff rules?**
   - A. yes, keep per-feed cadence/backoff in the fetch loop
   - B. no, introduce a new global polling cadence for fetching too
   - **Decision: Costa chose A**
   - Keep per-feed cadence/backoff in the fetch loop.

6. **Do `.new` markers apply only to source feeds, or also to database feeds (`use:[asn]`, `use:[geoip]`)?**
   - A. source feeds only
   - B. source feeds plus database feeds
   - **Decision: Costa chose B**
   - `.new` startup-recovery semantics apply to all downloaded artifacts.

## Design (target architecture)

### Requirement clarity check

The requirement is **mostly clear at the architectural level**:

- separate fetch from processing
- make fetch disk-backed and parallel
- make processing queue-based and cadence-based
- do not let slow downloads delay processing

What is **not fully specified yet** is queue semantics:

- what exactly is queued
- whether the queue is durable
- how repeated changes are coalesced
- how manual/admin actions interact with the queue
- how ASN/Geo database feeds fit into the split model
- whether fetch still honors per-feed `frequency` / failure-backoff semantics
- whether `.new` semantics apply to database feeds too
- whether cadence-driven merges should be allowed to consume stale same-batch merge inputs by design

Those are implementation-shaping decisions, not cosmetic details.

### Feed identity

One struct: `config.Source`. Fields added/removed:

- **Removed**: `History []int` (moves into the YAML-loader sugar; no longer stored on the in-memory struct).
- **Removed from top-level config**: the `merges:` block. Either fully deleted or rewritten to `sources:` entries with `url: internal://merge?inputs=a,b,c` at the loader level.
- **Added** (implied by design, not new fields — they already exist): `URL`, `Frequency`, all the rest.

There is **no** `Source.Parent`, no `Source.Kind`, no `Source.IsDerivative`. The URL scheme carries all routing information.

### YAML sugar expansion

At config load time, after parsing the YAML into the raw struct:

1. For every source whose YAML had `history: [m1, m2, m3]`:
   - Emit N new `Source` entries:
     ```go
     {Name: "viriback_1d",  URL: "internal://retention_window?parent=viriback&minutes=1440",  Frequency: 0, ...all other fields copied from parent}
     {Name: "viriback_7d",  URL: "internal://retention_window?parent=viriback&minutes=10080", Frequency: 0, ...}
     {Name: "viriback_30d", URL: "internal://retention_window?parent=viriback&minutes=43200", Frequency: 0, ...}
     ```
   - The parent's `history` field is consumed and dropped.
   - Label for the variant is `"<parent-label> (1d window)"` or similar; info text carries the window description.

2. For every merge in the legacy `merges:` block:
   - Emit a `Source` entry:
     ```go
     {Name: "firehol_level1", URL: "internal://merge?inputs=dshield,feodo,fullbogons,spamhaus_drop", Frequency: 0, Category: "...", ...}
     ```
   - The `merges:` block is emptied.

3. Validate: check for name collisions, cycles in the dependency graph (a source whose inputs transitively include itself), unknown parent references.

4. Build the reverse index: `map[string][]string` from input name → list of dependent source names. Store on the engine struct, used by the dynamic injection step.

**Note**: I'll support both the new form (standalone sources with `internal://` URLs) AND the legacy `history: [...]` / `merges:` form in the loader, so existing YAML keeps working unchanged. The physical YAML doesn't need to be rewritten.

### New downloader providers

Add to `pkg/downloader/internal.go`:

1. **`retention_window`** — params: `parent`, `minutes`.
   - Check `max(mtime of lib/{parent}/history/*.set where ts > now - minutes)` vs. our last successful update time.
   - If newer → stream the union of those snapshots, emit a binary-set payload, return as "updated".
   - Else → return "not modified" (304-equivalent).

2. **`merge`** — params: `inputs` (comma-separated source names).
   - Check `max(mtime of data/{input}.ipset|netset for input in inputs)` vs. last update.
   - If newer → concatenate the input files, return as "updated".
   - Else → "not modified".

Both are implemented as `downloader.InternalProvider` (the existing interface used by `rfc_reserved_baseline`). They integrate with the existing prefetch pool and cache-entry update path — zero changes to `processConcreteSource`.

### The batch pipeline

`RunOnce` becomes a linear sequence of explicit queues. Each queue is a function with a clear input and output.

```
RunOnce(ctx, opts) {
    applyRenamesAndDeletes()

    // Queue 1: derive who needs updating.
    initial := DeriveBatch(cfg, state, now, opts)

    // Queue 2: download + parse + finalize.
    //
    // Iteratively processes the work queue. Each worker:
    //   - asks the downloader "is this feed updated?"
    //   - if yes: parse, finalize, append retention snapshot, mark updated
    //   - if yes AND this feed has dependents in the reverse index:
    //       push dependents into the work queue
    //   - if no: mark skipped
    //
    // Bounded by MaxProcessingWorkers. Handles cycles via a seen-set
    // (each feed processed at most once per batch) and panics if the
    // reverse index contains a cycle (which should have been caught at
    // config load time).
    updated := DownloadQueue(ctx, initial)

    // Queue 3: retention snapshot append.
    //
    // For each feed in `updated`, append a new binary snapshot to
    // lib/{name}/history/{ts}.set and trim the directory to the
    // longest window any derivative references.
    //
    // This is cheap (one binary write per feed). It runs AFTER the
    // download queue finishes because the download queue for retention
    // derivatives reads lib/{parent}/history/; we don't want a race
    // between a parent appending its snapshot and a derivative reading it.
    //
    // WAIT — this is the crux. See "Retention timing" below.
    RunRetention(updated)

    // Queue 4: pairwise overlap (only pairs where either side is in `updated`).
    RunOverlaps(updated)

    // Queue 5: ASN per provider.
    //
    // Load provider #1 → write per-feed ASN JSON for every feed in `updated`
    //                    (and every feed if any ASN provider updated this tick) →
    //                    unload provider #1 → load #2 → ...
    //
    // One provider in memory at a time. Bogon union for the three-bucket
    // split is computed once per provider, not globally held.
    for p in ASNProviders: RunPerProvider(p, updated)

    // Queue 6: Geo per provider.
    for p in GeoProviders: RunPerProvider(p, updated)

    // Queue 7: Insights.
    RunInsights(updated)

    writeMetadataFiles(updated)
    cache.Save()
}
```

### Retention timing — the hard question

Retention derivatives (`viriback_1d`) need to run AFTER the parent appended its new snapshot — otherwise the 1d window is stale.

Two possible orderings inside the batch:

**Option A (simpler, my preference)**: retention-append happens INSIDE the download queue, immediately after a parent feed is finalized. The snapshot is appended BEFORE any dependent is injected. Then dependents are injected and process normally. Retention is not a separate queue; it is a per-feed side-effect that runs during finalize. This matches how it works today (just refactored so the derivative is a separate feed, not inline cloning).

**Option B (Costa's phrasing)**: retention-append is its own queue (Queue 3). But then dependents need to wait until Queue 3 finishes before Queue 2 can inject them. That breaks the "dynamic injection" requirement because the dependents can't be injected into Queue 2 — they'd have to go into a future Queue 2 which doesn't exist.

I recommend **Option A** and will use it unless Costa objects. The retention snapshot append is a single binary write per updated feed (~tens of milliseconds total across all feeds) — making it a separate queue is structural overhead without a memory benefit.

To Costa: does "the retention queue" you described mean a separate queue stage, or just "every updated feed appends its retention snapshot as part of its finalize step"? I suspect the latter.

### Dynamic injection mechanism

At config load time, build:

```go
type Engine struct {
    ...
    // dependents maps an input feed name to the list of feed names
    // whose URL input set contains it. Built once at config load
    // from the `internal://*` sources. Used by the download queue
    // to inject a feed's derivatives into the work queue as soon
    // as the parent finishes updating.
    dependents map[string][]string

## Plan

1. Define the new runtime model precisely:
   - fetch loop responsibilities
   - queued item model
   - processing loop cadence
   - manual/admin behavior
2. Introduce a durable changed-feeds queue abstraction.
3. Move plain-source download/change detection out of `RunOnce()` into the fetch loop.
4. Change the scheduler loop so it drains queued changed feeds every `processing_interval_minutes` instead of deriving due feeds and calling `RunOnce()` directly.
5. Preserve dependency injection inside the processing phase for internal derivatives and merges.
6. Keep heavy-block fan-out semantics correct for overlaps, databases, and insights.
7. Add restart-safety, queue-coalescing, and admin visibility tests.

## Implied decisions

- The fetch loop must update enough cache state to avoid re-fetch storms after restart.
- The processing loop consumes a stable snapshot of the processing queue at tick start.
- Feeds that become dirty while a processing batch is already running wait for the next processing tick.
- The admin API will need separate visibility for:
  - feeds waiting to be downloaded
  - feeds being downloaded now
  - feeds waiting to be processed
  - feeds being processed now, including their active phase
- No other lists are needed above the all-feeds table.
- The all-feeds table itself must remain as it is today.

## Testing requirements

- Unit tests for queue coalescing and queue persistence.
- Unit tests for fetch-loop behavior when remote body is:
  - `304 not modified`
  - `same` body
  - changed body
  - timeout / failure
- Integration tests proving:
  - slow downloads do not delay processing of already-queued changes
  - repeated changes before the next tick do not create duplicate processing
  - restart preserves queued work if durability is chosen
  - derivatives still process in the correct order after a parent changes
- Regression tests for admin/scheduler status so UI reflects:
  - feeds waiting to be downloaded
  - feeds being downloaded now
  - feeds waiting to be processed
  - feeds being processed now, with phases

## Documentation updates required

- `AGENTS.md` runtime/pipeline section
- methodology/admin docs for the new queue semantics
- config documentation for:
  - fetch worker count
  - processing interval
  - queue persistence/coalescing behavior
- admin UI documentation for the four operator lists above the table

## Remaining decisions before implementation

- None at the moment.
- The spec is now clear enough to start implementation.

The download queue is a channel plus a worker pool:

```go
work := make(chan string, len(initial))
for _, name := range initial { work <- name }

processed := make(map[string]bool)
var mu sync.Mutex
var updated []string

for i := 0; i < workers; i++ {
    go worker(ctx, work, &updated, processed, &mu)
}
```

Each worker:

```go
for name := range work {
    mu.Lock()
    if processed[name] { mu.Unlock(); continue }
    processed[name] = true
    mu.Unlock()

    result := e.processSource(ctx, cfg.Sources[name], opts)
    if result.Updated {
        mu.Lock()
        updated = append(updated, name)
        for _, dep := range e.dependents[name] {
            if !processed[dep] { work <- dep }
        }
        mu.Unlock()
    }
}
```

Cycle protection: `processed[name] = true` is set BEFORE processing, so a cycle A→B→A cannot re-inject A. Combined with load-time cycle detection, this is double-safe.

Termination: when the `work` channel is empty AND all workers are idle, close the channel. This requires a small WaitGroup dance — standard Go pattern.

### Fan-out simplification

`targetFeedsForFanOut` (my band-aid) collapses to:

```go
func targetFeedsForFanOut(_, updatedNames, outputNames []string) []string {
    if len(updatedNames) == 0 {
        return outputNames
    }
    // Filter updatedNames against outputNames — retain only names that
    // have on-disk state. The provider-update early return is unchanged.
    // No retention/split expansion. No reverse lookups. No helpers.
}
```

The `sourceForOutputName`, `outputNamesForSource`, and `stripRetentionLabel` helpers are deleted.

The provider-update-forces-full-fan-out branch stays — it's still correct for the "a new geo database arrived" case.

## Implementation plan (one PR, phased commits inside the PR for reviewability)

### Commit 1 — revert the band-aid fan-out fix

- Revert `targetFeedsForFanOut` to its pre-2026-04-09 form.
- Delete `sourceForOutputName`, `outputNamesForSource`, `stripRetentionLabel`.
- Delete the regression tests I added in `bogons_test.go` for retention/split fan-out expansion (they'll be replaced by the new integration tests in later commits).
- Make the empty-pages bug visible again in tests. Add a xfail test that documents the bug so we can close it in Commit N.

### Commit 2 — YAML loader sugar expansion

- Extend `pkg/config/config.go` YAML loader:
  - After parsing, walk sources with `history: [...]` and expand into standalone entries with `internal://retention_window?parent=...&minutes=...`.
  - Walk `cfg.Merges` and expand into sources with `internal://merge?inputs=...`. Delete the `Merges` map entry.
  - Keep `Source.History` as an IGNORED field (present for YAML parsing, unused after expansion) so the YAML files don't break. Or delete the field and switch the YAML tag to a temporary one. I'll decide after reading the struct tags.
- New helpers: `ExpandDerivatives(cfg)`, `ExpandMerges(cfg)`.
- Validate no name collisions after expansion.
- Build the `dependents` reverse index.
- Cycle detection via DFS on the reverse index. Fail loud with the cycle participants.
- Update `cfg.Sources` count-based tests — they grow by ~30 sources (retention variants for ~10 sources × 3 windows each) + ~10 merges = ~40 new entries.

### Commit 3 — internal downloader providers

- `pkg/downloader/internal.go`: register `retention_window` and `merge` provider kinds.
- `retention_window`: parses URL params, walks `lib/{parent}/history/` using mmap-backed FileSets, streams the union, returns "updated" or "not modified" based on max input mtime vs. last-update time from the cache entry.
- `merge`: concatenates input `.ipset|.netset` files, returns "updated" or "not modified".
- Both return the same `*fetchOutcome` shape as HTTP downloaders — zero changes to callers.
- Unit tests for each: happy path, "not modified" path, missing parent, empty input directory, dependency on a feed that failed.

### Commit 4 — delete `processMerge` and the Phase 2 merge pool

- Delete `pkg/engine/process.go:499` (`processMerge`) and the merge loop in `run.go:130-176`.
- Merges are now just sources with `internal://merge?...` URLs; they flow through the same Phase 1 worker pool.
- Delete `cfg.Merges` reads throughout the code (engine, scheduler, admin API, insights, web, tests).
- Update `configuredNames` to drop the merge walk.

### Commit 5 — delete `updateHistoryVariants` and the inline retention cloning

- Delete `pkg/engine/finalize.go:updateHistoryVariants` and its call site in `process.go:386`.
- Retention derivatives are now standalone sources (post Commit 2) with `internal://retention_window` URLs (post Commit 3). They flow through the same Phase 1 worker pool.
- The retention snapshot append (`keepHistorySnapshot`) stays — it is called inside `finalize` and is the mechanism the `retention_window` downloader reads from.

### Commit 6 — rewrite `RunOnce` as an explicit queue pipeline

- Refactor `RunOnce` into the queue shape in the Design section.
- Replace the two-phase (sources, then merges) worker pool with a single dynamic-injection worker pool.
- Implement `DeriveBatch`, `DownloadQueue`, `RunOverlaps`, `RunPerProvider`, `RunInsights`.
- Each queue is a separate function in a new `pkg/engine/pipeline.go` file (keeping `run.go` slim).
- Shrink `targetFeedsForFanOut` to the simple form. Delete the expansion helpers.

### Commit 7 — scheduler uniformity

- `pkg/scheduler/scheduler.go:BuildSnapshot`: drop the `cfg.Merges` walk. All feeds come from `cfg.Sources`.
- Retention and merge derivatives (now in `cfg.Sources`) get scheduler items with `frequency: 0` → "static, never expires". The scheduler only processes them when the download queue's dynamic-injection mechanism picks them up.
- Verify the admin `/api/v1/admin/feeds` endpoint exposes the new derivatives uniformly.

### Commit 8 — migrate tests

- `pkg/config/catalog_verify_test.go`: update `TestCatalogExpectedCounts`, `TestCatalogSourcesComplete` with the post-expansion source counts. Add explicit assertions for the existence of retention variants and merges as `cfg.Sources` entries.
- `pkg/config/config_coverage_test.go`, `config_test.go`, `pkg/processor/processor_test.go`: same count updates.
- `pkg/engine/bogons_test.go`: drop the retention/split fan-out test cases added in Commit 1's revert.
- New integration test: spin up an `Engine` with a tiny test config (1 plain source, 1 retention variant, 1 merge). Run one batch. Verify:
  - All three appear in `report.Updated`.
  - All three have their per-provider JSONs written (geo, asn, bogons, comparison, insights).
  - The retention variant's data is the union of the parent's history snapshots.
  - The merge's data is the union of its inputs.
- New unit test: the `dependents` reverse index rejects cycles at config load time.
- New unit test: dynamic injection runs a dependent in the same batch after the parent finishes.
- New stress test: a 3-level chain (A → A_1d → top10_of(A_1d)) converges in one batch.

### Commit 9 — documentation

- Update `AGENTS.md`:
  - Replace any references to `cfg.Merges` or retention variants as second-class concepts.
  - Add a new section: "Pipeline stages and queue semantics" describing the new batch shape.
  - Update the architecture diagram (if any) and the list of internal URL schemes.
- Update the memory notes about the license pass-through rule and organizations category (these are unchanged — just make sure they're still current).
- Update `CLAUDE.md` (which is a symlink to AGENTS.md, so this is the same file).

### Commit 10 — deploy + verify

- `./install.sh` with the systemd env vars properly set.
- Force a clean cache pass: delete `cache/.cache.json` and let the daemon rebuild. (The cache format didn't change so this is optional but clean.)
- Verify:
  - `curl http://localhost:18888/api/v1/sets` returns all feeds including retention variants as first-class entries.
  - `curl http://localhost:18888/api/v1/sets/viriback_1d` returns full metadata including `geo`, `asn` fields.
  - `viriback_1d_*_country.json`, `viriback_1d_asn_*.json`, `viriback_1d_bogons_*.json` exist and have non-empty content.
  - Open the `/viriback_1d` page in the UI — no empty sections.
  - Same for `firehol_level1` and its 3 siblings.

## Testing requirements

- Unit tests: each new internal downloader, each new loader expansion function, each new pipeline queue function, the reverse-index cycle detector, the dynamic-injection worker pool with a mock source processor.
- Integration tests: full batch with a tiny hand-built config covering every feed kind (plain, retention derivative, merge, merge-of-derivative, derivative-of-merge).
- Regression tests: all existing catalog verification tests must pass after expected count updates. `TestCatalogSourcesComplete` must list the new retention variants and merges explicitly so an accidental regression fails the test.
- Race tests: `go test -race ./...` must pass on the new worker pool.
- Real-world smoke test: run the daemon for one tick, verify the 5 new Maltrail feeds plus 3 existing high-retention feeds all produce full per-variant JSON output.

## Documentation updates

- `AGENTS.md` — new pipeline section, delete references to `cfg.Merges`, update any diagrams.
- `pkg/web/static/methodology/*.md` — spot-check that nothing references "retention variant" or "merge" as special concepts. If it does, rewrite to describe them as normal feeds.
- `configs/firehol.yaml` — no changes required (YAML sugar is loader-level). Optionally add a comment at the `history:` fields explaining that they expand into standalone feeds at load time.
- Memory file `MEMORY.md` — add a new feedback memory documenting this design decision and the rationale so future sessions don't re-debate it.

## Implied decisions (not asked, proposed)

1. **Option A for retention timing** (append snapshot inside finalize, not as a separate queue) — see Design section.
2. **Keep `history: [...]` as YAML sugar** — do not rewrite the YAML file. The loader handles expansion.
3. **Delete `targetFeedsForFanOut` expansion helpers** in the same PR (Commit 1 reverts, Commit 6 wouldn't need them).
4. **Build a reverse index at load time** for dynamic injection, with cycle detection in the same pass.
5. **Workers in the download queue pool** is still `MaxProcessingWorkers` (default 2) — no change.
6. **Retention derivative naming convention** stays `{parent}_{window}` (`viriback_1d`, etc.) — matches current expectations from the web UI and blocklist-ipsets git repo.
7. **Merges keep their current names** (`firehol_level1`, etc.) — no renames.
8. **Bogons refactor (load one-at-a-time like ASN/Geo)** — out of scope for this PR. Flag for follow-up.

## Risks and watch-outs

- **Cache format**: `cache.Entry.HistoryMinutes` field is written today. After this change, retention variants have their OWN cache entries and the parent's `HistoryMinutes` is obsolete. I'll leave the field in the struct (tolerant loader) so old cache files keep working.
- **Scheduler snapshot shape change**: merges no longer appear as a separate `kind: "merge"`. The admin UI and any consumer that groups by kind will see a slightly different picture. Check `pkg/web/admin.go` for grouping logic.
- **`frequency: 0` meaning**: today it means "static source, never expires, run once". For retention derivatives that's the right semantic — they never have a cadence, they run when inputs change. Double-check the scheduler's `nextDue` handles this without immediately re-firing.
- **Pairwise comparison matrix growth**: the matrix size is `O(N^2)` where N is the number of live feeds. Today N=163. After this change (existing retention variants + merges were already counted via `configuredNames` for output purposes, but are now distinct sources) N stays approximately the same — no size change. Good.
- **Insights rules**: they already operate per-feed. No change needed as long as the new derivatives have cache entries with the right shape. Spot-check.
- **Admin API endpoints**: `/api/v1/admin/feeds/{name}/recheck` and `/reprocess` currently work on sources. They'll now work on retention variants and merges too (as a bonus). Verify no name-based special-casing remains.
- **`enabled_by_all` field**: the parent source has it. Retention derivatives should inherit it (so enabling all also enables the variants). Handled by the loader expansion.

## Decisions (confirmed by Costa on 2026-04-09)

1. **Q1 — Retention timing**: **Option A** — retention snapshot append happens inside `finalize` as a per-feed side-effect. Parent finalizes, snapshot lands on disk, then dependents are dynamically injected into the download queue. No separate retention queue stage.
2. **Q2 — Merges in scope**: **Yes**. Merges and retention derivatives both get unified in this PR.
3. **Q3 — Keep `merges:` YAML block as curator sugar**: **Yes**. Curators keep the short syntax; the loader flattens both `history: [...]` and `merges:` into standalone `Source` entries with `internal://` URLs.
4. **Q4 — Cache file migration**: **Delete** `cache/.cache.json` once after deploy. Clean rebuild.

## Feasibility findings (2026-04-09) — adjustments to the plan

Feasibility verdict: **feasible as specified, no blockers**. Three adjustments:

1. **Extend `InternalProvider`** from `func() ([]byte, error)` (`pkg/downloader/internal.go:29`) to an interface that can check input mtimes and return "not modified" without regenerating output. Necessary for `retention_window` to avoid rebuilding the union on every tick when no new snapshots exist. Backwards compatible — the existing `rfc_reserved_baseline` wraps through an adapter.
2. **Add `Source.DerivedFrom []string`** for uniform admin API exposure (replaces the `Kind: "merge"` + `MergeSources []string` special-casing in `pkg/web/admin.go:404-418`).
3. **Delete `cache.Entry.HistoryMinutes`** — populated from `src.History` at `pkg/engine/process.go:208`, informational only, nothing else reads it. After expansion, parents have empty `History` so this field becomes vestigial.

Loader hook point confirmed: `Config.UnmarshalYAML` at `pkg/config/config.go:44-74`, expansion runs after the shadow decode. Downloader routing confirmed: `Client.Fetch` at `pkg/downloader/downloader.go:118-119` already dispatches `internal://` URLs via `IsInternalURL` + `fetchInternal` — zero engine changes needed. URL scheme validation at `pkg/config/validate.go:164` already allows `internal://`. Test pattern confirmed: `TestRunOnceAndQuery` at `pkg/engine/engine_test.go:18-96` uses httptest + YAML on disk.

Revised commit count: 11 (one more than planned — split "new internal downloaders" into "extend interface" (C2) and "add providers" (C4) to keep each commit small).

## Progress

- 2026-04-09: design approved by Costa, feasibility pass done, implementation complete through deploy.
- ✅ Commit 1 (revert band-aid) — reverted `targetFeedsForFanOut` to its pre-bandaid form; deleted helpers + regression tests.
- ✅ Commit 2 (extend InternalProvider interface) — added `referencePath` arg + `ErrInternalNotModified` sentinel; rfc_reserved adapter.
- ✅ Commit 3 (infrastructure: DerivedFrom field, Dependents(), DetectCycles()) — new `pkg/config/dependents.go` with tests; field added to Source struct.
- ✅ Commit 4 (new internal downloaders) — `BuildRetentionWindow` and `BuildMerge` in `pkg/engine/provider_retention.go` and `provider_merge.go` with unit tests covering the happy path, ErrInternalNotModified short-circuit, empty inputs, cycle detection, etc.
- ✅ Commit 5 (loader expansion + provider registration + dynamic-injection worker pool) — bundled with Commits 6+7 because they are not independently shippable:
  - `pkg/config/expand.go` with `ExpandDerivatives` called from `LoadYAML` after shadow decode.
  - `registerInternalProviders` in `pkg/engine/provider_registry.go` called from `engine.New`.
  - `RunOnce` Phase 1 rewritten as a dynamic-injection worker pool driven by `cfg.Dependents()`.
  - `prefetchSources` skips internal:// URLs.
  - `processConcreteSource` calls `keepHistorySnapshot` unconditionally for non-internal sources.
  - `BuildRetentionWindow` produces text CIDR output so it flows through `parseProcessedFile` without a binary-format detection race.
  - Closures in `registerInternalProviders` capture the `*Engine` pointer so `e.now` overrides in tests propagate to the retention provider.
  - `AcceptEmpty: true` set on retention variants and merges during expansion — empty output on the first tick (no snapshots yet) is legitimate.
- ✅ Commit 6 (delete processMerge and Phase 2 merge pool) — deleted `processMerge`, `recordMergeOutcome`, the Phase 2 worker pool in `RunOnce`. Merges flow through the normal Phase 1 pool via dynamic injection when their inputs update.
- ✅ Commit 7 (delete updateHistoryVariants, historyUnion, cleanupHistory, sortedMergeNames, sortedMerges) — retention snapshot append moved to an inline call in `processConcreteSource`. Snapshot cleanup is a follow-up (see Risks below).
- ✅ Commit 8 (scheduler uniformity) — dropped `cfg.Merges` walk in `BuildSnapshot`; `sortedMerges` and `mergeDue` are dead code (kept in the file only because `scheduler_test.go` still references `mergeDue` directly — safe to delete in a follow-up).
- ✅ Commit 9 (admin API) — `buildMergeFeed` deleted; `buildSourceFeed` recognises derivatives via URL scheme, copies `DerivedFrom` into the `adminFeed` response, sets `Kind` to `"merge"` or `"retention"` based on `internal://` scheme.
- ✅ Commit 10 (test migration) — source counts updated from 163 → 215 in `config_coverage_test.go`, `config_test.go`, `catalog_verify_test.go`, `processor_test.go`. `TestCatalogSourcesComplete` expanded with 52 new retention + merge names. `TestCatalogFrequenciesArePositive` updated to skip internal:// sources. `TestLoadDirectoryMergesSupplementalSources` updated to check `cfg.Sources["combined"]` with an `internal://merge` URL. `TestCatalogMergeSourceCountsMatchBash` + `TestCatalogSpecificFireholLevels` check `DerivedFrom` instead of `merge.Sources`. `TestHistoryUsesObservationTimeInsteadOfSourceTimestamp` passes without changes — the retention_window provider respects `e.now` via the closure-captured engine pointer. New integration tests for `BuildRetentionWindow` and `BuildMerge` in their own `_test.go` files.
- ⬜ Commit 11 (docs: AGENTS.md pipeline section + memory file note) — pending (next step).
- ✅ Commit 12 (deploy + verify) — `./install.sh` + force-copied `configs/firehol.yaml` → `/opt/update-ipsets/etc/config.yaml` + `rm .cache.json` (per Q4) + `systemctl restart`.

## Verified runtime behavior (2026-04-09 after deploy)

- Daemon logs `sources=215 merges=0` — expansion working.
- First tick processed `stopforumspam`, then `stopforumspam_1d`, `_7d`, `_30d`, `_90d`, `_180d`, `_365d` via dynamic injection (each ran AFTER the parent finished updating).
- `/opt/update-ipsets/web/stopforumspam_180d_*_country.json`, `_asn_*.json`, `_bogons_*.json`, `_comparison.json`, `_insights.json`, `_history.csv` all exist and contain real data — the heavy-block fan-out picks up dynamically-injected names via `report.Updated` naturally.
- `firehol_level1_*` per-provider JSONs exist (merges flow through the same pipeline as plain sources).
- `go test ./...` ✅
- `go test -race ./pkg/engine/... ./pkg/config/...` ✅

## Phase 2 — blockers and integrity check (2026-04-09, Costa's follow-up)

Costa escalated these from "follow-ups" to "blockers" and added a new integrity-check requirement:

### Blocker 1: snapshot cleanup
Without cleanup, `lib/{name}/history/` grows forever. Implementation: compute `retentionMaxWindowByParent map[string]time.Duration` at `engine.New` time by walking sources with `internal://retention_window?parent=X&minutes=N` URLs and recording the max N per parent. Call a new `pruneHistoryOlderThan(name, cutoff)` from `keepHistorySnapshot` after the new snapshot is written. Non-fatal on failure (log and continue).

### Blocker 2: delete `mergeDue` dead code
Delete the function from `pkg/scheduler/scheduler.go` and the test at `pkg/scheduler/scheduler_test.go:122` that references it directly.

### Integrity check at startup + API
New requirement: at startup the system must verify every feed's secondary outputs exist and are at least as recent as the source file. A feed whose source is newer than any dependent JSON (`_*_country.json`, `_asn_*.json`, `_bogons_*.json`, `_comparison.json`, `_insights.json`, `_history.csv`, `{name}.json`) is "dirty" — the pipeline broke mid-processing and the feed must be reprocessed.

Implementation:
- New `pkg/engine/integrity.go` with `Engine.CheckIntegrity() []IntegrityFinding`. Each finding carries `feed`, `source_path`, `source_mtime`, `missing_files []string`, `stale_files []string`, `reason string`.
- Called from the daemon startup AFTER the runner is constructed but BEFORE `runner.Run(ctx)` starts its main loop. Findings are logged at WARN level, then the affected feed names are queued via `runner.TriggerSources(scheduler.PendingAction{Names: ..., Rebuild: true})`.
- New admin API:
  - `GET /api/v1/admin/integrity` — returns `{findings, count}` so the admin UI can display the table.
  - `POST /api/v1/admin/integrity/reprocess` — runs the check, schedules a rebuild for every affected feed, returns `{status, count, names}`.
- Admin UI: new React component showing the findings table with a "reprocess" action. **Deferred** — the JSON API unblocks operators; the UI can follow.

### Naturally solves stale variant files
Feeds with `frequency: 60` still carry pre-refactor variant JSONs. The startup integrity check finds that e.g. `viriback_1d_dbip_country.json` is older than `viriback.ipset` (or the `.ipset` mtime is newer than `viriback_1d.json`'s mtime), queues the affected feeds for reprocess, done.

## Phase 2 progress

- ✅ B1 (snapshot cleanup) — `engine.buildRetentionMaxWindow` precomputes the longest window per parent; `keepHistorySnapshot` calls `pruneHistoryOlderThan` non-fatally after each new snapshot.
- ✅ B2 (delete mergeDue + test) — function and test deleted; only survive as comments explaining why.
- ✅ B3 (integrity check: Engine method) — `pkg/engine/integrity.go` with `CheckIntegrity() []IntegrityFinding`. Catches missing outputs, stale secondaries (source newer than secondaries), pre-2000 sentinel mtimes, and uses a 60-second in-flight tolerance to suppress false positives during active processing. 7 unit tests.
- ✅ B4 (integrity check: admin API) — `GET /api/v1/admin/integrity` (returns findings), `POST /api/v1/admin/integrity/reprocess` (schedules every affected feed with Rebuild: true).
- ✅ B5 (integrity check: daemon startup call + auto-reprocess) — `pkg/web/server.go:Run` calls `CheckIntegrity` before `runner.Run`, logs findings at WARN, queues affected feeds via `TriggerSources(PendingAction{Rebuild: true})`.
- ✅ B6 (deploy + verify) — `./install.sh` + restart, integrity check caught 22 stale/missing feeds at startup (from the pre-refactor sentinel-time breakage), auto-queued them, reprocess populated them correctly. Final state: only 2 legit findings (dshield + griffinguard) which were real broken pipelines from earlier ticks.

### Additional fixes discovered while implementing Phase 2

- **InternalSentinelTime leakage** — `fetchInternal` was stamping ALL internal source bodies with `InternalSentinelTime` (Unix epoch). This propagated through `finalize → touchFileAt` to the `.ipset` file, leaving every retention variant and merge stuck at 1970 on disk. Fixed by using `time.Now().UTC()` for both the tmp-file Chtimes AND the `Result.ModifiedTime` on the StatusOK AND StatusSame branches.
- **Retention variants inherited parent's processor pipeline** — e.g. `viriback_1d` ran `csv_column(index=3)` on the CIDR-text output of `BuildRetentionWindow`, stripping everything to 0 entries. Fixed in `config.ExpandDerivatives` by replacing the variant's processor with `[passthrough]` + `processor_raw: "cat"`.
- **Scoped rebuild fan-out explosion** — `/api/v1/admin/feeds/{name}/reprocess` was passing `fanOutUpdated = nil` to the heavy block, triggering a global O(N²) fan-out of all 215 feeds for every single-feed reprocess. Fixed: only pass nil when Rebuild AND no Selected (global rebuild).
- **Worker pool's `shouldProcess` bug** — dynamically-injected dependents were being skipped because the worker checked `shouldProcess` (which respects `opts.Selected`). Fixed: enforce `shouldProcess` only at the initial enqueue; dynamic injections always run (they were pulled in because their parent updated, which is an implicit "select").
- **Selected-derivative parent-selected skip** — `POST /admin/feeds/viriback_1d/reprocess` with viriback NOT in selected now correctly includes `viriback_1d` in the initial wave. When BOTH parent and derivative are selected, the derivative is skipped in the initial wave and picked up by injection after the parent finishes (correct ordering).
- **BuildRetentionWindow/BuildMerge refPath semantics** — `refPath` was being treated as "the previous output file" but the engine passes the source *trigger* file (`{BaseDir}/{name}.source`), not the output. Dropped the (broken) not-modified shortcut from both providers; the downloader's hash-based same-body check handles the unchanged case after generation.

## Final verification

- `go test ./...` ✅
- `go test -race ./pkg/engine/... ./pkg/config/... ./pkg/web/...` ✅
- Daemon running on `/opt/update-ipsets`, integrity check shows 2 findings (real broken pipelines from earlier ticks, auto-reprocessed).
- `viriback_1d.ipset` has 7910 unique IPs (was 0 before). Mtime is current time (was 1970 before).
- All secondary files for `viriback_1d` (country, ASN, bogons, comparison, insights, history) exist with current mtimes.
- `firehol_level1.netset` has 4416 CIDR lines; secondaries refreshed.
- Snapshot cleanup works: `lib/{name}/history/` gets pruned based on the longest retention window any derivative declares.

## Spec review findings — 2026-04-20

Verified against `specs/*.md` after external review.

Accepted as real spec gaps or contradictions:

- merge composition currently uses conflicting terminology:
  - `committed processed inputs`
  - `committed local output`
  - `committed local input`
- history derivatives are described in two incompatible ways:
  - downloader-composed from new parent prepared body + derivative history
  - ordered as if they read fresh committed parent outputs during processing
- merges have the same split-brain wording:
  - downloader-composed before processing
  - but processing-order text still explains them as if they read freshly
    committed source outputs during the same batch
- `empty` versus `failed` downloader outcomes are still underdefined despite
  having opposite publication consequences
- forced-processing terminology is not defined tightly enough:
  - `recheck`
  - `reprocess`
  - provider-triggered full reprocess
  - integrity recovery
- health vocabulary is incomplete:
  - `category thresholds`
  - `one-observation grace`
  - `successful empty publication`
- deferred due events while dirty are not explicitly defined as coalescing into
  one immediate eligibility event
- provider-triggered full reprocess waves do not yet specify coalescing /
  serialization when a second provider update lands during an active wave
- retry behavior after processing failure is underspecified:
  - no backoff
  - no retry cap
  - no operator-visible stuck/failing policy

Likely wording fixes rather than design contradictions:

- lifecycle stages in `feeds.md` and runtime state machine in `pipeline.md`
  describe different abstractions and should be made explicitly non-competing
- `not modified` versus `same content` likely intends transport-level no-change
  vs content-level equality, but the spec should say so explicitly

Pending design decisions to resolve before specs are finalized:

1. What exact committed object do merges read when composing:
   - committed prepared input
   - canonical committed set output
   - something else named more clearly
2. What exact committed object do history derivatives read besides the new parent
   prepared body:
   - only derivative history / prior derivative prepared input
   - or the freshly committed parent canonical output
3. Should history derivatives age on wall-clock time without parent updates, or
   is their retention window defined strictly by parent update history?
4. For `reprocess` of merges and other synthetic feeds, should it mean:
   - exact replay from existing committed/staged prepared input
   - or re-compose now from the latest source state
5. What exact downloader outcomes count as successful empty versus failure,
   especially for:
   - empty file
   - comment-only file after filtering
   - malformed input
   - 404 / unavailable upstream

## Decisions recorded — 2026-04-20 after spec clarification

1. History feeds are "the last X days of source feed IPs".
   - always in days
   - additive semantics: they are the superset of all IPs listed by the source
     during the last X days
   - implemented using daily rollups
   - on each source update, add the new day and drop the oldest day as needed
   - the window is always relative to the datetime the source feed was updated
   - history feeds do not progress independently with wall-clock time; they move
     only when the parent source updates

2. Merges are downloader-stage work.
   - they are "downloaded" / recomposed on their own cadence
   - after that they are processed like all other feeds

3. Downloader result semantics are:
   - `empty`: the downloaded/composed feed is valid but contains no IPs
   - `failed`: the source is not there or a fresh version cannot be fetched
   - `not-updated`: the source was checked and explicitly reported not modified
   - `same`: the feed was downloaded/composed and is byte/content-equivalent to
     what was already committed

4. Time progression rule:
   - only merges progress as time passes
   - all other feeds that depend on external sources stay frozen at their last
     successful update until their source updates again

5. Processing failure policy:
   - processing failure is a major event
   - the feed MUST be returned to `waiting to be processed`
   - the exact staged feed body MUST be retained for retry/debugging
   - the last committed good outputs MUST remain authoritative
   - these failures MUST be logged with enough context to debug them later

6. Downloader rollups versus engine retention:
   - retention is owned by the engine
   - daily rollups are owned by the downloader
   - these are different concerns and the specs must not mix them
   - history derivatives are built from downloader-owned rollups
   - feed retention / age / removal policies remain engine-owned artifacts

## Spec review follow-up — 2026-04-21

Remaining work after the latest pass is now grouped as follows.

### Ready to fix directly in specs

- define merge downloader outcomes precisely:
  - merge recomposition yields `updated` or `same`
  - transport-style `not-updated` belongs to direct freshness checks, not
    merge recomposition
- define provider visibility in runtime queues:
  - providers may appear in downloader queues
  - providers must not appear as normal feed rows in processing queues
- define artifact-child bootstrap and derivative-rollup recovery:
  - first-time artifact child recovery requires rechecking the parent artifact
  - missing/corrupt history rollups require rechecking the parent feed
- align integrity `rebuild` wording with operator-facing `reprocess`
- define `historical` as a visibility/catalog flag, not a scheduling exclusion
- define `exclude_from_unmaintained` precisely:
  - it suppresses only age-based unmaintained classification
  - it does not suppress `empty` or `unavailable`
- define immediate processing-loop wake triggers for manual actions
- add the missing `Idle -> WaitingProcess` reprocess arrow to the state machine

### Important implementation/spec drift already visible

- specs now define downloader-owned UTC daily rollups for history derivatives
- implementation still uses per-update retention snapshots for historical
  windows
- this is an intentional spec-vs-implementation gap that should be reviewed
  after the contract text is finalized

## Decisions recorded — 2026-04-21

1. Waiting-to-be-processed queue age:
   - queue age MUST represent total waiting time since first admission
   - processing failure requeue MUST NOT reset the operator-visible queue age

2. Automatic retry cadence after hard downloader `failed` outcomes:
   - first retry delay is `configured_cadence / 16`
   - each subsequent unsuccessful retry doubles the delay
   - this doubling continues until the delay reaches the configured cadence
   - while the feed has not yet reached `unmaintained` health, retry delay MUST
     NOT exceed the configured cadence
   - once the feed has reached `unmaintained` health, further unsuccessful
     retries continue doubling beyond the configured cadence
   - post-unmaintained retry delay is capped at 1 month
   - intent:
     - temporary hard failures retry again soon
     - repeated hard failures back off gradually to normal cadence
     - only long-term stale/unmaintained feeds back off beyond their normal
       cadence

## Spec cleanup pass — 2026-04-21

This pass addressed the remaining spec-only gaps that did not require product
design decisions:

- clarified lifecycle stages 3/4 `or not applicable`
- defined bootstrap semantics for newly-created history derivatives
- classified merge composition failure within the downloader outcome model
- mapped feed-family vocabulary to admin-ui kind vocabulary
- removed remaining term drift from `prepared input` to `feed body`
- defined `reprocess` coalescing behavior when the same feed/body is already
  waiting to be processed

The remaining design decisions after this pass were:

- what equivalence relation defines `same` / `updated`
- staged-vs-committed priority for `reprocess`
- initial health for a never-successfully-observed feed
- provider database operator and enable/disable semantics

## Decisions recorded — 2026-04-21 follow-up

### Decision recorded — 2026-04-21: remove the legacy direct `RunOnce()` fetch path

Evidence from the current code:

- `cmd/update-ipsets/batch.go` still exposes `update-ipsets run`
- that path calls `eng.RunOnce(...)` directly instead of going through the
  scheduler queues
- `pkg/engine/run.go` still contains a dedicated `prefetch` phase
- `pkg/engine/process.go` still downloads plain feeds inside
  `processConcreteSource()`
- `pkg/engine/asn.go` and `pkg/engine/geoloc.go` still contain direct
  downloader fallback logic in the heavy block

Why this is wrong:

- the specs now define downloader-stage acquisition/composition and processing
  as two separate loops
- the daemon already follows that model through `FetchAndStage()` plus queued
  `RunOnce(...processing only...)`
- the remaining direct fetch path is a legacy compatibility leftover that keeps
  the engine capable of violating the intended separation of concerns

User decision:

- the leftover direct in-engine fetch path must be deleted
- if a CLI entrypoint still depends on that path, that CLI entrypoint must be
  removed or rewritten to use the scheduler model instead of preserving the
  old behavior

Implementation direction:

- delete `update-ipsets run`
- make `RunOnce()` processing-only
- remove the old `prefetch` phase and feed-local/plain-source downloader path
- remove provider-database direct download fallback from the heavy block
- update tests to stage inputs explicitly before processing

1. Equivalence relation for `same` / `updated`:
   - decided: semantic IP-set equivalence, not byte-equivalence of source/body
   - intent:
     - downloader outcomes should reflect the IP delta, not source formatting
     - reorder-only / formatting-only body changes should not count as updates

2. Provider database operator actions:
   - decided: provider databases use the normal feed-level actions
   - they can be rechecked/reprocessed/enabled/disabled through the same feed
     inventory surface as other source rows

3. Provider database enable/disable semantics:
   - decided: normal source enable/disable semantics apply
   - disabling a provider stops future refresh and future enrichments from that
     provider until it is re-enabled

4. Reprocess body priority:
   - decided: if a staged feed body exists, `reprocess` uses it
   - fallback: when no staged feed body exists, `reprocess` uses the committed
     feed body
   - intent:
     - the downloader-produced staged body is the newest local source of truth
       pending publication

5. Initial health for a never-successfully-observed feed:
   - decided: `unavailable`
   - intent:
     - a new feed is due immediately, so this state is normally brief
     - if a newly added feed cannot be downloaded successfully, it should not
       remain in a fake transitional state forever

6. History-derivative rollup availability:
   - decided: daily rollups are sparse and reflect only successful parent
     update days
   - the product MUST use whatever rollups are available
   - it MUST NOT expect one rollup per calendar day or infer corruption merely
     because fewer than `X` rollups exist
   - this is required because parent feeds may legitimately update less often
     than daily

## Implementation gap analysis — 2026-04-21

Facts verified in the current codebase against the current specs:

1. Scheduler retry cadence still uses the pre-spec logic
   - evidence:
     - `pkg/scheduler/scheduler.go:1003-1042`
     - `pkg/engine/helpers.go:34-73`
   - current behavior:
     - on failures it halves the cadence first, then multiplies by failure count
     - it is not driven by the spec's `cadence/16 -> doubling -> capped at
       cadence until unmaintained -> capped at 1 month`
   - required fix:
     - implement the new hard-failure retry schedule in the scheduler/runtime
       due calculation

2. Processing-failure requeue resets queue age
   - evidence:
     - `pkg/scheduler/scheduler.go:722-731`
   - current behavior:
     - `finishProcessing(..., requeue=true)` overwrites `QueuedAt` with `now`
   - required fix:
     - preserve the first queue admission timestamp for the same staged body

3. Feed health still defaults to `healthy` before first successful publication
   - evidence:
     - `pkg/feedhealth/feedhealth.go:72-74`
   - current behavior:
     - `Classify(nil, ...)` returns `healthy`
   - required fix:
     - align with spec: enabled-but-never-successfully-published feeds classify
       as `unavailable`

4. Queued processing still short-circuits instead of always running the full
   engine pipeline
   - evidence:
     - `pkg/engine/process.go:279-287`
     - `pkg/engine/process.go:340-347`
   - current behavior:
     - mtime shortcut returns `source file has not been updated`
     - semantic same-set shortcut returns `processed set is the same as the previous one`
   - required fix:
     - once a feed is admitted to processing, it must continue through finalize,
       retention, history/rollup handling, and downstream publication work

5. History derivatives are still implemented as rolling timestamp snapshots,
   not downloader-owned sparse UTC daily rollups
   - evidence:
     - `pkg/config/expand.go:14-24`, `pkg/config/expand.go:70-123`,
       `pkg/config/expand.go:208-242`
     - `pkg/engine/finalize.go:135-218`
     - `pkg/engine/provider_retention.go:16-134`
   - current behavior:
     - derivatives still use `internal://retention_window?parent=<name>&minutes=<n>`
     - successful parent processing writes one snapshot per update timestamp
     - derivative synthesis unions snapshots from the last `N` minutes
   - required fix:
     - switch to downloader-owned daily rollups keyed by UTC day
     - fold multiple same-day parent updates into one day slot
     - compose derivatives from the last `X` available rollups

6. Downloader same/not-modified decisions are still based on raw body/file
   equality, not semantic feed-body equality
   - evidence:
     - `pkg/downloader/downloader.go:273-299`
     - `pkg/downloader/internal.go:162-188`
     - `pkg/engine/download_stage.go:281-307`
   - current behavior:
     - same-body detection compares SHA256 of the raw downloaded/composed body
       against the current reference file
   - required fix:
     - downloader-stage queue admission must reflect semantic IP/CIDR-set
       equivalence, not formatting-only differences

7. Pairwise comparison file symmetry is already correct
   - evidence:
     - `pkg/engine/output.go:345-552`
   - current behavior:
     - when one feed changes, pairwise facts are recomputed against every peer
       and merged into both sides' `_comparison.json`
   - implication:
     - this part already matches the current spec and should not be regressed

8. Insights are currently written for every output feed on every heavy run
   - evidence:
     - `pkg/engine/insights.go:68-77`
   - current behavior:
     - full sweep, regardless of which feed changed
   - note:
     - correctness is acceptable, but this is still a possible future
       optimization if the specs later tighten relationship-only insight
       refresh behavior

9. Merges and history derivatives are still engine-owned synthetic sources
   instead of downloader-stage feed families
   - evidence:
     - `pkg/config/expand.go`
     - `pkg/engine/provider_registry.go`
     - `pkg/engine/provider_retention.go`
     - `pkg/engine/provider_merge.go`
   - current behavior:
     - config expansion emits `internal://retention_window` and
       `internal://merge`
     - engine startup registers internal providers for them
     - synthetic bodies are still produced through engine-time provider hooks
   - required fix:
     - move synthetic feed-body preparation fully into downloader-stage
       `FetchAndStage()` semantics
     - keep the processing engine limited to staged/committed feed bodies only

10. Automatic merge cadence still bypasses the downloader queue
    - evidence:
      - `pkg/scheduler/scheduler.go:414-440`
    - current behavior:
      - scheduled plain feeds enter `downloadWaiting`
      - scheduled merges enter `processingWaiting` directly
    - required fix:
      - merges must behave like downloader-stage items on cadence and only
        enter processing after downloader-stage composition decides they should

11. RunOnce still injects dependents dynamically during processing
    - evidence:
      - `pkg/engine/run.go`
      - `pkg/config/dependents.go`
    - current behavior:
      - plain feeds process first
      - updated parents inject history derivatives
      - explicit phases later enqueue history and merge synthetic feeds
    - required fix:
      - remove same-run synthetic feed generation from the engine
      - scheduler/downloader must queue the exact processing set before the
        processing batch starts

## Execution plan — 2026-04-21

1. Replace runtime dependence on `internal://merge` and
   `internal://retention_window` providers with explicit feed-family metadata
   carried by config expansion.
2. Extend downloader-stage `FetchAndStage()` so it can:
   - fetch plain feeds
   - compose merges from committed feed bodies
   - compose history derivatives from parent body plus sparse UTC daily
     rollups
3. Change automatic merge scheduling to queue downloader work, not direct
   processing work.
4. Remove engine-time synthetic provider registration and dynamic dependent
   injection from `RunOnce()`.
5. Keep processing strictly consumption-based:
   - read staged/committed feed bodies
   - run the full downstream publication pipeline once admitted
6. Update recovery and tests to the new downloader-owned composition model.
