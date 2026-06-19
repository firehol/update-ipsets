# SOW-0105 - Production Unresponsiveness Diagnosis

## Status

Status: in-progress

Sub-state: diagnosis complete enough to proceed; user accepted a goal-by-goal
resource-control process. Goal 1 scheduler, comparison, and entity
missing-sidecar slices are implemented locally, validated, and reviewed cleanly
by the final external-review iteration. Goal 2 first-slice context/cancellation
and runtime-validation contracts are implemented locally, validated, and
externally reviewed cleanly. Goal 1 and Goal 2 first-slice returns are now
negligible unless new production evidence contradicts the current findings; the
SOW remains open for Goal 3 comparison-generation optimization. Goal 3
third-gate review found all six external reviewers ready for implementation
after the all-pair ledger lookup, full-ledger replacement, and production-impact
boundary clarifications were added. Goal 3 first implementation slice is now
implemented locally and validated. First external implementation review found
one accepted test-coverage blocker; the missing tests were added and validated.
Second external implementation review returned six `PRODUCTION GRADE` verdicts
with no blockers. Goal 3 first-slice returns are now limited to already
deferred later-slice work unless production evidence shows a remaining
comparison-specific resource problem. New 2026-06-19 production evidence shows
the engine can spend hours in source retention work before watchdog termination;
the next slice is diagnostic/accounting instrumentation for source, phase, feed,
and retention progress before any retention algorithm optimization.

## Requirements

### Purpose

Diagnose why the managed production-candidate install becomes unresponsive
enough for systemd watchdog failures while it also serves the public website and
APIs. The output of this SOW is a concrete, evidence-backed remediation plan
that preserves end-user functionality and prevents the public site from becoming
unresponsive under background ingest, processing, and artifact-maintenance load.

### User Request

The user rejected speculative explanations and requested a professional
diagnosis SOW that:

- records working theories as hypotheses to verify, not conclusions;
- uses subagents to inspect what the application actually does;
- uses existing production telemetry, including Netdata metrics collected for
  the host in Netdata Cloud;
- uses production logs and runtime evidence from the installed service;
- produces concrete answers and actionable next steps;
- avoids generic recommendations such as "add telemetry" or "change this or
  that" without evidence.

On 2026-06-19, after additional crash evidence, the user accepted a
goal-by-goal remediation process:

- define focused goals such as stopping unnecessary work, improving required
  work performance, optimizing retention, and other bounded follow-ups;
- for each goal, perform local analysis first;
- run `glm`, `minimax`, `kimi`, `mimo`, `deepseek`, and `qwen` against the same
  goal for independent read-only analysis;
- merge, validate, and weight findings by evidence, not vote count;
- implement only the accepted findings for that goal;
- review implementation with the same reviewer set;
- iterate per goal until remaining returns are negligible;
- preserve functional requirements and output semantics;
- allow the engine to run less frequently only when this does not make any feed
  run less frequently than its configured update cadence.

The user explicitly rejected treating public/worker split as the solution to
the root problem. Split/static serving may be a later deployment hardening
pattern, but the primary solution must make the backend resource model bounded,
predictable, and non-crashing.

### Assistant Understanding

Facts:

- The public website and public APIs run in the same daemon process as the
  scheduler/background work.
- The managed install uses systemd `Type=notify`, `WatchdogSec=300`,
  `MemoryHigh=1536M`, `MemoryMax=2G`, and `GOMEMLIMIT=1536MiB`
  (`install.sh:367`, `install.sh:384`, `install.sh:427`).
- The watchdog goroutine sends a timer-based systemd notification at half the
  systemd watchdog interval (`pkg/web/server_run.go:221`,
  `pkg/systemd/notify.go:40`).
- Systemd notification uses a Unix datagram dial/write per notification
  (`pkg/systemd/notify.go:52`).
- The HTTP public/admin servers are built and served in the same process
  (`pkg/web/server_run.go:43`, `pkg/web/server_run.go:252`).
- The processing loop drains all pending processing items into one batch and
  always calls `RunOnce` with `Reprocess: true` once work is admitted
  (`pkg/scheduler/processing_loop.go:29`, `pkg/scheduler/processing_loop.go:44`).
- After a processing batch, entity-artifact refresh is queued for
  `report.EntityRefreshTargets` or all updated feeds (`pkg/scheduler/processing_loop.go:103`).
- Entity refresh coalesces pending feed names and processes them through
  `withEntityArtifactMutation` (`pkg/engine/entity_refresh_queue.go:46`,
  `pkg/engine/entity_refresh_queue.go:240`).
- Background-only work is limited by `max_background_workers`, currently set to
  `1` in the default runtime configuration (`configs/firehol/runtime.yaml:86`).
- The current service on the production-candidate host has already had
  systemd watchdog restarts after the SOW-0103 install; cgroup evidence must be
  collected and recorded in this SOW before concluding why.

Inferences:

- The watchdog failure is evidence that the process stopped sending systemd
  watchdog notifications for the watchdog window. It is not, by itself, proof
  that the public website was reachable or unreachable during the full window.
- Because public serving and background processing share one process, CPU
  starvation, cgroup memory throttling, lock contention, blocked syscalls, or
  heavy file I/O in background paths may also affect public HTTP latency.
- The correct next step is read-only diagnosis, not remediation.

Unknowns:

- Which exact runtime phase was active during each watchdog failure.
- Whether watchdog misses correlate more strongly with CPU saturation,
  cgroup memory high throttling, disk I/O pressure, Go runtime GC/assist time,
  blocked systemd notification calls, web-handler lock contention, or a
  combination.
- Whether public API endpoints were slow because they performed heavy work, or
  because they were indirectly starved by shared process resources.
- Whether Netdata Cloud has enough historical resolution for the incident
  windows; this must be verified.

### Acceptance Criteria

- Every working theory listed in this SOW is classified as `confirmed`,
  `refuted`, or `unresolved-needs-specific-measurement`, with evidence.
- Production evidence includes concrete timestamps around each watchdog failure,
  phase/activity logs, cgroup memory events, CPU, I/O, PSI or equivalent
  pressure metrics if available, and public/admin HTTP responsiveness evidence
  if available.
- Code evidence identifies the exact functions/files that can create the
  observed resource pressure or contention.
- Netdata Cloud evidence is queried and summarized without storing bearer
  tokens, credentials, customer data, or raw sensitive data in the repository.
- The final action plan names specific changes, affected files/modules,
  expected CPU/memory/latency impact, validation method, and user-visible
  behavior guarantee for each item.
- No code, config, or operational change is made under this SOW before the
  diagnosis evidence supports it and the active goal has a goal-specific
  implementation gate, baseline validation, and output-equivalence plan.

## Analysis

Sources checked at SOW creation:

- `install.sh`
- `configs/firehol/runtime.yaml`
- `pkg/web/server_run.go`
- `pkg/systemd/notify.go`
- `pkg/scheduler/processing_loop.go`
- `pkg/engine/entity_refresh_queue.go`
- `pkg/engine/background_tasks.go`
- SOW-0097 and SOW-0103 current state

Current state:

- SOW-0097 and SOW-0103 reduced several known resource risks, but the
  production-candidate service still had watchdog restarts. Therefore SOW-0103
  is not enough evidence that the application is production-safe.
- The current code has telemetry and status surfaces, but this SOW must first
  use the existing signals before proposing any new instrumentation.
- The service is both an ingest processor and a public API/site server. A
  watchdog miss is operationally severe because an unresponsive daemon can also
  make the public service unavailable.

Risks:

- False root cause: optimizing the wrong phase can preserve the watchdog
  failure and hide the real production blocker.
- Functional regression: reducing work incorrectly can change published feeds,
  comparisons, retention, entity artifacts, or public API results.
- Operational regression: treating watchdog failures as a monitoring problem can
  leave the public website unavailable under load.
- Sensitive data exposure: Netdata Cloud tokens, environment secrets, and raw
  production details must not be written to durable artifacts.

### Evidence Collected - 2026-06-15

Evidence was collected read-only from:

- the production-candidate service's systemd unit and cgroup files;
- the service's systemd journal namespace;
- the public/admin HTTP status endpoints;
- Netdata Cloud metrics for the host and service;
- code review of the scheduler, public routes, entity refresh, comparison,
  retention, publication, and systemd watchdog paths;
- six read-only subagent workstreams.

No code, config, service, restart, install, or daemon state change was made. A
temporary status JSON capture was written under `/tmp` on the host for parsing
and removed after use.

#### Production Incident Timeline

After the latest install boundary, the service had watchdog restarts and no
post-install OOM kill was found.

Incident 1:

- `2026-06-14 20:35:10 UTC`: markdown pages generated for 391 feeds.
- `2026-06-14 20:35:23 UTC`: run finished with `updated=402`, `skipped=0`,
  `failed=0`, elapsed `23m26.172s`.
- `2026-06-14 20:35:23 UTC`: processing batch completed with `updated=402`,
  `skipped=0`, `failed=0`.
- `2026-06-14 20:35:23 UTC`: entity artifact refresh queued with `feeds=177`,
  `pending=177`, trigger `scheduled_due`.
- `2026-06-14 20:40:01 UTC`: systemd watchdog timeout at `5min`.
- `2026-06-14 20:41:32 UTC`: systemd killed the service after abort timeout;
  result `watchdog`.
- `2026-06-14 20:42:27 UTC`: service restarted.

Incident 2:

- `2026-06-15 04:50:37 UTC`: markdown pages generated for 391 feeds.
- `2026-06-15 04:50:50 UTC`: run finished with `updated=402`, `skipped=0`,
  `failed=0`, elapsed `28m19.288s`.
- `2026-06-15 04:50:50 UTC`: processing batch completed with `updated=402`,
  `skipped=0`, `failed=0`.
- `2026-06-15 04:50:50 UTC`: entity artifact refresh queued with `feeds=202`,
  `pending=202`, trigger `scheduled_due`.
- `2026-06-15 04:59:58 UTC`: systemd watchdog timeout at `5min`.
- `2026-06-15 05:01:28 UTC`: systemd killed the service after abort timeout;
  result `watchdog`.
- `2026-06-15 05:02:32 UTC`: service restarted.

Current service state at read-only inspection:

- service active/running with `NRestarts=2`;
- systemd `WatchdogUSec=5min`;
- cgroup `MemoryHigh=1610612736`, `MemoryMax=2147483648`;
- `memory.current` about 1.54 GB and `memory.peak=1633554432`;
- `memory.events high=321808`, `max=0`, `oom=0`, `oom_kill=0`;
- cgroup CPU and I/O pressure were still non-zero during inspection.

Conclusion from timeline:

- The post-install failures are watchdog failures, not OOM kills.
- The watchdog windows begin immediately after broad processing batches finish
  and entity artifact refresh is queued.
- The service is repeatedly operating at or above `MemoryHigh`, so kernel memory
  reclaim/throttling is real, not theoretical.

#### Netdata Cloud Evidence

Cloud telemetry was queried for these incident windows:

- `2026-06-14 20:30:00` to `20:42:00 UTC`;
- `2026-06-15 04:50:00` to `05:02:00 UTC`.

Confirmed resource pressure:

- service CPU reached about 193-194% on the two-core host during the incident
  windows;
- system CPU pressure reached about 30-33%;
- service memory reached roughly the configured `MemoryHigh` area, with service
  memory samples around 1.5 GB and cgroup/service memory above that including
  file/slab pressure;
- memory full pressure reached about 85-90%;
- swap was zero and `mem.oom_kill` was zero;
- app disk I/O reached about 149-218 MiB/s;
- disk utilization reached about 84-88%;
- I/O full pressure reached about 39-40%;
- dirty/writeback data reached about 96-100 MiB.

Confirmed phase/activity correlation:

- `app.uptime` reset at the two restart times.
- Engine phase metrics show metadata, insights, and publish immediately before
  the entity background task starts.
- Background metrics show an `entity_artifacts` task active from the
  post-publish moment until the end of both watchdog windows.
- Worker limit for background work was `1`.

Important telemetry limitation:

- Per-incident entity lock hold/wait metrics were not available from Cloud for
  the June 14/15 windows. Those contexts had no rows for these windows.
- Generic HTTP duration counters had no useful observations in the incident
  windows, so Cloud does not prove the exact public HTTP latency during the
  failures.

Conclusion from Cloud:

- CPU, memory pressure, and I/O pressure were all severe enough to explain
  process-wide starvation.
- Cloud proves the entity-artifact background task was active during the
  watchdog windows, but it does not by itself prove the exact lock duration
  during those two windows.

#### Admin Runtime Evidence

The admin status endpoint is reachable during non-incident inspection and shows
which operations dominate accumulated runtime.

Current/lifetime examples from the live service:

- `metadata.write_comparison_files`: lifetime count `49`, total about
  `4,756,747 ms`, max about `910,488 ms`.
- `metadata.comparison_pair_overlap`: lifetime count `136,980`, total about
  `4,440,949 ms`, max about `1,465 ms`.
- `entity.writer_lock_hold`: lifetime count `49`, total about `4,049,828 ms`,
  max about `299,272 ms`.
- `sources.update_retention`: lifetime count `805`, total about
  `1,979,035 ms`, max about `45,161 ms`.
- last completed run showed `entity.writer_lock_hold` about `127,322 ms`.
- last completed run showed `metadata.write_comparison_files` about
  `242,783 ms`.
- last completed run showed `metadata.comparison_pair_overlap` about
  `237,622 ms`.
- last completed run showed `sources.update_retention` about `107,679 ms`.

Conclusion from admin status:

- Comparison generation, entity artifact mutation, and retention are not small
  background details; they are measured multi-minute cost centers.
- The entity writer lock is not proven to block `/healthz`, but it proves the
  entity writer path performs long single critical sections.

#### Code Evidence

Single-process architecture:

- `pkg/web/server_run.go:34` creates the scheduler runner.
- `pkg/web/server_run.go:37` starts background work.
- `pkg/web/server_run.go:43` builds public/admin HTTP servers.
- `pkg/web/server_run.go:49` starts the watchdog goroutine.
- `pkg/web/server_run.go:252` serves HTTP servers in goroutines in the same
  process.

Watchdog semantics:

- `pkg/web/server_run.go:221` sends watchdog pings on a timer.
- `pkg/systemd/notify.go:40` uses half of `WATCHDOG_USEC` as the ping interval.
- `pkg/systemd/notify.go:52` opens a Unix datagram notification path per ping.
- The watchdog does not perform an internal HTTP request and does not prove that
  public endpoints can answer requests.

Scheduler broad processing:

- `pkg/scheduler/processing_loop.go:29` drains all queued processing items into
  one batch.
- `pkg/scheduler/processing_loop.go:37` documents the full-pipeline behavior.
- `pkg/scheduler/processing_loop.go:44` sets `Reprocess=true`.
- `pkg/scheduler/processing_loop.go:103` logs completed batches.
- `pkg/scheduler/processing_loop.go:104` to `pkg/scheduler/processing_loop.go:110`
  queues entity artifact refresh after the batch.

Entity refresh:

- `pkg/engine/entity_refresh_queue.go:46` queues changed feed names.
- `pkg/engine/entity_refresh_queue.go:64` coalesces pending names.
- `pkg/engine/entity_refresh_queue.go:69` starts the refresh goroutine.
- `pkg/engine/entity_refresh_queue.go:240` drains queued refresh waves.
- `pkg/engine/entity_refresh_queue.go:269` runs feed update refresh under
  `withEntityArtifactMutation`.
- `pkg/engine/background_tasks.go:254` to
  `pkg/engine/background_tasks.go:270` holds `entityArtifactsMu` across the
  full mutation function and records lock hold duration.

Comparison generation:

- `pkg/engine/output_comparison.go:65` writes comparison files.
- `pkg/engine/output_comparison.go:80` prepares set infos for all public
  output names.
- `pkg/engine/output_comparison.go:85` runs pair comparisons.
- `pkg/engine/output_comparison.go:148` starts comparison workers.
- `pkg/engine/output_comparison.go:175` to
  `pkg/engine/output_comparison.go:184` enumerates updated-vs-all pair
  candidates.
- `pkg/engine/output_comparison.go:218` to
  `pkg/engine/output_comparison.go:230` opens both sets and counts overlap.
- `pkg/engine/output_comparison_helpers.go:353` to
  `pkg/engine/output_comparison_helpers.go:399` sanitizes comparison artifacts
  by scanning both live and staged output directories.

Retention:

- `pkg/engine/process.go:116` diffs current set against previous latest.
- `pkg/engine/process.go:131` updates retention from that diff.
- `pkg/engine/retention_update.go:175` reads retention cohort files.
- `pkg/engine/retention_update.go:216` intersects cohort data with current set.
- `pkg/engine/retention_update.go:244` skips write only after expensive
  intersection already happened.

Public HTTP serving:

