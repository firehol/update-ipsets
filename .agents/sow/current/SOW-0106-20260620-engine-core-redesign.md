# SOW-0106 - Engine Core Redesign

## Status

Status: in-progress

Sub-state: paper-design phase. No implementation is approved. Five-phase
boundary map, responsibility map, crash-safety invariant, history-preservation
test strategy, and first paper performance analysis are recorded. User resolved
provider prepared-index ownership: processors own the self-contained prepared
state for normal feeds, ASN, GeoIP, bogon, and critical-infrastructure inputs.
Phase-domain and artifact-classification rules are recorded. Remaining paper
work: map current sidecar/page files to the domain model, finalize global
affected-input rules, and turn the draft into accepted spec updates.

## Requirements

### Purpose

Make update-ipsets fit for long-running production service by redesigning the
ingestion and processing core around clear phase boundaries, bounded work,
durable crash recovery, and lossless preservation of the existing feed-history
legacy.

The product purpose is not only to fetch today's feeds. The on-disk artifacts
also contain about ten years of feed history. That history is a core project
asset and MUST NOT be lost, discarded, corrupted, or silently reinterpreted by
this redesign.

### User Request

The user requested that the current broad engine be redesigned on paper before
implementation. The required baseline model has five processing phases:

1. downloader
2. feed processor
3. feeds comparisons
4. feed insights
5. feed public artifacts

For these phases, this SOW must write down:

- explicit boundaries
- inputs and outputs
- artifacts and state files
- processing flows
- crash/restart behavior
- gap analysis for missing entities or responsibilities
- paper performance analysis for duplicated work, caches, and file-format
  changes
- consolidation of pending performance-improvement SOWs into this single plan

The user also clarified:

- the five phases come from memory and may be expanded only if the analysis
  proves another entity is needed;
- the pipeline must not become an overcomplicated dynamic dependency graph;
- phases should have strong separation of concerns but may trust each other;
- every phase may keep its own disk state so reprocessing is fast;
- the whole pipeline must resume after crashes without data loss;
- the engine must not hold long-lived global feed-list locks or long-lived
  per-feed locks while processing; active in-memory feed/catalog state must be
  immutable to readers and replaced by copy-on-write snapshot swaps;
- implementation must wait until the design, gap analysis, and performance
  analysis are complete on paper.

### Assistant Understanding

Facts:

- Current code already has downloader canonicalization, but the engine still
  reparses canonical text and mixes feed-local processing, retention, heavy
  comparisons, insights, and publication in broad runs.
- Current specs describe two loops, downloader and processing engine, not the
  five explicit entities requested here.
- Current on-disk state includes canonical feed bodies, history snapshots,
  binary latest snapshots, history ledgers, changesets, retention cohorts,
  retention summaries, comparison artifacts, insight artifacts, and public
  publication artifacts.
- Existing SOWs SOW-0097, SOW-0103, SOW-0104, and SOW-0105 describe overlapping
  parts of the same backend resource and design problem.

Inferences:

- The existing implementation has accumulated behavior from the old shell
  pipeline, later enrichment work, and partial refactors. The resource problem
  is therefore unlikely to be solved by small local tuning alone.
- A phase-oriented design can keep the pipeline simple without building a
  general dependency graph, provided each phase has precise durable inputs,
  outputs, and commit markers.
- A separate pipeline coordinator/state ledger is likely required as shared
  infrastructure, but it is not a sixth processing phase unless the gap
  analysis proves it must own product work.

Unknowns:

- Whether the current retention representation can be kept with only algorithmic
  changes, or whether a new lossless format is needed.
- Which current public artifacts can be regenerated from phase state and which
  must be treated as historical compatibility evidence during migration.

### Acceptance Criteria

- This SOW contains a complete phase map for the five requested phases.
- Each phase has explicit purpose, non-goals, inputs, outputs, durable state,
  crash states, restart behavior, and artifact ownership.
- The SOW identifies whether any additional non-phase infrastructure or phase is
  required and explains why.
- A gap analysis maps every known current responsibility to one phase, one
  supporting infrastructure component, or a deliberate open decision.
- A performance analysis lists duplicated work, avoidable work, required but
  inefficient work, candidate caches, candidate file-format changes, expected
  benefit, risk, and validation method.
- The runtime concurrency model forbids processing-time locks over the whole
  feed list or an active feed. Any in-memory update uses immutable snapshots and
  a near-instant swap.
- A history-preservation and migration test strategy is written before any
  implementation.
- SOW-0097, SOW-0103, SOW-0104, and SOW-0105 are closed as consolidated into
  this SOW, without deleting their evidence or claiming their original outcomes
  were completed.
- No implementation starts until this SOW's paper design reaches
  `design-ready` state and any required user decisions are recorded.

## Analysis

### Sources Checked

Project sources and specs:

- `.agents/sow/specs/files-layout.md`
- `.agents/sow/specs/pipeline.md`
- `.agents/sow/specs/processing-engine.md`
- `.agents/sow/specs/memory-management.md`
- `.agents/sow/specs/operating-principles.md`
- `pkg/downloader/canonical.go`
- `pkg/engine/process.go`
- `pkg/engine/feed_body_stage.go`
- `pkg/engine/run_pipeline.go`
- `pkg/engine/output_comparison.go`
- `pkg/engine/integrity.go`
- `.agents/sow/done/SOW-0097-20260601-ingest-cpu-concurrency-limits.md`
- `.agents/sow/done/SOW-0103-20260613-cpu-memory-optimization-without-functional-change.md`
- `.agents/sow/done/SOW-0104-20260614-retention-storage-compaction-design.md`
- `.agents/sow/done/SOW-0105-20260615-production-unresponsiveness-diagnosis.md`
- `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md`

Open-source reference checks:

- `prometheus/prometheus @ 505095b64b43dd76baf08839e1800a8d473c97e0`
  - `tsdb/wlog/checkpoint.go:52`
  - `tsdb/wlog/checkpoint.go:88`
  - `tsdb/wlog/checkpoint.go:96`
- `grafana/loki @ eecfe8a42c441a6dad7c40309183117bb6282204`
  - `pkg/ingester/recovery.go:36`
  - `pkg/ingester/recovery.go:57`
  - `pkg/ingester/recovery.go:83`

Reference conclusion:

- The useful pattern is not to copy WAL complexity. The useful pattern is the
  simpler durable-processing principle: complete work into temporary durable
  locations, protect the latest valid recovery point, clean incomplete temporary
  state on startup, and replay/resume from durable committed records.

### Current Evidence

- Downloader canonicalization currently runs in `PrepareCanonicalFeedBody`.
  Evidence: `pkg/downloader/canonical.go:18`.
- Engine reparses the already prepared canonical body in `processAndCommit`.
  Evidence: `pkg/engine/process.go:102`.
- Engine feed processing currently includes parse, retention diff, finalize,
  retention update, rotation, and later heavy phases in one broad path.
  Evidence: `pkg/engine/process.go:104`.
- Current pipeline spec only defines downloader and processing loops.
  Evidence: `.agents/sow/specs/pipeline.md:33`.
- Current processing-engine spec makes one engine own feed-local processing,
  comparison, insight generation, and publication.
  Evidence: `.agents/sow/specs/processing-engine.md:24`.
- Current files-layout spec already treats historical state as durable product
  evidence, including feed history ledgers and retention cohorts.
  Evidence: `.agents/sow/specs/files-layout.md:388` and
  `.agents/sow/specs/files-layout.md:408`.

### Risks

- Data loss: any retention, history, or format migration bug can destroy or
  reinterpret ten years of project history.
- Silent correctness regression: skipping or narrowing work without an exact
  input/output contract can publish stale comparisons, insights, or public
  artifacts.
- Crash regression: a phase split that keeps transitions in memory can lose
  admitted work after process death.
- Overengineering: a generic dependency graph can add moving parts without
  solving the actual simple pipeline.
- Underengineering: only renaming the current engine phases would preserve the
  same broad ownership confusion and resource waste.

## Pre-Implementation Gate

Status: blocked

Problem / root-cause model:

- The current backend core is not fit for production because it still behaves as
  one broad engine pipeline. It mixes downloader output consumption, feed-local
  state, historical retention work, global comparisons, deterministic insights,
  and public artifact publication.
- That broad ownership makes it hard to prove which work is required, which work
  is duplicated, which state is authoritative, and how to recover safely after
  a crash.
- The design must be rewritten on paper first. Implementation before this map
  is complete would continue the partial-refactor pattern that created the
  current confusion.

Evidence reviewed:

- Current code and spec evidence listed in the Analysis section.
- Prior resource SOWs listed in the consolidation section.
- Local open-source reference checks for durable checkpoint/recovery patterns.

Affected contracts and surfaces:

- Downloader acquisition and canonicalization.
- Feed-local processing, history, changes, retention, and latest snapshots.
- Comparison artifacts for peer feeds and provider datasets.
- Insight artifacts.
- Public JSON, markdown, raw mirrors, entity pages, indexes, and static
  publication.
- Admin UI/API run states and progress reporting.
- Integrity, repair, migration, and startup recovery.
- Runtime resource controls and worker pools.
- Specs under `.agents/sow/specs/`.
- Future operator docs if file layout, migration, or repair workflows change.

Existing patterns to reuse:

- Atomic generated-file writes and staged publish directories.
- Explicit `.new` and `.processing` file states where already present.
- Existing `iprange` binary/file-backed readers and range-source interfaces.
- Existing comparison pair ledger concept, if it survives the redesign.
- Existing admin active-operation progress model, after phase boundaries are
  corrected.
- Existing integrity checks as repair-signal inputs, not as public request work.

Risk and blast radius:

- Very high. This work touches durable history, core feed correctness, public
  artifacts, and production availability.
- The first implementation slice after design must be small and must prove
  migration/read-compatibility before changing any writer for historical state.

Sensitive data handling plan:

- Durable artifacts in this SOW, specs, docs, skills, agent instructions, and
  code comments must not include secrets, bearer tokens, private endpoints,
  customer names, community member names, personal data, raw feed payloads, or
  non-private customer-identifying IP addresses.
- Production observations must be summarized with sanitized timings, file
  families, counts, and relative paths only.

Implementation plan:

1. Paper design only:
   - finish the five-phase map;
   - finish the gap analysis;
   - finish the paper performance analysis;
   - record required decisions.
2. Spec update only after the paper design is accepted:
   - update pipeline, processing-engine, downloader, files-layout,
     memory-management, integrity, and operating-principles specs.
3. Implementation only after specs and decisions are approved:
   - start with a read-compatible state inventory and migration harness;
   - then implement one phase boundary at a time.

Validation plan:

- SOW and spec review for complete responsibility mapping.
- External reviewer analysis after the paper design is complete, using the full
  SOW filename and asking reviewers to find missing responsibilities, data-loss
  risks, performance blind spots, and unwanted side effects.
- Migration and crash-safety test plan written before code.
- No code validation is applicable until implementation begins.

Artifact impact plan:

- AGENTS.md: not expected in this design pass unless a new project-wide SOW rule
  is discovered.
- Runtime project skills: update only if the design produces durable new agent
  workflow rules.
- Specs: expected after design acceptance, not before.
- End-user/operator docs: expected only if file layout, migration, or repair
  behavior changes.
- End-user/operator skills: not expected in paper-design phase.
- SOW lifecycle: SOW-0097, SOW-0103, SOW-0104, and SOW-0105 are consolidated
  into this SOW and closed under `.agents/sow/done/` as
  superseded/consolidated, not completed.

Open-source reference evidence:

- Listed under Sources Checked. References are used only for durable recovery
  principles, not as direct architecture templates.

Open decisions:

- None blocking this paper-design SOW. Future implementation decisions will be
  added after the gap and performance analyses are complete.

## Phase Model Draft

### Global Pipeline

The baseline pipeline is:

```text
downloader
  -> feed processor
  -> feeds comparisons
  -> feed insights
  -> feed public artifacts
```

The feed processor and feeds comparisons MAY run in parallel only when:

- both consume the same durable downloader output revision;
- neither requires uncommitted state from the other;
- resource policy allows both to run without starving public serving;
- crash recovery can retry either phase independently.

If those conditions are not met, processor and comparisons run serially. This
is a scheduling decision, not a product dependency graph.

### Phase State Machine

Each phase follows the same durable state shape:

```text
waiting -> active -> staged -> committed
                 \-> failed-retryable
                 \-> failed-hard
```

Meaning:

- `waiting`: a durable work item exists and has not been claimed.
- `active`: a worker claimed the work item; progress may be in memory, but the
  work item itself remains durable.
- `staged`: outputs are complete in phase-owned staging, but the completion
  marker is not committed yet.
- `committed`: the phase completion marker references the exact output revision
  now safe for downstream consumption.
- `failed-retryable`: old committed state remains authoritative and the same
  work item may retry.
- `failed-hard`: old committed state remains authoritative, but operator/admin
  action or upstream change is needed.

Restart rules:

- `waiting` resumes.
- `active` is treated as interrupted and retried from the phase input.
- `staged` is validated and either committed if complete or discarded/retried if
  incomplete.
- `committed` is never recomputed merely because the process restarted.
- `failed-retryable` follows retry/backoff policy.
- `failed-hard` stays visible to operators and does not erase old committed
  output.

### End-To-End Flow

Normal changed feed:

```text
feed cadence due
  -> downloader writes canonical revision
  -> feed processor updates feed-local state
  -> comparisons update peer/provider facts and fan out affected-feed list
  -> insights regenerate for affected feeds that need insight changes
  -> public artifacts regenerate for affected feeds/pages
```

Example batch shape:

```text
10 due feeds
  -> downloader: 9 updated canonical revisions, 1 no-update
  -> processor: 9 processed revisions
  -> comparisons: 9 updated feeds produce 100 affected feeds
  -> insights: 100 affected feeds checked, 90 insight payloads changed
  -> artifacts: public artifacts refreshed for the affected publication set
```

No-change feed:

```text
feed cadence due
  -> downloader proves canonical revision unchanged
  -> no processor, comparison, insight, or public-artifact work for this feed
     unless an explicit repair/rebuild reason exists
```

Provider dataset changed:

```text
provider cadence/config drift
  -> downloader commits provider revision
  -> matching processor updates provider-local prepared state/indexes
  -> comparisons rebuild only provider-dependent facts for affected feeds
  -> insights regenerate affected feeds
  -> public artifacts regenerate affected feeds/entities/indexes
```

Explicit repair:

```text
integrity/admin request
  -> coordinator starts at the earliest phase whose durable output is missing,
     stale, malformed, or explicitly requested
  -> later phases run only for the affected feed/provider set
```

### Phase Domain Model v1

Artifact ownership rule:

- An artifact belongs to the phase whose domain question it answers.
- Downstream phases may consume upstream artifacts, but must not mutate them.
- If an artifact is only a public representation of already computed facts, it
  belongs to the public artifacts phase.
- If an artifact is a prepared local index derived from one downloader artifact
  without comparing it to other feeds, it belongs to the processor phase.
- If an artifact records relationships, overlaps, attribution, or affected
  blast radius between feeds and/or provider-reference state, it belongs to the
  comparisons phase.

| Phase | Domain question | Input kind | Output kind | Disk artifacts owned | Cardinality rule |
|---|---|---|---|---|---|
| Downloader | What changed upstream or in configured source composition? | cadence/admin/repair trigger plus source config | updated canonical/provider artifact, no-update, or error | raw/source inputs where retained, canonical text revisions, candidate canonical binary revisions, download manifests, source/provider revision manifests | due feeds become `<=N` updated downloader artifacts; no-update stops here |
| Processor | What is the self-contained state of this one downloaded artifact over time? | downloader artifact revision | processed artifact revision and self-state manifest | normal-feed latest state, history, changesets, retention, feed vitals; ASN/GeoIP/bogon/critical prepared indexes; processor manifests | usually preserves updated-artifact cardinality; does not fan out to peers |
| Comparisons | Which other feeds/entities are affected by these processed updates? | processed updated feeds/providers plus committed peer/provider state | comparison facts plus affected-feed/entity set | pairwise comparison ledger/artifacts, ASN/GeoIP/bogon/critical overlap facts, comparison-derived entity sidecars, affected-feed manifests | fans out: `X` processed updates may produce `Y` affected feeds where `Y >= X` |
| Insights | What deterministic interpretation changed for affected feeds? | affected feeds plus processor and comparison facts | insight artifacts and publication target set | per-feed insight artifacts and insight manifests | filters or preserves affected-feed set; does not discover new comparison blast radius |
| Public artifacts | How are computed facts published cheaply for users and APIs? | publication target set plus processor/comparison/insight artifacts | public JSON/markdown/html/index/homepage/raw mirror outputs | public feed pages, public entity pages, maintainer pages, homepage/global indexes, sitemap, raw mirrors, publication manifests | formats all publication targets; may update global/index artifacts when their inputs changed |

### Artifact Classification v1

| Artifact family | Owner | Reason |
|---|---|---|
| Raw upstream/source files | Downloader | Acquisition evidence/input, not processed state. |
| Canonical text feed body | Downloader | Downloader output and public raw compatibility input. |
| Candidate canonical binary revision | Downloader | Binary form of the same canonical downloader output, if adopted. |
| Latest operational feed state | Processor | Normal-feed self-state after atomic processing. |
| History ledger | Processor | Feed self-history. |
| Changesets | Processor | Feed self-diff over time. |
| Retention current-membership/first-seen state | Processor | Feed self-bookkeeping over time. |
| Retention removed-life evidence | Processor | Feed self-bookkeeping over time. |
| ASN prepared index/database | Processor | Prepared from one ASN downloader artifact, no peer comparison. |
| GeoIP prepared index/database | Processor | Prepared from one GeoIP downloader artifact, no peer comparison. |
| Bogon prepared reference set | Processor | Prepared from one reference input, no peer comparison. |
| Critical-infrastructure prepared reference set/provider identity | Processor | Prepared from critical reference inputs/config before comparison fan-out. |
| Pairwise feed comparison rows/ledger | Comparisons | Relationship facts between feeds. |
| ASN/GeoIP/bogon/critical overlap facts | Comparisons | Relationship facts between feed state and prepared provider/reference state. |
| Per-feed country/ASN contribution sidecars | Comparisons | Machine-readable comparison-derived attribution facts for one feed. |
| Country/ASN aggregate detail sidecars | Comparisons | Machine-readable aggregate relationship facts used to publish entity pages. |
| Affected-feed/entity manifests | Comparisons | Blast-radius output of relationship changes. |
| Insight JSON | Insights | Deterministic interpretation over processor/comparison facts. |
| Public feed JSON/markdown/html | Public artifacts | Public representation only. |
| Public country/ASN JSON/markdown/html | Public artifacts | Public representation of comparison-derived entity facts. |
| Public maintainer JSON/markdown/html | Public artifacts | Public representation of feed metadata/state, not a comparison fact. |
| Homepage/global indexes/sitemap | Public artifacts | Public navigation and aggregate presentation from already computed facts. |
| Raw public mirror | Public artifacts | Public compatibility publication of downloader canonical text. |

Current terminology note:

- The existing code uses `sidecar` for private machine-readable JSON state.
- Sidecars are not the public pages themselves.
- If the sidecar content is a relationship/comparison fact, it belongs to
  comparisons.
- The rendered public page or API payload derived from that sidecar belongs to
  public artifacts.

### Phase-By-Phase Artifact Audit Protocol

The redesign is audited one phase at a time. A later phase must not be used to
justify an artifact until the current phase has been mapped on its own domain.

Ownership granularity:

- Ownership is at file/artifact-family level, not directory level.
- Existing directories may contain files owned by different phases because the
  current code was not organized by phase.
- Directory reorganization is a later design step, only after file-level
  ownership is mapped end to end.
- No directory reorganization may lose, rewrite, or discard valuable legacy data.

For each phase, the audit must record:

1. Domain boundary:
   - the phase question;
   - accepted inputs;
   - produced outputs;
   - forbidden responsibilities.
2. Current artifacts:
   - path or artifact family;
   - current producer;
   - current consumer;
   - current purpose.
3. Proposed ownership:
   - owner/producer after the redesign;
   - consumer/user after the redesign;
   - whether the artifact is a phase output, phase-private state, scratch,
     compatibility output, migration/repair input, or control-plane state.
4. Crash and history contract:
   - whether the artifact is safe to delete on restart;
   - whether it contains history that must be preserved;
   - the commit marker or manifest needed before another phase may consume it.
5. Performance support:
   - indexes, manifests, summaries, or alternate formats that would let the
     phase iterate without rediscovery or repeated parsing;
   - the operation they replace.
6. Elimination review:
   - artifacts that can be deleted;
   - artifacts that can be demoted to scratch;
   - artifacts that can be merged with another artifact;
   - artifacts that must stay for compatibility or history.
7. Phase exit condition:
   - open questions and risks;
   - explicit decision to accept the phase map before auditing the next phase.

This SOW must not jump across phases during the artifact audit. The active audit
order is:

1. Downloader.
2. Processor.
3. Comparisons.
4. Insights.
5. Public artifacts.

### Cross-Phase Invariants

These rules apply to every phase:

- Inputs are durable before a phase starts.
- Outputs are written to phase-owned temporary or staged paths first.
- A phase commits success only by writing or updating a durable completion
  marker after all required outputs are complete.
- A crash before the completion marker means the phase is incomplete and safe to
  retry.
- A crash after the completion marker means the phase output is authoritative
  and downstream phases may consume it.
- Temporary files and incomplete staged directories must be safe to delete or
  retry on startup.
- Existing committed history remains authoritative until a new representation is
  proven equivalent and committed.
- Public request handlers remain readers. They do not trigger upstream fetches,
  broad recomputation, migration, or repair.
- Every phase reports work size, progress, rate, and completion percentage to
  the admin/status surfaces.

### Durable Phase Handover Contract

In-memory phase handover is allowed only as an optimization. Every handover
between phases must also be represented durably on disk.

Purpose:

- Preserve continuity after crashes, watchdog kills, OOM kills, restarts, and
  abnormal termination.
- Make pending work discoverable without guessing from partially updated output
  files.
- Prevent a new pipeline wave from starting while an older wave has unprocessed
  durable handovers.

Required handover behavior:

- A phase writes its own complete output and completion marker first.
- The coordinator or phase owner writes a durable handover record before the
  downstream phase is considered admitted.
- The in-memory enqueue happens only after the durable handover exists.
- The downstream phase consumes handover records as its inbox.
- After the downstream phase commits its completion marker, the handover is
  acknowledged, archived, or removed.
- If a crash happens after the handover is written but before in-memory enqueue,
  startup must enqueue it from disk.
- If a crash happens while the downstream phase is running, startup must rerun or
  complete the same handover idempotently.
- If a crash happens after downstream completion but before handover
  acknowledgement, startup must detect the completed downstream marker and
  acknowledge the handover without repeating effects.

Handover records must include enough identity to resume without broad discovery:

- pipeline cycle id;
- source phase and destination phase;
- input revision ids or generation ids consumed by the source phase;
- source phase output manifest path/id;
- feed/provider/entity affected set handed to the next phase;
- work size summary needed for progress reporting;
- creation time and completion/acknowledgement state.

Startup rule:

- On startup, before a new synchronized processor/comparisons/insights/artifacts
  wave is admitted, the coordinator scans durable handovers and completion
  markers.
- Existing handovers are processed oldest cycle first until they are completed,
  acknowledged, or explicitly marked for operator repair.
- Downloader may continue acquiring new source revisions, but newly downloaded
  revisions must not enter the synchronized phases while older phase handovers
  are pending.

This contract replaces fragile inference from half-updated files. Phase
manifests state what is complete; handover records state what is pending.

### Phase 1 - Downloader

Purpose:

- Acquire source material.
- Run raw-source processors.
- Resolve hostnames when required.
- Normalize source material into the canonical feed representation.
- Decide whether the canonical content changed.
- Admit changed canonical feed revisions into downstream processing.

Inputs:

- Feed update frequency due events.
- Explicit operator/admin download requests.
- Restart recovery of staged downloader work.
- Artifact-parent/provider source refresh events, where the configured source is
  not itself a public feed.

Outputs:

- `updated`: a complete canonical feed revision.
- `not updated`: content-equivalent to the last committed canonical revision.
- `error`: acquisition, parse, resolve, or materialization failure.

Phase-owned artifacts and state:

- Raw/source debug artifacts where configured and safe.
- Download/extraction scratch directories.
- Canonical text body for compatibility and public raw serving.
- Canonical binary set for internal downstream use.
- Canonical revision manifest with at least:
  - feed name
  - source configuration identity
  - observed time
  - upstream mtime/etag where available
  - content hash
  - entry count
  - unique IP count
  - min/max range bounds
  - IP version/family
  - hostname count and resolution stats
  - processor stats
  - text and binary artifact paths

Boundaries:

- Downloader does not update retention.
- Downloader does not compare feeds.
- Downloader does not generate insights.
- Downloader does not write public markdown or public comparison artifacts.

Crash/restart behavior:

- Incomplete fetch/extract/canonicalization scratch is discarded or retried.
- A canonical revision becomes visible to downstream phases only after both the
  canonical representation and manifest are complete.
- The previous committed canonical revision remains usable if the downloader
  crashes before commit.
- On restart, complete staged canonical revisions are admitted; incomplete
  revisions are ignored or cleaned.

Paper performance questions:

- Should downloader persist binary canonical sets as first-class output so the
  feed processor and comparisons do not reparse canonical text?
- Which current downloader outputs are compatibility artifacts versus internal
  operational inputs?
- Can content identity, range bounds, prefix occupancy, and counts be produced
  once here and reused by comparisons and insights?

#### Downloader Artifact Audit v1

Scope:

- This subsection audits downloader-domain artifacts only.
- It does not decide processor retention formats, comparison sidecars, insights,
  or public artifact layout.
- Anything derived from comparing a feed with another feed/provider is out of
  downloader scope.

Current artifact map:

| Artifact / family | Current producer | Current consumer | Current purpose | Proposed owner | Proposed consumer | Classification | Keep / change |
|---|---|---|---|---|---|---|---|
| `data/{feed}.source` | Downloader for direct/static source acquisition | Downloader repair/rebuild paths and operator diagnostics | Retained raw upstream body | Downloader | Downloader only | Acquisition evidence / repair input | Keep where it is needed for rebuild, no-update recovery, or diagnostics; do not let later phases consume it. |
| Downloader temp files under `tmp/` and `*.tmp` | Downloader | Downloader only | Incomplete fetch/canonicalization workspace | Downloader | None after completion | Scratch | Safe to delete on restart; must not become phase handoff. |
| `data/{feed}.ipset.new` / `data/{feed}.netset.new` | Downloader | Current engine claim step | Staged canonical text body | Downloader | Processor/coordinator | Required phase handoff / compatibility text | Keep as staged text handoff unless a manifest-based replacement is approved; downstream must consume only after manifest completion. |
| `data/{feed}.ipset.processing` / `data/{feed}.netset.processing` | Current engine claim step | Current engine processing | In-flight claimed canonical body | Processor/coordinator | Processor | In-flight work marker | Reassign out of downloader domain. It may remain as an implementation marker, but it is not a downloader artifact. |
| `data/{feed}.ipset` / `data/{feed}.netset` | Current engine promotion | Downloader comparison against new body, public raw mirror generation, processing | Committed canonical text body | Processor for committed current state; downloader for candidate content | Downloader for same-body comparison, public artifacts for raw mirror input, processor/comparisons through binary/current state | Compatibility output plus committed feed state | Keep text compatibility. Redesign must clarify whether processor or coordinator promotes candidate text to committed current. |
| Candidate canonical binary feed revision | Not first-class today | Not first-class today | Avoid canonical text reparse | Downloader | Processor, then comparisons through processor-owned committed state | Performance handoff artifact | Add beside text output if equivalence and compatibility tests pass. |
| Canonical downloader revision manifest | Not first-class today | Not first-class today | Durable identity, stats, and phase handoff marker | Downloader | Processor/coordinator/admin status | Required manifest / performance index | Add. It is the commit marker that makes the staged downloader revision consumable. |
| `data/history/{parent}/{unix_timestamp}.set` | Downloader via history-snapshot append | Downloader history derivative composition | Parent snapshots used to compose derivative feeds | Downloader | Downloader for derivative composition | History-sensitive internal state | Do not delete. This file family is downloader-owned when it is retained solely to compose configured history-derivative feed revisions. Feed-local evolution history belongs to processor-owned ledgers instead. |
| History derivative canonical output | Downloader | Processor | Synthetic feed revision derived from retained history snapshots | Downloader | Processor | Canonical feed output | Keep. It enters downstream phases exactly like a normal updated feed. |
| Merge canonical output | Downloader | Processor | Synthetic canonical feed revision from source feeds | Downloader | Processor | Canonical feed output | Keep. Merge composition is downloader-domain because it produces feed input, but downstream processing is normal. |
| `lib/artifacts/{artifact}/source.new` | Downloader artifact-parent fetch | Downloader artifact-parent promotion/materialization | Staged parent artifact input | Downloader | Downloader materializer / provider processor when applicable | Staged acquisition artifact | Keep; consumable only after successful staged fetch. |
| `lib/artifacts/{artifact}/source` | Downloader artifact-parent promotion | Downloader child materialization | Committed parent artifact input | Downloader | Downloader materializer | Acquisition/materialization input | Keep. It is not a public feed artifact by itself. |
| `lib/artifacts/{artifact}/fetch/` | Downloader artifact fetcher | Downloader materializer | Fetcher workspace, often archive/extracted upstream files | Downloader | Downloader only | Scratch or bounded private cache | Default should be scratch cleanup after materialization. Retain only with explicit manifest reason. |
| `lib/artifacts/{artifact}/extract/` | Downloader materializer | Downloader materializer | Artifact-child extraction/materialization workspace | Downloader | Downloader only | Scratch or bounded private cache | Default should be scratch cleanup before/after a materialization attempt. |
| Artifact-child canonical output | Downloader materializer | Processor | Feed revision derived from artifact parent | Downloader | Processor | Canonical feed output | Keep. The child feed enters downstream phases exactly like a normal updated feed. |
| `lib/geolocation/{provider}.source.new` | Downloader provider fetch | Provider processor | Staged GeoIP provider source | Downloader | GeoIP processor/coordinator | Provider-source handoff | Keep as downloader output with manifest. |
| `lib/geolocation/{provider}.source` | Downloader provider promotion | GeoIP processor / current comparisons code | Committed GeoIP provider source | Downloader for source acquisition; processor for prepared state | GeoIP processor | Provider acquisition artifact | Keep source. Prepared indexes belong to processor, not downloader. |
| `lib/asn/{provider}/source.new` | Downloader provider fetch | Provider processor | Staged ASN provider source | Downloader | ASN processor/coordinator | Provider-source handoff | Keep as downloader output with manifest. |
| `lib/asn/{provider}/source` | Downloader provider promotion | ASN processor / current comparisons code | Committed ASN provider source | Downloader for source acquisition; processor for prepared state | ASN processor | Provider acquisition artifact | Keep source. Prepared indexes belong to processor, not downloader. |
| `data/{feed}.enabled` | Operator/config control path | Scheduler/downloader | Enable/disable marker | Control plane, not downloader content | Scheduler/downloader | Control-plane state | Exclude from downloader artifact ownership; downloader consumes it. |
| `data/.cache.json` and scheduler/download status entries | Current runtime/engine | Scheduler, admin UI, downloader decisions | Runtime status and scheduling memory | Coordinator/control plane | Scheduler/admin/downloader | Control-plane state | Exclude from phase data ownership; redesign should move phase progress to durable work/manifests. |

Downloader-owned additional artifacts/indexes to evaluate:

| Candidate artifact | Producer | Consumer | Speeds up / replaces | Notes |
|---|---|---|---|---|
| Canonical revision manifest | Downloader | Processor, coordinator, admin UI | Rediscovering content identity, counts, source identity, and handoff completeness | Required for crash-safe phase handoff. |
| Canonical binary candidate | Downloader | Processor | Reparse of canonical text by the next phase | Text body remains the compatibility/public raw form. |
| Canonical summary block | Downloader | Processor and later comparison planning through processor state | Recomputing hash, range count, unique IP count, min/max, IP family, hostname/resolution counters | May live inside the downloader manifest. |
| Artifact-parent materialization manifest | Downloader | Downloader/coordinator | Re-extracting or re-scanning parent artifacts to know which children were produced | Must record parent revision id and child outputs. |
| Provider-source revision manifest | Downloader | Provider processors/coordinator | Rechecking provider archive identity and staged completeness | Prepared indexes remain processor-owned. |
| Download admission/work-item record | Downloader/coordinator | Processor/coordinator/admin UI | Inferring pending processing from filenames only | Could be the same object as the canonical/provider revision manifest plus queue state. |

Downloader elimination and reassignment review:

| Candidate | Proposed action | Reason | Risk / validation needed |
|---|---|---|---|
| Downloader scratch in `tmp/`, artifact `fetch/`, artifact `extract/` | Demote to scratch and clean aggressively after successful materialization | These are not durable phase outputs and can grow disk unexpectedly. | Must keep enough evidence for failed-fetch diagnostics without retaining unbounded upstream payloads. |
| `.processing` as downloader-owned state | Reassign to processor/coordinator | It represents processor in-flight work, not acquisition output. | Restart recovery must still find and resume/rollback claimed feed revisions. |
| Canonical text as internal hot-path input | Keep for compatibility, but stop using it as the preferred internal hot-path format if binary is adopted | Text compatibility is required; repeated parsing is not. | Binary/text equivalence tests and public raw byte-diff tests are mandatory. |
| `data/history/{parent}/...` ownership | Resolve before implementation; do not delete | It is history-sensitive and may be processor self-history or downloader derivative input. | Any move/format change requires non-destructive migration and derivative-output equivalence tests. |
| Raw `.source` retention for every source | Do not eliminate globally in this phase | Current rebuild/no-update recovery and operator diagnostics may depend on it. | A future policy may classify which sources truly need raw retention, but only with repair-path evidence. |

Downloader phase exit status:

- Accepted facts:
  - Downloader owns acquisition, materialization, canonical candidate creation,
    and source/provider revision identity.
  - Downloader does not own feed self-history, retention, comparisons,
    insights, or public rendering.
  - Prepared ASN/GeoIP/bogon/critical indexes are processor outputs.
  - No-update stops at downloader unless an explicit repair/rebuild item targets
    existing stale state.
- Open questions before moving to processor implementation:
  1. Whether `data/history/{parent}/...` remains downloader-owned derivative
     composition input or moves to processor-owned self-history consumed by
     downloader.
  2. Whether committed canonical text promotion belongs to processor commit or a
     coordinator commit after processor success.
     - Plain-language version: after downloader creates a new canonical feed
       body, when does that body become the official current feed that the rest
       of the system trusts?
     - Current code evidence: `finalize()` writes the binary `latest`, writes
       the committed canonical text, removes the processing body, and updates
       cache/history state together. The redesign must keep those official
       current-feed artifacts consistent.
  3. Exact manifest schema for downloader canonical/provider revisions.
  4. Exact binary canonical format and migration/equivalence test plan.

### Phase 2 - Processor

Purpose:

- Consume one downloader artifact revision.
- Process that revision atomically according to its configured type.
- Maintain local state derived only from that artifact's own data over time.
- Preserve exact historical evidence required for first-seen, retention, and
  evolution semantics for normal feeds.
- Maintain prepared local indexes/state required by later phases for provider or
  reference inputs.

Processor families:

- normal feed processor;
- ASN provider processor;
- GeoIP provider processor;
- bogon/reference processor;
- critical-infrastructure reference processor;
- merge/history/artifact-child processors when their downloader output is a
  normal canonical feed revision.

Inputs:

- Updated canonical feed revisions from downloader.
- Updated provider/reference artifact revisions from downloader.
- Explicit local rebuild requests over committed canonical revisions.
- Restart recovery of processor-staged work.

Outputs:

- Processed revision for the artifact/feed/provider/reference input.
- Feed-local metrics and artifacts about the feed itself for normal feeds.
- Provider/reference prepared indexes and manifests for ASN, GeoIP, bogon, and
  critical-infrastructure inputs.
- A list of processed names whose self-state changed and should be considered by
  later phases.

Phase-owned artifacts and state:

- Committed latest binary set.
- Feed history ledger.
- Feed changesets ledger.
- Retention current-membership state.
- Retention removed-life evidence.
- Retention summaries and indexes.
- Feed-local evolution/rotation/change-rate metrics.
- Feed-local processing manifest with dependency on the downloader revision.
- ASN prepared indexes/databases keyed by ASN downloader revision and relevant
  config identity.
- GeoIP prepared indexes/databases keyed by GeoIP downloader revision and
  relevant config identity.
- Bogon/reference prepared sets keyed by downloader revision and config
  identity.
- Critical-infrastructure prepared reference sets and provider-set identity
  keyed by downloader revision and config identity.

Boundaries:

- Processor does not compare one processed input with other feeds.
- Processor does not use external source data to enrich a normal feed.
- Processor does not produce public markdown/html pages.
- Processor does not decide which peer feeds are affected by overlaps.
- Provider/reference processors prepare their own artifact for comparison use;
  they do not fan out effects to feeds by themselves.

Crash/restart behavior:

- Processor-local writes are staged before the processed revision marker is
  updated.
- Existing history, changesets, and retention state remain authoritative until
  the normal-feed processor commit marker is complete.
- If a crash happens after appending a ledger but before the completion marker,
  restart must detect and either complete the same revision idempotently or
  roll forward without double-counting.
- Legacy historical layouts remain readable until a tested migration proves
  equivalent output and keeps rollback/read-compatibility.
- Provider/reference prepared indexes remain tied to their downloader revision;
  old prepared indexes remain authoritative until the new processor commit
  marker is complete.

Paper performance questions:

- Which feed-local operations currently reread full history when a bounded
  summary or manifest is enough?
- Can retention current-membership state be represented more compactly without
  losing exact first-seen semantics?
- Can previous-latest diffing always use file-backed binary readers?
- Can changesets and retention be updated from one streaming diff instead of
  multiple passes?

#### Processor Artifact Audit v1

Scope:

- Processor-owned artifacts answer questions about one feed, provider, or
  reference input over time.
- Processor artifacts may be normal feed state, typed provider/reference
  prepared state, or feed-local summaries.
- Processor artifacts must not encode peer-feed overlap facts. Those belong to
  comparisons.
- The current code still writes some processor, comparison, and public artifacts
  from one pipeline pass. This table separates current producer from proposed
  owner.

| Artifact / file family | Current producer evidence | Current consumers | Proposed owner | Proposed consumers | Keep / change decision |
|---|---|---|---|---|---|
| `lib/{feed}/latest` | Finalize writes latest binary set after a successful feed update. | Retention diff, comparisons, lookup/search, admin metadata, repair. | Processor / normal feed processor. | Comparisons, insights, public artifacts, admin/status. | Keep. It becomes the committed hot-path set for the processed feed revision. |
| `lib/{feed}/latest.set` legacy input | Retention loader still accepts it as a legacy fallback. | Migration/read compatibility. | Processor / normal feed processor. | Migration/import and rollback compatibility. | Keep read compatibility until an exhaustive migration proves it unnecessary. |
| `data/{feed}.ipset` / `data/{feed}.netset` committed canonical text | Finalize writes canonical text from the staged downloader body. | Public raw mirror, same-body checks, legacy users, git publication, admin. | Processor for committed current text; downloader owns staged candidate text. | Public artifacts, compatibility users, migration/repair. | Keep as compatibility/public artifact input even if binary becomes the engine hot path. |
| `lib/{feed}/history.csv` | Finalize appends one committed update point. | Public history projection, insights, admin, feed-local metrics. | Processor / normal feed processor. | Insights, public artifacts, admin/status. | Keep. It is legacy history evidence and must not be lost. |
| `lib/{feed}/changesets.csv` | Retention update appends added/removed change rows. | Public changesets projection, insights, operator/debug history. | Processor / normal feed processor. | Insights, public artifacts, admin/status. | Keep. Header/schema cleanup is allowed only with read compatibility and migration tests. |
| `lib/{feed}/new/{timestamp}` | Retention update records added/current cohort files. | Retention builder/reconciler, current first-seen reconstruction. | Processor / normal feed processor. | Processor, insights, public artifacts via retention summaries. | Keep until a lossless compact representation is designed and proven. Highest data-loss risk. |
| `lib/{feed}/retention.csv` | Retention update appends removed-life rows. | Retention reconstruction, histogram, public retention, insights. | Processor / normal feed processor. | Processor, insights, public artifacts. | Keep. It is historical evidence, not disposable cache. |
| `lib/{feed}/retention_cohorts.csv` | Retention output writer emits cohort index/summary. | Retention bootstrap, public/operator retention views. | Processor / normal feed processor. | Processor, insights, public artifacts. | Keep as current performance/index artifact; may be replaced only after equivalence tests. |
| `lib/{feed}/histogram` | Retention output writer emits bash-compatible histogram cache. | Compatibility and public/operator retention views. | Processor / normal feed processor. | Public artifacts, compatibility users. | Keep unless proven unused or replaced with compatibility output. |
| `lib/{feed}/retention.json` | Retention output writer emits summary JSON. | Insights and public retention projection. | Processor / normal feed processor. | Insights, public artifacts, admin/status. | Keep. Public artifacts may copy/project it, but processor owns the source summary. |
| `data/.cache.json` | Runtime state save records finalized feed/provider fields. | Scheduler, admin UI/API, integrity, pipeline control. | Pipeline coordinator/control-plane, populated by phase events. | Downloader, processor, comparisons, admin/status. | Keep for now. Future split should move phase-specific facts into phase manifests while preserving scheduler/admin semantics. |
| `lib/asn/{provider}/source` | ASN provider processing creates or reuses provider source material. | ASN parser/prepared database load. | Processor / ASN provider processor after downloader materialization. | Comparisons and entity sidecar builders. | Keep if it remains a normalized provider source; exact downloader/processor handoff must be explicit. |
| `lib/asn/{provider}/{dataFile}` | ASN provider processing extracts or prepares the configured data file. | ASN overlap comparisons, critical infrastructure ASN context, entity sidecars. | Processor / ASN provider processor. | Comparisons, insights indirectly, public artifacts indirectly. | Keep as a prepared provider artifact keyed by downloader revision and config identity. |
| ASN provider load stats in cache state | ASN provider processing records loaded date, ranges, bytes. | Admin/status, scheduler, diagnostics. | Processor emits event; coordinator/control-plane owns durable cache file. | Admin/status and integrity. | Keep behavior; move durable phase dependency details to ASN processor manifest. |
| GeoIP prepared provider state | Current GeoIP provider cache prepares data in memory during heavy phases. | Country overlap comparisons, entity sidecars, homepage aggregates. | Processor / GeoIP provider processor. | Comparisons, insights indirectly, public artifacts indirectly. | Add durable prepared index/manifest if paper performance analysis proves repeated parse cost material. |
| GeoIP provider load stats in cache state | GeoIP processing records loaded date and range/byte counters. | Admin/status, diagnostics. | Processor emits event; coordinator/control-plane owns durable cache file. | Admin/status and integrity. | Keep behavior; move durable phase dependency details to GeoIP processor manifest. |
| Bogon/reference prepared set and union | Current bogon code loads sources and builds union inside comparison work. | Bogon overlaps and ASN bogon splits. | Processor / bogon-reference processor. | Comparisons. | Move preparation out of comparisons; add revision-keyed prepared-set manifest if needed. |
| Critical-infrastructure prepared reference sets | Current critical code loads configured references inside comparison work. | Critical infrastructure overlap files and entity/public context. | Processor / critical-reference processor. | Comparisons. | Move preparation out of comparisons; key by source revisions and critical provider-set identity. |
| Processor revision manifest (missing today) | Not a durable first-class artifact today. | Would remove rediscovery work and enable crash recovery. | Processor family that produced the processed state. | Coordinator, comparisons, insights, public artifacts, integrity. | Add. This is the durable commit marker for processor output and the handover to comparisons. |

