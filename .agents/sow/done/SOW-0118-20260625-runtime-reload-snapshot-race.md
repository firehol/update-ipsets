# SOW-0118 - Runtime Reload Snapshot Race Audit

## Status

Status: completed

Sub-state: activated after SOW-0117 closure. This is a focused follow-up for a
valid runtime reload concurrency finding that was too broad to hide inside the
panic/liveness cleanup loop. User direction: for every design choice, choose the
long-term-best option.

## Requirements

### Purpose

Ensure runtime reloads cannot race with active engine, downloader, public,
integrity, metadata, or entity code that reads runtime configuration.

### User Request

Fix all deadlock/liveness findings from production and external review, while
preserving the application contract and avoiding hidden broad rewrites.

### Assistant Understanding

Facts:

- `Engine.ReloadContext()` writes the engine runtime under `e.mu`.
- Many engine paths read `e.runtime` fields directly without taking `e.mu`.
- Existing reload-overlap tests cover already-locked public/runtime readers,
  reload mutex behavior, and scheduler activity snapshots, but they do not
  cover the active processing paths targeted by this SOW.
- The direct-read surface is broad enough to deserve its own focused design and
  review.

Inferences:

- The safest long-term approach is likely a runtime snapshot contract:
  operation entrypoints take one `Runtime` copy and pass it down, or all callers
  use a concurrency-safe runtime accessor.
- A mechanical `e.runtime` replacement across many files could change behavior,
  performance, or stale-runtime semantics if done without a contract.

Unknowns:

- Which direct reads are reachable concurrently with `ReloadContext()` in real
  daemon operation.
- Whether the right design is operation-local snapshots, reload admission
  through the engine lane, atomic runtime storage, or targeted accessor
  replacement.

### Acceptance Criteria

- A race-detector test reproduces or disproves the reload/runtime race for at
  least one active engine path. If a deterministic pre-fix race reproduction is
  not possible, the SOW must record the structural reason, the substitute proof,
  and acceptance from at least two external reviewers. If all six external
  reviewers fail to return usable results for that proof, user review may
  substitute and must be recorded explicitly.
- All runtime read paths reachable concurrently with reload are inventoried and
  classified.
- The chosen fix has explicit contract text for whether in-flight work uses the
  old runtime snapshot or observes the reloaded runtime.
- In-flight background work is proven to keep one snapshot generation and not
  observe later reload state mid-operation.
- Scheduler/admin/public builders that need both config and runtime are proven
  to use one coherent generation.
- Background entity refresh, entity health refresh, entity artifact mutation,
  integrity refresh, and metadata/entity fan-out paths are either converted to
  operation snapshots or explicitly proven safe with file/line evidence.
- Runtime-ledger, retention, and history derived-cache update paths touched by
  this SOW are proven with file/line evidence to append/write durable files
  before updating derived in-memory cache state, or are changed so this ordering
  is enforced. The proof or fix must be covered by a focused regression test or
  structural test.
- Durable-write failure semantics are explicit for every ledger/cache update
  path touched by this SOW. On durable-write failure, the derived in-memory
  cache must not advance unless the SOW records a deliberate divergence with
  file/line evidence and external-review acceptance.
- Old-generation pointers captured by an operation snapshot remain valid until
  the operation finishes. This must be proven for downloader clients,
  geo-provider caches, runtime ledger caches, retention-window maps, and ASN
  lookup caches, or the implementation must add the needed lifetime guard.
- Old-generation downloader clients reachable from an admitted operation
  snapshot must not be closed by reload. If downloader closure is added during
  implementation, it must be guarded by explicit lease/release ownership.
- No operation snapshot is captured while already holding the engine write
  mutex in the same goroutine. Validation must include an explicit nested-lock
  scan or structural test for final snapshot constructor names.
- A focused assignment scan for `e.state` and equivalent receiver aliases is
  completed before closure. If any reassignment path beyond engine construction
  is found, the `e.state` exclusion is revisited with file/line evidence before
  the SOW can close.
- A focused `cfg.Runtime` mutation-hazard scan is completed before closure.
  Every remaining `cfg.Runtime` read must be converted to the captured
  operation/request/build `Runtime` value or classified as startup, reload, or
  runtime-construction code with file/line evidence.
- A direct-read inventory generated from the same-failure scan is completed
  before implementation and reconciled after implementation.
- An accessor-generation inventory for `Config()` / `Runtime()` calls is
  completed before implementation and reconciled after implementation, because
  accessor calls are individually race-safe but can still mix reload
  generations.
- The 50 current single-accessor sites receive caller-chain classification and
  terminal handling before closure: converted to an operation/request/build
  snapshot, proved startup-only/already coherent, or documented as
  intentionally latest-at-use with evidence.
- Specs that define reload and pipeline behavior are updated with the final
  operation-snapshot contract before closure.
- `go test -race -count=10 ./pkg/engine ./pkg/scheduler ./pkg/web` passes
  after the fix.
- Touched hot paths show no meaningful benchmark regression, or the SOW records
  a benchmark-gap note explaining why no meaningful local benchmark exists. The
  gap note must be written before closure and must state which touched packages
  lack meaningful benchmarks and what timing/allocation evidence substitutes.

## Analysis

Sources checked:

- `pkg/engine/engine.go`
- `pkg/engine/run.go`
- `pkg/engine/download_stage.go`
- `pkg/engine/artifact_stage.go`
- `pkg/engine/status_snapshot.go`
- `rg -n "\be\\.runtime\\." pkg/engine --glob '*.go' --glob '!**/*_test.go'`

Current state:

- `Engine.ReloadContext()` records the old web directory and assigns
  `e.runtime` under `e.mu`.
- Direct runtime reads exist in active processing and serving paths, including
  `pkg/engine/run.go`, `pkg/engine/download_stage.go`,
  `pkg/engine/artifact_stage.go`, `pkg/engine/entity_feed_sidecar_build.go`,
  `pkg/engine/status_snapshot.go`, `pkg/engine/metadata_write.go`,
  `pkg/engine/feed_body_stage.go`, `pkg/engine/integrity_check.go`, and
  related helpers.

Risks:

- A real data race can corrupt runtime reads or fail under `-race` once test
  coverage exercises the overlap.
- A broad runtime accessor rewrite can accidentally change whether in-flight
  work uses old or new runtime values.

External gap-review round 1:

- `glm`: usable. Confirmed active engine/download paths can race with reload
  and highlighted reload-swapped engine-owned pointers.
- `qwen`: usable. Confirmed direct `e.runtime` / `e.cfg` races, scheduler
  mixed snapshots, and missing race tests for active paths.
- `kimi`: usable. Confirmed the SOW needed explicit visibility decisions,
  direct-read inventory, and active-path tests.
- `deepseek`: usable. Confirmed RunOnce, FetchAndStage, scheduler, integrity,
  and web/admin mixed-snapshot risks.
- `minimax`: excluded for this round after two failed/overlong attempts.
- `mimo`: excluded for this round after two failed attempts.

Validated reviewer findings:

- Accepted in SOW-0118: direct `e.cfg` / `e.runtime` active-path reads,
  reload-swapped `e.downloads`, reload-swapped `e.geoProviders`, scheduler
  mixed config/runtime snapshots, missing active-path race tests, integrity and
  metadata snapshot gaps, and the reload-swapped `e.retentionMaxWindow` map used
  by history-snapshot pruning.
- Already safe / not a bug in this SOW: `StatusSnapshot*` locks while copying;
  `runtimeLedgerSnapshot` locks while copying the runtime and ledger cache
  pointer; `lookupContextSnapshot` locks while copying config/runtime and
  lookup cache pointers; ASN lookup cache leases keep retired databases open
  until release.
- Derivative candidate: public route root live-rebinding after `WebDir` or
  `WebDirForIPSets` reload. This is a reload contract gap, not the same data
  race class.

External plan-review round 1:

- `qwen`: not ready. Valid blockers: concrete snapshot type and pointer
  lifetime contract were underspecified; scheduler and surface route mixed
  snapshots needed stronger treatment; race validation needed repeated runs.
- `deepseek`: ready with minor notes. Valid notes: reuse the existing
  `configRuntimeSnapshot()` pattern and classify `querySetCache`.
- `mimo`: not ready. Valid blockers: snapshot type definition, pointer
  immutability contract, exact same-failure scan, and broader representative
  tests were underspecified.
- `glm`: not ready. Valid blockers: `e.retentionMaxWindow` was missing from the
  inventory; scheduler was missing from validation; the acceptance criterion was
  too weak; ledger-cache handoff and derivative closure needed explicit
  treatment.
- `kimi`: no usable final result available from this round after the session was
  lost before a conclusion. Re-run on the revised plan.
- `minimax`: still running during this revision. If it completes, validate its
  findings before the next review round; if it fails or times out, re-run once
  on the revised plan.

