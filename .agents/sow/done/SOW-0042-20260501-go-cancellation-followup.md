# SOW-0042 | 2026-05-01 | go-cancellation-followup

## Status

completed

## Requirements

### Purpose

Complete the Go cancellation and shutdown contract exposed by SOW-0038's
second-round review. Long-running daemon, web, scheduler, engine, and
entity-artifact work should stop scheduling new work on cancellation, propagate
the caller context into current work where feasible, and wait for owned
goroutines before returning. This is operational correctness work: shutdown
must be predictable and tests must not hide races behind process exit.

### User request quoted verbatim

> Reviewers have created SOWs 38-41 as a follow up work on 31-34.

Follow-up approval:

> ok

### Assistant understanding

- Work one SOW at a time.
- SOW-0038 is analysis-only and now closed with internal maintainer decisions.
- This SOW executes SOW-0038 Decision 1(a): cancellation/shutdown completeness.
- The work is internal behavior and quality, not a product UX/design decision.
- Preserve existing admin/runtime behavior while improving cancellation paths.

### Acceptance criteria

- `web.Run` waits for the `runner.Run` goroutine it starts before returning.
- Entity artifact rebuild/refresh APIs accept and propagate `context.Context`
  through queue execution and explicit rebuild paths.
- Engine heavy-phase bounded workers pass `context.Context` into per-name
  worker bodies and stop admitting new work on cancellation.
- Residual sender-side fan-out loops avoid blocking forever when their parent
  context is canceled.
- Residual project-internal `context.Background()` sites in engine/processor
  work are removed or justified with evidence.
- `gunzipFile` observes cancellation instead of discarding its context.
- Existing admin behavior and installed service startup/shutdown remain intact.
- Tests cover the changed cancellation contracts.

## Analysis

Source analysis from SOW-0038:

- `pkg/web/server.go` starts `go runner.Run(runCtx)` but only waits for HTTP
  listener goroutines before returning.
- Entity artifact API boundaries are contextless:
  `EnsureEntityArtifactsCurrentWithTrigger`, `RebuildEntityArtifactsWithTrigger`,
  and queue helpers in `pkg/engine/entity_refresh_queue.go`.
- `runBoundedNameJobs` accepts `ctx` for dispatch but calls `fn(name)` without
  passing `ctx`; current in-flight worker bodies can continue until their
  current feed finishes.
- Remaining fan-out send loops in DNS/processor code send to unbuffered job
  channels without selecting on cancellation.
- Several `context.Background()` sites are internal work calls that should
  inherit the caller context or be explicitly justified.
- `gunzipFile(_ context.Context, ...)` declares a context parameter but ignores
  it.

Pre-implementation verification to do:

- Re-read all listed sites in current code before editing; SOW-0038 was
  produced by reviewers and may be stale after SOW-0037.
- Identify which context changes affect public package APIs and update tests
  and call sites atomically.

## Implications and Decisions

Autonomous maintainer decision: implement SOW-0038 Decision 1(a).

Risk and mitigation:

- `context.Context` threading can be broad. Keep edits scoped to the six
  cancellation findings and avoid unrelated fan-out refactors.
- `pkg/iprange` stays standalone. Do not import project packages into it.
- Public/admin behavior must remain unchanged except for better cancellation.
- If a residual `context.Background()` is intentionally used because no caller
  context exists, record the justification in this SOW and leave code clear.

## Plan

1. Re-read and map cancellation/shutdown sites from SOW-0038.
2. Patch `web.Run` runner ownership.
3. Thread context through entity artifact public APIs and queue runners.
4. Thread context into `runBoundedNameJobs` worker functions and adopters.
5. Fix sender-side cancellation in residual fan-out loops where in scope.
6. Fix or justify residual `context.Background()` and `gunzipFile` context use.
7. Add/update targeted tests.
8. Run Go validation, installed-service smoke, and update specs/skills if a
   durable contract changes.

## Execution Log

- Created from SOW-0038 Decision 1(a).
- Threaded `context.Context` through entity artifact ensure, rebuild, refresh,
  repair, queue-runner, health-transition, and feed-update paths.
- Made daemon startup/reload, scheduler feed-update/health-transition refresh,
  and admin entity rebuild calls use a service/root operation context.
- Changed `web.Run` to own and wait for the scheduler runner goroutine before
  returning, including early listener-error returns.
- Changed `runBoundedNameJobs` so per-name worker bodies receive the same
  cancellable context as dispatch.
- Fixed sender-side cancellation in hostname/DNS fan-out loops so blocked
  senders stop scheduling after cancellation.
- Made gzip decompression observe cancellation and moved compression helpers
  into a focused processor file to preserve architecture posture.
- Threaded caller context through retention snapshot parsing, query
  first-seen cohort scans, and history-derivative recheck target resolution.
- Split large files instead of updating architecture baselines:
  `entity_artifact_repair.go`, `surface_handler.go`, `surface_routes.go`,
  and `compression.go`.

Residual `context.Background()` audit:

- Top-level CLI/daemon roots remain legitimate root contexts:
  `cmd/update-ipsets/main.go`, `cmd/update-ipsets/daemon.go`, and
  `cmd/update-ipsets/query.go`.
- Nil-context fallbacks remain legitimate defensive code:
  `nonNilContext`, processor/iprange nil-context guards, web test/helper
  constructors, scheduler recovery fallback, observability helpers, and
  `iprange` OpenTelemetry helpers.
- Bounded shutdown context remains deliberate:
  `pkg/web/server.go` uses a fresh 10-second shutdown context after service
  cancellation.
- `Engine.RebuildEntityArtifacts()` remains a compatibility wrapper around the
  context-aware `RebuildEntityArtifactsWithTrigger(ctx, ...)`; production
  daemon/admin/scheduler paths now call the context-aware APIs.
- Remaining local synchronous fallbacks without a live owner context are
  documented as acceptable for this SOW: retention JSON fallback from
  `Retention`/public-series generation, geolocation archive parsing,
  bootstrap disk-stat parsing, and `openLatestSet` text fallback. These are not
  daemon-owned background goroutines; they either run from root CLI/API calls
  that do not yet expose a context or from startup/local-state bootstrap.

## Validation

- `go test ./pkg/engine` - pass.
- `go test ./pkg/iprange` - pass.
- `go test ./pkg/processor` - pass.
- `go test ./pkg/web ./pkg/scheduler` - pass.
- `go test ./...` - pass, including `tools/archposture`.
- `make build` - pass.
- `./install.sh` - pass; service restarted.
- `curl http://localhost:18888/healthz` - pass (`ok`).
- `curl http://localhost:18888/api/v1/admin/integrity` - clean, zero
  findings.
- `curl http://localhost:18888/api/v1/admin/integrity/entities` - clean,
  zero findings after startup entity background refresh completed.
- `systemctl is-active update-ipsets` - active.

## Outcome

- Cancellation/shutdown follow-up implemented.
- Entity artifact background work now has a context-aware API boundary and
  daemon-owned callers pass service context.
- Web daemon shutdown now waits for the scheduler runner it starts.
- DNS/hostname/gzip cancellation tests cover the formerly fragile paths.
- Architecture posture stayed within existing baselines by splitting files by
  responsibility instead of weakening the guard.

## Lessons Extracted

- Context work is not complete when only the dispatch loop sees cancellation;
  per-item workers and queue runners need the same context.
- Background tasks started from admin routes should use service lifetime, not
  request lifetime, or they can be canceled as soon as the response is written.
- Architecture guard failures should be handled by improving file/module
  boundaries first, not by baseline updates.
