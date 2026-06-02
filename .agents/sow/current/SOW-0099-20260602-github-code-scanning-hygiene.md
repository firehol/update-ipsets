# SOW-0099 - GitHub Code Scanning And Hygiene Configuration

## Status

Status: in-progress

Sub-state: approved; implementation starting with strict scanner policy for an AI-generated repository.

## Requirements

### Purpose

Make the repository's GitHub security and hygiene scanning fit production-grade open-source maintenance for an AI-generated, fully automated repository without human review as the normal safety net. Scanner policy should bias toward early defensive detection, clear scanner ownership, visible findings, repeatable repo-controlled configuration where useful, and enforcement that reduces the chance of generated code reaching `main` with security or quality regressions.

### User Request

Configure CodeQL and other GitHub code scanning / hygiene scanners properly, review the findings visible in GitHub, and choose scanner/enforcement policy with the explicit goal of reducing quality/security risk in an AI-generated repository that normally lands changes without human reviews.

The user later added the repository to Codacy and reported 8.8k Codacy issues,
with the goal of eventually making Codacy clean. The user also pointed to
GitHub Security and quality AI findings and to `codacy/codacy-skills`, and
asked that the project hygiene skill stay current so future agents discover
these finding surfaces without needing direct links from the user.

### Lifecycle Note - 2026-06-02

The user redirected active work to SOW-0016 before choosing scanner/enforcement
options. This SOW is parked in `pending` and should resume only after the user
returns to GitHub scanner configuration decisions.

The user returned to scanner decisions on 2026-06-02 and clarified that this is
an AI-generated, fully automated repository without human reviews. The scanner
policy should therefore favor earlier and broader defensive detection, with
enforcement once workflow names and false-positive baselines are stable.

### Assistant Understanding

Facts:

- The repository currently has GitHub CodeQL default setup enabled dynamically, not via a checked-in workflow.
- The user states this repository is AI-generated and fully automated without human reviews, so scanners are a primary quality/security control rather than a secondary review aid.
- GitHub secret scanning and push protection are enabled.
- Dependabot security updates and dependency graph are enabled.
- There are no open GitHub CodeQL, Dependabot, or secret-scanning alerts at the time of analysis.
- Historical CodeQL alerts on this repository are all fixed, not dismissed.
- The repository has no checked-in `.github/dependabot.yml`.
- The default branch has no branch protection and no repository rulesets.
- Local ShellCheck currently reports warnings/info findings on tracked shell scripts.
- Local actionlint reports no workflow findings.
- Local redacted gitleaks scan reports no leaks.

Inferences:

- The immediate risk is not active unfixed scanner findings; it is incomplete, partly opaque scanner configuration and lack of merge enforcement in a repository that relies heavily on automation.
- Keeping GitHub CodeQL default setup is viable for low-maintenance security scanning, but it cannot run the `security-and-quality` query suite because that suite is advanced-setup only.
- Adding more scanners without first deciding enforcement can create noisy workflow failures or non-actionable Security tab alerts.

Unknowns:

- Resolved: scanners should become enforced gates for `main`, staged after workflows have stable successful check names.
- Resolved: CodeQL should use checked-in advanced setup with broader `security-and-quality` queries.

### Acceptance Criteria

- Record the chosen CodeQL setup and query scope with evidence.
- Add repo-controlled scanner workflows/configs for the chosen scanner set.
- Avoid duplicate CodeQL scans for the same languages and commits.
- Make hygiene scanners run only against tracked/relevant files, not ignored local scratch artifacts.
- Fix or explicitly baseline existing ShellCheck findings before turning ShellCheck into a blocking CI gate.
- Add Dependabot version-update configuration if chosen, covering Go modules, UI npm/pnpm, and GitHub Actions.
- Add branch/ruleset enforcement only if explicitly chosen.
- Validate workflows/config with local tooling where possible and GitHub API/run evidence after push.
- Create a durable project hygiene skill requiring future hygiene checks to
  cover the full scanner posture and resolve both blocking and non-blocking
  valid findings.
- Update the project hygiene skill so Codacy Cloud issues, Codacy tools/patterns,
  Codacy PR analysis, Codacy configuration, GitHub standard code-quality
  findings, and GitHub AI findings are mandatory hygiene surfaces.
- Record how Codacy configuration works so future work does not confuse
  repository `.codacy.yml` / `.codacy.yaml` path/language configuration with
  local Codacy Analysis CLI `.codacy/codacy.config.json` tool/pattern tuning.

## Analysis

Sources checked:

- `.github/workflows/ci.yml`
- `.github/workflows/wiki-sync.yml`
- `Makefile`
- `go.mod`
- `tools/dronebl2ipsets/go.mod`
- `ui/package.json`
- `.gitignore`
- GitHub API: workflows, CodeQL default setup, code scanning alerts, code scanning analyses, Dependabot alerts, secret scanning alerts, repository security settings, rulesets, branch protection, Actions permissions.
- GitHub Docs: CodeQL default setup, CodeQL query suites, CodeQL workflow configuration, CodeQL compiled-language build modes, SARIF upload, dependency review, Dependabot options, secret scanning, code scanning merge protection.
- OpenSSF Scorecard action documentation.

