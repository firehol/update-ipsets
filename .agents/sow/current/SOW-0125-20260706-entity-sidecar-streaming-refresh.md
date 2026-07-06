# SOW-0125 - Entity Sidecar Streaming Refresh

## Status

Status: in-progress

Sub-state: implementation complete locally; broad validation passed; pending production validation

## Requirements

### Purpose

Stabilize entity artifact refresh under production memory pressure by removing the need to keep every feed entity sidecar JSON decoded in memory while rebuilding affected country and ASN artifacts.

### User Request

The user asked to create an SOW and implement the fix after production memory evidence showed entity refresh for a large changed-feed batch pushing the service close to its 3 GiB cgroup memory limit.

### Assistant Understanding

Facts:

- Production memory samples showed total cgroup memory peaking near the 3 GiB hard limit during entity refresh.
- The current production-facing runtime already limits ingest/background workers to one, so the next bottleneck is the memory shape of a single admitted entity refresh.
- The ordinary surgical refresh path loads all committed feed entity sidecars into a map, overlays changed feed sidecars, then scans that map to rebuild affected country/ASN details.
- The selected entity repair and full entity rebuild paths also use all-sidecar maps to rebuild details, indexes, and presence indexes.
- Feed history and retention data are not touched by this work.

Inferences:

- The root cause is bulk in-memory sidecar aggregation/materialization, not raw feed comparison and not high worker concurrency.
- The entity refresh algorithm needs logical access to all relevant sidecars, but it does not need all decoded sidecars resident in heap at the same time.
- Streaming one decoded sidecar at a time into bounded builders preserves the public/private artifact contract while reducing peak heap residency.

Unknowns:

- Exact production heap savings depend on real sidecar sizes and affected country/ASN fan-out, but the current full-sidecar map residency can be removed deterministically.

### Acceptance Criteria

- Ordinary feed-update entity refresh no longer calls the full `loadAllFeedEntitySidecarsWithRuntime` map path to rebuild affected detail artifacts or the feed-presence index.
- Selected entity repair no longer loads all feed entity sidecars into one decoded map when only selected country/ASN details or indexes are repaired.
- Full entity rebuild no longer needs a decoded all-sidecar map for detail aggregation, country index, ASN index, or feed-presence index.
- Generated country/ASN detail sidecars, public detail payloads, country/ASN indexes, and feed-presence index preserve the existing semantic contract.
- Tests cover streaming detail aggregation, streaming index aggregation, feed-presence index generation, and a regression guard proving the selected builder path does not require preloaded all-sidecar maps.
- Memory-management spec records that entity sidecar scans must stream/bound decoded sidecars.

## Analysis

Sources checked:

- Production memory timeline supplied by the user.
- `configs/firehol/runtime.yaml`
- `pkg/engine/runtime.go`
- `pkg/engine/entity_surgical_refresh.go`
- `pkg/engine/entity_artifacts_write.go`
- `pkg/engine/entity_artifact_selected_repair.go`
- `pkg/engine/entity_detail_selection.go`
- `pkg/engine/entity_feed_sidecar.go`
- `pkg/engine/entity_feed_presence_index.go`
- `.agents/sow/specs/memory-management.md`
- SOW-0123 and SOW-0124.

Current state:

- `loadAllFeedEntitySidecarsWithRuntime` decodes every committed feed sidecar into `map[string]*feedEntitySidecar`.
- `entitySurgicalRefreshState.loadMergedFeedSidecars` overlays pending changes into that full map.
- `buildSelectedEntityDetailSidecarsFromFeedSidecars` stores the full map on the selected-detail builder and scans it.
- `entityArtifactWriteState` keeps `liveSidecars`, `newSidecars`, and `allSidecars` maps during rebuild-style paths.
- Selected repair loads the full sidecar map before detail and index repair.

Risks:

- Reordering generated rows would create noisy diffs and could affect integrity; detail builders sort their output rows, and streaming helpers must keep deterministic name handling.
- Index and presence index generation must include staged new sidecars and exclude deleted sidecars.
- Full rebuild still has to hold generated pending sidecars for target feeds; this SOW removes the additional committed-all-sidecar decoded map and all-sidecar aggregate map where feasible.
- This work does not change systemd memory limits. It reduces heap pressure inside the admitted work.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- Entity refresh must rebuild country/ASN artifacts from feed entity sidecar contributions.
- The current implementation chooses a full decoded map of all feed sidecars as the input abstraction.
- That map is convenient but memory-expensive; the rebuild only needs to visit each sidecar contribution and add matching rows to bounded builders.
- Production memory evidence shows the current shape leaves too little headroom under the managed 3 GiB memory limit.

Evidence reviewed:

- `configs/firehol/runtime.yaml:81` and `configs/firehol/runtime.yaml:91` set single-worker ingest/background defaults.
- `pkg/engine/runtime.go:263` clamps heavy/background/engine-lane workers under `MaxIngestWorkers`.
- `pkg/engine/entity_surgical_refresh.go:216` loads all committed feed entity sidecars.
- `pkg/engine/entity_surgical_refresh.go:240` builds selected details from the full map.
- `pkg/engine/entity_artifact_selected_repair.go:40` loads all sidecars for selected repair.
- `pkg/engine/entity_artifacts_write.go:138` loads all live sidecars for rebuild-style paths.
- `pkg/engine/entity_detail_selection.go:70` scans the full map through sorted names.
- `pkg/engine/entity_feed_presence_index.go:150` builds presence names from a full map.

Affected contracts and surfaces:

- Private entity feed sidecars, country detail sidecars, and ASN detail sidecars.
- Public country/ASN detail payloads, country/ASN index JSON, sitemap dependencies, and feed-presence index.
- Admin/operator progress visibility for entity refresh and repair.
- Memory-management spec and project coding guardrails.

Existing patterns to reuse:

- `sortedJSONFiles` for deterministic sidecar file walking.
- `feedEntitySidecarHasCountries`, `feedEntitySidecarHasASNs`, and `indexFeedEntitySidecar` for contribution filtering.
- Existing detail builders and index payload builders for semantic output.
- Staged publish batches and explicit logical mtimes.

Risk and blast radius:

- Scope is contained to entity artifact aggregation paths.
- History, retention, source downloads, raw feed bodies, and `pkg/iprange` are not touched.
- Behavior must remain semantically identical; tests will compare streaming results against existing map-based output for representative cases.
- Memory improvement is structural, but production peak memory also includes kernel file cache and slab; this SOW does not claim to eliminate all cgroup pressure.

Sensitive data handling plan:

- Durable artifacts will include only code paths, sanitized production memory totals, and no raw production feed content.
- No secrets, credentials, bearer tokens, SNMP communities, customer names, personal data, private endpoints, or customer-identifying non-private IPs will be written to SOWs, specs, docs, skills, or code comments.

Implementation plan:

1. Add streaming sidecar walk helpers that can visit committed sidecars plus staged replacements/deletions without building one decoded all-sidecar map.
2. Add streaming detail/index/presence builders that consume one sidecar at a time and produce the existing payload types.
3. Convert ordinary surgical refresh, selected repair, and rebuild-style entity artifact paths to use the streaming helpers for detail aggregation, indexes, and presence index.
4. Keep generated artifact writes and logical mtimes unchanged.
5. Add regression tests and update memory-management/project coding rules.

Validation plan:

- Targeted engine tests for streaming sidecar detail/index/presence behavior.
- Targeted tests around entity surgical refresh if existing fixtures allow this cheaply.
- `go test -count=1 ./pkg/engine`
- `go test ./tools/archposture -count=1`
- Broader `make test` and `make build` if targeted tests pass.
- Same-failure scan for remaining `loadAllFeedEntitySidecarsWithRuntime` callers in entity refresh/repair hot paths.

Artifact impact plan:

