# SOW-0030 | 2026-04-30 | code-quality-refactor-phases

## Status

completed

## Requirements

### Purpose

Plan a phased code-quality improvement program for update-ipsets that improves
separation of concerns, clean-code posture, maintainability, and regression
resistance without destabilizing the working feed pipeline.

### User request quoted verbatim

> Can you please come up with a plan on how to improve it in phases, and
> evaluate the codebase posture/improvement after each phase. Give me a summary.

### Assistant understanding

- This is a planning request, not approval to refactor code now.
- The plan must be grounded in SOW-0029 evidence, not generic clean-code advice.
- Each phase should have a clear scope, measurable success criteria, expected
  posture improvement, and risk level.
- The plan should preserve the current operational strengths: tests, integrity
  checks, cache-first public serving, config-driven semantics, and installability.

### Acceptance criteria

- Identify phased improvements in dependency/order form.
- For each phase, state scope, risk, validation, and expected codebase posture
  after completion.
- Keep recommendations incremental; avoid broad rewrites.
- Include enough evidence to justify the phase order.

## Analysis

Evidence from SOW-0029 and follow-up code reads:

- `pkg/cache/cache.go` exposes mutable cache entries:
  - `State` stores `Entries map[string]*Entry`.
  - `Entry` mixes identity, downloader state, timestamps, health, stats,
    legal metadata, and critical-overlap metadata.
  - `Entry(name)` returns a mutable pointer and documents that concurrent
    mutation must be serialized by callers.
- `pkg/engine/run.go` `RunOnce()` coordinates preflight, source processing,
  heavy artifact generation, metadata, insights, publish, marker writes, and
  cache persistence.
- `pkg/scheduler/scheduler.go` has one large `Runner` owning engine pointer,
  queue state, action channel, snapshots, loop state, and metrics.
- `pkg/web/server.go` centralizes public route registration and many endpoint
  decisions in one large surface handler.
- UI admin files such as `feeds-table.tsx` and `feed-modal.tsx` are large and
  mix filtering/sorting, URL state, API actions, and presentation.

Interpretation:

- The first refactor should target ownership boundaries and tests, not package
  reshuffling. Package splits before ownership cleanup would move coupling
  around instead of reducing it.
- External review consensus:
  - Five independent read-only reviews returned `approve with changes`.
  - No reviewer rejected the direction, but no reviewer approved the SOW as-is.
  - The recurring findings were:
    - Phase 0 needs a measurable posture rubric, not subjective letter grades.
    - Phase 1 is much larger than scoped; it needs a mutation inventory/design
      sub-phase before migration.
    - Phase 2 is not merely file movement; route-family extraction must preserve
      listener separation, auth, raw-feed safety, redistributability, and
      critical-artifact staleness checks.
    - Phase 3 should preserve the scheduler's shared queue-lock invariant and
      may be lower priority unless scheduler work is already queued.
    - Phase 4 is the highest-value backend refactor but must start with a
      pipeline data-flow/resource-lifecycle map and must include `output.go`,
      generated-file mtimes, marker ordering, and cache-save-on-error behavior.
    - Phase 5 needs URL-state regression tests for admin/home filtering and
      sorting, not only lint/build.
    - Phase 6 should be optional/deferred instead of a standalone near-term
      phase.

Revised ordering principle:

1. First create guardrails and baselines that make improvement measurable.
2. Then inventory the high-risk ownership surfaces before changing them.
3. Prefer the engine pipeline refactor once its data-flow and rollback gates are
   understood, because it has the largest maintainability payoff.
4. Treat cache mutation migration as a major refactor, not a small prerequisite.
5. Keep package-boundary enforcement deferred until smaller ownership boundaries
   are already real.

## Implications and decisions

Implementation requires a future user decision because each phase can be shipped
independently.

User decision recorded 2026-04-30:

- Proceed with this SOW after the phase plan and independent reviews.
- Treat Phase 0, Phase 1a, and Phase 4a as approved because they are
  guardrail/inventory/spec work and do not change runtime product behavior.
- Do not treat this as approval for Phase 1b, Phase 2, Phase 3, Phase 4b, or
  Phase 5 implementation. Those phases still need a decision after the new
  posture evidence is reviewed.

This SOW deliberately does not authorize product-code refactors beyond the
architecture posture tooling/spec/skill work recorded below.

User decision recorded 2026-04-30 after Phase 0/1a/4a evidence:

- Proceed with the next implementation task.
- Interpreted as approval for Phase 4b because it was the first recommended
  next phase and has the highest maintainability payoff.
- Scope remains behavior-preserving engine pipeline decomposition. Any new
  product behavior, cache mutation API design, package split, or route/UI
  refactor remains out of scope.

User decision recorded 2026-04-30 after Phase 4b completion:

- Proceed with the next recommended implementation task.
- Interpreted as approval for Phase 2 web route-family separation because it
  was the first recommended next phase after Phase 4b.
- Scope remains behavior-preserving route registration decomposition. URL
  paths, response schemas, public/admin listener policy, admin auth, raw-feed
  eligibility, redistributability, safe path handling, and stale artifact checks
  remain unchanged.

User decision recorded 2026-04-30 after Phase 2 completion:

- Proceed with the next recommended implementation task.
- Interpreted as approval for Phase 5 UI admin and home component
  decomposition because it was the first recommended next phase after Phase 2.
- Scope remains behavior-preserving frontend decomposition. URL state,
  filtering, sorting, admin actions, query invalidation, responsive layout, and
  existing visual design remain unchanged.

User decision recorded 2026-04-30 after Phase 5 completion:

- Proceed with the next recommended implementation task.
- Interpreted as approval for bounded Phase 1b cache ownership slices, not the
  full cache mutation API redesign.
- Scope is limited to small semantic cache-owned transitions that preserve the
  JSON schema and can be independently validated. Generic setters, generic
  mutation callbacks, and broad lifecycle-transition redesign remain out of
  scope until their semantic API is explicitly approved.

User decision recorded 2026-04-30 after initial Phase 1b cache ownership slices:

- Proceed with focused analysis and the next safe Phase 1b cache ownership
  migration.
- Analysis found `pkg/engine/download_stage.go` is the largest remaining cache
  mutation hot spot, with 108 direct production cache-entry field writes.
- Scope for this slice is limited to download attempt preflight state:
  checked timestamp, disabled status, downloading status, and missing-env
  status. Result handling, prepared-body handling, provider staging, source
  date, and broader downloader lifecycle APIs remain separate slices.
- To avoid duplicating status string literals, the persisted download status
  vocabulary may move to `pkg/cache`, while `pkg/engine` keeps compatibility
  aliases for existing engine code.

User decision recorded 2026-04-30 after download-preflight Phase 1b slice:

- Proceed with the next Phase 1b cache ownership slice in `download_stage.go`.
- Analysis found the remaining direct writes in `download_stage.go` are almost
  entirely persisted download result state:
  - `LastStatus`/`LastError` for download failed, URL resolve failed, not
    modified, same, downloaded, empty, prepare failed, history snapshot failed,
    and generic local operation failed,
  - `SourceDate` updates from downloader or prepared-body observed timestamps,
  - resolved provider `URL`/`PublicURL` metadata after ASN URL indirection.
- Scope for this slice is limited to named cache entry lifecycle methods for
  those download result, source-date, and resolved-URL transitions.
- This is not approval for a generic cache mutation setter, a broad downloader
  redesign, or changes outside existing persisted cache semantics.
- Existing failure-counter decisions remain in the engine for now; this slice
  preserves the existing places that call `incrementFailure()` and
  `clearFailure()`.

User decision recorded 2026-04-30 after download-result Phase 1b slice:

- Proceed with the next Phase 1b cache ownership slice.
- Analysis found the largest remaining hot spot is
  `pkg/engine/bootstrap_entries.go`, with 40 direct cache-entry field writes.
- Scope for this slice is limited to bootstrap/config seeding state:
  - applying authored source/artifact metadata to a cache entry,
  - applying filesystem bootstrap evidence from restored source/set files,
  - applying source-date/processed-date/checked-date fallback evidence from
    history and current set stats,
  - refreshing critical-infrastructure content-hash stats from disk.
