# SOW-0103 - CPU And Memory Optimization Without Functional Change

## Status

Status: in-progress

Sub-state: DroneBL, publish-stage, Git-object, retention, binary writer, comparison, history no-op, provider-overlap, ASN lookup-cache, admin snapshot, and memory-guardrail fixes implemented; local validation, external review, commit, push, and local install smoke complete; long-term live observation and SOW-0104 retention-compaction decision pending

## Requirements

### Purpose

Improve the daemon's CPU use, memory footprint, and disk/I/O pressure without
changing end-user functionality, published artifacts, public/admin API
semantics, feed content, retention semantics, or operator-visible behavior
except for improved performance, bounded resource use, and better diagnostics.

### User Request

The user requested a dedicated SOW to track, prioritize, verify, and solve all
possible CPU and memory causes behind the live install's OOM, high CPU, and
disk growth symptoms. The user explicitly prioritized:

1. Work not needed: work performed only as an implementation side effect or
   quick workaround, where skipping it would not change user-facing behavior.
2. Work needed but inefficient: work that must happen, but whose current
   implementation wastes CPU, memory, disk, or file-cache resources.

The user also required that existing behavior be fully tested before
optimization, and only then optimized, so user-facing functionality cannot
silently regress.

### Assistant Understanding

Facts:

- SOW-0097 reduced ingest concurrency and added stale publish-stage cleanup, but
  did not prove OOM fixed.
- The live test install has had repeated OOM kills under a 2 GiB systemd
  `MemoryMax`.
- The live cgroup memory at inspection time was mostly file cache and slab, not
  Go anonymous memory, so large file I/O can contribute to OOM even when the Go
  heap is below 2 GiB.
- DroneBL is a verified high-impact suspect:
  - `configs/firehol/artifacts/dronebl.yaml` downloads the rsync module root.
  - `tools/dronebl2ipsets/fetch.go` runs `rsync -HaSPvz`; `-P` means
    `--partial --progress`.
  - `pkg/engine/artifact_stage.go` consumes only `fetch/buildzone`.
  - The live test install retained multiple DroneBL rsync outputs in the fetch
    directory, including `buildzone`, `buildzone.combined`, `buildzone.new`,
    and `buildzone6`.
  - Large DroneBL-derived retention directories were observed, especially the
    anonymizer and IRC drone outputs.
- External reviewers agreed that current work mitigates resource pressure but
  does not prove the OOM fixed.
- External reviewers disagreed on whether ordinary scheduler work is broadly
  wasted; the stronger shared view is that no-change downloads are often
  filtered before processing, but admitted work and restart/recovery paths can
  still trigger heavy processing or broad rewrites.

Inferences:

- The highest-value work should start with verified or easily verifiable
  no-op work: DroneBL extra rsync outputs, stale extract/fetch cleanup,
  provider-default restart loops, no-op heavy phases, and byte-identical
  artifact rewrites.
- The next class should optimize necessary work: streaming processing,
  file-backed retention diffing, mmap or cached ASN/Bogon providers, bounded
  entity sidecar builds, and cgroup-aware memory limits.
- Disk growth and OOM are coupled because cgroup `MemoryMax` accounts file cache
  and slab in addition to Go heap.

Remaining unknowns:

- The local test suite and external reviewers prove functional equivalence, and
  the local managed install smoke passed. Longer installed-service CPU, memory,
  file-cache, slab, OOM, and disk-growth impact still needs observation over
  real update cycles.
- Retention storage compaction remains intentionally unresolved in this SOW
  because it requires a functional/product decision captured in SOW-0104.
- Exact phase-level peak memory instrumentation is not implemented here because
  the SOW prioritized verified waste removal and bounded algorithm changes over
  adding a new observability surface.

### Acceptance Criteria

- A maintained resource-theory ledger exists in this SOW with every candidate
  classified as:
  - `work-not-needed`
  - `needed-but-inefficient`
  - `rejected`
  - `needs-more-evidence`
- Every theory records evidence, affected code paths, expected CPU/memory/disk
  benefit, risk, verification method, and implementation status.
- Before the first optimization is implemented, the existing behavior is
  captured with tests and fixtures covering the affected outputs.
- Each optimization preserves user-facing functionality, proven by before/after
  output equivalence on deterministic fixtures and by existing test suites.
- Each optimization includes a targeted regression test that would fail on the
  old inefficient or wasted-work behavior when practical.
- The first implementation pass prioritizes highest expected CPU/memory impact,
  not easiest code change.
- DroneBL resource behavior is fixed or explicitly rejected with evidence before
  this SOW can close.
- The live install can be validated with observable metrics or commands that do
  not require guessing from systemd OOM events alone.
- No raw secrets, private endpoints, customer data, personal data, or
  proprietary incident details are written to durable artifacts.

## Analysis

Sources checked:

- `.agents/sow/current/SOW-0097-20260601-ingest-cpu-concurrency-limits.md`
- `configs/firehol/artifacts/dronebl.yaml`
- `tools/dronebl2ipsets/fetch.go`
- `tools/dronebl2ipsets/parse.go`
- `tools/dronebl2ipsets/outputs.go`
- `tools/dronebl2ipsets/ranges.go`
- `pkg/engine/artifact_stage.go`
- `pkg/scheduler/processing_loop.go`
- `pkg/engine/run_pipeline.go`
- `pkg/engine/process.go`
- `pkg/engine/finalize.go`
- `pkg/engine/retention_update.go`
- `pkg/engine/entity_artifacts_write.go`
- Live test-install read-only observations, with private endpoint details
  omitted from this artifact.
- External reviewer outputs from OOM-only and wasted-work-only review sets.

Current state:

- SOW-0097 current implementation is committed at `4bb393d`.
- `max_ingest_workers` reduces worker fan-out but does not itself remove
  large per-feed in-memory data structures, file-cache pressure, or no-op
  artifact rewrites.
- DroneBL uses a persistent rsync fetch directory and syncs a broader remote
  directory than the application consumes.
- Several heavy phases write or recompute artifacts even when content may be
  byte-identical, partly to preserve integrity mtimes.
- Some work is probably necessary but implemented with full materialization
  instead of streaming/file-backed operations.
- Read-only live test-install admin metrics after the 2026-06-14 run showed
  `metadata.write_comparison_files` at 835,455 ms, with
  `metadata.comparison_pair_overlap` accounting for 830,193 ms across 38,223
  overlap counts. The same run showed `sources.finalize.observe_history` at
  104,280 ms across 402 feeds.

Risks:

- Incorrectly skipping work can silently publish stale feeds, stale metadata,
  stale comparisons, stale entity details, or stale retention artifacts.
- Changing timestamp behavior can break integrity checks that intentionally
  compare generated artifact mtimes with feed processing timestamps.
- Compacting or deleting retention artifacts can change historical API behavior
  unless retention semantics are explicitly preserved.
- Optimizing DroneBL fetch or parse incorrectly can change the derived child
  feeds.
- Raising memory limits would mask the bug instead of proving resource behavior
  is bounded.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The daemon is doing too much resource-intensive work relative to the value
  delivered per run. The root causes appear to fall into two classes:
  unnecessary side-effect work and necessary work with inefficient
  materialization/I/O behavior.
- OOM is probably not a single Go heap leak. It is a combined effect of heap,
  mmap/file-backed reads, cgroup file cache, slab, temp files, staged artifacts,
  retention growth, and repeated processing.
- CPU saturation is probably a combined effect of admitted processing,
  pairwise/heavy phases, entity rebuilds, metadata rewrites, and provider
  parsing.

Evidence reviewed:

- `pkg/scheduler/processing_loop.go` always enters queued processing with
  `Reprocess=true`, which disables the `skipHeavy` gate in
  `pkg/engine/run_pipeline.go`.
- Reviewers found normal unchanged downloads are often filtered before
  processing, so `Reprocess=true` is not by itself proof that all scheduled work
  is wasted.
- `pkg/engine/process.go` materializes parsed sets and loads previous sets for
  processing/retention.
- `pkg/engine/artifact_stage.go` fetches DroneBL with rsync, then streams the
  local `buildzone` through the generic downloader and materializes child
  feeds.
- `tools/dronebl2ipsets/*` parses the full buildzone and builds all selected
  output sets in memory.
- Live test-install evidence showed DroneBL's persistent fetch directory kept
  extra buildzone files, and DroneBL-derived retention directories were among
  the largest runtime trees.

Affected contracts and surfaces:

- Scheduler queue admission and processing semantics.
- Download/artifact acquisition, especially DroneBL.
- Feed processing and retention.
- Heavy phase outputs: comparison, GeoIP, ASN, bogons, critical infrastructure,
  entity sidecars, metadata, home aggregates, sitemap/auxiliary outputs.
- Integrity checks and generated artifact mtime contracts.
- Admin status and diagnostics.
- Install/service memory settings.
- Specs under `.agents/sow/specs/`, especially operating principles,
  pipeline, memory management, downloader, processing engine, integrity, and
  files layout.

Existing patterns to reuse:

- Existing downloader same-body detection and `StatusSame` behavior.
- Existing generated-file atomic write and mtime helpers.
- Existing entity sidecar `DeepEqual` write-skip pattern.
- Existing `iprange` `RangeSource` / file-set iteration APIs.
- Existing stale-stage cleanup introduced by SOW-0097.
- Existing admin metrics/status surface for exposing bounded background work.

Risk and blast radius:

- High blast radius if processing or heavy-phase skip semantics are changed
  without complete output-equivalence tests.
- Medium blast radius for DroneBL fetch hardening, because it affects only one
  artifact family but that family fans out into many popular feeds.
- Medium blast radius for retention optimizations, because public historical
  behavior may depend on retained snapshots.
- Low-to-medium blast radius for no-op write suppression if mtimes are preserved
  deliberately.

Sensitive data handling plan:

- Do not write private hostnames, private endpoints, secrets, rsync passwords,
  bearer tokens, customer names, community member names, personal data, or
  non-private customer-identifying IPs into this SOW, specs, docs, skills,
  prompts, or code comments.
- Live evidence will be summarized as sanitized test-install observations.
- Paths such as `/opt/update-ipsets/...` may be recorded when they do not reveal
  private endpoints or credentials.

Implementation plan:

1. Baseline and proof phase:
   - Build deterministic fixtures for outputs touched by each candidate.
   - Run existing tests and focused fixture generation before any optimization.
   - Add instrumentation or tests that prove whether each theory is true,
     false, or materially small.
2. Work-not-needed phase:
   - Fix verified no-op work from highest expected impact downward.
   - Preserve generated artifact mtimes and public/admin behavior.
   - Add tests proving skipped work is safe.
3. Needed-but-inefficient phase:
   - Optimize necessary work without changing outputs.
   - Prefer streaming, file-backed iteration, bounded caches, and explicit
     cgroup-aware memory behavior.
4. Live validation phase:
   - Install and observe CPU, memory, cgroup breakdown, disk growth, and phase
     timings on the test install with sanitized evidence.

Validation plan:

- Baseline before optimization:
  - `make test`
  - focused Go tests for touched packages
  - deterministic fixture output checksums before/after
  - targeted benchmarks or stress tests for large feeds/artifacts
