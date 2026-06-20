# SOW-0113 - iprange Post-Optimization Performance Investigation

## Status

Status: completed

Sub-state: Closed after implementation, validation, and artifact update.

## Requirements

### Purpose

Make `pkg/iprange` fit for heavy production use by repeating the performance
and delegation investigation on the new baseline after commit `966d06f`.

Performance and accuracy are the top priorities. Recommendations must be based
on measured behavior and code evidence, not guesses.

### User Request

Repeat the new-baseline investigation:

- how can `pkg/iprange` be made faster or lighter;
- what update-ipsets is doing that could be moved or delegated to
  `pkg/iprange` and become faster or lighter.

### Assistant Understanding

Facts:

- The new baseline commit is `966d06f`.
- SOW-0110, SOW-0111, and SOW-0112 already moved and optimized major range
  hot paths in `pkg/iprange`.
- `pkg/iprange` must remain standalone and telemetry-framework agnostic.
- Engine code should orchestrate domain/artifact policy, while reusable
  range-source algorithms belong in `pkg/iprange`.

Inferences:

- Remaining opportunities are likely narrower and must be proven with
  benchmarks, profiles, and production caller evidence.
- Some remaining cost may be acceptable if the production path already uses a
  faster source-level primitive.

Unknowns:

- Exact benchmark/profile shape after commit `966d06f`.
- Whether remaining engine-local range logic is generic enough to belong in
  `pkg/iprange`.

### Acceptance Criteria

- Preserve fresh benchmark/profile evidence for `pkg/iprange` after
  `966d06f`.
- Search update-ipsets for remaining heavy range processing or custom range
  work that could move to `pkg/iprange`.
- Rank opportunities by production value, expected benefit, risk, and fit with
  the standalone `pkg/iprange` contract.
- Record findings, validation commands, and follow-up recommendations in this
  SOW.
- Implement the user-approved hot paths from the investigation:
  header-only file-set metadata, explicit trusted internal file-set open, engine
  retention migration to those APIs, and IPv4 indexed public iterator fast
  paths.
- Add behavioral tests for strict versus trusted file-set opening, metadata
  reads, malformed-file rejection, mixed file-backed iterators, and k-way
  file-backed union.
- Update durable specs when implementation creates or changes a product
  contract.

## Pre-Implementation Gate

Status: completed before implementation.

Problem / root-cause model:

- After the previous optimization batch, remaining bottlenecks must be
  re-measured from the new baseline. Old benchmark priorities may no longer be
  valid.

Evidence to review:

- Fresh `pkg/iprange` benchmark output.
- Focused CPU and allocation profiles for the largest remaining costs.
- Source evidence in `pkg/iprange`, `pkg/engine`, `pkg/asnloc`, and related
  packages.
- Local FireHOL C `iprange` reference only where it clarifies algorithm shape.

Affected contracts and surfaces:

- SOW artifact only during the investigation.
- No code, specs, docs, or skills are changed unless the investigation finds a
  durable artifact gap.

Existing patterns to reuse:

- `pkg/iprange` benchmark suite.
- `pkg/iprange` source-level APIs and file-backed range source helpers.
- Engine migration rules from SOW-0110 through SOW-0112.

Risk and blast radius:

- Low for investigation because no implementation code is changed.
- Recommendations may affect high-blast-radius code later, so each must include
  evidence and risk.

Sensitive data handling plan:

- Record only source paths, line numbers, synthetic benchmark data, and
  sanitized observations. Do not write secrets, private endpoints, customer
  data, or customer-identifying IPs.

Implementation plan:

- Implement the user-approved scope only.
- Keep `OpenFileSet(path)` strict and introduce explicit opt-in trusted opening
  for internal generated artifacts.
- Add a header-only metadata API for count-only callers.
- Use the metadata API for retention cohort index reconstruction.
- Use trusted opening for internal retention cohort scans.
- Add indexed IPv4 public iterator fast paths for file-backed and mixed
  sources.
- Leave IPv6 iterator fast paths, overlap-filter representation changes, and
  error-aware range-source adapters out of this batch.

Validation plan:

- Run focused `pkg/iprange` and `pkg/engine` tests.
- Run focused race tests for touched packages.
- Run targeted benchmarks for changed hot paths.
- Run architecture posture because `pkg/engine` changed; record unrelated
  failures if the gate is already failing outside this change.

Artifact impact plan:

- SOW will be updated with findings.
- Specs/skills/docs will be left unchanged unless the investigation discovers a
  durable contract gap.

