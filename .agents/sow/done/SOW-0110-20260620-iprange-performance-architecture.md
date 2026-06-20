# SOW-0110 - pkg/iprange Performance Architecture Repair

## Status

Status: completed

Sub-state: Implementation, focused validation, artifact updates, and follow-up
mapping complete. Moved to `.agents/sow/done/` as part of the implementation
commit.

## Requirements

### Purpose

Make `pkg/iprange` fit for heavy update-ipsets production use: standalone,
telemetry-framework agnostic, allocation-light, CPU-efficient, behaviorally
tested, and safe for large feed processing without hidden hot-path costs.

Hard priority rule: `pkg/iprange` is optimized for performance and accuracy
above all other concerns. Hot paths must be allocation-storm free. They should
be zero-allocation where possible, or have an extremely low measured allocation
rate only when allocation cannot be avoided without sacrificing correctness.
Any feature, instrumentation, convenience API, or abstraction that contradicts
performance or accuracy must be dropped from the hot path or implemented in a
different layer.

This SOW is intentionally separate from `SOW-0106` engine redesign work. It
must be executed as focused `pkg/iprange` and direct-caller repair, not blended
into a broad engine rewrite.

### User Request

The user asked to create a SOW to fix all identified `pkg/iprange` allocation
storms and inefficient operations. The required process is:

- ensure tests exist before each code change;
- tests must be behavioral, outside-in, and include corner cases;
- optimize the code only after the behavioral tests and benchmark baselines
  exist;
- rerun tests and benchmarks after each change class;
- make `pkg/iprange` telemetry agnostic: hot paths should compute local plain
  counters/stats, and callers should decide how to export those stats.
- make `pkg/iprange` allocation-storm free by design: performance and accuracy
  are mandatory and outrank instrumentation, convenience, or implementation
  fashion.

### Assistant Understanding

Facts:

- `pkg/iprange` currently imports OpenTelemetry directly in `pkg/iprange/otel.go`.
- `pkg/iprange` currently records OTel counters/spans internally.
- Hot paths call telemetry directly, including `IPSet.AddRange`, `IPSet.Contains`,
  and file-backed `Contains`.
- Benchmarks showed per-lookup allocation from `Contains`, parser allocation
  storms, generic iterator fallback storms, and expensive source summaries.
- The earlier source-level set-algebra APIs improved production union,
  intersection, exclusion, and exclusion-count paths, but additional slow-by-design
  paths remain.

Inferences:

- The package boundary is wrong: `pkg/iprange` should be a standalone library
  with plain stats, not an OTel-aware component.
- Repeated hot-path framework calls and generic iterator fallback are structural
  performance risks, not one-off slow spots.
- Fixing this properly requires tests, benchmarks, API design, caller updates,
  and durable project-skill/spec updates.
- The package should reject designs that make hot paths easier to observe or
  more convenient at the cost of measurable allocation storms or accuracy risk.

Unknowns:

- Exact final speedups require before/after `benchstat` across representative
  operations.
- Whether all legacy compare/count APIs should be kept as compatibility wrappers
  or replaced by source-level implementations needs implementation-time API
  review, but the behavior must stay compatible.

### Acceptance Criteria

- `pkg/iprange` no longer imports OpenTelemetry packages.
  Verification: `rg -n "go.opentelemetry.io|otel\\." pkg/iprange` returns no
  production-code matches.
- `pkg/iprange` exposes plain operation stats/counters for material operations
  that callers can convert to OTel, logs, admin metrics, or ignore.
  Verification: behavioral tests prove callers receive correct stats for parse,
  set algebra, file-backed lookup, and write operations.
- Hot paths do not allocate due to telemetry.
  Verification: `BenchmarkSetContains` and `BenchmarkFileSetContains` report
  zero allocations from the lookup operation itself, or the SOW records a
  measured unavoidable allocation with evidence.
- All identified hot paths are allocation-storm free.
  Verification: benchmarks for parse, add, lookup, compare, source algebra,
  summary/hash, binary read/write, and IPv6 equivalents where touched report
  zero allocations where possible, or documented extremely low allocation rates
  with proof that the remaining allocations are unavoidable without harming
  accuracy.
