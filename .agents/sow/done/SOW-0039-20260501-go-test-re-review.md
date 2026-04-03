# SOW-0039 | 2026-05-01 | go-test-re-review

## Status

completed

## Requirements

### Purpose

Second-round gap-analysis of Go tests after SOW-0032 implementation. Full
`go-behavioral-testing` rubric — not fix-verification only. Black-box framing
throughout.

LLMs stop reporting when they think the requirements are satisfied, not when
the task is exhausted. SOW-0032 closed quickly because the implementation pass
addressed the most visible offenders. Re-running the full rubric with the same
scope is the only way to surface the residue.

### User request quoted verbatim

> the SOWs have been implemented. Spawn agents to review the codebase and the
> tests again

### Assistant understanding

- This is a fresh full-scope review against the same `go-behavioral-testing`
  rubric used for SOW-0032, not a verification of SOW-0032 fixes only.
- This SOW started as an analysis-only re-review. The maintainer instruction
  is now to implement the justified Go-test hardening items directly instead
  of leaving them as untracked deferred work.
- Black-box framing is the bar: tests must verify the public contract —
  inputs, outputs, observable side effects, errors — not private state,
  helper signatures, or implementation details.
- Findings are evidence-first (file:line). Numbered. Bucketed A/B/C.
- Verification of every prior SOW-0032 finding is included as a compact
  table; new findings are listed independently.

### Acceptance criteria

- All `*_test.go` files under `cmd/`, `internal/`, `pkg/`, and `tools/` are
  surveyed under the full rubric.
- Every SOW-0032 finding (A1–A11, B1–B14, C1–C10) has a verdict:
  FIXED / PARTIALLY FIXED / NOT FIXED / REGRESSED, with file:line where
  residual instances exist.
- New A/B/C findings are independently produced; new test files added since
  2026-04-30 are inspected with the same lens.
- The SOW records maintainer decisions before implementation.
- Selected fixes are implemented with black-box behavioral tests and no broad
  test rewrites that obscure intent.
- Valid items not implemented here are either rejected with evidence or moved
  to a concrete pending SOW.
- Validation includes Go tests, strict test mode, blocking analysis gates,
  install, and live integrity smoke checks when runtime-serving behavior is
  touched.

## Analysis

### Methodology

Skills loaded:

- `go-behavioral-testing` — primary rubric.
- `project-testing` — repo conventions.
- `project-coding` — Go 1.26, package boundaries, "stay-standalone"
  `pkg/iprange`.
- `sow` — SOW format and numbering.

Read-only commands used:

- `find`, `wc`, `rg`, `grep`, `git log --since=2026-04-30 --stat`,
  `git diff --name-only`, hand-read of representative test files.
- Cross-reference of unexported helper calls vs unexported helper definitions.
- Greppable smells: `time.Sleep`, `synctest`, `os.Setenv`, `os.Unsetenv`,
  `reflect.`, `t.Parallel`, `t.Setenv`, `t.Cleanup`, `t.Context`, `t.Chdir`,
  `t.Helper`, `httptest.NewServer`, `httptest.NewRecorder`,
  `handler.ServeHTTP(rec`, `for [a-z]+ := 0; [a-z]+ < b\.N`, `for b.Loop()`,
  `stretchr/testify`, `gomock`, `goleak`, `Fuzz[A-Z]`,
  external `_test` packages.
- Git diffs since 2026-04-30 to identify NEW test files — fresh tests are the
  most likely place for new smells to live.

Test surface metrics (current snapshot, 2026-05-01):

- `*_test.go` files in main module: 130 (was 121).
- `*_test.go` files in nested modules: 2 (unchanged).
- New test files since 2026-04-30 (15):
  - `pkg/config/fuzz_test.go`, `pkg/config/runtime_controls_test.go`
  - `pkg/engine/concurrency_test.go`,
    `pkg/engine/output_cancellation_test.go`,
    `pkg/engine/output_metadata_test.go`,
    `pkg/engine/provider_defaults_test.go`,
    `pkg/engine/run_pipeline_test.go`
  - `pkg/iprange/set_properties_test.go`
  - `pkg/processor/fuzz_test.go`
  - `pkg/scheduler/policy_test.go`
  - `pkg/web/cache_test.go`, `pkg/web/direct_artifact_test.go`,
    `pkg/web/middleware_test.go`, `pkg/web/routes_test.go`
  - `tools/archposture/collect_test.go`
- **Package style: 100% same-package** — still zero `_test` external test
  packages anywhere (verified by reading first line of every `*_test.go`).
- Modern stdlib helpers:
  - `t.TempDir` heavily used.
  - `t.Helper` 108+ sites (good).
  - `t.Cleanup` 10 sites (still sparse vs ~90 `defer X.Close()` sites).
  - `t.Setenv` adopted in `pkg/systemd/notify_test.go`,
    `pkg/web/feature_test.go`, `pkg/web/routes_test.go`.
  - `t.Parallel` in **2 files** only:
    `pkg/engine/runtime_ledger_cache_test.go`, `pkg/engine/runtime_test.go`
    (1 file → 2 files; 6 → 8 calls).
  - `t.Context()` in **1 file** only: `pkg/engine/home_detail_test.go`
    (unchanged from SOW-0032).
  - `t.Chdir` 0 sites.
  - `synctest` 0 imports (Go 1.26 ships it stable).
  - `for b.Loop()` 28 sites; legacy `b.N` loop pattern: 0 sites.
- HTTP test style:
  - `httptest.NewRecorder` 124 call sites (was 120 — **increased**).
  - `httptest.NewServer` 54 call sites (unchanged).
  - `handler.ServeHTTP(rec, req)` direct invocation still dominant in
    `pkg/web`. New web test files added since SOW-0032 (`cache_test.go`,
    `direct_artifact_test.go`, `middleware_test.go`, `routes_test.go`) all
    use the recorder pattern.
- Banned dependencies: zero `testify`, `gomock`, `mockery`, `ginkgo`,
  `gomega`, `goleak`. Unchanged.
- Fuzz: 4 `f.Fuzz` targets — `pkg/iprange/fuzz_test.go` (2),
  `pkg/processor/fuzz_test.go` (1), `pkg/config/fuzz_test.go` (1). No
  `testdata/fuzz/` corpora committed.
- Property-based: `pkg/iprange/set_properties_test.go` uses stdlib
  `testing/quick` (no external dep). `pgregory.net/rapid` not added.
- Golden files: zero `*.golden` files; zero `update = flag.Bool` patterns.
- `&Engine{...}` literal construction in tests: **74 sites across 27 files**
  (was 73 / 15 files). The site count is roughly stable but the file count
  nearly doubled — the pattern has spread to more test files.
- `&Runner{...}` literal construction in `pkg/scheduler` tests:
  `pkg/scheduler/scheduler_test.go:555,1116`,
  `pkg/scheduler/policy_test.go:16,36`. Internal-state field access:
  `runner.download.waiting`, `runner.download.refetchPending`,
  `runner.processing.waiting`, `runner.processing.deferred`,
  `runner.processing.active`, `runner.processing.wake`, `runner.now =`,
  `runner.actionCh`. New file `pkg/scheduler/policy_test.go` is the worst
  offender.
- Direct unexported mutex/field access in `pkg/web/cache_test.go`:
  `cache.mu.Lock()`, `cache.files`, `cache.bytes`,
  `routes.cache.mu.Lock()`. New file from SOW-0036 implementation.
