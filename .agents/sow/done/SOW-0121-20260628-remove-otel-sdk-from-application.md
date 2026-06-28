# SOW-0121 - Remove OpenTelemetry SDK From Application Hot Paths

## Status

Status: completed

Sub-state: implementation, validation, and final external review completed;
ready for SOW move and commit.

## Requirements

### Purpose

Make telemetry subordinate to service availability and correctness. The
application must own exact local instrumentation state, while remote telemetry
exporters are isolated adapters that cannot block ingesting, serving,
watchdog, admin, or local diagnostics.

### User Request

Create an SOW to completely remove OpenTelemetry SDK use from the application.
OpenTelemetry is allowed only as an exporter, in one isolated module that is
never called by ingestion or serving hot paths.

The application must have its own instrumentation framework:

- metrics use finite compile-time-decided cardinality
- metric hot paths are lockless atomic updates
- metrics, logs, and traces follow a zero-allocation principle after startup
- log/trace buffering is bounded
- buffering should pre-allocate all memory needed for maximum queueing, or use
  a dedicated allocator such as two large buffers that are swapped/flushed as a
  whole without per-record frees

### Assistant Understanding

Facts:

- OpenTelemetry currently exists as a direct dependency set in `go.mod`.
- OpenTelemetry imports are concentrated in `internal/observability`, but
  application packages also import OTel types and instrumentation directly.
- Current request-path and daemon-path work was already hardened to avoid
  blocking, but that is not the final accepted design because metric samples can
  be dropped and OTel SDK remains in process hot-path-adjacent code.
- User-approved direction is complete architectural separation: local
  instrumentation is the source of truth; OpenTelemetry is only an optional
  export format/transport.

Inferences:

- Existing OTel metric attribute types must disappear from application-facing
  APIs because even OTel API types in callers couple hot paths to OTel's data
  model and allocation behavior.
- Dynamic metric label values such as feed names, provider names, raw routes,
  client IPs, error strings, URLs, and user agents are incompatible with
  compile-time-decided cardinality. Per-feed/operator detail can remain in
  admin status, logs, traces, and local diagnostic snapshots, but not as metric
  label cardinality unless the identity is represented by a compile-time enum.
- `/metrics` should be served from the local metric registry, not from an OTel
  SDK reader.

Unknowns at SOW creation, resolved during implementation below:

- Exact compile-time metric series list and histogram/timing bucket layout.
- Exact bounded log/trace record schema.

### Acceptance Criteria

- A source guard proves no production package imports `go.opentelemetry.io/*`
  except one explicitly named isolated exporter module and its tests.
- The downloader no longer wraps its HTTP transport with `otelhttp`; HTTP
  download metrics/traces come from local instrumentation.
- Application-facing instrumentation APIs do not expose OTel types, variadic
  OTel attributes, spans, metric instruments, or OTel contexts.
- Local metrics are exact:
  - counter increments are not dropped under exporter blockage
  - gauges read current local truth
  - timing aggregates use bounded predeclared series
  - exporter failure cannot change local metric values
- Metric hot-path updates are zero-allocation after initialization and use
  atomics or equivalent lockless per-series state.
- Metric cardinality is compile-time finite. Any unrecognized runtime value is
  mapped to a predeclared bucket such as `other` or rejected with an exact local
  telemetry fault counter.
- Logs and traces use bounded, non-blocking local queues or arenas:
  - healthy downstream ingestion receives all records within configured bounds
  - full buffers never block application work
  - overflow/drop counters are exact local metrics
  - record capture performs no heap allocation after initialization
- The OTel exporter, if kept, snapshots or drains local telemetry from its own
  goroutine and cannot apply backpressure to producers.
- Prometheus export is served from local telemetry snapshots and has bounded
  scrape behavior.
- Documentation, specs, and project skills no longer describe OTel SDK as the
  application instrumentation system.
- Validation includes unit/behavioral tests, source guards, allocation tests or
  benchmarks for hot paths, race tests, and broad project gates.

## Analysis

Sources checked:

- `go.mod`
- `internal/observability/observability.go`
- `internal/observability/metrics_async.go`
- `cmd/update-ipsets/daemon.go`
- `pkg/downloader/downloader.go`
- `internal/fileutil/fileutil.go`
- `.agents/sow/specs/operating-principles.md`
- `.agents/skills/project-coding/SKILL.md`
- `.agents/skills/project-operations/SKILL.md`
- OpenTelemetry official specifications:
  - `https://opentelemetry.io/docs/specs/otel/performance/`
  - `https://opentelemetry.io/docs/specs/otel/trace/sdk/`
  - `https://opentelemetry.io/docs/specs/otel/logs/sdk/`
  - `https://opentelemetry.io/docs/specs/otel/metrics/sdk/`
