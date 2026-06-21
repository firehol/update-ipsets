# SOW-0116 - History-Safe Runtime Artifact Optimization

## Status

Status: open

Sub-state: implementation in progress; comparison-pair ledger cache fixed and benchmarked; entity artifact staging/publish lock scope optimized with generation revalidation.

## Requirements

### Purpose

Make update-ipsets materially faster and lighter during startup recovery and steady-state processing while preserving all historical feed data. Ten years of feed history is critical source-of-truth data and must not be lost, truncated, compacted destructively, or migrated without a proven reversible path.

### User Request

Create a SOW for the performance findings from production monitoring. Treat history preservation as mandatory. Investigate JSON parsing/generation as a recurring hot path and determine whether replacing Go's standard `encoding/json` library with a faster library is appropriate.

### Assistant Understanding

Facts:

- Production monitoring on 2026-06-21 showed update-ipsets working but repeatedly hitting CPU, memory, and I/O-heavy paths.
- The production service uses a managed memory limit pattern with `MemoryHigh=1536M`, `MemoryMax=2G`, and `GOMEMLIMIT=1536MiB`.
- The project currently uses Go `encoding/json` broadly across engine, cache, scheduler, web API, markdown context generation, processor code, and tests.
- `pkg/iprange` must stay standalone and must not import project packages.
- Public serving must stay cache-first and cheap.
- Feed history and retention ledgers under runtime data directories are durable data, not disposable caches.

Inferences:

- A global JSON library swap may improve some encode/decode costs, but it will not fix all observed bottlenecks because several hot paths are slow by artifact shape: full-file JSON ledgers, per-ASN JSON sidecar read/patch/write loops, and broad pair matrix scans.
- Some internal artifacts should likely move away from JSON on hot paths while preserving public JSON output compatibility.
- JSON replacement is safest behind a narrow local wrapper and validated per artifact class, not as a blind repository-wide import rewrite.

Unknowns:

- Which candidate JSON library performs best on this project's actual payloads and Go version.
- Whether faster JSON libraries preserve every output contract the project relies on, including HTML escaping, map key ordering, number handling, custom marshaler behavior, invalid UTF-8 behavior, streaming decoder behavior, and exact API coverage.
- Whether internal artifact formats can be changed without affecting public API/file contracts or operator expectations.

### Acceptance Criteria

- History preservation is proven by pre/post inventory, row counts, monotonic timestamp checks, checksum manifests, and rollback-safe migration tests before any production migration is proposed.
- No implementation deletes, truncates, rewrites, compacts, or migrates history/retention ledgers in place without copy-on-write migration and explicit user approval.
- Hot JSON paths are benchmarked with project payload fixtures before and after any JSON-library or format change.
- Public JSON APIs and public artifact schemas remain byte-compatible where byte compatibility is required, and behavior-compatible where ordering/spacing are not part of the contract.
- Internal cache/artifact changes have repair/rebuild paths that do not depend on public requests and do not require losing historical data.
- Production hot paths identified in this SOW are either fixed, explicitly rejected with evidence, or split into concrete follow-up SOWs before this SOW can be completed.

## Analysis

Sources checked:

- Live production admin status and namespaced journal logs from host `iplists.aegean-bramble.ts.net` on 2026-06-21.
- `pkg/engine/background_tasks.go`
- `pkg/engine/run_pipeline.go`
- `pkg/engine/output_comparison.go`
- `pkg/engine/output_comparison_pair_ledger.go`
- `pkg/engine/entity_surgical_detail.go`
- `pkg/engine/entity_surgical_refresh.go`
- `pkg/engine/finalize.go`
- `pkg/engine/runtime_ledger_cache.go`
- `pkg/engine/retention_update.go`
- `pkg/iprange/set_ops.go`
- `go.mod`
- Official upstream JSON-library sources:
  - `https://github.com/bytedance/sonic`
  - `https://github.com/goccy/go-json`
  - `https://github.com/segmentio/encoding`
  - `https://pkg.go.dev/github.com/segmentio/encoding/json`
  - `https://github.com/go-json-experiment/jsonbench`
  - `https://github.com/velox-io/json`

Initial production state:

- Entity artifact writer lock scope is too broad:
  - Production showed `entity.writer_lock_hold` as high as `217700ms`, with total `226931ms` in one run.
  - The old `withEntityArtifactMutation` path locked `entityArtifactsMu` for the whole callback in `pkg/engine/background_tasks.go`.
  - `publishRunArtifacts` locks `entityArtifactsMu` around `entityBatch.publishWorkTotal` and `entityBatch.publishContext` in `pkg/engine/run_pipeline.go`.
  - Follow-up code review found this lock is currently more than a file-rename mutex. Background callbacks read committed sidecars, read pending sidecars, stage changed files, and publish as one serialized read-modify-publish operation.
  - `buildFeedEntityDeltaWithPresence` reads committed `entities/feeds/{feed}.json` and pending `entities/feeds-pending/{feed}.json` before patching entity details.
  - `stageFeedEntitySidecarResult` reads the committed feed sidecar and touches unchanged committed files before staging pending replacements.
  - `rewriteSelectedEntityArtifacts` loads all feed sidecars, stages selected country/ASN details and indexes, then publishes entity and web batches.
  - Narrowing the lock to publish-only without a revalidation/generation check would allow overlapping entity tasks to stage from stale sidecar state and publish last-writer-wins artifacts.
  - Implemented state: background entity rebuild, feed refresh, health refresh, selected repair, and home aggregate repair now stage outside the entity publish lock and publish through a generation-revalidated plan; stale staged plans are discarded and rebuilt.
- Source finalization, history, and retention are a steady-state bottleneck:
  - One production cycle spent `153375ms` in `sources` for 14 feeds.
  - Same cycle showed `sources.update_retention` `76250ms`, `sources.finalize` `74359ms`, and `sources.finalize.observe_history` `66398ms`.
  - Some feeds with tiny input bodies spent seconds in history/retention bookkeeping.
- Entity refresh creates JSON I/O storms:
  - One production cycle patched `26661` ASN sidecars.
  - Same cycle read about `189952125` bytes of ASN sidecar JSON, wrote about `189965090` bytes of private ASN JSON, and wrote about `343414490` bytes of public ASN JSON.
  - The code reads, decodes, patches, materializes, and writes per ASN detail artifact.
- Metadata comparison ledger is full-file JSON:
  - Production file `/opt/update-ipsets/cache/comparison-pairs-v1.json` was `24568735` bytes.
  - Metadata reads the whole JSON ledger, scans all pair combinations, and rewrites the full JSON ledger.
  - One production run showed `metadata.write_comparison_files` `166594ms`, `metadata.comparison_pair_overlap` `159631ms`, `metadata.comparison_pair_ledger_lookup` `81003`, and ledger read/write of about `24.6MB` each.
  - The ledger is conceptually reproducible because entries are derived from current comparison set metadata, content hashes, and overlap counts.
  - The current missing/corrupt-ledger fallback is not sufficient for a binary rewrite: during incremental updates, an empty ledger can cause unchanged pair misses to be skipped and only the changed-pair subset to be written back. The optimized implementation must force a full rebuild when the ledger is missing, corrupt, incompatible, or unreadable.
- `pkg/iprange.CompareNextSources` exists but currently loops over the input pair product and calls `OverlapCountIterContext` per pair. It is useful but not a true amortized many-to-many comparison engine.
- JSON-library candidates:
  - `bytedance/sonic` provides compatibility configurations including `ConfigStd`, but uses JIT/SIMD techniques, has platform support constraints, and defaults that may differ from `encoding/json`.
  - `goccy/go-json` positions itself as a drop-in replacement compatible with `encoding/json`.
  - `segmentio/encoding/json` exposes a similar API and aims at standard-library behavior, but documents missing streaming decoder support and different exact error messages.
  - `velox-io/json` is a newer high-performance candidate with an `encoding/json`-style API, pure-Go unmarshal, prebuilt native marshal acceleration for common platforms, Go 1.24+ requirement, and a zero-copy string warning that callers must not mutate/reuse input buffers after unmarshal.
  - `go-json-experiment/jsonbench` shows that speed claims must be validated against semantics: UTF-8 handling, RFC parsing behavior, deterministic map ordering, and output mutation after unmarshal errors differ across libraries.
  - Go JSON v2 benchmark work shows faster future standard-library paths, but local `go doc encoding/json/v2` and `go doc encoding/json/jsontext` did not expose a usable package in the current project toolchain.