- Text parsing allocations are materially reduced while preserving lenient C
  iprange-compatible behavior.
  Verification: behavioral parser tests cover comments, BOM, CIDR, ranges,
  short IPv4 forms, invalid lines, hostnames, IPv6-in-IPv4 mode, prefix masks,
  and EOF-without-newline; `BenchmarkParseIPs` improves materially.
- Old compare APIs no longer materialize and sort per pair when overlap counts
  can be computed by direct scans.
  Verification: `CompareAll`, `CompareNext`, and `CompareFirst` behavioral tests
  continue to pass; benchmarks show reduced allocations versus the current
  `Combine().Optimize()` implementation.
- Generic iterator fallback remains correct but production callers avoid it for
  package-owned source types.
  Verification: tests cover both indexed and fallback `RangeSource` inputs,
  including `RangeSourceFromIter`.
- Source summary/filter building avoids repeated large allocations where the
  engine reuses the same opened set during one heavy phase.
  Verification: engine behavioral tests or package-level source-summary tests
  prove repeated use returns the same public output and benchmark allocation
  impact is reduced.
- FileSet opening and validation remain safe while repeated open/scan costs are
  bounded through caller caching or explicit validated-open semantics.
  Verification: corrupt binary files are still rejected; engine cache tests prove
  repeated heavy-phase opens do not repeatedly validate the same file.
- IPv6 hot paths receive parity where they are part of supported CLI/library
  behavior.
  Verification: IPv6 behavioral tests and benchmarks cover compare, overlap,
  union, intersection, exclusion, parse, write, and FileSet lookup surfaces.
- All changed Go tests are behavioral and black-box by default.
  Verification: test files use external test packages where feasible; same-package
  tests are justified only for internal algorithm contracts that cannot be
  observed efficiently through exported APIs.

## Analysis

Sources checked:

- `pkg/iprange/otel.go`
- `pkg/iprange/set.go`
- `pkg/iprange/fileset.go`
- `pkg/iprange/parse.go`
- `pkg/iprange/text_reader.go`
- `pkg/iprange/range.go`
- `pkg/iprange/iter_ops.go`
- `pkg/iprange/materialize.go`
- `pkg/iprange/range_source.go`
- `pkg/iprange/set_ops.go`
- `pkg/iprange/iter6_ops.go`
- `pkg/iprange/set6_ops.go`
- `pkg/engine/query.go`
- `pkg/engine/output_comparison.go`
- `pkg/engine/bogons.go`
- `pkg/engine/asn.go`
- `pkg/engine/critical.go`
- `pkg/engine/retention_update.go`
- Project skills: `project-content-surfaces`, `project-testing`,
  `project-go-behavioral-testing`, `project-go-best-practices`, `project-coding`.

Current state:

- `pkg/iprange/otel.go:48` records directly to OTel and allocates metric
  attribute slices in `iprangeMetricAttributes`.
- `pkg/iprange/set.go:44` calls telemetry for every `AddRange`.
- `pkg/iprange/set.go:160` and `pkg/iprange/fileset.go:336` call telemetry
  twice for every `Contains`.
- `pkg/iprange/parse.go:131`, `pkg/iprange/text_reader.go:14`, and
  `pkg/iprange/range.go:119` use string-heavy parsing that allocates per line
  and per token.
- `pkg/iprange/set_ops.go:312`, `pkg/iprange/set_ops.go:342`, and
  `pkg/iprange/set_ops.go:465` compare sets through `Combine().Optimize()`.
- `pkg/iprange/iter_ops.go:69`, `pkg/iprange/iter_ops.go:124`, and
  `pkg/iprange/iter_ops.go:292` use generic `iter.Pull` paths that are correct
  but too expensive for package-owned hot sources when materialized.
- `pkg/iprange/materialize.go:15` correctly routes known source types through
  indexed fast paths but still falls back to generic iterator collection for
  unknown `RangeSource` implementations.
- `pkg/iprange/range_source.go:312` builds source summaries with a large prefix
  bitmap and content hash scan.
- `pkg/iprange/fileset_mmap.go:90` and `pkg/iprange/fileset_pread.go:43`
  validate all ranges on each `OpenFileSet`.
- `pkg/iprange/iter6_ops.go:28` and `pkg/iprange/set6_ops.go:312` show IPv6
  still uses older generic compare/iterator patterns.

