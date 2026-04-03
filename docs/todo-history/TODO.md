# Rewrite update-ipsets in Go — Autonomous Implementation Task

## Open Items

### 1. Structured journald logging with go-systemd
- Use `github.com/coreos/go-systemd/v22/journal` (Red Hat library)
- Write a `slog.Handler` that sends structured fields to journald
- All custom fields prefixed with `UPIP_` (e.g. `UPIP_SOURCE`, `UPIP_STAGE`)
- Auto-detect systemd via `journal.Enabled()` — fallback to text stderr
- Enables: `journalctl -u update-ipsets UPIP_SOURCE=dshield -p err`

### 2. DNSBL download support
- 9 sorbs sources have no HTTP URL — they use DNS-based blocklist queries
- The bash script downloads them via a `dnsbl` mechanism not yet ported to Go
- Currently skipped with "no download URL and no source file"
- Need: implement a `dnsbl` downloader that queries DNS TXT/A records

### 3. Rewrite UI in Vue 3 + PrimeVue (DECIDED)
- **Decision**: Vue 3 + PrimeVue for both public and admin sites
- **Why**: PrimeVue provides 80+ ready-made components (DataTable with sort/filter/search/CSV/pagination, charts, dialogs, toasts, badges)
- **Build step**: npm build pipeline, output embedded via go:embed
- **Scope**: Rewrite both admin.html and index.html/app.js/app.css
- **Status**: Starting implementation

### 4. Git remote for this repo
- No remote configured — `git push` fails
- Need: Costa to decide which GitHub org/repo to push to

### 5. Public site design review
- The public site renders but needs Costa's visual approval
- Disqus dark theme integration pending — N/A, Disqus removed entirely (2026-04-07)
- Search should answer geoIP questions (not yet implemented)
- Hand-crafted HTML fragments need updating to match new design

### 6. globe.gl deprecation warning
- `THREE.Clock` is deprecated in three.js — globe.gl 2.x still depends on it
- Console emits two warnings on every page load: `THREE.THREE.Clock: This module has been deprecated. Please use THREE.Timer instead.`
- Triggered from `initHeroGlobe`/`buildGlobe` in `pkg/web/static/app.js:1745-1773` via the bundled `globe.gl.min.js`
- Not a functional bug today — globe still renders — but the warning will become a hard error when three.js drops the alias
- Resolution path: upgrade `globe.gl.min.js` to a release that uses `THREE.Timer` (or replace globe.gl with a smaller hand-rolled three.js scene since we only use it for the hero)

---

## Implementation Tracker

### Purpose

Build a production-grade Go replacement for FireHOL `update-ipsets` that preserves the current data and CLI compatibility contract while replacing the bash + C pipeline with a single maintainable binary. The implementation now includes the standalone `iprange` layer, config extraction/loading, downloader, processor registry, runtime engine, history snapshots, query support, enable markers, merge handling, and the built-in daemon/web/API/admin surface.

### Separate upcoming refactor

- A dedicated out-of-core memory refactor plan is tracked in `TODO-out-of-core-memory.md`.
- Current sizing from that plan:
  - week-sized meaningful milestone: about 4 to 7 days for out-of-core read paths, iterator-based comparisons, and operator/runtime visibility
  - full end-to-end solution: about 8 to 14 days including downloader spill-to-disk, processor pipeline redesign, geolocation cleanup, and constrained-memory verification
- Weekly priority recommendation from that plan:
  - if time is limited, stop after the read-path / comparison / metadata / geolocation-comparison refactor
  - the largest uncertainty and schedule risk is the processor pipeline redesign, because `processor.Run()` is currently `[]byte`-based

### Current status analysis

- The workspace now contains a functional Go module with:
  - `pkg/iprange` for the C `iprange` replacement
  - `pkg/config` for YAML loading and legacy bash extraction
  - `pkg/downloader` for conditional HTTP fetches and same-body detection
  - `pkg/processor` for the real processor functions used by the extracted FireHOL sources
  - `pkg/cache` for persisted runtime state
  - `pkg/engine` for one-shot runs, merges, history, metadata, query, and kernel apply orchestration
  - `pkg/web` for scheduler, API, static file serving, and admin UI
- The repository now includes `configs/firehol.yaml`, a generated YAML snapshot of the extracted legacy FireHOL sources and merges.
- The reference bash implementation exists at `/home/costa/src/firehol/firehol/sbin/update-ipsets` and is currently the behavioral source of truth.
- The reference C implementation exists at `/home/costa/src/firehol/iprange/` and exposes the standalone `iprange` behavior and binary format compatibility target.
- The current website compatibility targets and generated artifact examples exist under:
  - `/home/costa/src/firehol/firehol/html/ipsets/`
  - `/home/costa/src/firehol/blocklist-ipsets/`
- The highest-risk dependency is the `iprange` rewrite:
  - It defines the binary history format.
  - It implements the set algebra used by processing, history, retention, comparison, and query APIs.
  - It defines the CLI compatibility surface required for batch migration and verification against the current C tool.
- The directory is not a git repository, so verification must rely on file-by-file inspection plus `go test`, `go test -race`, `go vet`, and `go build`.

### Decisions made

- Decision 1: Start with Phase 1 and the `iprange` CLI surface before the daemon/web stack.
  - Context: every downstream phase depends on correct range parsing, normalization, comparison, printing, and binary serialization.
  - Consumer impact: this gives a trustworthy core that can be used both by the future daemon and as a standalone drop-in tool for migration testing.
- Decision 2: Prefer the Go standard library unless a third-party package is required by the spec.
  - Context: this reduces supply-chain and maintenance risk for a long-lived systems binary.
  - Consumer impact: simpler builds, easier static distribution, fewer dependency break risks.
- Decision 3: Keep the project layout aligned with the spec from the first commit, even if most packages are initially skeletons.
  - Context: the requested architecture is explicit and later refactors would be noisier and riskier.
  - Consumer impact: future implementation can expand without structural churn.
- Decision 4: Continue autonomously without stopping again until the full implementation is complete.
  - Context: user instruction received after the first slice was delivered.
  - Consumer impact: execution will proceed across all remaining phases without interim handoff, unless the full implementation is finished.

### Current implementation plan

1. Keep the C-compatible `iprange` layer verified and stable.
2. Finish the remaining parity gaps in the daemon/web side:
   - geolocation enrichment
   - retention histogram generation
   - broader legacy website artifact parity
3. Keep expanding verification:
   - unit tests
   - engine integration tests
   - race / vet / build / benchmark runs
4. Preserve self-contained runtime behavior using the generated YAML catalog when available.

### Implied decisions

- The first delivery slice will prioritize exactness of core data operations over breadth of daemon features.
- IPv6 support will be represented as a stable future-facing API surface plus explicit stubs, while IPv4 will be fully implemented now.
- CLI compatibility will be added incrementally, but the initial flags implemented must match the current C behavior instead of inventing new semantics.

### Testing requirements

- For the initial slice:
  - Unit tests for all set operations and printers.
  - Binary round-trip tests.
  - Parser coverage for IPs, CIDRs, ranges, comments, blank lines, and malformed inputs.
  - Fuzz targets for text parsing and binary decoding.
  - Benchmarks for parse, optimize, exclude/intersect, compare, and binary I/O.
- Verification commands for this slice:
  - `go test ./...`
  - `go test -race ./...`
  - `go vet ./...`
  - `go test -bench=. ./...`
  - `go build ./cmd/update-ipsets`

### Documentation updates required

- Add a repository `README.md` describing the Go rewrite layout and supported commands as soon as the initial binary is runnable.
- Document any intentionally deferred compatibility gaps so later work does not accidentally treat them as complete.

### Progress update

- Implemented so far:
  - project bootstrap (`go.mod`, `Makefile`, `.gitignore`)
  - `cmd/update-ipsets` entrypoint with `iprange`, `run`, `query`, `enable`, `daemon`, and `version`
  - `pkg/iprange` core IPv4 library
  - parsing, set algebra, compare/count modes, printing, binary v1.0 I/O, hostname resolution, prefix reduction
  - `pkg/config` YAML loader plus legacy extractor from the FireHOL bash script
  - `pkg/downloader`, `pkg/processor`, `pkg/cache`, `pkg/engine`, and `pkg/web`
  - generated YAML source catalog: `configs/firehol.yaml`
  - unit tests, fuzz targets, property checks, benchmarks, and runtime integration tests
  - `README.md` and `BENCHMARKS.md`
- Verification completed successfully:
  - `go test ./...`
  - `go test -race ./...`
  - `go vet ./...`
  - `go build ./cmd/update-ipsets`
  - `go build -o update-ipsets ./cmd/update-ipsets`
  - `go test -run '^$' -bench=. ./pkg/iprange`
  - top-level binary smoke:
    - `./update-ipsets run --config <tmp.yaml> --enable-all --rebuild`
    - `./update-ipsets query --config <tmp.yaml> <ip>`
- Direct CLI smoke checks against the existing C binary matched for:
  - `--compare`
  - `A --diff B`
- Remaining major work:
  - geolocation enrichment
  - broader static website artifact parity
  - direct netlink kernel integration instead of `ipset restore`

### Gap analysis against checklist

- Fully covered or substantially covered already:
  - most of section A
  - most of section B
  - most of section C except `ipsets.d/*.yaml` scanning
  - much of sections D, E, and F
  - basic parts of sections H, I, K, and L
- Partially covered:
  - output compatibility: current JSON/history/retention/comparison formats are close but not yet fully spec-exact
  - CLI compatibility: core subcommands exist, but lock handling, some flags, cache migration, and structured logging are incomplete
  - daemon/admin: scheduler and web/admin exist, but auth, rate limiting, reverse-proxy handling, cache headers, robots, and broader old-URL parity are incomplete
  - kernel apply: current code uses `ipset restore`; direct netlink parity is not complete
- Not covered yet:
  - `pkg/geoloc` provider parsers and country JSON generation
  - old bash `.cache` migration support
  - `ipsets.d/*.yaml` directory scanning
  - git integration (`git add`, `.gitignore`, README generation, `set_file_timestamps.sh`)
  - explicit scheduler package and tests
  - compose/search/detail/data API parity
  - admin auth and feed detail views
  - systemd notify/watchdog integration

### Current execution plan after gap analysis

1. Finish configuration/runtime gaps:
   - `ipsets.d/*.yaml` scanning from all configured directories
   - legacy `.cache` file migration into the JSON cache
   - lock-file handling for batch and daemon execution
2. Finish processor parity gaps:
   - add missing primitive processors (`trim`, `cut_delimiter`, `grep`, `grep_not`, `hostname_resolve`, `json_path`, explicit IPv4 filters)
   - add direct unit coverage for each missing primitive
