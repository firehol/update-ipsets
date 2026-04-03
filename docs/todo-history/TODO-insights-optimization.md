# TODO - Insights Optimization

## Purpose

Fit for purpose goal: make the `insights` phase cheap enough to run continuously on production-sized datasets without wasting CPU, memory, or I/O, while preserving correctness and public API behavior.

## TL;DR

- Analyze what per-feed data already exists after each update.
- Trace exactly which artifacts the `insights` phase reads today and why.
- Recommend concrete optimization paths to minimize resource use during `insights`, with pros, cons, implications, and risks.

## Analysis

### Current live behavior observed

- Local production service on 2026-04-19 was in `current_phase = "insights"` while consuming about one full core (`ps` showed `update-ipsets` around `85.8%` CPU).
- The live install currently exposes:
  - `288` output feeds from `/api/v1/sets`
  - `248` feed history directories under `data/history`
  - `21,263` retained full snapshot files under `data/history/*/*.set`
- A few feeds are especially expensive if a code path scans all snapshots:
  - `dronebl_auto_botnets`: `2288` snapshots
  - `blocklist_net_ua`: `1074`
  - `botscout`: `853`
  - `dronebl_dictionary_attacks`: `752`

### Per-feed information already computed and maintained

#### Core persisted state

- Cache entry in `.cache.json` / in-memory `cache.Entry`
  - updated in `pkg/engine/finalize.go`
  - contains source timestamps, processed timestamps, counts, min/max bounds, cadence stats, rotation stats, category, maintainer, downloader state, and health inputs

#### Primary set artifacts

- `data/<name>.ipset` or `data/<name>.netset`
  - final published processed list with header
- `lib/<name>/latest`
  - binary latest set used by `FileSet` readers and comparisons
- `data/<name>.source`
  - trigger/source marker path

#### History and retention internals

- `lib/<name>/history.csv`
  - append-only full ledger of `DateTime,Entries,UniqueIPs`
  - written on every successful finalize
- `data/history/<name>/<timestamp>.set`
  - full set snapshot per successful update
  - written by `keepHistorySnapshot()`
  - pruned by longest declared retention window when configured
- `lib/<name>/changesets.csv`
  - append-only add/remove ledger
- `lib/<name>/new/<timestamp>`
  - retained "added at update X and still present" incremental sets
- `lib/<name>/retention.csv`
  - append-only removed-age ledger
- `lib/<name>/retention.json`
  - derived summary used by the public site and insights
- `lib/<name>/histogram`
  - bash-compatible retention cache

#### Public / web per-feed artifacts

- `web/<name>.json`
  - per-feed metadata/spec payload
- `web/<name>_history.csv`
  - bounded public history window
- `web/<name>_changesets.csv`
  - bounded public churn window, bootstrap row removed
- `web/<name>_retention.json`
  - public copy of retention summary
- `web/<name>_<geoProvider>.json`
  - country composition per geo provider
- `web/<name>_asn_<provider>.json`
  - ASN composition per ASN provider
- `web/<name>_bogons_<provider>.json`
  - bogon overlap per bogon provider
- `web/<name>_comparison.json`
  - pairwise overlap rows against other feeds
- `web/<name>_insights.json`
  - final derived insights payload

### What `insights` actually needs

The current rule catalog has 17 rules split into 4 dependency shapes:

1. Local temporal rules
   - size variation
   - churn high / low
   - currently listed age / observation wall
   - removed age / multiple retention policies / permanent bans
   - dependencies:
     - size history
     - churn history
     - retention summary

2. Local geography rules
   - country concentrated / diverse / single country
   - dependencies:
     - per-feed geo provider JSON

3. Local composition rules
   - bogon present
   - infrastructure present
   - dependencies:
     - per-feed bogon JSON across providers
     - per-feed ASN JSON from preferred provider

4. Catalog-wide relationship rules
   - independent
   - subset_of
   - cross_category_overlap
   - dependencies:
     - per-feed `_comparison.json`

### How `insights` works today

#### Phase entry

- `RunOnce()` enters `RunPhaseInsights` after geo / bogon / ASN / metadata.
- `pkg/engine/run.go:407` calls `writeInsightsForFeeds(insightUpdated, webOutDir)`.

#### Whole-catalog sweep

- `writeInsightsForFeeds()` ignores the `updatedNames` argument and loops over `e.outputNames()` for every run that reaches the heavy block.
- file: `pkg/engine/insights.go:82`