Risks:

- Data loss risk is unacceptable. Any history/retention migration without copy-on-write and validation can destroy irreplaceable feed history.
- JSON library swap risk includes subtle compatibility differences, public artifact changes, non-portable builds, unsafe/JIT behavior, and dependency maintenance risk.
- Format migration risk includes broken integrity checks, broken repair paths, stale published artifacts, and operator confusion.
- Performance regression risk exists if faster libraries improve CPU but increase memory, allocation pressure, binary size, or GC behavior for this project's real payloads.

## Pre-Implementation Gate

Status: user-decisions-recorded; ready to move to current for benchmark and implementation planning.

Problem / root-cause model:

- The production bottleneck is not one bug. It is a set of hot runtime artifact paths that repeatedly read, decode, generate, scan, and rewrite large or numerous files.
- Some paths can benefit from faster JSON encode/decode, but several need data-shape changes:
  - global writer locks around long publish operations,
  - full-file comparison ledger JSON,
  - per-ASN JSON sidecar patching,
  - history/retention bookkeeping that repeatedly scans or refreshes old data.
- History ledgers are durable project data. They must be optimized only through non-destructive indexes, derived summaries, or copy-on-write migrations.

Evidence reviewed:

- Production log/admin metrics listed in the Analysis section.
- Code paths listed in the Analysis section.
- Official upstream JSON-library documentation listed in the Analysis section.

Affected contracts and surfaces:

- Runtime feed history under `lib/<feed>/history.csv`.
- Retention ledgers and cohort files under `lib/<feed>/retention.csv`, `lib/<feed>/retention.json`, `lib/<feed>/retention_cohorts.csv`, and `lib/<feed>/new/`.
- Internal cache file `cache/comparison-pairs-v2.bin`, with
  `cache/comparison-pairs-v1.json` as read-only upgrade input when v2 is absent.
- Private entity artifact directories for feed, country, and ASN sidecars.
- Public JSON artifacts served from the web files directory.
- Admin API metrics and live operation visibility.
- Pipeline integrity and repair behavior.
- Install/service memory behavior.
- Go module dependencies and cross-platform build behavior.
- Specs for processing engine, files layout, pipeline, integrity, operating principles, and memory management.

Existing patterns to reuse:

- Copy-on-write staged publishing via web/entity publish batches.
- `writeObservedJSONFileAt`, `writeFileAtomic`, logical mtimes, and generated artifact timestamp tracking.
- Admin active operations and local runtime counters for hot path visibility.
- Existing `pkg/iprange` streaming APIs and exact set operation semantics.
- Existing behavioral tests around retention, output comparison ledger, entity refresh, and pipeline integrity.

Risk and blast radius:

- Highest risk: accidental loss or corruption of historical feed ledgers.
- High risk: subtle public JSON compatibility changes that break users or UI.
- High risk: changing integrity semantics or mtimes in a way that causes false repair/reprocess waves.
- Medium risk: adding unsafe/JIT JSON dependencies that are not reliable across Linux/macOS/Windows and amd64/arm64.
- Medium risk: optimizing for one production payload and regressing other feed sizes.
- Low risk: changing internal cache format for disposable caches if rebuild is correct and startup fallback is safe.

Sensitive data handling plan:

- SOW/spec/docs/skills/code comments must not include raw customer data, personal data, secrets, bearer tokens, private endpoints, non-private customer-identifying IPs, or proprietary incident details.
- Production evidence will use metric names, file paths, durations, counts, byte counts, and sanitized host/service names only.
- No raw feed contents or long history excerpts will be written into durable artifacts.

Implementation plan:

1. Build a history-safe measurement and fixture harness.
   - Add long-history synthetic fixtures and, if approved separately, sanitized production-shape fixtures.
   - Add inventory/checksum tooling for history and retention artifacts.
   - Add tests proving no migration changes source history unless explicitly expected.
