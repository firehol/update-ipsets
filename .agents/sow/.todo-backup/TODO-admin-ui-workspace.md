# Admin UI Workspace Improvements

## TL;DR

Purpose: make the admin UI more useful as an operator console, not a decorative dashboard.

Costa asked to implement the improvements that are clearly required, install them, and then decide whether to keep the result. The work should preserve utility and reduce operational ambiguity.

## Analysis

Facts from current code:

- `ui/src/pages/admin.tsx` keeps health filters and the selected feed in component-local state. Browser back/forward cannot restore those states.
- `ui/src/components/admin/feeds-table.tsx` keeps search, filters, and sort in component-local state. Complex table views are not shareable and reset on reload.
- `ui/src/components/admin/feed-modal.tsx` opens feed details as a wide modal. This hides the list context and is not ideal for sequential feed review.
- `ui/src/components/admin/current-run.tsx` only renders the background-work panel when background tasks exist. Idle background state is invisible.
- `ui/src/components/admin/entity-integrity-panel.tsx` has a direct `Rebuild All` button and uses broad wording for mixed entity scopes.
- `ui/src/components/ui/command.tsx` exists, but admin does not use a command palette.
- `specs/admin-ui.md` must be updated because these changes affect admin behavior.

Relevant design guidance from `~/llmwiki/.agents/knowledge/Modern Interactive Admin: CRM Web Interface Design.md`:

- Admin UIs should preserve context like an OS/IDE workspace.
- Filters, sort, selected record, and table state should live in the URL.
- Drawers/master-detail are preferred over modals for record inspection.
- Background work and stale/in-progress states must be explicit.
- Command palettes are high-leverage for power operators.

## Decisions

- User delegated implementation of the changes believed required.
- No unresolved user decisions before the first iteration because the user explicitly asked to implement and then verify whether to keep it.

## Plan

1. Done: commit existing work first so the baseline is clean.
2. Done: add admin URL state helpers for filters, sort, and selected feed.
3. Done: move admin selected feed and health filters to URL parameters.
4. Done: move feed-table search, filter, and sort state to URL parameters.
5. Done: convert feed detail modal into a right-side drawer while keeping its existing content and actions.
6. Done: add an admin command palette for feed navigation and panel jumps.
7. Done: always render the background-work section with idle/running state.
8. Done: improve entity integrity grouping and add an explicit confirmation step for full rebuild.
9. Done: update `specs/admin-ui.md`.
10. Done: install with `./install.sh`.

## Implied Decisions

- Keep the backend API unchanged for this iteration.
- Do not add saved views, column picker, or virtualization yet; those need more product decisions and are not required for this first validation.
- Keep the existing visual language and density. This is a functional operator-console pass, not a redesign.

## Testing Requirements

- Done: `npm run build` in `ui/`.
- Done: `npm run lint` in `ui/`.
- Done: `go test ./...`.
- Done: `./install.sh` after implementation.
- Done: `curl http://localhost:18888/healthz`.
- Done: HTTP smoke checks for `/admin`, `/api/v1/status`, `/api/v1/sets`, and `/api/v1/admin/status`.
- Done: `git status --short` after install.

## Documentation Updates Required

- Update `specs/admin-ui.md` to state URL-backed admin view state, drawer details, persistent background-work visibility, and command palette expectations.
