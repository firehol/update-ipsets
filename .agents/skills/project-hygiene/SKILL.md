---
name: project-hygiene
description: "Security, quality, dependency, CI, GitHub, and Codacy scanner hygiene for update-ipsets. MUST be followed when checking project hygiene, GitHub code scanning, GitHub AI findings, Codacy issues/findings, scanner findings, dependency hygiene, CI posture, branch/ruleset enforcement, secret scanning, or supply-chain security."
---

## Purpose

Project hygiene is a defensive safety net for an AI-generated, highly automated
repository. Treat scanners as primary controls, not as optional advisory noise.

## Mandatory Rule

When checking project hygiene, check the full scanner and repository posture.
Resolve both blocking and non-blocking valid findings.

"Not blocking CI" is not a reason to leave a finding unresolved. Every finding
must end in one of these states:

- fixed and validated;
- explicitly baselined or allowlisted with narrow scope and evidence;
- rejected as false positive or non-goal with file/API evidence in the SOW.

Never hide a finding by broad suppression, alert dismissal, or workflow
exclusion unless the SOW records why it is safe.

Do not rely on the user to provide scanner links. Discover the scanner surfaces
yourself from the repository, GitHub, Codacy, CI checks, and local scanner
configuration.

## Required Hygiene Surfaces

Check these surfaces when the user asks for project hygiene, scanner setup, or
scanner finding review:

- GitHub CodeQL setup, analyses, open/fixed/dismissed alerts, query breadth,
  duplicate-analysis risk, and default-vs-advanced ownership.
- GitHub Security and quality pages, including standard code-quality findings
  and AI findings. These are separate from CodeQL/code-scanning alerts and must
  be triaged separately.
- GitHub Dependabot alerts, dependency graph/security updates, version-update
  config, and dependency-review pull-request gating.
- Dependabot cooldown alignment with package-manager supply-chain policy. For
  pnpm v11+ UI dependencies, Dependabot npm updates must not create PRs for
  package versions younger than pnpm's active release-age gate unless the SOW
  records a narrow emergency exception.
- GitHub secret scanning, push protection, non-provider patterns, and local
  redacted generic secret scanning.
- GitHub Actions workflow security: token permissions, untrusted checkout
  paths, third-party actions, action pinning, workflow lint, and SARIF upload
  permissions.
- Branch protection or repository rulesets for `main`, including whether
  scanner checks are only advisory or actually enforced.
- Codacy Cloud repository status: dashboard grade, issue totals, issue
  category/severity/language breakdown, current and ignored issues, security
  findings, pull-request analysis, quality gate, coverage status, tools,
  patterns, coding standard, branch state, and whether findings are advisory or
  enforced.
- Codacy configuration:
  - root `.codacy.yml` / `.codacy.yaml` for Codacy Cloud path/language/engine
    configuration such as `exclude_paths`, `include_paths`, languages, and tool
    path scopes;
  - `.codacy/codacy.config.json` for Codacy Analysis CLI local tool/pattern
    configuration;
  - Cloud tool/pattern changes require Codacy UI/API/CLI import or equivalent
    Cloud action. Do not assume committing `.codacy/codacy.config.json` changes
    Cloud analysis.
  - Root `.codacy.yml` can configure or exclude paths for a tool, but it cannot
    enable or disable tools. If a tool is enabled by a Codacy coding standard,
    repository-level tool toggles cannot disable it; change the coding standard
    only with explicit SOW evidence because it can affect other repositories.
- OpenSSF Scorecard or equivalent supply-chain posture checks.
- Local quality/security gates from `project-testing`, including Go, UI,
  nested-tool, vulnerability, static-analysis, race, and strict-test commands
  relevant to the changed surface.
- Shell and workflow hygiene for tracked scripts/workflows.

## Command Pattern

Prefer Makefile targets when they exist. If a target does not exist yet, use
the direct equivalent and record that gap:

- `make vulncheck`
- `make staticcheck`
- `make golangci-lint`
- `make actionlint`
- `make shellcheck`
- `make gitleaks`
- `go run github.com/rhysd/actionlint/cmd/actionlint@<pinned-version> .github/workflows/*.yml`
- `shellcheck $(git ls-files '*.sh')`
- `go run github.com/zricethezav/gitleaks/v8@<pinned-version> detect --no-banner --redact=100 --source . --exit-code 2`
- `codacy repository gh firehol update-ipsets --output json`
- `codacy issues gh firehol update-ipsets --branch main --overview --output json`
- `codacy issues gh firehol update-ipsets --branch main --severities Critical,High --output json`
- `codacy findings gh firehol update-ipsets --severities Critical,High --output json`
- `codacy tools gh firehol update-ipsets --output json`
- `codacy pull-request gh firehol update-ipsets <pr-number> --output json`
- `codacy-analysis discover --output-format json --output /tmp/codacy-discover.json`
- `codacy-analysis analyze --inspect --output-format json`
- `codacy-analysis analyze --diff --output-format json --output /tmp/codacy-diff.json`

