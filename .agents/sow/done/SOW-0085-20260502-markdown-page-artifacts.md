# SOW-0085 | 2026-05-02 | markdown-page-artifacts

## Status

completed

Sub-state: 2026-05-16 regression closed. MCP feed markdown now uses the
configured canonical ASN/GEO providers, readable critical-provider labels, and
day-based retention tables. See `## Regression - 2026-05-16` at the end of
this file for the closeout narrative.

## Requirements

### Purpose

Enable AI assistants to consume structured feed/entity data as markdown pages — equivalent to the public web page but in a format optimized for LLM context windows. This is a prerequisite for SOW-0013 (MCP server) and SOW-0014 (AI feed quality evaluation).

### User Request

A templatized mechanism to generate markdown pages for feeds, countries, ASNs, and maintainers. The markdown mirrors the public web page structure (all sections, same order) but renders charts as tables with rollup logic. Total page capped to ~500-1000 rows. Templates are external files (editable without recompilation). Generated markdown is an artifact on disk, served cache-first.

### Assistant Understanding

Facts:

- The feed detail page has 11 sections + hero: IP Search, Insights, About, Critical Infrastructure, AS Composition, Geographic Coverage, Bogons, Behavior, Retention, Overlap/Comparison, Specs
- Each section has a corresponding pre-published JSON artifact in `web/` (e.g., `{name}_history.csv`, `{name}_asn_{provider}.json`)
- All public serving is cache-first from pre-published artifacts — no on-demand computation
- No `text/template` usage exists in the codebase today
- History CSV is already capped to last 500 data points
- Country, ASN, and maintainer pages have similar section structures with top-N lists
- The precomputed homepage aggregates pattern (SOW-0058) established: producer generates artifact → consumer reads from disk

Inferences:

- The markdown generator reads the same JSON artifacts the web UI consumes
- The data context struct for each entity type is the template's API contract
- On-demand generation (for compose/query MCP tools) uses the same templates + context, just populated from dynamic data
- External templates live in `configs/templates/markdown/` (part of the repo, editable, version-controlled)

Unknowns:

- Exact template formatting decisions (table style, section separators, heading levels) — resolved during implementation
- Whether insights should include methodology links (they reference public methodology pages) — include as inline text

### Acceptance Criteria

(Original criteria — note: after the 2026-05-13 regression, layout changed to entity-local; HTTP route was never wired up because MCP `fetch_analysis` is the only consumer.)

- `web/{name}.md` is published for every public feed with content including hero stats, insights, about, critical infrastructure, ASN top-50+other, GEO top-50+other, bogons, behavior time-series table (~50 rows), retention, comparison top-50+other, specs
- `web/countries/{code}.md` is published for every country with data
- `web/asns/{asn}.md` is published for every ASN with data
- `web/maintainers/{slug}.md` is published for every maintainer with feeds
- MCP `fetch_analysis(type, name)` returns the corresponding markdown via JSON-RPC for all four entity types
- Time-series rollup: try 1h → 1d → 1w → 1mo, first producing <100 rows wins; all time-series charts merged into a single table (each chart = a column)
- Non-time-series tables capped to 50 rows with "other" aggregation
- Total markdown page length ≤ 1000 rows for any feed
- Templates are external files editable without recompilation
- Markdown artifacts are generated during processing, committed to disk, served cache-first
- On-demand mode works: same template + context can be populated from dynamic data for compose/query use cases
- `make test`, `make lint`, `make race` all pass

## Analysis

Sources checked:

- `ui/src/pages/feed-detail.tsx` — page structure, section ordering
- `ui/src/components/feed-detail/section-*.tsx` — section implementations
- `pkg/web/routes.go` — API endpoints and data sources
- `pkg/engine/public_catalog.go` — `PublicFeedSummary` struct
- `pkg/engine/home_detail.go` — country/ASN/maintainer detail payloads
- `pkg/engine/home_aggregates.go` — precomputed aggregate pattern
- `.agents/sow/specs/pipeline.md` — artifact generation pipeline
- `.agents/sow/specs/files-layout.md` — published artifact paths

Current state:

- Feed page has 11 sections + hero, each backed by a JSON/CSV artifact
- History CSV: `timestamp,entries,ips` format, capped to 500 points
- Changesets: `timestamp,added,removed` format
- ASN/GEO: provider-based attribution, `{name}_{provider}.json`
- Country/ASN/maintainer: precomputed detail pages in `web/countries/`, `web/asns/`
- No template engine in use; all rendering is React client-side

Risks:

- Template complexity: 11 sections per feed means substantial template (600-800 lines per entity type)
- Data contract stability: templates depend on JSON artifact schemas; schema changes may break templates
- Performance: generating markdown for 500+ feeds every cycle could be expensive; mitigated by only regenerating changed feeds
- Size capping: aggressive capping may lose important detail for large feeds; mitigated by "other" aggregation preserving totals

## Implications And Decisions

1. **Generation timing**: Generate per-feed after each processing cycle (only changed feeds). Rationale: consistent with existing artifact generation, avoids unnecessary work.

2. **Template location**: `configs/templates/markdown/` — version-controlled, editable, loaded at startup. Rationale: matches `configs/` convention for YAML configs.

3. **Artifact location**: `{web-dir}/markdown/{entity_type}/{id}.md` — served alongside other web artifacts. Rationale: same directory, same cache, same serving path.

4. **Time-series rollup**: Try 1h → 1d → 1w → 1mo, first producing <100 rows wins. All time-series charts merged into one table with columns for each metric. Rationale: user-specified.

5. **Top-N capping**: 50 rows + "other" aggregation for all list/table data. Rationale: user-specified.

6. **On-demand mode**: Same `pkg/markdown/` package, same templates, context populated from dynamic data instead of pre-published artifacts. Output to temp directory with TTL. Rationale: code reuse, no duplication.

7. **Page target**: ≤ 1000 rows total. Rationale: user-specified cap for LLM context windows.

## Pre-Implementation Gate

### Problem/Root-Cause Model

No markdown artifact exists today. The web UI renders JSON data client-side in React. LLM-based tools (future MCP server, AI feed quality evaluation) need structured text pages equivalent to the web UI but optimized for context windows.

### Evidence Reviewed