Processor-side indexes to evaluate:

- Per-feed processor manifest keyed by downloader revision, config identity,
  canonical hash, latest binary hash, history point id, and retention revision.
- One streaming diff result feeding latest stats, changesets, retention, and
  feed-local metrics.
- File-backed or mmap-friendly latest/current cohort readers for large feeds.
- Typed prepared provider/reference manifests for ASN, GeoIP, bogon, and
  critical-infrastructure inputs.
- Optional compact retention indexes only after lossless migration and rollback
  tests prove equivalence against the ten-year history.

Processor cleanup candidates:

- Provider/reference preparation currently embedded in comparison-heavy phases
  should move to typed processors.
- Public history/changesets/retention copies should move to the public artifacts
  phase; processor keeps the authoritative internal ledgers/summaries.
- The hot-path engine should stop reparsing canonical text when a verified
  binary/latest representation is already committed.

### Phase 3 - Feeds Comparisons

Purpose:

- For each processed updated feed/provider/reference input, compute facts that
  depend on peer feeds or prepared provider/reference state.
- Fan out the blast radius of the update from the input set to the affected-feed
  set.
- Produce overlap/comparison artifacts and the affected feed list for insights
  and public artifacts.

Inputs:

- Processed updated normal-feed revisions from processor.
- Committed canonical/binary revisions for all comparison peers.
- Processor-prepared ASN, GeoIP, bogon, and critical-infrastructure state.
- Explicit local rebuild/repair requests.

Outputs:

- Per-feed and pairwise comparison artifacts.
- Provider-overlap artifacts for ASN, GeoIP, bogons, and critical
  infrastructure.
- A durable affected-feed list for the insights and public-artifact phases.
- Comparison manifest with dependency revision ids.

Phase-owned artifacts and state:

- Pairwise comparison ledger or equivalent cache keyed by feed revision
  identities.
- Feed set summaries used for cheap exact skips:
  - content hash
  - range bounds
  - prefix occupancy
  - unique IP count
- Provider prepared indexes or revision manifests.
- Staged comparison artifacts.
- Affected-feed manifest.

Boundaries:

- Comparisons do not update feed-local retention/history.
- Comparisons do not generate editorial insight text.
- Comparisons do not publish public markdown/html.
- Comparisons do not download upstream provider data; it consumes
  processor-prepared provider/reference state.
- Comparisons may produce machine-readable sidecars that encode comparison facts
  or affected entities. Rendered public pages remain owned by the artifacts
  phase.

Crash/restart behavior:

- Comparison outputs are staged and committed by revision manifest.
- A crash before the affected-feed manifest is committed means the comparison
  wave is incomplete and safe to retry.
- Existing comparison artifacts remain authoritative until the new comparison
  wave commits.
- Pair caches must never allow stale comparison facts when either side's input
  revision changed.

Paper performance questions:

- Can updated feeds be compared against all peers without rebuilding artifacts
  for unaffected feeds?
- Can all-pair scanning be avoided by revision-keyed pair caches without
  changing zero-overlap absence semantics?
- Which provider overlaps currently duplicate work across providers or feeds?
- Which comparison-derived sidecars are necessary durable outputs, and which are
  only current implementation conveniences?

#### Comparisons Artifact Audit v1

Scope:

- Comparison artifacts answer questions that require at least two inputs:
  feed-vs-feed, feed-vs-provider, feed-vs-reference, or feed-vs-entity derived
  facts.
- Comparison artifacts may be stored under a public `web/` directory today, but
  their owner is still comparisons when the content is a machine-readable
  comparison fact.
- Rendered country, ASN, maintainer, homepage, markdown, and index pages remain
  public-artifact outputs even when their source facts come from comparisons.

| Artifact / file family | Current producer evidence | Current consumers | Proposed owner | Proposed consumers | Keep / change decision |
|---|---|---|---|---|---|
| `cache/comparison-pairs-v1.json` | Pairwise comparison code reads/writes a revision-keyed pair ledger. | Pairwise overlap skipping and merge of existing comparison rows. | Feeds comparisons. | Comparisons and integrity/repair. | Keep. Strengthen identity so stale pair facts are impossible after either side changes. |
| `web/{feed}_comparison.json` | Pairwise overlap writer emits per-feed overlap rows. | Public API/UI, insights, metadata aggregates, homepage/global views. | Feeds comparisons. | Insights, public artifacts, web server, admin/status. | Keep as comparison fact artifact. Later design may move internal source to `lib/` and publish a public projection. |
| `web/{feed}_{geoProvider}.json` | GeoIP comparison writer emits country overlap facts per provider. | Public API/UI, insights, entity sidecars, homepage country aggregates. | Feeds comparisons. | Insights, public artifacts, web server. | Keep. Invalidate by feed revision, GeoIP provider revision, and GeoIP config identity. |
| `web/{feed}_asn_{provider}.json` | ASN comparison writer emits ASN overlap facts per provider. | Public API/UI, insights, critical ASN context, entity sidecars. | Feeds comparisons. | Insights, public artifacts, web server. | Keep. Invalidate by feed revision, ASN provider revision, ASN config identity, and bogon split inputs when included. |
| `web/{feed}_bogons_{provider}.json` | Bogon comparison writer emits bogon/reference overlap facts. | Public API/UI, insights, ASN unknown/bogon context. | Feeds comparisons. | Insights, public artifacts, web server. | Keep. Move provider/reference preparation to processor and keep overlap here. |
| `web/{feed}_critical_{provider}.json` | Critical-infrastructure comparison writer emits per-provider critical overlap facts. | Critical aggregate builder, public API/UI, insights. | Feeds comparisons. | Insights, public artifacts, web server. | Keep. Invalidate by feed revision and critical provider/reference revision. |
| `web/{feed}_critical_infrastructure.json` | Critical aggregate writer emits per-feed critical-infrastructure summary. | Public API/UI, insights, admin/status. | Feeds comparisons. | Insights, public artifacts, web server. | Keep. It is a comparison aggregate, not editorial insight. |
| `lib/critical_infrastructure/provider_set_id` | Current publish step writes/removes the public critical provider-set marker. | Critical drift detection and repair decisions. | Feeds comparisons plus coordinator commit marker. | Coordinator, integrity/repair, comparisons. | Keep as serving/comparison identity marker; do not confuse it with processor-prepared reference identity. |
| `lib/provider_defaults/provider_set_id` | Current publish step writes provider-default identity marker. | Provider-default drift detection for ASN/GeoIP-derived artifacts. | Coordinator/control-plane with comparisons as primary consumer. | Scheduler/coordinator, comparisons, integrity/repair. | Keep; future phase manifests should make exact affected surfaces explicit. |
| `lib/entities/feeds-pending/{feed}.json` | Entity sidecar staging writes pending feed sidecars before entity publish. | Entity artifact publisher and repair paths. | Feeds comparisons as durable handover sidecar. | Public artifacts, coordinator, integrity/repair. | Keep or replace with a comparison handover manifest. It is not a rendered public page. |
| `lib/entities/feeds/{feed}.json` | Entity artifact publisher stores feed-to-country/ASN sidecar facts. | Country/ASN detail builders, homepage aggregates, repair. | Feeds comparisons. | Public artifacts, insights indirectly, admin/status. | Keep as comparison-derived machine-readable sidecar. |
| `lib/entities/countries/{code}.json` | Current entity publisher writes country detail sidecars. | Public country JSON/markdown/index and homepage country aggregates. | Feeds comparisons for fact payload; public artifacts for rendered/public projection. | Public artifacts and web server projections. | Keep but split ownership: internal fact sidecar vs public `web/countries/*` output. |
| `lib/entities/asns/{asn}.json` | Current entity publisher writes ASN detail sidecars. | Public ASN JSON/markdown/index and homepage ASN aggregates. | Feeds comparisons for fact payload; public artifacts for rendered/public projection. | Public artifacts and web server projections. | Keep but split ownership: internal fact sidecar vs public `web/asns/*` output. |
| `lib/entities/version` | Entity publisher writes entity sidecar schema/version marker. | Entity repair, selected refresh, integrity. | Feeds comparisons. | Coordinator and integrity/repair. | Keep as comparison sidecar schema/version marker. |
| Affected-feed manifest (missing today as first-class handover) | Currently inferred from updated names, fan-out work, existing artifacts, and sidecar refresh results. | Insights and public artifact generation. | Feeds comparisons. | Insights, public artifacts, coordinator/admin. | Add. It is the durable handover from comparisons to insights. |
| Comparison revision manifest (missing today as first-class commit marker) | Current outputs rely on file presence/mtime and selected ledgers. | Integrity, restart recovery, downstream phases. | Feeds comparisons. | Coordinator, insights, public artifacts, integrity/repair. | Add. It records input revisions, config identities, output artifacts, affected feeds/entities, and completion status. |

Comparison-side indexes to evaluate:

- Pairwise overlap ledger keyed by both feed processor revision ids, comparison
  algorithm version, and relevant config identity.
- Per-feed summary index for cheap exact skips: range count, unique IP count,
  min/max bounds, prefix occupancy, and content hash.
- Provider-overlap ledgers keyed by feed processor revision plus provider
  processor revision.
- Critical/provider-default drift manifests that say exactly which comparison
  families and public surfaces are invalidated.
- Affected feed/entity handover manifest so insights and artifacts do not
  rediscover work by scanning public directories.

Comparison cleanup candidates:

- Move ASN, GeoIP, bogon, and critical reference preparation out of comparison
  code and into typed processors.
- Stop treating public JSON paths as the only source of comparison truth; keep a
  comparison-owned internal source or manifest even if a public projection keeps
  the same URL contract.
- Separate machine-readable entity sidecars from public entity JSON/markdown
  rendering.

### Phase 4 - Feed Insights

Purpose:

- Produce deterministic per-feed interpretation from feed-local state and
  comparison state.
- Consume the affected-feed set from comparisons.
- Decide which affected feeds actually need insight output changes.
- Pass the affected publication set to public artifacts.

Inputs:

- Updated feed list from feed processor.
- Affected feed list from comparisons.
- Feed-local processor manifests and artifacts.
- Comparison manifests and artifacts.
- Explicit insight rebuild requests.

Outputs:

- Per-feed insight artifacts.
- Insight manifest with dependency revision ids.
- List of affected feeds/pages whose public artifacts must be regenerated.

Phase-owned artifacts and state:

- Per-feed insights JSON or equivalent machine-readable artifact.
- Insight dependency manifest.
- Insight generation diagnostics and counters.

Boundaries:

- Insights do not parse feed bodies.
- Insights do not compute peer overlaps.
- Insights do not update retention/history.
- Insights do not publish public markdown/html directly.

Crash/restart behavior:

- Per-feed insight outputs are staged before commit.
- A crash before insight manifest commit leaves the old insight authoritative.
- Restart retries only feeds whose insight dependencies are missing, stale, or
  incomplete.

Paper performance questions:

- Can insights be regenerated only for processor-updated feeds and
  comparison-affected feeds?
- Which current insight inputs force broad catalog reads?
- Can insight dependencies be recorded so restart does not rediscover them by
  scanning public artifacts?

#### Insights Artifact Audit v1

Scope:

- Insight artifacts answer deterministic interpretation questions about one feed
  using already computed processor and comparison facts.
- Insights must not parse feed bodies, compute overlaps, prepare providers, or
  publish markdown/html.
