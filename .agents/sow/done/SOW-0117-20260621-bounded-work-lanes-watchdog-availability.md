# SOW-0117 - Bounded Work Lanes And Watchdog Availability

## Status

Status: completed

Sub-state: implementation, review remediation, validation, and SOW closure complete; ready for commit.

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
    locks around heavy work. Add a lightweight admin status contract for frequent
    UI polling, implemented as the query parameter
    `GET /api/v1/admin/status?mode=light` on the existing admin status route.
    The same handler branches on `mode=light` and calls a lightweight builder;
    do not add a separate route unless a later compatibility decision explicitly
    changes this. Missing `mode`, `mode=full`, and any unrecognized mode value
    continue to use the existing full-status builder for backward compatibility;
    only the exact value `light` selects the lightweight builder. The admin UI
    must use that light path for high-frequency polling and while the engine
    lane is active. The existing full admin status route may remain for
    compatibility, but it must not be the hot polling path during heavy work.
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
      `entity:refresh:feed_updates` for queued feed-update refresh waves;
      `entity:refresh:health` for queued health-transition refresh waves;
      `entity:repair:startup` and `entity:repair:operator` or equivalent
      trigger-scoped keys for entity repair;
      `integrity:pipeline:refresh` for pipeline integrity refresh scans;
      `integrity:entity:refresh` for entity integrity refresh scans;
      `entity:integrity:reload` for SIGHUP entity checks;
      `cleanup:critical_infrastructure:reload` for reload critical-infrastructure
      cleanup; and `cleanup:publish_stages:delayed` for delayed publish-stage
      cleanup. Implementations may add narrower suffixes, but they must not
      coalesce incompatible work families.
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

## Outcome

Implementation, external implementation review, reviewer follow-up fixes, and
local validation are complete. The SOW is closed.

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

None yet.

Append regression entries here only after this SOW was completed or closed and
later testing or use found broken behavior. Use a dated `## Regression -
YYYY-MM-DD` heading at the end of the file. Never prepend regression content
above the original SOW narrative.