- `/healthz` is a lock-free constant response at `pkg/web/routes.go:31`.
- `/api/v1/status` builds a lightweight public status at `pkg/web/routes.go:35`.
- `/api/v1/sets/{name}/compare` serves the precomputed comparison artifact at
  `pkg/web/routes.go:140`, not live pairwise comparison.
- `/api/v1/compose` performs on-request set opens, union/exclude, and output
  generation at `pkg/web/routes.go:75` and `pkg/engine/public.go:354`.

Conclusion from code:

- Go is not the demonstrated problem. The demonstrated problem is that public
  serving, watchdog, ingest, processing, comparison generation, publication,
  and entity refresh share one process and one cgroup under resource pressure.
- Public compare is not the culprit for these incidents because it serves a
  static artifact. Compose and IP lookup remain request-cost risks, but no
  incident evidence points to public request load as the primary trigger.

### Evidence Collected - 2026-06-19

The server was checked again after several days of runtime.

Read-only service evidence:

- current service state was active/running;
- current `MainPID` was `617215`;
- current `ActiveEnterTimestamp` was `2026-06-19 08:30:46 UTC`;
- current systemd `NRestarts` was `68`;
- public `/healthz` returned HTTP 200 in about `0.003s`;
- public `/api/v1/status` returned HTTP 200 in about `0.006s`;
- admin status showed the engine running in `sources` phase with
  `processing_waiting_count=354` and `processing_deferred_count=48`.

Crash evidence since `2026-06-15 05:02 UTC`:

- watchdog timeout count: `66`;
- service-journal OOM line count: `0`;
- invalid-argument exits: `4`;
- watchdog timeouts by day:
  - `2026-06-15`: `1`;
  - `2026-06-16`: `32`;
  - `2026-06-17`: `25`;
  - `2026-06-18`: `6`;
  - `2026-06-19`: `2`.

Latest observed crash:

- `2026-06-19 08:28:15 UTC`: watchdog timeout;
- `2026-06-19 08:29:45 UTC`: SIGKILL after watchdog stop timeout;
- `2026-06-19 08:30:46 UTC`: service restarted.

Current cgroup evidence:

- `memory.events high=25948`;
- `memory.events max=0`;
- `memory.events oom=0`;
- `memory.events oom_kill=0`;
- `memory.current` about `1.38 GB`;
- `memory.peak=1611665408`.

Conclusion from 2026-06-19 evidence:

- The service is still not production-stable.
- The failure mode remains watchdog timeout, not service-journal OOM.
- Crash frequency appears lower after 2026-06-17, but the service still crashed
  twice on 2026-06-19, so the core problem remains unsolved.

### Accepted Remediation Process - 2026-06-19

Primary objective:

- make backend ingest/processing/artifact work bounded, predictable, and
  non-crashing without changing functional output semantics.

Rejected as primary solution:

- public/worker split as the fix for this problem;
- static-site conversion as the fix for this problem;
- relaxing watchdog behavior or increasing memory as the fix for this problem.

Accepted as possible later hardening:

- public/worker split, lower-priority workers, or static public pages may still
  be useful after the backend resource model is fixed, but they must not be used
  to hide a backend that still stalls or crashes.

Goal order:

1. Stop unnecessary work.
2. Bound required work and enforce progress/cancellation/resource contracts.
3. Optimize comparison generation.
4. Optimize entity refresh.
5. Optimize retention.
6. Add budgets around dynamic public API work.
7. Correct watchdog semantics after the backend can meet the deadline under
   stress.

Per-goal workflow:

1. Update this SOW or create a focused follow-up SOW with the goal's exact scope.
2. Perform local analysis with file/line and runtime evidence.
3. Run `glm`, `minimax`, `kimi`, `mimo`, `deepseek`, and `qwen` against the same
   scope in read-only mode.
4. Merge and weight findings by evidence.
5. Define implementation acceptance criteria and tests before coding.
6. Implement only accepted findings for the current goal.
7. Run the same reviewers on the full implementation scope.
8. Iterate until reviewers and local validation show remaining returns are
   negligible or out of scope.

Functional constraints:

- Feed outputs, merges, retention artifacts, comparison artifacts, entity
  artifacts, public API semantics, and admin semantics must not change unless a
  difference is explicitly documented and approved.
- The scheduler may run the engine less frequently only when no feed is checked
  or processed less frequently than its configured update cadence.
- Heavy processing should run only when source inputs or required derived
  dependencies actually changed.

Goal 1 definition:

- identify and remove unnecessary work, especially broad processing, publishing,
  comparison, entity refresh, retention, or other heavy phases after unchanged,
  all-failed, zero-success, or already-current inputs;
- prove which work is necessary vs unnecessary using code, tests, runtime
  metrics, and output-equivalence checks;
- implement no functional behavior changes beyond eliminating redundant work.

### Goal 1 Reviewer Synthesis - 2026-06-19

External reviewers used:

- `glm`
- `minimax`
- `kimi`
- `mimo`
- `deepseek`
- `qwen`

Baseline validation before any code change:

- `timeout 1800 go test -count=1 ./pkg/engine ./pkg/scheduler ./pkg/web`
  passed on the unchanged tree.

Weighted accepted Goal 1 findings:

1. Zero-success scheduled processing batches must not force global heavy work.
   Evidence: the scheduler always calls `RunOnce` with `Reprocess: true`
   (`pkg/scheduler/processing_loop.go:44`), which prevents `skipHeavy` and
   forces `shouldPublish` in `pkg/engine/run_pipeline.go:154-165`. If
   `report.Updated` is empty and no database/provider/default/critical repair
   reason exists, global comparison, metadata, entity-sidecar fan-out, insights,
   markdown, and publication cannot produce new public truth. Explicit manual,
   integrity, startup, provider-default, database, and critical-provider repair
   semantics must remain unchanged.
2. Broad comparison sanitization is redundant on the hot path. Evidence:
   `writeComparisonFiles` always calls `sanitizeComparisonArtifacts`
   (`pkg/engine/output_comparison.go:95`), and the sanitizer scans live and
   staged `*_comparison.json` files (`pkg/engine/output_comparison_helpers.go:353-400`).
   The normal comparison writer already removes explicit zero-overlap rows
   through `mergeCompareRows` (`pkg/engine/output_comparison_helpers.go:331-351`),
   so the full sweep is repair/migration work, not per-run publication work.
3. Byte-identical comparison row writes may be skipped only when content and
   logical mtime are already equivalent. Evidence: `writeMergedComparisonRowsForFeed`
   always writes staged comparison JSON after merging rows
   (`pkg/engine/output_comparison.go:388-399`). This is accepted only as a
   scoped no-op write suppression, not as a change to comparison semantics.
4. Entity artifact missing-sidecar fallback can rescan the full country/ASN
   sidecar corpus once per missing feed sidecar. Evidence:
   `buildFeedEntityDelta` calls `entityArtifactsContainFeed` on each missing
   old feed sidecar (`pkg/engine/entity_surgical_delta.go:28-42`), and that
   helper scans all country and ASN sidecars
   (`pkg/engine/entity_surgical_io.go:9-57`). This is accepted as the next Goal
   1 repair-path optimization after the scheduler/comparison hot-path items.

Rejected or deferred findings for Goal 1:

- Retention no-op skipping is deferred to Goal 5. Reviewers disagreed, and
  `refreshRetentionWithoutRemovals` intentionally rebuilds current buckets using
  `updatedAt` (`pkg/engine/retention_update.go:164-168`). Skipping this can
  change the public retention aging model, so it is not a safe Goal 1 no-op.
- Full comparison signature preparation is deferred to Goal 3. It is expensive
  and measured, but current pair pruning depends on per-feed signatures built in
  `prepareComparisonSetInfos` (`pkg/engine/output_comparison.go:98-130`).
- Home aggregate rebuilds are deferred. They are broad, but global homepage
  aggregate output can legitimately change after any feed/provider input change
  (`pkg/engine/home_aggregates.go:118-215`).
- Insights, markdown, metadata indexes, and public sitemap/list generation are
  not removed in Goal 1 unless the entire run is proven to be a no-op.
- Entity mtime touches are not removed. The pipeline integrity contract depends
  on logical mtimes for generated artifacts.
- Public/worker split, dynamic public API budgets, entity refresh chunking, and
  watchdog health semantics remain Goals 4, 6, and 7 or later hardening, not
  Goal 1 implementation.

Goal 1 first implementation slice:

1. Add tests for zero-success scheduled batches versus explicit manual/integrity
   reprocess semantics.
2. Suppress global heavy publication when a scheduled processing batch has no
   successful updates and no independent repair/provider reason.
3. Move broad comparison sanitization out of the normal comparison hot path or
   scope it to explicit repair behavior while preserving zero-row absence.
4. Add tests proving untouched comparison files are not scanned/rewritten during
   ordinary publication and zero-row repair still has a deterministic path.

### Goal 2 Definition - 2026-06-19

Goal 2 scope:

- bound required work and enforce progress/cancellation/resource contracts
  without changing successful functional output;
- focus on contracts that keep necessary work interruptible, observable, and
  admitted through bounded resource paths;
- do not perform algorithm-specific comparison, entity refresh, retention, or
  dynamic public API policy optimization in Goal 2 unless the change is required
  to enforce a general resource contract and preserves successful output.

Goal 2 is not allowed to:

- change feed cadence, feed contents, retention aging semantics, comparison
  outputs, entity outputs, public API successful-response semantics, or admin
  operation semantics;
- hide unresponsiveness by weakening the watchdog, splitting the process, or
  moving public serving elsewhere as a substitute for backend resource control;
- introduce broad timeout/drop behavior without a documented contract and
  external review.

Read-only internal analysis used four explorer agents plus local code review.
Accepted Goal 2 evidence before external reviewer pass:

1. Processing/publication is serialized against other `RunOnce` executions, but
   not against all process work. Evidence: scheduler processing calls
   `eng.RunOnce` (`pkg/scheduler/processing_loop.go:41`), and `RunOnce` rejects
   concurrent runs (`pkg/engine/run.go:49`, `pkg/engine/run.go:199`). Downloads,
   entity background work, admin-triggered actions, and dynamic public APIs use
   separate admission paths.
2. Downloads can overlap with processing/heavy phases. Evidence: scheduler
   starts fetch, processing, and recovery loops independently
   (`pkg/scheduler/scheduler.go:273`), and normal automatic due work is not
   globally suppressed while the engine is running except for a specific
   critical-provider case (`pkg/scheduler/automatic_due.go:39`).
3. Processing active workers are bounded, but goroutine admission is not bounded
   by the worker count. Evidence: `processRunSources` starts one goroutine per
   selected source and gates actual work on a semaphore
   (`pkg/engine/run_pipeline.go:52-57`).
4. Processing drains the whole waiting queue into one batch. Evidence:
   `drainProcessingQueue` copies the entire waiting map and clears it
   (`pkg/scheduler/processing_loop.go:127-140`). This is a valid Goal 2 concern,
   but batch caps can increase repeated global work if implemented naively, so
   they require external review before coding.
5. Background worker waits are not context-aware. Evidence:
   `backgroundLimiter.Acquire` waits on a condition variable without `ctx`
   (`pkg/engine/background_tasks.go:53-63`), and `withBackgroundTask` calls it
   before running task work (`pkg/engine/background_tasks.go:175-190`).
6. Entity artifact mutation lock wait and hold are coarse resource contracts.
   Evidence: queued entity refresh drains all pending names and runs under
   `withEntityArtifactMutation` (`pkg/engine/entity_refresh_queue.go:240-274`),
   while the mutation wrapper uses a plain `entityArtifactsMu.Lock`
   (`pkg/engine/background_tasks.go:254-270`). Production evidence already
   recorded long `entity.writer_lock_hold` durations, including a maximum near
   the watchdog window.
7. Artifact publish paths lack context/progress contracts. Evidence:
   `publishRunArtifacts` has no `ctx` parameter
   (`pkg/engine/run_pipeline.go:346-367`), staged publish walks the stage tree
   without cancellation/progress (`pkg/engine/web_batch.go:108-192`), and
   same-content comparison reads file chunks without cancellation
   (`pkg/engine/web_batch.go:194-249`).
8. Raw feed copy publication lacks context/byte progress. Evidence:
   `copyUpdatedIPSetsToWeb` and `copyUpdatedIPSetToWeb` are called during
   publish without `ctx` (`pkg/engine/run_pipeline.go:367`), and the copy helper
   streams with `io.Copy` without cancellation or progress accounting
   (`pkg/engine/web_ipsets.go:15-118`).
9. Comparison has phase-level cancellation but not inner-loop cancellation.
   Evidence: `writeComparisonFiles` passes `ctx` to pair scheduling
   (`pkg/engine/output_comparison.go:66-95`), but
   `prepareComparisonSetInfos` does not take `ctx`
   (`pkg/engine/output_comparison.go:98-130`),
   `buildComparisonSetSignature` iterates ranges without `ctx`
   (`pkg/engine/output_comparison_helpers.go:161`), and
   `iprange.OverlapCountIter` has no context (`pkg/iprange/iter_ops.go:35`).
10. Public compare API currently serves a precomputed artifact, not live pairwise
    overlap. Evidence: `servePublicSetAction` handles `compare` by serving
    `{name}_comparison.json` (`pkg/web/routes.go:140-142`). Dynamic compose and
    search remain residual Goal 6 risks; Goal 2 may only add context propagation
    where existing request contexts are already accepted.
11. Public IP query accepts request context but loses part of it. Evidence:
    web search handlers pass `r.Context` (`pkg/web/search_api.go:30`), while
    query cache/open paths use non-context cache acquire/open behavior
    (`pkg/engine/query_set_cache.go:32-44`).
12. Public compose has count/output limits but final rendering is not cancellable.
    Evidence: compose caps include/exclude and output bytes
    (`pkg/engine/public.go:348-351`), checks context during composition
    (`pkg/engine/public.go:364`, `pkg/engine/public.go:401`,
    `pkg/engine/public.go:426`), then writes final output without a context-aware
    writer (`pkg/engine/public.go:443-447`).
13. Startup integrity scanning is synchronous and not context-aware. Evidence:
    startup calls integrity recovery before normal server flow
    (`pkg/web/server_run.go:34`, `pkg/web/server_run.go:78`), while integrity
    checks scan sources/artifacts without `ctx`
    (`pkg/engine/integrity_check.go:40`,
    `pkg/engine/integrity_check.go:84`,
    `pkg/engine/integrity_check.go:199`). Moving this work to background is a
    design change, so Goal 2 should only consider context/checkpoint plumbing
    unless explicitly approved.
14. Runtime validation does not reject negative worker knobs consistently.
    Evidence: config validation checks web cache caps and `max_ingest_workers`
    (`pkg/config/validate.go:348`), while runtime defaults silently convert
    several `<=0` worker fields to defaults (`pkg/engine/runtime.go:227-243`).

Weighted Goal 2 candidate groups before external reviewer pass:

1. High-confidence no-output-change contract work:
   - use real worker-pool admission for processing so goroutines are bounded by
     configured workers;
   - make background worker acquisition context-aware;
   - thread existing contexts through publish/copy/same-content loops and check
     between files/chunks;
   - add context checks/progress to derived writer loops, retention outer loops,
     startup integrity loops, and entity setup loops where successful output
     remains identical;
   - add context-aware overlap/signature helpers while keeping existing
     non-context wrappers for tests and callers that do not need cancellation;
   - reject negative worker/runtime values instead of silently treating them as
     default values.
2. Requires external review before coding:
   - run/phase deadline defaults and visible `deadline_at`;
   - scheduler batch item/time caps;
   - entity mutation lock hold caps or chunk/release behavior;
   - global weighted work-admission shared by downloads, processing, background
     tasks, and dynamic public APIs;
   - public compose/search concurrency rejection policy.
3. Explicitly later-goal work:
   - comparison caching/signature persistence and pair-pruning redesign (Goal 3);
   - entity refresh chunking and aggregate algorithm redesign (Goal 4);
   - retention cohort/no-op algorithm changes (Goal 5);
   - dynamic public API concurrency, rate, or overload response policies (Goal 6);
   - watchdog semantic changes (Goal 7).

### Goal 2 External Reviewer Synthesis - 2026-06-19

External reviewers used:

- `glm`
- `minimax`
- `mimo`
- `kimi`
- `qwen`
- `deepseek`

Consensus findings:

1. The Goal 2 scope and all 14 code-evidence items are accepted. Reviewers did
   not find fabricated evidence or a missing code path that invalidates the
   Goal 2 boundary.
2. Context/cancellation plumbing is safe when it preserves successful output,
   but it is only groundwork until a later contract introduces deadlines,
   operator cancellation, shutdown, or explicit overload behavior. The current
   scheduler/run context is effectively daemon-lifetime for normal operation.
3. The processing goroutine-admission item is valid but low direct production
   value. The default managed runtime sets `max_ingest_workers: 1`, and runtime
   ceiling logic clamps processing, heavy, background, download, and DNS worker
   pools to one worker. Therefore, the observed two-core saturation is more
   likely cross-path overlap than hundreds of processing goroutines.
