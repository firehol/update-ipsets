# SOW-0094 - Operator Docs And Specs Sync

## Status

Status: completed

Sub-state: operator-doc/wiki/spec sync remains completed and validated; regression fix completed on 2026-06-02 for review-only docs accuracy findings; application-review notes are mapped to SOW-0095.

## Requirements

### Purpose

Bring `docs/` and `.agents/sow/specs/` up to date with the current application. `docs/` should teach deployment, configuration, operations, troubleshooting, and supported public API use. Specs should describe the product/application contracts as the current application actually behaves, not desired future behavior. Internal implementation memory belongs in `.agents/sow/specs/` and `.agents/skills/`, not in operator docs.

### User Request

Update `docs/` against the current codebase. Treat `docs/` as operator documentation. Only the API Reference is third-party developer documentation. Keep internal documentation in specs and skills. Fix GitHub wiki navigation links so the table of contents opens rendered wiki pages instead of raw markdown files.

Follow-up user request: the application is stable and fairly complete, but specs may not be up to date. Under the repo rules, ensure specs describe the current application as-is. If this process finds issues, bugs, or clearly wrong logic, gather the notes in this SOW for review; do not change code or redesign specs to desired behavior before review.

### Assistant Understanding

Facts:

- Project SOW status is initialized, so this non-trivial docs work requires a SOW.
- `project-content-surfaces` classifies operator docs / README as deployer and operator material, where users should learn to configure, run, debug, and call supported APIs.
- `sync-docs-specs-skills` requires auditing docs, specs, and skills for surface separation and then repeating the same-scope audit after repairs.
- `cmd/update-ipsets/daemon.go` defines daemon flags including `--trust-proxy-headers` and `--trust-cloudflare-headers`.
- `cmd/update-ipsets/query.go` defines `--config`, `--set`, `--ip`, `--format`, `--silent`, and `--verbose`; it does not define `--details`.
- `pkg/web/routes.go` registers public routes, admin routes, direct artifact routes, and the `/mcp` endpoint. It does not register `/api/v1/sets/about/{name}`.
- Source docs should keep normal repository-relative `.md` links so they render correctly in the code repository. The GitHub Wiki publishing workflow must flatten docs into wiki-root page filenames and rewrite local links to full GitHub wiki URLs such as `https://github.com/firehol/update-ipsets/wiki/<page>`. Direct wiki links with nested `.md` paths can redirect to raw markdown outside the rendered wiki, and bare sidebar slugs render as relative links instead of explicit GitHub wiki URLs.
- Specs are normative product/application contracts for future agents and maintainers; they should make product behavior predictable without rereading all code.
- The spec audit must update specs to current application behavior and record suspected bugs or wrong logic here for later review, instead of changing code during this pass.

Inferences:

- Mechanical mismatches between docs and code can be fixed without changing product behavior.
- Moving or deleting misplaced docs requires explicit user approval because project rules forbid deleting or moving docs without approval.
- Evidence-backed spec drift can be fixed by changing specs to match the current application, unless the evidence points to a likely application defect that should be reviewed first.

Unknowns:

- Whether the `docs/contributing/` directory should be renamed in a later change. The content is now catalog-operator material, but the path was preserved because moving docs requires explicit approval.
- Whether current admin-API rate limiting through the shared `/api/` prefix is an intentional product decision or an implementation side effect.
- Whether the accepted `ignore_repeating_download_errors` runtime field is intentionally inert in scheduler retry timing or is a missed implementation path.

### Acceptance Criteria

- Every documented public and admin API route in `docs/api/` and admin docs is supported by current route registration, or clearly marked unsupported/non-route.
- Every documented CLI flag in `docs/cli/`, `docs/running/`, and related install/security pages is supported by the current CLI flag sets and described with the correct operational meaning.
- Configuration and feed catalog docs use current YAML field names, current role names, current category model, and current legal classification rules.
- `docs/` navigation and top-level pages do not present internal implementation history or maintainer-only process as operator documentation unless the user explicitly approves an exception.
- The GitHub Wiki publishing output uses full GitHub wiki URLs without `.md` extensions for local pages, while source docs keep repository-relative `.md` links.
- API Reference remains suitable for third-party developers.
- No raw sensitive data is written to docs, specs, skills, or the SOW.
- Validation includes code-vs-doc route/flag checks, stale internal-content scans, markdown link checks where practical, and a same-scope repeat audit after repair.
- Specs under `.agents/sow/specs/` describe current application behavior as-is for audited areas, with file/line evidence recorded for material changes.
- Suspected bugs, wrong logic, or product-quality concerns found during the spec audit are recorded in this SOW with evidence and are not silently "fixed" by changing the spec to hide the issue.

## Analysis

Sources checked:

- `AGENTS.md`
- `.agents/skills/project-content-surfaces/SKILL.md`
- `/home/user/.agents/skills/sync-docs-specs-skills/SKILL.md`
- `cmd/update-ipsets/main.go`
- `cmd/update-ipsets/daemon.go`
- `cmd/update-ipsets/query.go`
- `cmd/update-ipsets/enable.go`
- `cmd/update-ipsets/cache_merge.go`
- `pkg/config/config.go`
- `pkg/config/validate.go`
- `pkg/web/routes.go`
- `pkg/web/server.go`
- `pkg/web/http.go`
- `pkg/web/middleware.go`
- `pkg/web/search_api.go`
- `pkg/engine/critical.go`
- `pkg/engine/integrity_payloads.go`
- `pkg/engine/public.go`
- `pkg/web/feature_test.go`
- `docs/Home.md`
- `docs/_Sidebar.md`
- `docs/api/api-overview.md`
- `docs/api/feed-endpoints.md`
- `docs/api/rate-limits-cors.md`
- `docs/cli/query-command.md`
- `docs/running/daemon-reference.md`
- `docs/contributing/*.md`
- `docs/todo-history/README.md`
- `docs/todo-history/TODO.md`
- `.github/workflows/wiki-sync.yml`
- `scripts/build-wiki.mjs`

Current state:

- `docs/api/feed-endpoints.md:167` documents `GET /api/v1/sets/about/{name}`, but `pkg/web/routes.go:45-58` registers only `/api/v1/sets`, `/api/v1/ipsets`, `/api/v1/sets/{name...}`, `/api/v1/ipsets/{name...}`, search, and compose under the feed route family.
- `docs/cli/query-command.md:21` documents `--details`; `cmd/update-ipsets/query.go:18-23` defines no such flag.
- `docs/running/daemon-reference.md:106-108` says `--web-files-dir` points to static web assets, but `pkg/web/surface_routes.go:30-42` and `pkg/web/routes.go:164-174` use the resolved files directory for raw `.ipset` / `.netset` serving.
- `docs/api/api-overview.md:7-12` says all public API endpoints live under `/api/v1/` and accept `GET` / `HEAD` only, but `pkg/web/routes.go:26-28` registers public MCP methods `POST`, `GET`, and `DELETE` at `/mcp`.
- `docs/_Sidebar.md:73-85` omits `docs/api/mcp-endpoint.md`, while `docs/api/api-overview.md:56` links it.
- `docs/contributing/contribution-guide.md:7-10` describes fork and pull request process. That is contributor workflow, not operator workflow.
- `docs/contributing/step-by-step-add-feed.md:43-76` uses stale YAML names such as `processors` and `homepage`, while `pkg/config/config.go:199-251` uses `processor` and `maintainer_url`.
- `docs/todo-history/README.md:6-11` points readers to historical SOW/spec locations, and `docs/todo-history/TODO.md:1-120` is implementation planning history, not operator documentation.
- Live GitHub Wiki root-cause check on 2026-05-31 showed `https://github.com/firehol/update-ipsets/wiki/api/api-overview.md` redirected to `raw.githubusercontent.com/wiki/firehol/update-ipsets/api/api-overview.md`, while `https://github.com/firehol/update-ipsets/wiki/api-overview` rendered as a GitHub Wiki page. Current workflow copied docs subdirectories directly to the wiki, so sidebar links such as `api/api-overview.md` could leave the rendered wiki.
- `docs/_Sidebar.md` and many docs pages are also viewed in the source repository, so making source links wiki-only is not sufficient; wiki-specific link rewriting belongs in the publish workflow.
- Follow-up live GitHub Wiki check on 2026-06-01 showed the remaining navigation problem after extensionless flattening: raw `_Sidebar.md` used bare links such as `[About update-ipsets](about-update-ipsets)`, and rendered custom sidebar links remained relative rather than explicit GitHub wiki URLs. The user reported these links opening outside the rendered wiki context, so the builder must emit full GitHub wiki URLs, not just extensionless slugs.
- Initial spec audit surface is `.agents/sow/specs/*.md`; `.agents/sow/specs/README.md` defines the canonical ownership map and single-owner rule.
- `.agents/sow/specs/operating-principles.md:58-64`, `.agents/sow/specs/feeds.md:521-525`, and `.agents/sow/specs/files-layout.md:457-471` still described the older public critical-overlap contract where public routes reject artifacts with stale `provider_set_id` values. Current code and tests preserve cache-first public serving: `pkg/web/routes.go:225-263` serves published critical artifacts without comparing identity, `pkg/web/feature_test.go:750-830` requires stale-identity public artifacts to return `200`, and `.agents/sow/specs/website.md:214-227` / `.agents/sow/specs/integrity.md:81-86` already document this split.
- `.agents/sow/specs/pipeline.md:119-134` described critical provider-set identity as including provider content hash and cardinality. Current code excludes materialized cache state: `pkg/engine/critical.go:169-177` states the identity deliberately excludes content hash, entries, and unique IPs; `pkg/engine/critical.go:329-363` fingerprints source configuration fields, not materialized range content.
- `.agents/sow/specs/operating-principles.md:257-259` said the general 240/min API rate limit excludes admin endpoints. Current middleware applies it to every path beginning with `/api/` or `/mcp`, including `/api/v1/admin/*`: `pkg/web/middleware.go:73-87`. Search/query endpoints then add a separate 10/min limiter in `pkg/web/search_api.go:96-105`.
- `.agents/sow/specs/website.md` listed `/api/v1/compose` but did not define its current bounds. Current compose behavior requires at least one include, caps includes and excludes at 20 each, caps output at 32 MiB, supports CIDR/range/single-IP formats, and validates each set through public raw-feed policy: `pkg/engine/public.go:349-482`.
- `.agents/sow/specs/architecture-posture.md:42-76` contained stale measured posture highlights. `go run ./tools/archposture -root .` on 2026-05-31 reported 575 scoped files, 120,643 lines, 53 `HandleFunc` route registrations, and 3 `Handle` registrations.
- After `docs/todo-history/` was moved, stale references to the old path were removed from code comments and the removed-config validation error. Current references now point to `.agents/sow/specs/processing-engine.md` in `pkg/insights/insight.go:5`, to the processing-engine spec entry point in `pkg/insights/insights.go:5`, and to operator migration docs in `pkg/config/config.go:736`.
- Repeat operator-doc audit found additional code/doc mismatches:
  - `docs/updating/updating-binary.md` used `jq '.version'` against `/api/v1/status`, but `pkg/web/public_status.go` exposes only `engine` and `system` objects and no version field.
  - `docs/api/health-status.md` showed a stale `/api/v1/status` response with top-level `status`, `feeds`, `countries`, and `asns`; current response fields are built in `pkg/web/public_status.go`.
  - `docs/api/rate-limits-cors.md` and `docs/security/rate-limiting.md` described the search limiter as independent from the general limiter, but `pkg/web/middleware.go:73-87` applies the general limiter to `/api/*` before the search handler applies the stricter search limiter.
  - `docs/api/compose-endpoint.md` omitted current include/exclude/output bounds and accepted format aliases from `pkg/engine/public.go:349-482`.
  - `docs/api/raw-file-downloads.md` omitted the direct root-level `.ipset` / `.netset` compatibility route served by `pkg/web/routes.go:374-386`.
  - `docs/installation/systemd-setup.md` used the wrong drop-in path form for one example.
  - `docs/running/environment-variables.md` did not explain that the installed unit's `HOME=/opt/update-ipsets` makes the service env file `/opt/update-ipsets/.update-ipsets.env`, matching `pkg/engine/envfile.go` and `install.sh`.
  - `docs/installation/installation.md` simplified service restart behavior and missed the enabled-but-inactive start path in `install.sh`.
- Further repeat operator-doc audit found additional mismatches:
  - `docs/integrity/running-integrity-checks.md` described queuing recovery for individual findings, but the current admin UI and backend expose one `Recover all` action through `POST /api/v1/admin/integrity/reprocess`; individual rows show recovery plans but do not have a per-finding recovery endpoint.
  - `docs/cli/iprange-command.md` described `--ipset-reduce` and `--reduce-factor` as different reduction models, but `pkg/iprange/cli.go` and `pkg/iprange/cli6.go` treat them as aliases with an allowed entry-growth percentage.
  - CLI docs and navigation did not include the implemented `cache-merge` migration helper subcommand from `cmd/update-ipsets/cache_merge.go`.
  - `docs/configuration/runtime-settings.md` described `ipset_reduce_factor` and `ipset_reduce_entries` as active output-generation knobs, but current code only accepts these fields in config defaults/structs and does not read them during Go public output generation.
  - `docs/api/methodology-endpoints.md` documented the methodology index as `pages` and described page bodies as structured content, but `pkg/web/methodology.go` returns `items` for the index and rendered HTML in the `body` field.
  - `docs/integrity/finding-classes.md` used the internal `provider_set_id` field name in an operator-facing integrity example, even though the practical operator meaning is that the file was generated for an older reference-provider set.
  - `docs/api/search-query.md` and `docs/security/security-overview.md` still described IP search rate limits as independent from the general API rate limit, but current middleware applies the general `/api/` limiter before the search handler applies the stricter search limiter.
  - `docs/configuration/runtime-settings.md` omitted accepted runtime fields `skip_comparison_if_no_updates` and the per-category health threshold subfields `healthy_cadence_minutes` / `risky_cadence_minutes`, which are accepted by `pkg/config/config.go` and validated in `pkg/config/feed_health.go`.
  - Feed-name docs warned against path separators, commas, controls, and non-ASCII, but `pkg/config/validate.go:176-191` also rejects reserved filename characters (`: * ? " < > |`).
  - Feed docs only listed common processors, but `pkg/processor` accepts a wider operator-facing `processor:` / `processor_raw:` surface with generic, structured-data, archive, and compatibility processors.
  - OpenTelemetry docs only documented the generic OTLP endpoint and `OTEL_TRACES_EXPORTER`, but `internal/observability/observability.go:177-182` also enables export from per-signal endpoint variables and `internal/observability/observability.go:257-264` honors standard traces, metrics, and logs exporter variables.
  - Monitoring docs still described stale admin-status fields under `counters.*`, `process.*`, and `scheduler_state`. Current admin status exposes `system`, `engine`, `scheduler`, `queues`, `metrics`, `feeds`, and `artifacts` at `pkg/web/admin.go:20-29`; system resource fields live under `system` at `pkg/web/admin.go:32-59`; scheduler counters live under `metrics` at `pkg/scheduler/metrics.go:13-29`; engine timing/counter snapshots live under `engine.current_metrics`, `engine.last_metrics`, and `engine.lifetime_metrics` at `pkg/engine/engine.go:119-152`.
  - Monitoring docs also described OpenTelemetry as if every old admin counter appeared under the admin response. Current OpenTelemetry emits counters, byte counters, and duration histograms through `internal/observability/observability.go:336-370`, while admin status exposes only selected scheduler and engine snapshots.
- Further admin-UI docs audit found stale operator wording:
  - `docs/admin-ui/runtime-status.md` described a generic top resource panel, but the current page composes a heartbeat, current-run, artifacts, integrity, entity-integrity, feed table, and feed modal at `ui/src/pages/admin.tsx:9-15`; the heartbeat first row is daemon plus feed-health tiles at `ui/src/components/admin/heartbeat.tsx:76-145`, and the second row is uptime, config, heap, goroutines, disk free, and integrity at `ui/src/components/admin/heartbeat.tsx:147-235`.
  - `docs/admin-ui/schedule-panel.md` described a standalone schedule panel, but the current admin page has no such component; schedule state appears in feed rows and drawers, artifact rows, and the admin schedule API. The feed schedule drawer renders cadence, health thresholds, next check, scheduler state, and retry/backoff detail at `ui/src/components/admin/feed-modal-status-sections.tsx:21-185`; artifact schedule detail is shown in `ui/src/components/admin/artifacts-panel.tsx:151-164`.
  - `docs/admin-ui/operator-actions.md` overstated broad reprocess and history recheck behavior. The backend rejects global recheck and maps broad reprocess through `/api/v1/admin/run?reprocess=true` at `pkg/web/routes.go:315-345`; the scheduler skips reprocess targets without local staged or committed state at `pkg/scheduler/actions.go:42-64`; history recheck falls back to the parent when local derivative composition is unavailable at `pkg/engine/download_stage.go:232-248`.
  - `docs/admin-ui/feed-inventory.md` and `docs/admin-ui/artifact-inventory.md` had older column names. Current feed table headers are `Feed`, `Category`, `Vis`, `Sched`, `Actual`, `Next`, `Processed`, `Why`, `Took`, `Upstream`, `IPs`, `Entries`, `State`, `Fail`, `Files`, and public-page action at `ui/src/components/admin/feeds-table-header.tsx:21-130`; current artifact table headers and actions are in `ui/src/components/admin/artifacts-panel.tsx:81-89` and `ui/src/components/admin/artifacts-panel.tsx:187-221`.
- Additional operator-doc audit found stale product-boundary wording:
  - `README.md` and `docs/about-update-ipsets.md` still described IPv6 as wholly unsupported. Current code shows the standalone `iprange` package and CLI have IPv6 support through `pkg/iprange/ipv6.go:3-8` and `pkg/iprange/cli_family.go:9-55`, while the public feed/IP lookup path remains IPv4-only through `cmd/update-ipsets/query.go:56-67` and `pkg/engine/query.go:20-69`. `pkg/config/config_coverage_test.go:916-927` also validates that catalog sources may declare `ipv6`.
  - `docs/migration-from-bash.md` described the migration helper too narrowly. Current helper usage, defaults, and legacy path overrides are defined in `scripts/sync-from-bash-version.sh:58-91`; daemon stop and pre-sync backup behavior is in `scripts/sync-from-bash-version.sh:204-230`; manifest/cache merge behavior is in `scripts/sync-from-bash-version.sh:315-353`; API-key extraction is in `scripts/sync-from-bash-version.sh:361-410`; summary and rollback output is in `scripts/sync-from-bash-version.sh:438-469`.
  - `docs/security/admin-authentication.md` did not state the current status-code split for missing versus wrong admin credentials. Current middleware returns `503 Service Unavailable` when required credentials are not configured and `401 Unauthorized` for wrong credentials at `pkg/web/middleware.go:120-135`.
- Further less-audited operator-doc pass found stale API/status/template details:
  - `docs/api/feed-endpoints.md` used stale field names such as `ip_version`, `tracked`, and `source_timestamp`. Current public feed summaries expose `ipv`, `unique_ips`, `frequency_minutes`, and related fields in `pkg/engine/public_catalog.go:13-60`; per-feed metadata exposes `ipv`, `ips`, `started`, `updated`, `processed`, `checked`, `source`, and `file` in `pkg/engine/output.go:36-100`.
  - `docs/api/classification-endpoints.md` used stale country/ASN/maintainer field examples such as `feeds` and `ips`. Current country and ASN index payloads expose `provider`, `countries`/`asns`, and per-row `feed_count` / `attributed_ips` at `pkg/engine/home_index.go:3-28`; current detail and maintainer payloads are defined at `pkg/engine/home_detail.go:63-145` and `pkg/engine/home_detail.go:232-244`.
  - `docs/pipeline/download-lifecycle.md` documented a downloader status `ok`, but current durable download statuses use `downloaded` for changed content at `pkg/cache/download_status.go:5-20`.
  - `docs/pipeline/feed-status-reference.md` and `docs/troubleshooting/processing-failures.md` missed current `invalid_input` processing status and mixed in nonexistent `integrity_failed` last-status wording. Current processing exceptions are in `pkg/engine/processing_result.go:11-18`; ordinary feed status transitions are in `pkg/cache/entry_lifecycle.go:22-78`; provider/support status transitions are in `pkg/cache/entry_config.go:259-300`; operator labels are in `pkg/engine/operator_status.go:84-140`.
  - Install/config docs did not describe installed Markdown templates and reload behavior. The installer copies templates to the active config tree at `install.sh:187-198`; the engine loads templates from `{config_path}/templates/markdown/` at startup in `pkg/engine/markdown.go:14-27`; feed/entity Markdown writers produce published `.md` artifacts in `pkg/engine/markdown.go:30-84`. The current SIGHUP reload path in `pkg/engine/engine.go:339-424` reloads YAML configuration and runtime state but does not reload Markdown templates.
- Further operator-doc coverage audit found admin UI docs described actions but did not expose the authenticated operator API routes behind them. Current route registration is in `pkg/web/routes.go:272-287`; feed action routing is in `pkg/web/admin.go:296-384`; artifact action routing is in `pkg/web/admin.go:388-456`; integrity and entity-integrity routing is in `pkg/web/integrity.go:93-188` and `pkg/web/integrity.go:269-330`.
- `docs/updating/updating-config.md` used a workstation-specific checkout path and described restart as a separate required step after `install.sh`. Current installer behavior restarts an active service, starts an enabled inactive service, or skips restart only with `--no-restart` at `install.sh:260-282`.
- Legal-field docs had a critical-infrastructure redistributability drift: `docs/contributing/license-requirements.md`, `docs/feeds/legal-fields.md`, and `docs/critical-infrastructure-reference-feeds.md` said critical reference feeds default to non-redistributable, while `.agents/sow/specs/ai-classification-rules.md` defines the direct-upstream-only rule and the active catalog has critical reference feeds marked `redistributable: true`, including `critical_soft_cloudflare_edge`, `critical_soft_auth0`, and `critical_soft_akamai_edge_secondary`.
- Merge-health docs flattened additive and subtractive merge-parent behavior. Current code skips disabled/archived/unmaintained additive inputs but treats disabled/archived/unmaintained/missing subtractive inputs as composition blockers because omitting an exclude parent would broaden the output: `pkg/engine/merge_inputs.go:96-141` and `pkg/engine/feed_body_stage.go:467-475`.
- Configuration-reload docs understated the enable-state recalculation. Current reload replaces config/runtime state, applies runtime overrides, refreshes provider/cache state, reconciles entries, and effective enablement is recalculated from the new catalog plus existing source/artifact enable marker files: `pkg/engine/engine.go:339-424` and `pkg/engine/enabled_state.go:30-88`.
- Split-listener docs understated the startup requirement and surface split. Current validation rejects `--admin-listen` unless `runtime.public_base_url` is configured, and the admin-only handler registers admin routes plus embedded assets but not public API/artifact/SPAs: `pkg/web/server.go:98-104` and `pkg/web/surface_handler.go:35-54`.
- Environment-variable docs omitted systemd notification variables read by the daemon. `pkg/systemd/notify.go:40-54` reads `WATCHDOG_USEC` and `NOTIFY_SOCKET`; operators normally should not set them manually because systemd supplies them for `Type=notify` and `WatchdogSec=`.
- Logging docs described local logs as JSON, but the CLI, web server, scheduler, and engine default loggers use Go `slog` text handlers: `cmd/update-ipsets/common.go:8-16`, `pkg/web/server.go:66-76`, `pkg/scheduler/scheduler.go:55-58`, and `pkg/engine/engine.go:214-217`. OpenTelemetry can export logs when configured, but local stderr/journald records are text `key=value` records.
- `enabled_by_all` docs described the field as controlling which feeds `--enable-all` includes, but current runtime enablement does not read that field. `cmd/update-ipsets/daemon.go:28` exposes `--enable-all` as "treat all sources as enabled"; `pkg/engine/enabled_state.go:30-88` checks the daemon override, explicit override, and marker files without consulting `Source.EnabledByAll`; `pkg/engine/engine.go:439-458` writes enable markers for all sorted sources when `enable --all` is used. `pkg/config/config.go:217` and `pkg/config/extract.go:168` still accept/populate the field as catalog metadata.
- Trigger docs said critical provider-set drift includes content changes, but current critical provider-set identity deliberately excludes materialized provider content, entries, and unique IPs: `pkg/engine/critical.go:169-177`; scheduler tests also require volatile cache fields not to mark a critical provider due at `pkg/scheduler/scheduler_test.go:222-233`.
- Quick-start docs started the daemon without `--enable-all`, but a fresh checkout has no source enable marker files under `configs/firehol/` and current enablement requires marker files or the daemon override: `pkg/engine/enabled_state.go:30-88`. Without `--enable-all`, the first-run daemon shows interfaces but has no active bundled-catalog feeds until the operator enables them.
- Targeted API payload audit found additional `docs/api/feed-endpoints.md` drift:
  - Feed history examples used ISO timestamps, but public history CSV writes integer Unix timestamps in the `DateTime` column: `pkg/engine/public_series.go:35-48`.
  - Pairwise comparison docs described older fields `peer`, `common_ips`, `common_pct_self`, and `common_pct_peer`, but current comparison rows expose `name`, `category`, `ips`, `common`, and optional `related`: `pkg/engine/engine.go:79-101`.
  - Retention docs described median/oldest-current-IP fields, but current retention JSON exposes `ipset`, `started`, `updated`, `incomplete`, `past`, and `current`; the series objects expose `hours`, `ips`, and `total`: `pkg/engine/engine.go:104-117`.
