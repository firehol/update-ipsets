---
name: project-hygiene
description: "Security, quality, dependency, CI, and GitHub scanner hygiene for update-ipsets. MUST be followed when checking project hygiene, GitHub code scanning, scanner findings, dependency hygiene, CI posture, branch/ruleset enforcement, secret scanning, or supply-chain security."
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

## Required Hygiene Surfaces

Check these surfaces when the user asks for project hygiene, scanner setup, or
scanner finding review:

- GitHub CodeQL setup, analyses, open/fixed/dismissed alerts, query breadth,
  duplicate-analysis risk, and default-vs-advanced ownership.
- GitHub Dependabot alerts, dependency graph/security updates, version-update
  config, and dependency-review pull-request gating.
- GitHub secret scanning, push protection, non-provider patterns, and local
  redacted generic secret scanning.
- GitHub Actions workflow security: token permissions, untrusted checkout
  paths, third-party actions, action pinning, workflow lint, and SARIF upload
  permissions.
- Branch protection or repository rulesets for `main`, including whether
  scanner checks are only advisory or actually enforced.
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

Use GitHub API or `gh` for repository-side evidence:

- workflows and dynamic scanners;
- CodeQL default setup and analyses;
- code scanning alerts in open, fixed, and dismissed states;
- Dependabot alerts;
- secret-scanning alerts;
- repository security settings;
- branch protection and rulesets;
- Actions permissions.

## Finding Handling

- Start with open/blocking findings, but continue through warnings,
  informational findings, fixed-alert regressions, dismissed alerts, scanner
  configuration gaps, and missing enforcement.
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
- every valid blocker and non-blocker outcome;
- same-failure searches;
- remaining accepted baselines or suppressions with evidence;
- whether scanner findings are advisory or enforced;
- whether project skills, specs, docs, or AGENTS.md need updates.
