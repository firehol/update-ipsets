## TL;DR

Purpose: restore historically tracked IP feeds that still have synced production data, classify every feed by maintenance health from its past, and surface that classification consistently in both the public site and the admin.

User requirements:
- Detect and present 4 classes:
  - Unavailable Feeds: cannot download for 10x their max update frequency
  - Empty Feeds: can download, but empty, immediately
  - Unmaintained Feeds: can download, has entries, but is stale for 10x their max update frequency
  - OK Feeds
- Exclude selected feeds from unmaintained detection when they are static by nature
  - example: `bogons`
- Detect feeds that were added once but never changed again
  - only classify these after a separate grace period
  - current requested grace period: 10 days after first add
- Make the health thresholds user-configurable in config:
  - unavailable / unmaintained threshold multiplier over max update frequency
  - single-update grace period
- Surface details behind the classification:
  - average update frequency
  - max update frequency
- Public side:
  - show the classification
  - add a toggle to show old/historical feeds in the menu, default off
- Admin side:
  - show the same classification and cadence details
- Restore old bash feeds only when we have their historical synced data
- Mine the old bash configuration from git history and add those feeds to the new config
- Restart the server and verify the classification works
- Review why `unavailable` is still too low after time passes, even for feeds
  that have been failing for years in imported d1 data
- Fix the admin integrity endpoint returning `500 Internal Server Error`

## Analysis

### Verified current data already available

- Cache entries already store the fields needed for health classification and presentation:
  - `pkg/cache/cache.go`
  - `Entries`, `UniqueIPs`
  - `CheckedDate`, `SourceDate`, `ProcessedDate`, `StartedDate`
  - `DownloadFailures`, `LastError`, `LastStatus`
  - `AverageUpdateMins`, `MinUpdateMins`, `MaxUpdateMins`
  - `LastRunReason`, `LastProcessingMS`

- Public feed metadata already exposes cadence fields:
  - `pkg/engine/output.go`
  - JSON fields:
    - `errors`
    - `average_update`
    - `min_update`
    - `max_update`

- Admin feed rows already expose cadence and failure fields:
  - `pkg/web/admin.go`
  - JSON fields:
    - `avg_update_mins`
    - `min_update_mins`
    - `max_update_mins`
    - `download_failures`
    - `last_error`
    - `last_status`
    - `entries`

### Verified current public/admin drift

- Public `/api/v1/sets` is currently just a filtered cache snapshot:
  - `pkg/web/server.go`
  - It excludes `Hidden`, but it does not derive the requested health class.

- Public feed detail `/api/v1/sets/{name}` returns metadata JSON:
  - `pkg/web/server.go`
  - `pkg/engine/output.go`
  - This already has cadence numbers but no unified health classification.

- Admin uses its own status derivation:
  - `pkg/web/admin.go`
  - `deriveFeedStatus()` currently returns `error`, `new`, `running`, `stale`, `ok`
  - This does not match the requested 4-class health model.

- Admin frontend also derives its own buckets from that legacy admin status:
  - `ui/src/lib/admin-format.ts`
  - Current buckets are `healthy`, `stale`, `error`, `running`, `never_run`, `disabled`, `hidden`

Conclusion:
- Health classification must move to one shared backend contract, then public/admin should both consume it.

### Verified startup-time regression after timestamp repair

- The service now times out during startup before it reaches `READY=1`.
  - evidence from `journalctl -u update-ipsets`:
    - startup logs stop at:
      - `configuration loaded`
      - `cache loaded`
    - no later:
      - `update-ipsets daemon listening`
      - `READY=1`
  - systemd then kills it with:
    - `start operation timed out. Terminating.`

- The expensive work is inside `engine.New()` after cache load:
  - `pkg/engine/engine.go`
  - order:
    - `reconcileEntriesFromSourceConfig()`
    - `bootstrapMissingEntriesFromDisk()`
    - `repairInvalidEntryTimestamps()`
    - `bootstrapLegacyFailureStarts()`

- The timestamp repair path currently does unnecessary full-disk work on every
  startup, even when the cache is already clean:
  - `pkg/engine/entry_timestamp_sanitize.go`
  - `repairEntryTimestampsFromDisk()` calls:
    - `latestObservedTimestamp()`
    - `firstObservedTimestamp()`
  - before checking whether the entry actually has any invalid timestamps

- `latestObservedTimestamp()` and `firstObservedTimestamp()` both read feed
  history / latest-set evidence from disk:
  - `bootstrapHistoryPoints()`
  - `currentSetStats()`
  - for all configured feeds

Implication:
- The integrity fix itself is correct.
- The regression is that the repair path still scans history for every feed on
  every startup, even when there is nothing left to repair.

### Verified cadence correctness issue

- The current methodology document claims cadence is recomputed from observed deltas:
  - `pkg/web/static/methodology/update-cadence.md`

- The current Go code does not do that. It only seeds cadence once:
  - `pkg/engine/helpers.go`
  - `applyEntryStatsUpdate()` sets `AverageUpdateMins`, `MinUpdateMins`, `MaxUpdateMins` only when `AverageUpdateMins == 0`

- Legacy bash really computed cadence from the full history ledger:
  - `/home/costa/src/firehol/firehol/sbin/update-ipsets`
  - around the `history.csv` processing block
  - It derives average/min/max update intervals from the actual recorded timestamps