#### Eager full snapshot assembly

- For each feed, `buildSignalSnapshot()` eagerly assembles all fields, whether the eventual rules need them or not.
- file: `pkg/engine/insights.go:94`
- It always loads:
  - size series
  - churn series
  - retention histograms
  - top countries
  - bogon share
  - ASN facts
  - overlap facts

#### Expensive local temporal path

- `readSizeSeries()` calls `HistorySeries()`
  - `pkg/engine/insights_series.go:7`
- `HistorySeries()` loads:
  - full `lib/<name>/history.csv`
  - fallback `web/<name>_history.csv`
  - full snapshot history from `data/history/<name>/*.set`
  - then merges everything and only afterwards trims to the last `WebChartsEntries`
  - `pkg/engine/query.go:111`
- `historyFromSnapshots()` opens and parses every snapshot file it finds
  - `pkg/engine/query.go:205`
- `loadSnapshotSet()` fully parses each snapshot with `iprange.ParseReader()`
  - `pkg/engine/retention.go:28`

#### Expensive churn path

- `readChurnSeries()` calls `ChangesetSeries()`
  - `pkg/engine/insights_series.go:23`
- `ChangesetSeries()` reads the full `lib/<name>/changesets.csv`
  - `pkg/engine/query.go:143`

#### Already-cheap temporal path

- Retention insights already use the precomputed `lib/<name>/retention.json`
  - `pkg/engine/insights.go:150`
- This is the right shape: bounded derived summary, not raw snapshot scan.

#### Composition / overlap path

- Geography reads the chosen provider's per-feed JSON
  - `pkg/engine/insights.go:196`
- Bogon share loops over every configured bogon provider and reads each per-feed JSON, taking the max share
  - `pkg/engine/insights.go:319`
- ASN facts read the preferred ASN provider JSON
  - `pkg/engine/insights.go:269`
- Overlap facts read the per-feed `_comparison.json`
  - `pkg/engine/insights.go:355`

### Important nuance: why a global sweep is partly justified today

- A normal feed update does not only affect that feed's overlap facts.
- `writeComparisonFiles()` computes pairs involving the updated feed, then writes rows for both sides of each pair.
- So if `A` updates:
  - `A_comparison.json` changes
  - every peer `B_comparison.json` may also change because the `B ↔ A` row is refreshed
- files:
  - `pkg/engine/output.go:425`
  - `pkg/engine/output.go:447`

Implication:

- The current whole-catalog `insights` sweep is semantically justified for the 3 overlap rules.
- It is **not** justified for reloading local history/churn inputs for every feed during that same sweep.

### Main waste identified

The big waste is not the rule math itself.

- The rule engine just evaluates 17 deterministic rules over an already-built snapshot.
- The expensive part is eager snapshot assembly:
  - every feed
  - every heavy run
  - full local history paths
  - even when only overlap-based rules need global recomputation

### Resource-minimizing recommendations

#### Option 1: Stop reading full history internals in the hot insights path

Change:

- Make `insights` read bounded public artifacts first:
  - `web/<name>_history.csv`
  - `web/<name>_changesets.csv`
  - `web/<name>_retention.json`
- Use staged `webOutDir` first, then live `webDir`.
- Fall back to internal ledgers only if the public file is genuinely missing.
- Do **not** scan `data/history/<name>/*.set` from `insights`.

Why this is valid:

- Current rules only need the last bounded public windows for size and churn.
- Retention rules already use a derived summary.

Pros:

- Biggest immediate CPU / I/O reduction with minimal semantic change.
- Removes the `21k+` snapshot parse risk from the hot path.
- Keeps the current full-catalog sweep, so overlap correctness is preserved.

Cons:

- `insights` becomes explicitly dependent on the public artifact generation order.
- Needs careful staged/live fallback logic to avoid regressions during the same batch.

Implications:

- This should be treated as the minimum safe first fix.

Risks:

- Legacy installs missing public files may need fallback coverage.
- Any mismatch between public CSV semantics and old `HistorySeries()` semantics must be tested explicitly.

#### Option 2: Split insights by dependency class

Change:

- Separate rule execution into groups:
  - local temporal
  - local geography
  - local composition
  - global overlap
- Recompute only the affected groups:
  - updated feed only for local temporal rules
  - updated feed only for local geo/asn/bogon rules
  - all feeds when a provider update requires it
  - all feeds for overlap rules after comparison files are refreshed

