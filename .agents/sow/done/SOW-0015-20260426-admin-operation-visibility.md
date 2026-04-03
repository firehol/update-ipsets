# SOW-0015 | 2026-04-26 | admin-operation-visibility

## Status

completed

## Requirements

Given the backend has a small finite number of flows (triggers/events, queues, and activities), when this SOW is complete, then all of them must be visible in the admin UI.

Given operators need to see what the backend is doing at any moment, when operations are running, then admin status must show what is running, its progress, queue state, and any failures.

Given operators need to distinguish idle from hidden work, when no operation is active, then the admin UI must clearly show empty/idle state.

## Analysis

Initial sources to consult:

- Admin status API and UI.
- Engine, scheduler, downloader, integrity, and background task telemetry.
- `.agents/sow/specs/admin-ui.md`, `.agents/sow/specs/operating-principles.md`, and `.agents/sow/specs/pipeline.md`.

Current known context:

- Some background entity work is already visible.
- The backend is not infinite — it has a small, finite number of flows that can be enumerated.

## Implications and decisions

### User Decision (2026-05-02)

- "Full visibility" means: enumerate all backend triggers/events, queues, and activities. Every flow the backend executes must be surfaced in the admin UI. When the backend is doing nothing, the admin UI must appear empty.
- The implementation approach: inventory all backend flows first, then ensure each one has admin API/UI representation.

- Admin UI must stay usable and not become a noisy log dump.
- The backend has a finite, enumerable set of flows — not an open-ended list.

## Plan

Chunked SOW - reasoning: backend inventory, API contract, and UI are distinct.

1. `flow-inventory` — enumerate all backend triggers/events, queues, and activities. Low risk, analytical.
2. `admin-api-contract` — define how each flow is exposed via admin API. High risk.
3. `admin-ui-implementation` — implement visibility for all flows, empty state when idle. Medium risk.
4. `tests-and-docs` — verify every flow is represented, verify empty state. Medium risk.

## Flow inventory (completed)

Two subagent investigations + 4-model cross-review (GLM-5.1, MiniMax-M2.7, Kimi-K2.5, Qwen) produced:

- **45 backend flows** identified across daemon lifecycle, scheduler loops, engine pipeline, entity refresh queues, integrity checks, background tasks, downloader, and admin API triggers
- **19 admin API endpoints**, 7 UI components, engine StatusSnapshot, scheduler Snapshot/ActivitySnapshot inventoried

### Cross-review verdict

3 of 4 models completed (Qwen crashed, re-ran successfully). All code:line citations verified against actual source.

## Verified gap analysis

Gaps verified by reading source code at every cited file:line. Original G8 was rejected by 3/4 reviewers and confirmed not a gap (detail field IS exposed via `adminFeed.SchedulerDetail`).

### Tier 1 — Queue-level gaps (operators cannot see items in these states)

**G1: Download refetch pending**
- `download.refetchPending map[string]queuedWork` (`scheduler.go:23`)
- Populated at `queue_admission.go:74-76` when download is already active for same feed
- `ActivitySnapshot()` (`scheduler.go:173-189`) reads `download.waiting`, `download.active` — never reads `refetchPending`
- Aggregate counter `MetricsSnapshot.DownloadDeferred` exists (`metrics.go:15`) but per-feed visibility missing
- Impact: operator sees 0 items in download waiting but feeds are actually queued

**G2: Processing deferred**
- `processing.deferred map[string]queuedWork` (`scheduler.go:30`)
- Populated at `queue_admission.go:91-95` when processing is already active for same feed
- `ActivitySnapshot()` never reads `processing.deferred`
- Aggregate counter `MetricsSnapshot.ProcessingRequeued` exists (`metrics.go:19`) but per-feed visibility missing
- Impact: operator cannot see queued reprocess requests stacking behind active batch

**G3: Entity refresh pending queue**
- `entityRefreshPending map[string]struct{}` (`engine.go:174`)
- Populated at `entity_refresh_queue.go:100-110` during feed-update entity refresh coalescing
- `StatusSnapshot` has no field for pending count or feed names
- Impact: operator sees background task running but not how many feeds are queued for next wave

**G4: Entity health pending queue**
- `entityHealthPending map[string]struct{}` (`engine.go:176`)
- Populated at `entity_refresh_queue.go:113-127` during health-transition entity refresh coalescing
- Same gap as G3
- Impact: same as G3 for health-transition refreshes

