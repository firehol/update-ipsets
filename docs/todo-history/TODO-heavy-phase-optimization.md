# TODO - Heavy Phase Backend Optimization

## Purpose

Improve the backend performance of the heavy post-processing phases of
`update-ipsets`, especially geolocation and comparison/metadata work, with the
goal of materially reducing CPU time and end-to-end batch duration without
changing the product contract.

## TL;DR

- Costa asked for autonomous backend optimization of the sensitive and heavy
  post-processing path.
- Scope is backend only. No website/frontend work.
- The work now explicitly includes runtime instrumentation:
  - add scheduler/engine counters and timings for the operations performed
  - install and restart the daemon
  - observe the measured runtime facts on the live system
  - optimize the dominant operations based on those facts
- New live symptoms after the first cache pass:
  - daemon startup still takes minutes
  - feeds remain in "waiting turn" for too long / seemingly forever
- The main suspected hotspots are:
  - repeated pairwise comparison work hidden inside the `metadata` phase
  - repeated GeoIP/ASN dataset parsing and helper rebuilding on every heavy run
- Implementation may add a bounded cache configuration if it materially helps.

## Analysis

### Confirmed current behavior

- The heavy run phases are orchestrated in `pkg/engine/run.go`.
- `RunPhaseMetadata` currently includes `writeMetadataFiles()`, which itself
  starts by running `writeComparisonFiles()`.
- So the operator-visible `metadata` phase is not just metadata JSON writing; it
  includes pairwise overlap generation first.
- `processGeoIPDatabases()` reparses every configured GeoIP provider archive on
  every heavy run.
- `processASNDatabases()` reparses every configured ASN provider dataset on
  every heavy run.
- `writeCountryComparisonFiles()` rebuilds per-provider `countrySets` and a
  `providerUnion` on every heavy run.
- Current configured provider counts in `configs/firehol.yaml`:
  - 5 geo providers
  - 4 ASN providers
  - 6 bogon providers
- Current approximate public output feed count from the live catalog/config is
  about 250+ feeds.

### Important observations

- GeoIP parsing/setup is wasteful, but live logs show it is only around
  sub-second to low-single-digit seconds per run. It is not likely the main
  cause of multi-minute runs.
- ASN parsing/setup is similarly repeated, but also only low-single-digit
  seconds per run.
- Pairwise comparison is likely much heavier:
  - `writeComparisonFiles()` compares every relevant updated feed against all
    other feeds.
  - For ~20 updated feeds and ~250 outputs, this means thousands of pairwise
    `OverlapCountIter()` calls.
  - Each pair currently reopens both latest sets again.
- The actual metadata JSON generation (`buildSetMetadata*`, index/all-ipsets
  generation) is likely relatively cheap compared to comparison.
- The first cache pass improved repeated heavy runs, but it does not explain
  multi-minute startup stalls or queue starvation by itself.
- The new live symptom strongly suggests there is still a startup-critical or
  scheduler-blocking path outside the provider parse/setup waste already fixed.
- Live scheduler/engine instrumentation now exists and exposes:
  - per-run phase timings
  - per-run operation counters/timings
  - per-feed slow-operation breakdowns
  - scheduler queue and batch counters

### Measured source/metadata facts

- Baseline before the latest source-side optimizations:
  - `blocklist_net_ua`
    - `sources.finalize`: about `3739 ms`
    - `sources.update_retention`: about `2426 ms`
    - total source work: about `6298 ms`
  - metadata on a 3-feed update run:
    - `metadata.write_comparison_files`: about `3321 ms`
    - `metadata.comparison_pair_overlap`: `993` pairs, about `5521 ms` CPU
- The instrumentation exposed why some feeds are pathological:
  - `blocklist_net_ua`
    - `history.csv`: about `416k` rows
    - `changesets.csv`: about `416k` rows
    - `new/`: about `1567` cohort files
  - `firehol_level4`
    - `history.csv`: about `488k` rows
    - `changesets.csv`: about `487k` rows
    - `new/`: about `2501` cohort files
  - `firehol_proxies`
    - `history.csv`: about `315k` rows
    - `changesets.csv`: about `313k` rows
    - `new/`: about `69287` cohort files, about `290 MB`
