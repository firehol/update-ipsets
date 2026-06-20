---
name: project-coding
description: "Go, React, config, and repo conventions for update-ipsets. MUST be followed for all code changes."
---

## Languages and frameworks

- Go 1.26 main module: `github.com/firehol/update-ipsets` (evidence: `go.mod`).
- React 19 + Vite + TypeScript SPA under `ui/` (evidence: `ui/package.json`).
- Tailwind CSS v4, shadcn-style UI primitives, TanStack Query/Table, Radix UI, Recharts/D3/VisX for visualization (evidence: `ui/package.json`, `ui/components.json`).
- Nested Go module: `tools/dronebl2ipsets`, imported through the root `replace` (evidence: `go.mod`, `tools/dronebl2ipsets/go.mod`).

## Code style and formatting

- Go code follows standard `gofmt` and standard-library `testing`; lint is `go vet ./...` (evidence: `Makefile`).
- TypeScript is strict: `strict`, `noUnusedLocals`, `noUnusedParameters`, and `noFallthroughCasesInSwitch` are enabled (evidence: `ui/tsconfig.app.json`).
- UI imports may use the `@/*` alias for `ui/src/*` (evidence: `ui/tsconfig.app.json`, `ui/vite.config.ts`, `ui/components.json`).
- Prettier is installed but not enforced by scripts; do not claim it is a project gate (evidence: `ui/package.json` has Prettier but no Prettier script).

## Project contracts

- Product/application behavior is specified in `.agents/sow/specs/*.md`; update the matching spec immediately when behavior, config, layout, pipeline, website/admin, integrity, memory, or compatibility contracts change (from SOW-0009).
- Do not hardcode feed names. Use config fields, `use:` roles, or backend-exposed flags (from SOW-0006).
- Do not hardcode operator-policy IP/CIDR lists in Go or UI code. Small curated reference feeds belong in YAML `static:` data, and larger lists belong in `url:` sources, artifacts, or merges so operators can customize them without rebuilding. Public reference metadata does not imply raw redistribution; critical reference feeds still follow the direct-upstream redistribution rule and should be marked non-redistributable only when direct-upstream evidence explicitly forbids redistribution, republication, copying, mirroring, public display, or public sharing (from SOW-0017 and SOW-0014).
- Before changing public methodology, docs, UI copy, admin copy, SOW text, or specs, use `project-content-surfaces`; public methodology explains user-facing meaning and limits, while config/API/code details belong in operator docs or specs (from SOW-0017 regression).
- Treat generated public artifact names as part of the config contract. New
  suffix-based artifacts must reserve any provider names and source names that
  would collide with real public feed files, and public serving/cleanup code
  must prefer exact feed identity before generated-artifact parsing (from
  SOW-0017).
- Never infer feed/provider semantics from substrings, prefixes, or suffixes
  in configured names. Use config fields, `use:` roles, typed metadata, or an
  exact configured-name identity lookup; artifact suffix parsing is only a
  storage-address decoder after exact feed/provider identities are known (from
  SOW-0017 regression).
- ASN and geolocation defaults must come from explicit config defaults, not
  from source ordering. Provider-list APIs should move the configured default
  to the front while preserving catalog order for the remaining providers
  (from SOW-0028).
- Generated artifact mtimes are pipeline integrity data. Any app-created file
  checked by integrity must get an explicit logical timestamp from the owning
  source, processing event, or dependency aggregate before publication; do not
  rely on write/rename wall-clock mtimes (from SOW-0017 regression).
- Feed health semantics must distinguish threat-feed freshness from
  reference/provider stability. Roles `critical_infrastructure`,
  `provider_context`, `asn`, and `geoip` suppress age-only health classes
  (`delayed`, `risky`, `unmaintained`) while preserving real `empty`,
  `unavailable`, and `archived` states; implement this through configured
  `use:` roles or typed fields, never feed-name matching (from SOW-0037).
- Entity country/ASN private sidecars and matching public payloads must share
  the same dependency-derived logical mtime when rewritten or freshness-touched.
  Unchanged entity refresh paths must touch both sides together instead of
  letting private/public publication order create false integrity drift (from
  SOW-0017 regression).
- Entity feed sidecars consume ASN payload freshness, and ASN payloads consume
  bogon providers for the `bogon_ips` versus `unknown_ips` split. Any entity
  sidecar fan-out or repair path that depends on ASN payloads must include the
  bogon `use:` role as an input dependency, even if the sidecar JSON itself
  does not serialize bogon counts (from SOW-0017 regression).