3. Finish web/API/output compatibility gaps:
   - legacy-compatible `/api/v1/ipsets/*`, `/api/v1/search`, and `/api/v1/compose`
   - legacy file routes such as `/all-ipsets.json`, `/{name}.json`, `/files/*`
   - robots, sitemap, cache headers, ETag/Last-Modified
4. Finish broader runtime compatibility:
   - old artifact generation parity (`all-ipsets.json`, per-set JSON, search/detail helpers)
   - geolocation provider support and country JSON generation
   - systemd notify/watchdog integration

### Additional gaps found during code-vs-TODO reconciliation

- Package layout still diverges from Rule 2:
  - no dedicated `pkg/scheduler`
  - no dedicated `pkg/kernel`
  - no dedicated `pkg/output`
  - `pkg/engine/engine.go` is currently far above the target file size
- CLI parity is still incomplete:
  - no global `--silent` / `--verbose`
  - no `--cleanup` alias matching the TODO wording
  - no `--reprocess` alias
  - no `--push-git`
  - no SIGHUP reload handling in daemon mode
  - logging is plain `log.Logger`, not structured levels
- Web/API parity is still incomplete:
  - no CORS headers
  - no web/API tests
  - no HTTPS listener support
  - admin page lacks feed detail, enable/disable actions, system status, and schedule/queue visibility
- Kernel parity is still incomplete:
  - kernel apply still depends on `ipset restore`
  - no explicit kernel inventory helper for already-loaded sets
- Output/git parity is still incomplete:
  - no git add/commit/push integration
  - no `.gitignore` generation for `dont_redistribute`
  - no generated per-directory `README.md`
  - no `set_file_timestamps.sh`

### Updated finish plan

1. Add missing runtime packages and move critical logic behind them:
   - `pkg/kernel` for native Linux ipset handling
   - `pkg/scheduler` for reusable scheduling/state logic
   - `pkg/output` for git/documentation/timestamp artifact generation
2. Close CLI and daemon parity:
   - add compatibility flags and structured log levels
   - add SIGHUP reload support
   - wire push-to-git behavior through runtime options
3. Close web/admin/API parity:
   - add CORS
   - add feed detail, enable/disable, schedule/queue, and system-status admin views/actions
   - add HTTPS support
   - add endpoint tests
4. Close kernel and output parity:
   - native kernel apply with fallback
   - git/timestamp/README generation
5. Re-run full verification:
   - `go test ./...`
   - `go test -race ./...`
   - `go vet ./...`
   - `go test -bench=. ./...`
   - `go build -o update-ipsets ./cmd/update-ipsets`

### Latest verified remaining gaps

- Processor compatibility:
  - extracted YAML still references `$CAT_CMD`, which is not a valid registered processor token
- Scheduler/runtime:
  - split-output feeds still bypass the parallel prefetch path
  - scheduler queue state is not yet persisted for crash recovery
- Downloader compatibility:
  - curl-like downloader options still need broader support for explicit method and basic-auth forms
- Web daemon compatibility:
  - daemon CLI still lacks `--web-dir` / `--web-files-dir` overrides even though config supports them

### Final verified status after the last gap-closing pass

- The remaining compatibility gaps listed above have been implemented:
  - `$CAT_CMD` processor alias added
  - split-output feeds now participate in the download prefetch path
  - scheduler queue state now persists in `scheduler-state.json`
  - downloader options now cover explicit method and basic-auth forms
  - daemon CLI now exposes `--web-dir` and `--web-files-dir`
- The remaining architecture gap from Rule 1 is now closed:
  - `pkg/engine/engine.go` was split into focused files
  - `pkg/processor/processor.go` was reduced below the size target by moving helpers
  - `pkg/iprange/cli.go` was reduced below the size target by moving input helpers
  - current largest files under `cmd/` and `pkg/` are below 500 lines
- Additional web parity improvement completed:
  - set-specific history/comparison responses now use the same in-memory file cache path as the other generated web artifacts
- Final verification completed successfully after the refactor:
  - `go test ./...`
  - `go test -race ./...`
  - `go vet ./...`
  - `go test -bench=. ./...`
  - `go test -run '^$' -bench=. ./pkg/iprange`
  - `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/update-ipsets-linux-amd64 ./cmd/update-ipsets`
  - `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /tmp/update-ipsets-linux-arm64 ./cmd/update-ipsets`
- No further gaps are currently verified from the code-vs-TODO reconciliation pass.

### 2026-04-01 verification/fix pass

- Scope for this pass:
  - verify checklist items `A1` through `O12` by execution, not code presence
  - start the daemon and hit the documented API and legacy URL surfaces
  - run batch mode with `--enable-all` and verify generated artifacts on disk
  - validate all YAML source definitions through the real config + processor pipeline
  - verify query mode against generated ipsets
  - verify geolocation definitions and parser wiring in batch/daemon paths
  - enforce the sub-500-line file target across `cmd/` and `pkg/`
  - rerun `go test ./...`, `go test -race ./...`, `go vet ./...`, and `go test -bench=. ./...`
- Execution rule for this pass:
  - treat every unchecked checklist row as unverified until it is exercised directly
  - fix any mismatch between TODO claims and actual runtime behavior before considering the pass complete
- Runtime defects found and fixed during this pass:
  - `split` feeds failed on the generated `_net` half with `invalid prefix` when host-only entries were present
  - daemon scheduler hot-looped because `split` parents never inherited child check timestamps
  - daemon scheduler hot-looped on merges because preserved upstream mtimes made old merge sources permanently overdue
  - history snapshots and retention windows used upstream `Last-Modified` timestamps instead of local observation time, making `_1h` history variants empty and history CSVs blank for old feeds
  - `--rebuild` incorrectly stopped on HTTP 304 / same-source responses instead of reprocessing existing source files
  - `/api/v1/ipsets/{name}/comparison` and `/countries/{provider}` did not consistently match their generated static-file equivalents
  - `/admin/` returned 404 even though `/admin` worked
  - `sitemap.xml` generated broken per-set URLs without a separating slash
  - the generic `regex` processor from section `E17` was missing from the runtime registry
  - `unzip_csv` emitted extra blank lines for CRLF archives instead of normalized CSV-split output
  - the dynamic `/api/v1/ipsets/{name}/history` fallback returned `timestamp,entries,unique_ips` instead of the required `DateTime,Entries,UniqueIPs`
  - `_changesets.csv` used `IPsAdded,IPsRemoved` instead of the required `AddedIPs,RemovedIPs`
  - git sync staged `../web/...` files outside the repo root when `web_dir` differed from `base_dir`
- Additional regression coverage added:
  - split-output net rendering with disabled `/32` host prefixes
  - scheduler snapshot handling for split feeds
  - merge scheduling with historical source mtimes
  - history retention based on observation time
  - rebuild behavior after conditional `304 Not Modified`
  - real catalog validation for `configs/firehol.yaml` processor names and geolocation provider types
  - full processor checklist coverage, including empty/malformed/large-input execution
  - web tests for auth, admin actions, gzip, cache headers, reverse-proxy IP handling, rate limiting, and TLS listener startup
  - output metadata markdown-link rendering and millisecond timestamp checks
  - git sync behavior for files outside the repo root
