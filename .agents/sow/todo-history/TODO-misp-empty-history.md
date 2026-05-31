# TODO-misp-empty-history

## Purpose
Restore correct MISP feed ingestion and repair the false empty-history period for all `misp_*` feeds.

## TL;DR
user reported that all `misp_*` feeds now show zero IPs. We need to find the regression, fix the current downloader/parser behavior, and remove the incorrect empty/zero history entries introduced by the bug so these feeds no longer appear to have gone empty.

## Analysis
- Verified facts:
- Current retained raw `misp_*.source` files are healthy MISP warninglist JSON and
  parse correctly with the current downloader pipeline.
- Current published `misp_*.{ip,net}set` files are stale empty publications from
  the bad interval, not proof that the current parser is still broken.
- The exact live bug still present in the code is that downloader-stage `same`
  and `not_modified` reuse the existing canonical feed body instead of
  reparsing the retained raw source on manual `recheck`.
- Because of that short-circuit, parser fixes do not heal already-bad feeds
  unless the upstream raw file changes.
- A second live bug is that when a corrected publication reuses the same
  upstream timestamp as the stale cached publication, runtime history stats keep
  the old last point, so the public/admin API can continue to serve the stale
  empty cache state until restart.
- A third robustness gap is that the startup/bootstrap history parser was not
  trimming CRLF line endings, so repaired `history.csv` files written by a
  generic CSV writer could silently collapse back to version `1` on restart.
- For every `misp_*` feed inspected, the current parsed unique-IP count matches
  the last good non-empty history row exactly, so the false-empty interval can
  be reversed without inventing new content.
- Affected ledgers/state are broader than `history.csv`: the false-empty event
  also wrote the full-removal row to `changesets.csv`, `retention.csv`,
  emptied `retention_cohorts.csv`, and deleted the active `lib/{feed}/new/*`
  cohort files.

## Decisions
- No open design decisions yet.
- User requirement: reverse the false zero/empty history caused by this bug.
- Implementation decision: manual downloader `recheck` for raw-source feeds
  must rebuild the canonical feed body from retained raw source on `same` and
  `not_modified`, so parser fixes can repair bad local canonical state.
- Repair decision: for each affected `misp_*` feed, remove only the false-empty
  interval and restore the live cohort anchor at the last valid non-empty
  timestamp before the bug.

## Plan
1. Identify how `misp_*` feeds are defined in config and how they were handled in the bash implementation.
2. Inspect live downloaded sources, canonical outputs, and history files to find the exact regression point and affected files.
3. Fix downloader `same` / `not_modified` handling so manual `recheck` reparses retained raw source for raw-source feeds.
4. Fix same-timestamp runtime history refresh so corrected publications replace stale empty cache state immediately without waiting for restart.
5. Add regression tests proving retained-raw `recheck` healing, same-timestamp cache correction, and CRLF-safe history bootstrap parsing.
6. Install the fixes and manually recheck all affected `misp_*` feeds so committed feed bodies and cache state become non-empty again.
7. Repair the historical artifacts/ledgers by removing the false zero/empty interval from affected `misp_*` feeds and restoring their retention cohort anchor.
8. Drop the stale repaired MISP cache entries and restart so startup rebuilds them from the cleaned ledgers.
9. Verify with targeted tests, live data inspection, repaired API output, and reinstall/restart.

## Implied decisions
- Keep the repair bounded to false entries introduced by this bug; do not rewrite unrelated legitimate history.
- Prefer repairing from local retained historical evidence when available.
- Prefer conservative continuity restoration from the last valid non-empty
  timestamp per feed rather than inventing earlier per-IP ages we cannot prove.

## Testing requirements
- Add or update downloader tests for manual `recheck` on `same` and
  `not_modified` retained-raw-source repair.
- Verify repaired `misp_*` feeds no longer publish zero canonical bodies.
- Verify history/evolution no longer contains the false empty transition.
- Verify the restored retention cohort state supports non-zero `first_seen`
  again for current MISP feed matches.

## Documentation updates required
- Update specs/docs to clarify that manual downloader `recheck` reparses
  retained raw source even when upstream is `same` or `not_modified`.