Open decisions:

- User approved the recommended implementation path on 2026-06-21.

Approved implementation scope:

- Add `pkg/iprange` header-only file-set metadata APIs.
- Add explicit trusted internal file-set open options that skip O(n) sortedness
  validation while keeping strict `OpenFileSet` unchanged.
- Move engine count-only cohort loading to the metadata API.
- Move internal retention cohort range scans to the trusted-open API where the
  files are application-generated binary `.set` artifacts.
- Add indexed fast paths for IPv4 public set-operation iterators over
  file-backed and mixed sources.
- Leave IPv6 iterator fast paths, overlap-filter representation changes, and
  the error-aware range-source adapter out of this implementation batch unless
  the approved work exposes a direct need.

## Analysis

### New Baseline

The investigation baseline is commit `966d06f`.

Facts confirmed on this baseline:

- Retention cohort comparison now uses `iprange.CompareNextSources`:
  `pkg/engine/retention_update.go:366`.
- `CompareNextSources` accepts file-backed `RangeSource` inputs and calls
  `OverlapCountIterContext` instead of forcing materialization:
  `pkg/iprange/set_ops.go:349`.
- `OverlapCountIterContext` has an indexed/file-backed fast path before the
  public iterator fallback: `pkg/iprange/iter_ops.go:39` and
  `pkg/iprange/overlap_fast.go:7`.
- Source-level materializers (`UnionSourcesContext`,
  `IntersectSourcesContext`, `ExcludeSourcesContext`, `ExcludeCountContext`)
  already use indexed scans before falling back to iterator composition:
  `pkg/iprange/materialize.go:18`, `pkg/iprange/materialize.go:42`,
  `pkg/iprange/materialize.go:66`, `pkg/iprange/materialize.go:89`.

### Benchmark Evidence

Command:

```bash
go test -run '^$' -bench . -benchmem ./pkg/iprange
```

Key results:

- `BenchmarkCompareNextSourcesFileSet/n=100000`: `1912277 ns/op`,
  `224 B/op`, `6 allocs/op`.
- `BenchmarkOverlapCountFileSet/n=100000`: `1879243 ns/op`, `48 B/op`,
  `3 allocs/op`.
- `BenchmarkIntersectIterFileSet/n=100000`: `15413559 ns/op`, `520 B/op`,
  `17 allocs/op`.
- `BenchmarkUnionIterFileSet/n=100000`: `15956829 ns/op`, `568 B/op`,
  `19 allocs/op`.
- `BenchmarkCollectIterContextFileSetUnion/n=100000`: `21460541 ns/op`,
  `8370235 B/op`, `56 allocs/op`.
- `BenchmarkUnionSourcesContextFileSet/n=100000`: `4664714 ns/op`,
  `1606162 B/op`, `9 allocs/op`.
- `BenchmarkIntersectSourcesContextFileSet/n=100000`: `3213409 ns/op`,
  `803314 B/op`, `8 allocs/op`.
- `BenchmarkExcludeSourcesContextFileSet/n=100000`: `3747410 ns/op`,
  `803314 B/op`, `8 allocs/op`.
- `BenchmarkBuildRangeOverlapFilterFileSet/n=100000`: `933506 ns/op`,
  `131457 B/op`, `9 allocs/op`.
- `BenchmarkParseIPs6`: `1207419 ns/op`, `1376493 B/op`, `9 allocs/op`.

Profile evidence:

- `BenchmarkIntersectIterFileSet/n=100000` CPU profile was dominated by
  `iter.Pull` coroutine machinery, not heap allocation. Top flat costs
  included `internal/runtime/atomic.(*Uint32).CompareAndSwap`,
  `iter.Pull[Range].func2`, `(*mmapFileSet).Iter.func1`, and
  `runtime.coroswitch_m`.
- `BenchmarkBuildRangeOverlapFilterFileSet/n=100000` allocation profile was
  dominated by `buildRangeOverlapFilterIndexed`, caused by the coarse prefix
  bitmap allocation.
- `BenchmarkParseIPs6` allocation profile was dominated by
  `(*IPSet6).AddRange6`, i.e. final range-slice materialization/growth.

Scratch measurements for open-time validation:

```text
n=100000  open_validate_avg=497.166us  header_only_avg=6.013us  compare_opened_avg=1.648024ms
n=1000000 open_validate_avg=4.857739ms header_only_avg=5.691us  compare_opened_avg=15.616288ms
```