Implication:
- The new “10x max update frequency” classification should not trust only the current Go cadence updater.
- We should derive cadence from persisted history ledger data where available.

### Verified synced production data is much larger than the live config

- Synced local cache contains `1599` feed entries:
  - `sudo jq -r '.entries | keys[]' /opt/update-ipsets/data/.cache.json | wc -l`

- The current active config has roughly 298 current sources (observed from recent live status/logs), so the synced dataset clearly includes many historical feeds beyond the live catalog.

- The synced install contains internal history ledgers:
  - `sudo find /opt/update-ipsets/lib -mindepth 2 -maxdepth 2 -name history.csv`

- Example feeds with live history/ledger data confirmed:
  - `abuseipdb_1d`
  - `bogons`
  - `firehol_proxies`
  - `dronebl_anonymizers`

### Verified historical bash feed definitions are available locally

- Legacy bash repo exists locally:
  - `/home/costa/src/firehol/firehol`

- Full git history for `sbin/update-ipsets` is available:
  - `git -C /home/costa/src/firehol/firehol log --oneline --follow -- sbin/update-ipsets`
  - currently `308` historical revisions are reachable for this file

- We already have a parser for legacy bash `update` / `merge` statements:
  - `pkg/config/extract.go`
  - `ExtractLegacyScript(path, ExtractOptions{})`

Implication:
- We can mine old feed configs from past bash commits programmatically instead of hand-copying them.

### Verified historical restoration scope

- Across bash git history, there are `289` unique `update` / `merge` feed names.
- The current expanded Go config contains `298` runtime source names.
- The number of bash-history names not present in the current expanded Go config is `132`.
- Re-audited on April 11, 2026 against the actual synced files:
  - verified missing bash names with synced historical evidence: `22`
  - plus `2` dependency-only source definitions needed so restored historical merges validate and run:
    - `cleantalk_new`
    - `cleantalk_updated`

- The earlier `79` figure was wrong.
  - It was superseded by a direct audit against:
    - `/opt/update-ipsets/data/.cache.json`
    - `/opt/update-ipsets/lib/<feed>/history.csv`
    - `/opt/update-ipsets/lib/<feed>/new/`
    - `/opt/update-ipsets/data/history/<feed>/`
  - The corrected audit artifact is currently at:
    - `/tmp/update-ipsets-restore-audit.json`
    - `/tmp/update-ipsets-restore-details.json`

Verified restorable names with synced historical evidence:
- `blueliv_crimeserver_last`
- `cleanmx_phishing`
- `cleanmx_viruses`
- `cleantalk`
- `cleantalk_1d`
- `cleantalk_7d`
- `cleantalk_30d`
- `coinbl_hosts`
- `coinbl_hosts_browser`
- `coinbl_hosts_optional`
- `coinbl_ips`
- `datacenters`
- `iblocklist_abuse_zeus`
- `ipblacklistcloud_recent`
- `maxmind_proxy_fraud`
- `proxylists`
- `proxz`
- `spamhaus_edrop`
- `uscert_hidden_cobra`
- `xroxy`
- `zeus`
- `zeus_badips`

Dependency-only source definitions needed by restored merges:
- `cleantalk_new`
- `cleantalk_updated`

Breakdown:
- `4` verified restorable candidates still exist in current bash HEAD:
  - `cleantalk`
  - `cleantalk_1d`
  - `cleantalk_7d`
  - `cleantalk_30d`
- The rest require older bash commits for config recovery.

Implication:
- Most of the restoration work requires mining old bash commits, not just copying the current bash script.

### Verified current catalog already knows some removed feed names, but not as active sources

- `configs/firehol.yaml` currently keeps many retired names under `deleted:`
- `pkg/config/catalog_verify_test.go` also documents multiple retired feed families:
  - `bitcoin_nodes`
  - `bm_tor`
  - `cleantalk_*`
  - `cta_cryptowall`
  - `darklist_de`
  - `proxyrss`
  - `ri_connect_proxies`
  - `ri_web_proxies`
  - `sorbs_*`

This is useful as a seed list, but it is not enough by itself because the user explicitly asked to check bash git history and restore only feeds for which synced data still exists.

### Verified not all deleted feeds still have synced data

- Direct checks already confirmed mixed reality:
  - `cleantalk`: history/lib/ledger data exist
  - `proxyrss`: no synced history/lib/ledger found
  - `ri_connect_proxies`: no synced history/lib/ledger found
  - `ri_web_proxies`: no synced history/lib/ledger found
  - `bm_tor`: no synced history/lib/ledger found
  - `bitcoin_nodes`: no synced history/lib/ledger found
  - `cta_cryptowall`: no synced history/lib/ledger found
  - `darklist_de`: no synced history/lib/ledger found
  - `sorbs_anonymizers`: no synced history/lib/ledger found

Implication:
- Restoration must be based on actual synced-data existence, not only on git history or the `deleted:` list.

### Verified recovery from old bash commits is feasible

- The legacy extractor works against historical `sbin/update-ipsets` snapshots, not just HEAD.
- Verified extraction examples:
  - `alienvault_reputation` from an older commit was extracted as a source with:
    - `url=https://reputation.alienvault.com/reputation.generic`
    - `frequency=360`
    - `output=ipset`
    - `category=reputation`
  - `cleantalk` was extracted as a merge with inputs:
    - `cleantalk_new`
    - `cleantalk_updated`

