# SOW-0123 - Entity JSON Streaming Stability

## Status

Status: completed

Sub-state: implementation locally validated and committed with SOW closure

## Requirements

### Purpose

Stabilize the daemon so routine entity artifact refreshes cannot stall the process, starve web-serving watchdog heartbeats, or trigger systemd watchdog kills under production memory and I/O pressure.

### User Request

The user asked to proceed, create an SOW, and stabilize the application after production crashes tied to entity artifact refresh and large JSON marshaling.

### Assistant Understanding

Facts:

- Production crashed after `queued entity artifact refresh` with no normal app logs until systemd watchdog action.
- Production service uses `GOMEMLIMIT=1536MiB`, `MemoryHigh=1536MiB`, and `MemoryMax=2GiB`; cgroup memory events show repeated `high` throttling and no kernel OOM kill.
- Crash evidence includes a runnable goroutine in `encoding/json.MarshalIndent` called from ASN detail artifact writing.
- `pkg/engine/entity_surgical_io.go` currently builds a whole indented JSON `[]byte`, appends a newline, then writes the whole buffer atomically.
- Entity artifact mtimes are integrity data and must remain explicit logical mtimes.

Inferences:

- The strongest root-cause model is an allocation/GC/cgroup-memory-pressure feedback loop caused by whole-buffer indented JSON generation for large entity artifacts.
- The same writer is used by ordinary entity refresh, selected entity repair, and entity indexes, so fixing the shared writer stabilizes more than one entity path.

Unknowns:

- The incomplete crash dumps do not prove an `encoding/json` infinite loop. The proven failure class is runnable GC/allocation pressure plus watchdog starvation.

### Acceptance Criteria

- Entity JSON artifact writes for feed sidecars, country/ASN detail sidecars, public detail payloads, and entity indexes do not require full output JSON buffers or an additional full-buffer newline copy.
- Atomic temp-file publication semantics are preserved for entity JSON writes.
- Entity artifact logical mtime behavior is preserved.
- Public/private entity JSON remains semantically valid JSON with a trailing newline.
- Tests cover successful streaming JSON writes, writer failure cleanup, mtime preservation, and large-payload allocation shape.
- Relevant specs record that entity artifact publication must stream generated JSON when possible and avoid whole-output buffering in refresh/repair hot paths.

## Analysis

Sources checked:

- Production systemd and namespaced application journals for the July 3 crash window.
- `pkg/engine/entity_surgical_io.go`
- `pkg/engine/entity_surgical_refresh.go`
- `pkg/engine/entity_artifact_selected_repair.go`
- `pkg/engine/entity_surgical_index.go`
- `internal/fileutil/fileutil.go`
- `.agents/sow/specs/memory-management.md`
- `.agents/sow/specs/pipeline.md`
- `.agents/sow/specs/operating-principles.md`
- Local Go 1.26 docs for `encoding/json.Encoder`, `os.CreateTemp`, `os.File.Chmod`, and `os.File.Sync`.

Current state:

- `writeObservedJSONFile()` calls `jsonMarshalTabIndent()`, which calls `json.MarshalIndent()`, then `append(data, '\n')`, then `writeFileAtomicNoSync()`.
- `writeFileAtomicNoSync()` accepts only `[]byte`; callers must materialize the full file in heap before file I/O begins.
- Entity refresh calls `stageRebuiltASNDetail()`, materializes an ASN payload, then calls the whole-buffer JSON writer for the public ASN detail file.
- `internal/fileutil` has atomic write helpers but no streaming atomic writer.

Risks:

- Changing JSON formatting may affect downstream consumers only if they incorrectly depend on whitespace. The product contract is JSON, not byte-identical pretty formatting.
- `encoding/json.Encoder` was checked and rejected for this hot path because the local Go implementation still marshals into an internal full buffer before writing.
- The entity-scoped compact streamer is intentionally limited to known entity artifact payload shapes; unrelated generic JSON writers keep the existing standard-library writer.
- Writing metrics byte counts now depends on counting bytes written by the stream, not `len([]byte)`.
- Atomic writer error handling must not leave temp files on encode/write/sync/chmod/close failures.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- Entity artifact refresh writes large country/ASN detail JSON files.
- The current writer builds one full indented JSON buffer and may copy it again to append the newline before writing to disk.
- Under production `GOMEMLIMIT` and `MemoryHigh`, this causes large transient heap pressure, GC pressure, and cgroup reclaim/throttle events.
- During the observed crashes, the application stopped logging and stopped satisfying watchdog heartbeats; systemd killed it after the watchdog timeout.

Evidence reviewed:

- Production logs: last normal event before the July 3 crash was `queued entity artifact refresh` for 25 feeds, followed by systemd watchdog action.
- Production cgroup: high memory events were present, OOM events were absent.
- Code evidence:
  - `pkg/engine/entity_surgical_io.go` whole-buffer JSON writer.
  - `pkg/engine/output.go` `json.MarshalIndent` implementation.
  - `pkg/engine/entity_surgical_refresh.go` ASN detail refresh path.
  - `internal/fileutil/fileutil.go` `[]byte`-only atomic writer.

