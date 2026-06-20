# SOW-0107 - Retention Reconciliation Hotfix

## Status

Status: completed

Sub-state: implemented, locally validated, and ready for commit/push. Not
deployed from this session.

## Requirements

### Purpose

Restore production-safe retention reconciliation for high-churn feeds without
waiting for the broad SOW-0106 engine redesign.

The hotfix must preserve the existing retention history, cohort file format,
first-seen behavior, `retention.csv`, `retention_cohorts.csv`, and public
retention outputs. The purpose is to fix the broken processing cost shape, not
to migrate retention storage or redesign the engine.

### User Request

The user reported production retention reconciliation processing about 123,786
cohort files at about 8.4 files/second, causing the systemd watchdog to kill the
daemon before completion.

The user directed the implementation to use `iprange` CompareNext semantics for
this comparison, like the original bash implementation, because the comparison
should finish in seconds. After clarification, the accepted design is to improve
the Go `pkg/iprange` package first with a daemon-usable, file-backed CompareNext
API, then remove the engine's own retention comparison workaround and call the
package API.

### Assistant Understanding

Facts:

- The current Go retention path scans retained cohort files after removals.
- The original bash implementation first identifies affected cohorts using
  `iprange latest --compare-next @new`, then rewrites only affected cohorts.
- The sibling `iprange` binary supports binary input, directory/file-list
  loading, and `--compare-next`.
- A local synthetic benchmark with 20,000 binary cohort files completed
  `latest --compare-next @new` in 236 ms with the sibling `iprange` binary.
- The bundled Go `update-ipsets iprange` subcommand completed the same synthetic
  comparison in 627 ms.

Inferences:

- The production slowness is not inherent to set comparison. The broken part is
  the current reconciliation shape: per-cohort processing against the full
  current set instead of the bash-era affected-cohort flow.

Conclusion:

- The Go `pkg/iprange` package is incomplete for the project's retention needs:
  it has CLI-shaped `CompareNext(before, after []*IPSet)`, but no reusable
  file-backed CompareNext API over `RangeSource` / `FileSet`.
- The engine retention workaround exists because the package boundary was
  missing the needed capability. The hotfix must repair that package boundary
  first and then consume it from the engine.

### Acceptance Criteria

- Retention removals identify and rewrite only affected cohorts in the normal
  path.
- Existing retention file formats and mtimes remain compatible.
- `retention.csv`, `retention_cohorts.csv`, `retention.json`, histogram cache,
  and first-seen query behavior remain equivalent to the old semantics.
- A focused test covers unaffected cohorts staying untouched while affected
  cohorts are rewritten/deleted.
- A cost-shape test or benchmark guard demonstrates that unchanged cohorts do
  not require materialized full-current intersection work.
- Targeted Go tests and build validation pass before deployment.

## Analysis

Sources checked:

- `pkg/engine/retention_update.go`
- `pkg/engine/query.go`
- `pkg/engine/runtime_ledger_cache.go`
- `pkg/iprange/fileset.go`
- `pkg/iprange/set_ops.go`
- `pkg/iprange/cli.go`
- `pkg/iprange/cli_inputs.go`
- `firehol/firehol @ 4d68515ab2165c6067a6c87c7aa323e4f3e1673d`
  - `sbin/update-ipsets:1696`
  - `sbin/update-ipsets:1701`
  - `sbin/update-ipsets:1821`
  - `sbin/update-ipsets:1835`
  - `sbin/update-ipsets:1855`
  - `sbin/update-ipsets:1917`
- `firehol/iprange @ 72b99971df78da7b685f418d9d913a3fd5676c2b`
  - `README.md:55`
  - `README.md:56`
  - `README.md:69`
  - `README.md:168`
  - `README.md:209`
  - `src/iprange.c:1086`
  - `src/iprange.c:1088`
  - `src/iprange.c:1091`

Current state:

- `pkg/engine/retention_update.go` updates retention from the feed diff.
- When removals exist, it calls `reconcileRetentionCohorts`.
- `reconcileRetentionCohorts` walks entries in `<LibDir>/<feed>/new`.
- `reconcileRetentionCohort` opens each cohort and intersects it with the full
  current set before deciding whether to rewrite or delete that cohort.
