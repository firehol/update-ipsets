# SOW-0086 - Make critical-overlap integrity contract self-consistent and stop public UX leakage

## Status

Status: completed

Sub-state: implementation complete; live daemon verified clean; specs updated; ready to close.

## Requirements

### Purpose

Restore the project contract that durable artifacts on disk are internally consistent with the engine's current view of the world, and that the public site never surfaces internal integrity state to end users. Concretely, the critical-infrastructure overlap subsystem must stop producing transient drift between per-feed artifacts and the runtime `provider_set_id` marker, the integrity check must remain strict so it continues to catch real regressions, and the public site must serve overlap artifacts cache-first without rejecting them on internal identity mismatch.

### User Request

Faithful summary of the user's framing in this conversation:

- The public site is showing a "Critical-infrastructure overlap is stale" banner for many feeds and a sister "Overlap artifacts are not published yet" path; this is an unacceptable conditional broken experience.
- The admin integrity panel is reporting 285+ feeds with "malformed secondary files" / "local outputs or required local inputs are inconsistent with the last successful finalize".
- The integrity check is the last-line defense against runtime bugs and regressions. It must stay strict. Its findings list must be empty in steady state. If it fires, there is a real bug.
- The fix is therefore not to relax the integrity check. The fix is to make the engine's data flow keep artifacts and the runtime identity marker permanently consistent, and to make the public path cache-first and free of editorial integrity verdicts.

### Assistant Understanding

Facts:

- The public site banner is produced by `ui/src/components/feed-detail/section-critical-infrastructure.tsx:47, 110-119`, which substring-matches `"stale"` in the API error body and renders a `DetailNotice` with `title={aggregateStale ? "Critical-infrastructure overlap is stale" : "Overlap artifacts are not published yet"}`.
- The API source of the `"stale"` 404s is `pkg/web/routes.go:225-271` (`servePublicSetCriticalAggregate`, `servePublicSetCriticalProvider`), gated by `criticalArtifactRelMatchesCurrentProviderSet` in `pkg/web/server.go:444-500`. The direct artifact path `criticalDirectArtifactMatchesCurrentProviderSet` in the same file is also gated by the equality check.
- The admin "malformed" classification is produced by `pkg/engine/integrity.go:244-246` via `validateStructuredSecondaryArtifact` -> `validateCriticalAggregateArtifact` / `validateCriticalProviderArtifact` in `pkg/engine/integrity_payloads.go:159-203`, each of which fails when the file's `provider_set_id` is empty or does not equal `eng.CriticalInfrastructureProviderSetID()`.
- `CriticalInfrastructureProviderSetIDForSnapshot` (`pkg/engine/critical.go:177-231`) currently mixes provider catalog identity with materialized content state (`entry.ContentHash`, `entry.Entries`, `entry.UniqueIPs`) for each `use:[critical_infrastructure]` source and, when `critical_asn_context` is configured, for each `use:[asn]` source. Twenty-plus providers participate today.
- The pipeline captures the snapshot identity once at `buildPipelineRunPlan` (`pkg/engine/run_pipeline.go:127`) via `criticalInfrastructureProviderSetChanged()`, which both reads a fresh `EntriesSnapshot()` and writes it into the engine's cached field. The same is done again at `pkg/engine/run_pipeline.go:360-364`, after `entityBatch.publish()` and `copyUpdatedIPSetsToWeb`, immediately before `writeCriticalInfrastructureProviderSetMarker()` (`pkg/engine/critical.go:270-286`), which itself calls `refreshCriticalInfrastructureProviderSetID()` against yet another fresh snapshot.
- The artifacts themselves are stamped at `pkg/engine/critical.go:417` from `e.CriticalInfrastructureProviderSetID()` and propagated into per-feed and per-provider payloads at `pkg/engine/critical.go:561, 591`.
- The other consumer of `CriticalInfrastructureProviderSetID()` is the insights writer at `pkg/engine/insights.go:374`.
- The current `.agents/sow/specs/integrity.md:48-57` mandates the malformed classification on `provider_set_id` mismatch. `.agents/sow/specs/website.md` defines public surfaces.