2. Add a project JSON strategy with broad replacement only where behavior gates pass.
   - Benchmark `encoding/json` vs selected candidates on actual project payload classes.
   - Prefer a local import/wrapper pattern that allows broad `encoding/json` replacement without scattering a library decision across the codebase.
   - Validate behavior compatibility for marshal/unmarshal, indenting, HTML escaping, key ordering, streaming needs, and error behavior.
   - Keep `encoding/json` for any path where streaming support, strict semantics, deterministic output, input-buffer lifetime, build portability, or public byte compatibility does not pass.
   - Remove JSON entirely from internal hot state where JSON is the wrong storage format.
3. Fix entity artifact writer lock scope.
   - Stage work outside `entityArtifactsMu`.
   - Hold the lock only for the minimal publish/swap section.
   - Preserve active operation visibility.
4. Replace the comparison-pair ledger shape.
   - Move from one full pretty JSON file to the fastest practical binary internal cache.
   - Cross-platform binary portability is not required for this cache.
   - Treat the ledger as disposable/reproducible: missing, corrupt, incompatible, or unreadable cache files must be ignored and force a full rebuild from canonical comparison inputs without losing public or historical data.
   - Do not let an incremental update write a partial replacement ledger after a ledger-load failure.
   - Avoid scanning all pair combinations when only a small changed set exists.
5. Reduce entity refresh JSON storms.
   - Avoid reading and writing every affected ASN JSON detail when a compact index or delta representation can be updated.
   - Keep public JSON artifacts compatible and generated from canonical internal state.
6. Optimize history and retention bookkeeping without touching source history in place.
   - Add derived indexes/summaries that accelerate reads.
   - Regenerate derived indexes from preserved history when missing or invalid.
   - Use copy-on-write for any format evolution.
   - Delete old history/retention files only after tests, copy-on-write migration validation, post-switch validation, and explicit approval.
7. Evaluate `pkg/iprange` many-to-many compare APIs.
   - Add a true amortized compare API only if benchmarks show overlap computation remains material after engine ledger fixes.

Validation plan:

- Unit and behavioral tests for history/retention invariants.
- Golden public JSON output tests before/after JSON-library trials.
- Fuzz or differential tests comparing candidate JSON output/parse behavior against `encoding/json` on project payloads.
- Benchmarks for:
  - entity ASN detail read/patch/write,
  - comparison-pair ledger load/update/write,
  - metadata pair overlap,
  - retention/history observe paths,
  - admin status under load.
- Integrity scenario tests proving generated artifacts and mtimes remain correct.
- Production-style dry-run on a copied data directory before any live deployment.
- External reviewer pass before production-grade completion.

Artifact impact plan:

- AGENTS.md: updated to add a mandatory historical feed data preservation guardrail.
- Runtime project skills: likely update `project-coding`, `project-testing`, or `project-operations` if the work creates new mandatory patterns for history-safe migrations or JSON hot path handling.
- Specs: expected updates to `.agents/sow/specs/processing-engine.md`, `.agents/sow/specs/files-layout.md`, `.agents/sow/specs/integrity.md`, `.agents/sow/specs/operating-principles.md`, and `.agents/sow/specs/memory-management.md`.
- End-user/operator docs: update only if operator-visible migration, repair, backup, or rollback steps are introduced.
- End-user/operator skills: likely unaffected unless operator docs introduce new workflows consumed outside repo work.
- SOW lifecycle: this SOW remains open until implemented or split into concrete pending SOWs for every valid remaining finding.

Open-source reference evidence:

- Official upstream JSON-library sources were checked via GitHub/pkg.go.dev/Context7. No local mirrored repository evidence was used for this SOW.

Resolved decisions:

1. JSON replacement strategy.
   - Decision: B plus C.
   - B means broad replacement of most `encoding/json` usage is allowed because JSON is used frequently across the project.
   - C means internal hot state should stop using JSON where JSON is the wrong storage format.
   - Implication: this is a long-term-best path, not a mechanical import rewrite. Every replaced surface must pass behavior tests, benchmark tests, and public-contract checks. Any path that fails compatibility keeps the standard library or gets a purpose-built internal format.
2. Candidate JSON library benchmark.
   - Decision: search current online sources and benchmark current high-performance candidates, including JIT/SIMD libraries.
   - Required starting candidates: `bytedance/sonic`, `goccy/go-json`, `segmentio/encoding/json`, `velox-io/json`, and `encoding/json` as baseline.
   - Implication: upstream benchmark claims are evidence to investigate, not proof for this project. The selected implementation must win on this project's payloads and pass compatibility gates.
