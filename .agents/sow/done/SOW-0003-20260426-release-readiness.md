# SOW-0003 | 2026-04-26 | release-readiness

## Status

completed
completed as the superseded release-readiness migration baseline; pending release work moved to `SOW-0007-20260426-release-pending-work.md`

## Requirements

Completion scope after the 2026-04-26 split:

Given this SOW compressed active and completed release-tracker state too much,
when it is completed, then it must clearly stop being the active pending-release
queue and must point to the new canonical pending SOW.

Given release readiness itself is not complete, when this SOW is completed, then
the completion must not claim that the public release is ready; it only closes
the migrated baseline/tracker role.

Given update-ipsets is being prepared for a public upstream release and later production cutover, when this SOW is complete, then the application must be cheap to serve, operationally visible, secure, documented, and ready for a flattened-history upstream publication.

Given public pages and APIs should not create avoidable server CPU load, when country, ASN, homepage, maintainer, compose, overlap, and search surfaces are finalized, then heavy work must be precomputed, bounded, cached, or explicitly queued.

Given operators need predictable behavior, when background work, feed health, cadence, integrity, and update queues run, then admin UI/API must make that work visible.

Given the repo will become public, when release preparation finishes, then secrets, hardcoded local paths, licensing, docs, CI, and generated artifacts must be audited.

## Analysis

Original source:

- `.agents/sow/.todo-backup/TODO-release-master.md`

Split note:

- On 2026-04-26, Costa directed cleanup of the migrated release tracker because
  `SOW-0003` was too compressed and the detailed active/pending work still lived
  in the preserved TODO backup.
- `SOW-0004`, `SOW-0005`, and `SOW-0006` already exist, so the next valid SOW
  number is `SOW-0007` per the SOW numbering rule.
- Pending release work is now tracked in
  `.agents/sow/pending/SOW-0007-20260426-release-pending-work.md`.

Implemented or substantially completed from the original 22-item request:

- Config catalog is directory-based.
- Homepage client-IP bootstrap exists.
- Homepage copy and bogon/private-space copy updates are done.
- Country and ASN index pages exist.
- Country and ASN detail APIs are file-backed artifact readers.
- Background entity work is visible in admin status/UI.
- OpenTelemetry export and dependency updates were added.

Active/current areas:

- Country/ASN precompute and entity artifact operational profile still need release-grade validation.
- MISP feed coverage and zero-history repair need continued audit.
- Adaptive cadence and health-transition public logs are not complete release features.
- Critical ASN and visualization reviews remain data-quality work.
- Public composer has backend groundwork, but public UI/release contract is not complete.

These items are no longer controlled from this SOW; they were moved into
`SOW-0007-20260426-release-pending-work.md` as the canonical pending-work
inventory.

Pending release streams:

- Contributor methodology for adding feeds and making PRs.
- Operator docs in `docs/` and optional wiki mirror workflow.
- Homepage presentation/review of `firehol_*` merges.
- Upstream flattened-history/security release audit.
- Project-local operational AI skills beyond the SOW project skills.
- Public Streamable HTTP MCP endpoint at `/mcp`.
- Public paste-a-set overlap page and bounded backend APIs.
- AI-assisted feed enrichment/evaluation design.

## Implications and decisions

- This SOW is completed only as the migrated release-readiness baseline and is no
  longer the active release work queue.
- Future release work starts from `SOW-0007` and can split into child SOWs when
  implementation starts and the scope becomes concrete.
- Product decisions already captured in the original TODO remain preserved in `.agents/sow/.todo-backup/TODO-release-master.md`.
- No release-stream implementation starts from this SOW without explicit Costa approval.
- `SOW-0003` being completed does not mean release readiness is complete.

## Plan

1. Preserve the original release TODO backup.
2. Record which release work was already substantially completed.
3. Move pending and regressing release work into a new canonical pending SOW.
4. Complete this SOW as the migrated baseline so active work is no longer split
   across `SOW-0003` and the hidden TODO backup.

## Execution log

2026-04-26:

- Migrated the active release tracker into SOW form.
- Preserved the original detailed tracker at `.agents/sow/.todo-backup/TODO-release-master.md`.
- Split pending release work into
  `.agents/sow/pending/SOW-0007-20260426-release-pending-work.md`.
- Completed this SOW as the superseded release-readiness baseline after Costa
  explicitly asked to mark it completed.

## Validation

- [x] Acceptance criteria evidence

  Evidence: pending release work was moved to
  `.agents/sow/pending/SOW-0007-20260426-release-pending-work.md`; this SOW now
  states it is only the completed migrated baseline and does not claim release
  readiness.

- [x] Real-use validation evidence

  Evidence: `bash ~/.agents/skills/sow/scripts/audit.sh` passed after creating
  `SOW-0007`; `SOW-0007` appears in `pending/` and this SOW points to it.

- [x] Cross-model reviewer findings (logged + addressed)

  N/A - reason: Costa explicitly asked to mark this tracker/baseline SOW
  completed. No code, runtime behavior, release artifact, or product behavior is
  changed by this SOW completion. The pending work requiring review is captured
  in `SOW-0007`.

- [x] Lessons extracted (or "none, reasoning: ...")

  Evidence: see `## Lessons extracted`.

- [x] Same-failure-at-other-scales check

  Evidence: SOW numbering was checked across `pending/`, `current/`, and `done/`
  before creating the replacement. `SOW-0004`, `SOW-0005`, and `SOW-0006`
  already existed, so the replacement pending-work SOW was correctly numbered
  `SOW-0007`.

## Outcome

Completed as the superseded migrated release-readiness baseline.

Pending release work is now tracked by
`.agents/sow/pending/SOW-0007-20260426-release-pending-work.md`.

This file remains the migrated release-readiness baseline and points to the
original detailed backup at `.agents/sow/.todo-backup/TODO-release-master.md`.

## Lessons extracted

None, reasoning: this completion only closes the tracker split. The durable
process lesson is already encoded in `SOW-0007`: broad release work must be
tracked in the visible SOW queue, not hidden in a preserved TODO backup.

## Followup

- Pending release-work inventory: see
  `.agents/sow/pending/SOW-0007-20260426-release-pending-work.md`.
