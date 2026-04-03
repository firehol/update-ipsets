# SOW-0071 - Frontend Interactive Accessibility Cleanup

## Status

Status: closed

Sub-state: completed

## Requirements

### Purpose

Ensure mouse-clickable frontend interactions have keyboard-accessible equivalents, clear roles/names, and visible focus behavior.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- `ui/src/components/admin/integrity-panel.tsx:220` has a clickable table row that toggles detail expansion without keyboard semantics.
- `ui/src/components/editorial/data-table.tsx:232` has sortable `<th>` elements with `onClick` rather than a button/`aria-sort` pattern.
- `ui/src/components/home/home-explorer-view-treemap.tsx:157` has clickable SVG `<g>` leaves without keyboard navigation.
- `ui/src/components/admin/feeds-table-body.tsx:286` keeps a mouse-clickable row, but it also has a nested named button at `ui/src/components/admin/feeds-table-body.tsx:291`; this specific case needs verification before changing.
- Audit cycle 2 found some keyboard tests call `.focus()` directly instead of
  proving controls are reachable through realistic tab navigation.

Inferences:

- The feed-row case may already have a keyboard equivalent through the nested button; the row itself should not be blindly made focusable around nested controls.
- Integrity rows, sortable headers, and SVG treemap leaves have clearer accessibility gaps.

Unknowns:

- None remaining for this implementation cleanup.

### Acceptance Criteria

- Every mouse-clickable table row/header/chart element either becomes a native button/link or has an accessible keyboard equivalent.
- Sortable data tables expose sorting state through accessible labels or `aria-sort`.
- Treemap feed navigation is keyboard-accessible without requiring SVG internals if a better fallback exists.
- Tests cover keyboard activation and visible outcomes.
- Keyboard tests for reachable controls use `userEvent.tab()` or equivalent
  user navigation where tab reachability is part of the contract; direct
  `.focus()` remains only for cases where programmatic focus itself is the
  contract.
- Axe checks are added or updated for touched surfaces where reliable.

## Analysis

Sources checked:

- `ui/src/components/admin/feeds-table-body.tsx`
- `ui/src/components/admin/integrity-panel.tsx`
- `ui/src/components/editorial/data-table.tsx`
- `ui/src/components/home/home-explorer-view-treemap.tsx`
- `project-frontend-best-practices` and `project-reviewing` skills.

Current state:

- Some interactions remain mouse-first or role-incomplete.

Risks:

- Keyboard users cannot reach visible actions.
- Naively adding `tabIndex` to rows with nested controls can create invalid or confusing focus behavior.

## Implications And Decisions

No user decision was required. The work did not introduce a new product feature
or operator-facing policy; it converted existing mouse interactions to native
or keyboard-reachable controls.

Assistant implementation decision:

1. Treemap keyboard strategy
   - Chosen: keep the SVG as the visual chart and overlay native HTML links for
     leaf tiles.
   - Reason: this preserves the existing treemap UX, avoids adding a new public
     surface, and avoids inconsistent SVG focus behavior in browsers.

## Plan

1. Inventory clickable non-button/non-link UI elements. Done.
2. Classify each as needing native control conversion, keyboard equivalent, or explicit decorative treatment. Done.
3. Fix integrity table, data-table sorting, and treemap navigation first. Done.
4. Verify admin feed rows do not regress after any changes. Done.
5. Add behavioral tests and axe checks. Done.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Replaced sortable `DataTable` header click handlers with native sort buttons
  inside `th` cells and exposed `aria-sort`.
- Replaced admin integrity clickable rows with explicit expand/collapse buttons
  that expose `aria-expanded` and detail row ownership.
- Removed the admin feeds table row click target and kept the named feed button
  as the single drawer-opening control.
- Replaced homepage treemap SVG click navigation with native HTML tile links
  over the visual SVG chart.
- Added/updated keyboard behavioral coverage for sorted tables, integrity
  expansion, admin feed table sorting semantics, and treemap tile navigation.

## Validation

Acceptance criteria evidence:

- Mouse-clickable table/header/chart interactions touched by this SOW now have
  native buttons or links:
  - `ui/src/components/editorial/data-table.tsx`
  - `ui/src/components/admin/integrity-panel.tsx`
  - `ui/src/components/admin/feeds-table-body.tsx`
  - `ui/src/components/home/home-explorer-view-treemap.tsx`
- Sortable data tables expose `aria-sort` and named sort buttons:
  - `ui/src/components/editorial/data-table.tsx`
  - `ui/src/components/admin/feeds-table-body.tsx`
- Treemap feed navigation is keyboard-accessible through native HTML links over
  the SVG surface.
- Keyboard and axe coverage was added or updated in:
  - `ui/src/components/editorial/data-table.test.tsx`
  - `ui/src/components/admin/feeds-table.test.tsx`
  - `ui/src/pages/admin-actions.test.tsx`
  - `ui/e2e/smoke.spec.ts`

Tests or equivalent validation:

- PASS: `pnpm --dir ui test -- ui/src/components/editorial/data-table.test.tsx ui/src/pages/admin-actions.test.tsx ui/src/components/admin/feeds-table.test.tsx`
- PASS: `pnpm --dir ui test:e2e`
- PASS: `pnpm --dir ui test`
- PASS: `pnpm --dir ui lint`
- PASS: `git diff --check`

Real-use evidence:

- Browser e2e test tabs through the homepage to a treemap tile link and presses
  Enter, then verifies navigation to `/ipsets/alpha_feed`.
- Admin integrity test tabs to the expand control, presses Enter, verifies the
  expanded state and detail content, and runs axe on the page.

Reviewer findings:

- Frontend best-practices review found clickable rows/charts are not consistently keyboard-accessible.
- Iterative audit cycle 2 found keyboard tests need tab-reachability coverage,
  not only direct `.focus()` setup.

Same-failure scan:

- Completed scan for `onClick`, `onKeyDown`, `tabIndex`, `role="button"`, and
  `role="link"` in `ui/src/**/*.tsx`.
- Remaining graphical handlers in ASN/geo chart components already expose
  link roles, keyboard handlers, and `tabIndex`, and are covered by the
  visualization/browser validation SOWs.
- Shared clickable list/card helpers reviewed in this pass render native
  `<button>` elements when an action exists.

Artifact maintenance gate:

- AGENTS.md: no update needed; no workflow rule changed.
- Runtime project skills: no update needed; existing frontend accessibility
  guidance already covers this pattern.
- Specs: no update needed; public behavior is unchanged apart from keyboard
  reachability of existing interactions.
- End-user/operator docs: no update needed; no documented workflow changed.
- End-user/operator skills: no update needed; no exported skill changed.
- SOW lifecycle: moved from pending to current for implementation, then to done
  after validation.

Specs update:

- Not needed.

Project skills update:

- Not needed.

End-user/operator docs update:

- Not needed.

End-user/operator skills update:

- Not needed.

Lessons:

- For SVG-based public visualizations that navigate to application routes,
  native HTML links over the visual surface are more reliable than relying on
  browser-specific SVG focus behavior.
- E2E tab-reachability checks must account for the real DOM order, including
  filter rails and other focusable controls before the target surface.

Follow-up mapping:

- None.

## Outcome

Completed.

## Lessons Extracted

- Prefer native controls for existing interactive semantics before adding
  custom keyboard handling.
- If a table row already contains a named action button, remove duplicate row
  click behavior rather than adding focus semantics to the row.

## Followup

None.
