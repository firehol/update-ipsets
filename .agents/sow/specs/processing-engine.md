# Processing Engine Contract

## Status

This document is normative.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** describe the required
behavior of the processing-engine subsystem.

## Purpose

The processing engine is the isolated subsystem responsible for turning local
canonical feed bodies and supporting provider data into the published artifacts
required by the website, mirrors, and operator surfaces.

It exists to answer four classes of product questions:

1. what changed in this feed over time
2. how does this feed compare with provider datasets and peer feeds
3. which deterministic insights follow from those facts
4. which published artifacts need updating because this feed changed or because
   supporting datasets changed

## Boundary

The processing engine MUST own:

- claiming staged canonical feed bodies for execution
- consuming processing or committed local feed bodies
- feed-local analysis/finalize work over canonical feeds
- feed-local history, retention, and change-rate artifacts
- provider enrichment over public feeds
- pairwise comparison updates
- deterministic insight generation
- publication of feed-facing public artifacts

The processing engine MUST NOT own:

- remote acquisition
- raw-source parsing, cleanup, or normalization
- merge or history composition logic
- upstream freshness detection
- downloader retry/backoff policy
- artifact-parent acquisition

## Accepted inputs

The processing engine MUST accept only local downloader-produced state.

Ordinary inputs are:

- a staged feed body after the engine claims it into `.processing`
- otherwise a committed feed body during explicit reprocess or local repair

The engine MUST NOT fetch or compose upstream source material by itself.
The engine MUST NOT be responsible for parsing non-canonical upstream formats.

## Supported feed families

The engine processes public feeds regardless of how the downloader produced
their local input.

This includes:

- plain feeds
- history derivatives
- merges
- artifact-backed child feeds

Provider datasets are not processed as public feeds. Instead, updated provider
datasets change how the engine enriches or reprocesses public feeds.

Artifact parents are not public feeds and are not processed as feed rows by the
engine.

Provider-backed enrichment families that the engine must understand include at
least:

- ASN
- geography
- bogons
- critical-infrastructure reference feeds

## Processing scheduler contract

The processing engine MUST have its own consumer loop and its own live states:

1. waiting to be processed
2. being processed now

The processing engine is a consumer of admitted work, not an autonomous
selector of new acquisition work.

Automatic source freshness decisions belong to the downloader.

## Admissions into processing

The engine may receive work directly only from:

- downloader admission
- provider-update reprocess waves
- explicit admin reprocess
- integrity-triggered local rebuild/reprocess
- restart recovery of already durable staged input

No other ordinary runtime condition may admit processing work directly.

## Core pipeline

For each admitted feed, the engine MUST conceptually execute these stages:

1. claim any staged `.{ip,net}set.new` body for that feed by renaming it to
   `.{ip,net}set.processing`, or use an existing committed body for explicit
   local reprocess work
2. load the admitted canonical feed body
3. analyze that canonical feed for engine-local state
4. finalize the successful normal feed body into the committed canonical feed
   body and latest binary set
5. update feed-local historical and retention artifacts
6. update feed-local change-rate and rotation statistics
7. run required downstream enrichment, comparison, and insight work
8. stage the resulting public artifacts and assign their logical mtimes
9. publish the staged public artifacts and save the updated cache state

The precise end-to-end queue choreography is owned by [pipeline.md](pipeline.md).
This document owns the engine-local contract.

## Feed-local processing contract

Feed-local processing MUST produce or maintain at least:

- feed-local metadata
- bounded history artifacts
- retention summaries
- feed-local change and rotation measurements

The internal history ledger remains append-only evidence. The engine MAY avoid
rescanning that ledger for same-timestamp observations only when the newly
observed entries and unique-IP counts exactly match the cached last observation.
If a same-timestamp observation changes counts, the engine MUST preserve the
existing correction behavior and reload or recompute the effective ledger state
so min/max, version, and public history-tail facts remain correct.

For retention/state ownership, the engine MUST keep distinct:

- current-membership retention cohorts that preserve the start time of the
  current contiguous listing interval for currently listed IPs
- removed-life ledgers that preserve how long removed IP cohorts had remained
  listed when they were removed

The engine MUST NOT treat downloader-owned `data/history/...` snapshots as a
substitute for those retention facts.