- Production journal stack summaries from the private runtime host, sanitized to
  function names and behavior only.
- Local mirrored open-source reference implementations:
  - `VictoriaMetrics/metrics @ 02a2d77a40117f24bcf44ba461cf1a87f478ff43`
  - `prometheus/client_golang @ 28914d017fba1a0de991374a2584ad82c8f93e5c`
  - `VictoriaMetrics/VictoriaMetrics @ 679646a3b38361f2ee06a9eca14188798ea4bd5e`

Initial state at SOW creation:

- `go.mod:12` through `go.mod:27` list OTel bridges, instrumentation,
  exporters, API, SDK, metrics, logs, and traces as direct dependencies.
- `internal/observability/observability.go:17` through
  `internal/observability/observability.go:36` import OTel bridge, API,
  exporter, SDK, semantic-convention, metric, log, and trace packages in the
  central observability package.
- `internal/observability/observability.go:261` through
  `internal/observability/observability.go:302` build OTel trace and metric
  SDK providers and install them as global providers.
- `internal/observability/observability.go:314` through
  `internal/observability/observability.go:323` builds an OTel log provider
  and tees the application logger to an OTel slog handler.
- `cmd/update-ipsets/daemon.go:17` imports OTel attributes directly, and
  `cmd/update-ipsets/daemon.go:148` records a daemon panic counter with an
  OTel attribute on the daemon control path.
- `pkg/downloader/downloader.go:15` imports `otelhttp`, and
  `pkg/downloader/downloader.go:84` wraps the HTTP transport with OTel
  instrumentation.
- `internal/fileutil/fileutil.go:15` imports OTel attributes, and
  `internal/fileutil/fileutil.go:93` through
  `internal/fileutil/fileutil.go:104` start/end spans around atomic writes.
- `internal/observability/metrics_async.go:42` through
  `internal/observability/metrics_async.go:84` currently drop metric samples
  under async queue pressure. This is an availability hotfix, not the accepted
  long-term metric correctness contract.
- `.agents/sow/specs/operating-principles.md` currently allows drop-before-delay
  request-path metric samples and best-effort local telemetry snapshots. This
  must be rewritten for exact local metrics and bounded logs/traces.
- `.agents/skills/project-operations/SKILL.md` currently says OpenTelemetry log
  export is best-effort. This wording is no longer accepted; logs/traces are
  bounded, loss-accounted, and expected to deliver under healthy ingestion.
- The production watchdog failure showed a liveness failure with near-zero CPU
  because goroutines were parked, not computing. The watchdog tick error path
  entered `internal/observability.Count`, then OTel SDK metric instrument
  creation/caching, and blocked on the SDK metric pipeline mutex while the SDK
  collection/export path held the same pipeline lock. This is the concrete
  failure mode this SOW removes from application hot paths.

Risks:

- Broad replacement can temporarily reduce operator visibility if local metrics,
  `/metrics`, admin status, logs, and traces are not migrated together.
- Removing OTel context propagation may affect any downstream trace correlation
  users relied on. This is acceptable only if replaced by local trace IDs and
  exporter-side adaptation or explicitly removed from the contract.
- Zero-allocation hot paths are easy to regress with variadic APIs, dynamic
  strings, maps, slices, formatted logging, or unbounded attribute/value
  capture. The implementation needs source guards and allocation tests.
- Compile-time metric cardinality means some previously exported label detail
  must move out of metrics. This is intentional, but operator docs must explain
  where the detail moved.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- OpenTelemetry SDK was treated as the application instrumentation substrate.
  That couples local telemetry capture to a generic SDK that owns processors,
  batching, exporters, queues, global providers, cardinality behavior,
  attribute allocation, and shutdown/flush behavior.
- OTel's general contract is not the product contract needed here. The product
  requires exact local metrics, bounded non-blocking logs/traces, and no
  availability impact from remote ingestion or exporter behavior.
- The root fix is not another wrapper around OTel. The root fix is a project
  local instrumentation framework, with OTel only as a detached export adapter.

Evidence reviewed:

- Dependency evidence: `go.mod:12` through `go.mod:27`.
- Central OTel SDK setup evidence:
  `internal/observability/observability.go:17` through
  `internal/observability/observability.go:36`,
  `internal/observability/observability.go:261` through
  `internal/observability/observability.go:323`.
