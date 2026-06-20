# SOW-0108 - IPrange Hot-Path Boundary Cleanup

## Status

Status: completed

Sub-state: implementation complete; validation complete with one unrelated
pre-existing architecture-posture blocker recorded

## Requirements

### Purpose

Make the application fit for high-volume feed processing by ensuring generic
range/set algebra in engine hot paths is owned by the high-performance Go
`pkg/iprange` package, not by engine-local workarounds.

### User Request

The user approved continuing after the retention hotfix to find all other engine
areas that should be handled by the Go `pkg/iprange` package, then replace
custom engine workarounds with benchmarked, proven, high-performance Go
`iprange` implementations in application hot paths. The user explicitly wants
the Go package, not shelling out to the C `iprange` command.

### Assistant Understanding

Facts:

- SOW-0107 moved retention cohort reconciliation to
  `pkg/iprange.CompareNextSources`.
- SOW-0107 same-failure scan identified additional package-boundary candidates
  in engine helpers.
- `pkg/iprange` already owns `RangeSource`, `FileSet`, `IntersectIter`,
  `ExcludeIter`, `UnionIter`, `OverlapCountIter`, and `CompareNextSources`.
- Engine still contains reusable range-source equality, materialization/counting,
  bounds, prefix/filter, and adapter helpers.

Inferences:

- Generic `RangeSource` operations should be exported from `pkg/iprange` so the
  engine can orchestrate processing without owning low-level range algorithms.
- Engine-specific scheduling, operator progress, artifact semantics, JSON
  payloads, and provider/database attribution should stay in `pkg/engine`.

Unknowns:

- None that block implementation. Candidate scope can be refined by source
  evidence and benchmarks during execution.

### Acceptance Criteria

- Engine hot-path helpers that are generic range-source algorithms are moved to
  `pkg/iprange` exported APIs with context and range-source error handling where
  needed.
- Engine call sites use the new `pkg/iprange` APIs instead of custom workaround
  helpers.
- Engine domain logic remains in engine.
- New `pkg/iprange` APIs have behavioral tests that cover in-memory `IPSet` and
  file-backed `FileSet` sources where relevant.
- Benchmarks prove the new APIs preserve high-performance cost shape on
  realistic range-source sizes.
- A same-failure scan shows no remaining engine-local generic `RangeSource`
  workaround in hot paths unless explicitly documented as domain-specific.
- Specs/skills are updated if a durable package-boundary rule is needed.

## Analysis

Sources checked:

- `.agents/sow/done/SOW-0107-20260620-retention-reconciliation-hotfix.md`
- `pkg/iprange/iter_ops.go`
- `pkg/iprange/set_ops.go`
- `pkg/iprange/fileset.go`
- `pkg/engine/fileset_helpers.go`
- `pkg/engine/feed_body_stage.go`
- `pkg/engine/output_comparison_helpers.go`
- `pkg/engine/output_comparison.go`
- `pkg/engine/asn.go`
- `pkg/engine/bogons.go`
- `pkg/engine/critical_feed_writer.go`
- `pkg/engine/public.go`
- `pkg/engine/query.go`
- `.agents/sow/specs/processing-engine.md`
- `.agents/sow/specs/memory-management.md`
- `.agents/sow/specs/operating-principles.md`

Current state:

- `pkg/engine/feed_body_stage.go:230` implements `rangeSourcesEqual` with two
  `iprange.ExcludeIter` scans and no context/error reporting.
- `pkg/engine/fileset_helpers.go:149` and `pkg/engine/fileset_helpers.go:169`
  implement context-aware iterator materialization/counting outside `pkg/iprange`.
- `pkg/engine/output_comparison_helpers.go:97` implements range-source bounds.
- `pkg/engine/output_comparison_helpers.go:153` implements range-source
  signatures, prefix occupancy filters, sparse prefix filters, and content
  hashes in engine.