Current state:

- `.github/workflows/ci.yml` already runs build, Go tests, UI tests, UI e2e, UI lint, Go vet, `govulncheck`, `staticcheck`, `golangci-lint`, race tests, strict tests, fuzz seed replay, nested tool tests, cross compile, and static binary checks.
- GitHub workflow list includes dynamic CodeQL at `dynamic/github-code-scanning/codeql`.
- CodeQL default setup is configured for `actions`, `go`, `javascript`, `javascript-typescript`, `python`, and `typescript`, with `query_suite: default`, `schedule: weekly`, and `threat_model: remote`.
- Current CodeQL analyses for commit `b4104e772e54351dff93c9b94cce917e27bef7b1` report zero results for Go, JS/TS, Python, and Actions.
- Open CodeQL alerts: none.
- Fixed CodeQL alerts: four Go `go/path-injection`, one Python `py/incomplete-url-substring-sanitization`, and two Actions `actions/missing-workflow-permissions`.
- Dismissed CodeQL alerts: none.
- Open Dependabot alerts: none.
- Open secret-scanning alerts: none.
- Repository security settings show secret scanning enabled, push protection enabled, non-provider patterns disabled, validity checks disabled, private vulnerability reporting enabled.
- No repository rulesets are configured.
- `main` is not branch-protected.
- Actions are enabled, allowed actions are unrestricted, and SHA pinning is not required.
- `.github/dependabot.yml` does not exist.
- `shellcheck $(git ls-files '*.sh')` currently reports warnings in `.agents/sow/audit.sh` and `scripts/sync-from-bash-version.sh`, plus informational findings.
- `go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/ci.yml .github/workflows/wiki-sync.yml` passes.
- `go run github.com/zricethezav/gitleaks/v8@latest detect --no-banner --redact=100 --source . --exit-code 2` passes with no leaks found.
- GitHub accepted a repository settings PATCH for non-provider secret patterns,
  but a follow-up settings read still reports `secret_scanning_non_provider_patterns:
  disabled`; GitHub documentation/changelog identifies this repository-level
  setting as a GitHub Advanced Security feature.
- GitHub Security and quality AI findings page reports 4 findings in 2 files:
  `pkg/web/admin_manifest.go` and `pkg/web/server.go`.
- Codacy dashboard reports Grade C and 8845 total current issues on `main`
  after analysis of commit `e2a24f095ee57662124d33770e962b230a38e98d`.
  The visible category breakdown is: Error prone about 3k, Compatibility about
  2k, Code complexity 973, Code style 916, Security 700, Best practice 492,
  Performance 122, Unused code 24, Documentation 18, and Comprehensibility 15.
- Codacy visible top patterns include strong evidence of tool/pattern mismatch
  for this repo, such as ES2015 module bans, block-scoped variable bans,
  missing React-in-JSX bans, Flowtype rules, markdown style rules, and broad
  Go file-permission rules. These need triage, not blanket dismissal.
- `codacy/codacy-skills` was cloned for reference at
  `bb6e7fc75159f360efa27d89568078272048be93`. Relevant skill guidance covers
  Codacy Cloud CLI, Codacy Analysis CLI, Codacy code review, and Codacy
  configuration/noise reduction.
- Official Codacy documentation checked on 2026-06-02 says:
  - `.codacy.yml` or `.codacy.yaml` in the repository root configures advanced
    Codacy Cloud behavior such as global/tool-specific `exclude_paths`,
    `include_paths`, languages, and base subdirectories.
  - When a repository Codacy configuration file exists, ignored-file settings
    in the Codacy UI do not apply; ignored paths must be in the file.
  - Codacy Cloud CLI can query issues/findings/PRs/tools/patterns, trigger
    reanalysis, and import tool/pattern configuration.
  - `.codacy/codacy.config.json` is the Codacy Analysis CLI local tool/pattern
    configuration; committing it alone does not change Codacy Cloud analysis.
  - `.codacy.yml` can configure tool-specific path exclusions, but it cannot
    enable or disable tools. Tools enabled by a Codacy coding standard cannot
    be disabled at repository level; changing a coding standard can affect
    every repository that follows it.
- Official pnpm documentation checked on 2026-06-02 says pnpm v11 enables
  supply-chain protection by default with `minimumReleaseAge: 1440` minutes.
  The `minimumReleaseAge` setting blocks installation of package versions
  published too recently, including transitive dependencies.
- Official GitHub Dependabot documentation checked on 2026-06-02 says
  `cooldown` delays version-update PRs; when a dependency's release is inside
  the cooldown period, Dependabot skips that version until the cooldown ends.
- Official Codacy documentation checked on 2026-06-02 says ESLint v9 config
  file detection supports `eslint.config.js`, `eslint.config.mjs`, and
  `eslint.config.cjs` at the default branch root.