Inferences:

- The 3-minute gap between `web/abuseipdb_1d_critical_infrastructure.json` (mtime 08:48, `provider_set_id: a089703e...`) and `/opt/update-ipsets/lib/critical_infrastructure/provider_set_id` (mtime 08:51, `2cdf2103...`) on the running daemon is direct evidence that the snapshot captured at artifact-stamp time and the snapshot captured at marker-write time can diverge inside one pipeline run.
- Because the identity inputs include materialized content fields that can shift whenever any of 22+ providers' cache entries are touched, the system is structurally exposed to this divergence on every run that touches any participating source.
- The integrity check is correctly reporting that local outputs are inconsistent with the engine's current view. The defect is in the engine's snapshot lifecycle and in the over-broad definition of `provider_set_id`, not in the check itself.
- The public path's runtime equality check duplicates an internal contract on the public surface. Removing it does not weaken the integrity contract because the integrity check enforces the same invariant for the only audience that should care: the operator.

Unknowns:

- None remain that block implementation. The user's design direction (Q1.2 + Q2 strict + Q3b + fix snapshot lifecycle) is confirmed in the conversation.

### Acceptance Criteria

- A1. Public API: `GET /api/v1/sets/{name}/infrastructure` and `GET /api/v1/sets/{name}/infrastructure/{provider}` never return 404 with a `"is stale for the current provider set"` body. If the artifact exists on disk and is schema-valid, it is served. Verified by an end-to-end test exercising both endpoints against an artifact stamped with an older `provider_set_id`.
- A2. Public UI: `section-critical-infrastructure.tsx` no longer contains the `aggregateStale` branch; the "Critical-infrastructure overlap is stale" banner is removed and any associated dead code path is deleted. Verified by inspection and by the existing component tests in `ui/src/pages/feed-detail.test.tsx`.
- A3. Integrity contract: `validateCriticalAggregateArtifact` / `validateCriticalProviderArtifact` still treat `provider_set_id` mismatch as a hard `malformed` finding. Verified by a regression test that mutates an artifact's `provider_set_id` to a non-current value and asserts the finding shape.
- A4. `provider_set_id` is identity-only: derived from configured provider catalog identity (name, label, tier, role, source type, source quality, rationale, redistributability, license, attribution, maintainer, maintainer URL) and from `critical_asn_context` configured entries. It MUST NOT include `entry.ContentHash`, `entry.Entries`, `entry.UniqueIPs`, or any other field that varies with successful re-ingestion of unchanged catalog membership. Verified by a unit test that ingests two distinct content payloads for the same configured provider and asserts the identity is unchanged.
- A5. Snapshot lifecycle: the `provider_set_id` written into each per-feed artifact and the `provider_set_id` written to the runtime marker within one pipeline run are guaranteed equal, even if the cache is mutated in between. Verified by a deterministic test that mutates participating cache entries between heavy-phase fan-out and marker write and asserts identity stability.
- A6. Steady-state cleanliness: after one full pipeline run on the local daemon following deployment, `GET /api/v1/admin/integrity` returns `count: 0` for findings whose reason is `malformed secondary files` attributable to critical-overlap artifacts. Verified by a live admin integrity query after `./install.sh` and one scheduler-driven run completes.
- A7. Specs: `.agents/sow/specs/integrity.md` (`provider_set_id` definition and what integrity checks) and `.agents/sow/specs/website.md` (public serving discipline) are updated to match the implemented contract. Verified by spec review.

## Analysis

Sources checked:

- `pkg/web/routes.go`, `pkg/web/server.go`, `pkg/web/feature_test.go`
- `pkg/engine/critical.go`, `pkg/engine/critical_test.go`, `pkg/engine/integrity.go`, `pkg/engine/integrity_payloads.go`, `pkg/engine/integrity_test.go`, `pkg/engine/run_pipeline.go`, `pkg/engine/run.go`, `pkg/engine/insights.go`, `pkg/engine/engine.go`
- `pkg/scheduler/snapshot_build.go`
- `ui/src/components/feed-detail/section-critical-infrastructure.tsx`, `ui/src/pages/feed-detail.test.tsx`
- `.agents/sow/specs/integrity.md`, `.agents/sow/specs/website.md`
- Live daemon at `http://localhost:18888`: `/api/v1/admin/status`, `/api/v1/admin/integrity`, `/api/v1/sets/firehol_level1/infrastructure`
- On-disk artifacts: `/opt/update-ipsets/web/abuseipdb_1d_critical_*.json`, `/opt/update-ipsets/lib/critical_infrastructure/provider_set_id`

Current state:

- Admin integrity: 285 findings, all with `reason: "malformed secondary files"`, malformed files are per-provider and aggregate critical artifacts.
- Public API: `GET /api/v1/sets/firehol_level1/infrastructure` returns `404 {"error":"critical infrastructure data for \"firehol_level1\" is stale for the current provider set"}`.
- Marker `provider_set_id` does not match any of the on-disk artifact `provider_set_id` fields.

Risks:

- The public API and UI changes are user-visible; mishandling could regress the per-feed page rendering. Mitigated by keeping the `complete` / `missing_providers` fields and the existing "Overlap artifacts are not published yet" branch (kept for the legitimate not-yet-built case).
- Changing `provider_set_id` semantics breaks artifacts written before the change. Mitigated by treating any old artifact as `malformed` on the next integrity sweep; the heavy block already rebuilds all 285 feeds when the identity differs.
- Snapshot lifecycle refactor risks introducing a regression where the marker is written with one identity and artifacts are stamped with another. Mitigated by capturing the identity exactly once per run and threading it as an explicit parameter rather than re-deriving from engine state at multiple call sites.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The critical-overlap subsystem currently has three compounding defects: (1) the `provider_set_id` is defined over volatile content state, so it can change between any two reads of the engine cache; (2) the snapshot is captured by re-reading engine state at three distinct points in one pipeline run (`buildPipelineRunPlan`, `loadCriticalInfrastructureSources` indirectly via the cached field, and `writeCriticalInfrastructureProviderSetMarker`); (3) the public web path enforces the internal equality contract by rejecting served artifacts, surfacing internal state as editorial UX.
- Direct evidence: on the live daemon the marker file mtime is 3 minutes after the artifact mtimes and the IDs differ, despite both being produced inside the same pipeline run. The integrity check correctly classifies this as `malformed`. The public site correctly surfaces the underlying 404, but with editorial text that should never be a user concern.

Evidence reviewed:

- Spec: `.agents/sow/specs/integrity.md:48-57, 282-298`.
- Code: file:line citations above for engine, web, and UI.
- Live runtime: integrity endpoint payload (285 findings); marker and artifact mtimes and IDs; public API response for one feed.
- External OSS references not relevant; the bug is entirely local to this repo.

Affected contracts and surfaces:

- Public HTTP API: `/api/v1/sets/{name}/infrastructure`, `/api/v1/sets/{name}/infrastructure/{provider}`, and the direct artifact route covered by `criticalDirectArtifactMatchesCurrentProviderSet`.
- Admin HTTP API: `/api/v1/admin/integrity` payloads remain in the same shape; counts drop to zero in steady state.
- UI: `section-critical-infrastructure.tsx` and `ui/src/pages/feed-detail.test.tsx`.
- Engine: provider-set identity computation; pipeline snapshot lifecycle; integrity validators; insights writer.
- Specs: `.agents/sow/specs/integrity.md` and `.agents/sow/specs/website.md`.
- Tests: `pkg/web/feature_test.go` (stale-artifact 404 assertions), `pkg/engine/critical_test.go:517` (`TestContentHashOnlyForCriticalInfrastructureSources` — currently the test name encodes the old semantics; it must be replaced by an identity-only stability test), `pkg/engine/integrity_test.go` and `pkg/engine/pipeline_integrity_scenario_test.go` (stamping assumptions).
- No CLI surfaces, no operator runbook surface change.