- Verification completed successfully after fixes:
  - fixture batch mode: `./update-ipsets run --config tmp/verify2/config.yaml --enable-all --recheck --cleanup`, `./update-ipsets run --config tmp/verify2/config.yaml --enable-all --reprocess`, and `./update-ipsets run --config tmp/verify2/config.yaml --enable-all --rebuild`
  - fixture query mode: `./update-ipsets query --config tmp/verify2/config.yaml 1.2.3.4`
  - fixture daemon mode: started on `127.0.0.1:18082`, validated `/api/v1/ipsets`, `/api/v1/ipsets/{name}`, `/api/v1/ipsets/{name}/data`, `/api/v1/ipsets/{name}/history`, `/api/v1/ipsets/{name}/retention`, `/api/v1/ipsets/{name}/countries/{provider}`, `/api/v1/ipsets/{name}/comparison`, `/api/v1/search`, `/api/v1/compose`, `/api/v1/status`, `/api/v1/schedule`, `/admin`, `/admin/`, `/{name}.json`, `/files/{name}.ipset`, `/all-ipsets.json`, `/{name}_history.csv`, `/{name}_comparison.json`, `/sitemap.xml`, and `/robots.txt`, then exercised real `SIGHUP` reload and `SIGTERM` shutdown against the daemon PID
  - fresh artifact run: `./update-ipsets run --config tmp/verify4-config.yaml --enable-all --recheck --cleanup`, with on-disk verification of `.ipset`, `.netset`, `.setinfo`, per-set `.json`, `all-ipsets.json`, `_history.csv`, `_comparison.json`, `_*_country.json`, `_changesets.csv`, `sitemap.xml`, and timestamp fields
  - CLI surface: executed `version`, `enable --all`, `iprange --count-unique-all`, `run`, `query`, and `run --push-git` against a local bare remote
  - kernel surface: verified `LoadedSets()` and `ApplyIfLoaded()` inside an isolated `unshare -Urn` namespace, including atomic swap and "only touch already-loaded sets" behavior
  - catalog validation: `configs/firehol.yaml` loads with 167 regular sources, 5 geolocation feeds, 14 merges, and no missing processor registry entries
  - size limit check: all files under `cmd/` and `pkg/` remain below 500 lines
  - toolchain: final post-fix passes of `go test ./...`, `go test -race ./...`, `go vet ./...`, `go test -bench=. ./...`, `make clean build test lint bench`, and `GOBIN=$(pwd)/tmp/gobin make install`
  - binary builds: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/update-ipsets-linux-amd64 ./cmd/update-ipsets` and `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /tmp/update-ipsets-linux-arm64 ./cmd/update-ipsets`

## AUTONOMOUS WORK RULES — READ FIRST

**You are working fully autonomously. You must NOT stop, ask questions, or wait for human input at any point.**

1. **Do not stop for any reason** until the entire implementation is complete, fully tested, properly benchmarked, and production-quality.
2. **Make all design decisions yourself.** If a detail is not specified in this document, choose the best approach based on Go best practices and the reference implementation.
3. **Do not ask for clarification.** Ambiguities should be resolved by reading the original bash script (path below) and matching its behavior.
4. **Test everything.** Every feature must have unit tests, integration tests where applicable, and benchmarks for performance-critical code.
5. **Benchmark everything performance-critical.** IP range operations, parsing, comparison, file generation — all must have Go benchmarks.
6. **Follow the checklist at the bottom of this file.** Every single item must be completed and verified before you consider the work done.
7. **Production quality means:** proper error handling, no panics in library code, graceful degradation, structured logging, clean shutdown, resource cleanup.
8. **Use a Makefile** for build orchestration (build, test, bench, lint, install targets).
9. **Use `go mod`** for dependency management. Minimize external dependencies — prefer stdlib where possible.
10. **All code must pass `go vet`, `staticcheck` (if available), and have no data races** (`go test -race`).

**Your working directory is this directory. All code goes here.**

---

## Reference Implementation

The original bash implementation and supporting C code are available for reference:

- **update-ipsets bash script** (~8300 lines): `/home/costa/src/firehol/firehol/sbin/update-ipsets`
- **iprange C tool** (~3000 lines across 11 .c files): `/home/costa/src/firehol/iprange/`
  - Core headers: `iprange.h`, `ipset.h`
  - Operations: `ipset_optimize.c`, `ipset_merge.c`, `ipset_combine.c`, `ipset_exclude.c`, `ipset_common.c`, `ipset_diff.c`, `ipset_reduce.c`, `ipset_print.c`, `ipset_binary.c`, `ipset_load.c`, `ipset_copy.c`
- **HTML description pages**: `/home/costa/src/firehol/firehol/html/ipsets/`
- **Generated output examples** (blocklist-ipsets repo): `/home/costa/src/firehol/blocklist-ipsets/`

**Read the original code when in doubt about any behavior.** The bash script IS the specification for edge cases.

---

## 1. TL;DR

Rewrite the FireHOL `update-ipsets` system (currently ~8300 lines of bash + the ~3000-line C `iprange` tool) as a **single Go binary** running as an always-on daemon. The rewrite must:

**Core (replaces update-ipsets + iprange):**
- Embed all iprange operations natively (no external binary)
- Move all ~166 ipset source definitions from embedded bash code to a YAML configuration file
- Produce byte-compatible output files (the website reads them)
- Achieve a full update cycle in under 15 minutes (current: 1+ hour)
- Use under 512MB RAM for all ipsets simultaneously

**Daemon mode (new):**
- Internal scheduler — manages update timing per source, no cron needed
- Kernel ipset management — loads/updates ipsets via netlink when running as root
- Built-in web server — serves the iplists.firehol.org website directly (replaces nginx)
- REST API — powers the website's dynamic features including IP lookup ("which lists contain this IP?")
- Admin interface — web UI showing update queue/schedule, feed status, errors, manual triggers

**Single binary, three modes:**
1. `update-ipsets daemon` — the main mode: scheduler + web server + API + admin
2. `update-ipsets run [ipset...]` — one-shot batch mode (backwards compatible, for testing)
3. `update-ipsets query <ip>` — CLI tool to check which lists contain an IP

---

## 2. Current Architecture Analysis

### 2.1 End-to-End Pipeline

The system runs as a single bash script invoked every 4 minutes by cron. It uses an exclusive file lock (`flock` on `/run/update-ipsets.lock`) to ensure single-instance execution. A full run processes ALL enabled ipsets sequentially and can take over 1 hour.

**Pipeline per ipset:**
1. **Check if enabled**: `${BASE_DIR}/${ipset}.source` file must exist
2. **Download**: Conditional HTTP GET via curl (If-Modified-Since, compression, retry logic)
3. **Compare**: Binary diff downloaded file vs previous `.source` file
4. **Process**: Pipe through processor function -> trim -> pre_filter -> filter (iprange) -> post_filter -> post_filter2
5. **History**: If history_mins > 0, save binary snapshot to `${HISTORY_DIR}/${ipset}/`, merge historical snapshots for each time window
6. **Finalize**: Compare processed output with previous `.ipset`/`.netset` via `iprange --diff --quiet`, generate header, save file, apply to kernel ipset (if root), update cache, commit to git
7. **Web generation**: After ALL ipsets processed, generate JSON/CSV/XML files, compare ALL ipsets pairwise, cross-reference with GeoIP databases, compute retention histograms

### 2.2 iprange Tool — Architecture (~3000 lines of C across 11 `.c` files)

**Core data structure** (`iprange.h`):
```c
typedef struct network_addr {
    in_addr_t addr;       // 32-bit start address (host byte order)
    in_addr_t broadcast;  // 32-bit end address (host byte order)
} network_addr_t;         // 8 bytes per range entry
```

**ipset container** (`ipset.h`):
```c
typedef struct ipset {
    char filename[FILENAME_MAX+1];
    size_t lines, entries, entries_max, unique_ips;
    uint32_t flags;              // IPSET_FLAG_OPTIMIZED = 0x1
    struct ipset *next, *prev;   // linked list
    network_addr_t *netaddrs;    // dynamic array of ranges
} ipset;
```

**Operations** (each in a separate `.c` file):
- **optimize** (`ipset_optimize.c`): qsort by addr then merge overlapping/adjacent ranges. O(n log n).
- **merge** (`ipset_merge.c`): memcpy one ipset's ranges to the end of another. O(n). Marks as non-optimized.
- **combine** (`ipset_combine.c`): Create new ipset = union (unoptimized). O(n+m).
- **exclude** (`ipset_exclude.c`): Two-pointer sweep on sorted ranges. A - B. O(n+m). Returns optimized.
- **common** (`ipset_common.c`): Two-pointer sweep, intersection. A ∩ B. O(n+m). Returns optimized.
- **diff** (`ipset_diff.c`): Symmetric difference (XOR). A △ B. O(n+m). Returns optimized.
- **reduce** (`ipset_reduce.c`): Reduce CIDR prefix count by merging smaller prefixes into larger ones, allowing controlled increase in covered IPs. Used for kernel ipset optimization.
- **print** (`ipset_print.c`): Output as CIDR, ranges, single IPs, or binary. CIDR printing uses recursive `split_range()` to decompose ranges into optimal CIDRs.
- **binary** (`ipset_binary.c`): Fast serialization — text header + endianness marker + raw `network_addr_t` array.
- **load** (`ipset_load.c`): Parse text (IPs, CIDRs, ranges, hostnames), binary, or mixed input. DNS resolution via pthreads.
- **copy** (`ipset_copy.c`): Deep copy of an ipset.

**Compare modes**:
- `--compare`: All-vs-all pairwise comparison. CSV output: `name1,name2,entries1,entries2,ips1,ips2,combined_ips,common_ips`
- `--compare-first`: Compare first file against all others.
- `--compare-next`: Compare files-before vs files-after the flag.
- `--count-unique` / `--count-unique-all`: Print entry/IP counts per ipset.

### 2.3 Download Manager

- Uses curl with: `--connect-timeout`, `--max-time`, `--retry 0`, `--fail`, `--compressed`, `--user-agent`, `--time-cond` (If-Modified-Since), `--output`, `--remote-time`, `--location`, `--referer`, `--write-out '%{http_code}'`
- **Frequency control**: Each ipset has a `mins` parameter. Adds a 1% margin (capped at 10min) to avoid re-downloading too soon.
- **Failure backoff**: After 10 consecutive failures, the check interval is multiplied by `(failures - 10)`. For fewer failures, the interval is halved (faster retries).
- **Dedup**: After successful download, `diff -q` compares new file with existing `.source`. If identical, the download is discarded.
- **Not-Modified**: HTTP 304 responses handled — cache timestamp updated, no re-processing.
- **Custom downloaders**: Ipsets can specify `downloader` and `downloader_options` attributes.

### 2.4 Cache System

**File**: `${BASE_DIR}/.cache` — a bash script generated by `declare -p` that serializes ~43 associative arrays. Loaded at startup via `source`.

**Cached per ipset** (~36 tracking arrays):
- `IPSET_INFO`, `IPSET_SOURCE`, `IPSET_URL`, `IPSET_FILE`, `IPSET_IPV`, `IPSET_HASH`
- `IPSET_MINS`, `IPSET_HISTORY_MINS`, `IPSET_ENTRIES`, `IPSET_IPS`
- `IPSET_SOURCE_DATE`, `IPSET_PROCESSED_DATE`, `IPSET_CHECKED_DATE`
- `IPSET_CATEGORY`, `IPSET_MAINTAINER`, `IPSET_MAINTAINER_URL`
- `IPSET_LICENSE`, `IPSET_GRADE`, `IPSET_PROTECTION`, `IPSET_INTENDED_USE`
- `IPSET_FALSE_POSITIVES`, `IPSET_POISONING`, `IPSET_SERVICES`
- `IPSET_ENTRIES_MIN/MAX`, `IPSET_IPS_MIN/MAX`, `IPSET_STARTED_DATE`
- `IPSET_CLOCK_SKEW`, `IPSET_DOWNLOAD_FAILURES`, `IPSET_VERSION`
- `IPSET_AVERAGE_UPDATE_TIME`, `IPSET_MIN_UPDATE_TIME`, `IPSET_MAX_UPDATE_TIME`
- `IPSET_DOWNLOADER`, `IPSET_DOWNLOADER_OPTIONS`

Cache is saved after every download attempt, after every finalize, and at program exit. Atomic write (write to temp, then `mv`).

### 2.5 History/Retention System

**History**:
- After processing, the current ipset is saved in binary format to `${HISTORY_DIR}/${ipset}/<timestamp>.set`
- `history_get()` uses iprange `--union-all` on all `.set` files newer than the given window
- `history_cleanup()` deletes files older than the maximum window
- Time-windowed variants (e.g., `_1d`, `_7d`, `_30d`) are generated by merging history files

**Retention detection**:
- Tracks which IPs were added/removed between updates
- Maintains binary diff files in `${LIB_DIR}/${ipset}/new/` — each file contains the IPs that were new at that timestamp
- On each update: finds new IPs (current - latest), finds removed IPs (latest - current)
- Compares current against all historical "new" files to find which historical IPs have been removed
- Builds a histogram: `RETENTION_HISTOGRAM[hours] = count_of_ips_removed_after_N_hours`
- Builds `RETENTION_HISTOGRAM_REST[hours] = count_of_ips_still_present_after_N_hours`
- Saves histogram as bash `declare -p` in `${LIB_DIR}/${ipset}/histogram`
- Saves changeset CSV and retention CSV

### 2.6 Comparison System

After all ipsets are processed:
1. Builds arrays of all ipset files + their names
2. Separates geolocation ipsets from "regular" ipsets
3. Calls `iprange --compare` on ALL regular ipsets simultaneously — produces pairwise overlap data
4. Calls `iprange --compare-next` for each geolocation provider: updated ipsets vs country ipsets
5. Output is CSV parsed to generate per-ipset `_comparison.json` and `_*_country.json` files

### 2.7 Geolocation Mapping

Five geolocation providers:
- **GeoLite2**: MaxMind GeoLite2-Country-CSV.zip (requires license key), per-country and per-continent netsets, plus anonymous/satellite flags
- **IPDeny**: Per-country files from ipdeny.com
- **IP2Location**: IP2Location LITE DB1
- **IPIP**: ipip.net country database
- **DB-IP**: DB-IP Lite Country (gzipped CSV, monthly). URL pattern: `https://download.db-ip.com/free/dbip-country-lite-{YYYY}-{MM}.csv.gz`. Format is `ip_start,ip_end,country_code` (IP ranges, not CIDR — needs conversion). CC BY 4.0 license, no API key required.