This measurement used synthetic `.set` files under `/tmp`. It shows that
`OpenFileSet` validation is not the whole cost, but it is material when
repeated across many retention/history files, especially when the caller only
needs header metadata.

Text-input sanity check against local C `../iprange/iprange`:

```text
100k ranges: C compare-next text elapsed=0.02s, maxrss=3720 KB
100k ranges: Go parse+CompareNext elapsed=25.349ms, maxrss=8628 KB
1M ranges:   C compare-next text elapsed=0.22s, maxrss=24776 KB
1M ranges:   Go parse+CompareNext elapsed=250.761ms, maxrss=45864 KB
```

This is not the production retention benchmark because production uses
file-backed `.set` inputs. It does show that Go text parse plus compare is in
the same elapsed-time range as the C tool on synthetic text, while using more
memory.

### Findings

1. Highest-value optimization: split trusted/header-only file-set open from
   full payload validation.

   Evidence:

   - `OpenFileSet` parses the header and verifies optimized mode:
     `pkg/iprange/fileset.go:261`.
   - It checks exact file size before opening the platform backend:
     `pkg/iprange/fileset.go:274`.
   - The mmap backend then validates every range in the payload:
     `pkg/iprange/fileset_mmap.go:84`.
   - The pread backend also validates every range in the payload:
     `pkg/iprange/fileset_pread.go:39`.
   - The validation loop is O(number of ranges):
     `pkg/iprange/fileset_validate.go:5`.
   - `loadRetentionCohorts` opens each cohort only to read `UniqueIPs()` from
     the header: `pkg/engine/runtime_ledger_cache.go:524`.
   - Public/API fallback retention building can call this path:
     `pkg/engine/public_series.go:72` and `pkg/engine/query.go:503`.
   - Retention reconciliation opens every processed cohort before comparing:
     `pkg/engine/retention_update.go:361`.

   Recommendation: long-term-best.

   Add `pkg/iprange` APIs for:

   - header-only metadata, for example `ReadFileSetMetadata(path)`;
   - explicit open options, for example
     `OpenFileSetWithOptions(path, FileSetOpenOptions{ValidateSorted:
     false})`, while keeping `OpenFileSet(path)` as the strict default.

   Then update engine code:

   - use header-only metadata for cohort indexes and other count-only paths;
   - use trusted-open only for internally generated `.set` artifacts where
     exact file size, endianness, optimized marker, and header consistency are
     still checked;
   - keep strict validation for external/public/untrusted file opens.

   Risks:

   - If trusted-open is used on corrupt or unsorted data, binary search and
     range sweeps can produce wrong results.
   - The API must make the trust boundary explicit. The strict default must not
     change.
   - Tests must prove strict open still rejects corrupt sortedness, trusted
     open still rejects malformed headers/size/endianness, and header-only
     metadata does not scan payload bytes.

2. Public file-backed set-operation iterators still use the slow generic
   `iter.Pull` path.

   Evidence:

   - `IntersectIter` only has an in-memory `*IPSet` fast path, then falls back
     to `iter.Pull`: `pkg/iprange/iter_ops.go:59`.
   - `ExcludeIter` does the same: `pkg/iprange/iter_ops.go:117`.
   - `DiffIter` does the same: `pkg/iprange/iter_ops.go:183`.
   - `UnionIter` uses `unionTwo`, whose file-backed path also falls back to
     `iter.Pull`: `pkg/iprange/iter_ops.go:291` and
     `pkg/iprange/iter_ops.go:309`.
   - IPv6 has the same shape in `pkg/iprange/iter6_ops.go:145`,
     `pkg/iprange/iter6_ops.go:237`, `pkg/iprange/iter6_ops.go:325`, and
     `pkg/iprange/iter6_ops.go:423`.
   - Benchmarks show file-backed public iterators around `15-16ms` for 100k
     ranges, while file-backed source-level APIs are around `3-5ms`.

   Recommendation: long-term-best.

   Add indexed fast paths for file-backed and mixed inputs to public iterator
   APIs. This should reuse `indexedRangeSources` and the same direct two-pointer
   scan shape already used by source-level materializers.

   Production impact:

   - Current engine production paths mostly already use source-level APIs, so
     this is not the main production bottleneck.
   - It improves standalone `pkg/iprange` users, tests/stress paths, and any
     future engine path that needs streaming set algebra without materializing.

