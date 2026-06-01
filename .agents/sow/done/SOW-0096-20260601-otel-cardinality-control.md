# SOW-0096 - OpenTelemetry Metric Cardinality Control

## Status

Status: completed

Sub-state: area-by-area telemetry redesign implemented, validated, installed,
and ready for the closure commit.

## Requirements

### Purpose

Reduce OpenTelemetry metric cardinality so update-ipsets remains fit for
purpose as an operationally cheap, pragmatic, cache-first service. A few
thousand useful, bounded metric series are acceptable. Ephemeral or unbounded
values such as process IDs, queue depth, batch size, selected-count, and similar
runtime counters must not become metric labels. Expose an admin-surface
Prometheus scrape endpoint so operators can directly measure the exact
post-policy series set without relying only on downstream collector metadata.

The metric surface itself must be designed before further implementation.
Telemetry must have explicit operational goals, use bounded labels, and follow
standard service/resource models such as RED, USE, and the SRE golden signals.
The application should expose fewer than 100 useful metric names in total, with
only a small set of per-feed metrics and a holistic component-level set for each
major module. API methods are first-class operations: public/admin API calls
must be measured by method/route/status/latency, and API calls that trigger
recalculation instead of serving cached artifacts must be separately visible.

Telemetry redesign process:

1. Identify all application areas that need monitoring.
2. For each area, understand what it does, decide the operational metrics
   required using RED, USE, and golden-signal framing, then model those metrics
   into the code.
3. Work exclusively area by area until the selected area is complete. Do not
   batch unrelated telemetry changes.
4. Verify each area before moving to the next.

Cardinality must stay low. The default surface should have a small number of
per-feed metrics and low-cardinality metrics per component, feature, or
function.

### User Request

The user reported that the application generated about 120k unique
OpenTelemetry time-series and called this wasteful and impragmatic. The user
then clarified that a few thousand metrics are acceptable and the real defect is
dynamic labels: PID, queue depth, batch size, and other ephemeral values cannot
be metric labels. The user then requested an unauthenticated admin `/metrics`
endpoint for Prometheus and asked to measure the unique time-series exposed by
that endpoint. After the endpoint showed 364 Prometheus metric names, the user
rejected the broader implementation as meaningless ad hoc telemetry and required
a designed metric model: a few important per-feed metrics, component-level
operational metrics, and standard RED/USE/golden-signal framing. The user then
clarified that API methods are important operations to measure, especially API
calls that trigger recalculation rather than using cached artifacts.
The user then defined the redesign workflow: identify monitorable areas first,
then repeat area by area, understanding the area, selecting RED/USE/golden
metrics, modeling them in code, and verifying before moving on.

### Assistant Understanding

Facts:

- Local Netdata evidence shows OpenTelemetry metrics from update-ipsets have
  high cardinality in the last 24 hours.
- `otel.download.fetch` exposes `feed.name` with 311 total observed values and
  1,155 total metric instances in the local Netdata view.
- `otel.sources.finalize.duration_ms.bucket` exposes `feed.name` with 402 total
  observed values and 1,209 total metric instances before multiplying by
  histogram bucket dimensions.
- The OpenTelemetry SDK supports metric views with attribute filters.
- The user accepts bounded high-cardinality dimensions when they have clear
  operational value.
- The current metric surface has 364 Prometheus sample metric names, 200
  normalized logical metric names, and 8,252 series in the local scrape.
- The largest logical groups are entity, engine, iprange, metadata, processor,
  sources, and download metrics. Many of these are low-level internal operation
  timings rather than metrics tied to an operator question or alert.
- The dynamic-label cardinality defect was addressed, but the resulting metric
  inventory is still not fit for purpose because it lacks a telemetry design.

Inferences:

- The reported 120k unique time-series is consistent with bounded feed labels
  being multiplied by histogram buckets and then further multiplied by
  ephemeral resource/measurement labels.
- Feed-specific metric detail may remain acceptable if it is bounded by the
  catalog and does not combine with unbounded runtime identity.

Unknowns:

- The exact post-fix series count is not known until the local OTEL collector is
  checked after implementation.
- Whether the 100-metric target should be enforced as Prometheus sample names
  or normalized logical metric families. Given the user raised the issue from
  the 364 Prometheus metric-name count, the current working interpretation is a
  Prometheus sample-name budget unless the user decides otherwise.

### Acceptance Criteria

- OpenTelemetry metrics have a bounded attribute policy documented in specs and
  operator docs.
- Process IDs, queue depths, batch sizes, selected-feed counts, processor-step
  counts, and similar runtime quantities no longer create distinct metric
  time-series by default.
- Service-level counters, gauges, and duration histograms remain exported for
  feed state, HTTP/API paths, artifact cache, scheduler, downloader, processor,
  engine phases, integrity, background work, config/runtime cache, daemon
  liveness, and iprange operations.
- Bounded labels such as feed name, download status, HTTP route/status,
  processor mode/status/temp kind, engine phase, run reason/status, component,
  operation, and result remain available where they answer an operator question.
- Focused Go tests prove the metric SDK configuration filters unwanted
  attributes or the observability helpers do not pass them to metric
  instruments.
- Real-use validation after install confirms representative high-cardinality
  contexts collapse to bounded instances.
- `/metrics` is served on the admin surface without basic authentication.
- When a separate admin listener is configured, `/metrics` is available on the
  admin listener and not on the public-only listener.
- The Prometheus endpoint uses the same bounded OpenTelemetry metric identity
  policy as OTLP export.
- Validation measures unique Prometheus time-series by counting distinct sample
  identities in the `/metrics` response.
- A redesigned telemetry inventory is produced before additional metric code
  changes.
- Every retained metric has an explicit operator question, alerting/use case,
  owner component, label policy, and standard model mapping.
- Per-feed metrics are limited to essential feed state and health signals such
  as current state, health state, entries/IPs, errors, freshness, and last
  successful update.
- Component metrics cover each major module holistically using RED for service
  operations, USE for resources/queues/workers, and golden-signal framing for
  latency, traffic, errors, and saturation.
- Default Prometheus exposure stays below 100 metric names.
- Detailed internal timings that do not support a clear operational question
  are removed from default metrics or moved to traces/logs/admin snapshots.
- API request metrics retain bounded route/method/status identity.
- API-triggered recalculation work is visible as its own operation class, with
  cache-hit/cache-miss or artifact-served/recalculation-triggered semantics
  where the route contract can distinguish them.

## Analysis

Sources checked:

- `internal/observability/observability.go`
- `pkg/downloader/downloader.go`
- `pkg/engine/run_metrics.go`
- `pkg/scheduler/metrics.go`
- `pkg/engine/run.go`
- `docs/monitoring/telemetry-reference.md`
- `.agents/sow/specs/operating-principles.md`
- Local Netdata MCP output for update-ipsets OpenTelemetry contexts, with
  workstation-identifying host labels redacted from durable notes.