4. The entity artifact mutex is more important than the initial Goal 2 text
   made explicit. It is shared by background mutation, foreground publish, and
   entity feed sidecar build. A foreground publish can wait behind a long
   background entity mutation. Releasing the lock between chunks, however, can
   expose a partial entity surface unless publish/refresh serialization is
   redesigned, so chunking remains Goal 4 work.
5. Publishing cancellation has a correctness risk if a deadline interrupts a
   publish after some files are promoted. Goal 2 may add cancellation checks for
   shutdown/operator cancellation, but deadline-driven mid-publish aborts need a
   separate visible contract before coding.
6. Query request context propagation and compose final-render pre-checks are
   safe no-output-change items. They improve cancellation behavior for existing
   request contexts without introducing public API overload policy.
7. Negative runtime worker values should be rejected, while zero should retain
   existing "use default" semantics. This avoids silently accepting invalid
   resource configuration without breaking existing default behavior.

Accepted Goal 2 first implementation slice:

1. Thread existing contexts through publish and raw-copy helpers:
   `publishRunArtifacts`, staged publish, generated timestamp application,
   same-content reads, and raw ipset copy. Checks must occur before starting
   the publish, between file promotions, and between read/write chunks where a
   cancelled shutdown can exit without changing successful output semantics.
2. Make background worker acquisition context-aware. If the context cancels
   before a worker is acquired, the task must finish cleanly without releasing a
   slot that was not acquired and without leaking the queued task state.
3. Propagate request context into public query set cache opens and add a final
   compose render pre-check. Do not add public API concurrency rejection,
   throttling, or output-policy changes in Goal 2.
4. Add context checks to comparison preparation/signature/overlap loops using
   ctx-aware helper variants while keeping existing non-context wrappers. The
   standalone `pkg/iprange` package may import only standard-library packages
   and must keep the existing `OverlapCountIter` API.
5. Add context checkpoints to derived writer loops, startup integrity scans,
   retention outer loops, and entity setup loops where the only behavior change
   on cancellation is returning the existing cancellation error.
6. Reject negative runtime worker values in validation. Keep zero as default.

Deferred from Goal 2 first implementation slice:

- Real processing worker-pool admission: valid but low direct value under the
  current managed runtime clamp. Reconsider only after higher-impact cancellation
  plumbing or if tests show it is cheaper to change while touching the same code.
- Run/phase deadlines and visible `deadline_at`: user/product decision because
  they can abort useful work and create partial-publish semantics.
- Scheduler batch caps: user/product decision because naive caps can increase
  repeated global work or change timing/retry behavior.
- Entity mutation lock chunk/release or publish barrier: Goal 4, because it is
  the highest-value entity item but also the highest correctness risk.
- Global weighted work admission: architectural follow-up after the per-path
  resource contracts are explicit.
- Public compose/search overload rejection: Goal 6.

Goal 2 validation requirements:

- Run baseline validation before Goal 2 code edits.
- Add focused tests for cancellation before publish, cancellation during
  same-content comparison, cancellation during raw copy, cancelled background
  worker wait, request-context propagation to query cache, and negative runtime
  worker validation.
- Preserve tests proving successful publish/copy/comparison/query behavior when
  contexts are live.
- Run affected package tests and `make test-strict` after implementation.

### Goal 2 First Implementation Slice - 2026-06-19

Baseline before Goal 2 code edits:

- `timeout 1800 go test -count=1 ./pkg/cache ./pkg/engine ./pkg/scheduler ./pkg/web`
  passed on the post-Goal-1 tree.

Implemented:

1. Publish and raw mirror copy now accept the active operation context:
   `publishRunArtifacts`, staged publish, generated timestamp application,
   same-content comparison, raw ipset mirror copy, and context-bearing entity
   repair publish paths now check cancellation before admission, between files,
   or between chunks where appropriate.
2. Background worker admission is context-aware. Cancellation before acquiring a
   worker returns `context.Canceled`, removes the queued visible task, and does
   not release a slot that was never acquired.
3. Public query set cache opens now use the request context instead of
   `context.Background()`. Compose checks request cancellation immediately
   before final output rendering.
4. Comparison preparation and exact overlap counting now have context-aware
   helpers. The existing `pkg/iprange.OverlapCountIter` API remains unchanged;
   context-bearing callers use `OverlapCountIterContext`.
5. Insight, markdown, startup integrity, retention, and entity-artifact setup
   loops now have cancellation checkpoints without changing successful output.
6. Runtime validation now rejects negative authored values for worker,
   scheduling, download-error suppression, and public artifact cache resource
   controls. Zero continues to mean default or disabled.

Tests added or updated:

- cancelled staged publish leaves the live file untouched and returns
  `context.Canceled`;
- cancelled same-content comparison returns `context.Canceled`;
- raw copy cancellation before replacement leaves the destination untouched;
- stream-copy cancellation after a read stops before writing partial data;
- background limiter cancellation while waiting preserves the running count;
- cancelled background wait does not leak a visible task or worker slot;
- public query cache honors cancelled request context;
- comparison signature and `pkg/iprange` overlap helpers honor cancellation;
- negative runtime resource controls are rejected and zero values are accepted.

Validation after implementation:

- `timeout 1800 go test -count=1 ./pkg/engine -run 'TestStagedPublishBatch|TestSameRegularFileContentContextCancelled|TestCopyFileViaNew|TestCopyWithContext|TestBackgroundLimiter|TestWithBackgroundTask|TestOpenLatestSetForQuery|TestBuildComparisonSetSignatureContextCancelled|TestCheckIntegrity|TestBuildFeedEntityDelta|TestRefreshEntityArtifacts'`
  passed.
- `timeout 1800 go test -count=1 ./pkg/iprange -run 'TestOverlapCountIter'`
  passed.
- `timeout 1800 go test -count=1 ./pkg/config -run 'TestValidate.*RuntimeResourceControls'`
  passed.
- `timeout 1800 go test -count=1 ./pkg/config ./pkg/iprange ./pkg/cache ./pkg/engine ./pkg/scheduler ./pkg/web`
  passed.
- `timeout 600 make staticcheck` passed.
- `timeout 600 golangci-lint run --timeout=10m ./pkg/config/... ./pkg/iprange/... ./pkg/engine/... ./pkg/scheduler/... ./pkg/web/...`
  passed with `0 issues`.
- `timeout 1800 make test-strict` passed.
- `timeout 1800 go test -race -count=1 ./pkg/engine -run 'TestStagedPublishBatchPublishContextCancelledBeforeStartLeavesLiveUntouched|TestCopyWithContextStopsBeforeWritingAfterCancellation|TestBackgroundLimiterAcquireContextCancelledWhileWaiting|TestWithBackgroundTaskCancelledWaitDoesNotLeakTaskOrWorker|TestOpenLatestSetForQueryHonorsCancelledContext|TestBuildComparisonSetSignatureContextCancelled'`
  passed.
- `timeout 1800 make build` passed.

Deferred after implementation:

- run/phase deadlines and `deadline_at`;
- scheduler batch caps;
- entity mutation lock chunking or publish barriers;
- global weighted admission;
- public compose/search overload rejection;
- processing worker-pool admission, because current production worker clamping
  makes it low-value relative to the implemented cancellation contracts.

### Goal 2 Implementation Review Synthesis - 2026-06-19

External reviewers used:

- `glm`
- `minimax`
- `mimo`
- `kimi`
- `qwen`
- `deepseek`

Review result:

- all six reviewers returned `PRODUCTION GRADE`;
- no reviewer found a blocking correctness, security, race, resource leak,
  output-semantic, or production-readiness issue;
- all reviewers accepted that deferred policy/architecture items remain outside
  this Goal 2 first slice.

External review transcripts:

- `/tmp/sow105-goal2-impl-review-glm.txt`
- `/tmp/sow105-goal2-impl-review-minimax.txt`
- `/tmp/sow105-goal2-impl-review-mimo.txt`
- `/tmp/sow105-goal2-impl-review-kimi.txt`
- `/tmp/sow105-goal2-impl-review-qwen.txt`
- `/tmp/sow105-goal2-impl-review-deepseek.txt`

Non-blocking notes and disposition:

1. `entityArtifactFeedPresence` does not cache scans that return early after
   finding the target feed. This is intentional and required by the SOW/spec
   rule that incomplete scans must not be cached. Completed not-found scans are
   reused; early-found scans preserve correctness and may rescan later in the
   same batch.
2. `publishContext` can promote some complete files before shutdown/operator
   cancellation is observed. This is accepted for this slice because there are
   no new deadlines, each file promotion remains atomic, callers clean staging
   directories, and integrity/recovery paths detect missing/stale outputs.
   Deadline-driven mid-publish abort semantics remain a later explicit
   contract.
3. `compareSetPair` returns `ok=false` when exact overlap cancellation is
   observed. The parent comparison phase checks the context before writing
   merged rows, so no partial comparison output is published.
4. `CleanupStaleCriticalInfrastructureArtifacts` and admin integrity reports
   still use non-context wrappers. Reviewers accepted this as outside the
   agreed first slice because startup integrity, run publish, entity repair,
   public query, and request compose paths were the accepted targets.
5. The startup integrity cancellation log level may be operator polish, but it
   is not a correctness issue and does not affect resource control.
6. Processing worker-pool admission remains deferred because managed production
   worker clamping makes it low-value relative to the implemented contracts.

### Goal 3 Definition - 2026-06-19

Goal 3 scope:

- reduce CPU, memory pressure, and disk I/O from pairwise public comparison
  generation without changing comparison artifacts, public compare semantics,
  unique-share inputs, integrity behavior, or repair behavior;
- prioritize eliminating recomputation of unchanged exact pair overlap counts;
- keep comparison generation as a pipeline producer of published artifacts;
  public requests must remain artifact readers and must not generate missing
  comparison data on demand.

Goal 3 first-slice production-impact boundary:

- the pair-result ledger primarily optimizes incremental/scoped reprocess,
  artifact repair, and broad forced-reprocess cases where current feed content
  hashes already match cached pair keys;
- it cannot avoid exact overlap for a pair whose left or right normalized range
  content actually changed, because the current content hash is intentionally
  part of the safety key;
- broad runs where many feeds truly changed still require exact overlap for
  those changed-content pairs. Remaining broad-run cost is mapped to later Goal
  3 slices such as safe signature/content-identity persistence and preparation
  cost reduction;
- the first run after this feature ships has no pair ledger yet, so savings
  begin only after full runs or incremental computations populate current pair
  entries.

Goal 3 is not allowed to:

- change `{feed}_comparison.json` row schema, sorting, JSON formatting, trailing
  newline, or logical mtime contract;
- preserve stale positive rows when a recomputed pair now has zero overlap;
- drop fresh zero-overlap pair results before they can delete stale rows;
- write only the updated feed side of a symmetric comparison pair;
- rely on feed names, provider names, substring matching, or generated artifact
  filename semantics to decide comparison behavior;
- use mtime, size, `SourceDate`, `ProcessedDate`, `Entries`, `UniqueIPs`, or
  `cache.Entry.ContentHash` alone as proof that a public feed's normalized range
  content is unchanged;
- move comparison generation to public request handlers.

Local evidence:

1. Comparison generation is a measured production cost center. Existing admin
   evidence in this SOW recorded `metadata.write_comparison_files` lifetime
   total about `4,756,747 ms`, max about `910,488 ms`, recent-run duration about
   `242,783 ms`, and `metadata.comparison_pair_overlap` recent-run duration
   about `237,622 ms`. This makes exact pair overlap work the first measured
   Goal 3 target, with set-signature preparation as the second target.
2. Current comparison preparation opens and scans every public output set before
   pair filtering: `writeComparisonFiles` gets `publicOutputNames` and calls
   `prepareComparisonSetInfos` for all names
   (`pkg/engine/output_comparison.go:72-82`), while
   `prepareComparisonSetInfos` opens each usable set and builds a range
   signature by iterating ranges (`pkg/engine/output_comparison.go:102-127`,
   `pkg/engine/output_comparison_helpers.go:158-194`).
3. Pair enumeration is already incremental, but it still recomputes every
   updated-vs-all candidate pair. Non-empty `updatedNames` keeps pairs where
   either side is updated (`pkg/engine/output_comparison.go:185-196`), and exact
   overlap is counted after range/prefix filters
   (`pkg/engine/output_comparison.go:204-245`).
4. Existing cache state does not contain a safe general comparison identity.
   `cache.Entry.ContentHash` exists, but it is documented as reference-set-only
   state (`pkg/cache/cache.go:37-39`) and finalize populates it only for
   critical-infrastructure feeds (`pkg/engine/finalize.go:73-78`). Counts and
   timestamps are not range-content identity.
5. Existing normalized range hashing can be reused as an algorithmic primitive,
   not as persisted state. `rangeSourceContentHash` hashes canonical range
   bounds while checking iterator errors (`pkg/engine/helpers.go:387-403`), and
   comparison signatures currently compute an in-memory equivalent
   (`pkg/engine/output_comparison_helpers.go:168-190`).
6. Comparison output semantics are symmetric and incremental. Every computed
   pair is grouped into both feed artifacts
   (`pkg/engine/output_comparison.go:319-339`), and incremental merge preserves
   existing positive rows while fresh zero-overlap rows delete stale positive rows
   (`pkg/engine/output_comparison_helpers.go:332-354`).
7. Public compare serving is artifact-only: `/api/v1/sets/{name}/compare`
   serves `{name}_comparison.json` (`pkg/web/routes.go:140-141`). Unique-share
   reads the same comparison artifact, preferring staged output and then live
   output (`pkg/engine/unique_share.go:75-94`). Integrity expects
   `_comparison.json` for each feed and rejects explicit zero-overlap rows
   (`pkg/engine/integrity.go:284-291`,
   `pkg/engine/integrity_payloads.go:205-219`).

Read-only explorer findings:

1. The cache-key explorer found no existing safe global key. Existing
   `ContentHash`, `Entries`, `UniqueIPs`, `SourceDate`, `ProcessedDate`, binary
   `latest` headers, and mtimes are useful hints or primitives only. Safe
   signature reuse would require a new comparison signature sidecar/ledger or a
   dedicated content-identity contract; it is not available for the accepted
   first slice.
2. The output-semantics explorer confirmed that optimizations must preserve
   symmetric peer writes, zero-overlap absence, pair-scoped stale-row deletion,
   positive-lineage relatedness, name sorting, tab-indented JSON with trailing
   newline, logical mtimes, full-reprocess behavior, unique-share consumption,
   and artifact-only public serving.

Initial Goal 3 candidate design sent to external analysis review:

1. Add an internal comparison signature ledger plus pair-result ledger under
   engine-owned runtime state.
2. Use persisted signatures to skip opening/scanning unchanged feeds.
3. Use persisted pair results to skip exact overlap for unchanged pairs.

External analysis corrected this candidate. The accepted first implementation
slice below supersedes the initial candidate.

### Goal 3 External Analysis Reviewer Synthesis - 2026-06-19

External reviewers used:

- `glm`
- `minimax`
- `mimo`
- `kimi`
- `qwen`
- `deepseek`

Final verdicts captured before this synthesis:

- `glm`: `NOT READY`
- `mimo`: `READY FOR IMPLEMENTATION` with required clarifications
- `kimi`: `READY FOR IMPLEMENTATION` with required clarifications
- `qwen`: `NOT READY`
- `deepseek`: `NOT READY`
- `minimax`: no final verdict; the read-only review reached the `timeout 1800`
  limit without emitting a conclusion.

External review transcripts:

- `/tmp/sow105-goal3-analysis-glm.txt`
- `/tmp/sow105-goal3-analysis-minimax.txt`
- `/tmp/sow105-goal3-analysis-mimo.txt`
- `/tmp/sow105-goal3-analysis-kimi.txt`
- `/tmp/sow105-goal3-analysis-qwen.txt`
- `/tmp/sow105-goal3-analysis-deepseek.txt`

Consensus findings:

1. The Goal 3 scope, code evidence, and no-output-change constraints are valid.
   Reviewers verified that public compare is artifact-only, unique-share reads
   comparison artifacts, integrity expects and validates comparison artifacts,
   and the current exact-overlap path is the dominant measured cost.
2. A persisted pair-result ledger is the right first optimization target. The
   cached value must be only the exact `common` overlap count. `Category`,
   `Related`, row encoding, and artifact bytes must still be produced from the
   current config/state through the existing `groupComparisonRows`,
   `mergeCompareRows`, and `writeMergedComparisonRowsForFeed` path.
3. The signature ledger is not safe for the first implementation slice. To reuse
   a persisted signature without scanning the current set, the implementation
   needs a cheap and safe validation key. Computing the canonical range hash is
   the scan being avoided, while mtime, size, `SourceDate`, `ProcessedDate`,
   `Entries`, `UniqueIPs`, and `cache.Entry.ContentHash` are not safe proof of
   general normalized range identity. Therefore signature-ledger reuse is
   deferred.