- Application package OTel coupling evidence:
  `cmd/update-ipsets/daemon.go:17`,
  `cmd/update-ipsets/daemon.go:148`,
  `pkg/downloader/downloader.go:15`,
  `pkg/downloader/downloader.go:84`,
  `internal/fileutil/fileutil.go:15`,
  `internal/fileutil/fileutil.go:93`.
- Current local metric loss evidence:
  `internal/observability/metrics_async.go:42` through
  `internal/observability/metrics_async.go:84`.
- Current specs/skills requiring correction:
  `.agents/sow/specs/operating-principles.md`,
  `.agents/skills/project-coding/SKILL.md`,
  `.agents/skills/project-operations/SKILL.md`.
- Official OTel behavior evidence:
  OTel performance guidance discusses resource/usefulness tradeoffs for
  blocking and information loss; OTel trace/log SDKs define bounded queues that
  drop after queue limits; OTel metrics SDK defines aggregation and cardinality
  behavior in the SDK.

Affected contracts and surfaces:

- Go module dependencies.
- `internal/observability` and `internal/telemetry`.
- Daemon startup/shutdown telemetry setup.
- Downloader HTTP instrumentation.
- File write instrumentation.
- Engine, scheduler, processor, web, admin, and watchdog instrumentation calls.
- `/metrics` endpoint.
- Admin status telemetry sections.
- Operator docs for telemetry environment variables.
- Runtime project skills.
- Operating-principles spec.
- Tests and source guards.

Existing patterns to reuse:

- `pkg/iprange` local-operation-stat model: hot-path code returns plain local
  stats and does not import telemetry frameworks.
- SOW-0117 source guards in `pkg/web/telemetry_contract_test.go` and
  `internal/observability/metric_contract_test.go`, expanded into stronger
  import/API guards.
- Existing admin status snapshot structure for operator-visible details that
  should not become metric labels.
- Existing bounded `/metrics` handler wrapper in `pkg/web/metrics.go`, adapted
  to serve local registry snapshots instead of OTel/Prometheus SDK state.
- `VictoriaMetrics/metrics @ 02a2d77a40117f24bcf44ba461cf1a87f478ff43`
  `counter.go:30` through `counter.go:57`: hot-path counter mutation is direct
  `sync/atomic` add/load/store on pre-created metric objects.
- `prometheus/client_golang @ 28914d017fba1a0de991374a2584ad82c8f93e5c`
  `prometheus/counter.go:80` through `prometheus/counter.go:170`: hot-path
  integer counter increments use atomics and collection reads local state.
- `VictoriaMetrics/VictoriaMetrics @ 679646a3b38361f2ee06a9eca14188798ea4bd5e`
  `app/vmagent/main.go:765` through `app/vmagent/main.go:834`: high-volume
  request metrics are predeclared as finite series rather than discovered from
  runtime values.

Risk and blast radius:

- High. This touches telemetry APIs used across daemon, web, scheduler, engine,
  downloader, file utilities, docs, and tests.
- Performance risk is also high: a naive local framework could recreate the
  same allocation/locking problems. The implementation must prove allocation
  and blocking shape before broad call-site migration is considered complete.
- Observability regression risk is high: remote telemetry may temporarily lose
  fields if the local schema is not defined before migration.
- Operational risk is moderate: existing OTLP environment variables may change
  meaning or become no-ops unless the isolated exporter preserves them.

Sensitive data handling plan:

- No raw production logs, remote endpoints, credentials, bearer tokens, private
  IPs, customer data, or proprietary incident details are needed.
- SOW/spec/doc evidence will use file paths, line numbers, and sanitized
  behavior descriptions only.
- Tests will use synthetic telemetry names and values.

Implementation plan:

1. Inventory every OTel import and every instrumentation call site. Classify
   each as metric, log, trace, propagation, HTTP instrumentation, Prometheus
   export, or documentation-only.
2. Design the local telemetry framework before changing call sites:
   - compile-time metric descriptor table
   - fixed metric series IDs
   - atomic counter/gauge/timing storage
   - fixed log/trace record schemas
   - bounded queue or double-buffer arena strategy
   - exact telemetry fault counters
   - snapshot/export APIs
3. Add tests and source guards first:
   - OTel import boundary guard
   - no OTel types in application instrumentation APIs
   - exact metric update under blocked exporter
   - bounded log/trace queue non-blocking behavior
   - allocation checks for representative metric/log/trace hot paths
4. Replace application instrumentation calls with local APIs.
5. Rewrite `/metrics` to export from the local registry.
6. Move all OTel SDK/exporter code, if retained, into one isolated exporter
   module. The module consumes snapshots/drained buffers and cannot be called
   from ingestion or serving hot paths.