- `open-telemetry/opentelemetry-go @ 794c0724001cdbe38b40881f44f04190152306fe`
- `open-telemetry/opentelemetry-collector-contrib @ 6698bc24dc8ee69f839f16eb9950b99b074f8191`

Current state:

- `pkg/downloader/downloader.go:113-134` records `feed.name`,
  `download.downloader`, `http.response.status_code`, and `download.status` on
  downloader metrics and also creates status-specific metric names. These are
  bounded by catalog/config/status codes and are not the immediate defect under
  the user's clarified policy.
- `pkg/engine/run_metrics.go:100-104` emits per-feed operation duration
  histograms with `feed.name`.
- `pkg/scheduler/metrics.go:80`, `pkg/scheduler/metrics.go:123`,
  `pkg/scheduler/metrics.go:136`, `pkg/scheduler/metrics.go:149`, and
  `pkg/scheduler/metrics.go:161` attach queue depth or batch size as metric
  attributes.
- `pkg/engine/run.go:22-39` attaches selected-feed count and run flags/reason
  to the engine run metric path.
- `internal/observability/observability.go:88-99` adds host, OS, process, and
  service-version resource attributes; process ID and dirty version values
  churn across daemon restarts.
- `internal/observability/observability.go:331-369` forwards every provided
  attribute to metric instruments without a project-level metric attribute
  policy.

Risks:

- Dropping all metric attributes would be an overcorrection and would remove
  useful bounded breakdowns such as feed, download status, and HTTP status.
- Keeping feed names in metrics may still produce more series than desired
  because histograms multiply by bucket dimensions; this needs post-fix
  measurement before another policy change.
- Removing process resource attributes affects metric, trace, and log resource
  identity unless implementation separates resources per signal.
- Metric views can drop measurement attributes, but they do not filter resource
  attributes such as process ID.

## Telemetry Area Inventory

Status: area inventory identified; area-specific metric models implemented.

The redesign must proceed area by area. This inventory defines the areas that
need monitoring and the code evidence for each area. It does not approve
batched implementation.

1. Public and admin API operations.
   - What it does: serves public APIs, admin APIs, MCP, health, Prometheus
     scrape, and operator-triggered actions.
   - Evidence: `pkg/web/routes.go:25-58` registers public API routes;
     `pkg/web/routes.go:266-290` registers admin API routes and admin actions;
     `pkg/web/server.go:110-115` wraps HTTP serving with `otelhttp`.
   - Monitoring need: RED metrics by route/method/status, plus explicit
     operation classification for cache/artifact reads versus dynamic work or
     operator-triggered recalculation.
   - Priority: first, because the user called API methods and recalculation
     API calls important operations.

2. Public artifact serving and web artifact cache.
   - What it does: serves generated JSON/CSV/XML/TXT/HTML artifacts, raw
     ipset/netset bodies, SPA fallback, and the bounded in-memory artifact
     cache.
   - Evidence: `pkg/web/routes.go:394-488` handles raw files, direct
     published artifacts, and direct raw feeds; `pkg/web/cache.go:78-90` serves
     cached and uncached files; `pkg/web/cache.go:173-217` resolves rooted
     files and decides cache hit/miss/oversize behavior.
   - Monitoring need: USE/cache metrics for cache hits, misses, oversize
     streams, errors, evictions, and served bytes without per-file labels.

3. Feed state and health inventory.
   - What it does: exposes feed state, health, entries, unique IP counts,
     timestamps, failures, and visibility/redistribution status.
   - Evidence: `pkg/engine/public_catalog.go:67-83` builds public feed
     summaries; `pkg/engine/public_catalog.go:86-151` maps cache/config state
     to feed summary fields; `pkg/feedhealth/feedhealth.go:78-165` classifies
     feed health.
   - Monitoring need: small per-feed gauges for state, health, entries,
     unique IPs, errors/failure streak, freshness, and last successful
     publication.

4. Scheduler and work queues.
   - What it does: admits download and processing work, tracks active/waiting
     queues, deferrals, processing batches, and queue snapshots.
   - Evidence: `pkg/scheduler/metrics.go:11-27` defines scheduler-visible
     counters and saturation snapshots; `pkg/scheduler/metrics.go:74-165`
     records enqueue/start/finish/requeue/batch activity.
   - Monitoring need: USE/golden metrics for queue depth, saturation,
     admission/requeue/defer rates, active workers, batch latency, and errors.

5. Downloader/source acquisition.
   - What it does: fetches HTTP/file/internal source bodies, follows safe
     redirect policy, tracks download status, HTTP status, body size, and
     failures.
   - Evidence: `pkg/downloader/downloader.go:111-136` wraps fetches with
     status/duration/body metrics and spans.
   - Monitoring need: RED metrics per bounded downloader/status class and
     optionally feed-level terminal state; avoid status-specific metric names.

6. Processor/source normalization.
   - What it does: runs processor steps over bytes or streams and turns source
     bodies into normalized feed data.
   - Evidence: `pkg/processor/processor.go:104-146` runs byte processors and
     step handling; `pkg/processor/run_stream.go:20-34` runs streaming
     processors.
   - Monitoring need: RED metrics for processor runs by bounded mode/status and
     perhaps step category; detailed per-step timings are trace/admin-detail
     data unless needed for alerts.

7. Engine processing and publication pipeline.
   - What it does: owns full runs, phases, selected/recheck/reprocess reasons,
     heavy phases, metadata/insights, and publish.
   - Evidence: `pkg/engine/run.go:18-63` wraps each run and persists run
     outcome; `pkg/engine/run_phase.go:5-16` defines phases;
     `pkg/engine/run_metrics.go:72-83` finalizes phase timing.
   - Monitoring need: golden signals for run success/error, run duration,
     current phase, phase duration, publish failures, and cancellation/conflict.

8. Integrity and recalculation/recovery actions.
   - What it does: checks generated artifacts, computes recovery plans, queues
     recheck/reprocess, and handles entity artifact rebuild requests.
   - Evidence: `pkg/web/integrity.go:93-101` serves integrity reports;
     `pkg/web/integrity.go:159-170` handles entity rebuild POST requests;
     `pkg/web/integrity.go:269-331` computes integrity recovery and schedules
     recheck/reprocess.
   - Monitoring need: explicit operation metrics for integrity checks,
     findings, scheduled rechecks/reprocesses, rebuild queue acceptance/conflict,
     and recovery outcomes.

9. Entity artifacts and background work.
   - What it does: maintains country/ASN entity sidecars and background
     refresh/rebuild work, including startup/reload/operator triggers.
   - Evidence: `pkg/web/server.go:202-230` runs startup integrity and recovery
     admission before the scheduler; entity rebuild is exposed through
     `pkg/web/integrity.go:159-170`.
   - Monitoring need: queue/saturation/error metrics for background workers and
     entity rebuild/refresh outcomes, without per-entity labels.

