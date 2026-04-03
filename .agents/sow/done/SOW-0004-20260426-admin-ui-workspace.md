# SOW-0004 | 2026-04-26 | admin-ui-workspace

## Status

completed
implemented before SOW initialization and migrated with evidence

## Requirements

Given the admin UI is an operator console, when operators inspect feeds and background work, then the UI should preserve context, expose background state, and avoid ambiguous destructive actions.

Given feed review often needs navigation, when filters, sort, selected feed, and search are changed, then meaningful state should be URL-backed.

Given full entity rebuild can be expensive, when the operator uses `Rebuild All`, then the action should be explicit and visible.

## Analysis

Original source:

- `.agents/sow/.todo-backup/TODO-admin-ui-workspace.md`

Evidence:

- Commit `7d7e728 Improve admin workspace usability`.
- URL-backed admin view state exists in `ui/src/pages/admin.tsx`.
- Feed table URL state exists in `ui/src/components/admin/feeds-table.tsx`.
- Feed details were moved to drawer behavior in `ui/src/components/admin/feed-modal.tsx`.
- Command palette exists in `ui/src/components/admin/admin-command-palette.tsx`.
- Background work section is always represented in `ui/src/components/admin/current-run.tsx`.
- Entity rebuild confirmation exists in `ui/src/components/admin/entity-integrity-panel.tsx`.
- Admin contract was updated in `.agents/sow/specs/admin-ui.md`.

## Implications and decisions

- The backend API was intentionally kept stable for this iteration.
- Saved views, column picker, and virtualization were deferred.
- Existing visual language and density were preserved.

## Plan

Single-unit implementation, no chunking - reasoning: this work was already completed before SOW initialization.

## Execution log

2026-04-26:

- Migrated completed TODO into SOW history.
- Original TODO preserved at `.agents/sow/.todo-backup/TODO-admin-ui-workspace.md`.

## Validation

- [x] Acceptance criteria evidence
  - Evidence is the implementation commit and files listed in Analysis.
- [x] Real-use validation evidence
  - Original TODO records `npm run build`, `npm run lint`, `go test ./...`, `./install.sh`, and HTTP smoke checks as completed.
- [x] Cross-model reviewer findings (logged + addressed)
  - N/A - completed before SOW system; no active SOW review cycle existed.
- [x] Lessons extracted (or "none, reasoning: ...")
  - Lesson captured in `project-operations`: background work must be visible in admin status/UI.
- [x] Same-failure-at-other-scales check
  - Admin URL state and background-work visibility were captured as project skill review concerns.

## Outcome

Admin UI gained URL-backed state, command palette, drawer feed details, persistent background-work visibility, and explicit rebuild confirmation.

## Lessons extracted

- Operator-visible background work is a project rule; recorded in `project-operations/SKILL.md`.
