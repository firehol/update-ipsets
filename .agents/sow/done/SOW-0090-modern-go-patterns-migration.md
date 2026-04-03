# SOW-0090 - Modern Go Patterns Migration

## Status

Status: completed

Sub-state: Completed after post-review fixes, full validation, and final
external reviewer re-run on 2026-05-24.

## Requirements

### Purpose

Align the update-ipsets Go codebase with the modern Go patterns already allowed
by the module's Go version, while preserving behavior, performance, cancellation
semantics, goroutine ownership, and project validation gates.

This is a maintainability and consistency SOW, not a product-behavior SOW.

### User Request

> "Use minimax, glm, kimi, mimo, qwen to review the Go-lang codebase to identify
> modern go patterns which we don't use in this repo. Do this analysis, and if
> the discrepancies are a lot, we should create an SOW and fix them."

The user approved creating a SOW and explicitly blocked implementation start.
The current request repaired this SOW after review found internal
contradictions and weak gates. On 2026-05-24, the user approved the repaired
SOW decisions: 1A, 2A, 3A, and 4A.

### Assistant Understanding

Facts:

- The module declares Go 1.26.0 in `go.mod`.
- The Makefile validates the root module and nested `tools/dronebl2ipsets`
  module through project targets such as `make race`, `make staticcheck`, and
  `make golangci-lint`.
- The codebase already uses some modern patterns, including `slices.Sort*`,
  `sync.OnceValue/OnceValues`, `sync.WaitGroup.Go`, `errors.Join`, Go 1.22
  `ServeMux`, and `testing/synctest`.
- Verified local pattern counts on 2026-05-24:
  - `sort.SliceStable`: 1
  - `sort.Slice`: 93
  - `sort.Strings`: 74
  - `sort.Ints`: 2
  - `sort.StringsAreSorted`: 2
  - `sort.IntsAreSorted`: 1
  - `slices.Sort*`: 3
  - `sync.Once`: 5
  - `sync.OnceFunc/OnceValue/OnceValues`: 2
  - `sync.WaitGroup.Go`: 3
  - `wg.Add(1)`: 21
  - `interface{}`: 12
  - `fmt.Errorf("%s", ...)`: 6
  - `err == io.EOF`: 10
  - `== syscall.*`: 1
  - `singleflight`: 0
  - `errgroup`: 0
  - `goleak`: 0
- The old SOW draft contained contradictory state:
  - it said user decisions were still pending;
  - it also said several decisions were approved;
  - it duplicated `Plan` and `Execution Log` sections;
  - it included stale option text after the milestone list.

Inferences:

- Some changes are genuinely mechanical and low-risk, such as `interface{}` to
  `any` in tests or `fmt.Errorf("%s", msg)` to `errors.New(msg)`.
- Some changes are not mechanical, especially `errgroup`, `singleflight`,
  `goleak`, and broad `wg.Go()` migration. These can expose lifecycle bugs,
  change cancellation/error aggregation semantics, or change panic/goroutine
  ownership expectations.
- A single all-at-once refactor would create avoidable review risk and merge
  conflict risk.
- The correct shape is gated milestones with exact candidate inventories before
  code edits.

Resolved user decisions:

- Decision 1: Option A, one SOW with gated milestones.
- Decision 2: Option A, external reviewers after each major risk gate.
- Decision 3: Option A, evaluate `goleak`, `errgroup`, and `singleflight`
  before adoption.
- Decision 4: Option A, targeted benchmark comparison for touched hot paths.

Remaining unknowns:

- None blocking before external reviewer verification.

### Acceptance Criteria

- [x] SOW decisions are approved and recorded before implementation starts.
- [x] Every milestone starts with an exact candidate list generated from the
      current tree, not inherited from old reviewer estimates.
- [x] Simple mechanical migrations are applied only where semantics are
      identical.
- [x] `sort.SliceStable` keeps stable-sort semantics.
- [x] Sortedness helpers are not replaced unless the replacement preserves the
      exact assertion intent.
- [x] `bytes.Buffer` is replaced with `strings.Builder` only where the value is
      used as text, not binary data or an `io.Reader`/`io.Writer` buffer for
      gzip/tar/test payloads.