10. Config, cache state, and daemon runtime.
    - What it does: loads config, supplemental directories, runtime cache, and
      maintains process/server lifecycle.
    - Evidence: `pkg/engine/engine.go:214-284` loads config and cache during
      engine construction; `pkg/engine/engine.go:341-384` reloads config and
      rebuilds runtime helpers; `pkg/web/server.go:187-205` validates runtime
      options and starts scheduler/server ownership.
    - Monitoring need: low-cardinality config/cache load success/error,
      reload count/error, daemon up, server listener health, and process-level
      USE signals only when they have operator value.

## Area 1 Metric Model - Public And Admin API Operations

Status: selected for implementation.

Purpose:

- Give operators a clear RED view of API methods.
- Make API calls that trigger recalculation or dynamic compute explicit.
- Remove ad hoc per-handler API metrics from the default metric surface.

Operational questions:

- Which API routes are receiving traffic?
- Which API routes are slow?
- Which API routes are returning errors?
- Which API calls schedule recalculation, recovery, recheck, reprocess, or
  rebuild work?
- Which public API calls perform dynamic compute instead of serving cached
  artifacts?

Retained default metrics:

1. `http.server.request.duration`
   - Model: RED latency plus request rate/error rate through histogram
     count/sum/buckets.
   - Labels: `http.route`, `http.request.method`,
     `http.response.status_code`.
   - Cardinality: bounded by route templates, HTTP methods, and status codes.
   - Reason: this is the primary API method metric.

2. `api.recalculation.requests`
   - Model: API-triggered recalculation/dynamic-work rate.
   - Labels: `api.surface`, `api.action`, `api.result`.
   - Allowed `api.surface`: `public`, `admin`.
   - Allowed `api.action`: bounded verbs such as `compose`, `search`,
     `feed_search`, `run_due`, `feed_recheck`, `feed_reprocess`,
     `artifact_recheck`, `integrity_reprocess`, and `entity_rebuild`.
   - Allowed `api.result`: `ok`, `error`, `scheduled`, `conflict`,
     `rejected`, `in_progress`, `clean`.
   - No feed, artifact, provider, client, query, or target-count labels.

3. `api.recalculation.targets`
   - Model: number of feeds/artifacts queued by recalculation/recovery API
     calls.
   - Labels: same as `api.recalculation.requests`.
   - Cardinality: same as above; target count is the metric value, never a
     label.

Dropped from default API metrics:

- `http.server.request.body.size`
- `http.server.response.body.size`
- ad hoc custom metrics under `http.admin_*`, `http.home_*`,
  `http.compare_set.*`, and `http.entity_artifact.*`

Implementation constraints:

- Route labels must use normalized route templates, not raw paths.
- `/api/v1/ipsets/*` may remain distinct where it represents a real API alias,
  but names/providers/feed IDs must not appear in route labels.
- Handler-specific body size metrics are not default metrics; response size can
  be handled later in the artifact/cache area if it has an operator use case.
- Detailed timing inside handlers stays in traces, logs, admin snapshots, or a
  later explicitly designed area.

Validation for this area:

- Focused tests prove HTTP body-size instruments are dropped and HTTP route
  labels are allow-listed to route/method/status.
- Route-normalization tests cover public set actions and admin action routes.
- API recalculation tests prove selected routes increment only bounded
  `api.recalculation.*` labels.
- Live `/metrics` scrape shows the dropped API metrics are absent and the
  remaining API metric names are within this area budget.

## Pending Decision - Area 2 Metric Model

Area: public artifact serving and web artifact cache.

Context:

- Area 1 already provides RED request rate/error/latency for artifact-serving
  routes through `http.server.request.duration`.
- `pkg/web/routes.go:400-421` serves `/files/{name}` raw feed downloads.
- `pkg/web/routes.go:455-471` serves direct published artifacts through the web
  artifact cache.
- `pkg/web/routes.go:483-493` serves direct raw `.ipset` and `.netset`
  downloads without adding them to the JSON/static artifact cache.
- `pkg/web/cache.go:78-99` serves cached, uncached, and rooted cached files.
- `pkg/web/cache.go:173-220` decides rooted cache hit, miss, oversize, and
  read behavior.

Decision 1: Area 2 default OpenTelemetry metric model.

Option A - cache USE metrics only (recommended):

- Keep HTTP request rate/error/latency in the Area 1 HTTP RED metric.
- Add only cache health/use metrics:
  - `web.artifact.cache.lookups` with bounded `cache.result` values such as
    `hit`, `miss`, `oversize`, and `error`.
  - `web.artifact.cache.evictions` with bounded `cache.reason` values such as
    `max_entries` and `max_bytes`.
  - `web.artifact.cache.entries` gauge.
  - `web.artifact.cache.bytes` gauge.
- Do not label by feed, artifact name, filename, provider, extension, or path.
- Implication: low metric count and enough signal for cache saturation and
  broken cache behavior.
- Risk: per-file hot spots stay out of OpenTelemetry metrics; operators must
  use HTTP route metrics, logs, or admin detail when investigating a specific
  artifact.

Option B - cache metrics plus artifact-serving counters:

- Add Option A plus serving counters by bounded route family/result, such as
  `published_artifact`, `raw_feed`, `spa`, and `methodology`.
- Implication: more direct artifact-serving traffic visibility.
- Risk: duplicates much of Area 1 HTTP RED, adds metric names/series, and
  tempts future path/feed labels.

Option C - no Area 2 OpenTelemetry metrics:

- Rely only on Area 1 HTTP RED and keep cache details in admin snapshots/logs.
- Implication: smallest metric surface.
- Risk: cache saturation, oversize streaming, and eviction churn are not
  alertable from OpenTelemetry.

Recommendation:

- Choose Option A. It follows USE for the cache resource without duplicating
  HTTP RED, keeps cardinality low, and gives operators alertable saturation and
  cache-behavior signals.

User decision:

- Proceed without further user approval and answer these questions from the
  code evidence. Selected Option A.

## Area 2 Metric Model - Public Artifact Serving And Web Artifact Cache

Status: implemented and validated.

Purpose:

- Measure artifact-cache USE signals without duplicating Area 1 HTTP RED.
- Keep artifact/file/feed identity out of cache metric labels.

Retained default metrics:

1. `web.artifact.cache.lookups`
   - Model: cache traffic/error behavior.
   - Labels: `cache.result` with `hit`, `miss`, `oversize`, or `error`.
   - No path, feed, provider, extension, or artifact labels.

2. `web.artifact.cache.evictions`
   - Model: cache saturation/churn.
   - Labels: `cache.reason` with `max_entries` or `max_bytes`.

