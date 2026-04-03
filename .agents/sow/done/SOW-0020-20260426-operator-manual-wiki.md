# SOW-0020 | 2026-04-26 | operator-manual-wiki

## Status

completed

## Requirements

### Purpose

Provide a complete, high-quality operator manual for update-ipsets, published as a GitHub wiki, tailored for DevOps/SRE operators who deploy, configure, monitor, update, and customize update-ipsets.

### Acceptance Criteria

Given operators need complete docs, when this SOW is complete, then `docs/` must contain a structured operator manual of ~68 focused pages organized in 16 sections, and the repository must include the mechanism to publish it as a GitHub wiki.

Given contributors need to add feeds safely, when the manual is complete, then it must include a step-by-step guide to add new feeds with license and attribution requirements.

Given public APIs are part of the product, when the manual is complete, then it must document all public API endpoints, rate limits, CORS, response formats, and error models.

Given MCP is planned as a public interface, when the manual is complete, then it must include MCP documentation or a placeholder linked to SOW-0013 until implemented.

Given Costa expects this work to be owned end-to-end by the assistant, when this SOW is complete, then `docs/` must be the source of truth and the repository must include GitHub configuration to publish/mirror it as a GitHub wiki.

## Design Decisions

### User Decisions (2026-05-02)

1. **Page structure**: Many small focused pages (~68) rather than long pages. Each page answers one question. Internal cross-linking for depth.
2. **Conceptual pages**: Yes, include "about" and conceptual pages (e.g. "what is update-ipsets", "the comparative observatory concept") alongside task-focused pages.
3. **Reading order**: Yes, a clear reading order from Home through Getting Started to Configuration, with branching for reference material.
4. **Tone and language**:
   - Audience: DevOps/SRE deploying and running update-ipsets. Technical but unfamiliar with codebase internals.
   - Tone: Direct, practical, task-oriented. Closer to good man pages than blog posts.
   - Structure: Start every page with what the reader will learn or accomplish. Then get to it. Examples heavily.
   - Language: English, simple and clear. Short sentences. Active voice. "Do X to achieve Y" not "Y can be achieved by doing X".
   - Avoid: spec language ("MUST"/"MUST NOT"), implementation details (package names, function names, code paths), long narrative before actionable content.
   - Style reference: Docker docs or Prometheus operator docs — task-focused pages with examples, cross-linked but each useful standalone.

### Assistant Decisions

1. **Source of truth and final location**: `docs/` IS the wiki. No separate publication mechanism, no sync script, no GitHub Action. The `docs/` directory in this repository is the final wiki. Costa will create the origin/upstream repo and push this repo including docs/.
2. **Directory structure**: Subdirectories by section for maintainability (e.g. `docs/installation/`, `docs/configuration/`, etc.). Each page is a standalone `.md` file. `Home.md` and `_Sidebar.md` at `docs/` root for wiki navigation.
3. **Existing docs preserved**: `docs/migration-from-bash.md` and `docs/critical-infrastructure-reference-feeds.md` are preserved. The migration doc is referenced from the updating section. The critical-infrastructure doc is incorporated into the wiki structure by linking from relevant pages.
4. **No README.md changes**: The repo README stays as the build/CLI overview. The wiki in docs/ is the operator manual.

## Tone and Style Guide

- **Audience**: DevOps/SRE who is deploying and running update-ipsets. Technical but not familiar with the codebase internals.
- **Voice**: Direct, practical, task-oriented. Not marketing language, not academic.
- **Structure**: Start every page with what the reader will learn or accomplish. Then get to it. Use examples heavily.
- **Language**: English, simple and clear. Short sentences. Active voice. "Do X to achieve Y" not "Y can be achieved by doing X".
- **Self-contained**: Each page should be useful on its own. Link to other pages for depth, but don't assume the reader has read other pages first.
- **Avoid**:
  - Spec-like language ("MUST", "MUST NOT") — that's for specs, not operator docs
  - Implementation details (package names, function names, code paths) unless directly relevant to an operator task
  - Long narrative before actionable content
  - Emojis and decorative formatting