Measured benchmark evidence from 2026-06-20 local runs:

- `BenchmarkFileSetContains`: about `848 B/op`, `10 allocs/op` per lookup.
- `BenchmarkSetContains`: about `848 B/op`, `10 allocs/op` per lookup.
- `BenchmarkParseIPs`: about `5.46 MB/op`, `70,039 allocs/op` for 10k IP lines.
- `BenchmarkCollectIterContextFileSetUnion/n=100000`: about `93 MB/op`,
  about `998k allocs/op`.
- `BenchmarkUnionSourcesContextFileSet/n=100000`: about `1.6 MB/op`,
  `27 allocs/op`.
- `BenchmarkBuildRangeSourceSummaryFileSet/n=100000`: about `272 KB/op`,
  `28 allocs/op`.
- `BenchmarkRangeSourceContentHashFileSet/n=100000`: about `368 B/op`,
  `8 allocs/op`.

Risks:

- Removing OTel from `pkg/iprange` changes telemetry ownership. Engine and CLI
  callers must preserve useful operator metrics through their own telemetry
  layer.
- Parser optimization can regress lenient compatibility with C `iprange` if
  short forms, comments, BOM, hostnames, or malformed lines are not covered.
- Optimizing compare APIs can regress edge cases around empty sets, identical
  sets, adjacent ranges, max IPv4, overlapping ranges, and non-optimized inputs.
- Relaxing file validation would risk accepting corrupt binary sets; validation
  cost must be handled by caching or explicit validated-open design, not by
  removing safety blindly.
- IPv6 parity can expand scope. It should be completed where it is part of the
  exported CLI/library contract, but it must not distract from the IPv4
  production hot paths.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- `pkg/iprange` is slow by design in several areas because framework telemetry,
  generic iterator boundaries, string-heavy parsing, per-pair materialization,
  repeated full-source summaries, and repeated file validation are embedded in
  package-level operations.
- The previous source-level set-algebra work fixed the worst retention/historical
  comparison production path, but several structural patterns remain and can
  recreate the same problem in other paths.

Evidence reviewed:

- Code evidence listed in `Analysis`.
- Benchmark evidence listed in `Analysis`.
- User requirement that tests must exist before code changes and must be
  behavioral with corner cases.
- Project testing and Go testing skills requiring black-box behavior tests,
  `t.TempDir`, `t.Context`, table tests, no mock frameworks, no sleeps, and
  benchmark validation with allocation reporting.

Affected contracts and surfaces:

- `pkg/iprange` exported API and performance contract.
- `pkg/iprange` CLI behavior and compatibility with C `iprange` expectations.
- Engine heavy phases using `RangeSource`, `FileSet`, comparison filters, ASN,
  bogons, critical infrastructure, public IP lookup, and retention.
- OpenTelemetry integration ownership in engine/CLI layers.
- Specs: performance, memory-management, processing-engine, operating-principles,
  and possibly compatibility.
- Runtime project skills: coding/testing guidance for `pkg/iprange` hot paths
  and telemetry ownership.

Existing patterns to reuse:

- Source-level APIs from `SOW-0109`: `UnionSourcesContext`,
  `IntersectSourcesContext`, `ExcludeSourcesContext`, `ExcludeCountContext`,
  and `ExcludeRangesContext`.
- File-backed `FileSet` / mmap / pread model.
- `latestSetCache` and `sharedLatestSetCache` for avoiding repeated opens in
  engine heavy phases and public query paths.
- Existing deterministic `pkg/iprange` equivalence tests and benchmarks.
- Standard-library Go tests with external packages where feasible.

Risk and blast radius:

- Medium/high: `pkg/iprange` is core to feed processing and public serving.
- Compatibility risk: parser and CLI behavior must remain stable.
- Operational risk: removing package-internal OTel must not remove operator
  visibility; metrics move to callers.
- Performance risk: local stats must stay allocation-free and must not use
  interface boxing in hot loops.
- Safety risk: file validation must keep corrupt-set rejection.

Sensitive data handling plan:

- This work uses source code, synthetic benchmark fixtures, and generated
  temporary files only.
- SOWs, specs, docs, skills, instructions, and code comments will not include
  raw secrets, bearer tokens, customer names, personal data, private endpoints,
  or customer-identifying non-private IPs.
