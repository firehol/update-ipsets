---
name: project-go-behavioral-testing
description: "Black-box behavioral testing for Go: test public contracts and observable behavior, not internals. Modern stdlib testing patterns, anti-mock guidance, and AI-generated-test pitfalls. Use when writing or reviewing Go tests."
---

## TL;DR

- Treat the unit under test as a black box. Drive it through its public surface; assert on observable outputs and side effects only.
- Default test package is `pkg_test` (external). Same-package tests are reserved for genuinely package-internal helpers, and never used as a back door into private state.
- Stdlib `testing` is the bar: table-driven subtests, `t.TempDir`, `t.Cleanup`, `t.Context`, `t.Setenv`, `httptest`, fuzzing, golden files. No `testify`, no `gomock`, no `ginkgo`.
- Mocks are last resort. Prefer real implementations and small handwritten fakes over generated mocks. A mock asserts on calls; a fake provides behavior. Prefer fakes.
- LLM-generated tests fail in stereotyped ways (over-mocking, `assert.Nil(err)` as the only assertion, happy-path coverage padding, `time.Sleep` for synchronization, missing edge cases). Review every AI-generated test against the checklist in this skill before merging.

## Why this skill exists

This codebase is a Go 1.26 service with HTTP APIs, a background pipeline, an embedded React UI, integrity invariants tied to file mtimes, and bounded-memory contracts (see `.agents/sow/specs/`). Tests that lock implementation details have repeatedly forced spec-correct refactors to be reverted because "the test broke", and AI-generated tests are now a regular review item.

The hard rule for this repo: **a test must fail when the externally observable contract is broken, and must not fail when only internals change.** Everything below serves that rule.

This skill is layered on top of `project-testing/SKILL.md` (commands, fixtures, validation gates) and `project-coding/SKILL.md` (Go conventions). Read those first.

## 1. The contract is the test

The "contract" of a unit is everything an external caller is entitled to rely on. The test must drive the unit through its contract and assert only on contract-visible outcomes.

| Unit kind | Public surface = contract |
|-----------|--------------------------|
| Exported function | Arguments and returned values; documented errors; observable side effects on arguments (slices, maps, files, contexts) |
| Package | The set of exported identifiers and their documented semantics |
| HTTP handler / server | Method, path, request body, headers; response status, headers, body bytes; side effects on the configured filesystem/cache/queue |
| Background worker / pipeline | Inputs it consumes (config, queue items, files), outputs it produces (artifacts, metrics, logs that are part of the contract), state transitions visible via admin API |
| CLI subcommand | argv, stdin, env; exit code, stdout, stderr, files written |
| Process (binary) | All of the above plus signals, lifecycle hooks |

Things that are NOT part of the contract and must NOT appear in assertions:

- Internal helper function names, signatures, or call counts
- Private struct fields, even if reachable through `reflect`
- The order of internal events that the contract does not specify
- Log line wording, unless logs are part of the user/operator contract (in this repo, structured operator logs from `slog` are observable but their exact phrasing is not; only fields with documented meaning are testable)
- The exact set of goroutines or their identities (only externally visible behavior such as "no leak after `Close`" is testable)

When you cannot describe the contract in one paragraph, the unit is too entangled to test behaviorally; that is a design problem to surface in the SOW, not a justification for testing internals.