- This proved two concrete waste patterns:
  - repeated full re-parsing of giant history/changeset ledgers
  - repeated retention scans across massive cohort directories even when the
    current update removed zero IPs

### Working optimization direction

- Highest-yield likely changes:
  1. reduce repeated reopening/reloading of feed sets inside comparison
  2. cache parsed provider datasets and prebuilt helper structures
  3. separate phase labeling so comparison is no longer hidden under `metadata`
- Possible deeper change:
  - persist split geo country sets and provider unions to disk, so unchanged
    providers never need reparsing between runs or restarts

## Decisions

### Made by Costa

- Backend optimization may proceed autonomously.
- This is an implementation detail and Costa does not want to approve every
  step.
- If useful, add a configuration option for a fixed cache size for admins.
- Do not touch the website work currently in progress.

### Working implementation decisions

- Start with the highest-yield backend-only optimizations before introducing
  larger structural changes.
- Keep behavior/spec contracts intact unless a clear bug is discovered.
- Prefer bounded caches keyed by committed input freshness (mtime/path) over
  unbounded process-wide retention.
- First implementation pass will do two things:
  - add a run-scoped latest-set cache shared by geo, bogons, ASN, and
    comparison so the heavy block stops reopening the same feed sets
    thousands of times in one run
  - add a GeoIP prepared-provider cache keyed by the committed/staged source
    file freshness so unchanged providers do not get reparsed and re-split on
    every heavy run
- Do not add a runtime cache-size knob in this first pass unless real code
  complexity forces it; the initial caches are naturally bounded by feed count
  within a run and by configured GeoIP provider count across runs.
- The next pass will instrument the scheduler/engine itself rather than rely on
  external profiling first:
  - count operations performed in the feed-processing pipeline
  - measure elapsed time for the dominant operations and phases
  - surface the measurements through backend status/logging so they can be
    observed on the installed daemon
  - use those measurements to drive the next optimization decisions

## Plan

1. Isolate the current heavy-phase hotspots precisely in code. Done.
2. Implement the highest-yield low-risk optimization first. Done.
3. Measure/verify with tests and targeted runtime evidence. Done for backend
   automated coverage.
4. Inspect the live daemon startup path and queue state to identify what keeps
   startup slow and feeds stuck in "waiting turn". Done.
5. Fix any false-positive/measurement blocker that invalidates live evidence.
   Done for the startup integrity storm.
6. Add internal scheduler/engine counters and timing measurements for feed
   processing work, especially source processing and metadata/comparison work.
7. Install/restart the daemon and collect live measurements from the current
   workload.
8. Optimize the dominant paths revealed by the instrumentation.
9. Stop when added complexity starts producing diminishing returns.

## Progress notes

- Implemented:
  - a run-scoped latest-set cache shared across the heavy block
  - a prepared GeoIP provider cache keyed by provider source freshness
- Verified with:
  - `go test ./pkg/engine ./pkg/scheduler ./pkg/web`
  - `go test ./...`
- Remaining likely wins are now more structural:
  - pairwise comparison algorithm redesign
  - persistent materialization of geo provider country sets / unions
  - possible ASN provider caching if later profiling proves it worthwhile