External plan-review round 1 completion update:

- `minimax`: not ready. Valid additional blockers: background entity refresh
  operations were not explicitly enumerated; `e.cfg.Runtime` mutation by
  runtime overrides needed a snapshot rule; the direct-read inventory needed to
  be a required artifact; and downloader/client snapshot usage needed to be
  stated as a caller rule.

External plan-review round 2:

- `glm`: ready. Non-blocking notes: clarify `asnLookupCache` pointer stability,
  background entity-refresh wave snapshot granularity, and reload publish-then-
  validate ordering during implementation.
- `deepseek`: ready. Non-blocking notes: `e.state` is a separate stable-pointer
  concern outside the reload-swapped-field class; large unsafe-convert count
  should be batched by caller chain.
- `mimo`: ready. Non-blocking notes: implement field-line fixes by caller chain
  rather than line-by-line.
- `minimax`: not ready. Valid blockers: the inventory missed accessor-method
  generation risks in web/admin/scheduler/engine; the pre-fix active-path race
  target was not named; runtime-ledger cache durability needed a concrete plan
  statement; and specs must be updated unconditionally.
- `kimi`: ready. Non-blocking note: the direct-read inventory scan needed to
  include non-`e` engine pointer variable names such as `eng.cfg`; this was
  validated as a real inventory gap and fixed before the next plan-review
  round.
- `qwen`: ready, but reviewed the pre-expansion 331-hit direct inventory. Its
  approval is treated as stale after the inventory scan was broadened.
- `kimi`: not ready in the broadened-inventory re-review. Valid blockers:
  caller-chain reachability classification for the 285 unsafe direct reads,
  explicit `e.state` exclusion evidence, and clearer wording that the active
  thread objective remains open until SOW-0119 is completed even though the
  SOW framework permits SOW-0118 to close once the derivative is concrete.

External revised plan-review round 3:

- `qwen`: ready. Non-blocking notes included making the primary race target
  explicit and watching retention-cohort ordering during implementation.
- `mimo`: not ready in the latest available revised review. Valid blockers:
  add objective nested-lock validation for snapshot captures, prove old
  generation pointer stability, strengthen SOW-0119 cache invalidation, and
  define objective criteria for structural race-proof fallbacks.
- `kimi`: not ready in the latest available revised review. Valid blockers:
  promote runtime-ledger preservation to a hard gate before touching retention
  and history write paths, verify retired snapshot object safety, and classify
  the 50 single-accessor sites.
- `glm`: not ready in the latest revised review. Valid blocker: runtime-ledger
  durable-first ordering was load-bearing design text, but not yet an explicit
  acceptance criterion or validation-plan item.
- `minimax`: ready in the latest available revised review. Non-blocking notes
  overlapped with the runtime-ledger ordering and implementation-evidence
  concerns above.
- `deepseek`: no usable final result was recovered for this revised round; rerun
  after the SOW text updates before implementation.

## Pre-Implementation Gate

Status: plan-approved-for-implementation

Problem / root-cause model:

- Runtime reload mutation is protected by `e.mu`, but many runtime reads are
  not. Locking only writes is not enough in Go; reads and writes must be
  synchronized by the same happens-before relationship, or the value must be
  immutable/atomic.

Evidence reviewed:

- `pkg/engine/engine.go:291`: reload takes `e.mu`, then assigns
  `e.cfg`, `e.runtime`, `e.downloads`, `e.geoProviders`, and `e.ledgerCache`
  at `pkg/engine/engine.go:293`, `pkg/engine/engine.go:294`,
  `pkg/engine/engine.go:296`, `pkg/engine/engine.go:301`, and
  `pkg/engine/engine.go:307`.
- `pkg/engine/engine.go:309`: reload rebuilds `e.retentionMaxWindow` through
  `buildRetentionMaxWindow`, which assigns the map at
  `pkg/engine/engine.go:227`.
- `pkg/engine/engine.go:26` declares `state *cache.State`, and
  `pkg/engine/engine.go:169` assigns it during engine construction.
  `pkg/engine/engine.go:291` through `pkg/engine/engine.go:310` shows reload
  does not assign `e.state`; state is therefore not reload-swapped, though
  callers that combine `e.state` with config/runtime still need snapshot
  coherence for the config/runtime side.
- `pkg/engine/feed_body_stage.go:288` and
  `pkg/engine/feed_body_stage.go:331`: history snapshot append/prune reads
  `e.retentionMaxWindow` directly and also reads `e.runtime.HistoryDir` at
  `pkg/engine/feed_body_stage.go:294` and
  `pkg/engine/feed_body_stage.go:338`.
- `pkg/engine/finalize.go:99` appends durable `history.csv` before
  `pkg/engine/finalize.go:107` updates the runtime history cache.
- `pkg/engine/retention_update.go:214` appends durable `changesets.csv` before
  `pkg/engine/retention_update.go:218` updates the runtime changeset cache.
- `pkg/engine/retention_update.go:538` appends durable `retention.csv` before
  `pkg/engine/retention_update.go:546` updates the runtime retention-past
  cache.
- `pkg/engine/retention_update.go:153` currently replaces in-memory retention
  cohorts before `pkg/engine/retention_update.go:569` writes
  `retention_cohorts.csv`. This violates the durable-first runtime-ledger
  contract and must be fixed during implementation.
- `pkg/engine/run.go:239`: active processing starts after the run has entered
  the engine lane, but the body reads live runtime/config state directly at
  `pkg/engine/run.go:255`, `pkg/engine/run.go:308`,
  `pkg/engine/run.go:334`, and `pkg/engine/run.go:349`.
- `pkg/engine/download_stage.go:79`: downloader work reads live config,
  runtime, and downloader client state directly, including
  `pkg/engine/download_stage.go:83`,
  `pkg/engine/download_stage.go:249`,
  `pkg/engine/download_stage.go:264`,
  `pkg/engine/download_stage.go:268`,
  `pkg/engine/download_stage.go:488`, and
  `pkg/engine/download_stage.go:495`.
- `pkg/engine/artifact_stage.go:20`: artifact download/materialization has
  the same issue for artifacts and DroneBL child materialization, including
  live `e.downloads`, `e.runtime`, and `e.cfg` use.
- `pkg/engine/geoloc.go:74`: geo provider processing reads `e.geoProviders`
  directly even though reload replaces `e.geoProviders`.
- `pkg/engine/integrity_check.go:77`: integrity checks capture `webDir` and
  `baseDir` from live runtime but continue reading `c.e.cfg` during the scan
  at `pkg/engine/integrity_check.go:100`.
- `pkg/engine/metadata_write.go:54`: metadata writes construct a run object
  using live `e.cfg` and `e.runtime`, then continue using live runtime fields
  in later write methods such as `pkg/engine/metadata_write.go:161`,
  `pkg/engine/metadata_write.go:182`, and
  `pkg/engine/metadata_write.go:252`.
- `pkg/scheduler/download_loop.go:21`: scheduler snapshots call
  `Config()` and `Runtime()` separately, so a reload can produce a mixed
  config/runtime pair.
- `pkg/scheduler/download_loop.go:22`: artifact item snapshots repeat the same
  separate `Config()` and `Runtime()` pattern.
- `pkg/scheduler/actions.go:45`, `pkg/scheduler/actions.go:71`, and
  `pkg/scheduler/automatic_due.go:20`: scheduler action/due code reads
  config through separate accessor calls and must be classified as coherent,
  single-generation, or intentionally latest-at-use.
- `pkg/scheduler/processing_loop.go:11`: the processing cadence reads runtime
  once for the loop lifetime; this is not a data race by itself, but it is a
  reload-visibility contract that must be explicit.
- `pkg/engine/runtime.go:303` through `pkg/engine/runtime.go:313`: runtime
  overrides mutate both `e.runtime` and `e.cfg.Runtime` under `e.mu`. Snapshot
  consumers must treat the captured `Runtime` value as the authoritative runtime
  generation and must not read `snapshot.cfg.Runtime` as live runtime policy.
- Focused `cfg.Runtime` hazard scan:
  `rg -n '\bcfg\.Runtime\b|\.cfg\.Runtime\b' pkg/engine pkg/scheduler pkg/web --glob '*.go' --glob '!**/*_test.go'`
  currently finds 15 production hits. Eleven are consumer-side policy reads
  that must use the captured `Runtime` value after implementation:
  `pkg/web/admin.go:688`, `pkg/web/admin.go:949`,
  `pkg/scheduler/snapshot_build.go:58`,
  `pkg/scheduler/snapshot_build.go:110`,
  `pkg/engine/home_aggregates.go:126`,
  `pkg/engine/public_catalog.go:87`,
  `pkg/engine/home_entity_builders.go:222`,
  `pkg/engine/feed_health.go:16`, `pkg/engine/integrity.go:266`,
  `pkg/engine/home_detail.go:160`, and `pkg/engine/home_detail.go:257`.
  Four are runtime construction or override sites:
  `pkg/engine/runtime.go:98`, `pkg/engine/runtime.go:99`,
  `pkg/engine/runtime.go:306`, and `pkg/engine/runtime.go:312`.
