# SOW-0126 - Memory Materialization Cleanup

## Status

Status: completed

Sub-state: completed and ready for commit

## Requirements

### Purpose

Reduce production memory pressure by removing convenience materialization and caches that keep decoded or generated data in memory when the operation can stream, discard, or bound that data without changing application behavior.

### User Request

The user asked to identify and fix other application "shortcuts" that unnecessarily waste memory by loading or keeping data in memory when it is not required.

### Assistant Understanding

Facts:

- SOW-0125 removed the largest ordinary entity-refresh all-sidecar map, but remaining code paths still retain decoded or generated data for convenience.
- `entityOutputView` caches decoded country/ASN comparison JSON for the lifetime of an operation.
- Full entity rebuild style code still collects newly generated feed entity sidecars into a map before staging.
- Selected country/ASN detail builders still retain all affected entity builders during a pass.
- Comparison ledger and older history/changeset fallback paths still have full-read materialization risks, but they are not the same immediate entity-refresh memory spike.
- Historical feed history/retention data is irreplaceable and must not be rewritten, truncated, migrated, or compacted by this work.

Inferences:

- The highest-value safe first fix is to stop retaining decoded comparison JSON when callers normally read each feed/provider pair once.
- Generated sidecar collection can likely be converted to a streaming staging path, but that touches rebuild orchestration and must preserve artifact comparison, logical mtimes, and integrity semantics.
- Detail-builder batching, comparison-ledger streaming, and history fallback cleanup may need separate chunks if they require design changes beyond the first pass.

Unknowns:

- Exact production heap reduction will depend on active feeds, provider payload sizes, and affected entity fan-out. This work removes proven retained objects but does not claim a precise MiB savings before production measurement.

### Acceptance Criteria

- `entityOutputView` no longer retains decoded country/ASN comparison JSON on single-pass bulk paths unless a caller explicitly requests caching.
- Tests prove single-pass output view reads do not populate long-lived caches and still return the same country/ASN data.
- Full entity rebuild sidecar staging no longer requires retaining every newly generated feed sidecar in `entityArtifactWriteState` if this can be done without changing artifact semantics.
- If full-rebuild streaming staging is not safely completed in this SOW, the SOW records the blocker and creates a concrete pending follow-up instead of leaving it as prose.
- Specs and project skills record the durable memory-shape rule where behavior changes.
- Validation includes targeted engine tests and a same-failure scan for retained cache/materialization patterns touched by this SOW.

## Analysis

Sources checked:

- `.agents/sow/current/SOW-0125-20260706-entity-sidecar-streaming-refresh.md`
- `.agents/sow/specs/memory-management.md`
- `pkg/engine/home_entity_builders.go`
- `pkg/engine/home_aggregates.go`
- `pkg/engine/entity_feed_sidecar_build.go`
- `pkg/engine/entity_artifacts_write.go`
- `pkg/engine/entity_detail_selection.go`
- `pkg/engine/output_comparison_pair_ledger.go`
- `pkg/engine/query_history.go`
- `pkg/engine/runtime_ledger_loaders.go`
- `pkg/web/server.go`

Current state:

- `entityOutputView` owns `countryCache` and `asnCache`; decoded payloads are stored after reads.
- Most checked `entityOutputView` callers create one view for a single bulk pass and do not appear to revisit the same feed/provider pair enough to justify retaining decoded data.
- `buildFeedEntitySidecarsWithSnapshot` returns a map of generated `*feedEntitySidecar` values, and rebuild-state code stores that map as `newSidecars`.
- SOW-0125 already converted ordinary committed-sidecar scanning to streaming and left selected output-builder materialization as a known residual risk.
- Runtime history paths have streaming/tail loaders for normal chart work, while older compatibility APIs still full-read CSV files.

Risks:

- Removing caches can increase repeated disk reads if a caller actually revisits the same feed/provider pair; implementation must keep caching available for any caller that needs it.
- Streaming generated sidecar staging can affect comparison and stale deletion logic if the code relies on the generated sidecars map later.
- Entity artifact mtimes participate in integrity, so writer changes must preserve logical timestamp behavior.
- History/retention data must not be modified by this SOW.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- Some application paths use whole-object caches or maps as convenience abstractions even when the operation is single-pass.
- These retained objects increase live heap during already memory-intensive entity and metadata work.
- The immediate root cause is not wrong data; it is data lifetime that is longer than the algorithm requires.

Evidence reviewed:

- `pkg/engine/home_entity_builders.go:17` defines `entityOutputView` caches.
- `pkg/engine/home_entity_builders.go:54` and `pkg/engine/home_entity_builders.go:109` decode country/ASN JSON and retain it in cache maps.
- `pkg/engine/home_aggregates.go:140` creates a view for a bulk home aggregate pass.
- `pkg/engine/entity_feed_sidecar_build.go:48` collects generated sidecars into an output map.
- `pkg/engine/entity_artifacts_write.go:39` stores `newSidecars` on rebuild state.
- `pkg/engine/entity_detail_selection.go:215` materializes selected country/ASN detail sidecar maps.
- `pkg/engine/output_comparison_pair_ledger.go:138` reads the whole comparison pair ledger and then expands it into map state.
- `pkg/engine/query_history.go:174` and `pkg/engine/query_history.go:185` full-read and split history CSV in legacy APIs.
- `pkg/engine/runtime_ledger_loaders.go:121` provides a streaming history loader used by runtime tail/cache paths.

Affected contracts and surfaces:

- Generated homepage aggregate/entity sidecar inputs.
- Country and ASN comparison payload reading.
- Full entity rebuild private sidecar staging if implemented.
- Entity artifact integrity and logical mtimes.
- Memory-management spec and project-coding guardrails.
- No public API schema or operator configuration changes are intended.

Existing patterns to reuse:

- Streaming sidecar helpers from SOW-0125 in `pkg/engine/entity_feed_sidecar_stream.go`.
- Existing JSON read helpers in `pkg/engine/home_entity_builders.go`.
- Existing generated artifact writer and logical mtime helpers.
- Existing engine fixture and behavioral tests under `pkg/engine`.

Risk and blast radius:

- Scope is contained to engine memory shape and tests.
- Public serving, downloader scheduling, source parsing, `pkg/iprange`, and history/retention writes are out of scope.
- The largest risk is subtle entity artifact drift if streaming generated-sidecar staging is changed incorrectly.
- The fallback is to complete safe cache lifetime fixes first and track any larger staging rewrite in a separate pending SOW.

Sensitive data handling plan:

- Durable artifacts will include only sanitized code-path evidence and generic memory behavior.
- No raw production feed content, secrets, credentials, bearer tokens, SNMP communities, customer names, personal data, private endpoints, or customer-identifying non-private IPs will be written to SOWs, specs, skills, docs, or code comments.

Implementation plan:

1. Make `entityOutputView` caching opt-in or bounded so single-pass callers do not retain decoded comparison payloads by default.
2. Update callers that benefit from caching, if any are found, to request it explicitly; otherwise keep default no-cache.
3. Add tests proving no-cache behavior and unchanged country/ASN payload results.
4. Inspect and, if safe, convert full entity rebuild sidecar staging to stage generated sidecars as they complete rather than storing them all.
5. Record remaining materialization risks that require separate design work with concrete pending SOW paths.

Validation plan:

- `go test -count=1 ./pkg/engine -run 'Test.*OutputView|Test.*Entity.*Sidecar|Test.*Streaming'`
- `go test -count=1 ./pkg/engine`
- `go test ./tools/archposture -count=1`
- `git diff --check`
- Same-failure `rg` scans for `countryCache`, `asnCache`, `newSidecars`, and full-read helpers touched by this SOW.

Artifact impact plan:

- AGENTS.md: no update expected; existing memory/data-preservation guardrails apply.
- Runtime project skills: update `project-coding` if a new durable anti-materialization rule is needed.
- Specs: update `memory-management.md` if the implementation changes the memory contract.
- End-user/operator docs: no update expected; behavior and configuration are unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: keep this SOW current during implementation; if completed, move to `.agents/sow/done/` with the implementation commit.

Open-source reference evidence:

- None. This is a project-specific memory-shape cleanup based on local production observations and local code paths.

Open decisions:

- None. The user approved the recommended long-term-best direction: fix the proven memory shortcuts in priority order.

## Implications And Decisions

1. Implementation direction
   - Selected: long-term-best cleanup of retained decoded/generated state where the code can stream or discard it.
   - Reasoning: production memory pressure is from live data shape under load; preserving convenience caches works against the system's bounded-memory goal.
   - Risk: some changes touch artifact-generation internals and need behavioral tests.

2. Scope order
   - Selected: entity output view cache first, generated sidecar collection second, then residual risks only if they are safe within this SOW.
   - Reasoning: the output-view cache has clear evidence and low semantic risk; generated sidecar collection is higher value but higher blast radius.
   - Risk: lower-priority findings may become separate SOWs instead of being mixed into one risky patch.

3. History/retention handling
   - Selected: do not modify history/retention data or storage formats in this SOW.
   - Reasoning: 10+ years of production history are irreplaceable and require their own copy-on-write migration or read-only optimization plan.
   - Risk: legacy full-read history APIs may remain as tracked residual risk if not safely converted here.

## Plan

1. Implement and test no-retention entity output view behavior.
2. Evaluate generated sidecar map removal against rebuild-state dependencies.
3. Implement streaming generated sidecar staging if safe.
4. Update specs/skills for durable memory-shape rules.
5. Run targeted and broad validation; record residual risks as concrete follow-up SOWs if needed.

## Execution Log

### 2026-07-06

- Created SOW from the user's approved memory-materialization cleanup request.
- Changed `entityOutputView` so decoded country/ASN comparison JSON caching is opt-in. Default bulk callers now decode and discard instead of retaining payloads for the operation lifetime.
- Added tests proving default output views do not populate decoded-payload caches and explicit cached views still cache when requested.
- Added a visitor path for generated feed entity sidecars so full entity rebuild can stage each completed sidecar immediately instead of retaining the full generated sidecar map.
- Updated full entity rebuild state to keep only generated sidecar names plus affected country/ASN sets while streaming staged sidecars.
- Fixed an implementation regression caught by tests: the first streaming version staged sidecars, then the existing full-rebuild staging loop ran again with an empty generated map and deleted the same sidecars. Full rebuild now skips the old second loop.
- Updated the memory-management spec and project coding skill with the durable decoded-payload cache rule.

## Validation

Acceptance criteria evidence:

- Default output-view no-retention behavior: `pkg/engine/entity_output_view_test.go`.
- Explicit opt-in cache behavior: `pkg/engine/entity_output_view_test.go`.
- Full entity rebuild streaming sidecar staging without retaining the generated map: `pkg/engine/entity_artifacts_write_test.go`.
- Full entity rebuild correctness was revalidated through existing entity, integrity, health-transition, and pipeline-integrity tests in `pkg/engine`.

Tests or equivalent validation:

- `go test -count=1 ./pkg/engine -run 'TestHealthTransitionRefreshRewritesPublishedEntityDetails|TestRebuildEntityArtifactsWritesPrecomputedCountryAndASNPayloads|TestFullEntityArtifactWriteStagesGeneratedSidecarsWithoutRetainingMap|TestEntityOutputView'` - pass.
- `go test -count=1 ./pkg/engine -run 'TestEntityOutputView|TestStreaming|TestMergedSidecarWalker|TestEntityArtifact|TestEntity|TestHealthTransition|TestPipelineIntegrityScenario'` - pass.
- `go test -count=1 ./pkg/engine` - pass.
- `go test -count=1 ./tools/archposture` - pass.
- `git diff --check` - pass.
- `make test` - pass. Vite emitted existing font-resolution and chunk-size warnings during the UI build; tests and builds completed successfully.
- `make lint` - pass.
- `make staticcheck` - failed on the existing project-wide U1000 unused-code backlog. The output did not flag the new explicit cache helper; the existing backlog is already represented by pending SOW-0120 and was not mixed into this memory-shape change.