7. Remove unused OTel dependencies. Keep only dependencies required by the
   isolated exporter module; if no isolated exporter is ready, remove all OTel
   dependencies.
8. Update specs, docs, and project skills to describe the local instrumentation
   contract and the isolated exporter boundary.

Validation plan:

- `rg -n "go\\.opentelemetry\\.io|otelhttp|otelslog"` must show only the
  approved isolated exporter module and tests.
- `go test ./internal/telemetry ./internal/observability ./pkg/web ./pkg/engine ./pkg/scheduler ./pkg/downloader ./internal/fileutil`
- `make test`
- `make lint`
- `make race`
- `make staticcheck` or explicit SOW-0120 interaction if staticcheck is still
  blocked by unrelated engine unused-code findings.
- Allocation tests or benchmarks for:
  - counter increment
  - gauge set
  - timing observe
  - log enqueue
  - trace event enqueue
- Admin/API smoke:
  - `/healthz`
  - `/metrics`
  - `/api/v1/admin/status`
- Telemetry exporter blockage test:
  - blocked/stalled exporter does not block producers
  - local metric truth remains exact
  - logs/traces overflow only at bounded capacity and increments exact counters

Artifact impact plan:

- AGENTS.md: likely update if telemetry rules become project-wide guardrails.
- Runtime project skills: update `project-coding`, `project-operations`, and
  `project-testing`.
- Specs: update `operating-principles.md`, `admin-ui.md`, and any telemetry
  docs/spec sections affected by `/metrics` and admin status behavior.
- End-user/operator docs: update `docs/monitoring/*` and runtime environment
  variable docs.
- End-user/operator skills: likely unaffected unless telemetry instructions are
  exported outside repo skills.
- SOW lifecycle: this SOW is current and in progress. It must close only after
  implementation, validation, artifact updates, and review are complete.

Open-source reference evidence:

- Official OpenTelemetry specifications were checked for SDK behavior.
- Local reference implementations show the patterns this SOW will reuse:
  predeclared series, atomic hot-path mutation, and separate collection/export
  reads.

Open decisions:

- None. The exact metric descriptor table, allowed dimensions, log/trace record
  schema, and buffer config names are recorded below.

Local telemetry schema details now fixed for implementation:

- Metric descriptors and dimensions are the compile-time table in
  `internal/observability/metrics_schema.go`. Runtime values that do not match
  a declared dimension value map to the declared `other` bucket.
- Buffer configuration names:
  - `UPDATE_IPSETS_TELEMETRY_BUFFER_BYTES`: total local log/trace buffer budget.
    Default: `52428800` bytes (50 MiB).
  - `UPDATE_IPSETS_LOG_BUFFER_BYTES`: optional log-buffer budget override.
  - `UPDATE_IPSETS_TRACE_BUFFER_BYTES`: optional trace-buffer budget override.
  - If per-signal overrides are unset, the total budget is split evenly between
    logs and traces.
- Log record schema:
  - fixed record fields: timestamp, level, message, program counter, and a
    bounded inline attribute array
  - hot path copies only fixed record data into a preallocated queue
  - full queue never blocks the caller and increments `telemetry.logs.dropped`
- Trace record schema:
  - fixed record fields: local trace/span ID, event kind, operation name,
    timestamp, elapsed duration, error flag, and a bounded inline attribute
    array
  - hot path copies only fixed record data into a preallocated queue
  - full queue never blocks the caller and increments `telemetry.traces.dropped`
- Sink/export behavior:
  - application producers only write to local state/queues
  - the stderr/local slog sink runs from the log queue
  - OTel export, where configured, consumes snapshots/drains from the isolated
    exporter path and cannot backpressure producers

## Implications And Decisions

1. Decision: OpenTelemetry SDK is not application instrumentation.

   Selection: required by user.

   Implications:

   - OTel imports are forbidden outside one isolated exporter module.
   - Application code cannot use OTel attributes, spans, contexts, meters, log
     providers, or HTTP wrappers.
   - Existing direct OTel call sites must be migrated or removed.

2. Decision: Metrics are exact local state.

   Selection: required by user.

   Implications:

   - Metric updates cannot be dropped to protect availability.
   - Exporter failure cannot affect local metric truth.
   - Any bounded/dropped behavior belongs to remote export, logs, or traces,
     and must be reflected in exact local counters.

3. Decision: Metric cardinality is compile-time finite.

   Selection: required by user.

   Implications:

   - Runtime labels such as feed names, provider names, URL values, client IPs,
     error messages, user agents, and raw paths are not valid metric
     dimensions.
   - Operator drill-down detail must live in admin snapshots, logs, traces, or
     artifacts rather than metric label cardinality.
   - Every metric series must be predeclared or mapped to a predeclared bucket.

