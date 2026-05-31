# Out-of-Core Memory Refactor for `update-ipsets`

## TL;DR

- Purpose: keep the daemon operational when individual ipsets or the full catalog exceed available RAM.
- The target behavior is not "consume all RAM and die". The target behavior is "continue working from disk-backed data, with slower performance under pressure".
- The current implementation is heap-centric, so soft or hard memory limits alone cannot deliver this behavior. The data path must be redesigned.

## Analysis

### Current behavior in the codebase

- Startup is lightweight:
  - `pkg/engine/engine.go:110-158` loads config, runtime, and `.cache.json`.
  - The daemon does **not** preload all saved `.ipset` / `.netset` files at startup.
- The update pipeline is heap-heavy:
  - `pkg/downloader/downloader.go:137` buffers the entire HTTP body with `io.ReadAll(resp.Body)`.
  - `pkg/engine/process.go:256` reads the full saved source again with `os.ReadFile(sourcePath)`.
  - `pkg/iprange/parse.go:44` buffers the full input again before parsing.
  - `pkg/engine/finalize.go:251-259` renders a full output buffer and reparses it.
  - `pkg/engine/finalize.go:221-233` loads the existing final set fully to diff against it.
- In-memory representation is large-object friendly, not out-of-core:
  - `pkg/iprange/set.go:8-14` stores the whole set as `[]Range`.
  - `pkg/iprange/binary.go:118-148` still allocates all ranges when reading the binary format.
- High-memory read paths already exist outside updates:
  - `pkg/engine/query.go:18-56` loads whole sets one by one for `search`.
  - `pkg/engine/query.go:98-153` loads the target plus every other set for comparison.
  - `pkg/engine/public.go:61-122` loads all include/exclude sets for compose.
- Batch metadata generation is especially dangerous for large catalogs:
  - `pkg/engine/metadata.go:17-24` always calls comparison generation.
  - `pkg/engine/output.go:150-195` loads all output sets and compares every pair.
  - `pkg/iprange/set_ops.go:268-348` pairwise comparison materializes combined sets repeatedly.
- Geolocation enrichment has the same issue:
  - `pkg/engine/geoloc.go:116-181` loads all output sets into memory before comparing against country datasets.
- Current runtime protections are not memory protections:
  - `pkg/engine/runtime.go:124-165` only provides timeouts and parallelism defaults.
  - `pkg/web/middleware.go:55-68` rate-limits HTTP API calls, not RAM usage.
  - `pkg/web/server.go:422-436` reports Go heap allocation, not a real memory guardrail.

### Operational conclusion

- Today the daemon can persist large sets to disk.
- Today the daemon cannot safely process or compare sets significantly larger than available RAM.
- On real pressure, the likely outcomes are:
  - extreme GC churn and latency
  - cgroup reclaim pressure
  - kernel or cgroup OOM termination

### External research

- Go soft memory limit:
  - `runtime/debug.SetMemoryLimit` and `GOMEMLIMIT` are soft limits. They drive more aggressive GC and memory return, but they do not turn a heap-bound algorithm into an out-of-core algorithm.
  - Official references:
    - https://pkg.go.dev/runtime/debug
    - https://go.dev/doc/go1.19
- Linux memory control:
  - `memory.high` is the throttling / reclaim pressure control.
  - `memory.max` is the hard containment limit and can trigger OOM handling.
  - Official reference:
    - https://docs.kernel.org/admin-guide/cgroup-v2.html
- `mmap`:
  - File-backed mappings let the kernel fault pages in on demand.
  - This can provide "slower but alive" behavior only if the code works directly on mapped read-only files without copying them into heap objects.
  - Official reference:
    - https://www.man7.org/linux/man-pages/man2/mmap.2.html
- Relevant open-source pattern:
  - Prometheus TSDB uses mmapped read-only chunks for older data while keeping hot mutable data on the heap.
  - Example:
    - `/opt/baddisk/monitoring/prometheus/prometheus/tsdb/head.go:1910-1925`