### 2.8 Merge System

Merges multiple existing ipsets into a composite ipset:
- Concatenates all source `.ipset`/`.netset` files
- Only re-merges if any source file is newer than the merge target
- Current: 14 merge definitions (firehol_level1 through level4, abusers, webserver, proxies, anonymous, webclient, cleantalk variants)

### 2.9 Git Integration

- Commits all updated `.ipset`/`.netset` files to the git repo in `${BASE_DIR}`
- Respects `dont_redistribute` — excluded files are added to `.gitignore` instead
- Generates `README.md` per directory from `.setinfo` files
- Generates `set_file_timestamps.sh` for timestamp restoration
- Supports merged commits or per-file commits
- Supports `--amend` and force-push via config options

### 2.10 Processor Functions

All processors are bash functions used in pipes. Complete list:

**Text processors:**
- `remove_comments` — strips `#` comments, whitespace, empty lines (with `\r` -> `\n` conversion)
- `remove_comments_semi_colon` — same but for `;` comments
- `trim` — strips leading/trailing/double whitespace and empty lines
- `extract_ipv4_from_any_file` — regex-extracts IPv4 from arbitrary text
- `csv_comma_first_column` — first column of CSV
- `hostname_resolver` — resolves hostnames via parallel DNS threading

**Compression processors:**
- `gz_remove_comments` — zcat + remove_comments
- `gz_second_word` — zcat + second word extraction
- `gz_proxyrss` — zcat + remove_comments + cut port
- `unzip_and_split_csv` — funzip + comma->newline
- `unzip_and_extract` — funzip passthrough
- `p2p_gz` / `p2p_gz_ips` / `p2p_gz_proxy` — P2P blocklist format (zcat + cut + iprange)

**Format-specific parsers:**
- `dshield_parser` — DShield block format (net + mask columns)
- `snort_alert_rules_to_ipv4` — Snort alert rules -> IPs
- `pix_deny_rules_to_ipv4` — PIX ACL deny rules -> CIDRs
- `parse_rss_rosinstrument` — XML RSS -> IPs + hostname resolution
- `parse_rss_proxy` — XML RSS -> IPs (prx:ip tag)
- `parse_php_rss` — PHP RSS -> IPs (title tag)
- `parse_xml_clean_mx` — XML -> IPs (ip tag)
- `parse_dshield_api` — XML API -> IPs (ip tag + zero-padding removal)
- `subnet_to_bitmask` — netmask notation -> CIDR prefix
- `parse_asprox` — HTML div extraction
- `torproject_exits` — grep ExitAddress lines

**Internal filters** (applied automatically based on `limit` parameter):
- `filter_ip4` — strict single IPv4 (no subnet)
- `filter_net4` — strict CIDR (no /32)
- `filter_all4` — both IPs and CIDRs
- `filter_invalid4` — removes `0.0.0.0` and `/0` entries
- `append_slash32` / `remove_slash32` — CIDR normalization

### 2.11 Attributes System

**Boolean flags** (single keyword):
- `redistribute` / `dont_redistribute` — controls git inclusion
- `can_be_empty` / `never_empty` — allows zero-entry ipsets
- `no_if_modified_since` — disables conditional download
- `dont_enable_with_all` — excludes from `--enable-all`
- `inbound` / `outbound` — protection direction

**Key-value pairs** (keyword + value):
- `downloader <name>` — custom download function
- `downloader_options <string>` — custom curl options (e.g., auth headers)
- `category`, `maintainer`, `maintainer_url` — overrides
- `license`, `grade`, `protection`, `intended_use`, `false_positives`, `poisoning`
- `service`/`services` — service tags
- `public_url` — display URL (hides actual download URL)

### 2.12 Ipset Enable/Disable Mechanism

An ipset is enabled when `${BASE_DIR}/${ipset}.source` exists. To enable:
```bash
touch -t 0001010000 ${BASE_DIR}/${ipset}.source
```
`--enable-all` creates `.source` files for all defined ipsets except those with `dont_enable_with_all`.

### 2.13 Lock Mechanism

Uses file descriptor lock via `flock(2)`. Non-blocking — exits immediately if another instance is running.

---

## 3. Architectural Rules

These rules are non-negotiable and must be followed throughout the implementation.

### Rule 1: Small files, modular code

- No source file longer than ~500 lines. If it grows beyond that, split it.
- Each file has a single, clear responsibility.
- Package boundaries enforce separation of concerns.

### Rule 2: Full separation of concerns

The codebase must be organized as independent packages with clean interfaces:

```
cmd/
  update-ipsets/          # CLI entry point — flag parsing, mode dispatch
    main.go               # wire everything together
    daemon.go             # daemon mode orchestration
    batch.go              # one-shot batch mode
    query.go              # CLI IP lookup mode

pkg/
  iprange/                # IP range library — ZERO dependencies on the rest
    range.go              # core type: Range{Lo, Hi uint32}
    set.go                # IPSet: sorted []Range + optimize/merge
    set_ops.go            # union, intersect, exclude, diff, compare
    parse.go              # parse IPs, CIDRs, ranges, mixed input
    print.go              # output as CIDR, ranges, single IPs
    binary.go             # binary serialization format
    dns.go                # parallel DNS resolution
    ipv6.go               # IPv6 stub/interface for future support

  config/                 # YAML config parsing
    config.go             # main config structure
    sources.go            # source/feed definitions
    validate.go           # config validation

  downloader/             # HTTP download manager
    manager.go            # scheduling, caching, conditional GET
    client.go             # HTTP client wrapper (retry, backoff, auth)

  processor/              # data transformation pipeline
    pipeline.go           # composable processor chain
    filters.go            # remove_comments, trim, extract_ipv4, etc.
    formats.go            # CSV, JSON, DShield, Snort parsers
    decompress.go         # gzip, zip extraction

  engine/                 # update engine — orchestrates the pipeline
    engine.go             # per-ipset update cycle
    history.go            # snapshot management, time-windowed variants
    retention.go          # retention histogram computation
    finalize.go           # header generation, file writing

  geoloc/                 # geolocation databases
    geolite2.go           # MaxMind GeoLite2 parser
    ipdeny.go             # IPDeny parser
    ip2location.go        # IP2Location parser
    ipip.go               # IPIP parser
    dbip.go               # DB-IP parser
    compare.go            # cross-reference ipsets with country data

  web/                    # built-in web server
    server.go             # HTTP server setup, routing
    static.go             # serve static files (website HTML/CSS/JS)
    api.go                # REST API handlers
    api_search.go         # IP lookup endpoint
    admin.go              # admin interface handlers
    templates/            # Go templates for admin UI
    compat.go             # old URL compatibility layer

  kernel/                 # kernel ipset management
    ipset.go              # netlink interface for ipset load/swap/destroy

  scheduler/              # internal update scheduler
    scheduler.go          # priority queue, timing, backoff
    state.go              # persistence, crash recovery

  output/                 # output file generation
    json.go               # per-ipset JSON, all-ipsets.json
    csv.go                # history CSV, changesets CSV
    sitemap.go            # sitemap.xml
    comparison.go         # pairwise overlap JSON
    git.go                # git add/commit/push integration
```

**The `iprange` package must have ZERO imports from other packages in this project.** It is a standalone library that could be published separately.

### Rule 3: CLI modes

```
update-ipsets daemon [flags]           # main mode: scheduler + web + API + admin
update-ipsets run [ipset...] [flags]   # one-shot batch mode (backwards compatible)
update-ipsets query <ip> [flags]       # which lists contain this IP?
update-ipsets iprange [flags]          # standalone iprange operations (drop-in replacement)
update-ipsets enable <ipset...>        # enable ipsets (create .source files)
update-ipsets version                  # version info
```

The `iprange` subcommand must accept the same flags as the current C `iprange` tool for backwards compatibility: `--compare`, `--compare-next`, `--compare-first`, `--count-unique`, `--count-unique-all`, `--exclude`, `--diff`, `--print-ranges`, `--print-binary`, `--print-single-ips`, `--min-prefix`, `--default-prefix`, `--dns-threads`, etc.

### Rule 4: iprange as a library, IPv6-ready

The `iprange` package must:
- Define an interface/generic that works for both IPv4 and IPv6
- IPv4 implementation complete from day one
- IPv6 as a stub/interface that can be filled in later without changing the API
- Practical approach: build IPv4 first with concrete types, then refactor to generics when IPv6 is needed. Keep the public API clean enough that this refactor won't break callers.

### Rule 5: Composable ipset operations via API and CLI

Users must be able to combine and manipulate ipsets at query time:

**CLI examples:**
```bash
# Get firehol_level1 but exclude bogons
update-ipsets query --set "firehol_level1 - bogons"

# Combine multiple lists
update-ipsets query --set "firehol_level1 + firehol_level2 + firehol_level3"

# Check if an IP is in a combined set excluding another
update-ipsets query --set "firehol_level1 - bogons" --ip 1.2.3.4

# Download a composed set via the web API
# GET /api/v1/compose?include=firehol_level1,firehol_level2&exclude=bogons
```

**API endpoint:**
```
GET /api/v1/compose?include=set1,set2&exclude=set3,set4&format=cidr|range|single
```

---

## 4. Configuration Language Design

### 4.1 Main Configuration File

Location: `${SYSCONFDIR}/firehol/update-ipsets.yaml` (replaces `update-ipsets.conf`)

```yaml
runtime:
  base_dir: "/etc/firehol/ipsets"
  tmp_dir: "/dev/shm"
  cache_dir: "/var/cache/update-ipsets"
  lib_dir: "/var/lib/update-ipsets"
  history_dir: "{base_dir}/history"
  errors_dir: "{base_dir}/errors"
  web_dir: "/var/www/blocklists"
  web_dir_for_ipsets: "/var/www/blocklists/files"
  web_owner: "www-data:www-data"
  web_url: "http://iplists.firehol.org/?ipset="
  local_copy_url: "https://iplists.firehol.org/files/"
  github_changes_url: "https://github.com/firehol/blocklist-ipsets/commits/master/"
  github_setinfo: "https://github.com/firehol/blocklist-ipsets/tree/master/"
  lock_file: "/run/update-ipsets.lock"
  user_agent: "FireHOL-Update-Ipsets/4.0 (linux) https://iplists.firehol.org/"

  max_connect_time: 10
  max_download_time: 300
  ignore_repeating_download_errors: 10
  parallel_downloads: 8
  parallel_dns_queries: 300

  ipset_reduce_factor: 20
  ipset_reduce_entries: 65536
  web_charts_entries: 500

  push_to_git: false
  push_to_git_merged: true
  push_to_git_commit_options: "--amend"
  push_to_git_push_options: "-f"
  push_to_git_web: false

  ipsets_apply: true
```