3. Add non-materializing source-level walkers for set algebra.

   Evidence:

   - `ExcludeRangesContext` already exists for `a\b`:
     `pkg/iprange/materialize.go:114`.
   - There is no equivalent `IntersectRangesContext`,
     `UnionRangesContext`, or `DiffRangesContext`.
   - Callers that need a stream must use public iterators; callers that need
     the fast indexed path generally materialize through `*IPSet`.

   Recommendation: long-term-best, after finding 2.

   This would let callers stream indexed union/intersection/diff output with
   context/error handling and without forcing a heap materialization. It should
   be implemented only where a production caller can use it immediately.

4. Overlap filter broad-source bitmap still allocates about 128 KiB per broad
   filter.

   Evidence:

   - The coarse bitmap uses 20 prefix bits and `1 << 14` uint64 words:
     `pkg/iprange/range_source.go:12`.
   - The sparse prefix builder overflows at 8192 sparse prefixes:
     `pkg/iprange/range_source.go:16`.
   - On overflow, the builder allocates `rangePrefixBitmap`:
     `pkg/iprange/range_source.go:595`.
   - Benchmark/profile evidence shows `BuildRangeOverlapFilterContext` at
     about `131 KiB/op`, dominated by `buildRangeOverlapFilterIndexed`.
   - `latestSetCache` retains summaries/filters for the batch:
     `pkg/engine/latest_set_cache.go:73` and
     `pkg/engine/latest_set_cache.go:113`.

   Recommendation: surgical only if memory pressure is observed; otherwise
   lower priority than findings 1 and 2.

   Possible implementation paths:

   - Add a compact medium-density representation before promoting to the full
     128 KiB bitmap.
   - Keep the existing conservative semantics: false disjoint means unknown,
     never a wrong zero-overlap proof.

   Risks:

   - This filter exists to skip expensive exact comparisons. Reducing memory
     must not reduce pruning accuracy enough to increase total CPU.
   - More representations increase correctness complexity.

5. IPv6 parser allocation can be reduced, but it is not currently the main
   engine bottleneck.

   Evidence:

   - `ParseReader6` preallocates from `EstimateRangeCapacityHint`:
     `pkg/iprange/parse6.go:31`.
   - IPv6 capacity uses `parseIPv6BytesPerRange = 32`:
     `pkg/iprange/parse.go:48`.
   - `BenchmarkParseIPs6` still allocates about `1.37 MiB/op`.
   - The allocation profile is dominated by `(*IPSet6).AddRange6`, which is
     the range slice growing/materializing: `pkg/iprange/set6.go:41`.

   Recommendation: lower priority.

   Improve only if IPv6-heavy production feeds become material. Options include
   a better capacity estimator for compressed IPv6 text, or a builder API where
   callers can pass known line counts.

6. Engine geolocation segment preparation is domain-specific; do not move it
   to `pkg/iprange` yet.

   Evidence:

   - `prepareGeoProvider` expands country-coded sets into segment events:
     `pkg/engine/geo_provider_cache.go:141`.
   - The loop attaches country-code metadata, not generic range algebra:
     `pkg/engine/geo_provider_cache.go:167`.
   - The resulting segment index is consumed through generic
     `WalkRangeOverlapsContext`: `pkg/engine/geo_provider_cache.go:269`,
     `pkg/engine/home_detail_helpers.go:67`, and
     `pkg/engine/home_entity_precompute.go:246`.

   Recommendation: no move now.

   A generic labeled range-partition builder may become useful later, but there
   is only one proven production domain today. Keep it in engine until another
   caller proves the abstraction.

7. `RangeSourceFromIter` has no error channel, which forces an engine
   workaround.

   Evidence:

   - `RangeSourceFromIter` only carries `Iter` and `Len`:
     `pkg/iprange/range_source.go:26`.
   - `countryFilteredRangeSource` wraps `WalkRangeOverlapsContext` and ignores
     its returned error: `pkg/engine/home_detail_helpers.go:44`.

   Recommendation: surgical cleanup, not a performance priority.

   Add an error-aware range-source adapter in `pkg/iprange`, for example a
   `RangeSourceFromIterWithErr` or context-aware walker adapter. Then remove
   the ignored-error workaround from engine.

## Implementation

Implemented from the approved scope:

1. Header-only binary file-set metadata:
   `pkg/iprange.ReadFileSetMetadata(path)`.

   The API validates the binary header, optimized marker, exact file size, and
   endianness marker, then returns header counters without scanning the range
   payload.

2. Explicit trusted internal file-set open:
   `pkg/iprange.OpenFileSetWithOptions(path, FileSetOpenOptions{TrustOptimizedPayload: true})`.

   `OpenFileSet(path)` remains the strict default. Trusted opening skips only
   sorted/non-overlap payload validation and still validates structure, file
   size, optimized marker, and endianness.