3. `web.artifact.cache.entries`
   - Model: current cache utilization.
   - Labels: none.

4. `web.artifact.cache.bytes`
   - Model: current cache byte utilization.
   - Labels: none.

Validation:

- Unit tests cover hit, miss, oversize, error, eviction, and gauge recording
  without path/feed labels.
- Live `/metrics` scrape confirms these names appear only with bounded labels.

## Areas 3-10 Metric Models

Status: implemented and validated.

Area 3 - Feed state and health inventory:

- Purpose: a small per-feed operational inventory.
- Metrics:
  - `feed.state` gauge, label `feed.name`, numeric state code.
  - `feed.health.state` gauge, label `feed.name`, numeric health code.
  - `feed.entries` gauge, label `feed.name`.
  - `feed.unique_ips` gauge, label `feed.name`.
  - `feed.errors` gauge, label `feed.name`.
  - `feed.freshness.seconds` gauge, label `feed.name`.
  - `feed.last_success.timestamp` gauge, label `feed.name`.
- State codes: `0=unknown`, `1=disabled`, `2=never_run`, `3=running`,
  `4=ok`, `5=warning`, `6=error`.
- Health codes: `0=unknown`, `1=healthy`, `2=delayed`, `3=risky`,
  `4=unavailable`, `5=archived`, `6=empty`, `7=unmaintained`.
- No category, maintainer, URL, provider, license, redistributability, or
  error-text labels.

Area 4 - Scheduler and work queues:

- Purpose: USE/golden signals for queue admission, work rate, saturation, and
  batch latency.
- Metrics:
  - `scheduler.queue.admissions`, labels `scheduler.queue`,
    `scheduler.result`.
  - `scheduler.work.started`, label `scheduler.queue`.
  - `scheduler.work.completed`, label `scheduler.queue`.
  - `scheduler.queue.depth`, label `scheduler.queue`.
  - `scheduler.batch.items`, label `scheduler.queue`.
  - `scheduler.batch.duration_ms`, label `scheduler.queue`.
- Queue values: `download`, `processing`.
- Result values: `queued`, `deferred`, `requeued`.

Area 5 - Downloader/source acquisition:

- Purpose: RED for source acquisition without per-feed metric labels.
- Metrics:
  - `download.fetches`, labels `download.downloader`, `download.status`.
  - `download.fetch.bytes`, labels `download.downloader`,
    `download.status`.
  - `download.fetch.duration_ms`, labels `download.downloader`,
    `download.status`.
  - `download.errors`, labels `download.downloader`, `download.status`.
- No feed-name, URL, host, redirect target, or raw HTTP-code labels. Detailed
  HTTP status remains in logs/spans/admin detail; status codes are too noisy
  for the default metric surface here.

Area 6 - Processor/source normalization:

- Purpose: RED for processor runs and temporary file writes.
- Metrics:
  - `processor.runs`, labels `processor.mode`, `processor.status`.
  - `processor.run.duration_ms`, labels `processor.mode`,
    `processor.status`.
  - `processor.temp.writes`, label `processor.temp.kind`.
  - `processor.temp.write.duration_ms`, label `processor.temp.kind`.
- Mode values: `memory`, `stream`.
- Status values: `ok`, `error`.
- No step-name, input-size, output-size, or feed labels.

Area 7 - Engine processing and publication pipeline:

- Purpose: golden signals for full runs and bounded phase latency.
- Metrics:
  - `engine.runs`, labels `run.reason`, `run.status`.
  - `engine.run.duration_ms`, labels `run.reason`, `run.status`.
  - `engine.running` gauge, labels none.
  - `engine.phase.duration_ms`, label `engine.phase`.
  - `engine.phase.current` gauge, label `engine.phase`.
- No selected-count, feed-name, batch-size, or per-operation timing labels.

Area 8 - Integrity and recovery actions:

- Purpose: alertable integrity findings, check cost, and recovery scheduling.
- Metrics:
  - `integrity.checks`, labels `integrity.kind`, `integrity.result`.
  - `integrity.check.duration_ms`, labels `integrity.kind`,
    `integrity.result`.
  - `integrity.findings` gauge, label `integrity.kind`.
  - `integrity.recovery.targets`, label `integrity.action`.
- Kind values: `pipeline`, `entity`.
- Result values: `clean`, `issues`, `in_progress`, `error`.
- Action values: `recheck`, `reprocess`, `rebuild`.

Area 9 - Entity artifacts and background work:

- Purpose: background worker saturation and task outcomes.
- Metrics:
  - `background.tasks`, labels `background.component`,
    `background.result`.
  - `background.worker.wait.duration_ms`, label `background.component`.
  - `background.workers.active` gauge, label `background.component`.
  - `background.workers.limit` gauge, label `background.component`.
- Component values: `entity`, `other`.
- Result values: `started`, `completed`, `failed`.

Area 10 - Config, runtime cache, and daemon runtime:

- Purpose: startup/reload/cache health and daemon liveness without process
  identity labels.
- Metrics:
  - `config.loads`, label `config.result`.
  - `config.load.duration_ms`, label `config.result`.
  - `runtime.cache.operations`, labels `cache.operation`, `cache.result`.
  - `runtime.cache.operation.duration_ms`, labels `cache.operation`,
    `cache.result`.
  - `daemon.up` gauge, labels none.
- Result values: `ok`, `error`.
- Operation values: `load`, `save`.

Final default metric-surface policy:

- The SDK metric view MUST drop any instrument outside the designed Area 1-10
  allow-list.
- Dropped detailed timings remain available in admin snapshots, logs, traces,
  or code-local timing books when the code already records them there.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The application records ephemeral runtime values as metric attributes and
  shares process-rich OpenTelemetry resource identity with metrics.
  OpenTelemetry backends turn each unique metric name plus attribute/resource
  set into a distinct time-series. Bounded feed/status labels are acceptable
  under the clarified policy; PID, queue depth, batch size, selected count, and
  similar runtime quantities are not.

Evidence reviewed:

- Local Netdata MCP, 2026-06-01: `otel.download.fetch` had 311 feed-name label
  values and 1,155 metric instances in 24 hours.
- Local Netdata MCP, 2026-06-01:
  `otel.sources.finalize.duration_ms.bucket` had 402 feed-name label values and
  1,209 metric instances in 24 hours before bucket-dimension multiplication.
- `pkg/downloader/downloader.go:113-134` records feed names on downloader
  metrics.
- `pkg/engine/run_metrics.go:100-104` records feed names on per-feed operation
  histograms.
- `pkg/scheduler/metrics.go:80-161` records queue depth and batch size as
  metric attributes.
- `pkg/engine/run.go:35-39` records selected-feed count as a metric attribute.
- `internal/observability/observability.go:88-99` records process resource
  attributes, including process ID.
