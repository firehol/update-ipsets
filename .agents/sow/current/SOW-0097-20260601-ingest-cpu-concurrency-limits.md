# SOW-0097 - Production Resource Boundaries

## Status

Status: open

Sub-state: external reviewer rerun completed on 2026-06-13 with all requested
models voting PRODUCTION GRADE; awaiting user install and live validation.
Scheduled-processing semantic changes are not approved without stronger
evidence.

## Requirements

### Purpose

Make update-ipsets fit for production operation by bounding ingest-side CPU,
memory, I/O, and disk-growth behavior while preserving cheap public/admin
serving from already-published artifacts.

The immediate production blocker is a live managed install where CPU is
effectively saturated, the daemon is repeatedly OOM-killed at the systemd memory
limit, and stale generated staging directories are accumulating under the
runtime tree.

### User Request

Initial request: the application uses uncontrolled parallel threads leading to
multi-core consumption. Add a way to control the number of concurrent threads
and therefore the maximum CPU cores consumed.

Clarification from user:

- The application has two purposes: ingest and serve.
- The requested cap is for ingest.
- The cap must not apply to both features as a whole.

Additional production-blocker request from the user on 2026-06-13:

- Extreme CPU utilization: two cores are almost always fully utilized.
- Increasing disk usage: the install grew from about 24 GB to about 48 GB in
  10 days.
- If these two problems are not solved, the application is a no-go for
  production.

### Assistant Understanding

Facts:

- The project already has separate runtime knobs for ingest-side downloader,
  processing, heavy-phase, DNS, and background concurrency.
- The project does not currently expose one operator-facing ingest worker
  ceiling across all ingest domains.
- The live install is running an older June 3 binary while current `main`
  contains later quality/refactor work, but the core scheduled-processing and
  staging-batch contracts involved in this investigation are still present on
  current `main`.
- The Go runtime's `GOMAXPROCS` controls the number of CPUs that can execute Go
  code at the same time for the whole process; it would affect serving as well
  as ingest.
- `GOMAXPROCS` does not guarantee the process will only ever own that many
  operating-system threads because blocked syscalls and runtime work can involve
  additional threads.
- A hard CPU quota is an OS/container/systemd concern. An in-process
  `GOMAXPROCS` cap is the wrong primary control for this requirement because it
  is process-wide, not ingest-only.
- The live disk-growth evidence is not explained by ordinary current feed files
  alone. Hidden publish staging directories exist under both the public web tree
  and the private entity-artifact tree after daemon restarts.
- The live service has been repeatedly killed by the systemd memory limit, so
  best-effort deferred cleanup inside the process cannot be relied on as the
  only stale-stage cleanup mechanism.

Inferences:

- The practical fix for this version is an ingest-side concurrency ceiling that
  clamps downloader/composition, processing, heavy fan-out, DNS resolution, and
  background ingest repair/rebuild worker counts without limiting public
  request serving.
- Existing per-domain worker knobs remain valuable because they control memory,
  I/O, DNS pressure, and work admission inside the ingest subsystem.
- Working theory, not established fact: routine scheduled processing currently
  enters the engine with `Reprocess=true`, which forces publication and heavy
  phases for queued processing work and defeats the "skip heavy when no
  updates" optimization. The user challenged that this is unnecessary work, so
  this SOW will not change scheduled-processing semantics until the premise is
  proven with stronger evidence.
- The disk-growth fix needs both prevention and recovery: fewer aborted large
  staging batches, startup/reinstall cleanup of stale stage directories, and a
  safe operational cleanup path for existing installs.

Unknowns:

- Whether a future hard ingest-only CPU quota is worth the operational
  complexity of a separate process/service/cgroup topology.
- Whether the intended production default should publish every expensive
  derivative artifact after every scheduled changed feed, or only refresh
  heavyweight artifacts on a slower cadence / explicit repair path.

### Acceptance Criteria

- The accepted runtime policy is recorded in this SOW before implementation.
- Operators can configure the intended ingest concurrency/CPU parallelism limit
  through the chosen surface and see/verify the effective ingest values.
- Public/admin serving is not limited by the ingest cap except for normal
  resource contention caused by the operating system scheduler.
- The implementation does not change scheduled-processing semantics in this
  version; it must preserve enough runtime/status evidence to support a later
  proof-based decision about whether the scheduled path does unnecessary work.
- Stale web/entity staging directories from aborted runs are detected and safely
  removed during daemon startup and install/repair, without removing live
  published artifacts.
- The live install can be cleaned and reinstalled through a documented,
  non-destructive operational path.
- Existing concurrency-domain controls remain supported and documented.
- Config/spec/docs/tests are updated for any new runtime contract.
- Validation proves configured limits are parsed, applied, reloaded safely where
  applicable, and do not silently expand automatic heavy/background work beyond
  the chosen policy.