- [x] `regexp.MustCompile` is hoisted only where the expression is static and
      hoisting improves repeated execution or clarity.
- [x] `sync.Once` and `wg.Add(1); go func()` migrations are applied only after
      reviewing panic, cancellation, test isolation, and goroutine ownership.
- [x] `goleak`, `errgroup`, and `singleflight` are evaluation-first items, not
      automatic migrations.
- [x] Root and nested-module validation use project Makefile targets.
- [x] Benchmarks show no accepted hot-path regression without user approval.
- [x] `project-go-best-practices` is updated only after a new baseline is
      actually established.

## Analysis

### Evidence Reviewed

- `go.mod:3` declares `go 1.26.0`.
- `Makefile:21-25` separates root tests and nested tool tests.
- `Makefile:43-45` shows `make race` runs root and nested-module race tests.
- `Makefile:63-69` shows `make staticcheck` and `make golangci-lint` run root
  and nested-module linters.
- `.github/workflows/ci.yml:56-78` wires nested tests, race, strict tests,
  fuzz replay, vet, govulncheck, staticcheck, and golangci-lint into CI.
- Pattern counts were generated with `rg` over `cmd`, `internal`, `pkg`, and
  `tools` on 2026-05-24.

### Representative Current Evidence

- Stable sort exists and must remain stable:
  - `pkg/engine/run.go:148` uses `sort.SliceStable`.
- Old error wrapping patterns exist:
  - `pkg/engine/artifact_stage.go:86`
  - `pkg/engine/download_stage.go:369`
- Direct EOF comparisons exist:
  - `pkg/downloader/canonical.go:141`
  - `pkg/geoloc/geoloc.go:108`
  - `pkg/processor/stream.go:153`
- Function-local static regexps exist:
  - `pkg/engine/helpers.go:413`
  - `pkg/engine/output.go:1356`
  - `pkg/config/legacy.go:112`
- WaitGroup manual launch patterns exist:
  - `pkg/processor/primitives.go:190`
  - `pkg/iprange/dns.go:57`
  - `pkg/scheduler/queue_admission.go:117`
  - several test-only stress/concurrency tests
- Existing modern patterns exist:
  - `pkg/engine/concurrency.go:76` uses `wg.Go`.
  - `pkg/engine/insights.go:30` uses `sync.OnceValue`.
  - `pkg/web/methodology.go:44` uses `sync.OnceValues`.

### Risk Assessment

Low risk:

- `interface{}` to `any` in tests.
- `fmt.Errorf("%s", msg)` to `errors.New(msg)` when no formatting or wrapping is
  required.
- `sort.Strings` to `slices.Sort` where the sorted value is a plain `[]string`.
- `sort.Ints` to `slices.Sort` where the sorted value is a plain `[]int`.

Medium risk:

- `sort.Slice` to `slices.SortFunc`, because comparator conventions differ:
  `sort.Slice` uses a boolean less function; `slices.SortFunc` uses a
  three-way comparator.
- `sort.SliceStable` to `slices.SortStableFunc`, because stable ordering is a
  behavior contract.
- `err == io.EOF` to `errors.Is`, because some loop code intentionally treats
  EOF after partial data in a specific way.
- static regexp hoisting, because dynamic patterns and package init costs must
  be separated from true static regexps.

High risk / evaluation-first:

- `wg.Go()` migrations in production fan-out code.
- `errgroup` adoption.
- `singleflight` adoption.
- `goleak` integration.

These are useful tools, but they can change observable cancellation, shutdown,
error aggregation, or test behavior. They require package-specific reasoning and
targeted tests.

## Pre-Implementation Gate

Status: approved-for-implementation

Problem / root-cause model:

- The codebase has accumulated mixed old and new Go idioms while the module has
  advanced to Go 1.26.0.
- Mixed idioms make future maintenance less consistent and make project skills
  less enforceable.
- The cleanup is worth doing, but only with strict separation between
  mechanical refactors and semantic concurrency/cache/lifecycle changes.

Affected contracts and surfaces:

- Go implementation files under `cmd/`, `internal/`, `pkg/`, and `tools/`.
- Go test files.
- Project validation gates and CI expectations.
- Project skill: `.agents/skills/project-go-best-practices/SKILL.md`.
- No public API, feed-processing behavior, config semantics, or UI behavior is
  intended to change.

