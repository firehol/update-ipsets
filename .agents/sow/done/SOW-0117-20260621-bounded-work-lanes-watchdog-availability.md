# SOW-0117 - Bounded Work Lanes And Watchdog Availability

## Status

Status: completed

Sub-state: reopened for 2026-06-25 production deadlock/watchdog regression
analysis and fix; V27 full-scope external rerun found no remaining SOW-0117
deadlock/liveness blockers after explicit light-status coverage hardening. Closed
with the implementation, validation evidence, and follow-up SOW-0118 split
recorded.

## Requirements

### Purpose

Make the daemon reliably usable while heavy processing is running. The web
server, admin status, health endpoint, and systemd watchdog must remain
responsive even when feed processing, entity artifacts, integrity, metadata,
history derivatives, or downloader work are active.

This is a production-availability SOW. The purpose is not to redesign the whole
engine from SOW-0106. The purpose is a focused concurrency and admission model
that prevents uncoordinated heavy work from starving the daemon.

### User Request

The user asked for an SOW and analysis of whether the system can implement a
simple bounded model:

- engine, artifact background work, and integrity are bounded by an explicit
  concurrency setting and FIFO queue
- if the limit is `1`, work starts in pure FIFO order
- if the limit is `2`, two work items run and the rest wait in FIFO order
- downloaders have their own FIFO
- DroneBL acquisition/processing is downloader work and must also enter the
  download FIFO
- the web server and watchdog are not limited by those queues
- if this cannot be solved well in Go, consider rewriting the system in Rust

The approved model has three lanes:

- `download` lane: the scheduler-owned downloader FIFO. It owns upstream fetches,
  artifact acquisition, artifact child materialization, DroneBL buildzone work,
  and downloader-stage local composition. Its public worker count remains the
  existing download concurrency setting.
- `engine` lane: a new FIFO admission lane for heavyweight engine, entity
  artifact, and integrity entrypoints. It defaults to one worker. It controls
  top-level admission, not the internal fan-out inside an admitted engine run.
- `free` lane: a no-queue, no-slot contract for cheap availability paths:
  public cached serving, `/healthz`, watchdog notify, and bounded admin status
  snapshots. These paths may observe lane state but must not acquire a download
  or engine lane slot.

### Assistant Understanding

Facts:

- Production had a systemd watchdog kill after a long period with no useful
  response. The stack dump showed the watchdog goroutine was runnable, not
  logically deadlocked.
- Production had concurrent heavy paths during the same window: entity artifact
  JSON write fan-out, admin entity-integrity JSON decode/scan work, and
  downloader-stage history derivative `pkg/iprange` union work.
- Existing specs already require separate concurrency domains and bounded
  background work, but the implementation still has direct heavy goroutine
  entrypoints that are not admitted by one central FIFO lane.
- Existing background work has a limiter, but it is a concurrent-slot limiter,
  not a strict FIFO work queue, and it does not cover all heavy work.
- The current admin entity-integrity handler uses a status snapshot heuristic
  before running a live integrity scan; it does not acquire a queue lease that
  proves the scan cannot race with entity mutation.

Inferences:

- The failure is architectural, not a fundamental Go limitation.
- A Rust rewrite would still need the same queue/admission design. Rust would
  not automatically prevent CPU, GC-equivalent allocation pressure, disk I/O, or
  request starvation caused by starting too many heavy jobs at once.
- The fastest safe path is a Go work-lane coordinator with behavioral tests,
  then moving every heavyweight entrypoint behind it.

### Acceptance Criteria

- The implementation preserves the three-lane model above.
- The `engine` lane has strict FIFO admission. With one worker, only one
  engine-lane item may start; with two workers, exactly two may start and later
  items start in enqueue order as slots free.
- The `download` lane remains separate and FIFO. DroneBL acquisition and child
  materialization stay in the downloader lane.
- The `free` lane is not implemented as a third worker queue. It is an
  availability contract: cheap web, watchdog, health, and bounded status paths
  remain outside heavy admission and stay responsive while lanes are busy.
- `RunOnce` is one `engine` lane item. Existing `max_processing_workers` and
  `max_heavy_phase_workers` continue to control fan-out inside that admitted
  run; they are not replaced by the lane.
- Entity artifact publication remains serialized even if the engine lane worker
  count is raised above one.
- Admin entity-integrity page loads never run live filesystem scans directly.
  They return `in_progress`, cached settled findings, or a queued/accepted state.
  Explicit refresh/rebuild actions queue engine-lane work.
- Admin pipeline-integrity page loads and reprocess actions never run live
  filesystem scans directly. Reprocess uses cached settled findings; if no
  usable cached report exists, it queues/requests refresh instead of scanning in
  the HTTP handler.
- Existing background-task visibility is preserved, but the old background slot
  limiter does not remain as a second admission point for engine-lane work.
- Existing `max_background_workers` remains a background/entity fan-out setting;
  it is not an alias for engine-lane admission.
- Admin pipeline-integrity page loads follow the same no-live-scan rule as entity
  integrity page loads.
- Lane state is visible in admin status/UI without violating the four-list admin
  queue contract; engine-lane work appears in the background/operations status
  area, not as extra downloader/processing live lists.
- Tests prove FIFO, cancellation, no slot leaks after panic/error, free-path
  responsiveness, admin pipeline/entity integrity non-blocking behavior, and
  DroneBL downloader ownership.
- Specs, project skills, and operator docs are updated where runtime config,
  integrity semantics, admin visibility, or background-work guidance changes.

## Analysis

Sources checked:

- Production journal and admin-status evidence from 2026-06-21, summarized
  without private hostnames, private client addresses, or raw credentials.
- `pkg/web/server_run.go:120`
- `pkg/web/server_run.go:225`
- `cmd/update-ipsets/daemon.go:88`
- `cmd/update-ipsets/daemon.go:92`
- `pkg/web/integrity.go:112`
- `pkg/web/integrity.go:169`
- `pkg/web/integrity.go:184`
- `pkg/web/integrity.go:254`
- `pkg/web/integrity.go:287`
- `pkg/web/integrity.go:297`
- `pkg/web/routes.go:294`
- `pkg/web/routes.go:299`
- `pkg/web/server.go:160`
- `pkg/web/admin.go:257`
- `pkg/web/admin.go:518`
- `pkg/web/sysinfo.go:69`
- `pkg/engine/types.go:110`
- `pkg/engine/background_tasks.go:38`
- `pkg/engine/background_tasks.go:185`
- `pkg/engine/background_tasks.go:239`
- `pkg/engine/entity_refresh_queue.go:61`
- `pkg/engine/entity_refresh_queue.go:149`
- `pkg/engine/entity_refresh_queue.go:265`
- `pkg/engine/entity_refresh_queue.go:331`
- `pkg/engine/entity_surgical.go:25`
- `pkg/engine/entity_surgical_refresh.go:39`
- `pkg/engine/entity_artifacts_write.go:347`
- `pkg/engine/entity_feed_sidecar_build.go:49`
- `pkg/engine/feed_body_stage.go:442`
- `pkg/engine/entity_artifact_repair.go:5`
- `pkg/engine/engine_fixture_test.go:19`
- `pkg/scheduler/scheduler.go:273`
- `pkg/scheduler/download_loop.go:10`
- `pkg/scheduler/queue_admission.go:102`
- `pkg/scheduler/recovery.go:10`
- `ui/src/lib/admin-api-types.ts:305`
- `.agents/sow/specs/operating-principles.md:273`
- `.agents/sow/specs/operating-principles.md:483`
- `.agents/sow/specs/operating-principles.md:579`
- `.agents/sow/specs/config.md:558`
- `.agents/sow/specs/pipeline.md:308`
- `.agents/sow/specs/pipeline.md:530`
- `.agents/sow/specs/integrity.md:215`
- `.agents/sow/specs/integrity.md:406`
- Official Go docs:
  - `https://pkg.go.dev/context`
  - `https://go.dev/doc/effective_go#channels`
  - `https://pkg.go.dev/golang.org/x/sync/errgroup`
  - `https://pkg.go.dev/golang.org/x/sync/semaphore`
  - `https://pkg.go.dev/runtime`
  - `https://pkg.go.dev/runtime#ReadMemStats`
- Open-source reference evidence:
  - `argoproj/argo-cd @ 24af376521be6c444e333a22fc11bc018f999814`
    `controller/appcontroller.go:200`
    `controller/appcontroller.go:908`
    `controller/appcontroller.go:951`
    `controller/appcontroller.go:975`
  - `grafana/loki @ eecfe8a42c441a6dad7c40309183117bb6282204`
    `pkg/engine/compactor/config.go:25`

Current state:

- The watchdog is launched as an independent goroutine and calls systemd on a
  ticker (`pkg/web/server_run.go:225`). That means the watchdog path exists,
  but it can still starve if the process is saturated by CPU, GC, or disk I/O.
- Startup starts entity artifact checking in a direct goroutine and starts the
  scheduler in another direct goroutine (`pkg/web/server_run.go:120`). These are
  independently running long-lived paths.
- The scheduler starts fetch, processing, and staged-recovery loops inside
  `Runner.Run` (`pkg/scheduler/scheduler.go:273`).
- Scheduler staged-work recovery currently runs as a direct goroutine at
  scheduler startup (`pkg/scheduler/scheduler.go:285`). It calls
  `RecoverStagedSources` and `RecoverStagedArtifacts`
  (`pkg/scheduler/recovery.go:17`, `pkg/scheduler/recovery.go:30`).
  `RecoverStagedArtifacts` currently calls `materializeArtifactChildren`
  directly (`pkg/engine/download_stage.go:151`), so a recovered staged DroneBL
  artifact can materialize children outside the downloader FIFO. That violates
  the user requirement that DroneBL acquisition/processing is downloader-lane
  work and must be fixed by this SOW. After direct materialization,
  `recoverStagedWork` currently enqueues recovered children into the processing
  queue (`pkg/scheduler/recovery.go:42`) instead of routing the recovered
  artifact through a downloader worker. The implementation must preserve normal
  child processing after materialization, but the materialization decision must
  be made by the downloader lane.
- The processing loop calls `eng.RunOnce` synchronously for a drained processing
  batch (`pkg/scheduler/processing_loop.go:41`). `RunOnce` already has a
  defensive `tryMarkRunStart` running flag, but that flag is a binary
  in-progress guard, not FIFO admission.
- The processing loop installs a `BeforePublish` callback
  (`pkg/scheduler/processing_loop.go:48`) that promotes committed downloads via
  `PromoteCommittedDownloads` (`pkg/scheduler/processing_loop.go:55`) from
  inside `RunOnce`, before the publish step (`pkg/engine/run_pipeline.go:373`).
  After `RunOnce` is engine-lane admitted, this callback inherits the engine
  lane. It is an artifact commit handoff, not downloader fetch/materialization
  work.
- The downloader loop has an explicit worker count from
  `Runtime().ParallelDownloads` (`pkg/scheduler/download_loop.go:10`) and
  dispatches downloads while active count is below the worker count
  (`pkg/scheduler/queue_admission.go:102`).
- DroneBL is already downloader-owned. Artifact due work enters the scheduler
  download queue (`pkg/scheduler/automatic_due.go:47` in
  `enqueueAutomaticArtifactDue`), `runDownload` calls
  `eng.FetchAndStage` (`pkg/scheduler/download_loop.go:60`), and DroneBL
  dispatches through `fetchAndStageArtifact` into
  `fetchAndStageDroneBLArtifact` (`pkg/engine/artifact_stage.go:25`,
  `pkg/engine/artifact_stage.go:39`). The implementation must preserve this
  normal due path and close the staged-recovery bypass described above.
- Entity artifact refresh and health refresh still start direct goroutines when
  the local coalescing map transitions from idle to active
  (`pkg/engine/entity_refresh_queue.go:61`, `pkg/engine/entity_refresh_queue.go:85`).
- Background tasks are visible and pass through `withBackgroundTask`, which uses
  `backgroundLimiter.AcquireContext` (`pkg/engine/background_tasks.go:185`).
  This bounds running background tasks, but it does not provide strict FIFO
  start order and it does not cover engine runs, admin integrity scans, or
  downloader-stage local set algebra.
- The background limiter uses a condition variable and wakes waiters
  (`pkg/engine/background_tasks.go:38`). This is a slot limiter, not a queue
  with stable admission order.
- Admin entity integrity checks a status snapshot and returns `in_progress` only
  if the snapshot says the engine or an entity background task is active
  (`pkg/web/integrity.go:121`, `pkg/web/integrity.go:254`). If the check passes,
  it calls `eng.CheckEntityArtifactsIntegrity()` directly (`pkg/web/integrity.go:133`).
  This is a race-prone guard because the status snapshot is not an acquired
  execution lease.
- Entity integrity scans country and ASN details by walking sidecar files and
  decoding JSON (`pkg/engine/entity_integrity_detail_scan.go:119`). Production
  showed this path active during the stall window.
- Surgical entity refresh can fall back to a full entity rebuild
  (`pkg/engine/entity_surgical_refresh.go:39`). Production showed broad entity
  detail writing active during the stall window.
- Entity detail writes loop over affected countries and ASNs and write private
  and public JSON payloads (`pkg/engine/entity_artifacts_write.go:347`,
  `pkg/engine/entity_artifacts_write.go:406`). This is expected work, but it
  must not run concurrently with another live integrity scan over the same file
  family.
- `Engine.Reload()` currently holds `e.mu` while replacing runtime/config state
  and while running reload cleanup (`pkg/engine/engine.go:204`,
  `pkg/engine/engine.go:245`, `pkg/engine/engine.go:293`). The lane migration
  must not submit cleanup while holding `e.mu`; otherwise the lane and engine
  locks can form a deadlock cycle.
- Admin full status currently reads up-to-date runtime memory statistics on
  request (`pkg/web/admin.go:518`, `pkg/web/sysinfo.go:69`). The new light
  status path must not copy this runtime-wide sampling into high-frequency
  polling.
- Existing entity publication already has a separate serialization invariant:
  entity artifact publishers must not overlap even when background worker limits
  are raised. The current code uses `entityArtifactsMu` around publication
  paths; the engine lane must preserve that invariant and must not rely on the
  default worker count of one as the only safety mechanism.
- Downloader-stage history derivative composition opens history snapshots and
  calls `iprange.UnionSourcesContext` (`pkg/engine/feed_body_stage.go:442`).
  This is queue-owned by the downloader model, but operationally it is local
  CPU/IO-heavy set algebra.

Production evidence:

- The watchdog kill was not explained by a goroutine count explosion. The stack
  dump had a small number of heavyweight goroutines doing expensive work.
- The web/admin symptom matches resource starvation: an admin reload blocked
  while status building and JSON writing were very expensive in the same run.
- A previous production run reported very high admin status build times during
  metadata/entity-heavy phases. That means the admin status path itself must
  stay lightweight and must not compete with heavyweight work.

External evidence:

- Official Go docs provide the required primitives:
  - contexts propagate cancellation across goroutines and APIs
  - buffered channels and worker loops are valid ways to bound work admission
  - `errgroup.SetLimit` bounds active goroutines in a group
  - weighted semaphores bound concurrent access to a resource
  - `GOMAXPROCS` controls how many CPUs execute Go code simultaneously, but it is
    not a queue or workload-isolation mechanism
- Argo CD uses named typed work queues and separate worker counts for different
  controller workloads. Its hydration comments explicitly separate enqueueing
  from heavy work and document the concurrency knob.
- Loki exposes a task concurrency cap for compaction work and records that it is
  applied via `errgroup.SetLimit`.

Doability conclusion:

- Yes, this is doable in Go.
- The brutal truth: the current implementation is not failing because Go cannot
  express FIFO lanes. It is failing because heavyweight work still enters the
  process through multiple independent paths, and some paths use status
  heuristics instead of a shared admission contract.
- Rewriting in Rust would not remove the need for a work-lane coordinator. A
  Rust implementation without the same admission model would reproduce the same
  operational bug with different syntax.

Risks:

- If only the background limiter is improved, engine runs and downloader-stage
  heavy composition can still starve admin/watchdog paths.
- If admin integrity is only guarded by status snapshots, it can still race with
  entity mutation and produce transient false findings or resource contention.
- If the downloader lane includes CPU-heavy local set algebra and runs
  concurrently with the engine lane, the daemon may still saturate CPU/IO under
  a combined workload.
- If the new queue is hidden from admin status/UI, operators will think the
  service is idle while work is merely queued.
- If implementation changes queue order without tests, production regressions
  will be hard to reproduce.
- If the engine lane is confused with existing heavy-phase workers, operators
  will lose the ability to tune admitted-run fan-out independently from
  top-level admission.
- If `withBackgroundTask` keeps the old slot limiter under the new lane,
  engine-lane work can double-block or deadlock under a one-worker default.
- If free-path status handlers take long engine locks or run live scans, the
  admin UI can still stall even though lane admission exists.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The daemon has multiple heavyweight work sources: engine runs, background
  entity refresh/rebuild/repair, admin integrity scans, and downloader-stage
  local set algebra.
- Some are bounded by existing worker counts, some use `withBackgroundTask`, and
  some run directly after a status snapshot.
- The existing controls do not define one admission point for all heavyweight
  local work. This allows expensive jobs to overlap in ways the operator did not
  explicitly configure.
- The watchdog and web server are not intentionally blocked by locks in the
  evidence reviewed; they become unresponsive because the process is saturated
  by heavy work and allocation/I/O pressure.

Evidence reviewed:

- `pkg/web/server_run.go:225` proves the watchdog goroutine exists.
- `pkg/web/server_run.go:120` proves startup entity work runs in a direct
  goroutine next to the scheduler.
- `pkg/engine/background_tasks.go:38` and
  `pkg/engine/background_tasks.go:185` prove background work has a limiter but
  not a FIFO queue covering all heavyweight work.
- `pkg/engine/entity_refresh_queue.go:61` proves entity refresh work is started
  by direct goroutine admission.
- `pkg/web/integrity.go:121` and `pkg/web/integrity.go:133` prove admin entity
  integrity can transition from a status heuristic into a live scan without an
  acquired execution lease.
- `pkg/engine/feed_body_stage.go:442` proves downloader-stage composition can
  execute heavy `pkg/iprange` union work.
- `.agents/sow/specs/config.md:558` and
  `.agents/sow/specs/operating-principles.md:579` already require separate
  concurrency domains.
- Official Go docs and the Argo CD / Loki references show this queue/worker
  shape is normal Go practice.

Affected contracts and surfaces:

- Daemon liveness and systemd watchdog behavior.
- Admin status and admin integrity APIs.
- Admin UI active/pending work visibility.
- Scheduler downloader and processing queue semantics.
- Engine run admission and background entity artifact work.
- Integrity "in progress" versus settled findings semantics.
- Runtime config for concurrency limits.
- Specs: `operating-principles.md`, `pipeline.md`, `config.md`,
  `integrity.md`, and `admin-ui.md`.
- Tests: scheduler, engine background tasks, admin integrity, web health/status,
  and race/cancellation coverage.

Existing patterns to reuse:

- Runtime concurrency fields and status exposure in `pkg/engine/runtime.go` and
  `pkg/engine/status_snapshot.go`.
- Existing scheduler queue state and admin live-list model.
- Existing `BackgroundTaskSnapshot` structure for visible task progress.
- Existing context propagation rules and cancellation tests.
- Existing `withBackgroundTask` visibility and metrics. Its old
  `backgroundLimiter` slot acquisition must be replaced for engine-lane work,
  not stacked under the new lane.

Risk and blast radius:

- Medium-to-high because it changes runtime scheduling and liveness behavior.
- Data-loss risk is low if the queue only controls admission and does not change
  artifact writers, but publication cancellation paths must still be validated.
- Operator-behavior risk is medium because queued background and integrity work
  will become more visible and may wait longer under `1` concurrent engine slot.
- Performance risk is positive for liveness but may increase total wall-clock
  time if concurrency is set conservatively. That is acceptable when the purpose
  is production availability.
- The SOW must not delete or migrate historical feed data.

Sensitive data handling plan:

- Durable artifacts must not include raw production hostnames, private endpoint
  names, Tailscale/private client IPs, credentials, bearer tokens, customer
  names, or raw proprietary incident details.
- Production evidence in this SOW is summarized by subsystem, timing class, and
  code path only.
- If future implementation needs stack dumps or logs in tests, fixtures must be
  synthetic or redacted.

Implementation plan:

1. Add an engine-owned FIFO work-lane coordinator. Keep it in `pkg/engine`
   in `pkg/engine/work_lane.go` with tests in `pkg/engine/work_lane_test.go`
   unless implementation proves a smaller named package is clearer. Avoid
   generic `manager`, `common`, or `utils` names.
2. The lane API must support:
   - `Run(ctx context.Context, work LaneWork, fn func(context.Context) error) error`
     for scheduler/engine call sites that already run outside HTTP request
     lifetime
   - `Submit(ctx context.Context, work LaneWork, fn func(context.Context) error) (LaneTicket, error)`
     for operator actions and entity coalescing paths; `LaneTicket.Queued` and
     `LaneTicket.Coalesced` report whether this call created a new queued item or
     joined an existing compatible item
   - `TryRun(ctx context.Context, work LaneWork, fn func(context.Context) error) (bool, error)`
     only for internal tests or explicitly documented non-blocking probes
   - stable snapshots of active and waiting work
   - cancellation of queued work, guaranteed slot release on error or panic, and
     bounded shutdown behavior
   - non-blocking submission from an already active lane worker without waiting
     on its own queued child work
   `LaneWork` and `LaneTicket` are concrete typed contracts, not placeholders:

   ```go
   type LaneWork struct {
       ID            string
       Kind          LaneWorkKind
       Component     LaneWorkComponent
       Name          string
       Trigger       string
       Phase         string
       Stage         string
       Detail        string
       CoalescingKey string
       QueuedAt      time.Time
   }

   type LaneTicket struct {
       ID        string
       Kind      LaneWorkKind
       Component LaneWorkComponent
       Queued    bool
       Coalesced bool
       State     LaneWorkState
   }
   ```

   Implementation may add fields, but HTTP/admin responses and metrics must use
   typed `Kind` / `Component` / `State` values rather than parsing display
   strings. `LaneWorkKind`, `LaneWorkComponent`, and `LaneWorkState` are typed
   string constants with JSON string serialization. `LaneWorkState` must include
   at least `queued`, `active`, `completed`, `failed`, `canceled`, and
   `skipped`. A coalesced ticket's `State` mirrors the existing work item it
   joined; for example, coalescing with a running rebuild returns `State:
   active`, not `queued`. `LaneTicket` is an immutable submission-time value
   copy for the accepted/coalesced request. It is not a durable polling handle
   and this SOW does not add a ticket lookup HTTP endpoint; later operator
   visibility comes from lane snapshots keyed by stable IDs. A coalesced ticket
   carries the existing work item's stable ID; repeated `Submit` calls for the
   same coalescing key do not mint new IDs while the original work item is
   queued or active.

   Canonical `LaneWorkKind` values are: `engine_run`, `entity_rebuild`,
   `entity_refresh`, `entity_repair`, `integrity_refresh`,
   `integrity_reprocess`, and `cleanup`. Canonical `LaneWorkComponent` values
   are: `engine_run`, `entity_artifacts`, `entity_artifacts_health`,
   `entity_integrity`, `pipeline_integrity`, `critical_infrastructure`, and
   `publish_stages`. Implementations may add a value only when a new production
   work family exists; admin JSON, metrics, UI fixtures, and docs must be
   updated in the same change.

   Production `Submit` callers must provide a non-empty `CoalescingKey`.
   Coalescing keys must come from the finite canonical key set in item 18, or
   from bounded configured domains such as feed names where the config bounds
   the key space. They must not include timestamps, random IDs, file paths,
   request IDs, or other unbounded values. A production `Submit` with an empty
   or unbounded coalescing key returns typed `ErrLaneMissingCoalescingKey` or
   equivalent. This keeps repeated admin, reload, entity, cleanup, and integrity
   requests memory-bounded by coalescing instead of creating an unbounded FIFO.
   Blocking `Run` is reserved for scheduler/engine call sites and respects the
   caller context while waiting for admission; if the caller context is canceled
   before admission, `Run` returns the context error without starting work.
   Struct sketches in this SOW omit repetitive tags for readability, but the
   implementation must add explicit `json` tags for every field that reaches
   admin/API JSON. JSON keys use the existing snake_case style shown in examples
   (`queued_at`, `max_engine_lane_workers`, `engine_lane`, etc.); internal-only
   fields use `json:"-"`.
   The coordinator implementation contract is a mutex-protected FIFO queue with
   monotonic sequence numbers, not map iteration order. Canceled queued items are
   removed or skipped by sequence before slot acquisition. Slot release is
   protected by `defer` so error and panic paths cannot leak capacity. Background
   submissions recover panics into task failure/log state after releasing the
   slot; synchronous `Run` may return the panic as an error or re-panic only
   after the slot is released, but it must never leave the lane wedged.
   The lane marks worker contexts with an internal lane context key or
   equivalent typed signal. `Submit` from inside a worker remains non-blocking;
   blocking `Run` from inside the same lane must return an error or fail tests
   instead of waiting on itself.
3. Add `runtime.max_engine_lane_workers` as the operator-facing worker count for
   the new engine lane, default `1`. Preserve existing public knobs:
   - `parallel_downloads` remains the download-lane worker count
   - `max_processing_workers` remains feed-processing fan-out inside an admitted
     engine run
   - `max_heavy_phase_workers` remains heavy-phase fan-out inside an admitted
     engine run
   - `max_background_workers` remains a background/entity fan-out setting used
     inside admitted work, notably `pkg/engine/entity_feed_sidecar_build.go`
     where `Runtime.BackgroundWorkers()` controls internal sidecar build fan-out;
     it is not an alias for `max_engine_lane_workers` and must not create a
     second admission limiter under the engine lane
   - `max_ingest_workers` remains a ceiling for all worker pools and must clamp
     `max_engine_lane_workers`
   Config validation must add `max_engine_lane_workers` to the runtime resource
   control validation path so negative values fail with the same quality as the
   existing worker fields. Missing or zero values resolve to the default of `1`;
   the lane `SetLimit` implementation defensively clamps any value below `1` to
   `1` so no runtime path can wedge the queue with a zero-worker limit.
   `pkg/engine/runtime.go` must carry the resolved runtime field
   `MaxEngineLaneWorkers`, expose `EngineLaneWorkers()` or equivalent, and apply
   the same ingest ceiling as other worker pools. The implementation must add the
   equivalent of
   `r.MaxEngineLaneWorkers = clampRuntimeWorkers(r.MaxEngineLaneWorkers, r.MaxIngestWorkers)`
   to `applyRuntimeIngestWorkerCeiling`. `Runtime.BackgroundWorkers()` continues
   to return `MaxBackgroundWorkers`; the new `Runtime.EngineLaneWorkers()` (or
   equivalent) returns `MaxEngineLaneWorkers`. They are distinct fields with
   distinct validation and reporting semantics. Concrete structural additions:
   - add `MaxEngineLaneWorkers int` with YAML key
     `max_engine_lane_workers,omitempty` to `pkg/config/config.go` `RuntimeConfig`
   - set `MaxEngineLaneWorkers: 1` in `pkg/config/config.go:DefaultRuntime`
     or the local defaulting path, so omitted/zero config resolves to one lane
     worker before validation and runtime snapshots
   - add `MaxEngineLaneWorkers int` to `pkg/engine/runtime.go` `Runtime`
   - add `MaxEngineLaneWorkers int` with JSON key
     `max_engine_lane_workers,omitempty` to `pkg/engine/types.go`
     `StatusSnapshot`
   - add `"runtime.max_engine_lane_workers": runtime.MaxEngineLaneWorkers` to
     `pkg/config/validate.go` `validateRuntimeResourceControls`
   - add `func (r Runtime) EngineLaneWorkers() int` or equivalent, returning the
     resolved clamped worker count with default `1`
4. Initialize the engine lane in `Engine` construction and update it on runtime
   reload before startup entity work or scheduler work can submit. Runtime
   reload must call the lane equivalent of `SetLimit(rt.EngineLaneWorkers())`
   after resolving and storing the new runtime, preserving FIFO order while
   applying the new limit. `Engine` receives explicit fields for the engine lane
   and for the in-memory pipeline/entity integrity caches; production code must
   not rely on nil-lane or nil-cache fallbacks. The intended `Engine` fields are
   `engineLane *WorkLane` or equivalent, `pipelineIntegrityCache
   pipelineIntegrityCacheState`, and `entityIntegrityCache
   entityIntegrityCacheState`. Naming may vary to match local style, but these
   must be explicit fields on `Engine`, not package globals or web-layer state.
   `newEngineFixture` and every direct `Engine` test construction must
   initialize the lane and both caches.
   `engine.New()` is called before the daemon context exists, so the lane is
   constructed without a daemon context and then registered for shutdown from
   `web.Run` before server listeners are started. Add an `AttachContext`,
   `SetShutdownContext`, or equivalent lane method. After that context is
   canceled, new submissions return a typed `ErrLaneShuttingDown` error and
   queued items are canceled/skipped.
5. Replace and remove `backgroundLimiter` admission for engine-lane work.
   Preserve background task visibility and compatible metric names with a new
   `withEngineLaneBackgroundTask` wrapper. Required shape:
   `func (e *Engine) withEngineLaneBackgroundTask(ctx context.Context, name, trigger, stage, detail string, current, total int, fn func(*BackgroundTaskHandle) error) error`.
   The wrapper must call the same background-task begin/update/finish and metric
   paths used today for visibility, but it must never call
   `backgroundLimiter.AcquireContext` or release the old limiter. After lane
   migration this wrapper is visibility/metrics/progress plumbing inside an
   already-admitted lane worker; it does not acquire the engine lane itself.
   Lane admission happens at the outer `Run` / `Submit` call site. After
   migration, `backgroundLimiter`, `withBackgroundTask`, and
   `newBackgroundLimiter` are dead code and must be removed or left only as test
   helpers with no production call sites. No engine, entity, integrity, repair,
   rebuild, or cleanup path may use an old limiter as a second admission point.
   Migration order is mandatory: introduce the lane and wrapper first, migrate
   every production caller of `withBackgroundTask`, update tests/metrics, then
   remove `backgroundLimiter` and `withBackgroundTask` from production code.
   During the migration window, migrated callers bypass the old limiter
   entirely and old callers remain on the old limiter until they are migrated.
   The two admission paths are independent; do not add synchronization between
   them. The terminal state has no production callers of the old limiter or old
   wrapper.
6. Admit the entire `RunOnce` call as one engine-lane work item. Do not split
   only `runHeavyPhases` in this SOW. Keep `tryMarkRunStart` as a defensive
   internal running-state guard after lane admission, not as the primary
   scheduler/operator admission mechanism. Public `RunOnce` must perform lane
   admission and then call an internal admitted helper containing the existing
   run body; no external caller may bypass the lane by calling the admitted
   helper directly. Direct test calls to public `RunOnce` remain synchronous:
   they block until their lane-admitted run completes, so existing tests must not
   need a separate production-only entrypoint. `RunOnce` remains serialized by
   `tryMarkRunStart` even when `max_engine_lane_workers` is greater than one;
   raising the lane limit allows other engine-lane work to overlap, not two
   concurrent engine runs. If `tryMarkRunStart` returns false after lane
   admission, the implementation must release the lane slot before returning the
   existing concurrency error (`run already in progress` today or its typed
   equivalent). This is a defensive bypass/concurrency failure, not a lane
   shutdown condition, and must not be mapped to `ErrLaneShuttingDown` unless
   the daemon context is actually closing. The path should log/record a
   defensive warning and must not wedge the lane.
7. Preserve non-blocking producer semantics:
   - processing-loop entity refresh remains fire-and-forget after a run
   - downloader health-transition refresh remains fire-and-forget
   - operator-triggered entity rebuild/refresh returns queued/accepted instead
     of waiting for a lane slot inside an HTTP request
8. Replace direct heavyweight entity background goroutine starts with lane
   submission. Preserve feed-name coalescing before queue admission. The entity
   refresh drain loop is one engine-lane item for a bounded wave: it drains the
   pending set captured when the worker starts, plus at most a bounded number of
   extra pending waves or a bounded elapsed-time budget. If producers keep
   adding work, the drain loop re-submits/coalesces a follow-up lane item and
   releases its slot so integrity/reload/cleanup work can make progress at the
   default one-worker limit. Internal lowercase workers such as
   `refreshEntityArtifactsForFeedUpdates` and health-transition refresh helpers
   run under an already-acquired lane slot and must not acquire or submit to the
   engine lane again. Public queue entrypoints such as
   `QueueEntityArtifactsRebuild` and queued feed/health refresh methods use
   non-blocking `Submit`, not blocking `Run`, so HTTP handlers and scheduler
   producers return queued/coalesced status without waiting for a lane slot.
   Queue entrypoints return typed queue state, including queued/coalesced/error
   outcomes. A queueing error maps to HTTP 5xx; "already queued" or coalesced
   maps to accepted/in-progress metadata and never falls back to a live scan.
   The concrete fairness bound is: drain the captured pending set plus at most
   one additional pending wave, or stop earlier when 60 seconds have elapsed.
   If work remains after either bound, submit/coalesce a follow-up work item and
   release the slot. Each follow-up work item uses the same one-extra-wave or
   60-second bound, so continuous producers create a bounded series of lane
   items rather than one unbounded slot holder. `QueueEntityArtifactsRebuild`,
   `QueueEntityArtifactsRefreshForFeedUpdates`, and the health-refresh queue
   entrypoint must all return a typed result such as
   `EntityArtifactQueueResult { Ticket LaneTicket; Queued bool; Coalesced bool;
   State LaneWorkState }` plus an `error`, so HTTP code can distinguish
   accepted/coalesced work from submission failure.
   Current return shapes differ: `QueueEntityArtifactsRebuild` returns `bool`,
   while `QueueEntityArtifactsRefreshForFeedUpdates` and
   `QueueEntityArtifactsRefreshForHealthTransitions` currently return no value.
   All three must return typed queue state plus `error`. Update production
   callers at `pkg/scheduler/processing_loop.go:102` and
   `pkg/scheduler/download_loop.go:27` to observe/log queue errors without
   blocking those loops. Scheduler loop callers log the error and continue;
   they must not fall back to direct goroutines or live scans when queue
   submission fails, because that would bypass the engine lane.
   If `entityArtifactsNeedBootstrapFast()` causes a bootstrap rebuild inside an
   admitted refresh wave, the 60-second fairness budget applies to total
   drain-loop wall-clock time but does not interrupt an already-running
   bootstrap rebuild midway. When that bootstrap wave finishes, remaining work
   is re-submitted/coalesced as a follow-up lane item before the slot is
   released.
9. Route startup/reload entity checks, entity repair, delayed publish-stage
   cleanup, reload-time critical-infrastructure cleanup, operator-triggered
   rebuild, explicit integrity refresh, and integrity repair through the engine
   lane. Startup/reload checks use blocking `Run` from their startup goroutine so
   the existing startup completion channel closes only after actual work
   completes. Pre-server cleanup that runs before the lane and scheduler exist
   may remain direct only when it cannot compete with live HTTP/watchdog work.
   `prepareEngineForRun` pre-server calls to `CleanupStalePublishStages`,
   startup pipeline recovery, and startup critical-infrastructure cleanup remain
   direct because the web server, scheduler, and watchdog are not yet competing.
   The current startup pipeline-integrity recovery scan remains a synchronous
   pre-server/pre-scheduler step outside the engine lane because it feeds the
   scheduler initial recovery actions before live daemon work begins. It must not
   be moved into a request-time or concurrent runtime path by this SOW, and its
   scan result must populate the pipeline integrity cache as the first settled
   report when it succeeds.
   Delayed publish-stage cleanup that fires after the daemon is serving keeps the
   existing delayed goroutine shape, but after the timer fires that goroutine
   calls blocking lane `Run` and waits its turn. It does not skip solely because
   the lane is busy, and it must respect daemon context cancellation.
   SIGHUP/runtime reload is different from pre-server startup: `Engine.Reload`
   keeps synchronous only the config/runtime operations that later processing
   depends on, including config parse/apply, runtime resolution, ledger cache
   reset, entry reconciliation, retention-window rebuild, entry timestamp
   repair, legacy failure bootstrap, provider-set ID refresh, and lane
   `SetLimit`. It must release `e.mu` before submitting reload-time cleanup to
   the engine lane. Cleanup submission returns after accept/coalesce and does
   not wait for the cleanup slot to execute. During the short window before
   cleanup finishes, public/admin views may still observe stale
   critical-infrastructure artifacts; this is an accepted availability trade-off
   and must be visible through lane/background status. The reload handler in
   `cmd/update-ipsets/daemon.go:92-99` must also replace the direct
   `EnsureEntityArtifactsCurrentWithTrigger(ctx, "reload")` goroutine with a
   typed non-blocking engine-lane submission visible in admin status.
   Duplicate reload cleanup submissions with the same typed coalescing key
   coalesce. The second submit returns the existing ticket with
   `LaneTicket.Coalesced == true` and does not create another cleanup worker.
   Use a stable coalescing key such as `cleanup:critical_infrastructure:reload`
   for reload cleanup and a separate stable key for SIGHUP entity checks.
   If the daemon somehow receives SIGHUP before the lane has been initialized,
   the handler must log/record the skipped submit and must not fall back to a
   direct heavy goroutine. In normal construction the lane exists before daemon
   signal handling starts.
   `EnsureEntityArtifactsCurrentWithTrigger` is split or wrapped so the live
   check+repair body has an admitted helper and startup can block on `Run` while
   SIGHUP can use `Submit` without starting a direct heavy goroutine. The
   admitted helper wraps the existing live check plus
   `repairEntityArtifactsWithPlan` / `RepairEntityArtifactsWithPlan` behavior;
   do not duplicate a second repair implementation with different artifact
   semantics.
   `CleanupStaleCriticalInfrastructureArtifacts` remains callable directly for
   pre-server `prepareEngineForRun`; reload-time use wraps the same helper in
   typed lane work. A reload-time cleanup failure does not make `Engine.Reload`
   fail after the submit has been accepted; it is recorded through lane task
   failure state and logs.
   Current `Engine.Reload()` uses a deferred unlock around the main reload
   critical section (`pkg/engine/engine.go:245-249`). The implementation must
   replace that broad deferred-unlock shape with an explicit lock scope: collect
   stale ASN lookup handles and typed cleanup work while holding `e.mu`, unlock,
   call `closeASNLookupDatabases` after the unlock as today, then call
   non-blocking lane `Submit` outside the `e.mu` scope. Re-lock only for small
   post-submit state updates if absolutely required. Lane `Submit` must never be
   called while `e.mu` is held. The captured stale ASN lookup handles are closed
   after `e.mu.Unlock()` and before any reload cleanup lane `Submit`, preserving
   the old close-before-cleanup ordering without holding the engine mutex.
10. Preserve entity publication serialization with `entityArtifactsMu` or an
    equivalent critical section even if `max_engine_lane_workers` is raised above
    one. A lane worker waiting on `entityArtifactsMu` still occupies its engine
    lane slot; raising `max_engine_lane_workers` may improve overlap with
    unrelated engine-lane work, but entity publication concurrency remains one.
    The engine lane is not the only protection for entity artifact publication.
11. Change admin integrity semantics for both pipeline integrity and entity
    integrity:
    - GET never runs `CheckIntegrityWithOptions` or
      `CheckEntityArtifactsIntegrity` directly
    - GET returns `in_progress` when a relevant engine-lane scan/reprocess/rebuild
      is queued or running
    - GET returns the last settled cached report when available, clearly labeled
      as last-settled with `checked_at`, `generation`, and
      `cache_state` (`cold`, `fresh`, `stale`, `refresh_queued`,
      `refresh_running`)
    - if the cache is cold, GET returns `in_progress` with `cache_state: cold`
      instead of scanning live files
    - startup/reload checks and successful explicit refresh scans populate the
      relevant cache
    - entity mutation or pipeline mutation marks affected cached reports stale;
      stale cached findings may be returned only with clear last-settled/stale
      labeling
    - `POST /api/v1/admin/integrity/refresh` queues a pipeline-integrity scan
      through the engine lane and returns `202 Accepted` with queued status
    - `POST /api/v1/admin/integrity/entities/refresh` queues an entity-integrity
      scan through the engine lane and returns `202 Accepted` with queued status
    - the refresh routes above are new additive routes, not renames; existing
      `POST /api/v1/admin/integrity/reprocess` and
      `POST /api/v1/admin/integrity/entities/rebuild` remain supported
    - existing rebuild/reprocess actions remain rebuild/reprocess actions and
      queue their work through the engine lane
    - when rebuild/reprocess is already queued, handlers return `in_progress` or
      accepted status without falling back to a live GET scan
    - `handleAdminEntityIntegrityRebuild` must remove the current fallback from
      `QueueEntityArtifactsRebuild == false` to `buildEntityIntegrityReport`;
      already-queued rebuild state returns queued/in-progress/cached metadata
      only. Queue submission errors return HTTP 5xx and must not be collapsed
      into an in-progress response.
      Successful/accepted rebuild responses keep the existing
      `entityIntegrityActionResult` compatibility shape and add typed fields:
      `status`, `cache_state`, `queued`, `coalesced`, and `ticket` when a ticket
      is available. The `ticket` object contains stable typed lane fields only.
    - `handleAdminIntegrityReprocess` must remove the current request-time call
      to `buildIntegrityReport`; it uses a cached settled pipeline report to
      plan recovery. A stale but usable cached report may be used to plan
      recovery only when the response labels `cache_state: stale`; this avoids a
      live scan while making the operator-visible freshness explicit. If the
      cache is cold or unusable it queues a refresh and returns HTTP
      `202 Accepted` with
      `{"status":"in_progress","cache_state":"refresh_queued","queued":true,"coalesced":false}`
      or the same shape with `coalesced:true` when a compatible refresh is
      already queued/running, rather than scanning live files
    - `buildIntegrityReport` is removed or narrowed to an engine-lane scan
      helper. GET handlers and reprocess handlers read from the pipeline
      integrity cache; the live `CheckIntegrityWithOptions` call is allowed only
      inside engine-lane refresh/startup scan workers.
    - `buildEntityIntegrityReport` is removed or narrowed to an engine-lane scan
      helper, matching `buildIntegrityReport`; GET handlers read from the entity
      integrity cache and never call `eng.CheckEntityArtifactsIntegrity()`
      directly.
    - `handleAdminIntegrityReprocess` reads findings through an
      `eng.PipelineIntegrityCache()` or equivalent snapshot method. The cache
      stores `[]engine.IntegrityFinding` so the existing
      `eng.IntegrityRecoveryPlan` path can compute recovery targets from cached
      findings. If a stale cache is used, the response must include
      `cache_state: stale` and a warning field or message naming the refresh
      endpoint so the operator knows reprocess used last-settled data.
    - replace current background-task-name prefix heuristics with typed lane state
      and typed integrity cache state
12. Preserve a separate downloader FIFO. The current downloader queue selects by
    `QueuedAt` and name from a map; implementation must add a monotonic enqueue
    sequence or equivalent stable tie-break so equal-time items are true FIFO,
    not name-order priority. Add the sequence to `queuedWork` or equivalent
    queue state, assign it in `enqueueDownloadLocked`, preserve the earliest
    sequence when queue entries merge, and use `QueuedAt`, sequence, then name
    for deterministic ordering in dispatch and snapshots. Downloader-stage local
    composition, including CPU-heavy history-derivative set algebra, stays in
    the downloader FIFO and does not acquire the engine lane because the user
    selected option `3A`.
    The concrete field contract is `queuedWork.EnqueueSeq uint64` or equivalent,
    assigned from a per-`Runner` monotonic counter such as
    `Runner.downloadEnqueueSeq`. `mergeQueuedWork` must preserve the lowest
    sequence from merged entries.
    Add a typed work-kind marker to `queuedWork`, for example
    `Kind queuedWorkKind` with string values `normal`, `recovered_artifact`, and
    any existing special cases needed locally. Recovered DroneBL staged-artifact
    work uses `Kind: recovered_artifact` so dispatch, snapshots, tests, and
    status can distinguish it from a normal due download without parsing names.
13. Preserve DroneBL downloader ownership. All DroneBL acquisition and child
    materialization must flow through the scheduler download queue. The normal
    due path already does this, but staged artifact recovery currently does not:
    `RecoverStagedArtifacts` materializes children directly. Replace that
    recovery bypass with downloader-lane recovery work. The recovery goroutine
    performs lightweight discovery only, then enqueues a typed recovered-artifact
    download item ahead of normal due downloads. The downloader worker calls a
    recovered-artifact engine helper that materializes children from the existing
    staged artifact file and returns the normal `DownloadDecision`; `runDownload`
    then enqueues child processing exactly as it does for normal artifact
    downloads. Recovery must not refetch the remote artifact solely to satisfy
    the queue contract, must not wait for downloader slots in the recovery
    goroutine, and must not materialize DroneBL children in the recovery
    goroutine. The recovery discovery API should return recovered artifact
    identities to enqueue, not child processing names; for example,
    `RecoverStagedArtifacts` may become a discovery helper returning
    `[]RecoveredArtifact` or `[]string` artifact names plus error, while a new
    downloader-worker helper performs materialization and returns
    `DownloadDecision`.
    Recovery discovery/enqueue must complete before the first normal due-cycle
    enqueue after scheduler startup. Implement this by running staged-work
    recovery discovery synchronously in `Runner.Run` before starting
    `runFetchLoop`; the same startup sequencing must not allow a normal
    artifact due enqueue to race ahead of recovered-artifact enqueue. Recovery
    discovery may enqueue recovered-artifact download items but must not perform
    heavy DroneBL materialization itself. The processing loop may start after
    this discovery step using the existing queue wake mechanics; the hard
    ordering requirement is that recovered artifact download items are visible
    in the downloader queue before normal artifact due work is enqueued. The
    recovered helper must use the scheduler's `r.enableAll` policy, not an
    unconditional `enableAll=true`, so recovery does not enable
    operator-disabled child feeds. If the staged DroneBL artifact fails with a
    corruption-class or unreadable-format error, the downloader worker records
    the failed recovery, removes the corrupt staged artifact from the active
    staged-work path, and enqueues a normal due-cycle fetch for that artifact.
    If the implementation keeps evidence instead of deleting it, it must rename
    the file under the same staging directory with a `.corrupt` suffix and write
    a JSON sidecar named `<renamed-file>.json` containing only `name`,
    `artifact`, `corruption_class`, and `timestamp`. The renamed file and
    sidecar must not be rediscovered as pending staged work on restart. This is
    the only recovery case where a staged recovery failure may lead to remote
    refetch. Transient I/O, context cancellation, or resource errors do not
    remove the staged artifact; they leave it for retry.
14. Keep the existing `BeforePublish` promotion handoff inside the admitted
    engine run. The callback may promote already-staged committed downloads
    because it is part of publishing the processed batch. It must not grow into
    downloader fetches, DroneBL child materialization, broad scans, or any other
    work that belongs in the downloader FIFO.
15. Keep public cached serving, `/healthz`, watchdog notify, and cheap admin
    status outside the download and engine lanes. These paths must observe lane
    state but not acquire a lane slot, run live integrity scans, or hold engine
    locks around heavy work. The default
    `GET /api/v1/admin/status` response is the lightweight status payload for
    frequent UI polling. The full status payload remains available only through
    explicit `GET /api/v1/admin/status?mode=full`. Missing `mode`,
    `mode=light`, and unrecognized mode values use the lightweight builder. The
    admin UI must use the light/default path for high-frequency polling and
    while the engine lane is active. The existing full admin status payload may
    remain for compatibility behind `?mode=full`, but it must not be the hot
    polling path during heavy work.
16. Admin status/UI must keep the existing four top live lists for downloader
    waiting/active and processing waiting/active. Engine-lane active/waiting work
    appears in the existing background/operations status area or its spec-updated
    successor, not as extra top live lists.
17. Admin status JSON contract:
    - add a typed `engine.engine_lane` snapshot with limit, active count, waiting
      count, active work, waiting work, and wait durations. The existing admin
      status response nests engine state under `engine`; do not move these
      fields to a new top-level `engine_lane` without a separate compatibility
      decision.
    - add the typed lane snapshot fields to `engine.StatusSnapshot`
      (`pkg/engine/types.go:110`) and populate them through
      `StatusSnapshotLight`; `pkg/web/admin.go:523` inside `buildAdminStatus`
      should not synthesize lane state by parsing background task names
    - add typed `PipelineIntegrityCache` and `EntityIntegrityCache` snapshot
      substructures to `engine.StatusSnapshot` or the equivalent admin-status
      source object, each carrying `generation`, `cache_state`, `checked_at`,
      queued/running/coalesced flags, and stale/error metadata
    - keep existing `background_limit` and `background_running` fields as
      backward-compatible aliases under `engine.background_limit` and
      `engine.background_running` for engine-lane limit and active count
    - keep `engine.max_background_workers` as the background/entity fan-out
      setting
    - add `engine.max_engine_lane_workers` to status payloads
    - populate `BackgroundTasks` from `withEngineLaneBackgroundTask` for
      human-readable task progress while typed lane snapshots remain the source
      of truth for admission state
    - engine-lane IDs, names, stages, and details exposed via admin JSON must be
      stable operator labels. They must not include filesystem paths, goroutine
      IDs, pointer values, raw panic text, or unredacted internal error strings.
18. Coalescing and internal flags:
    - keep entity refresh/health pending maps and running flags only as
      coalescing state before engine-lane submission
    - replace rebuild "already queued/running" string-name checks with typed
      rebuild lane/coalescing state
    - introduce or keep one `entityArtifactFullRebuildQueuedOrRunning` gate if
      useful, but it must inspect typed engine-lane active/waiting work and
      explicit coalescing fields, not background task display-name prefixes
    - replace `entityIntegrityBusy` and `entityBackgroundTaskRunning` with typed
      lane/cache state. No admin integrity decision may depend on
      `strings.HasPrefix(task.Name, "Entity artifacts ")`.
    - track rebuild coalescing with explicit fields such as
      `entityRebuildQueued`, `entityRebuildLaneActive`, or a stored
      `LaneTicket`; task display names are not state
    - do not use task display names as semantic state
    - migration order for rebuild coalescing is mandatory: introduce typed
      lane/coalescing inspection first, switch
      `tryMarkEntityArtifactFullRebuildQueued` and
      `entityArtifactFullRebuildQueuedOrRunning` to that typed state, then
      migrate background task display names. No intermediate state may depend on
      `backgroundTaskNamedLocked("Entity artifacts rebuild")` or any display
      name for correctness.
    - required coalescing keys:
      `entity:rebuild:full` for full entity rebuilds;
      `entity:refresh:feed_updates:continuation:0` and
      `entity:refresh:feed_updates:continuation:1` for queued feed-update
      refresh waves;
      `entity:refresh:health:continuation:0` and
      `entity:refresh:health:continuation:1` for queued health-transition
      refresh waves;
      `entity:repair:startup` and `entity:repair:operator` or equivalent
      trigger-scoped keys for entity repair;
      `integrity:pipeline:refresh` for pipeline integrity refresh scans;
      `integrity:entity:refresh` for entity integrity refresh scans;
      `entity:integrity:reload` for SIGHUP entity checks;
      `cleanup:critical_infrastructure:reload` for reload critical-infrastructure
      cleanup; and `cleanup:publish_stages:delayed` for delayed publish-stage
      cleanup. Entity refresh continuation keys deliberately alternate between
      the `:0` and `:1` suffixes so a queued continuation cannot coalesce with
      the lane item that is still completing. Implementations may add narrower
      suffixes, but they must not coalesce incompatible work families.
19. Runtime reload updates engine-lane limit dynamically. Lowering the limit does
    not cancel active work; it prevents new starts until active work is below the
    new limit. For example, lowering the limit from `4` to `1` while `3` items
    are active starts no new work until the active count reaches `0`; then FIFO
    admission resumes one item at a time. Raising the limit admits queued work in
    FIFO order immediately.
20. Preserve background metric continuity for existing metric names:
    `background.tasks`, `background.worker.wait`,
    `background.workers.active`, and `background.workers.limit`. New
    `engine.lane.*` metrics may be added alongside them, but existing names must
    not silently disappear. After engine-lane migration,
    `background.workers.active` and `background.workers.limit` are emitted from
    the engine-lane snapshot as compatibility aliases. The old
    `backgroundLimiter` must not emit these metrics after migration.
    `background.worker.wait` is emitted from the engine-lane queue wait duration
    when work starts. `background.tasks` keeps the existing result labels. The
    legacy label `background.component` must be derived from typed `LaneWork`
    component values first, falling back to `Kind` only if `Component` is empty;
    it must never parse display-name prefixes. Required mapping:
    `engine_run` -> `engine_run`; `entity_artifacts` -> `entity_artifacts`;
    `entity_artifacts_health` -> `entity_artifacts_health`;
    `entity_integrity` and `pipeline_integrity` -> `integrity`;
    `critical_infrastructure` and `publish_stages` -> `cleanup`;
    `integrity_refresh` / `integrity_reprocess` fallback kind -> `integrity`;
    `cleanup` fallback kind -> `cleanup`; unknown/empty -> `other`.
    This intentionally changes some legacy `background.component="other"` label
    values to more specific typed values. Operator docs/release notes must call
    this out for dashboards or alerts that group by `background.component`.
21. Update specs, project skills, tests, and operator docs before closure.

Detailed implementation contracts:

### Engine Lane FIFO, Re-Entrancy, And Shutdown

- The engine lane owns one FIFO queue and one active set. Every work item gets a
  monotonic sequence number at enqueue time.
- With limit `1`, sequence `N+1` cannot start before sequence `N` completes,
  errors, panics, or is canceled while still queued.
- With limit `2`, the first two non-canceled queued items may start, and later
  items start in sequence order as slots free.
- A `Submit` call made from inside an active lane worker is non-blocking. It may
  enqueue/coalesce work, but it must not wait for that work to run. Active work
  that truly needs an internal helper must call the helper directly rather than
  submit-and-wait through the lane.
- A blocking `Run` call made from inside an active worker of the same lane is a
  programming error. The implementation must detect this with lane context
  state or equivalent and fail fast instead of deadlocking.
- Lane shutdown uses the daemon context: new submissions are rejected after
  shutdown starts, queued work is canceled or marked skipped, and in-flight work
  receives cancellation through its context and exits through existing safe-stop
  paths. Shutdown must not leave the queue locked or a slot permanently held.
  Submit-after-shutdown returns typed `ErrLaneShuttingDown` or equivalent. HTTP
  handlers map that error to `503 Service Unavailable`; internal producers log
  and drop/coalesce according to their existing cancellation semantics.
  Shutdown sequence is deterministic: first close admission and mark queued
  items `skipped`/`canceled` without starting them, then cancel active work
  contexts, then wait up to 30 seconds for active work to leave its callbacks.
  If active work has not exited after that grace period, shutdown returns/logs a
  timeout but still releases lane bookkeeping locks and lets process-level
  shutdown continue.
- Panic/error handling must always release the slot before recording failure,
  logging, returning, or re-panicking.
- `SetLimit` accepts the new limit atomically and applies it to future
  admissions only. In-flight work is never canceled by lowering the limit. If
  active work is above the new lower limit, no new work starts until active count
  drops below the new limit. If active count equals the new limit, no new item
  starts until one active item exits and the count becomes lower than the limit.
- Lane snapshots used by free-path status handlers may take only a short mutex
  read of in-memory lane state. They must not wait for a lane slot, call worker
  callbacks, or hold the lane mutex while doing JSON encoding, filesystem work,
  or engine status aggregation.
- The lane mutex protects only lane state and is never held while executing a
  worker callback. A lane worker may occupy a lane slot while waiting on
  `entityArtifactsMu`, but it must not hold the lane mutex at the same time.
  Engine reload and status paths must avoid acquiring `e.mu` and then blocking
  on a lane operation that can call back into engine state.
  Snapshot assembly must never hold `e.mu` while acquiring the lane mutex. Use
  independent short snapshots (lane first, then engine, or engine first then
  lane) only when neither lock is held while acquiring the other.

### Reload And SIGHUP Contract

- `Engine.Reload()` remains responsible for synchronous config reload,
  runtime resolution, and engine-lane `SetLimit`.
- The synchronous portion of `Engine.Reload()` includes only state that later
  scheduler/engine work depends on immediately: parsed config, runtime, ledger
  cache reset, entry reconciliation, retention max-window rebuild, invalid
  timestamp repair, `bootstrapLegacyFailureStarts` legacy failure bootstrap,
  provider-set ID refresh, and lane limit update.
- `Engine.Reload()` must not submit or wait for lane work while holding `e.mu`.
  If cleanup work needs to be queued after reload, collect the needed typed work
  description while protected by `e.mu`, release `e.mu`, then call lane
  `Submit`. Any stale ASN lookup handles captured during reload are closed after
  unlocking and before cleanup submission.
- Reload-time heavy cleanup is not executed inline after the daemon is serving.
  `Engine.Reload()` submits typed cleanup work to the engine lane and returns
  after the work is accepted or coalesced. Completion and failure are visible via
  background task state and engine-lane status.
- Until queued reload cleanup completes, stale critical-infrastructure artifacts
  may still be visible. That is accepted for availability, but the queued/running
  cleanup must be visible through admin status.
- `Engine.Reload()` may return an error if it cannot parse/apply config or if
  the daemon context is already canceled before cleanup submission. It must not
  block for minutes waiting for an engine-lane slot.
- The SIGHUP handler in `cmd/update-ipsets/daemon.go` must not start direct
  heavy goroutines after reload. Its reload entity check is submitted to the
  engine lane as typed `entity.integrity.reload` or equivalent work and is
  visible in admin status.
- Pre-server startup cleanup in `prepareEngineForRun` remains direct because it
  happens before live HTTP/watchdog/scheduler competition exists.
- If reload changes the effective published web directory used by pipeline
  integrity checks, existing pipeline-integrity cache scopes for the old web
  directory are invalidated or discarded under `e.mu`. A later GET/reprocess for
  the new web directory must see a cold or queued matching scope, not a fresh
  report computed for the previous directory.

### Integrity Cache Lifecycle And HTTP Contract

- The implementation maintains separate in-memory last-settled caches on
  `Engine` for pipeline integrity and entity integrity. This SOW does not
  require a persisted cache file, so restart state is `cold` until a startup or
  explicit refresh scan completes.
- Integrity caches are protected by `e.mu`. Cache readers take only a short
  snapshot under `e.mu` and never call live scan functions while holding it.
  Cache setters/stale markers also run under `e.mu` and must not acquire the
  lane mutex while holding `e.mu`. If a mutation path already holds `e.mu`, it
  must update cache fields directly or call a locked-helper variant; it must not
  call a public cache method that reacquires `e.mu`.
- Required cache snapshot fields are `generation`, `cache_state`, `checked_at`,
  `queued`, `running`, `coalesced`, `startup_scan_running` where applicable,
  `last_error` where applicable, and the cached findings slice. Pipeline cache
  findings are `[]IntegrityFinding`; entity cache findings are
  `[]EntityIntegrityFinding` or the existing public entity-integrity report row
  type if that is the local contract. Canonical `cache_state` string values are
  `cold`, `fresh`, `stale`, `refresh_queued`, and `refresh_running`; specs,
  backend JSON, frontend types, fixtures, and tests must use these exact values.
  Snapshot structs must be explicit, with snake_case JSON tags matching the
  admin API:

  ```go
  type PipelineIntegrityCacheSnapshot struct {
      Scope              PipelineIntegrityCacheScope `json:"scope,omitempty"`
      Generation         uint64                      `json:"generation"`
      CacheState         string                      `json:"cache_state"`
      CheckedAt          time.Time                   `json:"checked_at,omitempty"`
      Queued             bool                        `json:"queued"`
      Running            bool                        `json:"running"`
      Coalesced          bool                        `json:"coalesced,omitempty"`
      StartupScanRunning bool                        `json:"startup_scan_running,omitempty"`
      LastError          string                      `json:"last_error,omitempty"`
      Findings           []IntegrityFinding          `json:"findings,omitempty"`
  }

  type EntityIntegrityCacheSnapshot struct {
      Generation         uint64                   `json:"generation"`
      CacheState         string                   `json:"cache_state"`
      CheckedAt          time.Time                `json:"checked_at,omitempty"`
      Queued             bool                     `json:"queued"`
      Running            bool                     `json:"running"`
      Coalesced          bool                     `json:"coalesced,omitempty"`
      StartupScanRunning bool                     `json:"startup_scan_running,omitempty"`
      LastError          string                   `json:"last_error,omitempty"`
      Findings           []EntityIntegrityFinding `json:"findings,omitempty"`
  }
  ```

  If the existing public entity-integrity report row type is used instead of
  `EntityIntegrityFinding`, the field name and JSON key stay `findings`; only
  the Go element type changes.
- Pipeline integrity cache is scoped by scan options that change findings. Define
  `PipelineIntegrityCacheScope` or equivalent with `IncludeArchived bool`,
  `EnableAll bool`, and the effective `WebDir` identity used by
  `CheckIntegrityWithOptions`. Keep separate cached snapshots per scope (at
  most the small set of boolean combinations used by admin/API callers), or an
  equivalent keyed map. A GET/reprocess request may use only a cache snapshot
  whose scope matches its requested `include_archived`, `enableAll`, and web-dir
  scope. If no matching settled snapshot exists, it queues a refresh for that
  exact scope and returns queued/cold status instead of serving a mismatched
  "fresh" report. A runtime reload that changes the effective `WebDir` drops or
  marks stale old pipeline cache scopes before serving status for the new scope.
  Entity integrity cache has no `include_archived` / `enableAll` scope today.
- Required engine methods or equivalents:
  `StorePipelineIntegrityFindings(scope PipelineIntegrityCacheScope, findings []IntegrityFinding, checkedAt time.Time)`,
  `MarkPipelineIntegrityCacheStale(reason string)`,
  `PipelineIntegrityCache(scope PipelineIntegrityCacheScope) PipelineIntegrityCacheSnapshot`,
  `StoreEntityIntegrityFindings(findings []EntityIntegrityFinding, checkedAt time.Time)`,
  `MarkEntityIntegrityCacheStale(reason string)`, and
  `EntityIntegrityCache() EntityIntegrityCacheSnapshot`. HTTP handlers may call
  snapshot methods only; cache population methods are called by startup scans,
  engine-lane refresh workers, and repair/re-scan workers.
- On daemon start the caches are `cold`. A cache becomes `fresh` only after a
  successful startup/reload check or explicit queued refresh scan completes.
- Mutations that can change the related artifact family mark the related cache
  `stale` immediately. Stale data may be served only as clearly labeled
  last-settled data; it must not be presented as current.
- A queued scan sets `cache_state: refresh_queued`; an active scan sets
  `cache_state: refresh_running`.
- `GET /api/v1/admin/integrity` and
  `GET /api/v1/admin/integrity/entities` never call live scan functions. Their
  allowed responses are cached settled data, stale cached settled data, or
  in-progress/cold status.
- The cache token exposed to callers is `generation`, an incrementing uint64 per
  cache. The word "token" in older discussion resolves to `generation` for this
  implementation.
- Entity integrity cache invalidation is whole-cache and conservative: any
  entity artifact mutation, entity artifact rebuild, entity publish generation
  bump, or entity integrity repair marks it stale.
- Pipeline integrity cache invalidation is whole-cache and conservative: any
  successful feed output mutation, recheck/reprocess run completion, staged
  publish promotion, `applyRenamesAndDeletes` mutation, or pipeline integrity
  repair marks it stale.
- Required invalidation/population hooks include, at minimum:
  - entity cache stale on `bumpEntityArtifactGenerationLocked`,
    `publishEntityArtifactMutationPlan`, entity rebuild, entity repair, and
    optimistic entity artifact mutation success.
  - entity cache fresh from successful entity-integrity refresh scans and from
    repair-path re-scans that call `CheckEntityArtifactsIntegrity`.
  - pipeline cache stale on successful `RunOnce` publication,
    `publishRunArtifacts`, staged publish promotion, recovery recheck/reprocess
    scheduling completion, and any pipeline-integrity repair action.
  - pipeline cache fresh from startup `queueStartupIntegrityRecovery` scans and
    explicit pipeline-integrity refresh scans.
- Mutation point inventory:
  - entity cache stale hooks must be added at
    `bumpEntityArtifactGenerationLocked`, `publishEntityArtifactMutationPlan`,
    `RebuildEntityArtifactsWithTrigger`, `repairEntityArtifactsWithPlan`, and
    successful optimistic entity artifact mutation paths.
  - pipeline cache stale hooks must be added at successful `publishRunArtifacts`
    / `RunOnce` publication, `PromoteCommittedDownloads` / staged publish
    promotion, successful `applyRenamesAndDeletes` mutation, recovery
    recheck/reprocess scheduling completion, and any pipeline-integrity repair
    action.
  - `queueStartupIntegrityRecovery` must call an engine cache setter after
    `CheckIntegrityWithOptionsContext` succeeds, storing findings as
    `cache_state: fresh` with an incremented `generation`. A successful startup
    scan with zero findings also stores an empty findings list as `fresh`;
    success with no findings must not leave the cache `cold`.
- While the startup integrity scan is running and before its cache setter
  commits a settled report, request-time GET responses expose cold/in-progress
  state. They may include a typed `startup_scan_running` or equivalent boolean,
  but they must not start another live scan. In the current startup flow the
  startup scan runs before server listeners start, so this flag is primarily
  forward-compatible for tests or future startup-order changes.
- `POST /api/v1/admin/integrity/refresh` queues a pipeline-integrity scan in the
  engine lane. It returns `202 Accepted` when queued or coalesced, with JSON
  fields `status`, `cache_state`, `queued`, `coalesced`, and a lane ticket or
  stable work identifier when available.
- `POST /api/v1/admin/integrity/entities/refresh` queues an entity-integrity scan
  with the same response contract.
- `pkg/web/routes.go` registers GET method-not-allowed handlers for both refresh
  routes, matching the existing rebuild/reprocess action-route pattern.
- `pkg/web/server.go:160` route normalization/telemetry allowlist adds both
  refresh routes so admin spans and client-error logging use stable route names.
- These refresh routes are additive. They do not replace existing operator
  actions:
  - `POST /api/v1/admin/integrity/reprocess` still schedules recheck/reprocess
    recovery, but plans from cached settled pipeline findings.
  - `POST /api/v1/admin/integrity/entities/rebuild` still schedules entity
    artifact rebuild, but reports typed queued/running state when coalesced.
- `POST /api/v1/admin/integrity/reprocess` uses the cached settled pipeline
  report to compute recovery targets. If the cache is cold or unusable, it
  queues pipeline-integrity refresh and returns `202 Accepted` with
  `status: in_progress`, `cache_state: refresh_queued`, and no live scan.
  If the cache scope does not match requested `include_archived`, `enableAll`,
  or web-dir scope, this is treated as cold for that scope and queues a matching
  refresh.
- `POST /api/v1/admin/integrity/entities/rebuild` queues or coalesces entity
  rebuild work. If rebuild work is already queued/running, it returns
  queued/in-progress metadata and must not call the entity live scan fallback.

### Admin Status JSON Contract

Admin status must expose a typed engine-lane snapshot without breaking existing
consumers:

`Engine.StatusSnapshotLight()` or equivalent returns a dedicated lightweight
snapshot type, not a full `StatusSnapshot` with expensive fields zeroed after
the fact:

```go
type StatusSnapshotLight struct {
    Running                bool
    Phase                  string
    LastReason             string
    LastStarted            time.Time
    LastEnded              time.Time
    SourceCount            int
    MergeCount             int
    MaxEngineLaneWorkers   int
    MaxBackgroundWorkers   int
    EngineLane             LaneSnapshot
    BackgroundLimit        int
    BackgroundRunning      int
    PipelineIntegrityCache []PipelineIntegrityCacheSnapshot
    EntityIntegrityCache   EntityIntegrityCacheSnapshot
}

type LaneSnapshot struct {
    Limit        int
    ActiveCount  int
    WaitingCount int
    Active       []LaneWorkSnapshot
    Waiting      []LaneWorkSnapshot
}

type LaneWorkSnapshot struct {
    ID        string
    Kind      LaneWorkKind
    Component LaneWorkComponent
    Name      string
    Trigger   string
    Phase     string
    Stage     string
    State     LaneWorkState
    QueuedAt  time.Time
    StartedAt time.Time
    WaitMS    int64
    ElapsedMS int64
}
```

The light snapshot includes only in-memory run state, engine-lane state,
background compatibility aliases, and cached integrity summaries. It omits
runtime/proc detail, per-feed arrays, artifact arrays, filesystem-derived
integrity findings not already cached, and any field that requires feed/artifact
walking or entity JSON decoding. The web layer's light-status builder wraps this
engine snapshot with scheduler queue summaries, watchdog/health state, and
optional cached runtime sampler values. The implementation must add explicit
snake_case JSON tags to these structs; for example `MaxEngineLaneWorkers`
serializes as `max_engine_lane_workers`, `EngineLane` as `engine_lane`, and
`WaitMS` / `ElapsedMS` as `wait_ms` / `elapsed_ms`.

Caller routing after this type split is explicit:

- `pkg/web/public_status.go` uses `StatusSnapshotLight` and keeps the cheap
  public fields it already needs: `running`, `last_started`, `last_ended`,
  `source_count`, and `merge_count`.
- `pkg/scheduler/automatic_due.go` and `pkg/scheduler/recovery.go` use
  `StatusSnapshotLight().Running` only.
- `pkg/web/integrity.go` stops using the light status snapshot as a background
  task heuristic; it reads typed lane and integrity-cache snapshots instead.
- `pkg/web/admin.go` full `buildAdminStatus` uses the existing full
  `Engine.StatusSnapshot()` or equivalent full-status source, not the light
  type, so the default/full response remains backward compatible.
- Existing tests that call `StatusSnapshotLight()` are updated to assert the
  light contract or moved to full status when they need full fields.

```json
{
  "engine": {
    "max_engine_lane_workers": 1,
    "max_background_workers": 1,
    "engine_lane": {
      "limit": 1,
      "active_count": 1,
      "waiting_count": 2,
      "active": [
        {
          "id": "engine-run:2026-06-22T00:00:00Z",
          "name": "engine.run",
          "phase": "metadata",
          "stage": "compare",
          "started_at": "2026-06-22T00:00:00Z",
          "elapsed_ms": 1200
        }
      ],
      "waiting": [
        {
          "id": "entity-rebuild:operator",
          "name": "entity.rebuild",
          "queued_at": "2026-06-22T00:00:01Z",
          "wait_ms": 200
        }
      ]
    },
    "background_limit": 1,
    "background_running": 1
  }
}
```

- `engine.engine_lane` is the source of truth for admission state.
- `background_limit` and `background_running` remain compatibility aliases for
  engine-lane limit and active count under the existing `engine` object.
- `engine.max_background_workers` remains visible as background/entity fan-out
  and must not be reported as the engine-lane admission limit.
- Operator docs and the config spec must explicitly warn about the compatibility
  naming trade-off: `background_limit` / `background_running` describe
  engine-lane admission in the status payload, while `max_background_workers`
  remains the separately tunable background/entity fan-out setting.

### Free-Lane Status Budget

- Free-lane status handlers may read in-memory snapshots and cached integrity
  state only.
- They must not walk output directories, decode entity detail JSON, run
  integrity checks, build comparison files, or aggregate full per-feed metrics
  on demand.
- The light status path must not call `runtime.ReadMemStats` or other
  runtime-wide sampling on each request. If memory/runtime stats are shown in
  light status, they must come from a periodically refreshed cached sample
  outside the request hot path.
- The light admin status path is a new builder function. It may read
  `StatusSnapshotLight`, lane snapshots, scheduler queue snapshots, watchdog
  state, and integrity-cache snapshots only. It must skip `runtime.ReadMemStats`,
  `/proc` reads, feed/artifact walking, feed-health aggregation, and any
  detailed per-feed rebuild work.
- Light status may omit memory/runtime statistics. If it includes them, the web
  server owns a `runtimeStatsSampler` or equivalent background sampler that
  calls `runtime.ReadMemStats` on a default 5s interval and stores a cached
  sample for request handlers to copy without runtime-wide sampling. The sampler
  starts in `web.Run` before admin polling can use light status, stops when the
  daemon/server context is canceled, and is not user-configurable in this SOW.
  Start is idempotent for a single `web.Run` / server instance, so tests or
  repeated server setup cannot leak multiple sampler goroutines for the same
  running server. Tests may inject a faster sampling cadence to verify
  cancellation without waiting for the 5s default. Tests must prove the sampler
  stops on shutdown without leaking a goroutine.
- Full admin status may continue to call `runtime.ReadMemStats` and detailed
  `/proc` helpers for compatibility because it is not the hot polling path. The
  admin UI must not use full status for high-frequency polling while heavy work
  is active; full status is for on-demand operator drill-down or slow refresh,
  not polling intervals shorter than 30 seconds.
- Use `GET /api/v1/admin/status?mode=light` for
  high-frequency UI polling. The light response contains engine lane state,
  watchdog/health-critical fields, cached queue counters, and no feed/artifact
  detail rebuild. The existing full status response may remain, but UI polling
  must not depend on rebuilding it while heavy work is active.
- The light response includes only: engine lane snapshot, compatible
  background aliases, scheduler queue counters/summaries, watchdog/health
  fields, cached integrity-cache summaries, and optional cached runtime sampler
  values. It omits full `runtime` detail, feed health aggregation, per-feed
  detail arrays, artifact detail arrays, entity JSON decoding results, and any
  filesystem-derived integrity findings not already cached.
- Behavioral tests must prove lightweight health/status responses complete while
  a synthetic engine-lane item is blocked. Timing assertions should use a loose
  bounded-response budget suitable for CI, and the stronger guarantee is the
  absence of live scans or filesystem walks in these handlers.
- Add a structural or behavioral regression test covering all free-path handlers
  (`/healthz`, public cached serving, watchdog notify path where testable, and
  light admin status) to prove they do not acquire an engine/download slot or
  call live integrity functions.

### Old Background Task Wrapper Disposition

- After migration, `withBackgroundTask` must either be removed as dead code or
  left only as test helper code with no production call sites. Prefer removal.
- Existing entity-related callers must migrate to `withEngineLaneBackgroundTask`
  after their outer direct goroutine admission is replaced by lane submission:
  `repairEntityArtifactsWithPlan`, `RebuildEntityArtifactsWithTrigger`,
  `RefreshEntityArtifactsForHealthTransitions`,
  `RefreshEntityArtifactsForFeedUpdates`, `runQueuedEntityArtifactRefresh`, and
  `runQueuedEntityHealthRefresh`.
- `RefreshEntityArtifactsForFeedUpdates` has no production queue caller today;
  it is a synchronous test/utility entrypoint. After migration it uses blocking
  engine-lane `Run`. Production work uses queue entrypoints with non-blocking
  `Submit`.
- No engine run, entity artifact, pipeline integrity, entity integrity, repair,
  rebuild, reload cleanup, or startup/reload entity check may use
  `withBackgroundTask` as admission.
- `backgroundMetricComponent` or any successor must not classify semantic state
  from task-name prefixes. Use typed `LaneWork` component/kind fields.

### DroneBL Downloader Contract

- DroneBL buildzone acquisition and child materialization remain downloader-lane
  work.
- DroneBL must not acquire the engine lane during acquisition, local artifact
  generation, or child materialization.
- Staged DroneBL recovery after restart is also downloader-lane work. Scheduler
  recovery may discover a staged artifact and enqueue recovery, but it must not
  call child materialization directly. A downloader worker performs recovered
  artifact child materialization from the staged artifact file and then queues
  normal processing for generated children.
- Recovered DroneBL child processing still uses the existing processing queue
  after downloader-lane materialization. The change is that discovery no longer
  materializes children or directly produces processing children; it enqueues a
  recovered-artifact download item whose `DownloadDecision.ProcessingNames`
  drives the normal `runDownload` child-processing enqueue path.
- The downloader-lane recovery item must be explicit and typed enough that tests
  and status can distinguish "recover staged artifact" from normal due download.
  It must avoid a remote refetch when a valid staged artifact file already
  exists.
- The recovered helper uses scheduler `enableAll` policy. Corrupt or unreadable
  staged artifacts are marked/removed before a normal due fetch is queued, so a
  restart cannot repeat the same failed recovered-artifact materialization
  forever. Transient failures leave the staged artifact in place for retry.
- Tests must cover DroneBL queue visibility through the scheduler download queue,
  child materialization from the downloader worker, and absence of engine-lane
  acquisition.
  Required test names are:
  - `TestDroneBLArtifactQueuesThroughDownloadLane`
  - `TestDroneBLChildrenMaterializeInDownloadWorker`
  - `TestDroneBLDoesNotAcquireEngineLane`
  - `TestRecoveredDroneBLArtifactMaterializesInDownloadWorker`

Validation plan:

- Unit tests:
  - engine lane with limit `1` starts work strictly FIFO
  - engine lane with limit `2` starts exactly two active jobs and starts later
    jobs in FIFO order when slots free
  - cancelled queued work never acquires or releases a slot and remains visible
    only until cancellation is observed
  - panic or error inside a lane worker releases the slot and starts the next
    queued item
  - blocking `Run` from inside an active worker of the same lane fails fast
    instead of deadlocking
  - blocking `Run` waiting for admission returns the caller context error when
    the caller context is canceled before a slot opens
  - production `Submit` rejects empty or unbounded coalescing keys with typed
    `ErrLaneMissingCoalescingKey` or equivalent, while repeated canonical keys
    coalesce instead of growing the waiting queue
  - public `RunOnce` admits through the engine lane and internal admitted helper
    call sites are not reachable from scheduler/web/operator code
  - `tryMarkRunStart` false after lane admission is treated as a defensive
    bypass/concurrency failure, returns the existing run-already-in-progress
    error class rather than lane-shutdown state, and still releases the lane
    slot
  - non-blocking submit from inside an active lane worker does not deadlock at
    limit `1`
  - `withEngineLaneBackgroundTask` preserves background-task visibility and
    metrics without acquiring the old `backgroundLimiter`
  - background compatibility metrics use typed `LaneWork` component labels and
    do not rely on task-name prefixes
  - queue snapshot shows active and waiting jobs with stable names, stage,
    timestamps, lane name, and wait duration
  - `LaneWork` and `LaneTicket` expose stable typed state for admin/API
    responses without parsing display names
  - coalesced tickets mirror existing work state, including `active` when the
    existing item is already running, and reuse the existing work item's stable
    ID rather than minting a new ID per coalesced submit
  - `StatusSnapshotLight`, `LaneSnapshot`, and `LaneWorkSnapshot` expose only
    the lightweight in-memory fields listed in the admin status contract
  - runtime resolution defaults `max_engine_lane_workers` to `1`, clamps it by
    `max_ingest_workers`, and keeps `max_background_workers` as separate
    background/entity fan-out
  - runtime validation rejects negative `max_engine_lane_workers`
  - `pkg/config/config.go:DefaultRuntime`, `pkg/config/validate.go`, and
    `pkg/config/runtime_controls_test.go` cover `max_engine_lane_workers`
  - `pkg/config/config.go` `RuntimeConfig`, `pkg/engine/runtime.go` `Runtime`,
    and `pkg/engine/types.go` `StatusSnapshot` all expose
    `MaxEngineLaneWorkers`; config validation includes
    `"runtime.max_engine_lane_workers"`
  - runtime reload changes engine-lane limit for queued/future starts without
    canceling active work
  - `Engine.Reload()` updates the live lane limit through the lane `SetLimit`
    equivalent after resolving runtime config
  - `newEngineFixture` and any direct `Engine` test construction initialize the
    engine lane and both integrity caches; nil-lane or nil-cache fallback is not
    accepted for production paths
  - entity artifact queue entrypoints return typed queued/coalesced/error state
    so HTTP handlers can distinguish accepted/coalesced work from submission
    failure
  - `QueueEntityArtifactsRebuild`, `QueueEntityArtifactsRefreshForFeedUpdates`,
    and `QueueEntityArtifactsRefreshForHealthTransitions` all return typed queue
    result plus `error`; scheduler callers in `processing_loop.go` and
    `download_loop.go` handle errors without blocking and without direct
    goroutine fallbacks
  - background task tests are updated or replaced so old `backgroundLimiter`
    admission tests do not keep testing dead code
  - old `backgroundLimiter`, `withBackgroundTask`, and `newBackgroundLimiter`
    have no production call sites after migration
  - lane shutdown closes admission, cancels/skips queued items, cancels active
    contexts, waits up to the configured 30 second grace period in production,
    and releases lane locks/bookkeeping even when a test uses a shorter injected
    grace period
- Behavioral engine/web tests:
  - `RunOnce` waits behind active entity artifact work and then starts in FIFO
    order
  - entity background refresh queued while an engine run is active waits instead
    of starting concurrently
  - entity refresh drains the current pending set as one engine-lane item and
    preserves feed-name coalescing behavior
  - continuously arriving entity refresh work yields after the bounded wave or
    60 second elapsed budget, requeues itself, and allows a queued
    integrity/reload item to start at `max_engine_lane_workers=1`
  - startup entity repair submitted while an engine run is active waits in the
    engine lane
  - startup pipeline-integrity recovery remains a synchronous pre-server step
    and does not become a request-time or concurrent runtime scan
  - startup pipeline-integrity recovery populates the pipeline integrity cache on
    successful scan, and request-time GET during a synthetic startup scan reports
    cold/in-progress instead of starting a second scan
  - startup pipeline-integrity recovery with zero findings stores a fresh empty
    cache instead of leaving the cache cold
  - pipeline integrity cache scope includes `include_archived`, `enableAll`, and
    web-dir identity; mismatched-scope GET/reprocess requests queue a matching
    refresh instead of serving stale-but-fresh-looking findings
  - operator entity rebuild queues and returns accepted without waiting in the
    HTTP handler
  - admin pipeline-integrity and entity-integrity GET while relevant work is
    queued or running returns `in_progress` or cached status instead of scanning
    live files
  - admin pipeline-integrity and entity-integrity GET while cache is cold returns
    cold/in-progress status instead of scanning live files
  - explicit admin entity-integrity refresh queues an engine-lane scan and
    updates the cached settled report on completion
  - explicit admin pipeline-integrity refresh queues an engine-lane scan and
    updates the cached settled report on completion
  - new refresh routes are additive and existing `/integrity/reprocess` and
    `/integrity/entities/rebuild` routes remain supported
  - `pkg/web/routes.go` registers `POST /api/v1/admin/integrity/refresh` and
    `POST /api/v1/admin/integrity/entities/refresh`
  - `pkg/web/routes_test.go` covers GET/HEAD method-not-allowed behavior for the
    two new refresh routes, including `Allow: POST`
  - `pkg/web/server.go:160` route normalization/telemetry allowlist and
    `pkg/web/routes_test.go` are updated for the new admin routes
  - entity and pipeline integrity caches expose incrementing `generation` values
    and conservative whole-cache stale transitions after relevant mutations
  - pipeline cache becomes stale after successful `applyRenamesAndDeletes`
    mutation or an equivalent feed-tree rename/delete mutation
  - startup `queueStartupIntegrityRecovery` populates the pipeline integrity
    cache when its scan succeeds
  - repair-path entity re-scans populate the entity integrity cache when they
    produce settled results
  - pipeline reprocess uses cached settled findings and queues refresh instead
    of scanning live files when the cache is cold or unusable
  - entity rebuild already queued/running path does not call the entity live scan
    fallback
  - existing rebuild/reprocess handlers return queued/in-progress status without
    falling back to a live GET scan when work is already queued
  - reload-time critical-infrastructure cleanup is admitted through the engine
    lane when reload happens in a running daemon
  - `Engine.Reload()` queues reload cleanup and returns after submit/coalesce,
    without waiting for the cleanup lane slot to execute
  - duplicate reload cleanup submissions coalesce by typed key and return a
    coalesced lane ticket instead of starting duplicate cleanup workers
  - reload cleanup tests use a bounded helper such as
    `waitForLaneTicketCompletion(t, eng, ticket, timeout)` instead of sleeping
    or assuming synchronous cleanup
  - `cmd/update-ipsets/daemon.go` SIGHUP entity check submits typed engine-lane
    work instead of starting a direct heavy goroutine
  - `Engine.Reload()` releases `e.mu` before submitting reload cleanup; existing
    reload cleanup tests such as critical-infrastructure reload cleanup are
    updated to assert queued/completed cleanup behavior instead of synchronous
    inline cleanup
  - delayed publish-stage cleanup queues through the engine lane after startup
    and respects daemon cancellation
  - `prepareEngineForRun` pre-server cleanup remains direct and is covered by a
    test or explicit assertion that it runs before server/scheduler startup
  - foreground run plus queued entity refresh plus integrity request does not
    create concurrent heavyweight entity file mutation/scan
  - entity artifact publication remains serialized when engine lane worker count
    is greater than one
  - history derivative composition remains downloader-FIFO work
  - DroneBL buildzone acquisition and child materialization are visible through
    the downloader queue and do not bypass it
  - DroneBL acquisition and child materialization do not acquire the engine lane
  - staged DroneBL artifact recovery enqueues explicit downloader-lane recovery
    work and does not materialize children directly in `recoverStagedWork`
  - recovered staged DroneBL work is enqueued before the first normal due-cycle
    enqueue after scheduler startup because recovery discovery runs before
    `runFetchLoop`
  - recovered staged DroneBL materialization uses scheduler `enableAll` policy
    and preserves the normal child processing enqueue path through
    `DownloadDecision.ProcessingNames`
  - corrupt staged DroneBL recovery removes the corrupt staged file or renames
    it with a `.corrupt` suffix that restart discovery ignores, then schedules a
    normal due fetch instead of looping on the same staged file
  - `TestDroneBLArtifactQueuesThroughDownloadLane`,
    `TestDroneBLChildrenMaterializeInDownloadWorker`, and
    `TestDroneBLDoesNotAcquireEngineLane`, plus recovered-artifact coverage, or
    equivalent behavioral tests exist
  - downloader queue dispatch is stable FIFO, including equal-time enqueue cases
    and merged queued-work sequence preservation
  - `/healthz` and lightweight status routes respond while an engine-lane worker
    is blocked in a synthetic heavy job
  - lightweight status routes do not walk output directories, decode entity JSON,
    run integrity scans, or aggregate full per-feed metrics on demand
  - lightweight status routes do not call `runtime.ReadMemStats` on each
    request; tests may enforce this structurally or with an injectable sampler
  - light admin status is implemented through `GET /api/v1/admin/status?mode=light`
    on the existing status route, not by adding an unapproved separate route
  - if light status includes runtime memory fields, it reads a cached
    `runtimeStatsSampler` sample rather than calling `runtime.ReadMemStats`
  - `TestAdminStatusLightUsesRuntimeStatsSampler` or equivalent proves the light
    status path uses the cached sampler and full status remains compatible
  - runtime stats sampler starts from `web.Run`, stops on server/daemon context
    cancellation, and does not leak goroutines in shutdown tests
  - admin UI polling uses the new light status route/query while the engine lane
    is active or for high-frequency refresh; full status is not used for polling
    intervals shorter than 30 seconds
  - free-path structural/behavioral tests prove `/healthz`, public cached
    serving, and light admin status do not acquire an engine/download slot
  - status snapshot uses typed lane state instead of entity background task name
    prefixes
  - `entityIntegrityBusy`, `entityBackgroundTaskRunning`, and rebuild
    queued/running checks no longer parse background task display names
  - `rg 'Entity artifacts |HasPrefix.*task\\.Name|backgroundTaskNamedLocked' pkg/`
    returns no production semantic-state matches after migration; any remaining
    matches are test fixtures or human-readable labels only
  - admin status JSON exposes `engine.engine_lane`, preserves
    `engine.background_limit` and `engine.background_running` compatibility
    aliases, and keeps `engine.max_background_workers` separate from
    `engine.max_engine_lane_workers`
  - admin status exposes typed pipeline/entity integrity cache snapshots with
    `generation`, `cache_state`, and `checked_at`
  - `ui/src/lib/admin-api-types.ts`, `ui/src/test/fixtures.ts`, and
    `ui/e2e/api-fixtures.ts` are updated for `engine.engine_lane` and
    `engine.max_engine_lane_workers`
  - `ui/e2e/api-fixtures.ts` is also brought up to date for existing runtime
    fields already present in source fixtures, including `max_ingest_workers`,
    `parallel_downloads`, `parallel_dns_queries`, `max_processing_workers`,
    `max_heavy_phase_workers`, and `max_background_workers`
  - status/config/operator docs explain that `background_limit` compatibility
    aliases engine-lane admission while `max_background_workers` remains fan-out
  - `.agents/sow/specs/processing-engine.md` describes top-level engine-lane
    admission and the fact that `RunOnce` is admitted as one lane item
  - compatibility metrics `background.workers.active` and
    `background.workers.limit` are emitted from engine-lane state after
    migration and do not conflict with any remaining legacy limiter
  - daemon context cancellation cancels queued lane work, lets in-flight work
    reach a safe stop, and does not leave partial entity artifacts
  - a reload that changes effective `WebDir` invalidates/drops old
    pipeline-integrity cache scopes, and the next GET/reprocess for the new
    scope reports cold/queued state instead of old fresh data
  - entity refresh bootstrap inside a bounded wave is not interrupted mid-build
    by the 60-second fairness budget; follow-up work is resubmitted after the
    wave completes
- Existing tests to update or replace:
  - `pkg/web/integrity_test.go` tests that currently expect request-time live
    entity or pipeline findings must pre-populate cache or assert cold/queued
    status instead, including
    `TestBuildIntegrityReportAnnotatesRecoveryMetadata`,
    `TestBuildIntegrityReportExcludesArchivedFeedsUnlessRequested`,
    `TestHandleAdminEntityIntegrityReturnsEntityFindings`,
    `TestHandleAdminEntityIntegrityRebuildSchedulesBackgroundRebuild`,
    `TestHandleAdminIntegrityReprocessReturnsSplitTargets`, and
    `TestEntityIntegrityBusyDuringEngineRunOrEntityBackgroundTask`.
  - `pkg/engine/background_tasks_test.go` tests that assert old limiter
    admission for entity/integrity work must move to lane admission and
    compatibility metric checks.
  - `pkg/engine/engine_fixture_test.go` must initialize the engine lane for
    direct fixture construction.
  - `pkg/engine/runtime_test.go` must cover `MaxEngineLaneWorkers` defaults,
    validation, and ingest ceiling clamping.
  - `pkg/web/admin_status_test.go`, UI fixtures, and admin status e2e fixtures
    must cover the `engine.engine_lane` JSON shape and light-status polling path.
  - `StatusSnapshotLight` caller tests must cover the cheap public fields
    needed by `pkg/web/public_status.go`; full admin status tests must keep
    using or asserting the full snapshot source.
  - `pkg/web/admin_unification_test.go` and
    `pkg/engine/integrity_blocked_test.go` should be checked and updated where
    their current assumptions conflict with typed lane state or cached integrity
    results.
  - `pkg/engine/entity_rebuild_coordination_test.go`,
    `pkg/engine/pipeline_integrity_deferred_refresh_test.go`, and
    `pkg/engine/run_reason_test.go` currently simulate rebuild-active state with
    `beginBackgroundTask("Entity artifacts rebuild", ...)`. These tests must
    submit actual lane work or set typed coalescing/lane state instead of relying
    on background task display names.
  - `pkg/engine/web_batch_test.go` and critical-infrastructure reload tests must
    cover cleanup lane ownership where applicable.
- Validation commands:
  - `go test ./pkg/engine -count=1`
  - `go test ./pkg/scheduler -count=1`
  - `go test ./pkg/web -count=1`
  - `go test ./tools/dronebl2ipsets -count=1`
  - `make test`
  - `make lint`
  - `make race`
  - `make bench`

Artifact impact plan:

- AGENTS.md: no project-wide guardrail change is required yet.
- Runtime project skills: update `project-coding` and
  `project-operations` to require heavy engine/entity/integrity work to use the
  engine lane, not direct goroutine admission.
- Specs:
  - `config.md`: add `max_engine_lane_workers`, document that
    `max_background_workers` remains internal background/entity fan-out such as
    entity feed-sidecar build workers, not top-level admission. Document the
    `max_ingest_workers` ceiling, revise the runtime concurrency contract from
    four domains to at least five, and state that download, processing fan-out,
    heavy-phase fan-out, background/entity fan-out, and engine-lane admission
    remain separately tunable. Document that background-maintenance admission and
    engine-run admission intentionally share `max_engine_lane_workers`, while
    `max_background_workers` does not create a second limiter.
  - `pipeline.md`: document `RunOnce` as one engine-lane item while preserving
    internal processing/heavy-phase fan-out.
  - `processing-engine.md`: document top-level engine-lane admission,
    `RunOnce` serialization, and bounded entity refresh waves.
  - `downloader.md`: document stable downloader FIFO ordering and recovered
    DroneBL artifact materialization through downloader workers.
  - `operating-principles.md`: document three-lane liveness contract, lane
    metrics, cancellation, free-path constraints, and update the existing
    concurrency-domain list from four domains to include engine-lane admission.
    The spec update must revise existing concurrency-domain statements rather
    than adding a contradictory new paragraph beside them.
  - `integrity.md`: document cached settled pipeline/entity integrity reports,
    stale/last-settled labeling, and explicit queued refresh/rebuild/reprocess
    behavior, including WebDir-scoped pipeline cache invalidation on reload.
  - `admin-ui.md`: preserve the four top live lists, define where engine-lane
    active/waiting work appears, and define light status polling semantics,
    including that full status is not the high-frequency polling endpoint.
- End-user/operator docs: update runtime config, especially
  `docs/configuration/runtime-settings.md`, and admin integrity semantics.
- Frontend contracts: update `ui/src/lib/admin-api-types.ts`,
  `ui/src/test/fixtures.ts`, and `ui/e2e/api-fixtures.ts` for
  `engine.engine_lane`, `engine.max_engine_lane_workers`, and light status
  polling. Verify existing fixture fields before adding new ones; add only
  missing fields and avoid duplicating runtime keys already present in source
  fixtures.
- End-user/operator skills: none expected.
- SOW lifecycle: this SOW remains the focused implementation SOW. It cannot close
  until implementation, specs, skills, docs, tests, and review gates pass.

Open-source reference evidence:

- `argoproj/argo-cd @ 24af376521be6c444e333a22fc11bc018f999814`
  - `controller/appcontroller.go:200` creates named work queues.
  - `controller/appcontroller.go:951` starts a configured number of refresh
    workers.
  - `controller/appcontroller.go:975` separates a lightweight enqueuer from a
    heavier hydration queue.
- `grafana/loki @ eecfe8a42c441a6dad7c40309183117bb6282204`
  - `pkg/engine/compactor/config.go:25` exposes a per-cycle task concurrency
    cap applied through `errgroup.SetLimit`.

Recorded design decisions:

1. Implementation language and scope: Go work-lane implementation now.
   Classification: surgical.
   - Source: user decision `1A`.
   - Rationale: the evidence does not justify a rewrite for this failure. The
     failure is architectural and testable in the current codebase.

2. Engine lane default: one worker.
   Classification: surgical.
   - Source: user decision `2A`.
   - Rationale: pure FIFO, safest production behavior, easiest to reason about,
     and prevents concurrent entity mutation/integrity scan by default.

3. Downloader-stage CPU-heavy local composition: keep downloader-stage work only
   in the downloader FIFO.
   Classification: surgical.
   - Source: user decision `3A`.
   - Rationale: this preserves current pipeline ownership. It intentionally
     accepts that CPU-heavy downloader-local set algebra can overlap with engine
     lane work.

4. Admin entity-integrity behavior: page-load GET returns `in_progress` or
   cached settled findings and does not wait for a heavy slot inside the request.
   Classification: surgical.
   - Source: user decision `4A`.
   - Rationale: admin page stays responsive, there is no request-time live scan,
     and mutation/scan races are avoided.

5. Public runtime concurrency model: preserve existing knobs, add
   `max_engine_lane_workers`, and keep `max_background_workers` as fan-out.
   Classification: long-term-best.
   - Source: review resolution from existing config/pipeline specs and user
     decisions `1A`/`2A`.
   - Rationale: collapsing public concurrency fields would break existing
     operator expectations. Reusing `max_background_workers` for engine-lane
     admission would make a background-named fan-out field control engine runs
     and integrity scans. A new explicit engine-lane field is clearer, while the
     old field remains available for background/entity fan-out inside admitted
     work.

6. `RunOnce` admission scope: the entire `RunOnce` call is one engine-lane work
   item.
   Classification: surgical.
   - Source: review resolution from the user's "engine" lane requirement.
   - Rationale: splitting only heavy phases would be a broader engine rewrite and
     would leave feed-processing and entity mutation interactions underdefined.
     Existing `max_processing_workers` and `max_heavy_phase_workers` remain
     internal fan-out knobs inside an admitted run.

7. Admin lane visibility: preserve the four top live lists and expose engine
   lane work in background/operations status.
   Classification: surgical.
   - Source: review resolution from the admin UI spec.
   - Rationale: the admin UI spec already reserves the four top lists for
     downloader and processing queues. Engine-lane visibility is required, but it
     must not be presented as a fifth or sixth top live list.

8. Free lane semantics: no worker queue.
   Classification: surgical.
   - Source: review resolution from the user's "web server and watchdog should
     not be limited" requirement.
   - Rationale: the free lane is an availability contract for cheap paths, not a
     third bounded queue that could itself block watchdog or health checks.

## Implications And Decisions

User decisions recorded on 2026-06-22:

1. A selected: implement bounded work lanes in Go now.
2. A selected: default engine lane to one worker.
3. A selected: downloader-stage work stays only in the downloader FIFO,
   including CPU-heavy local composition.
4. A selected: admin integrity returns `in_progress` or cached settled state
   while the lane is busy; explicit refresh queues work.

Additional user requirement:

- DroneBL is downloader work and must also be in the download queue.
- SOW analysis verified that the normal DroneBL due path already enters through
  the scheduler download queue, but staged DroneBL artifact recovery currently
  materializes children directly. Implementation must close that recovery bypass
  and test both normal and recovered DroneBL paths.

Design resolutions from review findings:

These resolutions do not replace user decisions. They remove implementation
ambiguity while preserving the user's selected options and the existing specs.

5. Preserve public concurrency domains. Add `max_engine_lane_workers`, default
   `1`; preserve `parallel_downloads`, `max_processing_workers`,
   `max_heavy_phase_workers`, and `max_background_workers`; keep
   `max_background_workers` as background/entity fan-out, not as an engine-lane
   admission alias.
6. Entire `RunOnce` is one engine-lane item; internal fan-out remains controlled
   by existing processing/heavy-phase settings.
7. Engine-lane work is visible in the admin background/operations status area,
   not as additional top downloader/processing live lists.
8. The free lane is a no-slot availability contract, not a third worker queue.

## Plan

1. Record user decisions and review-derived design resolutions in this SOW.
2. Add behavioral tests for FIFO lane semantics, cancellation, slot release,
   re-entrant submit, and snapshots before production code changes.
3. Add runtime/config tests for `max_engine_lane_workers` defaulting, ingest
   ceiling clamping, runtime reload, and separation from `max_background_workers`.
4. Add admin pipeline/entity integrity tests proving GET does not scan live files
   and explicit refresh/rebuild/reprocess queues engine-lane work.
5. Add scheduler/download tests proving stable FIFO dispatch and DroneBL
   downloader ownership, including staged DroneBL artifact recovery.
6. Implement the engine lane coordinator, runtime config, metrics, and status
   snapshot.
7. Move entity background refresh/rebuild/repair, startup/reload entity checks,
   explicit integrity refresh, and integrity repair behind the engine lane.
8. Move `RunOnce` admission behind the engine lane while preserving internal
   processing/heavy-phase fan-out knobs.
9. Replace direct request-time admin integrity scans with cached settled status
   and queued refresh/rebuild actions.
10. Preserve downloader FIFO ownership for downloader-stage local composition and
    DroneBL work, including recovered staged DroneBL artifacts.
11. Update admin status/UI so engine-lane queued and active work is visible in
    the background/operations area while the four top live lists remain
    downloader/processing only.
12. Update specs, project skills, and operator docs.
13. Run full validation and review before commit.

## Execution Log

### 2026-06-21

- Created this SOW for focused concurrency and watchdog availability analysis.
- Reviewed production failure evidence, local code paths, project specs,
  official Go concurrency documentation, and two open-source queue/concurrency
  references.
- No implementation started.

### 2026-06-22

- Recorded user decisions: `1A 2A 3A 4A`.
- Recorded explicit requirement that DroneBL work must be admitted through the
  download queue.
- Re-confirmed after approval that DroneBL staged recovery, acquisition, and
  child materialization are part of the downloader FIFO contract.
- Implemented the engine work lane with FIFO admission, finite coalescing keys,
  cancellation, shutdown, lane snapshots, typed tickets, and typed
  kind/component/state labels.
- Routed `RunOnce`, entity artifact repair, entity refresh, entity rebuild,
  pipeline integrity refresh, pipeline integrity reprocess admission, and entity
  integrity refresh through the engine lane. Kept `max_background_workers` as
  internal fan-out, not top-level admission.
- Added `runtime.max_engine_lane_workers`, validation, runtime resolution,
  reload-time lane limit updates, and admin status exposure.
- Reworked admin pipeline/entity integrity handlers to be cache-first for GET
  requests. Cold/stale cache queues refresh and returns in-progress state.
  Reprocess uses fresh cached findings and submits the recovery admission to
  the engine lane.
- Added typed pipeline/entity integrity cache summaries to admin status without
  embedding full finding lists in the high-frequency status payload.
- Moved DroneBL staged artifact recovery into the downloader FIFO. Startup
  recovery now discovers staged artifact parents and queues recovered artifact
  work instead of materializing children directly.
- Added downloader queue item kind `recovered_artifact` and deterministic FIFO
  ordering for recovered artifact work.
- Added lazy short-TTL runtime/process sampling for admin status so frequent
  reloads do not repeatedly force expensive runtime/process snapshots while
  heavy work is running.
- Updated the admin UI Background Work section to render active and waiting
  engine-lane work from `engine.engine_lane` while preserving the four live feed
  queue tiles and their scrollable full-height list bodies.
- Preserved the admin listener `/api/v1/categories` route for split/admin-only
  serving mode.
- Updated specs, operator docs, and project skills for the bounded lane,
  cache-first integrity, runtime knob, and DroneBL recovery contracts.
- Clarified operator docs that DroneBL staged-artifact recovery is downloader
  FIFO work, not engine-lane background work.
- Fixed the `make race` gate by making `pkg/iprange` allocation-shape tests skip
  only under race-detector builds. Normal `make test` still enforces the
  allocation ceilings.
- Moved SOW status to `in-progress`.
- Ran external SOW review. All reviewers voted not ready because the SOW still
  had analysis-mode acceptance criteria, unresolved config semantics, ambiguous
  engine-lane admission, undefined admin-integrity cache/refresh behavior,
  unclear background-limiter interaction, unclear free-lane semantics, and
  incomplete validation requirements.
- Revised this SOW to define the three-lane contract, preserve existing runtime
  concurrency domains, add `max_engine_lane_workers`, define `RunOnce` admission
  scope, replace old background-limiter admission for engine-lane work, preserve
  entity publication serialization, preserve DroneBL downloader ownership, keep
  admin lane visibility out of the four top live lists, and convert validation
  into implementation criteria.
- Ran a second SOW review pass. Most reviewers voted ready, but remaining
  blockers identified concrete implementation-contract gaps: exact
  `withBackgroundTask` bypass mechanism, status JSON compatibility fields,
  pipeline-integrity scope, integrity cache lifecycle, refresh API routes,
  `BackgroundWorkers()` fan-out ambiguity, `max_ingest_workers` clamping,
  runtime reload behavior, and delayed publish cleanup.
- Revised this SOW again to specify `withEngineLaneBackgroundTask`, keep
  `max_background_workers` as background/entity fan-out instead of an
  engine-lane alias, bring pipeline integrity into the no-live-scan/cached
  refresh model, define refresh routes, define cache state and stale labeling,
  preserve status JSON compatibility aliases, route delayed publish cleanup
  through the engine lane, and require dynamic lane limit reload semantics.
- Resolved third-pass review findings by documenting exact admin integrity
  fallback removals, cached-report lifecycle, refresh/reprocess HTTP contracts,
  FIFO sequence/shutdown/re-entrant submit behavior, status JSON shape,
  `entityArtifactsMu` slot implications, typed rebuild coalescing, reload-time
  critical-infrastructure cleanup lane ownership, and DroneBL downloader-lane
  test coverage.
- Resolved additional P2 review findings by specifying startup pipeline
  integrity recovery as a synchronous pre-server step, entity refresh drain-loop
  granularity, `LaneWork` / `LaneTicket` typed state expectations,
  `max_engine_lane_workers` config validation, integrity cache generations and
  conservative stale transitions, free-lane status budget, compatibility alias
  documentation, and old `withBackgroundTask` removal/allowed-use rules.
- Resolved another reviewer's ambiguity findings by making integrity refresh
  routes additive instead of replacements, defining how compatibility background
  status/metrics are emitted from engine-lane state, enumerating entity
  `withBackgroundTask` callers that must migrate, clarifying delayed publish
  cleanup lane behavior, and requiring updates to existing web/background/runtime
  tests plus `make race`.
- Tightened the final old-review findings by selecting `pkg/engine/work_lane.go`
  as the default lane file, requiring runtime reload to call lane `SetLimit`,
  keeping public `RunOnce` synchronous while admitting through the lane, defining
  background compatibility metric labels from typed `LaneWork`, placing the
  integrity caches on `Engine` as in-memory state, and naming required DroneBL
  downloader-lane tests.
- Resolved fresh-review P0/P1/P2 findings by specifying non-blocking SIGHUP
  reload cleanup/entity-check submission, removing the old background limiter
  from production paths, naming the omitted `RefreshEntityArtifactsForFeedUpdates`
  migration, requiring fixture lane initialization, moving status JSON fields
  under the existing `engine` object, adding a light admin status polling
  contract, enumerating route/telemetry/UI type updates, and naming integrity
  tests that must be rewritten for cached reports.
- Resolved the staged DroneBL recovery gap by recording that
  `RecoverStagedArtifacts` currently materializes artifact children directly and
  requiring recovered DroneBL artifacts to be enqueued as downloader-lane
  recovery work instead.
- Ran a fresh six-reviewer pass. All six reviewer sessions completed. Some
  reviewers voted ready and others voted not ready; all P0/P1/P2 findings were
  treated as mandatory. Blocking themes were SOW precision, not design:
  recovered DroneBL processing/recovery semantics, reload cleanup coalescing,
  explicit integrity cache fields and synchronization, entity refresh fairness,
  stale-cache reprocess semantics, queueing error behavior, free-lane
  enforcement, `runtime.ReadMemStats` on light status, typed status snapshots,
  background metric component mapping, downloader FIFO sequence placement,
  zero-finding startup cache population, and missing spec/test artifact anchors.
- Resolved those fresh P0/P1/P2 findings by correcting stale file references,
  requiring explicit `Engine` lane/cache fields and cache methods, protecting
  integrity caches with `e.mu`, defining zero-worker and lowered-limit behavior,
  specifying re-entrant lane detection, adding mandatory `withBackgroundTask`
  migration order, documenting `RunOnce` serialization at lane limits greater
  than one, bounding entity refresh waves to one extra wave or 60 seconds,
  detailing reload synchronous state and `e.mu` release before lane submission,
  defining reload cleanup coalescing, defining stale-cache reprocess warnings
  and queue-error HTTP behavior, specifying recovered DroneBL discovery,
  downloader materialization, normal child-processing enqueue, scheduler
  `enableAll` use, corrupt staged-file handling, adding typed `StatusSnapshot`
  and safe admin JSON contracts, mapping background compatibility components
  from typed lane state, adding cache mutation inventory and startup zero-finding
  cache setter requirements, forbidding request-time `runtime.ReadMemStats` in
  light status, selecting `GET /api/v1/admin/status?mode=light`, adding
  free-lane enforcement tests, anchoring refresh-route method/telemetry updates,
  and adding downloader and processing-engine spec impacts.
- Ran another six-reviewer pass. One reviewer voted ready and five voted not
  ready. The not-ready findings were specification precision gaps: scoped
  pipeline-integrity cache options, exact `max_engine_lane_workers` struct and
  validation fields, entity queue return shapes, production queue callers,
  recovered-artifact queue item typing, recovery/fetch-loop ordering,
  reload unlock and `closeASNLookupDatabases` preservation, rebuild coalescing
  migration ordering, lane shutdown context registration, coalescing key names,
  tests that mocked rebuild state with background task names, and light-status
  response/sampler specifics.
- Resolved those P0/P1/P2 findings by defining pipeline-integrity cache scope,
  adding exact `RuntimeConfig` / `Runtime` / `StatusSnapshot` field requirements,
  naming the config validation map entry and `EngineLaneWorkers()` helper,
  changing all entity queue entrypoints to typed result plus error, naming
  `processing_loop.go` and `download_loop.go` queue callers, adding a
  `queuedWork.Kind` recovered-artifact marker, choosing synchronous
  staged-recovery discovery before `runFetchLoop`, preserving ASN lookup cleanup
  after explicit reload unlock, adding required coalescing keys and migration
  order, adding lane shutdown registration from `web.Run`, adding
  `ErrLaneShuttingDown`, updating tests that fake rebuild-active state, and
  specifying light-status fields plus optional runtime stats sampler behavior.
- Ran the next six-reviewer pass. Five reviewers voted ready and one voted not
  ready. The remaining P0/P1/P2 findings were finite/coalesced engine-lane
  admission, exact `StatusSnapshotLight` / `LaneSnapshot` type contracts,
  canonical lane kind/component values, removal of the redundant `Submit`
  boolean, deterministic shutdown grace behavior, `Run` caller-context
  cancellation while waiting for admission, explicit `tryMarkRunStart` slot
  release, runtime sampler lifecycle, coalesced-ticket state semantics, cache
  state enum values, locked-helper requirements for cache mutation, and
  testable corrupt DroneBL staged-artifact handling.
- Resolved those findings by making production `Submit` calls require finite
  coalescing keys, changing `Submit` to return `LaneTicket, error`, defining
  canonical lane kinds/components/states and snapshot structs, adding
  deterministic shutdown and cancellation behavior, specifying `RunOnce`
  defensive slot release, defining `runtimeStatsSampler` lifecycle, enumerating
  cache states, forbidding recursive `e.mu` cache calls, and defining corrupt
  staged-artifact removal/rename behavior.
- Ran another six-reviewer pass. All six reviewers voted ready for
  implementation. Several P2 polish findings were still recorded because this
  SOW is fixing all P0/P1/P2 findings, not only blockers: non-light admin status
  mode fallback, explicit cache snapshot structs and JSON tag expectations,
  one-shot lane-ticket lifetime, `DefaultRuntime` defaulting, background metric
  component mapping source, staged recovery discovery return shape, processing
  loop ordering during recovery, missing rebuild scheduling test name, corrupt
  sidecar format, and lint validation.
- Resolved those P2 findings by documenting full-status fallback for any
  non-`light` mode, adding explicit cache snapshot structs and JSON tag rules,
  defining lane tickets as immutable submission-time values, adding
  `DefaultRuntime` defaulting, clarifying `background.component` derives from
  typed component first, specifying recovered artifact discovery return shape
  and processing-loop ordering, adding
  `TestHandleAdminEntityIntegrityRebuildSchedulesBackgroundRebuild`, defining a
  JSON corrupt sidecar convention, and adding `make lint`.
- Ran the final six-reviewer ready-for-implementation pass. All six reviewers
  voted ready, but review still identified P1/P2 precision gaps that this SOW
  treats as required fixes before implementation.
- Resolved those precision gaps by adding cheap `StatusSnapshotLight` public
  fields and explicit caller routing, documenting coalesced-ticket ID reuse,
  pinning defensive `tryMarkRunStart` error semantics, forbidding scheduler
  direct-goroutine fallback on queue errors, clarifying entity bootstrap wave
  fairness, preserving reload ASN close-before-submit ordering, invalidating
  WebDir-scoped pipeline integrity caches on reload, documenting sampler
  lifecycle/test cadence, forbidding full-status high-frequency polling, and
  tightening DroneBL recovery enqueue ordering.
- Ran the required follow-up six-reviewer SOW pass after those edits. All six
  reviewers voted `READY FOR IMPLEMENTATION`; no P0/P1/P2 SOW issues remained.
- Ran post-implementation external review. Most reviewers voted production-grade
  or acceptable, but valid findings remained:
  - pipeline integrity cache was single-slot instead of per web-dir/options
    scope
  - reload could leave old web-dir integrity state looking fresh
  - DroneBL corrupt staged-artifact recovery renamed the artifact but did not
    write the required JSON sidecar
  - web-run engine-lane shutdown grace was 10 seconds instead of the SOW's
    30-second contract
  - runtime stats sampler was not idempotent per server instance
  - public status still used the detailed runtime sampler path
  - browser admin fixtures did not include the new engine-lane/integrity cache
    fields
  - processing-engine spec did not state the engine-lane admission contract
  - extra behavioral guards were needed for free-path status, refresh POST
    routes, downloader FIFO sequence ordering, and idle lane shutdown
- Fixed those review findings:
  - pipeline integrity cache is now per scope and uses hashed web-dir
    coalescing keys
  - reload marks existing pipeline integrity scopes stale when the effective
    web root changes
  - corrupt recovered DroneBL staged artifacts now produce a sibling
    `.corrupt.json` sidecar with artifact name, corruption class, and timestamp
  - web-run lane shutdown grace is 30 seconds
  - runtime sampling uses a per-server `runtimeStatsSampler` with `sync.Once`
  - public status uses cached runtime status
  - e2e admin fixtures include engine lane and integrity cache summaries
  - `processing-engine.md` records engine-lane admission and DroneBL downloader
    FIFO ownership
  - added tests for scoped integrity cache, reload stale-scope behavior, DroneBL
    corrupt sidecar, idle lane shutdown, admin light status while the engine
    lane is busy, admin refresh POST queueing, and download FIFO tie ordering
- Ran repeat post-remediation external review. Five reviewers completed and
  voted production-grade or production-grade with residual risks; one reviewer
  timed out without a verdict. A completed reviewer found one valid
  medium-severity entity-refresh liveness race:
  - when a refresh wave set `entityRefreshRunning=false` while its lane item
    still owned the old coalescing key, a concurrent producer could coalesce
    into the completing item and leave `entityRefreshRunning=true` with no
    future lane item
  - the same race shape existed for health-transition refresh
  - another reviewer raised a possible `RunOnce` lane slot leak but withdrew it
    after verifying that `WorkLane.finishItem` releases the slot after callback
    return
- Fixed the entity-refresh liveness race by using finite alternating
  coalescing keys for both initial refresh work and continuation work, and by
  reserving continuation state under `e.mu` while keeping the queue running
  flag true until the continuation is submitted. Added deterministic tests for
  feed-update and health-transition producers racing with a completing lane
  item, and added a lane test proving `Submit` from inside an active worker
  queues without deadlocking at lane limit one.
- Ran repeat round-3 external review after the entity-refresh fix. All six
  reviewers voted production-grade. No blocking code, security, data-loss, or
  liveness findings remained. Valid non-blocking artifact/test gaps were fixed
  before closure:
  - canonical specs now list five concurrency domains instead of stale four
    domain wording
  - specs and operator docs now name `GET /api/v1/admin/status?mode=light` as
    the high-frequency polling endpoint and reserve full status for lower-rate
    diagnostics
  - integrity spec now uses `refresh_queued` / `refresh_running` cache state
    names and documents web-directory-scoped pipeline cache staleness on reload
  - specs now state that `RunOnce` is one engine-lane work item
  - downloader spec now states stable FIFO tie ordering and merge-position
    preservation
  - runtime settings docs now distinguish `max_background_workers` from the
    `engine.background_limit` compatibility status alias
  - added `WorkLane` tests for limit lowering while active workers are running
    and for the `TryRun` busy path

## Validation

Current implementation validation:

- The approved model has been implemented locally:
  - engine-owned heavyweight work is admitted through the engine lane
  - downloader-owned work remains in the scheduler downloader FIFO
  - recovered DroneBL staged artifact parents are discovered first and then
    materialized by a downloader worker
  - admin pipeline/entity integrity GET handlers are cache-first and queue
    refresh work instead of scanning live files
  - admin status exposes typed engine-lane and integrity-cache summaries
  - admin UI keeps the four downloader/processing live tiles and renders
    engine-lane work in the Background Work section
- Local validation passed:
  - `go test ./pkg/engine -run TestReloadCleansCriticalInfrastructureArtifactsWhenProvidersRemoved -count=1 -v`
  - `go test ./pkg/engine -run TestReloadAppliesChangedIngestWorkerCeiling -count=1 -v`
  - `go test ./pkg/engine -run TestProviderOnlyRunDefersEntitySidecarStagingWhileFullRebuildActive -count=1 -v`
  - `go test ./pkg/engine -run TestPipelineIntegrityScenarioDeferredQueuedEntityRefreshSettlesMissingEmptySidecar -count=1 -v`
  - `go test ./pkg/engine ./pkg/web ./pkg/scheduler -count=1`
  - `pnpm --dir ui build`
  - `pnpm --dir ui lint`
  - `pnpm --dir ui test`
  - `make test-tools`
  - `go test ./pkg/engine ./pkg/web ./pkg/scheduler ./tools/archposture -count=1`
  - `make test`
  - `make build`
  - `make lint`
  - `make race`
  - `make bench`
- `pnpm --dir ui build` completed with existing non-fatal asset warnings:
  unresolved InterDisplay font URLs at build time and Vite chunk-size warnings.
- Post-review remediation focused validation passed:
  - `go test ./pkg/engine -run 'Test(PipelineIntegrityCacheKeepsIndependentScopes|ReloadStalesOldWebDirIntegrityScope|RecoverStagedDroneBLCorruptBuildzoneRenamesAside|WorkLaneShutdownIdleReturnsAndRejectsFutureWork)' -count=1`
  - `go test ./pkg/scheduler -run 'Test(DownloadQueueUsesEnqueueSequenceWhenQueuedAtTies|DownloadQueueMergeKeepsRecoveredArtifactOwnershipAndEarliestSequence|RecoveredCorruptDroneBLArtifactRequeuesNormalDownloaderFetch)' -count=1`
  - `go test ./pkg/web -run 'Test(AdminStatusLightRespondsWhileEngineLaneBusy|AdminIntegrityRefreshRoutesQueueEngineLaneWork|RuntimeStatsSampler)' -count=1`
  - `go test ./pkg/engine -count=1`
  - `go test ./pkg/scheduler -count=1`
  - `go test ./pkg/web -count=1`
- Post-review remediation full validation passed:
  - `go test ./pkg/engine ./pkg/web ./pkg/scheduler ./tools/archposture -count=1`
  - `make test`
  - `make build`
  - `make lint`
  - `make test-tools`
  - `pnpm --dir ui test`
  - `pnpm --dir ui lint`
  - `pnpm --dir ui build`
  - `make race`
  - `make bench`
  - `pnpm --dir ui build` completed with the known non-fatal InterDisplay
    asset-resolution warnings and Vite chunk-size warning.
- Entity-refresh liveness fix validation passed:
  - `go test ./pkg/engine -run 'Test(EntityArtifactRefreshQueueCoalescesFeedNames|EntityHealthRefreshQueueCoalescesFeedNames|QueueEntityArtifactRefreshDoesNotCoalesceWithCompletingLaneItem|QueueEntityHealthRefreshDoesNotCoalesceWithCompletingLaneItem|WorkLaneSubmitFromWorkerQueuesWithoutDeadlock)' -count=1`
  - `go test ./pkg/engine -count=1`
  - `go test ./pkg/engine ./pkg/web ./pkg/scheduler ./tools/archposture -count=1`
- Round-3 review follow-up validation passed:
  - `go test ./pkg/engine -run 'TestWorkLane(SetLimitLoweringDoesNotCancelActiveWork|TryRunDoesNotQueueWhenBusy|SubmitFromWorkerQueuesWithoutDeadlock)' -count=1`
  - `rg -n -i "four domain|four concurrency|at least four|four domain-specific|cold, fresh, stale, queued|mode=light|max_engine_lane_workers|engine-lane admission" .agents/sow/specs docs/admin-ui/runtime-status.md docs/configuration/runtime-settings.md`
  - `go test ./pkg/engine ./pkg/web ./pkg/scheduler ./tools/archposture -count=1`

Remaining validation before release/deployment:

- Installed-service smoke has not been run in this SOW. The local validation
  gate covers build, unit/integration behavior, race tests, benchmarks, UI
  tests, and DroneBL tooling. Service smoke is treated as a release/deployment
  verification step, not a blocker for this code/SOW closure.

Real-use evidence:

- Production logs and stack dumps from 2026-06-21 were reviewed and summarized.

Reviewer findings:

- First SOW review pass: not ready.
- P0/P1/P2 themes addressed in this revision:
  - stale analysis-mode acceptance criteria
  - public config model and existing concurrency-domain preservation
  - engine-lane versus existing heavy-phase worker terminology
  - entire `RunOnce` admission scope
  - relationship between engine lane and old `backgroundLimiter`
  - admin entity-integrity GET/refresh/rebuild behavior
  - cached settled integrity report semantics
  - startup entity work ordering
  - cancellation, panic/error slot release, and shutdown behavior
  - entity publication serialization when lane workers exceed one
  - free-lane no-slot semantics
  - admin UI four-list contract
  - DroneBL downloader-queue evidence and recovered-artifact test requirement
  - typed lane state replacing background-task-name prefix heuristics
- Second SOW review pass: not ready because remaining P1/P2 implementation
  contract details needed to be recorded. Addressed themes:
  - concrete `withEngineLaneBackgroundTask` wrapper to bypass old
    `backgroundLimiter` admission without losing visibility
  - config contract updated from four to at least five concurrency domains
  - `max_background_workers` kept as background/entity fan-out, not engine-lane
    alias
  - `max_engine_lane_workers` clamped by `max_ingest_workers`
  - admin status `engine_lane` snapshot plus `background_limit` /
    `background_running` compatibility aliases
  - pipeline integrity included in the no-live-scan/cached refresh model
  - integrity cache lifecycle, cache states, stale labeling, and refresh routes
  - runtime reload lane-limit behavior
  - delayed publish-stage cleanup lane ownership
  - update/replace old `backgroundLimiter` tests
- Third SOW review round was partially ready, with remaining P1/P2 findings around
  exact integrity handler fallbacks, explicit cache/API/status contracts,
  re-entrant FIFO semantics, panic/shutdown slot release, entity publish
  serialization under lane workers greater than one, typed rebuild coalescing,
  reload-time cleanup, and DroneBL-specific downloader tests. These are now
  recorded as implementation and validation requirements in this revision.
- Additional P2 findings from a ready reviewer were also recorded before
  implementation because the goal is to fix P0/P1/P2 review findings, not only
  blockers.
- Fresh SOW review pass: not ready because reviewers found unresolved reload
  behavior, old background-limiter disposition, missing entity refresh caller
  migration, admin status JSON nesting ambiguity, light-status polling gap,
  route registration/telemetry gaps, fixture initialization, frontend type
  updates, and specific integrity tests that would fail under cached-report
  semantics. These are now recorded as implementation and validation
  requirements.
- Latest SOW review round: five reviewers voted ready and one voted not ready.
  The remaining P0/P1/P2 findings were finite/coalesced engine-lane admission,
  exact light-status and lane snapshot type contracts, canonical lane
  kind/component values, redundant `Submit` boolean semantics, shutdown and
  cancellation details, `tryMarkRunStart` defensive slot release, runtime
  sampler lifecycle, coalesced-ticket state, cache-state enumeration, locked
  cache mutation helpers, and testable corrupt DroneBL staged-artifact handling.
- Latest completed SOW review round: all six reviewers voted ready for
  implementation. Non-blocking P2 polish items from that round are also recorded
  as implementation and validation requirements in this revision.

Same-failure scan:

- `rg` scans identified direct entity background goroutine starts, live admin
  integrity scan entrypoints, downloader-stage `pkg/iprange` union work, and
  existing limiter/status-snapshot patterns.

Sensitive data gate:

- This SOW contains no raw secrets, credentials, bearer tokens, private client
  addresses, production hostnames, customer names, personal data, or raw
  proprietary incident details.

Artifact maintenance gate:

- AGENTS.md: not updated; no project-wide guardrail change is required yet.
- Runtime project skills: updated `project-coding`, `project-testing`, and
  `project-operations` for engine-lane, cache-first integrity, DroneBL recovery,
  and runtime-operation guidance.
- Specs: updated `config.md`, `pipeline.md`, `downloader.md`, `integrity.md`,
  `operating-principles.md`, `processing-engine.md`, and `admin-ui.md`.
- End-user/operator docs: updated runtime settings, admin background-work,
  runtime-status, pipeline integrity, and entity-integrity docs.
- End-user/operator skills: not affected.
- SOW lifecycle: created in `.agents/sow/current/`; moved to
  `Status: in-progress` after user decisions were recorded; closed as
  `Status: completed` after implementation review and final validation passed.

Specs update:

- Completed for the affected contracts listed above.

Project skills update:

- Completed for the affected runtime/code/testing guidance listed above.

End-user/operator docs update:

- Completed for `max_engine_lane_workers`, engine-lane visibility, and admin
  pipeline/entity integrity refresh/cache behavior.

End-user/operator skills update:

- None.

Lessons:

- Heavy work needs an admission lease, not a status snapshot heuristic.
- A concurrency limiter is not the same as a FIFO work queue.
- Downloader-owned local composition can still be local CPU-heavy work.

Follow-up mapping:

- No deferred implementation item is intentionally left without a closure path.
  Installed-service smoke remains release/deployment verification, not an
  implementation blocker for this SOW.

## Original Outcome Before Regressions

Implementation, external implementation review, reviewer follow-up fixes, and
local validation for the original bounded-lane work were completed. This
original closure was later superseded by the regression sections below; the
current lifecycle status is the top-level status of this active SOW.

## Lessons Extracted

- The old failure mode was not a Go limitation. It was missing centralized
  admission for heavyweight work.
- Admin integrity must be cache-first. A status snapshot is not an execution
  lease.
- Recovered DroneBL artifacts are downloader work even when the parent artifact
  is already staged locally.

## Followup

No implementation follow-up remains. Installed-service smoke should be run as a
release/deployment verification step before promoting this build.

## Regression Log

See entries below.

Append regression entries here only after this SOW was completed or closed and
later testing or use found broken behavior. Use a dated `## Regression -
YYYY-MM-DD` heading at the end of the file. Never prepend regression content
above the original SOW narrative.

## Regression - 2026-06-22

Issue:

- The admin heartbeat feed-health counters showed zeros after the admin UI
  switched high-frequency polling to `GET /api/v1/admin/status?mode=light`.

Root cause:

- `buildAdminStatusLight` populated only `feeds.total_configured`. The rest of
  the `adminFeedsSummary` fields stayed at Go zero values, so the heartbeat
  rendered zero enabled, healthy, delayed, risky, unavailable, archived, empty,
  and unmaintained feeds even though `/api/v1/admin/feeds` had live rows.

Fix:

- The light status builder now computes the same feed summary as full status
  from the admin feed rows, while still omitting artifacts and full metric
  trees from the high-frequency response.
- Added a regression test that compares the light status `feeds` summary with
  the summary derived from `/api/v1/admin/feeds` rows.
- Updated the admin UI spec and runtime-status operator doc to state that the
  light status `feeds` block is a complete heartbeat summary, not only
  `total_configured`.

Validation:

- `go test ./pkg/web -run 'TestAdminStatus(LightIncludesFeedHealthSummary|KeepsFullDefaultAndLightPollingSnapshot|LightRespondsWhileEngineLaneBusy)' -count=1`
- `go test ./pkg/web -count=1`

## Regression - 2026-06-25

Issue:

- Production watchdog kills continued after the bounded lane work shipped.
- The production symptom pattern is silence/stall, not overload:
  - Local-time 2026-06-24 12:04:12 to 12:09:12: Netdata showed idle CPU
    activity; from 12:08:40 to 12:09:12 the process was not running.
  - Local-time 2026-06-24 16:05:51 to 16:09:48: pause, then activity.
  - Local-time 2026-06-24 16:11:03 to 16:16:48: no activity; systemd killed
    the process at 16:16:15 local time.
- The same production service recovered automatically after systemd restarted
  it, so the immediate incident impact was downtime/unresponsiveness rather
  than durable artifact loss.

Production observations:

- A read-only production check showed the service currently running and serving
  `/healthz`, with `NRestarts=2`.
- The service had restarted at 2026-06-24 13:17:14 UTC after a watchdog failure.
- systemd logs showed:
  - 2026-06-24 09:07:10 UTC: watchdog timeout.
  - 2026-06-24 09:08:41 UTC: stop-watchdog timeout, SIGKILL, result
    `watchdog`, restart counter 1.
  - 2026-06-24 13:14:45 UTC: watchdog timeout.
  - 2026-06-24 13:16:15 UTC: stop-watchdog timeout, SIGKILL.
  - 2026-06-24 13:16:16 UTC: main process exited with status `9/KILL`,
    result `watchdog`.
  - 2026-06-24 13:17:14 UTC: service started again.
- Kernel logs in the crash window contained no OOM-kill evidence.
- `coredumpctl` was unavailable on the host, so no coredump could be inspected.
- The Go stack dump started after SIGABRT but did not complete before systemd
  sent SIGKILL. The available dump showed only the main goroutine waiting in the
  web server runner; it did not reach the worker goroutines needed to prove the
  deadlock site.
- The last meaningful app sequence before the 13:14 UTC watchdog timeout was:
  - 2026-06-24 13:05:01 UTC: a large processing run finished after processing
    402 feeds.
  - The same run queued an entity artifact refresh for 205 feeds.
  - There was about 9 minutes and 44 seconds of application silence before the
    watchdog timeout path began.
- Current production pressure after restart still showed high I/O pressure, but
  the incident windows showed idle CPU. The primary failure model is therefore
  a stall/deadlock class, not insufficient CPU throughput.

Corrected interpretation:

- The earlier implementation/review focus on "heavy work must not starve the
  daemon" was necessary but incomplete.
- This regression is specifically about the daemon entering a no-progress state
  while CPU is idle and logs are silent.
- Go does not make deadlocks impossible. Go only detects the narrow global case
  where all goroutines are asleep and the runtime can prove no progress is
  possible. A server can still have partial deadlocks, lock inversions, blocked
  syscalls, or watchdog starvation without a Go runtime fatal deadlock message.

Gap analysis:

- The original SOW required the web server, admin status, health endpoint, and
  watchdog to remain responsive, but validation did not reproduce a production
  idle-CPU stall.
- The existing tests prove lane FIFO and some busy-lane responsiveness, but they
  do not prove watchdog progress when application goroutines are blocked on lock
  cycles or blocking notify syscalls.
- There is no regression test for lock-order inversion between engine status
  snapshots and lifetime telemetry books.
- There is no watchdog-notify test proving that the notify path is
  deadline-bound, failure-visible, and incapable of wedging the heartbeat
  goroutine forever.
- There is no automatic pre-watchdog goroutine dump / pprof capture path to
  preserve evidence before systemd sends SIGKILL.
- The admin UI freeze is not fully covered by light-status tests because the
  production report included complete admin frontend unresponsiveness during
  the stall window.

Hypotheses to verify or reject:

1. Status/telemetry/run-metrics lock-order fragility.
   - Evidence:
     - `pkg/engine/status_snapshot.go:62` takes `e.mu.RLock()`.
     - `pkg/engine/status_snapshot.go:76` calls `e.lifetimeMetricsSnapshot()`
       while still holding `e.mu.RLock()`.
     - `pkg/engine/run_metrics_state.go:14` and
       `pkg/engine/run_metrics_state.go:59` take lifetime telemetry locks
       before taking `e.mu.RLock()` to read `currentMetrics`.
     - `pkg/engine/run_metrics.go` uses per-run metrics locks that status and
       diagnostic paths can also inspect while `e.mu` is held.
     - `internal/telemetry/timing.go:42` and
       `internal/telemetry/counter.go:37` protect telemetry books with mutexes.
     - Go `RWMutex` blocks new readers once a writer is waiting, so a waiting
       `e.mu.Lock()` can amplify long read-lock holds into broad status/admin
       stalls.
   - Current evidence level:
     - Confirmed: full status holds `e.mu.RLock()` while it acquires telemetry
       and run-metrics locks. This is a wide-lock and lock-order fragility.
     - Disputed: the exact AB-BA deadlock cycle is not proven in the current
       code, because several observer paths release telemetry locks before
       taking `e.mu.RLock()`. Some reviewers still identified a plausible
       three-lock cycle involving run metrics. The implementation must prove or
       reject this with a targeted regression test instead of assuming it.
   - This explains admin-status/frontend freezes and some engine silence. It
     does not by itself fully explain watchdog heartbeat loss, because the
     watchdog goroutine does not directly take `e.mu`.

2. Watchdog notify can block forever or silently fail.
   - Evidence:
     - `pkg/web/server_run.go:230` starts the watchdog goroutine.
     - `pkg/web/server_run.go:243` calls `systemd.Watchdog(...)` and ignores
       the returned error.
     - `pkg/systemd/notify.go:61` uses `net.DialUnix` without a deadline.
     - `pkg/systemd/notify.go:66` writes to the notify socket without a
       deadline.
   - If a notify call blocks, the single heartbeat goroutine stops sending
     future watchdog notifications. This is unacceptable even if the root
     application stall is elsewhere.

3. Post-run entity artifact refresh and entity artifact publish have
   no-progress / long-held-lock paths.
   - Evidence:
     - The last large-run log queued entity artifact refresh for 205 feeds.
     - `pkg/engine/entity_refresh_queue.go:68` coalesces feed names.
     - `pkg/engine/entity_refresh_queue.go:75` submits entity refresh work to
       the engine lane.
     - `pkg/engine/entity_refresh_queue.go:230` wraps refresh execution as a
       background task.
     - `pkg/engine/entity_surgical_refresh.go:36` starts optimistic entity
       mutation.
     - `pkg/engine/entity_surgical_refresh.go:46` stages refreshed artifacts,
       with multiple I/O and JSON paths under the same broad operation.
     - `pkg/engine/entity_artifact_publish.go:95` holds `entityArtifactsMu`
       across publish work.
     - `pkg/engine/entity_artifact_publish.go:116`,
       `pkg/engine/entity_artifact_publish.go:122`, and
       `pkg/engine/entity_artifact_publish.go:127` can do filesystem work while
       `entityArtifactsMu` is held.
     - `pkg/engine/entity_artifact_publish.go:131` marks integrity caches stale
       while still inside the entity artifact publication lock.
   - The entity refresh may be the trigger path that hits a lock-order
     fragility, blocks on entity artifact publication, or blocks in filesystem
     I/O while progress logs are silent.

4. Admin full-status/build path can still be too coupled to engine state.
   - Evidence:
     - `pkg/web/admin.go:524` uses `eng.StatusSnapshot()` for full admin status.
     - Full status includes lifetime metrics and broad engine state, not only a
       cheap heartbeat view.
   - Even if light status is cheap, full admin reload or frontend data loading
     can enter the lock cycle and make the admin UI appear completely paused.

5. Missing diagnostic capture hides the true stall source.
   - Evidence:
     - systemd SIGABRT stack dump was incomplete before SIGKILL.
     - No coredump was available.
   - The daemon needs a self-diagnostic mechanism that emits compact goroutine
     and lane/status snapshots before the systemd watchdog window expires, so
     future production stalls preserve actionable evidence.

Pre-implementation gate update:

Status: approved and implemented

The user approved the recommended regression fix path on 2026-06-25. This gate
remains as historical context for the decision point; the active implementation
state is tracked by the implementation updates after this section.

Problem / root-cause model:

- The current best model is a deadlock/stall class, with a confirmed
  watchdog-notify fragility and several lock-order / long-held-lock candidates.
  The status/telemetry/run-metrics cycle is not proven as a strict deadlock in
  the current code. The exact production goroutine stack for the stalled worker
  is not proven because the available stack dump was incomplete.

Evidence reviewed:

- Read-only production systemd logs and service status from 2026-06-24.
- User-supplied Netdata timing observations for idle-CPU/no-process windows.
- `pkg/web/server_run.go:230`
- `pkg/web/server_run.go:243`
- `pkg/systemd/notify.go:52`
- `pkg/systemd/notify.go:61`
- `pkg/systemd/notify.go:66`
- `pkg/engine/status_snapshot.go:62`
- `pkg/engine/status_snapshot.go:76`
- `pkg/engine/run_metrics_state.go:14`
- `pkg/engine/run_metrics_state.go:59`
- `internal/telemetry/timing.go:42`
- `internal/telemetry/counter.go:37`
- `pkg/engine/entity_refresh_queue.go:68`
- `pkg/engine/entity_refresh_queue.go:75`
- `pkg/engine/entity_refresh_queue.go:230`
- `pkg/engine/entity_surgical_refresh.go:36`
- `pkg/engine/entity_surgical_refresh.go:46`
- Official Go documentation for channel blocking, `sync.RWMutex`, and runtime
  global deadlock detection.

Affected contracts and surfaces:

- systemd watchdog availability.
- public `/healthz`.
- public and admin HTTP serving availability.
- full admin status and light admin status.
- engine status snapshots and lifetime telemetry snapshots.
- entity artifact refresh and publish progress.
- runtime diagnostics and production supportability.
- specs and operator docs for watchdog/deadlock observability if behavior
  changes.

Existing patterns to reuse:

- Engine lane typed snapshots and background-task visibility.
- Existing telemetry books, but with corrected lock ordering.
- Existing `systemd` package, but with deadline-bound notify behavior.
- Existing admin light/full status separation.
- Existing SOW-0117 validation style for lane and availability tests.

Risk and blast radius:

- High production availability risk: the current behavior can wedge the daemon
  until systemd kills it.
- Medium diagnostic risk: changing watchdog behavior without observability could
  hide failures.
- Medium compatibility risk: status JSON shape should remain stable unless a
  spec and UI update explicitly changes it.
- Low data-loss risk from the proposed analysis itself; code changes must still
  preserve the 10-year feed history requirement and artifact timestamp
  integrity.

Sensitive data handling plan:

- Durable artifacts must use sanitized production evidence only. Do not write
  raw private hostnames, private endpoints, client IPs, credentials, tokens, or
  raw proprietary operational logs. Record exact dates/times, sanitized event
  classes, code paths, and line references.

Implementation plan:

1. Write behavioral tests that prove or reject the status/telemetry/run-metrics
   lock cycle using public engine/web APIs or package-visible test seams without
   testing private internals as the only signal. Keep tests that enforce the
   broader lock-order rule even if the exact current cycle is rejected.
2. Change status snapshot assembly so no code holds `e.mu` while acquiring
   lifetime telemetry locks, lane locks, scheduler locks, or doing JSON/file
   work.
3. Make `systemd.Notify` deadline-bound and make watchdog notify failures
   visible through logs/counters without introducing noisy normal-operation
   logs.
4. Add watchdog-liveness and admin-full-status responsiveness tests under a
   blocked/slow telemetry or blocked entity-refresh condition.
5. Add stall diagnostics before watchdog expiry: a bounded, sanitized goroutine
   snapshot plus lane/status summary, rate-limited and safe for production logs.
6. Re-audit entity refresh/publish and admin status paths for remaining lock
   order cycles and long synchronous work under global locks.
7. Update specs/docs/skills only where durable contracts change.

Validation plan:

- Unit/behavior tests for status/telemetry lock inversion.
- Unit tests for `systemd.Notify` timeout/deadline behavior.
- Web tests showing full and light admin status return while telemetry/status
  contention exists.
- Engine tests showing entity refresh cannot wedge the lane/status snapshots.
- `go test ./pkg/systemd ./pkg/engine ./pkg/web ./pkg/scheduler -count=1`
- Broader `make test` after implementation.
- Production smoke after deployment, with explicit watchdog timestamp and admin
  status checks.

Artifact impact plan:

- AGENTS.md: likely no update unless a new durable production-debugging rule is
  needed.
- Runtime project skills: likely update `project-operations` and possibly
  `project-coding` with watchdog/deadlock lock-order guidance.
- Specs: likely update `operating-principles.md`, `pipeline.md`,
  `admin-ui.md`, and possibly `memory-management.md` if stall diagnostics or
  status contracts change.
- End-user/operator docs: likely update runtime status / troubleshooting docs
  if new diagnostics or watchdog counters are added.
- End-user/operator skills: likely unaffected.
- SOW lifecycle: this completed SOW is reopened in `.agents/sow/current/` with
  this regression entry. Closure must mark it `Status: completed` and move it
  back to `.agents/sow/done/` together with the implementation commit.

Open-source reference evidence:

- No new open-source reference implementations have been checked yet for this
  regression entry. External reviewers are explicitly requested to perform their
  own analysis and may recommend reference patterns.

Open decisions:

1. Diagnostic capture policy:
   - A. Add bounded pre-watchdog goroutine/lane/status diagnostic logging.
     Pros: preserves evidence before systemd SIGKILL; directly addresses the
     current diagnostic gap. Cons: must sanitize and rate-limit carefully.
     Recommendation: A, long-term-best.
   - B. Only fix suspected code paths and rely on systemd stack dumps.
     Pros: less code. Cons: production already proved stack dumps can be
     incomplete before SIGKILL.
2. Watchdog notify policy:
   - A. Make notify calls deadline-bound and observable.
     Pros: prevents the heartbeat goroutine from wedging on notify I/O; improves
     operator evidence. Cons: requires careful timeout choice and tests.
     Recommendation: A, long-term-best.
   - B. Leave notify as-is and focus only on engine locks.
     Pros: narrower. Cons: leaves a known heartbeat fragility in place.
3. Status snapshot policy:
   - A. Treat status/admin snapshot code as a free-lane critical path and forbid
     nested acquisition of engine, lane, telemetry, scheduler, or filesystem
     locks while holding another global lock.
     Pros: fixes the concrete lock-order candidate and reduces future deadlock
     risk. Cons: may require small refactors in status assembly.
     Recommendation: A, long-term-best.
   - B. Patch only the currently suspected lifetime metrics call.
     Pros: surgical. Cons: likely misses similar future cycles.

Reviewer task:

- The SOW must be reviewed by `glm`, `minimax`, `mimo`, `kimi`, `deepseek`, and
  `qwen`.
- Reviewers must independently analyze potential deadlock/stall sources, verify
  or reject the hypotheses above, identify missing evidence/tests, and propose
  fixes without modifying files.

Validation status:

- Pending external review and implementation.

### External Review - 2026-06-25

Reviewers run:

- `glm`
- `minimax`
- `mimo`
- `kimi`
- `deepseek`
- `qwen`

Reviewer consensus:

1. Deadline-bound watchdog notification is the highest-leverage fix.
   - Evidence:
     - `pkg/systemd/notify.go:52`
     - `pkg/systemd/notify.go:61`
     - `pkg/systemd/notify.go:66`
     - `pkg/web/server_run.go:243`
   - Consensus: one blocked notify call can stop the single watchdog heartbeat
     goroutine from sending future notifications. Ignoring notify errors also
     leaves no production evidence.

2. Pre-watchdog diagnostics are mandatory.
   - Evidence:
     - The production stack dump was incomplete before SIGKILL.
     - No coredump was available.
   - Consensus: the daemon needs a bounded, sanitized, rate-limited diagnostic
     path before systemd kills it. The dump should include goroutine stacks,
     lane state, watchdog notify state, runtime stats, and compact queue/status
     summaries.

3. Full admin status remains a wide-lock and heavy-work path.
   - Evidence:
     - `pkg/web/admin.go:262`
     - `pkg/web/admin.go:521`
     - `pkg/web/admin.go:524`
     - `pkg/engine/status_snapshot.go:56`
     - `pkg/engine/status_snapshot.go:118`
     - `pkg/web/sysinfo.go:89`
     - `pkg/web/sysinfo.go:160`
   - Consensus: default or full admin status can still take broad engine,
     telemetry, runtime, and `/proc` work. Light status must be the hot polling
     path, and full status must be explicit slow-path/operator drill-down work.

4. `Engine.Reload()` holds `e.mu.Lock()` across filesystem work.
   - Evidence:
     - `pkg/engine/engine.go:253`
     - `pkg/engine/engine.go:298`
   - Consensus: this does not directly block the watchdog, but under slow I/O it
     can block all `e.mu.RLock()` readers and make admin/status paths appear
     deadlocked.

5. Entity artifact publication holds a global artifact lock across I/O.
   - Evidence:
     - `pkg/engine/entity_artifact_publish.go:95`
     - `pkg/engine/entity_artifact_publish.go:116`
     - `pkg/engine/entity_artifact_publish.go:122`
     - `pkg/engine/entity_artifact_publish.go:127`
     - `pkg/engine/entity_artifact_publish.go:131`
     - `pkg/engine/run_pipeline.go:400`
     - `pkg/engine/run_pipeline.go:409`
   - Consensus: no reverse lock cycle was proven, but this is a long-held-lock
     and I/O-pressure amplifier. Integrity-cache stale marking should not happen
     inside the artifact publication lock if it can be safely moved after
     unlock.

6. Several continuation / cleanup submissions use `context.Background()`.
   - Evidence:
     - `pkg/engine/entity_refresh_queue.go:338`
     - `pkg/engine/entity_refresh_queue.go:383`
     - `pkg/engine/engine.go:303`
   - Consensus: background submissions must either use the daemon/caller
     context or explicitly document why they outlive shutdown. Unbounded
     continuations can delay clean shutdown and make watchdog recovery harder.

Reviewer disagreement:

- Status/telemetry/run-metrics was disputed.
  - `glm`, `minimax`, `deepseek`, and `qwen` rejected the exact
    status/telemetry AB-BA cycle as proven in the current code.
  - `mimo` and `kimi` identified a stronger possible cycle involving
    `runMetrics.mu` and telemetry locks.
  - SOW conclusion: treat this as a confirmed lock-order/wide-lock defect and
    a disputed deadlock hypothesis. The implementation must remove nested
    global lock acquisition from status/diagnostic paths and add tests that
    either reproduce the cycle or prove the desired lock-order contract.

Additional findings added to scope:

1. `Runner.TriggerSources()` can block HTTP callers on a full scheduler action
   channel.
   - Evidence:
     - `pkg/scheduler/scheduler.go:67`
     - `pkg/scheduler/scheduler.go:111`
     - `pkg/scheduler/scheduler.go:115`
   - Impact: if the fetch loop stalls, trigger endpoints can block handler
     goroutines indefinitely. This does not directly stop the watchdog, but it
     can make the website/admin API appear frozen.
   - Required fix: replace bare channel send with non-blocking or
     context-bounded queueing, using the existing queued-action pattern where
     appropriate.

2. Work lane shutdown sends on item `syncStart` channels while holding
   `WorkLane.mu`.
   - Evidence:
     - `pkg/engine/work_lane.go:307`
     - `pkg/engine/work_lane.go:349`
     - `pkg/engine/work_lane.go:407`
     - `pkg/engine/work_lane.go:411`
   - Impact: reviewers disagreed on whether this is a current deadlock, but
     channel sends while holding the lane mutex are a liveness smell.
   - Required fix: prove current behavior with tests and, if needed, snapshot
     queued items under the lane mutex and perform sends after unlock or use
     safe non-blocking sends.

3. Lane callbacks have no slot-hold diagnostic threshold.
   - Evidence:
     - `pkg/engine/work_lane.go:471`
     - `pkg/engine/work_lane.go:481`
   - Impact: a callback that blocks in I/O or ignores context can occupy the
     only engine slot indefinitely.
   - Required fix: add warning diagnostics for lane slot hold time and audit
     callbacks for context checks at I/O boundaries. Do not force arbitrary
     cancellation that can corrupt artifacts.

4. `AttachContext` is not idempotent.
   - Evidence:
     - `pkg/engine/work_lane.go:351`
     - `pkg/engine/work_lane.go:359`
   - Impact: production is expected to attach once, but future repeated attach
     calls could create shutdown races.
   - Required fix: guard with `sync.Once` or document and test single-call
     semantics.

5. Full/light admin status contract still needs verification in the UI.
   - Evidence:
     - `pkg/web/admin_status_light.go:22`
     - `pkg/web/admin_status_light.go:27`
     - `pkg/web/admin.go:674`
     - `pkg/web/admin.go:705`
   - Impact: at least one reviewer reported that light status still builds
     feed summaries by iterating configured sources. This must be verified. If
     true, light status is not cheap enough for high-frequency polling.
   - Required fix: audit UI polling and backend light-status construction; keep
     the hot path summary-only and cache-backed.

6. Progress diagnostics can block on the same locks as status snapshots.
   - Evidence:
     - `pkg/engine/run_diagnostics.go:117`
     - `pkg/engine/run_diagnostics.go:178`
     - `pkg/engine/run_diagnostics.go:234`
     - `pkg/engine/run_diagnostics.go:248`
   - Impact: progress logging should not depend on the same wide snapshot path
     when diagnosing stalls.
   - Required fix: use a coarse progress snapshot during active work and reserve
     full snapshots for summaries or explicit diagnostics.

7. `tryMarkRunStart` and lane-limit behavior need a cancellation proof.
   - Evidence:
     - `pkg/engine/run.go:220`
     - `pkg/engine/run.go:280`
   - Impact: if lane concurrency is greater than one, a second admitted run
     must not occupy a lane slot while waiting indefinitely for the run guard.
   - Required fix: verify the wait path honors `ctx.Done()` and releases the
     lane slot in all outcomes.

Open decisions after review:

1. Diagnostic capture policy:
   - A. Enable bounded pre-watchdog diagnostics by default.
     Pros: preserves evidence before SIGKILL and directly addresses the current
     observability gap. Cons: stack dumps can be large and must be sanitized and
     rate-limited.
     Recommendation: A, long-term-best.
   - B. Enable diagnostics behind an environment flag for the first release.
     Pros: lower log-noise risk. Cons: the next production stall may again have
     no evidence if the flag is not enabled.
   - C. Do not add diagnostics.
     Pros: least code. Cons: repeats the current blind failure mode.

2. Watchdog notify policy:
   - A. Add deadline-bound notify calls and visible notify failure counters.
     Pros: prevents a wedged notify syscall from permanently stopping
     heartbeats. Cons: timeout must be chosen carefully.
     Recommendation: A, long-term-best.
   - B. Only log notify errors.
     Pros: narrower. Cons: does not prevent blocking writes.
   - C. Leave notify behavior unchanged.
     Pros: no code change. Cons: leaves the most direct watchdog-kill vector.

3. Status/admin lock policy:
   - A. Treat status/admin/diagnostic snapshots as availability-critical and
     forbid holding one global lock while acquiring telemetry, lane, scheduler,
     runtime, or filesystem locks.
     Pros: reduces current stalls and future deadlock risk. Cons: status
     snapshots may become slightly less point-in-time consistent.
     Recommendation: A, long-term-best.
   - B. Patch only the lifetime telemetry call.
     Pros: smaller change. Cons: leaves similar run-metrics and diagnostic
     paths.

4. Scheduler trigger policy:
   - A. Make scheduler trigger sends non-blocking or context-bounded.
     Pros: protects HTTP handlers when the downloader/fetch loop stalls. Cons:
     requires explicit dropped/coalesced-trigger semantics.
     Recommendation: A, long-term-best.
   - B. Keep blocking sends and rely on the queue buffer.
     Pros: current semantics unchanged. Cons: a full channel can freeze caller
     goroutines.

5. Admin status hot-path policy:
   - A. Make light status the high-frequency/default polling path and require
     full status to be explicit slow-path work.
     Pros: protects the admin UI during heavy processing. Cons: any existing
     consumer depending on full status as the default may need a documented
     endpoint/mode.
     Recommendation: A, long-term-best.
   - B. Only change the React UI to request light status.
     Pros: smaller UI-only change. Cons: stale tabs, scripts, or monitoring can
     still hit the heavy default path.

Validation additions after review:

- Add tests proving `systemd.Watchdog` / notify returns within deadline and
  reports errors.
- Add an HTTP handler test proving scheduler trigger calls do not block when
  the scheduler action channel is full.
- Add lock-order / responsiveness tests for status snapshots under blocked
  telemetry/run-metrics snapshots.
- Add tests for work-lane shutdown with queued and active work.
- Add tests for entity refresh continuation cancellation on daemon shutdown.
- Add tests proving light admin status avoids full feed aggregation and heavy
  runtime/proc capture on the polling path.
- Add tests that progress diagnostics can still emit coarse progress while full
  snapshots are blocked.

Implementation readiness after review:

- The SOW is ready to move into implementation only after the user confirms the
  reviewed decisions above, or explicitly delegates all long-term-best choices
  to implementation.
- The first implementation slice should be watchdog notify deadlines plus
  pre-watchdog diagnostics, because that removes the silent-kill failure mode
  and preserves evidence if another stall remains.

### User Direction - 2026-06-25

The user directed the assistant to use the following process until no more
deadlock/stall findings remain:

1. Perform gap analysis for potential deadlocks/stalls with external reviewers.
2. Verify every finding against the current code.
3. Build a fix plan.
4. Review the plan with external reviewers and tune it iteratively.
5. Implement the fixes.
6. Review the fixes with external reviewers and tune them iteratively.
7. Repeat the gap analysis on the new baseline.
8. Stop only when gap analysis cannot find anything else to fix.

Decision resolution:

- The user explicitly prioritizes application liveness over narrower surgical
  scope for this regression. The implementation should take the long-term-best
  choices from the reviewed decision list, while still rejecting findings that
  are disproven by code evidence.
- The active target is not merely "fix the last watchdog kill"; it is to remove
  verified deadlock/stall classes, add tests that would have caught them, and
  repeat external gap analysis until it is clean.

### Verification Pass - 2026-06-25

Confirmed findings:

1. Watchdog notify can block or fail without visibility.
   - Evidence:
     - `pkg/systemd/notify.go:52` enters `Notify`.
     - `pkg/systemd/notify.go:61` dials the Unix datagram socket without a
       deadline.
     - `pkg/systemd/notify.go:66` writes without a deadline.
     - `pkg/web/server_run.go:230` starts the watchdog goroutine.
     - `pkg/web/server_run.go:243` discards the watchdog notification error.
   - Verification result: confirmed.

2. There is no pre-watchdog diagnostic capture.
   - Evidence:
     - `pkg/web/server_run.go:230` only sends watchdog heartbeats.
     - No production stack/coredump evidence survived the systemd kill window.
   - Verification result: confirmed.

3. Full admin status is still the default backend path.
   - Evidence:
     - `pkg/web/admin.go:262` uses light status only when `mode=light`.
     - `pkg/web/admin.go:265` uses full status for default requests.
     - `pkg/web/admin.go:521` builds full admin status.
     - `pkg/web/admin.go:522` calls `detailedStatus()`.
     - `pkg/web/admin.go:524` calls `eng.StatusSnapshot()`.
   - Verification result: confirmed. The React admin client already polls
     `?mode=light`, but stale tabs, scripts, monitors, or manual default route
     calls can still hit the heavy path.

4. Light admin status still builds full feed rows just to summarize counters.
   - Evidence:
     - `pkg/web/admin_status_light.go:27` calls
       `buildAdminFeedsWithStatusEntries`.
     - `pkg/web/admin.go:674` starts the full feed-row builder.
     - `pkg/web/admin.go:699` iterates all configured sources.
     - `pkg/web/admin.go:742` builds full `adminFeed` rows.
   - Verification result: confirmed. The UI needs the global counters, but the
     hot path should compute only the summary, not full feed rows and merge
     metadata.

5. Full status holds `e.mu.RLock()` while acquiring run metrics and lifetime
   telemetry locks.
   - Evidence:
     - `pkg/engine/status_snapshot.go:62` takes `e.mu.RLock()`.
     - `pkg/engine/status_snapshot.go:66` snapshots current run metrics while
       holding `e.mu.RLock()`.
     - `pkg/engine/status_snapshot.go:76` snapshots lifetime metrics while
       holding `e.mu.RLock()`.
     - `pkg/engine/run_metrics.go:214` takes `runMetrics.mu`.
     - `pkg/engine/run_metrics.go:230`, `pkg/engine/run_metrics.go:240`,
       `pkg/engine/run_metrics.go:246`, `pkg/engine/run_metrics.go:301`, and
       `pkg/engine/run_metrics.go:302` take telemetry book snapshots.
     - `internal/telemetry/timing.go:60` and
       `internal/telemetry/counter.go:52` take telemetry book mutexes.
   - Verification result: confirmed as a wide-lock / lock-order defect. The
     exact AB-BA deadlock remains unproven on the current baseline.

6. Run completion takes a final metrics snapshot while holding `e.mu.Lock()`.
   - Evidence:
     - `pkg/engine/run.go:254` takes `e.mu.Lock()`.
     - `pkg/engine/run.go:269` calls `e.currentMetrics.finish()`.
     - `pkg/engine/run.go:270` calls `e.currentMetrics.snapshot(false)`.
   - Verification result: confirmed. This is the same lock-order class as full
     status and happens at every run end.

7. Progress diagnostics can block on the same metrics locks as status.
   - Evidence:
     - `pkg/engine/run_diagnostics.go:117` logs progress.
     - `pkg/engine/run_diagnostics.go:121` calls
       `currentRunDiagnosticsSnapshot`.
     - `pkg/engine/run_diagnostics.go:247` calls `current.snapshot(true)`.
   - Verification result: confirmed. The diagnostic path is less dangerous than
     full status because it releases `e.mu` before metrics snapshot, but it can
     still block on metrics/telemetry locks while trying to report progress.

8. `Engine.Reload()` holds `e.mu.Lock()` across filesystem and rebuild work.
   - Evidence:
     - `pkg/engine/engine.go:253` takes `e.mu.Lock()`.
     - `pkg/engine/engine.go:279` calls `ensureDirectories` while locked.
     - `pkg/engine/engine.go:285` calls `bootstrapMissingEntriesFromDisk`
       while locked.
     - `pkg/engine/engine.go:288` calls `repairInvalidEntryTimestamps` while
       locked.
     - `pkg/engine/engine.go:291` calls `bootstrapLegacyFailureStarts` while
       locked.
     - `pkg/engine/engine.go:298` releases the lock.
   - Verification result: confirmed.

9. Reload/status/config access has an unprotected pointer-read risk.
   - Evidence:
     - `pkg/engine/engine.go:253` to `pkg/engine/engine.go:256` swaps
       `e.cfg` and `e.runtime` under `e.mu.Lock()`.
     - `pkg/engine/engine.go:316` returns `e.cfg` without a lock.
     - `pkg/engine/engine.go:320` returns `e.runtime` without a lock.
     - Admin and scheduler code call `eng.Config()` and `eng.Runtime()` from
       concurrent request/worker paths.
   - Verification result: confirmed as a race risk. It is not itself a
     deadlock, but it belongs in this liveness/crash regression because reload
     can run concurrently with admin/status work.

10. Entity artifact publication holds `entityArtifactsMu` across I/O and nested
    integrity-cache lock acquisition.
    - Evidence:
      - `pkg/engine/entity_artifact_publish.go:95` takes
        `entityArtifactsMu`.
      - `pkg/engine/entity_artifact_publish.go:96` observes telemetry while the
        artifact lock is held.
      - `pkg/engine/entity_artifact_publish.go:116` publishes entity files
        while locked.
      - `pkg/engine/entity_artifact_publish.go:122` publishes web files while
        locked.
      - `pkg/engine/entity_artifact_publish.go:127` syncs generated files while
        locked.
      - `pkg/engine/entity_artifact_publish.go:131` calls
        `MarkIntegrityCachesStale()` while locked.
      - `pkg/engine/integrity_cache.go:300` and
        `pkg/engine/integrity_cache.go:308` take integrity-cache locks.
    - Verification result: confirmed as a long-held-lock / lock-order
      amplifier. No reverse deadlock cycle was proven.

11. Pipeline publish also holds `entityArtifactsMu` across entity publication
    I/O.
    - Evidence:
      - `pkg/engine/run_pipeline.go:400` takes `entityArtifactsMu`.
      - `pkg/engine/run_pipeline.go:401` computes publish work total while
        locked.
      - `pkg/engine/run_pipeline.go:404` publishes entity artifacts while
        locked.
      - `pkg/engine/run_pipeline.go:409` releases the lock.
    - Verification result: confirmed.

12. Entity refresh continuations submit with `context.Background()`.
    - Evidence:
      - `pkg/engine/entity_refresh_queue.go:338`
      - `pkg/engine/entity_refresh_queue.go:383`
    - Verification result: confirmed.

13. Reload cleanup submits with `context.Background()`.
    - Evidence:
      - `pkg/engine/engine.go:303`
    - Verification result: confirmed.

14. `Runner.TriggerSources()` can block on a full channel.
    - Evidence:
      - `pkg/scheduler/scheduler.go:67` creates `actionCh` with buffer 64.
      - `pkg/scheduler/scheduler.go:111` enters `TriggerSources`.
      - `pkg/scheduler/scheduler.go:112` does a bare send.
      - HTTP handlers call `TriggerSources` from admin and public action paths.
    - Verification result: confirmed.

15. Work-lane callbacks have no slot-hold diagnostic threshold.
    - Evidence:
      - `pkg/engine/work_lane.go:471`
      - `pkg/engine/work_lane.go:481`
    - Verification result: confirmed. This is a diagnostic gap, not a safe
      generic cancellation point.

16. `AttachContext` is not idempotent.
    - Evidence:
      - `pkg/engine/work_lane.go:351`
      - `pkg/engine/work_lane.go:359`
    - Verification result: confirmed as a low-risk lifecycle hardening item.

Rejected or downgraded findings:

1. `tryMarkRunStart` does not currently have an indefinite wait loop.
   - Evidence:
     - `pkg/engine/run.go:234` enters `tryMarkRunStart`.
     - `pkg/engine/run.go:235` takes `e.mu.Lock()`.
     - `pkg/engine/run.go:237` returns `false` immediately if already running.
   - Verification result: rejected for the current baseline. Keep no-op beyond
     ensuring future lane-limit tests continue to prove rejected duplicate runs
     release their slots.

2. Work-lane shutdown sends on queued `syncStart` channels while holding
   `WorkLane.mu`, but the currently identified deadlock cycle is not proven.
   - Evidence:
     - `pkg/engine/work_lane.go:321` sends only for queued `Run` items.
     - `pkg/engine/work_lane.go:407` removes an item from the queue before
       sending start.
   - Verification result: downgraded. It is still worth hardening so channel
     sends never happen under `WorkLane.mu`, but the exact reviewer deadlock is
     not established for the current implementation.

### Implementation Plan V1 - 2026-06-25

This plan is intentionally ordered to add tests before implementation and to
remove silent failure first.

1. Watchdog notification and diagnostics.
   - Add a deadline-bound notify API in `pkg/systemd`.
   - Use the bounded API for watchdog heartbeats and preferably all systemd
     notifications that run on daemon lifecycle paths.
   - Log watchdog notify failures through `slog` with bounded rate limiting.
   - Add a pre-watchdog diagnostic emitter that records a bounded goroutine
     stack sample, lane snapshot, runtime stats, and compact scheduler/queue
     state on notify failure or slow notify.
   - Tests:
     - payload compatibility for normal notify calls;
     - deadline/error behavior for bounded notify;
     - watchdog loop does not silently discard notify errors;
     - diagnostic emission is bounded and rate-limited.

2. Scheduler trigger liveness.
   - Replace bare `TriggerSources` channel sends with non-blocking or
     context-bounded queueing.
   - Preserve explicit accepted/rejected semantics for HTTP/admin callers.
   - Keep startup/internal callers observable through warnings/counters when an
     action cannot be queued.
   - Tests:
     - full action channel cannot block a handler;
     - accepted trigger still wakes downloader/processor loops;
     - rejected trigger returns a clear HTTP conflict/service-unavailable state
       where applicable.

3. Status/admin hot-path lock cleanup.
   - Make default admin status use the light payload; keep full status behind
     `?mode=full`.
   - Refactor light feed counters to compute a summary directly from config,
     cache entries, scheduler snapshot, and live queue state without building
     full `adminFeed` rows.
   - Refactor full `StatusSnapshot` so it copies engine fields and metric
     pointers under `e.mu`, then snapshots run metrics and lifetime telemetry
     after releasing `e.mu`.
   - Refactor `markRunEnd` so it detaches `currentMetrics` under `e.mu`, then
     finishes and snapshots it outside `e.mu`, then stores `lastMetrics` under a
     short lock.
   - Tests:
     - default admin status omits full-only fields while `?mode=full` keeps
       them;
     - light summary matches full summary for representative fixtures;
     - status snapshot remains responsive when metrics snapshot is blocked by a
       test seam;
     - run-end metrics snapshot no longer happens while `e.mu` is held.

4. Reload lock/race cleanup.
   - Make `Config()` and `Runtime()` read `e.cfg` and `e.runtime` under
     `e.mu.RLock()`.
   - Minimize `Reload()`'s exclusive section: swap config/runtime and state
     pointers under lock, but move filesystem work that can safely use the new
     immutable config/runtime outside the long critical section where possible.
   - Keep config/runtime state publication atomic from caller perspective.
   - Tests:
     - concurrent reload/status/admin snapshot under `-race`;
     - reload still updates worker limits and stale cleanup submission;
     - status remains available while reload filesystem repair is delayed by a
       test seam, if a seam is needed.

5. Entity artifact publication lock narrowing.
   - Move telemetry observation out of `entityArtifactsMu` critical sections.
   - Move `MarkIntegrityCachesStale()` after unlock where the artifact
     generation invariant permits it.
   - Avoid holding `entityArtifactsMu` while computing publish work totals when
     safe.
   - Add lane slot-hold warnings so future filesystem stalls are visible.
   - Tests:
     - stale-stage retry behavior remains correct under concurrent generation
       changes;
     - integrity caches become stale after successful publication;
     - entity writer lock-hold telemetry is still emitted without being inside
       the lock;
     - slot-hold warning appears for a deliberately blocked lane callback.

6. Context and work-lane lifecycle hardening.
   - Pass caller/daemon context into entity refresh continuation submissions and
     reload cleanup, or document any intentionally longer-lived submission.
   - Make `AttachContext` single-call safe or explicitly idempotent.
   - Move queued `syncStart` shutdown notifications out from under
     `WorkLane.mu`, or prove with tests that non-blocking sends are sufficient.
   - Tests:
     - shutdown cancels queued continuation submissions;
     - multiple `AttachContext` calls do not race or spawn competing shutdowns;
     - shutdown with queued blocking `Run` items returns without a mutex/channel
       deadlock.

7. Post-fix validation and repeat gap analysis.
   - Run targeted package tests after each slice.
   - Run `go test -race -count=1 ./pkg/systemd ./pkg/scheduler ./pkg/engine ./pkg/web`.
   - Run `make test-strict`.
   - Run `make test` and broader gates required by changed files.
   - Run the same external reviewer set on the code diff.
   - Repeat the gap analysis on the new baseline. Continue until no verified
     deadlock/stall finding remains.

External plan-review status:

- Reviewed by `glm`, `minimax`, `mimo`, `kimi`, `deepseek`, and `qwen` on
  2026-06-25. All reviewers rejected Plan V1 as not yet production grade.

### External Plan Review - 2026-06-25

Consensus accepted into the SOW:

1. Systemd notifications need a concrete deadline policy, and the bounded path
   must apply to watchdog, ready, stopping, and status notifications. Evidence:
   `pkg/systemd/notify.go:12` to `pkg/systemd/notify.go:38` all route through
   unbounded `Notify()`, while `pkg/systemd/notify.go:61` and
   `pkg/systemd/notify.go:66` can block on socket dial/write.
2. Watchdog failures need bounded diagnostics with an explicit content policy.
   The diagnostics may include runtime counters, lane/scheduler snapshots, and a
   capped goroutine stack sample. They must not include request bodies,
   secrets, raw feed contents, or unbounded path/list dumps.
3. `TriggerSources` has two different contracts. Startup/integrity recovery is
   must-deliver and must use context-bounded queueing. HTTP/admin callers must
   not block a handler and should get an explicit rejected/conflict response
   when the queue cannot accept work. Evidence:
   `pkg/web/server_run.go:109` and `pkg/web/server_run.go:116` call
   `TriggerSources()` for startup recovery, while user/API handlers also call it
   from `pkg/web/admin.go`, `pkg/web/integrity.go`, and `pkg/web/routes.go`.
4. The admin-status default change is a user-visible contract change. If default
   status becomes light, `?mode=full` must preserve the full payload and specs,
   docs, UI polling assumptions, and operator notes must be updated in the same
   change.
5. `StatusSnapshotLight()` is also part of the hot path and must minimize
   `e.mu.RLock()` hold time. Evidence: `pkg/engine/status_snapshot.go:15`
   through `pkg/engine/status_snapshot.go:53` holds `e.mu.RLock()` for the full
   light struct construction. Go `RWMutex` writer preference means a pending
   writer blocks new readers, so long writer holds in `markRunEnd()` or
   `Reload()` can still freeze light status.
6. The metrics/telemetry observer side must be included in the status lock
   cleanup, even though the exact AB-BA deadlock remains unproven. Evidence:
   `pkg/engine/run_metrics_state.go:14` and
   `pkg/engine/run_metrics_state.go:59` take lifetime telemetry locks before a
   later `e.mu.RLock()` at `pkg/engine/run_metrics_state.go:15` and
   `pkg/engine/run_metrics_state.go:60`. The telemetry lock itself is released
   before `e.mu.RLock()`, so the strict reviewer claim of a proven telemetry
   AB-BA deadlock is rejected for the current baseline. The lock-order shape is
   still confusing and should be simplified.
7. Background task progress updates amplify `e.mu` contention. Evidence:
   `pkg/engine/background_tasks.go:82` to `pkg/engine/background_tasks.go:101`
   and `pkg/engine/background_tasks.go:104` to `pkg/engine/background_tasks.go:110`
   lock `e.mu` on every update/finish.
8. Progress logging samples runtime and process state directly on each progress
   tick. Evidence: `pkg/engine/run_diagnostics.go:121` to
   `pkg/engine/run_diagnostics.go:124` calls `captureEngineRuntimeStats()`, and
   `pkg/engine/run_diagnostics.go:382` to `pkg/engine/run_diagnostics.go:400`
   calls `runtime.ReadMemStats()` plus `/proc` readers. This is a stall source
   and should use a cached sample.
9. Full admin status can still trigger runtime/sysinfo capture on the request
   path. Evidence: `pkg/web/admin.go:522` calls `detailedStatus()`, and
   `pkg/web/sysinfo.go:89` to `pkg/web/sysinfo.go:108` can call
   `captureDetailedStatus()` on cache miss.
10. Reload also establishes `e.mu -> WorkLane.mu` lock order by calling
    `SetLimit()` while holding `e.mu`. Evidence: `pkg/engine/engine.go:253`
    starts the exclusive reload section and `pkg/engine/engine.go:263` calls
    `e.engineLane.SetLimit(...)`; `pkg/engine/work_lane.go:263` takes the lane
    mutex. The plan must avoid introducing any reverse `WorkLane.mu -> e.mu`
    path.
11. `Reload()` context plumbing must be explicit. Evidence:
    `pkg/engine/engine.go:303` uses `context.Background()` for reload cleanup,
    while daemon startup has a real service context at
    `cmd/update-ipsets/daemon.go:77`.
12. Entity artifact lock narrowing must cover both publish paths and state the
    cache-staleness invariant. Evidence:
    `pkg/engine/entity_artifact_publish.go:95` to
    `pkg/engine/entity_artifact_publish.go:131` and
    `pkg/engine/run_pipeline.go:400` to `pkg/engine/run_pipeline.go:409` both
    hold `entityArtifactsMu` across publish I/O or nested cache work.
13. Work-lane `syncStart` sends under `WorkLane.mu` need hardening in both
    `Shutdown()` and `scheduleLocked()`. Evidence:
    `pkg/engine/work_lane.go:321` to `pkg/engine/work_lane.go:322` and
    `pkg/engine/work_lane.go:409` to `pkg/engine/work_lane.go:410`.
14. The context audit must cover all production `context.Background()` uses, not
    only entity refresh continuations. Each use must be classified as
    daemon-owned, request/CLI-owned, local short helper, or test-only.
15. Run exit holds the engine lane slot while saving cache and logging final
    diagnostics. Evidence: `pkg/engine/run.go:84` to `pkg/engine/run.go:98`
    runs `cache.Save()`, final logging, diagnostic summary, and `markRunEnd()`
    inside the admitted run callback.

Reviewer claims rejected or constrained:

1. `make test-strict` does exist and is a CI target. Evidence:
   `Makefile:58` defines it and `.github/workflows/ci.yml:72` runs it.
2. The strict "proven telemetry AB-BA deadlock" claim is not accepted as a fact
   without a reproducer. Evidence:
   `internal/telemetry/timing.go:42` to `internal/telemetry/timing.go:43` and
   `internal/telemetry/counter.go:37` to `internal/telemetry/counter.go:38`
   release telemetry locks before observer code reaches `e.mu.RLock()`. The
   fix will still remove the confusing observer lock shape and avoid holding
   `e.mu` while taking telemetry snapshots.
3. Tests that only assert private helper call counts are rejected as brittle.
   Where a same-package test is needed for a lock invariant, it must explain the
   externally observable liveness contract it protects.
4. History-retention safety remains mandatory, but this regression fix should not
   edit history migration, retention deletion, or feed-history serialization
   paths unless a verified liveness bug is found there. Any later touch to those
   paths requires explicit retention-preservation tests.

### Regression Pre-Implementation Gate Update - 2026-06-25

Purpose:

- Long-term-best fix for watchdog kills and admin/API freezes during heavy
  processing. The daemon must remain observable and stoppable while engine,
  artifact, integrity, and downloader work continue under bounded concurrency.

Root-cause model:

- Fact: production showed watchdog kills after multi-minute silence and idle CPU
  windows, not obvious CPU saturation.
- Fact: the watchdog goroutine can block forever in an unbounded systemd notify
  call and currently discards errors.
- Fact: status, reload, entity publish, background task updates, diagnostics,
  and lane callbacks contain several wide critical sections and unbounded I/O or
  telemetry work under shared locks.
- Working theory: the observed production pauses are one or more stalls rather
  than a classic CPU-bound deadlock: a blocked notify path, lock convoy, stalled
  filesystem I/O, runtime/sysinfo sampling, or blocked queue send can make the
  service silent until systemd kills it.
- Unproven: a single exact AB-BA deadlock cycle. The implementation must remove
  the verified stall/lock-order classes and add diagnostics so any remaining
  stall has evidence.

Affected contracts and surfaces:

- Daemon systemd lifecycle: ready/stopping/status/watchdog notifications.
- Admin API/UI: `/api/v1/admin/status` default payload and `?mode=full`
  compatibility.
- Scheduler queueing: must-deliver startup recovery vs bounded HTTP/admin
  admission.
- Engine status, reload, run lifecycle, entity artifact publication, work-lane
  shutdown, diagnostics, and background task visibility.
- Specs/docs: `.agents/sow/specs/admin-ui.md`,
  `.agents/sow/specs/operating-principles.md`,
  `.agents/sow/specs/pipeline.md`, and operator runtime status docs if present.

Sensitive data handling:

- SOW/spec/docs may record sanitized file paths and line numbers.
- Do not write raw production logs, request bodies, raw feed data, secrets,
  public/non-private identifying IPs, or host-specific incident details to
  durable artifacts.
- Pre-watchdog diagnostics must be capped and sanitized by design.

Risk and blast radius:

- High runtime-safety blast radius because this touches daemon lifecycle,
  scheduler admission, engine locks, and admin status.
- Admin status default change is intentional but breaking for clients that
  expected full payload by default. Preserve `?mode=full`, document the change,
  and update the UI to request the intended mode explicitly.
- Lock narrowing around entity publish can introduce transient integrity-cache
  windows. The accepted invariant is: artifact generation is bumped while
  `entityArtifactsMu` is held; cache stale marking may happen after unlock as a
  conservative invalidation, because a stale mark after a momentarily fresh scan
  forces a later refresh instead of hiding corruption.

Validation plan:

- Add behavioral tests before or with each implementation slice.
- Required focused gates: targeted `go test`, targeted `go test -race`, and
  `make test-strict`.
- Required broad gates before closure: `make test`, `make race` if runtime is
  acceptable, and relevant web/UI tests if admin UI code changes.
- External reviewers must review the tuned plan, then the code diff, and the
  gap analysis repeats on the new baseline until no verified liveness finding
  remains.

### Implementation Plan V2 - 2026-06-25

Plan V2 supersedes V1.

1. Watchdog and systemd notification safety.
   - Add deadline-bound notify support in `pkg/systemd`.
   - Use the bounded path for ready, stopping, status, and watchdog messages.
   - Deadline policy: default notify deadline is the smaller of 2 seconds and
     half the systemd watchdog interval when the watchdog is active; lifecycle
     calls without a watchdog use a small fixed deadline.
   - The watchdog loop must log/counter notify errors with rate limiting and
     emit a bounded pre-watchdog diagnostic on notify failure or slow notify.
   - Tests:
     - bounded notify returns within deadline on a blocked/unresponsive socket;
     - ready/stopping/status/watchdog payload compatibility;
     - watchdog loop does not discard notify errors;
     - diagnostics are bounded, sanitized, and rate-limited.

2. Scheduler action admission contracts.
   - Split must-deliver startup/internal queueing from non-blocking HTTP/admin
     queueing.
   - Startup integrity recovery must use context-bounded queueing and must not
     silently drop recovery actions.
   - HTTP/admin trigger handlers must not block indefinitely; on full queue they
     return an explicit conflict/service-unavailable response and preserve
     operator visibility.
   - Tests:
     - startup recovery actions are delivered or return a visible error under a
       full action channel;
     - HTTP/admin trigger path returns promptly on a full channel;
     - accepted triggers still wake downloader and processing loops.

3. Admin status and engine metrics liveness.
   - Make the default admin status payload light; preserve the existing full
     payload behind `?mode=full`.
   - Update specs/docs/UI polling assumptions in the same change.
   - Replace light feed summary construction with a direct summary path that
     does not build full feed rows.
   - Minimize `StatusSnapshotLight()` and full `StatusSnapshot()` `e.mu`
     critical sections by copying pointers/scalars under lock and doing metrics,
     lifetime telemetry, and larger derived snapshots after unlock.
   - Refactor `markRunEnd()` to detach `currentMetrics` under lock, finish and
     snapshot metrics outside `e.mu`, then publish `lastMetrics` under a short
     lock.
   - Simplify observer-side metric updates so high-frequency
     `observeRunOperation`, `observeRunCounter`, `observeRunOperationAggregate`,
     `observeFeedOperation`, and `observeFeedWork` do not add avoidable `e.mu`
     contention. Preferred implementation is an atomic current-metrics pointer;
     otherwise use a single documented lock order and tests.
   - Move full-status sysinfo collection to cached sampling unless explicitly
     refreshed by a non-hot operator path.
   - Replace progress-log runtime/sysinfo capture with cached runtime samples.
   - Tests:
     - default status is light and `?mode=full` preserves full shape;
     - light feed summary equals full summary for representative fixtures;
     - light status stays responsive while metrics/sysinfo capture is delayed;
     - status snapshot does not hold `e.mu` while taking telemetry snapshots;
     - run-end metrics finish/snapshot happens outside `e.mu`;
     - observer metrics path does not require `e.mu` under high-frequency calls.

4. Reload race and lock-scope cleanup.
   - Make `Config()` and `Runtime()` read under `e.mu.RLock()`.
   - Move `WorkLane.SetLimit()` out of the `e.mu` critical section where safe, or
     document and test the only allowed `e.mu -> WorkLane.mu` order.
   - Move `ensureDirectories()` outside the long reload lock using the new
     runtime. Keep state-mutating bootstrap/repair steps locked unless refactored
     with precise state snapshots.
   - Add `ReloadContext(ctx)` and call it from daemon SIGHUP/reload paths with
     the daemon context; keep `Reload()` as a compatibility wrapper where needed.
   - Use daemon context for reload cleanup queueing.
   - Tests:
     - concurrent reload with status/config/runtime reads passes under `-race`;
     - reload still updates worker limits and cleanup submission;
     - status remains available while safe reload filesystem work is delayed by a
       test seam.

5. Entity artifact publication lock narrowing.
   - Move telemetry observations out of `entityArtifactsMu`.
   - Keep artifact-generation bump under `entityArtifactsMu`.
   - Move integrity-cache stale marking after unlock and document the transient
     conservative invalidation window.
   - Avoid holding `entityArtifactsMu` while computing publish work totals or
     doing filesystem publish I/O where generation checks can still protect
     correctness.
   - Apply the rule to both background entity mutation publish and pipeline
     entity batch publish.
   - Tests:
     - stale-stage retry behavior remains correct under concurrent generation
       changes;
     - integrity caches become stale after successful publication;
     - `MarkIntegrityCachesStale()` and telemetry observation do not run while
       `entityArtifactsMu` is held;
     - pipeline publish path follows the same invariant.

6. Work-lane lifecycle and slot-hold visibility.
   - Make `AttachContext` idempotent.
   - Harden `syncStart` sends in both shutdown and scheduling so no channel send
     can block while `WorkLane.mu` is held.
   - Add lane slot-hold diagnostics/telemetry with a conservative threshold, so
     slow filesystem or callback work is visible without forcibly canceling
     valid long operations.
   - Classify production `context.Background()` uses and replace long-running or
     daemon-owned paths with caller/daemon contexts. Local synchronous helpers
     may keep background contexts only with evidence that they cannot outlive the
     caller or block shutdown.
   - Tests:
     - multiple `AttachContext` calls do not spawn competing shutdown behavior;
     - shutdown with queued blocking `Run` items cannot deadlock on `syncStart`;
     - entity refresh continuations use the lane/daemon context and cancel on
       shutdown;
     - slot-hold diagnostics are emitted for a deliberately blocked lane
       callback.

7. Run-exit and cache-save slot-hold review.
   - Review `cache.Save()` and final diagnostic logging inside the admitted run
     callback.
   - If moving cache save out of the lane would risk persisted-state loss, keep
     it in-lane but add explicit slot-hold diagnostics and cancellation-aware
     behavior around final diagnostics.
   - Tests:
     - cache persistence remains reliable on early run abort;
     - long cache save/final diagnostics are visible as slot-hold diagnostics and
       do not block admin light status.

8. Validation, reviewer loop, and repeated gap analysis.
   - Run focused tests after each slice.
   - Run `go test -race -count=1 ./pkg/systemd ./pkg/scheduler ./pkg/engine ./pkg/web`.
   - Run `make test-strict`.
   - Run broader gates required by changed files.
   - Run the same external reviewer set on the code diff.
   - Repeat deadlock/stall gap analysis on the new baseline and continue until
     no verified fixable liveness finding remains.

### External Plan Review Round 2 - 2026-06-25

Review scope:

- Plan V2 was reviewed by `glm`, `minimax`, `mimo`, `kimi`, `deepseek`, and
  `qwen` as requested. The `qwen` rerun progressed slowly after verifying the
  same code-evidence areas; the available reviewer set still converged on the
  material gaps below.

Consensus:

- Plan V2 is directionally correct but still not production-grade as a literal
  implementation contract.
- The top priorities remain watchdog notify deadline/diagnostics, status/admin
  liveness, reload lock/race cleanup, entity artifact lock narrowing, scheduler
  trigger admission, context propagation, and work-lane hardening.
- The telemetry AB-BA deadlock claim remains unproven in current code, because
  observer paths release telemetry locks before taking `e.mu`. The verified
  defect is wide-lock and lock-order amplification.

Accepted gaps to close before implementation:

1. Background task state updates must be included in the lock-narrowing plan.
   Evidence: `pkg/engine/background_tasks.go:82` to
   `pkg/engine/background_tasks.go:111` takes `e.mu.Lock()` on every
   `Update()` and `Finish()`. During entity refresh waves this can repeatedly
   block light status and reload readers.
2. `StatusSnapshotLight()` needs a precise short-lock contract. Evidence:
   `pkg/engine/status_snapshot.go:15` to `pkg/engine/status_snapshot.go:53`
   holds `e.mu.RLock()` while constructing the entire light status payload.
3. Run exit needs a concrete invariant. Evidence: `pkg/engine/run.go:84` to
   `pkg/engine/run.go:98` performs `cache.Save()`, diagnostic summary, and
   `markRunEnd()` inside the admitted lane callback; `pkg/engine/run.go:254` to
   `pkg/engine/run.go:279` updates final engine state and metrics under
   `e.mu`.
4. `WorkLane.SetLimit()` must move outside `e.mu`; documenting the existing
   lock order is not enough. Evidence: `pkg/engine/engine.go:253` and
   `pkg/engine/engine.go:263` establish `e.mu -> WorkLane.mu`, while status
   snapshots call `engineLane.Snapshot()` before taking `e.mu`.
5. The reader-method race audit must cover more than `Config()` and `Runtime()`.
   Known concurrent readers include `EntriesSnapshot()`,
   `EntriesSnapshotWithArtifacts()`, `ActiveFeedsSnapshot()`, and
   `MergeCompositions()`.
6. Runtime sampling must be explicit. The engine progress logger currently uses
   `captureEngineRuntimeStats()` directly, while web status uses
   `runtimeStatsSampler`. The implementation must choose a shared sampler or an
   engine-owned sampler and prove progress ticks do not call
   `runtime.ReadMemStats()` directly.
7. Pre-watchdog diagnostics must be non-blocking with a precise contract:
   goroutine stack sample first, no user locks for the initial evidence, then
   best-effort lane/scheduler snapshots under short timeout/try-lock behavior.
8. The scheduler trigger contract must name the methods and HTTP behavior:
   must-deliver queueing for startup/internal recovery, non-blocking queueing
   for HTTP/admin paths, and explicit `503 Service Unavailable` JSON responses
   for temporary queue saturation.
9. `ReloadContext(ctx)` must have a concrete API contract and daemon SIGHUP
   must use it. Legacy `Reload()` remains as `ReloadContext(context.Background())`
   for compatibility, but daemon/runtime paths must not use it.
10. Entity artifact lock narrowing must state the generation invariant for both
    optimistic entity mutation publish and pipeline entity batch publish.
11. All production `context.Background()` sites must be classified. Known sites
    requiring review include `pkg/engine/entity_artifacts.go:125`,
    `pkg/engine/public_series.go:78`, `pkg/engine/query.go:519`,
    `pkg/engine/latest_set_cache.go:61`, `pkg/engine/query_set_cache.go:33`,
    `pkg/engine/web_batch.go:87`, `pkg/engine/unique_share.go:31`,
    `pkg/engine/integrity_check.go:42`, `pkg/web/surface_handler.go:12`, and
    `pkg/web/integrity.go:196`.
12. Light status also depends on scheduler snapshots. Evidence:
    `pkg/web/admin_status_light.go:25` to
    `pkg/web/admin_status_light.go:26` calls `runner.ActivitySnapshot()` and
    `runner.Snapshot()`. The scheduler snapshot paths must be verified as
    short-lock/cached hot paths, or refactored if they can block on downloader
    or processing-loop work.

Rejected or constrained reviewer requests:

1. Do not add brittle tests that assert a private helper was not called by name.
   Use behavioral cost/shape tests, race tests, and same-package lock-invariant
   tests only where the liveness contract cannot be observed through public API.
2. Do not move `cache.Save()` out of the run lane unless tests prove no
   persisted progress can be lost on early abort or crash. If moving it is not
   safe, keep it in-lane and make slot hold observable.
3. Do not claim the runtime sampler alone proves the production root cause.
   It removes a verified stall source, but the root cause remains a working
   theory until diagnostics capture the next incident.

### Implementation Plan V3 - 2026-06-25

Plan V3 supersedes V2.

1. Watchdog and systemd notification safety.
   - Add deadline-bound notify support in `pkg/systemd`.
   - Use it for ready, stopping, status, and watchdog messages.
   - Deadline policy: if the watchdog interval is active, use the smaller of 2
     seconds and half the watchdog interval; otherwise use a 2-second lifecycle
     notify deadline.
   - Watchdog errors and slow notify calls emit rate-limited logs and counters.
   - Pre-watchdog diagnostics emit a capped goroutine stack sample first, without
     taking engine/scheduler locks. Best-effort engine-lane, scheduler, and
     runtime snapshots run after that under short timeout/try-lock behavior. The
     diagnostic sanitizer must redact request bodies, credentials, raw feed
     contents, raw stack argument values when possible, unbounded paths/lists,
     and identifying IP values.
   - Tests:
     - notify deadline on blocked/unresponsive unixgram socket;
     - payload compatibility for ready/stopping/status/watchdog;
     - watchdog notify errors are observable;
     - diagnostics are bounded, sanitized, rate-limited, and do not block on an
       intentionally blocked engine lock.

2. Scheduler action admission contracts.
   - Keep a context-bounded must-deliver queueing method for startup/internal
     recovery.
   - Add or reuse a non-blocking queueing method for HTTP/admin trigger paths.
   - HTTP/admin queue saturation returns `503 Service Unavailable` with a JSON
     body containing a stable error code and a short operator message. The admin
     UI must show this as temporary queue saturation, not as success.
   - Audit every `TriggerSources()` caller and classify it as must-deliver or
     request-bound before changing code.
   - Tests:
     - startup recovery action is delivered or returns visible error under a
       full queue;
     - HTTP/admin trigger returns promptly with `503` under full queue;
     - lane-internal integrity reprocess trigger cannot hold a lane slot
       indefinitely on full queue.

3. Engine reader and status liveness.
   - Add a dedicated `backgroundTasksMu` or equivalent copy-on-write state for
     background tasks. `BackgroundTaskHandle.Update()` and `Finish()` must not
     take `e.mu` on every progress update.
   - Make `Config()` and `Runtime()` race-free without adding hot-path
     `e.mu` contention. Preferred implementation is immutable values published
     through `atomic.Pointer`/`atomic.Value` while reload still updates canonical
     state under `e.mu`.
   - Make `EntriesSnapshot()`, `EntriesSnapshotWithArtifacts()`,
     `ActiveFeedsSnapshot()`, `MergeCompositions()`, and other public
     reader/snapshot methods either take the correct short lock or operate from
     immutable/copy snapshots.
   - Make `/api/v1/admin/status` default to light and preserve `?mode=full`.
     Update specs, docs, and UI polling assumptions in the same implementation
     slice.
   - Replace light feed summary construction with a direct summary path that
     does not build `adminFeed` rows, merge-composition rows, artifact rows, or
     per-feed UI payloads. It may iterate configured sources once and use cache
     and scheduler snapshots needed only for the summary counters.
   - Verify `runner.ActivitySnapshot()` and `runner.Snapshot()` are short-lock or
     cached enough for the light-status hot path. If not, add light-specific
     scheduler summaries that cannot wait behind downloader/processing-loop
     work.
   - `StatusSnapshotLight()` contract: take `e.mu.RLock()` only long enough to
     copy scalar fields, immutable pointers, and small map/slice snapshots that
     require `e.mu`; release it before any metrics, telemetry, runtime/sysinfo,
     lane, or larger derived snapshots.
   - Full `StatusSnapshot()` follows the same rule and snapshots current/lifetime
     metrics after releasing `e.mu`.
   - Observer metrics contract: use `atomic.Pointer[runMetrics]` for the current
     run metrics pointer. Set/detach it under `e.mu`; observer calls load it
     atomically and do not acquire `e.mu`. `markRunEnd()` detaches the pointer
     under `e.mu`, finalizes/snapshots the metrics outside `e.mu`, then stores
     `lastMetrics` under a short lock.
   - Runtime sampling contract: progress logging and status use cached samples,
     not direct `runtime.ReadMemStats()` or `/proc` reads on hot request/progress
     paths. Implement either a shared sampler or an engine-owned sampler; record
     the choice in code and tests.
   - Tests:
     - default status is light; `?mode=full` preserves full shape;
     - light feed summary equals full summary for representative fixtures;
     - light status remains responsive while metrics/sysinfo capture is blocked;
     - `StatusSnapshotLight()` and full `StatusSnapshot()` do not hold `e.mu`
       while taking telemetry snapshots;
     - high-frequency background task updates do not block light status on
       `e.mu`;
     - light status remains responsive while scheduler download/processing
       snapshot paths are busy;
     - concurrent reload/status/admin reader paths pass under `-race`;
     - progress logging uses cached runtime samples.

4. Run-end consistency and lane-slot visibility.
   - `markRunEnd()` must preserve this invariant: once status reports
     `Running=false`, the final report fields and `LastMetrics` are either both
     from the just-finished run or both still from the previous settled run. No
     status response may combine a finished run flag with a missing/half-final
     metrics pointer.
   - Split final state update into:
     - short lock to detach `currentMetrics` and mark the run no longer current
       only when enough final state is ready;
     - metrics finish/snapshot outside `e.mu`;
     - short lock to publish `lastMetrics` and final status fields consistently.
   - Review `cache.Save()` inside run exit. If moving it outside the lane risks
     persisted-progress loss, keep it in-lane but add slot-hold diagnostics and
     avoid direct runtime/sysinfo capture in final diagnostic summary.
   - Tests:
     - status consistency across run end;
     - run-end metrics snapshot outside `e.mu`;
     - long cache save/final diagnostics are visible as slot-hold diagnostics and
       do not block light status.

5. Reload context, race, and lock-scope cleanup.
   - Add `ReloadContext(ctx context.Context) error`.
   - `Reload()` remains a compatibility wrapper for `ReloadContext(context.Background())`.
   - Daemon SIGHUP and runtime callers use `ReloadContext(ctx)`.
   - Move `WorkLane.SetLimit()` outside `e.mu`. Do not keep the current
     `e.mu -> WorkLane.mu` ordering as a documented exception.
   - Move `ensureDirectories()` outside the long reload lock using the new
     runtime. Evaluate each remaining reload sub-step explicitly; keep
     state-mutating steps locked unless they are refactored with precise state
     snapshots.
   - Use the reload context for cleanup queueing.
   - Tests:
     - concurrent reload with status/config/runtime/entries readers under
       `-race`;
     - reload cancellation before cleanup submission;
     - status remains available while safe reload filesystem work is delayed by
       a test seam;
     - `SetLimit()` is not called while `e.mu` is held.

6. Entity artifact publication lock narrowing.
   - Preserve the generation invariant:
     - staging reads expected generation before publish;
     - publish rechecks expected generation under `entityArtifactsMu`;
     - generation bump happens under `entityArtifactsMu` after the live mutation
       is committed;
     - integrity-cache stale marking happens after unlock as conservative
       invalidation.
   - Apply the same invariant to background entity mutation publish and pipeline
     entity batch publish.
   - Move telemetry observation, publish work-total calculation when safe,
     filesystem publish I/O where safe, and integrity-cache stale marking out of
     the artifact lock.
   - Tests:
     - stale-stage retry under concurrent generation changes;
     - pipeline and background publish both preserve generation ordering;
     - `MarkIntegrityCachesStale()` and telemetry observation do not run while
       `entityArtifactsMu` is held;
     - concurrent integrity scan plus publish cannot hide stale artifacts.

7. Context and work-lane lifecycle hardening.
   - Make `AttachContext` idempotent.
   - Harden `syncStart` sends in both `Shutdown()` and `scheduleLocked()` so no
     channel send can block while `WorkLane.mu` is held.
   - Add lane slot-hold diagnostics with a conservative threshold. Long work is
     reported, not automatically canceled.
   - Classify all production `context.Background()` uses as daemon-owned,
     request-owned, CLI-owned, local short helper, or test-only. Replace
     daemon/request long-running paths with the correct context before closure.
   - Entity refresh continuations must submit with the current lane/daemon
     context, not `context.Background()`.
   - Tests:
     - multiple `AttachContext` calls produce one shutdown behavior;
     - `Run` waiting for admission returns the caller context error when
       canceled before a slot opens;
     - queued blocking `Run` items cannot deadlock during shutdown;
     - continuation submissions cancel on daemon shutdown;
     - slot-hold diagnostics emit for blocked callbacks.

8. Validation, reviewer loop, and repeated gap analysis.
   - Run focused tests after each slice.
   - Run `go test -race -count=1 ./pkg/systemd ./pkg/scheduler ./pkg/engine ./pkg/web`.
   - Run `make test-strict`.
   - Run broader gates required by changed files.
   - Run the same external reviewer set on the code diff.
   - Repeat gap analysis on the new baseline and continue until no verified,
     fixable liveness finding remains.

### External Plan Review Round 3 - 2026-06-25

Review scope:

- Plan V3 was sent to `glm`, `minimax`, `mimo`, `kimi`, `deepseek`, and `qwen`
  for independent review against the SOW and current code.
- Completed outputs available at the time of this SOW update:
  - `glm`: not production-grade plan.
  - `deepseek`: not production-grade plan.
  - `qwen`: production-grade plan.
  - `mimo`: production-grade plan with non-blocking observations.
  - `minimax`: production-grade plan with P2 polish observations.
  - `kimi`: not production-grade plan.

Accepted blocker findings:

1. Diagnostics must not depend only on notify failure or slow notify.
   - Reason: the production stall was silence. If the watchdog goroutine itself
     is not progressing, diagnostics triggered only inside the notify call may
     never run.
   - Required fix: record the last successful heartbeat/attempt in atomic state
     and add a separate watchdog self-health observer that emits bounded
     diagnostics when heartbeat age exceeds the expected interval threshold.

2. Pre-watchdog diagnostics need a concrete non-blocking contract.
   - Required fix: capture goroutine stacks first into a capped buffer without
     taking engine, scheduler, lane, or telemetry locks. Only after that initial
     evidence may diagnostics attempt best-effort snapshots with short timeout
     or try-lock behavior. Diagnostics must be rate-limited and sanitized.

3. Watchdog and diagnostic goroutines need panic containment.
   - Required fix: watchdog heartbeat and watchdog self-health observer
     goroutines recover panics, log them once with rate limiting, and do not
     silently terminate the only heartbeat path.

4. Entity artifact lock narrowing must not write live artifacts outside the
   publication lock unless staging and atomic promotion preserve correctness.
   - Required fix: if publish I/O is moved outside `entityArtifactsMu`, it must
     write to staging/temp paths first, then recheck generation and atomically
     promote under the publication lock. If that cannot be done safely in this
     SOW, live publish I/O stays under the publication lock and the fix is
     limited to moving telemetry/cache-stale work outside plus adding slot-hold
     diagnostics.

5. `syncStart` hardening must specify the mechanism.
   - Required fix: snapshot the affected `syncStart` channels while holding
     `WorkLane.mu`, release the mutex, then close/send notifications after
     unlock. Any fallback send must be non-blocking. No channel send may happen
     while `WorkLane.mu` is held.

6. `AttachContext` idempotency must specify the mechanism.
   - Required fix: guard shutdown-context attachment with `sync.Once` or an
     equivalent single-registration state. Repeated calls are no-ops and do not
     create competing shutdown goroutines.

7. Entity refresh continuation contexts must be explicit.
   - Required fix: continuations use the active lane/daemon context, not
     `context.Background()` and not `context.WithoutCancel()`. On daemon
     shutdown, continuation submission is canceled and recovery/next run owns
     repair. Continuations must not outlive daemon shutdown.

8. Progress runtime sampling must name the concrete call sites.
   - Required fix: `logRunProgress` and `logRunDiagnosticSummary` must not call
     `captureEngineRuntimeStats()` directly. They use cached runtime samples
     from the chosen sampler. Direct runtime/proc sampling is allowed only in
     the sampler goroutine or in explicit non-hot diagnostic code.

9. Reload lock-scope cleanup must be concrete, not aspirational.
   - Required fix: no broad filesystem I/O or lane submission may remain under
     `e.mu`. `SetLimit()` and `ensureDirectories()` move outside `e.mu`.
     Remaining reload steps are each audited and either proven in code comments
     and tests to be memory-only/short under lock, or split into scan/work
     outside the lock plus short state-apply under lock.

10. Background task state needs its own lock-order rule.
    - Required fix: `backgroundTasksMu` and `e.mu` are never held together.
      Snapshot assembly copies background task state independently and releases
      that lock before acquiring any engine/global lock, and vice versa.

11. The light/full status contract must make the expensive path explicit.
    - Required fix: high-frequency UI polling uses `?mode=light`. Defaulting
      `/api/v1/admin/status` to light is acceptable only with `?mode=full`
      preserving full payload compatibility and docs/spec updates. Any full
      status request remains a slow operator drill-down path, not a polling path.

12. Validation gates must include the full changed-surface gate.
    - Required fix: plan closure requires `make test-strict`, targeted race
      tests, `make test`, and any UI build/test/lint commands required by admin
      UI/status-contract changes.

13. Reload disk-scan helpers must be named explicitly.
    - Required fix: `bootstrapMissingEntriesFromDisk()`,
      `repairInvalidEntryTimestamps()`, and `bootstrapLegacyFailureStarts()`
      must not execute while `e.mu` is held unless implementation proves with a
      blocking test seam that the kept code cannot block status readers. The
      preferred fix is scan/work outside `e.mu`, then short state apply under
      `e.mu`.

14. Git sync must not run inside the entity artifact publication lock.
    - Required fix: `syncGeneratedFiles()` / `output.SyncGit()` must run after
      `entityArtifactsMu` is released in both background entity mutation publish
      and pipeline entity batch publish. It may still run inside the admitted
      engine-lane callback, and lane slot-hold diagnostics must make slow git
      sync visible.

Accepted P2 polish findings:

- Define the lane slot-hold warning threshold in code or config; do not leave it
  implicit.
- Define run-end status behavior during the brief metrics-finalization window.
- Keep telemetry book snapshot work outside global locks; further internal
  telemetry optimization is optional unless profiling proves it is a stall
  source after the global lock cleanup.

### Implementation Plan V4 - 2026-06-25

Plan V4 supersedes V3. It incorporates all completed Plan V3 reviewer findings,
including Kimi's reload and git-sync stall objections.

1. Watchdog, systemd notification, and independent self-health diagnostics.
   - Add deadline-bound notification support in `pkg/systemd` and use it for
     ready, stopping, status, and watchdog messages.
   - Deadline policy: if the watchdog interval is active, use the smaller of 2
     seconds and half the watchdog interval; otherwise use a 2-second lifecycle
     notify deadline.
   - Record watchdog heartbeat attempts, successful notifications, failures,
     slow calls, and last-success timestamps in atomic state.
   - Add a separate watchdog self-health observer. It wakes independently of the
     notify call and emits diagnostics when the last successful watchdog notify
     is older than the allowed threshold, for example greater than 1.5 times the
     watchdog interval or interval plus notify deadline. The exact threshold must
     be named in code and tests.
   - The watchdog heartbeat goroutine and self-health observer both recover
     panics and keep the failure visible through logs/counters.
   - Diagnostics run in two stages:
     1. capture a capped goroutine stack sample first, with no engine,
        scheduler, lane, telemetry, filesystem, or request locks;
     2. collect best-effort lane, scheduler, runtime, and compact status
        snapshots under short timeout/try-lock behavior.
   - Diagnostic output is bounded, sanitized, and rate-limited. It must not log
     request bodies, secrets, credentials, raw feed contents, unbounded path
     lists, or identifying IP values.
   - Tests:
     - bounded notify returns within deadline on an unresponsive socket;
     - ready/stopping/status/watchdog payloads stay compatible;
     - notify errors and slow calls update visible counters/log state;
     - self-health diagnostics fire when heartbeat age is stale even if the
       notify path does not return;
     - diagnostics emit a stack-first bounded sample and do not block on an
       intentionally blocked engine lock;
     - watchdog and self-health goroutine panic recovery is covered.

2. Scheduler action admission contracts.
   - Split scheduler action submission into must-deliver context-bounded
     queueing and request-bound non-blocking queueing.
   - Startup/internal recovery uses must-deliver queueing and returns a visible
     error if the action cannot be queued before context cancellation/deadline.
   - HTTP/admin trigger paths use non-blocking queueing. Queue saturation returns
     `503 Service Unavailable` with JSON containing a stable error code and a
     short operator-facing message. Handlers must not block on a full action
     channel.
   - Audit every `TriggerSources()` caller and record the classification in code
     or tests.
   - Tests:
     - startup recovery action is delivered or returns a visible error under a
       full queue;
     - HTTP/admin trigger returns promptly with `503` under a full queue;
     - lane-internal integrity reprocess trigger cannot hold a lane slot
       indefinitely on a full queue;
     - accepted triggers still wake downloader and processing loops.

3. Engine reader, background task, and status liveness.
   - Move background task state behind `backgroundTasksMu` or an equivalent
     copy-on-write structure. `BackgroundTaskHandle.Update()` and `Finish()` do
     not take `e.mu` on each progress update.
   - Lock ordering rule: `backgroundTasksMu` and `e.mu` are never held together.
     Snapshot code copies one state source, releases its lock, then reads the
     next source.
   - Make `Config()` and `Runtime()` race-free with immutable values published
     through `atomic.Pointer` / `atomic.Value`, or with short locks if atomic
     publication cannot fit local style. The preferred path is atomic immutable
     snapshots to avoid hot-path `e.mu` contention.
   - Make `EntriesSnapshot()`, `EntriesSnapshotWithArtifacts()`,
     `ActiveFeedsSnapshot()`, `MergeCompositions()`, and other public
     reader/snapshot methods either take the correct short lock or operate from
     immutable/copy snapshots.
   - Make high-frequency admin polling use `GET /api/v1/admin/status?mode=light`.
     If default `/api/v1/admin/status` changes to light, preserve the full
     payload behind `?mode=full` and update specs/docs/UI assumptions in the
     same change.
   - Replace light feed summary construction with a direct summary path that
     does not build `adminFeed` rows, merge-composition rows, artifact rows, or
     per-feed UI payloads.
   - Verify `runner.ActivitySnapshot()` and `runner.Snapshot()` are short-lock
     or cache-backed enough for light status. If not, add light-specific
     scheduler summaries.
   - Light status must not call `buildAdminFeedsWithStatusEntries()`.
   - Light status must not hold `e.mu.RLock()` while calling
     `runner.ActivitySnapshot()` or `runner.Snapshot()`.
   - `StatusSnapshotLight()` contract: take `e.mu.RLock()` only long enough to
     copy scalar fields, immutable pointers, and small map/slice snapshots that
     require `e.mu`; release it before metrics, telemetry, runtime/sysinfo, lane
     snapshots, scheduler snapshots, or larger derived work.
   - Full `StatusSnapshot()` follows the same rule and snapshots
     current/lifetime metrics after releasing `e.mu`.
   - Observer metrics contract: use `atomic.Pointer[runMetrics]` for the current
     run metrics pointer. Set/detach it under `e.mu`; observer calls load it
     atomically and do not acquire `e.mu`.
   - Runtime sampling contract: `logRunProgress`, `logRunDiagnosticSummary`,
     light status, and high-frequency status paths use cached samples, not direct
     `runtime.ReadMemStats()` or `/proc` reads. Direct
     `captureEngineRuntimeStats()` is limited to the sampler or explicit
     non-hot diagnostics.
   - Tests:
     - light/default status and `?mode=full` behavior match the documented
       compatibility contract;
     - light feed summary equals full summary for representative fixtures;
     - light status remains responsive while metrics/sysinfo capture is blocked;
     - `StatusSnapshotLight()` and full `StatusSnapshot()` do not hold `e.mu`
       while taking telemetry snapshots;
     - high-frequency background task updates do not block light status on
       `e.mu`;
     - light status remains responsive while scheduler snapshot paths are busy;
     - light status does not call `buildAdminFeedsWithStatusEntries()` or build
       merge/artifact/per-feed UI payloads;
     - light status does not hold `e.mu` while reading scheduler snapshots;
     - concurrent reload/status/admin reader paths pass under `-race`;
     - progress logging uses cached runtime samples at the named call sites.

4. Run-end consistency and lane-slot visibility.
   - Preserve this invariant: once status reports `Running=false`, final report
     fields and `LastMetrics` are either both from the just-finished run or both
     still from the previous settled run. No status response may combine a
     finished run flag with a missing or half-final metrics pointer.
   - Implement run-end state in short phases:
     - detach the current metrics pointer and enough immutable final state under
       a short lock;
     - finish/snapshot metrics outside `e.mu`;
     - publish `lastMetrics` and final status fields under a short lock.
   - Define the temporary finalization window explicitly: while metrics are being
     finalized, status either still reports the previous settled metrics or a
     typed `finalizing`/equivalent state; it must not report a misleading fresh
     settled state.
   - Keep `cache.Save()` inside the lane unless tests prove moving it cannot lose
     persisted run progress. If kept in-lane, lane slot-hold diagnostics must
     make long cache saves visible.
   - Tests:
     - status consistency across run end;
     - run-end metrics finish/snapshot happens outside `e.mu`;
     - long cache save/final diagnostics emit slot-hold diagnostics and do not
       block light status.

5. Reload context, race, and lock-scope cleanup.
   - Add `ReloadContext(ctx context.Context) error`.
   - `Reload()` remains a compatibility wrapper for
     `ReloadContext(context.Background())`; daemon SIGHUP and runtime paths use
     `ReloadContext(ctx)`.
   - Move `WorkLane.SetLimit()` outside `e.mu`.
   - Move `ensureDirectories()` outside the long reload lock using the new
     runtime.
   - Move or split the current disk-scan helpers
     `bootstrapMissingEntriesFromDisk()`, `repairInvalidEntryTimestamps()`, and
     `bootstrapLegacyFailureStarts()` so their filesystem scan/work phase does
     not execute while `e.mu` is held. If any part remains under `e.mu`, it must
     be memory-only/short state application and covered by a blocking test seam
     that proves status readers are not blocked by the scan/work phase.
   - Audit every remaining reload sub-step. Any broad filesystem I/O is split
     into scan/work outside `e.mu` plus short state-apply under `e.mu`. Any step
     kept under `e.mu` must be memory-only/short and covered by tests or a code
     comment explaining why it cannot block.
   - Lane `Submit` is never called while `e.mu` is held. Reload cleanup uses the
     reload/daemon context.
   - Tests:
     - concurrent reload with status/config/runtime/entries readers under
       `-race`;
     - reload cancellation before cleanup submission;
     - status remains available while safe reload filesystem work is delayed by
       a test seam;
     - reload disk-scan helpers do not block concurrent light status snapshots;
     - `SetLimit()` and lane `Submit` are not called while `e.mu` is held.

6. Entity artifact publication lock narrowing.
   - Preserve generation correctness for both background entity mutation publish
     and pipeline entity batch publish:
     - staging reads expected generation before publish;
     - live mutation rechecks expected generation under `entityArtifactsMu`;
     - generation bump happens under `entityArtifactsMu` after live mutation is
       committed;
     - integrity-cache stale marking happens after unlock as conservative
       invalidation.
   - Live artifact files must not be written outside `entityArtifactsMu` unless
     the implementation writes to temp/staging paths first, then rechecks
     generation and atomically promotes under the lock.
   - If staging/atomic promotion is not already available or cannot be added
     safely in this SOW, keep live publish I/O under the artifact lock and only
     move telemetry, work-total calculation when safe, and cache-stale marking
     outside the lock.
   - `syncGeneratedFiles()` / `output.SyncGit()` must run after
     `entityArtifactsMu` is released in both background entity mutation publish
     and pipeline entity batch publish. It must not run inside the artifact
     publication lock even if live file promotion remains locked.
   - Tests:
     - stale-stage retry under concurrent generation changes;
     - pipeline and background publish both preserve generation ordering;
     - `MarkIntegrityCachesStale()` and telemetry observation do not run while
       `entityArtifactsMu` is held;
     - `syncGeneratedFiles()` / git sync does not run while `entityArtifactsMu`
       is held;
     - any moved live publish I/O uses staging/atomic promotion and cannot hide
       stale artifacts.

7. Context and work-lane lifecycle hardening.
   - Make `AttachContext` idempotent with `sync.Once` or equivalent
     single-registration state.
   - Harden `syncStart` notifications in both `Shutdown()` and scheduling:
     snapshot channels under `WorkLane.mu`, release the lock, then notify after
     unlock. No channel send may happen while `WorkLane.mu` is held.
   - Add lane slot-hold diagnostics with a named conservative threshold. Long
     work is reported, not automatically canceled.
   - Classify all production `context.Background()` uses as daemon-owned,
     request-owned, CLI-owned, local short helper, or test-only. Replace
     daemon/request long-running paths with the correct context before closure.
   - Entity refresh continuations submit with the current lane/daemon context.
     On daemon shutdown, continuation submission is canceled and no new direct
     goroutine fallback is allowed.
   - Tests:
     - multiple `AttachContext` calls produce one shutdown behavior;
     - `Run` waiting for admission returns caller context error when canceled
       before a slot opens;
     - queued blocking `Run` items cannot deadlock during shutdown;
     - continuation submissions cancel on daemon shutdown;
     - slot-hold diagnostics emit for blocked callbacks.

8. Validation, reviewer loop, and repeated gap analysis.
   - Check the pending `kimi` Plan V3 review output before implementation.
   - Run focused tests after each slice.
   - Run targeted race tests for touched packages:
     `go test -race -count=1 ./pkg/systemd ./pkg/scheduler ./pkg/engine ./pkg/web`.
   - Run `make test-strict`.
   - Run `make test`.
   - Run `make race` unless the change set or environment makes it impossible;
     if not run, record why.
   - Run relevant UI build/test/lint commands if admin status/UI code changes.
   - Run the same external reviewer set on the code diff.
   - Repeat deadlock/stall gap analysis on the new baseline and continue until
     no verified, fixable liveness finding remains.

### External Plan Review Round 6 - 2026-06-25

Review scope:

- Plan V6 was sent to `glm`, `minimax`, `mimo`, `kimi`, `deepseek`, and `qwen`
  for read-only review against the SOW and current code.

Verdicts:

- `deepseek`: production-grade plan.
- `qwen`: production-grade plan.
- `mimo`: production-grade plan with non-blocking implementation checks.
- `minimax`: production-grade plan with non-blocking implementation checks.
- `kimi`: production-grade plan.
- `glm`: not production-grade plan. It identified a new lock-held I/O class on
  public-serving and lookup cache paths.

Accepted new gaps:

1. Public latest-set lookup cache holds a shared cache mutex during disk open,
   mmap, and text-parse fallback.
   - Evidence:
     - `pkg/engine/query_set_cache.go:45` locks `sharedLatestSetCache.mu`.
     - `pkg/engine/query_set_cache.go:52` calls `openLatestSet()` while the
       lock is held.
     - `pkg/engine/query.go:20` exposes `QueryIP()`.
     - `pkg/engine/query.go:93` calls `querySetCache.AcquireContext()`.
     - `pkg/web/search_api.go:30` calls `eng.QueryIP()` from public search.
     - `pkg/web/routes.go:54` and `pkg/web/routes.go:55` expose
       `/api/v1/query` and `/api/v1/search`.
   - Risk: one slow or stuck open/mmap/parse can serialize all public IP lookup
     requests behind a single mutex, matching the production symptom of idle
     CPU and silent stalled HTTP responses.
   - Existing pattern to reuse:
     - `pkg/engine/geo_provider_cache.go:80` checks the cache under lock.
     - `pkg/engine/geo_provider_cache.go:86` releases the lock before parsing.
     - `pkg/engine/geo_provider_cache.go:102` re-acquires the lock only to
       publish the prepared result.

2. ASN lookup cache holds its mutex during provider database open.
   - Evidence:
     - `pkg/engine/ip_context.go:98` locks `asnDatabaseCache.mu`.
     - `pkg/engine/ip_context.go:106` calls the supplied `open()` function
       while the lock is held.
     - `pkg/engine/ip_context.go:282` passes an opener that calls
       `asnloc.Open()`.
     - `pkg/web/search_api.go:58` and `pkg/web/search_api.go:97` call
       `eng.LookupIPContext()` for detailed search responses.
   - Risk: one slow ASN database open serializes detailed public lookup
     enrichment and any other ASN lookup using the same cache.

3. Per-run latest-set cache holds its mutex during disk open and uses
   `context.Background()`.
   - Evidence:
     - `pkg/engine/latest_set_cache.go:51` locks `latestSetCache.mu`.
     - `pkg/engine/latest_set_cache.go:61` calls `openLatestSet()` with
       `context.Background()` while the lock is held.
   - Risk: heavy phases serialize set opens behind one mutex and do not observe
     run cancellation during the open path.

4. Runtime ledger cache holds per-feed locks during ledger and retention disk
   reads.
   - Evidence:
     - `pkg/engine/runtime_ledger_cache.go:537` locks a feed ledger state
       before `loadHistoryLedgerState()`.
     - `pkg/engine/runtime_ledger_cache.go:560` locks the same feed ledger
       state before another possible `loadHistoryLedgerState()`.
     - `pkg/engine/runtime_ledger_cache.go:729` locks a feed ledger state
       before `loadRetentionCohorts()`.
   - Risk: public feed-detail/insight reads and processing updates can contend
     on per-feed locks while one side is doing slow filesystem work.

5. Reload swaps cache pointer fields while public readers access them without a
   lock or atomic publication.
   - Evidence:
     - `pkg/engine/engine.go:265` swaps `geoProviders`.
     - `pkg/engine/engine.go:269` mutates or replaces ASN lookup cache state.
     - `pkg/engine/engine.go:271` swaps `ledgerCache`.
     - `pkg/engine/ip_context.go:251` reads `geoProviders`.
     - `pkg/engine/ip_context.go:262` reads `asnLookupCache`.
     - `pkg/engine/runtime_ledger_cache.go:533` and following helpers read
       `ledgerCache`.
   - Risk: race detector can catch pointer races, and in-flight public work can
     race with reload cache replacement.

6. Runtime diagnostics still has one extra direct sampler call site not named
   in V6.
   - Evidence:
     - `pkg/engine/run_diagnostics.go:81` initializes diagnostic start stats
       with `captureEngineRuntimeStats()`.
     - `pkg/engine/run_diagnostics.go:123` and
       `pkg/engine/run_diagnostics.go:154` were already named in V6.
   - Risk: implementation could remove two direct calls and leave one
     runtime/proc sampling path outside the single sampler owner.

Rejected or downgraded review notes:

- `detailedStatusCached()` is already a cached-only light path. Evidence:
  `pkg/web/sysinfo.go:111` only reads cached state or returns a minimal
  fallback; live sampling happens in `detailedStatus()` and
  `refreshDetailedStatus()`.
- `clientRateLimiter` prune work is bounded and not a credible deadlock source
  for this SOW.
- Scheduler single-threaded action consumption is an amplifier when action
  work is slow, but V6/V7's non-blocking admission and light snapshot
  requirements address the immediate HTTP and lane-slot liveness risk.

### Implementation Plan V7 - 2026-06-25

Plan V7 supersedes V6 and is the active plan for implementation. It includes
all V5 and V6 requirements plus the mandatory deltas below. If plans conflict,
V7 wins.

1. Public latest-set lookup cache: no cache-wide lock during disk open/mmap.
   - Refactor `sharedLatestSetCache.AcquireContext()` so it:
     - checks existing usable entries under `sharedLatestSetCache.mu`;
     - releases the lock before `openLatestSet()` or any file/mmap/text-parse
       work;
     - re-acquires the lock only to publish or discard the newly opened source;
     - closes any duplicate source if another goroutine published the same key
       while the open was in progress;
     - honors caller context while waiting for a same-key in-flight load.
   - Use a per-key in-flight load marker or equivalent so concurrent misses for
     the same feed do not create an open/mmap storm, while unrelated feeds are
     not blocked by that load.
   - Preserve stale-entry/refcount correctness: invalidation can mark an entry
     stale while a load is in progress; old sources close only after their refs
     are released.
   - Tests:
     - a slow cache-miss open for feed A does not block a cache hit or miss for
       feed B;
     - concurrent cache misses for the same feed deduplicate or otherwise stay
       bounded and do not hold the cache-wide lock during open;
     - request cancellation while waiting for an in-flight same-key load returns
       promptly;
     - invalidation during an in-flight load does not publish stale data as a
       fresh usable entry.

2. ASN lookup cache: no cache mutex during provider open.
   - Refactor `asnDatabaseCache.acquire()` with the same check/release/open/
     publish shape as `geoProviderCache.LoadOrParse()`.
   - Use per-provider in-flight coordination or equivalent to avoid multiple
     heavy opens for the same provider while allowing unrelated provider opens
     or cache hits to proceed.
   - Preserve lease/refcount/retire semantics. A database replaced during reload
     must not be closed until all existing leases are released.
   - Tests:
     - a slow ASN provider open does not block cache hits for already-loaded
       providers;
     - same-provider concurrent cold opens are bounded/deduplicated;
     - reload retirement during an in-flight open preserves lease safety and
       does not leak or prematurely close databases.

3. Per-run latest-set cache: no cache mutex during open, and no naked
   `context.Background()` for run-owned opens.
   - Refactor `latestSetCache.Open` to an `OpenContext(ctx, name)` shape.
   - Existing `Open(name)` can remain as a compatibility wrapper for local
     non-run callers, but production heavy phases must use the run context.
   - `Summary()` and `OverlapFilter()` must call `OpenContext(ctx, name)`.
   - Open/mmap/text parse happens outside `latestSetCache.mu`.
   - Tests:
     - run cancellation interrupts or short-circuits cold latest-set opens;
     - concurrent heavy-phase opens for different feeds do not serialize on one
       cache mutex;
     - cached error/source publication remains race-free.

4. Runtime ledger cache: no per-feed lock during filesystem ledger loads.
   - Refactor `historyStatsFromRuntime`, `observeHistoryPoint`,
     `historyTailFromRuntime`, `changesetsFromRuntime`,
     `retentionCohortsFromRuntime`, and related helpers so filesystem reads run
     outside `feedLedgerState.mu`.
   - Apply under lock only if the cache state still needs the loaded data.
     Duplicate loads are acceptable only if bounded; prefer per-feed in-flight
     coordination for large retention cohort reads.
   - Tests:
     - public history/retention reads do not hold `feedLedgerState.mu` while a
       test seam blocks disk loading;
     - processing observe/update paths do not block on another goroutine's slow
       public ledger disk read after cached state is available;
     - duplicate in-flight loads do not corrupt cached history, tail,
       changesets, or cohort maps.

5. Reload-published cache pointers are race-free.
   - Extend V6's atomic/short-lock reader discipline beyond `cfg` and
     `runtime` to `geoProviders`, `asnLookupCache`, `ledgerCache`, and any other
     pointer field swapped during reload and read by public or background
     goroutines.
   - Either publish these pointers through `atomic.Pointer`/`atomic.Value`, or
     provide short-lock snapshot accessors and require all readers to use them.
   - Tests:
     - concurrent reload with public search, detailed search, feed history,
       retention, and status readers passes under `-race`;
     - old cache instances are not prematurely closed while active leases or
       requests still use them.

6. Runtime sampler grep-based closure.
   - Replace all direct production uses of `captureEngineRuntimeStats()` with
     the single cached sampler owner except inside the sampler implementation
     itself.
   - Required implementation check:
     - `rg -n 'captureEngineRuntimeStats' pkg/engine pkg/web` must show only
       sampler implementation/tests or explicitly documented non-hot-path
       compatibility wrappers.
   - Include `pkg/engine/run_diagnostics.go:81`,
     `pkg/engine/run_diagnostics.go:123`, and
     `pkg/engine/run_diagnostics.go:154` in the tests.

7. V7 validation additions.
   - Add focused liveness tests for public `/api/v1/search` and detailed search
     paths showing unrelated requests remain responsive while a cache miss is
     blocked by a test seam.
   - Add targeted race tests for:
     `go test -race -count=1 ./pkg/engine ./pkg/web`.
   - The external code-review loop after implementation must explicitly ask
     reviewers to re-check public-serving cache mutexes, cache pointer
     publication, and runtime ledger lock-held I/O in addition to the V6 engine
     lane/watchdog items.

### External Plan Review Round 5 - 2026-06-25

Review scope:

- Plan V5 was sent to `glm`, `minimax`, `mimo`, `kimi`, `deepseek`, and `qwen`
  for read-only review against the SOW and current code.
- The first `kimi` run returned only trace output and no verdict. It was rerun
  with the same scope and an explicit verdict requirement.

Verdicts:

- `deepseek`: production-grade plan, with implementation notes.
- `qwen`: production-grade plan.
- `mimo`: production-grade plan.
- `glm`: production-grade plan, with one test-contract precision gap.
- `minimax`: not production-grade plan. Several findings were about unimplemented
  code rather than plan quality, but it identified concrete precision gaps.
- `kimi`: not production-grade plan. The rerun identified concrete call-site
  and lock-order precision gaps.

Accepted Plan V5 gaps:

1. Entity publication tests must forbid every filesystem syscall under
   `entityArtifactsMu` except the atomic promotion operation and required
   promoted-file timestamp setting.
   - Evidence:
     - `pkg/engine/web_batch.go:202` walks staged files.
     - `pkg/engine/web_batch.go:239` stats staged files.
     - `pkg/engine/web_batch.go:306` stats destination files.
   - Risk: slow filesystem operations could remain under the global entity
     artifact lock while tests still pass.

2. `publishEntityArtifactMutationPlan()` must replace deferred unlock with an
   explicit unlock before post-publish side effects.
   - Evidence:
     - `pkg/engine/entity_artifact_publish.go:98` defers
       `entityArtifactsMu.Unlock()`.
     - `pkg/engine/entity_artifact_publish.go:127` calls
       `syncGeneratedFiles()`.
     - `pkg/engine/entity_artifact_publish.go:130` calls
       `MarkIntegrityCachesStale()`.
   - Risk: the stated V5 requirement to run git sync and cache stale marking
     after unlock is not mechanically achievable while the deferred unlock
     remains.

3. Entity writer lock telemetry call sites must be named and moved outside
   `entityArtifactsMu`.
   - Evidence:
     - `pkg/engine/entity_artifact_publish.go:96` observes
       `entity.writer_lock_wait`.
     - `pkg/engine/entity_artifact_publish.go:99` starts lock-hold timing.
     - `pkg/engine/entity_artifact_publish.go:100` defers
       `entity.writer_lock_hold`.
   - Risk: global artifact lock can still nest with telemetry/run metrics.

4. Pipeline entity batch publication call sites must be named explicitly.
   - Evidence:
     - `pkg/engine/run_pipeline.go:400` locks `entityArtifactsMu`.
     - `pkg/engine/run_pipeline.go:401` calls `publishWorkTotal()`.
     - `pkg/engine/run_pipeline.go:404` calls `publishContext()`.
   - Risk: directory walking and file publication can remain under
     `entityArtifactsMu` in the pipeline path even if the background path is
     fixed.

5. Direct runtime/proc sampler call sites must be named.
   - Evidence:
     - `pkg/engine/run_diagnostics.go:123` calls
       `captureEngineRuntimeStats()`.
     - `pkg/engine/run_diagnostics.go:154` calls
       `captureEngineRuntimeStats()`.
   - Risk: progress logging can still perform stop-the-world and `/proc` reads
     outside the single sampler owner.

6. Systemd lifecycle notify call sites must be named.
   - Evidence:
     - `pkg/web/server_run.go:217` calls `systemd.Stopping()`.
     - `pkg/web/server_run.go:256` calls `systemd.Ready()`.
   - Risk: ready/stopping notification can still block even if watchdog notify
     is bounded.

7. `e.mu` and `WorkLane.mu` must have an explicit no-nesting rule.
   - Evidence:
     - `pkg/engine/status_snapshot.go:10` calls `engineLane.Snapshot()`.
     - `pkg/engine/status_snapshot.go:14` then takes `e.mu.RLock()`.
     - `pkg/engine/engine.go:253` takes `e.mu.Lock()`.
     - `pkg/engine/engine.go:263` currently calls `engineLane.SetLimit()`.
   - Risk: moving `SetLimit()` fixes the current cycle, but the lock graph can
     regress unless status and reload avoid holding both locks together.

8. Heavy light-status call sites must be named.
   - Evidence:
     - `pkg/web/admin_status_light.go:27` calls
       `buildAdminFeedsWithStatusEntries()`.
     - `pkg/web/admin_status_light.go:25` calls `runner.ActivitySnapshot()`.
     - `pkg/web/admin_status_light.go:26` calls `runner.Snapshot()`.
   - Risk: default-light status can still do full feed-row or scheduler-heavy
     work and reproduce the admin UI stall.

9. Daemon-context replacement call sites must be named.
   - Evidence:
     - `pkg/engine/entity_refresh_queue.go:338` submits a continuation with
       `context.Background()`.
     - `pkg/engine/entity_refresh_queue.go:383` submits a continuation with
       `context.Background()`.
     - `pkg/engine/engine.go:303` queues reload cleanup with
       `context.Background()`.
   - Risk: background continuations can outlive daemon shutdown or fail to exit
     during restart.

10. HTTP/admin scheduler trigger call sites must be named.
    - Evidence:
      - `pkg/web/admin.go:344`, `pkg/web/admin.go:360`, and
        `pkg/web/admin.go:432` call `TriggerSources()`.
      - `pkg/web/routes.go:345` calls `TriggerSources()`.
      - `pkg/web/integrity.go:461` and `pkg/web/integrity.go:469` call
        `TriggerSources()`.
    - Risk: request handlers or lane-internal integrity callbacks can still
      block on a full scheduler action channel.

11. Git timeout recovery semantics must be explicit.
    - Risk: after a timed-out git sync, the implementation could leave the
      operator unable to distinguish published artifacts from failed git mirror
      promotion.

12. Entity refresh continuation submission failure must not leave queue state
    marked as running forever.
    - Risk: if continuation admission fails during shutdown, the queue can look
      active without a lane item.

13. Slot-hold diagnostics need visible signal requirements, not only a
    threshold.
    - Risk: long lane holds become visible only in logs that operators may miss.

14. `cache.Save()` remains an accepted lane-held operation, but diagnostics
    must prove long saves are visible and bounded by daemon cancellation where
    possible.
    - Evidence:
      - `pkg/engine/run.go:84` calls `cache.Save()` in the run-exit defer.
    - Risk: slow or stuck storage can hold the engine lane even after lock
      issues are fixed. This SOW treats it as visible lane work unless a safe
      persistence redesign is proven.

Rejected or downgraded reviewer findings:

- "Plan V5 has not implemented the code yet" is not a plan defect. Code review
  happens after implementation.
- V5 already required status builders and progress logging to use a single
  cached runtime sampler; V6 keeps that requirement and adds exact call sites.
- V5 already required daemon context audit for all `context.Background()` uses;
  V6 keeps the audit and adds exact critical call sites.
- V5 already required `cache.Save()` lane-slot diagnostics. V6 does not move
  cache persistence out of the lane because that could lose durable run state
  without a separate persistence design.

### Implementation Plan V6 - 2026-06-25

Plan V6 supersedes V5. Implementation must satisfy every V5 requirement plus
the mandatory deltas below. If V5 and V6 conflict, V6 wins.

1. Entity artifact publication lock narrowing, exact call sites.
   - In `pkg/engine/entity_artifact_publish.go`, replace the deferred
     `entityArtifactsMu.Unlock()` in `publishEntityArtifactMutationPlan()` with
     an explicit unlock before `syncGeneratedFiles()` and
     `MarkIntegrityCachesStale()`.
   - Move `entity.writer_lock_wait` and `entity.writer_lock_hold`
     observations outside `entityArtifactsMu`. If measuring hold time requires
     the lock boundary, capture timestamps under the lock boundary and emit
     telemetry after unlock.
   - In `pkg/engine/run_pipeline.go`, remove `publishWorkTotal()` and
     `publishContext()` from the `entityArtifactsMu` critical section. The
     lock may cover only generation recheck, atomic live promotion, required
     promoted-file timestamp setting, and generation mutation.
   - Atomic promotion staging must live on the same filesystem as the live
     published tree. If an atomic rename returns a cross-device error, fail
     visibly; do not silently fall back to non-atomic copy inside the lock.
   - Tests must prove no filesystem syscall executes while
     `entityArtifactsMu` is held except atomic promotion and required
     promoted-file timestamp setting. Forbidden under-lock work includes
     directory walking, content comparison, `os.Stat`, `os.MkdirAll`,
     `os.RemoveAll`, `os.Chmod`, `chownPath`, `os.Chtimes`,
     `syncGeneratedFiles()`, telemetry observation, and
     `MarkIntegrityCachesStale()`.

2. Git sync timeout and recovery semantics.
   - Add a first-class runtime config field `push_to_git_timeout` with a
     documented default of 10 minutes and validation rejecting negative values.
     This is no longer optional; avoid the V5 fallback to a hidden constant
     unless config evolution is technically impossible and recorded in this
     SOW before implementation continues.
   - `SyncGitContext()` timeout/cancellation returns a visible operation error
     and releases the engine-lane slot. Published artifacts are not rolled
     back, because publication happened before git sync. The dirty git tree is
     safe and must be retried on the next run.
   - All git subprocesses use context-bound execution: `git add`,
     `git diff --cached`, `git commit`, `git push`, and `git gc --auto`.
   - Tests must prove a hung git subprocess is canceled, the lane slot is
     released, the failure is visible, and a later run can retry the dirty git
     state.

3. Runtime sampler exact call sites.
   - `pkg/engine/run_diagnostics.go:123` and
     `pkg/engine/run_diagnostics.go:154` must stop calling
     `captureEngineRuntimeStats()` directly.
   - Progress summary code uses the single cached runtime sampler owner.
   - Tests must prove `logRunProgress()` and `logRunDiagnosticSummary()` use
     cached samples and remain responsive when direct runtime/proc sampling is
     blocked or made slow by a test seam.

4. Systemd notification exact call sites and diagnostics.
   - `pkg/web/server_run.go:217` (`systemd.Stopping`) and
     `pkg/web/server_run.go:256` (`systemd.Ready`) must use the same
     context/deadline-bound notify path as watchdog heartbeat and status
     messages.
   - The watchdog panic-recovery test must inject a notifier or diagnostic
     callback that panics, prove recovery is logged/counted, and prove a later
     tick still runs. The test must use a cancellable context and assert no
     goroutine leak through owner shutdown.

5. Engine lock-order and status snapshot contract.
   - `e.mu` and `WorkLane.mu` must never be held together. If implementation
     finds an unavoidable case, stop and update the SOW before coding that
     exception.
   - Refactor `StatusSnapshotLight()` and `StatusSnapshot()` so lane snapshots
     are collected without nesting `WorkLane.mu` with `e.mu`.
   - Reload must capture the engine lane pointer and desired worker limit under
     a short lock if needed, release `e.mu`, then call `SetLimit()`.
   - Tests must prove reload cannot hold `e.mu` while calling
     `WorkLane.SetLimit()` or `WorkLane.Submit()`, and status snapshots cannot
     hold `e.mu` while calling `WorkLane.Snapshot()`.

6. Light admin status exact call sites.
   - Replace `pkg/web/admin_status_light.go:27` with the new
     summary-only builder.
   - Default `GET /api/v1/admin/status` returns the light payload.
     `?mode=full` returns the existing full payload.
   - UI callers must use the default light status unless they explicitly need
     full rows. Operator docs/specs must describe the compatibility change.
   - Tests must prove the frontend/admin status flow works with the default
     light payload, and full mode remains available for screens that need full
     feed rows.
   - Add a bounded-time behavioral test proving light status completes while
     scheduler snapshot paths are blocked or slow through a test seam. If the
     existing scheduler snapshots are too heavy, add light-specific cached
     scheduler summaries.

7. Scheduler trigger exact call sites.
   - HTTP/admin request handlers at `pkg/web/admin.go:344`,
     `pkg/web/admin.go:360`, `pkg/web/admin.go:432`, and
     `pkg/web/routes.go:345` must use non-blocking trigger admission and return
     `503` JSON with a stable error code on saturation.
   - Integrity paths at `pkg/web/integrity.go:461` and
     `pkg/web/integrity.go:469` must not block a lane slot on scheduler action
     admission. If admission fails, record a visible operation failure.
   - Startup and internal must-deliver paths use `TriggerSourcesContext()` or
     equivalent context-bound admission.
   - Lane-internal callers must use non-blocking/visible-failure admission, not
     an unbounded must-deliver wait.

8. Daemon context and continuation state.
   - Replace `context.Background()` at
     `pkg/engine/entity_refresh_queue.go:338`,
     `pkg/engine/entity_refresh_queue.go:383`, and `pkg/engine/engine.go:303`
     with the daemon/reload context described in V5.
   - If an entity refresh continuation cannot be admitted because daemon
     shutdown has started, clear the running reservation state so the queue
     cannot remain marked running without a lane item. Preserve enough pending
     state for restart/recovery to repair on the next daemon start.
   - Tests must prove failed continuation submit on shutdown cannot leave
     `entityRefreshRunning` or the health-refresh equivalent permanently true.

9. Reload split exact scope.
   - In addition to the V5 collect/apply helpers, classify these reload/startup
     helpers as memory-only under `e.mu` or split them:
     `reconcileEntriesFromSourceConfig`, `registerSyntheticInternalSources`,
     `buildRetentionMaxWindow`, and
     `refreshCriticalInfrastructureProviderSetID`.
   - Helpers that read files, stat artifacts, hash sets, or call into lane or
     telemetry code must run outside `e.mu` and apply short memory-only changes
     under lock.

10. Slot-hold diagnostics contract.
    - Add an injectable/default lane slot-hold warning threshold. Production
      default is 30 seconds.
    - A slot-hold warning must emit a structured operator log with work kind,
      component, operation/stage when available, elapsed milliseconds, and
      threshold milliseconds.
    - It must increment a bounded telemetry counter and be visible in the lane
      or admin status snapshot as recent slot-hold warning state.
    - Tests must set a short threshold and prove a blocked callback emits the
      log/counter/status signal without canceling the work automatically.

11. `cache.Save()` visibility.
    - `cache.Save()` may remain in the engine lane for this SOW to preserve
      durable run-state ordering.
    - Long `cache.Save()` must trigger slot-hold diagnostics and must not block
      light status.
    - If implementation finds a safe context-bound save path without weakening
      persistence, it may add it; otherwise record the residual I/O risk in the
      outcome and repeat gap analysis after implementation.

### External Plan Review Round 4 - 2026-06-25

Review scope:

- Plan V4 was sent to `glm`, `minimax`, `mimo`, `kimi`, `deepseek`, and `qwen`
  for read-only review against the SOW and current code.

Verdicts:

- `deepseek`: production-grade plan.
- `qwen`: production-grade plan with implementation notes.
- `minimax`: not production-grade plan.
- `kimi`: production-grade plan.
- `glm`: not production-grade plan.
- `mimo`: not production-grade plan, but close.

Accepted new blockers and precision gaps:

1. Git sync must be bounded and cancelable, not only moved outside
   `entityArtifactsMu`.
   - Evidence:
     - `pkg/output/sync.go:165` `SyncGit` takes no context.
     - `pkg/output/sync.go:222` `runGitAutoGC` uses `exec.Command`.
     - `pkg/output/sync.go:231` `runGit` uses `exec.Command`.
     - `pkg/output/sync.go:238` waits in `CombinedOutput()`.
     - `pkg/output/sync.go:243` `gitClean` uses `exec.Command`.
   - Risk: a hung `git push`, `git commit`, `git diff`, or `git gc --auto`
     can hold the only engine-lane slot indefinitely even after it is moved out
     of the artifact publication lock.
   - Required fix: `SyncGit` and internal git helpers accept context/deadline,
     use cancelable `exec.CommandContext` or equivalent, kill child processes on
     expiry, and return a typed timeout/cancellation error. Tests must prove a
     hung git command releases the engine lane within the configured deadline.

2. Entity artifact publication must narrow the lock to atomic promotion only.
   - Evidence:
     - `pkg/engine/entity_artifact_publish.go:95` holds `entityArtifactsMu`.
     - `pkg/engine/entity_artifact_publish.go:116` publishes staged entity
       files while locked.
     - `pkg/engine/entity_artifact_publish.go:122` publishes staged web files
       while locked.
     - `pkg/engine/run_pipeline.go:400` holds `entityArtifactsMu` for pipeline
       entity publication.
     - `pkg/engine/web_batch.go` publish helpers can walk directories and
       compare file contents.
   - Required fix: prepare/walk/compare outside `entityArtifactsMu`; under the
     lock, recheck generation and perform only atomic live promotions, timestamp
     setting needed for promoted files, and generation state mutation. Git sync
     and cache stale marking run after unlock.

3. Default admin status must be a decision, not conditional.
   - Required fix: `GET /api/v1/admin/status` defaults to light. The full
     payload remains available through `?mode=full`. Specs, docs, UI usage, and
     release/operator notes must describe the compatibility change.

4. Deadline-bound systemd notify needs a concrete socket mechanism.
   - Required fix: implement a context/deadline-capable notify path using
     `net.Dialer.DialContext` for `unixgram` or an equivalent goroutine/select
     wrapper around `net.DialUnix`, and set a write deadline before writing.
     Tests must exercise a real blocked Unix datagram socket, not only a mock.

5. Watchdog self-health observer needs a concrete wake policy.
   - Required fix: a separate goroutine uses a `time.Ticker` with interval
     `max(1s, min(watchdogInterval/4, 15s))` or an equivalent documented formula
     and emits diagnostics when last successful watchdog notification age
     exceeds `max(watchdogInterval+notifyDeadline, watchdogInterval*3/2)`.

6. Scheduler trigger semantics must define failure behavior.
   - Required fix: startup/internal recovery uses `TriggerSourcesContext(ctx)`
     or equivalent must-deliver bounded queueing. HTTP/admin uses
     `TryTriggerSources` or equivalent non-blocking queueing. Lane-internal
     reprocess trigger queue failure returns a visible operation failure and
     does not silently drop work or fall back to direct execution.

7. Reload disk-scan split needs function-level shape.
   - Required fix: split reload helpers into collect/apply forms, for example
     `collectMissingEntriesFromDisk(...)` plus `applyMissingEntriesLocked(...)`,
     `collectInvalidEntryTimestampRepairs(...)` plus
     `applyInvalidEntryTimestampRepairsLocked(...)`, and
     `collectLegacyFailureStarts(...)` plus
     `applyLegacyFailureStartsLocked(...)`, or equivalent names. Collection runs
     outside `e.mu`; apply is short and memory-only under `e.mu`.

8. Light feed summary replacement needs a concrete function contract.
   - Required fix: add a summary-only builder such as
     `buildAdminFeedsSummaryLight(eng, runner, activity, snapshot)` returning
     `adminFeedsSummary` only. It must not call
     `buildAdminFeedsWithStatusEntries`, `MergeCompositions`, artifact builders,
     or per-feed row construction.

9. Background and active operation progress state both need lock separation.
   - Required fix: move background task progress and active operation progress
     out of `e.mu` hot update paths. Snapshot functions must not hold both
     `backgroundTasksMu`/active-operation state lock and `e.mu`.

10. Continuation context source must be explicit.
    - Required fix: `Engine` stores the daemon context, attached from `web.Run`
      through the same lifecycle path as the work lane. Entity refresh
      continuations and reload cleanup use that daemon context unless a narrower
      caller context is explicitly available and guaranteed to remain valid for
      the continuation.

11. Runtime sampler plumbing must be explicit.
    - Required fix: either the engine owns a runtime sampler shared by progress
      logging, or web exposes a sampler object to engine initialization. The
      chosen path must have one owner, one lifecycle, and tests proving
      `logRunProgress` and `logRunDiagnosticSummary` do not call
      `captureEngineRuntimeStats()` directly.

12. Additional liveness tests are required.
    - `markRunEnd()` exclusive `e.mu` section is memory-only and contains no
      telemetry, runtime sampling, filesystem, git, or output calls.
    - bounded notify deadline is proven against a real blocked Unix datagram
      socket or an equivalent syscall-level test, not only a mock.
    - concurrent reload, status snapshot, and lane admission complete within a
      bounded time.
    - status snapshot does not hold `e.mu` and background/active-operation locks
      together.
    - an active lane callback blocked on scheduler trigger admission is released
      or fails visibly during shutdown.

Rejected or downgraded reviewer findings:

- Several reviewers asked for tests already present in Plan V4: watchdog panic
  recovery, self-health diagnostics, `syncStart` shutdown, `AttachContext`
  idempotency, slot-hold diagnostics, scheduler trigger saturation, and light
  status shape. These remain required but are not new blockers.
- `tryMarkRunStart` is not a current indefinite wait path; the existing
  defensive slot-release coverage remains enough unless implementation changes
  that code.

### Implementation Plan V5 - 2026-06-25

Plan V5 supersedes V4.

1. Watchdog, systemd notification, and independent self-health diagnostics.
   - Add context/deadline-bound notification support in `pkg/systemd`.
   - Use the bounded path for ready, stopping, status, and watchdog messages.
   - Implementation mechanism:
     - `NotifyContext(ctx context.Context, msg string) error` or equivalent uses
       `net.Dialer.DialContext(ctx, "unixgram", notifySocket)` where supported,
       or a goroutine/select wrapper around `net.DialUnix` if local behavior
       requires it.
     - After dialing, set a write deadline from the context deadline before
       writing.
     - Close the connection on all paths.
   - Deadline policy:
     - watchdog calls use `min(2s, watchdogInterval/2)`;
     - lifecycle calls use 2 seconds;
     - tests may inject shorter deadlines.
   - Record watchdog heartbeat attempts, successful notifications, failures,
     slow calls, and last-success timestamps in atomic state.
   - Add a separate watchdog self-health observer goroutine with ticker interval
     `max(1s, min(watchdogInterval/4, 15s))`. It emits diagnostics when last
     successful watchdog notification age exceeds
     `max(watchdogInterval+notifyDeadline, watchdogInterval*3/2)`.
   - The watchdog heartbeat goroutine and self-health observer both recover
     panics and keep the failure visible through logs/counters.
   - Diagnostics run in two stages:
     1. capture a capped goroutine stack sample first, with no engine,
        scheduler, lane, telemetry, filesystem, or request locks;
     2. collect best-effort lane, scheduler, runtime, and compact status
        snapshots under short timeout/try-lock behavior.
   - Diagnostic output is bounded, sanitized, and rate-limited to at most one
     diagnostic per reason per minute unless the process is shutting down. It
     must not log request bodies, secrets, credentials, raw feed contents,
     unbounded path lists, or identifying IP values.
   - Tests:
     - bounded notify returns within deadline on a real blocked Unix datagram
       socket or equivalent syscall-level test;
     - ready/stopping/status/watchdog payloads stay compatible;
     - notify errors and slow calls update visible counters/log state;
     - self-health diagnostics fire when heartbeat age is stale even if the
       notify path does not return;
     - diagnostics emit a stack-first bounded sample and do not block on an
       intentionally blocked engine lock;
     - watchdog and self-health goroutine panic recovery is covered.

2. Scheduler action admission contracts.
   - Replace bare scheduler trigger sends with two explicit APIs:
     - `TriggerSourcesContext(ctx, reason)` or equivalent for startup/internal
       must-deliver recovery;
     - `TryTriggerSources(reason)` or equivalent for HTTP/admin non-blocking
       queueing.
   - Startup/internal recovery uses context-bounded queueing and returns a
     visible error if the action cannot be queued before context
     cancellation/deadline.
   - HTTP/admin trigger paths use non-blocking queueing. Queue saturation returns
     `503 Service Unavailable` with JSON containing a stable error code and a
     short operator-facing message.
   - Lane-internal reprocess trigger queue failure records the operation as
     failed and returns a visible error. It must not silently drop work and must
     not fall back to direct execution.
   - Tests:
     - startup recovery action is delivered or returns a visible error under a
       full queue;
     - HTTP/admin trigger returns promptly with `503` under a full queue;
     - lane-internal integrity reprocess trigger cannot hold a lane slot
       indefinitely on a full queue;
     - active lane callback blocked on trigger admission exits or fails visibly
       during shutdown;
     - accepted triggers still wake downloader and processing loops.

3. Engine reader, background/active operation, and status liveness.
   - Move background task state behind `backgroundTasksMu` or an equivalent
     copy-on-write structure. `BackgroundTaskHandle.Update()` and `Finish()` do
     not take `e.mu` on each progress update.
   - Move active operation progress state behind a separate lock or
     copy-on-write/atomic state so active-operation updates do not take `e.mu`
     on each progress update.
   - Lock ordering rule: background task state locks, active-operation state
     locks, and `e.mu` are never held together. Snapshot code copies one state
     source, releases its lock, then reads the next source.
   - Rename/refactor helpers accordingly: `snapshotBackgroundTasksLocked()` must
     either disappear or only require the background-task lock, not `e.mu`.
     Active-operation snapshot helpers follow the same pattern.
   - Make `Config()` and `Runtime()` race-free with immutable values published
     through `atomic.Pointer` / `atomic.Value`.
   - Make `EntriesSnapshot()`, `EntriesSnapshotWithArtifacts()`,
     `ActiveFeedsSnapshot()`, `MergeCompositions()`, and every other public
     reader/snapshot method race-free through short locks or immutable/copy
     snapshots.
   - `GET /api/v1/admin/status` defaults to light. The full payload remains
     available with `?mode=full`. Docs/specs/UI/release notes must explain the
     compatibility change.
   - Add a summary-only light builder, for example
     `buildAdminFeedsSummaryLight(eng, runner, activity, snapshot)
     adminFeedsSummary`. It must not call `buildAdminFeedsWithStatusEntries`,
     `MergeCompositions`, artifact builders, or per-feed row builders.
   - Light status must not hold `e.mu.RLock()` while calling
     `runner.ActivitySnapshot()` or `runner.Snapshot()`. If scheduler snapshots
     are not short-lock/cached enough, add light-specific scheduler summaries.
   - `StatusSnapshotLight()` takes `e.mu.RLock()` only long enough to copy scalar
     fields, immutable pointers, and small map/slice snapshots that require
     `e.mu`; it releases before metrics, telemetry, runtime/sysinfo, lane,
     scheduler, background-task, active-operation, or larger derived snapshots.
   - Full `StatusSnapshot()` follows the same rule.
   - Observer metrics use `atomic.Pointer[runMetrics]` for the current run
     metrics pointer. Observer calls load it atomically and do not acquire
     `e.mu`.
   - Runtime sampling owner: the engine owns a runtime sampler used by progress
     logging and exposed to web status builders, or web owns a sampler passed to
     engine status/progress code. The chosen owner must be single-lifecycle and
     explicitly tested. `logRunProgress` and `logRunDiagnosticSummary` must not
     call `captureEngineRuntimeStats()` directly.
   - Tests:
     - default status is light and `?mode=full` preserves full shape;
     - light feed summary equals full summary for representative fixtures
       covering normal, disabled, archived, errored, empty, merge, and no-data
       feeds;
     - light status does not call `buildAdminFeedsWithStatusEntries()` or build
       merge/artifact/per-feed UI payloads;
     - light status remains responsive while metrics/sysinfo capture is blocked;
     - light status remains responsive while scheduler snapshot paths are busy;
     - `StatusSnapshotLight()` and full `StatusSnapshot()` do not hold `e.mu`
       while taking telemetry snapshots;
     - status snapshot does not hold `e.mu` together with background-task or
       active-operation locks;
     - high-frequency background task and active-operation updates do not block
       light status on `e.mu`;
     - concurrent reload/status/admin reader paths pass under `-race`;
     - progress logging uses cached runtime samples at the named call sites.

4. Run-end consistency and lane-slot visibility.
   - Preserve this invariant: once status reports `Running=false`, final report
     fields and `LastMetrics` are either both from the just-finished run or both
     still from the previous settled run. No status response may combine a
     finished run flag with a missing or half-final metrics pointer.
   - Split `markRunEnd()` into short phases:
     - detach the current metrics pointer and enough immutable final state under
       a short `e.mu` lock;
     - finish/snapshot metrics outside `e.mu`;
     - publish `lastMetrics` and final status fields under a short `e.mu` lock.
   - The exclusive `e.mu` sections in `markRunEnd()` are memory-only. They must
     contain no telemetry book snapshots, observability emission, runtime
     sampling, filesystem, git, output, cache save, or other blocking I/O.
   - Define the temporary finalization window explicitly: while metrics are being
     finalized, status either reports previous settled metrics or a typed
     finalizing/equivalent state; it must not report a misleading fresh settled
     state.
   - Keep `cache.Save()` inside the lane unless tests prove moving it cannot
     lose persisted run progress. If kept in-lane, lane slot-hold diagnostics
     must make long cache saves visible.
   - Tests:
     - status consistency across run end;
     - `markRunEnd()` exclusive `e.mu` sections are memory-only;
     - run-end metrics finish/snapshot happens outside `e.mu`;
     - long cache save/final diagnostics emit slot-hold diagnostics and do not
       block light status.

5. Reload context, race, and lock-scope cleanup.
   - Add `ReloadContext(ctx context.Context) error`.
   - `Reload()` remains a compatibility wrapper for
     `ReloadContext(context.Background())`; daemon SIGHUP and runtime paths use
     `ReloadContext(ctx)`.
   - Move `WorkLane.SetLimit()` outside `e.mu`.
   - Move `ensureDirectories()` outside the long reload lock using the new
     runtime.
   - Split reload disk helpers into collect/apply forms:
     - `collectMissingEntriesFromDisk(...)` plus
       `applyMissingEntriesLocked(...)` or equivalent;
     - `collectInvalidEntryTimestampRepairs(...)` plus
       `applyInvalidEntryTimestampRepairsLocked(...)` or equivalent;
     - `collectLegacyFailureStarts(...)` plus
       `applyLegacyFailureStartsLocked(...)` or equivalent.
   - Collection runs outside `e.mu` using immutable snapshots. Apply steps are
     short, memory-only state mutations under `e.mu`.
   - Lane `Submit` is never called while `e.mu` is held. Reload cleanup uses the
     reload/daemon context.
   - Tests:
     - concurrent reload with status/config/runtime/entries readers under
       `-race`;
     - concurrent reload, status snapshot, and lane admission complete within a
       bounded time;
     - reload cancellation before cleanup submission;
     - reload disk-scan helpers do not block concurrent light status snapshots;
     - `SetLimit()` and lane `Submit` are not called while `e.mu` is held.

6. Entity artifact publication and git sync bounding.
   - Preserve generation correctness for both background entity mutation publish
     and pipeline entity batch publish:
     - staging reads expected generation before publish;
     - live mutation rechecks expected generation under `entityArtifactsMu`;
     - generation bump happens under `entityArtifactsMu` after live mutation is
       committed;
     - integrity-cache stale marking happens after unlock as conservative
       invalidation.
   - Narrow `entityArtifactsMu` to atomic promotion only. Prepare, directory
     walking, file comparison, and publish planning run outside the lock. Under
     the lock, recheck generation and perform only atomic live promotions,
     required timestamp setting for promoted files, and generation state
     mutation.
   - `syncGeneratedFiles()` / `output.SyncGit()` runs after `entityArtifactsMu`
     is released in both background entity mutation publish and pipeline entity
     batch publish.
   - Add context/deadline support to git sync:
     - introduce `SyncGitContext(ctx context.Context, opts SyncOptions, files
       []GeneratedFile) error` or equivalent and migrate engine callers;
     - internal git helpers use `exec.CommandContext` or equivalent cancellation;
     - timeout/cancellation kills the child process and returns a typed or
       inspectable timeout/cancellation error;
     - `git add`, `git diff --cached`, `git commit`, `git push`, and
       `git gc --auto` are all bounded.
   - Timeout policy:
     - use a runtime config value such as `runtime.push_to_git_timeout` with a
       documented default of 10 minutes and validation rejecting negative values;
     - if adding a config field is rejected during implementation, use a named
       constant with the same default and document it. Zero/omitted means the
       default, not unbounded.
   - Slow or timed-out git sync is visible through lane slot-hold diagnostics and
     operation failure state. It must release the engine-lane slot when the
     timeout/cancellation path returns.
   - Tests:
     - stale-stage retry under concurrent generation changes;
     - pipeline and background publish both preserve generation ordering;
     - no directory walk, content comparison, `syncGeneratedFiles`, telemetry
       observation, or `MarkIntegrityCachesStale()` runs while
       `entityArtifactsMu` is held;
     - hung git command is canceled at the deadline and releases the engine-lane
       slot within a bounded test timeout;
     - git timeout/cancellation returns a visible operation failure;
     - publish cache stale marking still happens after successful live mutation.

7. Context and work-lane lifecycle hardening.
   - `Engine` stores the daemon context attached from `web.Run`, using the same
     lifecycle setup as the work lane. It is read safely by continuation
     submitters.
   - Entity refresh continuations and reload cleanup use the stored daemon
     context unless a narrower caller context is explicitly available and
     guaranteed to remain valid for the continuation. They must not use
     `context.Background()` or a lane item context that will be canceled when
     the current item exits.
   - Make `AttachContext` idempotent with `sync.Once` or equivalent
     single-registration state.
   - Harden `syncStart` notifications in both `Shutdown()` and scheduling:
     snapshot channels under `WorkLane.mu`, release the lock, then notify after
     unlock. No channel send may happen while `WorkLane.mu` is held.
   - Add lane slot-hold diagnostics with a named 30-second default threshold.
     Long work is reported, not automatically canceled.
   - Classify all production `context.Background()` uses as daemon-owned,
     request-owned, CLI-owned, local short helper, or test-only. The audit must
     include at least `entity_artifacts.go`, `public_series.go`, `query.go`,
     `latest_set_cache.go`, `query_set_cache.go`, `web_batch.go`,
     `unique_share.go`, `integrity_check.go`, `web/surface_handler.go`,
     `web/integrity.go`, `entity_refresh_queue.go`, and `engine.go`.
   - Tests:
     - multiple `AttachContext` calls produce one shutdown behavior;
     - `Run` waiting for admission returns caller context error when canceled
       before a slot opens;
     - queued blocking `Run` items cannot deadlock during shutdown;
     - active callback blocked on scheduler trigger admission exits or fails
       visibly during shutdown;
     - continuation submissions cancel on daemon shutdown;
     - slot-hold diagnostics emit for blocked callbacks.

8. Validation, reviewer loop, and repeated gap analysis.
   - Run focused tests after each slice.
   - Run targeted race tests for touched packages:
     `go test -race -count=1 ./pkg/systemd ./pkg/output ./pkg/scheduler ./pkg/engine ./pkg/web`.
   - Run `make test-strict`.
   - Run `make test`.
   - Run `make race` unless the change set or environment makes it impossible;
     if not run, record why.
   - Run relevant UI build/test/lint commands because admin status behavior and
     API shape change.
   - Update specs/docs:
     - `operating-principles.md`: watchdog diagnostics, liveness, bounded git
       sync, lock-order rules.
     - `pipeline.md`: bounded git sync and engine-lane slot-hold semantics.
     - `admin-ui.md`: default light status and `?mode=full`.
     - `config.md` and runtime docs: `push_to_git_timeout` if added.
     - `files-layout.md`: generated git sync remains post-publish and bounded.
   - Run the same external reviewer set on the code diff.
   - Repeat deadlock/stall gap analysis on the new baseline and continue until
     no verified, fixable liveness finding remains.

### External Plan Review Round 7 - 2026-06-25

Plan reviewed: `Implementation Plan V7 - 2026-06-25`.

Reviewers run:

- `deepseek`: **production-grade plan**.
- `qwen`: **production-grade plan**.
- `mimo`: **production-grade plan**, with implementation precision notes.
- `glm`: **not production-grade plan**.
- `minimax`: **not production-grade plan**.
- `kimi`: **not production-grade plan**.

Accepted findings that V7 did not fully cover:

1. Scheduler panic can create a silent action-drain failure.
   - Evidence:
     - `pkg/scheduler/scheduler.go:111-115` sends to `actionCh` with no context
       or scheduler-health escape path.
     - `pkg/scheduler/scheduler.go:293-301` drains `actionCh` synchronously via
       `handleAction`.
     - `rg "recover()" pkg/scheduler` found no panic containment.
     - `pkg/web/integrity.go:461` and `pkg/web/integrity.go:469` call scheduler
       triggers from lane callbacks; the lane callback checks cancellation around
       the send, but the send itself can still block forever if the scheduler
       action consumer died.
   - Failure model: a panic in scheduler action handling, fetch loop, processing
     loop, or downloader work can stop the goroutine that drains actions. Later
     HTTP/admin/background callbacks can block forever trying to submit an
     action. This matches an idle-CPU/no-log stall more closely than overload.

2. The self-health observer decision path must be explicitly lock-free.
   - V7 says diagnostics are stack-first and lock-free, but the fire/no-fire
     decision itself must also avoid engine, lane, scheduler, telemetry,
     filesystem, and HTTP state locks.
   - The decision path must use only atomic timestamps/counters and monotonic
     time. Best-effort state capture can happen only after the decision.

3. HTTP-accepted background work must not inherit a request context after the
   handler returns.
   - Evidence:
     - `pkg/web/integrity.go:208` queues entity artifact rebuild work using the
       HTTP handler context.
     - `pkg/engine/entity_refresh_queue.go:32-42` passes that context to the
       engine lane admission path.
   - Failure model: a successful admin response can cancel the queued work as
     soon as the request ends, producing incomplete background work without a
     clear operator-facing cause.

4. Git subprocess timeout handling must explicitly reap child processes.
   - Evidence:
     - `pkg/output/sync.go:222-250` shells out through `exec.Command` helpers.
   - The implementation must prove every timed-out/canceled git child reaches
     `Wait` through `Run`, `Output`, or `CombinedOutput`, or equivalent explicit
     process reaping.

5. ASN database reload can race with an in-flight acquire/open.
   - Evidence:
     - `pkg/engine/ip_context.go:98-124` opens/caches ASN databases.
     - `pkg/engine/ip_context.go:148-168` retires cached databases.
     - `pkg/engine/engine.go:266-269` reload calls `retireAll`.
   - The implementation needs epoch/generation handling or an equivalent guard
     so an old in-flight open cannot be published as fresh state after reload,
     and so waiting callers do not trigger duplicate opens.

6. `markRunEnd()` needs a typed finalizing contract.
   - Evidence:
     - `pkg/engine/run.go:253-279` clears run state while final accounting and
       final telemetry/status updates are still possible.
     - `pkg/engine/status_snapshot.go:62-77` exposes status without a typed
       finalizing state.
   - Operators need to distinguish `running`, `finalizing`, and `idle` instead
     of seeing a temporarily ambiguous half-final status.

7. Status/metrics lock-order changes need mechanical tests.
   - The implementation must prove light and full status do not hold `e.mu`
     while calling metrics, lane, scheduler, background, or active-operation
     snapshot providers.

8. Reload-swapped pointers and public/admin reader methods need exact coverage.
   - V7 named cache pointer publication but did not explicitly name every
     relevant reader. The audit must include at least `Config`, `Runtime`,
     `EntriesSnapshot`, `EntriesSnapshotWithArtifacts`, `ActiveFeedsSnapshot`,
     `MergeCompositions`, `lookupSource`, and Geo/ASN provider readers.

9. Direct runtime/proc sampling must be closed beyond `runtime.ReadMemStats`.
   - Evidence:
     - `pkg/engine/run_diagnostics.go:386` samples `runtime.NumGoroutine`.
     - `pkg/web/sysinfo.go:123` and `pkg/web/sysinfo.go:175` can sample runtime
       data from HTTP-facing paths.
   - The sampler closure must include `runtime.NumGoroutine` and `/proc/self`
     reads, not only `runtime.ReadMemStats`.

10. Scheduler snapshot use in light admin status must be cached or bounded.
    - Evidence:
      - `pkg/scheduler/scheduler.go:142-167` can rebuild a stale snapshot under
        the scheduler snapshot lock.
      - `pkg/web/admin_status_light.go:26` currently calls the scheduler
        snapshot path from the light status response.
    - The light status endpoint must not synchronously rebuild scheduler state
      or wait behind long scheduler snapshot work.

11. Validation and reviewer prompts need to explicitly re-check the findings in
    this round. Reviewers must be asked to verify the new deadlock class, not
    only the original engine-lane/watchdog hypothesis.

Downgraded or rejected findings:

- Generic rate-limiter pruning is not tied to the observed production stalls and
  is outside this liveness fix.
- Pipeline progress watchdog signals for scheduler-owned loops are useful
  diagnostics but secondary to panic containment, action admission health, and
  lock-free self-health diagnostics.

### Implementation Plan V8 - 2026-06-25

Plan V8 supersedes V7, V6, V5, V4, V3, V2, and V1. It keeps all prior accepted
watchdog, work-lane, status, git-timeout, entity-artifact, cache, reload, and
runtime-sampler work, and adds the Round 7 findings below.

1. Scheduler panic containment and bounded action admission.
   - Add panic containment around scheduler action handling and scheduler-owned
     loops:
     - `handleAction` dispatch from `Run`;
     - fetch loop;
     - processing loop;
     - downloader work started by the scheduler.
   - A recovered panic must:
     - be logged with bounded structured context;
     - increment a scheduler health/failure counter;
     - update scheduler/admin-visible degraded state;
     - keep the scheduler running when continuing is safe, or mark the scheduler
       stopped/fatal when continuing is unsafe.
   - Replace unbounded action sends with context-aware admission:
     - add or harden `TriggerSourcesContext`;
     - select on action send, caller context, daemon shutdown, and scheduler
       fatal/stopped signal;
     - return a visible error instead of blocking forever.
   - Keep a compatibility wrapper only if it is bounded by a named timeout or is
     removed from production call sites.
   - Tests:
     - injected panic in action handling does not permanently block later HTTP
       or lane trigger admission;
     - injected scheduler-loop/downloader panic is visible in scheduler status
       and logs/counters;
     - an integrity lane callback cannot hold an engine-lane slot forever when
       scheduler action admission is unavailable;
     - shutdown unblocks pending trigger admission.

2. Lock-free self-health observer decision path.
   - The decision to emit a self-health diagnostic reads only atomic
     last-success timestamps/counters and monotonic time.
   - Before the decision, it must not call engine, lane, scheduler, telemetry,
     filesystem, HTTP, or status snapshot code.
   - After the decision, diagnostics capture goroutine stacks first, then gather
     best-effort state through try-lock or bounded-time helpers.
   - Tests:
     - with engine/lane/telemetry/status locks deliberately blocked, the
       observer still emits the stack-first diagnostic;
     - diagnostics clearly mark any skipped best-effort state.

3. HTTP accepted work uses daemon/job contexts after admission.
   - `Engine` owns a daemon context attached from the server lifecycle.
   - HTTP request context gates only request admission and response generation.
   - Once a background job is accepted, work that should outlive the HTTP request
     uses daemon context or a daemon-derived job context.
   - Audit HTTP/admin queueing call sites, including
     `pkg/web/integrity.go:208` and `QueueEntityArtifactsRebuild`.
   - Tests:
     - a handler can return an accepted response, the request context can be
       canceled, and the accepted job still reaches a terminal visible state;
     - daemon shutdown cancels accepted jobs visibly.

4. Git subprocess timeout and reaping contract.
   - All git subprocesses use context-bounded commands or equivalent explicit
     timeout handling.
   - Timed-out/canceled subprocesses must be waited/reaped before the helper
     returns.
   - The git sync timeout remains operator-visible and releases the engine-lane
     slot.
   - Tests:
     - a fake hung git command times out, returns a typed/visible failure, and
       releases the engine-lane slot;
     - the timeout path reaches command wait/reap completion.

5. ASN/Geo cache acquire, reload, and retire safety.
   - Public query cache opens must not hold cache-wide mutexes across disk open
     or expensive parsing.
   - Waiting on an in-flight open must be context-cancelable.
   - `retireAll` must use an epoch/generation guard or equivalent mechanism so
     old in-flight opens cannot publish stale cache state after reload.
   - Tests:
     - concurrent reload and in-flight acquire does not leak, double-close, or
       publish stale state;
     - concurrent callers for the same provider share one open;
     - canceled public request stops waiting for the in-flight open.

6. Typed run finalizing state.
   - Add a public/admin status field with explicit run state, such as
     `run_state` with values `idle`, `running`, and `finalizing`.
   - `markRunEnd()` must make finalization observable until all final metrics,
     active operation cleanup, and final status state are coherent.
   - Tests:
     - during a synthetic finalization window, light and full status expose
       `finalizing`;
     - after finalization, status returns to `idle`;
     - no half-final metrics are visible as an ordinary idle run.

7. Mechanical status and metrics lock-order tests.
   - Prove light status and full status do not hold `e.mu` while calling:
     - current metrics snapshot;
     - lifetime metrics snapshot;
     - scheduler snapshot;
     - work-lane snapshot;
     - background task snapshot;
     - active operation snapshot.
   - Use behavioral blocking seams or lock-order guards in tests; do not rely on
     code review alone.

8. Reload-swapped pointer and reader audit.
   - Use atomic pointer publication or short-lock snapshotting for reload-swapped
     fields.
   - Audit and update at least:
     - `Config`;
     - `Runtime`;
     - `EntriesSnapshot`;
     - `EntriesSnapshotWithArtifacts`;
     - `ActiveFeedsSnapshot`;
     - `MergeCompositions`;
     - `lookupSource`;
     - Geo/ASN provider readers;
     - runtime ledger cache readers.
   - Race tests must run concurrent reload and public/admin readers.

9. Runtime sampler closure.
   - Move direct `runtime.ReadMemStats`, `runtime.NumGoroutine`, and `/proc/self`
     process-stat reads out of HTTP-facing hot paths.
   - HTTP status/sysinfo handlers read the cached sampler output only. If no
     sample exists yet, return a clearly marked unavailable/zero sample instead
     of sampling synchronously.
   - Static validation:
     - `rg -n 'captureEngineRuntimeStats|runtime\\.ReadMemStats|runtime\\.NumGoroutine|/proc/self' pkg/engine pkg/web`
       must show only the sampler implementation, tests, or documented
       non-hot-path exceptions.

10. Scheduler snapshot light-path hardening.
    - The light admin status response uses a cached-only scheduler summary or a
      bounded try-refresh.
    - It must not synchronously rebuild a stale scheduler snapshot under
      request handling.
    - Tests:
      - with scheduler snapshot rebuild deliberately blocked, light status still
        returns within a bounded timeout;
      - full status may request richer data only through explicit full-mode
        semantics and must still obey bounded behavior.

11. Validation, reviewer loop, and repeated gap analysis.
    - Run focused tests after each implementation slice.
    - Run targeted race tests:
      `go test -race -count=1 ./pkg/systemd ./pkg/output ./pkg/scheduler ./pkg/engine ./pkg/web`.
    - Run `make test-strict`.
    - Run `make test`.
    - Run `make race` unless the environment makes it impossible; if skipped,
      record the reason.
    - Run relevant UI build/test/lint commands because admin status API shape
      changes.
    - Update specs/docs:
      - `operating-principles.md`: liveness observer, panic containment,
        watchdog semantics, bounded git sync, status/cache lock rules;
      - `pipeline.md`: bounded scheduler/action admission and engine-lane
        slot-hold semantics;
      - `admin-ui.md`: light/full status behavior and `run_state`;
      - `config.md`: git timeout setting if the implementation exposes one;
      - `files-layout.md`: generated git sync remains post-publish and bounded.
    - Run the same external reviewer set on the code diff.
    - Repeat deadlock/stall gap analysis on the new baseline and continue until
      no verified, fixable liveness finding remains.

### External Plan Review Round 8 - 2026-06-25

Plan reviewed: `Implementation Plan V8 - 2026-06-25`.

Reviewers run:

- `glm`: **production-grade plan**, with optional implementation-watch items.
- `deepseek`: **production-grade plan**, with optional precision additions.
- `qwen`: **production-grade plan**, with caveats about implementation order and
  load validation.
- `mimo`: **not production-grade plan**, but several findings were stale against
  V8 or contradicted by local code verification.
- `minimax`: **not production-grade plan**, mainly because the current code has
  not implemented V8 yet; it also raised useful precision items.
- `kimi`: **not production-grade plan**, mainly due to scope and precision gaps;
  several precision items were accepted.

Accepted findings to fold into V9:

1. The `WorkLane`/engine lock contract needs an explicit mechanical test.
   - Evidence:
     - `pkg/engine/engine.go:253-263` currently calls `WorkLane.SetLimit()` while
       holding `e.mu`.
     - `pkg/engine/status_snapshot.go:10-15` calls `engineLane.Snapshot()` before
       taking `e.mu.RLock()`.
   - Local verification: the status path does **not** hold `WorkLane.mu` while
     taking `e.mu`; `Snapshot()` returns after releasing `WorkLane.mu`. So the
     cited status/reload path is not a proven AB-BA deadlock. Still, V9 must
     make the no-nested-`e.mu`/`WorkLane.mu` contract mechanical because reload
     already creates one side of a risky ordering.

2. `syncStart` send-under-lock needs exact call-site treatment.
   - Evidence:
     - `pkg/engine/work_lane.go:321-322` sends to `syncStart` while
       `WorkLane.mu` is held during shutdown.
     - `pkg/engine/work_lane.go:409-410` sends to `syncStart` while
       `WorkLane.mu` is held during scheduling.
   - V9 must require snapshotting notification channels under lock and sending
     after unlock, with tests for shutdown/admission races.

3. Scheduler panic containment needs exact main-loop semantics.
   - Evidence:
     - `pkg/scheduler/scheduler.go:293-301` calls `handleAction` directly from
       the main scheduler select loop.
     - `pkg/scheduler/actions.go:18` begins the action handler.
     - `rg "recover\\(\\)" pkg/scheduler` finds no scheduler panic containment.
   - V9 must state that the main scheduler loop recovers `handleAction` panics,
     logs/counts them, marks scheduler degraded, and continues draining unless a
     fatal scheduler shutdown is explicitly recorded.

4. `handleAction` must remain bounded and memory-only.
   - Evidence: `pkg/scheduler/actions.go:18-92` dispatches recheck/reprocess and
     enqueue decisions inline on the scheduler action drain.
   - V9 must require `handleAction` to stay free of disk/network work and avoid
     indefinite waits; any slow work belongs in download/processing workers or a
     bounded admission path.

5. Status snapshot short-lock mechanics need exact wording.
   - Evidence:
     - `pkg/engine/status_snapshot.go:15-16` uses `defer e.mu.RUnlock()` in light
       status.
     - `pkg/engine/status_snapshot.go:62-63` uses `defer e.mu.RUnlock()` in full
       status.
   - V9 must require copying scalars and immutable/current pointers under a short
     lock and explicitly unlocking before any helper that can take telemetry,
     scheduler, lane, background, active-operation, cache, filesystem, or runtime
     sampler locks.

6. `finishItem` needs defensive panic containment.
   - Evidence: `pkg/engine/work_lane.go:492-515` finalizes lane work, closes
     `l.idle`, schedules queued work, observes gauges, and starts more workers.
   - The normal path is safe, but a panic here would bypass slot release
     invariants. V9 must require defensive recovery that leaves the lane in a
     visible failed/degraded state and does not leave the slot held.

7. The light admin status builder must be explicitly detached from full feed-row
   generation.
   - Evidence:
     - `pkg/web/admin_status_light.go:27` currently calls
       `buildAdminFeedsWithStatusEntries`.
   - V9 must require a true summary-only builder and a test that the light status
     path does not call the full feed-row builder or force a scheduler snapshot
     rebuild.

8. Runtime sampler ownership must be unambiguous.
   - Evidence:
     - `pkg/engine/run_diagnostics.go:81`, `:123`, and `:154` call
       `captureEngineRuntimeStats()` directly.
     - `pkg/web/sysinfo.go` has web-side runtime status capture.
   - V9 must define one owner for runtime/proc sampling and make engine/web hot
     paths consume cached samples only. The static grep must include
     `run.go:97`/`logRunDiagnosticSummary` reachability.

9. `web_batch.go` publish helpers must be named in the artifact-lock tests.
   - Evidence:
     - `pkg/engine/web_batch.go:202` walks staged files.
     - `pkg/engine/web_batch.go:238-239` compares/stat files.
     - `pkg/engine/web_batch.go:305-306` stats destination files.
   - V9 must explicitly prove these helpers do not run while
     `entityArtifactsMu` is held.

10. Startup trigger ordering needs a bounded contract.
    - Evidence:
      - `pkg/web/server_run.go` queues startup integrity recovery actions before
        the background runner is fully running.
      - `pkg/scheduler/scheduler.go:67` gives `actionCh` a fixed capacity of 64.
    - V9 must require startup recovery to use bounded trigger admission and not
      depend on action channel spare capacity before the scheduler consumer is
      live.

11. Diagnostic sanitization needs fixture-based tests.
    - V8 requires sanitized diagnostics. V9 must add a fixture with request
      bodies, secrets, raw feed content, long path lists, and IP values and prove
      diagnostics do not emit them.

12. The admin status default contract should stay backward-compatible unless the
    user explicitly approves a breaking API change.
    - Evidence:
      - `pkg/web/admin.go:262-266` currently makes full status the default and
        light status opt-in via `?mode=light`.
    - V9 implementation should make the admin UI poll `?mode=light` and keep
      `?mode=full`/default full unless an explicit design decision changes the
      API contract. This fixes the UI polling issue without breaking existing
      external admin-status consumers.

Rejected or downgraded findings:

- Strict AB-BA between lifetime telemetry locks and `e.mu` is not proven.
  `internal/telemetry.TimingBook.Observe` and `CounterBook.Add` release their
  locks before `observeRunOperation`/`observeRunCounter` take `e.mu.RLock()`
  (`pkg/engine/run_metrics_state.go:13-17`, `:58-62`).
- Strict AB-BA between `WorkLane.mu` and `e.mu` via the cited status path is not
  proven because `engineLane.Snapshot()` releases `WorkLane.mu` before
  `StatusSnapshotLight()` takes `e.mu.RLock()` (`pkg/engine/status_snapshot.go:10-15`).
  The reload side remains a lock-order smell and V9 keeps the no-nesting
  contract.
- `debug.SetMemoryLimit(-1)` in `goMemLimit()` is not a setter bug. Go's
  `runtime/debug.SetMemoryLimit` documents that negative input retrieves the
  current limit without changing it. V9 still moves memory-limit reads to the
  sampler-owned path for consistency.
- Downloader HTTP timeout absence is rejected for the main HTTP downloader path:
  `pkg/downloader/downloader.go:78-89` builds an `http.Client` with
  `Timeout: MaxDownloadTime` and `ResponseHeaderTimeout: MaxConnectTime`, and
  `pkg/engine/runtime.go:219-225` defaults those to 10s connect and 300s
  download.
- Splitting this regression into multiple SOWs is rejected for now. The project
  SOW rules say overlapping SOWs must be merged or consolidated, and the
  production symptom spans scheduler admission, watchdog, status, reload, git,
  and cache serving. V9 keeps one SOW but requires strict implementation slices
  with validation after each slice.

### Implementation Plan V9 - 2026-06-25

Plan V9 supersedes V8, V7, V6, V5, V4, V3, V2, and V1. It keeps all V8
contracts and adds the Round 8 precision items below.

1. Work in strict implementation slices, not as one unreviewed batch.
   - Slice order:
     1. watchdog/systemd deadlines and self-health diagnostics;
     2. scheduler panic containment and bounded trigger admission;
     3. admin light-status and status/metrics/background lock cleanup;
     4. reload context, lock scope, and atomic pointer publication;
     5. git subprocess timeout/reaping;
     6. entity artifact publish lock narrowing;
     7. public query/cache mutex refactors;
     8. work-lane lifecycle hardening and slot-hold diagnostics;
     9. final typed `run_state`, docs/specs, UI contract, and full validation.
   - Each slice gets focused tests before the next slice starts.
   - The SOW stays one unit because the findings overlap on the same production
     symptom and shared lock/admission contracts.

2. Scheduler liveness semantics.
   - The `Runner.Run` main loop recovers panics from `handleAction`, logs them,
     increments a scheduler failure counter, marks scheduler degraded in
     snapshots/admin status, and continues draining actions unless the scheduler
     is explicitly marked fatal/stopped.
   - Fetch loop, processing loop, and downloader worker panics receive the same
     bounded logging/counter/degraded-state treatment.
   - `handleAction` must remain memory-only and bounded; it may enqueue work and
     wake loops, but it must not perform disk/network work or wait indefinitely
     on engine locks.
   - `TriggerSourcesContext`/non-blocking trigger paths must be used by HTTP,
     startup recovery, integrity lane callbacks, and operator/admin routes.
   - Tests:
     - injected `handleAction` panic does not stop action draining;
     - startup recovery with more than the action-channel capacity does not
       block before the scheduler consumer is live;
     - slow or unavailable trigger admission fails visibly and releases any
       engine-lane slot.

3. Status/admin lock mechanics.
   - `StatusSnapshotLight()` and full `statusSnapshot()` must not use
     long-lived `defer e.mu.RUnlock()` patterns. They copy scalars and immutable
     pointer references under a short lock, explicitly unlock, and only then call
     helper functions that may take other locks.
   - Mechanical tests must fail if status code holds `e.mu` while acquiring:
     telemetry, lane, scheduler, background-task, active-operation, cache,
     filesystem, or runtime sampler locks.
   - The admin UI polls `?mode=light`; the server's default status response
     remains backward-compatible unless the user explicitly approves changing
     the API default.
   - `buildAdminStatusLight` must not call `buildAdminFeedsWithStatusEntries`.
     It uses summary-only cached data and bounded/cached scheduler state.

4. Runtime sampler ownership.
   - Define one owner for runtime/proc sampling. Engine and web request/progress
     hot paths consume cached samples only.
   - `logRunDiagnosticSummary` reached from `pkg/engine/run.go:97` and progress
     logging reached from `pkg/engine/run_diagnostics.go:81`, `:123`, and `:154`
     must not directly call `captureEngineRuntimeStats()`.
   - Static validation must show direct `runtime.ReadMemStats`,
     `runtime.NumGoroutine`, `debug.SetMemoryLimit(-1)`, and `/proc/self` reads
     only in the sampler implementation and tests.

5. Work-lane hardening.
   - `syncStart` notifications are collected under `WorkLane.mu`, then sent
     after unlock. No send to `syncStart` may occur while `WorkLane.mu` is held.
   - `finishItem` has defensive panic containment. If finalization panics, the
     lane records a visible failure/degraded state and does not leave the work
     slot held.
   - `AttachContext` is idempotent. Duplicate calls are visible through a debug
     log or counter so construction bugs are not hidden.
   - Tests:
     - shutdown cannot deadlock with queued `Run` callers waiting on `syncStart`;
     - a forced finalization panic does not wedge later queued work;
     - duplicate `AttachContext` calls do not create duplicate shutdown
       behaviors and are observable.

6. Entity artifact and web publish lock proof.
   - Tests must explicitly prove no filesystem walk/stat/compare/open/remove,
     `web_batch.go` helper work, git sync, telemetry observation, or
     `MarkIntegrityCachesStale()` executes while `entityArtifactsMu` is held.
   - Named helper evidence includes `pkg/engine/web_batch.go:202`, `:238-239`,
     and `:305-306`.

7. Diagnostics sanitizer fixtures.
   - Pre-watchdog diagnostics use a fixture with request bodies, secrets, raw
     feed snippets, long path lists, and IP values. Tests must prove the emitted
     diagnostic omits or redacts them.

8. Load/liveness validation.
   - Add a focused end-to-end liveness test or test harness that simulates:
     - active processing/background work;
     - concurrent admin light-status polling;
     - scheduler triggers;
     - entity refresh/publish activity.
   - The test must assert that health/status handlers respond within a bounded
     timeout and that watchdog/self-health diagnostics are not blocked by engine
     work.

9. Validation and review.
   - Keep all V8 validation commands and add the V9 mechanical/fixture/load
     tests.
   - Re-run the external reviewer set on the implementation diff, not only on
     the plan.
   - Repeat the deadlock/stall gap analysis on the new baseline after tests pass.

### Active Plan Note - 2026-06-25

The active implementation plan is `Implementation Plan V9 - 2026-06-25`.
Historical sections above are retained as review evidence. V9 supersedes V8,
V7, V6, V5, V4, V3, V2, and V1 regardless of section ordering in this
regression log.

## External Plan Review Round 9 - 2026-06-25

Review scope:

- Active SOW plan V9.
- Production observations from the 2026-06-21 and 2026-06-25 watchdog/deadlock
  reports.
- Current code paths that can block with idle CPU, no logs, or stalled admin UI.

Reviewers run:

- `glm`: **not production-grade plan**, close after eight concrete liveness
  additions.
- `minimax`: **production-grade plan with required refinements**.
- `mimo`: **not production-grade plan**, due to mechanical precision gaps.
- `kimi`: **not production-grade plan**, due to residual accepted stall
  fallbacks.
- `deepseek`: **production-grade plan with closeable gaps**.
- `qwen`: **production-grade plan**.

Consensus:

- V9 has the correct root-cause model.
- V9 is still too ambiguous in several places where implementation could leave a
  confirmed production stall source in place.
- V10 must remove residual "acceptable stall" fallbacks and make the free-path,
  reload, cache, entity-publish, scheduler-trigger, and run-exit contracts
  mechanical.

Accepted findings to fold into V10:

1. SIGHUP and shutdown watcher goroutines need panic containment.
   - Evidence:
     - `cmd/update-ipsets/daemon.go:82-100` starts the SIGHUP reload goroutine
       with no `recover`.
     - `pkg/web/server_run.go:214` starts the shutdown watcher goroutine with no
       `recover`.
   - Risk: a panic can silently kill a control-plane goroutine and make reload or
     shutdown behavior disappear until process restart.

2. Blocking scheduler trigger sends remain a stall source.
   - Evidence:
     - `pkg/scheduler/scheduler.go:111-115` sends to `actionCh` without timeout.
     - `pkg/web/integrity.go:461` and `:469` call `TriggerSources()` from inside
       an engine-lane callback.
     - `pkg/web/server_run.go:109` and `:116` call `TriggerSources()` from
       startup recovery.
   - Risk: a full scheduler action channel blocks an HTTP/startup/lane caller
     and can hold the only engine-lane slot.

3. `handleAction` has to be bounded more precisely.
   - Evidence:
     - `pkg/scheduler/actions.go:18-92` performs recheck/reprocess/due dispatch
       inline on the scheduler action drain.
     - The `RunDue` branch builds a full scheduler snapshot from config,
       runtime, and `EntriesSnapshot()`.
   - Risk: if `handleAction` performs broad snapshots or waits on engine locks,
     one action can stall the whole scheduler action drain.

4. Public query cache open-under-global-lock is a free-path violation.
   - Evidence:
     - `pkg/engine/query_set_cache.go:45-52` holds
       `sharedLatestSetCache.mu` while calling `openLatestSet`.
   - Risk: public IP lookup cache misses serialize on one mutex and can stall
     public serving under I/O pressure.

5. Runtime ledger cache holds per-feed locks while loading files.
   - Evidence:
     - `pkg/engine/runtime_ledger_cache.go:537-560` holds `feedLedgerState.mu`
       while loading history.
     - `pkg/engine/runtime_ledger_cache.go:729-738` holds `feedLedgerState.mu`
       while loading retention cohorts.
   - Risk: public history/retention views can block behind disk I/O while holding
     cache locks.

6. Light admin status must be truly summary-only.
   - Evidence:
     - `pkg/web/admin_status_light.go:26` calls `runner.Snapshot()`.
     - `pkg/web/admin_status_light.go:27` calls
       `buildAdminFeedsWithStatusEntries(..., eng.EntriesSnapshot())`.
     - `pkg/scheduler/scheduler.go:153-165` rebuilds a full scheduler snapshot
       when the cached snapshot is stale.
   - Risk: the "light" polling path can still do full feed copies and scheduler
     rebuilds, freezing the admin UI under load.

7. Reload still has lock and race hazards.
   - Evidence:
     - `pkg/engine/engine.go:253-298` holds `e.mu` while updating runtime,
       calling `engineLane.SetLimit`, ensuring directories, reconciling entries,
       and bootstrapping entries from disk.
     - `pkg/engine/engine.go:316-322` exposes `Config()` and `Runtime()` without
       synchronization while reload swaps those fields under `e.mu`.
   - Risk: SIGHUP can block status readers and public paths through writer
     preference on `sync.RWMutex`; unsynchronized config/runtime reads are a
     race.

8. Entity artifact publish must not keep a live-I/O fallback under
   `entityArtifactsMu`.
   - Evidence:
     - `pkg/engine/entity_artifact_publish.go:95-131` holds
       `entityArtifactsMu` across `publishContext`, `syncGeneratedFiles`,
       telemetry observation, and `MarkIntegrityCachesStale`.
     - `pkg/engine/run_pipeline.go:400-423` has the pipeline batch equivalent.
     - `pkg/engine/web_batch.go:202`, `:238-239`, and `:305-306` walk/stat
       staged and live files.
   - Risk: the fallback permitted by earlier plans would keep the exact I/O
     stall source that production exposed.

9. Git subprocess cancellation needs process-tree reaping detail.
   - Evidence:
     - `pkg/output/sync.go:222-250` uses `exec.Command` without context.
   - Risk: `CommandContext` alone may kill only the direct `git` process while
     helper children continue running.

10. Run-exit cache persistence remains an accepted lane-held I/O operation.
    - Evidence:
      - `pkg/engine/run.go:84-98` calls `cache.Save`, final diagnostics, and
        `markRunEnd` inside the admitted run callback before the engine-lane slot
        is released.
    - Risk: slow disk can hold the only engine-lane slot after processing has
      effectively finished.

11. Work-lane finalization and start notification semantics need exact
    boundaries.
    - Evidence:
      - `pkg/engine/work_lane.go:321-322` and `:409-410` send `syncStart`
        notifications while `WorkLane.mu` is held.
      - `pkg/engine/work_lane.go:492-515` finalizes lane work with no recovery.
      - `pkg/engine/work_lane.go:471-481` starts lane worker goroutines.
    - Risk: a finalization panic can leave a slot held; a future change to
      notification buffering could make the current send-under-lock pattern a
      real deadlock.

12. Runtime/proc sampler ownership must be implementable.
    - Evidence:
      - `pkg/engine/run_diagnostics.go:81`, `:123`, and `:154` call
        `captureEngineRuntimeStats()` directly.
      - `pkg/web/sysinfo.go:120-127` uses a fallback that calls
        `runtime.NumGoroutine()` and `goMemLimit()`.
    - Risk: a literal "single web-owned sampler" would create bad package
      direction because engine cannot import web; the fallback would violate the
      static rule.

13. Diagnostic sanitizer tests need concrete caps and redaction rules.
    - Risk: diagnostics intended to explain watchdog stalls could leak request
      bodies, raw feed snippets, secrets, unbounded path lists, or IP values.

14. Load/liveness validation needs concrete success thresholds.
    - Risk: an end-to-end test that only "runs" does not prove that health,
      watchdog diagnostics, and light admin status stay available during blocked
      engine work.

Accepted decisions:

- Keep this as one SOW, but implement it in strict phases. Splitting into
  separate SOWs is rejected because the production symptom spans overlapping
  locks, queues, status, reload, cache, and entity-publish behavior. The project
  SOW rules require overlapping work to be consolidated.
- Preserve the current default `GET /api/v1/admin/status` full payload for
  external compatibility unless the user explicitly approves changing the API
  default. This is acceptable only if the full path is also short-lock and
  bounded; the admin UI must poll `?mode=light`.
- Do not accept a fallback that keeps live filesystem work under
  `entityArtifactsMu`.
- Do not accept unbounded lane-held run-exit I/O. If cache persistence remains
  associated with run completion, it must not be capable of wedging the engine
  lane indefinitely.

Rejected or downgraded findings:

- The current buffered `syncStart` sends are not proven deadlocks today, because
  the channel is capacity one and normally empty. V10 still hardens them because
  send-under-lock is fragile and unnecessary.
- Direct downloader HTTP timeout absence remains rejected. The main downloader
  already has connect and download timeouts.
- `debug.SetMemoryLimit(-1)` remains rejected as a setter bug. The call is valid
  as a read, but V10 moves or removes the direct hot-path usage.
- Client rate-limiter pruning is excluded from this liveness fix. The pruning
  path is bounded map cleanup, does not perform disk/network work, and does not
  take engine locks.

### Implementation Plan V10 - 2026-06-25

Plan V10 supersedes V9, V8, V7, V6, V5, V4, V3, V2, and V1. Historical plans
above remain review evidence only. V10 is the active implementation contract.

1. Work in phases with focused tests after each phase.
   - Phase 1: watchdog/systemd deadlines, self-health diagnostics, SIGHUP and
     shutdown watcher panic containment.
   - Phase 2: scheduler panic containment, bounded/non-blocking trigger
     admission, and bounded `handleAction`.
   - Phase 3: true light admin status, full-status short-lock cleanup,
     background-task lock separation, and runtime/proc sampler.
   - Phase 4: reload lock narrowing, config/runtime atomic publication, and
     daemon-context reload continuations.
   - Phase 5: run-exit finalization and cache persistence so the engine lane
     cannot be wedged by post-run disk I/O.
   - Phase 6: git subprocess timeout/reaping.
   - Phase 7: entity artifact and web publish lock narrowing.
   - Phase 8: public query/cache and runtime ledger cache lock refactors.
   - Phase 9: work-lane lifecycle hardening, final typed `run_state`,
     docs/specs/UI contract, full validation, and external implementation
     review.
   - Each phase must leave tests passing before the next phase starts.

2. Watchdog, systemd notify, and control-goroutine liveness.
   - `pkg/systemd.Notify` and all ready/stopping/status/watchdog messages are
     deadline-bound. If the watchdog interval is active, use the smaller of two
     seconds and half the watchdog interval; otherwise use a two-second lifecycle
     notify deadline.
   - Watchdog notify failures and slow calls are visible through rate-limited
     logs and counters.
   - The watchdog heartbeat goroutine, SIGHUP reload goroutine, and shutdown
     watcher goroutine recover panics, log/counter them, and keep their control
     loops alive where continuing is safe.
   - Pre-watchdog diagnostics first emit a capped, sanitized goroutine sample
     without taking engine/scheduler locks. Engine-lane, scheduler, and runtime
     snapshots are best-effort and timeout/try-lock bounded.
   - Tests:
     - blocked/unresponsive notify socket returns within the deadline;
     - ready/stopping/status/watchdog payload compatibility is preserved;
     - a panic in SIGHUP reload handling does not permanently disable later
       reload signals;
     - diagnostics do not block on intentionally blocked engine locks.

3. Scheduler action admission and bounded action handling.
   - `Runner.Run` recovers panics from `handleAction`, logs/counters them, marks
     scheduler degraded in snapshots/admin status, and continues draining
     actions unless explicitly stopped or fatal.
   - Fetch loop, processing loop, and downloader worker panics receive the same
     bounded logging/counter/degraded-state treatment.
   - Replace blocking `TriggerSources()` use from HTTP/admin, startup recovery,
     and engine-lane callbacks with:
     - a context-bounded must-deliver API for startup/internal recovery;
     - a non-blocking/try API for HTTP/admin calls that returns a visible
       saturated result;
     - lane-internal trigger calls that can merge compatible pending actions or
       fail visibly, but never block while holding an engine-lane slot.
   - Full-channel semantics must be explicit: merge compatible name-set actions
     when safe, otherwise return a saturated error/counter instead of blocking.
   - `handleAction` must not perform disk/network work, broad snapshot rebuilds,
     lane operations, goroutine starts, or indefinite waits. It may only do
     bounded in-memory queue admission and wake loops. Any data it needs from
     engine state must be obtained through short-lock snapshot helpers outside
     the action-channel critical path.
   - Tests:
     - injected `handleAction` panic does not stop later action draining;
     - a full action channel does not block startup recovery, HTTP/admin calls,
       or a lane callback;
     - lane-internal trigger saturation releases the engine-lane slot.

4. Admin status and free-path lock mechanics.
   - `StatusSnapshotLight()` and full `statusSnapshot()` copy scalars and
     immutable/current pointer references under a short `e.mu` lock, explicitly
     unlock, and only then call helpers that may take telemetry, lane,
     scheduler, background-task, active-operation, cache, filesystem, or runtime
     sampler locks.
   - The light admin status path must not call:
     - `buildAdminFeedsWithStatusEntries`;
     - `eng.EntriesSnapshot()` or `EntriesSnapshotWithArtifacts()`;
     - `runner.Snapshot()` when it would rebuild from all entries.
   - The light path consumes bounded cached counters and cheap scheduler
     activity only. If a cached scheduler summary is stale, it reports stale or
     unknown summary state rather than rebuilding the full snapshot inline.
   - The admin UI polls `GET /api/v1/admin/status?mode=light`. The default
     status route keeps the current full payload for compatibility, but the full
     path must also obey short-lock rules and must not block health/watchdog or
     the light path.
   - Background task updates move off `e.mu` to a dedicated lock or equivalent
     atomic state. `BackgroundTaskHandle.Update()` and `Finish()` must not take
     `e.mu`.
   - Tests:
     - light status responds under a bounded timeout while engine work, scheduler
       triggers, and entity publish are active or blocked;
     - light status does not rebuild a stale full scheduler snapshot;
     - default full status and `?mode=full` preserve the external payload shape.

5. Runtime/proc sampler ownership.
   - Put runtime/proc sampling in a low-level package that both engine and web
     can import, or keep separate bounded samplers with an explicit exception for
     admitted engine diagnostics. Do not make engine import web.
   - Web request/status paths consume cached samples only. The sampler publishes
     an initial sample before handlers can need the fallback, or the fallback
     must avoid direct runtime/proc calls.
   - Engine progress and run-end diagnostics use cached samples or a bounded
     admitted-work sampler that cannot block the free path.
   - Static validation must show direct `runtime.ReadMemStats`,
     `runtime.NumGoroutine`, `debug.SetMemoryLimit(-1)`, and `/proc/self` reads
     only in sampler implementation files and tests.

6. Reload and configuration publication.
   - Move `WorkLane.SetLimit()` outside `e.mu`. Collect the new limit while
     reloading, release `e.mu`, then apply it.
   - Move disk-touching reload work such as directory creation, entry bootstrap,
     timestamp repair, and legacy failure bootstrap out of the `e.mu` critical
     section where possible. If a small apply phase must hold `e.mu`, it can
     only mutate in-memory state and must be bounded.
   - Publish `cfg`, `runtime`, geo provider cache, ASN lookup cache, and ledger
     cache through atomic pointers or short-lock snapshot APIs so readers cannot
     race reload.
   - `Config()` and `Runtime()` must be data-race free.
   - SIGHUP and runtime callers use `ReloadContext(ctx)` or an equivalent
     context-bound reload API. Accepted reload continuations use daemon context,
     not `context.Background()` or request context.
   - Tests:
     - `SetLimit()` is not called while `e.mu` is held;
     - reload disk work does not block light status beyond the bounded threshold;
     - `go test -race` covers concurrent reload with admin status and public
       serving.

7. Run-exit finalization and cache persistence.
   - No unbounded disk I/O may hold an engine-lane slot after the admitted run's
     processing work is done.
   - `cache.Save` is moved to a serialized daemon-context persistence worker, or
     run finalization is split so the lane slot releases before persistence.
   - Persistence remains safe: writes use the existing atomic temp/rename
     behavior, saves are coalesced in order, shutdown waits for the last accepted
     save up to a bounded grace period, and failure is visible in status/logs.
   - `logRunDiagnosticSummary` and `markRunEnd` must not hold `e.mu` or the
     engine-lane slot while doing runtime/proc sampling or heavy metrics
     snapshot work.
   - Tests:
     - a blocked or slow cache save does not block a later engine-lane item;
     - shutdown reports a pending/failed save without wedging watchdog or web.

8. Git subprocess timeout and reaping.
   - Add context/deadline support to generated artifact git sync.
   - All git subprocesses, including `git add`, `git diff --cached`,
     `git commit`, `git push`, and `git gc --auto`, are bounded.
   - Timed-out/canceled commands kill and reap the process. On Unix, use a
     process group or equivalent when needed so helper children are not orphaned.
   - Timeout policy: use `runtime.push_to_git_timeout` with a documented default
     of ten minutes and validation rejecting negative values. Zero/omitted means
     the default, not unbounded.
   - Timeout/cancellation is visible through operation failure state and releases
     any engine-lane slot.

9. Entity artifact and web publish lock narrowing.
   - Remove any fallback that keeps live filesystem work under
     `entityArtifactsMu`.
   - The lock may protect generation checks, generation mutation, pointer/state
     swaps, and a short atomic promotion decision only. It must not cover:
     - filesystem walk/stat/compare/open/remove/rename/chmod/chown/chtimes;
     - `web_batch.go` helper work;
     - `syncGeneratedFiles()` or git sync;
     - telemetry observation;
     - `MarkIntegrityCachesStale()`.
   - Replace deferred unlock patterns with explicit critical sections so the
     lock scope is visible in code review.
   - Use structural tests or AST/static checks plus behavioral probes to prove
     the critical-section helper contains no `os`, `filepath`, git sync,
     telemetry, or integrity-cache stale calls.
   - Apply the same contract to both background entity mutation publish and the
     pipeline entity batch publish path.

10. Public query and runtime ledger cache lock refactors.
    - `sharedLatestSetCache.AcquireContext()` must not hold its global mutex
      while opening/mmaping/parsing a latest set. Use double-checking,
      per-entry/in-flight state, or another bounded pattern.
    - ASN database cache, per-run latest-set cache, and any related public query
      caches must not hold global cache locks across disk open/parse/close.
    - Runtime ledger per-feed state must not hold `feedLedgerState.mu` across
      `loadHistoryLedgerState()` or `loadRetentionCohorts()`. Use
      release/load/recheck/publish or per-feed in-flight state.
    - Tests:
      - public lookup on one slow-opening feed does not block cached lookup of a
        different feed;
      - concurrent same-feed loads deduplicate or wait context-cancelably;
      - runtime history/retention requests do not hold per-feed locks during disk
        load.

11. Work-lane lifecycle hardening.
    - `syncStart` notifications are collected under `WorkLane.mu`, then sent
      after unlock using non-blocking send semantics. No send to `syncStart`
      may occur while `WorkLane.mu` is held.
    - Worker goroutines recover panics around callback execution and finalization.
    - `finishItem` recovery must guarantee the active slot is removed or marked
      failed, lane degraded state is visible, and later queued work can start.
    - `AttachContext` is idempotent. Duplicate calls are visible through a debug
      log or counter.
    - Tests:
      - shutdown cannot deadlock queued callers waiting on `syncStart`;
      - a forced finalization panic does not wedge later queued work;
      - duplicate `AttachContext` calls do not create duplicate shutdown
        behavior and are observable.

12. Diagnostics sanitizer fixtures.
    - Diagnostic output is capped. Default caps: at most 100 goroutines and at
      most 64 KiB of diagnostic text per event unless tests justify different
      constants.
    - Redact or omit request bodies, credential-like key/value pairs, bearer
      tokens, raw feed snippets, IP addresses, long path lists, and raw stack
      argument values when possible.
    - Path values are truncated to bounded suffixes when they are needed for
      debugging.
    - Tests use fixtures with request bodies, secrets, raw feed snippets, long
      path lists, IP values, and large payloads.

13. Load/liveness validation.
    - Add a focused liveness harness or end-to-end test that simulates:
      - active processing/background work;
      - blocked or slow entity publish;
      - blocked or slow cache persistence;
      - concurrent admin light-status polling;
      - scheduler trigger saturation;
      - reload while status is being polled.
    - Success criteria:
      - `/healthz` responds within 100 ms under the synthetic blockers;
      - `GET /api/v1/admin/status?mode=light` responds within 250 ms under the
        synthetic blockers;
      - watchdog/self-health diagnostics emit without waiting on engine,
        scheduler, entity-publish, or cache locks;
      - no lane slot remains held after panic, cancellation, or trigger
        saturation.

14. Validation and review.
    - Keep all previous validation commands that still apply, plus the V10
      focused tests.
    - Run `make test`, focused package tests, race tests for touched packages,
      and UI build/lint if admin UI polling changes.
    - Re-run the external reviewer set on the implementation diff.
    - Repeat deadlock/stall gap analysis on the new baseline after tests pass.

### Active Plan Note - 2026-06-25

The active implementation plan is `Implementation Plan V10 - 2026-06-25`.
Historical sections above are retained as review evidence. V10 supersedes V9,
V8, V7, V6, V5, V4, V3, V2, and V1 regardless of section ordering in this
regression log.

## External Plan Review Round 10 - 2026-06-25

Review scope:

- Active SOW plan V10.
- Current codebase.
- Whether V10 is precise enough to implement without re-opening design choices.

Reviewers run:

- `glm`: **production-grade plan**, with precision clarifications.
- `minimax`: **not production-grade plan**, due to dropped V8 requirements and
  unresolved choices.
- `mimo`: **not production-grade plan**, but the main stated reason was that
  the code still contains the unfixed defects. That is not a valid reason to
  reject a pre-implementation plan; useful code-evidence findings were retained.
- `kimi`: **production-grade plan**, with precision gaps.
- `qwen`: **production-grade plan with required precision additions**.
- `deepseek`: V10 run did not return a usable final result; rerun is required
  after V11.

Accepted findings to fold into V11:

1. Active-operation progress uses the same `e.mu` contention pattern as
   background tasks.
   - Evidence:
     - `pkg/engine/run_diagnostics.go:279-335` updates active operation state
       under `e.mu`.
   - V11 must move active-operation update state off `e.mu` together with
     background-task state.

2. `handleAction` `RunDue` is the broad snapshot rebuild V10 meant to forbid.
   - Evidence:
     - `pkg/scheduler/actions.go:67` calls
       `BuildSnapshot(..., r.eng.EntriesSnapshot(), ...)` inline on the scheduler
       action drain.
   - V11 must name this call site and require a cached/short-lock due snapshot
     path outside the action-channel critical path.

3. Light status needs a mechanical cached-only scheduler API.
   - Evidence:
     - `pkg/web/admin_status_light.go:26` calls `runner.Snapshot()`.
     - `pkg/scheduler/scheduler.go:153-165` rebuilds a full snapshot when the
       cached snapshot is older than `snapshotReadMaxAge`.
   - V11 must require a cached-only scheduler summary method for the light path.

4. Query cache race semantics need to be explicit after removing open-under-lock.
   - Evidence:
     - `pkg/engine/query_set_cache.go:45-52` currently avoids races by holding
       the global lock through open, but that is the free-path stall source.
   - V11 must require per-key in-flight/singleflight behavior, recheck after
     open, and close/discard of stale or losing opens.

5. `cache.Save` ordering and shutdown semantics need a chosen design.
   - Evidence:
     - `pkg/engine/run.go:87` saves inline before lane slot release today.
     - `pkg/cache/cache.go:307-360` performs JSON encode and filesystem
       temp/write/sync/rename work.
   - V11 must choose the serialized persistence-worker design and define
     coalescing/order/shutdown behavior.

6. `newEngineRunDiagnostics` is another runtime sampler call site.
   - Evidence:
     - `pkg/engine/run.go:81` calls `newEngineRunDiagnostics`.
     - `pkg/engine/run_diagnostics.go:81` captures runtime stats at run start.
   - V11 must include run-start diagnostics in the sampler closure.

7. V10 dropped the typed `run_state` / finalizing state requirement from V8.
   - Evidence:
     - `pkg/engine/run.go:253-279` clears/updates run state and final metrics in
       one broad finalization path.
   - V11 must restore explicit `run_state` values: `idle`, `running`,
     `finalizing`.

8. V10 dropped the explicit HTTP-accepted-work daemon-context requirement.
   - Evidence:
     - `pkg/web/integrity.go:208` passes handler context to
       `QueueEntityArtifactsRebuild`.
     - `pkg/engine/entity_refresh_queue.go:32` uses the supplied context for lane
       admission/work.
   - V11 must state: request context gates admission only; accepted work runs
     under daemon/job context.

9. V10 dropped the concrete self-health observer wake policy.
   - Earlier plans required:
     - observer tick: `max(1s, min(watchdogInterval/4, 15s))`;
     - fire threshold: `max(watchdogInterval+notifyDeadline,
       watchdogInterval*3/2)`.
   - V11 must restore these formulas as named constants/functions.

10. Runtime/proc sampler ownership must be decided.
    - V11 chooses a low-level shared sampler package importable by both engine
      and web. Engine must not import web.

11. Entity artifact publication needs a concrete no-live-I/O-under-lock design.
    - V11 chooses a publish-lease design:
      - `entityArtifactsMu` protects generation state and a publish-in-progress
        lease flag;
      - expensive prepare/walk/stat/compare/rename/git/cache work runs outside
        the mutex;
      - only one publisher owns the lease at a time;
      - after I/O, the publisher re-locks, validates generation/lease, commits
        generation state, and releases the lease;
      - stale or failed publishers clean up and report visibly.
    - This keeps serialization without holding the mutex across filesystem I/O.

12. Existing good local pattern for cache open should be named.
    - Evidence:
      - `pkg/engine/geo_provider_cache.go` checks under lock, opens/parses
        outside lock, and publishes under lock.
    - V11 should require public query/cache refactors to mirror that pattern or
      a per-key in-flight variant.

13. Tests need explicit additions.
    - Add tests for:
      - `run_state=finalizing`;
      - accepted HTTP work surviving request-context cancellation after
        admission;
      - self-health observer cadence/fire thresholds;
      - active-operation updates not taking `e.mu`;
      - `RunDue` not rebuilding broad snapshots inline;
      - light status not triggering stale scheduler snapshot rebuild;
      - same-feed query cache load dedupe/stale discard;
      - blocked cache persistence not blocking later engine-lane work;
      - entity publish lease no filesystem work under `entityArtifactsMu`;
      - finish-item panic recovery;
      - duplicate `AttachContext` idempotency.

Rejected or downgraded Round 10 findings:

- "The code still contains the defect" is not a valid reason to reject the plan
  before implementation. It is only evidence that the plan targets real work.
- Splitting this work into a new SOW remains rejected. The findings are
  overlapping and share lock/admission contracts.
- Making live artifact I/O under `entityArtifactsMu` an explicit fallback is
  rejected. V11 keeps the no-live-I/O-under-lock requirement.
- Moving cache persistence out of the lane is high risk but accepted as the
  long-term-best design because watchdog/web/lane availability is the purpose of
  this SOW.
- The diagnostic caps are useful defaults but should be named constants with
  tests rather than magic numbers.
- Reload in-memory helpers such as synthetic source registration and config
  reconciliation can remain under `e.mu` if proven memory-only and bounded.
  Disk-touching helpers must move outside.

### Implementation Plan V11 - 2026-06-25

Plan V11 supersedes V10, V9, V8, V7, V6, V5, V4, V3, V2, and V1. Historical
plans above remain review evidence only. V11 is the active implementation
contract.

1. Work in phases with focused tests after each phase.
   - Phase 1: watchdog/systemd deadlines, self-health diagnostics, SIGHUP and
     shutdown watcher panic containment.
   - Phase 2: scheduler panic containment, bounded/non-blocking trigger
     admission, bounded `handleAction`, and `RunDue` snapshot refactor.
   - Phase 3: true light admin status, full-status short-lock cleanup,
     background-task and active-operation lock separation, shared runtime/proc
     sampler.
   - Phase 4: reload lock narrowing, config/runtime/cache atomic publication,
     daemon-context reload and HTTP-accepted-work continuations.
   - Phase 5: run-state finalization and serialized cache persistence worker.
   - Phase 6: git subprocess timeout/reaping.
   - Phase 7: entity artifact publish lease and web publish lock narrowing.
   - Phase 8: public query/cache and runtime ledger cache lock refactors.
   - Phase 9: work-lane lifecycle hardening, docs/specs/UI contract, full
     validation, and external implementation review.
   - Each phase must leave tests passing before the next phase starts.

2. Watchdog, systemd notify, and control-goroutine liveness.
   - `pkg/systemd.Notify` and all ready/stopping/status/watchdog messages are
     deadline-bound. If the watchdog interval is active, use the smaller of two
     seconds and half the watchdog interval; otherwise use a two-second lifecycle
     notify deadline.
   - Watchdog notify failures and slow calls are visible through rate-limited
     logs and counters.
   - The watchdog heartbeat goroutine, SIGHUP reload goroutine, and shutdown
     watcher goroutine recover panics, log/counter them, and keep their control
     loops alive where continuing is safe.
   - Self-health observer cadence:
     - tick interval: `max(1s, min(watchdogInterval/4, 15s))`;
     - diagnostic fire threshold:
       `max(watchdogInterval+notifyDeadline, watchdogInterval*3/2)`.
   - Pre-watchdog diagnostics first emit a capped, sanitized goroutine sample
     without taking engine/scheduler locks. Engine-lane, scheduler, and runtime
     snapshots are best-effort and timeout/try-lock bounded.
   - The cadence, threshold, goroutine cap, and byte cap are named constants or
     small functions with tests, not hidden magic numbers.
   - Tests:
     - blocked/unresponsive notify socket returns within the deadline;
     - ready/stopping/status/watchdog payload compatibility is preserved;
     - a panic in SIGHUP reload handling does not permanently disable later
       reload signals;
     - self-health observer cadence/fire threshold uses the formulas above;
     - diagnostics do not block on intentionally blocked engine locks.

3. Scheduler action admission and bounded action handling.
   - `Runner.Run` recovers panics from `handleAction`, logs/counters them, marks
     scheduler degraded in snapshots/admin status, and continues draining
     actions unless explicitly stopped or fatal.
   - Fetch loop, processing loop, and downloader worker panics receive the same
     bounded logging/counter/degraded-state treatment.
   - Replace blocking `TriggerSources()` use from HTTP/admin, startup recovery,
     and engine-lane callbacks with:
     - a context-bounded must-deliver API for startup/internal recovery;
     - a non-blocking/try API for HTTP/admin calls that returns a visible
       saturated result;
     - lane-internal trigger calls that can merge compatible pending name-set
       actions or fail visibly without holding an engine-lane slot.
   - Lane-internal recovery trigger saturation must not silently drop work. It
     logs, increments a counter, marks scheduler/admin status degraded, and
     returns an inspectable error to the lane callback.
   - `handleAction` must not perform disk/network work, broad snapshot rebuilds,
     lane operations, goroutine starts, or indefinite waits.
   - `pkg/scheduler/actions.go:67` `RunDue` must not rebuild from
     `EntriesSnapshot()` inline on the action drain. Move due snapshot building
     to a cached/short-lock path outside the action-channel critical path.
   - Tests:
     - injected `handleAction` panic does not stop later action draining;
     - a full action channel does not block startup recovery, HTTP/admin calls,
       or a lane callback;
     - lane-internal trigger saturation releases the engine-lane slot and marks
       scheduler/admin state degraded;
     - `RunDue` action handling does not block on a deliberately blocked
       `EntriesSnapshot`/reload lock.

4. Admin status and free-path lock mechanics.
   - `StatusSnapshotLight()` and full `statusSnapshot()` copy scalars and
     immutable/current pointer references under a short `e.mu` lock, explicitly
     unlock, and only then call helpers that may take telemetry, lane,
     scheduler, background-task, active-operation, cache, filesystem, or runtime
     sampler locks.
   - Move both background task state and active-operation state off `e.mu` to
     dedicated locks or equivalent atomic state. Progress `Update`/`Add`/`Finish`
     calls must not take `e.mu`.
   - The light admin status path must not call:
     - `buildAdminFeedsWithStatusEntries`;
     - `eng.EntriesSnapshot()` or `EntriesSnapshotWithArtifacts()`;
     - `runner.Snapshot()` when it would rebuild from all entries.
   - Add a scheduler cached-only summary API for light status, for example
     `Runner.CachedSnapshot()` / `Runner.LightSnapshot()`, that never rebuilds
     from all entries. If the cached summary is stale, light status reports stale
     or unknown summary state instead of rebuilding inline.
   - The admin UI polls `GET /api/v1/admin/status?mode=light`. The default
     status route keeps the current full payload for compatibility, but the full
     path must also obey short-lock rules and must not block health/watchdog or
     the light path.
   - Tests:
     - light status responds under a bounded timeout while engine work, scheduler
       triggers, reload, and entity publish are active or blocked;
     - light status does not rebuild a stale full scheduler snapshot;
     - active-operation and background-task updates do not take `e.mu`;
     - default full status and `?mode=full` preserve the external payload shape.

5. Runtime/proc sampler ownership.
   - Implement a low-level shared sampler package importable by both engine and
     web, such as `internal/runtimeinfo` or equivalent. Engine must not import
     web.
   - Web request/status paths consume cached samples only. The sampler publishes
     an initial sample before handlers can need the fallback, or the fallback
     must avoid direct runtime/proc calls.
   - Engine run-start diagnostics, progress diagnostics, and run-end diagnostics
     use cached samples or bounded sampler reads from the shared sampler. This
     includes:
     - `pkg/engine/run.go:81` / `newEngineRunDiagnostics`;
     - `pkg/engine/run_diagnostics.go:81`, `:123`, and `:154`;
     - `pkg/engine/run.go:97` / `logRunDiagnosticSummary`.
   - Static validation must show direct `runtime.ReadMemStats`,
     `runtime.NumGoroutine`, `debug.SetMemoryLimit(-1)`, and `/proc/self` reads
     only in sampler implementation files and tests.

6. Reload and configuration publication.
   - Move `WorkLane.SetLimit()` outside `e.mu`. Collect the new limit while
     reloading, release `e.mu`, then apply it.
   - Disk-touching reload work such as directory creation, entry bootstrap,
     timestamp repair, and legacy failure bootstrap moves out of the `e.mu`
     critical section.
   - Memory-only bounded helpers may remain under `e.mu` if tests/review prove
     they do not touch filesystem, network, lane, telemetry, or cache locks.
   - Audit `refreshCriticalInfrastructureProviderSetID()` separately; if it can
     touch heavy data or locks, move its expensive part outside `e.mu`.
   - Publish `cfg`, `runtime`, geo provider cache, ASN lookup cache, ledger
     cache, and query cache references through atomic pointers or short-lock
     snapshot APIs so readers cannot race reload.
   - `Config()` and `Runtime()` must be data-race free.
   - Add `ReloadContext(ctx)` or equivalent. SIGHUP and runtime callers use the
     daemon context; `Reload()` may remain as a compatibility wrapper using a
     bounded non-request context.
   - Request context gates HTTP/admin admission only. After work is accepted,
     entity artifact rebuild, integrity refresh/reprocess, reload cleanup, and
     other daemon background jobs run under daemon/job context, not request
     context.
   - Tests:
     - `SetLimit()` is not called while `e.mu` is held;
     - reload disk work does not block light status beyond the bounded threshold;
     - accepted HTTP work reaches a terminal visible state after the request
       context is canceled;
     - `go test -race` covers concurrent reload with admin status and public
       serving.

7. Run-state finalization and cache persistence.
   - Add typed status state: `run_state` with at least `idle`, `running`, and
     `finalizing`. Keep legacy boolean `running` compatible, but derive it from
     `run_state` where possible.
   - Run state transitions:
     - `running` starts when a run is admitted;
     - `finalizing` starts when processing has ended but metrics/persistence
       finalization is still in progress;
     - `idle` starts only after final metrics state is published and finalization
       has either accepted cache persistence or reported its failure state.
   - No unbounded disk I/O may hold an engine-lane slot after the admitted run's
     processing work is done.
   - Use a serialized daemon-context persistence worker for `cache.Save`.
     Contract:
     - one worker serializes saves with a channel or mutex;
     - saves are coalesced so the newest accepted state is saved after any
       in-flight save finishes;
     - writes keep existing atomic temp/write/sync/rename behavior;
     - shutdown waits for the last accepted save up to a bounded grace period;
     - pending/failed persistence is visible in status/logs and does not wedge
       watchdog, web, or the engine lane.
   - `logRunDiagnosticSummary` and `markRunEnd` must not hold `e.mu` or the
     engine-lane slot while doing runtime/proc sampling or heavy metrics
     snapshot work.
   - Tests:
     - status exposes `run_state=finalizing` during the finalization window;
     - a blocked or slow cache save does not block a later engine-lane item;
     - save ordering/coalescing preserves the newest accepted state;
     - shutdown reports a pending/failed save without wedging watchdog or web.

8. Git subprocess timeout and reaping.
   - Add context/deadline support to generated artifact git sync.
   - All git subprocesses, including `git add`, `git diff --cached`,
     `git commit`, `git push`, and `git gc --auto`, are bounded.
   - Timed-out/canceled commands kill and reap the process. On Unix, use a
     process group or equivalent when needed so helper children are not orphaned.
   - Timeout policy: use `runtime.push_to_git_timeout` with a documented default
     of ten minutes and validation rejecting negative values. Zero/omitted means
     the default, not unbounded.
   - Timeout/cancellation is visible through operation failure state and releases
     any engine-lane slot.

9. Entity artifact and web publish lock narrowing.
   - Remove any fallback that keeps live filesystem work under
     `entityArtifactsMu`.
   - Use a publish-lease design:
     - `entityArtifactsMu` protects generation state and a publish-in-progress
       lease flag;
     - only one publisher may hold the lease at a time;
     - expensive prepare/walk/stat/compare/open/remove/rename/chmod/chown/chtimes
       work runs outside `entityArtifactsMu` while the lease is owned;
     - after I/O, the publisher re-locks, verifies the lease and expected
       generation, commits generation state, and releases the lease;
     - stale, failed, or canceled publishers clean up staged state and report
       visibly.
   - The mutex must not cover:
     - filesystem walk/stat/compare/open/remove/rename/chmod/chown/chtimes;
     - `web_batch.go` helper work;
     - `syncGeneratedFiles()` or git sync;
     - telemetry observation;
     - `MarkIntegrityCachesStale()`.
   - Apply the same contract to both background entity mutation publish and the
     pipeline entity batch publish path.
   - Same-filesystem atomic rename remains the live-file update primitive. If a
     cross-filesystem rename occurs, fail visibly rather than silently falling
     back to non-atomic copy.
   - Tests:
     - structural/AST or equivalent checks prove the critical-section helper
       contains no `os`, `filepath`, git sync, telemetry, or integrity-cache
       stale calls;
     - behavioral probes prove no filesystem helper runs while
       `entityArtifactsMu` is held;
     - cross-filesystem rename failure is visible and does not widen the lock.

10. Public query and runtime ledger cache lock refactors.
    - Mirror the good local pattern in `geo_provider_cache.go`: check under lock,
      open/parse outside lock, re-lock, recheck, publish or discard.
    - `sharedLatestSetCache.AcquireContext()` must not hold its global mutex
      while opening/mmaping/parsing a latest set.
    - Use per-key in-flight/singleflight state for same-feed loads.
    - After open, re-lock and recheck staleness/reload generation. If another
      goroutine published first, or invalidation/reload happened during open,
      close and discard the losing source.
    - ASN database cache, per-run latest-set cache, and related public query
      caches must not hold global cache locks across disk open/parse/close.
    - Runtime ledger per-feed state must not hold `feedLedgerState.mu` across
      `loadHistoryLedgerState()`, `loadChangesetTail()`, `loadRetentionPast()`,
      or `loadRetentionCohorts()`.
    - Tests:
      - public lookup on one slow-opening feed does not block cached lookup of a
        different feed;
      - concurrent same-feed loads deduplicate or wait context-cancelably;
      - invalidation/reload during open closes/discards the stale source;
      - runtime history/retention requests do not hold per-feed locks during disk
        load.

11. Work-lane lifecycle hardening.
    - `syncStart` notifications are collected under `WorkLane.mu`, then sent
      after unlock using non-blocking send semantics. No send to `syncStart`
      may occur while `WorkLane.mu` is held.
    - Worker goroutines recover panics around callback execution and finalization.
    - `finishItem` recovery must guarantee the active slot is removed or marked
      failed, lane degraded state is visible, and later queued work can start.
    - `AttachContext` is idempotent. Duplicate calls are visible through a debug
      log or counter.
    - Tests:
      - shutdown cannot deadlock queued callers waiting on `syncStart`;
      - a forced finalization panic does not wedge later queued work;
      - duplicate `AttachContext` calls do not create duplicate shutdown
        behavior and are observable.

12. Diagnostics sanitizer fixtures.
    - Diagnostic output caps are named constants with test-overridable values.
      Default targets remain at most 100 goroutines and at most 64 KiB of
      diagnostic text per event unless implementation evidence requires
      different defaults.
    - Redact or omit request bodies, credential-like key/value pairs, bearer
      tokens, raw feed snippets, IP addresses, long path lists, and raw stack
      argument values when possible.
    - Path values are truncated to bounded suffixes when they are needed for
      debugging.
    - Tests use fixtures with request bodies, secrets, raw feed snippets, long
      path lists, IP values, and large payloads.

13. Load/liveness validation.
    - Add a focused liveness harness or end-to-end test that simulates:
      - active processing/background work;
      - blocked or slow entity publish;
      - blocked or slow cache persistence;
      - concurrent admin light-status polling;
      - scheduler trigger saturation;
      - reload while status is being polled.
    - Initial success targets:
      - `/healthz` responds within 100 ms under the synthetic blockers;
      - `GET /api/v1/admin/status?mode=light` responds within 250 ms under the
        synthetic blockers;
      - watchdog/self-health diagnostics emit without waiting on engine,
        scheduler, entity-publish, or cache locks;
      - no lane slot remains held after panic, cancellation, or trigger
        saturation.
    - If local CI proves these timing targets are too strict or too loose, adjust
      them with evidence in this SOW before closing the work.

14. Validation and review.
    - Keep all previous validation commands that still apply, plus the V11
      focused tests.
    - Run `make test`, focused package tests, race tests for touched packages,
      `make test-strict`, and UI build/lint if admin UI polling changes.
    - Re-run the external reviewer set on the implementation diff.
    - Repeat deadlock/stall gap analysis on the new baseline after tests pass.
    - Closure gate: no P0/P1 liveness finding may remain unimplemented or
      accepted as "diagnostics only"; any rejected finding must have file/line
      evidence and rationale in this SOW.

### Active Plan Note - 2026-06-25

The active implementation plan is `Implementation Plan V11 - 2026-06-25`.
Historical sections above are retained as review evidence. V11 supersedes V10,
V9, V8, V7, V6, V5, V4, V3, V2, and V1 regardless of section ordering in this
regression log.

## External Plan Review Round 11 - 2026-06-25

Review scope:

- Active SOW plan V11.
- Current codebase.
- Whether V11 still leaves implementation ambiguity.

Reviewers run:

- `glm`: **production-grade plan**, with three required clarifications.
- `minimax`: **not production-grade as a closure document**; useful findings
  retained, while "not implemented yet" remains rejected as a plan objection.
- `mimo`: **not production-grade plan**, due to precision gaps around concrete
  call sites and tests.
- `deepseek`: **production-grade plan**, with one required context-cancellation
  addition for ledger load functions.
- `qwen`: V11 run did not return a usable final result.
- `kimi`: V11 run timed out after code-reading output and no final verdict.

Accepted findings to fold into V12:

1. Observer metrics are a separate `e.mu` hot path.
   - Evidence:
     - `pkg/engine/run_metrics_state.go:13-62` reads `currentMetrics` under
       `e.mu.RLock()`.
     - `pkg/web/admin.go:267-272` and `:282-287` call
       `ObserveOperation`/`ObserveCounter` on each admin request.
   - V12 must require observer paths to read current run metrics without taking
     `e.mu`.

2. Public request-owned heavy work must propagate request context.
   - Evidence:
     - `pkg/engine/public_series.go:78` calls `buildRetentionData` with
       `context.Background()`.
     - `pkg/engine/query.go:519` calls `buildRetentionData` with
       `context.Background()`.
     - `pkg/engine/integrity_check.go:42` calls integrity work with
       `context.Background()`.
   - V12 must classify public request work separately from daemon work: public
     requests use request context or a request-derived deadline.

3. Runtime ledger load functions need context support.
   - Evidence:
     - `pkg/engine/runtime_ledger_cache.go` load call sites hold locks today,
       and several load helpers do not accept context.
   - V12 must require `loadHistoryLedgerState`, `loadChangesetTail`, and
     `loadRetentionPast` to accept and check context during iteration.

4. Entity refresh continuations need explicit daemon context handling.
   - Evidence:
     - `pkg/engine/entity_refresh_queue.go:338` and `:383` use
       `context.Background()`.
   - V12 must name these continuations directly.

5. Ready, stopping, status, and watchdog notifications all need deadline/error
   behavior.
   - Evidence:
     - `pkg/web/server_run.go:217` calls `systemd.Stopping`.
     - `pkg/web/server_run.go:256` calls `systemd.Ready`.
   - V12 must explicitly include lifecycle notifications, not only watchdog
     heartbeats.

6. Status snapshot helper names need to be explicit.
   - Evidence:
     - `pkg/engine/status_snapshot.go` calls `snapshotRunBatchLocked`,
       `snapshotRunPhasePlanLocked`, `snapshotActiveFeedsLocked`,
       `snapshotActiveOperationsLocked`, and `snapshotBackgroundTasksLocked`
       while holding `e.mu`.
   - V12 must require shallow copy under lock and processing outside lock.

7. `MarkIntegrityCachesStale()` must be named as after-unlock work for both
   publish paths.
   - Evidence:
     - `pkg/engine/entity_artifact_publish.go:131` runs it under
       `entityArtifactsMu`.
     - The pipeline path must follow the same final contract.

8. Work-lane recovery needs exact outcome and location.
   - Evidence:
     - `pkg/engine/work_lane.go:471-481` starts worker goroutines.
     - `pkg/engine/work_lane.go:492-515` finalizes work.
   - V12 must name these boundaries and require degraded state, log/counter, and
     slot release.

9. Shared runtime sampler, cache persistence worker, publish lease, and git
   timeout need concrete API names/defaults.
   - V12 should choose:
     - `internal/runtimeinfo` shared sampler package or equivalent;
     - an engine-owned `cachePersistenceWorker`;
     - an `entityArtifactPublishLease` helper;
     - YAML key `runtime.push_to_git_timeout`, default 600 seconds, negative
       invalid, zero/omitted means default.

10. Pipeline integrity cache scopes and query/cache pointers also need safe
    publication across reload.

11. New goroutines need shutdown-leak tests.
    - Self-health observer, cache persistence worker, and diagnostics emitter
      must stop on daemon shutdown.

Rejected or downgraded Round 11 findings:

- "The code is not implemented yet" remains rejected as a reason to reject the
  plan. This SOW is currently in pre-implementation regression planning.
- Splitting into new SOWs remains rejected. Phases may ship through separate
  commits if needed, but this SOW remains the single active regression fix
  because the contracts overlap.
- "Minimum viable fix" is useful operationally, but it is not sufficient for
  SOW closure. Phase 1-3 may reduce watchdog kills; all V12 phases are required
  before this SOW can close.
- Magic timing thresholds remain acceptable as initial targets only if they are
  named constants/functions and adjusted with evidence if CI proves them flaky.

### Implementation Plan V12 - 2026-06-25

Plan V12 supersedes V11, V10, V9, V8, V7, V6, V5, V4, V3, V2, and V1.
Historical plans above remain review evidence only. V12 is the active
implementation contract.

1. Work in phases with focused tests after each phase.
   - Phase 1: deadline-bound lifecycle/systemd notifications, self-health
     diagnostics, SIGHUP/startup/shutdown/watchdog goroutine panic containment.
   - Phase 2: scheduler panic containment, bounded/non-blocking trigger
     admission, bounded `handleAction`, and `RunDue` snapshot refactor.
   - Phase 3: true light admin status, full-status short-lock cleanup,
     background-task, active-operation, and observer-metrics lock separation,
     shared runtime/proc sampler.
   - Phase 4: reload lock narrowing, config/runtime/cache/integrity-cache safe
     publication, daemon-context continuations, and request-context public work.
   - Phase 5: run-state finalization and serialized cache persistence worker.
   - Phase 6: git subprocess timeout/reaping.
   - Phase 7: entity artifact publish lease and web publish lock narrowing.
   - Phase 8: public query/cache and runtime ledger cache lock/context refactors.
   - Phase 9: work-lane lifecycle hardening, docs/specs/UI contract, full
     validation, and external implementation review.
   - Each phase must leave tests passing before the next phase starts.
   - Phase 1-3 are the minimum operational watchdog-relief subset, but all phases
     are required for SOW closure.

2. Watchdog, systemd notify, and control-goroutine liveness.
   - `pkg/systemd.Notify` and all ready/stopping/status/watchdog messages are
     deadline-bound.
   - Deadline policy:
     - if the watchdog interval is active, use the smaller of two seconds and
       half the watchdog interval;
     - otherwise use a two-second lifecycle notify deadline.
   - On notify deadline/error:
     - log and count the failure with rate limiting;
     - do not permanently stop the heartbeat/control goroutine unless the daemon
       context is closing.
   - Lifecycle callers explicitly covered:
     - `systemd.Ready`;
     - `systemd.Stopping`;
     - `systemd.Status`;
     - `systemd.Watchdog`.
   - The watchdog heartbeat goroutine, startup entity artifact goroutine, SIGHUP
     reload goroutine, and shutdown watcher goroutine recover panics,
     log/counter them, and keep their control loops alive where continuing is
     safe.
   - Self-health observer cadence:
     - tick interval function: `max(1s, min(watchdogInterval/4, 15s))`;
     - diagnostic fire threshold function:
       `max(watchdogInterval+notifyDeadline, watchdogInterval*3/2)`.
   - Pre-watchdog diagnostics first emit a capped, sanitized goroutine sample
     without taking engine/scheduler locks. Engine-lane, scheduler, and runtime
     snapshots are best-effort and timeout/try-lock bounded.
   - The cadence, threshold, goroutine cap, and byte cap are named constants or
     small functions with tests, not hidden magic numbers.
   - Tests:
     - blocked/unresponsive notify socket returns within the deadline;
     - ready/stopping/status/watchdog payload compatibility is preserved;
     - notify deadline does not stop future heartbeat attempts;
     - a panic in SIGHUP reload handling does not permanently disable later
       reload signals;
     - startup entity artifact goroutine panic is logged/counted and does not
       block server startup forever;
     - self-health observer cadence/fire threshold uses the formulas above;
     - diagnostics do not block on intentionally blocked engine locks;
     - self-health observer and diagnostics emitter stop on daemon shutdown.

3. Scheduler action admission and bounded action handling.
   - `Runner.Run` recovers panics from `handleAction`, logs/counters them, marks
     scheduler degraded in snapshots/admin status, and continues draining
     actions unless explicitly stopped or fatal.
   - Fetch loop, processing loop, and downloader worker panics receive the same
     bounded logging/counter/degraded-state treatment.
   - Replace blocking `TriggerSources()` use from HTTP/admin, startup recovery,
     and engine-lane callbacks with:
     - a context-bounded must-deliver API for startup/internal recovery;
     - a non-blocking/try API for HTTP/admin calls that returns a visible
       saturated result;
     - lane-internal trigger calls that can merge compatible pending name-set
       actions or fail visibly without holding an engine-lane slot.
   - Lane-internal recovery trigger saturation must not silently drop work. It
     logs, increments a counter, marks scheduler/admin status degraded, and
     returns an inspectable error to the lane callback.
   - `handleAction` must not perform disk/network work, broad snapshot rebuilds,
     lane operations, goroutine starts, or indefinite waits.
   - Audit all `handleAction` branches:
     - `RunDue` must not rebuild from `EntriesSnapshot()` inline on the action
       drain;
     - recheck/reprocess/default branches must use bounded in-memory state and
       must not start goroutines or bypass lane/downloader admission;
     - `enqueueProviderWave` must remain bounded and must not start work outside
       scheduler queues.
   - Tests:
     - injected `handleAction` panic does not stop later action draining;
     - a full action channel does not block startup recovery, HTTP/admin calls,
       or a lane callback;
     - lane-internal trigger saturation releases the engine-lane slot and marks
       scheduler/admin state degraded;
     - `RunDue` action handling does not block on a deliberately blocked
       `EntriesSnapshot`/reload lock.

4. Admin status, observer metrics, and free-path lock mechanics.
   - `StatusSnapshotLight()` and full `statusSnapshot()` copy scalars and
     immutable/current pointer references under a short `e.mu` lock, explicitly
     unlock, and only then call helpers that may take telemetry, lane,
     scheduler, background-task, active-operation, cache, filesystem, or runtime
     sampler locks.
   - Helpers explicitly covered:
     - `snapshotRunBatchLocked`;
     - `snapshotRunPhasePlanLocked`;
     - `snapshotActiveFeedsLocked`;
     - `snapshotActiveOperationsLocked`;
     - `snapshotBackgroundTasksLocked`;
     - `lifetimeMetricsSnapshot`;
     - `currentMetrics.snapshot`.
   - Map/slice-backed state is shallow-copied under lock and iterated or
     transformed after unlock.
   - Move background task state, active-operation state, and observer current-run
     metrics off `e.mu` to dedicated locks, atomics, or copy-on-write state.
   - Observer APIs such as `observeRunOperation`, `observeRunOperationAggregate`,
     `observeFeedOperation`, and `observeRunCounter` must not take `e.mu` on
     HTTP/admin request paths.
   - The light admin status path must not call:
     - `buildAdminFeedsWithStatusEntries`;
     - `eng.EntriesSnapshot()` or `EntriesSnapshotWithArtifacts()`;
     - `runner.Snapshot()` when it would rebuild from all entries.
   - Add a scheduler cached-only summary API for light status, for example
     `Runner.CachedSnapshot()` / `Runner.LightSnapshot()`, that never rebuilds
     from all entries. If the cached summary is stale, light status reports stale
     or unknown summary state instead of rebuilding inline.
   - The admin UI polls `GET /api/v1/admin/status?mode=light`. The default
     status route keeps the current full payload for compatibility, but the full
     path must also obey short-lock rules and must not block health/watchdog or
     the light path.
   - Tests:
     - light status responds under a bounded timeout while engine work, scheduler
       triggers, reload, and entity publish are active or blocked;
     - light status does not rebuild a stale full scheduler snapshot;
     - active-operation and background-task updates do not take `e.mu`;
     - observer metrics from HTTP/admin request paths do not take `e.mu`;
     - default full status and `?mode=full` preserve the external payload shape.

5. Runtime/proc sampler ownership.
   - Implement a low-level shared sampler package importable by both engine and
     web, named `internal/runtimeinfo` unless implementation discovers a better
     existing local package.
   - Required API shape:
     - `type Sampler`;
     - `func NewSampler(clock ClockLike, opts Options) *Sampler` or an equivalent
       constructor that is testable without sleeping;
     - `func (s *Sampler) Start(ctx context.Context)`;
     - `func (s *Sampler) Snapshot() Snapshot`;
     - `func CaptureOnce() Snapshot` for initialization/tests.
   - The sampler publishes an initial sample before web handlers or engine
     diagnostics can need a fallback.
   - The sampler goroutine recovers panics, logs/counters them, and stops on
     daemon context cancellation.
   - Engine run-start diagnostics, progress diagnostics, and run-end diagnostics
     use cached samples or bounded sampler reads from the shared sampler. This
     includes:
     - `pkg/engine/run.go:81` / `newEngineRunDiagnostics`;
     - `pkg/engine/run_diagnostics.go:81`, `:123`, and `:154`;
     - `pkg/engine/run.go:97` / `logRunDiagnosticSummary`.
   - Static validation must show direct `runtime.ReadMemStats`,
     `runtime.NumGoroutine`, `debug.SetMemoryLimit(-1)`, and `/proc/self` reads
     only in sampler implementation files and tests.
   - Tests:
     - sampler starts with an initial sample;
     - sampler stops on shutdown without goroutine leaks;
     - static grep closure rejects direct runtime/proc reads outside sampler and
       tests.

6. Reload, context ownership, and safe publication.
   - Move `WorkLane.SetLimit()` outside `e.mu`. Collect the new limit while
     reloading, release `e.mu`, then apply it.
   - Disk-touching reload work such as directory creation, entry bootstrap,
     timestamp repair, and legacy failure bootstrap moves out of the `e.mu`
     critical section.
   - Memory-only bounded helpers may remain under `e.mu` if tests/review prove
     they do not touch filesystem, network, lane, telemetry, or cache locks.
   - Audit `refreshCriticalInfrastructureProviderSetID()` separately; if it can
     touch heavy data or locks, move its expensive part outside `e.mu`.
   - Publish the following through atomic pointers or short-lock snapshot APIs so
     readers cannot race reload:
     - `cfg`;
     - `runtime`;
     - geo provider cache;
     - ASN lookup cache;
     - ledger cache;
     - query/latest-set cache references;
     - pipeline integrity cache scope map.
   - Reader call sites explicitly covered include `Config()`, `Runtime()`,
     ASN/geo provider readers, runtime ledger readers, public query readers, and
     integrity cache readers.
   - Add `ReloadContext(ctx)` or equivalent. SIGHUP and runtime callers use the
     daemon context; `Reload()` may remain as a compatibility wrapper using a
     bounded non-request context.
   - Context ownership:
     - request context gates HTTP/admin admission only;
     - after work is accepted, entity artifact rebuild, entity artifact ensure,
       integrity refresh/reprocess, reload cleanup, entity refresh continuations,
       and entity health continuations run under daemon/job context;
     - public request-owned work such as retention/history series, public query
       retention data, and public integrity checks use the request context or a
       request-derived deadline, not `context.Background()`.
   - Tests:
     - `SetLimit()` is not called while `e.mu` is held;
     - reload disk work does not block light status beyond the bounded threshold;
     - accepted HTTP work reaches a terminal visible state after the request
       context is canceled;
     - public request-owned retention/history/integrity work is canceled when
       its request context is canceled;
     - entity refresh continuations and reload cleanup cancel on daemon shutdown;
     - `go test -race` covers concurrent reload with admin status, public
       search/query/history/retention, ASN/geo lookups, and integrity cache
       reads.

7. Run-state finalization and cache persistence.
   - Add typed status state: `run_state` with at least `idle`, `running`, and
     `finalizing`. Keep legacy boolean `running` compatible, but derive it from
     `run_state` where possible.
   - Run state transitions:
     - `running` starts when a run is admitted;
     - `finalizing` starts when processing has ended but metrics/persistence
       finalization is still in progress;
     - status during `finalizing` returns either previous settled metrics or
       explicit `run_state=finalizing`; it must not report a misleading freshly
       settled idle state;
     - `idle` starts only after final metrics state is published and finalization
       has accepted cache persistence or reported its failure state.
   - No unbounded disk I/O may hold an engine-lane slot after the admitted run's
     processing work is done.
   - Use an engine-owned `cachePersistenceWorker` for `cache.Save`.
   - Required worker contract:
     - one worker serializes saves with a channel and internal mutex;
     - API shape includes `Submit(stateSnapshot)`, `Snapshot()`, and `Stop(ctx)`;
     - saves are coalesced so the newest accepted state is saved after any
       in-flight save finishes;
     - writes keep existing atomic temp/write/sync/rename behavior;
     - shutdown waits for the last accepted save up to a bounded grace period;
     - pending/failed persistence is visible in status/logs and does not wedge
       watchdog, web, or the engine lane.
   - Ordering:
     - processing ends;
     - metrics are detached under a short lock and `run_state=finalizing`;
     - the lane slot is released before disk persistence blocks;
     - cache save is accepted by the worker;
     - final metrics and `run_state=idle` are published under a short lock, with
       persistence state visible if the save is pending or failed.
   - `logRunDiagnosticSummary` and `markRunEnd` must not hold `e.mu` or the
     engine-lane slot while doing runtime/proc sampling or heavy metrics
     snapshot work.
   - Tests:
     - status exposes `run_state=finalizing` during the finalization window;
     - a blocked or slow cache save does not block a later engine-lane item;
     - save ordering/coalescing preserves the newest accepted state;
     - shutdown reports a pending/failed save without wedging watchdog or web;
     - cache persistence worker stops on shutdown without goroutine leaks.

8. Git subprocess timeout and reaping.
   - Add context/deadline support to generated artifact git sync.
   - All git subprocesses, including `git add`, `git diff --cached`,
     `git commit`, `git push`, and `git gc --auto`, are bounded.
   - Timed-out/canceled commands kill and reap the process. On Unix, use a
     process group or equivalent when needed so helper children are not orphaned.
   - Runtime config:
     - YAML key: `runtime.push_to_git_timeout`;
     - default: 600 seconds;
     - zero or omitted means default;
     - negative values are invalid.
   - Operators may now see timeout errors for git operations that previously
     appeared as silent hangs. This is intentional and must be documented.
   - Timeout/cancellation is visible through operation failure state and releases
     any engine-lane slot.
   - Tests:
     - a real hung git subprocess is killed and reaped;
     - timeout errors are inspectable;
     - default/zero/negative config handling matches the contract.

9. Entity artifact and web publish lock narrowing.
   - Remove any fallback that keeps live filesystem work under
     `entityArtifactsMu`.
   - Implement an `entityArtifactPublishLease` helper or equivalent.
   - Required lease protocol:
     - acquire under `entityArtifactsMu`: verify expected generation and no
       active publisher, then set `publishInProgress`;
     - perform expensive prepare/walk/stat/compare/open/remove/rename/chmod/
       chown/chtimes outside `entityArtifactsMu` while owning the lease;
     - re-lock to commit: verify the same lease owner and expected generation,
       bump generation, clear `publishInProgress`, then unlock;
     - on stale/cancel/error: clean staged state, clear lease under lock, and
       report visibly.
   - The mutex must not cover:
     - filesystem walk/stat/compare/open/remove/rename/chmod/chown/chtimes;
     - `web_batch.go` helper work;
     - `syncGeneratedFiles()` or git sync;
     - telemetry observation;
     - `MarkIntegrityCachesStale()`.
   - `MarkIntegrityCachesStale()` runs after `entityArtifactsMu` is released in
     both background entity mutation publish and pipeline entity batch publish.
   - Apply the same lease contract to both background entity mutation publish and
     the pipeline entity batch publish path.
   - Same-filesystem atomic rename remains the live-file update primitive. If a
     cross-filesystem rename occurs, fail visibly rather than silently falling
     back to non-atomic copy.
   - Tests:
     - structural/AST or equivalent checks prove the critical-section helper
       contains no `os`, `filepath`, git sync, telemetry, or integrity-cache
       stale calls;
     - behavioral probes prove no filesystem helper runs while
       `entityArtifactsMu` is held;
     - background and pipeline publish paths both move `MarkIntegrityCachesStale`
       after unlock;
     - lease contention between background and pipeline publishers is serialized
       and visible;
     - stale generation after I/O discards staged state and reports stale;
     - cross-filesystem rename failure is visible and does not widen the lock.

10. Public query and runtime ledger cache lock/context refactors.
    - Mirror the good local pattern in `geo_provider_cache.go`: check under lock,
      open/parse outside lock, re-lock, recheck, publish or discard.
    - `sharedLatestSetCache.AcquireContext()` must not hold its global mutex
      while opening/mmaping/parsing a latest set.
    - Use per-key in-flight/singleflight state for same-feed loads.
    - After open, re-lock and recheck staleness/reload generation. If another
      goroutine published first, or invalidation/reload happened during open,
      close and discard the losing source.
    - ASN database cache, per-run latest-set cache, and related public query
      caches must not hold global cache locks across disk open/parse/close.
    - Runtime ledger per-feed state must not hold `feedLedgerState.mu` across
      `loadHistoryLedgerState()`, `loadChangesetTail()`, `loadRetentionPast()`,
      or `loadRetentionCohorts()`.
    - `loadHistoryLedgerState`, `loadChangesetTail`, `loadRetentionPast`, and
      `loadRetentionCohorts` accept `context.Context` and check it during file
      iteration.
    - Tests:
      - public lookup on one slow-opening feed does not block cached lookup of a
        different feed;
      - concurrent same-feed loads deduplicate or wait context-cancelably;
      - invalidation/reload during open closes/discards the stale source;
      - runtime history/retention requests do not hold per-feed locks during disk
        load;
      - ledger/history/retention load stops when context is canceled.

11. Work-lane lifecycle hardening.
    - `syncStart` notifications are collected under `WorkLane.mu`, then sent
      after unlock using non-blocking send semantics. No send to `syncStart`
      may occur while `WorkLane.mu` is held.
    - Worker goroutine boundary:
      - `startAsync` wraps callback execution and `finishItem` finalization with
        panic recovery;
      - callback panic remains reported as callback error;
      - finalization panic marks lane degraded, logs/counters the failure,
        releases or marks the active slot failed, and allows later queued work to
        start.
    - `AttachContext` is idempotent. Duplicate calls are visible through a debug
      log or counter.
    - Tests:
      - shutdown cannot deadlock queued callers waiting on `syncStart`;
      - a forced finalization panic does not wedge later queued work;
      - duplicate `AttachContext` calls do not create duplicate shutdown
        behavior and are observable.

12. Diagnostics sanitizer fixtures.
    - Diagnostic output caps are named constants with test-overridable values.
      Default targets remain at most 100 goroutines and at most 64 KiB of
      diagnostic text per event unless implementation evidence requires
      different defaults.
    - Redact or omit request bodies, credential-like key/value pairs, bearer
      tokens, raw feed snippets, IP addresses, long path lists, and raw stack
      argument values when possible.
    - Path values are truncated to bounded suffixes when they are needed for
      debugging.
    - Tests use fixtures with request bodies, secrets, raw feed snippets, long
      path lists, IP values, and large payloads.

13. Load/liveness validation.
    - Add a focused liveness harness or end-to-end test that simulates:
      - active processing/background work;
      - blocked or slow entity publish;
      - blocked or slow cache persistence;
      - concurrent admin light-status polling;
      - scheduler trigger saturation;
      - reload while status is being polled.
    - Include one combined production-shape test that admits an engine run,
      triggers entity-refresh continuation waves, and polls `/healthz` plus
      `GET /api/v1/admin/status?mode=light` under synthetic blockers.
    - Initial success targets:
      - `/healthz` responds within 100 ms under the synthetic blockers;
      - `GET /api/v1/admin/status?mode=light` responds within 250 ms under the
        synthetic blockers;
      - watchdog/self-health diagnostics emit without waiting on engine,
        scheduler, entity-publish, or cache locks;
      - no lane slot remains held after panic, cancellation, or trigger
        saturation.
    - If local CI proves these timing targets are too strict or too loose, adjust
      them with evidence in this SOW before closing the work.

14. Validation and review.
    - Keep all previous validation commands that still apply, plus the V12
      focused tests.
    - Run `make test`, focused package tests, race tests for touched packages,
      `make test-strict`, and UI build/lint if admin UI polling changes.
    - Re-run the external reviewer set on the implementation diff.
    - Repeat deadlock/stall gap analysis on the new baseline after tests pass.
    - Closure gate: no P0/P1 liveness finding may remain unimplemented or
      accepted as "diagnostics only"; any rejected finding must have file/line
      evidence and rationale in this SOW.

### Active Plan Note - 2026-06-25

The active implementation plan is `Implementation Plan V12 - 2026-06-25`.
Historical sections above are retained as review evidence. V12 supersedes V11,
V10, V9, V8, V7, V6, V5, V4, V3, V2, and V1 regardless of section ordering in
this regression log.

### Implementation Progress - 2026-06-25 - V12 Phase 1

Status: in progress.

Scope completed in this increment:

1. Deadline-bound systemd notifications.
   - `pkg/systemd/notify.go:59` defines the notify deadline policy:
     no watchdog uses two seconds; active watchdog uses the smaller of two
     seconds and half the watchdog heartbeat interval.
   - `pkg/systemd/notify.go:74` sends every `Ready`, `Stopping`, `Status`, and
     `Watchdog` payload through `NotifyWithDeadline`.
   - `pkg/systemd/notify.go:86` bounds the dial with a context deadline and
     `pkg/systemd/notify.go:93` applies the same deadline to the write.

2. Watchdog heartbeat liveness.
   - `pkg/web/server_run.go:248` starts the watchdog loop only when systemd
     watchdog is active and starts the self-health observer with the same
     deadline policy.
   - `pkg/web/server_run.go:277` wraps each watchdog tick with panic recovery,
     updates a heartbeat timestamp, reports notify failures, and continues
     later ticks after an error.
   - `pkg/web/liveness.go:44` and `pkg/web/liveness.go:58` implement the named
     cadence and fire-threshold formulas from V12.
   - `pkg/web/liveness.go:69` emits pre-watchdog diagnostics from a goroutine
     stack sample without taking engine or scheduler locks.
   - `pkg/web/liveness.go:167` caps diagnostic text and `pkg/web/liveness.go:211`
     redacts credential-like tokens and IPv4 addresses.

3. Lifecycle/control goroutine panic containment.
   - `pkg/web/server_run.go:145` isolates startup entity artifact checking in a
     done-channel helper with panic recovery, so shutdown waiting does not hang
     forever after a panic.
   - `pkg/web/server_run.go:226` wraps the shutdown watcher and
     `pkg/web/server_run.go:241` separately wraps the systemd stopping
     notification so HTTP shutdown can continue after a notification panic.
   - `pkg/web/server_run.go:300` wraps ready notification panics.
   - `cmd/update-ipsets/daemon.go:114` moves SIGHUP handling into a recoverable
     loop and `cmd/update-ipsets/daemon.go:126` recovers each reload action so a
     panic in one reload does not permanently disable future reload signals.

4. Failure counters and rate-limited logs.
   - `pkg/web/liveness.go:109` counts systemd notify failures with bounded
     labels and rate-limits repeated logs by notification kind.
   - `pkg/web/liveness.go:149` and `cmd/update-ipsets/daemon.go:140` count
     recovered daemon-control panics.
   - `internal/observability/observability.go:73`,
     `internal/observability/observability.go:76`, and
     `internal/observability/observability.go:111` add the designed metric
     names used by this phase.

Behavioral tests added:

- `pkg/systemd/notify_internal_test.go:12` proves a blocked notify dial returns
  within the configured deadline.
- `pkg/systemd/notify_test.go:55` preserves ready/stopping/status/watchdog
  payload compatibility.
- `pkg/systemd/notify_test.go:118` verifies the deadline policy.
- `pkg/web/run_lifecycle_test.go:298` verifies self-health cadence and
  threshold formulas.
- `pkg/web/run_lifecycle_test.go:316` proves a notify error does not stop later
  watchdog attempts.
- `pkg/web/run_lifecycle_test.go:339` proves startup entity artifact panic
  recovery closes the wait channel.
- `pkg/web/run_lifecycle_test.go:355` proves the self-health observer stops on
  daemon context cancellation.
- `pkg/web/run_lifecycle_test.go:369` verifies diagnostic redaction and cap
  behavior.
- `cmd/update-ipsets/daemon_test.go:39` proves a panic in one SIGHUP reload does
  not prevent a later reload signal from being handled.

Validation run:

- `go test ./pkg/systemd`
- `go test ./pkg/web -run 'Test(Watchdog|RunWatchdog|StartupEntityArtifacts|DelayedPublishStageCleanup|RunServesHTTPS|RunServesSplitAdmin)'`
- `go test ./cmd/update-ipsets -run 'Test(ReloadSignalLoop|CompactCLIArgs)'`
- `go test ./internal/observability ./pkg/systemd ./pkg/web ./cmd/update-ipsets`
- `git diff --check`

Validation result: all commands passed.

Remaining work:

- This increment does not close the production deadlock SOW. V12 Phase 2 through
  Phase 9 remain required, especially scheduler trigger admission, admin-status
  free-path lock mechanics, reload lock narrowing, run-state finalization,
  cache persistence, git subprocess bounding, publish lock narrowing, runtime
  ledger/query cache lock refactors, work-lane lifecycle hardening, full
  validation, external implementation review, and repeated gap analysis on the
  new baseline.

### Implementation Progress - 2026-06-25 - V12 Phase 2

Status: in progress.

Scope completed in this increment:

1. Scheduler action admission is now bounded or explicitly non-blocking.
   - `pkg/scheduler/scheduler.go:123` keeps the compatibility
     `TriggerSources` method, but it now delegates to bounded admission instead
     of an unbounded channel send.
   - `pkg/scheduler/scheduler.go:127` adds `TriggerSourcesWithin` for
     must-deliver internal paths with a caller-selected timeout.
   - `pkg/scheduler/scheduler.go:139` adds context-bound delivery and records
     action admission failure on timeout/saturation.
   - `pkg/scheduler/scheduler.go:158` adds `TryTriggerSources` for HTTP/admin
     paths that must never block.
   - `pkg/scheduler/scheduler.go:175` keeps `TriggerQueuedAction` non-blocking
     and delegates accepted work through `TryTriggerSources`.

2. Scheduler action/loop/worker panic containment is now explicit.
   - `pkg/scheduler/scheduler.go:331` runs the fetch loop through
     `runRecoverableLoop`.
   - `pkg/scheduler/scheduler.go:336` runs the processing loop through
     `runRecoverableLoop`.
   - `pkg/scheduler/scheduler.go:353` drains scheduler actions through
     `handleActionRecovered`.
   - `pkg/scheduler/liveness.go:9` restarts a loop after a recovered panic
     while the daemon context is still active.
   - `pkg/scheduler/liveness.go:38` recovers action handling panics and keeps
     later actions drainable.
   - `pkg/scheduler/download_loop.go:62` recovers downloader worker panics,
     clears active download state, releases deferred work, and wakes the
     download loop.
   - `pkg/scheduler/processing_loop.go:30` recovers processing-batch panics.
   - `pkg/scheduler/processing_loop.go:116` requeues active processing work
     after a recovered processing-batch panic.

3. Scheduler degraded state is now visible.
   - `pkg/scheduler/metrics.go:27` adds recovered panic counts to admin metrics.
   - `pkg/scheduler/metrics.go:28` adds action admission failure counts.
   - `pkg/scheduler/metrics.go:29` adds an explicit degraded flag, reason, and
     timestamp.
   - `pkg/scheduler/liveness.go:47` records recovered panics as degraded
     scheduler state.
   - `pkg/scheduler/liveness.go:60` records bounded action admission failures
     as degraded scheduler state.

4. `RunDue` no longer rebuilds scheduler snapshots inline on the action-drain
   goroutine.
   - `pkg/scheduler/actions.go:65` now wakes the scheduler loops only; the
     fetch loop remains the owner of due-work snapshot rebuilding.

5. Blocking callers were replaced.
   - Startup integrity recovery uses bounded must-deliver admission in
     `pkg/web/server_run.go:110` and `pkg/web/server_run.go:119`.
   - Admin global reprocess uses non-blocking admission and returns conflict on
     saturation in `pkg/web/routes.go:345`.
   - Admin feed recheck/reprocess use non-blocking admission and return conflict
     on saturation in `pkg/web/admin.go:344` and `pkg/web/admin.go:364`.
   - Admin artifact recheck uses non-blocking admission and returns conflict on
     saturation in `pkg/web/admin.go:440`.
   - Pipeline integrity lane callbacks use bounded lane admission and return an
     error on saturation in `pkg/web/integrity.go:462` and
     `pkg/web/integrity.go:472`; this releases the lane slot instead of
     silently blocking or dropping work.

Behavioral tests added:

- `pkg/scheduler/policy_test.go:37` proves context-bound action admission
  returns on caller timeout, records failure, and marks scheduler degraded.
- `pkg/scheduler/policy_test.go:68` proves try-admission is non-blocking when
  the action queue is full.
- `pkg/scheduler/policy_test.go:89` proves recovered action panic marks
  degraded state and a later `RunDue` action can still be handled.
- `pkg/scheduler/policy_test.go:114` proves `RunDue` no longer needs an engine
  snapshot on the action-drain path.
- `pkg/scheduler/policy_test.go:135` proves a downloader worker panic clears
  active queue state and marks degraded state.
- `pkg/scheduler/policy_test.go:156` proves processing-batch panic cleanup
  clears active state, requeues the work, and marks degraded state.

Validation run:

- `go test ./pkg/scheduler -run 'Test(TriggerSourcesContext|TryTriggerSources|HandleActionRecovered|RunDueAction|DownloadWorkerPanic|ProcessingBatchPanic)' -count=1 -v`
- `go test ./pkg/scheduler`
- `go test ./internal/observability ./pkg/scheduler ./pkg/web ./pkg/systemd ./cmd/update-ipsets`
- `git diff --check`

Validation result: all commands passed.

Remaining work:

- This increment does not close the production deadlock SOW. V12 Phase 3 through
  Phase 9 remain required, especially admin-status free-path lock mechanics,
  reload lock narrowing, run-state finalization, cache persistence, git
  subprocess bounding, publish lock narrowing, runtime ledger/query cache lock
  refactors, work-lane lifecycle hardening, full validation, external
  implementation review, and repeated gap analysis on the new baseline.

### Implementation Progress - 2026-06-25 - V12 Phase 3

Status: in progress.

Scope completed in this increment:

1. Light admin status no longer rebuilds full scheduler/cache state.
   - `pkg/web/admin_status_light.go:23` builds light status from
     `runner.ActivitySnapshotLight()` and `runner.CachedSnapshot()`.
   - `pkg/web/admin_status_light.go:39` summarizes feed health from cached
     scheduler items and live queue state, not from the full admin feed-row
     builder.
   - `pkg/scheduler/scheduler.go:215` exposes `CachedSnapshot()` as a
     no-rebuild read for high-frequency admin polling.
   - `pkg/scheduler/scheduler.go:272` exposes `ActivitySnapshotLight()`, which
     uses scheduler-owned queue maps without cache-entry status lookups or
     engine active-feed snapshots.
   - `pkg/scheduler/snapshot_build.go:18` adds cached item fields for entries,
     unique IPs, and never-run state so the heartbeat summary does not need a
     request-time cache walk.

2. Scheduler wake/startup checks avoid full admin snapshot construction.
   - `pkg/scheduler/actions.go:65` keeps `RunDue` as a wake action only.
   - `pkg/scheduler/download_loop.go:19` keeps snapshot rebuilding on the fetch
     loop, where scheduling decisions are owned.
   - `pkg/scheduler/processing_loop.go:29` keeps processing admission in the
     processing loop rather than in HTTP/admin request paths.

3. Engine status hot state was split away from the broad engine mutex.
   - `pkg/engine/engine.go:42` adds separate locks for active feeds, active
     operations, and background tasks.
   - `pkg/engine/active_feeds.go:46` snapshots active feeds through
     `activeFeedsMu`, not `e.mu`.
   - `pkg/engine/background_tasks.go:44` and
     `pkg/engine/background_tasks.go:82` update background task lifecycle state
     through `backgroundTasksMu`, not `e.mu`.
   - `pkg/engine/run_diagnostics.go:301` and
     `pkg/engine/run_diagnostics.go:410` update and snapshot active operations
     through `activeOperationsMu`.
   - `pkg/engine/run.go:249` stores current run metrics in an atomic pointer
     for observer hot paths, and `pkg/engine/run.go:268` clears it at run end.
   - `pkg/engine/run_metrics_state.go:16` records operation/counter/feed
     telemetry without taking the broad engine mutex.

4. Engine status snapshots now use short-lock assembly.
   - `pkg/engine/status_snapshot.go:9` gets lane and integrity-cache state
     before taking `e.mu`.
   - `pkg/engine/status_snapshot.go:16` through
     `pkg/engine/status_snapshot.go:44` copies scalar/run state under a short
     read lock and releases it before reading active feeds, active operations,
     and background tasks.
   - `pkg/engine/status_snapshot.go:85` applies the same ownership split to the
     full status path before collecting current/lifetime metric snapshots.

5. Runtime/process sampling is shared.
   - `internal/runtimeinfo/runtimeinfo.go:58` centralizes runtime memory,
     process memory, process CPU, process I/O, and file-descriptor sampling.
   - `pkg/web/sysinfo.go:137` refreshes admin runtime samples from one sampler
     instead of recomputing them on each light poll.
   - `pkg/web/sysinfo.go:109` returns cached runtime details for light status,
     with only minimal fallback fields when no sample has been collected yet.
   - `pkg/engine/run_diagnostics.go:50` reuses the shared runtime delta model
     for engine progress logs.

Behavioral tests added or extended:

- `pkg/web/admin_status_test.go:233` proves light admin status uses a deliberately
  stale cached scheduler snapshot instead of rebuilding from cache entries.
- `pkg/web/admin_status_test.go:202` preserves the feed-health summary contract
  for the light status payload when cached scheduler state exists.
- `pkg/engine/run_reason_test.go:126` proves observer metric updates do not
  block on `e.mu`.
- `pkg/engine/run_reason_test.go:150` proves active-operation and background
  task progress updates do not block on `e.mu`.
- `pkg/engine/run_reason_test.go:176` proves light status snapshot assembly does
  not hold `e.mu` while waiting for active-operation state.
- `internal/runtimeinfo/runtimeinfo_test.go:9` and
  `internal/runtimeinfo/runtimeinfo_test.go:22` cover runtime/process capture
  shape and unsigned delta clamping.

Validation run:

- `go test ./pkg/web -run 'TestAdminStatusLight|TestAdminStatusKeepsFullDefaultAndLightPollingSnapshot' -count=1 -v`
- `go test ./pkg/scheduler -run 'Test(BuildSnapshot|TriggerSourcesContext|TryTriggerSources|RunDueAction|HandleActionRecovered)' -count=1`
- `go test ./pkg/engine -run 'Test(StatusSnapshot|StatusSnapshotLight|EngineLaneBackgroundTask|RunReason|Active|Background)' -count=1 -v`
- `go test ./pkg/engine -run 'Test(StatusSnapshot|ObserverMetrics|ActiveAndBackground|EngineLaneBackgroundTask)' -count=1 -v`
- `go test ./internal/runtimeinfo ./pkg/engine ./pkg/web ./pkg/scheduler -count=1`
- `go test -race ./pkg/scheduler -run TestScheduledDownloadWithProcessingWorkWakesProcessLoop -count=1 -v`
- `go test -race ./pkg/scheduler -count=1`
- `go test -race ./internal/runtimeinfo ./pkg/engine ./pkg/web ./pkg/scheduler -count=1`

Validation result:

- All commands above passed.
- One earlier combined race run reported a scheduler race involving
  `pkg/scheduler/download_loop.go:75` and `pkg/cache/entry_config.go:104`
  during `TestScheduledDownloadWithProcessingWorkWakesProcessLoop`. It did not
  reproduce in the focused race test, the full scheduler race test, or the
  combined touched-package race test. This is recorded as a residual risk to
  keep visible while later phases continue.

Spec updates made:

- `.agents/sow/specs/admin-ui.md` now states that light status must use cached
  scheduler/feed heartbeat state and must not rebuild the full feed inventory or
  cache-entry snapshot from the HTTP handler.
- `.agents/sow/specs/operating-principles.md` now states that frequently polled
  status paths must not refresh scheduler snapshots, full feed rows, runtime
  process stats, or procfs state inline.
- `.agents/sow/specs/processing-engine.md` now states that active feeds, active
  operations, background tasks, and current metrics must have independent
  ownership from the broad engine state mutex.

Remaining work:

- This increment does not close the production deadlock SOW. V12 Phase 4 through
  Phase 9 remain required: reload lock narrowing, run-state finalization, cache
  persistence worker, git subprocess timeout/reaping, entity artifact publish
  lease, public query/cache/runtime ledger lock and context refactors,
  work-lane lifecycle hardening, full validation, external implementation
  review, and repeated gap analysis on the new baseline.

### Implementation Progress - 2026-06-25 - V12 Phase 4

Status: in progress.

Scope completed in this increment:

1. Reload now has daemon-context ownership.
   - `pkg/engine/engine.go:226` adds `ReloadContext(ctx)` and keeps
     `Reload()` as a compatibility wrapper in `pkg/engine/engine.go:223`.
   - `cmd/update-ipsets/daemon.go:109` requires reload-capable engines to
     expose `ReloadContext(context.Context) error`.
   - `cmd/update-ipsets/daemon.go:128` passes the daemon SIGHUP context into
     reload, and `cmd/update-ipsets/daemon.go:133` keeps the same context for
     the reload entity-integrity ensure submission.

2. Reload lock scope is narrower.
   - `pkg/engine/engine.go:29` adds a dedicated `reloadMu` so reloads serialize
     without using the broad engine state mutex as the long-lived reload owner.
   - `pkg/engine/engine.go:226` checks caller cancellation before starting
     reload work.
   - `pkg/engine/engine.go:269` through `pkg/engine/engine.go:290` swaps config,
     runtime, downloader client, runtime ledger cache, provider caches, and
     retention-window state under a short `e.mu` section.
   - `pkg/engine/engine.go:296` calls `ensureDirectoriesForRuntime` after
     releasing `e.mu`, so directory creation cannot freeze light status behind
     the broad engine mutex.
   - `pkg/engine/engine.go:302` through `pkg/engine/engine.go:313` keep the
     existing cache bootstrap/repair steps synchronous for reload correctness,
     but they now run outside the broad engine mutex.
   - `pkg/engine/engine.go:316` closes retired ASN lookup handles after the
     broad engine mutex is released and before reload cleanup submission.
   - `pkg/engine/engine.go:321` queues reload critical-infrastructure cleanup
     with the reload caller context, not `context.Background()`.

3. Race-sensitive runtime accessors were tightened.
   - `pkg/engine/engine.go:213` locks `SetPushToGit`.
   - `pkg/engine/engine.go:352` reads the lock-file path through `Runtime()`.
   - `pkg/engine/engine.go:356` and `pkg/engine/engine.go:365` make exported
     `Config()` and `Runtime()` readers take a short read lock.
   - `pkg/engine/runtime.go:280` copies effective runtime before creating
     override directories, instead of rereading mutable runtime fields after
     unlocking.

4. Entity refresh continuation resubmission no longer detaches from shutdown.
   - `pkg/engine/entity_refresh_queue.go:262` and
     `pkg/engine/entity_refresh_queue.go:310` pass the active lane context to
     follow-up entity refresh submissions.
   - `pkg/engine/entity_refresh_queue.go:333` and
     `pkg/engine/entity_refresh_queue.go:379` submit continuation work with the
     caller context instead of `context.Background()`.

Behavioral tests added or extended:

- `cmd/update-ipsets/daemon_test.go:41` now proves the SIGHUP reload loop
  survives a reload panic and passes the daemon context to both reload and the
  reload entity-integrity queue submission.
- `pkg/engine/runtime_test.go:268` proves `ReloadContext` honors a canceled
  context before mutating reload state.
- `pkg/engine/runtime_test.go:291` blocks reload at directory creation and
  proves `StatusSnapshotLight()` still returns while the filesystem step is
  paused.
- `pkg/engine/entity_refresh_queue_test.go:138` proves feed-update entity
  refresh continuation does not queue detached work after caller cancellation.
- `pkg/engine/entity_refresh_queue_test.go:160` proves health-transition entity
  refresh continuation follows the same cancellation rule.

Validation run:

- `go test ./cmd/update-ipsets -run 'Test(ReloadSignalLoop|CompactCLIArgs)' -count=1 -v`
- `go test ./pkg/engine -run 'TestReload(Context|Applies|Retires|Stales|CleansCritical)|TestStatusSnapshotReportsEffectiveRuntimeWorkers' -count=1 -v`
- `go test ./pkg/engine -run 'TestEntity.*Continuation|TestQueueEntity.*Refresh|TestEntityArtifactRefreshQueue|TestEntityHealthRefreshQueue' -count=1 -v`
- `go test ./pkg/engine -run 'Test(ReloadContext|ReloadApplies|ReloadRetires|ReloadStales|ApplyRuntimeOverrides|StatusSnapshotReportsEffectiveRuntimeWorkers)' -count=1 -v`
- `go test ./internal/runtimeinfo ./pkg/engine ./pkg/web ./pkg/scheduler ./pkg/systemd ./cmd/update-ipsets -count=1`
- `go test -race ./pkg/engine -run 'TestReload(Context|Applies|Retires|Stales|CleansCritical)|TestEntity.*Continuation|TestQueueEntity.*Refresh|TestStatusSnapshot|TestObserverMetrics|TestActiveAndBackground' -count=1`
- `go test -race ./cmd/update-ipsets -run 'Test(ReloadSignalLoop|CompactCLIArgs)' -count=1`
- `go test -race ./internal/runtimeinfo ./pkg/engine ./pkg/web ./pkg/scheduler ./cmd/update-ipsets -count=1`
- `git diff --check`

Validation result: all commands passed.

Spec updates made:

- `.agents/sow/specs/operating-principles.md` now states the reload-specific
  daemon-context and broad-engine-mutex rules.

Remaining work:

- This increment does not close the production deadlock SOW. V12 Phase 5 through
  Phase 9 remain required: run-state finalization, cache persistence worker,
  git subprocess timeout/reaping, entity artifact publish lease, public
  query/cache/runtime ledger lock and context refactors, work-lane lifecycle
  hardening, full validation, external implementation review, and repeated gap
  analysis on the new baseline.

### Implementation Progress - 2026-06-25 - V12 Phase 5

Status: in progress.

Scope completed in this increment:

1. Run state is now typed.
   - `pkg/engine/types.go:42` defines `RunState` values `idle`, `running`, and
     `finalizing`.
   - `pkg/engine/status_snapshot.go:18` and
     `pkg/engine/status_snapshot.go:98` derive legacy `running` from the typed
     state, so existing clients stay compatible while admin/status clients can
     read `run_state`.
   - `pkg/engine/types.go:146` and `pkg/engine/types.go:191` expose
     `run_state` in full and light engine status snapshots.

2. Run finalization no longer holds the engine lane for diagnostic logging or
   cache disk I/O.
   - `pkg/engine/run.go:37` calls `completeRunFinalization` only after
     `engineLane.Run` returns, so the lane slot has already been released.
   - `pkg/engine/run.go:118` takes a detached cache snapshot before run state is
     released.
   - `pkg/engine/run.go:336` moves the in-memory run state to `finalizing`,
     detaches final metrics, clears active-feed/operation state, and publishes
     final report fields under short locks.
   - `pkg/engine/run.go:282` logs the final run summary and diagnostic snapshot
     outside the lane.

3. Cache persistence uses a serialized worker with detached state.
   - `pkg/cache/cache.go:499` adds `SnapshotState()` so async persistence saves
     a stable state copy rather than the live mutable cache.
   - `pkg/engine/cache_persistence.go:40` creates the worker.
   - `pkg/engine/cache_persistence.go:50` implements `Submit(snapshot)`.
   - `pkg/engine/cache_persistence.go:71` implements `Snapshot()`.
   - `pkg/engine/cache_persistence.go:116` implements bounded `Wait(ctx, seq)`.
   - `pkg/engine/cache_persistence.go:138` implements `Stop(ctx)`.
   - `pkg/engine/cache_persistence.go:148` serializes saves and coalesces newer
     pending snapshots while an older save is in flight.
   - `pkg/engine/cache_persistence_engine.go:10` through
     `pkg/engine/cache_persistence_engine.go:40` add engine-owned submit and
     status helpers.
   - `pkg/engine/types.go:50` and `pkg/engine/types.go:60` define the
     cache-persistence status contract exposed as `cache_persistence`.

4. Daemon and direct-run durability semantics are separated.
   - `pkg/engine/types.go:19` adds `AsyncCachePersistence` to `RunOptions`.
   - `pkg/scheduler/processing_loop.go:55` enables async persistence for daemon
     scheduler runs, so the engine lane remains available while cache I/O is
     pending or saving.
   - `pkg/engine/run.go:316` waits outside the engine lane for direct one-shot
     runs and then stops the worker, avoiding temp-dir/test cleanup races and
     avoiding one-shot process exit before durable cache state is written.
   - `pkg/web/server_run.go:44` stops cache persistence during daemon shutdown
     with a bounded 30-second context.

5. Admin UI and API types expose the new state.
   - `ui/src/lib/admin-api-types.ts:386` and
     `ui/src/lib/admin-api-types.ts:416` add `run_state` and
     `cache_persistence` to the admin status type.
   - `ui/src/components/admin/heartbeat.tsx:64` shows `FINALIZING` and cache
     saving state in the daemon heartbeat tile.
   - `ui/src/components/admin/current-run.tsx:97` and
     `ui/src/components/admin/current-run.tsx:160` include cache persistence in
     the background-work surface.

Behavioral tests added:

- `pkg/cache/cache_test.go:65` proves `SnapshotState()` is detached from later
  mutations.
- `pkg/engine/cache_persistence_test.go:14` proves status exposes
  `run_state=finalizing` and that engine-lane work can run while finalization is
  paused outside the lane.
- `pkg/engine/cache_persistence_test.go:84` proves a blocked cache save does
  not block `RunOnce` or later engine-lane work for daemon async runs.
- `pkg/engine/cache_persistence_test.go:145` proves the worker saves the newest
  accepted snapshot after an in-flight save, coalescing stale intermediate
  snapshots.

Validation run:

- `go test ./pkg/cache -run TestStateSnapshotIsDetachedFromLaterMutations -count=1`
- `go test ./pkg/engine -run 'Test(RunFinalizationReleasesLaneBeforeCachePersistenceSubmit|BlockedCachePersistenceDoesNotBlockRunOrEngineLane|CachePersistenceWorkerCoalescesNewestAcceptedSnapshot)' -count=1 -v`
- `go test ./pkg/cache ./pkg/engine ./pkg/web ./pkg/scheduler ./cmd/update-ipsets -count=1`
- `pnpm --dir ui lint`
- `pnpm --dir ui test -- --run current-run heartbeat`
- `go test -race ./pkg/engine -run 'Test(RunFinalizationReleasesLaneBeforeCachePersistenceSubmit|BlockedCachePersistenceDoesNotBlockRunOrEngineLane|CachePersistenceWorkerCoalescesNewestAcceptedSnapshot|StatusSnapshotLightOmitsMetricsButKeepsLiveProgress)' -count=1`
- `go test -race ./pkg/cache ./pkg/engine ./pkg/web ./pkg/scheduler ./cmd/update-ipsets -count=1`

Validation result: all commands passed.

Spec updates made:

- `.agents/sow/specs/operating-principles.md` now defines the typed run-state
  contract, cache-persistence worker contract, direct-run durability behavior,
  and admin status visibility requirements.

Remaining work:

- This increment does not close the production deadlock SOW. V12 Phase 6 through
  Phase 9 remain required: git subprocess timeout/reaping, entity artifact
  publish lease, public query/cache/runtime ledger lock and context refactors,
  work-lane lifecycle hardening, full validation, external implementation
  review, and repeated gap analysis on the new baseline.

### Implementation Progress - 2026-06-25 - V12 Phase 6

Status: in progress.

Scope completed in this increment:

1. Generated artifact Git sync is now context-bound and timeout-bound.
   - `pkg/output/sync.go:26` adds a `Timeout` field to `SyncOptions`.
   - `pkg/output/sync.go:35` defines the default timeout as 10 minutes.
   - `pkg/output/sync.go:176` adds `SyncGitContext`.
   - `pkg/output/sync.go:213` through `pkg/output/sync.go:238` routes `git add`,
     `git diff --cached`, `git commit`, and `git push` through context-bound
     execution.
   - `pkg/output/sync.go:245` routes `git gc --auto` through the same bounded
     execution path.
   - `pkg/output/sync.go:346` runs git subprocesses with `exec.CommandContext`,
     a custom cancellation hook, and a `WaitDelay` so canceled children are
     reaped instead of silently blocking forever.
   - `pkg/output/git_exec_unix.go:12` starts git in a separate process group
     on Unix, and `pkg/output/git_exec_unix.go:16` kills that process group on
     timeout/cancellation.
   - `pkg/output/git_exec_default.go:10` keeps a non-Unix direct-process
     fallback.

2. Engine publication passes the run context and runtime timeout into git sync.
   - `pkg/engine/metadata.go:15` changes generated-file sync to accept
     `context.Context`.
   - `pkg/engine/metadata.go:16` through `pkg/engine/metadata.go:49` passes
     `e.runtime.PushToGitTimeout` into base-dir and web-dir Git sync.
   - `pkg/engine/run_pipeline.go:423` passes the run context from normal
     publication.
   - `pkg/engine/entity_artifact_publish.go:127` passes the entity artifact
     publish context from entity artifact publication. This call is still under
     `entityArtifactsMu`; V12 Phase 7/9 remain responsible for narrowing that
     lock.

3. Runtime config now has the documented timeout knob.
   - `pkg/config/config.go:129` adds YAML key `runtime.push_to_git_timeout`.
   - `pkg/config/config.go:607` sets the config default to 600 seconds.
   - `pkg/config/validate.go:361` rejects negative authored values.
   - `pkg/engine/runtime.go:49` stores the resolved runtime value as a
     `time.Duration`.
   - `pkg/engine/runtime.go:174` converts YAML seconds to duration.
   - `pkg/engine/runtime.go:239` through `pkg/engine/runtime.go:252` preserve
     zero/omitted as the 600-second default.
   - `configs/firehol/runtime.yaml` now documents and sets
     `push_to_git_timeout: 600` in the shipped runtime catalog.

Behavioral tests added or extended:

- `pkg/output/sync_test.go:318` proves a hung fake git command returns
  `context.DeadlineExceeded`.
- `pkg/output/sync_test.go:330` through `pkg/output/sync_test.go:347` prove the
  fake git helper child is gone after timeout, catching orphaned subprocesses.
- `pkg/output/sync_test.go:358` proves direct negative `SyncOptions.Timeout`
  is rejected.
- `pkg/config/runtime_controls_test.go:70` and
  `pkg/config/runtime_controls_test.go:113` prove config validation rejects
  negative `push_to_git_timeout` and accepts zero/default semantics.
- `pkg/engine/runtime_test.go` now proves runtime resolution maps omitted/zero
  timeout to 600 seconds and explicit positive values to that many seconds.

Validation run:

- `go test ./pkg/output -run 'TestSyncGitContextTimesOutAndReapsHungGitChildren|TestSyncGitCommitsAndPushes|TestSyncGitIgnoresFilesOutsideRepository' -count=1 -v`
- `go test ./pkg/config -run 'TestValidateRejectsNegativeRuntimeResourceControls|TestValidateAcceptsZeroRuntimeResourceControls' -count=1 -v`
- `go test ./pkg/engine -run 'TestResolveRuntimeDefaultsBackgroundWorkersToOne' -count=1 -v`
- `go test ./pkg/output ./pkg/config ./pkg/engine -count=1`
- `go test -race ./pkg/output -run 'TestSyncGitContextTimesOutAndReapsHungGitChildren|TestSyncGitContextRejectsNegativeTimeout' -count=1`
- `git diff --check`

Validation result: all commands passed.

Spec updates made:

- `.agents/sow/specs/config.md` now documents `runtime.push_to_git_timeout`,
  the default, zero/omitted behavior, and negative-value rejection.
- `.agents/sow/specs/files-layout.md` now states generated Git publication sync
  is bounded and cannot hold the engine lane indefinitely.
- `.agents/sow/specs/operating-principles.md` now includes Git sync in the
  bounded cancellation contract.
- `.agents/sow/specs/pipeline.md` now states Git sync is optional, bounded, and
  must not corrupt or roll back local publication when it times out.

Remaining work:

- This increment does not close the production deadlock SOW. V12 Phase 7
  through Phase 9 remain required: entity artifact publish lease, public
  query/cache/runtime ledger lock and context refactors, work-lane lifecycle
  hardening, full validation, external implementation review, and repeated gap
  analysis on the new baseline.

### Implementation Progress - 2026-06-25 - V12 Phase 7

Status: in progress.

Scope completed in this increment:

1. Entity artifact generation state and live publication serialization are now
   separate locks.
   - `pkg/engine/engine.go:58` adds `entityArtifactPublishMu`.
   - `pkg/engine/entity_artifact_publish.go:42` keeps
     `entityArtifactsMu` scoped to the generation snapshot.
   - `pkg/engine/entity_artifact_publish.go:82` introduces
     `acquireEntityArtifactPublishLease`.
   - `pkg/engine/entity_artifact_publish.go:88` serializes live publication
     with `entityArtifactPublishMu`.
   - `pkg/engine/entity_artifact_publish.go:94` through
     `pkg/engine/entity_artifact_publish.go:96` revalidates the generation
     under the short generation mutex, then releases it before live filesystem
     work.
   - `pkg/engine/entity_artifact_publish.go:107` through
     `pkg/engine/entity_artifact_publish.go:119` bumps generation under the
     short mutex and releases the publish lease.

2. Optimistic entity refresh/repair publish no longer holds
   `entityArtifactsMu` during slow work.
   - `pkg/engine/entity_artifact_publish.go:149` through
     `pkg/engine/entity_artifact_publish.go:190` publish private entity
     artifacts, public entity artifacts, and bounded Git sync under the publish
     lease rather than the generation mutex.
   - The existing stale-generation retry behavior remains in
     `pkg/engine/entity_artifact_publish.go:122` through
     `pkg/engine/entity_artifact_publish.go:146`.

3. Normal processing-run entity sidecar publication uses the same lease model.
   - `pkg/engine/entity_artifacts.go:16` records the expected entity artifact
     generation on staged entity publish batches.
   - `pkg/engine/run_pipeline.go:321` captures the expected generation before
     staging feed entity sidecars.
   - `pkg/engine/run_pipeline.go:406` revalidates that generation before
     normal run publication can publish entity sidecars.
   - `pkg/engine/run_pipeline.go:408` and `pkg/engine/run_pipeline.go:409`
     publish and release the lease at the same point where the old broad mutex
     used to be released.

Behavioral tests added or extended:

- `pkg/engine/entity_artifact_publish_test.go:50` proves a second optimistic
  entity mutation can reach its staging function while a first entity artifact
  publish is paused after acquiring the publish lease.
- `pkg/engine/entity_artifact_publish_test.go:10` continues to prove stale
  generation detection discards and restages old entity artifact batches.

Validation run:

- `go test ./pkg/engine -run 'Test(OptimisticEntityArtifactMutationRestagesAfterGenerationChange|EntityArtifactPublishDoesNotBlockLaterStaging)' -count=1 -v`
- `go test ./pkg/engine -run 'Test(EntityArtifact|OptimisticEntityArtifact|Pipeline|RunFinalization|StatusSnapshotLight)' -count=1`
- `go test -race ./pkg/engine -run 'Test(OptimisticEntityArtifactMutationRestagesAfterGenerationChange|EntityArtifactPublishDoesNotBlockLaterStaging|RunFinalizationReleasesLaneBeforeCachePersistenceSubmit|BlockedCachePersistenceDoesNotBlockRunOrEngineLane)' -count=1`
- `go test ./pkg/cache ./pkg/output ./pkg/config ./pkg/engine ./pkg/web ./pkg/scheduler ./cmd/update-ipsets -count=1`
- `git diff --check`

Validation result: all commands passed.

Spec updates made:

- No new spec text was required in this increment. Existing
  `.agents/sow/specs/pipeline.md` and
  `.agents/sow/specs/operating-principles.md` already require staging outside
  the serialized entity-artifact publish lock and generation revalidation before
  publish.

Remaining work:

- This increment does not close the production deadlock SOW. V12 Phase 8 and
  Phase 9 remain required: public query/cache/runtime ledger lock and context
  refactors, work-lane lifecycle hardening, full validation, external
  implementation review, and repeated gap analysis on the new baseline.

### Implementation Progress - 2026-06-25 - V12 Phase 8

Status: in progress.

Scope completed in this increment:

1. Public IP lookup latest-set cache no longer holds its global mutex while
   opening, mmaping, or parsing local latest-set artifacts.
   - `pkg/engine/query_set_cache.go:58` through
     `pkg/engine/query_set_cache.go:77` check cached state and install per-feed
     in-flight state under the mutex, then release it before file work.
   - `pkg/engine/query_set_cache.go:79` through
     `pkg/engine/query_set_cache.go:103` perform the open outside the mutex,
     handle post-open cancellation, wake same-feed waiters, and discard/retry
     sources opened across a cache invalidation generation.
   - `pkg/engine/query_set_cache.go:112` through
     `pkg/engine/query_set_cache.go:125` re-lock only to publish the winning
     cache entry.

2. Heavy-phase per-run latest-set cache now uses the same lock-split pattern.
   - `pkg/engine/latest_set_cache.go:83` through
     `pkg/engine/latest_set_cache.go:108` check cached state and per-feed
     in-flight state under the cache mutex, then release it before file work.
   - `pkg/engine/latest_set_cache.go:110` through
     `pkg/engine/latest_set_cache.go:145` open outside the mutex, honor
     post-open cancellation, discard losing duplicate opens, and avoid
     publishing into a closed cache.
   - `pkg/engine/latest_set_cache.go:248` through
     `pkg/engine/latest_set_cache.go:264` make cache teardown wake in-flight
     waiters and mark the cache closed.

3. Runtime history, changeset, and retention loaders now accept context-aware
   paths and stop on cancellation checkpoints.
   - `pkg/engine/runtime_ledger_loaders.go:75` through
     `pkg/engine/runtime_ledger_loaders.go:123` add context checks to CSV
     iteration.
   - `pkg/engine/runtime_ledger_loaders.go:129` through
     `pkg/engine/runtime_ledger_loaders.go:179` add context-aware history ledger
     loading.
   - `pkg/engine/runtime_ledger_loaders.go:280` through
     `pkg/engine/runtime_ledger_loaders.go:310` add context-aware removed-life
     retention loading.
   - `pkg/engine/runtime_ledger_loaders.go:327` through
     `pkg/engine/runtime_ledger_loaders.go:424` add context-aware retention
     cohort index/fallback loading.
   - `pkg/engine/runtime_ledger_tail.go:99` through
     `pkg/engine/runtime_ledger_tail.go:120` add context-aware changeset tail
     loading.

4. Runtime ledger per-feed locks no longer cover disk reads.
   - `pkg/engine/runtime_ledger_cache.go:186` through
     `pkg/engine/runtime_ledger_cache.go:218` check history cache state under
     lock, load outside the lock, then publish under the lock.
   - `pkg/engine/runtime_ledger_cache.go:236` through
     `pkg/engine/runtime_ledger_cache.go:286` preserve same-timestamp
     correction behavior while moving ledger reload outside the lock.
   - `pkg/engine/runtime_ledger_cache.go:306` through
     `pkg/engine/runtime_ledger_cache.go:386` apply the same pattern to
     changeset and removed-life retention runtime state.
   - `pkg/engine/runtime_ledger_cache.go:424` through
     `pkg/engine/runtime_ledger_cache.go:456` apply the same pattern to current
     retention cohorts.

Behavioral tests added or extended:

- `pkg/engine/query_set_cache_test.go:34` proves a slow open for one public
  lookup feed does not block a cached lookup for a different feed.
- `pkg/engine/query_set_cache_test.go:109` proves same-feed public lookup
  waiters unblock on request context cancellation.
- `pkg/engine/query_set_cache_test.go:181` proves invalidation during an
  in-flight open causes a retry rather than publishing a stale generation.
- `pkg/engine/heavy_phase_cache_test.go:137` proves the per-run heavy-phase
  latest-set cache does not block unrelated cached feeds behind a slow open.
- `pkg/engine/runtime_ledger_cache_test.go:506` proves a history ledger load
  does not hold the per-feed runtime ledger lock while disk work is blocked.
- `pkg/engine/runtime_ledger_cache_test.go:551` proves the context-aware
  history, changeset, retention-past, and retention-cohort loaders stop on
  cancellation.

Validation run:

- `go test ./pkg/engine -run 'TestSharedLatestSetCache|TestLatestSetCache(SlowOpenDoesNotBlockDifferentCachedSet|ReusesOpenSets|ReusesSummaries|OverlapFilterDoesNotBuildSummary|DoesNotReuseTextFallbackSets)|TestRuntimeLedger(HistoryLoadDoesNotHoldFeedLock|LoadersHonorCancelledContext)|TestHistoryLedgerCacheAppliesAndObserves|TestObserveHistoryPoint|TestChangesetTailFromRuntime' -count=1`
- `go test -race ./pkg/engine -run 'TestSharedLatestSetCache|TestLatestSetCacheSlowOpenDoesNotBlockDifferentCachedSet|TestRuntimeLedger(HistoryLoadDoesNotHoldFeedLock|LoadersHonorCancelledContext)' -count=1`
- `go test ./pkg/engine -count=1`

Validation result: all commands passed.

Spec updates made:

- `.agents/sow/specs/operating-principles.md` now states local file/index
  caches must not hold global cache mutexes while opening, mmaping, parsing,
  closing, or hashing local artifacts, and same-key loads should use
  context-cancellable in-flight wait state.
- `.agents/sow/specs/processing-engine.md` now states runtime history,
  changeset, and retention cache locks protect in-memory feed state only and
  must not be held during ledger file work.

Remaining work:

- This increment does not close the production deadlock SOW. V12 Phase 9
  through Phase 13 remain required: work-lane lifecycle hardening, diagnostics
  sanitizer fixtures, combined load/liveness validation, full validation,
  external implementation review, and repeated gap analysis on the new
  baseline.

### Implementation Progress - 2026-06-25 - V12 Phase 9

Status: in progress.

Scope completed in this increment:

1. Engine-lane start/shutdown notifications no longer happen while holding
   `WorkLane.mu`.
   - `pkg/engine/work_lane.go:128` through `pkg/engine/work_lane.go:131`
     define explicit start notifications.
   - `pkg/engine/work_lane.go:178` through `pkg/engine/work_lane.go:181`,
     `pkg/engine/work_lane.go:236` through `pkg/engine/work_lane.go:240`, and
     `pkg/engine/work_lane.go:290` through `pkg/engine/work_lane.go:294`
     collect notifications under the mutex, unlock, then notify/start work.
   - `pkg/engine/work_lane.go:326` through `pkg/engine/work_lane.go:363`
     applies the same pattern to shutdown.
   - `pkg/engine/work_lane.go:531` through `pkg/engine/work_lane.go:540`
     sends notifications with non-blocking semantics.

2. Work-lane service context attachment is idempotent.
   - `pkg/engine/work_lane.go:116` through `pkg/engine/work_lane.go:118`
     track the attached context and duplicate count.
   - `pkg/engine/work_lane.go:376` through `pkg/engine/work_lane.go:389`
     rejects duplicate attachments, records an observable counter, and avoids
     starting a duplicate shutdown goroutine.

3. Work-lane finalization panic is contained and does not permanently consume a
   lane slot.
   - `pkg/engine/work_lane.go:543` through `pkg/engine/work_lane.go:550`
     wraps finalization in panic recovery.
   - `pkg/engine/work_lane.go:553` through `pkg/engine/work_lane.go:580`
     marks the affected work failed, removes active/coalescing ownership,
     closes idle safely, reschedules later work, and records a finalization
     panic counter.
   - `pkg/engine/work_lane.go:612` through `pkg/engine/work_lane.go:618`
     makes idle-channel close idempotent through explicit open-state tracking.

Behavioral tests added or extended:

- `pkg/engine/work_lane_test.go:522` proves shutdown cannot block on a queued
  synchronous caller whose start channel is already full.
- `pkg/engine/work_lane_test.go:552` proves a finalization panic returns
  `ErrLanePanic` and releases the slot for later work.
- `pkg/engine/work_lane_test.go:578` proves duplicate `AttachContext` calls are
  counted, duplicate context cancellation does not shut down the lane, and the
  primary attached context still owns shutdown.

Validation run:

- `go test ./pkg/engine -run 'TestWorkLane(ShutdownDoesNotBlockOnFullQueuedSyncStart|FinishPanicReleasesSlotForLaterWork|AttachContextIsIdempotent)' -count=1 -v`
- `go test ./pkg/engine -run 'TestWorkLane' -count=1`
- `go test -race ./pkg/engine -run 'TestWorkLane' -count=1`
- `go test ./pkg/engine -count=1`

Validation result: all commands passed.

Spec updates made:

- `.agents/sow/specs/pipeline.md` now states engine-lane start/shutdown
  notifications must happen outside the lane mutex, callback/finalization
  panics must be contained, and duplicate context attachment must not create
  duplicate shutdown owners.
- `.agents/sow/specs/operating-principles.md` now states the engine lane must
  stay live under shutdown and panic paths.

Remaining work:

- This increment does not close the production deadlock SOW. V12 Phase 10
  through Phase 13 remain required: diagnostics sanitizer fixtures, combined
  load/liveness validation, full validation, external implementation review,
  and repeated gap analysis on the new baseline.

### Implementation Progress - 2026-06-25 - V12 Phase 10

Status: in progress.

Scope completed in this increment:

1. Watchdog/daemon diagnostics now use named, test-overridable caps.
   - `pkg/web/liveness.go:25` through `pkg/web/liveness.go:29` define named
     watchdog goroutine-count, watchdog byte, and daemon-panic byte caps.
   - `pkg/web/liveness.go:174` through `pkg/web/liveness.go:184` applies the
     caps to runtime goroutine samples.

2. Diagnostic text sanitization covers the sensitive classes identified in the
   production-deadlock SOW.
   - `pkg/web/liveness.go:38` through `pkg/web/liveness.go:42` define
     credential, JSON credential, bearer token, IP, and long-path matchers.
   - `pkg/web/liveness.go:209` through `pkg/web/liveness.go:228` redacts those
     fields and caps output.
   - `pkg/web/liveness.go:230` through `pkg/web/liveness.go:239` keeps only a
     bounded suffix for long paths needed for debugging.

3. CLI daemon panic recovery no longer logs a raw `debug.Stack()` payload.
   - `cmd/update-ipsets/daemon.go:20` defines a named daemon-control panic
     diagnostic cap.
   - `cmd/update-ipsets/daemon.go:149` through `cmd/update-ipsets/daemon.go:152`
     sanitize and cap the stack text before logging it.

Behavioral tests added or extended:

- `pkg/web/run_lifecycle_test.go:369` through
  `pkg/web/run_lifecycle_test.go:405` now proves diagnostics redact bearer
  auth, cookies, key/value secrets, JSON secrets, request-body payloads, raw IP
  values/feed snippets, and full long paths while preserving a bounded path
  suffix and enforcing byte caps.

Validation run:

- `go test ./pkg/web -run 'TestWatchdogDiagnosticSanitizesAndCaps|TestWatchdogSelfHealth|TestDaemonControl|TestStartupEntityArtifacts' -count=1 -v`
- `go test ./cmd/update-ipsets -run 'Test' -count=1`
- `go test ./pkg/web -count=1`

Validation result: all commands passed.

Spec updates made:

- `.agents/sow/specs/operating-principles.md` now states watchdog and
  daemon-control panic diagnostics must be bounded and sanitized, including
  credentials, bearer tokens, cookies, request bodies, payloads, raw IPs, raw
  feed snippets, and long path lists.

Remaining work:

- This increment does not close the production deadlock SOW. V12 Phase 11
  through Phase 13 remain required: combined load/liveness validation, full
  validation, external implementation review, and repeated gap analysis on the
  new baseline.

### Implementation Progress - 2026-06-25 - V12 Phase 11

Status: in progress.

Scope completed in this increment:

1. Added a full `Run` server liveness regression test for the exact free-lane
   contract.
   - `pkg/web/run_lifecycle_test.go:191` through
     `pkg/web/run_lifecycle_test.go:214` start the real daemon HTTP server over
     TCP with admin auth disabled only for the test fixture, then wait for
     `/healthz`.
   - `pkg/web/run_lifecycle_test.go:216` through
     `pkg/web/run_lifecycle_test.go:241` starts a real `Engine.RunOnce` through
     the public engine-lane admission path and blocks it at the exported
     `RunOptions.BeforePublish` boundary after real feed processing.
   - `pkg/web/run_lifecycle_test.go:249` through
     `pkg/web/run_lifecycle_test.go:270` proves `/healthz` and
     `/api/v1/admin/status?mode=light` return while the engine run is still
     occupying the lane, and proves the returned light status reports active
     engine work.
   - `pkg/web/run_lifecycle_test.go:272` through
     `pkg/web/run_lifecycle_test.go:290` releases the blocked run and waits for
     both the engine run and daemon server to shut down cleanly.

2. Added focused local test helpers for bounded HTTP assertions and a minimal
   staged-feed engine fixture.
   - `pkg/web/run_lifecycle_test.go:609` through
     `pkg/web/run_lifecycle_test.go:631` use a per-request context deadline so
     the regression fails as a timeout if serving is wedged.
   - `pkg/web/run_lifecycle_test.go:643` through
     `pkg/web/run_lifecycle_test.go:687` create a one-feed engine with a staged
     canonical body, avoiding live network dependencies while still exercising
     the normal processing and publication path.

Validation run:

- `go test ./pkg/web -run 'TestRunServesHealthAndLightStatusWhileEngineRunBlocked' -count=1 -v`
- `go test ./pkg/web -run 'TestRunServes(HealthAndLightStatusWhileEngineRunBlocked|HTTPS|SplitAdminOnSeparateListeners)|TestAdminStatusLightRespondsWhileEngineLaneBusy|TestRunWatchdog|TestWatchdogSelfHealth|TestStartupEntityArtifacts' -count=1`
- `go test -race ./pkg/web -run 'TestRunServesHealthAndLightStatusWhileEngineRunBlocked|TestAdminStatusLightRespondsWhileEngineLaneBusy' -count=1`

Validation result: all commands passed.

Remaining work:

- This increment does not close the production deadlock SOW. V12 Phase 12 and
  Phase 13 remain required: full changed-surface validation, external
  implementation review, and repeated gap analysis on the new baseline.

### Implementation Progress - 2026-06-25 - V12 Phase 12

Status: in progress.

Scope completed in this increment:

1. Ran the changed-surface validation set after Phase 11.

2. The architecture posture gate initially failed because test and production
   files grew past their existing large-file baselines. This was fixed by
   splitting code instead of updating the architecture baseline.
   - `pkg/cache/cache_snapshot_test.go:8` through
     `pkg/cache/cache_snapshot_test.go:37` now hold the detached-cache-snapshot
     regression test outside the already-large `pkg/cache/cache_test.go`.
   - `pkg/engine/runtime_ledger_loaders.go:1` through
     `pkg/engine/runtime_ledger_loaders.go:440` now hold runtime ledger CSV,
     history, retention, and cohort loader helpers outside
     `pkg/engine/runtime_ledger_cache.go`, leaving the original cache file
     focused on state and lock coordination.
   - No architecture baseline update was needed or made.

Validation run:

- `go test ./pkg/web -count=1`
- `go test ./pkg/engine -count=1`
- `go test ./pkg/scheduler -count=1`
- `go test ./pkg/output ./pkg/config ./pkg/cache ./pkg/systemd ./internal/observability ./internal/runtimeinfo ./cmd/update-ipsets -count=1`
- `go test -race ./pkg/engine -run 'TestWorkLane|TestSharedLatestSetCache|TestLatestSetCacheSlowOpenDoesNotBlockDifferentCachedSet|TestRuntimeLedger(HistoryLoadDoesNotHoldFeedLock|LoadersHonorCancelledContext)|TestBlockedCachePersistenceDoesNotBlockRunOrEngineLane|TestEntityArtifactPublishLease|TestRunFinalizationReleasesLaneBeforeCachePersistenceSubmit' -count=1`
- `go test -race ./pkg/web -run 'TestRunServesHealthAndLightStatusWhileEngineRunBlocked|TestAdminStatusLightRespondsWhileEngineLaneBusy|TestRunWatchdog|TestStartupEntityArtifacts' -count=1`
- `go test ./tools/archposture -count=1`

Validation result:

- All package and race commands passed.
- `go test ./tools/archposture -count=1` passed after the file split above.

Remaining work:

- This increment does not close the production deadlock SOW. V12 Phase 13
  remains required: external implementation review and repeated gap analysis on
  the new baseline.

### Implementation Progress - 2026-06-25 - V12 Phase 13 Round 1

Status: in progress.

External implementation review run:

- Requested reviewers: `glm`, `minimax`, `mimo`, `kimi`, `deepseek`, `qwen`.
- Five reviewers produced final verdicts. Four marked the implementation
  production-grade with non-blocking concerns. `glm` marked it not
  production-grade because it identified one direct correctness bug and one
  verified stall amplifier. The first `kimi` run collected context but did not
  produce a final verdict before exit, so it is treated as incomplete and must
  be rerun after this fix round.

Accepted blocking finding 1: worker-submitted continuations inherited the
current lane item context.

- Evidence before fix: `pkg/engine/entity_refresh_queue.go` submitted entity
  refresh and health continuations with the active lane callback context. In
  `pkg/engine/work_lane.go`, queued items derive their active context from the
  submitted parent context, and `startAsync()` cancels the completing item
  immediately after `finishItemSafely()`. A continuation queued from the active
  worker could therefore start with a context that is canceled by the previous
  worker's completion.
- Impact: this matched the production symptom class. A large entity refresh can
  hit the bounded-wave limit, queue a follow-up item, then abandon remaining
  pending feeds because the follow-up observes `context.Canceled` before it
  drains the pending set. This is not a Go runtime deadlock, but it is a real
  queue-liveness bug in the engine lane.
- Fix: `pkg/engine/work_lane.go:116` through `pkg/engine/work_lane.go:119`
  now stores the attached daemon/root context on the lane. `pkg/engine/work_lane.go:212`
  through `pkg/engine/work_lane.go:227` still rejects already-canceled caller
  contexts, then chooses a safe parent context for the queued item.
  `pkg/engine/work_lane.go:380` through `pkg/engine/work_lane.go:412`
  attaches the daemon context and makes worker-submitted follow-up work use the
  attached daemon context, or `context.Background()` in tests/direct use when no
  daemon context is attached. This preserves daemon shutdown cancellation while
  preventing cancellation by the finishing worker.
- Tests: `pkg/engine/work_lane_test.go:382` through
  `pkg/engine/work_lane_test.go:421` proves a worker-submitted queued item does
  not inherit cancellation from the completing worker. `pkg/engine/work_lane_test.go:423`
  through `pkg/engine/work_lane_test.go:470` proves the same queued item is
  still canceled by the attached daemon/root context. Existing canceled-context
  tests for entity continuations still pass, so external canceled submissions
  remain rejected.

Accepted blocking finding 2: engine progress/summary diagnostics still captured
runtime/process stats inline.

- Evidence before fix: `pkg/engine/run_diagnostics.go` called the engine runtime
  capture helper at run start, progress log time, and diagnostic summary time.
  The helper called `internal/runtimeinfo.Capture()`, which uses
  `runtime.ReadMemStats()` and reads `/proc/self/status`, `/proc/self/io`, and
  `/proc/self/fd`.
- Impact: this was a verified stall amplifier under DigitalOcean wait-I/O. It
  was not proven to be the deadlock root cause, but it violated the SOW's
  liveness rule that progress/status paths should read cached samples rather
  than performing process-wide runtime/proc scans inline.
- Fix: `pkg/engine/engine.go:54` through `pkg/engine/engine.go:57` now stores
  cached engine runtime stats. `pkg/engine/work_lane.go:415` through
  `pkg/engine/work_lane.go:420` starts the sampler when the daemon lane context
  is attached. `pkg/engine/run_diagnostics.go:114` through
  `pkg/engine/run_diagnostics.go:123`, `pkg/engine/run_diagnostics.go:158`
  through `pkg/engine/run_diagnostics.go:183`, and
  `pkg/engine/run_diagnostics.go:194` through `pkg/engine/run_diagnostics.go:220`
  now read the cached sample. `pkg/engine/run_diagnostics.go:461` through
  `pkg/engine/run_diagnostics.go:486` confines `runtimeinfo.Capture()` to the
  sampler refresh path and returns a cheap minimal sample before the sampler has
  populated.
- Tests: `pkg/engine/run_diagnostics_test.go:83` through
  `pkg/engine/run_diagnostics_test.go:100` proves run diagnostics use the cached
  runtime sample.

Reviewer findings assessed as non-blocking for this fix round:

- Cache persistence worker recreation after stop: valid low-risk shutdown
  hardening idea, but not connected to the observed silence/deadlock. Shutdown
  already waits for background work then calls `StopCachePersistence()` with a
  bounded context. Leave as non-blocking unless a future shutdown leak is
  observed.
- `observeHistoryPoint()` background CSV load: valid cancellation improvement,
  but the current runtime-ledger loaders already have context-aware variants
  and this is not connected to the production stall signature.
- `/healthz` and light-status free-path structural tests: useful hardening, but
  Phase 11 added a black-box `Run` server liveness test that proves `/healthz`
  and `?mode=light` respond while a real engine run occupies the lane.
- DroneBL-specific ordering tests: useful follow-up coverage, but reviewer
  inspection agreed the staged recovery path routes through the download lane
  and does not acquire the engine lane. This SOW's regression target is the
  engine/web/watchdog stall; DroneBL was explicitly included in lane ownership
  and is already covered by the scheduler policy tests added earlier.

Validation run after Round 1 fixes:

- `go test ./pkg/engine -run 'TestWorkLaneSubmitFromWorker(QueuesWithoutDeadlock|DetachesQueuedContextFromCompletingWorker|UsesAttachedContextForContinuationShutdown)|TestEntityRefreshContinuationUsesCallerContext|TestEntityHealthContinuationUsesCallerContext|TestRunDiagnosticsUseCachedRuntimeStats|TestRunDiagnosticSummaryIncludes' -count=1 -v`
- `go test -race ./pkg/engine -run 'TestWorkLaneSubmitFromWorker(QueuesWithoutDeadlock|DetachesQueuedContextFromCompletingWorker|UsesAttachedContextForContinuationShutdown)|TestRunDiagnosticsUseCachedRuntimeStats' -count=1 -v`
- `go test ./pkg/engine -count=1`
- `go test ./pkg/web -count=1`
- `go test ./pkg/scheduler -count=1`
- `go test ./pkg/output ./pkg/config ./pkg/cache ./pkg/systemd ./internal/observability ./internal/runtimeinfo ./cmd/update-ipsets -count=1`
- `go test -race ./pkg/engine -run 'TestWorkLane|TestSharedLatestSetCache|TestLatestSetCacheSlowOpenDoesNotBlockDifferentCachedSet|TestRuntimeLedger(HistoryLoadDoesNotHoldFeedLock|LoadersHonorCancelledContext)|TestBlockedCachePersistenceDoesNotBlockRunOrEngineLane|TestEntityArtifactPublishLease|TestRunFinalizationReleasesLaneBeforeCachePersistenceSubmit|TestRunDiagnosticsUseCachedRuntimeStats' -count=1`
- `go test -race ./pkg/web -run 'TestRunServesHealthAndLightStatusWhileEngineRunBlocked|TestAdminStatusLightRespondsWhileEngineLaneBusy|TestRunWatchdog|TestStartupEntityArtifacts' -count=1`
- `go test ./tools/archposture -count=1`

Validation result: all commands passed.

Remaining work:

- Run changed-surface validation again after this fix round.
- Rerun the external reviewers on the updated baseline, including `kimi`, and
  keep this SOW open until the repeated review loop has no blocking findings.

### Implementation Progress - 2026-06-25 - V12 Phase 13 Round 2 Follow-Up

Status: in progress.

External review status:

- `qwen`, `deepseek`, and `glm` completed Round 2 with
  `PRODUCTION GRADE` verdicts.
- `glm` identified a non-blocking lock-hold cleanup: the optimistic entity
  publish path called `MarkIntegrityCachesStale()` before the deferred entity
  publish lease release. This was not a deadlock because integrity cache locks
  do not acquire the entity publish mutex, but it unnecessarily kept the
  serialization mutex held during stale marking.

Cleanup implemented:

- `pkg/engine/entity_artifact_publish.go:187` now explicitly releases the
  entity artifact publish lease before stale marking. The existing deferred
  release remains as an idempotent safety net for success and error paths.
- `pkg/engine/entity_artifact_publish.go:188` through
  `pkg/engine/entity_artifact_publish.go:190` now mark integrity caches stale
  after the lease has been released.

Behavioral test added:

- `pkg/engine/entity_artifact_publish_test.go:110` through
  `pkg/engine/entity_artifact_publish_test.go:168` blocks the integrity cache
  stale-marking path and proves the entity publish mutex is already available
  while stale marking is blocked.

Validation run:

- `go test ./pkg/engine -run 'TestEntityArtifactPublish(DoesNotBlockLaterStaging|MarksIntegrityStaleAfterReleasingPublishLease)|TestOptimisticEntityArtifactMutationRestagesAfterGenerationChange' -count=1 -v`
- `go test -race ./pkg/engine -run 'TestEntityArtifactPublish(DoesNotBlockLaterStaging|MarksIntegrityStaleAfterReleasingPublishLease)' -count=1 -v`

Validation result: all commands passed.

Remaining work:

- Wait for the remaining Round 2 external reviewers.
- Re-run changed-surface validation after this follow-up cleanup.

### Implementation Progress - 2026-06-25 - V12 Phase 13 Round 3 Kimi Blocker Fixes

Status: in progress.

External review status:

- The first Round 2 `kimi` run timed out after collecting context but before a
  verdict.
- The second Round 2 `kimi` run completed and returned `NOT PRODUCTION GRADE`.
  The two blocking findings were verified against the code and accepted.

Accepted blocking finding 1: background entity artifact publish still held the
entity artifact publish lease while running generated-file git sync.

- Evidence before fix: `pkg/engine/entity_artifact_publish.go:184` called
  `syncGeneratedFiles()` before `lease.release(mutatesLive)`. The function can
  run `output.SyncGitContext()` and therefore git commit/push/network I/O.
- Impact: this kept `entityArtifactPublishMu` held across slow I/O in the
  background entity mutation path. The pipeline publish path already released
  the same lease before git sync; the background path had not been brought into
  that safer lock shape.
- Fix: `pkg/engine/entity_artifact_publish.go:184` now releases the entity
  artifact publish lease before calling `syncGeneratedFiles()`. The existing
  deferred release remains as an idempotent error-path safety net.
- Test: `pkg/engine/entity_artifact_publish_test.go:170` through
  `pkg/engine/entity_artifact_publish_test.go:228` blocks generated-file sync
  and proves `entityArtifactPublishMu` is already available while sync is
  blocked.

Accepted blocking finding 2: ad-hoc ASN lookup cache opened provider databases
while holding the cache mutex.

- Evidence before fix: `pkg/engine/ip_context.go:98` acquired
  `asnDatabaseCache.mu` and `pkg/engine/ip_context.go:106` called the provider
  `open()` callback while still holding that mutex.
- Impact: a slow ASN database open could block unrelated public IP lookups that
  need ASN attribution, even for different providers.
- Fix: `pkg/engine/ip_context.go:109` through
  `pkg/engine/ip_context.go:177` now uses an in-flight load record keyed by
  provider, path, and file fingerprint. The provider `open()` call runs after
  the cache mutex is released. Concurrent same-key callers wait for the in-flight
  load and reuse the result, while unrelated provider opens can proceed. Cache
  retirement increments a generation value; a database opened before retirement
  is closed and retried instead of being republished after `retireAll()`.
- Tests: `pkg/engine/ip_context_test.go:124` through
  `pkg/engine/ip_context_test.go:174` proves an independent provider acquire is
  not blocked by another provider's slow open.
  `pkg/engine/ip_context_test.go:176` through
  `pkg/engine/ip_context_test.go:238` proves concurrent same-provider/same-key
  acquires deduplicate the cold open.

Validation run after Round 3 fixes:

- `go test ./pkg/engine -run 'TestEntityArtifactPublish(SyncsGeneratedFilesAfterReleasingPublishLease|MarksIntegrityStaleAfterReleasingPublishLease|DoesNotBlockLaterStaging)|TestASNDatabaseCache(OpenDoesNotBlockIndependentProvider|DeduplicatesConcurrentSameProviderOpen|KeepsExistingEntryWhenReplacementOpenFails|SurvivesConcurrentAcquireAndRetire)' -count=1 -v`
- `go test -race ./pkg/engine -run 'TestEntityArtifactPublish(SyncsGeneratedFilesAfterReleasingPublishLease|MarksIntegrityStaleAfterReleasingPublishLease|DoesNotBlockLaterStaging)|TestASNDatabaseCache(OpenDoesNotBlockIndependentProvider|DeduplicatesConcurrentSameProviderOpen|KeepsExistingEntryWhenReplacementOpenFails|SurvivesConcurrentAcquireAndRetire)' -count=1 -v`
- `go test ./pkg/engine -count=1`
- `go test ./pkg/web -count=1`
- `go test ./pkg/scheduler -count=1`
- `go test ./pkg/output ./pkg/config ./pkg/cache ./pkg/systemd ./internal/observability ./internal/runtimeinfo ./cmd/update-ipsets -count=1`
- `go test -race ./pkg/engine -run 'TestWorkLane|TestSharedLatestSetCache|TestLatestSetCacheSlowOpenDoesNotBlockDifferentCachedSet|TestRuntimeLedger(HistoryLoadDoesNotHoldFeedLock|LoadersHonorCancelledContext)|TestBlockedCachePersistenceDoesNotBlockRunOrEngineLane|TestEntityArtifactPublish(Lease|MarksIntegrityStaleAfterReleasingPublishLease|SyncsGeneratedFilesAfterReleasingPublishLease)|TestASNDatabaseCache(OpenDoesNotBlockIndependentProvider|DeduplicatesConcurrentSameProviderOpen|SurvivesConcurrentAcquireAndRetire)|TestRunFinalizationReleasesLaneBeforeCachePersistenceSubmit|TestRunDiagnosticsUseCachedRuntimeStats' -count=1`
- `go test -race ./pkg/web -run 'TestRunServesHealthAndLightStatusWhileEngineRunBlocked|TestAdminStatusLightRespondsWhileEngineLaneBusy|TestRunWatchdog|TestStartupEntityArtifacts' -count=1`
- `go test ./tools/archposture -count=1`

Validation result: all commands passed.

Remaining work:

- Rerun the external reviewer loop on the updated baseline. The SOW remains
  open until the repeated review loop has no blocking findings.

### Implementation Progress - 2026-06-25 - V12 Phase 13 Round 4 Reload Snapshot Fixes

Status: in progress.

External review status:

- Round 3 follow-up reviewers `qwen` and `minimax` marked the implementation
  `PRODUCTION GRADE`.
- Round 3 `glm` marked the core deadlock/stall fixes production-grade but
  identified one remaining correctness gap: reload swaps `runtime`, `cfg`,
  `geoProviders`, `asnLookupCache`, and `ledgerCache` under `e.mu`, while some
  public/admin reader paths still read those fields directly.
- Round 3 `deepseek` and `mimo` returned `NOT PRODUCTION GRADE` verdicts, but
  the blocking claims were checked against the current code and rejected:
  `ReloadContext()` does not hold `e.mu` across directory creation, full status
  does not hold `e.mu` while collecting telemetry snapshots, run finalization
  happens after the lane callback returns, and entity artifact lease release is
  idempotent.
- Round 3 `kimi` returned a conditional pass with one shutdown/cache persistence
  concern. The proposed timeout-wrapper fix was rejected because a goroutine
  blocked in filesystem sync cannot be canceled safely by wrapping it in another
  goroutine; hiding that stuck write would risk concurrent cache writes. The
  bounded cache persistence worker already remains visible and bounded by
  shutdown waiting.

Accepted blocking finding: reload-published runtime/config/cache pointers were
not consistently read through synchronized snapshots.

- Evidence before fix: public lookup and synthetic provider paths read
  `e.cfg`, `e.runtime`, `e.geoProviders`, and `e.asnLookupCache` directly while
  `ReloadContext()` can replace those fields. Runtime ledger helpers read
  `e.runtime` and `e.ledgerCache` directly. Integrity status normalized
  `WebDir` from `e.runtime` directly. Critical-infrastructure reload cleanup
  read `e.cfg` and `e.runtime` directly while later reloads could be active.
- Impact: this was not the original production silence root cause by itself,
  but it is a real reload/read data race class. A data race in these paths can
  produce undefined behavior in Go and can also create apparent stalls if a
  status or cleanup path self-deadlocks around `e.mu`.
- Fix:
  - `pkg/engine/ip_context.go:295` now snapshots config, runtime, geo cache, and
    ASN cache under one short `e.mu.RLock()` before public IP lookup uses them.
  - `pkg/engine/runtime_ledger_cache.go:32` now snapshots runtime ledger state
    before history, changeset, and retention readers load files or update tails.
  - `pkg/engine/integrity_cache.go:315` normalizes pipeline integrity options
    through the synchronized runtime getter instead of reading `e.runtime`
    directly.
  - `pkg/engine/critical.go:679` and `pkg/engine/critical.go:740` now run stale
    critical-infrastructure cleanup from captured config/runtime values instead
    of repeatedly reading live engine fields during filesystem work.
  - `pkg/engine/public.go:455` and `pkg/engine/web_batch.go:57` provide runtime
    snapshot helpers for output directory and web publish batch creation.
  - `pkg/engine/public.go:499`, `pkg/engine/query.go:547`, and
    `pkg/engine/query.go:552` make entry snapshots use captured config.
  - `pkg/engine/status_snapshot.go` now uses `mergeCountForConfig()` while
    already holding `e.mu`, avoiding a nested `Config()` read-lock that can
    self-deadlock when a reload writer is waiting.
  - `pkg/engine/engine.go:382` adds `configRuntimeSnapshot()` for callers that
    need config and runtime from the same publication point.
- Test: `pkg/engine/runtime_test.go:395` adds
  `TestReloadConcurrentPublicRuntimeReadersRace`. The first `-race` run failed
  and exposed `normalizeIntegrityOptions()`, critical-infrastructure cleanup,
  and nested status snapshot lock hazards. After the fixes above, the same test
  passes under `-race`.
- Architecture cleanup: provider-selection helpers were moved out of
  `pkg/engine/insights.go` into `pkg/engine/provider_selection.go` so the
  existing architecture posture line-count guard did not need a baseline update.

Validation run after Round 4 fixes:

- `go test ./pkg/engine -run 'TestReloadConcurrentPublicRuntimeReadersRace' -count=1 -v`
- `go test -race ./pkg/engine -run 'TestReloadConcurrentPublicRuntimeReadersRace' -count=1 -v`
- `go test ./pkg/engine -run 'TestReloadConcurrentPublicRuntimeReadersRace|TestProviderDefaults|TestStatusSnapshotReportsEffectiveRuntimeWorkers' -count=1 -v`
- `go test -race ./pkg/engine -run 'TestReloadConcurrentPublicRuntimeReadersRace|TestWorkLane|TestSharedLatestSetCache|TestLatestSetCacheSlowOpenDoesNotBlockDifferentCachedSet|TestRuntimeLedger(HistoryLoadDoesNotHoldFeedLock|LoadersHonorCancelledContext)|TestBlockedCachePersistenceDoesNotBlockRunOrEngineLane|TestEntityArtifactPublish(Lease|MarksIntegrityStaleAfterReleasingPublishLease|SyncsGeneratedFilesAfterReleasingPublishLease)|TestASNDatabaseCache(OpenDoesNotBlockIndependentProvider|DeduplicatesConcurrentSameProviderOpen|SurvivesConcurrentAcquireAndRetire)|TestRunFinalizationReleasesLaneBeforeCachePersistenceSubmit|TestRunDiagnosticsUseCachedRuntimeStats' -count=1`
- `go test -race ./pkg/web -run 'TestRunServesHealthAndLightStatusWhileEngineRunBlocked|TestAdminStatusLightRespondsWhileEngineLaneBusy|TestRunWatchdog|TestStartupEntityArtifacts' -count=1`
- `go test ./pkg/engine -count=1`
- `go test ./pkg/web ./pkg/scheduler -count=1`
- `go test ./pkg/output ./pkg/config ./pkg/cache ./pkg/systemd ./internal/observability ./internal/runtimeinfo ./cmd/update-ipsets ./tools/archposture -count=1`
- `git diff --check`

Validation result: all commands passed.

Remaining work:

- Rerun the external reviewer loop on the updated baseline and verify that the
  new reload snapshot fixes did not introduce new lock-order or lifecycle
  regressions.

### Implementation Progress - 2026-06-25 - V12 Phase 13 Round 5 Reload Self-Deadlock Fixes

Status: in progress.

Production-observation update:

- The production symptom reported by the user remains best classified as
  no-progress silence, not CPU overload. The 2026-06-24 windows had idle CPU,
  missing logs, and watchdog recovery.
- The Round 4 reload-snapshot work was directionally correct but incomplete.
  The widened race test reproduced a concrete no-progress class that matches the
  production symptom better than the earlier generic starvation hypothesis.

Verified root cause 1: reload self-deadlocked on `e.mu`.

- Evidence before fix:
  - `pkg/engine/engine.go:277` through `pkg/engine/engine.go:299` publish the
    new config/runtime under `e.mu.Lock()`.
  - `pkg/engine/engine.go:297` called
    `reconcileEntriesFromSourceConfig()` while the write lock was still held.
  - `reconcileEntriesFromSourceConfig()` called source/artifact seeding, and
    source seeding used `finalPath()`.
  - `finalPath()` called `Runtime()`, which attempted `e.mu.RLock()` while the
    same goroutine already held `e.mu.Lock()`.
  - The forced test-time goroutine dump showed the reload goroutine blocked in
    `Runtime()` from `finalPath()` while all public readers were blocked behind
    the same `RWMutex`.
- Impact:
  - This is a real Go deadlock. It does not require high CPU, and it can leave
    the process silent while HTTP/admin readers wait behind the lock.
  - Go did not report a fatal global deadlock because the whole process was not
    necessarily in the narrow runtime-detectable "all goroutines asleep"
    condition. This confirms the user's point: Go programs can deadlock in
    ordinary production service patterns.
- Fix:
  - `pkg/engine/bootstrap_entries.go:64` snapshots the already-held runtime once
    inside the locked reload section.
  - `pkg/engine/bootstrap_entries.go:77` and
    `pkg/engine/bootstrap_entries.go:91` use runtime-parameterized seeding
    helpers instead of lock-taking accessors.
  - `pkg/engine/bootstrap_entries.go:166` and
    `pkg/engine/bootstrap_entries.go:215` add runtime-parameterized source
    seeding and current-set stats helpers.
  - `pkg/engine/bootstrap_entries.go:281` makes the critical-content hash path
    use the same captured runtime, so critical feeds cannot keep a hidden
    reload self-deadlock through `currentSetStats()`.

Verified root cause 2: query file loading read mutable cache entries while
reload updated them.

- Evidence before fix:
  - After the self-deadlock fix, `go test -race` reported a data race between
    `cache.Entry.ApplySourceConfig()` and `loadTextSetWithRuntime()`.
  - `loadTextSetWithRuntime()` used the mutable `State.Entry()` pointer for a
    read-only query path.
- Impact:
  - Public query/comparison readers could observe in-place cache metadata writes
    during reload. In Go this is undefined behavior and can corrupt the
    assumptions used by public serving paths.
- Fix:
  - `pkg/engine/fileset_helpers.go:163` through
    `pkg/engine/fileset_helpers.go:171` now uses `EntrySnapshot()` for the
    read-only file metadata consumed by text set loading.

Accepted external-review findings fixed in this round:

- Admin status default must be cheap/light. `pkg/web/admin.go:262` now returns
  full status only for `mode=full`; default and `mode=light` use
  `buildAdminStatusLight()`.
- The admin-status contract test was updated at
  `pkg/web/admin_status_test.go:162` to prove default/light omit full-only
  fields while `mode=full` still returns them.
- Detailed runtime status sampling now uses the normal 5-second runtime stats
  sampling interval at `pkg/web/sysinfo.go:16`, avoiding avoidable status
  sampling amplification.
- Work-lane finalization panic handling was hardened in
  `pkg/engine/work_lane.go` and covered by
  `TestWorkLaneFinishPanicAfterLockDoesNotDeadlock`.

Regression tests / guards:

- `pkg/engine/runtime_test.go:395` now exercises concurrent reload against a
  broad set of public/admin readers, including public catalog, providers,
  query, comparison, retention, metadata, and status readers.
- The test logs per-reader active/done counters if it times out, so future
  no-progress failures identify the blocked reader class instead of producing a
  generic timeout.

Validation run after Round 5 fixes:

- `go test -race ./pkg/engine -run 'TestReloadConcurrentPublicRuntimeReadersRace|TestWorkLaneFinishPanicAfterLockDoesNotDeadlock|TestWorkLaneFinishPanicReleasesSlotForLaterWork' -count=1 -timeout 300s`
- `go test ./pkg/web -run 'TestAdminStatusDefaultsToLightAndRequiresFullModeForDetailedSnapshot|TestAdminStatusLightIncludesFeedHealthSummary|TestAdminStatusLightIncludesEngineLane|TestAdminStatusLightIncludesIntegritySummary' -count=1`
- `go test ./pkg/cache ./pkg/engine ./pkg/web ./pkg/scheduler ./cmd/update-ipsets ./internal/observability ./pkg/output ./pkg/systemd -count=1`
- `go test -race ./pkg/cache ./pkg/engine ./pkg/web ./pkg/scheduler -count=1 -timeout 300s`

Validation result: all commands passed.

Updated gap analysis:

- The confirmed reload self-deadlock means production stalls can come from
  config/runtime publication and reload-sensitive public readers, not only from
  entity refresh or watchdog notification.
- The original watchdog-deadline and diagnostic-capture findings remain valid
  hardening items unless reviewers prove they are fully addressed elsewhere.
- The main remaining uncertainty is whether any other reload-time helper still
  calls a lock-taking engine accessor while `e.mu.Lock()` is held, or reads a
  mutable cache entry directly from a public/admin reader path.

Remaining work:

- Rerun the six external reviewers on this updated SOW and current diff.
- Treat any newly identified reload lock re-entry, mutable-entry reader, or
  watchdog liveness issue as blocking until verified and fixed or explicitly
  rejected with evidence.

### Implementation Progress - 2026-06-25 - V12 Phase 13 Round 6 Reviewer Follow-Up Fixes

Status: in progress.

Reviewer results assessed in this round:

- `qwen`: returned `PRODUCTION GRADE` on the Round 5 baseline. Non-blocking
  note: the compatibility `output.SyncGit()` wrapper still uses
  `context.Background()`, but production callers use `SyncGitContext()`.
- `deepseek`: returned `NOT PRODUCTION GRADE`. Accepted blocking substance:
  long-held engine-lane slots had no daemon-side warning, and entity refresh
  needed additional context checkpoints around index and feed-presence staging.
- `minimax`: returned `NOT PRODUCTION GRADE`. Accepted blocking substance:
  request/run-owned helper paths still had detached `context.Background()`
  wrappers for generated metadata, retention, and runtime-ledger cache loading.
  Rejected or constrained claims:
  - The public `/api/v1/sets/<feed>/retention` endpoint does not rebuild
    retention on demand; `pkg/web/routes.go:153` serves the cached
    `<feed>_retention.json` artifact through `servePublicSetFile()`.
  - A broad atomic-pointer rewrite for `cfg/runtime` is not required for the
    verified bug. Current `Config()` and `Runtime()` use `e.mu.RLock()` at
    `pkg/engine/engine.go:365` and `pkg/engine/engine.go:374`; the verified
    failure was same-goroutine lock re-entry, not unsynchronized publication.
- `kimi`: invalidated. The rerun violated the reviewer prompt by spawning other
  model reviews from inside its own review session, so its output is not
  treated as independent evidence.
- `mimo`: returned `NOT PRODUCTION GRADE` on a stale pre-Round-6 snapshot.
  Its repeated claim that status snapshot still holds `e.mu` while collecting
  run metrics is rejected against current evidence: `pkg/engine/status_snapshot.go:133`
  releases `e.mu` before `current.snapshot(true)` at
  `pkg/engine/status_snapshot.go:136` and lifetime metrics at
  `pkg/engine/status_snapshot.go:142`.
- `glm`: timed out without a final verdict. Its partial output identified a
  useful liveness correlate: git sync/push can consume most of the default
  600-second timeout while running inside the admitted engine-run lane.
  This does not directly explain a systemd watchdog kill because the watchdog
  goroutine is decoupled, but it can starve engine-lane work and previously had
  poor active-operation visibility.

Accepted fixes implemented:

- Long-held engine-lane diagnostics:
  - `pkg/engine/engine_lane_diagnostics.go:16` starts a daemon-side lane
    diagnostics loop from `AttachWorkLaneContext()`.
  - `pkg/engine/engine_lane_diagnostics.go:40` logs a rate-limited warning when
    an active lane item exceeds the existing one-minute progress interval.
  - `pkg/engine/work_lane.go:303` adds `snapshotAt(now)` so diagnostics and
    tests can evaluate elapsed lane time deterministically.
  - `pkg/engine/engine_lane_diagnostics_test.go:12` verifies exactly one
    warning is emitted for a long-held slot within the rate-limit window.
- Run/request context propagation:
  - `pkg/engine/public_series.go:108` adds
    `writePublicRetentionJSONContext()` and uses the run context for fallback
    retention generation.
  - `pkg/engine/metadata_write.go:197` passes the metadata run context through
    per-feed derivative artifact generation.
  - `pkg/engine/query.go:592` adds `RetentionContext()` while preserving the
    existing `Retention()` compatibility wrapper.
  - `pkg/engine/runtime_ledger_cache.go:320` and
    `pkg/engine/runtime_ledger_cache.go:429` add context-aware history tail and
    retention-past cache accessors.
  - `pkg/engine/retention_update.go:144` uses the run context for retention
    past-cache loading.
  - `pkg/engine/retention.go:61` checks cancellation at retention rebuild
    entry, and the CSV/cohort loops check cancellation before each item.
  - `pkg/engine/public_series_context_test.go:12` and
    `pkg/engine/public_series_context_test.go:33` verify cancellation stops
    retention artifact and derivative artifact generation without writing
    output.
- Critical cleanup context propagation:
  - `pkg/engine/critical.go:745` adds
    `CleanupStaleCriticalInfrastructureArtifactsContext()`.
  - `pkg/engine/critical.go:837` makes admitted cleanup use the lane context
    instead of detached publish work.
- Entity refresh cancellation checkpoints:
  - `pkg/engine/entity_surgical_refresh.go:312` checks context before entity
    index patching.
  - `pkg/engine/entity_surgical_refresh.go:383` checks context before and after
    loading merged sidecars for feed-presence staging.
- Git-sync visibility and reload safety:
  - `pkg/engine/metadata.go:42` snapshots runtime once for git sync instead of
    reading `e.runtime` directly during publication.
  - `pkg/engine/metadata.go:53` registers
    `publish.sync_generated_files` as an active operation while git sync/push is
    running. A long `git push` should now appear in periodic engine progress
    logs and the admin light status lane snapshot instead of looking like
    unexplained silence.

Decision recorded:

- Moving git sync/push fully outside the engine lane is not part of this
  hotfix. That is a larger design change because git writes must remain
  serialized across engine runs and entity artifact refreshes. The accepted
  surgical fix is bounded timeout plus explicit active-operation visibility.
  A future long-term-best SOW can introduce a dedicated serialized git-sync
  worker if production evidence shows lane starvation from push latency remains
  operationally unacceptable.

Validation run after Round 6 fixes:

- `go test ./pkg/engine -run 'TestWritePublicRetentionJSONHonorsCancelledContext|TestWritePerFeedDerivativeArtifactsHonorsCancelledContext|TestEngineLaneDiagnosticsLogsLongHeldSlot|TestReloadConcurrentPublicRuntimeReadersRace|TestRunDiagnosticSummaryIncludesOperationsCountersAndActiveWork|TestRunFinalizationReleasesLaneBeforeCachePersistenceSubmit' -count=1 -timeout 300s`
- `go test ./pkg/engine ./pkg/web ./pkg/scheduler ./cmd/update-ipsets ./internal/observability ./pkg/output ./pkg/systemd -count=1 -timeout 300s`
- `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -count=1 -timeout 300s`
- `git diff --check`

Validation result: all commands passed.

Remaining work:

- None for the Round 6 reviewer rerun. Results and residual risk mapping are
  recorded in External Review Round 7 below.

### External Review Round 7 - 2026-06-25 - Round 6 Baseline Rerun

Status: completed.

Reviewer prompt scope:

- Review the reopened SOW and current working tree against production
  deadlock, liveness, starvation, and request-path blocking risks.
- Verify the Round 6 fixes:
  - engine-lane long-held-slot diagnostics;
  - context-aware generated metadata, retention, and runtime-ledger helpers;
  - caller/lane context for critical-infrastructure cleanup;
  - entity refresh cancellation checkpoints;
  - git-sync runtime snapshot and `publish.sync_generated_files` active
    operation visibility.
- Return a final verdict of `PRODUCTION GRADE` or `NOT PRODUCTION GRADE`.
- Do not modify files, do not write `/tmp` files, and do not run nested model
  reviewers.

Reviewer results:

- `qwen`: `PRODUCTION GRADE`.
  - Verified the reload self-deadlock fix, worker-submitted continuation
    context rebinding, entity publish lease release before git sync,
    status-snapshot lock split, runtime-stats sampler, query/cache lock
    narrowing, work-lane notification hardening, finalization panic handling,
    reload reader race coverage, and engine-lane long-held-slot diagnostics.
  - Non-blocking notes: remaining direct `e.runtime` reads in metadata and
    retention paths should be tightened in a future snapshot-hardening pass;
    compatibility `output.SyncGit()` still uses `context.Background()` but
    production callers use `SyncGitContext()`.
- `mimo`: `PRODUCTION GRADE`.
  - Verified the Round 6 fixes and the broader V7/V11 liveness work.
  - Non-blocking notes: full status still copies several fields under
    `e.mu.RLock()`; entity publication intentionally serializes filesystem
    work under `entityArtifactPublishMu`; direct test fixture nil-lane fallback
    is acceptable because it is test-only.
  - Corrected reviewer wording: the publication path holds
    `entityArtifactPublishMu`, not `entityArtifactsMu`, across the serialized
    publish section.
- `deepseek`: `PRODUCTION GRADE`.
  - Ran local read-only validation:
    `go build ./...`, `go test ./pkg/engine/... ./pkg/web/... ./pkg/scheduler/...`,
    and `go test -race ./pkg/engine/... ./pkg/web/... ./pkg/scheduler/...`;
    all passed.
  - Non-blocking notes: callbacks must remain cancellation-cooperative; Go
    cannot assign scheduling priority to the watchdog goroutine; git sync can
    still occupy an engine-lane slot until its timeout.
- `glm`: `PRODUCTION GRADE`.
  - Verified watchdog notify deadlines, pre-watchdog diagnostics, short status
    locks, reload lock-scope narrowing, bounded scheduler admission, async run
    finalization, runtime stats sampling, query/ASN cache lock narrowing, git
    subprocess timeout/reaping, SIGHUP recovery, HTTP-accepted work daemon
    context, observer metrics atomics, and engine-lane long-held-slot
    diagnostics.
  - Medium non-blocking residual: `entityArtifactPublishMu` is still held
    across `web_batch.publishContext()` filesystem work, including full-file
    content comparison and atomic promotion. This is not a deadlock because
    public serving, light status, and watchdog paths do not take that mutex,
    and the work is context-aware and visible through lane diagnostics. It is
    the strongest next optimization candidate for slow I/O environments.
- `minimax`: `PRODUCTION GRADE`.
  - Verified all Round 6 fixes and rechecked the reload helper path that still
    runs while `e.mu.Lock()` is held. The helper no longer re-enters `Runtime()`
    because `bootstrap_entries.go` now uses runtime-parameterized helpers from
    the captured runtime.
  - Non-blocking notes: git sync still runs inside the engine lane by design;
    `metadata_write.go` still has direct `e.runtime.BaseDir` reads that should
    be folded into a snapshot-hardening follow-up; diagnostics start only from
    `AttachWorkLaneContext()`, which is correct for production daemon paths.
- `kimi`: invalid result.
  - The reviewer violated the prompt by writing `/tmp/reviewer_prompt.txt` and
    launching nested `opencode` model reviews from inside its own session.
  - The session was stopped and its nested output is not used as independent
    evidence.

Consensus:

- All valid reviewers returned `PRODUCTION GRADE`.
- No valid reviewer identified a remaining blocking deadlock, watchdog-kill,
  or request-path freeze bug in the current implementation.
- The current production deadlock class is considered fixed by the verified
  reload self-deadlock removal, lock splitting, bounded work queues, watchdog
  deadline/diagnostic hardening, context propagation, and active-operation
  visibility.

Accepted residual risks and follow-up mapping:

- Entity artifact publication can still be slow on I/O-constrained hosts because
  `entityArtifactPublishMu` intentionally serializes filesystem promotion and
  content comparison. This is not a deadlock blocker for this SOW, but it should
  be the first item in the next performance/liveness investigation if
  production logs show long `entity.refresh` or publish lane holds after this
  fix.
- Git sync/push still occupies the engine lane until completion or timeout.
  This SOW accepted active-operation visibility and timeout as the surgical
  fix. A dedicated serialized git-sync worker remains the long-term-best design
  if production proves push latency is still operationally harmful.
- Remaining direct runtime reads in metadata-writing paths are not tied to the
  confirmed deadlock. They should be tightened in a future snapshot-hardening
  pass, especially if reloads are triggered while runs are active.

Validation after reviewer rerun:

- No additional code changes were made after the Round 6 validation commands.
- The valid reviewer rerun added independent validation from `deepseek`:
  `go build ./...`, `go test ./pkg/engine/... ./pkg/web/... ./pkg/scheduler/...`,
  and `go test -race ./pkg/engine/... ./pkg/web/... ./pkg/scheduler/...`, all
  passing.

### Gap Analysis Round 8 - 2026-06-25 - Residual Liveness Findings

Status: in progress.

Purpose:

- Continue the mandated deadlock/liveness loop until a fresh gap analysis cannot
  find anything fixable.
- Treat reviewer "non-blocking" findings as real work when they still preserve
  avoidable long-held locks, lane starvation, or reload race risk.

Surface note:

- This section is SOW-only maintainer evidence. It is not public documentation
  or admin UI copy.

Finding 1: entity artifact publication still performs slow filesystem work
under the entity artifact publish serialization mutex.

- Evidence:
  - `pkg/engine/entity_artifact_publish.go:162` acquires
    `entityArtifactPublishMu` through `acquireEntityArtifactPublishLease()`.
  - `pkg/engine/entity_artifact_publish.go:172` through
    `pkg/engine/entity_artifact_publish.go:180` publish private entity
    artifacts and public entity artifacts while the lease is still held.
  - `pkg/engine/web_batch.go:206` walks the staged tree during
    `publishContext()`.
  - `pkg/engine/web_batch.go:242` calls
    `sameRegularFileContentContext()` before deciding whether to touch or
    replace a live file.
  - `pkg/engine/web_batch.go:370` allocates two 32 KiB buffers and
    `pkg/engine/web_batch.go:373` through `pkg/engine/web_batch.go:393` reads
    full staged/live file contents chunk by chunk.
  - `pkg/engine/web_batch.go:274` through `pkg/engine/web_batch.go:331`
    performs deletes, touches, ownership changes, and stage cleanup in the same
    publish path.
- Failure mode:
  - This is not a lock-cycle deadlock because public serving, light admin
    status, and watchdog paths do not take `entityArtifactPublishMu`.
  - It is still avoidable lane starvation: on a slow I/O host, full-file
    content comparison and promotion can hold the serialization mutex and the
    admitted engine-lane slot for minutes.
  - Production already showed I/O wait and silence windows, so this remains a
    credible contributor to bad operator experience even after watchdog
    hardening.
- Plan:
  - Split staged publication into a preparation phase and a commit phase.
  - Preparation runs outside `entityArtifactPublishMu`; it walks staged files,
    compares staged/live contents, sorts delete/touch work, and records a
    concrete publish plan.
  - Commit runs under `entityArtifactPublishMu`; it only applies already-planned
    filesystem mutations, removes stage files, bumps generation, and updates
    progress.
  - The generation check remains after preparation and before commit. If another
    entity publication changed generation while preparation was running, the
    prepared plan is rejected as stale and the optimistic restage path retries.
  - Tests must prove a blocked content-comparison preparation does not hold
    `entityArtifactPublishMu`, while identical-file in-place touch behavior and
    changed-file replacement behavior remain intact.

Finding 2: git sync/push still occupies the engine lane.

- Evidence:
  - `pkg/engine/run_pipeline.go:427` calls `e.syncGeneratedFiles()` inside
    `publishRunArtifacts()`, which runs inside the admitted engine-run lane.
  - `pkg/engine/entity_artifact_publish.go:185` calls `e.syncGeneratedFiles()`
    from entity artifact publication after releasing `entityArtifactPublishMu`,
    but still inside the admitted entity lane callback.
  - `pkg/engine/metadata.go:52`, `pkg/engine/metadata.go:71`, and
    `pkg/engine/metadata.go:84` run `output.SyncGitContext()`.
  - `pkg/output/sync.go:176` through `pkg/output/sync.go:242` can run git
    add/commit/push/gc for up to `PushToGitTimeout`.
- Failure mode:
  - The operation is now timeout-bounded and visible as
    `publish.sync_generated_files`, so it is not silent anymore.
  - It can still block the engine lane until git completes or times out,
    delaying integrity refresh, entity refresh, cleanup, and future engine runs.
- Plan:
  - Introduce a serialized git-sync worker owned by the engine, separate from
    the engine lane. It must be FIFO or otherwise prove that coalescing cannot
    lose a required committed artifact.
  - Worker jobs contain the runtime snapshot, generated-file list, and
    published web paths needed by the sync operation.
  - Main `RunOnce` submits git sync from inside the lane but waits for the job
    during post-lane finalization so the current "git failure makes the run
    fail" contract is preserved without occupying the engine lane.
  - Background entity artifact refresh submits git sync to the same worker and
    exposes worker state in admin status; it should not hold the engine lane
    while git waits on network or disk.
  - Tests must prove a blocked git sync does not hold the engine lane and that
    direct/synchronous `RunOnce` still observes a git-sync failure after the
    lane slot is released.

Finding 3: metadata writing still reads runtime fields directly from the engine.

- Evidence:
  - `pkg/engine/metadata_write.go:70` reads `e.runtime.BaseDir` to decide
    `baseGit`.
  - `pkg/engine/metadata_write.go:161`, `pkg/engine/metadata_write.go:182`,
    `pkg/engine/metadata_write.go:189`, `pkg/engine/metadata_write.go:252`,
    `pkg/engine/metadata_write.go:256`, and
    `pkg/engine/metadata_write.go:260` read `e.runtime.BaseDir` directly.
- Failure mode:
  - This is not the confirmed reload self-deadlock; these reads do not take
    `e.mu`.
  - It is still reload-safety debt: concurrent reload can publish a new runtime
    while a metadata write uses direct fields instead of one coherent runtime
    snapshot.
- Plan:
  - Extend `metadataWriteRun` with immutable runtime-derived paths captured at
    construction.
  - Replace direct `r.e.runtime.BaseDir` reads in metadata writing with captured
    fields.
  - Add or update tests to prove metadata generated-file paths are derived from
    the runtime snapshot captured at run construction, not a later runtime
    mutation.

Plan review requirement:

- Run `glm`, `minimax`, `mimo`, `kimi`, `deepseek`, and `qwen` against this
  plan before code changes.
- If a reviewer again violates read-only/no-nested-review constraints, record
  the result as invalid and continue with the valid reviewers.
- Tune this plan before implementation if valid reviewers identify a safer or
  simpler design.

### External Plan Review Round 8 - 2026-06-25 - Residual Liveness Plan Review

Status: completed, plan needs changes.

Valid reviewers:

- `glm`: PLAN NEEDS CHANGES.
  - Confirmed Findings 1 through 3.
  - Corrected the liveness framing for Finding 1: narrowing
    `entityArtifactPublishMu` helps parallel engine-lane configurations and
    background artifact publishers, but it does not reduce the default
    one-slot engine-lane hold time by itself.
  - Required explicit `RunOnce` git-sync error propagation after lane release.
  - Required no replacement-style git job coalescing unless union semantics are
    proven safe.
- `mimo`: PLAN NEEDS CHANGES.
  - Confirmed Findings 1 through 3.
  - Required an exact staged-publish split:
    `preparePublishPlan(ctx)` and `commitPublishPlan(ctx, plan, progress)`.
  - Required the same split for both the background entity path and
    `pkg/engine/run_pipeline.go:405` through
    `pkg/engine/run_pipeline.go:409`.
  - Required daemon-context git worker ownership, explicit submit/wait points,
    and shutdown/drain behavior.
- `deepseek`: PLAN NEEDS CHANGES.
  - Confirmed Findings 1 through 3.
  - Required per-job git completion signaling instead of reusing `WorkLane`
    tickets, no unsafe coalescing, preserved direct-run error propagation,
    shutdown behavior, and admin visibility.
  - Required stage cleanup semantics to be explicit when a prepared publish plan
    is rejected as stale or fails during commit.
- `kimi`: PLAN NEEDS CHANGES.
  - Confirmed Findings 1 through 3 and the need for FIFO git work.
  - Accepted concern: commit still performs filesystem syscalls under the
    publish mutex even after preparation. This is residual bounded work, not a
    full journaled publish design.
  - Rejected concern as written: concurrent staging does not modify one shared
    staged tree. `pkg/engine/web_batch.go:28` through
    `pkg/engine/web_batch.go:45` create a unique temporary stage directory for
    every batch, and `pkg/engine/entity_artifacts.go:21` through
    `pkg/engine/entity_artifacts.go:27` use that helper for entity artifacts.
    The real stale-plan risk is concurrent live publication, covered by the
    generation recheck.
- `qwen`: PLAN NEEDS CHANGES.
  - Confirmed Findings 1 through 3.
  - Added the missing in-lane cache persistence finding at
    `pkg/engine/run_pipeline.go:445`.
  - Suggested reviewing `MarkIntegrityCachesStale()` inside the lane. Local
    verification showed `pkg/engine/integrity_cache.go:296` through
    `pkg/engine/integrity_cache.go:313` only flips in-memory cache state under
    two short mutexes; this is not prioritized as a liveness fix.
- `minimax`: PLAN NEEDS CHANGES.
  - Confirmed Findings 1 through 3.
  - Independently found the same in-lane `cache.Save(e.cachePath, e.state)` at
    `pkg/engine/run_pipeline.go:445`.
  - Noted that this duplicates the post-lane cache persistence worker and
    directly contradicts the V12 Phase 5 invariant.
  - Added `copyUpdatedIPSetsToWebContext()` at
    `pkg/engine/run_pipeline.go:419` as a lower-frequency lane-held I/O path.

Reviewer consensus:

- The current implementation is not production-grade for the deadlock/liveness
  objective because a fresh gap analysis found more lane-held I/O.
- The highest-priority newly confirmed gap is the leftover direct
  `cache.Save()` in `publishRunArtifacts()`.
- The git worker design is valid only if it preserves direct `RunOnce` failure
  semantics through per-job result signaling after the engine lane has been
  released.
- The staged-publish split is valid only if stale-plan detection, stage cleanup,
  and pipeline/background paths are all covered by tests.

Additional verified finding 4: cache persistence still runs inside the engine
lane and duplicates the worker save.

- Evidence:
  - `pkg/engine/run.go:26` through `pkg/engine/run.go:35` execute
    `runOnceAdmitted()` inside `engineLane.Run()`.
  - `pkg/engine/run.go:180` calls `publishRunArtifacts()` inside that admitted
    callback.
  - `pkg/engine/run_pipeline.go:445` returns
    `cache.Save(e.cachePath, e.state)` before the callback can return.
  - `pkg/engine/run.go:282` through `pkg/engine/run.go:313` also submit the
    final cache snapshot to the `cachePersistenceWorker` after the lane
    returns.
  - `pkg/cache/cache.go:307` through `pkg/cache/cache.go:365` perform
    directory creation, temporary file creation, write, chmod, `Sync()`, close,
    and rename.
- Failure mode:
  - Daemon runs pass `AsyncCachePersistence: true` at
    `pkg/scheduler/processing_loop.go:55`, but they still perform one
    synchronous cache save inside the lane before the async worker save.
  - On an I/O-constrained host, the in-lane `Sync()` can contribute to the
    same silent lane-starvation symptom class production showed.
- Accepted fix:
  - Remove the direct `cache.Save()` from `publishRunArtifacts()`.
  - Use the existing post-lane `cachePersistenceWorker` as the single normal
    run cache persistence path.
  - Preserve direct-run durability by keeping the existing post-lane wait in
    `waitForSynchronousCachePersistence()`.
  - Preserve daemon availability by keeping `AsyncCachePersistence` non-waiting
    after submission.
- Required tests:
  - A blocked cache persistence save must not hold the engine lane.
  - A direct/synchronous run must still wait outside the lane for cache
    persistence before returning.
  - Daemon async cache persistence must not perform duplicate in-lane saves.

Additional verified finding 5: raw ipset web copy can still hold the engine
lane for filesystem comparison/copy work.

- Evidence:
  - `pkg/engine/run_pipeline.go:419` calls
    `copyUpdatedIPSetsToWebContext()` inside `publishRunArtifacts()`, still
    inside the admitted engine-lane callback.
  - `pkg/engine/web_ipsets.go:16` through `pkg/engine/web_ipsets.go:70` read
    runtime paths directly and copy updated raw ipset files.
  - `pkg/engine/web_ipsets.go:98` calls `sameRegularFileContentContext()`.
  - `pkg/engine/web_ipsets.go:99` through `pkg/engine/web_ipsets.go:145`
    perform chmod, chtimes, chown, full copy, close, and rename.
- Failure mode:
  - This path is lower frequency than metadata and entity publication, but it is
    the same slow I/O class and it holds the scarce engine lane.
- Accepted fix:
  - Convert the raw ipset web-copy path to prepared work where comparisons and
    source validation happen before the minimal commit step.
  - If the commit step must still copy bytes for correctness, it must have
    progress visibility and bounded cancellation points, and it must not be
    hidden behind silent finalization.
  - Runtime-derived paths used by this path should come from one runtime
    snapshot.

Rejected or downgraded reviewer claims:

- `MarkIntegrityCachesStale()` inside the lane is downgraded to reviewed but not
  a current fix target. Evidence: `pkg/engine/integrity_cache.go:296` through
  `pkg/engine/integrity_cache.go:313` only performs short in-memory state
  updates under `pipelineIntegrityCacheMu` and `entityIntegrityCacheMu`; no
  filesystem, network, or broad recomputation occurs there.
- Shared-stage stale-plan mutation is rejected as stated. Every publish batch
  gets a unique stage directory via `os.MkdirTemp()`. Stale live publication is
  still real and remains covered by the generation recheck.

### Tuned Implementation Contract V13 - 2026-06-25

This contract supersedes the initial Round 8 plan bullets where they differ.

Purpose:

- Long-term-best for this SOW: remove avoidable slow I/O and external process
  waits from scarce serialization points while preserving artifact correctness,
  direct-run failure semantics, admin visibility, and the ten-year feed-history
  safety requirement.

Implementation sequence:

1. Remove duplicate in-lane cache persistence.
   - Delete the direct `cache.Save(e.cachePath, e.state)` from
     `publishRunArtifacts()`.
   - Keep final cache snapshot capture in `runOnceAdmitted()` and persistence
     submission in `completeRunFinalization()`.
   - Make `completeRunFinalization()` return an error only if a synchronous
     direct-run wait must propagate a persistence failure. Daemon async
     submission errors remain logged and surfaced through existing telemetry.
   - Add tests proving the engine lane is available while cache persistence is
     blocked after run processing has completed.

2. Split staged publication into prepare and commit phases.
   - Add a `stagedPublishPlan` built by
     `(*stagedPublishBatch).preparePublishPlan(ctx)`.
   - Preparation runs outside `entityArtifactPublishMu`; it walks the unique
     stage directory, compares staged/live file content, validates relative
     paths, records mkdir/file replace/identical-touch/delete/touch actions, and
     records publish totals.
   - Preparation must not delete staged files. This keeps stale-plan rejection
     cheap and lets existing cleanup remove the unique stage directory.
   - Commit runs under `entityArtifactPublishMu` through
     `commitPublishPlan(ctx, plan, progress)`. It applies the already planned
     filesystem mutations, removes staged files after successful identical
     touches, updates progress, returns published paths, and removes the stage
     directory at the end.
   - The generation check remains after preparation and before commit. If the
     generation changed, reject the prepared plan as stale and let the
     optimistic retry restage.
   - Apply this split to both:
     - `publishEntityArtifactMutationPlan()` in
       `pkg/engine/entity_artifact_publish.go`;
     - the pipeline entity publish path in
       `pkg/engine/run_pipeline.go:405` through
       `pkg/engine/run_pipeline.go:409`.
   - Record residual risk: commit still performs filesystem syscalls under the
     publish mutex. A journaled or rename-only design is not part of this
     change unless fresh testing shows commit itself is still a blocker.

3. Move git sync to a dedicated serialized worker.
   - Add an engine-owned `gitSyncWorker`, separate from `WorkLane`.
   - Jobs are FIFO, not replacement-coalesced. Each job carries immutable copies
     of the runtime snapshot, generated-file slice, web-published path slice,
     and labels for admin visibility.
   - Each submit returns a handle with `Wait(ctx) error` so direct `RunOnce`
     can observe git failures after the engine lane is released.
   - Main pipeline runs submit the git job inside the lane, store the handle in
     `runFinalization`, release the lane, then wait in finalization and return
     the git error as the `RunOnce` error.
   - Background entity artifact refresh submits to the same worker and does not
     keep an engine-lane slot while git waits on disk/network. Its failures are
     logged and exposed in admin status as asynchronous background failures.
   - Worker shutdown must cancel or drain with a bounded grace period, and
     queued/running state must be visible through admin status.

4. Snapshot runtime-derived metadata and raw web-copy paths.
   - Extend `metadataWriteRun` with captured `Runtime`, `baseDir`, and any
     derived base paths needed by `addFeedIndexRows()`,
     `writePerFeedArtifacts()`, and `writeGitArtifacts()`.
   - Replace direct `r.e.runtime.BaseDir` reads in metadata writing with the
     captured fields.
   - Convert `copyUpdatedIPSetsToWebContext()` to use one runtime snapshot for
     `BaseDir`, `WebDirForIPSets`, and `WebOwner`.
   - Audit remaining direct `e.runtime.*` reads in `pkg/engine` and classify
     them as one of:
     - constructor/reload mutation;
     - cheap status snapshot;
     - existing accepted run-snapshot limitation;
     - fix in this SOW because it crosses reload with file paths.

5. Validation.
   - New or updated behavioral tests must cover:
     - blocked cache persistence does not hold the engine lane;
     - direct `RunOnce` still waits outside the lane for synchronous cache
       persistence;
     - git sync failure still fails direct `RunOnce` after lane release;
     - blocked git sync does not hold the engine lane;
     - FIFO git job ordering and shutdown behavior;
     - staged publish preparation does not hold `entityArtifactPublishMu`;
     - stale generation between prepare and commit triggers retry;
     - pipeline entity publish uses the same prepare/commit path;
     - metadata writer paths use construction-time runtime snapshot;
     - raw web-copy path uses one runtime snapshot.
   - Required local validation before external review rerun:
     - `go test -race ./pkg/engine -run 'TestEntityArtifactPublish|TestMetadataWriteRun|TestWorkLane|TestSharedLatestSetCache|TestRuntimeLedger|TestBlockedCachePersistence|TestRunFinalization|TestRunDiagnostics|TestReloadConcurrent|TestSyncGeneratedFiles|TestGitSync' -count=1 -timeout 300s`
     - `go test -race ./pkg/web -run 'TestRunServesHealthAndLightStatusWhileEngineRunBlocked|TestAdminStatusLight|TestRunWatchdog|TestStartupEntityArtifacts' -count=1`
     - `go test ./pkg/cache ./pkg/output ./pkg/config ./pkg/engine ./pkg/web ./pkg/scheduler ./cmd/update-ipsets ./internal/observability ./internal/runtimeinfo ./tools/archposture -count=1`
     - `git diff --check`

Implementation recommendation:

- The recommended design is **long-term-best**: FIFO git worker, no git job
  replacement coalescing, remove duplicate in-lane cache save, and split
  staged publication. This costs more code than a surgical timeout-only fix, but
  it directly targets the production silence windows without sacrificing
  artifact or history safety.

### External Plan Review Round 9 - 2026-06-25 - V13 Rerun

Status: completed, plan still needs changes before implementation.

Valid reviewers:

- `glm`: PLAN NEEDS CHANGES.
  - Confirmed V13's cache, git, staged-publish, metadata, and raw-copy findings.
  - Added a git commit-fidelity concern: if the engine clears `e.running` before
    the deferred git worker stages files, a later run can publish newer content
    into the same paths before the earlier job commits.
  - Required an explicit RunOnce error merge path for lane errors, git errors,
    and cache persistence errors.
  - Required git worker lifecycle and daemon-context ownership to be explicit.
- `minimax`: PRODUCTION GRADE PLAN with clarifications.
  - Confirmed all V13 findings.
  - Requested explicit treatment for `applyGeneratedFileTimestampsContext()` and
    git worker shutdown/drain details.
  - Recommended implementing the duplicate in-lane `cache.Save()` removal first.
- `mimo`: PLAN NEEDS CHANGES.
  - Confirmed all V13 findings.
  - Required `copyUpdatedIPSetsToWebContext()` to actually move slow comparison
    and copy work out of the lane, not only snapshot runtime fields.
  - Requested an explicit `return nil` success path after removing
    `cache.Save()`, and an explicit direct-run cache-persistence failure test.
  - Requested small marker writes be listed as accepted in-lane work if they
    remain in-lane.
- `kimi`: PLAN NEEDS CHANGES.
  - Added a critical panic-safety issue in the pipeline entity publish path:
    `pkg/engine/run_pipeline.go:405` through
    `pkg/engine/run_pipeline.go:409` acquire `entityArtifactPublishMu` but do
    not use `defer lease.release(true)`, so a panic in
    `entityBatch.publishContext()` leaks the publish mutex.
  - Required the main web batch publish path
    `pkg/engine/run_pipeline.go:392` to be fixed or explicitly accepted as
    residual lane-held I/O.
  - Required daemon/scheduler git semantics to be explicit.
  - Required git jobs to handle both base and web repository sync targets
    sequentially.
- `deepseek`: PLAN NEEDS CHANGES.
  - Confirmed V13 findings.
  - Required exact generation recheck locking semantics in the staged-publish
    commit path.
  - Required cache snapshot capture timing to be explicit.
  - Added a lock-hygiene finding:
    `pkg/engine/work_lane.go:456` through `pkg/engine/work_lane.go:518` can
    call synchronous observability gauge recording while holding `WorkLane.mu`.
- `qwen`: PLAN NEEDS CHANGES, minor.
  - Confirmed V13 findings.
  - Required explicit post-`cache.Save()` success return, non-lane git wait
    context, stage cleanup ownership, git worker admin status, shutdown tests,
    and direct-run cache persistence failure propagation.

Newly accepted findings from Round 9:

1. Pipeline entity publish can leak `entityArtifactPublishMu` on panic.
   - Evidence:
     - `pkg/engine/run_pipeline.go:405` through
       `pkg/engine/run_pipeline.go:409` call
       `acquireEntityArtifactPublishLease()`, then
       `entityBatch.publishContext()`, then `lease.release(true)` with no
       defer.
     - `pkg/engine/work_lane.go:548` through
       `pkg/engine/work_lane.go:555` recover panics at the lane callback
       boundary, after the stack has already skipped the explicit release.
   - Failure mode:
     - One panic during pipeline entity publication permanently leaves
       `entityArtifactPublishMu` locked, blocking all future entity
       publication until process restart.
   - Accepted fix:
     - The shared staged-publish commit helper must acquire the publish lease
       and immediately defer release.
     - Tests must inject a panic during pipeline entity commit and prove a later
       publish can acquire the lease.

2. Deferred git sync needs run-to-git fidelity, not only FIFO ordering.
   - Evidence:
     - `pkg/engine/run.go:255` through `pkg/engine/run.go:263` use
       `e.running` to reject overlapping `RunOnce` calls.
     - `pkg/engine/run.go:336` through `pkg/engine/run.go:360` currently clear
       `e.running` when marking finalizing.
     - `pkg/output/sync.go:213` stages live paths with `git add`; git jobs do
       not carry file content snapshots.
   - Failure mode:
     - If `e.running` is cleared before a queued git job stages files, a later
       run can publish newer content to the same paths before the earlier job
       commits. FIFO job ordering alone does not preserve run-to-commit
       attribution because each job stages current disk contents.
   - Accepted fix:
     - Keep `e.running=true` and `runState=finalizing` until finalization git
       jobs finish. The engine lane slot is released, but a new `RunOnce` cannot
       start and mutate published paths until the previous run's git job has
       staged/committed or failed.
     - Admin status must clearly show `run_state=finalizing` and the active git
       worker state during this wait.

3. RunOnce must merge post-lane finalization errors into its returned error.
   - Evidence:
     - `pkg/engine/run.go:36` through `pkg/engine/run.go:47` call
       `completeRunFinalization(finalization)` as a void operation and return
       only the lane callback error.
   - Failure mode:
     - Moving git or cache persistence out of the lane without changing the
       return path would silently swallow post-lane failures.
   - Accepted fix:
     - `completeRunFinalization()` returns an error that includes git wait and
       synchronous cache persistence failures.
     - `RunOnce()` returns `errors.Join(laneErr, finalizationErr)` so callers
       and the scheduler still see a non-nil error for finalization failures.

4. `WorkLane` should not call observability recorders while holding
   `WorkLane.mu`.
   - Evidence:
     - `pkg/engine/work_lane.go:456` through `pkg/engine/work_lane.go:480`
       call `observeWorkerStartLocked()` from `scheduleLocked()`.
     - `pkg/engine/work_lane.go:495` through `pkg/engine/work_lane.go:518`
       call `observability.Duration()` and `observability.Gauge()`.
     - `internal/observability/observability.go:612` through
       `internal/observability/observability.go:623` record gauges
       synchronously.
   - Failure mode:
     - Even if the current exporter is usually fast, calling observability
       instrumentation while holding the work-lane mutex is unnecessary lock
       coupling and fits the class of avoidable liveness risks this SOW is
       removing.
   - Accepted fix:
     - Capture metric values under `WorkLane.mu`, then emit observability
       records after unlocking.

Round 9 rejected or downgraded claims:

- Fire-and-forget daemon git sync is rejected for this SOW. It would change the
  existing scheduler contract at `pkg/scheduler/processing_loop.go:72` through
  `pkg/scheduler/processing_loop.go:82`, where a `RunOnce` error marks the
  processing batch failed. The long-term-best choice is to wait outside the
  engine lane and still return git failure to callers.
- Small marker writes at `pkg/engine/run_pipeline.go:435` through
  `pkg/engine/run_pipeline.go:443` are accepted as residual bounded writes if
  publish finalization remains inside the run lifecycle. They are tiny marker
  files, not broad scans or full-file copies.
- Startup/pre-processing cleanup I/O in `ensureDirectories()` and
  `applyRenamesAndDeletes()` is accepted as residual preflight lane work for
  this SOW unless production evidence shows it is slow. It is outside the
  confirmed production silence windows and happens before expensive processing.

### Tuned Implementation Contract V14 - 2026-06-25

This contract supersedes V13 where they differ.

Purpose:

- Long-term-best for this SOW: keep public/admin serving and watchdog work free,
  release the scarce engine lane before final publication/git/cache waits, keep
  `RunOnce` serialization and error semantics intact, and remove the confirmed
  publish mutex leak.

Implementation decisions requiring approval before code:

1. Git finalization semantics.
   - **A. Preserve current `RunOnce` failure semantics and run-to-git fidelity
     (Recommended, long-term-best).**
     - `RunOnce` submits git work, releases the engine lane, waits during
       finalization, and returns git errors.
     - `e.running` stays true with `runState=finalizing` until git finalization
       completes, so a second `RunOnce` cannot publish newer content before the
       first run's git job stages files.
     - Scheduler may wait on git outside the engine lane, but web/status and
       other non-run work stay available.
   - **B. Make daemon/scheduler git sync fire-and-forget.**
     - Faster scheduler turnover, but changes the current contract: git failure
       no longer fails the processing batch, and commits can drift to later live
       content under slow git worker overlap.
     - This is not recommended for the production correctness goal.

2. Publish finalization lane scope.
   - **A. Move final artifact publication out of the engine lane while keeping
     `RunOnce` in `finalizing` state (Recommended, long-term-best).**
     - The lane callback performs acquisition, source processing, heavy phases,
       and writes artifacts to unique stage directories.
     - Post-lane finalization performs `BeforePublish`, timestamp application,
       web batch publish, entity batch commit, raw ipset web copy, marker
       writes, git submission/wait, and cache persistence.
     - `e.running` remains true until finalization completes, preserving
       non-overlapping `RunOnce` publication.
     - Errors from publish/git/cache finalization are merged into `RunOnce`.
   - **B. Keep web batch publication inside the lane and only fix cache/git and
     entity publish mutex hold.**
     - Smaller implementation, but leaves `webBatch.publishContext()` and raw
       ipset copy as accepted lane-held I/O.
     - This is surgical, but not recommended for "fix the production silence
       class" because reviewers found the same slow-I/O pattern there.

3. WorkLane observability under lock.
   - **A. Move observability emissions outside `WorkLane.mu` (Recommended,
     surgical and long-term-best).**
     - Low risk and removes avoidable lock coupling.
   - **B. Keep current behavior.**
     - Smaller change, but leaves a plausible lock-coupling liveness risk.

User decision - 2026-06-25:

- Approved: 1A, 2A, 3A.
- Meaning: preserve `RunOnce` failure semantics and run-to-git fidelity; move
  final artifact publication out of the engine lane while the run remains
  `finalizing`; move WorkLane observability emissions outside `WorkLane.mu`.

Approved-by-recommendation implementation details once decisions are accepted:

1. Cache persistence.
   - Replace `return cache.Save(e.cachePath, e.state)` in
     `publishRunArtifacts()` with `return nil`, or remove the call entirely if
     publish finalization is split.
   - Capture the cache snapshot after publication work has finished and before
     the run leaves the serialized finalization state.
   - `completeRunFinalization()` returns a finalization error. `RunOnce()`
     returns `errors.Join(laneErr, finalizationErr)`.
   - Direct/synchronous runs continue to wait outside the lane for cache
     persistence. Daemon async runs submit and return after the required
     finalization waits selected above.

2. Staged publish prepare/commit.
   - `preparePublishPlan(ctx)` walks the unique stage directory, validates
     relative paths, performs content comparisons, computes totals, and records
     actions. It does not mutate live files and does not remove the stage
     directory.
   - `commitPublishPlan(ctx, plan, progress)` acquires
     `entityArtifactPublishMu`, immediately defers lease release, re-reads
     `entityArtifactsGeneration` under `entityArtifactsMu`, rejects stale plans
     with `errEntityArtifactStageStale`, then applies planned mutations.
   - Stage directory cleanup remains caller-owned through existing
     `cleanup()`/defer paths so stale plans, panics, and commit failures do not
     leak stage directories.
   - Both the background entity path and pipeline entity path use the same
     helper, eliminating the current no-defer pipeline leak.

3. Git worker.
   - Construct one engine-owned `gitSyncWorker` eagerly with the engine.
   - Register it with the daemon context and bounded shutdown grace; do not
     lazily recreate it per run.
   - Jobs are FIFO and may include both base-repository and web-repository sync
     targets. Targets in one job run sequentially.
   - Each job carries immutable copies of runtime sync options, generated-file
     metadata, published web paths, and labels.
   - Each submit returns a handle with `Wait(ctx) error`; finalization waits
     using the original run context or a bounded finalization context, not a
     canceled lane callback context.
   - Admin status exposes queue depth, current job labels, last error, and
     completed/failed counts.

4. Runtime snapshots.
   - `metadataWriteRun` captures runtime-derived fields at construction.
   - `grep 'e\\.runtime' pkg/engine/metadata_write.go` must return no direct
     runtime reads outside construction or explicitly reviewed helper code.
   - `copyUpdatedIPSetsToWebContext()` uses one runtime snapshot for `BaseDir`,
     `WebDirForIPSets`, and `WebOwner`.
   - The broader `pkg/engine` direct-runtime-read audit remains part of
     validation; each read is classified as constructor/reload, cheap status,
     accepted run snapshot, or fixed.

5. Validation additions beyond V13.
   - Panic during pipeline entity commit releases `entityArtifactPublishMu`.
   - A blocked web batch publish/raw ipset copy does not hold the engine lane if
     decision 2A is approved; if 2B is chosen, add a characterization test and
     record the accepted residual hold.
   - Deferred git failure returns a non-nil `RunOnce` error after lane release.
   - A slow/queued git worker cannot let a later `RunOnce` contaminate the
     earlier run's git commit.
   - Synchronous cache persistence failure propagates as a non-nil `RunOnce`
     error.
   - Git worker shutdown drains or cancels according to the chosen policy and
     updates admin status.
   - WorkLane observability emissions do not occur while `WorkLane.mu` is held.

Implementation status:

- V14 implementation is in progress.
- The three numbered decisions above were approved and recorded before code
  changes started.

### Implementation Progress - 2026-06-25 - V14 Approved Decisions

Status: in progress, ready for external implementation review.

Implemented:

1. `RunOnce` finalization split.
   - Engine-lane work now prepares the run, processes sources, runs heavy
     phases, and writes staged artifacts.
   - Final artifact publication, git sync, cache snapshot capture, and cache
     persistence happen after the engine lane callback returns.
   - `run_state=finalizing` and the legacy `running=true` flag remain set until
     finalization completes, so a second `RunOnce` is still rejected while the
     previous run is publishing/git-syncing/cache-persisting.
   - Finalization errors are merged into the `RunOnce` return error.

2. Git publication lane.
   - Generated-file git sync now runs through a dedicated one-slot FIFO work
     lane.
   - Synchronous run finalization waits for its git job outside the engine lane.
   - Background entity artifact publication submits git sync work after releasing
     the entity artifact publish lease.
   - Admin status exposes `engine.git_lane`; the admin UI includes git-lane
     active/waiting work in the background-work tile.

3. Entity publish lease safety.
   - Pipeline entity publication now defers publish lease release immediately
     after acquisition.
   - The shared acquisition helper is panic-safe for hook/acquisition paths.
   - Behavioral tests inject panics and prove later publish acquisition is not
     blocked.

4. WorkLane lock hygiene.
   - Work-lane scheduling captures worker metric events under `WorkLane.mu` and
     emits observability records after unlocking.
   - Tests cover scheduling/panic/finalization paths through the public work-lane
     behavior.

5. Architecture posture.
   - The full Go suite initially failed because `pkg/engine/query.go` exceeded
     its large-file baseline.
   - The baseline was not changed.
   - History/changeset query helpers were split into
     `pkg/engine/query_history.go`, reducing `pkg/engine/query.go` from 666
     lines to 406 lines and keeping the architecture guard green.

Validation run:

- `go test ./pkg/engine ./pkg/web -count=1`
- `go test ./pkg/engine ./pkg/web ./pkg/scheduler ./pkg/output ./pkg/systemd ./pkg/cache ./pkg/config -count=1`
- `go test ./tools/archposture -count=1`
- `go test ./... -count=1`
- `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -run 'TestWorkLane|TestPublishFinalization|TestRunFinalization|TestSynchronousCachePersistenceErrorIsRunOnceError|TestPublishRunArtifactsReleasesEntityLeaseAfterPanic|TestEntityArtifactPublishSyncsGeneratedFilesAfterReleasingPublishLease|TestRunServesHealthAndLightStatusWhileEngineRunBlocked|TestAdminStatusDefaultsToLightAndRequiresFullModeForDetailedSnap|TestAdminStatusLightRespondsWhileEngineLaneBusy' -count=1`
- `pnpm --dir ui lint`
- `pnpm --dir ui build`
- `pnpm --dir ui test`
- `make build`

Validation result:

- All commands passed.
- `pnpm --dir ui build` and `make build` emitted existing Vite warnings about
  unresolved runtime font URLs and one large chunk; these were warnings, not
  failures.

Next:

- Run the required external reviewer loop on the full SOW and working tree.
- Repeat with the same scope after any accepted reviewer fixes until the review
  loop is clean.

### Implementation Progress - 2026-06-25 - V14 Reviewer Follow-Up Fixes

Status: in progress, ready for full-scope external reviewer rerun.

Reviewer findings accepted and fixed:

1. Final publication cancellation.
   - Finding: `completeRunPublication()` used `context.Background()`, so final
     web/entity publication, git sync, and synchronous cache wait ignored the
     original run context.
   - Fix: `RunOnce()` now carries the caller context into `runFinalization`;
     final publication and synchronous cache persistence wait use that context.
   - Test: `TestRunOnceFinalPublicationUsesRunContext`.

2. Entity artifact git fidelity.
   - Finding: background entity artifact publication queued async git work with
     live file paths, so a later publish could change those paths before git
     staged them.
   - Fix: entity artifact publication now releases the entity publish lease,
     then waits on the dedicated git lane before returning. The obsolete async
     helper was removed.
   - Trade-off: this can keep the specific background task active while git runs,
     but it preserves run-to-git/file-to-git fidelity. The entity publish lease
     is not held during git sync.
   - Test: `TestEntityArtifactPublishSyncsGeneratedFilesAfterReleasingPublishLease`
     now proves publish does not return before git sync completes.

3. Cache persistence worker panic.
   - Finding: a panic in the cache save callback could kill the worker goroutine
     and leave a zombie worker that still accepted submissions.
   - Fix: cache persistence save now recovers panic, records it as a failed save,
     wakes waiters, and keeps the worker available for later snapshots.
   - Test: `TestCachePersistenceWorkerRecoversPanicAndAcceptsNextSave`.

4. Enable/disable reload race.
   - Finding: `Enable()`, `Disable()`, `EnableArtifacts()`, and
     `DisableArtifacts()` read `e.cfg`/`e.runtime` directly while
     `ReloadContext()` can swap them under `e.mu`.
   - Fix: these methods now snapshot config/runtime under `e.mu.RLock()` and
     perform filesystem operations after unlocking.

5. Server goroutine panic.
   - Finding: a panic in a listener goroutine could leave `serveRunServers()`
     waiting forever for an error-channel send that would never happen.
   - Fix: listener goroutines recover panic, cancel the run context, and report
     a listener error.
   - Test: `TestServeRunServersReturnsErrorWhenServerGoroutinePanics`.

6. Scheduler snapshot persistence under lock.
   - Finding: `storeSnapshot()` persisted scheduler JSON while holding
     `Runner.mu`.
   - Fix: it now swaps the in-memory snapshot under lock and writes the JSON
     after unlocking.
   - Test: `TestStoreSnapshotKeepsCachedSnapshotReadableWhilePersisting`.

7. Heavy-phase latest-set close under lock.
   - Finding: `latestSetCache.CloseAll()` called source `Close()` while holding
     the cache mutex.
   - Fix: it marks the cache closed and detaches maps under lock, then closes
     sources after unlocking.
   - Test: `TestLatestSetCacheCloseAllDoesNotHoldLockWhileClosingSources`.

8. Stale publish-stage cleanup queue coalescing.
   - Finding: cleanup work had no coalescing key, so repeated equivalent cleanup
     requests could accumulate in the engine lane.
   - Fix: `CleanupPublishStagesBeforeWithTrigger()` now uses a stable
     `cleanup:publish_stages:delayed` coalescing key.

9. Admin light status payload size.
   - Finding: `StatusSnapshotLight` still carried the full `last_report` payload.
   - Fix: `last_report` was removed from the light engine snapshot. The admin
     heartbeat now uses `current_batch.total` for the active-run caption and
     keeps full `last_report` available only through full status mode.
   - Intentional non-change: `current_batch`, `phase_plan`,
     `active_operations`, `background_tasks`, and lane snapshots remain in light
     status because they are cheap in-memory progress fields and are required by
     the admin UI "Being Processed Now" and background-work tiles.
   - Test: `TestAdminStatusDefaultsToLightAndRequiresFullModeForDetailedSnapshot`
     forbids `last_report` in default/light status.

Validation run after reviewer follow-up fixes:

- `go test ./pkg/engine -run 'TestRunOnceFinalPublicationUsesRunContext|TestEntityArtifactPublishSyncsGeneratedFilesAfterReleasingPublishLease|TestRunFinalization|TestCompleteRunPublication|TestPublishRunArtifactsReleasesEntityLeaseAfterPanic' -count=1`
- `go test ./pkg/scheduler -run 'TestStoreSnapshotKeepsCachedSnapshotReadableWhilePersisting|TestBuildSnapshotOrdersDueFirst' -count=1`
- `go test ./pkg/web -run 'TestAdminStatusDefaultsToLightAndRequiresFullModeForDetailedSnapshot' -count=1`
- `pnpm --dir ui test -- --run current-run heartbeat`
- `go test ./pkg/engine -run 'TestCachePersistenceWorkerRecoversPanicAndAcceptsNextSave|TestLatestSetCacheCloseAllDoesNotHoldLockWhileClosingSources|TestRunOnceFinalPublicationUsesRunContext|TestEntityArtifactPublishSyncsGeneratedFilesAfterReleasingPublishLease' -count=1`
- `go test ./pkg/scheduler -run 'TestStoreSnapshotKeepsCachedSnapshotReadableWhilePersisting|TestBuildSnapshotOrdersDueFirst' -count=1`
- `go test ./pkg/web -run 'TestServeRunServersReturnsErrorWhenServerGoroutinePanics|TestAdminStatusDefaultsToLightAndRequiresFullModeForDetailedSnapshot' -count=1`
- `pnpm --dir ui test -- --run heartbeat`
- `go test ./pkg/engine ./pkg/web ./pkg/scheduler ./pkg/output ./pkg/systemd ./pkg/cache ./pkg/config -count=1`
- `pnpm --dir ui lint`
- `pnpm --dir ui build`
- `go test ./tools/archposture -count=1`
- `go test ./pkg/scheduler -run 'TestStoreSnapshotKeepsCachedSnapshotReadableWhilePersisting' -count=1`
- `go test ./pkg/engine -run 'TestRunOnceFinalPublicationUsesRunContext' -count=1`
- `go test ./... -count=1`
- `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -run 'TestWorkLane|TestPublishFinalization|TestRunFinalization|TestRunOnceFinalPublicationUsesRunContext|TestSynchronousCachePersistenceErrorIsRunOnceError|TestCachePersistenceWorkerRecoversPanicAndAcceptsNextSave|TestLatestSetCacheCloseAllDoesNotHoldLockWhileClosingSources|TestStoreSnapshotKeepsCachedSnapshotReadableWhilePersisting|TestServeRunServersReturnsErrorWhenServerGoroutinePanics|TestEntityArtifactPublishSyncsGeneratedFilesAfterReleasingPublishLease|TestRunServesHealthAndLightStatusWhileEngineRunBlocked|TestAdminStatusDefaultsToLightAndRequiresFullModeForDetailedSnapshot|TestAdminStatusLightRespondsWhileEngineLaneBusy' -count=1`
- `git diff --check`
- `make build`

Validation result:

- All commands passed.
- `go test ./tools/archposture -count=1` initially failed because the new
  tests enlarged existing large test files. The baseline was not changed.
  The new tests were split into focused files:
  `pkg/scheduler/snapshot_store_test.go` and
  `pkg/engine/run_finalization_context_test.go`; architecture posture then
  passed.
- `pnpm --dir ui build` and `make build` emitted existing Vite warnings about
  unresolved runtime font URLs and one large chunk; these were warnings, not
  failures.

Next:

- Rerun the required full-scope external reviewer loop with the same review
  scope and the fix notes above.
- Fix or explicitly classify any new reviewer findings before closing the SOW.

### Implementation Progress - 2026-06-25 - V14 Static Analysis And Reviewer Follow-Up

Status: in progress, local validation clean, ready for full-scope external
reviewer rerun.

Additional reviewer findings accepted and fixed:

1. Cache persistence worker-loop panic recovery.
   - Finding: save-level panic recovery was present, but an unexpected panic in
     the worker loop itself could still close the goroutine and leave callers
     waiting without a clear stopped/failed state.
   - Fix: `cachePersistenceWorker.run()` now has a top-level recover path that
     marks the worker stopped, clears pending/in-flight state, records the
     failure, broadcasts waiters, and logs the panic.
   - Evidence: `pkg/engine/cache_persistence.go:164`,
     `pkg/engine/cache_persistence.go:212`.
   - Test: `TestCachePersistenceWorkerLoopPanicStopsWorkerAndWakesCallers`
     proves `Stop()`, `Snapshot()`, and later `Submit()` observe the stopped
     state instead of hanging.
   - Evidence: `pkg/engine/cache_persistence_test.go:201`.

2. Download worker panic recovery.
   - Finding: download workers should follow the same availability contract as
     the engine/action lanes: a panic must not leave the download active map or
     deferred-download bookkeeping wedged.
   - Fix: `runDownload()` now recovers panics, records the recovered panic,
     finishes the active download if needed, releases deferred download state,
     and wakes the download loop.
   - Evidence: `pkg/scheduler/download_loop.go:62`.
   - Test coverage: `TestDownloadWorkerPanicClearsActiveQueue`.

3. Entity artifact lock-order documentation.
   - Finding: the intended ordering between `entityArtifactPublishMu` and
     `entityArtifactsMu` was only implicit in code.
   - Fix: the lock-order contract is documented next to the fields: take
     `entityArtifactPublishMu` before `entityArtifactsMu`; the latter protects
     generation state only.
   - Evidence: `pkg/engine/engine.go:65`.

4. Enable/disable reload race contract.
   - Finding: the runtime snapshot behavior for marker-file enable/disable
     operations needed to be a documented product contract, not just an
     implementation choice.
   - Fix: `.agents/sow/specs/operating-principles.md` now states that
     enable/disable marker writes use a point-in-time config/runtime snapshot
     and must not hold the broad engine mutex during filesystem work.
   - Evidence: `.agents/sow/specs/operating-principles.md:245`.

5. Static-analysis hygiene.
   - Finding: `make staticcheck` and `make golangci-lint` found stale wrapper
     functions and small lint issues in the touched area and in adjacent
     hot-path packages.
   - Fix: removed unused wrappers/helpers, replaced nil contexts with
     `context.Background()`, made deferred closes explicitly ignore close
     errors, removed redundant assignments, and cleaned test style warnings.
   - Scope: no behavior changes were intended for these lint-only removals.

DroneBL reviewer note:

- One reviewer reported missing DroneBL tests. That was stale against the
  current working tree: the tests already exist and pass:
  `TestDroneBLArtifactQueuesThroughDownloadLane`,
  `TestDroneBLChildrenMaterializeInDownloadWorker`,
  `TestDroneBLDoesNotAcquireEngineLane`,
  `TestRecoveredDroneBLArtifactMaterializesInDownloadWorker`, and
  `TestRecoveredCorruptDroneBLArtifactRequeuesNormalDownloaderFetch`.
- These tests validate that DroneBL recovered artifacts and normal artifact
  work stay in the download FIFO lane, not the engine lane.

Validation run after static-analysis and reviewer follow-up:

- `make golangci-lint`
- `make staticcheck`
- `go test ./pkg/engine ./pkg/scheduler -run 'TestCachePersistenceWorkerLoopPanicStopsWorkerAndWakesCallers|TestCachePersistenceWorkerRecoversPanicAndAcceptsNextSave|TestEntityArtifactPublishSyncsGeneratedFilesAfterReleasingPublishLease|TestPublishRunArtifactsReleasesEntityLeaseAfterPanic|TestDroneBL|TestRecoveredCorruptDroneBLArtifactRequeuesNormalDownloaderFetch|TestDownloadWorkerPanicClearsActiveQueue|TestStatusSnapshotLightDoesNotHoldEngineMutexWhileWaitingOnActiveOperations' -count=1`
- `go test ./pkg/asnloc ./pkg/iprange ./pkg/engine ./pkg/scheduler -count=1`
- `go test ./... -count=1`
- `go test ./... -count=1` from `tools/dronebl2ipsets`
- `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -run 'TestWorkLane|TestPublishFinalization|TestRunFinalization|TestRunOnceFinalPublicationUsesRunContext|TestSynchronousCachePersistenceErrorIsRunOnceError|TestCachePersistenceWorkerRecoversPanicAndAcceptsNextSave|TestCachePersistenceWorkerLoopPanicStopsWorkerAndWakesCallers|TestLatestSetCacheCloseAllDoesNotHoldLockWhileClosingSources|TestStoreSnapshotKeepsCachedSnapshotReadableWhilePersisting|TestServeRunServersReturnsErrorWhenServerGoroutinePanics|TestEntityArtifactPublishSyncsGeneratedFilesAfterReleasingPublishLease|TestRunServesHealthAndLightStatusWhileEngineRunBlocked|TestAdminStatusDefaultsToLightAndRequiresFullModeForDetailedSnapshot|TestAdminStatusLightRespondsWhileEngineLaneBusy|TestDroneBL|TestRecoveredCorruptDroneBLArtifactRequeuesNormalDownloaderFetch|TestDownloadWorkerPanicClearsActiveQueue' -count=1`
- `pnpm --dir ui lint`
- `pnpm --dir ui build`
- `make build`
- `make test`
- `git diff --check`

Validation result:

- All commands passed.
- `pnpm --dir ui build` emitted the existing Vite warnings about unresolved
  runtime font URLs and one large chunk. These are warnings, not failures, and
  were already present before this follow-up.

Artifact maintenance update:

- `AGENTS.md`: no project-wide rule change needed.
- Runtime project skills: no skill change needed.
- Specs: updated `.agents/sow/specs/operating-principles.md` for the
  enable/disable marker snapshot contract.
- End-user/operator docs: no public/operator docs change needed; behavior is
  internal liveness and admin availability.
- SOW lifecycle: this SOW remains `Status: in-progress` pending full external
  reviewer rerun and closure.

Next:

- Rerun full-scope external reviewers with the same broad scope.
- Fix, reject with evidence, or explicitly classify every new reviewer finding
  before closure.

### Implementation Progress - 2026-06-25 - V14 Second Reviewer Follow-Up

Status: local validation clean after follow-up fixes; full-scope external
reviewer rerun still required before closure.

Additional reviewer findings accepted and fixed:

1. Watchdog startup ordering.
   - Finding: the watchdog was still started after startup integrity recovery,
     so a slow or wedged startup integrity scan could delay first watchdog
     notification.
   - Fix: `Run()` now attaches the lane context and starts the watchdog before
     runtime sampling, scheduler construction, startup integrity recovery, and
     other background work. Shutdown waits for the watchdog goroutine after
     canceling the run context.
   - Evidence: `pkg/web/server_run.go:38`,
     `pkg/web/server_run.go:41`, `pkg/web/server_run.go:48`.
   - Test: `TestRunWatchdogTicksWhileStartupIntegrityRecoveryBlocked` blocks
     startup integrity recovery and proves watchdog ticks continue before the
     daemon is released to serve `/healthz`.
   - Evidence: `pkg/web/run_lifecycle_test.go:445`.

2. Work-lane activation cancellation race.
   - Finding: synchronous `WorkLane.Run()` removed queued work on cancellation,
     but if the caller context was canceled exactly as the item was activated,
     the callback could still start.
   - Fix: after activation, `Run()` checks the lane-provided start context
     before executing the callback. If it is already canceled, the item is
     finalized and the callback is not called.
   - Evidence: `pkg/engine/work_lane.go:197`,
     `pkg/engine/work_lane.go:203`.
   - Test: `TestWorkLaneRunCanceledAtActivationDoesNotStart` cancels during the
     lane start-notification point and proves the callback never runs and the
     lane returns to idle.
   - Evidence: `pkg/engine/work_lane_liveness_test.go:10`.

3. Cache-persistence wait polling.
   - Finding: `cachePersistenceWorker.Wait()` used ticker polling. That was not
     the main production stall, but it was needless wakeup work in a hot
     lifecycle path.
   - Fix: `Wait()` now blocks on the worker condition variable and uses
     `context.AfterFunc()` to wake waiters when the caller context is canceled.
   - Evidence: `pkg/engine/cache_persistence.go:114`,
     `pkg/engine/cache_persistence.go:119`,
     `pkg/engine/cache_persistence.go:144`.

4. Cache-persistence test hook race and async worker cleanup.
   - Finding: the race detector found a test lifecycle race: an async cache
     persistence worker from one test could still read the package-level save
     hook while a later test restored or replaced it.
   - Fix: `saveCachePersistenceState()` now copies the hook under the existing
     hook mutex, `setCachePersistenceSaveForTest()` writes/restores under the
     same mutex, and async-cache tests explicitly stop the cache persistence
     worker before they finish.
   - Evidence: `pkg/engine/cache_persistence.go:254`,
     `pkg/engine/cache_persistence.go:266`,
     `pkg/engine/cache_persistence_helpers_test.go:9`,
     `pkg/engine/cache_persistence_test.go:92`,
     `pkg/engine/run_finalization_context_test.go:79`.

5. Architecture posture.
   - Finding: adding liveness tests to already-large test files tripped the
     repository architecture posture gate.
   - Fix: activation-cancellation and reload-entry-reconcile tests were split
     into focused files rather than growing the existing large files.
   - Evidence: `pkg/engine/work_lane_liveness_test.go:1`,
     `pkg/engine/reload_entry_reconcile_test.go:1`.

Intentional non-changes in this round:

1. Admin light status still omits heavyweight artifact detail.
   - Reason: the admin UI already fetches artifact state through the dedicated
     artifact endpoint, while default/light status is the cheap availability
     path. Re-adding heavyweight artifact detail to light status would
     contradict this SOW's web/watchdog availability purpose.

2. Failed async cache saves are surfaced, not retried in this follow-up.
   - Reason: accepted pending saves are drained on `Stop()`, direct synchronous
     saves return their error to `RunOnce`, and async failures remain visible in
     cache-persistence status. A retry policy would be a separate durability
     design choice because retries can reorder or amplify disk I/O under an I/O
     constrained VM.

Validation run after second reviewer follow-up:

- `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -run 'TestWorkLane|TestPublishFinalization|TestRunFinalization|TestRunOnceFinalPublicationUsesRunContext|TestSynchronousCachePersistenceErrorIsRunOnceError|TestCachePersistenceWorkerRecoversPanicAndAcceptsNextSave|TestCachePersistenceWorkerLoopPanicStopsWorkerAndWakesCallers|TestCachePersistenceWorkerCoalescesNewestAcceptedSnapshot|TestLatestSetCacheCloseAllDoesNotHoldLockWhileClosingSources|TestStoreSnapshotKeepsCachedSnapshotReadableWhilePersisting|TestServeRunServersReturnsErrorWhenServerGoroutinePanics|TestEntityArtifactPublishSyncsGeneratedFilesAfterReleasingPublishLease|TestRunServesHealthAndLightStatusWhileEngineRunBlocked|TestAdminStatusDefaultsToLightAndRequiresFullModeForDetailedSnapshot|TestAdminStatusLightRespondsWhileEngineLaneBusy|TestRunWatchdog|TestWatchdogSelfHealth|TestStartupEntityArtifacts|TestDroneBL|TestRecoveredCorruptDroneBLArtifactRequeuesNormalDownloaderFetch|TestDownloadWorkerPanicClearsActiveQueue' -count=1`
- `go test ./... -count=1`
- `go test ./tools/archposture -count=1`
- `git diff --check`
- `make build`
- `make test`
- `make staticcheck`
- `make golangci-lint`
- `make race`
- `go test ./... -count=1` from `tools/dronebl2ipsets`
- `pnpm --dir ui lint`
- `pnpm --dir ui build`

Validation result:

- All commands passed.
- The earlier targeted race failure was fixed by protecting the test hook and
  explicitly stopping async cache persistence workers in async-cache tests.
- `pnpm --dir ui build` emitted the existing Vite warnings about unresolved
  runtime font URLs and one large chunk. These are warnings, not failures, and
  were already present before this follow-up.

Artifact maintenance update:

- `AGENTS.md`: no project-wide rule change needed.
- Runtime project skills: no skill change needed.
- Specs: no additional spec change needed beyond the already-recorded
  operating-principles update in this SOW.
- End-user/operator docs: no additional docs change needed; this round fixed
  implementation liveness and test-race coverage.
- SOW lifecycle: this SOW remains `Status: in-progress` pending full external
  reviewer rerun and closure.

Next:

- Rerun the full-scope external reviewers with the same broad review scope.
- Fix, reject with evidence, or explicitly classify every new reviewer finding
  before closure.

### Implementation Progress - 2026-06-25 - V15 Reviewer Triage Follow-Up

Status: local validation clean after reviewer triage fixes; another full-scope
external reviewer rerun is required before closure.

Captured reviewer results from the full-scope rerun:

- GLM: production grade. It verified the watchdog start order, cache
  persistence condition-variable wait, no live integrity scans in GET
  handlers, DroneBL downloader-lane tests, and the three-lane contract.
- Qwen: production grade with low/medium notes. The admin status default-mode
  note was stale against the accepted contract: light status is now the default
  and full status requires `?mode=full`. The finalization test-hook mutex note
  was accepted and fixed.
- Deepseek: production grade with low notes. The unused light-status
  `artifacts` field was accepted and removed. The pprof-diagnostic suggestion
  was classified as non-blocking future instrumentation, not part of this
  deadlock fix.
- Kimi: not production grade. One high race finding was rejected with evidence
  because the alleged reload-swapped pointer reads are already under
  `Engine.mu.RLock()` on this baseline. The request-context inheritance finding
  was accepted and fixed.
- The harness did not retain durable output for two finished reviewer sessions
  after compaction. The next rerun will cover all six reviewers again.

Additional findings accepted and fixed:

1. Accepted asynchronous lane work must not inherit an HTTP request context.
   - Finding: `WorkLane.Submit()` used the caller context as the queued work
     parent unless the caller was already inside a lane worker. For admin HTTP
     actions, this meant accepted background work could start with a canceled
     request context after the response returned.
   - Fix: `Submit()` still uses the caller context for admission, but accepted
     work now uses the attached daemon context whenever one exists. If the lane
     is already marked shut down, the old `ErrLaneShuttingDown` post-shutdown
     contract is preserved.
   - Evidence: `pkg/engine/work_lane.go:235`,
     `pkg/engine/work_lane.go:246`,
     `pkg/engine/work_lane.go:436`,
     `pkg/engine/work_lane.go:444`.
   - Test: `TestWorkLaneSubmitUsesAttachedContextAfterAdmission` fills the
     only lane slot, accepts a queued item with a cancelable request context,
     cancels that request before the item starts, and proves the accepted item
     starts with a live context.
   - Evidence: `pkg/engine/work_lane_liveness_test.go:61`.

2. Finalization test hook needs race-detector-safe access.
   - Finding: the run-finalization test hook was a package-level function read
     during run finalization and written/restored by tests without a mutex.
   - Fix: the hook is now protected by a mutex, and production code copies the
     current hook under the mutex before calling it.
   - Evidence: `pkg/engine/run.go:76`,
     `pkg/engine/run.go:81`,
     `pkg/engine/run.go:93`.

3. Light status should not expose a dead artifact field.
   - Finding: `adminStatusLight` declared `artifacts` but never populated it.
     That suggested the default/light endpoint carried artifact details when
     those are intentionally served by `/api/v1/admin/artifacts`.
   - Fix: removed the unused field from the light-status response struct.
   - Evidence: `pkg/web/admin_status_light.go:12`,
     `pkg/web/admin_status_light.go:27`.

4. Reload race coverage should be harder to dismiss.
   - Finding: the existing public-reader/reload race test already covered the
     alleged reload-swapped pointer paths, but the reviewer considered the
     iteration count light.
   - Fix: increased reload cycles from 8 to 20 and reader cycles from 20 to 50.
   - Evidence: `pkg/engine/runtime_test.go:449`,
     `pkg/engine/runtime_test.go:506`.

Rejected reviewer finding:

- Alleged unsynchronized reload-swapped pointer reads in IP context and runtime
  ledger snapshots.
  - Evidence: `lookupContextSnapshot()` copies config, runtime, geo-provider
    cache, and ASN lookup cache under `Engine.mu.RLock()`.
  - Evidence: `pkg/engine/ip_context.go:295`,
    `pkg/engine/ip_context.go:299`.
  - Evidence: `runtimeLedgerSnapshot()` copies runtime and ledger cache under
    `Engine.mu.RLock()`.
  - Evidence: `pkg/engine/runtime_ledger_cache.go:32`,
    `pkg/engine/runtime_ledger_cache.go:36`.

Validation run after V15 triage fixes:

- `go test ./pkg/engine -run 'TestWorkLaneSubmitUsesAttachedContextAfterAdmission|TestWorkLaneSubmitCoalescesByKey|TestWorkLaneRunCanceledAtActivationDoesNotStart|TestRunFinalization' -count=1`
- `go test -race ./pkg/engine -run 'TestWorkLaneSubmitUsesAttachedContextAfterAdmission|TestWorkLaneSubmitCoalescesByKey|TestWorkLaneRunCanceledAtActivationDoesNotStart|TestRunFinalization|TestReloadConcurrentPublicRuntimeReadersRace' -count=1`
- `go test ./pkg/web -run 'TestAdminStatus|TestRunWatchdog|TestRunServesHealthAndLightStatusWhileEngineRunBlocked' -count=1`
- `go test -race ./pkg/engine -run 'TestReloadConcurrentPublicRuntimeReadersRace|TestWorkLaneSubmitUsesAttachedContextAfterAdmission|TestRunFinalization' -count=1`
- `go test ./pkg/engine -run 'TestWorkLaneAttachContextIsIdempotent|TestWorkLaneSubmitUsesAttachedContextAfterAdmission|TestWorkLaneSubmitFromWorkerUsesAttachedContextForContinuationShutdown' -count=1`
- `go test -race ./pkg/engine -run 'TestReloadConcurrentPublicRuntimeReadersRace|TestWorkLaneSubmitUsesAttachedContextAfterAdmission|TestWorkLaneAttachContextIsIdempotent|TestRunFinalization' -count=1`
- `go test ./pkg/engine ./pkg/web ./pkg/scheduler -count=1`
- `go test ./tools/archposture -count=1`

Validation result:

- All commands passed.
- The first broad package run exposed a post-shutdown error-contract mismatch
  introduced by the daemon-context inheritance fix. `Submit()` now maps a
  canceled attached context to `ErrLaneShuttingDown` when the lane is already
  marked shut down, and the focused plus broad package tests pass afterward.
- The first architecture posture run exposed that the new work-lane liveness
  test pushed `work_lane_test.go` over the large-file threshold. The test now
  lives in the focused `work_lane_liveness_test.go` file and the posture gate
  passes.

Artifact maintenance update:

- `AGENTS.md`: no project-wide rule change needed.
- Runtime project skills: no skill change needed.
- Specs: no additional spec change needed; this round preserves the existing
  daemon-context and light-status contracts already recorded in specs/docs.
- End-user/operator docs: no additional docs change needed; this was internal
  liveness and API payload cleanup.
- SOW lifecycle: this SOW remains `Status: in-progress` pending another
  full-scope external reviewer rerun.

Next:

- Rerun all six full-scope external reviewers on the new baseline.
- Fix, reject with evidence, or explicitly classify every new reviewer finding
  before closure.

### Implementation Progress - 2026-06-25 - V16 Final External Reviewer Rerun

Status: all six full-scope external reviewers returned `PRODUCTION GRADE` on
the new baseline. No blocking findings remain.

Reviewer results:

- GLM: production grade. Verified all eight fix notes, the three-lane contract,
  reload self-deadlock removal, watchdog ordering, deadline-bound notify,
  cache-first integrity handlers, git-lane separation, query/ASN/latest-set
  open-outside-lock behavior, and full local validation.
- Minimax: production grade. Verified all eight fix notes and V11 contract
  items. Reported only non-blocking hardening/cosmetic observations, classified
  below.
- Mimo: production grade. Verified all eight fix notes, cache-first integrity,
  SIGHUP/lane submission, runtime sampler behavior, DroneBL downloader FIFO,
  and validation gates.
- Kimi: production grade. Verified all eight fix notes, three-lane behavior,
  lock ordering, watchdog availability, admin integrity cache-first behavior,
  DroneBL downloader ownership, and test/race/lint gates.
- Qwen: production grade. Verified all eight fix notes, background task lock
  separation, systemd notification safety, scheduler trigger behavior,
  runtime-stats removal from production status paths, and build/test/race/lint
  gates.
- Deepseek: production grade. Verified all eight fix notes, engine/download/free
  lane contracts, integrity cache mutation coverage, and race-clean validation.

Reviewer-run validation evidence:

- `go build ./...`
- `make build`
- `make test`
- `make race`
- `make lint`
- `go vet ./...`
- `go test -race -count=1 ./pkg/systemd ./pkg/scheduler ./pkg/web ./pkg/engine`
- `go test ./pkg/engine ./pkg/web ./pkg/scheduler ./pkg/systemd -count=1`
- `go test ./tools/archposture -count=1`

Local validation after the final local test move:

- `git diff --check`
- `go test ./tools/archposture -count=1`
- `go test ./pkg/engine -run 'TestWorkLaneSubmitUsesAttachedContextAfterAdmission|TestWorkLaneAttachContextIsIdempotent' -count=1`
- `go test ./pkg/engine ./pkg/web ./pkg/scheduler -count=1`

Residual reviewer notes classified:

1. Pre-watchdog diagnostics on notify failure.
   - Classification: not a blocker and not deferred work for this SOW.
   - Evidence: watchdog notify calls are deadline-bound and errors are logged
     and counted in `sendRunWatchdogTick()`.
   - Evidence: `pkg/web/server_run.go:314`,
     `pkg/web/server_run.go:316`.
   - Evidence: the watchdog self-health goroutine emits capped, sanitized
     goroutine samples when heartbeat progress stalls.
   - Evidence: `pkg/web/liveness.go:76`,
     `pkg/web/liveness.go:105`.
   - Reason: adding a second diagnostic sample on every notify failure would be
     a logging/noise policy change, not a fix for the production deadlock class.

2. Alleged reverse lock order between run start and finalization.
   - Classification: rejected as stale against current code.
   - Evidence: `tryMarkRunStart()` releases `e.mu` before taking
     `activeFeedsMu` or `activeOperationsMu`.
   - Evidence: `pkg/engine/run.go:280`,
     `pkg/engine/run.go:295`,
     `pkg/engine/run.go:296`,
     `pkg/engine/run.go:299`.
   - Evidence: `markRunFinalizing()` clears active-feed and active-operation
     state before taking `e.mu`.
   - Evidence: `pkg/engine/run.go:417`,
     `pkg/engine/run.go:420`,
     `pkg/engine/run.go:424`.
   - Reason: no opposite lock acquisition order remains in this path.

3. Cache-persistence `Wait()` returns `errors.New(lastErr)`.
   - Classification: rejected as a current-SOW defect.
   - Evidence: the worker stores the last error as a status string for
     cross-goroutine snapshot/reporting state, and no caller relies on
     `errors.Is`/`errors.As` through that stored string.
   - Evidence: `pkg/engine/cache_persistence.go:130`,
     `pkg/engine/cache_persistence.go:135`.
   - Reason: changing the cache-persistence status model from string state to
     retained error values is an API/observability design choice, not a
     liveness or correctness requirement for this SOW.

4. Unknown admin status mode falls back to light status.
   - Classification: intentional safer default, not a defect.
   - Evidence: only `?mode=full` selects heavyweight full status; all other
     values use the cheap light status path.
   - Evidence: `pkg/web/admin.go:262`,
     `pkg/web/admin.go:264`.
   - Reason: rejecting unknown modes with HTTP 400 would be an API behavior
     change outside the production-deadlock fix. The current behavior favors
     the availability contract.

5. Duplicate daemon-control panic helpers.
   - Classification: non-blocking maintainability observation.
   - Reason: both helpers are small, bounded, and panic-recovery only. No
     reviewer found a behavioral issue or availability impact.

Artifact maintenance update:

- `AGENTS.md`: no project-wide rule change needed.
- Runtime project skills: no skill change needed.
- Specs: no additional spec change needed in this final reviewer pass.
- End-user/operator docs: no additional docs change needed in this final
  reviewer pass.
- SOW lifecycle: implementation and reviewer validation are complete. The SOW
  is ready for the normal close step: mark `Status: completed`, move it to
  `.agents/sow/done/`, and commit the implementation plus SOW move together.

### Gap Analysis Loop - 2026-06-25 - V17 Fresh Baseline

Status: fresh gap analysis found additional actionable work. The SOW must not
close yet.

Purpose of this loop:

- Repeat the deadlock/liveness gap analysis on the new baseline after the V16
  implementation-review pass.
- Validate every reviewer finding against current code.
- Prepare a concrete fix plan.
- Send the plan to external reviewers before implementation.

Reviewer results:

- `glm`: `FINDINGS`. Accepted blockers: a panic inside an admitted engine run
  can leave `e.running` / `run_state` wedged; entity refresh and integrity
  refresh callbacks can leave engine-level running/cache state wedged after a
  recovered panic; `RunOnce` publication/finalization currently happens outside
  the engine-lane slot; `WorkLane.Run` has an active-cancel spin edge.
- `kimi`: `NO REMAINING CRITICAL DEADLOCK/LIVENESS FINDINGS`. Accepted
  hardening items: runtime stats sampler goroutines lack panic recovery; git
  sync uses an unbounded value as its coalescing key. Rejected reload
  filesystem work as a liveness blocker because the current reload contract
  intentionally keeps config/runtime-dependent reload work synchronous while
  moving heavy cleanup to the engine lane.
- `qwen`: `FINDINGS`. Accepted item: git sync uses an unbounded coalescing key.
- `mimo`: `FINDINGS`. Accepted item: admin status mode routing contradicts the
  earlier compatibility contract. Missing `mode`, `mode=full`, and unknown
  values must return full status; only exact `mode=light` may return the light
  payload. Startup partial runtime stats in the light payload is non-blocking.
- `minimax`: `FINDINGS`. Accepted item: `WorkLane.Run` can spin after caller
  cancellation if the item is already active and the start notification remains
  unread. Rejected item: entity refresh double-submit race, because the code
  keeps `entityRefreshRunning` / `entityHealthRunning` true when pending work
  exists and only clears them on the no-pending branch.
- `deepseek`: `NO REMAINING DEADLOCK/LIVENESS FINDINGS`. Accepted observation
  for validation: direct `e.runtime` reads can race with `Engine.Reload()` writes
  because the engine lane does not serialize reload mutation. This is a
  concurrency correctness risk; it needs a race regression test and either a
  targeted fix or an evidence-backed rejection.

Validated accepted findings:

1. `RunOnce` panic/finalization wedge.
   - Evidence: `RunOnce` admits only `runOnceAdmitted()` through the engine lane,
     then runs `completeRunFinalization()` after `engineLane.Run()` returns.
   - Evidence: `pkg/engine/run.go:29`,
     `pkg/engine/run.go:36`,
     `pkg/engine/run.go:41`,
     `pkg/engine/run.go:44`.
   - Evidence: `runOnceAdmitted()` marks the run finalizing in a defer, but
     `running=false` is set only later in `markRunIdleAfterFinalization()`.
   - Evidence: `pkg/engine/run.go:141`,
     `pkg/engine/run.go:145`,
     `pkg/engine/run.go:339`,
     `pkg/engine/run.go:473`.
   - Evidence: if `runOnceAdmitted()` panics, `runLaneCallback()` recovers the
     panic and releases the lane slot, but the caller assignment to
     `finalization` never completes, so `completeRunFinalization()` is skipped.
   - Evidence: `pkg/engine/work_lane.go:606`,
     `pkg/engine/work_lane.go:609`.
   - Classification: accepted blocker. This can create a silent functional
     deadlock where the daemon and watchdog remain alive, but future engine runs
     are rejected as already running.

2. Entity refresh and integrity refresh panic state wedge.
   - Evidence: entity refresh sets `entityRefreshRunning=true` before submit and
     clears it only on explicit normal paths.
   - Evidence: `pkg/engine/entity_refresh_queue.go:166`,
     `pkg/engine/entity_refresh_queue.go:253`,
     `pkg/engine/entity_refresh_queue.go:325`,
     `pkg/engine/entity_refresh_queue.go:425`.
   - Evidence: entity health refresh has the same shape.
   - Evidence: `pkg/engine/entity_refresh_queue.go:184`,
     `pkg/engine/entity_refresh_queue.go:301`,
     `pkg/engine/entity_refresh_queue.go:371`,
     `pkg/engine/entity_refresh_queue.go:431`.
   - Evidence: background task visibility uses `defer task.Finish()`, but does
     not recover panic and convert it to an error that the caller can use to
     clear engine-level flags.
   - Evidence: `pkg/engine/background_tasks.go:130`,
     `pkg/engine/background_tasks.go:137`.
   - Evidence: integrity refresh callbacks set cache state to running and settle
     only after the scan returns normally.
   - Evidence: `pkg/engine/integrity_cache.go:187`,
     `pkg/engine/integrity_cache.go:189`,
     `pkg/engine/integrity_cache.go:254`,
     `pkg/engine/integrity_cache.go:259`.
   - Classification: accepted blocker. The lane slot is released after panic,
     but the engine-level admission/cache state can remain permanently
     `running`.

3. Admin status mode compatibility regression.
   - Evidence: the current handler returns full status only for `mode=full` and
     light status for missing or unknown modes.
   - Evidence: `pkg/web/admin.go:262`,
     `pkg/web/admin.go:265`.
   - Evidence: the SOW contract says missing `mode`, `mode=full`, and unknown
     values continue to use full status; only exact `mode=light` selects the
     lightweight builder.
   - Evidence: `.agents/sow/current/SOW-0117-20260621-bounded-work-lanes-watchdog-availability.md:873`,
     `.agents/sow/current/SOW-0117-20260621-bounded-work-lanes-watchdog-availability.md:876`,
     `.agents/sow/current/SOW-0117-20260621-bounded-work-lanes-watchdog-availability.md:878`.
   - Classification: accepted correctness/compatibility bug. The V16 residual
     classification that treated unknown/default light status as intentional is
     superseded by this revalidation.

4. Git sync coalescing key is unbounded.
   - Evidence: `gitSyncWork()` formats `git-sync:%d` from a monotonic sequence
     and uses it as both `ID` and `CoalescingKey`.
   - Evidence: `pkg/engine/metadata.go:49`,
     `pkg/engine/metadata.go:54`,
     `pkg/engine/metadata.go:60`.
   - Classification: accepted low-severity SOW-contract bug. `Run()` does not
     currently use coalescing, so this is not a present liveness bug, but
     coalescing keys must be finite and stable.

5. `WorkLane.Run` active-cancel spin edge.
   - Evidence: after caller context cancellation, if the item is no longer
     queued, the code unlocks and loops back to a select where `ctx.Done()`
     remains ready.
   - Evidence: `pkg/engine/work_lane.go:197`,
     `pkg/engine/work_lane.go:222`,
     `pkg/engine/work_lane.go:230`.
   - Evidence: the start notification is buffered and normally already ready by
     the time the loop starts, so this is expected to be short-lived, not an
     indefinite deadlock.
   - Classification: accepted low-severity liveness/performance hardening.

6. Runtime stats sampler panic hardening.
   - Evidence: the web runtime sampler goroutine and engine runtime sampler
     goroutine do not recover panics.
   - Evidence: `pkg/web/sysinfo.go:137`,
     `pkg/web/sysinfo.go:143`,
     `pkg/engine/run_diagnostics.go:444`,
     `pkg/engine/run_diagnostics.go:450`.
   - Classification: accepted low-severity hardening. A sampler panic would not
     deadlock the daemon, but it would silently stop useful status telemetry.

7. Runtime reload data-race risk.
   - Evidence: `Engine.ReloadContext()` writes `e.runtime` under `e.mu`.
   - Evidence: `pkg/engine/engine.go:284`,
     `pkg/engine/engine.go:287`.
   - Evidence: active run code reads `e.runtime` fields directly without holding
     `e.mu`.
   - Evidence: `pkg/engine/run.go:164` plus broad direct runtime reads found by
     `rg -n "\be\\.runtime\b" pkg/engine --glob '*.go'`.
   - Classification: accepted for test-first validation. This is not proven to
     be the production deadlock source, but it is a concurrency correctness risk
     in the same reload/SIGHUP surface.

Rejected or non-blocking findings:

- Entity refresh double-submit race: rejected. The reviewed code clears
  `entityRefreshRunning` / `entityHealthRunning` only when `pending == 0`;
  when pending work exists the flag remains true until continuation submit.
  Evidence: `pkg/engine/entity_refresh_queue.go:249`,
  `pkg/engine/entity_refresh_queue.go:252`,
  `pkg/engine/entity_refresh_queue.go:259`,
  `pkg/engine/entity_refresh_queue.go:297`,
  `pkg/engine/entity_refresh_queue.go:300`,
  `pkg/engine/entity_refresh_queue.go:307`.
- Reload filesystem work outside the engine lane: rejected as a SOW-0117
  blocker. The reload contract explicitly keeps config/runtime-dependent
  synchronous reload work in `Engine.Reload()` while moving heavy cleanup to the
  engine lane. Evidence: this SOW's `Reload And SIGHUP Contract` section.
- Light-status partial runtime stats during the first sampler interval:
  non-blocking. Light status is allowed to omit memory/runtime details or use a
  cached sampler.

Fix plan for external review:

1. Tests first.
   - Add an engine-run panic regression test that injects a panic after
     `tryMarkRunStart()` and proves the run returns `ErrLanePanic`, status
     returns to idle, and a second `RunOnce` can start.
   - Update finalization tests that currently assert the engine lane is free
     during blocked finalization; those tests protect the unsafe behavior. New
     tests must assert the engine lane remains occupied until publication and
     finalization finish, or explicitly assert the approved replacement contract.
   - Add entity refresh and entity health panic regression tests proving
     `entityRefreshRunning` / `entityHealthRunning` clear and later refresh
     submissions can start.
   - Add pipeline and entity integrity refresh panic regression tests proving
     the cache state settles to an error/stale/cold state and does not remain
     `RefreshRunning`.
   - Add admin status mode tests for missing mode, `mode=full`, `mode=light`,
     and unknown mode.
   - Add a git sync work test proving `CoalescingKey` is finite/stable while
     `ID` may remain unique for observability.
   - Add a WorkLane active-cancel regression test that forces cancellation after
     activation and proves `Run()` does not spin or hang.
   - Add a reload/run race regression test under `-race`. If it reproduces the
     `e.runtime` race, fix the race in this loop.

2. Implementation plan.
   - Make `runOnceAdmitted()` use named returns and panic-safe defers so a panic
     after the run is marked started becomes an `ErrLanePanic` run error and
     still produces a finalization object or clears the run state.
   - Move `completeRunFinalization()` execution into the engine-lane callback,
     so the whole `RunOnce` lifecycle is one engine-lane item. This intentionally
     replaces the earlier split-finalization behavior because the fresh gap
     analysis showed it can preserve the same heavy-overlap window this SOW was
     created to eliminate.
   - Make `completeRunFinalization()` itself panic-safe and guarantee
     `markRunIdleAfterFinalization()` runs exactly once for a started run.
   - Refactor `withEngineLaneBackgroundTask()` to recover panics, record failed
     task telemetry, finish task visibility, and return an `ErrLanePanic` error
     to callers.
   - Wrap pipeline/entity integrity refresh callbacks with named-return defers
     that settle cache state even when a scan panics.
   - Change admin status mode routing so only exact `mode=light` selects light
     status; missing, `mode=full`, and unknown values use full status.
   - Keep git sync `ID` unique if useful for visibility, but change
     `CoalescingKey` to a stable bounded value such as `git-sync:publish`.
   - Adjust `WorkLane.Run` so after cancellation of an active item it waits for
     the already-scheduled start notification instead of reselecting on an
     already-closed `ctx.Done()`.
   - Add panic recovery to web and engine runtime sampler goroutines.
   - For the reload/runtime race: first prove it with the new race test. If
     proven, prefer a focused runtime/config snapshot or lane-admitted reload
     mutation fix that preserves the documented reload contract and does not
     hold `e.mu` around heavy work.

3. Validation plan after implementation.
   - Focused new tests for every accepted finding.
   - `go test ./pkg/engine ./pkg/web ./pkg/scheduler -count=1`.
   - `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -count=1`.
   - `go test ./tools/archposture -count=1`.
   - `make test`, `make race`, and UI build/lint only if touched surfaces or
     elapsed time permit before the next reviewer loop.

Next required step:

- Run all six external reviewers on this V17 fix plan before implementation.
  They must verify whether the plan fixes the accepted liveness bugs, whether
  moving finalization back inside the engine lane is the right long-term-best
  choice, and whether the reload/runtime race should be fixed in this loop or
  split into a separate focused concurrency SOW.

### Plan Review - 2026-06-25 - V18 Supersedes V17 Implementation Plan

Status: V17 plan review found that the accepted findings are real, but the
proposed implementation plan needed correction before code changes.

User decision - 2026-06-25:

- Approved implementing the recommendations.
- This loop uses the surgical option for `RunOnce`: preserve the earlier
  approved finalization-outside-engine-lane contract, and fix the panic cleanup
  wedge without moving publication/finalization back inside the lane.
- Rationale: finalization-outside-lane was deliberately approved earlier in
  this SOW so slow publish/git/cache work does not block unrelated engine-lane
  work. The panic wedge is real, but it can be fixed without reversing that
  availability decision.

Reviewer consensus:

- All reviewed agents agreed the `RunOnce` panic wedge, entity refresh panic
  wedge, integrity refresh panic wedge, admin status mode mismatch, git sync
  bounded-key issue, sampler panic hardening, and `WorkLane.Run` active-cancel
  edge are valid findings.
- Multiple reviewers rejected treating finalization-inside-lane as an
  implementation detail. They identified it as a contract reversal of the
  earlier V14/V12 decision and recommended either explicit fresh approval or a
  narrower cleanup fix.
- The chosen implementation is the narrower cleanup fix. This avoids changing
  the application contract and keeps existing finalization availability tests
  valid.

Superseded V17 item:

- Superseded: "Move `completeRunFinalization()` execution into the engine-lane
  callback."
- Replacement: keep `completeRunFinalization()` after `engineLane.Run()`, but
  make `runOnceAdmitted()` always convert panics after a started run into an
  `ErrLanePanic` return and a non-nil finalization object so
  `completeRunFinalization()` and `markRunIdleAfterFinalization()` still run.

Tests to add or update:

- Add `TestRunOncePanicDoesNotWedgeRunningFlag`: inject a panic after
  `tryMarkRunStart()` and verify `RunOnce()` returns an `ErrLanePanic` error,
  status returns to idle, and a second `RunOnce()` is not rejected as already
  running.
- Preserve existing finalization availability tests:
  `TestRunFinalizationReleasesLaneBeforeCachePersistenceSubmit`,
  `TestPublishFinalizationDoesNotHoldEngineLane`, and
  `TestBlockedCachePersistenceDoesNotBlockRunOrEngineLane`.
- Add `TestEngineLaneBackgroundTaskPanicFinishesTaskAndReturnsError`: verify a
  panic in a background task becomes an `ErrLanePanic` error, records failure
  state, and leaves no visible background task.
- Add entity refresh and entity health panic tests: verify panic recovery clears
  `entityRefreshRunning` / `entityHealthRunning` when there is no pending work
  and later submissions can start.
- Add pipeline/entity integrity refresh panic tests: verify refresh callbacks
  settle cache state with an error instead of remaining `RefreshRunning`.
- Update admin status mode test: missing mode, `mode=full`, and unknown modes
  return full status; only exact `mode=light` returns light status.
- Add `TestGitSyncWorkUsesStableCoalescingKey`: keep unique work IDs if useful,
  but make the coalescing key finite and stable.
- Add or update `WorkLane.Run` cancellation test: cancellation after activation
  must return promptly without starting canceled work.
- Add sampler recovery tests where practical. If panic injection requires too
  much production-only hook surface, implement same-goroutine recovery and rely
  on focused code review plus existing sampler population/idempotence tests.
- Add a reload/runtime race test only as a scoped validation gate. If the fix
  requires broad runtime snapshot changes across many engine files, split it to
  a focused follow-up SOW instead of expanding this liveness cleanup loop.

Implementation plan:

1. Add test hooks only where necessary for deterministic panic injection. Hooks
   must be package-local, used only by tests, and must not change production
   behavior when nil.
2. Update/add the tests above and run the focused package tests to observe the
   expected failures before the behavior fixes.
3. Change `runOnceAdmitted()` to use named returns and a panic-safe defer:
   - before run admission, panics remain ordinary lane panics;
   - after `tryMarkRunStart()` succeeds, panics become `ErrLanePanic` run
     errors;
   - the returned finalization object is non-nil for started runs, so
     post-lane finalization clears `e.running` and `run_state`.
4. Change `completeRunFinalization()` to use a panic-safe defer that guarantees
   `markRunIdleAfterFinalization()` and the `engine.running` gauge update run
   exactly once for a valid finalization.
5. Change `withEngineLaneBackgroundTask()` to recover panics, finish task
   visibility, record failed task telemetry, and return `ErrLanePanic`.
6. Change entity refresh/health queue loops to rely on the recovered error path
   and always evaluate pending/clear-running state after task errors.
7. Change pipeline/entity integrity refresh callbacks to settle cache state in
   a defer even when the scan panics.
8. Change admin status mode routing so only exact `mode=light` selects the
   lightweight payload.
9. Change git sync work to use stable bounded `CoalescingKey`.
10. Change `WorkLane.Run` so canceled active items wait for their already
    scheduled start notification instead of repeatedly reselecting on the
    closed caller context.
11. Add sampler panic recovery around the web and engine runtime sampler
    goroutines.
12. Run focused tests, then race tests for touched packages.

Runtime reload race scope decision:

- Keep the reload/runtime race as test-first validation in this SOW, not as a
  broad mechanical rewrite by default.
- If a focused test proves a race and the fix is small, fix it here.
- If the fix requires sweeping `e.runtime` read replacement across many engine
  files or a new runtime snapshot contract, create a dedicated follow-up SOW and
  do not hide that broader design change inside this liveness cleanup loop.
- Follow-up created: `.agents/sow/pending/SOW-0118-20260625-runtime-reload-snapshot-race.md`.
  Reason: the direct-read inventory spans many engine files, while
  `go test -race ./pkg/engine ./pkg/web -count=1` passed on the current
  coverage. The finding is valid enough to preserve, but too broad to bundle
  with the owner-layer panic cleanup.

Validation plan:

- `go test ./pkg/engine ./pkg/web -count=1`.
- Focused `go test` patterns for the new tests.
- `go test -race ./pkg/engine ./pkg/web -count=1`.
- `go test ./tools/archposture -count=1` if large engine/web test files are
  touched.
- Run external reviewers again after the fix chunk is complete.

### Implementation Update - 2026-06-25 - V18 Fix Chunk

Implemented:

- `RunOnce` owner-layer panic cleanup:
  - `runOnceAdmitted()` now uses named returns and a panic-safe defer.
  - Panics after a run is admitted become an `ErrLanePanic` run error.
  - Started runs still return a non-nil finalization object, so post-lane
    finalization clears `e.running` and `run_state`.
  - `completeRunFinalization()` now has a panic-safe defer that always calls
    `markRunIdleAfterFinalization()` for valid finalizations.
  - The earlier finalization-outside-engine-lane contract was preserved.

- Background task panic cleanup:
  - `withEngineLaneBackgroundTask()` now recovers panics, returns
    `ErrLanePanic`, finishes visible task state, and records failed task
    telemetry.

- Entity refresh and health refresh panic cleanup:
  - Added deterministic panic hooks after pending work drains.
  - Existing queue loops now clear running state after recovered task errors
    when no pending work remains.

- Integrity refresh panic cleanup:
  - Pipeline and entity integrity refresh callbacks now settle cache state from
    a defer, including panic paths.

- Admin status compatibility:
  - Only exact `mode=light` selects lightweight admin status.
  - Missing mode, `mode=full`, and unknown modes use full status.

- Bounded git sync key:
  - Git sync work keeps unique IDs but uses stable bounded
    `CoalescingKey: "git-sync:publish"`.

- Work-lane active-cancel hardening:
  - `WorkLane.Run()` now consumes the already-scheduled start notification for
    active canceled items instead of looping on a closed caller context.

- Runtime stats sampler hardening:
  - Engine and web runtime samplers now recover per sample refresh path.
  - Focused tests prove a one-shot panic does not prevent a later valid sample
    from updating cached state.

- Runtime reload race follow-up:
  - Created `.agents/sow/pending/SOW-0118-20260625-runtime-reload-snapshot-race.md`
    because the direct `e.runtime` read inventory is broad and the package race
    gate passed with current coverage.

Tests added or updated:

- `TestRunOncePanicDoesNotWedgeRunningFlag`
- `TestEngineLaneBackgroundTaskPanicFinishesTaskAndReturnsError`
- `TestEntityArtifactRefreshPanicClearsRunningFlag`
- `TestEntityHealthRefreshPanicClearsRunningFlag`
- `TestPipelineIntegrityRefreshPanicSettlesCache`
- `TestEntityIntegrityRefreshPanicSettlesCache`
- `TestAdminStatusModeSelectsLightOnlyForExactLightMode`
- `TestGitSyncWorkUsesStableCoalescingKey`
- `TestEngineRuntimeStatsSamplerRecoversPanic`
- `TestRuntimeStatsSamplerRecoversPanic`

Validation evidence:

- Focused regression suite:
  `go test ./pkg/engine ./pkg/web -run 'TestRunOncePanicDoesNotWedgeRunningFlag|TestEngineLaneBackgroundTaskPanicFinishesTaskAndReturnsError|TestEntityArtifactRefreshPanicClearsRunningFlag|TestEntityHealthRefreshPanicClearsRunningFlag|TestPipelineIntegrityRefreshPanicSettlesCache|TestEntityIntegrityRefreshPanicSettlesCache|TestAdminStatusModeSelectsLightOnlyForExactLightMode|TestGitSyncWorkUsesStableCoalescingKey|TestEngineRuntimeStatsSamplerRecoversPanic|TestRuntimeStatsSamplerRecoversPanic|TestWorkLaneRunCanceledAtActivationDoesNotStart' -count=1`
  passed.
- Broader package tests:
  `go test ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed.
- Race tests:
  `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed.
- Architecture posture:
  `go test ./tools/archposture -count=1` passed.
- Patch hygiene:
  `git diff --check` passed.

Next required step:

- Run external reviewers against the V18 fix chunk and this SOW before closing
  or committing.

### Implementation Update - 2026-06-25 - V18 Reviewer Follow-Up

External reviewer finding:

- Minimax found a valid owner-layer panic gap in the newly added engine-lane
  diagnostics goroutine. The goroutine logged long-held engine-lane work from a
  ticker, but the diagnostic tick did not have its own panic recovery at the
  time of review. This was the same class of liveness problem the SOW is fixing:
  a periodic daemon-control goroutine could die silently after a panic.

Fix implemented:

- Added `logLongRunningEngineLaneWorkSafely()` around each engine-lane
  diagnostics tick.
- The safe wrapper recovers panics from the diagnostic path, increments a local
  telemetry counter, logs a bounded recovery message, and protects the recovery
  log path from a second logger panic.
- Added `TestEngineLaneDiagnosticsRecoversPanic`, which forces the diagnostics
  logger to panic once, verifies the recovery log, and proves a later diagnostic
  tick still emits the long-held-work warning.

Additional validation evidence:

- `go test ./pkg/engine -run 'TestEngineLaneDiagnosticsLogsLongHeldSlot|TestEngineLaneDiagnosticsRecoversPanic' -count=1`
  passed.
- `go test ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed after the
  diagnostics recovery fix.
- `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed after
  the diagnostics recovery fix.
- GLM reviewer completed after the fix was visible and returned
  `PRODUCTION GRADE`.
- Minimax reviewer completed after the fix was visible and returned
  `PRODUCTION GRADE`.

Next required step:

- Run a clean final external reviewer pass against the patched V18 baseline, so
  the close decision is based on the current worktree rather than mixed
  pre-fix reviewer output.

### Implementation Update - 2026-06-25 - V18 Final Reviewer Follow-Up

External reviewer findings:

- Reviewers found one real entity refresh owner-layer gap: panics after
  `withEngineLaneBackgroundTask()` returned, but before the refresh loop cleared
  `entityRefreshRunning` / `entityHealthRunning`, could still leave the queue
  marked as running.
- Reviewers found a WorkLane FIFO/diagnostic consistency issue: sequence numbers
  were allocated before the enqueue lock was acquired, so concurrent callers
  could receive IDs that did not match actual enqueue order.
- Reviewers found a defense-in-depth gap in WorkLane finish-panic recovery: if
  the recovery path itself panicked, the error path did not have its own
  secondary guard.
- Reviewers found a very low-probability `RunOnce` finalization-state panic
  gap: if the panic recovery defer itself panicked while calling
  `markRunFinalizing()`, `completeRunFinalization()` could be skipped and
  `e.running` could remain set.

Fixes implemented:

- Added panic-only owner-layer cleanup to queued entity artifact refresh and
  queued entity health refresh. If code outside the background-task wrapper
  panics, the running flag is cleared and the panic is rethrown for WorkLane's
  callback recovery.
- Added post-task panic regression hooks and behavioral tests for both entity
  refresh queues.
- Moved WorkLane item sequence/ID allocation into the same mutex-protected
  section that enqueues or activates the item. Existing test-only construction
  remains available through the wrapper.
- Added a secondary recovery guard around `recoverFinishPanic()`, so the lane
  returns an `ErrLanePanic` error instead of recursively panicking if recovery
  work itself fails.
- Added nested recovery inside `runOnceAdmitted()`'s finalization defer, and
  extended `markRunIdleAfterFinalization()` to clear the same started run even
  if `markRunFinalizing()` failed before switching the run state to
  `finalizing`.
- Added `TestRunOnceFinalizingPanicDoesNotWedgeRunningFlag`.

Additional validation evidence:

- `go test ./pkg/engine -run 'TestRunOncePanicDoesNotWedgeRunningFlag|TestRunOnceFinalizingPanicDoesNotWedgeRunningFlag|TestEntityArtifactRefreshPanicClearsRunningFlag|TestEntityArtifactRefreshPostTaskPanicClearsRunningFlag|TestEntityHealthRefreshPanicClearsRunningFlag|TestEntityHealthRefreshPostTaskPanicClearsRunningFlag|TestWorkLaneFinishPanicReleasesSlotForLaterWork|TestWorkLaneFinishPanicAfterLockDoesNotDeadlock|TestWorkLaneRunCanceledAtActivationDoesNotStart' -count=1`
  passed.
- `go test ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed after
  this reviewer follow-up.
- `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed after
  this reviewer follow-up.
- `go test ./tools/archposture -count=1` passed after this reviewer follow-up.
- `go vet ./pkg/engine ./pkg/web ./pkg/scheduler` passed after this reviewer
  follow-up.
- `git diff --check` passed after this reviewer follow-up.
- `go test ./... -count=1` passed after this reviewer follow-up.

Next required step:

- Run a clean final external reviewer pass against this updated V18 baseline.

### Final External Review - 2026-06-25 - V18 Patched Baseline

External reviewer consensus:

- GLM returned `PRODUCTION GRADE`.
- Minimax returned `PRODUCTION GRADE`.
- Mimo returned `PRODUCTION GRADE`.
- Kimi returned `PRODUCTION GRADE`.
- Qwen returned `PRODUCTION GRADE`.
- Deepseek returned `PRODUCTION GRADE`.

Non-blocking reviewer maintenance findings addressed:

- One reviewer found that the coalescing-key contract still described generic
  `entity:refresh:feed_updates` and `entity:refresh:health` keys, while the
  implementation deliberately alternates `:continuation:0` and
  `:continuation:1` suffixes. This SOW's coalescing-key list now documents the
  actual continuation key convention and its reason: preventing a queued
  continuation from coalescing with the lane item still completing.
- One reviewer found that `markRunEnd()` had no production callers and only a
  test caller. The helper was removed so future code cannot accidentally bypass
  the explicit `markRunFinalizing()` / `markRunIdleAfterFinalization()` flow.
  The test now uses the idle-finalization cleanup path directly.
- One reviewer found that queued entity refresh callbacks returned `nil` to
  WorkLane even when the real refresh task returned a non-panic error. The
  queue workers now return accumulated task errors after queue state is settled,
  while keeping the existing bounded wave and continuation behavior. Added
  regression coverage for feed-update and health-transition refresh errors.

Non-blocking reviewer findings explicitly accepted or split:

- `entityArtifactFullRebuildQueuedOrRunning()` reads rebuild queued,
  background-task, and lane-snapshot state under separate locks. This is an
  advisory fast-path guard only; the real admission guard for full rebuilds is
  `tryMarkEntityArtifactFullRebuildQueued()`. The accepted worst case is an
  unnecessary surgical refresh attempt, not duplicate full rebuild admission or
  unbounded work.
- A reviewer suggested that worker-submitted continuations could bypass daemon
  cancellation. Production uses `AttachWorkLaneContext()` from the web server,
  so `submitParentContext()` returns the attached daemon context before the
  worker-context fallback. Existing coverage
  `TestWorkLaneSubmitFromWorkerUsesAttachedContextForContinuationShutdown`
  proves an inner submitted item is canceled by the attached root context.
- Light admin status still reads `Engine.Config()` and `Engine.Runtime()` via
  the existing unprotected snapshot accessors. This is the broader runtime
  reload race inventory already split to
  `.agents/sow/pending/SOW-0118-20260625-runtime-reload-snapshot-race.md`; it
  remains non-blocking for this liveness SOW.

Additional reviewer validation evidence:

- Reviewers independently reran and reported passing focused panic tests.
- Reviewers independently reran and reported passing
  `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -count=1`.
- Reviewers independently reran and reported passing `go test ./... -count=1`.
- Reviewers independently reran and reported passing `go vet ./pkg/engine ./pkg/web ./pkg/scheduler`.
- Local focused regression suite after the entity refresh error propagation
  fix:
  `go test ./pkg/engine -run 'TestQueuedEntityArtifactRefreshReturnsTaskErrorToLane|TestQueuedEntityHealthRefreshReturnsTaskErrorToLane|TestEntityArtifactRefreshPanicClearsRunningFlag|TestEntityHealthRefreshPanicClearsRunningFlag|TestEntityArtifactRefreshPostTaskPanicClearsRunningFlag|TestEntityHealthRefreshPostTaskPanicClearsRunningFlag' -count=1`
  passed.
- Local package suite after the entity refresh error propagation fix:
  `go test ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed.

Reviewer conclusion:

- The V18 panic-wedge cleanup is production-grade.
- Finalization remains outside the engine lane.
- WorkLane FIFO/coalescing/cancel behavior is preserved.
- Admin status defaults to the lightweight payload, and full status requires
  explicit `?mode=full`.
- SOW-0118 is a valid pending split for the broader runtime reload snapshot/race
  issue and does not block this SOW.

### Implementation Update - 2026-06-25 - V19 Reviewer Follow-Up

Additional reviewer findings:

- GLM found a real low-risk WorkLane accounting bug: caller-provided duplicate
  `LaneWork.ID` values could collide in the active map when the lane limit was
  above one and the items had different coalescing families. The visible work ID
  is operator metadata and is not guaranteed unique, so it must not be the
  internal active-slot key.
- A later reviewer noted that delayed publish-stage cleanup still used the old
  `cleanup.publish_stages` key even though this SOW's queue contract lists
  `cleanup:publish_stages:delayed`.

Fixes implemented:

- WorkLane active-slot bookkeeping now uses an internal monotonically allocated
  `activeKey` stored on each lane item. The public `LaneWork.ID` remains
  unchanged for snapshots, tickets, logs, and operator visibility.
- Added `TestWorkLaneDuplicateExplicitIDsDoNotShareActiveSlot` in a focused
  test file so this regression stays covered without growing the already-large
  WorkLane test file past the architecture baseline.
- Delayed publish-stage cleanup now uses the canonical
  `cleanup:publish_stages:delayed` coalescing key. This is a contract alignment:
  the current delayed cleanup path is one-shot and blocking through `Run()`, so
  the key is not relied on for duplicate suppression in that path.

Validation evidence:

- `go test ./pkg/engine -run 'TestWorkLaneDuplicateExplicitIDsDoNotShareActiveSlot|TestWorkLaneLimitTwoRunsTwoJobs|TestWorkLaneFinishPanicReleasesSlotForLaterWork|TestWorkLaneFinishPanicAfterLockDoesNotDeadlock|TestWorkLaneRunPanicReturnsErrorAndReleasesSlot|TestWorkLaneSubmitPanicReleasesSlotForQueuedWork' -count=1`
  passed.
- `go test ./pkg/engine -run 'TestQueuedEntityArtifactRefreshReturnsTaskErrorToLane|TestQueuedEntityHealthRefreshReturnsTaskErrorToLane|TestEntityArtifactRefreshPanicClearsRunningFlag|TestEntityHealthRefreshPanicClearsRunningFlag|TestEntityArtifactRefreshPostTaskPanicClearsRunningFlag|TestEntityHealthRefreshPostTaskPanicClearsRunningFlag' -count=1`
  passed.
- `go test ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed.
- `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed.
- `go test ./... -count=1` passed.
- `go vet ./pkg/engine ./pkg/web ./pkg/scheduler` passed.
- `go test ./tools/archposture` passed.
- `git diff --check` passed.

Next required step:

- Run a clean final external reviewer pass against the V19 patched baseline.

### Implementation Update - 2026-06-25 - V20 Reviewer Follow-Up

User decision recorded:

- The user approved implementing the safer liveness recommendations on
  2026-06-25. This approval includes making default admin status lightweight
  and requiring explicit `?mode=full` for the detailed payload. The tradeoff is
  accepted: default-light is better for the production liveness contract, while
  external admin clients that relied on full fields from the default route must
  request `?mode=full`.

Additional reviewer findings:

- GLM found a real cancellation gap in the ASN lookup cache: a second caller
  waiting on an in-flight load for the same ASN provider did not observe its
  request context until the first load completed.
- GLM found that `observeEnginePhaseCurrent(RunPhaseUnknown)` still ran while
  the engine mutex was held in the finalization/idle transition helpers.
- Kimi found two contract gaps on the reviewed baseline:
  admin status defaulted to the full snapshot unless callers explicitly passed
  `mode=light`, and heavy engine phases still had production
  `setCache.Open(...)` calls without the run context.
- Mimo reported a `statusSnapshot()` lock-hold concern, but local evidence
  rejected it for the current code: `statusSnapshot()` unlocks before calling
  `currentRunMetrics()` and `lifetimeMetricsSnapshot()`.
- The previous Minimax reviewer run timed out and produced only partial output,
  so it is treated as inconclusive for this baseline.

Fixes implemented:

- Added context-aware ASN cache acquisition. Same-provider waiters now select
  on the in-flight load's completion channel and the caller context, and close
  an opened database if cancellation wins before publication into the cache.
- Added `LookupIPContextContext(ctx, ip)` and moved public search enrichment to
  the request context. The existing `LookupIPContext(ip)` API remains as a
  compatibility wrapper.
- Moved phase-gauge clearing outside the engine mutex in both finalization and
  idle transition paths.
- Changed admin status semantics so default and unknown modes return the light
  snapshot; only explicit `?mode=full` returns the detailed full snapshot.
- Converted production heavy-phase latest-set opens in metadata comparison,
  geo, ASN, bogon, critical-infrastructure, entity feed sidecar, and live
  entity detail paths to `OpenContext(ctx, ...)`.
- Added context variants for live country/ASN detail helpers while keeping the
  old exported methods as compatibility wrappers.

Validation evidence:

- `go test ./pkg/engine -run 'TestASNDatabaseCache|TestWorkLaneDuplicateExplicitIDsDoNotShareActiveSlot|TestRunOnceFinalizingPanicDoesNotWedgeRunningFlag|TestRunOncePanicDoesNotWedgeRunningFlag' -count=1`
  passed.
- `go test ./pkg/engine -run 'TestQueuedEntityArtifactRefreshReturnsTaskErrorToLane|TestQueuedEntityHealthRefreshReturnsTaskErrorToLane|TestEntityArtifactRefreshPanicClearsRunningFlag|TestEntityHealthRefreshPanicClearsRunningFlag|TestEntityArtifactRefreshPostTaskPanicClearsRunningFlag|TestEntityHealthRefreshPostTaskPanicClearsRunningFlag' -count=1`
  passed.
- `go test ./pkg/web -run 'TestAdminStatusDefaultsToLightAndRequiresFullModeForDetailedSnapshot|Test.*Search' -count=1`
  passed.
- `rg -n "setCache\\.Open\\(" pkg/engine --glob '!**/*_test.go'` returned no
  production matches.
- `go test ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed.
- `go vet ./pkg/engine ./pkg/web ./pkg/scheduler` passed.
- `go test ./tools/archposture` passed.
- `git diff --check` passed.
- `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed.
- `go test ./... -count=1` passed.
- `make build` passed.
- `make lint` passed.
- `make race` passed.
- `make test-strict` passed.

Current context-background classification:

- Remaining `context.Background()` hits are compatibility wrappers,
  daemon/server root or shutdown contexts, telemetry/reporting contexts, nil
  context fallbacks, or already-split runtime reload snapshot work in
  `.agents/sow/pending/SOW-0118-20260625-runtime-reload-snapshot-race.md`.
- This classification must be rechecked by the final V20 external reviewer
  pass before the SOW can close.

Next required step:

- Run a clean final external reviewer pass against the V20 patched baseline.

### Implementation Update - 2026-06-25 - V21 Reviewer Follow-Up

External reviewer consensus on V20:

- GLM, Minimax, Mimo, Kimi, Qwen, and Deepseek all returned
  `PRODUCTION GRADE` for the stated liveness/deadlock contract.
- Multiple reviewers found the same SOW consistency issue: earlier plan text
  still said missing/unknown admin status modes should return the full payload
  unless explicitly approved. That is superseded by the V20 user decision above:
  default admin status is lightweight and full status requires `?mode=full`.
- Reviewers also found two minor non-blocking context propagation gaps in live
  entity detail helpers, plus one non-blocking entity integrity scan gap where
  lane cancellation was not passed into the scan API.

Fixes implemented:

- Updated the active admin status requirement to match the approved contract:
  default/missing/unknown/light modes use the lightweight payload, and only
  explicit `?mode=full` returns the full payload.
- Threaded caller context through live country/ASN detail overlap walks:
  `countryFilteredRangeSource`, `countCountriesForASNSource`, and
  `countCountryASNJointSource`.
- Added `CheckEntityArtifactsIntegrityContext(ctx)` and made lane-owned entity
  integrity refresh/repair paths pass the lane/request context into the scan.
  The original `CheckEntityArtifactsIntegrity()` remains a compatibility
  wrapper.
- Added cancellation checks at entity integrity scanner phase boundaries and
  inside feed/country/ASN/health loops.
- Added `TestCheckEntityArtifactsIntegrityContextHonorsCanceledContext`.

Validation evidence:

- `go test ./pkg/engine -run 'TestCheckEntityArtifactsIntegrityContextHonorsCanceledContext|TestCheckEntityArtifactsIntegrityFlagsMissingCountryPublicJSON|TestCheckEntityArtifactsIntegrityRepairsMissingHomeAggregate|TestCheckEntityArtifactsIntegrityFlagsStaleHomeAggregate|TestCheckEntityArtifactsIntegrityFlagsMalformedHomeAggregate|TestCheckEntityArtifactsIntegrityFlagsHealthTransitionDrift|TestCountryFilteredRangeSourcePropagatesSourceErrors' -count=1`
  passed.
- `go test ./pkg/engine -run 'TestASNDatabaseCache|TestQueuedEntityArtifactRefreshReturnsTaskErrorToLane|TestQueuedEntityHealthRefreshReturnsTaskErrorToLane|TestWorkLaneDuplicateExplicitIDsDoNotShareActiveSlot' -count=1`
  passed.
- `go test ./pkg/web -run 'TestAdminStatusDefaultsToLightAndRequiresFullModeForDetailedSnapshot|Test.*Search' -count=1`
  passed.
- `rg -n "WalkRangeOverlapsContext\\(context\\.Background\\(\\)|setCache\\.Open\\(" pkg/engine --glob '!**/*_test.go'`
  returned no production matches.
- `go test ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed.
- `go vet ./pkg/engine ./pkg/web ./pkg/scheduler` passed.
- `go test ./tools/archposture` passed.
- `git diff --check` passed.
- `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed.
- `go test ./... -count=1` passed.
- `make build` passed.
- `make lint` passed.
- `make test-strict` passed.
- `make race` passed.

Next required step:

- Run a final clean external reviewer pass against the V21 patched baseline.

### Implementation Update - 2026-06-25 - V22 Reviewer Follow-Up

External reviewer result on V21:

- Minimax, Mimo, Qwen, and Deepseek returned `PRODUCTION GRADE`.
- Kimi produced a `PRODUCTION GRADE` conclusion in partial output, but the
  process timed out, so it is treated as inconclusive instead of a clean pass.
- GLM timed out, but its partial output contained two valid findings that were
  fixed in this update:
  - A race detector report showed `download_stage.go` reading `cache.Entry.Name`
    directly while config reload could write the same field through
    `Entry.ApplyProcessingSourceConfig()`.
  - Repeated race runs exposed a flaky integrity-cache ordering bug:
    if a lane refresh panicked and settled very quickly, the caller-side
    queue-ticket write could run afterward and move the same work ID back to
    `refresh_running`.

Fixes implemented:

- Replaced direct `cache.Entry.Name` reads in the downloader result paths with
  `entry.Snapshot().Name`, matching the cache snapshot contract used elsewhere.
- Made pipeline and entity integrity queue-state updates monotonic for the same
  work ID: once a work item has settled into a terminal state, a late queued or
  active ticket for that same work ID is ignored.
- Cleared pipeline and entity integrity lane tickets on both successful and
  failed settlement, so a panic/error settlement cannot retain stale active
  ticket metadata.
- Added deterministic tests for the late queue-ticket ordering in both
  integrity caches, in addition to the panic-settlement tests that exercise the
  lane path.

Validation evidence:

- `go test ./pkg/engine -run 'Test(Pipeline|Entity)Integrity.*(PanicSettlesCache|LateQueuedStateDoesNotOverrideSettledWork)' -count=50`
  passed.
- `go test -race ./pkg/engine -run 'Test(Pipeline|Entity)Integrity.*(PanicSettlesCache|LateQueuedStateDoesNotOverrideSettledWork)' -count=30`
  passed.
- Ten repeated runs of
  `go test -race ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed.
- `go test ./pkg/engine ./pkg/web ./pkg/scheduler -count=1` passed.
- `go vet ./pkg/engine ./pkg/web ./pkg/scheduler` passed.
- `go test ./tools/archposture` passed.
- `git diff --check` passed.
- `go test ./... -count=1` passed.
- `make build` passed.
- `make lint` passed.
- `make test-strict` passed.
- `make race` passed.

Next required step:

- Run a clean final external reviewer pass against the V22 patched baseline.

### Implementation Update - 2026-06-25 - V23 Reviewer Follow-Up

External reviewer result on V22:

- GLM returned `PRODUCTION GRADE` and reported only low-severity polish items.
- Minimax and Kimi verified the V22 fixes, but found one actionable defensive
  hardening item: `recoverFinishPanic()` released the lane slot, but did not
  itself cancel the lane item's context if a future call path failed to cancel
  after `finishItemSafely()`.
- Multiple reviewers reported stale SOW lifecycle text: the original outcome
  still said "closed", and the regression pre-implementation gate still said
  `needs-user-decision` even though the user approved the recommended path and
  the implementation has proceeded through V20-V23.
- Reviewers also found low-risk cleanup items that were cheap to address:
  a dead admin rebuild handler wrapper using `context.Background()`, an
  idempotent-but-noisy missing `RefreshQueued` guard in integrity cache queued
  setters, and a missing defensive warning when a lane-admitted run cannot mark
  itself started.
- One reviewer found a real cancellation gap: finalization history observation
  used `context.Background()` for history ledger reloads. That path can now
  honor the run/finalization context.

Fixes implemented:

- `recoverFinishPanic()` now cancels the lane item context inside the recovery
  path before emitting metrics or starting follow-up work. The existing caller
  cancellation remains harmlessly idempotent.
- Added `TestWorkLaneFinishPanicCancelsItemContext`.
- Added `observeHistoryPointContext(ctx, ...)`, switched the production
  finalize path to pass the run context, and kept `observeHistoryPoint(...)` as
  a compatibility wrapper.
- Added `TestObserveHistoryPointContextHonorsCanceledContext`.
- Added `IntegrityCacheRefreshQueued` to the same-workID no-op branch in both
  pipeline and entity integrity queued-state setters.
- Removed the unused `handleAdminEntityIntegrityRebuild()` wrapper; routes and
  tests now use `handleAdminEntityIntegrityRebuildWithContext(...)`.
- Added a warning log for the defensive path where the engine lane admits a run
  but `tryMarkRunStart()` reports that another run is already active.
- Updated stale SOW lifecycle text so the original closed outcome is clearly
  historical and the regression pre-implementation gate is marked approved and
  implemented.

Reviewer findings rejected with evidence:

- Entity artifact publication no longer holds the entity artifact generation
  lock across git sync or integrity stale marking. `publishEntityArtifactMutationPlan()`
  explicitly releases the publication lease before `syncGeneratedFiles(...)`
  and `MarkIntegrityCachesStale()`.
- Entity refresh continuations do not depend on a raw `context.Background()`
  parent in production. Continuations pass the lane context, and
  `submitParentContext()` prefers the daemon context attached by
  `AttachWorkLaneContext(...)`.
- Admin rebuild routes already used the daemon-context handler; the removed
  wrapper was dead code and only created audit noise.

Validation evidence:

- `go test ./pkg/engine -run 'TestWorkLaneFinishPanic|TestObserveHistoryPointContextHonorsCanceledContext|Test(Pipeline|Entity)Integrity.*LateQueuedStateDoesNotOverrideSettledWork' -count=20`
  passed.
- `go test -race ./pkg/engine -run 'TestWorkLaneFinishPanic|TestObserveHistoryPointContextHonorsCanceledContext|Test(Pipeline|Entity)Integrity.*LateQueuedStateDoesNotOverrideSettledWork' -count=20`
  passed.
- `go test ./pkg/engine ./pkg/web ./pkg/scheduler ./pkg/systemd -count=1`
  passed.
- `go vet ./pkg/engine ./pkg/web ./pkg/scheduler ./pkg/systemd` passed.
- `go test ./tools/archposture` passed with `pkg/engine/work_lane.go` at 799
  lines, below the large-file threshold.
- `git diff --check` passed.
- `go test ./... -count=1` passed.
- `make build` passed.
- `make lint` passed.
- `make test-strict` passed.
- `make race` passed.

Next required step:

- Run a clean final external reviewer pass against the V23 patched baseline.

### Implementation Update - 2026-06-25 - V24 Reviewer Follow-Up

External reviewer result on V23:

- Reviewers broadly confirmed the watchdog/deadlock fixes are implemented:
  deadline-bound systemd notify, watchdog self-health diagnostics, light admin
  status, narrowed status/reload locking, scheduler admission timeouts, lane
  shutdown hardening, and panic-safe integrity/entity refresh settlement.
- GLM found one actionable correctness/race issue: the V22 downloader fix
  changed read-side `entry.Snapshot().Name` usage, but `cache.State.RenameEntry()`
  still wrote `Entry.Name` without taking the entry mutex that `Snapshot()`
  uses. A concurrent config rename and snapshot could race on the string field.
- Qwen found the same mutable-name pattern in the processing path:
  `beginFeedAttempt()` read `entry.Name` directly before marking the run
  started.
- Several stale reviewer findings were rejected with code evidence:
  systemd notify is already deadline-bound; watchdog self-health diagnostics
  already exist; status snapshots already copy scalar state under `e.mu` and
  release it before reading active-operation, background-task, lane, and
  telemetry state; reload filesystem work and lane limit updates run outside
  the exclusive engine mutex; scheduler trigger admission already has
  context-bounded variants.
- Remaining broad direct `e.runtime` snapshot races are intentionally tracked
  in `.agents/sow/pending/SOW-0118-20260625-runtime-reload-snapshot-race.md`
  and are not part of this SOW closure.

Fixes implemented:

- `cache.State.RenameEntry()` now takes the entry-level lock before mutating
  `Entry.Name`, matching the `Entry.Snapshot()` read-side contract.
- Added `TestRenameEntryConcurrentSnapshot` in a dedicated focused test file so
  race validation covers concurrent rename/snapshot without growing the already
  large cache test file.
- `beginFeedAttempt()` now captures the feed name through `entry.Snapshot().Name`
  instead of directly reading the mutable field.
- Added `TestBeginFeedAttemptConcurrentEntryConfigUpdate` to cover concurrent
  processing-attempt naming while config metadata updates use the entry lock.

Validation evidence:

- `go test ./pkg/cache ./pkg/engine -run 'TestRenameEntryConcurrentSnapshot|TestBeginFeedAttemptConcurrentEntryConfigUpdate' -count=20`
  passed.
- `go test -race ./pkg/cache ./pkg/engine -run 'TestRenameEntryConcurrentSnapshot|TestBeginFeedAttemptConcurrentEntryConfigUpdate' -count=20`
  passed.
- `go test ./tools/archposture -count=1` passed after moving the cache rename
  test to `pkg/cache/cache_rename_race_test.go`.
- Minimax also reran `go test ./... -count=1 -timeout 600s` and
  `go test -count=1 -race ./pkg/engine ./pkg/web ./pkg/scheduler ./pkg/systemd -timeout 600s`
  successfully after the V24 fix.

Next required step:

- Run a clean final external reviewer pass against the V24 patched baseline.

### Implementation Update - 2026-06-25 - V25 Final Reviewer Follow-Up

External reviewer result on V24:

- GLM, Minimax, Mimo, Kimi, and Qwen all verified the V24 cache-entry/name
  race fixes and judged the SOW-0117 liveness contract production-grade or
  production-grade with non-blocking follow-up items.
- One reviewer returned stale negative findings that matched older code, not
  the current baseline. Examples rejected with source evidence: systemd notify
  is deadline-bound, watchdog self-health diagnostics exist, `AttachContext()`
  is duplicate-guarded, `syncStart` notifications are emitted after releasing
  the lane mutex, admin status defaults to light mode, and SOW-0118 exists in
  `.agents/sow/pending/`.
- The final pass still exposed five real contract gaps worth fixing in this
  SOW before closure:
  - engine-lane long-hold diagnostics used the one-minute progress interval
    instead of the SOW's named 30-second default;
  - long-hold diagnostics were visible through logs/counters but not through a
    lane/admin status snapshot field;
  - scheduler activity snapshots could hold `Runner.stateMu` while calling
    engine config helpers that acquire `Engine.mu`;
  - `Engine.IsProviderDatabase()` read `e.cfg` without taking `Engine.mu`;
  - `detailedStatusCached()` returned fallback runtime values by calling
    `runtime.NumGoroutine()` and `runtimeinfo.GoMemLimit()` before the sampler
    had populated the cache.

Fixes implemented:

- Set `engineLaneDiagnosticsInterval` and `engineLaneLongHoldThreshold` to a
  named 30-second default and added `TestEngineLaneLongHoldThresholdDefault`.
- Added `LaneLongHoldWarning` and `LaneSnapshot.long_hold_warning`, recorded
  recent long-held engine-lane work when diagnostics fire, and attached that
  warning to `StatusSnapshotLight()`/`StatusSnapshot()` lane snapshots.
- Added `SchedulerConfigSnapshot()` on the engine. It copies only the scheduler
  facts needed by queue/status paths while holding `Engine.mu`: provider
  database names, source parent lists, and artifact names.
- Changed scheduler `ActivitySnapshot()`, `ActivitySnapshotLight()`,
  `startNextDownload()`, and `queueStatusLookup()` to use the copied scheduler
  config facts outside `Runner.stateMu`. The scheduler no longer calls
  `eng.Config()` or `eng.IsProviderDatabase()` while holding its state lock for
  these activity/status paths.
- Made `Engine.IsProviderDatabase()` take `Engine.mu.RLock()` before reading
  `e.cfg`.
- Changed `detailedStatusCached()` so an empty cache returns uptime plus
  `disk_free:"unknown"` and zero runtime counters. It no longer samples runtime
  state synchronously on the light status path before the sampler has produced
  its first cached sample.

Tests added:

- `TestSchedulerConfigSnapshotCopiesSchedulerFacts`
- `TestEngineLaneLongHoldThresholdDefault`
- `TestActivitySnapshotLightUsesCopiedConfigFacts`
- `TestActivitySnapshotLightConcurrentConfigReload`
- `TestDetailedStatusCachedDoesNotCaptureRuntimeBeforeSampler`
- Extended `TestEngineLaneDiagnosticsLogsLongHeldSlot` to assert that the
  warning is visible through the light admin/status lane snapshot.

Validation evidence:

- `go test ./pkg/engine ./pkg/scheduler ./pkg/web -run 'TestEngineLaneDiagnosticsLogsLongHeldSlot|TestEngineLaneLongHoldThresholdDefault|TestActivitySnapshotLightUsesCopiedConfigFacts|TestActivitySnapshotLightConcurrentConfigReload|TestDetailedStatusCachedDoesNotCaptureRuntimeBeforeSampler|TestSchedulerConfigSnapshotCopiesSchedulerFacts' -count=20`
  passed.
- `go test -race ./pkg/engine ./pkg/scheduler ./pkg/web -run 'TestEngineLaneDiagnosticsLogsLongHeldSlot|TestEngineLaneLongHoldThresholdDefault|TestActivitySnapshotLightUsesCopiedConfigFacts|TestActivitySnapshotLightConcurrentConfigReload|TestDetailedStatusCachedDoesNotCaptureRuntimeBeforeSampler|TestSchedulerConfigSnapshotCopiesSchedulerFacts' -count=10`
  passed.
- `go test ./pkg/engine ./pkg/scheduler ./pkg/web ./pkg/systemd ./pkg/cache -count=1`
  passed.
- `go test -race ./pkg/engine ./pkg/scheduler ./pkg/web ./pkg/systemd ./pkg/cache -count=1 -timeout 600s`
  passed.
- `go test ./tools/archposture -count=1` passed with `pkg/engine/work_lane.go`
  still at 799 lines.
- `git diff --check` passed.
- `go test ./... -count=1` passed.
- `make build` passed.
- `make lint` passed.
- `make test-strict` passed.
- `make race` passed.
- `make test` passed.
- `pnpm --dir ui build` passed with the existing Vite font-resolution and
  chunk-size warnings only.
- `pnpm --dir ui lint` passed.

Residual follow-up:

- The broad direct `e.runtime`/`e.cfg` audit remains tracked in
  `.agents/sow/pending/SOW-0118-20260625-runtime-reload-snapshot-race.md`.
  This V25 pass fixed the status/scheduler/provider helper issues found by
  reviewers but did not convert every engine-internal direct runtime/config
  read; that larger audit is the SOW-0118 scope.

### Gap Analysis Loop - 2026-06-26 - V26 Fresh Baseline

External reviewer result:

- GLM, Minimax, Mimo, Kimi, Qwen, and DeepSeek all returned
  `GAP ANALYSIS CLEAN` for the current post-V25 baseline.
- No reviewer found a remaining SOW-0117 deadlock/liveness/availability
  blocker.
- Multiple reviewers independently verified the same key contracts:
  - default admin status is the light path;
  - integrity GET handlers read cached snapshots rather than live-scanning;
  - engine-lane and downloader-lane work remain separate;
  - DroneBL recovered artifacts are materialized by downloader workers;
  - `Runner.stateMu` is not held while reading engine config facts in activity
    snapshots;
  - `detailedStatusCached()` no longer samples runtime state before the sampler
    has populated the cache;
  - SOW-0118 correctly tracks the broader runtime reload/config snapshot race
    audit and is not a current SOW-0117 deadlock blocker.

Validated reviewer notes:

- One reviewer reported three missing light-status tests. Local validation found
  partial equivalent coverage already existed:
  - `TestDetailedStatusCachedDoesNotCaptureRuntimeBeforeSampler` covered the
    empty-cache fallback;
  - `TestAdminStatusModeSelectsLightOnlyForExactLightMode` and
    `TestAdminStatusLightRespondsWhileEngineLaneBusy` covered default light
    mode and lane visibility.
- The same reviewer was correct that the SOW still listed explicit light-status
  acceptance names and that the integrity-summary path deserved direct backend
  coverage. This was accepted as test hardening rather than a production code
  blocker.
- The reviewer also suggested deleting a redundant DroneBL test. That suggestion
  was rejected: both `TestDroneBLChildrenMaterializeInDownloadWorker` and
  `TestRecoveredDroneBLArtifactMaterializesInDownloadWorker` are explicitly
  listed in this SOW as acceptance evidence, even though they intentionally
  share a helper.

Tests added:

- `TestAdminStatusLightUsesRuntimeStatsSampler` proves the light admin status
  reads the cached runtime stats sample.
- `TestAdminStatusLightIncludesEngineLane` proves the light admin status exposes
  typed engine-lane state, `max_engine_lane_workers`, and compatibility aliases.
- `TestAdminStatusLightIncludesIntegritySummary` proves the light admin status
  exposes typed pipeline and entity integrity cache summaries.

Validation evidence:

- `go test ./pkg/web -run 'TestAdminStatusModeSelectsLightOnlyForExactLightMode|TestAdminStatusLightUsesRuntimeStatsSampler|TestAdminStatusLightIncludesFeedHealthSummary|TestAdminStatusLightIncludesEngineLane|TestAdminStatusLightIncludesIntegritySummary|TestAdminStatusLightUsesCachedSchedulerSnapshotWithoutRebuild|TestAdminStatusLightRespondsWhileEngineLaneBusy' -count=20`
  passed.
- `go test -race ./pkg/web -run 'TestAdminStatusModeSelectsLightOnlyForExactLightMode|TestAdminStatusLightUsesRuntimeStatsSampler|TestAdminStatusLightIncludesEngineLane|TestAdminStatusLightIncludesIntegritySummary|TestAdminStatusLightRespondsWhileEngineLaneBusy' -count=10`
  passed.
- `go test ./pkg/web -count=1` passed.
- `go test -race ./pkg/web -count=1 -timeout 300s` passed.
- `go test ./pkg/engine ./pkg/scheduler ./pkg/web ./pkg/systemd ./pkg/cache -count=1`
  passed.
- `git diff --check` passed.

Current conclusion:

- The required repeat gap-analysis loop did not find any remaining SOW-0117
  deadlock/liveness blocker to fix.
- The only remaining related work is the already-split SOW-0118 runtime
  reload/config snapshot race audit.

### Gap Analysis Loop - 2026-06-26 - V27 Full-Scope Rerun

External reviewer result:

- GLM, Minimax, Mimo, Kimi, Qwen, and DeepSeek all returned
  `GAP ANALYSIS CLEAN` for the current baseline after the explicit light-status
  tests were added.
- No reviewer found a remaining SOW-0117 deadlock/liveness/availability
  blocker.
- Minimax was rerun because the first session ended before a final verdict could
  be collected. The replacement full-scope run returned `GAP ANALYSIS CLEAN`.

Validated reviewer notes:

- The V27 reviewers accepted the three explicit light-status tests as sufficient
  evidence for their stated contracts:
  - `TestAdminStatusLightUsesRuntimeStatsSampler`
  - `TestAdminStatusLightIncludesEngineLane`
  - `TestAdminStatusLightIncludesIntegritySummary`
- One reviewer briefly explored whether `WorkLane.Run()` could deadlock during
  the cancel/start race, then rejected that theory after tracing the
  `LaneWorkQueued` removal path and the activated-item `syncStart` notification
  path. The lane race subset passed under `-race`.
- Reviewers continued to identify SOW-0118 as the correct scope for broader
  runtime reload/config snapshot race hardening. That follow-up is not a
  current SOW-0117 deadlock/liveness blocker.

Additional validation independently run by the replacement Minimax review:

- `go test ./pkg/web -run 'TestAdminStatusLightUsesRuntimeStatsSampler' -count=1`
  passed.
- `go test ./pkg/web -run 'TestAdminStatusLightIncludesEngineLane' -count=1`
  passed.
- `go test ./pkg/web -run 'TestAdminStatusLightIncludesIntegritySummary' -count=1`
  passed.
- `go test ./pkg/web -run 'TestAdminStatusModeSelectsLightOnlyForExactLightMode' -count=1`
  passed.
- `go test -race ./pkg/web -run 'TestAdminStatusLightUsesRuntimeStatsSampler|TestAdminStatusLightIncludesEngineLane|TestAdminStatusLightIncludesIntegritySummary|TestAdminStatusModeSelectsLightOnlyForExactLightMode' -count=2`
  passed.
- `go test -race ./pkg/web -count=1 -timeout 300s` passed.
- `go test -race ./pkg/engine -count=1 -run 'TestWorkLaneRun|TestWorkLaneSubmit|TestWorkLaneShutdown|TestEngineLaneDiagnosticsLogsLongHeldSlot|TestEngineLaneDiagnosticsRecoversPanic' -timeout 120s`
  passed.
- `go test -race ./pkg/engine -count=1 -run 'TestPipelineIntegrity|TestEntityIntegrity' -timeout 120s`
  passed.
- `go test -race ./pkg/scheduler -count=1 -run 'TestActivitySnapshot|TestSchedulerConfig|TestDroneBL' -timeout 120s`
  passed.

Current conclusion:

- The repeat gap-analysis loop has now produced two clean full-scope passes
  after the latest fixes: V26 and V27.
- There is no remaining SOW-0117 deadlock/liveness blocker identified by local
  validation or external review.
- The next related work is SOW-0118, the separately tracked runtime
  reload/config snapshot race audit.