## Analysis

Sources checked:

- `pkg/config/config.go`
- `pkg/engine/runtime.go`
- `pkg/engine/run.go`
- `pkg/engine/run_pipeline.go`
- `pkg/engine/output.go`
- `pkg/engine/geoloc.go`
- `pkg/engine/asn.go`
- `pkg/engine/bogons.go`
- `pkg/engine/critical.go`
- `pkg/engine/entity_feed_sidecar_build.go`
- `pkg/engine/background_tasks.go`
- `pkg/scheduler/download_loop.go`
- `pkg/scheduler/processing_loop.go`
- `pkg/scheduler/queue_admission.go`
- `pkg/processor/primitives.go`
- `pkg/iprange/dns.go`
- `pkg/iprange/dns6.go`
- `configs/firehol/runtime.yaml`
- `pkg/engine/web_batch.go`
- `pkg/engine/entity_artifacts.go`
- `pkg/engine/entity_surgical_refresh.go`
- `pkg/engine/metadata_write.go`
- `pkg/engine/output_comparison.go`
- `.agents/sow/specs/config.md`
- `.agents/sow/specs/pipeline.md`
- `.agents/sow/specs/operating-principles.md`
- Official Go runtime docs: https://pkg.go.dev/runtime@go1.25.6#GOMAXPROCS
- Go blog, "Container-aware GOMAXPROCS": https://go.dev/blog/container-aware-gomaxprocs
- Local open-source mirror references listed below.

Current state:

- `Runtime` stores ingest-side `ParallelDownloads`, `ParallelDNSQueries`,
  `MaxProcessingWorkers`, `MaxHeavyPhaseWorkers`, and `MaxBackgroundWorkers`,
  but no ingest worker ceiling setting (`pkg/engine/runtime.go:40`,
  `pkg/engine/runtime.go:42`, `pkg/engine/runtime.go:53`).
- Config has YAML fields for those worker knobs but no ingest ceiling field
  (`pkg/config/config.go:136`, `pkg/config/config.go:138`,
  `pkg/config/config.go:149`).
- Defaults set `ParallelDownloads=5`, `ParallelDNSQueries=10`,
  `MaxProcessingWorkers=2`, `MaxHeavyPhaseWorkers=0` auto, and
  `MaxBackgroundWorkers=1` (`pkg/config/config.go:610`,
  `pkg/config/config.go:612`, `pkg/config/config.go:619`).
- Automatic heavy-phase workers use `runtime.NumCPU()`, capped at 8 but forced
  to at least `MaxProcessingWorkers` (`pkg/engine/runtime.go:246`).
- Processing runs use `MaxProcessingWorkers` for feed-local work
  (`pkg/engine/run.go:81`, `pkg/engine/run_pipeline.go:40`).
- Pairwise comparison, GeoIP, ASN, bogon, critical-infrastructure, and entity
  sidecar fan-out use `HeavyPhaseWorkers()` (`pkg/engine/output.go:411`,
  `pkg/engine/geoloc.go:129`, `pkg/engine/asn.go:222`,
  `pkg/engine/bogons.go:179`, `pkg/engine/critical.go:453`,
  `pkg/engine/entity_feed_sidecar_build.go:109`).
- Background tasks use a dedicated limiter and default to one worker
  (`pkg/engine/background_tasks.go:32`, `pkg/engine/background_tasks.go:171`,
  `pkg/engine/entity_feed_sidecar_build.go:42`).
- Downloader dispatch is bounded by `ParallelDownloads`
  (`pkg/scheduler/download_loop.go:10`, `pkg/scheduler/queue_admission.go:102`).
- Hostname/DNS resolution can create its own bounded workers, defaulting to
  `parallel_dns_queries=10` or per-step `threads`, capped at 100 in the
  processor step (`pkg/iprange/dns.go:33`, `pkg/iprange/dns6.go:10`,
  `pkg/processor/primitives.go:131`).
- The configuration spec already requires independent downloader,
  feed-processing, heavy-phase, and background worker domains
  (`.agents/sow/specs/config.md:539`).
- The pipeline spec requires heavy-phase concurrency to be independently
  configurable and automatic defaults to remain bounded, deterministic, and no
  lower than feed-processing worker count (`.agents/sow/specs/pipeline.md:320`).
- The operating principles require background work to remain resource-bounded
  and not expand to machine-wide parallelism (`.agents/sow/specs/operating-principles.md:277`).
- The scheduler currently calls `RunOnce` with `Reprocess=true` for every
  queued processing batch (`pkg/scheduler/processing_loop.go:37` through
  `pkg/scheduler/processing_loop.go:52`).
