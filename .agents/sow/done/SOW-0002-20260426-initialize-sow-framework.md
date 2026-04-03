# SOW-0002 | 2026-04-26 | initialize-sow-framework

## Status

completed
stale current SOW closed after audit verified initialization is complete

## Requirements

Given the repo does not contain `Project SOW status: initialized` in `AGENTS.md`, when initialization completes, then the repo must contain the canonical SOW directories, project skills, migrated SOW files, and final AGENTS marker.

Given root `TODO-*.md` files are ignored by git, when they are migrated, then originals must first be backed up under `.agents/sow/.todo-backup/` and must not be removed from the root without explicit approval.

Given the current `AGENTS.md` has useful repo rules, when it is rewritten, then concise repo-specific rules must be preserved and the full diff must be shown before writing.

## Analysis

Sources consulted:

- `~/.agents/skills/sow/SKILL.md`
- `~/.agents/skills/sow/sow-init.md`
- `~/.agents/skills/sow/sow-file-format.md`
- `~/.agents/skills/sow/sow-project-skills.md`
- `~/.agents/skills/sow/sow-todo-migration.md`
- current `AGENTS.md`, README, Makefile, CI, manifests, specs, and root TODO files

Current state before execution:

- `AGENTS.md` exists but lacks SOW canonical headings and marker.
- `.agents/sow/` did not exist.
- `.agents/skills/` existed but had no `project-*` skills.
- Root TODO files were ignored by `.gitignore`.

Delegated Phase A findings:

- Project role: maintainer/local-maintainer based on all reachable commits authored by Costa and no configured remote/CODEOWNERS evidence.
- Product specs already live under `.agents/sow/specs/*.md` and must remain authoritative.
- CI validates Go only; UI build/lint exists but is not wired into CI.
- Root `go test ./...` does not cover `tools/dronebl2ipsets`.

## Implications and decisions

- Costa approved full analysis, heavy init, and steering mode.
- Costa approved keeping `.agents/sow/specs/*.md` as product truth.
- Costa approved creating `project-coding`, `project-reviewing`, `project-testing`, and `project-operations`.
- Costa approved migrating completed TODOs to `done`, active work to `current`, and asking again before root TODO removal.
- Costa approved one active `release-readiness` SOW with future phases grouped as followups.
- Costa approved showing the `AGENTS.md` diff before writing it.

## Plan

1. Create `.agents/sow/{specs,pending,current,done}` and indexes.
2. Back up root TODO files under `.agents/sow/.todo-backup/`.
3. Create project skills from observed repo conventions.
4. Create SOWs for bootstrap specs, SOW initialization, release readiness, and completed migrated TODO work.
5. Prepare the rewritten `AGENTS.md` draft and show the diff for approval.
6. After approval, write `AGENTS.md`, remove root TODOs only if approved, append the final marker, and run the SOW audit.

## Execution log

2026-04-26:

- Phase A completed with delegated analysis agents.
- Phase B recommendation package approved by Costa with `all`.
- Created SOW directories and copied root TODO backups to `.agents/sow/.todo-backup/`.
- Created initial indexes, project skills, and migrated SOW files.
- Wrote the approved SOW `AGENTS.md` rewrite after preserving
  `AGENTS.md.pre-sow.bak`.
- Removed the approved root TODO files after verifying their backups exist.
- Final marker was appended and the SOW audit now reports initialization clean.
- 2026-04-26 cleanup: Costa approved closing this stale current SOW after later
  completed SOWs proved the repo was already initialized and operating under the
  SOW system.

## Validation

- [x] Acceptance criteria evidence

  `AGENTS.md` contains `Project SOW status: initialized`. Canonical SOW
  directories exist under `.agents/sow/`. Project skills exist under
  `.agents/skills/project-*`. Root `TODO-*.md` files are absent and preserved in
  `.agents/sow/.todo-backup/`.

- [x] Real-use validation evidence

  `bash ~/.agents/skills/sow/scripts/audit.sh` reports the repo initialized and
  clean. Later SOWs have been created, moved, completed, and committed under the
  initialized SOW workflow.

- [x] Cross-model reviewer findings (logged + addressed)

  Phase A used delegated analysis before initialization. No open review findings
  remain in this SOW.

- [x] Lessons extracted (or "none, reasoning: ...")

  Lessons were captured in the created project skills. No additional lesson was
  found during stale-SOW cleanup.

- [x] Same-failure-at-other-scales check

  SOW audit confirms only this stale initialization SOW remained in `current/`.
  Other completed SOWs are already in `done/`, and pending work is under
  `pending/`.

## Outcome

SOW initialization is complete. The repo has canonical SOW directories,
project skills, preserved TODO backups, no root TODO files, the initialized
marker in `AGENTS.md`, and a clean SOW audit.

## Lessons extracted

None during stale-SOW cleanup. The initialization work already created the
project skills that preserve the relevant operating lessons.