- `pkg/engine/entity_refresh_queue.go:298`,
  `pkg/engine/entity_refresh_queue.go:359`,
  `pkg/engine/entity_refresh_queue.go:590`, and
  `pkg/engine/entity_refresh_queue.go:619`: queued entity artifact and entity
  health refresh waves are long-running background operations that call into
  entity rebuild/refresh paths through `*Engine`.
- `pkg/engine/entity_artifact_publish.go:128`: optimistic entity artifact
  mutation stages work before acquiring the publish lease and can call broad
  entity staging code through the engine pointer.
- `pkg/engine/status_snapshot.go:21` and `pkg/engine/status_snapshot.go:104`
  are examples of safe current code because status readers hold `e.mu.RLock`
  while copying runtime/config fields.
- `pkg/engine/runtime_ledger_cache.go:36` and
  `pkg/engine/ip_context.go:334` are examples of safe current code because
  they copy reload-swapped pointers and runtime/config fields under one lock.
- `pkg/engine/engine.go:391`: `configRuntimeSnapshot()` is the existing
  combined config/runtime snapshot pattern to extend or wrap for scheduler and
  operation snapshots.
- `pkg/web/surface_routes.go:30`, `pkg/web/surface_routes.go:31`, and
  `pkg/web/surface_routes.go:44`: public route construction calls
  `Runtime()` multiple times, so route construction itself can mix runtime
  generations. Live route-root rebinding after reload remains a separate
  derivative contract.
- Accessor-generation inventory found 72 `Config()` / `Runtime()` accessor
  calls across `pkg/engine`, `pkg/scheduler`, and `pkg/web`. These calls are
  individually race-safe because they take `e.mu`, but callers that need both
  config and runtime, or several runtime fields for one build, can mix reload
  generations unless they use one combined snapshot.
- Focused `e.state` assignment scan:
  `rg -n '^\s*(e|eng)\.state\s*=' pkg/engine pkg/scheduler pkg/web cmd/update-ipsets --glob '*.go' --glob '!**/*_test.go'`
  currently finds no production assignments. Focused constructor scan
  `rg -n 'state:\s*' pkg/engine/engine.go pkg/scheduler pkg/web cmd/update-ipsets --glob '*.go' --glob '!**/*_test.go'`
  finds only `pkg/engine/engine.go:169`, the engine construction assignment.
- Focused production engine-field assignment scan:
  `rg -n -P '^\s*e\.[A-Za-z_][A-Za-z0-9_]*\s*(?<![=!<>])=(?!=)' pkg/engine/engine.go pkg/engine/runtime.go --glob '*.go' --glob '!**/*_test.go'`
  currently finds runtime override storage at `pkg/engine/runtime.go:291` and
  `pkg/engine/runtime.go:292`, constructor-only `querySetCache` assignment at
  `pkg/engine/engine.go:179`, retention-window rebuild at
  `pkg/engine/engine.go:227`, reload-swapped fields at
  `pkg/engine/engine.go:288`, `pkg/engine/engine.go:289`,
  `pkg/engine/engine.go:293`, `pkg/engine/engine.go:298`,
  `pkg/engine/engine.go:300`, and `pkg/engine/engine.go:304`, lane creation at
  `pkg/engine/engine.go:295`, and config-reload status fields at
  `pkg/engine/engine.go:336`, `pkg/engine/engine.go:337`,
  `pkg/engine/engine.go:356`, and `pkg/engine/engine.go:365`.
- Go's official memory model says data modified while simultaneously accessed
  by multiple goroutines must be serialized, and defines a data race as a
  concurrent unordered read/write or write/write to the same memory location:
  https://go.dev/ref/mem.
- `go test -race ./pkg/engine ./pkg/web -count=1` passed during SOW-0117 V18,
  but current reload-overlap tests do not prove the suspected active processing
  race.
- Existing reload-overlap baseline:
  `pkg/engine/runtime_test.go:395` exercises reload against public/runtime
  readers that route through locked helpers such as `runtimeLedgerSnapshot`;
  `pkg/engine/reload_entry_reconcile_test.go:14` verifies reload does not hold
  the engine mutex during entry reconciliation; and
  `pkg/scheduler/activity_snapshot_test.go:46` exercises scheduler activity
  snapshots during reload. These tests are useful, but they do not cover
  unlocked active processing paths such as `RunOnce`, `FetchAndStage`,
  artifact staging, feed-body history/retention work, provider phases,
  integrity checks, or metadata writers.

Affected contracts and surfaces:

- SIGHUP/config reload behavior.
- Active processing runs.
- Downloader and artifact acquisition helpers.
- Public/admin status snapshots.
- Integrity and metadata writers.
- Scheduler snapshot builders, action admission, due evaluation, and artifact
  item builders.
- Background entity artifact refresh, entity health refresh, optimistic entity
  artifact mutation, and their rebuild/patch staging paths.
- Public route construction and public route-root live-rebinding semantics.
- Operator expectation for whether in-flight work observes old or new runtime
  values.

Existing patterns to reuse:

- `Engine.Runtime()` already returns a runtime copy through the engine mutex.
- Several newer paths already snapshot runtime before use.
- SOW-0117 bounded work-lane rules separate engine work from public/watchdog
  availability.

Risk and blast radius:

- High concurrency blast radius if fixed mechanically without tests.
- Medium performance risk if hot paths repeatedly lock `e.mu` instead of taking
  one local snapshot per operation.
- Low data-loss risk if the fix preserves existing file-layout semantics.

Sensitive data handling plan:

- Evidence must stay at file/line and field-name level. Do not copy production
  paths, private endpoints, customer data, secrets, tokens, or non-private
  customer-identifying IP addresses into durable artifacts.

Resolved decisions:

1. Runtime reload visibility contract: **long-term-best, split by caller
   class**.
   - Engine runs, downloader work, integrity checks, metadata/entity artifact
     generation, and other long-running background operations capture one
     operation snapshot at admission/start. In-flight work keeps that snapshot
     until the operation finishes.
   - Public/admin request builders capture the latest snapshot at request or
     response-build start. They do not hold the engine mutex while doing
     expensive work, and they do not mix config/runtime values from separate
     reload generations.
   - Reload publishes the next snapshot for new work only. It does not mutate
     in-flight operation state.
   - Reload performs pre-publication validation and runtime-directory creation
     before swapping the engine generation. If that pre-publication work fails,
     the previous runtime/config generation stays installed and the reload error
     is recorded.
   - If reload post-publication maintenance fails after the engine generation has
     been swapped, already-admitted operations still use the snapshot they
     captured. New operations use the latest published engine state because that
     is the state now installed on the engine, and the reload error is recorded
     separately.