- The pipeline treats `Reprocess` and `Recheck` as explicit rebuild intent and
  disables heavy-skip behavior when either is true
  (`pkg/engine/run_pipeline.go:151` through
  `pkg/engine/run_pipeline.go:165`).
- Publishing creates hidden stage directories under the live output directory
  and removes them only at the end of `publish()` or by caller cleanup
  (`pkg/engine/web_batch.go:24` through `pkg/engine/web_batch.go:57`,
  `pkg/engine/web_batch.go:103` through `pkg/engine/web_batch.go:166`).

Live install evidence from a user-approved test host:

- The daemon process had about 8.25 CPU-hours after about 6.4 wall-clock hours,
  and `ps` sampled it at about 129% CPU.
- `vmstat 1 5` sampled user CPU between 97% and 100% for several seconds while
  the daemon was active.
- The admin status endpoint showed one run spending tens of seconds in metadata
  and entity refresh work; lifetime operations were dominated by pairwise
  comparison, entity writer-lock hold time, comparison-file writes, and ASN
  entity refreshes.
- The daemon had written about 140 GB and read about 52 GB since its latest
  start.
- `/opt/update-ipsets` used about 36 GB; the largest top-level subtrees were
  `web` at about 16 GB, `lib` at about 17 GB, and `data` at about 3.4 GB.
- The public web tree contained 51 hidden `.update-ipsets-web-*` staging
  directories.
- The private entity tree contained 43 hidden `.update-ipsets-entities-*`
  staging directories and about 677k files.
- The service had 24 restarts and repeated `oom-kill` results at the 2 GB
  `MemoryMax` limit between 2026-06-07 and 2026-06-13.
- Stale staging directory timestamps correlate with OOM/restart windows, so
  aborted runs are a concrete source of unbounded disk growth.

Risks:

- Calling `runtime.GOMAXPROCS` from config would cap serving and ingest together,
  which violates the clarified requirement.
- Setting a default ingest cap in the shipped catalog may unexpectedly increase
  queue latency on dedicated hosts and existing deployments.
- Relying only on current separate worker counts can still allow additive ingest
  concurrency when download, processing, heavy, DNS, and background work overlap.
- The ingest worker ceiling must not sit in public request paths or it can turn
  ingest protection into a public-serving regression.
- Cleaning stale staging directories must be prefix- and age-scoped. A broad
  recursive cleanup under `web` or `lib/entities` could destroy live artifacts.
- Reclassifying scheduled processing so it does not imply full `Reprocess` must
  preserve the reason the original forced path was added: once a body is
  admitted to processing, finalize, retention, and publication for that body must
  not be skipped.
- Entity artifacts are large by design, but the current shape can become too
  costly if every small feed change fans out to thousands of ASN/country payload
  writes.

## Pre-Implementation Gate

Status: approved for the internal ingest ceiling and stale-stage cleanup.

Problem / root-cause model:

- The product has bounded ingest worker domains, but no single configured
  ceiling across those domains. Heavy phases can use up to the automatic
  heavy-worker default while downloader/composition, DNS, processing, and
  background ingest tasks may also be active.
- Working theory only: scheduled processing currently treats every queued
  processing batch as `Reprocess=true`. The user challenged the premise that
  this is unnecessary work, so scheduler semantics are not changed in this
  version.
- The daemon creates publish/entity staging directories inside live runtime
  trees. Normal error returns and successful publishes usually clean them, but
  OOM kills and hard process exits leave them behind. The live install proves
  they accumulate across restarts.
- On Go 1.25+, default `GOMAXPROCS` may account for cgroup CPU quota, CPU
  affinity, and logical CPU count, but only when `GOMAXPROCS` is not explicitly
  set by environment or code.
- `GOMAXPROCS` is process-wide, so it is not the right primary mechanism for an
  ingest-only cap.

Evidence reviewed:

- Local code and spec evidence listed in Analysis.
- Live host evidence listed in Analysis, with private host identity omitted from
  this durable artifact.
- Official Go docs say `GOMAXPROCS` sets the maximum number of CPUs executing
  simultaneously and that a custom value disables automatic updates:
  https://pkg.go.dev/runtime@go1.25.6#GOMAXPROCS
- Official Go blog says `GOMAXPROCS` is a parallelism limit, while container CPU
  limits are throughput limits and a hard CPU limit belongs to the container/OS:
  https://go.dev/blog/container-aware-gomaxprocs
- User clarified that serving and ingest must not be capped as one process-wide
  feature.
- VictoriaMetrics and OpenTelemetry Collector Contrib both treat GOMAXPROCS as
  an explicit runtime/cgroup alignment concern.

Affected contracts and surfaces:

- Runtime configuration schema and defaults.
- Engine runtime resolution and reload behavior.
- Ingest worker defaulting and admission policy.
- Scheduler processing semantics.
- Engine run-plan semantics for scheduled updates versus explicit operator
  reprocess/recheck.
- Staged publication lifecycle and stale-stage cleanup.
- Admin/system info surface if effective ingest limits are exposed.
- Operator documentation and specs.
- Tests for config defaults, validation, runtime application, and reload.

Existing patterns to reuse:

- Runtime config fields in `config.RuntimeConfig` and `engine.Runtime`.
- `resolveRuntime` defaulting and `Reload` re-resolution.
- Existing background limiter `SetLimit` reload behavior.
- Existing staged publication prefix patterns:
  `.update-ipsets-web-*` and `.update-ipsets-entities-*`.
- Existing admin/system status resource counters for CPU, I/O, disk, and
  process memory.
- Runtime specs under `.agents/sow/specs/config.md`,
  `.agents/sow/specs/pipeline.md`, and
  `.agents/sow/specs/operating-principles.md`.
- Existing process runtime info under `pkg/web/sysinfo.go` and admin status
  support if exposing effective values.

Risk and blast radius:

- Ingest-side: affects downloader, processing, heavy fan-out, DNS, staged
  publishing, and background ingest/repair work.
- Operational: lower ingest parallelism can increase processing latency and queue
  drain time.
- Operational: stale-stage cleanup may free many GB on existing installs, but
  must run only after proving directories are not active for the current process.
- Compatibility: new config must default to current behavior unless the user
  chooses a default cap.
- Security: no direct sensitive-data exposure expected.
- Performance: capping ingest admission can reduce ingest-side peak CPU use but
  may increase wall time for feed updates, heavy artifact generation, and repair
  tasks.
- Public serving risk: the ingest limiter must be kept out of public/admin
  serving code paths.

Sensitive data handling plan:

- No raw secrets, credentials, tokens, community/customer identifiers, private
  endpoints, or non-private customer-identifying IPs are needed.
- Durable artifacts will cite code paths, line numbers, config field names, and
  public upstream documentation only.
- Live evidence must not record private hostnames, private addresses, operator
  usernames, or unrelated SSH noise.

Implementation plan:

1. Implement the selected scheduled-processing policy so scheduled batches do
   not force heavyweight rebuild behavior unless explicitly required.
2. Implement the selected ingest-only runtime policy in config, engine runtime
   resolution, daemon/startup or engine construction, and reload handling.
3. Add stale-stage cleanup for known publish/entity stage prefixes at safe
   lifecycle points.
4. Update automatic worker defaulting only as required by the selected policy.
5. Update install/repair flow for existing stale stages if approved.
6. Update specs and operator docs for exact semantics and limitations.
7. Add focused tests for defaults, explicit values, validation, application,
   reload behavior, scheduled-processing semantics, and stale-stage cleanup.

Validation plan:

- Unit tests for config decode/default/validation.
- Engine runtime tests for worker defaulting under explicit ingest caps.
- Scheduler/engine tests proving ingest work admission respects the cap while
  public/admin request handlers do not acquire ingest permits.
- Scheduler/engine tests proving scheduled queued processing can finalize and
  publish admitted work without forcing full heavy reprocess semantics.
- Publication tests proving stale stage cleanup removes only inactive
  `.update-ipsets-web-*` and `.update-ipsets-entities-*` directories.
- Reload test if reload changes the ingest cap.
- Installed-service validation on the test host: after deployment and cleanup,
  disk usage stops growing from stale stages and CPU is not continuously
  saturated during ordinary scheduled operation.
- `make test` or narrower package tests first, then broader gates depending on
  implementation scope.

Artifact impact plan:

- AGENTS.md: likely no update unless a new durable project rule is learned.
- Runtime project skills: likely no update unless implementation reveals a new
  repeatable rule.
- Specs: update config, pipeline, and/or operating-principles contracts.
- End-user/operator docs: update runtime/daemon configuration documentation.
- End-user/operator skills: none expected.
- SOW lifecycle: this pending SOW moves to current only after the user decision;
  SOW-0016 and SOW-0102 remain paused while this production blocker is active.

Open-source reference evidence:

- VictoriaMetrics/VictoriaMetrics @ cbb34395267bb6d231988b06586d4123af4a522a
  - `lib/cgroup/cpu.go:16` exposes available CPUs through `runtime.GOMAXPROCS`.
  - `lib/cgroup/cpu.go:23` applies CPU quota to GOMAXPROCS when present.
  - `lib/cgroup/cpu.go:37` avoids overriding an explicit `GOMAXPROCS`
    environment variable.