## Decisions

### Made / implied by the request

- Decision 1: The consumer-facing goal is out-of-core behavior.
  - Target effect: large sets should degrade performance before they kill the daemon.
  - This excludes solutions that only add hard kill limits without changing the data path.
- Decision 2: The refactor should preserve the current external behavior where practical.
  - CLI, daemon, API, and generated artifacts should keep the same contract unless a specific incompatibility is chosen explicitly.

### Design decisions made

- Decision 3: The first slice will be the full pipeline, not read-path only.
  - Rationale: the project goal is operational safety for large real feeds. Query-only improvements leave the ingestion path heap-bound and still vulnerable to OOM.
  - Consumer impact: large upstream feeds, not just large queries, will be handled by the new design.
- Decision 4: The internal canonical read path will use the existing binary `.set` representation, centered on `lib/{name}/latest.set`.
  - Evidence:
    - the project already writes `latest.set` for every current set in `pkg/engine/retention.go:23-54`
    - the binary format is fixed-width over ranges in `pkg/iprange/binary.go:31-57` and `pkg/iprange/binary.go:118-148`
  - Rationale: this is the smallest and cleanest path to out-of-core reads.
  - Implication: text `.ipset` / `.netset` files remain compatibility artifacts, but internal heavy reads will stop using them as the primary source.
- Decision 5: The out-of-core reader will be a file-backed abstraction with `mmap` as an optimization, not a hard requirement.
  - Rationale:
    - `mmap` is valuable on supported platforms for read-only large-set access
    - a fallback plain file reader keeps portability, testability, and simpler failure handling
  - Consumer impact: the daemon gets the performance win where available, without coupling correctness to one kernel feature.
- Decision 6: Comparison and geolocation outputs will keep their current exact contract and be reimplemented with bounded-memory iterators plus bounded concurrency.
  - Rationale: changing those artifacts would create an avoidable compatibility regression.
  - Risk accepted: these steps may still be slow on very large catalogs, but they will be memory-bounded instead of heap-explosive.
- Decision 7: Runtime memory controls will be complementary safeguards, not the main solution.
  - Rationale: `memory.high`, `memory.max`, and Go soft memory limits help contain damage and enforce slowdown under pressure, but the daemon must be correct even without relying on kill-oriented controls.
  - Consumer impact: operators get safer deployment defaults, but the refactor itself remains the primary fix.
- Decision 8: Implementation should stay on a high-capability model with review support, not be delegated as primary implementation to an open-source model.
  - Evidence:
    - the refactor cuts across downloader, processor, `pkg/iprange`, engine read paths, metadata generation, geolocation, daemon behavior, operator docs, and memory/concurrency testing.
    - correctness depends on preserving existing external behavior while replacing the internal storage and execution model.
    - the main risk is subtle unwanted side effects, not just writing code that compiles.
  - Rationale: this is an architectural refactor with broad regression surface, not a simple isolated feature.
  - Execution strategy:
    - use a frontier/high-capability model for architecture, invasive code changes, and final integration
    - if token pressure matters, use smaller or open-source models only for narrow read-only reviews or tightly bounded mechanical subtasks
    - every non-trivial patch still requires high-quality review before acceptance

## Plan

1. Introduce an out-of-core read layer in `pkg/iprange`.
   - Add a file-backed reader for the existing binary `.set` format.
   - Read from `lib/{name}/latest.set` for current-set heavy operations.
   - Support sequential iteration and membership checks without loading the entire set into Go heap memory.
   - Use `mmap` where supported and beneficial, with a plain file-reader fallback.
   - Add direct binary-search support over the fixed-width range payload before considering any extra sidecar index.
2. Refactor query/read operations to use the out-of-core layer.
   - `QueryIP`
   - `CompareSet`
   - `Compose`
   - metadata comparison generation
   - geolocation comparison generation
