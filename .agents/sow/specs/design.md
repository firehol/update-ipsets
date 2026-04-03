# Design Contract

## Status

This document is normative.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** describe the required
behavior of the application, not the shape of the current implementation.

## Mission

`update-ipsets` exists to monitor publicly available IP-based threat and
blocking feeds, preserve their history, compare them against each other, and
publish factual results without editorial judgment.

The product value is not any single feed. The product value is the **comparative
observatory** formed by tracking many feeds over time.

## End-user value

The product creates value for end users by turning many heterogeneous feed
sources into one comparable factual observatory.

That value includes:

- one place to discover many public IP feeds
- one consistent way to inspect a feed in depth
- current pairwise comparisons between feeds
- current provider-enriched context such as ASN and geography
- historical evidence showing how feeds change over time
- reproducible machine-readable artifacts that downstream users can consume

The public website and public data contracts deliver that value.

The downloader and processing engine exist to make those public artifacts
possible, but the user-facing value is the published result, not the internal
mechanics.

## Product charter

The system MUST:

- collect live IP-based feeds and related supporting datasets
- prepare them into canonical comparable feed bodies
- preserve enough historical evidence to reason about change over time
- compute factual comparisons and derived measurements
- publish both machine-readable and human-readable results
- give operators clear visibility into acquisition, processing, and integrity

The system MUST NOT:

- rank feeds by opinion
- suppress feeds because they are small, niche, academic, personal, or unknown
- present probabilistic language as fact when the data is deterministic
- depend on implementation details such as package names, file layout, or UI
  component structure as part of the product definition

## Inclusion policy

The catalog MUST include any source that is all of the following:

- publicly reachable
- materially about IP-based blocking, routing, abuse, or network hygiene
- alive
- changing over time

The catalog SHOULD exclude sources that are any of the following:

- permanently dead
- permanently static when the product needs change history
- unrelated to IP-based decision making
- impossible to transform into the product's supported set outputs

The catalog MUST NOT exclude a source merely because:

- it is small
- it is maintained by one person
- it is academic or experimental
- it is not widely known
- it is opinionated or specialized

## Truthfulness policy

The system MUST prefer factual description over editorial labeling.

This means:

- measurements MUST be presented as measurements
- rule-based insights MUST be phrased as deterministic findings
- uncertainty MUST be explicit when the system truly lacks enough evidence
- the system MUST NOT present internal heuristics as objective truth

## Core product entities

The product has four core operational entities:

1. **Feeds**
   - processable inputs that ultimately produce a public set or a derivative of a
     public set
2. **Artifact parents**
   - downloadable upstream artifacts that are not public feeds themselves but
     produce one or more child feeds
3. **Provider databases**
   - supporting datasets used to enrich feeds, such as ASN or geolocation data
4. **Published artifacts**
   - the public outputs consumed by humans, APIs, mirrors, and downstream tools

## Canonical vocabulary

The product contract uses these meanings consistently:

- **raw source**
  - upstream bytes retained only for downloader-side debugging and operator
    inspection
- **feed body**
  - the canonical plain-text feed for one public source, already normalized into
    its configured output family (`ipset` or `netset`)
- **staged feed body**
  - a complete durable canonical feed body in `.{ip,net}set.new`, waiting for
    the engine to claim it
- **processing feed body**
  - a claimed in-flight canonical feed body in `.{ip,net}set.processing`
- **committed feed body**
  - the last committed canonical feed body in `.{ip,net}set`
- **published outputs**
  - everything the processing engine derives from a committed or processing feed
    body, excluding the canonical feed body itself

The downloader owns feed bodies.

The processing engine owns published outputs.

## Separation of concerns

Detailed subsystem contracts live in:

- [downloader.md](downloader.md)
- [processing-engine.md](processing-engine.md)
- [operating-principles.md](operating-principles.md)
- [compatibility.md](compatibility.md)

The product MUST keep the following concerns distinct, even if a specific
implementation chooses different modules or processes:

### 1. Catalog definition

- what exists
- how it is named
- how it is grouped
- what kind of thing it is

### 2. Acquisition

- how upstream or local source material is fetched or otherwise obtained
- how every feed family's next feed body is composed
- how raw source material is preserved for debugging when that feed family has
  raw upstream bytes