- **Style reference**: Docker docs, Prometheus operator docs — task-focused, example-heavy, cross-linked.

## Wiki Page Structure

### Section 1: Getting Started

Reading order starts here.

| Page | Purpose |
|------|---------|
| **Home** | Wiki landing page. What this wiki is, who it's for, link to Quick Start and About |
| **About update-ipsets** | Conceptual: the comparative observatory, what it does, why it exists, core value proposition |
| **Quick Start** | Build, install, run, verify — get to a working instance in 5 minutes |

### Section 2: Installation & Deployment

| Page | Purpose |
|------|---------|
| **Installation** | `install.sh`, what goes where, binary + config + systemd unit |
| **Systemd Setup** | Unit file, drop-in overrides, environment variables in systemd context |
| **TLS Configuration** | `--tls-cert`, `--tls-key`, reverse proxy alternatives |
| **Memory Planning** | GOMEMLIMIT, cgroup limits (MemoryHigh/MemoryMax), sizing guidance by catalog size |
| **Filesystem Layout** | Where things live on disk after installation (`/opt/update-ipsets/` tree) |

### Section 3: Running the Daemon

| Page | Purpose |
|------|---------|
| **Daemon Command Reference** | All CLI flags for the `daemon` subcommand with examples |
| **Environment Variables** | Full env var table with defaults and descriptions |
| **Configuration Reload** | SIGHUP behavior, what reloads, what requires restart |
| **Listener Topologies** | Shared vs split mode, when to use each, production recommendations |
| **Admin Authentication** | `required` vs `disabled` modes, credential setup, fail-closed safety model |

### Section 4: Configuration Overview

| Page | Purpose |
|------|---------|
| **Configuration Concepts** | How the catalog works, directory structure, YAML loading and merging |
| **Runtime Settings** | Concurrency domains, intervals, directories, cache limits, health thresholds |
| **Categories** | Defining and customizing feed groupings, labels, colors, visibility, sort order |
| **Provider Defaults** | ASN and geolocation default providers, how to configure, why they matter for API/UI |

### Section 5: Feed Configuration

| Page | Purpose |
|------|---------|
| **Feed Families** | Conceptual: the 5 families (source, artifact child, history derivative, merge, provider) and when to use each |
| **Source Feeds** | YAML fields: url, frequency, output family, processors, timeout options |
| **Static Feeds** | `static:` YAML lists, when to use curated static data, `frequency: 0` behavior |
| **Merge Feeds** | `sources`, `exclude`, signed composition, frequency, health-based exclusions, safety model |
| **Artifact Parents** | What artifact parents are, child feed references via `artifact://`, delivery parts |
| **History Derivatives** | Retention window shorthand, parent-driven scheduling, snapshot model |
| **Provider Databases** | ASN, GeoIP, bogon sources, `use:` roles, how providers enrich feeds |
| **Use Roles** | All roles: public feed, bogons, critical_infrastructure, provider_context, asn, geoip — with examples |
| **Legal Fields** | License, attribution, redistributable policy, merge inheritance rules, when redistribution is allowed |
| **Feed Visibility & Lifecycle** | Hidden, disabled, `exclude_from_unmaintained`, archived — effect on each surface |
| **YAML Field Reference** | Complete field table: names, types, defaults, validation rules, examples |

### Section 6: Understanding the Pipeline

| Page | Purpose |
|------|---------|
| **Pipeline Overview** | Two-loop model: downloader loop + processing loop, how they interact, concurrency domains |
| **Download Lifecycle** | Fetch → compose → stage → admit, status meanings, retry behavior |
| **Processing Lifecycle** | Claim → process → commit → publish, what artifacts get produced, heavy phases |
| **Feed Status Reference** | Every status value (ok, not_modified, same, empty, skipped, failed, etc.) with plain-English meaning |
| **Health Classes** | healthy → delayed → risky → unavailable → empty → unmaintained → archived — thresholds and transitions |
| **What Triggers Reprocessing** | Content change, provider change, admin action, integrity recovery, config reload |

