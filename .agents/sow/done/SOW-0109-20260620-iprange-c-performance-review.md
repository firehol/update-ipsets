# SOW-0109 - Go IPrange Compare Hot Path Optimization

## Status

Status: completed

Sub-state: committed-ready; unrelated archposture gate noted

## Requirements

### Purpose

Identify why the Go `pkg/iprange` implementation is slower than the sibling C
`iprange` command on comparable set operations, then add package-owned hot
paths that preserve the Go package's lower-memory and in-process integration
benefits while removing avoidable iterator overhead from production compares.

### User Request

The user first asked to review `pkg/iprange` performance against the C
`../iprange` tool. Prior evidence indicated Go was about 2.2x slower while
using significantly less memory.

After reviewing benchmark and profile evidence, the user approved adding
specialized hot paths in Go to achieve optimal performance while keeping the
package API and memory-oriented `FileSet` model.

After the fast path was implemented and validated, the user asked for a broader
codebase audit to find similar optimization opportunities: heavy processing
work that should move to `pkg/iprange`, engine-local workarounds that duplicate
range/set behavior, and existing `pkg/iprange` operations that still have
avoidable hot-loop overhead.

After reviewing the audit, the user approved the recommended long-term-best
follow-up: implement optimized materialized set algebra in `pkg/iprange` and
switch production engine call sites away from generic iterator materialization.

After implementation validation, the user asked whether the tests were enough
for production-grade confidence. The answer was that existing tests were good
for review/staging, but deterministic randomized equivalence tests against the
generic iterator implementation would be valuable before calling the core range
algebra change fully production-grade. The user approved adding those tests.

### Assistant Understanding

Facts:

- The current application uses Go `pkg/iprange` hot-path APIs after SOW-0108.
- The sibling C project exists at `../iprange` and has a built `iprange`
  executable.
- The previous comparison suggested a speed gap, and this SOW verified the
  cause with fresh evidence.
- The current implementation routes `CompareNextSources()` through
  `OverlapCountIterContext()` -> `IntersectIter()` -> `iter.Pull()`, which
  profiles as the dominant overhead.
- SOW-0109 remains the active SOW, so this follow-up audit is recorded here
  rather than opening a concurrent SOW.

Inferences:

- The implementation should optimize the exact overlap-count operation used by
  compare, not unrelated set operations.
- The lowest-risk design is a type-specialized fast path for package-owned
  source types with the existing generic iterator implementation retained as a
  fallback.

Unknowns:

- Whether a direct mmap-backed byte-slice scan can reach the direct in-memory
  slice scan speed without materializing full sets.
- Whether cancellation checks in the direct loop need tuning to balance
  responsiveness and hot-loop cost.
- Which remaining engine or `pkg/iprange` paths are actually production-heavy
  versus test-only or occasional maintenance paths.

### Acceptance Criteria

- Keep the public `RangeSource`, `CompareNextSources()`, and
  `OverlapCountIterContext()` contracts unchanged.
- Add direct overlap-count fast paths for package-owned sources:
  - `*IPSet` against `*IPSet` through optimized `[]Range`.
  - mmap-backed `FileSet` against mmap-backed `FileSet` through package-private
    mapped range bytes.
  - mixed `*IPSet` / mmap-backed `FileSet` pairs without `iter.Pull()`.
- Preserve the existing iterator-based generic fallback for arbitrary
  `RangeSource` implementations and non-mmap fallback file sets.
- Preserve context cancellation behavior for long scans.
- Add or update tests proving the optimized path produces the same observable
  results as the generic path.
- Add deterministic randomized equivalence tests comparing the new materialized
  set-algebra APIs against the existing generic iterator implementations across
  memory, FileSet, mixed, and generic fallback source combinations.
- Add or update benchmarks so future regressions show the hot-path cost shape.
- Validate with targeted tests, benchmarks, and formatting checks.
- Audit the codebase for additional range/set/IP hot paths and classify each
  candidate as:
  - move-to-`pkg/iprange`;
  - optimize-inside-`pkg/iprange`;
  - leave as-is because it is domain policy, not range algebra;
  - reject because there is no production caller or material cost evidence.
- Add package-level materialized set algebra APIs for union, intersection, and
  exclusion with direct fast paths for package-owned sources and generic
  fallbacks for arbitrary `RangeSource` implementations.
- Add count-only set-difference support where production callers only need the
  total IP count.
- Replace production engine call sites that currently compose
  `CollectIterContext()` or `CountIterContext()` with generic
  `UnionIter()`, `IntersectIter()`, or `ExcludeIter()` when the new package API
  expresses the same contract.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The bottleneck is now identified: the current overlap-count path spends most
  CPU in `iter.Pull()` coroutine machinery while scanning lock-step range
  sources.
- The C command is fast because it uses direct array indexing in a simple
  two-pointer scan after loading binary records into contiguous memory.
- The Go package can retain the lower-memory mmap `FileSet` model, but the hot
  compare path should scan known package-owned data directly instead of
  converting both sides into pulled iterators.

Evidence reviewed:

- Repository worktree status shows unrelated SOW moves and SOW-0106 files are
  present; they are outside this investigation.
