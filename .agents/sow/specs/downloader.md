# Downloader Contract

## Status

This document is normative.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** describe the required
behavior of the downloader subsystem.

## Purpose

The downloader is the isolated subsystem responsible for obtaining or composing
the canonical local feed files that the rest of the product consumes.

Its job is to convert heterogeneous upstream or local source material into one
of three durable outcomes:

- a retained **raw source** for downloader-side debugging when the feed family
  begins from raw upstream bytes
- a canonical plain-text **feed body** for a public feed
- a **provider archive/body** for a supporting dataset such as ASN or GEO

The downloader is the only subsystem allowed to decide whether newly observed
source material is:

- failed
- not_modified
- semantically the same
- downloaded
- empty

## Boundary

The downloader MUST own:

- due scheduling for downloader-stage items
- upstream acquisition
- local composition for synthetic feeds
- preservation of raw source material for debugging where applicable
- normalization of source material into canonical feed bodies
- semantic comparison of canonical feed bodies against the latest local version
- durable staging of downloader outputs
- downloader failure tracking and retry/backoff
- admission of processing work

The downloader MUST NOT own:

- feed-local retention artifacts
- pairwise comparison artifacts
- public methodology or insight artifacts
- website page generation
- integrity judgments about settled published outputs

## Exclusive responsibility

The downloader has the exclusive responsibility of converting every supported
source family into the form expected by the processing engine.

This includes all families, without exception:

- direct remote feeds
- local-file feeds
- history derivatives
- merges
- artifact-backed synthetic feeds
- provider datasets

The processing engine consumes only downloader-produced local state.

## Downloader-visible item families

The downloader MUST support these downloader-stage item families:

### 1. Direct upstream feeds

- fetch from `http://`, `https://`, or `file://`
- retain the fetched raw source for debugging
- apply the configured extraction/normalization pipeline
- produce a canonical feed body in the configured output family

### 2. History derivatives

- do not fetch their own upstream body
- compose a new feed body from:
  - the fresh parent feed body for the successful parent update that triggered
    recomposition
  - downloader-owned retained history snapshots for that parent
- are triggered by parent update, manual recheck, or restart recovery of valid
  staged state

### 3. Merges

- do not fetch their own upstream body
- compose a new feed body from the latest durable local canonical feed bodies
  of their inputs that are both:
  - currently enabled
  - not currently excluded from merge composition by health policy
    (`archived` and `unmaintained`)
- run on their own cadence

### 4. Artifact parents

- fetch or otherwise obtain a parent artifact that is not itself a public feed
- materialize one or more child feed bodies from that artifact
- built-in examples include custom downloader families such as DroneBL /
  DNSBL-style artifact downloaders
- recover durable staged parent artifacts left by an interrupted process and
  materialize their children through the same downloader FIFO path used by
  fresh acquisition

### 5. Artifact-backed child feeds

- do not fetch upstream directly
- use already materialized local child input produced from one artifact parent
- may then normalize that local input into their canonical feed body

### 6. Provider datasets

- fetch or compose supporting datasets such as ASN, geolocation, or bogon data
- stage a durable provider input/archive
- may trigger broad feed reprocessing when they update

Built-in provider examples include:

- ASN providers
- geolocation providers
- bogon/reference providers

## Normalization and composition contract

For every feed body the downloader produces, it MUST perform the source-family
specific preparation needed so the processing engine receives a canonical local
input rather than raw upstream material.

The downloader normalization pipeline MUST conceptually support these steps:

1. resolve the configured reference
2. acquire or locate source material
3. spill acquisition to disk-backed staging rather than heap
4. persist raw source material for debugging when that feed family has a raw
   source artifact
5. decompress, extract, or materialize intermediate local input as required
6. run the configured processor/extraction pipeline
7. parse, clean, deduplicate, and canonicalize the resulting IP/CIDR content
8. render the canonical feed body directly in the feed's configured output
   family (`ipset` or `netset`)
9. compare that canonical feed body against the latest local canonical version
   of the same feed
10. stage the new durable downloader outcome only when the canonical feed body
    changed or became newly empty
11. admit processing work only when the downloader outcome requires it

`latest local canonical version` means, in priority order:

- an existing staged `.{ip,net}set.new`
- otherwise an existing in-flight `.{ip,net}set.processing`
- otherwise the committed `.{ip,net}set`

Semantic sameness is about the normalized IP/CIDR content, not about upstream
formatting differences.

The downloader MUST determine its result enum/status from this canonical
comparison model. The processing engine MUST NOT repeat raw-source parsing just
to rediscover whether a feed changed.