- GitHub Security and quality AI findings page reports 1 current finding in
  `eslint.config.mjs` after the root ESLint config bridge commit. The finding
  warns that a direct default re-export assumes the UI config default shape and
  recommends importing the UI config into a named binding before exporting it
  as the root default config.

Risks:

- Switching to an advanced CodeQL workflow without disabling default setup can create duplicate CodeQL analyses.
- Enabling broad query suites or third-party SARIF scanners can produce noisy findings that distract maintainers.
- Enforcing branch protection/rulesets can disrupt the current direct-push workflow.
- Adding ShellCheck as a blocking gate before fixing or baselining current findings will make CI fail immediately.
- Dependabot version updates can create PR volume; grouping and scheduling are needed to keep maintenance load reasonable.
- Dependabot npm updates can create PRs that pnpm v11 CI rejects when the
  versions are newer than pnpm's default one-day release-age policy. Without a
  matching Dependabot cooldown, automated npm PRs can be born unmergeable.
- Secret scanning non-provider patterns or validity checks may increase alert volume and may require organization/plan-specific availability.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The repository has strong checked-in CI, but GitHub security scanning is partly configured outside the repository. Evidence: `.github/workflows/` contains only `ci.yml` and `wiki-sync.yml`, while GitHub API reports dynamic CodeQL at `dynamic/github-code-scanning/codeql`.
- GitHub CodeQL currently has no open findings. Evidence: code scanning alert API returned no open alerts, and the latest analyses for commit `b4104e772e54351dff93c9b94cce917e27bef7b1` show zero results for Go, JS/TS, Python, and Actions.
- Historical CodeQL findings were real and are now fixed. Evidence: fixed alert API lists four path-injection findings, one URL-sanitization finding, and two workflow permission findings, all fixed on 2026-05-31.
- The main enforcement gap is repository policy, not scanner availability. Evidence: GitHub API reports no rulesets and branch protection API reports `main` is not protected.
- Hygiene scanner readiness is mixed. Evidence: actionlint and gitleaks pass locally; ShellCheck reports current warnings/info findings.

Evidence reviewed:

- Local files: `.github/workflows/ci.yml`, `.github/workflows/wiki-sync.yml`, `Makefile`, `go.mod`, `tools/dronebl2ipsets/go.mod`, `ui/package.json`, `.gitignore`.
- GitHub API evidence from 2026-06-02 for workflows, CodeQL default setup, code scanning alerts/analyses, Dependabot alerts, secret scanning alerts, security settings, rulesets, branch protection, Actions permissions.
- GitHub Docs:
  - CodeQL default setup can choose languages and the query suite; default setup uses `autobuild` for compiled languages other than C/C++, C#, Java, and Rust.
  - CodeQL query suites: default setup supports `default` and `security-extended`; `security-and-quality` is available through advanced setup.
  - Advanced CodeQL/SARIF workflows require `security-events: write` for upload.
  - Dependency review can fail pull requests based on vulnerability severity and scopes.
  - Dependabot supports `github-actions`, `gomod`, and `npm` ecosystems; pnpm is handled through the `npm` ecosystem.
  - Secret scanning runs automatically for public repositories; non-provider patterns can expand detection beyond provider secrets.
  - Code scanning merge protection can block merges that introduce code scanning alerts when rules are configured.
- OpenSSF Scorecard documentation: Scorecard can output SARIF for GitHub code scanning and has workflow restrictions when publishing results.

Affected contracts and surfaces:

- GitHub workflows under `.github/workflows/`.
- Optional scanner configs under `.github/`.
- Optional dependency update config `.github/dependabot.yml`.
- `Makefile` local validation targets if hygiene scanners are added to local/CI gates.
- SOW lifecycle and durable project memory.
- No product runtime, public website routes, feed pipeline, installer behavior, or API schemas are directly affected.

Existing patterns to reuse:

- Existing CI workflow uses explicit least-privilege `permissions: contents: read`.
- Existing `Makefile` pins Go tool versions and runs Go tools through `go run`.
- Existing CI validates root module and nested tool module separately.
- Existing project SOW system records decisions and validation before commit.

Risk and blast radius:

- Security: better scanner coverage improves supply-chain and code-risk visibility; overbroad scanners can create alert fatigue.
- Operations: branch/ruleset enforcement can block direct pushes and automation if required checks are misnamed or too broad.
- Maintenance: Dependabot version updates can add PR volume unless grouped.
- CI time: additional workflows add GitHub Actions minutes and may slow pull-request feedback.
- False positives: `security-and-quality`, Scorecard, ShellCheck, and generic secret patterns are useful but can require baselines or policy exceptions.

Sensitive data handling plan:

- Do not write raw secrets, tokens, secret-scanning payloads, private endpoints, customer identifiers, personal data, or proprietary incidents into SOWs, docs, specs, skills, code comments, or workflow comments.
- Use only redacted scanner output and metadata such as rule id, file path, line number, severity, and alert state.
- Secret-scanning API output must not include secret values; local gitleaks scans use `--redact=100`.

