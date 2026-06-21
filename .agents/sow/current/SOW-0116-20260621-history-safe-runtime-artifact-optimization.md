# SOW-0116 - History-Safe Runtime Artifact Optimization

## Status

Status: open

Sub-state: implementation in progress; comparison-pair ledger cache fixed and benchmarked; entity artifact staging/publish lock scope optimized with generation revalidation; JSON replacement candidates re-analyzed; new-baseline startup entity rebuild overlap hotfix implemented and validated; entity feed-presence index and `pkg/iprange` batched comparison implemented and validated; bounded retention removal reconciliation and history audit harness implemented and validated.

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
- Initial finding: `pkg/iprange.CompareNextSources` existed but looped over the input pair product and called `OverlapCountIterContext` per pair. It was useful but not a true amortized many-to-many comparison engine.
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
- Re-ran the JSON replacement analysis after the compatibility-risk model was
  corrected:
  - Public artifacts promise valid JSON, not exact `encoding/json` bytes,
    numeric text, HTML escaping, or identical error strings unless a specific
    caller test proves a byte contract.
  - The valid high-risk compatibility issue is old-file/input decode behavior:
    a replacement must correctly read every project JSON artifact shape that can
    exist on disk, including legacy sidecars, retention payloads, old comparison
    ledger JSON, scheduler/cache state, and public artifacts.
  - Current tree imports `encoding/json` in 54 Go files. Excluding tests and
    `tools/jsonbench`, 26 production/tool files import it: 15 in `pkg/engine`,
    3 in `pkg/markdown`, 2 in `pkg/processor`, 2 in `tools/archposture`, and
    one each in `pkg/cache`, `pkg/insights`, `pkg/scheduler`, and `pkg/web`.
  - `pkg/engine/output.go` centralizes many public artifact writes through
    `jsonMarshalTabIndent`, while entity sidecar and entity public writes fan
    out through `writeJSONFile`, `writeJSONFileAt`,
    `writeObservedJSONFile`, and `writeObservedJSONFileAt`.
  - A temp dry-run replacing all `encoding/json` imports with
    `github.com/goccy/go-json` passed `go test ./...`.
  - A temp dry-run replacing all imports with
    `github.com/segmentio/encoding/json` passed the application packages but
    failed `tools/archposture` because `tools/archposture/go_metrics.go` uses
    `Decoder.More`, which that package does not provide.
  - `make jsonbench` on the current machine, 400 feeds / 79,800 legacy ledger
    entries, produced:
    - `encoding/json` marshal `28.854ms/op`, unmarshal `178.608ms/op`,
      unmarshal `45.849MB/op`, `319236 allocs/op`.
    - `goccy/go-json` marshal `11.633ms/op`, unmarshal `32.958ms/op`,
      unmarshal `38.108MB/op`, `79810 allocs/op`.
    - `segmentio/encoding/json` marshal `15.675ms/op`, unmarshal
      `27.976ms/op`, unmarshal `25.918MB/op`, `319221 allocs/op`.
    - `velox-io/json` default marshal `4.881ms/op`, unmarshal
      `21.276ms/op`, unmarshal `7.364MB/op`, `3 allocs/op`.
    - `velox-io/json` safe-string mode marshal `4.989ms/op`, unmarshal
      `23.092ms/op`, unmarshal `18.521MB/op`, `319202 allocs/op`.
    - Sonic default marshal `23.770ms/op` but `102.658MB/op`, and Sonic std
      marshal `33.414ms/op` with `133.318MB/op`.
  - A retained-heap probe that creates the input bytes inside the measured
    operation and releases them before GC, using the same 20.4MB
    ledger-shaped payload, showed median live heap retained by the decoded
    object:
    - `encoding/json`: `18.692MB`
    - `goccy/go-json`: `27.460MB`
    - `segmentio/encoding/json`: `18.700MB`
    - Sonic default: `31.089MB`
    - Sonic std: `22.149MB`
    - Velox default: `27.460MB`
    - Velox `WithCopyString`: `18.520MB`
  - Source inspection explains the retained-heap result:
    - `goccy/go-json` copies the whole input into an internal buffer before
      decoding, then decoded strings can reference that copied buffer.
    - `segmentio/encoding/json` copies strings and `RawMessage` by default;
      zero-copy behavior is opt-in through `DontCopyString`,
      `DontCopyRawMessage`, or `ZeroCopy`.
    - `velox-io/json` uses zero-copy strings by default and has explicit
      `WithCopyString` / `DecoderCopyString` options for retained objects.
    - Sonic `ConfigStd` enables `CopyString`, `EscapeHTML`, `SortMapKeys`, and
      validation; Sonic default is faster on some decode paths but not stdlib
      compatible and used materially more marshal memory on this payload.
  - Current recommendation for the implementation slice:
    - Do not use Sonic as the broad default.
    - Do not use Velox default for long-lived decoded objects; use Velox only
      behind a per-hot-path wrapper with copy-string mode and additional tests
      if it still wins after project payload benchmarks.
    - Use either `goccy/go-json` as the simplest global replacement candidate
      if retained heap is acceptable for the project payloads, or
      `segmentio/encoding/json` for application code while keeping/adapting
      `tools/archposture` separately.
    - Before changing production code, add behavioral compatibility tests that
      decode old and current JSON artifact shapes and golden tests that prove
      public/API artifacts stay valid and schema-compatible.
