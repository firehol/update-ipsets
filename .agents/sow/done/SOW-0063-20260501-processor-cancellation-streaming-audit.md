# SOW-0063 - Processor Cancellation And Streaming Audit

## Status

Status: completed

Sub-state: validated

## Requirements

### Purpose

Improve processor cancellation and memory posture so large feed processing remains bounded and responsive to shutdown.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- `RunStream` falls back to `runBytesSegment` for non-streamable steps in `pkg/processor/run_stream.go`.
- `runBytesSegment` reads the whole source file before byte processing.
- `Run` copies the full input before processing in `pkg/processor/processor.go`.
- Some byte processors accept a context parameter but do not observe it.

Inferences:

- Memory remains bounded by upstream download/decompression limits, but byte-only segments can still be large.
- Shutdown cancellation can wait for long byte processors to complete.

Unknowns:

- None for this SOW after catalog inventory. Remaining byte-only processors are documented below as accepted whole-input classes or low-use legacy parsers.

### Acceptance Criteria

- Inventory processor steps by streaming support, catalog usage, and expected input size.
- Context cancellation is checked between byte steps and inside expensive processors.
- High-value remaining processors are converted to streaming where practical.
- Accepted memory ceilings for non-streamable processors are documented.
- Tests cover cancellation during byte segments and streaming fallback behavior.

## Analysis

Sources checked:

- `pkg/processor/run_stream.go`
- `pkg/processor/processor.go`
- Processor tests and fuzz targets.

Current state:

- The processor package has streaming support but still falls back to whole-file byte segments.

Risks:

- Converting processors incorrectly can change legacy feed normalization behavior.
- Cancellation checks must not corrupt staged output files or publish partial results.

## Implications And Decisions

User delegated implementation-quality, cleanup, testing, and audit SOWs that do
not require product direction. This SOW is classified as assistant-owned because
it tightens processor cancellation/memory posture without changing product
features or public behavior.

Decision:

1. Scope
   - A. Convert every byte processor to streaming.
     - Pros: strongest posture.
     - Cons: high risk and likely unnecessary for small transforms.
   - B. Prioritize catalog-used, large-input processors and add cancellation checks elsewhere. Selected.
     - Pros: risk-adjusted improvement.
     - Cons: leaves some byte processors intentionally bounded by policy.
   - C. Only document current limits.
     - Pros: low risk.
     - Cons: does not improve cancellation responsiveness.

Rationale:

- The highest-use remaining byte processor was the p2p gzip blocklist family at
  61 catalog uses (`p2p_blocklist`, `p2p_blocklist_ips`, and
  `p2p_blocklist_proxy`).
- JSON, XML/HTML, ZIP, hostname, and one-off legacy parsers have whole-input or
  low-use characteristics that make streaming rewrites higher risk than this
  quality SOW should take.

## Plan

1. Generate a processor usage and streaming-support inventory.
2. Identify large/hot non-streamable processor chains.
3. Add context checks around byte segment execution.
4. Convert selected processors to streaming.
5. Add tests/fuzz replay and update memory-management specs if needed.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Moved to current as assistant-owned implementation-quality work.
- Generated catalog processor inventory:
  - Streamed: `remove_comments` 100, `extract_ipv4_cidr` 67,
    `extract_ipv4` 17, `$CAT_CMD` 14, `filter_all4` 13,
    `dataplane_column3` 13, `csv_column` 6, `passthrough` 4,
    `append_slash32` 4, `remove_comments_semi` 3, `filter_ip4` 3,
    `pix_deny_rules` 2, and single-use `torproject_exits`, `snort_rules`,
    `dshield_format`.
  - Converted in this SOW: p2p gzip blocklists, 61 catalog uses total.
  - Accepted byte fallback: `json_path` 7, `unzip` 6, `xml_rss_title` 5,
    `json_paths` 5, `xml_rss_proxy` 3, `hphosts2ips` 3,
    `parse_cleantalk` 2, and single-use XML/HTML/ZIP/CSV/JSON legacy parsers.