- The engine keeps ownership of config interpretation, path selection,
  filesystem probing, parsing IP sets, and constructing the semantic input
  structs. The cache package owns the cache-entry field transitions.
- This is not approval for moving `config` imports into `pkg/cache`, generic
  setters, or changing bootstrap behavior.

User decision recorded 2026-04-30 after bootstrap/config seeding Phase 1b
slice:

- Proceed with the next Phase 1b cache ownership slice.
- Analysis found the largest remaining engine hot spots are
  `pkg/engine/asn.go`, with 37 direct cache-entry field writes, and
  `pkg/engine/geoloc.go`, with 29 direct cache-entry field writes.
- Scope for this slice is limited to ASN/geolocation provider cache state:
  applying provider source metadata, marking provider load/config/filesystem
  statuses, applying provider freshness/stats evidence, and preserving the
  existing updated-versus-stale status decision after a provider is loaded.
- The engine keeps ownership of source selection, file paths, archive
  extraction/parsing, database opening, stats collection, and logging. The
  cache package owns only the cache-entry field transitions.
- This is not approval for moving provider parsing into `pkg/cache`, adding a
  generic status setter, changing persisted status strings, or changing
  provider fan-out behavior.

User decision recorded 2026-04-30 after ASN/geolocation provider Phase 1b
slice:

- Proceed with the next Phase 1b cache ownership slice.
- Analysis found the next cohesive cache mutation surface is ordinary source
  processing and finalization:
  - `pkg/engine/process.go`, with 28 direct cache-entry field writes,
  - `pkg/engine/finalize.go`, with 20 direct cache-entry field writes,
  - `pkg/engine/helpers.go`, with 7 direct writes in the shared
    `applyEntryStatsUpdate()` helper.
- Scope for this slice is limited to source processing/finalization cache
  state: applying per-run source metadata, disabled/missing-input/processing
  and processing-error statuses, final set metadata/freshness/content hash,
  shared min/max/version/cadence stats, and final empty/updated completion
  status.
- The engine keeps ownership of body claiming, file paths, parsing,
  final-set writing, kernel apply, retention update, history ledger append,
  rotation stats, output publication, and logging. The cache package owns only
  the cache-entry field transitions.
- This is not approval for changing processing status strings, moving parsing
  or filesystem writes into `pkg/cache`, changing timestamp semantics, or
  changing the retention/history pipeline.

User instruction recorded 2026-04-30 after Phase 1b completion:

- The software is maintained by the assistant for internal maintainability
  choices.
- Do not ask the user non-behavior questions.
- Proceed to the next maintenance step when the remaining choice is internal
  ordering or decomposition, and stop only for behavior, risk acceptance,
  destructive, production, or externally visible design decisions.
- Interpreted as approval to start Phase 3 scheduler decomposition because it
  is a behavior-preserving internal ownership split.
- Scope remains limited to same-package scheduler decomposition. Queue cadence,
  action semantics, download/processing admission, active/deferred/refetch
  handling, provider-default reprocess behavior, staged-work recovery,
  scheduler-to-engine `RunOptions`, metrics, snapshots, and admin-visible
  background work must remain unchanged.

## Cross-cutting constraints

- Preserve cache-first public serving; public requests must not trigger broad
  recomputation.
- Preserve generated-artifact timestamp integrity. Every moved writer must keep
  deliberate logical mtimes; incidental wall-clock mtimes are correctness bugs.
- Preserve public/admin listener separation and admin auth behavior.
- Preserve raw feed trust boundaries: name validation, public-feed eligibility,
  redistributability, safe path resolution, and bounded streaming behavior.
- Preserve the scheduler's shared lock authority over download and processing
  queue maps unless a later SOW explicitly designs a replacement.
- Preserve `pkg/iprange` as standalone.
- Each implementation phase must be independently revertable. If validation
  fails after a phase, the default response is revert/pause, not forward-fix
  through unrelated scope.

## Posture rubric

Phase 0 must produce the baseline and thresholds. Later phase outcomes must use
these metrics instead of subjective grades:

- Size: max file lines, max function lines, count of files above threshold.
- Complexity: function complexity/cognitive-complexity hot spots.
- Coupling: direct imports, transitive deps, package import-boundary violations.
- Ownership: count of direct mutable cache-entry field writes and direct
  pointer-returning cache access outside allowed packages/tests.
- Public/admin safety: listener-mode route coverage, raw-feed safety coverage,
  public API contract coverage.
- Pipeline integrity: generated artifact count, generated artifact mtimes,
  provider-set markers, cache-save-on-error behavior, integrity findings.
- Runtime posture: benchmark deltas, race-test status, memory/heap deltas where
  touched.

## Plan

### Phase 0 — Baseline And Architecture Gates

Scope:

- Add automated structure/posture checks without changing product behavior.
- Capture package import metrics, largest-file thresholds, largest-function
  thresholds, complexity hot spots, direct cache mutation counts, forbidden
  semantic shortcuts, and package-boundary invariants.
- Produce a posture baseline artifact or test fixture containing:
  - per-package file counts and line counts,
  - max file/function size,
  - direct/transitive dependency counts,
  - direct `*cache.Entry` mutation and pointer-access counts,
  - public/admin route coverage inventory,
  - benchmark baseline for representative backend paths.
- Add a short architecture section to the relevant project spec and reviewing
  skill so future SOWs measure separation-of-concerns impact.
- Add architecture tests that fail only on new/worsened violations unless the
  SOW records explicit user approval for the regression.

Validation:

- New tests/scripts run in CI/local validation.
- Existing `make test`, `make lint`, `make race`, frontend lint/build remain
  green.
- `make bench` or a targeted benchmark baseline is captured for paths that
  later phases may touch.
- Listener-mode regression tests exist or are recorded as required work before
  Phase 2:
  - public listener cannot reach admin routes,
  - admin listener can reach admin routes,
  - shared listener preserves admin auth behavior.

Expected posture after phase:

- No product behavior changes.
- Architecture drift becomes visible through metrics instead of judgment.
- Later phases can prove improvement numerically.
- Regression resistance improves because large-file, large-function, cache
  mutation, route-safety, and package-boundary changes become explicit review
  events.

Risk: low. Mostly tests/spec/skill additions.

Rollback/pause criteria:

- Baseline checks are too noisy to distinguish real architectural drift from
  existing debt.
- Existing validation fails.
- Thresholds would block normal development without user-approved exceptions.

### Phase 1a — Cache Mutation Inventory And API Design

Scope:

- Catalog every direct `cache.Entry` mutation and every direct pointer-returning
  cache access in production code and tests.
- Classify mutations by semantic transition, not by field group:
  - bootstrap/config seeding,
  - download attempt/success/failure,
  - staged artifact/fetch result,
  - prepared body processing,
  - processing/finalization result,
  - failure-state migration,
  - source/processed/checked timestamp repair,
  - history/rotation/change-ratio/unique-share stats,
  - metadata/publication/version updates,
  - provider/default/critical-overlap updates,
  - integrity repair state.
- Design the smallest practical mutation API surface that preserves the JSON
  schema while making ownership explicit.
- Record which direct mutation sites are intentionally allowed only in tests or
  transitional helpers.

Validation:

- Inventory includes call-site counts and file lists.
- API design maps every production call site to either a semantic mutation API
  or a documented exception.
- No product code behavior changes.

Expected posture after phase:

- Cache ownership risk becomes known and bounded.
- The project can decide whether migration should happen before or after the
  engine pipeline refactor.

Risk: low/medium. Analysis/design only, but it affects a core future API.

Rollback/pause criteria:

- Mutation categories do not map cleanly to stable semantic transitions.
- The API design would create a larger or more confusing surface than direct
  mutation.

### Phase 1b — Cache State Ownership Migration

Scope:

- Implement the approved mutation API from Phase 1a.
- Migrate production call sites by semantic category, preferably in small
  independently revertable chunks.
- Keep the underlying JSON schema stable.
- Restrict pointer-returning APIs to a narrow, documented allow-list.
- Add lint/analyzer/test coverage that prevents new direct production mutation
  outside the allow-list.

Validation:

- Same-entry and multi-entry race tests focused on the new mutation APIs.
- Existing cache tests.
- Engine pipeline tests and integrity harness.
- Affected tests migrated to approved setup helpers or mutation APIs.
- `make test`, `make lint`, `make race`, and relevant benchmarks are mandatory.

Expected posture after phase:

- Direct production mutation of cache entries is removed or documented in an
  allow-list.
- Concurrency posture improves because mutation ownership is enforced by API and
  tests, not caller convention.
- `pkg/cache` becomes an owned state API instead of a shared mutable map.

Risk: medium/high. This touches many engine write paths and tests.

Rollback/pause criteria:

- Any race detector warning.
- Integrity findings appear after migration.
- The migration leaves mixed ownership without an explicit allow-list and
  follow-up.
- Benchmark or memory regressions exceed the Phase 0 threshold.

### Phase 2 — Web Route Family Separation

Scope:

- Extract `newSurfaceHandler()` into composable route-registration functions or
  a narrow server dependency struct. The goal is not file movement by itself;
  the goal is to preserve shared listener/cache/path dependencies without a
  single central switch.
- Split route families where this reduces local complexity:
  - public feed routes,
  - raw/feed artifact serving,
  - country/ASN/entity route registration,
  - critical-infrastructure routes,
  - admin routes and listener policy, including `admin.go`,
  - shared HTTP/cache helpers.
- Keep URL paths and response schemas unchanged.
- Preserve route validation order:
  - route/name validation,
  - public/admin/listener eligibility,
  - feed eligibility and redistributability,
  - safe path resolution,
  - stale-artifact/provider-set checks,
  - bounded response serving.

Validation:

- Existing web feature tests.
- Public API smoke tests for representative routes.
- Admin UI smoke around integrity/admin pages.
- Listener-mode regression tests from Phase 0.
- Raw feed route tests for path traversal, redistributability, public-feed
  eligibility, missing-file behavior, and bounded streaming/cache behavior.
- Critical-infrastructure route tests for stale provider-set rejection.

Expected posture after phase:

- Web package remains coupled to engine/scheduler, but route behavior becomes
  locally understandable.
- Future endpoint additions stop increasing one central switch.
- Public vs admin serving discipline becomes easier to review.

Risk: medium. The route split touches security, licensing, public API, and admin
operator behavior even when response schemas stay unchanged.

Rollback/pause criteria:

- Any public/admin listener exposure regression.
- Any raw feed safety or redistributability regression.
- Any response schema change not explicitly approved.

### Phase 3 — Scheduler State Machine Decomposition

Scope:

- Keep this phase lower priority unless scheduler/background-work changes are
  already queued.
- Split scheduler concerns inside the same package first, focusing on the most
  review-sensitive policies:
  - admission/deduplication policy,
  - provider-default reprocess enqueue policy,
  - staged-work recovery,
  - manual/admin actions,
  - snapshot/metrics persistence.
- Avoid changing runtime semantics.
- Preserve the single shared lock authority over download and processing queue
  maps unless a later SOW explicitly designs a replacement.
- Preserve the scheduler to engine `RunOptions` contract unless Phase 4
  explicitly changes it in a coordinated plan.

Validation:

- Existing scheduler tests.
- New targeted tests for action dedupe, provider-default reprocess enqueue,
  staged-work recovery, and processing admission.
- Install smoke with admin activity visibility.
- `make race` is mandatory.
- Tests for refetch while active download, processing deferred while active, and
  download-input-settled behavior.

Expected posture after phase:

- Scheduler moves from dense state machine to separable policies where it
  matters.
- Background-work CPU and queue behavior become easier to reason about.
- Future operational features have smaller blast radius.

Risk: medium. Scheduling semantics are user-visible and operationally sensitive.

Rollback/pause criteria:

- Background work disappears from admin visibility.
- Queue dedupe/refetch behavior changes without explicit approval.
- Race detector reports queue-state access issues.

### Phase 4a — Engine Pipeline Data-Flow And Lifecycle Map

Scope:

- Map the current `RunOnce()` data flow and resource lifecycle before changing
  product code:
  - trigger decisions: updates, selected feeds, provider defaults, critical
    provider set changes, database source selection, recheck/reprocess,
  - fan-out decisions: scoped vs global reprocess, critical-only runs,
  - phase dependencies: GeoIP, bogons, ASN, critical infrastructure, entity
    sidecars, metadata, insights, publish,
  - shared resources: web batch, entity batch, heavy set cache, prepared
    provider datasets, bogon union, ASN databases, critical datasets,
  - cleanup/error paths: `closeAll`, staged-batch cleanup, cache-save-on-error,
    marker writes, generated-file timestamp application,
  - downstream write surfaces in `output.go` and other artifact writers.
- Identify which `Engine` fields belong to cohesive ownership groups such as
  run state, cache state, background tasks, entity state, provider caches, and
  runtime overrides.
- Produce the implementation plan for Phase 4b with rollback and fixture
  comparison details.

Validation:

- No product code behavior changes.
- Data-flow map covers all existing `RunPhase` values and every heavy block in
  `RunOnce()`.
- Lifecycle map identifies every resource with open/close/cleanup ownership.

Expected posture after phase:

- Engine refactor risk becomes explicit instead of hidden in one procedural
  function.
- The project can choose whether to proceed with Phase 4b before Phase 1b.

Risk: low/medium. Analysis/design only, but incomplete analysis would make
Phase 4b dangerous.

Rollback/pause criteria:

- Data dependencies are too entangled to extract safely without first improving
  tests or state ownership.
- Fixture comparison cannot be made deterministic enough to validate Phase 4b.

### Phase 4b — Engine Pipeline Phase Boundaries

Scope:

- Refactor `RunOnce()` into explicit phase functions/objects with stable inputs
  and outputs:
  - preflight,
  - source processing,
  - heavy artifact planning,
  - provider comparison phases,
  - entity sidecar phase,
  - metadata/insights phase,
  - publish/marker/cache-save phase.
- Introduce a run plan/result structure so trigger decisions are computed once
  and passed forward explicitly.
- Group `Engine` state into cohesive internal structs or narrow interfaces when
  doing so reduces access to unrelated fields.
- Include `output.go` and publish/timestamp helpers in the scope where they are
  part of the same pipeline ownership problem.
- Keep package split optional until the function-level boundary is proven.

Validation:

- Full engine tests.
- Integrity harness scenarios.
- Real install smoke.
- Deterministic fixture run comparing generated artifact set, artifact content
  where stable, generated artifact mtimes, provider-set markers, cache-save
  behavior, and integrity output before/after.
- Tests for cache save on early abort/cancel.
- Tests for marker write ordering after provider-default or critical-provider
  drift.
- `make race` and targeted benchmarks are mandatory.

Expected posture after phase:

- `pkg/engine` remains large, but the central workflow becomes inspectable.
- New artifact families can attach to a phase instead of modifying a long
  procedural block.
- Pipeline timestamp and artifact ownership contracts become easier to enforce.

Risk: high. This is the highest-value backend refactor and also the highest
regression risk.

Rollback/pause criteria:

- Any generated artifact, mtime, marker, or integrity difference that is not
  explicitly expected and documented.
- Any loss of cache-save-on-error behavior.
- Any benchmark or memory regression exceeding the Phase 0 threshold.
- Any install smoke or admin visibility regression.

### Phase 5 — UI Admin And Home Component Decomposition

Scope:

- Target concrete files first:
  - `ui/src/components/admin/feeds-table.tsx`,
  - `ui/src/components/admin/feed-modal.tsx`,
  - other admin/home/feed-detail components only when they exceed the Phase 0
    size/mixed-concern threshold.
- Split large admin components into:
  - data model/hooks,
  - filters/sort model,
  - action dispatch,
  - table presentation,
  - row/cell components.
- Apply the same pattern to large home/feed-detail surfaces only where file size
  or mixed concerns justify it.

Validation:

- `pnpm --dir ui lint`
- `pnpm --dir ui build`
- Playwright or equivalent browser tests for admin filter/sort URL roundtrips,
  modal action flows, home filters, and feed detail tab selection.
- Screenshot/manual checks for admin feeds table, integrity tables, home
  filters, and feed detail tabs when visual structure changes.

Expected posture after phase:

- Frontend maintainability improves from "feature works but large files" to
  "feature behavior has local ownership".