- AGENTS.md: no update expected; existing memory and SOW guardrails already cover the principle.
- Runtime project skills: update `project-coding` if a durable rule is needed to prevent full sidecar-map reintroduction.
- Specs: update `memory-management.md`.
- End-user/operator docs: no update expected; behavior and configuration are unchanged.
- End-user/operator skills: no update expected.
- SOW lifecycle: this SOW stays current until validated; closure will move it to `done` with the implementation commit when requested/approved.

Open-source reference evidence:

- None. This is a local data-shape regression in project-specific entity artifacts; external implementations are not needed for the focused fix.

Open decisions:

- None. The user approved the long-term-best fix: replace full decoded sidecar-map aggregation with streaming/bounded aggregation.

## Implications And Decisions

1. Implementation direction
   - Selected: long-term-best streaming/bounded sidecar aggregation.
   - Reasoning: worker limits are already one; reducing worker count cannot fix the heap shape of one entity refresh.
   - Risk: touches several entity artifact paths, so semantic regression tests are mandatory.

2. Artifact compatibility
   - Selected: preserve JSON semantics and deterministic sorted output, not byte-for-byte implementation internals.
   - Reasoning: generated entity artifacts are contract JSON; sorting already defines stable row order.
   - Risk: any accidental row ordering drift would create noisy diffs, so tests compare payloads structurally.

3. Scope
   - Selected: include ordinary refresh, selected repair, and rebuild-style detail/index/presence aggregation.
   - Reasoning: leaving another full-map path would recreate the same memory shape during integrity repair or full rebuild fallback.
   - Risk: broader than a one-call-site patch, but still within entity artifact aggregation.

## Plan

1. Implement streaming sidecar scan/build helpers.
2. Convert surgical refresh.
3. Convert selected repair and rebuild-style aggregation.
4. Update memory spec and project coding guardrail.
5. Add tests and run validation.

## Execution Log

### 2026-07-06

- Created SOW after production memory timeline showed entity refresh peak cgroup memory near the managed hard limit.
- Added streaming entity sidecar walkers and streaming aggregation helpers in `pkg/engine/entity_feed_sidecar_stream.go`.
- Converted ordinary surgical entity refresh to rebuild affected details and the feed-presence index from a merged streaming sidecar view instead of a decoded all-sidecar map.
- Converted selected entity repair to stream committed sidecars for selected details and optional index repair.
- Converted rebuild-style entity artifact writing to load old committed sidecars only as needed for target comparison or names-only stale deletion, and to stream the current sidecar view for details, indexes, and feed-presence index.
- Added regression tests comparing streaming details, indexes, and feed-presence names with the existing map-based helpers, plus a replacement/deletion test that proves replaced committed files are not decoded.
- Updated the memory-management spec and project coding skill with the streaming sidecar aggregation rule.

## Validation

Acceptance criteria evidence:

- Streaming sidecar walk helpers exist at `pkg/engine/entity_feed_sidecar_stream.go:13` and `pkg/engine/entity_feed_sidecar_stream.go:41`.
- Streaming detail aggregation helper exists at `pkg/engine/entity_feed_sidecar_stream.go:142`.
- Streaming feed-presence helper exists at `pkg/engine/entity_feed_sidecar_stream.go:175`.
- Ordinary surgical refresh uses the streaming merged view at `pkg/engine/entity_surgical_refresh.go:223`, `pkg/engine/entity_surgical_refresh.go:228`, and `pkg/engine/entity_surgical_refresh.go:389`.
- Selected entity repair no longer loads all sidecars before detail/index repair; it calls streaming detail/index paths at `pkg/engine/entity_artifact_selected_repair.go:41`, `pkg/engine/entity_artifact_selected_repair.go:83`, and `pkg/engine/entity_artifact_selected_repair.go:211`.
- Rebuild-style artifact writing no longer stores `allSidecars`; it loads target old sidecars or names only at `pkg/engine/entity_artifacts_write.go:137`, walks current sidecars at `pkg/engine/entity_artifacts_write.go:259`, and streams details/indexes/presence at `pkg/engine/entity_artifacts_write.go:376`, `pkg/engine/entity_artifacts_write.go:486`, and `pkg/engine/entity_artifacts_write.go:532`.
- Memory-management spec records the contract at `.agents/sow/specs/memory-management.md:171`.
- Project coding guardrail records the implementation rule at `.agents/skills/project-coding/SKILL.md:70`.