Implication:
- Recovering old source definitions is technically feasible with the existing extractor plus git history walking.

### Verified restored-feed visibility gap

- The public catalog and admin feeds list currently enumerate cache-backed entries:
  - `pkg/engine/public_catalog.go`
  - `pkg/web/admin.go`
- Several restorable feeds have synced on-disk historical evidence but no `.cache.json` entry:
  - examples from the corrected audit:
    - `coinbl_hosts`
    - `coinbl_hosts_browser`
    - `coinbl_hosts_optional`
    - `coinbl_ips`
    - `datacenters`
    - `uscert_hidden_cobra`
- The daemon currently does not bootstrap missing cache entries from the synced ledgers on startup:
  - `pkg/engine/engine.go`
  - cache is loaded, but there is no reconciliation step that synthesizes entries from:
    - `lib/<feed>/history.csv`
    - `data/history/<feed>/*.set`
    - `lib/<feed>/latest`

Implication:
- Restoring the feeds only in YAML is insufficient.
- Without a bootstrap/reconciliation step, some restored feeds would stay invisible or would reappear only after a fresh run, losing the point of the synced 10-year historical data.

### Verified current `unavailable` undercounts for long-dead failing feeds

- Live admin status currently reports:
  - `unavailable = 2`
  - `unmaintained = 81`
  - while `16` feeds have `download_failures > 0`

- The imported d1 cache only proves an old failure streak for:
  - `cleanmx_phishing`
  - `cleanmx_viruses`

- Other currently failing feeds either:
  - are absent from `import-d1/{local-cache,merged-cache}.json`, or
  - exist there without `checked_date` / `download_failures`

- Verified examples:
  - `proxylists`, `proxz`, `xroxy`, `ipblacklistcloud_recent`,
    `blueliv_crimeserver_last`, `coinbl_hosts*` are absent from imported
    cache, but their imported d1 `web/<feed>.json` and `lib/<feed>/latest`
    prove a last successful published state years ago.
  - `zeus`, `zeus_badips`, `cleantalk_new`, `cleantalk_updated` exist in
    imported d1 `web/<feed>.json` with successful published metadata, but
    imported cache still lacks exact failure-streak timing.

- This means:
  - the current bootstrap is working as implemented
  - but the current `unavailable` rule still under-classifies currently
    failing long-stale feeds as `unmaintained` or `empty`
  - broadening that behavior is a consumer-visible health-policy decision

### Verified malformed legacy timestamps still exist in the live cache

- Live `.cache.json` still contains out-of-range Unix-second values:
  - `blueliv_crimeserver_last_7d.processed_date = 152791031527933101`
  - `coinbl_hosts_browser.source_date = 1521527945506`
  - `coinbl_hosts_browser.processed_date = 1521527945506`

- These values are impossible for JSON `time.Time` marshaling and are already
  causing:
  - repeated scheduler persistence errors:
    - `failed to persist scheduler snapshot`
    - `Time.MarshalJSON: year outside of range [0,9999]`
  - intermittent admin integrity `500` when a response includes one of these
    invalid times

- We have valid recovery evidence on disk for these feeds:
  - `lib/<feed>/latest`
  - `lib/<feed>/history.csv`
  - imported d1 `web/<feed>.json`

- This is a real runtime bug and can be fixed without changing the feed-health
  policy.

### Verified current config model gap

- `pkg/config/config.go` `Source` has `Hidden`, but no field for “historical/old/restored”.

Implication:
- We need an explicit config field for restored historical feeds so they can:
  - stay out of the public menu by default
  - still be available through an explicit public toggle
  - remain visible in admin
  - avoid overloading `Hidden`, which means “not in public catalog at all”

### Verified failure-duration gap

- Current cache/runtime data does NOT persist the exact timestamp at which the current failure streak started.
- What we have now:
  - `DownloadFailures`
  - `LastError`
  - `CheckedDate` (last check time, including failed checks)
- What we do NOT have now:
  - `failure_started_at`
  - `last_successful_check_at`

Implication:
- To implement exact “Failed for 10x max update frequency” semantics reliably going forward, we need to persist failure-streak start time.
- For imported existing data, bootstrap logic is needed until the new field has been observed in live operation.

### Verified d1 imported payloads do not carry exact failure-start timestamps

- The current Go cache schema would preserve a legacy JSON `failure_started_date`
  automatically if it existed:
  - `pkg/cache/cache.go`
  - `FailureStartedDate int64 \`json:"failure_started_date,omitempty"\``

- But the imported d1 JSON payloads do not contain that field.

- Verified examples from the synced d1 cache:
  - `/opt/update-ipsets/import-d1/local-cache.json`
  - `cleanmx_phishing`:
    - `checked_date=1745090223`
    - `download_failures=1`
    - no `failure_started_date`
  - `badips`:
    - `checked_date=1773905839`
    - `download_failures=36`
    - no `failure_started_date`

- Verified examples from the synced d1 public feed JSON:
  - `/opt/update-ipsets/import-d1/web/cleanmx_phishing.json`
  - fields present:
    - `updated`
    - `processed`
    - `checked`
    - `errors`
  - field absent:
    - no failure-start timestamp