- grafana/mimir @ b71a23f4975841c45b687d6624ea677a541e55d7
  - `cmd/mimir/main.go:171` clamps GOMAXPROCS during startup.
  - `cmd/mimir/main.go:240` avoids GOMAXPROCS values higher than `NumCPU`.
- open-telemetry/opentelemetry-collector-contrib @ 6698bc24dc8ee69f839f16eb9950b99b074f8191
  - `extension/cgroupruntimeextension/README.md:18` documents automatic
    GOMAXPROCS/GOMEMLIMIT alignment with Linux cgroups.
  - `extension/cgroupruntimeextension/factory.go:31` enables GOMAXPROCS
    auto-configuration by default for that extension.
  - `extension/cgroupruntimeextension/extension.go:45` applies and logs the
    effective GOMAXPROCS value at start.

Open decisions:

- Decisions 1, 3, and 4 are recorded below and unblock the approved
  implementation scope.
- Decision 2 records an explicit user challenge: scheduled-processing semantic
  changes are not part of this version.

## Implications And Decisions

### Decision 1 - Ingest CPU-Limit Contract

Context:

- Worker knobs already exist, but they do not create one operator-facing ingest
  worker ceiling across downloader, processing, heavy fan-out, DNS, and
  background work.
- `GOMAXPROCS` is process-wide and would also cap serving, so it does not match
  the clarified requirement.
- A hard CPU quota still requires systemd/cgroup/container configuration, but
  that also caps serving unless the ingest component runs in a separate process
  or cgroup.

Options:

A. Shared ingest worker ceiling, serving excluded.

- Add `runtime.max_ingest_workers` or equivalent.
- Ingest worker domains are clamped to the configured ceiling; public and admin
  request handlers are not clamped by this setting.
- Existing domain knobs remain upper bounds inside the ceiling.
- Pros: matches the clarified purpose; avoids process-wide serving cap; avoids
  nested semaphore deadlock risk at `1`.
- Cons: it is not a strict global token pool and not a hard CPU quota.
- Risk: missed ingest paths can still bypass the worker ceiling.

B. Per-domain ingest caps only.

- Keep using `max_processing_workers`, `max_heavy_phase_workers`,
  `max_background_workers`, `parallel_downloads`, and `parallel_dns_queries`.
- Pros: no runtime-wide scheduler side effects; no new schema.
- Cons: separate caps can add up when domains overlap; harder for operators to
  answer "how many ingest workers can run at once?"
- Risk: may still feel uncontrolled during overlap between download/composition,
  processing, heavy fan-out, DNS, and background repair.

C. Separate ingest and serve processes/cgroups.

- Split or run ingest under a separate process/service/cgroup from serving, then
  apply OS-level CPUQuota to ingest only.
- Pros: strongest hard isolation; serving can keep a separate CPU budget.
- Cons: large architectural/operational change; more deployment complexity; not
  a small fix for the current daemon.
- Risk: introduces coordination, artifact ownership, lifecycle, and deployment
  failure modes.

D. Hybrid: shared ingest worker ceiling now, optional future process split.

- Implement option A in this SOW and document that a hard ingest-only CPU quota
  requires a future split or an external deployment topology with ingest isolated
  from serving.
- Pros: solves the immediate daemon behavior without capping serving; keeps a
  credible path to hard isolation later.
- Cons: not a kernel-enforced CPU cap in the current single-process daemon.
- Risk: operators may expect hard CPU enforcement unless docs/admin wording is
  explicit.

Selection:

- User decision on 2026-06-13: implement an internal ingest limit in this
  version. Do not split ingest/serve processes for this version.
- Clarification recorded with the decision: Go can control runnable Go-code
  parallelism with `GOMAXPROCS`, but that control is process-wide, not
  ingest-only, and it is not a hard per-subsystem thread count. Therefore the
  accepted fix is an application-level ingest worker ceiling, while serving
  remains outside the ceiling.
- Implementation default for this production-blocker version: ship a
  conservative `max_ingest_workers: 1` default in the bundled runtime
  configuration so the normal install path actually reduces the observed
  two-core saturation. Operators can raise it after CPU, memory, and disk
  behavior are stable.

### Decision 2 - Scheduled Processing Semantics

Context:

- Live evidence shows routine scheduled work keeps the daemon almost
  continuously busy.
- The scheduler currently sends every queued processing batch to `RunOnce` with
  `Reprocess=true` (`pkg/scheduler/processing_loop.go:37` through
  `pkg/scheduler/processing_loop.go:52`).
- The engine treats `Reprocess=true` as explicit operator rebuild intent and
  therefore disables the no-update heavy-skip path
  (`pkg/engine/run_pipeline.go:151` through
  `pkg/engine/run_pipeline.go:165`).