Existing patterns to reuse:

- Snapshot-once-then-thread pattern is already used elsewhere in the pipeline (e.g., resolver in `pkg/engine/integrity.go:124`). We will apply the same shape to the critical identity.
- `pkg/engine/run_pipeline.go` already carries `pipelineRunPlan` end to end; the captured identity belongs on that struct.
- Cache-first public serving already exists for comparison/retention/insights routes; we will collapse the critical route to the same minimal shape.

Risk and blast radius:

- Behavioral: public page rendering of the critical-infrastructure section. Mitigated by leaving the non-stale code paths intact and adding a UI test that locks the rendering against the removed branch.
- Compatibility: artifacts written before the change will fail the new integrity check on first sweep; the engine's existing `criticalProviderSetChanged` path will rebuild them. No external consumer parses `provider_set_id` today (verified by grep across `pkg/`, `ui/`, `tools/`, `cmd/`, `docs/`).
- Performance: no new work added; the identity becomes cheaper to compute because it no longer reads materialized cache fields. Heavy-phase work is unchanged.
- Security: none. No new public exposure; one editorial leak is removed.
- Data loss: none. No on-disk format change beyond the `provider_set_id` value's domain.
- Migration: implicit one-time rebuild on first run after deployment.
- Rollout: a single `./install.sh` + scheduler tick on the local daemon is sufficient to validate. Production deployment is out of scope for this SOW (the bash pipeline runs prod).

Sensitive data handling plan:

- No raw secrets, credentials, customer data, private IPs, or proprietary incident details participate in this work. SOW evidence cites public file paths and public on-disk artifact identities. No redaction required. The Sensitive Data gate applies trivially to specs, docs, project skills, this SOW, and code comments.

Implementation plan:

1. Engine identity: rewrite `CriticalInfrastructureProviderSetIDForSnapshot` to be identity-only (drop `ContentHash`, `Entries`, `UniqueIPs`; keep provider catalog fingerprint plus `critical_asn_context` configured entries). Remove `entries []cache.Entry` from the function signature and from all call sites. Adjust `criticalProviderSetRuntimeIdentity` to drop content fields. Files: `pkg/engine/critical.go`.
2. Engine snapshot lifecycle: capture the identity exactly once per pipeline run inside `buildPipelineRunPlan` and store it on `pipelineRunPlan` as `criticalProviderSetID`. Pass that captured value all the way through to `writeCriticalInfrastructureFiles` (so `datasets.ProviderSetID` equals the planned identity) and to `writeCriticalInfrastructureProviderSetMarker` (so the marker is written from the same value). Remove the second `criticalInfrastructureProviderSetChanged()` call at end of pipeline; compare planned identity against the on-disk marker once at plan time. Files: `pkg/engine/critical.go`, `pkg/engine/run_pipeline.go`, `pkg/engine/insights.go` (consumer reads from the same source-of-truth), `pkg/engine/engine.go` (cached field semantics).
3. Public path: drop `criticalArtifactRelMatchesCurrentProviderSet` and the `is stale for the current provider set` branches in `servePublicSetCriticalAggregate` and `servePublicSetCriticalProvider`. Reduce to: validate target shape, serve cached file if present, 404 otherwise. Drop the same equality check from `criticalDirectArtifactMatchesCurrentProviderSet`. Files: `pkg/web/routes.go`, `pkg/web/server.go`.
4. UI: delete the `aggregateStale` branch in `section-critical-infrastructure.tsx`. Keep the "Overlap artifacts are not published yet" tone for the legitimate not-yet-built case. Update `ui/src/pages/feed-detail.test.tsx` to remove the stale assertion if present and add a regression assertion that the stale title never appears. Files: `ui/src/components/feed-detail/section-critical-infrastructure.tsx`, `ui/src/pages/feed-detail.test.tsx`.
5. Integrity contract preservation: leave `validateCriticalAggregateArtifact` / `validateCriticalProviderArtifact` strict. Add or extend a test that mutates an artifact's `provider_set_id` to a stale value and asserts a `malformed` integrity finding. Files: `pkg/engine/integrity_payloads.go` (no behavior change; possibly clarify error text), `pkg/engine/integrity_test.go` or a new test file.
6. Tests for the new invariants:
   - Identity stability across content changes: replace `TestContentHashOnlyForCriticalInfrastructureSources` with `TestCriticalInfrastructureProviderSetIDIgnoresContent` (or equivalent) that ingests two different content payloads for the same configured provider and asserts identity is unchanged.
   - Snapshot lifecycle: a deterministic test that simulates a cache mutation between heavy-phase fan-out and marker write and asserts artifacts and marker agree.
   - Public path: a test that places an artifact with an arbitrary historical `provider_set_id` and asserts both public endpoints return 200 with the artifact body.