- `pkg/engine/run_pipeline.go:321-345` — publish flow, BeforePublish hook, no AfterPublish
- `pkg/engine/entity_artifacts.go:384-654` — entity detail page generation
- `pkg/engine/home_aggregates.go:77-115` — precomputed aggregate pattern (reference)
- `pkg/engine/output.go:32-92` — setMetadata struct (feed metadata schema)
- `pkg/engine/insights.go:21-25` — insightsPayload struct
- `pkg/engine/asn.go:163-188` — asnFeedJSON struct
- `pkg/engine/geoloc.go:179-185` — GEO artifact shape
- `pkg/engine/bogons.go:128-146` — bogonFeedJSON struct
- `pkg/engine/critical.go:110-123` — criticalAggregateJSON struct
- `pkg/engine/engine.go:78-116` — CompareRow, RetentionData, RetentionSeries
- `pkg/engine/home_detail.go:18-144` — Country/ASN/Maintainer detail payloads
- `pkg/engine/public_series.go:34-68` — history/changeset CSV formats
- `pkg/web/cache.go:90-100` — ServeRootedFile pattern
- `pkg/web/routes.go:366-458` — route registration and SPA fallback
- `pkg/output/sync.go:18-22` — GeneratedFile struct
- `pkg/engine/web_batch.go:64-97` — applyGeneratedFileTimestamps

### Affected Contracts and Surfaces

- Pipeline: new artifact family (markdown files in staging dir)
- File layout: new `markdown/` subdirectory in web output
- Web serving: new route for `.md` files
- Integrity: new generated files registered in GeneratedFile ledger
- Specs: `files-layout.md` and `pipeline.md` need updates

### Existing Patterns to Reuse

- `stageHomeAggregates` pattern: compute → write to stageDir → return GeneratedFile
- `writeJSONFile` helper for staging dir writes
- `ServeRootedFile` for cache-first public serving
- `output.GeneratedFile` for mtime tracking
- Go stdlib `text/template` (new usage, no third-party dependency)

### Architecture Decision

Generate markdown during the pipeline, write to staging dir, publish atomically with other artifacts. NOT post-publish.

Reason: keeps markdown consistent with all other artifacts (same staging→publish flow, same mtime tracking, same integrity contract). Post-publish would lag one cycle and need separate staging.

The markdown generator reads from the staging dir (same dir where ASN/GEO/bogon/critical/metadata JSON was written by earlier phases). The files are in the OS page cache so I/O is negligible.

### Risk and Blast Radius

- Low risk: new artifact generation only; existing artifacts and serving paths untouched
- Blast radius: zero — missing/stale markdown does not break any existing functionality
- Performance: reading ~10 JSON files per feed from staging dir (OS page cache hot) is negligible

### Sensitive Data Handling Plan

Markdown pages contain IP counts, ASN names, country names, feed metadata — all already public via JSON API. No credentials, tokens, private IPs, or personal data. Templates are version-controlled in `configs/templates/markdown/`.

### Validation Plan

- Table-driven tests for rollup logic (1h/1d/1w/1mo, edge cases)
- Table-driven tests for capping logic (top-N, "other" aggregation)
- Golden file tests for template output
- `make test`, `make lint`, `make race`

### Artifact Impact Plan

- Specs: update `files-layout.md` (add markdown dir), `pipeline.md` (add markdown step)
- No AGENTS.md changes needed
- No project skill changes needed
- No operator doc changes needed (markdown is internal artifact)

### Open Decisions

None — all decisions recorded in "Implications And Decisions" section.

## Plan

### Chunk 1: Core infrastructure

Scope: `pkg/markdown/` package with template loading, data context structs, rollup logic, capping logic.

- `pkg/markdown/context.go` — `FeedPageContext`, `CountryPageContext`, `ASNPageContext`, `MaintainerPageContext` structs
- `pkg/markdown/rollup.go` — time-series rollup: load points, try intervals (1h/1d/1w/1mo), aggregate, return table rows
- `pkg/markdown/cap.go` — top-N capping with "other" aggregation for any `[{name, value}]` slice
- `pkg/markdown/generate.go` — template loading (`configs/templates/markdown/*.tmpl`), `ExecuteTemplate`, write to disk
- `pkg/markdown/context_feed.go` — populate `FeedPageContext` from pre-published artifacts (read JSON files)
- Tests for rollup and capping logic

### Chunk 2: Feed page template

Scope: `configs/templates/markdown/feed.md.tmpl` + integration into processing pipeline.