- Implemented from the new instrumentation pass:
  - process-local runtime ledger caches for:
    - exact history stats
    - bounded history tails
    - bounded changeset tails
    - retention past histogram state
  - public history/changeset output generation now reuses the bounded runtime
    tails when available instead of reparsing the full ledgers on every update
  - `normalizeChangesetLedgerHeader()` no longer reads the entire file just to
    inspect the first line
  - retention now skips rewriting unchanged cohort files
  - retention now avoids the second full rescan of `new/` plus `retention.csv`
    by building the current histogram directly during the update pass
  - retention now keeps a compact runtime cohort index (`added_at -> current
    count`) so updates with `removed = 0` can skip scanning tens of thousands
    of cohort files
  - pairwise comparison now short-circuits obviously disjoint pairs and records
    how many were skipped
  - exact history gap totals/min/max are now persisted in cache entries so the
    first post-restart update can rebuild history cadence state without
    reparsing the full internal ledger
  - when older cache entries do not yet have the new exact gap fields, history
    bootstrap now falls back to the already-persisted minute cadence stats
    instead of rescanning the full ledger on the first post-restart update
  - history bootstrap now prefers the already-published
    `web/<feed>_history.csv` tail and falls back to the internal ledger only
    when needed
  - comparison skip filtering now uses a finer exact prefix occupancy bitmap,
    reducing expensive overlap work for clearly disjoint pairs
  - heavy fan-out work now has its own worker domain
    (`max_heavy_phase_workers`) so metadata/comparison, GeoIP, ASN, and bogons
    are no longer capped by the low feed-processing worker count
  - retention now persists a compact on-disk cohort index
    (`lib/<feed>/retention_cohorts.csv`) so future restarts can rebuild
    retention state without reopening every `new/` snapshot file

- Verified improvements with live evidence:
  - `blocklist_net_ua`
    - before:
      - `sources.finalize`: about `3739 ms`
      - `sources.update_retention`: about `2426 ms`
      - total: about `6298 ms`
    - after:
      - `sources.finalize`: about `1687 ms`
      - `sources.update_retention`: about `424 ms`
      - total: about `2241 ms`
  - on a later mixed run, `blocklist_net_ua` dropped further to about:
    - `sources.update_retention`: about `230 ms`
    - total source work: about `362 ms`
  - `firehol_proxies` before the no-removal fast path:
    - `sources.update_retention`: about `44787 ms`
  - `firehol_proxies` after the no-removal fast path:
    - it no longer appears as a retention outlier
    - in the measured mixed run, total `sources.update_retention` across seven
      updated feeds was about `1277 ms` with max about `517 ms`
  - metadata on a single-feed heavy run after the source fixes:
    - `metadata.write_comparison_files`: about `1959 ms`
    - `metadata.comparison_pair_overlap`: `307` pairs, about `2845 ms` CPU
  - after the history bootstrap pass on a larger live run:
    - `sources.finalize.observe_history`: `17` calls, about `4078 ms` total,
      about `239 ms` average
    - after one persistence cycle and restart:
      - `sources.finalize.observe_history`: `10` calls, about `1809 ms` total,
        about `180 ms` average
  - after the finer comparison prefix filter on a smaller live run:
    - phase timings:
      - `sources`: about `1699 ms`
      - `metadata`: about `2923 ms`
      - `geoip`: about `1380 ms`
      - `asn`: about `1388 ms`
    - comparison:
      - `metadata.comparison_pair_overlap`: `484` pairs, about `4094 ms` CPU
      - `metadata.comparison_pair_skipped`: `179` pairs
  - after separating heavy-phase workers from source-processing workers:
    - similar startup-sized batches dropped from about:
      - `metadata`: `2923 ms`
      - to about `1584 ms`
    - CPU sampling during a live batch showed:
      - average daemon CPU about `196%`
      - peak daemon CPU about `307%`
    - this confirms the heavy block is no longer effectively capped at 2
      workers
  - after the rounded history bootstrap fallback:
    - first post-restart batches no longer show `sources.finalize.observe_history`
      as a dominant cost
    - example:
      - `sources.finalize.observe_history`: `1 ms` total across updated feeds
  - after writing durable retention cohort indexes:
    - index files were confirmed on disk for updated feeds, for example:
      - `lib/blocklist_net_ua/retention_cohorts.csv`
      - `lib/firehol_level4/retention_cohorts.csv`
      - `lib/botscout/retention_cohorts.csv`
    - the live scheduler kept mixing in newly-due feeds, so there is not yet a
      perfectly isolated apples-to-apples measurement for the same feed across
      two cold restarts

- Current conclusion from live evidence:
- the cold history path has been effectively removed as a meaningful startup
  offender
- the metadata/comparison phase was under-parallelized and is now materially
  faster on multi-core machines
- the remaining dominant backend cost is retention on large feeds and large
  derivatives during the first run after restart