- `pkg/engine/home_detail_helpers.go:25` implements a generic iterator-backed
  `RangeSource` adapter in engine.
- `pkg/engine/output_comparison.go:305`, `pkg/engine/query.go:451`,
  `pkg/engine/public.go:382`, `pkg/engine/asn.go:223`,
  `pkg/engine/bogons.go:236`, and `pkg/engine/critical_feed_writer.go:125`
  already use `pkg/iprange` iterator primitives directly.

Risks:

- Moving unexported comparison filter types can accidentally widen public API
  without a clean contract. Keep API names focused on range-source semantics.
- Prefix/filter shortcuts must remain conservative; false disjoint/identity
  results would corrupt public comparison, bogon, critical, and ASN facts.
- Replacing helpers used by tests can create implementation-coupled tests. Add
  package-level `iprange` tests for generic behavior and keep engine tests
  focused on observable artifacts.
- Benchmarks can be noisy. Use repeated benchmark runs and record only relevant
  signals in this SOW.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The retention bug showed that engine-local range comparison logic can become
  the wrong abstraction and miss optimized `pkg/iprange` capabilities.
- The same pattern still exists in other hot paths: generic range-source
  equality, collection/counting, bounds, signatures, and filters live in
  `pkg/engine` even though they are not engine-specific.
- This is not a request to rewrite the engine. It is a focused package-boundary
  cleanup so hot paths use reusable Go `pkg/iprange` primitives.

Evidence reviewed:

- SOW-0107 same-failure scan listed the candidate helpers.
- `pkg/iprange/iter_ops.go` already contains streaming iterator algebra.
- `pkg/iprange/fileset.go` exposes file-backed `RangeSource` access and `Err`.
- Engine files listed in Analysis contain the remaining generic helpers.
- `.agents/sow/specs/memory-management.md` already requires file-backed or
  iterator-based set operations and conservative overlap filters.

Affected contracts and surfaces:

- Go package API: `pkg/iprange`.
- Engine internals: comparison, bogon, ASN precompute, critical overlap,
  compose, retention, history derivative, and tests using generic range-source
  helpers.
- Public/output semantics should not change.
- No public UI, API schema, config, file layout, or operator workflow should
  change.

Existing patterns to reuse:

- `pkg/iprange.RangeSource`, `FileSet`, `Err`, `CountUniqueIter`,
  `OverlapCountIterContext`, `CompareNextSources`.
- Engine `latestSetCache` ownership for opened file-backed sets.
- Existing engine comparison tests in
  `pkg/engine/output_comparison_optimization_test.go`.
- Existing iprange iterator validation and stress tests.

Risk and blast radius:

- Medium code blast radius: shared `pkg/iprange` plus several engine call sites.
- Low product blast radius if behavior remains unchanged and tests compare
  existing outputs.
- High correctness risk for overlap filter shortcuts if implemented incorrectly.
- Moderate performance risk if new APIs add allocation or extra scans; mitigated
  with benchmarks and cost-shape tests.

Sensitive data handling plan:

- Work uses synthetic ranges and local source-code evidence only.
- Durable artifacts must not include raw production logs, customer identifiers,
  non-private customer-identifying IPs, secrets, credentials, bearer tokens,
  SNMP communities, private endpoints, or proprietary incident details.
- Benchmarks will use generated private/test ranges, not production feed data.

Implementation plan:

1. Add generic `pkg/iprange` APIs for context-aware collection/counting,
   range-source equality, range-source error access, range-source bounds, and
   reusable overlap/filter signatures.
2. Replace engine-local helpers and call sites with those APIs while keeping
   engine orchestration, metrics, payloads, and provider/database logic in
   engine.
3. Add or migrate tests and benchmarks to `pkg/iprange`; keep engine tests
   focused on observable behavior and cost shape.
4. Search for remaining engine-local generic range-source helpers and document
   any intentionally domain-specific leftovers.

Validation plan:

- `go test -count=1 ./pkg/iprange`
- `go test -count=1 ./pkg/engine`
- Targeted shuffled engine tests for comparison, compose, retention, ASN,
  bogon, and critical paths.
- `go test -bench='(RangeSource|Overlap|Collect|Compare)' -benchmem -count=10 ./pkg/iprange`
- `make build`
- `git diff --check`
- SOW audit for SOW-0108.

Artifact impact plan:

- AGENTS.md: likely no update; project-wide rule already says `pkg/iprange`
  stays standalone.
- Runtime project skills: likely update `project-coding` with the extracted
  lesson that generic range-source hot-path algorithms belong in `pkg/iprange`.
- Specs: likely no product behavior change; if a durable memory/performance
  contract gap is found, update `.agents/sow/specs/memory-management.md`.
- End-user/operator docs: no expected update; behavior and operation are
  unchanged.
- End-user/operator skills: no expected update.
- SOW lifecycle: separate SOW from SOW-0106 and SOW-0107; complete and move to
  `.agents/sow/done/` only when validation is complete.

Open-source reference evidence:

- None checked. This work is local package-boundary cleanup over this project's
  existing `pkg/iprange` APIs, not a protocol or external library design.

Open decisions:

- None blocking. The user already approved the design direction: improve the Go
  `pkg/iprange` package first, then replace engine workarounds in hot paths.

## Implications And Decisions

1. Package-boundary strategy
   - Option A - Surgical: only move the exact helpers flagged in SOW-0107.
     Benefit: lower blast radius. Risk: leaves the same pattern in other
     comparison/provider hot paths.
   - Option B - Long-term-best: move all generic hot-path range-source
     algorithms found in this pass into `pkg/iprange`, then replace engine
     call sites. Benefit: fixes the class of workaround, not one symptom. Risk:
     broader API and benchmark surface.
   - Selected: Option B, by user objective. This is the correct fit because the
     explicit goal is "all other areas" and "all hot paths", not a narrow fix.

2. C `iprange` command use
   - Option A - Shell out to C command from engine.
   - Option B - Use and improve the Go `pkg/iprange` package.
   - Selected: Option B, explicitly requested by the user.

3. SOW separation
   - Option A - Blend this into SOW-0106 engine core redesign.
   - Option B - Keep this as a separate focused SOW.
   - Selected: Option B. The user explicitly rejected blending the hotfix into
     SOW-0106, and this is a direct follow-up to SOW-0107.

## Plan

1. Inventory and classify engine-local range-source helpers.
2. Implement `pkg/iprange` APIs for the generic helpers.
3. Replace engine call sites and remove generic helper duplication.
4. Add package tests, engine behavioral/cost-shape tests, and benchmarks.
5. Validate, update durable artifacts, and record same-failure scan.

## Execution Log

### 2026-06-20

- Created SOW-0108 after completing and pushing the SOW-0107 retention hotfix.
- Confirmed unrelated SOW-0106 worktree files exist and will remain out of this
  SOW.
- Completed initial source inventory and pre-implementation gate.
- Added `pkg/iprange` APIs for generic range-source hot paths:
  `RangeSourceFromIter`, `CollectIterContext`, `CountIterContext`,
  `RangeSourceErr`, `RangeSourceUniqueIPs`, `RangeSourceBoundsContext`,
  `RangeSourceContentHashContext`, `BuildRangeSourceSummaryContext`,
  `BuildRangeOverlapFilterContext`, and `RangeSourcesEqualContext`.
- Updated `CompareNextSources` internals to reuse the new range-source
  unique-count/error helpers.
- Replaced engine-local generic range helpers in retention, public compose,
  feed history, merge/history derivative composition, output comparison,
  bogon, ASN, critical-infrastructure, and home-detail range-source adapter
  paths.
- Removed the engine-local comparison signature/filter implementation from
  `pkg/engine/output_comparison_helpers.go`; output comparison now consumes
  `iprange.RangeSourceSummary`, `RangeOverlapFilter`, and `RangeContentHash`.