- Race / shuffle gates in CI: `make race` runs in CI, `make test-strict`
  exists in the Makefile but is **not** wired into `.github/workflows/ci.yml`
  (verified: only `make test`, `make ui-test`, `pnpm --dir ui lint`,
  `pnpm --dir ui build`, `make test-tools`, `make race`, `make lint`,
  `make cross` are invoked).
- Coverage gate unchanged: 50% global threshold.

### Verification of SOW-0032 findings

| ID  | Title (short)                                | Verdict        | Evidence (file:line)                                                                                                                                                                                                       |
| --- | -------------------------------------------- | -------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A1  | Every test file is in the producer's package | NOT FIXED      | 132/132 `*_test.go` files still in same package. Zero external `_test` packages exist (verified by reading first line of every file).                                                                                       |
| A2  | Tests construct `&Engine{...}` partially     | REGRESSED      | 74 sites / 27 files (was 73 / 15). New files compounding: `pkg/engine/output_metadata_test.go:11`, `pkg/engine/output_cancellation_test.go:23`, `pkg/engine/provider_defaults_test.go:21,48`, `pkg/engine/run_pipeline_test.go:94`, `pkg/engine/insights_series_test.go:37,67,102`. Pattern has spread; no shared exported constructor introduced. |
| A3  | Direct `handler.ServeHTTP(rec, req)`         | REGRESSED      | 124 `httptest.NewRecorder` sites (was 120). New web test files all adopt the recorder pattern: `pkg/web/cache_test.go:81-82`, `pkg/web/direct_artifact_test.go:22-23`, `pkg/web/middleware_test.go:42-43`, `pkg/web/routes_test.go:40-41`. No shared `httptest.NewServer` fixture introduced. |
| A4  | `time.Sleep` as synchronization              | FIXED          | `rg "time\.Sleep" --glob '*_test.go'` returns 0 results. `pkg/web/feature_test.go:1213` `waitForHTTPGet` polls with ticker; `pkg/web/integrity_test.go:187` `waitForEntityRebuildOutput` polls observable state; `pkg/scheduler/cancel_test.go` uses `done` channel + `time.After`. |
| A5  | No goroutine-leak guard                      | NOT FIXED      | Zero `goleak` imports (`rg "goleak"`). Long-lived units (`Runner`, `Run`-served HTTP) still rely on `done` channel + `time.After(5s)` patterns at `pkg/scheduler/cancel_test.go:81-86`, `pkg/web/feature_test.go:919-926` — these only prove the caller returned, not that internal goroutines exited. No live-goroutines status field added. |
| A6  | Log-string substring in `retention_test.go`  | FIXED          | `pkg/engine/retention_test.go:14-51` now asserts on the structured `retention.Current.Total` field; the `strings.Contains(logs.String(), ...)` check is gone. Logger is `slog.New(slog.DiscardHandler)`. |
| A7  | HTML substring `"Topology"` assertion        | FIXED          | `pkg/web/server_test.go:281-289` now decodes JSON and asserts `topology.Type == "Topology"` on the parsed structure, not the raw body bytes. |
| A8  | Raw `os.Setenv`/`os.Unsetenv` in systemd     | FIXED          | `pkg/systemd/notify_test.go:20-21` uses `t.Setenv`. `rg "os\.Setenv\|os\.Unsetenv" --glob '*_test.go'` returns 0 hits. |
| A9  | Weak-assertion smell                         | PARTIALLY FIXED | DroneBL parser: addressed (assertions on `data.Include.UniqueCount()`, output bodies, retained mtimes). Remaining: `pkg/config/runtime_controls_test.go:32-34` only checks `err == nil` without inspecting error category/wrapping. `pkg/web/feature_test.go` (~12 subtests) still has `if rec.Code != 200 || !strings.Contains(rec.Body.String(), ...)` patterns where the substring is the only contract assertion. No mutation-test pass was performed; this bucket can only be fully closed by mutation testing. |
| A10 | Same-package tests call unexported helpers   | NOT FIXED      | Sample call sites unchanged or expanded: `pkg/engine/output_test.go:20-490` calls `eng.buildSetMetadataWithEnableAll`, `eng.feedBodyPath`, `leafAncestors`, `eng.isRedistributable`, `eng.buildSetMetadataFromEffectiveEntryInDir`, `eng.writeComparisonFiles`, `buildPublicFeedSummary` — all unexported. New files compound the issue: `pkg/engine/output_metadata_test.go:27` calls `eng.buildSetMetadata`; `pkg/engine/output_cancellation_test.go:33` calls `eng.writeComparisonFiles`; `pkg/engine/run_pipeline_test.go:15,31,50,70` calls `eng.buildPipelineRunPlan`; `pkg/engine/insights_series_test.go:38,68,106` calls `eng.readInsightsSizeSeries`/`readInsightsChurnSeries`; `pkg/engine/concurrency_test.go:21` calls `runBoundedNameJobs`; `pkg/scheduler/policy_test.go:26-30,44,62-63,104,139` calls `runner.TriggerQueuedAction`, `runner.enqueueDownload`, `runner.finishDownload`, `runner.releaseDeferredDownload`, `runner.enqueueProviderDefaultsReprocess`, `runner.recoverStagedWork` — all unexported. |
| A11 | DroneBL test module not in `go test ./...`   | FIXED          | `Makefile:18-19` defines `test-tools`. CI step `make test-tools` at `.github/workflows/ci.yml:42-43`. `tools/dronebl2ipsets/parse_test.go` now has substantive table-driven assertions (entry counts, body equality, mtime preservation). |
| B1  | No `-shuffle=on` / `-count=N` race-strict gate | PARTIALLY FIXED | `Makefile:21-22` defines `test-strict` = `go test -shuffle=on -count=3 ./pkg/scheduler ./pkg/engine ./pkg/web`. Not invoked from `.github/workflows/ci.yml` (verified). The Makefile target exists but CI does not run it; flake/order protection lives only on a developer's local invocation. |
| B2  | No `synctest` coverage                       | NOT FIXED      | Zero `synctest` imports (`rg "synctest"` returns nothing). Scheduler timer tests still use wall-clock `time.NewTicker(10ms)` poll loops (`pkg/scheduler/cancel_test.go:170-188`, `pkg/web/feature_test.go:1213-1234`, `pkg/web/integrity_test.go:187-209`). |
| B3  | No fuzz coverage for config YAML parsing     | FIXED          | `pkg/config/fuzz_test.go:9` `FuzzLoadYAML` with 3 seeds. Note: only verifies "no panic" (return values discarded); contract verification beyond crash safety is absent — see new finding C-new-3. |
| B4  | No fuzz coverage for downloaded-feed parsers | FIXED          | `pkg/processor/fuzz_test.go:10` `FuzzRunDeterministicTextProcessors` with 4 seeds × 25 step combinations. Same caveat: only verifies "no panic"; output is discarded. |
| B5  | No property-based testing for `pkg/iprange`  | FIXED (alt)    | `pkg/iprange/set_properties_test.go:8-94` uses stdlib `testing/quick` for 6 properties (union idempotent, union/intersect commutative, exclude idempotent, exclude/intersect partition, pointwise membership). `pgregory.net/rapid` not adopted; SOW-0032 decision (2)(a) was implemented as (2)(c)-stdlib instead. |
| B6  | Raw-body routes through full HTTP server     | NOT FIXED      | `pkg/web/feature_test.go:259-470` (compose, raw, history, comparison routes) still goes through `httptest.NewRecorder`. The four raw-body routes named by `project-testing` are not covered by a shared `httptest.NewServer` fixture. New test files (`cache_test.go:81`, `direct_artifact_test.go:22`) reinforce the pattern. |
| B7  | `Options.WebDir` end-to-end serving test     | PARTIALLY FIXED | `pkg/web/feature_test.go:321-330,341-350,361-370,382-390,402-409` removes the served artifact and asserts 404. This is the missing-served-artifact test. Still missing: the inverse — a test that proves no live builder rebuilds the artifact when only `WebDir` content was removed. The current 404 assertions prove "no fallback creates the file"; they do not prove "no internal cache served stale bytes after the file was removed" — see new finding C-new-2. |
| B8  | No property check on integrity timestamp invariant | NOT FIXED | `pkg/engine/pipeline_integrity_scenario_test.go` is still scenario-enumerated. No final invariant pass ("for every artifact, mtime ≥ contributing input mtime") was added. |
| B9  | No goroutine-leak guard in scheduler/web tests | NOT FIXED    | Same as A5. Zero `goleak`. No status-field-based leak assertion introduced. |
| B10 | No `synctest` test for engine context propagation | NOT FIXED | `pkg/engine/output_cancellation_test.go:31-44` adds a cancellation test for `writeComparisonFiles`, but it pre-cancels the context — the test does not exercise cancel-mid-download, which was the B10 contract. `pkg/engine/concurrency_test.go:11-52` is similar (cancel before/during in-flight). Both use real time, not `synctest`. |
| B11 | `t.Parallel` essentially absent              | NOT FIXED / MAPPED | `t.Parallel` now in 2 files (`pkg/engine/runtime_test.go` joined `pkg/engine/runtime_ledger_cache_test.go`). 8 calls / 130 main-module test files. The structural problem remains and is mapped to SOW-0083 for classification or rejection. |
| B12 | No `t.Context()` adoption                    | NOT FIXED      | Still 1 site only: `pkg/engine/home_detail_test.go:448`. Every cancellation test in the new files uses `context.WithCancel(context.Background())` instead. |
| B13 | No golden-file pattern                       | NOT FIXED      | Zero `*.golden` files. Zero `update = flag.Bool`. Public artifacts `entities.json`, sitemap XML, robots.txt, llms.txt — still asserted via in-line literals or substring checks. |
| B14 | No bench-regression guard for engine hot paths | NOT FIXED   | Engine hot-path archposture guard (`tools/archposture/collect_test.go`) covers static shape only (no fresh-snapshot helper calls inside loops). No allocs/op or ns/op floor exists. |
| C1  | Benchmarks still use `b.N`                   | FIXED          | `rg "for [a-z]+ := 0; [a-z]+ < b\.N"` returns 0. All `iprange/bench_test.go` benchmarks now use `for b.Loop()`; setup explicitly outside the loop. |
| C2  | `reflect.DeepEqual` for `[]string`           | FIXED          | `rg "reflect\."` returns 0 across all `*_test.go`. `pkg/engine/insights_test.go` switched to `slices.Equal`. |
| C3  | `t.Cleanup` sparse vs `defer`                | NOT FIXED / MAPPED | 10 `t.Cleanup` sites; ~90 `defer X.Close()` sites. Status quo; mapped to SOW-0083 for classification or rejection. |
| C4  | `t.Helper` missing in many helpers           | NOT FIXED / MAPPED | Spot-check `pkg/engine/output_test.go` setup blocks still inline `os.WriteFile(...) // err check` rather than using a `writeFile(t, ...)` helper. Mapped to SOW-0083. |
| C5  | Test naming inconsistency                    | NOT FIXED / MAPPED | Some new tests describe behavior precisely (`TestRunBoundedNameJobsStopsSchedulingAfterContextCancel`); others describe state (`TestProviderDefaultsMarkerPathEmptyWithoutLibDir`). Mapped to SOW-0083. |
| C6  | Tests could be table-driven                  | PARTIALLY FIXED / MAPPED | `pkg/config/runtime_controls_test.go:5-37` is the only new table-driven file. Engine and web tests largely remain function-per-scenario. Mapped to SOW-0083. |
| C7  | Opt-in `TestExtractLegacyScriptCounts`       | NOT FIXED / MAPPED | `pkg/config/config_test.go` still has the same opt-in shape; counts only. Mapped to SOW-0083. |
| C8  | `TestLoadFireholCatalogCounts` describes metrics | NOT FIXED / MAPPED | `pkg/config/catalog_verify_test.go` was updated several times since SOW-0032 (rebalance, critical-infrastructure churn) — confirms structural debt is tax-paid. Mapped to SOW-0083. |
| C9  | UI tests not in CI                           | FIXED (out of scope) | `.github/workflows/ci.yml:32-37` runs `make ui-test`, `pnpm --dir ui lint`, `pnpm --dir ui build`. SOW-0034 scope. |
| C10 | `tools/archposture` minor weak-assertion shape | FIXED        | `tools/archposture/collect_test.go` was significantly expanded (114 lines added). Now asserts on multiple posture fields. |

