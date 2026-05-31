# TODO: Specs Documentation Split

## Purpose

Create a durable project specification set for `update-ipsets` that is fit for maintainers and operators: concise enough to navigate, complete enough to prevent re-discovery work, and structured so `AGENTS.md` can stay as an agent handbook instead of a giant mixed handbook/specification dump.

## TL;DR

user asked for a new `specs/` directory with:

- `design.md`
- `config.md`
- `feeds.md`
- `pipeline.md`
- `integrity.md`
- `memory-management.md`
- `website.md`
- `admin-ui.md`

At the same time:

- move the relevant project/spec content out of `AGENTS.md`
- keep references in `AGENTS.md` to the new spec files
- explicitly document in `AGENTS.md` that assistants must keep `specs/*.md` up to date as the project evolves
- after the initial split, do one more pass to make the specs stricter and more exhaustive, especially `config.md` and `feeds.md`
- expand `specs/pipeline.md` so the process is described in detail, not just at a high level
- the specs are not field/function reference notes; they must define the application contract well enough that the same application, doing the same work, could be recreated from the specs alone
- the specs are the authoritative source of truth above the implementation: future changes should be requested against the specs first, then implemented in code, and reviewers should judge compliance against the specs
- implementation details such as files, functions, field names, and code layout are secondary and non-authoritative unless they are required to define externally visible behavior or operational guarantees

## Analysis

Facts from the current repository:

- There is no `specs/` directory yet.
- `AGENTS.md` currently mixes:
  - agent workflow rules
  - project philosophy
  - runtime contracts
  - configuration semantics
  - pipeline architecture
  - file layout
  - frontend/admin behavior
  - API summaries
- `AGENTS.md` currently has relevant sections under these headings:
  - `## Mission`
  - `## License & Redistribution Policy`
  - `## Project Overview`
  - `## Architecture`
  - `## Pipeline architecture (queue stages, dynamic injection, derivatives)`
  - `## Frontend Guidelines (React SPA)`
  - `## Development Workflow`
  - `## Key Configuration Files`
  - `## Key Behavioral Rules`
  - `## File Organization`
  - `## REST API Summary`
  - `## Environment Variables (systemd service)`
  - `## Critical Operational Notes`
- Existing adjacent docs that should be used as source material or cross-referenced:
  - `README.md`
  - `ui/README.md`
  - `tools/dronebl2ipsets/README.md`
  - `pkg/web/static/methodology/*.md`
  - multiple `TODO-*.md` files that capture recent design decisions, especially:
    - `TODO-batch-pipeline-rewrite.md`
    - `TODO-insights-optimization.md`
    - `TODO-out-of-core-memory.md`
    - `TODO-website*.md`

Working interpretation:

- `AGENTS.md` should remain the operational handbook for assistants.
- `specs/*.md` should become the stable project specification set.
- `AGENTS.md` should reference specs rather than duplicating the full project knowledge.
- The current first-pass specs are too code-shaped and too close to implementation notes.
- The real target is a contract/specification set that captures:
  - purpose
  - design philosophy
  - separated concerns
  - logic
  - policies
  - flows
  - fallback/retry behavior
  - operator semantics
  - expected outcomes
  - suitability/limits
  in a way that allows reimplementation from the specs alone.
- The specs must be written as application contracts:
  - what the system must do
  - under which conditions
  - with which guarantees, fallback rules, and visible outcomes
  - independent of the current Go/React implementation shape

## Decisions

Made by user:

- Create the eight exact spec files listed in the request.
- Clean up `AGENTS.md` by moving relevant project information into `specs/`.
- Keep references in `AGENTS.md` to all spec files.
- Add an explicit maintenance rule in `AGENTS.md` that assistants must keep `specs/*.md` up to date.
- After the initial migration, do a second pass to make the specs stricter and more exhaustive, especially `config.md` and `feeds.md`.
- `specs/pipeline.md` must describe the process in detail.
- The specs must define the application contract, not mirror code structure.
- The application should be reproducible from the specs alone.
- Pipeline specs should include diagrams, key conditions, logic, fallback strategies, retry schedules, policies, rules, and flows for the different feed families.
- The specs sit above the implementation as the authoritative contract for future behavior changes and compliance review.
- Specs will be written as normative contracts using requirement language (`MUST`, `MUST NOT`, `SHOULD`, `MAY`) with rationale separated from contract where useful.
- The main body of the specs will be implementation-independent and must not rely on package names, function names, field names, or current code layout as part of the contract.
- Rewrite order for the contract pass:
  1. `design.md`
  2. `pipeline.md`
  3. `feeds.md`
  4. `integrity.md`
  5. `config.md`
  6. `admin-ui.md`
  7. `website.md`
  8. `memory-management.md`
- Commit the rewritten spec set first so the repo is clean for review.
- Then run external reviewers against the implementation with the specs treated as primary truth and implementation as the thing being judged.
- Merges must fail when any required input feed output is missing; silent degraded merge composition is not acceptable.
- Fix the identified implementation gap where integrity only semantically validates geo structured secondaries.
- The spec rewrite must optimize for reproducibility of the product, not for navigability of the current code.

Implied decisions to execute unless contradicted:

- `AGENTS.md` will keep agent-only rules and short project pointers, not full duplicated specs.
- The new specs will be written as authoritative project docs, not TODO-style notes.
- Existing details will be reorganized, not weakened or dropped.

## Plan

1. Inspect the relevant sections of `AGENTS.md`, `README.md`, UI docs, and recent TODO files. Completed.
2. Draft `specs/` with one file per requested topic, using current codebase facts and existing contracts. Completed.
3. Rewrite `AGENTS.md` so it stays an agent handbook and points to the new specs for project knowledge. Completed.
4. Verify internal consistency:
   - references from `AGENTS.md`
   - no major project contract lost
   - doc structure matches the requested topics
   Completed.
5. Perform a second-pass tightening of the specs with extra detail and stricter wording, prioritizing `config.md` and `feeds.md`. Completed.
6. Expand `specs/pipeline.md` into a detailed process specification based on the actual scheduler/engine code paths. Completed.
7. Rewrite the spec set from "documentation of current code" into "authoritative application contract" form. Pending.
8. Commit the spec rewrite.
9. Run external review against the implementation using the specs as the review contract.
10. Resolve the verified gaps:
   - tighten merge contract in specs and implementation to fail on missing inputs
   - extend integrity semantic validation beyond geo structured secondaries
   Completed.

## Implied Decisions

- `design.md` will hold mission, philosophy, inclusion rules, operational principles, and major architectural intent.
- `config.md` will own catalog/config/runtime/user-facing configuration semantics, including license/redistribution policy.
- `feeds.md` will own feed lifecycle knowledge, derived feeds, artifact parents, per-feed files, and stored metadata.
- `pipeline.md` will own scheduler, download queue, processing queue, dynamic derivative expansion, and heavy fan-out phases.
- `integrity.md` will own integrity checks, integrity recovery, and the distinction between actionable breakage and unavailable feeds.
- `memory-management.md` will own mmap/fileset/out-of-core design and resource-bound guarantees.
- `website.md` will own public website/frontend stack/design system/public routes/public data contracts.
- `admin-ui.md` will own admin operations, queue displays, run actions, and operator semantics.

## Testing Requirements

- Validate that every requested file exists under `specs/`.
- Validate that `AGENTS.md` references all spec files.
- Review for contradictory statements between `AGENTS.md` and `specs/*.md`.
- No code tests are required unless incidental code changes become necessary.

## Documentation Updates Required

- New:
  - `specs/design.md`
  - `specs/config.md`
  - `specs/feeds.md`
  - `specs/pipeline.md`
  - `specs/integrity.md`
  - `specs/memory-management.md`
  - `specs/website.md`
  - `specs/admin-ui.md`
- Update:
  - `AGENTS.md`

## Completed

- Created the new `specs/` directory.
- Added:
  - `specs/design.md`
  - `specs/config.md`
  - `specs/feeds.md`
  - `specs/pipeline.md`
  - `specs/integrity.md`
  - `specs/memory-management.md`
  - `specs/website.md`
  - `specs/admin-ui.md`
- Rewrote `AGENTS.md` into an assistant handbook with references to the spec files.
- Added the explicit maintenance rule in `AGENTS.md` that assistants must keep `specs/*.md` up to date.
- Tightened `specs/config.md` with:
  - validation/collision rules
  - output canonicalization
  - accepted URL families
  - derivative expansion semantics
  - artifact-backed child constraints
- Tightened `specs/feeds.md` with:
  - timestamp semantics
  - stronger enable rules
  - clearer committed/staged file semantics
  - public/internal artifact meanings
  - rename/delete cleanup behavior
- Rewrote `specs/pipeline.md` into a detailed process spec covering:
  - startup order
  - fetch loop behavior
  - queue state transitions
  - manual action routing
  - staged download handling
  - artifact parent materialization
  - processing batch semantics
  - `RunOnce()` phases
  - heavy fan-out and publish behavior
- Ran external implementation-vs-spec reviews and manually classified the main candidates:
  - integrity semantic validation gap: real implementation gap
  - merge missing-input behavior: real spec gap, then fixed after user decided merges must fail
  - comparison-path memory claim: reviewer mistake
- Implemented the verified gaps:
  - merges now fail when any required committed input output is missing
  - integrity now semantically validates structured JSON secondaries for:
    - metadata
    - retention
    - comparison
    - insights
    - geo
    - ASN
    - bogon payloads
  - integrity findings now distinguish malformed secondary files from stale ones
- Added regression coverage for:
  - merge failure on missing input
  - malformed structured secondaries across all supported JSON artifact families
- Verified with:
  - `go test ./pkg/engine ./pkg/web`
  - `go test ./...`

## Known problem with current output

- The current `specs/*.md` set is still too implementation-shaped.
- It is useful as internal technical documentation, but it does not yet meet user's required bar for a true application contract / reimplementation spec.
- A rewrite is required, especially for:
  - `specs/design.md`
  - `specs/pipeline.md`
  - `specs/feeds.md`
  - `specs/admin-ui.md`
- The rewrite target is now explicit:
  - specs first
  - implementation second
  - reviewer compliance against specs, not against code shape