- After each optimization:
  - output equivalence for affected public files
  - integrity check pass
  - no new broad startup work
  - no public request-time recomputation
  - code review for mtime contract preservation
- Before completion:
  - external reviewers with one set focused on functional equivalence and one
    set focused on performance/resource claims
  - live test-install observation with sanitized results

Artifact impact plan:

- AGENTS.md: likely no update unless a new durable workflow rule is learned.
- Runtime project skills: likely update `project-reviewing`, `project-testing`,
  or `project-operations` if this work discovers repeatable review/test rules.
- Specs: expected updates to memory management, downloader, processing engine,
  pipeline, integrity, and operating principles specs.
- End-user/operator docs: expected only if service/runtime settings or
  diagnostics change.
- End-user/operator skills: no expected impact unless operator docs introduce a
  reusable external skill.
- SOW lifecycle: this SOW is current and in progress; SOW-0097 remains the
  prior concurrency/live-install validation SOW.

Open-source reference evidence:

- None checked yet. This SOW is currently based on local code, prior reviewer
  findings, and live test-install observations. No external reference was
  needed for the implemented streaming range-set processing, rsync staging, or
  cgroup memory behavior.

Open decisions:

1. Resolved: this umbrella SOW owns related CPU/memory optimization work as one
   sequenced program with milestones.
2. Resolved: live test-install validation may include read-only cgroup,
   journal, and disk inspections as routine validation evidence.
3. Resolved: operational memory settings such as `GOMEMLIMIT` are in scope for
   this SOW as an operational guardrail, while still not replacing root-cause
   CPU/memory/I/O fixes.
4. Resolved: implementation organization and non-functional implementation
   details do not require user decisions while this SOW preserves existing
   specs and user-facing behavior. If an optimization requires a functional or
   product decision, skip that optimization, record the evidence and reason in
   this SOW, and continue with the next highest-impact theory.

## Implications And Decisions

1. Priority model.
   - Decision: selected by user.
   - Selection: first remove work that is not needed, then optimize needed work.
   - Implication: no-op detection and side-effect cleanup outrank deeper
     streaming rewrites unless evidence shows a streaming rewrite has much
     higher immediate impact.
   - Risk: a skipped-work bug can silently publish stale outputs, so every skip
     needs output-equivalence and integrity tests.

2. Functionality constraint.
   - Decision: selected by user.
   - Selection: do not change end-user functionality; only improve memory
     footprint and performance.
   - Implication: published feed contents, metadata semantics, public/admin API
     behavior, and retention semantics must remain equivalent unless the user
     explicitly approves a functional change.
   - Risk: disk cleanup and retention compaction are especially risky because
     they can look operational but may affect historical behavior.

3. Baseline-first workflow.
   - Decision: selected by user.
   - Selection: fully test the existing solution before improving it.
   - Implication: implementation must start with tests/fixtures/instrumentation
     proving current behavior and resource profile.
   - Risk: this makes the first visible code changes slower, but it is required
     to avoid breaking user-facing behavior while optimizing.

4. SOW structure.
   - Decision: selected by user on 2026-06-14.
   - Selection: one umbrella SOW with milestones.
   - Implication: the SOW can keep one prioritized theory ledger and sequence
     fixes by measured CPU/memory/disk impact, while still splitting work into
     reviewable milestones.
   - Risk: the SOW must actively prevent scope creep by requiring each
     milestone to prove its theory, acceptance criteria, and validation before
     implementation.

5. Live validation evidence.
   - Decision: selected by user on 2026-06-14.
   - Selection: read-only cgroup, journal, and disk inspections on the live
     test install are allowed as routine validation evidence.
   - Implication: resource claims can be checked against real cgroup memory,
     file-cache, slab, OOM, disk-growth, and phase-timing evidence instead of
     relying only on local tests.
   - Risk: live evidence must remain read-only and sanitized in durable
     artifacts; no service restarts, cleanup, or operational changes are allowed
     without separate explicit approval.

6. Go memory limit scope.
   - Decision: selected by user on 2026-06-14.
   - Selection: include `GOMEMLIMIT` in this SOW as an operational memory
     guardrail.
   - Implication: the SOW may evaluate and, if validated, set a Go soft memory
     limit below systemd `MemoryMax` so Go starts garbage collection earlier and
     returns memory more aggressively.
   - Risk: `GOMEMLIMIT` does not control Linux file cache, kernel slab, or all
     mmap/external memory counted by cgroup `MemoryMax`; it must not be treated
     as the root fix for DroneBL rsync, file-cache pressure, retention growth,
     or unnecessary work.

7. Autonomous non-functional execution.
   - Decision: selected by user on 2026-06-14.
   - Selection: do not stop for implementation-organization decisions or other
     non-functional implementation details, as long as existing specs and
     user-facing behavior are retained.
   - Implication: the assistant may choose the lowest-risk implementation
     details, tests, and code organization needed to preserve the contracts and
     continue making progress.
   - Risk: any optimization that would change published content, public/admin
     semantics, retention behavior, or operator-visible functionality remains a
     functional decision. Such steps must be skipped, documented with evidence,
     and left out of implementation unless the user explicitly approves them.

## Theory Ledger

### Priority 1 - Work Not Needed

1. DroneBL rsync fetch keeps files the app does not consume.
   - Classification: work-not-needed.
   - Status: implemented and locally validated.
   - Evidence: `rsync_url` points to the module root; code consumes only
     `fetch/buildzone`; live test-install fetch directory contained multiple
     buildzone siblings.
   - Expected benefit: lower disk growth, lower file-cache pressure, less rsync
     I/O, lower OOM risk under cgroup `MemoryMax`.
   - Risk: must not change derived DroneBL child feed content.
   - Verification: `tools/dronebl2ipsets/fetch_test.go` proves rsync targets
     only `buildzone`, promotes only that file, and removes stale sibling files;
     `go test ./pkg/engine`, `make test-tools`, and `make test` pass.

2. DroneBL rsync uses `-P`, keeping partial files in a persistent directory.
   - Classification: work-not-needed.
   - Status: implemented and locally validated.
   - Evidence: `tools/dronebl2ipsets/fetch.go` uses `-HaSPvz`; `-P` is
     `--partial --progress`.
   - Expected benefit: lower stale temp/partial accumulation and journal noise.
   - Risk: interrupted downloads restart from scratch unless a managed
     temporary partial directory is used.
   - Verification: `tools/dronebl2ipsets/fetch_test.go` proves the rsync
     command uses `-HaSz`, does not use `-HaSPvz`, cleans per-run temporary
     fetch directories, and keeps the previous `buildzone` on rsync failure.

3. DroneBL extract directories can persist after failed materialization.
   - Classification: work-not-needed.
   - Status: implemented and locally validated.
   - Evidence: stale `outputs-*` directories and a `.tmp` child output existed
     under the DroneBL extract directory.
   - Expected benefit: lower disk growth and cleanup repeated startup work.
   - Risk: cleanup must not remove the active extract dir.
   - Verification: `pkg/engine/dronebl_test.go` proves stale `outputs-*`
     materialization directories are removed while unrelated files/directories
     are preserved.

4. Public/entity publish stage directories can persist after OOM or process
   death.
   - Classification: work-not-needed.
   - Status: implemented and locally validated.
   - Evidence: read-only test-install disk inspection found stale
     `.update-ipsets-web-*` and `.update-ipsets-entities-*` stage directories
     under the generated publication roots. Existing startup cleanup removes
     old stage directories immediately, but an OOM/restart can leave a
     pre-start stage that is still newer than the cleanup grace period at
     startup; if no subsequent restart happens, that stage can remain.
   - Expected benefit: lower disk growth after failed large publish/entity
     rebuilds, especially under `MemoryMax`-triggered restarts.
   - Risk: cleanup must not delete stage directories created by the current
     process or by active publishers.
   - Verification: `TestEngineCleanupPublishStagesBeforeKeepsCurrentProcessStages`
     proves cutoff-based cleanup removes only pre-cutoff stage directories and
     keeps current-process stages; `TestDelayedPublishStageCleanupStopsOnContextCancel`
     proves daemon shutdown cancels the delayed cleanup goroutine.

5. Generated Git repositories can accumulate private loose objects after failed
   sync attempts.
   - Classification: work-not-needed.
   - Status: implemented and locally validated.
   - Evidence: read-only test-install disk inspection found a generated data
     repository with roughly 2.12 GiB of loose Git objects and only one commit.
     Current installed config does not enable `push_to_git`, so this is private
     generated-repository object-store waste rather than public feed data.
   - Expected benefit: lower disk growth when Git publication is enabled or was
     enabled previously and sync attempts fail after staging large generated
     files.
   - Risk: Git maintenance must not change working-tree content, generated feed
     files, or public behavior.
   - Verification: `pkg/output.SyncGit` now runs best-effort `git gc --auto`
     after enabled Git sync attempts; `install.sh` runs `git gc --prune=now`
     for existing generated data/web repositories only during allowed mutable
     runtime repair. `go test ./pkg/output -run 'TestSyncGit|TestWriteGit' -count=1`
     and `bash -n install.sh` passed.

6. Provider-defaults marker can cause repeated full reprocess after interrupted
   runs.
   - Classification: rejected as work-not-needed under the no-functional-change
     constraint.
   - Status: rejected for implementation in this SOW milestone.
   - Evidence: `.agents/sow/specs/pipeline.md` requires a processing rebuild
     when the current default-provider identity differs from the last
     successfully published default-provider marker; `pkg/scheduler/recovery.go`
     enqueues that provider-default wave; `pkg/engine/run_pipeline.go` writes
     the marker only after successful publication; existing tests assert missing
     marker drift and full global fan-out.
   - Expected benefit if skipped: lower restart CPU and memory after a crash or
     OOM before marker write.
   - Rejection reason: without a successful marker, the daemon cannot prove that
     canonical ASN/GEO-derived public artifacts were fully regenerated and
     published with the current defaults. Writing the marker earlier or skipping
     the wave can make stale provider-derived artifacts look current.
   - Verification: existing tests
     `TestProviderDefaultsMarkerDetectsConfigDrift`,
     `TestProviderDefaultsReprocessQueuesFullFeedTargets`, and
     `provider default drift forces global fan-out` preserve the current
     required behavior.

7. Admitted feed body differs byte-for-byte but parses to the same set.
   - Classification: mostly rejected as already handled for automatic
     downloader-stage work; remaining cases are functional forced/manual/recovery
     reprocess behavior.
   - Status: proven with targeted regression test; no behavior change needed.
   - Evidence: `pkg/engine/download_stage.go` prepares a canonical feed body
     before queuing processing and compares it with the latest committed
     canonical body; `TestFetchAndStageSkipsProcessingWhenRawBytesChangeButCanonicalBodySame`
     proves a byte-different raw download with the same canonical set returns
     `same` and does not queue processing.
   - Expected benefit: no material benefit on the ordinary automatic download
     path because it is already filtered before processing.
   - Rejection reason: adding a post-parse shortcut inside
     `processAndCommit` would affect forced/manual/recovery reprocess runs that
     intentionally refresh headers, versions, source/processed timestamps,
     retention outputs, and derived artifacts.
   - Verification: `go test ./pkg/engine` passed after adding the proof test.