- Added project-shaped JSON compatibility tests and benchmarks to
  `tools/jsonbench`:
  - Velox safe-string compatibility passed for legacy comparison ledger,
    current feed entity sidecars, legacy feed entity sidecars, raw-message
    sidecars, ASN detail payloads, scheduler snapshots, invalid input
    rejection, retained decoded strings, and retained `json.RawMessage`.
  - Velox v0.1.4 `Marshal(..., WithStdCompat())` crashes with a segmentation
    fault on the cache-state-shaped payload. The test suite records this with a
    child-process crash reproducer so the parent test process remains safe.
  - The cache-state Velox compatibility case and cache-state Velox benchmark
    rows are skipped until the crash is resolved.
  - `make jsonbench` project-shaped payload results from 2026-06-21 showed:
    - feed entity sidecar marshal, 250 rows: stdlib `92.623us/op`; goccy
      `52.574us/op`; Segmentio `60.840us/op`; Velox safe-string
      `34.915us/op` but `110.503KB/op` versus stdlib `57.363KB/op`.
    - ASN detail marshal, 1000 feed rows: stdlib `516.785us/op`; goccy
      `292.365us/op`; Segmentio `315.937us/op`; Velox safe-string
      `122.039us/op` but `784.138KB/op` versus stdlib `394.511KB/op`.
    - cache state marshal, 1000 entries: stdlib `2.884ms/op`; goccy
      `1.918ms/op`; Segmentio `1.664ms/op`; Velox skipped because it crashes.
    - scheduler snapshot marshal, 1000 items: stdlib `696.328us/op`;
      Segmentio `205.902us/op`; Velox safe-string `82.549us/op`.
    - feed entity sidecar unmarshal, 250 rows: stdlib `541.229us/op`; goccy
      `113.610us/op`; Segmentio `219.757us/op`; Velox safe-string
      `111.260us/op`.
    - ASN detail unmarshal, 1000 feed rows: stdlib `3.049ms/op`; goccy
      `533.144us/op`; Segmentio `608.938us/op`; Velox safe-string
      `566.796us/op`.
    - cache state unmarshal, 1000 entries: stdlib `11.273ms/op`; goccy
      `3.288ms/op`; Segmentio `3.438ms/op`; Velox skipped because cache-state
      marshal crash blocks that surface.
    - scheduler snapshot unmarshal, 1000 items: stdlib `1.672ms/op`; goccy
      `487.749us/op`; Segmentio `373.192us/op`; Velox safe-string
      `363.182us/op`.
  - Updated recommendation:
    - Velox safe-string is no longer acceptable as a broad JSON replacement
      candidate in this version because one project-shaped marshal crashes.
    - Velox can only remain a selected hot-path candidate for payload classes
      with explicit compatibility tests and no cache-state/shared-state use.
    - goccy and Segmentio remain the safer broad-candidate families for the
      next implementation decision, with Segmentio still blocked from a literal
      all-files replacement by `tools/archposture` streaming API coverage.
- User decision after project-shaped tests:
  - Ignore Velox v0.1.4 for implementation because the cache-state marshal
    crash is unacceptable.
  - Do not pursue a broad JSON-library migration with goccy or Segmentio in
    this slice. Their measured wins are real on some payloads, but after the
    binary ledger fix and Velox disqualification, they do not justify the
    compatibility, dependency, and migration work as a project-wide change.
  - Keep JSON codec work available only as a targeted future optimization for
    a measured hot path where a safe candidate gives a material win and passes
    artifact-specific compatibility tests.
- Production was restarted to activate the latest changes and monitored again.
  Fresh evidence from the new baseline showed:
  - Startup integrity recovery for one feed completed successfully in about
    `271892ms`; the process was not killed during the observed run.
  - A startup full entity artifact rebuild ran concurrently with the foreground
    startup recovery run.
  - The foreground run still entered the entity phase, built feed sidecars, and
    then waited to publish entity artifacts while the full rebuild held the
    entity artifact writer path.
  - After the foreground run completed, the scheduler queued a changed-feed
    entity refresh for `99` feeds, which is the correct repair path for changes
    that happen while a full rebuild is in flight.
  - The foreground entity phase therefore duplicated work that the queued
    refresh would already repair after the full rebuild, increasing CPU, file
    I/O, memory pressure, and poor admin progress visibility.
- Rechecked the suspected `publishContext` progress bug:
  - `defer op.Add(...)` is inside the `filepath.WalkDir` callback, so the
    progress increment runs when each file callback returns, not only when the
    whole publish function returns.
  - The observed `0` progress is better explained by creating
    `publish.promote_entity_artifacts` before waiting on `entityArtifactsMu`,
    while the operation total is still unknown.
  - No progress-code change will be made for this item without a failing test
    that proves an operator-visible bug.
- Implementation decision for the new baseline hotfix:
  - While a full entity rebuild is queued or running, foreground processing
    runs must not stage or publish feed entity sidecars.
  - The foreground run must still compute and report the same
    `EntityRefreshTargets` it would have staged, so the scheduler can queue the
    existing changed-feed entity refresh after the run.
  - This is a surgical production hotfix: it removes duplicate work and writer
    lock contention without changing entity artifact schemas, public JSON
    output, historical data, or the background repair contract.
- Added failing behavioral tests for the new-baseline overlap:
  - Active full rebuild background task: provider-only processing must report
    affected entity refresh targets but must not stage pending feed sidecars.
  - Queued full rebuild before visible background-task registration:
    provider-only processing must make the same deferral.