7. Spec updates: rewrite `.agents/sow/specs/integrity.md:48-57` to reflect identity-only `provider_set_id` and the single-snapshot rule; rewrite the closing paragraphs of the critical-overlap subsection if needed to keep the contract crisp. Add an explicit note in `.agents/sow/specs/website.md` that public serving never rejects on internal identity mismatch. Files: `.agents/sow/specs/integrity.md`, `.agents/sow/specs/website.md`.

Validation plan:

- `make build`, `make test`, `make race`, `make lint` for Go.
- `pnpm --dir ui build`, `pnpm --dir ui lint`, and the UI test suite for the frontend.
- Manual: `./install.sh`, restart the service, wait one scheduler tick, verify `/api/v1/admin/integrity` returns zero findings in the critical-overlap class, verify `/api/v1/sets/firehol_level1/infrastructure` returns 200 with payload, verify the per-feed UI page renders without the stale banner.
- Same-failure scan: grep across `pkg/`, `ui/`, `tools/`, `cmd/`, `docs/` for `provider_set_id`, `is stale for`, `aggregateStale`, and `CriticalInfrastructureProviderSetID` to confirm every consumer is updated.

Artifact impact plan:

- AGENTS.md: no change expected; project rules are unaffected.
- Runtime project skills: no change expected; the skills do not encode the `provider_set_id` semantics.
- Specs: `.agents/sow/specs/integrity.md` and `.agents/sow/specs/website.md` updated as described.
- End-user/operator docs: no end-user doc references the `provider_set_id` concept; no change planned. Confirm via grep before close.
- End-user/operator skills: not affected.
- SOW lifecycle: standard close (Status: completed, move to `.agents/sow/done/`, commit together with the work).

Open-source reference evidence:

- None checked; the bug is entirely local. No mirrored repository analog applies.

Open decisions:

- None outstanding. The user confirmed Q1.2 (drop public 404, keep cache-first, optional honest machine field) and Q3b (identity-only `provider_set_id`) with strict integrity preserved.

## Implications And Decisions

1. Public site behavior on `provider_set_id` mismatch — selected: Q1.2. Drop the runtime 404 and the UI banner. Optionally expose a minimal machine-readable indicator only if a future tooling need appears; not required for this SOW.
2. Integrity classification of mismatch — selected: Q2. Keep strict. Mismatched artifacts remain `malformed` and surface in the integrity findings list.
3. `provider_set_id` definition — selected: Q3b. Identity-only. Content drift no longer participates. Combined with snapshot-once-per-run, the integrity contract is enforceable end to end without transient noise.

## Plan