8. Byte-identical heavy-phase artifact writes.
   - Classification: work-not-needed.
   - Status: implemented for the safe no-functional-change publication
     boundary and locally validated; writer-specific pre-stage skip rewrites
     are rejected for this SOW.
   - Evidence: several writers build payloads then write atomically with
     processing mtimes; entity sidecars already have a `DeepEqual` skip pattern;
     heavy public artifacts and entity artifacts are written into staging trees
     before publication, so the safe shared no-functional-change boundary is
     staged publish rather than each writer.
   - Expected benefit: lower disk I/O, file cache, slab pressure, and CPU.
   - Risk: integrity mtime contract must be preserved using deliberate touches;
     compare failures must not introduce new publication failures.
   - Verification: `TestStagedPublishBatchTouchesIdenticalLiveFileInPlace`
     proves a byte-identical staged artifact keeps the live file in place,
     updates logical mtime, removes stage scratch, and reports the live path as
     published; direct comparison tests cover missing, changed, non-regular,
     empty, small identical, and buffer-boundary files; `go test ./pkg/engine`
     passed.
   - Remaining-scope decision: skipping each writer before it creates staged
     output would require per-artifact dependency proofs, per-writer failure
     ordering review, and separate mtime/integrity validation. The shared
     publication boundary removes the live rewrite/file-cache/inode churn
     without changing writer contracts, and live timing evidence identified
     comparison scans rather than writer serialization as the dominant CPU
     cost. The deeper writer-specific rewrite is rejected as not worth the
     added risk in SOW-0103.

9. Full home aggregate rebuild when output is unchanged.
   - Classification: work-not-needed or needed-but-inefficient depending on
     measured input changes.
   - Status: rejected under the no-functional-change constraint.
   - Expected benefit: lower per-run CPU and small-file reads.
   - Risk: stale homepage summaries.
   - Evidence: `pkg/engine/home_aggregates.go` stores `generated_at` in the
     precomputed `web/home/aggregates.json` artifact and sets it from
     `e.now().UTC()` during each rebuild. `pkg/web/home_api.go` serves summary
     and globe responses from that artifact, and `.agents/sow/specs/files-layout.md`
     defines the artifact as the precomputed homepage rollup participating in
     integrity validation.
   - Rejection reason: skipping rebuilds when the derived aggregate facts are
     unchanged would also preserve the old `generated_at` value and change the
     observable artifact body/freshness semantics. A deterministic
     dependency-derived timestamp would require a separate product/contract
     decision, not a no-functional-change optimization.

10. Unconditional history CSV append for duplicate observations.
   - Classification: rejected.
   - Status: rejected under the no-functional-change constraint.
   - Expected benefit: lower disk growth for frequently reprocessed large feeds.
   - Risk: historical chart behavior must not change.
   - Evidence: `pkg/engine/finalize.go` appends every finalized observation to
     the internal `lib/<feed>/history.csv` ledger; `pkg/engine/file_contract_test.go`
     requires this ledger to contain the full internal history, while the public
     `_history.csv` is only the configured tail; `pkg/engine/runtime_ledger_cache_test.go`
     intentionally uses same-timestamp rows as correction history where the
     latest row wins.
   - Rejection reason: suppressing duplicate or same-timestamp ledger rows can
     change internal version/history statistics, correction behavior, and public
     tail generation after bootstrap. The downloader history-snapshot path is
     already timestamp-keyed and returns no change for identical same-timestamp
     snapshots.
   - Verification: existing tests `TestBashCompatibleHistoryChangesetAndRetentionFiles`,
     `TestObserveHistoryPointReloadsSameTimestampCorrectionFromLedger`, and
     `TestAppendHistorySnapshotSameTimestampNoChangeReturnsFalse` document the
     current contracts.

11. Raw mirror/download publication rewrites byte-identical feed files.
   - Classification: work-not-needed.
   - Status: implemented and locally validated.
   - Evidence: `pkg/engine/web_ipsets.go` copied every updated redistributable
     `.ipset`/`.netset` into `runtime.web_dir_for_ipsets` through a temporary
     file, `fsync`, and rename even when the destination already had identical
     bytes.
   - Expected benefit: lower disk I/O, file-cache pressure, and inode churn for
     forced/recovery runs or other updates that republish unchanged raw mirror
     files.
   - Risk: mirror file content, permissions, owner, and mtime must remain
     replacement-equivalent.
   - Verification: `TestCopyFileViaNewTouchesIdenticalDestinationInPlace`
     proves byte-identical mirror files are touched in place with source mtime
     and generated mode; `TestCopyFileViaNewReplacesChangedDestination` proves
     changed mirror files still use the replacement path; the existing
     `TestCopyUpdatedIPSetsToWebDirForIPSets` integration test still passes.

12. Same-timestamp, same-count history observations reload the full internal
    ledger.
   - Classification: work-not-needed.
   - Status: implemented and locally validated.
   - Evidence: read-only live test-install metrics showed
     `sources.finalize.observe_history` consumed 104,280 ms across 402 feeds in
     one run. `pkg/engine/runtime_ledger_cache.go` previously reloaded
     `lib/<feed>/history.csv` whenever the new observation timestamp matched
     the cached last timestamp, even when entries and unique IP counts were
     unchanged and therefore no effective history correction was possible.
   - Expected benefit: lower CPU and disk I/O on forced, recovery, or
     provider-triggered reprocess waves where many feeds keep the same source
     timestamp and same effective counts.
   - Risk: same-timestamp corrections with changed counts must continue to
     reload/recompute ledger state so min/max, version, and public history tail
     remain correct.
   - Verification: `TestObserveHistoryPointSameTimestampSameCountsUsesCachedStats`
     proves the no-op case uses cached stats without requiring the internal
     ledger; `TestObserveHistoryPointReloadsSameTimestampCorrectionFromLedger`
     continues to prove changed same-timestamp counts reload the ledger and
     correct min/max facts.

### Priority 2 - Work Needed But Inefficient

1. Pairwise comparison overlap scans dominate the metadata phase.
   - Classification: needed-but-inefficient.
   - Status: implemented and locally validated.
   - Evidence: read-only live test-install metrics showed one run spent
     835,455 ms in `metadata.write_comparison_files`; 830,193 ms of that was
     `metadata.comparison_pair_overlap` over 38,223 exact overlap counts. The
     existing implementation already used min/max bounds and a coarse occupied
     prefix bitmap, but many pairs still reached full two-stream overlap
     counting.
   - Expected benefit: lower CPU in metadata comparison runs by skipping exact
     zero-overlap pairs earlier and replacing identical normalized set pairs
     with their known full overlap count.
   - Risk: comparison artifacts must still contain every non-zero overlap row
     and remove every zero-overlap stale row. Any shortcut must be
     conservative: uncertainty must fall back to full overlap counting.
   - Implementation: the comparison preparation pass now builds the existing
     coarse prefix bitmap, a capped sparse `/24` occupied-prefix set, and a
     normalized range-content digest in one pass over each set. Pair workers
     skip a full overlap scan only for exact identical content, disjoint
     bounds, disjoint sparse prefixes, or the existing disjoint coarse prefix
     proof.
   - Verification: `TestComparisonSparsePrefixOverlap`,
     `TestComparisonSparsePrefixOverflowFallsBack`,
     `TestComparisonSetsIdentical`, and
     `TestWriteComparisonFilesUsesSparsePrefixToRemoveSameCoarsePrefixStaleRows`
     cover the new exact skip paths; existing comparison zero-row and merge
     tests still pass.

2. Bogon and critical-infrastructure provider overlaps repeat exact range scans
   for provably disjoint feed/provider pairs.
   - Classification: needed-but-inefficient.
   - Status: implemented and locally validated.
   - Evidence: read-only live test-install metrics showed the last run spent
     104,830 ms in the `critical_infrastructure` phase and 34,797 ms in the
     `bogons` phase. Both phases compare feed sets against provider/reference
     sets through exact range overlap/intersection operations.
   - Expected benefit: lower CPU in critical-infrastructure and bogon phases
     when feed/provider occupied address spaces are provably disjoint.
   - Risk: provider artifacts must still publish zero-overlap payloads where
     required, and non-zero overlaps must never be skipped.
   - Implementation: critical and bogon provider sets now carry precomputed
     conservative overlap filters. The bogon phase precomputes target-feed
     filters once per run. Provider scans are skipped only when range bounds,
     sparse occupied `/24` prefixes, or coarse occupied prefixes prove
     disjointness.
   - Verification: `TestRangeOverlapFiltersDisjoint` covers the shared
     disjoint/overlap filter behavior; focused `TestCritical` and `TestBogon`
     engine tests still pass.

3. Feed processing materializes current and previous sets in memory.
   - Classification: needed-but-inefficient.
   - Status: implemented for the safe no-functional-change retention and
     binary-persistence paths and locally validated; deeper active-set
     streaming is rejected for SOW-0103.
   - Evidence: `pkg/engine/process.go` parses the current set and previously
     loaded the previous latest set into heap before retention diff; the new
     path opens the previous binary latest through `openPreviousLatestSet` as a
     file-backed `iprange.FileSet` when possible, uses `retentionDiffFromSources`
     over `iprange.RangeSource`, and counts removed IPs through
     `countUniqueIter` instead of materializing a removed set. Retention cohort
     reconciliation now also opens existing binary cohort files through the
     file-backed cohort opener and materializes only the still-listed cohort
     that may need to be persisted.
   - Expected benefit: lower peak heap and GC pressure for high-entry feeds by
     eliminating the previous-latest heap allocation and the removed-set heap
     allocation during retention diff, plus lower heap during removal-heavy
     cohort reconciliation. The current parsed set, new cohort set, and
     rewritten still-listed cohort set are still materialized because they are
     committed outputs when they exist.
   - Risk: retention diff correctness and binary latest format compatibility.
   - Verification: `TestRetentionDiffUsesFileBackedPreviousLatest` proves the
     previous latest is opened as an `iprange.FileSet` and the file-backed diff
     matches the old in-memory diff; existing retention and file-contract tests
     still pass. `TestReconcileRetentionCohortUsesFileBackedSource` proves a
     binary retention cohort is opened file-backed, reconciled correctly, and
     rewritten with only still-listed IPs.
   - Remaining-scope decision: the active current set is still the engine's
     canonical representation for finalize, latest-binary persistence, header
     rendering, content hash, kernel apply, retention, and downstream fan-out.
     Removing it would require redesigning parser, writer, and downstream
     contracts across the processing engine. SOW-0103 therefore keeps the
     current active-set contract and removes only the extra previous/latest,
     removed-set, and binary serialization buffers that can be removed without
     changing behavior.

