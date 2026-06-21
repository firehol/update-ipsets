# SOW-0115 - Admin Live Queue Scrollbars

## Status

Status: completed

Sub-state: regression fix implemented and validated; lifecycle move is included with the implementation.

## Requirements

### Purpose

Keep the admin current-run queue tiles stable while queue contents change, so operators can monitor live downloader and processing work without the page layout growing or jumping as feeds are added.

### User Request

The admin UI tiles "Waiting To Be Downloaded", "Being Downloaded Now", and "Waiting To Be Processed" currently grow vertically as feed rows are added. They should not grow vertically; their feed lists should scroll instead.

### Assistant Understanding

Facts:

- The admin UI spec requires the four live-list viewports to reserve a stable fixed height, show roughly four feed rows, and scroll for overflow (`.agents/sow/specs/admin-ui.md:178`).
- Operator docs state that each current-run queue column is fixed-height and scrollable (`docs/admin-ui/live-queues.md:16`).
- The existing admin operation visibility SOW recorded the reusable queue column pattern as `h-56 overflow-y-auto` (`.agents/sow/done/SOW-0015-20260426-admin-operation-visibility.md:160`).
- The three user-reported tiles render through `QueueColumn` and `ActiveDownloadColumn` in `ui/src/components/admin/current-run-queue-columns.tsx:46`, `ui/src/components/admin/current-run-queue-columns.tsx:91`, and `ui/src/components/admin/current-run.tsx:232`.
- The current shared tile viewport class is `min-h-0 flex-1 overflow-y-auto` (`ui/src/components/admin/current-run-shared.ts:4`), which lets the viewport consume available tile height instead of reserving a fixed list height.

Inferences:

- The latest admin-run tile optimization changed the list viewport from the older fixed `h-56 overflow-y-auto` pattern to a flex-growing viewport. That is the direct layout regression for queues with more rows.
- Applying the fixed viewport through the shared queue tile class should also protect "Being Processed Now" from list-driven height growth, which matches the spec's four-list contract even though the user named three affected tiles.

Unknowns:

- None that block implementation. Browser rendering should still be smoke-checked because jsdom cannot measure real scrollbars or layout height.

### Acceptance Criteria

- The waiting-download, active-download, waiting-processing, and active-processing list bodies use a fixed-height vertical scroll viewport.
- Adding feed rows changes only the internal scroll range, not the tile height.
- Empty states still fill the reserved viewport.
- The fix is validated with focused UI tests plus lint/build.

## Analysis

Sources checked:

- `.agents/sow/specs/admin-ui.md`
- `docs/admin-ui/live-queues.md`
- `.agents/sow/done/SOW-0015-20260426-admin-operation-visibility.md`
- `.agents/sow/todo-history/TODO-admin-current-run-fixed-height.md`
- `ui/src/components/admin/current-run.tsx`
- `ui/src/components/admin/current-run-queue-columns.tsx`
- `ui/src/components/admin/current-run-shared.ts`
- MDN CSS overflow documentation: `overflow: auto` creates a scroll container when content overflows a box with a constrained vertical size (`https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/overflow`).

Current state:

- `LIVE_QUEUE_VIEWPORT_CLASS` already exists as `h-56 overflow-y-auto` for background work (`ui/src/components/admin/current-run-shared.ts:1`).
- Queue tiles use `LIVE_QUEUE_TILE_VIEWPORT_CLASS` as `min-h-0 flex-1 overflow-y-auto` (`ui/src/components/admin/current-run-shared.ts:4`).
- The user-reported queue tiles all consume `LIVE_QUEUE_TILE_VIEWPORT_CLASS` through `QueueColumn` and `ActiveDownloadColumn`.

Risks:

- Changing the shared tile viewport also affects "Being Processed Now"; this is intended by the spec, but its tile contains extra phase/batch content above the list. The fixed list viewport must not hide phase progress.
- jsdom cannot prove pixel height or scrollbar rendering. Validation must include class-level behavioral coverage and build/lint; browser smoke can be done if the user wants visual confirmation in the running admin UI.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The queue list viewport lost its fixed height and now flexes inside tiles. Evidence: `ui/src/components/admin/current-run-shared.ts:4` defines `LIVE_QUEUE_TILE_VIEWPORT_CLASS` as `min-h-0 flex-1 overflow-y-auto`, while the older fixed list pattern still exists at `ui/src/components/admin/current-run-shared.ts:1`.
- Because the viewport can grow with its parent tile, queues with more feed rows can increase tile height instead of creating only an internal scroll range.

Evidence reviewed:

- Admin UI spec requires stable fixed-height live-list viewports with scrolling: `.agents/sow/specs/admin-ui.md:178`.
- Operator docs require fixed-height scrollable columns: `docs/admin-ui/live-queues.md:16`.
- Prior SOW evidence records `h-56 overflow-y-auto` as the queue column pattern: `.agents/sow/done/SOW-0015-20260426-admin-operation-visibility.md:160`.
- Current component wiring uses the shared tile viewport class for the affected tiles: `ui/src/components/admin/current-run-queue-columns.tsx:53` and `ui/src/components/admin/current-run-queue-columns.tsx:98`.
- MDN documents that `overflow: auto` clips overflowing content and provides scrolling when content exceeds a constrained box.

Affected contracts and surfaces:

- Admin UI current-run queue tile layout.
- Admin operator docs and admin UI spec already describe the desired behavior; no contract wording change is expected.
- No backend API, public website, generated artifacts, scheduler semantics, or data model changes.

Existing patterns to reuse:

- Reuse `LIVE_QUEUE_VIEWPORT_CLASS = "h-56 overflow-y-auto"` from `ui/src/components/admin/current-run-shared.ts:1`.
- Keep queue tile rendering in `current-run-queue-columns.tsx` unchanged except for the shared viewport class behavior.
- Keep tests in `ui/src/components/admin/current-run.test.tsx`, using real components and Testing Library queries.

Risk and blast radius:

- Low UI-only blast radius.
- No data loss, security, migration, API, or runtime pipeline risk.
- Main regression risk is visual: the active-processing tile has more header/body content than the other three tiles, so validation should confirm its list viewport is fixed without removing progress context.

Sensitive data handling plan:

- This work only references source files, specs, docs, and public CSS documentation.
- Durable artifacts will not include secrets, credentials, bearer tokens, SNMP communities, customer data, personal data, non-private customer-identifying IPs, private endpoints, proprietary incident details, or raw feed data.
- Evidence will cite file paths, line numbers, and sanitized code-class names only.

Implementation plan:

1. Change `LIVE_QUEUE_TILE_VIEWPORT_CLASS` to reuse the fixed `h-56 overflow-y-auto` queue viewport contract.
2. Add or update a focused UI test that renders more than four rows in a queue and asserts the queue viewport uses the fixed scroll class.
3. Run focused UI tests, lint, build, and `git diff --check`.

Validation plan:

- `pnpm --dir ui test -- current-run.test.tsx`
- `pnpm --dir ui lint`
- `pnpm --dir ui build`
- `git diff --check`
- Same-failure scan for remaining `flex-1 overflow-y-auto` queue tile viewports.

Artifact impact plan:

- AGENTS.md: no update expected; workflow rules unchanged.
- Runtime project skills: no update expected; existing frontend/testing rules covered this.
- Specs: no update expected; `.agents/sow/specs/admin-ui.md` already states the desired fixed-scroll behavior.
- End-user/operator docs: no update expected; `docs/admin-ui/live-queues.md` already states the desired fixed-scroll behavior.
- End-user/operator skills: no update expected.
- SOW lifecycle: close this narrow SOW with the implementation when commit is requested.

Open-source reference evidence:

- None. This is local UI contract restoration; MDN CSS documentation was sufficient for the overflow behavior.

Open decisions:

- None. The user request and existing admin UI contract both choose the design: fixed-height list viewport with internal scrollbar.

## Implications And Decisions

- No user decision is required. This is a surgical fix because the desired design is already defined by the user request, the admin UI spec, and operator docs.
- Recommendation classification: surgical. The fix should restore the prior shared fixed viewport behavior with minimal component churn.

## Plan

1. Patch the shared queue tile viewport class.
2. Add a focused regression guard in the existing current-run component test.
3. Validate focused test, lint, build, and diff whitespace.

## Execution Log

### 2026-06-21

- Opened SOW after confirming the regression against the admin UI spec and operator docs.
- Restored `LIVE_QUEUE_TILE_VIEWPORT_CLASS` to the fixed `h-56 overflow-y-auto` queue viewport contract.
- Added named queue regions for the four live queue list bodies so the scroll regions are accessible and testable.
- Added a focused current-run regression test with six queue rows in each live queue list.

## Validation

Acceptance criteria evidence:

- Fixed-height scroll body is now shared by all queue tile list regions through `LIVE_QUEUE_TILE_VIEWPORT_CLASS = LIVE_QUEUE_VIEWPORT_CLASS` in `ui/src/components/admin/current-run-shared.ts`.
- Waiting-download and waiting-processing use the fixed queue tile viewport through `QueueColumn` in `ui/src/components/admin/current-run-queue-columns.tsx`.
- Active-download and active-processing use the fixed queue tile viewport in `ui/src/components/admin/current-run-queue-columns.tsx`.
- The component test `keeps live queue feed lists in fixed scroll viewports` renders six rows per live queue and asserts each named queue region has `h-56 overflow-y-auto`.