2. Runtime synchronization design: **long-term-best, operation-local
   snapshots**.
   - Add a single internal engine operation snapshot type that copies these
     fields under `e.mu`: `cfg`, `runtime`, `downloads`, `geoProviders`,
     `ledgerCache`, `retentionMaxWindow`, `asnLookupCache`, and the derived
     `feedHealthPolicy`.
   - Implementation must not add fields to the operation snapshot after
     SOW-0118 closure without reopening this SOW or creating an explicit
     successor SOW that records the added field's reload/lifetime contract.
   - The snapshot captures `*config.Config`, `Runtime`, and derived feed-health
     policy together. The implementation closes the old runtime-override hazard
     by keeping overrides on `e.runtime` only; snapshot consumers must still use
     the captured `Runtime` value or captured policy for effective runtime
     state, not a fresh accessor read.
   - The snapshot captures pointer fields by reference. Old downloader clients,
     geo-provider caches, ledger caches, and ASN lookup caches remain valid for
     in-flight operations until those operations finish. Reload creates or
     retires the next generation but must not close state still reachable from
     an operation snapshot.
   - Old-generation lifetime mechanisms differ by field. Downloader clients,
     geo-provider caches, runtime ledger caches, and retention-window maps are
     pointer-swapped and remain alive through ordinary references held by
     in-flight snapshots. ASN lookup cache state is not pointer-swapped in the
     same way; reload retires databases and existing lease/release logic keeps
     old databases alive until released. Implementation proof must use the
     correct mechanism per field and must acknowledge temporary memory overlap
     while old snapshots are still in flight.
   - Any operation that downloads data must call `snapshot.downloads.Fetch`, or
     an equivalent operation-snapshot method. Calling `e.downloads.Fetch` from
     an in-flight operation is a violation of this SOW.
   - `engineLane` is not an operation snapshot field. The lane is the admission
     boundary; reload changes its concurrency limit through `SetLimit`, while
     already-admitted work continues under the lane's normal ownership rules.
     Current evidence: initial lane creation happens at
     `pkg/engine/engine.go:297` through `pkg/engine/engine.go:300`, and reload
     updates an existing lane with `SetLimit` at `pkg/engine/engine.go:312`
     through `pkg/engine/engine.go:314`.
   - `querySetCache` is not a reload-swapped field in the current code and does
     not need operation-snapshot treatment unless implementation evidence proves
     otherwise.
   - `e.state` is not a reload-swapped field in the current code. Calls such as
     `e.state.SnapshotEntries()` still need caller-chain review for config/runtime
     coherence, but `state` itself does not belong in the operation snapshot
     unless implementation evidence proves otherwise. Current evidence:
     `pkg/engine/engine.go:169` sets `e.state` during construction and
     `pkg/engine/engine.go:291` through `pkg/engine/engine.go:310` reloads
     config/runtime/downloader/provider/ledger/cache state without assigning
     `e.state`.
   - Before closure, implementation must rerun a focused assignment scan for
     `e.state` and equivalent receiver aliases. If any state reassignment path
     beyond engine construction is found, this exclusion must be revisited
     before the SOW can close. The closure record must include the scan command,
     scanned paths, and reassignment hits found.
   - Refactor unsafe active paths to use that snapshot through their call
     chain instead of repeatedly locking hot paths or reading `e.cfg` /
     `e.runtime` directly.
   - Existing safe snapshot helpers remain valid projections of the same rule:
     `StatusSnapshot*`, `runtimeLedgerSnapshot`, and `lookupContextSnapshot`
     already copy their required generation under `e.mu`. The new operation
     snapshot must reuse, wrap, or clearly coexist with those helpers; ad hoc
     duplicate locking patterns are not acceptable.
   - Runtime ledger safety contract: disk artifacts remain the durable source
     of truth. The runtime ledger cache is a derived in-memory view over
     durable `history.csv`, `changesets.csv`, and `retention.csv` data. Any
     operation that updates a derived ledger cache must first append/write the
     corresponding durable file, or must be changed so that this ordering is
     true. A later cache generation must be able to reload the durable data
     rather than depend on the retired cache's memory.
   - Durable-first means both order and failure semantics. If a durable append
     or write fails, the derived runtime ledger cache must not advance unless a
     deliberate exception is recorded with file/line evidence and external
     reviewer acceptance. Existing `history.csv` append failure currently logs
     and continues to `observeHistoryPointContext`; implementation must either
     change that behavior or explicitly record why the divergence is acceptable.
   - Current code already satisfies durable-first ordering for history,
     changesets, and retention-past points, but retention-cohort replacement is
     currently cache-first. The implementation must move cohort cache
     replacement after `writeRetentionOutputs` returns nil, so partial durable
     output failure cannot advance the in-memory cohort cache.
   - `criticalProviderSetID` and `criticalProviderSetCached` are not reload-
     swapped fields and are governed by `criticalProviderSetMu` plus pipeline
     plan capture. They do not belong in the operation snapshot unless
     implementation evidence proves otherwise.

3. Scheduler snapshot contract: **long-term-best, coherent iteration
   snapshots**.
   - Add an exported combined config/runtime snapshot accessor for scheduler
     snapshot builds, artifact item builds, admin summary builders, and public
     route construction. It may expose `ConfigRuntimeSnapshot() (*config.Config,
     Runtime)` or an equivalent small value type, but every caller must use one
     call per generation-sensitive build.
   - Download-loop worker count and processing-loop cadence should be derived
     from the current snapshot at loop iteration boundaries. A currently
     sleeping processing timer may finish its old wait; the new cadence applies
     on the next wake/timer reset.

4. Public serving root reload: **valid derivative, tracked separately**.
   - `pkg/web/surface_routes.go:30` through `pkg/web/surface_routes.go:45`
     captures public roots and cache limits when the web routes are built.
   - SOW-0118 must fix the construction-time mixed runtime generation by using
     one combined config/runtime snapshot at route construction.
   - Live rebinding of `WebDir`, `WebDirForIPSets`, and file-cache limits after
     reload is not the same data race class. It is tracked as
     `.agents/sow/pending/SOW-0119-20260626-public-serving-runtime-rebind.md`.
   - SOW-0118 lifecycle closure follows the project SOW framework: it may close
     once the derivative is represented by the concrete pending SOW above and
     every SOW-0118 acceptance criterion is satisfied. The active thread
     objective remains incomplete until SOW-0119 is implemented, validated,
     reviewed, closed, committed, pushed, and installed immediately after
     SOW-0118.

5. Deadlock-safety contract: **long-term-best, no nested engine mutex capture**.
   - Operation snapshots take `e.mu.RLock`. No code path may capture an
     operation snapshot while already holding `e.mu` for write in the same
     goroutine.
   - Operation snapshot capture must not take `reloadMu`. Reload already uses
     the order `reloadMu` then `e.mu`; snapshot consumers should take only
     `e.mu.RLock` so no two-mutex ordering cycle is introduced.
   - Review and static scans must classify snapshot captures inside locked
     regions before closure.

6. Direct-read inventory contract: **long-term-best, explicit inventory**.
   - Before implementation, generate an initial inventory with:
     `rg -n '\b[a-zA-Z_][a-zA-Z0-9_]*\.(cfg|runtime|downloads|geoProviders|ledgerCache|retentionMaxWindow|asnLookupCache)\b' pkg/engine pkg/scheduler pkg/web --glob '*.go' --glob '!**/*_test.go'`.
   - The current initial scan finds 342 direct reload-swapped-field hits across
     production files before implementation. The broader scan intentionally
     includes aliases and helper receiver names such as `eng.cfg`, `r.cfg`,
     and `b.cfg`; the earlier `e.`-only scan missed real engine pointer reads in
     `pkg/engine/integrity_payloads.go`.
   - Current classification: 285 hits are treated as unsafe until converted or
     proved safe, 32 are currently safe locked-helper reads, and 25 are reload,
     override, or accessor reads that must remain locked or be proved startup-only.
     The highest-count files include `pkg/engine/download_stage.go`,
     `pkg/engine/status_snapshot.go`, `pkg/engine/helpers.go`,
     `pkg/engine/engine.go`, `pkg/engine/integrity.go`,
     `pkg/engine/artifact_stage.go`, `pkg/engine/feed_body_stage.go`,
     `pkg/engine/home_entity_builders.go`,
     `pkg/engine/entity_integrity_refs.go`, and
     `pkg/engine/run_pipeline.go`.
   - Each hit must be classified as one of: to-fix with operation snapshot,
     already locked/safe helper, startup-only, reload-only, lane-admission-only,
     public/request snapshot, or intentionally latest-at-use.
   - The completed inventory may live in this SOW or a companion file named
     `.agents/sow/current/SOW-0118-direct-read-inventory.md`; if a companion is
     used, SOW-0118 closure must move or commit it with the SOW.
   - The 285 `unsafe-convert` rows are caller-chain classified in
     `.agents/sow/current/SOW-0118-caller-chain-classification.md`. That
     companion file groups every unsafe row by reachability class and defines
     the operation/request/build snapshot plan for each group.

7. Accessor-generation inventory contract: **long-term-best, explicit
   mixed-generation inventory**.
   - Before implementation, generate an accessor inventory with:
     `rg -n '\b(eng|e|r\.eng)\.(Runtime|Config)\(\)' pkg/engine pkg/scheduler pkg/web --glob '*.go' --glob '!**/*_test.go'`.
   - The current initial accessor scan finds 72 accessor calls before
     implementation: 22 mixed-generation review sites and 50 single-accessor
     review sites.
   - The completed inventory lives in
     `.agents/sow/current/SOW-0118-accessor-generation-inventory.md`.
   - Each hit must be classified as one of: combined snapshot required,
     request snapshot required, operation snapshot required, intentionally
     latest-at-use, startup-only, or already coherent.
   - The 50 `single-accessor-review` rows are grouped by caller-chain handling
     in the same companion inventory. They are lower-risk than mixed accessors
     but not ignored, because a single local accessor can still sit inside a
     wider caller chain that mixes runtime/config generations.

Implementation plan:

1. Add tests first:
   - primary pre-fix race target: reload overlapping with `RunOnce` after start
     admission using an existing deterministic synchronization point, or a
     focused test-only hook added before the active path performs its first
     runtime/config/dependency read if no suitable hook exists, proving
     in-flight work keeps one generation;
   - secondary pre-fix race target if `RunOnce` cannot produce a deterministic
     race: reload overlapping with `FetchAndStage` for a normal downloader
     source, using an `httptest` server gate and a concurrent reload loop;
   - reload overlapping with artifact staging or artifact child
     materialization, proving `e.downloads` and runtime path coherence;
   - reload overlapping with history derivative snapshot append/prune, proving
     `retentionMaxWindow` and `HistoryDir` coherence;
   - reload overlapping with `CheckIntegrityWithOptionsContext`;
   - reload overlapping with queued entity artifact refresh and entity health
     refresh, or a structural test proving their staging paths receive an
     operation snapshot;
   - scheduler snapshot and artifact-item builders using one coherent
     config/runtime generation;
   - public surface route construction using one coherent runtime generation.
