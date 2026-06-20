# Operating Principles Contract

## Status

This document is normative.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** describe required
cross-cutting operational behavior of the product.

## Purpose

This document defines the operational principles that apply across downloader,
processing engine, website, admin UI, and integrity.

It exists so startup, performance, dependency, and publication rules do not
remain scattered across unrelated docs.

Memory-specific out-of-core rules remain detailed in
[memory-management.md](memory-management.md). This document owns the broader
cross-cutting operational discipline.

## Cache-first publication rule

The product is a publisher of computed artifacts, not a live query engine over
upstream networks.

For ordinary public browsing:

- viewing a feed page, comparison page, or analysis surface MUST use already
  published local artifacts
- public page views MUST NOT trigger:
  - upstream downloads
  - live recomputation of feed analytics
  - provider refresh
  - integrity scans
  - operator-only runtime actions

The public website MAY read local machine-readable artifacts over HTTP, but
those artifacts MUST be precomputed local outputs, not live recomputation.

Public artifacts MUST contain only data with direct public product value. The
site is expected to grow to many more feeds and providers, so "complete" but
valueless data is waste, not transparency. Absence MUST represent empty facts
when that is enough: for example, a missing pairwise comparison row means the
two feeds have zero overlap; the public artifact MUST NOT carry an explicit
zero row unless the zero itself has user-facing meaning beyond absence.

When a change adds a provider, comparison dimension, artifact suffix, public tab,
index, or generated public surface, the implementation MUST define and validate:

- the producer path that writes the artifact
- the event that refreshes the artifact
- the integrity/reprocess or repair path that restores missing or stale artifacts
- the serving route that reads the artifact without generating it on demand
- any daemon/runtime directory override that changes the served artifact tree
  MUST also change where producer and repair paths publish

Critical-infrastructure overlap artifacts are covered by this rule: their
aggregate/per-provider JSON files MUST be precomputed and never computed from a
public request path. The pipeline and admin integrity paths own the strict
`provider_set_id` equality contract. Public routes remain cache-first readers:
they serve structurally valid published artifacts that exist on disk regardless
of their internal `provider_set_id`, and they do not surface provider-set drift
as public editorial content.

## Dynamic-view exception

The product MAY expose dynamic APIs when the response is inherently specific to
the current viewer request.

Examples include:

- one-off IP lookup
- operator/admin runtime status
- explicitly requested diagnostic views

Even in those cases, the product SHOULD prefer local committed artifacts and
indexes over broad recomputation.

For request-scoped dynamic lookups over local feed state:

- the product SHOULD reuse already-open local indexes / filesets when that can
  be done safely
- request contexts MUST be propagated into local file/index cache opens and
  final response materialization checkpoints
- request handling MUST NOT repeatedly reopen the same committed local files
  when a correct local cache can avoid that cost
- if timing metadata such as `first_seen` is exposed, the request path SHOULD
  stop as soon as the contract allows instead of scanning farther than needed

## No repeated-view upstream dependency rule

Repeated visits to the public website MUST NOT create repeated dependence on
remote third-party APIs or remote upstream feeds.

The product MUST prefer:

- downloaded local provider data
- committed local feed bodies
- committed published artifacts

It MUST NOT build ordinary public browsing around live third-party API calls.

## External dependency minimization

The product SHOULD obtain data from downloadable files, local copies, or
durable mirrors whenever that preserves the same externally visible outcome.

Third-party APIs SHOULD be used only when:

- no practical downloadable source exists
- the product would otherwise lose required functionality
- the resulting operational cost and fragility are acceptable

## Startup availability rule

Startup MUST make the service available quickly.

Startup MUST NOT block availability on:

- broad historical rescans
- whole-catalog heavy recomputation
- optional analytical refreshes that can be deferred

Startup MAY queue local recovery work, but it SHOULD do so in a way that does
not delay basic service availability.

Examples of work that MAY be deferred into background startup refreshes:

- integrity-driven follow-up work
- publication repair for optional analytical surfaces
- country/ASN entity-artifact rebuilds used by public reference pages

Startup SHOULD prefer guarded integrity/repair checks over unconditional
full-surface rebuilds. If durable artifacts are already current, daemon restart
MUST NOT rebuild them just because the process restarted.

