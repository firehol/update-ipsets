# SOW-0119 - Public Serving Runtime Rebind

## Status

Status: completed

Sub-state: implementation, validation, external review, and close-out complete.

This derivative was required by the active thread objective and was completed
after SOW-0118 was completed, committed, pushed, and locally installed.

## Requirements

### Purpose

Ensure public serving roots and public file-cache limits follow successful
runtime reloads without requiring a daemon restart, while preserving cache-first
public serving and cheap request handling.

### User Request

Complete SOW-0118 and any derivative from it. This derivative tracks the public
serving reload contract gap found during SOW-0118 plan review.

### Assistant Understanding

Facts:

- `pkg/web/surface_routes.go` captures public output roots and file-cache limits
  when routes are constructed.
- A successful runtime reload can change `WebDir`, `WebDirForIPSets`, `BaseDir`,
  and file-cache limit settings.
- Public serving must stay cache-first and must not trigger downloads or broad
  recomputation.

Inferences:

- The long-term design is a route-owned atomic public-serving state that is
  rebuilt after a new runtime generation is committed and loaded cheaply by
  public request handlers.
- Public requests must not hold the engine mutex while serving files or
  performing expensive public route work.

Unknowns:

- None for the design direction. Implementation details may be refined during
  SOW-0119 activation, but the selected approach is route-owned atomic serving
  state updated after committed runtime generation publication.

### Acceptance Criteria

- A committed runtime generation that changes the resolved public-serving
  `WebDir` is reflected by public artifact routes without restarting the web
  server.
- A committed runtime generation that changes the resolved public-serving
  `WebDirForIPSets` is reflected by raw ipset/netset routes without restarting
  the web server.
- Public file-cache limits use the latest committed runtime generation or are
  deliberately rebuilt when the effective limits change.
- Public file-cache entries from old roots become unreachable after a committed
  runtime generation changes `WebDir`, `WebDirForIPSets`, `BaseDir`, or cache
  limits.
  The selected strategy is to atomically swap a new serving state and fresh file
  cache when the effective root/limit tuple changes.
- The effective root/limit tuple is explicitly the resolved post-override
  tuple: `WebDir`, `WebDirForIPSets`, `BaseDir`,
  `WebArtifactCacheMaxEntries`, `WebArtifactCacheMaxBytes`, and
  `WebArtifactCacheMaxFileBytes`. Runtime changes hidden by stronger CLI/server
  options are correct no-ops; tests must include partial changes to each root
  class and at least one cache-limit-only change.
- A reload attempt that installs a new engine runtime generation must not leave
  public serving bound to the previous generation. Pre-publication reload
  failures, including candidate runtime directory-creation failures, leave the
  previous public-serving state in place. Post-publication maintenance errors
  must still leave public serving bound to the installed engine generation, with
  the reload error recorded separately.
- The reload-publication listener API is bounded. Registration must use a fixed
  named slot or replacement-by-name semantics, not unbounded append-only
  listener growth. The public web route layer registers exactly one listener for
  public-serving state refresh.
- Reload listener dispatch must be panic-isolated. A panicking listener must not
  crash the daemon or leave `e.mu`/reload ownership corrupted; the panic must be
  recovered, logged/recorded as reload listener failure, and the serving-state
  implementation must be designed so the normal public-serving refresh path is
  pure/non-failing.
- Runtime reload that reduces public file-cache limits is honored without
  continuing to serve from an old oversized cache generation.
- MCP markdown fetches follow the same effective `WebDir` generation as public
  markdown/artifact routes. They must not continue to read from the pre-reload
  `FileMarkdownStore` root.
- Public requests remain cache-first readers. They must not trigger upstream
  downloads, broad recomputation, metadata generation, or integrity scans.
- Reload to an unpopulated new public root serves from the new root and may
  return normal missing-file responses. It must not silently fall back to the
  old root and must not generate missing artifacts on demand.
- Behavioral web tests prove the rebind with real HTTP requests.
- Race tests prove reload can overlap public requests without mixed roots or
  data races.

## Analysis

Sources checked:

- `pkg/web/surface_routes.go`
- `pkg/web/server_run.go`
- `.agents/sow/specs/website.md`
- `.agents/sow/specs/operating-principles.md`

