# SOW-0111 - pkg/iprange Performance Second Batch

## Status

Status: completed

Sub-state: Implementation started after SOW-0110 was committed and pushed as
`f8947f5`. Implementation and validation are complete locally; SOW close,
commit, and push were approved by the user and completed with this SOW close.

## Requirements

### Purpose

Continue making `pkg/iprange` fit for heavy production use by removing the
remaining proven slow paths after the first performance architecture repair.
The purpose is performance and accuracy first: hot paths should be
allocation-storm free, bounded, behaviorally tested, and measurably faster.

### User Request

The user approved committing the current SOW-0110 work and then starting the
second batch of `pkg/iprange` performance work identified during the follow-up
review.

### Assistant Understanding

Facts:

- SOW-0110 removed framework telemetry from `pkg/iprange`, optimized IPv4
  parser/lookup paths, added stats APIs, and cached latest-set summaries in the
  engine.
- A benchmark pass after SOW-0110 still found meaningful gaps in IPv6 parsing,
  IPv6 source algebra, exact source equality, filter-only summaries, and binary
  I/O.
- Production engine paths use `RangeSourcesEqualContext`,
  `RangeSourceContentHashContext`, `BuildRangeSourceSummaryContext`, and
  source-level overlap APIs.

Inferences:

- The next highest-value work is not a broad rewrite. It is a focused
  performance pass over remaining measured hot paths.
- IPv6 parity matters because the CLI/library exposes IPv6 and the benchmarks
  show it still has the old string/parser and generic iterator costs.
- Exact equality and filter-only summaries matter because they are used by
  engine production paths.

Unknowns:

- Exact speedups require before/after benchmark runs and possibly focused CPU
  profiles.
- Whether binary read/write allocations matter in production depends on how
  often engine paths load through heap binary readers instead of mmap/pread
  file sets.

### Acceptance Criteria

- `RangeSourcesEqualContext` uses indexed fast paths for package-owned source
  types such as `IPSet` and `FileSet`, while keeping generic fallback behavior.
  Verification: behavioral equality tests pass and FileSet equality benchmarks
  improve.
- IPv6 parsing gets allocation-light parity with the IPv4 byte parser where
  practical.
  Verification: behavioral IPv6 parser tests cover existing semantics and
  `BenchmarkParseIPs6` allocation count drops materially.
- IPv6 overlap/source algebra avoids generic `iter.Pull` where package-owned
  source types can use indexed scans.
  Verification: IPv6 overlap/source benchmarks improve and IPv6 behavior tests
  pass.
- Filter-only overlap building can avoid content hashing when callers do not
  need content identity.
  Verification: behavior remains identical for disjoint filtering and
  filter-only benchmarks avoid SHA work.
- Binary read/write allocation is reduced where safe.
  Verification: binary round-trip tests pass and `BenchmarkBinaryRoundTrip` /
  `BenchmarkBinary6RoundTrip` allocation counts improve.
- No `pkg/iprange` hot path reintroduces OpenTelemetry, project package imports,
  or per-item allocation storms.
  Verification: same-failure scans and `-benchmem` results.

## Analysis

Sources checked:

- `pkg/iprange/parse6.go`
- `pkg/iprange/iter6_ops.go`
- `pkg/iprange/range_source.go`
- `pkg/iprange/materialize.go`
- `pkg/iprange/binary.go`
- `pkg/iprange/binary6.go`
- `pkg/iprange/bench_test.go`
- `pkg/engine/feed_body_stage.go`
- `pkg/engine/output_comparison.go`
- `pkg/engine/bogons.go`
- `pkg/engine/asn.go`

Current state:

- `ParseReader6` still reads text through string lines and `strings.*` helpers.
- IPv6 overlap and set-algebra iterators still use generic `iter.Pull`.
- `RangeSourcesEqualContext` uses `iter.Pull` even for package-owned indexed
  sources.
- `BuildRangeOverlapFilterContext` calls full summary construction, including
  content hashing, even when callers only need overlap filtering.
- IPv4 binary read uses per-field `binary.Read`; IPv6 binary write materializes
  the whole payload in one byte slice.

Benchmark evidence from 2026-06-21 local run:

- `BenchmarkParseIPs`: about `912 us/op`, `423 KB/op`, `23 allocs/op`.
- `BenchmarkParseIPs6`: about `1.81 ms/op`, `1.54 MB/op`, `10,023 allocs/op`.
- `BenchmarkRangeSourcesEqualContextFileSet/n=100000`: about `15.48 ms/op`,
  `448 B/op`, `14 allocs/op`.
- Generic IPv6 overlap/iterator benchmarks at `n=100000`: about `17-22 ms/op`,
  `14-19 allocs/op`.
- Source-level IPv4 FileSet union/intersection/exclusion paths are much faster
  than generic materialization, so the pattern is proven locally.

Risks:

- Parser changes can regress permissive IPv6, IPv4-mapped IPv6, hostname, BOM,
  comment, CIDR, range, and invalid-line behavior.
- Indexed equality/source algebra can hide file I/O errors if `RangeSourceErr`
  handling is not preserved.
- Filter-only summaries can regress comparison correctness if content-hash and
  overlap-filter call sites are mixed up.
- Binary I/O changes can break compatibility with existing `.set` files if
  header, endianness, payload, or validation semantics drift.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- Remaining slowness is concentrated in paths that still use string-heavy IPv6
  parsing, generic `iter.Pull` boundaries for package-owned sources, full
  summary hashing for filter-only use, and allocation-heavy binary I/O.
- SOW-0110 established the correct architecture: package-owned source types
  should use direct indexed scans, while generic iterator fallbacks remain for
  compatibility.

Evidence reviewed:

- Benchmark evidence listed in `Analysis`.
- Code evidence listed in `Analysis`.
- SOW-0110 implementation and validation results.
- Project rules requiring behavioral tests first, SOW tracking, standalone
  `pkg/iprange`, and allocation-storm-free hot paths.

Affected contracts and surfaces:

- `pkg/iprange` exported parser, source, equality, binary, and IPv6 APIs.
- Engine history snapshot comparison, comparison filtering, ASN/bogon split
  preparation, and critical provider-set hashing/filtering.
- Specs and project skills if new durable hot-path rules emerge.
- Tests and benchmarks under `pkg/iprange` and focused engine callers.

Existing patterns to reuse:

- SOW-0110 indexed source APIs in `pkg/iprange/materialize.go`.
- Existing `indexedRangeSource` and mmap lock helpers.
- Existing parser byte-slice helpers in `pkg/iprange/parse.go`.
- Existing behavioral parser, binary, file-set, compare, and source tests.

Risk and blast radius:

- Medium/high inside `pkg/iprange`; the package is core to feed correctness.
- Medium engine impact through equality/filter/hash call sites.
- Low public/API surface impact if exported behavior remains compatible.

Sensitive data handling plan:

- Use only source code, synthetic fixtures, documentation-range IP examples, and
  benchmark-generated temporary files.
- Do not record raw secrets, bearer tokens, customer names, personal data,
  private endpoints, customer-identifying non-private IPs, or proprietary
  incidents in SOWs, specs, docs, skills, or code comments.

Implementation plan:

1. Add behavioral tests and benchmark baselines for equality fast paths,
   filter-only summaries, IPv6 parser parity, IPv6 source algebra, and binary
   I/O allocation.
2. Implement indexed `RangeSourcesEqualContext` for package-owned source types.
3. Add filter-only overlap summary construction without content hashing and
   update callers that do not need hashes.
4. Port IPv4 byte-parser techniques to IPv6 where safe.
5. Add IPv6 indexed source-algebra/count fast paths for package-owned sources.
6. Reduce binary I/O allocation while preserving file compatibility and
   validation.
7. Update specs/skills/SOW validation if durable rules or contracts change.

Validation plan:

- `go test -count=1 ./pkg/iprange ./pkg/engine ./pkg/asnloc`
- `go test -race -count=1 ./pkg/iprange ./pkg/engine ./pkg/asnloc`
- `make build`
- `make lint`
- `make test` with known unrelated archposture issue recorded if still present.
- Targeted benchmarks:
  - `BenchmarkRangeSourcesEqualContextFileSet`
  - `BenchmarkParseIPs6`
  - IPv6 overlap/source-algebra benchmarks
  - `BenchmarkBinaryRoundTrip`
  - `BenchmarkBinary6RoundTrip`
  - filter-only summary benchmark if added