- `../iprange/iprange` exists and is executable.
- `pkg/iprange` contains package benchmarks and file-backed range-source
  operations added in SOW-0108.
- Benchmark/profile evidence recorded below shows `runtime.coroswitch_m` and
  `iter.Pull()` dominate the current Go compare path.

Affected contracts and surfaces:

- User approved optimization work in this SOW.
- Code surface: `pkg/iprange`.
- No public UI, API, config, file layout, operator docs, or runtime behavior
  should change.

Existing patterns to reuse:

- Go benchmarks with `testing.B.Loop`.
- `go test -bench ... -benchmem`.
- C command timing via `/usr/bin/time -v` and repeated runs.
- CPU profiles via Go `-cpuprofile` and standard profiling tools when useful.
- Existing package-owned `FileSet` mmap implementation exposes package-private
  `rangesData`, `records`, and `decodeRange()` that can be reused without new
  dependencies or cross-package imports.

Risk and blast radius:

- Behavioral risk is low if the generic fallback remains and fast paths return
  the same `CommonIPs` counts.
- Performance risk is moderate: a specialized path can accidentally optimize
  one source combination while leaving production on the fallback path.
- Memory risk is low if mmap-backed scans do not materialize whole file sets.
- Cancellation risk is low-to-moderate: direct loops must keep bounded context
  checks.

Sensitive data handling plan:

- Use synthetic private/test ranges only.
- Do not write production logs, customer identifiers, customer-identifying
  public IPs, credentials, bearer tokens, SNMP communities, private endpoints,
  or proprietary incident details to the SOW or generated artifacts.

Implementation plan:

1. Keep the existing generic iterator fallback.
2. Add a package-private fast-path dispatcher used by
   `OverlapCountIterContext()`.
3. Add direct scan helpers for optimized `[]Range` and mmap-backed range bytes.
4. Add mixed `*IPSet` / mmap-backed `FileSet` scan coverage.
5. Add correctness tests comparing fast-path-visible behavior through exported
   APIs.
6. Update benchmarks to show the new cost shape for `CompareNextSources()` and
   `OverlapCountIterContext()`.
7. Run targeted tests and benchmarks.

Validation plan:

- `go test -count=1 ./pkg/iprange`
- `go test -run '^$' -bench 'Benchmark(CompareNextSourcesFileSet|OverlapCount(InMemory|FileSet))' -benchmem -count=5 ./pkg/iprange`
- `gofmt`/`git diff --check`
- Larger project gates if targeted changes expose package-level failures.

Artifact impact plan:

- AGENTS.md: no expected update.
- Runtime project skills: no expected update unless a durable benchmark lesson
  is found.
- Specs: no expected update unless the memory/performance contract needs
  refinement.
- End-user/operator docs: no update expected.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW current during analysis; complete or update
  follow-up mapping after reporting.

Open decisions:

- None blocking. The user approved specialized hot paths for package-owned Go
  sources while keeping generic fallback behavior.
- None blocking for the materialized set algebra work. The user approved the
  recommended long-term-best follow-up after reviewing the audit.

## Execution Log

### 2026-06-20

- Created SOW-0109 for the performance review.
- Confirmed unrelated SOW worktree changes exist and are out of scope.
- Inspected C `--compare-next` implementation:
  - `../iprange/CMakeLists.txt:74`-`../iprange/CMakeLists.txt:76` builds with
    `COMPARE_WITH_COMMON=1`.
  - `../iprange/Makefile:260` builds the local binary with `-O3 -flto`.
  - `../iprange/src/iprange.c:1088`-`../iprange/src/iprange.c:1104`
    optimizes both input sides and calls `ipset_common()`.
  - `../iprange/src/ipset_common.c:42`-`../iprange/src/ipset_common.c:83`
    uses a direct two-pointer array scan over `netaddrs`.
  - `../iprange/src/ipset_binary.c:298`-`../iprange/src/ipset_binary.c:300`
    binary-loads all records into one contiguous heap array.
- Inspected Go compare implementation:
  - `pkg/iprange/set_ops.go:375`-`pkg/iprange/set_ops.go:399`
    routes `CompareNextSources()` through `OverlapCountIterContext()`.
  - `pkg/iprange/iter_ops.go:43`-`pkg/iprange/iter_ops.go:60` counts
    overlap by ranging over `IntersectIter()`.
  - `pkg/iprange/iter_ops.go:66`-`pkg/iprange/iter_ops.go:75` adapts both
    push iterators through `iter.Pull()`.
  - `pkg/iprange/fileset_mmap.go:146`-`pkg/iprange/fileset_mmap.go:163`
    iterates mmap-backed binary data without loading the full array into Go
    heap memory.
  - `pkg/iprange/set.go:149`-`pkg/iprange/set.go:157` uses the same push
    iterator style for in-memory `IPSet`.
- Generated synthetic `/tmp` benchmark datasets:
  - 100k optimized ranges per side, two IPs per range, one common IP per pair.
  - 1M optimized ranges per side with the same shape.
  - All data is synthetic and contains no production-sensitive values.

Benchmark evidence:

```text
100k binary ranges per side:

C command, 100 full process runs:
c_binary_100_processes elapsed=0.36 rss_kb=4256
=> about 3.6 ms/run including process startup, binary load, validate, compare, print

Go FileSet, 100 compare iterations after one load:
mode=fileset iters=100 common=100000 load_ms=0.892 run_total_ms=1769.573 run_per_iter_ms=17.696

Go in-memory iterator, 100 compare iterations after binary load:
mode=mem-binary-iter iters=100 common=100000 load_ms=8.847 run_total_ms=1594.431 run_per_iter_ms=15.944

Go direct in-memory slice scan, 10k compare iterations after binary load:
mode=mem-binary-direct iters=10000 common=100000 load_ms=12.058 run_total_ms=1690.464 run_per_iter_ms=0.169
```

```text
1M binary ranges per side:

C command, 10 full process runs:
c_big_binary_10_processes elapsed=0.18 rss_kb=25420
=> about 18 ms/run including process startup, binary load, validate, compare, print

Go FileSet, 10 compare iterations after one load:
mode=fileset iters=10 common=1000000 load_ms=13.057 run_total_ms=1698.683 run_per_iter_ms=169.868

Go direct in-memory slice scan, 1k compare iterations after binary load:
mode=mem-binary-direct iters=1000 common=1000000 load_ms=157.344 run_total_ms=2522.138 run_per_iter_ms=2.522
```

```text
100k text ranges per side:

C command, 10 full process runs:
c_text_10_processes elapsed=0.27 rss_kb=4228
=> about 27 ms/run including text parse, optimize, compare, print

Go text load plus iterator compare, 10 full process runs:
go_text_iter_10_processes elapsed=1.70 rss_kb=18600

Go text load plus direct slice compare, 10 full process runs:
go_text_direct_10_processes elapsed=1.53 rss_kb=19036

Single Go text direct run:
mode=mem-text-direct iters=1 common=100000 load_ms=134.747 run_total_ms=0.387 run_per_iter_ms=0.387
```

Go package benchmark evidence:

```text
go test -run '^$' -bench 'Benchmark(CompareNextSourcesFileSet|OverlapCount(InMemory|FileSet)|IntersectIter(InMemory|FileSet)|FileSetIter|SetIter)$' -benchmem -count=5 ./pkg/iprange

BenchmarkCompareNextSourcesFileSet/n=100000: about 15.4-15.6 ms/op, about 3.5 KiB/op, 53 allocs/op
BenchmarkOverlapCountInMemory/n=100000: about 14.7-15.8 ms/op, about 2.6 KiB/op, 41 allocs/op
BenchmarkOverlapCountFileSet/n=100000: about 15.2-17.3 ms/op, about 2.6 KiB/op, 41 allocs/op
BenchmarkFileSetIter/n=100000: about 0.42-0.50 ms/op, 40 B/op, 3 allocs/op
BenchmarkSetIter/n=100000: about 0.011-0.016 ms/op, 0 B/op, 0 allocs/op
```

Profile evidence:

```text
go test -run '^$' -bench 'BenchmarkOverlapCountInMemory/n=100000$' -benchtime=5s -cpuprofile /tmp/iprange-overlap-inmem.pprof ./pkg/iprange

BenchmarkOverlapCountInMemory/n=100000: 14.836902 ms/op

Top CPU samples:
25.21% internal/runtime/atomic.(*Uint32).CompareAndSwap
23.55% github.com/firehol/update-ipsets/pkg/iprange.OverlapCountIterContext.IntersectIter.func2 cumulative
11.44% iter.Pull[Range].func1.1
11.28% iter.Pull[Range].func2
48.92% runtime.coroswitch_m cumulative
19.07% github.com/firehol/update-ipsets/pkg/iprange.(*IPSet).Iter.func1 cumulative
```

```text
go test -run '^$' -bench 'BenchmarkOverlapCountFileSet/n=100000$' -benchtime=5s -cpuprofile /tmp/iprange-overlap-fileset.pprof ./pkg/iprange

BenchmarkOverlapCountFileSet/n=100000: 15.215439 ms/op

Top CPU samples:
23.24% internal/runtime/atomic.(*Uint32).CompareAndSwap
12.88% iter.Pull[Range].func2 cumulative
21.74% github.com/firehol/update-ipsets/pkg/iprange.OverlapCountIterContext.IntersectIter.func2 cumulative
23.24% github.com/firehol/update-ipsets/pkg/iprange.(*mmapFileSet).Iter.func1 cumulative
46.15% runtime.coroswitch_m cumulative
5.18% github.com/firehol/update-ipsets/pkg/iprange.decodeRange cumulative
```

Findings:

- The dominant compare slowdown is not mmap decoding. File-backed and
  in-memory iterator overlap have similar timings.
- The dominant compare slowdown is the generic push-iterator/`iter.Pull()`
  design used for lock-step two-source comparison.
- The current Go path creates an intersection stream and then counts it. For
  pure overlap counting this adds an avoidable yield layer.
- Direct Go slice scanning is fast. On the same synthetic 100k binary dataset,
  direct Go overlap counting took about 0.169 ms per comparison, while the
  current exported source path took about 16-18 ms.