Daemon startup SHOULD remove stale publish staging directories left by aborted
or killed runs. This cleanup MUST be limited to known generated staging
prefixes under configured publication roots, MUST NOT remove published
artifacts, and MUST NOT run as a broad historical rescan or rebuild.

## Graceful shutdown rule

On normal termination, the product SHOULD shut down gracefully.

That means:

- stop accepting new externally triggered work
- allow in-flight HTTP serving to wind down with a bounded timeout
- preserve durable staged state for restart recovery
- keep committed published truth authoritative if active work does not finish

Graceful shutdown MUST NOT require deleting staged `.new` files merely to exit
cleanly.

Processing cancellation MUST remain visible to operators and callers. If a
processing run is cancelled after work has been selected but before a worker can
start an item, the run report SHOULD record that item as `cancelled` instead of
silently omitting it from updated/skipped/failed accounting.
Heavy analytical fan-out is part of the same cancellation contract. Once a
shutdown or caller cancellation is observed, heavy phases SHOULD stop admitting
new geo/ASN/bogon/critical/comparison/entity work, wait for already-running
bounded workers to settle, preserve committed truth, and return cancellation
instead of publishing a partially computed batch.

Long-running scheduler runners MUST have structured ownership of their child
goroutines. When the runner context is cancelled, `Run` should not return until
fetch, processing, recovery, and in-flight download workers have observed the
cancelled context and exited. Otherwise shutdown and test cleanup can race with
cache/staging writes.

Daemon-owned background work MUST inherit the daemon/service context, not a
short-lived HTTP request context and not an unbounded root context. Entity
artifact startup checks, reload checks, admin-triggered rebuilds, scheduler
feed-update refreshes, and health-transition refreshes MUST stop admitting new
background work after shutdown cancellation and SHOULD return
`context.Canceled` from the active worker path rather than continuing after the
service has begun stopping.

## Reload rule

If the product supports live configuration reload, reload MUST be fail-safe.

Rules:

- a valid reload MAY replace the active configuration without process restart
- an invalid reload MUST leave the previous valid configuration authoritative
- reload MUST NOT corrupt committed state, staged state, or queue ownership
- reload-visible success and failure MUST be logged clearly

## Logging and diagnostics rule

The product MUST emit structured diagnostics for operator-relevant events.

At minimum, logs SHOULD make it possible to identify:

- startup and shutdown
- configuration reload success or failure
- downloader-stage failures
- processing-stage severe faults
- integrity-triggered recovery scheduling
- the affected item, subsystem, phase, and error text when known

Operators SHOULD NOT need to infer subsystem ownership from ambiguous free-form
messages.

HTTP handlers MUST be wrapped with same-goroutine panic recovery that logs
structured request context and returns a 500 response. Recovery does not make
handler panics acceptable control flow; it exists to keep one bad request from
terminating or silently dropping the operator/public HTTP contract.

Public/API rate limiting MUST avoid unbounded background cleanup goroutines.
Per-client limiter state MAY be pruned lazily from request handling, and the
limiting algorithm SHOULD use a standard token-bucket implementation rather
than a fixed-window counter.

The public JSON/static artifact cache MUST be resource-bounded. Published
metadata, history, comparison, retention, insight, entity, sitemap, robots,
LLM, and direct JSON/CSV/XML/TXT/HTML artifact routes may use the cache, but
the cache must enforce configured maximum entries, configured total bytes, and
a configured per-file cache eligibility limit. Files above the per-file limit
must be served from disk without being retained. Raw `.ipset` and `.netset`
downloads remain streaming paths and must not enter this cache.

## HTTP serving policy

### CORS

Public endpoints (all paths except `/api/v1/admin/` and `/admin`) respond to `GET` and `OPTIONS` requests with:

- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: GET, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type`

The Streamable HTTP MCP endpoint at `/mcp` is a public endpoint with a
transport-specific CORS contract. It responds with:

- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: GET, POST, DELETE, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type, Mcp-Session-Id, MCP-Protocol-Version, Last-Event-ID`
- `Access-Control-Expose-Headers: Mcp-Session-Id`

