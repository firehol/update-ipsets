# SOW-0064 - Go External Test Boundary Audit

## Status

Status: completed

Sub-state: validated

## Requirements

### Purpose

Move Go tests toward black-box package contracts where exported behavior can express the same outcome, without banning useful package-level tests for internal algorithms.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- The Go behavioral-testing skill says the default new test package is external (`pkg_test`), with same-package tests reserved for justified internal cases.
- The review found all current Go test files use same-package tests.
- Existing SOW-0046 reduced partial `Engine` construction.
- Existing SOW-0049 migrated some engine tests away from private helpers and explicitly rejected a noisy broad "no unexported calls" rule.

Inferences:

- The issue is not "same-package tests are always wrong"; the issue is same-package tests that lock private helpers when public contracts are available.
- This should be handled package by package, starting with packages that have stable exported APIs.

Unknowns:

- None for the first wave. Remaining same-package packages are classified below
  as justified current exceptions or as optional cleanup candidates only when a
  separate SOW explicitly targets their package boundary.

### Acceptance Criteria

- Inventory same-package tests across `pkg/`, `internal/`, `cmd/`, and `tools/`.
- Classify tests as external-migratable, intentionally same-package, or needing fixture/API work.
- Migrate a focused first wave in stable packages such as `pkg/iprange`, `pkg/processor`, `pkg/downloader`, or selected `pkg/web` tests.
- Document explicit reasons for same-package tests that remain.
- Add a low-noise regression rule only if one exists.

## Analysis

Sources checked:

- `project-go-behavioral-testing` skill.
- SOW-0046 and SOW-0049.
- Representative same-package test files in `pkg/engine`, `pkg/web`, and `pkg/iprange`.

Current state:

- Same-package tests are the norm even when testing exported behavior.

Risks:

- Tests can make private implementation details the effective API.
- A mechanical migration could create worse public APIs or slower tests.

## Implications And Decisions

User delegated implementation-quality, cleanup, testing, and audit SOWs that do
not require product direction. This SOW is classified as assistant-owned because
it changes test package boundaries and guidance without changing runtime
behavior.

Decision:

1. Migration approach
   - A. Blanket convert all tests to external packages.
     - Pros: strict black-box enforcement.
     - Cons: unrealistic and likely harmful.
   - B. Package-by-package migration with explicit same-package exceptions. Selected.
     - Pros: aligns with Go testing skill while preserving useful internal tests.
     - Cons: slower, requires judgment.
   - C. Keep current pattern and document as project exception.
     - Pros: no churn.
     - Cons: contradicts the current skill default.

## Plan

1. Build inventory and classification table.
2. Pick a first package wave with stable exported contracts.
3. Convert tests to external packages or add black-box tests alongside existing internal tests.
4. Record remaining same-package rationale in SOW and project testing guidance.
5. Run package tests, strict tests, race, and lint gates.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Moved to current as assistant-owned test-quality work.
- Inventory before migration showed same-package tests in every Go package with
  tests.
- Migrated first-wave stable exported API packages to external test packages:
  - `pkg/feedhealth/feedhealth_test.go` -> `package feedhealth_test`
  - `pkg/geoloc/geoloc_test.go` -> `package geoloc_test`
  - `pkg/systemd/notify_test.go` -> `package systemd_test`
- Remaining package classification:
  - External-migrated: `pkg/feedhealth`, `pkg/geoloc`, `pkg/systemd`.
  - Optional package-boundary candidates: `pkg/cache`, `pkg/downloader`,
    `pkg/output`, selected `pkg/config`, selected `pkg/insights`.
  - Intentionally same-package or fixture-bound today: `cmd/update-ipsets`,
    `internal/*`, `tools/*`, `pkg/kernel`, `pkg/processor`, `pkg/scheduler`,
    `pkg/engine`, `pkg/web`, and much of `pkg/iprange`.
  - Rationale: those tests exercise main-package behavior, internal packages,
    unexported parsers, global registries, scheduler queues, package-local
    fixtures, or internal algorithm invariants where exported behavior would be
    much broader or slower.
- Updated `.agents/skills/project-testing/SKILL.md` so new stable-API Go tests
  default to external `pkg_test`, with same-package exceptions requiring a
  contract reason.
- No automated regression rule was added. A package-specific rule for the first
  wave would be noisy if an internal invariant test becomes justified; the
  durable project-testing rule is the lower-noise control.

## Validation

Acceptance criteria evidence:

- Inventory completed across `cmd`, `internal`, `pkg`, and `tools`.
- First-wave stable exported API packages were migrated to external tests:
  `pkg/feedhealth`, `pkg/geoloc`, and `pkg/systemd`.
- Remaining packages are classified with rationale.
- Project testing guidance now records the default/exceptions for new tests.
- No low-noise automated regression rule was identified; none was added.

Tests or equivalent validation:

- `go test ./pkg/feedhealth ./pkg/geoloc ./pkg/systemd`
- `make test`
- `make lint`
- `git diff --check`

Real-use evidence:

- The migrated tests now compile only against exported package APIs, proving
  the first-wave behavior is observable without private access.

Reviewer findings:

- Go behavioral-testing review found same-package tests are global and unexported helper calls remain.

Same-failure scan:

- Scanned package declarations with `rg -l '^package [a-zA-Z0-9_]+$' cmd internal pkg tools -g '*_test.go'`.
- Post-migration inventory shows external tests in `pkg/feedhealth`,
  `pkg/geoloc`, and `pkg/systemd`; remaining same-package packages are
  classified above.

Artifact maintenance gate:

- AGENTS.md: no update needed; project workflow already points to project
  testing skills.
- Runtime project skills: updated `.agents/skills/project-testing/SKILL.md`.
- Specs: no update needed; no product behavior changed.
- End-user/operator docs: no update needed; no user/operator behavior changed.
- End-user/operator skills: no update needed.
- SOW lifecycle: current SOW completed and ready to move to done.

Specs update:

- Not needed.

Project skills update:

- Updated `.agents/skills/project-testing/SKILL.md`.

End-user/operator docs update:

- Not needed.

End-user/operator skills update:

- Not needed.

Lessons:

- External test migration works best package-by-package. Stable exported APIs
  can move immediately; queue/fixture/parser-heavy packages are valid
  same-package exceptions unless a separate SOW targets a specific boundary.

Follow-up mapping:

- Remaining same-package packages are classified as optional candidates or
  justified current exceptions; no unmapped item remains in this
  SOW.

## Outcome

Completed. First-wave Go tests were migrated to external packages, remaining
same-package tests were classified, project testing guidance was updated, and
validation passed.

## Lessons Extracted

Use external `pkg_test` by default for new stable exported API tests. Keep
same-package tests only for internal algorithms, package fixtures, global
registries, queue/lock invariants, or cases where public behavior would make
the test much broader or slower.

## Followup

None.
