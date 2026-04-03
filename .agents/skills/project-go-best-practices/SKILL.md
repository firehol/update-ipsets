---
name: project-go-best-practices
description: "Modern Go (1.22+) DOs and DON'Ts for high-performance APIs, clean code, separation of concerns, and avoiding common AI-generated code pitfalls. Use when writing or reviewing Go code."
---

## TL;DR

- Reach for the standard library first; modern stdlib (`log/slog`, `net/http` ServeMux 1.22, `errors.Join`, `iter.Seq`, `unique`, `testing/synctest`) covers most needs that used to require third-party packages.
- Pass `context.Context` as the first argument; never store it in a struct, never pass it by pointer, never use `WithValue` for required dependencies.
- Bound everything: timeouts on every HTTP server, cancellation on every goroutine, `errgroup` for fan-out, `singleflight` for cache stampedes, `sync.Pool` only for short-lived hot-path objects.
- Errors wrap with `%w`, compare with `errors.Is/As`, return early; do not swallow with `_` and do not log+return.
- Treat LLM-generated Go with extra suspicion: it tends to use deprecated `ioutil`, dead routers like `gorilla/mux`, leaky goroutines, panic-spammed defers, `interface{}` boxing, and "utils" packages. The checklist in section 10 catches most of it.

## 1. Modern stdlib first

Pick stdlib over third-party when both work. Smaller dep graph, lower CVE surface, faster to learn for new contributors.

