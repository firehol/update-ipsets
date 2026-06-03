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
- Upload Codacy SARIF to GitHub Code Scanning so Codacy findings are visible
  in the GitHub security/code-scanning surface, while keeping rule removal
  evidence-gated.

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
- Codacy Cloud CLI was installed and authenticated by the user on 2026-06-02.
  `codacy repository gh firehol update-ipsets --output json` reports the latest
  Codacy analysis on `main` ended at `2026-06-02T16:40:33Z` for commit
  `f4ffd7b00b85652b849764df08b3dbd52c7826a5`, with `issuesCount: 2726`, grade
  gate policy `Codacy Gate Policy`, and the `Default coding standard`.
- Codacy issue overview from `codacy issues gh firehol update-ipsets --branch
  main --overview --output json` reports 2726 issues: 262 Security, 982
  Complexity, 901 CodeStyle, 463 BestPractice, 41 Compatibility, 39 ErrorProne,
  18 Documentation, 15 Comprehensibility, and 5 Performance. Levels are 170
  High, 131 Error, 1118 Warning, and 1307 Info.
- Codacy findings from `codacy findings gh firehol update-ipsets --limit 500
  --output json` report 262 active security findings: 131 Critical and 131 High.
  Category totals are FileAccess 179, InsecureModulesLibraries 37, XSS 18,
  Cryptography 8, CommandInjection 7, UnexpectedBehaviour 6, DoS 2, HTTP 2,
  SSL 2, and Regex 1.
- Codacy security issue grouping from `codacy issues gh firehol update-ipsets
  --branch main --categories Security --limit 1000 --output json` shows the
  largest classes are dynamic file/path reads (96), file permission findings
  (80), package dependency version-range findings in `ui/package.json` (33),
  direct writes to `http.ResponseWriter` (11), and `math/rand` findings (6).
  The file-permission group includes many `os.MkdirAll(..., 0o700)` test
  directory creations; those must be triaged as directory false positives or
  rule/config mismatch rather than weakened to unusable `0600` directory modes.
- Codacy tools from `codacy tools gh firehol update-ipsets --output json` show
  many enabled tools, including Gosec, Staticcheck, Trivy, Opengrep,
  GolangCI-Lint, Ruff, ESLint9, Bandit, ShellCheck, markdownlint, Stylelint,
  Lizard, Agentlinter, Prospector, and legacy/deprecated tools such as TSLint
  and Pylint (deprecated). `ESLint9` is enabled and uses a configuration file;
  `ESLint (deprecated)` is disabled, while another `ESLint` tool remains enabled
  by the default coding standard. Tool overlap and deprecated-tool ownership
  must be handled as configuration hygiene, not by broad issue dismissal.
- Codacy high/error issue details showed `ESLint8_*` findings still attached to
  `scripts/build-wiki.mjs` even though `.codacy.yml` attempted to exclude
  `scripts/**/*.mjs`. The top-level `scripts/build-wiki.mjs` path was not
  matched by that glob, so `.codacy.yml` now also excludes `scripts/*.mjs` for
  the legacy `eslint-8` engine. This does not disable current JavaScript
  coverage; `scripts/build-wiki.mjs` remains covered by the repository's
  modern Node syntax and root ESLint configuration tests.
- GitHub Code Scanning and Codacy are separate reporting surfaces. GitHub Code
  Scanning shows CodeQL plus SARIF uploads from workflows, while Codacy Cloud
  issue counts do not appear there unless a workflow generates and uploads
  Codacy SARIF.
- Official GitHub documentation checked on 2026-06-02 says third-party SARIF
  uploads require a workflow with `security-events: write`, use
  `github/codeql-action/upload-sarif`, and should use a distinct `category`
  when multiple analyses are uploaded for the same commit.
- Official Codacy action documentation checked on 2026-06-02 documents the
  GitHub Code Scanning integration with `format: sarif`, `output`,
  `gh-code-scanning-compat: true`, and
  `max-allowed-issues: 2147483647` so issue existence does not make SARIF
  generation fail.
- The latest Codacy Analysis CLI action release checked through the GitHub API
  is `v4.4.7`, published on 2025-07-17, resolving to commit
  `562ee3e92b8e92df8b67e0a5ff8aa8e261919c08`.

## User Decisions - 2026-06-02 Codacy Cleanup Direction

The user approved these implementation choices:

1. Pin direct UI dependency and dev-dependency version specifiers exactly rather
   than keeping npm range specifiers and baselining Codacy's dependency-version
   findings.
2. Move daemon-generated runtime/publication file modes to private
   owner-readable/owner-writable files and private owner-accessible directories
   where no documented install/operator contract requires group access.
3. Handle Codacy tool mismatch with narrow repo/tool/path configuration rather
   than broad tool disabling or bulk issue dismissal.
4. Tighten XSS/HTML rendering code and tests first, then only baseline paths
   proven sanitized or structurally non-HTML.
5. Audit dynamic file/path findings by production surface, add or verify
   root-bound helpers, and baseline test/CLI false positives only with evidence.

## User Decisions - 2026-06-02 Codacy GitHub Visibility

The user approved these implementation choices:

1. Upload all Codacy SARIF results to GitHub Code Scanning now, even if noisy,
   so GitHub shows the Codacy scanner surface more frequently.
2. Fix Codacy issues or remove/disable irrelevant rules only after evidence.
   Do not bulk-disable rules merely to reduce the GitHub Code Scanning count.

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
  treated as `golang/stdlib@v1.26.0`. Official Go downloads listed Go 1.26.3
  at that time, and local `go version` reported `go1.26.3-X:nodwarf5`, so both
  module directives and checked-in `actions/setup-go` versions were updated to
  `1.26.3`.
