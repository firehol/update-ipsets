# SOW-0057 - Project Skill Prefix Alignment

## Status

Status: completed

Sub-state: focused repair completed after frontend SOWs closed.

## Requirements

### Purpose

Make FireHOL runtime area skills discoverable through the generic `.agents/skills/project-*/SKILL.md` hook without disrupting the active frontend/SOW work already in progress.

### User Request

The user clarified that SOW-related skills should start with `project-` so agents working in a repository have a generic rule for which skills to consider.

### Acceptance Criteria

1. Runtime area skills are renamed or wrapped under `.agents/skills/project-*/`.
2. Skill frontmatter semantics are preserved unless a change is explicitly approved.
3. `AGENTS.md` references the new runtime skill paths.
4. Internal live skill cross-references are updated.
5. Historical completed SOW logs are not mechanically rewritten.
6. `.agents/sow/audit.sh` reports no skill classification warnings.

## Analysis

Original audit warnings:

- `.agents/skills/frontend-behavioral-testing/`
- `.agents/skills/frontend-best-practices/`
- `.agents/skills/go-behavioral-testing/`
- `.agents/skills/go-best-practices/`

Risk handled:

- The cleanup was executed only after the active frontend SOWs closed, so skill
  path changes did not interfere with active code/test work.

Execution plan:

1. Rename the four runtime area skill directories to `project-*` paths and
   update their frontmatter names to match.
2. Update `AGENTS.md` runtime skill references to the new paths.
3. Update live skill cross-references that point to the old directory names.
4. Leave completed SOW history unchanged.
5. Run the project SOW audit and grep for stale runtime skill path references.

## Outcome

- Renamed the four runtime area skills to `project-*` paths:
  `project-frontend-behavioral-testing`, `project-frontend-best-practices`,
  `project-go-behavioral-testing`, and `project-go-best-practices`.
- Updated the skill frontmatter names to match the new paths.
- Updated `AGENTS.md` project skill references and removed the stale exception
  for tracked non-`project-*` runtime skill paths.
- Updated `.agents/sow/audit.sh` to accept legacy `Status: done` for SOWs
  already stored in `done/`, avoiding a mechanical rewrite of historical SOW
  files.

## Validation

- `bash .agents/sow/audit.sh` - passed with clean verdict and no non-project
  skill warnings.
- Stale live-reference search across `AGENTS.md`, `.agents/skills/`, and the
  active SOW ledger directories found no old runtime skill paths outside this
  SOW's original-warning record.