**Verification table summary**:

- A: 11 items → 5 FIXED, 1 PARTIAL, 3 NOT FIXED, **2 REGRESSED** (A2, A3).
- B: 14 items → 3 FIXED, 2 PARTIAL, 9 NOT FIXED, 0 REGRESSED.
- C: 10 items → 4 FIXED, 1 PARTIAL, 5 NOT FIXED, 0 REGRESSED.
- **Regressions: A2, A3** — the `&Engine{}` literal pattern grew across more
  files; `httptest.NewRecorder` count rose from 120 to 124 with new web test
  files all using the recorder pattern.

### NEW Findings — Category A: Anti-patterns to eliminate

These are findings the SOW-0032 pass either did not surface, or that have
emerged from new tests added since 2026-04-30.

- **A-new-1. `pkg/web/cache_test.go` reaches into the `fileCache` private
  mutex and field map.** Evidence:
  - `pkg/web/cache_test.go:30,32,47,50,65,70` — `cache.mu.Lock() / Unlock()`
    around `len(cache.files)`, `cache.bytes`, `cache.files[a]`,
    `cache.files[b]` reads.
  - `pkg/web/cache_test.go:87,89` — `routes.cache.mu.Lock()` over
    `len(routes.cache.files)`.

  Why bad: rubric §4.3. The cache's contract is "what HTTP clients observe
  when this URL is requested" — body bytes, status, ETag, Last-Modified,
  cache-hit speed. Locking the cache's internal mutex and counting entries
  is exactly the implementation-coupled assertion the rubric forbids. A
  refactor that switches the LRU to a different data structure (e.g., a
  `container/list` linked-list with a separate index map) breaks every test
  here without changing the user-visible behavior.

  Fix sketch: assert on observable contract. For LRU correctness:
  send N+1 requests through the public route, modify the file content (with
  preserved mtime + size to defeat the staleness fast-path, as the test
  already does), request the first URL again, assert the body matches the
  new disk content. For oversize-bypass: same shape — write file > limit,
  assert response body matches updated disk content. The cache's internal
  state can be observed through admin metrics if needed (admin status
  exposes cache stats today; assert there).

  Effort: S–M.

  Risk if left: every cache refactor pays a tax in private-field churn;
  the test passes when the internal state matches the test's expectations
  even if the user-visible behavior is broken (the test is satisfied
  by `len(cache.files) == 0` even if a different code path returned stale
  bytes).