3. Replace pairwise in-memory set combination with iterator-based algorithms.
   - union
   - intersection
   - exclusion
   - diff
   - pairwise overlap counting
4. Refactor ingestion to reduce heap spikes.
   - stream downloads to files
   - avoid `io.ReadAll` where possible
   - stream decompression and normalization where processor semantics allow
   - keep only bounded scratch buffers on the heap
5. Add runtime controls that complement the refactor.
   - lower default parallelism when large inputs are detected
   - expose a size cap for raw feed ingestion
   - document recommended `systemd` / cgroup settings using `memory.high` and `memory.max`
6. Improve observability.
   - expose real process memory signals where available
   - distinguish Go heap from file-backed / RSS pressure in status output
7. Execution strategy for the work itself.
   - keep architecture and integration work on the main high-capability model
   - optionally use cheaper models for read-only review passes and isolated helper subtasks
   - do not offload the core refactor as primary implementation to a weaker model

## Detailed implementation breakdown and size estimate

### Phase 1: File-backed binary reader in `pkg/iprange`

- Goal:
  - read the existing binary `.set` format without materializing the whole set as `[]Range`
  - support sequential iteration and `Contains(ip)` from disk-backed storage
- Why this phase exists:
  - it is the foundation for every bounded-memory read path
  - the current `ReadBinary()` allocates the full range slice in memory
- Current evidence:
  - `pkg/iprange/binary.go:60-148`
  - `pkg/iprange/set.go:8-14`
- Likely implementation:
  - add a reader abstraction over binary `.set` files
  - add a plain file-reader implementation first
  - add `mmap`-backed implementation behind the same interface where supported
  - add binary-search over fixed-width range records for point lookups
- Likely files:
  - `pkg/iprange/binary.go`
  - new `pkg/iprange/*reader*.go`
  - new tests and benchmarks in `pkg/iprange`
- Acceptance criteria:
  - `Contains()` works without full in-memory load
  - iteration over all ranges is correct
  - corrupted files are rejected safely
  - benchmarks show bounded heap use on large synthetic datasets
- Estimated size:
  - medium
  - about 1 to 2 focused days

### Phase 2: Iterator-based set operations

- Goal:
  - replace heap-heavy `Combine/Compare*` style logic with bounded-memory iterator logic
- Why this phase exists:
  - current comparison logic repeatedly materializes merged in-memory sets
- Current evidence:
  - `pkg/iprange/set_ops.go:268-348`
  - `pkg/engine/output.go:150-195`
  - `pkg/engine/geoloc.go:116-181`
- Likely implementation:
  - add iterator-driven overlap counting
  - add iterator-driven union / intersection / exclusion / diff where needed
  - keep existing in-memory operations for small or test-only paths where useful
- Likely files:
  - `pkg/iprange/set_ops.go` or new `pkg/iprange/*iter*.go`
  - `pkg/iprange/bench_test.go`
  - `pkg/iprange/*test.go`
- Acceptance criteria:
  - results match current logic for compare / diff / compose semantics
  - operations no longer need all compared sets resident in heap memory
- Estimated size:
  - medium
  - about 1 to 2 days

### Phase 3: Engine read-path migration

- Goal:
  - move heavy daemon and API read paths from text artifacts to `lib/{name}/latest.set`
- Why this phase exists:
  - most steady-state OOM risk is in the read and artifact-generation paths
- Current evidence:
  - `pkg/engine/query.go:18-56`
  - `pkg/engine/query.go:98-153`
  - `pkg/engine/public.go:61-122`
  - `pkg/engine/output.go:150-195`
  - `pkg/engine/geoloc.go:116-181`
  - `pkg/engine/retention.go:23-54`
- Likely implementation:
  - add helper(s) to resolve the canonical binary path for a set
  - update `QueryIP`, `CompareSet`, and `Compose`
  - update metadata comparison generation
  - update geolocation comparison generation
  - keep a safe fallback to text artifacts for transitional or missing cases
