# SOW-0006 | 2026-04-26 | config-catalog-decomposition

## Status

completed
core catalog decomposition implemented before SOW initialization and migrated with evidence

## Requirements

Given feed/catalog management should be maintainable, when the catalog is authored, then feed definitions should live in a directory-based catalog instead of a monolithic authored YAML file.

Given Costa explicitly approved no backwards compatibility for the old authored catalog, when the repo defaults are resolved, then `configs/firehol/` should be the primary authored catalog source.

## Analysis

Original source:

- `.agents/sow/.todo-backup/TODO-config-catalog-decomposition.md`

Evidence:

- Commit `87e8b49 Split catalog into directory fragments`.
- `configs/firehol.yaml` was removed.
- `configs/firehol/` contains shared files plus per-feed fragments under `sources/`, `merges/`, and `artifacts/`.
- `pkg/config/config.go` accepts directory config paths and recursively merges fragments before finalization.
- `install.sh` deploys `configs/firehol/` to `/opt/update-ipsets/etc/config/`.
- `README.md`, `AGENTS.md`, and specs were updated to reference the directory catalog.

## Implications and decisions

- Costa's no-backwards-compatibility decision for the monolithic authored catalog is preserved.
- Contributor-facing add-feed docs are not complete here; that remains part of release readiness.

## Plan

Single-unit implementation, no chunking - reasoning: core implementation was already completed before SOW initialization.

## Execution log

2026-04-26:

- Migrated completed catalog decomposition TODO into SOW history.
- Original TODO preserved at `.agents/sow/.todo-backup/TODO-config-catalog-decomposition.md`.

## Validation

- [x] Acceptance criteria evidence
  - Directory catalog exists, monolithic source catalog is absent, loader/install/spec references were updated.
- [x] Real-use validation evidence
  - Original TODO records config tests, full Go tests, install, healthz, and status smoke checks as completed.
- [x] Cross-model reviewer findings (logged + addressed)
  - N/A - completed before SOW system; no active SOW review cycle existed.
- [x] Lessons extracted (or "none, reasoning: ...")
  - Lesson captured in `project-coding`: config/catalog changes should be data-driven and not hardcode feed names.
- [x] Same-failure-at-other-scales check
  - Install, runtime defaults, specs, README, and tests were included in the original implementation scope.

## Outcome

The authored catalog moved from a monolithic YAML file to `configs/firehol/` directory fragments.

## Lessons extracted

- Config/catolog source-of-truth changes must update loader defaults, install behavior, specs, README, and tests together.

## Followup

- Contributor documentation for adding feeds and making PRs remains in `SOW-0003 release-readiness`.