- On 2026-06-03, GitHub CI govulncheck failed on commit
  `71d5e393d6216e4220812cae64f3e6ffbd9473db` because Go `1.26.3` is affected
  by `GO-2026-5039` in `net/textproto` and `GO-2026-5037` in `crypto/x509`.
  Official Go release history lists Go `1.26.4` released on 2026-06-02 with
  security fixes for `crypto/x509`, `mime`, and `net/textproto`, so the module
  directives and checked-in `actions/setup-go` versions were updated to
  `1.26.4`.
- Codacy analyzed commit `c24357a150cad529223e32d3e364561e865d5134` after the
  Go patch-version update. The repository remains Grade A and current issues
  dropped from 3153 to 3113. Remaining aggregate counts are: Security 655,
  Complexity 973, CodeStyle 900, BestPractice 463, Markdown 1332 issues by
  language, Go 1376 issues by language, and 560 High-level issues.
- Re-checked GitHub Security and quality AI findings after
  `c24357a150cad529223e32d3e364561e865d5134`. The previous server and admin
  manifest findings are no longer shown, but GitHub still reports 1 finding in
  `eslint.config.mjs`. The finding asks for stronger assurance that the root
  ESLint bridge resolves and applies the UI config across representative file
  types. The root bridge now validates that the imported config is an object or
  array of objects, and `ui/scripts/eslint-root-config.test.mjs` verifies root
  config shape, TS/TSX/JS/MJS resolution, and UI TypeScript rule application.
  CI now runs this through `make eslint-root-config`.
- Re-checked GitHub Security and quality AI findings after
  `25bfa710c48bdca0859e828513cc953bc8473fe5`. GitHub reports 0 AI findings.
- Codacy analyzed commit `25bfa710c48bdca0859e828513cc953bc8473fe5`. The
  repository remains Grade A and current issues dropped from 3113 to 3095.
- Triage of remaining Codacy Go security/path issue classes found:
  - `Semgrep_go_file-permissions_rule-fileperm`: 243 total, 236 in
    `*_test.go`, 7 in production files.
  - `Semgrep_go.lang.correctness.permissions.file_permission.incorrect-default-permission`:
    109 total, 77 in `*_test.go`, 32 in production files.
  - `Semgrep_go_file-permissions_rule-mkdir`: 102 total, 76 in `*_test.go`,
    26 in production files.
  - `Semgrep_go_filesystem_rule-fileread`: 93 total, 55 in `*_test.go`, 38 in
    production files.
- Updated test fixtures from broad `0644`/`0755` style modes to restrictive
  `0600`/`0700` modes across tracked `*_test.go` files. Production permission
  findings are intentionally left for a separate contract review because public
  artifacts, install outputs, and shared runtime directories may require
  readable/searchable modes for operators or the service user.
- Fixed the managed install ownership contract for the trusted install surface.
  `install.sh` now installs the binary as `root:iplists` mode `0750`, sets
  install root, `bin/`, and `etc/` to `root:iplists` mode `0750`, sets
  config/template directories to `0750`, config/template files to `0640`, and
  keeps mutable runtime directories owned by `iplists:iplists` with `0750`
  directory modes. This implements the requirement that the binary and configs
  are root-owned and accessible by `iplists` without making them readable by
  every local user.
- Codacy analyzed commit `0c99f4afc3ea74fb54c3820b4a955ad951168b20`. The
  repository remains Grade A with 2802 current issues. Sanitized current issue
  evidence for `Semgrep_go_file-permissions_rule-fileperm` shows 8 remaining
  findings: 7 production writer/publisher paths using `0644` and 1 test
  cleanup path using `0700` on a file. The next permission pass will make
  daemon-owned writer outputs private by default (`0600` for files, `0700` for
  directories). Public HTTP access is not a POSIX world-read contract because
  the embedded daemon serves published artifacts as the service user, and the
  managed install already keeps mutable runtime roots non-world-readable.
  Install-time group access remains reserved for trusted root-owned binary and
  configuration paths that the `iplists` service user must read or execute.
- Codacy analyzed commit `4516edad38f6cdd414c85f15b0c23178dfaa13f2` after
  the daemon runtime writer mode cleanup. The repository remains Grade A and
  current issues dropped from 2802 to 2763. Security issues dropped from 344 to
  303, High-level issues dropped from 249 to 208, and the earlier
  `Semgrep_go_file-permissions_rule-fileperm` and
  `Semgrep_go_file-permissions_rule-mkdir` overview patterns are no longer
  visible.
- Triage of the next visible Codacy JavaScript/script issue cluster found
  `scripts/build-wiki.mjs` carrying legacy `eslint-8` compatibility findings
  such as ES modules, `const`, and `async`/`await` being forbidden. The project
  should not downgrade modern Node maintenance scripts to obsolete JavaScript
  just to satisfy that legacy engine. The repo-controlled fix is a narrow
  `eslint-8` path exclusion for modern `.mjs` maintenance scripts, explicit
  root ESLint9/core-rule coverage for those scripts, and code fixes for true
  risks found during the same triage.
- The true `scripts/build-wiki.mjs` risks fixed in that pass are:
  destination cleanup now rejects paths outside the repository workspace and
  rejects any source/destination overlap before `rm` is called; the flagged
  trailing-slash regex was replaced with bounded character scanning.
- Triage of the visible Python subprocess/security cluster found two classes:
  a validator subprocess in `tools/build-firehol-static-enrichment.py` that can
  be replaced by loading the local validator module directly, and required
  `git`/`gh` subprocess calls in `agents/enrichment-refresh.py`. The former was
  removed. The latter now resolves executable paths with `shutil.which`, passes
  arguments as lists with no shell, sets explicit `check` behavior, closes
  stdin, and carries narrow `# nosec` rationale for the fixed command surface.
  Broad YAML parse exceptions in the touched generator were narrowed to
  `OSError` and `yaml.YAMLError`.