Retention diffing SHOULD use file-backed or iterator-based reads for the
previous committed latest set when that binary set is available. The engine MAY
precompute the retention diff before replacing the committed latest set, but it
MUST NOT write retention artifacts until the canonical feed body and latest
binary set have been finalized successfully. The diff implementation SHOULD
materialize only the new currently-listed cohort set that must be persisted;
removed counts SHOULD be counted through bounded iteration.

Latest binary set and retention cohort writers SHOULD write through bounded
buffers into their atomic destination path. They SHOULD NOT allocate a second
whole-file binary payload while an in-memory `IPSet` is already live for the
same feed.

When reconciling existing retention cohorts after removals, the engine SHOULD
open binary cohort files through file-backed range sources. It SHOULD
materialize only the still-listed cohort that must be rewritten and count
removed IPs without building a separate removed set. The comparison phase
SHOULD send bounded batches of cohort sources to `pkg/iprange` source-pair
comparison APIs instead of invoking one engine-local comparison per cohort; the
batch bound MUST avoid opening the full retention history at once.

If the input is valid but empty, the engine MUST still produce the empty-result
publication defined by the product contract.

## Global enrichment and comparison contract

For every admitted feed, the engine MUST update the published facts that depend
on:

- ASN providers
- geolocation providers
- bogon providers
- critical-infrastructure reference feeds and provider-set identity
- peer-feed comparison

The engine MUST also keep peer-facing comparison artifacts current.

Live lookup support state, such as the public IP-search ASN lookup cache, MUST
respect provider refresh and reload boundaries. Successful reloads and
provider-file replacements must retire old lookup entries so subsequent lookups
open current provider data. In-flight lookups or builders may finish with the
database they already acquired, but retired databases must be closed after that
work releases them.

Pairwise comparison rule:

- when feed `A` changes or is reprocessed in a way that can affect comparison
  facts, the engine MUST refresh the comparison relationship between `A` and
  every relevant peer feed
- when two feeds have a non-zero overlap, the resulting pairwise fact MUST be
  written so both sides expose the current relationship
- when two feeds have zero overlap, the writer MUST omit the public row and
  remove any stale existing row for that peer during incremental merge; absence
  is the public representation of no overlap
- pairwise-comparison-driven insights MUST follow the same freshness rule
- the `related` flag MUST be based on shared positive lineage. Retention parents
  and additive merge inputs are positive lineage; subtractive merge inputs are
  dependencies, not positive lineage, and MUST NOT by themselves make two feeds
  related.

The implementation MAY skip an expensive pairwise overlap scan only when it has
an exact proof that the result is unchanged or zero, such as identical
normalized range content, disjoint min/max bounds, or disjoint occupied address
prefixes. Any prefix, bound, or fingerprint shortcut MUST be conservative:
uncertainty means run the full overlap count and publish the exact result.
The same rule applies to provider-reference overlap scans such as bogon and
critical-infrastructure overlaps: exact zero-overlap proofs may skip scans,
but uncertain pairs must run the existing exact overlap logic.

ASN comparison artifacts MAY compute the provider-independent bogon overlap
once per feed when several ASN providers are evaluated in the same run. Each
provider MUST still count ASN attribution over the feed's non-bogon residual
through its own database, so `bogon_ips`, `unknown_ips`, `attributed_ips`, and
`by_asn` remain equivalent to a full per-provider `CountFeedWithBogons` pass.

This ensures public comparisons remain "current now", not merely "current the
last time each feed changed independently".

`relevant peer feed` means:

- every other operationally enabled public feed whose pairwise comparison is
  supposed to appear on either side's published comparison artifacts
- hidden/non-public feeds, provider datasets, and artifact parents are never
  peer feeds in published public comparison artifacts

Feed-scoped comparison and insight publication MUST follow the same
public-feed eligibility boundary as the rest of the published website artifact
tree. Non-public feed identities MUST NOT leak into public comparison rows or
public insight inputs merely because local files exist for them.

## Provider-triggered reprocessing

When a provider dataset updates successfully, the engine MUST be able to
reprocess the relevant public feeds even when their own feed bodies did not
change.

This is required because:

- ASN classification may change
- GEO classification may change
- bogon classification may change
- critical-infrastructure provider-set membership or reference content may change
- insights and peer-facing derived artifacts may need regeneration

## Deterministic insight families