- The legacy shell-cache importer also confirms the old bash `.cache` contract
  did not define a failure-start field:
  - `pkg/cache/legacy.go`
  - importer maps:
    - `IPSET_DOWNLOAD_FAILURES`
    - `IPSET_CHECKED_DATE`
    - `IPSET_PROCESSED_DATE`
    - `IPSET_SOURCE_DATE`
  - importer does not map any:
    - `IPSET_FAILURE_STARTED_*`

Implication:
- The old d1 payloads preserve consecutive failure counts, but not the exact
  timestamp when the streak began.
- So the current Go runtime is indeed assuming a fresh failure start when the
  streak resumes under Go control.
- If we want to avoid that, we need an explicit bootstrap rule that reconstructs
  an approximate start time from legacy data; the exact timestamp is not present
  in the imported d1 JSON/bash payloads.

### Verified historical-restore compatibility gap

- Several verified historical feeds rely on legacy bash `processor_raw` names
  that are not registered in the current Go processor registry:
  - `blueliv_parser`
  - `parse_cvs_clean_mx_phishing`
  - `hphosts2ips`
  - `parse_client9_ipcat_datacenters`
  - `parse_ipblacklistcloud`
  - `parse_maxmind_proxy_fraud`
  - `parse_uscert_csv`

- Verified from the legacy bash script:
  - `parse_cvs_clean_mx_phishing`
    - `sed 's/|/_/g' -e 's/\",\"/|/g' | cut -d '|' -f 10`
  - `parse_ipblacklistcloud`
    - grep IPv4 values between HTML tags
  - `parse_client9_ipcat_datacenters`
    - CSV columns 1 and 2, convert `start,end` to an IP range for `iprange`
  - `parse_maxmind_proxy_fraud`
    - grep the linked sample values from HTML
  - `parse_uscert_csv`
    - grep `IP Watchlist`, cut CSV column 1
  - `blueliv_parser`
    - jq `.crimeServers[].ip`, excluding nulls
  - `hphosts2ips`
    - remove comments
    - take the hostname columns
    - split to one hostname per line
    - resolve hostnames to IPv4

- Important correction:
- `hphosts2ips` is not a blocker; the current Go processor layer already has:
    - `hostname_resolve`
    - `hostname_resolver`
  - So CoinBlocker host feeds can be expressed as a processor pipeline
    instead of needing a new DNS subsystem.

### Verified current health-tuning failure mode

- The classifier currently uses only two threshold bases:
  - `max_observed_gap`
  - `single_observation_grace`
- Evidence:
  - `pkg/feedhealth/feedhealth.go`
  - `threshold()` at lines 161-172 uses:
    - `entry.MaxUpdateMins * policy.UpdateGapMultiplier`
    - or the single-observation grace period

- The configured multiplier is currently:
  - `configs/firehol.yaml:21`
  - `feed_health_update_gap_multiplier: 10`

- Live evidence from the running daemon shows many non-historical feeds are
  still classified `ok` even though they have not changed for years:
  - total feeds in admin: `344`
  - feeds with `health.class == ok` and `last_update > 365 days ago`: `55`
  - of those, non-historical: `47`
  - excluding feeds explicitly excluded from unmaintained detection: `45`

- Example false negative from live admin API:
  - `iblocklist_org_ubisoft`
    - `last_update`: 2866 days ago
    - `avg_update_mins`: 63 days
    - `max_update_mins`: 814 days
    - `threshold_mins`: 8140 days
    - `health.class`: `ok`

- Verified from the persisted ledger:
  - `/opt/update-ipsets/lib/iblocklist_org_ubisoft/history.csv`
  - unique timestamps: `17`
  - gaps in minutes:
    - `4373`
    - `31963`
    - `175954`
    - `1451`
    - `1450`
    - `1453`
    - `1450`
    - `1442`
    - `1451`
    - `1452`
    - `1173469`
    - `52066`
    - `1441`
    - `8684`
    - `1448`
    - `5777`
  - The single `1173469` minute gap dominates the lifetime max and therefore
    the health threshold.

- The false-negative set is mostly sparse-history feeds:
  - among the `45` non-historical `ok` feeds older than one year:
    - `40` have exactly `6` observed updates
    - `3` have `7`
    - `1` has `8`
    - `1` has `9`

- This means the current lifetime-max rule is too permissive on sparse ledgers.
  It is behaving as coded, but it is not fit for purpose.

### Verified impact of alternative threshold baselines on the live dataset

- Using the current live admin feed payload as evidence:
  - current `10x max_observed_gap` flips `0` of the `45` non-historical
    stale-but-ok feeds
  - `10x avg_update_gap` would flip `45 / 45`
  - `3x max_observed_gap` would flip `44 / 45`
  - `min(10x max_observed_gap, 365 days)` would flip `45 / 45`

- Approximate total class counts under these alternatives:
  - current:
    - `ok=296`
    - `unmaintained=40`
    - `empty=8`
  - `10x avg_update_gap`:
    - `ok=234`
    - `unmaintained=102`
    - `empty=8`
  - `3x max_observed_gap`:
    - `ok=249`
    - `unmaintained=87`
    - `empty=8`
  - `min(10x max_observed_gap, 365 days)`:
    - `ok=243`
    - `unmaintained=93`
    - `empty=8`