- IP examples in tests and docs will use RFC documentation/reserved ranges or
  synthetic private ranges unless a fixture contract requires otherwise.

Implementation plan:

1. Add behavioral tests and benchmark baselines before code changes.
   - Cover parser behavior, `Contains`, stats ownership, compare APIs, source
     materialization, summary/filter behavior, FileSet validation, and IPv6
     parity where relevant.
   - Add allocation-oriented benchmarks with stable fixture generation.
2. Decouple telemetry from `pkg/iprange`.
   - Remove OpenTelemetry imports from `pkg/iprange`.
   - Introduce plain stats/counters returned through explicit result structs,
     callback-free options, or caller-owned collectors after operation completion.
   - Move OTel conversion to engine/CLI or internal observability code.
3. Remove hot-path per-range/per-lookup allocation.
   - Replace `iprangeCount` in `AddRange` and `Contains` with local counters.
   - Ensure no attribute slice/string operation happens per range or lookup.
4. Optimize text parsing.
   - Replace per-line `ReadString` and token splitting with allocation-light
     byte/string scanning while preserving lenient parsing semantics.
   - Keep hostname handling correct and non-fatal.
5. Repair old compare/count APIs.
   - Reimplement `CompareAll`, `CompareNext`, `CompareFirst`, and merged counts
     on top of direct overlap/source scans where possible.
   - Keep output rows and error semantics compatible.
6. Guard generic fallback use.
   - Keep generic `RangeSource` fallback correct.
   - Add tests proving package-owned `IPSet`/`FileSet` paths use indexed logic
     behaviorally through allocation/performance guard benchmarks, not private
     call assertions.
7. Reduce repeated summary/filter allocation.
   - Add reusable/cached summary ownership in engine caches or a `pkg/iprange`
     cacheable summary API without importing engine.
   - Preserve content-hash and disjoint-filter correctness.
8. Keep FileSet validation safe while bounding repeated open cost.
   - Prefer caching validated opens in engine.
   - If adding explicit trust/validation modes, require tests that corrupt files
     are rejected in normal open paths.
9. Add IPv6 parity where exported CLI/library behavior needs it.
   - Add source-level or indexed IPv6 operations if benchmarks show the same
     structural slow path.
10. Update specs, skills, and docs as needed.
    - Record telemetry ownership and hot-path API rules.
    - Keep public/operator docs unchanged unless user-facing CLI behavior or
      operator metrics names change.

Validation plan:

- `go test -count=1 ./pkg/iprange ./pkg/engine ./pkg/asnloc`
- `go test -race -count=1 ./pkg/iprange ./pkg/engine ./pkg/asnloc`
- `make test`
- `make race`
- `make lint`
- `make bench`
- Targeted benchstat runs:
  - parser benchmarks;
  - `SetContains` / `FileSetContains`;
  - compare APIs;
  - source materialization APIs;
  - source summary/hash APIs;
  - IPv6 parity benchmarks when touched.
- Same-failure scans:
  - `rg -n "go.opentelemetry.io|otel\\." pkg/iprange`
  - `rg -n "CollectIterContext\\(|CountIterContext\\(|UnionIter\\(|IntersectIter\\(|ExcludeIter\\(" pkg/engine pkg/asnloc`
  - `rg -n "Combine\\(|CompareAll\\(|CompareFirst\\(|CompareNext\\(" --glob '!**/*_test.go'`
  - `rg -n "time\\.Sleep|testify|gomock|mockery" pkg/iprange pkg/engine pkg/asnloc --glob '*_test.go'`

Artifact impact plan:

- AGENTS.md: likely no update unless project-wide telemetry or hot-path rules
  need stronger language.
- Runtime project skills: update `project-coding` and possibly `project-testing`
  with durable `pkg/iprange` telemetry/performance rules.
- Specs: update performance/memory/processing/operating-principles specs for
  `pkg/iprange` telemetry ownership and bounded hot paths.
- End-user/operator docs: update only if CLI output, metrics names, or operator
  telemetry behavior changes.
- End-user/operator skills: likely unaffected unless docs/spec changes require
  synced operator skill updates.
