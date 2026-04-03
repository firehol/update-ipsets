# SOW-0021 | 2026-04-26 | upstream-release-checklist

## Status

completed
release blockers fixed and validated; history squashed to one commit and
post-squash Git-history secret scan passed

## Requirements

Given the repo will be published remotely, when this SOW is complete, then the repo must be audited for references to `Costa`, `/home/costa`, `d1`, and `update-ipsets.sh`.

Given secrets must not be published, when this SOW is complete, then the code and repository history must be searched for API keys, tokens, credentials, and other sensitive values.

Given production `d1` runs the old bash version, when this SOW is complete, then backwards compatibility and operational continuity for installing the Go rewrite over the production system must be verified and documented.

Given the public repo should start cleanly, when the above checks pass and Costa approves, then the repo history must be squashed to a single commit before creating the remote GitHub repo.

## Analysis

Initial sources to consult:

- Full repository tree and history.
- `docs/migration-from-bash.md`.
- `.agents/sow/specs/compatibility.md` and `.agents/sow/specs/files-layout.md`.
- Install/service scripts.
- Legacy bash implementation behavior.

Current known context:

- The old tracker required upstream flattened-history/security release audit.
- Costa explicitly listed strings and secret classes to scan.
- Destructive git/history changes require explicit approval.

Read-only audit evidence gathered on 2026-04-26:

- Current repo shape:
  - `git ls-files` reports 260 tracked files.
  - `git log --oneline --all` reports 218 commits.
  - Current unshipped SOW-only change is the move of SOW-0021 from
    `pending/` to `current/`.
- Official release/security guidance consulted:
  - GitHub "About secret scanning" says GitHub scans public repositories'
    Git history for hardcoded credentials, including API keys, passwords, and
    tokens:
    https://docs.github.com/en/code-security/concepts/secret-security/about-secret-scanning
  - GitHub "Removing sensitive data from a repository" says history rewriting
    has side effects, requires coordination, and recommends revoking/rotating
    true secrets first:
    https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/removing-sensitive-data-from-a-repository
- Secret scan:
  - `gitleaks dir --redact --no-banner --exit-code 0 -f json -r - .`
    found 1 current-tree issue:
    `docs/todo-history/TODO-website-phase3-design.md:1064`,
    rule `generic-api-key`, redacted match `TAB_STORAGE_KEY = 'REDACTED'`.
	  - `gitleaks git --redact --no-banner --exit-code 0 -f json -r - .`
	    found the same issue in history at short commit `63ef474`
	    (`2026-04-07`, message
    `commit everything`), where the file was originally
    `TODO-website-phase3-design.md`.
  - Local inspection of the current line shows a JavaScript constant assignment
    to a browser storage key string, not an obvious live credential. The
    conservative release interpretation is still "tooling red flag that must be
    removed or explicitly accepted before publishing".
  - Hard-pattern manual scan for `AKIA...`, private-key blocks,
    `Authorization:`, and `bearer ` found one tracked source reference:
    `configs/firehol/sources/malware_infrastructure/blueliv_crimeserver_last.yaml:23`
    uses `Authorization: bearer ${BLUELIV_API_KEY}`. This is an environment
    placeholder, not a committed token.
- Current-tree references outside `.agents/`:
  - `AGENTS.md:23` names Costa in the local maintainer-role evidence.
  - `AGENTS.md:133` points to `/home/costa/src/firehol/firehol/`.
  - `sync-from-d1.sh:5`, `sync-from-d1.sh:7`, and `sync-from-d1.sh:11`
    preserve `d1` as the default legacy import host.
  - `docs/migration-from-bash.md:47` documents `sync-from-d1.sh` as an older
    compatibility wrapper.
  - `docs/todo-history/TODO.md:27`, `:30`, `:71`, `:72`, `:74`, `:75`,
    `:371`, `:372`, `:375`, `:376`, and `:1156` contain Costa/local path or
    `update-ipsets.sh` references.
  - UI source comments naming Costa exist in
    `ui/src/index.css`, `ui/src/components/editorial/clear-on-exit.ts`,
    `ui/src/components/feed-detail/section-specs.tsx`,
    `ui/src/components/feed-detail/section-asn.tsx`,
    `ui/src/components/feed-detail/geo-map.tsx`,
    `ui/src/components/feed-detail/section-comparison.tsx`,
    `ui/src/components/feed-sidebar.tsx`,
    `ui/src/components/admin/admin-layout.tsx`, and
    `ui/src/components/admin/current-run.tsx`.
  - `ui/src/components/site-footer.tsx:82` contains the public copyright credit
    `Costa Tsaousis, for FireHOL`; this may be intentional public attribution,
    not automatically a cleanup bug.
  - Test/comment references naming Costa exist in
    `pkg/config/catalog_verify_test.go:1325` and
    `pkg/insights/rules_age_test.go:65`.
  - `pkg/iprange/iter_ops_validate_test.go` contains false-positive `d1`
    variable names in property tests.
  - `pkg/engine/legacy_failure_bootstrap.go:69` and
    `pkg/engine/legacy_failure_bootstrap_test.go:111` mention `import-d1`,
    retained only as transitional compatibility with the older import helper.
