# SOW-0102 - Quality Complexity Duplication Coverage

## Status

Status: in-progress

Sub-state: ninth implementation slice selected; surgical entity refresh complexity in progress

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
- No daemon, scheduler, public serving, admin UI, install, or runtime behavior was changed intentionally; comparison and entity artifact generation changed only by helper extraction and one behavior-focused test addition.

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
- SOW lifecycle: remains in `.agents/sow/current/`; the first nine slices are validated but broader quality work continues.

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
  - `pkg/downloader/downloader.go:112` `(*Client).Fetch`: 231 lines, complexity 48.
  - `pkg/engine/home_entity_builders.go:644` `(*Engine).buildASNDetailSidecar`: 231 lines, complexity 44.
  - `pkg/web/admin_manifest.go:134` `buildFeedManifest`: 217 lines, complexity 29.
  - `pkg/engine/home_entity_builders.go:430` `(*Engine).buildCountryDetailSidecar`: 213 lines, complexity 42.
  - `pkg/config/validate.go:274` `Validate`: 197 lines, complexity 85.
- Next measured duplication targets:
  - `ui/src/components/admin/feeds-table-header.tsx`: 130 duplicated lines.
  - `ui/src/pages/asn-detail.tsx` and `ui/src/pages/country-detail.tsx`: 20, 35, and 107 duplicated line blocks.
  - `ui/src/components/feed-detail/asn-bubble-chart.tsx` and `ui/src/components/feed-detail/asn-treemap.tsx`: 50 duplicated lines.
  - `pkg/iprange/set6_ops.go` and `pkg/iprange/set_ops.go`: 25 duplicated lines.
  - `pkg/engine/runtime_ledger_cache.go`, `pkg/engine/home_entity_precompute.go`, `pkg/engine/home_entity_builders.go`, and `pkg/engine/download_stage.go`: remaining Go duplicate blocks.

## Outcome

First through ninth implementation slices are complete and validated locally. The SOW remains open for the next focused coverage, complexity, or duplication slice.

## Lessons Extracted

Update project hygiene practice to always pair Codacy Cloud metrics with local actionable structural scans before changing code.

## Followup

Continue with one focused slice at a time, starting with either UI duplicate pages/components, the downloader fetch path, or the entity detail builders after a fresh pre-implementation gate update for that chosen surface.

## Regression Log

None yet.