- Triage of the visible shell `IFS` cluster found avoidable CSV splitting and
  joining in `agents/run-enrichment-pool.sh`. The script now uses explicit
  quoted helper functions for comma splitting and joining instead of mutating
  `IFS` for those cases.
- Triage of the visible Codacy critical/high production issue classes found
  SHA-1 used for web cache ETags and kernel temporary ipset names, unbounded
  ASN gzip/tar.gz provider extraction, and one processor copy fallback using
  default output file permissions. The weak hashes are not password or
  signature primitives, but SHA-256 is a cheap compatible replacement. ASN
  provider extraction now enforces an expanded-payload ceiling, writes private
  temp/provider files, and removes incomplete temp files on failure. The
  processor copy fallback now writes private generated files.
- Re-checked GitHub Security and quality AI findings after
  `7d29654e8d73dbeb679ef28ef1b260d3fe207df4`. GitHub reports 3 current
  findings in `pkg/web/cache.go`: an unlocked per-file cache limit read and a
  concurrent miss/insert race that could duplicate LRU entries and overcount
  cache bytes. The cache now reads the per-file limit under the cache mutex and
  re-checks for a fresh entry under the mutex immediately before inserting a
  loaded file.
- Re-checked GitHub Security and quality AI findings after
  `cffe063383f0b51a7fb65a46bc71c2d23a4afe9a`. GitHub reports 2 current
  findings in `pkg/web/cache_test.go`: the byte-limit test should prove which
  entry remained cached after eviction, and the rooted symlink escape test
  should prove the failed helper did not write a response. The tests now assert
  the exact cache/LRU/byte state after byte-limit eviction and assert that the
  symlink escape path leaves the recorder status/body untouched.
- After the user installed and authenticated the Codacy Cloud CLI, re-queried
  Codacy on 2026-06-02. The latest completed Codacy analysis on `main` reports
  2726 open issues and 262 active security findings. The biggest actionable
  security groups are file/path reads, file permissions, dependency version
  ranges in `ui/package.json`, direct response writes, and test-only
  `math/rand` findings.
- Implemented the approved first cleanup batch for the current Codacy evidence:
  pinned direct UI dependency and dev-dependency specifiers exactly, refreshed
  only the pnpm lockfile importer specifiers, added a narrow top-level
  `scripts/*.mjs` legacy `eslint-8` exclusion, changed daemon-generated
  runtime/publication modes to private `0700` directories and `0600` files,
  updated the managed installer to repair mutable runtime trees to `0700` and
  `0600`, changed the generated systemd unit to `UMask=0077`, kept the trusted
  binary/config install contract as `root:iplists` with group access, rendered
  unsafe feed commit-history values as text instead of links, and replaced the
  `pkg/iprange` unsafe endian probe with `encoding/binary.NativeEndian`.
- Updated `.agents/sow/specs/files-layout.md`,
  `docs/installation/filesystem-layout.md`, `docs/installation/installation.md`,
  `docs/installation/systemd-setup.md`, and
  `.agents/skills/project-operations/SKILL.md` so future operators and agents
  see the same split contract: root-owned installed binary/config are group
  accessible to `iplists`, while daemon-created runtime/publication files are
  private to the service user.
- Recorded the user's Codacy GitHub visibility decision: upload all Codacy
  SARIF to GitHub Code Scanning now, and remove/disable rules only after
  evidence.
- Added `.github/workflows/codacy-sarif.yml` to run the official Codacy
  Analysis CLI action pinned to release commit
  `562ee3e92b8e92df8b67e0a5ff8aa8e261919c08`, generate `codacy-results.sarif`,
  and upload it to GitHub Code Scanning under the `codacy` SARIF category.
- Updated `.agents/skills/project-hygiene/SKILL.md` so future hygiene checks
  include Codacy SARIF visibility in GitHub Code Scanning and preserve the
  evidence-gated rule-removal policy.
- The first pushed Codacy SARIF run for
  `f53b3198a510299c15e2b00efea5121f94b15a2c` was still inside `Run Codacy
  Analysis CLI` after about 30 minutes and had not started the upload step.
  The run was cancelled to avoid wasting runner time. Cancellation also showed
  that `if: always()` could attempt to upload an incomplete SARIF file, which
  GitHub rejected as invalid JSON. The workflow was tuned to run Codacy with
  `parallel: 4` and upload SARIF only when the Codacy generation step succeeds.
- The replacement official Codacy Analysis CLI action run for
  `478e611ddb216cf5ac2d704d88b61d4c465f3f00` was still inside `Run Codacy
  Analysis CLI` after about 24 minutes and had not started the upload step.
  This shows the official local-analyzer action is not operationally suitable
  for timely full Codacy visibility on this repository.
- Replaced the official local-analyzer action with a Codacy Cloud export
  workflow. The workflow installs the pinned Codacy Cloud CLI, requires a
  `CODACY_API_TOKEN` GitHub Actions secret, queries the current Codacy Cloud
  issue categories for the branch, converts issue JSON to SARIF with
  `.github/scripts/codacy-issues-to-sarif.mjs`, and uploads the SARIF to GitHub
  Code Scanning under category `codacy`.
- Re-checked the Codacy/GitHub visibility state on 2026-06-03. GitHub Code
  Scanning reports 2642 open alerts with `tool.name == "Codacy Cloud"`, and
  Codacy Cloud reports 2642 current issues on `main`.
