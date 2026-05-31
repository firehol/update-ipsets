# Retention Empty State

## TL;DR

The feed detail page must keep the Retention section visible for new feeds. When retention data is missing or not ready yet, the section should say there are no retention data yet instead of disappearing.

## Purpose

Make the public UI stable and clear for new feeds: users should understand that retention tracking has not accumulated enough data yet, not think the page is broken or that the Retention section does not exist.

## Analysis

- `ui/src/pages/feed-detail.tsx` always mounts `<SectionRetention feedName={feedName} />` inside the feed detail page.
- `ui/src/components/feed-detail/section-retention.tsx` currently hides the whole section in two cases:
  - query error, including expected 404 for young feeds;
  - successful response with neither `current.hours` nor `past.hours`.
- The backend API type allows both retention windows to be absent: `RetentionData { past?: RetentionWindow; current?: RetentionWindow }`.
- Therefore the bug is in the frontend section rendering policy, not page routing.

## Decisions

No user decision required. user specified the desired behavior directly: the Retention chart section should not disappear; it should say there are no retention data yet.

## Plan

1. Keep the existing loading skeleton while the query is loading.
2. Replace the `return null` paths in `SectionRetention` with a visible `DetailSection`.
3. Add a compact empty-state panel with exact user-facing message: "No retention data yet."
4. Keep existing chart rendering unchanged when `current` or `past` data exists.
5. Run the UI production build to verify TypeScript and Vite.

## Implied Decisions

- Treat 404 and malformed/missing retention payloads the same from the user's perspective: show the empty state.
- Do not change backend APIs.
- Do not add new visual components unless needed; use existing editorial section styling.

## Testing Requirements

- `pnpm --dir ui build` passed.
- `pnpm --dir ui exec eslint src/components/feed-detail/section-retention.tsx` passed.
- `git diff --check` passed.
- `pnpm --dir ui lint` was run and failed on pre-existing unrelated lint errors outside the changed file:
  - `Date.now()` during render in admin components;
  - fast-refresh export warnings in shared component files;
  - synchronous state updates in existing effects.

## Documentation Updates Required

- No public documentation update required.
- Keep `AGENTS.md` unchanged unless this reveals a new durable project rule.

## Implementation Result

- `SectionRetention` no longer returns `null` for missing retention data.
- Loading still renders the existing skeleton.
- Query errors, including expected 404 for new feeds, render the Retention section with "No retention data yet."
- Empty successful payloads with no `current` or `past` histogram data render the same empty state.
- Existing chart rendering is unchanged when retention data exists.