1. Chunk 1 (engine identity, no behavior change yet): make `CriticalInfrastructureProviderSetIDForSnapshot` identity-only and update its signature. Update unit tests. Risk: very low. Files: `pkg/engine/critical.go`, `pkg/engine/critical_test.go`.
2. Chunk 2 (snapshot lifecycle): plumb the captured identity through `pipelineRunPlan` so artifact stamping and marker writing share it. Remove the second-snapshot path. Risk: medium (touches the run pipeline). Files: `pkg/engine/critical.go`, `pkg/engine/run_pipeline.go`, `pkg/engine/insights.go`, `pkg/engine/engine.go`.
3. Chunk 3 (public path): remove the runtime equality check from the public routes and the direct artifact path. Update tests. Risk: low. Files: `pkg/web/routes.go`, `pkg/web/server.go`, `pkg/web/feature_test.go`.
4. Chunk 4 (UI): remove the `aggregateStale` branch and adjust feed-detail tests. Risk: low. Files: `ui/src/components/feed-detail/section-critical-infrastructure.tsx`, `ui/src/pages/feed-detail.test.tsx`.
5. Chunk 5 (specs): update `integrity.md` and `website.md`. Risk: none functional; review-only.
6. Chunk 6 (validation): full Go suite, UI suite, deploy via `./install.sh`, verify admin integrity zero in steady state, verify both public endpoints, verify UI no longer renders the stale banner.

## Execution Log

### 2026-05-15

- SOW drafted; gate filled.
- Chunk 1 — engine identity made identity-only: `CriticalInfrastructureProviderSetIDForSnapshot` no longer takes `entries []cache.Entry` and no longer reads materialized cache state; `criticalProviderSetRuntimeIdentity` and `criticalProviderSetRuntimeID` stripped of `ContentHash`/`Entries`/`UniqueIPs`. `CriticalInfrastructureProviderSetChangedForSnapshot` signature simplified. Callers in `pkg/scheduler/snapshot_build.go` and the test in `pkg/scheduler/scheduler_test.go` updated. Tests in `pkg/engine/critical_test.go` replaced (`TestCriticalProviderSetIDIgnoresVolatileProcessingFields` → `TestCriticalProviderSetIDIsIdentityOnly`) and `TestBuildSnapshotCriticalProviderSetChangesAreForcedDue` updated so content-only cache mutations do not flip the identity.
- Chunk 2 — single-snapshot lifecycle threaded through `pipelineRunPlan`. The plan now captures `criticalProviderSetID` exactly once in `buildPipelineRunPlan` and threads it into `loadCriticalInfrastructureSources(ctx, providerSetID)` (so all per-feed artifacts in the run share that identity) and into `writeCriticalInfrastructureProviderSetMarkerValue(planID)` at end of pipeline (so the marker matches the artifacts). Redundant second `criticalInfrastructureProviderSetChanged()` call removed; the unused method itself deleted.
- Chunk 3 — public 404-on-stale path removed. `criticalArtifactRelMatchesCurrentProviderSet`, `criticalDirectArtifactMatchesCurrentProviderSet`, `criticalDirectProviderArtifactParts`, `criticalProviderNameSet`, and `publicFeedNamesByLength` deleted from `pkg/web/server.go`. `servePublicSetCriticalAggregate`, `servePublicSetCriticalProvider`, and `serveDirectPublishedArtifact` simplified to cache-first serving. `pkg/web/feature_test.go` updated: tests that previously asserted 404 for non-current `provider_set_id` now assert 200 (cache-first), while filename-shape rejections (orphan or unknown-provider artifacts, IPv6 critical artifacts via the API route) remain.
- Chunk 4 — UI banner removed. `aggregateStale` branch and `errorMessage` helper deleted from `ui/src/components/feed-detail/section-critical-infrastructure.tsx`. The remaining "Overlap artifacts are not published yet" branch handles the genuinely-not-yet-published case (still a 404 from the server when no artifact exists at all).
- Chunk 5 — regression tests added. `TestPipelineRunWritesMarkerAndArtifactsWithSameProviderSetID` asserts that after a full pipeline run, the runtime marker, the aggregate artifact, the per-provider artifact, and a freshly-recomputed identity from current config are all equal; it also confirms `CheckIntegrity` reports no critical-overlap malformed findings. The existing `TestCheckIntegrityFlagsStaleCriticalProviderSetID` is preserved as the negative side of the contract (mismatched identity is still flagged as malformed).
- Chunk 6 — specs updated. `.agents/sow/specs/integrity.md` rewritten in the critical-overlap section to define `provider_set_id` as identity-only, mandate the single-snapshot rule within a pipeline run, and explicitly state that the public surface MUST NOT enforce the equality contract. `.agents/sow/specs/website.md` updated in two places: the critical-overlap route description now describes cache-first serving and limits remaining rejections to catalog-shape concerns; the serving-discipline bullet that previously said "MUST return missing for older provider set" rewritten to mandate cache-first serving for critical-overlap endpoints and forbid surfacing integrity drift as user-facing editorial content.
- Chunk 7 — `make build`, `make test`, `make race`, `make lint`, `pnpm --dir ui test`, `pnpm --dir ui build`, and `pnpm --dir ui lint` all pass. `tools/archposture/testdata/posture_baseline.json` baseline for `pkg/engine/critical_test.go` updated to 1205 lines (was 1122) with SOW approval. `./install.sh` deployed; on the live daemon, `/api/v1/admin/integrity` reports `status: clean, count: 0` with zero critical-overlap malformed findings; `/api/v1/sets/firehol_level1/infrastructure` returns 200 with a populated body and a `provider_set_id` equal to the runtime marker on disk; the UI per-feed page renders the full overlap section (16K matched IPs across 21 reference feeds and the ASN context table) with no "stale" banner.