### 4.2 Ipset Source Definition File

Location: `${SYSCONFDIR}/firehol/ipsets.d/*.yaml` or embedded in main config under `sources:`.

**Processor pipeline design**: Processors are composable, defined as a list:

```yaml
# Built-in processors:
#   remove_comments         - strip # comments, whitespace, empty lines, \r->\n
#   remove_comments_semi    - strip ; comments, same as above
#   trim                    - strip whitespace, empty lines
#   extract_ipv4            - regex-extract IPv4 from any text
#   csv_column N            - extract Nth column (1-based) from CSV
#   cut_delimiter D field N - cut by delimiter D, field N
#   gunzip                  - decompress gzip
#   unzip                   - extract first file from zip
#   unzip_file PATTERN      - extract specific file from zip
#   unzip_csv               - extract first file from zip, comma->newline
#   p2p_blocklist           - P2P blocklist format (Name:IP-IP ranges)
#   snort_rules             - extract IPs from Snort alert rules
#   pix_deny_rules          - extract CIDRs from PIX ACL deny rules
#   dshield_format          - DShield block.txt format (net mask columns)
#   xml_tag TAG             - extract content of XML tag
#   xml_rss_title           - extract IPs from RSS title tags
#   xml_rss_title_resolve   - same but resolve hostnames
#   xml_rss_proxy           - extract IPs from prx:ip RSS tags
#   regex PATTERN           - extract matches of regex pattern
#   grep PATTERN            - filter lines matching pattern
#   grep_not PATTERN        - filter lines NOT matching pattern
#   hostname_resolve        - resolve hostnames to IPs via DNS
#   subnet_to_cidr          - convert netmask notation to CIDR
#   json_path PATH          - extract from JSON
#   passthrough             - no processing (cat)
```

### 4.3 Example YAML — Real Ipset Definitions

```yaml
sources:
  dshield:
    url: "https://feeds.dshield.org/block.txt"
    frequency: 10
    history: [1440, 10080, 43200]
    ipv: ipv4
    output: both
    processor: [dshield_format]
    category: attacks
    info: >
      [DShield.org](https://dshield.org/) top 20 attacking class C (/24)
      subnets over the last three days
    maintainer: DShield.org
    maintainer_url: "https://dshield.org/"

  tor_exits:
    url: "https://check.torproject.org/exit-addresses"
    frequency: 5
    history: [1440, 10080, 43200]
    ipv: ipv4
    output: ip
    processor:
      - grep: "^ExitAddress "
      - cut_delimiter: " " field 2
    category: anonymizers
    info: >
      [TorProject.org](https://www.torproject.org) list of all current TOR
      exit points (TorDNSEL)
    maintainer: TorProject.org
    maintainer_url: "https://www.torproject.org/"

  griffinguard:
    url: "https://griffinguard.io/feeds/abuse7d_top10k.txt"
    frequency: 30
    ipv: ipv4
    output: ip
    processor: [csv_comma_first_column]
    category: attacks
    info: >
      [GriffinGuard](https://griffinguard.io/) top 10K abusive IP addresses
    maintainer: GriffinGuard
    maintainer_url: "https://griffinguard.io/"
    attributes:
      dont_redistribute: true

  bogons:
    url: "http://www.team-cymru.org/Services/Bogons/bogon-bn-agg.txt"
    frequency: 1440
    ipv: ipv4
    output: both
    processor: [remove_comments]
    category: unroutable
    info: >
      [Team-Cymru.org](http://www.team-cymru.org) private and reserved
      addresses defined by RFC 1918, RFC 5735, and RFC 6598
    maintainer: Team Cymru
    maintainer_url: "http://www.team-cymru.org/"
    attributes:
      dont_redistribute: true

geolocation:
  geolite2_country:
    url: "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country-CSV&license_key={maxmind_license_key}&suffix=zip"
    frequency: 10080
    type: maxmind_country_csv
    info: "[MaxMind GeoLite2](http://dev.maxmind.com/geoip/geoip2/geolite2/)"
    maintainer: MaxMind.com
    maintainer_url: "http://www.maxmind.com/"

  dbip_country:
    url: "https://download.db-ip.com/free/dbip-country-lite-{YYYY-MM}.csv.gz"
    frequency: 43200
    type: dbip_country_csv
    info: "[DB-IP](https://db-ip.com/) IP to Country Lite database"
    maintainer: DB-IP.com
    maintainer_url: "https://db-ip.com/"

merges:
  firehol_level1:
    ipv: ipv4
    output: both
    category: attacks
    info: >
      A firewall blacklist composed from IP lists, providing maximum protection
      with minimum false positives.
    maintainer: FireHOL
    maintainer_url: "http://iplists.firehol.org/"
    sources:
      - dshield
      - feodo
      - fullbogons
      - spamhaus_drop

renames:
  tor: et_tor
  compromised: et_compromised
  botnet: et_botcc
  autoshun: shunlist
  stop_forum_spam: stopforumspam

deleted:
  - openbl
  - openbl_1d
  - atlas_attacks
  - palevo
  - infiltrated
```

**IMPORTANT**: The YAML configuration must include ALL ~166 active ipset sources from the original bash script. Extract every `update` and `merge` call from the bash script and convert to YAML format.

---

## 5. Core Data Structures (Go)

### 5.1 IP Range Storage

```go
// Range represents a single IP range: [Lo, Hi] inclusive.
// 8 bytes — matches C struct exactly for binary format compatibility.
type Range struct {
    Lo uint32 // start IP, host byte order
    Hi uint32 // end IP, host byte order
}

// IPSet is a set of IP ranges, sorted and non-overlapping after optimization.
type IPSet struct {
    Name      string
    Ranges    []Range   // sorted & optimized if Optimized==true
    Lines     int       // input lines counted
    UniqueIPs uint64    // sum of (Hi - Lo + 1)
    Optimized bool
}
```

Memory: 8 bytes per range. A typical ipset with 50,000 entries = 400KB. All 166+ ipsets loaded simultaneously ~= 70MB.

### 5.2 Ipset Metadata

```go
type IpsetMetadata struct {
    Name             string
    Info             string
    SourceFile       string
    OutputFile       string
    URL              string
    IPV              string    // "ipv4"
    Hash             string    // "ip" or "net"
    FrequencyMins    uint32
    HistoryWindows   []uint32  // minutes
    Category         string
    Maintainer       string
    MaintainerURL    string

    // Tracking
    Entries          int
    UniqueIPs        uint64
    EntriesMin       int
    EntriesMax       int
    IPsMin           uint64
    IPsMax           uint64
    SourceDate       int64     // unix timestamp
    ProcessedDate    int64
    CheckedDate      int64
    StartedDate      int64
    ClockSkew        int64
    DownloadFailures uint32
    Version          uint32
    AvgUpdateTime    uint32    // minutes
    MinUpdateTime    uint32
    MaxUpdateTime    uint32

    // Attributes
    License          string
    Grade            string
    Protection       string
    IntendedUse      string
    FalsePositives   string
    Poisoning        string
    Services         []string
    DontRedistribute bool
    AcceptEmpty      bool
    NoIfModifiedSince bool
    DontEnableWithAll bool
    Downloader       string
    DownloaderOptions string
}
```

---

## 6. Algorithm Specifications

### 6.1 IP Range Optimization (Sort + Merge)

```
1. Sort by (Lo ASC, Hi DESC)                    // O(n log n)
2. Sweep left-to-right:                          // O(n)
   lo = ranges[0].Lo, hi = ranges[0].Hi
   for i in 1..n:
     if ranges[i].Hi <= hi: continue             // fully contained
     if ranges[i].Lo <= hi + 1: hi = ranges[i].Hi  // overlapping/adjacent
     else: emit(lo, hi); lo = ranges[i].Lo; hi = ranges[i].Hi
   emit(lo, hi)
```

### 6.2 IP Range Exclusion (A - B)

Both inputs must be optimized. Two-pointer sweep:
```
i1 = 0, i2 = 0
while i1 < n1 && i2 < n2:
  if lo1 > hi2: advance i2
  if lo2 > hi1: emit(lo1, hi1); advance i1
  // overlap:
  if lo1 < lo2: emit(lo1, lo2-1); lo1 = lo2
  if hi1 == hi2: advance both
  if hi1 < hi2: advance i1
  if hi1 > hi2: lo1 = hi2+1; advance i2
emit remaining from A
```

### 6.3 IP Range Intersection (A ∩ B)

```
while i1 < n1 && i2 < n2:
  if lo1 > hi2: advance i2
  if lo2 > hi1: advance i1
  lo = max(lo1, lo2)
  hi = min(hi1, hi2)
  emit(lo, hi)
  advance whichever has lower hi
```

### 6.4 IP Range Diff (A △ B — Symmetric Difference)

Two-pointer sweep, emit non-overlapping parts of both.

### 6.5 IP Range Comparison (Pairwise)

For N ipsets, compute N*(N-1)/2 comparisons. For each pair (A, B):
```
combined = combine(A, B)    // concatenate
optimize(combined)
common_ips = A.unique_ips + B.unique_ips - combined.unique_ips
Output CSV: name1,name2,entries1,entries2,ips1,ips2,combined.unique_ips,common_ips
```

### 6.6 IP Range Reduce (Prefix Optimization)

For kernel ipsets where fewer prefixes = faster lookups:
```
1. Decompose all ranges into CIDRs, count per prefix
2. Iteratively merge smallest prefix into next larger, tracking entry increase
3. Stop when increase exceeds acceptable_increase% or min_accepted entries
4. Disable eliminated prefixes in prefix_enabled[] array
```

### 6.7 CIDR Decomposition (split_range)

Recursive binary split of range [lo, hi] into optimal CIDRs:
```
split_range(addr, prefix, lo, hi):
  bc = broadcast(addr, prefix)
  if lo == addr && hi == bc && prefix_enabled[prefix]: emit(addr/prefix); return
  upper = addr | (1 << (31 - prefix))
  if hi < upper: recurse lower half
  if lo >= upper: recurse upper half
  else: recurse both halves
```
Max recursion depth: 32. Max CIDRs for 0.0.0.1-255.255.255.254: 62.

### 6.8 Retention Histogram

Per update:
1. Find new IPs: `current_ipset - latest_saved` (exclude operation)
2. Find removed IPs: `latest_saved - current_ipset` (exclude operation)
3. Save new IPs as binary file `new/<timestamp>`
4. Compare current against all `new/*` files to find which historical batches lost IPs
5. For each affected batch: find remaining IPs, record removals in histogram keyed by hours-since-addition
6. Clean up empty batch files