Pros:

- Preserves correctness for overlap rules.
- Prevents global reload of local history inputs.
- Makes invalidation rules explicit and maintainable.

Cons:

- More code than Option 1.
- Needs a merge step to assemble the final `_insights.json`.

Implications:

- This is the best medium-term architecture if insights stay as a separate phase.

Risks:

- Easy to get invalidation wrong if rule grouping is not made explicit in code and tests.

#### Option 3: Precompute tiny insights-input summaries earlier in the pipeline

Change:

- Generate a compact per-feed sidecar with exactly the fields each rule group needs, for example:
  - bounded size series
  - bounded churn series
  - retention percentiles or histograms
  - geo top-country summary
  - max bogon share
  - infra share
  - overlap summary by category and best rows
- Then `insights` becomes almost pure formatting.

Pros:

- Cleanest long-term design.
- Very cheap final insight generation.
- Makes dependencies explicit.

Cons:

- Most invasive refactor.
- Adds new artifact contracts and migration concerns.

Implications:

- Worth it only if Costa wants a proper phase redesign, not just a hot fix.

Risks:

- New sidecar schema must stay synchronized with rule semantics.

#### Option 4: Remove eager loading of unused data

Change:

- Stop loading `TopASNs` unless a rule actually needs them.
- Prefer a tiny ASN summary path when only `InfraShare` is needed.
- Prefer a precomputed max bogon share instead of reopening every bogon JSON per feed if that summary can be produced earlier.

Pros:

- Small additional win.
- Low conceptual risk.

Cons:

- Smaller impact than Options 1 and 2.
- Can devolve into piecemeal micro-optimizations if done alone.

Implications:

- Good companion change, not the main fix.

Risks:

- Minimal, provided the current rule set remains unchanged.

#### Option 5: Parallelize the insights sweep

Change:

- Run per-feed insight generation with a worker pool.

Pros:

- Reduces wall-clock time.

Cons:

- Does not reduce total CPU or I/O.
- Can increase contention and hide inefficient data paths instead of fixing them.

Implications:

- Not recommended as the first optimization.

Risks:

- Makes the machine busier while preserving the same waste.

### Recommended sequence

1. Implement Option 1 first.
2. Then implement Option 2 if more reduction is needed.
3. Apply Option 4 opportunistically while touching the snapshot builder.
4. Treat Option 5 as a last-mile wall-clock optimization only after the data path is fixed.

### Recommendation rationale

- Option 1 removes the worst waste immediately without changing the public semantics or the global overlap invalidation behavior.
- Option 2 is the right structural fix if `insights` should remain a dedicated phase.
- Anything that starts with parallelism before fixing the data path is the wrong priority.

## Decisions

### 2026-04-19 - User decision

- Implement waste elimination now.
- Start with Option 1 first:
  - replace full-history hot-path reads in `insights`
  - use bounded staged/live public artifacts first
- Keep global overlap-driven invalidation intact.
- Do not add parallelism.

## Plan

1. If implementation is approved, replace `HistorySeries()` / raw `changesets.csv` hot-path reads in `insights` with bounded staged/live public artifact readers.
2. Add tests proving the new readers preserve current size/churn rule semantics for the last `WebChartsEntries` window.
3. Add tests proving overlap-driven global recomputation still happens when comparison files change for non-updated feeds.
4. Measure CPU and wall-clock again before deciding whether rule-group splitting is still needed.

## Implied Decisions

- Preserve existing public insight semantics unless a recommendation explicitly proposes changing them.
- Treat correctness of public per-feed insight endpoints as mandatory.

## Testing Requirements

- Verify `insights` no longer scans `data/history/<feed>/*.set` during the hot path.
- Verify staged `webOutDir` files are preferred over live files during the active batch.
- Verify fallback to live public artifacts for feeds not rebuilt in the current batch.
- Verify overlap rules still refresh correctly for peers whose `_comparison.json` changed indirectly.
- Verify young / legacy feeds with missing public history files still have a safe fallback path.
- Verify no regression in `/api/v1/sets/{name}/insights` availability.

## Documentation Updates Required

- `AGENTS.md` has already been updated with the current overlap-driven `insights` invalidation contract.
- If implementation follows later, update `AGENTS.md` again with the final optimized artifact-read contract.