- Targeted operator-command audit found one stale jq example in `docs/troubleshooting/merge-failures.md`: it referenced nonexistent `merge_health_excluded`. Current admin feed payloads expose `merge_included`, `merge_subtracted`, and `merge_excluded` at `pkg/web/admin.go:188-191`; those fields are populated from `MergeComposition` at `pkg/web/admin.go:782-789`, whose input-state rows expose reason and health fields at `pkg/engine/merge_inputs.go:19-31`.
- Admin operator-action audit found contradictory artifact-child recheck wording in `docs/admin-ui/operator-actions.md`: the table said artifact children do not trigger a parent fetch, while the same page said missing child input targets the parent. Current code calls `ResolveRecheckTarget` for feed recheck at `pkg/web/admin.go:339-347`; `pkg/engine/download_stage.go:232-258` keeps the child target only when staged or committed child input exists and otherwise returns the configured artifact parent.
- Install/runtime audit found two operator-facing path drifts:
  - `docs/installation/installation.md` documented `./install.sh /opt/custom-path` as a custom install directory without explaining that the shipped systemd unit still hardcodes `/opt/update-ipsets` for `ExecStart`, `WorkingDirectory`, `ReadWritePaths`, and path environment values in `install.sh:223-280`.
  - `docs/glossary.md` listed the web directory default as `/opt/update-ipsets/data/web/`, while the installed unit sets `WEB_DIR=/opt/update-ipsets/web` at `install.sh:268` and the daemon applies that value through runtime path expansion.
- Environment-variable and runtime-setting docs also needed the current non-root default-path nuance: `pkg/engine/runtime.go:86-117` changes the bundled default path templates to user-owned state paths when the daemon runs as a non-root user.
- Install/update docs also overstated the scope of installer config backups. The installer backs up and replaces the repository YAML catalog from `configs/firehol/` at `install.sh:166-184`, then handles Markdown templates separately at `install.sh:187-198`. Identical installed templates are left untouched, but differing repository template files are copied over the installed template directory in place; this separate template path does not create a template-specific backup and does not remove extra local files.
- Metadata/raw-file API audit found two operator-facing wording drifts:
  - `docs/glossary.md` said individual netset IPs render as `/32` prefixes, but netset rendering uses `iprange.PrintCIDR` at `pkg/downloader/canonical.go:66-77`, and `pkg/iprange/print.go:47-56` prints single-host CIDR fragments as bare IP addresses.
  - `docs/api/metadata-files.md` showed an outdated `llms.txt` example shape, while the current generator writes `# FireHOL IP Lists` and sections for primary pages, public APIs, feed surfaces, and optional metadata at `pkg/engine/output.go:1320-1359`.
- Less-touched operator-doc audit found additional current-code drifts:
  - `docs/troubleshooting/download-failures.md` suggested textual `daily` / `hourly` frequency values, but `frequency` is an integer minute field at `pkg/config/config.go:195-200`.
- Follow-up CLI example audit found one stale `enable` command example:
  - `docs/cli/enable-command.md` placed `--disable` after the feed name, but the current command uses Go `flag.FlagSet` parsing in `cmd/update-ipsets/enable.go:11-24`; flags must be provided before positional feed names. The example now uses `update-ipsets enable --disable firehol_level1`, matching the already-correct example in `docs/running/daemon-reference.md`.
- Top-level config-registry audit found a stale operator description:
  - `docs/configuration/configuration-concepts.md` described `renames.yaml` as backward-compatible aliases and `docs/feeds/yaml-field-reference.md` did not cover top-level `renames` / `deleted`. Current config decodes those fields at `pkg/config/config.go:26-27`, scheduler-style runs enable cleanup at `pkg/scheduler/processing_loop.go:48-53`, and cleanup migrates or removes local generated state at `pkg/engine/helpers.go:157-223`. The docs and config spec now describe them as cleanup registries, not public API aliases.
- Environment and telemetry audit found one current-code drift:
  - `docs/running/environment-variables.md`, `docs/monitoring/opentelemetry-setup.md`, and `.agents/sow/specs/operating-principles.md` described OpenTelemetry metric export intervals as integer milliseconds only. Current code reads `UPDATE_IPSETS_OTEL_METRIC_INTERVAL` before `OTEL_METRIC_EXPORT_INTERVAL` at `internal/observability/observability.go:223-224`; `parseMetricExportInterval` accepts integer milliseconds and also `time.ParseDuration` strings at `internal/observability/observability.go:235-254`. The docs and spec now state both accepted forms.
  - `docs/feeds/provider-databases.md` said provider databases do not appear as public feeds. Current public-feed filtering excludes ASN/GeoIP roles but not bogon roles at `pkg/engine/public.go:26-33`, and bogon provider tabs intentionally include hidden reference sources at `pkg/engine/public.go:189-211`.
  - `docs/feeds/history-derivatives.md` described only `<parent>_<N>d` suffixes, but current suffix generation also supports hour and mixed day/hour labels at `pkg/config/expand.go:437-456`; validation also rejects history windows on critical-infrastructure reference feeds at `pkg/config/validate.go:350-354`.
  - `docs/api/infrastructure-endpoints.md` described matched IP/range lists that current per-provider critical-infrastructure payloads do not expose. Current payload fields are `provider`, `provider_set_id`, `feed_ips`, `critical_ips`, and `percent` at `pkg/engine/critical.go:71-77`; aggregate payload fields are at `pkg/engine/critical.go:110-122`.
  - `docs/api/api-overview.md` flattened `503` handling. Current country/ASN aggregate entity artifacts return `503` when not ready or unreadable at `pkg/web/home_detail_api.go:35-49`, while missing feed-scoped artifacts return `404` through `pkg/web/routes.go:114-185`.
- Follow-up API/status audit found no required `docs/api/health-status.md` change: the documented public status fields match `pkg/web/public_status.go:9-43`; `globe` requires `categories` at `pkg/web/home_api.go:12-17`; `summary` has optional `categories` and a non-negative `limit` capped by default/max logic at `pkg/web/home_api.go:35-55` and `pkg/engine/home_summary.go:92-105`.
- Follow-up compose/raw-download eligibility audit found stale "provider datasets are excluded" wording in `docs/api/compose-endpoint.md` and `docs/api/raw-file-downloads.md`. Current public-feed filtering excludes hidden sources and ASN/GeoIP roles, but not bogon roles, at `pkg/engine/public.go:26-33`; public compose canonicalization then requires public-feed identity, redistributability, and raw-feed availability at `pkg/engine/public_compose.go:32-49`; raw file routes use the same public/redistributable/raw-allowed gate at `pkg/web/server.go:365-373`.
- OpenTelemetry setup docs showed port `4317` in endpoint-only examples while the daemon default protocol is `http/protobuf` at `internal/observability/observability.go:190-199`. The installed Netdata defaults explicitly switch to gRPC for `4317` at `install.sh:249-256`. OpenTelemetry Go HTTP exporter docs for `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.43.0/doc.go:5-14` and `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp@v1.43.0/doc.go:5-14` use the OTLP/HTTP default port `4318`; gRPC endpoint environment documentation for `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.43.0/doc.go:12-16` requires a scheme and host.
- New `docs/cli/cache-merge-command.md` was checked against `cmd/update-ipsets/cache_merge.go:12-70` and `scripts/sync-from-bash-version.sh:315-359`. The documented flags, required inputs, summary output, helper preference, and staging/promotion model match the current command and helper; no content patch was required for that page.
- Less-touched admin-access and integrity-recovery docs audit found two small operator clarity gaps:
  - `docs/admin-ui/accessing-admin.md` did not state the concrete fail-closed status split or split-listener public-base-url requirement. Current tests prove missing admin credentials return `503`, missing request Basic auth returns `401`, disabled admin auth requires the explicit acknowledgement flag, and split admin refuses startup without `runtime.public_base_url`: `pkg/web/feature_test.go:19-83`, `pkg/web/feature_test.go:236-245`, `pkg/web/feature_test.go:923-943`, and `pkg/web/run_lifecycle_test.go:71-205`. The page now records the same behavior.
  - `docs/integrity/recovery-model.md` described recheck/reprocess correctly at a high level, but did not spell out current merge recheck semantics or that the recovery endpoint returns split `recheck_names` / `reprocess_names` and reports `in_progress` while an engine run is active. Current behavior is in `pkg/engine/integrity_recovery.go:9-77` and `pkg/web/integrity.go:249-330`. The page now records those operator-visible details.
- TLS/proxy/security no-patch audit found no required docs change:
  - `docs/installation/tls-configuration.md`, `docs/security/production-deployment.md`, `docs/security/security-overview.md`, `docs/running/daemon-reference.md`, and `docs/configuration/runtime-settings.md` already document the current `--tls-cert`, `--tls-key`, `--trust-proxy-headers`, `--trust-cloudflare-headers`, `trust_proxy_headers`, `trust_cloudflare_headers`, and `runtime.public_base_url` behavior. Current code registers those daemon flags at `cmd/update-ipsets/daemon.go:32-37`, applies runtime trust settings at `cmd/update-ipsets/daemon.go:103-117`, uses TLS when both cert and key are configured at `pkg/web/server.go:170-174`, and resolves client IPs in `pkg/web/middleware.go:168-199`.
- Memory/resource audit found one operator-doc overclaim:
  - `docs/installation/memory-planning.md` described the daemon as not loading IP sets into RAM and the whole processor pipeline as line-by-line. Current long-lived query/comparison paths are file-backed through `pkg/iprange/fileset.go:29-63`, `pkg/iprange/fileset.go:236-244`, and `pkg/engine/query.go:207`, and HTTP/local downloads stream to temp files through `pkg/downloader/downloader.go:248-277` and `pkg/downloader/downloader.go:398-426`. Current active processing still builds an in-memory `IPSet` range slice at `pkg/iprange/set.go:11-20`, parses canonical feed bodies into that set at `pkg/downloader/canonical.go:37-64`, renders the canonical body into bytes at `pkg/downloader/canonical.go:66-83`, and uses that set during finalization at `pkg/engine/process.go:102-141`. The operator doc and memory spec now distinguish long-lived file-backed serving/comparison behavior from per-worker active feed processing memory.
- Suspected application issue for review: `ignore_repeating_download_errors` is an accepted runtime field with default `10` in `pkg/config/config.go:137` and `pkg/config/config.go:611`, is copied into runtime state in `pkg/engine/runtime.go:144` and defaulted again in `pkg/engine/runtime.go:189-190`, and is passed to `nextDue` from `pkg/scheduler/snapshot_build.go:56` and `pkg/scheduler/snapshot_build.go:105`. The `nextDue` parameter is not used in `pkg/scheduler/snapshot_build.go:174-234`; current retry timing is driven by failure count and health class through `failureRetryDelayMinutes` in `pkg/scheduler/snapshot_build.go:237-264`.
- Suspected application issue for review: `cmd/update-ipsets/main.go:27-28` supports the `cache-merge` subcommand, but `cmd/update-ipsets/main.go:46-56` omits `cache-merge` from the top-level usage text shown by `update-ipsets help`.
- Suspected application issue for review: `skip_comparison_if_no_updates` is accepted and defaulted true in `pkg/config/config.go:154` and `pkg/config/config.go:624`, but current no-update runs do not publish public artifacts because `pkg/engine/run_pipeline.go:161-166` returns when `plan.shouldPublish` is false. The field affects `plan.skipHeavy` in `pkg/engine/run_pipeline.go:154-160`, but setting it false does not by itself force a no-update heavy regeneration.
- Suspected application issue for review: the feed detail drawer labels retry behavior as "linear backoff" at `ui/src/components/admin/feed-modal-status-sections.tsx:185-191`, but scheduler retry delay doubles with repeated failures until capped by the configured cadence or the 30-day maximum at `pkg/scheduler/snapshot_build.go:237-264`.
- Suspected application issue for review: `enabled_by_all` is accepted and tested as converted catalog metadata, but runtime enable-all paths do not use it. Current docs/specs now describe the observed behavior as-is; product review should decide whether the field should be implemented, deprecated, or removed from operator examples.
- Suspected application issue for review: the MCP `fetch_analysis` tool description says ASN names may include an `AS` prefix at `pkg/mcp/server.go:147-151`, but `pkg/mcp/fetch_analysis.go:26-35` maps ASN names directly to `asns/{name}.md` and does not normalize or strip the prefix. Current public docs use numeric ASN examples only.
- Suspected application issue for review: `install.sh` parses a custom install directory at `install.sh:54-73` and copies files under that path at `install.sh:148-184`, but the generated systemd unit still hardcodes `/opt/update-ipsets` for `ExecStart`, path environment variables, `WorkingDirectory`, and `ReadWritePaths` at `install.sh:223-280`. Current docs now warn operators not to treat a custom path as a complete managed-service install.
- Cleared previous suspected application issue: current `BackgroundTaskHandle.Update` locks `h.engine.mu` once at `pkg/engine/background_tasks.go:130-131` and updates the task state before releasing it. There is no repeated engine-mutex lock in the current implementation.

Risks:

- Leaving false route or flag docs creates direct operator failure: copy-pasted commands and API calls will not work.
- Leaving internal TODO history under `docs/` contradicts the requested surface split and can confuse operators about current behavior.
- Moving or deleting docs without approval risks violating repository rules and losing preserved history.
- Over-rewriting config docs as "operator-only" could accidentally drop useful catalog-curation guidance that operators need when maintaining a private catalog.
- Incorrect spec updates are dangerous because future agents treat specs as normative product contracts; spec edits must be tied to current-code evidence and must not normalize obvious bugs as intended behavior.
- The current general rate limiter's admin-API coverage may be intentional shared protection or accidental path-prefix fallout. This pass records the behavior as-is in specs, but keeps the product question visible for review.
- Stale code references to moved history files can mislead maintainers and operators. The stale references found in this pass were updated to current specs/operator docs.
- The `ignore_repeating_download_errors` runtime field may create false operator expectations because the scheduler currently accepts and carries the value without using it for retry timing.

## Pre-Implementation Gate

Status: ready for evidence-backed spec-as-is updates; operator-doc repairs, approved todo-history move, and GitHub Wiki publish/link repairs have been applied.

Problem / root-cause model:

- The `docs/` tree was expanded broadly, but several pages mix audiences and some were not rechecked against current route/flag/config structures. Evidence: stale `--details` flag, nonexistent `/api/v1/sets/about/{name}` route, incorrect `--web-files-dir` meaning, and internal TODO history inside `docs/`.
- The specs are extensive and normative, but the user has requested verification that they describe the current application as-is. The audit starts from the assumption that spec drift is possible, not proven, and must be checked against current code, config, routes, docs, generated/static surfaces, and tests before editing.

Evidence reviewed:

- Code route registration: `pkg/web/routes.go:26-58`, `pkg/web/routes.go:270-290`, `pkg/web/routes.go:364-386`.
- CLI flag sets: `cmd/update-ipsets/daemon.go:22-37`, `cmd/update-ipsets/query.go:18-23`, `cmd/update-ipsets/enable.go:14-18`, `cmd/update-ipsets/cache_merge.go:16-19`.
- Runtime path behavior: `pkg/web/surface_routes.go:30-42`, `pkg/web/routes.go:164-174`.
- Config field model: `pkg/config/config.go:115-251`, `pkg/config/config.go:428-467`.
- Docs evidence listed in the analysis section above.
- Spec map and ownership: `.agents/sow/specs/README.md`.
- Spec surface rules: `.agents/skills/project-content-surfaces/SKILL.md`.
- Sync workflow: `/home/user/.agents/skills/sync-docs-specs-skills/SKILL.md`.

Affected contracts and surfaces:

- Operator docs under `docs/`.
- API Reference under `docs/api/`.
- Wiki/navigation pages `docs/Home.md` and `docs/_Sidebar.md`.
- Product/application specs under `.agents/sow/specs/`.
- Potentially misplaced internal documentation under `docs/todo-history/`.
- Catalog-maintenance docs under `docs/contributing/`; content is operator-facing, path remains unchanged until a file move is explicitly approved.
- Runtime project skills only if the audit finds durable agent rules missing; none identified yet.

Existing patterns to reuse:

- Keep operator docs task-oriented and concise, matching the existing section structure.
- Keep route and flag references as tables with examples.
- Use current config field names from `pkg/config`.
- Use `docs/api/mcp-endpoint.md` as the existing MCP API reference page, and add it to navigation rather than duplicating it.

Risk and blast radius:

- Documentation-only changes have no runtime blast radius.
- Spec changes have maintainer/agent blast radius because they define expected behavior for future work; wrong spec edits can legitimize current bugs or create false contracts.
- Removing or moving files has repository-history and navigation blast radius, so only the user-approved `docs/todo-history/` move was performed.
- Editing config examples can affect operators who copy them into private catalogs, so examples must be checked against the current YAML loader.
- Security docs must not encourage unsafe admin exposure or trusted-proxy misuse.
- The spec audit must not change code to match specs during this pass. Suspected code defects are SOW findings for review.

Sensitive data handling plan:

- Do not include raw credentials, bearer tokens, private keys, session cookies, customer names, customer identifiers, personal data, non-private customer-identifying IPs, private endpoints, or proprietary incident details in the SOW, docs, specs, skills, or comments.
- Use documentation-safe placeholders such as `example.com`, `192.0.2.0/24`, `[REDACTED_SECRET]`, and environment-variable names.
- Existing example credentials such as `admin` / `secret` should be replaced where touched.

Implementation plan:

1. Repair mechanical API and CLI mismatches in `docs/api/`, `docs/cli/`, `docs/running/`, `docs/security/`, and sidebar navigation.
2. Repair configuration and feed catalog examples so they use current YAML fields and operator wording.
3. Apply the approved decision for `docs/todo-history/`; rewrite `docs/contributing/` content in place as catalog-operator guidance without moving files.
4. Audit specs against current code and docs by product area: configuration, feed model, downloader, processing pipeline, file layout, integrity, web/public API, admin UI, memory/operating principles, and compatibility.
5. Update specs only where evidence shows they do not describe the current application as-is.
6. Record suspected bugs, wrong logic, or product-quality concerns in this SOW with file/line evidence for later review.
7. Run the same-scope audit again and repair remaining required mismatches.

Validation plan:

- Extract registered routes from `pkg/web/routes.go` and compare against documented API paths.
- Extract CLI flags from `cmd/update-ipsets/*.go` and compare against documented flags.
- Search `docs/` for internal-only terms and stale implementation references.
- Search `docs/` for local markdown links ending in `.md`; none should remain except literal examples that intentionally refer to filenames.
- Search specs for aspirational or future-tense contracts and verify each against code or reword/remove it.
- Compare registered routes, CLI flags, config fields, generated artifact writers, public artifact readers, and admin handlers against the relevant specs.
- Run markdown link validation with a local script or shell checks for relative links.
- Run `make test` only if documentation edits alter examples that are validated by tests or if the audit changes generated docs assumptions; otherwise record why docs-only validation is sufficient.

Artifact impact plan:

- AGENTS.md: likely unaffected; no project-wide workflow rule change identified.
- Runtime project skills: likely unaffected unless the audit finds a repeatable docs/surface rule missing.
- Specs: primary artifact class for the new user request; update `.agents/sow/specs/` where current behavior is not accurately described.
- End-user/operator docs: primary artifact class; update `docs/`.
- End-user/operator skills: none currently present outside project runtime skills.
- SOW lifecycle: keep this SOW current; close only after final same-scope docs/spec audit passes or remaining non-fixes are explicitly mapped.

Open-source reference evidence:

- None checked. This work is local application documentation synchronization; the authoritative evidence is the current application code and project specs.

Open decisions:

1. `docs/todo-history/` handling resolved by user decision on 2026-05-31: move it to `.agents/sow/todo-history/`.
2. `docs/contributing/` path rename remains optional and requires explicit user approval; content was rewritten in place as catalog-operator guidance.
3. Admin API rate limiting remains a product decision for review: specs now describe current behavior, where `/api/v1/admin/*` shares the `/api/` 240/min limiter.
4. Stale `docs/todo-history/` code references were cleaned up after the approved move because one reference was operator-facing validation output.
5. `ignore_repeating_download_errors` remains a product/code review item: docs now describe current behavior, but the application may need either implementation or deprecation of the inert field.

## Implications And Decisions

Recorded user decisions:

1. `docs/todo-history/`
   - Option A: Move it out of `docs/` into SOW/spec history storage. Pro: `docs/` becomes operator-only. Con: file move touches many historical files and needs explicit approval. Risk: links from old wiki pages break unless redirects or pointers are added.
   - Option B: Keep the directory but remove it from operator navigation and replace `docs/todo-history/README.md` with a short "not operator docs" pointer. Pro: minimal disruption, no mass move. Con: internal history still lives under `docs/`, so the folder is not pure operator material. Risk: search users can still find old internal notes.
   - Option C: Keep it unchanged. Pro: lowest change. Con: contradicts the stated operator-doc goal. Risk: stale implementation history keeps confusing readers.
   - Recommendation: A, if the user approves a file move; otherwise B as a conservative interim.
   - User decision: A, with target `.agents/sow/todo-history/`.

