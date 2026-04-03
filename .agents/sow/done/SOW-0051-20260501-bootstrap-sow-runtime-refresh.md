# SOW-0051 - Bootstrap SOW Runtime Refresh

## Status

Status: completed

Sub-state: completed; `AGENTS.md` now treats project-local runtime rules as authoritative and points setup/repair work at `bootstrap-sow`.

## Requirements

### Purpose

Keep this project's SOW system aligned with the new framework split: project `AGENTS.md` is the runtime authority, while the global `bootstrap-sow` skill is only for setup, review, repair, migration, and upgrades.

### User Request

The user approved reviewing this project first because it is the best current SOW implementation and should be used as the baseline for other project migrations.

### Assistant Understanding

Facts:

- `AGENTS.md` ends with `Project SOW status: initialized`.
- `bash ~/.agents/skills/bootstrap-sow/scripts/audit.sh` reports the SOW setup complete and clean.
- `AGENTS.md` still references `~/.agents/skills/sow/SKILL.md`.
- The project has strong area skills for Go, frontend, testing, operations, review, and content surfaces.
- The current `AGENTS.md` project skill index lists only the project-wide skills, not the Go/frontend area skills.

Working theory:

- This project needs a minimal runtime refresh, not a rewrite.
- Existing project-specific guidance should be preserved.
- The main repair is to remove the obsolete global runtime dependency and strengthen local skill hooks.

### Acceptance Criteria

1. `AGENTS.md` no longer points normal work at the old `~/.agents/skills/sow` path.
   - Verification: stale-path search.
2. `AGENTS.md` states that this project file is the runtime SOW authority and `bootstrap-sow` is for setup/review/repair/upgrade only.
   - Verification: inspect SOW System section.
3. `AGENTS.md` lists all relevant project skills, including Go and frontend area skills.
   - Verification: compare `.agents/skills/*/SKILL.md` to the Project Skills section.
4. Existing project-specific rules are preserved.
   - Verification: diff review.
5. Audit still passes.
   - Verification: `bash ~/.agents/skills/bootstrap-sow/scripts/audit.sh`.

## Analysis

Sources checked:

- `AGENTS.md`
- `.agents/skills/*/SKILL.md`
- `.agents/sow/done/SOW-0023-20260427-refresh-sow-skills.md`
- `bash ~/.agents/skills/bootstrap-sow/scripts/audit.sh`

Findings:

- `AGENTS.md` line near the working pattern says SOW init/re-review follows the global SOW skill.
- `AGENTS.md` SOW System says global policy lives in `~/.AGENTS.md` and `~/.agents/skills/sow/SKILL.md`.
- `AGENTS.md` "Where things live" points details at `~/.agents/skills/sow/SKILL.md`.
- Existing skills are not generic. The Go/frontend best-practice and behavioral-testing skills have strong trigger descriptions and detailed checklists, but the handbook does not index them.

## Implications And Decisions

Global framework decisions already recorded in the bootstrap-sow redesign SOW:

1. Minimal global pointer only.
2. Start with this project.
3. Strict preserve-by-default for project `AGENTS.md`.

No additional user decision is needed for this minimal repair because it preserves project-specific content and updates stale framework references.

## Plan

1. Update `AGENTS.md` SOW wording to point to `bootstrap-sow` only for setup/review/repair/upgrade.
   - Risk: low.

2. Add Go/frontend area skills to the Project Skills index.
   - Risk: low.

3. Run audit and stale-reference checks.
   - Risk: low.

4. Update this SOW validation and close it after the project audit passes.
   - Risk: low.

## Execution Log

### 2026-05-01

- Opened this SOW.
- Updated `AGENTS.md` working pattern and SOW System wording to reference `bootstrap-sow` only for setup/review/repair/migration.
- Added Go and frontend area skills to the Project Skills index.
- Ran audit, stale-reference search, whitespace check, and non-ASCII scan.

## Validation

Acceptance criteria evidence:

1. `AGENTS.md` no longer points normal work at the old `~/.agents/skills/sow` path.
   - Evidence: `rg -n "~/.agents/skills/sow(/|$)|\\.agents/skills/sow(/|$)|skills/sow(/|$)" AGENTS.md .agents/skills || true` returned no matches.
2. `AGENTS.md` states that project-local runtime rules are authoritative and `bootstrap-sow` is for setup/review/repair/migration.
   - Evidence: `AGENTS.md` now says this file is the runtime SOW authority for this project.
3. `AGENTS.md` lists all relevant project skills, including Go and frontend area skills.
   - Evidence: the Project Skills section now lists `go-best-practices`, `go-behavioral-testing`, `frontend-best-practices`, and `frontend-behavioral-testing`.
4. Existing project-specific rules are preserved.
   - Evidence: diff changed only stale SOW references and the Project Skills index; no project-specific repo rules were removed.
5. Audit still passes.
   - Evidence: `bash ~/.agents/skills/bootstrap-sow/scripts/audit.sh` reports `SOW initialization complete and clean`.

Additional checks:

- `git diff --check -- AGENTS.md .agents/sow/done/SOW-0051-20260501-bootstrap-sow-runtime-refresh.md` passed.
- `LC_ALL=C rg -n "[^\\x00-\\x7F]" AGENTS.md .agents/sow/done/SOW-0051-20260501-bootstrap-sow-runtime-refresh.md || true` returned no matches.

Specs update:

- No spec update. Reason: this SOW changed assistant runtime instructions only; product behavior, API, file layout, pipeline, website/admin behavior, integrity, memory, and compatibility contracts did not change.

Project skills update:

- No project skill update. Reason: the existing Go/frontend/project skills were already specific and hook-based; the missing hook was the `AGENTS.md` index, now repaired.

Lessons:

- Project `AGENTS.md` must index area skills, not only project-wide skills, because skill descriptions are the practical hook for future assistants.
- Project `AGENTS.md` should not point normal project work at a global SOW skill; it should be the runtime authority and reserve `bootstrap-sow` for setup/review/repair/migration.

## Outcome

Completed.

- Updated `AGENTS.md`.
- Added no new project skills.
- Changed no product specs.
- Preserved existing project-specific rules.

## Lessons Extracted

- Baseline review result: this project is a good SOW implementation structurally. Its main repair was stale global-skill wording and incomplete area-skill indexing.

## Followup

None yet.