- Implemented the deferral:
  - Full entity rebuilds now mark a queued/running flag at the rebuild API
    boundary, covering the startup window before the visible background task is
    registered.
  - Foreground full-heavy processing checks the queued/running rebuild state
    before creating an entity publish batch.
  - When a rebuild is in flight, the foreground run records the same
    role-scoped entity refresh targets and returns no entity publish batch, so
    publish has no foreground entity writer work to lock or promote.
- Updated `.agents/sow/specs/pipeline.md` and
  `.agents/sow/specs/operating-principles.md` for the active/full-rebuild
  deferral rule.
- Follow-up production monitoring on the same restarted baseline showed:
  - `sources` was no longer the main bottleneck in the latest observed
    2-feed scheduled batch: `sources.finalize.observe_history` was `1ms`
    total and `sources.update_retention` was `626ms` total. The earlier
    tens-of-seconds history observation cost appears to have been a cold
    cache/stat warm-up after the latest changes, not a recurring hot path in
    the current baseline.
  - `metadata` was the latest foreground bottleneck at `14786ms`, including
    `metadata.write_comparison_files` `11392ms` and
    `metadata.comparison_pair_overlap` `6739ms` for `493` real overlap
    computations.
  - Background entity refresh work overlapped the run and scanned entity actor
    sidecars heavily: `entity.repair_feed_scan.asn_files` `61690`,
    `entity.repair_feed_scan.asn_sidecar_read` `28405` / `108495114` bytes,
    and `entity.repair_feed_scan.country_sidecar_read` `244` /
    `16245347` bytes. This happens when a committed feed sidecar is missing
    and the engine must prove whether country/ASN actor artifacts still
    reference that feed before applying a surgical delta.

Open decision for the next implementation slice:

1. Entity feed-presence proof for surgical refresh.
   - A. Add a durable internal entity feed-presence index generated and
     published with entity artifacts, updated by rebuild/refresh paths, and
     used before falling back to full actor-sidecar scans.
     - Pros: removes the 61k-file / 100MB+ JSON scan class from ordinary small
       entity refreshes; keeps the missing-sidecar safety check; supports
       bounded repair.
     - Cons: introduces a new internal artifact, integrity rules, migration
       behavior, and tests.
   - B. Keep the current full actor-sidecar scan fallback.
     - Pros: no new format or integrity surface.
     - Cons: known production cost remains; small refreshes can scan every ASN
       sidecar when committed feed sidecars are missing.
   - Recommendation: A, long-term-best. The current scan is correct but too
     expensive for the production artifact shape.
2. Metadata comparison overlap engine.
   - A. Keep the current ledger plus per-pair `OverlapCountIterContext`.
     - Pros: already tested and now much faster than the old JSON ledger path.
     - Cons: changed-feed comparison still re-reads/re-iterates sources for
       hundreds of real pair overlaps.
   - B. Add a true `pkg/iprange` one-to-many or many-to-many comparison API,
     benchmark it against the current Go pair loop and C `iprange`, then use it
     from metadata comparison writing if it wins.
     - Pros: moves the remaining foreground comparison cost into the standalone
       optimized package where it belongs; can reuse source iteration more
       efficiently than the engine pair loop.
     - Cons: broader algorithm/API work in `pkg/iprange`; needs behavioral,
       corner-case, and performance tests before engine adoption.
   - C. Add another engine-local comparison workaround.
     - Pros: narrower local edit.
     - Cons: repeats the design mistake this SOW is removing by keeping heavy
       range logic in the engine instead of `pkg/iprange`.
   - Recommendation: B, long-term-best. The engine should not grow another
     custom range-comparison workaround.

User decision:

- Approved `1A`: add a durable internal entity feed-presence index generated
  and published with entity artifacts, updated by rebuild/refresh paths, and
  used before falling back to full actor-sidecar scans.
- Approved `2B`: add a true optimized one-to-many or many-to-many comparison
  API in `pkg/iprange`, benchmark it against the current Go pair loop and C
  `iprange`, then use it from metadata comparison writing if it wins.
- Added failing behavioral tests before implementation for:
  - full entity rebuild writing a durable feed-presence index
  - missing committed feed sidecar proof using the feed-presence index without
    scanning country/ASN actor sidecars
  - metadata comparison delegating exact candidate batches to `pkg/iprange`
    instead of using engine-local `OverlapCountIterContext` loops
  - `pkg/iprange.CompareSourcePairs` selected-pair behavior, file-backed
    repeated-left behavior, context cancellation, invalid pair rejection,
    spanning target ranges, and arbitrary pair-order differential behavior
- Implemented `lib/entities/feed-presence-v1.bin` as a small reproducible
  internal binary index generated by full entity rebuild and surgical entity
  refresh publish batches.
- Updated missing committed per-feed sidecar proof to read the feed-presence
  index first, record local engine counters for read/missing/ignored states,
  and fall back to the existing actor-sidecar scan only when the index is
  missing or untrusted.
- Implemented `pkg/iprange.CompareSourcePairs(ctx, sources, pairs)` with
  selected-pair output ordering, validation, cost-gated indexed one-to-many
  scanning for `IPSet`/`FileSet` sources, and the existing iterator fallback for
  arbitrary `RangeSource` inputs.
- Converted `CompareAllSources` and `CompareNextSources` to use
  `CompareSourcePairs`, so existing callers receive the optimized source-pair
  path without switching APIs.
