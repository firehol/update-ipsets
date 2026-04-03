# Integrity Contract

## Status

This document is normative.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** describe the required
behavior of the product.

## Purpose

Integrity exists to answer one question:

> Does the local on-disk state still match the last successful local
> publication of each feed?

Integrity is about **local correctness after success**, not about whether an
upstream feed currently exists or is healthy.

## What integrity checks

For every feed that the product considers a public publishable feed, integrity
MUST be able to verify:

- that the committed primary output expected from the last successful local
  publication still exists
- that the expected secondary public artifacts still exist
- that those secondaries are not older than the last successful local
  publication when freshness is required
- that structured JSON secondaries are still semantically readable, not just
  present as bytes
- that pairwise comparison artifacts do not contain explicit zero-overlap rows;
  zero overlap is represented by absence, so stale `common: 0` rows are
  malformed public artifact noise

Structured JSON secondaries include at least:

- public metadata
- retention summaries
- feed comparison results
- deterministic insights
- geographic distribution payloads
- ASN enrichment payloads
- bogon enrichment payloads
- critical-infrastructure overlap payloads, for comparable IPv4 feeds when
  critical-infrastructure reference providers are configured

Critical-infrastructure aggregate and per-provider JSON payloads MUST carry the
current provider-set identity. Integrity MUST report a critical-overlap payload
as malformed when its `provider_set_id` is missing or does not match the
current configured provider-set identity. The check stays strict on purpose:
it is the last-line tripwire for any pipeline-level regression that lets the
on-disk artifacts and the runtime marker disagree.

The provider-set identity is derived only from the configured catalog
(configured critical-infrastructure providers and their per-source
configuration fingerprint, plus the configured `critical_asn_context` list
when present, plus the configured ASN provider source shape when
`critical_asn_context` is configured). It MUST NOT include materialized cache
state such as a provider's content hash, materialized entries count, unique
IPs, on-disk file path, processed-date, version counter, or any other field
that fluctuates while the pipeline is running. Per-feed overlap freshness is
enforced separately through the processing-time mtime contract on the
secondary artifacts.

Within a single pipeline run, the engine MUST capture the provider-set
identity exactly once at plan time and thread that captured value through
every artifact write and through the runtime marker write that ends the run.
The engine MUST NOT re-derive the identity from engine state for either of
those writes within the same run. This single-snapshot rule guarantees that
artifacts and the marker always agree within a run, so integrity findings in
this class are always evidence of a real regression, never of a transient
race.

Per-provider critical-overlap artifacts are expected only for configured
critical providers with a materialized latest set. The aggregate artifact is
still expected when critical providers are configured, because it records
`complete` and `missing_providers` for providers that were unavailable during
the last overlap build.

The public HTTP surface is cache-first and MUST NOT enforce the
`provider_set_id` equality contract. Public endpoints that serve
critical-overlap artifacts MUST serve any artifact that exists on disk and
passes structural validation, regardless of its `provider_set_id` value. The
public surface MUST NOT surface integrity drift as user-facing editorial
content; admin integrity is the operator-facing channel for that signal.

When the daemon is configured to serve a web artifact directory override,
startup and admin integrity checks MUST validate that served artifact tree, not
only the `web_dir` value stored in the base runtime configuration.
The same override MUST be applied to the engine runtime before scheduler,
startup integrity, and admin repair work are created, so queued repair writes
to the same published tree the public server reads.

## Reference point

Integrity MUST use the last successful local publication time as its reference
point.

Integrity MUST NOT use upstream timestamps as the primary truth for local
correctness, because upstream clocks can be missing, wrong, stale, or in the
future.

## Timestamp contract

The product has three distinct timestamp meanings:

- `source_timestamp`: when the feed body or provider input content is believed
  to have last changed, derived from upstream metadata when reliable or from
  the local successful acquisition/composition event when not
- `processing_timestamp`: when the application successfully produced and
  committed local derived outputs from a specific local input
- `wallclock_timestamp`: when the local process performed an operational action
  such as writing a runtime marker, temporary file, lock file, or operator log