2. `docs/contributing/`
   - Option A: Rewrite in place as "catalog operations" docs for operators who maintain a private catalog, removing PR/fork workflow. Pro: preserves useful feed-addition guidance while matching operator purpose. Con: path name remains `contributing/` unless separately moved. Risk: public contributor workflow disappears from `docs/`.
   - Option B: Move contributor workflow out of `docs/` and keep only operator catalog-maintenance docs under `docs/configuration` or `docs/feeds`. Pro: cleanest surface split. Con: requires explicit file moves. Risk: external links to contributor docs break.
   - Option C: Keep contributor docs and classify them as the public contributor exception. Pro: low disruption. Con: conflicts with "Only the API Reference is third party developer documentation." Risk: audience mixing remains.
   - Recommendation: B for a clean final state, or A if preserving paths matters more than purity.
   - Working decision for initial pass: A. The content was rewritten as catalog-operator material.
   - Final user decision on 2026-06-01: rename the path to `docs/catalog-maintenance/` and commit the closed SOW with the docs/spec/wiki changes.

## Plan

1. Fix docs/code mismatches that are independent of the two content-scope decisions.
2. Record the user's choices for `docs/todo-history/` and `docs/contributing/`.
3. Apply the chosen restructuring or rewrite.
4. Repeat the full docs audit, fix remaining required issues, and validate links/routes/flags.
5. Audit and repair specs so they describe the current application as-is.
6. Record suspected application issues in this SOW for review instead of changing code.
7. Update validation, artifact maintenance, lessons, and outcome before closing.

## Execution Log

### 2026-05-31

- Loaded project content-surface and docs/specs/skills synchronization skills.
- Ran docs inventory and inspected current CLI, route, config, and docs evidence.
- Created this SOW before operator-doc repairs.
- Repaired API docs to match current route registration:
  - `/mcp` is documented as a public endpoint outside `/api/v1/` with `GET`, `POST`, and `DELETE`.
  - Removed the nonexistent `/api/v1/sets/about/{name}` route.
  - Documented `/api/v1/sets/{name}/comparison` as an alias for `/compare`.
  - Corrected `/api/v1/home/globe` and `/api/v1/home/summary` query-parameter behavior.
  - Corrected `all-ipsets.json` as legacy metadata, not a bulk feed-body dump.
- Repaired CLI and daemon docs to match current flags:
  - Removed nonexistent query `--details`.
  - Corrected query output and `--silent` semantics.
  - Corrected `--web-files-dir` as the raw `.ipset` / `.netset` serving directory.
  - Added trust-proxy flags and the `cache-merge` subcommand.
- Repaired catalog docs and examples:
  - Changed feed family count to six.
  - Changed `frequency` descriptions from seconds to minutes.
  - Replaced stale YAML field names and processor examples with current fields.
  - Aligned direct-upstream license and redistribution wording with project rules.
- Rewrote `docs/contributing/*.md` in place as catalog-maintenance documentation for operators. No file move was performed.
- Moved `docs/todo-history/` to `.agents/sow/todo-history/` after explicit user approval and updated `AGENTS.md` to point to the new location.
- Updated `.gitignore` to allow tracked `TODO-*.md` files under `.agents/sow/todo-history/`, preserving the moved history instead of committing deletions without replacements.
- Added `scripts/build-wiki.mjs` and changed `.github/workflows/wiki-sync.yml` so wiki publishing flattens docs into wiki-root page filenames, rewrites local docs links to wiki-safe generated targets, and re-runs when the builder changes. This keeps source docs usable in the repository while preventing GitHub Wiki links from opening raw `.md` files.
- Repaired the follow-up GitHub Wiki sidebar bug by changing generated wiki links from bare extensionless slugs to full GitHub wiki URLs. The workflow passes `https://github.com/${GITHUB_REPOSITORY}/wiki` to the builder, and local builds default to `https://github.com/firehol/update-ipsets/wiki`.
- Restored local Markdown links under `docs/` to repository-relative `.md` targets so source docs links resolve correctly when viewed from the code repository.
- Repaired security docs so `/admin` browser shell pages are described as outside the `/api/` rate limiter while `/api/v1/admin/*` remains under the general API limiter.
- Expanded this SOW to include the user's spec-as-current-application audit request rather than creating a parallel current SOW.
- Audited the critical-infrastructure `provider_set_id` contract across code, tests, and specs. Updated specs to current behavior: admin integrity remains strict; public routes are cache-first and do not enforce provider-set identity equality at request time.
- Updated the critical provider-set identity specs to match current code: the identity is derived from stable catalog/source configuration and configured critical ASN context, not materialized provider content hashes or cardinality.
- Updated operating-principles rate-limit wording to current middleware behavior: `/api/*` and `/mcp` share the 240/min limiter, including authenticated admin API routes, while search/query add a 10/min limiter.
- Added the current `/api/v1/compose` bounds and raw-feed eligibility contract to the website spec.
- Refreshed architecture-posture measured highlights from `go run ./tools/archposture -root .`.
- Cleaned stale `docs/todo-history/` code references after the approved move:
  - `pkg/insights/insight.go` now points maintainers to `.agents/sow/specs/processing-engine.md`.
  - `pkg/insights/insights.go` now describes the file as the processing-engine spec entry point.
  - `pkg/config/config.go` now points removed-block validation errors to `docs/feeds/use-roles.md` and `docs/migration-from-bash.md`.
  - Feed-detail UI comments no longer point to removed TODO-history files.
- Repaired additional docs drift found during the repeat audit:
  - Reload docs now use `systemctl kill -s HUP update-ipsets` and `kill -HUP <update-ipsets-pid>` instead of unsupported PID-file or `systemctl reload` examples.
  - Admin-auth docs now distinguish missing credentials (`503 Service Unavailable`) from wrong credentials (`401 Unauthorized`) and state that required auth without configured credentials blocks admin requests rather than falling back to open access.
  - Runtime docs now include current path, download, git, chart, web, proxy, and apply settings and document `ignore_repeating_download_errors` as an accepted field that does not currently drive retry timing.
  - Password examples now use placeholder-style values instead of `secret`.
- Repaired additional operator-doc drift found after the GitHub Wiki navigation fix:
  - `iprange` docs now describe the current flag-based CLI, output modes, IPv6 mode, compare summary output, and local-file-only scope.
  - Critical-infrastructure reference docs now describe operator-visible catalog behavior and cache-first public serving instead of internal provider-set artifact mechanics.
  - Feed YAML docs now include current downloader, downloader-options, public URL, and AI/context attribute fields used by the active catalog and downloader.
  - Install and update docs now match the current repository URL, Go 1.26 requirement, UI build commands, and config-backup directory naming.
  - Config-update docs now state that a differing installed config directory is backed up and replaced as a whole, matching `install.sh`.
- Repaired repeat-audit operator-doc drift:
  - `/api/v1/status` docs now show the current `engine` and `system` response shape, and binary-update docs no longer query a nonexistent `.version` field.
  - API docs now include `/api/v1/client-ip`, direct root-level raw feed compatibility paths, current compose bounds, current compose format aliases, and the 32 MiB compose output cap.
  - Rate-limit docs now state that search/query requests consume the general `/api/` bucket and then the stricter search bucket.
  - Install/systemd/env docs now use the correct drop-in path, service restart/start semantics, and installed `$HOME/.update-ipsets.env` path.
- Repaired additional repeat-audit operator-doc drift:
  - Integrity docs now describe the current `Re-check` and `Recover all` workflow instead of unsupported per-finding recovery.
  - `iprange` docs now document current reduction flag semantics and the full supported flag set exposed by the compatibility CLI.
  - Query docs now document compose include/exclude bounds, output cap, and accepted format aliases.
  - Added `docs/cli/cache-merge-command.md` and linked it from the sidebar and README CLI list.
  - Runtime settings docs now classify `ipset_reduce_factor` and `ipset_reduce_entries` as accepted compatibility fields that are not used by current Go public output publishing.
  - Methodology API docs now match the current `items` index envelope and rendered-HTML body field.
  - Integrity finding-class docs now describe older reference-provider sets without exposing the internal `provider_set_id` field name.
  - Search/API security docs now state that search requests consume the general `/api/` limiter before the stricter search limiter.
  - Runtime settings docs now include `skip_comparison_if_no_updates` and the `feed_health_category_thresholds` subfields `healthy_cadence_minutes` / `risky_cadence_minutes`, with current validation constraints.
  - Feed-name docs now include the reserved filename characters rejected by config validation.
  - Added `docs/feeds/processors.md` and linked it from source-feed docs, the sidebar, and the operator-manual catalog flow.
  - OpenTelemetry setup and environment docs now include per-signal OTLP endpoint variables and standard per-signal exporter suppression for traces, metrics, and logs.
  - Monitoring overview, telemetry reference, Netdata integration, memory-planning, and troubleshooting docs now use the current admin status shape (`metrics`, `engine.lifetime_metrics`, `queues`, and `system`) instead of stale `counters.*`, `process.*`, and `scheduler_state` examples.
  - Telemetry reference now distinguishes admin status snapshots from OpenTelemetry metrics, documents current scheduler counters, system fields, engine/HTTP/entity/cache/config/processor/file/iprange metric namespaces, and explains the `<metric>.bytes` / `<metric>.duration_ms` OpenTelemetry suffix behavior.
- Repaired current admin UI documentation drift:
  - Runtime status docs now describe the heartbeat strip, the current status API blocks, and where process-level resource fields actually live.
  - Schedule docs now describe schedule state in the feed table, feed detail drawer, artifact inventory, and admin schedule API instead of a nonexistent standalone schedule panel.
  - Operator-action docs now describe broad reprocess as limited to feeds with local staged or committed input, and history derivative recheck fallback to the parent.
  - Feed inventory and artifact inventory docs now match the current visible table columns, detail drawer sections, and artifact action behavior.
  - Live queue docs now refer to the current-run panel columns below the heartbeat.

### 2026-06-01

- Re-ran a broad operator-doc scan across docs, README, routes, CLI help, install scripts, migration scripts, security docs, and less-audited API/security/MCP surfaces.
- Repaired IPv6 product-boundary wording in `README.md` and `docs/about-update-ipsets.md`: the shipped feed catalog and public lookup/enrichment pipeline remain IPv4-oriented, while the standalone `iprange` CLI supports IPv6 set operations.
- Repaired `docs/migration-from-bash.md` so it documents the canonical helper signature, default install directory, legacy path override variables, staged import directory, backup behavior, legacy cache merge, env-file extraction, summary output, and restart behavior.
- Repaired `docs/security/admin-authentication.md` so missing required admin credentials are documented as `503 Service Unavailable`, while wrong credentials are documented as `401 Unauthorized`.
- Repaired stale API response field examples in feed, country, ASN, and maintainer API docs so they match the current public JSON payloads.
- Repaired pipeline and troubleshooting docs so downloader changed-content status is `downloaded`, current processing status `invalid_input` is covered, provider/support-data statuses are separated, and nonexistent `integrity_failed` last-status wording is removed.
- Repaired install/config/reload docs so operators can see that Markdown templates live under the installed config tree, generate published `.md` artifacts, and require a service restart after template edits.
- Repaired admin operator docs so feed actions, artifact actions, feed-output integrity recovery, and entity-integrity rebuilds include their authenticated admin API endpoints.
- Repaired config-update docs so repository paths are generic and installer restart behavior matches `install.sh`: active services restart automatically unless `--no-restart` is used.
- Repaired critical-infrastructure redistributability wording in operator docs and `.agents/sow/specs/config.md`: the `critical_infrastructure` role no longer implies `redistributable: false`; direct-upstream terms, merge inheritance, and operator policy for static private data control raw-body publication.
- No runtime-skill update was needed for these repairs. Specs were already current for the Markdown template directory and generated markdown artifact contract; `.agents/sow/specs/config.md` was updated only for the verified critical-infrastructure redistributability rule drift.
- Repaired role-sensitive merge-health wording in admin, feed, pipeline, troubleshooting, and integrity docs: excluded additive parents can leave a merge usable when at least one additive parent remains, but unavailable subtractive parents block composition.
- Repaired configuration-reload docs so effective enable/disable state is described as recalculated from the reloaded catalog and existing enable marker files.
- Repaired split-listener docs so `runtime.public_base_url` is documented as required for `--admin-listen`, and so the admin listener is described as serving admin routes and embedded SPA assets, not public API/artifact routes.
- Added systemd notification variables to environment docs as automatically supplied variables, not operator drop-in knobs.
- Repaired logging docs so local stderr/journald output is described as Go `slog` text format with structured `key=value` attributes, not JSON. Troubleshooting log filtering now uses `level=ERROR` text records.
- Repaired `enabled_by_all` docs and `.agents/sow/specs/config.md` so the field is described as accepted legacy catalog metadata while current `--enable-all` behavior is documented as a global runtime override that enables every configured source.
- Repaired critical provider-set trigger docs so provider-set drift is described as stable configured-reference-set identity drift, not materialized content/entry-count drift.
- Repaired quick-start first-run instructions so a fresh checkout starts with `--enable-all` and warns that initial catalog downloads/processes continue in the background before query results are expected.
- Repaired feed API payload docs for history CSV timestamps, pairwise comparison row fields, and retention JSON series fields after a targeted API payload audit.
- Repaired the merge troubleshooting jq command so it uses the current admin payload fields `merge_included`, `merge_subtracted`, and `merge_excluded`.
- Repaired admin operator-action docs so artifact-child recheck behavior matches the current parent-fallback logic.
- Repaired install/runtime path docs so custom install-directory usage is documented as manual/experimental unless the operator also maintains a matching systemd override; added the current non-root user-mode state paths to environment-variable and runtime-setting docs; fixed the glossary's installed web-directory default to `/opt/update-ipsets/web/`.
- Repaired metadata/raw-file API docs so the glossary describes actual netset single-host rendering and the `llms.txt` example structure matches the current generated metadata headings.
- Repaired less-touched operator docs for numeric frequency examples, provider database public-visibility semantics, history-derivative suffix and eligibility rules, critical-infrastructure API payload fields, and public API `503` versus `404` error semantics.
- Confirmed `docs/api/health-status.md` already matches the current public status and homepage-data handler contracts; no docs patch was required for that page.
- Repaired compose and raw-download API docs so ASN/GeoIP provider databases are described as excluded public bodies while public redistributable bogon sources remain eligible when they satisfy the same raw-feed gates.
- Repaired OpenTelemetry setup/environment docs so endpoint-only examples use the OTLP/HTTP port `4318`, while Netdata/plaintext gRPC examples keep `4317` together with `UPDATE_IPSETS_OTEL_PROTOCOL=grpc` and an explicit `http://` scheme.
- Confirmed the new cache-merge CLI doc matches the current command and canonical bash-migration helper; no docs patch was required for that page.
- Repaired search rate-limit and scoped-search docs so per-feed `/api/v1/sets/{name}/search` and `/api/v1/ipsets/{name}/search` are included in the 10/min search bucket, and so scoped misses are documented as HTTP 200 with an empty `matches` array.
- Audited the MCP endpoint docs against the current registered tools. Later precision repairs documented the current maintainer substring filter and numeric-only MCP ASN markdown identifiers; the remaining MCP issue is the already-recorded application-review note that the tool description accepts `AS`-prefixed ASN names while markdown lookup does not normalize that prefix.
- Repaired less-touched configuration/feed-family docs:
  - category `public: false` is now documented as taxonomy/aggregation visibility, not as a feed privacy control; `.agents/sow/specs/config.md` was updated with the same current-behavior contract
  - provider defaults now document insights/entity-page usage and fallback to first configured provider when a default is omitted
  - feed-family and artifact-parent docs now describe bogon provider-style sources and the currently supported `dronebl_buildzone` artifact type
- Repaired pipeline publication-order docs and specs. Current processing claims staged feed bodies into `.processing`, finalizes successful normal feed bodies into committed canonical feed bodies during feed-local processing, stages public artifacts separately, promotes supporting staged provider/artifact inputs before public artifact publication, publishes staged artifacts, then saves cache state; docs and `.agents/sow/specs/{downloader,processing-engine,pipeline,files-layout}.md` now describe that order.
- Repaired methodology API examples so the feed-health page uses the current embedded slug `feed-health`, not the nonexistent slug `health`.
- Repaired migration-from-bash docs so the legacy API-key extraction list includes the current helper's preserved variables: `AUTOSHUN_API_KEY`, `BLUELIV_API_KEY`, `XFORCE_API_KEY`, `XFORCE_API_PASSWORD`, `IP2LOCATION_API_KEY`, and `MAXMIND_LICENSE_KEY`.
- Repaired source-feed and YAML field-reference docs so `attributes.downloader_options` documents the actual supported curl-like option surface, including `--data`, `--data-raw`, `-d`, `--request`, `-X`, `--referer`, `--user`, `-u`, `--header`, `-H`, and the accepted equals forms except for headers.
- Repaired homepage health/status API docs so `/api/v1/home/summary` documents the current `limit` behavior: omitted or `0` uses the default of 20, negative values are rejected, and values above 200 are clamped to 200.
- Rechecked compose, raw-file, metadata, critical-infrastructure, install, TLS, systemd, runtime environment, and CLI docs against current code after the latest repairs; no additional docs patch was required for those surfaces.
- Repaired production reverse-proxy guidance so it no longer claims admin actions send JSON request bodies. Current admin UI action calls use bodyless POST requests, while the public `/mcp` endpoint sends small JSON-RPC messages.
- Repaired `use: [bogons]` role wording so public catalog visibility is conditional on `hidden: true`; configured hidden bogon providers can still appear in per-feed bogon provider tabs.
- Repaired the MCP API Reference so it lists the exact registered `POST /mcp`, `GET /mcp`, and `DELETE /mcp` route forms instead of describing them only in prose.
- Expanded the Feed Endpoints API Reference so the public feed catalog and single-feed detail sections enumerate the current JSON field groups from `PublicFeedSummary` and the per-feed metadata schema.
- Rechecked the previously unchanged operator docs (`admin-ui/accessing-admin.md`, `admin-ui/background-work.md`, `cli/enable-command.md`, `contributing/contribution-guide.md`, `integrity/recovery-model.md`, and `pipeline/pipeline-overview.md`) against current code and UI.
- Repaired `docs/pipeline/pipeline-overview.md` and `docs/pipeline/processing-lifecycle.md` so operator-facing pipeline docs no longer describe a late `.processing` promotion as the visibility boundary. They now match `pkg/engine/process.go`, `pkg/engine/finalize.go`, `pkg/scheduler/processing_loop.go`, and `pkg/engine/download_stage.go`.
- Repaired `docs/admin-ui/background-work.md` so it describes the current Background Work section inside the runtime/queues panel, current entity-artifact background task classes, coalesced pending work, worker counts, and the distinction between feed-output integrity queue recovery and entity-artifact background repair.
- Repaired `.agents/sow/specs/downloader.md`, `.agents/sow/specs/files-layout.md`, `.agents/sow/specs/pipeline.md`, and `.agents/sow/specs/processing-engine.md` for the same current processing/publication order and for the current `finalize_failed` meaning.
- Repaired memory planning docs and `.agents/sow/specs/memory-management.md` so they no longer overclaim that all IP set handling and all processor pipelines are fully out-of-core. They now document the current split: long-lived lookup/comparison and raw downloads are file-backed/streamed, streamable processor segments stay streaming, and each active processing worker can hold the current range set plus canonical output in memory.

## Validation

Acceptance criteria evidence:

- Route evidence checked against:
  - `pkg/web/routes.go:26-58` for public `/mcp`, `/healthz`, `/api/v1/*`, search, compose, and feed routes.
  - `pkg/web/routes.go:89-170` for feed-scoped actions (`data`, `history`, `changesets`, `compare`, `comparison`, `retention`, `insights`, `countries`, `asn`, `bogons`, and `infrastructure`).
  - `pkg/web/routes.go:270-290` for admin API routes.
  - `pkg/web/routes.go:364-386` for raw files, methodology, and SPA routes.
- Flag evidence checked against:
  - `cmd/update-ipsets/daemon.go:22-37`.
  - `cmd/update-ipsets/query.go:18-23`.
  - `cmd/update-ipsets/enable.go:14-18`.
  - `cmd/update-ipsets/cache_merge.go:16-19`.
  - `pkg/iprange/cli.go:101-159` and `pkg/iprange/cli6.go:85-139`.
- Rate-limit evidence checked against `pkg/web/middleware.go:74-87`, `pkg/web/surface_routes.go:50`, and `pkg/web/search_api.go:18-103`.
- Home API query-parameter behavior checked against `pkg/web/home_api.go:12-55`.
- Config/feed docs checked against current public field names and scheduler evidence including `pkg/engine/public_catalog.go:41` and scheduler `FrequencyMinutes` use.
- Category visibility evidence checked against `pkg/engine/public.go:26-33`, `pkg/engine/public_catalog.go:67-78`, `pkg/engine/public_categories.go:13-27`, `pkg/engine/home_summary.go:127-130`, and `pkg/engine/home_detail_helpers.go:44-52`.
- Provider-default evidence checked against `pkg/config/validate.go:472-504`, `pkg/engine/insights.go:269-296`, and `pkg/engine/provider_defaults.go:29-62`.
- Artifact-type evidence checked against `pkg/config/artifacts.go:12` and `pkg/config/validate.go:151-316`.
- Processing publication-order evidence checked against `pkg/scheduler/processing_loop.go:37-72`, `pkg/engine/run.go:89-123`, and `pkg/engine/run_pipeline.go:312-390`.
- Methodology slug evidence checked against `pkg/web/methodology.go:43-76` and the embedded files under `pkg/web/static/methodology/*.md`.

Tests or equivalent validation:

- `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test` passed and reported: `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`.
- `node --check scripts/build-wiki.mjs` passed.
- Generated wiki output validation passed: every local generated wiki link resolves to a generated wiki-root page, and no generated wiki link contains `.md` or a path segment.
- Source docs link validation passed: every local source-doc Markdown link resolves to an existing repository docs `.md` file.
- Focused stale-content scan returned no matches for stale route, flag, processor, category, frequency, and static-asset wording:
  - `five feed types`
  - `frequency.*seconds`
  - `Seconds between`
  - `number of seconds`
  - `Expressed in seconds`
  - `strip_comments`
  - `strip_blank_lines`
  - `cidr_expand`
  - `category: combined`
  - `web_reputation`
  - `/api/v1/sets/about`
  - `--details`
  - `dns-root`
  - `all public API endpoints live under`
  - `all public endpoints accept`
  - `static web assets`
  - `All admin endpoints`
  - `Disable the daemon`
- Repeat stale-content scan also returned no matches for reload/auth/password drift:
  - `systemctl reload`
  - `/run/update-ipsets.pid`
  - `admin endpoints return 401`
  - `UPDATE_IPSETS_ADMIN_PASSWORD=secret`
  - ``every `--interval` seconds``
  - ``All public API endpoints (except `/healthz` and admin endpoints)``
- Repeat install/update drift scan returned no stale blocking matches for:
  - `Go 1.22`
  - `github.com/firehol/firehol.git`
  - `cd firehol/update-ipsets`
  - stale `pnpm install` / `pnpm build` wording
  - stale `config.2025-...` backup paths
  - stale claims that unchanged manually edited config files remain untouched during reinstall