4. DroneBL parser builds all parsed lists and output sets in memory.
   - Classification: needed-but-inefficient.
   - Status: implemented for the configured-output update path and locally
     validated; the full parser is intentionally retained for its existing
     tool/helper contract.
   - Evidence: `tools/dronebl2ipsets` still exposes `ParseBuildzone` as a
     full-fidelity parser for callers that need every class, but the normal
     `Update` path now computes the configured output class set from
     `OutputSpec` values and retains ranges only for `global` plus selected
     classes while still scanning all lines and preserving parse warnings.
   - Expected benefit: lower heap and less GC during DroneBL materialization,
     especially when operators enable only a subset of DroneBL child feeds.
     The selected output sets are still materialized because the existing
     contract writes complete `.source` files for every selected child.
   - Risk: derived child source ordering/content must remain identical or
     intentionally canonical-equivalent.
   - Verification: `TestParseBuildzoneForListsStoresOnlySelectedListsAndPreservesWarnings`
     proves unselected classes are not retained while parse warnings survive;
     `TestBuildOutputsFromFilteredParseMatchesFullParse` proves configured
     outputs from the filtered parse match the old full-parse output.
   - Remaining-scope decision: the normal update path no longer retains
     unselected classes. The full parser remains available because it is the
     existing full-fidelity parser surface for tests and callers that request
     all classes. Removing or narrowing that surface would be a functional
     tool/API change, not a no-functional-change optimization.

5. Retention storage for large high-churn DroneBL feeds is expensive.
   - Classification: needed-but-inefficient unless retention semantics are
     changed.
   - Status: requires functional/product decision; recorded as follow-up
     `.agents/sow/pending/SOW-0104-20260614-retention-storage-compaction-design.md`.
   - Evidence: live test-install showed very large `new/` retention directories
     for DroneBL-derived feeds. `.agents/sow/specs/files-layout.md` defines
     `lib/{feed}/new/{unix_timestamp}` as the authoritative current-membership
     cohort source for per-IP listing age and search `first_seen`;
     `pkg/engine/retention_update.go` rewrites these cohorts after removals and
     appends removal-life evidence to `retention.csv`.
   - Expected benefit: high disk and file-cache reduction.
   - Risk: high; retention semantics and historical APIs may depend on these
     snapshots.
   - Verification: not implemented in this no-functional-change SOW; SOW-0104
     must define exact retention facts, compaction policy, migration, repair,
     integrity, and equivalence tests before implementation.

6. ASN/Bogon provider parsing and memory residency.
   - Classification: needed-but-inefficient.
   - Status: implemented for the verified repeated ASN/bogon work and locally
     validated; broader provider-residency changes are rejected for SOW-0103
     without stronger live evidence.
   - Evidence: provider fan-out is required by the processing-engine and
     files-layout specs, so broad provider loading is not removable under the
     no-functional-change rule. However, ASN comparison previously computed
     the provider-independent feed-vs-bogon overlap once per feed per ASN
     provider via `CountFeedWithBogons`.
   - Expected benefit: lower heavy-run CPU for ASN comparison when several ASN
     providers are configured; with four providers this removes three repeated
     bogon-overlap sweeps per target feed while preserving provider-specific
     residual ASN counting.
   - Risk: `bogon_ips`, `unknown_ips`, `attributed_ips`, and `by_asn` must stay
     byte/semantics equivalent to the previous full per-provider
     `CountFeedWithBogons` behavior.
   - Implementation: `asnloc.Database` now exposes `CountFeedExcluding` for
     the residual-count half of `CountFeedWithBogons`. The engine precomputes
     `bogon_ips` once per target feed only when more than one ASN provider is
     loaded, then each provider counts ASN attribution over the same non-bogon
     residual. A feed uses the optimized residual path only when the precompute
     map contains an explicit split value; missing split entries fall back to
     the original `CountFeedWithBogons` behavior, while proven-disjoint feeds
     record an explicit zero split.
   - Verification: `TestCountFeedExcludingMatchesBogonResidual` proves the
     residual API matches `CountFeedWithBogons`; `TestWriteASNComparisonFilesReusesPrecomputedBogonSplit`
     proves two ASN providers publish the same expected bogon/unknown split
     through the engine path; `TestCountASNFeedWithBogonSplitFallsBackWhenSplitMissing`
     proves missing precompute entries do not become false zero-bogon outputs;
     `TestPrecomputeASNBogonSplitsRecordsDisjointZero` proves a proven-disjoint
     feed gets an explicit zero split rather than an absent split.
   - Remaining-scope decision: provider fan-out and provider-specific
     databases are required by the public artifact contract. The implemented
     change removes repeated provider-independent bogon work. Replacing broader
     provider residency/parsing with a different storage model would be a
     larger provider architecture change without current evidence that it is a
     primary live OOM or CPU cause.

7. ASN lookup cache replacement on reload can retain old provider memory.
   - Classification: needed-but-inefficient.
   - Status: implemented and locally validated.
   - Evidence: before this change, `Reload()` replaced `e.asnLookupCache` with
     a new cache after validation, while the old cache could hold open text/MMDB
     provider databases used by public IP lookup.
   - Expected benefit: lower long-lived heap/mmap retention after reloads that
     change or refresh ASN provider files.
   - Risk: public lookup reads the cache pointer outside the engine mutex today;
     closing old databases during reload needs a concurrency-safe cache
     ownership change, otherwise an in-flight lookup could use a just-closed
     database.
   - Implementation: the ASN lookup cache object now stays stable across
     reloads, while reload retires every cached provider entry. Lookups and
     entity/detail builders acquire leases for cached databases and release
     them when the operation finishes. New lookups cannot acquire retired
     entries, and retired entries close after the last active lease releases
     them.
   - Verification: `TestASNDatabaseCacheRetiresLeasedEntriesAfterRelease`,
     `TestASNDatabaseCacheReplacementKeepsOldLeaseOpenUntilRelease`,
     `TestASNDatabaseCacheKeepsExistingEntryWhenReplacementOpenFails`, and
     `TestReloadRetiresASNLookupCacheWithoutReplacingCache` cover the
     ownership rules; focused race tests pass.

8. Entity sidecar rebuilds hold broad maps in memory.
   - Classification: needed-but-inefficient.
   - Status: implemented for safe result-buffer bounding and locally
     validated; full rebuild behavior is retained as required repair/bootstrap
     behavior.
   - Evidence: full entity rebuild and repair paths are required behavior, but
     the sidecar build worker channel was buffered to `len(names)`, allowing a
     full rebuild to hold one completed sidecar result per target feed before
     the consumer stages or aggregates them.
   - Expected benefit: lower peak heap during full publish/entity rebuilds and
     repair/bootstrap paths. Normal surgical feed updates already limit the
     target set and are lower impact.
   - Risk: worker cancellation must still drain/close cleanly and output
     sidecars must remain identical.
   - Implementation: sidecar build result buffering is now bounded by worker
     concurrency, capped by target count; error sends are context-aware so
     cancellation cannot strand workers on a bounded channel.
   - Verification: `TestFeedEntitySidecarResultBufferSizeBoundedByWorkers`
     covers the buffer-size contract; focused entity/home-detail/integrity
     tests and race tests pass.
   - Remaining-scope decision: the full rebuild itself is necessary for repair,
     bootstrap, and provider fan-out. Incremental detail staging beyond the
     bounded result channel would require a broader entity-artifact design pass
     and has not been shown to dominate the live OOM/CPU/disk symptoms after
     the higher-impact fixes in this SOW.

9. Public/admin status snapshots allocate broad copies.
   - Classification: needed-but-inefficient.
   - Status: implemented for the duplicated `/api/v1/admin/status` snapshot
     path and locally validated; broader one-endpoint snapshots are rejected as
     not material to SOW-0103.
   - Expected benefit: lower allocation spikes during admin polling.
   - Risk: race-safety and snapshot consistency.
   - Implementation: `/api/v1/admin/status` now builds one
     artifact-inclusive entry snapshot and reuses it for both feed and artifact
     sections in the same response. Feed rows still iterate only configured
     source names, so artifact-only entries do not become feed rows.
   - Verification: existing admin endpoint tests cover artifact parents staying
     out of the feeds list; focused web tests passed.
   - Remaining-scope decision: the status response previously built two broad
     snapshots for one response path. Other admin endpoints build their own
     single-purpose response snapshots. Those copies are metadata-only,
     request-scoped, and lower risk than range-body/provider/heavy-phase work;
     they are not a material remaining OOM or disk-growth cause for SOW-0103.

10. Go memory limit not aligned with systemd cgroup limit.
   - Classification: needed operational guardrail.
   - Status: implemented and locally validated.
   - Evidence: the generated systemd unit already set `MemoryMax=2G`, but did
     not set `GOMEMLIMIT` or `MemoryHigh`; operator docs already recommended
     pairing cgroup memory controls with `GOMEMLIMIT`.
   - Expected benefit: lower OOM-kill risk and earlier Go GC under cgroup
     pressure, while leaving headroom for file cache, slab, mmap/file-backed
     reads, and other memory outside the Go runtime soft target.
   - Risk: too-low limit can increase CPU/GC or slow runs.
   - Verification: `install.sh` now writes `MemoryHigh=1536M`,
     `MemoryMax=2G`, and `GOMEMLIMIT=1536MiB`; operator docs, memory spec, and
     project operations skill were updated to match. Local install smoke confirmed
     systemd reports `MemoryHigh=1610612736`, `MemoryMax=2147483648`, and
     `GOMEMLIMIT=1536MiB`.

11. Binary latest and retention cohort writes double-buffer serialized payloads.
   - Classification: needed-but-inefficient.
   - Status: implemented and locally validated.
   - Evidence: `pkg/engine/helpers.go` previously serialized every binary set
     into a `bytes.Buffer` before the atomic write. `pkg/iprange/binary.go`
     also built one `len(set.Ranges)*8` payload slice before writing ranges.
     This duplicated memory while the active `IPSet` was already live during
     finalize and retention cohort writes.
   - Expected benefit: lower peak heap and GC pressure when persisting large
     `latest` snapshots and large retention cohorts.
   - Risk: binary format, atomic replacement, generated file mode, and logical
     mtime must remain unchanged.
   - Implementation: binary set writes now stream directly to the atomic temp
     file, and the binary payload writer emits fixed-size range chunks instead
     of allocating a whole payload slice.
   - Verification: focused binary round-trip, file-backed set, retention,
     staged publication, and same-content publication tests passed.

## Plan

1. Approve SOW scope and move it to current.
2. Baseline existing behavior:
   - run full tests appropriate to current branch;
   - create deterministic output-equivalence fixtures for DroneBL, processing,
     retention, heavy phases, metadata, and entity artifacts;
   - add resource instrumentation where needed to verify theories.
3. Rank all theories by measured CPU, heap, cgroup memory, file-cache, disk, and
   I/O impact.
4. Implement highest-impact work-not-needed fixes first:
   - DroneBL rsync/fetch/extract cleanup and bounded staging;
   - provider-default interrupted-run loop if proven;
   - post-parse same-set short-circuit if proven;
   - byte-identical write suppression with mtime preservation.
5. Implement highest-impact needed-but-inefficient fixes:
   - streaming/file-backed processing and retention diffing;
   - DroneBL bounded parsing/output generation;
   - entity sidecar memory reduction;
   - provider cache/mmap improvements;
   - cgroup-aware memory settings if kept in this SOW.
6. Validate with local tests, external reviewers, and sanitized live
   test-install observations.

## Execution Log

### 2026-06-13

- Created SOW from user request and prior SOW-0097/reviewer/live-install
  evidence.
- No code or specs changed.
- No implementation started.

### 2026-06-14

- User approved autonomous execution under the no-functional-change constraint.
- User selected one umbrella SOW with milestones, read-only live validation, and
  `GOMEMLIMIT` in scope as an operational guardrail.