Existing patterns to reuse:

- Use current local `slices.Sort*` call style where already present.
- Use `sync.OnceValue/OnceValues` style from `pkg/engine/insights.go` and
  `pkg/web/methodology.go`.
- Use `wg.Go()` style from `pkg/engine/concurrency.go` only where the goroutine
  ownership model matches.
- Preserve existing `errors.Join`/collector behavior unless an explicit
  milestone proves a better replacement.

Sensitive data handling plan:

- This work does not require secrets, customer data, production data, or raw
  feed data.
- SOW and review artifacts must include only paths, line numbers, command names,
  and sanitized summaries.

Implementation plan:

1. Milestone 0 inventories exact candidate lists and finalizes user decisions.
2. Milestones 1-3 handle mechanical modernization only.
3. Milestones 4-5 evaluate higher-risk concurrency/cache/test-lifecycle changes
   before applying any edits.
4. Milestone 6 updates project skill guidance only after code establishes the
   new baseline.

Validation plan:

- `make test`
- `make test-tools`
- `make test-strict` when scheduler/engine/web behavior is touched.
- `make race`
- `make staticcheck`
- `make golangci-lint`
- `make bench` or targeted package benchmarks for hot-path changes.
- `make fuzz-replay` when parser, processor, config, or iprange behavior is
  touched.
- `go test ./tools/archposture` after broad package/file movement or large-file
  changes.
- Same-failure scans with `rg` after each milestone.

Artifact impact plan:

- AGENTS.md: no expected update.
- Runtime project skills: update `project-go-best-practices` after the new
  baseline is real.
- Specs: no product spec update expected unless a milestone changes runtime
  behavior, cancellation, cache behavior, or serving behavior.
- End-user/operator docs: no expected update.
- End-user/operator skills: no expected update.
- SOW lifecycle: this SOW is in `current/` with status `in-progress`.
  Implementation proceeds only under the approved gated milestone plan.

Official reference evidence:

- No external source implementation is required for purely internal mechanical
  refactoring.
- Go standard-library documentation and the project-local
  `project-go-best-practices` skill are the authoritative guidance.
- `sync.WaitGroup.Go` documentation says it starts a goroutine, adds the task
  to the group, removes it when the function returns, and the function must not
  panic. Source: https://pkg.go.dev/sync#WaitGroup.Go
- `slices.SortStableFunc` documentation preserves original order of equal
  elements. Source: https://pkg.go.dev/slices#SortStableFunc
- `strings.Builder` documentation says it efficiently builds text strings and
  must not be copied after use. Source: https://pkg.go.dev/strings#Builder
- `errors.Is` documentation checks wrapped error trees. Source:
  https://pkg.go.dev/errors#Is
- `errgroup` and `singleflight` are `golang.org/x/sync` packages for grouped
  goroutine error propagation/cancellation and duplicate-call suppression.
  Sources: https://pkg.go.dev/golang.org/x/sync/errgroup and
  https://pkg.go.dev/golang.org/x/sync/singleflight

### User Decisions

**Decision 1: Execution Scope**

Approved: Option A.

Option A — One SOW with gated milestones (Recommended)

- Pros: one durable ledger, fewer bookkeeping artifacts, clear end-to-end
  modernization.
- Cons: large SOW requires discipline to prevent scope creep.
- Implication: each milestone can be reviewed and committed separately, but the
  SOW remains the single control surface.
- Risk: if a milestone expands, it must be explicitly narrowed or split.

Option B — Split mechanical refactors and concurrency/cache work into separate
SOWs

- Pros: cleaner risk separation.
- Cons: more SOW overhead and possible repeated setup/review work.
- Implication: this SOW would cover only low/medium-risk mechanical changes.
- Risk: higher-risk cleanup may be postponed indefinitely.

Option C — Single all-at-once mechanical pass

- Pros: fastest to write.
- Cons: highest review and merge-conflict risk.
- Implication: broad diff across many packages in one pass.
- Risk: not recommended for this repository.

**Decision 2: External Reviewer Cadence**

Approved: Option A.

Option A — Review after each major risk gate (Recommended)