Implementation plan:

1. Record the user's decisions in this SOW.
2. Implement the selected CodeQL setup:
   - if default setup is kept, update it through GitHub API/UI and document the repo state;
   - if advanced setup is chosen, add checked-in workflow/config and disable dynamic default setup to avoid duplicate analyses.
3. Add selected scanner workflows/configs:
   - dependency review for pull requests if chosen;
   - Dependabot version-update config if chosen;
   - actionlint/ShellCheck hygiene gates if chosen;
   - optional gitleaks and/or OpenSSF Scorecard SARIF if chosen.
4. Fix or baseline current ShellCheck findings before making ShellCheck blocking.
5. Add branch protection/ruleset enforcement only if chosen.
6. Validate locally, push, and verify GitHub runs/alerts through API.
7. Disable dynamic CodeQL default setup after the checked-in advanced CodeQL
   workflow has successfully analyzed `main`.
8. Add `main` ruleset enforcement after scanner workflow check names are stable.

Validation plan:

- `git diff --check`
- `go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/*.yml`
- `shellcheck $(git ls-files '*.sh')` after ShellCheck fixes/baseline if ShellCheck is chosen.
- `go run github.com/zricethezav/gitleaks/v8@latest detect --no-banner --redact=100 --source . --exit-code 2` if gitleaks is chosen.
- CodeQL manual build command: `make ui-static && go build ./... && (cd tools/dronebl2ipsets && go build ./...)`.
- GitHub API check for workflows/default setup after push.
- GitHub API check for open CodeQL, Dependabot, and secret-scanning alerts after workflow completion.
- If branch/ruleset enforcement is configured, verify repository rulesets or branch protection via GitHub API.

Artifact impact plan:

- AGENTS.md: no update expected; existing SOW and GitHub process rules are sufficient.
- Runtime project skills: possible update only if scanner policy becomes a durable project convention.
- Specs: no product/application spec update expected because GitHub scanning does not change runtime product behavior.
- End-user/operator docs: no update expected unless a public security policy or contributor docs are added.
- End-user/operator skills: no update expected.
- SOW lifecycle: current SOW remains in `.agents/sow/current/`; move to done only with implementation, validation, and commit if completed.

Open-source reference evidence:

- No mirrored source repositories were checked. The task is repository GitHub security configuration; official GitHub docs and primary OpenSSF Scorecard documentation are the authoritative references.

Open decisions:

- Resolved by user approval on 2026-06-02. See `## Implications And Decisions`.

## Implications And Decisions

1. CodeQL ownership and query breadth
   - Option A: Keep GitHub default setup and change query suite from `default` to `security-extended`.
     - Pros: lowest maintenance; still official GitHub-managed; fewer false positives than quality queries; current dynamic setup already analyzes Actions, Go, JS/TS, and Python successfully.
     - Cons: configuration stays partly outside git; cannot use `security-and-quality`; less explicit build/path control.
     - Risks: future maintainers may not see scanner config in repo; default setup may include/exclude languages automatically as GitHub evolves.
   - Option B: Switch to checked-in advanced CodeQL workflow using `security-and-quality`.
     - Pros: repo-controlled and reviewable; broader quality/reliability queries; can explicitly define languages, schedules, paths, and build commands.
     - Cons: more maintenance; likely more findings/noise; dynamic default setup must be disabled to avoid duplicate CodeQL workflows.
     - Risks: broken manual build config can reduce scan coverage; duplicate analyses confuse alerts if default setup remains enabled.
   - Option C: Keep current default setup unchanged.
     - Pros: no churn; current alerts are clean.
     - Cons: remains at the narrowest query suite; does not address the user's request to configure scanners properly.
     - Risks: missed lower-confidence but relevant issues.
   - Recommendation: Option B. In an AI-generated repository without human review, broader CodeQL coverage is worth the extra noise. Disable dynamic default setup after the advanced workflow is validated to avoid duplicate analyses.
   - User decision: Option B.

2. Pull-request dependency enforcement
   - Option A: Add `dependency-review` workflow for pull requests, failing on `moderate` and above for runtime/unknown scopes.
     - Pros: blocks newly introduced vulnerable dependencies before merge; complements existing post-merge Dependabot alerts and `govulncheck`.
     - Cons: only meaningful on PRs; may block transitive dev tooling changes until reviewed.
     - Risks: without branch/ruleset enforcement it reports but does not stop direct pushes.
   - Option B: Add dependency review as report-only.
     - Pros: less disruptive.
     - Cons: vulnerable dependency changes can still merge.
     - Risks: easy to ignore.
   - Option C: Do not add dependency review.
     - Pros: no new workflow.
     - Cons: misses GitHub's PR-level dependency diff signal.
     - Risks: vulnerable packages may be introduced and caught only after landing.
   - Recommendation: Option A.
   - User decision: Option A.