- User clarified that non-functional implementation details should not block on
  user decisions; any step needing a functional/product decision should be
  skipped, recorded, and followed by the next highest-impact theory.
- Moved SOW to `.agents/sow/current/` and started the baseline phase.
- First milestone selected: verify and fix DroneBL rsync/fetch/extract waste
  before optimizing lower-confidence theories.
- Baseline validation before code changes passed:
  - `cd tools/dronebl2ipsets && go test ./...`
  - `go test ./pkg/engine`
  - `make test`
- Implemented DroneBL acquisition cleanup:
  - rsync now targets the consumed `buildzone` input directly;
  - rsync no longer uses persistent partial/progress flags in the committed
    fetch area;
  - custom rsync acquisition inherits the runtime artifact timeout;
  - stale DroneBL fetch siblings and per-run fetch scratch are removed;
  - failed rsync leaves the previous committed `buildzone` usable.
- Implemented DroneBL materialization cleanup:
  - stale private `extract/outputs-*` directories are removed before a new
    materialization attempt.
- Updated maintainer-facing specs for artifact acquisition scope and scratch
  cleanup contracts.
- Post-change validation passed:
  - `cd tools/dronebl2ipsets && go test ./...`
  - `go test ./pkg/engine`
  - `make test-tools`
  - `make test`
- Analyzed the provider-default marker repeated-reprocess theory and rejected
  code changes under the no-functional-change rule:
  - current specs require missing/stale provider-default markers to force
    provider-derived artifact rebuilds;
  - existing engine/scheduler tests assert that behavior;
  - skipping the rebuild or writing the marker before successful publication
    could serve stale canonical ASN/GEO-derived artifacts.
- Analyzed the byte-different/same-set processing theory:
  - added `TestFetchAndStageSkipsProcessingWhenRawBytesChangeButCanonicalBodySame`;
  - verified normal automatic downloads already compare prepared canonical
    bodies and avoid processing when the parsed set is unchanged;
  - rejected a deeper `processAndCommit` shortcut because it would change
    forced/manual/recovery reprocess semantics.
- Implemented byte-identical public/entity publish suppression at the shared
  staged publish boundary:
  - when staged and live artifact bytes are identical, publication keeps the
    live file in place and updates mode, owner, and logical mtime;
  - comparison is streaming/file-backed and falls back to normal replacement
    when comparison cannot prove equality;
  - updated pipeline, files-layout, and memory-management specs for this
    public/entity publication contract.
- Completed the first external reviewer round for the current milestone. The
  reviewers found no correctness, security, crash-recovery, race, data-loss, or
  integrity-mtime blockers. All material findings were low-risk test/documentation
  gaps.
- Addressed the accepted reviewer findings:
  - added changed-live-file publish coverage so different staged content cannot
    be misclassified as byte-identical;
  - added direct streaming comparison tests for missing live files,
    different-size files, same-size different content, symlink/non-regular live
    paths, empty files, and files crossing the comparison buffer boundary;
  - added entity publish coverage proving the shared staged publish helper also
    touches byte-identical entity artifacts in place;
  - added DroneBL URL-normalization coverage for module-root, direct
    `buildzone`, trailing-slash, path-only, and empty inputs;
  - added DroneBL `Timeout == 0` coverage proving rsync `--timeout` is omitted
    when no timeout is configured;
  - strengthened the rsync-failure test so a failed transfer after writing a
    partial staged `buildzone` still preserves the previous committed
    `buildzone` and cleans scratch.
- Documented the shared public/entity publication contract in the relevant
  maintainer-facing specs.
- Explicitly rejected non-blocking reviewer suggestions that would not improve
  the no-functional-change milestone enough to justify churn:
  - logging byte-comparison errors was left out because fallback-to-replace is
    the contract and per-artifact comparison misses would add operator noise;
  - simplifying `promoteFetchedBuildzone` was left out because reviewers found
    the current atomic two-step promotion correct and this SOW is optimizing
    resource behavior, not code style;
  - forcing `mod.UTC()` in the publish touch path was left out because
    replacement equivalence means preserving the staged file's mtime exactly;
    producers already normalize logical mtimes where required.
- Collected second-round valid reviewer outputs for the same milestone scope
  before continuing to the next theory. Valid completed reviewers again found no
  correctness, security, data-loss, crash-recovery, race, or integrity-mtime
  blockers for the milestone.
- Disregarded one reviewer output as invalid because it violated the mandatory
  read-only/no-recursion review rule by attempting to run other reviewers.
- Accepted a low-risk comparator robustness finding:
  - `sameRegularFileContent` now uses `io.ReadFull` against the stat size and
    verifies EOF afterward, so short reads or size drift fall back safely to the
    normal replacement path;
  - added the missing small byte-identical file case to
    `TestSameRegularFileContent`.
- Validation after the comparator fix passed:
  - `go test ./pkg/engine -run 'TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent|TestCleanupDroneBLExtractDir|TestFetchAndStageSkipsProcessing' -count=1`
  - `cd tools/dronebl2ipsets && go test ./... -count=1`
  - `make test`
  - `make test-tools`
  - `go test ./pkg/engine -race -run 'TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent|TestCleanupDroneBLExtractDir|TestFetchAndStageSkipsProcessing' -count=1`
  - `git diff --check`
- Analyzed duplicate internal history/history-snapshot write theory:
  - rejected suppressing internal `history.csv` rows because the project
    explicitly keeps a full internal append-only ledger and supports
    same-timestamp correction rows;
  - confirmed downloader history snapshots already avoid rewriting identical
    same-timestamp snapshots.
- Implemented byte-identical raw mirror/download publish suppression:
  - `copyFileViaNew` now keeps an existing mirror file in place when it is
    byte-identical to the committed canonical feed file;
  - the live mirror file still receives generated mode, configured owner, and
    canonical feed mtime;
  - comparison failures and changed files fall back to the existing
    temporary-copy-and-rename path.
- Updated maintainer-facing specs for raw mirror/download publication
  equivalence and bounded comparison.
- Implemented retention diff heap reduction:
  - previous committed binary latest sets are opened file-backed for retention
    diff when possible;
  - removed-IP counts are computed by streaming iteration instead of
    materializing a removed set;
  - existing binary retention cohorts are opened file-backed during removal
    reconciliation, and only the still-listed rewritten cohort is materialized;
  - durable retention writes still happen only after successful canonical feed
    and latest-binary finalization, preserving the existing failure order.
- Updated processing-engine and memory-management specs for file-backed
  retention diffing and the finalize-before-retention write boundary.
- Implemented DroneBL parser heap reduction:
  - `Update` now derives the selected DroneBL class list from configured
    output specs and uses a filtered parser path;
  - the full parser remains available for tests and full-fidelity callers;
  - filtered parsing still scans all buildzone lines and preserves warnings,
    but avoids retaining ranges for unselected classes.
- Implemented post-start cleanup for publish stage leftovers:
  - daemon startup still performs the existing immediate cleanup of old publish
    stage directories;
  - a daemon-owned delayed cleanup now runs after the grace period and removes
    only publish stage directories whose mtimes predate the current process
    start time;
  - this catches OOM/restart leftovers that were too recent for immediate
    startup cleanup without deleting stage directories created by the current
    process.
- Implemented generated Git object-store maintenance:
  - enabled runtime Git sync attempts now run best-effort `git gc --auto` after
    sync work, including failure paths after Git has created loose objects;
  - the installer now compacts existing generated `data/` and `web/` Git
    repositories with `git gc --prune=now` only when mutable runtime repair is
    allowed and after ownership repair has made the repositories usable by the
    service user.
- Collected read-only live timing evidence after a completed run:
  - metadata pairwise comparison dominated the run, with
    `metadata.write_comparison_files` at 835,455 ms and
    `metadata.comparison_pair_overlap` at 830,193 ms across 38,223 overlap
    counts;
  - source history observation also showed material cost, with
    `sources.finalize.observe_history` at 104,280 ms across 402 feeds.
- Implemented exact pairwise comparison prefilters:
  - comparison preparation now builds a capped sparse `/24` occupied-prefix set
    and a normalized range-content digest while it already scans each set for
    the existing coarse prefix bitmap;
  - pair workers skip full overlap scans only for exact identical content,
    disjoint bounds, disjoint sparse prefixes, or disjoint coarse prefixes;
  - uncertain pairs still run the full exact overlap count.
- Implemented same-timestamp history observation no-op:
  - when a finalized observation has the same timestamp, entries, and unique IP
    count as the cached last observation, the engine reuses cached ledger stats
    instead of rescanning the full internal history ledger;
  - same-timestamp observations with changed counts still reload the internal
    ledger to preserve correction behavior.
- Implemented conservative provider-overlap filtering for bogon and
  critical-infrastructure phases:
  - provider/reference sets now carry the same conservative range-overlap
    filter used by pairwise comparison;
  - bogon target feed filters are precomputed once per phase;
  - feed/provider overlap scans are skipped only when bounds or occupied-prefix
    filters prove zero overlap.
- Validation found that manually constructed test provider sets can have an
  unknown zero-value overlap filter. Fixed the shared filter so unknown filters
  never prove disjointness and always fall back to exact overlap counting; only
  known-empty filters may skip scans as empty.
- Implemented the managed-service memory guardrail:
  - the installed systemd unit still keeps the hard cgroup limit at
    `MemoryMax=2G`;
  - the unit now sets `MemoryHigh=1536M` and `GOMEMLIMIT=1536MiB`, leaving
    headroom for file cache, slab, mmap/file-backed reads, and non-Go memory;
  - operator docs, the memory-management spec, and the project operations skill
    now describe the installed defaults and the fact that `GOMEMLIMIT` is a Go
    soft target, not a hard limit.
- Collected three read-only explorer reports for remaining SOW-0103 theories:
  - provider fan-out is required behavior, but ASN/bogon comparison had
    provider-independent work repeated per ASN provider;
  - entity sidecar full rebuilds are necessary repair/bootstrap behavior, with
    medium-to-high peak-heap reduction available through bounded buffering and
    incremental detail staging;
  - admin/public status snapshots and home aggregate rebuilds are real lower
    priority costs, but not the likely primary OOM cause.
- Implemented ASN/bogon split reuse:
  - `asnloc.Database.CountFeedExcluding` exposes the residual counting half of
    `CountFeedWithBogons`;
  - when several ASN providers are loaded, the engine computes feed-vs-bogon
    overlap once per target feed and reuses that `bogon_ips` count across
    providers;
  - every provider still counts the non-bogon residual through its own ASN
    database, preserving `unknown_ips`, `attributed_ips`, and `by_asn`.
- Implemented ASN lookup-cache retirement on reload:
  - reload now retires the stable cache object's provider entries instead of
    replacing the cache pointer;
  - public IP lookup and entity/detail builders hold leases while using a
    cached ASN database;
  - retired provider databases close after their last in-flight lease releases,
    preserving lookup behavior while freeing old provider memory after reload.
- Implemented entity sidecar result-buffer bounding:
  - sidecar build workers now use a result buffer bounded by worker
    concurrency instead of target feed count;
  - error sends are context-aware so bounded buffering does not strand workers
    during cancellation;
  - this preserves full rebuild/repair semantics while reducing peak heap when
    many sidecars finish faster than the staging/aggregation consumer.
