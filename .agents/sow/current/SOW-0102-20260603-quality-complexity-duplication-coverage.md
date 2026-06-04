# SOW-0102 - Quality Complexity Duplication Coverage

## Status

Status: in-progress

Sub-state: twentieth implementation slice validated; output metadata PR pending

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
2. Quality target model
   - Decision: target scanner-clean complexity and duplication with meaningful coverage around `90%`, not literal zero complexity or duplication.
   - Evidence: user approved the pragmatic quality-front plan after reviewing current coverage, complexity, and duplication evidence.
   - Implication: fix true complexity, duplication, and coverage gaps that improve maintainability or confidence.
   - Risk: intentional optimized duplication, generated artifacts, and high-risk broad refactors may need narrow documentation or scanner tuning instead of mechanical rewrites.

## Plan

1. Baseline local and remote quality metrics.
2. Pick one focused slice from measured hotspots.
3. Prefer low-risk behavior coverage for stable public/operator contracts before high-risk engine refactors.
4. Remove duplication when the shared abstraction improves clarity without hurting optimized paths.
5. Reduce complexity in artifact/engine paths only after nearby behavior tests prove the contract.
6. Validate and check scanners after every slice.
7. Update SOW, skills/specs only if needed, and close through the normal SOW lifecycle.

## Pre-Implementation Gate - Slice 2

Status: ready.

Problem / root-cause model:

- Facts: after the first slice, root Go coverage is `70.8%`; low measured package coverage includes `cmd/update-ipsets` at `18.4%`, `internal/observability` at `30.3%`, `pkg/kernel` at `32.8%`, `pkg/asnloc` at `44.9%`, and `pkg/systemd` at `51.5%`.
- Facts: the largest production complexity hotspots are in engine/config/web/downloader paths where broad refactors can affect artifact integrity, publication, serving, or config contracts.
- Working theory: the next pragmatic slice should improve low-coverage command, observability, and systemd behavior before engine refactoring. This increases confidence and coverage with low blast radius and avoids touching optimized `pkg/iprange` internals or high-risk artifact paths too early.

Evidence reviewed:

- `go tool cover -func=coverage.out`
- `tools/archposture` large-function output
- `cmd/update-ipsets/*.go`
- `cmd/update-ipsets/*_test.go`
- `pkg/systemd/notify.go`
- `pkg/systemd/notify_test.go`
- `internal/observability/observability.go`
- `internal/observability/observability_test.go`
- project coding, testing, hygiene, Go best-practices, and Go behavioral-testing skills.

Affected contracts and surfaces:

- `cmd/update-ipsets`: command dispatch, usage/version output, validation-only failure paths, logger level selection, set-expression parsing, and local-only name-list parsing.
- `pkg/systemd`: readiness/status/stopping/watchdog notification payloads, invalid/no-op notification behavior, and watchdog interval parsing.
- `internal/observability`: environment opt-in parsing, metric interval options, shutdown aggregation, tracing helpers, designed metric recording, bounded API recalculation labels, and slog tee fan-out.
- SOW only; specs/docs are not expected to change because no runtime behavior is intended to change.

Existing patterns to reuse:

- Existing same-package command tests for package `main`.
- Existing external package systemd tests that use a real Unix datagram socket.
- Existing same-package observability tests for OpenTelemetry policy helpers and package-level instrumentation state.
- `t.TempDir`, `t.Setenv`, table-driven tests, and small inline fixtures.
- Behavior-first assertions on exit code, stdout/stderr, return errors, notification payloads, metric output, log fan-out, and file outputs.

Risk and blast radius:

- Command tests temporarily replace `os.Stdout` and `os.Stderr`; tests must not run in parallel and must restore globals.
- Systemd tests use Unix sockets and should keep timeouts bounded.
- Observability tests temporarily replace package-level meter/instrument globals; tests must restore them and must not run in parallel where global state is mutated.
- No daemon engine creation path should be exercised unless the test is intentionally validating an early parse/validation failure.
- No public serving, scheduler, pipeline artifact generation, install behavior, or optimized `pkg/iprange` internals should change.

Sensitive data handling plan:

- This slice uses synthetic command arguments, synthetic local file names, and temporary Unix sockets only.
- No secrets, tokens, cookies, private endpoints, customer data, or personal data are needed.
- Durable artifacts will record only command names, paths, metrics, and validation outcomes.

Implementation plan:

1. Add command behavior tests for `run`, `runQuery`, `runEnable`, `runCacheMerge`, `parseSetExpression`, `readNameList`, and `newLogger`.
2. Add systemd behavior tests for notification payload construction, no-op paths, invalid socket errors, and watchdog interval parsing.
3. Add observability behavior tests for environment parsing, shutdown handling, tracing helper defaults, designed metric recording, API recalculation label bounding, and slog tee fan-out.
4. Avoid engine-backed happy paths in this slice; leave them for a later engine/API fixture slice.
5. Re-run package coverage for `cmd/update-ipsets`, `internal/observability`, and `pkg/systemd`, then root `make coverage`.
6. Re-run relevant lint/static checks before committing.

Validation plan:

- `go test ./cmd/update-ipsets ./pkg/systemd ./internal/observability`
- `go test -coverprofile=/tmp/update-ipsets-slice2.cover -covermode=atomic ./cmd/update-ipsets ./pkg/systemd ./internal/observability`
- `go run ./tools/archposture -root .`
- `make lint`
- `make staticcheck`
- `make golangci-lint`
- `make coverage`
- `git diff --check -- ...`

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if this slice finds a durable testing/hygiene lesson.
- Specs: no update expected; behavior is preserved.
- End-user/operator docs: no update expected; CLI/systemd semantics are unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW in `.agents/sow/current/` until the quality campaign reaches an appropriate closure point.

Open decisions:

- None. The user approved the pragmatic quality target model and behavior-preserving quality work.

## Pre-Implementation Gate - Slice 3

Status: ready.

Problem / root-cause model:

- Facts: after the second slice, `pkg/kernel` remains at `32.8%` statement coverage.
- Facts: `pkg/kernel` contains operator-facing ipset replacement logic, but its exported behavior calls real netlink/kernel operations that should not require root privileges or host ipset state in unit tests.
- Working theory: a small unexported operation seam can make `ApplyIfLoaded` and loaded-set mapping testable with a fake netlink backend while preserving production behavior.

Evidence reviewed:

- `pkg/kernel/ipset_linux.go`
- `pkg/kernel/ipset_stub.go`
- `pkg/kernel/ipset_test.go`
- `go test -coverprofile=/tmp/update-ipsets-kernel-baseline.cover -covermode=atomic ./pkg/kernel`
- `go tool cover -func=/tmp/update-ipsets-kernel-baseline.cover`
- `github.com/vishvananda/netlink` local module source for `IPSetResult`, `IPSetEntry`, and ipset function signatures.

Affected contracts and surfaces:

- `pkg/kernel`: `LoadedSets`, `ApplyIfLoaded`, temporary ipset creation, entry parsing, destroy-on-error cleanup, and loaded-set mapping.
- No daemon, scheduler, installer, public serving, UI, or config contracts are expected to change.
- SOW only; specs/docs are not expected to change because the public behavior is unchanged.

Existing patterns to reuse:

- Same-package tests for Linux-only internal helpers already exist in `pkg/kernel/ipset_test.go`.
- Small handwritten fakes are preferred over generated mocks.
- Error assertions should check observable returned errors and side effects on the fake operation ledger.

Risk and blast radius:

- The production path must continue using `github.com/vishvananda/netlink` exactly as before.
- Temporary set cleanup must still run after create success even when parse/add/swap fails.
- Tests must not call real kernel ipset operations.
- This slice is Linux-specific because the production ipset implementation is Linux-specific.

Sensitive data handling plan:

- This slice uses synthetic set names and private documentation-range IPs only.
- No secrets, tokens, cookies, private endpoints, customer data, or personal data are needed.
- Durable artifacts will record only package names, coverage values, and validation outcomes.

Implementation plan:

1. Introduce an unexported Linux-only ipset operation interface backed by netlink.
2. Move loaded-set mapping and apply-if-loaded orchestration into unexported helpers that accept that interface.
3. Add fake-backed tests for loaded-set mapping, not-loaded no-op behavior, successful replace behavior, invalid entry handling, create/add/swap errors, cleanup, and helper edge cases.
4. Re-run package coverage for `pkg/kernel`, then root coverage and relevant lint/static checks.

Validation plan:

- `go test ./pkg/kernel`
- `go test -coverprofile=/tmp/update-ipsets-kernel.cover -covermode=atomic ./pkg/kernel`
- `go tool cover -func=/tmp/update-ipsets-kernel.cover`
- `make lint`
- `make staticcheck`
- `make golangci-lint`
- `CI=true make coverage`
- `go run ./tools/archposture -root .`
- `git diff --check -- ...`

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if this slice finds a durable testing/hygiene lesson.
- Specs: no update expected; behavior is preserved.
- End-user/operator docs: no update expected; kernel/ipset semantics are unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW in `.agents/sow/current/` until the quality campaign reaches an appropriate closure point.

Open decisions:

- None. The user approved the pragmatic quality target model and behavior-preserving quality work.

## Pre-Implementation Gate - Slice 4

Status: ready.

Problem / root-cause model:

- Facts: after the third slice merged to `main`, root Go coverage is `71.7%`; `pkg/asnloc` remains at `44.9%` statement coverage.
- Facts: uncovered `pkg/asnloc` areas include public `Database` wrappers, count-feed range walking, bogon splitting, path-based text loaders, and nil/error handling.
- Working theory: synthetic range-table tests can materially improve ASN coverage and confidence without requiring MMDB files, network access, large fixtures, or engine changes.

Evidence reviewed:

- `go test -coverprofile=/tmp/update-ipsets-asnloc-baseline.cover -covermode=atomic ./pkg/asnloc`
- `go tool cover -func=/tmp/update-ipsets-asnloc-baseline.cover`
- `pkg/asnloc/asnloc.go`
- `pkg/asnloc/backend_rangetable.go`
- `pkg/asnloc/loader_iptoasn.go`
- `pkg/asnloc/loader_caida.go`
- existing `pkg/asnloc/*_test.go`

Affected contracts and surfaces:

- `pkg/asnloc`: `Open` for text-backed providers, `Close`, `Lookup`, `Stats`, `CountFeed`, `CountFeedWithBogons`, range-table miss/hit behavior, and helper edge cases.
- No daemon, scheduler, public serving, UI, install, config, or generated artifact contracts are expected to change.
- SOW only; specs/docs are not expected to change because the public behavior is unchanged.

Existing patterns to reuse:

- Same-package tests already exist for ASN parser and range-table internals.
- Small inline text fixtures and `t.TempDir` are already used in project tests.
- `iprange.New` provides a small in-memory `RangeSource` for count-feed tests.

Risk and blast radius:

- Tests must not depend on local MMDB provider files.
- Tests must avoid very large ranges that make failures slow to diagnose.
- No production behavior change is expected in this slice.

Sensitive data handling plan:

- This slice uses RFC 5737 documentation IPv4 ranges and synthetic ASNs only.
- No secrets, tokens, cookies, private endpoints, customer data, or personal data are needed.
- Durable artifacts will record only package names, coverage values, and validation outcomes.

Implementation plan:

1. Add wrapper tests for nil `Database`, `Close`, `Lookup`, `Stats`, and text-backed `Open`.
2. Add range-table count tests for attributed, unknown, and bogon-split feed ranges.
3. Add loader path tests using temporary `iptoasn` and CAIDA text files.
4. Add range-table/helper edge-case tests where coverage shows simple meaningful gaps.
5. Re-run package coverage for `pkg/asnloc`, then root coverage and relevant lint/static checks.

Validation plan:

- `go test ./pkg/asnloc`
- `go test -coverprofile=/tmp/update-ipsets-asnloc.cover -covermode=atomic ./pkg/asnloc`
- `go tool cover -func=/tmp/update-ipsets-asnloc.cover`
- `make lint`
- `make staticcheck`
- `make golangci-lint`
- `CI=true make coverage`
- `go run ./tools/archposture -root .`
- `git diff --check -- ...`

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if this slice finds a durable testing/hygiene lesson.
- Specs: no update expected; behavior is preserved.
- End-user/operator docs: no update expected; ASN semantics are unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW in `.agents/sow/current/` until the quality campaign reaches an appropriate closure point.

Open decisions:

- None. The user approved the pragmatic quality target model and behavior-preserving quality work.

## Pre-Implementation Gate - Slice 12

Status: ready.

Problem / root-cause model:

- Facts: after PR #15 merged to `main`, local `main` matches `origin/main` at `a25d39b68b55`.
- Facts: `tools/archposture` reports `615` source files, `125937` source lines, `54` large files, and `39` large functions.
- Facts: the next highest production large function is `pkg/web/admin_manifest.go:134` `buildFeedManifest` at `217` lines with complexity `29`.
- Facts: baseline `pkg/web` coverage is `78.3%` by `go tool cover`; `buildFeedManifest` coverage is `86.8%`.
- Working theory: `buildFeedManifest` mixes several independent responsibilities: processed-date lookup, runtime directory normalization, database/provider-source file enumeration, non-database base file enumeration, web secondary artifact enumeration, provider fan-out enumeration, binary/latest file enumeration, downloader history snapshot enumeration, and summary rollup. Extracting those phases into focused helpers should reduce complexity without changing the admin API contract.

Evidence reviewed:

- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice12-baseline.json`
- `jq '{source:.source, large_files:(.large_files|length), large_functions:(.large_functions|length), targets:(.large_functions | map(select((.path // "") == "pkg/web/admin_manifest.go")))}' /tmp/update-ipsets-archposture-slice12-baseline.json`
- `go test -coverprofile=/tmp/update-ipsets-web-slice12-baseline.cover -covermode=atomic ./pkg/web`
- `go tool cover -func=/tmp/update-ipsets-web-slice12-baseline.cover`
- `pkg/web/admin_manifest.go`
- `pkg/web/admin_manifest_test.go`
- `pkg/web/http_test_helpers_test.go`
- `.agents/sow/specs/admin-ui.md`
- `.agents/sow/specs/files-layout.md`
- `.agents/sow/specs/integrity.md`
- `.agents/sow/specs/operating-principles.md`

Affected contracts and surfaces:

- `GET /api/v1/admin/feeds/{name}/manifest` response fields: feed, processed date, file rows, row kind/provider/required/existence/stale state, and summary totals.
- Admin manifest expectations for base files, database provider source files, web secondary files, geo/ASN/bogon provider fan-out artifacts, binary latest files, and optional history snapshots.
- No public serving, pipeline writers, config schema, UI source, install behavior, or artifact generation behavior is expected to change.
- SOW only; specs/docs are not expected to change because behavior is preserved.

Existing patterns to reuse:

- Existing `TestBuildFeedManifestRequiresConfiguredProviderFanOutArtifacts` validates configured provider fan-out file expectations.
- `statManifestFile`, `daemonRoot`, and `relOrPath` already isolate file-state and path-display behavior.
- Web route tests use `newWebHTTPTestServer` for route-level behavior when needed, but this slice can stay mostly focused on manifest builder output because the handler is a thin validation wrapper.
- Recent complexity slices keep the top-level function as orchestration and move file-family enumeration into private helpers.

Risk and blast radius:

- Admin manifest is an operator trust surface. A behavior change can hide missing required artifacts, mark the wrong files stale, or show confusing paths.
- Database feeds and non-database feeds intentionally have different required files; this distinction must remain.
- Optional files must remain optional, especially enable markers, raw sources, setinfo, and history snapshots.
- Provider fan-out enumeration must stay config-driven, not hardcoded to feed/provider names.

Sensitive data handling plan:

- This slice uses local synthetic test fixtures, temporary directories, file names, and structural metrics only.
- No secrets, tokens, cookies, private endpoints, customer data, raw external IP lists, or personal data are needed.
- Durable artifacts will record only package paths, metrics, test outcomes, and sanitized behavior evidence.

Implementation plan:

1. Keep `buildFeedManifest` as the top-level manifest assembly function.
2. Extract processed-date lookup, manifest row creation, base/provider/canonical file enumeration, web secondary artifact enumeration, provider fan-out enumeration, binary/history snapshot enumeration, and summary rollup into focused private helpers.
3. Preserve existing row ordering, kind/provider labels, required/optional policy, stale policy, and relative path calculation.
4. Add or adjust behavior tests only if inspection finds uncovered branches affected by extraction.
5. Re-run package coverage, architecture posture, strict web tests, and repository quality gates.

Validation plan:

- `go test ./pkg/web`
- `go test -coverprofile=/tmp/update-ipsets-web-slice12.cover -covermode=atomic ./pkg/web`
- `go tool cover -func=/tmp/update-ipsets-web-slice12.cover`
- `go test -shuffle=on -count=3 ./pkg/web`
- `go run ./tools/archposture -root .`
- `make lint`
- `make staticcheck`
- `make golangci-lint`
- `CI=true make coverage`
- `make test-strict`
- `git diff --check -- ...`

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if this slice finds a durable admin-manifest lesson.
- Specs: no update expected; behavior is preserved.
- End-user/operator docs: no update expected; admin manifest semantics are unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW in `.agents/sow/current/` until the quality campaign reaches an appropriate closure point.

Open decisions:

- None. The user approved behavior-preserving quality work and the next measured production hotspot is clear from local evidence.

## Pre-Implementation Gate - Slice 11

Status: ready.

Problem / root-cause model:

- Facts: after PR #14 merged to `main`, local `main` matches `origin/main` at `acb3ed0d41ab`.
- Facts: `tools/archposture` reports `612` source files, `126072` source lines, `55` large files, and `41` large functions.
- Facts: two of the next highest production large functions live in the same file and surface: `pkg/engine/home_entity_builders.go:644` `(*Engine).buildASNDetailSidecar` at `231` lines with complexity `44`, and `pkg/engine/home_entity_builders.go:430` `(*Engine).buildCountryDetailSidecar` at `213` lines with complexity `42`.
- Facts: baseline `pkg/engine` coverage is `71.2%`; `buildASNDetailSidecar` coverage is `83.6%`; `buildCountryDetailSidecar` coverage is `82.9%`.
- Working theory: these functions are already covered by real entity artifact tests but still duplicate aggregation, sorting, provider setup, matching-feed collection, and cross-provider enrichment logic. Existing builder types in `pkg/engine/home_entity_precompute.go` already encapsulate the country/ASN detail sidecar aggregation contract, so the live builders should delegate to those helpers instead of hand-assembling totals.

Evidence reviewed:

- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice11-baseline.json`
- `jq '{source:.source, large_files:(.large_files|length), large_functions:(.large_functions|length), targets:(.large_functions | map(select((.path // "") == "pkg/engine/home_entity_builders.go")))}' /tmp/update-ipsets-archposture-slice11-baseline.json`
- `go test -coverprofile=/tmp/update-ipsets-engine-slice11-baseline.cover -covermode=atomic ./pkg/engine`
- `go tool cover -func=/tmp/update-ipsets-engine-slice11-baseline.cover`
- `pkg/engine/home_entity_builders.go`
- `pkg/engine/home_entity_precompute.go`
- `pkg/engine/home_detail_test.go`
- `pkg/engine/entity_artifacts_write.go`
- `pkg/engine/entity_feed_sidecar.go`
- `.agents/sow/specs/integrity.md`
- `.agents/sow/specs/pipeline.md`
- `.agents/sow/specs/files-layout.md`
- `.agents/sow/specs/operating-principles.md`