- Merge composition has signed inputs: additive `sources` and subtractive `exclude`. Preserve both the full dependency list and the signed include/exclude lists; do not overload `DerivedFrom` or `merge_excluded` when adding merge behavior (from SOW-0025).
- Merge-derived feeds may carry supported `use:` roles. When adding one, prove the role propagates from merge YAML to the expanded `Source`, the provider list, generated artifact expectations, and public serving path (from SOW-0025 regression).
- Configured subtractive merge inputs are strict dependencies for any otherwise-computable merge; do not skip disabled/archived/unmaintained/missing subtractive parents and publish a broader set (from SOW-0025 regression).
- Keep `pkg/iprange` standalone; it must not import other project packages.
- Keep `pkg/iprange` telemetry-framework agnostic. Do not import
  OpenTelemetry or project packages there. Return plain local operation stats
  from `pkg/iprange` APIs when callers need counters; engine/CLI/daemon callers
  own exporting those stats to OpenTelemetry, logs, admin status, or other
  surfaces (from SOW-0110).
- Keep `pkg/iprange` hot paths allocation-storm free. Per-range inserts,
  per-IP lookups, file-backed lookups, parser inner loops, and source algebra
  must not use telemetry callbacks, avoidable interface boxing, or avoidable
  per-item heap allocation (from SOW-0110).
- Do not edit generated frontend bundle files: `pkg/web/static/assets/*` or generated `pkg/web/static/index.html`. Edit `ui/`.
- Do not put expensive historical rescans on daemon startup critical path.
- Public sitemap entity detail URLs must come from the published/staged entity
  index artifacts used by the public API. Do not derive sitemap entity coverage
  from an independent live aggregation path unless the public index artifacts
  are unavailable (from SOW-0012).

## Go patterns

- Pass `context.Context` through long-running download, processing, scheduler, web, and engine operations (example: `pkg/downloader/downloader.go`).
- Return errors with context and wrap underlying failures with `%w` (example: `pkg/downloader/downloader.go`).
- Use structured `log/slog` for daemon/operator logs (examples: `cmd/update-ipsets/daemon.go`, `pkg/scheduler/scheduler.go`).
- Use OpenTelemetry helpers and existing telemetry counters/spans for material CPU, memory, network, and I/O operations outside standalone hot-path libraries (examples: `internal/observability/observability.go`, `pkg/downloader/downloader.go`, `pkg/processor/processor.go`).
- OpenTelemetry metric labels must be bounded identity, not runtime
  measurements. Do not add process IDs, queue depths, batch sizes,
  selected-feed counts, byte counts, fan-in counts, or other ephemeral values
  as metric attributes or metric resource attributes. Bounded labels such as
  feed name, status, HTTP status code, processor step, and engine phase are
  acceptable when they have direct operator value; put live quantities in metric
  values, admin status, logs, or traces instead (from SOW-0096).
- When deferring elapsed-time observation, defer a closure that calls
  `time.Since(started)` inside the closure; deferred direct-call arguments are
  evaluated immediately and will fail `go vet` (from SOW-0022).
- Treat `Close`, `Remove`, response-body, gzip, mmap/file-set, and temp-file
  cleanup errors deliberately. Return them where they affect publication or
  persisted data, log/propagate them where the caller can act, and use explicit
  `_ = ...` only for best-effort cleanup or test teardown (from SOW-0045).
- Preserve bounded-memory and cache-first behavior; prefer streaming, mmap/pread-backed sets, temp files, and iterator-based algorithms over heap-wide loads.
- Generic `iprange.RangeSource` hot-path algorithms belong in standalone
  `pkg/iprange`, not engine-local helpers. Exact source comparison, iterator
  materialization/counting, normalized content hashing, bounds extraction, and
  conservative overlap filters should be added to or reused from `pkg/iprange`;
  engine code should only orchestrate domain/artifact policy around those
  primitives (from SOW-0108).
- Engine code that needs materialized range-source union, intersection,
  exclusion, or exclusion counts should use `pkg/iprange` source-level APIs
  (`UnionSourcesContext`, `IntersectSourcesContext`, `ExcludeSourcesContext`,
  `ExcludeCountContext`, or `ExcludeRangesContext`) rather than composing
  `CollectIterContext`/`CountIterContext` with `UnionIter`, `IntersectIter`, or
  `ExcludeIter` in production hot paths (from SOW-0109).
- Before optimizing an apparent hot path, prove the production caller path
  exists. If an unexported helper has no production callers, remove the dead
  helper instead of adding tests or complexity around it (from SOW-0031).
- Public/API rate limiting must not create background cleanup goroutines. Use a
  bounded/lazily-pruned limiter state and standard token-bucket behavior for
  request rate limiting (from SOW-0031).