- UI regressions around sorting/filtering become easier to isolate.

Risk: medium. UI refactors can create subtle state/URL regressions.

Rollback/pause criteria:

- URL state cannot roundtrip through reload/back/forward.
- Query invalidation or admin action behavior changes without explicit approval.
- Mobile/desktop screenshots show overlapping or clipped controls.

### Phase 6 — Deferred Package Boundary Enforcement

Scope:

- This is not a near-term implementation phase.
- After Phases 1-4 reduce implicit coupling, evaluate whether
  compiler-enforced package boundaries would still add value:
  - `pkg/engine/pipeline` or internal phase modules,
  - route packages under `pkg/web`,
  - scheduler policy helpers,
  - cache mutation package or interfaces.
- Add or tighten import-boundary tests with `go list` only for boundaries that
  are already true or intentionally created by earlier phases.

Validation:

- Full backend validation.
- Architecture tests prevent cycles and accidental imports.

Expected posture after phase:

- Codebase can move from B/B+ to A-/B+ structurally.
- Maintainers can understand major subsystems through package boundaries, not
  just file names and conventions.

Risk: medium/high. Package splits can introduce cycles and false boundaries if
done before ownership cleanup.

Rollback/pause criteria:

- Package boundaries create cycles or large interface shims.
- Boundaries move coupling instead of reducing it.
- The only benefit is metric improvement without simpler implementation.

## Execution log

2026-04-30:

- Moved this SOW from pending to current and marked it `in-progress`.
- Implemented Phase 0 architecture posture tooling:
  - added `tools/archposture`,
  - added `tools/archposture/testdata/posture_baseline.json`,
  - added `go test ./tools/archposture` as the guard for accepted posture
    regressions.
- Implemented the Phase 0 durable memory updates:
  - added `.agents/sow/specs/architecture-posture.md`,
  - added the spec to `.agents/sow/specs/README.md`,
  - updated `.agents/skills/project-reviewing/SKILL.md` so future reviews run
    the posture guard when touching architecture-risk areas.
- Completed Phase 1a inventory in the architecture posture spec:
  - production `Entry()` calls: 34,
  - test `Entry()` calls: 83,
  - production mutable cache-entry field writes: 385,
  - test mutable cache-entry field writes: 308,
  - production full-entry replacements: 3,
  - semantic mutation categories recorded for future API design.
- Completed Phase 4a pipeline map in the architecture posture spec:
  - mapped `(*Engine).RunOnce` phase order,
  - recorded fan-out and heavy-trigger decisions,
  - recorded resource lifecycle ownership,
  - identified `pkg/engine/output.go` as part of the same pipeline ownership
    problem.
- Implemented Phase 4b behavior-preserving engine pipeline decomposition:
  - reduced `(*Engine).RunOnce` to orchestration,
  - added `pkg/engine/run_pipeline.go` with explicit source-processing,
    run-plan, heavy-phase, metadata/insights, and publication helpers,
  - preserved the existing publication sequence:
    `BeforePublish` hook, generated-file timestamp application, web publish,
    entity publish, raw public set copy, generated-file ledger sync, marker
    writes, final cache save,
  - preserved cache save on early abort through the existing deferred run-exit
    save.
- Added `pkg/engine/run_pipeline_test.go` to characterize the run-plan branches
  most likely to regress:
  - no-update runs do not publish,
  - selected ASN/geolocation database sources force heavy phases,
  - critical provider-set drift can take the critical-only branch,
  - provider-default drift forces global fan-out.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted improvement so `RunOnce` cannot silently grow back to the previous
  large-function shape.
- Implemented Phase 2 behavior-preserving web route-family separation:
  - reduced `newSurfaceHandler` to listener-mode orchestration,
  - added `pkg/web/routes.go` with explicit route-family registration for
    public API routes, admin routes, embedded assets, public artifacts, raw feed
    downloads, and SPA fallback routes,
  - preserved public/admin listener mode behavior, admin auth, raw-feed
    eligibility, redistributability checks, safe path resolution, and stale
    critical-artifact rejection,
  - added `pkg/web/routes_test.go` to characterize shared, public-only, and
    admin-only listener surfaces.
- Refreshed `tools/archposture/testdata/posture_baseline.json` again with the
  accepted Phase 2 improvement so `newSurfaceHandler` cannot silently grow back
  to the previous large-function shape.
- Implemented Phase 5 behavior-preserving admin UI decomposition:
  - reduced `ui/src/components/admin/feeds-table.tsx` to the exported table
    container and URL-state orchestration,
  - added `feeds-table-model.ts`, `feeds-table-filters.tsx`, and
    `feeds-table-body.tsx` for filter/sort/count logic, filter chips, and row
    presentation,
  - reduced `ui/src/components/admin/feed-modal.tsx` to the exported sheet
    container and modal section orchestration,
  - added focused feed-modal owner files for hero/actions, identity,
    schedule/timeline/content, manifest, diagnostics, and shared primitives,
  - preserved public imports, URL parameters, filters, sort keys, admin action
    invalidation, modal section order, and existing visual structure.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted Phase 5 improvement so the old 1,200+ line admin files are no longer
  accepted as the debt ceiling.
- Implemented the bounded Phase 1b initial cache replacement migration:
  - added `cache.State.ReplaceEntry(name, entry)` for complete synthesized
    entry replacement,
  - copied slice fields during replacement so callers cannot mutate cached
    `HistoryMinutes` or `CriticalOverlapTiers` after storing,
  - normalized the stored `Entry.Name` to the configured cache key,
  - migrated disk bootstrap and invalid timestamp repair away from
    `*state.Entry(name) = entry` replacement through mutable pointers,
  - added cache tests for copy semantics and nil-map initialization.
- Implemented the bounded Phase 1b failure-state lifecycle migration:
  - added `cache.Entry.RecordDownloadFailure()` and
    `cache.Entry.ClearDownloadFailure()`,
  - moved the engine failure-state helper away from direct
    `DownloadFailures`/`FailureStartedDate` field writes,
  - added cache tests for initial failure, repeated failure, missing-start
    repair, and success reset semantics.
- Implemented bounded Phase 1b run-attempt and critical-overlap lifecycle
  migrations:
  - added `cache.Entry.MarkRunStarted()`,
    `cache.Entry.RecordProcessingDuration()`,
    `cache.Entry.SetCriticalOverlapTiers()`, and
    `cache.Entry.ClearCriticalOverlapTiers()`,
  - moved run-attempt status, processing duration, and critical-overlap tier
    summary writes out of engine direct field mutation,
  - added cache tests for run-attempt state, processing duration, overlap-tier
    copy semantics, and overlap-tier clear semantics.
- Implemented the bounded Phase 1b unique-share lifecycle migration:
  - added `cache.Entry.SetUniqueShare()` with the existing `[0, 100]` clamp,
  - moved unique-share percent/sample writes out of the engine helper,
  - added cache tests for clamping and sample recording.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted Phase 1b cache ownership improvements so production full-entry
  replacement through `Entry()` and direct engine failure-state writes cannot
  silently return.
- Implemented the bounded Phase 1b download-preflight lifecycle migration:
  - moved persisted download status vocabulary to `pkg/cache`,
  - kept `pkg/engine` compatibility aliases for existing engine call sites,
  - added `cache.Entry.MarkDownloadStarted()`,
    `cache.Entry.MarkDownloadDisabled()`, and
    `cache.Entry.MarkDownloadMissingEnv()`,
  - moved checked timestamp, downloading, disabled, and missing-environment
    URL-template writes out of five `download_stage.go` preflight branches,
  - added cache tests for each new preflight transition.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted download-preflight improvement so these direct engine writes cannot
  silently return.
- Implemented the bounded Phase 1b download-result lifecycle migration:
  - added named cache entry methods for resolved provider URL metadata,
    source-observed date, download failed, URL resolve failed, generic local
    operation failed, prepare failed, history snapshot failed, not modified,
    same, downloaded, and empty download-stage results,
  - migrated the remaining direct `cache.Entry` field writes out of
    `download_stage.go`,
  - preserved existing `incrementFailure()` and `clearFailure()` call sites so
    failure-counter semantics do not change,
  - added cache tests for resolved URL, source date, failure statuses, and
    success statuses.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted download-result improvement so direct cache-entry writes cannot
  silently return to `download_stage.go`.