- Internal work-record references:
  - `.agents/sow/*.md`, `.agents/skills/project-operations/SKILL.md`, and
    `.agents/skills/project-reviewing/SKILL.md` contain Costa, `d1`,
    `/home/costa`, or `update-ipsets.sh` references.
  - `.agents/sow/.todo-backup/*.md` is tracked and preserves original TODO
    files.
  - `docs/todo-history/*.md` is tracked and preserves archived TODO design
    history.
  - These records are useful locally, but they are noisy and risky for a clean
    public upstream repository unless Costa explicitly wants them published.
- Git history references:
  - `git log -S'/home/costa'` shows local path references across multiple
    commits including `0541f98`, `23ec574`, `d7aec67`, `a4c6b32`,
    `d9c6ee8`, `02c1732`, `b7db742`, and `2c8f5d4`.
  - `git log -S'Costa'` shows references across many UI/SOW/doc commits.
  - `git log -S'update-ipsets.sh'` shows references in `0541f98`, `4ecddd9`,
    and `2c8f5d4`.
  - `git log -S'sync-from-d1'` shows the older helper path across several
    commits, including `0a27ba0`, `3317be4`, `0619195`, and `02c1732`.
  - Conclusion: if the public repo must not expose local development history,
    publishing the existing history is not acceptable. A clean single initial
    commit after tree cleanup is the right release shape.
- Compatibility/continuity evidence:
  - `docs/migration-from-bash.md` documents staged cut-over, backup, state
    preservation, compatibility-output validation, and rollback principles.
  - `.agents/sow/specs/compatibility.md` is the normative compatibility
    contract and requires legacy cache/state import, retained history snapshot
    handling, write compatibility for public artifacts, and public feed-name
    stability.
  - `.agents/sow/specs/files-layout.md` defines `import-bash-version/` as the
    canonical import workspace and allows `import-d1/merged-cache.json` only as
    transition compatibility while the older helper name still exists.
  - `sync-from-bash-version.sh` implements pre-sync backup, staging import,
    production/local-only feed manifests, legacy `.cache` merge, API-key
    extraction into `/opt/update-ipsets/.update-ipsets.env` with mode `600`,
    stale old-Go file cleanup, ownership fix, summary output, and restart only
    if the daemon was running before.
  - `sync-from-d1.sh` is only a compatibility wrapper around
    `sync-from-bash-version.sh` with default `SOURCE_HOST=d1`.
  - `cmd/update-ipsets/cache_merge.go` and
    `cmd/update-ipsets/cache_merge_test.go` test preserving local-only cache
    entries while importing legacy cache state.
  - `pkg/engine/legacy_failure_bootstrap.go` and its tests cover bootstrapping
    failure start timestamps from `import-bash-version/merged-cache.json` and
    fallback to `import-d1/merged-cache.json`.
  - `install.sh` installs under `/opt/update-ipsets`, refreshes config with
    backup behavior, writes the systemd service, and restarts only when the
    service is active or enabled unless `--no-restart` is used.
  - Not yet verified: actual production continuity on `d1`. No production
    access was used.

## Implications and decisions

- History rewrite is destructive and must not happen without Costa approval.
- Production compatibility requires care because `d1` currently runs the bash implementation.
- Secret scanning must cover current tree and history, not just tracked source text.
- The current tree still contains release-noisy personal/local references.
- The existing 218-commit history contains the same classes of references, so
  current-tree cleanup alone is not enough for a clean public repo.
- The tracked `docs/todo-history/` and `.agents/sow/.todo-backup/` trees are
  the highest-risk publication content because they preserve internal planning
  history and include the single Gitleaks finding.
- `sync-from-d1.sh` and `import-d1` are the main `d1` compatibility surface.
  Removing them makes the public tree cleaner but may break local operational
  muscle memory unless the canonical `sync-from-bash-version.sh d1` path is
  accepted.