Current state:

- `newSurfaceRoutesWithContext` reads `eng.Runtime()` several times during
  route construction and stores `outputDir`, `ipsetsDir`, `baseDir`, and
  `fileCache` on the route struct.
- `newSurfaceRoutesWithContext` also constructs an MCP server with
  `mcppkg.NewFileMarkdownStore(outputDir)`. `FileMarkdownStore` stores that
  `webDir` and reads markdown files under it, so MCP markdown fetches are part
  of the same rebind surface.
- Once the server is running, those stored values do not automatically follow a
  successful runtime reload.

Risks:

- Operators can reload a changed `WebDir` or `WebDirForIPSets` and still have
  the public server read the old roots until restart.
- A naive request-time fix could make public requests more expensive or
  accidentally generate missing artifacts on demand.
- A naive cache fix could serve stale artifacts from a previous root.

## Pre-Implementation Gate

Status: plan-approved-for-implementation

Problem / root-cause model:

- Public route construction stores serving roots and cache policy from one
  runtime generation. Reload updates engine runtime state but does not rebuild
  or rebind the already-constructed public route stack.

Evidence reviewed:

- `pkg/web/surface_routes.go`: `newSurfaceRoutesWithContext` stores
  `outputDir`, `ipsetsDir`, `baseDir`, and `fileCache` on `surfaceRoutes`.
- `pkg/web/surface_routes.go:55` through `pkg/web/surface_routes.go:58`
  binds `mcpServer` to `mcppkg.NewFileMarkdownStore(outputDir)`.
- `pkg/mcp/fetch_analysis.go:41` through `pkg/mcp/fetch_analysis.go:47`
  stores the markdown root, and `pkg/mcp/fetch_analysis.go:61` reads markdown
  under that root.
- `pkg/web/server_run.go`: route construction happens during web server start.
- `cmd/update-ipsets/daemon.go:128` through `cmd/update-ipsets/daemon.go:139`
  is the current production SIGHUP reload path. It calls
  `eng.ReloadContext(ctx)`, logs the reloaded config path, and queues entity
  artifact checks, but it does not notify the already-constructed public route
  stack to rebuild serving roots or caches.
- `pkg/engine/engine.go:282` through `pkg/engine/engine.go:287` resolves runtime
  overrides and creates candidate runtime directories before engine generation
  installation; failures here leave the previous generation installed.
- `pkg/engine/engine.go:291` through `pkg/engine/engine.go:310` installs the new
  config/runtime/downloader/provider/cache generation before fallible
  post-publication maintenance at `pkg/engine/engine.go:315` through
  `pkg/engine/engine.go:343`. SOW-0119 must eliminate any installed-engine-
  generation versus public-serving-generation mismatch; it must not assume every
  non-nil `ReloadContext` return means the old engine generation is still
  installed.
- `.agents/sow/specs/operating-principles.md`: reload must be fail-safe and
  must not corrupt committed state, staged state, or queue ownership.

Affected contracts and surfaces:

- Public web routes and raw feed routes.
- MCP markdown fetch routes/tools.
- Runtime reload behavior.
- Public file-cache correctness and memory bounds.
- Operator expectation for reload-applied public serving without process
  restart.

Existing patterns to reuse:

- SOW-0118 combined runtime snapshot accessor.
- Engine-owned reload-publication listener registration. Public routes register
  a listener that rebuilds route-owned serving state when a new engine runtime
  generation is committed, outside the engine mutex and without request-time
  polling. The implementation may name the API as a reload-success hook only if
  reload publication and nil return are made equivalent by moving fallible
  maintenance before publication; otherwise the API must reflect committed
  generation publication rather than nil-return success. Current SOW-0118 code
  makes runtime-directory creation pre-publication, but post-publication
  maintenance can still return an error after the engine generation is
  installed.
- Go atomic pointer/value serving-state swaps already used elsewhere in the
  codebase if available; otherwise add a small route-owned atomic holder.
- Existing web file-cache tests and web HTTP test server fixtures.
- Cache-first public serving rules in project specs.

Risk and blast radius:

- Medium public serving regression risk.
- Medium stale-cache risk if roots or limits change without cache generation
  invalidation.
- Low data-loss risk because public serving should remain read-only.

Sensitive data handling plan:

- Evidence must stay at file/line and field-name level. Do not copy production
  paths, private endpoints, customer data, secrets, tokens, or non-private
  customer-identifying IP addresses into durable artifacts.

Implementation plan:

1. Finalize SOW-0118 first so the combined config/runtime snapshot accessor is
   available.
2. Add behavioral HTTP tests that serve from an initial web root, reload to a
   second root, and verify public artifact routes read the second root.
3. Add raw ipset/netset route tests for `WebDirForIPSets` reload.
4. Implement a route-owned atomic public-serving state containing effective
   `outputDir`, `ipsetsDir`, `baseDir`, file-cache limits, file cache, and MCP
   markdown serving root/server state.
5. Add a bounded generic engine reload-publication hook/listener registration
   API.
   The API must use a fixed named slot or replacement-by-name semantics, so a
   route rebuild or server wrapper cannot append unbounded listeners.
   The web route layer registers one listener that rebuilds and atomically
   publishes serving state from one coherent config/runtime snapshot. Hooks run
   outside `e.mu`, must not re-enter `ReloadContext`, and must not mutate
   runtime override state. Hook dispatch must recover panics and record them as
   reload listener failures without crashing the daemon.
6. After every committed runtime generation, rebuild and atomically publish the
   serving state from one coherent config/runtime snapshot. The listener
   dispatch point must be deterministic and code-locatable: in the current
   `ReloadContext` shape it belongs immediately after the engine state is
   installed and `e.mu` is unlocked, before any post-publication maintenance
   that can still return an error. Pre-publication reload failure must leave the
   previous serving state in place. If post-publication maintenance can still
   fail, public serving must follow the installed engine generation and the
   maintenance failure must be reported as a reload error, not hidden by serving
   old roots. Any reload entrypoint must route through `ReloadContext` or the
   same hook path so public serving cannot miss a committed runtime generation.
7. When the effective serving root or cache-limit tuple changes, publish a fresh
   file cache so old-root cache entries become unreachable. If the tuple is
   unchanged, the implementation may preserve the existing cache.
8. Ensure public request paths remain read-only and bounded.

Validation plan:

- `go test -race -count=10 ./pkg/web ./pkg/engine ./pkg/mcp`.
- Behavioral or structural reload-hook test proving successful `ReloadContext`
  or equivalent committed-generation publication invokes the public-serving
  state refresh once. The test must separately cover pre-publication reload
  failure leaving prior serving state in place and post-publication maintenance
  failure, if still possible, keeping public serving aligned with the installed
  engine generation.
- Behavioral or structural listener panic-isolation test proving a panicking
  reload listener is recovered and recorded without crashing the daemon or
  corrupting reload ownership.
- Behavioral or structural listener-registration test proving repeated web
  setup/re-registration replaces the public-serving listener instead of growing
  an unbounded listener list.
- Web same-failure scan for public route runtime snapshots and file-cache root
  handling.
- Behavioral cache test proving an old root is not served after reload when the
  effective root changes.
- Behavioral cache-limit test proving a reduced limit is applied after reload,
  and stale oversized cache state is unreachable.
- Behavioral partial-tuple tests proving a fresh cache is published when
  `WebDir`, `WebDirForIPSets`, or cache limits change independently, and proving
  unchanged tuples may preserve the existing cache without serving stale roots.
- Behavioral MCP markdown test proving fetches read from the new `WebDir` after
  reload and do not fall back to the old root.
- Behavioral missing-root or missing-file test proving reload to an unpopulated
  root serves missing-file responses from the new root without on-demand
  generation.
- External review after implementation.

Artifact impact plan:

- AGENTS.md: no expected update unless a broader reload rule is discovered.
- Runtime project skills: likely update if a durable public-serving reload rule
  is selected.
- Specs: update website and operating-principles reload contracts.
- End-user/operator docs: likely update if reload semantics become documented
  operator behavior.
- End-user/operator skills: no expected impact.
- SOW lifecycle: this derivative is required by SOW-0118.