- Added cancellation checks before byte materialization, between byte
  processors, after byte processing, before temp writes/copies, and before
  final file moves/copies.
- Converted `p2p_blocklist`, `p2p_blocklist_ips`, and
  `p2p_blocklist_proxy` plus raw aliases to streaming via the existing limited
  gzip stream path.
- Added behavioral tests for canceled byte runs, canceled byte fallback,
  intermediate cleanup after cancellation, no-step copy cancellation, and p2p
  streaming equivalence.
- Updated `.agents/sow/specs/memory-management.md` with the processor memory
  and cancellation contract.

## Validation

Acceptance criteria evidence:

- Inventory completed and recorded in the execution log.
- Byte fallback now checks context before materialization, between byte steps,
  after byte processing, and before temp output writes/copies.
- The p2p gzip blocklist processor family moved to streaming.
- Whole-input processor classes and accepted memory ceilings are documented in
  `.agents/sow/specs/memory-management.md`.
- Behavioral tests cover cancellation before byte processing, between byte
  steps, no-step copy cancellation, byte fallback cancellation, intermediate
  cleanup, and p2p streaming equivalence.

Tests or equivalent validation:

- `go test ./pkg/processor -run 'TestRunHonorsCanceledContextBeforeProcessing|TestRunStopsBetweenByteStepsAfterContextCancel|TestStreamP2PBlocklistEquivalence|TestStreamNoStepsHonorsCanceledContext|TestStreamByteFallbackHonorsCanceledContext|TestStreamCleansUpIntermediateAfterByteFallbackCancel|TestStreamFallbackForNonStreamable|TestStreamMixedPipeline|TestIsStreamable'`
- `go test ./pkg/processor`
- `make test`
- `make lint`
- `go test ./tools/archposture`
- `git diff --check`

Real-use evidence:

- Catalog inventory shows 61 p2p gzip blocklist catalog uses now stream instead
  of entering byte fallback.

Reviewer findings:

- Go best-practices review found non-streamable processor cancellation and memory posture gaps.

Same-failure scan:

- Scanned processor context usage with `rg -n "func .*\\(.*context\\.Context" pkg/processor -g '*.go'` and `rg -n "ctx\\.Err\\(|context\\.Canceled|context\\.Deadline" pkg/processor -g '*.go'`.
- Remaining ignored contexts are either streamable through `RunStream`,
  accepted whole-input classes, or low-use legacy parsers bounded by the memory
  spec.

Artifact maintenance gate:

- AGENTS.md: no update needed; existing processor/memory/cancellation rules
  already route this class of work through SOW/specs.
- Runtime project skills: no update needed; existing project-coding and
  project-testing rules already cover context, bounded memory, and validation.
- Specs: updated `.agents/sow/specs/memory-management.md`.
- End-user/operator docs: no update needed; no operator-facing command,
  configuration, or public behavior changed.
- End-user/operator skills: no update needed; no external operator workflow
  changed.
- SOW lifecycle: current SOW completed and ready to move to done.

Specs update:

- Updated `.agents/sow/specs/memory-management.md`.

Project skills update:

- Not needed.

End-user/operator docs update:

- Not needed.

End-user/operator skills update:

- Not needed.

Lessons:

- High-use compressed line-oriented processors should use the streaming
  pipeline and existing decompression caps instead of byte fallback.
- Cancellation tests should assert externally visible behavior: returned
  `context.Canceled`, no final result path, and no leaked intermediate temp
  output.

Follow-up mapping:

- No valid deferred work remains from this SOW.

## Outcome

Completed. Processor byte fallback is more cancellation-responsive, p2p gzip
blocklists stream through the limited gzip path, accepted byte fallback classes
are documented, and validation passed.

## Lessons Extracted

No runtime project skill update needed. Existing skills already require context
propagation, bounded memory, and behavioral tests; the SOW records the concrete
processor inventory and accepted fallback classes.

## Followup

None.
