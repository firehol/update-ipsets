# SOW-0031 | 2026-04-30 | go-code-gap-analysis

## Status

completed

Maintainer-owned execution. The user explicitly delegated SOW-0031 through
SOW-0034 to the assistant with no further design questions. The analysis
findings remain the evidence base, but the prior "analysis only" constraint is
superseded: implement the findings that are justified by current code evidence,
quality impact, and risk/benefit. Defer or reject low-value/high-churn items
with recorded reasoning.

## Requirements

### Purpose

Gap-analysis of Go code against modern Go best practices (skill:
`go-best-practices`), to surface anti-patterns to eliminate, missing modern
patterns to add, and neutral improvements. Analysis only — implementation
deferred to separate per-batch SOWs after user review.

### User request quoted verbatim

> You are performing a gap-analysis of the Go code in this repo against modern
> best practices, and writing the result as a SOW.
>
> Repo: `/home/costa/src/firehol/update-ipsets` (Go 1.26 service: HTTP API,
> OTEL, background pipeline, embedded React UI). Public-facing, must be
> cache-first, bounded CPU/memory/IO.
>
> This is ANALYSIS ONLY. Do NOT modify any Go source, configs, tests, or other
> project files. The only file you write is the SOW itself.
>
> Skills to load and follow: `go-best-practices`, `project-coding`,
> `project-reviewing`, `project-content-surfaces`, `sow`.
>
> Scope: all Go code under `cmd/`, `internal/`, `pkg/`, `tools/` (do NOT
> include `pkg/iprange/` deep — only check that it remains standalone). Test
> files are out of scope for this SOW. Generated artifacts and frontend
> assets are out of scope.
>
> For every finding, classify it into Category A (anti-patterns to
> eliminate), Category B (missing gaps to fill), Category C (neutral
> improvements). Number every finding so the user can reply by number. Cap
> each category at the most important findings — quality over quantity. Do
> not invent issues to fill space. Mark anything you couldn't verify as
> "needs verification". Cross-reference SOW-0029 / SOW-0030 instead of
> restating findings already captured there.

Superseding execution request:

> the next 4 sows 31-34 are yours, about the code quality and testing of this
> application. I don't want to be involved. Consider them a gift from me. I
> have also researched 4 related skills which you can use while coding. I need
> you to review them, decide what is valid and what is not, research the
> application properly, and implement the ones you believe are justified. No
> questions for me.

Follow-up process correction:

> please also read the relevant skills. I recommend to deal with them one at
> time and avoid treating them mechanically. You first need to understand, then
> act. So, reading SOW + skill will give you the best clarity.

### Assistant understanding

- This SOW is no longer analysis-only. The original gap analysis remains the
  evidence base; the superseding maintainer request authorizes implementation
  of justified findings.
- Work must proceed one SOW at a time. For SOW-0031, read this SOW plus
  `go-best-practices`, `project-coding`, `project-reviewing`,
  `project-testing`, and `project-content-surfaces` before acting.
- The rubric is the `go-best-practices` skill; conflicts with `project-coding`
  resolve in favor of `project-coding`.
- SOW-0029 covered structural posture (file/package sizes, ownership, mutable
  cache state) and SOW-0030 has the phased plan addressing those. Findings
  already captured in those SOWs are referenced, not duplicated.
- Implementation happens in this SOW for findings whose current-code evidence,
  risk, and validation path justify immediate maintainer action.

### Acceptance criteria

- Each Category A/B/C finding has at least one concrete `file:line` evidence
  citation.
- Each finding has a numeric ID stable enough to be referenced in user
  decisions.
- Findings already covered by SOW-0029 or SOW-0030 are referenced rather than
  re-listed.
- Accepted implementation changes are validated with targeted tests and the
  relevant project gates.
- Deferred or rejected findings are recorded with reasoning so they do not
  disappear.

## Analysis

### Methodology

- Skills loaded: `go-best-practices`, `project-coding`, `project-reviewing`,
  `project-content-surfaces`, `sow`. No conflicts encountered between
  `go-best-practices` and `project-coding`; the project skill already
  encodes the project-specific overrides (cache-first serving, no startup
  rescans, `pkg/iprange` standalone, generated artifact mtimes,
  config-driven semantics).
- Commands run for evidence: `wc -l` over Go source files; `rg` for
  pattern-level smells (`ioutil`, `interface{}`, `panic(`, `init()`,
  `_ =` swallowed errors, `context.Background()`, `time.Sleep`,
  `sync.WaitGroup`, `sync.Pool`, `errors.Is/As/Join`, `errgroup`,
  `singleflight`, `slices`/`maps`, `iter.Seq`, `unique`, `weak`,
  `cmp.Or`, `pprof`, `fmt.Errorf("%v"`, `recover()`, modern HTTP timeout
  fields, `%w` wrapping, `WaitGroup.Go`).
- Files read end-to-end or in relevant ranges:
  `cmd/update-ipsets/daemon.go`, `pkg/engine/engine.go`,
  `pkg/engine/run.go`, `pkg/engine/run_pipeline.go`,
  `pkg/engine/entity_refresh_queue.go`, `pkg/engine/critical.go`,
  `pkg/engine/asn.go`, `pkg/engine/output.go`,
  `pkg/engine/format_handlers.go`, `pkg/engine/helpers.go` (header),
  `pkg/engine/asn_url_resolver.go`, `pkg/engine/download_stage.go`
  (around recovery), `pkg/scheduler/scheduler.go` (Run/Loop section),
  `pkg/web/server.go`, `pkg/web/middleware.go`, `pkg/web/http.go`,
  `pkg/web/cache.go`, `pkg/cache/cache.go`,
  `internal/fileutil/fileutil.go`, `Makefile`,
  `.github/workflows/ci.yml`, `tools/archposture/collect.go` (header).
