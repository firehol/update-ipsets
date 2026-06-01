# SOW-0095 - Application Review From Docs Sync

## Status

Status: open

Sub-state: created as the concrete follow-up for product/code questions found while syncing operator docs and specs in SOW-0094.

## Requirements

### Purpose

Review and resolve the application behavior questions found while bringing operator docs and specs up to date. The goal is not to make docs describe desired future behavior; SOW-0094 already aligned docs/specs to current behavior. This SOW decides whether the recorded application behaviors should be kept, changed, deprecated, or further documented.

### User Request

Follow-up from SOW-0094. The user asked that issues, bugs, or clearly wrong logic found during docs/spec sync be gathered for review instead of silently changing code or specs to desired behavior.

### Assistant Understanding

Facts:

- SOW-0094 updated operator docs and specs to current behavior.
- SOW-0094 recorded several behaviors that may be intentional product decisions, missed implementation paths, or UI/help copy bugs.
- These items are not current docs blockers because SOW-0094 either documented the observed behavior or recorded the mismatch explicitly.

Inferences:

- The items should be handled in a focused application-review SOW because they span CLI help, scheduler retry semantics, frontend copy, MCP argument normalization, install/service behavior, and runtime configuration semantics.
- Some items may be resolved by code changes, while others may be rejected as intentional current behavior.

Unknowns:

- Which behaviors the user wants to keep versus change.
- Whether any behavior changes should be grouped or split into smaller SOWs after review.

### Acceptance Criteria

- Each review item from SOW-0094 has an explicit decision: keep as-is, implement, deprecate/remove, document-only, or split into a new focused SOW.
- Decisions are recorded with evidence and implications before implementation.
- Any accepted code/UI behavior change updates the relevant specs and operator docs in the same SOW.
- Validation covers each accepted change with targeted tests or equivalent observable checks.
- Rejected items remain documented as current behavior where operator-visible.

## Analysis

Sources checked:

- `.agents/sow/current/SOW-0094-20260531-operator-docs-sync.md`
- `pkg/web/middleware.go`
- `pkg/config/config.go`
- `pkg/engine/runtime.go`
- `pkg/scheduler/snapshot_build.go`
- `cmd/update-ipsets/main.go`
- `pkg/engine/run_pipeline.go`
- `ui/src/components/admin/feed-modal-status-sections.tsx`
- `pkg/engine/enabled_state.go`
- `pkg/mcp/server.go`
- `pkg/mcp/fetch_analysis.go`
- `install.sh`

Current state:

- The general `/api/` limiter includes authenticated admin API routes.
- `ignore_repeating_download_errors` is accepted and carried into runtime state, but scheduler retry timing is driven by failure count and health class instead of that field.
- `cache-merge` is a supported subcommand but the top-level help text omits it.
- `skip_comparison_if_no_updates=false` does not by itself force no-update public artifact regeneration.
- The admin feed detail drawer says `linear backoff`, while the scheduler doubles retry delays until capped.
- `enabled_by_all` is accepted catalog metadata, but current runtime `--enable-all` behavior enables every configured source without filtering by that field.
- MCP `fetch_analysis` describes `AS`-prefixed ASN names, but the markdown lookup path does not normalize or strip the `AS` prefix.
- `install.sh` accepts a custom install directory argument while the generated managed systemd unit hardcodes `/opt/update-ipsets`.

Risks:

- Changing scheduler retry or no-update regeneration semantics can affect upstream load, CPU, I/O, and operator expectations.
- Changing admin/API rate limiting can affect security posture and automation behavior.
- Changing `enabled_by_all` semantics can surprise operators who rely on current `--enable-all` behavior.
- Changing install path behavior can affect managed-service upgrades and path permissions.

## Pre-Implementation Gate

Status: needs-user-decision

Problem / root-cause model:

- SOW-0094 was a docs/spec sync pass. It identified application behaviors that may deserve product decisions but intentionally did not change code.

Evidence reviewed:

- SOW-0094 current-state and validation notes.

Affected contracts and surfaces:

- CLI help
- scheduler retry policy
- no-update processing/publishing policy
- admin UI copy
- catalog enablement semantics
- MCP tool argument contract
- install/systemd managed-service contract
- specs and operator docs if any behavior changes

Existing patterns to reuse:

- Keep docs and specs aligned with current behavior unless a behavior change is accepted.
- Use focused package tests for behavior changes and same-scope docs/spec validation after repairs.

Risk and blast radius:

- Medium. Most items are small individually, but several touch runtime scheduling, operator safety, or installation behavior.

Sensitive data handling plan:

- This review should use sanitized evidence only. Durable artifacts must not include secrets, credentials, customer identifiers, personal data, private endpoints, or non-private customer-identifying IPs.

Implementation plan:

1. Present the eight review items as numbered decisions with concrete evidence, options, implications, and recommendations.
2. Record user decisions in this SOW.
3. Implement only accepted changes, splitting into narrower SOWs if decisions are independent or high risk.
4. Update specs, operator docs, and tests for each accepted behavior change.

Validation plan:

- Targeted Go/UI tests for accepted behavior changes.
- CLI help command checks if help output changes.
- Docs/spec coverage scans for changed routes, flags, config fields, or UI copy.
- `git diff --check` over touched artifacts.

Artifact impact plan:

- AGENTS.md: no expected update unless project-wide SOW or process rules change.
- Runtime project skills: no expected update unless a durable new review lesson emerges.
- Specs: update affected specs only for accepted behavior changes.
- End-user/operator docs: update only when operator-visible behavior changes or a documented current behavior is intentionally changed.
- End-user/operator skills: none expected.
- SOW lifecycle: this SOW remains pending until the user prioritizes the review.

Open-source reference evidence:

- None. These are local application behavior questions found by comparing local docs/specs to local code.

Open decisions:

1. API/admin rate limiting: keep shared `/api/` limiter for admin API routes, or split admin API limits.
2. `ignore_repeating_download_errors`: implement in retry timing, deprecate/remove, or keep as accepted inert compatibility metadata.
3. Top-level CLI help: add `cache-merge` to help, or intentionally keep it out as a migration-only helper.
4. `skip_comparison_if_no_updates`: keep current limited role, implement explicit no-update regeneration, or deprecate/remove.
5. Admin retry copy: change `linear backoff` to match scheduler behavior, or intentionally keep current copy.
6. `enabled_by_all`: implement legacy filtering behavior, deprecate/remove, or keep as accepted catalog metadata only.
7. MCP ASN names: normalize `AS`-prefixed names, or require numeric ASN identifiers in the tool contract.
8. Custom install directory: generate matching managed units, reject custom paths for managed installs, or keep custom paths as manual/experimental.

## Implications And Decisions

Pending.

## Plan

Pending user prioritization and decisions.

## Execution Log

### 2026-06-01

- Created as the concrete pending follow-up for application-review items deferred from SOW-0094.

## Validation

Pending.

## Outcome

Pending.

## Lessons Extracted

Pending.

## Followup

None yet.

## Regression Log

None yet.
