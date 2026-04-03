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