2. Confirm at least one new active-path race test fails under `-race` before the
   fix. If the race detector cannot be made deterministic, record the exact
   structural proof and get acceptance from at least two external reviewers
   before treating the substitute proof as enough. The substitute proof must
   include: a
   test-controlled interleaving that reaches the active path, file/line evidence
   of an unlocked read and reload write to the same field without a common
   happens-before relationship, the same-failure scan showing the pre-fix read
   class, and acceptance from at least two external reviewers before closure.
   If all six external reviewers fail to return usable results for that proof,
   user review may substitute and must be recorded explicitly.
3. Add an internal engine operation snapshot type and small constructor
   helpers. The snapshot must include `cfg`, `runtime`, `downloads`,
   `geoProviders`, `ledgerCache`, `retentionMaxWindow`, and `asnLookupCache`.
   Before converting callers, prove or add lifetime guards showing old
   downloader clients, geo-provider caches, runtime ledger caches,
   retention-window maps, and ASN lookup caches remain valid while an
   already-admitted operation snapshot holds them.
4. Refactor downloader/artifact paths to capture one snapshot per queued item,
   including recursive history derivative and artifact child materialization.
5. Refactor by caller-chain batch, following
   `.agents/sow/current/SOW-0118-caller-chain-classification.md`. Do not fix
   285 line hits one by one; each batch must capture the snapshot at the
   operation/request/build boundary and pass it down.
6. Refactor `RunOnce` and its heavy phases to use one run snapshot for config,
   runtime, downloader-independent path helpers, provider caches, and metadata
   writers.
7. Refactor integrity checks and metadata write runs to capture one snapshot at
   check/write-run construction and use it for all config/runtime path fields.
8. Refactor background entity refresh, entity health refresh, optimistic entity
   artifact mutation staging, and related rebuild/patch helpers so each
   background operation uses one operation snapshot or a smaller explicit
   snapshot derived from the same constructor.
9. Refactor scheduler/admin/public route snapshot builders to use a combined
   config/runtime snapshot so they cannot mix generations.
10. Verify and preserve runtime-ledger cache ordering. For history, changeset,
   and retention updates, the implementation must prove with file/line evidence
   that durable file append/write already happens before derived runtime-ledger
   cache updates, or must change the code so durable-first ordering is enforced.
   Add a reload-overlap regression test or a focused structural test for this
   ordering. Current evidence already identifies retention-cohort replacement
   as cache-first; fix that ordering as part of this step. Also prove or fix
   durable-write failure semantics so failed writes do not advance derived
   runtime ledger caches unless an explicit accepted exception is recorded.
11. Update specs unconditionally for the final snapshot contract. Required
   review set: `.agents/sow/specs/operating-principles.md`,
   `.agents/sow/specs/pipeline.md`,
   `.agents/sow/specs/processing-engine.md`,
   `.agents/sow/specs/integrity.md`,
   `.agents/sow/specs/admin-ui.md`,
   `.agents/sow/specs/website.md`, and
   `.agents/sow/specs/memory-management.md`. Closure must record either an
   update or an unchanged-with-reason note for every required spec in this set.
12. Keep same-failure scans in validation:
   `rg -n '\b[a-zA-Z_][a-zA-Z0-9_]*\.(cfg|runtime|downloads|geoProviders|ledgerCache|retentionMaxWindow|asnLookupCache)\b' pkg/engine pkg/scheduler pkg/web --glob '*.go' --glob '!**/*_test.go'`.
   Also rerun
   `rg -n '\b(eng|e|r\.eng)\.(Runtime|Config)\(\)' pkg/engine pkg/scheduler pkg/web --glob '*.go' --glob '!**/*_test.go'`.
   Also rerun the focused `cfg.Runtime` hazard scan:
   `rg -n '\bcfg\.Runtime\b|\.cfg\.Runtime\b' pkg/engine pkg/scheduler pkg/web --glob '*.go' --glob '!**/*_test.go'`.
   Also run a broad direct-field/accessor smoke scan over
   `cmd/update-ipsets`, `pkg/mcp`, `internal`, and `tools` so closure records
   that package surfaces outside `pkg/engine`, `pkg/scheduler`, and `pkg/web`
   do not bypass the safe engine accessors:
   `rg -n '\b[a-zA-Z_][a-zA-Z0-9_]*\.(cfg|runtime|downloads|geoProviders|ledgerCache|retentionMaxWindow|asnLookupCache)\b|\b[a-zA-Z_][a-zA-Z0-9_]*\.(Runtime|Config)\(\)' cmd/update-ipsets pkg/mcp internal tools --glob '*.go' --glob '!**/*_test.go'`.
   After implementation, every remaining production read/call must be
   classified as locked, startup-only, reload-only, lane-admission-only,
   intentionally latest-at-use, or safe snapshot helper code. Consumer-side
   `cfg.Runtime` policy reads must disappear or be classified with file/line
   evidence as startup/reload/runtime-construction code.
13. Run a nested-lock validation scan or structural test using the final
   operation snapshot constructor names. No snapshot capture may occur inside a
   region that already holds `e.mu` for write.
14. Run a performance sanity check. At minimum, run the package benchmarks or
   explain why the touched paths have no meaningful local benchmark. The design
   must avoid repeated `e.mu` locking in hot loops.

Validation plan:

- Focused `go test -race` tests for reload overlap.
- `go test -race -count=10 ./pkg/engine ./pkg/scheduler ./pkg/web`.
- `go test -count=1 ./pkg/engine ./pkg/scheduler ./pkg/web`.
- Same-failure scan for direct reload-swapped field reads after the fix.
- Same-failure scan for `Config()` / `Runtime()` accessor generation mixing
  after the fix.
- Same-failure scan for `cfg.Runtime` policy reads after the fix:
  `rg -n '\bcfg\.Runtime\b|\.cfg\.Runtime\b' pkg/engine pkg/scheduler pkg/web --glob '*.go' --glob '!**/*_test.go'`.
  Remaining hits must be limited to startup, reload, or runtime-construction
  code with file/line evidence.
- Durable-first runtime-ledger ordering proof/test for history, changesets,
  retention past, and retention cohorts. Retention cohort replacement must be
  durable-first after implementation.
- Durable-write failure proof/test for every touched ledger/cache update path.
  Failed durable writes must not advance derived runtime ledger caches unless an
  explicit accepted exception is recorded.
- Old-generation pointer lifetime proof/test for downloader clients,
  geo-provider caches, runtime ledger caches, retention-window maps, and ASN
  lookup caches.
- Nested-lock scan or structural test for final operation snapshot constructor
  names.
- Focused state-assignment scan for `e.state` and equivalent receiver aliases.
- Focused production assignment scan for engine fields reassigned during reload
  or runtime override, with each hit classified before closure.
- `make race` before final implementation review unless an unrelated package
  failure is recorded with evidence.
- Benchmark or benchmark-gap note for touched hot paths.

Artifact impact plan:

- AGENTS.md: likely no update unless a new runtime-access rule is needed.
- Runtime project skills: likely update `project-coding` if a durable
  runtime-snapshot rule is selected.
- Specs: update operating-principles and the affected processing/reload/admin/
  website specs with the final operation-snapshot and request-snapshot
  semantics.
- End-user/operator docs: likely no update unless reload behavior changes.
- End-user/operator skills: no expected impact.
- SOW lifecycle: this SOW is the concrete follow-up for the SOW-0117 V18
  deferred runtime reload race item.

Open-source reference evidence:

- None. This is a project-specific runtime ownership issue.

Open decisions:

- None. The user directed every choice to use the long-term-best option.

## Plan

1. Complete direct-read inventory and classify safe vs unsafe paths.
2. Review this implementation plan with external reviewers before writing
   code.
3. Write focused race tests before code changes.
4. Implement operation/request/iteration snapshot fixes in small batches.
5. Validate with race detector, same-failure scan, and package tests.
6. Review the implemented fixes with external reviewers.
7. Repeat external gap analysis on the new baseline and fix any remaining
   valid findings before closure.

## Execution Log

### 2026-06-25

- Created as pending follow-up from SOW-0117 V18 plan review.

### 2026-06-26

- Activated for implementation after SOW-0117 was completed and installed.
- User direction recorded: for all choices, always pick the long-term-best
  option.
- External gap-review round completed before implementation planning. Usable
  reviewers agreed on the core issue: reload swaps config/runtime and
  engine-owned pointers while active work still reads them directly. Two
  reviewers failed twice and are excluded for that round under the repository
  reviewer-failure protocol.