- Repeat `iprange` and critical-infrastructure drift scans found no remaining stale examples for old subcommand-style `iprange` usage or old public critical-overlap provider-set rejection wording.
- Repeat post-wiki audit found and repaired stale status, compose, raw-file, rate-limit, systemd, install, and env-file wording as recorded above.
- Repeat CLI/admin-integrity audit found and repaired stale integrity recovery wording, stale iprange reduction semantics, and missing `cache-merge` CLI reference.
- Runtime field usage scan found and repaired stale `ipset_reduce_factor` / `ipset_reduce_entries` output-generation wording.
- Methodology endpoint audit found and repaired the stale index envelope and body-format wording.
- Operator-facing integrity docs no longer expose the internal `provider_set_id` field name; the remaining `docs/` match is in `docs/api/infrastructure-endpoints.md`, where it is a documented API response field.
- Repeat stale rate-limit scan returned no matches for independent search-limit wording after repairing `docs/api/search-query.md` and `docs/security/security-overview.md`.
- `go run ./cmd/update-ipsets help` confirmed the recorded application-review note: top-level help lists `iprange`, `query`, `enable`, `daemon`, and `version`, but omits the supported `cache-merge` subcommand.
- Runtime YAML field coverage scan found and repaired missing docs for `skip_comparison_if_no_updates`, `healthy_cadence_minutes`, and `risky_cadence_minutes`.
- Monitoring telemetry scan found and repaired stale `counters.*`, `process.*`, `scheduler_state`, old `download.download_failed`, old comparison/entity metric names, and stale process-resource wording in monitoring, Netdata, memory-planning, and troubleshooting docs.
- Repeat CLI flag coverage reported all documented flags present: daemon 16, query 6, enable 5, cache-merge 4, and iprange 53.
- Processor coverage found no missing docs entries; the later full registry audit below established the current registry count as 77.
- Environment-variable coverage reported 29 expected runtime/OpenTelemetry/admin variables covered by docs.
- Configuration-field coverage reported 107 active YAML field names covered by configuration/feed docs.
- Telemetry metric coverage reported 144 literal metric names from code covered directly or by documented prefix patterns in `docs/monitoring/telemetry-reference.md`.
- Focused stale telemetry scan returned no matches for the old `counters.download`, `counters.engine`, `counters.iprange`, `scheduler_state`, `process.cpu_*`, `process.memory_*`, old download-status names, old comparison names, or old entity-sidecar names.
- Focused admin UI stale scan returned no matches for old admin-doc wording:
  - `Schedule Panel`
  - `top panel`
  - `process-level resource usage`
  - `Force reprocess all feeds`
  - `all feeds.`
  - `Current pipeline state`
  - `four live-list panels at the top`
  - `Last check/change/published`
  - `next schedule`
  - `Last change`
  - `Family / type`
  - `Family/type`
- Retry/backoff scan found remaining `exponential backoff` wording only in operator troubleshooting/logging docs, which matches the current scheduler's repeated doubling behavior; it also exposed the feed-detail UI's contradictory "linear backoff" label, recorded above as an application-review note.
- Internal-content scan over `docs/` returned no matches for `todo-history`, `docs/todo-history`, `SOW`, `.agents`, `pkg/`, `cmd/`, `pull request`, `submit`, `reviewers`, `developer documentation`, `implementation plan`, `TODO`, or `future work`. The only broad-pattern hits were YAML field examples for `source_contributor` and `forked`, which are catalog enum/status examples rather than contributor-workflow documentation.
- Filesystem validation passed: `docs/todo-history/` no longer exists and `.agents/sow/todo-history/` exists.
- Repository stale TODO-history path scan returned only the intentional `AGENTS.md` location note after excluding generated frontend assets and `node_modules`.
- Sensitive-name scan over changed durable artifacts found historical references in moved TODO-history files; the moved history and this SOW were sanitized to use `user` and `/home/user` before staging.
- Repeat post-wiki validation passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`
  - local source-doc and generated-wiki link validator, which reported `source docs and generated wiki links validate`
  - generated-wiki scan for `.md` links
  - focused stale status/rate-limit/systemd scan for nonexistent `/api/v1/status.version`, independent search limiter wording, wrong drop-in path, and old status response fields
- Repeat admin-doc validation after the admin UI repairs passed:
  - focused stale admin UI wording scan
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`
  - local source-doc and generated-wiki link validator, which reported `source docs and generated wiki links validate`
  - `git diff --check -- README.md docs .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md`
- Repeat 2026-06-01 validation after IPv6, migration-helper, and admin-auth repairs passed:
  - stale scan over `README.md` and `docs/` returned no matches for the old IPv6 blanket-unsupported wording, old `enable` / `disable` CLI wording, or wrong admin-auth missing-credential wording
  - migration-helper coverage scan confirmed `docs/migration-from-bash.md` now documents `BASH_BASE`, `BASH_LIB`, `BASH_CONFIG`, `BASH_WEB`, `import-bash-version`, `cache-merge`, pre-sync backup behavior, and restart behavior
  - internal-content scan over `docs/` and `README.md` returned no matches for `todo-history`, `docs/todo-history`, `SOW`, `.agents`, source-code path prefixes, contributor-review workflow wording, `TODO`, or `future work`
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - local source-doc and generated-wiki link validator, which reported `source docs and generated wiki links validate`
  - `git diff --check -- README.md docs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md`
  - sensitive-name scan over changed README/docs/SOW diff found no personal-name or workstation-path hits
- Repeat 2026-06-01 validation after API field, status, and Markdown-template docs repairs passed:
  - focused stale API/status scan over repaired API, pipeline, and troubleshooting docs returned no matches for stale `ip_version`, `source_timestamp`, `tracked`, old `ok` changed-content status, stale classification field lists, or nonexistent integrity-failed last-status wording
  - broad confirmation scan showed expected current docs hits for `downloaded`, `invalid_input`, and `templates/markdown/`; stale field/status hits remained only in this SOW as evidence of what was repaired
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - local source-doc and generated-wiki link validator, which reported `source docs and generated wiki links validate`
  - `git diff --check -- README.md docs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md scripts/build-wiki.mjs`
  - sensitive-name scan over changed README/docs/SOW/wiki-builder diff found no personal-name or workstation-path hits
- Repeat 2026-06-01 validation after admin operator API and config-update repairs passed:
  - registered-route coverage check found docs hits for all operator/API routes after expansion, excluding only intentional fallback 404 handlers (`POST /api/v1/admin/run/`, `POST /api/v1/admin/enable/`, `POST /api/v1/admin/disable/`) and the embedded static asset route `GET /world/`
  - internal-content scan over `docs/` and `README.md` returned no remaining workstation-checkout path, source-code path prefix, SOW, `.agents`, PR/reviewer workflow, TODO, or future-work hits; remaining broad-pattern hits were README's repository build/test overview and the documented `forked` catalog enum value
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - local source-doc and generated-wiki link validator, which reported `source docs and generated wiki links validate`
  - `git diff --check -- README.md docs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/SOW/wiki-builder diff found no personal-name or workstation-path hits
- Repeat legal-field validation after critical-infrastructure redistributability repair passed:
  - focused scan over `docs/` and `.agents/sow/specs/` found no remaining wording that critical-infrastructure feeds default to `redistributable: false`
  - `git diff --check -- docs/contributing/license-requirements.md docs/feeds/legal-fields.md docs/critical-infrastructure-reference-feeds.md .agents/sow/specs/config.md`
- Repeat route, flag, config, processor, environment, source-link, and wiki-link validation passed:
  - route coverage checked 58 registered routes; only intentional trailing-slash admin fallback 404 handlers were excluded from docs coverage
  - CLI flag coverage checked daemon 16, query 6, enable 5, and cache-merge 4 flags with no missing docs coverage
  - active YAML field coverage checked 107 YAML tags; the only unlisted tag was `args`, the internal representation for structured processor syntax
  - processor coverage found no missing docs entries; the later full registry audit below established the current registry count as 77
  - operator environment-variable coverage checked 44 runtime, installer, legacy-runtime, admin-auth, OpenTelemetry, and Go-runtime variables with no missing docs coverage
  - source docs and generated wiki links validated successfully
  - generated-wiki scans found no `.md` Markdown links, bare generated-page slugs, or relative `wiki/...` links
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - `git diff --check -- README.md docs .agents/sow/specs/config.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder diff found no personal-name or workstation-path hits
- Repeat 2026-06-01 validation after the follow-up GitHub Wiki sidebar fix passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - local source-doc and generated-wiki link validator, which reported `source docs and generated wiki links validate`
  - generated-wiki scans found no `.md` Markdown links, bare generated-page slugs, or relative `wiki/...` links
  - `git diff --check -- .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md`
  - added-line sensitive/path scan over the wiki-builder/workflow/SOW diff found no personal-name or workstation-path hits
- Repeat 2026-06-01 continuation validation passed:
  - live GitHub Wiki inspection confirmed the deployed wiki was still old and showed the exact prior failure mode; the current local builder output contains 88 flat wiki pages and full `https://github.com/firehol/update-ipsets/wiki/...` links.
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source-doc link validation reported `source docs links validate`
  - generated-wiki link validation reported `generated wiki links validate`
  - documented endpoint coverage checked 68 operator/API endpoint examples with zero unsupported documented endpoints
  - CLI flag coverage checked daemon 16, query 6, enable 5, and cache-merge 4 flags with no missing docs coverage
  - production environment-variable coverage checked 6 Go runtime variables after excluding tests/tools, with no missing docs coverage
  - YAML tag coverage over `pkg/config/config.go` checked 102 tags; the only unlisted tag was `args`, the internal representation for structured processor syntax
  - processor coverage found no missing docs entries; the later full registry audit below established the current registry count as 77
  - focused stale merge/listener/wiki scans found no remaining stale wording for all-parents-required merge behavior, optional split-listener `public_base_url`, or relative `wiki/...` generated links; the only nested `.md` wiki-path hit was the intentional workflow comment explaining the fixed failure mode
  - broad internal-content scan over `docs/` and `README.md` returned only benign hits: `not yet risky` as feed-health prose, `forked` as a catalog enum value, and `not yet materialized` as API payload state wording
  - stale field/status scan returned only current/benign hits such as README catalog history, Go 1.26 requirements, current monitoring counter text, and the public `provider_set_id` API field in infrastructure endpoints
  - `git diff --check -- README.md docs .agents/sow/specs/config.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs` passed
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name or workstation-path hits
- Repeat logging-format validation passed:
  - code evidence confirmed default local loggers use `slog.NewTextHandler`: `cmd/update-ipsets/common.go:8-16`, `pkg/web/server.go:66-76`, `pkg/scheduler/scheduler.go:55-58`, and `pkg/engine/engine.go:214-217`
  - stale JSON-log scan over `docs/` and `README.md` found no remaining local-log JSON examples or JSON grep examples; the only remaining JSON wording was about an integrity artifact file, not daemon logs
- Final targeted validation after the logging-format repair passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source-doc and generated-wiki link validation reported `source docs and generated wiki links validate`
  - generated wiki output contained 88 root-level Markdown pages, no nested files, no `.md` local wiki links, no bare generated-page slugs, and no relative `wiki/...` links
  - documented endpoint coverage checked 85 endpoint examples with zero unsupported documented endpoints
  - CLI flag coverage checked daemon 16, query 6, enable 5, cache-merge 4, and iprange 53 flags with no missing docs coverage
  - processor coverage found no missing docs entries; the later full registry audit below established the current registry count as 77
  - production environment-variable coverage checked 6 fixed Go runtime names with no missing docs coverage
  - YAML tag coverage checked 102 active tags; the only unlisted tag was `args`, the internal structured-processor representation
  - internal-content scan over `docs/` and `README.md` returned no matches for `todo-history`, old `docs/todo-history`, SOW/spec/internal path markers, contributor-review workflow wording, TODO/future-work language, or workstation/user-identifying paths
  - stale field/status/log scan returned only benign current hits: README catalog history, source-IP rate-limit wording, integrity JSON artifact wording, process I/O counter wording, and current `download.ok` / `download.status.*` telemetry names
  - `git diff --check -- README.md docs .agents/sow/specs/config.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name or workstation-path hits
- Repeat validation after the enablement/provider-set/quick-start repairs passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source-doc and generated-wiki link validation reported `source docs and generated wiki links validate`
  - generated wiki output contained 88 root-level Markdown pages, no nested files, no `.md` local wiki links, no bare generated-page slugs, and no relative `wiki/...` links
  - documented endpoint coverage checked 85 endpoint examples with zero unsupported documented endpoints
  - CLI flag coverage checked daemon 16, query 6, enable 5, cache-merge 4, and iprange 53 flags with no missing docs coverage
  - processor coverage found no missing docs entries; the later full registry audit below established the current registry count as 77
  - production environment-variable coverage checked 6 fixed Go runtime names with no missing docs coverage
  - YAML tag coverage checked 109 active config tags across config, artifact, category, and feed-health structs; the only unlisted tag was `args`, the internal structured-processor representation
  - download and processing status coverage passed against `pkg/cache/download_status.go` and `pkg/engine/processing_result.go`
  - internal-content scan over `docs/` and `README.md` returned no matches for `todo-history`, old `docs/todo-history`, SOW/spec/internal path markers, contributor-review workflow wording, TODO/future-work language, or workstation/user-identifying paths
  - stale field/status/log/enablement scan returned only benign current hits: README catalog history, source-IP rate-limit wording, current process I/O counter wording, and current telemetry field names
  - `git diff --check -- README.md docs .agents/sow/specs/config.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name or workstation-path hits
- Targeted API payload validation after the feed-endpoint repair passed:
  - history CSV docs now describe `DateTime` as Unix seconds, matching `pkg/engine/public_series.go:35-48`
  - comparison docs now list current `CompareRow` fields from `pkg/engine/engine.go:79-101`
  - retention docs now list current `RetentionData` and `RetentionSeries` fields from `pkg/engine/engine.go:104-117`
  - MCP ASN-prefix mismatch remains recorded as an application-review note rather than silently changing code or docs to claim unsupported behavior
- Targeted operator-command validation after the merge troubleshooting repair passed:
  - merge troubleshooting now uses only admin feed-detail fields present in `pkg/web/admin.go:188-191`
  - the stale `merge_health_excluded` field no longer appears in `docs/`
- Admin operator-action validation after the artifact-child wording repair passed:
  - docs now state that artifact-child recheck uses local child input when present and targets the parent artifact when no child input exists, matching `pkg/engine/download_stage.go:232-258`
- Repeat continuation validation after the API-payload and command-example repairs passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test /firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source-doc and generated-wiki link validation reported `source docs and generated wiki links validate`
  - fenced `update-ipsets` command examples use only supported subcommands
  - stale docs scan returned no matches for `merge_health_excluded`, old comparison fields, old retention wording, old ISO history example timestamps, `docs/todo-history`, internal SOW/spec/source path markers, contributor-review workflow wording, TODO/future-work language, or workstation/user-identifying paths
  - `git diff --check -- README.md docs .agents/sow/specs/config.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name or workstation-path hits
- Final continuation validation after the install/runtime path repairs passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test /firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - generated `_Sidebar.md` now uses full GitHub wiki links such as `https://github.com/firehol/update-ipsets/wiki/about-update-ipsets`
  - generated wiki output contained no nested Markdown files, no `.md` local wiki links, and no `wiki/wiki` relative-link artifacts
  - source-doc and generated-wiki link validation reported `source docs and generated wiki links validate`
  - fenced `update-ipsets` command examples use only supported subcommands
  - focused stale scan returned no matches for old merge/comparison/retention/todo-history/workstation-path/raw-wiki terms; the only `api/api-overview.md` hits were expected repository-source links in `docs/_Sidebar.md` and `docs/Home.md`, which the wiki builder rewrites for the published wiki
  - `git diff --check -- README.md docs .agents/sow/specs/config.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Final metadata/raw-file API validation after the glossary and `llms.txt` repairs passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test /firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source-doc and generated-wiki link validation reported `source docs and generated wiki links validate`
  - fenced `update-ipsets` command examples use only supported subcommands
  - focused stale scan returned no matches in the patched docs for old `/32` netset singleton wording or old `llms.txt` title/description examples
  - focused docs/README scan returned no matches for old `docs/todo-history`, `wiki/wiki`, or raw-markdown navigation wording
- Less-touched operator-doc validation after the provider/history/infrastructure/frequency/API-overview repairs passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test /firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source-doc and generated-wiki link validation reported `source docs and generated wiki links validate`
  - fenced `update-ipsets` command examples use only supported subcommands
  - focused stale scan returned no matches for old textual `daily`/`hourly` frequency examples, old provider-database public-visibility wording, old history-derivative naming-only wording, old critical-infrastructure matched-range claims, or old flattened `503` explanation
  - `git diff --check -- README.md docs .agents/sow/specs/config.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Follow-up health/status and wiki output validation passed:
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test /firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - generated `_Sidebar.md` uses full GitHub wiki links such as `https://github.com/firehol/update-ipsets/wiki/about-update-ipsets`
  - generated wiki output is flat at the wiki root and the generated-link scan returned no `.md` local wiki links
  - `docs/api/health-status.md` field claims were rechecked against `pkg/web/public_status.go`, `pkg/web/home_api.go`, and `pkg/engine/home_summary.go`
- Compose/raw-download eligibility validation passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test /firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source-doc and generated-wiki link validation reported `source docs and generated wiki links validate`
  - fenced `update-ipsets` command examples use only supported subcommands
  - focused docs/README stale scan returned no matches for old broad provider-exclusion wording
  - `git diff --check -- README.md docs .agents/sow/specs/config.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- OpenTelemetry endpoint validation passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test /firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source-doc and generated-wiki link validation reported `source docs and generated wiki links validate`
  - fenced `update-ipsets` command examples use only supported subcommands
  - focused docs/README/SOW scan returned no matches for endpoint-only `localhost:4317` HTTP examples or the old bare-host rejection wording
  - `git diff --check -- README.md docs .agents/sow/specs/config.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Cache-merge CLI doc validation passed:
  - `go test ./cmd/update-ipsets -run TestRunCacheMergePreservesLocalOnlyEntries -count=1`
  - command/help/code audit confirmed `docs/cli/cache-merge-command.md` flags match `cmd/update-ipsets/cache_merge.go:16-24`
  - migration-helper audit confirmed the documented helper uses `cache-merge` with `--legacy`, `--local-json`, `--local-only`, and `--out` at `scripts/sync-from-bash-version.sh:345-359`
- Final search/MCP/category/default-provider validation passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test /firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source-doc and generated-wiki link validation reported `source docs and generated wiki links validate`
  - generated wiki output contained no nested Markdown files, no local `.md` wiki links, and no `wiki/wiki` artifacts
  - fenced `update-ipsets` command examples use only supported subcommands
  - focused docs/README stale scan returned no matches for the old search, status, wiki, OpenTelemetry, provider-exclusion, merge-health, TODO-history, or raw-markdown navigation wording
  - `go test ./pkg/config -run 'TestValidateDefaultProviderContract|TestCatalogCategoryRegistry|TestCatalogHasProviderDefaults|TestCatalogArtifactChildren' -count=1`
  - `go test ./pkg/engine -run 'TestPublicCategoriesExcludeConfiguredNonPublicCategories|TestHomeSummaryExcludesConfiguredNonPublicCategories|TestProviderDefaultsOrderProviders|TestProviderDefaultsChangedForConfig' -count=1`
  - `git diff --check -- README.md docs .agents/sow/specs/config.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Final continuation validation after the pipeline publication-order and methodology slug repairs passed:
  - documented HTTP endpoint examples matched the registered route surface, including dynamic feed routes, admin feed manifests, methodology slugs, `AS`-prefixed ASN examples, and sharded sitemap files
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test /firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source Markdown and generated wiki link validation reported `source Markdown links and generated wiki links validate`
  - generated wiki output contained no nested Markdown files, no local `.md` wiki links, no old `docs/todo-history` links, and no `wiki/wiki` artifacts
  - fenced `update-ipsets` command examples use only supported subcommands, including `version`
  - internal-content scan over `README.md` and `docs/` returned no matches for old TODO-history paths, SOW/spec/internal path markers, contributor-review workflow wording, TODO/future-work language, or workstation/user-identifying paths
  - broad stale-term scan over `README.md`, `docs/`, and specs returned only current/benign hits such as explicit `enabled_by_all` compatibility notes, public `provider_set_id` API fields, current retry/backoff prose, and spec guardrails for removed routes
  - `go test ./pkg/web -run 'TestMethodology(IndexEndpointIsJSON|PageEndpointIsJSON|InsightSlugsPresent)|TestCountryAndASNAPIEndpointsServePrecomputedArtifacts|TestCriticalInfrastructureRouteServesOnlyPublishedArtifacts|TestRawFeedRoutesReturn404WhenMaterializedFileMissing' -count=1`
  - `go test ./pkg/engine -run 'TestPublicCategoriesExcludeConfiguredNonPublicCategories|TestHomeSummaryExcludesConfiguredNonPublicCategories|TestProviderDefaultsOrderProviders|TestProviderDefaultsChangedForConfig|TestPublicComposeRejectsNonRedistributableExclude|TestComposeRejectsTooManyIncludes|TestComposeRejectsTooManyExcludes|TestComposeRejectsUnsupportedFormat|TestComposeMergeBodyFailsWhenConfiguredExcludeIsMissing|TestComposeMergeBodyFailsWhenConfiguredExcludeIsDisabled|TestComposeMergeBodyFailsWhenConfiguredExcludeIsArchived|TestComposeMergeBodyFailsWhenConfiguredExcludeIsUnmaintained' -count=1`
  - `go test ./pkg/config -run 'TestValidateDefaultProviderContract|TestCatalogCategoryRegistry|TestCatalogHasProviderDefaults|TestCatalogArtifactChildren|TestCatalogProcessorRawPresent|TestValidateRejectsLegacyInfrastructureASNs|TestValidateCriticalASNContextContract' -count=1`
  - `go test ./cmd/update-ipsets -run 'TestRunCacheMergePreservesLocalOnlyEntries' -count=1`
  - `git diff --check -- README.md docs .agents/sow/specs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Repeat continuation validation after legacy env-var, downloader-options, and home-summary limit repairs passed:
  - combined coverage scan reported YAML and environment coverage plus no processor-doc gaps; the later full registry audit below established the current registry count as 77
  - documented HTTP endpoint examples matched the registered route surface
  - fenced `update-ipsets` command examples use only supported subcommands
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test /firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source Markdown and generated wiki link validation reported `source Markdown links and generated wiki links validate`
  - generated wiki output contained no nested Markdown files, no local `.md` wiki links, no old `docs/todo-history` links, and no `wiki/wiki` artifacts
  - focused repair scan confirmed the repaired docs contain the legacy migration variables, downloader option forms, and home-summary limit/default wording
  - internal-content scan over `README.md` and `docs/` returned no matches for old TODO-history paths, SOW/spec/internal path markers, contributor-review workflow wording, TODO/future-work language, or workstation/user-identifying paths
  - `git diff --check -- README.md docs .agents/sow/specs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
  - `go test ./pkg/downloader -run 'TestFetchDownloaderOptionsPOSTAndHeaders|TestFetchDownloaderOptionsExplicitContentTypeWins|TestFetchDownloaderOptionsMethodAndBasicAuth|TestSplitShellWordsEdgeCases' -count=1`
  - `go test ./pkg/web -run 'TestMethodology(IndexEndpointIsJSON|PageEndpointIsJSON|InsightSlugsPresent)|TestCountryAndASNAPIEndpointsServePrecomputedArtifacts|TestCriticalInfrastructureRouteServesOnlyPublishedArtifacts|TestRawFeedRoutesReturn404WhenMaterializedFileMissing' -count=1`
  - `go test ./pkg/engine -run 'TestHomeSummaryReadsPrecomputedAggregateWithoutPerFeedArtifacts|TestHomeSummaryMissingAggregateReturnsNotReady|TestPublicCategoriesExcludeConfiguredNonPublicCategories|TestHomeSummaryExcludesConfiguredNonPublicCategories|TestPublicComposeRejectsNonRedistributableExclude|TestComposeRejectsTooManyIncludes|TestComposeRejectsTooManyExcludes|TestComposeRejectsUnsupportedFormat' -count=1`
  - `go test ./pkg/config -run 'TestValidateDefaultProviderContract|TestCatalogCategoryRegistry|TestCatalogHasProviderDefaults|TestCatalogArtifactChildren|TestCatalogProcessorRawPresent|TestValidateRejectsLegacyInfrastructureASNs|TestValidateCriticalASNContextContract' -count=1`
  - `go test ./cmd/update-ipsets -run 'TestRunCacheMergePreservesLocalOnlyEntries' -count=1`
- Repeat continuation validation after the reverse-proxy and bogon-role wording repairs passed:
  - `scripts/inventory.sh` was still absent, so direct inventory/audit fallback was used
  - documented HTTP endpoint examples matched the registered route surface
  - fenced `update-ipsets` command examples use only supported subcommands
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test /firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source Markdown and generated wiki link validation reported `source Markdown links and generated wiki links validate`
  - generated wiki output contained no nested Markdown files, no local `.md` wiki links, no old `docs/todo-history` links, and no `wiki/wiki` artifacts
  - internal-content and stale-wording scan over `README.md` and `docs/` returned only benign operator-doc hits such as current Go 1.26 requirements, normal docs navigation links, public `context` wording, and catalog enum examples; it returned no stale `admin actions send JSON` or absolute `Bogon feeds appear in the public catalog and...` wording
  - combined coverage scan again reported YAML and environment coverage plus no processor-doc gaps; the later full registry audit below established the current registry count as 77
  - `git diff --check -- README.md docs .agents/sow/specs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Repeat continuation validation after MCP exact-route and feed-field API repairs passed:
  - navigation coverage checked 88 docs files and 87 sidebar links with no orphaned operator pages outside the expected wiki sidebar file
  - registered route coverage checked 54 registered route patterns; only method-not-allowed guards, embedded assets, and public SPA shell routes were intentionally excluded
  - feed API summary/detail field coverage confirmed every JSON field from `PublicFeedSummary` and the single-feed metadata schema is mentioned in `docs/api/feed-endpoints.md`
  - documented HTTP endpoint examples matched the registered route surface
  - fenced `update-ipsets` command examples use only supported subcommands
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test /firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source Markdown and generated wiki link validation reported `source Markdown links and generated wiki links validate`
  - generated wiki output contained no nested Markdown files, no local `.md` wiki links, no old `docs/todo-history` links, and no `wiki/wiki` artifacts
  - combined coverage scan reported YAML and environment coverage plus no processor-doc gaps; the later full registry audit below established the current registry count as 77
  - internal-content and stale-wording scan over `README.md` and `docs/` returned only benign current hits: README catalog counts, source-IP rate-limit wording, and category-list prose
  - `git diff --check -- README.md docs .agents/sow/specs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs`
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Repeat wiki navigation validation after the live GitHub sidebar failure passed:
  - live published wiki HTML showed custom sidebar links rendered as relative links such as `href="about-update-ipsets"` or `href="wiki/about-update-ipsets"` instead of full GitHub wiki URLs
  - GitHub's wiki documentation says Markdown wiki links use the full wiki page URL, so `scripts/build-wiki.mjs` now emits full `https://github.com/firehol/update-ipsets/wiki/<page>` links for generated wiki pages
  - `.github/workflows/wiki-sync.yml` now passes the full GitHub wiki base URL to the builder
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - generated `_Sidebar.md` and `Home.md` now use full GitHub wiki links such as `https://github.com/firehol/update-ipsets/wiki/about-update-ipsets`
  - explicit root-path compatibility check with `/firehol/update-ipsets/wiki` also emits full GitHub wiki links
  - generated wiki link parser checked 88 pages: all Markdown links are absolute URLs or anchors, and every generated GitHub wiki target resolves to a generated page
  - generated wiki scans returned no local `.md` links, no old root-absolute wiki-path links, no `docs/todo-history` links, and no `wiki/wiki` artifacts
  - final `git diff --check -- README.md docs .agents/sow/specs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs` passed
  - final added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Repeat unchanged-docs and pipeline-publication audit passed:
  - rechecked the six previously unchanged operator docs against current route, UI, scheduler, processing, and integrity code
  - `docs/pipeline/pipeline-overview.md` and `docs/pipeline/processing-lifecycle.md` now match the current feed-body finalization and public artifact publication order
  - `docs/admin-ui/background-work.md` now matches current background task names, trigger classes, pending/coalesced work, worker counts, and entity-artifact repair boundaries
  - `.agents/sow/specs/{downloader,files-layout,pipeline,processing-engine}.md` no longer contain stale `.processing` promotion or publication-as-finalize wording
  - stale scan over `docs` and `.agents/sow/specs` returned no remaining matches for `.processing` promotion, `.processing` commit, old background own-panel wording, old startup-repairs wording, or old config-reload rebuild wording
