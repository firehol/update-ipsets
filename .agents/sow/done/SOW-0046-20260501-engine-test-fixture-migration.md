# SOW-0046 | 2026-05-01 | engine-test-fixture-migration

## Status

completed

## Requirements

### Purpose

Reduce engine test fragility by replacing partial `&Engine{}` literals and
private-helper coupling with focused fixtures that exercise observable engine
contracts.

### User request quoted verbatim

> deferred items from an SOW does not mean "let's do it later". It means "I
> want to be focused for this, let's do it immediately after alone".

### Assistant understanding

- SOW-0039 confirmed that `&Engine{}` literals grew from 73 sites across 15
  files to 74 sites across 27 files.
- The current SOW implemented small, safe Go-test hardening items. This larger
  migration is intentionally isolated so it can be designed and reviewed as its
  own change.
- The target is not cosmetic cleanup. The target is tests that keep passing
  when engine internals are reorganized, and fail when public engine behavior
  changes.

### Acceptance criteria

- Inventory all `&Engine{}` literals and unexported engine-helper calls in
  tests.
- Define the minimum fixture API needed for current engine tests without
  exporting production-only internals.
- Migrate engine tests in reviewable waves.
- Preserve or improve current coverage of processing, artifacts, metadata,
  comparisons, critical-infrastructure outputs, provider defaults, and
  cancellation behavior.
- Validation includes `go test ./pkg/engine`, `make test`, `make test-strict`,
  `make race`, and blocking analysis gates.

## Analysis

- Source SOW: SOW-0039.
- Finding class: SOW-0032 A2/A10 regression.
- Evidence from SOW-0039: partial engine construction spread to many more
  files after the original "defer" decision, proving this needs focused work.
- Direct `&Engine{}` construction inventory before this SOW: 74 sites across
  27 engine test files, plus the package fixture that did not yet exist.
- Direct `&Engine{}` construction inventory after this SOW: 1 site, only in
  `pkg/engine/engine_fixture_test.go`.
- Unexported helper coupling from SOW-0039 A10 is real but is not the same
  failure mode as partial `Engine` construction. Mass-rewriting those tests
  inside this fixture migration would mix behavior-boundary decisions with
  constructor hygiene. It is tracked as SOW-0049 instead of being silently
  dismissed.

## Plan

1. Add a package-local engine test fixture that centralizes default config,
   runtime, state, logger, caches, and background limiter setup without
   exporting production-only APIs.
2. Migrate direct `&Engine{}` literals in waves, starting with tests that only
   need config/runtime/state setup and no bespoke constructor behavior.
3. Keep tests that already use production `New(cfgPath, ...)` unchanged.
4. Run `go test ./pkg/engine` after each migration wave, then the full SOW
   validation gates.

## Validation

- `go test ./pkg/engine` - passed.
- `make test` - passed.
- `make test-tools` - passed.
- `make test-strict` - passed.
- `make fuzz-replay` - passed.
- `make lint` - passed.
- `make staticcheck` - passed.
- `make golangci-lint` - passed.
- `make vulncheck` - passed.
- `make race` - passed.
- `git diff --check` - passed.
- Product specs: not updated. Reason: this SOW changes test construction
  hygiene only; it does not change runtime behavior, file layout, public API,
  admin API, UI behavior, integrity semantics, or operator configuration.
- Project skills: `project-testing` updated with the engine fixture and direct
  constructor guard rule.

## Outcome

Completed.

Shipped changes:

- Added `pkg/engine/engine_fixture_test.go` with a package-local
  `newEngineFixture` helper that centralizes default runtime directories,
  config, cache state, logger, downloader, provider caches, ledger cache,
  query-set cache, clock, and background limiter setup.
- Replaced all direct `&Engine{}` literals in engine tests with
  `newEngineFixture` options.
- Added an AST regression test that fails if any engine test file except the
  fixture file constructs `Engine` directly.
- Kept tests that already exercise the production constructor
  `New(cfgPath, ...)` unchanged.

## Lessons extracted

- Test fixtures must protect the construction contract they introduce. The new
  AST guard makes direct `&Engine{}` regression observable immediately.
- Do not mix separate test-quality concerns in one cleanup. Partial engine
  construction and same-package unexported-helper coupling are related, but
  they need different acceptance criteria and review boundaries.

## Followup

- `.agents/sow/done/SOW-0049-20260501-engine-behavior-boundary-tests.md`
