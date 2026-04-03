# SOW-0016 | 2026-04-26 | review-firehol-merges

## Status

open
blocked on SOW-0014 (AI-in-the-loop): user wants AI-assisted feed grading to
decide which feeds belong in merges before making merge composition decisions

## Requirements

Given the `firehol_*` merge feeds are project-owned aggregates, when this SOW is complete, then their continued existence, dependencies, descriptions, categories, and public presentation must be reviewed and corrected.

Given merge composition should be informed by feed quality, when deciding which feeds to include in merges, then AI-assisted feed methodology collection and grading (from SOW-0014) should inform the decision.

Given merge changes can affect compatibility and user expectations, when any merge is changed, then compatibility, docs, redirects/renames, and migration impact must be explicit.

## Analysis

### User Context (2026-05-02)

- The project synthesizes a few feeds for general consumption (the `firehol_*` merges).
- Deciding which feeds should be part of these merges requires understanding each source feed's methodology and quality.
- User wants to use AI (SOW-0014) to collect the mechanics/methodology of each feed, grade it based on feed analysis, and then decide merge composition.
- This SOW is effectively blocked until SOW-0014 provides feed quality grading.

Initial sources to consult:

- `configs/firehol/merges/firehol_*.yaml`
- Existing public feed descriptions for `firehol_*`.
- Legacy FireHOL merge semantics.
- Public homepage/feed explorer presentation.
- `.agents/sow/specs/compatibility.md`, `.agents/sow/specs/feeds.md`, and `.agents/sow/specs/config.md`.

## Implications and decisions

- This SOW is blocked on SOW-0014 (AI-in-the-loop) for feed quality grading.
- Removing merges can break users who rely on historical FireHOL names.
- Dependency updates can change feed contents and trust semantics.

## Plan

1. `merge-inventory` — enumerate all `firehol_*` merges and their source feeds. Can start now (no dependency on SOW-0014).
2. `feed-grading` — use AI to collect methodology and grade each source feed. Blocked on SOW-0014.
3. `recommend-actions` — propose merge composition based on feed grades. Blocked on step 2.
4. `implement-approved-actions` — apply user-approved changes. High risk.
5. `compatibility-docs-tests` — verify no user breakage. High risk.

## Execution log

Pending.

## Validation

- [ ] Acceptance criteria evidence
- [ ] Real-use validation evidence
- [ ] Cross-model reviewer findings (logged + addressed)
- [ ] Lessons extracted (or "none, reasoning: ...")
- [ ] Same-failure-at-other-scales check

## Outcome

Pending.

## Lessons extracted

Pending.

## Cross-cutting dependency: FireHOL static enrichment (2026-05-19)

The AI-in-the-loop work (SOW-0014) produced a policy: FireHOL-maintained feeds — the 15 entries in `configs/firehol/merges/` plus `critical_dns` and `rfc_reserved` in sources — are NEVER enriched by the research agent (no-firehol-self-reference rule). Instead, `tools/build-firehol-static-enrichment.py` generates their enrichment deterministically from each YAML, using per-shape templates and hand-written editorial copy for the merge-tier semantics.

**Coupling**: any merge composition change made by this SOW (excluding strong critical infra, adding `_no_private` variations, removing archived-feed components, adjusting tier thresholds, renaming merges) requires regenerating the static enrichment so the public page reflects the new composition. Concretely:

1. **Component lists in the JSON** (`derivation.source_feeds[]`) must match the YAML's actual additive/subtractive composition.
2. **Tier-semantic editorial copy** in the generator (e.g., level1 = "low-FP edge blocking", level2 = "medium-confidence augment") must be revisited if the tier definitions change.
3. **`unlist_request` / `unlisting_policy`** for merges defer to component feeds — if a merge starts maintaining its own whitelisting or exclusion rules (e.g., a curated whitelist for `firehol_level1_no_critical_infra`), the generator's "defer to components" template stops being accurate.

**Long-term architectural direction (preferred)**: instead of carrying tier-semantic editorial copy in the static enrichment generator, derive the merge composition fully from YAML configuration at engine boot/refresh time. The engine already supports automated removal of archived/abandoned feeds from merges; extending that path so the public page renders composition dynamically from the live `additive:` / `subtractive:` / `excludes:` config would eliminate the enrichment-vs-config drift entirely. Under that model, the generator's role shrinks to producing only the immutable parts (license, redistribution, maintainer identity, neutral lifecycle); the dynamic composition and exclusion semantics come straight from the YAML on each render.

Whoever picks up this SOW should decide between (a) regenerate-on-config-change with the existing static-enrichment generator, or (b) move composition/exclusion semantics into engine-rendered dynamic blocks. Option (b) is cleaner; option (a) is faster to ship.

Affected paths:
- `tools/build-firehol-static-enrichment.py` (generator)
- `agents/run-enrichment.sh` (refuses FireHOL-maintained feeds via `maintainer: FireHOL` check)
- `.local/agents/feed-enrichment/<merge>/*/output.json` (generated outputs)