- Direct Go slice scanning is also fast at 1M ranges: about 2.5 ms per compare.
  This means Go itself is not the reason for the slowdown.
- C remains faster than the current Go `FileSet` source path because C loads
  ranges into a contiguous array and uses a direct while-loop, while Go uses
  coroutine-style iterator pulling for each range.
- Go text parsing is also slower than the C command on the synthetic text
  input. That is a separate bottleneck from the retention binary compare path.

Implementation:

- Added `pkg/iprange/overlap_fast.go` with direct overlap-count loops for
  optimized `[]Range`, mmap binary byte slices, and mixed range/byte sources.
- Added `pkg/iprange/overlap_fast_mmap.go` for Linux/macOS mmap-backed
  `FileSet` fast paths.
- Added `pkg/iprange/overlap_fast_nommap.go` so non-mmap platforms keep the
  generic iterator fallback.
- Updated `pkg/iprange/iter_ops.go` so `OverlapCountIterContext()` tries the
  fast path first and falls back to `IntersectIter()` for arbitrary
  `RangeSource` implementations.
- Updated `pkg/iprange/fileset_mmap.go` with a package-private read lock helper
  used by the fast path to prevent concurrent close/unmap races.
- Added `TestOverlapCountIterSourceCombinations` in
  `pkg/iprange/iter_ops_test.go` covering memory/memory, FileSet/FileSet,
  mixed memory/FileSet, mixed FileSet/memory, and generic/generic sources
  through the exported overlap API.

Post-implementation benchmark evidence:

```text
go test -run '^$' -bench 'Benchmark(CompareNextSourcesFileSet|OverlapCount(InMemory|FileSet))$' -benchmem -count=5 ./pkg/iprange

BenchmarkCompareNextSourcesFileSet/n=100000:
about 1.91-2.26 ms/op, 2360 B/op, 31 allocs/op
previously about 15.4-15.6 ms/op, 3504 B/op, 53 allocs/op

BenchmarkOverlapCountInMemory/n=100000:
about 0.72-0.79 ms/op, 1440 B/op, 18 allocs/op
previously about 14.7-15.8 ms/op, 2608 B/op, 41 allocs/op

BenchmarkOverlapCountFileSet/n=100000:
about 1.98-2.19 ms/op typical, one run at 2.90 ms/op, 1464 B/op, 19 allocs/op
previously about 15.2-17.3 ms/op, 2608 B/op, 41 allocs/op
```

Command-level warm-cache comparison evidence after rebuilding the temporary Go
harness against the modified package:

```text
100k binary ranges per side:

Go FileSet harness, 100 compare iterations after one load:
mode=fileset iters=100 common=100000 load_ms=1.621 run_total_ms=155.260 run_per_iter_ms=1.553

C command, 100 full process runs:
c_binary_100_processes elapsed=0.29 rss_kb=4256
=> about 2.9 ms/run including process startup, binary load, validate, compare, print
```

```text
1M binary ranges per side:

Go FileSet harness, 10 compare iterations after one load:
mode=fileset iters=10 common=1000000 load_ms=15.844 run_total_ms=157.911 run_per_iter_ms=15.791

C command, 10 full process runs:
c_big_binary_10_processes elapsed=0.16 rss_kb=175532
=> about 16 ms/run including process startup, binary load, validate, compare, print
```

Notes:

- The command-level comparison is not a language benchmark. The C measurement
  includes command startup/load/output per run; the Go harness reports compare
  time after one open/load. It is useful only as a production-shape sanity
  check that the Go hot path is no longer orders of magnitude behind.
- The package benchmarks are the authoritative evidence for the changed Go
  function cost.

Broader audit evidence requested after the hot-path implementation:

- `pkg/engine/critical_feed_writer.go:131` still streams
  `iprange.IntersectIter(w.src.RangeSource, provider.Set)` and then calls
  `AddRange()` on aggregate sets for every intersected range.
- `pkg/engine/retention_update.go:168` materializes new retention entries via
  `CollectIterContext(..., ExcludeIter(current, previous))`.
- `pkg/engine/retention_update.go:179` counts removed retention entries via
  `CountIterContext(..., ExcludeIter(previous, current))`.
- `pkg/engine/retention_update.go:417` materializes retained cohort entries
  via `CollectIterContext(..., IntersectIter(oldSource.RangeSource, current))`.
- `pkg/engine/feed_body_stage.go:442` materializes history derivatives via
  `CollectIterContext(..., UnionIter(rangeSources...))`.
- `pkg/engine/feed_body_stage.go:487` materializes merge subtraction via
  `CollectIterContext(..., ExcludeIter(set, excludeSet))`.
- `pkg/engine/public.go:382`-`pkg/engine/public.go:384` materializes dynamic
  composed include sets via `UnionIter()` plus `CollectIterContext()`.
- `pkg/engine/public.go:414` materializes dynamic composed exclude sets via
  `CollectIterContext(..., ExcludeIter(result, exclSrc.RangeSource))`.
- `pkg/engine/bogons.go:129` materializes the bogon provider union via
  `CollectIterContext(..., UnionIter(sources...))`.
