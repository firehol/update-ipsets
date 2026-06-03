# SOW-0101 - Codacy Coverage Upload

## Status

Status: in-progress

Sub-state: approved option 1A; implementing main/master push-only Codacy coverage upload.

## Requirements

### Purpose

Make Codacy coverage metrics work for the repository without exposing Codacy credentials to untrusted pull-request contexts.

### User Request

Explain why Codacy coverage is not updated, then implement the approved option `1A`: upload coverage to Codacy on `push` to `main`/`master` only.

### Assistant Understanding

Facts:

- The CI workflow already generates Go coverage for the root module and nested DroneBL tool module.
- The CI workflow checks coverage thresholds but does not run Codacy Coverage Reporter.
- The repository has a GitHub Actions secret named `CODACY_API_TOKEN`.
- The existing Codacy workflow uses `CODACY_API_TOKEN` for issue/SARIF export, not coverage upload.
- Official Codacy documentation says Go coverage uploads must specify `--force-coverage-parser go`.

Inferences:

- Codacy coverage is absent because no CI step uploads the generated coverage reports.
- Uploading coverage only on trusted default-branch pushes avoids exposing secrets to fork, Dependabot, or pull-request contexts.

Unknowns:

- Codacy's UI processing result can only be verified after the workflow runs on the pushed commit.

### Acceptance Criteria

- CI uploads root Go coverage and nested tool Go coverage to Codacy only on trusted `push` events to `main` or `master`.
- Pull-request CI remains safe and does not require Codacy coverage secrets.
- Local workflow validation passes.
- The SOW records the coverage-upload decision, validation, and any remaining verification gap.

## Analysis

Sources checked:

- `.github/workflows/ci.yml`
- `Makefile`
- `.github/workflows/codacy-sarif.yml`
- GitHub Actions secret list
- Codacy setup-coverage skill
- Official Codacy coverage documentation:
  - `https://docs.codacy.com/coverage-reporter/`
  - `https://docs.codacy.com/coverage-reporter/uploading-coverage-in-advanced-scenarios/`

Current state:

- `.github/workflows/ci.yml` has a `CI coverage` job that runs `make coverage`, checks the root threshold, runs `make coverage-tools`, and checks the nested tool threshold.
- `Makefile` writes root coverage to `coverage.out` and nested tool coverage to `tools/dronebl2ipsets/coverage.out`.
- No workflow step invokes `coverage.codacy.com/get.sh` or a Codacy coverage reporter.
- `CODACY_API_TOKEN` is present as a GitHub Actions secret.

Risks:

- A coverage upload step that runs on pull requests can fail because GitHub does not expose secrets to untrusted PR contexts.
- A coverage upload step that runs before Codacy has analyzed the commit can appear delayed in Codacy; Codacy documentation notes coverage is reported after analysis completes.
- If coverage report paths do not map as Codacy expects, the upload may process but show pending or incomplete coverage.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- CI generates coverage reports but only checks thresholds locally. Codacy has no coverage because no CI step uploads those reports.

Evidence reviewed:

- `.github/workflows/ci.yml` coverage job lines running `make coverage` and `make coverage-tools`.
- `Makefile` coverage targets producing `coverage.out` files.
- `.github/workflows/codacy-sarif.yml` showing the existing token is scoped to issue/SARIF export.
- Official Codacy coverage docs requiring Coverage Reporter upload and Go parser selection.

Affected contracts and surfaces:

- GitHub Actions CI workflow.
- Codacy Cloud coverage metrics for default-branch commits.
- Main ruleset required `CI coverage` check, because this workflow job must continue to pass.

Existing patterns to reuse:

- Keep coverage in the existing `CI coverage` job so reports are uploaded only after threshold checks pass.
- Use the existing account-token environment-variable form already approved by the user.

Risk and blast radius:

- CI-only change; no runtime product behavior changes.
- Main/master pushes will fail `CI coverage` if the Codacy upload fails.
- Pull requests remain unaffected by the upload because the step is push-only.

Sensitive data handling plan:

- Do not write token values to repository files, SOWs, logs, docs, skills, or comments.
- Reference only the secret name `CODACY_API_TOKEN` and non-sensitive provider/repository identifiers.

Implementation plan:

1. Add a Codacy Coverage Reporter upload step after both coverage threshold checks in `.github/workflows/ci.yml`.
2. Gate the upload step to `push` events on `refs/heads/main` and `refs/heads/master`.
3. Validate workflow syntax with `make actionlint` and `git diff --check`.

Validation plan:

- `make actionlint`
- `git diff --check -- .github/workflows/ci.yml .agents/sow/current/SOW-0101-20260603-codacy-coverage-upload.md`
- Post-push GitHub Actions evidence after merge.
- Codacy coverage UI/API evidence after Codacy processes the uploaded report.