The engine MUST be able to publish deterministic insights derived from the
facts it computes.

At minimum, the insight contract includes families equivalent to:

- feed-local lifecycle and churn findings
- provider-enrichment findings such as ASN, geography, or bogon concentration
- pairwise-comparison findings such as overlap, uniqueness, or inclusion

The exact public insight catalog MAY evolve, but every published insight MUST
remain:

- deterministic from committed local artifacts
- methodologically documented
- reproducible from the same local facts

## Reprocess contract

`reprocess` means exact replay of the engine over existing local input.

It MUST:

- use an existing `.{ip,net}set.processing` body if one exists
- otherwise claim a staged `.{ip,net}set.new` body if one exists
- otherwise use the committed feed body
- not trigger a fresh downloader acquisition by itself

The engine MUST accept reprocess even when the feed body is semantically the
same as before, because supporting datasets, output formats, or implementation
contracts may have changed.

## Processing order

The engine MUST obey the batch ordering defined by [pipeline.md](pipeline.md):

1. normal feeds
2. history derivatives
3. merges ordered by configured dependency count

This is an engine execution rule, not a substitute for downloader composition.

## Processing outcomes and exceptions

The engine MUST expose a structured result model.

At minimum:

- `ok`
- exception enum for failure classes

The exception model MUST distinguish at least:

### `invalid_input`

- the engine was asked to process something that is not a valid feed target

### `missing_input`

- no staged or committed local feed body exists for the requested processing
  action

### `parse_failed`

- the local feed body exists but cannot be parsed into the engine's canonical
  set representation for downstream analysis

### `finalize_failed`

- feed-local finalization failed while writing the committed canonical feed
  body, latest binary set, kernel set, or feed-local history/metadata required
  before downstream publication

### `retention_failed`

- feed-local historical/retention updates failed after the main canonical-feed
  analysis succeeded

### `cancelled`

- processing work was admitted but did not complete because the run context was
  cancelled before or during execution

## Operator-visible engine status

Operator surfaces MAY present simpler labels, but they MUST preserve the
meaning of at least:

- running
- processing
- updated
- empty
- disabled
- failed with explicit exception class or message

Supporting-provider and heavy-phase surfaces MUST preserve the meaning of at
least:

- config_error
- extract_failed
- open_failed
- unavailable
- stale

The engine MUST NOT silently drop a processing request once it has been
admitted.

If a newer processing request arrives while the feed is already active, the
engine runtime MUST defer or coalesce it so at least one post-current-run
processing pass still occurs.

## Failure handling

When engine processing fails after `.{ip,net}set.processing` already exists,
that exact local feed body MUST remain available for later debugging or replay.

Engine failure MUST NOT require a fresh upstream fetch in order to retry local
processing of the same input.

Engine exceptions are severe runtime faults first.

Integrity reports them later only if they leave settled local inconsistency
behind.

## Configuration directives that affect the engine

The exact syntax belongs to [config.md](config.md). This document defines which
configuration concerns the engine must honor.

### Global runtime concerns

The engine MUST honor configuration for at least:

- processing cadence
- processing concurrency
- heavy-phase concurrency
- publication/output directories
- optional integration/application of produced sets

### Per-feed concerns

The engine MUST honor per-feed configuration for at least:

- output family (`ipset`, `netset`, or equivalent canonical output choice)
- IP family restrictions
- category and visibility metadata used in published artifacts
- hidden flags and other publication metadata where they affect publication or
  operator surfaces
- legal/publication metadata
- dependency relationships needed for comparison and insight publication
- provider-role configuration that determines which supporting datasets enrich
  which public outputs

## Admin visibility and controls

The admin UI and APIs MUST expose the processing engine as a first-class
operational subsystem.

Operators MUST be able to observe at least:

- waiting to be processed
- being processed now
- queue wait age
- current processing phase or reason
- current processing batch membership, including total, completed, active, and
  pending feeds
- current run phase plan, including phase order, current phase position, total
  phases, and whether the plan is final or still tentative
- active per-feed, per-phase, and per-operation progress when work is running
- last processing result
- whether enough local input exists for reprocess

Operators MUST be able to trigger at least:

- feed-level reprocess
- global run due work now
- integrity recovery that results in local reprocess when appropriate

Dedicated operator APIs SHOULD include equivalents of:

- feed reprocess
- global trigger of due processing work
- integrity recovery actions that map to local reprocess

## Engine-controlled fields per feed

The engine MUST be authoritative for at least:

- last successful local publication time
- processing start time for the current/last run
- last run reason
- last processing duration
- active operation name, phase, feed name when applicable, stage, unit of work,
  completed work, total work, completion percentage, elapsed time, and rate
  per second
- primary output size / unique-IP measurements
- retention measurements
- change-rate and rotation measurements
- engine-owned published artifact freshness

The engine also writes the latest engine terminal status and message when
processing-stage work is the latest completed action.

Structured processing diagnostics MUST define the unit of work for every
reported progress surface. Logs and admin status MUST NOT expose a bare counter
without enough context for an operator to know whether the count represents
feeds, files, operations, IPs, entries, or bytes. For completed runs and phases,
diagnostics SHOULD expose phase-scoped operation counts, phase work size,
completion percentage when bounded, and rate.

Long-running bounded phase work MUST be represented as active operations even
when the work is not tied to one feed. Examples include provider loads,
feed/provider comparison fan-outs, metadata/index generation, entity sidecar
fan-outs, and publish/copy loops. Such phase-level active operations MUST use
the current engine phase and MUST report the same unit, current, total,
completion, elapsed, and rate fields as feed-level active operations.

Source processing MUST NOT expose only one broad "process feed" progress
wrapper when material subwork is still running inside it. Long-running source
subwork SHOULD expose its own active operation, including at least canonical
body parsing, hostname resolution when present, previous-latest diffing,
finalization, retention update, and rotation-stat refresh. Parser progress
SHOULD report bytes and line/range/hostname counters so operators can
distinguish a large local input parse from later diff, retention, or finalize
work.

At minimum, engine-controlled persisted fields MUST include equivalents of:

- `StartedDate`
- `ProcessedDate`
- `Entries`
- `UniqueIPs`
- `EntriesMin`
- `EntriesMax`
- `IPsMin`
- `IPsMax`
- `AverageUpdateMins`
- `MinUpdateMins`
- `MaxUpdateMins`
- `HistoryTotalGapSecs`
- `HistoryMinGapSecs`
- `HistoryMaxGapSecs`
- `RotationMedianPct`
- `RotationP75Pct`
- `RotationSamples`
- `ChangeRatioMedianPct`
- `ChangeRatioP75Pct`
- `ChangeRatioSamples`
- `UniqueSharePct`
- `UniqueShareSamples`
- engine-stage values of `LastStatus`
- engine-stage values of `LastError`
- `LastRunReason`
- `LastProcessingMS`

## Engine-owned outputs

The engine MUST produce or maintain the public artifacts required by the
website and other consumers.

These include at least:

- per-feed metadata
- bounded history artifacts
- changeset/retention summaries
- pairwise comparison artifacts
- provider enrichment artifacts:
  - ASN
  - GEO
  - bogon
  - critical-infrastructure aggregate and per-provider overlap files, each tied
    to the current `provider_set_id`
- deterministic insight artifacts

Critical-infrastructure facts in the signal snapshot MUST come from the
critical-overlap aggregate artifact, not ASN composition. The snapshot MUST
carry the aggregate overlap count/share plus tier summaries so the
`infrastructure_present` insight can treat hard, soft, and contextual overlap
differently.

If the aggregate also includes critical ASN context, that section remains a
secondary UI/API signal. It MUST NOT be folded into signal-snapshot
`critical_ips` or `infrastructure_present` thresholds. Provider-context feeds
are also excluded from critical-overlap target generation.

Website-facing route and data contracts are owned by [website.md](website.md).
This document owns the fact that the engine is responsible for producing them.

## Invariants guaranteed to callers

The engine MUST guarantee:

- it can fully rebuild public feed artifacts from local downloader-produced
  input plus provider state
- reprocessing the same feed body is valid and meaningful
- peer-facing comparison artifacts stay current when one side changes
- empty valid inputs publish as empty, not as failure
- local processing failure does not require a new upstream fetch to debug or
  retry the same body

## What MUST NOT cross the boundary

The engine MUST NOT need to know:

- how upstream transport worked
- which archive format was fetched
- which extraction steps were needed before the feed body existed

Those concerns belong entirely to the downloader.