- Triage of production-facing Codacy security alerts in the next cleanup slice
  found 4 shared URL struct mutation findings in `pkg/engine/output.go` and 1
  legacy `?ipset=` redirect finding in `pkg/web/routes.go`. The same remote
  issue sample also shows dynamic file/path findings in `pkg/engine/output.go`;
  those remain for the path-safety triage track and were not claimed as fixed
  by this URL-focused patch.
- The first pushed URL cleanup commit `29e40f2f888a4ba646a478e8cc1dfa24bf9aa2b7`
  was not enough for Codacy: Codacy Cloud analyzed that commit and still
  reported 2642 issues, including the same legacy open-redirect finding and
  shared URL struct mutation findings. The copy-before-mutate and
  path-escaping changes were semantically safe, but not sufficient for these
  scanner rules.
- Fixed the shared URL mutation findings by constructing fresh `url.URL`
  literals for normalized public URL strings instead of assigning to URL struct
  fields after parse.
- Tightened the legacy `?ipset=` redirect contract: valid feed names still
  redirect to a local `/ipsets/{name}` path, while URL-shaped or
  protocol-relative values are rejected with `404` and no `Location` header.
  The remaining Semgrep open-redirect match is a documented false positive
  after validation, so the redirect sink carries a narrow inline `nosemgrep`
  marker with adjacent rationale.

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
- Codacy SARIF upload is checked in at `.github/workflows/codacy-sarif.yml`.
  It runs on default-branch push, weekly schedule, and manual dispatch; exports
  existing Codacy Cloud issues to SARIF; and uploads with category `codacy`.
  It requires a GitHub Actions secret named `CODACY_API_TOKEN`; the secret is
  present in the repository as of 2026-06-03.
- Project hygiene skill is valid and registered in `AGENTS.md`.
- GitHub alert API currently reports zero open CodeQL alerts, zero open
  Dependabot alerts, and zero open secret-scanning alerts before push.

Tests or equivalent validation:

- Pre-implementation evidence:
  - `go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/ci.yml .github/workflows/wiki-sync.yml`: passed.
  - `go run github.com/zricethezav/gitleaks/v8@latest detect --no-banner --redact=100 --source . --exit-code 2`: passed; no leaks found.
  - `shellcheck $(git ls-files '*.sh')`: failed with current warnings/info findings that must be fixed or baselined before ShellCheck becomes blocking.
- Implementation validation:
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`: passed.
  - `make actionlint`: passed.
  - `make shellcheck`: passed.
  - `make gitleaks`: passed; no leaks found.
  - `make hygiene`: passed.
  - `make ui-static && go build ./... && (cd tools/dronebl2ipsets && go build ./...)`: passed.
  - `git diff --check`: passed.
- Codacy / GitHub AI finding update validation:
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`: passed after the Codacy/GitHub AI finding skill update.
  - `go test ./pkg/web -run 'TestBuildFeedManifestRequiresConfiguredProviderFanOutArtifacts|TestRunServes|TestAdminReadRoutesAllowHEAD'`: passed.
  - `go test ./pkg/web`: passed.
  - `make hygiene`: passed.
  - `git diff --check`: passed.
- Root ESLint bridge / Codacy config validation:
  - `ruby -e 'require "yaml"; YAML.load_file(".codacy.yml"); puts "codacy yaml ok"'`:
    passed.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`:
    passed after the Codacy coding-standard lesson was added to the hygiene
    skill.
  - `pnpm --dir ui exec eslint --config ../eslint.config.mjs src/App.tsx`:
    passed after the named-binding bridge and after the explicit-array bridge.
  - `make eslint-root-config`: passed after adding the dedicated root ESLint
    bridge test.
  - `make eslint-root-config`: passed after adding Node `.mjs` maintenance
    script coverage to the root ESLint config and test.
  - `./ui/node_modules/.bin/eslint --config eslint.config.mjs scripts/build-wiki.mjs ui/scripts/check-bundle-budget.mjs`:
    passed from the repository root.
  - `tmpdir=$(mktemp -d ./.tmp-wiki-test.XXXXXX) && cp -R docs "$tmpdir/docs" && mkdir "$tmpdir/wiki" && node scripts/build-wiki.mjs "$tmpdir/docs" "$tmpdir/wiki"; rc=$?; rm -rf "$tmpdir"; exit $rc`:
    passed and built 88 wiki pages with workspace-local paths.
  - `node scripts/build-wiki.mjs docs docs/wiki-test`: failed as expected
    because the destination overlaps the source tree.
  - `rg -n "replace\(/\\/\+\$|\\/\+\$" scripts/build-wiki.mjs`: no matches
    after replacing the flagged trailing-slash regex.
  - `pnpm --dir ui exec eslint --config ../eslint.config.mjs src/App.tsx`:
    passed after the bridge shape guard was added.
  - `pnpm --dir ui lint`: passed.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-testing`:
    passed after documenting the root ESLint bridge validation command in the
    project-testing skill.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`:
    passed after documenting Codacy Go file-permission triage guidance.
  - `make actionlint`: passed after wiring the bridge test into CI.
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
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-testing`:
    passed after updating the Go manifest version in the project-testing skill.
  - `make hygiene`: passed.
