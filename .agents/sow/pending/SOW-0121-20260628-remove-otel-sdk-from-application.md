# SOW-0121 - Remove OpenTelemetry SDK From Application Hot Paths

## Status

Status: open

Sub-state: pending implementation SOW created from user-approved telemetry
architecture direction.

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

Unknowns:

- Exact compile-time metric series list and histogram/timing bucket layout.
- Exact bounded log/trace record schema and buffer capacities.
- Whether the isolated OTel exporter module should be retained immediately or
  removed first and reintroduced later after local telemetry is proven.

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

Current state:

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

Status: needs-user-decision

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
- SOW lifecycle: this SOW remains pending until implementation starts. It must
  not be merged into SOW-0117 unless the user explicitly decides to reopen and
  replace that work.

Open-source reference evidence:

- Official OpenTelemetry specifications were checked for SDK behavior. Before
  implementation starts, compare at least two local mirrored observability
  projects that isolate telemetry/export pipelines, and cite upstream repo and
  checked commit if any pattern is copied.

Open decisions:

1. Exact metric descriptor table and allowed dimensions must be reviewed before
   broad migration.
2. Log/trace buffer capacity and memory budget must be chosen before code.
3. Decide whether to:
   - remove all OTel dependencies first and add an isolated exporter later; or
   - retain a single isolated exporter module in the same implementation pass.

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

## Plan

1. Produce a complete OTel/import/call-site inventory.
2. Draft the local telemetry schema and memory model.
3. Present the exact metric table and log/trace memory budget decisions before
   code.
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
- No implementation started.

## Validation

Acceptance criteria evidence:

- Pending implementation.

Tests or equivalent validation:

- Pending implementation.

Real-use evidence:

- Pending implementation.

Reviewer findings:

- Pending implementation.

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

- AGENTS.md: pending implementation decision.
- Runtime project skills: pending implementation.
- Specs: pending implementation.
- End-user/operator docs: pending implementation.
- End-user/operator skills: likely unaffected; confirm during closure.
- SOW lifecycle: created as pending SOW; no implementation commit yet.

Specs update:

- Pending implementation.

Project skills update:

- Pending implementation.

End-user/operator docs update:

- Pending implementation.

End-user/operator skills update:

- Pending implementation.

Lessons:

- Emergency non-blocking wrappers are not enough when the accepted contract is
  exact local metrics and isolated remote telemetry export.

Follow-up mapping:

- Pending implementation.

## Outcome

Pending.

## Lessons Extracted

Pending.

## Followup

None yet.

## Regression Log

None yet.

