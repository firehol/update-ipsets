# SOW-0032 | 2026-04-30 | go-test-gap-analysis

## Status

completed

## Requirements

### Purpose

Improve the Go test posture using the existing gap analysis as evidence:
remove justified brittle-test patterns, add high-value behavioral coverage for
load-bearing contracts, and record evidence-backed non-goals instead of quietly
dismissing them.

### User request summary

- Work autonomously on the Go testing quality SOW after the code-design
  leftover SOWs are complete.
- Maintain black-box/behavioral testing discipline: tests should verify public
  contracts, observable side effects, errors, and generated outputs.
- Do not batch SOWs; complete this SOW before switching to frontend quality or
  frontend test work.

### Acceptance criteria

- Justified test anti-patterns from the analysis are fixed.
- New tests protect concrete parser, config, set-algebra, queue, and web
  contracts without adding unneeded dependencies.
- Nested Go modules are covered by a named Makefile/CI gate.
- All changed tests pass through targeted tests plus root project gates.
- Any analysis item not implemented in this SOW is recorded as an evidence-backed
  non-goal, not an untracked promise.

## Analysis

Skills loaded:

- `go-behavioral-testing`
- `project-testing`
- `project-coding`
- `project-reviewing`
- `sow`

Current-state evidence used for implementation:

- `cmd/`, `pkg/`, and `tools/` had no remaining `time.Sleep`,
  `os.Setenv`/`os.Unsetenv`, `reflect.DeepEqual`, or legacy benchmark
  `for i := 0; i < b.N; i++` loops after the hygiene pass.
- The nested `tools/dronebl2ipsets` module has its own `go.mod`; root
  `go test ./...` does not enter nested modules, so it needed an explicit gate.
- `pkg/iprange` is the core set-algebra package and must stay standalone.
  Existing table tests were good, but multi-range algebra properties were thin.
- `pkg/config` loads operator-supplied YAML and should fail cleanly on malformed
  input.
- `pkg/processor` parses arbitrary upstream feed payloads; deterministic parser
  steps are good fuzz targets.
- `pkg/web/integrity_test.go` still had a brittle entity-rebuild test shape:
  it asserted that a rebuild was scheduled, then could return before the
  background rebuild had created observable output.

Evidence-backed non-goals for this SOW:

- Full migration to external `<pkg>_test` packages is not part of this SOW.
  Evidence: every test file in the repo currently uses same-package style, and
  the engine tests build many private fixtures. A safe migration requires a
  package-boundary and fixture API design, not a mechanical testing hygiene
  patch.
- New third-party testing dependencies are not part of this SOW. The added
  coverage uses stdlib `testing/quick` and fuzz targets, preserving the
  standalone expectation for `pkg/iprange` and avoiding new dependency review
  work.
- A broad golden-file framework is not part of this SOW. No current failing
  contract required snapshot-style artifacts, and adding goldens without a
  specific artifact contract risks turning review into blind snapshot refresh.
- `testing/synctest` is not part of this SOW. The concrete wall-clock sleeps
  were removable through observable-state waits. The remaining candidate tests
  involve `httptest`/network or engine goroutines where direct `synctest`
  bubbles are not a clean fit.
- `go.uber.org/goleak` is not part of this SOW. The concrete issue found was a
  test that returned before scheduled output existed; that is now asserted
  directly. No package-level leak failure was reproduced during `make race`.

## Implementation

- Added `make test-tools` and wired it into GitHub Actions so the nested
  `tools/dronebl2ipsets` module is tested.
- Added `make test-strict` for shuffled, repeated scheduler/engine/web tests:
  `go test -shuffle=on -count=3 ./pkg/scheduler ./pkg/engine ./pkg/web`.
- Modernized benchmark loops to `b.Loop()` where benchmarks were touched.
- Replaced raw environment mutation in systemd tests with `t.Setenv`.
- Replaced `reflect.DeepEqual` test assertions with typed `slices`/`maps`
  helpers where applicable.
- Replaced log/body substring checks with structured state or parsed JSON checks
  where applicable.
- Replaced sleep-based waits with ticker/timer helpers that wait on observable
  state.
- Added stdlib property coverage in `pkg/iprange/set_properties_test.go` for:
  union idempotence, union commutativity, intersection commutativity, exclude
  idempotence, exclude/intersection partitioning, and pointwise membership over
  generated multi-range sets.
- Added `FuzzLoadYAML` for config YAML loading.
- Added `FuzzRunDeterministicTextProcessors` for deterministic, non-network
  processor steps.
- Hardened the admin entity-rebuild test to remove a known entity artifact,
  trigger the admin rebuild endpoint, and wait for the rebuilt artifact plus an
  empty background-task list.

## Validation

Passed:

- `rg -n "reflect\\.DeepEqual|\\\"reflect\\\"|time\\.Sleep|os\\.Setenv|os\\.Unsetenv|for .*:= 0; .* < b\\.N; .*\\+\\+" cmd pkg tools --glob '*_test.go'`
  returned no matches.
- `go test ./cmd/update-ipsets ./pkg/systemd ./pkg/engine ./pkg/web ./pkg/scheduler ./pkg/iprange ./pkg/processor ./pkg/config`
- `make test-tools`
- `make test-strict`
- `make test`
- `make race`
- `make lint`
- `make build`
- `go test ./tools/archposture`
- `git diff --check`

## Outcome

SOW-0032 is complete.

The Go test suite is now less brittle in the concrete areas found by the gap
analysis, includes broader behavioral/property/fuzz coverage for critical
parsers and set algebra, and has explicit gates for the nested tool module and
shuffled repeated scheduler/engine/web tests.

## Lessons extracted

- `project-testing` now records the strict and nested-tool gates, plus the
  testing rules for condition-based waits, `t.Setenv`, structured assertions,
  `b.Loop`, and stdlib fuzz/property coverage.
- `project-reviewing` now records review checks for sleep-based test
  synchronization, nested modules, benchmark loop shape, and background work
  tests that must wait for observable output instead of only "scheduled".