- Validated the core findings against current code and recorded the selected
  long-term-best snapshot contract in the pre-implementation gate.
- Plan-review round 1 returned NOT READY overall. Valid findings were folded
  into the SOW: snapshot field set, `retentionMaxWindow`, `cfg.Runtime`,
  downloader-client usage, scheduler artifact snapshots, background entity
  operations, direct-read inventory, stronger race validation, benchmark sanity,
  and the concrete public-serving derivative.
- Created `.agents/sow/current/SOW-0118-direct-read-inventory.md` from the
  same-failure scan. Initial classification after the broadened alias-aware
  scan: 342 direct reload-swapped-field hits, with 285 treated as unsafe until
  converted/proven, 32 currently safe locked-helper reads, and 25 reload/
  accessor reads that must remain locked or be proved startup-only.
- Created `.agents/sow/current/SOW-0118-accessor-generation-inventory.md` from
  the accessor-generation scan. Initial classification: 72 `Config()` /
  `Runtime()` accessor sites, with 22 mixed-generation review sites and 50
  single-accessor review sites.
- Created `.agents/sow/current/SOW-0118-caller-chain-classification.md` to
  group all 285 unsafe direct-read rows by caller-chain reachability and
  snapshot plan before implementation.
- Created `.agents/sow/pending/SOW-0119-20260626-public-serving-runtime-rebind.md`
  as the tracked derivative for public serving roots/cache live rebinding.
- Plan-review round 2 returned READY from `glm`, `deepseek`, `mimo`, `kimi`,
  and `qwen`; `minimax` found valid inventory and plan blockers. `qwen` reviewed
  the earlier 331-hit direct inventory, so the plan will be re-reviewed after
  the broadened 342-hit inventory and SOW text updates.
- Broadened-inventory review returned READY from `glm`, `minimax`, and `qwen`;
  `kimi` returned NOT READY. Valid `kimi` blockers were folded in by adding the
  caller-chain classification companion, `e.state` file/line evidence, and
  explicit active-thread objective wording for SOW-0119. `deepseek` and `mimo`
  sessions ended without recoverable final responses and must be re-run with
  the revised SOW before implementation.
- Revised plan review found additional valid gates before implementation:
  durable-first runtime-ledger ordering had to become an acceptance criterion,
  nested-lock validation needed objective closure criteria, old-generation
  pointer lifetimes needed explicit proof, SOW-0119 needed stale-cache
  acceptance criteria, and the 50 single-accessor rows needed caller-chain
  grouping.
- Verified current runtime-ledger ordering in code: history, changesets, and
  retention-past updates are durable-first; retention-cohort replacement is
  currently cache-first and must be fixed during implementation.
- Updated `.agents/sow/current/SOW-0118-accessor-generation-inventory.md` with
  a caller-chain plan covering all 50 single-accessor rows.
- Revised plan review found two additional load-bearing SOW text issues and
  several clarity concerns. Folded in the valid findings: existing reload tests
  now have an accurate baseline that distinguishes locked reader tests from
  active-processing race gaps; durable-first ledger semantics now include
  failed durable writes; `cfg.Runtime` wording now reflects that the config
  pointer is stable but embedded runtime fields are mutable; pointer lifetime
  proof distinguishes pointer-swapped fields from ASN lease/retire handling;
  and spec updates now have an explicit required review set.
- Plan-review convergence round on the updated SOW returned READY from `glm`,
  `qwen`, `deepseek`, and `mimo`. Valid low-severity notes were folded in or
  reserved as closure proofs: SOW-0119 validation now includes `pkg/mcp`;
  retention-cohort derived-cache ordering must explicitly account for
  `observeRetentionCohort`; closure must record the `e.state` assignment scan,
  nested-lock scan, required spec review set, durable-first proof, history CSV
  append-failure decision, pointer-lifetime proof, and benchmark/gap note.
- `kimi` and `minimax` still returned NOT READY in that convergence round, but
  their valid blockers were process and derivative-plan gaps rather than new
  implementation design choices: stale review-round bookkeeping, missing
  current `deepseek` record, missing `e.state` acceptance wording, and the
  missing SOW-0119 reload-notification mechanism. The `e.state` acceptance
  wording and SOW-0119 reload-notification mechanism are now recorded. The next
  external plan-review round must review SOW-0118, the three companion
  inventories, and SOW-0119 together before implementation starts.
- Next full-scope plan review found a valid `cfg.Runtime` blocker: the direct
  reload-swapped-field scan did not detect `cfg.Runtime` policy reads, and the
  caller-chain classification only named the direct `e.cfg.Runtime` subset.
  Folded in the fix by adding the focused `cfg.Runtime` hazard scan, listing all
  11 consumer-side reads and 4 startup/reload/runtime-construction hits, and
  requiring post-fix classification of every remaining hit. The same review also
  requested reproducible `e.state` and reload-swapped-field assignment scans;
  those scan commands and current results are now recorded in the evidence and
  validation gates.
- Full-scope plan-review round status after the `cfg.Runtime` fix:
  `mimo`, `deepseek`, `qwen`, and `minimax` returned READY FOR
  IMPLEMENTATION. `glm` returned NOT READY with valid blockers that were folded
  in: complete `cfg.Runtime` mutation-hazard coverage, a dedicated
  `cfg.Runtime` scan, reproducible `e.state` and reload-swapped-field
  assignment scans, and bounded reload-publication listener registration in
  SOW-0119. The `kimi` session exited without a recoverable final response and
  must be rerun on the updated SOW family. The next review round must use the
  same full scope plus a short note listing these fixes.
- The next full-scope plan-review round returned READY from the recovered
  reviewer sessions, with minor implementation-time clarifications that were
  folded into the plan before code: the `RunOnce` race test must use an
  existing deterministic synchronization point or add a focused test-only hook,
  closure must include a broad `cmd/update-ipsets` and `pkg/mcp` direct-field/
  accessor smoke scan, and SOW-0119 must define reload-hook panic/failure and
  partial tuple-change behavior before implementation.
- A contaminated plan-review prompt round was discarded because it copied the
  user's orchestration request and could recursively instruct reviewers to run
  reviewers. The sanitized round used only the review task, SOW filenames,
  evaluation questions, and an explicit "do not run other external assistants"
  rule.
- Sanitized full-scope plan-review round v4 returned READY FOR IMPLEMENTATION
  from `glm`, `minimax`, `mimo`, `kimi`, `deepseek`, and `qwen`. Reviewer
  advisory notes were folded in before implementation: the gate status is now
  `plan-approved-for-implementation`, closure scan coverage includes
  `internal` and `tools`, and SOW-0119 pins resolved public-serving tuple and
  listener-dispatch timing semantics.
- Implemented the reload snapshot boundary for admitted engine work,
  scheduler/admin builders, entity artifact refresh/rebuild waves, metadata
  comparison generation, and config-bound cache-entry snapshots.
  Evidence: `pkg/engine/runtime_snapshot.go:12`, `pkg/engine/runtime.go:299`,
  `pkg/engine/query.go:371`, `pkg/scheduler/download_loop.go:17`, and
  `pkg/web/admin.go:535`.
- Removed the old runtime-override config mutation hazard. Runtime directory
  overrides now update only `e.runtime`; they no longer mutate
  `e.cfg.Runtime.WebDir` or `e.cfg.Runtime.WebDirForIPSets`.
  Evidence: `pkg/engine/runtime.go:299`.
- Reconciled the main post-implementation scans. Remaining direct
  reload-swapped-field hits are locked accessors/reload code, startup
  construction code, status snapshots, IP lookup snapshots, or public-serving
  route binding explicitly tracked by SOW-0119. Remaining `cfg.Runtime` reads
  are runtime construction and feed-health policy derivation from a captured
  config pointer. The `e.state` assignment scan found work-lane item state and
  integrity-cache state mutations, not engine state pointer replacement.
- Objective nested-lock scan found only `operationSnapshot()` itself as a
  function containing both `e.mu.RLock()` and `operationSnapshot` field
  capture. No caller currently captures a snapshot while already holding the
  broad engine mutex.
- Broad smoke scan across `cmd`, `internal`, `tools`, and `pkg/mcp` found no
  direct reload-swapped-field reads. It found two `eng.Runtime()` accessor
  reads in `cmd/update-ipsets/daemon.go`: one startup web-option snapshot and
  one post-reload log field. Neither is an admitted processing/admin-builder
  race.
- Initial sanitized implementation review found valid fixes before closure:
  `history.csv` append failure still advanced in-memory history state,
  public route/startup construction still fetched runtime through multiple
  accessors, `download_stage.go` and then `public.go` tripped the architecture
  posture large-file guard, and the benchmark/gap note was missing.
