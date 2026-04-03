# SOW-0019 | 2026-04-26 | feed-list-sidebar-ux

## Status

open
analysis-first: user wants powerful sidebar filtering but needs analysis of
current homepage filtering vs sidebar limitations before defining exact scope;
high user involvement expected for design decisions

## Requirements

Given the homepage has powerful filtering but the feed-page sidebar is limited, when this SOW is complete, then the sidebar must provide significantly more powerful filtering and navigation.

Given users navigate many feeds, when the drawer is improved, then it must handle large lists without layout shift, scroll traps, or confusing focus behavior.

Given this is public navigation, when changes are made, then desktop and mobile behavior must both be validated.

## Analysis

### User Context (2026-05-02)

- The homepage already has powerful filtering capabilities.
- The sidebar that opens in feed pages is very limited by comparison.
- User wants a much more powerful sidebar, but exact requirements need significant user involvement to define.
- This SOW should start with analysis: compare homepage filtering to sidebar filtering, identify gaps, and propose options for user decision.

Initial sources to consult:

- Feed list/sidebar/drawer React components.
- Homepage filtering components and capabilities.
- Site header/navigation components.
- Existing search/filter state and route behavior.

## Implications and decisions

- This SOW requires significant user involvement for design decisions — the exact sidebar capabilities must be defined by the user.
- Navigation changes can affect every feed-detail workflow.
- Mobile drawer behavior needs separate validation.
- More powerful filtering must still preserve fast scanning.

## Plan

1. `analysis` — compare homepage filtering to sidebar, identify gaps, propose options. Low risk, requires user decision.
2. `interaction-design` — user decides direction. Blocked on user.
3. `implementation` — implement approved design. Medium risk.
4. `responsive-validation` — verify desktop and mobile. Medium risk.
5. `tests-and-docs` — low risk.

## Execution log

Pending.

## Validation

- [ ] Acceptance criteria evidence
- [ ] Real-use validation evidence
- [ ] Cross-model reviewer findings (logged + addressed)
- [ ] Lessons extracted (or "none, reasoning: ...")
- [ ] Same-failure-at-other-scales check

## Outcome

Pending.

## Lessons extracted

Pending.
