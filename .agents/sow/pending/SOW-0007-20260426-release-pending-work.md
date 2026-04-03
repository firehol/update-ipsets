# SOW-0007 | 2026-04-26 | release-pending-work

## Status

open
release work has been split into explicit pending child SOWs `SOW-0008` through `SOW-0021`

## Requirements

Purpose: keep one visible release-control SOW that points to the exact pending
work SOWs approved by Costa, while preserving completed work and the full
historical evidence from the migrated release TODO.

Given `SOW-0003` compressed the detailed release tracker too aggressively, when
release work continues, then pending work must be tracked here instead of in
`.agents/sow/.todo-backup/TODO-release-master.md`.

Given some release items were completed or substantially completed before this
split, when this SOW is used for planning, then those items must not be reopened
unless evidence shows a regression or an incomplete release gate.

Given Costa provided the exact release-work list, when release work continues,
then the pending inventory must be `SOW-0008` through `SOW-0021` and not the
generic list previously drafted in this file.

Given each item is non-trivial, when implementation starts, then work must happen
inside the corresponding child SOW with its own analysis, plan, tests, docs, and
review gates.

## Analysis

Sources consulted:

- `.agents/sow/done/SOW-0003-20260426-release-readiness.md`
- `.agents/sow/.todo-backup/TODO-release-master.md`
- `~/.agents/skills/sow/sow-file-format.md`
- `find .agents/sow -maxdepth 2 -type f -name 'SOW-*.md'`

Current state before this correction:

- `SOW-0003` is now completed and retained as the migrated release-readiness
  baseline/completed-work map.
- `SOW-0004`, `SOW-0005`, and `SOW-0006` already exist in `done/`, so the next
  valid SOW number is `SOW-0007`.
- The detailed release tracker backup is 1,878 lines and still contains active
  work, completed work, decisions, implementation notes, and regressions.
- This SOW was the canonical pending-release inventory after the split, but its
  previous 17-item list was too generic and did not match Costa's intended
  release work breakdown.

Evidence from the old tracker:

- The original release request had 22 items grouped into 8 workstreams:
  `TODO-release-master.md` lines 20-57.
- Phase 3 lists config/catalog/contributor workflow and MISP gap audit:
  lines 963-971.
- Phase 4 lists adaptive cadence and public health-transition history:
  lines 973-977.
- Phase 5 lists FireHOL merge, infrastructure ASN, and visualization reviews:
  lines 979-983.
- Phase 6 lists operator docs, wiki/export, security audit, and flattened-history
  upstream plan: lines 985-990.
- Phase 7 lists public composer, paste-overlap, MCP, and AI enrichment/evaluation:
  lines 992-997.
- Release-prep/security audit is mandatory before upstream publication:
  line 1015.
- Entity artifact performance was made a release-gate investigation:
  lines 1105-1111.
- Later notes say this must be repeated before production release:
  line 1111.

Known completed or substantially completed from `SOW-0003` / tracker evidence:

- Config catalog is directory-based.
- Homepage client-IP bootstrap exists.
- Homepage copy and bogon/private-space copy updates were done.
- Country and ASN index pages exist.
- Country and ASN detail APIs are file-backed artifact readers.
- Background entity work is visible in admin status/UI.
- OpenTelemetry export and dependency updates were added.
- `runtime.max_background_workers` was added with default `1`.
- Startup/reload changed from unconditional full entity rebuilds to guarded
  entity-integrity repair.
- Scheduler cold-start no longer treats an empty previous health snapshot as
  "all feeds changed".
- Entity artifact integrity was added as a separate admin surface.
- Known admin wheel-trap containers in queue panels and feeds table were removed.
- Costa made final decisions to keep merges time-based and to leave eager
  Geo/ASN provider fetching as-is for now.

Pending child SOW inventory approved by Costa:

1. `SOW-0008-add-misp-feeds`
   - Add all MISP feeds that are reasonable to track after judging current MISP
     feeds, update cadence, value, and project category fit.

2. `SOW-0009-finalize-sow-specs`
   - Finalize the SOW system and reconcile/move existing specs into SOW specs per
     the SOW skill, resolving the current project rule that product specs live in
     `.agents/sow/specs/*.md`.

3. `SOW-0010-public-feed-composer`
   - Build the public page for composing a custom feed by including and excluding
     feeds/IPs, with documented APIs, telemetry, and MCP tools.

4. `SOW-0011-public-address-space-check`
   - Build the public page/API for checking a user-provided IP address space via
     text area and file upload, with documented APIs, telemetry, and MCP tools.