- Fixed the review findings. `history.csv` append failure is now fatal before
  feed cache/history cache advancement. Public route construction and startup
  integrity recovery now use one config/runtime snapshot. Oversized helper code
  moved from `download_stage.go` and `public.go` into focused companion files
  without changing architecture baselines.
- Fixed the final local scheduler admission gaps found by the post-change
  scan. Manual recheck, reprocess, and run actions now use batched engine
  admission helpers that capture one operation snapshot per action class, and
  automatic due evaluation now classifies downloadable feeds from its supplied
  config snapshot instead of calling back into live engine accessors.
  Evidence: `pkg/engine/download_queries.go:195`,
  `pkg/engine/download_queries.go:250`,
  `pkg/engine/download_queries.go:262`,
  `pkg/scheduler/actions.go:27`, `pkg/scheduler/actions.go:43`,
  `pkg/scheduler/actions.go:59`, and
  `pkg/scheduler/automatic_due.go:26`.
- Sanitized implementation re-review found one valid non-blocking
  reconciliation gap: processing-loop cadence was still captured once at loop
  start, despite decision #3 requiring reload-visible cadence on the next
  wake/timer reset. The loop now computes cadence from a current
  config/runtime snapshot at timer reset and wake boundaries, and a scheduler
  test verifies `ReloadContext` updates the interval seen by the runner.
  Evidence: `pkg/scheduler/processing_loop.go:12`,
  `pkg/scheduler/processing_loop.go:20`,
  `pkg/scheduler/processing_loop.go:23`,
  `pkg/scheduler/processing_loop.go:28`, and
  `pkg/scheduler/policy_test.go:135`.
- Full sanitized implementation re-review found no blocking runtime, race,
  durable-ordering, or deadlock findings. Valid non-blocking cleanup findings
  were reconciled before closure: stale processing-loop inventory text,
  contradictory pre/post `cfg.Runtime` mutation wording, missing structural
  race-proof wording, missing explicit `feedHealthPolicy` snapshot-field
  documentation, and a dead production artifact max-size wrapper used only by
  tests. The wrapper was removed and tests now call the runtime-specific helper
  directly.

## Validation

Acceptance criteria evidence:

- Operation snapshot exists and copies config, runtime, downloader client,
  provider cache, ledger cache, retention window map, ASN lookup cache, and
  feed-health policy while holding the broad mutex only for the copy.
  Evidence: `pkg/engine/runtime_snapshot.go:12`.
- Old-generation pointer lifetime is intentionally operation-scoped. Reload
  swaps `e.cfg`, `e.runtime`, `e.downloads`, `e.geoProviders`,
  `e.ledgerCache`, and ASN lookup cache retirement state for future work, while
  already-admitted work keeps its captured pointers.
  Evidence: `pkg/engine/engine.go:287`,
  `pkg/engine/runtime_snapshot.go:29`.
- `cfg.Runtime` embedded mutation hazard is closed by keeping runtime overrides
  on `e.runtime` only. Remaining `cfg.Runtime` reads are stable config-derived
  policy/runtime-construction reads, not mutable override reads.
  Evidence: `pkg/engine/runtime.go:98`,
  `pkg/engine/runtime.go:299`,
  `pkg/engine/runtime_snapshot.go:54`.
- Scheduler/admin full builders use one config/runtime/policy generation per
  iteration or response and pass that generation into entry, artifact, and feed
  row builders instead of mixing fresh accessors in inner loops.
  Evidence: `pkg/scheduler/download_loop.go:17`,
  `pkg/scheduler/scheduler.go:202`,
  `pkg/web/admin.go:535`,
  `pkg/web/admin.go:624`,
  `pkg/web/admin.go:688`.
- Scheduler action admission now uses one engine operation snapshot per
  batched recheck, reprocess, or run admission class. Automatic due admission
  uses the scheduler's existing config snapshot directly for source
  classification.
  Evidence: `pkg/engine/download_queries.go:195`,
  `pkg/engine/download_queries.go:250`,
  `pkg/engine/download_queries.go:262`,
  `pkg/scheduler/actions.go:27`, `pkg/scheduler/actions.go:43`,
  `pkg/scheduler/actions.go:59`, and
  `pkg/scheduler/automatic_due.go:26`.
- Processing-loop cadence is now reload-visible at loop boundaries. The
  scheduler reads one current config/runtime snapshot when creating and
  resetting the processing timer; a focused test rewrites the runtime config,
  reloads the engine, and verifies the runner observes the new interval.
  Evidence: `pkg/scheduler/processing_loop.go:28`,
  `pkg/scheduler/policy_test.go:135`.
- Public route construction now uses one config/runtime snapshot for startup
  route roots and cache limits. Live route-root rebinding after reload remains
  outside this SOW.
  Evidence: `pkg/web/surface_routes.go:30`,
  `pkg/web/server_run.go:92`.
- Runtime-ledger failure semantics are now durable-first for history as well
  as retention. A failed `history.csv` append returns an error before the
  in-memory source-set and history-tail state advance.
  Evidence: `pkg/engine/finalize.go:91`,
  `pkg/engine/reload_snapshot_test.go:112`.
- The public-serving route-root live-rebind problem remains intentionally
  outside this SOW and is tracked as `.agents/sow/pending/SOW-0119-20260626-public-serving-runtime-rebind.md`.
  Evidence: `.agents/sow/pending/SOW-0119-20260626-public-serving-runtime-rebind.md`.
- Broad closure smoke scan:
  `rg -n '\.(cfg|runtime|downloads|geoProviders|ledgerCache|retentionMaxWindow|asnLookupCache)\b' cmd internal tools pkg/mcp --glob '*.go' --glob '!**/*_test.go'`
  returned no matches. The matching accessor smoke scan returned startup,
  request/iteration snapshot, and post-reload logging calls only; notably
  route construction now uses `ConfigRuntimeSnapshot()`.
- Deterministic pre-fix `-race` reproduction was not reliable enough to be the
  primary proof because the overlap hook that makes the test deterministic runs
  reload synchronously at the admitted-run boundary. The substitute proof is the
  generation-contract test (`TestRunOnceUsesStartGenerationWhenReloadOverlapsAfterAdmission`),
  structural direct-read/accessor reconciliation, repeated race validation, and
  external reviewer acceptance of that proof.

Tests or equivalent validation:

- Passed focused scheduler cadence validation after external re-review:
  `go test ./pkg/scheduler -run 'TestProcessingIntervalUsesReloadedRuntimeSnapshot|TestManualProviderReprocessQueuesTargetsWithPromotion|TestProviderDefaultsReprocessQueuesFullFeedTargets|TestManualRecheckArtifactChildWithoutLocalInputQueuesParentArtifact|TestManualRecheckArtifactChildWithLocalInputQueuesChild|TestManualReprocessWithoutLocalStateDoesNotQueue' -count=1`.
- Passed focused package validation after final fixes:
  `go test ./pkg/engine ./pkg/scheduler ./pkg/web -count=1`
  (`pkg/engine` 38.608s, `pkg/scheduler` 1.614s, `pkg/web` 25.513s).
- Passed command package validation:
  `go test ./cmd/update-ipsets -count=1`.
- Passed full test suite:
  `make test`, including `tools/archposture`.
- Passed required race validation after final fixes:
  `go test -race -count=10 ./pkg/engine ./pkg/scheduler ./pkg/web`
  (`pkg/engine` 409.477s, `pkg/scheduler` 16.802s, `pkg/web` 251.195s).
- Passed benchmark sanity check:
  `go test ./pkg/engine -run=^$ -bench='BenchmarkEffectiveEntryResolverBatchView|BenchmarkLoadChangesetTailLargeLedger|BenchmarkComparisonPairLedgerBinaryCodec' -benchmem -benchtime=2s`.
  Results: `BenchmarkEffectiveEntryResolverBatchView` 585031 ns/op,
  742623 B/op, 3005 allocs/op; `BenchmarkComparisonPairLedgerBinaryCodec`
  marshal 4953710 ns/op, 6530898 B/op, 25 allocs/op; parse 3122063 ns/op,
  8955404 B/op, 402 allocs/op; `BenchmarkLoadChangesetTailLargeLedger`
  375804 ns/op, 283936 B/op, 14 allocs/op. These are sanity numbers only; the
  touched reload/snapshot paths do not have a pre-existing stable benchmark
  baseline, so regression confidence comes from structural scans, normal tests,
  race tests, full-suite validation, and these hot-path sanity timings.
- Sanitized implementation re-review found no blocking runtime, deadlock,
  durable-ordering, or race findings. Valid low-severity findings were fixed
  before closure: duplicated feed-health policy derivation moved to
  `pkg/feedhealth.PolicyFromConfig`, `website.md` now records the
  construction-time public-route generation contract, and the unused metadata
  fallback wrapper that called live `e.Config()` was deleted so fallback uses
  only the operation snapshot's config.