4. Decision: Hot-path metrics are lockless atomic and zero allocation.

   Selection: required by user.

   Implications:

   - No maps, string formatting, variadic attribute slices, dynamic labels, or
     lazy metric creation in metric update paths.
   - Series lookup must resolve to a fixed ID before hot-path update, or use
     typed functions per metric/dimension.

5. Decision: Logs/traces are bounded and non-blocking, not disposable.

   Selection: required by user.

   Implications:

   - A healthy remote ingester should receive all records within the configured
     bound.
   - Under remote failure or sustained overload, producers do not block.
   - Overflow is a measured telemetry fault with exact local counters.

6. Decision: Log/trace memory must be preallocated or arena-style.

   Selection: required by user.

   Implications:

   - Acceptable implementation shapes include fixed rings or double-buffer
     arenas.
   - A double-buffer arena means writers append to one large buffer while the
     exporter flushes the other; swap happens as a whole, and individual record
     memory is not freed.
   - Any implementation that allocates per record after startup fails the SOW.

7. Decision: OTel exporter rollout.

   Selection: `1.B` from user. Build the isolated OTel exporter in this SOW.

   Implications:

   - The final implementation must keep remote OTLP export available when
     configured.
   - OTel SDK code may exist only in one isolated exporter module.
   - Ingesting, serving, admin, watchdog, downloader, scheduler, engine, and
     local telemetry APIs cannot call that exporter directly from hot paths.

8. Decision: Log/trace buffer size.

   Selection: `2` from user. Buffer sizes are configurable, with a default
   total log/trace buffer budget of 50 MB.

   Implications:

   - The implementation must expose operator configuration for the buffer
     budget.
   - The default combined budget across bounded log/trace buffers is 50 MB.
   - Buffer admission must remain non-blocking when the configured budget is
     exhausted and must increment exact local overflow counters.

9. Decision: Metric cardinality policy.

   Selection: `3.A` from user. Absolute compile-time series only.

   Implications:

   - Config-derived series are not allowed for metrics.
   - Feed/provider/source names cannot be metric labels.
   - Metric descriptors and allowed dimensions must be defined in code as a
     finite compile-time table.

10. Decision: Trace semantics.

    Selection: `4.A` from user. Use local trace/event IDs internally. Any OTLP
    span conversion is allowed only inside the isolated exporter module.

    Implications:

    - The application must not preserve an OTel-style span API internally.
    - Trace capture must use project-owned types and bounded buffers.
    - Any OTLP span conversion happens only in the isolated exporter module.

## Plan

1. Produce a complete OTel/import/call-site inventory.
2. Draft the local telemetry schema and memory model.
3. Record the exact metric table, allowed dimensions, log/trace record schema,
   buffer config names, and 50 MB default budget before broad migration.
4. Add source guards and behavioral/allocation tests.
5. Implement the local telemetry registry and bounded log/trace buffers.
6. Migrate application call sites away from OTel APIs.
7. Rebuild `/metrics` and admin telemetry views from local snapshots.
8. Isolate or remove the OTel exporter.
9. Update specs, docs, and skills.
10. Run full validation and external review before closure.

## Execution Log

### 2026-06-28

- Created SOW from the user-approved telemetry architecture direction.
- Recorded user decisions:
  - build the isolated OTel exporter in this SOW
  - make log/trace buffer sizes configurable with a 50 MB default total budget
  - use absolute compile-time metric series only
  - use local trace/event IDs internally, with OTLP conversion only inside the
    isolated exporter
- Moved SOW to `.agents/sow/current/` and started implementation.
- Added production liveness-failure evidence and open-source reference evidence
  for predeclared atomic metrics.
- Implemented first local instrumentation slice:
  - replaced OTel SDK metric instruments with project-owned atomic local
    metric series
  - replaced OTel attribute types in production call sites with project-owned
    attributes
  - removed `otelhttp` downloader transport wrapping
  - moved `/metrics` to local Prometheus text rendering from local snapshots
  - converted per-feed metric labels into aggregate feed catalog gauges
  - added source guards, exact-counter tests, finite-bucket tests,
    Prometheus-output tests, and representative zero-allocation hot-path tests
- Completed the local telemetry framework implementation:
  - split `internal/observability` into focused local metric, Prometheus,
    environment, trace, log, and exporter files
  - added bounded local log and trace queues with exact local drop counters
  - added configurable local log/trace buffer budgets with a 50 MiB default
  - kept OTLP metric export in one isolated module:
    `internal/observability/otelexporter`
  - made daemon shutdown logging independent of the local logger/exporter
  - removed OTel HTTP, log, trace, and Prometheus bridge dependencies that are
    no longer used by the application