Use GitHub API or `gh` for repository-side evidence:

- workflows and dynamic scanners;
- CodeQL default setup and analyses;
- code scanning alerts in open, fixed, and dismissed states;
- Dependabot alerts;
- secret-scanning alerts;
- repository security settings;
- branch protection and rulesets;
- Actions permissions.

Use Codacy Cloud CLI, Codacy Analysis CLI, or the authenticated Codacy UI for
Codacy evidence. If the CLIs are unavailable or unauthenticated, record that
gap and use the browser/UI evidence that is available. Do not write Codacy API
tokens or credentials to durable artifacts.

## Finding Handling

- Start with open/blocking findings, but continue through warnings,
  informational findings, fixed-alert regressions, dismissed alerts, scanner
  configuration gaps, and missing enforcement.
- For Codacy, start with the issue overview and top patterns by count. Separate
  real findings from configuration mismatch before fixing thousands of issues.
  Wrong-stack, deprecated-tool, generated-file, vendored-file, fixture, and
  project-convention mismatches should be fixed through narrow tool/pattern/path
  configuration, not broad suppression.
- For Go file-permission findings, separate test fixtures from production
  writers before changing modes. Test fixtures should normally use restrictive
  `0600` files and `0700` directories. Daemon-owned generated runtime and
  publication outputs should also default to `0600` files and `0700`
  directories because public HTTP availability is served by the daemon, not by
  POSIX world-read bits. Keep broader group access only where a documented
  install/operator contract requires it, such as root-owned binary/config paths
  readable or executable by the `iplists` group.
- When Codacy runs both `eslint-8` and `eslint-9`, treat the repository's
  checked-in ESLint v9 flat config as the owner for React/TypeScript UI code.
  If legacy `eslint-8` is enforced by a coding standard, prefer a narrow
  engine-specific `.codacy.yml` path exclusion for UI paths covered by ESLint9
  instead of editing the organization coding standard or disabling security
  scanners.
- For modern Node `.mjs` maintenance scripts, do not rewrite ES modules,
  `async`/`await`, `const`, or Node builtin imports into obsolete JavaScript only
  to satisfy legacy `eslint-8` compatibility rules. Add or verify current
  ESLint9/root-config coverage for the scripts, then use a narrow `eslint-8`
  path exclusion when the legacy engine is the mismatch. Still fix true script
  risks found during triage, such as destructive path cleanup without workspace
  bounds, unsafe regexes, shell injection, or untrusted subprocess execution.
- For Dependabot npm/pnpm PRs, preserve release-age supply-chain protection.
  If CI fails with `ERR_PNPM_MINIMUM_RELEASE_AGE_VIOLATION`, do not weaken the
  installer policy to make the PR pass. Align Dependabot cooldown with pnpm,
  wait for the release-age window, or explicitly document a narrow emergency
  exception.
- Security, Critical, and High findings stay enabled unless another active
  scanner/pattern covers the same security concern with better precision. If a
  security rule is noisy, prefer file-specific exclusion or a documented false
  positive over disabling the whole concern.
- For Python subprocess findings, first remove subprocess use when an
  in-process project API can do the same work. When subprocess is required for
  fixed tools such as `git` or `gh`, resolve the executable path with
  `shutil.which`, pass arguments as a list, keep `shell=False`, set an explicit
  `check` policy, close stdin with `subprocess.DEVNULL`, and use only narrow
  `# nosec` suppressions with a rationale tied to the fixed command surface.
- Search for the same failure class before committing a fix.
- Prefer fixing findings over suppressing them.
- If suppression is necessary, make it narrow, durable, and justified by
  evidence. Broad path ignores for generated or scratch artifacts are allowed
  only when the ignored path is not a source or release input.
- Do not write raw secret values or scanner payloads to SOWs, specs, docs,
  skills, code comments, or workflow comments. Record only redacted evidence,
  rule IDs, paths, line numbers, severities, and states.

## Closure Gate

Before closing a hygiene SOW, record:

- scanner commands and GitHub API checks run;
- Codacy Cloud/UI/CLI checks run, including issue counts before/after and any
  tool/pattern/path configuration changes;
- every valid blocker and non-blocker outcome;
- same-failure searches;
- remaining accepted baselines or suppressions with evidence;
- whether scanner findings are advisory or enforced;
- whether project skills, specs, docs, or AGENTS.md need updates.