4. The pair-result ledger version key was initially over-broad and imprecise.
   Because the ledger stores only `common`, the key needs the ordered feed pair,
   both current canonical range content hashes, and an explicit overlap
   algorithm/range-normalization version constant. It must not store or reuse
   `Category`, `Related`, or serialized rows.
5. Ledger file format, lifecycle, corruption handling, growth bounds, and public
   access boundaries must be explicit before code changes.

Accepted Goal 3 first implementation slice:

1. Keep `prepareComparisonSetInfos` semantically unchanged for this slice: it
   must still inspect current public feeds, open usable sets, and compute current
   signatures/content hashes from canonical normalized ranges. This preserves
   the safe proof for every pair key and avoids stale-signature reuse.
2. Add an internal pair-result ledger only. The ledger stores exact overlap
   counts keyed by:
   - ordered feed pair names;
   - current left and right canonical range content hashes;
   - an explicit comparison overlap algorithm/range-normalization version
     constant.
3. The ledger must live under `runtime.CacheDir`, not under `WebDir` and not in
   a public artifact path. It is an optimization cache, not integrity data. It
   must not be added to `expectedSecondaryArtifacts`, and public/admin request
   handlers must not read or repair it. The ledger stores only feed names,
   canonical range content hashes, algorithm version, and `common` counts; an
   operator with write access to `runtime.CacheDir` can poison optimization
   state, so corrupt/malformed/version-mismatched data is ignored and full
   reprocess remains the repair path.
4. The first-slice ledger format must be a single bounded, atomically-written
   file or similarly bounded shard set, not one file per pair. The initial
   implementation should prefer the simplest debuggable format that keeps
   expected production-size read/write cost small enough to be validated by the
   required benchmark. A corrupt, truncated, missing, or version-mismatched
   ledger must be ignored and rebuilt from recomputed/reused current-run data.
   The implementation should reuse the existing temp-file-and-rename atomic
   writer (`internal/fileutil/fileutil.go:80-130`), and `ensureDirectories`
   already creates `runtime.CacheDir` (`pkg/engine/process.go:16-30`).
5. Pair cache reads must use a read-only in-memory snapshot during worker
   execution. Ledger writes happen after comparison pair processing, through an
   atomic replacement. A failed ledger write may lose future optimization only;
   it must not change public comparison output or fail the run after public
   artifacts were successfully produced.
6. Full all-pair comparison runs (`len(updatedNames) == 0`) must bypass ledger
   hits, recompute every pair with the existing overlap algorithm, and then
   replace the ledger with fresh current entries. This preserves explicit repair
   and full-reprocess intent.
7. Incremental runs must consider every current pair for ledger lookup, not
   only updated-vs-all pairs. The updated-name filter is applied only after a
   ledger miss:
   - ledger hit: emit the cached `comparisonPairResult`, including
     `common == 0`, so stale positive rows are deleted through
     `mergeCompareRows` on both peer artifacts;
   - ledger miss and at least one feed updated: compute exact overlap with the
     existing algorithm and emit the fresh result;
   - ledger miss and neither feed updated: skip the pair because no safe cached
     result exists and existing artifacts remain the authoritative state until
     a later updated/feed-repair/full-reprocess run recomputes it.
8. Incremental ledger replacement must write the full current ledger, not only
   pairs computed in the current run. The replacement is built from retained
   current-key hits plus fresh current-run computations, then pruned. This
   prevents a successful incremental run from discarding valid unchanged-pair
   entries needed by later runs. If the loaded ledger is missing or ignored as
   corrupt, there are no retained hits; the replacement may be sparse but
   remains correct and is completed by later incremental computations or a full
   all-pair run.
9. Ledger cleanup must prune entries for feeds not present in current
   `publicOutputNames` and entries whose content-hash pair no longer matches the
   current signatures. Removed, disabled, renamed, archived, unavailable, or
   unusable feeds must not be resurrected from the ledger.
10. Missing public comparison artifacts, malformed comparison artifacts, stale
   artifact mtimes, and full reprocesses are public artifact repair concerns.
   They must still go through `writeComparisonFiles` and the existing artifact
   merge/write path. The ledger may provide a `common` value only in eligible
   incremental runs; it must not suppress artifact rewrite when bytes, mode, or
   logical mtime require repair.
11. The signature ledger and any binary latest-format change to store
   normalized range hashes are deferred to a later Goal 3 slice unless fresh
   evidence shows pair-result caching alone has negligible returns.

The implementation must add focused equivalence tests that compare baseline
fresh computation against cached reuse for:

- initial full run;
- one changed feed with stale positive rows becoming zero;
- one changed peer preserving unrelated existing positive rows;
- unchanged pair reuse;
- missing/unreadable/stale pair ledger fallback;
- full reprocess;
- changed category or relatedness-affecting config;
- missing public comparison artifact repair when current ledger entries exist.

Additional required tests:

- cached `common == 0` deletes stale positive rows in both peer artifacts;
- config category and merge-lineage changes update `Category` and `Related` in
  artifacts even when a cached `common` value is reused;
- corrupt, truncated, missing, and version-mismatched pair ledgers fall back to
  recomputation;
- removed/disabled/renamed/unusable feeds are pruned and never resurrected;
- global full reprocess bypasses ledger hits and replaces the ledger;
- incremental replacement preserves valid unchanged-pair entries instead of
  degrading the ledger to only current-run pairs;
- an updated feed whose current content hash still matches the ledger produces
  ledger hits for its pairs;
- repeated feed addition/removal/update churn keeps the ledger bounded by the
  current public pair set;
- no public request path reads the ledger;
- unique-share output is byte/field-equivalent before and after cached reuse.

Deferred from Goal 3 unless external analysis proves they are required for safe
equivalence:

- changing the public comparison JSON schema or adding public comparison
  metadata files;
- changing unique-share semantics;
- using dynamic public compare as a repair fallback;
- changing scheduler cadence or download/processing admission;
- changing entity, retention, or public compose/query budgets.

Goal 3 validation requirements:

- run a baseline comparison-focused test suite before Goal 3 code edits;
- add golden/equivalence tests for cached vs uncached comparison output;
- add corruption/fallback tests for the pair-result ledger;
- add tests proving zero-overlap stale rows are deleted when cached filters or
  pair results decide a pair has no overlap;
- add tests proving unchanged pair reuse avoids exact overlap work without
  skipping peer-side artifact updates;
- add observability for pair-ledger hits, misses, ignored corrupt ledgers, and
  writes so production value can be verified;
- run affected package tests, `make staticcheck`, `golangci-lint` for touched
  packages, `make test-strict`, and `make build`;
- use benchmarks or instrumentation counters to show exact-overlap work drops on
  unchanged-pair runs and that pair-ledger read/write cost stays small relative
  to the avoided exact-overlap work while output remains equivalent.

Goal 3 baseline validation before code edits:

- `timeout 1800 go test -count=1 ./pkg/engine -run 'TestWriteComparisonFiles|TestComparison|TestMergeCompareRows|TestValidateComparisonPayload|TestComparisonRowsDoNotMark'`
  passed on the post-Goal-2 tree.

### Goal 3 Revised Gate Review Synthesis - 2026-06-19

Second-round external reviewers used:

- `glm`
- `minimax`
- `mimo`
- `kimi`
- `qwen`
- `deepseek`

Second-round transcripts:

- `/tmp/sow105-goal3-revised-review-glm.txt`
- `/tmp/sow105-goal3-revised-review-minimax.txt`
- `/tmp/sow105-goal3-revised-review-mimo.txt`
- `/tmp/sow105-goal3-revised-review-kimi.txt`
- `/tmp/sow105-goal3-revised-review-qwen.txt`
- `/tmp/sow105-goal3-revised-review-deepseek.txt`

Second-round verdicts:

- `glm`: `READY FOR IMPLEMENTATION`
- `minimax`: `READY FOR IMPLEMENTATION`
- `mimo`: `READY FOR IMPLEMENTATION`
- `qwen`: `READY FOR IMPLEMENTATION`
- `deepseek`: `READY FOR IMPLEMENTATION`
- `kimi`: `NOT READY`

Accepted second-round findings:

1. Five reviewers found the pair-result-only scope safe and bounded after the
   first-round fixes. They verified that `common` reuse is safe only when keyed
   by ordered pair names, current content hashes, and algorithm version; that
   `Category` and `Related` remain current-run values; and that public serving,
   unique-share, integrity, and artifact mtime contracts remain unchanged.
2. `kimi` found a real performance blocker in the revised SOW: if implementation
   kept current updated-vs-all pair enumeration, a pair-result ledger keyed by
   current hashes would miss whenever an updated feed's normalized range content
   actually changed. That implementation would be correct but could produce no
   measurable savings for the intended workload.
3. `kimi` also found that atomic replacement must be specified as a full
   current-ledger replacement, not only current-run processed pairs; otherwise
   successive incremental runs could discard valid unchanged-pair entries and
   degrade the ledger.
4. The SOW now explicitly requires all-pair ledger lookup during incremental
   runs, updated-name filtering only after a ledger miss, full current-ledger
   replacement, and a production-impact boundary stating that this first slice
   does not optimize broad runs where most feed contents truly changed.
5. Goal 3 implementation remains blocked until these second-round fixes receive
   the same-scope external re-review.

### Goal 3 Third Gate Review Synthesis - 2026-06-19

Third-round external reviewers used:

- `glm`
- `minimax`
- `mimo`
- `kimi`
- `qwen`
- `deepseek`

Third-round transcripts:

- `/tmp/sow105-goal3-third-review-glm.txt`
- `/tmp/sow105-goal3-third-review-minimax.txt`
- `/tmp/sow105-goal3-third-review-mimo.txt`
- `/tmp/sow105-goal3-third-review-kimi.txt`
- `/tmp/sow105-goal3-third-review-qwen.txt`
- `/tmp/sow105-goal3-third-review-deepseek.txt`

Third-round verdicts:

- `glm`: `READY FOR IMPLEMENTATION`
- `minimax`: `READY FOR IMPLEMENTATION`
- `mimo`: `READY FOR IMPLEMENTATION`
- `kimi`: `READY FOR IMPLEMENTATION`
- `qwen`: `READY FOR IMPLEMENTATION`
- `deepseek`: `READY FOR IMPLEMENTATION`

Accepted third-round findings:

1. The second-round blockers are resolved. All-pair ledger lookup is now
   required for incremental runs, the updated-name filter applies only after a
   ledger miss, and incremental replacement keeps retained current-key hits plus
   fresh current-run computations instead of degrading the ledger.
2. The all-pair lookup is bounded for the current public-feed scale because it
   is an in-memory key lookup for each current pair, while exact overlap remains
   restricted to misses where at least one feed is updated. The implementation
   still has to validate the cost with the required benchmark/instrumentation.
3. Public output semantics remain unchanged when cached `common` values flow
   through the existing `groupComparisonRows`, `mergeCompareRows`, and
   `writeMergedComparisonRowsForFeed` path. `Category`, `Related`, row
   encoding, sorting, trailing newline, and logical mtime are not cached.
4. The production-impact boundary is honest: this first slice improves
   incremental/scoped/repair/unchanged-content workloads and does not avoid
   exact overlap for pairs whose normalized content truly changed.
5. Non-blocking implementation notes accepted for code/tests: handle empty-feed
   content hashes deterministically, ignore or clean stale atomic-writer temp
   files, record first-deployment zero-savings behavior, add churn boundedness
   and updated-feed-unchanged-content hit tests, and expose bounded hit/miss/
   corrupt/write counters.

## Pre-Implementation Gate

Status: Goal 1 implementation is complete. Goal 2 first-slice implementation
is complete and externally reviewed. Goal 3 analysis reviewers found the
initial signature-ledger design not ready. Goal 3 second-round review found the
pair-result-only design safe in principle, but exposed missing enumeration and
full-ledger replacement requirements. Those requirements are now recorded above.
Goal 3 third-round review found all six reviewers ready for implementation.
Goal 3 first-slice implementation may proceed under the contract recorded
above. Goals 4-7 remain blocked until their own goal-specific gates are added.

Problem / root-cause model:

- Established problem: the managed service has missed systemd watchdog
  notifications after the latest optimization install, and the same process
  serves the public website and APIs.
- Root cause is not established yet. The working theories below must be tested
  against production telemetry, logs, and exact code paths before any fix is
  proposed as implementation work.

Evidence reviewed:

- Process model and watchdog path:
  `pkg/web/server_run.go:43`, `pkg/web/server_run.go:49`,
  `pkg/web/server_run.go:221`, `pkg/systemd/notify.go:40`.
- Processing admission and post-processing entity refresh:
  `pkg/scheduler/processing_loop.go:29`,
  `pkg/scheduler/processing_loop.go:44`,
  `pkg/scheduler/processing_loop.go:103`.
- Entity refresh queue and mutation path:
  `pkg/engine/entity_refresh_queue.go:46`,
  `pkg/engine/entity_refresh_queue.go:240`.
- Managed install resource limits:
  `install.sh:384`, `install.sh:427`.
- Runtime concurrency defaults:
  `configs/firehol/runtime.yaml:78`,
  `configs/firehol/runtime.yaml:86`.

Affected contracts and surfaces:

- Public HTTP site and public APIs, including health, status, feed browsing,
  compare, compose, and IP lookup.
- Admin status/activity surfaces.
- Scheduler download/processing queues.
- Entity artifact refresh/rebuild.
- Systemd watchdog/readiness behavior.
- Resource-limit behavior under managed install.
- SOW/spec/operator documentation if diagnosis proves an operational contract
  needs to change.

Existing patterns to reuse:

- Admin/public status API for runtime snapshots.
- Existing engine/scheduler metrics and OpenTelemetry/Netdata collection.
- Systemd journal and cgroup counters for service-local evidence.
- Netdata Cloud historical charts for host/service resource pressure.
- SOW evidence ledger for separating facts, inferences, and unknowns.

Risk and blast radius:

- Diagnosis commands must be read-only and must not restart, stop, kill, or
  reconfigure the production-candidate service.
- Remote commands may expose secrets or private details; durable notes must
  include only sanitized summaries.
- Follow-up implementation may affect core pipeline behavior; it must be
  handled in a separate implementation SOW or an approved implementation phase
  with tests first.

Sensitive data handling plan:

- Do not write Netdata Cloud tokens, `.env` values, service credentials, DroneBL
  secrets, private endpoints, raw customer-identifying IPs, or proprietary
  incident details to this SOW, specs, docs, skills, prompts, or comments.
- When environment variables are inspected, record only variable names and
  whether they exist.
- When Netdata Cloud data is queried, record chart names, time windows, metric
  patterns, and aggregate values only.
- Hostnames may be recorded only when already provided by the user in the
  conversation; secrets and token-bearing URLs must be redacted.

Implementation plan:

1. Run baseline validation on the current post-Goal-1 tree before Goal 2 code
   edits.
2. Implement only the accepted Goal 2 first-slice cancellation/context and
   validation work recorded above.
3. Keep successful output semantics unchanged: no feed cadence changes, no
   deadline/drop policy, no public API overload rejection, no entity chunking,
   and no comparison/retention algorithm changes.
4. Update specs for the new resource-contract guarantees.
5. Run focused tests for each new cancellation/validation contract, then run
   affected package tests and `make test-strict`.
6. Run the same external reviewers against the full Goal 2 implementation scope
   before considering the slice production-grade.
7. Iterate only on accepted Goal 2 findings until remaining returns are
   negligible or explicitly mapped to Goals 3-7.

Validation plan:

- Preserve the diagnosis cross-checks already recorded for systemd journals,
  app logs, cgroup state, admin status, and Netdata Cloud metrics.
- Before changing Goal 2 code, run focused baseline tests for the affected
  packages.
- Add or update behavioral tests proving successful-output semantics are
  unchanged when contexts are live.
- Add cancellation tests proving a cancelled context exits without leaking a
  background worker slot, publishing a non-atomic single file, or silently
  ignoring request cancellation.
- Add validation tests proving negative runtime worker values are rejected while
  zero/default behavior is preserved.
- After implementation, run focused package tests and `make test-strict`.
- Treat reviewer suggestions that would change timing policy, overload
  behavior, entity chunking, comparison algorithms, retention aging semantics,
  public API results, or feed cadence as later-goal or user-decision items.

Artifact impact plan:

- AGENTS.md: no expected update unless a project-wide guardrail gap is found.
- Runtime project skills: no expected update unless the implementation reveals a
  reusable testing/operations rule.
- Specs: update `pipeline.md` or `operating-principles.md` for successful-output
  invariance, cancellation checkpoints, and negative runtime worker validation.
- End-user/operator docs: no expected update unless admin/runtime semantics
  visible to operators change.
- End-user/operator skills: no expected update.
- SOW lifecycle: this file remains in `.agents/sow/current/` until the Goal 2
  accepted slice is implemented, validated, reviewed, and either closed or
  mapped to the next focused goal.