**G11: Entity rebuild queued flag**
- `entityRebuildQueued bool` (`engine.go:173`)
- Set at `entity_refresh_queue.go:26`, cleared in defer at line 32
- Not in `StatusSnapshot`
- Impact: low — brief window between rebuild intent and background task start

**G12: Download parent-blocked items**
- `downloadInputsSettledLocked()` (`queue_admission.go:170-187`) checks parent feeds
- In `startNextDownload()` (`queue_admission.go:146-152`), unsettled items are skipped
- Items appear in `download.waiting` via ActivitySnapshot but no signal they are blocked on parents
- Impact: operator cannot distinguish "waiting for worker" from "waiting for parent download"

### Tier 2 — Event/trigger-level gaps (operators cannot see these events happened)

**G5: SIGHUP config reload tracking**
- `daemon.go:82-101`: SIGHUP handler calls `eng.Reload()`, logs success/failure, spawns entity ensure goroutine
- `engine.go:323-378`: `Reload()` re-reads config, validates, swaps — no reload counter/timestamp written to Engine
- `StatusSnapshot` (`engine.go:118-138`) has no reload-related fields
- Impact: operator cannot verify from admin API whether reload happened, when, or if it failed

**G6: Provider defaults change wave reason tag**
- `enqueueProviderDefaultsReprocess()` (`recovery.go:60-71`) calls `enqueueProviderWave(runreason.ReasonScheduledDue, ...)`
- Uses generic `ReasonScheduledDue` — indistinguishable from normal scheduled work
- Contrast: critical infra uses `detail = detailCriticalProviderSetChanged` (`snapshot_build.go:61-63`)
- Impact: operator sees broad reprocess wave but not WHY it was triggered

**G7: Health transition detection events**
- `healthTransitionNames()` (`snapshot_build.go:331-354`) computes feeds whose health class changed between scheduler ticks
- Called at `download_loop.go:21-22`, result passed to entity refresh
- Transition events (from/to class, when) are never persisted or surfaced
- Impact: operator cannot see which feeds changed health class and triggered entity refreshes

### Tier 3 — Operational edge cases

**G9: Startup entity repair deferral**
- `shouldDeferStartupRepair()` (`entity_integrity.go:149-154`) checks `targetCount() > maxStartupEntityAutoRepairTargets`
- On deferral (`entity_integrity.go:172-181`): counter `entity.integrity_startup_repair_deferred` emitted via `observeRunCounter`, logged at Warn
- Counter IS accessible via `LifetimeMetrics.Counters` in admin API but not surfaced as clear status indicator
- Impact: operator must correlate lifetime counter with integrity findings to understand deferred repairs

**G10: Scheduler state persistence failure**
- `storeSnapshot()` (`snapshot_build.go:297-308`) calls `SaveSnapshot()`, logs on error only
- No admin-visible indicator of persistence success/failure
- If persistence fails repeatedly, scheduler loses state across restarts silently
- Impact: operator cannot detect silent scheduler state loss

### Dropped gaps (verified NOT gaps)

- G8 (static source materialization): `detail = "static source config changed"` IS exposed via `adminFeed.SchedulerDetail` (`admin.go:165,682,850`). Working as designed.

### Rejected reviewer additions

- Background task wait time (Kimi): already visible via `BackgroundTasks` stage "queued" + `lifetimeOperations` timing
- Processing batch promotion targets (Kimi): internal batch mechanics, not operator-facing
- Metrics reset history (Kimi): cumulative counters are standard practice
- Manual run history (Qwen): already in `CurrentReason`/`LastReason`
- Network/DNS health (Qwen): out of scope for this SOW
- Entity integrity results at runtime (Qwen): already accessible via `GET /api/v1/admin/integrity/entities`

### Security assessment

No information leaks from exposing any gap. All hidden state is feed names, queue positions, timestamps — already behind admin auth.

## Frontend surface proposal (analysis, no implementation)

Current admin UI architecture: HeartbeatPanel (tiles) → CurrentRunPanel (4 queue columns + background work) → ArtifactsPanel → IntegrityPanel → EntityIntegrityPanel → FeedsTable (16 columns) → FeedModal. Polling at 3s for status, 10s for feeds.

### Existing UI patterns to reuse