- Current insight code writes to a public `web/` path. File ownership is still
  insights because the artifact content is an insight model, not a rendered page.

| Artifact / file family | Current producer evidence | Current consumers | Proposed owner | Proposed consumers | Keep / change decision |
|---|---|---|---|---|---|
| `web/{feed}_insights.json` | Insight writer builds and writes per-feed insight JSON. | Public API/UI, feed markdown, admin/status, operator interpretation. | Feed insights. | Public artifacts, web server, admin/status. | Keep public contract. Consider internal source plus public projection only if needed for cleaner manifests. |
| Insight target set | Current code derives targets from updated names, comparison fan-out, and missing public insight files. | Insight writer. | Feed insights input handover from comparisons/coordinator. | Feed insights. | Replace rediscovery with durable comparison-to-insights handover. |
| Insight dependency manifest (missing today) | Not a durable first-class artifact today. | Would avoid scanning public artifacts to decide freshness. | Feed insights. | Public artifacts, coordinator, integrity/repair. | Add. It records processor/comparison revisions consumed by each insight artifact. |
| Insight diagnostics/counters | Current run progress/logging exposes partial operation counts. | Admin/status and operator diagnosis. | Feed insights for phase-local counters; coordinator for durable/live status. | Admin UI/API and logs. | Keep as observability, not as a product artifact unless the status API contract requires durability. |
| Insight publication handover (missing today) | Current public artifacts regenerate based on updated/per-feed names and generated files. | Markdown/public artifact generation. | Feed insights. | Public artifacts. | Add if insights can conclude an affected feed did not change. It prevents unnecessary public rewrites. |

Insight-side indexes to evaluate:

- Per-feed insight manifest keyed by processor revision, comparison revision,
  insight algorithm version, and config fields that affect presentation.
- Missing/stale insight repair index so restart can retry only affected feeds.
- Input fact snapshots that reference processor/comparison artifacts by revision
  instead of reading public paths as source of truth.

Insight cleanup candidates:

- Stop using public artifact presence as the freshness oracle for insight work.
- Consume processor and comparison manifests directly.
- Keep insight generation deterministic and feed-local after comparison fan-out
  has produced the affected-feed set.

### Phase 5 - Feed Public Artifacts

Purpose:

- Format and publish public-facing feed artifacts from already computed local
  state.
- Keep public serving cache-first and cheap.
- Generate homepage/global/entity pages as byproducts of the artifacts phase,
  using the affected publication set and already computed state.

Inputs:

- Feed list from insights.
- Affected feed/entity/global publication set from comparisons and insights.
- Feed-local processor artifacts.
- Comparison artifacts.
- Insight artifacts.
- Static/config metadata needed for public presentation.
- Explicit public-artifact rebuild requests.

Outputs:

- Public JSON artifacts.
- Feed markdown.
- Public HTML/static artifacts where applicable.
- Raw canonical feed mirrors.
- Indexes, sitemap, homepage/entity aggregates when affected.
- Publication manifest.

Phase-owned artifacts and state:

- Staged public publish directories.
- Public `web/` artifacts.
- Raw mirror artifacts.
- Generated-file ledger.
- Public artifact manifest with dependency revision ids.

Boundaries:

- Public artifact generation does not fetch upstream data.
- Public artifact generation does not compute retention, comparisons, or
  insights.
- Public request handlers do not generate missing broad artifacts on demand.
- Public artifact generation may format comparison-derived sidecars into country,
  ASN, maintainer, homepage, and index pages.

Crash/restart behavior:

- Public artifacts are built in staging.
- Existing public artifacts remain served until a complete publish commit.
- Incomplete staging directories are safe to remove or retry on startup.
- Publication manifest updates only after the staged batch is complete.

Paper performance questions:

- Which public artifacts are rewritten byte-identically today?
- Can publication compare staged/live bytes streaming without loading whole
  files into heap?
- Can public artifact generation consume phase manifests instead of scanning
  public directories to infer freshness?

#### Public Artifacts Artifact Audit v1

Scope:

- Public artifacts own formatting and publication of already computed state.
- Public artifacts do not compute feed-local history, retention, comparisons, or
  insights.
- Public request handlers consume published artifacts and must remain cache-first
  readers except for explicitly dynamic API actions.

| Artifact / file family | Current producer evidence | Current consumers | Proposed owner | Proposed consumers | Keep / change decision |
|---|---|---|---|---|---|
| Public staged directories `.update-ipsets-web-*` | Web publish batch stages generated files before atomic publish. | Publish step and crash cleanup. | Feed public artifacts. | Public artifacts and coordinator/startup cleanup. | Keep. Staging is required for crash-safe publication. |
| `web/{feed}.json` | Metadata writer emits per-feed public metadata. | Public API/UI, admin manifest, feed pages. | Feed public artifacts. | Web server, UI, operators. | Keep public contract; source should be processor/comparison/insight manifests, not rediscovered state. |
| `web/index.json` | Metadata writer emits public feed index. | Public UI/API and client discovery. | Feed public artifacts. | Web server and UI. | Keep. Rebuild only when visible index inputs change. |
| `web/all-ipsets.json` | Metadata writer emits global public feed catalog. | Public API/UI and external clients. | Feed public artifacts. | Web server and users. | Keep public contract. |
| `web/{feed}_history.csv` | Public series writer projects processor history ledger. | Public API/UI and users. | Feed public artifacts. | Web server and users. | Keep as public projection. Processor owns the authoritative `lib/{feed}/history.csv`. |
| `web/{feed}_changesets.csv` | Public series writer projects processor changesets ledger. | Public API/UI and users. | Feed public artifacts. | Web server and users. | Keep as public projection. Processor owns the authoritative `lib/{feed}/changesets.csv`. |
| `web/{feed}_retention.json` | Public series writer projects processor retention summary. | Public API/UI and users. | Feed public artifacts. | Web server and users. | Keep as public projection. Processor owns the authoritative `lib/{feed}/retention.json`. |
| `web/{feed}.md` | Markdown writer emits feed page markdown. | Public website rendering and static serving. | Feed public artifacts. | Web server/static site. | Keep. It must be generated from phase-owned facts, not by recomputing them. |
| `web/countries/index.json` and `web/countries/{code}.json` | Entity artifact writer emits public country index/details. | Public country API/UI, sitemap, homepage. | Feed public artifacts. | Web server, UI, sitemap generator. | Keep public contract. Source facts are comparison-owned entity sidecars. |
| `web/asns/index.json` and `web/asns/{asn}.json` | Entity artifact writer emits public ASN index/details. | Public ASN API/UI, sitemap, homepage. | Feed public artifacts. | Web server, UI, sitemap generator. | Keep public contract. Source facts are comparison-owned entity sidecars. |
| `web/countries/{code}.md` and `web/asns/{asn}.md` | Entity markdown writer emits rendered entity markdown. | Public website rendering/static serving. | Feed public artifacts. | Web server/static site. | Keep as rendered public pages. |
| `web/maintainers/{slug}.md` | Maintainer markdown writer emits rendered maintainer pages. | Public website rendering/static serving and sitemap. | Feed public artifacts. | Web server/static site. | Keep. Maintainer source may be metadata-derived; it is not necessarily a comparison sidecar. |
| `web/home/aggregates.json` | Home aggregate writer emits homepage/global aggregate payload. | Public homepage UI/API. | Feed public artifacts. | Web server and UI. | Keep. Rebuild only when its declared feed/entity inputs change. |
| `web/sitemap.xml`, sitemap shards, `robots.txt`, `llms.txt` | Public metadata writer emits crawler and LLM discovery files. | Public crawlers, users, LLM consumers. | Feed public artifacts. | Web server. | Keep. Rebuild from public artifact manifests and indexes. |
| Raw public mirror under configured web ipsets directory | IP set copy step publishes redistributable canonical feed bodies. | External users and compatibility workflows. | Feed public artifacts. | Users, web server, git publication. | Keep. It must mirror processor-committed canonical text exactly. |
| Base-dir `{feed}.setinfo` | Metadata writer emits legacy setinfo rows when base dir is git-backed. | Legacy users, README generation, git publication. | Feed public artifacts. | Users and compatibility workflows. | Keep while compatibility requires it. |
| Base-dir `README.md`, `.gitignore`, timestamp script | Public metadata/output sync writes git-oriented publication helpers. | Git publication and legacy consumers. | Feed public artifacts. | Operators and external users of git output. | Keep where git publication is enabled. |
| Generated-file list passed to `output.SyncGit` | Current pipeline accumulates generated paths and timestamps. | Mtime contract, git sync, generated output publication. | Feed public artifacts. | Output sync, integrity, git publication. | Keep as publication ledger; future manifest should make it durable and phase-scoped. |
| Publication manifest (missing today) | Current publish success is inferred from staged publish, generated list, mtimes, and cache save. | Restart recovery, integrity, public serving. | Feed public artifacts. | Coordinator, integrity/repair, admin/status. | Add. It records source manifests, published files, timestamps, and completion status. |

Public-artifact indexes to evaluate:

- Publication manifest keyed by insight/comparison/processor revision ids and
  public contract version.
- Affected public surface index mapping changed feeds/entities to exact public
  files that must be rebuilt.
- Streaming staged-vs-live byte comparison to skip byte-identical rewrites while
  preserving deliberate mtime contracts.
- Durable generated-file ledger for git/publication sync and crash recovery.

Public-artifact cleanup candidates:

- Stop generating public artifacts from broad scans when phase handovers already
  state the affected feeds/entities.
- Stop reading public paths as internal source of truth for later computation.
- Preserve public URLs while allowing internal source artifacts to move under
  phase-owned `lib/` paths.

## Gap Analysis v1

### Candidate Supporting Entity - Pipeline Coordinator

Initial classification: required infrastructure, not a sixth processing phase.

Responsibilities:

- Maintain phase queues and visible states.
- Admit work from one phase to the next.
- Persist durable phase handover records before in-memory enqueue.
- Persist phase work items and completion markers.
- Recover incomplete phase work on startup.
- Drain discovered durable handovers before admitting a new synchronized
  pipeline wave.
- Enforce global resource policy across phase workers.
- Provide admin/status progress over the whole batch and each phase.

Non-goals:

- It does not parse feeds.
- It does not compute retention.
- It does not compute comparisons.
- It does not generate insights or public artifacts.

Reason this is not a dynamic dependency graph:

- The coordinator follows the fixed five-phase order.
- It records durable work items and phase markers.
- It does not infer arbitrary dependencies between artifact nodes.

### Candidate Supporting Entity - Provider Dataset Preparation

Initial classification: resolved as processor-domain work, not a sixth phase.

Placement:

- Downloader owns acquisition and raw provider revision materialization.
- Typed processors own prepared provider/reference indexes and manifests.
- Comparisons consume prepared provider/reference state and produce overlap
  facts.

Reason:

- The user selected the fixed flow downloader -> processor -> comparisons ->
  insights -> artifacts.
- Provider preparation is self-contained processing of one downloaded provider
  or reference input.
- Comparisons should not spend broad-cycle CPU preparing raw provider data before
  they can compute overlaps.

### Candidate Supporting Entity - Integrity And Repair

Initial classification: required infrastructure, not a normal phase.

Responsibilities:

- Detect missing, stale, malformed, or incomplete phase outputs.
- Enqueue fixed-pipeline repair work.
- Avoid public request triggered repair.
- Preserve old committed state until repair succeeds.

Open gap:

- Integrity currently reasons over public artifacts and processed dates. It must
  be remapped to phase manifests and durable revision ids.

### Candidate Supporting Entity - Migration/Import

Initial classification: required one-time and repair infrastructure, not a
normal phase.

Responsibilities:

- Inventory legacy state.
- Read old and new formats.
- Prove equivalence.
- Build missing new sidecars/indexes without deleting old evidence.
- Support rollback/read-compatibility until migration is proven safe.

Open gap:

- The exact migration boundary for ten years of history must be designed before
  any writer changes historical formats.

### Responsibility Map