## Processor pipeline contract

The downloader MUST own the configurable extraction pipeline that turns raw
source material into IP/CIDR content.

That pipeline MUST be:

- composable
- deterministic for the same input
- ordered
- reusable across feed families that begin from raw source material
- robust against arbitrary upstream formatting, including very long lines or
  single-line HTML/JSON payloads

Adding a new source family SHOULD follow one of these patterns:

- reuse the existing processor pipeline for raw-material extraction
- add a new downloader-specific acquisition/materialization stage before the
  processor pipeline
- add a new provider/artifact family only when the data cannot be expressed as
  an ordinary direct feed

The downloader MUST NOT require the processing engine to understand raw
upstream formats.

The downloader MUST parse non-canonical source material only once per feed run.
Any intermediate parsing or cleanup needed to decide `same`, `downloaded`, or
`empty` belongs here, not in the engine.

## Durable staging contract

Downloader writes MUST use the product-wide temporary and staged semantics:

1. incomplete write to `{file}.tmp`
2. raw source retention to its committed debug path when that file family
   exists
3. complete durable staging of the canonical feed body to
   `.{ip,net}set.new`
4. later promotion of that exact canonical feed body to committed
   `.{ip,net}set` only after successful downstream use by the engine

The downloader MUST write feed bodies and provider inputs to disk before they
can be admitted to processing.

The file families and ownership model are defined in
[files-layout.md](files-layout.md). This document defines which subsystem
controls them.

## Downloader result statuses

The downloader MUST expose strongly-typed result statuses.

There are two related status layers:

- low-level acquisition results returned by the downloader package
- cache/operator `LastStatus` values written by the engine while it applies
  downloader-stage work

Specs and UI MUST NOT collapse these layers. A low-level `ok` means source
material was obtained. An operator-visible `downloaded` means the engine staged
new or changed local input for downstream processing.

At minimum, the status model MUST include these meanings:

### `disabled`

- the item was not eligible because it is operationally disabled

### `downloading`

- acquisition or composition is actively in progress

### `materializing`

- artifact-parent work is actively materializing child-local input from an
  already acquired parent artifact

### `missing_env`

- the configured URL or downloader configuration could not be resolved because a
  required environment variable is absent

### `url_resolve_failed`

- the item could not resolve a local or synthetic reference into a usable local
  source

### `download_failed`

- acquisition or composition could not produce a fresh usable downloader result
- examples:
  - HTTP 404/410/5xx
  - timeout
  - provider archive missing
  - merge input missing
  - artifact child local materialized input missing

### `prepare_failed`

- source material was obtained, but normalization into a canonical feed body
  failed

### `history_snapshot_failed`

- the downloader obtained or composed a valid parent feed body but could not
  update the downloader-owned history-snapshot state needed for history
  derivatives

### `ok`

- the downloader obtained or recomposed input successfully and the result
  differs semantically from the latest local canonical version — the body was
  downloaded and is new or changed
- this is the downloader's "new content obtained" outcome, distinct from `same`

### `same`

- the downloader obtained or recomposed input successfully
- after normalization, the resulting feed body is semantically equivalent to
  the latest local canonical version of that feed
- this is the downloader's "no change" outcome

### `not_modified`

- upstream positively asserted that the source has not changed
- this is a downloader scheduling outcome, not a processing outcome

### `downloaded`

- the engine applied a successful downloader-stage result and staged a local
  feed body or provider input that differs from the latest local canonical
  version, or was explicitly forced for processing admission
- this is the cache/operator terminal status for refreshed local input, distinct
  from the low-level downloader result `ok`

### `empty`

- the downloader produced a valid successful empty result
- empty is not failure

### `failed`

- local downloader-stage bookkeeping failed after acquisition/composition should
  otherwise have succeeded

### `skipped`

- the named item does not participate in downloader-stage work

## Status applicability by family

- direct upstream feeds may emit any acquisition/normalization status
- history derivatives may emit:
  - `disabled`
  - `downloading`
  - `download_failed`
  - `history_snapshot_failed`
  - `same`
  - `downloaded`
  - `empty`
  - `failed`
- merges MUST NOT emit `not_modified`, because merges do not have an upstream
  freshness protocol
- artifact parents may emit acquisition/materialization statuses but do not
  enter the processing queue as public feeds
- provider datasets may emit downloader statuses and, on successful update, may
  admit processing reprocess work for dependent feeds

## Scheduler and queue model

The downloader MUST run autonomously.

Downloader admission MUST be stable and deterministic. Items with the same
effective due time or queue timestamp MUST start in FIFO enqueue order, and
queue merges MUST preserve the earliest enqueue position for that work item.

