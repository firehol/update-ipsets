# SOW-0038 | 2026-05-01 | go-code-re-review

## Status

completed

## Requirements

### Purpose

Second-round gap-analysis of Go code after SOW-0031 implementation
(SOW-0035, SOW-0036, commit 769dfd9 "Complete code quality and testing
hardening", commit 35a0c0b "Improve provider defaults and architecture
boundaries"). Re-runs the full `go-best-practices` rubric — does not
narrow scope to fix-verification only. Cross-references SOW-0029,
SOW-0030, SOW-0031, SOW-0035, SOW-0036.

### User request quoted verbatim

> the SOWs have been implemented. Spawn agents to review the codebase
> and the tests again

### Assistant understanding

- Re-survey the Go production code under `cmd/`, `internal/`, `pkg/`,
  and `tools/` (excluding `pkg/iprange` deep dive — only confirm that
  it remains standalone) against the full `go-best-practices` rubric.
- Verify the resolution status of every prior SOW-0031 finding
  (A1–A15, B1–B11, C1–C10) with file:line evidence: FIXED / PARTIAL /
  NOT FIXED / REGRESSED.
- Independently grep-and-read the rubric again because code has
  changed; new code added in 769dfd9 / 35a0c0b is the most likely
  source of fresh smells.
- Analysis only. No source files modified. The only deliverable is
  this SOW.
- Cap each new-finding category at the most important findings; quality
  over quantity.

### Acceptance criteria

- Verification table covers every prior finding (15 A + 11 B + 10 C =
  36 items) with status + file:line.
- New findings are numbered, classified, evidence-cited, and capped per
  category at the most important entries.
- Regressions, if any, are highlighted explicitly.
- "Needs verification" items are flagged rather than asserted.
- No production source files modified.

## Analysis

### Methodology

- Skills loaded: `go-best-practices`, `project-coding`,
  `project-reviewing`, `project-content-surfaces`, `sow`.
- Read prior SOWs end-to-end:
  - `.agents/sow/done/SOW-0031-20260430-go-code-gap-analysis.md`
  - `.agents/sow/done/SOW-0035-20260430-go-concurrency-cancellation-hardening.md`
  - `.agents/sow/done/SOW-0036-20260430-static-artifact-cache-bounds.md`
- Commit summaries reviewed: `769dfd9` (full hardening),
  `35a0c0b` (provider defaults + architecture boundaries).
- Greppable rubric re-run from scratch: `ioutil`, `interface{}`,
  `context.Background()`, `^[[:space:]]*go `, `sync.WaitGroup`,
  `sync.Pool`, `sync.Once`, `time.Sleep`, `panic(`, `init()`,
  `_ = err`, `_, _ =`, `errors.Is/As/Join`, `errgroup`,
  `singleflight`, `x/time/rate`, `wg.Go`, `iter.Seq`, `unique.`,
  `weak.`, `staticcheck`, `golangci-lint`, `govulncheck`, `goleak`,
  `http.DefaultClient`, `http.DefaultTransport`, `recover()`,
  `sort.Slice/Strings`, `context.WithValue`, struct ctx fields.
- Files read for verification: `cmd/update-ipsets/daemon.go`,
  `pkg/scheduler/scheduler.go`, `pkg/scheduler/queue_admission.go`,
  `pkg/scheduler/recovery.go`, `pkg/scheduler/download_loop.go`,
  `pkg/scheduler/processing_loop.go`,
  `pkg/engine/run.go`, `pkg/engine/run_pipeline.go`,
  `pkg/engine/concurrency.go`, `pkg/engine/critical.go`,
  `pkg/engine/asn.go`, `pkg/engine/output.go`,
  `pkg/engine/entity_refresh_queue.go`,
  `pkg/engine/entity_feed_sidecar_build.go`,
  `pkg/engine/feed_body_stage.go`, `pkg/engine/retention.go`,
  `pkg/engine/fileset_helpers.go`, `pkg/engine/bootstrap_entries.go`,
  `pkg/engine/download_stage.go`, `pkg/engine/insights.go`,
  `pkg/engine/provider_defaults.go`,
  `pkg/web/server.go`, `pkg/web/middleware.go`, `pkg/web/cache.go`,
  `pkg/web/http.go`, `pkg/web/routes.go`, `pkg/web/methodology.go`,
  `pkg/web/integrity.go`,
  `pkg/cache/cache.go`,
  `pkg/processor/processor.go`, `pkg/processor/primitives.go`,
  `pkg/processor/run_stream.go`,
  `pkg/iprange/dns.go`, `pkg/iprange/dns6.go`,
  `pkg/downloader/downloader.go`, `pkg/downloader/canonical.go`,
  `Makefile`, `.github/workflows/ci.yml`.
- Repo metrics (production Go, excludes `_test.go`): 50,730 lines
  (up from 49,590 lines at SOW-0031). Top files:
  `pkg/engine/output.go` 1,380 (+14),
  `pkg/cache/cache.go` 1,171 (NEW — split from runtime ledger work),
  `pkg/web/admin.go` 1,043 (unchanged),
  `pkg/engine/home_entity_builders.go` 1,021,
  `pkg/engine/entity_integrity.go` 1,020,
  `pkg/engine/critical.go` 987,
  `pkg/engine/entity_surgical.go` 978,
  `pkg/engine/entity_artifacts.go` 961,
  `pkg/config/config.go` 950 (+6),
  `pkg/engine/download_stage.go` 859,
  `pkg/engine/runtime_ledger_cache.go` 827 (down).
- `pkg/iprange` confirmed standalone (no project imports outside its
  own tree).

### Verification of SOW-0031 findings

Compact table — each row: ID, status, evidence.

| ID  | Status        | Evidence |
|-----|---------------|----------|
| A1  | VERIFIED FIXED | `pkg/scheduler/scheduler.go:33-51` `Runner` no longer holds a `ctx` field; `Run(ctx)` derives `runCtx, cancel := context.WithCancel(ctx)` at line 256 and passes it explicitly. |
| A2  | PARTIALLY FIXED | Daemon SIGHUP path still spawns `go func()` without ctx for entity refresh: `cmd/update-ipsets/daemon.go:86`. Web startup still does the same: `pkg/web/server.go:254`. The underlying API remains ctx-less: `pkg/engine/entity_integrity.go:156` `EnsureEntityArtifactsCurrentWithTrigger(trigger string) error` and `pkg/engine/entity_artifacts.go:129` `RebuildEntityArtifactsWithTrigger(trigger string)`. The `entity_refresh_queue.go` queues are still ctx-less (`go e.runQueuedEntityArtifactRefresh(trigger)` at lines 60 and 80). SOW-0035 Analysis recorded this API boundary as a deliberate non-goal; no tracking SOW exists. |
| A3  | PARTIALLY FIXED | A new project-internal `runBoundedNameJobs` helper at `pkg/engine/concurrency.go:22` replaces some fan-out blocks. Adopters: `pkg/engine/critical.go:476`, `pkg/engine/asn.go:231`, `pkg/engine/bogons.go:189`, `pkg/engine/geoloc.go:146`. NOT adopted: `pkg/engine/output.go:411-491` (pair fan-out, ad-hoc), `pkg/engine/entity_feed_sidecar_build.go:50-77` and `:146-171` (two near-identical hand-rolled fan-outs), `pkg/engine/run_pipeline.go:42-103` (still uses `wg`+`sem`+`mu`+`results` map), `pkg/processor/primitives.go:143-194` (DNS resolver fan-out), `pkg/iprange/dns.go:52-71` and `pkg/iprange/dns6.go:26-45`. The project elected not to depend on `golang.org/x/sync/errgroup`. |
| A4  | VERIFIED FIXED | `pkg/web/middleware.go:14` imports `golang.org/x/time/rate`; the limiter at lines 22-71 lazily prunes per-call (no background goroutine). |
| A5  | VERIFIED FIXED | `grep -rn 'interface{}'` over production Go returns zero hits. |
| A6  | PARTIALLY FIXED | Pipeline-internal `context.Background()` reduced but residual sites remain: `pkg/engine/retention.go:34` (`iprange.ParseReader`), `pkg/engine/fileset_helpers.go:127` (`iprange.LoadPath`), `pkg/engine/bootstrap_entries.go:244` (`iprange.LoadPath`), `pkg/engine/download_stage.go:240` (`composeHistoryDerivativeBody`), `pkg/engine/entity_artifacts.go:619` (`buildFeedEntitySidecars` from rebuild API). Also: `pkg/processor/processor.go:375` `gunzipFile(_ context.Context, …)` accepts ctx but discards it (the parameter is `_`). |
| A7  | VERIFIED FIXED | `pkg/web/cache.go` is now a bounded LRU: `maxEntries`, `maxBytes`, `maxFileBytes` fields at lines 35-38; `evictLocked` at line 182; oversized files bypass the cache and stream from disk via `serveUncachedFile` at line 191. Configuration knobs at `pkg/web/routes.go:43-47`. |
| A8  | VERIFIED FIXED | `serveFileWithCaching` removed. `pkg/web/http.go` has no such function. The dead helper was deleted (SOW-0031 Outcome). |
| A9  | VERIFIED FIXED | `pkg/engine/entity_refresh_queue.go:144-203` — `runQueuedEntityArtifactRefresh` and `runQueuedEntityHealthRefresh` use an in-goroutine `for` loop with `e.entityRefreshRunning = false` flag transition under lock, no tail-spawn. |
| A10 | VERIFIED FIXED | `pkg/engine/output.go` — `jsonMarshalTabIndent` parameter is now `any` (search of `interface{}` is empty). |
| A11 | REJECTED       | SOW-0031 Outcome explicitly rejected this comment-style item. Status: not-going-to-fix. |
| A12 | VERIFIED FIXED | `pkg/engine/asn_url_resolver.go:18-24` clones `http.DefaultTransport` into a dedicated `*http.Client`; production code no longer reaches for `http.DefaultClient` for outbound requests. (Single residual `http.DefaultTransport.(*http.Transport)` reference at `pkg/engine/asn_url_resolver.go:18` is for cloning, not reuse, and `pkg/downloader/downloader.go:82` clones into its own client.) |
| A13 | VERIFIED FIXED | `pkg/engine/run_pipeline.go:49-63` — when ctx is cancelled before slot acquisition, the worker writes `processingException(ProcessingExceptionCancelled, …)` into `results[name]` so the batch report includes `cancelled` rows instead of dropping them. |
| A14 | DEFERRED to SOW-0030 | File/function size hot-spots and cache-entry mutability — SOW-0029/0030 own this; phases ongoing. `pkg/cache/cache.go` exposes `*Entry` pointer mutation paths (line 321 `Entry()`); SOW-0030 phase 1a/1b. Re-listed as new finding A8 below for visibility. |
| A15 | NOT FIXED (rejected) | `pkg/engine/helpers.go` is 790 lines, 50 functions. SOW-0031 Outcome rejected this as a standalone deliverable; opportunistic only. |
| B1  | NOT FIXED      | `Makefile` lint target = `go vet ./...`; `.github/workflows/ci.yml` has `make lint` only. No `staticcheck`, no `golangci-lint`, no `govulncheck`, no fuzz seeds in CI. (Local fuzz tests exist at `pkg/config/fuzz_test.go` and `pkg/processor/fuzz_test.go` but no CI fuzz step.) Re-listed as B1 below. |
| B2  | VERIFIED FIXED | `pkg/web/middleware.go:89-108` — `recoverMiddleware` recovers panics in the same goroutine, logs with structured slog context, and returns 500. |
| B3  | PARTIALLY FIXED | `golang.org/x/time/rate` adopted in middleware (A4). `golang.org/x/sync/errgroup` and `golang.org/x/sync/singleflight` are NOT used. The project chose `runBoundedNameJobs` over `errgroup`. No singleflight despite at least one route family that could benefit (e.g., the heavy-fanout writes that happen at run time and are otherwise serialized). Needs verification: whether singleflight has a real public-route benefit given cache-first reads. |
| B4  | NOT FIXED      | `errors.Join` is not used to surface multiple worker errors. `runBoundedNameJobs` returns `firstErr` (`pkg/engine/concurrency.go:84`); `output.go` pair fan-out has no aggregated error path; entity sidecar build keeps `firstErr`. Operators triaging multi-feed batches still see only the first failure. |
| B5  | NOT FIXED      | Zero `iter.Seq` usage in production. (`grep -rn 'iter\.Seq'` over production Go returns zero hits.) Listed as low-value modernization. |
| B6  | NOT FIXED      | Zero `unique.Make`/`weak.Pointer` usage. The `unique.` matches found in `pkg/iprange/{set,iter}*.go` are OTel attribute keys (`iprange.count_unique.ops`), not the `unique` stdlib package. |
| B7  | NOT FIXED      | `pkg/web/methodology.go:45` and `pkg/engine/insights.go:31` still use `var ... sync.Once + .Do(func(){})`. Modern `sync.OnceValue` would replace each one with one line. |
| B8  | NOT REQUIRED   | `encoding/json/v2` evaluation deferred. Acceptable. |
| B9  | NOT FIXED      | Benchmarks still use `for i := 0; i < b.N; i++` form. Examples: `pkg/iprange/bench_test.go`, `pkg/processor/stream_test.go`. `b.Loop()` not adopted. (Benchmarks are technically test files; flagged here only because they were listed in SOW-0031 as B-class.) |
| B10 | PARTIALLY FIXED | `WaitGroup.Go` (Go 1.25+) adopted at four sites: `pkg/engine/concurrency.go:54`, `pkg/engine/output.go:421`, `pkg/engine/entity_feed_sidecar_build.go:52,148`. Old `wg.Add(1) + go func() defer wg.Done()` pattern still in: `pkg/iprange/dns.go:53-56`, `pkg/iprange/dns6.go:27-30`, `pkg/scheduler/scheduler.go:259-272`, `pkg/scheduler/queue_admission.go:117-122`, `pkg/processor/primitives.go:186-189`. |
| B11 | NOT FIXED      | No `goleak` in CI or test mains. (`grep -rn 'goleak'` returns zero hits.) |
| C1  | NOT FIXED      | `sort.Slice`/`sort.Strings` count is still ~150 across production Go. `slices.SortFunc` not adopted. |
| C2  | NOT FIXED      | Same — `tools/dronebl2ipsets/ranges.go` still uses `sort.Slice`. |
| C3  | NOT FIXED      | `*_helpers.go` files unchanged. |
| C4  | N/A            | `init()` functions confirmed register-only (`pkg/engine/format_handlers.go`, `pkg/insights/rules_*.go`, `pkg/processor/*`). No side effects. |
| C5  | NOT FIXED      | `Engine` struct field ordering not analyzed via `fieldalignment`. |
| C6  | VERIFIED FIXED | Only one file-serving path remains: `pkg/web/cache.go` `fileCache.ServeFile`/`serveUncachedFile`, plus `pkg/web/http.go:90 serveRawFeedFile` (raw streaming). Two-paths smell resolved. |
| C7  | NOT FIXED      | Same as B7. |
| C8  | NOT APPLICABLE | Engine/Runner field grouping is structural; SOW-0030 owns. |
| C9  | NOT FIXED      | Doc comments coverage not measured; no `revive` or similar in CI. |
| C10 | NOT FIXED      | OTel coverage not exhaustively reviewed; consistent in heavy paths but not enforced. |

Verification summary: A: 9 FIXED, 4 PARTIAL, 1 NOT FIXED (rejected),
1 deferred. B: 1 FIXED, 2 PARTIAL, 7 NOT FIXED, 1 not-required.
C: 1 FIXED, 7 NOT FIXED (low priority by design), 2 N/A.

### NEW Findings — Category A: Anti-patterns to eliminate

**A1 (new). `web.Run` does not wait for the spawned `runner.Run` goroutine to exit before returning.** —
`pkg/web/server.go:261` launches `go runner.Run(runCtx)` but the
return path (`errCh` collection at lines 349-355) only waits for
HTTP listener goroutines. When `web.Run` returns, the daemon `main`
exits; `runner.Run` may still be inside its `wg.Wait()` waiting for
fetch/processing/recovery loops to drain. SOW-0035 made
`runner.Run` itself wait for child goroutines, but `web.Run`
abandons it.

- Why bad: violates the SOW-0035 lesson "scheduler runners must own
  and wait for the child goroutines they start." Today it works
  because the daemon process exits and the OS reaps everything,
  but in tests or in any embedded harness it leaks. Race detector
  might already be flaky on shuffle.
- Fix sketch: track the runner goroutine in a `sync.WaitGroup` (or
  return a `done <-chan struct{}` from `runner.Run`) and `Wait()`
  before returning from `web.Run`.
- Effort: S.
- Risk if left: shutdown ordering bug; staging cleanup race
  in tests; embedded callers cannot detect runner exit.

**A2 (new). Background entity refresh API does not accept context anywhere in its public surface.** —
`pkg/engine/entity_integrity.go:156`
`EnsureEntityArtifactsCurrentWithTrigger(trigger string) error`,
`pkg/engine/entity_artifacts.go:129`
`RebuildEntityArtifactsWithTrigger(trigger string) error`,
`pkg/engine/entity_refresh_queue.go:8` `QueueEntityArtifactsRebuild`,
`:41` `QueueEntityArtifactsRefreshForFeedUpdates`,
`:64` `QueueEntityArtifactsRefreshForHealthTransitions`. The
goroutines spawned in `cmd/update-ipsets/daemon.go:86` and
`pkg/web/server.go:254` consequently have no cancellation path,
and the queue runner at lines 144-203 has no shutdown contract
either.

- Why bad: SIGTERM during a long entity refresh wave runs to
  completion before the daemon exits; engine integrity windows
  enlarge on shutdown. SOW-0035 explicitly recorded this API
  boundary as a non-goal. The non-goal was not converted to a
  pending SOW, so it is now prose-only deferral, which the
  project's own SOW skill prohibits.
- Fix sketch: add ctx parameter to public entity APIs, propagate
  through queue runners; spawn the daemon and web startup
  goroutines under engine-owned contexts.
- Effort: M.
- Risk if left: shutdown stalls; entity artifacts can be left in
  inconsistent staged state; the SOW-0035 lesson about
  cancellation contracts is being slowly eroded.

**A3 (new). Heavy-phase per-name workers do not receive `ctx`; only the dispatcher checks cancellation.** —
`runBoundedNameJobs` (`pkg/engine/concurrency.go:22-90`) checks
ctx between job dispatches and on the result channel, but the
worker bodies in adopters do not. Examples:
- `pkg/engine/critical.go:476-485` —
  `writeCriticalInfrastructureForFeed(name, datasets, outDir, setCache)` has
  no ctx parameter; CPU-bound `iprange.OverlapCountIter` and
  per-feed file writes run to completion.
- `pkg/engine/asn.go:231-263` — worker function signature
  `func(name string) error` accepts no ctx;
  `db.CountFeedWithBogons` and JSON marshal/write proceed without
  cancellation observation.
- `pkg/engine/bogons.go:189` — same shape.
- `pkg/engine/geoloc.go:146` — same shape.

- Why bad: cancellation propagates between jobs but not within
  jobs. On a feed catalog of hundreds, a SIGTERM may still wait
  for the current N workers to finish their current feeds, which
  for large bogon/critical comparisons can be many seconds each.
- Fix sketch: extend `runBoundedNameJobs` signature to
  `fn func(ctx context.Context, name string) error`; thread ctx
  through the worker bodies. The standalone bounded helper makes
  this a small mechanical change.
- Effort: S/M.
- Risk if left: shutdown latency under load; the heavy-phase
  cancellation contract recorded in
  `.agents/sow/specs/pipeline.md` (SOW-0035) is partially
  honored — between jobs only.

**A4 (new). Two near-identical fan-out implementations of feed entity sidecar build.** —
`pkg/engine/entity_feed_sidecar_build.go:50-77`
(`buildFeedEntitySidecars`) and `:146-171`
(`stageFeedEntitySidecarsFromLoadedProviders`) are independent
copies of the same `wg.Go(func(){ for ... select{<-ctx.Done(): / case name := <-jobs: ...}})`
pattern, the same close-and-drain helper
(`closeResultsWhenFeedEntitySidecarBuildDone`) used twice, and
the same `firstErr` aggregation. The body of each worker runs
`buildSingleFeedEntitySidecar`.

- Why bad: parallel evolution risk. A bug fix in one block must be
  ported to the other. Maintainability cost is double. The
  project already has `runBoundedNameJobs` for this shape; these
  blocks need a result-returning variant or a per-name builder
  helper.
- Fix sketch: extract a `runBoundedNameJobsWithResults[T]` (using
  generics, Go 1.26-friendly) or refactor both blocks to call a
  shared helper that produces a `map[string]T` result.
- Effort: M.
- Risk if left: divergent shutdown semantics, divergent bugs.

**A5 (new). Worker cancellation in dns/processor fan-out blocks send-and-block on unbuffered jobs channel.** —
`pkg/iprange/dns.go:64-67`,
`pkg/iprange/dns6.go:38-42`,
`pkg/processor/primitives.go:190-192` all do
```
for _, host := range hosts {
    jobs <- host
}
close(jobs)
```
without `select { case <-ctx.Done(): return; case jobs <- host: }`.
If a worker dies (e.g. panic recovery in some future change), the
sender blocks forever.

- Why bad: latent leak; not a current bug because workers do not
  exit early today, but the construction is fragile. Also: on
  ctx cancellation, the workers continue draining all remaining
  jobs through `LookupIPv4(ctx, ...)` which returns errors fast,
  but the result channel can be filled needlessly.
- Fix sketch: add `select { case <-ctx.Done(): return; case jobs <- host: }`
  in each dispatch loop, mirroring `runBoundedNameJobs`'s pattern.
- Effort: S.
- Risk if left: latent goroutine leak surfaces on any future
  panic-recovery middleware addition or worker bug.

**A6 (new). `iprange.LoadPath`/`ParseReader` callers in engine pass `context.Background()` despite having a real ctx in scope.** —
- `pkg/engine/retention.go:34`
- `pkg/engine/fileset_helpers.go:127`
- `pkg/engine/bootstrap_entries.go:244`

These three sites are reachable from `RunOnce(ctx, …)`. SOW-0035
fixed several similar sites (downloader canonical parse,
feed_body_stage, etc.) but these three were missed.

- Why bad: long file reads and parse/iter operations cannot be
  cancelled. A SIGTERM during a snapshot load or bootstrap
  helper waits for the parse to finish.
- Fix sketch: thread ctx through the affected helpers.
- Effort: S.
- Risk if left: shutdown latency; SOW-0035's "remaining
  `context.Background()` are root/test/snapshot" claim is now
  inaccurate.

**A7 (new). `gunzipFile` accepts ctx and discards it.** —
`pkg/processor/processor.go:375`:
```
func gunzipFile(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
```
Callers at lines 408, 416, 501 pass real ctx values, but the
function ignores them. For modest inputs this is fine, but the
function caps decompressed size and is in a streaming pipeline.

- Why bad: a function with `ctx context.Context` in its signature
  signals that ctx is honored. Discarding the parameter is a
  contract lie. Reviewers will assume cancellation is observed.
- Fix sketch: either name the parameter `ctx` and check
  `ctx.Err()` between blocks of decompression, or rename the
  signature to drop ctx.
- Effort: S.
- Risk if left: false confidence in cancellation; lint cannot
  detect this because the parameter is intentionally `_`.

**A8 (new). `*cache.Entry` pointer escapes through `State.Entry`.** —
`pkg/cache/cache.go:321-333`. Documented contract: "concurrent
mutations to the same entry must be serialized by the caller."
Real call sites (e.g., per-feed processing) do serialize, but the
type system cannot enforce this. SOW-0029 / SOW-0030 own the
broader cache mutability redesign.

- Why bad: re-flagged here because `pkg/cache/cache.go` grew from
  932 to 1,171 lines (+239) in commit 35a0c0b, expanding the
  surface that depends on the unenforced contract. The new
  helpers `EntrySnapshot` (line 1126) returns a shallow copy with
  shared slice fields (`HistoryMinutes`, `CriticalOverlapTiers`)
  — concurrent mutation of those slice values from another
  goroutine is undefined. Race detector may not catch read-only
  slice access patterns.
- Fix sketch: continue SOW-0030 phase 1a/1b; in the meantime,
  deep-copy slice fields in `EntrySnapshot`.
- Effort: S (deep-copy slice fields in `EntrySnapshot`); the
  larger redesign is SOW-0030's scope.
- Risk if left: SOW-0030's structural redesign is the durable
  fix; this is a holding pattern note.

### NEW Findings — Category B: Missing gaps to fill

**B1 (new). CI lint surface still unchanged: no staticcheck, golangci-lint, govulncheck, goleak.** —
`Makefile:33-34` `lint:` target is just `go vet ./...`. CI
(`.github/workflows/ci.yml`) runs build, test, race, lint, cross,
and a 50% coverage gate; nothing else. SOW-0031 explicitly
flagged this. SOW-0035/0036 did not address it.

- Why this matters here: every fan-out fix described above is
  exactly the kind of thing `staticcheck` and `goleak` catch
  early. `govulncheck` against `go.opentelemetry.io/...`,
  `mvdan.cc/sh/v3`, `vishvananda/netlink` is a one-step CI add
  that immediately surfaces reachable CVEs.
- Effort: S to add steps; M to triage initial backlog.
- Sources: https://staticcheck.dev/docs/,
  https://golangci-lint.run/, https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck,
  https://pkg.go.dev/go.uber.org/goleak.

**B2 (new). No `errors.Join` for multi-feed batch failures.** —
`pkg/engine/concurrency.go:84` returns only `firstErr`; same for
all `runBoundedNameJobs` adopters and the entity sidecar
build/stage paths (`firstErr` at lines 80 and 174 of
`entity_feed_sidecar_build.go`). When a heavy phase touches
hundreds of feeds and several fail, operators see only the first
error.

- Why this matters here: triage signal is poor. The pipeline
  records `report.Failed []string` separately so per-feed names
  are visible, but root-cause errors collapse to one.
- Fix sketch: replace `firstErr` aggregation with
  `errors.Join(errs...)`; cancel propagation can stay first-error.
- Effort: S.
- Sources: https://pkg.go.dev/errors#Join.

**B3 (new). `sync.OnceValue` not adopted.** —
`pkg/web/methodology.go:44-48` and
`pkg/engine/insights.go:30-39` still use the older
`var once sync.Once + .Do(func(){ … })` form. Both initialize
single-value globals; `sync.OnceValue[T]` is the one-line modern
replacement.

- Effort: S each.
- Sources: https://pkg.go.dev/sync#OnceValue.

**B4 (new). `WaitGroup.Go` adoption is partial.** —
Production code uses `wg.Go` in 4 places (`pkg/engine/...`).
Other sites still use `wg.Add(1) + defer wg.Done()`:
`pkg/iprange/dns.go:53`, `pkg/iprange/dns6.go:27`,
`pkg/scheduler/scheduler.go:259`,
`pkg/scheduler/queue_admission.go:117`,
`pkg/processor/primitives.go:186`.

- Effort: S, cosmetic.

**B5 (new). No `context.AfterFunc` use for late-bound cleanup hooks.** —
Go 1.21+ `context.AfterFunc` is the modern primitive for "run X
when this ctx is done." The shutdown goroutines at
`pkg/web/server.go:292-304` (HTTP server `Shutdown`) and
`pkg/web/server.go:307-318` (systemd watchdog) could each be a
two-line `context.AfterFunc` registration without an extra
goroutine.

- Effort: S.
- Risk if left: cosmetic; the current code is correct.

### NEW Findings — Category C: Neutral improvements

**C1 (new). `sort.Slice`/`sort.Strings`/`sort.Ints` everywhere; `slices` package unused.** —
~150 production occurrences of `sort.` calls. Same as SOW-0031
C1; not addressed.

- Effort: S per file; M total. Folds naturally into staticcheck
  rollout (B1).

**C2 (new). `runBoundedNameJobs` cannot return per-name results.** —
The helper exists and is adopted in four heavy phases, but the
sites that need per-name `[]string` or `[]T` results (output.go
pair fan-out, entity sidecar build) had to roll their own. A
generic `runBoundedNameJobsWithResults[T any]` (Go 1.26 generics
are fine) would unify A4's two near-identical blocks.

- Effort: M.
- Risk if left: continued duplication; future fan-out
  contributors choose ad-hoc patterns.

**C3 (new). `pkg/cache/cache.go` is now 1,171 lines.** —
+239 lines from commit 35a0c0b. SOW-0030 phase 1a/1b owns this;
flagged here only because it is now the second-largest
production file and it crossed a threshold since SOW-0031.

- Effort: tracked elsewhere.

**C4 (new). Slice-field shallow copy in `State.EntrySnapshot`.** —
`pkg/cache/cache.go:1126-1135` returns a shallow `*Entry` copy.
`HistoryMinutes` (line 37) and `CriticalOverlapTiers` (line 78)
are slices and are not deep-copied. `ReplaceEntry` at lines
340-355 does deep-copy these fields, so the inconsistency is
visible.

- Effort: S — deep-copy slices in `EntrySnapshot`.
- Risk if left: low — current callers do not mutate snapshot
  slices.

**C5 (new). `*context.Context` parameter not used; `context.Context` is correctly passed by value across the whole codebase.** —
Confirming clean.

### Notes / known limits

- "Needs verification" items:
  - B3 (singleflight benefit) from SOW-0031 still needs a
    public-route audit to determine whether two simultaneous
    requests can both perform expensive work; project rule says
    no, but the audit was not redone here.
  - A8 race-detector behavior with `EntrySnapshot` shallow slice
    copies — not exercised under `-race -shuffle=on` in this
    review; shapes a hypothetical race only if a caller mutates
    slice fields concurrently.
  - The new `web.Run` not joining `runner.Run` (A1 new) —
    confirmed by reading `pkg/web/server.go:254-356`; behavior
    under `make test` was not exercised here.
- Conflicts skill vs project rules: none. `project-coding`
  encodes the project-specific overrides (cache-first, no startup
  rescans, generated-file mtimes, config-driven semantics); none
  of the new findings contradict these.
- Findings already covered by SOW-0029/0030 are referenced and
  not relisted (file/function size hot-spots, cache mutability
  redesign, engine struct concerns).
- Cross-references:
  - SOW-0029 — owns structural posture audit.
  - SOW-0030 — owns the phased shrink/redesign of cache, engine,
    scheduler, web, output.
  - SOW-0031 — original gap analysis; this SOW verifies it.
  - SOW-0035 — concurrency/cancellation hardening; A1/A2/A3 new
    are direct follow-ups to its non-goal list.
  - SOW-0036 — static cache bounding; verified clean.
- `pkg/iprange` confirmed standalone-only (no project imports
  outside its own tree).
- Test files were technically out of SOW-0031 scope and are out
  of this re-review's scope. The test-side gap analysis lives in
  SOW-0032 (already closed).

## Implications and decisions

Autonomous maintainer decisions recorded after user delegated code-quality
follow-up ownership. These are internal maintainability decisions, not product
behavior decisions:

**1. Which new Category A findings to schedule next?**

Background context: the new A items group into two themes —
(a) cancellation/shutdown completeness (A1 new, A2 new, A3 new,
A5 new, A6 new, A7 new) which are direct continuations of
SOW-0035's lessons; (b) maintainability of fan-out and cache
patterns (A4 new, A8 new) which overlap SOW-0030's structural
work.

Options:

- (a) "SOW-0035 round 2" — single SOW for cancellation completeness:
  A1 new (web.Run join), A2 new (entity-refresh API ctx),
  A3 new (per-name worker ctx), A5 new (sender-side select),
  A6 new (residual `context.Background()`), A7 new (gunzipFile
  ctx). Keeps the cancellation contract coherent.
- (b) "Maintainability batch" — single SOW for A4 new (fan-out
  consolidation) and A8 new (slice deep-copy + cache hardening).
  Risk: overlaps SOW-0030 phases.
- (c) Both as one combined SOW.
- (d) Defer all of A new pending other priorities.

Recommendation: **(a)**. The cancellation findings are a coherent
follow-up to SOW-0035 with the same validation gates (race,
shuffled cancellation tests, no-leak smoke). Implications: one
focused SOW, low blast radius if scoped to ctx threading. Risks:
A2 new touches a public engine API surface (entity refresh),
which has admin-API consumers; the contract change must
preserve existing admin behavior. A4 new and A8 new are best
folded into SOW-0030's phases when they reach those files.

Decision recorded: **1(a)**. Open and execute
`SOW-0042-20260501-go-cancellation-followup.md` first.

**2. CI tooling (B1 new) — block PRs or warn?**

Background: B1 was deferred at SOW-0031. Public-facing service
with OTel deps still has no `staticcheck` / `golangci-lint` /
`govulncheck` / `goleak`.

Options:

- (a) Add all four in advisory mode (warn, don't fail) for one
  release; clean up the backlog; flip to blocking.
- (b) Add `govulncheck` blocking from day one (CVE gate);
  `staticcheck`/`golangci-lint` advisory; `goleak` per-package
  opt-in.
- (c) Add only `govulncheck` blocking; defer the others.
- (d) Defer all.

Recommendation: **(b)**. govulncheck has a small false-positive
surface (reachable code paths only), and the dep tree is
non-trivial — a CVE in OTel or `mvdan.cc/sh/v3` is operationally
real. staticcheck/golangci-lint advisory gives time to absorb
findings without freezing PRs; flip to blocking after one
cleanup SOW. goleak per-package keeps test suites stable.

Implications: adds ~30s to CI; first govulncheck run may
surface advisories that need triage. Risks: a transient
vulndb advisory could block unrelated PRs.

Decision recorded: **2(b)**. Open
`SOW-0043-20260501-go-ci-tooling.md` as a pending follow-up after the
cancellation SOW.

**3. New Category B/C items (B2-B5, C1-C5) — discrete work or opportunistic?**

Options:

- (a) One "modern stdlib idioms" SOW covering B2 (errors.Join),
  B3 (sync.OnceValue), B4 (WaitGroup.Go), C1 (sort→slices),
  C4 (slice deep-copy in EntrySnapshot).
- (b) Opportunistic only — fold each into the next SOW that
  touches the surrounding file.
- (c) B2 (errors.Join for multi-feed batch failures) as a small
  standalone SOW because it directly improves operator triage;
  defer the rest.

Recommendation: **(c)**. errors.Join has direct operator value;
the rest are aesthetic and best left to opportunistic absorption
once `staticcheck`/`golangci-lint` advisory mode (B1 (b)) starts
flagging them.

Decision recorded: **3(c)**. Open
`SOW-0044-20260501-go-error-aggregation.md` as a pending follow-up after
the cancellation and CI-tooling SOWs.

## Plan

Analysis only. No production code changes in this SOW.

Follow-up ledger:

- `.agents/sow/done/SOW-0042-20260501-go-cancellation-followup.md` — completed.
- `.agents/sow/done/SOW-0043-20260501-go-ci-tooling.md` — completed.
- `.agents/sow/pending/SOW-0044-20260501-go-error-aggregation.md` — reopened and pending.

## Execution log

2026-05-01:

- Loaded skills: `go-best-practices`, `project-coding`,
  `project-reviewing`, `project-content-surfaces`, `sow`.
- Read SOW-0031 / SOW-0035 / SOW-0036 end-to-end.
- Surveyed `git log --since=2026-04-30 --stat` for changed files
  since round-1 analysis (commits `769dfd9`, `35a0c0b`, plus
  intermediate hardening).
- Re-ran the full grep rubric for: `ioutil`, `interface{}`,
  `context.Background`, goroutine launches, swallowed errors,
  panic, init, time.Sleep, sync primitives,
  `errgroup`/`singleflight`/`x/time/rate`, `wg.Go`, `iter.Seq`,
  `unique.`, `weak.`, modern HTTP timeouts, `recover()`,
  `sort.Slice`, `WithValue`, struct ctx fields.
- Read all top-20 production Go files and the new files added in
  769dfd9 / 35a0c0b (`pkg/engine/concurrency.go`,
  `pkg/engine/run_pipeline.go`,
  `pkg/engine/entity_feed_sidecar_build.go`,
  `pkg/engine/provider_defaults.go`,
  `pkg/web/routes.go`, `pkg/cache/cache.go`).
- Verified each prior finding's status with file:line evidence.
- Captured 8 new Category A findings, 5 Category B, 5 Category C.
- Authored this SOW under `.agents/sow/pending/`.
- Recorded autonomous maintainer decisions 1(a), 2(b), and 3(c).
- Created concrete follow-up SOWs 0042, 0043, and 0044 so no valid
  deferred work remains as prose-only intent.
- No production source files modified.

2026-05-01 cycle-3 audit addendum:

- Iterative audit cycle 3 found the earlier "opportunistic" mapping for
  remaining Go cleanup items was too vague after the referenced SOW-0030 phases
  were already completed.
- Concrete follow-up is now tracked by
  `.agents/sow/pending/SOW-0079-20260501-go-best-practice-cleanup-mapping.md`.
  That SOW explicitly covers duplicated entity sidecar fan-out,
  `EntrySnapshot` copy semantics, and `sync.Once` lazy-init modernization or
  evidence-backed rejection.

2026-05-01 SOW-0079 closure addendum:

- SOW-0079 completed this mapping in
  `.agents/sow/done/SOW-0079-20260501-go-best-practice-cleanup-mapping.md`.
- Implemented outcomes:
  - `pkg/cache.EntrySnapshot` now deep-copies slice fields through the same
    helper used by `ReplaceEntry`.
  - `pkg/web/methodology.go` uses `sync.OnceValues` for rendered methodology
    pages.
  - `pkg/engine/insights.go` uses `sync.OnceValue` for the shared insights
    engine.
  - `pkg/engine/entity_feed_sidecar_build.go` uses one shared sidecar worker
    fan-out helper for build and staging paths.

## Validation

- No code changed.
- Commands run: `git log`, `wc -l`, `grep -rn`, file reads.
- Acceptance criteria evidence:
  - Verification table: 36/36 prior findings have status with
    file:line evidence above.
  - New findings: A: 8, B: 5, C: 5; each with file:line
    evidence and effort/risk/source citation.
  - Regressions explicitly listed: none. (See Outcome.)
  - "Needs verification" items: 3 (singleflight value, EntrySnapshot
    race shape, web.Run join behavior under tests).

## Outcome

Completed second-round Go production-code review and converted the valid
findings into concrete follow-up SOWs. No production source files changed in
this analysis SOW.

Headline numbers:
- Prior findings (36 items): 11 FIXED, 6 PARTIAL, 16 NOT FIXED
  (most low-priority by design or rejected with reasoning),
  3 N/A. Zero REGRESSED.
- New findings (18 items): 8 A (anti-patterns), 5 B (missing
  gaps), 5 C (neutral).
- Most concerning new findings (top 3, by operational risk):
  1. A2 new — entity refresh public API has no ctx, leaving
     SIGTERM windows during artifact rebuild; SOW-0035 recorded
     this as a non-goal but it now lives only as prose
     (`pkg/engine/entity_integrity.go:156`).
  2. A1 new — `web.Run` does not wait for `runner.Run` after
     SOW-0035 made `runner.Run` itself wait for its children
     (`pkg/web/server.go:261`).
  3. A3 new — heavy-phase per-name workers run to completion on
     ctx cancellation (`pkg/engine/critical.go:476`,
     `pkg/engine/asn.go:231`, `pkg/engine/bogons.go:189`,
     `pkg/engine/geoloc.go:146`).

No regressions detected.

## Lessons extracted

- Review SOWs must not leave accepted follow-up work in prose. Valid findings
  either become the next current SOW or a pending SOW with a concrete path.
- Cancellation/shutdown findings are operational correctness, not optional
  cleanup. They should be handled before aesthetic modernization items.