| Responsibility | Owner | Inputs | Outputs | Notes / gaps |
|---|---|---|---|---|
| Normal feed acquisition | Downloader | feed cadence, admin download, retained source repair | raw source state, canonical revision or error | Downloader owns raw upstream work only. |
| Artifact-parent acquisition | Downloader | artifact parent cadence/admin action | committed parent artifact input | Artifact parents are not public feed processing units. |
| Artifact-child materialization | Downloader | committed parent artifact input, child config | canonical child feed revisions | Child feeds enter the same downstream phases as normal feeds. |
| History derivative composition | Downloader | retained parent snapshots, derivative config | canonical derivative feed revision | Downloader owns composition because it creates canonical input. |
| Merge composition | Downloader | committed canonical source feeds, merge config | canonical merge feed revision | Merge output is a feed revision; downstream phases treat it normally. |
| ASN provider acquisition | Downloader | provider cadence/admin action | provider source revision | Provider dataset is not a public feed. |
| ASN provider processing/indexing | Processor | ASN provider source revision | prepared ASN index/database and manifest | Special processor family; no comparison fan-out by itself. |
| GeoIP provider acquisition | Downloader | provider cadence/admin action | provider source revision | Provider dataset is not a public feed. |
| GeoIP provider processing/indexing | Processor | GeoIP provider source revision | prepared GeoIP index/database and manifest | Special processor family; no comparison fan-out by itself. |
| Bogon reference acquisition | Downloader | feed/provider cadence/config | canonical/provider revision | Classify by config role, not by name. |
| Bogon reference processing/indexing | Processor | bogon/reference revision | prepared bogon/reference set and manifest | Special processor family. |
| Critical-infrastructure reference acquisition | Downloader | configured critical sources/provider-set drift | canonical/provider revision | Downloader acquires source; comparisons interpret critical overlap. |
| Critical-infrastructure reference processing/indexing | Processor | critical reference revision/config identity | prepared critical reference set and provider-set manifest | Special processor family. |
| Canonical text output | Downloader | processed source stream | text canonical body | Compatibility/public raw serving artifact; not the internal hot-path format if binary is added. |
| Canonical binary output | Downloader | canonical IP set | binary canonical set | New candidate first-class output; currently engine writes `latest`. Needs format/migration decision. |
| Canonical revision manifest | Downloader | canonical text/binary/stats | manifest | New candidate state; required to stop rediscovery/reparse work. |
| Latest feed state | Feed processor | canonical revision | committed latest state | Feed processor owns feed-local current state, not downloader. |
| Feed history ledger | Feed processor | canonical revision, prior feed state | append/effective history rows | Must preserve ten years of history. |
| Changesets | Feed processor | streaming diff current vs previous | added/removed ledger | Should share diff pass with retention if possible. |
| Current first-seen state | Processor | streaming diff and prior retention state | exact current-membership first-seen state | Normal-feed processor; must remain exact unless user approves a semantic change. |
| Removed-life evidence | Processor | removed ranges and prior first-seen state | removal-life ledger/summary | Normal-feed processor; must remain exact enough to preserve public/operator semantics. |
| Feed-local rotation/evolution metrics | Processor | history/changes/current stats | feed-local metrics | Normal-feed processor; must not require broad catalog work. |
| Pairwise feed overlap | Feeds comparisons | processed feed revisions, peer processed feed state | comparison artifacts and pair ledger | Uses revision-keyed cache; zero-overlap absence semantics preserved. |
| ASN overlap | Feeds comparisons | processed feed revisions, processor-prepared ASN state | ASN comparison artifacts | Processor owns prepared ASN indexes. |
| GeoIP overlap | Feeds comparisons | processed feed revisions, processor-prepared GeoIP state | country comparison artifacts | Processor owns prepared GeoIP indexes. |
| Bogon overlap | Feeds comparisons | processed feed revisions, processor-prepared bogon state | bogon comparison artifacts | Bogon union should be cached by provider revision if expensive. |
| Critical-infrastructure overlap | Feeds comparisons | processed feed revisions, processor-prepared critical reference state | critical overlap artifacts | Provider-set identity drift invalidates relevant comparison state. |
| Entity/country/ASN comparison sidecars | Feeds comparisons | provider comparison facts and affected feeds | machine-readable affected-entity/comparison sidecars | Byproduct of comparisons when they encode comparison facts, not rendered pages. |
| Country/ASN/maintainer public pages and indexes | Feed public artifacts | comparison sidecars, feed metadata, insights | public JSON/markdown/pages/indexes | Rendered pages are artifacts phase. Maintainer pages may be metadata-derived rather than comparison-derived. |
| Deterministic insights | Feed insights | processor manifests, comparison manifests, affected feed list | insight artifacts, artifact-regeneration feed list | Insights should not rediscover by reading public artifacts. |
| Feed markdown | Feed public artifacts | insight/feed/comparison state | public markdown | Formatting only. |
| Public JSON artifacts | Feed public artifacts | phase-owned computed state | public JSON | Formatting/public contract only; no raw computation. |
| Raw mirror artifacts | Feed public artifacts | canonical text body | raw public mirror | Must preserve compatibility and cheap serving. |
| Homepage and global indexes | Feed public artifacts | public artifact inputs, feed metadata, entity indexes | global public artifacts | Byproduct of artifacts phase; affected-input rules must avoid unnecessary broad rebuilds. |
| Admin progress and queues | Pipeline coordinator | durable phase work plus live progress | admin status/API state | Supporting infrastructure, not product computation. |
| Integrity checks | Integrity/repair infrastructure | phase manifests, committed artifacts | repair findings/work items | Must be remapped from processed-date/public-file checks to phase manifests. |
| Startup recovery | Pipeline coordinator plus phase owners | durable in-flight markers | resumed/retried/cleaned work | Must be safe after any crash point. |
| Explicit reprocess/recheck/admin rebuild | Pipeline coordinator | operator request | phase work item(s) | Must enter the fixed pipeline at the correct phase, not force unrelated work. |
| No-update handling | Downloader plus coordinator | same canonical body decision | no downstream work for this feed | If downloader says no-update, there is nothing to do for this feed unless explicit repair/rebuild targets existing stale state. |
| Provider-default drift | Coordinator plus comparisons | provider identity marker/config | comparison/insight/artifact work | Should not force feed processor work unless canonical feed data changed. |
| Config-only drift | Coordinator plus relevant phase | stable config identity | targeted phase work | Each phase needs explicit config identity fields that affect its outputs. |

### Initial Gap Conclusions

No sixth processing phase is justified yet.

Required supporting infrastructure:

- pipeline coordinator/state ledger;
- integrity and repair;
- migration/import/read-compatibility harness.

Resolved ownership decisions from the user:

1. Provider prepared indexes belong to processors.
   - The processor phase has specialized processor families for normal feeds,
     ASN, GeoIP, bogon, and critical-infrastructure inputs.
   - Reason: the fixed flow is downloader -> processor -> comparisons ->
     insights -> artifacts. Processor atomically maintains whatever local state
     later phases need from each downloader artifact.
2. Homepage/global outputs belong to public artifacts.
   - They are generated from already computed state as publication byproducts.
3. No-update means no work for that feed.
   - If downloader proves no canonical update, processor/comparisons/insights/
     artifacts do not run for that feed unless an explicit repair/rebuild reason
     targets stale existing state.
4. Retention/storage representation is processor-internal.
   - Format changes are allowed only if they preserve history exactly and pass
     migration tests.

Remaining design gaps after artifact mapping:

1. Affected-input rules for global artifacts:
   - Homepage/global outputs belong to artifacts, but the exact affected-input
     rules must be explicit so small updates do not force unnecessary global
     rewrites.
2. Directory layout:
   - File-level ownership is mapped in v1.
   - Directory-level ownership is intentionally not assigned yet because current
     directories contain mixed ownership and may include legacy data that must
     not be moved or deleted without a migration plan.

## Performance And Continuity Design v1

### Continuity Model

User clarification:

- The target is not filesystem-level multi-file transaction atomicity.
- Phases 2-5 are sequential under the pipeline coordinator.
- The coordinator is the internal isolation mechanism: downstream phases do not
  consume a phase output until the phase handover says the output is committed.
- Startup recovery must complete, roll forward, restore, or discard staged work
  before the coordinator admits a new synchronized pipeline wave.

Implication:

- A phase may update several files during commit as long as:
  - the phase output is not handed to the next phase until commit is complete;
  - restart recovery can detect the exact state left on disk;
  - recovery can converge to either the previous official dataset or the new
    complete dataset without losing historical evidence;
  - handover artifacts are durable and idempotent.

Public serving caveat:

- Public web readers are outside the internal phase-to-phase orchestrator.
- The public artifacts phase therefore needs its own publish contract: readers
  must serve an existing official public dataset while new public artifacts are
  staged, and restart must finish or clean up any interrupted publication before
  accepting new publication work.

### Non-Blocking Runtime Snapshot Contract

User requirement:

- While the engine is running, processing work must not lock the whole feed list.
- Processing work must not hold a lock on the active feed being processed.
- Active in-memory feed/catalog state must be immutable to readers.
- Updates build new state privately and publish it by copy-on-write snapshot
  replacement.
- A lock, if used, may be held only for an instant to swap a pointer/snapshot or
  update a tiny coordinator bookkeeping record.

Forbidden while holding a feed/catalog lock:

- upstream downloads;
- parsing or canonicalization;
- binary format reads/writes;
- retention computation;
- feed comparisons;
- provider/reference preparation;
- insight generation;
- public artifact rendering;
- JSON/markdown serialization;
- disk I/O other than the tiny state needed for the swap marker itself.

Reader contract:

- Public APIs, admin reads, lookups, comparisons, and status views acquire a
  reference to one immutable snapshot and operate on that snapshot without
  holding processing locks.
- A reader may observe the previous official snapshot while a new snapshot is
  being built.
- A reader must never observe a partially mutated feed object or catalog map.
- Snapshot retirement waits until existing readers have released references or
  the language/runtime equivalent makes the old snapshot unreachable safely.

Writer contract:

- Writers build replacement feed/provider/catalog state off to the side.
- Writers validate the replacement state before publishing it.
- Writers publish by replacing a snapshot reference, not by mutating the active
  snapshot in place.
- The replacement may be per-feed, per-provider, per-phase, or whole-catalog
  depending on the artifact family, but the active object seen by readers remains
  immutable.

Implementation-language notes:

- In Go, candidate mechanisms include immutable structs/maps plus
  `atomic.Pointer`, `atomic.Value`, or a very short mutex-protected pointer swap.
- In Rust, candidate mechanisms include `Arc` snapshots plus an atomic swap
  primitive such as an `ArcSwap`-style pattern.
- These are examples, not design decisions. The invariant is language
  independent: no long-lived reader/writer locks around engine work.

Relationship to sequential phases:

- Sequential phases 2-5 control phase execution and durable handover order.
- The snapshot contract controls live in-memory visibility to public/admin/API
  readers.
- These are complementary: the orchestrator may serialize phase work while
  readers continue using the previous immutable snapshot.

### Commit Patterns

These are design patterns, not approved directory names. The implementation may
choose different names, but every phase must select one explicit pattern per
artifact family.

#### Whole-Dataset Swap

Use when a phase owns a directory as one dataset and replacing the whole dataset
is acceptable.

Example shape:

- `x/`: current official dataset.
- `x.new/`: new dataset being built.
- `x.old/`: previous official dataset during commit/recovery.
- phase manifest/handover: durable record of the committed revision and next
  phase work.

Commit sketch:

1. Build all output in `x.new/`.
2. Write and fsync a completion marker/manifest inside `x.new/`.
3. Rename `x/` to `x.old/` when `x/` exists.
4. Rename `x.new/` to `x/`.
5. Publish the phase handover.
6. Remove `x.old/` only after the new `x/` is verified as current.

Restart rules:

- If `x/` exists and has a valid completion marker, it is authoritative.
  Remove stale `x.old/` and incomplete `x.new/`.
- If `x/` is missing and `x.old/` exists, restore `x.old/` to `x/`.
- If `x/` and `x.old/` are missing but `x.new/` has a valid completion marker,
  promote `x.new/` to `x/`.
- If only incomplete `x.new/` exists, discard or quarantine it and rebuild from
  the previous durable input.
- If a handover exists after recovery, process it before starting a new wave.

Fit:

- Prepared provider/reference datasets.
- Future phase-owned generation directories.
- Full public-artifact publication if the public serving layer can point at a
  stable generation.

Risk:

- Directory-level ownership is not true today for many paths.
- Moving legacy files into whole-dataset ownership requires migration and
  rollback tests.

#### Selective Staged File Commit

Use when a phase updates a subset of files inside a shared or very large
directory and copying the whole current dataset is not acceptable.

Example shape:

- `x/`: current official dataset containing many files.
- `x.tmp/`: work-in-progress files.
- `x.staged/`: complete staged file set ready to apply.
- staged manifest: list of files to replace, create, or delete.

Commit sketch:

1. Build replacement files in `x.tmp/`.
2. Write a staged manifest that lists every target path and intended action.
3. Rename `x.tmp/` to `x.staged/`.
4. Apply the staged manifest by renaming staged files into `x/` and applying
   deletes only after required replacements are present.
5. Write/rename the phase completion marker or handover.
6. Remove `x.staged/` after all manifest actions are applied.

Restart rules:

- Delete incomplete `x.tmp/`.
- If `x.staged/` exists, replay its manifest idempotently until every target in
  `x/` matches the staged manifest, then remove `x.staged/`.
- If `x/` contains a mix of old and new files from an interrupted commit, the
  staged manifest is the source of truth and recovery rolls forward.
- If a delete was planned, recovery applies it only when the manifest proves the
  rest of the staged dataset is complete.
- If a handover exists after recovery, process it before starting a new wave.

Fit:

- Existing mixed legacy directories.
- Per-feed public JSON/markdown updates.
- Comparison fact files under a shared comparison namespace.
- Entity sidecars where only affected feeds/entities changed.

Risk:

- This pattern rolls forward; it does not require rollback of every replaced
  file.
- It is safe for internal phases because the coordinator does not let downstream
  phases consume the dataset until recovery/commit completes.

#### Idempotent Ledger Append

Use when the artifact is an append-only or append-mostly ledger and rewriting
the whole file is expensive or risky.

Example shape:

- `ledger`: current official ledger.
- `ledger.append/{revision}`: staged append batch for one input revision.
- ledger manifest: records which revision append batches have been applied.

Commit sketch:

1. Write append rows to a revision-scoped append batch.
2. Include revision id, phase id, and row identity in the batch.
3. Append or merge the batch into the ledger.
4. Mark the revision as applied only after the ledger update succeeds.

Restart rules:

- If append batch exists and revision is not marked applied, replay it
  idempotently.
- If revision is marked applied, remove stale append batch.
- Rows must be distinguishable enough to avoid double-counting after a crash
  between ledger write and applied marker.
- If idempotency cannot be proven for a legacy ledger, rebuild from authoritative
  source artifacts or use a replacement-file commit for that ledger.

Fit:

- History rows.
- Changesets.
- Retention removed-life rows.
- Other phase diagnostics that must survive restart.

Risk:

- Current legacy CSV rows may not include enough revision identity. Adding
  identity fields requires read compatibility or an external applied-batch
  manifest.

### Phase Performance And Continuity

#### Phase 1 - Downloader

Performance opportunities:

- Produce canonical text, binary set, hash, counters, and manifest in one pass.
- Hash while streaming writes instead of holding large canonical bodies in heap.
- Let downstream phases consume binary/latest artifacts instead of reparsing
  canonical text.
- Keep history-derivative parent snapshots only when they are required to
  compose configured derivative feeds.

Continuity contract:

- Downloader outputs are staged by source/feed revision.
- A canonical revision is committed only after text, binary, stats, and manifest
  are complete.
- No-update creates no downstream handover.
- Updated canonical revisions create durable processor handovers.
- Incomplete staged downloads are discarded or retried on restart.