### Verified recent-max-gap is not enough on this dataset

- A “recent max gap” heuristic was evaluated against the latest ledger points.
- Result:
  - `10x recent max gap over last 5 gaps` would flip only `35 / 45`
  - `10x recent max gap over last 10 gaps` would flip only `7 / 45`

- Reason:
  - most false-negative feeds only have `6-9` observed updates total
  - so even a “recent” window still includes the large dormant gap

Implication:
- This is not a pure multiplier-tuning problem.
- The real decision is what baseline should define “normal update gap” for
  sparse and long-lived feeds:
  - lifetime max
  - average observed gap
  - lifetime max capped by a fixed ceiling

### Verified existing cache entries are not fully rehydrated from ledger stats

- Startup bootstrap from disk only synthesizes entries that are missing from
  the cache:
  - `pkg/engine/bootstrap_entries.go`
  - `bootstrapMissingEntriesFromDisk()` skips every source that already has a
    cache entry

- `refreshHistoryStatsFromLedger()` is currently only called during finalize:
  - `pkg/engine/finalize.go`

- Live mismatch confirmed for `iblocklist_org_ubisoft`:
  - admin API reports:
    - `version=6`
    - `started_date=1440370835`
    - `last_update=1528290704`
  - ledger file `/opt/update-ipsets/lib/iblocklist_org_ubisoft/history.csv`
    contains `17` unique timestamps with the same first/last timestamps

Implication:
- For imported existing feeds, at least some health inputs are stale cache
  derivatives rather than freshly rebuilt ledger-derived values.
- This does not explain the whole stale-`ok` problem, because the huge
  lifetime max gaps are real in the ledgers too, but it means health tuning
  should rely on fresh ledger-derived stats where possible.

## Decisions

### User decisions already made

1. Health classes are exactly:
   - Unavailable
   - Empty
   - Unmaintained
   - OK

2. Unavailable means:
   - cannot download now
   - and the failure duration exceeds `10x max update frequency`

3. Empty means:
   - feed can download
   - but has zero entries
   - immediate classification, no wait threshold

4. Unmaintained means:
   - feed can download
   - has entries
   - but time since last actual update exceeds `10x max update frequency`

5. Public navigation should hide old/historical feeds by default, with an explicit toggle to show them.

6. Restore historical feeds only when we have synced historical data for them.

7. Terminology contract:
   - `failed` means `unavailable` / `not-available`
   - `abandoned` means `unmaintained`
   - `empty` stays `empty`
   - `ok` stays `ok`

8. Static-by-nature feeds can be excluded from unmaintained detection.

9. Feeds that only ever had a single observed version need a separate grace period
   before they can become unavailable / unmaintained.

10. Both thresholds must be configurable in the user config.

12. User rejected a flat global fix such as only capping `10x max gap`.
    New direction:
    - health logic should use the feed's own past
    - and should also consider the feed category
    - rationale:
      - very stale `attacks` / `malware` feeds are far more problematic than
        equally stale `organizations` / `unroutable` feeds

13. New proposed shape under evaluation:
    - apply a multi-state age-based rule to all feeds
    - category model under analysis:
      - each category defines two thresholds:
        - `healthy cadence`
        - `risky cadence`

14. Legacy imported failure streaks must not be treated as fresh when old d1
    artifacts prove the feed was already failing.
    - Since the old payloads do not contain an exact failure-start timestamp,
      bootstrap an approximate `failure_started_date` from the legacy
      `checked_date` as the earliest proven failure time.
    - Apply this only when the current feed is still failing and there is no
      evidence of a successful recovery after that legacy `checked_date`.

15. After rollout, only 2 feeds show `unavailable` after 2 hours.
    - Re-review the live classifier behavior against imported d1 failing feeds.
    - Fix any remaining bug or policy mismatch.

16. The admin integrity check endpoint currently returns HTTP 500.
    - Review the live error, identify the exact broken code path, and fix it.
      - intended monotonic ladder:
        - `effective_healthy_gap = max(real average historical gap, category healthy cadence)`
        - `ok`: `gap <= effective_healthy_gap`
        - `delayed`: `effective_healthy_gap < gap < category risky cadence`
        - `risky`: `category risky cadence <= gap < 2x category risky cadence`
        - `unmaintained`: `gap >= 2x category risky cadence`
    - analysis required:
      - recommend the two thresholds per category
      - estimate false positives / false negatives on the live dataset

### Draft per-category threshold recommendations from live data

Live analysis source:
- `curl -fsS http://localhost:18888/api/v1/admin/feeds`
- analyzed on April 11, 2026 against `344` live admin feed rows
- sanity filter used for cadence-distribution work:
  - non-historical
  - not hidden
  - `avg_update_mins > 0`
  - `avg_update_mins <= 525600` (discard obviously broken multi-year cadence values)
  - `version > 1`

Observed real average-gap distributions by category, in days:

| Category | n | p50 | p75 | p90 | max |
|---|---:|---:|---:|---:|---:|
| abuse | 23 | 1.01 | 1.07 | 11.46 | 167.11 |
| anonymizers | 23 | 0.09 | 1.00 | 1.43 | 11.29 |
| attacks | 85 | 0.26 | 1.04 | 1.48 | 240.29 |
| malware | 30 | 1.68 | 7.00 | 7.04 | 67.93 |
| organizations | 61 | 11.33 | 63.63 | 63.68 | 72.68 |
| reputation | 18 | 1.25 | 11.33 | 59.06 | 63.74 |
| spam | 15 | 1.00 | 1.00 | 1.00 | 1.00 |
| unroutable | 8 | 2.79 | 56.63 | 67.99 | 67.99 |

Observed feed-shape notes:
- `attacks`
  - mostly daily or faster
  - examples:
    - fastest: `blocklist_de_*`, `dronebl_dictionary_attacks`
    - slower but still active: `dshield`, `iblocklist_dshield`
- `malware`
  - split between sub-daily / daily feeds and weekly-ish feeds
  - examples:
    - fast: `viriback`, `threatfox_ips`
    - slow but still real: `c2_tracker*`, `iblocklist_abuse_palevo`
- `abuse`
  - mostly daily, with a few very slow legacy outliers
  - outliers:
    - `iblocklist_forumspam`
    - `graphiclineweb`
    - `stopforumspam_toxic`
- `anonymizers`
  - mostly sub-daily to daily
  - slow outliers:
    - `ip2proxy_px1lite`
    - `iblocklist_proxies`
- `organizations`
  - clearly bimodal:
    - fast 1-day MISP scanner feeds
    - slow ~60-70 day `iblocklist_org_*` / `iblocklist_isp_*` feeds
  - implication:
    - this category is too broad, but for now needs a compromise threshold
- `reputation`
  - mixed category with both fast reactive feeds and slow long-lived reputation lists
- `unroutable`
  - partly static-by-nature; this category still needs explicit exclusions such as `bogons`

Draft recommended category thresholds:

| Category | Healthy cadence | Risky cadence | Rationale |
|---|---:|---:|---|
| attacks | 1 day | 7 days | Live p75 is ~1 day; operationally these should go stale fast |
| malware | 1 day | 7 days | Similar operational urgency to attacks; weekly risk boundary fits the observed data |
| abuse | 1 day | 14 days | Mostly daily, but a 1-week risky boundary is too aggressive for several long-lived abuse feeds |
| anonymizers | 1 day | 14 days | Usually fast-moving, but not as operationally urgent as attacks / malware |
| spam | 1 day | 14 days | The live spam family is consistently daily; 2 weeks gives enough slack |
| reputation | 3 days | 30 days | Mixed category; needs more slack than abuse/anonymizers but should not stay green for months |
| organizations | 14 days | 90 days | Compromise for the current bimodal category; tighter than today, but not hostile to legitimate slow org lists |
| unroutable | 90 days | 365 days | Many are static or near-static by nature; still keep explicit feed-level exclusions for true static baselines |
| geolocation | 7 days | 30 days | Live providers range from daily to monthly; this keeps monthly lag visible without overreacting |
| asn | 7 days | 30 days | Live providers range from hourly to monthly; 7/30 is a balanced envelope |

Stress-test of the draft table using the proposed age ladder:
- `effective_healthy_gap = max(real average historical gap, category healthy cadence)`
- `ok`: `gap <= effective_healthy_gap`
- `delayed`: `effective_healthy_gap < gap < category risky cadence`
- `risky`: `category risky cadence <= gap < 2x category risky cadence`
- `unmaintained`: `gap >= 2x category risky cadence`

Observed behavior on the live dataset with the draft table:
- `attacks`: `72 ok`, `18 delayed`, `1 risky`, `14 unmaintained`
- `malware`: `22 ok`, `7 delayed`, `0 risky`, `5 unmaintained`, `2 unknown`
- `abuse`: `25 ok`, `1 delayed`, `0 risky`, `14 unmaintained`, `2 unknown`
- `anonymizers`: `17 ok`, `5 delayed`, `0 risky`, `14 unmaintained`
- `spam`: `16 ok`, `4 delayed`, `0 risky`, `0 unmaintained`, `2 unknown`
- `reputation`: `11 ok`, `0 delayed`, `0 risky`, `8 unmaintained`
- `organizations`: `32 ok`, `0 delayed`, `0 risky`, `34 unmaintained`
- `unroutable`: `4 ok`, `0 delayed`, `0 risky`, `4 unmaintained`, `1 unknown`
- `geolocation`: `3 ok`, `1 delayed`, `0 risky`, `1 unmaintained`
- `asn`: `4 ok`

Important findings from the stress test:
- The `attacks` / `malware` recommendation behaves as intended:
  - no feed older than 1 week remains `ok`
  - no feed under 1 day becomes `risky` or `unmaintained`
- The biggest judgment-call categories are:
  - `organizations`
  - `reputation`
  - `unroutable`
- `organizations` is the noisiest category:
  - with `14d / 90d`, `34` feeds become `unmaintained`
  - this is probably directionally correct for dead legacy org feeds, but it is the category most likely to need feed-level overrides or a future split
- `unroutable` still cannot rely on category thresholds alone:
  - explicit exclusions remain mandatory for feeds like `bogons`

