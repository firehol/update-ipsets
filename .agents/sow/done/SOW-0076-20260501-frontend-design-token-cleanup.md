# SOW-0076 - Frontend Design Token Cleanup

## Status

Status: closed

Sub-state: completed

## Requirements

### Purpose

Keep frontend styling maintainable under the project Tailwind v4/design-token
contract by removing ad-hoc colors and stale utility idioms where safe.

### User Request

Review project quality against the four named project skills, identify gaps,
create SOWs for actionable improvements, and iterate the audit until valid
findings are fixed or tracked.

### Assistant Understanding

Facts:

- Audit cycle 2 found hardcoded hex colors in UI source such as
  `ui/src/lib/feed-health.ts`, `ui/src/components/admin/admin-layout.tsx`,
  `ui/src/components/admin/heartbeat.tsx`,
  `ui/src/components/home/ip-lookup-country-map.tsx`, and
  `ui/src/components/feed-detail/geo-map.tsx`.
- The frontend best-practices skill says design tokens should own theme values.
- Audit cycle 2 also found pre-rename Tailwind utility names such as
  `shadow-sm`, `rounded-sm`, and `outline-none` in active components.
- Iterative audit cycle 5 found typography cleanup was under-mapped:
  `ui/src/index.css` still has viewport-scaled display sizes and negative
  letter-spacing for editorial display classes.

Inferences:

- Some literals may be data-visualization palette values and need deliberate
  tokenization rather than blind replacement.
- Tailwind utility cleanup is low risk when visual diffs and lint/build remain
  green.

Unknowns:

- None remaining.

### Acceptance Criteria

- Inventory hardcoded colors and classify each as design token, semantic data
  palette, third-party interop value, or acceptable exception.
- Replace ad-hoc UI colors with existing or new semantic tokens where
  appropriate.
- Inventory editorial typography scale/tracking values such as
  `ui/src/index.css` display classes and classify them as design-system tokens,
  required visualization/editorial exceptions, or cleanup targets.
- Remove viewport-scaled font sizing and negative letter-spacing from ordinary
  UI surfaces unless a specific responsive fitting component or visualization
  interop requirement justifies the exception.
- Modernize stale Tailwind utility names in touched surfaces when the Tailwind
  v4 equivalent is clear.
- Run `pnpm --dir ui lint`, `pnpm --dir ui build`, and relevant UI tests.

## Analysis

Sources checked:

- `project-frontend-best-practices`
- Cycle-2 frontend best-practices findings

Current state:

- Styling is mostly tokenized, but several source files still bypass the token
  system.

Risks:

- Blind token replacement can damage map/chart contrast or category semantics.
- Utility churn without validation can create subtle visual regressions.

## Implications And Decisions

No user decision was required. This work did not change product behavior,
routes, or operator policy.

Assistant implementation decisions:

1. Status colors
   - Chosen: add semantic `status` Tailwind tokens backed by CSS variables and
     replace ad-hoc admin/feed-health hex classes with those tokens.
   - Reason: the same operational colors were repeated across admin and public
     health surfaces.
2. Chart/map colors
   - Chosen: use existing chart tokens for the IP lookup map and treemap
     fallback color.
   - Explicit exception: keep `GeoMap`'s d3-scale hex palette local because
     `scaleSqrt` interpolates parseable color endpoints and cannot consume CSS
     custom properties.
3. Typography
   - Chosen: replace viewport-scaled display sizes and negative letter-spacing
     with fixed breakpoint steps and `letter-spacing: 0`.
   - Reason: this matches the frontend project rule that font size must not
     scale directly with viewport width.

## Plan

1. Inventory color literals and stale utility classes. Done.
2. Pick a conservative token strategy for ordinary UI and visualization
   palettes. Done.
3. Patch focused surfaces and avoid generated assets. Done.
4. Validate lint/build/tests and inspect representative UI where needed. Done.

## Execution Log

### 2026-05-01

- Created from iterative audit cycle 2.
- Corrected bad evidence paths after iterative audit cycle 4. The country map
  is under `ui/src/components/home/`, and the feed-detail geography map is
  under `ui/src/components/feed-detail/`.
- Added typography scale/tracking ownership after iterative audit cycle 5 found
  it was not covered by the color/utility cleanup wording.
- Added status CSS variables and Tailwind `status.*` color tokens.
- Replaced repeated admin/feed-health status hex classes with semantic status
  classes.
- Added chart `context` and `tooltipShadow` fields to the typed chart theme.
- Moved IP lookup map and treemap fallback colors to chart tokens.
- Replaced viewport-scaled display typography and negative tracking in
  `ui/src/index.css` and `ui/src/components/home/home-hero.tsx`.

## Validation

Acceptance criteria evidence:

- Hardcoded status colors were replaced with semantic tokens in:
  - `ui/src/lib/feed-health.ts`
  - `ui/src/lib/admin-format.ts`
  - `ui/src/components/admin/admin-layout.tsx`
  - `ui/src/components/admin/heartbeat.tsx`
  - `ui/src/components/admin/feed-modal-*`
  - `ui/src/components/admin/feeds-table-*`
  - `ui/src/components/admin/integrity-panel.tsx`
  - `ui/src/components/admin/entity-integrity-panel.tsx`
- Chart token usage was expanded in:
  - `ui/src/lib/chart-theme.ts`
  - `ui/src/components/home/ip-lookup-country-map.tsx`
  - `ui/src/components/home/home-explorer-view-treemap.tsx`
  - `ui/src/components/feed-detail/section-behavior.tsx`
  - `ui/src/components/feed-detail/section-retention.tsx`
- Typography cleanup:
  - `ui/src/index.css` display classes no longer use `vw` font-size scaling or
    negative letter spacing.
  - `ui/src/components/home/home-hero.tsx` no longer uses a `clamp(...vw...)`
    text class or negative tracking.
- Remaining color literals are classified:
  - `ui/src/components/feed-detail/geo-map.tsx`: accepted local visualization
    palette required by d3 color interpolation.
  - `ui/src/components/feed-detail/hero.tsx` and
    `ui/src/components/feed-detail/provider-tabs.tsx`: fixed elevation/hero
    visual effects, not repeated status/data colors.
  - `ui/src/index.css`: design-token definitions and comments.
- `rounded-sm` and `outline-none` were not blindly replaced. They are still
  valid in this codebase where they preserve the configured radius scale and
  are paired with focus-visible rings/outlines.

Tests or equivalent validation:

- PASS: `pnpm --dir ui lint`
- PASS: `pnpm --dir ui test`
- PASS: `pnpm --dir ui build`
- PASS: `pnpm --dir ui test:e2e`
- PASS: `git diff --check`

Reviewer findings:

- Frontend best-practices review found hardcoded colors and stale Tailwind
  utility idioms not covered by an existing pending SOW.

## Outcome

Completed.

## Artifact maintenance gate

- AGENTS.md: no update needed; no workflow rule changed.
- Runtime project skills: no update needed; existing design-token guidance
  already covers this pattern.
- Specs: no update needed; behavior and routes unchanged.
- End-user/operator docs: no update needed; no user-facing workflow changed.
- End-user/operator skills: no update needed.
- SOW lifecycle: moved from pending to current for implementation, then to done
  after validation.

## Lessons Extracted

- Treat repeated operational colors as semantic status tokens, not one-off
  arbitrary classes.
- Do not replace visualization palettes blindly when a rendering library needs
  parseable colors for interpolation.

## Followup

None.