3. Engine retention migration:

   - `loadRetentionCohorts` uses `ReadFileSetMetadata` for count-only cohort
     discovery.
   - `openRetentionCohortSet` uses trusted opening for internal generated
     retention `.set` cohorts before falling back to text/binary parsing.

4. IPv4 public iterator fast paths:

   - `IntersectIter`, `ExcludeIter`, `DiffIter`, and `UnionIter` now use
     indexed scans for package-owned file-backed and mixed inputs.
   - Arbitrary `RangeSource` implementations still use the generic `iter.Pull`
     fallback.
   - Indexed helper loops were split into `pkg/iprange/iter_ops_indexed.go` to
     keep the generic iterator file focused.

5. Durable contract update:

   - `.agents/sow/specs/memory-management.md` now records the metadata,
     strict-open, and trusted-open contracts.

## Validation

Commands passed:

```bash
go test ./pkg/iprange
go test ./pkg/engine
go test ./pkg/iprange ./pkg/engine
go test -race ./pkg/iprange ./pkg/engine
```

Targeted benchmark command:

```bash
go test -run '^$' -bench 'Benchmark(ReadFileSetMetadata|OpenFileSetStrict|OpenFileSetTrusted|IntersectIterFileSet|UnionIterFileSet|ExcludeIterFileSet|OverlapCountFileSet|CompareNextSourcesFileSet)/n=100000$' -benchmem ./pkg/iprange
```

Final benchmark results on the local workstation:

```text
BenchmarkCompareNextSourcesFileSet/n=100000-24   1970801 ns/op   224 B/op   6 allocs/op
BenchmarkReadFileSetMetadata/n=100000-24            6702 ns/op  4624 B/op  12 allocs/op
BenchmarkOpenFileSetStrict/n=100000-24            443435 ns/op  4704 B/op  13 allocs/op
BenchmarkOpenFileSetTrusted/n=100000-24            11955 ns/op  4704 B/op  13 allocs/op
BenchmarkOverlapCountFileSet/n=100000-24         2071125 ns/op    48 B/op   3 allocs/op
BenchmarkIntersectIterFileSet/n=100000-24        2816318 ns/op   504 B/op   9 allocs/op
BenchmarkUnionIterFileSet/n=100000-24            3887091 ns/op   552 B/op  11 allocs/op
BenchmarkExcludeIterFileSet/n=100000-24          2985806 ns/op   504 B/op   9 allocs/op
```

Performance delta versus the investigation baseline:

- `IntersectIterFileSet/n=100000`: about `15.4ms` to `2.8ms`.
- `UnionIterFileSet/n=100000`: about `16.0ms` to `3.9ms`.
- New count-only metadata path: about `6.7us` for a 100k-range `.set`.
- New trusted open path: about `12us` for a 100k-range `.set`, versus strict
  open at about `443us`.

Command with unrelated failure:

```bash
go test ./tools/archposture
```

Result:

- Failed because `ui/src/lib/api-types.ts` grew from `1045` to `1099` lines.
- This implementation did not touch `ui/src/lib/api-types.ts`.

## Artifact Impact

- `AGENTS.md`: no update needed; existing standalone `pkg/iprange`, hot-path,
  and SOW rules already covered this work.
- Runtime project skills: no update needed; no new working rule was discovered.
- Specs: updated `.agents/sow/specs/memory-management.md`.
- End-user/operator docs: no update needed; this is internal engine/library
  behavior, not an operator-facing configuration or API change.
- SOW lifecycle: completed and moved to `.agents/sow/done/` with the
  implementation commit.

## Follow-Up Recommendations

Remaining recommendations from the investigation, still not implemented in this
batch:

1. Add file-backed indexed fast paths to public IPv6 set-operation iterators
   only if IPv6 iterator benchmarks or production callers justify the work.
2. Add non-materializing source-level walkers for union/intersection/diff only
   when a production caller is selected.
3. Revisit overlap-filter memory after measuring broad-filter counts and live
   retained memory in a realistic run.
4. Add an error-aware range-source adapter as a small cleanup for the existing
   engine ignored-error workaround.

## Regression Log

None yet.

Append regression entries here only after this SOW was completed or closed and
later testing or use found broken behavior. Use a dated `## Regression -
YYYY-MM-DD` heading at the end of the file. Never prepend regression content
above the original SOW narrative.