- DO use `log/slog` for structured logging (Go 1.21+, https://go.dev/blog/slog). Replaces `logrus`, `zap` in most projects unless you need extreme low-allocation logging.
- DO use `net/http.ServeMux` with method+wildcard patterns (Go 1.22+, https://go.dev/blog/routing-enhancements). Replaces `gorilla/mux` (deprecated Dec 2022) and is enough for most APIs.
  ```go
  mux := http.NewServeMux()
  mux.HandleFunc("GET /api/v1/sets/{name}", getSet)
  mux.HandleFunc("DELETE /api/v1/sets/{name}", deleteSet)
  // ...later: name := r.PathValue("name")
  ```
  Why: the mux returns 405 for wrong-method matches automatically (Go 1.22 release notes).
- DO use `errors.Join` for grouped errors (Go 1.20+, https://pkg.go.dev/errors#Join). Replaces `multierror.Append`.
- DO use `slices`, `maps`, `cmp` for collection helpers (Go 1.21+). Replaces hand-rolled or `samber/lo` helpers for sort, search, contains, min, max, clone.
  In this repo, prefer `slices.Sort`, `slices.IsSorted`, and
  `slices.SortStableFunc` for simple ordered or stable sort cases. Do not
  blanket-convert complex `sort.Slice` comparators; review descending order,
  tie-breaks, stability, and domain-specific comparison logic first.
- DO use `sync.OnceFunc`, `sync.OnceValue`, `sync.OnceValues` for lazy init (Go 1.21+, https://victoriametrics.com/blog/go-sync-once/). Cleaner than `var once sync.Once; var x T` patterns.
  ```go
  var loadConfig = sync.OnceValues(func() (*Config, error) { return parse(path) })
  ```
- DO use `iter.Seq`/`iter.Seq2` for custom iteration (Go 1.23+, https://go.dev/blog/range-functions). The compiler inlines them; performance is comparable to hand-written loops (https://go.dev/blog/range-functions).
- DO use `unique.Make[T]` to intern repeated comparable values (Go 1.23+, https://go.dev/blog/unique). Pointer-cheap equality, GC reclaims unreferenced canonical copies.
- DO use `testing/synctest` for deterministic concurrency tests (Go 1.25 GA, https://go.dev/blog/synctest). Bubbles virtualize time; flaky timing tests stop being flaky.
- DO use `os.Root` for filesystem operations confined to a directory (Go 1.24+, https://go.dev/doc/go1.24). Path-traversal-safe by construction.
- DO use `weak.Pointer` for cache values that should not pin objects (Go 1.24+, https://go.dev/blog/cleanups-and-weak). Replaces hand-rolled finalizer dances.
- DO use `WaitGroup.Go` (Go 1.25+, https://go.dev/doc/go1.25). Removes the `wg.Add(1) ... defer wg.Done()` boilerplate.
- DO use `any` not `interface{}` (Go 1.18+, https://pkg.go.dev/builtin#any). Identical type, more readable.

DON'T:

- DON'T import `io/ioutil` (deprecated Go 1.16, https://pkg.go.dev/io/ioutil). Use `os.ReadFile`, `os.WriteFile`, `io.ReadAll`, `os.MkdirTemp`. LLM-generated Go almost always still uses `ioutil`.
- DON'T add `gorilla/mux`, `gin`, `chi`, `echo` reflexively. Reach for them only when stdlib `ServeMux` is provably insufficient (subrouter groups with common middleware, regex routes, embedded route metadata). Most update-ipsets-style APIs do not need a third-party router.
- DON'T add `logrus`, `zap` reflexively when `log/slog` is enough. `slog` has JSON and text handlers, attribute groups, context-aware logging, and a stable API.
- DON'T import a package just for `Min`/`Max`/`Contains` — `slices`, `cmp` cover those.

## 2. Clean code & modularity

- DO put transport (HTTP handlers), domain (business logic), and storage (cache, files, DB) in separate packages. Handlers translate HTTP to function calls; domain functions take primitive/domain types and return primitive/domain types; storage hides files/cache details.
- DO accept interfaces, return concrete types ("accept interfaces, return structs", https://medium.com/@cep21/what-accept-interfaces-return-structs-means-in-go-2fe879e25ee8). Define the interface where it is consumed, not where it is implemented; this avoids "interface pollution" and inverted dependencies.
  ```go
  // GOOD: interface defined at the consumer
  package engine
  type FeedReader interface { Read(name string) (Feed, error) }
  func (e *Engine) Process(r FeedReader) error { ... }

  // GOOD: concrete return type
  package cache
  func New(dir string) *Cache { ... } // not: func New(dir string) Cacher
  ```
- DO keep interfaces narrow. One or two methods is the norm in Go (`io.Reader`, `io.Writer`, `fmt.Stringer`). If an interface has 8 methods, it is probably wrong.
- DO keep functions small enough to read on one screen. If a function exceeds ~80 lines or 3 levels of nesting, split it. Project posture is enforced by `tools/archposture` (see `project-coding`).
- DO write single-purpose functions: a function name that says "and" ("validateAndPersist") is a fork in the road. Split it.
- DO keep files small. ~500 lines of Go is a soft ceiling; beyond that the file is hiding multiple responsibilities. The project enforces this via `tools/archposture`.
- DO extract a package only when the new package has a name that says what it provides, not what it contains (https://go.dev/blog/package-names).

DON'T:

- DON'T create packages named `util`, `utils`, `common`, `helpers`, `base`, `misc`, `shared`, `lib`, `core` (https://dave.cheney.net/2019/01/08/avoid-package-names-like-base-util-or-common). They are import-cycle escape hatches that grow into god-packages. The fix is almost always to move the helper to the calling package or split it into a package named after what it provides (e.g., `httpheader`, `cachekey`, `iprange`).
- DON'T add a struct for every concept. Free functions on plain types are normal Go. Avoid Java-style "Manager"/"Service"/"Handler"/"Factory" objects unless they have real state.
- DON'T add getters/setters on every struct field. Exported fields are fine when invariants are not required.
- DON'T add abstractions before two callers exist. "Just in case" interfaces are the most common over-engineering smell in Go (https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- DON'T put `init()` functions with side effects (network, files, env mutation). They make tests fragile and dependency order unclear. Expose explicit constructors.
- DON'T use global mutable state. Pass configuration and dependencies explicitly through constructors.

## 3. Errors

- DO wrap errors with `%w` to preserve the chain (Go 1.13+, https://go.dev/blog/go1.13-errors).
  ```go
  if err := download(ctx, url); err != nil {
      return fmt.Errorf("download %s: %w", name, err)
  }
  ```
- DO use `errors.Is` for sentinel comparisons and `errors.As` for typed extraction (https://pkg.go.dev/errors).
  ```go
  if errors.Is(err, context.Canceled) { return nil }
  var netErr *net.OpError
  if errors.As(err, &netErr) { /* inspect netErr.Op */ }
  ```
- DO use `errors.Join` to combine errors from concurrent or batched operations (Go 1.20+).
- DO use `errors.New(message)` instead of `fmt.Errorf("%s", message)` when no
  formatting or wrapping is needed.
- DO start error messages lowercase, no trailing punctuation, no `failed to` prefix. The wrapping caller adds context. Convention: `"open %s: %w"`, not `"Failed to open file: %s"`.
- DO return errors; the caller decides whether to log. Logging and returning is double-reporting.
- DO wrap recovers in HTTP middleware so a single panicking handler does not crash the server (https://medium.com/@iarsham/dont-let-panics-crash-your-go-application-mastering-recoveries-in-middleware-9e1cf657987f). Recover only the same goroutine; spawned workers need their own recovery.

DON'T:

- DON'T swallow errors with `_`. If you intentionally ignore, comment why: `_ = closer.Close() // best effort`.
- DON'T use `errors.New("...")` for dynamic strings; use `fmt.Errorf` so wrapping with `%w` works.
- DON'T panic across package boundaries for control flow. Panic is for unrecoverable invariants. Library code returns errors.
- DON'T use bare `panic(err)` in HTTP handlers. Recover middleware exists, but make panics rare and surprising.
- DON'T compare errors by `==` after wrapping. Use `errors.Is`. Direct `==` only matches unwrapped sentinels.

## 4. Concurrency

- DO accept `ctx context.Context` as the first parameter on functions that block, do I/O, or spawn goroutines (https://pkg.go.dev/context).
- DO propagate `ctx` into every blocking call you make (`http.NewRequestWithContext`, `db.QueryContext`, `time.NewTimer` + `select`, etc.).
- DO use `errgroup.WithContext` for fan-out work that can fail (https://pkg.go.dev/golang.org/x/sync/errgroup). It cancels the shared context on first error and waits for all goroutines.
  ```go
  g, ctx := errgroup.WithContext(ctx)
  for _, src := range sources {
      src := src
      g.Go(func() error { return process(ctx, src) })
  }
  if err := g.Wait(); err != nil { return err }
  ```
- DO use `singleflight` for deduplicating concurrent identical requests (https://pkg.go.dev/golang.org/x/sync/singleflight). Standard cache-stampede defense.
- DO use `golang.org/x/time/rate.Limiter` for token-bucket rate limiting on outbound requests (https://pkg.go.dev/golang.org/x/time/rate).
- DO close channels from the sender side when no more values will be sent. Receivers detect close via the second return value (`v, ok := <-ch`) or `for range ch`.
- DO test for goroutine leaks. `go.uber.org/goleak.VerifyTestMain` in tests catches goroutines that outlive the test.
- DO use `sync.WaitGroup.Go` for goroutines already owned by the same
  `WaitGroup` when the function has no returned error and the existing
  ownership/cancellation model is clear. Do not blanket-convert stress tests,
  lifecycle code, or goroutines that need package-specific panic/error
  handling review.

DON'T:

- DON'T store `context.Context` in a struct; pass it explicitly per call (https://go.dev/blog/context-and-structs). Exception: long-lived servers may store a base ctx for shutdown, but pass per-request ctx through call paths.
- DON'T pass `context.Context` by pointer (`*context.Context`). It is an interface, already pointer-sized; pass by value. (https://medium.com/@christianotieno/stop-passing-context-context-in-go-why-value-is-the-way-c593c405f81f)
- DON'T put dependencies (DB handle, logger, config) in `context.WithValue`. That bypasses the type system. `context.WithValue` is for request-scoped metadata only (request ID, trace span, user identity).
- DON'T launch goroutines without a way to cancel/observe them. The pattern `go someFunc()` with no `ctx` and no `wg` is a leak waiting to happen (https://rednafi.com/go/early-return-and-goroutine-leak/).
- DON'T return early from a function that has spawned goroutines writing to unbuffered channels — the writers block forever. Drain the channels or use buffered/contextful channels (https://rednafi.com/go/early-return-and-goroutine-leak/).
- DON'T use a buffered channel as a single-goroutine queue; if there is no receiver, you deadlock as soon as the buffer fills.
- DON'T `recover()` in normal flow. Recover is for crash containment in HTTP handlers and in package-internal goroutines whose panic must not kill the process.
- DON'T sprinkle `time.Sleep` in tests to wait for goroutines. Use `testing/synctest.Test` (Go 1.25 GA, https://go.dev/blog/synctest) or explicit synchronization channels.

## 5. High-performance APIs

These are the patterns that matter on hot paths. Apply them where measurement justifies it; do not pre-optimize.

- DO configure server timeouts. A `&http.Server{}` with no timeouts can be DoS'd by slow-loris clients (https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/).
  ```go
  srv := &http.Server{
      Addr:              addr,
      Handler:           h,
      ReadHeaderTimeout: 5 * time.Second,
      ReadTimeout:       15 * time.Second,
      WriteTimeout:      30 * time.Second,
      IdleTimeout:       60 * time.Second,
      MaxHeaderBytes:    1 << 20,
  }
  ```
- DO implement graceful shutdown via SIGINT/SIGTERM and `srv.Shutdown(ctx)` (https://dev.to/yanev/a-deep-dive-into-graceful-shutdown-in-go-484a). Drain in-flight requests before exit.
- DO serve published artifacts with `http.ServeContent` or `http.ServeFile` so `If-None-Match`, `If-Modified-Since`, and `Range` are handled for free (https://pkg.go.dev/net/http#ServeContent). Set `ETag` per RFC 7232 §2.3 (quoted) before calling `ServeContent`.
- DO stream large responses (`io.Copy`, `bufio.Writer`) instead of buffering whole bodies into memory. For files, this enables `sendfile(2)` zero-copy on Linux when the destination is a TCP socket.
- DO reuse buffers via `sync.Pool` for short-lived per-request scratch space (https://wundergraph.com/blog/golang-sync-pool). Always `Reset()` on get or before put. Never retain a reference after `Put`.
  ```go
  var bufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}
  func handle(w http.ResponseWriter, r *http.Request) {
      buf := bufPool.Get().(*bytes.Buffer)
      buf.Reset()
      defer bufPool.Put(buf)
      // ...write to buf...
  }
  ```
- DO use `strings.Builder` for hot-path string concatenation; never `s += s2` in a loop.
- DO use `strings.Builder` only for text construction where the final contract
  is a string. Keep `bytes.Buffer` for binary data, gzip/tar payloads,
  tests that need byte slices, or writers whose result is intentionally
  consumed as `[]byte`.
- DO pre-size slices and maps when the capacity is known (`make([]T, 0, n)`, `make(map[K]V, n)`). Saves growth-copy churn.
- DO check escape analysis with `go build -gcflags='-m' ./...` when chasing allocations (https://goperf.dev/01-common-patterns/stack-alloc/). A surprising heap escape often comes from passing a value to `interface{}` (`any`) or capturing it in a closure.
- DO benchmark with `testing.B.Loop` (Go 1.24+, https://go.dev/doc/go1.24). It removes timer-management boilerplate and is more accurate than `b.N`-based loops.
- DO honor `GOMEMLIMIT` for containerized deployments (https://weaviate.io/blog/gomemlimit-a-game-changer-for-high-memory-applications). Set it ~10-20% below the cgroup hard limit so the GC becomes aggressive before OOM kill.

DON'T:

- DON'T reach for `valyala/fasthttp` reflexively. It is faster than `net/http` but has different semantics (no per-request allocations means handlers cannot retain pointers into the request after return). Most APIs do not need it; `net/http` plus pooling is enough (https://github.com/valyala/fasthttp).
- DON'T read whole large files into memory with `os.ReadFile` for serving. Stream them.
- DON'T pool objects that hold large buffers. `sync.Pool` victimization in GC can keep them pinned across cycles, defeating the win. Pool small reusable objects only.
- DON'T pool objects across goroutine ownership boundaries. The contract is: get -> use -> put. Never share a pooled object between goroutines or store one in a long-lived struct.
- DON'T use `interface{}`/`any` in hot loops if the concrete type is known. Conversion to `any` boxes the value to the heap.
- DON'T trigger upstream downloads from public read paths. Cache-first serving is a project rule (see `project-coding`); reads serve published artifacts and return missing rather than computing on demand.
- DON'T enable gzip middleware blindly. It defeats `sendfile`, costs CPU, and breaks `Content-Length`. Negotiate per-route based on content type and size; pre-compress static assets when feasible.
- DON'T expose `net/http/pprof` on a public listener. Bind it to a separate internal listener or gate it behind authentication (https://go.dev/doc/diagnostics).

## 6. Memory & GC

- DO know that values "escape" to the heap when their address outlives the function frame (returned, stored in a global, captured by a goroutine, stored in an interface). Stack allocation is one instruction; heap allocation involves the allocator and the GC (https://goperf.dev/01-common-patterns/stack-alloc/).
- DO order struct fields largest-to-smallest to minimize padding (https://goperf.dev/01-common-patterns/fields-alignment/). The `fieldalignment` linter (in `golang.org/x/tools/go/analysis/passes/fieldalignment`) flags wastage.
- DO prefer `[]T` over `[]*T` when `T` is small and pointer indirection is not needed. Saves allocations and improves cache behavior.
- DO use `unique.Make` for deduplicating millions of repeated comparable values (https://go.dev/blog/unique).
- DO use `weak.Pointer` for caches that must not pin entries (https://go.dev/blog/cleanups-and-weak).
- DO profile before optimizing. Workflow:
  ```bash
  go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof ./pkg/...
  go tool pprof -http=:8080 cpu.prof
  go tool pprof -alloc_objects mem.prof   # allocation rate
  go tool pprof -inuse_space mem.prof     # live heap shape
  ```
- DO set `GOMEMLIMIT` (or `runtime/debug.SetMemoryLimit`) in containers (https://go.dev/doc/gc-guide). Combined with `GOGC=off` it gives a predictable memory ceiling at the cost of more CPU when full.
- DO add false-sharing padding for hot atomic fields accessed by different goroutines (cache line is 64 bytes on x86_64).

DON'T:

- DON'T disable the GC (`GOGC=off`) without `GOMEMLIMIT`. You will OOM under sustained allocation.
- DON'T retain references to whole files just to serve them later. Memory-map (with `syscall.Mmap` or `golang.org/x/exp/mmap`) when you need random access without heap growth.
- DON'T use `sync.Pool` for things that "look like a pool" (DB connection pools, worker pools). `sync.Pool` is a temp-object cache with GC-driven eviction — it is not a resource pool.

## 7. Project structure

The "Standard Go Project Layout" repo (https://github.com/golang-standards/project-layout) is community-maintained and explicitly NOT an official standard (https://github.com/golang-standards/project-layout/issues/117). The official guide is https://go.dev/doc/modules/layout.

What is broadly accepted:

- `cmd/<binary>/main.go` for binary entrypoints (one binary per subdirectory).
- `internal/` for packages that must not be imported by other modules. The Go toolchain enforces this.
- `pkg/` is **debated**: many maintainers (Cheney, the official Go modules guide) discourage it for application repos because it adds depth without payoff. This project uses `pkg/` because the codebase is large and the boundary between `pkg/` and `internal/` is real (`pkg/iprange` is an explicit standalone library; `internal/` holds private helpers). Mirror the existing layout; do not re-litigate.

For this project specifically:

- DO put binary entrypoints under `cmd/update-ipsets/`.
- DO put private helpers under `internal/` (observability, file utilities).
- DO put major Go packages under `pkg/` (`config`, `downloader`, `engine`, `scheduler`, `web`, `iprange`, `geoloc`, `asnloc`, `processor`).
- DO keep `pkg/iprange` standalone — no imports from other project packages (project rule).
- DO put nested helper modules under `tools/` (e.g., `tools/dronebl2ipsets/`).
- DO put YAML catalog under `configs/`.
- DO put React source under `ui/src/`; do not edit `pkg/web/static/assets/*` or generated `pkg/web/static/index.html`.

DON'T:

- DON'T create `pkg/util`, `pkg/common`, `pkg/helpers`, `pkg/shared`, `pkg/lib`, `pkg/core` (https://dave.cheney.net/2019/01/08/avoid-package-names-like-base-util-or-common).
- DON'T create deeply nested package trees just to mirror the OOP class hierarchy of another language.
- DON'T introduce a new top-level directory (`api/`, `services/`, `domain/`, `infrastructure/`) without an explicit SOW. Mirror existing structure.
- DON'T put exported packages under `internal/` "for now". Once you do, external imports break and the boundary is hard to reverse.

## 8. Tooling & CI gates

- DO run `gofmt -s -w` (or `goimports`). Project uses standard `gofmt` — this is enforced by reviewers, not a CI gate per se.
- DO run `go vet ./...`. Catches common mistakes including the deferred-call-argument bug (`defer log.Duration(time.Since(start))` evaluates `time.Since` immediately).
- DO run `staticcheck` (https://staticcheck.dev/docs/) — gold-standard analyzer with 150+ checks. In golangci-lint v2 (https://golangci-lint.run/docs/product/changelog/), `staticcheck`, `stylecheck`, `gosimple` are merged into one linter.
- DO run `go test -race ./...` for race-detector coverage on every CI build (https://go.dev/doc/articles/race_detector). Race-free in tests does not prove race-free in production, but races in tests definitely mean races in production.
- DO run `govulncheck ./...` (https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) in CI. It checks reachable vulnerable code paths, not just dependency versions.
- DO write fuzz tests (`func FuzzX(f *testing.F)`) for parsers, deserializers, and any function that consumes untrusted input (https://go.dev/doc/security/fuzz/). Add seeds with `f.Add(...)`.
- DO use `go test -count=1` to bypass test caching when you genuinely want a re-run.
- DO check architecture posture with `go test ./tools/archposture` for changes touching `pkg/engine`, `pkg/web`, `pkg/scheduler`, `pkg/cache`, route registration, or large UI components (project rule from `project-reviewing`).

DON'T:

- DON'T disable `go vet` checks file-by-file with `//go:build` tricks. Fix the issue.
- DON'T silence `staticcheck` findings without a comment explaining why. The default config is conservative; if it complains, it usually has a point.
- DON'T commit `t.Skip(...)` to silence a flaky test. Either fix the race/timing bug or use `testing/synctest`.

## 9. Testing

- DO use table-driven tests with subtests:
  ```go
  for _, tc := range cases {
      t.Run(tc.name, func(t *testing.T) {
          got, err := fn(tc.in)
          if err != nil { t.Fatalf("fn(%q): %v", tc.in, err) }
          if got != tc.want { t.Errorf("got %v want %v", got, tc.want) }
      })
  }
  ```
- DO use `t.TempDir()` and `t.Cleanup` instead of `os.MkdirTemp` + manual cleanup.
- DO use `httptest.NewServer` for integration tests against your own handlers; test the real HTTP contract.
- DO use `testing/synctest` (Go 1.25+) for time-dependent concurrency tests (https://go.dev/blog/synctest).
- DO use the `goleak` library in `TestMain` to fail tests that leak goroutines (https://pkg.go.dev/go.uber.org/goleak).

DON'T:

- DON'T mock the standard library. Use `httptest`, real files in `t.TempDir()`, fake clocks (or `synctest`).
- DON'T introduce a mocking framework (`gomock`, `mockery`) unless interfaces are already defined for non-test reasons. Hand-written test doubles in `_test.go` are usually clearer.
- DON'T rely on `time.Sleep` to "let the goroutine finish". Synchronize on a channel or use `synctest`.

## 10. Working with AI-generated Go (mandatory review section)

LLMs trained on years of Go source produce confident-looking output that frequently misses recent stdlib additions and idioms. Treat AI-generated Go as suspect by default. The community signal is real: r/golang now removes "vibe-coded" project posts because the volume swamped genuine work (https://www.clientserver.dev/p/rgolang-draws-a-line-on-ai-generated, 2025-10), and an AppSignal/InfoWorld report found AI-generated PRs contain ~1.7x more issues than human PRs (https://www.infoworld.com/article/4109129/ai-assisted-coding-creates-more-problems-report.html, 2025).

### Failure modes to grep for

- **Deprecated `io/ioutil`** — replace with `io`/`os` (https://pkg.go.dev/io/ioutil deprecated since Go 1.16).
  ```bash
  grep -rn "ioutil\." --include='*.go' .
  ```
- **`gorilla/mux` and friends** when stdlib `ServeMux` would do (gorilla/mux deprecated Dec 2022, https://github.com/gorilla/mux). Verify the route patterns; method+wildcard `ServeMux` is sufficient since Go 1.22.
- **`interface{}` instead of `any`** — purely cosmetic but a tell that the model trained on pre-1.18 corpus.
- **Goroutines without `ctx` or `wg`** — `go someFunc()` with no cancellation path is a leak.
  ```bash
  grep -rn '^[[:space:]]*go ' --include='*.go' .
  ```
  Each hit deserves a "where does this stop?" check.
- **Channels never closed and ranged over** — `for v := range ch` plus a sender that never closes blocks forever.
- **Errors swallowed with `_`** — `_ = json.Unmarshal(...)`. Often LLMs do this to make examples compile cleanly.
- **`fmt.Errorf("%v", err)` wrapping** — should be `%w` to preserve the chain.
- **`log.Println(err); return err`** — double-reporting. Either log or return.
- **`init()` with side effects** — DB connections, file reads, env mutation in `init()`.
- **Packages named `utils`, `helpers`, `common`, `shared`, `core`** — see section 2.
- **Manager/Service/Factory/Handler god-structs** with no real state — Java idioms ported to Go.
- **`time.Sleep` in production code or tests as synchronization** — almost always wrong.
- **`panic(err)` in library code** — return the error.
- **`gorilla/handlers`, `negroni` for middleware** — stdlib handler-wrapping pattern is enough.
- **`logrus`, `zap` newly added** — if `log/slog` already covers it, don't add a second logger.
- **`context.WithValue` carrying loggers, DB handles, configs** — type-system bypass.
- **`*context.Context` parameter type** — should be `context.Context` (interface, by value).
- **Vendoring whole libraries** — Go modules + proxy is the default since Go 1.16.
- **OOP factories**: `NewXFactory().CreateX(...)` chains. Just write `NewX(...)`.
- **Reflection used to avoid generics** — if the version is Go 1.18+ and the use case is type-parametric, generics are the right tool.
- **Fake "production-ready" claims** — multi-thousand-line components in a single commit are a smell (r/golang documented examples include "production ready" loggers with memory leaks and 7000-line message queues in single commits, https://www.clientserver.dev/p/rgolang-draws-a-line-on-ai-generated).

### Checklist (run before merging AI-assisted Go)

1. `gofmt -l ./...` empty? `go vet ./...` clean? `staticcheck ./...` clean? `go test -race ./...` passing?
2. `grep -rn "ioutil\." --include='*.go' .` empty?
3. `grep -rn "interface{}" --include='*.go' .` either empty or all in code that predates the change?
4. Every `go someFunc(...)` has a clear cancellation path (ctx + select on `<-ctx.Done()`, or `errgroup`).
5. Every channel send-only or close-only has documented ownership.
6. No `_ =` on error returns without an explanatory comment.
7. No package named `utils`/`common`/`helpers`/`shared`/`core`/`misc`/`base`/`lib`.
8. No `context.WithValue` carrying anything except request-scoped metadata (request ID, span, user identity).
9. HTTP server has `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `ReadHeaderTimeout`, and a graceful shutdown path.
10. No `time.Sleep` in tests; synchronization is explicit or via `testing/synctest`.
11. Every panic recover is in a defer, in the same goroutine that may panic, and logs the panic before recovering.
12. Generated files are minimal and the producer/refresh/repair/serving paths are evidence-cited (project rule).
13. New dependencies justified: is there a stdlib equivalent that already works?
14. Function and file sizes within project posture limits (`tools/archposture`).

### Why AI-generated Go fails specifically

- LLMs trained pre-2024 do not know about `slog`, `iter.Seq`, `unique`, `weak`, `synctest`, `errors.Join`, `sync.OnceFunc`, ServeMux 1.22 patterns, `cmp.Or`, `slices.Concat`.
- LLMs latch onto the most-represented training data, which is older Go (1.13-1.18 idioms). They mass-produce `interface{}`, `ioutil`, `gorilla/mux`, `logrus`.
- LLMs invent plausible-sounding APIs (e.g., `slices.Filter` did not exist as of Go 1.25 — only `slices.DeleteFunc`). Verified compile + `go vet` is the floor; running tests is the ceiling (https://simonwillison.net/2025/Mar/2/hallucinations-in-code/).
- LLMs over-abstract: factories, registries, dependency-injection containers, where Go idiom is plain functions and explicit constructors.
- LLMs under-handle context cancellation in concurrent code; they will spawn goroutines that read from network or files without `ctx`. (https://news.ycombinator.com/item?id=45405869, 2025)
- LLMs frequently produce middleware chains with `recover()` in a goroutine — `recover` only catches panics in the same goroutine.

If you must use AI-generated Go, paste 30-50 lines of nearby project code into the prompt as exemplars. Models imitate strongly from few-shot examples (https://simonwillison.net/2025/Mar/2/hallucinations-in-code/).

## 11. Quick-reference DON'T list

- DON'T `import "io/ioutil"` — deprecated since Go 1.16.
- DON'T add `gorilla/mux` reflexively — stdlib `ServeMux` is enough since Go 1.22.
- DON'T pass `*context.Context` — pass `context.Context` by value.
- DON'T store `context.Context` in a struct.
- DON'T put dependencies in `context.WithValue`.
- DON'T `go someFunc()` without a cancellation path.
- DON'T `recover()` outside a `defer` or in a different goroutine than the panic.
- DON'T return early while holding goroutines that write to unbuffered channels.
- DON'T name packages `util`/`common`/`helpers`/`shared`/`core`/`misc`/`base`/`lib`.
- DON'T add `init()` with side effects.
- DON'T use global mutable state.
- DON'T `panic(err)` in library code.
- DON'T `log.Println(err); return err` — pick one.
- DON'T compare wrapped errors with `==` — use `errors.Is`.
- DON'T `time.Sleep` for synchronization in tests.
- DON'T mock the standard library.
- DON'T pool large or long-lived objects in `sync.Pool`.
- DON'T `interface{}` in hot loops — boxing escapes to heap.
- DON'T expose `net/http/pprof` publicly.
- DON'T disable GC without `GOMEMLIMIT`.
- DON'T introduce gzip middleware blindly — measure, negotiate per route.
- DON'T add `logrus`/`zap` when `log/slog` already covers the use case.
- DON'T edit generated frontend assets (`pkg/web/static/assets/*`, generated `pkg/web/static/index.html`).
- DON'T let `pkg/iprange` import other project packages.
- DON'T trigger upstream downloads from public read paths (project rule).
- DON'T put expensive historical rescans on daemon startup critical path (project rule).
- DON'T derive semantic meaning from feed/provider/artifact name substrings (project rule).

## 12. Sources

### Go release notes & official docs

- Go 1.22 release notes (https://go.dev/doc/go1.22) — ServeMux method+wildcard routing.
- Go 1.23 release notes (https://go.dev/doc/go1.23) — range-over-func, `iter`, `unique`.
- Go 1.24 release notes (https://go.dev/doc/go1.24) — Swiss Tables maps, `weak`, `os.Root`, `testing.B.Loop`, generic type aliases.
- Go 1.25 release notes (https://go.dev/doc/go1.25) — `testing/synctest` GA, `WaitGroup.Go`, container-aware `GOMAXPROCS`, `encoding/json/v2` experimental.
- Go 1.26 release notes (https://go.dev/doc/go1.26) — Green Tea GC default, `crypto/hpke`, `io.ReadAll` ~2x faster, `errors.AsType`, generic self-referential constraints, `new()` accepts expressions.
- Organizing a Go module (https://go.dev/doc/modules/layout) — official layout guide.
- A Guide to the Go Garbage Collector (https://go.dev/doc/gc-guide).
- Go diagnostics (https://go.dev/doc/diagnostics) — pprof, trace, profiling.
- Working with Errors in Go 1.13 (https://go.dev/blog/go1.13-errors).
- Structured Logging with slog (https://go.dev/blog/slog).
- Routing Enhancements for Go 1.22 (https://go.dev/blog/routing-enhancements).
- Range Over Function Types (https://go.dev/blog/range-functions).
- New unique package (https://go.dev/blog/unique).
- From unique to cleanups and weak (https://go.dev/blog/cleanups-and-weak).
- Faster Go maps with Swiss Tables (https://go.dev/blog/swisstable).
- Testing concurrent code with testing/synctest (https://go.dev/blog/synctest).
- Contexts and structs (https://go.dev/blog/context-and-structs).
- Go Wiki: Rate Limiting (https://go.dev/wiki/RateLimiting).

### Idioms & style

- Dave Cheney, "Avoid package names like base, util, or common" (https://dave.cheney.net/2019/01/08/avoid-package-names-like-base-util-or-common).
- Dave Cheney, "Practical Go" (https://dave.cheney.net/practical-go/presentations/qcon-china.html).
- Jack Lindamood, "What 'accept interfaces, return structs' means in Go" (https://medium.com/@cep21/what-accept-interfaces-return-structs-means-in-go-2fe879e25ee8).
- Eli Bendersky, "Better HTTP server routing in Go 1.22" (https://eli.thegreenplace.net/2023/better-http-server-routing-in-go-122/).
- Eli Bendersky, "Ranging over functions in Go 1.23" (https://eli.thegreenplace.net/2024/ranging-over-functions-in-go-123/).
- Stop passing *context.Context (https://medium.com/@christianotieno/stop-passing-context-context-in-go-why-value-is-the-way-c593c405f81f).
- Project layout debate (https://github.com/golang-standards/project-layout/issues/117).

### Performance & memory

- Go Optimization Guide — Object Pooling (https://goperf.dev/01-common-patterns/object-pooling/).
- Go Optimization Guide — Stack Allocations (https://goperf.dev/01-common-patterns/stack-alloc/).
- Go Optimization Guide — Struct Field Alignment (https://goperf.dev/01-common-patterns/fields-alignment/).
- Go Optimization Guide — GC (https://goperf.dev/01-common-patterns/gc/).
- Weaviate, "GOMEMLIMIT is a game changer" (https://weaviate.io/blog/gomemlimit-a-game-changer-for-high-memory-applications).
- VictoriaMetrics, "Go sync.Once is Simple..." (https://victoriametrics.com/blog/go-sync-once/).
- VictoriaMetrics, "Weak Pointers in Go" (https://victoriametrics.com/blog/go-weak-pointer/).
- VictoriaMetrics, "Go synctest: Solving Flaky Tests" (https://victoriametrics.com/blog/go-synctest/).
- WunderGraph, "Golang Sync.Pool | Memory Pool Example" (https://wundergraph.com/blog/golang-sync-pool).
- DataDog go-profiler-notes (https://github.com/DataDog/go-profiler-notes/blob/main/guide/README.md).

### Concurrency

- redowan, "Early return and goroutine leak" (https://rednafi.com/go/early-return-and-goroutine-leak/).
- errgroup docs (https://pkg.go.dev/golang.org/x/sync/errgroup).
- singleflight docs (https://pkg.go.dev/golang.org/x/sync/singleflight).
- rate docs (https://pkg.go.dev/golang.org/x/time/rate).
- goleak (https://pkg.go.dev/go.uber.org/goleak).

### HTTP & APIs

- Cloudflare, "The complete guide to Go net/http timeouts" (https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/).
- "A Deep Dive into Graceful Shutdown in Go" (https://dev.to/yanev/a-deep-dive-into-graceful-shutdown-in-go-484a).
- Alex Edwards, "Making and Using HTTP Middleware in Go" (https://www.alexedwards.net/blog/making-and-using-middleware).
- "How to Rate Limit HTTP Requests in Go" (https://www.alexedwards.net/blog/how-to-rate-limit-http-requests).
- ETag and HTTP caching (https://rednafi.com/misc/etag-and-http-caching/).

### AI-generated code & community signals

- Client/Server, "/r/golang draws a line on AI-generated projects" (https://www.clientserver.dev/p/rgolang-draws-a-line-on-ai-generated, 2025-10).
- HN discussion, "Go experts: 'I don't want to maintain AI-generated code'" (https://news.ycombinator.com/item?id=45405869, 2025).
- HN discussion, "Are developers trusting AI-generated code too much?" (https://news.ycombinator.com/item?id=47425058, 2025).
- Simon Willison, "Hallucinations in code are the least dangerous form of LLM mistakes" (https://simonwillison.net/2025/Mar/2/hallucinations-in-code/, 2025-03).
- Honeycomb, "How I Code With LLMs These Days" (https://www.honeycomb.io/blog/how-i-code-with-llms-these-days).
- Addy Osmani, "Comprehension Debt — the hidden cost of AI generated code" (https://addyosmani.com/blog/comprehension-debt/).
- InfoWorld, "AI-assisted coding creates more problems – report" (https://www.infoworld.com/article/4109129/ai-assisted-coding-creates-more-problems-report.html, 2025).
- Arxiv, "Exploring and Evaluating Hallucinations in LLM-Powered Code Generation" (https://arxiv.org/html/2404.00971v1, 2024).

### Tooling

- Staticcheck docs (https://staticcheck.dev/docs/).
- golangci-lint changelog v2 (https://golangci-lint.run/docs/product/changelog/).
- govulncheck (https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck).
- Go Fuzzing (https://go.dev/doc/security/fuzz/).
- Go race detector (https://go.dev/doc/articles/race_detector).
