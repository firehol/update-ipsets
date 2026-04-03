# update-ipsets - Agent Handbook

## Goals

update-ipsets is a Go rewrite of the FireHOL `update-ipsets` pipeline. It downloads, normalizes, compares, publishes, and serves public cybercrime IP feeds with an embedded public/admin web UI.

Success for this project means factual feed comparison, cheap cache-first public serving, explicit operator visibility, bounded CPU/memory/I/O behavior, safe release hygiene, and clear documentation for operators and contributors.

Website: https://iplists.firehol.org

## Working pattern

Default: investigate first, then delegate where it materially helps. Use subagents for heavy or independent analysis, review, and test work when the harness supports it and the global assistant policy allows it.

Synchronous, step-by-step work happens only when the user requests it explicitly (e.g., to follow reasoning live), or for trivial tasks not worth the delegation overhead.

For SOW initialization, re-init, repair, migration, and re-review, preserve existing SOWs, specs, skills, and project-specific `AGENTS.md` content unless the user explicitly approves specific changes.

## SOW System

This `AGENTS.md` is the runtime SOW authority for this project. The SOW framework is self-contained in this repository; normal SOW work must not rely on `~/.agents`, `~/.AGENTS.md`, global templates, or global scripts.

### Roles

- **Our role in this project:** maintainer/local-maintainer. Evidence: this clone has no configured remote or CODEOWNERS, and all reachable commits are authored by the user.
- **Assistant's responsibilities in SOWs:** analyze current state, propose options with evidence, implement approved changes, delegate review/testing where useful, update `.agents/sow/specs/`, docs, and project skills, validate, and commit only explicitly approved paths when asked.
- **User's responsibilities in SOWs:** define purpose and design direction, approve design decisions, approve AGENTS/TODO-removal gates, review outcomes, and provide production/release approval.

### Project SOW Rules

- Non-trivial work in this repo uses SOW because most changes affect feed correctness, operator behavior, release hygiene, public serving, or durable project memory.
- Trivial mechanical edits may bypass SOW only when they do not affect behavior, contracts, tests, specs, skills, or release/operation guidance.
- Regressions reopen the closest matching completed SOW; do not create a detached replacement SOW for behavior that a prior SOW claimed working.
- User design, risk, destructive-operation, and validation-gap decisions are recorded in the active SOW before implementation.
- "Deferred" is not a terminal outcome by itself. Before a SOW can be completed, every deferred valid item must be either implemented in the current SOW, explicitly rejected as not worth doing with evidence, or represented by a concrete pending SOW path. Untracked deferrals are treated as lost work, not backlog.
- A deferred valid item represented by a pending SOW is the next focused work after the current SOW completes, unless a newer user priority explicitly supersedes it. Deferring means "do it immediately afterward alone with proper focus", not "maybe later".
- Never batch SOWs together as one execution unit. If multiple SOWs overlap, merge or consolidate them into one SOW before implementation; otherwise work exactly one SOW at a time through analysis, implementation, validation, and closure before starting the next.

### SOW Completion And Commit

The successful terminal SOW status is `completed`. `done` is a directory name, not a status value. Never write `Status: done` or `Status: complete`.

When a SOW's work is ready to close:

1. Finish implementation, docs, specs, skills, validation, and follow-up mapping.
2. Update the SOW to `Status: completed`.
3. Move the SOW file to `.agents/sow/done/`.
4. Commit the work, artifact updates, SOW status change, and SOW move together as one commit, unless the user explicitly requested a different commit split.

Do not create a separate commit just to mark or move the SOW. Do not claim a SOW is completed while the implementation and the SOW lifecycle change live in separate uncommitted or separately committed states.

### Regressions

A regression exists when a SOW was considered completed or closed, then later testing or use finds broken behavior.

Reopen the original SOW and append a dated `## Regression - YYYY-MM-DD` section at the end of the file, after the original outcome, lessons, and follow-up content. Never prepend regression content above the original SOW narrative.

### Git Worktrees