- `open-telemetry/opentelemetry-go @ 794c0724001cdbe38b40881f44f04190152306fe`
  `sdk/metric/instrument.go:139-150` documents `Stream.AttributeFilter` and
  recommends `NewAllowKeysFilter` for allow-listing metric attributes.
- `open-telemetry/opentelemetry-go @ 794c0724001cdbe38b40881f44f04190152306fe`
  `attribute/filter.go:13-49` implements allow-list and deny-list attribute
  filters.
- `open-telemetry/opentelemetry-collector-contrib @ 6698bc24dc8ee69f839f16eb9950b99b074f8191`
  `processor/cardinalityguardianprocessor/README.md:15-30` frames cardinality
  explosions as something to catch before they reach a TSDB and shows removing
  only the bad label while preserving the metric.

Affected contracts and surfaces:

- OpenTelemetry metric labels and resource labels.
- OpenTelemetry traces and logs if resource identity changes globally. Preferred
  implementation is to give metrics a stable resource while preserving richer
  trace/log resources when practical.
- Admin status metrics and slow-feed views.
- Operator documentation under `docs/monitoring/`.
- Operating-principles telemetry spec.
- Tests for `internal/observability` and any package-specific telemetry helper
  behavior touched by the implementation.

Existing patterns to reuse:

- `internal/observability` owns OpenTelemetry setup and generic metric helper
  functions; use this boundary for global metric attribute/resource policy.
- `pkg/iprange/otel.go` owns standalone iprange telemetry and must stay
  package-local because `pkg/iprange` must not import other project packages.
- Admin status already exposes current and last-run feed timing snapshots under
  engine metrics without requiring OpenTelemetry metric labels.
- Operator docs already separate admin snapshot telemetry from OpenTelemetry.

Risk and blast radius:

- Medium operational blast radius: the change alters exported metric identity
  for dynamic labels and existing dashboards using those labels will need
  updates. Dashboards based on bounded feed/status labels should survive.
- Low public API blast radius: public serving should not change.
- Medium validation risk: live collector behavior must be checked because SDK
  metric views, resource attributes, and Netdata's OTEL ingestion all affect
  final time-series identity.
- Low security risk if durable artifacts keep local host/user-identifying labels
  redacted.

Sensitive data handling plan:

- Do not write raw hostnames, usernames, customer data, secrets, tokens,
  non-private customer-identifying IPs, private endpoints, or proprietary
  incident details to SOWs, specs, docs, project skills, agent instructions, or
  code comments.
- Redact local workstation-identifying labels in durable artifacts.
- Use file paths, line numbers, metric names, and cardinality counts as
  evidence.

Implementation plan:

1. Add a metric attribute deny-list or equivalent metric view in
   `internal/observability` for ephemeral measurement attributes such as
   `scheduler.waiting`, `engine.batch.size`, `run.selected`,
   `processor.steps`, `processor.input.bytes`, and `iprange.sources`.
2. Update `.agents/sow/specs/operating-principles.md` and
   `docs/monitoring/telemetry-reference.md` to state the bounded metric-label
   contract.
3. Add focused tests for metric attribute filtering and resource-attribute
   behavior.
4. Install or run a local OTEL smoke check and verify representative contexts
   collapse to bounded instances.

Validation plan:

- `go test ./internal/observability ./pkg/downloader ./pkg/engine ./pkg/scheduler ./pkg/iprange -count=1`
- `make test` if the touched surface is broader than the focused packages.
- Real-use local Netdata MCP check for representative contexts after install:
  `otel.download.fetch`, `otel.download.fetch.duration_ms.bucket`,
  `otel.sources.finalize.duration_ms.bucket`, `otel.engine.batch.completed`,
  and `otel.iprange.*`.
- Same-failure scan for dynamic numeric metric attributes used in metric paths.

Artifact impact plan:

- AGENTS.md: likely no update unless a durable project-wide telemetry guardrail
  is selected.
- Runtime project skills: update `project-coding` with the durable telemetry
  cardinality rule.
- Specs: update `.agents/sow/specs/operating-principles.md`.
- End-user/operator docs: update `docs/monitoring/telemetry-reference.md` and
  possibly `docs/monitoring/monitoring-overview.md`.
- End-user/operator skills: none expected.
- SOW lifecycle: keep current until implementation and validation finish; close
  by setting `Status: completed`, moving to `.agents/sow/done/`, and committing
  together only if the user asks for a commit.

Open-source reference evidence:

- `open-telemetry/opentelemetry-go @ 794c0724001cdbe38b40881f44f04190152306fe`
  `sdk/metric/instrument.go:139-150`
- `open-telemetry/opentelemetry-go @ 794c0724001cdbe38b40881f44f04190152306fe`
  `attribute/filter.go:13-49`
- `open-telemetry/opentelemetry-collector-contrib @ 6698bc24dc8ee69f839f16eb9950b99b074f8191`
  `processor/cardinalityguardianprocessor/README.md:15-30`

Open decisions:

- None blocking. The user clarified that bounded metric series are acceptable
  and ephemeral labels are the defect to fix first.

## Pre-Implementation Gate - Prometheus Metrics Endpoint

Status: ready

Problem / root-cause model:

- Downstream OpenTelemetry collector metadata can retain old label values until
  retention expires. A direct Prometheus scrape endpoint provides a current,
  exact view of the application's exported metric series after the bounded-label
  policy is applied.

Evidence reviewed:

- `pkg/web/routes.go:266-290` registers admin routes and wraps existing admin
  UI/API routes with basic auth.
- `pkg/web/surface_handler.go:31-48` creates the admin-only handler for split
  listener deployments.
- `pkg/web/server.go:271-284` serves a public handler plus an admin handler
  when `AdminListen` is configured.
- `go.mod:6-34` has OpenTelemetry SDK/exporter dependencies but no direct
  Prometheus scrape exporter or `promhttp` dependency.
- `open-telemetry/opentelemetry-go @ 794c0724001cdbe38b40881f44f04190152306fe`
  `exporters/prometheus/example_test.go:79-103` shows a custom Prometheus
  registry, `otelprom.New(otelprom.WithRegisterer(reg))`, and
  `promhttp.HandlerFor(reg, ...)`.
- Current OpenTelemetry Go documentation confirms the Prometheus exporter is a
  metric reader registered with `sdkmetric.NewMeterProvider` and that metric
  views can apply attribute filters to control cardinality.

Affected contracts and surfaces:

- Admin HTTP route contract for `/metrics`.
- OpenTelemetry metric provider setup and resource/view policy.
- Operator monitoring docs.
- Operating-principles telemetry spec.
- Web route tests and observability tests.

Existing patterns to reuse:

- Admin-only route registration belongs in `surfaceRoutes.registerAdmin`.
- Split-listener public/admin behavior is already covered by
  `pkg/web/run_lifecycle_test.go`.