3. Dependabot version updates
   - Option A: Add `.github/dependabot.yml` for GitHub Actions, root Go module, nested tool Go module, and UI npm/pnpm, grouped weekly.
     - Pros: makes dependency-update policy explicit and reviewable; covers action versions and both Go modules.
     - Cons: creates regular PR volume.
     - Risks: UI ecosystem updates may be noisy; grouping reduces but does not remove review work.
   - Option B: Configure only GitHub Actions and Go modules; leave UI manual.
     - Pros: lower PR volume.
     - Cons: UI dependencies remain manually maintained.
     - Risks: stale frontend tooling and browser test dependencies.
   - Option C: Do not add version-update config.
     - Pros: no Dependabot PR noise.
     - Cons: repo remains dependent on ad hoc updates.
     - Risks: slow patch adoption.
   - Recommendation: Option A with weekly grouping.
   - User decision: Option A.

4. Hygiene scanner gates
   - Option A: Add `make actionlint` and `make shellcheck`, fix/baseline current ShellCheck findings, and run them in CI.
     - Pros: catches broken workflows and shell footguns in repo-native gates; actionlint is already clean.
     - Cons: requires cleaning current ShellCheck findings first.
     - Risks: ShellCheck warnings on SOW tooling may block unrelated code unless scoped or baselined carefully.
   - Option B: Add actionlint only now; keep ShellCheck manual until warnings are cleaned in a focused SOW.
     - Pros: no immediate ShellCheck cleanup.
     - Cons: shell scripts remain ungated.
     - Risks: install/admin scripts are important enough that this is weaker than desired.
   - Option C: Do not add hygiene gates.
     - Pros: no CI changes.
     - Cons: known ShellCheck findings remain untracked.
     - Risks: shell/script regressions reach main.
   - Recommendation: Option A.
   - User decision: Option A.

5. Secret scanning beyond GitHub provider patterns
   - Option A: Keep GitHub secret scanning/push protection as-is and add a scheduled/PR redacted gitleaks workflow for generic patterns.
     - Pros: local scan is currently clean; catches generic secrets GitHub provider scanning may not alert on.
     - Cons: third-party scanner can produce false positives; needs redaction and possibly an allowlist.
     - Risks: noisy findings in generated/test fixtures if future fixtures include fake tokens.
   - Option B: Enable GitHub non-provider/generic secret patterns in repository settings, without gitleaks.
     - Pros: keeps detection in GitHub-native secret scanning.
     - Cons: availability/behavior depends on GitHub plan/settings; may increase alert volume outside repo config.
     - Risks: alert policy not versioned in git.
   - Option C: Keep current GitHub provider secret scanning and push protection only.
     - Pros: already enabled and clean; no extra noise.
     - Cons: less generic secret coverage.
     - Risks: fake-looking but real generic credentials could slip through.
   - Option D: Enable GitHub non-provider patterns and add redacted gitleaks.
     - Pros: strongest coverage; combines GitHub-native alerts with a repo-controlled generic secret scanner.
     - Cons: highest false-positive risk.
     - Risks: future generated fixtures may need explicit allowlisting.
   - Recommendation: Option D. The current redacted gitleaks scan is clean, so this is a reasonable time to add the stronger guardrail.
   - User decision: Option D.

6. OpenSSF Scorecard
   - Option A: Add scheduled Scorecard SARIF upload to code scanning.
     - Pros: exposes repository supply-chain hygiene gaps such as branch protection, pinned actions, security policy, and dependency update posture.
     - Cons: may create findings that are repository-policy issues, not code bugs.
     - Risks: noisy until branch protection/action pinning/security policy decisions are resolved.
   - Option B: Run Scorecard only manually or scheduled without SARIF upload.
     - Pros: useful audit signal without Security tab noise.
     - Cons: weaker visibility.
     - Risks: easy to ignore.
   - Option C: Do not add Scorecard now.
     - Pros: avoids policy noise until core scanners are stable.
     - Cons: misses useful supply-chain health signal.
     - Risks: branch protection/action pinning gaps stay invisible in scanners.
   - Recommendation: Option A. In a no-human-review repository, supply-chain hygiene findings belong in the same Security/code-scanning surface, even if they initially expose policy gaps.
   - User decision: Option A.

7. Merge/direct-push enforcement
   - Option A: Add a repository ruleset for `main` requiring pull requests and required checks.
     - Pros: scanner failures block merges before main.
     - Cons: changes current direct-push workflow.
     - Risks: maintainers and automation can be blocked if required checks are too broad or misnamed.
   - Option B: Add required status checks but allow direct admin bypass.
     - Pros: encourages scanner gating while preserving emergency/main-maintainer escape hatch.
     - Cons: bypass can undermine enforcement.
     - Risks: direct pushes still land before scanners run.
   - Option C: Do not configure branch/ruleset enforcement in this SOW.
     - Pros: avoids disrupting current workflow.
     - Cons: scanners run after direct pushes and cannot prevent main from receiving bad changes.
     - Risks: "configured scanners" may be mostly advisory.
   - Recommendation: Option A, staged after scanner workflows have one successful run so required-check names are known. Do not require human review, but do require scanner/status checks before `main` updates.
   - User decision: Option A, staged after successful scanner runs.

