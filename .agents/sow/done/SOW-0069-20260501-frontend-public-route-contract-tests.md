# SOW-0069 - Frontend Public Route Contract Tests

## Status

Status: completed

Sub-state: validated

## Requirements

### Purpose

Add black-box UI tests for public route contracts that remain uncovered after the first page-level test batches.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- Route-level tests now cover homepage, feed detail success, entity detail pages, and admin action success paths.
- Public index and utility routes remain thin or uncovered: countries index, ASNs index, maintainers index, methodology index/detail, `/catalog` redirect, and not-found route.
- Page-level axe coverage is partial.
- Audit cycle 2 found an existing public-surface test with multiple assertions
  inside one `waitFor`, which can hide which observable state actually failed.
- Iterative audit cycle 5 identified exact existing files behind those broad
  statements: `ui/src/pages/entities.test.tsx` has page-level public entity
  tests without axe coverage, and
  `ui/src/components/ip-search/ip-search-surface.test.tsx` batches IP/details
  request assertions inside one `waitFor`.

Inferences:

- These routes are part of the public website contract and can regress at the router/query/rendering boundary.

Unknowns:

- Whether methodology detail test fixtures should use real static markdown-derived payloads or minimal MSW HTML payloads.

### Acceptance Criteria

- Tests cover countries index, ASNs index, maintainers index, methodology index/detail, `/catalog` redirect, and not-found route where practical.
- Tests use MSW/network seam or router-level behavior, not mocked hooks or children.
- Page-level tests include `vitest-axe` where jsdom can evaluate reliably.
- Add or explicitly justify page-level axe coverage for existing
  `ui/src/pages/entities.test.tsx` country/ASN/maintainer route tests.
- Tests assert visible roles/text/links and URL outcomes, not component internals.
- Replace the multi-assertion `waitFor` in
  `ui/src/components/ip-search/ip-search-surface.test.tsx` with one readiness
  wait followed by normal assertions, or record why the request-observation
  seam requires the current shape.
- Touched tests avoid batching multiple independent expectations inside one
  `waitFor`; waits target one observable readiness condition, followed by
  normal assertions.

## Analysis

Sources checked:

- `ui/src/App.tsx`
- `ui/src/pages/entities.test.tsx`
- `ui/src/pages/feed-detail.test.tsx`
- `ui/src/pages/home.test.tsx`
- `project-frontend-behavioral-testing` skill.

Current state:

- Public route coverage improved in SOW-0052 and SOW-0053, but several stable routes remain untested.

Risks:

- Query, routing, sanitization, or link regressions can ship for public index/methodology routes.
- Adding too many low-value tests can slow UI test feedback; keep one high-signal test per route family.

## Implications And Decisions

No implementation decision is taken in this pending SOW.

Recommended starting decision:

1. Test layer
   - A. Component/page tests through Vitest + MSW. Recommended.
     - Pros: fast and black-box enough for route/data rendering.
     - Cons: not real browser layout.
   - B. Playwright tests for all public routes.
     - Pros: real browser.
     - Cons: slower and duplicates component coverage.
   - C. Skip index/utility routes.
     - Pros: smaller suite.
     - Cons: leaves published route contracts uncovered.

## Plan

1. Extend MSW page scenarios for public index/methodology routes.
2. Add route-level tests with visible assertions and axe checks.
3. Add redirect/not-found assertions through router behavior.
4. Run UI tests/lint/build and relevant project gates.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Moved to current for autonomous implementation because this is frontend test
  coverage and implementation hygiene, not a product feature decision.
- Added MSW fixtures for public country, ASN, and maintainer index routes.
- Added black-box route tests for countries index, ASNs index, maintainers
  index, methodology detail, `/catalog` redirect, and not-found recovery.
- Added `vitest-axe` checks to public entity index/detail tests and
  methodology/not-found route tests.
- Replaced the multi-assertion `waitFor` in the IP search surface test with a
  single readiness assertion followed by a normal request-parameter assertion.
- Fixed invalid status-dot ARIA exposed by the new axe coverage by giving
  labeled dot spans an explicit image role across the affected public surfaces.

## Validation

Acceptance criteria evidence:

- `ui/src/pages/entities.test.tsx` covers `/countries`, `/asns`,
  `/maintainers`, `/countries/:code`, `/asns/:asn`, and
  `/maintainers/:slug` through rendered page components, router paths, and MSW
  API payloads.
- `ui/src/pages/methodology.test.tsx` covers methodology index and detail
  payload rendering through the API seam.
- `ui/src/pages/public-routes.test.tsx` covers the `/catalog` redirect URL
  outcome and the not-found route recovery link.
- `ui/src/components/ip-search/ip-search-surface.test.tsx` now waits on one
  observable request readiness condition before asserting the second query
  parameter.
- `vitest-axe` now covers the entity route tests, methodology tests, and
  not-found route test where jsdom can evaluate them reliably.

Tests or equivalent validation:

- `pnpm --dir ui test -- ui/src/pages/entities.test.tsx ui/src/pages/methodology.test.tsx ui/src/pages/public-routes.test.tsx ui/src/components/ip-search/ip-search-surface.test.tsx` passed.
- `pnpm --dir ui test` passed.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui build` passed.
- `git diff --check` passed.

Real-use evidence:

- The route tests exercise the public API/query/rendering seam with MSW payloads
  and assert user-visible headings, links, route outcomes, and recovery links.

Reviewer findings:

- Frontend behavioral-testing review found several public contract routes have no behavioral coverage.
- Iterative audit cycle 2 found a public-surface test wait pattern that should
  be cleaned while extending this route test layer.
- Iterative audit cycle 5 required exact mapping for
  `ui/src/pages/entities.test.tsx` axe coverage and
  `ui/src/components/ip-search/ip-search-surface.test.tsx` waitFor cleanup.

Same-failure scan:

- Compared `ui/src/App.tsx` public route table with page tests and added
  coverage for the missing public index, methodology detail, redirect, and
  not-found routes identified in this SOW.
- Searched touched tests for batched `waitFor(() => { ... })` patterns; none
  remain in this SOW scope.
- Searched labeled feed-health/freshness dot spans and fixed the repeated
  invalid `aria-label` pattern with an explicit semantic role.

Artifact maintenance gate:

- AGENTS.md: no update needed; this SOW did not change workflow or project-wide guardrails.
- Runtime project skills: no update needed; existing frontend behavioral
  testing guidance already requires user-visible assertions and axe coverage.
- Specs: no update needed; public route behavior was existing product contract,
  and this SOW added tests plus a semantic ARIA correction.
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

- Axe coverage is valuable for these page-level route tests because it caught a
  real invalid ARIA pattern in existing public status-dot markup.
- Public route tests should wait on the first async route-specific element, then
  assert the rest synchronously to keep failures precise.

Follow-up mapping:

- None.

## Outcome

Completed. Public route contract tests now cover the previously thin index,
methodology, redirect, and not-found routes, and the accessibility issue found
by the new checks was fixed in the affected public status-dot components.

## Lessons Extracted

- Use `role="img"` when a purely visual status dot carries an accessible label.
- Keep `waitFor` to one observable readiness assertion and move independent
  assertions outside the wait.

## Followup

None.