- OpenTelemetry provider setup is centralized in `internal/observability`.

Risk and blast radius:

- Security risk: `/metrics` is deliberately unauthenticated. In split-listener
  deployments this stays on the admin listener. In shared-listener deployments,
  the shared listener exposes `/metrics` without auth; operators must bind the
  shared listener accordingly.
- Dependency risk: adding the Prometheus exporter brings
  `github.com/prometheus/client_golang` and related dependencies into the main
  module.
- Operational risk: using the default Prometheus registry would mix Go runtime
  and handler self-metrics into the count. The implementation should use a
  custom registry so the endpoint represents update-ipsets OpenTelemetry
  metrics.

Sensitive data handling plan:

- Do not write raw hostnames, usernames, private endpoints, credentials, or
  local listener addresses into durable artifacts. Use endpoint paths and
  sanitized count evidence.

Implementation plan:

1. Add a Prometheus exporter/registry to `internal/observability` and attach it
   as a reader on the existing OpenTelemetry meter provider.
2. Expose the Prometheus scrape handler through `web.Options` so web routing
   does not own exporter setup.
3. Register `GET /metrics` on the admin surface without `wrapAdminAuth`.
4. Keep `/metrics` out of the public-only handler when a split admin listener is
   configured.
5. Update operator docs and the operating-principles spec.

Validation plan:

- Focused Go tests for observability setup and route behavior.
- `go test ./internal/observability ./pkg/web -count=1`.
- `make test`.
- Installed-service smoke: scrape `/metrics`, count distinct Prometheus sample
  identities, and record the resulting series count.

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: no update expected beyond the existing SOW-0096
  metric-label rule.
- Specs: update `.agents/sow/specs/operating-principles.md`.
- End-user/operator docs: update monitoring/environment docs for `/metrics`.
- End-user/operator skills: none expected.
- SOW lifecycle: keep current and in-progress until user approves commit and
  closure.

## Implications And Decisions

1. User decision: Do not remove useful bounded labels only to minimize the
   absolute series count.
   - Evidence: user clarified that a few thousand metrics are acceptable.
   - Implication: keep bounded labels such as feed name/status unless post-fix
     validation still shows unacceptable cardinality.

2. User decision: Ephemeral runtime values must not be metric labels.
   - Evidence: user explicitly named PID, queue depth, and batch size as labels
     that must not exist.
   - Implication: remove or filter process PID resource labels and dynamic
     numeric measurement attributes from metric identity.

3. Implementation choice: Prefer central filtering/resource policy over
   one-off call-site edits.
   - Benefit: catches future accidental dynamic labels.
   - Risk: must test that the OpenTelemetry SDK view behavior actually applies
     to all project metric instruments.
   - Recommendation: use a global metric view/attribute filter plus a stable
     metrics resource, then keep source-level comments/docs aligned.

## Plan

1. Apply the ephemeral metric attribute/resource policy with focused
   helper-level changes.
2. Remove or filter dynamic metric attributes at the source or SDK view layer.
3. Update specs/docs for the new telemetry contract.
4. Validate with focused tests and a live Netdata cardinality check.

## Execution Log

### 2026-06-01

- Investigated live local Netdata cardinality evidence and code-level metric
  attribute emitters.
- Checked official OpenTelemetry Go SDK documentation and local mirrored
  OpenTelemetry repositories for attribute-filter and cardinality-guard patterns.
- Created this SOW and stopped at the user-decision gate before code changes.
- Added a global OpenTelemetry metric attribute deny-list in
  `internal/observability/observability.go:188-197`.
- Split OpenTelemetry resource construction so metrics use stable service
  identity while traces and logs keep richer process/version resource data in
  `internal/observability/observability.go:98-149`.
- Removed queue depth and batch size from scheduler OpenTelemetry metric
  attributes in `pkg/scheduler/metrics.go:74-159`; the values remain in admin
  scheduler snapshots.
- Removed selected-feed count, processor-step count, processor-input bytes, and
  iprange fan-in counts from metric identity while preserving trace attributes
  where useful.
- Updated the operating-principles telemetry contract, operator telemetry
  reference, environment-variable docs, and project coding skill.
- Installed the daemon locally with `./install.sh` and smoke-checked the public
  health and status endpoints.
- Added a custom-registry Prometheus exporter in
  `internal/observability/observability.go` and attached it as a second metric
  reader on the OpenTelemetry meter provider.
- Added `web.Options.MetricsHandler` and registered `GET /metrics` on the admin
  surface without basic auth.
- Added a public-only route block so split-listener deployments do not expose
  `/metrics` on the public handler.
- Updated admin auth, monitoring, environment-variable, and operating-principles
  docs for the unauthenticated admin-surface Prometheus endpoint.

## Validation

Acceptance criteria evidence:

- OpenTelemetry metric identity policy is documented in
  `.agents/sow/specs/operating-principles.md:317-331`.
- Operator-facing telemetry behavior is documented in
  `docs/monitoring/telemetry-reference.md:24-34`.
- Metrics now use `metricCardinalityView()` and `metricAttributeFilter()` in
  `internal/observability/observability.go:132-136` and
  `internal/observability/observability.go:188-197`.
- Metric resources omit process identity and service-version churn by default in
  `internal/observability/observability.go:98-102` and
  `internal/observability/observability.go:167-185`.
- Scheduler queue depth and batch size remain admin snapshot fields in
  `pkg/scheduler/metrics.go:64-68`, but are no longer metric labels in
  `pkg/scheduler/metrics.go:74-159`.
- Bounded labels remain covered by test evidence in
  `internal/observability/observability_test.go:89-123`.

Tests or equivalent validation:

- `go test ./internal/observability -count=1` passed.
- `go test ./internal/observability ./pkg/scheduler ./pkg/engine ./pkg/processor ./pkg/iprange -count=1` passed.
- `make test` passed.
- `git diff --check` for changed files passed.
- `go test ./internal/observability ./pkg/web -count=1` passed after adding the
  Prometheus endpoint.
- `make test` passed after adding the Prometheus endpoint.
- `go test ./internal/observability ./pkg/web -count=1` passed after the Area 1
  API telemetry redesign implementation.
- `make test` passed after the Area 1 API telemetry redesign implementation.
- `git diff --check` passed after the Area 1 API telemetry redesign
  implementation.

Real-use evidence:

- `./install.sh` rebuilt and installed the local daemon, then restarted only
  `update-ipsets.service` through the project install script.
- A local public health request returned `ok`.
- A local public status request returned a running engine status with 423
  configured sources and 13 merges.
- Local Netdata MCP queries after the install showed new samples for
  `otel.download.queued` grouped by `scheduler.waiting` under `[unset]` with
  old `scheduler.waiting` dimensions contributing zero in the post-install
  window.