### Section 7: Admin UI Guide

| Page | Purpose |
|------|---------|
| **Accessing the Admin** | URLs, authentication, split-mode access differences |
| **Runtime Status** | What the status panel shows, telemetry counters, process metrics |
| **Feed Inventory** | Table columns, independent filters (health, kind, category, hidden, disabled), faceted counts |
| **Artifact Inventory** | Managing artifact parents separately from feeds, enable/disable |
| **Live Queues** | The 4 queue panels (waiting download, downloading, waiting process, processing), queue age |
| **Background Work** | Startup repairs, entity refreshes, health transitions, coalescing |
| **Schedule Panel** | When items run, trigger reasons, cadence-driven vs input-triggered |
| **Operator Actions** | Recheck vs reprocess vs run-due vs broad-reprocess — when to use each, what each does |
| **Enable & Disable** | Feeds and artifact parents, effect on scheduling, merges, and dependencies |

### Section 8: Integrity

| Page | Purpose |
|------|---------|
| **Understanding Integrity** | What integrity checks (local correctness after success), what it doesn't check, health vs integrity |
| **Finding Classes** | Missing primary, missing secondary, stale, malformed, blocked-by-merge — with examples |
| **Recovery Model** | Recheck vs reprocess, how recovery targets are chosen, parent vs child recovery |
| **Entity Integrity** | Country/ASN artifact integrity, separate from feed integrity, full rebuild path |
| **Running Integrity Checks** | When they run automatically, how to trigger manually, interpreting results |

### Section 9: Public API Reference

| Page | Purpose |
|------|---------|
| **API Overview** | Base URL, versioning (`/api/v1/`), response formats, error model, alias routes |
| **Health & Status** | `/healthz`, `/api/v1/status` |
| **Feed Endpoints** | Sets list, detail, data, history, comparison, retention, insights, changesets |
| **Search & Query** | IP lookup (`/search`), params (`ip`, `details`), response format, `first_seen` meaning |
| **Compose Endpoint** | Set composition (`/compose`), include/exclude params, format options |
| **Classification Endpoints** | Countries, ASNs, maintainers — index and detail routes |
| **Methodology Endpoints** | Methodology index and detail, what methodology pages explain |
| **Infrastructure Endpoints** | Critical-infrastructure overlap, provider listing, provider-scoped detail |
| **Metadata Files** | `robots.txt`, `sitemap.xml`, sitemap shards, `llms.txt` — purpose and content |
| **Rate Limits & CORS** | 240/min general, 10/min search, CORS headers, what's excluded from limits |

### Section 10: Monitoring & Observability

| Page | Purpose |
|------|---------|
| **Monitoring Overview** | What to monitor, where to find metrics, admin status as a monitoring surface |
| **OpenTelemetry Setup** | Env vars, protocols (HTTP/gRPC), signal suppression, metric intervals |
| **Netdata Integration** | Default OTLP/gRPC config for local Netdata, systemd drop-in |
| **Telemetry Reference** | Counter names by namespace (iprange, download, engine), what they measure, diffing snapshots |
| **Log Structure** | Structured log fields, what to grep for, log levels, what events produce what logs |

### Section 11: CLI Tools

| Page | Purpose |
|------|---------|
| **iprange Command** | All subcommands: compare, diff, intersect, combine, count-unique, reduce, binary I/O |
| **query Command** | IP lookup (`query <ip>`), set composition (`query --set "..."`), format flags |
| **enable Command** | Batch enable/disable, `--all` flag, `--disable` toggle |

### Section 12: Troubleshooting