Affected contracts and surfaces:

- `pkg/engine`: live country/ASN detail sidecar builders used by detail API helpers and entity artifact rebuild paths.
- Private entity detail sidecars under `lib/entities/countries` and `lib/entities/asns`.
- Public country/ASN detail payload materialization that consumes those sidecars.
- No config schema, downloader behavior, scheduler queue behavior, public request-time generation, UI source, install behavior, or artifact schema change is expected.
- SOW only; specs/docs are not expected to change because behavior is preserved.

Existing patterns to reuse:

- `countryDetailBuilder` and `asnDetailBuilder` in `pkg/engine/home_entity_precompute.go` already own feed/category/maintainer/top-ASN/top-country aggregation and stable sort behavior for precomputed entity detail sidecars.
- Existing `TestRebuildEntityArtifactsWritesPrecomputedCountryAndASNPayloads` drives real rebuild outputs and validates country/ASN indexes, public ASN detail payloads, private feed sidecars, and joint country/ASN counts.
- Existing `TestRefreshEntityArtifactsForFeedUpdatesSurgicallyPatchesAggregates` and health-transition tests cover downstream entity detail persistence and refresh behavior.
- Recent complexity slices keep public methods as small orchestration wrappers and move phase logic into focused private helpers without changing output schemas.

Risk and blast radius:

- Entity detail sidecars are production public-serving inputs. A behavior change can alter country/ASN totals, top categories, top maintainers, top ASNs/countries, country distributions, or provider labels.
- Live builders must preserve the current behavior of returning valid empty country/ASN detail payloads when no feeds match, and an empty ASN sidecar when no ASN provider is configured.
- Cross-provider enrichment must keep current error-tolerant behavior: skip unreadable set files or failed provider counts rather than failing the whole detail builder.
- Public requests must remain cache-first readers; no request-time broad generation may be introduced.

Sensitive data handling plan:

- This slice uses local synthetic test fixtures and structural metrics only.
- No secrets, tokens, cookies, private endpoints, customer data, raw external IP lists, or personal data are needed.
- Durable artifacts will record only package paths, metrics, test outcomes, and sanitized behavior evidence.

Implementation plan:

1. Keep `buildCountryDetailSidecar` and `buildASNDetailSidecar` as the live public package helpers for their existing callers.
2. Extract provider setup and feed matching into focused private helpers where it reduces complexity without hiding behavior.
3. Reuse `countryDetailBuilder` and `asnDetailBuilder` for feed/category/maintainer/detail aggregation instead of duplicating sorting and total construction.
4. Extract country-to-ASN enrichment and ASN-to-country enrichment loops into focused helpers that preserve best-effort skip behavior.
5. Add or adjust behavior tests only if inspection finds an uncovered branch affected by the extraction.
6. Re-run package coverage, architecture posture, strict engine tests, and repository quality gates.

Validation plan:

- `go test ./pkg/engine`
- `go test -coverprofile=/tmp/update-ipsets-engine-slice11.cover -covermode=atomic ./pkg/engine`
- `go tool cover -func=/tmp/update-ipsets-engine-slice11.cover`
- `go test -shuffle=on -count=3 ./pkg/engine`
- `go run ./tools/archposture -root .`
- `make lint`
- `make staticcheck`
- `make golangci-lint`
- `CI=true make coverage`
- `make test-strict`
- `git diff --check -- ...`

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if this slice finds a durable entity detail builder lesson.
- Specs: no update expected; behavior is preserved.
- End-user/operator docs: no update expected; entity detail semantics are unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW in `.agents/sow/current/` until the quality campaign reaches an appropriate closure point.

Open decisions:

- None. The user approved behavior-preserving quality work and the next measured production hotspot is clear from local evidence.

## Pre-Implementation Gate - Slice 10

Status: ready.

Problem / root-cause model:

- Facts: after PR #13 merged to `main`, local `main` matches `origin/main` at `8150a4c007cd`.
- Facts: `tools/archposture` reports `611` source files, `126013` source lines, `56` large files, and `42` large functions.
- Facts: the highest remaining production large function is `pkg/downloader/downloader.go:112` `(*Client).Fetch` at `231` lines with complexity `48`; `pkg/downloader/downloader.go` is `606` lines.
- Facts: baseline `pkg/downloader` coverage is `66.8%`; `Fetch` coverage is `93.8%`.
- Working theory: `Fetch` is well covered but mixes dispatch, URL validation, curl-like option application, HTTP request construction, conditional headers, response classification, streaming, hashing, size limit enforcement, empty-body policy, modified-time extraction, same-body detection, temp-file ownership, and observability in one function. Extracting focused helpers should reduce measured complexity without changing downloader behavior.

Evidence reviewed:

- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice10-baseline.json`
- `jq '{source:.source, large_files:(.large_files|length), large_functions:(.large_functions|length), downloader_large_functions:(.large_functions | map(select((.path // "") == "pkg/downloader/downloader.go"))), downloader_large_files:(.large_files | map(select((.path // "") == "pkg/downloader/downloader.go"))) }' /tmp/update-ipsets-archposture-slice10-baseline.json`
- `go test -coverprofile=/tmp/update-ipsets-downloader-slice10-baseline.cover -covermode=atomic ./pkg/downloader`
- `go tool cover -func=/tmp/update-ipsets-downloader-slice10-baseline.cover`
- `pkg/downloader/downloader.go`
- `pkg/downloader/downloader_test.go`
- `pkg/downloader/edge_cases_test.go`
- `pkg/downloader/localcopy_test.go`
- `pkg/downloader/internal_test.go`

Affected contracts and surfaces:

- `pkg/downloader`: HTTP(S), `file:`, `copyfile`, and `internal://` fetch behavior; curl-like options; headers; referer; basic auth; If-Modified-Since; HTTP status handling; response streaming; max download size; empty body handling; Last-Modified handling; SHA-256 body hash; same-body detection; temp-file ownership and cleanup; and observability metrics/spans.
- No config schema, engine pipeline, scheduler, public serving, UI, install behavior, or generated artifact contract is expected to change.
- SOW only; specs/docs are not expected to change because behavior is preserved.

Existing patterns to reuse:

- Existing downloader tests already drive `Fetch` through real `httptest` servers, temporary files, local copy paths, internal sources, redirects, bad schemes, max-size edges, same-body detection, and POST/header behavior.
- Project pattern from recent slices: keep public method behavior intact, move private phases to focused files/helpers, and rely on existing behavior tests for equivalence.
- Existing temp-file helper `createGeneratedTemp`, hashing helper `fileHashEquals`, and local/internal fetch helpers.

Risk and blast radius:

- Downloader behavior is production input acquisition. A subtle change can break upstream feeds, conditional downloads, POST-based feeds, local copies, or size-limit safety.
- Temp file cleanup and ownership must not regress; failed downloads and same-body results must not leave owned temp files behind.
- SSRF scheme restrictions and redirect safety must remain intact.
- Max download size must continue to reject after reading only `limit + 1` bytes.
- Observability counters/spans must continue to report status, bytes, duration, and errors.

Sensitive data handling plan:

- This slice uses synthetic URLs, `httptest` servers, temporary files, and structural metrics only.
- No secrets, tokens, cookies, private endpoints, customer data, raw external IP lists, or personal data are needed.
- Durable artifacts will record only package paths, metrics, test outcomes, and sanitized behavior evidence.

Implementation plan:

1. Keep `Fetch` as the public observability wrapper and dispatch to focused private helpers.
2. Extract downloader dispatch for `copyfile`, empty URL, `internal://`, `file:`, and HTTP(S) URL validation.
3. Extract HTTP request building for method/body, headers, referer, default content type, basic auth, and conditional headers.
4. Extract HTTP response classification and body streaming into focused helpers that preserve max-size, empty-body, hash, Last-Modified, same-body, and temp-file cleanup behavior.
5. Move HTTP-specific fetch helpers to a focused downloader file so `downloader.go` drops below the large-file threshold.
6. Add or adjust behavior tests only if inspection finds a meaningful uncovered branch affected by the extraction.
7. Re-run package coverage, architecture posture, strict downloader tests, and repository quality gates.

Validation plan:

- `go test ./pkg/downloader`
- `go test -coverprofile=/tmp/update-ipsets-downloader-slice10.cover -covermode=atomic ./pkg/downloader`
- `go tool cover -func=/tmp/update-ipsets-downloader-slice10.cover`
- `go run ./tools/archposture -root .`
- `make lint`
- `make staticcheck`
- `make golangci-lint`
- `CI=true make coverage`
- `make test-strict`
- `git diff --check -- ...`

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if this slice finds a durable downloader refactor/testing lesson.
- Specs: no update expected; behavior is preserved.
- End-user/operator docs: no update expected; downloader semantics are unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW in `.agents/sow/current/` until the quality campaign reaches an appropriate closure point.

Open decisions:

- None. The user approved behavior-preserving quality work and the next measured production hotspot is clear from local evidence.

## Pre-Implementation Gate - Slice 9

Status: ready.

Problem / root-cause model:

- Facts: after PR #12 merged to `main`, local `main` matches `origin/main` at `5571260ef5d`.
- Facts: `tools/archposture` reports `606` source files, `125864` source lines, `57` large files, and `43` large functions.
- Facts: the highest remaining production large function is `pkg/engine/entity_surgical.go:66` `(*Engine).refreshEntityArtifactsForFeedUpdates` at `267` lines with complexity `68`; `pkg/engine/entity_surgical.go` is `981` lines.
- Facts: baseline `pkg/engine` coverage is `71.2%`; `refreshEntityArtifactsForFeedUpdates` coverage is `60.4%`.
- Working theory: the complexity is orchestration coupling inside one function: publish batch setup, target filtering, feed delta loading, pending sidecar promotion/deletion, affected country/ASN tracking, unchanged detail touches, changed detail sidecar/public payload writes, index patching, version publication, generated-file timestamp application, and final sync. Extracting these phases into private helpers should reduce measured complexity without changing surgical entity refresh behavior.

Evidence reviewed:

- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice9-baseline.json`
- `jq '{source:.source, large_files:(.large_files|length), large_functions:(.large_functions|length), large_functions: (.large_functions[:15] | map({path,start_line,name,lines,complexity})), entity_surgical_large_functions:(.large_functions | map(select((.path // "") == "pkg/engine/entity_surgical.go"))) }' /tmp/update-ipsets-archposture-slice9-baseline.json`
- `go test -coverprofile=/tmp/update-ipsets-engine-slice9-baseline.cover -covermode=atomic ./pkg/engine`
- `go tool cover -func=/tmp/update-ipsets-engine-slice9-baseline.cover`
- `pkg/engine/entity_surgical.go`
- `pkg/engine/home_detail_test.go`
- `pkg/engine/entity_artifacts.go`
- `pkg/engine/entity_artifacts_write.go`
- `pkg/engine/entity_artifacts_health.go`
- `.agents/sow/specs/integrity.md`
- `.agents/sow/specs/pipeline.md`
- `.agents/sow/specs/files-layout.md`
- `.agents/sow/specs/operating-principles.md`

Affected contracts and surfaces:

- `pkg/engine`: surgical entity artifact refresh after selected feed updates, feed sidecar promotion, pending sidecar cleanup, private country/ASN sidecars, public country/ASN detail JSON and markdown, country/ASN index patching, generated-file timestamps, and entity artifact version marker.
- Private entity artifacts under `lib/entities/feeds`, `lib/entities/feeds-pending`, `lib/entities/countries`, `lib/entities/asns`, and `lib/entities/version`.
- Public entity artifacts under `web/countries`, `web/asns`, and indexes.
- No config schema, downloader behavior, public request-time generation, UI source, install behavior, scheduler queue admission, or artifact schema change is expected.
- SOW only; specs/docs are not expected to change because behavior is preserved.

Existing patterns to reuse:

- Slice 8 entity artifact writer split: keep top-level orchestration small and move phase-specific helpers into focused private types/files.
- Existing `TestRefreshEntityArtifactsForFeedUpdatesSurgicallyPatchesAggregates` drives a real temporary engine fixture, pending sidecars, public/private artifacts, index updates, and pending sidecar cleanup.
- Existing mtime helpers and public/private path helpers from `entity_artifacts_helpers.go` and `entity_artifacts.go`.
- Existing batch APIs: `webPublishBatch`, `entityPublishBatch`, `markDelete`, `writeObservedJSONFileAt`, generated-file timestamp application, and `syncGeneratedFiles`.
- Existing background task update wording and progress phase names.

Risk and blast radius:

- Surgical refresh is a production incremental-repair path. A behavior change can silently leave stale country/ASN pages, stale indexes, stale private sidecars, or repeated integrity repair loops.
- Full rebuild fallback must remain intact for legacy/missing/unreadable sidecars, underflow, and inconsistent detail state.
- Unchanged detail paths must preserve the touch behavior for private and public artifacts only when both files exist.
- Generated-file ledger entries and mtimes must continue to match published public artifacts.
- Public requests must remain cache-first readers; no request-time artifact generation may be introduced.

Sensitive data handling plan:

- This slice uses synthetic test feeds, country codes, ASNs, paths, and structural metrics only.
- No secrets, tokens, cookies, private endpoints, customer data, raw external IP lists, or personal data are needed.
- Durable artifacts will record only package paths, metrics, test outcomes, and sanitized behavior evidence.

Implementation plan:

1. Introduce a package-private surgical refresh state/helper that carries context, batches, task, target feeds, deltas, affected country/ASN sets, feed mtimes, health classifier, generated files, and update maps.
2. Extract target-feed filtering and feed delta loading/promotion into focused helpers while preserving full-rebuild fallback behavior.
3. Extract no-delta version publication into a focused helper.
4. Extract country and ASN detail patch loops into focused helpers, preserving unchanged touches, changed writes, delete markers, materialization metrics, markdown generation, progress updates, and generated-file ledger entries.
5. Extract index patching and final publish/sync into focused helpers.
6. Add or adjust behavior tests only if inspection finds a meaningful uncovered branch affected by the extraction.
7. Re-run package coverage, architecture posture, strict engine tests, and repository quality gates.

Validation plan:

- `go test ./pkg/engine`
- `go test -coverprofile=/tmp/update-ipsets-engine-slice9.cover -covermode=atomic ./pkg/engine`
- `go tool cover -func=/tmp/update-ipsets-engine-slice9.cover`
- `go test -shuffle=on -count=3 ./pkg/engine`
- `go run ./tools/archposture -root .`
- `make lint`
- `make staticcheck`
- `make golangci-lint`
- `CI=true make coverage`
- `make test-strict`
- `git diff --check -- ...`

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if this slice finds a durable surgical-refresh refactor/testing lesson.
- Specs: no update expected; behavior is preserved.
- End-user/operator docs: no update expected; entity artifact semantics are unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW in `.agents/sow/current/` until the quality campaign reaches an appropriate closure point.

Open decisions:

- None. The user approved behavior-preserving quality work and the next measured production hotspot is clear from local evidence.

## Pre-Implementation Gate - Slice 8

Status: ready.

Problem / root-cause model:

- Facts: after PR #11 merged to `main`, local `main` matches `origin/main` at `6d8a4f09f69e`.
- Facts: `tools/archposture` reports `602` source files, `125547` source lines, `58` large files, and `45` large functions.
- Facts: the highest remaining production large function is `pkg/engine/entity_artifacts.go:395` `(*Engine).writeEntityArtifacts` at `290` lines with complexity `78`; `pkg/engine/entity_artifacts.go` is `799` lines.
- Facts: baseline `pkg/engine` coverage is `70.7%`; `writeEntityArtifacts` coverage is `70.4%`.
- Working theory: the complexity is mostly orchestration coupling: target-feed selection, provider reference loading, feed sidecar staging, stale sidecar deletion, changed-feed tracking, affected country/ASN discovery, detail sidecar generation, public detail materialization, index generation, sitemap/home aggregate staging, and version marker writing. Extracting these phases into private helpers should reduce complexity without changing entity artifact behavior.

Evidence reviewed:

- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice8-baseline.json`
- `jq '{source: .source, large_files_count: (.large_files|length), large_functions_count: (.large_functions|length), large_functions: .large_functions[:20], large_files: .large_files[:20]}' /tmp/update-ipsets-archposture-slice8-baseline.json`
- `go test -coverprofile=/tmp/update-ipsets-engine-slice8-baseline.cover -covermode=atomic ./pkg/engine`
- `go tool cover -func=/tmp/update-ipsets-engine-slice8-baseline.cover`
- `pkg/engine/entity_artifacts.go`
- `pkg/engine/entity_integrity_test.go`
- `pkg/engine/entity_integrity_global_test.go`
- `pkg/engine/home_detail_test.go`
- `pkg/engine/pipeline_integrity_scenario_test.go`
- `.agents/sow/specs/integrity.md`
- `.agents/sow/specs/pipeline.md`
- `.agents/sow/specs/files-layout.md`
- `.agents/sow/specs/operating-principles.md`

Affected contracts and surfaces:

- `pkg/engine`: entity feed sidecars, pending sidecar cleanup, private country/ASN sidecars, public country/ASN JSON and markdown, country/ASN indexes, public sitemap files, homepage aggregate refresh, generated-file ledger entries, and entity artifact version marker.
- Private entity artifacts under `lib/entities/feeds`, `lib/entities/feeds-pending`, `lib/entities/countries`, `lib/entities/asns`, and `lib/entities/version`.
- Public entity artifacts under `web/countries`, `web/asns`, sitemap files, and home aggregate artifacts.
- No config schema, downloader behavior, public request-time generation, UI source, install behavior, or scheduler queue semantics are expected to change.
- SOW only; specs/docs are not expected to change because behavior is preserved.