- Implemented admin status snapshot reuse:
  - `/api/v1/admin/status` now reuses one artifact-inclusive entry snapshot for
    its feed and artifact sections;
  - feed rows still iterate configured source names, so artifact-only entries
    remain excluded from the feed table.
- Recorded no-functional-change skips:
  - homepage aggregate rebuild skipping was rejected because
    `web/home/aggregates.json` includes an observable `generated_at` value;
  - retention storage compaction was moved to
    `.agents/sow/pending/SOW-0104-20260614-retention-storage-compaction-design.md`
    because exact cohort/removal evidence semantics need a product decision.
- Implemented binary set writer heap reduction:
  - latest and retention cohort writes now stream directly to their atomic temp
    files instead of first building a whole-file `bytes.Buffer`;
  - the standalone `iprange.WriteBinary` writer now emits fixed-size range
    chunks instead of one `len(set.Ranges)*8` payload slice;
  - updated memory-management and processing-engine specs for bounded binary
    set writes.

## Validation

Acceptance criteria evidence:

- The first optimization was preceded by baseline validation of the affected
  nested DroneBL module, engine package, and full Go test suite.
- DroneBL resource behavior is fixed for verified fetch/extract waste:
  acquisition no longer persists unconsumed rsync sibling files, no longer uses
  persistent partial/progress flags, cleans stale private scratch, and keeps the
  last committed parent input on acquisition failure.
- Public/admin API behavior, feed semantics, retention semantics, and published
  output contracts were not intentionally changed.

Tests or equivalent validation:

- Baseline before implementation:
  - `cd tools/dronebl2ipsets && go test ./...` passed.
  - `go test ./pkg/engine` passed.
  - `make test` passed.
- Regression tests added:
  - `TestFetchBuildzonePromotesOnlyBuildzone`
  - `TestFetchBuildzoneKeepsExistingBuildzoneOnRsyncFailure`
  - `TestFetchBuildzoneAcceptsDirectBuildzoneURL`
  - `TestCleanupDroneBLExtractDirRemovesOnlyOutputScratchDirs`
- Post-change validation:
  - `cd tools/dronebl2ipsets && go test ./...` passed.
  - `go test ./pkg/engine` passed.
  - `make test-tools` passed.
  - `make test` passed.
- Provider-default marker contract check:
  - `go test ./pkg/engine ./pkg/scheduler` passed.
- Same-canonical-body proof:
  - `go test ./pkg/engine` passed.
- Byte-identical publish validation:
  - `go test ./pkg/engine` passed.
- Broad validation after moving the same-canonical-body proof into its own
  focused test file:
  - `make test` passed, including `tools/archposture`.
  - `make test-tools` passed.
  - `git diff --check` passed.
- Post-review validation after addressing accepted reviewer findings:
  - `go test ./pkg/engine -run 'TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent|TestCleanupDroneBLExtractDir|TestFetchAndStageSkipsProcessing' -count=1` passed.
  - `cd tools/dronebl2ipsets && go test ./... -count=1` passed.
  - `go test ./pkg/engine -count=1` passed.
  - `make test` passed, including `tools/archposture`.
  - `make test-tools` passed.
  - `git diff --check` passed.
- Post-second-review validation after the comparator robustness fix:
  - `go test ./pkg/engine -run 'TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent|TestCleanupDroneBLExtractDir|TestFetchAndStageSkipsProcessing' -count=1` passed.
  - `cd tools/dronebl2ipsets && go test ./... -count=1` passed.
  - `make test` passed, including `tools/archposture`.
  - `make test-tools` passed.
  - `go test ./pkg/engine -race -run 'TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent|TestCleanupDroneBLExtractDir|TestFetchAndStageSkipsProcessing' -count=1` passed.
  - `git diff --check` passed.
- Raw mirror/download publish validation:
  - `go test ./pkg/engine -run 'TestCopyFileViaNew|TestCopyUpdatedIPSetsToWebDirForIPSets|TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent' -count=1` passed.
  - `make test` passed, including `tools/archposture`.
  - `make test-tools` passed.
  - `go test ./pkg/engine -race -run 'TestCopyFileViaNew|TestCopyUpdatedIPSetsToWebDirForIPSets|TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent' -count=1` passed.
  - `git diff --check` passed.
- Retention diff validation:
  - `go test ./pkg/engine -run 'TestRetentionDiffUsesFileBackedPreviousLatest|TestBashCompatibleHistoryChangesetAndRetentionFiles|TestBuildCurrentRetentionBuckets|TestHistoryLedgerCache|Retention' -count=1` passed.
  - `go test ./pkg/engine -run 'TestRetentionDiffUsesFileBackedPreviousLatest|TestReconcileRetentionCohortUsesFileBackedSource|TestBashCompatibleHistoryChangesetAndRetentionFiles|Retention' -count=1` passed.
  - `go test ./pkg/iprange -run 'TestIterMatchesInMemory|TestIterOpsWithFileSet|TestLargeFileSet|TestFileSet' -count=1` passed.
  - `make test` passed, including `tools/archposture`.
  - `make test-tools` passed.
  - `go test ./pkg/engine -race -run 'TestRetentionDiffUsesFileBackedPreviousLatest|TestBashCompatibleHistoryChangesetAndRetentionFiles|TestCopyFileViaNew|TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent' -count=1` passed.
  - `git diff --check` passed.
- DroneBL filtered-parser validation:
  - `cd tools/dronebl2ipsets && go test ./... -count=1` passed.
  - `go test ./pkg/config ./pkg/engine -run 'TestCatalogDroneBLMappings|TestDroneBL|TestCleanupDroneBLExtractDir|TestFetchAndStageSkipsProcessing' -count=1` passed.
  - `cd tools/dronebl2ipsets && go test -race ./... -count=1` passed.
  - `go test ./pkg/engine -race -run 'TestRetentionDiffUsesFileBackedPreviousLatest|TestBashCompatibleHistoryChangesetAndRetentionFiles|TestCopyFileViaNew|TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent|TestFetchAndStageSkipsProcessing|TestCleanupDroneBLExtractDir' -count=1` passed.
- Publish stage cleanup validation:
  - `go test ./pkg/engine -run 'TestEngineCleanup.*PublishStages|TestCleanupStalePublishStageDirs' -count=1` passed.
  - `go test ./pkg/web -run 'TestPrepareEngineForRunCleansOnlyOldPublishStages|TestDelayedPublishStageCleanupStopsOnContextCancel' -count=1` passed.
- Generated Git object-store validation:
  - `bash -n install.sh` passed.
  - `go test ./pkg/output -run 'TestSyncGit|TestWriteGit' -count=1` passed.
- Pairwise comparison and history no-op validation:
  - `go test ./pkg/engine -run 'TestComparisonPrefixOverlap|TestComparisonSparsePrefixOverlap|TestComparisonSparsePrefixOverflowFallsBack|TestComparisonSetsIdentical|TestWriteComparisonFilesUsesSparsePrefixToRemoveSameCoarsePrefixStaleRows|TestWriteComparisonFilesRemovesStaleZeroOverlapRows|TestBuildSetMetadata|TestMergeCompareRows|TestValidateComparisonPayload' -count=1` passed.
  - `go test ./pkg/iprange -run 'TestIterOps|TestFileSet|TestOverlap|TestCompare' -count=1` passed.
  - `go test ./pkg/engine -run 'TestHistoryLedgerCacheAppliesAndObserves|TestObserveHistoryPoint|TestBashCompatibleHistoryChangesetAndRetentionFiles|TestFeedBodySame|TestRetentionDiffUsesFileBackedPreviousLatest|TestReconcileRetentionCohortUsesFileBackedSource' -count=1` passed.
  - `go test ./pkg/engine -run 'TestComparison|TestWriteComparisonFiles|TestHistoryLedgerCache|TestObserveHistoryPoint|TestRetention|TestCopyFileViaNew|TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent' -count=1` passed.
  - `go test ./pkg/engine -race -run 'TestComparison|TestWriteComparisonFiles|TestHistoryLedgerCache|TestObserveHistoryPoint|TestRetentionDiffUsesFileBackedPreviousLatest|TestReconcileRetentionCohortUsesFileBackedSource|TestCopyFileViaNew|TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent' -count=1` passed.
  - `make test` passed.
  - `make test-tools` passed.
  - `bash -n install.sh` passed.
  - `git diff --check` passed.
- Provider-overlap filter validation:
  - `go test ./pkg/engine -run 'TestRangeOverlapFiltersDisjoint|TestCritical|TestBogon|TestComparisonSparsePrefix|TestComparisonSetsIdentical|TestWriteComparisonFilesUsesSparsePrefix' -count=1` passed.
- Provider-overlap filter follow-up validation after the unknown-filter fix:
  - `go test ./pkg/engine -run 'TestWriteBogonComparisonFilesIncludesMergeDerivedProvider|TestWriteCriticalInfrastructureFilesDeduplicatesAndSkipsReferenceTargets|TestRangeOverlapFiltersDisjoint|TestCritical|TestBogon' -count=1` passed.
  - `go test ./pkg/engine -race -run 'TestRangeOverlapFiltersDisjoint|TestCritical|TestBogon|TestComparison|TestWriteComparisonFiles|TestHistoryLedgerCache|TestObserveHistoryPoint|TestRetentionDiffUsesFileBackedPreviousLatest|TestReconcileRetentionCohortUsesFileBackedSource|TestCopyFileViaNew|TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent' -count=1` passed.
  - `make test` passed, including `tools/archposture`.
  - `make test-tools` passed.
  - `bash -n install.sh` passed.
  - `git diff --check` passed.
- Managed-service memory guardrail validation:
  - `bash -n install.sh` passed.
  - Documentation/spec updates were checked for surface fit: operator-facing
    install/runtime docs describe what operators need to configure and inspect;
    the maintainer-facing memory spec records the product contract and
    limitations.
- ASN/bogon split reuse validation:
  - `go test ./pkg/asnloc -run 'TestCountFeedWithBogons|TestCountFeedExcludingMatchesBogonResidual' -count=1` passed.
  - `go test ./pkg/engine -run 'TestWriteASNComparisonFilesReusesPrecomputedBogonSplit|TestWriteBogonComparisonFilesIncludesMergeDerivedProvider|TestBuildASNFeedJSONThreeBucketInvariant' -count=1` passed.
  - `go test ./pkg/asnloc ./pkg/engine -run 'TestCountFeed|TestWriteASNComparisonFiles|TestBogon|TestCritical|TestPipeline|TestASN|TestEntity|TestHomeDetail|TestIntegrity' -count=1` passed.
  - `go test ./pkg/asnloc ./pkg/engine -race -run 'TestCountFeed|TestWriteASNComparisonFilesReusesPrecomputedBogonSplit|TestWriteBogonComparisonFilesIncludesMergeDerivedProvider|TestBuildASNFeedJSONThreeBucketInvariant|TestCritical|TestBogon' -count=1` passed.
  - `make test` passed, including `tools/archposture`.
  - `make test-tools` passed.
  - `make test-strict` passed.
  - `make race` passed, including nested `tools/dronebl2ipsets` race tests.
  - `bash -n install.sh` passed.
  - `git diff --check` passed.