Recommendation at this stage:
- Use the draft table above as the first implementation target.
- Keep `exclude_from_unmaintained` as a per-feed override for static baselines.
- Be prepared to revisit `organizations` after the first live rollout; that category is structurally too broad for perfect behavior from category thresholds alone.

14. User decision on April 11, 2026:
    - implement the category-threshold model
    - thresholds must be configurable in config, per category
    - implement the backend logic with these thresholds
    - update the UI if needed
    - explain the logic to users:
      - tooltip in UI
      - methodology page
    - install the service after implementation

15. Implementation status on April 11, 2026:
    - implemented category-configurable health thresholds in config
    - implemented the shared backend ladder:
      - `ok`
      - `delayed`
      - `risky`
      - `unmaintained`
      - plus `empty` and `unavailable`
    - updated public/admin UI to consume the new classes and show richer cadence details
    - added methodology page:
      - `/methodology/feed-health`
    - added health tooltips in public/admin surfaces linking to that methodology page
    - installed and restarted the daemon with the new config
    - verified live API behavior after install:
      - `/healthz` returns `ok`
      - `/api/v1/admin/status` now exposes `delayed` and `risky` counts
      - example live classifications:
        - `bogons` -> `ok` with `exclude_from_unmaintained=true`
        - `criticalpath_log4j` -> `delayed`
        - `maltrail_scanners` -> `risky`
        - `feodo` -> `unmaintained`

16. User decision on April 11, 2026:
    - all feeds in category `unroutable` must be excluded from the
      age-based maintenance ladder
    - rationale verified from live data and config:
      - all bogon/reference-style unroutable feeds already carry
        `exclude_from_unmaintained: true`
      - the remaining `iblocklist_iana_*` feeds were the inconsistent
        outliers inside the same category
    - implementation scope:
      - set `exclude_from_unmaintained: true` on every
        `category: unroutable` feed in `configs/firehol.yaml`
      - update methodology / handbook text to reflect that unroutable
        feeds are currently treated as excluded static/reference feeds
      - reinstall and verify the live daemon

17. Follow-up implementation status on April 11, 2026:
    - applied `exclude_from_unmaintained: true` to the remaining
      `unroutable` outliers:
      - `iblocklist_iana_multicast`
      - `iblocklist_iana_private`
      - `iblocklist_iana_reserved`
    - verified the catalog is now consistent:
      - every `category: unroutable` feed in `configs/firehol.yaml`
        carries `exclude_from_unmaintained: true`
    - updated the methodology page and handbook to document the current
      catalog policy for `unroutable`

18. Pending decision on April 13, 2026:
    - current-failure classification for long-stale feeds
    - live evidence after the timestamp fix:
      - `16` feeds currently have `download_failures > 0`
      - current classifier reports only `2 unavailable`
      - those `2` are the only feeds whose imported d1 cache proves an old
        continuous failure streak:
        - `cleanmx_phishing`
        - `cleanmx_viruses`
    - live alternatives:
      - `A. Keep strict continuous-failure proof`
        - live result: `2 unavailable`
      - `B. Current failure + last successful change older than threshold`
        - live result: `11 unavailable`
        - affected feeds:
          - `blueliv_crimeserver_last`
          - `cleanmx_phishing`
          - `cleanmx_viruses`
          - `cleantalk`
          - `coinbl_hosts`
          - `coinbl_hosts_optional`
          - `coinbl_ips`
          - `ipblacklistcloud_recent`
          - `proxylists`
          - `proxz`
          - `xroxy`
      - `C. Any current fetch failure is unavailable`
        - live result: `16 unavailable`
    - recommendation:
      - `18. B`
      - reason:
        - it matches operator expectation much better than the strict-proof
          rule
        - it still preserves a real distinction between current fetch failure
          and `empty`

19. User decision on April 13, 2026:
    - implement option `18. B`
    - new rule:
      - if a feed is currently failing to fetch
      - and either:
        - the continuous failure streak duration exceeds the unavailable
          threshold, or
        - the last successful change is already older than the unavailable
          threshold
      - then classify it as `unavailable`
    - keep the distinction from `empty`:
      - feeds with no successful published state still rely on failure-streak
        timing, not invented last-change ages
    - installed and replaced the live `/opt/update-ipsets/etc/config.yaml`
    - verified live API behavior after restart:
      - `systemctl is-active update-ipsets` -> `active`
      - `/healthz` -> `ok`
      - all public `unroutable` feeds now report:
        - `health.class = ok`
        - `health.exclude_from_unmaintained = true`

11. Live install verification was blocked by config drift:
    - `install.sh` does not overwrite `/opt/update-ipsets/etc/config.yaml`
    - the daemon is therefore still running with the old live catalog
    - evidence:
      - `systemctl cat update-ipsets` shows `--config /opt/update-ipsets/etc/config.yaml`
      - `install.sh` copied the new catalog only to `/opt/update-ipsets/etc/config.yaml.default`
      - the live config lacks:
        - `feed_health_update_gap_multiplier`
        - `feed_health_single_observation_grace_minutes`
        - `exclude_from_unmaintained`
        - restored historical feeds such as `coinbl_ips`, `spamhaus_edrop`, `zeus`, `cleantalk`
    - consequence:
      - live verification is currently reading the old catalog, so restored historical feeds do not appear
      - `bogons` is still wrongly classified `unmaintained` in the running daemon

    User decision:
    - A. Overwrite `/opt/update-ipsets/etc/config.yaml` with the repo `configs/firehol.yaml`, restart, and verify against the new catalog.