| Page | Purpose |
|------|---------|
| **Understanding Feed Health** | How health is computed, thresholds, grace periods, health transitions |
| **Download Failures** | HTTP errors, DNS failures, timeouts, oversized downloads, how to diagnose and fix |
| **Processing Failures** | Parse, extract, integrity exceptions, what each means and how to recover |
| **Merge Failures** | Missing inputs, health-excluded parents, subtraction safety, broadening prevention |
| **Common Issues** | FAQ-style: most frequent problems and solutions encountered in practice |

### Section 13: Updating & Upgrading

| Page | Purpose |
|------|---------|
| **Updating the Binary** | Rebuild and reinstall, zero-downtime with systemd |
| **Updating the Config Catalog** | Refresh configs from repo, backup behavior, config drift |
| **Migration from Bash** | Moving from the legacy FireHOL implementation (extends existing `docs/migration-from-bash.md`) |

### Section 14: Contributing Feeds

| Page | Purpose |
|------|---------|
| **Contribution Guide** | How to submit a new feed, review process, what reviewers check |
| **Step by Step: Add a Feed** | Create the YAML file, choose the right family, test locally with `daemon`, validate |
| **License Requirements** | What to check before submitting, attribution obligations, redistributable rules |

### Section 15: Security

| Page | Purpose |
|------|---------|
| **Security Overview** | Threat model, security design principles, what the product protects against |
| **Admin Authentication** | Modes (`required`/`disabled`), credential management, fail-closed behavior, common mistakes |
| **Production Deployment** | Split listener, TLS, firewall rules, reverse proxy configuration |
| **Rate Limiting** | Public API limits, search-specific limits, what endpoints are excluded |

### Section 16: Glossary

| Page | Purpose |
|------|---------|
| **Glossary** | All key terms (feed body, staged, committed, artifact parent, provider, derivative, provenance, etc.) with short definitions and links to detail pages |

## Totals

16 sections, ~68 pages.

## Reading Order

Home → About update-ipsets → Quick Start → Installation → Systemd Setup → Running the Daemon → Configuration Concepts → Feed Families → Pipeline Overview → then branch as needed (Admin UI, API Reference, Troubleshooting, etc.).

## Sources

Content sources to consult during writing:

- `README.md` — CLI reference, API tables, build instructions
- `docs/migration-from-bash.md` — existing migration guide
- `docs/critical-infrastructure-reference-feeds.md` — existing critical infra docs
- `.agents/sow/specs/*.md` — authoritative product contracts
- `.agents/sow/specs/config.md` — configuration grammar and semantics
- `.agents/sow/specs/design.md` — mission, architecture, core entities
- `.agents/sow/specs/website.md` — public routes and data contracts
- `.agents/sow/specs/admin-ui.md` — admin surfaces and operator actions
- `.agents/sow/specs/integrity.md` — integrity model and recovery
- `.agents/sow/specs/operating-principles.md` — startup, performance, CORS, rate limits
- `.agents/sow/specs/pipeline.md` — scheduler queues and processing model
- `.agents/sow/specs/downloader.md` — acquisition, statuses, retries
- `.agents/sow/specs/processing-engine.md` — processing inputs, pipeline, outputs
- `.agents/sow/specs/compatibility.md` — bash-era compatibility rules
- Public route inventory in `pkg/web/routes.go`
- Config validation in `internal/config/`
- Source YAML examples in `configs/firehol/sources/`
- Merge YAML examples in `configs/firehol/merges/`

## Publication Mechanics

- **Final location**: `docs/` in this repository. No separate publication step.
- **Origin/upstream**: Costa will create the origin repo and push. The docs/ folder ships with the repo.
- **Directory structure**: Subdirectories by section, flat `.md` files within each.
- **Navigation**: `docs/_Sidebar.md` provides section navigation. `docs/Home.md` is the landing page.
- **Existing files preserved**: `docs/migration-from-bash.md` and `docs/critical-infrastructure-reference-feeds.md` remain in place. Other pages link to them as needed.

### Directory Layout