- Removed normal-path unknown-metric storms:
  - detailed engine and scheduler operation names now remain admin snapshot
    telemetry only
  - processor step names no longer create metric families
  - downloader metric labels are status-only; feed/downloader identity remains
    local trace detail
  - added a source guard proving production metric helper calls use
    string-literal names that resolve to the compile-time schema
- Updated operator docs, operating-principles spec, and runtime project skills
  to describe exact local metrics, bounded local logs/traces, metric-only OTLP
  export, and the isolated exporter boundary.
- External reviewers completed usable reviews from qwen, kimi, glm, mimo, and
  minimax. Deepseek did not return a usable final report in this session.
- Addressed external-review findings:
  - replaced the trace queue contention-prone producer path with non-blocking
    channel admission and a queue-owner flush barrier for deterministic
    snapshots
  - added watchdog diagnostic and systemd notification failure counter call
    sites
  - added finite labels for daemon-control and scheduler recovered-panic
    counters
  - added runtime/process metric snapshots to the local `/metrics` surface
  - removed unwired default `iprange.*` metric families and documented that
    `pkg/iprange` stats remain caller-returned local counters until a
    production caller records them through a predeclared metric surface
  - added OTel exporter tests, route-label/schema tests, post-shutdown log-drop
    counting, string-size bounding for logs/traces, and a stalled-exporter
    producer non-blocking test
  - corrected the `background.worker.wait.duration_ms` spec name
  - added zero-allocation coverage for prebuilt `[]Attr{...}` variadic call
    shapes used by production metric call sites
- Ran a final external-review round after those fixes. Five reviewers returned
  production-grade verdicts; minimax timed out before a final verdict.
- Addressed concrete final-review findings:
  - added first-class finite `download.status="skipped"` metric coverage and
    removed stale unproduced `downloaded`/`download_failed` metric label values
  - corrected operator docs for the exact metric count, downloader metric
    names, binary buffer suffixes, the 10s default metric export interval, and
    the absence of default `iprange` metric families
  - documented `scheduler.action.admission_failures`
  - registered the local trace queue in telemetry shutdown, made
    `SnapshotTraceEvents` nil-safe, and bounded `WithAttrs` string retention
  - added zero-allocation coverage for log enqueue with attributes
  - split source-guard tests into a separate file to keep architecture posture
    large-file limits intact
- Addressed a further reviewer pass after those fixes:
  - added first-class finite `integrity.result` buckets for `issues`,
    `in_progress`, and `scheduled`
  - added first-class finite `integrity.action="recheck"` coverage
  - documented `file.write_atomic`, `file.write_atomic.bytes`, and
    `file.write_atomic.duration_ms`
  - clarified exported HTTP/API metrics versus admin-only engine counters in the
    monitoring overview
  - clarified binary telemetry buffer suffixes in the spec
  - counted serving-safe liveness async-log drops in `telemetry.logs.dropped`
  - moved runtime/process metric collection out of `/metrics` and OTLP export
    paths into a daemon-owned local sampler
  - added tests for watchdog diagnostics, systemd notify failures, scheduler
    recovered-panic counters, and scheduler action-admission failures
  - removed the trace-flush-timeout increment from
    `telemetry.traces.dropped`, preserving that counter for actual drops

## Validation

Acceptance criteria evidence:

- Complete except optional installed-service smoke:
  - `rg -n "go\\.opentelemetry\\.io|otelhttp|otelslog|attribute\\.KeyValue|attribute\\." cmd internal pkg --glob '*.go'`
    shows OTel imports only in `internal/observability/otelexporter` and test
    guards.
  - `internal/observability/observability_test.go` guards the OTel import
    boundary for `cmd`, `internal`, and `pkg`.
  - `internal/observability/observability_test.go` guards production metric
    helper calls so metric names are string literals and resolve to the
    compile-time metric schema.
  - Local metric tests prove exact counter updates, finite runtime-value
    bucketing, local Prometheus output, and zero allocations for representative
    metric/log/trace hot paths.
  - Local log and trace tests prove full buffers drop before blocking and
    increment exact local counters.
  - Environment tests prove trace/log OTLP endpoint variables do not enable
    unsupported trace/log export and that buffer budgets are parsed from the
    configured local env vars.
  - `TestBlockedMetricExporterDoesNotDelayLocalMetricProducers` proves a
    started but stalled OTLP metric exporter does not delay local producer
    metric updates and does not change exact local counter truth.
  - `pkg/web/routes_test.go` proves actual web route telemetry names resolve to
    predeclared metric buckets.