- The query path uses retention cohort files to answer first-seen.

Risks:

- Missing an affected cohort would leave removed IPs in first-seen state.
- Incorrect cohort index updates would make current retention histograms wrong.
- A literal external-process implementation adds packaging, path, timeout,
  stderr, and process-management risks inside the daemon.
- An in-process implementation must still be validated against the sibling
  `iprange` behavior so it does not drift from the original bash semantics.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- Production shows retention reconciliation scanning 123,786 cohort files at
  about 8.4 files/second.
- The original bash pipeline did not blindly rewrite every cohort. It used
  `iprange latest --compare-next @new` to find only affected cohorts, then ran
  `--common` and `--exclude-next` only for those affected files.
- The Go rewrite currently uses a broad per-cohort scan shape after removals,
  which is the wrong algorithm for large retention histories.
- The Go `pkg/iprange` package needs a file-backed CompareNext API before the
  engine can use the right package-level abstraction.

Evidence reviewed:

- Code and reference files listed above.
- Synthetic local benchmark using generated binary cohorts only; no production
  feed payloads or sensitive data were used.

Affected contracts and surfaces:

- `pkg/iprange` exported comparison API.
- Retention cohort files under `<LibDir>/<feed>/new`.
- `retention.csv`.
- `retention_cohorts.csv`.
- `histogram`.
- `retention.json`.
- First-seen query answers.
- Admin active-operation progress and logs.
- Specs for processing-engine, files-layout, integrity, and operating
  principles if behavior documentation needs correction.

Existing patterns to reuse:

- Existing `iprange` set algebra and binary format readers/writers.
- Existing atomic `writeBinaryPath` writer and deliberate cohort mtimes.
- Existing retention cohort index loader/writer.
- Existing active-operation progress reporting.
- Original bash affected-cohort algorithm as the reference behavior.

Risk and blast radius:

- High correctness risk because retention is durable history evidence.
- Moderate implementation blast radius if scoped to `pkg/iprange`, `pkg/engine`
  retention code, and tests.
- High operational benefit because the current path can exceed watchdog limits.

Sensitive data handling plan:

- Do not write raw production payloads, public IP samples from production logs,
  private endpoints, customer names, community member names, secrets, tokens, or
  proprietary incident details into SOWs, specs, docs, skills, or code comments.
- Use only sanitized counts, code paths, command names, and synthetic benchmark
  data.

Implementation plan:

1. Add a file-backed CompareNext API to `pkg/iprange` over `RangeSource` /
   `FileSet`, while keeping the existing CLI behavior intact.
2. Add an affected-cohort reconciliation path that preserves existing file
   formats and mtime behavior.
3. Update retention output generation to derive current buckets from the
   cohort index plus affected deltas, avoiding a second full scan.
4. Keep a defensive fallback for missing or invalid cohort indexes.
5. Add focused tests for affected-only reconciliation, deletion, index updates,
   retention output equivalence, and first-seen semantics.
6. After the hotfix is implemented and validated, audit the engine for similar
   workarounds that should become `pkg/iprange` APIs instead.

Validation plan:

- `go test ./pkg/engine -run 'Retention|Reconcile'`
- `go test ./pkg/engine`
- `go test ./pkg/iprange`
- `make build`
- Optional local synthetic benchmark comparing the cost shape against the
  sibling `iprange` command.

Artifact impact plan:

- AGENTS.md: no update expected.
- Runtime project skills: no update expected.
- Specs: update only if the hotfix changes documented processing behavior.
- End-user/operator docs: no update expected unless operator-visible behavior
  or troubleshooting guidance changes.
- End-user/operator skills: no update expected.
- SOW lifecycle: this SOW remains separate from SOW-0106 and must complete or
  return to pending independently.

Open-source reference evidence:

- Listed under Sources checked using upstream repository and commit references.

Open decisions:

- None. The user approved the in-process package-first design on 2026-06-20.

## Implications And Decisions

### Decision 1 - Scope Boundary

Selection: separate surgical hotfix SOW.

Reason:

- The user explicitly requested a hotfix before the broad SOW-0106 engine
  redesign implementation.