3. History and retention migration.
   - Decision: copy-on-write migration and later deletion of old files is acceptable only if thoroughly tested and proven reliable.
   - Constraint: backup restore is not an acceptable normal rollback path because restoring backups can lose the newest history accumulated after the backup.
   - Implication: originals remain until post-migration validation and explicit approval. Migration must be resumable, auditable, and must not depend on public requests.
4. Comparison ledger storage.
   - Decision: if the ledger is reproducible, store it in the fastest binary way. Non-cross-platform binary format is acceptable.
   - Constraint: missing, corrupt, incompatible, or unreadable ledger files must be detected, ignored, fully rebuilt, and rewritten from canonical comparison inputs.
   - Constraint: current incremental fallback is insufficient and must be fixed before switching to binary.
   - Implication: binary is acceptable only for reproducible internal caches, not for irreplaceable history or public artifacts.

## Implications And Decisions

User decisions recorded on 2026-06-21:

- Use broad JSON replacement where compatibility gates pass, and remove JSON from internal hot state where JSON is the wrong format.
- Research current online high-performance JSON libraries, including JIT/SIMD candidates, and benchmark them on project payloads before selecting.
- Allow carefully tested copy-on-write migration and deletion of old history/retention files only after validation and explicit approval; avoid backup restore as a normal rollback strategy because newest history could be lost.
- Use the fastest binary format for reproducible comparison ledger cache data, with mandatory missing/corrupt/unreadable full-rebuild behavior.

## Plan

1. Establish history-safety harness and artifact inventory tests before code changes.
2. Benchmark JSON-library candidates and internal format alternatives on project payload fixtures.
3. Fix entity artifact writer lock scope.
4. Redesign comparison-pair ledger to avoid full pair scans and full-file JSON rewrites.
5. Redesign entity detail refresh storage to avoid per-ASN JSON patch storms.
6. Add non-destructive history/retention indexes or summaries.
7. Re-evaluate `pkg/iprange` many-to-many compare opportunities after engine artifact shape fixes.
8. Update specs, project skills, and operator docs as required.
9. Validate with unit, behavioral, benchmark, integrity, and dry-run evidence.

## Execution Log

### 2026-06-21

- Created SOW from production monitoring evidence.
- Recorded history preservation as mandatory.
- Recorded JSON-library replacement as an open design decision, not an approved implementation choice.
- Updated `AGENTS.md` with a project-wide guardrail that treats 10+ years of feed history and retention data as irreplaceable source-of-truth data.
- Recorded user decisions: broad JSON replacement plus internal non-JSON formats, online benchmark of current high-performance JSON candidates, copy-on-write history migration with explicit deletion approval, and binary reproducible comparison ledger cache.
- Online source check added `velox-io/json` to benchmark candidates and recorded semantic compatibility risks from `go-json-experiment/jsonbench`.
- Verified that comparison ledger data is conceptually reproducible, but current incremental missing-ledger fallback can write a partial ledger. Recorded full rebuild as mandatory before binary migration.
- Added failing behavioral tests proving missing, corrupt, and incompatible comparison ledgers must force full rebuild instead of sparse incremental replacement.
- Implemented `cache/comparison-pairs-v2.bin` as a compact binary internal cache with feed-name table, content hashes, common-count entries, atomic writes, full rebuild on missing/corrupt/untrusted v2 state, and read-only v1 JSON upgrade input.
- Fixed v1 cleanup: after a successful v2 binary ledger write, `cache/comparison-pairs-v1.json` is removed so stale legacy JSON cannot be reloaded if the v2 cache is later missing or untrusted.
- Added `tools/jsonbench` and `make jsonbench` to benchmark JSON candidates outside the main application dependency graph.
- Updated `.agents/sow/specs/files-layout.md`, `.agents/sow/specs/pipeline.md`, and `.agents/skills/project-testing/SKILL.md` for the v2 binary ledger and JSON benchmark target.
- Benchstat, 10 samples, 400 feeds / 79,800 pair entries:
  - binary v2 marshal: `4.171ms ±5%`, `6.228MiB/op`, `25 allocs/op`
  - binary v2 parse: `2.608ms ±7%`, `8.541MiB/op`, `402 allocs/op`
  - `encoding/json` legacy-shape marshal: `26.11ms ±11%`
  - `encoding/json` legacy-shape unmarshal: `132.4ms ±4%`, `319.2k allocs/op`
  - fastest JSON marshal candidate on this payload: `velox_io_json_default`, `4.225ms ±10%`
  - fastest safe-string JSON unmarshal candidate on this payload: `velox_io_json_safe_strings`, `19.98ms ±11%`