Tests or equivalent validation:

- `go test ./internal/observability` passed.
- `go test ./cmd/update-ipsets ./internal/fileutil ./pkg/downloader ./pkg/processor ./pkg/scheduler ./pkg/cache ./pkg/config ./pkg/engine ./pkg/web` passed.
- `go test ./...` passed, including `tools/archposture`.
- `go test ./internal/observability ./internal/observability/otelexporter` passed.
- `go test ./internal/observability ./pkg/downloader ./pkg/processor ./pkg/scheduler ./pkg/engine` passed.
- `go test -count=1 ./internal/observability ./internal/observability/otelexporter ./pkg/web` passed.
- `go test ./internal/observability ./internal/observability/otelexporter ./pkg/web ./pkg/engine ./pkg/scheduler ./pkg/processor ./pkg/downloader ./pkg/cache ./pkg/config ./internal/fileutil ./cmd/update-ipsets` passed.
- `make test` passed.
- `make lint` passed.
- `make race` passed, including `tools/dronebl2ipsets`.
- After final-review fixes, `make test` passed again, including
  `tools/archposture`.
- After final-review fixes, `make lint` passed again.
- After final-review fixes, `make race` passed again, including
  `tools/dronebl2ipsets`.
- After the runtime-sampler and final low-finding hardening fixes,
  `make test`, `make lint`, and `make race` passed again, including
  `tools/dronebl2ipsets`.
- After the runtime-sampler and final low-finding hardening fixes,
  `go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./internal/observability ./internal/observability/otelexporter ./internal/fileutil ./pkg/downloader ./pkg/processor ./pkg/scheduler ./pkg/cache ./pkg/config ./pkg/web ./cmd/update-ipsets`
  passed.
- `go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./internal/observability ./internal/observability/otelexporter ./internal/fileutil ./pkg/downloader ./pkg/processor ./pkg/scheduler ./pkg/cache ./pkg/config ./pkg/web ./cmd/update-ipsets` passed.
- `make staticcheck` remains blocked by the existing broad `pkg/engine` U1000
  unused-code baseline. The SOW-local staticcheck finding in
  `internal/observability/observability_test.go` was fixed before the scoped
  staticcheck pass.
- Final schema-drift scan over current source/docs/spec surfaces, excluding
  this SOW's review note, found no stale default-iprange metric definitions, no
  stale 86-count wording, and no stale unsuffixed background-worker-wait metric
  reference.

Real-use evidence:

- Not run yet. Installed-service smoke changes the local installed daemon and
  still requires explicit user approval.

Reviewer findings:

- qwen:
  - reported trace queue producer contention/drop risk, ignored daemon/scheduler
    labels, route-label drift, missing five-attribute allocation coverage, and
    stale queue/drop fields
  - addressed by the queue redesign, label schema updates, route schema test,
    allocation guard, and dead-field cleanup
- kimi:
  - reported string-retention risks in bounded queues, missing exporter tests,
    ignored labels, post-shutdown log-drop accounting, and stale race-skip code
  - addressed by string bounding, exporter tests, label schema updates,
    post-shutdown drop tests, and removal of the race-skip pattern
- glm:
  - reported missing exporter tests, route/schema mismatches, dead feed-summary
    helpers, unreachable background component values, dead queue fields, and
    spec wording drift
  - addressed by exporter tests, route-schema tests, dead-helper cleanup,
    component schema cleanup, queue cleanup, and spec/doc updates
- mimo:
  - reported lost runtime/process metrics on `/metrics`, fail-open exporter test
    gaps, dead feed-summary helpers, dead queue fields, and post-shutdown log
    drops
  - addressed by runtime/process local metrics, exporter fail-open tests,
    helper cleanup, queue cleanup, and post-shutdown drop tests
- minimax:
  - reported dead watchdog/systemd/iprange metric families, spec metric-name
    drift, allocation coverage gaps, and missing stalled-exporter validation
  - addressed by wiring watchdog/systemd counters, removing unwired iprange
    metric families, correcting the spec name, adding the prebuilt-attr
    allocation guard, and adding the stalled-exporter producer test
- Deepseek:
  - no usable final report was captured in this session
- Final external-review round after the first reviewer fixes:
  - five reviewers returned production-grade verdicts
  - minimax timed out before a final verdict
  - valid findings were limited to downloader `skipped` status bucket coverage,
    stale operator docs, trace/log defensive hardening, and the
    architecture-posture large-test-file gate
  - all valid findings have been addressed and validated locally