- Repeat catalog-maintenance and processor-field audit passed:
  - `docs/contributing/*.md` content is operator catalog-maintenance guidance; the directory name remains a known naming mismatch because moving docs requires explicit approval
  - command-doc audit confirmed daemon, query, enable, and cache-merge docs match current flag sets; top-level `help` still omits `cache-merge` and remains a recorded application-review note
  - `docs/feeds/processors.md`, `docs/feeds/source-feeds.md`, and `docs/feeds/yaml-field-reference.md` now describe `processor_raw` as a legacy single processor-name fallback/metadata field, not a separate raw-archive pipeline
  - `.agents/sow/specs/config.md` now records the same current `processor` / `processor_raw` precedence contract from `pkg/engine/helpers.go`
  - current `BackgroundTaskHandle.Update` was rechecked and no longer supports the old suspected repeated-lock issue
  - refined stale-content scan returned no matches for old `processor_raw` raw-archive wording, old `docs/todo-history` paths, internal SOW/spec/source-code markers, contributor-review workflow wording, TODO/future-work language, or workstation/user-identifying paths
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - generated wiki link parser checked 88 pages and found no local `.md` links, no `docs/todo-history` links, no `wiki/wiki` links, and no repository blob/tree links
  - `git diff --check -- README.md docs .agents/sow/specs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs` passed
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Repeat deployment/security/listener audit passed:
  - `docs/security/production-deployment.md` and `docs/installation/tls-configuration.md` now state that generated public URLs come from `runtime.public_base_url` and `runtime.web_url`, not from request `Host` or `X-Forwarded-Proto` headers
  - CORS and rate-limit docs were rechecked against current middleware: `/api/*` and `/mcp` share the general limiter, search uses the stricter search limiter too, admin paths emit no wildcard CORS headers, and admin `OPTIONS` may return `204`
  - installed systemd environment/path docs were rechecked against `install.sh`, `pkg/engine/envfile.go`, `pkg/engine/runtime.go`, and raw file serving/copy code; no additional patch was required
  - exact stale-phrase scan returned no matches for the old "Host header makes the daemon generate correct URLs" guidance
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - generated wiki link parser checked 88 pages and found no local `.md` links, no `docs/todo-history` links, no `wiki/wiki` links, and no repository blob/tree links
  - generated wiki stale-link scan returned no matches for old `docs/todo-history`, `wiki/wiki`, or local Markdown link targets
  - `git diff --check -- README.md docs .agents/sow/specs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs` passed
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Repeat API/admin route-method audit passed:
  - public route registration was rechecked at `pkg/web/routes.go:26-58`, feed-scoped route actions at `pkg/web/routes.go:89-170`, admin route registration at `pkg/web/routes.go:267-290`, public artifact/methodology/raw routes at `pkg/web/routes.go:365-386`, and route-method regression tests at `pkg/web/routes_test.go:52-199`
  - docs for public methods, admin action methods, CORS, split-listener behavior, and authenticated operator API tables already matched current behavior: public `GET` routes also allow `HEAD`, admin read routes also allow `HEAD`, admin action routes require `POST` with `Allow: POST`, `/mcp` accepts `POST`, `GET`, and `DELETE`, and admin CORS preflight may return `204` without wildcard headers
  - no additional docs patch was required for this slice
  - `go test ./pkg/web -run 'TestSurfaceHandlerModesRegisterExpectedSurfaces|TestRouteMethodContracts|TestMCPAndAdminCORSContracts|TestAdminReadRoutesAllowHEAD|TestAdminActionRoutesRejectHEAD' -count=1` passed
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - generated wiki link parser checked 88 pages and found no local Markdown links or missing GitHub wiki targets
  - generated wiki stale-link scan returned no matches for old `docs/todo-history`, `wiki/wiki`, or local Markdown link targets
  - `git diff --check -- README.md docs .agents/sow/specs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .github/workflows/wiki-sync.yml scripts/build-wiki.mjs` passed
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Repeat broad operator-doc hygiene scan found no new required patch:
  - docs count remained 88 Markdown pages under `docs/`
  - docs/README stale-link scan returned expected repository-source `.md` links only; generated wiki validation covers the published wiki form
  - internal-content scan over `README.md` and `docs/` returned no old `docs/todo-history` files, SOW/spec/source path leakage, contributor-review workflow wording, TODO/future-work language, or workstation/user-identifying paths
  - current `enabled_by_all` hits in docs all describe the accepted compatibility metadata and current `--enable-all` global override behavior
- Repeat monitoring/telemetry audit found and repaired a small operator-doc gap:
  - `docs/monitoring/telemetry-reference.md` now lists the scheduler operation names exposed under admin status `metrics.operations`: `scheduler.fetch_and_stage`, `scheduler.promote_committed_downloads`, `scheduler.run_once`, and `scheduler.processing_batch_total`
  - entity artifact HTTP metrics now list exact hit and miss metric names instead of shorthand combined rows
  - metric coverage script extracted 147 metric-like names or admin operation names from current `pkg/`, `internal/`, and `cmd/` Go code and found them covered by the monitoring docs, excluding the non-metric `feed.name` attribute key
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - generated wiki link parser checked 88 pages and found no local Markdown links or missing GitHub wiki targets
  - `git diff --check -- docs/monitoring/telemetry-reference.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md scripts/build-wiki.mjs` passed
  - added-line sensitive/path scan over changed telemetry reference and SOW diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Repeat broad validation after the monitoring patch passed:
  - source Markdown link validator checked all 88 docs pages
  - generated wiki link validator checked all 88 generated wiki pages
  - documented `update-ipsets` shell examples and inline command spans use supported subcommands
  - internal-content scan over `README.md` and `docs/` returned no matches for old `docs/todo-history`, SOW/spec/internal path markers, TODO/future-work language, or workstation/user-identifying paths
- Repeat install/update template-backup validation passed:
  - focused install/update scan over `docs/installation/`, `docs/updating/`, `docs/running/`, and `install.sh` showed current hits for `templates/markdown/`, `config.bak`, `--no-restart`, and the custom-install warning, with no stale claim that template edits receive a template-specific backup
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source Markdown link validator checked all 88 docs pages
  - generated wiki link validator checked all 88 generated wiki pages and found no local/non-absolute wiki links or missing GitHub wiki targets
  - generated wiki stale-link scan returned no matches for old `docs/todo-history`, `wiki/wiki`, local Markdown wiki links, or repository blob/tree links
  - `git diff --check -- docs/installation docs/updating .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md scripts/build-wiki.mjs` passed
  - added-line sensitive/path scan over changed install/update docs, SOW, and wiki builder diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Migration-helper no-patch audit passed:
  - `docs/migration-from-bash.md` was rechecked against `scripts/sync-from-bash-version.sh` for command signature, default install directory, legacy path override variables, staged import directory, pre-sync backup, cache merge, managed env keys, summary output, rollback output, and restart-if-previously-running behavior
  - `bash -n scripts/sync-from-bash-version.sh`
  - `bash scripts/sync-from-bash-version.sh --help`
  - coverage scan over the migration doc and helper showed matching current hits for `sync-from-bash-version`, `BASH_BASE`, `BASH_LIB`, `BASH_CONFIG`, `BASH_WEB`, `cache-merge`, `.cache`, `import-bash-version`, pre-sync backup, managed API-key variables, and restart behavior
- Catalog-operator and processor-reference no-patch audit passed:
  - `docs/contributing/*.md` is now catalog-operator guidance, not fork/PR contributor workflow; the directory name remains unchanged because moving docs requires explicit user approval
  - category guidance in `docs/contributing/step-by-step-add-feed.md` was rechecked against `configs/firehol/categories.yaml`; the public operator categories match the current catalog keys
  - source and merge examples in `docs/contributing/contribution-guide.md` and `docs/contributing/step-by-step-add-feed.md` were rechecked against current YAML fields such as `frequency`, `ipv`, `output`, `category`, `maintainer_url`, `license`, `redistributable`, `attribution`, and `processor`
  - `docs/feeds/processors.md` documents all 77 registered processor names found in `pkg/processor/processor.go`, `pkg/processor/primitives.go`, `pkg/processor/legacy_processors.go`, and `pkg/processor/regex.go`
  - every processor name currently used by `configs/firehol/sources/**/*.yaml` is covered by `docs/feeds/processors.md`
- MCP endpoint audit found and repaired two small contract/doc precision issues:
  - `docs/api/mcp-endpoint.md` was rechecked against `pkg/web/routes.go`, `pkg/web/http.go`, `pkg/web/middleware.go`, `pkg/mcp/server.go`, `pkg/mcp/find_feeds.go`, `pkg/mcp/fetch_analysis.go`, and `pkg/mcp/server_test.go`
  - documented methods `POST /mcp`, `GET /mcp`, and `DELETE /mcp` match route registration
  - documented CORS headers match `pkg/web/http.go` and are covered by `pkg/web/routes_test.go`
  - documented tools match the two registered tools, `find_feeds` and `fetch_analysis`
  - `docs/api/mcp-endpoint.md` and `.agents/sow/specs/website.md` now state that `find_feeds.maintainer` is a case-insensitive substring filter, matching `pkg/mcp/find_feeds.go` and `TestHandleFindFeedsFilterByMaintainer`
  - `docs/api/mcp-endpoint.md` and `.agents/sow/specs/website.md` now state that MCP `fetch_analysis` ASN names are numeric artifact identifiers without the `AS` prefix, matching `pkg/mcp/fetch_analysis.go` and `TestHandleFetchAnalysisEntityLayout`
  - documented `fetch_analysis` entity layouts and path traversal behavior match `NewFileMarkdownStore`
  - the MCP tool-description claim that `AS`-prefixed ASN names are accepted remains mapped to `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md`
  - `go test ./pkg/mcp` passed
- Public/API route no-patch audit passed:
  - documented public and admin route examples were rechecked against `pkg/web/routes.go`, `pkg/web/home_api.go`, `pkg/web/home_detail_api.go`, `pkg/web/public_status.go`, and current route-method tests
  - `docs/api/health-status.md` matches current `/healthz`, `/api/v1/status`, `/api/v1/client-ip`, `/api/v1/categories`, `/api/v1/home/globe`, and `/api/v1/home/summary` behavior
  - `docs/api/classification-endpoints.md` matches current country, ASN, and maintainer handlers, including REST ASN `AS` prefix normalization
  - `go test ./pkg/web -run 'TestSurfaceHandlerModesRegisterExpectedSurfaces|TestRouteMethodContracts|TestMCPAndAdminCORSContracts|TestAdminReadRoutesAllowHEAD|TestAdminActionRoutesRejectHEAD|TestCountryAndASNAPIEndpointsServePrecomputedArtifacts' -count=1` passed
- Admin UI route/status/queue audit found and repaired two small operator-doc drifts:
  - `docs/admin-ui/runtime-status.md` now includes the `scheduler` block exposed by `buildAdminStatus`, matching `pkg/web/admin.go:560-563`
  - `docs/admin-ui/live-queues.md` no longer says only public feeds appear in processing panels; `pkg/scheduler/scheduler.go:175-204` filters processing activity only to exclude provider databases, and `pkg/engine/engine_test.go:573-645` proves hidden feeds are valid reprocess targets
  - `docs/admin-ui/enable-disable.md` now distinguishes automatic scheduler disablement from explicit manual feed actions; `pkg/web/admin.go:335-360` always queues feed recheck/reprocess actions, `pkg/scheduler/actions.go:18-62` marks them as forced/immediate selected work, and `pkg/engine/helpers.go:50-57` allows selected/manual work to override the root feed marker while preserving parent constraints from `pkg/engine/enabled_state.go:23-49`
- Repeat broad validation after the no-patch audit additions passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source Markdown link validator checked all 88 docs pages
  - generated wiki link validator checked all 88 generated wiki pages and found no local/non-absolute wiki links or missing GitHub wiki targets
  - generated wiki stale-link scan returned no matches for old `docs/todo-history`, `wiki/wiki`, local Markdown wiki links, or repository blob/tree links
  - internal-content scan over `README.md` and `docs/` returned only benign operator terms for deferred work and no old TODO-history, SOW/spec/source-code path, TODO/future-work, or workstation/user-identifying path leakage
  - `git diff --check -- README.md docs .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/specs` passed
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Focused validation after the MCP precision repair passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test https://github.com/firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - generated wiki scans returned no local `.md` links, raw-wiki redirects, nested docs-section links, or `wiki/wiki` paths
  - `git diff --check -- docs/api/mcp-endpoint.md .agents/sow/specs/website.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md`
  - `go test ./pkg/mcp -count=1`
  - `bash .agents/sow/audit.sh`
  - sensitive/path scan over the changed MCP doc, website spec, and SOW returned no personal-name, workstation-path, or obvious secret-pattern hits
- Focused validation after the late admin UI queue/status/enablement audit passed:
  - `node --check scripts/build-wiki.mjs`
  - `git diff --check -- docs/admin-ui/runtime-status.md docs/admin-ui/live-queues.md docs/admin-ui/enable-disable.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md`
  - `go test ./pkg/engine -run 'TestFullFeedReprocessTargetsIncludeHiddenFeeds|TestResolveRecheckTargetFallsBackToParentWhenHistoryRollupsMissing' -count=1`
  - `go test ./pkg/scheduler -run 'TestManualRecheckQueuesDownloadWork|TestManualRecheckArtifactChildWithoutLocalInputQueuesParentArtifact|TestManualRecheckArtifactChildWithLocalInputQueuesChild|TestManualReprocessWithoutLocalStateDoesNotQueue|TestProviderDefaultsReprocessQueuesFullFeedTargets|TestBuildSnapshotDisablesArtifactChildWhenParentDisabled|TestBuildArtifactItemsIncludesEnabledArtifact' -count=1`
  - `go test ./pkg/web -run 'TestAdminAuthAndActions|TestAdminReadRoutesAllowHEAD|TestAdminActionRoutesRejectHEAD' -count=1`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test https://github.com/firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-test`
  - source Markdown link validator checked 88 docs pages
  - generated wiki link validator checked 88 pages and found no local or relative wiki links
  - generated wiki stale-link scan returned no matches for old `docs/todo-history`, `wiki/wiki`, or generated local Markdown wiki links
  - internal-content scan over `README.md` and `docs/` returned no matches for old TODO-history, SOW path, TODO-file, or workstation/user-identifying path leakage
  - `git diff --check -- README.md docs .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/specs` passed
  - added-line sensitive/path scan over changed README/docs/spec/SOW/wiki-builder/workflow diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Focused validation after the late CLI example repair passed:
  - `rg -n -- "update-ipsets enable [^\n]*(firehol_[^\n]* --| [a-zA-Z0-9_-]+ --disable| [a-zA-Z0-9_-]+ --all| [a-zA-Z0-9_-]+ --config)" docs README.md` returned no matches
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-final https://github.com/firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-final`
  - source Markdown link validator checked 88 docs pages and 202 local links
  - generated wiki link validator checked 88 pages and 202 wiki links
  - generated wiki stale-link scan returned no matches for old `docs/todo-history`, `wiki/wiki`, local Markdown wiki links, or repository blob/tree links
  - internal-content scan over `README.md` and `docs/` returned no matches for old TODO-history, SOW path, TODO-file, workstation/user-identifying path leakage, TODO/future placeholders, or old developer workflow wording
  - `git diff --check -- README.md docs .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/specs` passed
  - added-line sensitive/path scan over changed CLI enable docs and SOW diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Focused validation after the cleanup-registry documentation repair passed:
  - config YAML field coverage script found every YAML-tagged field from `Config`, `DefaultProviders`, `Source`, `Merge`, `CriticalMetadata`, `CriticalASNContext`, `Artifact`, `CategoryDefinition`, and `FeedHealthCategoryThresholds` covered in the scoped operator docs
  - stale cleanup-alias wording scan for `Backward-compatible name aliases`, `Old-name.*aliases`, and `rename aliases` returned no matches in docs or `.agents/sow/specs/config.md`
  - `go test ./pkg/config -run 'TestLoadFireHOLCatalog|TestMergeNilOther|TestMerge' -count=1` passed
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-after-cleanup-registry https://github.com/firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-after-cleanup-registry`
  - source Markdown link validator checked 88 docs pages and 202 local links
  - generated wiki link validator checked 88 pages and 202 wiki links
  - generated wiki stale-link scan returned no matches for old `docs/todo-history`, `wiki/wiki`, local Markdown wiki links, or repository blob/tree links
  - `git diff --check -- docs/configuration/configuration-concepts.md docs/feeds/yaml-field-reference.md .agents/sow/specs/config.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md` passed
  - `git diff --check -- README.md docs .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/specs` passed
  - added-line sensitive/path scan over changed cleanup-registry docs, config spec, and SOW diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Focused validation after the OpenTelemetry metric-interval documentation repair passed:
  - environment variable coverage script found all 43 runtime/install/admin/systemd/OpenTelemetry variables from current code and install template covered by scoped operator docs
  - stale metric-interval wording scan for milliseconds-only guidance returned no matches in docs or specs
  - `go test ./internal/observability -run 'Test.*Metric|Test.*Protocol|Test.*Signal|Test.*Enabled' -count=1` passed
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-after-otel-interval https://github.com/firehol/update-ipsets/wiki`, which reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-after-otel-interval`
  - source Markdown link validator checked 88 docs pages and 202 local links
  - generated wiki link validator checked 88 pages and 202 wiki links
  - generated wiki stale-link scan returned no matches for old `docs/todo-history`, `wiki/wiki`, local Markdown wiki links, or repository blob/tree links
  - `git diff --check -- docs/running/environment-variables.md docs/monitoring/opentelemetry-setup.md .agents/sow/specs/operating-principles.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md` passed
  - added-line sensitive/path scan over changed OpenTelemetry docs, operating-principles spec, and SOW diff found no personal-name, workstation-path, or obvious secret-pattern hits
- Focused validation after the processor-reference repeat audit passed:
  - processor registry/catalog/docs comparison reported 77 registered names, 77 documented names, 34 catalog/raw names, and 34 `processor_raw` names, with no missing or extra names
  - `go test ./pkg/processor -run 'TestFireholCatalogProcessorsAreRegistered|TestFireholCatalogProcessorRawNamesAreRegistered|TestFireholCatalogProcessorRawAndProcessorConsistent' -count=1` passed
  - stale intermediate processor-count scan returned no remaining `75` / `76` registered-processor count wording in this SOW
  - `git diff --check -- docs/feeds/processors.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md` passed