## Plan

1. Audit and materialize the historical-feed candidate set
   - Intersect:
     - feeds found in synced cache / ledgers / history dirs
     - feeds present in legacy bash history
   - Produce the exact list of restorable historical feeds with evidence

2. Extend the config/runtime model for historical feeds
   - Add an explicit source flag for historical/restored feeds
   - Add an explicit source flag for unmaintained-excluded/static feeds
   - Make sure it affects public presentation only, not admin visibility

3. Implement one shared backend health classifier
   - Base it on persisted feed history and current error state
   - Include:
     - class
     - avg update frequency
     - max update frequency
     - time since last successful change
     - time since failure streak started
     - threshold used

4. Use ledger-based cadence derivation where possible
   - Prefer persisted history ledger over the currently weak one-time cadence seed
   - Keep cache fields as fallback only
   - Detect feeds with just one observed update and apply the separate grace threshold

5. Persist exact failure-streak timing
   - Add `failure_started_at` tracking to cache/runtime bookkeeping
   - Bootstrap imported entries conservatively so existing production data classifies immediately

6. Expose the new health contract through both APIs
   - Public catalog rows
   - Public feed metadata
   - Admin feed rows
   - Admin feed detail

7. Restore historical feeds into config
   - Recover definitions from bash history using the extractor
   - Add only feeds with verified synced data
   - Make them historical/restored in config so the public toggle controls visibility

8. Implement UI
   - Public:
     - health class and cadence details
     - old/historical toggle in the menu and catalog surfaces
   - Admin:
     - same health class
     - same cadence details in table and modal

9. Install, restart, verify
  - run tests
  - install/restart daemon
  - verify restored feeds appear correctly
  - verify health classes with live API responses

10. Fix malformed legacy timestamps and JSON time marshaling
   - sanitize imported entry timestamps from on-disk evidence during
     engine startup/reload
   - guard scheduler snapshot generation against out-of-range Unix timestamps
   - sanitize integrity response timestamps before JSON encoding

11. Re-review `unavailable` using live failing feeds
   - separate code bugs from the consumer-visible policy choice
   - quantify the impact of candidate rules on the live dataset before
     changing classification semantics

12. Remove the startup-time timestamp-repair regression
   - short-circuit timestamp repair when an entry has no invalid JSON-range
     timestamps
   - keep the repair logic itself unchanged for genuinely broken entries
   - verify the daemon reaches readiness again after restart

## Implied decisions

- The classification logic should be backend-owned and API-visible, not duplicated in TypeScript.
- Historical/restored feeds should remain queryable and viewable when explicitly requested, not merely stored as dead admin records.
- Missing historical ledgers should not block classification for current feeds; fallback to cache fields is acceptable there.
- Existing `Hidden` semantics should stay unchanged for synthetic/internal feeds.
- The static bogon family should be excluded from `unmaintained` using the existing semantic grouping already present in the catalog:
  - this now expands to the whole `unroutable` category
  - including every feed with `use: [bogons]`
  - including the synthetic `rfc_reserved` baseline
- `cleantalk_new` and `cleantalk_updated` need to return to the catalog as hidden historical support feeds so the restored `cleantalk*` merges validate and can expose their historical data, without adding extra public feeds that were not in the audited restorable set.
- Terminology contract:
  - `failed` is renamed to `unavailable`
  - `abandoned` is renamed to `unmaintained`
  - `empty` stays `empty`
  - `ok` stays `ok`
- Empty successful fetches decision:
  - if a feed is fetched successfully and the resulting collection is empty,
    this must be treated as a valid published state
  - the engine must zero out the stored set/counts and publish the feed as
    `empty`
  - keeping the last non-empty successful set visible after a successful
    empty run is incorrect because it serves stale data to users
- Static-by-nature feeds need an explicit config escape hatch so they do not get marked `unmaintained` just because they rarely or never change.
- The `10x max update frequency` multiplier and the `10 day single-update grace period` must not be hardcoded in classification logic.
- Invalid legacy timestamps in imported cache entries should be repaired from
  trustworthy on-disk evidence, not guessed from digit length.

## Testing requirements

- Unit tests for health classification:
  - unavailable
  - empty
  - unmaintained
  - ok
  - threshold edge cases
  - excluded/static feeds
  - single-update grace period
  - missing ledger fallback

- Unit tests for ledger cadence derivation
  - arithmetic average/min/max from real history points
  - single-point history fallback
  - malformed rows ignored

- Config/extractor tests for restored historical feeds
  - recovered from legacy bash definitions
  - marked historical/restored in config

- API tests
  - public `/api/v1/sets`
  - public `/api/v1/sets/{name}`
  - admin feeds/detail responses

- UI build verification
  - `pnpm -C ui build`

- Full backend verification
  - `go test ./...`
  - `go vet ./...`

## Documentation updates required

- `AGENTS.md`
  - document the historical-feed flag and public toggle contract
  - document the shared health classification contract

- Methodology docs
  - add/update methodology for feed health / maintenance classification
  - fix `update-cadence.md` so it matches the actual implementation after this work