- Likely files:
  - `pkg/engine/query.go`
  - `pkg/engine/public.go`
  - `pkg/engine/output.go`
  - `pkg/engine/geoloc.go`
  - `pkg/engine/helpers.go`
- Acceptance criteria:
  - public API behavior remains unchanged
  - large current-set reads avoid text reparsing and avoid loading full catalogs into heap
- Estimated size:
  - medium
  - about 1 to 2 days

### Phase 4: Observability and runtime safeguards — COMPLETED

- Status: **done**
- What was implemented:
  - `pkg/web/sysinfo.go`: new file with `detailedSystemInfo` struct and `detailedStatus()` function
  - Process memory (RSS, VMS, VmData) read from `/proc/self/status` on Linux
  - Go heap stats: HeapAlloc, HeapSys, HeapInuse, HeapIdle, HeapReleased, HeapObjects
  - GC stats: NumGC, PauseTotalNs, LastGC
  - GOMEMLIMIT reporting (reads current value via `debug.SetMemoryLimit(-1)`)
  - Goroutine count and uptime
  - `/api/v1/status` endpoint updated to return all new fields
  - Admin HTML page updated to show heap and RSS separately
  - `max_download_size` config option added (see Phase 5)
  - `pkg/web/sysinfo_test.go`: tests for RSS on Linux, detailed fields, humanBytes
  - Status endpoint integration test verifying JSON fields

### Phase 5: Downloader spill-to-disk — COMPLETED

- Status: **done**
- What was implemented:
  - `pkg/downloader/downloader.go`:
    - `io.ReadAll(resp.Body)` replaced with `io.Copy` to a temp file via `io.TeeReader` with SHA-256 hasher
    - Same-body detection via file hash comparison (`fileHashEquals`) instead of in-memory `bytes.Equal`
    - `Result.Body []byte` replaced with `Result.BodyPath string` (temp file path) + `BodySize` + `BodyHash`
    - `Result.CleanUp()` method for callers to remove temp files
    - `MaxDownloadSize` enforcement: aborts download if body exceeds limit (default 100 MB)
    - `fetchLocalCopy` also refactored to stream through temp file
  - `pkg/engine/process.go`:
    - `applyFetchOutcome` uses `moveDownloadedBody()` instead of `writeFileAtomic(result.Body)`
    - All status branches call `result.CleanUp()` to prevent temp file leaks
    - Download requests now pass `TmpDir` and `MaxDownloadSize` from runtime config
  - `pkg/engine/geoloc.go`:
    - Same refactor as process.go for geolocation downloads
  - `pkg/engine/helpers.go`:
    - `moveDownloadedBody()`: atomic rename with cross-device fallback
  - `pkg/config/config.go`:
    - `MaxDownloadSize int64` added to `RuntimeConfig`
  - `pkg/engine/runtime.go`:
    - `MaxDownloadSize int64` added to `Runtime` struct and wired in `ResolveRuntime`
  - `pkg/downloader/downloader_test.go`: comprehensive test suite:
    - streaming download creates correct file
    - same-body detection works
    - 304 Not Modified handling (no body file)
    - download size limit enforcement (exceeds → abort)
    - temp file cleanup on error (connection drop mid-body)
    - empty body rejection / AcceptEmpty
    - copyfile downloader streaming + same-body
    - POST with headers

### Phase 6: Processor pipeline redesign — COMPLETED