Artifact impact plan:

- AGENTS.md: no update expected; existing hygiene and SOW rules cover this.
- Runtime project skills: possible hygiene/testing skill update only if validation reveals a durable lesson.
- Specs: no product spec update; CI coverage upload does not change product behavior.
- End-user/operator docs: no update expected.
- End-user/operator skills: no update expected.
- SOW lifecycle: complete and move to `.agents/sow/done/` with the workflow change if validation passes.

Open-source reference evidence:

- No mirrored source repositories checked. Official Codacy documentation is the authoritative source for this integration.

Open decisions:

- Resolved by user: option `1A`, upload on `push` to `main`/`master` only.

## Implications And Decisions

1. Coverage upload scope
   - Option A: upload only on `push` to `main`/`master`.
     - Pros: trusted context, secrets available, fixes main branch Codacy coverage.
     - Cons: PR coverage may not appear until main has uploaded enough baseline commits.
     - Risks: Codacy PR coverage may stay limited for untrusted PRs.
   - Option B: upload on push plus trusted same-repo PRs.
     - Pros: better PR coverage signal for trusted branches.
     - Cons: more complex secret gating.
     - Risks: accidental secret exposure or CI failures in PR contexts if gating is wrong.
   - Option C: upload on all PRs.
     - Pros: broadest coverage attempt.
     - Cons: secrets are unavailable for forks and Dependabot.
     - Risks: broken PR checks and unsafe credential design.
   - User decision: Option A.

## Plan

1. Patch `.github/workflows/ci.yml` with the push-only Codacy coverage upload step.
2. Validate workflow syntax and diff hygiene.
3. Commit, push via a PR because `main` is ruleset-protected, and verify the workflow after merge.

## Execution Log

### 2026-06-03

- Created this SOW and recorded the approved upload-scope decision before implementation.
- Added a Codacy Coverage Reporter upload step to `CI coverage`, gated to
  trusted pushes on `refs/heads/main` and `refs/heads/master`.
- The step uploads both root `coverage.out` and
  `tools/dronebl2ipsets/coverage.out` using Codacy's documented Go parser
  flag, `--force-coverage-parser go`.
- Used `curl -fsSL` when fetching the official Codacy Coverage Reporter script
  so HTTP and network failures stop the step before bash executes incomplete
  input.

## Validation

Acceptance criteria evidence:

- `CI coverage` now generates both coverage reports, checks thresholds, and
  uploads both reports to Codacy only on push events to `main` or `master`.
- Pull-request runs skip the upload step because the `if:` condition requires
  `github.event_name == 'push'`.
- `gh secret list --repo firehol/update-ipsets` reports `CODACY_API_TOKEN`
  present. The token value was not read or written.

Tests or equivalent validation:

- `make actionlint`: passed.
- `git diff --check -- .github/workflows/ci.yml .agents/sow/current/SOW-0101-20260603-codacy-coverage-upload.md`:
  passed.

Real-use evidence:

- Pending until the workflow runs on a pushed `main` commit.

Reviewer findings:

- Pending.

Same-failure scan:

- `rg -n "codacy|coverage|get\\.sh|CODACY|coverage.out|lcov|cobertura|upload" .github Makefile ui/package.json package.json go.mod tools/dronebl2ipsets/go.mod`:
  confirmed there was no existing Codacy coverage upload step before this
  patch.

Sensitive data gate:

- Durable artifacts reference only the GitHub secret name
  `CODACY_API_TOKEN` and non-sensitive repository identifiers. No token value
  or credential payload was read or recorded.

Artifact maintenance gate:

- AGENTS.md: no update needed; existing hygiene and SOW rules cover scanner
  posture and secret handling.
- Runtime project skills: no update needed unless post-merge validation shows a
  durable Codacy coverage lesson.
- Specs: no update needed; product behavior is unchanged.
- End-user/operator docs: no update needed; this is CI/Codacy integration.
- End-user/operator skills: no update needed.
- SOW lifecycle: remains current until post-merge Codacy coverage evidence is
  available.

Specs update:

- None; product behavior is unchanged.

Project skills update:

- None yet.

End-user/operator docs update:

- None.

End-user/operator skills update:

- None.

Lessons:

- Pending post-merge validation.

Follow-up mapping:

- Post-merge Codacy coverage processing must be verified before this SOW is
  completed.

## Outcome

Pending.

## Lessons Extracted

Pending.

## Followup

None yet.

## Regression Log

None yet.

Append regression entries here only after this SOW was completed or closed and later testing or use found broken behavior. Use a dated `## Regression - YYYY-MM-DD` heading at the end of the file. Never prepend regression content above the original SOW narrative.
