# TODO - Feed Health Regression

## Purpose

Restore correct feed-health classification so feeds that should be marked
`unmaintained` are not incorrectly shown as `ok`.

## TL;DR

- user reports that many historical / stale feeds are now classified as `ok`.
- This is a correctness bug in the health classifier or in the data it consumes.
- The task is to identify the regression with evidence, fix it, verify it on the
  live admin output, and install the corrected daemon.

## Analysis

- Verified live examples show the regression is real for some derivatives, but
  not for all stale feeds.
- Confirmed root cause:
  - internal derivatives (`internal://retention_window`, `internal://merge`)
    are finalized with a fresh local rebuild timestamp in `entry.SourceDate`
  - feed health consumes `entry.SourceDate` as "last upstream change"
  - stale derivatives can therefore look freshly updated and be classified
    `ok`
- Concrete live example:
  - `cleantalk` is old and `unavailable`
  - `cleantalk_1d`, `cleantalk_7d`, `cleantalk_30d` showed fresh
    `last_update` values around the current time and therefore `ok`
- Important contract constraint:
  - `ProcessedDate` is the authoritative local publication time for integrity
  - the fix must correct operator-facing "last upstream change" and health
    without changing integrity semantics
- Follow-up live investigation after the derivative fix:
  - the classifier was no longer misclassifying stale derivatives as healthy
  - the admin heartbeat still reported only `7` unmaintained feeds because
    `buildAdminStatus()` skipped hidden feeds before counting health classes
  - live `/api/v1/admin/feeds` showed `14` unmaintained feeds
  - `7` of those were hidden feeds, so the summary underreported the true
    operational count

## Decisions

- 2026-04-20: rename feed health class `ok` to `healthy` across the full
  product contract:
  - backend classifier
  - APIs
  - admin UI
  - public UI
  - specs and methodology text
- 2026-04-20: rename the remaining integrity/UI `ok` badge text so integrity
  state does not reuse feed-health wording.
- 2026-04-20: investigate why the live system still reports only 7
  `unmaintained` feeds after the derivative timestamp fix:
  - determine whether classification is still wrong
  - or whether cached/on-disk state is stale/corrupt and needs repair

## Plan

1. Compute an effective last-upstream-change timestamp for derived feeds from
   their parents.
2. Use that effective timestamp in health classification and operator-facing
   `last_update` / `updated` fields.
3. Keep `ProcessedDate` unchanged for integrity.
4. Rename the health class contract from `ok` to `healthy` across backend,
   APIs, frontend, and specs.
5. Add focused regression tests for retention/merge derivatives and the renamed
   health class.
6. Run targeted backend and frontend tests.
7. Install and verify on the live admin API/UI.
8. Audit the live `unmaintained` population against the persisted cache/state
   and fix any remaining classification/state regression.

## Implied decisions

- Keep the health contract from `specs/` intact unless the investigation proves
  the specs themselves are wrong.

## Testing requirements

- Add/extend focused health-classification regression tests.
- Run at least:
  - `go test ./pkg/feedhealth ./pkg/web ./pkg/engine`
  - `pnpm --dir ui build`
- Verify live admin/API counts after install when the classifier or cache/state
  repair path changes.

## Verification

- 2026-04-20:
  - `go test ./pkg/feedhealth ./pkg/engine ./pkg/web` passed
  - `pnpm --dir ui build` passed
  - admin feeds summary contract normalized to `healthy`; legacy health-summary
    `ok` counter removed to avoid duplicate meanings in the API
  - follow-up fix:
    - hidden feeds now contribute to heartbeat health totals
    - integrity badge text changed from `ok` to `clean`
    - `./install.sh` completed successfully
    - live `/api/v1/admin/status` now reports `unmaintained: 14`
    - live `/api/v1/admin/feeds` also reports `14` unmaintained feeds

## Documentation updates required

- Update the specs and methodology pages to use `healthy` instead of `ok`.