### 6.9 Download Manager

```
For each ipset:
  1. Check elapsed time since last check
  2. Apply failure backoff (>10 failures: multiply interval by (failures-10))
  3. Apply success acceleration (<10 failures: halve interval)
  4. If too soon: skip
  5. HTTP GET with If-Modified-Since (unless no_if_modified_since)
  6. Handle 304 Not Modified -> update check time, skip processing
  7. Handle success -> diff with previous .source, skip if identical
  8. Handle failure -> increment failure counter
```

---

## 7. Output File Format Specifications

### 7.1 `.ipset` / `.netset` Format

```
#
# {name}
#
# {ipv} hash:{hash} ipset
#
# {info text wrapped at 60 chars, each line prefixed with "# "}
#
# Maintainer      : {maintainer}
# Maintainer URL  : {maintainer_url}
# List source URL : {url}
# Source File Date: {UTC date of .source file}
#
# Category        : {category}
# Version         : {version}
#
# This File Date  : {current UTC date}
# Update Frequency: {human readable, e.g. "30 mins"}
# Aggregation     : {human readable or "none"}
# Entries         : {entries} subnets, {ips} unique IPs  (or just "{ips} unique IPs" for ip hash)
#
# Full list analysis, including geolocation map, history,
# retention policy, overlaps with other lists, etc.
# available at:
#
#  {web_url}{name}
#
# Generated by FireHOL's update-ipsets.sh
# Processed with FireHOL's iprange
#
{CIDR entries, one per line}
```

`.ipset` = hash:ip (single IPs without /32), `.netset` = hash:net (CIDRs).

### 7.2 `.setinfo` Format

Single line:
```
[{name}]({web_url}{name})|{info}|{ipv} hash:{hash}|{quantity}|updated every {frequency} from [this link]({url})
```

### 7.3 Per-ipset `.json` Format

```json
{
    "name": "{name}",
    "entries": 1234,
    "entries_min": 1000,
    "entries_max": 1500,
    "ips": 5678,
    "ips_min": 5000,
    "ips_max": 6000,
    "ipv": "ipv4",
    "hash": "ip",
    "frequency": 30,
    "aggregation": 0,
    "started": 1616000000000,
    "updated": 1616000000000,
    "processed": 1616000000000,
    "checked": 1616000000000,
    "clock_skew": 0,
    "category": "attacks",
    "maintainer": "Example",
    "maintainer_url": "https://example.com/",
    "info": "Description with <a href=\"url\">links</a>",
    "source": "https://example.com/feed.txt",
    "file": "example.ipset",
    "history": "example_history.csv",
    "geolite2": "example_geolite2_country.json",
    "ipdeny": "example_ipdeny_country.json",
    "ip2location": "example_ip2location_country.json",
    "ipip": "example_ipip_country.json",
    "dbip": "example_dbip_country.json",
    "comparison": "example_comparison.json",
    "file_local": "https://iplists.firehol.org/files/example.ipset",
    "commit_history": "https://github.com/firehol/blocklist-ipsets/commits/master/example.ipset",
    "license": "",
    "grade": "",
    "protection": "",
    "intended_use": "",
    "false_positives": "",
    "poisoning": "",
    "services": [],
    "errors": 0,
    "version": 1,
    "average_update": 30,
    "min_update": 25,
    "max_update": 45,
    "downloader": ""
}
```

Note: Timestamps are in **milliseconds** (unix_seconds * 1000). `dont_redistribute` ipsets have empty `source`, `file_local`, `commit_history`.

Info field: Markdown `[text](url)` is converted to `<a href="url">text</a>` HTML. Quotes are escaped.

### 7.4 `all-ipsets.json` Format

```json
[
    {
        "ipset": "{name}",
        "category": "{category}",
        "maintainer": "{maintainer}",
        "started": 1616000000000,
        "updated": 1616000000000,
        "checked": 1616000000000,
        "clock_skew": 0,
        "ips": 5678,
        "errors": 0
    }
]
```

Only includes non-geolocation ipsets. Checked is `max(checked_date, processed_date)`.

### 7.5 `_history.csv` Format

```csv
DateTime,Entries,UniqueIPs
1616000000,1234,5678
```

Last 500 entries, sorted by DateTime ascending.

### 7.6 `_retention.json` Format

```json
{
    "ipset": "{name}",
    "started": 1616000000000,
    "updated": 1616000000000,
    "incomplete": 0,
    "past": {
        "hours": [1, 2, 3],
        "ips": [100, 50, 25],
        "total": 175
    },
    "current": {
        "hours": [1, 2, 3],
        "ips": [200, 150, 100],
        "total": 450
    }
}
```

### 7.7 `_comparison.json` Format

```json
[
    {
        "name": "{other_ipset}",
        "category": "{other_category}",
        "ips": 5678,
        "common": 1234
    }
]
```

Only entries with `common > 0`. Bidirectional.

### 7.8 `_*_country.json` Format

```json
[
    {"code": "US", "value": 12345},
    {"code": "CN", "value": 6789}
]
```

Only entries with `value > 0`. Country codes uppercase.

### 7.9 `_changesets.csv` Format

```csv
DateTime,AddedIPs,RemovedIPs
1616000000,100,50
```

Last 500 entries.

### 7.10 `sitemap.xml` Format

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
    <url>
        <loc>http://iplists.firehol.org/</loc>
        <lastmod>2024-01-15</lastmod>
        <changefreq>always</changefreq>
    </url>
    <url>
        <loc>http://iplists.firehol.org/?ipset=dshield</loc>
        <lastmod>2024-01-15</lastmod>
        <changefreq>always</changefreq>
    </url>
</urlset>
```

### 7.11 Binary Format (iprange v1.0)

```
iprange binary format v1.0\n
optimized\n                          (or "non-optimized\n")
record size {sizeof(network_addr_t)}\n   (always 8)
records {N}\n
bytes {N * 8 + 4}\n
lines {N}\n
unique ips {count}\n
{4 bytes: endianness marker 0x1A2B3C4D}
{N * 8 bytes: raw network_addr_t array}
```

---

## 8. Compatibility Requirements

1. **Output files must be EXACTLY the same format** — the JavaScript website (`index.html`) parses all JSON/CSV/XML files.
2. **`.source` file mechanism**: Check for `${BASE_DIR}/${ipset}.source` to determine if an ipset is enabled.
3. **Command-line interface**: Must support `enable`, `run`, `--silent`, `--verbose`, `--push-git`, `--recheck`, `--enable-all`, `--rebuild`, `--reprocess`, `--cleanup`, `--config`, `--help`.
4. **Configuration**: Must read both existing `.conf` format (for migration) and new YAML format.
5. **Lock file**: Must use same `flock` mechanism on same path.
6. **Cache**: Must read existing `.cache` file format for migration, then switch to a new format (JSON).
7. **Binary iprange format**: Must read/write the same `v1.0` format for history/retention binary files.
8. **Directory structure**: `${BASE_DIR}`, `${HISTORY_DIR}`, `${LIB_DIR}`, `${CACHE_DIR}` paths must be compatible.
9. **Git integration**: Same commit pattern, same `.gitignore` management.
10. **User vs root mode**: When not root, use `$HOME/ipsets/` as BASE_DIR and disable kernel ipset operations.

---

## 9. Performance Targets

| Metric | Current | Target |
|--------|---------|--------|
| Full run (all ipsets) | 60-90 minutes | < 15 minutes |
| Pairwise comparison (166 ipsets) | ~10 minutes | < 30 seconds |
| Memory usage | Unbounded | < 512 MB peak |
| Download concurrency | 1 (sequential) | 8-16 parallel |
| Disk I/O | Many small writes | Buffered, batched writes |
| Startup | ~5s | < 100ms |

---

## 10. Implementation Phases

### Phase 1: Core IP Range Library (`pkg/iprange/`)
- `Range` struct with `(uint32, uint32)` representation
- `IPSet` container with dynamic slice
- Algorithms: optimize (sort+merge), exclude, intersect, diff, combine
- Compare: pairwise CSV output matching current format
- Reduce: prefix optimization for kernel ipsets
- Print: CIDR, range, single IP, binary output modes
- Binary format: read/write v1.0 compatible with C implementation
- Load: parse text input (IPs, CIDRs, ranges, comments, hostnames)
- DNS resolution: parallel hostname resolution
- **Test**: Verify bit-exact output against C iprange for all operations

### Phase 2: Configuration Parser (`pkg/config/`)
- YAML schema for all ipset definitions
- Built-in processor registry
- Processor pipeline composition
- Merge/composite definition support
- Migration path: read existing `.conf` format
- Attribute parsing
- **Test**: Parse all ~166 current ipset definitions correctly

### Phase 3: Download Manager (`pkg/downloader/`)
- HTTP client with: conditional GET, compression, redirect following, timeouts
- Parallel downloads with configurable concurrency
- Failure tracking and backoff
- File deduplication (compare with previous .source)
- Custom download options (auth headers, etc.)
- **Test**: Download sources, verify same behavior as curl

### Phase 4: Processing Pipeline (`pkg/processor/`)
- Implement all built-in processors as composable transformations
- Text processing: comment removal, whitespace normalization, regex extraction
- Compression: gzip, zip decompression
- Format parsers: DShield, Snort, PIX, P2P blocklist, CSV, XML
- IPv4 filter chain (filter_ip4, filter_net4, filter_all4, filter_invalid4)
- **Test**: Process sources, verify identical output

### Phase 5: History and Retention (`pkg/engine/`)
- History snapshot management (binary files in HISTORY_DIR)
- Time-windowed variant generation (_1d, _7d, _30d)
- History cleanup (delete files older than max window)
- Retention detection (new IPs, removed IPs, histogram tracking)
- Changeset CSV generation
- **Test**: Verify retention histogram computation

### Phase 6: Geolocation Mapping (`pkg/geoloc/`)
- GeoLite2 Country CSV parser (zip extraction, country/continent assignment, authenticated download)
- IPDeny country file aggregation
- IP2Location LITE DB1 parser
- IPIP country database parser
- DB-IP Lite Country CSV parser (gzipped CSV, IP range -> CIDR conversion, monthly URL pattern)
- Cross-reference comparison (ipsets vs country databases)
- **Test**: Verify country JSON output format

### Phase 7: Web File Generation (`pkg/output/`)
- Per-ipset JSON generation
- all-ipsets.json index
- History CSV (last 500 entries)
- History statistics (avg/min/max update times)
- Retention JSON
- Comparison JSON (pairwise overlap)
- Country JSON (per geo provider)
- Changesets CSV
- sitemap.xml
- **Test**: Compare all web output formats against specification

### Phase 8: Git Integration (`pkg/output/`)
- Detect git repos in BASE_DIR and WEB_DIR
- Add/commit changed files (respecting dont_redistribute)
- .gitignore management
- README.md generation from .setinfo files
- set_file_timestamps.sh generation
- Push support (optional, configurable)
- **Test**: Verify git operations

### Phase 9: CLI Compatibility Layer (`cmd/update-ipsets/`)
- Command-line parsing matching current interface
- `enable`, `run`, `--silent`, `--verbose`, etc.
- Configuration file loading (both .conf and .yaml)
- Lock file mechanism (flock)
- Cache load/save (read old bash format, write new JSON format)
- Kernel ipset interaction via netlink (create, restore, swap, destroy)
- User vs root mode detection
- **Test**: Drop-in replacement for batch mode

### Phase 10: Daemon Mode & Internal Scheduler (`pkg/scheduler/`)
- Long-running daemon with signal handling (SIGHUP=reload config, SIGTERM=graceful shutdown)
- Internal scheduler: per-source update intervals, backoff, priority queue
- Parallel downloads with sequential processing per ipset
- Kernel ipset management (detect loaded ipsets, swap atomic)
- State persistence for crash recovery
- Systemd integration: notify, watchdog, journal logging
- **Test**: Daemon stability test

### Phase 11: Built-in Web Server (`pkg/web/`)
- **Static files**: Website HTML/CSS/JS embedded in binary via `//go:embed`. Override with `--web-dir` for disk serving.
- Serve generated data files (JSON, CSV) from memory
- Serve ipset download files (.ipset, .netset) from BASE_DIR
- **sitemap.xml**: Generated dynamically
- HTTPS support or reverse proxy mode
- Cloudflare-compatible (X-Real-IP, CF-Connecting-IP headers)
- Gzip compression, cache headers, rate limiting
- **Test**: Verify all website functionality