Open-source reference evidence:

- Not checked at SOW creation. The problem is production behavior of this
  application under its own workload. External Go/service references may be
  consulted later only for a concrete implementation approach after diagnosis.

Open decisions:

- The user accepted the goal-by-goal resource-control process and requested that
  implementation details not stop progress when functional requirements are
  preserved.
- If a finding requires a product semantic change, skip it for the active goal
  and map it to the later goal or a user decision rather than changing behavior.

### Goal 3 First Implementation Slice - 2026-06-19

Implemented:

1. Added an internal pair-result ledger under `runtime.CacheDir`:
   `pkg/engine/output_comparison_pair_ledger.go:12-156`.
   The format records feed-pair names, valid/invalid normalized content-hash
   markers, the comparison algorithm version, and the cached exact `common`
   count. It is loaded as a read-only in-memory snapshot and written through
   the existing atomic writer.
2. Incremental comparison runs now perform all-pair ledger lookup before
   applying the updated-name filter: `pkg/engine/output_comparison.go:93-130`
   and `pkg/engine/output_comparison.go:190-236`. Ledger hits emit cached
   `comparisonPairResult` values, including `common == 0`, so the existing
   merge path can delete stale positive rows. Ledger misses compute exact
   overlap only when at least one feed in the pair was updated.
3. Full/global comparison runs continue to bypass ledger hits and recompute the
   current pair set before replacing the ledger:
   `pkg/engine/output_comparison.go:93-110`.
4. Public row metadata is still generated from current state and config through
   the existing grouping/merge/write path. The ledger never stores category,
   relatedness, serialized rows, mtimes, or public artifact bytes.
5. Ledger load/write observability was added for lookup, hit, miss,
   miss-unchanged-skipped, ignored corrupt ledgers, write failures, and entry
   counts: `pkg/engine/output_comparison.go:45-57`,
   `pkg/engine/output_comparison.go:93-130`, and
   `pkg/engine/output_comparison.go:331-349`.
6. Specs were updated to describe the internal drop-safe ledger contract:
   `.agents/sow/specs/pipeline.md:663-680`,
   `.agents/sow/specs/operating-principles.md:457-461`, and
   `.agents/sow/specs/files-layout.md:170-180`.
7. The engine test fixture now accepts `testing.TB`, so benchmarks can use the
   same fixture boundary as tests without adding direct `Engine{}` literals:
   `pkg/engine/engine_fixture_test.go:19`.

Tests added:

- unchanged updated-feed content produces pair-ledger hits and avoids new exact
  overlap work: `pkg/engine/output_comparison_pair_ledger_test.go:14-36`;
- incremental replacement preserves valid unchanged-pair entries:
  `pkg/engine/output_comparison_pair_ledger_test.go:38-60`;
- corrupt ledger fallback ignores the ledger and rebuilds only safe current
  entries: `pkg/engine/output_comparison_pair_ledger_test.go:62-89`;
- version-mismatched and algorithm-version-mismatched ledgers are ignored and
  rebuilt through the sparse fallback path:
  `pkg/engine/output_comparison_pair_ledger_test.go:92-143`;
- removed feeds are pruned from the rewritten ledger and not resurrected:
  `pkg/engine/output_comparison_pair_ledger_test.go:145-170`;
- cached `common == 0` deletes stale positive rows from both peer artifacts:
  `pkg/engine/output_comparison_pair_ledger_test.go:172-194`;
- a missing public comparison artifact is repaired from cached ledger rows:
  `pkg/engine/output_comparison_pair_ledger_test.go:196-220`;
- unique-share results are equivalent after cached comparison reuse:
  `pkg/engine/output_comparison_pair_ledger_test.go:222-252`;
- cached overlap values still rebuild category and relatedness from current
  config/state: `pkg/engine/output_comparison_pair_ledger_test.go:254-279`;
- full reprocess bypasses poisoned ledger values:
  `pkg/engine/output_comparison_pair_ledger_test.go:281-302`;
- all-pair ledger-hit cost shape is covered by
  `BenchmarkRunComparisonPairsPairLedgerHits`:
  `pkg/engine/output_comparison_pair_ledger_test.go:304-332`.

Validation after implementation:

- `timeout 1800 go test -count=1 ./pkg/engine -run 'TestWriteComparisonFilesUsesPairLedger|TestComparisonPairLedger|TestEngineTestsUseFixtureForDirectConstruction|TestMergeCompareRows|TestValidateComparisonPayload|TestComparisonArtifactMinimality|TestComparisonRows'`
  passed.
- `timeout 1800 go test -run '^$' -bench 'BenchmarkRunComparisonPairsPairLedgerHits' -benchmem ./pkg/engine`
  passed with `BenchmarkRunComparisonPairsPairLedgerHits-24 45 23882161 ns/op
  13046606 B/op 36 allocs/op`.
- `timeout 1800 go test -count=1 ./pkg/engine` passed.
- `timeout 1800 make test-strict` passed.
- `timeout 1800 make test` passed.
- `timeout 1800 make build` passed.
- `timeout 1800 make lint` passed.
- `timeout 1800 make staticcheck` passed.
- `timeout 1800 make golangci-lint` passed with `0 issues`.
- `timeout 1800 make race` passed.

Goal 3 first implementation review synthesis:

- First same-scope implementation review transcripts:
  - `/tmp/sow105-goal3-impl-review-glm.txt`
  - `/tmp/sow105-goal3-impl-review-minimax.txt`
  - `/tmp/sow105-goal3-impl-review-mimo.txt`
  - `/tmp/sow105-goal3-impl-review-kimi.txt`
  - `/tmp/sow105-goal3-impl-review-qwen.txt`
  - `/tmp/sow105-goal3-impl-review-deepseek.txt`
- Verdicts:
  - `glm`: `PRODUCTION GRADE`
  - `minimax`: `PRODUCTION GRADE`
  - `mimo`: `PRODUCTION GRADE`
  - `kimi`: `PRODUCTION GRADE`
  - `qwen`: `PRODUCTION GRADE`
  - `deepseek`: `NOT PRODUCTION GRADE`
- Accepted blocker: DeepSeek identified missing SOW-required tests for
  version/algorithm-version fallback, removed-feed pruning, unique-share
  equivalence, and missing-artifact repair. The other five reviewers treated
  these as non-blocking because the implementation was structurally correct,
  but the SOW explicitly required them, so the blocker was accepted.
- Fix: added the missing behavioral tests in
  `pkg/engine/output_comparison_pair_ledger_test.go:92-252`.

Validation after reviewer-requested test additions:

- `timeout 1800 go test -count=1 ./pkg/engine -run 'TestComparisonPairLedger|TestWriteComparisonFilesUsesPairLedger|TestEngineTestsUseFixtureForDirectConstruction'`
  passed.
- `timeout 1800 go test -count=1 ./pkg/engine` passed.
- `timeout 1800 go test -run '^$' -bench 'BenchmarkRunComparisonPairsPairLedgerHits' -benchmem ./pkg/engine`
  passed with `BenchmarkRunComparisonPairsPairLedgerHits-24 45 23882161 ns/op
  13046606 B/op 36 allocs/op`.
- `timeout 1800 make test-strict` passed.
- `timeout 1800 make test` passed.
- `timeout 1800 make build` passed.
- `timeout 1800 make lint` passed.
- `timeout 1800 make staticcheck` passed.
- `timeout 1800 make golangci-lint` passed with `0 issues`.
- `timeout 1800 make race` passed.

External implementation review status:

- second same-scope review completed after test additions.

Goal 3 second implementation review synthesis:

- Second same-scope implementation review transcripts:
  - `/tmp/sow105-goal3-impl-review-round2-glm.txt`
  - `/tmp/sow105-goal3-impl-review-round2-minimax.txt`
  - `/tmp/sow105-goal3-impl-review-round2-mimo.txt`
  - `/tmp/sow105-goal3-impl-review-round2-kimi.txt`
  - `/tmp/sow105-goal3-impl-review-round2-qwen.txt`
  - `/tmp/sow105-goal3-impl-review-round2-deepseek.txt`
- Verdicts:
  - `glm`: `PRODUCTION GRADE`
  - `minimax`: `PRODUCTION GRADE`
  - `mimo`: `PRODUCTION GRADE`
  - `kimi`: `PRODUCTION GRADE`
  - `qwen`: `PRODUCTION GRADE`
  - `deepseek`: `PRODUCTION GRADE`
- No reviewer reported a blocking correctness, security, race, output-semantic,
  resource-bound, public-surface, or test-completeness issue after the added
  tests.
- Non-blocking hardening notes accepted as later-slice or monitoring items:
  explicit repeated add/remove/update churn test, explicit public-path
  no-ledger-read architectural guard, ledger write-cost measurement, and
  possible no-op ledger rewrite suppression. The current implementation is
  structurally bounded by current pair replacement and exposes production
  counters for value verification.

## Working Theory Ledger

Theories below are not conclusions. Each must be proved or rejected.

| ID | Theory | Status | Evidence-based finding |
|---|---|---|---|
| T1 | Cgroup `MemoryHigh` throttling stalls the process enough to miss watchdog pings and HTTP deadlines. | confirmed contributor | Live cgroup shows `memory.events high=321808`, `oom=0`, `oom_kill=0`, `memory.current` about 1.54 GB, and `memory.peak=1633554432` against `MemoryHigh=1610612736`. Cloud showed memory full pressure about 85-90% in the incident windows. This proves memory reclaim/throttling pressure, not OOM, after the latest install. |
| T2 | CPU saturation from scheduler processing or heavy phases starves the watchdog goroutine and HTTP handlers. | confirmed contributor | Cloud showed service CPU about 193-194% on a two-core host, load above capacity, and CPU pressure about 30-33% in the incident windows. |
| T3 | A post-processing entity-artifact refresh after large batches performs too much work in one wave. | confirmed active phase; exact per-incident lock duration unresolved | Logs show entity refresh queued immediately before both watchdog windows (`feeds=177` and `feeds=202`). Cloud background metrics show `entity_artifacts` active through the watchdog windows. Admin lifetime metrics show long entity writer lock holds, including max about `299,272 ms`, but Cloud did not retain per-incident entity lock rows for June 14/15. |
| T4 | Public compare/compose/IP lookup handlers can perform enough work or share enough resources to compound background load. | primary trigger refuted; residual risk confirmed | Code review shows compare serves a precomputed artifact (`pkg/web/routes.go:140`) and does not compute pairwise overlap on request. Compose and IP lookup can compute on request (`pkg/engine/public.go:354`), but incident telemetry did not show public request load. |
| T5 | The watchdog implementation proves goroutine scheduling only, not actual public HTTP responsiveness, and may send pings even when public APIs are bad. | confirmed | Watchdog is a timer goroutine (`pkg/web/server_run.go:221`) and does not call `/healthz`. `/healthz` itself is lock-free (`pkg/web/routes.go:31`). Therefore watchdog success/failure is not a direct public HTTP SLO. |
| T6 | Go GC/GC-assist pressure is a primary cause of the stalls. | unresolved, weak evidence | No per-incident GC/assist evidence was found. Partial abort dumps did not prove GC as root cause. Current admin status exposes GC totals, but incident-specific GC pause/assist data was not available from the collected evidence. |
| T7 | File I/O and generated artifact publication create enough kernel file-cache, writeback, or I/O wait pressure to stall the process. | confirmed contributor | Cloud showed app disk I/O about 149-218 MiB/s, disk utilization about 84-88%, I/O full pressure about 39-40%, and dirty/writeback about 96-100 MiB. Code paths write comparisons, metadata, public artifacts, entity sidecars, and retention files. |
| T8 | Systemd notify itself can block or fail under pressure because notification uses a fresh Unix datagram dial/write with no deadline. | unresolved, plausible but not proven | Code shows `Notify` opens and writes a Unix datagram without a deadline (`pkg/systemd/notify.go:52`), but no incident goroutine dump proved it blocked there. It remains a specific measurement target, not a conclusion. |
| T9 | Repeated downloader/feed failures create unnecessary scheduler churn that materially contributes to the incident. | refuted as primary for observed incidents | The two watchdog incidents correlate with completed 402-feed processing batches and entity refresh starts. Downloader failures may exist operationally, but they do not explain these two post-install watchdog restarts. |
| T10 | The watchdog restarts are caused by a specific long critical section or shared lock blocking status/API paths. | partially refuted for public health; confirmed long lock risk for entity writer | `/healthz` is lock-free and public compare is artifact-only, so no evidence shows a shared lock directly blocking public health. Admin metrics prove long `entity.writer_lock_hold` durations, so the entity writer lock remains a resource and latency risk for entity publication paths. |

## Subagent Workstreams

Read-only agents must not modify files, stop services, restart services, kill
processes, or print secrets.

1. Code path and lock analysis:
   identify exact background paths that can monopolize CPU, memory, I/O, or
   locks while public HTTP handlers are active.
2. Public API and serving analysis:
   identify which public endpoints are cache-only and which compute on request;
   classify compare, compose, and IP lookup cost under production-sized data.
3. Production log and cgroup timeline:
   build the exact timeline around watchdog failures from journal, app logs,
   systemd status, cgroup counters, and admin status snapshots.
4. Netdata telemetry:
   query Netdata Cloud/agent metrics for the host and service around watchdog
   failure windows and correlate CPU, memory, I/O, pressure, and process metrics.
5. Processing/entity-artifact workload:
   map which post-batch phases execute after broad updates and estimate whether
   the work is necessary, avoidable, or necessary-but-unoptimized.

## Implications And Decisions

- The user accepted the 2026-06-19 goal-by-goal implementation process.
- Goal 1 code changes are allowed only after the baseline tests, reviewer
  synthesis, and Goal 1 acceptance criteria are recorded in this SOW.
- Findings that require product semantics decisions must be skipped or mapped to
  later goals instead of blocking the current implementation slice.

## Plan

1. Run Goal 2 baseline validation on the current post-Goal-1 tree.
2. Implement the accepted Goal 2 first-slice context/cancellation and validation
   contracts.
3. Update specs for the new bounded-work contracts.
4. Run focused and strict validation.
5. Run external implementation review and iterate.
6. Run Goal 3 external analysis reviewers against the recorded Goal 3 gate.
7. Synthesize Goal 3 findings into accepted implementation criteria or rejected
   findings with evidence.
8. Run baseline comparison validation before any Goal 3 code edits.
9. Implement and review only the accepted Goal 3 slice.
10. Map remaining valid findings to Goals 4-7 or the next focused SOW section.

## Superseded Diagnosis Remediation Plan

This was the initial diagnosis remediation plan from 2026-06-15. The
2026-06-19 user decision supersedes its ordering. In particular, public/worker
split is not accepted as the primary solution to this SOW; it remains a
possible later hardening pattern only after backend resource control is fixed.

### Superseded 1. Separate Public Serving From Heavy Batch Work

Recommendation class: long-term-best.

Finding:

- The public site, public APIs, admin APIs, scheduler, downloader, processing,
  comparison generation, entity refresh, artifact publication, and watchdog run
  in one process and one cgroup.
- The observed failures happened when the process was under simultaneous CPU,
  memory, and I/O pressure.

Required change:

- Run public HTTP/API serving in a resource-light process that reads published
  artifacts and accepts only explicitly dynamic public actions.
- Run ingest/processing/entity/artifact production in a separate worker process
  or separately limited service.
- Keep public API behavior and URLs unchanged.
- Preserve the current artifact directory contract; the public process must be a
  reader of already-published artifacts.

Affected modules:

- `cmd/`
- `pkg/web/server_run.go`
- scheduler startup and run mode wiring
- install/systemd unit generation in `install.sh`
- specs under `.agents/sow/specs/operating-principles.md`,
  `.agents/sow/specs/files-layout.md`, and `.agents/sow/specs/pipeline.md`

Expected impact:

- Removes the main production safety flaw: public requests and public liveness
  no longer compete directly with batch CPU, memory reclaim, and artifact write
  pressure in the same process/cgroup.
- This is the only option in the current evidence set that can give a hard
  public-serving isolation boundary.

Behavior guarantee:

- Public endpoints must return the same data from the same published artifacts.
- No feed, merge, retention, comparison, entity, or lookup semantics may change.

Validation:

- Existing Go tests first, before changing behavior.
- Add integration tests proving public serving starts without scheduler work and
  reads the configured published artifact directory.
- Add a stress test that runs a heavy processing worker while public `/healthz`,
  `/api/v1/status`, static feed artifacts, compare, and representative lookup
  endpoints stay within defined latency budgets.
- Confirm with Netdata that public-service cgroup CPU, memory, and I/O stay
  bounded while worker cgroup performs heavy processing.

Risk:

- This is an operational architecture change. It is larger than a local
  optimization, but it directly addresses the public-site unresponsiveness risk.
- If this is rejected, the remaining steps reduce probability and severity, but
  they do not provide the same isolation guarantee.