- `pkg/engine/output_comparison.go:150` builds comparison summaries through
  `BuildRangeSourceSummaryContext(ctx, src.RangeSource)`.
- `pkg/engine/bootstrap_entries.go:225`,
  `pkg/engine/bootstrap_entries.go:250`, and `pkg/engine/finalize.go:78`
  compute content hashes through `RangeSourceContentHashContext()`.
- `pkg/engine/feed_body_stage.go:308` uses `RangeSourcesEqualContext()` to
  avoid rewriting unchanged history snapshots.
- `pkg/asnloc/asnloc.go:164` counts ASN residuals through
  `iprange.ExcludeIter(src, exclude)`.
- `pkg/engine/geo_provider_cache.go:249`-`pkg/engine/geo_provider_cache.go:280`,
  `pkg/engine/home_detail_helpers.go:38`-`pkg/engine/home_detail_helpers.go:76`,
  and `pkg/engine/home_entity_precompute.go:234`-`pkg/engine/home_entity_precompute.go:265`
  walk range sources while combining them with prepared geo/ASN domain indexes.
- `pkg/iprange/iter_ops.go:69`-`pkg/iprange/iter_ops.go:118`,
  `pkg/iprange/iter_ops.go:124`-`pkg/iprange/iter_ops.go:180`, and
  `pkg/iprange/iter_ops.go:292`-`pkg/iprange/iter_ops.go:380` implement
  intersect, exclude, and union through generic push iterators and
  `iter.Pull()`.
- `pkg/iprange/range_source.go:47`-`pkg/iprange/range_source.go:63`
  materializes generic iterators by calling `AddRange()` for every range and
  then optimizing the set.
- `pkg/iprange/set.go:44`-`pkg/iprange/set.go:53` records per-range
  `iprange.add.ops` telemetry from `AddRange()`.
- `pkg/iprange/otel.go:70`-`pkg/iprange/otel.go:83` constructs metric
  attributes for each telemetry call.
- `pkg/iprange/range_source.go:184`-`pkg/iprange/range_source.go:212`,
  `pkg/iprange/range_source.go:312`-`pkg/iprange/range_source.go:372`, and
  `pkg/iprange/range_source.go:385`-`pkg/iprange/range_source.go:418` scan
  source iterators for hash, summary, and equality.
- `pkg/iprange/binary.go:168`-`pkg/iprange/binary.go:184` reads in-memory
  binary payload ranges through repeated `binary.Read()` calls.

Focused benchmark evidence for remaining candidates:

```text
go test -run '^$' -bench 'Benchmark(IntersectIter(FileSet|InMemory)|UnionIter(FileSet|InMemory)|ExcludeIter(InMemory|FileSet)|CollectIterContextFileSetUnion|RangeSourceContentHashFileSet|BuildRangeSourceSummaryFileSet|RangeSourcesEqualContextFileSet)$' -benchmem -count=3 ./pkg/iprange

BenchmarkIntersectIterInMemory/n=100000:
about 14.5-15.5 ms/op, about 1.1 KiB/op, 23 allocs/op

BenchmarkIntersectIterFileSet/n=100000:
about 16.0-16.5 ms/op, about 1.1 KiB/op, 23 allocs/op

BenchmarkUnionIterInMemory/n=100000:
about 17.4-18.7 ms/op, about 2.0 KiB/op, 37 allocs/op

BenchmarkUnionIterFileSet/n=100000:
about 17.0-17.6 ms/op, about 2.0 KiB/op, 37 allocs/op

BenchmarkExcludeIterInMemory/n=100000:
about 15.0-15.6 ms/op, about 1.1 KiB/op, 23 allocs/op

BenchmarkCollectIterContextFileSetUnion/n=100000:
about 89-106 ms/op, about 93 MiB/op, about 998k allocs/op

BenchmarkRangeSourcesEqualContextFileSet/n=100000:
about 15.0-15.8 ms/op, about 448-460 B/op, 14 allocs/op

BenchmarkBuildRangeSourceSummaryFileSet/n=100000:
about 1.74-1.83 ms/op, about 266 KiB/op, 28 allocs/op

BenchmarkRangeSourceContentHashFileSet/n=100000:
about 1.27-1.42 ms/op, 368 B/op, 8 allocs/op
```

Ranked audit findings:

1. Move materialized set algebra into `pkg/iprange`.
   - Classification: move-to-`pkg/iprange`.
   - Evidence: production callers still compose `IntersectIter()`,
     `ExcludeIter()`, or `UnionIter()` with `CollectIterContext()` in retention,
     critical-infrastructure output, history derivatives, merge composition,
     public dynamic compose, and bogon provider unions.
   - Why this matters: the worst benchmarked remaining path is
     `CollectIterContextFileSetUnion/n=100000` at about 89-106 ms/op and about
     93 MiB/op, because it performs generic iterator pulling, per-range
     `AddRange()` telemetry, and a final optimize pass.
   - Recommended package API direction: add focused materializers such as
     `UnionSourcesContext(ctx, name, sources...)`,
     `IntersectSourcesContext(ctx, name, a, b)`, and
     `ExcludeSourcesContext(ctx, name, a, b)`, with direct source readers for
     `*IPSet` and mmap-backed `FileSet`, and generic fallback for arbitrary
     `RangeSource`.
   - Risk: these APIs produce sets, not just counts. They must preserve sorted,
     non-overlapping output and cancellation behavior, and must not hide
     FileSet I/O errors.