### Phase 12: REST API (`pkg/web/`)
- `GET /api/v1/ipsets` — list all ipsets
- `GET /api/v1/ipsets/{name}` — ipset detail
- `GET /api/v1/ipsets/{name}/data` — download the ipset file
- `GET /api/v1/ipsets/{name}/history` — history CSV
- `GET /api/v1/ipsets/{name}/retention` — retention data
- `GET /api/v1/ipsets/{name}/countries/{provider}` — geo data
- `GET /api/v1/ipsets/{name}/comparison` — overlap data
- `GET /api/v1/search?ip={ip}` — IP lookup: binary search each ipset, O(log n)
- `GET /api/v1/compose?include=...&exclude=...&format=...` — dynamic composition
- `GET /api/v1/status` — scheduler status
- **Backwards compatibility**: All old URLs must continue to work:
  - `/?ipset={name}` -> ipset page
  - `/files/{name}.ipset` -> download
  - `/{name}.json` -> ipset detail
  - `/all-ipsets.json` -> ipset list
  - `/{name}_history.csv`, `/{name}_retention.json`, etc.
- CORS headers
- **Test**: Verify API responses match static file equivalents

### Phase 13: Admin Interface (`pkg/web/`)
- Web UI at `/admin/` (protected by basic auth or API key)
- Dashboard: all feeds status, last/next update, error count, IP count
- Schedule view: timeline/queue
- Feed detail: history, download logs, processing time, errors
- Manual controls: trigger update, enable/disable feeds
- System status: memory, uptime, goroutines, disk usage
- Built with minimal JS (htmx or vanilla JS + Go templates)
- **Test**: All admin actions work

### Phase 14: YAML Source Extraction
- Parse ALL ~166 `update` calls from the original bash script
- Parse ALL 14 `merge` calls
- Parse ALL renames and deleted ipsets
- Convert to the YAML format specified above
- Include ALL attributes, processor pipelines, history windows
- Verify completeness (count of sources in YAML == count in bash script)
- **Test**: Validate every source definition matches the bash original

---

## 11. Testing Strategy

### 11.1 Unit Tests

Every package must have comprehensive unit tests:
- `pkg/iprange/`: test all operations with edge cases (empty sets, single IP, full /0, adjacent ranges, overlapping ranges, uint32 overflow boundaries)
- `pkg/config/`: test YAML parsing, validation, defaults, migration from .conf
- `pkg/downloader/`: test conditional GET logic, backoff, failure counting (use httptest)
- `pkg/processor/`: test each processor with real and edge-case input
- `pkg/engine/`: test history window calculation, retention histogram
- `pkg/geoloc/`: test each parser format
- `pkg/output/`: test JSON/CSV/XML output format correctness
- `pkg/web/`: test API endpoints with httptest
- `pkg/scheduler/`: test scheduling logic, priority queue, backoff

### 11.2 Property-Based Testing

- Merge is commutative and associative
- Exclude: `|A - B| + |A ∩ B| = |A|`
- Common: `A ∩ B ⊆ A` and `A ∩ B ⊆ B`
- Diff: `A △ B = (A - B) ∪ (B - A)`
- Optimize is idempotent
- `unique_ips` count is exact after optimization

### 11.3 Fuzzing

- Feed random bytes to the IP parser
- Feed random CIDR strings with edge cases (0.0.0.0/0, 255.255.255.255/32, malformed)
- Feed truncated binary format files
- Feed empty files, huge files, files with only comments

### 11.4 Integration Tests

- Round-trip: write binary -> read binary -> verify identical IPSet
- Process a real `.source` file through the full pipeline -> verify output matches
- Download a real feed URL -> process -> verify output is valid

### 11.5 Benchmarks

Every performance-critical operation MUST have a Go benchmark (`BenchmarkXxx`):

- `BenchmarkParseIPs` — parse 1M lines of IPs
- `BenchmarkOptimize` — optimize 1M unsorted ranges
- `BenchmarkExclude` — exclude 100K ranges from 1M ranges
- `BenchmarkIntersect` — intersect two 500K-range sets
- `BenchmarkCompare` — pairwise compare 100 ipsets
- `BenchmarkCIDRPrint` — print 1M ranges as CIDRs
- `BenchmarkBinaryWrite` / `BenchmarkBinaryRead` — binary serialization
- `BenchmarkJSONGenerate` — generate per-ipset JSON
- `BenchmarkRetentionHistogram` — retention computation

**Performance targets:**
- Parse 1M lines of IPs: < 200ms
- Optimize 1M ranges: < 500ms
- Compare 100 ipsets pairwise: < 30s
- Memory peak during full run: < 512MB

---

## VERIFICATION CHECKLIST

**You must complete EVERY item below. Check each item after implementation and testing.**
**2026-04-01 status:** every row below was re-verified through an executed code path in tests, live fixture runs, shell commands, or direct artifact inspection during the final verification pass above.

### A. Project Setup
- [x] A1. `go mod init` with appropriate module path
- [x] A2. Directory structure matches the package layout in Rule 2
- [x] A3. Makefile with targets: `build`, `test`, `bench`, `lint`, `clean`, `install`
- [x] A4. `.gitignore` for Go binary, vendor, tmp files
- [x] A5. Build produces a single static binary

### B. IP Range Library (`pkg/iprange/`)
- [x] B1. `Range` type: `Lo uint32`, `Hi uint32`
- [x] B2. `IPSet` type with sorted `[]Range`, metadata, optimized flag
- [x] B3. `Optimize()`: sort + merge overlapping/adjacent ranges — unit tested
- [x] B4. `Exclude(a, b)`: A - B two-pointer sweep — unit tested
- [x] B5. `Intersect(a, b)`: A ∩ B two-pointer sweep — unit tested
- [x] B6. `Diff(a, b)`: A △ B symmetric difference — unit tested
- [x] B7. `Combine(a, b)`: union (concatenate + re-optimize) — unit tested
- [x] B8. `Compare(sets)`: pairwise N*(N-1)/2 comparison, CSV output — unit tested
- [x] B9. `CompareFirst(sets)`: first vs all others — unit tested
- [x] B10. `CompareNext(before, after)`: two groups comparison — unit tested
- [x] B11. `CountUnique()` / `CountUniqueAll()` — unit tested
- [x] B12. `Reduce()`: prefix optimization for kernel ipsets — unit tested
- [x] B13. `PrintCIDR()`: range to optimal CIDRs via split_range — unit tested
- [x] B14. `PrintRanges()`: output as IP-IP ranges — unit tested
- [x] B15. `PrintSingleIPs()`: output individual IPs — unit tested
- [x] B16. `WriteBinary()`: write v1.0 binary format with endianness marker — unit tested
- [x] B17. `ReadBinary()`: read v1.0 binary format — unit tested
- [x] B18. Binary round-trip test: write -> read -> verify identical
- [x] B19. `Parse()`: parse IPs, CIDRs, ranges, comments, mixed input — unit tested
- [x] B20. `Parse()` handles: `1.2.3.4`, `1.2.3.0/24`, `1.2.3.4-1.2.3.10`, comments, blank lines
- [x] B21. DNS resolution: parallel hostname resolution with configurable thread count
- [x] B22. Edge cases tested: empty set, single IP, /0, /32, 0.0.0.0, 255.255.255.255, adjacent ranges
- [x] B23. Property-based tests: commutativity, associativity, set identities
- [x] B24. Fuzz tests: random input to parser, truncated binary files
- [x] B25. Benchmarks: parse, optimize, exclude, intersect, compare, print, binary I/O
- [x] B26. Zero imports from other packages in this project
- [x] B27. IPv6 interface/stub defined (can be filled in later)
- [x] B28. All tests pass with `-race` flag

### C. Configuration (`pkg/config/`)
- [x] C1. YAML config parsing for runtime settings
- [x] C2. YAML config parsing for ipset source definitions
- [x] C3. YAML config parsing for merge definitions
- [x] C4. YAML config parsing for renames and deleted ipsets
- [x] C5. YAML config parsing for geolocation sources
- [x] C6. Processor pipeline parsing (list of processors with params)
- [x] C7. Attribute parsing (all boolean flags and key-value pairs)
- [x] C8. Config validation (required fields, valid values, URL format)
- [x] C9. Legacy `.conf` file reading (for migration)
- [x] C10. Default values for all optional fields
- [x] C11. `ipsets.d/*.yaml` directory scanning for additional sources
- [x] C12. ALL ~166 ipset sources extracted from bash script into YAML
- [x] C13. ALL 14 merge definitions extracted into YAML
- [x] C14. ALL renames extracted into YAML
- [x] C15. ALL deleted ipsets extracted into YAML
- [x] C16. Unit tests for config parsing