8. GitHub Actions supply-chain hardening
   - Option A: Keep version-tagged actions and unrestricted allowed actions.
     - Pros: least maintenance.
     - Cons: weakest supply-chain posture.
     - Risks: a compromised action tag or newly introduced third-party action can affect automated workflows.
   - Option B: Pin third-party actions to commit SHAs, keep GitHub first-party actions on major tags, and avoid third-party actions where a first-party action or `go run` tool is practical.
     - Pros: strong practical improvement with moderate maintenance.
     - Cons: still trusts first-party major tags.
     - Risks: pinned third-party actions need update discipline.
   - Option C: Require SHA pinning for all actions and restrict allowed actions at repository settings.
     - Pros: strongest policy.
     - Cons: highest maintenance; first-party action updates become more manual.
     - Risks: repository automation can break if required pins are stale or missing.
   - Recommendation: Option B now, then use Scorecard findings to decide whether Option C is worth the extra maintenance.
   - User decision: Option B.

## Plan

1. Wait for user decisions and record selections in this SOW.
2. Implement selected scanner configs and any required ShellCheck cleanup.
3. Validate locally.
4. Push if requested.
5. Verify GitHub-side workflows, alerts, and settings through API.

## Execution Log

### 2026-06-02

- Investigated current GitHub security/scanner state.
- Confirmed no open CodeQL, Dependabot, or secret-scanning alerts.
- Confirmed historical CodeQL findings are fixed, not dismissed.
- Ran local actionlint, ShellCheck, and redacted gitleaks checks.
- Created decision-gated SOW before implementation.
- Recorded user approval for the strict scanner policy:
  advanced CodeQL `security-and-quality`, dependency review, grouped
  Dependabot updates, actionlint/ShellCheck, GitHub non-provider secret
  patterns plus gitleaks, Scorecard SARIF, staged `main` ruleset enforcement,
  and practical GitHub Actions supply-chain hardening.
- Added checked-in advanced CodeQL workflow, dependency review workflow,
  hygiene workflow, Scorecard SARIF workflow, and Dependabot update config.
- Added Makefile hygiene targets for actionlint, ShellCheck, and gitleaks.
- Fixed all current ShellCheck warnings/info findings on tracked shell scripts
  before making ShellCheck a blocking hygiene gate.
- Added `SECURITY.md` so vulnerability reporting is explicit and Scorecard has
  a security-policy surface to inspect.
- Created `.agents/skills/project-hygiene/SKILL.md` and registered it in
  `AGENTS.md`; the skill requires full hygiene posture checks and resolution
  of both blocking and non-blocking valid findings.
- Hardened non-push `actions/checkout` steps with `persist-credentials: false`.
- Attempted to enable GitHub non-provider secret patterns through the
  repository API; follow-up settings read still shows the feature disabled, so
  generic secret detection is covered by the checked-in redacted gitleaks gate.
- Investigated GitHub Security and quality AI findings through the browser UI.
  Two `pkg/web/server.go` findings are valid Go correctness fixes
  (`http.ErrServerClosed` should be compared with `errors.Is`). The
  `pkg/web/admin_manifest.go` findings require producer-path verification
  before changing manifest semantics.
- Investigated Codacy through the authenticated browser UI. Codacy is currently
  noisy enough that the first Codacy task must be configuration and triage,
  not mechanical issue-by-issue fixing.
- Cloned `codacy/codacy-skills` and updated the project hygiene skill so
  future scanner work includes Codacy Cloud, Codacy local analysis, and GitHub
  AI findings without relying on user-provided links.
- Fixed the valid GitHub AI findings in `pkg/web/server.go` by using
  `errors.Is` for `http.ErrServerClosed` comparisons.
- Verified the `pkg/web/admin_manifest.go` GitHub AI findings against producer
  paths and specs. The geo filename is intentional
  (`web/{feed}_{geo_provider}.json`), and configured provider fan-out artifacts
  are required repair signals for settled non-database public feed manifests.
  The misleading manifest comment was corrected and a focused manifest test now
  locks the required geo/ASN provider artifact contract.
- Investigated the two Dependabot PRs opened after adding `.github/dependabot.yml`.
  PR #1 is a Go module update PR; its only failed checks were CodeQL runs from
  before dynamic default setup was disabled. PR #2 is an npm/pnpm UI update PR;
  CI and Go CodeQL failed because `pnpm --dir ui install --frozen-lockfile`
  rejected recently published packages with
  `ERR_PNPM_MINIMUM_RELEASE_AGE_VIOLATION`.
- Added a one-day Dependabot cooldown to the npm `/ui` update block so future
  npm version-update PRs align with pnpm v11's default one-day
  `minimumReleaseAge` supply-chain guardrail.
- Updated `.agents/skills/project-hygiene/SKILL.md` so future hygiene checks
  verify Dependabot cooldown alignment with package-manager release-age policy
  and do not weaken pnpm release-age protection merely to merge a dependency PR.