Assistants must not create git worktrees on their own. Create a git worktree only when the user explicitly asks for it or approves it.

### Sensitive Data In Durable Artifacts

SOWs, specs, documentation, project skills, agent instructions, and code comments are commit-ready artifacts. Treat them as public unless a repository-specific policy explicitly says otherwise.

CRITICAL: Never write raw sensitive data to durable artifacts. This includes passwords, API keys, bearer tokens, SNMP communities, private keys, connection strings with embedded credentials, session cookies, community member names, customer names, customer identifiers, personal data, non-private IP addresses that can identify customers, private endpoints, account IDs, and proprietary incident details.

Write only sanitized evidence:

- use placeholders such as `[REDACTED_SECRET]`, `[CUSTOMER]`, `[ACCOUNT]`, `[PRIVATE_ENDPOINT]`;
- use stable aliases such as `customer-a` only when the real mapping is not stored in the repository;
- cite file paths, line numbers, command names, schema fields, or error classes instead of copying sensitive values;
- summarize logs and traces; include only minimal redacted snippets.

If sensitive data is required to continue, stop and ask the user for a secure handling path. If sensitive data is found in a durable artifact, sanitize it before any commit. If sensitive data was already committed, tell the user and do not rewrite history without explicit approval.

### Open-Source Reference Evidence

When SOW evidence comes from local mirrored or cloned open-source repositories, cite the upstream repository and checked commit instead of the workstation absolute path.

Use:

```text
owner/repo @ commit
relative/path/inside/repo:line
```

Resolve `owner/repo` from the repository remote, record the checked commit, and keep paths relative to the upstream repository root. Never write workstation absolute paths for external open-source evidence into SOW evidence.

### Pre-Implementation Gate

Implementation must not begin until the active SOW contains a concrete `## Pre-Implementation Gate` section. Before moving a SOW from `pending/open` to `current/in-progress`, or before continuing implementation in an existing current SOW that lacks this section, fill the gate.

The gate must record the problem/root-cause model, evidence reviewed, affected contracts and surfaces, existing patterns to reuse, risk and blast radius, sensitive data handling plan, implementation plan, validation plan, artifact impact plan, and open decisions. The sensitive data plan must cover SOWs, specs, documentation, project skills, agent instructions, and code comments. Generic placeholders such as `TBD`, `N/A`, or "to be checked later" are invalid unless the SOW explains why the item truly does not apply. If the gate exposes an unknown that cannot be resolved by investigation, stop and ask the user before implementation.

### Project Skills

The assistant MUST follow these for the work they cover:

- `.agents/skills/project-coding/` - Go, React, config, and repo conventions; required for code changes
- `.agents/skills/project-reviewing/` - review checklist and standards; required for code reviews
- `.agents/skills/project-testing/` - test commands, fixtures, and validation patterns; required for test work
- `.agents/skills/project-operations/` - install, daemon, admin, and runtime operation guidance; required for operational changes
- `.agents/skills/project-content-surfaces/` - audience/surface discipline for SOWs, specs, docs, methodology pages, UI copy, and admin copy; required for non-code content changes
- `.agents/skills/project-go-best-practices/` - modern Go implementation checklist; required for Go code changes
- `.agents/skills/project-go-behavioral-testing/` - black-box Go testing workflow; required for Go test work or reviewing Go tests
- `.agents/skills/project-frontend-best-practices/` - React/TypeScript/Tailwind implementation checklist; required for frontend code changes
- `.agents/skills/project-frontend-behavioral-testing/` - black-box UI testing workflow; required for frontend test work or reviewing UI tests

The assistant maintains these skills during SOW retrospection (lessons -> updates).

### Where things live

- `.agents/sow/specs/` - authoritative product/application specs and long-lived SOW specs
- `.agents/sow/pending/` - SOWs awaiting work
- `.agents/sow/current/` - SOWs in progress
- `.agents/sow/done/` - completed SOWs
- `.agents/sow/.todo-backup/` - preserved root TODO files migrated during SOW initialization