- Final pre-closure review polish after that round also found and fixed four
  valid non-blocking generation-coherence issues: retention-removal in-memory
  observation now uses the reconciliation operation snapshot
  (`pkg/engine/retention_update.go:578`), admin feed rows no longer call fresh
  merge or redistributability accessors after capturing `cfg/rt/policy`
  (`pkg/web/admin.go:718`, `pkg/web/admin.go:982`), raw-feed compatibility
  routes use one engine decision helper instead of three separate snapshot
  checks (`pkg/engine/public.go:65`, `pkg/web/server.go:290`), and the unused
  `operationSnapshot.isZero` helper was removed.
- Passed post-polish local validation:
  `go test ./pkg/feedhealth ./pkg/engine ./pkg/scheduler ./pkg/web -count=1`
  (`pkg/feedhealth` 0.009s, `pkg/engine` 12.410s, `pkg/scheduler` 0.734s,
  `pkg/web` 8.560s);
  `go test -race -count=3 -run 'TestRunOnceUsesStartGenerationWhenReloadOverlapsAfterAdmission|TestRetentionCohortsCacheDoesNotAdvanceWhenDurableWriteFails|TestFinalizeDoesNotAdvanceHistoryCacheWhenHistoryAppendFails|TestProcessingIntervalUsesReloadedRuntimeSnapshot' ./pkg/engine ./pkg/scheduler`
  (`pkg/engine` 1.645s, `pkg/scheduler` 1.060s);
  `go test ./tools/archposture`; `git diff --check`; and targeted scans for
  stale metadata fallback helpers, duplicate feed-health policy helpers,
  production admin fresh merge/redistributability fallbacks, dead
  `operationSnapshot.isZero`, and production `cfg.Runtime` uses outside runtime
  construction.
- A later full-scope reviewer found one valid reload failure boundary issue and
  several low-severity cleanup items. Fixed before closure:
  `ReloadContext` now resolves runtime overrides and creates runtime directories
  before installing the new engine generation (`pkg/engine/engine.go:282` through
  `pkg/engine/engine.go:287`), so directory-creation failure leaves the previous
  config/runtime generation installed while recording the reload error. Added
  `TestReloadContextDoesNotInstallRuntimeWhenDirectoryCreationFails` in
  `pkg/engine/runtime_reload_failure_test.go`.
- The same review found SOW inventory line-number drift and low-severity cleanup
  noise. Fixed before closure: `SOW-0118-accessor-generation-inventory.md` now
  points policy derivation to `pkg/engine/runtime_snapshot.go:37` and
  `pkg/scheduler/snapshot_build.go:40,100`; the unused
  `publicRawFeedAllowedWithSnapshot` helper was removed; `isArtifact` now uses a
  light `Config()` read instead of a full operation snapshot; and run-batch
  labeling now captures one snapshot before classifying history/merge items.
- Passed post-reload-boundary-fix validation:
  `go test ./pkg/engine -run 'TestReloadContextDoesNotInstallRuntimeWhenDirectoryCreationFails|TestReloadContextDoesNotHoldEngineMutexDuringDirectoryCreation|TestReloadContextDoesNotHoldEngineMutexDuringEntryReconcile|TestRunOnceUsesStartGenerationWhenReloadOverlapsAfterAdmission' -count=1`;
  `go test ./pkg/engine -run 'TestReloadContextDoesNotInstallRuntimeWhenDirectoryCreationFails|TestReloadContextDoesNotHoldEngineMutexDuringDirectoryCreation|TestReloadContextDoesNotHoldEngineMutexDuringEntryReconcile|TestRunOnceUsesStartGenerationWhenReloadOverlapsAfterAdmission' -race -count=3`;
  `go test ./pkg/feedhealth ./pkg/engine ./pkg/scheduler ./pkg/web -count=1`
  (`pkg/feedhealth` 0.005s, `pkg/engine` 23.809s, `pkg/scheduler` 0.788s,
  `pkg/web` 11.917s);
  `go test -race -count=3 -run 'TestRunOnceUsesStartGenerationWhenReloadOverlapsAfterAdmission|TestRetentionCohortsCacheDoesNotAdvanceWhenDurableWriteFails|TestFinalizeDoesNotAdvanceHistoryCacheWhenHistoryAppendFails|TestProcessingIntervalUsesReloadedRuntimeSnapshot|TestReloadContextDoesNotInstallRuntimeWhenDirectoryCreationFails' ./pkg/engine ./pkg/scheduler`
  (`pkg/engine` 2.390s, `pkg/scheduler` 1.114s);
  `go test ./tools/archposture -count=1`; and `git diff --check`.
- Final sanitized full-scope reviewer round completed after the reload-boundary,
  added-cohort durable-ordering, and cleanup fixes. `glm`, `minimax`, `mimo`,
  `kimi`, and `deepseek` returned `PRODUCTION GRADE`. `qwen` was run three
  times; the final harness sessions did not return a usable final verdict, but
  the saved partial logs show read-only verification of the core claims and
  passing package tests. The closure decision does not count `qwen` as a final
  approval.
- Reviewer validation evidence:
  `glm` ran `go test ./...`, `go test -race -count=1 ./pkg/engine`,
  `go vet ./pkg/engine ./pkg/scheduler ./pkg/web`, and confirmed the reload
  path does not capture snapshots under the write lock. `mimo` ran the focused
  race tests, `go test ./pkg/engine ./pkg/scheduler ./pkg/web -count=1`, and
  verified the `cfg.Runtime` mutation and added-cohort durable-ordering fixes.
  `kimi` ran the focused race tests, broad package tests, and `git diff --check`.
  `deepseek` ran focused tests and `go test -race -count=5 ./pkg/engine
  ./pkg/scheduler`. `minimax` ran build, focused race, and broad package tests.
- Final reviewer findings were non-blocking: a low code-smell around
  `feedScopedPublicArtifactName` relying on its caller for final public-feed
  authorization, a test-hook mutex naming note, and the already-tracked
  SOW-0119 public route-root live-rebind gap. The current caller still enforces
  public-feed authorization before serving direct published artifacts.
- Final local validation after moving SOW-0118 to `.agents/sow/done/` passed:
  `git diff --check`;
  `go test -race -count=3 -run 'TestRunOnceUsesStartGenerationWhenReloadOverlapsAfterAdmission|TestRetentionCohortsCacheDoesNotAdvanceWhenDurableWriteFails|TestRetentionCohortsCacheDoesNotAdvanceWhenAddedCohortOutputFails|TestFinalizeDoesNotAdvanceHistoryCacheWhenHistoryAppendFails|TestProcessingIntervalUsesReloadedRuntimeSnapshot|TestReloadContextDoesNotInstallRuntimeWhenDirectoryCreationFails' ./pkg/engine ./pkg/scheduler`
  (`pkg/engine` 2.293s, `pkg/scheduler` 1.085s); and
  `go test ./pkg/feedhealth ./pkg/engine ./pkg/scheduler ./pkg/web ./tools/archposture -count=1`
  (`pkg/feedhealth` 0.005s, `pkg/engine` 35.831s,
  `pkg/scheduler` 1.538s, `pkg/web` 24.341s,
  `tools/archposture` 2.506s).

Sensitive data gate:

- No raw secrets, credentials, bearer tokens, SNMP communities, community
  member names, customer names, personal data, non-private customer-identifying
  IPs, private endpoints, or proprietary incident details are included.

Artifact maintenance gate:

- SOW lifecycle: SOW-0118 completed and moved to `.agents/sow/done/` with
  the implementation commit; SOW-0119 remains pending for public-serving
  runtime rebind.
- Specs: processing, pipeline, admin, operating-principles, and website specs
  updated with the operation/request/public-route construction snapshot
  contract. `integrity.md` unchanged because SOW-0118 changes integrity
  execution ownership and snapshot coherence, not integrity finding semantics
  or artifact dependency rules. Live public-serving route-root rebinding remains
  tracked by SOW-0119. `memory-management.md` unchanged because the accepted
  old-generation lifetime contract relies on Go reachability and existing
  cache/database handle ownership, not a new memory-budget, mmap, streaming, or
  out-of-core contract.
- Runtime project skills: no skill updates required yet; no reusable new
  workflow rule has been proven beyond this SOW's implementation contract.
- End-user/operator docs: no user-facing docs updated; this is internal
  runtime correctness work.
- AGENTS.md: no project-wide rule change required.

Follow-up mapping:

- This SOW is the follow-up mapping for the runtime reload race finding
  deferred from SOW-0117 V18.
- Remaining valid deferred work is not loose backlog: public-serving route-root
  live rebind is tracked by
  `.agents/sow/pending/SOW-0119-20260626-public-serving-runtime-rebind.md`.
  SOW-0119 is the immediate next focused work after SOW-0118 closes; no unrelated
  SOW should start between SOW-0118 closure and SOW-0119 activation unless the
  user explicitly supersedes this priority.