Admin endpoints explicitly exclude CORS to prevent cross-origin credential theft
from basic-auth protected surfaces. Shared middleware MAY answer admin
`OPTIONS` requests with `204 No Content`, but MUST NOT emit wildcard CORS
headers or advertise cross-origin admin access.

### Response compression

The server applies gzip compression to responses for paths matching:

- `/api/` prefix
- `/static/` prefix
- `.json`, `.xml`, `.txt`, `.csv`, `.js`, `.css`, `.html` suffixes
- root path `/`

Compression is conditional on the `Accept-Encoding: gzip` request header. When enabled, responses include `Content-Encoding: gzip` and `Vary: Accept-Encoding`.

### IP search rate limiting

IP search endpoints (`/api/v1/search`, `/api/v1/query`) apply per-client rate limiting of 10 requests per minute, independent of the general API rate limit. This prevents IP lookup abuse without affecting other API consumers.

### General API rate limiting

The HTTP middleware applies per-client rate limiting of 240 requests per minute
to all paths whose URL starts with `/api/` or `/mcp`. This includes public API
routes, MCP, and authenticated admin API routes. `/healthz`, direct static or
artifact routes, raw `/files/` downloads, and SPA/static routes outside `/api/`
and `/mcp` are outside this limiter. This is independent of the search-specific
rate limit.

## Admin visibility rule

Operator-relevant daemon work MUST be visible in the admin surface.

If the daemon performs background work outside the normal downloader or
processing queues, the admin API/UI MUST expose that work explicitly rather
than leaving it invisible.

The product MUST NOT rely only on logs for routine operator-facing awareness of
such background work.

Background work MUST also remain resource-bounded by its own concurrency limit.
It MUST NOT implicitly expand to machine-wide parallelism just because the work
is deferred.

Startup integrity repair MUST be conservative. If existing public entity
artifacts are usable but a guarded startup check finds a broad country/ASN
detail refresh, the daemon SHOULD defer automatic repair and expose the finding
instead of silently spending startup/background resources on tens of thousands
of actor rewrites. Full bootstrap remains allowed when the entity artifact tree
is missing, version-incompatible, or otherwise unusable.

## Resource telemetry rule

The daemon MUST make material CPU, memory, network, and I/O activity measurable.

The product MUST NOT rely on operator intuition to identify resource waste.
Resource-relevant work SHOULD emit monotonic counters and operation timings that
can be compared across admin-status snapshots.

Diagnostic progress surfaces MUST define their unit of work. A new progress
counter, structured log field, or admin-status field MUST make clear whether it
counts feeds, files, operations, IPs, entries, bytes, or another bounded unit.
When total work is known, the same surface SHOULD include completed work, total
work, completion percentage, elapsed time, and rate in that unit per second.
Run and phase summaries SHOULD include phase-scoped operation counts and rates
instead of only process-wide cumulative counters.

Every material engine phase SHOULD expose live active-operation progress for
bounded loops while that phase is running. When a phase has no meaningful
bounded work unit, it MUST still be visible through phase timing; the product
MUST NOT invent synthetic totals that would hide the real absence of a bounded
operation.

Operator status for a running engine batch SHOULD expose the whole batch and
the phase plan, not only the currently active worker names. When the exact
post-source phase list cannot be known until source results are classified,
the status MAY mark the phase plan as tentative; it MUST NOT present a guessed
phase total as final.

Parsing large local source bodies is material engine work. Parser progress
SHOULD expose byte, line, accepted-range, and hostname-resolution counters so a
source phase that spends minutes on local input can be distinguished from
retention, finalization, or downstream phase work.

At minimum, telemetry SHOULD cover:

- download requests, HTTP statuses, and transferred bytes
- processing and heavy-analysis operation counts and timings
- `iprange` primitive operations, including text/binary load, text/binary save,
  merge, compare, diff, intersect, exclude, union, count-unique, optimize,
  contains, and binary-search counters
- pairwise-comparison candidate counts, overlap checks, and skip reasons
- latest-set opens, fallback text parses, and mmap/file byte estimates
- entity sidecar reads/writes, pending sidecar reads/writes, affected
  country/ASN counts, and range-attribution work