### D. Download Manager (`pkg/downloader/`)
- [x] D1. HTTP client with configurable timeouts, user-agent, compression
- [x] D2. Conditional GET (If-Modified-Since header)
- [x] D3. HTTP 304 Not Modified handling
- [x] D4. Redirect following
- [x] D5. Failure counter and backoff logic (>10 failures: multiply by failures-10)
- [x] D6. Success acceleration (<10 failures: halve interval)
- [x] D7. File deduplication (compare downloaded vs existing .source)
- [x] D8. Parallel downloads with configurable concurrency
- [x] D9. Custom downloader support (auth headers, custom options)
- [x] D10. Remote timestamp preservation
- [x] D11. Frequency control with 1% margin (capped at 10min)
- [x] D12. Unit tests with httptest mock server
- [x] D13. All tests pass with `-race` flag

### E. Processing Pipeline (`pkg/processor/`)
- [x] E1. `remove_comments` — strip # comments, \r->\n, whitespace — tested
- [x] E2. `remove_comments_semi` — strip ; comments — tested
- [x] E3. `trim` — strip whitespace, empty lines — tested
- [x] E4. `extract_ipv4` — regex-extract IPv4 from any text — tested
- [x] E5. `csv_column N` — extract Nth column — tested
- [x] E6. `cut_delimiter D field N` — cut by delimiter — tested
- [x] E7. `gunzip` — gzip decompression — tested
- [x] E8. `unzip` — zip extraction — tested
- [x] E9. `unzip_csv` — zip + comma->newline — tested
- [x] E10. `p2p_blocklist` — P2P format (Name:IP-IP) — tested
- [x] E11. `snort_rules` — Snort alert rules to IPs — tested
- [x] E12. `pix_deny_rules` — PIX ACL deny to CIDRs — tested
- [x] E13. `dshield_format` — DShield block.txt format — tested
- [x] E14. `xml_tag TAG` — XML tag extraction — tested
- [x] E15. `xml_rss_title` — RSS title extraction — tested
- [x] E16. `xml_rss_proxy` — RSS prx:ip extraction — tested
- [x] E17. `regex PATTERN` — regex extraction — tested
- [x] E18. `grep PATTERN` / `grep_not PATTERN` — line filtering — tested
- [x] E19. `hostname_resolve` — DNS resolution — tested
- [x] E20. `subnet_to_cidr` — netmask to CIDR — tested
- [x] E21. `json_path PATH` — JSON extraction — tested
- [x] E22. `passthrough` — no-op — tested
- [x] E23. Pipeline composition (chain multiple processors) — tested
- [x] E24. IPv4 filter chain: filter_ip4, filter_net4, filter_all4, filter_invalid4 — tested
- [x] E25. append_slash32 / remove_slash32 — tested
- [x] E26. All processors handle empty input, malformed input, huge input

### F. Update Engine (`pkg/engine/`)
- [x] F1. Per-ipset update cycle: check -> download -> compare -> process -> finalize
- [x] F2. History snapshot saving (binary format)
- [x] F3. Time-windowed variant generation (_1d, _7d, _30d via history merge)
- [x] F4. History cleanup (delete files older than max window)
- [x] F5. Retention detection (new IPs, removed IPs)
- [x] F6. Retention histogram computation
- [x] F7. Changeset tracking (IPs added/removed per update)
- [x] F8. Header generation matching exact format from section 7.1
- [x] F9. Atomic file writes (write to temp, then rename)
- [x] F10. Merge ipset processing (composite lists)
- [x] F11. `.source` file enable/disable mechanism
- [x] F12. Unit tests for history and retention logic

### G. Geolocation (`pkg/geoloc/`)
- [x] G1. GeoLite2 Country CSV parser (zip extraction, authenticated URL)
- [x] G2. GeoLite2 per-country and per-continent netset generation
- [x] G3. IPDeny country file parser
- [x] G4. IP2Location LITE DB1 parser
- [x] G5. IPIP country database parser
- [x] G6. DB-IP Lite Country CSV parser (gzipped, IP range -> CIDR conversion)
- [x] G7. DB-IP monthly URL pattern (`{YYYY-MM}`)
- [x] G8. Cross-reference comparison (ipsets vs country databases)
- [x] G9. Country JSON output format matching section 7.8
- [x] G10. Unit tests for each parser

### H. Output Generation (`pkg/output/`)
- [x] H1. Per-ipset JSON matching section 7.3 format exactly
- [x] H2. Markdown-to-HTML conversion in info field (`[text](url)` -> `<a>`)
- [x] H3. `all-ipsets.json` matching section 7.4 format
- [x] H4. `_history.csv` matching section 7.5 format (last 500 entries)
- [x] H5. `_retention.json` matching section 7.6 format
- [x] H6. `_comparison.json` matching section 7.7 format
- [x] H7. `_*_country.json` matching section 7.8 format
- [x] H8. `_changesets.csv` matching section 7.9 format
- [x] H9. `sitemap.xml` matching section 7.10 format
- [x] H10. `.setinfo` file matching section 7.2 format
- [x] H11. `.ipset`/`.netset` file matching section 7.1 format
- [x] H12. Git integration: add, commit, .gitignore, README.md generation
- [x] H13. `set_file_timestamps.sh` generation
- [x] H14. `dont_redistribute` handling in git and output files
- [x] H15. Timestamps in milliseconds in JSON
- [x] H16. Unit tests for all output formats

### I. CLI (`cmd/update-ipsets/`)
- [x] I1. `daemon` subcommand
- [x] I2. `run [ipset...]` subcommand
- [x] I3. `query <ip>` subcommand
- [x] I4. `iprange` subcommand (backwards-compatible flags)
- [x] I5. `enable <ipset...>` subcommand
- [x] I6. `version` subcommand
- [x] I7. `--silent`, `--verbose` flags
- [x] I8. `--config` flag
- [x] I9. `--enable-all` flag
- [x] I10. `--recheck`, `--rebuild`, `--reprocess`, `--cleanup` flags
- [x] I11. `--push-git` flag
- [x] I12. Lock file mechanism (flock, non-blocking)
- [x] I13. Cache load (read old bash `.cache` format) and save (JSON)
- [x] I14. User vs root mode detection (different BASE_DIR, disable kernel ipsets)
- [x] I15. Signal handling: SIGHUP=reload, SIGTERM=graceful shutdown
- [x] I16. Structured logging (levels: debug, info, warn, error)

### J. Scheduler (`pkg/scheduler/`)
- [x] J1. Priority queue ordered by next-due time
- [x] J2. Per-source update intervals from config
- [x] J3. Backoff on failures matching download manager logic
- [x] J4. Parallel downloads with sequential processing per ipset
- [x] J5. State persistence for crash recovery
- [x] J6. Immediate trigger support (for admin interface)
- [x] J7. Unit tests for scheduling logic

### K. Web Server (`pkg/web/`)
- [x] K1. Static file serving (embedded via `//go:embed`, override with `--web-dir`)
- [x] K2. JSON/CSV data served from memory (no disk writes for web data)
- [x] K3. Ipset file serving from BASE_DIR
- [x] K4. Dynamic sitemap.xml generation
- [x] K5. HTTPS support (Let's Encrypt or provided certs)
- [x] K6. Reverse proxy mode (X-Real-IP, CF-Connecting-IP)
- [x] K7. Gzip compression for text responses
- [x] K8. Cache headers (ETag, Last-Modified, Cache-Control)
- [x] K9. Rate limiting
- [x] K10. robots.txt
- [x] K11. Unit tests with httptest

### L. REST API (`pkg/web/`)
- [x] L1. `GET /api/v1/ipsets` — list all ipsets
- [x] L2. `GET /api/v1/ipsets/{name}` — detail
- [x] L3. `GET /api/v1/ipsets/{name}/data` — download
- [x] L4. `GET /api/v1/ipsets/{name}/history` — history CSV
- [x] L5. `GET /api/v1/ipsets/{name}/retention` — retention JSON
- [x] L6. `GET /api/v1/ipsets/{name}/countries/{provider}` — geo JSON
- [x] L7. `GET /api/v1/ipsets/{name}/comparison` — comparison JSON
- [x] L8. `GET /api/v1/search?ip={ip}` — IP lookup (binary search, O(log n))
- [x] L9. `GET /api/v1/search?ip={ip}&details=true` — extended lookup
- [x] L10. `GET /api/v1/compose?include=...&exclude=...&format=...` — dynamic composition
- [x] L11. `GET /api/v1/status` — scheduler status
- [x] L12. Backwards compatibility: all old URLs work (`/{name}.json`, `/files/*.ipset`, etc.)
- [x] L13. CORS headers
- [x] L14. Unit tests for all API endpoints

### M. Admin Interface (`pkg/web/`)
- [x] M1. Dashboard: all feeds status overview
- [x] M2. Schedule view: timeline/queue
- [x] M3. Feed detail page: history, logs, timing, errors
- [x] M4. Manual trigger: immediate update for a feed
- [x] M5. Enable/disable feeds
- [x] M6. System status: memory, uptime, goroutines, disk
- [x] M7. Authentication (basic auth or API key)
- [x] M8. Minimal JS (htmx or vanilla JS + Go templates)

### N. Kernel Ipset Management (`pkg/kernel/`)
- [x] N1. Netlink-based ipset operations (no external commands)
- [x] N2. Detect which ipsets are loaded in kernel
- [x] N3. Atomic swap: create tmp -> restore -> swap -> destroy old
- [x] N4. Only touch ipsets that were already loaded
- [x] N5. Graceful fallback when not root

### O. Cross-cutting Concerns
- [x] O1. All tests pass: `go test ./...`
- [x] O2. All tests pass with race detector: `go test -race ./...`
- [x] O3. No `go vet` warnings
- [x] O4. All benchmarks run: `go test -bench=. ./...`
- [x] O5. Benchmark results documented in a BENCHMARKS.md file
- [x] O6. No panics in library code (return errors instead)
- [x] O7. Context-aware cancellation throughout (context.Context)
- [x] O8. Graceful shutdown (finish current work, save state)
- [x] O9. Structured logging with levels
- [x] O10. No hardcoded paths — everything configurable
- [x] O11. Systemd integration (notify, watchdog)
- [x] O12. Binary builds for linux/amd64 (primary), linux/arm64 (secondary)

---

## COMPLETION CRITERIA

The implementation is complete ONLY when:

1. **Every item in the checklist above is checked off**
2. **`go test ./...` passes with zero failures**
3. **`go test -race ./...` passes with zero data races**
4. **`go vet ./...` reports zero issues**
5. **`go build ./cmd/update-ipsets/` produces a working binary**
6. **All benchmarks run and results are documented**
7. **The YAML configuration contains ALL ~166 ipset sources from the original bash script**
8. **The binary can run in batch mode (`run --enable-all`) and process ipsets correctly**
9. **The binary can run in daemon mode with scheduler and web server**
10. **The API serves correct JSON matching the format specifications**

**Do not stop until all of the above are satisfied.**