Affected contracts and surfaces:

- Private entity JSON sidecars under `lib/entities/...`.
- Public entity JSON artifacts under `web/countries/...` and `web/asns/...`.
- Entity artifact integrity through explicit mtimes.
- Admin/operator stability and watchdog availability.
- Local file utility contract for atomic writes.

Existing patterns to reuse:

- `fileutil.WriteAtomic` and `WriteAtomicNoSync` temp-file/rename behavior.
- Engine `writeObservedJSONFileAt()` mtime wrapper.
- Existing entity artifact tests in `pkg/engine/home_detail_test.go` and `pkg/engine/entity_surgical_test.go`.
- Existing fileutil atomic writer tests.

Risk and blast radius:

- Scope is limited to JSON writer implementation used by entity artifact paths.
- The expected public visible change is JSON whitespace only.
- No history, retention, or source-of-truth feed data is modified.
- No public request path should generate or repair artifacts because of this change.

Sensitive data handling plan:

- Durable artifacts will include only redacted production evidence and code paths.
- No secrets, bearer tokens, SNMP communities, customer names, personal data, private endpoints, or customer-identifying non-private IPs will be written to SOWs, specs, docs, skills, or code comments.

Implementation plan:

1. Add streaming atomic writer support in `internal/fileutil` that accepts a writer callback, counts bytes, preserves mode, optional sync behavior, temp cleanup, and rename semantics.
2. Add engine helper wrappers for streaming atomic writes and switch entity artifact JSON writers to an entity-scoped compact streaming encoder.
3. Preserve `writeObservedJSONFileAt()` mtime behavior.
4. Add fileutil tests for successful streaming write and failed callback cleanup.
5. Add engine tests for semantic JSON output, trailing newline, logical mtime, and large-payload allocation shape.
6. Update specs to record the entity JSON streaming/bounded-memory contract.

Validation plan:

- Run targeted fileutil and engine tests.
- Run package-level engine tests touched by entity artifact behavior.
- Run `make test` if time permits after targeted validation.
- Run `make build` to prove the daemon compiles.
- Same-failure scan for remaining entity artifact hot-path full-buffer JSON writes.

Artifact impact plan:

- AGENTS.md: no update expected; the project already has bounded-memory and history-preservation guardrails.
- Runtime project skills: no update expected unless validation reveals a durable lesson not already covered.
- Specs: update memory-management and/or pipeline spec for streaming entity JSON artifact writes.
- End-user/operator docs: no update expected; this is internal stability with no operator-facing option.
- End-user/operator skills: no update expected.
- SOW lifecycle: this SOW is completed and moved to `done` with the implementation commit.

Open-source reference evidence:

- None. The fix uses local code and Go standard library APIs; no external implementation comparison is needed for this focused crash hotfix.

Open decisions:

- Resolved by user approval: proceed with the long-term-best stabilization fix, not a watchdog-only mitigation.

## Implications And Decisions

1. JSON formatting contract
   - Selected: JSON semantic compatibility is required; byte-identical pretty output is not required.
   - Reasoning: public artifacts promise valid JSON. Preserving exact indentation would keep the allocation-heavy behavior that crashed production.
   - Risk: downstream consumers depending on whitespace are outside the intended contract.

2. Scope
   - Selected: stabilize entity JSON artifact writer first.
   - Reasoning: crash evidence points to entity artifact JSON writing; broad JSON migration can be a separate optimization SOW if later evidence justifies it.
   - Risk: other JSON writers may still be inefficient, but they are not the proven crash path.

3. Atomicity
   - Selected: keep temp-file plus rename semantics.
   - Reasoning: staged publish and integrity depend on complete files, not partial writes.
   - Risk: streaming writer needs careful temp cleanup on encode/write errors.

## Plan

1. Create streaming fileutil writer and tests.
2. Convert entity artifact JSON writers to the compact streaming encoder and preserve metric byte accounting.
3. Add engine regression tests for semantic JSON, newline, mtime, and allocation shape.
4. Update bounded-memory/entity artifact specs.
5. Validate with targeted tests, build, and same-failure scan.

## Execution Log

### 2026-07-04

- Created SOW with production crash evidence, root-cause model, approved decisions, and implementation gate.
- Checked local Go 1.26 `encoding/json.Encoder` implementation before using it. It still creates an encode state, marshals the full value, appends the newline, and writes the resulting buffer. This does not remove the whole-output buffer failure mode.
- Added streaming atomic writer support in `internal/fileutil/fileutil.go` using the existing `file.write_atomic` metric identity, byte counting, temp-file cleanup, chmod, optional sync, close, and rename semantics.
- Added entity-scoped compact JSON streaming in `pkg/engine/json_stream.go`.
- Switched entity feed sidecars, country/ASN detail sidecars, public entity detail JSON, entity indexes, and observed entity repair writes to the streaming entity writer.
- Kept the generic `writeJSONFile()` path on the existing standard-library indented writer so unrelated home aggregate JSON is not silently moved to the custom entity streamer.
- Kept short string quoting on stack in the compact streamer to avoid replacing one large allocation with many small string-quote allocations.
- Updated memory-management, pipeline, and operating-principles specs with the bounded entity JSON emission contract.