- Local Netdata MCP queries after the install showed new samples for
  `otel.download.queued` grouped by `resource.attributes.process.pid` under
  `[unset]` with old process-PID dimensions contributing zero in the
  post-install window.
- Local Netdata MCP queries after the install showed new samples for
  `otel.engine.batch.started` grouped by `engine.batch.size` and
  `otel.iprange.union.ops` grouped by `iprange.sources` under `[unset]` with
  old dynamic dimensions contributing zero in the post-install window.
- Existing Netdata chart metadata can still list historical label values until
  retention ages them out; the post-install query data shows those dimensions
  are not receiving new samples.
- After installing the Prometheus endpoint, waiting for the local daemon's
  processing run to settle, and warming `/metrics`, the scrape exposed 8,252
  unique Prometheus sample identities across 364 metric names.
- The same `/metrics` scrape had zero matches for the forbidden dynamic label
  names: scheduler waiting depth, engine batch size, selected count,
  processor-step count, processor-input bytes, iprange source count, file byte
  count, process PID, and service-version churn.
- The local service was in shared-listener development mode because an existing
  systemd drop-in clears the separate admin listener argument. In this topology
  `/metrics` is served on the shared local listener, matching the documented
  shared-listener behavior.
- After the Area 1 API telemetry redesign was installed through `./install.sh`,
  the local service was active and serving on the shared local listener.
- A warmed local `/metrics` scrape after Area 1 exposed 2,399 unique Prometheus
  sample identities across 347 sample metric names.
- The Area 1 scrape had zero forbidden dynamic-label hits and zero hits for the
  dropped API metric families: HTTP request body size, HTTP response body size,
  `http.admin_*`, `http.home_*`, `http.compare_set.*`, and
  `http.entity_artifact.*`.
- The Area 1 scrape exposed `api_recalculation_requests_total` for a public
  search with only `api.surface`, `api.action`, and `api.result` metric labels,
  plus exporter scope labels.
- The Area 1 scrape exposed `http_server_request_duration_seconds` with
  normalized `http_route`, `http_request_method`, and
  `http_response_status_code` metric labels, plus histogram/exporter labels.

Reviewer findings:

- No external reviewer was run because the user did not request one.

Same-failure scan:

- Scan for dynamic numeric attributes in metric helper calls found no remaining
  metric-path uses of `scheduler.waiting`, `engine.batch.size`, `file.bytes`,
  `run.selected`, `processor.steps`, `processor.input.bytes`, or
  `iprange.sources`.
- Remaining hits for `run.selected`, `processor.steps`,
  `processor.input.bytes`, and `file.bytes` are span attributes only, which are
  not metric identity.
- `/metrics` scrape scan found zero forbidden dynamic label names in exported
  Prometheus sample identities.

Area 1 implementation evidence:

- `internal/observability/observability.go:48-91` defines the bounded
  ephemeral-attribute deny-list, HTTP route/method/status allow-list, and
  bounded API recalculation surface/action/result values.
- `internal/observability/observability.go:240-289` applies one metric policy
  view: drop HTTP body-size instruments and ad hoc API handler metrics,
  allow-list HTTP server duration labels, and keep the existing generic
  ephemeral-label deny-list for other metrics.
- `internal/observability/observability.go:509-527` records
  `api.recalculation.requests` and `api.recalculation.targets`, with unknown
  surface/action/result values collapsed to `other`.
- `pkg/web/server.go:110-235` normalizes HTTP route labels to stable route
  templates and generic fallback templates instead of raw request paths.
- `pkg/web/search_api.go:18-100` records public dynamic search and feed-search
  API work as bounded recalculation/dynamic-work metrics.
- `pkg/web/routes.go:74-87` records public compose dynamic work, and
  `pkg/web/routes.go:320-354` records admin run/reprocess scheduling or
  conflict results.
- `pkg/web/admin.go:339-364` records feed recheck/reprocess scheduling and
  conflict results, while `pkg/web/admin.go:428-435` records artifact recheck
  scheduling.
- `pkg/web/integrity.go:170-185` records entity rebuild scheduling,
  in-progress, and error results; `pkg/web/integrity.go:285-325` records
  integrity reprocess in-progress, clean, and scheduled results.

Area 1 test evidence:

- `internal/observability/observability_test.go:129-184` proves dropped noisy
  API metrics and HTTP server duration label allow-list behavior.
- `internal/observability/observability_test.go:300-328` proves
  `api.recalculation.*` exposes only bounded labels and collapses unknown
  labels to `other`.
- `pkg/web/routes_test.go:220-267` proves dynamic public/admin paths normalize
  to route templates and the HTTP middleware overrides the mux pattern before
  `otelhttp` records metrics.

Areas 2-10 implementation evidence:

- `internal/observability/observability.go:64-112` defines the final default
  OpenTelemetry allow-list: 48 designed instrument names before Prometheus
  counter/histogram sample expansion.
- `internal/observability/observability.go:352-376` applies per-instrument
  attribute allow-lists so component metrics keep only their bounded labels.
- `internal/observability/observability.go:245` records `daemon.up`, and
  `internal/observability/observability.go:232-243` resets cached instruments
  when a new meter provider is installed.
- `internal/observability/observability.go:174-187` uses a service-only
  resource for metrics while traces/logs keep richer resource data;
  `internal/observability/observability.go:273-291` attaches host/OS/process
  resource detectors only when the richer resource is requested.
- Area 2: `pkg/web/cache.go:128-250` records artifact cache hit, miss,
  oversize, error, eviction, entry, and byte gauges without file/path/feed
  labels; `pkg/web/cache.go:295` is the lookup helper.
- Area 3: `pkg/engine/public_catalog.go:86` observes public feed summaries,
  and `pkg/engine/public_catalog.go:164-174` records the per-feed state,
  health, entries, unique IPs, errors, freshness, and last-success gauges.
- Area 4: `pkg/scheduler/metrics.go:79-174` records queue admissions, work
  starts/completions, queue depth values, batch item values, and batch duration
  with only scheduler queue/result labels.
- Area 5: `pkg/downloader/downloader.go:126-130` records downloader fetches,
  bytes, duration, and errors with only downloader/status labels.
- Area 6: `pkg/processor/processor.go:117-118` and
  `pkg/processor/run_stream.go:37-38` record processor run rate/duration by
  mode/status; `pkg/processor/run_stream.go:249-319` records bounded temp-write
  metrics by temp kind.
- Area 7: `pkg/engine/run.go:40-42` records engine run rate/duration/running
  state, `pkg/engine/run_metrics.go:187` records phase duration under one
  metric family, and `pkg/engine/batch_state.go:23-31` records current phase
  gauges across the fixed phase set.
- Area 8: `pkg/web/integrity.go:75-119` records pipeline/entity integrity
  checks and findings, and `pkg/web/integrity.go:357-390` records bounded
  integrity check/recovery metrics.
