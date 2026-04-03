# SOW-0022 | 2026-04-26 | fix-linter-vet-timing

## Status

completed
`make lint` now passes.

## Requirements

Given `make lint` currently fails, when this SOW is complete, then `make lint`
must pass.

Given the linter failures are deferred `time.Since(...)` calls, when the code
is patched, then elapsed observability durations must be computed when the
deferred function runs, not when the defer statement is registered.

Given the fix is narrow, when the SOW is complete, then unrelated behavior and
files must remain untouched.

Verification methods:

- Run `make lint`.
- Run `go test ./...`.
- Inspect the diff for only the three reported vet call sites and SOW/project
  skill documentation.

## Analysis

Sources consulted:

- `make lint` output from the previous SOW and current inspection.
- `pkg/cache/cache.go:91`
- `pkg/config/config.go:535`
- `pkg/engine/background_tasks.go:200`
- `.agents/skills/project-coding/SKILL.md`
- `.agents/skills/project-testing/SKILL.md`

Current state:

- `make lint` runs `go vet ./...`.
- `go vet` reports three `call to time.Since is not deferred` findings.
- In each case, `time.Since(started)` is evaluated immediately because it is an
  argument to a deferred direct function call.

Root cause:

- Deferred direct calls evaluate arguments at defer registration time. These
  observability durations therefore record near-zero elapsed time and fail vet.

Scope:

- Change only the three deferred observability call sites to deferred closures.
- Add the small project-skill lesson so future deferred timing instrumentation
  avoids the same pattern.
- No product specs need updates; this is internal instrumentation correctness.

## Implications and decisions

- Decision: use deferred closures at each reported call site.
- Reasoning: this is the standard Go pattern for measuring elapsed time at
  function exit and preserves existing observability calls.
- Risk: very low. The changes affect timing telemetry values only; they do not
  change cache/config parsing, locking behavior, or public serving.

## Plan

Single-unit implementation, no chunking — reasoning: the failure is three
specific `go vet` call sites with the same mechanical fix.

Steps:

1. Patch the three deferred direct calls into deferred closures.
2. Add a concise project-coding lesson for deferred timing instrumentation.
3. Run `gofmt`, `make lint`, and `go test ./...`.
4. Complete this SOW and commit the exact touched files.

## Execution log

2026-04-26:

- Opened SOW-0022 from Costa's direct request to fix the linter.
- Analysis identified exactly three vet failures caused by eager evaluation of
  `time.Since(...)` in deferred direct calls.
- Patched:
  - `pkg/cache/cache.go`
  - `pkg/config/config.go`
  - `pkg/engine/background_tasks.go`
- Updated `.agents/skills/project-coding/SKILL.md` with the deferred
  elapsed-time observation pattern.

## Validation

- [x] Acceptance criteria evidence

  `make lint` passed. `make test` passed.

- [x] Real-use validation evidence

  For this SOW, the user-facing workflow is the developer validation command.
  `make lint` now runs `go vet ./...` with no findings.

- [x] Cross-model reviewer findings (logged + addressed)

  N/A — low-risk three-call-site lint fix; no external assistants were invoked.
  In-session review confirmed the diff only changes the reported deferred
  timing call sites plus the SOW/project-skill documentation.

- [x] Lessons extracted (or "none, reasoning: ...")

  Lesson extracted and added to `.agents/skills/project-coding/SKILL.md`.

- [x] Same-failure-at-other-scales check

  `rg -n "defer\\s+[A-Za-z0-9_\\.]+\\([^\\n]*time\\.Since" pkg internal cmd`
  found no remaining deferred direct calls with `time.Since(...)`.

## Outcome

The linter failure is fixed. Deferred observability timing now computes elapsed
duration at function exit for cache load, config load, and entity writer lock
hold timing.

## Lessons extracted

- Deferred direct-call arguments are evaluated immediately in Go. For elapsed
  timing instrumentation, defer a closure and call `time.Since(started)` inside
  the closure. Captured in `.agents/skills/project-coding/SKILL.md`.
