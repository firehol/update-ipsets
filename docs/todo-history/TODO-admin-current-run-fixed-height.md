# TODO-admin-current-run-fixed-height — stabilize the admin live-run panel

> **Purpose**: Keep the admin top-area live queue panel visually stable during auto-refresh. The `Run in progress` section must stop changing height as the live queues grow and shrink. Each live list should reserve enough vertical space for 4 visible feed rows and then scroll.

## TL;DR

Costa reported that the admin UI jumps up and down on every update because the
`Run in progress` section grows and shrinks with the live queue sizes.

Requested fix:

- make the `Run in progress` section stable
- reserve height for 4 visible feeds
- use a scrollbar for overflow

## Analysis

Verified in the current React admin implementation:

- the panel lives in `ui/src/components/admin/current-run.tsx`
- the live lists currently use:
  - `max-h-[320px] overflow-y-auto`
  - in three places:
    - waiting to be downloaded
    - being downloaded now
    - being processed now
- because this is a **max** height rather than a fixed reserved height, the
  panel shrinks when there are fewer rows and expands until the cap when there
  are more rows
- this is the direct cause of the visible page jump during auto-refresh

Additional nuance found during code review:

- queue rows are mostly single-line / truncated and therefore close to a fixed
  height already
- the `ProcessingFeedItem` detail line currently uses wrapped text:
  - `break-words whitespace-normal`
- this means one processing row can become taller than the others, which can
  undermine the "4 visible feeds" rule unless that detail is constrained

## Decisions

### Costa decisions already made

- the `Run in progress` section should have a fixed height
- it should show 4 visible feeds
- overflow should scroll

## Plan

1. Replace the current `max-h` live-list treatment with a fixed-height scroll
   region sized for 4 rows.
2. Make the empty states fill that reserved region so the panel keeps the same
   height even when a queue is empty.
3. Constrain any row content that can vertically expand enough to break the
   four-row visibility target.
4. Update the admin UI spec to state that the live queue panel must reserve a
   stable viewport and scroll instead of changing page height with queue size.
5. Build/install and verify the running admin UI still works.

## Implied decisions

- This is an admin UI presentation fix only; no backend/API changes are needed.
- The stabilization should apply consistently across all four live queue
  columns in the panel, not only one of them.

## Testing requirements

- `pnpm --dir ui build`
- `git diff --check`
- `./install.sh`
- `curl http://localhost:18888/healthz`

## Verification notes

- Implemented in `ui/src/components/admin/current-run.tsx`
- Live queue columns now use a fixed `h-56` viewport with overflow scrolling
- Empty states fill the reserved viewport instead of collapsing the panel
- Processing detail text is truncated with a hover reveal so one row cannot
  expand unpredictably and break the four-row viewport
- Verified with:
  - `pnpm --dir ui build`
  - `git diff --check`
  - `./install.sh`
  - `curl http://localhost:18888/healthz` -> `ok`

## Documentation updates required

- `specs/admin-ui.md`