- Pending Costa decisions before cleanup:
  - whether public source comments should be anonymized even where they only
    explain prior design feedback
  - whether the public footer should keep personal copyright attribution
  - whether tracked historical TODO/SOW/agent records should be published,
    archived privately, or removed from the public tree
  - whether `sync-from-d1.sh` and `import-d1` fallback should be removed now
  - whether production `d1` verification is authorized, and under what exact
    non-destructive constraints
  - whether the final single-commit release should be created as a separate
    clean public branch/repo instead of rewriting this local working branch
- Costa decision on 2026-04-26:
  - no audited item is considered sensitive enough to hide now
  - repository history remains useful and MUST NOT be squashed yet
  - release polish and final squash are deferred until public GitHub push time
  - SOW-0021 is complete as a read-only release audit, not as a cleanup or
    history-rewrite implementation

## Plan

Chunked SOW - reasoning: audit, compatibility, and history rewrite have different risks.

1. `reference-scan` - medium risk
2. `secret-scan` - high risk
3. `compatibility-and-continuity` - high risk
4. `release-checklist` - medium risk
5. `single-commit-plan` - high risk
6. `execute-after-approval` - high risk

## Execution log

2026-04-26:

- Moved SOW-0021 to `current/` after Costa approved the recommendation to start
  the upstream release checklist.
- Scope guard: initial work is read-only repository and history audit. No
  production access, no remote publication, and no history rewrite will happen
  without explicit Costa approval.
- Ran focused current-tree scans for `Costa`, `/home/costa`, `d1`, and
  `update-ipsets.sh`, excluding generated world-map JSON and the local binary.
- Ran focused `.agents/` reference counts for the same strings.
- Ran Gitleaks current-tree and Git-history scans with redaction.
- Ran manual hard-pattern secret scan for private-key blocks, AWS access-key
  shapes, and bearer/authorization literals.
- Reviewed the migration, compatibility, install, legacy cache merge, and
  legacy failure bootstrap surfaces without touching production.
- Costa accepted the audit conclusion: no clear secret or must-hide item was
  found. No files were removed, no production access was used, and no history
  rewrite was performed.

## Validation

- [x] Acceptance criteria evidence
  - Reference scan covered `Costa`, `/home/costa`, `d1`, and
    `update-ipsets.sh` in the current tree and Git history.
  - Secret scan covered current tree and Git history with Gitleaks plus manual
    hard-pattern scans. The only Gitleaks finding was judged to be scanner
    noise, not an obvious live credential.
  - Compatibility and continuity were reviewed through migration docs, specs,
    install script, import helper, cache merge implementation/tests, and legacy
    failure bootstrap implementation/tests.
  - Final squash was explicitly deferred by Costa until public push time.
- [x] Real-use validation evidence
  - N/A by Costa decision: this SOW was completed as a read-only release audit.
    No runtime behavior, install behavior, production state, or public output
    was changed.
- [x] Cross-model reviewer findings (logged + addressed)
  - N/A by Costa decision and current harness rules: no external reviewer was
    requested for this read-only audit. The user accepted the audit result.
- [x] Lessons extracted (or "none, reasoning: ...")
  - See `## Lessons extracted`.
- [x] Same-failure-at-other-scales check
  - Same-class checks were performed by scanning both current tree and Git
    history, plus separating false positives from actual environment-variable
    placeholders.

## Outcome

SOW-0021 completed as a read-only upstream release audit. No security blocker
was found that must be hidden before continuing local work. The repo keeps its
current history for now because it may still be useful; cleanup polish and a
single clean public commit are deferred until the actual GitHub push/release
step.

## Lessons extracted

None requiring project skill or spec updates.

Reasoning: the project-testing skill already requires explicit secret/path
scans for release/security changes, and this SOW only records a one-time
release readiness decision. No product behavior, operator contract, or testing
pattern changed.

## Regression - 2026-05-31

### Requirements

Purpose: prepare the repository for its first public release and production
deployment as a fit-for-purpose v0/beta release: release enough value to users
without pretending the product is final, while preserving feed correctness,
operator safety, cheap public serving, and a clean public source history.

User request:

- Proceed with release blocker cleanup and full validation.
- Squash all commit history so the public repository starts as one commit.

Assistant understanding:

Facts:

- The user accepted that pending product SOWs can remain pending after the first
  public release, but hard release gates must be green.
- The user explicitly approved destructive Git history squashing for this repo.
- Production systems must not be touched without separate explicit approval.

Inferences:

- This SOW now changes from read-only audit to implementation and validation.
- The public release should be treated as v0/beta unless the user later chooses
  stronger wording.

Unknowns:

- Exact remote GitHub repository URL and final tag name are not configured in
  this clone. This does not block local cleanup, validation, or local history
  squash.

Acceptance criteria:

- `make test` passes.
- `.agents/sow/audit.sh` passes.
- Tracked-tree and Git-history secret scans are clean or have recorded,
  justified allowlist handling.
- A root repository license file exists.
- Full release validation gates requested in the readiness review are run and
  results are recorded.
- Repository history is rewritten locally to a single release commit after
  validation and before final report.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The repo is close to release shape, but the current local release gates are
  red:
  - `make test` fails because two engine tests still expect merge `.ipset`
    outputs that are no longer materialized where the tests check them.
  - The architecture posture test rejects unapproved growth in
    `pkg/markdown/context_feed.go`.
  - The SOW audit reports three status/directory mismatches.
  - Secret scanning of a tracked-file snapshot reports current-tree findings in
    docs and SOW evidence.
  - The repo root has no visible license file.
- The Git history still contains all old commits, and the user now wants a
  single public initial commit.

Evidence reviewed:

- `make test` output on 2026-05-31:
  - `pkg/engine/engine_test.go:95` expects `merged.ipset`.
  - `pkg/engine/stress_test.go:152` expects `all_feeds.ipset`.
  - `tools/archposture/collect_test.go:33` rejects large-file growth beyond
    the recorded baseline.
- `.agents/sow/audit.sh` output on 2026-05-31:
  - `SOW-0087` has status `pending` in `pending/`.
  - `SOW-0059` and `SOW-0082` have invalid status `done` in `done/`.
- Tracked-file Gitleaks snapshot on 2026-05-31:
  - `docs/*` curl examples are flagged as `curl-auth-user`.
  - `docs/todo-history/TODO-website-phase3-design.md:1064` is flagged as
    `generic-api-key`.
  - `.agents/sow/done/SOW-0014-20260426-ai-in-the-loop.md` commit-hash
    evidence is flagged as `sourcegraph-access-token`.
- Root license check on 2026-05-31 found no tracked `LICENSE`, `COPYING`, or
  `NOTICE` file in this repo.
- Legacy FireHOL repository contains `COPYING` with GPL-2.0 text.

Affected contracts and surfaces:

- Release process and Git history.
- SOW lifecycle state.
- Test expectations for generated merge artifacts.
- Architecture posture baseline or code layout.
- Operator docs and SOW text that trigger secret scanners.
- Root repository metadata.

Existing patterns to reuse:

- SOW completion and commit rules from `AGENTS.md`.
- Release hygiene rules from `.agents/skills/project-operations/SKILL.md`.
- Validation gates from `.agents/skills/project-testing/SKILL.md`.
- Architecture posture gate and baseline under `tools/archposture/`.
- Existing docs placeholder style for credentials and environment variables.

Risk and blast radius:

- History rewrite is destructive for local commit ancestry. This is approved by
  the user for public release preparation.
- Fixing test expectations incorrectly could hide a real merge publication
  regression; the implementation must verify the actual intended output path,
  not merely delete assertions.
- Secret-scan cleanup must not remove useful operator examples; replace only
  scanner-hostile examples with equivalent safer syntax.
- License selection is a legal/release metadata surface. The current
  evidence-supported action is to add the same GPL-2.0 license text used by the
  legacy FireHOL repository; changing license family later is a separate user
  decision.

Sensitive data handling plan:

- Do not write raw secrets, credentials, bearer tokens, private endpoints, or
  personal data to SOWs, docs, specs, skills, code comments, or commit text.
- Secret-scan evidence in this SOW uses file paths, line numbers, rule names,
  and redacted summaries only.
- Any scanner finding that could be a real credential is removed or rewritten
  before public release; false positives are documented without copying the
  matched token shape.

Sensitive data gate:

- Before closure, rerun tracked-tree and post-squash Git-history secret scans;
  record only redacted evidence and do not commit any raw matched secrets.

Implementation plan:

1. Repair SOW ledger statuses and rerun the SOW audit.
2. Investigate merge `.ipset` test failures against current engine behavior and
   fix code or tests based on the contract.
3. Address architecture posture growth by splitting code or, if justified,
   updating the baseline with SOW evidence.
4. Rewrite scanner-hostile docs/SOW evidence and add root license metadata.
5. Run full release validation.
6. If validation passes, squash local Git history to one release commit with a
   neutral message that does not mention any assistant or vendor.

Validation plan:

- `.agents/sow/audit.sh`
- `make test`
- `make test-tools`
- `make race`
- `make test-strict`
- `make fuzz-replay`
- `make lint`
- `make vulncheck`
- `make staticcheck`
- `make golangci-lint`
- `pnpm --dir ui test`
- `pnpm --dir ui build`
- `make ui-e2e`
- tracked-tree Gitleaks scan
- Git-history Gitleaks scan after squash