- Blending the hotfix into SOW-0106 would make urgent production repair depend
  on a large design effort.

Implication:

- SOW-0106 remains paper-design work only.
- This SOW owns only retention reconciliation cost-shape repair.

Risk:

- The hotfix must avoid silently starting broader engine redesign work.

Recommendation classification: surgical.

### Decision 2 - Comparator Execution Model

Selection: in-process `pkg/iprange` improvement before engine changes.

Reason:

- The Go package already has the set math and file-backed readers, but its
  `CompareNext` API only accepts fully materialized `[]*IPSet`.
- The engine needs a daemon-usable API over `RangeSource` / `FileSet`, not a
  literal external child process.
- This keeps runtime packaging simple while restoring the original bash-era
  `iprange --compare-next` behavior shape.

Implication:

- `pkg/iprange` gets the missing file-backed CompareNext capability.
- Engine retention removes its own comparison workaround and calls the package
  API.
- The existing CLI CompareNext behavior must remain compatible.

Risk:

- The new API must preserve CompareNext row semantics and avoid loading all
  cohort ranges into heap.
- Engine retention must not miss affected cohorts, otherwise first-seen data can
  remain stale.

Recommendation classification: surgical.

### Decision 3 - Follow-Up Audit

Selection: audit for similar engine workarounds after this hotfix is finished
and tested.

Reason:

- The retention issue exposed a package-boundary failure: engine code
  reimplemented set comparison because `pkg/iprange` lacked the right API.
- The user wants to know whether more engine code repeats this pattern.

Implication:

- The audit happens after the production hotfix validation, so it does not
  delay the urgent repair.
- Any discovered non-trivial cleanup should become a separate SOW or a clearly
  scoped follow-up, not hidden inside this hotfix.

Risk:

- Folding unrelated cleanups into this SOW would increase production hotfix
  blast radius.

Recommendation classification: surgical.

## Plan

1. Resolve Decision 2. Completed.
2. Implement the selected affected-cohort path. Completed.
3. Add focused tests and cost-shape validation. Completed.
4. Run targeted validation and update this SOW. Completed locally.

## Execution Log

### 2026-06-20

- Created this SOW after the user rejected blending the hotfix into SOW-0106.
- Removed the hotfix material from SOW-0106 so the broad redesign remains
  paper-only.
- Recorded the user's approved package-first design: improve `pkg/iprange` with
  file-backed CompareNext semantics, then update engine retention to use it.
- Added `pkg/iprange.CompareSource` and `CompareNextSources(ctx, before,
  after)`.
- Updated retention reconciliation to call `iprange.CompareNextSources` before
  deciding whether a cohort needs materialization.
- Preserved empty-cohort cleanup behavior from the previous implementation.
- Added tests for file-backed CompareNext behavior, context cancellation,
  affected-only retention rewrite/delete behavior, first-seen correctness, and
  the no-materialization-before-unchanged-return cost shape.
- Audited the engine for similar iprange package-boundary leaks after the
  hotfix validation.

## Validation

Acceptance criteria evidence:

- `pkg/iprange` now exposes file-backed CompareNext semantics through
  `CompareNextSources`.
- Engine retention no longer owns the broad per-cohort comparison decision; it
  calls `iprange.CompareNextSources` and materializes only affected cohorts.
- `TestReconcileRetentionCohortsOnlyRewritesAffectedFiles` verifies unchanged
  cohort bytes and mtime stay untouched, affected cohorts are rewritten/deleted,
  current buckets are updated, and first-seen lookup no longer finds removed
  IPs.
- `TestRetentionReconcileUsesIPrangeCompareNextBeforeMaterializing` guards the
  cost shape so unchanged cohorts return before `collectIter` materialization.

Tests or equivalent validation:

- `go test ./pkg/iprange` passed.
- `go test ./pkg/engine -run 'Retention|Reconcile|QueryIPFirstSeen'` passed.
- `go test ./pkg/engine -run 'Retention|Reconcile' -count=3 -shuffle=on`
  passed.