2. Add count-only set-difference and intersection helpers in `pkg/iprange`.
   - Classification: move-to-`pkg/iprange`.
   - Evidence: `pkg/engine/retention_update.go:179` counts removed ranges with
     `CountIterContext(..., ExcludeIter(...))`; `pkg/asnloc/asnloc.go:164`
     counts ASN residuals through `ExcludeIter()`.
   - Why this matters: count-only paths should not materialize or emit ranges
     when the caller needs only IP totals or downstream domain counting.
   - Recommended package API direction: add direct count helpers such as
     `ExcludeCountContext(ctx, a, b)` and, if needed, scanner/callback helpers
     for residual ranges that avoid `iter.Pull()` for package-owned sources.
   - Risk: ASN residual counting is domain-specific; only the range-difference
     scanner belongs in `pkg/iprange`, not ASN attribution.

3. Optimize equality, hash, and summary scans inside `pkg/iprange`.
   - Classification: optimize-inside-`pkg/iprange`.
   - Evidence: comparison metadata, bootstrap, finalize, and history snapshot
     dedupe call `BuildRangeSourceSummaryContext()`,
     `RangeSourceContentHashContext()`, and `RangeSourcesEqualContext()`.
   - Why this matters: hash/summary are not the worst bottleneck, but equality
     is still about 15 ms for 100k FileSet ranges because it uses
     `iter.Pull()`. Summary/hash are already much cheaper at about 1.3-1.8 ms
     for 100k FileSet ranges, but they can share the direct reader machinery.
   - Recommended package API direction: add direct source scan helpers and use
     them from equality/hash/summary. Equality should fast-reject on known
     length and unique IP count, then directly compare known source ranges.
   - Risk: summary currently combines bounds, sparse filters, and content hash
     in one pass. Splitting this incorrectly could regress comparison pruning.

4. Keep geo/ASN/home-domain range walks outside `pkg/iprange`.
   - Classification: leave as-is because they are domain policy, not generic
     range algebra.
   - Evidence: `geo_provider_cache.go`, `home_detail_helpers.go`, and
     `home_entity_precompute.go` combine range sources with prepared
     geo/ASN-specific segment indexes and output domain payloads.
   - Why this matters: moving these wholesale into `pkg/iprange` would make the
     package depend on engine/provider semantics, violating the standalone
     package boundary.
   - Useful improvement: expose a faster package-private or public source
     scanner if these paths profile hot later, but keep the geo/ASN logic in
     its current domain packages.

5. Optimize `ReadBinary()` only after the set-algebra work.
   - Classification: optimize-inside-`pkg/iprange`, lower priority.
   - Evidence: `ReadBinary()` uses repeated `binary.Read()` calls per record,
     while production latest-set reads normally use `OpenFileSet()` first and
     only fall back to text parsing through `loadTextSet()`.
   - Why this matters: previous command evidence showed in-memory binary load
     is slower than `OpenFileSet()`, but the daemon's hot production paths are
     FileSet-backed when binary artifacts exist.
   - Recommended package API direction: replace per-field `binary.Read()` with
     chunked payload reads and direct decoding, with the same validation.
   - Risk: lower operational impact unless profiling shows binary load fallback
     or CLI usage dominates.

6. Do not prioritize old in-memory `CompareAll()` / `CompareNext()` APIs now.
   - Classification: reject for this optimization pass because there is no
     production caller evidence.
   - Evidence: production retention uses `CompareNextSources()`; `CompareAll()`
     and `CompareNext()` are referenced by CLI/tests and package compatibility
     surfaces.
   - Why this matters: optimizing compatibility APIs is useful, but lower value
     than removing iterator/materialization overhead from production engine
     paths.

7. Treat IPv6 iterator operations as a separate, lower-priority mirror.
   - Classification: optimize-inside-`pkg/iprange`, lower priority.
   - Evidence: `pkg/iprange/iter6_ops.go` has the same iterator structure, but
     the audited production engine paths are IPv4-heavy.
   - Why this matters: the same design likely applies, but doing IPv4 first
     keeps the blast radius aligned to the current production bottleneck.

Materialized set-algebra implementation after user approval:

- Added package-level IPv4 APIs in `pkg/iprange`:
  - `UnionSourcesContext(ctx, name, sources...)`
  - `IntersectSourcesContext(ctx, name, a, b)`
  - `ExcludeSourcesContext(ctx, name, a, b)`
  - `ExcludeCountContext(ctx, a, b)`
  - `ExcludeRangesContext(ctx, a, b, yield)`
- Added direct indexed range-source access for:
  - optimized in-memory `*IPSet`;
  - mmap-backed `FileSet` through package-private mapped bytes under a read
    lock;
  - other `FileSet` implementations through indexed `Range(i)` access.
- Added deterministic multi-mmap read-lock ordering so concurrent multi-source
  operations cannot deadlock on opposite source order.
