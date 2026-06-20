# SOW-0111 - pkg/iprange Performance Second Batch

## Status

Status: open

Sub-state: Created as the concrete follow-up from SOW-0110; implementation has
not started.

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

## Validation

Acceptance criteria evidence:

- Pending implementation.

Tests or equivalent validation:

- Pending implementation.

Real-use evidence:

- Pending implementation.

Reviewer findings:

- Pending implementation.

Same-failure scan:

- Pending implementation.

Sensitive data gate:

- Pending implementation.

Artifact maintenance gate:

- AGENTS.md: pending.
- Runtime project skills: pending.
- Specs: pending.
- End-user/operator docs: pending.
- End-user/operator skills: pending.
- SOW lifecycle: pending.

Specs update:

- Pending implementation.

Project skills update:

- Pending implementation.

End-user/operator docs update:

- Pending implementation.

End-user/operator skills update:

- Pending implementation.

Lessons:

- Pending implementation.

Follow-up mapping:

- Pending implementation.

## Outcome

Pending.

## Lessons Extracted

Pending.

## Followup

None yet.

## Regression Log

None yet.

Append regression entries here only after this SOW was completed or closed and
later testing or use found broken behavior. Use a dated `## Regression -
YYYY-MM-DD` heading at the end of the file. Never prepend regression content
above the original SOW narrative.