- Removed the engine-local iterator collection/counting helpers from
  `pkg/engine/fileset_helpers.go`; materialization/counting now goes through
  `pkg/iprange`.
- Removed the remaining engine-local generic range-source content hash loop;
  critical content hashing now calls `iprange.RangeSourceContentHashContext`.
- Threaded processing/run context through `appendHistorySnapshot` and
  `finalize` so moved range scans remain cancellable in hot paths.
- Replaced remaining engine non-context `iprange.OverlapCountIter` hot-path
  calls with `iprange.OverlapCountIterContext` in ASN and bogon comparison
  paths.
- Added `pkg/iprange` behavioral tests and benchmarks for the moved APIs.
- Updated `.agents/sow/specs/memory-management.md` and
  `.agents/skills/project-coding/SKILL.md` with the durable package-boundary
  rule.

## Validation

Acceptance criteria evidence:

- Generic helpers moved:
  - `pkg/iprange/range_source.go` owns context-aware collection/counting,
    source errors, unique counts, bounds, content hashes, summaries, overlap
    filters, iterator adapters, and exact equality.
  - `pkg/iprange/set_ops.go` reuses the exported range-source helpers in
    `CompareNextSources`.
- Engine call sites replaced:
  - `pkg/engine/retention_update.go` uses `iprange.CollectIterContext`,
    `CountIterContext`, and `CompareNextSources`.
  - `pkg/engine/public.go` and `pkg/engine/feed_body_stage.go` use
    `iprange.CollectIterContext` and `RangeSourcesEqualContext`.
  - `pkg/engine/output_comparison.go` uses
    `iprange.BuildRangeSourceSummaryContext` and `RangeOverlapFilter`.
  - `pkg/engine/bogons.go`, `pkg/engine/asn.go`, and
    `pkg/engine/critical_feed_writer.go` use
    `iprange.BuildRangeOverlapFilterContext`.
  - `pkg/engine/finalize.go` and `pkg/engine/bootstrap_entries.go` use
    `iprange.RangeSourceContentHashContext`.
  - `pkg/engine/asn.go` and `pkg/engine/bogons.go` use
    `iprange.OverlapCountIterContext` for remaining exact overlap counts.
- Domain-specific engine loops retained:
  - geolocation/ASN attribution loops remain in engine because they join range
    streams to provider databases and output product-specific aggregate rows.
  - critical-infrastructure overlap writer loops remain in engine because they
    write tier/provider aggregate artifacts while scanning exact intersections.

Tests or equivalent validation:

- Passed: `go test -count=1 ./pkg/iprange`
- Passed: `go test -count=1 ./pkg/engine`
- Passed: `go test -count=1 ./pkg/iprange ./pkg/engine`
- Passed: `make build`
- Passed: `make lint`
- Passed: `make test-strict`
- Passed: `go test -race -count=1 ./pkg/iprange ./pkg/engine`
- Passed: `git diff --check`
- Passed: `go test -run '^$' -bench='(CompareNextSources|CollectIterContext|RangeSourcesEqualContext|BuildRangeSourceSummary|RangeSourceContentHash)' -benchmem -count=10 ./pkg/iprange`
- Failed with unrelated pre-existing gate: `make test` ran all root Go
  packages successfully through `pkg/web`, then failed only in
  `tools/archposture` because unchanged `ui/src/lib/api-types.ts` is 1099
  lines while the architecture baseline is 1045 lines. `git status --short`
  and `git diff --name-only -- ui/src/lib/api-types.ts tools/archposture`
  showed this SOW did not touch that UI file or the archposture baseline.