- Same-failure scans:
  - `rg -n "go.opentelemetry.io|otel\\." pkg/iprange`
  - `rg -n "time\\.Sleep|testify|gomock|mockery" pkg/iprange --glob '*_test.go'`

Artifact impact plan:

- AGENTS.md: likely no update unless a project-wide rule changes.
- Runtime project skills: update only if implementation reveals durable
  hot-path lessons not already captured.
- Specs: update memory/operating principles if API or performance contracts
  change.
- End-user/operator docs: likely no update; behavior should remain compatible.
- End-user/operator skills: likely unaffected.
- SOW lifecycle: move this SOW to `current/` before implementation; close only
  after validation and follow-up mapping are complete.

Open-source reference evidence:

- None checked for SOW creation. This is a local performance follow-up over
  already identified `pkg/iprange` implementation paths. If binary or parser
  compatibility changes are made, compare with the local FireHOL C `iprange`
  reference during implementation.

Open decisions:

- User has approved starting this second batch after SOW-0110 commit and push.

## Implications And Decisions

1. Decision: Scope classification.
   - Selected: long-term-best.
   - Evidence: the user approved the second batch after asking for all remaining
     `pkg/iprange` performance opportunities.
   - Implication: implement measured remaining hot paths rather than a single
     surgical tweak.
   - Risk: broader blast radius; mitigated by tests-before-code and focused
     package boundaries.

2. Decision: Priority order.
   - Selected: indexed equality, filter-only summaries, IPv6 parser/source
     parity, then binary I/O cleanup.
   - Evidence: benchmark and caller evidence in `Analysis`.
   - Implication: production engine paths get priority over purely CLI-only
     cleanup.
   - Risk: binary/printing improvements may remain for later if lower value;
     any deferral must be mapped before SOW completion.

## Plan

1. Baseline tests and benchmarks.
2. Indexed equality fast path.
3. Filter-only overlap summary path.
4. IPv6 parser parity.
5. IPv6 indexed source-algebra/count parity.
6. Binary I/O allocation cleanup.
7. Specs/skills updates and full validation.

## Execution Log

### 2026-06-21

- Created as follow-up from SOW-0110 and user approval to start the second
  performance batch after committing the current work.
- Moved to `current/` and marked `in-progress` after SOW-0110 commit `f8947f5`
  was pushed to `origin/main`.
- Added behavioral tests before implementation for:
  - exact `RangeSourcesEqualContext` behavior across in-memory and file-backed
    sources;
  - filter-only `BuildRangeOverlapFilterContext` behavior;
  - IPv6 parser compatibility around BOM, comments, CRLF, ranges, mapped IPv4,
    hostnames, and CIDR;
  - IPv6 overlap counts across memory and file-backed source combinations;
  - IPv6 iterator outputs for intersection, exclusion, union, and input
    non-mutation.
- Implemented indexed `RangeSourcesEqualContext` for package-owned source
  types while keeping generic fallback behavior.
- Split filter-only overlap construction from full source summary hashing so
  callers that only need conservative overlap bounds do not compute content
  hashes.
- Reworked IPv6 parsing to use byte-line processing and `net/netip` token
  parsing while preserving hostname resolution behavior.
- Added IPv6 indexed source adapters for in-memory, mmap-backed, and pread
  file-backed sources, and used them where the API can report operation errors.
- Added direct in-memory IPv6 range sweeps for overlap count, intersection,
  exclusion, and two-source union to remove `iter.Pull` overhead from those
  hot paths.
- Reworked IPv4 and IPv6 binary read/write loops to avoid full payload
  materialization and buffered-writer heap growth where stack chunks are
  sufficient.
- Preserved binary writer short-write behavior by returning
  `io.ErrShortWrite` when an underlying writer reports a partial write without
  an error.

## Validation

Acceptance criteria evidence:

- Indexed equality:
  - `BenchmarkRangeSourcesEqualContextFileSet/n=100000` improved from about
    `17.18 ms/op`, `448 B/op`, `14 allocs/op` to about `1.23 ms/op`,
    `432 B/op`, `6 allocs/op`.
  - Behavioral equality tests cover equal and different file-backed sources.