Artifact impact plan:

- AGENTS.md: no expected update unless release work reveals a missing durable
  rule.
- Runtime project skills: no expected update unless validation reveals a
  repeatable release-process lesson.
- Specs: no expected update unless merge output contract or release behavior
  changes.
- End-user/operator docs: expected updates for scanner-safe credential examples
  and root license metadata.
- End-user/operator skills: no expected update expected.
- SOW lifecycle: this SOW is reopened in `current/`; on success it returns to
  `done/` with `Status: completed` in the single release commit.

Open-source reference evidence:

- None needed for implementation. This work repairs local release gates and
  repository metadata; the only external reference is the legacy FireHOL
  license text already present in the sibling repository.

Open decisions:

- D1: User approved proceeding with blocker cleanup and full validation.
- D2: User approved squashing all commit history into one public release commit.
- D3: Production deployment is not approved in this turn; do not touch
  production systems.

### Execution Log

2026-05-31:

- Reopened this SOW from `done/` to `current/`.
- Recorded the release purpose, blocker evidence, validation plan, and approved
  single-commit history rewrite.
- Fixed temporal engine test fixtures by pinning test engine clocks near their
  HTTP `Last-Modified` fixture time instead of the wall clock.
- Split `pkg/markdown/context_feed.go` into focused activity, artifact, and
  value helper files to satisfy the architecture posture gate without relaxing
  the baseline.
- Fixed `pkg/web.Run` shutdown ownership so startup entity-artifact work cannot
  keep writing under the configured web directory after `Run` returns.
- Rewrote scanner-hostile docs examples to use admin credential environment
  variables instead of fake inline passwords.
- Rewrote scanner false positives in archived SOW/TODO evidence without copying
  token-shaped strings.
- Added root `COPYING` with the GPL-2.0 license text used by the legacy FireHOL
  repository.
- Added an accessible label to the feed-detail IP-count chart and updated the
  Playwright smoke test to target the chart, not the section header icon.
- Fixed the golangci-lint/staticcheck quickfix finding in merge expansion.

### Validation - 2026-05-31

Acceptance criteria evidence:

- `make test`: passed.
- `.agents/sow/audit.sh`: passed.
- Tracked-tree Gitleaks scan: passed, no leaks found.
- Root license file: `COPYING`, 340 lines, copied from the legacy FireHOL GPL-2.0
  license text.
- Repository history rewrite: completed. `git rev-list --count --all` reports
  1 commit, and all local refs point at the single `Initial release` commit.
- Post-squash Git-history Gitleaks scan: passed. Gitleaks scanned 1 commit and
  found no leaks.

Tests and release gates:

- `make build`: passed.
- `make test`: passed.
- `make test-tools`: passed.
- `make race`: passed.
- `make test-strict`: passed after fixing web startup goroutine ownership.
- `make fuzz-replay`: passed.
- `make lint`: passed.
- `make vulncheck`: passed. It reported 0 called vulnerabilities; it also noted
  vulnerabilities in imported/required packages that this code does not call.
- `make staticcheck`: passed.
- `make golangci-lint`: passed.
- `pnpm --dir ui test`: passed, 13 test files and 39 tests.
- `pnpm --dir ui lint`: passed.
- `pnpm --dir ui build`: passed.
- `make ui-budget`: passed with the existing warning that feed-detail route
  visualization gzip size is below the budget but close.
- `make ui-e2e`: passed, 5 Chromium tests.
- `git diff --check`: passed.

Sensitive data gate:

- Durable artifact edits use redacted evidence, paths, line numbers, and rule
  names only.
- Tracked-tree Gitleaks scan on the intended release tree found no leaks.
- Post-squash Git-history Gitleaks scan passed after the one-commit history
  rewrite.

Artifact maintenance gate:

- AGENTS.md: no update needed; release and SOW lifecycle rules already covered
  the work.
- Runtime project skills: no update needed; the existing testing, operations,
  content-surface, Go, and frontend skills covered the fixes.
- Specs: no update needed; no product contract, file layout, public API, or
  operator behavior changed.
- End-user/operator docs: updated admin `curl` examples to avoid fake inline
  password-shaped strings.
- End-user/operator skills: no update needed; no exported operator skill changed.
- SOW lifecycle: this SOW returns to `done/` with `Status: completed` in the
  single release commit.

Closure:

- History squash completed with commit message `Initial release`.
- Stale non-existent `/tmp` worktree metadata was pruned so old commits were no
  longer reachable from dead worktree refs.
- The stale local `feature/ipv6-and-perf` branch ref now points to the same
  single release commit as `main`.