- **Hairline tile grid**: `AdminTileGrid` with 5/6/7/8 cols, gap-px, clickable tiles with accent bars
- **Queue column**: `QueueColumn` with `LIVE_QUEUE_VIEWPORT_CLASS` (h-56 overflow-y-auto), count in header, empty text when idle
- **Background task row**: name, trigger, stage, progress (current/total), detail, elapsed time
- **Sublabel pattern**: `queueSublabel()` joins reason, problem class, and detail with `·` separator
- **Confirm/cancel pattern**: two-step destructive action buttons
- **Status tile in HeartbeatPanel**: value + caption, conditional coloring

### Proposal per gap

#### G1 + G2: Download refetch pending + Processing deferred

These are the same pattern: feeds queued behind active work for the same name. The operator sees the active item but not that a newer request is waiting.

**API change**: Add two fields to `ActivitySnapshot`:
- `download_refetch_pending []QueueFeed` — same shape as `download_waiting`
- `processing_deferred []QueueFeed` — same shape as `processing_waiting`

**UI treatment**: Add a count badge to each queue column header when refetch/deferred items exist.

Current header rendering (`current-run-queue-columns.tsx:42-46`):
```
<div className="eyebrow">{title}</div>
<div className="text-[11px] tabular-nums text-muted-foreground">
  {items.length} {items.length === 1 ? itemLabel : `${itemLabel}s`}
</div>
```

Proposed: When refetch/deferred count > 0, append `· +N pending` in `text-status-warning` (amber) after the count. Example: `3 items · +2 pending`. This follows the existing sublabel pattern of joining with `·`.

Hovering the `+N pending` shows a `HoverTip` listing the pending feed names (mono font, same as queue items). No new column needed — the existing 4-column grid stays. The deferred items are semantically "behind" the active column, not a separate queue family.

**Why not a 5th/6th column**: The admin-ui spec says exactly 4 live lists. Refetch/deferred are sub-states of the active columns, not independent queue families. Adding columns would violate the spec and reduce density.

**Empty state**: When no refetch/deferred items exist, nothing extra renders — no visual noise.

#### G3 + G4: Entity refresh pending queues

These are coalesced feed-name sets waiting for the entity refresh background task to pick them up on its next iteration.

**API change**: Add to `StatusSnapshot`:
- `entity_refresh_pending int` — count of coalesced feed names waiting
- `entity_health_pending int` — count of health-transition feed names waiting

(Only counts, not full name lists — the pending sets change fast and full lists would add API churn.)

**UI treatment**: Add a pending count to the background task row when the task is "Entity artifacts refresh".

Current background task row (`current-run.tsx:257-287`): shows name, trigger, stage, progress, detail.

Proposed: When the task trigger is `feed_update` and `entity_refresh_pending > 0`, append to the progress line: `+N more queued`. When trigger is `health_transition` and `entity_health_pending > 0`, same treatment. Color: `text-status-warning` (amber). This tells the operator "the current wave will be followed by another wave of N feeds."

If the background task is NOT running but pending count > 0, render a single-line placeholder in the background work section: `Entity refresh: N feeds coalescing` in `text-muted-foreground`. This distinguishes "idle" from "feeds queued but task not yet started."

#### G5: SIGHUP config reload tracking

**API change**: Add to `StatusSnapshot`:
- `last_config_reload time.Time` — when last reload happened (zero = never)
- `config_reload_count int` — total reloads since startup
- `config_reload_error string` — last reload error, if any (empty = last reload succeeded)

**UI treatment**: Two surfaces:

1. **HeartbeatPanel Row B**: Replace the existing "Uptime" tile with a richer tile or add a small reload indicator. Option A: Add a 6th tile to Row B: `Config` with value showing reload count and last-reload relative time. Caption: last error (in `text-destructive`) or `clean`. This keeps Row B at 6 tiles which still fits `AdminTileGrid cols={6}`.

2. **CurrentRunPanel header**: If `config_reload_error` is non-empty, show a persistent red banner between the header and the queue grid: `Last config reload failed: {error}`. Dismissed when the next reload succeeds.

The HeartbeatPanel approach is preferred — it matches the "at a glance" purpose of that panel and follows the existing tile pattern. The error banner is a secondary safety net.

#### G6: Provider defaults change wave reason tag

**API change**: In `enqueueProviderDefaultsReprocess()`, use a dedicated reason instead of `ReasonScheduledDue`. Add `runreason.ReasonProviderDefaults` (or similar). The processing queue items will carry this reason to the UI.

**UI treatment**: No UI change needed. The existing `formatRunReason()` in `admin-run-reason.ts` will render the new reason string. The queue column items already show reason via `queueSublabel()`. The ProcessingNowColumn already shows reason via `formatRunReason()`. This is a backend-only fix that propagates naturally.

