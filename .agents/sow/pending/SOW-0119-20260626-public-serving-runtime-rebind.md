# SOW-0119 - Public Serving Runtime Rebind

## Status

Status: pending

Sub-state: derivative from SOW-0118. Do not start until SOW-0118 has completed
unless the user explicitly changes priority.

This derivative is required by the active thread objective. The overall
SOW-0118 goal is not complete until this SOW is implemented, validated, reviewed,
closed, committed, pushed, and installed after SOW-0118.

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

Status: pending

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
  makes runtime-directory creation pre-publication, but later maintenance can
  still return an error after the engine generation is installed.
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
   route rebuild or future server wrapper cannot append unbounded listeners.
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
   old roots. Future reload entrypoints must route through `ReloadContext` or
   the same hook path so public serving cannot miss a committed runtime
   generation.
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
- SOW lifecycle: this pending derivative is required by SOW-0118.

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

- Created as a pending derivative from SOW-0118 plan review so the public
  serving reload gap is tracked explicitly.
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
  publish-then-maintain behavior found during plan review. SOW-0118 later moved
  runtime-directory creation before engine generation installation; later
  maintenance can still return after publication. The SOW requires public serving
  to follow the installed engine runtime generation after any committed runtime
  publication, while pre-publication reload failures keep the prior
  public-serving state. The SOW also names the exact root/cache-limit tuple,
  requires partial tuple-change tests, and requires reload-listener panic
  isolation.
- Folded in sanitized plan-review v4 advisory clarifications before
  implementation: the effective tuple is the resolved post-override serving
  tuple, and listener dispatch must happen at the committed-generation
  publication point, before post-publication maintenance can return an error.

## Validation

Acceptance criteria evidence:

- Pending.

Tests or equivalent validation:

- Pending.

Sensitive data gate:

- No raw secrets, credentials, bearer tokens, SNMP communities, community
  member names, customer names, personal data, non-private customer-identifying
  IPs, private endpoints, or proprietary incident details are included.

Artifact maintenance gate:

- SOW lifecycle: pending derivative created.

Follow-up mapping:

- This SOW tracks the public serving root/cache reload derivative from
  SOW-0118.

## Outcome

Pending.

## Lessons Extracted

Pending.

## Followup

None yet.

## Regression Log

None yet.