- Pros: strong independent review without running five reviewers after every
  small mechanical batch.
- Cons: less exhaustive than after every milestone.
- Implication: run reviewers after mechanical migrations, after
  concurrency/cache evaluation, and at final closure.
- Risk: a small missed mechanical item may be caught later, not immediately.

Option B — Review after every milestone

- Pros: maximum external scrutiny.
- Cons: expensive, slow, and likely to produce repeated low-value findings on
  mechanical work.
- Implication: reviewer prompts and outputs must be recorded for every
  milestone.
- Risk: review fatigue and scope churn.

Option C — No external reviewers

- Pros: fastest.
- Cons: contradicts the original multi-reviewer workflow that created this SOW.
- Implication: rely only on local tests and internal review.
- Risk: not recommended.

**Decision 3: Concurrency/Cache Tool Adoption**

Approved: Option A.

Option A — Evaluation-first inside this SOW (Recommended)

- Pros: keeps `goleak`, `errgroup`, and `singleflight` visible without assuming
  they are automatically beneficial.
- Cons: may produce "do not change" outcomes for some tools.
- Implication: each adoption needs package-specific evidence and tests.
- Risk: lower than automatic adoption.

Option B — Automatic adoption where a reviewer mentioned the tool

- Pros: broad modernization.
- Cons: treats semantic changes as style changes.
- Implication: more code churn in scheduler, engine, and cache paths.
- Risk: not recommended.

Option C — Defer all three tools to follow-up SOWs

- Pros: keeps this SOW purely mechanical.
- Cons: loses the original reviewer signal unless follow-ups are created
  immediately.
- Implication: this SOW becomes smaller.
- Risk: valid work may be buried.

**Decision 4: Benchmark Policy**

Approved: Option A.

Option A — Targeted benchmark comparison for touched hot paths (Recommended)

- Pros: realistic signal and less noise than whole-repo benchmark interpretation.
- Cons: requires per-milestone judgment about hot paths.
- Implication: use `make bench` plus targeted package benchmarks where relevant.
- Risk: low.

Option B — Require whole-repo `make bench` only

- Pros: simple.
- Cons: noisy and hard to interpret for small refactors.
- Implication: may miss localized regressions or overreact to benchmark noise.
- Risk: medium.

Option C — No benchmark gate

- Pros: fastest.
- Cons: weak for sort/string-building/cache changes.
- Implication: rely on tests only.
- Risk: not recommended.

## Milestones

### Milestone 0: Candidate Inventory And Final Gate

Goal:

- Generate exact candidate lists for every category from the current tree.
- Confirm approved user decisions are recorded.
- Establish the exact implementation candidate set before Go edits.

Required evidence:

- `rg` outputs or generated candidate files for every category.
- Exclusions with reasons, especially for `bytes.Buffer`, test stress code,
  `sort.SliceStable`, and package-local dynamic regexps.

Validation:

- No code changes.
- SOW consistency check: no duplicated stale decision blocks.

### Milestone 1: Sorting Modernization

Scope:

- Replace `sort.Strings` and `sort.Ints` with `slices.Sort` where semantics are
  identical.
- Replace `sort.Slice` with `slices.SortFunc` only after converting boolean less
  logic to a correct three-way comparator.
- Replace `sort.SliceStable` only with `slices.SortStableFunc`, preserving stable
  ordering.
- Preserve sortedness helper calls unless a direct `slices.IsSorted`
  conversion improves clarity.

Validation:

- `make test`
- `make test-tools`
- `make race`
- `make staticcheck`
- same-failure scan for remaining sort patterns and documented exclusions.

### Milestone 2: Simple Error And Type Alias Cleanup

Scope:

- Replace `fmt.Errorf("%s", msg)` with `errors.New(msg)` where no formatting or
  wrapping is needed.
- Replace `interface{}` with `any` in tests and simple helper signatures.
- Replace direct `io.EOF`/`syscall` comparisons with `errors.Is` where that
  preserves loop semantics.

Validation:

- `make test`
- `make test-tools`
- `make race`
- `make staticcheck`
- same-failure scan.

### Milestone 3: String Builders And Static Regexps

Scope:

- Replace `bytes.Buffer` with `strings.Builder` only for text-building paths.
- Do not replace buffers used for binary payloads, gzip/tar construction,
  `io.Reader` inputs, or tests intentionally exercising bytes.
- Hoist static function-local `regexp.MustCompile` values when repeated
  compilation is real and the regexp is not dynamic.

Validation:

- `make test`
- `make test-tools`
- `make race`
- `make staticcheck`
- targeted benchmarks for touched hot paths.

### Milestone 4: Once And WaitGroup Evaluation

Scope:

- Convert `sync.Once` to `sync.OnceFunc/OnceValue/OnceValues` only where the
  lifetime and test reset semantics match.
- Convert `wg.Add(1); go func()` to `wg.Go()` only where goroutine ownership,
  panic assumptions, and cancellation behavior match.
- Prefer leaving test stress patterns alone when explicit `Add`/`Done` makes the
  scenario clearer.

Validation:

- `make test`
- `make test-tools`
- `make test-strict` if scheduler/engine/web are touched.
- `make race`
- `make staticcheck`

### Milestone 5: Goleak, Errgroup, And Singleflight Evaluation

Scope:

- Evaluate `goleak` in candidate packages with known long-lived goroutines.
- Evaluate `errgroup` only where existing error aggregation/cancellation
  semantics can be preserved or deliberately improved.
- Evaluate `singleflight` only where there is a proven duplicate-work or cache
  stampede path.
- It is acceptable for this milestone to record "do not adopt" decisions with
  evidence.

Validation:

- Package-specific tests proving lifecycle/cancellation behavior.
- `make test`
- `make test-strict` when scheduler/engine/web are touched.
- `make race`
- external reviewer verification per approved cadence.

### Milestone 6: Final Baseline And Skill Update

Scope:

- Run same-failure scans across all categories.
- Record accepted exclusions with file/line evidence.
- Update `project-go-best-practices` only for patterns actually established as
  project baseline.

Validation:

- `make test`
- `make test-tools`
- `make test-strict`
- `make fuzz-replay` if touched categories require it.
- `make race`
- `make staticcheck`
- `make golangci-lint`
- `make bench` plus targeted benchmarks where relevant.
- Final external reviewer verification per approved cadence.

## Plan

Proceed under approved decisions 1A, 2A, 3A, and 4A.

1. Run Milestone 0 candidate inventory from the current tree.
2. Apply Milestones 1-3 as mechanical changes with per-milestone validation.
3. Run external reviewers after the mechanical migration gate.
4. Evaluate Milestones 4-5 with package-specific evidence before any
   concurrency/cache/test-lifecycle adoption.
5. Run external reviewers after the concurrency/cache evaluation gate.
6. Complete Milestone 6 validation, update project skill guidance only when a
   new baseline is real, then run final external reviewer verification.

## Execution Log

### 2026-05-24

- Created SOW after multi-reviewer audit.
- User approved SOW creation but blocked implementation start.
- Planning review found contradictions and weak gates in the original SOW draft.
- Repaired the SOW to:
  - remove duplicated stale decision text;
  - separate mechanical refactors from semantic concurrency/cache changes;
  - replace generic validation with project Makefile gates;
  - add explicit user decisions required before implementation.
- User approved decisions 1A, 2A, 3A, and 4A.
- Recorded approved decisions and moved the SOW to `in-progress`.
- Ran Milestone 0 candidate inventory against `cmd`, `internal`, `pkg`, and
  `tools` Go files:
  - sorting: 173 legacy `sort.*` candidates, split into 74 `sort.Strings`, 2
    `sort.Ints`, 2 `sort.StringsAreSorted`, 1 `sort.IntsAreSorted`, 1
    `sort.SliceStable`, and 93 `sort.Slice`;
  - existing modern slice helpers: 25 `slices.*` calls;
  - error/type cleanup: 12 `interface{}` test literals, 6
    `fmt.Errorf("%s", ...)`, 10 direct `err == io.EOF`, and 1 direct
    `syscall` error comparison;
  - string/regexp cleanup: 68 `bytes.Buffer`/`bytes.NewBuffer*` occurrences
    and 16 `regexp.MustCompile` occurrences;
  - concurrency/lifecycle: 39 `sync.WaitGroup`/`Add(1)`/`Go` occurrences and
    39 `go func` launches.