- Status: **done**
- What was implemented:
  - `pkg/processor/stream.go`: `StreamFunc` type and `streamRegistry` mapping 30+ processor names to streaming implementations
  - `pkg/processor/stream_filters.go`: IP filters, format-specific parsers (snort, pix, dshield, dataplane), and gzip decompression as streaming readers
  - `pkg/processor/run_stream.go`: `RunStream()` and `RunStreamToFile()` — pipeline orchestrator that:
    - Classifies steps into streamable vs non-streamable segments
    - Chains streamable segments as nested `io.Reader` pipelines (zero materialization)
    - Falls back to `[]byte` processing only for non-streamable steps (json_path, xml_tag, unzip)
    - Manages intermediate temp files with automatic cleanup
  - `lineFilterReader`: lazy `io.Reader` that scans/filters lines on demand with bounded 64KB buffer
  - `IsStreamable()` public API to check if a pipeline can be fully streamed
  - `pkg/processor/stream_test.go`: comprehensive test suite:
    - Byte-equivalence tests for every streamable processor
    - Gunzip + line processor streaming
    - Fallback for non-streamable processors
    - Mixed pipeline (non-streamable then streamable)
    - Empty input, no-steps, RunStreamToFile
    - Bounded memory test (5MB input with <2MB heap growth)
    - Pipeline classification tests
  - `pkg/processor/checklist_test.go`: ensures all processor names in the byte registry also have stream equivalents
  - `pkg/engine/process.go`: hot path now calls `RunStream()` instead of `Run()` for all streamable pipelines

### Phase 7: Geolocation ingestion cleanup — COMPLETED

- Status: **done**
- What was implemented:
  - `pkg/geoloc/geoloc.go`:
    - New `ParseFile(providerType, path)` API — opens files from disk without `os.ReadFile`
    - `parseIPDenyFile`: streams through `os.Open` -> `gzip.NewReader` -> `tar.NewReader`, each tar entry parsed via `iprange.ParseReader` directly (no `io.ReadAll`)
    - `parseIP2LocationFile`: uses `zip.OpenReader(path)` (OS file handle, not in-memory), streams CSV rows via `csv.Reader.Read()` (not `ReadAll`)
    - `parseIPIPFile`: `zip.OpenReader(path)`, streams lines via `bufio.Scanner` (not `strings.Split`)
    - `parseDBIPFile`: `os.Open` -> `gzip.NewReader`, streams CSV rows via `csv.Reader.Read()` (not `ReadAll`)
    - `parseGeoLite2File`: `zip.OpenReader(path)`, streams both blocks and locations CSVs record-by-record (not `ReadAll`)
    - `openZipEntry` / `openZipEntrySuffix` return `io.ReadCloser` for streaming (not `[]byte`)
    - Old `Parse(providerType, []byte)` API preserved for backward compatibility and tests
  - `pkg/engine/geoloc.go`:
    - `processGeolocationFeeds` now calls `geoloc.ParseFile()` instead of `os.ReadFile` + `geoloc.Parse()`
  - `pkg/geoloc/geoloc_test.go`:
    - Added `ParseFile` tests for all 5 providers (IPDeny, IP2Location, IPIP, DB-IP, GeoLite2)
    - Archive builders extracted to shared helpers for reuse

### Phase 8: Verification, stress tests, and benchmarks — COMPLETED

- Status: **done**
- What was implemented:
  - `pkg/iprange/stress_test.go`:
    - `TestLargeFileSetBounded`: 1M-range FileSet, Contains + Iter + OverlapCountIter, verifies heap growth <10MB for 8MB on-disk data
    - `TestLargeFileSetIntersectIter`: 100K-range FileSet pair, verifies IntersectIter matches in-memory result
    - `TestLargeFileSetUnionExcludeDiff`: 50K random ranges, verifies Union/Exclude/Diff FileSet results match in-memory
  - `pkg/engine/stress_test.go`:
    - `TestEndToEndMultiFeedBatch`: 5 synthetic feeds with merge, verifies all artifacts (ipset, binary, retention, comparison, index.json, QueryIP)
    - `TestEndToEndHeapBounded`: 4 feeds x 5000 lines with remove_comments processor, measures heap growth through full pipeline
  - `pkg/processor/stress_test.go`:
    - `TestStreamProcessorHeapBounded`: 10MB input through remove_comments + extract_ipv4, verifies heap growth <5MB and byte-equivalence with `Run()`
    - `TestStreamProcessorChainedLargeInput`: 100K lines through 3-step pipeline, verifies stream/bytes equivalence
  - `pkg/iprange/bench_test.go`:
    - Added `b.ReportAllocs()` to ALL benchmarks (FileSet and in-memory)
    - Added `b.ReportAllocs()` to all iterator operation benchmarks
  - Verification results:
    - `go test ./...` — all pass
    - `go test -race ./...` — all pass, no races
    - `go vet ./...` — clean
    - FileSet Contains: 0 allocs at 100K ranges
    - FileSet Iter: 48 bytes / 3 allocs (constant) at 100K ranges
    - OverlapCountIter on FileSet: 448 bytes / 14 allocs at 100K ranges
    - All iterator operations: constant O(1) memory regardless of input size