- Kept generic iterator fallbacks for arbitrary third-party `RangeSource`
  implementations.
- Switched production call sites from generic iterator materialization to the
  new package APIs:
  - retention new/removed/still calculations;
  - history derivative union;
  - merge subtraction;
  - public dynamic compose include/exclude;
  - bogon provider union;
  - critical-infrastructure provider overlap aggregation;
  - ASN residual counting through `ExcludeRangesContext()`.
- Updated the retention static guard so it still proves the same cost-shape
  contract: compare first, materialize only after a removal is detected.
- Added deterministic randomized equivalence tests for the materialized
  set-algebra APIs:
  - edge cases for empty sources, full IPv4 ranges, maximum-`uint32` boundaries,
    alternating single-IP ranges, and dense overlaps;
  - 64 fixed-seed pairwise random cases across memory/memory, FileSet/FileSet,
    memory/FileSet, FileSet/memory, generic/generic, generic/FileSet, and
    FileSet/generic source combinations;
  - 32 fixed-seed k-way union cases across all-memory, all-FileSet,
    all-generic, and mixed source combinations;
  - oracle comparisons against the existing generic iterator implementation
    through `CollectIterContext(... UnionIter/IntersectIter/ExcludeIter ...)`
    and `CountIterContext`-equivalent range counting.

Focused benchmark evidence after materialized set-algebra implementation:

```text
go test -run '^$' -bench 'Benchmark(CollectIterContextFileSetUnion|UnionSourcesContextFileSet|IntersectSourcesContextFileSet|ExcludeSourcesContextFileSet|ExcludeCountContextFileSet)$' -benchmem -count=3 ./pkg/iprange

Old generic union materialization:
BenchmarkCollectIterContextFileSetUnion/n=100000:
about 92-100 ms/op, about 93 MiB/op, about 998k allocs/op

New direct union materialization:
BenchmarkUnionSourcesContextFileSet/n=100000:
about 4.67-4.87 ms/op, about 1.6 MiB/op, 24 allocs/op

New direct intersection materialization:
BenchmarkIntersectSourcesContextFileSet/n=100000:
about 3.30-3.32 ms/op, about 785 KiB/op, 14 allocs/op

New direct exclusion materialization:
BenchmarkExcludeSourcesContextFileSet/n=100000:
about 3.66-3.83 ms/op, about 785 KiB/op, 14 allocs/op

New count-only exclusion:
BenchmarkExcludeCountContextFileSet/n=100000:
about 2.99-4.58 ms/op, 944 B/op, 12 allocs/op
```

Implementation notes:

- The materialized APIs intentionally produce optimized `IPSet` values directly
  instead of calling `AddRange()` per yielded range. This removes per-range
  telemetry/attribute allocation from large set-algebra output.
- Critical-infrastructure aggregation now materializes each provider overlap
  once and merges it into aggregate/tier sets, preserving aggregate
  de-duplication through the existing final `Optimize()` calls.
- `pkg/iprange` remains standalone and imports no project packages.
- Updated `.agents/skills/project-coding/SKILL.md` so future engine work uses
  source-level `pkg/iprange` materialized/count APIs instead of reintroducing
  generic iterator materialization in production hot paths.

## Validation

Commands run:

- `go test -run '^$' -bench 'Benchmark(CompareNextSourcesFileSet|OverlapCount(InMemory|FileSet)|IntersectIter(InMemory|FileSet)|FileSetIter|SetIter)$' -benchmem -count=5 ./pkg/iprange`
- `go test -run '^$' -bench 'BenchmarkOverlapCountInMemory/n=100000$' -benchtime=5s -cpuprofile /tmp/iprange-overlap-inmem.pprof ./pkg/iprange`
- `go test -run '^$' -bench 'BenchmarkOverlapCountFileSet/n=100000$' -benchtime=5s -cpuprofile /tmp/iprange-overlap-fileset.pprof ./pkg/iprange`
- Synthetic C and Go command-level timing runs recorded above.
- `go test -count=1 ./pkg/iprange`
- `go test -run '^$' -bench 'Benchmark(CompareNextSourcesFileSet|OverlapCount(InMemory|FileSet))$' -benchmem -count=5 ./pkg/iprange`
- `go test -count=1 ./pkg/iprange ./pkg/engine`
- `go test -race -count=1 ./pkg/iprange`
- `GOOS=freebsd go test -c -o /tmp/iprange-freebsd.test ./pkg/iprange`
- `GOOS=windows go test -c -o /tmp/iprange-windows.test.exe ./pkg/iprange`
- `make build`
- `make lint`
- `git diff --check`
- `go test -run '^$' -bench 'Benchmark(IntersectIter(FileSet|InMemory)|UnionIter(FileSet|InMemory)|ExcludeIter(InMemory|FileSet)|CollectIterContextFileSetUnion|RangeSourceContentHashFileSet|BuildRangeSourceSummaryFileSet|RangeSourcesEqualContextFileSet)$' -benchmem -count=3 ./pkg/iprange`
- `go test -count=1 ./pkg/iprange ./pkg/asnloc ./pkg/engine`
- `go test -race -count=1 ./pkg/iprange ./pkg/asnloc ./pkg/engine`
- `GOOS=freebsd go test -c -o /tmp/iprange-freebsd.test ./pkg/iprange`
- `GOOS=windows go test -c -o /tmp/iprange-windows.test.exe ./pkg/iprange`
- `go test -run '^$' -bench 'Benchmark(CollectIterContextFileSetUnion|UnionSourcesContextFileSet|IntersectSourcesContextFileSet|ExcludeSourcesContextFileSet|ExcludeCountContextFileSet)$' -benchmem -count=3 ./pkg/iprange`
- `make test`
- `go test -count=1 ./pkg/iprange`
- `go test -race -count=1 ./pkg/iprange`