- Reviewer pass after the final-review fixes:
  - Deepseek reported missing finite integrity buckets and documentation drift;
    addressed by the integrity schema/test and doc/spec updates above.
  - Mimo reported missing liveness async-log drop accounting; addressed by
    counting dropped records and adding a focused web test.
  - GLM, qwen, and kimi returned production-grade verdicts with low polish
    observations. Valid low hardening items were addressed by runtime sampling
    outside scrape/export paths, direct counter tests for watchdog/systemd and
    scheduler counters, and trace drop-counter semantic cleanup.
  - Findings that claimed trace truncation and engine admin snapshot counters
    were blockers were rejected as non-blocking: trace truncation to a fixed
    marker is the accepted bounded-record design, and engine lifetime/current
    counters are admin diagnostic snapshots, not Prometheus/OTLP metric
    families. The existing docs explicitly separate admin snapshots from the
    exported local metric surface.
- Closure reviewer round after the final runtime-sampler and drop-counter
  hardening:
  - GLM, kimi, qwen, and Deepseek returned production-grade verdicts with no
    blocking findings.
  - Minimax timed out without a final verdict after read-only validation checks;
    Mimo's final session ended without a retrievable final verdict. These are
    recorded as reviewer execution failures, not approvals.
  - The recurring valid low finding is that local trace events are captured in
    a bounded ring but have no production reader yet. This is not a SOW-0121
    blocker because the trace queue is bounded, non-blocking, and counted, but
    it is valid work and is tracked by
    `.agents/sow/pending/SOW-0122-20260628-local-trace-visibility-policy.md`.

Same-failure scan:

- Initial scan command:
  `rg -n "OpenTelemetry|OTel|observability|telemetry|metrics|traces|slog|otelslog|otelhttp" go.mod internal cmd pkg docs .agents/sow/specs .agents/skills`
- Initial evidence is recorded in the Analysis and Pre-Implementation Gate.

Sensitive data gate:

- This SOW records file paths, line numbers, official documentation URLs, and
  sanitized behavior descriptions only. It contains no raw secrets,
  credentials, bearer tokens, private endpoints, customer data, personal data,
  or production log payloads.

Artifact maintenance gate:

- AGENTS.md: no project-wide handbook change needed; existing SOW-0121
  requirements are now captured in runtime project skills and specs.
- Runtime project skills: updated `project-coding` and `project-operations`.
- Specs: updated `operating-principles.md`.
- End-user/operator docs: updated monitoring, environment, and install docs.
- End-user/operator skills: not affected.
- SOW lifecycle: moved from pending to current; implementation, external
  review, and post-review fixes are validated. Status is `completed`; the SOW
  move to `.agents/sow/done/` and commit are performed together.

Specs update:

- `.agents/sow/specs/operating-principles.md` updated for exact local metrics,
  bounded local logs/traces, metric-only OTLP export, fail-open exporter
  startup, and request-path independence from telemetry export.

Project skills update:

- `.agents/skills/project-coding/SKILL.md` and
  `.agents/skills/project-operations/SKILL.md` updated with the local
  instrumentation contract and isolated exporter boundary.

End-user/operator docs update:

- Updated:
  - `docs/running/environment-variables.md`
  - `docs/monitoring/opentelemetry-setup.md`
  - `docs/monitoring/netdata-integration.md`
  - `docs/monitoring/monitoring-overview.md`
  - `docs/monitoring/telemetry-reference.md`
  - `docs/monitoring/log-structure.md`
  - `docs/installation/systemd-setup.md`

End-user/operator skills update:

- No exported end-user/operator skills are affected.

Lessons:

- Emergency non-blocking wrappers are not enough when the accepted contract is
  exact local metrics and isolated remote telemetry export.

Follow-up mapping:

- Valid follow-up: local trace events currently have no production reader.
  Tracked by
  `.agents/sow/pending/SOW-0122-20260628-local-trace-visibility-policy.md`.
- Existing non-SOW blocker: full project-wide `make staticcheck` remains
  blocked by existing `pkg/engine` U1000 findings outside this SOW; scoped
  staticcheck for the changed packages passes.

## Outcome

Implementation and post-review fixes validated. Final usable external reviewer
verdicts returned production-grade with no blockers. The SOW is completed and
will be moved to `.agents/sow/done/` with the implementation commit.

## Lessons Extracted

- Telemetry contract tests must guard both import boundaries and metric-name
  resolution. Dynamic metric names compile successfully and can otherwise turn
  into `telemetry.metrics.unknown` storms only in production.

## Followup

- `.agents/sow/pending/SOW-0122-20260628-local-trace-visibility-policy.md`
  tracks the local trace visibility policy follow-up.

## Regression Log

None yet.