### Overall size estimate

- Meaningful week-sized milestone:
  - Phases 1 through 4
  - about 4 to 7 days
  - outcome:
    - steady-state daemon reads become out-of-core
    - query / compose / compare / metadata / geolocation comparison stop being the main heap explosion paths
    - ingestion of huge raw upstream feeds is still only partially addressed
- Full end-to-end solution:
  - Phases 1 through 8
  - about 8 to 14 days
  - main uncertainty is Phase 6, the processor pipeline redesign

## Suggested weekly priority checkpoint

- If this week needs a realistic high-value cutoff, the best checkpoint is:
  - implement Phases 1 through 4
  - start Phase 5 only if there is remaining time
- Why this is the best cutoff:
  - it removes the largest current steady-state and artifact-generation memory risks
  - it avoids committing this week to the riskiest and broadest part of the work, which is processor streaming
  - it gives a concrete operational improvement without forcing the full ingestion refactor into a rushed implementation
- Brutal truth:
  - if the goal is "huge upstream raw feeds must also be safe this week", then this is probably too large for a high-confidence one-week delivery with full verification
  - if the goal is "daemon reads and catalog-wide comparisons stop being heap bombs this week", that is realistic

## Implied Decisions

- `lib/{name}/latest.set` becomes the primary operational storage format for large-read paths.
- Text `.ipset` / `.netset` artifacts remain outputs for compatibility, but they should not be the primary structure used for heavy internal reads.
- The daemon should prefer bounded-memory algorithms even when they are slower than current heap-heavy implementations.
- The refactor should avoid large behavior changes in API responses or generated files unless explicitly approved.

## Testing Requirements

- Unit coverage for file-backed set readers:
  - iterate all ranges
  - membership checks
  - binary format corruption handling
  - large sparse and dense datasets
- Unit coverage for iterator-based operations:
  - union
  - intersect
  - exclude
  - diff
  - comparison counts
- Integration coverage for daemon/API paths on large synthetic datasets:
  - `/api/v1/search?ip=`
  - `/api/v1/compose`
  - `/api/v1/ipsets/{name}/comparison`
  - geolocation comparison generation
- Ingestion tests with large fixture files:
  - verify no full-buffer regressions on the hot path
  - verify bounded concurrency behavior
- Memory-focused verification:
  - run under constrained cgroup memory with `memory.high`
  - verify degraded performance rather than immediate process death
  - verify `memory.max` containment works as a last-resort safety net
- Regression verification:
  - `go test ./...`
  - `go test -race ./...`
  - `go vet ./...`
  - `go test -bench=. ./...`
- Add benchmarks for:
  - heap reader vs mmapped/file-backed reader
  - query latency under cache-hot and cache-cold conditions
  - comparison throughput with bounded-memory iterators

## Documentation Updates Required

- Update `README.md`:
  - explain out-of-core behavior
  - explain performance trade-offs
  - explain recommended runtime limits and deployment settings
- Add operator guidance for `systemd`:
  - `MemoryHigh=`
  - `MemoryMax=`
  - expected behavior under pressure
- Document binary format changes if the persisted set format evolves.
- Document any deliberate limitations:
  - worst-case random-access latency
  - slower all-pairs comparison generation on large catalogs