- Replaced engine-local exact comparison worker loops in
  `pkg/engine/output_comparison.go` with engine-owned ledger/skip filtering plus
  one batched exact comparison call to `pkg/iprange.CompareSourcePairs`.
- During testing, the spanning-range case exposed a real cursor-heap bug where
  a pending cursor advanced past the current left range and prematurely blocked
  another pending cursor that still overlapped the current left range. Fixed the
  heap scan so it continues while any pending cursor can still overlap.
- Allocation profiling of the new file-backed comparison benchmark exposed an
  allocation storm from `container/heap` boxing `oneToManyCursor` values through
  `any`. Replaced it with a typed cursor heap, reducing
  `BenchmarkCompareNextSourcesFileSet/n=10000` from about `20015 allocs/op` and
  `2.56MB/op` to low constant allocation on this machine.
- Added an indexed single-target fast path so one-pair and one-target groups use
  the specialized overlap scanner instead of the generic one-to-many heap path;
  current `BenchmarkCompareNextSourcesFileSet/n=10000` after the retention
  dispatcher work is `195827 ns/op`, `672 B/op`, and `9 allocs/op`.
- Updated `.agents/sow/specs/files-layout.md`,
  `.agents/sow/specs/pipeline.md`, and
  `.agents/sow/specs/operating-principles.md` for the entity feed-presence index
  and batched `pkg/iprange` comparison contract.
- Added a failing behavioral test proving ordinary changed-feed entity refresh
  must rebuild affected country/ASN details from committed-plus-pending
  per-feed sidecars instead of decoding existing actor JSON sidecars. The
  pre-change path failed on intentionally malformed private country/ASN actor
  sidecars with `unexpected end of JSON input`.
- Changed surgical entity refresh so it loads the committed feed sidecar set
  once, merges the pending feed deltas, rebuilds only the affected country/ASN
  actor details from that merged feed-sidecar map, and reuses the same merged
  map to stage `feed-presence-v1.bin`.
- Removed the old actor-JSON patch implementation in
  `pkg/engine/entity_surgical_detail.go`; ordinary feed-update refresh now
  treats country/ASN actor JSON sidecars as derived outputs, not canonical
  hot-path patch inputs.
- Updated `.agents/sow/specs/files-layout.md`,
  `.agents/sow/specs/pipeline.md`,
  `.agents/sow/specs/operating-principles.md`, and
  `.agents/skills/project-coding/SKILL.md` so future work keeps per-feed
  sidecars as the canonical private contribution state for changed-feed entity
  refreshes.
- Added a bounded retention reconciliation path for removal updates:
  - `reconcileRetentionCohorts` now accumulates valid cohort files into bounded
    batches of 256 file-backed sources.
  - Each batch delegates exact current-vs-cohort overlap checks to
    `pkg/iprange.CompareSourcePairs` instead of making one `CompareNextSources`
    call per cohort file.
  - The destructive decisions remain unchanged and exact: unchanged cohorts are
    closed without rewriting, partially retained cohorts are materialized through
    `IntersectSourcesContext` and atomically rewritten with the original cohort
    timestamp, and fully removed cohorts are deleted only after exact common-IP
    count is zero.
  - The batch bound prevents opening the full historical retention directory at
    once, preserving the 10-year history data while reducing per-file compare
    overhead.
- Completed the `pkg/iprange` selected-pair dispatcher so repeated-left groups
  actually use the one-to-many scanner when the combined target range count is
  not larger than the left source. Dense repeated-left groups stay on the tight
  pair scanner because benchmarks showed the generic heap cursor path is slower
  for heavily overlapping peer sets.
- Updated `.agents/sow/specs/processing-engine.md`,
  `.agents/sow/specs/memory-management.md`,
  `.agents/sow/specs/operating-principles.md`, and
  `.agents/skills/project-coding/SKILL.md` with the retention batching contract.
- Added `tools/historyaudit`, a read-only local audit tool for pre/post history
  and retention validation:
  - walks a `lib/` directory and emits a sanitized JSON manifest
  - records relative paths, file sizes, mtimes, SHA-256 checksums, CSV row
    counts, first/last timestamps, monotonic timestamp flags, and retention
    cohort counts
  - skips hidden atomic temp files in retention cohort directories
  - fails on stat/checksum/read errors instead of treating inaccessible files as
    missing
  - does not print raw feed contents and does not modify runtime artifacts
  - includes `tools/historyaudit/README.md` with read-only usage guidance

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
- New-baseline startup entity rebuild overlap validation:
  - Pre-change focused test failed as expected:
    `go test -run 'TestProviderOnlyRunDefersEntitySidecarStagingWhileFullRebuildActive' ./pkg/engine`
  - `go test -run 'TestProviderOnlyRunDefersEntitySidecarStagingWhileFullRebuild(Active|Queued)|TestProviderOnlyRunReportsEntityRefreshTargets|TestEntityArtifactRefreshQueueCoalescesFeedNames' ./pkg/engine`
  - `go test ./pkg/engine`
  - `go test -race -run 'TestProviderOnlyRunDefersEntitySidecarStagingWhileFullRebuild(Active|Queued)|TestProviderOnlyRunReportsEntityRefreshTargets|TestEntityArtifactRefreshQueueCoalescesFeedNames' ./pkg/engine`
  - `go test ./tools/archposture`
  - `make test`
  - `make lint`