Existing patterns to reuse:

- Recent comparison and entity-integrity slices: keep the public orchestration function small and move phase-specific private helpers into focused files.
- Existing engine tests drive `RebuildEntityArtifacts`, `RefreshEntityArtifactsForFeedUpdates`, repair paths, and pipeline integrity through real temporary artifacts.
- Existing mtime helpers: `entityFeedSidecarReferenceMTime`, `countryDetailLogicalMTime`, `asnDetailLogicalMTime`, and `feedProcessingTimestamp`.
- Existing batch APIs: `stagedPublishBatch.markDelete`, `writeJSONFileAt`, `writeJSONFile`, and generated-file timestamp application.
- Existing background task update wording and progress phase names.

Risk and blast radius:

- Entity artifacts are a production public-serving and integrity surface. False deletion, stale mtimes, or missing generated-file ledger entries can surface as missing country/ASN pages, stale homepage aggregates, or integrity repair loops.
- Full rebuild behavior must delete stale private sidecars and pending sidecars; incremental refresh behavior must preserve unchanged sidecars and touch unchanged feed sidecars to the provider/feed reference mtime.
- Public requests must remain cache-first readers; no request-time artifact generation may be introduced.
- Background task progress must remain bounded and visible.
- The slice should avoid changing output schema, sorted output order, logical timestamp selection, redistributability flags, or repair semantics.

Sensitive data handling plan:

- This slice uses local synthetic test feeds, country codes, ASNs, paths, and structural metrics only.
- No secrets, tokens, cookies, private endpoints, customer data, raw external IP lists, or personal data are needed.
- Durable artifacts will record only package paths, metrics, test outcomes, and sanitized behavior evidence.

Implementation plan:

1. Introduce a package-private entity artifact generation context/helper that carries target feeds, provider references, live/new sidecars, feed mtimes, changed feeds, affected countries/ASNs, batches, generated files, and task handle.
2. Extract feed sidecar staging, full-rebuild stale deletion, changed-feed tracking, and all-sidecar merge behavior into focused helpers.
3. Extract affected country/ASN discovery and full-rebuild stale detail deletion into focused helpers.
4. Extract selected country/ASN detail writing into focused helpers while preserving logical mtimes, public JSON/markdown generation, generated-file ledger entries, progress updates, and delete markers.
5. Extract index/sitemap/home/version staging into focused helpers.
6. Add or adjust behavior tests only if inspection finds a meaningful uncovered branch affected by the extraction.
7. Re-run package coverage, architecture posture, strict engine tests, and repository quality gates.

Validation plan:

- `go test ./pkg/engine`
- `go test -coverprofile=/tmp/update-ipsets-engine-slice8.cover -covermode=atomic ./pkg/engine`
- `go tool cover -func=/tmp/update-ipsets-engine-slice8.cover`
- `go test -shuffle=on -count=3 ./pkg/engine`
- `go run ./tools/archposture -root .`
- `make lint`
- `make staticcheck`
- `make golangci-lint`
- `CI=true make coverage`
- `make test-strict`
- `git diff --check -- ...`

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if this slice finds a durable entity-artifact refactor/testing lesson.
- Specs: no update expected; behavior is preserved.
- End-user/operator docs: no update expected; entity artifact semantics are unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW in `.agents/sow/current/` until the quality campaign reaches an appropriate closure point.

Open decisions:

- None. The user approved behavior-preserving quality work and the next measured production hotspot is clear from local evidence.

## Pre-Implementation Gate - Slice 7

Status: ready.

Problem / root-cause model:

- Facts: after PR #10 merged to `main`, local `main` matches `origin/main` at `8393bc24bc04`.
- Facts: `tools/archposture` reports `600` source files, `125455` source lines, `58` large files, and `46` large functions.
- Facts: the highest remaining production large function is `pkg/engine/output.go:336` `(*Engine).writeComparisonFiles` at `295` lines with complexity `55`; `pkg/engine/output.go` is `1311` lines.
- Facts: baseline `pkg/engine` coverage is `70.7%`; `writeComparisonFiles` coverage is `80.3%`.
- Working theory: the complexity is mostly phase coupling inside one function: public-set preparation, update filtering, worker fan-out, skip accounting, relatedness construction, row grouping, merge/write accounting, and artifact sanitization. Extracting these phases into private helpers should reduce measured complexity without changing comparison output behavior.

Evidence reviewed:

- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice7-baseline.json`
- `jq '{source: .source, large_files_count: (.large_files|length), large_functions_count: (.large_functions|length), large_functions: .large_functions[:15], large_files: .large_files[:15]}' /tmp/update-ipsets-archposture-slice7-baseline.json`
- `go test -coverprofile=/tmp/update-ipsets-engine-slice7-baseline.cover -covermode=atomic ./pkg/engine`
- `go tool cover -func=/tmp/update-ipsets-engine-slice7-baseline.cover`
- `pkg/engine/output.go`
- `pkg/engine/output_test.go`
- `pkg/engine/output_cancellation_test.go`
- `pkg/engine/fileset_helpers_test.go`
- `.agents/sow/specs/integrity.md`
- `.agents/sow/specs/pipeline.md`
- `.agents/sow/specs/operating-principles.md`

Affected contracts and surfaces:

- `pkg/engine`: pairwise public feed comparison artifacts, comparison skip accounting, related-row classification, incremental comparison-row merge behavior, zero-overlap row removal, and generated artifact mtimes.
- Public artifacts named `<feed>_comparison.json`.
- Downstream unique-share, insights, integrity, public API/UI comparison consumers that read comparison artifacts.
- No config schema, downloader behavior, scheduler queue behavior, public request-time generation, UI source, install behavior, or generated frontend assets are expected to change.
- SOW only; specs/docs are not expected to change because behavior is preserved.

Existing patterns to reuse:

- Existing `CompareRow` contract and `mergeCompareRows` zero-overlap policy.
- Existing `leafAncestors` and `positiveLineageParents` relatedness model for signed merges.
- Existing `latestSetCache` sharing for heavy engine phases.
- Existing comparison tests for subtractive merge relatedness, prefix overlap, stale zero-overlap cleanup, file-set generation, and cancellation.
- Private helper extraction in the engine package, matching recent entity-integrity slice structure without creating a new package.

Risk and blast radius:

- Comparison artifacts are public product data. A behavior change can affect feed pages, unique-share estimates, insights, and integrity results.
- Incremental runs must preserve untouched comparison rows and delete stale zero-overlap rows only for recomputed counterparts.
- Cancellation must stop publication before partial comparison files are written.
- Heavy-worker fan-out must keep bounded worker counts and avoid opening more set files than the current cache model permits.
- Generated file mtimes must continue using `feedProcessingTimestamp(name)`.

Sensitive data handling plan:

- This slice uses local synthetic test feeds and local structural metrics only.
- No secrets, tokens, cookies, private endpoints, customer data, raw external IP lists, or personal data are needed.
- Durable artifacts will record only package paths, metrics, test outcomes, and sanitized behavior evidence.

Implementation plan:

1. Extract comparison set preparation into a focused helper that returns per-feed metadata and prefix/range pruning state.
2. Extract updated-feed filtering, pair generation, worker execution, skip counters, and operation metric emission into focused helpers.
3. Extract relatedness caching and fresh-row grouping into focused helpers.
4. Extract comparison-row merge/write accounting into a focused helper while preserving existing read-from-live then write-to-stage behavior and logical mtimes.
5. Keep all helper types private to `pkg/engine`; avoid new abstractions outside the selected production path.
6. Add or adjust behavior tests only if inspection finds a meaningful uncovered contract branch affected by the extraction.
7. Re-run package coverage, architecture posture, strict engine tests, and repository quality gates.

Validation plan:

- `go test ./pkg/engine`
- `go test -coverprofile=/tmp/update-ipsets-engine-slice7.cover -covermode=atomic ./pkg/engine`
- `go tool cover -func=/tmp/update-ipsets-engine-slice7.cover`
- `go test -shuffle=on -count=3 ./pkg/engine`
- `go run ./tools/archposture -root .`
- `make lint`
- `make staticcheck`
- `make golangci-lint`
- `CI=true make coverage`
- `git diff --check -- ...`

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if this slice finds a durable comparison-artifact refactor/testing lesson.
- Specs: no update expected; behavior is preserved.
- End-user/operator docs: no update expected; comparison semantics are unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW in `.agents/sow/current/` until the quality campaign reaches an appropriate closure point.

Open decisions:

- None. The user approved behavior-preserving quality work and the next measured production hotspot is clear from local evidence.

## Pre-Implementation Gate - Slice 6

Status: ready.

Problem / root-cause model:

- Facts: after PR #9 merged to `main`, root Go coverage is `72.2%`.
- Facts: `tools/archposture` reports `pkg/engine/entity_integrity.go` at `1028` lines and `(*Engine).CheckEntityArtifactsIntegrity` at `449` lines with complexity `61`.
- Facts: baseline `pkg/engine` coverage is `70.5%`; `CheckEntityArtifactsIntegrity` coverage is `62.8%`.
- Facts: entity-artifact integrity is a production safety surface for public country/ASN artifacts, private entity sidecars, indexes, homepage aggregates, health-sensitive payloads, and startup/operator repair planning.
- Working theory: the high complexity is mostly orchestration coupling, not algorithmic necessity. Splitting global prerequisite checks, feed sidecar scanning, country/ASN detail scanning, index checks, and health drift checks into focused helpers should reduce complexity without changing behavior.

Evidence reviewed:

- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice6-baseline.json`
- `go test -coverprofile=/tmp/update-ipsets-engine-baseline.cover -covermode=atomic ./pkg/engine`
- `go tool cover -func=/tmp/update-ipsets-engine-baseline.cover`
- `pkg/engine/entity_integrity.go`
- `pkg/engine/entity_integrity_test.go`
- `.agents/sow/specs/integrity.md`
- `.agents/sow/specs/pipeline.md`
- `.agents/sow/specs/files-layout.md`
- `.agents/sow/specs/operating-principles.md`

Affected contracts and surfaces:

- `pkg/engine`: entity-artifact integrity findings, repair plans, and startup/operator repair behavior.
- Entity private sidecars under `lib/entities/feeds`, `lib/entities/countries`, and `lib/entities/asns`.
- Public entity artifacts under `web/countries`, `web/asns`, and indexes.
- Homepage aggregate integrity is called from this path but owned by `home_aggregate_integrity.go`; no behavior change is intended.
- No public serving, admin API/UI, config schema, install, downloader, scheduler, or generated artifact format change is intended.

Existing patterns to reuse:

- Use package-local engine fixtures from `pkg/engine/*_test.go`; do not construct raw `Engine` literals outside established helpers.
- Existing entity-integrity tests assert observable finding kinds, repair actions, affected entity IDs, and repair plan contents.
- Keep same-package tests only because the engine repair plan type is intentionally unexported operator-internal state.
- Keep integrity findings sorted by scope, subject, and kind.

Risk and blast radius:

- Entity integrity is high-risk: false positives can trigger unnecessary background repair; false negatives can leave stale public artifacts after production rollout.
- Mtime semantics are delicate; refactor must not change reference timestamp selection or unchanged-actor mtime behavior.
- Startup availability matters; refactor must preserve broad repair deferral and avoid adding startup recomputation.
- File size will remain a broader follow-up because this slice targets the highest-complexity function first.

Sensitive data handling plan:

- This slice uses synthetic test feeds, ASNs, country codes, paths, and timestamps only.
- No secrets, tokens, cookies, private endpoints, customer data, raw external IP lists, or personal data are needed.
- Durable artifacts will record package paths, metrics, test outcomes, and sanitized behavior evidence only.

Implementation plan:

1. Add focused behavior tests for currently under-covered entity-integrity branches before refactoring: missing/mismatched version marker and missing ASN public detail.
2. Introduce a package-private scanner/state helper for `CheckEntityArtifactsIntegrity` orchestration.
3. Extract global prerequisite checks, provider references, feed sidecar scan, country detail scan, ASN detail scan, index checks, and health drift scan into focused helpers.
4. Preserve finding payloads, repair actions, plan mutation, and final sort order.
5. Re-run package coverage, architecture posture, strict engine tests, and repository quality gates.

Validation plan:

- `go test ./pkg/engine`
- `go test -coverprofile=/tmp/update-ipsets-engine-slice6.cover -covermode=atomic ./pkg/engine`
- `go tool cover -func=/tmp/update-ipsets-engine-slice6.cover`
- `go test -shuffle=on -count=3 ./pkg/engine`
- `go run ./tools/archposture -root .`
- `make lint`
- `make staticcheck`
- `make golangci-lint`
- `CI=true make coverage`
- `git diff --check -- ...`

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if this slice finds a durable entity-integrity refactor/testing lesson.
- Specs: no update expected; behavior is preserved.
- End-user/operator docs: no update expected; operator-facing behavior is unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW in `.agents/sow/current/` until the quality campaign reaches an appropriate closure point.

Open decisions:

- None. The user selected the engine complexity slice and approved behavior-preserving quality work.

## Pre-Implementation Gate - Slice 5

Status: ready.

Problem / root-cause model:

- Facts: after the fourth slice merged to `main`, root Go coverage is `72.0%`; `pkg/feedhealth` remains at `63.9%` statement coverage.
- Facts: `pkg/feedhealth` is operator-facing classification logic for unavailable, archived, empty, delayed, risky, unmaintained, and healthy states.
- Working theory: behavior-focused `Classify` tests can cover meaningful policy branches without production changes or broad engine fixtures.

Evidence reviewed:

- `go test -coverprofile=/tmp/update-ipsets-feedhealth-baseline.cover -covermode=atomic ./pkg/feedhealth`
- `go tool cover -func=/tmp/update-ipsets-feedhealth-baseline.cover`
- `pkg/feedhealth/feedhealth.go`
- `pkg/feedhealth/feedhealth_test.go`

Affected contracts and surfaces:

- `pkg/feedhealth`: runtime policy extraction, category threshold lookup, classification branches, configured cadence fallback, single-observation grace, delayed/risky/unmaintained age classes, empty/unavailable precedence, and source/category fallback behavior.
- No daemon, scheduler, public serving, UI, install, config schema, or generated artifact contracts are expected to change.
- SOW only; specs/docs are not expected to change because public behavior is unchanged.

Existing patterns to reuse:

- External-package tests already validate operator-visible `Classify` outputs.
- Small synthetic `cache.Entry`, `config.Source`, `config.RuntimeConfig`, and fixed `time.Time` fixtures are enough.
- Assertions should focus on snapshot class and fields operators/API callers observe.

Risk and blast radius:

- No production behavior change is expected in this slice.
- Tests must avoid coupling to unexported helper names.
- Feed health semantics are user-facing, so expected class precedence must be explicit in tests.

Sensitive data handling plan:

- This slice uses synthetic feed names, categories, and timestamps only.
- No secrets, tokens, cookies, private endpoints, customer data, or personal data are needed.
- Durable artifacts will record only package names, coverage values, and validation outcomes.

Implementation plan:

1. Add public `PolicyFromRuntime` and threshold fallback tests.
2. Add `Classify` table coverage for delayed, risky, unmaintained, healthy, empty, unavailable, and single-observation grace behavior.
3. Add cadence/category fallback tests using observable snapshot fields.
4. Re-run package coverage for `pkg/feedhealth`, then root coverage and relevant lint/static checks.

Validation plan:

- `go test ./pkg/feedhealth`
- `go test -coverprofile=/tmp/update-ipsets-feedhealth.cover -covermode=atomic ./pkg/feedhealth`
- `go tool cover -func=/tmp/update-ipsets-feedhealth.cover`
- `make lint`
- `make staticcheck`
- `make golangci-lint`
- `CI=true make coverage`
- `go run ./tools/archposture -root .`
- `git diff --check -- ...`

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if this slice finds a durable testing/hygiene lesson.
- Specs: no update expected; behavior is preserved.
- End-user/operator docs: no update expected; feed-health semantics are unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW in `.agents/sow/current/` until the quality campaign reaches an appropriate closure point.

Open decisions:

- None. The user approved the pragmatic quality target model and behavior-preserving quality work.

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
- Continued from first-slice `main` commit `164c6b1ec1fb` for a low-risk coverage slice before engine refactoring.
- Added command behavior tests for dispatch, usage/version output, validation-only subcommand failures, set-expression parsing, name-list parsing, and logger level selection.
- Added systemd behavior tests for readiness/status/stopping/watchdog payloads, no-op notification behavior, invalid socket errors, and watchdog interval parsing.
- Added observability behavior tests for environment opt-in, metric reader intervals, shutdown aggregation, tracing helper defaults, designed metric recording, bounded API recalculation labels, and slog tee fan-out.
- Split the new observability test coverage into `internal/observability/observability_helpers_test.go` so the slice does not add a new large test file.
- Repaired local ownership of generated/dependency directories needed by the coverage gate (`ui/node_modules`, `ui/dist`, and `pkg/web/static`) after a prior root-owned local install left them unwritable; no tracked generated assets were manually edited.
- Refactored `pkg/kernel` Linux ipset orchestration behind an unexported operation interface backed by netlink.
- Added fake-backed `pkg/kernel` tests for loaded-set mapping, not-loaded no-op behavior, successful replacement, fallback type selection, invalid entry handling, create/add/swap errors, cleanup, parse errors, and helper edge cases.
- Pulled merged PR #7; local `main` now matches `origin/main` at `a40467c3eceb`.
- Added `pkg/asnloc` coverage for text-backed `Open`, nil and wrapper behavior, range-walking counts, bogon splitting, lookup-error propagation, gzip/plain text loader paths, range-table overlap/nil behavior, and IPv4 helper edge cases.
- Pulled merged PR #8; local `main` now matches `origin/main` at `4b46fa04a620`.
- Added `pkg/feedhealth` coverage for runtime policy extraction, category fallback, delayed/risky/unmaintained age classes, single-observation grace, entry/source cadence fallback, source category/date fallback, unavailable-over-empty precedence, nil entries, and never-published entries.
- Pulled merged PR #9; local `main` now matches `origin/main` at `6db9850ea70d`.
- Added pre-refactor entity-integrity coverage for missing version marker, mismatched version marker, missing ASN public JSON, and malformed ASN private sidecar behavior.
- Refactored `CheckEntityArtifactsIntegrity` into focused entity-integrity scanner helpers:
  - public orchestration and repair-plan types remain in `pkg/engine/entity_integrity.go`.
  - dependency/reference timestamp helpers moved to `pkg/engine/entity_integrity_refs.go`.
  - scanner orchestration and global prerequisites moved to `pkg/engine/entity_integrity_scanner.go`.
  - feed-sidecar scanning moved to `pkg/engine/entity_integrity_feed_scan.go`.
  - country/ASN/index/health-drift scanning moved to `pkg/engine/entity_integrity_detail_scan.go`.