- Template with all 11 sections + hero
- Integration: call markdown generator after feed artifacts are published (in the processing loop's publish phase)
- Route: `GET /markdown/feeds/{name}.md` serving from `{web-dir}/markdown/feeds/`
- Manual verification against real feed data

### Chunk 3: Country, ASN, maintainer templates

Scope: entity templates + integration.

- `configs/templates/markdown/country.md.tmpl`
- `configs/templates/markdown/asn.md.tmpl`
- `configs/templates/markdown/maintainer.md.tmpl`
- Context population for each entity type
- Routes for each entity type
- Integration into processing pipeline

### Chunk 4: On-demand mode

Scope: compose/query support.

- `pkg/markdown/generate.go` — on-demand variant that accepts dynamic context
- Temp directory management with TTL
- Route for on-demand generation (used by future MCP tools)

## Execution Log

### 2026-05-02

- SOW created, user decisions recorded

## Validation

(Backfilled 2026-05-13 after regression reopening.)

Acceptance criteria evidence:

- Feed markdown: `/opt/update-ipsets/web/abuseipdb_1d.md` exists at 39,879 bytes after one reprocess; contains hero, insights, about, critical, ASN, GEO, behavior, retention, comparison, specs sections per template.
- Country markdown: 243 files under `/opt/update-ipsets/web/countries/*.md` (sample: `US.md` at 507,232 bytes).
- ASN markdown: 53,350 files under `/opt/update-ipsets/web/asns/*.md` (sample: `13335.md` at 14,663 bytes).
- Maintainer markdown: 85 files under `/opt/update-ipsets/web/maintainers/*.md` (sample: `firehol.md` at 487 bytes).
- Old `web/markdown/...` subtree absent — `ls /opt/update-ipsets/web/markdown` reports "No such file or directory".

Tests or equivalent validation:

- `make build`, `make test`, `make race`, `make lint` — all pass post-change.
- `pkg/mcp/server_test.go` adds `TestHandleFetchAnalysisEntityLayout` covering all four entity-local paths.
- `go test ./pkg/mcp/... ./pkg/engine/... ./pkg/markdown/...` — all green.

Real-use evidence:

- `journalctl -u update-ipsets` after restart: `INFO markdown templates loaded count=4 dir=/opt/update-ipsets/etc/config/templates/markdown` (previously logged "markdown template directory not found" on every restart since 2026-05-10).
- After manual reprocess `POST /api/v1/admin/feeds/abuseipdb_1d/reprocess`: logs show `markdown generated feed=abuseipdb_1d path=abuseipdb_1d.md` followed by `markdown pages generated feeds=1`.
- MCP end-to-end via direct JSON-RPC POST to `/mcp` after `initialize` + `notifications/initialized`:
  - `fetch_analysis(feed, abuseipdb_1d)` → 39,879-byte markdown, first line `# abuseipdb_1d`
  - `fetch_analysis(country, US)` → 507,232-byte markdown, first line `# United States`
  - `fetch_analysis(asn, 13335)` → 14,663-byte markdown, first line `# AS13335 (CLOUDFLARENET)`
  - `fetch_analysis(maintainer, firehol)` → 487-byte markdown, first line `# FireHOL`
  - `fetch_analysis(feed, nonexistent_xyz)` → `isError=true`, body `feed "nonexistent_xyz" not found`

Reviewer findings:

- None requested; this regression scope is narrow (deployment bug + internal path refactor) with full end-to-end validation captured above.

Same-failure scan:

- Searched for other `templates/` directories install.sh might gate behind an unrelated diff: only `configs/templates/markdown/` exists. No other engine-side "directory not found" silent disables found in `pkg/engine` (`grep -n 'directory not found' pkg/engine/`).
- Searched for any remaining `web/markdown/` references: `grep -rn 'markdown/feeds\|markdown/countries\|markdown/asns\|markdown/maintainers' --include='*.go' --include='*.tmpl'` → no matches outside SOW history sections.

Sensitive data gate:

- No raw secrets, credentials, customer data, or private endpoints in this SOW, the spec update, or generated markdown. Markdown contains public feed metadata only (IP counts, ASN names, country names — all already public via JSON API).

Artifact maintenance gate:

| Artifact class | Updated | Reason |
|---|---|---|
| `AGENTS.md` | No | No workflow/responsibility changes; install.sh and layout changes covered by spec |
| Runtime project skills | No | No new working patterns; existing `project-operations` skill already covers install.sh/restart flow |
| Specs: `.agents/sow/specs/files-layout.md` | Yes | "Markdown page artifacts" section rewritten with entity-local paths |
| Specs: other | No | No changes to pipeline.md, integrity.md, website.md needed |
| End-user/operator docs | No | Markdown is an internal artifact consumed by MCP; existing `docs/api/mcp-endpoint.md` already documents `fetch_analysis` without reference to on-disk paths |
| End-user/operator skills | No | No output/reference skills consume markdown paths |
| SOW lifecycle | Yes | Status returns to `completed`; SOW moves to `.agents/sow/done/` in the same commit as the work |

Specs update:

- `.agents/sow/specs/files-layout.md:550-578` — rewrote "Markdown page artifacts" section. Paths now entity-local. Added explicit note that the old `web/markdown/...` subtree has been removed.

Project skills update:

- None needed. The `project-operations` skill at `.agents/skills/project-operations/SKILL.md` already mandates `./install.sh` + `systemctl restart update-ipsets` as the local deploy path.

End-user/operator docs update:

- None needed. `docs/api/mcp-endpoint.md` describes `fetch_analysis(type, name)` without prescribing on-disk artifact paths.

End-user/operator skills update:

- None needed.

Lessons:

1. **A "diff-gated" install block must own everything it installs.** `install.sh` short-circuited its config update when `configs/firehol/` matched the deployed copy, but the markdown-templates install lived inside the same `else` branch and was dropped silently. Anything inside a conditional install block must either be inside the same conditional's scope of comparison or moved out entirely with its own idempotent guard. The fix moves the template install out of the `else` and gives it its own diff check.
2. **Layout uniformity is not free — locality matters when entities have different homes.** SOW-0085 chose `web/markdown/{type}/{name}.md` for one-rule MCP path resolution. But feeds, countries, ASNs, and maintainers already live in different shapes inside `web/`, so the uniform tree lost locality for feeds without gaining anything users observe. When entity types have heterogeneous existing layouts, mirror the existing layout instead of imposing a new one.
3. **"Pending" validation is not validation.** SOW-0085 closed with the entire `## Validation` and `## Outcome` sections as "Pending". The install.sh bug existed in commit `123e8a1` (2026-05-02) and was masked for 11 days because nobody ran end-to-end. SOW closure must produce concrete evidence (file paths, output bytes, log lines, command output) — placeholder "Pending" wording is a process failure that breaks the artifact maintenance gate.
4. **Silent "feature off" should be discoverable.** The engine logs `markdown template directory not found` at DEBUG level only, so even with the templates missing for 11 days, no INFO/WARN appeared and no admin-API state surfaced the disabled feature. Pre-existing behavior, but worth a follow-up: missing templates when the subsystem is expected to run should be at least WARN, or visible through admin-API/UI surface per the project's "Background work must be visible through the admin API/UI" rule. Tracked as follow-up below.

Follow-up mapping:

- (new) **Surface markdown-disabled state in admin API/UI.** Today the engine logs at DEBUG when the template dir is missing and silently disables generation. Per the project's "Background work must be visible through the admin API/UI" rule, this should at least emit a WARN and ideally show up in admin status. **Not implemented in this SOW** — out of scope for the regression fix and tracked as `.agents/sow/pending/SOW-0087-20260516-markdown-disabled-admin-visibility.md`.
- Original SOW-0013 (MCP server, completed) — `fetch_analysis` is now confirmed working end-to-end for all four entity types.
- Original SOW-0014 (AI in the loop, pending) — no longer blocked on markdown artifact availability; remains paused on user's design discussion.

## Outcome

Original SOW-0085 (2026-05-02) delivered the markdown subsystem in code: `pkg/markdown/` package, four external templates in `configs/templates/markdown/`, per-feed and per-entity write paths, MCP integration. The layout chosen was `web/markdown/{entity_type}/{id}.md` for uniform MCP path resolution.

Closure on 2026-05-02 was premature: the `## Validation` section was left "Pending", so end-to-end never ran. This masked an install.sh gating bug that prevented templates from ever deploying after the first install. As of 2026-05-13, no markdown had been generated on the local workstation despite the feature being merged 11 days earlier.

Regression on 2026-05-13 fixed two things:

1. **Install.sh template-install gating bug**: moved the markdown-templates install out of the `else` branch of the config-diff check, gave it its own idempotent diff guard. Templates now install on every reinstall regardless of whether `configs/firehol/` changed.
2. **Layout revision (D1=A)**: markdown moved from `web/markdown/{entity_type}/{id}.md` to entity-local paths: `web/{feed}.md`, `web/countries/{CODE}.md`, `web/asns/{ASN}.md`, `web/maintainers/{slug}.md`. The MCP `fetch_analysis` resolver now branches per entity type. Old `web/markdown/` paths fully removed; no aliases.

End-to-end validated: install.sh fix exercised on workstation; daemon restarted; logs show templates loaded; manual reprocess produced feed markdown at the new path; MCP `fetch_analysis` confirmed for all four entity types via direct JSON-RPC. Spec at `.agents/sow/specs/files-layout.md` updated.

## Lessons Extracted

See "Lessons" entries 1-4 in the Validation section above.

## Followup

- SOW-0013 (MCP server) — consumes markdown artifacts for tools 1-4
- SOW-0014 (AI in the loop) — uses markdown pages for feed quality evaluation

## Regression Log

- 2026-05-13 — reopened. See section below.

## Regression - 2026-05-13

### Trigger

While answering a status question on markdown generation and MCP, two problems surfaced:

1. **Templates were never deployed on this workstation.** `journalctl -u update-ipsets` logs `markdown template directory not found dir=/opt/update-ipsets/etc/config/templates/markdown` on every restart since 2026-05-10 20:58 (last deploy). `/opt/update-ipsets/etc/config/templates/` does not exist. `/opt/update-ipsets/web/markdown/` does not exist. The feature has never run end-to-end on this host.

2. **User asked why markdown lives in a separate `web/markdown/...` subtree** instead of next to each entity's other artifacts. The original SOW's stated rationale (decision #3, line 91 — "same directory, same cache, same serving path") does not actually justify a separate subdirectory. The real driver was layout uniformity across feeds/countries/ASNs/maintainers, but locality was traded away.

### Root Causes

**Issue 1 — install.sh template-install gating bug.**

`install.sh:168` short-circuits when `diff -qr configs/firehol "${CONFIG_TARGET}"` reports no difference:

```bash
if [ -d "${CONFIG_TARGET}" ] && diff -qr configs/firehol "${CONFIG_TARGET}" >/dev/null; then
    echo -e "${GREEN}Active configuration already up to date.${NC}"
else
    ...
    # lines 186-189: markdown template install
    MARKDOWN_TEMPLATES="${CONFIG_TARGET}/templates/markdown"
    run sudo mkdir -p "${MARKDOWN_TEMPLATES}"
    run sudo cp -a --no-preserve=ownership configs/templates/markdown/. "${MARKDOWN_TEMPLATES}/"
fi
```

The markdown-templates install at lines 186-189 lives inside the `else` branch. When the active config is up-to-date, the entire `else` is skipped and the templates are never copied. The diff check looks at `configs/firehol` only; it never observes `configs/templates/markdown/`. After commit `123e8a1` (2026-05-02) added the SOW-0085 markdown subsystem, subsequent reinstalls (including the 2026-05-10 20:58 reinstall) silently dropped the template install.

The engine at `pkg/engine/markdown.go:18-21` then logs DEBUG `markdown template directory not found` and silently disables generation, which is by design — but combined with the install bug this masks the failure entirely. No INFO/WARN is emitted, no admin-API state surfaces, nothing in the public surface signals the feature is off.

**Issue 2 — layout decision needs user review.**

Original SOW chose `web/markdown/{entity_type}/{id}.md` for one-rule MCP resolution. User raised the locality concern: a feed's `.md` is not next to its `.json`, `_history.csv`, `_comparison.json`, etc. Operators listing `web/` for a feed don't see its markdown.

### Validation Gate That Was Skipped

The original SOW closed with `## Validation` and `## Outcome` entirely "Pending" (lines 225-285 of the original close). The artifact-maintenance gate and same-failure scan were not filled in. End-to-end never ran. Per `AGENTS.md` SOW completion rules, this should not have been marked `completed`. This reopening backfills that gate.

### User Decisions Required

**D1. Markdown artifact layout** (recommended: Option A)

**Option A — Entity-local placement** (recommended)
- `web/{feed}.md` (next to `web/{feed}.json`, `web/{feed}_history.csv`, etc.)
- `web/countries/{CODE}.md` (next to `web/countries/{CODE}.json`)
- `web/asns/{ASN}.md` (next to `web/asns/{ASN}.json`)
- `web/maintainers/{slug}.md` (new subtree; maintainers have no JSON sibling today — `/api/v1/maintainers/{slug}` is computed at request time)

Pros: locality — operators see all of a feed's artifacts in one listing. Matches the spec convention of feed-prefixed files at the top level of `web/`.

Cons: MCP `fetch_analysis` needs four path-resolution rules instead of one (mechanically small — ~20 lines in `pkg/mcp/fetch_analysis.go`).

Risk: low. No public HTTP route serves markdown directly today — the MCP tool is the only consumer. The redistribution mirror at `web/files/` is unaffected.

**Option B — Keep current uniform tree**
- `web/markdown/feeds/{name}.md`
- `web/markdown/countries/{CODE}.md`
- `web/markdown/asns/AS{ASN}.md`
- `web/markdown/maintainers/{slug}.md`

Pros: zero code/spec churn beyond the install.sh fix. Single MCP resolution rule.

Cons: locality lost (the original complaint).

Recommendation: **Option A**. The user explicitly raised locality; the SOW-0085 rationale for the uniform tree was weak; migration is internal-only.

**D2. ASN file naming under Option A**

1. `web/asns/{ASN}.md` — matches existing JSON naming `web/asns/{ASN}.json` (no `AS` prefix).
2. `web/asns/AS{ASN}.md` — matches the old `web/markdown/asns/AS{ASN}.md` naming.

Recommendation: **1**. Rationale: symmetry with the JSON sibling that already exists.

**D3. Old `web/markdown/...` paths**

1. Drop entirely. No public route serves them; the only consumer (MCP `fetch_analysis`) is updated atomically in the same commit.
2. Keep as symlinks/aliases for a transition window.

Recommendation: **1**. Rationale: no external consumers; symlinks add operational complexity for no benefit.

**D4. Install.sh fix shape** (no design choice — confirming the approach)

Move the markdown-templates install OUT of the `else` branch so it runs on every reinstall, and make it idempotent (compare repo `configs/templates/markdown/` to the deployed copy; replace only when content differs, preserving mtime when identical — same pattern the existing config block uses to avoid spurious entity-integrity rebuilds).

### Decisions (approved by user 2026-05-13)

- **D1 = A**: entity-local layout (`web/{feed}.md`, `web/countries/{CODE}.md`, `web/asns/{ASN}.md`, `web/maintainers/{slug}.md`).
- **D2 = 1**: ASN naming `web/asns/{ASN}.md` (no `AS` prefix; matches existing JSON).
- **D3 = 1**: drop old `web/markdown/...` paths entirely; no aliases.
- **D4 = ok**: install.sh fix as described — unconditional, idempotent, preserves mtime when identical.

### Implementation Plan (gated on D1-D3)

1. **Fix `install.sh`** — make template install unconditional and idempotent. Run `./install.sh && sudo systemctl restart update-ipsets`. Verify `journalctl -u update-ipsets -n 50` no longer logs `markdown template directory not found`. Verify `/opt/update-ipsets/etc/config/templates/markdown/*.tmpl` exists.

2. **(D1=A)** Update markdown layout:
   - `pkg/engine/markdown.go:67-81` — change `publicFeedMarkdownRelPath`, `publicCountryMarkdownRelPath`, `publicASNMarkdownRelPath`, `publicMaintainerMarkdownRelPath`
   - `pkg/mcp/fetch_analysis.go:13-49` — replace the `validEntityTypes` map with per-type path resolution
   - `pkg/mcp/server_test.go` — update test fixtures
   - `.agents/sow/specs/files-layout.md:550-577` — rewrite the "Markdown page artifacts" section with new entity-local paths
   - Search for any other reference to the old `web/markdown/` paths (`grep -rn 'markdown/feeds\|markdown/countries\|markdown/asns\|markdown/maintainers' --include='*.go' --include='*.md'`) and update.

3. **End-to-end validation**:
   - Trigger one full pipeline cycle (via admin API or wait for scheduler)
   - Verify markdown files appear at the new paths: `ls /opt/update-ipsets/web/{feed}.md`, `ls /opt/update-ipsets/web/countries/AD.md`, `ls /opt/update-ipsets/web/asns/13335.md`, `ls /opt/update-ipsets/web/maintainers/firehol.md`
   - Test MCP `fetch_analysis` for each entity type via direct JSON-RPC POST to `/mcp`
   - Verify response is non-empty markdown for each type

4. **Backfill `## Validation` section** with actual evidence: acceptance criteria with file paths, real-use evidence from MCP calls, sensitive-data gate confirmation, artifact-maintenance gate (specs updated, no docs changes needed since markdown is internal), same-failure scan (look for other "directory not found" silent disables in the engine).

5. **Single commit** containing: install.sh fix + (D1=A: layout change + spec update + test fixtures) + validation backfill + `Status: completed` + move SOW from `current/` to `done/`.

### Risk And Blast Radius

- Install.sh fix: low. Idempotent; only adds a missing copy.
- Layout change: low. Internal-only; no public route serves markdown today.
- End-to-end pipeline cycle: medium. First time markdown generation actually runs on this host. The original SOW's "Pending" validation hid any latent bugs in the per-entity write paths (`pkg/engine/entity_artifacts.go`, `entity_surgical.go`, `entity_artifact_selected_repair.go`). These may need fixes once exercised.

### Sensitive Data Handling

No sensitive data involved. Templates and generated markdown contain only public feed metadata, IP counts, country/ASN names — all already public via the JSON API. Templates are version-controlled under `configs/templates/markdown/`. No credentials or tokens.

### Affected Contracts And Surfaces

- `install.sh` — deployment script
- `pkg/engine/markdown.go` — feed markdown path resolver
- `pkg/engine/markdown_entity.go` — country/ASN/maintainer staging
- `pkg/mcp/fetch_analysis.go` — MCP path resolution
- `pkg/mcp/server_test.go` — MCP tests
- `.agents/sow/specs/files-layout.md` — normative spec
- `pkg/engine/run_pipeline.go`, `pkg/engine/entity_artifacts.go`, `pkg/engine/entity_surgical.go`, `pkg/engine/entity_artifact_selected_repair.go` — call sites that pass relative paths (unchanged, they call the helpers in `markdown.go`)

### Followup

This regression reopens the validation gate that should have been closed in 2026-05-02. No downstream SOW (SOW-0013 MCP, SOW-0014 AI-in-the-loop) is blocked by the layout decision itself — MCP is already deployed and works for `find_feeds`; `fetch_analysis` will start working as soon as templates install and one pipeline cycle runs.

## Regression - 2026-05-16

### Status

Status: completed

Sub-state: Reopened because MCP `fetch_analysis(feed, ...)` markdown is too
verbose and leaks raw provider metadata in the critical-infrastructure table.

### Requirements

#### Purpose

Make feed markdown returned by MCP `fetch_analysis` compact and caller-friendly
while preserving the public artifact contract: generated once during processing,
served cache-first, and never recomputed on MCP requests.

#### User Request

The user provided a `fetch_analysis` response for `blocklist_de` and requested:

- ASN composition should keep only `iptoasn`.
- Geographic coverage should keep only `dbip`.
- Retention and age tables should aggregate by day instead of hour.
- Retention and age tables should represent the first 365 days, skip zero-count
  days with an explicit note that missing days are zero, and roll later buckets
  into `>365 days`.
- Overlap with other feeds should include `This %` and `Their %` columns, and
  the markdown should explain each denominator.
- `iblocklist_bogons` should not be categorized or used as a bogon reference
  provider because the stale iBlocklist source makes unrelated feeds appear to
  overlap bogon space.
- The Technical Specifications table should not render blank `Format` or update
  timing rows.

#### Assistant Understanding

Facts:

- MCP `fetch_analysis` reads pre-generated markdown via `pkg/mcp`, it does not
  generate markdown during the request.
- Feed markdown is generated from `configs/templates/markdown/feed.md.tmpl` and
  context built by `pkg/markdown/context_feed.go`.
- The current template renders all ASN provider artifacts and all GeoIP provider
  artifacts that exist for a feed.
- The project has configured provider defaults: `defaults.asn_provider:
  iptoasn` and `defaults.geo_provider: dbip_country` in
  `configs/firehol/defaults.yaml`.
- `configs/firehol/sources/special_use/iblocklist_bogons.yaml` carried
  `use: [bogons]`, which made it a reference provider for bogon overlap.
- The raw `map[...]` string in the user-provided critical provider table comes
  from rendering a JSON object through `strVal(m["provider"])`.
- Retention JSON stores hourly buckets as `hours[]` and `ips[]`.
- Per-feed metadata emits update timing fields as `average_update`,
  `min_update`, and `max_update`; the markdown reader was looking for
  `avg_update_mins`, `min_update_mins`, and `max_update_mins`.
- Normal ipset/netset feeds have configured `output:` values. They usually do
  not have specialized `format:` values, so a template row that only reads
  `format` can be blank.

Inferences:

- Feed markdown should use the configured default ASN and GeoIP providers when
  rendering the single canonical provider view for MCP.
- `dbip` in the user request maps to the configured GeoIP default
  `dbip_country`.
- Retention data should remain stored hourly in JSON for existing APIs; only the
  markdown context/table should roll it up to daily buckets.
- The catalog fix for `iblocklist_bogons` should remove the bogon role and
  health exemption while leaving it as a normal public feed.

Unknowns:

- None blocking. The current request gives direct output defects and target
  behavior.

#### Acceptance Criteria

- Feed markdown renders only the configured default ASN provider, which is
  `iptoasn` in the current catalog.
- Feed markdown renders only the configured default GeoIP provider, which is
  `dbip_country` in the current catalog.
- Critical provider tables render a readable provider label or name, never raw
  Go `map[...]` output.
- Retention current and past series render day buckets, not hour buckets.
- Retention current and past series list non-zero days within days 1-365,
  include a note that missing days are zero, and include one `>365 days` row for
  all later buckets when later buckets exist.
- Overlap tables include `This %`, common IPs divided by total IPs in the
  current feed, and `Their %`, common IPs divided by total IPs in the row feed.
- Overlap tables explain `This %` and `Their %` next to the table.
- `iblocklist_bogons` remains a normal feed and no longer carries
  `use: [bogons]` or `exclude_from_unmaintained`.
- Technical Specifications renders output-backed `Format` for normal feeds,
  uses generated update timing metadata, and omits those rows when unknown.
- Specs record the MCP markdown content contract.
- Tests cover provider filtering, provider-label rendering, and retention daily
  rollup.
- Tests cover overlap percentage columns and explanations.

### Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The MCP response is noisy because feed markdown reused the full website feed
  artifact context instead of a compact canonical-provider markdown contract.
- ASN context uses a glob over every `{feed}_asn_*.json` artifact, so every ASN
  provider appears.
- Geo context iterates every metadata `geo` artifact entry, so every GeoIP
  provider appears.
- Critical provider context treats the `provider` JSON object as a scalar string,
  causing Go's map formatting to leak into markdown.
- Retention context passes hourly retention buckets straight through to the
  template, producing extremely long tables.

Evidence reviewed:

- User-provided `fetch_analysis` response shows four ASN providers, five GeoIP
  providers, raw `map[info:...]` provider cells, and hour-based retention rows.
- `pkg/mcp/fetch_analysis.go` reads markdown from disk; it is not the content
  generator.
- `pkg/engine/markdown.go` creates `markdown.NewFeedArtifactReader(outDir)`
  during processing.
- `pkg/markdown/context_feed.go` builds ASN, GeoIP, critical, and retention
  feed context.
- `configs/templates/markdown/feed.md.tmpl` renders all context rows it
  receives.
- `configs/firehol/defaults.yaml` configures `iptoasn` and `dbip_country` as
  canonical providers.
- `.agents/sow/specs/config.md` and `.agents/sow/specs/website.md` require
  configured ASN/GeoIP defaults for canonical feed-detail behavior.
- `configs/firehol/sources/special_use/iblocklist_bogons.yaml` showed the bad
  `use: [bogons]` role and age-health exemption.
- `pkg/engine/output.go` writes `average_update`, `min_update`, and
  `max_update`; `pkg/markdown/context_feed.go` read mismatched field names.

Affected contracts and surfaces:

- Generated feed markdown artifacts consumed by MCP `fetch_analysis`.
- Markdown template/context package.
- Engine markdown generation wiring.
- MCP/operator docs only if endpoint behavior wording changes; expected no API
  schema change.
- Specs for markdown artifact content.
- Tests in `pkg/markdown` and focused package validation.

Existing patterns to reuse:

- `Engine.preferredASNProvider()` and `Engine.preferredGeoProvider()` already
  implement configured provider-default selection.
- `FeedArtifactReader` already centralizes feed markdown context construction.
- Existing external `pkg/markdown` template tests validate rendered markdown
  output through the public template store API.

Risk and blast radius:

- Low request-path risk: MCP remains cache-first and reads existing markdown
  files only.
- Medium artifact-content risk: generated feed markdown becomes intentionally
  shorter and no longer includes secondary ASN/GeoIP providers.
- Compatibility risk: callers expecting all providers in markdown should use the
  JSON/API provider routes instead. The user explicitly requested a single
  canonical provider in MCP markdown.
- Performance risk: reduced output size and less context construction work.
- Data risk: no raw feed bodies or secrets are touched.

Sensitive data handling plan:

- Durable artifacts will contain only public feed names, provider names, file
  paths, enum-free behavior descriptions, and redacted/generalized user output
  evidence. No credentials, bearer tokens, SNMP communities, customer data,
  private endpoints, personal data, or proprietary incident details are needed.

Implementation plan:

1. Extend `FeedArtifactReader` with optional preferred ASN and GeoIP provider
   configuration.
2. Pass engine configured provider defaults into the markdown reader during feed
   markdown generation.
3. Filter ASN and GeoIP context to the preferred provider when configured.
4. Render critical provider labels from the provider object instead of raw map
   formatting.
5. Roll retention hourly buckets into day buckets for markdown: non-zero days
   within days 1-365 plus `>365 days` for later buckets, with a template note
   that missing days are zero.
6. Update feed markdown template labels from hours to days.
7. Add focused markdown tests for provider filtering, critical provider labels,
   and daily retention rollup.
8. Update specs for the MCP markdown content contract.
9. Add overlap percentage columns and denominator explanations to feed markdown.
10. Remove the bogon-reference role from `iblocklist_bogons` and pin that in
    catalog validation.
11. Populate feed markdown technical-spec format from output when specialized
    format is absent, and read the correct generated update timing fields.

Validation plan:

- Run `go test ./pkg/markdown ./pkg/engine ./pkg/mcp`.
- Run a focused same-failure search for `map[` rendering and stale `Age
  (hours)` markdown template text.
- Run `git diff --check` on changed paths.
- If committing from a dirty worktree, validate the staged tree separately.

Artifact impact plan:

- AGENTS.md: no update expected; no workflow or project-wide guardrail changes.
- Runtime project skills: no update expected unless validation finds a durable
  markdown-specific working rule.
- Specs: update `.agents/sow/specs/files-layout.md` and/or
  `.agents/sow/specs/website.md` for canonical provider and retention rollup
  behavior.
- End-user/operator docs: no update expected unless API docs need wording; MCP
  schema is unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: reopen SOW-0085 from `done` to `current`; move back to `done`
  with `Status: completed` after validation and commit with the fix.

Open-source reference evidence:

- None. This is a project-local markdown artifact contract change; no external
  implementation reference is needed.

Open decisions:

- None. The user directly specified the desired behavior. Provider selection
  will reuse configured defaults so `iptoasn` and `dbip_country` remain catalog
  policy, not hardcoded markdown policy.

### Execution Log

- Reopened SOW-0085 from `.agents/sow/done/` to `.agents/sow/current/`.
- Added preferred-provider options to `pkg/markdown.FeedArtifactReader`.
- Wired engine configured defaults into feed markdown generation:
  `iptoasn` for ASN and `dbip_country` for GeoIP on this catalog.
- Changed feed markdown context construction to render only the preferred ASN
  and GeoIP providers when configured.
- Changed critical provider rendering to use provider object `label`, `name`,
  or `maintainer` instead of raw Go map formatting.
- Rolled retention hourly buckets into day buckets for markdown: non-zero days
  within days 1-365 plus `>365 days` for later data.
- Added the feed markdown retention note that omitted days have zero IP count.
- Added `This %` and `Their %` overlap columns and in-markdown denominator
  explanations.
- Removed `use: [bogons]` and the unmaintained-health exemption from
  `iblocklist_bogons` so it no longer participates as a bogon reference
  provider.
- Added generated metadata `output` to per-feed JSON so feed markdown can render
  an output-backed `Format` row for normal feeds.
- Fixed feed markdown update timing extraction to read `average_update`,
  `min_update`, and `max_update`.
- Changed the Technical Specifications template to omit unknown format/update
  rows rather than rendering blank values.
- Renamed the Behavior section's rollup label from `Interval` to
  `Table bucket` and added explanatory text so readers do not confuse the
  history aggregation bucket with observed feed cadence.
- Updated the feed markdown template from hour labels to day labels.
- Added focused `pkg/markdown` behavioral tests for provider filtering,
  critical provider labels, daily retention rollup, and overlap percentage
  columns.
- Added catalog and metadata tests for the `iblocklist_bogons` role contract
  and output-backed technical format.
- Updated specs and MCP docs for the compact feed markdown contract.
- Installed locally with `./install.sh`; systemd restarted the daemon.
- Rebuilt all feed markdown artifacts by scheduling a full admin reprocess with
  `POST /api/v1/admin/run?reprocess=true`.
- Mapped the pre-existing markdown-disabled admin visibility follow-up to
  `.agents/sow/pending/SOW-0087-20260516-markdown-disabled-admin-visibility.md`.

### Validation

Code and template validation:

- `go test ./pkg/config ./pkg/markdown ./pkg/engine ./pkg/mcp` passed.
- `git diff --check` passed for the touched source, template, docs, spec, and
  SOW files.
- Same-failure scan for `Age (hours)` and raw markdown `| map[` output found
  only the negative assertion in `pkg/markdown/feed_template_test.go`; no
  template/spec/doc output path still emits those strings.
- Stale route scan for `markdown/feeds`, `markdown/countries`,
  `markdown/asns`, `markdown/maintainers`, and `{web-dir}/markdown` found no
  active spec/doc/code route references.

Install validation:

- `./install.sh` completed successfully.
- Service logs after restart showed:
  - `markdown templates loaded count=4 dir=/opt/update-ipsets/etc/config/templates/markdown`
  - `integrity check passed — all feeds have up-to-date and readable secondary files`
  - `update-ipsets daemon listening` on the public listener.
- Local health check returned `ok` from `http://localhost:18888/healthz`.

All-feed markdown rebuild validation:

- Admin reprocess request returned
  `{"recheck":"false","reprocess":"true","status":"scheduled"}`.
- Logs for the final broad manual reprocess showed
  `markdown pages generated feeds=391`, followed by
  `run finished updated=400 skipped=0 failed=0 elapsed=2m29.676s` and
  `processing batch completed updated=400 skipped=0 failed=0`.
- A follow-on scheduled run generated another 2 changed feed markdown files and
  finished with `updated=2 skipped=0 failed=0`.
- `/opt/update-ipsets/web` contains 391 top-level feed markdown files after the
  rebuild.
- Subsequent scheduler work continued normally; the admin status endpoint showed
  the engine not running after the later scheduled cycle completed.

Sample artifact and MCP validation:

- `/opt/update-ipsets/web/blocklist_de.md` was regenerated after the install.
- The sample file contains `### iptoasn` and does not contain the other ASN
  provider sections previously shown in the user report.
- The sample file contains `### dbip_country` and does not contain secondary
  GeoIP provider sections.
- The sample file contains `Age (days)`, does not contain `Age (hours)`, states
  that missing days have zero IP count, and includes a `>365 days` row when
  later retention data exists.
- The sample file includes `This %` and `Their %` columns in Overlap with Other
  Feeds, with the requested denominator explanations.
- The sample file labels the Behavior section aggregation as `Table bucket` and
  explains that Update Cadence is the observed feed-change interval
  distribution.
- The sample file does not contain `map[` in critical provider rows; provider
  rows render readable labels such as `GitHub hosted compute ranges`.
- Direct MCP JSON-RPC `fetch_analysis` for `blocklist_de` returned the same
  compact markdown shape: only `iptoasn`, only `dbip_country`, day-based
  retention with `>365 days`, overlap percentages, nonblank technical specs,
  and the clarified Behavior section.

Operational note:

- One unrelated scheduled downloader later logged `dm_tor` as `HTTP/403
  Forbidden`; the manual reprocess itself completed with `failed=0`.

### Artifact Maintenance Gate

- `AGENTS.md`: no update. The project workflow and guardrails did not change.
- Runtime project skills: no update. The existing coding, testing, content, and
  operations rules already covered this work.
- Specs: updated `.agents/sow/specs/files-layout.md` and
  `.agents/sow/specs/website.md` for feed markdown provider selection,
  retention rollup, behavior table labeling, critical-provider rendering, and
  entity-local markdown paths.
- End-user/operator docs: updated `docs/api/mcp-endpoint.md` to document the
  pre-generated markdown behavior and compact feed-markdown contract.
- End-user/operator skills: no update. No portable operator skill consumes this
  markdown contract directly.
- SOW lifecycle: reopened SOW-0085 from `done`, recorded this regression
  section, mapped the remaining valid follow-up to SOW-0087, marked SOW-0085
  `completed`, and moved it back to `.agents/sow/done/`.

### Lifecycle Keyword Audit

- Historical `future` and on-demand-generation wording in the original
  2026-05-02 narrative is context for why the markdown package exists; it is not
  a remaining action from the 2026-05-16 regression.
- The valid remaining markdown-disabled admin visibility item is tracked in
  `.agents/sow/pending/SOW-0087-20260516-markdown-disabled-admin-visibility.md`.
- The original SOW-0014 reference remains a separate, pre-existing design
  discussion SOW and is not blocked by this markdown regression.
- `later` occurrences in the 2026-05-16 section refer to retention buckets
  older than 365 days or to runtime events after the manual reprocess; they do
  not describe untracked project work.

### Outcome

The 2026-05-16 regression is fixed. MCP `fetch_analysis` feed markdown is now
bounded to the configured canonical ASN and GeoIP providers, critical
infrastructure provider rows are readable, and retention tables are rendered as
daily buckets with omitted zero days and a bounded `>365 days` rollup.
Overlap rows now include both current-feed and compared-feed percentages with
explicit denominator text. The Behavior table now clearly labels its history
aggregation as a table bucket and points readers to Update Cadence for observed
feed-change intervals.

All current feed markdown artifacts were rebuilt on the local install after the
code change. The broad manual reprocess generated markdown for 391 feed pages
and completed without processing failures.

## Regression - 2026-05-16 - sparse behavior table

### Status

Status: completed

### Requirements

#### Purpose

Make the feed markdown Behavior table show useful activity, not sparse hourly
history buckets with zero added/removed counters.

#### User Request

The user reported that the Behavior table is useless when it appears as a fixed
`1h` interval because it shows many hourly rows whose counters are mostly zero.

#### Assistant Understanding

Facts:

- `pkg/markdown/rollup.go` chooses the first bucket interval from `1h`, `1d`,
  `1w`, and `1mo` that produces at most 100 rows.
- The row-count heuristic does not measure signal density.
- A bucket with `Added == 0` and `Removed == 0` does not add behavior signal to
  a markdown table; it repeats size state only.

Required outcome:

- Markdown behavior rows should include buckets with observed additions or
  removals.
- Zero-change buckets should not be emitted as markdown behavior rows.
- If no behavior rows remain after filtering, the Behavior section should be
  omitted by the existing context assignment rule.

### Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The markdown rollup was optimized for bounded row count, not useful rows.
  Hourly history can be under the row cap and still be empty-looking because
  changesets are sparse relative to history snapshots.

Evidence reviewed:

- `pkg/markdown/rollup.go` selects `1h` before coarser buckets when the row
  count is `<=100`.
- `configs/templates/markdown/feed.md.tmpl` renders every row returned by the
  rollup.
- `pkg/markdown/context_feed.go` already omits the whole Behavior section when
  the rollup returns no rows.

Affected contracts and surfaces:

- Feed markdown returned by `fetch_analysis`.
- Markdown rollup tests.
- Website spec MCP/feed-markdown contract.
- MCP endpoint operator docs.

Existing patterns to reuse:

- Keep rollup logic centralized in `pkg/markdown/rollup.go`.
- Keep markdown serving cache-first; rebuild artifacts through the existing
  admin reprocess path.
- Use focused package tests for the rollup contract.

Risk and blast radius:

- Low. This changes markdown rendering only; JSON history/change artifacts are
  unchanged.
- Some feeds with no observed changes will omit the Behavior section. That is
  preferable to publishing an empty-looking table.

Sensitive data handling plan:

- No sensitive data is needed. Durable artifacts will mention only public
  markdown behavior and code paths.

Implementation plan:

1. Filter rollup rows to buckets where `Added > 0` or `Removed > 0`.
2. Add tests proving inactive hourly buckets are omitted and no-change history
   produces no Behavior rows.
3. Update spec/docs to describe the activity-row contract.
4. Install and rebuild markdown artifacts.

Validation plan:

- Run `go test ./pkg/markdown`.
- Run the focused package set used for this markdown/MCP work.
- Reinstall with `./install.sh`.
- Run a full admin reprocess and spot-check published markdown.

Artifact impact plan:

- Specs: update `.agents/sow/specs/website.md`.
- Operator docs: update `docs/api/mcp-endpoint.md`.
- SOW lifecycle: reopen SOW-0085 from `done`, record the regression, and move
  it back to `done` with the fix.

Open decisions:

- None. The user identified zero-counter hourly rows as useless; filtering them
  preserves the useful activity rows while keeping existing generated JSON
  artifacts unchanged.

### Execution Log

- Reopened SOW-0085 from `.agents/sow/done/` to `.agents/sow/current/`.
- Changed `pkg/markdown/rollup.go` to filter Behavior rows to buckets with
  observed additions or removals.
- Added tests for sparse hourly history and no-change history.
- Updated feed markdown copy, website spec, and MCP endpoint docs to describe
  activity-only Behavior rows.
- Installed locally with `./install.sh`; systemd restarted the daemon.
- Rebuilt feed markdown artifacts with `POST /api/v1/admin/run?reprocess=true`.

### Validation

- `go test ./pkg/markdown`: passed.
- `go test ./pkg/config ./pkg/markdown ./pkg/engine ./pkg/mcp`: passed.
- `git diff --check`: passed.
- `./install.sh`: passed.
- Local health check returned `ok`.
- Manual admin reprocess logs showed `markdown pages generated feeds=391`,
  followed by `run finished updated=400 skipped=0 failed=0 elapsed=2m39.927s`
  and `processing batch completed updated=400 skipped=0 failed=0`.
- A scan of `/opt/update-ipsets/web/*.md` Behavior tables found no rows where
  both `Added` and `Removed` were `0`.
- Live MCP `fetch_analysis` for `apnic_telnet_bruteforce` returned a `1h`
  Behavior table containing only rows with non-zero additions/removals.

### Artifact Maintenance Gate

- `AGENTS.md`: no update. Workflow and project-wide guardrails did not change.
- Runtime project skills: no update. Existing markdown/testing rules covered
  the regression.
- Specs: updated `.agents/sow/specs/website.md` for activity-only Behavior
  rows.
- End-user/operator docs: updated `docs/api/mcp-endpoint.md` for MCP
  `fetch_analysis` markdown behavior.
- End-user/operator skills: no update.
- SOW lifecycle: reopened from `done`, recorded this regression, marked it
  completed, and moved it back to `.agents/sow/done/`.

### Outcome

Regression fixed. Feed markdown Behavior tables now omit zero-change buckets.
Hourly tables can still appear when the activity bucket itself is hourly, but
they no longer publish empty-looking hourly rows with zero added/removed
counters.