- Entity refresh JSON storm validation:
  - Pre-change focused test failed as expected because surgical refresh tried
    to decode malformed private country/ASN actor JSON sidecars:
    `go test -run 'TestSurgicalRefreshRebuildsAffectedDetailsFromFeedSidecars' ./pkg/engine`
  - Post-change focused validation:
    `go test -run 'TestSurgicalRefreshRebuildsAffectedDetailsFromFeedSidecars|TestSurgicalRefresh|TestBuildFeedEntityDelta|TestRebuildEntityArtifactsWritesFeedPresenceIndex|TestMissingCommittedFeedSidecarUsesPresenceIndexForFullRebuildProof' ./pkg/engine`
  - Feed-sidecar indexing validation:
    `go test -run 'Test(FeedEntitySidecarTargetDetection|IndexFeedEntityJointSidecarProvidesConstantTimePatchRows|LoadFeedEntitySidecarAcceptsLegacyMembershipArrays)' ./pkg/engine`
  - `go test ./pkg/engine`
  - `go test ./tools/archposture`
  - `go test -count=1 ./pkg/engine ./tools/archposture`
  - `go test -race -run 'TestSurgicalRefreshRebuildsAffectedDetailsFromFeedSidecars|TestBuildFeedEntityDelta|TestMissingCommittedFeedSidecarUsesPresenceIndexForFullRebuildProof' ./pkg/engine`
  - `make test`
  - `make lint`
  - The new behavioral test asserts zero
    `entity.refresh.country_sidecar_read` and zero
    `entity.refresh.asn_sidecar_read` counters during changed-feed entity
    refresh, proving ordinary affected actor rebuild no longer performs one
    actor JSON decode per affected country/ASN.
- Retention removal reconciliation validation:
  - `go test -run 'TestCompareSourcePairs' ./pkg/iprange`
  - `go test -run 'TestReconcileRetentionCohort|TestReconcileRetentionCohorts|TestRetentionReconcileUsesIPrangeComparePairs|TestLoadRetentionCohortsFromIndex' ./pkg/engine`
  - The new `TestReconcileRetentionCohortsAcrossCompareBatches` fixture creates
    more cohorts than one retention compare batch, then proves unchanged cohort
    bytes/mtime are untouched, a partially retained cohort is rewritten with
    only still-listed IPs, and a fully removed cohort file is deleted.
  - Focused benchmark command:
    `go test -run '^$' -bench 'BenchmarkCompareSourcePairs(RepeatedLeft|PartitionedOneToMany)FileSet|BenchmarkCompareNextSourcesFileSet' -benchmem ./pkg/iprange`
  - Benchmark evidence on linux/amd64, i9-12900K:
    `BenchmarkCompareSourcePairsRepeatedLeftFileSet/n=10000/targets=64`
    `9033737 ns/op`, `22024 B/op`, `20 allocs/op`; repeated-left dense
    batches avoid per-call setup but stay on the tight pair scanner.
  - Retention-shaped benchmark evidence:
    `BenchmarkCompareSourcePairsPartitionedOneToManyFileSet/n=10000/targets=64`
    `1925305 ns/op`, `40968 B/op`, `22 allocs/op`; partitioned cohort-like
    batches use the one-to-many path and are about 4.7x faster than the dense
    repeated-left batch shape in this synthetic benchmark.
- History-safe audit harness validation:
  - `go test ./tools/historyaudit`
  - Tests prove the manifest records history/retention checksums, row counts,
    retention cohort counts, first/last timestamps, monotonic timestamp state,
    deterministic feed/artifact ordering, and hidden atomic temp-file skipping.
  - Tests also prove a non-monotonic history ledger is flagged without modifying
    the ledger.
- Entity feed-presence and `pkg/iprange` batched comparison validation:
  - Pre-change focused tests failed as expected for the missing index and old
    engine-local comparison delegation behavior.
  - `go test -run 'TestCompareSourcePairs' ./pkg/iprange`
  - `go test -run 'TestComparisonPairsDelegateExactBatchToIPrange|TestWriteComparisonFilesUsesPairLedgerForUnchangedUpdatedFeed|TestComparisonPairLedgerIncrementalReplacementPreservesUnchangedEntries|TestComparisonPairLedgerMissingFallbackRebuildsFullLedger|TestComparisonPairLedgerCachedZeroDeletesStaleRowsOnBothPeers|TestRebuildEntityArtifactsWritesFeedPresenceIndex|TestMissingCommittedFeedSidecarUsesPresenceIndexForFullRebuildProof' ./pkg/engine`
  - `go test ./pkg/iprange`
  - `go test ./pkg/engine`
  - `go test ./tools/archposture`
  - Allocation profile before typed heap:
    `BenchmarkCompareNextSourcesFileSet/n=10000` `1588264 ns/op`,
    `2561168 B/op`, `20015 allocs/op`; top allocation source was
    `oneToManyCursorHeap.Pop` / `container/heap.Pop`.
  - Production-shaped scratch comparison after dispatch correction:
    one generated 10k-range binary set compared with 64 generated 10k-range
    binary peers took the old raw `OverlapCountIterContext` pair loop about
    `9.99ms/op`; `pkg/iprange.CompareSourcePairs` over the same already-open
    FileSets took about `10.10ms/op`, with fewer allocation objects. This
    validates that ownership moved to `pkg/iprange` without the earlier
    one-to-many heap regression, but it does not prove a large CPU win for this
    synthetic shape.
  - Final benchmark command:
    `go test -run '^$' -bench 'BenchmarkCompareNextSourcesFileSet|BenchmarkRunComparisonPairsPairLedgerHits|BenchmarkComparisonPairLedgerBinaryCodec' -benchmem ./pkg/iprange ./pkg/engine`
  - Final `pkg/iprange` benchmark evidence on linux/amd64, i9-12900K:
    `BenchmarkCompareNextSourcesFileSet/n=1000` `18585 ns/op`, `672 B/op`,
    `9 allocs/op`; `n=10000` `195827 ns/op`, `672 B/op`, `9 allocs/op`;
    `n=100000` `2152579 ns/op`, `672 B/op`, `9 allocs/op`.
  - Final engine ledger benchmarks:
    `BenchmarkRunComparisonPairsPairLedgerHits` `32746818 ns/op`,
    `13046638 B/op`, `35 allocs/op`;
    `BenchmarkComparisonPairLedgerBinaryCodec/marshal` `6022684 ns/op`,
    `6530902 B/op`, `25 allocs/op`;
    `BenchmarkComparisonPairLedgerBinaryCodec/parse` `3208054 ns/op`,
    `8955397 B/op`, `402 allocs/op`.
  - `make test`
  - `make lint`
