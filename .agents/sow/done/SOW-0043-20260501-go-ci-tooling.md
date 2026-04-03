# SOW-0043 | 2026-05-01 | go-ci-tooling

## Status

completed

## Requirements

### Purpose

Implement SOW-0038 Decision 2(b): add Go CI tooling that improves security and
static-analysis confidence without freezing development on first adoption.

### User request quoted verbatim

> Reviewers have created SOWs 38-41 as a follow up work on 31-34.

### Assistant understanding

- SOW-0042 is complete; this SOW was the next focused SOW at the time of
  implementation and is now completed.
- Implement Go CI tooling without hiding existing code-quality debt.
- `govulncheck` should be blocking because a vulnerability finding is a
  release-quality/security issue and the first run is clean.
- `staticcheck` and `golangci-lint` should be advisory first because first
  adoption reveals a real existing backlog that would otherwise freeze all CI.
- `goleak` is not enabled globally in this SOW; it needs per-package ownership
  inside Go test hardening work because this daemon intentionally owns
  long-lived background workers.

### Acceptance criteria

- Tool versions and install paths are pinned or documented.
- CI behavior is explicit: blocking versus advisory.
- Initial findings are triaged and either fixed, documented, or scoped out.
- Project testing/reviewing skills are updated with the final gates.

## Analysis

Facts:

- Existing CI already ran build/test/race/vet/cross-build/coverage and UI
  checks, but had no Go vulnerability check, Staticcheck gate, or golangci-lint
  gate (`.github/workflows/ci.yml` before this SOW).
- The root module and nested `tools/dronebl2ipsets` module are separate Go
  modules. Root `go test ./...` does not enter the nested module, so any
  analysis gate must explicitly run both.
- Official Go documentation recommends `govulncheck` for source-level
  vulnerability analysis (`https://go.dev/doc/tutorial/govulncheck`).
- Tool versions were checked through Go module version discovery on
  2026-05-01:
  - `golang.org/x/vuln/cmd/govulncheck@v1.3.0`
  - `honnef.co/go/tools/cmd/staticcheck@v0.7.0`
  - `github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4`
- Blocking first run:
  - `make vulncheck` passed for both root and `tools/dronebl2ipsets` with
    "No vulnerabilities found."
- Advisory first run:
  - `make lint-advisory` returned success as intended.
  - Underlying Staticcheck output is not clean. Representative classes:
    `SA1012` nil context usage, `SA5011` possible nil dereference in
    `pkg/cache/cache.go`, `U1000` unused helpers, `ST1005`, `SA4006`, `S1002`,
    and `S1017`.
  - Underlying golangci-lint output is not clean: 94 findings
    (`errcheck`: 50, `ineffassign`: 6, `staticcheck`: 16, `unused`: 22).
  - Nested `tools/dronebl2ipsets` is included by the new targets; its first
    adoption findings include unchecked close and ineffectual assignment.

Inference:

- `govulncheck` can be blocking immediately because the repository is clean.
- Staticcheck/golangci-lint should start advisory because they expose a real
  backlog outside the narrow CI-tooling change. Making them blocking in this
  SOW would convert "add visibility" into an unplanned broad code cleanup.
- The advisory backlog is not a non-issue. It needs a concrete cleanup SOW so
  "advisory" does not become a permanent ignored signal.

## Implications and Decisions

Autonomous maintainer decision from SOW-0038: implement option 2(b).

- `govulncheck`: blocking in CI and Makefile.
- `staticcheck`: advisory in CI; direct Makefile target fails when findings
  exist so local users can opt into strict behavior.
- `golangci-lint`: advisory in CI; direct Makefile target fails when findings
  exist so local users can opt into strict behavior.
- `lint-advisory`: local helper that runs both advisory tools but returns
  success, matching first-adoption CI posture.
- `goleak`: scoped out of this SOW and left to Go testing hardening. Reason:
  leak tests require per-package lifecycle control and should not be bolted
  onto daemon packages mechanically.
- Advisory backlog cleanup: captured as `SOW-0045-20260501-go-advisory-lint-backlog.md`.
  `SOW-0044` remains the next focused SOW because it was already pending from
  SOW-0038 and overlaps the largest `errcheck`/error-path class; SOW-0045 then
  owns remaining advisory cleanup and blocking-graduation decisions.

## Plan

- Add pinned Makefile targets for vulnerability and advisory analysis.
- Add CI steps with explicit blocking/advisory behavior.
- Update project testing/reviewing skills so future work knows the gates.
- Run blocking checks and advisory checks.
- Record first-adoption backlog and create a concrete follow-up SOW.

## Execution Log

- Created as concrete follow-up from SOW-0038 so CI-tooling work is not lost.
- Added version-pinned analysis tool variables and targets in `Makefile`.
- Added CI steps in `.github/workflows/ci.yml`:
  - `Govulncheck` blocking.
  - `Staticcheck (advisory)` with `continue-on-error: true`.
  - `golangci-lint (advisory)` with `continue-on-error: true`.
- Updated `.agents/skills/project-testing/SKILL.md` with canonical commands,
  pinned tool versions, nested-module coverage, and CI behavior.
- Updated `.agents/skills/project-reviewing/SKILL.md` so reviews account for
  blocking/advisory Go analysis gates.
- Created follow-up, now completed:
  `.agents/sow/done/SOW-0045-20260501-go-advisory-lint-backlog.md`.

## Validation

- `make test` passed.
- `make lint` passed.
- `make vulncheck` passed for root and `tools/dronebl2ipsets`.
- `make lint-advisory` passed as an advisory helper while logging the known
  Staticcheck/golangci-lint backlog.
- `git diff --check` passed.

## Outcome

Go CI now has explicit security and advisory analysis gates:

- `make vulncheck` runs `govulncheck` for the root module and
  `tools/dronebl2ipsets`.
- `make staticcheck` runs Staticcheck for both Go modules.
- `make golangci-lint` runs golangci-lint for both Go modules.
- `make lint-advisory` runs the advisory tools without blocking first-adoption
  workflows.
- GitHub Actions runs `govulncheck` as a blocking gate and runs Staticcheck
  and golangci-lint as advisory gates.
- Project testing/reviewing skills now document the gates and the known
  advisory-first posture.

## Lessons Extracted

- First-adoption lint gates must be explicit about strict versus advisory
  behavior. A target that fails (`make staticcheck`, `make golangci-lint`) is
  useful for local strict checks; a target that does not block
  (`make lint-advisory`) is useful for first-adoption visibility.
- Advisory findings are still product debt. Every advisory backlog discovered
  by a SOW must either be fixed inside scope or mapped to a concrete pending
  SOW before the parent SOW closes.

## Followup

- `.agents/sow/done/SOW-0045-20260501-go-advisory-lint-backlog.md` tracks
  the Staticcheck/golangci-lint backlog and the decision about when advisory
  gates can graduate to blocking.