- Cross-referenced SOW-0029 (code-quality-analysis) and SOW-0030
  (code-quality-refactor-phases) so structural ownership findings already
  captured there are not duplicated here.
- Date: 2026-04-30.

### Repo metrics (current snapshot)

Source counts (production Go, excluding `_test.go`): 207 files, 49,590 lines
(`find cmd internal pkg tools -type f -name '*.go' ! -name '*_test.go' |
xargs wc -l`).

Top largest production Go files (file:lines):

- `pkg/scheduler/scheduler.go` 1,474
- `pkg/engine/output.go` 1,366
- `pkg/web/admin.go` 1,043
- `pkg/engine/home_entity_builders.go` 1,021
- `pkg/engine/entity_integrity.go` 1,020
- `pkg/engine/critical.go` 987
- `pkg/engine/entity_surgical.go` 978
- `pkg/engine/entity_artifacts.go` 960
- `pkg/config/config.go` 944
- `pkg/engine/entity_feed_sidecar.go` 915
- `pkg/engine/download_stage.go` 887
- `pkg/engine/runtime_ledger_cache.go` 825
- `pkg/engine/helpers.go` 807

(Already tracked by `tools/archposture` baseline; SOW-0030 owns the
shrink work.)

CI gates currently active (`.github/workflows/ci.yml`):

- `make build` / `make test` / `make race` / `make lint` (= `go vet`)
- `make cross` for linux/amd64, linux/arm64
- `go test -coverprofile=coverage.out -covermode=atomic ./...` with a
  50% total-coverage threshold

Notable absences from CI: `staticcheck`, `golangci-lint`, `govulncheck`,
fuzz seeds, `goleak` for goroutine-leak detection.

Concurrency primitive count (production code): 22 anonymous `go func()`
launches, 12+ ad-hoc `sync.WaitGroup`+`sem`+`mu` fan-out blocks across
`pkg/engine`, `pkg/iprange`, `pkg/processor`, `pkg/web`,
`pkg/scheduler`, `cmd/update-ipsets`. Zero usages of
`golang.org/x/sync/errgroup`, `golang.org/x/sync/singleflight`, or
`golang.org/x/time/rate` (rate limiter is hand-rolled in
`pkg/web/middleware.go`).

Logging is consistently `log/slog` across daemon, engine, scheduler, web,
internal observability — no legacy `log` import outside test files.

### Findings — Category A: Anti-patterns to eliminate

**A1. `Runner` stores `context.Context` as a struct field.** —
`pkg/scheduler/scheduler.go:74` declares `ctx context.Context // parent
context for deriving run contexts`, and `Run(ctx)` writes
`r.ctx = ctx` at line 292.

- Evidence: only one caller reads `r.ctx` (line 292 itself; no later
  reader), so the field is effectively dead but the pattern is still
  the canonical "stored ctx" anti-pattern listed in
  `go-best-practices` §4 / Go blog "Contexts and structs".
- Why bad: encourages future code to call `r.ctx` instead of
  threading the per-call ctx through; teaches the wrong shape.
- Fix sketch: remove the field; if a future caller needs the runner
  base ctx, pass it explicitly.
- Effort: S.
- Risk if left: pattern proliferates; future scheduler features stop
  threading ctx and silently break cancellation.

**A2. Background goroutines spawned without context cancellation.** —
recurs across:

- `cmd/update-ipsets/daemon.go:86` — nested `go func()` inside the
  SIGHUP reload loop calls `eng.EnsureEntityArtifactsCurrentWithTrigger("reload")`
  with no ctx; on SIGTERM during reload, this goroutine runs to
  completion, blocking shutdown.
- `pkg/web/server.go:254` — `go func() {
  EnsureEntityArtifactsCurrentWithTrigger("startup") }()` — same
  shape, no ctx.
- `pkg/engine/entity_refresh_queue.go:24,60,80,170,197` — five
  goroutine launches into long-running entity refresh queues that
  accept no ctx and have no shutdown mechanism. Tail-spawn pattern at
  lines 170 and 197 (`go e.runQueuedEntityArtifactRefresh(trigger)`)
  also makes the lifetime non-obvious.
- Why bad: `go-best-practices` §4 "DON'T launch goroutines without
  cancellation"; rednafi.com on goroutine leaks; project rule
  "Background work must be visible through the admin API/UI" — work
  that ignores shutdown corrupts the visibility/integrity contract.
- Fix sketch: thread a parent ctx (engine ctx for engine-spawned
  goroutines, daemon ctx for daemon-spawned ones); on
  `<-ctx.Done()` exit early; turn the tail-spawn into a `for` loop.
- Effort: M.
- Risk if left: shutdown hangs; entity artifacts get truncated mid-write
  if the process is killed during the window between SIGTERM and
  process exit; admin reload can race with SIGTERM and complete an
  unwanted refresh after operator intent to stop.

**A3. Hand-rolled `sync.WaitGroup`+`sem`+`mu` fan-out instead of
`errgroup.WithContext`.** — Recurs in:

- `pkg/engine/critical.go:463-488` (worker loop with `firstErr`,
  `errOnce`, `mu`)
- `pkg/engine/asn.go:237-265` (same shape, per-provider)
- `pkg/engine/bogons.go:180-187`
- `pkg/engine/entity_feed_sidecar.go:578-668` (multiple blocks)
- `pkg/engine/run_pipeline.go:42-93` (the new Phase 4b code still
  uses ad-hoc fan-out: `wg`+`sem`+`mu`+`results map`)