- JSON replacement analysis validation:
  - temp global import replacement with `github.com/goccy/go-json@v0.10.6`
    followed by `go test ./...`: passed.
  - temp global import replacement with
    `github.com/segmentio/encoding/json@v0.5.4` followed by `go test ./...`:
    application packages passed; `tools/archposture` failed because
    `Decoder.More` is missing.
  - scratch retained-heap probe for a 20.4MB legacy ledger-shaped payload:
    confirmed copy/zero-copy lifetime differences that normal benchmark
    allocation counters do not show.
  - `go test -run 'TestVelox' -count=1 -v ./...` in `tools/jsonbench`:
    passed, with cache-state marshal crash reproduced safely in a child
    process.
  - `go test ./...` in `tools/jsonbench`: passed.
  - `make jsonbench`: passed; Velox cache-state rows were skipped because the
    compatibility test proves that candidate currently crashes on that payload.
- Benchmark evidence:
  - `go test -run '^$' -bench 'BenchmarkComparisonPairLedgerBinaryCodec' -benchmem -count=10 ./pkg/engine`
  - `go test -run '^$' -bench 'BenchmarkComparisonPairLedgerJSON(Marshal|Unmarshal)' -benchmem -count=10 ./...` in `tools/jsonbench`
  - `go run golang.org/x/perf/cmd/benchstat@latest ...`

Real-use evidence:

- Production monitoring evidence recorded in Analysis.
- Production integrity blocker observed on 2026-06-21 after the optimized
  entity/feed-sidecar baseline was deployed:
  - `/api/v1/admin/integrity` returned `status=issues`, `count=1`; the single
    feed-output finding was `cymru_unassigned` blocked by unavailable input
    feed `bogons`.
  - `/api/v1/admin/integrity/entities` returned `status=issues`, `count=341`;
    all findings were `feed_sidecar_missing`.
  - The production entity store had `lib/entities/version` set to `3`,
    `lib/entities/feed-presence-v1.bin` present but only 604 bytes,
    `lib/entities/feeds/` containing 36 feed sidecars, no
    `lib/entities/feeds-pending/` directory, 227 country sidecars, and 22514
    ASN sidecars.
  - Representative missing feed sidecars had existing local inputs such as
    `lib/{feed}/latest`, `web/{feed}_dbip_country.json`, and
    `web/{feed}_asn_iptoasn.json`, so the integrity findings were not empty
    input false positives.
  - Root-cause model from code inspection: `entityArtifactsNeedBootstrapFast`
    treats the entity surface as bootstrapped when the version marker, public
    country index, public ASN index, and home aggregate exist. It does not
    prove the new canonical feed-sidecar store or feed-presence index is
    complete. Targeted refreshes can then keep publishing a partial
    feed-sidecar universe while updating `version` and `feed-presence-v1.bin`.
  - Missing test coverage: no behavioral test reproduced the upgrade shape
    where `version` already matches the current entity artifact version but
    the feed-sidecar store and feed-presence index are partial.
  - Hotfix implemented from this evidence:
    - bumped the private entity artifact version from `3` to `4`, so the
      deployed service cannot accept the partial v3 feed-sidecar store as
      current after restart
    - `entityArtifactsNeedBootstrapFast` now requires a readable
      `feed-presence-v1.bin`; missing or unreadable state forces full entity
      bootstrap instead of targeted refresh
    - admin/startup entity integrity now reports missing or unreadable
      feed-presence indexes as global full-rebuild findings
    - added behavioral tests for the production-shaped upgrade state:
      previous-version partial feed-sidecar store plus current public entity
      artifacts must trigger a full rebuild and end clean
    - added behavioral tests proving a missing feed-presence index is not a
      clean current-version entity surface