- IPv6 parser allocation-light parity:
  - `BenchmarkParseIPs6` improved from about `1.72 ms/op`, `1.54 MB/op`,
    `10,023 allocs/op` to about `1.22 ms/op`, `1.38 MB/op`, `23 allocs/op`.
  - Parser behavior tests cover comments, BOM, CRLF, ranges, CIDR, and mapped
    IPv4.
- IPv6 overlap/source algebra:
  - `BenchmarkOverlapCount6InMemory/n=100000` baseline from pushed commit
    `f8947f5`: `18.32 ms/op`, `492 B/op`, `14 allocs/op`.
  - Current `BenchmarkOverlapCount6InMemory/n=100000`: `1.03 ms/op`,
    `0 B/op`, `0 allocs/op`.
  - `BenchmarkIntersectIter6InMemory/n=100000`: `22.59 ms/op`,
    `495 B/op`, `14 allocs/op` -> `1.20 ms/op`, `88 B/op`, `3 allocs/op`.
  - `BenchmarkUnionIter6InMemory/n=100000`: `23.14 ms/op`, `600 B/op`,
    `19 allocs/op` -> `1.77 ms/op`, `136 B/op`, `5 allocs/op`.
  - `BenchmarkExcludeIter6InMemory/n=100000`: `19.97 ms/op`, `493 B/op`,
    `14 allocs/op` -> `1.32 ms/op`, `88 B/op`, `3 allocs/op`.
- Filter-only overlap construction:
  - `BenchmarkBuildRangeOverlapFilterFileSet/n=100000` improved from about
    `2.06 ms/op`, `272717 B/op`, `29 allocs/op` to about `1.07 ms/op`,
    `272480 B/op`, `26 allocs/op`.
  - Full `BenchmarkBuildRangeSourceSummaryFileSet` still computes content hash
    and remains available for callers that need identity.
- Binary I/O:
  - `BenchmarkBinaryRoundTrip` improved from about `17.09 us/op`,
    `135983 B/op`, `26 allocs/op` to about `4.27 us/op`, `8897 B/op`,
    `24 allocs/op`.
  - `BenchmarkBinary6RoundTrip` improved from about `10.91 us/op`,
    `70559 B/op`, `27 allocs/op` to about `3.62 us/op`, `9250 B/op`,
    `27 allocs/op`.
- `pkg/iprange` remains standalone and telemetry-framework agnostic.

Tests or equivalent validation:

- Passed: `go test -count=1 ./pkg/iprange -run 'TestRangeSourcesEqualContextInMemoryAndFileSet|TestRangeSourceSummaryAndOverlapFilter|TestParseReader6CommentsBOMRangesAndCR'`
- Passed: `go test -count=1 ./pkg/iprange -run 'TestOverlapCountIter6InMemoryAndFileSet'`
- Passed: `go test -count=1 ./pkg/iprange -run 'TestIPv6IteratorsInMemory'`
- Passed: `go test -count=1 ./pkg/iprange -run 'TestIPv6IteratorsInMemory|TestOverlapCountIter6InMemoryAndFileSet|Test.*6|Test.*Binary|TestFileSet|TestParseReaderWrongBinaryFamilyErrors|TestOperationStats'`
- Passed: `go test -count=1 ./pkg/iprange -run 'TestBinary|Test.*Binary|TestFileSet|TestFileSet6|TestParseReaderWrongBinaryFamilyErrors|TestOperationStats'`
- Passed: `go test -count=1 ./pkg/iprange`
- Passed: `go test -count=1 ./pkg/iprange ./pkg/engine ./pkg/asnloc`
- Passed: `go test -race -count=1 ./pkg/iprange ./pkg/engine ./pkg/asnloc`
- Passed: `make build`
- Passed: `make lint`
- Failed, unrelated existing gate: `make test` passed normal Go packages,
  including `pkg/iprange`, `pkg/engine`, and `pkg/asnloc`, then failed
  `tools/archposture` because `ui/src/lib/api-types.ts` is recorded as
  growing from `1045` to `1099` lines.
- Passed: `git diff --check`

Real-use evidence:

- Production-facing engine packages that use the `pkg/iprange` source APIs
  pass normal and race tests.