- Public JSON/static artifact serving may use the web file cache only with
  configured entry, byte, and per-file bounds. Oversized JSON/CSV/XML/TXT/HTML
  artifacts should stream from disk without entering the long-lived cache; raw
  `.ipset`/`.netset` routes must stay streaming and outside this cache (from
  SOW-0036).
- Public artifact and raw-download routes must open files relative to the
  configured served root with traversal-resistant APIs such as `os.Root`.
  Lexical path checks alone are not enough because a symlink under `WebDir`,
  `FilesDir`, or `BaseDir` could otherwise escape the served tree (from
  SOW-0062).
- HTTP handler stacks must include same-goroutine panic recovery that logs
  structured request context and returns a 500 response. Put recovery inside
  compression middleware so negotiated gzip responses remain valid on panics
  (from SOW-0031).
- HTTP route registrations should use Go 1.22+ `ServeMux` method patterns for
  public and admin route contracts. Public/API read routes are GET/HEAD,
  admin actions are POST, and unsupported methods for known routes should
  return `405 Method Not Allowed` with `Allow` instead of reaching the wrong
  handler (from SOW-0061).
- New generated public artifact dimensions need producer, refresh, repair, and serving-path evidence. Public handlers should serve/read existing artifacts or return missing; do not build metadata, history, changesets, retention, provider, comparison, or insight artifacts on first request, and honor `Options.WebDir` when serving published artifacts (from SOW-0025 regression).
- Staged public publication must preserve explicit `output.GeneratedFile`
  timestamps before rename. When adding a generated artifact family, add it to
  the generated-file ledger with the correct logical mtime; otherwise
  integrity will report stale files after normal processing (from SOW-0017
  regression).
- Critical-infrastructure public routes must compare artifact `provider_set_id`
  values against a cached engine provider-set ID refreshed on reload and
  critical-provider processing; do not rebuild the provider-set identity from
  cache entries on each public request (from SOW-0017).
- Public artifacts must be minimal: include only data with direct public product value. Do not serialize explicit empty facts when absence is sufficient; pairwise comparison artifacts must omit `common == 0` rows and incremental merge must delete stale zero-overlap rows (from SOW-0026).
- Global country/ASN entity API routes are also public artifact readers: serve `countries/*.json` and `asns/*.json` from the configured published tree, with no live builder fallback on public requests; repair/rebuild paths own regeneration (from SOW-0025 follow-up).
- Public metadata visibility is not raw redistribution permission. Any route returning or composing a raw feed body must enforce public-feed eligibility, redistributability, and archived-feed policy for every input, including JSON API body routes, compose routes, and compatibility download routes (from SOW-0025 follow-up).
- Raw feed body routes must require the cache entry's materialized file to exactly match `<feed>.ipset` or `<feed>.netset`, resolve it through safe path joining, and use the curated `WEB_DIR_FOR_IPSETS` mirror before falling back to the engine base directory (from SOW-0025 follow-up).
- Raw feed body routes must stream large `.ipset`/`.netset` files from disk or an equivalent bounded reader; do not serve them through the long-lived JSON/static `fileCache`, which reads and retains whole files in memory (from SOW-0025 regression).
- Comparison relatedness uses positive lineage, not all dependencies. For signed merges, additive inputs are positive ancestors; subtractive inputs are dependencies only and must not by themselves make comparison rows related (from SOW-0025 follow-up).
- Integrity and admin/public composition checks must use the same `enableAll` semantics as the running scheduler/admin surface; hardcoded `enableAll=false` causes false blocked-feed noise in development and operator workflows (from SOW-0025 regression).
- For entity country/ASN materialization, build shared lookup state such as effective-entry resolvers and feed-health classifiers once per batch, not inside per-feed-row loops; repeated full cache snapshots caused GC-heavy CPU waste (from SOW-0024).
- Names must expose expensive helper cost. Fresh full-cache snapshot helpers belong only on single-entry paths and should be named like `entryViewFromFreshStateSnapshot`; loop/batch paths must use `effectiveEntryResolver` or `feedHealthClassifier` created outside the loop (from SOW-0024 hardening).
- Scheduler runtime ownership is split by concern inside `pkg/scheduler`
  (`actions.go`, `automatic_due.go`, `download_loop.go`,
  `processing_loop.go`, `queue_admission.go`, `recovery.go`,
  `snapshot_build.go`). Preserve `Runner.stateMu` as the single authority over
  download and processing queue maps; do not re-merge these concerns into
  `scheduler.go` or move them behind package boundaries without a new design
  SOW (from SOW-0030).
- Scheduler runners must own and wait for the child goroutines they start.
  `Runner.Run(ctx)` should return only after fetch, processing, recovery, and
  in-flight download workers have observed cancellation and exited; otherwise
  shutdown and tests can race with cache/staging writes (from SOW-0035).