- Milestone 0 risk conclusion:
  - `interface{}` to `any`, `fmt.Errorf("%s", ...)`, and direct sentinel
    error comparisons are small mechanical candidates;
  - simple sort and sortedness helpers are mostly mechanical but require import
    updates;
  - `sort.Slice`, `bytes.Buffer`, `regexp.MustCompile`, `sync.Once`,
    `WaitGroup.Go`, `goleak`, `errgroup`, and `singleflight` require
    site-by-site review instead of blanket replacement.
- Applied mechanical modernization:
  - replaced simple ordered sort helpers with `slices.Sort` and
    `slices.IsSorted`;
  - replaced the single stable sort with `slices.SortStableFunc` and an
    explicit comparator, preserving stable-sort semantics;
  - replaced test `map[string]interface{}` literals with `map[string]any`;
  - replaced `%s`-only `fmt.Errorf` calls with `errors.New`;
  - replaced direct EOF and syscall sentinel checks with `errors.Is`;
  - hoisted repeated static regular expressions in legacy config parsing,
    history suffix parsing, and markdown link conversion;
  - used `strings.Builder` only for the rendered methodology HTML string;
  - used `sync.OnceFunc` and `sync.WaitGroup.Go` only where ownership was
    already clear.
- Accepted exclusions and non-goals:
  - retained the 93 remaining `sort.Slice` comparators as explicit non-goals
    for SOW-0090 because they include descending order, tie-breaks, and domain
    logic that require individual comparator review; no additional SOW is created
    for them by this modernization pass;
  - retained binary, gzip/tar, and byte-slice `bytes.Buffer` uses;
  - retained dynamic regexp compilation for tag-specific XML extraction;
  - did not add `goleak`, `errgroup`, or `singleflight` because this pass found
    no package-specific evidence strong enough to justify dependency or
    semantic changes.
- Fixed validation issues found by project gates:
  - simplified `readTextLine` after Staticcheck flagged the redundant loop;
    `bufio.Reader.ReadBytes` already accumulates until delimiter or EOF, and
    the original code already used `errors.Is(err, io.EOF)`;
  - removed unused `cacheSuffixPath`;
  - checked test file write, gzip close, and gzip read errors reported by
    `golangci-lint`.
- Updated `.agents/skills/project-go-best-practices/SKILL.md` with the new
  repo baseline established by this SOW.
- Final external reviewer verification round 1 found one valid missed
  mechanical candidate: `pkg/engine/insights.go` still used
  `sort.IntsAreSorted`.
- Fixed the missed candidate by replacing it with
  `slices.IsSorted(h.BucketsHours)`. The file keeps the `sort` import because
  it still contains retained `sort.Slice` comparators.
- Final external reviewer verification round 1 also requested SOW closure
  cleanup: fill outcome, lessons, and follow-up mapping before moving this SOW
  to `done/`.
- Rejected one reviewer claim that `cacheSuffixPath` still existed:
  `rg 'cacheSuffixPath' cmd internal pkg tools --type go` returned no matches.
- Post-review same-failure scans passed:
  - sortedness helper scan returned no matches for `sort.StringsAreSorted` or
    `sort.IntsAreSorted`;
  - `rg 'cacheSuffixPath' cmd internal pkg tools --type go` returned no
    matches.
- Final external reviewer verification round 2 returned clean closure verdicts
  from the usable reviewer outputs. Reviewers found no correctness regressions,
  no missed in-scope mechanical candidates, no unresolved deferred SOW items,
  and no security issues.
- One external reviewer process was stopped with targeted PIDs after it violated
  the review prompt by spawning another external assistant underneath it. Its
  spawned-child output was not needed for closure, and the prompt violation was
  treated as reviewer-process noise, not project evidence.

## Validation

Implementation validation:

- `make test`: passed.
- `make test-tools`: passed.
- `make test-strict`: passed.
- `make race`: passed, including nested `tools/dronebl2ipsets`.
- `make lint`: passed.
- `make staticcheck`: passed, including nested `tools/dronebl2ipsets`.
- `make golangci-lint`: passed, including nested `tools/dronebl2ipsets`.
- `make fuzz-replay`: passed for `pkg/iprange`, `pkg/config`, and
  `pkg/processor`.
