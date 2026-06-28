# SOW-0120 - Engine Staticcheck Unused Code Cleanup

## Status

Status: open

Sub-state: pending follow-up discovered while validating SOW-0117.

## Requirements

### Purpose

Keep CI hygiene reliable without mixing broad engine cleanup into the
web-serving liveness regression fix.

### User Request

No direct user request yet. This is a follow-up ledger item created because
`make staticcheck` reports a broad `pkg/engine` unused-code class while
validating SOW-0117.

### Assistant Understanding

Facts:

- `make staticcheck` runs in CI.
- During SOW-0117 validation, the web-serving/liveness staticcheck findings
  were fixed.
- A broad unrelated `pkg/engine` `U1000` unused-code class remains across many
  engine modules.

Inferences:

- The cleanup is larger than the web-serving SOW because it may remove or
  rewire many engine helper wrappers unrelated to admin/telemetry/watchdog
  liveness.

Unknowns:

- Whether each unused helper is truly dead production code, test-only intended
  code, an API planned for near-term engine work, or code that should be
  reconnected.

### Acceptance Criteria

- `make staticcheck` passes, or the SOW records an evidence-backed decision to
  configure/exclude a specific class of intentional findings.
- Any removed helpers are proven unused by production and tests before removal.
- Engine behavior is unchanged unless explicitly approved.

## Analysis

Sources checked:

- `make staticcheck` output during SOW-0117 validation.

Current state:

- Staticcheck reports `U1000` for many unexported `pkg/engine` helpers outside
  the SOW-0117 changed files.

Risks:

- Blind deletion can break planned engine refactors, tests, or operational
  repair paths.
- Leaving the findings unaddressed keeps the documented CI staticcheck gate
  unreliable.

## Pre-Implementation Gate

Status: needs-user-decision

Problem / root-cause model:

- The engine contains many unexported helper wrappers that staticcheck sees as
  unused. The root cause may be completed refactors that left wrappers behind,
  intentionally staged code, or missing call connections.

Evidence reviewed:

- `make staticcheck` output from SOW-0117 local validation.

Affected contracts and surfaces:

- Engine internals, CI/static analysis hygiene, tests.

Existing patterns to reuse:

- SOW-0117 validation used focused staticcheck for touched packages to avoid
  mixing unrelated engine cleanup into a web-serving regression.

Risk and blast radius:

- Potentially broad engine source cleanup. Treat as separate work.

Sensitive data handling plan:

- No raw production data is needed. Staticcheck output and file/function names
  are safe to record.

Implementation plan:

1. Inventory all `U1000` findings and classify each as remove, reconnect, or
   intentionally suppress with evidence.
2. Make the smallest safe cleanup, then run full Go/static analysis gates.

Validation plan:

- `make test`
- `make lint`
- `make staticcheck`
- `make test-strict`
- `make race`

Artifact impact plan:

- AGENTS.md: likely unaffected.
- Runtime project skills: update only if a durable staticcheck policy changes.
- Specs: likely unaffected unless engine behavior changes.
- End-user/operator docs: likely unaffected.
- End-user/operator skills: likely unaffected.
- SOW lifecycle: keep this as pending follow-up, not part of SOW-0117.

Open-source reference evidence:

- Not relevant for static analyzer cleanup.

Open decisions:

- Decide whether to remove dead engine helpers, reconnect them, or configure
  staticcheck policy for intentional helper wrappers.
