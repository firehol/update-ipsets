# SOW-0081 - Go Test Artifact Invariant And Bench Coverage

## Status

Status: completed

Sub-state: closed

## Requirements

### Purpose

Track remaining Go behavioral-testing gaps from SOW-0039 that were still valid
but not represented by a concrete pending SOW.

### User Request

Review project quality against the four named project skills, identify gaps,
create SOWs for actionable improvements, and iterate the audit until valid
findings are fixed or tracked.

### Assistant Understanding

Facts:

- Iterative audit cycle 5 found SOW-0039 still listed valid `NOT FIXED`
  testing gaps without a concrete pending SOW path.
- The unmapped gaps are:
  - integrity timestamp invariant coverage for generated artifacts,
  - golden-file/update-pattern coverage for durable public artifacts,
  - benchmark or performance guard coverage for engine hot paths.

Inferences:

- These are test-quality improvements, not immediate behavior defects.
- The work should be split or rejected with evidence if one item proves too
  broad for one focused SOW.

Unknowns:

- Which generated artifacts are stable enough for golden files without making
  feed catalog churn painful.
- Which engine hot paths should receive benchmark thresholds versus ordinary
  benchmark visibility.

### Acceptance Criteria

- Review the SOW-0039 B8/B13/B14 findings against current tests.
- Add or explicitly reject an invariant test that verifies generated artifact
  mtimes respect their contributing input timestamps.
- Add or explicitly reject a golden-file/update pattern for stable public
  artifacts such as sitemap, robots, llms, entity indexes, or selected API
  payloads.
- Add or explicitly reject benchmark/performance guard coverage for selected
  engine hot paths, with evidence for any threshold or non-threshold choice.
- Update SOW-0039 follow-up mapping so these items are no longer prose-only
  leftovers.
- Run focused package tests plus `make test`.

## Analysis

Sources checked:

- `project-go-behavioral-testing`
- `project-testing`
- Iterative audit cycle 5 Go behavioral-testing findings
- `.agents/sow/done/SOW-0039-20260501-go-test-re-review.md`

Current state:

- B8 was still a valid gap. `pkg/engine/file_contract_test.go` already had a
  narrow mtime assertion for metadata/history/changesets/retention, but
  `pkg/engine/pipeline_integrity_scenario_test.go` had no scenario-wide pass
  over every expected generated secondary artifact after each pipeline step.
- B13 was still valid. There were no `*.golden` files and no `-update` golden
  refresh flag in the main module.
- B14 was still valid for engine hot paths. Benchmarks existed for
  `pkg/iprange` and `pkg/processor`, but none for the engine effective-entry
  resolver path that protects batch views from repeated full cache snapshots.

Risks:

- Golden files can become churn-heavy if they cover unstable catalog data.
- Bench thresholds can become flaky across workstations if hardware variance is
  not considered.

## Plan

1. Re-check B8/B13/B14 against current tests and artifacts.
2. Implement narrow high-value coverage where stable.
3. Split or reject any item that needs a separate product/performance decision.
4. Update SOW-0039 mapping and validation evidence.

## Execution Log

### 2026-05-01

- Created from iterative audit cycle 5.
- Moved to current for assistant-owned test-quality cleanup.
- Added `assertGeneratedArtifactMTimeInvariant()` to the pipeline integrity
  scenario. It runs after each step, walks processed public feeds through the
  configured expected secondary artifact descriptors, and fails if any
  generated artifact mtime is older than the feed `ProcessedDate`.
- Added `pkg/engine/testdata/robots.golden` and
  `pkg/engine/testdata/llms.golden` plus a package-level `-update` flag for
  stable public metadata artifact snapshots.
- Added `BenchmarkEffectiveEntryResolverBatchView` with allocation reporting.
  The benchmark covers batch construction/use of `effectiveEntryResolver`
  across parent, peer, retention-derivative, and merge-derivative entries.
- Rejected fixed ns/op or alloc thresholds in tests. Reason: benchmark
  thresholds would be hardware- and load-sensitive on developer workstations;
  the benchmark provides repeatable `go test -bench ... -benchmem` visibility
  without turning normal unit tests flaky.

## Validation

Acceptance criteria evidence:

- B8: `pkg/engine/pipeline_integrity_scenario_test.go` now checks the
  generated-artifact mtime invariant after every scenario step.
- B13: `pkg/engine/output_test.go` now has an `-update` golden-file pattern
  covering `robots.txt` and `llms.txt`.
- B14: `pkg/engine/effective_entry_bench_test.go` adds benchmark visibility
  for the engine effective-entry resolver hot path. Thresholds were explicitly
  rejected as flaky for normal test gates.
- SOW-0039 follow-up mapping updated with this SOW's closure addendum.
- Focused package tests and full `make test` passed.

Reviewer findings:

- Go behavioral-testing review found SOW-0039's B8/B13/B14 leftovers were not
  represented by concrete pending SOWs.

Validation commands:

- `go test ./pkg/engine -run 'TestPipelineIntegrityScenario|TestWritePublicMetadataFilesBuildsSitemapIndexAndDetailShards'` — passed.
- `go test ./pkg/engine -run '^$' -bench BenchmarkEffectiveEntryResolverBatchView -benchmem` — passed; sample result `493351 ns/op`, `742626 B/op`, `3005 allocs/op` on this workstation.
- `go test ./pkg/engine ./pkg/web` — passed.
- `make test` — passed.
- `git diff --check` — passed.

Artifact maintenance gate:

- `AGENTS.md`: not updated. Reason: no project-wide workflow or guardrail changed.
- Runtime project skills: not updated. Reason: the existing testing skill already
  covers golden files and benchmark commands.
- Specs: not updated. Reason: this SOW changes test coverage only, not product
  behavior or runtime contracts.
- End-user/operator docs: not updated. Reason: no user/operator surface changed.
- End-user/operator skills: not updated. Reason: no exported operator workflow changed.
- SOW lifecycle: SOW-0081 moved pending -> current -> done; SOW-0039 residual
  mapping updated for B8/B13/B14.

## Outcome

Completed.

Shipped changes:

- Added a scenario-wide generated-artifact mtime invariant to the pipeline
  integrity test harness.
- Added golden files and a `-update` refresh path for stable public metadata
  artifacts (`robots.txt`, `llms.txt`).
- Added benchmark/allocation visibility for the engine effective-entry resolver
  batch path.

No pending follow-up SOW was created from this work.