- Go stdlib govulncheck follow-up validation:
  - GitHub CI run `26855627397` failed at `make vulncheck` because
    govulncheck reported reachable standard-library vulnerabilities
    `GO-2026-5039` and `GO-2026-5037` in Go `1.26.3`; both report fixed
    versions at Go `1.26.4`.
  - Official Go release history checked on 2026-06-03 lists Go `1.26.4`
    released on 2026-06-02 with security fixes for `crypto/x509`, `mime`, and
    `net/textproto`.
  - `go env GOVERSION GOTOOLCHAIN`: reported `go1.26.4` and `auto` after
    updating the module directives to Go `1.26.4`.
  - `make actionlint`: passed after updating checked-in `actions/setup-go`
    versions to `1.26.4`.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-testing`:
    passed after updating the Go manifest version in the project-testing skill.
  - `git diff --check -- go.mod tools/dronebl2ipsets/go.mod .github/workflows/ci.yml .github/workflows/codeql.yml .github/workflows/hygiene.yml .agents/skills/project-testing/SKILL.md .agents/sow/current/SOW-0099-20260602-github-code-scanning-hygiene.md`:
    passed.
  - `make vulncheck`: passed for both the root module and
    `tools/dronebl2ipsets`; govulncheck reported no vulnerabilities.
  - `make test`: passed.
  - `make test-tools`: passed.
- GitHub run evidence for `c24357a150cad529223e32d3e364561e865d5134` before
  the root ESLint bridge test commit:
  - checked-in CodeQL workflow: success;
  - dynamic Code Quality/CodeQL surface: success;
  - Hygiene workflow: success;
  - CI coverage job: success;
  - CI build job: still in progress as of 2026-06-02 11:17 EEST.
- GitHub run evidence for `25bfa710c48bdca0859e828513cc953bc8473fe5` before
  the test-fixture permission cleanup commit:
  - checked-in CodeQL workflow: success;
  - dynamic Code Quality/CodeQL surface: success;
  - Hygiene workflow: success;
  - CI coverage job: success;
  - CI build job: still in progress as of 2026-06-02 11:23 EEST.
- Codacy Go test-fixture permission cleanup validation:
  - `rg -n "0o(644|666|755|777)|\b0(644|666|755|777)\b" --glob '*_test.go'`:
    no matches after the fixture rewrite.
  - `go test ./...`: passed.
  - `cd tools/dronebl2ipsets && go test ./...`: passed.
  - `make hygiene`: passed.
- Managed install ownership validation:
  - `make shellcheck`: passed after the installer ownership change.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-operations`: passed.
  - `./install.sh --no-restart`: passed locally.
  - `sudo stat -c '%U:%G %a %n' ...`: confirmed install root, `bin/`, binary,
    `etc/`, and config directories are `root:iplists 750`; active config files
    are `root:iplists 640`; runtime directories are `iplists:iplists 750`.
  - `sudo -u iplists test -x /opt/update-ipsets/bin/update-ipsets`: passed.
  - `sudo -u iplists test -r /opt/update-ipsets/etc/config/runtime.yaml`:
    passed.
  - `sudo -u iplists test -w /opt/update-ipsets/data` and
    `sudo -u iplists test -w /opt/update-ipsets/web`: passed.
  - `sudo -u nobody test -r /opt/update-ipsets/etc/config/runtime.yaml`:
    failed as expected.
  - `sudo -u nobody test -x /opt/update-ipsets/bin/update-ipsets`: failed as
    expected.
- Runtime writer permission validation:
  - `rg -n "0o644|0644|0o755|0755" --glob '*.go' --glob '!*_test.go' pkg internal tools cmd`:
    only the intentionally executable generated timestamp script remains.
  - `rg -n "os\.(OpenFile|WriteFile|Chmod)\([^\n]*(0o644|0644|0o700|0700)" --glob '*.go' pkg internal tools cmd`:
    no matches.
  - `rg -n "MkdirAll\([^\n]*(0o755|0755)" --glob '*.go' --glob '!*_test.go' pkg internal tools cmd`:
    no matches.
  - `go test ./internal/fileutil ./pkg/output ./pkg/markdown ./pkg/engine`:
    passed.
  - `go test ./...`: passed.
  - `cd tools/dronebl2ipsets && go test ./...`: passed.
  - `make hygiene`: passed.
  - `make lint`: passed.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`:
    passed.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-operations`:
    passed.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`:
    passed after documenting modern Node `.mjs` versus legacy `eslint-8`
    triage.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-testing`:
    passed after documenting Node `.mjs` root ESLint coverage.
- Python subprocess/security validation:
  - `python -m py_compile agents/enrichment-refresh.py tools/build-firehol-static-enrichment.py`:
    passed.
  - `python agents/enrichment-refresh.py --help`: passed.
  - `python tools/build-firehol-static-enrichment.py --dry-run`: passed and
    discovered 17 FireHOL-maintained feeds without writing outputs.
  - `rg -n "import subprocess|subprocess\.run|except Exception" agents/enrichment-refresh.py tools/build-firehol-static-enrichment.py`:
    only the intentional `git`/`gh` subprocess calls in
    `agents/enrichment-refresh.py` remain, each with narrow `# nosec`
    rationale; no broad `except Exception` remains in the two touched files.
  - `python -m bandit -q agents/enrichment-refresh.py tools/build-firehol-static-enrichment.py`:
    not run because Bandit is not installed locally.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`:
    passed after Python subprocess guidance update.
- Shell `IFS` scanner validation:
  - `shellcheck agents/run-enrichment-pool.sh`: passed.
  - `agents/run-enrichment-pool.sh --feeds alpha,beta --dry-run --no-finalize`:
    passed and printed a two-feed queue.
  - `rg -n "IFS=,|IFS=','|IFS=;|IFS=.*echo" agents/run-enrichment-pool.sh`:
    no matches.
  - `make shellcheck`: passed.
  - The generated dry-run pool directory was removed after validation.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`:
    passed after shell `IFS` guidance update.