- Entity sidecar result-buffer validation:
  - `go test ./pkg/engine -run 'TestFeedEntitySidecarResultBufferSizeBoundedByWorkers|TestEntity|TestHomeDetail|TestIntegrity' -count=1` passed.
  - `go test ./pkg/engine -race -run 'TestFeedEntitySidecarResultBufferSizeBoundedByWorkers|TestEntity|TestHomeDetail|TestIntegrity' -count=1` passed.
  - `make test` passed, including `tools/archposture`.
  - `make test-tools` passed.
  - `make test-strict` passed.
  - `make race` passed, including nested `tools/dronebl2ipsets` race tests.
  - `bash -n install.sh` passed.
  - `git diff --check` passed.
- ASN bogon split fallback/disjoint validation:
  - `go test ./pkg/engine -run 'TestCountASNFeedWithBogonSplitFallsBackWhenSplitMissing|TestPrecomputeASNBogonSplitsRecordsDisjointZero|TestWriteASNComparisonFilesReusesPrecomputedBogonSplit|TestBuildASNFeedJSONThreeBucketInvariant' -count=1` passed.
  - `go test ./pkg/asnloc -run 'TestCountFeedWithBogons|TestCountFeedExcludingMatchesBogonResidual' -count=1` passed.
  - `make test` passed, including `tools/archposture`.
  - `make test-tools` passed.
  - `make test-strict` passed.
  - `make race` passed, including nested `tools/dronebl2ipsets` race tests.
  - `bash -n install.sh` passed.
  - `git diff --check` passed.
- ASN lookup-cache retirement validation:
  - `go test ./pkg/engine -run 'TestASNDatabaseCache|TestReloadRetiresASNLookupCacheWithoutReplacingCache|TestReloadAppliesChangedIngestWorkerCeiling|TestLookup|TestBuild.*Detail|TestFeedEntitySidecarResultBufferSizeBoundedByWorkers' -count=1` passed.
  - `go test ./pkg/engine -race -run 'TestASNDatabaseCache|TestReloadRetiresASNLookupCacheWithoutReplacingCache' -count=1` passed.
  - `go test ./pkg/engine -count=1` passed.
  - `go test ./pkg/engine -race -run 'TestASNDatabaseCache|TestReloadRetiresASNLookupCacheWithoutReplacingCache|TestEntity|TestHomeDetail|TestIntegrity' -count=1` passed.
  - `make test` passed, including `tools/archposture`.
  - `make test-tools` passed.
  - `make test-strict` passed.
  - `make race` passed, including nested `tools/dronebl2ipsets` race tests.
  - `bash -n install.sh` passed.
  - `git diff --check` passed.
- Admin status snapshot reuse validation:
  - `go test ./pkg/web -run 'TestAdminArtifactsEndpointKeepsParentsOutOfFeedsList|TestBuild|TestPopulateAdmin|TestSanitizeSchedulerSnapshot|TestFeature|TestRoutes' -count=1` passed.
  - `make test` passed, including `tools/archposture`.
  - `make test-tools` passed.
  - `make test-strict` passed.
  - `make race` passed, including nested `tools/dronebl2ipsets` race tests.
  - `bash -n install.sh` passed.
  - `git diff --check` passed.
- Post-review validation after accepting the final reviewer test-coverage
  findings and spec/comment clarifications:
  - `go test ./pkg/engine -run 'TestASNDatabaseCache|TestReloadRetiresASNLookupCacheWithoutReplacingCache|TestComparisonSetsIdentical|TestComparisonSparse|TestRangeOverlapFiltersDisjoint|TestWriteComparisonFilesUsesSparsePrefix' -count=1` passed.
  - `go test ./pkg/engine -race -run 'TestASNDatabaseCacheSurvivesConcurrentAcquireAndRetire|TestASNDatabaseCache|TestReloadRetiresASNLookupCacheWithoutReplacingCache' -count=1` passed.
  - `make test` passed, including `tools/archposture`.
  - `make test-tools` passed.
  - `make test-strict` passed.
  - `make race` passed, including nested `tools/dronebl2ipsets` race tests.
  - `bash -n install.sh` passed.
  - `git diff --check` passed.
- Binary set writer validation:
  - `go test ./pkg/iprange -run 'TestBinary|TestFileSetLargeRoundTrip|TestFileSetRoundTrip|TestFileSetEmptyBinary' -count=1` passed.
  - `go test ./pkg/engine -run 'TestRetentionDiffUsesFileBackedPreviousLatest|TestReconcileRetentionCohortUsesFileBackedSource|TestBashCompatibleHistoryChangesetAndRetentionFiles|TestHistoryLedgerCache|TestFeedBody' -count=1` passed.
  - `go test ./pkg/engine -run 'TestCopyFileViaNew|TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent' -count=1` passed.
  - `go test ./pkg/iprange ./pkg/engine -run 'TestBinary|TestFileSetLargeRoundTrip|TestFileSetRoundTrip|TestRetentionDiffUsesFileBackedPreviousLatest|TestReconcileRetentionCohortUsesFileBackedSource|TestBashCompatibleHistoryChangesetAndRetentionFiles|TestFeedBody|TestCopyFileViaNew|TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent' -count=1` passed.
  - `go test -race ./pkg/iprange ./pkg/engine -run 'TestBinary|TestFileSetLargeRoundTrip|TestFileSetRoundTrip|TestRetentionDiffUsesFileBackedPreviousLatest|TestReconcileRetentionCohortUsesFileBackedSource|TestBashCompatibleHistoryChangesetAndRetentionFiles|TestFeedBody|TestCopyFileViaNew|TestStagedPublishBatch|TestEntityPublishBatch|TestSameRegularFileContent' -count=1` passed.
  - `go test ./tools/archposture -count=1` passed.
  - `make test` passed, including `tools/archposture`.
  - `make test-strict` passed.
  - `make race` passed, including nested `tools/dronebl2ipsets` race tests.
  - `make lint` passed.
  - `make test-tools` passed.
  - `bash -n install.sh` passed.
  - `git diff --check` passed.
- Current-state completion-audit validation after SOW-only evidence cleanup:
  - `make test` passed, including `tools/archposture`.
  - `make test-tools` passed.
  - `make lint` passed.
  - `make test-strict` passed.
  - `make race` passed, including nested `tools/dronebl2ipsets` race tests.
  - `bash -n install.sh` passed.
  - `git diff --check` passed.
  - `.agents/sow/audit.sh` reports SOW-0103 itself OK; remaining audit warnings
    are pre-existing unrelated SOW-0016/SOW-0097 hygiene.

Real-use evidence:

- Implementation commit `7f29116` was pushed to `origin/main`.
- Local `./install.sh` completed successfully and installed binary version
  `7f29116`.
- Local managed service smoke after install:
  - `systemctl is-active update-ipsets` returned `active`.
  - `systemctl show update-ipsets` reported `SubState=running`, `NRestarts=0`,
    `MemoryHigh=1610612736`, `MemoryMax=2147483648`, and
    `GOMEMLIMIT=1536MiB`.
  - `curl http://127.0.0.1:18888/healthz` returned `ok`.
  - `curl http://127.0.0.1:18888/api/v1/status` returned JSON with
    `engine.running=true`, `source_count=423`, and `merge_count=13`.
  - `curl http://127.0.0.1:18888/api/v1/sets` returned the public sets payload.
- Admin smoke on `100.97.171.108:18889` did not connect because the local
  `local-admin-dev.conf` systemd drop-in overrides
  `UPDATE_IPSETS_ADMIN_LISTEN_ARG=` to blank; `ss` showed only
  `127.0.0.1:18888` listening. This is a local override observation, not a
  SOW-0103 implementation change.
- A read-only disk snapshot during active entity publication reported
  `/opt/update-ipsets` around 13 GiB, but `du` raced with temporary
  `.update-ipsets-*` stage files being removed. Treat this as an approximate
  smoke snapshot, not a stable disk-growth measurement.
- Existing sanitized live evidence remains the basis for prioritizing this
  milestone: DroneBL fetch retained unconsumed sibling files and stale extract
  scratch existed before the fix.

Reviewer findings:

- First external reviewer round completed for the DroneBL fetch/extract and
  byte-identical publish milestone.
- Consensus: production-grade for the milestone scope, with no high/medium
  blockers and no unexpected functional behavior changes.
- Accepted and fixed:
  - missing changed-live-file publish regression guard;
  - missing direct `sameRegularFileContent` edge tests;
  - missing DroneBL URL-normalization edge tests;
  - missing `Timeout == 0` rsync argument test;
  - missing entity batch in-place publish coverage;
  - rsync-failure test did not previously simulate a partial staged
    `buildzone` before failure.
- Documented:
  - byte-identical publish applies to both public artifacts and entity
    artifacts because both use the shared staged publish helper.
  - removing rsync `-v` along with `-P` intentionally reduces daemon stderr
    noise; the artifact content and publication semantics are unchanged.
- Rejected as non-blocking/no-change-needed:
  - logging byte-comparison errors before fallback, because fallback-to-replace
    is safe and specified;
  - simplifying `promoteFetchedBuildzone`, because current behavior is correct
    and the suggestion is style-only;
  - changing publish touch to `mod.UTC()`, because exact staged-mtime
    preservation is the replacement-equivalent contract.
- Second reviewer status:
  - valid completed reviewers again assessed the milestone as production-grade,
    with no blocker or functional-regression findings;
  - accepted and fixed a low-risk streaming comparison robustness issue by
    changing the comparator to use full reads over the expected stat size and
    EOF verification;
  - accepted and fixed the missing small byte-identical file comparison test;
  - did not use one invalid reviewer output because that reviewer violated the
    mandatory no-recursion rule.
- Third reviewer status after ASN split fallback hardening:
  - five valid completed reviewers assessed the same milestone scope as
    production-grade, with no blocker or functional-regression findings;
  - did not use one invalid reviewer output because that reviewer created a
    temporary file during a read-only review, although its final conclusion was
    also production-grade;
  - accepted and fixed the real hardening issue from the previous round: ASN
    bogon split reuse now falls back to the original `CountFeedWithBogons` path
    when a feed lacks a precomputed split entry;
  - accepted and fixed the low-risk missing disjoint-zero branch test by adding
    `TestPrecomputeASNBogonSplitsRecordsDisjointZero`;
  - rejected the `MemoryHigh=1536M` versus `GOMEMLIMIT=1536MiB` unit concern as
    a false positive after checking local `systemd.resource-control(5)`, which
    states `K`, `M`, `G`, and `T` memory suffixes use base 1024 for
    `MemoryHigh=` and `MemoryMax=`.
- Fourth reviewer status after ASN lookup-cache retirement, admin snapshot
  reuse, and SOW-0104 follow-up creation:
  - six valid completed reviewers assessed the full SOW-0103 milestone scope as
    production-grade, with no critical/high blockers and no functional-regression
    findings;
  - accepted and fixed the useful test-coverage findings by adding a concurrent
    ASN cache acquire/retire race test and by adding a comparison hash test where
    different range content has the same IP count;
  - accepted and fixed low-risk clarity findings by documenting the known-empty
    range-filter invariant, documenting the RFC by-range reopen path, and
    strengthening the memory-management spec so request-serving lookup caches
    keep stable object identity across reloads;
  - rejected the suggestion to let new lookups reuse a retired ASN lookup entry
    after reload because it would violate the provider-refresh contract that
    subsequent lookups open current provider data;
  - rejected the empty-owner `chownPath` concern as a false positive after
    checking `pkg/engine/ownership.go`, where empty owner/path already no-op.