For every durable file that participates in integrity, the application MUST
deliberately assign the file mtime to the timestamp defined for that file
family. Accidental filesystem write time is a bug for integrity-participating
files.

Integrity MAY rely on file mtimes only for file families whose writers preserve
this contract. If a writer cannot assign the required mtime, the integrity
check MUST use another committed freshness field or report a contract violation
instead of accepting accidental wall-clock mtimes as truth.

The default mtime contract is:

- committed canonical feed bodies and raw source bodies use
  `source_timestamp`
- public feed metadata, comparisons, retention summaries, insights, and
  provider-enrichment artifacts use `processing_timestamp`
- critical-infrastructure aggregate and per-provider overlap artifacts use the
  target feed's `processing_timestamp` and must also carry the current
  `provider_set_id`
- private per-feed entity sidecars use the latest logical input timestamp they
  cover, normally the maximum of the feed's latest-set source timestamp and the
  consumed geo/ASN/provider artifact processing timestamps
- public country/ASN entity payloads and indexes use the processing timestamp
  of the entity composition they publish
- runtime markers, lock files, scratch directories, in-flight temporary files,
  logs, and purely observational telemetry files MAY use `wallclock_timestamp`
  because they are not committed feed-publication truth

Metadata-only freshness updates are allowed when content is semantically
unchanged, but they MUST set the mtime to the logical timestamp being covered,
not to the local wall clock.

Integrity regressions SHOULD be captured as table-driven pipeline scenarios.
Each scenario step mutates mocked source/provider input, advances a logical
timestamp, runs the normal scheduler-style update path, settles queued entity
refresh work, and then runs both feed-output and entity-artifact integrity.
Every discovered pipeline timestamp bug SHOULD become a scenario step or case,
so tests fail before operators see stale integrity findings at runtime.

### Entity-artifact reference model

Country/ASN reference artifacts are global published outputs, not feeds.

When entity-reference publishing is enabled, integrity MUST evaluate those
artifacts from hybrid local inputs, not from upstream/provider timestamps
alone.

At minimum:

- `lib/entities/feeds/{feed}.json` MUST be newer than the per-feed published
  geo/ASN inputs and local provider facts it is derived from
- per-feed ASN payloads depend on bogon providers because the ASN writer splits
  unmatched feed IPs into `bogon_ips` and `unknown_ips`; therefore any ordinary
  entity feed-sidecar writer or repair path that derives freshness from ASN
  payloads MUST fan out when configured bogon providers change, even when the
  sidecar JSON remains byte-identical
- `lib/entities/feeds-pending/{feed}.json`, when present, is an in-flight
  replacement produced by a processing run and awaiting background entity
  patching; it MUST NOT be treated as the committed sidecar for public
  country/ASN pages until promoted
- `lib/entities/countries/{CODE}.json` and `lib/entities/asns/{ASN}.json`
  MUST exist and parse for every country/ASN currently referenced by committed
  private per-feed sidecars; their mtimes MUST NOT be used alone as proof of
  staleness against newer feed sidecars, because ordinary surgical refreshes
  intentionally skip unchanged actor rewrites
- `web/countries/*.json` and `web/asns/*.json` MUST be newer than the
  corresponding private entity sidecars they publish

When a country/ASN private sidecar and its public payload are rewritten or
freshness-touched together, both files MUST receive the same logical mtime: the
latest committed per-feed entity sidecar timestamp among the feeds contributing
to that country or ASN. This prevents private/public publish ordering from
creating false `detail_public_stale` findings.

If a feed's entity sidecar is semantically unchanged after regenerated local
geo/ASN inputs, the engine SHOULD update freshness metadata on the committed
feed sidecar instead of rewriting identical JSON. Integrity MAY use that
feed-sidecar freshness metadata for targeted feed-sidecar repair, but it MUST
NOT cascade it into broad country/ASN actor repairs unless a concrete missing,
malformed, or semantically stale actor payload is detected.

