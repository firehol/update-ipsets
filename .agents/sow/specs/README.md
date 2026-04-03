# Specification Map

## Status

This file is normative for **documentation ownership**, not for product
behavior.

It defines where each kind of contract lives inside `.agents/sow/specs/` so
future changes extend the correct document instead of duplicating the same rule
in multiple places.

The repo-root `specs/` path was intentionally removed by `SOW-0009`; do not
recreate it as a compatibility path.

## Purpose

The spec set exists to define the product above the implementation.

The same application, doing the same work, should be reproducible by reading
the specifications alone.

That requires two things:

- complete coverage of the product contract
- a single canonical home for each normative rule

## Reading order

For a new reader, the recommended order is:

1. [design.md](design.md)
2. [feeds.md](feeds.md)
3. [files-layout.md](files-layout.md)
4. [pipeline.md](pipeline.md)
5. [downloader.md](downloader.md)
6. [processing-engine.md](processing-engine.md)
7. [integrity.md](integrity.md)
8. [operating-principles.md](operating-principles.md)
9. [config.md](config.md)
10. [website.md](website.md)
11. [homepage.md](homepage.md)
12. [admin-ui.md](admin-ui.md)
13. [compatibility.md](compatibility.md)
14. [architecture-posture.md](architecture-posture.md)

`docs/migration-from-bash.md` is procedural support material, not the primary
product contract.

## Canonical ownership

### Product-level contracts

- [design.md](design.md)
  - mission
  - product value
  - top-level architecture and boundaries
  - product-wide invariants
- [feeds.md](feeds.md)
  - feed families
  - feed identity
  - per-feed state model
  - feed time and health semantics
- [files-layout.md](files-layout.md)
  - authoritative on-disk layout
  - file and directory ownership
  - committed vs staged vs temporary naming
  - migration/import workspace layout
- [pipeline.md](pipeline.md)
  - downloader-loop and processing-loop choreography
  - queue admission
  - restart recovery flow
  - manual operation routing
- [integrity.md](integrity.md)
  - settled local correctness after successful publication

### Subsystem contracts

- [downloader.md](downloader.md)
  - downloader-only responsibilities
  - supported downloader item families
  - normalization/composition contract
  - downloader result statuses
  - downloader retries, backoff, and queue behavior
  - downloader-owned fields and files
- [processing-engine.md](processing-engine.md)
  - processing-engine-only responsibilities
  - accepted engine inputs
  - feed-local and global processing pipeline
  - processing results and exceptions
  - engine-owned fields and published outputs

### Cross-cutting operating contracts

- [operating-principles.md](operating-principles.md)
  - startup rules
  - bounded work
  - cache-first public serving
  - performance and dependency discipline
- [memory-management.md](memory-management.md)
  - focused memory-specific sub-contract
  - out-of-core and bounded-memory rules
- [compatibility.md](compatibility.md)
  - normative compatibility and non-compatibility rules with the bash-era
    system
- [architecture-posture.md](architecture-posture.md)
  - internal code-quality posture metrics
  - architecture-debt baseline ownership
  - cache mutation and engine lifecycle inventories
  - review gates for separation-of-concerns regressions

### Interface and surface contracts

- [config.md](config.md)
  - configuration grammar and semantics
- [website.md](website.md)
  - public website routes and data contracts
- [homepage.md](homepage.md)
  - homepage-specific contract
- [admin-ui.md](admin-ui.md)
  - operator-facing runtime, feed, artifact, and integrity surfaces

## Single-owner rule

Each normative rule MUST have one canonical owner.

Other documents MAY:

- summarize that rule briefly
- reference it
- explain why it matters in their own scope

Other documents MUST NOT redefine the same rule independently.

## Cross-reference discipline

The spec set SHOULD follow this pattern:

- product-level docs explain **what the product must do**
- subsystem docs explain **which component owns which behavior**
- interface docs explain **how users and operators interact with the product**

When a rule is already owned elsewhere, the referencing document SHOULD say so
explicitly rather than restating it in full.

## Non-goal

The product spec set is not a package-by-package description of the Go codebase.

Code layout, filenames, function names, and current module boundaries are
implementation details unless they are required to define an external or
operational contract.

`architecture-posture.md` is the explicit exception: it records internal
implementation posture and code-quality guardrails for future agents. It is not
a public API or product behavior contract.