- Implemented the bounded Phase 1b bootstrap/config seeding lifecycle migration:
  - added cache-owned source/artifact config snapshot methods,
  - added cache-owned restored artifact evidence, history timestamp, disk set
    stats, disk-bootstrap finalization, content-hash clear, and critical
    content-hash refresh methods,
  - migrated the direct `cache.Entry` field writes out of
    `bootstrap_entries.go`,
  - preserved engine ownership of config interpretation, path selection,
    filesystem probing, and set parsing,
  - added cache tests for config snapshot copy/fallback behavior, artifact
    bootstrap evidence, history timestamp evidence, disk set stats/finalization,
    and critical content-hash refresh.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted bootstrap/config seeding improvement so direct cache-entry writes
  cannot silently return to `bootstrap_entries.go`.
- Implemented the bounded Phase 1b ASN/geolocation provider lifecycle
  migration:
  - added cache-owned provider source metadata, provider load status, provider
    freshness/stats, and updated-versus-stale completion methods,
  - migrated the direct `cache.Entry` field writes out of `asn.go` and
    `geoloc.go`,
  - preserved engine ownership of provider selection, paths, archive
    extraction, parsing/opening, stats collection, fan-out, and logging,
  - self-review corrected the provider clock-skew handoff so it preserves the
    previous `time.Sub(...).Seconds()` precision instead of deriving skew from
    whole Unix seconds,
  - added cache tests for provider metadata, all provider status transitions,
    provider stats/min-max/version behavior, clock skew, and stale cached
    provider completion after download failure.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted ASN/geolocation provider improvement so direct cache-entry writes
  cannot silently return to `asn.go` or `geoloc.go`.
- Implemented the bounded Phase 1b source processing/finalization lifecycle
  migration:
  - added cache-owned source processing metadata, source processing status,
    finalized set evidence, finalized source metadata, completion status, and
    shared stats-update methods,
  - migrated the direct `cache.Entry` field writes out of `process.go`,
    `finalize.go`, and `helpers.go`,
  - preserved engine ownership of body claiming, parsing, final-set writing,
    kernel apply, retention, history ledger append, rotation stats, output
    publication, and logging,
  - kept clock-skew calculation in the engine with the existing
    `time.Sub(...).Seconds()` semantics before handing the value to the cache,
  - added cache tests for processing metadata copy behavior, disabled and
    missing-input statuses, parse/finalize/retention failure statuses,
    finalized set and metadata state, completion state, and shared stats
    updates.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted source processing/finalization improvement so direct cache-entry
  writes cannot silently return to `process.go`, `finalize.go`, or
  `helpers.go`.
- Phase 1b artifact-materialization implementation decision:
  - evidence: `pkg/engine/artifact_stage.go` still has 27 direct mutable
    `cache.Entry` field writes after the source processing/finalization slice,
  - scope: artifact parent download started/download result/source date and
    artifact-child materializing/download failure status,
  - cache should own status/timestamp transitions through existing download
    lifecycle methods plus one artifact-child materialization method,
  - engine keeps artifact lookup, provider-specific fetch, local file download,
    child spec selection, child output materialization, staged promotion, and
    scheduling decisions.
- Implemented the bounded Phase 1b artifact-materialization lifecycle
  migration:
  - reused cache-owned download lifecycle methods for artifact parent download
    start, source date, success, and failure statuses,
  - added `cache.Entry.MarkArtifactChildMaterializing()` for artifact-derived
    child feed materialization state,
  - migrated the direct `cache.Entry` field writes out of
    `artifact_stage.go`,
  - preserved engine ownership of artifact lookup, DroneBL fetch, local file
    download, child spec selection, materialized child output, staged promotion,
    and scheduling decisions,
  - added a cache test for artifact-child materialization state.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted artifact-materialization improvement so direct cache-entry writes
  cannot silently return to `artifact_stage.go`.
- Phase 1b runtime-ledger implementation decision:
  - evidence: `pkg/engine/runtime_ledger_cache.go` still has 20 direct mutable
    `cache.Entry` field writes, all inside `historyLedgerStats.apply`,
  - scope: applying already-computed history ledger statistics to cache entry
    version, first/last stats, min/max stats, gap totals, and update cadence,
  - cache should own the grouped field transition through a semantic history
    ledger stats snapshot method,
  - engine keeps ledger CSV parsing, runtime tail caching, duplicate timestamp
    handling, and cadence calculation before handing a snapshot to cache.
- Implemented the bounded Phase 1b runtime-ledger cache ownership migration:
  - added `cache.Entry.ApplyHistoryLedgerStats()` for grouped history ledger
    aggregate state,
  - migrated the direct `cache.Entry` field writes out of
    `runtime_ledger_cache.go`,
  - preserved engine ownership of ledger CSV parsing, duplicate timestamp
    handling, runtime tail caching, and cadence calculation,
  - added cache tests for applying and rejecting history ledger stat snapshots.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted runtime-ledger improvement so direct cache-entry writes cannot
  silently return to `runtime_ledger_cache.go`.
- Phase 1b timestamp-repair implementation decision:
  - evidence: `pkg/engine/entry_timestamp_sanitize.go` still has 15 direct
    mutable `cache.Entry` field writes across invalid timestamp repair
    branches,
  - scope: repairing invalid JSON timestamps for source, processed, checked,
    started, and failure-start timestamps using disk/history evidence,
  - cache should own the invalid-timestamp repair transition because these
    fields are persisted cache integrity state,
  - engine keeps disk/history evidence discovery and only passes latest/first
    observed timestamps to cache.
- Implemented the bounded Phase 1b timestamp-repair lifecycle migration:
  - added cache-owned JSON timestamp validity helpers and
    `cache.Entry.RepairInvalidTimestamps()`,
  - migrated the direct `cache.Entry` field writes out of
    `entry_timestamp_sanitize.go`,
  - preserved engine ownership of disk/history evidence discovery,
  - added cache tests for evidence-based repair, fallback repair, and clean
    entry no-op behavior.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted timestamp-repair improvement so direct cache-entry writes cannot
  silently return to `entry_timestamp_sanitize.go`.
- Phase 1b rotation-stats implementation decision:
  - evidence: `pkg/engine/rotation_stats.go` still has 15 direct mutable
    `cache.Entry` field writes for rotation and change-ratio summary fields,
  - scope: applying or clearing already-computed rotation/change-ratio summary
    values,
  - cache should own the grouped rotation-stat field transition,
  - engine keeps size/churn series reading, percentile calculation, rounding,
    and empty-input decisions.
- Implemented the bounded Phase 1b rotation-stats lifecycle migration:
  - added `cache.Entry.ApplyRotationStats()` and
    `cache.Entry.ClearRotationStats()` for grouped rotation/change-ratio
    summary state,
  - migrated the direct `cache.Entry` field writes out of `rotation_stats.go`,
  - preserved engine ownership of size/churn series reading, percentile
    calculation, rounding, and empty-input decisions,
  - added cache tests for applying and clearing rotation stats.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted rotation-stats improvement so direct cache-entry writes cannot
  silently return to `rotation_stats.go`.
- Phase 1b legacy failure-start bootstrap implementation decision:
  - evidence: `pkg/engine/legacy_failure_bootstrap.go` has one remaining direct
    mutable cache write, setting `FailureStartedDate` from an imported legacy
    cache,
  - scope: recording an imported failure start timestamp without incrementing
    download failure counters,
  - cache should own the field transition through a narrow legacy-import
    method,
  - engine keeps legacy cache discovery, import eligibility checks, recovery
    detection, and persistence.
- Implemented the bounded Phase 1b legacy failure-start bootstrap lifecycle
  migration:
  - added `cache.Entry.RecordLegacyFailureStart()` for imported legacy failure
    timestamps that must not increment current failure counters,
  - migrated the final direct engine-owned cache field write out of
    `legacy_failure_bootstrap.go`,
  - preserved engine ownership of legacy cache discovery, import eligibility
    checks, recovery detection, and persistence,
  - added cache tests for preserving failure counters and rejecting empty
    legacy timestamps.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted legacy failure-start improvement so direct cache-entry writes cannot
  silently return to `legacy_failure_bootstrap.go`.