- Pulled merged PR #10; local `main` now matches `origin/main` at `8393bc24bc04`.
- Refactored the comparison artifact writer:
  - `writeComparisonFiles` now orchestrates comparison preparation, pair execution, row grouping, merge/write, and sanitization through focused helpers.
  - comparison pair fan-out, skip accounting, update filtering, and operation metrics live in `pkg/engine/output_comparison.go`.
  - range/prefix pruning, positive-lineage relatedness helpers, zero-overlap row merge policy, and comparison-artifact sanitization live in `pkg/engine/output_comparison_helpers.go`.
  - `pkg/engine/output.go` now keeps metadata and sitemap helpers instead of also owning comparison artifact generation.

### 2026-06-04

- Pulled merged PR #11; local `main`, `origin/main`, and `quality-engine-entity-artifacts-complexity` all point at `6d8a4f09f69e`.
- Refactored entity artifact generation:
  - `writeEntityArtifacts` now orchestrates entity artifact generation through focused helper phases.
  - entity artifact write state and feed/detail/index/sitemap/home/version staging helpers live in `pkg/engine/entity_artifacts_write.go`.
  - shared entity artifact set, mtime, and sitemap helpers live in `pkg/engine/entity_artifacts_helpers.go`.
  - health-transition entity-detail refresh helpers live in `pkg/engine/entity_artifacts_health.go`.
  - `pkg/engine/entity_artifacts.go` now keeps path helpers and top-level rebuild/refresh orchestration instead of also owning detailed artifact generation.
- Added `pkg/engine/entity_artifacts_health_test.go` to prove health-transition refresh rewrites corrupted published country/ASN detail artifacts from private sidecars and preserves sidecar mtimes on publication.
- Refactored surgical entity refresh:
  - `RefreshEntityArtifactsForFeedUpdates` remains in `pkg/engine/entity_surgical.go` as the public entrypoint.
  - surgical refresh orchestration, progress, and publication phases live in `pkg/engine/entity_surgical_refresh.go`.
  - feed delta loading and target filtering helpers live in `pkg/engine/entity_surgical_delta.go`.
  - country/ASN detail patch helpers live in `pkg/engine/entity_surgical_detail.go`.
  - country/ASN index patch helpers live in `pkg/engine/entity_surgical_index.go`.
  - observed artifact I/O, touch, and empty sidecar helpers live in `pkg/engine/entity_surgical_io.go`.
- Pulled merged PR #13; local `main`, `origin/main`, and `quality-downloader-fetch-complexity` started this slice at `8150a4c007cd`.
- Refactored downloader fetch:
  - `Fetch` remains the public observability wrapper.
  - downloader dispatch, local URL handling, HTTP request construction, response classification, body streaming, max-size enforcement, same-body detection, and modified-time extraction live in `pkg/downloader/fetch_http.go`.
  - `pkg/downloader/downloader.go` now keeps types, client construction, redirect policy, hashing/file helpers, local/internal fetch helpers, curl-like option parsing, and temp-file creation.
- Pulled merged PR #14; local `main`, `origin/main`, and `quality-entity-detail-builders-complexity` started this slice at `acb3ed0d41ab`.
- Refactored live country/ASN entity detail builders:
  - `buildCountryDetailSidecar` and `buildASNDetailSidecar` now orchestrate provider setup, matching-feed collection, provider enrichment, and builder output through focused helpers.
  - live detail provider setup, matching-feed helpers, empty-sidecar builders, country-to-ASN enrichment, and ASN-to-country enrichment live in `pkg/engine/home_entity_detail_live.go`.
  - detail payload materialization, sidecar JSON loading, JSON writing, timestamped writes, and sorted JSON file listing live in `pkg/engine/home_entity_detail_payload.go`.
  - `pkg/engine/home_entity_builders.go` now owns output views, health classification, indexes, and short live detail orchestration instead of also owning the detail payload/I/O helpers.
- Added `pkg/engine/home_entity_detail_live_test.go` to validate invalid entity identifiers and empty country/ASN detail payloads through exported detail APIs.
- Pulled merged PR #15; local `main`, `origin/main`, and `quality-admin-manifest-complexity` started this slice at `a25d39b68b55`.
- Refactored admin feed manifest generation:
  - `buildFeedManifest` remains the top-level manifest assembly function.
  - processed-date lookup, row creation, base/provider/canonical file enumeration, web secondary artifacts, provider fan-out artifacts, binary/latest files, optional history snapshots, and summary rollup live in `pkg/web/admin_manifest_builder.go`.
  - `pkg/web/admin_manifest.go` now keeps the HTTP handler, manifest DTOs, file stat/stale policy, and path display helpers.
- Added database provider manifest tests for ASN and geolocation provider-source rows and database-feed exclusion of non-database artifacts.

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
- Second-slice local results:
  - Slice baseline from archived `HEAD` at `164c6b1ec1fb`: `cmd/update-ipsets` coverage `18.4%`, `internal/observability` coverage `30.3%`, and `pkg/systemd` coverage `51.5%`.
  - Package coverage after this slice: `cmd/update-ipsets` `54.1%`, `internal/observability` `64.1%`, and `pkg/systemd` `93.9%`.
  - Root coverage after this slice is `71.5%`, up from `70.8%` after the first slice.
  - `tools/archposture` after this slice: source files `595`, source lines `124256`, large files `59`, and large functions `47`.
  - No changed slice files or functions are listed in `tools/archposture` large-file/large-function output.
- Third-slice local results:
  - `pkg/kernel` package coverage moved from `32.8%` to `88.2%`.
  - `applyIfLoaded` function coverage is `96.4%`; real netlink wrapper methods remain intentionally untested locally because they require host kernel state.
  - Root coverage after this slice is `71.7%`, up from `71.5%` after the second slice.
  - `tools/archposture` after this slice: source files `595`, source lines `124561`, large files `59`, and large functions `47`.
  - No changed kernel files or functions are listed in `tools/archposture` large-file/large-function output.
- Fourth-slice local results:
  - `pkg/asnloc` package coverage moved from `44.9%` to `76.2%`.
  - Public ASN wrapper/counting functions now report: `Close` `100.0%`, `Lookup` `100.0%`, `Stats` `100.0%`, `CountFeed` `100.0%`, `CountFeedWithBogons` `100.0%`, and `countFeedRanges` `95.5%`.
  - Text loader functions now report: `loadIPToASNTSV` `86.4%` and `loadCAIDAPrefix2AS` `86.4%`.
  - Root coverage after this slice is `72.0%`, up from `71.7%` after the third slice.
  - `tools/archposture` after this slice: source files `595`, source lines `124907`, large files `59`, and large functions `47`.
  - No changed ASN files or functions are listed in `tools/archposture` large-file/large-function output.
- Fifth-slice local results:
  - `pkg/feedhealth` package coverage moved from `63.9%` to `88.9%`.
  - Public feed-health functions now report: `PolicyFromRuntime` `100.0%`, `ThresholdsForCategory` `100.0%`, and `Classify` `100.0%`.
  - Policy branch helpers now report: `classifyAge` `100.0%`, `configuredFrequency` `100.0%`, `graceActive` `100.0%`, and `archivalThresholdReached` `100.0%`.
  - Root coverage after this slice is `72.2%`, up from `72.0%` after the fourth slice.
  - `tools/archposture` after this slice: source files `595`, source lines `125145`, large files `59`, and large functions `47`.
  - No changed feed-health files or functions are listed in `tools/archposture` large-file/large-function output.
- Sixth-slice local results:
  - `pkg/engine/entity_integrity.go` moved from `1028` lines to `191` lines.
  - The former `(*Engine).CheckEntityArtifactsIntegrity` target moved from `449` lines / complexity `61` to a small scanner wrapper and no longer appears in `tools/archposture` large-function output.
  - New split files are below the project file-size threshold: `entity_integrity_refs.go` `403` lines, `entity_integrity_scanner.go` `151` lines, `entity_integrity_feed_scan.go` `138` lines, `entity_integrity_detail_scan.go` `320` lines, and `entity_integrity_global_test.go` `135` lines.
  - `pkg/engine` coverage moved from `70.5%` to `70.7%`.
  - Root coverage after this slice remains `72.2%`.
  - `tools/archposture` after this slice: source files `600`, source lines `125455`, large files `58`, and large functions `46`.
  - No `pkg/engine/entity_integrity*` files or functions are listed in `tools/archposture` large-file/large-function output.
- Seventh-slice local results:
  - `pkg/engine/output.go` moved from `1311` lines to `750` lines.
  - The former `(*Engine).writeComparisonFiles` target moved from `295` lines / complexity `55` to a small orchestration function and no longer appears in `tools/archposture` large-function output.
  - New split files are below the project file-size threshold: `output_comparison.go` `383` lines and `output_comparison_helpers.go` `267` lines.
  - `pkg/engine` coverage remained `70.7%`; this slice was behavior-preserving complexity reduction rather than coverage expansion.
  - Root coverage after this slice is `72.3%`.
  - `tools/archposture` after this slice: source files `602`, source lines `125547`, large files `58`, and large functions `45`.
  - No `pkg/engine/output_comparison*` files or comparison-writer functions are listed in `tools/archposture` large-file/large-function output.
- Eighth-slice local results:
  - `pkg/engine/entity_artifacts.go` moved from `799` lines to `259` lines.
  - The former `(*Engine).writeEntityArtifacts` target moved from `290` lines / complexity `78` to a small orchestration function and no longer appears in `tools/archposture` large-function output.
  - The remaining large health-transition refresh function was split into focused helpers, leaving no `entity_artifacts*` large files or large functions in `tools/archposture`.
  - New split files are below the project file-size threshold: `entity_artifacts_write.go` `474` lines, `entity_artifacts_health.go` `172` lines, `entity_artifacts_helpers.go` `127` lines, and `entity_artifacts_health_test.go` `84` lines.
  - `home_detail_test.go` remains at its baseline `761` lines; the added health-transition test was moved to a focused file to avoid growing an existing large test file.
  - `pkg/engine` coverage moved from `70.7%` to `71.2%`.
  - Root coverage after this slice is `72.5%`.
  - `tools/archposture` after this slice: source files `606`, source lines `125864`, large files `57`, and large functions `43`.
- Ninth-slice local results:
  - `pkg/engine/entity_surgical.go` moved from `981` lines to `38` lines.
  - The former `(*Engine).refreshEntityArtifactsForFeedUpdates` target moved from `267` lines / complexity `68` to a small orchestration function and no longer appears in `tools/archposture` large-function output.
  - New split files are below the project file-size threshold: `entity_surgical_refresh.go` `387` lines, `entity_surgical_detail.go` `278` lines, `entity_surgical_delta.go` `154` lines, `entity_surgical_index.go` `135` lines, and `entity_surgical_io.go` `138` lines.
  - `pkg/engine` coverage remained `71.2%`; this slice was behavior-preserving complexity reduction rather than coverage expansion.
  - Root coverage after this slice remains `72.5%`.
  - `tools/archposture` after this slice: source files `611`, source lines `126013`, large files `56`, and large functions `42`.
  - No `pkg/engine/entity_surgical*` files or functions are listed in `tools/archposture` large-file/large-function output.
- Tenth-slice local results:
  - `pkg/downloader/downloader.go` moved from `606` lines to `370` lines.
  - The former `(*Client).Fetch` target moved from `231` lines / complexity `48` to a public wrapper plus focused private helpers and no longer appears in `tools/archposture` large-function output.
  - New `pkg/downloader/fetch_http.go` is below the project file-size threshold at `295` lines.
  - `pkg/downloader` coverage moved from `66.8%` to `68.4%`; the public `Fetch` wrapper reports `100.0%`, HTTP response handling reports `100.0%`, and streaming body handling reports `89.7%`.
  - Root coverage after this slice remains `72.5%`.
  - `tools/archposture` after this slice: source files `612`, source lines `126072`, large files `55`, and large functions `41`.
  - No `pkg/downloader/downloader.go` or `pkg/downloader/fetch_http.go` files or functions are listed in `tools/archposture` large-file/large-function output.
- Eleventh-slice local results:
  - `pkg/engine/home_entity_builders.go` moved from `1030` lines to `457` lines.
  - The former `(*Engine).buildASNDetailSidecar` target moved from `231` lines / complexity `44` to a short orchestration function and no longer appears in `tools/archposture` large-function output.
  - The former `(*Engine).buildCountryDetailSidecar` target moved from `213` lines / complexity `42` to a short orchestration function and no longer appears in `tools/archposture` large-function output.
  - New split files are below the project file-size threshold: `home_entity_detail_live.go` `217` lines, `home_entity_detail_payload.go` `166` lines, and `home_entity_detail_live_test.go` `55` lines.
  - `pkg/engine` coverage remains `71.2%`; this slice was behavior-preserving complexity and file-size reduction with targeted coverage to avoid a net coverage regression.
  - Root coverage after this slice remains `72.5%`.
  - `tools/archposture` after this slice: source files `615`, source lines `125937`, large files `54`, and large functions `39`.
  - No `pkg/engine/home_entity_builders.go`, `pkg/engine/home_entity_detail_live.go`, `pkg/engine/home_entity_detail_payload.go`, or `pkg/engine/home_entity_precompute.go` files or functions are listed in `tools/archposture` large-file/large-function output.