5. `SOW-0012-robots-sitemap-llms`
   - Add `robots.txt`, `sitemap.xml`, and `llms.txt`.

6. `SOW-0013-public-mcp-server`
   - Make the service a public Streamable HTTP MCP server exposing the public
     website as request schemas plus markdown results: feed/country/ASN/
     maintainer pages, search, category browsing, compose, and custom
     address-space overlap.

7. `SOW-0014-ai-in-the-loop`
   - Postponed pending discussion. Original draft covers AI-assisted maintainer
     metadata and AI feed review workflows.

8. `SOW-0015-admin-operation-visibility`
   - Ensure the admin UI provides full visibility into backend operations.

9. `SOW-0016-review-firehol-merges`
   - Remove/rework FireHOL merge feeds, updating dependencies, descriptions, and
     related presentation.

10. `SOW-0017-review-critical-asns`
    - Review critical infrastructure ASNs conservatively to protect truly
      critical public infrastructure such as key public DNS and high-impact
      network services from misleading blacklist warnings.

11. `SOW-0018-content-header-footer`
    - Modernize methodology/content pages and improve the currently primitive
      header/footer after a design discussion, including a GitHub icon.

12. `SOW-0019-feed-list-sidebar-ux`
    - Improve the feed-list sidebar/drawer UX with more powerful filtering and
      better scanning/navigation.

13. `SOW-0020-operator-manual-wiki`
    - Assistant-owned delivery: create the end-user/operator manual in `docs/`
      and GitHub wiki configuration, including new feeds, public APIs/rate
      limits, and MCP docs.

14. `SOW-0021-upstream-release-checklist`
    - Run the pre-remote GitHub repo checklist: references, secrets, compatibility,
      production continuity, and final squash to one commit.

## Implications and decisions

- `SOW-0003` is no longer the active pending-release tracker.
- `TODO-release-master.md` remains preserved evidence, not the active work queue.
- This SOW intentionally does not authorize implementation of the child SOWs.
  Each child SOW needs its own start decision and workflow.
- The previous generic pending list has been replaced by Costa's exact SOW map.
- `SOW-0004` could not be used for this split because it already exists as
  `.agents/sow/done/SOW-0004-20260426-admin-ui-workspace.md`.
- `SOW-0009` records and resolves the prior inconsistency where the SOW skill
  expected project specs in `.agents/sow/specs/` while the repo previously kept
  product specs under `specs/*.md`.
- 2026-04-28 Costa clarified child SOW scope:
  - `SOW-0010` and `SOW-0011` each require public pages, API docs, telemetry,
    and MCP tools.
  - `SOW-0013` should represent the public website through MCP schemas and
    markdown results, ideally via JSON-to-markdown templates.
  - `SOW-0014` is postponed pending discussion.
  - `SOW-0017` needs evidence-based critical ASN research and documentation.
  - `SOW-0018` and `SOW-0019` need UX/design discussion and improvement.
  - `SOW-0020` is delegated to the assistant for full manual/wiki delivery.

## Plan

Single-unit tracking update, no chunking - reasoning: this SOW only coordinates
the child SOW inventory. Implementation happens in the child SOWs.

1. Create `SOW-0008` through `SOW-0021` under `.agents/sow/pending/`.
2. Replace the generic pending list in this file with Costa's exact child SOW map.
3. Run the SOW audit to verify the queue is clean.

## Execution log

2026-04-26:

- Created from Costa's request to clean up `SOW-0003` / `TODO-release-master.md`
  split state.
- `SOW-0003` was completed as the migrated baseline tracker.
- This SOW became the canonical pending release-work inventory.
- Replaced the generic pending inventory with Costa's exact SOW map:
  `SOW-0008` through `SOW-0021`.

2026-04-28:

- Updated child SOW summaries with Costa's clarified scope decisions for
  `SOW-0010`, `SOW-0011`, `SOW-0013`, `SOW-0014`, `SOW-0017`, `SOW-0018`,
  `SOW-0019`, and `SOW-0020`.

## Validation

- [ ] Acceptance criteria evidence
- [ ] Real-use validation evidence
- [ ] Cross-model reviewer findings (logged + addressed)
- [ ] Lessons extracted (or "none, reasoning: ...")
- [ ] Same-failure-at-other-scales check

## Outcome

Pending. This SOW has not shipped release work; it only records the pending
release inventory after the tracker cleanup.

## Lessons extracted

Pending.

## Regression

None in this tracker SOW. Regressions or revalidation items should be recorded
inside the focused child SOW that owns the affected behavior.