- Area 9: `pkg/engine/background_tasks.go:165-200` records background task
  starts/completions/failures, and `pkg/engine/background_tasks.go:212-251`
  records worker wait/active/limit metrics with a bounded component taxonomy.
- Area 10: `pkg/config/load.go:17-31`, `pkg/config/load.go:68-126`, and
  `pkg/config/load.go:146-153` record config loads by result;
  `pkg/cache/cache.go:261-314` and `pkg/cache/cache.go:362-376` record runtime
  cache load/save operations by operation/result.
- iprange primitive operations: `pkg/iprange/otel.go:48-68` collapses all
  package-local primitive operation names into `iprange.operations` and
  `iprange.operation.duration_ms`; `pkg/iprange/otel.go:73-100` keeps only
  `ip.version` and the code-derived `iprange.operation` label.

Areas 2-10 test evidence:

- `internal/observability/observability_test.go:198-257` proves designed
  component metrics keep only their own bounded labels and unknown instruments
  are dropped.
- `internal/observability/observability_test.go:341-383` proves API
  recalculation labels remain bounded after the allow-list conversion.
- `pkg/iprange/otel_test.go:5-24` proves iprange operation-name collapse.
- `internal/observability/observability_test.go:262-283` proves the default
  metrics resource omits process, service-version, host, and OS identity.
- Focused package validation passed for `./internal/observability`, `./pkg/web`,
  `./pkg/engine`, `./pkg/scheduler`, `./pkg/downloader`, `./pkg/processor`,
  `./pkg/iprange`, `./pkg/cache`, and `./pkg/config` after the area 2-10
  implementation.
- Full validation passed with `make test`, including UI static generation,
  `go test ./...`, and `tools/archposture`.
- `git diff --check` passed after the final implementation.
- `./install.sh` completed successfully and restarted the local service.
- Final live `/metrics` measurement after installation and endpoint warm-up:
  3,294 unique Prometheus sample identities across 46 sample metric names.
- Final scrape scans found zero forbidden dynamic labels: scheduler waiting
  depth, engine batch size, selected count, processor-step count,
  processor-input bytes, iprange source, file bytes, process PID,
  service-version churn, automatic host name, and automatic OS description.
- Final scrape scans found zero old rejected metric families, including HTTP
  body size, ad hoc `http.admin_*`, old iprange operation-specific names,
  metadata/source/entity internals, processor step/stream-step, file copy/write,
  cache load/save, and status-specific downloader metric names.
- Final `target_info` metric carries service identity only: service name,
  service namespace, and OpenTelemetry SDK metadata.

Sensitive data gate:

- Durable artifact uses redacted workstation evidence and does not include raw
  secrets, credentials, bearer tokens, customer names, personal data, private
  endpoints, or workstation-identifying host labels.

Artifact maintenance gate:

- AGENTS.md: no update needed; existing project-wide SOW and telemetry rules are
  sufficient.
- Runtime project skills: updated `.agents/skills/project-coding/SKILL.md` with
  the durable OpenTelemetry metric-label rule.
- Specs: updated `.agents/sow/specs/operating-principles.md` with bounded
  metric identity, Prometheus scrape, default allow-list, iprange collapse, and
  designed scheduler/downloader/engine metric-family contracts.
- End-user/operator docs: updated `docs/monitoring/telemetry-reference.md`,
  `docs/monitoring/opentelemetry-setup.md`,
  `docs/running/admin-authentication.md`, and
  `docs/running/environment-variables.md`; the telemetry reference now
  documents the full designed Area 1-10 default metric surface.
- End-user/operator skills: none expected.
- SOW lifecycle: remains current and in-progress until final validation,
  installed-service measurement, and user-requested closure/commit handling are
  complete.

Specs update:

- `.agents/sow/specs/operating-principles.md` updated with the bounded
  OpenTelemetry metric identity contract, stable metrics-resource rule, and
  unauthenticated admin-surface Prometheus scrape endpoint contract.

Project skills update:

- `.agents/skills/project-coding/SKILL.md` updated to prevent future dynamic
  metric labels.

End-user/operator docs update:

- `docs/monitoring/telemetry-reference.md` updated with metric-label semantics.
- `docs/monitoring/opentelemetry-setup.md` updated with the Prometheus scrape
  endpoint behavior.
- `docs/running/admin-authentication.md` updated with the `/metrics` auth
  exception and listener-risk guidance.
- `docs/running/environment-variables.md` updated with stable metric resource
  behavior and the distinction between OTLP push export and the Prometheus
  scrape endpoint.

End-user/operator skills update:

- None expected.

Lessons:

- Bounded labels and dynamic labels are different problems. Do not remove useful
  bounded feed/status labels just to reduce the absolute series count; remove
  runtime values that create unstable metric identity.
- Metric views protect against accidental future measurement attributes, but
  resource attributes need separate handling because SDK metric attribute
  filters do not filter resources.
- Direct `/metrics` measurement is the right way to count current exposed
  Prometheus series because downstream collector metadata can retain historical
  label values after an application-side fix.

Follow-up mapping:

- Historical Netdata label metadata is expected to remain visible until
  retention expires, but no separate implementation work is required for that.
- No implementation follow-up is intentionally deferred. Final installed-service
  measurement confirmed the default Prometheus sample-name and series counts.

## Outcome

The prior cardinality fix and Prometheus endpoint were implemented and
validated. The user then rejected the broader metric inventory as ad hoc, so
the SOW remained open for the area-by-area redesign. Areas 1-10 are now modeled
and implemented in code, with a 48-instrument allow-list before Prometheus
counter/histogram sample expansion.

Measured current Prometheus exposure after the endpoint was installed and the
local processing run settled: 8,252 unique time-series across 364 metric names,
with zero forbidden dynamic-label hits in the scrape.

Measured current Prometheus exposure after Area 1 was installed and warmed:
2,399 unique time-series across 347 sample metric names, with zero forbidden
dynamic-label hits and zero dropped API metric hits. This is still above the
final redesign target because Areas 2-10 have not been implemented yet.

Final installed-service measurement after the area 2-10 implementation:
3,294 unique Prometheus sample identities across 46 sample metric names, with
zero forbidden dynamic-label hits and zero old rejected metric-family hits.

## Lessons Extracted

- OpenTelemetry metric labels must represent bounded identity, not runtime
  measurements. Queue depth, batch size, PID, selected counts, processor input
  sizes, and fan-in counts belong in metric values, admin snapshots, traces, or
  logs, not metric identity.
- The Prometheus admin endpoint should use the application OpenTelemetry
  registry, not the global Prometheus registry, so the series count reflects
  update-ipsets telemetry rather than unrelated default collectors.

## Followup

None intentionally deferred.

## Regression Log

None yet.