- Twelfth-slice local results:
  - `pkg/web/admin_manifest.go` moved from `403` lines to `188` lines.
  - The former `buildFeedManifest` target moved from `217` lines / complexity `29` to a one-line orchestration function and no longer appears in `tools/archposture` large-function output.
  - New `pkg/web/admin_manifest_builder.go` is below the project file-size threshold at `179` lines.
  - `pkg/web` coverage moved from `78.3%` to `78.4%` by `go tool cover`; `buildFeedManifest` now reports `100.0%`, and the new provider-source path helper reports `100.0%`.
  - Root coverage after this slice remains `72.5%`.
  - `tools/archposture` after this slice: source files `616`, source lines `125955`, large files `54`, and large functions `38`.
  - No `pkg/web/admin_manifest.go` or `pkg/web/admin_manifest_builder.go` files or functions are listed in `tools/archposture` large-file/large-function output.
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
- `go test ./cmd/update-ipsets ./pkg/systemd ./internal/observability`: passed.
- `go test -coverprofile=/tmp/update-ipsets-slice2.cover -covermode=atomic ./cmd/update-ipsets ./pkg/systemd ./internal/observability`: passed.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice2-final.json`: passed.
- `CI=true make coverage`: passed.
- `go tool cover -func=coverage.out`: passed; total statement coverage `71.5%`.
- `git diff --check -- ...`: passed for changed files.
- `go test ./pkg/kernel`: passed.
- `go test -coverprofile=/tmp/update-ipsets-kernel.cover -covermode=atomic ./pkg/kernel`: passed, `88.2%`.
- `go tool cover -func=/tmp/update-ipsets-kernel.cover`: passed.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice3-final.json`: passed.
- `make lint`: passed after Slice 3.
- `make staticcheck`: passed after Slice 3.
- `make golangci-lint`: passed after Slice 3 with `0 issues`.
- `CI=true make coverage`: passed after Slice 3.
- `go tool cover -func=coverage.out`: passed after Slice 3; total statement coverage `71.7%`.
- Project skill validation: not run after Slice 3 because no project skill files changed in Slice 2 or Slice 3.
- `go test ./pkg/asnloc`: passed.
- `go test -coverprofile=/tmp/update-ipsets-asnloc.cover -covermode=atomic ./pkg/asnloc`: passed, `76.2%`.
- `go tool cover -func=/tmp/update-ipsets-asnloc.cover`: passed.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice4.json`: passed.
- `make lint`: passed after Slice 4.
- `make staticcheck`: passed after Slice 4.
- `make golangci-lint`: passed after Slice 4 with `0 issues`.
- `CI=true make coverage`: passed after Slice 4.
- `go tool cover -func=coverage.out`: passed after Slice 4; total statement coverage `72.0%`.
- `go test ./pkg/feedhealth`: passed.
- `go test -coverprofile=/tmp/update-ipsets-feedhealth.cover -covermode=atomic ./pkg/feedhealth`: passed, `88.9%`.
- `go tool cover -func=/tmp/update-ipsets-feedhealth.cover`: passed.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice5.json`: passed.
- `make lint`: passed after Slice 5.
- `make staticcheck`: passed after Slice 5.
- `make golangci-lint`: passed after Slice 5 with `0 issues`.
- `CI=true make coverage`: passed after Slice 5.
- `go tool cover -func=coverage.out`: passed after Slice 5; total statement coverage `72.2%`.
- `git diff --check -- .agents/sow/current/SOW-0102-20260603-quality-complexity-duplication-coverage.md pkg/feedhealth/feedhealth_test.go`: passed after Slice 5.
- `go test ./pkg/engine`: passed after adding pre-refactor entity-integrity tests.
- `go test ./pkg/engine`: passed after entity-integrity scanner refactor.
- `go test -coverprofile=/tmp/update-ipsets-engine-slice6.cover -covermode=atomic ./pkg/engine`: passed, `70.7%`.
- `go tool cover -func=/tmp/update-ipsets-engine-slice6.cover`: passed.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice6.json`: passed.
- `go test -shuffle=on -count=3 ./pkg/engine`: passed.
- `make lint`: passed after Slice 6.
- `make staticcheck`: passed after Slice 6.
- `make golangci-lint`: passed after Slice 6 with `0 issues`.
- `CI=true make coverage`: passed after Slice 6.
- `make test-strict`: passed after Slice 6.
- `go tool cover -func=coverage.out`: passed after Slice 6; total statement coverage `72.2%`.
- `git diff --check -- .agents/sow/current/SOW-0102-20260603-quality-complexity-duplication-coverage.md pkg/engine/entity_integrity.go pkg/engine/entity_integrity_refs.go pkg/engine/entity_integrity_scanner.go pkg/engine/entity_integrity_feed_scan.go pkg/engine/entity_integrity_detail_scan.go pkg/engine/entity_integrity_global_test.go`: passed after Slice 6.
- `go test ./pkg/engine`: passed after comparison-writer refactor.
- `go test -coverprofile=/tmp/update-ipsets-engine-slice7.cover -covermode=atomic ./pkg/engine`: passed, `70.7%`.
- `go tool cover -func=/tmp/update-ipsets-engine-slice7.cover`: passed.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice7.json`: passed.
- `go test -shuffle=on -count=3 ./pkg/engine`: passed after Slice 7.
- `make lint`: passed after Slice 7.
- `make staticcheck`: passed after Slice 7.
- `make golangci-lint`: passed after Slice 7 with `0 issues`.
- `CI=true make coverage`: passed after Slice 7.
- `make test-strict`: passed after Slice 7.
- `go tool cover -func=coverage.out`: passed after Slice 7; total statement coverage `72.3%`.
- `git diff --check -- .agents/sow/current/SOW-0102-20260603-quality-complexity-duplication-coverage.md pkg/engine/output.go`: passed after Slice 7.
- `git diff --no-index --check -- /dev/null pkg/engine/output_comparison.go`: passed after Slice 7.
- `git diff --no-index --check -- /dev/null pkg/engine/output_comparison_helpers.go`: passed after Slice 7.
- Forbidden durable-artifact personal/tool/vendor name scan over the Slice 7 SOW and changed Go files: no matches after Slice 7.
- `go test ./pkg/engine`: passed after Slice 8.
- `go test -coverprofile=/tmp/update-ipsets-engine-slice8.cover -covermode=atomic ./pkg/engine`: passed, `71.2%`.
- `go tool cover -func=/tmp/update-ipsets-engine-slice8.cover`: passed.
- `go test -shuffle=on -count=3 ./pkg/engine`: passed after Slice 8.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice8.json`: passed.
- `make lint`: passed after Slice 8.
- `make staticcheck`: passed after Slice 8.
- `make golangci-lint`: passed after Slice 8 with `0 issues`.
- `CI=true make coverage`: passed after Slice 8.
- `make test-strict`: passed after Slice 8.
- `go tool cover -func=coverage.out`: passed after Slice 8; total statement coverage `72.5%`.
- `git diff --check -- .agents/sow/current/SOW-0102-20260603-quality-complexity-duplication-coverage.md pkg/engine/entity_artifacts.go pkg/engine/home_detail_test.go`: passed after Slice 8.
- `git diff --no-index --check -- /dev/null pkg/engine/entity_artifacts_write.go`: passed after Slice 8.
- `git diff --no-index --check -- /dev/null pkg/engine/entity_artifacts_helpers.go`: passed after Slice 8.
- `git diff --no-index --check -- /dev/null pkg/engine/entity_artifacts_health.go`: passed after Slice 8.
- `git diff --no-index --check -- /dev/null pkg/engine/entity_artifacts_health_test.go`: passed after Slice 8.
- Forbidden durable-artifact personal/tool/vendor name scan over the Slice 8 SOW and changed Go files: no matches after Slice 8.
- `go test ./pkg/engine`: passed after Slice 9.
- `go test -coverprofile=/tmp/update-ipsets-engine-slice9.cover -covermode=atomic ./pkg/engine`: passed, `71.2%`.
- `go tool cover -func=/tmp/update-ipsets-engine-slice9.cover`: passed.
- `go test -shuffle=on -count=3 ./pkg/engine`: passed after Slice 9.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice9.json`: passed.
- `make lint`: passed after Slice 9.
- `make staticcheck`: passed after Slice 9.
- `make golangci-lint`: passed after Slice 9 with `0 issues`.
- `CI=true make coverage`: passed after Slice 9.
- `make test-strict`: passed after Slice 9.
- `go tool cover -func=coverage.out`: passed after Slice 9; total statement coverage `72.5%`.
- `git diff --check -- .agents/sow/current/SOW-0102-20260603-quality-complexity-duplication-coverage.md pkg/engine/entity_surgical.go`: passed after Slice 9.
- `git diff --no-index --check -- /dev/null pkg/engine/entity_surgical_refresh.go`: passed after Slice 9.
- `git diff --no-index --check -- /dev/null pkg/engine/entity_surgical_delta.go`: passed after Slice 9.
- `git diff --no-index --check -- /dev/null pkg/engine/entity_surgical_detail.go`: passed after Slice 9.
- `git diff --no-index --check -- /dev/null pkg/engine/entity_surgical_index.go`: passed after Slice 9.
- `git diff --no-index --check -- /dev/null pkg/engine/entity_surgical_io.go`: passed after Slice 9.
- Forbidden durable-artifact personal/tool/vendor name scan over the Slice 9 SOW and changed Go files: no matches after Slice 9.
- `go test ./pkg/downloader`: passed after Slice 10.
- `go test -shuffle=on -count=3 ./pkg/downloader`: passed after Slice 10.
- `go test -coverprofile=/tmp/update-ipsets-downloader-slice10.cover -covermode=atomic ./pkg/downloader`: passed, `68.4%`.
- `go tool cover -func=/tmp/update-ipsets-downloader-slice10.cover`: passed.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice10.json`: passed.
- `make lint`: passed after Slice 10.
- `make staticcheck`: passed after Slice 10.
- `make golangci-lint`: passed after Slice 10 with `0 issues`.
- `CI=true make coverage`: passed after Slice 10.
- `make test-strict`: passed after Slice 10.
- `go tool cover -func=coverage.out`: passed after Slice 10; total statement coverage `72.5%`.
- `git diff --check -- .agents/sow/current/SOW-0102-20260603-quality-complexity-duplication-coverage.md pkg/downloader/downloader.go`: passed after Slice 10.
- `git diff --no-index --check -- /dev/null pkg/downloader/fetch_http.go`: passed after Slice 10.
- Forbidden durable-artifact personal/tool/vendor name scan over the Slice 10 SOW and changed Go files: no matches after Slice 10.
- `go test ./pkg/engine`: passed after Slice 11.
- `go test -coverprofile=/tmp/update-ipsets-engine-slice11.cover -covermode=atomic ./pkg/engine`: passed, `71.2%`.
- `go tool cover -func=/tmp/update-ipsets-engine-slice11.cover`: passed.
- `go test -shuffle=on -count=3 ./pkg/engine`: passed after Slice 11.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice11.json`: passed.
- `make lint`: passed after Slice 11.
- `make staticcheck`: passed after Slice 11.
- `make golangci-lint`: passed after Slice 11 with `0 issues`.
- `CI=true make coverage`: passed after Slice 11.
- `make test-strict`: passed after Slice 11.
- `go tool cover -func=coverage.out`: passed after Slice 11; total statement coverage `72.5%`.
- `git diff --check -- .agents/sow/current/SOW-0102-20260603-quality-complexity-duplication-coverage.md pkg/engine/home_entity_builders.go`: passed after Slice 11.
- `git diff --no-index --check -- /dev/null pkg/engine/home_entity_detail_live.go`: passed after Slice 11.
- `git diff --no-index --check -- /dev/null pkg/engine/home_entity_detail_payload.go`: passed after Slice 11.
- `git diff --no-index --check -- /dev/null pkg/engine/home_entity_detail_live_test.go`: passed after Slice 11.
- Forbidden durable-artifact personal/tool/vendor name scan over the Slice 11 SOW and changed Go files: no matches after Slice 11.
- `go test ./pkg/web`: passed after Slice 12.
- `go test -coverprofile=/tmp/update-ipsets-web-slice12.cover -covermode=atomic ./pkg/web`: passed, `78.4%` by `go tool cover`.
- `go tool cover -func=/tmp/update-ipsets-web-slice12.cover`: passed.
- `go test -shuffle=on -count=3 ./pkg/web`: passed after Slice 12.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice12.json`: passed.
- `make lint`: passed after Slice 12.
- `make staticcheck`: passed after Slice 12.
- `make golangci-lint`: passed after Slice 12 with `0 issues`.
- `CI=true make coverage`: passed after Slice 12.
- `make test-strict`: passed after Slice 12.
- `go tool cover -func=coverage.out`: passed after Slice 12; total statement coverage `72.5%`.
- `git diff --check -- .agents/sow/current/SOW-0102-20260603-quality-complexity-duplication-coverage.md pkg/web/admin_manifest.go pkg/web/admin_manifest_test.go`: passed after Slice 12.
- `git diff --no-index --check -- /dev/null pkg/web/admin_manifest_builder.go`: passed after Slice 12.
- Forbidden durable-artifact personal/tool/vendor name scan over the Slice 12 SOW and changed Go files: no matches after Slice 12.
- `codacy-analysis analyze --inspect --output-format json --output /tmp/update-ipsets-codacy-inspect.json`: completed but reported `MissingConfig` because this repository does not currently have `.codacy/codacy.config.json`; not used as a code-validation signal.

Real-use evidence:

- The behavioral tests execute the public `RunCLI` entrypoint, validating stdout, stderr, and exit codes for documented operator-visible CLI modes.
- The command tests execute command dispatch and validation paths through `run`, validating stdout, stderr, and exit codes without starting the daemon or engine.
- The systemd tests send and receive real Unix datagram notification payloads.
- The observability tests expose metrics through the real Prometheus handler and validate bounded labels in the exported payload.
- The kernel tests drive `ApplyIfLoaded` orchestration through a fake ipset backend, validating replacement and cleanup behavior without real kernel ipset state.
- The ASN tests drive `Database` range counting through real in-memory range-table backends and temporary text provider files, validating attributed, unknown, and bogon-split outputs without external provider downloads.
- The feed-health tests drive exported policy and classification behavior through real `cache.Entry`, `config.Source`, and `config.RuntimeConfig` values with fixed timestamps.
- The entity-integrity tests drive real rebuild outputs in temporary engine fixtures and then mutate version markers, public ASN payloads, and private ASN sidecars to validate observable findings and targeted repair plans.
- The comparison-writer tests drive comparison artifact behavior through real engine fixtures, file sets, and JSON artifacts, validating subtractive-merge relatedness, prefix pruning, stale zero-overlap cleanup, cancellation, and file-set generation.
- The health-transition entity artifact test drives real rebuild outputs in a temporary engine fixture, corrupts published country/ASN artifacts, and validates refresh through the engine's published artifact outputs and mtimes.
- The surgical entity refresh path remains covered by `TestRefreshEntityArtifactsForFeedUpdatesSurgicallyPatchesAggregates`, which drives real pending sidecars, public/private country and ASN details, indexes, and pending sidecar cleanup.
- The downloader tests drive `Fetch` through real `httptest` servers, temporary files, local file URLs, local copy paths, internal sources, redirects, bad schemes, max-size edges, same-body detection, and POST/header behavior.
- The live entity detail tests drive exported `CountryDetail` and `ASNDetail` APIs, validating invalid identifier errors and empty country/ASN payload behavior without depending on private helper names.
- The admin manifest tests drive real engine test fixtures and manifest builder output, validating configured provider fan-out artifacts, database provider-source paths, and database-feed exclusion of non-database artifact families.
- No daemon, scheduler, public serving, admin UI, install, or runtime behavior was changed intentionally; comparison, entity artifact generation, surgical entity refresh, downloader fetch, live entity detail building, and admin manifest generation changed only by helper extraction and focused behavior tests.

Reviewer findings:

- No external reviewer was run for these local quality slices.

Same-failure scan:

- Source-only `jscpd` was rerun after the duplication refactors. The original `pkg/iprange/cli.go` versus `pkg/iprange/cli6.go` parser clone and the `cli_inputs.go` versus `cli_inputs6.go` loader clone no longer appear.
- Remaining measured duplicate blocks are mapped as follow-up targets below.

Sensitive data gate:

- Passed. No secrets, tokens, cookies, private endpoints, personal data, or customer data were written to durable artifacts.

Artifact maintenance gate:

- AGENTS.md: no update needed; no project-wide workflow or responsibility rule changed.
- Runtime project skills: updated `.agents/skills/project-hygiene/SKILL.md` with durable local structural-quality checks.
- Specs: no update needed; implementation preserves behavior and does not change product contracts.
- End-user/operator docs: no update needed; operator-visible CLI, comparison, entity artifact, and surgical refresh semantics did not change.
- End-user/operator skills: no update needed.
- SOW lifecycle: remains in `.agents/sow/current/`; the first twelve slices are validated but broader quality work continues.

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
  - `pkg/engine/retention.go:58` `(*Engine).updateRetention`: 142 lines, complexity 32.
  - `pkg/engine/runtime.go:81` `resolveRuntime`: 134 lines, complexity 32.
  - `pkg/engine/output.go:142` `(*Engine).buildSetMetadataFromEffectiveEntryInDirWithResolver`: 133 lines, complexity 27.
  - `pkg/engine/critical.go:489` `(*Engine).writeCriticalInfrastructureForFeed`: 130 lines, complexity 26.
  - `pkg/engine/entity_feed_sidecar.go:370` `(*Engine).buildSelectedEntityDetailSidecarsFromFeedSidecars`: 122 lines, complexity 39.
- Next measured duplication targets:
  - `ui/src/components/admin/feeds-table-header.tsx`: 130 duplicated lines.
  - `ui/src/pages/asn-detail.tsx` and `ui/src/pages/country-detail.tsx`: 20, 35, and 107 duplicated line blocks.
  - `ui/src/components/feed-detail/asn-bubble-chart.tsx` and `ui/src/components/feed-detail/asn-treemap.tsx`: 50 duplicated lines.
  - `pkg/iprange/set6_ops.go` and `pkg/iprange/set_ops.go`: 25 duplicated lines.
  - `pkg/engine/runtime_ledger_cache.go`, `pkg/engine/home_entity_precompute.go`, `pkg/engine/home_entity_builders.go`, and `pkg/engine/download_stage.go`: remaining Go duplicate blocks.

## Pre-Implementation Gate - Slice 13

Status: ready.

Problem / root-cause model:

- Facts: after the Slice 12 merge, local `tools/archposture` reports `pkg/config/validate.go:274` `Validate` at 197 lines with complexity 85.
- Facts: `go test -coverprofile=/tmp/update-ipsets-config-slice13-baseline.cover -covermode=atomic ./pkg/config` reports `pkg/config` coverage at `69.7%`; `go tool cover` reports `Validate` at `81.1%` and total package statement coverage at `71.6%`.
- Working theory: `Validate` is doing three separable jobs in one function: top-level/global validation, artifact validation, source validation, and merge validation. Extracting those concerns should reduce complexity without changing config semantics.

Evidence reviewed:

- `pkg/config/validate.go`
- `pkg/config/config.go`
- `pkg/config/config_coverage_test.go`
- `pkg/config/config_test.go`
- `/tmp/update-ipsets-archposture-slice13-baseline.json`
- `/tmp/update-ipsets-config-slice13-baseline.cover`
- Project coding, testing, hygiene, Go best-practices, Go behavioral-testing, and content-surface skills.

Affected contracts and surfaces:

- `pkg/config.Validate` and `pkg/config.ValidateDeep` programmatic behavior.
- YAML/catalog validation semantics for artifacts, sources, merges, runtime URL/resource controls, feed health thresholds, default ASN/geolocation providers, static sources, artifact-backed sources, critical-infrastructure metadata, and merge role constraints.
- FireHOL catalog loading and validation tests.
- SOW only; no end-user docs or specs are expected to change because this is behavior-preserving internal structure work.

Existing patterns to reuse:

- Existing package-local helper validators in `pkg/config/validate.go` such as `validateDefaultProviders`, `validateStaticSourceBody`, `validateCriticalArtifactNamespace`, `validateCriticalMetadata`, `validateUseRoles`, `validateRuntimeURLs`, and `validateRuntimeResourceControls`.
- Existing behavior tests that call the exported `Validate`, `ValidateDeep`, `LoadYAML`, and `LoadDirectory` surfaces rather than private helper functions.
- Existing table-driven config validation tests with synthetic `Config` values and the real FireHOL catalog fixture.

Risk and blast radius:

- Error order within a single artifact, source, or merge must remain stable so operators and tests keep seeing the same first actionable validation failure.
- Map iteration order is already undefined across different entries, so this slice must not add cross-entry ordering promises.
- Config validation protects feed correctness, generated artifact namespace safety, static-source safety, and provider-role correctness; any behavior change here can block valid catalogs or allow invalid ones.
- No public serving, scheduler, downloader, engine publication, install behavior, or UI behavior should change.

Sensitive data handling plan:

- This slice uses only synthetic config values and local test coverage/posture metrics.
- No secrets, tokens, cookies, private endpoints, customer data, or personal data are needed.
- Durable artifacts will record only file paths, metrics, validation outcomes, and sanitized command evidence.

Implementation plan:

1. Extract artifact validation from `Validate` into a package-local helper that preserves the current per-artifact validation order and error text.
2. Extract source validation into package-local helpers for source basics, role compatibility, output shape, provider format requirements, and URL/artifact URL checks while preserving validation order.
3. Extract merge validation into package-local helpers for merge basics and role compatibility while preserving validation order.
4. Add focused exported-contract tests only if the extraction exposes an uncovered validation branch or a contract not already represented in the current tests.
5. Re-run `pkg/config` tests and coverage, then `tools/archposture`, strict validation, lint/static checks, and root coverage as appropriate for this Go config change.

Validation plan:

- Run `go test ./pkg/config`.
- Run `go test -coverprofile=/tmp/update-ipsets-config-slice13.cover -covermode=atomic ./pkg/config` and inspect `go tool cover -func`.
- Run `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice13.json` and confirm `pkg/config/validate.go` no longer appears as a large `Validate` function target.
- Run `go test -shuffle=on -count=3 ./pkg/config`.
- Run `make lint`, `make staticcheck`, `make golangci-lint`, `CI=true make coverage`, and `make test-strict`.
- Run whitespace and durable-artifact forbidden-name scans over the changed files before commit.

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if a repeatable validation-refactor lesson is found.
- Specs: no update expected because config semantics are intended to stay unchanged.
- End-user/operator docs: no update expected.
- End-user/operator skills: no update expected.
- SOW lifecycle: this SOW remains in `.agents/sow/current/`; Slice 13 results will be recorded after validation.

Open decisions:

- No new user design decision is required because the slice is behavior-preserving quality work under the previously approved autonomy decision.

## Slice 13 Results

Changes made:

- Refactored `pkg/config.Validate` into a short coordinator.
- Split artifact, source, merge, and critical-infrastructure validation into smaller files:
  - `pkg/config/validate_artifacts.go`
  - `pkg/config/validate_sources.go`
  - `pkg/config/validate_merges.go`
  - `pkg/config/validate_critical.go`
- Added exported-contract tests in `pkg/config/validate_contract_test.go` for artifact validation and source URL validation, including file URLs, artifact-backed sources, malformed artifact URLs, unknown artifact references, and artifact-backed frequency rules.
- Preserved existing validation order and error messages inside each artifact/source/merge path.

Measured result:

- Baseline: `pkg/config/validate.go:274` `Validate` was 197 lines with complexity 85.
- After refactor: no `pkg/config/validate*` file or function appears in `tools/archposture` large-file or large-function output.
- `pkg/config/validate.go` is now 365 lines; new validation files are 34, 84, 154, and 221 lines.
- `pkg/config/config_test.go` remains at its existing 829-line baseline; the new validation tests live in a separate 180-line file.
- `pkg/config` coverage moved from `69.7%` to `71.2%`.
- `pkg/config` total statement coverage by `go tool cover` moved from `71.6%` to `73.1%`.
- Root coverage by `go tool cover -func=coverage.out` moved from `72.5%` to `72.6%`.
- `tools/archposture` after this slice: source files `621`, source lines `126237`, large files `53`, and large functions `37`.

Tests or equivalent validation:

- `go test ./pkg/config`: passed.
- `go test -coverprofile=/tmp/update-ipsets-config-slice13.cover -covermode=atomic ./pkg/config`: passed, `71.2%`.
- `go tool cover -func=/tmp/update-ipsets-config-slice13.cover`: passed, total `73.1%`.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice13.json`: passed.
- `go test -shuffle=on -count=3 ./pkg/config`: passed.
- `make lint`: passed.
- `make staticcheck`: passed.
- `make golangci-lint`: passed with `0 issues`.
- `CI=true make coverage`: passed.
- `make test-strict`: passed.
- `git diff --check -- .agents/sow/current/SOW-0102-20260603-quality-complexity-duplication-coverage.md pkg/config/validate.go pkg/config/config_test.go pkg/config/validate_contract_test.go pkg/config/validate_artifacts.go pkg/config/validate_sources.go pkg/config/validate_merges.go pkg/config/validate_critical.go`: passed.
- Durable-artifact forbidden-name scan over the changed SOW and Go files produced only public company-name false positives in moved, pre-existing critical ASN rationale strings; no personal name, authorship, tool, or vendor-attribution text was added.