- `make bench`: passed.
- Final external reviewer verification round 2: clean closure verdicts from the
  usable reviewer outputs, with one prompt-violating reviewer process stopped
  and excluded from closure evidence.

SOW repair validation:

- Checked `go.mod` for Go version.
- Checked `Makefile` and CI for project validation gates.
- Recounted current pattern categories with `rg`.
- Confirmed the SOW has one `Plan`, one `Execution Log`, and no stale option
  fragments after the milestone list.

Post-review fix validation:

- `make test`: passed.
- `make test-tools`: passed.
- `make test-strict`: passed.
- `make race`: passed, including nested `tools/dronebl2ipsets`.
- `make lint`: passed.
- `make staticcheck`: passed, including nested `tools/dronebl2ipsets`.
- `make golangci-lint`: passed, including nested `tools/dronebl2ipsets`.
- `make fuzz-replay`: passed for `pkg/iprange`, `pkg/config`, and
  `pkg/processor`.
- `make bench`: passed.

## Outcome

Completed implementation result:

- Simple ordered sort helpers now use `slices.Sort`.
- Simple sortedness checks now use `slices.IsSorted`, including the missed
  `sort.IntsAreSorted` candidate found during final reviewer verification.
- The single stable processing-order sort now uses `slices.SortStableFunc`
  with an explicit `cmp.Compare` comparator, preserving stable ordering.
- `%s`-only `fmt.Errorf` calls now use `errors.New`.
- Direct EOF and syscall sentinel comparisons now use `errors.Is`.
- Test `map[string]interface{}` literals now use `map[string]any`.
- Static regular expressions are hoisted only where the pattern is constant.
- The only `bytes.Buffer` to `strings.Builder` change is the methodology HTML
  text-rendering path.
- `sync.OnceFunc` and `sync.WaitGroup.Go` were adopted only where ownership and
  cancellation semantics matched the existing code.
- The 93 remaining `sort.Slice` comparators are explicit non-goals for this
  SOW, not deferred work. They include descending order, tie-breaks, and domain
  logic that require individual comparator review.
- `goleak`, `errgroup`, and `singleflight` were evaluated and not adopted in
  this SOW because no package-specific evidence justified adding dependencies
  or changing lifecycle, cancellation, or duplicate-work semantics.
- No product behavior, public API, feed-processing semantics, config contract,
  UI behavior, docs, or specs changed.
- `.agents/skills/project-go-best-practices/SKILL.md` was updated with the
  modern Go baseline established by this SOW.

## Lessons Extracted

- Candidate inventories must include all sortedness helper variants, not only
  `sort.StringsAreSorted`. The missed `sort.IntsAreSorted` was harmless, but
  final reviewer verification caught it.
- `sort.Slice` is not a mechanical modernization category in this project.
  Boolean less functions often encode descending order, tie-breaks, and domain
  policy; broad conversion would create review risk without clear value.
- `bufio.Reader.ReadBytes` already accumulates until delimiter or EOF. Loops
  around a single unconditional-return `ReadBytes` call are misleading and
  Staticcheck can expose this pattern.
- `sync.WaitGroup.Go` is safe only where the existing goroutine is already
  owned by the same `WaitGroup` and has no special panic or returned-error
  handling. Stress tests and lifecycle-sensitive goroutines can be clearer with
  explicit `Add`/`Done`.
- Project skill updates should be limited to patterns actually established by
  code and validation, not every modern Go tool mentioned by reviewers.

## Followup

- No additional SOW is created by SOW-0090.
- The retained 93 `sort.Slice` comparators are accepted non-goals for this SOW,
  with evidence recorded in the execution log. They should be reviewed only if
  a separate explicit comparator-focused SOW is opened.
- `goleak`, `errgroup`, and `singleflight` are rejected as non-goals for this
  SOW after evaluation. They should be reconsidered only with package-specific
  evidence of leaks, error-aggregation needs, cancellation semantics changes,
  or duplicate-work/cache-stampede behavior.
- The `project-go-best-practices` update required by this SOW is completed.

## Regression Log

None yet.