- `pkg/engine/output.go:415-440` (pair fan-out for comparisons)
- `pkg/engine/geoloc.go:154-170`
- `pkg/processor/primitives.go:143-188`
- Why bad: each block re-implements first-error capture, semaphore,
  result aggregation, and ctx-cancel propagation. None propagate
  ctx into the workers (workers in `critical.go` line 472 have no
  ctx parameter). `errgroup.WithContext`
  (https://pkg.go.dev/golang.org/x/sync/errgroup) replaces all of
  this with two lines and ensures first-error cancellation
  propagates to peer workers immediately.
- Fix sketch: introduce `golang.org/x/sync/errgroup` (already a
  zero-cost stdlib-adjacent dep; would join the existing
  `go.opentelemetry.io/...` and `vishvananda/netlink` peer set).
  Convert one fan-out at a time. Keep the `numWorkers`
  cap via `errgroup.SetLimit(n)` (Go 1.20+).
- Effort: M (per fan-out block, 12+ blocks).
- Risk if left: workers keep running after the first failure (waste of
  CPU and memory on heavy comparison phases); ctx cancellation does
  not propagate; bug-fix surface is multiplied by N.

**A4. `time.Sleep`-style busy reload — fixed-window rate limiter
goroutine leak.** — `pkg/web/middleware.go:32` starts
`go l.cleanupLoop()` on every `newFixedWindowLimiter`; the goroutine
only exits when its `time.Ticker` is stopped, which never happens
because there is no shutdown method. `rateLimitMiddleware`
(line 71-85) creates one limiter per call. In the daemon path the
function is called once per handler tree, so today it's a single
leak; but the structure invites multi-leak in tests, hot-reload,
and future surface restructures.

- Evidence: `pkg/web/middleware.go:32`, `:38-49`, `:71-72`. No
  `Stop()` method on `fixedWindowLimiter`.
- Why bad: `go-best-practices` §4 "every goroutine has a
  cancellation path"; also `golang.org/x/time/rate` (token bucket)
  is the canonical replacement and avoids the well-known fixed-window
  2x-burst-at-boundaries property.
- Fix sketch: replace with `rate.NewLimiter`
  (https://pkg.go.dev/golang.org/x/time/rate) keyed by client IP
  in an `lru.Cache` (or a simple map with bounded TTL eviction);
  no background goroutine needed because `rate.Limiter` is
  pull-based.
- Effort: S/M.
- Risk if left: behavior is non-canonical (boundary bursts); future
  refactors that re-create the limiter leak goroutines.

**A5. `interface{}` instead of `any` (single residual occurrence).** —
`pkg/engine/output.go:1357` — `func jsonMarshalTabIndent(v
interface{}) ([]byte, error)`. Every other `func` signature in the
codebase already uses `any`; this is a stale survivor.

- Why bad: cosmetic only; project consistency.
- Fix sketch: rename to `any`.
- Effort: S.
- Risk if left: trivial; flagged here only because it is the single
  remaining occurrence and a one-character fix.

**A6. `context.Background()` in non-startup pipeline paths.** —
26 production occurrences. Confirmed pipeline-internal (i.e. NOT
startup/CLI/observability) sites:

- `pkg/engine/download_stage.go:143` — inside
  `RecoverStagedArtifacts(enableAll)`, called from scheduler at
  startup; `materializeArtifactChildren` accepts ctx but is fed
  `Background()`.
- `pkg/engine/download_stage.go:853` —
  `e.fetchAndStageHistoryDerivative(context.Background(), …)` in a
  pipeline branch.
- `pkg/engine/feed_body_stage.go:32` — body parser called from
  pipeline.
- `pkg/engine/retention.go:34`, `pkg/engine/fileset_helpers.go:127`,
  `pkg/engine/bootstrap_entries.go:297` — `iprange.LoadPath`/`ParseReader`
  with `Background()`; these are reachable from `RunOnce(ctx, …)`
  which has a real ctx available.
- `pkg/processor/processor.go:408,416,501` — `gunzipFile(context.Background(), …)`
  called from feed processing helpers.
- `pkg/downloader/canonical.go:46,62` — `iprange.ParseReader(context.Background(), …)`
  in a path the caller could pass ctx through.
- Why bad: defeats cancellation. SIGTERM during a long
  `materializeArtifactChildren` or `gunzipFile` should be honored.
  `go-best-practices` §4 "propagate ctx into every blocking call".
- Fix sketch: thread ctx down (most callers already have it; the
  signatures need extending) one package at a time.
- Effort: M (per package), L (whole tree).
- Risk if left: long-running pipeline sub-operations cannot be
  cancelled; SIGTERM blocks until they finish naturally; integrity
  windows enlarge on shutdown.

**A7. Unbounded `fileCache` map without eviction.** —
`pkg/web/cache.go:21-87`. The `fileCache.files` map grows by one
entry per unique path; entries are never evicted, only replaced when
mtime changes. Each entry holds the file body in memory.

- Evidence: `c.files[path] = entry` at line 84; no LRU, no TTL, no
  size cap. `os.Stat` runs on every request even on a cache hit
  (line 56-67).
- Why bad: `go-best-practices` §5 "DON'T pool large or long-lived
  objects" / §6 "DON'T retain references to whole files just to
  serve them later"; project rule "public serving must stay
  cache-first and cheap" — but cache-first does not mean
  unbounded-memory. If `feed_descriptions/*.html` paths are bounded
  this is fine; if any caller serves user-derivable paths through
  this, memory grows.
- Fix sketch: bound the cache (size + TTL) or stream from disk
  (already done correctly in `serveRawFeedFile`,
  `pkg/web/http.go:129-145`). Strongly consider serving directly
  via `http.ServeFile`/`http.ServeContent` without buffering.
- Effort: M.
- Risk if left: memory growth proportional to set of distinct served
  paths; defeats `sendfile(2)` zero-copy on Linux.
- Needs verification: are all paths reaching `fileCache.ServeFile`
  bounded by static config (methodology pages, public artifacts) or
  can a public request reach an unbounded path space? Not verified
  in this analysis.

**A8. `serveFileWithCaching` reads the whole file and computes
SHA-1 on every request.** — `pkg/web/http.go:94-127`. Unlike
`fileCache`, this path performs a fresh `os.ReadFile` + `sha1.Sum`
on every request, then hands a `bytes.Reader` to `http.ServeContent`.

- Evidence: lines 100-105.
- Why bad: defeats `sendfile`; allocates the whole body each request;
  hashes whole body even when the request will return 304.
- Fix sketch: open the file, derive an ETag from `(size, mtime)` (or
  cache the digest in the file's `info`), pass the `*os.File` to
  `http.ServeContent`. The conditional check inside `ServeContent`
  will short-circuit before reading body bytes.
- Effort: S.
- Risk if left: per-request allocations and CPU on every static asset
  request; less measurable on modest traffic but disproportionate to
  the work needed to serve a 304.

**A9. Tail-spawn goroutine recursion in entity refresh queues.** —
`pkg/engine/entity_refresh_queue.go:170,197` — at the end of the
function, `go e.runQueuedEntityArtifactRefresh(trigger)` spawns a
new goroutine if pending work was added during the run.

- Evidence: lines 169-171, 196-198.
- Why bad: each iteration creates a fresh stack/goroutine; lifetime
  is opaque to admin visibility (the new goroutine appears as a
  separate "background task"); makes shutdown ordering harder. The
  entire pattern can be a single `for hasPending { … }` loop in one
  goroutine, since `entityRefreshRunning=true` already guards
  exclusivity.
- Fix sketch: convert to in-loop continuation.
- Effort: S.
- Risk if left: low; cosmetic but increases reasoning cost about
  background-task lifetimes.

**A10. JSON marshalling helpers using legacy idioms.** —
`pkg/engine/output.go:1357-1359` — `func jsonMarshalTabIndent(v
interface{}) ([]byte, error) { return json.MarshalIndent(v, "",
"\t") }`.

- Evidence: line 1357.
- Why bad: trivial wrapper that doesn't add value beyond the param
  name. Combined with A5 (`interface{}` survival) the wrapper looks
  like dead form. 8 callers across `pkg/engine` could call
  `json.MarshalIndent(v, "", "\t")` directly or use a single
  package-private helper at most.
- Fix sketch: inline OR rename to `any` and keep; either is fine.
- Effort: S.
- Risk if left: trivial.

**A11. `_ = err` after a comment block to silence Go's unused-error
rule.** — `pkg/downloader/internal.go:234`. The surrounding comment
explains why best-effort. The pattern is acceptable, but the
project-wide convention preference is `_ = closer.Close() // best
effort` style — i.e. the explanation goes on the same line.

- Why bad: borderline; flagged so the user can choose to standardize.
- Fix sketch: comment on the `_ = err` line; or remove the dummy
  assignment and let the comment-only block explain.
- Effort: S.
- Risk if left: minimal.

**A12. Reusing `http.DefaultClient` for one-off outbound HTTP
requests with a path-controlled URL.** — `pkg/engine/asn_url_resolver.go:63`.
The request itself has a 30s ctx-with-timeout (line 56), so it is
bounded — but `DefaultClient` shares Transport state (connection
pool, idle conns) across the whole binary including any future
caller, and has no per-client `Transport.MaxIdleConnsPerHost`,
`ResponseHeaderTimeout`, or `TLSHandshakeTimeout` overrides.

- Evidence: line 63.
- Why bad: `go-best-practices` §5; Cloudflare guide on net/http
  timeouts. The `pkg/downloader` package already constructs its own
  client; adding a tiny dedicated client to `pkg/engine` (or
  injecting the downloader client) would be more consistent.
- Fix sketch: instantiate `&http.Client{ Transport: &http.Transport{
  ResponseHeaderTimeout: …, TLSHandshakeTimeout: … }, Timeout: 30 * time.Second }`
  once at engine construction.
- Effort: S.
- Risk if left: minor today; surfaces as an incident if future code
  on the same default client misbehaves.

**A13. Sentinel fan-out skip-on-cancel uses raw channel sends.** —
`pkg/engine/run_pipeline.go:47-54`. The goroutine does
`select { case sem <- struct{}{}: case <-ctx.Done(): return }` then
`defer func() { <-sem }()` — if ctx is cancelled BEFORE acquiring
the slot, the goroutine returns without releasing, which is
correct. But the result map is then incomplete and downstream
`for _, name := range batchNames { results[name] }` skips with `if
!ok` — silently dropping the source.

- Evidence: `pkg/engine/run_pipeline.go:49-53`, `:95-100`.
- Why bad: when ctx is cancelled mid-batch, status of cancelled
  feeds is silently absent from `report.Statuses`. SOW-0030 rules
  require the engine to remain inspectable. The right outcome on
  cancel is to record an explicit "cancelled" status, not absence.
- Fix sketch: when cancelled, record `result: { Err:
  ctx.Err(), Message: "cancelled" }`. Also fits into the
  `errgroup` migration in A3.
- Effort: S/M.
- Risk if left: shutdown-during-run produces incomplete reports;
  callers (admin UI) cannot tell aborted from skipped.

**A14. Tracking note: file/function/cache mutation hotspots.** —
Already captured in SOW-0029 analysis (`pkg/engine/output.go:1366`,
`pkg/scheduler/scheduler.go:1474`, `pkg/web/admin.go:1043`,
`pkg/cache/cache.go` mutable `*Entry` exposure) and SOW-0030
phases 1b/3/4 plan to address them. Not re-listed here. Same for
the engine-struct-with-30+-fields shape (see SOW-0030 Phase 4a/4b).

**A15. `pkg/engine/helpers.go` is an 807-line dumping ground.** —
`pkg/engine/helpers.go` has 50 functions including URL expansion,
ipset path construction, atomic file writes, hash helpers, etc.
This is the project-internal version of the "package helpers"
anti-pattern (`go-best-practices` §2). It's a single package so
the cycle-risk is muted, but the file itself is a navigability
problem.

- Evidence: file line count 807; 50 `^func` matches.
- Why bad: single-purpose principle; new contributors cannot
  predict what `helpers.go` contains; conflict surface for
  parallel work.
- Fix sketch: split by concern (`urlexpand.go`, `paths.go`,
  `hashing.go`, `atomic_write.go`); some helpers belong in
  `internal/fileutil` (already exists) or a new
  `internal/textexpand`.
- Effort: M.
- Risk if left: continued growth of the file; lower discoverability
  for the right place to add new code.

### Findings — Category B: Missing gaps to fill

**B1. No `staticcheck` / `golangci-lint` / `govulncheck` in CI.** —
`.github/workflows/ci.yml` has `make build`, `make test`,
`make race`, `make lint` (= `go vet`), `make cross`, and a 50%
coverage gate. There is no `staticcheck`, no `golangci-lint`, no
`govulncheck`, and no `goleak` for goroutine leak detection.

- Why this matters here: a public-facing service with OTel exporters
  pulling in a wide dependency tree (`go.opentelemetry.io/...`,
  `mvdan.cc/sh/v3`, `vishvananda/netlink`) needs a vulnerability
  gate. `staticcheck`'s 150+ checks catch idiom drift that `go vet`
  does not. SOW-0030 already added an architecture posture gate;
  it does not replace these.
- Effort: S (drop a `.golangci.yml` and add 2 CI steps), recurring.
- Sources:
  https://staticcheck.dev/docs/,
  https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck,
  https://golangci-lint.run/

**B2. No panic-recovery middleware in the HTTP stack.** —
`pkg/web/middleware.go` has `gzipMiddleware`, `corsMiddleware`,
`rateLimitMiddleware`, `wrapAdminAuth`, `basicAuth`, `logMiddleware`
— and no panic-recovery middleware. `net/http` does have a
default recover-and-close-connection in `serverHandler`, but it
neither logs structured information nor increments a metric.

- Why this matters here: an unrecovered handler panic in admin or
  public routes makes the request fail silently from the operator's
  point of view. The project has detailed OTel instrumentation
  elsewhere; missing panic visibility is an asymmetric gap.
- Effort: S. Add `recoverMiddleware(next)` that defers a recover,
  logs `slog.Error("panic in handler", "stack", debug.Stack(),
  "method", r.Method, "path", r.URL.Path)`, and writes
  500. Add to the wrapper chain in
  `pkg/web/server.go:newSurfaceHandler`.
- Sources:
  https://medium.com/@iarsham/dont-let-panics-crash-your-go-application-mastering-recoveries-in-middleware-9e1cf657987f

**B3. No `errgroup`/`singleflight`/`x/time/rate`.** —
`go.mod` shows zero `golang.org/x/sync` or `golang.org/x/time`
imports. The codebase hand-rolls all three:

- `errgroup` — see A3.
- `singleflight` — public artifact serving could benefit if any
  cache-miss path exists. Need to verify whether any public route
  triggers an expensive computation that two simultaneous requests
  would both perform; project rule says "public requests must not
  trigger upstream downloads or broad recomputation", so this may
  be a non-issue. **Needs verification.**
- `x/time/rate` — see A4.
- Why this matters here: each is one of the canonical 1-line
  replacements; bringing them in once unblocks a cluster of
  improvements with low blast radius.
- Effort: S (deps), M-L (migrations per call site).
- Sources: https://pkg.go.dev/golang.org/x/sync,
  https://pkg.go.dev/golang.org/x/time/rate.

**B4. No `errors.Join` or `errors.Is/As` usage despite multiple
multi-error sites.** — Project-wide count of `errors.Is|As|Join`
in production: 40 (across all packages). The fan-out blocks (A3) all
collapse multiple goroutine errors into a single `firstErr`,
losing the others. `errgroup` returns the first; `errors.Join` is
the fix when the operator wants all of them.

- Why this matters here: the engine pipeline already produces
  `report.Failed []string`. The corresponding error chain is lost
  to a single `firstErr`. Operators triaging a multi-feed batch
  cannot see all errors.
- Effort: S (per fan-out converted in A3 already does this for
  free).

**B5. No `iter.Seq` usage where streaming over large derived sets
would simplify.** — Project is on Go 1.26. `pkg/iprange` exposes
`RangeSource` interfaces; the engine streams set membership through
custom iterator-style helpers. `iter.Seq2[K,V]` and the new
`for k, v := range seqfunc {}` syntax (Go 1.23+) reduce
boilerplate when iterating over map-like data structures.

- Why this matters here: not urgent. The current iterator pattern
  works. Listed because the codebase explicitly targets Go 1.26
  but uses no `iter.Seq` at all.
- Effort: M (only at refactor time).
- Risk if left: none acute; missed simplification.

**B6. No `unique.Make` / `weak.Pointer` for repeated string
metadata.** — Cache state stores `Category`, `Maintainer`,
`License`, `Attribution`, `LastStatus` per entry. Many entries
share the same value (`"attacks"`, `"GreenSnow.co"`, etc.).
`unique.Make[string]` (Go 1.23+) would intern these; cache
read-paths get pointer-cheap equality, GC reclaims unreferenced
canonical copies.

- Why this matters here: for hundreds-of-feeds scale and cache
  state held across the daemon's lifetime, this is a quantifiable
  memory win. Marked B because it's nice-to-have and changes the
  JSON marshalling shape (need `Value()` calls during marshal).
- Effort: M.
- Risk if left: small extra memory; not load-bearing.

**B7. No `sync.OnceValue`/`sync.OnceFunc` usage.** —
`pkg/web/methodology.go:45` and `pkg/engine/insights.go:31` both
use the older `sync.Once` + global var pattern. `sync.OnceValue`
(Go 1.21+) replaces the pattern with one line and avoids the
typo-prone double-write race window:

  Currently `var methodologyPagesOnce sync.Once; methodologyPagesOnce.Do(func(){ methodologyPages = ... })`.
  Modern: `var methodologyPages = sync.OnceValue(func() map[string]*methodologyPage { ... })`.

- Effort: S, per call site.
- Sources: https://victoriametrics.com/blog/go-sync-once/.

**B8. Hand-rolled JSON marshalling indent without
`encoding/json/v2` consideration.** — Go 1.25 introduced
`encoding/json/v2` as experimental; not a recommendation to
migrate yet, but worth knowing about given multiple JSON output
paths in the engine.

- Effort: defer.
- Marked here only so future SOWs know to evaluate it.

**B9. Benchmarks still use `for i := 0; i < b.N; i++`.** —
`pkg/processor/stream_test.go:536,551,572,587`,
`pkg/iprange/bench_test.go:22,37,56,72,132,157`. Go 1.24
introduced `b.Loop()` which is more accurate (handles the
warmup/measurement split correctly without manual `b.ResetTimer`)
and removes boilerplate.

- Why this matters here: benchmark accuracy improves; existing
  benches do not call `b.ResetTimer` consistently so warmup costs
  may be folded into measurement.
- Effort: S.
- Note: benchmarks are in `_test.go`, technically out of THIS SOW's
  scope. Listed under B because moving to `b.Loop()` would naturally
  pair with any benchmark refresh.

**B10. No `WaitGroup.Go` (Go 1.25+) usage.** — Every fan-out
block uses the older `wg.Add(1); go func(){ defer wg.Done(); …
}()` shape. Combined with A3 (move to `errgroup`), this is
mostly moot, but for fan-outs that legitimately do not need
error aggregation, `wg.Go(func(){ … })` removes the
`Add(1)`/`defer Done()` boilerplate.

- Effort: S (cosmetic).

**B11. No goroutine-leak test in CI.** —
`go.uber.org/goleak.VerifyTestMain` would catch the kinds of
leaks A2/A4 enable. Especially relevant given several test
files exercise scheduler/engine paths that spawn goroutines.

- Effort: S to add for one or two packages; M to roll out.
- Sources: https://pkg.go.dev/go.uber.org/goleak.

### Findings — Category C: Neutral improvements

**C1. `sort.Slice`/`sort.Strings` everywhere instead of
`slices.SortFunc`/`slices.Sort`.** — 70 production occurrences of
`"sort"` import vs 2 of `"slices"`. Examples:
`pkg/config/config.go:417,435,453,750,760,864,922`,
`pkg/config/validate.go:576,597`, `pkg/geoloc/helpers.go:139`,
`internal/telemetry/counter.go:65`, `tools/archposture/collect.go:173,176,182,188,230`.

- Why C not A: `sort.Slice` works fine; `slices.SortFunc` is
  more readable, generic, and slightly faster (no reflection).
  `slices.Sort` for strings replaces `sort.Strings`. Pure
  modernization.
- Effort: S per file; M total.

**C2. `sort.Slice(s.ranges, func(i, j int) bool { … })` reflection
overhead in `tools/dronebl2ipsets/ranges.go:45`.** — Replace with
`slices.SortFunc`. C-only because dronebl is a small helper.

**C3. `helpers.go`/`*_helpers.go` files are a navigability smell.**
— `pkg/engine/helpers.go` (807 lines), `pkg/engine/fileset_helpers.go`
(167), `pkg/engine/home_detail_helpers.go` (162). See A15 for
helpers.go specifically; C3 covers the generalized smell that
`*_helpers.go` is the spelling of "I didn't decide what package
this belongs to".

- Effort: M per file.

**C4. `pkg/engine/format_handlers.go` `init()` registration is
fine; just confirming.** — `init()` here populates an in-process
map (no side effects beyond the registry). Project skill
acknowledges this is OK. Listed as C only so future contributors
do not flag it.

**C5. Engine struct field ordering.** — `pkg/engine/engine.go:151`
defines `Engine` with mixed field sizes (booleans interspersed
with maps and slices). Running `golang.org/x/tools/go/analysis/passes/fieldalignment`
would identify padding waste. Marginal; the struct is allocated
once.

- Effort: S to run, S to apply.
- Sources: https://goperf.dev/01-common-patterns/fields-alignment/.

**C6. `pkg/web/cache.go` and `pkg/web/http.go`
`serveFileWithCaching` are two different file-serving paths
with different perf profiles.** — A8 (eager hash) and A7
(unbounded cache) are the substantive issues; C6 captures the
"two paths" smell separately. Long-term, a single
file-serving abstraction reduces the surface for further
divergence.

**C7. Multiple top-level "global" `sync.Once`+package-var
patterns could be `sync.OnceValue` (see B7) — listed under C
because the existing code is correct.**

**C8. The `Runner` / `Engine` structs hold logger and ctx and
clocks as fields side-by-side with state.** — Standard pattern,
not a violation; future ownership cleanup (SOW-0030 phases) will
naturally split this. Not actionable in isolation.

**C9. Doc comments missing on some exported APIs.** — Sample
spot-checks (e.g., `pkg/engine/run_pipeline.go:13` `type
sourceResult struct` is private, fine; `pkg/engine/engine.go`
`type Report struct` has no godoc) — coverage is partial. A
`golangci-lint` config with `revive`'s `exported` rule (B1)
would flag these systematically.

**C10. Cross-cutting OTel instrumentation discipline.** —
Many code paths have OTel spans (`observability.Start`); some
do not. Public methodology forbids shipping internal mechanics in
public copy, but operators benefit from consistent spans on
every download/parse/marshal path. Out of scope for THIS gap
analysis; flagged so future SOWs notice the partial coverage.

### Notes / known limits

- "Needs verification" items: B3 (`singleflight` value depends on
  whether any public path triggers expensive recomputation; project
  rule says no, but I did not exhaustively trace all public route
  handlers); A7 (`fileCache` path-space boundedness — depends on
  which callers pass which paths; I did not trace every call site
  to confirm).
- I did not fully trace per-package OTel span coverage; C10 is
  noted as a partial observation, not a confirmed gap.
- Test files are out of scope. I noticed in passing that several
  test packages spawn goroutines that may outlive `t.Cleanup`; a
  proper Go-tests SOW (separate from this one) is the right place.
- `pkg/iprange` was checked only for the standalone-imports
  invariant (no project imports outside `pkg/iprange`) — confirmed
  clean.
- Conflicts between `go-best-practices` and `project-coding`: none
  encountered. The project skill encodes a stricter overlay
  (cache-first, no startup rescans, generated-file mtimes,
  config-driven semantics) which `go-best-practices` does not
  contradict.
- Findings already covered by SOW-0029 / SOW-0030 (and therefore
  intentionally NOT re-listed):
  - File/function size hot-spots (`output.go`, `scheduler.go`,
    `admin.go`, etc.) — SOW-0029 §Analysis, SOW-0030 phases 4/5.
  - `pkg/cache/cache.go` mutable `*Entry` exposure — SOW-0030
    Phase 1a/1b.
  - Engine struct mixing concerns — SOW-0030 Phase 4a/4b.
  - Scheduler dense state machine — SOW-0030 Phase 3.
  - Route-family extraction in `pkg/web` — SOW-0030 Phase 2
    (already shipped; A2's `EnsureEntityArtifactsCurrentWithTrigger("startup")`
    goroutine survives that work).

## Implications and decisions

User decisions required (numbered for reply-by-number):

**1. Which Category A items to schedule first?**
Options:

- (a) Schedule all 13 Category A findings in one combined SOW
  ("modernize concurrency + ctx + middleware").
- (b) Split by risk into two SOWs:
  - A-fast (S/M effort, low risk): A1, A4, A5, A9, A10, A11, A12,
    A13, A15.
  - A-deep (M/L effort, higher risk): A2, A3, A6, A7, A8.
- (c) Split by domain:
  - Concurrency/ctx (A2, A3, A6, A13).
  - HTTP/middleware (A4, A7, A8, A12).
  - Cleanup (A1, A5, A9, A10, A11, A15).
- (d) Defer pending discussion.

Recommendation: **(b)** — A-fast can ship as one safe batch; A-deep
needs ctx-propagation discipline that touches many call sites and
benefits from being its own SOW with race/integrity validation
gates. Rationale: A2 and A6 in particular cross multiple packages
and need the same careful "thread ctx down" review pass. A3
benefits from `errgroup` being introduced by A-deep so subsequent
fan-out conversions don't re-debate the dependency.

Implications:

- Picking (a) means a single large SOW with broader review surface;
  higher chance of regressions slipping in.
- Picking (b) means two follow-up SOWs; A-fast can land within
  hours, A-deep within days.
- Picking (c) creates three smaller SOWs; more SOW overhead, but
  each is self-contained.

Risks:

- A2/A3/A6 changes can introduce new ctx-cancel paths that fail
  fast where previously the pipeline ran to completion. Engine
  integrity tests must run with race detector before merge.

**2. Are Category B gaps in scope for this codebase?**
Options:

- (a) All of B (B1-B11) — make this a "modern tooling and
  primitives" SOW.
- (b) Only the high-impact CI/runtime gates: B1
  (staticcheck/golangci-lint/govulncheck), B2 (panic-recovery
  middleware), B11 (goleak). Defer the "modern stdlib idioms"
  items (B5, B6, B7, B8, B10) to opportunistic refactors.
- (c) Only B2 (panic-recovery is a real correctness gap; others
  are nice-to-have).
- (d) None of B for now.

Recommendation: **(b)** — B1 is a CI gate that immediately starts
finding issues we will then file follow-ups for; B2 closes a real
incident-visibility gap; B11 is the single guardrail that makes
A2 less likely to regress in the future. The stdlib-idiom items
(B5/B6/B7/B8/B10) are aesthetic given the project already runs
correctly; they should be folded into A-fast or refactor-as-you-go.

Implications:

- (b) introduces three CI/runtime additions; first
  staticcheck/golangci-lint run may produce a long backlog of
  warnings. Plan for that.
- (b) requires deciding whether to enforce `staticcheck` blocking
  or warning. Recommendation: warning first, blocking after
  cleanup.
- B3 (`errgroup`/`singleflight`/`x/time/rate`) is partially
  subsumed under A3/A4; if option (b) is selected for B and
  option (b) for question 1, the deps land naturally.

Risks:

- staticcheck added blocking from day one would freeze unrelated
  PRs.
- govulncheck may surface CVEs in `mvdan.cc/sh/v3` or OTel that
  are non-trivial to fix.

**3. What to do with Category C items?**
Options:

- (a) Ignore — they are not problems today.
- (b) Opportunistic during related work (handle each C item only
  when its file is being touched for another reason).
- (c) Standalone "modernization SOW" (sort.Slice → slices.SortFunc,
  field alignment, doc comments).
- (d) Mix: handle C1 (sort → slices) as a one-shot mechanical
  cleanup; defer C2-C10 to (b).

Recommendation: **(b)** — Category C items by definition do not
warrant their own SOW today. The enforcement that matters
(via staticcheck once B1 lands) will surface them naturally.

Implications:

- (b) means the C list is reference material, not a work plan.
- (c) creates extra SOW overhead for items that have no concrete
  benefit beyond consistency.

Risks:

- (b) means C items may be ignored indefinitely if no one touches
  the surrounding files. That is the correct outcome for items
  whose value is consistency only.

## Plan

Execution plan after maintainer delegation:

- Implement justified low/medium-risk runtime/code fixes in this SOW:
  A1, A4, A5/A10, A8, A9, A12, A13, A15 where mechanical and bounded, and B2.
- Implement dependency-backed primitives only where they directly replace a
  confirmed problem: `x/time/rate` for A4; defer broad `errgroup` conversion
  until each fan-out block has a package-specific validation pass.
- Defer high-blast-radius ctx propagation/fan-out rewrites (A2, A3, A6) unless
  a touched call path exposes a contained fix; record follow-up rationale.
- Defer aesthetic modernization (C items) unless the same file is already being
  touched.
- Validate with targeted package tests, `make test`, `make race`, `make lint`,
  and `go test ./tools/archposture` when architecture-sensitive files change.

## Execution log

2026-04-30:

- Loaded project skills `go-best-practices`, `project-coding`,
  `project-reviewing`, `project-content-surfaces`, and `sow`.
- Read the most recent code-quality SOWs (SOW-0029 done, SOW-0030
  in progress) to avoid duplicating findings already captured.
- Collected metrics with `wc -l`, `rg`, and selective file reads.
- Spot-checked representative code paths in `cmd/update-ipsets`,
  `pkg/engine`, `pkg/scheduler`, `pkg/web`, `pkg/cache`,
  `internal/fileutil`, `tools/archposture`.
- Authored this SOW under `.agents/sow/pending/` per the user's
  instruction.
- No product files were modified.
- Superseding user request converted SOW-0031 from analysis-only into
  maintainer-owned implementation. User also instructed SOWs 31-34 to be handled
  one at a time after reading the relevant SOW and skills.
- Re-read SOW-0031 and the relevant skills:
  `go-best-practices`, `project-coding`, `project-reviewing`,
  `project-testing`, and `project-content-surfaces`.
- Accepted and implemented:
  - A1: removed the dead stored `context.Context` field from `scheduler.Runner`.
  - A4/B3: replaced the fixed-window rate limiter and background cleanup
    goroutine with a lazily pruned token-bucket limiter backed by
    `golang.org/x/time/rate`.
  - B2: added same-goroutine HTTP panic recovery inside the gzip middleware
    boundary so panic responses remain valid gzip responses when negotiated.
  - A5/A10: changed the remaining `interface{}` JSON helper argument to `any`.
  - A9: converted queued entity refresh tail-spawn recursion into an in-goroutine
    loop.
  - A12: replaced `http.DefaultClient` for CAIDA creation-log lookups with a
    dedicated bounded HTTP client/transport.
  - A13: record `cancelled` processing status when a selected source is
    cancelled before a worker slot is available, instead of silently omitting it
    from run accounting.
  - A8, revised after source verification: `serveFileWithCaching` had no
    production callers, so the runtime hot-path concern was not valid as stated.
    The dead helper was removed instead of being optimized mechanically.
- Added behavioral web middleware tests for rate limiting and panic recovery
  gzip behavior.
- Updated `.agents/sow/specs/operating-principles.md` and
  `.agents/sow/specs/pipeline.md` for the new cancellation/reporting and HTTP
  middleware contracts.
- Follow-up/rejection ledger correction:
  - A2/A3/A6 broad context propagation and errgroup conversion: valid,
    cross-package, and high-blast-radius. Tracked as
    `.agents/sow/pending/SOW-0035-20260430-go-concurrency-cancellation-hardening.md`.
  - A7 fileCache bounding: valid concern but not fully verified as an unbounded
    public path; raw feed routes already stream and the JSON/static cache path
    needs a caller/path-space audit before redesign. Tracked as
    `.agents/sow/pending/SOW-0036-20260430-static-artifact-cache-bounds.md`.
  - A11 comment style: rejected as a standalone work item. Evidence: the
    finding is readability-only, has no correctness/performance/release impact,
    and touching files only to change comments creates churn without improving
    the release posture.
  - A15 helper-file split and Category C modernization: rejected as standalone
    deliverables. Evidence: these are consistency/aesthetic improvements unless
    the owning package is already being changed for a behavior, test, or
    maintainability reason. They may be handled opportunistically inside future
    package-specific SOWs, but they are not backlog by themselves.

## Validation

- Targeted tests:
  - `go test ./pkg/web ./pkg/engine ./pkg/scheduler`
  - `go test ./pkg/web -run 'Test(ClientRateLimiter|RecoverMiddleware)' -count=1`
- Full Go gates:
  - `make test`
  - `make lint`
  - `make race`
  - `make build`
  - `go test ./tools/archposture`
- Validation notes:
  - No frontend build was required for SOW-0031; no frontend source changed.
  - No installed-service smoke was required for SOW-0031 because runtime
    behavior changed inside request middleware and processing accounting, and
    the validated paths are covered by Go tests/build/race. Install validation
    remains required when SOW-0033/SOW-0034 touch embedded UI assets.

## Outcome

Completed. The accepted SOW-0031 batch shipped:

- removed dead stored scheduler context
- replaced fixed-window request limiting with lazily-pruned token buckets
- added HTTP panic recovery in the middleware chain
- removed a dead private file-serving helper after proving it had no production
  callers
- bounded CAIDA creation-log HTTP fetch behavior with a dedicated client
- removed entity-refresh tail-spawn recursion
- made worker-slot cancellation visible as `cancelled` run accounting
- updated specs and project skills with the reusable lessons

## Lessons extracted

- Do not optimize a suspected hot path until caller evidence confirms it is a
  real production path. In this SOW, `serveFileWithCaching` had no production
  callers; the correct maintainer action was dead-code removal, not adding tests
  around unused code.
- Standard-library-adjacent dependencies are justified when they replace a
  confirmed custom runtime primitive. `golang.org/x/time/rate` replaced a
  leaky fixed-window limiter and removed a background goroutine.
