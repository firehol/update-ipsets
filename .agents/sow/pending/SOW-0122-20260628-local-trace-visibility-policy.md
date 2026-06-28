# SOW-0122 - Local Trace Visibility Policy

## Status

Status: open

Sub-state: Pending user decision; created as a follow-up from SOW-0121 reviewer findings.

## Requirements

### Purpose

Ensure local trace records either provide operator value through a deliberate
production surface or impose no default runtime/memory cost.

### User Request

SOW-0121 removed OpenTelemetry trace/log SDKs from application hot paths and
kept local bounded trace capture. Final reviewers found that the local trace
ring has no production reader yet. This SOW tracks the follow-up so the finding
is not lost.

### Assistant Understanding

Facts:

- `internal/observability/traces.go` stores local trace events in a bounded
  ring and exposes `SnapshotTraceEvents`.
- `SnapshotTraceEvents` is currently used by tests only; no admin API, public
  API, exporter, or operator command reads it in production.
- The default telemetry budget is split between logs and traces, so the trace
  ring can reserve memory even though operators cannot inspect it yet.
- SOW-0121 intentionally removed OpenTelemetry trace export from application
  hot paths.

Inferences:

- Capturing bounded local trace events without a reader is safe but has weak
  operator value.
- The next decision should optimize for production usefulness without
  reintroducing exporter backpressure into application paths.

Unknowns:

- Which trace policy the user wants long term: admin-local diagnostics, disabled
  trace capture until needed, or isolated trace export from local snapshots.

### Acceptance Criteria

- A user-approved trace policy is recorded before implementation.
- If trace capture remains enabled by default, a production reader exists and is
  documented.
- If trace capture is disabled by default, default memory usage and operator docs
  reflect that.
- Application hot paths remain free of OpenTelemetry SDK calls and exporter
  backpressure.
- Local trace/log queues remain bounded and non-blocking with exact drop
  counters.

## Analysis

Sources checked:

- SOW-0121 final reviewer findings.
- `internal/observability/traces.go`
- `internal/observability/observability.go`
- `docs/monitoring/telemetry-reference.md`
- `.agents/sow/specs/operating-principles.md`

Current state:

- Local trace capture exists and is tested.
- No production operator surface currently consumes the trace ring.

Risks:

- Keeping traces enabled without a reader spends memory and CPU for little value.
- Adding remote trace export directly would risk reintroducing the class of
  telemetry problems SOW-0121 removed.
- Exposing traces through admin surfaces must preserve sanitization and bounded
  payload behavior.

## Pre-Implementation Gate

Status: blocked

Problem / root-cause model:

- SOW-0121 correctly isolated telemetry export and bounded local traces, but the
  local trace ring is currently an internal-only diagnostic buffer. The missing
  production consumer is a product/design decision, not a correctness bug in
  SOW-0121.

Evidence reviewed:

- SOW-0121 closure reviewer findings identified no production calls to
  `SnapshotTraceEvents`.
- Search evidence in SOW-0121 showed only tests call `SnapshotTraceEvents`.

Affected contracts and surfaces:

- Admin API/status, operator docs, telemetry docs, observability internals,
  memory behavior, and SOW/spec guidance.

Existing patterns to reuse:

- Admin status cache-first behavior and bounded payload practices.
- Local metric/log/trace non-blocking design from SOW-0121.
- Serving-safe logger and diagnostics sanitization from `pkg/web/liveness.go`.

Risk and blast radius:

- Admin trace output can accidentally expose sensitive values if raw trace
  attributes are surfaced without sanitization.
- Disabling trace capture removes a future diagnostic aid.
- Remote trace export increases dependency and operational complexity.

Sensitive data handling plan:

- SOW/spec/docs will not include raw traces, production payloads, secrets,
  credentials, private endpoints, customer data, personal data, or
  customer-identifying IP addresses.
- Any trace reader must sanitize or avoid sensitive attribute values before
  durable/operator exposure.

Implementation plan:

1. Record the user-selected trace policy and update the relevant spec/doc
   contract.
2. Implement the selected policy with focused tests and no OpenTelemetry SDK
   imports outside the isolated exporter boundary.

Validation plan:

- Source guard proving no OpenTelemetry SDK imports in application paths.
- Behavioral tests for selected trace policy.
- `make test`, `make lint`, `make race`, and scoped staticcheck for affected
  packages.

Artifact impact plan:

- AGENTS.md: likely unaffected.
- Runtime project skills: update if the chosen policy creates durable agent
  rules.
- Specs: update operating-principles and admin-ui/monitoring specs as needed.
- End-user/operator docs: update monitoring/admin docs as needed.
- End-user/operator skills: likely unaffected.
- SOW lifecycle: this is a pending follow-up from SOW-0121.

Open-source reference evidence:

- None checked yet. The issue is local product policy around an internal trace
  ring, not a protocol implementation question.

Open decisions:

1. Trace policy.
   - A. Add an admin-local diagnostic trace reader. Recommended
     **long-term-best** path because it gives the existing bounded trace ring
     operator value without remote-export backpressure.
   - B. Disable local trace capture by default until a reader exists. This is
     the most surgical path and reduces memory/CPU, but loses local trace
     diagnostics.
   - C. Add isolated OTLP trace export from local trace snapshots. This may help
     centralized tracing, but it adds exporter complexity and must not affect
     application hot paths.

## Implications And Decisions

Pending.

## Plan

Pending user decision.

## Execution Log

### 2026-06-28

- Created as a concrete follow-up from SOW-0121 closure review.

## Validation

Pending.

## Outcome

Pending.

## Lessons Extracted

Pending.

## Followup

None yet.

## Regression Log

None yet.