- `go test ./pkg/engine` passed.
- `make build` passed.
- Synthetic performance validation with 123,786 binary cohort files:
  - C `../iprange/iprange latest --compare-next @new`: 0.97 seconds,
    about 1,009,368 KiB max RSS.
  - New Go file-backed `CompareNextSources` daemon-shaped loop: 2.03 seconds
    compare time, 2.13 seconds wall time, about 60,857 files/second, about
    31,516 KiB max RSS.
- Synthetic performance validation with 50,000 binary cohort files:
  - C `../iprange/iprange latest --compare-next @new`: 0.36-0.51 seconds
    across three runs, about 409 MiB max RSS.
  - New Go file-backed `CompareNextSources` daemon-shaped loop: 0.75-0.82
    seconds compare time across three runs, about 61k-67k files/second.

Real-use evidence:

- Not deployed to production in this session.
- No production feed payloads were copied into the repository.

Reviewer findings:

- External reviewers were not run because the user did not explicitly request
  external AI reviews for this hotfix.

Same-failure scan:

- Clear package-boundary candidate: `pkg/engine/feed_body_stage.go:230` has
  `rangeSourcesEqual`, a generic RangeSource equality helper implemented in the
  engine with two `iprange.ExcludeIter` calls. It should likely become
  `pkg/iprange` API with context and RangeSource error reporting.
- Clear package-boundary candidate: `pkg/engine/fileset_helpers.go:149` and
  `pkg/engine/fileset_helpers.go:169` define context-aware iterator
  materialization/counting helpers. `pkg/iprange` has `CountUniqueIter`, but no
  context-aware count or collect helper.
- Medium package-boundary candidate: `pkg/engine/output_comparison_helpers.go:97`
  and `pkg/engine/helpers.go:387` compute RangeSource bounds/content summaries.
  These are generic set-summary operations, although the prefix-filter details
  are currently comparison-engine-specific.
- Lower-confidence candidate: `pkg/engine/home_detail_helpers.go:25` defines a
  local `iterRangeSource` adapter. The adapter is generic, but the country/ASN
  filtering behavior built on it is engine/domain-specific.
- Not classified as a workaround: `pkg/engine/output_comparison.go:305`,
  `pkg/engine/query.go:451`, public compose, bogon, ASN, and critical paths use
  existing `iprange` iterator primitives directly for domain-specific outputs.
  They may benefit from smaller `iprange` helpers, but they are not the same
  kind of broken package-boundary failure as retention.

Sensitive data gate:

- Current SOW content uses sanitized production counts, command names, code
  paths, and synthetic benchmark data only.
- No raw production feed payloads, secrets, credentials, bearer tokens, private
  endpoints, customer names, community member names, personal data, or
  non-private customer-identifying IP addresses are recorded here.

Artifact maintenance gate:

- AGENTS.md: no update expected.
- Runtime project skills: no update expected.
- Specs: no update needed. Existing `processing-engine.md` and
  `memory-management.md` already require file-backed retention reconciliation
  and materializing only cohorts that must be rewritten.
- End-user/operator docs: no update needed; public/operator behavior and file
  formats are unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: separate current SOW; not merged into SOW-0106.

Specs update:

- No spec update needed; implementation now matches the existing retention
  memory/cost contract.

Project skills update:

- No update needed.

End-user/operator docs update:

- No update needed.

End-user/operator skills update:

- No update needed.

Lessons:

- The hotfix boundary must remain separate from the broad engine redesign.

Follow-up mapping:

- No follow-up is required to satisfy this hotfix.
- The same-failure scan identified package-boundary cleanup candidates. These
  should be handled in a separate user-approved SOW if the user wants to
  improve `pkg/iprange` beyond the retention CompareNext hotfix.

## Outcome

Completed. Implemented and locally validated; not deployed in this session.

## Lessons Extracted

- When engine code needs generic RangeSource set algebra, first improve
  `pkg/iprange`; do not add another engine-local workaround.
- Cost-shape tests are necessary for retention because correctness-only tests
  can pass while unchanged cohorts are still materialized.

## Followup

- Optional separate SOW: improve `pkg/iprange` with RangeSource equality,
  context-aware iterator collection/counting, and reusable RangeSource summary
  helpers after the production hotfix is deployed.

## Regression Log

None yet.