- Queried Codacy issue overview and tool settings through the authenticated
  browser UI without writing raw issue payloads to disk because issue payloads
  include author metadata. Redacted aggregate evidence shows 8845 total Codacy
  issues: ESLint8 5741, markdownlint 1281, Lizard 970, Opengrep/Semgrep 657,
  Agentlinter 51, Stylelint 50, Trivy 40, Prospector 35, Pylint 9, Bandit 9,
  and PMD 1.
- Codacy tool settings show `ESLint9` is enabled with configuration-file use,
  but the repository had no root ESLint v9 flat config file. Added
  `eslint.config.mjs` at the repository root to delegate to the existing
  `ui/eslint.config.js` config and make Codacy's enabled ESLint9 config-file
  path valid.
- Codacy tool settings also show `ESLint`/ESLint8 is enabled by the Default
  coding standard. Because official Codacy documentation says tools enabled by
  a coding standard cannot be disabled at repository level, the first safe
  cleanup is a repo-controlled `.codacy.yml` engine-specific path exclusion for
  `eslint-8` on UI paths already covered by ESLint9, not an organization-wide
  default coding-standard change.
- Re-checked GitHub Security and quality AI findings after the root ESLint
  config bridge. The previous 4 findings are no longer shown, but GitHub now
  reports 1 finding in `eslint.config.mjs`. Local ESLint validation confirms
  the bridge resolves the existing UI config, and the root bridge was rewritten
  to import `uiConfig` and export it as default so the root config shape is
  explicit.
- Re-checked GitHub Security and quality AI findings after the named-binding
  rewrite. GitHub still reports 1 finding in `eslint.config.mjs`, now warning
  that the root bridge should make the flat-config array shape explicit. The
  root bridge now exports `Array.isArray(uiConfig) ? [...uiConfig] :
  [uiConfig]`, which preserves array configs and wraps a single config object.
- Added `.codacy.yml` with `eslint-8` exclusions for `ui/**` and
  `eslint.config.mjs`. The expected effect on the next Codacy analysis is to
  remove the large wrong-stack UI ESLint8 issue class while leaving ESLint9,
  CodeQL, Opengrep/Semgrep, Trivy, gitleaks, and the rest of the scanner stack
  active for follow-up triage.
- Codacy analyzed commit `de892c69618b58367e588c6763901e3bdbfee30f` after
  the `.codacy.yml` change. The repository grade improved from C to A and
  current issues dropped from 8846 to 3153. Remaining issue classes are now
  mostly complexity, Markdown style, Go permission/path Semgrep findings,
  Trivy dependency findings, and smaller JS/TS/Python/CSS/Shell findings.
- Sanitized Codacy issue samples after the `.codacy.yml` change show Trivy
  high/medium/minor findings on `go.mod` and
  `tools/dronebl2ipsets/go.mod`, caused by the `go 1.26.0` directive being
  treated as `golang/stdlib@v1.26.0`. Official Go downloads list Go 1.26.3,
  and local `go version` reports `go1.26.3-X:nodwarf5`, so both module
  directives and checked-in `actions/setup-go` versions were updated to
  `1.26.3`.

## Validation

Acceptance criteria evidence:

- CodeQL advanced setup is checked in at `.github/workflows/codeql.yml`, with
  `security-and-quality` queries for Actions, Go, JavaScript/TypeScript, and
  Python.
- Dependency review is checked in at `.github/workflows/dependency-review.yml`
  and fails on `moderate` or worse vulnerable dependency additions.
- Dependabot version updates are checked in at `.github/dependabot.yml` for
  GitHub Actions, root Go module, nested DroneBL Go module, and UI npm/pnpm.
  The UI npm/pnpm block now has `cooldown.default-days: 1` to match pnpm v11's
  default one-day release-age guardrail.
- Hygiene gates are checked in through `make actionlint`, `make shellcheck`,
  `make gitleaks`, and `.github/workflows/hygiene.yml`.
- Scorecard SARIF upload is checked in at `.github/workflows/scorecard.yml`
  with the third-party Scorecard action pinned to commit
  `4eaacf0543bb3f2c246792bd56e8cdeffafb205a`.
- Project hygiene skill is valid and registered in `AGENTS.md`.
- GitHub alert API currently reports zero open CodeQL alerts, zero open
  Dependabot alerts, and zero open secret-scanning alerts before push.

Tests or equivalent validation:

- Pre-implementation evidence:
  - `go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/ci.yml .github/workflows/wiki-sync.yml`: passed.
  - `go run github.com/zricethezav/gitleaks/v8@latest detect --no-banner --redact=100 --source . --exit-code 2`: passed; no leaks found.
  - `shellcheck $(git ls-files '*.sh')`: failed with current warnings/info findings that must be fixed or baselined before ShellCheck becomes blocking.
