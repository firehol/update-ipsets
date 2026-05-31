# TODO — specs refactor

## TL;DR

Keep the current core contract docs, but add the missing subsystem and
cross-cutting contracts so the spec set is complete without duplicating the
same rules in multiple places.

The agreed shape is:

- keep:
  - `specs/design.md`
  - `specs/config.md`
  - `specs/feeds.md`
  - `specs/pipeline.md`
  - `specs/integrity.md`
  - `specs/memory-management.md`
  - `specs/website.md`
  - `specs/homepage.md`
  - `specs/admin-ui.md`
- add:
  - `specs/README.md`
  - `specs/downloader.md`
  - `specs/processing-engine.md`
  - `specs/operating-principles.md`
  - `specs/compatibility.md`
  - `docs/migration-from-bash.md`

Completeness goal:

- the product contract must be reconstructable from `specs/*.md`
- downloader and processing engine must each have a first-class contract
- cross-cutting operating rules and bash-compat/migration rules must be
  explicit
- each normative rule must have one canonical home, with other docs linking to
  it instead of restating it

## Purpose

- **Complete the spec set.** The current docs define the product and the
  pipeline well enough to reason about behavior, but they do not yet define the
  downloader and the processing engine as isolated subsystems with their own
  contracts.
- **Avoid duplication drift.** Adding more docs on top of the current set is
  only safe if each rule has one canonical owner.
- **Document the missing cross-cutting rules.** The product still needs a
  first-class operating-principles contract and an explicit bash compatibility
  and migration contract.

## Analysis

Facts verified from the repository and the current spec set:

- `specs/design.md` already owns the mission, charter, truthfulness policy, and
  top-level entity decomposition.
- `specs/config.md` already owns the user configuration contract and feed family
  expression in YAML.
- `specs/feeds.md` already owns feed families and what the product knows about a
  feed.
- `specs/pipeline.md` already owns the two-loop runtime model and the handoff
  between downloader and processing.
- `specs/admin-ui.md` already owns the four live queue panels and the operator
  view.
- `specs/website.md` and `specs/homepage.md` already own most of the public
  value-delivery surface, but they do not yet strongly state the cache-first
  discipline for repeated static page views.
- `specs/memory-management.md` owns only one part of the cross-cutting
  operational rules. Performance, startup behavior, bounded work, and static
  publication discipline are still split across several docs.
- The current spec set does **not** have standalone contracts for:
  - downloader boundary and responsibilities
  - processing-engine boundary and responsibilities
  - subsystem status enums / result classes
  - field ownership per feed/cache entry by subsystem
  - bash compatibility guarantees and explicit migration guidance

Facts verified from the code and legacy bash tree:

- The Go implementation still reads and/or writes multiple bash-era artifacts
  and paths, including legacy cache import and public compatibility outputs.
- Verified evidence:
  - legacy cache migration: [pkg/cache/legacy.go](/home/user/src/firehol/update-ipsets/pkg/cache/legacy.go)
  - config loader still accepts legacy script extraction and migration paths:
    [pkg/config/config.go](/home/user/src/firehol/update-ipsets/pkg/config/config.go),
    [pkg/config/extract.go](/home/user/src/firehol/update-ipsets/pkg/config/extract.go)
  - public/web compatibility artifacts still include history/changeset CSVs:
    [pkg/web/server.go](/home/user/src/firehol/update-ipsets/pkg/web/server.go)
  - legacy bash behavior and filenames are available for comparison in:
    [/home/user/src/firehol/firehol/sbin/update-ipsets](/home/user/src/firehol/firehol/sbin/update-ipsets)

External documentation-pattern research used for structure sanity-checking:

- Diátaxis: keep explanation and reference separate; let reference mirror the
  machinery.
- arc42: separate building-block descriptions from runtime-view/scenario
  descriptions.
- Kubernetes docs: separate concepts from tasks/reference rather than mixing
  them in a single narrative.

Working conclusion:

- We should **not** replace the current structure with two giant docs.
- We should add subsystem-owner docs and cross-cutting docs, then lightly
  refactor existing docs so each rule has one owner and the rest point to it.

## Decisions

Made by user in this conversation:

1. Add the missing subsystem docs as recommended:
   - `specs/downloader.md`
   - `specs/processing-engine.md`
2. Add the cross-cutting docs as recommended:
   - `specs/operating-principles.md`
   - `specs/compatibility.md`
   - `docs/migration-from-bash.md`
3. Keep the existing core docs instead of collapsing the whole spec set into
   giant downloader/engine documents.
4. Ensure the specs are complete, not just navigable.
5. End-user website value should be documented, but absorbed into
   `specs/design.md` and `specs/website.md` rather than split into a separate
   `value.md`.

Implied decisions to execute unless contradicted:

- `specs/README.md` will be added as the documentation map and ownership index.
- `specs/pipeline.md` will remain the runtime choreography contract; downloader
  and engine docs will reference it instead of duplicating the end-to-end
  workflow.
- `specs/memory-management.md` will be retained, but `specs/operating-principles.md`
  will become the canonical owner of broader operating invariants.
- `AGENTS.md` will be updated to point to the new docs and to state the
  ownership model clearly.
- The compatibility split will be:
  - `specs/compatibility.md` = normative compatibility contract
  - `docs/migration-from-bash.md` = procedural migration guide

## Canonical ownership model

Each topic must have one authoritative home:

- `specs/design.md`
  - mission
  - product value
  - major boundaries
  - top-level invariants
- `specs/config.md`
  - configuration directives and grammar
- `specs/feeds.md`
  - feed families
  - feed identity and per-feed knowledge model
- `specs/pipeline.md`
  - downloader-loop ↔ processing-loop choreography
  - queue admission and handoff
- `specs/downloader.md`
  - downloader responsibilities
  - downloader scheduler/retries/statuses
  - downloader-owned fields and files
  - normalization pipeline
- `specs/processing-engine.md`
  - engine input contract
  - engine pipeline/statuses
  - engine-owned fields and outputs
- `specs/integrity.md`
  - post-success local correctness checks and recovery
- `specs/operating-principles.md`
  - startup, bounded work, cache-first serving, performance and safety rules
- `specs/memory-management.md`
  - focused memory-specific sub-contract, referencing operating-principles for
    broader rules
- `specs/website.md` + `specs/homepage.md`
  - public user value and public surface behavior
- `specs/admin-ui.md`
  - operator visibility, controls, API behavior
- `specs/compatibility.md`
  - normative bash compatibility / non-compatibility rules
- `docs/migration-from-bash.md`
  - operator migration procedure

## Plan

1. Add `specs/README.md` to make the spec map and ownership rules explicit.
2. Write `specs/downloader.md` as the downloader subsystem contract.
3. Write `specs/processing-engine.md` as the processing subsystem contract.
4. Write `specs/operating-principles.md` as the cross-cutting operational
   contract.
5. Write `specs/compatibility.md` from verified current compatibility behavior.
6. Write `docs/migration-from-bash.md` as the procedural migration guide.
7. Refactor current docs lightly so they point to the new owners instead of
   duplicating the same rules:
   - `specs/design.md`
   - `specs/website.md`
   - `specs/memory-management.md`
   - `specs/pipeline.md`
   - `specs/admin-ui.md`
   - `AGENTS.md`
8. Run a completeness pass:
   - every subsystem checklist item requested by user is present
   - every new doc has clear ownership and no duplicated normative core
   - the compatibility/migration split is explicit and fact-based

## Testing Requirements

- Verify all newly added docs are linked from `AGENTS.md` and `specs/README.md`.
- Verify there is no stale reference to removed/nonexistent planned files such
  as `value.md` or `publication.md`.
- Grep the spec set for subsystem topics to ensure they have canonical homes:
  - downloader
  - processing engine
  - operating principles
  - compatibility
  - migration
- Review for contradictory ownership statements between old and new docs.

## Documentation Updates Required

- New:
  - `specs/README.md`
  - `specs/downloader.md`
  - `specs/processing-engine.md`
  - `specs/operating-principles.md`
  - `specs/compatibility.md`
  - `docs/migration-from-bash.md`
- Update:
  - `specs/design.md`
  - `specs/pipeline.md`
  - `specs/memory-management.md`
  - `specs/website.md`
  - `specs/admin-ui.md`
  - `AGENTS.md`

## Progress

- Added:
  - `specs/README.md`
  - `specs/downloader.md`
  - `specs/processing-engine.md`
  - `specs/operating-principles.md`
  - `specs/compatibility.md`
  - `docs/migration-from-bash.md`
- Updated ownership/cross-reference notes in:
  - `specs/design.md`
  - `specs/pipeline.md`
  - `specs/website.md`
  - `specs/memory-management.md`
  - `specs/admin-ui.md`
  - `AGENTS.md`
- Tightened the subsystem docs with:
  - explicit supported family coverage
  - downloader and engine field ownership
  - operator/API visibility checklists
- Closed the first review pass by:
  - aligning downloader status names across specs
  - making merge composition explicitly committed-body-only
  - narrowing history-derivative trigger wording
  - trimming `specs/pipeline.md` back toward choreography instead of subsystem
    restatement
  - adding provenance as a canonical spec concept
  - adding explicit admin/public API route-family contracts
  - adding shutdown/reload/logging/write-failure operating rules

## Follow-up gap closure

Second-pass external review plus local verification identified a small set of
real remaining spec gaps. These are now in scope for immediate closure:

- homepage aggregation policy must explicitly explain its intentional treatment
  of `risky` feeds so it does not read like a contradiction against the admin
  filter contract
- `run immediately` must be either defined or removed from the admin action
  surface; current docs define only `recheck` and `reprocess`
- `operator intent` / `explicit operator action` must be normalized to the
  concrete admin/integrity actions already defined elsewhere
- `specs/pipeline.md` feed-family enumeration must align with the canonical
  family set in `specs/feeds.md`
- disable/enable interaction with the downloader/processing state machine must
  be stated explicitly

Evidence verified locally:

- homepage aggregation filter:
  [specs/homepage.md](/home/user/src/firehol/update-ipsets/specs/homepage.md:253)
- admin health filter:
  [specs/admin-ui.md](/home/user/src/firehol/update-ipsets/specs/admin-ui.md:154)
- undefined `run immediately`:
  [specs/admin-ui.md](/home/user/src/firehol/update-ipsets/specs/admin-ui.md:243)
- canonical manual operations are only `recheck` / `reprocess`:
  [specs/pipeline.md](/home/user/src/firehol/update-ipsets/specs/pipeline.md:519)
- undefined `operator intent` references:
  [specs/design.md](/home/user/src/firehol/update-ipsets/specs/design.md:160),
  [specs/design.md](/home/user/src/firehol/update-ipsets/specs/design.md:276)
- family enumeration drift:
  [specs/feeds.md](/home/user/src/firehol/update-ipsets/specs/feeds.md:16),
  [specs/pipeline.md](/home/user/src/firehol/update-ipsets/specs/pipeline.md:156)
- state machine lacking explicit disable semantics:
  [specs/pipeline.md](/home/user/src/firehol/update-ipsets/specs/pipeline.md:623)

Plan for this follow-up:

1. Patch the owning docs only:
   - `specs/homepage.md`
   - `specs/admin-ui.md`
   - `specs/design.md`
   - `specs/pipeline.md`
2. Add explicit rationale where the behavior is intentional instead of changing
   semantics unnecessarily.
3. Re-run a local grep-based audit to confirm the undefined/contradictory
   phrases are gone.

Follow-up progress:

- patched `specs/homepage.md` to make the homepage aggregate health filter
  explicitly narrower than the admin/operator filter surface
- removed the undefined feed-level `run immediately` action and its API route
  from `specs/admin-ui.md`
- normalized vague `operator intent` wording in `specs/design.md` to concrete
  actions already defined elsewhere (`admin reprocess`, integrity-triggered
  local repair)
- aligned `specs/pipeline.md` feed-family enumeration with the canonical family
  list in `specs/feeds.md` by adding artifact parents explicitly
- added explicit pipeline-state-machine notes that enable/disable is an outer
  admission gate, not a separate queue state
- ran a local grep audit and verified that `run immediately` and
  `operator intent` no longer appear in `specs/`
  - tightening canonical `ipset` / `netset` output semantics

## Audit Findings

Fresh spec-only review after the first refactor found these real gaps:

- `specs/pipeline.md` still restates subsystem-local contracts that now belong
  to `specs/downloader.md` or `specs/processing-engine.md`, creating drift risk
- downloader status naming is inconsistent between docs:
  - `pipeline.md` still uses `updated` / `not_updated`
  - `downloader.md` uses `downloaded` / `not_modified`
- merge composition input source is inconsistent:
  - `config.md`, `feeds.md`, `pipeline.md` say committed feed bodies
  - `downloader.md` says committed or staged feed bodies
- history-derivative trigger wording is broader in `downloader.md` than in the
  main feed/pipeline contract
- provenance is used by the public explorer/homepage filters but is not yet
  defined canonically in the spec set
- admin/public API surfaces are described functionally but not yet tied to
  their stable endpoint families strongly enough
- cross-cutting operational rules still miss explicit treatment of:
  - graceful shutdown
  - config reload
  - structured logging expectations
  - disk-full / write-failure behavior
- canonical output format is described semantically but not concretely enough
  for `ipset` / `netset`
- terms like "relevant peer feed" and the insight families are still too loose
  for a fully reconstructable contract

## Cleanup Plan

1. Make downloader status names canonical in `specs/downloader.md` and align
   `pipeline.md` to them.
2. Make merge composition inputs canonical as committed feed bodies only.
3. Narrow history-derivative trigger wording so it matches the product
   contract.
4. Trim `specs/pipeline.md` back to choreography and routing, replacing
   subsystem-local restatements with explicit references to the owning docs.
5. Add provenance as a first-class feed/config concept with canonical values
   and public labels.
6. Add explicit API-contract sections for the public website and admin surface.
7. Add the missing operating-principles sections for shutdown, reload, logging,
   and write-failure behavior.
8. Tighten canonical output semantics and engine terminology where the current
   wording is too loose.