## Validation

Acceptance criteria evidence:

- Streaming atomic writer exists at `internal/fileutil/fileutil.go:91` and writes through a callback into a temp file before rename at `internal/fileutil/fileutil.go:147`.
- Entity streaming writer entrypoints are at `pkg/engine/home_entity_detail_payload.go:140` and `pkg/engine/home_entity_detail_payload.go:151`.
- Observed entity repair writes now use byte-counted streaming at `pkg/engine/entity_surgical_io.go:116`.
- Full entity feed sidecars, country/ASN sidecars, public details, and indexes use streaming writers at `pkg/engine/entity_artifacts_write.go:223`, `pkg/engine/entity_artifacts_write.go:398`, `pkg/engine/entity_artifacts_write.go:403`, `pkg/engine/entity_artifacts_write.go:424`, `pkg/engine/entity_artifacts_write.go:429`, `pkg/engine/entity_artifacts_write.go:473`, and `pkg/engine/entity_artifacts_write.go:482`.
- Logical mtime behavior is still applied after the streaming write at `pkg/engine/home_entity_detail_payload.go:151` and `pkg/engine/entity_surgical_io.go:127`.

Tests or equivalent validation:

- `go test -count=1 ./internal/fileutil ./pkg/engine ./internal/observability` passed.
- `make test` passed.
- `make build` passed.
- `make lint` passed.
- `make staticcheck` was attempted and failed on broad existing `U1000` unused-code findings across `pkg/engine`; the output did not identify the new streaming JSON files or changed entity writer callsites as the failing source.
- Fileutil tests cover successful streaming write byte counts, permissions, parent directory creation, and temp cleanup after callback failure at `internal/fileutil/fileutil_test.go:218` and `internal/fileutil/fileutil_test.go:249`.
- Engine tests compare representative entity payload JSON with `encoding/json.Marshal`, verify trailing newline and logical mtime, and prove a multi-megabyte entity payload is emitted incrementally at `pkg/engine/json_stream_test.go:13`, `pkg/engine/json_stream_test.go:124`, and `pkg/engine/json_stream_test.go:159`.

Real-use evidence:

- Not installed or observed in production yet in this SOW turn.

Reviewer findings:

- No external reviewer run yet for this SOW chunk.

Same-failure scan:

- `rg -n "jsonMarshalTabIndent\\(|MarshalIndent\\(" internal/fileutil pkg/engine -g'*.go'` still finds non-entity whole-buffer JSON writers in metadata, comparison, retention, geolocation, ASN, bogon, critical, insight, recovery, and public series paths. These are not the proven crash path and remain outside this focused stabilization SOW.
- `rg -n "writeJSONFile\\(|writeJSONFileAt\\(|writeEntityJSONFile\\(|writeEntityJSONFileAt\\(" pkg/engine -g'*.go'` shows entity runtime writers on `writeEntityJSONFile*`; remaining `writeJSONFile*` runtime use is home aggregates, plus tests that create fixtures.

Sensitive data gate:

- Passed. Current SOW contains only redacted production evidence, file paths, line numbers, and command names.

Artifact maintenance gate:

- AGENTS.md: no update; existing bounded-memory and history-preservation guardrails already cover this class.
- Runtime project skills: no update; project coding/testing guidance already required scoped changes and behavioral tests.
- Specs: updated memory-management, pipeline, and operating-principles.
- End-user/operator docs: no update; behavior is internal stability, no user-facing option.
- End-user/operator skills: no update.
- SOW lifecycle: completed and moved to `done` with the implementation commit.

Specs update:

- `.agents/sow/specs/memory-management.md:162` now requires bounded streaming entity JSON publication for feed sidecars, country/ASN detail sidecars, public details, and entity indexes.
- `.agents/sow/specs/pipeline.md:671` now requires bounded streaming writers for entity feed sidecars, actor sidecars, public details, and indexes.
- `.agents/sow/specs/operating-principles.md:765` now records that entity refresh and repair must not materialize complete pretty JSON artifacts in heap.

Project skills update:

- None.

End-user/operator docs update:

- None.

End-user/operator skills update:

- None.

Lessons:

- For this hot path, `json.Encoder` is not a streaming fix by itself. The implementation must be verified at source level before assuming writer APIs avoid whole-output buffering.
- Custom JSON streaming must stay scoped to known payload families unless it fully implements the standard-library JSON contract.

Follow-up mapping:

- Remaining non-entity whole-buffer JSON writers should be reviewed under a separate optimization SOW only if logs or profiling show they create comparable memory or watchdog pressure.

## Outcome

Implementation locally validated and ready for production install after push. The SOW is closed with the implementation commit.

## Lessons Extracted

- `json.Encoder` can still allocate a full output buffer; do not use writer-shaped APIs as proof of streaming behavior without checking implementation and tests.
- Entity JSON writer changes should be entity-scoped to avoid turning a specialized high-performance encoder into a generic compatibility liability.

## Followup

None yet.

## Regression Log

None yet.
