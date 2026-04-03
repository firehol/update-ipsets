# SOW-0067 - Nested Module Validation Gates

## Status

Status: completed

Sub-state: completed and validated

## Requirements

### Purpose

Ensure nested Go modules receive appropriate race and coverage validation, or record an evidence-backed exception.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- The repo has a nested Go module at `tools/dronebl2ipsets/go.mod`.
- `make test-tools` runs tests inside the nested module.
- `make race` currently runs `go test -race ./...` in the root module only.
- The CI coverage job runs `go test -coverprofile=coverage.out -covermode=atomic ./...` in the root module only.
- `make staticcheck`, `make golangci-lint`, and `make vulncheck` already include the nested module.

Inferences:

- Nested-module test execution exists, but race and coverage policy is inconsistent.
- Coverage may or may not be worth gating for this small tool; the decision should be explicit.

Unknowns:

- Resolved: nested race is cheap enough to include in `make race`.
- Resolved: separate nested coverage is simpler and clearer than merging root
  and nested module coverage profiles.

### Acceptance Criteria

- Nested module validation policy is documented in Makefile/CI/SOW.
- Race validation either includes the nested module or records a justified exception.
- Coverage validation either includes the nested module or records a justified exception.
- CI and local Makefile targets match the documented policy.

## Analysis

Sources checked:

- `Makefile`
- `.github/workflows/ci.yml`
- `tools/dronebl2ipsets/go.mod`

Current state:

- Nested module tests are covered by `make test-tools`, but not by root race or coverage commands.

Risks:

- Nested module regressions can miss race/coverage quality gates.
- Overcomplicating CI for a tiny helper module may not provide enough value.

## Implications And Decisions

Autonomous maintainer decision:

1. Gate policy: A, with separate coverage profiles instead of merged coverage.
   - Evidence: `make test-tools` completed from cache in about 0.06s,
     nested `go test -race ./...` completed in about 1.4s, and nested
     coverage completed in about 0.28s with 69.0% statement coverage.
   - Implication: CI gains nested race and coverage protection with low
     runtime cost and without coverage-profile merge complexity.
   - Risk: the nested helper module now has the same 50% coverage floor as the
     root module; this is currently safe because measured coverage is 69.0%.

## Plan

1. Measure nested module test/race/coverage runtime.
2. Choose gate policy with evidence.
3. Update Makefile and CI.
4. Add documentation to project testing skill if policy changes.
5. Run validation gates.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Moved from `pending/` to `current/` as autonomous maintainer-owned
  validation work.
- Measured nested module gates:
  - `make test-tools` passed from cache in about 0.06s.
  - `cd tools/dronebl2ipsets && go test -race ./...` passed in about 1.4s.
  - `cd tools/dronebl2ipsets && go test -coverprofile=coverage.out -covermode=atomic ./...`
    passed in about 0.28s with 69.0% statement coverage.
- Updated `make race` to include `tools/dronebl2ipsets`.
- Added `make coverage` and `make coverage-tools`.
- Updated CI coverage job to use the Makefile targets and enforce a separate
  50% threshold for the nested module.
- Updated `project-testing` with the nested module race/coverage policy.

## Validation

Acceptance criteria evidence:

- Nested module validation policy is documented in `Makefile`,
  `.github/workflows/ci.yml`, this SOW, and
  `.agents/skills/project-testing/SKILL.md`.
- Race validation includes `tools/dronebl2ipsets` through `make race`.
- Coverage validation includes `tools/dronebl2ipsets` through
  `make coverage-tools` and the CI nested coverage threshold step.
- CI and local Makefile targets match the documented policy.

Tests or equivalent validation:

- `make race` passed, including nested module race validation.
- `make coverage` passed.
- `make coverage-tools` passed.
- Root coverage threshold check passed at 67.1%.
- Nested tool coverage threshold check passed at 69.0%.
- `git diff --check` passed.

Real-use evidence:

- The exact Makefile targets used by CI were run locally.

Reviewer findings:

- Go behavioral-testing review found nested module race/coverage gate gaps.

Same-failure scan:

- `find . -name go.mod -print` found only `./go.mod` and
  `./tools/dronebl2ipsets/go.mod`.
- `rg 'test-tools|race|coverage|coverprofile|go test|dronebl2ipsets' Makefile .github/workflows/ci.yml`
  verified the touched local and CI gates.

Artifact maintenance gate:

- AGENTS.md: no update needed; project-wide rules already require nested module
  validation awareness.
- Runtime project skills: updated
  `.agents/skills/project-testing/SKILL.md`.
- Specs: no update needed; this is validation policy, not product behavior.
- End-user/operator docs: no update needed; no operator behavior changed.
- End-user/operator skills: no update needed.
- SOW lifecycle: completed; moved to `done/`.

Specs update:

- None needed.

Project skills update:

- Updated `.agents/skills/project-testing/SKILL.md`.

End-user/operator docs update:

- None needed.

End-user/operator skills update:

- None needed.

Lessons:

- For small nested Go modules, separate race and coverage gates are clearer
  than trying to merge coverage profiles across modules.

Follow-up mapping:

- None.

## Outcome

Completed.

## Lessons Extracted

Nested module coverage should stay separate unless the project later has a
clear reason to publish one aggregate coverage number.

## Followup

None yet.