- the new durable retention cohort index is the current mechanism to attack
  that remaining cold path; it is implemented and writing to disk, but needs
  more live cycles to accumulate cleaner before/after evidence
    - `asn`: about `1653 ms`
  - hottest measured operations:
    - `metadata.comparison_pair_overlap`: `2722` calls, about `18345 ms` total
      CPU
    - `metadata.write_comparison_files`: about `9863 ms`
    - `sources.finalize`: `9` calls, about `9958 ms` total, max about `3268 ms`
- Confirmed `geoip` waste pattern in code:
  - `writeCountryComparisonFiles()` currently does, for each
    `(feed, provider)` pair:
    - one full `OverlapCountIter()` per country set
    - one extra full `OverlapCountIter()` against the provider union
  - so a single feed/provider comparison performs hundreds of full sweeps over
    the same feed set
- Next implementation target:
  - flatten each prepared GeoIP provider into one sorted disjoint segment list
    with attached country-code indexes
  - count a feed/provider overlap in one pass over:
    - the feed ranges
    - the flattened provider segments
  - preserve current output semantics:
    - per-country values still count overlapping multi-country segments for each
      matching code
    - `total_mapped` still counts the de-duplicated union only once
  - drop the redundant prepared-provider union scan from runtime counting
  - reuse the same flattened structure for single-IP country lookup so the
    cached GeoIP provider becomes both faster and smaller

- Implemented and verified:
  - prepared GeoIP providers now flatten all country buckets into one sorted
    disjoint segment table with attached country-code indexes
  - per-feed GeoIP attribution now counts overlaps in one pass over:
    - the feed ranges
    - the flattened provider segments
  - single-IP country lookup now uses the same flattened provider structure via
    binary search
  - the prepared GeoIP cache no longer keeps the redundant per-country set list
    plus provider union just to support runtime counting

- Live result after install:
  - first completed post-install run:
    - `geoip`: about `1396 ms` (down from about `17963 ms`)
    - `metadata`: about `6388 ms`
    - `sources`: about `5982 ms`
    - `asn`: about `1338 ms`
  - same run operation totals:
    - `metadata.comparison_pair_overlap`: `2128` calls, about `11106 ms`
    - `metadata.write_comparison_files`: about `6183 ms`
    - `sources.finalize`: `7` calls, about `8924 ms`
  - conclusion:
    - GeoIP is no longer the dominant wall-clock phase.
    - The next highest-yield work is now:
      - comparison/metadata overlap cost
      - and then source finalize for the slower feeds

- Implemented and measured after the GeoIP fix:
  - comparison now builds an exact per-feed `/16` occupancy bitmap during the
    prepare pass and skips any pair whose bitmaps are disjoint before calling
    `OverlapCountIter()`
  - this increased exact zero-overlap skips materially:
    - before: about `230` skipped pairs on the sampled run
    - after: about `527` skipped pairs on one sampled run, about `431` on
      another
  - however, the overall metadata win was modest because the remaining pairs are
    still expensive; this is useful but not the main remaining source of time

- Instrumentation findings for source finalize:
  - file publication itself is cheap:
    - `sources.finalize.write_latest`: about `1-2 ms`
    - `sources.finalize.write_text`: about `1-2 ms`
    - `sources.finalize.append_history`: about `1 ms`
  - the expensive work was not file I/O; it was post-write ledger/stat work:
    - `sources.finalize.observe_history`
    - `sources.finalize.refresh_rotation`

- Correctness + performance fix implemented:
  - `refreshRotationStatsFromLedger()` used to run inside `finalize()` before
    `updateRetention()` appended the new changeset row
  - this meant:
    - rotation/change-ratio stats were one update behind
    - the refresh often paid cold-load cost before retention had warmed the
      runtime changeset cache
  - the refresh now runs after `updateRetention()`
  - regression coverage now asserts that rotation stats include the latest
    changeset in the same run

