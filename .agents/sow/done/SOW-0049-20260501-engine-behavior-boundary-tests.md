# SOW-0049 | 2026-05-01 | engine-behavior-boundary-tests

## Status

completed

## Requirements

### Purpose

Reduce brittle same-package engine tests that assert unexported helper behavior
when the same contract can be asserted through stable engine-level behavior,
published artifacts, or public test fixtures.

### User request quoted verbatim

> deferred items from an SOW does not mean "let's do it later". It means "I
> want to be focused for this, let's do it immediately after alone".

### Assistant understanding

- SOW-0039 identified A10: same-package tests call unexported engine helpers
  directly.
- SOW-0046 fixed the separate direct `&Engine{}` construction regression and
  introduced a fixture guard.
- This SOW is intentionally separate so behavior-boundary decisions are not
  hidden inside constructor cleanup.

### Acceptance criteria

- Inventory same-package unexported engine-helper tests and classify each as:
  behavior-testable through stable engine/API/artifact behavior, intentionally
  white-box because the helper is an internal algorithm with valuable narrow
  failure localization, or removable duplication.
- Convert the behavior-testable class to stable fixture-backed behavior tests.
- Keep intentional white-box tests only when the SOW records why black-box
  testing would be weaker, slower, or less precise.
- Add a regression guard only if a clear, low-noise rule exists. Do not add a
  broad "no unexported calls" gate if it would ban useful package-level tests.
- Validation includes `go test ./pkg/engine`, `make test`, `make test-strict`,
  `make race`, and blocking analysis gates.

## Analysis

- Source SOWs: SOW-0039 and SOW-0046.
- Known evidence from SOW-0039 A10 includes direct calls to helpers such as
  metadata builders, feed body path helpers, comparison writers, pipeline plan
  builders, insights series readers, and bounded job runners.
- Fresh inventory after SOW-0046:
  - 129 unexported production helper names called from engine tests.
  - 280 total helper calls from `pkg/engine/*_test.go`, excluding the new
    engine fixture guard file.
- Classification:
  - Behavior-testable through existing exported engine APIs:
    provider default list ordering, per-feed metadata conversion,
    public feed summaries, and redistributability decisions.
  - Intentionally white-box for narrow algorithm behavior:
    runtime ledger CSV parsing, run-plan branching, bounded job cancellation,
    fan-out target selection, merge body composition, staged publish batch
    behavior, and filesystem path contracts.
  - Artifact writer tests that directly call writer helpers remain white-box
    because the failure condition often concerns writer return values,
    cancellation, staged temp files, or mtime behavior that public route tests
    would only observe after much larger setup.
- A broad "no unexported calls" regression guard would be noisy today. It
  would ban useful package-level algorithm tests and force either overbroad
  public APIs or slow end-to-end tests. This SOW therefore avoids a broad
  allowlist gate and instead removes the behavior-testable calls where stable
  exported APIs already exist.

## Plan

1. Build the inventory from current engine tests.
2. Classify every helper-call cluster by observable contract.
3. Migrate high-value behavior-testable clusters first.
4. Preserve intentional algorithm-level tests with explicit rationale.
5. Run full Go validation and update project testing guidance if new rules are
   learned.

## Execution log

- Replaced direct preferred-provider helper assertions with exported
  `ASNProviders()` and `GeoProviders()` behavior.
- Replaced direct metadata builder coverage with exported `Metadata()`.
- Replaced direct public summary builder coverage with exported
  `PublicFeedSummaries()`.
- Replaced direct redistributability helper assertions with exported
  `IsRedistributable()`.
- Removed the now-unused private `buildSetMetadata` production wrapper after
  Staticcheck proved it became dead code.
- Post-migration inventory:
  - 123 unexported production helper names called from engine tests.
  - 269 total helper calls.
  - Removed all calls to `preferredASNProvider`, `preferredGeoProvider`,
    `buildSetMetadata`, `buildPublicFeedSummary`, `isRedistributable`, and
    `buildSetMetadataFromEffectiveEntryInDir`.

## Validation

- `go test ./pkg/engine` - passed.
- `make test` - passed.
- `make test-tools` - passed.
- `make test-strict` - passed after the final production cleanup.
- `make fuzz-replay` - passed.
- `make lint` - passed.
- `make staticcheck` - initially failed on unused
  `(*Engine).buildSetMetadata`; passed after removing the dead wrapper.
- `make golangci-lint` - passed.
- `make vulncheck` - passed.
- `make race` - passed.
- `git diff --check` - passed.
- Product specs: not updated. Reason: this SOW changes tests and removes a
  dead private wrapper only; no runtime behavior, file layout, API, UI,
  integrity, or operator contract changed.
- Project skills: `project-testing` updated with the behavior-boundary rule
  for same-package helper calls.

## Outcome

Completed.

Shipped changes:

- Migrated behavior-testable engine tests from private helpers to exported
  engine behavior where the exported API already existed.
- Preserved intentionally white-box tests for internal algorithms and writer
  contracts where public-route testing would be broader, slower, or less
  precise.
- Avoided adding a noisy broad "no unexported calls" gate.
- Removed one dead private production wrapper found by Staticcheck.

## Lessons extracted

- Do not treat all same-package tests as a smell. In Go, package-level tests
  are useful for internal algorithms, cancellation paths, and writer contracts.
- The smell is narrower: calling private helpers when a stable exported
  behavior already expresses the same contract.
- Staticcheck is a useful backstop after moving tests away from private helper
  wrappers; private wrappers kept alive only by tests should be removed when
  the exported behavior path replaces them.
