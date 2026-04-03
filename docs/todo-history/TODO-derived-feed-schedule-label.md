# TODO: Derived Feed Schedule Label

## Purpose

Make the admin UI communicate the real trigger model of derived feeds
(retention windows and merges) so operators are not misled into thinking
they are scheduled by wall-clock cadence when they are actually triggered
by parent/input updates.

## TL;DR

- Derived feeds such as `botscout_1d` are created from parent feeds via
  `history:` expansion.
- They are assigned `frequency: 0` and use `internal://...` URLs, so they
  are not polled on a fixed schedule.
- The scheduler currently returns `now + 365 days` as a sentinel next-due
  timestamp for `frequency: 0` feeds, and the admin UI surfaces that raw
  timestamp as "in 365 days", which is operationally misleading.

## Analysis

- `botscout` declares retention windows in the catalog:
  - `configs/firehol.yaml:376-383`
- Retention variants are expanded in the loader by shallow-cloning the
  parent and changing:
  - `Name` to `<parent>_<window>`
  - `URL` to `internal://retention_window?...`
  - `Frequency` to `0`
  - `DerivedFrom` to `[parent]`
  - evidence: `pkg/config/expand.go:89-95`
- The scheduler special-cases `frequency: 0`:
  - first run: `now`, detail `never checked (static source)`
  - later runs: `now + 365*24h`, detail `static source (never expires)`
  - evidence: `pkg/scheduler/scheduler.go:311-321`
- Catalog invariants explicitly document that internal sources
  (`retention_window`, `merge`, etc.) have `frequency 0` because they do
  not poll on a schedule:
  - `pkg/config/catalog_verify_test.go:473-478`
- Runtime behavior for derived feeds is dynamic injection, not time-based
  scheduling:
  - initial worker set excludes `internal://` sources
  - dependents are injected when the parent finishes with `updated`
  - evidence:
    - `pkg/engine/run.go:81-87`
    - `pkg/engine/run.go:175-180`
    - `pkg/config/dependents.go:15-19`
    - `pkg/engine/process.go:84-93`
- Admin API exposes raw scheduler fields:
  - `next_check` from `scheduler.Item.NextDue`
  - `scheduler_detail` from `scheduler.Item.Detail`
  - evidence: `pkg/web/admin.go:627-630`
- Admin UI renders the raw relative time from `next_check`, so the
  sentinel leaks to operators:
  - feeds table: `ui/src/components/admin/feeds-table.tsx:752-765`
  - feed modal: `ui/src/components/admin/feed-modal.tsx:584-597`
  - current run list: `ui/src/components/admin/current-run.tsx:441`

## Decisions

### Pending

1. None currently.

### Made

1. Operator-facing wording for derived feeds with `frequency: 0`
   - Choice: `triggered by inputs`
   - Reason:
     This is accurate for both retention and merge derivatives and tells
     operators what causes the feed to run.

2. Scope of replacement
   - Choice: replace both the visible relative time and the detail text
     for derived feeds
   - Reason:
     The visible `in 365 days` text is the misleading part. Leaving it in
     place would preserve the operator confusion even if the detail line
     changed.

## Plan

1. Reuse existing feed metadata (`kind`, `frequency_minutes`,
   `scheduler_detail`) to avoid adding new API fields unless needed.
   - Completed
2. Update the admin UI rendering so derived feeds stop showing the
   sentinel `next_check` time.
   - Completed
3. Preserve real `next_check` behavior for plain scheduled sources and
   failing sources with retry/backoff.
   - Completed
4. Add tests covering the derived-feed rendering path.
   - Completed for backend/admin payload normalization
   - UI has no existing component-test harness in `ui/`; verification for
     the UI layer was done with targeted eslint + production build

## Implied Decisions

- Keep scheduler internals unchanged unless the UI-only fix proves
  insufficient. The sentinel is acceptable internally; the problem is
  operator-facing presentation.
- Use existing semantic signals already exposed by the backend whenever
  possible instead of introducing feed-name special cases.

## Testing Requirements

- Add/extend frontend tests for admin table / feed detail rendering:
  - retention feed with `frequency_minutes: 0`
  - merge feed with `frequency_minutes: 0`
  - normal time-scheduled feed with positive frequency
- Verify derived feeds no longer show `in 365 days` in the admin table
  and modal.
- Verify normal feeds still show relative next-check time and scheduler
  detail normally.

### Verification run

- `go test ./pkg/web/...` ✅
- `npm run build` in `ui/` ✅
- `npx eslint src/lib/admin-format.ts src/components/admin/feeds-table.tsx src/components/admin/feed-modal.tsx src/components/admin/current-run.tsx` in `ui/` ✅
- `npm run lint` in `ui/` ⚠️ fails on pre-existing unrelated issues:
  - `src/components/ip-search/ip-search-surface.tsx:48`
  - `src/pages/home.tsx:21`

## Documentation Updates Required

- No public methodology page appears affected.
- If the admin UI includes helper text or operator docs for scheduler
  semantics, update that wording to mention derived feeds are triggered by
  parent/input updates rather than wall-clock cadence.
  - Completed in `AGENTS.md`
