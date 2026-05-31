# SOW-0094 - Operator Docs And Specs Sync

## Status

Status: in-progress

Sub-state: operator-doc, GitHub Wiki publishing/navigation, approved TODO-history move, stale-reference cleanup, and spec-as-current-application repairs applied and validated; recorded application-review notes remain open.

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
- Source docs should keep normal repository-relative `.md` links so they render correctly in the code repository. The GitHub Wiki publishing workflow must flatten docs into wiki-root page filenames and rewrite local links to extensionless wiki page slugs. Direct wiki links with nested `.md` paths can redirect to raw markdown outside the rendered wiki.
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
- The GitHub Wiki publishing output uses wiki-compatible extensionless targets for local pages, while source docs keep repository-relative `.md` links.
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
- Suspected application issue for review: `ignore_repeating_download_errors` is an accepted runtime field with default `10` in `pkg/config/config.go:137` and `pkg/config/config.go:611`, is copied into runtime state in `pkg/engine/runtime.go:144` and defaulted again in `pkg/engine/runtime.go:189-190`, and is passed to `nextDue` from `pkg/scheduler/snapshot_build.go:56` and `pkg/scheduler/snapshot_build.go:105`. The `nextDue` parameter is not used in `pkg/scheduler/snapshot_build.go:174-234`; current retry timing is driven by failure count and health class through `failureRetryDelayMinutes` in `pkg/scheduler/snapshot_build.go:237-264`.

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
   - Working decision for this pass: A. The content was rewritten as catalog-operator material and no files were moved. A later path rename remains optional and requires explicit user approval.

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
- Added `scripts/build-wiki.mjs` and changed `.github/workflows/wiki-sync.yml` so wiki publishing flattens docs into wiki-root page filenames, rewrites local docs links to extensionless wiki slugs, and re-runs when the builder changes. This keeps source docs usable in the repository while preventing GitHub Wiki links from opening raw `.md` files.
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
  - Admin-auth docs now distinguish missing credentials (`503 Service Unavailable`) from wrong credentials (`401 Unauthorized`) and state that required auth without configured credentials prevents daemon startup.
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
- Rate-limit evidence checked against `pkg/web/middleware.go:74-87`.
- Home API query-parameter behavior checked against `pkg/web/home_api.go:12-55`.
- Config/feed docs checked against current public field names and scheduler evidence including `pkg/engine/public_catalog.go:41` and scheduler `FrequencyMinutes` use.

Tests or equivalent validation:

- `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test` passed and reported: `Built 86 wiki pages in /tmp/update-ipsets-wiki-test`.
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
- Internal-content scan over `docs/` returned no matches for `todo-history`, `docs/todo-history`, `SOW`, `.agents`, `pkg/`, `cmd/`, `pull request`, `submit`, `reviewers`, `developer documentation`, `implementation plan`, `TODO`, or `future work`. The only broad-pattern hits were YAML field examples for `source_contributor` and `forked`, which are catalog enum/status examples rather than contributor-workflow documentation.
- Filesystem validation passed: `docs/todo-history/` no longer exists and `.agents/sow/todo-history/` exists.
- Repository stale TODO-history path scan returned only the intentional `AGENTS.md` location note after excluding generated frontend assets and `node_modules`.
- Sensitive-name scan over changed durable artifacts found historical references in moved TODO-history files; the moved history and this SOW were sanitized to use `user` and `/home/user` before staging.
- Repeat post-wiki validation passed:
  - `node --check scripts/build-wiki.mjs`
  - `node scripts/build-wiki.mjs docs /tmp/update-ipsets-wiki-test`
  - local source-doc and generated-wiki link validator
  - generated-wiki scan for `.md` links
  - focused stale status/rate-limit/systemd scan for nonexistent `/api/v1/status.version`, independent search limiter wording, wrong drop-in path, and old status response fields
- `git diff --check -- AGENTS.md README.md docs .github/workflows/wiki-sync.yml scripts/build-wiki.mjs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md .agents/sow/todo-history pkg/insights/insight.go pkg/insights/insights.go pkg/config/config.go ui/src/components/feed-detail/hero.tsx ui/src/components/feed-detail/section-asn.tsx` passed.
- `git diff --check -- .agents/sow/specs .agents/sow/current/SOW-0094-20260531-operator-docs-sync.md` passed after spec edits.
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
- Full `make test` was not run because the code changes were limited to comments and one validation-error documentation pointer; targeted package tests were run for the touched Go packages. The focused architecture-posture guard was run because that spec records tool output.

Real-use evidence:

- The live GitHub Wiki showed the raw-markdown failure mode for nested `.md` wiki links and the rendered-page success mode for flat wiki slugs.
- GitHub Wiki compatibility was validated locally by generating the wiki destination and requiring every generated local link to resolve to a wiki-root Markdown page without `.md` or directory path segments.

Reviewer findings:

- No external reviewers were run; the user did not request external second-opinion agents.

Same-failure scan:

- Repeated route/flag/stale-term scans after repairs found no required remaining docs repairs in the audited operator-doc scope.
- Repeated spec scans after repairs found no remaining occurrences of the stale public critical-overlap `provider_set_id` rejection wording or stale provider-set content-hash/cardinality identity wording.
- Remaining application-review notes are recorded in this SOW: the general `/api/` limiter includes admin API routes, and `ignore_repeating_download_errors` is accepted/carried through runtime state but is not used by current scheduler retry timing.

Sensitive data gate:

- Passed for touched artifacts after sanitizing moved TODO-history files and this SOW. Examples use public documentation domains, reserved example addresses where applicable, placeholder secrets instead of real credentials, and `user` / `/home/user` for personal references.

Artifact maintenance gate:

- AGENTS.md: updated only to point preserved TODO history at `.agents/sow/todo-history/*.md`.
- `.gitignore`: updated to allow tracked moved TODO-history files under `.agents/sow/todo-history/`.
- Runtime project skills: no update needed for this pass; the current surface rules already covered the observed issue.
- Specs: updated under `.agents/sow/specs/` for current critical-overlap serving/integrity split, provider-set identity inputs, compose bounds, rate-limit middleware behavior, MCP tool surface, direct critical artifact serving, and architecture-posture measured highlights.
- End-user/operator docs and publishing: updated across `docs/` for operator purpose, current routes/flags/config fields, security rate-limit/auth/reload behavior, runtime settings, and repository-relative links; updated `.github/workflows/wiki-sync.yml` and `scripts/build-wiki.mjs` so the GitHub Wiki destination receives flattened pages with wiki-safe links.
- End-user/operator skills: none present or affected.
- SOW lifecycle: this SOW remains in `.agents/sow/current/` with `Status: in-progress`; it records review notes that require user/product decision before further code changes.

Specs update:

- `.agents/sow/specs/operating-principles.md`: updated to current cache-first critical-overlap serving and `/api/`/`/mcp` rate-limit behavior.
- `.agents/sow/specs/files-layout.md`: updated to current provider-set marker purpose and identity inputs.
- `.agents/sow/specs/pipeline.md`: updated to exclude materialized provider content from critical provider-set identity.
- `.agents/sow/specs/feeds.md`: updated to current public aggregate-serving behavior when `provider_set_id` drifts.
- `.agents/sow/specs/integrity.md`: clarified the difference between cleanup/API target eligibility and direct static artifact cache-first serving.
- `.agents/sow/specs/website.md`: added `/api/v1/compose` bounds and narrowed MCP to the current registered tools.
- `.agents/sow/specs/architecture-posture.md`: refreshed measured posture highlights from the current repository.

Project skills update:

- No update needed in this pass. Existing content-surface rules already required the separation that was applied.

End-user/operator docs update:

- Updated as recorded above.

End-user/operator skills update:

- No end-user/operator skills were present or affected.

Lessons:

- GitHub Wiki publishing should flatten docs into wiki-root page files and rewrite local links to extensionless wiki slugs. Source docs should keep repository-relative `.md` links; otherwise the repository docs view regresses while trying to fix the wiki.

Follow-up mapping:

- Review whether the current `/api/` prefix rate limiter should continue to include authenticated admin API routes.
- Review whether `ignore_repeating_download_errors` should be implemented in scheduler retry timing, deprecated, or removed from the documented/runtime configuration surface.

## Outcome

Operator-doc, wiki-publishing/navigation, approved TODO-history relocation, stale-reference cleanup, and spec-as-current-application repairs are implemented and validated. The SOW remains open for review of recorded application issues and for any final lifecycle/commit decision.

## Lessons Extracted

- Keep GitHub Wiki output links extensionless through `scripts/build-wiki.mjs`; keep source docs repository-relative with `.md` links.
- Keep internal design history outside `docs/`; preserved history now lives under `.agents/sow/todo-history/`.
- Specs must describe current application behavior as-is even when that behavior may need product review later; suspected issues belong in the SOW issue notes with evidence.

## Followup

- Review the recorded product/code questions before moving this SOW to `.agents/sow/done/`.

## Regression Log

None yet.