- Weak hash, compressed provider extraction, and private writer validation:
  - `go test ./pkg/web ./pkg/engine ./pkg/kernel ./pkg/processor`: passed.
  - `rg -n "crypto/sha1|sha1\\.Sum|sha1\\.New" --glob '*.go' pkg internal cmd tools`:
    no matches after replacing SHA-1 in web cache ETags and kernel temporary
    ipset names.
  - `rg -n "os\\.Create\\(" --glob '*.go' pkg/engine pkg/web pkg/processor pkg/kernel`:
    only same-package test helper uses remain in the searched runtime areas.
  - `git diff --check -- .agents/skills/project-hygiene/SKILL.md .agents/sow/current/SOW-0099-20260602-github-code-scanning-hygiene.md .agents/sow/specs/memory-management.md pkg/web/cache.go pkg/engine/asn_formats.go pkg/engine/asn_formats_test.go pkg/kernel/ipset_linux.go pkg/processor/run_stream.go pkg/processor/copy_file_test.go`:
    passed.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`:
    passed after adding weak-hash and compressed-provider extraction guidance.
  - `go test ./pkg/processor ./tools/archposture`: passed after moving the new
    processor copy-mode test into a focused file instead of growing an
    already-large test file.
  - `go test ./...`: passed.
  - `make hygiene`: passed.
  - `make lint`: passed.
  - `make test`: passed.
- GitHub AI web cache validation:
  - `git diff --check -- pkg/web/cache.go pkg/web/cache_test.go`: passed.
  - `go test ./pkg/web`: passed.
  - `go test -race ./pkg/web -run TestFileCache`: passed.
  - `go test ./...`: passed.
  - `make hygiene`: passed.
  - `make lint`: passed.
  - `make test`: passed.
  - `./install.sh`: passed; local binary, configuration, and service were
    installed through the managed install path.
  - `systemctl is-active update-ipsets`: returned `active`.
  - `curl -fsS http://127.0.0.1:18888/healthz`: returned `ok`.
  - `curl -fsS http://127.0.0.1:18888/api/v1/status`: returned running engine
    status.
- GitHub AI web cache test validation:
  - `git diff --check -- .agents/sow/current/SOW-0099-20260602-github-code-scanning-hygiene.md pkg/web/cache_test.go`:
    passed.
  - `go test ./pkg/web -run 'TestFileCacheHonorsByteLimit|TestFileCacheRootedServingRejectsSymlinkEscapeAndKeepsServeContent|TestFileCacheInsertRecheckKeepsLRUStateConsistent'`:
    passed.
  - `go test ./pkg/web`: passed.
  - `go test -race ./pkg/web -run TestFileCache`: passed.
  - `go test ./...`: passed.
  - `make hygiene`: passed.
  - `make lint`: passed.
  - `make test`: passed.
- Current Codacy cleanup batch validation:
  - `git diff --check`: passed.
  - `ruby -e 'require "yaml"; YAML.load_file(".codacy.yml"); puts "codacy yaml ok"'`:
    passed.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-operations`:
    passed.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`:
    passed.
  - `pnpm --dir ui install --frozen-lockfile`: passed.
  - `go test ./internal/fileutil ./pkg/iprange ./pkg/output ./pkg/markdown ./pkg/cache`:
    passed.
  - `cd tools/dronebl2ipsets && go test ./...`: passed.
  - `pnpm --dir ui test -- src/components/feed-detail/section-specs.test.tsx src/lib/safe-url.test.ts`:
    passed; Vitest reported 14 test files and 41 tests.
  - `make shellcheck`: passed.
  - `go test ./...`: passed.
  - `pnpm --dir ui lint`: passed.
  - `pnpm --dir ui build`: passed. Vite emitted existing non-fatal font
    resolution and large-chunk warnings.
  - `make hygiene`: passed.
  - `make lint`: passed.
  - `./install.sh`: passed; local managed install completed and restarted
    `update-ipsets`.
  - `systemctl is-active update-ipsets`: returned `active`.
  - `curl -fsS http://127.0.0.1:18888/healthz`: returned `ok`.
  - `curl -fsS http://127.0.0.1:18888/api/v1/status | jq -r '.status // .engine_status // .state // "status-ok"'`:
    returned `status-ok`.
  - `systemctl cat update-ipsets | rg -n 'User=|Group=|UMask=|ExecStart='`:
    confirmed `User=iplists`, `Group=iplists`, and `UMask=0077`.
  - `sudo stat -c '%U:%G %a %n' ...`: confirmed install root, `bin/`,
    binary, `etc/`, and config directories are `root:iplists 750`; active
    config files are `root:iplists 640`; mutable runtime roots are
    `iplists:iplists 700`.
  - `sudo -u iplists test -x /opt/update-ipsets/bin/update-ipsets`,
    `sudo -u iplists test -r /opt/update-ipsets/etc/config/runtime.yaml`,
    `sudo -u iplists test -w /opt/update-ipsets/data`, and
    `sudo -u iplists test -w /opt/update-ipsets/web`: passed.
  - `sudo -u nobody` access checks for the binary, runtime config, and runtime
    data root failed as expected.
  - `rg -n 'UMask=0027|generated artifacts are group-readable|runtime files are readable by the iplists group|0750.*generated|0640.*generated|0o750|0o640' ...`:
    no matches in the touched runtime-mode contract surfaces.
  - `rg -n '"[~^][^"]+"' ui/package.json`: no matches after exact direct
    dependency pinning.
  - `git diff --name-only -- pkg/web/static ui/dist update-ipsets`: no tracked
    generated static or local binary diff from the install validation.