- **A-new-2. `pkg/scheduler/policy_test.go` and `pkg/scheduler/scheduler_test.go`
  partially construct `&Runner{}` literals and assert directly on
  unexported queue fields.** Evidence:
  - `pkg/scheduler/policy_test.go:16-24` — `&Runner{actionCh: make(chan ...),
    download: downloadLoopState{wake: make(chan struct{}, 1)},
    processing: processingLoopState{wake: make(chan struct{}, 1)}}` —
    a fake `Runner` built by hand from the producer's private types.
  - `pkg/scheduler/policy_test.go:36-42` — `&Runner{download:
    downloadLoopState{waiting: ..., active: ..., refetchPending: ...}}` with
    direct map mutation.
  - `pkg/scheduler/policy_test.go:51,54,56,65,68,106,108,113` — direct
    reads of `runner.download.waiting`, `runner.download.refetchPending`,
    `runner.processing.waiting`.
  - `pkg/scheduler/policy_test.go:137` — `runner.now = func() ...` — direct
    write to a private field.
  - `pkg/scheduler/scheduler_test.go:555,1116` — same `&Runner{...}`
    pattern.
  - `pkg/scheduler/scheduler_test.go:632-645` — `runner.processing.waiting["good"]
    = queuedWork{...}`, `runner.processing.deferred[...] = ...` — direct map
    writes.

  Why bad: same class as A2. The scheduler runtime ownership lives behind a
  documented public API (`New`, `Run`, `Trigger`, `Snapshot`,
  `TriggerQueuedAction`). The tests bypass the API and act on the queue
  state machine's private maps. Any refactor of the queue representation
  (e.g., the SOW-0030 split that already moved actions/automatic_due/
  download_loop/processing_loop/queue_admission/recovery into separate
  files) is paid for in test churn.

  Fix sketch: drive the runner through `New(...)`, then `runner.Trigger(...)`
  / `runner.TriggerQueuedAction(...)` / `runner.Reload(...)` calls. Assert
  on `runner.Snapshot()` (already exposed) for queue state. For deferred-
  refetch contract: `Snapshot` already includes the metadata the test wants;
  expose any missing fields as part of the public snapshot.

  Effort: M.

  Risk if left: SOW-0030 already split scheduler internals into multiple
  files; the next refactor (e.g., merging `download_loop` and
  `processing_loop` into a single fan-out) requires touching every literal
  `&Runner{}` site.

- **A-new-3. Substring assertions on JSON response bodies in
  `pkg/web/feature_test.go` are the dominant pattern, not the exception.**
  Evidence (sample, not exhaustive):
  - `pkg/web/feature_test.go:97-99` — `strings.Contains(string(body),
    "\"goroutines\"")`, `strings.Contains(string(body), "\"last_reason\":
    \"manual_run\"")`.
  - `pkg/web/feature_test.go:283` — `strings.Contains(string(body),
    "\"name\": \"sample\"")`.
  - `pkg/web/feature_test.go:318-319` — `!strings.Contains(rec.Body.String(),
    "\"name\": \"sample\"")`.
  - `pkg/web/feature_test.go:338-340` — `!strings.Contains(rec.Body.String(),
    "DateTime,Entries,UniqueIPs")`.
  - `pkg/web/feature_test.go:359-361` — `!strings.Contains(rec.Body.String(),
    "\"added\": 2")`.
  - `pkg/web/feature_test.go:379-381` — `!strings.Contains(rec.Body.String(),
    "\"ipset\":\"sample\"")`.
  - `pkg/web/feature_test.go:399-401` — `!strings.Contains(rec.Body.String(),
    "\"name\":\"other\"")`.
  - `pkg/web/feature_test.go:761-763` — `!strings.Contains(rec.Body.String(),
    "\"critical_ips\":1")`.
  - `pkg/web/feature_test.go:811-813` — `!strings.Contains(rec.Body.String(),
    "\"name\":\"critical_dns\"")`.

  Why bad: rubric §4.2 + §12.1 item 1. Substring matches on JSON are an
  A6/A7-class anti-pattern — the contract is that the response is JSON
  with certain fields and certain values. Substring matches succeed when:
  - the field is misnamed but the substring happens to be a value (e.g.,
    `"name": "sample"` would also match `{"some_other_name": "sample"}`).
  - whitespace changes (gofmt-style differences in the JSON encoder) break
    the test even when the contract is preserved.
  - the field is correct but the test passes a wrong sibling field
    (the substring lives anywhere in the body).

  The test rubric for this repo demands structured assertions: decode JSON
  into a typed struct (or `map[string]any`), assert on parsed fields.

  Fix sketch: a small helper `decodeJSON[T any](t *testing.T, body []byte) T`
  per file; replace each `strings.Contains` body assertion with a typed
  field check.

  Effort: M, mechanical.

  Risk if left: a refactor of any JSON encoder (e.g., switching from
  manual concatenation to `encoding/json` or vice versa) breaks every
  test in this file; tests catch fewer real regressions than they appear
  to.