- Representative benchmark evidence:
  - `BenchmarkCompareNextSourcesFileSet/n=100000`: about 15.4-17.6 ms/op,
    about 3504 B/op, 53 allocs/op.
  - `BenchmarkRangeSourcesEqualContextFileSet/n=100000`: about 15.2-17.8
    ms/op, 448 B/op, 14 allocs/op.
  - `BenchmarkRangeSourceContentHashFileSet/n=100000`: about 1.3-1.7 ms/op,
    368 B/op, 8 allocs/op.
  - `BenchmarkBuildRangeSourceSummaryFileSet/n=100000`: about 1.9-2.0 ms/op,
    about 272705 B/op, 28 allocs/op.

Real-use evidence:

- Built the application successfully with `make build` after the final hot-path
  edits. Installed-service validation was not run because this SOW changed
  internal range-source algorithms and package boundaries, not service install,
  configuration, public routes, file layout, or operator workflow.

Reviewer findings:

- External reviewers were not run. The user did not explicitly request external
  reviewers for this SOW, and project instructions restrict external assistant
  runs to explicit user requests.

Same-failure scan:

- Passed: `rg -n "collectIter\\(|countUniqueIter\\(|rangeSourcesEqual\\(|rangeOverlapFilter|buildRangeOverlapFilter|buildComparisonSetSignature|comparisonPrefixOverlap|comparisonSparsePrefixOverlap|rangeSourceBounds|buildComparisonPrefixBitmap|rangeSourceContentHash|ipSetContentHash|OverlapCountIter\\(" pkg/engine --glob '*.go'`
  returned no matches.
- Remaining `for range src.Iter()` and `iprange.IntersectIter` loops in engine
  were reviewed and classified as domain-specific geolocation, ASN, and
  critical-infrastructure attribution/writer logic, not generic range-source
  algorithms.

Sensitive data gate:

- Passed. Work used synthetic ranges and local source-code evidence only. No
  production logs, customer identifiers, customer-identifying public IPs,
  credentials, bearer tokens, SNMP communities, private endpoints, or
  proprietary incident details were written to durable artifacts.

Artifact maintenance gate:

- AGENTS.md: no update needed; project-wide rules already state
  `pkg/iprange` must remain standalone.
- Runtime project skills: updated `.agents/skills/project-coding/SKILL.md`
  with the generic `RangeSource` hot-path boundary rule.
- Specs: updated `.agents/sow/specs/memory-management.md` with the normative
  package-boundary rule for generic range-source algorithms.
- End-user/operator docs: no update needed; public behavior, configuration,
  routes, and operator workflows are unchanged.
- End-user/operator skills: no update needed; no external operator workflow
  changed.
- SOW lifecycle: completed; move to `.agents/sow/done/` with the implementation
  commit.

Specs update:

- Updated `.agents/sow/specs/memory-management.md`.

Project skills update:

- Updated `.agents/skills/project-coding/SKILL.md`.

End-user/operator docs update:

- Not needed.

End-user/operator skills update:

- Not needed.

Lessons:

- Generic range/set algorithms are a package-boundary concern. Engine-local
  implementations are easy to make correct for one call site but hard to keep
  optimized across retention, comparison, compose, and provider-reference
  paths.

Follow-up mapping:

- No deferred valid item identified. The only validation blocker is unrelated
  pre-existing `tools/archposture` drift for `ui/src/lib/api-types.ts`; it is
  not part of this SOW and is not a deferral from this work.

## Outcome

Completed.

The engine-wide hot-path audit found and removed the generic engine-local
range-source workarounds in scope for this SOW. Reusable comparison,
collection/counting, content hashing, bounds/summary, overlap-filter, iterator
adapter, and exact equality logic now lives in standalone `pkg/iprange`.
Engine code uses those APIs and retains only domain-specific provider,
artifact, and attribution orchestration.

## Lessons Extracted

- Generic range-source work should be added to `pkg/iprange` first, then wired
  into engine. Engine-local copies hide performance/cancellation/error
  semantics and make the same bug class recur across retention, comparison,
  compose, and provider-reference paths.

## Followup

None for this SOW.

## Regression Log

None yet.