It owns these operator-visible live states:

1. waiting to be downloaded
2. being downloaded now

The detailed end-to-end choreography is owned by [pipeline.md](pipeline.md).
This document owns the downloader-local semantics.

## Due scheduling

The downloader MUST evaluate due work from configured cadence plus downloader
retry/backoff rules.

The downloader MUST be the only automatic selector of new work.

The processing engine MUST NOT autonomously decide that a feed is due for fresh
source acquisition.

## Dirty-item rule

The downloader MUST be able to prepare a new staged canonical feed body while
the engine is processing an older `.{ip,net}set.processing` body for the same
feed.

This means downloader-stage work and engine-stage work for the same feed are
allowed to overlap as long as they operate on different durable file states.

The downloader MAY still coalesce repeated automatic due events when an
equivalent newer staged feed body already exists.

## Failure retry and backoff

Hard downloader failures MUST use the downloader retry policy defined by the
product contract:

- first retry at `cadence / 16`
- double on each hard failure
- cap at the configured ordinary cadence until the feed reaches unmaintained
- once unmaintained, continue doubling up to a hard cap of one month

This retry policy applies to hard downloader failures.

It MUST NOT apply to:

- `not_modified`
- `same`
- successful `empty`

## Family-specific scheduling rules

- direct upstream feeds follow their configured cadence
- merges follow their own configured cadence
- history derivatives do not own their own cadence; they follow parent-driven
  downloader behavior
- artifact children do not fetch independently; they depend on artifact parent
  downloader work or existing materialized local input
- provider datasets follow their configured cadence and may trigger dependent
  reprocess waves on successful update
- archived feeds MUST NOT be admitted by ordinary automatic due scheduling or
  failure backoff
- explicit operator `recheck` MAY still admit an archived feed to downloader
  work
- archived-inclusive integrity recovery MAY still admit archived feeds only when
  the operator explicitly enabled archived scope for that integrity pass

## Admission to processing

The downloader MUST be the ordinary source of new processing work.

The exact cross-loop admission rules are owned by [pipeline.md](pipeline.md).
This downloader contract owns only the ordinary downloader-stage admissions and
the downloader-originated provider-refresh wave admissions.

The downloader MUST NOT admit raw upstream material directly to the engine.

## Configuration directives that affect the downloader

The exact syntax belongs to [config.md](config.md). This document defines which
configuration concerns the downloader must honor.

### Global runtime concerns

The downloader MUST honor runtime configuration for at least:

- working directories used for downloader-owned files
- download concurrency
- size and timeout limits
- user agent / transport behavior
- downloader retry behavior
- environment-variable expansion for downloader URLs
- outbound HTTP proxy environment variables honored by Go's HTTP transport
  (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`, and lowercase equivalents)

### Per-feed and per-artifact concerns

The downloader MUST honor per-item configuration for at least:

- source reference / URL
- cadence
- processor pipeline
- downloader/downloader options when a custom downloader is used
- format hints needed for acquisition/extraction
- history windows
- merge inputs
- artifact parent type and deliveries
- artifact-specific acquisition bounds when one artifact family needs a
  different size cap than the global downloader default
- artifact-specific acquisition timeout when one artifact family uses a custom
  transport outside the generic HTTP/file downloader
- artifact-specific acquisition credentials from environment variables when a
  supported artifact family requires authenticated transport. For the current
  DroneBL rsync artifact family, the downloader MUST prefer
  `DRONEBL_RSYNC_PASSWORD` and fall back to `RSYNC_PASSWORD`.
- artifact-specific acquisition scope. A custom artifact transport MUST fetch
  only the parent input consumed by the application when the upstream transport
  supports addressing that input directly. It MUST NOT persist unconsumed
  sibling upstream files as a side effect of acquisition.
- provider roles
- enabled/disabled state
- runtime health-derived archival/composition exclusion state where scheduling or
  merge composition depends on it

For the current DroneBL rsync artifact family, the consumed parent input is the
`buildzone` file. The rsync acquisition path MUST target that file directly,
MUST avoid persistent partial/progress artifacts in the committed fetch area,
MUST promote the fetched file atomically only after success, and MUST leave the
last committed parent input usable when acquisition fails.

If process startup finds a durable staged DroneBL parent artifact from a prior
interrupted run, the downloader MUST recover it as downloader-stage work. The
recovery path MUST enqueue the artifact parent in the downloader FIFO and let a
downloader worker materialize child feed inputs. Startup or scheduler recovery
code MUST NOT bypass the downloader queue by materializing DroneBL children
directly or by enqueuing those children straight into processing.

## Admin visibility and controls

The admin UI and APIs MUST expose the downloader as a first-class operational
subsystem.

Operators MUST be able to observe at least:

- waiting to be downloaded
- being downloaded now
- per-feed downloader status and downloader error
- next due time and why an item is due or blocked
- artifact parent downloader state

Artifact-parent downloader state MUST be derived from the artifact parent's own
recorded check/failure timestamps and retry counters. An implementation MUST
NOT silently collapse artifact parents into a feed-only state view that makes a
recently failed artifact appear as `never checked`.

Operators MUST be able to trigger at least:

- downloader-stage recheck for a specific feed
- downloader-stage recheck for a specific artifact parent
- run due work now

Dedicated operator APIs SHOULD include equivalents of:

- downloader/feed recheck
- artifact-parent recheck
- run due work now / wake downloader evaluation

If an artifact child lacks local materialized input, a manual downloader-stage
recheck MUST target the artifact parent rather than failing the child blindly.

For raw-source feeds, a manual downloader-stage recheck MUST be able to heal a
stale or previously misparsed canonical feed body from the retained raw source
even when the downloader result is `same` or `not_modified`. It MUST NOT
blindly reuse the existing canonical feed body when that would preserve known
bad local state.

## Downloader-controlled fields per feed

The downloader MUST be authoritative for these per-feed facts:

- last check time
- last upstream change time
- download failure count and failure streak start
- whether a staged downloader result exists

The downloader also writes the latest downloader terminal status and message
when downloader-stage work is the latest completed action.

At minimum, downloader-controlled persisted fields MUST include equivalents of:

- `CheckedDate`
- `SourceDate`
- `DownloadFailures`
- `FailureStartedDate`
- downloader-stage values of `LastStatus`
- downloader-stage values of `LastError`

The downloader MUST NOT be authoritative for:

- last successful local publication time
- published set size and retention measurements
- comparison, enrichment, or insight artifacts

## Downloader-owned files and state

The downloader controls content creation and staging of:

- raw source files where applicable
- staged feed bodies
- temporary downloader scratch files
- downloader-owned history snapshots
- artifact parent local source state
- provider dataset local source/archive state

The engine controls claiming staged feed bodies into `.processing` and writing
successfully finalized normal feed bodies to committed canonical feed-body
files. Provider archives and artifact-parent source archives are promoted as
supporting inputs, not as public feed bodies.

The file names and file layout are defined in [files-layout.md](files-layout.md).

## Invariants guaranteed to callers

The downloader MUST guarantee:

- the processing engine never needs to understand raw upstream feed formats
- updater decisions are made from normalized feed content, not cosmetic source
  differences
- staged downloader outputs are durable before they are admitted
- slow or failing acquisition does not block already-admitted processing work
- provider updates can trigger full feed reprocessing without pretending the
  feeds themselves changed upstream

## Feed body header format

Every committed `.ipset` and `.netset` file MUST begin with a metadata header
block consisting of `#`-prefixed comment lines. The header MUST include at
minimum:

- feed name
- IP version and hash type (`ipv4 hash:ip` or `ipv4 hash:net`)
- description
- maintainer name and URL
- **list source URL** (raw, unexpanded from config)
- source file date
- category
- version
- file generation date
- update frequency
- aggregation window
- entry count
- link to full analysis on the public website
- generator attribution

The committed public file MAY be reused as local reprocess input. Reprocessing
or repairing from that committed file MUST be idempotent: final publication MUST
write exactly one header block and MUST treat existing `#` comment lines as
metadata/comments, not as canonical feed entries. Same-body detection and kernel
apply MUST compare/apply the comment-stripped canonical body.

Final publication MUST stream the committed body into the headered output
instead of materializing both the full body and the full `header + body` output
in heap.

## URL display contract

The "List source URL" line in the header MUST show the URL exactly as it
appears in the source configuration, before environment-variable expansion.

- If the source declares `public_url`, the header MUST show the raw
  `public_url` value.
- If the source does not declare `public_url`, the header MUST show the raw
  `url` value.
- In both cases, `${VAR}` syntax MUST appear literally in the header; real
  API keys, tokens, or secrets MUST NEVER be substituted into the displayed
  URL.

This contract prevents accidental credential exposure in publicly distributed
feed files.

## What MUST NOT cross the boundary

The downloader MUST NOT leak downloader-internal acquisition complexity into
the processing contract.

The processing engine should need to know only:

- which feed is being processed
- which durable local input it should use
- why the run was admitted

Everything else about acquisition, extraction, and composition belongs to the
downloader.