Likely commit patterns:

- Selective staged file commit for committed canonical text in mixed `data/`
  layouts.
- Whole-dataset swap for a future per-revision canonical artifact directory.
- Idempotent append or selective commit for retained history-derivative
  snapshots.

Expected benefit:

- High CPU and memory reduction for large feeds because parse/render/hash work
  is consolidated.

#### Phase 2 - Processor

Performance opportunities:

- Use one streaming diff between previous latest and new canonical set to feed
  latest stats, changesets, retention, history, and feed-local metrics.
- Maintain processor manifests keyed by downloader revision and config identity.
- Use file-backed binary/latest readers for large feeds.
- Move ASN, GeoIP, bogon, and critical-reference preparation into typed
  processors.
- Add retention indexes or compact current-membership state only after lossless
  migration tests.

Continuity contract:

- Existing processor state remains official until the processor commit marker is
  complete.
- Ledger updates are idempotent by downloader revision or applied-batch
  manifest.
- Latest/current state is staged before becoming official.
- Provider/reference prepared indexes are tied to exact downloader revision and
  config identity.
- Processor commit creates durable comparison handovers for updated processed
  inputs.

Likely commit patterns:

- Selective staged file commit for existing mixed `lib/{feed}/` directories.
- Idempotent ledger append for history, changesets, and retention CSVs.
- Whole-dataset swap for future typed provider/reference prepared-index
  directories.

Expected benefit:

- Very high CPU/I/O reduction for high-churn feeds.
- Lower memory by avoiding repeated full-set materialization and text reparsing.

#### Phase 3 - Feeds Comparisons

Performance opportunities:

- Maintain pairwise comparison ledger keyed by both feed processor revisions.
- Maintain provider-overlap ledgers keyed by feed revision plus provider
  processor revision.
- Reuse provider-independent computations such as bogon overlap when semantics
  allow it.
- Produce explicit affected-feed and affected-entity manifests instead of
  rediscovering blast radius from public files.
- Consume processor-prepared provider/reference indexes instead of preparing
  providers inside comparison loops.

Continuity contract:

- Old comparison facts remain official until the comparison wave commits.
- Pair/provider ledgers are ignored when any recorded input revision or
  algorithm/config identity differs.
- Comparison wave commit writes a durable affected-feed/entity handover to
  insights.
- Restart completes or replays interrupted comparison staged commits before a
  new wave starts.

Likely commit patterns:

- Selective staged file commit for per-feed comparison fact files.
- Idempotent or replacement-file commit for pair/provider ledgers.
- Whole-dataset swap for a future comparison wave directory if file layout is
  reorganized.

Expected benefit:

- Very high CPU reduction on normal update cycles where most feed pairs and
  provider overlaps did not change.

#### Phase 4 - Feed Insights

Performance opportunities:

- Consume processor and comparison manifests directly.
- Generate insights only for comparison-affected feeds.
- Compare generated insight payloads before handing feeds to public artifacts.
- Record per-feed insight dependency manifests so restart does not scan public
  artifacts for freshness.

Continuity contract:

- Old insight artifact remains official until the per-feed or wave insight
  commit completes.
- Insight manifest records exact processor/comparison revisions consumed.
- Insight commit creates durable public-artifact handover for only the feeds or
  pages whose publication may change.
- Restart retries only missing, stale, or incomplete insight work.

Likely commit patterns:

- Selective staged file commit for per-feed insight artifacts.
- Replacement-file commit for insight dependency manifests.

Expected benefit:

- Medium CPU/I/O reduction and a major correctness simplification because
  insights no longer rediscover state from public artifact paths.

#### Phase 5 - Feed Public Artifacts

Performance opportunities:

- Use an affected public-surface manifest rather than broad public directory
  scans.
- Skip byte-identical rewrites with streaming staged-vs-live comparison while
  preserving deliberate mtime contracts.
- Keep a durable generated-file/publication ledger for git sync and restart
  recovery.
- Rebuild homepage, global, entity, sitemap, robots, and LLM surfaces only from
  explicit affected-input rules.

Continuity contract:

- Existing public artifacts remain official while new artifacts are staged.
- Interrupted staging is completed, rolled forward, or discarded before new
  public publication work starts.
- Public publication writes a completion manifest after generated files, mtimes,
  and git/publication sync inputs are consistent.
- Public request handlers remain readers of official artifacts; they do not
  trigger broad recomputation or repair.

Likely commit patterns:

- Selective staged file commit for affected public files in today's `web/`
  layout.
- Whole-dataset swap or generation-pointer publication if public serving needs
  stronger whole-site snapshot isolation later.
- Durable generated-file ledger for git sync/publication state.

Expected benefit:

- Medium to high I/O reduction.
- Lower risk of public half-publication after crash/restart.

### Continuity Validation Requirements

Before implementation changes any writer, tests must cover these crash points
for each selected commit pattern:

- crash before staged work exists;
- crash after partial staged writes;
- crash after staged completion marker;
- crash after first target rename;
- crash after partial manifest replay;
- crash after phase commit marker but before handover removal;
- crash after handover creation but before downstream phase starts;
- restart with stale `*.tmp`, `*.new`, `*.old`, or `*.staged` paths.

Required assertions:

- No historical evidence is deleted or corrupted.
- Restart converges to one official dataset.
- Downstream phase handovers are processed exactly once.
- Ledger rows are not double-counted.
- Public serving uses official artifacts only.
- Re-running the same durable input revision is idempotent.

### Non-Blocking Snapshot Validation Requirements

Before implementation changes live in-memory feed/catalog state, tests must
cover these concurrency cases:

- public lookup/read while downloader is preparing an update;
- public lookup/read while processor builds replacement feed state;
- public lookup/read while comparisons build new overlap facts;
- public lookup/read while insights/public artifacts stage publication;
- admin status reads while phase state changes;
- multiple readers holding an old snapshot while a writer publishes a new
  snapshot;
- a writer failure before the snapshot swap;
- a writer failure immediately after the snapshot swap.

Required assertions:

- Readers are never blocked by long-running processing work.
- Readers observe either the old official snapshot or the new official snapshot,
  never a partially mutated one.
- No engine phase holds a global feed-list lock or active-feed lock while doing
  parsing, comparison, retention, serialization, or disk I/O.
- Snapshot swaps are bounded to tiny critical sections.
- Old snapshots remain valid for readers that acquired them before the swap.
- Failed writer work does not modify the active snapshot.

## Paper Performance Analysis v1

### Duplicated Or Avoidable Work To Verify

| Finding | Class | Current signal | Candidate fix | Expected benefit | Risk | Validation |
|---|---|---|---|---|---|---|
| Canonical text reparse | duplicated work | Downloader creates an `IPSet`; engine later parses canonical text again. | Downloader emits binary canonical set and manifest; downstream phases consume file-backed binary. | High CPU and heap reduction for large feeds. | Text compatibility/public raw body must remain exact. | Golden canonical text + binary equivalence tests; no public raw diff. |
| Canonical body byte comparison from heap body | needed but inefficient | Downloader-stage comparison uses prepared body bytes. | Stream canonical render to staged text and binary, hash while writing. | Lower heap for large feeds. | Same-body detection must remain exact. | Same-body/changed-body tests with large fixture. |
| Repeated latest/history reads during feed processing | duplicated work | Process/finalize/retention paths can read latest/history state separately. | One streaming diff feeds latest stats, changesets, retention, and feed-local metrics. | High CPU/I/O reduction on high-churn feeds. | First-seen and removed-life semantics must stay exact. | Old/new retention, changeset, first-seen equivalence tests. |
| Retention cohort reconciliation scans | needed but inefficient | Retention may scan/rewrite cohort files after removals. | File-backed cohort iteration plus lossless compact/index format if proven. | High disk/file-cache reduction for high-churn feeds. | Highest data-loss risk. | Migration, crash-injection, and API equivalence tests before writer changes. |
| Broad heavy phases after small updates | avoidable work | Prior SOWs found broad comparison/entity work after admitted processing. | Phase manifests and affected-feed lists. | High CPU reduction after small updates. | Stale provider-derived artifacts if affected rules are incomplete. | Fixture with one changed feed and stale-artifact assertions. |
| Pairwise comparison rescans | duplicated work | Many pair candidates can be scanned even when unchanged pair results exist. | Revision-keyed pair ledger with conservative invalidation. | High CPU reduction on normal update cycles. | Stale/wrong overlaps if ledger identity is incomplete. | Poisoned-ledger, changed-side, removed-feed, full-rebuild tests. |
| Provider parsing/preparation per run | needed but inefficient | ASN/GeoIP/bogon/critical provider work runs inside heavy phases. | Typed processors maintain prepared indexes keyed by provider/reference revision and config identity. | Medium to high CPU/I/O reduction on provider reuse. | Config/default drift must invalidate precisely. | Provider identity drift and unchanged-provider reuse tests. |
| Bogon overlap repeated across ASN providers | duplicated work | Prior memory spec notes provider-independent bogon overlap can be reused. | Compute once per feed per bogon revision, reuse for ASN providers. | Medium CPU reduction in ASN phase. | Public `bogon_ips`, `unknown_ips`, `by_asn` semantics must stay unchanged. | Multi-provider ASN fixture equivalence tests. |
| Byte-identical public artifact rewrites | avoidable work | Prior SOWs found no-op writes and publish-stage pressure. | Streaming byte comparison and manifest-based freshness. | Medium I/O/file-cache reduction. | Mtime contracts must remain deliberate for integrity. | Same-byte publish tests checking mtime and generated-file ledger. |
| Entity sidecar/public entity rebuild mixing | avoidable broad work | Current entity refresh can coalesce and rebuild broad country/ASN surfaces. | Split comparison facts from public formatting; patch only affected entity artifacts. | High CPU/I/O reduction after feed updates. | Missing affected entity can leave stale public detail/index pages. | Feed delta fixture with affected/unaffected entity assertions. |
| Insights reading public artifacts | duplicated discovery | Insight series readers prefer public files and fall back to ledgers. | Insights consume processor/comparison manifests directly. | Medium I/O reduction and cleaner correctness. | Public artifact absence must not block insight generation if internal state exists. | Missing public artifact fixture, insight output equivalence. |
| Startup integrity broad scans | avoidable startup work | Integrity currently walks live feeds and public artifacts. | Manifest-based repair queue, deferred bounded repair. | Startup latency and I/O reduction. | Missed stale artifact if manifests are wrong. | Startup fixture with stale/missing artifacts and bounded repair assertions. |
| Raw mirror/public artifact generation in same broad publish | possible avoidable work | Publication paths may handle many file families together. | Phase-specific public artifact manifests and affected sets. | Medium I/O reduction. | Public artifact consistency across indexes must be maintained. | Partial affected publish fixture and route-read tests. |

### Work That Must Not Be Optimized Away

- Historical feed evidence.
- Exact current first-seen semantics, unless the user later approves a semantic
  change.
- Removed-life retention evidence required for public/operator interpretation.
- Public raw text compatibility.
- Zero-overlap absence semantics in comparison artifacts.
- Provider drift rebuilds when the provider/config identity affecting public
  output changes.
- Integrity and repair paths, although they must be bounded and manifest-driven.

### Candidate Caches And Manifests

- Downloader canonical revision manifest.
- Per-feed set summary cache:
  - content hash
  - unique IP count
  - range count
  - min/max
  - prefix occupancy
  - IP family
- Binary canonical set as downstream operational input.
- Feed processor manifest keyed by downloader revision.
- Retention index or compacted current-membership state.
- Processor-owned provider/reference prepared-index manifest keyed by provider
  revision and config identity.
- Pairwise comparison ledger keyed by both feed revision identities.
- Feed affected-list manifest keyed by comparison wave.
- Insight manifest keyed by processor/comparison dependency revisions.
- Public artifact manifest keyed by insight and formatting/config identity.

### Candidate File-Format Changes

No file-format change is approved yet. Candidate changes must be evaluated with
migration tests first.

Candidates:

- Add binary canonical downloader output beside canonical text.
- Add phase manifests with revision ids and dependency ids.
- Add or replace comparison pair ledger format.
- Add processor-owned prepared provider/reference index files.
- Add lossless retention compact/index format.
- Add public artifact manifest format.

Rules for all format changes:

- Existing legacy files remain readable.
- New writers do not delete old history.
- Migration is additive first.
- Equivalence tests compare old and new API/artifact answers.
- Crash injection tests cover every migration transition.
- Rollback or read-compatibility exists until the new format is proven in
  production.

## History Preservation And Migration Test Strategy

Required test families before any historical format writer changes:

1. Legacy layout inventory tests
   - Build fixtures that contain canonical text bodies, binary latest snapshots,
     history ledgers, changesets, retention cohorts, retention CSV/JSON, and
     legacy suffix variants.
   - Prove the inventory detects every file family without deleting unknown
     files.

2. Read-equivalence tests
   - Read legacy state and new state.
   - Compare public API answers, first-seen answers, retention summaries,
     changesets, history tails, and feed counts.

3. Migration-equivalence tests
   - Migrate legacy fixture state into the candidate new format.
   - Verify every user-visible and operator-visible answer is identical unless a
     user-approved product decision says otherwise.

4. Crash-injection tests
   - Stop after temp write.
   - Stop after partial staged output.
   - Stop after one ledger append.
   - Stop before completion marker.
   - Stop after completion marker.
   - Restart and verify no data loss, no duplicate ledger effects, no stale
     publication, and eventual convergence.

5. Idempotency tests
   - Run every phase twice on the same durable input revision.
   - Verify outputs and ledgers do not double-count.

6. Non-destructive migration tests
   - Unknown files and legacy files remain present.
   - Failed migration preserves the previous committed state.

7. Performance regression tests
   - Large synthetic feed.
   - High-churn retention fixture.
   - Many small feed updates.
   - Provider-change fan-out fixture.
   - Measure work size, operation counts, bytes read/written, and elapsed rate.