- Sonic JIT/SIMD default was fast on unmarshal (`16.80ms ±15%`) but used more marshal memory (`97.89MiB/op`) and its default semantics are not stdlib-compatible.
- Reviewed the entity artifact writer lock after the ledger slice. The lock protects read-modify-publish consistency, not just final publication. Recorded a design decision before changing lock scope.

Open decision for next implementation slice:

1. Entity artifact mutation serialization.
   - A. Keep the current end-to-end mutation lock for correctness and first reduce the work inside it by fixing entity JSON storms.
   - B. Introduce optimistic staging: stage outside the lock, acquire the lock, revalidate an entity artifact generation/snapshot, then publish only if still current; otherwise restage/retry.
   - C. Replace ad hoc concurrent background mutation entrypoints with a single serialized entity mutation worker that coalesces rebuild, feed refresh, health refresh, and repair work.
   - Recommendation: B is the long-term-best path if lock wait/hold remains material after JSON-storm fixes. A is the surgical path for the next small slice because it avoids stale-publish risk while attacking the measured expensive work. C is attractive but broader and should be its own SOW or explicit subdesign.

Decision recorded on 2026-06-21:

- Use the long-term-best direction for entity artifact mutation serialization.
- Implement optimistic staging outside the entity writer lock, then acquire the
  lock, revalidate that the committed entity artifact inputs are still current,
  and publish only if the staged work is still valid. If not valid, restage or
  retry instead of publishing stale data.
- Do not switch to a single serialized mutation worker in this slice unless the
  optimistic model proves insufficient; that worker redesign has broader
  scheduler/admin lifecycle implications.
- Added failing tests proving unchanged country/ASN entity detail freshness
  updates must not mutate live files during staging.
- Added staged publish-batch touch intents so proven-current entity/public
  artifacts can receive metadata-only mtime updates during the serialized
  publish step without copying or rewriting identical JSON bodies.
- Removed the old direct live-file touch helper from entity surgical I/O and
  converted surgical refresh, selected repair, full/feed repair, and provider
  sidecar staging to queued touch intents.
- Added an entity artifact generation counter and optimistic mutation publisher:
  background rebuild, feed refresh, health refresh, selected repair, and home
  aggregate repair now stage expensive work outside the entity publish lock,
  acquire the lock only for publish/sync, revalidate the generation, and discard
  stale staged batches for restaging.
- Updated the normal processing-run entity sidecar publish path to advance the
  same generation counter so background work cannot publish stale staged entity
  state over a newer foreground run.
- Added a stale-generation behavioral test proving a staged entity mutation is
  discarded and restaged when another entity mutation commits before publish.
- Updated `.agents/sow/specs/pipeline.md`, `.agents/sow/specs/files-layout.md`,
  and `.agents/sow/specs/operating-principles.md` for optimistic entity
  staging, generation revalidation, and publish-batch touch intents.

## Validation

Acceptance criteria evidence:

- Comparison ledger cache is now an internal binary cache, not full-file JSON.
- Missing, corrupt, incompatible, or unreadable v2 ledger state forces a full pair rebuild before publication.
- Existing v1 JSON ledger can seed v2 when v2 is absent, avoiding a deployment-time full rebuild when the old cache is valid.
- Successful v2 writes remove the legacy v1 JSON ledger after migration.
- No history or retention data is touched by the implemented slice.

Tests or equivalent validation:

- Pre-change focused ledger tests failed as expected when they required full rebuild on missing/corrupt/incompatible ledgers.
- `go test -run 'Test(WriteComparisonFilesUsesPairLedger|ComparisonPairLedger)' ./pkg/engine`
- `go test ./pkg/engine`
- `go test -race -run 'Test(WriteComparisonFilesUsesPairLedger|ComparisonPairLedger)' ./pkg/engine`
- Pre-change focused entity tests failed as expected when they required
  unchanged country/ASN detail touch updates to avoid live-file mutation during
  staging.