- public request-time aggregation work that still reads many artifacts
- admin polling endpoints, including request counts, response bytes, and
  build/write timings
- process CPU, memory, file-descriptor, and process I/O snapshots where the
  platform exposes them

Adding telemetry does not make waste acceptable. Counters exist to prove where
work is happening, prioritize fixes, and verify that later changes reduce the
operational profile.

OpenTelemetry metric identity MUST be bounded and operationally meaningful.
Bounded labels such as feed name, status, HTTP route, component, operation,
engine phase, and static source/downloader type may be used when they provide
direct diagnostic value. Ephemeral runtime quantities MUST NOT be metric labels
or metric resource attributes. This includes process IDs, queue depths, batch
sizes, selected-feed counts, processor-step counts, input byte counts, fan-in
counts, and other values that are measurements or runtime state rather than
stable identity.

Default OpenTelemetry metric instruments MUST be allow-listed. Detailed
internal operation timings may remain in admin snapshots, traces, or logs, but
they MUST NOT automatically become Prometheus/OTLP metric families unless an
area-specific metric model records the operator question, alerting use case,
allowed labels, and cardinality impact.

OpenTelemetry metrics MUST use a stable service resource identity by default.
Automatic host, OS, process resource attributes, and service-version values MAY
be present on traces and logs, but MUST NOT be attached to metrics unless the
operator explicitly adds them through standard OpenTelemetry resource
environment configuration. Host, process CPU, memory, file-descriptor, and I/O
details remain available through admin status and host/process monitoring
instead of metric identity labels.

HTTP API metrics MUST follow a low-cardinality RED model. The default
OpenTelemetry HTTP server metric is `http.server.request.duration`; it MUST
keep only `http.route`, `http.request.method`, and
`http.response.status_code` labels. HTTP routes MUST be normalized templates
such as `/api/v1/sets/{name}/search` or
`/api/v1/admin/feeds/{name}/recheck`, never raw feed names, provider names,
client IPs, query strings, or arbitrary probe paths. Default OpenTelemetry
export MUST drop HTTP request/response body-size instruments unless a later
area-specific metric model reintroduces a bounded byte signal.

API calls that trigger dynamic compute, recalculation, recheck, reprocess, or
rebuild work MUST use bounded `api.recalculation.requests` and
`api.recalculation.targets` metrics. Their labels are limited to
`api.surface`, `api.action`, and `api.result`; target counts are metric values,
not labels. Feed names, artifact names, provider names, search terms, client
addresses, and target-count labels MUST NOT be attached.

The daemon MUST expose `GET /metrics` on the admin surface as a Prometheus
scrape endpoint without admin basic authentication. In split-listener mode,
this route MUST be served only by the admin listener. In shared-listener mode,
the shared listener exposes the route, so operators MUST treat listener binding
and network access control as the protection boundary for this endpoint. The
Prometheus endpoint MUST use the same OpenTelemetry metric views and stable
metric resource identity as OTLP metric export. It SHOULD use an application
registry rather than the process-global Prometheus registry so the scrape
surface represents update-ipsets telemetry rather than unrelated runtime
collectors.

The daemon MUST provide OpenTelemetry-compatible export for traces, metrics, and
logs. OpenTelemetry export is opt-in and MUST be enabled when either:

- `UPDATE_IPSETS_OTEL` is `1`, `true`, or `enabled`
- an OTLP endpoint environment variable is present, such as
  `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`,
  `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT`, or `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT`

`UPDATE_IPSETS_OTEL=0`, `false`, or `disabled` MUST disable export even if
endpoint variables are present. When enabled, the daemon MUST preserve the
existing local log output while also sending logs through the OpenTelemetry log
bridge.

The daemon MUST support both OTLP HTTP/protobuf and OTLP/gRPC exporters.
Protocol selection MUST accept `UPDATE_IPSETS_OTEL_PROTOCOL` first, then
`OTEL_EXPORTER_OTLP_PROTOCOL`; supported values are `http/protobuf` and `grpc`.
The default remains `http/protobuf` for generic OTLP collectors. OTLP/gRPC
endpoint environment values MUST include an `http` or `https` scheme because
the Go OpenTelemetry gRPC exporters reject bare `host:port` values.