- This was originally done to prevent admitted local bodies from skipping
  finalize, retention, and publication.

Options:

A. Split "admitted local body must process" from "operator requested full
reprocess".

- Add a narrower run option or internal pipeline state that forces selected
  source finalization/publication while preserving heavy-skip semantics unless
  inputs truly changed or the operator explicitly requested reprocess/recheck.
- Pros: directly targets the live CPU loop; preserves correctness reason behind
  the old forced path; smallest user-visible behavior change.
- Cons: needs careful tests around same-body, not-modified, staged download
  promotion, retention, and derivative publication.
- Risk: if the split is incomplete, a scheduled update could publish source
  artifacts but skip a required dependent heavy artifact.

B. Keep `Reprocess=true`, but add more skip checks inside heavy phases.

- Leave scheduler semantics unchanged and make each heavy phase decide whether
  it can cheaply no-op.
- Pros: less visible API/run-option churn.
- Cons: spreads policy across many phases; harder to prove; more likely to miss
  one expensive path.
- Risk: the scheduler would still label ordinary scheduled work as full
  reprocess, preserving the confusing root cause.

C. Increase processing interval / disable some artifacts by default.

- Treat this as configuration/cadence tuning.
- Pros: fastest operational relief.
- Cons: does not fix the design bug; large batches can still saturate CPU and
  memory when they happen.
- Risk: hides the problem until production load or a larger update wave.

Selection:

- User decision on 2026-06-13: not approved for this version.
- The user challenged the premise that current scheduled processing is doing
  unnecessary work. This SOW must not change scheduler/engine reprocess
  semantics until the claim is proven with stronger evidence.
- Approved scope for this SOW: keep existing scheduled-processing semantics,
  implement the ingest limit and stale-stage cleanup, and preserve/improve
  operational evidence for a later decision if needed.

### Decision 3 - Stale Stage Directory Recovery

Context:

- Publishing uses hidden staging directories under live trees:
  `.update-ipsets-web-*` and `.update-ipsets-entities-*`
  (`pkg/engine/web_batch.go:24` through `pkg/engine/web_batch.go:57`).
- Successful publish removes the stage directory at the end
  (`pkg/engine/web_batch.go:103` through `pkg/engine/web_batch.go:166`).
- Normal error returns usually run caller cleanup, but OOM kills do not run
  deferred cleanup.
- The live install has dozens of stale stage directories under both trees, with
  timestamps aligned to OOM/restart windows.

Options:

A. Startup cleanup of stale known-prefix stage directories, plus install repair.

- On daemon startup, remove only known stage prefixes older than the current
  process start / a conservative age threshold.
- Make `install.sh` repair existing installs by removing inactive stale stage
  directories with the same prefix and age guard.
- Pros: prevents old crash leftovers from accumulating; fixes existing host
  state through the normal install path.
- Cons: startup cleanup must be extremely scoped and observable.
- Risk: too-aggressive age logic could remove an active stage from another
  process if locking is broken.

B. Install-only cleanup.

- Only `install.sh` removes stale stage directories.
- Pros: lowest runtime risk.
- Cons: does not self-heal after future OOM/killed runs until next install.
- Risk: disk growth can return silently in production.

C. No automatic cleanup; document manual command only.

- Pros: zero code risk.
- Cons: not production-grade for a daemon that can be OOM-killed.
- Risk: repeats the exact 24 GB to 48 GB growth failure.

Selection:

- User decision on 2026-06-13: A.
- Implement startup cleanup of stale known-prefix stage directories plus
  install repair for existing stale stage directories.

### Decision 4 - Immediate Test-Host Mitigation

Context:

- The live host is still running an older binary from 2026-06-03.
- Current `main` includes later entity and metadata refactors, but not the core
  scheduled-processing/stage-cleanup fixes.
- The host currently has repeated OOM restarts and stale stage trees consuming
  space.

Options:

A. Pause/reduce damage now while implementing: stop service, clean stale stages,
install current main plus approved fixes, then restart.

- Pros: frees disk and stops continued churn while validating the fix.
- Cons: test site downtime during cleanup/install.
- Risk: if cleanup is done before a clean service stop, an active stage could be
  removed.

B. Keep service running until the code fix is ready.

- Pros: no immediate downtime.
- Cons: CPU remains saturated and more stale stages can accumulate after another
  OOM kill.
- Risk: disk can continue growing and the host can enter another OOM/restart
  cycle.

C. Temporary systemd throttles only.

- Add a temporary `CPUQuota` or lower memory/concurrency settings while keeping
  code unchanged.
- Pros: quick pressure reduction.
- Cons: CPUQuota is process-wide and also affects serving; memory pressure may
  still cause OOM if work remains too large.