- `go test -run 'TestSurgicalRefreshStagesUnchanged' ./pkg/engine`
- `go test -run 'Test(OptimisticEntityArtifactMutation|SurgicalRefreshStagesUnchanged|StagedPublishBatchMarkedTouch|RefreshEntityArtifacts|RebuildEntityArtifacts|EntityArtifacts|EntityPublishBatchTouchesIdentical|StagedPublishBatchTouchesIdentical)' ./pkg/engine`
- `go test ./pkg/engine`
- `go test -race -run 'Test(OptimisticEntityArtifactMutation|SurgicalRefreshStagesUnchanged|RefreshEntityArtifacts|RebuildEntityArtifacts|EntityArtifacts)' ./pkg/engine`
- `go test -race ./pkg/engine`
- `go test ./...`
- `go test ./tools/archposture`
- `go test ./...` in `tools/jsonbench`
- `make jsonbench`
- `make lint`
- `make test`
- Benchmark evidence:
  - `go test -run '^$' -bench 'BenchmarkComparisonPairLedgerBinaryCodec' -benchmem -count=10 ./pkg/engine`
  - `go test -run '^$' -bench 'BenchmarkComparisonPairLedgerJSON(Marshal|Unmarshal)' -benchmem -count=10 ./...` in `tools/jsonbench`
  - `go run golang.org/x/perf/cmd/benchstat@latest ...`

Real-use evidence:

- Production monitoring evidence recorded in Analysis.

Reviewer findings:

- Pending.

Same-failure scan:

- Initial scan found broad `encoding/json` usage across engine, cache, scheduler, web, markdown, processor, tools, and tests.

Sensitive data gate:

- This SOW contains no raw secrets, credentials, bearer tokens, SNMP communities, community member names, customer names, personal data, customer-identifying IPs, private endpoints, or proprietary incident details.

Artifact maintenance gate:

- AGENTS.md: updated with historical feed data preservation guardrail.
- Runtime project skills: `.agents/skills/project-testing/SKILL.md` updated with `make jsonbench`.
- Specs: `.agents/sow/specs/files-layout.md` and `.agents/sow/specs/pipeline.md` updated for `cache/comparison-pairs-v2.bin`, v1 read-only upgrade input, and full-rebuild semantics.
- End-user/operator docs: no public/operator docs update needed; `tools/jsonbench/README.md` added for developer benchmark usage.
- End-user/operator skills: no update needed; operator workflows did not change.
- SOW lifecycle: moved from `.agents/sow/pending/` to `.agents/sow/current/`; `Status: open`.

Specs update:

- Updated files-layout and pipeline specs for comparison ledger cache behavior.

Project skills update:

- Updated project-testing with the JSON benchmark target.

End-user/operator docs update:

- No operator-visible migration, backup, or rollback behavior introduced in this slice.

End-user/operator skills update:

- No operator workflow change in this slice.

Lessons:

- History data must be treated as irreplaceable source-of-truth data, not as an optimization target that can be rewritten in place.
- Faster JSON libraries may help selected hot paths, but full-file JSON artifact designs can remain slow even with a faster codec.
- For reproducible internal caches, binary format with a domain-specific table can beat JSON libraries on both speed and allocation profile while avoiding public JSON compatibility risk.
- Candidate JSON libraries still need per-surface compatibility gates; the fastest default modes often trade away stdlib-compatible behavior or safe input-buffer lifetime.

Follow-up mapping:

- Remaining work in this SOW:
  - history-safe measurement and fixture harness for retention/history paths
  - entity refresh JSON storm redesign
  - broad JSON replacement wrapper plus compatibility/golden tests for public and operator JSON surfaces
  - non-destructive history/retention indexes or summaries
  - re-evaluation of `pkg/iprange` many-to-many compare opportunities after engine artifact shape fixes

## Outcome

Pending.

## Lessons Extracted

Pending.

## Followup

None yet.

## Regression Log

None yet.

Append regression entries here only after this SOW was completed or closed and later testing or use found broken behavior. Use a dated `## Regression - YYYY-MM-DD` heading at the end of the file. Never prepend regression content above the original SOW narrative.