2026-06-19 decision:

- rejected as the primary solution for this SOW;
- retained only as possible later availability hardening;
- backend resource-control goals below are the accepted primary path.

### Accepted Goal 4. Bound Entity Artifact Refresh Waves

Recommendation class: surgical first, then long-term-best if needed.

Finding:

- Entity refresh starts immediately after the two failing broad batches.
- Cloud shows `entity_artifacts` active through both watchdog windows.
- Admin metrics show long entity writer lock holds and many ASN/country artifact
  updates.

Required change:

- Process entity refresh in bounded chunks instead of one large mutation wave.
- Release `entityArtifactsMu` between chunks.
- Coalesce pending feed changes without letting one wave monopolize the writer.
- Avoid rewriting unchanged ASN/country public files and sidecars by using
  deterministic content/signature checks before atomic writes.
- Keep current entity output semantics unchanged.

Affected modules:

- `pkg/engine/entity_refresh_queue.go`
- `pkg/engine/background_tasks.go`
- `pkg/engine/entity_surgical_refresh.go`
- `pkg/engine/entity_artifacts_write.go`
- entity artifact tests

Expected impact:

- Reduces long single critical sections.
- Reduces I/O amplification from unchanged entity artifacts.
- Reduces memory and writeback pressure during post-processing.

Behavior guarantee:

- The same entity artifacts must be present after refresh.
- If a refresh is interrupted, the repair/rebuild path must restore a complete
  entity surface.

Validation:

- Unit/behavior tests for chunked refresh equivalence against full refresh.
- Fixture with large affected ASN/country sets to compare file counts, content,
  mtimes, and interrupted-repair behavior.
- Runtime validation that entity writer lock hold max falls from minutes to a
  bounded chunk target.

### Goal 4 Definition - 2026-06-19

Status: ready for third external analysis review after second-round blocker fixes.

Purpose:

- Reduce entity artifact I/O amplification and writer-lock hold work without
  changing public country/ASN entity output semantics.
- Keep public/admin entity surfaces coherent while SOW105 continues to reduce
  resource pressure inside the existing single-process architecture.

Scope correction from the accepted sketch:

- The accepted sketch bundled two distinct changes:
  1. skip unchanged entity artifact writes;
  2. split entity refresh waves and release `entityArtifactsMu` between chunks.
- The first implementation slice is limited to no-output-change producer-level
  write avoidance for country/ASN private sidecars and public JSON detail
  artifacts.
- Entity refresh chunking remains in SOW105 as a later Goal 4 slice, but it is
  not safe as the first slice because current entity index integrity checks do
  not deeply validate stale-but-present country/ASN index contents. Lock release
  between chunks is allowed only after each chunk boundary can be proven to be a
  complete, coherent committed entity state, including indexes and public/private
  detail pairs.

Evidence reviewed:

- `pkg/engine/entity_refresh_queue.go:242` drains one pending feed-refresh wave,
  then `pkg/engine/entity_refresh_queue.go:271` runs the whole wave under
  `withEntityArtifactMutation`.
- `pkg/engine/background_tasks.go:288` acquires `entityArtifactsMu` for the whole
  mutation callback and records `entity.writer_lock_wait` and
  `entity.writer_lock_hold`.
- `pkg/engine/entity_surgical_refresh.go:37` runs the surgical refresh sequence:
  load feed deltas, patch country details, patch ASN details, patch indexes, and
  publish private/public batches.
- `pkg/engine/entity_surgical_refresh.go:216` and
  `pkg/engine/entity_surgical_refresh.go:287` write changed country/ASN private
  sidecars, public JSON payloads, and markdown artifacts.
- `pkg/engine/entity_artifacts_write.go:379` and
  `pkg/engine/entity_artifacts_write.go:405` do analogous full/repair detail
  writes.
- `pkg/engine/web_batch.go:164` can keep byte-identical live files during
  publish, but this happens after the producer has already written staged bytes,
  so it does not remove staging write amplification.
- `pkg/engine/helpers.go:596` and `internal/fileutil/fileutil.go:91` show the
  atomic writer path always creates/writes a temporary file before rename.
- `.agents/sow/specs/integrity.md:116` requires deliberate logical mtimes for
  integrity-participating files.
- `.agents/sow/specs/integrity.md:184` requires private and public entity
  detail files to share the same logical mtime when rewritten or
  freshness-touched together.
- `.agents/sow/specs/pipeline.md:577` already requires surgical entity refresh
  to skip private/public JSON rewrites when patched actor sidecars are
  semantically unchanged, preferring metadata-only touches when freshness is
  needed.
- `.agents/sow/specs/pipeline.md:707` and `.agents/sow/specs/pipeline.md:715`
  allow byte-identical publication and producer-level skip only when bytes,
  mode, ownership handling, and producer-assigned logical mtime are equivalent.
- `pkg/engine/entity_integrity_detail_scan.go:228` checks country/ASN index
  existence, but current integrity does not prove stale-but-present index
  content is semantically current.

Problem/root-cause model:

- Confirmed fact: entity refresh waves can hold the entity mutation lock for a
  long mutation callback and the production telemetry recorded long
  `entity.writer_lock_hold` durations.
- Confirmed fact: the entity detail producers can stage private JSON and public
  JSON for many affected actors, even when the generated content and logical
  freshness are already equivalent to committed files.
- Working theory: avoiding producer-stage writes for already-equivalent entity
  details will reduce filesystem writeback, staged-file churn, publish compare
  work, and lock-held time for refresh/repair paths with many unchanged or
  semantically equivalent actors.
- Confirmed risk: splitting waves and releasing `entityArtifactsMu` before
  proving index semantics can publish partial states that current integrity may
  not detect after the background task finishes.

Affected contracts and surfaces:

- Public country and ASN JSON payloads and markdown pages must be byte-identical
  to the current implementation after refresh or repair. Markdown output remains
  on the existing staged-write path in this first slice.
- Private entity sidecars must remain byte-identical or logically equivalent and
  have the same required logical mtimes as before.
- Generated-file ledger entries and public serving paths must still observe the
  same artifacts.
- Admin background task visibility, entity integrity busy state, and repair
  semantics must not regress.

Existing patterns to reuse:

- Existing entity unchanged-touch behavior in
  `pkg/engine/entity_surgical_refresh.go:236` and
  `pkg/engine/entity_surgical_refresh.go:307`.
- Existing selected-repair unchanged detail behavior in
  `pkg/engine/entity_artifact_selected_repair.go:108` and
  `pkg/engine/entity_artifact_selected_repair.go:155`, but only after adding
  byte-level public/private equivalence checks where the current pattern relies
  only on sidecar equality and file existence.
- Existing publication equivalence rules in `pkg/engine/web_batch.go`.
- Existing deterministic entity fixtures and pipeline integrity scenarios.

Implementation plan for first slice:

- Add a small engine-local helper that proves an existing generated file already
  equals the bytes, mode, and logical mtime the producer would publish. If the
  check cannot prove equivalence, fall back to the normal staged atomic write.
- The helper MUST use `os.Lstat`, MUST require a regular file, MUST reject
  symlinks and other non-regular paths, MUST compare raw bytes, MUST require
  `generatedFileMode`, and MUST compare logical mtime in UTC.
- The helper MAY metadata-touch an existing file only when bytes and mode are
  already correct but logical mtime differs. If bytes differ, mode differs, the
  file is missing, the path is not regular, any check errors, or ownership
  correction is configured for a public artifact, it MUST fall back to the
  existing staged write/publish path.
- Public JSON producer skips MUST be disabled when `runtime.WebOwner` is
  configured, so the existing publish path remains responsible for ownership
  correction. Private entity sidecars are not affected by `WebOwner`.
- Apply the helper to country/ASN private sidecars and public JSON payloads in
  surgical refresh and selected/full repair detail writers where the producer
  already has the final bytes and logical mtime.
- Explicit allow-list:
  - `patchCountryDetail`
  - `patchASNDetail`
  - `stageSelectedCountryDetail`
  - `stageSelectedASNDetail`
  - `writeCountryDetail`
  - `writeASNDetail`
- Explicit deny-list:
  - `stageHealthTransitionCountryPayload`
  - `stageHealthTransitionASNPayload`
  - entity feed-sidecar writers and touch paths
  - country/ASN markdown writers
  - country/ASN index writers
  - sitemap writers
  - home aggregate writers
  - version marker writers
- Selected repair MUST stop treating private-sidecar `DeepEqual` plus file
  existence as sufficient. It may skip only after private sidecar bytes, public
  JSON bytes, mode, and logical mtimes are proven current. Malformed/stale
  public JSON MUST fall back to the write path even when the private sidecar is
  unchanged. Markdown retains the existing behavior in this slice.
- When a public artifact write is skipped or metadata-touched in a call site
  that would normally append an `output.GeneratedFile`, the implementation MUST
  preserve generated-file accounting with the same public path, timestamp, and
  redistributable flag. The mixed case where some public artifacts are staged
  and others are skipped/touched MUST preserve `push_to_git_web` behavior; either
  pass an equivalent union of published and skipped/touched public paths to
  `syncGeneratedFiles`, or update `syncGeneratedFiles` with tests proving the
  same Git sync result. Skipped private sidecars do not use `output.GeneratedFile`.
- Preserve feed-sidecar freshness touches, delete handling, health-transition
  refresh semantics, sitemap stale-shard cleanup, and index writes in this
  slice.
- Add bounded counters for skipped equivalent writes, metadata touches, and
  fallback writes without high-cardinality labels. Counter names must be stable
  and scoped by path family and operation, for example:
  - `entity.refresh.country_sidecar_skip_equivalent`
  - `entity.refresh.country_sidecar_touch_equivalent`
  - `entity.refresh.country_sidecar_fallback_write`
  - same shape for `country_public`, `asn_sidecar`, and `asn_public`
  - same shape under `entity.repair.*` for selected/full repair paths
- Progress accounting must advance exactly as before for each actor, regardless
  of skip/touch/write outcome.
- The helper contract itself must have focused tests, independent of the broader
  entity refresh call-site tests.

Spec update plan:

- Update `.agents/sow/specs/pipeline.md` so the producer-level skip rule
  explicitly covers country/ASN entity detail producers, not only feed-scoped
  artifacts.
- Update `.agents/sow/specs/integrity.md` only if implementation changes
  timestamp or selected-repair semantics beyond the existing private/public
  mtime agreement rule. Current intent is to preserve the existing rule.
- Update `.agents/sow/specs/operating-principles.md` if final implementation
  adds a durable bounded-work/operator-visible counter contract.

Validation plan for first slice:

- Run existing targeted tests before implementation to establish the baseline:
  `timeout 1800 go test -count=1 ./pkg/engine -run 'TestRefreshEntityArtifacts|TestBuildFeedEntityDelta|TestCheckEntityArtifactsIntegrity|TestPipelineIntegrityScenario|TestEntityArtifactRefreshQueue|TestBackground'`.
- Add behavior tests proving:
  - equivalent country/ASN private sidecar and public JSON detail artifacts are
    not rewritten and keep the required logical mtime;
  - byte-identical detail artifacts with stale logical mtime are touched in
    place instead of rewritten;
  - malformed or stale public detail artifacts are not hidden by private-sidecar
    equality and are repaired by normal writes;
  - selected repair rewrites a stale/malformed public JSON artifact even when the
    private sidecar is unchanged;
  - private sidecar equality does not skip public JSON when feed health
    materialization changes the public payload bytes;
  - public artifact producer skip is disabled when `runtime.WebOwner` is
    configured;
  - a live symlink or any non-regular live path is not treated as equivalent and
    is replaced through the normal staged publication path;
  - surgical refresh output remains equivalent for public/private entity detail
    content and integrity is clean;
  - selected repair does not regress public/private mtime agreement.
- Add focused helper tests proving:
  - identical bytes, generated mode, and equal logical mtime returns skip;
  - identical bytes and generated mode with stale logical mtime returns touch;
  - different bytes, wrong mode, missing path, symlink, non-regular path,
    comparison error, and public `WebOwner` configuration all fall back to write.
- Add a progress-accounting test proving actor progress advances identically for
  skip, touch, and write outcomes.
- Add an entity refresh benchmark or focused test counter that demonstrates
  skipped producer writes for an unchanged/equivalent actor set.
- Add a generated-file accounting and `push_to_git_web` mixed-case test where one
  public artifact is staged and another is skipped/touched in the same publish
  wave.
- Run:
  - `timeout 1800 go test -count=1 ./pkg/engine -run 'TestRefreshEntityArtifacts|TestBuildFeedEntityDelta|TestCheckEntityArtifactsIntegrity|TestPipelineIntegrityScenario|TestEntityArtifactRefreshQueue|TestBackground'`
  - `timeout 1800 go test -race -count=1 ./pkg/engine -run 'TestRefreshEntityArtifacts|TestPipelineIntegrityScenario|TestEntityArtifactRefreshQueue|TestWithBackgroundTask'`
  - `timeout 1800 make test-strict`
  - `timeout 1800 make test`
  - `timeout 1800 make build`
  - `timeout 1800 make lint`
  - `timeout 1800 make staticcheck`
  - `timeout 1800 make golangci-lint`
  - `timeout 1800 make race`

Out of scope for first slice:

- Releasing `entityArtifactsMu` between chunks.
- Changing health-transition semantics.
- Skipping feed-sidecar freshness updates.
- Skipping country/ASN markdown writes.
- Skipping country/ASN index writes before index mtime/content integrity is
  explicitly specified and tested.
- Changing public API behavior, entity URLs, artifact names, markdown content, or
  repair fallback semantics.

Sensitive data handling:

- This slice uses only source paths, test fixtures, and sanitized telemetry
  summaries already recorded in this SOW. No raw production IP lists, customer
  data, tokens, host-private data, or service logs will be written to durable
  artifacts.

Open decisions:

- None for the first slice. The user approved autonomous implementation details
  when output semantics are preserved. Lock chunking remains a later Goal 4
  slice and requires a separate gate because it changes the mutation boundary.

First external analysis review - 2026-06-19:

- Reviewers: `glm`, `minimax`, `mimo`, `kimi`, `qwen`, `deepseek`.
- Result: not ready. Two reviewers returned `READY FOR IMPLEMENTATION`; four
  returned `NOT READY`.
- Accepted blockers fixed in this Goal 4 definition:
  - helper call-site allow-list and health-transition/index/sitemap deny-list
    were missing;
  - `WebOwner` ownership-correction fallback was missing;
  - `os.Lstat` regular-file/symlink rejection was missing;
  - generated-file accounting and `push_to_git_web` semantics were not
    specified;
  - markdown skip required render-to-buffer and determinism tests;
  - selected repair needed an explicit public JSON/markdown byte-equivalence
    correctness fix;
  - byte-identical but stale-mtime artifacts needed explicit metadata-touch
    semantics;
  - counter names and bounded label shape needed to be pinned down;
  - the spec update plan needed to be explicit.
- Non-blocking clarification accepted:
  - first-slice write avoidance reduces I/O and some lock-held write work, but
    it does not bound entity writer lock hold time by itself. The larger
    lock-hold reduction belongs to the later chunking slice after index
    integrity or a publish barrier is proven.

Second external analysis review - 2026-06-19:

- Reviewers: `glm`, `mimo`, `kimi`, `qwen`, `deepseek`; `minimax` timed out with
  no final verdict and was treated as a technical failure for this review round.
- Result: not ready. Three reviewers returned `READY FOR IMPLEMENTATION`; two
  returned `NOT READY`.
- Accepted blockers fixed in this Goal 4 definition:
  - markdown skip scope was still ambiguous and carried high implementation risk
    for low expected return, so markdown write avoidance was removed from the
    first slice;
  - focused tests for the equivalence helper contract were missing;
  - generated-file accounting needed an explicit mixed staged/skipped
    `push_to_git_web` test.
- Findings rejected as blockers:
  - broad JSON non-determinism was raised speculatively. The implementation must
    compare the exact bytes produced by the existing JSON writer path; if those
    bytes are not stable for a given payload, the helper will fall back to write
    and tests will expose low skip return. This is a performance-risk note, not a
    correctness blocker.
  - surgical unchanged-touch paths using `entityDetailFilesExist` are preserved
    in this slice to avoid changing existing semantics outside the selected
    producer-write path.

### Superseded Goal 3 Sketch. Make Comparison Generation Incremental By Signature

This sketch is superseded by `Goal 3 Definition - 2026-06-19` above. External
analysis rejected signature-ledger reuse for the first implementation slice
because there is no cheap safe validation key for current normalized range
content. The accepted first slice is pair-result caching only, with signatures
computed from current canonical ranges.

Recommendation class: long-term-best.

Finding:

- `metadata.write_comparison_files` is the largest measured cost center.
- Current code prepares all public set infos and runs updated-vs-all candidate
  comparisons.
- Sanitization scans live and staged comparison artifacts after comparison
  generation.

Required change:

- Persist per-feed comparison inputs: content signature, range bounds, range
  count, prefix/sparse-prefix summaries, and generated comparison metadata.
- Recompute only pairs whose input signatures changed.
- Move broad `sanitizeComparisonArtifacts` into explicit repair/migration paths
  or scope it to changed artifacts.

Affected modules:

- `pkg/engine/output_comparison.go`
- `pkg/engine/output_comparison_helpers.go`
- comparison metadata/artifact tests
- pipeline integrity specs

Expected impact:

- Avoids repeated all-public-set preparation and broad pair candidate work when
  most feed signatures are unchanged.
- Reduces CPU and disk I/O during normal scheduled runs.

Behavior guarantee:

- `{feed}_comparison.json` output must remain byte-equivalent for unchanged
  inputs, except for intentionally normalized ordering/format already defined by
  tests.
- Missing or stale comparison artifacts must be repaired deterministically.

Validation:

- Golden comparison outputs before and after optimization.
- Test changed one feed, changed many feeds, deleted artifact repair, and
  unchanged feed no-op behavior.
- Benchmark with production-like public feed counts and large sets.

### Accepted Goal 5. Optimize Retention Update For Large Feeds

Recommendation class: surgical.

Finding:

- `sources.update_retention` is a measured large cost center.
- Current retention reconciliation intersects each cohort with the current set
  before it can decide that no write is needed.

Required change:

- Add cheap no-op detection for retention cohorts when the source set did not
  remove ranges affecting the cohort.
- Keep streaming/out-of-core behavior for large feeds.
- Avoid holding large in-memory sets longer than necessary.

Affected modules:

- `pkg/engine/process.go`
- `pkg/engine/retention_update.go`
- retention tests and fixtures

Expected impact:

- Reduces CPU and I/O for large retention feeds.
- Reduces memory pressure during source processing before metadata/publish.

Behavior guarantee:

- Retention history, first-seen/removed counts, cohort files, and public
  retention artifacts must remain semantically identical.

Validation:

- Golden retention fixtures before and after change.
- Tests for no removals, partial removals, full cohort removal, and interrupted
  write repair.
- Benchmark on large-feed fixtures.

### Goal 5A Diagnostic Accounting Gate - 2026-06-19

Recommendation class: surgical.

Purpose:

- Add enough structured engine accounting to understand long source/retention
  runs from ordinary service logs.
- Every new diagnostic metric/log field must declare the unit of work, total
  work size when known, completed work, completion percentage when bounded, and
  processing rate.
- Diagnostic summaries must answer, per phase: which feeds were processed, how
  large they were, how fast they were processed, and how many operations ran per
  operation kind.
- Preserve feed outputs, public artifacts, retention semantics, scheduling
  semantics, and admin/public API contracts.
- Use the new evidence to decide Goal 5 retention optimizations later; do not
  optimize retention algorithms in this slice.

Production evidence reviewed:

- The production-candidate service had additional watchdog failures on
  `2026-06-19`; systemd reported watchdog timeout, SIGKILL after stop timeout,
  and memory peak near the configured 1.5 GiB soft limit.
- The restarted service reported `current_phase="sources"` with active feed
  `dronebl_anonymizers`.
- Earlier runtime evidence showed `dronebl_anonymizers` had parse and finalize
  timings but no completed `sources.update_retention` timing, while
  `retention.csv` was appending current timestamp rows.
- The DroneBL retention directory had very large `new/` cohort state, and
  `.cache.json` showed prior processing for the same feed took about 4 hours.
- Code evidence shows `processAndCommit()` finalizes latest artifacts before
  `updateRetentionFromDiff()`, and `reconcileRetentionCohorts()` then scans
  retention cohort files in `lib/<feed>/new`.

Problem/root-cause model for this slice:

- Fact: the existing run metrics record completed operations only.
- Fact: a watchdog kill during a long operation can prevent the final
  `run finished` log and can leave the operator without per-operation counts.
- Working theory: the current evidence points to long retention cohort
  reconciliation for DroneBL-sized feeds, but the next code change must measure
  that path before changing its algorithm.

Affected contracts and surfaces:

- Logs: new structured operational logs are added for run summaries, progress
  checkpoints, feed summaries, and retention reconciliation summaries.
- Admin API/UI: `/api/v1/admin/status` exposes bounded active-operation
  progress so the "Being Processed Now" admin list can show work size,
  completion percentage, and rate.
- Public APIs: unchanged.
- Generated feed, merge, comparison, entity, retention, and web artifacts:
  unchanged.
- Runtime configuration: unchanged.

Existing patterns to reuse:

- Reuse `runMetrics`, `observeRunOperation`, and `observeFeedOperation` for
  completed operation timings.
- Reuse existing process/runtime status patterns already used by the admin
  status surface, but do not expose secrets or raw feed content in logs.
- Keep retention code streaming/file-backed; do not add full-feed in-memory
  copies.

Implementation plan:

1. Extend current-run metrics to include bounded operation counters in addition
   to timings.
2. Add a low-frequency run progress logger that snapshots phase, active feeds,
   active long operations, operation timings, counters, process memory, CPU, I/O,
   open files, and Go runtime state.
3. Add run-end diagnostic summary logging before `markRunEnd()` clears current
   run state.
4. Add feed-level processing summary logging after each source worker completes.
5. Instrument retention reconciliation with exact cohort counts, skipped
   entries, processed cohorts, rewritten cohorts, deleted cohorts, kept IPs, and
   removed IPs; update active progress during long scans.
6. Surface active-operation progress in admin status and render it in the
   "Being Processed Now" admin column with a progress bar, work-size text,
   completion percentage, and processing rate.
7. Make phase summaries include phase-scoped operation counts, phase work unit,
   work completed/total, completion percentage, and rate.
8. Make feed summaries include feed name, input bytes, entries, unique IPs,
   elapsed time, byte/entry/IP rates, and per-feed operation counts.

Validation plan:

- Run retention-focused tests before edits where practical.
- Add behavioral tests for retention reconciliation accounting, run summary log
  fields, admin status active-operation exposure, and the admin progress display
  without asserting fragile string formatting.
- Run focused engine tests after edits.
- Do not install or restart production as part of this slice unless explicitly
  requested after validation.

Sensitive data plan:

- Logs and SOW notes must not include raw IP ranges, token-bearing URLs,
  credentials, Cloud identifiers, customer identifiers, or raw feed bodies.
- Log aggregate counts, durations, file counts, bytes/syscall counters,
  configured feed names, and safe phase/operation identifiers only.

Open decisions:

- None for this diagnostic slice. Any later retention algorithm change remains
  a separate Goal 5 decision and validation gate.

### Accepted Goal 1 Candidate. Stop Zero-Success Batches From Forcing Global Heavy Work

Recommendation class: surgical.

Finding:

- Scheduler currently enters `RunOnce` with `Reprocess=true` once queued work is
  admitted.
- This was not the primary cause of the two observed incidents because both had
  402 successful updates, but it remains unnecessary work for zero-success or
  all-failed batches.

Required change:

- Preserve full processing for admitted successful bodies.
- Avoid global publication/comparison/entity work when a batch has no successful
  updates and no artifact-repair reason.

Affected modules:

- `pkg/scheduler/processing_loop.go`
- `pkg/engine/run_pipeline.go`
- scheduler/engine tests

Expected impact:

- Prevents avoidable heavy work after failed or empty processing batches.
- Does not address the observed broad-update incidents by itself.

Behavior guarantee:

- Successful feed updates still run the full required pipeline.
- Failed downloads/parses do not silently publish stale or partial outputs.

Validation:

- Tests for all-success, mixed-success, all-failed, empty, repair, and manual
  reprocess cases.

### Accepted Goal 6. Put Explicit Budgets Around Dynamic Public Endpoints

Recommendation class: surgical.

Finding:

- Public compare is static and not the observed problem.
- Public compose and IP lookup can perform real work on request.
- No incident evidence shows public request load caused the restarts, but these
  endpoints share the same process today.

Required change:

- Keep compare artifact-only.
- Bound compose and lookup CPU/memory/time with request budgets and admission
  limits.
- Avoid holding shared caches or locks while opening/reading large set files.

Affected modules:

- `pkg/web/routes.go`
- `pkg/engine/public.go`
- `pkg/engine/query.go`
- public API tests

Expected impact:

- Prevents a public traffic spike from compounding background processing load.
- Needed even if public serving is split, because dynamic endpoints still need
  request-level protection.

Behavior guarantee:

- Existing valid requests continue to work within documented limits.
- Over-budget requests fail with explicit HTTP errors instead of degrading the
  whole service.

Validation:

- Public API behavior tests for normal and over-budget requests.
- Concurrency stress test for compose and lookup while static artifact serving
  remains responsive.

### Accepted Goal 7. Make Watchdog Prove The Correct Thing

Recommendation class: surgical after resource fixes.

Finding:

- The current watchdog is a timer-based systemd notification.
- It does not prove public HTTP responsiveness.
- Lowering `WatchdogSec` from 5 minutes to 5 seconds before fixing resource
  pressure would restart faster, but it would not solve public unresponsiveness.

Required change:

- After resource isolation/bounding, make the watchdog depend on an internal
  cheap health probe that exercises the public HTTP serving path or equivalent
  event-loop responsiveness.
- Track notify failures explicitly.
- Consider a shorter watchdog only after the service can reliably meet that
  deadline under processing stress.

Affected modules:

- `pkg/web/server_run.go`
- `pkg/systemd/notify.go`
- `install.sh`
- operations docs/specs

Expected impact:

- Prevents watchdog pings from hiding a wedged public-serving path.
- Makes failures faster and more meaningful once the underlying resource
  pressure is fixed.

Behavior guarantee:

- Watchdog changes must not mark the service healthy unless public serving is
  actually able to respond.

Validation:

- Unit tests for watchdog health gate.
- Integration test that blocks the public serving path and confirms watchdog
  notifications stop.
- Production soak with Netdata confirmation before reducing systemd timeout.

### Specific Non-Goals

- Do not treat this as a generic observability task.
- Do not increase memory limits as the primary fix. More memory may delay the
  failure, but the evidence shows unbounded batch/public coupling plus heavy
  CPU/I/O work.
- Do not "hide" the issue by weakening watchdog behavior.
- Do not change feed output semantics, public API results, or artifact names as
  part of performance work.

## Execution Log

### 2026-06-15

- Created diagnosis SOW from verified repo evidence and the user request.
- Spawned read-only subagents for code path/lock analysis, public API serving
  cost, production logs/cgroups, Netdata Cloud telemetry, processing/entity
  workload, and focused historical OTEL extraction.
- Queried the production-candidate host read-only for systemd state, cgroup
  memory/pressure counters, journal namespaces, public health/status, and admin
  status.
- Queried Netdata Cloud through the documented project skill path without
  recording tokens or raw sensitive identifiers.
- Classified the working theories and added the remediation plan.
- No code, config, service, restart, install, or daemon state change made. A
  temporary status JSON capture was written under `/tmp` on the host for parsing
  and removed after use.

### 2026-06-19

- Rechecked the production-candidate service read-only and recorded that the
  service was still experiencing watchdog restarts without service-journal OOM
  evidence.
- Recorded the user-accepted goal-by-goal resource-control process and superseded
  the public/worker split as the primary solution.
- Ran six read-only external reviewers for Goal 1: `glm`, `minimax`, `kimi`,
  `mimo`, `deepseek`, and `qwen`.
- Ran baseline validation before code changes:
  `timeout 1800 go test -count=1 ./pkg/engine ./pkg/scheduler ./pkg/web`.
- Implemented the first Goal 1 slice:
  - scheduler queued processing now sets `Reprocess` from the actual drained
    batch: explicit reprocess/repair/provider-default items preserve
    reprocess intent even when batched with ordinary scheduled work, while a
    purely ordinary scheduled/manual-run/manual-recheck batch does not force
    reprocess;
  - normal comparison generation no longer performs a full live/staged
    comparison-artifact sanitize sweep after every comparison run;
  - tests were added for scheduler reprocess reason classification, manual
    reprocess preservation, comparison zero-row merge behavior, and untouched
    comparison artifact non-sanitization during ordinary comparison generation.
- Updated `.agents/sow/specs/pipeline.md` to record that ordinary
  downloader-admitted processing is not explicit reprocess/repair intent.
- Ran external implementation review. Five requested reviewers returned final
  reports and one reviewer produced extensive read-only checks but no final
  verdict before being stopped; only concrete evidence from the partial report
  was used.
- Accepted and fixed external review findings:
  - mixed processing batches could lose repair/reprocess intent if the
    operator-facing combined reason collapsed to `manual_run`; fixed by scanning
    all batch item reasons for `Reprocess`;
  - `sanitizeComparisonArtifacts` became dead private code after removal from
    the hot path; removed the helper and its direct helper test instead of
    keeping an unused repair path;
  - the scheduler tests needed batch-level coverage; added mixed-batch and
    `combineReasons` tests;
  - the comparison non-sanitization test needed to prove paired artifacts are
    still touched; added a staged `beta_comparison.json` assertion.
- Rejected/refuted external review concerns:
  - provider/database staged promotion is still protected by
    `databaseSourceSelected(opts.Selected)`, so provider downloads force
    publication and `BeforePublish`;
  - artifact parent promotion is not regressed: successful child processing
    makes `report.Updated` non-empty, while all-failed child processing did not
    promote the parent under the existing `successItems` logic either;
  - manual recheck remains non-reprocess by design: docs define it as forced
    downloader-stage work, while manual reprocess is the rebuild action;
  - zero-overlap comparison rows still have a deterministic correctness path:
    touched artifacts remove them through `mergeCompareRows`, and existing
    malformed rows are rejected by comparison payload integrity validation and
    repaired through reprocess rather than by a broad hot-path sweep.
- Ran second external review iteration after fixes. Five reviewers returned
  final `PRODUCTION GRADE` verdicts for the Goal 1 first slice. The remaining
  reviewer ran additional read-only checks and validation without producing a
  final verdict before being stopped; no concrete blocker had been raised.
- Second-review findings accepted as non-blocking follow-ups:
  - added a cheap invariant assertion that failure status transitions do not
    advance `SourceDate`, `ProcessedDate`, or `StartedDate`, because
    zero-success skip safety depends on failed feeds not advancing the
    processed timestamp;
  - optionally add scheduler integration coverage for a zero-success scheduled
    batch and a `ProviderDefaults` display-reason combination case;
  - keep the byte-identical comparison write suppression finding explicitly
    mapped to a later Goal 1 slice, not silently implied by the first slice.
- Second-review findings rejected as blockers:
  - mixed-batch display reason collapsing to `manual_run` is existing
    operator-facing display behavior and does not control `Reprocess` after the
    fix; it can be polished later if operator clarity becomes important;
  - runtime malformed comparison rows are repaired through startup or operator
    integrity reprocess, not through a per-run broad sweep. This is the intended
    Goal 1 trade-off.
- Began the second Goal 1 implementation slice for byte-identical comparison
  write suppression:
  - scope is limited to feed-scoped `*_comparison.json` artifacts;
  - a staged write may be skipped only when the existing target artifact already
    has identical bytes, generated-file permissions, and the same feed
    processing logical mtime;
  - if the file is missing, different, has a stale mtime, has wrong
    permissions, or ownership correction is configured, the writer must use the
    existing staging/publication path so normal repair behavior is preserved.
- Updated `.agents/sow/specs/pipeline.md` to record the producer-side
  byte-identical skip contract.
- Implemented the second Goal 1 slice:
  - `writeMergedComparisonRowsForFeed` now builds the exact JSON bytes and
    skips staging only when the stage target or live target is already
    byte-identical, has generated-file permissions, and has the feed processing
    logical mtime;
  - configured `WebOwner` disables the producer-side skip so the existing
    staged publication path can still apply ownership correction;
  - stale mtimes, wrong modes, changed content, unreadable files, missing files,
    symlinks, and already-present stale stage files all fall back to the normal
    atomic write path;
  - follow-on readers already prefer staged comparison artifacts and then live
    artifacts, so skipping a stage file with current live data preserves
    `updateUniqueShares` and insights inputs.
- Ran six external reviewers against the full Goal 1 implementation scope after
  the second slice. Four returned final `PRODUCTION GRADE` verdicts. One
  reviewer session ended without a final verdict after read-only checks and no
  concrete blocker. One reviewer session ended without a captured final verdict
  after read-only checks and no concrete blocker.
- Accepted low-risk external review follow-ups and implemented the cheap test
  guards immediately:
  - added `SkipComparisonIfNoUpdates=false` plan coverage proving a no-update
    run still does not publish without an independent repair reason;
  - added a symlink-stage comparison artifact test proving a symlink is not
    treated as already-current and is replaced by a regular staged file.