- SOW lifecycle: keep this SOW separate from `SOW-0106`; move to `current/`
  only when implementation starts; close by setting `Status: completed`, moving
  to `.agents/sow/done/`, and committing implementation plus lifecycle move
  together.

Open-source reference evidence:

- None checked for SOW creation. This work is about local package architecture,
  local benchmark evidence, and compatibility with this project's existing C
  `iprange` expectations. If parser or CLI compatibility behavior is changed,
  implementation must compare against the local FireHOL C `iprange` reference
  and cite exact evidence then.

Open decisions:

- None blocking SOW creation.
- Before implementation starts, confirm this SOW should become the active focused
  work item ahead of broad engine redesign work.

## Implications And Decisions

1. Decision: Scope classification.
   - Selected: long-term-best.
   - Evidence: the user described the package as slow by design and asked to
     fix all identified allocation/inefficiency classes, not only one hotfix.
   - Implication: this SOW must repair package boundaries, tests, callers,
     benchmarks, specs, and skills, not just remove one allocation.
   - Risk: broader blast radius than a surgical patch; mitigated by test-first
     implementation and chunked validation.

2. Decision: Telemetry ownership.
   - Selected: `pkg/iprange` must be telemetry-framework agnostic and return
     plain stats/counters; callers own OTel/log/admin conversion.
   - Evidence: current `pkg/iprange` imports OTel directly and calls metrics in
     hot paths; the user explicitly rejected this model.
   - Implication: `pkg/iprange` public APIs may gain stats-returning variants or
     options, and engine/CLI code must adapt.
   - Risk: operator metrics can regress if caller conversion is incomplete;
     mitigated by tests and metrics/telemetry review in engine callers.

3. Decision: Test-first workflow.
   - Selected: every optimization class must start with behavioral tests and
     benchmark baselines before production code changes.
   - Evidence: user explicitly required tests first and corner-case coverage.
   - Implication: this SOW may look slower at the start, but it avoids optimizing
     untested internals and protects compatibility.
   - Risk: tests can become implementation-coupled if written poorly; mitigated
     by external test packages where feasible and behavior-only assertions.

4. Decision: `pkg/iprange` priority model.
   - Selected: performance and accuracy outrank every other concern inside
     `pkg/iprange`, especially in hot paths.
   - Evidence: the user explicitly stated that `pkg/iprange` must be
     allocation-storm free, optimized exclusively for performance and accuracy,
     and that anything contradicting those goals must be dropped or implemented
     another way.
   - Implication: telemetry, convenience abstractions, generic interfaces, and
     compatibility wrappers are acceptable only if they preserve performance and
     accuracy in hot paths; otherwise they move outside `pkg/iprange` hot paths
     or become cold-path wrappers.
   - Risk: stricter performance constraints can force API changes or separate
     fast/cold paths; mitigated by behavioral compatibility tests and explicit
     benchmark gates.

## Plan

1. Baseline and test inventory.
   - Identify existing behavioral coverage and benchmark gaps.
   - Add missing tests for parser, compare, source APIs, stats, FileSet safety,
     and IPv6 exported behavior before changing production code.
2. Telemetry boundary repair.
   - Remove direct OTel dependency from `pkg/iprange`.
   - Introduce plain stats and caller-owned metric conversion.
3. Hot-path allocation cleanup.
   - Fix `AddRange`, `Contains`, parser, and printer/write hot paths.
   - Verify with allocation benchmarks and behavioral tests.
4. Set algebra and compare API repair.
   - Convert old compare/count APIs to direct scans/source-level primitives.
   - Preserve compatibility and CLI output semantics.
5. Summary/FileSet/engine cache repair.
   - Reduce repeated source-summary allocation and repeated validated opens.
   - Preserve corrupt-file rejection and cache invalidation correctness.
6. IPv6 parity.
   - Add equivalent fast paths/tests for exported IPv6 operations where supported.
7. Documentation and durable rules.
   - Update specs and project skills with the telemetry and hot-path rules.
8. Full validation and review.
   - Run tests, race, lint, benchstat, same-failure scans, and external reviewers
     before commit if the implementation chunk is substantial.

## Execution Log

### 2026-06-20

- Created SOW from local code review and benchmark evidence.
- Recorded user-approved direction: fix all identified slow-by-design patterns,
  use behavioral tests first, and make `pkg/iprange` telemetry-framework
  agnostic.