Open-source reference evidence:

- None. This is a project-specific runtime ownership issue.

Open decisions:

- None. The long-term-best design choice is route-owned atomic public-serving
  state updated through a web-registered engine reload-publication listener
  after a runtime generation is committed; the listener registration API is
  bounded through fixed named slots or replacement-by-name semantics; old cache
  entries become unreachable by publishing a fresh cache when the effective
  root/limit tuple changes.

## Plan

1. Complete SOW-0118.
2. Re-open this SOW as current work.
3. Update specs for public serving reload behavior.
4. Write behavioral HTTP and race tests first.
5. Implement and externally review.

## Execution Log

### 2026-06-26

- Created as a derivative from SOW-0118 plan review so the public serving
  reload gap is tracked explicitly.
- Added stale-cache acceptance criteria from SOW-0118 plan review: old-root
  cache entries become unreachable by publishing a fresh cache when the
  effective root/limit tuple changes; reduced cache limits must be honored
  after reload.
- Selected the long-term-best implementation direction before activation:
  route-owned atomic public-serving state updated after committed runtime
  generation publication, with a fresh file cache when the effective root/limit
  tuple changes. Added MCP markdown serving root rebinding and unpopulated-root
  behavior to the scope.
- Added the reload-notification mechanism required by plan review: the engine
  will expose a generic reload-publication listener registration API, and
  public routes will register a listener that rebuilds route-owned serving state
  after committed runtime generation publication.
- Tightened the reload-publication listener API contract after plan review:
  registration must be bounded through fixed named slots or replacement-by-name
  semantics, the public route layer owns exactly one listener, hooks run outside
  `e.mu`, and hooks must not re-enter reload or mutate runtime override state.
- Reconciled the reload-listener contract with current `ReloadContext`
  publish-then-maintain behavior found during plan review. SOW-0118 moved
  runtime-directory creation before engine generation installation;
  post-publication maintenance can still return after publication. The SOW
  requires public serving to follow the installed engine runtime generation
  after any committed runtime publication, while pre-publication reload failures
  keep the prior public-serving state. The SOW also names the exact
  root/cache-limit tuple, requires partial tuple-change tests, and requires
  reload-listener panic isolation.
- Folded in sanitized plan-review v4 advisory clarifications before
  implementation: the effective tuple is the resolved post-override serving
  tuple, and listener dispatch must happen at the committed-generation
  publication point, before post-publication maintenance can return an error.

### 2026-06-27

- Activated after SOW-0118 was completed, committed, pushed, installed, and
  locally health-checked. Implementation starts with behavioral tests for public
  artifact rebind, raw-feed root rebind, cache replacement, MCP markdown
  rebind, and reload listener dispatch behavior.
- Added behavioral tests before implementation and confirmed they failed on the
  stale-root behavior: public direct artifacts, raw `.ipset` mirror serving,
  MCP `fetch_analysis`, public-only split serving, admin/public split listener
  preservation, pre-publication reload failure, and no runtime-root fallback.
- Added `pkg/engine` reload-publication listener support with named replacement
  registration, deterministic dispatch, panic recovery, listener error
  recording, and dispatch immediately after committed runtime generation
  installation.
- Reworked public route serving around an atomic route-owned serving generation
  that contains the effective post-override `outputDir`, `ipsetsDir`,
  `BaseDir`, and bounded file-cache generation. The serving generation is
  refreshed by fixed public/admin listener names after reload publication.
- Rebound public artifact routes, raw feed routes, entity/homepage artifact
  routes, admin integrity routes, and MCP markdown reads to load the current
  serving generation at request/tool execution time.
- Added coverage for the full effective tuple: `WebDir`, `WebDirForIPSets`,
  `BaseDir` fallback, CLI/server `WebDir` override no-op, and
  cache-limit-only changes that must publish a fresh cache generation.
- Updated `.agents/sow/specs/website.md`,
  `.agents/sow/specs/operating-principles.md`, and
  `.agents/skills/project-coding/SKILL.md` with the durable reload/public
  serving contract.

## Validation

Acceptance criteria evidence:

- Runtime reload publication hook:
  - `pkg/engine/reload_publication.go:24` provides named
    replacement-by-name listener registration.
  - `pkg/engine/reload_publication.go:69` dispatches a snapshot of listeners
    and joins listener errors.
  - `pkg/engine/reload_publication.go:83` recovers listener panics and records
    them as listener failures.
  - `pkg/engine/engine.go:317` dispatches reload-publication listeners after
    the new runtime generation is installed and outside the broad engine mutex.
  - `pkg/engine/engine.go:358` records listener failures in reload status
    without reverting the installed generation.
- Public serving generation:
  - `pkg/web/surface_routes.go:20` defines the effective public serving tuple:
    output root, raw-file root, base fallback root, and cache limits.
  - `pkg/web/surface_routes.go:74` and `pkg/web/surface_routes.go:78` register
    fixed public/admin serving refresh listener names.
  - `pkg/web/surface_routes.go:104` rebuilds serving state from one coherent
    config/runtime snapshot.
  - `pkg/web/surface_routes.go:120` preserves the existing cache only when the
    full tuple is unchanged.
  - `pkg/web/surface_routes.go:129` publishes a fresh serving state and fresh
    bounded file cache when the tuple changes.
  - `pkg/web/surface_routes.go:146` makes MCP markdown reads resolve the current
    public serving state instead of storing a stale markdown root.
- Route binding:
  - `pkg/web/public_routes.go:55` centralizes current-serving-state lookup for
    public handlers.
  - `pkg/web/public_routes.go:200`, `pkg/web/public_routes.go:278`, and
    `pkg/web/public_routes.go:348` use the current output root/cache for
    feed-scoped metadata, feed artifacts, and critical-infrastructure artifacts.
  - `pkg/web/public_routes.go:262` and `pkg/web/routes.go:282` use the current
    raw-file root plus base fallback for API and compatibility raw routes.
  - `pkg/web/routes.go:49` uses the current output root for admin pipeline
    integrity routes.
  - `pkg/web/http.go:237` keeps direct artifact path checks traversal-safe and
    now correctly handles a configured filesystem-root web directory.
- Tests:
  - `pkg/web/runtime_rebind_test.go:24` proves direct public artifact routes
    follow `WebDir` reload.
  - `pkg/web/runtime_rebind_test.go:59` proves reload to an empty new web root
    does not fall back to the old root.
  - `pkg/web/runtime_rebind_test.go:90` proves public artifacts follow
    `BaseDir` reload when `WebDir` is empty.
  - `pkg/web/runtime_rebind_test.go:119` proves public artifact requests can
    overlap reload without mixed roots or data races.
  - `pkg/web/runtime_rebind_test.go:225` proves raw routes follow
    `WebDirForIPSets` reload.
  - `pkg/web/runtime_rebind_test.go:263` proves raw routes follow `BaseDir`
    fallback reload when no ipsets mirror is configured.
  - `pkg/web/runtime_rebind_test.go:301` proves MCP `fetch_analysis` follows
    `WebDir` reload.
  - `pkg/web/runtime_rebind_test.go:332` proves stronger `Options.WebDir`
    overrides make runtime `WebDir` changes a correct no-op.
  - `pkg/web/runtime_rebind_test.go:369` proves admin integrity routes follow
    `WebDir` reload.
  - `pkg/web/runtime_rebind_test.go:408` and
    `pkg/web/runtime_rebind_test.go:416` prove cache-limit-only reloads publish
    a fresh cache generation and do not keep serving stale cached bytes.
  - `pkg/web/runtime_rebind_test.go:469` proves listener failure is recorded
    without preventing public serving from following the new generation.
  - `pkg/web/runtime_rebind_test.go:504` proves post-publication cleanup queue
    failure still leaves public serving bound to the installed generation.
  - `pkg/web/runtime_rebind_test.go:678` and
    `pkg/web/runtime_rebind_test.go:709` prove public-only and public/admin
    split handlers keep public reload binding.
  - `pkg/web/runtime_rebind_test.go:741` proves pre-publication reload failure
    keeps the previous public serving generation.
  - `pkg/web/runtime_rebind_test.go:768` proves public requests do not fall
    back to runtime `WebDir` when an override points elsewhere.
  - `pkg/web/http_test.go:8` and `pkg/web/http_test.go:19` prove `safePath`
    allows filesystem-root web directories for relative files while rejecting
    root traversal.
  - `pkg/engine/reload_publication_listener_test.go:12` proves named listener
    replacement.
  - `pkg/engine/reload_publication_listener_test.go:52` proves listener panic
    recovery and reload-status recording.