- Remaining external review follow-ups are non-blocking:
  - optional scheduler-level integration coverage for an all-failed/zero-success
    scheduled batch;
  - optional `ProviderDefaults` display-reason combination coverage;
  - the old sanitizer-only metric `metadata.comparison_zero_rows_removed` is now
    silent because the broad sweep no longer exists. This is expected from the
    Goal 1 removal, but may deserve an operator note if anyone has dashboards or
    alerts tied to that metric.
- Implemented the third Goal 1 slice for entity artifact missing-sidecar repair:
  - direct one-off `buildFeedEntityDelta` behavior remains a wrapper around the
    same logic (`pkg/engine/entity_surgical_delta.go:28-32`);
  - one surgical refresh batch now owns one `entityArtifactFeedPresence` scanner
    (`pkg/engine/entity_surgical_refresh.go:23-25`,
    `pkg/engine/entity_surgical_refresh.go:79-87`);
  - `loadFeedDeltas` passes that batch-local scanner while loading each target
    feed delta (`pkg/engine/entity_surgical_refresh.go:97-108`);
  - the scanner caches aggregate feed names only after scanning all country and
    ASN sidecars without finding the target; if it finds a referenced feed, it
    returns immediately and preserves the full-rebuild fallback
    (`pkg/engine/entity_surgical_io.go:30-94`);
  - tests prove missing committed sidecars still trigger full rebuild when
    existing aggregates reference the feed, and repeated absent feeds reuse one
    completed sidecar corpus scan (`pkg/engine/entity_surgical_test.go:152-205`).
- Ran external implementation review after the third slice. Five reviewers
  returned final `PRODUCTION GRADE` verdicts with no blockers. The remaining
  reviewer found a CI/hygiene blocker before being stopped after extensive
  checks:
  - `make staticcheck` and `golangci-lint` failed on a newly introduced unused
    private wrapper `entityArtifactsContainFeed`;
  - the same checks also exposed a pre-existing unused private retention helper
    `loadLatestSet`.
- Accepted and fixed the reviewer blocker:
  - removed the newly introduced dead `entityArtifactsContainFeed` wrapper;
  - removed the pre-existing dead `loadLatestSet` helper, because the live
    replacement path is `openPreviousLatestSet` and leaving the helper in place
    kept the branch failing static analysis;
  - this cleanup is behavior-neutral: one-off entity delta behavior remains
    available through `buildFeedEntityDelta(name)`, and retention continues to
    use `openPreviousLatestSet`.
- Ran the final external-review iteration after the blocker fixes. All six
  reviewers returned final `PRODUCTION GRADE` verdicts with no blockers:
  - reviewers independently verified that scheduler batches pass non-empty
    `Selected` names, so the reprocess-intent change only suppresses
    zero-success heavy-phase admission and does not widen full-run fan-out for
    ordinary scheduled work;
  - reviewers independently traced the comparison artifact skip through staged
    publication and follow-on readers, confirming that skipped stage files are
    safe only after byte, mode, and logical-mtime equivalence is proven against
    the stage or live target;
  - reviewers independently verified that the entity aggregate feed-presence
    scan caches only completed scans, preserves full-rebuild fallback when any
    aggregate references a missing committed feed sidecar, and remains
    batch-local and sequential;
  - reviewers confirmed that the previous static-analysis blocker is resolved:
    `entityArtifactsContainFeed` and `loadLatestSet` have no remaining Go
    references, `make staticcheck` is clean, and scoped `golangci-lint` reports
    zero issues.
- Final non-blocking Goal 1 notes:
  - optional integration coverage can still be added later for an all-failed
    scheduled batch, but the engine-plan and cache invariant tests already prove
    the zero-success no-publish contract at the behavior boundary changed here;
  - optional `ProviderDefaults` display-reason coverage remains a test-hardening
    item, not a behavior blocker;
  - `metadata.comparison_zero_rows_removed` is intentionally silent after
    removing the broad sanitizer sweep. Add an operator note only if production
    dashboards or alerts depended on this internal metric.

## Validation

Acceptance criteria evidence:

- Working theories T1-T10 are classified in the Working Theory Ledger.
- Production incident timestamps, journal events, cgroup counters, and current
  systemd state are recorded in this SOW.
- Netdata Cloud resource-pressure findings are recorded without credentials or
  raw Cloud identifiers.
- Code paths are cited for watchdog, public HTTP routes, scheduler processing,
  entity refresh, comparison generation, and retention.
- The remediation plan names modules, expected impact, behavior guarantees, and
  validation methods.

Tests or equivalent validation:

- Diagnosis validation was cross-source correlation: systemd journal timestamps,
  namespaced app logs, cgroup counters, admin status metrics, Cloud metrics, and
  code path review.
- Baseline before code changes:
  `timeout 1800 go test -count=1 ./pkg/engine ./pkg/scheduler ./pkg/web` passed.
- Targeted post-change tests passed:
  - `timeout 1800 go test -count=1 ./pkg/scheduler -run 'TestQueuedProcessingReasonReprocess|TestQueuedProcessingReprocessScansBatchReasons|TestCombineReasons'`
  - `timeout 1800 go test -count=1 ./pkg/engine -run 'TestBuildPipelineRunPlan|TestWriteComparisonFilesDoesNotSanitizeUntouchedLiveArtifacts|TestWriteComparisonFilesRemovesStaleZeroOverlapRows|TestValidateComparisonPayloadRejectsZeroOverlapRows|TestMergeCompareRowsDropsAndDeletesZeroOverlapRows'`
  - `timeout 1800 go test -count=1 ./pkg/cache -run 'TestEntrySourceProcessingStatusTransitions|TestEntryApplyFinalizedSourceSetAndMetadata'`
  - `timeout 1800 go test -count=1 ./pkg/engine -run 'TestWriteComparisonFilesSkipsCurrentLiveComparisonArtifacts|TestWriteComparisonFilesStagesComparisonArtifactWithStaleMTime|TestWriteComparisonFilesStagesComparisonArtifactWithWrongMode|TestWriteComparisonFilesStagesCurrentComparisonArtifactWhenOwnerConfigured'`
  - `timeout 1800 go test -count=1 ./pkg/engine -run 'TestWriteComparisonFiles|TestMergeCompareRows|TestValidateComparisonPayload|TestBuildPipelineRunPlan'`
  - `timeout 1800 go test -count=1 ./pkg/engine -run 'TestBuildPipelineRunPlan|TestWriteComparisonFilesSkipsCurrentLiveComparisonArtifacts|TestWriteComparisonFilesStagesComparisonArtifactWithStaleMTime|TestWriteComparisonFilesStagesComparisonArtifactWithWrongMode|TestWriteComparisonFilesStagesCurrentComparisonArtifactWhenOwnerConfigured|TestWriteComparisonFilesReplacesSymlinkStageComparisonArtifact'`
  - `timeout 1800 go test -count=1 ./pkg/engine -run 'TestBuildFeedEntityDelta|TestRefreshEntityArtifactsForFeedUpdatesSurgicallyPatchesAggregates|TestEntityArtifactsContainFeed|TestFeedEntitySidecar|TestIndexFeedEntity'`
  - `timeout 1800 go test -count=1 ./pkg/engine -run 'TestBuildFeedEntityDelta|TestRefreshEntityArtifactsForFeedUpdatesSurgicallyPatchesAggregates|TestEntityArtifactsContainFeed|TestFeedEntitySidecar|TestIndexFeedEntity|TestBuildRetentionData|TestUpdateRetention|TestRetention'`
  - `timeout 1800 go test -count=1 ./pkg/engine`
  - `timeout 1800 go test -race -count=1 ./pkg/engine -run 'TestBuildFeedEntityDelta|TestWriteComparisonFilesReplacesSymlinkStageComparisonArtifact|TestWriteComparisonFilesSkipsCurrentLiveComparisonArtifacts|TestBuildPipelineRunPlan'`
  - `timeout 1800 go test -race -count=1 ./pkg/engine -run 'TestBuildFeedEntityDelta|TestWriteComparisonFilesReplacesSymlinkStageComparisonArtifact|TestWriteComparisonFilesSkipsCurrentLiveComparisonArtifacts|TestBuildPipelineRunPlan|TestBuildRetentionData|TestRetention'`
- Focused package gate passed:
  `timeout 1800 go test -count=1 ./pkg/cache ./pkg/engine ./pkg/scheduler ./pkg/web`.
- Architecture posture gate passed:
  `timeout 1800 go test ./tools/archposture`.
- Strict shuffled scheduler/engine/web gate passed:
  `timeout 1800 make test-strict`.
- Static analysis gates passed after removing dead private helpers:
  - `timeout 600 make staticcheck`
  - `timeout 600 golangci-lint run --timeout=10m ./pkg/scheduler/... ./pkg/engine/... ./pkg/cache/...`
- Final external-review iteration passed after the blocker fixes:
  - six read-only reviewers returned `PRODUCTION GRADE` with no blockers;
  - reviewers reran or cited focused gates including `go build ./...`,
    `go vet ./pkg/scheduler/... ./pkg/engine/... ./pkg/cache/...`,
    `go test -count=1 ./pkg/cache ./pkg/engine ./pkg/scheduler ./pkg/web`,
    `go test -race -count=1` for focused engine/scheduler paths,
    `go test ./tools/archposture`, `make staticcheck`, and scoped
    `golangci-lint`;
  - no additional implementation changes were required after the final review.

Real-use evidence:

- Real production-candidate evidence shows two post-install watchdog restarts
  at `2026-06-14 20:40:01 UTC` and `2026-06-15 04:59:58 UTC`.
- Real cgroup evidence shows memory-high events and no post-install OOM kill.
- Real Cloud evidence shows severe CPU, memory pressure, and I/O pressure during
  the incident windows.
- Real app logs show broad 402-feed processing batches followed by queued entity
  refresh immediately before both watchdog windows.

Reviewer findings:

- Read-only subagents agreed on the major findings:
  single-process resource coupling, public compare being static, compose/lookup
  being dynamic residual risks, entity refresh being active during the watchdog
  windows, comparison/entity/retention being measured major cost centers, and
  no evidence of post-install OOM.
- A focused OTEL extraction found that per-incident entity lock metrics were not
  available from Cloud for the June 14/15 windows. This is recorded as a
  limitation rather than inferred as fact.

Same-failure scan:

- The observed post-install same-failure class is watchdog timeout after broad
  processing and post-publish entity refresh under CPU/memory/I/O pressure.
- Downloader failure churn was checked against the incident timing and refuted
  as the primary cause for these two failures.

Sensitive data gate:

- Passed for diagnosis notes. The SOW records aggregate metrics, timestamps,
  host/service names already provided in the work context, and code paths. It
  does not record Cloud tokens, raw Cloud node identifiers, machine GUIDs,
  credentials, private endpoint values, or token-bearing URLs.

Artifact maintenance gate:

- AGENTS.md: no update needed for this Goal 1 implementation slice.
- Runtime project skills: no update needed yet; follow-up implementation may
  update operations/testing guidance.
- Specs: `.agents/sow/specs/pipeline.md` updated for the Goal 1 processing
  intent contract, the producer-side byte-identical artifact skip contract, and
  the batch-local entity aggregate feed-presence scan contract.
- End-user/operator docs: no update needed; public/operator behavior is
  unchanged.
- End-user/operator skills: no update needed.
- SOW lifecycle: remains in `.agents/sow/current/` because SOW105 continues
  beyond Goal 1. Goal 1 implementation review is complete; remaining valid
  Goal 1 follow-ups are negligible test/operator-note hardening items, not
  blockers.

Specs update:

- `.agents/sow/specs/pipeline.md` records that ordinary downloader-admitted
  processing work is not explicit operator/repair reprocess intent and should not
  force global heavy publication after a zero-success scheduled batch.
- `.agents/sow/specs/pipeline.md` records that feed-scoped public artifact
  producers may skip staging only when existing target bytes, generated-file
  permissions, and producer-assigned logical mtime already match the artifact
  that would be produced.
- `.agents/sow/specs/pipeline.md` records that surgical entity refresh may
  reuse a completed batch-local aggregate feed-presence scan for missing
  committed feed sidecars, while preserving full-rebuild fallback when existing
  aggregates reference the missing feed.

Project skills update:

- Not updated in this diagnosis SOW.

End-user/operator docs update:

- Not updated in this diagnosis SOW.

End-user/operator skills update:

- Not updated in this diagnosis SOW.

Lessons:

- Public serving and batch artifact production sharing one constrained process
  is a production-safety risk when the batch side can create multi-minute CPU,
  memory, and I/O pressure.
- A watchdog ping must not be interpreted as proof of public HTTP
  responsiveness unless it tests the serving path.
- Entity refresh was correctly moved out of the foreground batch in prior work,
  but it still remains in the same process/cgroup and can run during the
  watchdog failure window.
- More memory alone is not a root-cause fix; it does not address the
  single-process coupling or the measured CPU/I/O cost centers.

Follow-up mapping:

- The 2026-06-19 accepted order supersedes the original public/worker-first
  follow-up. The current order is:
  unnecessary-work elimination, required-work bounding/performance, comparison,
  entity refresh, retention, dynamic public API budgets, and watchdog semantics.
- Public/worker split and static public serving remain possible later hardening
  options only after backend resource control is fixed or separately justified.

### Goal 5A Diagnostic Accounting Implementation - 2026-06-19

Implemented locally:

- Added structured run progress and run diagnostic summary logs with runtime
  snapshots, phase metrics, feed summaries, active operations, operation counts,
  counters, and rates.
- Added phase-scoped metrics so phase summaries report only the operation
  counts/counters for that phase, not process-wide cumulative totals.
- Added feed processing summaries that identify each feed, input bytes, parsed
  entries, unique IPs, elapsed time, byte rate, entry rate, IP rate, and
  per-feed operation timings.
- Added active-operation progress with explicit unit, completed work, total
  work, completion percentage, elapsed time, and rate per second.
- Added retention reconciliation accounting for total/scanned cohort files,
  skipped/malformed entries, processed/rewritten/deleted cohorts, input IPs,
  kept IPs, removed IPs, completion percentage, and files/sec.
- Exposed active-operation progress in `/api/v1/admin/status`.
- Updated the admin "Being Processed Now" list to show active operation label,
  progress bar, completed/total work, completion percentage, and rate.

Behavior boundary:

- Feed outputs, retention semantics, comparison artifacts, entity artifacts,
  public API behavior, scheduler cadence, and worker limits are unchanged.
- This slice adds measurement and operator visibility only. Retention algorithm
  optimization remains separate Goal 5 work.

Validation:

- `timeout 1800 go test -count=1 ./pkg/engine -run 'TestRunDiagnosticSummaryIncludesOperationsCountersAndActiveWork|TestFeedProcessingSummaryLogsWorkSizeAndRates|TestReconcileRetentionCohortsLogsExactAccounting|TestStatusSnapshotIncludesActiveOperations|TestReconcileRetentionCohortUsesFileBackedSource|TestRetentionDiffUsesFileBackedPreviousLatest|TestRetentionIgnoresAtomicTempFilesInNewDir'`
  passed.
- `pnpm --dir ui test -- --run src/components/admin/current-run.test.tsx`
  passed; Vitest ran the frontend test suite with 15 test files and 42 tests
  passing.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui build` passed. Vite emitted the existing chunk-size warning
  for `feed-detail`, not a build failure.
- `timeout 1800 go test -count=1 ./pkg/cache ./pkg/engine ./pkg/scheduler ./pkg/web`
  passed.

Artifact maintenance:

- Specs updated:
  - `.agents/sow/specs/processing-engine.md` records active-operation progress
    as part of engine/admin status.
  - `.agents/sow/specs/admin-ui.md` records the "Being Processed Now" progress
    display contract.
  - `.agents/sow/specs/operating-principles.md` records that progress metrics
    must declare unit, work size, completion, elapsed time, and rate.
- End-user/operator docs: no update needed for public docs; this is an admin
  operator surface change.
- Runtime project skills: no update needed yet.
- SOW lifecycle: SOW remains current/open; Goal 5A is a diagnostic slice that
  feeds later retention optimization decisions.

## Outcome

Diagnosis evidence and remediation plan drafted. Goal 1 scheduler,
comparison, and entity missing-sidecar repair-path slices are implemented
locally with validation passing. External review found and the implementation
fixed one static-analysis blocker. The final external-review iteration returned
six `PRODUCTION GRADE` verdicts with no blockers. Goal 1 is complete for this
SOW unless new production evidence contradicts the current findings. Goal 5A
diagnostic accounting is implemented locally and validated; the next retention
work should use the new measurements before changing retention algorithms.

## Lessons Extracted

Goal 1 reinforced the project rule that "unnecessary work" must be proven at
the behavior boundary before being removed. The useful pattern is: preserve the
same public artifact truth, prove zero-success or byte-identical conditions with
tests, and keep repair/operator intent as an explicit separate path.

## Followup

Continue this SOW with Goal 2: required-work bounding and performance. Deferred
Goal 1 notes are test/operator-note hardening only and do not block proceeding.

## Regression Log

None yet.
