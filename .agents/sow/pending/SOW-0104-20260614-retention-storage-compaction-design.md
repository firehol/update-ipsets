# SOW-0104 - Retention Storage Compaction Design

## Status

Status: open

Sub-state: pending product decision after SOW-0103 identified disk growth risk

## Requirements

### Purpose

Reduce long-term disk and file-cache growth from retention evidence while keeping
the application fit for operators who need trustworthy per-IP first-seen and
retention-age facts.

### User Request

SOW-0103 must not change user-facing functionality. Work that requires a
functional/product decision should be skipped, recorded, and handled separately.

### Assistant Understanding

Facts:

- Retention cohort files under `lib/{feed}/new/{unix_timestamp}` are currently
  the authoritative engine-owned source for current per-IP listing age and
  search `first_seen`.
- `lib/{feed}/retention.csv` is the detailed removal-life ledger for removed IPs.
- SOW-0103 already reduced heap pressure by reading previous latest sets and
  existing retention cohorts through file-backed range sources where possible.
- Disk growth remains possible for high-churn feeds because the current contract
  preserves per-cohort current-membership files and append-only removal evidence.

Inferences:

- Meaningful disk compaction may require changing one or more retention storage
  contracts, adding a migration/repair path, or introducing a new equivalent
  storage format.
- This is not safe to implement inside a no-functional-change SOW without a
  product decision.

Unknowns:

- Whether exact per-IP `first_seen` for current listings must remain queryable
  from lossless cohort files, or whether a compacted/indexed representation is
  acceptable.
- Whether old removal-life evidence can be bucketed, pruned, compressed, or
  migrated while preserving public retention summaries and operator debugging
  needs.

### Acceptance Criteria

- Inventory current retention disk usage by feed and file family using sanitized
  local/live evidence.
- Define which retention facts are product guarantees and which are
  implementation details.
- Present options with exact implications, risks, and migration requirements.
- If a design is approved, update specs before implementation.
- If no design is approved, record the rejection and keep SOW-0103's
  no-functional-change behavior intact.

## Analysis

Sources checked:

- `.agents/sow/specs/files-layout.md`
- `.agents/sow/specs/processing-engine.md`
- `pkg/engine/retention.go`
- `pkg/engine/retention_update.go`
- `pkg/engine/query.go`

Current state:

- `lib/{feed}/new/{unix_timestamp}` files partition current feed membership by
  contiguous listing start time.
- Public retention summaries are derived from `retention.json`, but IP lookup
  first-seen behavior uses the current cohort files.
- Removal evidence is appended to `retention.csv`.

Risks:

- Lossy compaction can change public/API answers for per-IP first-seen or
  retention histograms.
- Aggressive pruning can make operator investigations impossible after a feed
  churn spike.
- Migration bugs can corrupt durable retention evidence.

## Pre-Implementation Gate

Status: needs-user-decision

Problem / root-cause model:

- Retention stores exact current-membership cohorts and removal-life ledgers.
  This is disk-expensive for large, high-churn feeds, but those files are also
  current product evidence.

Evidence reviewed:

- `.agents/sow/specs/files-layout.md` defines cohort files as authoritative for
  current per-IP listing age and search `first_seen`.
- `pkg/engine/retention.go` rebuilds current retention buckets by reading cohort
  files.
- `pkg/engine/retention_update.go` rewrites cohort files after removals and
  appends removal-life rows to `retention.csv`.

Affected contracts and surfaces:

- Internal retention file layout.
- Public retention JSON and charts.
- IP lookup first-seen behavior.
- Integrity, repair, migration, and import from bash-era state.
- Operator docs and troubleshooting guidance.

Existing patterns to reuse:

- File-backed range sources.
- Explicit generated-file mtime contracts.
- Integrity and repair paths for generated artifacts.
- SOW-0103's no-functional-change retention heap improvements.

Risk and blast radius:

- High. Retention evidence is durable state, and errors can silently change
  historical interpretation.

Sensitive data handling plan:

- Use only sanitized disk-size summaries, file-family names, relative paths, and
  counts. Do not record raw IPs, feed payloads, customer-identifying addresses,
  private endpoints, secrets, credentials, or proprietary incident details.

Implementation plan:

1. Inventory retention disk usage and identify the largest file families.
2. Present retention storage design options and get a product decision.
3. Update specs for the approved design.
4. Implement migration, repair, integrity, and tests if a design is approved.

Validation plan:

- Current-vs-new retention API equivalence tests for unchanged semantics.
- Migration tests from current cohort layout.
- Integrity repair tests for missing/stale compacted state.
- Disk/heap benchmark or measurement for high-churn synthetic feeds.

Artifact impact plan:

- AGENTS.md: not expected unless a new project-wide retention rule is adopted.
- Runtime project skills: update if retention storage rules change.
- Specs: files-layout, processing-engine, memory-management, integrity.
- End-user/operator docs: update if layout, disk planning, repair, or migration
  behavior changes.
- End-user/operator skills: not expected unless docs expose new operational
  workflows.
- SOW lifecycle: pending follow-up from SOW-0103.

Open-source reference evidence:

- Not checked yet. This SOW is currently a product decision placeholder created
  from local project evidence.

Open decisions:

1. Which retention facts must stay exact?
2. Which retention evidence may be compacted, compressed, bucketed, or pruned?
3. Should existing installations migrate automatically, on operator action, or
   never?

## Plan

1. Gather sanitized retention disk inventory.
2. Present design options and recommendation.
3. Implement only after user approval.

## Execution Log

### 2026-06-14

- Created as a follow-up from SOW-0103 because retention storage compaction may
  require functional/product decisions.

## Validation

Acceptance criteria evidence:

- Pending.

Tests or equivalent validation:

- Pending.

Real-use evidence:

- Pending sanitized inventory.

Reviewer findings:

- Pending.

Sensitive data gate:

- This SOW contains no raw IP feed payloads, secrets, credentials, customer
  data, personal data, private endpoints, or proprietary incident details.

Artifact maintenance gate:

- SOW lifecycle: open in `.agents/sow/pending/` as follow-up from SOW-0103.

Follow-up mapping:

- Origin: `.agents/sow/current/SOW-0103-20260613-cpu-memory-optimization-without-functional-change.md`.
