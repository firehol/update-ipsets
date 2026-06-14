# Memory Management Contract

## Status

This document is normative.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** describe the required
behavior of the product.

## Purpose

The product MUST remain usable when the total size of downloaded data,
historical evidence, or supporting datasets is larger than available RAM.

Memory management is therefore part of the product contract, not a performance
afterthought.

This document owns the focused memory-specific sub-contract.

Broader cross-cutting operational rules such as startup availability,
cache-first public serving, bounded work, and external dependency discipline
are owned by [operating-principles.md](operating-principles.md).

## Core principle

The product MUST prefer:

- disk over heap for large durable data
- streaming over full materialization where possible
- bounded algorithms over unbounded accumulation

## Download memory contract

Downloads MUST be written to disk-backed staging, not fully buffered in memory,
before they become candidates for processing.

This serves both:

- correctness
- memory safety

## Processor memory contract

Processor pipelines MUST stream processor steps that have streaming
implementations, including line filters, gzip decompression, and gzip-backed
p2p blocklist extraction.

Processor steps that require whole-input semantics MAY materialize one active
intermediate file into heap at a time. Accepted whole-input classes are:

- structured JSON path extraction
- XML or HTML tag extraction implemented as whole-document matching
- ZIP archive entry extraction, where the archive index requires random access
- hostname resolution batches, where bounded worker fan-out owns the blocking
  network work
- legacy one-off parsers whose catalog inputs are small enough to remain within
  the downloader/decompression ceilings

Whole-input processor segments MUST remain bounded by the materialized source
or intermediate file plus the resulting output. They MUST NOT load unrelated
catalog sources or historical data while processing one source.

Canonical feed-body normalization MAY build an in-memory active range set and
rendered canonical body for the source currently being finalized. That active
heap use MUST be scoped to the source being processed and to the configured
processing-worker concurrency. Routine public lookup, comparison, and published
artifact serving MUST prefer file-backed or bounded streaming reads instead of
holding every committed feed body in heap.

Cancellation MUST be checked before whole-input materialization, between byte
processor steps, after byte processing, and before writing or copying temporary
processor output. A canceled processor run MUST NOT publish a partial output as
the final result.

## Set-processing contract

Large set operations MUST be designed so that memory usage is bounded relative
to the active working window, not to the total historical dataset.

This includes operations such as:

- union
- intersection
- exclusion
- overlap counting
- merge composition

The preferred model is file-backed or iterator-based processing.

Pairwise and provider-reference overlap counting SHOULD use exact cheap filters
before scanning both range streams. Acceptable filters include identical
normalized range-content identity, disjoint range bounds, and disjoint occupied
prefix sets. These filters MUST be conservative: if either side lacks enough
evidence, the engine must execute the full overlap count rather than assume
zero or equality.

When several ASN providers are evaluated against the same feed and bogon
reference set, the provider-independent bogon overlap count SHOULD be computed
once per feed and reused across providers. Provider-specific ASN attribution
must still count the non-bogon residual through the provider database so public
`bogon_ips`, `unknown_ips`, and `by_asn` semantics remain unchanged.

Feed-local retention diffing MUST NOT require loading the previous committed
latest set into heap when a valid binary latest set is available. The previous
set SHOULD be opened as a file-backed range source, the removed-IP count SHOULD
be counted by streaming iteration, and only the new cohort set that must be
persisted may be materialized as an in-memory set.

Retention cohort reconciliation SHOULD also open existing binary cohort files as
file-backed range sources. When removals require a cohort rewrite, the engine
SHOULD materialize only the still-listed cohort that must be persisted and avoid
materializing a separate removed set.

Binary set writers for latest snapshots and retention cohorts MUST stream to
their destination or atomic staging file with fixed-size buffers. They MUST NOT
build an additional whole-file byte slice or whole-payload range byte slice on
top of the active `IPSet` being persisted.

## Publication I/O contract

Public artifact, entity artifact, and raw mirror publication SHOULD avoid
rewriting byte-identical live files. When a staged artifact and the existing
live artifact have identical bytes, the publish step may update the live file's
metadata in place instead of replacing the file. When a committed canonical feed
file and the existing raw mirror file have identical bytes, mirror publication
may do the same. Comparisons MUST be bounded and streaming/file-backed rather
than loading large artifacts wholesale into heap.

Entity feed sidecar rebuild workers MUST NOT buffer one completed sidecar per
target feed before staging or aggregation can consume them. Worker result
buffers SHOULD be bounded by worker concurrency so full catalog rebuilds do not
retain every completed feed sidecar in a channel while slower staging work
catches up.