Validation results:

- `go test -count=1 ./pkg/iprange`: passed.
- `go test -count=1 ./pkg/iprange ./pkg/engine`: passed.
- `go test -race -count=1 ./pkg/iprange`: passed.
- FreeBSD and Windows compile-only package checks: passed.
- `make build`: passed.
- `make lint`: passed.
- `git diff --check`: passed.
- `go test -count=1 ./pkg/iprange ./pkg/asnloc ./pkg/engine`: passed.
- `go test -race -count=1 ./pkg/iprange ./pkg/asnloc ./pkg/engine`: passed.
- FreeBSD and Windows compile-only package checks after materialized set
  algebra: passed.
- Materialized set-algebra benchmarks passed and show the cost shape recorded
  above.
- Randomized materialized set-algebra equivalence tests passed in
  `go test -count=1 ./pkg/iprange`.
- `go test -race -count=1 ./pkg/iprange` passed after adding the randomized
  equivalence tests.
- `make test`: all main packages passed, but the command failed in
  `tools/archposture` because `ui/src/lib/api-types.ts` exceeds an existing
  line-count baseline (`1045` -> `1099`). That file and `tools/archposture`
  had no local diff in this SOW, so this is recorded as an unrelated validation
  blocker rather than a failure caused by this implementation.

All compared commands returned the expected common IP count:

- 100k range data: `common=100000`.
- 1M range data: `common=1000000`.

## Outcome

The current evidence identifies the main performance problem:

- The slow path is the Go streaming compare implementation, not the core
  two-pointer overlap algorithm.
- The expensive component is `iter.Pull()`/push-iterator plumbing used by
  `CompareNextSources()` -> `OverlapCountIterContext()` -> `IntersectIter()`.
- The fix should be a specialized overlap-count fast path inside `pkg/iprange`
  for known source types, with the current generic iterator implementation kept
  as the fallback for arbitrary `RangeSource` implementations.

Recommended follow-up design:

1. Add direct overlap-count paths for `*IPSet` sources using `[]Range`.
2. Add direct overlap-count paths for mmap-backed `FileSet` sources using the
   package-private `rangesData` slices.
3. Keep existing iterator APIs for generic sources and for operations that
   need to yield materialized ranges.
4. Consider optimizing in-memory `ReadBinary()` later; it currently loads
   binary data much slower than `OpenFileSet()`, but retention compare should
   use `FileSet`.

Implementation outcome:

- Items 1-3 are implemented and validated in this SOW.
- Item 4 remains non-blocking; this implementation optimizes the production
  `FileSet` compare path without changing binary load semantics.

Broader audit outcome:

- The highest-value remaining work is not another engine workaround. It is to
  add package-level materialized set algebra and count-only difference helpers
  to `pkg/iprange`, then replace the engine's current generic
  `CollectIterContext(..., UnionIter/IntersectIter/ExcludeIter(...))` call
  sites with those APIs.
- The next best package-only optimization is direct-source equality/hash/summary
  scanning.
- Geo/ASN/home detail range walks should stay in their domain packages; only
  reusable fast range-source scanning primitives should come from `pkg/iprange`.

Materialized set-algebra outcome:

- The highest-value broader audit item is implemented and validated.
- Engine/asnloc production callers no longer use the audited generic
  `CollectIterContext(..., UnionIter/IntersectIter/ExcludeIter(...))` or
  `CountIterContext(..., ExcludeIter(...))` patterns.
- Remaining lower-priority opportunities are package-local equality/hash/summary
  direct scans, in-memory `ReadBinary()` chunked decoding, and IPv6 mirror
  optimizations if those paths become production-heavy.

## Lessons Extracted

- Do not use `iter.Pull()` in inner loops for large lock-step range scans when
  both sources are known package-owned types. It is clean and generic, but the
  coroutine switching cost dominates at hundreds of thousands of ranges.
- Keep streaming APIs for flexibility, but add type-specialized hot paths for
  `pkg/iprange` operations that are expected to run against large binary
  artifacts.

## Followup

Potential implementation SOW:

- No separate materialized-set-algebra SOW is needed; the approved work was
  consolidated into this active SOW.
- Future focused SOW candidates, if prioritized later:
  - `SOW-0110-YYYYMMDD-iprange-summary-hash-equality-fast-paths.md`
  - `SOW-0111-YYYYMMDD-iprange-binary-load-fast-path.md`
  - `SOW-0112-YYYYMMDD-iprange-ipv6-set-algebra-fast-paths.md`

## Regression Log

None yet.