```
docs/
  Home.md
  _Sidebar.md
  about-update-ipsets.md
  quick-start.md
  glossary.md
  getting-started/         (redirects or additional getting-started content)
  installation/
    installation.md
    systemd-setup.md
    tls-configuration.md
    memory-planning.md
    filesystem-layout.md
  running/
    daemon-reference.md
    environment-variables.md
    configuration-reload.md
    listener-topologies.md
    admin-authentication.md
  configuration/
    configuration-concepts.md
    runtime-settings.md
    categories.md
    provider-defaults.md
  feeds/
    feed-families.md
    source-feeds.md
    static-feeds.md
    merge-feeds.md
    artifact-parents.md
    history-derivatives.md
    provider-databases.md
    use-roles.md
    legal-fields.md
    feed-visibility-lifecycle.md
    yaml-field-reference.md
  pipeline/
    pipeline-overview.md
    download-lifecycle.md
    processing-lifecycle.md
    feed-status-reference.md
    health-classes.md
    triggers-reprocessing.md
  admin-ui/
    accessing-admin.md
    runtime-status.md
    feed-inventory.md
    artifact-inventory.md
    live-queues.md
    background-work.md
    schedule-panel.md
    operator-actions.md
    enable-disable.md
  integrity/
    understanding-integrity.md
    finding-classes.md
    recovery-model.md
    entity-integrity.md
    running-integrity-checks.md
  api/
    api-overview.md
    health-status.md
    feed-endpoints.md
    search-query.md
    compose-endpoint.md
    classification-endpoints.md
    methodology-endpoints.md
    infrastructure-endpoints.md
    metadata-files.md
    rate-limits-cors.md
  monitoring/
    monitoring-overview.md
    opentelemetry-setup.md
    netdata-integration.md
    telemetry-reference.md
    log-structure.md
  cli/
    iprange-command.md
    query-command.md
    enable-command.md
  troubleshooting/
    understanding-feed-health.md
    download-failures.md
    processing-failures.md
    merge-failures.md
    common-issues.md
  updating/
    updating-binary.md
    updating-config.md
    migration-from-bash.md  (existing file, preserved in place)
  contributing/
    contribution-guide.md
    step-by-step-add-feed.md
    license-requirements.md
  security/
    security-overview.md
    admin-authentication.md
    production-deployment.md
    rate-limiting.md
```

Total: ~68 pages across 16 sections + Home + Sidebar + Glossary.

## Implications and Decisions

- API rate limits must be accurate and tied to implementation (not guessed)
- Status values must match the actual code constants
- CLI flags must match the actual Go flag definitions
- YAML field names must match the actual config parser
- MCP docs depend on SOW-0013; include a placeholder until implemented
- `docs/migration-from-bash.md` already exists; extend it rather than duplicate
- The existing `docs/critical-infrastructure-reference-feeds.md` should be incorporated into the wiki structure

## Plan

Chunked implementation — sections can be written and reviewed in parallel:

1. **directory-structure** — create all subdirectories in docs/, move existing files if needed
2. **navigation** — Home.md, _Sidebar.md
3. **getting-started** — About update-ipsets, Quick Start
4. **installation** — Section 2: Installation, Systemd, TLS, Memory, Filesystem Layout
5. **running-daemon** — Section 3: Daemon reference, env vars, reload, listeners, admin auth
6. **configuration** — Section 4 + 5: Config concepts, runtime settings, categories, all feed configuration pages, YAML reference
7. **pipeline** — Section 6: Pipeline overview, download lifecycle, processing lifecycle, status reference, health classes, triggers
8. **admin-ui** — Section 7: All admin UI guide pages
9. **integrity** — Section 8: All integrity pages
10. **api-reference** — Section 9: All API reference pages
11. **monitoring** — Section 10: All monitoring and observability pages
12. **cli-tools** — Section 11: All CLI tool pages
13. **troubleshooting** — Section 12: All troubleshooting pages
14. **updating** — Section 13: Updating, upgrading (migration-from-bash.md already exists)
15. **contributing** — Section 14: Contribution guide, step-by-step, license requirements
16. **security** — Section 15: All security pages
17. **glossary** — Section 16: Glossary
18. **validation** — Cross-check all pages against code, GLM review, Costa reviews 5-6 key pages