- **A-new-4. New cancellation tests use real wall-clock time + `time.After`
  rather than `synctest`.** Evidence:
  - `pkg/engine/concurrency_test.go:31-35` — `select { case <-firstStarted:
    case <-time.After(2 * time.Second): t.Fatal("first job did not start") }`.
  - `pkg/engine/concurrency_test.go:40-47` — `select { case err := <-errCh:
    ... case <-time.After(2 * time.Second): t.Fatal(...) }`.
  - `pkg/engine/output_cancellation_test.go:31-44` — `context.WithCancel(
    context.Background())` + immediate `cancel()` — works without `synctest`,
    but couples the test to the same "polling/timeout" idiom that B2/B10
    flagged.

  Why bad: rubric §3.4 + §8. New tests added to verify SOW-0035 cancellation
  contracts replicate the same wall-clock idioms SOW-0032 flagged for the
  pre-existing scheduler tests. The contract under test ("on cancel, no new
  jobs are admitted") is exactly the case `synctest.Test` is designed for.

  Fix sketch: rewrite both new files with `synctest.Test`. Each test
  inside a synctest bubble: start the worker, advance one logical step,
  cancel, advance, assert. No real `time.After`.

  Effort: M (low blast radius; only 2 files).

  Risk if left: the same flake class on overloaded CI runners — a 2-second
  `time.After` is exactly the wait that loaded GitHub-runner queues miss.
  And every new cancellation test will mirror this shape.

- **A-new-5. `pkg/web/feature_test.go:97-101` asserts on substring of
  unfreezed admin status JSON without parsing.** Evidence:
  ```
  if !strings.Contains(string(body), `"goroutines"`) {
      t.Fatalf("expected goroutines in admin status body: %s", body)
  }
  if !strings.Contains(string(body), `"last_reason": "manual_run"`) {
  ```

  Why bad: A-new-3 case study — the field name `goroutines` is a key, the
  test would still pass if the value were `null`, an empty array, or
  the string `"goroutines"`. The test asserts on the presence of the JSON
  encoding of a key, not on the value.

  Fix sketch: decode into a typed admin-status struct, assert on
  `snap.Goroutines > 0` and `snap.LastReason == "manual_run"`.

  Effort: trivial S.

- **A-new-6. `pkg/web/cache_test.go:80-93` (the test that proves raw feed
  routes do not enter artifact cache) couples to `routes.cache.files`
  internal map.** Evidence: lines 87-92 — `routes.cache.mu.Lock();
  entries := len(routes.cache.files); routes.cache.mu.Unlock(); if entries
  != 0`.

  Why bad: same as A-new-1. The contract under test is "no raw-feed bytes
  enter the long-lived heap cache". Observable via: serve a large raw
  feed, then look at runtime stats (admin status exposes cache size in
  bytes). Or: make a second request for the raw feed and assert the
  service's RSS/heap-allocated cache footprint did not grow by file size.

  Fix sketch: expose cache occupancy via admin status (already partially
  done in `Options.AdminStatus`); assert through the API. Or: use a
  `pprof`-style heap snapshot in tests via `runtime.ReadMemStats`. The
  former is cleanest.

  Effort: S. Mapped to SOW-0083.

- **A-new-7. `pkg/scheduler/policy_test.go:73-116` asserts on internal
  `runner.processing.waiting["sample"]` — the test reads a `queuedWork`
  struct's `Reason`, `ForceRun`, and `Immediate` fields directly.**
  Evidence: lines 106-115 — `got, ok := runner.processing.waiting["sample"]
  ... if got.Reason != runreason.ReasonScheduledDue || !got.ForceRun ||
  !got.Immediate`.

  Why bad: same as A-new-2; called out separately because the test asserts
  on the *transition state*, not the *outcome*. The provider-default
  reprocess feature's outcome is "the source's next download/processing
  pass runs forced, immediate, with `ScheduledDue` reason". The way to
  observe that today is: call `runner.Snapshot().Items[i]` (the public
  type) and check its public `Reason`/`ForceRun`/`Immediate` fields.
  `Snapshot()` already exposes them on `ActiveQueueItem`/`PendingQueueItem`
  types.

  Fix sketch: drive `runner.enqueueProviderDefaultsReprocess(now)` then
  call `runner.Snapshot()` and assert on the snapshot's queue items.

  Effort: S. Mapped to SOW-0083.

### NEW Findings — Category B: Missing gaps to fill

- **B-new-1. New cancellation tests do not assert that goroutines actually
  exited.** Evidence:
  - `pkg/engine/concurrency_test.go:11-52` — asserts on `processed.Load() ==
    1` and `errors.Is(err, context.Canceled)`. Does not assert that worker
    goroutines spawned inside `runBoundedNameJobs` have returned.
  - `pkg/engine/output_cancellation_test.go:31-44` — `writeComparisonFiles`
    test asserts on directory contents; does not assert on internal
    bounded-runner goroutine exit.
  - `pkg/scheduler/cancel_test.go:81-86` — only asserts that the test's own
    `done` channel closes within 5 seconds. The runner's child fetch loop,
    processing loop, and recovery loop goroutines are not separately
    verified.

  Why this matters: SOW-0035 explicitly added a `WaitGroup` to ensure
  `Runner.Run` blocks until child workers exit; the test that proves
  this is missing. A regression that returns from `Run` without
  waiting for workers (the original SOW-0035 bug) is not caught by
  the current test. The contract is observable: `runtime.NumGoroutine()`
  before and after, or a status field on `Runner` that exposes
  `child-goroutines-alive`.

  Fix sketch: see Decision 5 (A5/B9 reopened).

- **B-new-2. No test verifies that `WebDir` is honored for top-level
  artifacts vs. embedded SPA assets.** Evidence: SOW-0036 audit listed
  `all-ipsets.json`, sitemap files, `robots.txt`, `llms.txt` as cache-
  eligible. The tests added in `pkg/web/cache_test.go` use a synthetic
  cache and arbitrary file names, not the configured top-level artifacts.
  A regression that resolves `robots.txt` from the embedded asset bundle
  instead of `WebDir/robots.txt` would not be caught.

  Fix sketch: add a test that writes a custom `robots.txt` to
  `eng.Runtime().WebDir`, then GETs `/robots.txt` and asserts the body
  matches what was written, not the embedded default.

  Effort: S.

- **B-new-3. Fuzz targets discard outputs and check only "no panic".**
  Evidence:
  - `pkg/iprange/fuzz_test.go:14-16` — `_, _ = ParseReader(...)`.
  - `pkg/iprange/fuzz_test.go:28-30` — `_, _ = ReadBinary(...)`.
  - `pkg/processor/fuzz_test.go:16-22` — `_, _ = Run(...)`.
  - `pkg/config/fuzz_test.go:25-34` — `_, _ = LoadYAML(path)`.

  Why this matters: rubric §9.1. A fuzz target is most valuable when it
  asserts a contract invariant on every fuzz input — round-trip
  stability, monotonicity of unique counts, non-overlap of returned
  ranges, parser idempotence. "No panic" is a baseline; the targets here
  do not exercise contract invariants.

  Fix sketch: for `FuzzParseReader`, on no-error inputs, assert
  `set.UniqueCount() <= len(data)`. For `FuzzReadBinary` write-then-read,
  assert the round-tripped set equals the original. For
  `FuzzRunDeterministicTextProcessors`, assert output length ≤ input
  length (most processors are filtering). For `FuzzLoadYAML` on
  no-error inputs, assert that re-marshaling the result is loadable.

  Effort: M.

- **B-new-4. No fuzz corpus is committed.** Evidence: `find . -type d
  -name fuzz -path '*/testdata/*'` returns 0 results.

  Why this matters: a fuzz crash that surfaces locally is not committed
  as a regression test. The next CI run can re-discover it.

  Fix sketch: run `go test -fuzz=... -fuzztime=60s` on each fuzz target;
  commit any seeds that surface. If none surface in 60s, this is fine —
  but the policy of committing seeds-on-failure should be documented in
  `project-testing/SKILL.md`.

  Effort: trivial S; ongoing reviewer responsibility.

- **B-new-5. `make test-strict` is in the Makefile but is not run by CI.**
  Evidence: `Makefile:21-22` defines the target. `.github/workflows/ci.yml`
  invokes `make build`, `make test`, `make ui-test`, `pnpm --dir ui lint`,
  `pnpm --dir ui build`, `make test-tools`, `make race`, `make lint`,
  `make cross`. No `make test-strict`.

  Why this matters: SOW-0032 fix for B1 was the Makefile target; the gate
  was never wired into CI, so flake/order protection lives on opt-in
  developer invocations only. A regression that introduces test-order
  dependency is not caught.

  Fix sketch: add a CI step `make test-strict`. Cost: ~3× the cost of
  `make test` for the three packages (scheduler, engine, web). On a
  PR-only trigger this is acceptable.

  Effort: trivial S. Mapped to SOW-0083.

- **B-new-6. No test verifies the `make test-strict` mode actually
  runs the `pkg/scheduler`, `pkg/engine`, `pkg/web` packages 3× with
  `-shuffle=on`.** This is a meta-finding — the strict gate exists in
  the Makefile but no test asserts on its target arguments. Low priority
  but symptomatic: the strict-test gate is a contract operators rely on
  ("CI exercises N package iterations with shuffled order"); changing
  its argument list silently weakens it.

  Effort: skip (over-engineered for the value).

- **B-new-7. No test covers the rate-limiter's per-IP isolation.**
  Evidence: `pkg/web/middleware_test.go:14-30` only tests one IP.
  `newClientRateLimiter(2, time.Minute)` — Allow("198.51.100.10") three
  times, then `time.Minute` later succeeds. Missing: two distinct IPs
  share a limiter — does each get its own bucket? The contract is
  per-IP rate limiting; a regression that collapsed buckets to a
  single global bucket would not be caught.

  Fix sketch: add a row that calls Allow with a second IP after the
  first is exhausted; assert it succeeds.

  Effort: trivial S.

- **B-new-8. No test exercises the fuzz seed corpus's panic-replay
  behavior.** Evidence: zero `testdata/fuzz/` directories. If a future
  developer commits a `testdata/fuzz/FuzzLoadYAML/<hash>` regression
  case, there is no test that asserts the fuzz target runs against
  it; only the developer's `go test -fuzz=...` invocation does, and
  CI does not run fuzz at all.

  Why this matters: rubric §9.1 — committed fuzz seeds become
  permanent table-driven regression tests via the runtime's seed
  ingestion.

  Fix sketch: add `go test -run=Fuzz -short ./pkg/iprange ./pkg/config
  ./pkg/processor` to CI. The `go test -run=Fuzz` form replays seeds
  without entering fuzz mode.

  Effort: trivial S.

### NEW Findings — Category C: Neutral improvements

- **C-new-1. Some new test files are still single-function-per-scenario
  rather than table-driven.** Evidence:
  - `pkg/engine/run_pipeline_test.go:10-84` — 4 separate `TestBuild...`
    functions for `buildPipelineRunPlan` paths (no-update, database-
    selection, critical-only, provider-defaults-drift). All four share
    the same setup helper but each is its own function.
  - `pkg/engine/provider_defaults_test.go:13-105` — 4 functions for the
    same `&Engine{cfg: cfg}` setup with minor cfg variants.

  Fix sketch: convert to table-driven subtests. Improves the
  failing-case experience and makes adding a 5th case a one-row
  addition.

  Effort: S.

- **C-new-2. `pkg/web/cache_test.go:24-28` documents that the
  same-mtime-same-size LRU eviction is verified by changing only the
  body, but no test verifies the *opposite* path — that bumping
  the mtime alone (without size change) invalidates the cache and
  serves the new body without LRU eviction.** This is a covering
  gap, not an anti-pattern. The cache contract has two dimensions
  (LRU + freshness); LRU is exercised, freshness alone is not.

  Effort: S.

- **C-new-3. Fuzz targets do not bound their input size at the right
  layer.** Evidence: `pkg/processor/fuzz_test.go:17-19` and
  `pkg/config/fuzz_test.go:26-28` use `t.Skip` for inputs >64 KiB
  inside the fuzz body. The Go fuzzer mutates from seeds that are
  already small; large inputs are rare and skipping them throws away
  fuzzer-generated coverage.

  Fix sketch: cap the input via the seed-corpus distribution
  (which the fuzzer respects), or use `t.Skip` only for clearly
  pathological inputs (e.g., > 1 MiB), not the common 64 KiB bound.

  Effort: trivial S.

- **C-new-4. `pkg/iprange/set_properties_test.go:96-115` reaches into
  `set.Ranges` private slice for `sameSet` comparison.** This is
  same-package-allowed, but the comparison is on the canonical-form
  representation, which is an internal detail. A refactor of the
  range storage (e.g., to a balanced tree) would break the test
  even if the public set semantics are preserved.

  Fix sketch: use `set.UniqueCount()` plus `set.Contains(i)` for a
  representative sample as the equality check. Or expose a public
  `EqualSet` method on `*IPSet` that compares by membership, which
  is the actual contract.

  Effort: S; clean dependency-free win. Mapped to SOW-0083.

- **C-new-5. `pkg/scheduler/cancel_test.go:170-188`'s `waitForSchedulerSnapshotItems`
  helper is a per-test variant of the polling pattern; every `pkg/web`
  and `pkg/engine` test that polls reproduces a copy of it.** Evidence:
  `pkg/web/feature_test.go:1213-1234` (`waitForHTTPGet`),
  `pkg/web/integrity_test.go:187-209` (`waitForEntityRebuildOutput`),
  `pkg/scheduler/cancel_test.go:170-188`. Same shape, no shared helper.

  Fix sketch: a single `internal/testutil` (or `pkg/testsupport`)
  package with `WaitFor(t, timeout, predicate func() (bool, error))`.
  Or — adopt `synctest`, in which case none of these helpers need
  to exist.

  Effort: S, mostly a Decision item. Mapped to SOW-0083.

### Notes / known limits

- **What I did not do**: I did not run mutation testing. The A9
  weak-assertion bucket can only be enumerated empirically by a mutation
  pass; the A9 verdict above is structural. Not running `go test` (read-
  only request) means I cannot confirm which tests would still pass with
  flipped operators.

- **What I did read**: I hand-read every new test file added since
  2026-04-30 (15 files), `pkg/engine/{output,retention,critical,
  provider_defaults,run_pipeline,concurrency,output_cancellation,
  output_metadata,insights_series,runtime_ledger_cache}_test.go`,
  `pkg/scheduler/{cancel,policy,scheduler}_test.go`, every new file in
  `pkg/web/` and the high-signal portions of `pkg/web/feature_test.go`
  and `pkg/web/integrity_test.go`. I did not hand-read every existing
  `pkg/iprange` or `pkg/processor` test (largely table-driven, no
  smell density observed in spot checks).

- **Items flagged as "needs verification"**:
  - **A9 catalog completeness**: my verdict is PARTIAL based on a sample.
    A full enumeration requires a mutation-test pass.
  - **B7 (`Options.WebDir` end-to-end)**: I confirmed the missing-artifact-
    returns-404 test exists. I did not confirm the inverse (no live
    builder rebuilds the artifact under load). The verdict is PARTIALLY
    FIXED on that basis.
  - **B-new-1 (goroutine exit assertions)**: I confirmed the cancellation
    tests do not call `runtime.NumGoroutine` or check a status field.
    Whether the tests *implicitly* prove goroutine exit by some other
    means depends on the public API — the public Runner does not
    expose a `WaitForExit` or similar. So the gap is real.

- **Cross-references**:
  - SOW-0035 — adds the bounded-runner fan-out and runner-wait pattern
    that A-new-4 / B-new-1 should test through `synctest`.
  - SOW-0036 — adds the cache bounds that A-new-1 / A-new-6 / B-new-2
    should test through observable contract (admin status / disk-served
    bytes).
  - SOW-0030 — its own ownership-boundary refactors keep colliding with
    the `&Runner{}` literals (A-new-2, A-new-7).

## Implications and decisions

User decisions required, in order of impact. Each option lists implications
and risks; recommendations follow.

**Decision 1: Address the regressed A2/A3 by introducing a shared engine
test fixture and a shared HTTP test server fixture?**

Background: SOW-0032 left A1/A2/A3/A10 as "deferred to SOW-0030 phases".
Since SOW-0030 closed without doing the migration, both patterns have
**spread to more files** (A2: 73 sites/15 files → 74 sites/27 files; A3:
120 → 124 recorder calls, with all 4 new web test files using the
recorder pattern). The deferral has a net-negative cost; the pattern is
now embedded in fresher, less-mature code.

- (a) **Open a focused implementation SOW that introduces
  `enginetesting.New(t, opts...)` (or `engine.NewForTesting`) AND a
  shared `httptest.NewServer` fixture for `pkg/web`, then migrates
  the 27 engine test files and 16 web test files in two waves.**
  Pros: addresses the root cause; eliminates the regression vector;
  pays the migration cost once. Cons: large diff; ~3-5 days of work;
  must not break existing tests during migration. Risk: a half-done
  migration is worse than the status quo.

- (b) **Open the SOW for `pkg/web` only (the `httptest.NewServer`
  migration), defer the `&Engine{}` literal cleanup to a future SOW
  bundled with a SOW-0030-style refactor.** Pros: smaller blast radius;
  closes the regressed A3 cleanly; A2 stays open as known debt.
  Cons: A2 keeps growing (it grew between SOW-0032 close and now).

- (c) **Accept both as permanent test-debt; add a CI lint that fails
  PRs introducing new `&Engine{}` or new `httptest.NewRecorder` sites.**
  Pros: stops the bleeding without doing the migration. Cons: requires
  a custom static-analysis check; existing 74+124 sites are grandfathered
  forever.

- (d) **Status quo.** Cons: every new feature SOW pays an A2/A3 tax;
  the patterns will keep spreading.

Recommendation: **(b) for A3 in this round, defer A2 to a follow-on SOW
that lands together with the next engine-internal refactor.** A2 is too
big to migrate in isolation without a clear refactor target; A3 has a
self-contained fix (a single `httptest.NewServer` fixture).

**Decision 2: Adopt `testing/synctest` for cancellation tests
(B2 + B10 + A-new-4)?**

Background: Go 1.26 toolchain is in `go.mod`; `testing/synctest` is stable.
The new cancellation tests added for SOW-0035 (`pkg/engine/concurrency_test.go`,
`pkg/engine/output_cancellation_test.go`) replicate the wall-clock
`time.After` idioms. Same pattern in scheduler and web tests.

- (a) **Pilot `synctest` in `pkg/scheduler/cancel_test.go` and the two
  new engine cancellation tests, then expand on demand.** Pros: removes
  flake class; small blast radius; modern Go idiom. Cons: `synctest`
  requires the test body to live entirely inside the bubble; some
  existing tests interact with `httptest.NewServer` (real time outside
  the bubble), which complicates retrofitting.

- (b) **Defer until `synctest` matures further.** Cons: every new
  cancellation test reproduces the wall-clock pattern; the deferral
  is itself a regression.

- (c) **Skip; rely on bounded `time.After` polls.** Cons: B2/B10
  remain open; A-new-4 will reproduce on the next cancellation test.

Recommendation: **(a)**. The pilot is small, the blast radius is
contained, and it sets the pattern before more cancellation tests
are written.

**Decision 3: Wire `make test-strict` into CI (B-new-5)?**

Background: SOW-0032's B1 fix added the Makefile target. CI does not
invoke it.

- (a) **Add a CI step `make test-strict` after `make race`.** Pros:
  closes the unfinished SOW-0032 work; ~3× cost over the three
  packages on PR triggers only. Cons: ~30-60s additional CI time per PR.

- (b) **Skip.** Cons: flake/order protection lives only on developer
  laptops.

Recommendation: **(a)**. The Makefile target already exists; this is
a one-line CI addition.

**Decision 4: Add structured assertions for JSON response bodies
(A-new-3 + A-new-5)?**

Background: ~15 substring assertions on JSON in `pkg/web/feature_test.go`
match field names as raw bytes. The contract is JSON shape + values.

- (a) **Introduce a small `decodeJSON[T any]` helper in `pkg/web`
  test support and migrate the substring assertions in
  `pkg/web/feature_test.go`.** Pros: closes A-new-3/A-new-5; resilient
  to whitespace/encoder changes; mechanical refactor. Cons: ~50-line
  diff per call site type; still same-package coupling (but the
  decoded struct is the contract).

- (b) **Defer; track the substring assertions as known weak-test debt.**
  Cons: A-new-3 will reproduce in every new web test.

Recommendation: **(a)** scoped to `pkg/web/feature_test.go` first;
expand to `pkg/web/home_entity_api_test.go`, `pkg/web/categories_api_test.go`
in a follow-up if reviewer time allows.

**Decision 5: Goroutine-leak assertions (A5/B9 reopened + B-new-1)?**

Background: SOW-0032 left A5/B9 as "no concrete reproduction; defer".
SOW-0035 added a `WaitGroup` to `Runner.Run` to prevent the cleanup-
race on shuffled tests. The contract (`Run` blocks until child goroutines
exit) now exists in code, but is not asserted.

- (a) **Adopt `go.uber.org/goleak` with a per-package `TestMain` guard
  in `pkg/scheduler` and `pkg/web`.** Pros: cheap, proven; covers every
  test in the package; complements SOW-0035. Cons: new dep (Apache-2.0,
  small).

- (b) **Expose a `RuntimeGoroutines()` field on `Runner.Snapshot()`
  (and equivalent on web `Run`); assert in tests.** Pros: zero new
  deps; visible in admin status. Cons: more work; harder to make
  universal across all tests.

- (c) **Skip; trust the SOW-0035 `WaitGroup` without test enforcement.**
  Cons: a regression that breaks the wait pattern is not caught.

Recommendation: **(a)** — reopen the deferred decision from SOW-0032.
The `WaitGroup` invariant from SOW-0035 deserves a test.

**Decision 6: Bound and verify the fuzz contract (B-new-3 + B-new-4 +
B-new-8)?**

Background: 4 fuzz targets exist, all "no panic" only. No corpora
committed. CI does not run fuzz seed replays.

- (a) **Tighten each fuzz body to assert one invariant on success
  (round-trip stability for `iprange` / `processor` round-trip;
  `LoadYAML(LoadYAML(x).Bytes()) == LoadYAML(x)` for `config`); add
  CI step `go test -run=Fuzz` to replay any committed corpora.**
  Pros: makes the fuzz targets actually find bugs; closes B-new-3,
  B-new-4, B-new-8 together. Cons: invariants need careful design
  per parser.

- (b) **Add `-run=Fuzz` corpus replay only; leave fuzz bodies as
  is.** Cons: B-new-3 stays open.

- (c) **Skip.** Cons: fuzz coverage is mostly cosmetic.

Recommendation: **(a)**. Fuzz targets are zero-value if they only
verify "no panic"; one invariant per target is the minimum
contribution.

### Maintainer decisions for implementation

- Decision 1: choose **(b)**. Implement the web-side shared server/behavioral
  fixture work in this SOW where it is tractable; create a concrete pending
  SOW for the larger engine `&Engine{}` literal migration instead of silently
  burying it.
- Decision 2: choose **(a)**. Pilot `testing/synctest` in the new engine
  cancellation tests after verifying the local Go 1.26 toolchain exposes
  `testing/synctest`.
- Decision 3: choose **(a)**. Add `make test-strict` to CI.
- Decision 4: choose **(a)**. Replace high-risk JSON substring assertions in
  `pkg/web/feature_test.go` with structured JSON decoding.
- Decision 5: choose **(b)** for now, not `goleak`. A new dependency is not
  justified before trying the repo's existing public snapshots and process
  lifecycle contracts. Any remaining leak-guard gap gets a concrete pending
  SOW.
- Decision 6: choose **(a)**. Add useful fuzz invariants and CI seed replay.
- Additional selected items: add the per-IP rate-limiter test and a
  `WebDir` top-level artifact serving test because both are small, behavioral,
  and directly close identified gaps.

## Plan

Implementation plan:

1. Add CI strict-test and fuzz-seed replay gates.
2. Replace selected weak web JSON assertions with decoded JSON assertions.
3. Add behavioral tests for `WebDir` top-level artifacts and per-IP rate
   limiter isolation.
4. Pilot `testing/synctest` in the engine cancellation tests where no network
   or external process interaction exists.
5. Strengthen fuzz targets with success-path invariants and replay-friendly
   seeds where feasible.
6. Create concrete pending SOWs for valid larger work not finished here.
7. Validate with Go tests, strict tests, fuzz seed replay, blocking analysis
   gates, install, and live integrity smoke checks.

## Execution log

- 2026-05-01: Loaded `go-behavioral-testing`, `project-testing`,
  `project-coding`, `sow` skills.
- 2026-05-01: Read SOW-0032, SOW-0035, SOW-0036 (and the original
  SOW-0032 pending version with the full A/B/C findings).
- 2026-05-01: Re-ran the full smell-grep pass over all
  `*_test.go` files; cross-referenced against the SOW-0032 baseline
  metrics.
- 2026-05-01: Identified the 15 new test files added since 2026-04-30
  via `git log --since=2026-04-30 --diff-filter=A --name-only -- "*_test.go"`.
- 2026-05-01: Hand-read every new test file plus high-density A/B
  candidates from the existing files.
- 2026-05-01: Verified each SOW-0032 finding against current
  state; recorded verdict and evidence file:line.
- 2026-05-01: Drafted SOW with verification table, 7 new A findings,
  8 new B findings, 5 new C findings, and 6 user decisions.
- 2026-05-01: Converted this SOW from analysis-only to implementation after
  maintainer direction that valid deferred items must become work, not
  graveyard notes.
- 2026-05-01: Added `make fuzz-replay` and wired CI to run both
  `make test-strict` and `make fuzz-replay`.
- 2026-05-01: Replaced selected raw JSON substring assertions in
  `pkg/web/feature_test.go` with decoded structured assertions; moved the
  shared JSON helper into `pkg/web/json_test_helpers_test.go` so the
  architecture posture gate stays meaningful.
- 2026-05-01: Reworked `pkg/web/cache_test.go` to assert observable cache
  behavior instead of private cache maps and counters.
- 2026-05-01: Added behavioral coverage for per-IP rate limiter isolation and
  top-level `WebDir` artifact serving.
- 2026-05-01: Piloted `testing/synctest` in engine cancellation/fan-out tests
  that do not require real network time.
- 2026-05-01: Strengthened fuzz tests with success-path invariants:
  `pkg/iprange` binary round trip, `pkg/config` save/reload round trip, and
  `pkg/processor` deterministic rerun.
- 2026-05-01: Static analysis found an unused production helper
  `pkg/web/surface_routes.go:newSurfaceRoutes`; removed it instead of
  suppressing the gate.
- 2026-05-01: Created pending follow-up SOWs for larger valid leftovers:
  SOW-0046 engine test fixtures, SOW-0047 web HTTP fixtures, and SOW-0048
  lifecycle leak guards.
- 2026-05-01: Updated `project-testing` with the new fuzz/CI gates,
  `testing/synctest` pattern, and helper-placement lesson.

## Validation

- Targeted checks:
  - `go test ./pkg/web` — passed.
  - `go test ./pkg/engine` — passed.
  - `make fuzz-replay` — passed.
- Full gates:
  - `make test` — passed.
  - `make test-tools` — passed.
  - `make test-strict` — passed.
  - `make lint` — passed.
  - `make staticcheck` — passed.
  - `make golangci-lint` — passed.
  - `make vulncheck` — passed.
  - `make race` — passed.
  - `git diff --check` — passed.
- Staticcheck/golangci initially failed on the unused
  `pkg/web/surface_routes.go:newSurfaceRoutes`; the helper was removed and
  both gates then passed.
- Product specs: not updated. Reason: this SOW changes test/CI quality gates
  and removes a dead unused helper; no runtime product contract changes.
- Project skills: `project-testing` updated with the new CI/fuzz/synctest
  lessons.
- Real-use validation:
  - `./install.sh` — passed; service restarted.
  - `systemctl is-active update-ipsets` — `active`.
  - `curl -fsS http://localhost:18888/healthz` — `ok`.
  - `curl -fsS http://localhost:18888/api/v1/status` — engine not running,
    source count 423, merge count 16.
  - `curl -fsS http://localhost:18888/api/v1/admin/integrity` — clean,
    count 0.
  - `curl -fsS http://localhost:18888/api/v1/admin/integrity/entities` —
    clean, count 0.
- Post-commit install repeat:
  - `./install.sh` — passed; service restarted with the committed tree.
  - Initial post-restart checks observed a normal background run window, then
    two feed-output integrity findings (`blocklist_net_ua`,
    `firehol_level4`) for stale secondary files.
  - `curl -fsS -X POST http://localhost:18888/api/v1/admin/integrity/reprocess`
    scheduled the documented repair path for both feeds.
  - Final `curl -fsS http://localhost:18888/api/v1/admin/integrity` —
    clean, count 0.
  - Final `curl -fsS http://localhost:18888/api/v1/admin/integrity/entities`
    — clean, count 0.

## Outcome

Completed.

Shipped changes:

- CI now runs strict shuffled Go tests and fuzz seed replay.
- Fuzz targets now assert useful invariants instead of only "does not panic".
- Engine cancellation tests use `testing/synctest` where practical.
- Web tests gained structured JSON assertions, observable cache behavior,
  per-IP rate limiter isolation, and top-level `WebDir` serving coverage.
- Dead unused web route construction helper removed.
- Larger valid leftovers are tracked by concrete pending SOWs.

## Lessons extracted

- Do not update an architecture posture baseline just to make a quality gate
  green. If a large file grows because helpers were placed mechanically, move
  the helpers to a focused test helper file.
- "Deferred" must have a path. SOW-0039 now leaves larger valid leftovers as
  explicit SOWs: SOW-0046, SOW-0047, SOW-0048, SOW-0081, and SOW-0083.
- Fuzz targets should prove at least one success-path invariant. Crash-only
  fuzzing is weak coverage for parsers and set algebra.

## SOW-0083 Residual Closure Addendum

SOW-0083 reviewed the remaining B11, C3-C8, and C-new-1 through C-new-5
residual hygiene items from this SOW and closed the loose mapping:

- B11: focused `t.Parallel()` opt-ins added only for isolated cache/property
  tests; broad sweep rejected as unsafe churn.
- C3: broad `defer` to `t.Cleanup` rewrite rejected; helper-owned resources
  already use `t.Cleanup` where ownership crosses a helper boundary.
- C4: missing `t.Helper()` wrappers fixed; static helper scan is clean.
- C5: broad test rename churn rejected; new/edited tests use behavior names.
- C6 and C-new-1: cohesive `buildPipelineRunPlan` scenarios converted to a
  table-driven test; unrelated function-per-contract tests kept.
- C7: opt-in legacy script count smoke kept as a manual compatibility aid;
  stronger legacy catalog name-diff coverage already exists.
- C8: catalog count test accepted as intentional catalog-shape drift detection.
- C-new-2: file-cache mtime-only freshness coverage added.
- C-new-3: already addressed by this SOW's fuzz strengthening.

## SOW-0081 Residual Closure Addendum

SOW-0081 reviewed and closed the remaining B8, B13, and B14 residual items
from this SOW:

- B8: `pkg/engine/pipeline_integrity_scenario_test.go` now performs a
  scenario-wide generated-artifact mtime invariant pass after each pipeline
  step. It checks every expected secondary artifact for processed public feeds
  against the feed `ProcessedDate`.
- B13: `pkg/engine/output_test.go` now has a golden-file/update pattern for
  stable public metadata artifacts, with `pkg/engine/testdata/robots.golden`
  and `pkg/engine/testdata/llms.golden`.
- B14: `pkg/engine/effective_entry_bench_test.go` now provides benchmark and
  allocation visibility for batch effective-entry resolver use. Fixed
  benchmark thresholds were rejected because they would be flaky across
  developer machines and CI hosts.
- C-new-4: IP set property equality now compares public membership semantics.
- C-new-5: scheduler polling-helper policy mapped to SOW-0066.

## Followup

- `.agents/sow/done/SOW-0046-20260501-engine-test-fixture-migration.md`
- `.agents/sow/done/SOW-0047-20260501-web-http-test-fixture-migration.md`
- `.agents/sow/done/SOW-0048-20260501-lifecycle-leak-guards.md`
- `.agents/sow/pending/SOW-0081-20260501-go-test-artifact-invariant-bench-coverage.md`
- `.agents/sow/done/SOW-0083-20260501-go-test-hygiene-residual-mapping.md`