For project-local SOW runtime rules, use this `AGENTS.md`, `.agents/sow/SOW.template.md`, `.agents/sow/audit.sh`, project specs, and project skills.

### Artifact maintenance gate

Every SOW close must record whether each durable artifact class was updated or why no update was needed:

- `AGENTS.md`: workflow, responsibility, local framework, or project-wide guardrails.
- Runtime project skills: `.agents/skills/project-*/SKILL.md`.
- Specs: `.agents/sow/specs/`.
- End-user/operator docs: README, docs, public methodology pages, runbooks, help text, or published guides.
- End-user/operator skills: output/reference skills copied or consumed outside normal repo work.
- SOW lifecycle: split/merge decisions, status, directory, deferred work, regression reopening, and follow-up mapping.

### Specs

Product and application contracts live under `.agents/sow/specs/*.md`. Keep them current. There is no repo-root `specs/` compatibility path.

Canonical product spec map:

- `.agents/sow/specs/README.md` - spec map and canonical ownership of contracts
- `.agents/sow/specs/design.md` - mission, design goals, and high-level architecture
- `.agents/sow/specs/downloader.md` - acquisition, composition, statuses, retries, ownership
- `.agents/sow/specs/processing-engine.md` - processing inputs, pipeline, outputs, ownership
- `.agents/sow/specs/config.md` - user configuration, YAML model, roles, runtime knobs, license policy
- `.agents/sow/specs/feeds.md` - feed knowledge, state, and maintained files
- `.agents/sow/specs/files-layout.md` - filesystem layout, ownership, staging, migration/import paths
- `.agents/sow/specs/pipeline.md` - scheduler queues, processing model, derivatives, artifact parents
- `.agents/sow/specs/integrity.md` - integrity checks, recovery, suppression rules
- `.agents/sow/specs/operating-principles.md` - startup, performance, cache-first serving, dependency discipline, bounded work
- `.agents/sow/specs/memory-management.md` - out-of-core, mmap, streaming, bounded memory behavior
- `.agents/sow/specs/website.md` - public website routes, frontend stack, design system
- `.agents/sow/specs/homepage.md` - homepage hero, IP lookup, feed explorer
- `.agents/sow/specs/admin-ui.md` - admin API/UI operations and operator semantics
- `.agents/sow/specs/compatibility.md` - bash-era compatibility and non-compatibility rules

Supporting docs:

- `README.md` - CLI/build/deploy overview
- `docs/*.md` - operator documentation, API usage, runbooks, and migration guidance
- `docs/migration-from-bash.md` - operator migration from bash implementation
- `docs/todo-history/*.md` - preserved design history, not current work control
- `pkg/web/static/methodology/*.md` - public methodology pages for end-user/operator interpretation of site signals; not implementation notes

### Repo Rules