- Risk: masks the code issue and can degrade public/admin responsiveness.

Selection:

- User decision on 2026-06-13: prepare the new version in the repository.
- The user will run `./install.sh`. This implementation must make the normal
  install path repair existing stale generated stage directories without
  requiring the assistant to mutate the test host directly.

## Plan

1. Record user decision and move this SOW to `.agents/sow/current/`.
2. Implement the accepted internal ingest limit with tests.
3. Implement stale publish-stage cleanup during daemon startup and install
   repair with tests.
4. Update specs and operator docs for semantics, reload behavior, and risks.
5. Validate with targeted tests and broader project gates appropriate to the
   touched files.

## Execution Log

### 2026-06-01

- Created pending SOW from current code/spec inspection and official Go/runtime
  research.
- No implementation files changed.

### 2026-06-13

- Recorded user decisions:
  - implement an internal ingest worker ceiling;
  - do not split ingest and serving for this version;
  - do not change scheduled-processing semantics until the unnecessary-work
    premise is proven;
  - implement stale stage cleanup option A;
  - prepare the repository version for the user to install with `./install.sh`.
- Moved the SOW from pending to current before implementation.
- Added `runtime.max_ingest_workers` as an ingest-side worker ceiling.
- Set the shipped runtime catalog ceiling to `max_ingest_workers: 1`.
- Clamped effective download, DNS parsing, source-processing, heavy-phase, and
  background worker counts to the ingest ceiling when it is greater than zero.
- Exposed the configured ceiling and effective worker counts in engine/admin
  status.
- Added startup cleanup for stale `.update-ipsets-web-*` and
  `.update-ipsets-entities-*` publish stage directories.
- Added `install.sh` repair for old generated publish stage directories using
  the same known prefixes.
- Updated runtime configuration docs and the config/operating-principles specs.
- Updated UI admin-status TypeScript types and test fixtures for the new status
  fields.
- Applied external-review fixes:
  - limited the conservative `max_ingest_workers: 1` default to the bundled
    `configs/firehol/runtime.yaml` catalog instead of Go `DefaultRuntime()`;
  - added a 5-minute daemon-startup age buffer for stale publish-stage cleanup;
  - made normal `install.sh` repair stop the active service before mutating
    generated stage directories, while `--no-restart` skips repair when the
    service is still active;
  - added bundled-config, reload, and startup-cleanup wiring tests.

## Validation

Acceptance criteria evidence:

- Runtime policy recorded in this SOW before implementation.
- `configs/firehol/runtime.yaml` ships `max_ingest_workers: 1`.
- `pkg/config/config.go` leaves the generic Go default at `0`, so custom
  configs that omit `max_ingest_workers` keep prior per-domain worker defaults.
- `pkg/engine/runtime.go` applies the ceiling to effective ingest worker pools.
- `pkg/engine/query.go` and `pkg/engine/engine.go` expose effective worker
  values through status.
- `pkg/engine/stale_publish_stages.go` removes only known-prefix stale stage
  directories older than the daemon startup age buffer under configured publish
  roots.
- `pkg/web/server_run.go` runs stale-stage cleanup after runtime directory
  overrides are applied.
- `install.sh` repairs old stale generated publish stage directories during the
  normal install path with the service stopped when restart is requested.

Tests or equivalent validation:

- `go test ./pkg/config ./pkg/engine ./pkg/web` passed.
- `make test` passed.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui test` passed.
- `make lint` passed.
- `make shellcheck` passed.
- `make test-strict` passed.
- `make build` passed.
- After a robustness cleanup adjustment, `go test ./pkg/engine ./pkg/web` and
  `make lint` passed again.
- After external-review fixes:
  - `go test ./pkg/config ./pkg/engine ./pkg/web` passed.
  - Focused tests for bundled config default, reload ceiling, engine cleanup,
    and `prepareEngineForRun` startup cleanup passed.
  - `git diff --check` passed.
  - `make shellcheck` passed.
  - `make lint` passed.
  - `make test` passed.
  - `make test-strict` passed.
  - `pnpm --dir ui lint` passed.
  - `pnpm --dir ui test` passed.
  - `make build` passed.
  - `make race` passed.

Real-use evidence:

- Pending user-run `./install.sh` and live observation on the test install.
- Live success criteria remain:
  - disk usage stops growing from stale generated stage directories;
  - old stale stage directories are removed through install/startup repair;
  - CPU is no longer continuously saturating both cores during ordinary daemon
    operation;
  - admin status reports the effective ingest worker values.

Reviewer findings:

- External reviewer run on 2026-06-13:
  - `glm`: PRODUCTION GRADE; noted the ceiling is per ingest domain, not a
    hard global CPU quota, and startup cleanup depends on the daemon lifecycle
    lock.
  - `minimax`: PRODUCTION GRADE; noted the missing reload-specific ceiling test
    and the startup/install cleanup age-policy asymmetry.
  - `qwen`: PRODUCTION GRADE; noted install-time cleanup runs before restart but
    judged the 120-minute guard acceptable.
  - `mimo`: PRODUCTION GRADE with commit-hygiene reminder; the new cleanup Go
    file and moved SOW must be explicitly staged.
  - `deepseek`: NOT PRODUCTION GRADE until startup cleanup has a real age guard
    and reload/startup cleanup coverage is improved.
  - `kimi`: NOT PRODUCTION GRADE until the conservative default is limited to
    the bundled runtime config, startup cleanup has an age guard, and focused
    reload/startup tests are added.
- Accepted production-grade adjustments from the review:
  - Keep `max_ingest_workers: 1` in `configs/firehol/runtime.yaml`, but do not
    make it the process-wide Go `DefaultRuntime()` value for every custom config
    load path.
  - Add a conservative startup age buffer to stale stage cleanup so daemon
    startup does not remove newly-created matching stage directories.
  - Add focused tests for bundled config defaulting, reload re-clamping, and
    startup cleanup wiring.
- External reviewer rerun after those fixes on 2026-06-13:
  - `glm`: PRODUCTION GRADE; repeated that the ceiling is per ingest domain and
    not a strict shared semaphore or hard CPU quota.
  - `minimax`: PRODUCTION GRADE; listed non-blocking operator-doc/test follow-up
    ideas for custom-config defaults, cleanup age asymmetry, and override/SIGHUP
    guard coverage.
  - `kimi`: PRODUCTION GRADE; listed non-blocking polish items including a test
    rename, auto-heavy-worker ceiling coverage, install cleanup reporting, and
    release-note visibility for the new bundled default.
  - `qwen`: PRODUCTION GRADE; noted that install repair uses default install
    paths while daemon startup cleanup uses configured paths, making custom-path
    install repair a low-risk follow-up.
  - `deepseek`: PRODUCTION GRADE; noted test-only fixture and reload-disable
    coverage improvements, with no runtime production blocker.
  - `mimo`: PRODUCTION GRADE; noted that `max_ingest_workers` is a per-domain
    ceiling rather than a shared semaphore and that the SOW must remain open
    until live validation succeeds.
- Final reviewer-rerun residuals are non-blocking for this production-blocker
  SOW. The blocking review items from the first run were already addressed.

Same-failure scan:

- Initial search covered goroutine/fan-out points in `cmd`, `internal`, `pkg`,
  and `tools`.

Sensitive data gate:

- No sensitive data used or written. Evidence is limited to code paths, line
  numbers, public documentation, and open-source repository references.

Artifact maintenance gate:

- AGENTS.md: no update needed; no new project-wide workflow rule was learned.
- Runtime project skills: no update needed; existing operations/coding/testing
  rules covered the work.
- Specs: updated `.agents/sow/specs/config.md` and
  `.agents/sow/specs/operating-principles.md`.
- End-user/operator docs: updated `docs/configuration/runtime-settings.md`.
- End-user/operator skills: no update expected.
- SOW lifecycle: moved from `.agents/sow/pending/` to `.agents/sow/current/`;
  status remains `open` pending user install/live validation.

Specs update:

- Completed for the implemented runtime ceiling and stale-stage startup cleanup.

Project skills update:

- Not needed.

End-user/operator docs update:

- Completed for runtime settings.

End-user/operator skills update:

- None expected.

Lessons:

- Go's runtime-level CPU parallelism control is process-wide. For this product's
  single-process daemon, an ingest-specific control belongs at application
  worker-pool boundaries unless/until ingest and serving are split.
- Stale publish-stage cleanup should be lifecycle-scoped, prefix-scoped, and
  tied to daemon startup/install repair; it should not live in generic directory
  setup used by reload paths.

Follow-up mapping:

- Scheduled-processing semantic changes are explicitly not implemented in this
  SOW. If live evidence still shows excessive CPU after the ingest ceiling and
  stale-stage cleanup, the next step is a proof-focused investigation of whether
  ordinary scheduled processing is doing unnecessary heavy work.

## Outcome

Implementation prepared; pending user install and live validation.

## Lessons Extracted

- Application-level worker ceilings are safer for this release than a nested
  global semaphore because current pipeline phases contain nested bounded work
  and a naive permit pool can deadlock at an ingest limit of `1`.

## Followup

- No pending implementation SOW has been created for scheduled-processing
  semantics because the premise was challenged and remains unproven.

## Regression Log

None yet.