- Focused validation after the repeat HTTP route documentation audit passed:
  - documented endpoint checker found 97 endpoint examples in `README.md` and `docs/` and matched them to current public, admin, feed-scoped, methodology, direct raw-file, and direct artifact route families
  - registered route coverage checker found docs hits for 47 required route patterns after excluding intentional method-not-allowed guards, fallback 404 handlers, embedded assets, and SPA shell routes
  - `go test ./pkg/web -run 'TestAdminAuthAndActions|TestAdminReadRoutesAllowHEAD|TestAdminActionRoutesRejectHEAD|TestPublicRoutesAllowHEAD|TestCORSPreflightBypassesMethodSpecificRoutes|TestMethodology(IndexEndpointIsJSON|PageEndpointIsJSON)' -count=1` passed
- Repeat generated-wiki validation after the route no-patch audit passed:
  - `node --check scripts/build-wiki.mjs` passed
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-route-final https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-route-final`
  - generated wiki output contained 88 root-level pages and no local links, `.md` wiki targets, duplicated `wiki/wiki` paths, nested Markdown pages, or old `docs/todo-history` references
- Focused validation after the admin-access and integrity-recovery documentation repair passed:
  - `go test ./pkg/web -run 'TestAdminAuthAndActions|TestAdminFailsClosedWithoutConfiguredCredentials|TestAdminAuthDisabledAllowsUnauthenticatedAccess|TestRunServesSplitAdminOnSeparateListeners|TestRunRejectsSplitAdminWithoutPublicBaseURL|TestRunRejectsDisabledAdminWithoutAcknowledgement|TestBuildIntegrityReportAnnotatesRecoveryMetadata|TestHandleAdminIntegrityReprocessReturnsSplitTargets' -count=1` passed
  - `go test ./pkg/engine -run 'TestIntegrityRecoveryPlanRechecksDownloadableFeedWhenCommittedSourceMissing|TestIntegrityRecoveryPlanRebuildsDownloadableFeedWhenCommittedSourceExists|TestIntegrityRecoveryPlanRebuildsHistoryDerivativeWhenLocalStateExists|TestIntegrityRecoveryPlanRechecksArtifactParentWhenChildSourceMissing|TestIntegrityRecoveryPlanRechecksMissingMergeBogonProvider' -count=1` passed
  - `node --check scripts/build-wiki.mjs` passed
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-admin-recovery https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-admin-recovery`
  - source Markdown link validator checked 88 docs pages and 202 local links
  - generated wiki link validator checked 88 pages and 211 links
  - `git diff --check -- docs/admin-ui/accessing-admin.md docs/integrity/recovery-model.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md` passed
- Focused validation after the TLS/proxy/security no-patch audit passed:
  - `go test ./pkg/web -run 'TestRunServesHTTPS|TestClientIPResolver|TestClientIPEndpoint|TestRunServesSplitAdminOnSeparateListeners|TestRunRejectsSplitAdminWithoutPublicBaseURL' -count=1` passed
  - TLS/proxy docs coverage scan found current docs hits for `--tls-cert`, `--tls-key`, proxy/cloudflare trust flags, runtime trust fields, `runtime.public_base_url`, and the relevant forwarded headers
  - `git diff --check -- docs/installation/tls-configuration.md docs/security/production-deployment.md docs/security/security-overview.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md` passed
- Focused validation after the memory/resource documentation repair passed:
  - code audit checked file-backed serving/comparison evidence in `pkg/iprange/fileset.go`, active processing memory evidence in `pkg/iprange/set.go`, `pkg/downloader/canonical.go`, and `pkg/engine/process.go`, downloader streaming evidence in `pkg/downloader/downloader.go`, processor segmentation evidence in `pkg/processor/run_stream.go`, and installed `MemoryMax=2G` evidence in `install.sh`
  - `go test ./pkg/downloader -run 'TestFetchLocalCopyMaxDownloadSize|TestFetchLocalCopyDisabledLimit|TestMaxDownloadSize' -count=1` passed
  - `go test ./pkg/processor -run 'TestStreamGunzipEquivalence|TestStreamFallbackForNonStreamable|TestStreamMixedPipeline|TestStreamUnzipFallback|TestClassifyPipeline|TestIsStreamable|TestGunzipBombProtection|TestGunzipSizeConstantExists' -count=1` passed
  - `go test ./pkg/iprange -run 'TestFileSetMemoryBounded|TestLargeFileSetBounded|TestIterOpsWithFileSet|TestIterOpsWithFileSetLarge|TestFileSetRoundTrip|TestFileSetContains' -count=1` passed
  - `node --check scripts/build-wiki.mjs` passed
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-memory https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-memory`
  - source Markdown link validator checked 88 docs pages and 202 local docs links
  - generated wiki link validator checked 88 pages and 202 wiki links
  - `git diff --check -- docs/installation/memory-planning.md .agents/sow/specs/memory-management.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md` passed
- Focused validation after the processor `args` YAML documentation repair passed:
  - code evidence checked `pkg/config/config.go` `ProcessorStep` YAML handling, including scalar step names and single-key maps whose value becomes the step `args` map
  - `docs/feeds/processors.md`, `docs/feeds/yaml-field-reference.md`, and `.agents/sow/specs/config.md` now explicitly describe the processor step `args` map for argument-bearing processors such as `grep`, `csv_column`, `json_path`, and `regex`
- Completion audit on 2026-06-01 passed:
  - `node --check scripts/build-wiki.mjs` passed
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-completion https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-completion`
  - comprehensive docs/spec coverage validator checked 88 docs pages, 202 source links, 88 generated wiki pages, 202 generated wiki links, 77 accepted processor names, 109 config YAML fields, 25 CLI flags, 159 endpoint mentions, and 1 explicitly negative removed-route mention
  - endpoint route-family checker matched positive endpoint mentions to current public, admin, feed-scoped, methodology, MCP, raw-file, and direct-artifact route families
  - `go test ./pkg/web -count=1` passed
  - `git diff --check -- .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/specs README.md docs scripts/build-wiki.mjs .github/workflows/wiki-sync.yml` passed before and after this SOW evidence update
- Continuation audit after concrete follow-up mapping passed:
  - created `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md` to track the eight application-review items recorded by this SOW
  - `bash .agents/sow/audit.sh` passed and reported clean SOW initialization/status consistency with 9 pending SOWs, 1 current SOW, and 87 done SOWs
  - `node --check scripts/build-wiki.mjs` passed
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-continuation https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-continuation`
  - repeat comprehensive docs/spec coverage validator checked 88 docs pages, 202 source links, 88 generated wiki pages, 202 generated wiki links, 77 accepted processor names, 109 config YAML fields, 25 CLI flags, 159 endpoint mentions, and 1 explicitly negative removed-route mention
  - `git diff --check -- .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md .agents/sow/specs README.md docs scripts/build-wiki.mjs .github/workflows/wiki-sync.yml` passed
- Closure-readiness validation after the final SOW sub-state update passed:
  - `git diff --check -- .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md .agents/sow/specs README.md docs scripts/build-wiki.mjs .github/workflows/wiki-sync.yml` passed
  - `bash .agents/sow/audit.sh` passed and reported clean SOW initialization/status consistency with 9 pending SOWs, 1 current SOW, and 87 done SOWs
  - `node --check scripts/build-wiki.mjs` passed
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-closure https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-closure`
  - generated wiki link validation checked 88 source docs pages, 202 source docs links, 88 root-level wiki pages, 202 generated GitHub wiki links, and 211 generated links total; it found no generated relative wiki links, `.md` wiki targets, nested wiki page paths, or `docs/` wiki targets
- Goal-continuation completion audit on 2026-06-01 passed:
  - `node --check scripts/build-wiki.mjs && rm -rf /tmp/update-ipsets-wiki-goal-audit && node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-goal-audit https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-goal-audit`
  - source and generated wiki link validator checked 88 source docs pages, 202 source docs links, 88 generated wiki pages, 202 generated GitHub wiki links, and 211 generated links total; it found no missing source links, generated relative wiki links, `.md` wiki targets, nested wiki page paths, or `docs/` wiki targets
  - endpoint mention checker matched 165 operator-doc endpoint mentions, 151 unique file/path pairs, to the current public, admin, MCP, raw-file, and static route families
  - config YAML field coverage checker found the 102 current YAML fields from `pkg/config/config.go` represented in operator docs, with the loader-populated `derived_from` field treated as internal-only
  - CLI/install/downloader flag checker found 60 documented `--flag` mentions represented in current command, installer, iprange, or curl-like downloader-option surfaces
  - processor reference checker found 72 accepted processor names represented in `docs/feeds/processors.md` / `docs/feeds/yaml-field-reference.md`
  - `make test` passed and ran `go test ./...`
- Follow-up code-to-doc coverage audit on 2026-06-01 passed:
  - route-to-doc coverage checker found 49 must-document public/API/admin route registrations represented in operator docs, while ignoring 3 explicit admin not-found compatibility traps and UI/static asset routes
  - command-to-doc coverage checker found all 5 top-level commands and 31 command flags represented in operator docs
  - sensitive-data scan over README, docs, this SOW, and SOW-0095 found only placeholder credential examples such as `your-secret-password`; no raw personal name, workstation path, API key, bearer token, or real secret pattern was found
  - internal-content scan over README and docs found no implementation path markers, SOW/spec markers, TODO/FIXME language, or developer workflow leakage after excluding normal Markdown links and API/client examples
  - stale wiki/raw-Markdown scan over README and docs found no old `docs/todo-history`, nested `.md` wiki URL, raw-Markdown failure, or `api/api-overview.md` wiki-link pattern
- Docs-set coherence and environment-variable audit on 2026-06-01 passed after one repair:
  - sidebar coverage checker found all 88 docs files represented, with only `_Sidebar.md` intentionally not listed as a page target
  - category coverage checker found all 11 configured categories represented in operator docs
  - use-role coverage checker found all 5 current `use:` roles represented in operator docs
  - outbound proxy behavior was documented after audit: the downloader HTTP client and CAIDA ASN helper use Go's proxy-aware transport, and official Go `net/http` documentation states `ProxyFromEnvironment` honors `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`, and lowercase equivalents
  - `docs/running/environment-variables.md` now documents outbound proxy variables and `.agents/sow/specs/downloader.md` records the downloader transport contract
  - `node --check scripts/build-wiki.mjs && rm -rf /tmp/update-ipsets-wiki-env-audit && node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-env-audit https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-env-audit`
  - environment-variable coverage checker found 59 operator-relevant variables represented in environment, monitoring, security, or troubleshooting docs, with 4 template/processor-only entries ignored
  - source and generated wiki link validator checked 88 source docs pages, 202 source docs links, 88 generated wiki pages, 202 generated GitHub wiki links, and 211 generated links total; it found no missing source links, generated relative wiki links, `.md` wiki targets, nested wiki page paths, or `docs/` wiki targets
  - `git diff --check -- docs/running/environment-variables.md .agents/sow/specs/downloader.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md README.md docs scripts/build-wiki.mjs .github/workflows/wiki-sync.yml .agents/sow/specs` passed
- API/admin payload field-name audit on 2026-06-01 passed:
  - current JSON tags were extracted from `pkg/web`, `pkg/engine`, `pkg/mcp`, `pkg/scheduler`, and `internal/telemetry`
  - backticked operator-facing field names were extracted from `docs/api`, `docs/admin-ui`, and `docs/monitoring`
  - after classifying documented status values, query parameters, tool names, route terms, log fields, and metric names, the audit found no unexplained doc-only API/admin/telemetry field names requiring repair
- Post-audit validation on 2026-06-01 passed:
  - `node --check scripts/build-wiki.mjs && rm -rf /tmp/update-ipsets-wiki-api-audit && node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-api-audit https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-api-audit`
  - source and generated wiki link validator checked 88 source docs pages, 202 source docs links, 88 generated wiki pages, 211 generated links total, and found no missing source links, generated relative wiki links, `.md` wiki targets, nested wiki page paths, or `docs/` wiki targets
  - `git diff --check -- .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md .agents/sow/specs README.md docs scripts/build-wiki.mjs .github/workflows/wiki-sync.yml` passed
  - `bash .agents/sow/audit.sh` passed and reported clean SOW initialization/status consistency with 9 pending SOWs, 1 current SOW, and 87 done SOWs
  - sensitive-data scan over docs, this SOW, and SOW-0095 returned no matches for personal-name, workstation-path, bearer-token, API-key, or secret patterns
- Catalog-schema surface audit on 2026-06-01 passed after one repair:
  - non-operator surface scan confirmed `docs/contributing/*.md` content is catalog-operator guidance; the path-name mismatch remains the previously recorded move-approval question
  - current CLI/help output was checked for `daemon`, `query`, `enable`, `cache-merge`, and `iprange`; operator docs describe the supported commands and flags, while the binary's top-level help omission for `cache-merge` remains mapped to SOW-0095 as application work
  - installer/service docs were checked against `install.sh`, including `--no-restart`, custom-path caveat, config/template refresh behavior, systemd drop-ins, OpenTelemetry defaults, hardening, and restart/start semantics
  - `docs/feeds/yaml-field-reference.md` now includes top-level `defaults`, category registry fields, and `critical_asn_context` fields, while leaving detailed behavior in the focused operator docs and specs
  - YAML tag coverage checker found all 102 current YAML tags from `pkg/config/config.go` represented in docs/specs
  - `node --check scripts/build-wiki.mjs && rm -rf /tmp/update-ipsets-wiki-yaml-field-audit && node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-yaml-field-audit https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-yaml-field-audit`
  - source and generated wiki link validator checked 88 source docs pages, 202 source docs links, 88 generated wiki pages, 211 generated links total, and found no missing source links, generated relative wiki links, `.md` wiki targets, nested wiki page paths, or `docs/` wiki targets
  - `git diff --check -- docs/feeds/yaml-field-reference.md` passed
- Telemetry metric surface audit on 2026-06-01 passed without repair:
  - runtime metric names were extracted from literal `observability.*`, engine `Observe*`, and engine `observeRun*` calls in `pkg/engine`, `pkg/web`, `pkg/scheduler`, `pkg/downloader`, `pkg/processor`, `pkg/config`, `pkg/cache`, and `internal/fileutil`
  - the checker found 103 literal runtime metric names and no missing coverage in `docs/monitoring/telemetry-reference.md`, `docs/monitoring/monitoring-overview.md`, `docs/monitoring/netdata-integration.md`, or `docs/monitoring/opentelemetry-setup.md`, accounting for documented wildcard prefixes such as `entity.refresh.*_write`
- Runtime-default surface audit on 2026-06-01 passed after one repair:
  - `docs/configuration/runtime-settings.md` no longer includes `public_base_url` in the opening example that is described as "from the shipped catalog"; `configs/firehol/runtime.yaml` does not define `public_base_url` and the runtime default is empty
  - a runtime-example checker compared the first documented YAML example against `configs/firehol/runtime.yaml` and found 15/15 example keys and values matching the shipped catalog excerpt
  - `node --check scripts/build-wiki.mjs && rm -rf /tmp/update-ipsets-wiki-runtime-default-audit && node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-runtime-default-audit https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-runtime-default-audit`
  - `git diff --check -- docs/configuration/runtime-settings.md` passed
- Build-prerequisite surface audit on 2026-06-01 found and repaired one operator-doc drift:
  - `Makefile` target `build` depends on `ui-static`, and `ui-static` runs `pnpm --dir ui install --frozen-lockfile` and `pnpm --dir ui build`
  - `install.sh` exits with an explicit error when `pnpm` is missing
  - `docs/quick-start.md` now lists `pnpm` as a prerequisite and says `make build` builds the embedded web UI with `pnpm`
  - `README.md` now states that builds require Go and `pnpm`
- Processing run-status surface audit on 2026-06-01 found and repaired one operator-doc gap:
  - `pkg/engine/processing_result.go` defines `ProcessingOutcomeOK = "ok"` and `FeedProcessingResult.StatusString()` returns `ok` when no processing exception is present
  - `pkg/engine/run_pipeline.go` writes each processing result status into `report.Statuses`, and `pkg/engine/engine.go` exposes that report through the admin status snapshot as `last_report`
  - `docs/pipeline/feed-status-reference.md` now explains that `ok` is a per-run processing-report status, not the downloader changed-content status; changed downloads remain documented as `downloaded`
- Post processing run-status validation on 2026-06-01 passed:
  - `node --check scripts/build-wiki.mjs` passed
  - `git diff --check -- README.md docs .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md .agents/sow/specs` passed
  - `bash .agents/sow/audit.sh` reported `SOW initialization complete and clean`
  - generated-wiki validation built 88 root-level wiki pages and checked 202 source links, 202 generated GitHub wiki links, and 29 code-derived status values with no missing status-reference coverage
  - filesystem checks confirmed `docs/todo-history/` is absent and `.agents/sow/todo-history/` exists
  - focused stale-content scan over `README.md` and `docs/` returned no matches for old TODO-history, wiki/raw-Markdown, merge, integrity, API-field, or runtime-default drift terms
  - sensitive-data scan over README, docs, this SOW, and SOW-0095 returned no matches for personal-name, workstation-path, bearer-token, API-key, or secret patterns
- DroneBL artifact credential audit on 2026-06-01 found and repaired one operator-doc gap:
  - `tools/dronebl2ipsets/fetch.go` reads `DRONEBL_RSYNC_PASSWORD`, falls back to `RSYNC_PASSWORD`, and returns `ErrMissingPassword` when neither is set
  - `pkg/engine/artifact_stage.go` uses that fetcher for `dronebl_buildzone` artifact parents
  - `docs/feeds/artifact-parents.md`, `docs/running/environment-variables.md`, `docs/feeds/yaml-field-reference.md`, `.agents/sow/specs/config.md`, and `.agents/sow/specs/downloader.md` now document the credential variables and state that real secret values belong in environment, not YAML/docs/spec/SOW artifacts
- Post DroneBL artifact credential validation on 2026-06-01 passed:
  - `node --check scripts/build-wiki.mjs` passed
  - `git diff --check -- docs/feeds/artifact-parents.md docs/running/environment-variables.md docs/feeds/yaml-field-reference.md .agents/sow/specs/config.md .agents/sow/specs/downloader.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md` passed
  - `bash .agents/sow/audit.sh` reported `SOW initialization complete and clean`
  - source and generated wiki link validation built 88 root-level wiki pages and checked 202 source links plus 202 generated GitHub wiki links
  - sensitive-data scan over docs, specs, and this SOW found no personal-name, workstation-path, raw DroneBL/rsync password assignment, bearer-token, API-key, or secret-pattern matches
- Security/listener audit on 2026-06-01 found no operator-doc drift to patch:
  - `pkg/web/middleware.go:73` creates the general 240/minute limiter, applies it to `/api/` and `/mcp`, and returns `Retry-After: 60` on exhaustion; `docs/security/rate-limiting.md` and `docs/security/security-overview.md` already describe this behavior.
  - `pkg/web/search_api.go:96` applies the stricter 10/minute search limiter after requests pass through the general middleware; `docs/security/rate-limiting.md` already documents that search requests consume both buckets.
  - `pkg/web/middleware.go:111` wraps admin routes in Basic Auth by default, returns `503` when credentials are missing, and returns `401` for missing/wrong request credentials; `docs/security/admin-authentication.md` already describes the fail-closed behavior.
  - `pkg/web/server.go:95` rejects disabled admin auth unless `--allow-unauthenticated-admin` is also set, and `pkg/web/server.go:98` requires split `--admin-listen` to differ from `--listen` and have `runtime.public_base_url`; the security and listener-topology docs already cover these operator constraints.
  - `pkg/web/surface_handler.go:37` separates public/admin route registration by listener mode, `pkg/web/routes.go:266` registers admin routes only on admin-capable handlers, and `pkg/web/routes.go:349` returns 404 for `/admin` on the public-only listener; `docs/running/listener-topologies.md` already describes this split.
  - `pkg/web/middleware.go:173` uses trusted client-IP headers only when the matching trust flags are set, with Cloudflare first, proxy headers second, and TCP `RemoteAddr` fallback; `docs/security/production-deployment.md` already documents the same priority and spoofing risk.
- Post security/listener audit validation on 2026-06-01 passed:
  - `git diff --check -- .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md docs/security/rate-limiting.md docs/security/security-overview.md docs/security/production-deployment.md docs/security/admin-authentication.md docs/running/listener-topologies.md` passed.
  - `bash .agents/sow/audit.sh` reported `SOW initialization complete and clean`.
  - focused sensitive-data scan over the security/listener docs and this SOW returned no matches for personal names, workstation paths, bearer-token patterns, API-key assignment patterns, password assignment patterns, or secret assignment patterns.
- Navigation and surface-placement audit on 2026-06-01 found no additional docs patch needed:
  - all 88 Markdown files under `docs/` are reachable from `docs/Home.md`, `docs/_Sidebar.md`, or `README.md`; no linked docs page is missing.
  - generated GitHub Wiki output contains 88 root-level pages and no remaining local `.md`, nested wiki-path, or repository-relative page links after `scripts/build-wiki.mjs` rewrites local docs links to full GitHub Wiki URLs.
  - `docs/contributing/*.md` content is catalog-operator guidance, not fork/pull-request contributor workflow; the directory name remains the earlier recorded path mismatch because moving docs requires explicit user approval.
  - `docs/todo-history/` is absent, `.agents/sow/todo-history/` contains the preserved history files, and the only current non-SOW reference to `todo-history` is the intentional AGENTS.md pointer to `.agents/sow/todo-history/*.md`.
- CLI and config-field audit on 2026-06-01 found no additional operator-doc patch needed:
  - `cmd/update-ipsets/main.go` registers `iprange`, `query`, `enable`, `cache-merge`, `daemon`, and `version`; `docs/running/daemon-reference.md`, `docs/Home.md`, `docs/_Sidebar.md`, and `README.md` include the supported operator-visible command set. The remaining top-level help omission for `cache-merge` is an application issue already mapped to `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md`.
  - `cmd/update-ipsets/daemon.go`, `cmd/update-ipsets/query.go`, `cmd/update-ipsets/enable.go`, and `cmd/update-ipsets/cache_merge.go` flag definitions match the documented daemon/query/enable/cache-merge flag tables after the earlier repairs.
  - current `pkg/config` YAML tags total 109 operator-relevant fields; the scoped config docs (`docs/feeds/yaml-field-reference.md`, `docs/configuration/runtime-settings.md`, and `docs/configuration/configuration-concepts.md`) mention every field literally and no missing field was found in the focused coverage script.
- Route coverage audit on 2026-06-01 found no additional operator-doc patch needed:
  - route extraction from `pkg/web/routes.go` found 56 registered route patterns; every public/API/admin route family is mentioned in `docs/`, `README.md`, or both.
  - the only unmentioned registered paths were `POST /api/v1/admin/enable/` and `POST /api/v1/admin/disable/`; both are explicit `notFoundHandler` guards in `pkg/web/routes.go`, not supported operator API routes, so they should remain undocumented.
  - direct root-level `.ipset` / `.netset` compatibility downloads are covered in `docs/api/raw-file-downloads.md`, even though they are served through the SPA fallback route rather than a literal static route registration.