- Implemented the bounded Phase 3 scheduler decomposition:
  - split `pkg/scheduler/scheduler.go` into same-package concern files for
    admin/manual actions, automatic due policy, download loop, processing loop,
    queue admission/refetch/deferred handling, staged-work recovery/provider
    waves, and snapshot/due calculation,
  - preserved the existing `Runner` shared `stateMu` authority over download
    and processing queue maps,
  - kept scheduler-to-engine `RunOptions`, cadence, queue semantics, active
    work visibility, metrics, and persisted snapshot behavior unchanged,
  - added targeted policy tests for queued-action dedupe, active download
    refetch deferral/release, provider-default drift enqueue, and staged-source
    recovery.
- Updated durable project memory for the Phase 3 scheduler lesson:
  - `.agents/skills/project-coding/SKILL.md` records the same-package
    scheduler concern files and the `Runner.stateMu` ownership invariant,
  - `.agents/skills/project-testing/SKILL.md` records scheduler policy test
    coverage and installed-service smoke expectations,
  - `.agents/skills/project-reviewing/SKILL.md` records the shared queue-lock
    review gate.
- Refreshed `tools/archposture/testdata/posture_baseline.json` with the
  accepted scheduler decomposition so `pkg/scheduler/scheduler.go` cannot
  silently grow back into the combined state machine file.

Measured Phase 0 posture baseline:

- source scope: 436 Go/TS/TSX files and 102,629 lines,
- largest files:
  - `pkg/config/catalog_verify_test.go`: 1,675 lines,
  - `pkg/scheduler/scheduler.go`: 1,474 lines,
  - `pkg/engine/output.go`: 1,366 lines,
  - `ui/src/components/admin/feeds-table.tsx`: 1,298 lines,
  - `ui/src/components/admin/feed-modal.tsx`: 1,295 lines,
- largest functions:
  - `pkg/web/server.go` `newSurfaceHandler`: 523 lines, complexity 104,
  - `pkg/engine/entity_integrity.go`
    `(*Engine).CheckEntityArtifactsIntegrity`: 445 lines, complexity 60,
  - `pkg/engine/run.go` `(*Engine).RunOnce`: 430 lines, complexity 87,
  - `pkg/iprange/cli.go` `runCLIV4`: 381 lines,
  - `pkg/iprange/cli6.go` `runCLIV6`: 374 lines,
  - `pkg/engine/output.go` `(*Engine).writeComparisonFiles`: 283 lines,
- core package dependency posture:
  - `pkg/engine`: 119 files, 34,634 lines, 50 direct imports, 465 transitive
    dependencies,
  - `pkg/web`: 29 files, 8,330 lines, 38 direct imports, 478 transitive
    dependencies,
  - `pkg/scheduler`: 8 files, 3,449 lines, 21 direct imports, 466 transitive
    dependencies,
  - `pkg/cache`: 4 files, 1,097 lines, 14 direct imports, 428 transitive
    dependencies,
  - `pkg/iprange`: 53 files, 10,719 lines, 0 project imports, 216 transitive
    dependencies.

Measured Phase 4b posture after implementation:

- source scope: 438 Go/TS/TSX files and 102,752 lines,
- `pkg/engine`: 121 files, 34,757 lines, 50 direct imports, 465 transitive
  dependencies,
- `(*Engine).RunOnce` no longer appears in the large-function list,
- no `pkg/engine/run.go` or `pkg/engine/run_pipeline.go` function is above the
  120-line review threshold,
- production cache mutation counts are unchanged:
  - production `Entry()` calls: 34,
  - production mutable cache-entry field writes: 385,
  - production full-entry replacements: 3.

Measured Phase 2 posture after implementation:

- source scope: 440 Go/TS/TSX files and 102,816 lines,
- `pkg/web`: 31 files, 8,394 lines, 38 direct imports, 478 transitive
  dependencies,
- route registration counts are unchanged:
  - `HandleFunc` registrations: 43,
  - `Handle` registrations: 3,
- `newSurfaceHandler` no longer appears in the large-function list,
- no `pkg/web/routes.go` function is above the 120-line review threshold,
- production cache mutation counts are unchanged:
  - production `Entry()` calls: 34,
  - production mutable cache-entry field writes: 385,
  - production full-entry replacements: 3.

Measured Phase 5 posture after implementation:

- source scope: 449 Go/TS/TSX files and 102,681 lines,
- the two largest admin UI source files were decomposed:
  - `ui/src/components/admin/feeds-table.tsx`: 1,298 lines to 368 lines,
  - `ui/src/components/admin/feed-modal.tsx`: 1,295 lines to 68 lines,
- extracted admin owner files are all below 500 lines:
  - `feeds-table-body.tsx`: 490 lines,
  - `feeds-table.tsx`: 368 lines,
  - `feed-modal-status-sections.tsx`: 337 lines,
  - `feeds-table-model.ts`: 323 lines,
  - all other extracted files are below 250 lines,
- no `ui/src/components/admin/*` function appears in the architecture
  large-function list.

Measured Phase 1b initial cache ownership posture after implementation:

- source scope: 449 Go/TS/TSX files and 102,911 lines,
- production `Entry()` calls: 31, down from 34,
- production mutable cache-entry field writes: 376, down from 385,
- production full-entry replacements: 0, down from 3,
- mutation hot spots remain in `download_stage.go`, `bootstrap_entries.go`,
  `asn.go`, `geoloc.go`, `process.go`, `artifact_stage.go`, and `finalize.go`;
  these require semantic lifecycle APIs, not generic field setters.

Measured Phase 1b download-preflight posture after implementation:

- source scope: 450 Go/TS/TSX files and 102,997 lines,
- `pkg/cache`: 5 files, 1,443 lines,
- `pkg/engine`: 121 files, 34,727 lines,
- production `Entry()` calls: 31, unchanged from the previous Phase 1b slice,
- production mutable cache-entry field writes: 345, down from 376,
- production full-entry replacements: 0,
- `pkg/engine/download_stage.go`: 77 mutable cache-entry field writes, down
  from 108,
- remaining `download_stage.go` writes are result handling, prepared-body,
  provider staging, source-date, and broader downloader lifecycle transitions;
  they should be migrated in separate semantic slices.

Measured Phase 1b download-result posture after implementation:

- source scope: 450 Go/TS/TSX files and 103,174 lines,
- `pkg/cache`: 5 files, 1,658 lines,
- `pkg/engine`: 121 files, 34,689 lines,
- production `Entry()` calls: 31, unchanged from the previous Phase 1b slice,
- production mutable cache-entry field writes: 268, down from 345,
- production full-entry replacements: 0,
- `pkg/engine/download_stage.go`: 0 mutable cache-entry field writes, down
  from 77,
- remaining cache mutation hot spots are `bootstrap_entries.go`, `asn.go`,
  `cache/legacy.go`, `geoloc.go`, `process.go`, `artifact_stage.go`,
  `finalize.go`, `runtime_ledger_cache.go`, `entry_timestamp_sanitize.go`,
  `rotation_stats.go`, `helpers.go`, and `legacy_failure_bootstrap.go`.

Measured Phase 1b bootstrap/config seeding posture after implementation:

- source scope: 450 Go/TS/TSX files and 103,467 lines,
- `pkg/cache`: 5 files, 1,997 lines,
- `pkg/engine`: 121 files, 34,643 lines,
- production `Entry()` calls: 31, unchanged from the previous Phase 1b slice,
- production mutable cache-entry field writes: 228, down from 268,
- production full-entry replacements: 0,
- `pkg/engine/bootstrap_entries.go`: 0 mutable cache-entry field writes, down
  from 40,
- remaining cache mutation hot spots are `asn.go`, `cache/legacy.go`,
  `geoloc.go`, `process.go`, `artifact_stage.go`, `finalize.go`,
  `runtime_ledger_cache.go`, `entry_timestamp_sanitize.go`,
  `rotation_stats.go`, `helpers.go`, and `legacy_failure_bootstrap.go`.

Measured Phase 1b ASN/geolocation provider posture after implementation:

- source scope: 450 Go/TS/TSX files and 103,773 lines,
- `pkg/cache`: 5 files, 2,336 lines,
- `pkg/engine`: 121 files, 34,610 lines,
- production `Entry()` calls: 31, unchanged from the previous Phase 1b slice,
- production mutable cache-entry field writes: 162, down from 228,
- production full-entry replacements: 0,
- `pkg/engine/asn.go`: 0 mutable cache-entry field writes, down from 37,
- `pkg/engine/geoloc.go`: 0 mutable cache-entry field writes, down from 29,
- remaining cache mutation hot spots are `cache/legacy.go`, `process.go`,
  `artifact_stage.go`, `finalize.go`, `runtime_ledger_cache.go`,
  `entry_timestamp_sanitize.go`, `rotation_stats.go`, `helpers.go`, and
  `legacy_failure_bootstrap.go`.

Measured Phase 1b source processing/finalization posture after implementation:

- source scope: 450 Go/TS/TSX files and 104,069 lines,
- `pkg/cache`: 5 files, 2,645 lines,
- `pkg/engine`: 121 files, 34,597 lines,
- production `Entry()` calls: 31, unchanged from the previous Phase 1b slice,
- production mutable cache-entry field writes: 107, down from 162,
- production full-entry replacements: 0,
- `pkg/engine/process.go`: 0 mutable cache-entry field writes, down from 28,
- `pkg/engine/finalize.go`: 0 mutable cache-entry field writes, down from 20,
- `pkg/engine/helpers.go`: 0 mutable cache-entry field writes, down from 7,
- remaining cache mutation hot spots are `cache/legacy.go`,
  `artifact_stage.go`, `runtime_ledger_cache.go`,
  `entry_timestamp_sanitize.go`, `rotation_stats.go`, and
  `legacy_failure_bootstrap.go`.

Measured Phase 1b artifact-materialization posture after implementation:

- source scope: 450 Go/TS/TSX files and 104,079 lines,
- `pkg/cache`: 5 files, 2,671 lines,
- `pkg/engine`: 121 files, 34,581 lines,
- production `Entry()` calls: 31, unchanged from the previous Phase 1b slice,
- production mutable cache-entry field writes: 80, down from 107,
- production full-entry replacements: 0,
- `pkg/engine/artifact_stage.go`: 0 mutable cache-entry field writes, down
  from 27,
- remaining cache mutation hot spots are `cache/legacy.go`,
  `runtime_ledger_cache.go`, `entry_timestamp_sanitize.go`,
  `rotation_stats.go`, and `legacy_failure_bootstrap.go`.

Measured Phase 1b runtime-ledger posture after implementation:

- source scope: 450 Go/TS/TSX files and 104,174 lines,
- `pkg/cache`: 5 files, 2,764 lines,
- `pkg/engine`: 121 files, 34,583 lines,
- production `Entry()` calls: 31, unchanged from the previous Phase 1b slice,
- production mutable cache-entry field writes: 60, down from 80,
- production full-entry replacements: 0,
- `pkg/engine/runtime_ledger_cache.go`: 0 mutable cache-entry field writes,
  down from 20,
- remaining cache mutation hot spots are `cache/legacy.go`,
  `entry_timestamp_sanitize.go`, `rotation_stats.go`, and
  `legacy_failure_bootstrap.go`.

Measured Phase 1b timestamp-repair posture after implementation:

- source scope: 450 Go/TS/TSX files and 104,275 lines,
- `pkg/cache`: 5 files, 2,916 lines,
- `pkg/engine`: 121 files, 34,532 lines,
- production `Entry()` calls: 31, unchanged from the previous Phase 1b slice,
- production mutable cache-entry field writes: 45, down from 60,
- production full-entry replacements: 0,
- `pkg/engine/entry_timestamp_sanitize.go`: 0 mutable cache-entry field
  writes, down from 15,
- remaining cache mutation hot spots are `cache/legacy.go`,
  `rotation_stats.go`, and `legacy_failure_bootstrap.go`.

Measured Phase 1b rotation-stats posture after implementation:

- source scope: 450 Go/TS/TSX files and 104,339 lines,
- `pkg/cache`: 5 files, 2,984 lines,
- `pkg/engine`: 121 files, 34,528 lines,
- production `Entry()` calls: 31, unchanged from the previous Phase 1b slice,
- production mutable cache-entry field writes: 30, down from 45,
- production full-entry replacements: 0,
- `pkg/engine/rotation_stats.go`: 0 mutable cache-entry field writes, down
  from 15,
- remaining cache mutation hot spots are `cache/legacy.go` and
  `legacy_failure_bootstrap.go`.

Measured Phase 1b legacy failure-start bootstrap posture after implementation:

- source scope: 450 Go/TS/TSX files and 104,402 lines,
- `pkg/cache`: 5 files, 3,046 lines,
- `pkg/engine`: 121 files, 34,529 lines,
- production `Entry()` calls: 31, unchanged from the previous Phase 1b slice,
- production mutable cache-entry field writes: 29, down from 30,
- production full-entry replacements: 0,
- `pkg/engine/legacy_failure_bootstrap.go`: 0 mutable cache-entry field
  writes, down from 1,
- the only remaining production mutable cache-entry field writes are 29 writes
  inside `pkg/cache/legacy.go`, which is already in the cache package.

Measured Phase 3 scheduler decomposition posture after implementation:

- source scope: 458 Go/TS/TSX files and 104,618 lines,
- `pkg/scheduler`: 16 files, 3,665 lines, 21 direct imports, 466 transitive
  dependencies,
- `pkg/scheduler/scheduler.go`: 276 lines, down from 1,474,
- largest scheduler implementation files after decomposition:
  - `snapshot_build.go`: 430 lines,
  - `processing_loop.go`: 264 lines,
  - `queue_admission.go`: 207 lines,
  - `metrics.go`: 183 lines,
  - `download_loop.go`: 85 lines,
- production mutable cache-entry field writes remain 29 and still only inside
  `pkg/cache/legacy.go`.

## Validation

Passed:

- `go test ./pkg/engine`
- `go test ./pkg/cache`
- `go test ./pkg/engine -run 'TestBootstrap|TestRepair|TestRunOnce|TestPipelineIntegrityScenarioCoreBranches'`
- `go test ./pkg/engine -run 'TestRunOnce|TestRunQueuedProcessingPromotesSuccessfulItemsAndRequeuesFailure|TestPipelineIntegrityScenarioCoreBranches|TestNewRepairsInvalidCachedTimestampsFromDisk'`
- `go test ./pkg/engine -run 'TestRunPersistsRunReasonAndProcessingDuration|TestRunOnceGeneratesCriticalInfrastructureArtifactsFromConfigSources|TestPipelineIntegrityScenarioCoreBranches|TestContentHashOnlyForCriticalInfrastructureSources'`
- `go test ./pkg/engine -run 'TestUnique|TestRunOnce|TestPipelineIntegrityScenarioCoreBranches'`
- `go test ./pkg/engine -run 'TestRunOnce|TestStaticConfigSourceProcessesFromYAML|TestRunOnceAndQuery|TestFullFeedReprocessTargetsIncludeHiddenFeeds|TestPipelineIntegrityScenarioCoreBranches'`
- `go test ./pkg/engine -run 'TestRunOnce|TestStaticConfigSourceProcessesFromYAML|TestRunOnceAndQuery|TestFullFeedReprocessTargetsIncludeHiddenFeeds|TestPipelineIntegrityScenarioCoreBranches|TestRunPersistsRunReasonAndProcessingDuration|TestContentHashOnlyForCriticalInfrastructureSources'`
- `go test ./pkg/engine -run 'TestBootstrap|TestRepair|TestRunOnce|TestPipelineIntegrityScenarioCoreBranches|TestContentHashOnlyForCriticalInfrastructureSources|TestNewReconcilesCachedMetadataFromConfigWithoutRefreshingStats'`
- `go test ./pkg/engine -run 'TestGeo|TestASN|TestRunOnce|TestPipelineIntegrityScenarioCoreBranches|TestProviderOnlyRunReportsEntityRefreshTargets|TestBuildPipelineRunPlan'`
- `go test ./pkg/engine -run 'TestRunOnce|TestRunPersistsRunReasonAndProcessingDuration|TestDerivativeRunReasonIsDependencyUpdate|TestPipelineIntegrityScenarioCoreBranches|TestStaticConfigSourceProcessesFromYAML|TestRunOnceEmptyUpdateReplacesPreviousSet|TestRunOnceUsesStagedSourceFile|TestHistoryUsesSourceTimestamp|TestContentHashOnlyForCriticalInfrastructureSources'`
- `go test ./pkg/engine -run 'TestBuildPipelineRunPlan|TestPipelineIntegrityScenarioCoreBranches|TestPipelineIntegrityScenarioBogonUpdateRefreshesEntitySidecars|TestRunOnceAndQuery|TestRunOnceBeforePublishHookRunsBeforePublication|TestRunOnceGeneratesCriticalInfrastructureArtifactsFromConfigSources'`
- `go test ./pkg/engine -run 'TestDroneBL|TestArtifact|TestFetchAndStageArtifactChild|TestRecoverStagedArtifacts|TestIntegrityRecoveryPlanRechecksArtifactParent|TestPipelineIntegrityScenarioCoreBranches'`
- `go test ./pkg/engine -run 'TestObserveHistoryPoint|TestRoundSecondsToMinutes|TestRunOnce|TestHistoryUsesSourceTimestamp|TestPipelineIntegrityScenarioCoreBranches'`
- `go test ./pkg/engine -run 'TestNewRepairsInvalidCachedTimestampsFromDisk|TestRepairEntryTimestampsFromDiskSkipsCleanEntries|TestPipelineIntegrityScenarioCoreBranches'`
- `go test ./pkg/engine -run 'TestRefreshRotationStatsFromLedgerComputesBoundedChangeRatio|TestRunOnce|TestPipelineIntegrityScenarioCoreBranches'`
- `go test ./pkg/engine -run 'TestBootstrapLegacyFailureStarts|TestPipelineIntegrityScenarioCoreBranches'`
- `go test ./pkg/cache ./pkg/engine ./tools/archposture`
- `go test ./tools/archposture`
- `go test ./pkg/scheduler`
- `go test ./pkg/web -run 'TestRunServesSplitAdminOnSeparateListeners|TestAdminAuthDisabledAllowsUnauthenticatedAccess'`
- `go test ./pkg/web -run 'TestSurfaceHandlerModesRegisterExpectedSurfaces|TestAPIEndpointsAndCORS|TestAdminAuthAndActions|TestRunServesSplitAdminOnSeparateListeners|TestAdminAuthDisabledAllowsUnauthenticatedAccess|TestCriticalInfrastructureRouteServesOnlyPublishedArtifacts|TestCountryAndASNAPIEndpointsServePrecomputedArtifacts'`
- `go test ./pkg/web`
- `make build`
- `make test`
- `make lint`
- `make race`
- `make bench`
- `./install.sh`
- `curl -fsS http://localhost:18888/healthz`
- `curl -fsS http://localhost:18888/api/v1/status`
- `curl -fsS http://localhost:18888/api/v1/admin/status`
- `systemctl is-active update-ipsets`
- `git diff --check`
- `rg -n 'entry\.[A-Za-z0-9_]+\s*=' pkg/engine/download_stage.go` returned
  no matches after the download-result migration.
- `rg -n 'entry\.[A-Za-z0-9_]+\s*=' pkg/engine/bootstrap_entries.go` returned
  no matches after the bootstrap/config seeding migration.
- `rg -n 'entry\.[A-Za-z0-9_]+\s*=' pkg/engine/asn.go pkg/engine/geoloc.go`
  returned no matches after the ASN/geolocation provider migration.
- `rg -n 'entry\.[A-Za-z0-9_]+\s*=' pkg/engine/process.go pkg/engine/finalize.go pkg/engine/helpers.go`
  returned no matches after the source processing/finalization migration.
- `rg -n 'entry\.[A-Za-z0-9_]+\s*=' pkg/engine/artifact_stage.go`
  returned no matches after the artifact-materialization migration.
- `rg -n 'entry\.[A-Za-z0-9_]+\s*=' pkg/engine/runtime_ledger_cache.go`
  returned no matches after the runtime-ledger migration.
- `rg -n 'entry\.[A-Za-z0-9_]+\s*=' pkg/engine/entry_timestamp_sanitize.go`
  returned no matches after the timestamp-repair migration.
- `rg -n 'entry\.[A-Za-z0-9_]+\s*=' pkg/engine/rotation_stats.go`
  returned no matches after the rotation-stats migration.
- `rg -n 'Entry\([^)]*\)\.[A-Za-z0-9_]+\s*=|entry\.[A-Za-z0-9_]+\s*=' pkg/engine/legacy_failure_bootstrap.go`
  returned no matches after the legacy failure-start migration.
- `pnpm --dir ui lint`
- `pnpm --dir ui build`

Notes:

- `pnpm --dir ui build` completed with existing Vite warnings about unresolved
  `InterDisplay` font paths being left for runtime resolution and the main JS
  chunk exceeding 500 KB. These are warnings, not validation failures.
- Installed-service smoke after Phase 3 returned `healthz=ok`, public status
  with `engine.running=false`, `systemctl is-active update-ipsets=active`, and
  admin status queue/metrics fields with zero active/waiting items after the
  latest run settled.
- No browser/Playwright visual smoke was added in Phase 5 because the repo does
  not currently have a frontend test harness and this phase intentionally kept
  the exported component APIs, URL-state keys, rendered section order, and CSS
  class structure unchanged.
- No runtime product behavior was intentionally changed by this SOW step.

## Outcome

Phase 0, Phase 1a, Phase 4a, Phase 4b, Phase 2, Phase 5, Phase 1b, and Phase
3 are complete.

The revised plan from the five independent read-only reviews is now backed by
a measurable architecture baseline, a guard test, a canonical architecture
posture spec, a future-review rule, durable project-skill updates, and
behavior-preserving pipeline, route-family, admin UI, cache ownership, and
scheduler ownership migrations.

Phase 1b removed all direct mutable cache-entry field writes from engine
production code; the remaining 29 production writes are inside
`pkg/cache/legacy.go`. Phase 3 reduced `pkg/scheduler/scheduler.go` from 1,474
lines to 276 while preserving same-package queue ownership and runtime
behavior.

Deferred/non-goals:

- Phase 6 package boundary enforcement remains deferred because the same-package
  ownership cleanup now provides the immediate maintainability value without
  introducing package cycles or interface shims.
- Splitting `pkg/cache/legacy.go` internally is not necessary for this SOW
  because the remaining mutation count is already inside the cache package and
  no engine ownership leak remains.

## Lessons extracted

- Subjective architecture grades are not enough. Future architecture SOWs must
  define metrics before claiming posture improvement.
- Cache mutation ownership is a major refactor. It needs inventory and semantic
  API design before implementation.
- Route extraction is security-sensitive when public/admin listeners, raw feed
  serving, redistributability, and stale artifact checks share one handler.
- Broad refactor plans need guardrail tests before code movement. The posture
  baseline is now the acceptance gate for silent growth in large files/functions,
  production cache-entry mutation, semantic shortcut matches, and `pkg/iprange`
  imports.
- Engine refactors must preserve resource lifecycle and generated-file mtimes,
  not only generated content.
- After a large function is reduced, the architecture baseline must be refreshed
  in the same SOW so the old shape is not still accepted as the future debt
  ceiling.
- Route refactors must preserve listener exposure, auth policy, raw-feed
  serving rules, and stale artifact checks as observable contracts, not only
  compile-time route registration.
- UI decomposition should preserve the exported component API and move one
  coherent concern at a time: URL/state orchestration, pure model logic,
  presentation rows, actions, data-query sections, and shared primitives.
- Non-behavior maintenance choices are assistant-owned in this project. Future
  work should proceed on internal ordering/decomposition decisions and reserve
  user questions for behavior, external design, risk acceptance, destructive
  operations, or production actions.
- Scheduler decomposition must preserve one queue authority. Splitting files is
  acceptable only while `Runner.stateMu` remains the shared lock for download
  and processing queue maps, and tests cover dedupe, refetch/deferred handling,
  provider-default enqueue, staged recovery, and race behavior.
