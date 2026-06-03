# SOW-0102 - Quality Complexity Duplication Coverage

## Status

Status: in-progress

Sub-state: first implementation slice validated; next targets mapped

## Requirements

### Purpose

Improve production readiness before first production rollout by reducing measured complexity and duplication, increasing meaningful coverage, and keeping defensive scanner posture clean.

### User Request

The user asked to work autonomously on reducing complexity, reducing duplication, and improving coverage, with no user decisions required for behavior-preserving quality improvements.

### Assistant Understanding

Facts:

- Codacy Cloud reports `issuesCount: 0` for `firehol/update-ipsets` on `main` at commit `dcd1520d0f8b8f3681b9cce73533e056ab2cd86c`.
- Codacy Cloud reports coverage `65%`, complex files `25%`, and duplication `14%`.
- Codacy Cloud gate goals include minimum coverage `60%`, maximum complex files `10%`, and maximum duplicated files `10%`.
- GitHub Code Scanning currently has no open alerts from the paginated API query.
- The repository has local structural tooling under `tools/archposture`.
- The root Go coverage gate writes `coverage.out`; the nested DroneBL tool coverage gate writes `tools/dronebl2ipsets/coverage.out`.

Inferences:

- The active quality gap is no longer scanner issues; it is structural quality and test depth.
- The safest first pass is to target measured hotspots with small behavior-preserving refactors and black-box tests.
- Reducing Codacy's repository-wide percentages may require more than one focused PR because the repository is large and Codacy metrics update only after main analysis.

Unknowns:

- Whether this first focused slice is large enough to move Codacy's repository-wide structural percentages after merge is unknown until Codacy reanalyzes `main`.

### Acceptance Criteria

- Baseline coverage, complexity, and duplication signals are recorded before code edits.
- At least one measured structural hotspot is simplified or one meaningful coverage gap is covered with behavior-focused tests.
- Any refactor preserves public behavior and project contracts.
- Local validation for touched surfaces passes.
- Codacy and GitHub scanner posture remains clean or any new finding is fixed before closure.

## Analysis

Sources checked:

- `codacy repository gh firehol update-ipsets --output json`
- `codacy issues gh firehol update-ipsets --branch main --overview --output json`
- `gh api --method GET repos/firehol/update-ipsets/code-scanning/alerts -f state=open --paginate`
- `codacy-analysis analyze --inspect --output-format json --output /tmp/update-ipsets-codacy-inspect.json`
- `.agents/skills/project-coding/SKILL.md`
- `.agents/skills/project-testing/SKILL.md`
- `.agents/skills/project-hygiene/SKILL.md`
- `.agents/skills/project-go-best-practices/SKILL.md`
- `.agents/skills/project-go-behavioral-testing/SKILL.md`
- `.agents/skills/project-content-surfaces/SKILL.md`
- `tools/archposture/`
- `Makefile`
- `.codacy.yml`
- `.github/workflows/ci.yml`

Current state:

- Codacy Cloud has `0` open issues but structural metrics are above the configured goals for complexity and duplication.
- `.codacy.yml` excludes Codacy `lizard` from source paths, so Codacy's complex-file percentage is a repository metric rather than an actionable issue list.
- CI uploads Go coverage to Codacy on trusted pushes to `main` or `master`.
- UI has a `test:coverage` script and Vitest V8 coverage config, but Codacy coverage upload currently covers Go reports only.
- Local `tools/archposture` reports the largest production functions as:
  - `pkg/engine/entity_integrity.go:189` `(*Engine).CheckEntityArtifactsIntegrity`: 449 lines, complexity 61.
  - `pkg/iprange/cli.go:25` `runCLIV4`: 381 lines, complexity 115.
  - `pkg/iprange/cli6.go:10` `runCLIV6`: 374 lines, complexity 115.
  - `pkg/engine/output.go:336` `(*Engine).writeComparisonFiles`: 295 lines, complexity 55.
  - `pkg/engine/entity_artifacts.go:395` `(*Engine).writeEntityArtifacts`: 290 lines, complexity 78.
