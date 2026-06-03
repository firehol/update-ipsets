# SOW-0097 - Ingest CPU And Concurrency Limits

## Status

Status: open

Sub-state: revised requirement recorded; needs ingest-only design decision before implementation

## Requirements

### Purpose

Allow operators to predictably limit update-ipsets ingest-side CPU parallelism
and concurrent background ingest work so feed acquisition, processing, and heavy
artifact generation do not consume more CPU cores than intended, while public and
admin serving remain separately available and are not capped as part of the
ingest limit.

### User Request

Initial request: the application uses uncontrolled parallel threads leading to
multi-core consumption. Add a way to control the number of concurrent threads
and therefore the maximum CPU cores consumed.

Clarification from user:

- The application has two purposes: ingest and serve.
- The requested cap is for ingest.
- The cap must not apply to both features as a whole.

### Assistant Understanding

Facts:

- The project already has separate runtime knobs for ingest-side downloader,
  processing, heavy-phase, DNS, and background concurrency.
- The project does not currently expose one shared ingest CPU budget across all
  ingest domains.
- The Go runtime's `GOMAXPROCS` controls the number of CPUs that can execute Go
  code at the same time for the whole process; it would affect serving as well
  as ingest.
- `GOMAXPROCS` does not guarantee the process will only ever own that many
  operating-system threads because blocked syscalls and runtime work can involve
  additional threads.
- A hard CPU quota is an OS/container/systemd concern. An in-process
  `GOMAXPROCS` cap is the wrong primary control for this requirement because it
  is process-wide, not ingest-only.

Inferences:

- The practical fix is likely an ingest-side concurrency budget that limits work
  admission across downloader/composition, processing, heavy fan-out, DNS
  resolution, and background ingest repair/rebuild paths without limiting public
  request serving.
- Existing per-domain worker knobs remain valuable because they control memory,
  I/O, DNS pressure, and work admission inside the ingest subsystem.

Unknowns:

- Whether the ingest-only cap should be one shared global ingest budget, a set of
  per-domain caps with a common default ceiling, or a hybrid of both.
- Whether the default catalog should set a conservative ingest cap by default or
  only document and support an explicit override.

### Acceptance Criteria

- The accepted runtime policy is recorded in this SOW before implementation.
- Operators can configure the intended ingest concurrency/CPU parallelism limit
  through the chosen surface and see/verify the effective ingest values.
- Public/admin serving is not limited by the ingest cap except for normal
  resource contention caused by the operating system scheduler.
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
- `pkg/scheduler/queue_admission.go`
- `pkg/processor/primitives.go`
- `pkg/iprange/dns.go`
- `pkg/iprange/dns6.go`
- `configs/firehol/runtime.yaml`
- `.agents/sow/specs/config.md`
- `.agents/sow/specs/pipeline.md`
- `.agents/sow/specs/operating-principles.md`
- Official Go runtime docs: https://pkg.go.dev/runtime@go1.25.6#GOMAXPROCS
- Go blog, "Container-aware GOMAXPROCS": https://go.dev/blog/container-aware-gomaxprocs
- Local open-source mirror references listed below.

Current state:

- `Runtime` stores ingest-side `ParallelDownloads`, `ParallelDNSQueries`,
  `MaxProcessingWorkers`, `MaxHeavyPhaseWorkers`, and `MaxBackgroundWorkers`,
  but no shared ingest CPU/concurrency setting (`pkg/engine/runtime.go:40`,
  `pkg/engine/runtime.go:42`, `pkg/engine/runtime.go:53`).
- Config has YAML fields for those worker knobs but no shared ingest cap field
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

Risks:

- Calling `runtime.GOMAXPROCS` from config would cap serving and ingest together,
  which violates the clarified requirement.
- Setting a default ingest cap in the shipped catalog may unexpectedly increase
  queue latency on dedicated hosts and existing deployments.
- Relying only on current separate worker counts can still allow additive ingest
  concurrency when download, processing, heavy, DNS, and background work overlap.
- A shared ingest limiter must not sit in public request paths or it can turn an
  ingest protection into a public-serving regression.

## Pre-Implementation Gate

Status: needs-user-decision

Problem / root-cause model:

- The product has bounded ingest worker domains, but no shared ingest-side
  budget across those domains. Heavy phases can use up to the automatic
  heavy-worker default while downloader/composition, DNS, processing, and
  background ingest tasks may also be active.
- On Go 1.25+, default `GOMAXPROCS` may account for cgroup CPU quota, CPU
  affinity, and logical CPU count, but only when `GOMAXPROCS` is not explicitly
  set by environment or code.
- `GOMAXPROCS` is process-wide, so it is not the right primary mechanism for an
  ingest-only cap.

Evidence reviewed:

- Local code and spec evidence listed in Analysis.
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
- Admin/system info surface if effective ingest limits are exposed.
- Operator documentation and specs.
- Tests for config defaults, validation, runtime application, and reload.

Existing patterns to reuse:

- Runtime config fields in `config.RuntimeConfig` and `engine.Runtime`.
- `resolveRuntime` defaulting and `Reload` re-resolution.
- Existing background limiter `SetLimit` reload behavior.
- Runtime specs under `.agents/sow/specs/config.md`,
  `.agents/sow/specs/pipeline.md`, and
  `.agents/sow/specs/operating-principles.md`.
- Existing process runtime info under `pkg/web/sysinfo.go` and admin status
  support if exposing effective values.

Risk and blast radius:

- Ingest-side: affects downloader, processing, heavy fan-out, DNS, and
  background ingest/repair work.
- Operational: lower ingest parallelism can increase processing latency and queue
  drain time.
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

Implementation plan:

1. Implement the selected ingest-only runtime policy in config, engine runtime
   resolution, daemon/startup or engine construction, and reload handling.
2. Update automatic worker defaulting only as required by the selected policy.
3. Update specs and operator docs for exact semantics and limitations.
4. Add focused tests for defaults, explicit values, validation, application, and
   reload behavior.

Validation plan:

- Unit tests for config decode/default/validation.
- Engine runtime tests for worker defaulting under explicit ingest caps.
- Scheduler/engine tests proving ingest work admission respects the cap while
  public/admin request handlers do not acquire ingest permits.
- Reload test if reload changes the ingest cap.
- `make test` or narrower package tests first, then broader gates depending on
  implementation scope.

Artifact impact plan:

- AGENTS.md: likely no update unless a new durable project rule is learned.
- Runtime project skills: likely no update unless implementation reveals a new
  repeatable rule.
- Specs: update config, pipeline, and/or operating-principles contracts.
- End-user/operator docs: update runtime/daemon configuration documentation.
- End-user/operator skills: none expected.
- SOW lifecycle: this pending SOW moves to current only after the user decision.

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

- Decision 1 below blocks implementation.

## Implications And Decisions

### Decision 1 - Ingest CPU-Limit Contract

Context:

- Worker knobs already exist, but they do not create one shared ingest budget
  across downloader, processing, heavy fan-out, DNS, and background work.
- `GOMAXPROCS` is process-wide and would also cap serving, so it does not match
  the clarified requirement.
- A hard CPU quota still requires systemd/cgroup/container configuration, but
  that also caps serving unless the ingest component runs in a separate process
  or cgroup.

Options:

A. Shared ingest budget, serving excluded.

- Add `runtime.max_ingest_workers` or equivalent.
- Ingest work that consumes CPU or fans out acquires an ingest permit; public and
  admin request handlers do not.
- Existing domain knobs remain upper bounds inside the shared budget.
- Pros: matches the clarified purpose; prevents additive ingest concurrency;
  avoids process-wide serving cap.
- Cons: more careful integration work; must avoid deadlocks/nested permit
  acquisition; not a hard CPU quota.
- Risk: missed ingest paths can still bypass the shared budget.

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

D. Hybrid: shared ingest budget now, optional future process split.

- Implement option A in this SOW and document that a hard ingest-only CPU quota
  requires a future split or an external deployment topology with ingest isolated
  from serving.
- Pros: solves the immediate daemon behavior without capping serving; keeps a
  credible path to hard isolation later.
- Cons: not a kernel-enforced CPU cap in the current single-process daemon.
- Risk: operators may expect hard CPU enforcement unless docs/admin wording is
  explicit.

Recommendation:

- Choose D. Implement a shared ingest budget in this SOW, keep serving outside
  that limiter, and explicitly document that hard ingest-only CPU enforcement
  requires process/cgroup isolation. This matches the user's clarified purpose
  without using the wrong process-wide `GOMAXPROCS` mechanism.

Selection:

- Pending user decision.

## Plan

1. Record user decision and move this SOW to `.agents/sow/current/`.
2. Implement the chosen runtime CPU policy with tests.
3. Update specs and operator docs for semantics, reload behavior, and risks.
4. Validate with targeted tests and broader project gates appropriate to the
   touched files.

## Execution Log

### 2026-06-01

- Created pending SOW from current code/spec inspection and official Go/runtime
  research.
- No implementation files changed.

## Validation

Acceptance criteria evidence:

- Pending.

Tests or equivalent validation:

- Pending.

Real-use evidence:

- Pending.

Reviewer findings:

- Pending.

Same-failure scan:

- Initial search covered goroutine/fan-out points in `cmd`, `internal`, `pkg`,
  and `tools`.

Sensitive data gate:

- No sensitive data used or written. Evidence is limited to code paths, line
  numbers, public documentation, and open-source repository references.

Artifact maintenance gate:

- AGENTS.md: no update yet; decision pending.
- Runtime project skills: no update yet; decision pending.
- Specs: no update yet; decision pending.
- End-user/operator docs: no update yet; decision pending.
- End-user/operator skills: no update expected.
- SOW lifecycle: pending SOW created; status remains `open`.

Specs update:

- Pending decision.

Project skills update:

- Pending decision.

End-user/operator docs update:

- Pending decision.

End-user/operator skills update:

- None expected.

Lessons:

- Pending.

Follow-up mapping:

- Pending.

## Outcome

Pending.

## Lessons Extracted

Pending.

## Followup

None yet.

## Regression Log

None yet.