- README catalog-summary audit on 2026-06-01 found and repaired one wording drift:
  - `configs/firehol/sources/` contains 342 YAML files, but one file defines a merge, provider/supporting datasets are not normal public source feeds, and loaded catalog tests confirm the current expanded catalog count behavior.
  - focused catalog counting found 341 source entries, 329 public source entries after excluding hidden and ASN/GeoIP support sources, 13 merge entries, 342 public feed entries, and 11 categories.
  - `go test ./pkg/config -run 'TestCatalogExpectedCounts|TestLoadFireholCatalogCounts' -count=1` passed and confirms the current catalog still loads with the expected expanded count.
  - `README.md` now describes the project as tracking 342 public threat, blocking, and reference feeds, and the catalog table now states `342 public feeds: 329 source feeds + 13 curated merges`.
- Final SOW/content guard validation on 2026-06-01 passed after the runtime-default repair:
  - `bash .agents/sow/audit.sh` reported `SOW initialization complete and clean`
  - sensitive-data scan over changed docs and active/pending SOW files returned no matches for personal names, workstation paths, bearer-token patterns, or API-key assignment patterns
- `git diff --check -- AGENTS.md README.md docs .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/todo-history pkg/insights/insight.go pkg/insights/insights.go pkg/config/config.go ui/src/components/feed-detail/hero.tsx ui/src/components/feed-detail/section-asn.tsx` passed.
- `git diff --check -- .agents/sow/specs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md` passed after spec edits.
- Final broad validation on 2026-06-01 passed:
  - `git diff --check -- AGENTS.md README.md docs .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/specs pkg/insights/insight.go pkg/insights/insights.go pkg/config/config.go ui/src/components/feed-detail/hero.tsx ui/src/components/feed-detail/section-asn.tsx`
  - filesystem check confirmed `docs/todo-history/` is absent and `.agents/sow/todo-history/` exists
  - internal-content scan over `README.md` and `docs/` returned no matches for old todo-history paths, SOW/spec/internal path markers, TODO/FIXME/future-work language, contributor-review workflow wording, or workstation/user-identifying paths
  - stale-content scan over `README.md` and `docs/` returned no matches for old route, flag, rate-limit, reload, telemetry, processor, integrity, merge, wiki-link, or raw-Markdown failure patterns
  - `make test` passed; it rebuilt the embedded UI static bundle and ran `go test ./...`
- `go test ./tools/archposture` passed after refreshing `.agents/sow/specs/architecture-posture.md` measured highlights.
- `go test ./pkg/config ./pkg/insights` passed after updating the removed-config validation message and insights comments.
- Spec stale-contract scan returned no matches for the old rejected contract terms:
  - `public routes MUST reject stale`
  - `public stale-artifact guard`
  - `stable processed range content`
  - `processed range content`
  - `content_hash plus`
  - `excluding /healthz and admin endpoints`
  - `All public API endpoints.*admin`
  - `stale critical artifact rejection`
- Spec `provider_set_id` scan now shows the intended split only: admin integrity requires equality, public cache-first routes do not enforce equality, and direct static artifact routes do not enforce target eligibility at request time.
- Spec future/placeholder scan returned no blocking stale current-behavior issues after replacing the MCP future-tools clause with the current two-tool contract.
- `scripts/inventory.sh` was not present in this repository, so the sync-docs-specs-skills inventory helper could not be run.
- Full `make test` was run on 2026-06-01 after the final broad docs/spec audit; earlier focused package tests were also run for the behavior the docs cite. The focused architecture-posture guard was run earlier because that spec records tool output.
- Final continuation audit on 2026-06-01 found no additional operator-doc patch needed:
  - source Markdown link checker verified `README.md` plus 88 `docs/**/*.md` files and found no missing local targets or anchors
  - `cmd/update-ipsets/main.go` and subcommand help output were checked against `docs/cli/*.md` and `docs/running/daemon-reference.md`; the supported operator commands and flags are represented, while the top-level help omission for `cache-merge` remains an application issue mapped to SOW-0095
  - route coverage checker found every supported public/API/admin route family documented; `POST /api/v1/admin/enable/` and `POST /api/v1/admin/disable/` are intentional `notFoundHandler` traps and remain undocumented as usable endpoints
  - YAML tag coverage checker verified 102 current YAML tags from `pkg/config/config.go` are represented in `README.md`/`docs`, with only nested coordinate subfields allowlisted as non-operator-authored shape
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-final https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-final`
  - generated wiki output contained 88 root-level pages and no nested Markdown files, local `.md` wiki links, `.md` GitHub wiki targets, duplicated `wiki/wiki` paths, or old `docs/todo-history` references
  - internal-content scan over `README.md` and `docs/` returned no matches for old `docs/todo-history`, SOW/spec/source-code path markers, TODO/future-work wording, or workstation/user-identifying paths
  - `git diff --check` passed
  - `bash .agents/sow/audit.sh` reported `SOW initialization complete and clean`
- Explicit goal-scope completion audit on 2026-06-01 found no additional operator-doc patch needed:
  - `docs/todo-history/` is absent and `.agents/sow/todo-history/` contains the preserved internal history
  - `docs/` contains 88 Markdown files: 12 API Reference pages under `docs/api/` and 76 non-API operator pages
  - every non-navigation docs page is reachable from `docs/Home.md`, `docs/_Sidebar.md`, or `README.md`
  - broad scope scan over `README.md` and `docs/` found no SOW/spec/project-skill/source-path/TODO/future-work leakage; inspected hits were legitimate operator wording, API error wording, example clone/install commands, or catalog-maintenance content
  - `docs/contributing/*.md` content is catalog-operator guidance rather than fork, pull-request, reviewer, or developer workflow guidance; the path-name mismatch remains intentionally unmoved without explicit docs path approval
- Operator example audit on 2026-06-01 found and repaired one copy-paste behavior gap:
  - `pkg/iprange/cli.go` returns exit code `1` from `--diff` when differences are found, while still printing the differing ranges unless `--quiet` is set
  - `docs/cli/iprange-command.md` now documents the `--diff` exit-code contract for scripting operators
  - temporary-binary smoke validation passed for the documented `iprange` examples: compare, diff, intersect, exclude, combine, merge, count-unique, and reduce-factor
  - `docs/installation/installation.md` received a formatting-only blank line repair after the Markdown heading audit found one heading not preceded by a blank line outside code fences
  - Markdown heading audit passed for `README.md` and all docs pages
  - source Markdown link checker verified `README.md` plus 88 docs pages and found no missing local targets or anchors
  - generated wiki validation built 88 root-level pages and found no nested Markdown pages, local `.md` links, `.md` GitHub wiki targets, duplicated `wiki/wiki` paths, or old `docs/todo-history` references
  - `git diff --check` passed
  - `bash .agents/sow/audit.sh` reported `SOW initialization complete and clean`
- HTTP endpoint example audit on 2026-06-01 found no additional operator-doc patch needed:
  - endpoint-family checker extracted 221 URL/path examples from `README.md` and `docs/` and matched them against current public, admin, MCP, raw-file, direct-artifact, methodology, SPA, and intentional negative route families
  - method checker extracted 90 documented `GET`/`POST`/`DELETE`/`OPTIONS`/`HEAD` endpoint examples and matched them against current route method registrations and CORS handling
  - checker allowlists covered only route-family roots, placeholder raw-file examples, static/direct artifact families, and the documented traversal-attack negative example; no supported endpoint mismatch was found
- JSON and log example audit on 2026-06-01 repaired two stale operator-facing examples:
  - JSON parser validated all 7 `json` fenced blocks in `README.md` and `docs/`
  - `docs/api/health-status.md` now shows the expanded catalog `merge_count` as `13`, matching `pkg/config/catalog_verify_test.go` and `pkg/engine/query.go`
  - `docs/monitoring/log-structure.md` now shows the actual `configuration loaded` attribute names from `pkg/engine/engine.go` and explains why `sources` is the expanded in-memory catalog while `merges` is normally `0` after merge expansion
  - `go test ./pkg/config -run TestCatalogExpectedCounts -count=1` passed
  - generated wiki validation built 88 root-level pages and found no nested Markdown pages, local `.md` wiki links, or duplicated `wiki/wiki` paths
  - `git diff --check` passed
  - `bash .agents/sow/audit.sh` reported `SOW initialization complete and clean`
- Final CLI/config/navigation audit on 2026-06-01 repaired one remaining command-doc drift:
  - `docs/cli/enable-command.md` no longer claims `--verbose` prints per-feed change results; current `cmd/update-ipsets/enable.go` calls `engine.Enable` / `engine.Disable`, and `pkg/engine/engine.go` creates or removes markers without emitting per-feed stdout
  - CLI flag coverage checker verified `docs/cli/*.md` and `docs/running/daemon-reference.md` contain every current command flag for `enable`, `query`, `cache-merge`, `iprange`, and `daemon`
  - source Markdown link checker verified `README.md` plus all `docs/**/*.md` local links and anchors
  - navigation reachability checker verified every non-sidebar docs page is reachable from `README.md`, `docs/Home.md`, or `docs/_Sidebar.md`
  - route-family checker verified 42 public/API/admin route families from `pkg/web/routes.go` are documented
  - YAML tag coverage checker verified all 102 current YAML tags from `pkg/config/config.go` are represented in `README.md`/`docs`, with only nested coordinate subfields allowlisted
  - stale internal-content scan over `README.md` and `docs/` found no SOW, TODO, `docs/todo-history`, or workstation path leakage
  - `.agents/sow/todo-history/` contains 48 preserved Markdown history files and `docs/todo-history/` is absent
- Follow-up telemetry metric audit on 2026-06-01 repaired two remaining operator-doc gaps:
  - `pkg/downloader/downloader.go:122-134` defaults the downloader outcome status to `error` when no downloader `Result` exists and emits `download.<status>`, while `pkg/downloader/downloader.go:21-29` defines `failed` as a separate downloader result status; `docs/monitoring/telemetry-reference.md`, `docs/monitoring/monitoring-overview.md`, and `docs/monitoring/netdata-integration.md` now distinguish `download.failed` from `download.error`
  - `pkg/engine/run_metrics_state.go:27-33` emits aggregate operation counts under the base metric name and aggregate duration histograms under `<metric>.aggregate.duration_ms`; current aggregate callers are `metadata.comparison_pair_overlap`, `metadata.comparison_pair_skipped`, and `metadata.comparison_merge_rows` at `pkg/engine/output.go:508-516` and `pkg/engine/output.go:625`; `docs/monitoring/telemetry-reference.md` now documents the aggregate histogram suffix
  - focused telemetry coverage checked 6 downloader outcomes, 12 scheduler decision statuses, 10 engine phases, and 3 aggregate comparison operations against `docs/monitoring/telemetry-reference.md`
  - `git diff --check -- docs/monitoring/telemetry-reference.md docs/monitoring/monitoring-overview.md docs/monitoring/netdata-integration.md` passed
  - `node --check scripts/build-wiki.mjs && rm -rf /tmp/update-ipsets-wiki-telemetry-audit && node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-telemetry-audit https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-telemetry-audit`
  - `bash .agents/sow/audit.sh` reported `SOW initialization complete and clean`
  - added-line sensitive/path scan over the changed monitoring docs and this SOW found no personal name, workstation path, bearer-token pattern, API-key assignment, or raw secret assignment after excluding placeholder credential text
- Operator-surface, link, and route-example audit on 2026-06-01 found no additional operator-doc patch needed:
  - strict leakage scan over `README.md` and all 88 `docs/**/*.md` files found no SOW, `.agents`, `AGENTS.md`, source-path, TODO/FIXME, developer-workflow, personal-name, or workstation-path terms
  - source Markdown link checker verified `README.md` plus all 88 docs pages, including local anchors
  - generated wiki validation built 88 root-level pages and found no nested Markdown pages, local wiki links, `.md` GitHub wiki targets, or duplicated `wiki/wiki` paths
  - documented route-example checker matched 143 endpoint examples and 105 unique method/path examples against current public, admin, MCP, SPA, direct-artifact, raw-file, methodology, and CORS route families
  - no docs changes were required from this pass
- Generated metadata-file audit on 2026-06-01 repaired one stale `llms.txt` example:
  - `pkg/engine/output.go:1320-1359` currently renders `llms.txt` with service status, categories, feed catalog, global IP search, countries, ASNs, maintainers, methodology API, optional compose example, legacy catalog, public feed API index, optional example feed detail, sitemap, and robots links
  - `docs/api/metadata-files.md` now documents those current generated `llms.txt` surfaces instead of the older shorter subset
  - sitemap and robots documentation still matched the current generator and serving path: `pkg/engine/output.go:930-985`, `pkg/engine/output.go:1296-1318`, and `pkg/web/routes.go:363-462`
  - `go test ./pkg/engine -run 'TestRenderLLMSTXTOmitsAdminPaths|TestWritePublicMetadataFilesBuildsSitemapIndexAndDetailShards' -count=1` passed
  - metadata coverage script verified 13 documented generated surfaces against `docs/api/metadata-files.md`
  - `git diff --check -- docs/api/metadata-files.md` passed
  - `node --check scripts/build-wiki.mjs && rm -rf /tmp/update-ipsets-wiki-metadata-audit && node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-metadata-audit https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-metadata-audit`
- Install/runtime path audit on 2026-06-01 repaired one stale non-root fallback description:
  - `configs/firehol/runtime.yaml:2-6` currently defines bundled path templates as `${HOME}/ipsets`, `${HOME}/.update-ipsets`, `${HOME}/.update-ipsets/cache`, and `${HOME}/.update-ipsets/lib`
  - `pkg/engine/runtime.go:86-117` applies the effective non-root runtime fallback when the daemon is not root and path settings are unset or still equal to built-in defaults: `$HOME/.update-ipsets/ipsets`, `$HOME/.update-ipsets/run`, `$HOME/.cache/update-ipsets`, and `$HOME/.local/share/update-ipsets`
  - `docs/running/environment-variables.md` and `docs/configuration/runtime-settings.md` now distinguish the shipped YAML templates from the effective non-root runtime fallback
  - `.agents/sow/specs/files-layout.md` now records the same effective non-root runtime layout contract
  - install and systemd docs otherwise matched current installer behavior in `install.sh:54-74`, `install.sh:148-199`, `install.sh:215-284`, and `install.sh:293-310`
  - runtime path coverage script verified the documented shipped YAML template paths against `configs/firehol/runtime.yaml` and the documented effective non-root paths against `pkg/engine/runtime.go`
  - `git diff --check -- docs/running/environment-variables.md docs/configuration/runtime-settings.md` passed
  - `node --check scripts/build-wiki.mjs && rm -rf /tmp/update-ipsets-wiki-install-path-audit && node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-install-path-audit https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-install-path-audit`
- Operator-surface wording audit on 2026-06-01 repaired remaining developer-framed wording in operator docs:
  - `docs/running/admin-authentication.md:51` and `docs/admin-ui/accessing-admin.md:31-33` now describe disabled admin authentication as local testing or trusted lab use instead of local development use.
  - `docs/running/daemon-reference.md:3` and `docs/running/daemon-reference.md:160` now frame the unauthenticated example as local testing versus production.
  - `docs/running/listener-topologies.md:92`, `docs/security/security-overview.md:50`, and `docs/security/admin-authentication.md:71` now use local testing / controlled lab wording.
  - `docs/installation/installation.md:9` and `docs/installation/installation.md:33` now say pnpm builds the embedded web UI instead of naming the UI framework; operators need the build prerequisite and installed result, not internal framework identity.
  - repeat surface scan over `README.md` and `docs/` found no remaining `docs/todo-history`, SOW, `.agents`, `AGENTS.md`, TODO/FIXME, workstation path, `local development`, or `React app` leakage. The only remaining `implementation guides` hit is the deliberate API methodology-page boundary warning in `docs/api/methodology-endpoints.md`.
  - `git diff --check -- docs/running/admin-authentication.md docs/running/daemon-reference.md docs/running/listener-topologies.md docs/admin-ui/accessing-admin.md docs/security/security-overview.md docs/security/admin-authentication.md docs/installation/installation.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md` passed.
  - source Markdown link checker verified `README.md` plus 88 docs pages.
  - generated wiki validation built 88 root-level pages and found no nested Markdown pages, local wiki links, `.md` GitHub wiki targets, duplicated `wiki/wiki` paths, or old `docs/todo-history` references.
  - `bash .agents/sow/audit.sh` reported `SOW initialization complete and clean`.
  - focused added-line sensitive-data scan returned no personal name, workstation path, bearer token, API-key assignment, password assignment, secret assignment, or token assignment matches.
- Classification/provider-reference audit on 2026-06-01 repaired maintainer endpoint eligibility wording:
  - `pkg/engine/home_detail.go:270-277` builds maintainer index rows only after `homeSummaryEligible` passes and the feed health is `healthy` or `delayed`.
  - `pkg/engine/home_summary.go:123-140` defines the shared eligibility filter: source is not hidden, not ASN/GeoIP provider-only data, belongs to a public category, and uses `primary` or `secondary_upstream` provenance.
  - `pkg/engine/home_globe.go:87-88` defines the health subset as `healthy` or `delayed`.
  - `docs/api/classification-endpoints.md` now states that maintainer index/detail endpoints include only currently eligible public feeds, not every catalog source with maintainer metadata.
  - `.agents/sow/specs/website.md` now records the same maintainer index/detail eligibility contract.
  - category docs were checked against `configs/firehol/categories.yaml`; all 11 shipped category rows are represented.
  - provider-default docs were checked against `configs/firehol/defaults.yaml`; `asn_provider: iptoasn` and `geo_provider: dbip_country` are represented in operator docs.
  - `go test ./pkg/engine ./pkg/web -run 'Test.*Maintainer|Test.*HomeSummary|Test.*Categories|TestPublicCatalogExcludesProviderDatasets' -count=1` passed.
  - `git diff --check -- docs/api/classification-endpoints.md .agents/sow/specs/website.md` passed.
  - generated wiki validation built 88 root-level pages.
  - focused added-line sensitive-data scan returned no personal name, workstation path, bearer token, API-key assignment, password assignment, secret assignment, or token assignment matches.
- Feed-health classifier audit on 2026-06-01 repaired stale operator wording:
  - `pkg/feedhealth/feedhealth.go:98-164` classifies never-published feeds as `unavailable`, zero-entry successful publications as `empty`, one-observation feeds as `healthy` during grace, and age-ladder feeds as `delayed`, `risky`, or `unmaintained`.
  - `pkg/feedhealth/feedhealth.go:207-230` defines the age ladder and excludes only `exclude_from_unmaintained`, `critical_infrastructure`, `provider_context`, `asn`, and `geoip` sources from age-based freshness degradation.
  - `pkg/feedhealth/feedhealth.go:233-249` and `pkg/feedhealth/feedhealth.go:324-337` define unavailable/archived inputs as current download/provider failures or stale local data beyond the threshold, not generic processing failures.
  - `docs/pipeline/health-classes.md` and `docs/troubleshooting/understanding-feed-health.md` now describe grace, delayed/risky/unmaintained, empty, unavailable, archive, and additive/subtractive merge-health behavior in operator terms.
  - `docs/configuration/runtime-settings.md`, `docs/running/environment-variables.md`, and `docs/glossary.md` no longer carry stale health-class or runtime path wording.
  - `go test ./pkg/feedhealth ./pkg/engine ./pkg/web -run 'TestClassify|Test.*Health|Test.*Merge|Test.*AdminFeeds|Test.*HomeSummary' -count=1` passed.
  - runtime path coverage verified the shipped YAML template paths from `configs/firehol/runtime.yaml` and the effective non-root fallback paths from `pkg/engine/runtime.go` are represented in operator docs/specs.
  - stale health/path wording scan over `README.md`, `docs/`, and `.agents/sow/specs/` returned no matches for the old grace, unavailable, empty, risky-threshold, stale-path, or bogon-age-ladder wording.
  - source Markdown link checker verified `README.md` plus 88 docs pages and 205 local links.
  - `node --check scripts/build-wiki.mjs` passed.
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-feed-health-audit https://github.com/firehol/update-ipsets/wiki` reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-feed-health-audit`.
  - generated wiki validation checked 88 root-level pages, 202 generated GitHub wiki links, and 205 total links with no local `.md`, nested wiki page, duplicated `wiki/wiki`, `docs/`, or old `docs/todo-history` targets.
- Follow-up effective-runtime path audit on 2026-06-01 repaired the earlier path-template-only interpretation:
  - `configs/firehol/runtime.yaml:2-6` defines shipped YAML template fallbacks as `${HOME}/ipsets`, `${HOME}/.update-ipsets`, `${HOME}/.update-ipsets/cache`, and `${HOME}/.update-ipsets/lib`.
  - `pkg/engine/runtime.go:86-117` applies a different effective non-root fallback when path settings are unset or still equal to built-in defaults: `$HOME/.update-ipsets/ipsets`, `$HOME/.update-ipsets/run`, `$HOME/.cache/update-ipsets`, and `$HOME/.local/share/update-ipsets`.
  - `docs/running/environment-variables.md`, `docs/configuration/runtime-settings.md`, and `.agents/sow/specs/files-layout.md` now distinguish shipped YAML template fallbacks from effective non-root runtime fallbacks.
  - `docs/running/environment-variables.md` now also explains that `FIREHOL_CONFIG_DIR` and `FIREHOL_SHARE_DIR` are legacy placeholders inside the shipped supplementary-directory templates, and operators should set `ADMIN_SUPPLIED_IPSETS` or `DISTRIBUTION_SUPPLIED_IPSETS` directly when those legacy variables are not present.
  - `go test ./pkg/engine ./pkg/config -run 'TestExpandTemplateShellDefaults|TestResolveRuntimeDisablesKernelApplyForUserMode|TestRuntimeTemplateExpansionInFireholCatalog|TestLoadLegacyReadsRuntimeAssignments' -count=1` passed.
  - environment-variable coverage verified 48 documented runtime, observability, installer, DroneBL, proxy, and legacy placeholder variables against current code and installer inputs.
  - runtime path coverage verified 5 shipped YAML templates and 4 effective non-root paths against current code/docs/specs.
  - stale path-template-only wording scan returned no matches for old non-root fallback claims.
  - `git diff --check -- docs/running/environment-variables.md docs/configuration/runtime-settings.md .agents/sow/specs/files-layout.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md` passed.
- Follow-up provider-list API audit on 2026-06-01 repaired one stale per-feed provider route description:
  - `pkg/web/routes.go:143-156` serves `/api/v1/sets/{feed}/countries`, `/asn`, and `/bogons` by returning `GeoProviders`, `ASNProviders`, and `BogonProviders`.
  - `pkg/engine/public.go:195-262` builds those provider lists from configured provider-family sources, not from the current feed's materialized artifact availability.
  - `docs/api/feed-endpoints.md` now states that provider-list routes return configured providers and that provider-specific routes may return `404` when a configured provider has no readable feed-specific artifact.
  - `docs/feeds/provider-databases.md` now explains the same configuration-driven provider-tab behavior for operators.
  - `.agents/sow/specs/website.md` now records the contract that provider-list routes MUST NOT filter by current feed artifact availability, while provider-specific routes are artifact readers.
  - `go test ./pkg/web ./pkg/engine -run 'Test.*Provider|Test.*Bogon|Test.*ASN|Test.*Country|Test.*PublicSet|Test.*Critical|Test.*Home' -count=1` passed.
  - source Markdown link checker verified `README.md` plus 88 docs pages and 205 local links.
  - `node --check scripts/build-wiki.mjs && node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-provider-list-audit https://github.com/firehol/update-ipsets/wiki` passed and reported `Built 88 wiki pages in /tmp/update-ipsets-wiki-provider-list-audit`.
  - generated wiki validation checked 88 root-level pages, 202 generated GitHub wiki links, and 211 total links with no nested Markdown pages, local `.md` links, `.md` GitHub wiki targets, duplicated `wiki/wiki`, `docs/`, or old `docs/todo-history` targets.
  - `git diff --check -- README.md docs .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md .agents/sow/specs` passed.
  - `bash .agents/sow/audit.sh` reported `SOW initialization complete and clean`.
  - added-line sensitive-data scan over README, docs, wiki builder/workflow, this SOW, SOW-0095, and specs found no personal-name, workstation-path, bearer-token, API-key, password, secret, or token assignment matches.