Admin/status response builders SHOULD reuse already-created snapshots inside a
single response construction path when the same source of truth is needed by
multiple sections. They MUST NOT trade this for stale cross-request state; each
request still reports a fresh status view according to the existing admin API
semantics.

## History contract

The product MUST preserve historical evidence, but it MUST NOT treat every
historical dataset as something that needs to be loaded or rescanned for routine
availability or hot-path operations.

Examples of prohibited behavior on critical paths:

- full startup rescans of all historical evidence
- rebuilding bounded public chart data from larger raw stores when equivalent
  bounded artifacts already exist

## Provider dataset contract

Large supporting datasets such as ASN or geolocation databases MUST be handled
in a way that does not require the entire product working set to fit in heap at
once.

The product SHOULD:

- download them to disk
- process them one provider at a time where possible
- keep the last committed good provider data if a refresh fails

Ad-hoc provider lookup caches that survive between public requests MUST be
releasable on successful reload or provider-file replacement without invalidating
in-flight readers. Caches that live on request-serving objects MUST keep stable
object identity across reloads so live requests do not race with pointer
replacement. Their entries must have clear ownership: new lookups must stop
acquiring retired entries, and retired provider databases must close after the
last active lookup or builder releases them.

Compressed provider inputs MUST also remain bounded after expansion. Provider
gzip and tar/gzip extraction paths MUST enforce an expanded-payload ceiling
before committing a local provider file, reject entries that exceed that
ceiling, and clean up incomplete temporary files. Temporary files created for
provider extraction MUST be private to the service user unless an explicit
operator-facing install contract requires broader access.

## Managed service memory guardrail

Managed systemd installs SHOULD set a Go runtime soft memory target below the
service cgroup hard memory limit. The soft target does not replace bounded
algorithms or file-backed processing, but it tells the Go runtime to collect and
return managed memory before the kernel reaches the service `MemoryMax`.

The default managed unit uses:

- `MemoryHigh=1536M`
- `MemoryMax=2G`
- `GOMEMLIMIT=1536MiB`

These defaults leave headroom for memory that the Go runtime soft limit does not
directly manage, including kernel file cache, slab, mmap/file-backed reads, and
other operating-system-held memory charged to the service cgroup.

## Artifact acquisition and extraction contract

Custom artifact transports MUST keep disk, file-cache, and heap pressure bounded
by the input actually consumed by the application.

The product MUST:

- fetch only the required artifact member when the upstream transport supports
  direct member addressing
- avoid retaining unconsumed sibling files from a broader upstream directory
- use private per-run scratch directories for partial acquisition and
  materialization
- clean stale private scratch before reusing the artifact workspace
- preserve the last committed parent input when acquisition or materialization
  fails
- apply the configured acquisition timeout to custom transports as well as the
  generic downloader path

Artifact materializers that derive multiple child feeds from one parent input
SHOULD retain only the child classes or parts required by the currently selected
outputs when this does not change generated child bodies. They may still scan
the complete parent input when needed for parsing correctness, diagnostics, or
warning preservation, but they SHOULD NOT keep unselected child data in heap.

## Staging contract

Staged file semantics are part of the memory contract because they prevent
double-buffering giant datasets unnecessarily in memory.

The product MUST support:

- incomplete temporary files
- complete staged files
- committed files

without requiring all three to be simultaneously resident in heap.

## Insights and secondary-analysis contract

Secondary analysis SHOULD prefer already-bounded artifacts when those artifacts
preserve the same externally visible result.

The product MUST avoid rescanning much larger historical stores on hot paths
when a bounded artifact already exists specifically for that purpose.

## Startup availability contract

The product MUST NOT spend startup availability time on expensive memory-heavy
recomputation that can be deferred or avoided.

This includes:

- broad historical rescans
- whole-catalog analytical recomputation that is not required to answer health,
  status, or serve the initial web/admin surfaces

## Pressure behavior

Under memory pressure, the product SHOULD degrade by doing less speculative
work, not by losing committed correctness.

Examples:

- delay or skip non-essential secondary analysis before dropping committed state
- prefer stale-but-committed supporting data over crash-prone full recomputation

## Operational controls

The product SHOULD be operable under external memory controls such as:

- process-level heap limits
- service manager memory limits
- deployment-level memory ceilings

The product design MUST remain compatible with those controls.

## Implementation freedom

An implementation MAY choose different techniques such as:

- memory mapping
- paged reads
- streaming readers
- spill-to-disk temporary files

as long as the behavioral contract above remains true.