- Moved SOW to `current/` and marked it `in-progress` after user approval to
  implement.
- Added behavioral tests before implementation for plain operation stats,
  parser compatibility, stats-returning source algebra, file-backed lookup
  stats, binary I/O stats, and latest-set summary reuse.
- Removed direct OpenTelemetry imports and helpers from `pkg/iprange`.
- Added `OperationStats` and stats-returning APIs for parsing, binary I/O,
  source contains, and source algebra while preserving existing API wrappers.
- Removed per-range and per-lookup telemetry callbacks from IPv4 and IPv6 sets
  and file-backed sets.
- Reworked IPv4 text parsing to use byte-slice line scanning and fixed-array
  token parsing for normal decimal paths while preserving lenient base-0 and
  short IPv4 compatibility.
- Replaced old IPv4 and IPv6 compare APIs' pairwise `Combine().Optimize()`
  materialization with exact overlap counting.
- Made range-source summary prefix bitmaps lazy and retained coarse-prefix
  behavior through sparse-prefix fallback.
- Added latest-set summary/filter caching in the engine heavy-phase set cache
  and switched comparison, bogon, ASN, and critical feed filters to reuse it.
- Corrected critical-feed filter error handling so cache-owned latest-set
  sources are not closed outside the cache lifecycle.
- Made latest-set summary building close non-cacheable caller-owned sources
  after summary construction, while leaving cache-owned file sets open until
  cache teardown.
- Updated memory, operating telemetry, coding, and Go hot-path guidance with
  the `pkg/iprange` telemetry and allocation rules.

## Validation

Acceptance criteria evidence:

- `rg -n "go.opentelemetry.io|otel\\.|iprangeObserve|iprangeStart|iprangeCount" pkg/iprange`
  returned no matches after implementation.
- `pkg/iprange/stats.go` defines plain local `OperationStats`; callers own
  telemetry/log/admin export.
- `pkg/iprange/stats_behavior_test.go` covers parse stats, source contains
  stats, source algebra stats, binary I/O stats, and stats accumulation.
- `pkg/iprange/parse_test.go` covers permissive IPv4 base-0/short forms for
  the byte-parser optimization.
- `pkg/engine/heavy_phase_cache_test.go` covers latest-set summary/filter reuse.
- Engine summary/filter callers now use `latestSetCache.Summary` or
  `latestSetCache.OverlapFilter` for repeated latest-set work.

Tests or equivalent validation:

- `go test -count=1 ./pkg/iprange` passed.
- `go test -count=1 ./pkg/engine -run 'TestLatestSetCache'` passed.
- `go test -count=1 ./pkg/iprange ./pkg/engine ./pkg/asnloc` passed.
- `go test -race -count=1 ./pkg/iprange ./pkg/engine ./pkg/asnloc` passed.
- `make build` passed.
- `make lint` passed.
- `go test -count=1 ./tools/archposture` failed only in unrelated UI baseline
  posture: `ui/src/lib/api-types.ts` grew from 1045 to 1099 lines. This SOW did
  not touch that file.
- `make test` failed only in unrelated `tools/archposture`:
  `ui/src/lib/api-types.ts` grew from 1045 to 1099 lines. All normal Go
  packages in the `go test ./...` run passed, including `pkg/iprange`,
  `pkg/engine`, and `pkg/asnloc`.
- Benchmarks:
  - `BenchmarkSetContains/n=100000`: 95.84 ns/op, 0 B/op, 0 allocs/op.
  - `BenchmarkFileSetContains/n=100000`: 179.9 ns/op, 0 B/op, 0 allocs/op.
  - `BenchmarkParseIPs`: 862,411 ns/op, 423,378 B/op, 23 allocs/op for 10k
    input lines, down from the earlier 70,039 allocs/op baseline and 20,023
    allocs/op after telemetry removal alone.
  - `BenchmarkCompare`: 8,229 ns/op, 24,576 B/op, 1 alloc/op for the result
    slice after replacing pairwise union materialization.
  - `BenchmarkBuildRangeSourceSummaryFileSet/n=1000`: 13,408 B/op, down from
    the earlier 144,473 B/op baseline for sparse summaries.

Real-use evidence:

- Focused engine tests exercised latest-set open, summary/filter reuse, metadata
  comparison, and heavy-phase public artifact paths against real temporary
  files.

Reviewer findings:

- External reviewers not run yet for this implementation chunk.

Same-failure scan:

- `rg -n "go.opentelemetry.io|otel\\.|iprangeObserve|iprangeStart|iprangeCount" pkg/iprange`
  found no remaining framework telemetry hooks in `pkg/iprange`.

Sensitive data gate:

- Durable artifact contains no raw secrets, credentials, bearer tokens, SNMP
  communities, community member names, customer names, personal data,
  customer-identifying non-private IPs, private endpoints, or proprietary
  incident details.

Artifact maintenance gate:

- AGENTS.md: no update needed; existing project rules already require SOW,
  standalone `pkg/iprange`, and bounded work.
- Runtime project skills: updated `.agents/skills/project-coding/SKILL.md` and
  `.agents/skills/project-go-best-practices/SKILL.md`.
- Specs: updated `.agents/sow/specs/memory-management.md` and
  `.agents/sow/specs/operating-principles.md`.
- End-user/operator docs: no update needed; CLI/operator surface remains
  compatible and telemetry export ownership is an internal architecture rule.
- End-user/operator skills: no update needed.
- SOW lifecycle: SOW moved to `current/`, completed, and moved to
  `.agents/sow/done/` together with the implementation commit. Remaining valid
  performance follow-up work is tracked in
  `.agents/sow/pending/SOW-0111-20260621-iprange-performance-second-batch.md`.

Specs update:

- `.agents/sow/specs/memory-management.md`: added `pkg/iprange` hot-path
  allocation and plain-stats telemetry boundary rule.
- `.agents/sow/specs/operating-principles.md`: clarified that `pkg/iprange`
  does not import telemetry frameworks and callers export local stats.

Project skills update:

- `.agents/skills/project-coding/SKILL.md`: added `pkg/iprange` telemetry
  agnosticism and allocation-storm-free hot-path rules.
- `.agents/skills/project-go-best-practices/SKILL.md`: added Go hot-path
  guidance for plain stats and no telemetry-framework imports in `pkg/iprange`.

End-user/operator docs update:

- No update needed; no user-facing CLI, admin, public website, or operator-doc
  behavior changed.

End-user/operator skills update:

- No update needed.

Lessons:

- Telemetry callbacks in library hot paths can dominate allocation behavior even
  when the underlying algorithm is correct.
- Parser allocation should be measured separately from mixed benchmark suites;
  full-suite benchmark timing was noisy, but allocation counts were stable.
- Sparse overlap evidence can preserve public filter semantics without always
  allocating the coarse bitmap.

Follow-up mapping:

- Implemented in this SOW: telemetry-framework removal from `pkg/iprange`,
  plain stats APIs, zero-allocation lookup paths, IPv4 parser allocation
  reduction, compare overlap counting, source summary allocation reduction, and
  latest-set summary/filter reuse.
- Tracked for immediate focused follow-up:
  `.agents/sow/pending/SOW-0111-20260621-iprange-performance-second-batch.md`
  covers indexed equality, filter-only summaries, IPv6 parser/source parity,
  and binary I/O allocation cleanup.

## Outcome

Completed. `pkg/iprange` no longer imports OpenTelemetry, hot lookup paths are
zero-allocation, IPv4 parsing allocation is materially reduced, old compare APIs
avoid pairwise set materialization, and engine heavy phases reuse latest-set
summary/filter work.

## Lessons Extracted

- Framework telemetry inside standalone library hot paths is a performance bug.
- Plain local counters keep telemetry useful without binding `pkg/iprange` to a
  telemetry framework.
- Package-owned `RangeSource` implementations need indexed APIs; generic
  iterator fallbacks should remain compatibility paths, not production hot
  paths.
- Summary/filter work should be cached at caller-owned lifecycle boundaries
  when the same set is reused across heavy phases.

## Followup

Tracked in
`.agents/sow/pending/SOW-0111-20260621-iprange-performance-second-batch.md`.

## Regression Log

None yet.

Append regression entries here only after this SOW was completed or closed and
later testing or use found broken behavior. Use a dated `## Regression -
YYYY-MM-DD` heading at the end of the file. Never prepend regression content
above the original SOW narrative.