- Final operator-surface wording audit on 2026-06-01 repaired one remaining security-doc phrasing issue:
  - `pkg/web/middleware.go:73-87` applies the built-in 240/minute limiter to paths under `/api/` and `/mcp`.
  - `docs/security/rate-limiting.md` now labels the covered bucket as general `/api/` and `/mcp` requests, and the old `Implementation` heading was rewritten as `How Limits Are Applied`.
  - `docs/api/rate-limits-cors.md` now describes rate limiting, CORS, and compression for HTTP API surfaces rather than only the public API, because the page also documents admin API and MCP rate-limit behavior.
  - Repeat operator-surface scans over README and `docs/` found no remaining hits for developer workflow terms, SOW/TODO leakage, workstation paths, implementation-plan wording, or local-development/React-app wording. The remaining `goroutines` hits are operator-visible resource fields in admin status and telemetry docs.
  - Route example coverage verified 73 documented method/path examples against 54 registered routes plus dynamic/direct route families.
  - CLI flag coverage verified daemon, query, enable, cache-merge, and iprange documented flag surfaces against current command flag definitions.
  - YAML tag coverage verified 102 current YAML tags from `pkg/config/config.go` are represented in README/docs, with only nested coordinate/unit/value subfields allowlisted.
  - Processor coverage verified 77 registered processor names are represented in `docs/feeds/processors.md`.
  - Monitoring coverage verified 41 observability metric/span names are represented in monitoring docs.
  - `go test ./pkg/web ./pkg/processor ./internal/observability -count=1` passed.
  - Source Markdown link checker verified README plus 88 docs pages and 205 local links.
  - `node --check scripts/build-wiki.mjs` passed.
  - Generated wiki validation built 88 root-level pages and checked 202 generated GitHub wiki links and 211 total links with no nested Markdown pages, local `.md` links, `.md` GitHub wiki targets, duplicated `wiki/wiki`, `docs/`, or old `docs/todo-history` targets.
  - `git diff --check -- README.md docs .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md .agents/sow/specs` passed.
  - `bash .agents/sow/audit.sh` reported `SOW initialization complete and clean`.
  - Added-line sensitive-data scan over README, docs, wiki builder/workflow, this SOW, SOW-0095, and specs found no personal-name, workstation-path, bearer-token, API-key, password, secret, or token assignment matches.
- Continuation completion-audit pass on 2026-06-01 found and repaired one additional OpenTelemetry docs gap:
  - `internal/observability/observability.go:89` calls `resource.WithFromEnv()`, so the daemon reads standard OpenTelemetry resource identity variables through the SDK.
  - `internal/observability/observability.go:202-219` creates the default OTLP trace, metric, and log exporters, whose environment configuration is part of the `go.opentelemetry.io/otel` modules pinned in `go.mod`.
  - `go.opentelemetry.io/otel/sdk@v1.43.0` documents `OTEL_RESOURCE_ATTRIBUTES` and `OTEL_SERVICE_NAME`; the OTLP exporter modules pinned in `go.mod` document the standard endpoint, headers, timeout, compression, insecure, certificate, client certificate, and client key variables plus signal-specific variants.
  - `docs/running/environment-variables.md` now lists the standard resource/exporter variables alongside the existing project-specific OpenTelemetry toggles and endpoint variables.
  - `docs/monitoring/opentelemetry-setup.md` now gives operator examples for service identity, resource attributes, collector headers, timeout, compression, and TLS/mTLS certificate variables.
  - `.agents/sow/specs/operating-principles.md` now records that the daemon must honor standard OpenTelemetry SDK resource/exporter environment configuration when export is enabled.
  - `docs/` currently contains 88 Markdown pages, `docs/todo-history/` is absent, and `.agents/sow/todo-history/` contains 48 preserved Markdown history files.
  - A stricter operator-surface scan over README and `docs/` found no SOW/TODO leakage, developer-workflow terms, workstation paths, implementation-plan wording, local-development wording, or React-app wording. The only remaining `contributing` hits are repository source paths under `docs/contributing/` and telemetry metric names such as `http.home_summary.contributing_feeds`; the `docs/contributing/*.md` page titles and content are catalog-maintenance guidance for operators.
  - `make test` passed and ran `go test ./...` for the root module.
  - Post-repair environment coverage check verified `docs/running/environment-variables.md` and `docs/monitoring/opentelemetry-setup.md` cover 69 named runtime, installer, credential, proxy, systemd, Go runtime, and OpenTelemetry variables plus 7 OpenTelemetry signal-specific exporter variable families.
  - `go test ./internal/observability -count=1` passed.
  - Source Markdown link checker verified README plus 88 docs pages and 213 local links.
  - `node --check scripts/build-wiki.mjs` passed.
  - GitHub Wiki build generated 88 root-level pages in `/tmp/update-ipsets-wiki-otel-env-audit`; validation checked 201 generated GitHub Wiki links and 211 total links with no nested Markdown pages, local `.md` links, `.md` GitHub wiki targets, duplicated `wiki/wiki`, `docs/`, or TODO-history targets.
  - `git diff --check -- docs/running/environment-variables.md docs/monitoring/opentelemetry-setup.md .agents/sow/specs/operating-principles.md .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md` passed.
  - `bash .agents/sow/audit.sh` reported `SOW initialization complete and clean`.
  - Added-line sensitive-data scan over README, docs, wiki builder/workflow, this SOW, SOW-0095, and specs found no personal-name, workstation-path, bearer-token, API-key, password, secret, or token assignment matches after allowing documented placeholder values such as `change-this-secret`.
- Continuation completion-audit pass on 2026-06-01 rechecked the current worktree before closure:
  - Current tree has 88 Markdown files under `docs/`, 17 spec Markdown files under `.agents/sow/specs/`, 9 runtime project skills, no `docs/todo-history/` directory, and 48 preserved Markdown history files under `.agents/sow/todo-history/`.
  - Surface scan over README and `docs/` found no SOW/TODO leakage, workstation paths, old `docs/todo-history` paths, or unresolved TODO/future-work placeholders. Inspected hits were operator examples (`localhost`), repository install/update wording, API error terms, operator-visible resource names such as `goroutines`, catalog enum examples, or the existing catalog-maintenance path.
  - The `docs/contributing/*.md` content is operator-facing catalog-maintenance guidance, but the directory path still carries a developer-facing label. This was not renamed in this pass because moving public docs changes link/wiki slugs and needs an explicit user decision; the visible sidebar label is already `Catalog Maintenance`.
  - Config coverage verified 109 current YAML tags from `pkg/config` are represented in README/docs, excluding only nested coordinate/unit/value and evidence subfields that are intentionally not top-level operator fields.
  - Processor coverage verified all 43 currently registered processor names are represented in feed docs.
  - CLI coverage verified all discovered flags for `daemon`, `query`, `enable`, `cache-merge`, and `iprange` are represented in CLI/daemon docs.
  - Environment coverage verified 59 non-local runtime, installer, proxy, credential, systemd, Go runtime, and OpenTelemetry variables/families are represented in operator docs.
  - Route-family coverage checked 83 registered/dynamic route families and found the current routes represented in README/docs; only deliberate not-found compatibility stubs for old admin action prefixes were allowlisted.
  - Documented HTTP example coverage checked 131 examples against current route families or intentional external examples.
  - `make test` passed and ran `go test ./...` for the root module.
  - Source Markdown link checker verified README plus 88 docs pages and 213 local links.
  - `node --check scripts/build-wiki.mjs` passed, and the GitHub Wiki builder generated 88 root-level pages in `/tmp/update-ipsets-wiki-completion-audit`.
  - Generated wiki validation checked 201 generated GitHub Wiki links and 211 total links with no nested Markdown pages, local `.md` links, `.md` GitHub wiki targets, duplicated `wiki/wiki`, `docs/`, or TODO-history targets.
  - `git diff --check -- README.md docs .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md .agents/sow/specs` passed.
  - `bash .agents/sow/audit.sh` reported `SOW initialization complete and clean`.
  - Added-line sensitive-data scan over README, docs, wiki builder/workflow, this SOW, SOW-0095, and specs found no personal-name, workstation-path, bearer-token, API-key, password, secret, or token assignment matches after allowing documented placeholder values.
  - Stale-path scan over README, docs, wiki builder, and wiki workflow found no `docs/todo-history`, `.agents/sow/todo-history`, personal-name, workstation-path, SOW-file, or TODO-file leakage.
- Final user-approved closure pass on 2026-06-01:
  - Renamed `docs/contributing/` to `docs/catalog-maintenance/` after explicit user approval.
  - Updated source docs links in `docs/Home.md` and `docs/_Sidebar.md` to the new catalog-maintenance path.
  - User approved SOW closure, move to `.agents/sow/done/`, and one commit containing the approved docs/spec/wiki/SOW changes.
  - `make test` passed after the path rename and SOW move.
  - Source Markdown link checker verified 89 Markdown files and 213 local links.
  - `node --check scripts/build-wiki.mjs` passed, and the GitHub Wiki builder generated 88 root-level pages in `/tmp/update-ipsets-wiki-final`.
  - Generated wiki validation checked 88 wiki pages, 201 generated GitHub Wiki links, and 211 total links with no nested Markdown pages, local `.md` links, `.md` GitHub wiki targets, duplicated `wiki/wiki`, `docs/`, TODO-history targets, or stale `contributing/` wiki targets.
  - `git diff --check -- README.md docs .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/done/SOW-0094-20260531-operator-docs-sync.md .agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md .agents/sow/specs` passed.
  - `bash .agents/sow/audit.sh` reported current SOW empty, completed SOW status valid, and sensitive-data guardrail clean.
  - Added-line sensitive-data scan found no personal-name, workstation-path, bearer-token, API-key, password, secret, or token assignment matches after allowing documented placeholder values.
  - Stale-path scan over README, docs, wiki builder, and wiki workflow found no `docs/todo-history`, `.agents/sow/todo-history`, or stale `contributing/` references.

Real-use evidence:

- The live GitHub Wiki showed two navigation failure modes: nested `.md` wiki links redirect to raw markdown, and bare generated slugs render in custom sidebar content as relative links instead of explicit GitHub wiki URLs.
- GitHub Wiki compatibility was validated locally by generating the wiki destination and requiring every generated local link to resolve through a full `https://github.com/firehol/update-ipsets/wiki/...` URL without `.md`, bare page slugs, or nested relative `wiki/...` targets.

Reviewer findings:

- No external reviewers were run; the user did not request external second-opinion agents.

Same-failure scan:

- Repeated route/flag/config-field/environment/stale-term scans after repairs found no required remaining docs repairs in the audited operator-doc scope before the later admin UI queue/status/enablement audit; the later audit repaired the three concrete admin-doc drifts recorded above, the later CLI example audit repaired the one stale `enable --disable` flag-order example, the later config-field audits repaired stale/missing `renames` / `deleted` cleanup-registry documentation and the missing processor `args` YAML wording, and the later environment audits repaired OpenTelemetry metric-interval value semantics plus standard OpenTelemetry SDK resource/exporter variable coverage.
- Repeated spec scans after repairs found no remaining occurrences of the stale public critical-overlap `provider_set_id` rejection wording or stale provider-set content-hash/cardinality identity wording.
- Remaining application-review notes are recorded in this SOW and mapped to `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md`: the general `/api/` limiter includes admin API routes, `ignore_repeating_download_errors` is accepted/carried through runtime state but is not used by current scheduler retry timing, top-level CLI usage omits the supported `cache-merge` subcommand, `skip_comparison_if_no_updates=false` does not by itself force a no-update heavy regeneration, the feed detail drawer says "linear backoff" while scheduler delay doubles after repeated failures, `enabled_by_all` is accepted catalog metadata but not used by current runtime enable-all paths, MCP `fetch_analysis` describes `AS`-prefixed ASN names without normalizing them, and `install.sh` accepts a custom install directory while emitting a systemd unit hardcoded for `/opt/update-ipsets`.

Sensitive data gate:

- Passed for touched artifacts after sanitizing moved TODO-history files and this SOW. Examples use public documentation domains, reserved example addresses where applicable, placeholder secrets instead of real credentials, and `user` / `/home/user` for personal references.

Artifact maintenance gate:

- AGENTS.md: updated only to point preserved TODO history at `.agents/sow/todo-history/*.md`.
- `.gitignore`: updated to allow tracked moved TODO-history files under `.agents/sow/todo-history/`.
- Runtime project skills: no update needed for this pass; the current surface rules already covered the observed issue.
- Specs: updated under `.agents/sow/specs/` for current critical-overlap serving/integrity split, provider-set identity inputs, compose bounds, rate-limit middleware behavior, MCP tool surface, direct critical artifact serving, current feed-body finalization/publication order, current `enabled_by_all`/`--enable-all` behavior, maintainer endpoint eligibility, provider-list API semantics, processor step `args`, effective non-root runtime layout, OpenTelemetry metric-interval parsing and standard SDK resource/exporter environment handling, outbound downloader proxy variables, DroneBL artifact credential variables, current active-processing memory behavior, and architecture-posture measured highlights. No extra feed-health spec patch was needed in this audit because `.agents/sow/specs/feeds.md` already matched the current classifier contract for age ladder, unavailable/archive, empty, and age-ladder suppression behavior.
- End-user/operator docs and publishing: updated across `docs/` for operator purpose, current routes/flags/config fields, processor step `args`, cleanup-registry semantics, public API field groups, provider-list API semantics, generated metadata-file surfaces, downloader options and outbound proxy variables, DroneBL artifact credential variables, migration-helper variables, homepage summary query behavior, maintainer endpoint eligibility, feed-health class semantics, security rate-limit/auth/reload behavior including `/mcp` rate-limit coverage, runtime settings, path templates, effective non-root runtime layout, OpenTelemetry metric-interval values and standard SDK resource/exporter variables, downloader outcome metrics, aggregate timing histograms, build prerequisites, local testing/admin-auth wording, processing run-report status meaning, bogon role visibility, background-work visibility, pipeline publication order, installer template-update behavior, catalog-maintenance path naming, and repository-relative links; updated `.github/workflows/wiki-sync.yml` and `scripts/build-wiki.mjs` so the GitHub Wiki destination receives flattened pages with full GitHub wiki links.
- End-user/operator skills: none present or affected.
- SOW lifecycle: this SOW is completed and moved to `.agents/sow/done/` with the implementation/docs/spec/wiki changes in the same approved commit; review notes that require user/product decision before further code changes are tracked by `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md`.

Specs update:

- `.agents/sow/specs/operating-principles.md`: updated to current cache-first critical-overlap serving, `/api/`/`/mcp` rate-limit behavior, and OpenTelemetry metric export interval parsing.
- `.agents/sow/specs/downloader.md`: updated to distinguish staged feed bodies from supporting provider/artifact staged inputs and to record outbound HTTP proxy environment variables honored by the downloader transport plus DroneBL rsync credential environment-variable precedence.
- `.agents/sow/specs/files-layout.md`: updated to current provider-set marker purpose, identity inputs, committed feed-body finalization ownership, and effective non-root runtime layout.
- `.agents/sow/specs/pipeline.md`: updated to exclude materialized provider content from critical provider-set identity and to describe the current feed-body finalization/public-artifact publication order.
- `.agents/sow/specs/feeds.md`: updated to current public aggregate-serving behavior when `provider_set_id` drifts.
- `.agents/sow/specs/integrity.md`: clarified the difference between cleanup/API target eligibility and direct static artifact cache-first serving.
- `.agents/sow/specs/website.md`: added `/api/v1/compose` bounds, narrowed MCP to the current registered tools, and clarified configuration-driven provider-list route semantics for GeoIP, ASN, and bogon data.
- `.agents/sow/specs/architecture-posture.md`: refreshed measured posture highlights from the current repository.
- `.agents/sow/specs/config.md`: clarified that `enabled_by_all` is accepted legacy catalog metadata while current `--enable-all` does not filter by it, that `renames` / `deleted` are cleanup registries rather than public API aliases, and that `dronebl_buildzone` rsync credentials come from environment variables rather than YAML.
- `.agents/sow/specs/memory-management.md`: clarified that canonical feed-body normalization may build an in-memory active range set and rendered canonical body for the current source, while routine public lookup/comparison and published serving prefer file-backed or streaming reads.
- `.agents/sow/specs/processing-engine.md`: updated to current committed feed-body finalization, public artifact staging, and `finalize_failed` semantics.

Project skills update:

- No update needed in this pass. Existing content-surface rules already required the separation that was applied.

End-user/operator docs update:

- Updated as recorded above, including outbound proxy variables in `docs/running/environment-variables.md` and current feed-health class semantics in the pipeline, troubleshooting, runtime, and glossary pages.

End-user/operator skills update:

- No end-user/operator skills were present or affected.

Lessons:

- GitHub Wiki publishing should flatten docs into wiki-root page files and rewrite local links to full GitHub wiki URLs without `.md` extensions. Source docs should keep repository-relative `.md` links; otherwise the repository docs view regresses while trying to fix the wiki.

Follow-up mapping:

- `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md` tracks whether the current `/api/` prefix rate limiter should continue to include authenticated admin API routes.
- `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md` tracks whether `ignore_repeating_download_errors` should be implemented in scheduler retry timing, deprecated, or removed from the documented/runtime configuration surface.
- `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md` tracks whether top-level `update-ipsets help` should list the supported `cache-merge` subcommand.
- `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md` tracks whether `skip_comparison_if_no_updates` should keep its current limited no-update role, be implemented as an explicit regeneration control, or be documented/deprecated as a compatibility field.
- `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md` tracks whether the feed detail drawer should say "exponential backoff" or "retry backoff" instead of "linear backoff".
- `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md` tracks whether `enabled_by_all` should be implemented in runtime enable-all paths, deprecated, or removed from the catalog surface.
- `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md` tracks whether MCP `fetch_analysis` should normalize `AS`-prefixed ASN names or whether the tool description should require numeric ASN identifiers only.
- `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md` tracks whether `install.sh /custom/path` should generate a matching systemd unit for that path, reject custom paths for managed installs, or remain documented as manual/experimental.

## Outcome

Operator-doc, API Reference, wiki-publishing/navigation, approved TODO-history relocation, approved catalog-maintenance path rename, stale-reference cleanup, migration-helper documentation, downloader-option documentation, feed-health documentation, logging-format documentation, enablement documentation, and spec-as-current-application repairs are implemented and validated. Remaining application-review items are tracked by `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md`.

## Lessons Extracted

- Keep GitHub Wiki output links as full GitHub wiki URLs without `.md` extensions through `scripts/build-wiki.mjs`; keep source docs repository-relative with `.md` links.
- Keep internal design history outside `docs/`; preserved history now lives under `.agents/sow/todo-history/`.
- Specs must describe current application behavior as-is even when that behavior may need product review later; suspected issues belong in the SOW issue notes with evidence.

## Followup

- `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md` tracks the product/code questions found during this docs/spec sync.

## Regression Log

## Regression - 2026-06-02

The user requested a review-only documentation accuracy pass with four internal
reviewers and no code changes, tests, scripts, or external agents. The review
found twelve documentation accuracy issues after this SOW had been completed.
The user approved fixing those findings and committing the result.

Findings to repair:

- `docs/api/compose-endpoint.md`: compose currently buffers the output and can
  fall back from binary set files to materialized text files; it does not stream
  directly from binary-only inputs.
- `docs/api/metadata-files.md`: only ASN sitemap shards are chunked at 45,000
  URLs; the fixed category shards are expected to remain below protocol limits.
- `docs/security/security-overview.md`: the daemon CLI default requires admin
  auth, while the installed systemd unit deliberately disables admin auth for
  private-network operation unless the operator changes the drop-in variables.
- `docs/security/admin-authentication.md`: systemd authentication requires
  changing the auth-mode argument and clearing the unauthenticated-admin
  acknowledgment, not only adding username/password credentials.
- `docs/running/environment-variables.md`: several names are legacy config-file
  assignment names or YAML-template variables, not process environment overrides
  honored by the shipped YAML catalog.
- `docs/feeds/yaml-field-reference.md`: `ipv` is required for set-producing
  sources/merges; `downloader_options` are not environment-expanded;
  `accept_empty` is accepted but not currently effective for ordinary source
  downloads; declared category labels are required; source category is a public
  taxonomy requirement rather than a loader-enforced field.
- `docs/feeds/source-feeds.md`: the same downloader-options example implied
  secret environment expansion in header values, and the source-feed key table
  overstated `frequency` as required even though `0` / omitted means not
  auto-scheduled.
- `docs/feeds/merge-feeds.md`: merge `frequency` is optional and defaults to
  `runtime.processing_interval_minutes` when omitted or zero.
- `docs/glossary.md`: ASN and GeoIP provider databases are not public feeds, but
  bogon provider sources may also be public feeds unless hidden.

Validation plan for this regression:

- Re-read the corrected documentation snippets against the same code evidence
  cited by the reviewers.
- Run `git diff --check` on touched files.
- Do not run tests or scripts for this regression because the user explicitly
  asked for a review-only basis and these fixes are documentation-only.

Repairs applied:

- Corrected compose endpoint wording to cover binary-file preference, text-body
  fallback, and bounded buffering.
- Corrected sitemap shard wording so only ASN shards claim 45,000 URL chunking.
- Corrected admin-auth docs to distinguish daemon CLI defaults from the
  installed private-network systemd defaults, and to show the required drop-in
  variables for enabling installed-service authentication.
- Corrected environment-variable docs to distinguish process environment
  variables, runtime YAML fields, and legacy config-file assignment names.
- Corrected feed YAML docs for `ipv`, category labels, source category
  semantics, literal downloader options, and `accept_empty`.
- Corrected source-feed docs for literal downloader options, secret handling,
  optional `frequency`, and public-taxonomy category semantics.
- Corrected merge docs so `frequency` is optional and falls back to
  `runtime.processing_interval_minutes`.
- Corrected the provider-database glossary entry so ASN/GeoIP databases are
  distinguished from bogon sources that may also be public feeds.
- Updated SOW-0095 with the active-catalog downloader-options secret-handling
  behavior question found during same-failure review.

Validation performed:

- Targeted stale-claim scans over `docs/` and feed/glossary pages returned no
  matches for the repaired phrases.
- `git diff --check` passed for the touched documentation files, this SOW, and
  `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md`.
- No tests or scripts were run for this regression, per the user's review-only
  constraint and because the changes are documentation/SOW only.

Additional application-review follow-up:

- `configs/firehol/sources/malware_infrastructure/blueliv_crimeserver_last.yaml`
  still contains a bearer-token environment placeholder in
  `attributes.downloader_options`, while current code passes downloader options
  literally. This is mapped to
  `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md`
  for an explicit keep/change/deprecate decision.