- Do not store `context.Context` on long-lived structs when it is only needed
  by a run call; pass it through the call path. Cancelled selected processing
  work should be visible in run accounting instead of disappearing from reports
  (from SOW-0031).
- Engine heavy phases are context-bound work. Geo/ASN/bogon/critical,
  comparison, metadata, and entity fan-out must stop scheduling new jobs on run
  cancellation, wait for bounded in-flight workers, and return cancellation
  rather than publishing a partial staged batch (from SOW-0035).
- Bounded heavy/background fan-out should preserve all observed in-flight
  worker failures with `errors.Join` or equivalent aggregation. First failure
  may cancel new work admission, but sibling worker failures are operator
  triage data and should not be hidden behind a race-winning `firstErr` (from
  SOW-0044).
- Daemon-owned background work must receive a daemon/service context, not a
  request context that dies when the HTTP response is written and not a naked
  `context.Background()`. Entity artifact queues, startup/reload checks, and
  admin-triggered rebuilds must propagate that context through rebuild/patch
  work and stop admitting new queue waves on shutdown (from SOW-0042).

## Frontend patterns

- Keep API access typed and centralized through concern-specific modules under
  `ui/src/lib/api-client/`. `ui/src/lib/api.ts` is a compatibility shim only;
  new components and query factories should import the narrow API client module
  they need instead of the broad `api` object (from SOW-0050).
- Use TanStack Query with explicit keys, `enabled` guards, and invalidation after mutations (examples: `ui/src/pages/admin.tsx`, `ui/src/components/admin/feed-modal.tsx`).
- Use TanStack Query `queryOptions()` factories split by route/concern under
  `ui/src/lib/queries/`, with shared keys in `ui/src/lib/query-keys.ts`.
  Do not create a central query-options barrel that imports every API concern;
  Rollup can hoist it into the public shell and defeat route splitting (from
  SOW-0050).
- TanStack Query `queryFn` callbacks must pass the provided `AbortSignal` to
  `ui/src/lib/api.ts` helpers. API helpers should accept an optional
  `AbortSignal` and forward it to `fetch` so navigation and key changes cancel
  obsolete requests (from SOW-0033).
- Top-level route pages should be lazy-loaded behind one route-level
  `Suspense` and error boundary in `ui/src/App.tsx`; do not eagerly import
  admin, feed-detail, methodology, and entity pages into the public homepage
  entry bundle (from SOW-0033).
- Route wrapper components that fetch route-specific data, such as the admin
  layout, should be lazy-loaded with their route too. An eager layout import can
  leak admin query code into the public entry chunk even when the page itself is
  lazy (from SOW-0050).
- Keep admin view state in URL parameters when it affects operator workflow (examples: `ui/src/pages/admin.tsx`, `ui/src/components/admin/feeds-table.tsx`).
- Preserve existing public/admin routing separation in `ui/src/App.tsx`.
- Keep visual work inside the existing design system unless a SOW explicitly approves a redesign (evidence: `ui/components.json`, `ui/src/components/ui/`).
- Theme state is owned by `next-themes` through the project
  `ThemeProvider`; do not add a second local theme context or direct
  `localStorage`/`document.documentElement.classList` theme writer (from
  SOW-0033).
- If Three/WebGL components are reintroduced, they must dispose scene-owned
  geometries, materials, textures, renderer state, and explicit materials on
  unmount. Raw HTML labels for visualization libraries must escape or sanitize
  data-derived text before returning markup strings (from SOW-0033/SOW-0040).
- Large list search/filter inputs should use `useDeferredValue` or
  `useTransition` when the derived list work is non-trivial. Keep the visible
  input state immediate and defer the expensive list computation (from
  SOW-0033).
- Clickable table rows and other non-button interactive surfaces need keyboard
  activation and accessible labels, or should be converted to real buttons or
  links where table semantics allow it (from SOW-0033).

## File and module layout

- `cmd/update-ipsets/`: CLI entrypoint and subcommands.
- `internal/`: private helpers such as file utilities, observability, and telemetry.
- `pkg/`: main Go packages: config, downloader, engine, scheduler, web, iprange, geoloc/asnloc, processor.
- `configs/firehol/`: authored catalog fragments.
- `ui/src/`: React source, with `components/`, `pages/`, and `lib/`.
- `tools/dronebl2ipsets/`: nested helper Go module.

## Things to avoid

- Do not create a second source of truth for product contracts outside `.agents/sow/specs/` (from SOW-0009).
- Do not replace simple scheduler/cadence logic with dependency-graph machinery without explicit approval.
- Do not make background work invisible; operator-visible daemon work belongs in admin status/UI (from SOW-0004).