- how incomplete downloads are isolated from committed state
- how the system decides whether a feed body should enter the processing queue
- for history derivatives, how fresh parent feed bodies are folded into
  downloader-owned retained history snapshots and then recomposed into
  derivative feed bodies
- for merges, how committed feed bodies are recomposed on cadence

### 3. Processing engine

- how staged feed bodies are claimed for processing
- how feed-local history, retention, and summaries are updated
- how unchanged feed-body state is still reprocessed when global enrichment or
  explicit admin action or integrity recovery requires it
- how peer-facing outputs remain current when another feed changes

Downloader-owned history snapshots and engine-owned retention artifacts are
different concerns and MUST NOT be conflated.

### 4. Global enrichment and comparison

- geolocation
- ASN analysis
- bogon analysis
- overlap/comparison
- higher-level insights

### 5. Publication

- public APIs
- public website
- redistributable mirror files

### 6. Operations

- scheduling
- queue visibility
- manual controls
- integrity
- recovery

The operational contract is explicitly two-loop:

- downloader loop
- processing loop

Automatic cadence selection belongs to the downloader loop.

The processing loop consumes admitted work; it is not a second autonomous
scheduler for ordinary feeds.

## Product-wide invariants

The system MUST satisfy these invariants:

### Catalog invariants

- feed identity MUST be curator-defined, not hardcoded in product logic
- feed grouping and public taxonomy MUST be data-driven
- feed families MUST behave consistently according to their declared kind

### Data safety invariants

- incomplete writes MUST NOT become authoritative state
- restartable staged work MUST survive process death
- failed work MUST leave the last committed good state intact

### Availability invariants

- the daemon MUST become available quickly
- expensive historical recomputation MUST NOT block service availability
- public and admin surfaces MUST describe the real runtime state, not legacy
  internal artifacts

### Failure-class invariants

- downloader-stage failures, integrity findings, and processing-stage severe
  faults MUST remain distinct operator-visible classes
- the processing engine MUST NOT hide downloader/integrity contract violations
  behind hardcoded family-specific fallback branches
- severe processing exceptions MUST be treated as bugs or serious consistency
  faults unless a spec-defined recovery path explicitly says otherwise

### Resource invariants

- the product MUST remain usable when datasets exceed available RAM
- hot paths MUST prefer bounded work over full historical rescans
- large comparisons MUST be designed around file-backed or streaming behavior

### Operator invariants

- every operator-facing status MUST have a clear operational meaning
- integrity MUST report actionable local breakage, not unavoidable upstream
  absence
- manual actions MUST have predictable scope and visible consequences

## Design goals

### Simplicity

The product SHOULD express behavior through a small number of explicit concepts:

- feeds
- artifact parents
- provider databases
- download queue
- processing queue
- publication
- integrity

### Maintainability

The product SHOULD make it possible to change one concern without rewriting the
others.

Examples:

- adding a new feed SHOULD mostly affect catalog configuration
- adding a new artifact family SHOULD not require rethinking the whole
  downloader/processing loop contract
- changing website presentation SHOULD not change feed-processing semantics

### Performance

The product MUST avoid unnecessary work.

Examples:

- unchanged upstream content SHOULD NOT trigger full processing unless the
  downloader, a supporting-dataset update, an explicit admin `reprocess`, or
  integrity-triggered local repair schedules it
- once a feed is queued for processing, the processing engine MUST treat that
  work as mandatory and complete rather than re-deciding sameness locally
- bounded public artifacts SHOULD be reused instead of rebuilding equivalent
  history from larger internal stores when that changes no externally visible
  result
- slow downloads MUST NOT block unrelated processing work

### Operability

Operators MUST be able to answer these questions quickly:

- what is waiting to download?
- what is downloading now?
- what is waiting to process?
- what is processing now?
- what failed, and at which stage?
- which problems are actionable locally?
- which failures belong to downloader, integrity, or severe engine faults?

## Non-goals

The product is not intended to be:

- a feed-ranking authority
- a policy engine that tells operators which feeds are "best"
- a general-purpose ETL platform
- a system that keeps every possible derived statistic fully up to date at all
  times regardless of cost

## Suitability and limits

The product is well suited for:

- comparative analysis of IP-based feeds
- publication of feed-local and cross-feed facts
- operator-managed long-running collection pipelines

The product is less suited for:

- non-IP observables such as domains, URLs, or hashes unless they are first
  transformed into the supported set model
- opaque feed types that cannot be normalized into the comparative model