- Post-deploy production monitoring on 2026-06-21 after restart of the latest
  pushed build:
  - service reached active state and completed the monitored startup/reprocess
    work without watchdog kill
  - first monitored source wave: `sources` phase `295500ms`;
    `sources.refresh_rotation` `158451ms` total, max `129988ms`;
    `sources.parse_feed_body` `103052ms` total, max `100947ms`;
    `sources.update_retention` `8851ms` total, max `2340ms`;
    `sources.finalize.observe_history` `22ms` total
  - active retention reconciliation during the next wave scanned at about
    `1651` cohort files/s, confirming the previous `8` files/s production
    symptom was fixed by bounded `pkg/iprange` batch comparison
  - second monitored source wave: rotation stayed at `1ms`/feed after runtime
    changeset tails were loaded; older cache entries still triggered one-time
    history stats recomputation with `sources.finalize.observe_history`
    `26401ms` total, max `7459ms`
  - final small cycle: run duration about `12s`; `sources` phase `300ms`;
    `sources.refresh_rotation` `1ms`; metadata phase `6033ms`; comparison
    ledger read/write each about `6.56MB`; entity output view read/decoded
    about `57.5MB` of ASN JSON
  - process snapshot near completion: about `989s` CPU, `11.0GB` process
    read bytes, `3.5GB` write bytes, `345MB` RSS, and no active engine work
  - conclusion: retention is no longer the dominant bottleneck on this
    baseline; remaining measured opportunities are parser progress overhead on
    very large feeds, bounded changeset/history window access on cold runtime
    cache paths, metadata comparison ledger churn, latest-set open fan-out, and
    entity output JSON reads
- Implementation after this monitoring pass:
  - `pkg/iprange.ParseReader` progress callbacks no longer use per-line
    `defer` and avoid per-line time checks unless byte/line thresholds are met
  - runtime changeset windows now use bounded tail reads of
    `lib/{feed}/changesets.csv` instead of scanning the full append-only ledger
    for the recent rotation/change-ratio window
  - shared changeset CSV line parsing now uses `strings.Cut` and
    `strconv.Parse*` instead of per-row `strings.Split` slices and
    `fmt.Sscanf`
  - added `BenchmarkParseIPsWithProgress` and
    `BenchmarkLoadChangesetTailLargeLedger`
- Production incident follow-up on 2026-06-21:
  - The apparent production deadlock was not a Go deadlock. The service later
    recovered, and the logs show long metadata comparison work with CPU and I/O
    activity.
  - The operator-visible stall was real: `metadata.compare_pairs` stayed at
    `0/3629` while a single batched comparison call ran, so admin progress did
    not move during the heavy work.
  - Admin API responsiveness was also a real problem:
    `http.admin_status.build` reached `184308ms`, so reloading the admin UI
    could wait behind expensive live status construction.
  - The repeated `/api/v1/categories` `404` entries were caused by split
    listener mode. The shared SPA requests public category metadata from the
    current origin, but the admin-only listener registered admin routes and
    embedded assets without the read-only public category API.
  - Public website serving is intended to keep working during processing
    because public and admin listeners are split when configured that way and
    public routes serve cached/published artifacts. Host saturation can still
    degrade response time, but public reads must not wait on live admin status
    construction or metadata recomputation.
- Implementation decision for this incident:
  - Orient exact changed-feed comparison candidates so the changed feed becomes
    the `pkg/iprange` compare driver where a pair has exactly one changed side,
    preserving output pair identity while allowing the optimized repeated-left
    path to apply.
  - Split exact metadata comparisons into bounded left-driver batches and
    update `metadata.compare_pairs` progress after each batch.
  - Keep admin/public status endpoints on a lightweight engine status snapshot
    that includes current run state, active operations, and background tasks,
    but does not clone full current/last/lifetime run metrics on every poll.
  - Expose the read-only public categories API on the admin listener so the
    shared SPA can load on the admin origin without `404` noise.
- Implemented the incident fix:
  - metadata exact comparisons now orient one-sided changed-feed pairs around
    the changed feed, split large driver groups into bounded batches, and update
    `metadata.compare_pairs` progress after each batch
  - admin/public status and integrity suppression checks now use a lightweight
    engine status snapshot that omits full run metric trees
  - scheduler running-state checks now use the same lightweight snapshot
  - split admin listeners now serve `GET /api/v1/categories` as read-only
    product metadata for the shared SPA shell
  - status snapshot code was moved out of `pkg/engine/query.go` to keep the
    architecture file-size posture passing
- Validation after this monitoring pass:
  - `go test -run 'TestParseReader(MixedInput|ReportsProgress|ReportsOperationStats|RangeCapacityHintPreservesIPv4Result)' ./pkg/iprange`
  - `go test -run 'TestChangesetTailFromRuntime|TestPublicChangesets|TestChangesetSeries|TestReadInsightsChangesets' ./pkg/engine`
  - `go test ./pkg/engine ./pkg/iprange ./tools/archposture`
  - `make test`
  - `make lint`
  - `BenchmarkParseIPsWithProgress` improved from about
    `1.28-1.39ms/op` to about `0.86-0.96ms/op` on the local 10k-line
    progress fixture with the same `7 allocs/op`
  - `BenchmarkLoadChangesetTailLargeLedger` on a 100k-row ledger measured about
    `0.38-0.41ms/op`, `283938 B/op`, and `14 allocs/op`
- Validation after production integrity hotfix:
  - `go test ./pkg/engine -run 'TestCheckEntityArtifactsIntegrity(RequiresFeedPresenceIndex|FlagsMismatchedVersionMarker|FlagsMissingVersionMarker)|TestRefreshEntityArtifactsRebuildsPreviousVersionPartialFeedSidecarStore|TestSurgicalRefreshRebuildsAffectedDetailsFromFeedSidecars|TestRebuildEntityArtifactsWritesFeedPresenceIndex|TestMissingCommittedFeedSidecarUsesPresenceIndexForFullRebuildProof' -count=1`
  - `go test ./pkg/engine -count=1`
  - `make test`
  - `make lint`