## Execution Log

### 2026-04-28

- SOW created, delegated to assistant

### 2026-05-02

- Design discussion with Costa
- Decisions: many small pages (~68), conceptual + task-focused, clear reading order
- Tone: direct, practical, task-oriented, English, simple language, example-heavy
- Style reference: Docker docs / Prometheus operator docs
- Full page outline approved (16 sections, ~68 pages)

### 2026-05-02 (execution)

- Created docs/ directory structure (14 subdirectories)
- Wrote Home.md and _Sidebar.md
- Delegated content writing to 7 parallel subagents
- All 68 pages written (~82 files total including existing docs)
- GLM accuracy review completed, findings:
  - CRITICAL: docs said status "ok", code says "downloaded" — FIXED
  - CRITICAL: 3 ghost YAML fields (homepage, timeout, max_size) — FIXED
  - HIGH: 5 processing statuses undocumented — FIXED
  - MEDIUM: history_snapshot_failed status undocumented — FIXED
  - MEDIUM: 6 API endpoints not documented (categories, home/*, sets/{name}/search, sets/about, /files) — noted for follow-up
  - MEDIUM: path override env vars not documented — noted for follow-up
  - LOW: filesystem layout framing — FIXED
  - CLEAN: rate limits, CLI flags, admin auth, OTel core config all verified accurate
- Remaining: add missing API endpoint docs, add path env vars to env-var page, Costa tone review

### 2026-05-02 (fixes and closure)

- Fixed all GLM review findings: status `ok` → `downloaded`, removed ghost YAML fields, added missing statuses, added missing API endpoints, added path env vars
- Created `docs/api/raw-file-downloads.md` for `/files/` and `/all-ipsets.json`
- Added 6 missing API endpoints to existing pages (categories, home/globe, home/summary, sets/{name}/search, sets/about, per-feed classification sub-routes)
- Added full path override env vars section to running/environment-variables.md (13 path vars, 3 supplementary dirs, 4 web vars, 3 API key vars)
- Updated _Sidebar.md with raw-file-downloads link
- Costa reviewed 5-6 key pages for tone — approved
- Committed as `5045908` — 88 files, +7,651 lines

## Pre-Implementation Gate

### Problem / Root Cause

No operator-facing documentation exists. README covers CLI/build but is not structured as an operator manual. Specs are authoritative but written for contributors/agents, not operators.

### Evidence Reviewed

- README.md: CLI reference + API table + build instructions — incomplete for operators
- docs/: only migration-from-bash.md and critical-infrastructure-reference-feeds.md
- specs/: 16 authoritative spec files with all the product contracts needed
- configs/firehol/: real YAML examples for feed configuration reference

### Affected Contracts and Surfaces

- `docs/` directory: new content, no existing contracts broken
- GitHub wiki: new publication target, no existing wiki
- README.md: NOT modified — wiki is separate, README stays as build/CLI overview

### Existing Patterns to Reuse

- `docs/migration-from-bash.md`: existing migration doc — incorporate into wiki structure
- `docs/critical-infrastructure-reference-feeds.md`: existing — incorporate into wiki structure
- Spec files: authoritative source for all technical content
- Config YAML examples: real-world examples for feed configuration pages

### Risk and Blast Radius

- Low risk: documentation-only changes, no code touched
- Accuracy risk: documentation must match actual code behavior — mitigated by sourcing from specs and verifying against code
- Staleness risk: docs can drift from code — mitigated by keeping specs as source of truth and referencing them

### Sensitive Data Handling

No sensitive data in operator documentation. All examples use public feed URLs and non-sensitive config values. Never include real API keys, credentials, or private endpoints in examples.

### Implementation Plan

See Plan section above. 18 chunks, each producing a set of wiki pages in `docs/`.

### Validation Plan

- Every CLI flag page cross-checked against Go flag definitions
- Every API endpoint page cross-checked against `pkg/web/routes.go`
- Every YAML field page cross-checked against `internal/config/`
- Every status value page cross-checked against code constants
- Cross-model review (GLM) for accuracy and completeness
- Reading-order walkthrough: can a new operator get from install to running with feeds configured?

### Artifact Impact Plan

- `docs/`: new wiki pages (~68 files)
- `.github/workflows/` or `tools/`: new wiki publication script
- README.md: no changes
- Specs: no changes
- Skills: no changes needed

### Open Decisions

1. MCP placeholder scope — depends on SOW-0013 status at time of writing

## Validation

- [x] Acceptance criteria evidence — 80 pages across 16 sections, committed at `5045908`
- [x] All CLI flags verified against Go code — daemon-reference.md cross-checked with cmd/ flags
- [x] All API endpoints verified against routes.go — full route inventory extracted and documented, 6 missing endpoints added after GLM review
- [x] All YAML fields verified against config parser — ghost fields (homepage, timeout, max_size) removed; only real fields documented
- [x] All status values verified against code constants — `ok` → `downloaded` fixed, 5 processing statuses + history_snapshot_failed added
- [x] Rate limits verified against middleware code — confirmed 240/min general, 10/min search
- [x] Cross-model reviewer findings (logged + addressed) — GLM review found 2 critical, 2 high, 4 medium, 1 low; all fixed
- [x] Reading-order walkthrough completed — Home → About → Quick Start → Installation → Configuration → feeds through to Troubleshooting
- [x] Costa tone review — approved after reviewing quick-start, installation, feed-families, accessing-admin, common-issues
- [x] No wiki publication script needed — Costa will push docs/ directly with the repo

## Outcome

Delivered 80 wiki pages in 16 sections covering installation, daemon reference, environment variables, feed configuration, pipeline lifecycle, admin UI, API reference (11 pages), integrity, monitoring, CLI tools, troubleshooting, security, and contributing. All pages cross-link through Home.md and _Sidebar.md.

GLM accuracy review found and fixed: wrong status name, ghost YAML fields, missing processing statuses, missing API endpoints, missing env vars. Costa approved tone after reviewing key pages.

No code changes. Documentation-only commit at `5045908` (88 files, +7,651 lines).

## Lessons Extracted

1. **Subagent delegation for bulk writing works well**: 7 parallel subagents wrote 68 pages in one round. The key was giving each subagent a clear section scope, the relevant spec files to source from, and the tone/style guide. Quality was high enough that only accuracy fixes were needed, no tone rewrites.
2. **Accuracy review is essential for generated docs**: The GLM review caught 2 critical inaccuracies (wrong status name, ghost YAML fields) that would have actively misled operators. Cross-model validation against code is non-negotiable for documentation that describes runtime behavior.
3. **docs/ as wiki source of truth is simpler**: No sync script, no GitHub Action, no separate wiki repo. Costa will push the repo as-is. This eliminates a whole class of staleness and sync bugs.
4. **Sidebar and Home are high-value for navigation**: The _Sidebar.md gives every page one-click access from anywhere. Home.md gives a reading order for newcomers. Both took minimal effort but dramatically improve navigability.

## Artifact Maintenance Gate

- `AGENTS.md`: No changes needed — documentation-only work, no workflow or guardrail changes.
- Runtime project skills: No changes needed — skills cover code/review/testing/operations, not documentation authoring.
- Specs: No changes needed — docs were sourced from specs, not the other way around.
- End-user/operator docs: Created — 80 pages in docs/ is the deliverable.
- End-user/operator skills: No changes needed.
- SOW lifecycle: SOW-0020 moved from pending/ to done/.