Real-use evidence:

- Not run in production in this SOW turn. The code removes retained objects from the production entity/full-rebuild path; production RSS impact must be checked after install under a large changed-feed batch.

Reviewer findings:

- No external reviewers were run; the user did not request them for this SOW milestone.

Same-failure scan:

- `rg` scan for `countryCache` and `asnCache` shows caches remain only in `entityOutputView` behind explicit cache enablement and tests.
- `rg` scan for `newSidecars` shows the generated sidecar map remains for non-full bounded feed updates only; full rebuild uses staged sidecars and generated-name tracking.
- `rg` scan for `json.MarshalIndent` in `pkg/engine` shows remaining production uses outside this SOW's entity JSON hot path: legacy tab-indented public JSON helper and a tiny recovered-artifact corruption sidecar writer. These are different contracts and were not mixed into this entity-memory cleanup.
- `rg` scan for `os.ReadFile` confirms many test reads plus known production full-read paths outside this SOW scope, including comparison ledger reads and byte-identity comparison. Those are not the entity-refresh memory spike fixed here.

Sensitive data gate:

- Passed. Durable artifacts include only code paths, generic memory-shape descriptions, and sanitized validation results. No raw production feed content, secrets, credentials, customer data, private endpoints, or customer-identifying IP data were written.

Artifact maintenance gate:

- AGENTS.md: no update needed; existing memory and history-preservation guardrails already apply.
- Runtime project skills: updated `.agents/skills/project-coding/SKILL.md` with the decoded comparison JSON cache rule.
- Specs: updated `.agents/sow/specs/memory-management.md` with the decoded-payload cache contract.
- End-user/operator docs: no update needed; no operator-visible behavior or configuration changed.
- End-user/operator skills: no update needed.
- SOW lifecycle: SOW is marked completed and moved to `.agents/sow/done/` with the implementation commit.

Specs update:

- `.agents/sow/specs/memory-management.md` updated.

Project skills update:

- `.agents/skills/project-coding/SKILL.md` updated.

End-user/operator docs update:

- None needed.

End-user/operator skills update:

- None needed.

Lessons:

- Entity rebuild tests are essential for memory-shape refactors. A small staging-control-flow mistake made full rebuild delete freshly staged sidecars, and the existing health/detail tests caught it before completion.
- Full entity rebuild can stream generated feed sidecars safely when stale-deletion logic tracks generated names instead of relying on the full generated sidecar map.

Follow-up mapping:

- Selected country/ASN detail builders still retain builders for the selected output actor set. This is explicitly allowed by the memory-management spec because it is bounded by the selected emitted output set, not by all decoded feed sidecars.
- Comparison pair ledger full-read paths remain outside this SOW. They are not part of the entity-refresh memory spike and should stay with the existing history/runtime artifact optimization work rather than being mixed into this change.
- Legacy history full-read API paths remain outside this SOW because history/retention data preservation requires a separate read-only or copy-on-write plan. Existing SOW-0116 is the closest active history-safe optimization workstream.
- Existing project-wide Staticcheck U1000 findings remain outside this SOW and are represented by pending SOW-0120.

## Outcome

Completed. Implementation, validation, durable specs/skills updates, and SOW lifecycle are committed together.

## Lessons Extracted

- Default entity comparison JSON readers must be no-retain. Any decoded payload cache needs an explicit caller and a proven repeated-read reason.

## Followup

None yet.

## Regression Log

None yet.