- Validation after production metadata/admin stall fix:
  - `go test ./pkg/engine -run 'Test(ComparisonPairBatches|StatusSnapshotLight)' -count=1`
  - `go test ./pkg/web -run 'Test(SurfaceHandlerModesRegisterExpectedSurfaces|AdminStatusUsesLightEngineSnapshot)' -count=1`
  - `go test ./pkg/engine ./pkg/web ./pkg/scheduler ./pkg/iprange -count=1`
  - `go test ./tools/archposture -count=1`
  - `make test`
  - `make lint`

Reviewer findings:

- Pending.

Same-failure scan:

- Initial scan found broad `encoding/json` usage across engine, cache, scheduler, web, markdown, processor, tools, and tests.

Sensitive data gate:

- This SOW contains no raw secrets, credentials, bearer tokens, SNMP communities, community member names, customer names, personal data, customer-identifying IPs, private endpoints, or proprietary incident details.

Artifact maintenance gate:

- AGENTS.md: updated with historical feed data preservation guardrail.
- Runtime project skills: `.agents/skills/project-testing/SKILL.md` updated with `make jsonbench`; `.agents/skills/project-coding/SKILL.md` updated to keep changed-feed entity refresh from using actor JSON sidecars as hot-path patch state and to keep retention removal reconciliation delegated to bounded `pkg/iprange` batches.
- Specs: `.agents/sow/specs/files-layout.md`, `.agents/sow/specs/pipeline.md`, `.agents/sow/specs/integrity.md`, `.agents/sow/specs/operating-principles.md`, `.agents/sow/specs/processing-engine.md`, `.agents/sow/specs/memory-management.md`, `.agents/sow/specs/admin-ui.md`, and `.agents/sow/specs/website.md` updated for `cache/comparison-pairs-v2.bin`, v1 read-only upgrade input, full-rebuild semantics, the entity feed-presence index, current-version entity bootstrap integrity, batched `pkg/iprange` comparison, feed-sidecar-driven selected actor detail rebuilds, bounded retention cohort comparison batches, bounded changeset tail/window access for rotation/change-ratio consumers, lightweight admin polling status, split-listener category metadata, and bounded comparison progress.
- End-user/operator docs: no public/operator docs update needed; `tools/jsonbench/README.md` added for developer benchmark usage; `tools/historyaudit/README.md` added for local pre/post history and retention audit usage.
- End-user/operator skills: no update needed; operator workflows did not change.
- SOW lifecycle: moved from `.agents/sow/pending/` to `.agents/sow/current/`; `Status: open`.

Specs update:

- Updated files-layout, pipeline, and operating-principles specs for comparison
  ledger cache behavior, entity feed-presence proof, batched `pkg/iprange`
  exact comparison, and changed-feed entity refresh rebuilding selected actors
  from merged feed sidecars instead of actor JSON patching.
- Updated files-layout and processing-engine specs to prevent bounded
  changeset-window consumers from rescanning full append-only ledgers.

Project skills update:

- Updated project-testing with the JSON benchmark target.
- Updated project-coding with the feed-sidecar canonical-state rule for
  changed-feed entity refresh.
- Updated project-coding with the parser progress hot-path rule: no per-line
  defers, time calls, logging, telemetry callbacks, or interface-heavy hooks in
  `pkg/iprange` parser loops.

End-user/operator docs update:

- No operator-visible migration, backup, or rollback behavior introduced in this slice.
- Added `tools/historyaudit/README.md` for local read-only manifest generation
  before and after history/retention migration experiments.

End-user/operator skills update:

- No operator workflow change in this slice.

Lessons:

- History data must be treated as irreplaceable source-of-truth data, not as an optimization target that can be rewritten in place.
- Faster JSON libraries may help selected hot paths, but full-file JSON artifact designs can remain slow even with a faster codec.
- For reproducible internal caches, binary format with a domain-specific table can beat JSON libraries on both speed and allocation profile while avoiding public JSON compatibility risk.
- Candidate JSON libraries still need per-surface compatibility gates; the fastest default modes often trade away stdlib-compatible behavior or safe input-buffer lifetime.
- Ordinary changed-feed entity refresh should not use country/ASN actor JSON as
  patch state. Per-feed sidecars already contain the canonical contribution
  facts and are loaded for the feed-presence index, so selected actor rebuilds
  avoid actor JSON read storms while preserving clean-rebuild equivalence.

Follow-up mapping:

- Remaining work in this SOW:
  - evaluate the new bounded retention removal path against production data and
    decide whether a further non-destructive first-seen/history index is still
    needed
  - targeted JSON-library replacement only if a remaining measured hot path
    shows a material win and passes artifact-specific compatibility tests; the
    broad JSON migration is rejected for this slice after Velox crashed on a
    project-shaped payload and safer candidates did not justify the migration
    risk
  - further production-baseline monitoring after the entity feed-presence and
    batched comparison and selected actor-rebuild changes are deployed
  - decide whether one-time history stats recomputation on old cache entries
    needs a non-destructive derived stats sidecar if production restarts still
    risk repeating that work before the state cache is saved
  - decide whether metadata comparison latest-set open fan-out and entity
    output JSON reads should become the next focused optimization slice

## Outcome

Pending.

## Lessons Extracted

Pending.

## Followup

None yet.

## Regression Log

None yet.

Append regression entries here only after this SOW was completed or closed and later testing or use found broken behavior. Use a dated `## Regression - YYYY-MM-DD` heading at the end of the file. Never prepend regression content above the original SOW narrative.