When a feed's latest local set carries a source timestamp that is ahead of the
daemon wall clock, feed-sidecar mtime alone is not enough to prove staleness.
If the committed sidecar records the same source timestamp in its feed metadata,
integrity MUST treat that latest-set dependency as covered.

Remote provider timestamps MAY matter only through the committed local provider
files that the engine actually consumed.

## Scope

Integrity applies to public feed outputs and their public secondaries.

Integrity does not exist to restate normal queue state or temporary in-flight
conditions.

Admin integrity surfaces MUST NOT present settled findings while the pipeline is
actively mutating the file families they check. Feed-output integrity reports
`in_progress` while an engine run is active. Entity-artifact integrity reports
`in_progress` while either an engine run is active or entity background work is
active, because feed processing can advance local feed inputs before the
coalesced entity refresh task has published matching sidecars and public
payloads.

Integrity is also not the catch-all home for unexpected processing exceptions.
Those are runtime severe faults first; integrity reports them later only if
they leave settled local inconsistency behind.

Entity-artifact integrity is a separate operator surface from feed-output
integrity, because countries and ASNs are global outputs rather than feeds.

Startup entity-artifact integrity repair MUST NOT automatically execute broad
detail repairs when existing public country/ASN artifacts are usable. Broad
startup findings SHOULD be surfaced to operators and left for bounded ordinary
refreshes or explicit repair, unless the artifact tree is missing,
version-incompatible, or otherwise unusable.

The product MUST keep an explicit full entity rebuild path for repair and
operator safety. That full rebuild path MUST remain separate from the ordinary
incremental feed-update refresh path.

## Integrity finding classes

Integrity MUST distinguish at least these finding classes:

### Missing primary output

The committed main local output expected from the last successful publication is
gone.

### Missing secondary output

A required public secondary file is gone.

### Stale secondary output

A secondary exists but is older than the last successful local publication in a
way that proves the pipeline did not finish correctly.

### Malformed secondary output

A structured JSON secondary exists but cannot be parsed or interpreted as the
product claims it should be.

Integrity results MUST distinguish malformed payloads from merely stale ones.

Metadata payload validation MUST include public raw-body policy semantics.
If a feed is archived or non-redistributable, a published `{feed}.json`
metadata artifact that still exposes raw/source fields such as `file`,
`source`, `file_local`, or `commit_history` MUST be reported as malformed so
normal reprocess/repair paths regenerate it.

### Blocked by merge input

A merge-derived feed may be blocked because required local input is unavailable.

Rules:

- if a merge has at least one currently eligible additive parent, any missing,
  disabled, archived, unmaintained, or otherwise unavailable subtractive parent
  MUST be reported as a blocked input, because silently publishing without that
  subtraction would broaden the merge output
- missing durable bodies for required additive or subtractive parents SHOULD be
  reported as blocked inputs so recovery can recheck the parent before
  reprocessing the merge
- if a merge has no currently eligible additive parent, integrity MUST treat the
  merge as operationally disabled and MUST NOT report subtractive-parent noise
- integrity MUST evaluate merge eligibility with the same `enable all` setting as
  the running scheduler/admin surface, so disabled markers do not produce false
  blockers in an enable-all daemon

For entity-reference publishing, malformed findings MUST distinguish at least:

- malformed private entity sidecars
- malformed final public country JSON payloads
- malformed final public ASN JSON payloads

## Health-sensitive entity payloads

Country and ASN public detail payloads embed feed-health classes.

For those payloads, integrity MUST detect health-transition drift semantically:

- by comparing the currently rendered health in the public payloads against the
  health that should be rendered now
- not only by comparing file mtimes

File mtimes alone are insufficient proof that a newer file still contains the
currently correct feed-health class.

## Integrity must not create false noise

Integrity MUST suppress or downgrade findings that are not actionable local
breakage.

In particular:

- a feed that is already unavailable upstream and has no local rebuild path
  MUST NOT keep producing "broken pipeline" noise just because its committed
  local input is gone
- v1 critical-infrastructure overlap is IPv4-only; integrity MUST NOT expect
  critical-overlap artifacts for IPv6 feeds or for the critical reference feeds
  themselves