Artifact maintenance gate:

- AGENTS.md: no update needed.
- Runtime project skills: no update needed; no new durable process rule was found.
- Specs: no update needed; config semantics are unchanged.
- End-user/operator docs: no update needed; operator-visible validation semantics and error text are unchanged.
- End-user/operator skills: no update needed.
- SOW lifecycle: remains in `.agents/sow/current/`; Slice 13 is validated and pending PR merge.

## Pre-Implementation Gate - Slice 14

Status: ready.

Problem / root-cause model:

- Facts: after the Slice 13 merge, local `tools/archposture` reports `pkg/web/server.go:291` `Run` at 179 lines with complexity 34.
- Facts: `go test -coverprofile=/tmp/update-ipsets-web-slice14-baseline.cover -covermode=atomic ./pkg/web` reports `pkg/web` coverage at `78.5%`; `go tool cover` reports `Run` coverage at `64.4%` and package total statement coverage at `78.4%`.
- Working theory: `Run` is large because it owns several lifecycle concerns inline: runtime overrides, startup integrity recovery, startup entity artifact checks, scheduler lifetime, handler/server construction, listener acquisition, shutdown watcher, watchdog notification, and serving/waiting.

Evidence reviewed:

- `pkg/web/server.go`
- `pkg/web/run_lifecycle_test.go`
- `pkg/web/routes_test.go`
- `/tmp/update-ipsets-archposture-slice14-baseline.json`
- `/tmp/update-ipsets-web-slice14-baseline.cover`
- Project coding, testing, hygiene, Go best-practices, Go behavioral-testing, and content-surface skills.

Affected contracts and surfaces:

- `pkg/web.Run` daemon web lifecycle behavior.
- HTTP listener construction for shared public/admin mode and split public/admin mode.
- Startup integrity recovery queueing, startup entity artifact refresh, scheduler ownership, systemd ready/stopping/watchdog notifications, TLS serving, shutdown, and return-error behavior.
- Existing lifecycle tests for HTTPS, split admin listeners, and invalid split/admin auth options.
- SOW only; no end-user docs or specs are expected to change because this is behavior-preserving lifecycle structure work.

Existing patterns to reuse:

- Existing `namedServer`, `serveServer`, `isServerClosedError`, `readyMessage`, `newHandlerWithContext`, `newPublicHandlerWithContext`, and `newAdminHandlerWithContext`.
- Existing `run_lifecycle_test.go` black-box tests that drive the exported `Run` function through real TCP listeners and HTTP clients.
- Existing context-owned background worker pattern in `Run`: cancel on shutdown, then wait for startup entity artifact and scheduler goroutines.

Risk and blast radius:

- `Run` is a daemon lifecycle function; incorrect helper extraction can leak goroutines, leave listeners open on partial failure, change shutdown timing, or alter error returns.
- Startup integrity findings must still be logged and queued before the scheduler starts processing.
- Split public/admin listener behavior must stay unchanged, including admin-only route visibility and `runtime.public_base_url` enforcement.
- Systemd notification failures must remain non-fatal except for listener/serve errors that are already returned.
- No public route behavior, admin API behavior, scheduler semantics, engine artifact generation, install behavior, or UI behavior should change.

Sensitive data handling plan:

- This slice uses only local source code, synthetic test servers, temporary files, and local posture/coverage metrics.
- No secrets, tokens, cookies, private endpoints, customer data, or personal data are needed.
- Durable artifacts will record only file paths, metrics, validation outcomes, and sanitized command evidence.

Implementation plan:

1. Extract startup integrity recovery into a helper that preserves logging, recovery-plan queueing, and reason values.
2. Extract startup background worker ownership into a helper that starts entity artifact checks and scheduler run, and returns a wait function used by `Run`'s existing deferred shutdown path.
3. Extract public/admin server construction and listener acquisition into helpers that preserve partial-listener cleanup.
4. Extract shutdown watcher, systemd watchdog, ready/log announcement, and server serve/wait loop into focused helpers.
5. Keep `Run` as the exported lifecycle coordinator and avoid changing route handlers, scheduler APIs, or engine APIs.
6. Add tests only if an extracted contract is not already covered through exported `Run` lifecycle behavior.
7. Re-run `pkg/web` tests and coverage, `tools/archposture`, strict shuffled tests, lint/static checks, and root coverage.

Validation plan:

- Run `go test ./pkg/web`.
- Run `go test -coverprofile=/tmp/update-ipsets-web-slice14.cover -covermode=atomic ./pkg/web` and inspect `go tool cover -func`.
- Run `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice14.json` and confirm `pkg/web/server.go:Run` no longer appears as a large-function target.
- Run `go test -shuffle=on -count=3 ./pkg/web`.
- Run `make lint`, `make staticcheck`, `make golangci-lint`, `CI=true make coverage`, and `make test-strict`.
- Run whitespace and durable-artifact forbidden-name scans over the changed files before commit.

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if a repeatable lifecycle-refactor lesson is found.
- Specs: no update expected because web runtime semantics are intended to stay unchanged.
- End-user/operator docs: no update expected.
- End-user/operator skills: no update expected.
- SOW lifecycle: this SOW remains in `.agents/sow/current/`; Slice 14 results will be recorded after validation.

Open decisions:

- No new user design decision is required because the slice is behavior-preserving quality work under the previously approved autonomy decision.

## Slice 14 Results

Changes made:

- Refactored `pkg/web.Run` into a short exported lifecycle coordinator.
- Added `pkg/web/server_run.go` for web daemon lifecycle helpers:
  - engine startup preparation;
  - startup integrity recovery queueing;
  - startup entity artifact and scheduler goroutine ownership;
  - public/admin server construction;
  - listener acquisition and partial-listener cleanup;
  - shutdown watcher, systemd watchdog, ready announcement, and serve/wait loop.
- Preserved public/admin handler construction, split-listener behavior, systemd notification handling, scheduler reason values, and first-error return behavior.

Measured result:

- Baseline: `pkg/web/server.go:291` `Run` was 179 lines with complexity 34 and `64.4%` direct coverage.
- After refactor: no `pkg/web/server.go` or `pkg/web/server_run.go` file or function appears in `tools/archposture` large-file or large-function output.
- `pkg/web/server.go` is now 418 lines; `pkg/web/server_run.go` is 237 lines.
- `Run` direct coverage moved from `64.4%` to `86.4%`.
- `pkg/web` coverage moved from `78.5%` to `78.6%`; package total statement coverage by `go tool cover` is `78.5%`.
- Root coverage by `go tool cover -func=coverage.out` remains `72.6%`.
- `tools/archposture` after this slice: source files `622`, source lines `126293`, large files `52`, and large functions `36`.

Tests or equivalent validation:

- `go test ./pkg/web`: passed.
- `go test -coverprofile=/tmp/update-ipsets-web-slice14.cover -covermode=atomic ./pkg/web`: passed, `78.6%`.
- `go tool cover -func=/tmp/update-ipsets-web-slice14.cover`: passed, total `78.5%`.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice14.json`: passed.
- `go test -shuffle=on -count=3 ./pkg/web`: passed.
- `make lint`: passed.
- `make staticcheck`: passed.
- `make golangci-lint`: passed with `0 issues`.
- `CI=true make coverage`: passed.
- `make test-strict`: passed.
- `git diff --check -- .agents/sow/current/SOW-0102-20260603-quality-complexity-duplication-coverage.md pkg/web/server.go pkg/web/server_run.go`: passed.
- Durable-artifact forbidden-name scan over the changed SOW and Go files found no personal name, authorship, tool, or vendor-attribution text.

Artifact maintenance gate:

- AGENTS.md: no update needed.
- Runtime project skills: no update needed; no new durable process rule was found.
- Specs: no update needed; web runtime semantics are unchanged.
- End-user/operator docs: no update needed.
- End-user/operator skills: no update needed.
- SOW lifecycle: remains in `.agents/sow/current/`; Slice 14 is validated and pending PR merge.

## Pre-Implementation Gate - Slice 15

Status: ready.

Problem / root-cause model:

- Facts: after the Slice 14 merge, local `tools/archposture` reports `pkg/engine/entity_feed_sidecar.go:374` `(*Engine).buildSingleFeedEntitySidecar` at 171 lines with complexity 53.
- Facts: `go test -coverprofile=/tmp/update-ipsets-engine-slice15-baseline.cover -covermode=atomic ./pkg/engine` reports `pkg/engine` coverage at `71.1%`; `go tool cover` reports `buildSingleFeedEntitySidecar` at `65.0%` and package total statement coverage at `71.2%`.
- Working theory: `buildSingleFeedEntitySidecar` is large because it combines feed eligibility, base sidecar construction, country comparison loading, ASN loading, optional joint country/ASN attribution, ASN name/count backfill, and final sorting/filtering.

Evidence reviewed:

- `pkg/engine/entity_feed_sidecar.go`
- `pkg/engine/entity_surgical_test.go`
- `pkg/engine/home_detail_test.go`
- `pkg/engine/entity_integrity_test.go`
- `/tmp/update-ipsets-archposture-slice15-baseline.json`
- `/tmp/update-ipsets-engine-slice15-baseline.cover`
- Project coding, testing, hygiene, Go best-practices, Go behavioral-testing, and content-surface skills.

Affected contracts and surfaces:

- Feed entity sidecar generation under `pkg/engine`.
- Public country/ASN detail artifacts and indexes that consume feed entity sidecars.
- Entity integrity checks and surgical refresh paths that compare or patch feed sidecars.
- Generated sidecar JSON fields: feed, category, provenance, maintainer, unique IPs, last-change timestamp, provider names, countries, ASNs, joint country/ASN rows, attributed IP counts, and sort order.
- SOW only; no docs or specs are expected to change because the slice is behavior-preserving.

Existing patterns to reuse:

- Existing small helper style in `entity_feed_sidecar.go` for indexing, actor contributions, country/ASN row construction, and sorted outputs.
- Existing `latestSetCache` ownership pattern: create a local cache only when none is passed, and close it with the engine logger.
- Existing tests that drive real entity sidecar JSON through engine fixtures rather than testing private implementation details.

Risk and blast radius:

- Entity sidecars are generated artifacts. A behavior change can corrupt country/ASN detail pages, indexes, integrity checks, and surgical refresh outputs.
- Joint country/ASN attribution must still happen only when geo prepared data, ASN DB, country rows, and ASN rows are all available.
- `latestSetCache` ownership must not leak file descriptors or close a caller-owned cache.
- Sorting and filtering must remain stable: countries sorted by code, ASNs sorted by number, empty rows omitted.
- No scheduler, public serving, downloader, install behavior, or UI behavior should change.

Sensitive data handling plan:

- This slice uses only local source code, synthetic engine fixtures, and local posture/coverage metrics.
- No secrets, tokens, cookies, private endpoints, customer data, or personal data are needed.
- Durable artifacts will record only file paths, metrics, validation outcomes, and sanitized command evidence.

Implementation plan:

1. Move feed sidecar build orchestration into a focused file if needed to reduce file posture.
2. Extract base sidecar creation, country contribution loading, ASN contribution loading, joint attribution enrichment, ASN backfill, and final sidecar sorting/filtering into helper functions.
3. Preserve current error wrapping, counter names, cache ownership, sorting, and empty-sidecar return behavior.
4. Add tests only if a behavior branch is not already covered by exported or fixture-driven entity sidecar behavior.
5. Re-run `pkg/engine` tests and coverage, `tools/archposture`, strict shuffled tests, lint/static checks, and root coverage.

Validation plan:

- Run `go test ./pkg/engine`.
- Run `go test -coverprofile=/tmp/update-ipsets-engine-slice15.cover -covermode=atomic ./pkg/engine` and inspect `go tool cover -func`.
- Run `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice15.json` and confirm `buildSingleFeedEntitySidecar` no longer appears as a large-function target.
- Run `go test -shuffle=on -count=3 ./pkg/engine`.
- Run `make lint`, `make staticcheck`, `make golangci-lint`, `CI=true make coverage`, and `make test-strict`.
- Run whitespace and durable-artifact forbidden-name scans over the changed files before commit.

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if a repeatable entity-artifact refactor lesson is found.
- Specs: no update expected because entity sidecar semantics are intended to stay unchanged.
- End-user/operator docs: no update expected.
- End-user/operator skills: no update expected.
- SOW lifecycle: this SOW remains in `.agents/sow/current/`; Slice 15 results will be recorded after validation.

Open decisions:

- No new user design decision is required because the slice is behavior-preserving quality work under the previously approved autonomy decision.

## Slice 15 Results

Changes made:

- Extracted single-feed entity sidecar construction from `pkg/engine/entity_feed_sidecar.go` into `pkg/engine/entity_feed_sidecar_single.go`.
- Split the old monolithic `buildSingleFeedEntitySidecar` flow into focused helpers for:
  - base sidecar eligibility and metadata construction;
  - country contribution loading;
  - ASN contribution loading;
  - optional joint country/ASN attribution;
  - ASN name/count backfill from joint rows;
  - final country and ASN sorting/filtering.
- Preserved `latestSetCache` ownership semantics, existing metric counter names, error wrapping, empty-sidecar behavior, country sort order, ASN sort order, and omitted empty rows.
- Kept batch build, staging, and worker orchestration in `pkg/engine/entity_feed_sidecar_build.go`.

Measured result:

- Baseline: `pkg/engine/entity_feed_sidecar.go:374` `(*Engine).buildSingleFeedEntitySidecar` was 171 lines with complexity 53 and `65.0%` direct coverage.
- After refactor: `buildSingleFeedEntitySidecar` is 22 lines with `78.6%` direct coverage and no `pkg/engine/entity_feed_sidecar_single.go` function appears in `tools/archposture` large-function output.
- `pkg/engine/entity_feed_sidecar.go` moved from 743 lines to 567 lines.
- `pkg/engine/entity_feed_sidecar_single.go` is 237 lines.
- `pkg/engine/entity_feed_sidecar_build.go` remains 254 lines and unchanged in behavior.
- `pkg/engine` coverage moved from `71.1%` to `71.2%` by `go test` output; package total statement coverage by `go tool cover` remains `71.2%`.
- Root coverage by `go tool cover -func=coverage.out` remains `72.6%`.
- `tools/archposture` after this slice: source files `623`, source lines `126354`, large files `52`, and large functions `35`.
- Remaining entity sidecar posture: `pkg/engine/entity_feed_sidecar.go` is still a review-size file at 567 lines, and `buildSelectedEntityDetailSidecarsFromFeedSidecars` remains a lower-priority large function at 122 lines with complexity 39.

Tests or equivalent validation:

- `go test ./pkg/engine`: passed.
- `go test -coverprofile=/tmp/update-ipsets-engine-slice15.cover -covermode=atomic ./pkg/engine`: passed, `71.2%`.
- `go tool cover -func=/tmp/update-ipsets-engine-slice15.cover`: passed, total `71.2%`.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice15.json`: passed.
- `go test -shuffle=on -count=3 ./pkg/engine`: passed.
- `make lint`: passed.
- `make staticcheck`: passed.
- `make golangci-lint`: passed with `0 issues`.
- `CI=true make coverage`: passed.
- `make test-strict`: passed.
- `git diff --check`: passed.
- Durable-artifact forbidden-name scan over the changed SOW and Go files found no newly added personal name, authorship, tool, or vendor-attribution text.

Artifact maintenance gate:

- AGENTS.md: no update needed.
- Runtime project skills: no update needed; no new durable process rule was found.
- Specs: no update needed; entity sidecar semantics are unchanged.
- End-user/operator docs: no update needed.
- End-user/operator skills: no update needed.
- SOW lifecycle: remains in `.agents/sow/current/`; Slice 15 is validated and pending PR merge.

## Pre-Implementation Gate - Slice 16

Status: ready.

Problem / root-cause model:

- Facts: after the Slice 15 merge, local `tools/archposture` reports `pkg/engine/integrity.go:110` `(*Engine).CheckIntegrityWithOptions` at 156 lines with complexity 36.
- Facts: `go test -coverprofile=/tmp/update-ipsets-integrity-slice16-baseline.cover -covermode=atomic ./pkg/engine` reports `pkg/engine` coverage at `71.2%`; `go tool cover` reports `CheckIntegrityWithOptions` at `81.2%` and package total statement coverage at `71.2%`.
- Working theory: `CheckIntegrityWithOptions` is large because it combines runtime path resolution, feed eligibility, archive filtering, blocked-input finding construction, never-processed detection, source-body checks, in-flight suppression, history-derivative handling, secondary artifact scanning, policy validation, reason construction, sorting, and output accumulation.

Evidence reviewed:

- `pkg/engine/integrity.go`
- `pkg/engine/integrity_test.go`
- `/tmp/update-ipsets-archposture-slice16-baseline.json`
- `/tmp/update-ipsets-integrity-slice16-baseline.cover`
- Project coding, testing, hygiene, Go best-practices, Go behavioral-testing, and content-surface skills.

Affected contracts and surfaces:

- `Engine.CheckIntegrity` and `Engine.CheckIntegrityWithOptions`.
- Admin integrity API behavior that exposes `IntegrityFinding` fields.
- Startup integrity recovery decisions that consume integrity findings.
- Recovery planning for recheck versus reprocess.
- Generated artifact integrity semantics: missing, stale, malformed, blocked feeds, source mtime fields, processed timestamp, recovery reason text, and deterministic list sorting.
- SOW only; no docs or specs are expected to change because the slice is behavior-preserving.

Existing patterns to reuse:

- Existing integrity tests in `pkg/engine/integrity_test.go` that drive exported integrity checks through temp directories and engine fixtures.
- Existing helper style in `integrity.go` for reason building, blocked-feed calculation, history derivative checks, source-body lookup, and expected secondary artifact enumeration.
- Existing recovery/action semantics that treat missing committed sources differently from stale/missing secondary artifacts.

Risk and blast radius:

- Integrity findings protect startup recovery and operator repair workflows. A behavior change can hide broken artifacts or produce noisy false positives.
- Archive filtering and `EnableAll` behavior must remain identical.
- In-flight suppression must still happen before expensive or noisy secondary checks.
- History-derived feeds must still prefer parent recheck when downloader-owned snapshots are missing or corrupt.
- Metadata redistributability policy validation must still use the same archive-aware policy.
- Deterministic sorting of missing, stale, malformed, and blocked lists must remain unchanged.

Sensitive data handling plan:

- This slice uses only local source code, synthetic engine fixtures, temporary directories, and local posture/coverage metrics.
- No secrets, tokens, cookies, private endpoints, customer data, or personal data are needed.
- Durable artifacts will record only file paths, metrics, validation outcomes, and sanitized command evidence.

Implementation plan:

1. Move the exported integrity check coordinator and its new focused helpers into a dedicated integrity-check file if that reduces file posture.
2. Extract runtime-path validation, per-source eligibility/context gathering, blocked-input finding construction, unprocessed-source handling, missing-source handling, and secondary artifact scanning into focused helpers.
3. Preserve existing helper behavior for history derivatives, reason construction, blocked bogon provider artifacts, expected artifact enumeration, and structured artifact validation.
4. Add tests only if the extraction exposes a valid branch not already covered through exported integrity behavior.
5. Re-run `pkg/engine` tests and coverage, `tools/archposture`, strict shuffled tests, lint/static checks, and root coverage.

Validation plan:

- Run `go test ./pkg/engine`.
- Run `go test -coverprofile=/tmp/update-ipsets-integrity-slice16.cover -covermode=atomic ./pkg/engine` and inspect `go tool cover -func`.
- Run `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice16.json` and confirm `CheckIntegrityWithOptions` no longer appears as a large-function target.
- Run `go test -shuffle=on -count=3 ./pkg/engine`.
- Run `make lint`, `make staticcheck`, `make golangci-lint`, `CI=true make coverage`, and `make test-strict`.
- Run whitespace and durable-artifact forbidden-name scans over the changed files before commit.

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if a repeatable integrity-refactor lesson is found.
- Specs: no update expected because integrity semantics are intended to stay unchanged.
- End-user/operator docs: no update expected.
- End-user/operator skills: no update expected.
- SOW lifecycle: this SOW remains in `.agents/sow/current/`; Slice 16 results will be recorded after validation.

Open decisions:

- No new user design decision is required because the slice is behavior-preserving quality work under the previously approved quality plan.

## Slice 16 Results

Changes made:

- Moved the exported integrity check coordinator from `pkg/engine/integrity.go` into `pkg/engine/integrity_check.go`.
- Split `CheckIntegrityWithOptions` into focused helpers for:
  - runtime path validation;
  - per-source eligibility/context gathering;
  - blocked merge input findings;
  - enabled-but-never-processed findings;
  - missing committed source findings;
  - secondary artifact scanning;
  - final reason and list sorting.
- Preserved existing helper behavior for history derivatives, blocked-feed calculation, blocked bogon provider artifacts, expected secondary artifact enumeration, structured artifact validation, archive filtering, `EnableAll`, in-flight suppression, and recovery reason text.
- Added `pkg/engine/integrity_blocked_test.go` to cover blocked merge input reporting through the exported `CheckIntegrityWithOptions` path.
- Kept the new behavioral test out of the already-large `pkg/engine/integrity_test.go`; that file remains at its 1207-line baseline.

Measured result:

- Baseline: `pkg/engine/integrity.go:110` `(*Engine).CheckIntegrityWithOptions` was 156 lines with complexity 36 and `81.2%` direct coverage.
- After refactor: `CheckIntegrityWithOptions` is a short coordinator and no `pkg/engine/integrity*.go` production function appears in `tools/archposture` large-function output.
- `pkg/engine/integrity.go` moved to 376 lines.
- `pkg/engine/integrity_check.go` is 246 lines.
- `pkg/engine/integrity_blocked_test.go` is 59 lines.
- `pkg/engine` coverage moved from `71.2%` to `71.3%`.
- Root coverage by `go tool cover -func=coverage.out` remains `72.6%`.
- `tools/archposture` after this slice: source files `625`, source lines `126476`, large files `51`, and large functions `34`.
- Remaining top production complexity target: `pkg/engine/metadata.go:16` `(*Engine).writeMetadataFiles`, 150 lines, complexity 28.

Tests or equivalent validation:

- `go test ./pkg/engine`: passed.
- `go test -coverprofile=/tmp/update-ipsets-integrity-slice16.cover -covermode=atomic ./pkg/engine`: passed, `71.3%`.
- `go tool cover -func=/tmp/update-ipsets-integrity-slice16.cover`: passed, total `71.3%`.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice16.json`: passed.
- `go test -shuffle=on -count=3 ./pkg/engine`: passed.
- `make lint`: passed.
- `make staticcheck`: passed.
- `make golangci-lint`: passed with `0 issues`.
- `CI=true make coverage`: passed.
- `make test-strict`: passed.
- `git diff --check`: passed.
- Durable-artifact forbidden-name scan over the changed SOW and Go files found no newly added personal name, authorship, tool, or vendor-attribution text.

Artifact maintenance gate:

- AGENTS.md: no update needed.
- Runtime project skills: no update needed; no new durable process rule was found.
- Specs: no update needed; integrity semantics are unchanged.
- End-user/operator docs: no update needed.
- End-user/operator skills: no update needed.
- SOW lifecycle: remains in `.agents/sow/current/`; Slice 16 is validated and pending PR merge.

## Pre-Implementation Gate - Slice 17

Status: ready.

Problem / root-cause model:

- Facts: after the Slice 16 merge, local `tools/archposture` reports `pkg/engine/metadata.go:16` `(*Engine).writeMetadataFiles` at 150 lines with complexity 28.
- Facts: `go test -coverprofile=/tmp/update-ipsets-metadata-slice17-baseline.cover -covermode=atomic ./pkg/engine` reports `pkg/engine` coverage at `71.3%`; `go tool cover` reports `writeMetadataFiles` at `79.8%` and package total statement coverage at `71.3%`.
- Working theory: `writeMetadataFiles` is large because it combines comparison artifact refresh, public metadata file generation, per-feed metadata/index accumulation, per-feed JSON/history/changesets/retention writing, generated-file ledger updates, optional git-side artifact writing, and operation timing.

Evidence reviewed:

- `pkg/engine/metadata.go`
- `pkg/engine/file_contract_test.go`
- `pkg/engine/output_metadata_test.go`
- `pkg/engine/output_test.go`
- `/tmp/update-ipsets-archposture-slice17-baseline.json`
- `/tmp/update-ipsets-metadata-slice17-baseline.cover`
- Project coding, testing, hygiene, Go best-practices, Go behavioral-testing, and content-surface skills.

Affected contracts and surfaces:

- Metadata/index artifact publication from the engine pipeline.
- Generated-file ledger entries and logical timestamps for per-feed JSON, history, changesets, retention, index, all-ipsets, setinfo, and feed body outputs.
- Optional git-side outputs: README, `.gitignore`, and timestamp script.
- Public metadata JSON fields consumed by public APIs, integrity checks, and website pages.
- SOW only; no docs or specs are expected to change because the slice is behavior-preserving.

Existing patterns to reuse:

- Existing timing instrumentation with `observeRunOperation`.
- Existing generated-file ledger style using `output.GeneratedFile`.
- Existing effective-entry resolver reuse for batch loops.
- Existing test contracts that verify metadata fields, file contracts, and bash-compatible output behavior through engine fixtures.

Risk and blast radius:

- Generated-file timestamps are integrity data. Ledger path/timestamp changes can create false stale findings or hide stale artifacts.
- Public metadata files must be written before indexes observe their updated unique-share fields.
- Per-feed output filtering must still honor `perFeedNames`.
- Base git artifacts must only be written when `output.HasGitDir(e.runtime.BaseDir)` is true.
- No pipeline scheduling, downloader, public serving, install behavior, or UI behavior should change.

Sensitive data handling plan:

- This slice uses only local source code, synthetic engine fixtures, temporary files, and local posture/coverage metrics.
- No secrets, tokens, cookies, private endpoints, customer data, or personal data are needed.
- Durable artifacts will record only file paths, metrics, validation outcomes, and sanitized command evidence.

Implementation plan:

1. Move metadata write orchestration into a focused metadata writer file if that reduces file posture.
2. Introduce a package-local write-run helper that owns generated-file accumulation, per-feed/index buffers, resolver state, output directories, and git-output state.
3. Extract comparison/public metadata writes, per-feed output writes, index writes, and git artifact writes into focused helpers while preserving timing labels and error returns.
4. Add tests only if a behavior branch is not already covered by exported engine metadata/file-contract behavior.
5. Re-run `pkg/engine` tests and coverage, `tools/archposture`, strict shuffled tests, lint/static checks, and root coverage.

Validation plan:

- Run `go test ./pkg/engine`.
- Run `go test -coverprofile=/tmp/update-ipsets-metadata-slice17.cover -covermode=atomic ./pkg/engine` and inspect `go tool cover -func`.
- Run `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice17.json` and confirm `writeMetadataFiles` no longer appears as a large-function target.
- Run `go test -shuffle=on -count=3 ./pkg/engine`.
- Run `make lint`, `make staticcheck`, `make golangci-lint`, `CI=true make coverage`, and `make test-strict`.
- Run whitespace and durable-artifact forbidden-name scans over the changed files before commit.

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if a repeatable metadata-writer refactor lesson is found.
- Specs: no update expected because metadata semantics are intended to stay unchanged.
- End-user/operator docs: no update expected.
- End-user/operator skills: no update expected.
- SOW lifecycle: this SOW remains in `.agents/sow/current/`; Slice 17 results will be recorded after validation.

Open decisions:

- No new user design decision is required because the slice is behavior-preserving quality work under the previously approved quality plan.

## Slice 17 Results

Changes made:

- Moved metadata write orchestration from `pkg/engine/metadata.go` into `pkg/engine/metadata_write.go`.
- Split `writeMetadataFiles` into focused helpers for:
  - comparison and unique-share refresh;
  - public metadata list generation;
  - per-feed metadata/index row accumulation;
  - per-feed JSON/history/changesets/retention artifact writes;
  - index and all-ipsets writes;
  - optional git-side README, `.gitignore`, and timestamp-script writes;
  - generated-file ledger appends.
- Preserved timing labels, generated-file ledger paths, logical timestamps, redistributability flags, `perFeedNames` filtering, base-git behavior, resolver reuse, and error returns.

Measured result:

- Baseline: `pkg/engine/metadata.go:16` `(*Engine).writeMetadataFiles` was 150 lines with complexity 28 and `79.8%` direct coverage.
- After refactor: no `pkg/engine/metadata*.go` function appears in `tools/archposture` large-function output.
- `pkg/engine/metadata.go` moved to 111 lines.
- `pkg/engine/metadata_write.go` is 253 lines.
- `pkg/engine` coverage remains `71.3%`.
- Root coverage by `go tool cover -func=coverage.out` remains `72.6%`.
- `tools/archposture` after this slice: source files `626`, source lines `126576`, large files `51`, and large functions `33`.
- Remaining top production complexity target: `pkg/engine/retention.go:58` `(*Engine).updateRetention`, 142 lines, complexity 32.

Tests or equivalent validation:

- `go test ./pkg/engine`: passed.
- `go test -coverprofile=/tmp/update-ipsets-metadata-slice17.cover -covermode=atomic ./pkg/engine`: passed, `71.3%`.
- `go tool cover -func=/tmp/update-ipsets-metadata-slice17.cover`: passed, total `71.3%`.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice17.json`: passed.
- `go test -shuffle=on -count=3 ./pkg/engine`: passed.
- `make lint`: passed.
- `make staticcheck`: passed.
- `make golangci-lint`: passed with `0 issues`.
- `CI=true make coverage`: passed.
- `make test-strict`: passed.
- `git diff --check`: passed.
- Durable-artifact forbidden-name scan over the changed SOW and Go files found no newly added personal name, authorship, tool, or vendor-attribution text.

Artifact maintenance gate:

- AGENTS.md: no update needed.
- Runtime project skills: no update needed; no new durable process rule was found.
- Specs: no update needed; metadata semantics are unchanged.
- End-user/operator docs: no update needed.
- End-user/operator skills: no update needed.
- SOW lifecycle: remains in `.agents/sow/current/`; Slice 17 is validated and pending PR merge.

## Pre-Implementation Gate - Slice 18

Status: ready.

Problem / root-cause model:

- Facts: after the Slice 17 merge, local `tools/archposture` reports `pkg/engine/retention.go:58` `(*Engine).updateRetention` at 142 lines with complexity 32.
- Facts: `go test -coverprofile=/tmp/update-ipsets-retention-slice18-baseline.cover -covermode=atomic ./pkg/engine` reports `pkg/engine` coverage at `71.3%`; `go tool cover` reports `updateRetention` at `76.3%` and package total statement coverage at `71.3%`.
- Working theory: `updateRetention` is large because it combines directory preparation, new/removed set delta calculation, changeset ledger updates, cohort creation, no-removal fast-path artifact refresh, removal reconciliation, retention cache mutation, JSON publication, and bash histogram cache writing.

Evidence reviewed:

- `pkg/engine/retention.go`
- `pkg/engine/retention_test.go`
- `pkg/engine/runtime_ledger_cache.go`
- `pkg/engine/file_contract_test.go`
- `pkg/engine/public_series.go`
- `/tmp/update-ipsets-archposture-slice18-baseline.json`
- `/tmp/update-ipsets-retention-slice18-baseline.cover`
- Project coding, testing, hygiene, Go best-practices, Go behavioral-testing, and content-surface skills.

Affected contracts and surfaces:

- Retention cohort files under `lib/{feed}/new/`.
- Retention ledgers: `changesets.csv`, `retention.csv`, and `retention_cohorts.csv`.
- Generated retention artifacts: `retention.json`, bash `histogram`, and public `{feed}_retention.json` consumers.
- Runtime retention ledger cache updates used by query, public series, insights, and integrity paths.
- SOW only; no docs or specs are expected to change because the slice is behavior-preserving.

Existing patterns to reuse:

- Existing `loadSnapshotSet` compatibility with binary and legacy text retention snapshots.
- Existing runtime ledger cache helpers: `retentionPastFromRuntime`, `observeRetentionPast`, `retentionCohortsFromRuntime`, `observeRetentionCohort`, and `replaceRetentionCohorts`.
- Existing writer helpers: `appendCSV`, `writeBinaryPath`, `writeRetentionCohortIndex`, `writeRetentionHistogramCache`, and `writeFileAtomic`.
- Existing behavior tests that verify bash-compatible retention outputs and ignored atomic temp files.

Risk and blast radius:

- Retention cohort mtimes and file names are compatibility data. New cohort files must keep the logical `updatedAt` mtime.
- Legacy text-format retention snapshots must continue loading through `loadSnapshotSet`.
- Atomic temp files and malformed cohort names must continue to be ignored or warned about as before.
- Runtime retention caches must reflect additions and removals so query and public artifacts do not drift.
- No pipeline scheduling, downloader, public serving, install behavior, or UI behavior should change.

Sensitive data handling plan:

- This slice uses only local source code, synthetic engine fixtures, temporary files, and local posture/coverage metrics.
- No secrets, tokens, cookies, private endpoints, customer data, or personal data are needed.
- Durable artifacts will record only file paths, metrics, validation outcomes, and sanitized command evidence.

Implementation plan:

1. Move retention update orchestration into a focused retention update file if that improves file posture.
2. Extract changeset/cohort delta recording from removal reconciliation.
3. Extract no-removal artifact refresh into a helper that reuses cached cohorts.
4. Extract removal reconciliation over `new/` cohort files into focused helpers for scanning, per-cohort accounting, stale cohort removal, and partial cohort rewrite.
5. Keep retention artifact finalization in one helper so `retention_cohorts.csv`, `histogram`, and `retention.json` stay synchronized.
6. Add tests only if the refactor exposes an observable behavior branch that is not already covered.
7. Re-run `pkg/engine` tests and coverage, `tools/archposture`, strict shuffled tests, lint/static checks, and root coverage.

Validation plan:

- Run `go test ./pkg/engine`.
- Run `go test -coverprofile=/tmp/update-ipsets-retention-slice18.cover -covermode=atomic ./pkg/engine` and inspect `go tool cover -func`.
- Run `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice18.json` and confirm `updateRetention` no longer appears as a large-function target.
- Run `go test -shuffle=on -count=3 ./pkg/engine`.
- Run `make lint`, `make staticcheck`, `make golangci-lint`, `CI=true make coverage`, and `make test-strict`.
- Run whitespace and durable-artifact forbidden-name scans over the changed files before commit.

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if a repeatable retention-refactor lesson is found.
- Specs: no update expected because retention semantics are intended to stay unchanged.
- End-user/operator docs: no update expected.
- End-user/operator skills: no update expected.
- SOW lifecycle: this SOW remains in `.agents/sow/current/`; Slice 18 results will be recorded after validation.

Open decisions:

- No new user design decision is required because the slice is behavior-preserving quality work under the previously approved quality plan.

## Slice 18 Results

Changes made:

- Moved retention update orchestration from `pkg/engine/retention.go` into `pkg/engine/retention_update.go`.
- Split `updateRetention` into focused helpers for:
  - retention directory preparation;
  - new/removed set delta calculation;
  - changeset ledger and new cohort recording;
  - no-removal artifact refresh from cached cohorts;
  - removal reconciliation over `new/` cohort snapshots;
  - retention removal ledger/cache accounting;
  - retention cohort index, histogram, and JSON finalization.
- Preserved binary and legacy text snapshot loading through `loadSnapshotSet`, atomic temp-file ignore behavior, malformed filename warnings, cohort file mtimes, runtime retention cache updates, and public retention artifact contents.

Measured result:

- Baseline: `pkg/engine/retention.go:58` `(*Engine).updateRetention` was 142 lines with complexity 32 and `76.3%` direct coverage.
- After refactor: no retention production function appears in `tools/archposture` large-function output.
- `pkg/engine/retention.go` moved to 201 lines.
- `pkg/engine/retention_update.go` is 231 lines.
- `updateRetention` direct coverage moved to `78.9%`.
- `pkg/engine` coverage remains `71.3%`.
- Root coverage by `go tool cover -func=coverage.out` remains `72.6%`.
- `tools/archposture` after this slice: source files `627`, source lines `126662`, large files `51`, and large functions `32`.
- Remaining top production complexity target: `pkg/engine/runtime.go:81` `resolveRuntime`, 134 lines, complexity 32.

Tests or equivalent validation:

- `go test ./pkg/engine`: passed.
- `go test -coverprofile=/tmp/update-ipsets-retention-slice18.cover -covermode=atomic ./pkg/engine`: passed, `71.3%`.
- `go tool cover -func=/tmp/update-ipsets-retention-slice18.cover`: passed, total `71.3%`.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice18.json`: passed.
- `go test -shuffle=on -count=3 ./pkg/engine`: passed.
- `make lint`: passed.
- `make staticcheck`: passed.
- `make golangci-lint`: passed with `0 issues`.
- `CI=true make coverage`: passed.
- `make test-strict`: passed.
- `git diff --check`: passed.
- Durable-artifact forbidden-name scan over the changed SOW and Go files found no newly added personal name, authorship, tool, or vendor-attribution text.

Artifact maintenance gate:

- AGENTS.md: no update needed.
- Runtime project skills: no update needed; no new durable process rule was found.
- Specs: no update needed; retention semantics are unchanged.
- End-user/operator docs: no update needed.
- End-user/operator skills: no update needed.
- SOW lifecycle: remains in `.agents/sow/current/`; Slice 18 is validated and pending PR merge.

## Pre-Implementation Gate - Slice 19

Status: ready.

Problem / root-cause model:

- Facts: after the Slice 18 merge, local `tools/archposture` reports `pkg/engine/runtime.go:81` `resolveRuntime` at 134 lines with complexity 32.
- Facts: `go test -coverprofile=/tmp/update-ipsets-runtime-slice19-baseline.cover -covermode=atomic ./pkg/engine` reports `pkg/engine` coverage at `71.3%`; `go tool cover` reports `resolveRuntime` at `70.7%` and package total statement coverage at `71.3%`.
- Working theory: `resolveRuntime` is large because it combines nil-config validation, user-mode path template override selection, variable expansion, direct field mapping, duration conversion, root/user kernel-apply policy, and fallback defaults for directories, timeouts, queues, and worker pools.

Evidence reviewed:

- `pkg/engine/runtime.go`
- `pkg/engine/runtime_test.go`
- `pkg/engine/engine.go`
- `pkg/config/config.go`
- `/tmp/update-ipsets-archposture-slice19-baseline.json`
- `/tmp/update-ipsets-runtime-slice19-baseline.cover`
- Project coding, testing, hygiene, Go best-practices, Go behavioral-testing, and content-surface skills.

Affected contracts and surfaces:

- Runtime path resolution for base, run-parent, cache, lib, history, errors, temp, web, and published file directories.
- Shell-style template expansion variables: `HOME`, `run_parent_dir`, `RUN_PARENT_DIR`, `base_dir`, and `BASE_DIR`.
- Non-root user-mode defaults under the user's home directory.
- Root-only `IPSetsApply` behavior.
- Runtime defaults for timeouts, download/concurrency settings, scheduling intervals, and worker pools.
- SOW only; no docs or specs are expected to change because the slice is behavior-preserving.

Existing patterns to reuse:

- Existing `config.DefaultRuntime()` and `config.New()` defaults.
- Existing `expandTemplate` and shell-style expansion helpers.
- Existing `autoHeavyPhaseWorkers`, `HeavyPhaseWorkers`, and `BackgroundWorkers` helpers.
- Existing runtime tests for user-mode defaults, queue defaults, web artifact cache controls, heavy workers, background workers, and runtime overrides.

Risk and blast radius:

- Runtime defaults affect install-time, daemon, admin, local user, and CI behavior.
- User-mode path fallback depends on the effective UID, so tests must not assume root/non-root behavior without checking `os.Geteuid`.
- Template variable order matters because several runtime paths can depend on resolved `run_parent_dir` and `base_dir`.
- `IPSetsApply` must stay disabled for non-root runs even when config enables it.
- No scheduler, downloader, public serving, install script, or UI behavior should change.

Sensitive data handling plan:

- This slice uses only local source code, synthetic config values, temporary paths, and local posture/coverage metrics.
- No secrets, tokens, cookies, private endpoints, customer data, or personal data are needed.
- Durable artifacts will record only file paths, metrics, validation outcomes, and sanitized command evidence.

Implementation plan:

1. Extract runtime template context creation and user-mode default selection into focused helpers.
2. Extract direct runtime field mapping into a helper that performs only template expansion, duration conversion, and plain field copies.
3. Extract fallback default normalization for paths, timeouts, queues, intervals, and worker pools into focused helpers.
4. Preserve all field values, root/non-root behavior, template variables, and fallback constants.
5. Add tests only if the refactor exposes an observable runtime branch not already covered.
6. Re-run `pkg/engine` tests and coverage, `tools/archposture`, strict shuffled tests, lint/static checks, and root coverage.

Validation plan:

- Run `go test ./pkg/engine`.
- Run `go test -coverprofile=/tmp/update-ipsets-runtime-slice19.cover -covermode=atomic ./pkg/engine` and inspect `go tool cover -func`.
- Run `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice19.json` and confirm `resolveRuntime` no longer appears as a large-function target.
- Run `go test -shuffle=on -count=3 ./pkg/engine`.
- Run `make lint`, `make staticcheck`, `make golangci-lint`, `CI=true make coverage`, and `make test-strict`.
- Run whitespace and durable-artifact forbidden-name scans over the changed files before commit.

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if a repeatable runtime-defaults refactor lesson is found.
- Specs: no update expected because runtime semantics are intended to stay unchanged.
- End-user/operator docs: no update expected.
- End-user/operator skills: no update expected.
- SOW lifecycle: this SOW remains in `.agents/sow/current/`; Slice 19 results will be recorded after validation.

Open decisions:

- No new user design decision is required because the slice is behavior-preserving quality work under the previously approved quality plan.

## Slice 19 Results

Changes made:

- Split `resolveRuntime` into focused helpers for:
  - runtime path/template context creation;
  - user-mode default template selection;
  - direct runtime field mapping and duration conversion;
  - path fallback defaults;
  - download and queue fallback defaults;
  - worker and interval fallback defaults.
- Preserved path template variables, non-root user-mode path defaults, root-only `IPSetsApply`, runtime cache controls, timeout defaults, queue defaults, and worker defaults.

Measured result:

- Baseline: `pkg/engine/runtime.go:81` `resolveRuntime` was 134 lines with complexity 32 and `70.7%` direct coverage.
- After refactor: no `pkg/engine/runtime.go` function appears in `tools/archposture` large-function output.
- `pkg/engine/runtime.go` is 360 lines.
- `resolveRuntime` direct coverage moved to `83.3%`.
- `pkg/engine` coverage remains `71.3%`.
- Root coverage by `go tool cover -func=coverage.out` remains `72.6%`.
- `tools/archposture` after this slice: source files `627`, source lines `126696`, large files `51`, and large functions `31`.
- Remaining top production complexity target: `pkg/engine/output.go:142` `(*Engine).buildSetMetadataFromEffectiveEntryInDirWithResolver`, 133 lines, complexity 27.

Tests or equivalent validation:

- `go test ./pkg/engine`: passed.
- `go test -coverprofile=/tmp/update-ipsets-runtime-slice19.cover -covermode=atomic ./pkg/engine`: passed, `71.3%`.
- `go tool cover -func=/tmp/update-ipsets-runtime-slice19.cover`: passed, total `71.3%`.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice19.json`: passed.
- `go test -shuffle=on -count=3 ./pkg/engine`: passed.
- `make lint`: passed.
- `make staticcheck`: passed.
- `make golangci-lint`: passed with `0 issues`.
- `CI=true make coverage`: passed.
- `make test-strict`: passed.
- `git diff --check`: passed.
- Durable-artifact forbidden-name scan over the changed SOW and Go files found no newly added personal name, authorship, tool, or vendor-attribution text.

Artifact maintenance gate:

- AGENTS.md: no update needed.
- Runtime project skills: no update needed; no new durable process rule was found.
- Specs: no update needed; runtime semantics are unchanged.
- End-user/operator docs: no update needed.
- End-user/operator skills: no update needed.
- SOW lifecycle: remains in `.agents/sow/current/`; Slice 19 is validated and pending PR merge.

## Pre-Implementation Gate - Slice 20

Status: ready.

Problem / root-cause model:

- Facts: after the Slice 19 merge, local `tools/archposture` reports `pkg/engine/output.go:142` `(*Engine).buildSetMetadataFromEffectiveEntryInDirWithResolver` at 133 lines with complexity 27.
- Facts: `go test -coverprofile=/tmp/update-ipsets-output-metadata-slice20-baseline.cover -covermode=atomic ./pkg/engine` reports `pkg/engine` coverage at `71.3%`; `go tool cover` reports `buildSetMetadataFromEffectiveEntryInDirWithResolver` at `98.1%` and package total statement coverage at `71.3%`.
- Working theory: the metadata builder is large because it combines base bash-compatible metadata construction, live config fallback for license/attribution, comparison and geo artifact discovery, raw redistributable URL exposure, source-config enrichment/provenance fields, merge composition fields, and final raw-field suppression for non-redistributable or archived feeds.

Evidence reviewed:

- `pkg/engine/output.go`
- `pkg/engine/output_test.go`
- `pkg/engine/output_metadata_test.go`
- `pkg/engine/public_enrichment_test.go`
- `pkg/engine/file_contract_test.go`
- `pkg/engine/metadata_write.go`
- `/tmp/update-ipsets-archposture-slice20-baseline.json`
- `/tmp/update-ipsets-output-metadata-slice20-baseline.cover`
- Project coding, testing, hygiene, Go best-practices, Go behavioral-testing, and content-surface skills.

Affected contracts and surfaces:

- Per-feed public metadata JSON returned by `Metadata` and written as `{feed}.json`.
- Bash-compatible metadata fields and millisecond timestamp semantics.
- Redistributability and archived-feed raw field suppression.
- Geolocation provider artifact discovery from configured `use: geoip` sources.
- Source enrichment/provenance fields consumed by public API/UI.
- Merge composition fields and `enableAll` semantics.
- SOW only; no docs or specs are expected to change because the slice is behavior-preserving.

Existing patterns to reuse:

- Existing `lookupSource`, `publicProvenance`, `formatProcessorSteps`, `mergeCompositionWithResolver`, and health classification helpers.
- Existing file-existence checks against both selected output directory and engine output directory.
- Existing tests for merge composition, non-redistributable metadata suppression, markdown conversion, timestamp milliseconds, enrichment exposure, and file contracts.

Risk and blast radius:

- Public metadata is a stable API and generated artifact contract.
- Raw source/file URL exposure must stay tied to redistributability and archived-feed policy.
- Geo artifact discovery must remain config-driven and must not hardcode provider names.
- Merge composition must continue using the passed resolver and `enableAll` option.
- No public serving, scheduler, downloader, install script, or UI behavior should change.

Sensitive data handling plan:

- This slice uses only local source code, synthetic test fixtures, temporary paths, and local posture/coverage metrics.
- No secrets, tokens, cookies, private endpoints, customer data, or personal data are needed.
- Durable artifacts will record only file paths, metrics, validation outcomes, and sanitized command evidence.

Implementation plan:

1. Extract base bash-compatible metadata construction into a focused helper.
2. Extract config fallback fields for license and attribution.
3. Extract comparison/geo artifact discovery and redistributable URL population.
4. Extract source-config enrichment/provenance/merge fields.
5. Extract final raw-field suppression for non-redistributable or archived feeds.
6. Preserve all public metadata field values, artifact lookup behavior, and merge resolver semantics.
7. Re-run `pkg/engine` tests and coverage, `tools/archposture`, strict shuffled tests, lint/static checks, and root coverage.

Validation plan:

- Run `go test ./pkg/engine`.
- Run `go test -coverprofile=/tmp/update-ipsets-output-metadata-slice20.cover -covermode=atomic ./pkg/engine` and inspect `go tool cover -func`.
- Run `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice20.json` and confirm `buildSetMetadataFromEffectiveEntryInDirWithResolver` no longer appears as a large-function target.
- Run `go test -shuffle=on -count=3 ./pkg/engine`.
- Run `make lint`, `make staticcheck`, `make golangci-lint`, `CI=true make coverage`, and `make test-strict`.
- Run whitespace and durable-artifact forbidden-name scans over the changed files before commit.

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: update only if a repeatable metadata-builder refactor lesson is found.
- Specs: no update expected because public metadata semantics are intended to stay unchanged.
- End-user/operator docs: no update expected.
- End-user/operator skills: no update expected.
- SOW lifecycle: this SOW remains in `.agents/sow/current/`; Slice 20 results will be recorded after validation.

Open decisions:

- No new user design decision is required because the slice is behavior-preserving quality work under the previously approved quality plan.

## Slice 20 Results

Changes made:

- Moved per-feed public metadata schema and builders from `pkg/engine/output.go` into `pkg/engine/output_metadata.go`.
- Split `buildSetMetadataFromEffectiveEntryInDirWithResolver` into focused helpers for:
  - base bash-compatible metadata construction;
  - config fallback for license and attribution;
  - comparison, geo, local-copy, and GitHub artifact fields;
  - source provenance, enrichment, use-role, and processor fields;
  - merge composition fields;
  - raw source/file suppression for non-redistributable or archived feeds.
- Kept shared sitemap, robots, llms, JSON, and timestamp helpers in `pkg/engine/output.go`.
- Preserved public metadata field values, configured geo provider discovery, redistributability policy, archived-feed suppression, and merge resolver/`enableAll` behavior.

Measured result:

- Baseline: `pkg/engine/output.go:142` `(*Engine).buildSetMetadataFromEffectiveEntryInDirWithResolver` was 133 lines with complexity 27 and `98.1%` direct coverage.
- After refactor: no `pkg/engine/output*.go` function appears in `tools/archposture` large-function output.
- `pkg/engine/output.go` moved to 424 lines.
- `pkg/engine/output_metadata.go` is 364 lines.
- `buildSetMetadataFromEffectiveEntryInDirWithResolver` direct coverage moved to `100.0%`.
- `pkg/engine` coverage remains `71.3%`.
- Root coverage by `go tool cover -func=coverage.out` remains `72.6%`.
- `tools/archposture` after this slice: source files `628`, source lines `126734`, large files `50`, and large functions `30`.
- Remaining top production complexity target: `pkg/engine/critical.go:489` `(*Engine).writeCriticalInfrastructureForFeed`, 130 lines, complexity 26.

Tests or equivalent validation:

- `go test ./pkg/engine`: passed.
- `go test -coverprofile=/tmp/update-ipsets-output-metadata-slice20.cover -covermode=atomic ./pkg/engine`: passed, `71.3%`.
- `go tool cover -func=/tmp/update-ipsets-output-metadata-slice20.cover`: passed, total `71.3%`.
- `go run ./tools/archposture -root . > /tmp/update-ipsets-archposture-slice20.json`: passed.
- `go test -shuffle=on -count=3 ./pkg/engine`: passed.
- `make lint`: passed.
- `make staticcheck`: passed.
- `make golangci-lint`: passed with `0 issues`.
- `CI=true make coverage`: passed.
- `make test-strict`: passed.
- `git diff --check`: passed.
- Durable-artifact forbidden-name scan over the changed SOW and Go files found no newly added personal name, authorship, tool, or vendor-attribution text.

Artifact maintenance gate:

- AGENTS.md: no update needed.
- Runtime project skills: no update needed; no new durable process rule was found.
- Specs: no update needed; public metadata semantics are unchanged.
- End-user/operator docs: no update needed.
- End-user/operator skills: no update needed.
- SOW lifecycle: remains in `.agents/sow/current/`; Slice 20 is validated and pending PR merge.

## Outcome

First through twentieth implementation slices are complete and validated locally. The SOW remains open for the next focused coverage, complexity, or duplication slice.

## Lessons Extracted

Update project hygiene practice to always pair Codacy Cloud metrics with local actionable structural scans before changing code.

## Followup

Continue with one focused slice at a time, starting with the next measured complexity target, a high-signal duplicated UI block, or a targeted package coverage gap after a fresh pre-implementation gate update for that chosen surface.

## Regression Log

None yet.
