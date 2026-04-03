# SOW-0045 | 2026-05-01 | go-advisory-lint-backlog

## Status

completed

## Requirements

### Purpose

Turn the advisory Staticcheck/golangci-lint findings introduced by SOW-0043
into actionable cleanup so advisory CI does not become ignored noise.

### User request quoted verbatim

> Reviewers have created SOWs 38-41 as a follow up work on 31-34.

### Assistant understanding

- This SOW is a concrete continuation from SOW-0043 first-adoption lint
  results.
- It must not be batched with another SOW.
- `SOW-0044` should run first because it originated from SOW-0038 and overlaps
  the largest error-handling/`errcheck` class.
- This SOW then owns the remaining Staticcheck/golangci-lint backlog and the
  decision about graduating advisory gates to blocking gates.

### Acceptance criteria

- Re-run `make staticcheck` and `make golangci-lint` from a clean worktree and
  classify findings by true bug, intentional pattern, test-only issue, or
  tooling noise.
- Fix true positives in focused batches with tests where behavior can change.
- Avoid broad suppressions; every suppression or exclusion must be local,
  justified, and documented in this SOW.
- Include root module and `tools/dronebl2ipsets`.
- Decide whether any advisory gate can become blocking after cleanup, with
  evidence.
- Update project testing/reviewing skills when gate posture changes.

## Analysis

- `make staticcheck` initially failed on real issues in the root module:
  `SA1012` nil context usage in telemetry helpers, `SA5011` nil receiver risk
  in cache removal, `U1000` dead private helpers, empty branches, bool
  simplifications, and non-idiomatic error strings. The nested DroneBL module
  was included in every run.
- `make golangci-lint` initially reported an actionable `errcheck` backlog in
  CLI output, temp-file cleanup, gzip/file/mmap close paths, HTTP response body
  cleanup, tests, benchmarks, and the nested DroneBL module. Subsequent passes
  surfaced the remaining files after earlier findings were fixed.
- Findings were classified as:
  - True production cleanup/reporting issues: close/sync/remove handling for
    atomic writes, staging, web file copies, locks, env-file loading, scheduler
    state, compose/query file-backed sets, and gzip/static serving.
  - Intentional best-effort cleanup: temp-file removal after an already-returned
    error, test teardown, benchmark output cleanup, and diagnostic writes.
  - Dead code: unused private helpers with no production callers.
  - Tooling noise: none. No broad exclusions or global suppressions were added.
- The cleanup produced clean root and nested results for both Staticcheck and
  golangci-lint, so the advisory gates can be made blocking.

## Implications and Decisions

- Decision: graduate Staticcheck and golangci-lint from advisory to blocking CI
  gates.
- Evidence: after cleanup, `make staticcheck` and `make golangci-lint` both
  return 0 issues for the root module and `tools/dronebl2ipsets`.
- Risk: subsequent Go changes now fail CI on static-analysis regressions. This is
  intentional; it prevents the advisory backlog from returning as ignored
  noise.
- Non-goal: no broad linter configuration or suppressions were introduced. The
  existing tool defaults remain the project contract.

## Plan

1. Re-run Staticcheck and golangci-lint on root and nested modules.
2. Fix true positives in focused batches, preserving runtime semantics.
3. Remove dead private helpers instead of writing tests around unreachable code.
4. Make cleanup errors explicit: return/check where they affect persisted data,
   and explicitly ignore only best-effort teardown.
5. Graduate CI gate posture and update project skills.
6. Validate with build, tests, vet, Staticcheck, golangci-lint, govulncheck,
   install, and live service smoke checks.

## Execution Log

- Created from SOW-0043 after first-adoption advisory gates reported real
  Staticcheck/golangci-lint backlog.
- Removed unused private helpers and simplified Staticcheck-reported branches,
  boolean comparisons, and dead assignments.
- Added explicit background contexts for telemetry call sites that previously
  passed nil contexts.
- Fixed cleanup and error handling across file, gzip, mmap/pread file-set,
  HTTP response body, temp-file, scheduler-state, lock, CLI, test, benchmark,
  and nested DroneBL code paths.
- Added `closeClosableSources` for engine compose/query cleanup of file-backed
  range sources.
- Removed the `lint-advisory` make target and changed CI Staticcheck and
  golangci-lint steps to blocking.
- Updated project coding/testing/reviewing skills to reflect the new gate
  posture and cleanup-error rule.

## Validation

- `make build` passed.
- `make test` passed.
- `make test-tools` passed.
- `make lint` passed.
- `make staticcheck` passed for root and `tools/dronebl2ipsets`.
- `make golangci-lint` passed with 0 issues for root and
  `tools/dronebl2ipsets`.
- `make vulncheck` passed for root and `tools/dronebl2ipsets`.
- `git diff --check` passed.
- `./install.sh` passed and restarted `update-ipsets.service`.
- `systemctl is-active update-ipsets` returned `active`.
- `curl -fsS http://localhost:18888/healthz` returned `ok`.
- `GET /api/v1/admin/integrity` returned `status: clean`, `count: 0`.
- `GET /api/v1/admin/integrity/entities` returned `status: clean`, `count: 0`
  after the scheduled post-install run completed.

## Outcome

- The Go static-analysis backlog is cleared.
- Staticcheck and golangci-lint are now blocking project gates in CI.
- No global suppressions were added.
- The installed development service is healthy and both integrity surfaces are
  clean.
- Close-gate search found no valid untracked deferred work in this SOW.

## Lessons Extracted

- Advisory gates are useful for first adoption, but they must quickly become
  blocking or they become noise.
- Cleanup return values need deliberate handling. Production writers and
  persisted artifacts should return meaningful close/sync/remove failures;
  best-effort teardown should use explicit ignored errors so the intent is
  visible to reviewers and linters.
- Static-analysis validation must include nested modules, not just the root
  module.