References: Ian Cooper, "TDD: Where Did It All Go Wrong" (InfoQ presentation, original talk 2017, [infoq.com/presentations/tdd-original](https://www.infoq.com/presentations/tdd-original/)); Dave Cheney, "Practical Go" external test packages ([dave.cheney.net/practical-go](https://dave.cheney.net/practical-go/presentations/gophercon-singapore-2019.html)).

## 2. Where the test file lives

Default placement for every new test in this repo:

```
pkg/foo/foo.go            // package foo
pkg/foo/foo_test.go       // package foo_test    <-- external, black-box
```

The external test package can only see exported identifiers. This is a structural enforcement of black-box testing. It is the single most effective lever for keeping tests from rotting.

Same-package tests (`package foo`) are allowed only when one of these is true:

1. The package legitimately exports nothing useful for the test's contract, but the package itself is the unit (rare; usually means the package is too small or its API needs work).
2. A single helper has subtle invariants that the public API cannot demonstrate efficiently, and the helper is documented as internal-but-tested in the source. In this case the file is named `*_internal_test.go` and contains only that helper's behavior tests.

Even in same-package tests, the rule "assert on contracts, not internals" still holds. The package layout is not a license to test private fields.

Forbidden patterns:

- A `package foo` test file whose only reason to exist is access to a private function. Promote the function to exported with a documented contract, or test it through the exported caller.
- An `internal/testhelpers` package that exposes private state of `pkg/foo` to other packages. If a fixture genuinely belongs to many packages, give it its own behavior contract.
- `reflect.ValueOf(x).Field(0)` to read a private field in a test. There is no acceptable use of this pattern in this repo.

References: Mat Ryer, "5 simple tips and tricks for writing unit tests in #golang" ([medium.com/@matryer](https://medium.com/@matryer/5-simple-tips-and-tricks-for-writing-unit-tests-in-golang-619653f90742)).

## 3. Test patterns we use

### 3.1 Table-driven tests with subtests

The default shape for any function with multiple cases. Each row is a contract scenario; the row name tells the operator what failed.

```go
package parser_test

import (
    "testing"

    "github.com/firehol/update-ipsets/pkg/parser"
)

func TestExtractIPv4(t *testing.T) {
    t.Parallel()

    cases := []struct {
        name string
        in   string
        want []string
    }{
        {"empty", "", nil},
        {"single", "1.2.3.4", []string{"1.2.3.4"}},
        {"comments stripped", "# header\n1.2.3.4 # tail\n", []string{"1.2.3.4"}},
        {"ignores ipv6", "2001:db8::1\n1.2.3.4\n", []string{"1.2.3.4"}},
    }

    for _, tc := range cases {
        tc := tc // capture for parallel; harmless in Go 1.22+ but explicit is fine
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()

            got := parser.ExtractIPv4([]byte(tc.in))

            if !equalStrings(got, tc.want) {
                t.Fatalf("ExtractIPv4 returned %v; want %v", got, tc.want)
            }
        })
    }
}
```

Notes:

- Names are short and describe behavior, not inputs. "single" is fine; "TestExtractIPv4_OneIPv4_NoComments_ReturnsSlice" is not.
- One row covers one scenario. Do not pile six unrelated assertions into one row.
- The row's `want` is the contract. If a refactor changes internals, this stays untouched. If `want` has to change, the contract has changed; that change belongs in the SOW.

References: Go Wiki, "TableDrivenTests" ([go.dev/wiki/TableDrivenTests](https://go.dev/wiki/TableDrivenTests)); Dave Cheney, "Prefer table driven tests" ([dave.cheney.net/2019/05/07/prefer-table-driven-tests](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)).

### 3.2 Golden files

For functions whose output is large structured text (rendered HTML, generated JSON, formatted reports), keep the expected output in `testdata/` rather than literals.

Conventions:

- Inputs at `testdata/<name>.input`, expected output at `testdata/<name>.golden`.
- One subtest per pair, named after `<name>`.
- An `-update` flag refreshes goldens. The test must be runnable both ways from a clean checkout.

```go
var update = flag.Bool("update", false, "rewrite golden files")

func TestRenderReport(t *testing.T) {
    t.Parallel()

    matches, err := filepath.Glob(filepath.Join("testdata", "*.input"))
    if err != nil {
        t.Fatalf("glob: %v", err)
    }

    for _, in := range matches {
        in := in
        name := strings.TrimSuffix(filepath.Base(in), ".input")
        t.Run(name, func(t *testing.T) {
            t.Parallel()

            inputBytes, err := os.ReadFile(in)
            if err != nil {
                t.Fatalf("read input: %v", err)
            }

            got, err := report.Render(inputBytes)
            if err != nil {
                t.Fatalf("Render returned error: %v", err)
            }

            golden := filepath.Join("testdata", name+".golden")
            if *update {
                if err := os.WriteFile(golden, got, 0o644); err != nil {
                    t.Fatalf("write golden: %v", err)
                }
                return
            }

            want, err := os.ReadFile(golden)
            if err != nil {
                t.Fatalf("read golden: %v", err)
            }
            if !bytes.Equal(got, want) {
                t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
            }
        })
    }
}
```

Golden files are a contract artifact, not a snapshot of whatever the code happens to emit today. When a golden mismatch surfaces in a PR, the question is "does the new output still match the contract?", not "regenerate to make CI green". Regenerating without spec change is the snapshot-test anti-pattern.

References: Eli Bendersky, "File-driven testing in Go" ([eli.thegreenplace.net/2022/file-driven-testing-in-go](https://eli.thegreenplace.net/2022/file-driven-testing-in-go/)).

### 3.3 Filesystem and environment

Always use `t.TempDir()` and `t.Setenv()` / `t.Chdir()` (Go 1.24+ for `t.Chdir`). They register `t.Cleanup` automatically and survive subtest failures.

```go
func TestPublishWritesArtifacts(t *testing.T) {
    dir := t.TempDir()
    t.Setenv("UPDATE_IPSETS_WEB_DIR", dir)

    eng := engine.New(engine.Options{WebDir: dir})
    if err := eng.Publish(t.Context(), sampleSet()); err != nil {
        t.Fatalf("Publish: %v", err)
    }

    got, err := os.ReadFile(filepath.Join(dir, "sample.json"))
    if err != nil {
        t.Fatalf("artifact missing: %v", err)
    }
    // assert on contract-relevant fields of got, not on byte equality unless it is the contract
}
```

Rules:

- Never write into the repo source tree from a test.
- Never depend on cwd-relative paths unless `t.Chdir` made the cwd explicit.
- Never set process-wide env without `t.Setenv` (it forbids parallel ancestors, which is exactly the safety net you want).

References: `pkg.go.dev/testing` ([pkg.go.dev/testing](https://pkg.go.dev/testing)); Boldly Go, "T.Context" ([boldlygo.tech/archive/2025-04-09-t.context](https://boldlygo.tech/archive/2025-04-09-t.context/)).

### 3.4 Time

`time.Now()` in production code is observable behavior; tests must control it. Two acceptable approaches in this repo:

1. Inject a `func() time.Time` (or a small `Clock` interface) at the unit's construction boundary. Production passes `time.Now`; tests pass a deterministic function.
2. For code that uses sleeps, timers, or `context.WithTimeout`, use `testing/synctest` (stable in Go 1.25; the experimental form was removed in Go 1.26).

```go
// Option 1: inject a clock function
type Engine struct {
    now func() time.Time
}
func New(opts Options) *Engine { return &Engine{now: opts.Now} }
// In Options: Now func() time.Time `default:"time.Now"`

// Option 2: synctest for concurrent code with timers
func TestWithTimeoutFires(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        time.Sleep(5*time.Second + time.Nanosecond)
        synctest.Wait()

        if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
            t.Fatalf("ctx.Err()=%v; want DeadlineExceeded", ctx.Err())
        }
    })
}
```

Forbidden:

- `time.Sleep(...)` in tests as a synchronization primitive. Every such sleep is a future flake. Either use `synctest`, a channel, `sync.WaitGroup`, a deterministic clock, or refactor the production code so the test does not need to wait.
- Mocking `time.Now` via package-level monkey patching. It breaks parallel tests and is invisible to the reader.

References: Go blog, "Testing concurrent code with testing/synctest" ([go.dev/blog/synctest](https://go.dev/blog/synctest)); Go blog, "Testing Time (and other asynchronicities)" ([go.dev/blog/testing-time](https://go.dev/blog/testing-time)); Go 1.25 release notes ([go.dev/doc/go1.25](https://go.dev/doc/go1.25)).

### 3.5 Context

Use `t.Context()` (Go 1.24+) for the cancellation context passed into the unit; the test framework cancels it when the test ends, which surfaces "ignored context" bugs as goroutine leaks and timeouts.

```go
func TestDownloaderCancelsOnContext(t *testing.T) {
    srv := slowServer(t)            // helper that takes time.Duration
    d := downloader.New(srv.Client(), srv.URL)

    ctx, cancel := context.WithCancel(t.Context())
    cancel()                         // already canceled

    if _, err := d.Get(ctx, "/feed"); !errors.Is(err, context.Canceled) {
        t.Fatalf("Get on canceled ctx: err=%v; want context.Canceled", err)
    }
}
```

The contract under test is "Get must return `context.Canceled` (or wrap it) when ctx is canceled". The internal HTTP client, retry counter, or telemetry counters are not part of the assertion.

## 4. What NOT to test

Side-by-side examples. Each pair shows a real anti-pattern this repo has rejected and the behavioral version that survives refactors.

### 4.1 Internal call counting vs. observable output

Bad — locks the implementation:

```go
// BAD: counts internal calls; refactor to a single SQL JOIN breaks this test
type spyStore struct{ getCalls int }
func (s *spyStore) GetFeed(name string) (*Feed, error) { s.getCalls++; return &Feed{}, nil }

func TestPublishCallsGetFeedOnce(t *testing.T) {
    s := &spyStore{}
    Publish(s, "x")
    if s.getCalls != 1 {
        t.Fatalf("expected 1 GetFeed call, got %d", s.getCalls)
    }
}
```

Good — asserts on the artifact `Publish` is contracted to write:

```go
// GOOD: asserts the visible result; any internal change that still produces the file passes
func TestPublishWritesArtifact(t *testing.T) {
    dir := t.TempDir()
    p := publisher.New(publisher.Options{Dir: dir, Store: realStoreWithSeed(t)})

    if err := p.Publish(t.Context(), "x"); err != nil {
        t.Fatalf("Publish: %v", err)
    }
    if _, err := os.Stat(filepath.Join(dir, "x.ipset")); err != nil {
        t.Fatalf("artifact missing: %v", err)
    }
}
```

### 4.2 Log string matching vs. structured signal

Bad — couples to log copy:

```go
// BAD: any log refactor breaks the test
func TestRunLogsStarted(t *testing.T) {
    var buf bytes.Buffer
    Run(slog.New(slog.NewTextHandler(&buf, nil)))
    if !strings.Contains(buf.String(), "engine started") {
        t.Fatalf("missing 'engine started' log")
    }
}
```

Good — observe the event through its contract surface (admin status):

```go
// GOOD: the contract says /api/v1/status reports running:true after Start
func TestStartReportsRunning(t *testing.T) {
    eng := engine.New(t, defaultOptions(t))
    eng.Start(t.Context())
    t.Cleanup(eng.Stop)

    snap := eng.StatusSnapshot()
    if !snap.Running {
        t.Fatalf("snap.Running=false; want true")
    }
}
```

If a log line is genuinely part of the operator contract, assert on its structured `slog` attributes (key/value), not the rendered string.

### 4.3 Reaching into private state

Bad — uses `reflect` to peek at a private cache:

```go
// BAD
v := reflect.ValueOf(eng).Elem().FieldByName("cache")
if v.Len() != 5 { t.Fatalf("cache size") }
```

Good — exercise the cache through its contract (a public read returns cached results within freshness window):

```go
// GOOD: behavior visible at /api/v1/sets is the contract
resp := getJSON(t, srv.URL+"/api/v1/sets")
if got := len(resp.Sets); got != 5 {
    t.Fatalf("sets=%d; want 5", got)
}
```

## 5. Mocks: last resort

This repo uses neither `gomock` nor `mockery`. Generated mocks pull in dependencies the codebase otherwise does not need, encourage call-counting tests, and produce code that hides its own behavior from the reader.

### 5.1 Definitions

- **Stub**: returns canned values. Useful for narrow error-path tests.
- **Fake**: a working implementation, simplified for tests (e.g., in-memory filesystem, in-memory store). Behaves like the real thing for the contract of interest.
- **Mock**: records calls and asserts on them. Tests call counts and call ordering.
- **Spy**: a fake that also records calls.

We strongly prefer fakes. A fake gives behavior; tests assert on the resulting state. A mock gives nothing; tests assert on call patterns, which are implementation, not contract.

References: Quii, "Working without mocks" ([quii.gitbook.io/learn-go-with-tests/testing-fundamentals/working-without-mocks](https://quii.gitbook.io/learn-go-with-tests/testing-fundamentals/working-without-mocks)); HN discussion "Prefer Fakes over Mocks" ([news.ycombinator.com/item?id=24770954](https://news.ycombinator.com/item?id=24770954)); Redowan Delowar, "Your Go tests probably don't need a mocking library" ([rednafi.com/go/mocking-libraries-bleh](https://rednafi.com/go/mocking-libraries-bleh/)).

### 5.2 When a stub or fake is acceptable

Acceptable when the real dependency is one of:

- External network (use `httptest.NewServer` instead of a stub when feasible).
- Real time (use injected clock or `synctest`).
- Randomness (inject `*rand.Rand` or a seed).
- A nondeterministic third-party SDK that has no in-memory mode.
- Hardware or kernel resources that cannot be reproduced in a test container.

Not acceptable for:

- The standard library's `os`, `io`, `net/http`, `time` — Go ships testable substitutes.
- Code you control and can refactor.
- "It's slow" — write the integration test once in `_test.go`, mark it `testing.Short()` aware if needed; do not invent a fake to dodge writing real assertions.

### 5.3 Pattern: minimal interface at the consumer

Define interfaces where they are consumed, not where they are produced. Keep them small. A one-method interface can be a `func` type.

```go
// in pkg/engine/engine.go
type ipFetcher interface {
    Fetch(ctx context.Context, url string) ([]byte, error)
}

type Engine struct{ fetch ipFetcher }

// in pkg/engine/engine_test.go (external)
type fakeFetcher struct{ payload []byte; err error }
func (f fakeFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
    return f.payload, f.err
}
```

Two rules:

- The interface lives in the consumer package; the producer (e.g., `pkg/downloader`) returns concrete types and lets consumers describe what they need.
- The fake is a struct in the test file with the smallest fields it needs. No code generation.

### 5.4 Side-by-side: mock vs fake

Bad — counts calls on a mock, asserts implementation:

```go
// BAD
type mockStore struct{ saveCalls int; lastKey string }
func (m *mockStore) Save(k string, v []byte) error { m.saveCalls++; m.lastKey = k; return nil }

func TestSaveOnce(t *testing.T) {
    m := &mockStore{}
    h := NewHandler(m)
    h.Process("hello")
    if m.saveCalls != 1 || m.lastKey != "hello" {
        t.Fatalf("expected 1 Save with key=hello; got calls=%d key=%q", m.saveCalls, m.lastKey)
    }
}
```

Good — fake behaves like a real store; assertion is on the resulting state:

```go
// GOOD
type memStore struct{ items map[string][]byte }
func newMemStore() *memStore { return &memStore{items: map[string][]byte{}} }
func (s *memStore) Save(k string, v []byte) error { s.items[k] = append([]byte(nil), v...); return nil }
func (s *memStore) Load(k string) ([]byte, bool) { v, ok := s.items[k]; return v, ok }

func TestProcessPersists(t *testing.T) {
    s := newMemStore()
    h := NewHandler(s)
    if err := h.Process(t.Context(), "hello"); err != nil {
        t.Fatalf("Process: %v", err)
    }
    got, ok := s.Load("hello")
    if !ok || !bytes.Equal(got, []byte("processed:hello")) {
        t.Fatalf("Load(hello)=(%q, %v); want (processed:hello, true)", got, ok)
    }
}
```

If `Process` later calls `Save` twice, three times, or rewrites itself entirely, the good test still passes as long as the contract ("the processed value is persisted under that key") holds.

## 6. HTTP handlers: full black-box

The engine exposes public and admin HTTP routes. Tests must drive them through real HTTP, not by calling handler functions directly with `httptest.NewRecorder` (which skips middleware, route matching, and body negotiation).

```go
package web_test

import (
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/firehol/update-ipsets/pkg/web"
)

func TestStatusEndpoint(t *testing.T) {
    t.Parallel()

    eng := newTestEngine(t)
    srv := httptest.NewServer(web.NewMux(eng))
    t.Cleanup(srv.Close)

    resp, err := srv.Client().Get(srv.URL + "/api/v1/status")
    if err != nil {
        t.Fatalf("GET /api/v1/status: %v", err)
    }
    t.Cleanup(func() { resp.Body.Close() })

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        t.Fatalf("status=%d body=%s; want 200", resp.StatusCode, body)
    }
    if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
        t.Fatalf("Content-Type=%q; want application/json", got)
    }

    var snap struct {
        Running bool `json:"running"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
        t.Fatalf("decode: %v", err)
    }
    if !snap.Running {
        t.Fatalf("snap.Running=false; want true after Start")
    }
}
```

Rules:

- Build the full router (`web.NewMux` or equivalent) and serve it through `httptest.NewServer`. Middleware, auth, rate limits, headers — all part of the contract — get exercised.
- Use `srv.Client()` so HTTP/2, redirects, and cookies behave like a real client.
- Always `t.Cleanup(srv.Close)` and `resp.Body.Close()`.
- Assert on status, headers (only the contract-relevant ones), and decoded body — never on the in-memory `*httptest.ResponseRecorder` fields the handler did not produce.

For testing middleware in isolation, install a probe handler at a real route, hit it through `httptest.NewServer`, and assert on the observable rewrite (added headers, mutated context, rejected status).

For request cancellation/timeout: use `t.Context()` derived contexts; do not rely on `time.Sleep`.

For upstream HTTP that the production code calls, stand up a second `httptest.NewServer` configured to respond as the contract requires, and point the unit at its `srv.URL`.

References: `pkg.go.dev/net/http/httptest` ([pkg.go.dev/net/http/httptest](https://pkg.go.dev/net/http/httptest)); Speedscale, "Testing Golang with httptest" ([speedscale.com/blog/testing-golang-with-httptest](https://speedscale.com/blog/testing-golang-with-httptest/)).

## 7. Background workers and pipelines

The scheduler, downloader, and engine pipeline are the highest-stakes units in the repo. They must be tested end-to-end with small real inputs, not by mocking each stage.

Pattern:

1. Build the worker with real config pointing at `t.TempDir()` for any artifact directory.
2. Stand up `httptest.NewServer` instances for any upstream URLs (one per upstream is fine).
3. Give the worker a deterministic clock or use `synctest`.
4. Drive it through its public lifecycle: `Start`, enqueue (or trigger), `Stop`.
5. Assert on outputs that are part of the contract: artifacts written, admin-status transitions, integrity report contents, metrics with documented meaning.

Example shape:

```go
func TestSchedulerProcessesEnqueuedFeed(t *testing.T) {
    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        io.WriteString(w, "1.2.3.4\n")
    }))
    t.Cleanup(upstream.Close)

    cfg := config.Sample(t, config.WithFeed("test", upstream.URL))
    eng := engine.New(t, engine.Options{Config: cfg, WebDir: t.TempDir()})
    eng.Start(t.Context())
    t.Cleanup(eng.Stop)

    eng.Enqueue("test")

    // synchronize on observable state, not time.Sleep
    waitFor(t, 5*time.Second, func() bool {
        snap := eng.StatusSnapshot()
        return snap.LastSuccess("test") != nil
    })

    got, err := os.ReadFile(filepath.Join(eng.Options().WebDir, "files", "test.ipset"))
    if err != nil {
        t.Fatalf("artifact: %v", err)
    }
    if !bytes.Contains(got, []byte("1.2.3.4")) {
        t.Fatalf("artifact missing expected IP; got %q", got)
    }
}
```

Notes:

- `waitFor` is a small poller (`for time.Now().Before(deadline)`) that asserts on observable state, not time. Each project-testing scenario in this repo uses a logical timestamp advancer instead; use whichever the existing harness already uses (see `pkg/engine/pipeline_integrity_scenario_test.go`).
- Do not mock the downloader, processor, or publisher individually. Drive the whole pipeline; it is not slow when the upstream is `httptest.NewServer` and the artifact dir is `t.TempDir()`.
- Integrity checks rely on mtimes (see `.agents/sow/specs/integrity.md`). Tests for integrity behavior must not let `time.Now` leak into mtime stamping; use the same logical-timestamp seam the production code uses.

## 8. Concurrency

### 8.1 Race detector is mandatory

Every concurrent test must pass under `-race`. CI runs `make race` ([project-testing/SKILL.md](../project-testing/SKILL.md)). Local development:

```bash
go test -race -count=1 ./...
```

If a test passes only without `-race`, it is broken. Do not silence with `// +build !race`.

### 8.2 Determinism

Concurrent tests must be deterministic or use `synctest`. The combinations to use locally before committing:

```bash
go test -race -count=10 -shuffle=on ./pkg/<changed>/
```

`-shuffle=on` exposes hidden ordering dependencies between tests in the same package (a test that relies on the previous test's side effect will surface here).

### 8.3 Goroutine leaks

For long-lived units (engine, scheduler, web server), assert no leaked goroutines after `Close`/`Stop`. The standard library does not ship a leak detector; use `go.uber.org/goleak` only if a SOW approves the dependency. Otherwise:

- `Close` must block until all goroutines have exited.
- A test calls `Close`, then asserts on a flag the unit sets atomically when its last goroutine returns, exposed via a documented method or admin status field.

`goleak` is acceptable for a single TestMain-level guard if the SOW approves it; in that case, `func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }`.

References: `pkg.go.dev/go.uber.org/goleak` ([pkg.go.dev/go.uber.org/goleak](https://pkg.go.dev/go.uber.org/goleak)); brandur.org, "Habitually testing for goroutine leaks" ([brandur.org/fragments/goroutine-leaks](https://brandur.org/fragments/goroutine-leaks)).

### 8.4 Deadlocks

Never use `time.Sleep` to "let things settle". If two goroutines need to rendezvous, use a channel, a `sync.WaitGroup`, or `synctest.Wait()`. A test that hangs is a bug; surface it with a `t.Context()` or a small `select { case <-ctx.Done(): t.Fatal("timeout") }`.

## 9. Fuzz and property-based testing

### 9.1 Native `go test -fuzz`

The IP parsing surfaces in `pkg/iprange` already have fuzz tests. Pattern: a fuzz target is a stable invariant that should hold for any input.

```go
func FuzzParseIPv4(f *testing.F) {
    f.Add("1.2.3.4")
    f.Add("0.0.0.0")
    f.Add("255.255.255.255")

    f.Fuzz(func(t *testing.T, s string) {
        ip, err := iprange.ParseIPv4(s)
        if err != nil {
            return // documented contract: error is acceptable for malformed input
        }
        // Round-trip must be stable
        if got := ip.String(); got != s && !canonicalize(s, got) {
            t.Fatalf("ParseIPv4(%q).String()=%q; not a stable canonicalization", s, got)
        }
    })
}
```

Run locally before merging changes to a parser:

```bash
go test -run=^$ -fuzz=FuzzParseIPv4 -fuzztime=30s ./pkg/iprange
```

A fuzz failure produces a regression case in `testdata/fuzz/...`. Commit it; that case becomes a permanent table-driven regression test.

References: Go Fuzzing tutorial ([go.dev/doc/tutorial/fuzz](https://go.dev/doc/tutorial/fuzz)).

### 9.2 Property-based with rapid

For richer generators (state machines, structured data) `pgregory.net/rapid` is the supported choice. Add it only when a SOW approves the new dependency. Pattern:

```go
import "pgregory.net/rapid"

func TestMergeAssociative(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        a := genIPSet(t)
        b := genIPSet(t)
        c := genIPSet(t)

        left  := merge.Union(merge.Union(a, b), c)
        right := merge.Union(a, merge.Union(b, c))

        if !left.Equal(right) {
            t.Fatalf("Union not associative")
        }
    })
}
```

Properties to look for: idempotence, commutativity, associativity, monotonicity, round-tripping (parse→serialize→parse). They survive every refactor.

References: `pgregory.net/rapid` ([pgregory.net/rapid](https://pgregory.net/rapid/)).

## 10. Benchmarks

### 10.1 `b.Loop` (Go 1.24+)

Use `for b.Loop()` instead of `for i := 0; i < b.N; i++`. It runs setup once, prevents dead-code elimination, and reports cleaner results.

```go
func BenchmarkParseIPv4Stream(b *testing.B) {
    b.ReportAllocs()

    payload := loadFixture(b, "large.input") // setup: runs once
    for b.Loop() {
        if _, err := iprange.ParseIPv4Stream(bytes.NewReader(payload)); err != nil {
            b.Fatalf("Parse: %v", err)
        }
    }
}
```

References: Go blog, "More predictable benchmarking with testing.B.Loop" ([go.dev/blog/testing-b-loop](https://go.dev/blog/testing-b-loop)).

### 10.2 Benchmark workflow

For perf-sensitive changes (anything in the hot path: parsers, set algebra, processor pipeline):

```bash
# Baseline
git stash && go test -bench=. -benchmem -count=10 ./pkg/iprange > /tmp/old.txt
git stash pop

# Change
go test -bench=. -benchmem -count=10 ./pkg/iprange > /tmp/new.txt

benchstat /tmp/old.txt /tmp/new.txt
```

`-count=10` minimum (20 is better) so the t-test is meaningful. Single-run "benchmarks" are noise, not data. `benchstat` reports geomean and significance; use that, not the raw numbers.

References: `pkg.go.dev/golang.org/x/perf/cmd/benchstat` ([pkg.go.dev/golang.org/x/perf/cmd/benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)); bwplotka, "Leveraging benchstat Projections in Go Benchmark Analysis" ([bwplotka.dev/2024/go-microbenchmarks-benchstat](https://www.bwplotka.dev/2024/go-microbenchmarks-benchstat/)).

### 10.3 What NOT to micro-benchmark

- Allocator-only changes that are dominated by upstream noise.
- Anything that runs in <10ns; the loop overhead dominates and the result is noise.
- Anything where the contract does not include a perf budget. If the SOW does not say "this must stay under N µs", a micro-benchmark is implementation-locking.

The repo has hot-path regression guards (e.g., `TestEffectiveEntryHelpersExposeSnapshotCost`); their job is shape, not absolute time. Add similar guards when a SOW identifies a hot-path correctness-as-perf invariant.

## 11. Flake hunting

A flaky test is a bug. Do not bandage it with retries or `time.Sleep`. The root cause is one of:

| Cause | Fix |
|-------|-----|
| Real time | Inject a clock or use `synctest` |
| Randomness | Inject a seeded `*rand.Rand` |
| Goroutine ordering | Use a channel, `sync.WaitGroup`, or `synctest.Wait` |
| Network | Use `httptest.NewServer`; never hit a real domain |
| Filesystem shared state | `t.TempDir()` — never write under the repo |
| Test ordering | `-shuffle=on` will surface it; isolate fixtures |
| `t.Parallel` with shared mutable state | Remove the sharing; that is what `t.Parallel` exposes |

Reproduce locally:

```bash
go test -race -count=100 -shuffle=on -timeout=2m ./pkg/<package>/
```

If the test passes 100/100 here but fails once on CI, the next likely cause is CI machine noise (CPU scheduling, fewer cores). Lower concurrency or convert real-time waits to `synctest`. Do not mark it `t.Skip()`.

Marking a test `t.Skip()` to "fix" a flake is a contract violation. Either fix the bug, or remove the test and document the gap in the SOW.

References: VictoriaMetrics, "Go synctest: Solving Flaky Tests" ([victoriametrics.com/blog/go-synctest](https://victoriametrics.com/blog/go-synctest/)); Thoughtworks, "No more flaky tests on the Go team" ([thoughtworks.com/insights/blog/no-more-flaky-tests-go-team](https://www.thoughtworks.com/en-es/insights/blog/no-more-flaky-tests-go-team)).

## 12. Working with AI-generated Go tests

LLM-generated tests fail in stereotyped ways. Treat every AI-generated `*_test.go` as suspect until verified against this section. The rules below are the same rules as the rest of this skill; AI-generated code just produces these violations at scale.

### 12.1 Documented failure modes

Reported in the public discussion (citations below):

1. **Tautological assertions.** Tests that only check `if err != nil { t.Fatalf(err) }` and nothing else, or that re-run the function and compare its output to a freshly recomputed call. ([Markaicode, "How to Fix AI-Generated Go v1.23 Unit Test Errors"](https://markaicode.com/fix-go-unit-test-ai-errors/), 2025; [Aman Shekhar, "Unmasking the Flaws"](https://shekhar14.medium.com/unmasking-the-flaws-why-ai-generated-unit-tests-fall-short-in-real-codebases-71e394581a8e), 2024.)
2. **Over-mocking.** Pulling in `gomock`/`mockery`/`testify/mock` and mocking every collaborator, often "because there were a lot of hastily written examples on the web". ([Redowan Delowar, "Your Go tests probably don't need a mocking library"](https://rednafi.com/go/mocking-libraries-bleh/), 2024.)
3. **Happy-path padding.** 50 nearly-identical table rows, none of which exercise the boundary. CodeLlama study reported <30% branch coverage for functions with >5 branches even when the test "looked thorough". ([Aman Shekhar, "Unmasking the Flaws"](https://shekhar14.medium.com/unmasking-the-flaws-why-ai-generated-unit-tests-fall-short-in-real-codebases-71e394581a8e), 2024.)
4. **Missing edge cases.** Off-by-one on `>=` vs `>` survives 100% line coverage because boundary inputs are not in the table. ([dev.to, "Your Go Tests Pass, But Do They Actually Test Anything?"](https://dev.to/r4mimu/your-go-tests-pass-but-do-they-actually-test-anything-an-introduction-to-mutation-testing-1k9l), 2024.)
5. **`time.Sleep` for synchronization.** Common in AI-suggested concurrent tests; results in CI flakes. ([VictoriaMetrics, "Go synctest"](https://victoriametrics.com/blog/go-synctest/), 2025.)
6. **No `t.Parallel`, `t.Cleanup`, `t.TempDir`.** AI imitations of older code samples often miss the modern stdlib helpers. ([pkg.go.dev/testing](https://pkg.go.dev/testing).)
7. **Tests that rewrite the function under test.** A test that re-implements `Add(a, b)` as `a + b` and asserts equality is testing the test, not the unit. ([Joshua Kite, "Testing a Golang Application Written with Artificial Intelligence"](https://www.joshuakite.co.uk/posts/testing_a_golang_application_written_with_artifical_intelligence.html), 2024.)
8. **Goroutine leaks ignored.** AI tests rarely add leak guards; long-running engines silently accumulate goroutines. ([brandur.org, "Habitually testing for goroutine leaks"](https://brandur.org/fragments/goroutine-leaks).)
9. **Confidently wrong on tooling versions.** ChatGPT cited a `mockgen -type` flag that does not exist in the cited version. Treat any AI claim about flag/version availability as unverified. ([Joshua Kite, 2024.](https://www.joshuakite.co.uk/posts/testing_a_golang_application_written_with_artifical_intelligence.html))
10. **Tests passing on broken code.** GitHub Copilot has been observed editing the test (not the code under test) to make CI green; floating-point overflow tests that never trigger because the math was wrong. ([Jeremy Bytes, "Trying and Failing with GitHub Copilot"](https://jeremybytes.blogspot.com/2024/12/trying-and-failing-with-github-copilot.html), 2024.)
11. **Unit-tests-as-coverage.** AI prefers covering lines to verifying boundaries; tests pass when comparison operators flip silently. ([dev.to mutation testing article](https://dev.to/r4mimu/your-go-tests-pass-but-do-they-actually-test-anything-an-introduction-to-mutation-testing-1k9l), 2024.)
12. **r/golang has a moderation policy** specifically because AI-generated Go submissions ship "production ready" code with memory leaks and stub-only tests; a public reminder that AI test output is not self-validating. ([Client/Server, "/r/golang draws a line on AI-generated projects"](https://www.clientserver.dev/p/rgolang-draws-a-line-on-ai-generated), 2025.)

### 12.2 Reviewer checklist for AI-generated tests

Run through this list before approving any test you did not personally write:

- [ ] Test package is `pkg_test`, not `pkg`. If `pkg`, justify in the test file's top comment.
- [ ] No `gomock`, `mockery`, `testify`, `ginkgo`, `gomega`, `gocheck`, `convey` imports. Project policy is stdlib-only.
- [ ] No `time.Sleep` anywhere in `_test.go`. Search: `grep -rn 'time.Sleep' .` inside the changed package's tests.
- [ ] Every test that starts a goroutine has a `t.Cleanup` or explicit `Stop`/`Close`.
- [ ] `t.TempDir`, `t.Setenv`, `t.Chdir`, `t.Context` are used instead of manual `os.MkdirTemp`/`os.Setenv`.
- [ ] `t.Parallel()` is present unless the test mutates shared global state (in which case fix the shared state).
- [ ] Each subtest's body produces at least one assertion that would fail if the contract were broken. `if err != nil { t.Fatal(err) }` alone does not count.
- [ ] No assertion compares the function's output to a recomputation of the same function. Search for the function name appearing twice in the test body.
- [ ] No assertion reads a private field via `reflect`, `unsafe`, or `unexported access via test package shadowing`.
- [ ] No log-string substring matches as the primary assertion.
- [ ] Boundary inputs are present: empty, zero, one, max-int, max-len, min-len, off-by-one of any documented threshold, malformed input that the contract documents as an error.
- [ ] If the unit has any concurrent behavior, the test runs under `-race` and either uses `synctest` or has a documented synchronization strategy that does not involve sleeping.
- [ ] If HTTP, the test goes through `httptest.NewServer`, not `httptest.NewRecorder` with a direct handler call (unless the unit is a middleware tested through a probe handler).
- [ ] If the test name contains `Internal`, `Private`, `_DoesYWith`, `CallsXOnce`, `RecordsCall`, treat it as an over-mock smell and rewrite.
- [ ] Table tests have meaningfully different rows. 50 rows that vary only one trivial field signal padding.
- [ ] If the function under test is in the diff, mutate one of its operators locally (`>` to `>=`, `==` to `!=`) and rerun the new test; if it still passes, the test is not asserting the contract.

### 12.3 Greppable smells

When reviewing a PR with AI-generated tests, run:

```bash
git diff --name-only origin/main...HEAD -- '*_test.go' | xargs -r grep -nE '\
time\.Sleep|\
github\.com/stretchr/testify|\
github\.com/golang/mock|\
go\.uber\.org/mock|\
mockery|\
reflect\.ValueOf\([^)]*\)\.Elem\(\)\.FieldByName|\
EXPECT\(\)\.|\
Times\([0-9]+\)|\
RecordedCalls'
```

Any hit needs a written justification in the PR or rewrite.

### 12.4 Quick rewrite recipe

When a reviewer rejects an AI-generated test, the rewrite path is mechanical:

1. Identify the contract being verified. Write it as one sentence at the top of the test file.
2. Delete the mocks. Replace each with the smallest fake that gives the unit something to talk to.
3. Replace `assert.Nil(err)` with a meaningful end-state assertion.
4. Add boundary rows to the table.
5. Verify with one local mutation (operator flip) that the test fails on broken code.
6. Run `go test -race -count=10 -shuffle=on ./pkg/<package>/`.

## 13. Quick reference

### DO

| | |
|---|---|
| External package | `package foo_test` |
| Subtests | `t.Run(name, func(t *testing.T){ ... })` |
| Parallel | `t.Parallel()` at the top of each independent test/subtest |
| Filesystem | `t.TempDir()` |
| Env | `t.Setenv("KEY", "value")` |
| Context | `ctx := t.Context()` |
| Cleanup | `t.Cleanup(func(){ ... })` |
| Time injection | inject `func() time.Time` or use `testing/synctest` |
| HTTP server | `httptest.NewServer(handler)` + `srv.Client()` |
| Concurrent code | `synctest.Test`, race detector, observable-state polling |
| Table tests | anonymous struct slice + `t.Run(tc.name, ...)` |
| Golden files | `testdata/*.input` + `*.golden`, `-update` flag |
| Fuzz | `f.Add` seeds, contract-invariant `f.Fuzz` body |
| Benchmarks | `for b.Loop()` + `b.ReportAllocs()` + `benchstat -count=10` |
| Concurrency hygiene | race-clean, no leaked goroutines, no test-ordering deps |

### DON'T

| | |
|---|---|
| testify, ginkgo, gomega | banned dependencies |
| gomock, mockery | banned; use small handwritten fakes |
| `time.Sleep` in tests | flake source; use `synctest` or channels |
| `reflect`/`unsafe` to read private fields | contract violation |
| Log-string substring assertions | use structured data or admin-status |
| Tests in `package foo` reaching into private state | move to `foo_test` and use the API |
| `httptest.NewRecorder` direct handler calls | use `httptest.NewServer` to exercise routing/middleware |
| Counting calls on a mock | assert on resulting state instead |
| 100% coverage as the goal | mutation/boundary coverage is the goal |
| Snapshot-style golden refresh | only refresh when the contract changes |
| `t.Skip` to silence flakes | fix the root cause |
| Generated mocks for trivial 1-method interfaces | use a function-type fake |
| `_test.go` that re-implements the function under test | the test is its own bug |
| Sleeps to "wait for goroutines" | poll observable state, or use `synctest.Wait` |

## 14. Sources

Primary references, with last-known good URLs and the year of the cited content:

- Ian Cooper, "TDD: Where Did It All Go Wrong" — InfoQ recording of the original talk, evergreen ([infoq.com/presentations/tdd-original](https://www.infoq.com/presentations/tdd-original/)).
- Dave Cheney, "Practical Go: Real world advice for writing maintainable Go programs" — GopherCon Singapore 2019 ([dave.cheney.net/practical-go](https://dave.cheney.net/practical-go/presentations/gophercon-singapore-2019.html)).
- Dave Cheney, "Prefer table driven tests", 2019 ([dave.cheney.net/2019/05/07/prefer-table-driven-tests](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)).
- Mat Ryer, "5 simple tips and tricks for writing unit tests in #golang", 2019 ([medium.com/@matryer](https://medium.com/@matryer/5-simple-tips-and-tricks-for-writing-unit-tests-in-golang-619653f90742)).
- Peter Bourgon, "Go for Industrial Programming" ([peter.bourgon.org/go-for-industrial-programming](https://peter.bourgon.org/go-for-industrial-programming/)).
- Quii, "Working without mocks" — Learn Go with Tests ([quii.gitbook.io/learn-go-with-tests/testing-fundamentals/working-without-mocks](https://quii.gitbook.io/learn-go-with-tests/testing-fundamentals/working-without-mocks)).
- Quii, "Anti-patterns" — Learn Go with Tests ([quii.gitbook.io/learn-go-with-tests/meta/anti-patterns](https://quii.gitbook.io/learn-go-with-tests/meta/anti-patterns)).
- Redowan Delowar, "Your Go tests probably don't need a mocking library", 2024 ([rednafi.com/go/mocking-libraries-bleh](https://rednafi.com/go/mocking-libraries-bleh/)).
- Hacker News, "Prefer Fakes over Mocks", discussion thread, 2020 ([news.ycombinator.com/item?id=24770954](https://news.ycombinator.com/item?id=24770954)).
- Go blog, "Testing concurrent code with testing/synctest", 2024 ([go.dev/blog/synctest](https://go.dev/blog/synctest)).
- Go blog, "Testing Time (and other asynchronicities)" ([go.dev/blog/testing-time](https://go.dev/blog/testing-time)).
- Go 1.25 release notes, 2025 ([go.dev/doc/go1.25](https://go.dev/doc/go1.25)) — `testing/synctest` is stable; the experimental form is removed in Go 1.26.
- `pkg.go.dev/testing/synctest` ([pkg.go.dev/testing/synctest](https://pkg.go.dev/testing/synctest)).
- `pkg.go.dev/testing` — `T.Context`, `T.Setenv`, `T.Chdir`, `T.TempDir`, `T.Cleanup` ([pkg.go.dev/testing](https://pkg.go.dev/testing)).
- Go blog, "More predictable benchmarking with testing.B.Loop", 2025 ([go.dev/blog/testing-b-loop](https://go.dev/blog/testing-b-loop)).
- `pkg.go.dev/golang.org/x/perf/cmd/benchstat` ([pkg.go.dev/golang.org/x/perf/cmd/benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)).
- bwplotka, "Leveraging benchstat Projections in Go Benchmark Analysis", 2024 ([bwplotka.dev/2024/go-microbenchmarks-benchstat](https://www.bwplotka.dev/2024/go-microbenchmarks-benchstat/)).
- Go Fuzzing tutorial ([go.dev/doc/tutorial/fuzz](https://go.dev/doc/tutorial/fuzz)).
- Go security, "Go Fuzzing" ([go.dev/doc/security/fuzz/](https://go.dev/doc/security/fuzz/)).
- `pgregory.net/rapid` ([pgregory.net/rapid](https://pgregory.net/rapid/), [github.com/flyingmutant/rapid](https://github.com/flyingmutant/rapid)).
- `pkg.go.dev/net/http/httptest` ([pkg.go.dev/net/http/httptest](https://pkg.go.dev/net/http/httptest)).
- Speedscale, "Testing Golang with httptest" ([speedscale.com/blog/testing-golang-with-httptest](https://speedscale.com/blog/testing-golang-with-httptest/)).
- Eli Bendersky, "File-driven testing in Go", 2022 ([eli.thegreenplace.net/2022/file-driven-testing-in-go](https://eli.thegreenplace.net/2022/file-driven-testing-in-go/)).
- `pkg.go.dev/go.uber.org/goleak` ([pkg.go.dev/go.uber.org/goleak](https://pkg.go.dev/go.uber.org/goleak)).
- brandur.org, "Habitually testing for goroutine leaks" ([brandur.org/fragments/goroutine-leaks](https://brandur.org/fragments/goroutine-leaks)).
- VictoriaMetrics, "Go synctest: Solving Flaky Tests", 2025 ([victoriametrics.com/blog/go-synctest](https://victoriametrics.com/blog/go-synctest/)).
- Thoughtworks, "No more flaky tests on the Go team" ([thoughtworks.com/insights/blog/no-more-flaky-tests-go-team](https://www.thoughtworks.com/en-es/insights/blog/no-more-flaky-tests-go-team)).
- dev.to, "Your Go Tests Pass, But Do They Actually Test Anything? An Introduction to Mutation Testing", 2024 ([dev.to/r4mimu](https://dev.to/r4mimu/your-go-tests-pass-but-do-they-actually-test-anything-an-introduction-to-mutation-testing-1k9l)).
- Aman Shekhar, "Unmasking the Flaws: Why AI-Generated Unit Tests Fall Short in Real Codebases", 2024 ([shekhar14.medium.com](https://shekhar14.medium.com/unmasking-the-flaws-why-ai-generated-unit-tests-fall-short-in-real-codebases-71e394581a8e)).
- Joshua Kite, "Testing a Golang Application Written with Artificial Intelligence", 2024 ([joshuakite.co.uk](https://www.joshuakite.co.uk/posts/testing_a_golang_application_written_with_artifical_intelligence.html)).
- Markaicode, "How to Fix AI-Generated Go v1.23 Unit Test Errors", 2025 ([markaicode.com](https://markaicode.com/fix-go-unit-test-ai-errors/)).
- Jeremy Bytes, "Trying and Failing with GitHub Copilot", 2024 ([jeremybytes.blogspot.com](https://jeremybytes.blogspot.com/2024/12/trying-and-failing-with-github-copilot.html)).
- Client/Server, "/r/golang draws a line on AI-generated projects", 2025 ([clientserver.dev](https://www.clientserver.dev/p/rgolang-draws-a-line-on-ai-generated)).
