# SOW-0118 - Runtime Reload Snapshot Race Audit

## Status

Status: open

Sub-state: pending after SOW-0117 V18. This is a focused follow-up for a valid
runtime reload concurrency finding that was too broad to hide inside the
panic/liveness cleanup loop.

## Requirements

### Purpose

Ensure runtime reloads cannot race with active engine, downloader, public,
integrity, metadata, or entity code that reads runtime configuration.

### User Request

Fix all deadlock/liveness findings from production and external review, while
preserving the application contract and avoiding hidden broad rewrites.

### Assistant Understanding

Facts:

- `Engine.ReloadContext()` writes the engine runtime under `e.mu`.
- Many engine paths read `e.runtime` fields directly without taking `e.mu`.
- Existing `go test -race ./pkg/engine ./pkg/web -count=1` passed during
  SOW-0117 V18, so the race was not reproduced by current tests.
- The direct-read surface is broad enough to deserve its own focused design and
  review.

Inferences:

- The safest long-term approach is likely a runtime snapshot contract:
  operation entrypoints take one `Runtime` copy and pass it down, or all callers
  use a concurrency-safe runtime accessor.
- A mechanical `e.runtime` replacement across many files could change behavior,
  performance, or stale-runtime semantics if done without a contract.

Unknowns:

- Which direct reads are reachable concurrently with `ReloadContext()` in real
  daemon operation.
- Whether the right design is operation-local snapshots, reload admission
  through the engine lane, atomic runtime storage, or targeted accessor
  replacement.

### Acceptance Criteria

- A race-detector test reproduces or disproves the reload/runtime race for at
  least one active engine path.
- All runtime read paths reachable concurrently with reload are inventoried and
  classified.
- The chosen fix has explicit contract text for whether in-flight work uses the
  old runtime snapshot or observes the reloaded runtime.
- `go test -race ./pkg/engine ./pkg/web -count=1` passes after the fix.

## Analysis

Sources checked:

- `pkg/engine/engine.go`
- `pkg/engine/run.go`
- `pkg/engine/download_stage.go`
- `pkg/engine/artifact_stage.go`
- `pkg/engine/status_snapshot.go`
- `rg -n "\be\\.runtime\\." pkg/engine --glob '*.go' --glob '!**/*_test.go'`

Current state:

- `Engine.ReloadContext()` records the old web directory and assigns
  `e.runtime` under `e.mu`.
- Direct runtime reads exist in active processing and serving paths, including
  `pkg/engine/run.go`, `pkg/engine/download_stage.go`,
  `pkg/engine/artifact_stage.go`, `pkg/engine/entity_feed_sidecar_build.go`,
  `pkg/engine/status_snapshot.go`, `pkg/engine/metadata_write.go`,
  `pkg/engine/feed_body_stage.go`, `pkg/engine/integrity_check.go`, and
  related helpers.

Risks:

- A real data race can corrupt runtime reads or fail under `-race` once test
  coverage exercises the overlap.
- A broad runtime accessor rewrite can accidentally change whether in-flight
  work uses old or new runtime values.

## Pre-Implementation Gate

Status: needs-user-decision

Problem / root-cause model:

- Runtime reload mutation is protected by `e.mu`, but many runtime reads are
  not. Locking only writes is not enough in Go; reads and writes must be
  synchronized by the same happens-before relationship, or the value must be
  immutable/atomic.

Evidence reviewed:

- `pkg/engine/engine.go`: reload writes runtime under `e.mu`.
- `pkg/engine/run.go`: active run reads `e.runtime.MaxProcessingWorkers`.
- Direct-read scan found many `e.runtime` field reads outside tests.
- `go test -race ./pkg/engine ./pkg/web -count=1` passed during SOW-0117 V18,
  so current tests do not prove the suspected race.

Affected contracts and surfaces:

- SIGHUP/config reload behavior.
- Active processing runs.
- Downloader and artifact acquisition helpers.
- Public/admin status snapshots.
- Integrity and metadata writers.
- Operator expectation for whether in-flight work observes old or new runtime
  values.

Existing patterns to reuse:

- `Engine.Runtime()` already returns a runtime copy through the engine mutex.
- Several newer paths already snapshot runtime before use.
- SOW-0117 bounded work-lane rules separate engine work from public/watchdog
  availability.

Risk and blast radius:

- High concurrency blast radius if fixed mechanically without tests.
- Medium performance risk if hot paths repeatedly lock `e.mu` instead of taking
  one local snapshot per operation.
- Low data-loss risk if the fix preserves existing file-layout semantics.

Sensitive data handling plan:

- Evidence must stay at file/line and field-name level. Do not copy production
  paths, private endpoints, customer data, secrets, tokens, or non-private
  customer-identifying IP addresses into durable artifacts.

Implementation plan:

1. Add focused race tests for reload overlapping with active run, downloader
   stage, public/status snapshot, and integrity/status paths.
2. Inventory direct runtime reads and classify each as operation-local,
   request-local, reload-only, startup-only, or unsafe concurrent.
3. Present design options and user decision:
   - A. Operation-local runtime snapshots passed down call chains.
   - B. Atomic immutable runtime pointer.
   - C. Admit reload mutation through the engine lane while keeping public
     request access through `Engine.Runtime()`.
4. Implement the selected design in small batches with race tests.

Validation plan:

- Focused `go test -race` tests for reload overlap.
- `go test -race ./pkg/engine ./pkg/web -count=1`.
- `go test ./pkg/engine ./pkg/web -count=1`.
- Same-failure scan for direct `e.runtime` reads after the fix.

Artifact impact plan:

- AGENTS.md: likely no update unless a new runtime-access rule is needed.
- Runtime project skills: likely update `project-coding` if a durable
  runtime-snapshot rule is selected.
- Specs: update processing/reload/admin specs if in-flight runtime visibility
  semantics are formalized.
- End-user/operator docs: likely no update unless reload behavior changes.
- End-user/operator skills: no expected impact.
- SOW lifecycle: this SOW is the concrete follow-up for the SOW-0117 V18
  deferred runtime reload race item.

Open-source reference evidence:

- None. This is a project-specific runtime ownership issue.

Open decisions:

1. Runtime reload visibility contract:
   - A. In-flight work keeps the runtime snapshot it started with.
   - B. In-flight work may observe reloaded runtime values.
   - C. Split by path: engine runs keep snapshots; public/admin reads observe
     latest runtime.

2. Runtime synchronization design:
   - A. Operation-local snapshots.
   - B. Atomic immutable runtime pointer.
   - C. Engine-lane admitted reload mutation plus targeted accessors.

## Plan

1. Build race tests and direct-read inventory.
2. Present decisions with evidence.
3. Implement approved design.
4. Validate with race detector and same-failure scan.

## Execution Log

### 2026-06-25

- Created as pending follow-up from SOW-0117 V18 plan review.

## Validation

Acceptance criteria evidence:

- Pending.

Tests or equivalent validation:

- Pending.

Sensitive data gate:

- No raw secrets, credentials, bearer tokens, SNMP communities, community
  member names, customer names, personal data, non-private customer-identifying
  IPs, private endpoints, or proprietary incident details are included.

Artifact maintenance gate:

- SOW lifecycle: pending SOW created so the runtime reload race finding is not
  lost.

Follow-up mapping:

- This SOW is the follow-up mapping for the runtime reload race finding
  deferred from SOW-0117 V18.