- If a change affects behavior, configuration semantics, file layout, pipeline behavior, website/admin behavior, integrity, memory, or compatibility, update the relevant file under `.agents/sow/specs/` immediately.
- Do not hardcode feed names. Feed identity comes from `configs/firehol/`; use config fields, `use:` roles, or exposed backend flags for semantic distinctions.
- Never derive semantic meaning by pattern-matching feed names, provider names, or generated artifact filenames. No substring/prefix/suffix classification such as `_bogons_`, `_asn_`, or `_critical_`; use configuration fields, `use:` tags, typed metadata, or exact configured-name identity lookups.
- Configuration field names, `use:` tags, and typed metadata are the source of truth for source roles, provider roles, artifact families, criticality, redistributability, and UI/API semantics.
- Pipeline integrity depends on generated file mtimes. Read `.agents/sow/specs/integrity.md` and the pipeline timestamp contract before changing writers, staged publish paths, repair paths, or integrity checks.
- Integrity checks rely on mtime discrepancies to detect runtime drift. No file created by the application may accidentally inherit wall-clock mtime when it participates in pipeline integrity; each writer must deliberately set the mtime required by the spec.
- Do not hardcode operator-policy IP/CIDR lists in Go or UI code. Curated reference data belongs in YAML `static:`, `url:` sources, artifacts, or merges so operators can customize it without rebuilding.
- Before editing SOWs, specs, docs, public methodology pages, public UI copy, or admin UI copy, identify the surface, audience, purpose, success criteria, and forbidden content. Use `project-content-surfaces`.
- Public methodology pages must explain user-facing meaning, levels/taxonomy, interpretation, strengths, weaknesses, missing coverage, and false-positive/false-negative risks. They must not contain config schemas, code paths, artifact filenames, migration notes, or internal validation mechanics except when a brief API/operator link is explicitly needed.
- SOWs, specs, docs, public methodology pages, backend code, public UI, and admin UI have different goals and success criteria. Do not reuse text mechanically across them.
- When converting analysis SOWs into implementation SOWs, update the Requirements, Assistant understanding, Acceptance criteria, Plan, and Validation sections first so stale "analysis only" or "follow-up later" wording cannot silently control the work.
- Before closing any SOW, search the SOW for `defer`, `later`, `follow-up`, `future`, `TODO`, and `pending`; map each valid remaining item to an implemented change, a rejected/non-goal decision with evidence, or a pending SOW filename.
- `pkg/iprange` stays standalone; it must not import other project packages.
- Generated frontend assets are not source files. Do not edit `pkg/web/static/assets/*` or generated `pkg/web/static/index.html`; edit `ui/`.
- Startup availability matters. Do not put expensive historical rescans or broad rebuilds on the daemon startup critical path.
- Public serving must stay cache-first and cheap; public requests must not trigger upstream downloads or broad recomputation.
- Background work must be visible through the admin API/UI.
- Use `project-testing` for validation commands. Important commands include `make build`, `make test`, `make race`, `make lint`, `make bench`, `pnpm --dir ui build`, and `pnpm --dir ui lint` when relevant.
- Use `project-operations` for install/service work. The authoritative local install path is `./install.sh`.

### Project Map

- `cmd/` - Go entrypoints
- `internal/` - private helper packages
- `pkg/` - Go packages
- `ui/` - React SPA source
- `tools/` - nested helper modules, including `tools/dronebl2ipsets/`
- `configs/` - YAML catalog
- `.agents/sow/specs/` - authoritative product specifications
- `docs/` - operator docs and preserved history
- `install.sh` - authoritative build/install path

Legacy reference:

- `/home/costa/src/firehol/firehol/`

### Source-of-source legal scope (license / redistribution classification)

A feed's `license`, `redistributable`, and `redistribution_notes` fields are
based ONLY on the terms of that feed's direct upstream — the URL the
catalog actually downloads from. Terms found at upstream-of-upstream layers
(for example, a commercial provider's ToS when our direct upstream is a
community GitHub mirror that republishes their data) are research signal
worth recording in `research_notes`, but MUST NOT change our classification
fields. Resolving the legal relationship between our direct upstream and
its own upstream is their concern, not the project's.

When the direct upstream is publicly available and states no rule:
`redistributable` defaults to `true`; `license` defaults to `"public feed"`.
The defaults apply even when restrictive terms exist at upstream-of-upstream
layers.

Authoritative spec: `.agents/sow/specs/ai-classification-rules.md`.
Operational include shared across AI agents:
`agents/shared/classification-rules.md`.
Public end-user methodology page:
`pkg/web/static/methodology/ai-research-license-rules.md`.

### Project-specific overrides

- `.agents/sow/specs/` is the only canonical product/application spec path. The removed repo-root `specs/` path must not be recreated as a compatibility alias.
- Generated frontend assets under `pkg/web/static/assets/*` and generated `pkg/web/static/index.html` are not source files; edit `ui/` and rebuild through the normal flow.
- Public serving must remain cache-first and cheap; public requests must not trigger upstream downloads or broad recomputation.
- Daemon startup availability matters; do not add expensive historical rescans or broad rebuilds to the startup critical path.
- Background work must be visible through the admin API/UI.
- `pkg/iprange` stays standalone and must not import other project packages.

Project SOW status: initialized