When export is enabled, the daemon MUST use the standard OpenTelemetry resource
environment detector and default OTLP exporter environment configuration. This
means standard variables such as `OTEL_SERVICE_NAME`,
`OTEL_RESOURCE_ATTRIBUTES`, OTLP headers, timeout, compression, insecure, and
TLS/mTLS certificate variables are honored by the OpenTelemetry SDK/exporters
in addition to the project-specific enablement and signal-control variables.

The installed service MUST default to direct local Netdata export:

- `UPDATE_IPSETS_OTEL=1`
- `UPDATE_IPSETS_OTEL_PROTOCOL=grpc`
- `OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317`
- `OTEL_METRIC_EXPORT_INTERVAL=10000`
- `OTEL_TRACES_EXPORTER=none`

The metric export interval MUST be configurable through
`UPDATE_IPSETS_OTEL_METRIC_INTERVAL` or `OTEL_METRIC_EXPORT_INTERVAL`; integer
values are milliseconds, and duration strings such as `10s` MUST also be
accepted. Individual OpenTelemetry signals MUST be suppressible with
`UPDATE_IPSETS_OTEL_TRACES`, `UPDATE_IPSETS_OTEL_METRICS`,
`UPDATE_IPSETS_OTEL_LOGS`, or the standard `OTEL_<SIGNAL>_EXPORTER=none`
variables.

Primitive operation metrics MUST collapse operation-specific names into a small
stable surface. The default `iprange` OpenTelemetry namespace MUST include only
`iprange.operations` and `iprange.operation.duration_ms`, with labels limited
to `ip.version` and `iprange.operation`.

Queue and phase metrics MUST also use stable family names:

- scheduler queue admissions, starts, completions, depth, batch size values,
  and batch latency use the `scheduler.*` metric families
- downloader outcomes use `download.fetches`, `download.fetch.bytes`,
  `download.fetch.duration_ms`, and `download.errors`
- engine runs and phases use `engine.runs`, `engine.run.duration_ms`,
  `engine.running`, `engine.phase.duration_ms`, and `engine.phase.current`

Frequently polled HTTP handlers and background batch processors MUST be treated
as hot paths. They MUST avoid duplicating full-cache snapshots inside per-row
loops and SHOULD build each logical snapshot once per request or batch, then
reuse indexed views for row rendering.

## Write-failure and disk-exhaustion rule

Write failures, including disk exhaustion, MUST be treated as hard operational
errors.

Rules:

- incomplete writes remain temporary or failed state only
- committed authoritative files MUST remain untouched when a replacement write
  cannot complete safely
- staged replayable state MUST be preserved whenever it already exists
- the product MUST surface the fault clearly rather than silently claiming a
  successful publication

## Bounded-work rule

The product MUST favor bounded work over open-ended recomputation.

This means:

- only affected feeds SHOULD be reprocessed when possible
- only affected peer comparison rows SHOULD be refreshed when possible
- incremental peer comparison SHOULD reuse exact-overlap results for unchanged
  normalized feed pairs through a drop-safe internal ledger, while still
  rebuilding public rows from current metadata and lineage
- only affected country/ASN entity-detail payloads SHOULD be refreshed when
  possible, and any expensive country<->ASN intersection work SHOULD be reused
  from per-feed sidecars instead of repeated once per entity page
- ordinary feed-update background entity patching MUST NOT reopen feed sets or
  recompute country<->ASN range intersections; changed-feed entity sidecars
  SHOULD be precomputed during the processing run and consumed as pending
  private sidecars by the background patcher
- country/ASN entity refreshes after ordinary feed updates SHOULD be surgical
  per-feed deltas over existing entity artifacts, not rebuilds over the whole
  feed catalog
- surgical country/ASN refreshes SHOULD suppress unchanged actor rewrites after
  computing the actor-local patch; unchanged per-feed actor contributions MUST
  NOT trigger cosmetic patch work in the ordinary incremental path
- repeated ordinary feed-update and health-transition entity refresh requests
  SHOULD be coalesced by feed name before the next background refresh wave
  starts