## Consolidation

### Consolidated SOWs

The following SOWs are consolidated into this SOW:

- `.agents/sow/done/SOW-0097-20260601-ingest-cpu-concurrency-limits.md`
  - Remaining concern: production resource boundaries.
  - New home: phase resource policy, coordinator admission, and per-phase worker
    limits in this SOW.
- `.agents/sow/done/SOW-0103-20260613-cpu-memory-optimization-without-functional-change.md`
  - Remaining concern: CPU, memory, disk, and duplicate-work optimization.
  - New home: paper performance analysis and later phase-specific
    implementation slices in this SOW.
- `.agents/sow/done/SOW-0104-20260614-retention-storage-compaction-design.md`
  - Remaining concern: retention disk/file-cache growth and possible format
    change.
  - New home: feed-processor retention design and migration tests in this SOW.
- `.agents/sow/done/SOW-0105-20260615-production-unresponsiveness-diagnosis.md`
  - Remaining concern: production unresponsiveness and backend resource model.
  - New home: engine-core redesign and phase crash/resource model in this SOW.

### Referenced But Not Consolidated

- `.agents/sow/pending/SOW-0095-20260601-application-review-from-docs-sync.md`
  - Relevant item: `skip_comparison_if_no_updates` and no-update regeneration.
  - Reason not consolidated: SOW-0095 also covers unrelated app-review decisions
    such as API rate limiting, CLI help, MCP ASN names, install paths, and
    downloader secret handling.

## Implications And Decisions

### Decision 1 - Baseline Phase Count

Selection: five phases.

Reason:

- The user clarified that the baseline model is five phases.
- Additional entities may be added only if the gap analysis proves they are
  needed.

Implication:

- The pipeline coordinator, integrity/repair, and migration/import are treated
  as supporting infrastructure unless later analysis proves they must be
  processing phases.

Risk:

- If provider preparation is forced into the wrong phase, the design may hide a
  major resource hotspot. The gap analysis must explicitly resolve provider
  preparation.

### Decision 2 - Crash Recovery As A Phase Contract

Selection: every phase must have durable inputs, staged outputs, completion
markers, and restart recovery.

Reason:

- The user explicitly required the whole pipeline to continue after any crash
  without data loss.

Implication:

- In-memory-only phase handoff is not allowed for durable work.
- Admin UI progress can be in memory, but the work item itself must survive.

Risk:

- More durable markers can add complexity. The design must keep them simple and
  tied to the fixed five-phase pipeline, not a general dependency graph.

### Decision 3 - History Preservation

Selection: lossless and non-destructive.

Reason:

- The user declared the existing ten years of feed history as project legacy.

Implication:

- Format changes are allowed only with additive migration, read compatibility,
  equivalence tests, and crash tests.

Risk:

- Retention compaction may have attractive disk savings but is rejected unless
  exact semantics and safe migration are proven.

### Decision 4 - Processor Ownership

Selection: processor is a family of typed processors.

Processor families:

- normal feeds;
- ASN providers;
- GeoIP providers;
- bogon/reference inputs;
- critical-infrastructure reference inputs.

Reason:

- The fixed flow is downloader -> processor -> comparisons -> insights ->
  artifacts.
- Downloader produces updated artifacts.
- Processor atomically processes those artifacts and maintains whatever
  self-contained local state is required by the rest of the pipeline.
- Provider indexes are not a downloader concern and should not be pushed into
  comparisons as raw preparation work.

Implication:

- Comparisons consume processor-prepared provider/reference state.
- Processor never fans out effects to other feeds by itself.

Risk:

- Processor must stay self-contained. It must not start doing enrichment or
  comparison against external feeds/providers for normal-feed processing.

### Decision 5 - Comparison Fan-Out Contract

Selection: comparisons expand the affected-feed set.

Reason:

- Downloader and processor mostly preserve cardinality: due feeds become updated
  artifacts, and no-update feeds stop.
- Comparisons can turn a smaller updated set into a larger affected set because
  overlaps and provider-derived facts affect peers and pages beyond the updated
  feeds.

Example:

```text
10 input feeds -> downloader updates 9
9 processor inputs -> 9 processed outputs
9 comparison inputs -> 100 affected feeds
100 insight inputs -> 90 changed insight outputs
100 artifact inputs -> 100 refreshed publication targets
```

Implication:

- Insights and artifacts operate on the affected set, not only on the originally
  updated feeds.
- Comparisons own the affected-feed manifest.

Risk:

- If comparisons under-report affected feeds, insights/artifacts become stale.
- If comparisons over-report affected feeds, the product wastes CPU/I/O but
  remains correct. The design should prefer correctness first, then narrow the
  affected set with evidence.

### Decision 6 - No-Update Handling

Selection: if downloader says no-update, there is no downstream work for that
feed.

Reason:

- A feed cannot change randomly; all ordinary feed changes enter through the
  downloader.

Implication:

- No-update does not enter processor, comparisons, insights, or artifacts.
- Explicit integrity/admin repair remains separate and must state the phase it
  is repairing.

Risk:

- Repair/rebuild work must not be hidden behind no-update semantics; stale or
  missing existing artifacts need explicit repair work items.

### Decision 7 - File-Level Artifact Ownership

Selection: ownership is at file/artifact-family level, not directory level.

Reason:

- The current code and historical disk layout were not organized by phase.
- Existing directories may contain files maintained by different phases.
- Valuable legacy data must not be moved, rewritten, or discarded merely to make
  directory ownership look clean.

Implication:

- Every file or artifact family must have one owner/producer and one or more
  consumers/users.
- Directory organization can be revisited only after all phases have a complete
  file-level ownership map.
- Mixed-ownership directories are acceptable during the design and migration.

Risk:

- A file-level map is more detailed work than a directory-level map, but it is
  safer for the current codebase and protects legacy history.

### Decision 8 - Durable Phase Handover

Selection: every phase handover is disk-durable. In-memory handover is only an
optimization.

Reason:

- Direct phase-current writes can leave half-baked state after a crash or
  abnormal termination.
- A restart must know exactly which phase outputs are complete and which
  downstream work is still pending.
- Inferring pending work from partially updated data files is fragile and is the
  class of problem this redesign must remove.

Implication:

- Each phase must have a durable completion marker or manifest.
- Each transition to the next phase must have a durable handover record.
- Startup must process existing durable handovers before admitting a new
  synchronized processor/comparisons/insights/artifacts wave.
- Downloader may remain asynchronous, but its completed revisions cannot enter a
  new synchronized pipeline wave while older handovers remain pending.
- A durable handover is not a per-phase `ready` dataset. It is a persisted inbox
  contract that points to completed source-phase outputs.

Risk:

- Handover records add state that must be tested for idempotency.
- The implementation must handle crashes before handover write, after handover
  write, during downstream work, after downstream completion, and before
  handover acknowledgement.

### Decision 9 - Non-Blocking Runtime Snapshots

Selection: active in-memory feed/catalog state is immutable to readers and
updated by copy-on-write snapshot replacement. Processing work must not hold
long-lived locks on the global feed list or on a specific active feed.

Reason:

- The public website and APIs must remain responsive while ingestion/processing
  runs.
- A feed currently being processed is still part of the public product and must
  remain readable from its previous official state until the replacement is
  ready.
- Long-lived locks around parsing, comparisons, retention, disk I/O, or
  publication would turn ingestion into user-visible downtime or latency spikes.

Implication:

- Readers acquire an immutable snapshot reference and release it after the
  request/status operation.
- Writers build replacement state privately, validate it, and swap it into place
  with only a near-instant pointer/snapshot swap.
- Old snapshots remain valid until no active reader can observe them.
- This applies regardless of whether the final implementation language is Go,
  Rust, or a mixed system.

Risk:

- Copy-on-write can increase peak memory while a replacement snapshot is being
  built.
- The design must minimize replacement scope and use file-backed/mmap artifacts
  where possible so snapshot isolation does not double large heap objects.
- Tests must prove readers cannot observe partially mutated active state.

## Plan

1. Complete phase-boundary map.
2. Complete responsibility gap analysis.
3. Complete paper performance analysis.
4. Present any required design decisions with evidence and recommendations.
5. Update specs after the design is accepted.
6. Run external reviewers against the full SOW and specs.
7. Implement only after the design and reviewer findings are resolved.

## Execution Log

### 2026-06-20

- Created this SOW as the single controlling backend redesign SOW.
- Recorded user requirements for five phases, lossless history preservation,
  crash-safe restart, gap analysis, and paper performance analysis.
- Closed and moved SOW-0097, SOW-0103, SOW-0104, and SOW-0105 under
  `.agents/sow/done/` as consolidated/superseded, not completed.
- Ran `.agents/sow/audit.sh`; SOW-0106 and the moved consolidated SOWs passed
  status/directory checks. Remaining audit warnings are pre-existing SOW-0016
  helper-file hygiene issues.
- Recorded user decisions that processor is a typed processor family, provider
  prepared indexes belong to processors, comparisons fan out affected feeds,
  homepage/global outputs belong to artifacts, no-update feeds stop after
  downloader, and retention representation is processor-internal as long as
  history is not lost.
- Added phase-domain and artifact-classification rules so remaining sidecar and
  global-artifact questions can be resolved by domain ownership instead of
  implementation naming.
- Added the phase-by-phase artifact audit protocol and completed the first
  downloader-only artifact audit pass, including producer/consumer ownership,
  candidate indexes, elimination/reassignment candidates, and downloader exit
  questions.
- Recorded the durable phase handover decision: in-memory handoff is only an
  optimization, disk handover records are the source of truth, and startup must
  drain existing handovers before admitting a new synchronized pipeline wave.
- Continued the phase-by-phase artifact audit through processor, comparisons,
  insights, and public artifacts, including current producer evidence,
  proposed owner/consumer mapping, candidate indexes, and cleanup candidates.
- Resolved provider preparation as processor-domain work and resolved entity
  sidecar ownership by distinguishing comparison fact sidecars from rendered
  public artifacts.
- Added the performance and continuity design: the coordinator serializes
  phases 2-5, continuity is achieved by durable staged work, recovery, commit
  markers, and handovers, and each artifact family must choose an explicit
  commit pattern.

### 2026-06-21

- Recorded the non-blocking runtime snapshot requirement: processing must not
  hold long-lived locks on the global feed list or active feeds; active
  in-memory feed/catalog state is immutable to readers and updated by
  copy-on-write snapshot replacement.
- Added explicit validation requirements proving public/admin/API readers are
  not blocked by long-running processing and can observe only old or new
  official snapshots, never partially mutated active state.

## Validation

Acceptance criteria evidence:

- Phase map: recorded under `## Phase Model Draft`.
- Phase domain model: recorded under `## Phase Domain Model v1`.
- Artifact classification: recorded under `## Artifact Classification v1`.
- Phase artifact audit protocol: recorded under
  `## Phase-By-Phase Artifact Audit Protocol`.
- Downloader artifact audit: recorded under
  `#### Downloader Artifact Audit v1`.
- Processor artifact audit: recorded under
  `#### Processor Artifact Audit v1`.
- Comparisons artifact audit: recorded under
  `#### Comparisons Artifact Audit v1`.
- Insights artifact audit: recorded under
  `#### Insights Artifact Audit v1`.
- Public artifacts audit: recorded under
  `#### Public Artifacts Artifact Audit v1`.
- Durable handover contract: recorded under
  `## Durable Phase Handover Contract`.
- Performance and continuity design: recorded under
  `## Performance And Continuity Design v1`.
- Non-blocking runtime snapshot contract: recorded under
  `### Non-Blocking Runtime Snapshot Contract`.
- Non-blocking snapshot validation: recorded under
  `### Non-Blocking Snapshot Validation Requirements`.
- Gap analysis: recorded under `## Gap Analysis v1`.
- Paper performance analysis: recorded under `## Paper Performance Analysis v1`.
- History preservation and migration test strategy: recorded under
  `## History Preservation And Migration Test Strategy`.
- Remaining review/design work is not implementation work: downloader exit
  questions, homepage/global affected-input rules, and phase-specific artifact
  elimination/index/format choices still need acceptance before specs or code
  change.

Tests or equivalent validation:

- `.agents/sow/audit.sh`
  - Result for SOW-0106: OK in `current/` with `Status: in-progress`.
  - Result for consolidated SOWs: OK in `done/` with `Status: closed`.
  - Overall repo SOW verdict remains partial because of unrelated current SOW
    hygiene issues: SOW-0016 helper files missing status/gates, SOW-0116
    status/directory mismatch, and the existing root `TODO-GATED.md` marker.
  - No code has been changed.

Real-use evidence:

- Pending. Existing live production-candidate evidence remains preserved in the
  consolidated SOWs.

Reviewer findings:

- Pending. External reviewers should run after the paper design is complete
  enough to review as a whole.

Same-failure scan:

- Pending after the gap analysis is complete.

Sensitive data gate:

- This SOW contains no secrets, credentials, bearer tokens, private endpoints,
  customer names, community member names, personal data, raw feed payloads, or
  non-private customer-identifying IP addresses.

Artifact maintenance gate:

- AGENTS.md: no update made in this paper-design creation pass.
- Runtime project skills: no update made in this paper-design creation pass.
- Specs: not yet updated; specs should be updated only after the SOW design is
  accepted.
- End-user/operator docs: no update made.
- End-user/operator skills: no update made.
- SOW lifecycle: SOW-0097, SOW-0103, SOW-0104, and SOW-0105 were closed under
  `.agents/sow/done/` as consolidated into this SOW.

Specs update:

- Pending after design acceptance.

Project skills update:

- Pending only if durable process lessons emerge.

End-user/operator docs update:

- Pending only if file layout, migration, repair, or operation behavior changes.

End-user/operator skills update:

- None expected yet.

Lessons:

- Pending.

Follow-up mapping:

- Pending until paper design is completed.

## Outcome

Pending.

## Lessons Extracted

Pending.

## Followup

None yet.

## Regression Log

None yet.