- Implementation validation:
  - `python /home/costa/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`: passed.
  - `make actionlint`: passed.
  - `make shellcheck`: passed.
  - `make gitleaks`: passed; no leaks found.
  - `make hygiene`: passed.
  - `make ui-static && go build ./... && (cd tools/dronebl2ipsets && go build ./...)`: passed.
  - `git diff --check`: passed.
- Codacy / GitHub AI finding update validation:
  - `python /home/costa/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`: passed after the Codacy/GitHub AI finding skill update.
  - `go test ./pkg/web -run 'TestBuildFeedManifestRequiresConfiguredProviderFanOutArtifacts|TestRunServes|TestAdminReadRoutesAllowHEAD'`: passed.
  - `go test ./pkg/web`: passed.
  - `make hygiene`: passed.
  - `git diff --check`: passed.
- Root ESLint bridge / Codacy config validation:
  - `ruby -e 'require "yaml"; YAML.load_file(".codacy.yml"); puts "codacy yaml ok"'`:
    passed.
  - `python /home/costa/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`:
    passed after the Codacy coding-standard lesson was added to the hygiene
    skill.
  - `pnpm --dir ui exec eslint --config ../eslint.config.mjs src/App.tsx`:
    passed after the named-binding bridge and after the explicit-array bridge.
  - `pnpm --dir ui lint`: passed.
  - `make hygiene`: passed.
  - `git diff --check -- .codacy.yml .agents/skills/project-hygiene/SKILL.md .agents/sow/current/SOW-0099-20260602-github-code-scanning-hygiene.md eslint.config.mjs`:
    passed.
  - `timeout 120 npx -y @codacy/analysis-cli validate-configuration --directory "$PWD"`:
    failed because the installed `@codacy/analysis-cli` exposes `analyze`,
    `init`, `update-config`, `discover`, `info`, `login`, and `logout`, but not
    `validate-configuration`.
- Go stdlib Trivy finding validation:
  - `go version && go env GOVERSION GOTOOLCHAIN`: local Go reports
    `go1.26.3-X:nodwarf5` with `GOTOOLCHAIN=auto`.
  - `make actionlint`: passed after pinning workflow `actions/setup-go` inputs
    to `1.26.3`.
  - `go test ./...`: passed.
  - `cd tools/dronebl2ipsets && go test ./...`: passed.
  - `python /home/costa/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-testing`:
    passed after updating the Go manifest version in the project-testing skill.
  - `make hygiene`: passed.

Real-use evidence:

- Pre-push GitHub API evidence:
  - open CodeQL alerts: `0`;
  - open Dependabot alerts: `0`;
  - open secret-scanning alerts: `0`;
  - secret scanning and push protection remain enabled;
  - non-provider secret patterns remain disabled after API PATCH, consistent
    with GitHub Advanced Security availability constraints.
- Pending post-push workflow/default-setup/ruleset verification.

Reviewer findings:

- GitHub AI findings:
  - `pkg/web/server.go`: valid; fixed with `errors.Is`.
  - `pkg/web/admin_manifest.go`: filename asymmetry finding rejected as a false
    positive with evidence from producer path `pkg/engine/geoloc.go` and spec
    `.agents/sow/specs/files-layout.md`.
  - `pkg/web/admin_manifest.go`: required-provider finding rejected as stated
    because provider fan-out files are settled-run repair signals; misleading
    comment corrected and test added.

Same-failure scan:

- ShellCheck same-failure scan is represented by `make shellcheck` across all
  tracked shell scripts.
- Workflow same-failure scan is represented by `make actionlint` across all
  checked-in workflows.
- Secret same-failure scan is represented by redacted full-history `make gitleaks`.

Sensitive data gate:

- Scanner evidence recorded only as rule IDs, file paths, lines, states, and redacted command summaries. No raw secrets or secret values recorded.

Artifact maintenance gate:

- AGENTS.md: updated to register `.agents/skills/project-hygiene/`.
- Runtime project skills: added `.agents/skills/project-hygiene/SKILL.md`.
- Specs: no product/application spec update needed; scanner policy does not
  change runtime product behavior, feed semantics, APIs, or file layout.
- End-user/operator docs: added `SECURITY.md` for vulnerability reporting and
  scanner policy expectations.
- End-user/operator skills: pending.
- SOW lifecycle: current SOW remains in `.agents/sow/current/` until
  post-push GitHub workflow, CodeQL default-setup, and ruleset verification are
  complete.

Specs update:

- No update needed; this is repository security/CI posture, not product runtime behavior.

Project skills update:

- Added and later updated `.agents/skills/project-hygiene/SKILL.md`.

End-user/operator docs update:

- Added `SECURITY.md`.

End-user/operator skills update:

- None expected.

Lessons:

- Pending.

Follow-up mapping:

- Branch/ruleset enforcement may become a follow-up if not included in this SOW.

## Outcome

Pending.

## Lessons Extracted

Pending.

## Followup

None yet.

## Regression Log

None yet.

Append regression entries here only after this SOW was completed or closed and later testing or use found broken behavior. Use a dated `## Regression - YYYY-MM-DD` heading at the end of the file. Never prepend regression content above the original SOW narrative.