- Benchmarks use file-backed sets and large synthetic range sets up to
  `100000` ranges to exercise the same source shapes used by engine processing.

Reviewer findings:

- External reviewers were not run for this second batch because the user did
  not request them for this milestone.
- Self-review found and fixed a draft IPv6 exclusion fast-path issue that would
  have trimmed the source slice in place. `TestIPv6IteratorsInMemory` now
  asserts input ranges remain unchanged after iteration.

Same-failure scan:

- Passed: `go list -deps ./pkg/iprange | rg '^github\\.com/firehol/update-ipsets/(internal|pkg/)' | rg -v '^github\\.com/firehol/update-ipsets/pkg/iprange$' || true`
  - No project package imports outside `pkg/iprange`.
- Passed: `rg -n 'go\\.opentelemetry\\.io|otel\\.|iprangeObserve|iprangeStart|iprangeCount|iprangeBackground' pkg/iprange || true`
  - No telemetry-framework references in `pkg/iprange`.
- Passed: `rg -n 'time\\.Sleep|testify|gomock|mockery' pkg/iprange --glob '*_test.go' || true`
  - No sleeps or mock/assertion frameworks in `pkg/iprange` tests.

Sensitive data gate:

- Passed. Only synthetic ranges, benchmark-generated temporary files, source
  paths, and command outcomes were recorded. No raw secrets, customer data,
  private endpoints, proprietary incidents, or personal names were added to
  durable artifacts.

Artifact maintenance gate:

- AGENTS.md: no update needed; SOW-0110 already added the durable
  telemetry/performance rules used here.
- Runtime project skills: no update needed; `project-coding` and
  `project-go-best-practices` already require standalone, telemetry-agnostic,
  allocation-storm-free `pkg/iprange` hot paths.
- Specs: no update needed; exported behavior and product contracts are
  unchanged.
- End-user/operator docs: no update needed; behavior is compatible and this is
  internal performance work.
- End-user/operator skills: no update needed.
- SOW lifecycle: SOW moved to `done/`, status changed to `completed`, and the
  lifecycle close is committed with the implementation.

Specs update:

- No spec update needed. This SOW changes implementation performance and
  allocation behavior without changing public product behavior, configuration,
  pipeline semantics, file layout, or operator-facing contracts.

Project skills update:

- No project skill update needed; the relevant durable rules already exist.

End-user/operator docs update:

- No end-user/operator docs update needed.

End-user/operator skills update:

- No end-user/operator skill update needed.

Lessons:

- Direct in-memory IPv6 sweeps are necessary for parity with IPv4 hot paths.
  `iter.Pull` is useful for generic compatibility, but it is too expensive for
  package-owned in-memory source algebra at large range counts.
- Error-returning APIs can use indexed file-backed scans safely. No-error
  iterator APIs should keep generic file-backed iteration unless the public
  contract is changed to surface read errors.
- Filter-only metadata should not reuse content-hashing builders when callers
  only need overlap bounds.
- Parser allocation storms can hide in string conversion around otherwise
  efficient address parsing; behavior tests must cover compatibility before
  moving parser inner loops to bytes.

Follow-up mapping:

- No valid second-batch item is left as an untracked deferral in this SOW.
- Full `make test` remains blocked by the unrelated `tools/archposture`
  baseline issue for `ui/src/lib/api-types.ts`; this SOW does not modify UI
  generated API types.

## Outcome

Implementation and validation completed, then the user approved SOW close,
commit, and push.

## Lessons Extracted

- Keep package-owned source algebra on direct typed paths when benchmarked hot.
- Keep generic iterators as compatibility fallback, not as the default for
  known in-memory source types.
- Separate source identity work from overlap-filter work so callers pay only
  for the information they need.

## Followup

- None for `pkg/iprange` second-batch scope.
- Unrelated repository hygiene remains: `tools/archposture` reports
  `ui/src/lib/api-types.ts` line-count drift from `1045` to `1099`.

## Regression Log

None yet.

Append regression entries here only after this SOW was completed or closed and
later testing or use found broken behavior. Use a dated `## Regression -
YYYY-MM-DD` heading at the end of the file. Never prepend regression content
above the original SOW narrative.