- v1 integrity MUST NOT report per-provider critical-overlap files as missing
  for configured providers that have not been materialized yet
- startup/reload cleanup and public serving MUST reject stale critical-overlap
  artifacts for feeds that are no longer comparable targets
- transient in-flight work MUST NOT be reported as settled integrity failure

## In-flight tolerance

Integrity MUST tolerate feeds that are still being actively processed or whose
global secondaries are still being produced from a recent successful run.

When the product knows a run is still active:

- integrity MUST prefer an `in progress` state over emitting transient findings

## Integrity states

The integrity subsystem MUST expose settled states equivalent to:

- clean
- issues found
- in progress
- recovery scheduled

The exact labels MAY differ, but the meanings MUST remain distinct.

## Recovery model

Integrity recovery MUST split findings into two classes:

### Recheck

Use recheck when the product first needs fresh or staged input before it can
repair the feed.

Examples:

- the committed input is missing for a downloadable feed
- the child input is missing for an artifact-backed child and the parent must be
  refreshed first

### Reprocess

Use reprocess when the product already has enough local committed or staged
input to regenerate the missing outputs without a new upstream fetch.

This is the same local-only recovery path some implementations may call
`rebuild`.

Examples:

- public outputs are missing but the feed already has committed or staged local
  input
- an output-only repair is needed after local artifact loss

This is a local engine/output repair path.

It is allowed to admit work directly into processing because the downloader has
already done its job for that feed body and the integrity problem is in the
engine-owned outputs, not in downloader-stage acquisition or composition.

Additional recovery rules:

- integrity MUST NOT treat a history derivative as broken merely because fewer
  than `X` snapshots exist; snapshots are sparse by observed successful
  parent updates
- if the retained history snapshot set needed to rebuild the derivative for the
  parent's current update timestamp is missing or corrupt
  history is missing or corrupt, integrity recovery MUST target a parent
  `recheck`, not derivative `reprocess`
- if an artifact-backed child has never been materialized locally, integrity
  recovery MUST target a parent-artifact `recheck`, not child `reprocess`

## Startup integrity

On startup, integrity MUST:

- inspect the settled local state
- identify actionable repair work
- queue repair work without blocking availability on expensive execution

Startup integrity MUST NOT become a full historical recomputation phase.

For entity artifacts, startup integrity SHOULD prefer targeted repair of stale
or missing outputs over unconditional full rebuilds.

## Operator-triggered integrity

The operator MUST be able to request a fresh integrity evaluation and recovery
plan.

That operator-visible recovery plan MUST identify, for each finding:

- whether recovery is `recheck` or `reprocess`
- which feed or artifact target(s) will actually be queued

The machine-readable integrity surface MUST preserve malformed-output findings
as distinct from missing or stale outputs; it MUST NOT flatten malformed JSON
into a generic stale/missing bucket.

If a run is already active:

- the product MUST report that integrity is in progress or deferred
- it MUST NOT return a permanently stale waiting message after the activity has
  settled

## Integrity versus health

Integrity and health are separate concerns.

### Health answers:

- is this feed behaving like a healthy upstream source?

### Integrity answers:

- does the local product state still match the last successful local
  publication?

The system MUST NOT conflate:

- dead upstream feeds
- empty but successful feeds
- missing local outputs after a claimed successful publication

## Operator promise

When integrity reports an issue, the operator MUST be able to infer that one of
the following is true:

- the local product state lost or corrupted outputs it claimed to have
- the local product state needs a recheck or reprocess to restore consistency

When integrity reports no issue, the operator SHOULD be able to trust that the
local published state is self-consistent with the last successful run.

## Relationship to severe processing faults

If the processing engine throws an unexpected exception, the product MUST treat
that as a severe runtime fault, not as a normal integrity finding.

The product MAY later produce integrity findings if that severe fault leaves
settled local state inconsistent with the last successful publication.

But the runtime fault itself MUST remain distinguishable from integrity:

- integrity answers whether committed local state is self-consistent
- the severe runtime fault answers that the engine hit a bug or serious
  consistency problem while trying to process work