- `go test -coverprofile=/tmp/update-ipsets-iprange.cover -covermode=atomic ./pkg/iprange` reports package coverage `49.6%`.
- `go tool cover` reports `runCLIV4` at `0.0%`, `runCLIV6` at `10.0%`, `loadInputArgument` at `0.0%`, `loadInputArgument6` at `0.0%`, `loadSinglePath` at `0.0%`, `loadSinglePath6` at `0.0%`, and `printCLIUsage` at `0.0%`.

Risks:

- Broad refactors can introduce behavior changes in feed generation, public serving, or admin operation paths.
- Coverage-only tests can create weak confidence if they test private implementation details instead of public contracts.
- Codacy structural percentages update only after main analysis, so local validation may prove the change before the dashboard percentage moves.
- Duplication removal can accidentally merge logic that should stay separate because different product contracts happen to look similar.

## Pre-Implementation Gate

Status: ready.

Problem / root-cause model:

- Facts: Codacy reports no open issues and no current code-scanning alerts, while repository-level complexity and duplication metrics exceed Codacy goals.
- Working theory: the first low-risk improvement should target the standalone `pkg/iprange` CLI because two large parallel CLI functions have high complexity and poor direct coverage while the package is isolated from the daemon pipeline.
- Evidence: Codacy repository metrics show coverage `65%`, complex files `25%`, duplication `14%`, with goals of coverage at least `60%`, complex files at most `10%`, and duplicated files at most `10%`; local posture reports `runCLIV4` and `runCLIV6` as 381/374-line functions with complexity 115; local coverage reports `runCLIV4` at `0.0%` and `runCLIV6` at `10.0%`.

Evidence reviewed:

- Codacy Cloud repository and issue overview output.
- GitHub Code Scanning open-alert API output.
- Local `Makefile` coverage and validation targets.
- Local `tools/archposture` source and baseline file.
- Project coding, testing, hygiene, and content-surface skills.

Affected contracts and surfaces:

- Go package behavior and tests under `cmd/`, `internal/`, `pkg/`, and `tools/`.
- First slice scope: `pkg/iprange/cli.go`, `pkg/iprange/cli6.go`, `pkg/iprange/cli_family.go`, `pkg/iprange/cli_inputs.go`, `pkg/iprange/cli_inputs6.go`, and `pkg/iprange/cli_test.go`.
- Coverage-only leaf package test scope: `internal/telemetry`, `pkg/enrichment`, and `pkg/runreason`.
- User/operator contract: `update-ipsets iprange` documented CLI flags, exit codes, stdout/stderr behavior, IPv4/IPv6 mode selection, stdin/file/list/directory inputs, and CSV headers.
- SOW and possibly project skills if durable process lessons are found.

Existing patterns to reuse:

- `tools/archposture` for local large-file and large-function posture.
- `make coverage`, `make coverage-tools`, `go tool cover -func`, and `pnpm --dir ui test:coverage` for coverage evidence.
- External-package Go tests by default; same-package tests only for true internal invariants.
- Project behavior-first fixtures such as `t.TempDir`, `httptest`, table-driven tests, and package-local test harnesses.
- Existing `RunCLI` tests in `pkg/iprange/cli_test.go`.
- Existing standalone-package rule: `pkg/iprange` must not import other project packages.
- Existing docs in `docs/cli/iprange-command.md` define the CLI behavior to preserve.

Risk and blast radius:

- First pass is constrained to small, behavior-preserving refactors and coverage additions.
- The selected first slice affects only the standalone `iprange` CLI path called by `cmd/update-ipsets/main.go`.
- Public serving, pipeline artifact generation, scheduler queues, install behavior, and config semantics must not change.
- Generated frontend assets and published static bundle files are out of scope for manual edits.

Sensitive data handling plan:

- This work should not require secrets, customer data, private endpoints, or personal data.
- Codacy and GitHub outputs recorded in durable artifacts will be limited to metrics, rule states, file paths, line numbers, and commit IDs.
- No tokens, cookies, session values, or raw sensitive scanner payloads will be written to SOWs, specs, docs, skills, instructions, code comments, or commits.

Implementation plan:

1. Refactor `runCLIV4` and `runCLIV6` by extracting argument parsing and mode execution helpers inside `pkg/iprange`, preserving all flag names, stdout/stderr text, and exit-code behavior.
2. Add behavior-focused CLI tests through `RunCLI`, covering IPv4 and IPv6 count/compare/diff/exclude/common modes, alias handling, stdin/file-list/directory inputs, feature-detection flags, and representative invalid-option paths.
3. Extract shared CLI input expansion where IPv4 and IPv6 behavior is identical, while keeping family-specific parsing separate.
4. Add behavior-focused tests for zero-coverage leaf packages with clear exported contracts: `internal/telemetry`, `pkg/enrichment`, and `pkg/runreason`.
5. Re-run local `pkg/iprange` coverage and `tools/archposture` to prove the selected hotspot improved locally.
6. Run relevant root tests/static checks for the touched package and full validation as practical before closure.
7. If pushed and merged, verify Codacy and GitHub scanner state after analysis.

Validation plan:

- Run `go test ./pkg/iprange`.
- Run `go test -coverprofile=/tmp/update-ipsets-iprange.cover -covermode=atomic ./pkg/iprange` and inspect `go tool cover -func`.
- Run `go run ./tools/archposture -root .` and inspect large-function output for the touched functions.
- Run `make lint` or at minimum `go vet ./pkg/iprange`.
- Run `make coverage` or package-specific coverage commands for changed Go code as time permits.
- Run `make coverage-tools` if the nested tool is touched.
- Run relevant static checks from `project-testing` and `project-hygiene` for touched surfaces.
- Re-check Codacy repository metrics and GitHub open code-scanning alerts before closure if the work reaches `main`.

Artifact impact plan:

- AGENTS.md: no change expected unless a project-wide rule changes.
- Runtime project skills: update only if a repeatable coverage/complexity lesson is found.
- Specs: no change expected for behavior-preserving refactors and tests.
- End-user/operator docs: no change expected.
- End-user/operator skills: no change expected.
- SOW lifecycle: this SOW remains in `.agents/sow/current/` until implementation, validation, and closure are complete.

Open-source reference evidence:

- None checked yet. The first pass is local structural/test improvement rather than new external behavior, protocol, or library integration.

Open decisions:

- User decision recorded: behavior-preserving quality, complexity, duplication, and coverage improvements may proceed without additional user approval.
- No blocking design decision is currently known.

## Implications And Decisions

1. Scope autonomy
   - Decision: proceed autonomously on behavior-preserving quality improvements.
   - Evidence: user explicitly said the assistant does not need the user for reducing complexity, reducing duplication, and improving coverage.
   - Implication: small local refactors and tests can proceed after evidence collection.
   - Risk: larger behavior or architecture changes still need a new decision if discovered.

## Plan

1. Baseline local and remote quality metrics.
2. Pick the safest measured hotspot.
3. Implement one focused improvement with tests.
4. Validate and check scanners.
5. Update SOW, skills/specs only if needed, and close through the normal SOW lifecycle.

## Execution Log

### 2026-06-03

- Created SOW before code edits.
- Recorded Codacy and GitHub scanner baseline evidence.
- Refactored the standalone `pkg/iprange` CLI:
  - `runCLIV4` and `runCLIV6` now delegate to IPv4/IPv6 runners.
  - shared CLI option dispatch lives in `pkg/iprange/cli_options.go`.
  - shared runner state lives in `pkg/iprange/cli_runner.go`.
  - shared path expansion for stdin, `@filelist`, `@directory`, and direct files lives in `expandCLIInputPaths`.