- Live result after moving rotation refresh:
  - on the sampled post-install run:
    - `sources.refresh_rotation`: about `1 ms`
    - `sources.finalize.observe_history`: about `1157 ms`
    - `sources.finalize`: about `1165 ms`
    - `sources`: about `2663 ms`
    - `metadata`: about `2341 ms`
    - `geoip`: about `1409 ms`
  - this isolated the next remaining source-side hotspot very clearly:
    - `sources.finalize.observe_history` is now the dominant source-finalize
      cost

- Next likely optimization target:
  - avoid the cold full-scan of `lib/<feed>/history.csv` on the first update
    after daemon restart
  - likely direction:
    - persist or reconstruct the exact history aggregate state cheaply
    - load only the last chart window for tail consumers
    - avoid reparsing the full append-only ledger just to refresh entry stats

- Live startup investigation found the current blocking issue:
  - startup integrity is queuing a very large `startup_integrity_rebuild` run
  - the queue storm is not caused by missing or stale outputs
  - it is caused by false `malformed secondary files` findings across many feeds
  - journal evidence shows the findings start immediately at daemon boot and
    then the two processing workers spend minutes rebuilding feeds that were
    already good

- Confirmed root cause for the false integrity findings:
  - integrity validates `<feed>_insights.json` by unmarshalling it into
    `insightsPayload`
  - `insights.Insight.Section` serializes as strings like `"retention"`
  - `insights.Section` has `MarshalJSON()` but no matching `UnmarshalJSON()`
  - therefore valid non-empty insights files fail validation and make startup
    queue a fake recovery batch

- Immediate fix plan:
  - make `insights.Section` round-trip its serialized JSON form
  - add regression coverage proving a valid non-empty insights payload is
    accepted by integrity
  - reinstall and verify startup no longer queues the false rebuild storm

- New investigation target after the startup integrity fix:
  - live CPU load now points mainly at `sources` and `metadata`
  - we do not yet have enough in-product measurement to say which exact
    operations inside these broad phases dominate
  - the next work item is to instrument:
    - scheduler queue activity
    - per-run phase timings
    - per-feed operation timings inside source processing / metadata work
    - hot operation counters where repeated work may be hidden

- Second implementation pass will focus on persistent GeoIP preparation:
  - parse/split a GeoIP provider only when its committed source archive changes
  - write provider-union and per-country binary sets under the provider's lib
    directory
  - reopen those prepared files on later runs and after restarts
  - keep the public/output behavior identical

- Review of commit `aa71f03` found 2 concrete bugs to fix before any further
  optimization:
  - GeoIP provider cache invalidation is too weak:
    - current key uses only path + format + file mtime + file size
    - if a real download changes the archive body but upstream preserves the
      same Last-Modified timestamp and body size, the cache can incorrectly
      reuse stale parsed provider data
  - latest-set cache is too aggressive for text-fallback feeds:
    - it shares one in-memory `*iprange.IPSet` across multiple heavy workers
      when `lib/<feed>/latest` is unavailable and the engine falls back to
      parsing the text `.ipset` / `.netset`
    - `pkg/iprange.IPSet` explicitly documents that it is not safe for
      concurrent use

- Fix plan for these bugs:
  - strengthen GeoIP cache freshness to include content identity, not just
    metadata
  - keep latest-set cache only for file-backed latest sets; text-fallback sets
    remain uncached to preserve the original per-open isolation semantics

## Implied decisions

- This work should not include frontend/admin visual changes unless strictly
  needed to keep backend semantics coherent.
- If a cache is added, it must be bounded and invalidated by committed source
  freshness, not by guesswork.
- Specs and AGENTS must be updated if backend behavior/contracts become more
  explicit.

## Testing requirements

- `go test ./pkg/engine ./pkg/scheduler ./pkg/web`
- `go test ./...`
- Add focused regression tests for any new cache/invalidation behavior.
- Re-run `go test -race ./pkg/engine` after fixing the latest-set cache
  sharing semantics.

## Documentation updates required

- Update `specs/pipeline.md` if heavy phase behavior or caching semantics become
  part of the product contract.
- Update `specs/memory-management.md` if a new bounded cache becomes part of the
  supported runtime behavior.
- Update `AGENTS.md` only if assistant/backend workflow guidance changes.
