# Compatibility Contract

## Status

This document is normative.

The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** describe compatibility
and migration behavior that the product promises relative to the historical
FireHOL bash implementation.

## Purpose

This document answers a narrow question:

> Which bash-era inputs, files, directories, and externally consumed outputs
> must the Go product continue to understand or preserve, and which ones are no
> longer part of the contract?

This is not the migration procedure. The procedural guide lives in
`docs/migration-from-bash.md`.

The concrete modern locations of compatible files are defined in
`.agents/sow/specs/files-layout.md`.

## Compatibility goals

The product SHOULD preserve compatibility where it materially helps one of:

- operator migration from bash to Go
- continued service to downstream consumers of published artifacts
- continued understanding of retained local state from the bash era

The product MUST NOT preserve bash internals purely for nostalgia when they are
not part of a real operational or public contract.

## Compatibility classes

The compatibility contract distinguishes:

### Read compatibility

The Go product can read or import legacy bash-era inputs or state.

### Write compatibility

The Go product continues to publish or mirror outputs in forms that existing
downstream consumers still depend on.

### Operational compatibility

The Go product can coexist with legacy directory layouts, names, or deployment
assumptions even if its implementation is different.

### Non-compatibility

The Go product intentionally does not promise to preserve certain bash-era
internals.

## Read compatibility requirements

The product MUST support read/import compatibility for at least these
historically relevant assets:

### Legacy cache state

- the product MUST be able to import the legacy bash-style cache state when the
  new local cache is absent and migration requires it
- this compatibility exists so operators do not lose historical state
  immediately at cut-over

### Legacy binary snapshots

- the product MUST accept bash-era binary latest snapshots stored as:
  - `lib/{feed}/latest`
- the product MAY accept earlier Go transitional names such as:
  - `lib/{feed}/latest.set`

### Legacy per-update history snapshots

- the bash version stores retained update history as:
  - `/etc/firehol/ipsets/history/{feed}/{unix_timestamp}.set`
- these are per-update snapshots and they match the canonical retained-history
  layout used by the Go downloader
- migration/import tooling SHOULD preserve them directly as downloader-owned
  retained history

### Legacy retention and evolution ledgers

- the product SHOULD preserve or import bash-era per-feed ledgers where they
  materially help continuity:
  - `lib/{feed}/history.csv`
  - `lib/{feed}/changesets.csv`
  - `lib/{feed}/retention.csv`
  - `lib/{feed}/histogram`

### Legacy catalog extraction

- the product MAY load and extract the legacy FireHOL bash script during
  migration scenarios
- if supported, the extraction path MUST translate legacy semantics into the
  canonical Go-era configuration model rather than preserve shell-era grammar
  forever

### Legacy output aliases

- the product MAY accept legacy output aliases when loading configuration
- canonical meaning MUST remain the modern canonical value presented by the
  configuration contract

## Write compatibility requirements

The product MUST continue to publish the public artifacts that existing
website, mirror, or downstream consumers still rely on.

This includes, where those consumers still exist:

- raw downloadable feed outputs
- `.setinfo` sidecars used by repository or mirror consumers
- per-feed metadata artifacts
- bounded history artifacts such as history CSVs
- bounded change artifacts such as changeset CSVs
- pairwise comparison and enrichment artifacts used by the public site
- catalog indexes such as `all-ipsets.json`

The exact public data contracts remain owned by [website.md](website.md).
This document owns the promise that bash-era consumers must not be broken
accidentally while the migration is still in effect.

## File and directory compatibility

The product SHOULD be able to work with legacy directory conventions when
operators choose to deploy that way.

This includes compatibility with historical FireHOL-oriented directory
conventions for:

- base data/output directories
- library/state directories
- admin-supplied and distribution-supplied set directories

The product does not need to preserve the bash implementation's internal shell
layout to satisfy this requirement. It only needs to preserve the externally
meaningful directory and file contracts that operators or consumers depend on.

## Public name compatibility

The product MUST preserve stable public feed identity and public filenames for
feeds that are part of the historical catalog contract, unless a deliberate
catalog or product-level rename policy says otherwise.

## Compatibility for provider-derived synthetic feeds

Where the historical bash system exposed synthetic outputs derived from
provider datasets, the Go product MUST either:

- continue to expose equivalent feed identities and outputs
- or document a deliberate migration/retirement path before breaking them

This matters for historically visible synthetic feeds such as provider-derived
country/ASN-related public feeds.

## Migration-time coexistence

During migration, the product MAY:

- read legacy state
- write new native state
- continue publishing compatibility outputs for downstream consumers

The canonical migration/import helper path is:

- `scripts/sync-from-bash-version.sh`

That helper MUST support importing either:

- from a remote host over ssh/rsync
- or from `localhost` when migrating from a local bash-era layout

It MUST be acceptable for the internal implementation and on-disk internals to
diverge from bash as long as the promised compatibility surface remains intact.

## Explicit non-goals

The product does **not** need to preserve byte-identical bash internals for:

- temporary scratch files
- shell-only helper files
- internal batching ledgers
- private intermediate artifacts with no external consumer
- shell implementation structure

The product also does not need to preserve every historical bash quirk when the
modern specification has deliberately replaced it with a cleaner contract.

Example:

- older Go day-bucket files such as `history/{feed}/{YYYY-MM-DD}.set` are
  transitional compatibility input only
- the bash per-update `history/{feed}/{unix_timestamp}.set` store is now the
  canonical retained-history model

## Breaking-compatibility rule

Any deliberate compatibility break MUST be made explicit in the specifications
before implementation.

That change MUST identify:

- what old contract is being removed
- who is affected
- what replacement exists, if any
- whether migration tooling or operator procedure is required