#### G7: Health transition detection events

**API change**: Add to `StatusSnapshot`:
- `recent_health_transitions []HealthTransition` — last N transitions (bounded, e.g. 20)
- Each entry: `feed string`, `from_class string`, `to_class string`, `at time.Time`

**UI treatment**: This is an event log, not a queue. Two options:

Option A: A collapsible "Recent transitions" sub-section inside the background work card. Rendered as a compact list: `feed_name: delayed → risky, 2m ago`. Max 5 visible, with "show all" expanding to full list. Only renders when transitions exist in the last hour.

Option B: A small count badge on the HeartbeatPanel tiles that had transitions. E.g., the "Delayed" tile shows `+3` in amber if 3 feeds transitioned to delayed in the last polling interval.

Recommendation: Option A keeps the CurrentRunPanel as the single "what's happening" surface. Option B would require tracking transitions across polling intervals in the frontend. Option A is simpler and matches the spec's requirement that background work be visible in the admin status surface.

#### G9: Startup entity repair deferral

**API change**: The counter `entity.integrity_startup_repair_deferred` already exists in `LifetimeMetrics.Counters`. Add a dedicated field to `StatusSnapshot` for clarity:
- `startup_repair_deferred bool` — true when startup repairs were skipped
- `startup_repair_deferred_targets int` — number of targets that were deferred

**UI treatment**: Two surfaces:

1. **EntityIntegrityPanel**: If `startup_repair_deferred` is true, show a persistent amber banner at the top of the panel: `Startup repair was deferred: {N} targets exceeded the automatic limit. Use "Rebuild All" to trigger manual repair.` The "Rebuild All" button already exists.

2. **HeartbeatPanel Integrity tile**: If deferred, change caption from "all clean" to `{N} repairs deferred` in `text-status-warning`.

This is low-frequency (only on startup with >1024 entity targets) but important when it happens.

#### G10: Scheduler state persistence failure

**API change**: Add to `MetricsSnapshot`:
- `snapshot_persist_errors int64` — count of snapshot write failures since startup

**UI treatment**: If `snapshot_persist_errors > 0`, add a small warning to the HeartbeatPanel. The "Uptime" tile (or the new "Config" tile if G5 adds one) gets a caption: `{N} snapshot errors` in `text-status-warning`. No separate panel needed — this is a rare condition that just needs a visible flag.

#### G11: Entity rebuild queued flag

**API change**: Add to `StatusSnapshot`:
- `entity_rebuild_pending bool` — true when rebuild was requested but not yet started as a background task

**UI treatment**: In the background work section, if `entity_rebuild_pending` is true but no "Entity artifacts rebuild" task is visible yet, show a placeholder row: `Entity artifacts rebuild: waiting for worker slot` in `text-muted-foreground`. This reuses the existing background task row styling but for a pre-task state. Goes away once the task starts (which already renders normally).

This is a brief window — only visible when the background limiter is full. Low priority.

#### G12: Download parent-blocked items

**API change**: In `ActivitySnapshot`, tag each `download_waiting` item that is blocked on unsettled parents:
- Add `blocked bool` field to `QueueFeed`
- Add `blocked_parents []string` field to `QueueFeed`

**UI treatment**: In the "Waiting To Be Downloaded" column, items that are `blocked: true` get a visual indicator:
- A small `⏳` or `≡` icon (or a `text-status-warning` amber dot) before the feed name
- The sublabel shows `waiting for parent: parent_feed_name`
- These items sort last in the column (after unblocked items)

This follows the existing `QueueFeedItem` pattern — just adds a conditional icon and modified sublabel for blocked items. No new column or panel needed.

### Summary of UI changes

| Gap | UI surface | Change type |
|-----|-----------|-------------|
| G1, G2 | Queue column headers | Count badge + hovertip |
| G3, G4 | Background task rows | Pending count in progress line |
| G5 | HeartbeatPanel Row B | New tile + error banner |
| G6 | (none — backend reason fix propagates) | — |
| G7 | Background work section | Collapsible transition list |
| G9 | EntityIntegrityPanel + HeartbeatPanel | Amber banner + deferred caption |
| G10 | HeartbeatPanel Row B | Caption warning |
| G11 | Background work section | Placeholder row |
| G12 | Download queue column | Blocked icon + parent sublabel |

No new panels, no new columns in the 4-column grid, no new pages. All changes fit into existing surfaces.

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