Tests or equivalent validation:

- `go test -count=1 ./pkg/engine -run 'TestStreaming|TestMergedSidecarWalker|TestEntity'` passed.
- `go test -count=1 ./pkg/engine -run 'TestStreaming|TestMergedSidecarWalker'` passed.
- `go test -count=1 ./pkg/engine` passed.
- `go test ./tools/archposture -count=1` passed.
- `make test` passed.
- `make build` passed.
- `make lint` passed.
- `staticcheck -checks=inherit,-U1000 ./pkg/engine` passed.
- `git diff --check` passed.
- Regression tests:
  - `pkg/engine/entity_feed_sidecar_stream_test.go:12`
  - `pkg/engine/entity_feed_sidecar_stream_test.go:40`
  - `pkg/engine/entity_feed_sidecar_stream_test.go:64`
  - `pkg/engine/entity_feed_sidecar_stream_test.go:81`

Real-use evidence:

- Not installed or observed in production yet in this SOW turn.

Reviewer findings:

- External reviewers were not run in this turn. The user did not request them for this focused implementation chunk.

Same-failure scan:

- `rg -n "loadAllFeedEntitySidecarsWithRuntime|buildSelectedEntityDetailSidecarsFromFeedSidecars|entityFeedPresenceNamesFromSidecars|buildCountryIndexFromFeedSidecarsWithSnapshot|buildASNIndexFromFeedSidecarsWithSnapshot|allSidecars" pkg/engine` shows only legacy helper definitions and the new regression tests. Production refresh/repair/rebuild callers no longer reference those full-map helpers.
- Residual risk: streaming aggregation still retains builders for the selected affected country/ASN output set during a pass. This is smaller than retaining every decoded feed sidecar, but very large affected-entity sets may still justify future batching if production memory shows another spike.

Sensitive data gate:

- Passed. Durable artifacts include only code paths, sanitized production memory totals, and generic runtime behavior. No raw secrets, credentials, bearer tokens, SNMP communities, customer names, personal data, private endpoints, or customer-identifying non-private IPs were added.

Artifact maintenance gate:

- AGENTS.md: no update needed; existing memory/SOW guardrails already cover the principle.
- Runtime project skills: updated `.agents/skills/project-coding/SKILL.md`.
- Specs: updated `.agents/sow/specs/memory-management.md`.
- End-user/operator docs: no update needed; behavior and configuration are unchanged.
- End-user/operator skills: no update needed.
- SOW lifecycle: remains in `.agents/sow/current/` with `Status: in-progress` until commit/production validation or explicit close.

Specs update:

- Updated `.agents/sow/specs/memory-management.md`.

Project skills update:

- Updated `.agents/skills/project-coding/SKILL.md`.

End-user/operator docs update:

- No update needed; this is an internal memory-shape fix with no operator-facing option or API change.

End-user/operator skills update:

- No update needed.

Lessons:

- Entity artifact aggregation must distinguish logical access to all sidecars from heap residency of all decoded sidecars. A streaming scan preserves correctness without retaining the full decoded store.

Follow-up mapping:

- No new mandatory follow-up. If production still shows entity-refresh anon spikes after this change, the next focused work should batch selected country/ASN output builders so a very large affected-entity set is emitted in bounded chunks.

## Outcome

Implemented locally and validated. Pending production validation.

## Lessons Extracted

Do not use decoded all-sidecar maps as the default abstraction for entity artifact aggregation. Use a streaming sidecar walker and only retain the bounded output state required for the selected artifact set.

## Followup

None yet.

## Regression Log

None yet.
