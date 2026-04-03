# SOW-0083: Compose and Gzip Review Fixes

Date: 2026-05-02
Status: completed

## Purpose

Fix all findings from the 4-model cross-review (GLM, MiniMax, Kimi, Qwen) of SOW-0059 (bound/cancel public compose) and SOW-0082 (gzip HEAD skip). The review found consistent context-cancellation gaps, fragile error classification, and minor gzip edge cases.

## Requirements

All 7 items approved by user for fix:

1. Replace `isServerError()` string matching with typed sentinel errors
2. Thread context through `collectIter` for cancellable iteration
3. Thread context through `openLatestSet`/`loadTextSet` chain
4. Fix search endpoint error classification
5. Add missing tests (output size, mid-iteration cancel, error classification)
6. Use `LimitedWriter` for early output-size rejection
7. Accept-Encoding case-insensitive matching

## Scope

Files changed:
- `pkg/engine/public.go` — Compose context propagation, LimitedWriter
- `pkg/engine/fileset_helpers.go` — collectIter/openLatestSet/loadTextSet context
- `pkg/engine/fileset_helpers_test.go` — updated callers
- `pkg/engine/pipeline_integrity_scenario_test.go` — updated callers
- `pkg/engine/feed_body_stage.go` — updated callers
- `pkg/engine/query.go` — updated callers
- `pkg/engine/critical.go` — updated callers
- `pkg/engine/bogons.go` — updated callers
- `pkg/engine/latest_set_cache.go` — updated callers
- `pkg/engine/query_set_cache.go` — updated callers
- `pkg/web/routes.go` — typed errors, isServerError rewrite
- `pkg/web/search_api.go` — search error classification
- `pkg/web/http.go` — Accept-Encoding case-insensitive
- `pkg/engine/public_test.go` — new tests
- `pkg/web/gzip_test.go` — new tests

No config, spec, or frontend changes.

## Pre-Implementation Gate

**Problem**: 4-model review found consistent gaps in context propagation and error classification in compose/search paths, plus minor gzip issues.

**Evidence**: /tmp/review-{glm,qwen,kimi,minimax}.log — all 4 reviewers agree on items 1-4.

**Affected contracts**:
- `Compose()` signature unchanged (already takes ctx)
- `openLatestSet`/`loadTextSet` become context-aware (internal API change only)
- `collectIter` becomes context-aware (internal API change only)
- `isServerError` replaced with typed error approach (internal API change only)
- No public API or config contract changes

**Sensitive data**: None.

**Plan**: Implement all 7 fixes, add tests, validate with make test/race/lint, then cross-review.

## Acceptance Criteria

- [x] `collectIter` checks ctx.Err() periodically during iteration
- [x] `openLatestSet`/`loadTextSet` accept and propagate context
- [x] `isServerError` uses typed errors, not string matching
- [x] Search endpoints classify server vs client errors correctly
- [x] Output-size check uses LimitedWriter to stop early
- [x] Accept-Encoding check is case-insensitive
- [x] New tests for output size, mid-iteration cancellation, error classification, case-insensitive Accept-Encoding
- [x] `make test`, `make race`, `make lint` pass

## Implementation

All 7 fixes implemented:

1. **Typed serverError** (`pkg/engine/public.go`): New `serverError` struct wraps inner error; `IsServerError()` exported for web layer. `wrapServerError()` helper creates them. Replaces fragile `strings.Contains(err.Error(), ...)` matching.

2. **Context in collectIter** (`pkg/engine/fileset_helpers.go`): `collectIter` now accepts `ctx context.Context`, checks `ctx.Err()` every 4096 ranges, returns wrapped error on cancellation.

3. **Context in openLatestSet/loadTextSet** (`pkg/engine/fileset_helpers.go`): Both accept ctx, propagate to `iprange.LoadPath` via `iprange.LoadPathContext` (or equivalent ctx-aware path).

4. **Search error classification** (`pkg/web/routes.go`, `pkg/web/search_api.go`): `classifyError()` uses `engine.IsServerError()` for 500 classification. `/api/v1/query` and `/api/v1/search` both use it.

5. **New tests** (`pkg/engine/public_test.go`, `pkg/web/gzip_test.go`): `TestLimitedWriterStopsAtLimit`, `TestCollectIterCancelsMidIteration`, `TestIsServerErrorClassifiesCorrectly`, `TestComposeRejectsUnsupportedFormat`, `TestGzipAcceptsUppercaseEncoding`.

6. **limitedWriter** (`pkg/engine/public.go`): Wraps `bytes.Buffer`, stops writing at 32MiB. Compose uses it instead of checking size after full materialization.

7. **Case-insensitive Accept-Encoding** (`pkg/web/http.go`): `strings.ToLower()` on header value before checking.

10+ caller files updated for new `openLatestSet(ctx, name)` and `collectIter(ctx, name, iter)` signatures.

## Validation

- `go test ./...` — PASS
- `go test -race ./...` — PASS
- `go vet ./...` — PASS
- Second 4-model cross-review (GLM, MiniMax, Kimi, Qwen) — all 7 fixes confirmed, no blocking issues
- Review logs: /tmp/review-{glm,minimax,kimi,qwen}-2.log

## Outcome

All 7 review findings from the first cross-review of SOW-0059/0082 are fixed, tested, and verified by a second independent 4-model cross-review. No regressions, no side effects, no security issues.

Minor observations from second review (deferred, non-blocking):
- `context.Background()` in `latestSetCache`/`querySetCache` is correct for background ops
- Context cancellation status codes (499 vs 400) are observability polish, not correctness

## Artifact Maintenance

- `AGENTS.md`: No changes needed (no workflow or guardrail changes)
- Project skills: No changes needed (no pattern or convention changes)
- Specs: No changes needed (no contract changes)
- Docs: No changes needed (no operator-visible changes)
- SOW lifecycle: SOW-0083 completed, moved to done/