- insights regeneration SHOULD follow the same affected-feed/provider fan-out
  as the comparison files, plus missing insight files; it MUST NOT sweep every
  public feed after an ordinary single-feed update
- critical-infrastructure overlap regeneration SHOULD be limited to affected
  public feeds after ordinary feed updates; a full public-feed sweep is allowed
  only when the reference provider set itself changes
- repeated no-op work SHOULD be suppressed when it has no externally visible
  effect
- global recomputation SHOULD be reserved for cases where the contract truly
  requires it, such as provider-wide semantic changes

## Actor consistency rule

The product MUST avoid wasting resources on cross-actor synchronization when
bounded eventual consistency is sufficient.

Rules:

- consistency MUST be synchronized within one actor's committed artifact set
  - a feed page and its feed-local published outputs must agree with that feed
  - a country page must be internally self-consistent
  - an ASN page must be internally self-consistent
- consistency MAY be eventual across actors
  - a feed update does not require every affected country/ASN page to be
    refreshed inline before the feed-processing run can complete
  - delayed cross-actor entity refresh work MUST be visible in the admin surface
    and bounded by background-maintenance concurrency

The product MUST keep an explicit full entity rebuild path for operator repair
and recovery. That path MAY do broad work, but it MUST remain distinct from the
ordinary incremental feed-update flow.

## Disk-first durable-state rule

Large durable inputs and outputs MUST be treated as on-disk state first, not as
heap-resident application state.

This applies to at least:

- downloader-stage acquisitions
- staged feed bodies
- committed feed bodies
- provider datasets
- large committed canonical feed bodies

## Separation-of-concurrency rule

Different workload classes MUST keep separate concurrency controls when their
resource characteristics differ materially.

At minimum, the product MUST preserve separate control over:

- downloader concurrency
- feed-processing concurrency
- heavy-phase/global-analysis concurrency
- background-maintenance concurrency

Background-maintenance work includes startup/reload artifact repair,
health-transition artifact refreshes, and similar deferred daemon tasks.

If a default is needed for background-maintenance concurrency, it SHOULD be
single-threaded unless an operator explicitly raises it.

## Public-truth freshness rule

Published public facts MUST reflect the latest known settled local state.

This means:

- public feed pages MUST represent the latest committed outputs
- pairwise comparison surfaces MUST reflect the latest known state of either
  side
- provider-enriched views MUST reflect the latest committed provider data once
  corresponding reprocessing has completed

The product MUST NOT knowingly present stale peer-comparison facts as current.

## Failure-containment rule

The product SHOULD degrade by doing less speculative or optional work before it
risks losing committed correctness.

In practice:

- keep committed good data when refresh fails
- retain staged replayable local input after processing failure
- surface severe faults clearly instead of masking them with misleading
  "healthy" publication claims

## Operator-visibility rule

The operator surface MUST expose real state, not synthetic or misleading
internal narratives.

In particular:

- downloader and processing queues MUST remain distinct
- terminal failures MUST not masquerade as active work
- runtime state SHOULD be explained in terms that map to real subsystem
  behavior

## Performance contract

Performance optimization MUST prioritize:

- bounded memory
- bounded recomputation
- suppression of known no-op work
- keeping public and admin surfaces responsive while heavy background work
  continues

Entity artifact materialization MUST build expensive shared lookup state at batch
scope rather than row scope. In particular, feed health/effective-entry lookup
state MUST NOT snapshot the full cache once per feed row while generating
country/ASN detail payloads.

Helpers that take a fresh full-cache snapshot MUST make that cost visible in
their names and comments. Batch/loop code MUST use an explicitly scoped
resolver/classifier and MUST NOT call single-entry fresh-snapshot helpers inside
loops.

The product MUST NOT optimize for implementation convenience at the cost of
breaking these operational principles.

## Review guidance

A reviewer checking compliance with this document SHOULD be able to answer:

- does public browsing reuse published local artifacts?
- do downloader and processing remain separate loops?
- does startup avoid broad mandatory recomputation?
- are expensive tasks bounded to affected work where possible?
- are remote APIs avoided for repeated public views?