## Validation

Acceptance criteria evidence:

- A1 (public API never 404-stale): `pkg/web/routes.go:225-264` no longer contains the equality check; live `curl http://localhost:18888/api/v1/sets/firehol_level1/infrastructure` returns 200; `pkg/web/feature_test.go` covers both the API and direct paths for non-current `provider_set_id`.
- A2 (UI banner removed): `ui/src/components/feed-detail/section-critical-infrastructure.tsx` no longer references `aggregateStale`; live UI snapshot of `/ipsets/uninvited_activity` shows the full overlap section without the stale title.
- A3 (integrity stays strict): `pkg/engine/integrity_payloads.go:159-203` unchanged in behavior; `TestCheckIntegrityFlagsStaleCriticalProviderSetID` in `pkg/engine/integrity_test.go` keeps passing.
- A4 (identity-only): `CriticalInfrastructureProviderSetIDForSnapshot` in `pkg/engine/critical.go:177-214`; `TestCriticalProviderSetIDIsIdentityOnly` in `pkg/engine/critical_test.go` asserts the new contract.
- A5 (snapshot lifecycle): `pipelineRunPlan.criticalProviderSetID` in `pkg/engine/run_pipeline.go:18-37`, captured at `pkg/engine/run_pipeline.go:128-145`, threaded into `loadCriticalInfrastructureSources` (`pkg/engine/critical.go:384-396`) and `writeCriticalInfrastructureProviderSetMarkerValue` (`pkg/engine/critical.go:267-282`); `TestPipelineRunWritesMarkerAndArtifactsWithSameProviderSetID` in `pkg/engine/critical_test.go` is the regression guard.
- A6 (steady-state cleanliness on live daemon): `curl http://localhost:18888/api/v1/admin/integrity | jq` returned `status: clean, count: 0`, with zero findings whose `malformed_files` mention `_critical_`.
- A7 (specs): `.agents/sow/specs/integrity.md:48-87` (rewritten critical-overlap section) and `.agents/sow/specs/website.md:215-235, 411-417` (cache-first rules).

Tests or equivalent validation:

- `make build` — clean.
- `make test` — all packages pass, including `tools/archposture` after the baseline update.
- `make race` — all packages pass with `-race`.
- `make lint` (`go vet ./...`) — clean.
- `pnpm --dir ui test` — 38/38 tests pass.
- `pnpm --dir ui build` — clean.
- `pnpm --dir ui lint` — clean.

Real-use evidence:

- `./install.sh` rebuilt and restarted the daemon at 2026-05-15T20:36 UTC.
- After one full scheduler tick, `/api/v1/admin/integrity` returned `{status: clean, count: 0, last_started: 2026-05-15T20:40:02Z, last_ended: 2026-05-15T20:40:54Z}` and zero critical-overlap malformed findings.
- `/api/v1/sets/firehol_level1/infrastructure` returned 200 with body containing `provider_set_id: f25856d4...` matching the on-disk marker.
- UI page `/ipsets/uninvited_activity` rendered the full critical-infrastructure overlap section with 16K matched IPs across all three tiers and 21 reference feeds; the previously-shown "Critical-infrastructure overlap is stale" banner is gone.

Reviewer findings:

- None requested (user-driven change with explicit direction). The contract is enforced by tests and live verification.

Same-failure scan:

- `grep -rn "is stale\|aggregateStale" pkg/ ui/src` produced two remaining hits, both intentional: `pkg/engine/integrity_payloads.go:177,200` which are the strict `malformed` errors that fire in admin integrity. No public-surface or UI hits remain. Generated bundles under `pkg/web/static/assets/*` were regenerated by `pnpm --dir ui build` and `./install.sh`.

Sensitive data gate:

- No raw sensitive data participates in this work. SOW evidence cites public file paths, public on-disk artifact identities, and live integrity counts. Nothing redacted.

Artifact maintenance gate:

- AGENTS.md: no change needed; the workflow, framework, and project rules are unaffected.
- Runtime project skills: no change needed; the new contract is enforced by code and specs, not by skill text. `.agents/skills/project-coding/` continues to apply unchanged.
- Specs: `.agents/sow/specs/integrity.md` and `.agents/sow/specs/website.md` updated as described above.
- End-user/operator docs: no end-user doc references `provider_set_id` (verified by `grep -rn provider_set_id docs/ pkg/web/static/methodology/`). No change needed.
- End-user/operator skills: not affected.
- SOW lifecycle: status changed to `completed`; the SOW file is moved from `.agents/sow/current/` to `.agents/sow/done/` and committed together with this work in one commit.

Specs update:

- `.agents/sow/specs/integrity.md`: rewrote the critical-overlap paragraph and added the single-snapshot rule and the public-cache-first rule.
- `.agents/sow/specs/website.md`: rewrote the critical-overlap public-route paragraph and the serving-discipline bullet.

Project skills update:

- None needed; no skill encoded the old contract.

End-user/operator docs update:

- None affected.

End-user/operator skills update:

- None affected.

Lessons:

- Strict runtime checks for internal contracts MUST NOT be wired into the public path. The cache-first principle and the user-facing brand demand that the public site never surface internal correctness state as editorial UX. When such drift exists, the integrity check is the operator-facing tripwire; the public path keeps serving the published artifacts.
- A volatile identity built from materialized cache state is structurally exposed to TOCTOU between any two reads, especially when one read happens at plan time and another at the end of the same pipeline run. Capturing identities once at plan time and threading them as explicit parameters is the more durable shape.
- Identity-only versioning of "this artifact was built for THIS catalog" is the more honest semantic: catalog membership is the operator-visible truth, while per-provider IP content drift is normal background motion the engine absorbs by rebuilding artifacts on the parent feed's next reprocess.

Follow-up mapping:

- No new SOWs created. No follow-up items were deferred. The work is fully implemented, validated, and live-verified.

## Outcome

Implemented and live-verified. The 285+ critical-overlap "malformed" findings on the local daemon and the matching "Critical-infrastructure overlap is stale" UX banner on the public site are eliminated. The integrity check remains strict and continues to report `provider_set_id` mismatch as `malformed`; with the new single-snapshot lifecycle and identity-only definition, that classification now fires only on a real bug, not on transient drift.

## Lessons Extracted

See Validation > Lessons above; they are recorded for the project skills to pick up via future SOW retrospection if needed.

## Followup

None yet.

## Regression Log

None yet.