- Codacy SARIF workflow validation:
  - `ruby -e 'require "yaml"; YAML.load_file(".github/workflows/codacy-sarif.yml"); puts "workflow yaml ok"'`:
    passed.
  - `make actionlint`: passed.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`:
    passed.
  - `git diff --check -- .github/workflows/codacy-sarif.yml .agents/skills/project-hygiene/SKILL.md .agents/sow/current/SOW-0099-20260602-github-code-scanning-hygiene.md`:
    passed.
  - Redacted sensitive-string scan over the Codacy SARIF workflow, this SOW,
    and the project hygiene skill: no raw personal-name, home-path, session
    cookie, or token-assignment strings found.
  - First pushed run `26845032482`: cancelled after about 30 minutes with no
    upload started; cancellation exposed that `if: always()` attempted to
    upload an incomplete SARIF file. Workflow tuned afterward with
    `parallel: 4` and success-only SARIF upload.
  - Replacement official-action run `26846678833`: cancelled after about 24
    minutes with no upload started. Workflow replaced afterward with Codacy
    Cloud issue export to SARIF.
  - `gh secret list --repo firehol/update-ipsets`: no repository secrets were
    visible. The Codacy Cloud export workflow therefore requires adding a
    GitHub Actions secret named `CODACY_API_TOKEN` before it can complete in
    GitHub Actions.
  - Local Codacy Cloud export validation using temporary files: passed. The
    converter generated SARIF version `2.1.0` with 2639 results, 87 rules, 240
    `error` results, 1092 `warning` results, and 1307 `note` results.
  - `node --check .github/scripts/codacy-issues-to-sarif.mjs`: passed.
  - `make actionlint`: passed after replacing the official action workflow with
    the Codacy Cloud export workflow.
  - `ruby -e 'require "yaml"; YAML.load_file(".github/workflows/codacy-sarif.yml"); puts "workflow yaml ok"'`:
    passed after replacing the workflow.
  - `python $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/project-hygiene`:
    passed after documenting the Cloud export preference.
  - `git diff --check -- .github/workflows/codacy-sarif.yml .github/scripts/codacy-issues-to-sarif.mjs .agents/skills/project-hygiene/SKILL.md .agents/sow/current/SOW-0099-20260602-github-code-scanning-hygiene.md`:
    passed.
  - Redacted sensitive-string scan over the Codacy SARIF workflow, converter,
    this SOW, and the project hygiene skill: no raw personal-name, home-path,
    session cookie, token assignment, or user email strings found.
  - Post-push run `26848281498` for commit
    `e437a254f132b67383df30e01610d4f19001d51c`: failed fast in 10 seconds at
    `Verify Codacy API token` because the GitHub Actions secret
    `CODACY_API_TOKEN` is not present. This confirms the workflow no longer
    hangs in local analysis, but Codacy SARIF cannot upload to GitHub Code
    Scanning until that secret exists.
  - After the repository secret was added, rerun attempt 2 of `26848324745`
    for commit `56cf3f9c866effbc1b389e20da94976f5f44d5b8` passed in 1 minute
    3 seconds. The `Verify Codacy API token`, `Export Codacy issues`, and
    `Upload Codacy SARIF` steps all passed.
  - `gh secret list --repo firehol/update-ipsets`: reports
    `CODACY_API_TOKEN` present, last updated `2026-06-03T00:05:08Z`.
  - GitHub Code Scanning API after the successful rerun reports `2642` open
    alerts with `tool.name == "Codacy Cloud"`, matching the current Codacy
    Cloud issue total for `main`.
- Codacy URL/security cleanup validation:
  - `gh api -X GET /repos/firehol/update-ipsets/code-scanning/alerts -f state=open -f per_page=100 --paginate --jq '[.[] | select(.tool.name == "Codacy Cloud")] | length' | awk '{s+=$1} END{print s+0}'`:
    reported 2642 open Codacy Cloud alerts.
  - `codacy repository gh firehol update-ipsets --output json`: reported 2642
    current issues.
  - `go test ./pkg/web -run TestLegacyIPSetRedirectStaysOnLocalSite -count=1`:
    passed.
  - `go test ./pkg/engine -run 'Test.*Public.*URL|Test.*Sitemap|Test.*LLMS|Test.*Output' -count=1`:
    passed.
  - `go test ./pkg/web -run 'TestLegacyIPSetRedirectStaysOnLocalSite|TestRouteMethodContracts|TestSurfaceHandlerModesRegisterExpectedSurfaces' -count=1`:
    passed.
  - `go test ./pkg/web ./pkg/engine`: passed.
  - `codacy-analysis analyze . --inspect --files pkg/engine/output.go pkg/web/routes.go pkg/web/routes_test.go --output-format json --output /tmp/codacy-touched-inspect.json`:
    did not run analyzers because the repository has no
    `.codacy/codacy.config.json`; local Codacy Analysis CLI validation is not
    available for this patch. Cloud reanalysis remains the authoritative
    scanner confirmation after push.
  - Codacy Cloud analyzed `29e40f2f888a4ba646a478e8cc1dfa24bf9aa2b7` and still
    reported 2642 issues. A sanitized issue sample still included the legacy
    open-redirect finding in `pkg/web/routes.go` and the shared URL struct
    mutation findings in `pkg/engine/output.go`; follow-up changes were
    required before claiming this cleanup complete.
  - Upstream Semgrep rule evidence checked:
    `semgrep/semgrep-rules @ d04ae90ca63c7719a4a679485b2adce9b34599b5`,
    `go/lang/security/injection/open-redirect.yaml`, and
    `go/lang/security/shared-url-struct-mutation.yaml`.
  - Exact local Semgrep open-redirect rule run against `pkg/web/routes.go`
    passed after the validated local redirect carried the inline `nosemgrep`
    marker.
  - Exact local Semgrep shared URL mutation rule run against
    `pkg/engine/output.go`: passed after replacing URL field assignments with a
    fresh URL literal builder.
  - `go test ./pkg/web -run TestLegacyIPSetRedirectStaysOnLocalSite -count=1`:
    passed after invalid URL-shaped legacy values were changed to `404`.
  - `go test ./pkg/engine -run 'Test.*Public.*URL|Test.*Sitemap|Test.*LLMS|Test.*Output' -count=1`:
    passed after the fresh URL literal helper change.
  - `go test ./pkg/web -run 'TestLegacyIPSetRedirectStaysOnLocalSite|TestRouteMethodContracts|TestSurfaceHandlerModesRegisterExpectedSurfaces' -count=1 && go test ./pkg/web ./pkg/engine`:
    passed after the follow-up scanner changes.

Real-use evidence:

- Pre-push GitHub API evidence:
  - open CodeQL alerts: `0`;
  - open Dependabot alerts: `0`;
  - open secret-scanning alerts: `0`;
  - secret scanning and push protection remain enabled;
  - non-provider secret patterns remain disabled after API PATCH, consistent
    with GitHub Advanced Security availability constraints.
- Post-push GitHub evidence for `4516edad38f6cdd414c85f15b0c23178dfaa13f2`:
  checked-in CodeQL workflow succeeded, dynamic CodeQL/security-quality
  workflow succeeded, Hygiene succeeded, CI coverage succeeded, CI build
  succeeded, and GitHub Security and quality AI findings reported zero
  findings.
- Post-push GitHub API evidence after `4516edad38f6cdd414c85f15b0c23178dfaa13f2`:
  open CodeQL alerts `0`; open Dependabot security alerts `0`.
- Pending post-push workflow/default-setup/ruleset verification for the next
  scanner-cleanup commit.

Reviewer findings:

- GitHub AI findings:
  - `pkg/web/server.go`: valid; fixed with `errors.Is`.
  - `pkg/web/admin_manifest.go`: filename asymmetry finding rejected as a false
    positive with evidence from producer path `pkg/engine/geoloc.go` and spec
    `.agents/sow/specs/files-layout.md`.
  - `pkg/web/admin_manifest.go`: required-provider finding rejected as stated
    because provider fan-out files are settled-run repair signals; misleading
    comment corrected and test added.
  - `pkg/web/cache.go`: valid; fixed by protecting cache-limit reads and
    re-checking the cache under the mutex before loaded-file insertion.
  - `pkg/web/cache_test.go`: valid test-strengthening findings; fixed by
    asserting byte-limit eviction state and untouched response recorder state
    on rooted symlink escape rejection.

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
  `.agents/skills/project-operations/SKILL.md` updated with the split install
  versus daemon-generated artifact permission contract.
- Specs: `.agents/sow/specs/files-layout.md` updated with the daemon-created
  runtime/publication file and directory mode contract.
  `.agents/sow/specs/memory-management.md` updated with the bounded compressed
  provider extraction contract.
- End-user/operator docs: added `SECURITY.md` for vulnerability reporting and
  scanner policy expectations; updated `docs/installation/filesystem-layout.md`,
  `docs/installation/installation.md`, and
  `docs/installation/systemd-setup.md` with daemon-created private runtime file
  behavior and `UMask=0077`.
- End-user/operator skills: no separate operator skill needed for scanner
  posture.
- SOW lifecycle: current SOW remains in `.agents/sow/current/` until
  post-push GitHub workflow, CodeQL default-setup, GitHub AI finding, Codacy,
  and ruleset verification are complete.

Specs update:

- Updated `.agents/sow/specs/files-layout.md` to state that daemon-created
  mutable runtime and publication directories should be owner-private and
  daemon-created non-executable runtime/publication files should be
  owner-private. Public HTTP availability is served by the daemon or configured
  serving process, not by local world-readable generated files.

Project skills update:

- Added and later updated `.agents/skills/project-hygiene/SKILL.md`.
- Updated `.agents/skills/project-testing/SKILL.md` with the root ESLint
  bridge validation command.
- Updated `.agents/skills/project-hygiene/SKILL.md` and
  `.agents/skills/project-testing/SKILL.md` with Go test-fixture permission
  guidance learned from Codacy triage.
- Updated `.agents/skills/project-operations/SKILL.md` with the managed install
  ownership contract.
- Updated `.agents/skills/project-hygiene/SKILL.md` and
  `.agents/skills/project-operations/SKILL.md` with daemon-owned runtime writer
  mode guidance.
- Updated `.agents/skills/project-hygiene/SKILL.md` with weak-hash and
  compressed provider/archive extraction triage guidance.
- Updated `.agents/skills/project-operations/SKILL.md` with the final private
  daemon-generated runtime/publication artifact contract after changing the
  managed install to `UMask=0077`.

End-user/operator docs update:

- Added `SECURITY.md`.
- Updated `docs/installation/filesystem-layout.md` with daemon-created private
  runtime file behavior.
- Updated `docs/installation/installation.md` and
  `docs/installation/systemd-setup.md` with the managed unit `UMask=0077`
  behavior.

End-user/operator skills update:

- None expected.

Lessons:

- Pending.

Follow-up mapping:

- Branch/ruleset enforcement may become a follow-up if not included in this SOW.
- Remaining Codacy dynamic file/path findings in `pkg/engine/output.go` stay on
  the path-safety triage track; they were observed during the URL cleanup but
  are not part of this patch.

## Outcome

Pending.

## Lessons Extracted

Pending.

## Followup

None yet.

## Regression Log

None yet.

Append regression entries here only after this SOW was completed or closed and later testing or use found broken behavior. Use a dated `## Regression - YYYY-MM-DD` heading at the end of the file. Never prepend regression content above the original SOW narrative.