Tests or equivalent validation:

- `pnpm --dir ui test -- current-run.test.tsx` passed. Vitest reported 15 files and 46 tests passing.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui build` passed. Vite still reported existing unresolved `/static/fonts/InterDisplay-*.woff2` runtime references and the existing large chunk warning for `feed-detail`; these warnings are unrelated to this fix.
- `git diff --check` for the touched UI and SOW files passed.

Real-use evidence:

- Not run against a live daemon. This is a CSS/React source fix with component coverage; jsdom cannot measure actual scrollbar pixels.

Reviewer findings:

- Not run. The change is a small UI class regression fix, not a production-grade milestone or PR-ready chunk.

Same-failure scan:

- `rg -n "LIVE_QUEUE_TILE_VIEWPORT_CLASS|flex-1 overflow-y-auto|min-h-0 flex-1 overflow-y-auto" ui/src/components/admin` found no remaining `flex-1` queue tile viewport. Remaining hits are the fixed shared constant and its call sites.

Sensitive data gate:

- Passed. The durable SOW/code/test changes contain no raw secrets, credentials, bearer tokens, SNMP communities, customer names, customer identifiers, personal data, non-private customer-identifying IPs, private endpoints, or proprietary incident details. A safety scan only matched the generic sensitive-data policy terms in this SOW.

Artifact maintenance gate:

- AGENTS.md: no update needed; workflow and repo-wide guardrails did not change.
- Runtime project skills: no update needed; existing frontend/testing rules covered this work.
- Specs: no update needed; `.agents/sow/specs/admin-ui.md` already requires fixed-height scrollable live-list viewports.
- End-user/operator docs: no update needed; `docs/admin-ui/live-queues.md` already says each queue column is fixed-height and scrollable.
- End-user/operator skills: no update needed.
- SOW lifecycle: `Status: completed`; moved to `.agents/sow/done/` and ready to commit with the implementation.

Specs update:

- No update needed; the existing admin UI spec already describes the corrected behavior.

Project skills update:

- No update needed.

End-user/operator docs update:

- No update needed; operator docs already describe the corrected behavior.

End-user/operator skills update:

- No update needed.

Lessons:

- None. This was a localized regression against an existing class contract.

Follow-up mapping:

- None.

## Outcome

Implemented and validated.

## Lessons Extracted

None.

## Followup

None yet.

## Regression Log

## Regression - 2026-06-21

User validation after commit `63fc898` found the live queue tiles were still
not fit for purpose:

- the tiles remained too tall and should be about two-thirds of their current
  height;
- the feed list used only part of the tile height instead of filling the tile
  body and scrolling after the available body was used.

Root cause:

- The first fix changed the queue list viewport to fixed `h-56`, but left the
  tile container as `min-h-80` in `ui/src/components/admin/current-run-shared.ts`.
- That produced a 320px-or-taller tile with a 224px list body. When the
  `Being Processed Now` tile had extra phase/status content, the grid row could
  become taller while the three simpler list bodies stayed fixed, making the
  lists visibly use only part of the tile.

Corrected implementation:

- Make the tile itself the fixed-height object at `h-[13.5rem]`, which is
  approximately two-thirds of the previous `min-h-80` tile floor.
- Use CSS grid rows `auto minmax(0, 1fr)` so the header is fixed by content and
  the scroll body consumes all remaining tile height.
- Keep the list body as `min-h-0 overflow-y-auto`; this is the element that
  scrolls after it has consumed the available tile body.
- Move the active-processing phase/status block inside the scrollable body so
  it cannot stretch the whole grid row and make sibling tiles look empty.

Validation to rerun:

- Focused current-run component test.
- UI lint and build.
- Browser measurement with the static production server, because jsdom cannot
  measure actual element heights or scrollbar geometry.

Regression fix validation:

- `pnpm --dir ui test -- current-run.test.tsx` passed. Vitest reported 15
  files and 46 tests passing.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui build` passed. Vite still reported existing unresolved
  `/static/fonts/InterDisplay-*.woff2` runtime references and the existing
  large chunk warning for `feed-detail`; these warnings are unrelated.
- `git diff --check` for the touched UI and SOW files passed.
- Browser measurement against the built production bundle at 1440x900 with 10
  mocked rows per live queue showed:
  - every live queue tile height: 216px;
  - every live queue header height: 42px;
  - every live queue scroll body height: 174px;
  - expected body height from tile minus header: 174px;
  - `overflow-y: auto`;
  - scroll body `scrollHeight` greater than `clientHeight`, proving the body
    scrolls after using the full available tile body.
