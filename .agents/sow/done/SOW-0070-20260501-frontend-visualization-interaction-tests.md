# SOW-0070 - Frontend Visualization Interaction Tests

## Status

Status: completed

Sub-state: validated

## Requirements

### Purpose

Add behavioral coverage for feed-detail visualization controls and accessible chart/map interaction contracts.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- Current feed-detail tests mostly assert that sections and some chart surfaces render.
- Feed-detail visualization controls include ASN provider tabs, geo provider tabs, bogon/provider tabs, comparison view tabs, and chart/map navigation.
- Playwright smoke currently checks one SVG chart has visible dimensions.
- The frontend behavioral-testing skill says charts/maps should be tested through accessible summaries/controls in jsdom and through a small number of browser checks where pixels matter.

Inferences:

- The highest value is not pixel-perfect chart testing; it is verifying controls change visible data, keyboard activation works, and browser smoke catches blank chart surfaces.

Unknowns:

- Which chart interactions are considered part of the public contract versus decorative exploration.

### Acceptance Criteria

- Tests exercise provider/view tabs for ASN, geo, bogon/critical/comparison sections where practical.
- Keyboard activation and accessible names are asserted for visualization controls.
- Browser smoke adds nonblank/visible assertions for key SVG/map surfaces if jsdom cannot verify them honestly.
- Tests avoid chart internals, snapshots, and data-testid selectors.

## Analysis

Sources checked:

- `ui/src/pages/feed-detail.test.tsx`
- `ui/src/components/feed-detail/section-asn.tsx`
- `ui/src/components/feed-detail/section-geo.tsx`
- `ui/src/components/feed-detail/section-comparison.tsx`
- `ui/e2e/smoke.spec.ts`

Current state:

- Visualization rendering has some coverage; interaction coverage is thin.

Risks:

- Provider tabs, keyboard access, or chart rendering can break without failing tests.
- Over-testing SVG internals would create brittle tests.

## Implications And Decisions

No implementation decision is taken in this pending SOW.

Recommended starting decision:

1. Coverage shape
   - A. Test visualizations only through Playwright screenshots.
     - Pros: catches pixels.
     - Cons: slower and brittle.
   - B. Test controls in Vitest and add minimal browser nonblank checks. Recommended.
     - Pros: fast behavioral coverage plus honest pixel smoke.
     - Cons: requires fixture discipline.
   - C. Skip visualization interactions.
     - Pros: avoids flaky tests.
     - Cons: leaves high-risk UI unprotected.

## Plan

1. Inventory visualization controls and public labels.
2. Add MSW fixtures with multiple providers/data states.
3. Add Vitest user-event tests for tab/view switching and keyboard behavior.
4. Add or extend Playwright smoke for key SVG/map nonblank surfaces.
5. Run UI and browser gates.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Moved to current for autonomous implementation because this is frontend
  behavioral test coverage, not a product feature decision.
- Inventoried feed-detail visualization controls in ASN, Geo, Comparison,
  Bogons, and Critical Infrastructure sections.
- Added `ui/src/components/feed-detail/visualization-interactions.test.tsx`
  with MSW-backed user-event coverage for:
  - ASN provider tabs and Treemap/Bubble/List view tabs.
  - Geo provider tabs and Map/List view tabs.
  - Comparison List/Sankey/Network view tabs.
- Extended `ui/e2e/smoke.spec.ts` so the production bundle checks that ASN
  Bubble, Overlap Sankey, and Overlap Network SVG surfaces are visible, sized,
  and contain rendered SVG marks.
- Kept Bogons/Critical Infrastructure out of the visualization-tab test set
  because they expose tables/subsections, not visualization provider/view tabs;
  their table accessibility belongs to the separate interactive-accessibility
  SOW.

## Validation

Acceptance criteria evidence:

- `ui/src/components/feed-detail/visualization-interactions.test.tsx` exercises
  provider/view tab behavior through real components, TanStack Query, MSW, and
  `userEvent`.
- Keyboard activation is covered for ASN provider/view tabs, Geo provider/view
  tabs, and Comparison view tabs with focus and `aria-selected` assertions.
- `ui/e2e/smoke.spec.ts` checks browser-rendered SVG visualization surfaces by
  accessible image name, dimensions, and generic rendered mark presence.
- Tests avoid snapshots and `data-testid` selectors.

Tests or equivalent validation:

- `pnpm --dir ui test -- ui/src/components/feed-detail/visualization-interactions.test.tsx ui/src/pages/feed-detail.test.tsx` passed.
- `pnpm --dir ui test:e2e` passed.
- `pnpm --dir ui test` passed.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui build` passed as part of `pnpm --dir ui test:e2e`.
- `git diff --check` passed.

Real-use evidence:

- Playwright loaded the production bundle through the project static server and
  exercised the feed-detail route with fixture API responses.

Reviewer findings:

- Frontend behavioral-testing review found visualization interaction coverage is thin.

Same-failure scan:

- Scanned feed-detail section controls and found:
  - ASN and Geo have provider tabs plus visualization view tabs.
  - Comparison has visualization view tabs.
  - Bogons and Critical Infrastructure expose tables/subsections rather than
    visualization tabs.
- Existing chart/map keyboard navigation gaps around SVG links remain mapped to
  `SOW-0071-20260501-frontend-interactive-accessibility-cleanup.md`.

Artifact maintenance gate:

- AGENTS.md: no update needed; workflow and guardrails unchanged.
- Runtime project skills: no update needed; existing frontend behavioral test
  guidance already covers user-event, MSW, and browser checks for visual
  surfaces.
- Specs: no update needed; this added coverage for existing public behavior.
- End-user/operator docs: no update needed; no user-facing docs changed.
- End-user/operator skills: no update needed; no exported/operator skill changed.
- SOW lifecycle: pending SOW moved to current, completed after validation, and
  ready to move to done; no deferred work remains.

Specs update:

- Not needed.

Project skills update:

- Not needed.

End-user/operator docs update:

- Not needed.

End-user/operator skills update:

- Not needed.

Lessons:

- For chart-heavy sections, keep most behavior tests in Vitest against visible
  controls and reserve Playwright for a small number of production-bundle SVG
  render checks.

Follow-up mapping:

- Keyboard accessibility cleanup for SVG map/chart links remains in
  `SOW-0071-20260501-frontend-interactive-accessibility-cleanup.md`.

## Outcome

Completed. Feed-detail visualization provider/view controls now have behavioral
coverage, and browser smoke verifies the main SVG graph surfaces render
nonblank output in the production bundle.

## Lessons Extracted

- jsdom is appropriate for tab/view behavior but not for `ParentSize`-driven
  SVG chart rendering; use Playwright for the latter.

## Followup

`SOW-0071-20260501-frontend-interactive-accessibility-cleanup.md` remains the
tracked place for SVG map/chart keyboard accessibility cleanup.
