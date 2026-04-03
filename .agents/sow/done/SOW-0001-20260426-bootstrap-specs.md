# SOW-0001 | 2026-04-26 | bootstrap-specs

## Status

completed
superseded by `SOW-0009-20260426-finalize-sow-specs.md`, which made `.agents/sow/specs/` the canonical product spec location

## Requirements

Given update-ipsets had normative product specifications outside the SOW specs tree during initialization, when the bootstrap SOW completed, then the repo needed one explicit product spec ownership model.

Given Costa later approved moving product specs under SOW, when `SOW-0009` runs, then this bootstrap ownership model is superseded.

Given agents need a compact map after context compaction, when they read `AGENTS.md`, then they must know that `.agents/sow/specs/*.md` are now the product/application source of truth.

## Analysis

Sources consulted:

- `AGENTS.md`: current handbook now states that `.agents/sow/specs/*.md` are normative product/application contracts.
- `.agents/sow/specs/README.md`: maps canonical ownership across product specs and enforces a single-owner rule.
- SOW template: expects `.agents/sow/specs/` to exist.

Current state:

- Product specs were complete enough to serve as the authoritative application contract.
- `SOW-0009` later moved those specs under `.agents/sow/specs/`.

Root concern:

- Keeping product specs outside `.agents/sow/specs/` conflicts with the SOW skill's current spec ownership model.

## Implications and decisions

- Costa approved moving product/application specs under `.agents/sow/specs/` in `SOW-0009`.
- `.agents/sow/specs/` is no longer limited to process notes.
- If SOW work changes runtime behavior, the matching `.agents/sow/specs/*.md` file must be updated directly.

## Plan

Single-unit implementation, no chunking - reasoning: this SOW is a bootstrap control SOW with one documentation-ownership decision.

1. Preserve the original bootstrap decision history.
2. Defer the final ownership model to `SOW-0009`.
3. Complete this bootstrap SOW as superseded once `SOW-0009` owns the migration.

## Execution log

2026-04-26:

- Created `.agents/sow/specs/README.md`.
- Recorded the approved ownership model in `TODO-sow-initialization.md`.
- Superseded by `SOW-0009`, which moved product/application specs under
  `.agents/sow/specs/`.

## Validation

- [x] Acceptance criteria evidence

  Evidence: this SOW now points to `SOW-0009` as the final spec-ownership
  migration SOW.

- [x] Real-use validation evidence

  Evidence: `.agents/sow/specs/` exists and now contains the product specs.

- [x] Cross-model reviewer findings (logged + addressed)

  N/A - reason: this completion only closes the superseded bootstrap ownership
  decision. The actual migration is reviewed and validated in `SOW-0009`.

- [x] Lessons extracted (or "none, reasoning: ...")

  Evidence: see `## Lessons extracted`.

- [x] Same-failure-at-other-scales check

  Evidence: `SOW-0009` owns the repository-wide reference audit for spec paths.

## Outcome

Completed as superseded by `SOW-0009`.

## Lessons extracted

None, reasoning: `SOW-0009` records the durable lesson and updates the
project-level spec ownership instructions.