- Final reviewer status after the last accepted test/spec fixes:
  - five valid completed final-round reviewers assessed the full SOW-0103
    milestone scope as production-grade, with no correctness, security,
    data-loss, crash-recovery, race, or functional-regression blockers;
  - one original qwen reviewer session disappeared before producing a verdict,
    and the qwen rerun was stopped after it looped on the same broad engine
    `e.cfg` locking checks without producing the required final verdict;
  - the qwen rerun still provided limited validation signal before becoming
    inconclusive: it passed `make test`, `make test-tools`, `make race`,
    `make lint`, and `git diff --check`;
  - recorded the broad direct-`e.cfg` access observation as out of scope for
    SOW-0103 because it is an existing engine-wide pattern, not introduced by
    this optimization work, and changing it would be a separate concurrency
    design effort rather than a no-functional-change resource fix;
  - rejected the `asnDatabaseCache.acquire` cold-cache mutex contention
    observation as a SOW-0103 blocker because it affects only cold provider
    opens, the cache-hit path remains fast, the old implementation already
    opened under the cache lock, and moving opens outside the lock would add
    lifecycle complexity unrelated to the live CPU/OOM/disk symptoms;
  - rejected the sparse-prefix loop wraparound concern as a false positive:
    the `if prefix == end { break }` guard intentionally prevents unsigned
    wraparound when the final prefix is `0xffffffff`;
  - left byte-comparison fallback logging out by design because
    fallback-to-replace is the specified safe behavior and logging comparison
    misses would add noisy operator output without improving correctness;
  - left the harmless bogon RFC reopen-path clarity observation out of this
    milestone because it is style/clarity-only and does not affect resource
    behavior or user-facing output.
- Post-binary-writer final reviewer status:
  - six valid completed reviewers, `glm`, `minimax`, `kimi`, `qwen`, `mimo`, and
    `deepseek`, assessed the full uncommitted SOW-0103 diff and the SOW-0104
    deferral as production-grade;
  - all six ended with `PRODUCTION GRADE`;
  - no reviewer found correctness, security, data-loss, race, mtime,
    public/admin API, retention-semantics, or functional-regression blockers;
  - reviewers independently reran or checked broad validation including
    `make test`, `make test-strict`, `make race`, `make lint`,
    `make test-tools`, `bash -n install.sh`, and `git diff --check`;
  - rejected the duplicated retention iterator error-check comment request as
    non-blocking: the duplicated checks are intentional because the two
    iterator passes can observe separate file-backed I/O errors, and changing
    only comments is not a resource-impact fix;
  - recorded the now-unused `loadLatestSet` helper observation as non-blocking
    cleanup: it is not on a runtime path, has no CPU/memory/disk impact, and
    removing it after a clean final review would require another validation and
    review cycle without improving SOW-0103's production risk;
  - recorded the existing direct `e.cfg` read pattern and cold-cache ASN open
    under mutex as out of scope for this no-functional-change resource SOW,
    because they were not introduced by this work and would require separate
    engine concurrency design.

Same-failure scan:

- Searched existing engine and DroneBL tests for artifact/DroneBL coverage and
  added focused tests in the nearest existing test files.
- Checked provider-default marker/recovery paths and existing tests before
  rejecting a no-functional-change shortcut.
- Added a same-canonical-body test for downloader-stage processing admission
  before rejecting a deeper processing-stage shortcut.
- Added a staged-publish test for byte-identical live artifact handling before
  marking the publication-boundary optimization implemented.
- Added post-review same-failure tests for changed staged content, direct
  comparison edge cases, entity batch in-place publish, DroneBL URL
  normalization, no-timeout rsync args, and partial staged rsync failure.
- Broader same-failure search found retention disk compaction is not a safe
  no-functional-change implementation item; it is mapped to
  `.agents/sow/pending/SOW-0104-20260614-retention-storage-compaction-design.md`.
- Checked the separate raw mirror/download copy path after staged publish was
  optimized; added direct same-failure tests for byte-identical and changed
  mirror destinations.
- Checked the processing/retention memory theory and implemented the safe part:
  previous latest and removed-count diffing now use existing file-backed
  iterator patterns, and latest/cohort binary writers no longer allocate
  whole-file or whole-payload buffers; current-set and new-cohort
  materialization remain because removing those would require deeper parser,
  finalize, and downstream writer contract changes.

Sensitive data gate:

- This SOW intentionally omits private endpoint names, secrets, credentials,
  tokens, and customer-identifying data. Live evidence is summarized as
  sanitized test-install observations.

Artifact maintenance gate:

- AGENTS.md: no update needed for this milestone; existing bounded-work and
  SOW rules cover it.
- Runtime project skills: updated `project-operations` with installed memory
  defaults and the `GOMEMLIMIT` limitation.
- Specs: updated downloader, files-layout, memory-management, pipeline, and
  processing-engine contracts for scoped custom artifact acquisition, private
  scratch cleanup, byte-identical public/entity artifact publication,
  byte-identical raw mirror/download publication, file-backed retention
  diffing, pairwise/provider overlap filters, ASN bogon split reuse,
  bounded entity sidecar result buffering, managed-service memory defaults, and
  same-timestamp history no-op handling.
- End-user/operator docs: updated installed-service memory defaults in
  installation, systemd, runtime-environment, memory-planning, and
  troubleshooting docs.
- End-user/operator skills: no impact.
- SOW lifecycle: SOW moved to `.agents/sow/current/` and remains in progress.

Specs update:

- Updated:
  - `.agents/skills/project-operations/SKILL.md`
  - `.agents/sow/specs/downloader.md`
  - `.agents/sow/specs/files-layout.md`
  - `.agents/sow/specs/memory-management.md`
  - `.agents/sow/specs/pipeline.md`
  - `.agents/sow/specs/processing-engine.md`

Project skills update:

- Updated `.agents/skills/project-operations/SKILL.md` with the managed
  install memory defaults and guardrail limitation.

End-user/operator docs update:

- Updated:
  - `docs/installation/installation.md`
  - `docs/installation/memory-planning.md`
  - `docs/installation/systemd-setup.md`
  - `docs/running/environment-variables.md`
  - `docs/troubleshooting/common-issues.md`

End-user/operator skills update:

- No expected impact yet.

Lessons:

- For no-functional-change performance SOWs, keep the safe optimization
  boundary at already-owned publication or persistence seams unless evidence
  justifies deeper writer or parser contract changes.
- File-backed iteration and byte-identical publication suppression give useful
  resource wins without changing public feed content, retention facts, or API
  semantics when the mtime contract is preserved deliberately.
- Retention storage compaction is not merely an operational cleanup; it changes
  the evidence model for first-seen/current-membership facts and needs a product
  decision before implementation.

Follow-up mapping:

- `.agents/sow/pending/SOW-0104-20260614-retention-storage-compaction-design.md`
  tracks retention storage compaction. This is intentionally outside SOW-0103
  because changing cohort storage affects first-seen/current-retention contracts
  and requires a product decision.
- Local install smoke has completed. Long-term installed-service observation
  remains needed to confirm sustained CPU, memory, OOM, and disk-growth behavior
  over real update cycles.

## Completion Audit - 2026-06-14

- Requirement: work autonomously on non-functional CPU, memory, disk, and
  performance improvements.
  - Evidence: the theory ledger covers DroneBL acquisition, publish-stage
    cleanup, generated Git object stores, byte-identical publication, raw mirror
    publication, comparison scans, history no-op handling, provider overlap
    scans, retention diffing, DroneBL parser residency, ASN lookup cache
    retirement, entity sidecar buffering, admin status snapshots, managed memory
    guardrails, and binary writer buffering.
  - Result: satisfied for local implementation scope.
- Requirement: preserve existing specs and user-facing behavior.
  - Evidence: specs were updated only for bounded implementation contracts and
    operator guardrails; published content/API/retention semantics are preserved
    by targeted tests, broad test gates, and six production-grade external
    reviews.
  - Result: satisfied locally; live install validation remains a rollout gate.
- Requirement: implement, reject with evidence, or record each improvement as a
  functional/product decision.
  - Evidence: every theory ledger entry has a terminal status. The only
    functional/product item is retention storage compaction, mapped to
    `.agents/sow/pending/SOW-0104-20260614-retention-storage-compaction-design.md`.
  - Result: satisfied.
- Requirement: do not stop for implementation-design decisions.
  - Evidence: code organization, helper placement, tests, and specs were chosen
    conservatively in the implementation; non-functional reviewer observations
    such as dead helper cleanup and cold-cache mutex structure were recorded as
    non-blocking rather than sent for user decision.
  - Result: satisfied.
- Requirement: stop for live-system mutation, commit, push, or install approval.
  - Evidence: the user explicitly approved commit, push, and local install. The
    implementation was committed as `7f29116`, pushed to `origin/main`, installed
    locally, and smoke-tested. No production switch or remote production mutation
    was performed.
  - Result: satisfied for the approved local scope.

## Outcome

Implementation is committed, pushed, locally installed, and smoke-tested, but
the SOW remains in progress for sustained live observation. The SOW-0103
non-functional resource ledger is terminal: each identified CPU, memory, disk,
and performance theory is either implemented and locally validated, rejected
with evidence under the no-functional-change constraint, or mapped to a concrete
follow-up requiring a functional/product decision.

Local validation, external review, commit, push, and local install smoke are
complete. Longer live observation remains to prove sustained OOM, CPU, and disk
behavior over real update cycles.

## Lessons Extracted

- Preserve behavior by optimizing at shared boundaries first: staged publish,
  raw mirror copy, file-backed retention sources, and binary writers removed
  repeated work or heap buffers without changing producer semantics.
- Conservative filters are acceptable only when uncertainty falls back to exact
  counting. This applies to pairwise comparison, bogon overlap, critical
  overlap, and ASN bogon split reuse.
- Operational guardrails such as `GOMEMLIMIT` are useful but not root fixes.
  They must be paired with bounded algorithms and file-backed processing.
- Disk growth and OOM are coupled under systemd cgroups because file cache,
  slab, mmap/file-backed reads, and temp/stage artifacts can contribute to the
  same `MemoryMax` pressure as Go heap.

## Followup

- `.agents/sow/pending/SOW-0104-20260614-retention-storage-compaction-design.md`
  owns retention storage compaction because it requires product decisions about
  first-seen, current-membership, removal evidence, migration, and historical
  API behavior.
- Continue installed-service observation with sanitized live evidence for CPU,
  cgroup memory breakdown, OOM events, disk growth, publish-stage cleanup,
  DroneBL fetch behavior, and admin/integrity status.
- Decide whether the local `local-admin-dev.conf` drop-in should keep admin
  disabled or be changed to expose admin on localhost/Tailscale for validation.

## Regression Log

None yet.