- Added `RunCLI` behavioral tests for IPv4 and IPv6 documented modes, aliases, feature flags, stdin/file/list/directory inputs, representative validation errors, and quiet diff exit behavior.
- Added coverage tests for exported behavior in `internal/telemetry`, `pkg/enrichment`, and `pkg/runreason`.
- Updated `.agents/skills/project-hygiene/SKILL.md` with the local `archposture` and source-only `jscpd` structural-quality checks.

## Validation

Acceptance criteria evidence:

- Baseline recorded:
  - Codacy Cloud `main` at `dcd1520d0f8b8f3681b9cce73533e056ab2cd86c`: `issuesCount: 0`, coverage `65%`, complex files `25%`, duplication `14%`.
  - Codacy goals: coverage at least `60%`, complex files at most `10%`, duplicated files at most `10%`.
  - Local root coverage before the final coverage additions was `70.0%`.
  - Local `pkg/iprange` coverage baseline was `49.6%`.
  - `runCLIV4`: 381 lines, complexity 115, coverage `0.0%`.
  - `runCLIV6`: 374 lines, complexity 115, coverage `10.0%`.
  - `tools/archposture` baseline: source files `587`, source lines `122880`, large files `59`, large functions `49`.
  - Codacy Cloud pre-edit duplication baseline was `14%`.
  - Source-only `jscpd` first actionable local scan during this slice found `14` clones, `676` duplicated lines, and `334` duplicated Go lines.
- Final local results:
  - Root coverage is `70.8%`.
  - `pkg/iprange` coverage is `67.3%`.
  - `internal/telemetry` coverage is `91.9%`.
  - `pkg/enrichment` coverage is `94.3%`.
  - `pkg/runreason` coverage is `100.0%`.
  - `runCLIV4` and `runCLIV6` coverage are both `100.0%`.
  - `tools/archposture` final: source files `593`, source lines `123741`, large files `59`, large functions `47`.
  - No `pkg/iprange/cli*` files or functions remain in the `tools/archposture` large-file/large-function lists.
  - Source-only `jscpd` final found `12` clones, `559` duplicated lines, and `217` duplicated Go lines.
- Scanner posture:
  - Codacy Cloud still reports `issuesCount: 0` on the latest analyzed `main` commit; structural percentages still reflect remote `main`, not these local changes.
  - GitHub Code Scanning open alerts: `[]`.
  - GitHub secret-scanning open alerts: `[]`.
  - GitHub Dependabot open alerts: `[]`.

Tests or equivalent validation:

- `go test ./pkg/iprange`: passed.
- `go vet ./pkg/iprange`: passed.
- `go test -coverprofile=/tmp/update-ipsets-iprange-final4.cover -covermode=atomic ./pkg/iprange`: passed, `67.3%`.
- `go test ./internal/telemetry ./pkg/runreason ./pkg/enrichment`: passed.
- `go test -coverprofile=/tmp/update-ipsets-small-coverage.cover -covermode=atomic ./internal/telemetry ./pkg/runreason ./pkg/enrichment`: passed.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-final5.json`: passed.
- `npx --yes jscpd@4.2.4 --reporters json,console --output /tmp/update-ipsets-jscpd-src-final4 --exitCode 0 --min-lines 20 --min-tokens 120 --max-lines 2000 --max-size 512kb --ignore '**/*_test.go,pkg/web/static/**,ui/dist/**,ui/node_modules/**,.agents/**,tools/archposture/testdata/**' cmd internal pkg tools ui/src .github/scripts`: passed.
- `make lint`: passed.
- `make staticcheck`: passed.
- `make golangci-lint`: passed with `0 issues`.
- `make coverage`: passed.
- `quick_validate.py .agents/skills/project-hygiene`: passed.
- `git diff --check -- ...`: passed for changed files.
- `codacy-analysis analyze --inspect --output-format json --output /tmp/update-ipsets-codacy-inspect.json`: completed but reported `MissingConfig` because this repository does not currently have `.codacy/codacy.config.json`; not used as a code-validation signal.

Real-use evidence:

- The behavioral tests execute the public `RunCLI` entrypoint, validating stdout, stderr, and exit codes for documented operator-visible CLI modes.
- No daemon, scheduler, public serving, admin UI, install, or runtime artifact generation code was changed.

Reviewer findings:

- No external reviewer was run for this first local quality slice.

Same-failure scan:

- Source-only `jscpd` was rerun after the duplication refactors. The original `pkg/iprange/cli.go` versus `pkg/iprange/cli6.go` parser clone and the `cli_inputs.go` versus `cli_inputs6.go` loader clone no longer appear.
- Remaining measured duplicate blocks are mapped as follow-up targets below.

Sensitive data gate:

- Passed. No secrets, tokens, cookies, private endpoints, personal data, or customer data were written to durable artifacts.

Artifact maintenance gate:

- AGENTS.md: no update needed; no project-wide workflow or responsibility rule changed.
- Runtime project skills: updated `.agents/skills/project-hygiene/SKILL.md` with durable local structural-quality checks.
- Specs: no update needed; implementation preserves behavior and does not change product contracts.
- End-user/operator docs: no update needed; CLI semantics did not change.
- End-user/operator skills: no update needed.
- SOW lifecycle: remains in `.agents/sow/current/`; first slice is validated but broader quality work continues.

Specs update:

- None needed.

Project skills update:

- `.agents/skills/project-hygiene/SKILL.md` now requires local `tools/archposture` and source-only `jscpd` checks for structural quality work.

End-user/operator docs update:

- None needed.

End-user/operator skills update:

- None needed.

Lessons:

- Codacy's repository-level complexity/duplication percentages are useful gates but not a sufficient file-level work queue. Local `tools/archposture` and source-only `jscpd` provide actionable before/after evidence.
- `pkg/iprange` has enough CLI surface area that behavior-preserving refactors need stdout/stderr/exit-code tests through `RunCLI`, not private parser tests.

Follow-up mapping:

- Next measured complexity targets:
  - `pkg/engine/entity_integrity.go:189` `(*Engine).CheckEntityArtifactsIntegrity`: 449 lines, complexity 61.
  - `pkg/engine/output.go:336` `(*Engine).writeComparisonFiles`: 295 lines, complexity 55.
  - `pkg/engine/entity_artifacts.go:395` `(*Engine).writeEntityArtifacts`: 290 lines, complexity 78.
  - `pkg/engine/entity_surgical.go:66` `(*Engine).refreshEntityArtifactsForFeedUpdates`: 267 lines, complexity 68.
- Next measured duplication targets:
  - `ui/src/components/admin/feeds-table-header.tsx`: 130 duplicated lines.
  - `ui/src/pages/asn-detail.tsx` and `ui/src/pages/country-detail.tsx`: 20, 35, and 107 duplicated line blocks.
  - `ui/src/components/feed-detail/asn-bubble-chart.tsx` and `ui/src/components/feed-detail/asn-treemap.tsx`: 50 duplicated lines.
  - `pkg/iprange/set6_ops.go` and `pkg/iprange/set_ops.go`: 25 duplicated lines.
  - `pkg/engine/runtime_ledger_cache.go`, `pkg/engine/home_entity_precompute.go`, `pkg/engine/home_entity_builders.go`, and `pkg/engine/download_stage.go`: remaining Go duplicate blocks.

## Outcome

First implementation slice is complete and validated locally. The SOW remains open for the next focused complexity/duplication slice.

## Lessons Extracted

Update project hygiene practice to always pair Codacy Cloud metrics with local actionable structural scans before changing code.

## Followup

Continue with one focused slice at a time, starting with either UI duplicate pages/components or the engine artifact-integrity/write paths after a fresh pre-implementation gate update for that chosen surface.

## Regression Log

None yet.