Tests or equivalent validation:

- Passed focused reviewer-response tests after external review:
  - `go test ./pkg/web -run 'TestSafePath|TestPublicArtifactRoutesRebindBaseDirWhenWebDirIsEmpty|TestPublicServingStateFollowsReloadWhenCleanupQueueFails' -count=1`
- Passed broader package tests:
  - `go test ./pkg/engine ./pkg/web ./pkg/mcp -count=1`
  - `go test ./tools/archposture`
- Passed focused race tests:
  - `go test -race -count=3 -run 'TestReloadContextDoesNotInstallRuntimeWhenDirectoryCreationFails|TestReloadPublicationListener|TestPublicArtifactRoutesRebind|TestPublicArtifactRoutesReload|TestRawFeedRoutesRebind|TestMCPFetchAnalysisRebinds|TestPublicRuntimeOverrideKeeps|TestAdminIntegrityRouteRebinds|TestPublicFileCache.*ReloadPublishesFreshCache|TestPublicServingStateFollows|TestPublicOnlyHandlerRegisters|TestAdminOnlyHandlerDoesNotReplace|TestReloadFailureKeeps|TestNoPublicRequestFallback|TestSafePath' ./pkg/engine ./pkg/web`
- Passed strict shuffled package gate:
  - `make test-strict`
- Passed lint/build gates:
  - `make lint`
  - `make build`
- Passed diff whitespace check:
  - `git diff --check`
- External review:
  - Six requested open-model reviewers were launched.
  - Five completed with production-grade verdicts or no blocking findings.
  - One reviewer session repeated the same read-only inspection loop and was
    stopped by interrupting that specific session.
  - Valid reviewer findings were addressed before close: a web-level
    post-publication cleanup-failure test, a public artifact `BaseDir` reload
    test when `WebDir` is empty, and the `safePath("/")` edge case plus tests.

Sensitive data gate:

- No raw secrets, credentials, bearer tokens, SNMP communities, community
  member names, customer names, personal data, non-private customer-identifying
  IPs, private endpoints, or proprietary incident details are included.

Artifact maintenance gate:

- AGENTS.md: no update needed; project-wide workflow did not change.
- Runtime project skills: `.agents/skills/project-coding/SKILL.md` updated with
  the public-serving reload generation rule.
- Specs: `.agents/sow/specs/website.md` updated for public/MCP serving
  generation behavior; `.agents/sow/specs/operating-principles.md` updated for
  reload-publication listener behavior.
- End-user/operator docs: no separate docs update needed. This is transparent
  reload correctness; operators already expect a successful reload to apply
  without restart.
- End-user/operator skills: no impact.
- SOW lifecycle: current/in-progress derivative updated with implementation and
  validation evidence, then closed and moved to `.agents/sow/done/` with the
  implementation commit.

Follow-up mapping:

- Historical references to this SOW as a derivative from SOW-0118 are resolved
  by this completed implementation.
- External-review findings were implemented in this SOW. No valid item remains
  open for another SOW.

## Outcome

Completed. Public serving roots, raw feed roots, MCP markdown reads, admin
integrity routes, and web artifact cache limits now follow committed runtime
reload generations through bounded reload-publication listeners and an atomic
route-owned serving generation.

## Lessons Extracted

- Public handlers must not freeze runtime roots or cache limits at construction
  time. They need a route-owned serving generation that can be refreshed after
  committed runtime publication.
- Reload publication is not identical to a nil `ReloadContext` return. Some
  post-publication maintenance can fail after the engine generation is already
  installed, so public serving must follow the installed generation and record
  the maintenance error separately.
- Defensive path helpers should share the rooted relative-path validator so
  filesystem-root web directories do not create divergent traversal behavior.

## Followup

None.

## Regression Log

None yet.
