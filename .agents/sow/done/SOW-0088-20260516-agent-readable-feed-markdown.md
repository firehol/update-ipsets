# SOW-0088 - Agent-readable feed markdown

## Status

Status: completed

Sub-state: regression fixed, installed, regenerated, and validated.

## Requirements

### Purpose

Make each feed markdown file usable as a standalone input for model-based feed evaluation. A reasonably capable non-SOTA model should be able to understand what the feed data means, identify right and wrong uses, reason about maintainer operations, and avoid common false conclusions without needing hidden website context.

### User Request

Implement the first stage of the markdown fix now:

- make markdown self-contained
- split Behavior into clearer lifecycle/change/cadence sections
- make rollups semantic and explained
- add compact interpretation text based on the public website methodology

Defer the full automated evaluation harness until the enrichment work is integrated. The future harness should test markdown quality by feeding one markdown file to a common weaker local model and asking one isolated question per fresh context. Failures should reveal missing context, contradictions, ambiguous wording, weak prompts, model inference errors, or bad evaluator expectations.

Regression correction request:

- Remove invented markdown lifecycle/change/cadence sections that do not exist on the website.
- Do not infer initial publication from `history.csv` or `changesets.csv`.
- Keep monitoring start represented only as `Tracked since`.
- Explain behavior using the bounded published history and changeset artifacts, aligned with the public website behavior semantics.

### Assistant Understanding

Facts:

- The existing markdown was modeled after the HTML feed-detail structure but omits much of the website's explanatory text.
- The existing markdown Behavior section collapses history, changesets, cadence, and rollup logic into one table.
- The public behavior artifacts are bounded movement evidence. They do not prove first publication for old or imported feeds.
- Monitoring start is tracked separately as metadata and rendered as `Tracked since`.
- The public website already has local interpretation text and methodology pages that explain several signals.
- The future model-evaluation harness should use a common weak but reasonably smart model, with `qwen3.6-35b-a3b` identified as a likely local evaluator.

Inferences:

- Markdown needs compact local interpretation for agent reasoning, but must not invent analytical sections or lifecycle concepts that diverge from the website contract.
- The generated markdown should explicitly distinguish observations, rollups, omitted data, and limits of inference.
- The immediate implementation should improve the markdown output and tests, but avoid building the evaluator before enrichment lands.

Unknowns:

- The exact enriched fields that will be added later are not finalized in this SOW.

### Acceptance Criteria

- Feed markdown keeps a `## Behavior` section aligned with the website's behavior semantics.
- Feed markdown does not render `## Publication Lifecycle`, `## Change Activity`, `## Observed Cadence`, `Initial publication`, or `post-baseline`.
- Feed markdown uses `Tracked since` as the monitoring-start statement and does not infer first publication from retained ledgers.
- Behavior markdown explains that public history/changeset artifacts are a bounded retained window and not a full lifetime ledger.
- Markdown includes compact interpretation text for analysis sections so a model can reason without reading the website.
- Tests cover the prior failure class: daily feed data must not be presented as an unexplained hourly behavior table, and retained changesets must not be labeled as first publication.
- Specs/docs are updated for the markdown analysis contract.

## Analysis

Sources checked:

- `configs/templates/markdown/feed.md.tmpl`
- `pkg/markdown/rollup.go`
- `pkg/engine/query.go`
- `ui/src/components/feed-detail/section-behavior.tsx`
- `ui/src/components/feed-detail/section-retention.tsx`
- `pkg/web/static/methodology/evolution.md`
- `pkg/web/static/methodology/update-cadence.md`
- `pkg/web/static/methodology/change-rate.md`
- `pkg/web/static/methodology/ip-retention.md`

Current state:

- Markdown currently emits a `## Behavior` table with `Table bucket` and a short note.
- The rollup code tries `1h`, then `1d`, `1w`, and `1mo`, accepting the first bucket count under 100 rows.
- The website explains Behavior as four charts over the last 500 recorded runs, while markdown compresses these signals into one table.
- The public changeset API documents and tests bootstrap-row omission; analysis markdown must describe the retained behavior window without converting that omission into a lifecycle claim.

Risks:

- If markdown remains underspecified, model evaluations will produce confident but wrong conclusions.
- If markdown over-explains every section with long prose, fetch responses may become too large and noisy.
- If markdown uses runtime/internal ledgers in public request paths, it could violate cache-first serving. Generation must happen through artifact production paths, not on demand during MCP serving.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The markdown report is an information product for agents, but it currently follows the visual website layout without the interpretive scaffolding that makes the website understandable. The Behavior table is the clearest failure: it mixes size, deltas, cadence, rollups, and filtered bootstrap behavior, then labels the display with a bucket that may be smaller than the feed's cadence.

Evidence reviewed:

- `configs/templates/markdown/feed.md.tmpl` currently labels one mixed section `Behavior` and provides only a short note.
- `pkg/markdown/rollup.go` chooses the first row-count-fitting interval from `1h`, `1d`, `1w`, `1mo`.
- `pkg/engine/query.go` documents that public changesets drop the bootstrap row.
- `ui/src/components/feed-detail/section-behavior.tsx` has explicit user-facing interpretation and separates four behavior charts.
- `ui/src/components/feed-detail/section-retention.tsx` explains current age and age-at-removal separately.

Affected contracts and surfaces:

- Generated feed markdown artifacts under the public web artifact directory.
- MCP `fetch_analysis` text output.
- Markdown template and context structs in `pkg/markdown`.
- Markdown rollup behavior and tests.
- Website/MCP specs and operator API docs describing markdown output.

Existing patterns to reuse:

- Template-driven markdown generation through `configs/templates/markdown/feed.md.tmpl`.
- `FeedArtifactReader` as the artifact reader for markdown context.
- Existing public methodology text as the source for compact interpretation copy, rewritten for markdown.
- Current markdown tests under `pkg/markdown`.

Risk and blast radius:

- Public markdown output shape will change; callers that parse sections by exact heading may need to adjust.
- Larger markdown output can increase MCP payload size. The implementation should keep explanations compact and avoid dumping full methodology pages.
- Reading internal ledgers from public request handlers would violate cache-first serving. The fix must keep serving cheap and use published artifacts for behavior reporting.
- Reintroducing bootstrap or first retained changesets as a lifecycle event can mislead models for migrated feeds. Markdown must not label retained rows as initial publication.

Sensitive data handling plan:

- This work uses public feed metadata and local file paths only for evidence. Durable artifacts will not include secrets, credentials, bearer tokens, SNMP communities, customer data, personal data, private endpoints, or proprietary incident details.

Implementation plan:

1. Redesign markdown behavior context as a website-aligned `## Behavior` section that uses published bounded artifacts.
2. Add compact section explainers and rollup notes to the markdown template.
3. Adjust rollup selection so display buckets are semantic and never smaller than observed/configured cadence.
4. Update tests for bootstrap visibility, cadence-safe buckets, and explanation text.
5. Update specs/docs for the agent-readable markdown contract.

Validation plan:

- Run focused `go test ./pkg/markdown`.
- Run broader related `go test ./pkg/config ./pkg/markdown ./pkg/engine ./pkg/mcp`.
- Run `git diff --check`.
- Rebuild markdown artifacts and spot-check problematic feeds, especially `uninvited_activity`.

Artifact impact plan:

- AGENTS.md: no expected update; project-wide workflow is unchanged.
- Runtime project skills: no expected update unless a reusable lesson emerges.
- Specs: update website/MCP or relevant markdown artifact contract.
- End-user/operator docs: update MCP endpoint docs if `fetch_analysis` output semantics change.
- End-user/operator skills: none expected.
- SOW lifecycle: complete this SOW with the implementation commit; future evaluator harness remains deferred until enrichment lands.

Open-source reference evidence:

- None checked. This is a project-specific generated-analysis contract; comparable OSS reference implementations would not define this product's feed semantics.

Open decisions:

- Resolved: implement markdown improvements now; defer the model-evaluation harness until enrichment is integrated.
- Resolved: future evaluation should use a common weak but reasonably smart local model, likely `qwen3.6-35b-a3b`, with one isolated question per fresh context.

## Implications And Decisions

1. Markdown contract scope
   - Decision: build a self-contained analysis report now, not a full evaluator.
   - Implication: generated markdown becomes immediately more useful and less contradictory, while evaluator-specific work waits for the enriched data shape.

2. Retained ledger handling
   - Decision: do not expose initial publication inferred from retained ledgers.
   - Implication: models can use `Tracked since` for monitoring start and treat behavior rows as retained movement evidence only.

3. Evaluation method for later work
   - Decision: use isolated one-question contexts with a weaker local model.
   - Implication: failures are more likely to reveal missing markdown context instead of being masked by multi-question reasoning.

## Plan

1. Inspect markdown context/data availability and identify which facts can be stated safely from published bounded artifacts.
2. Replace the ambiguous Behavior table with a website-aligned behavior section and explicit retained-window notes.
3. Make rollup selection cadence-aware and emit rollup explanation notes.
4. Add compact interpretation text for AS, geo, critical infrastructure, bogons, behavior, retention, overlap, and technical specs where needed.
5. Update specs/docs and tests, then validate and rebuild artifacts.

## Execution Log

### 2026-05-16

- Created SOW for the agent-readable markdown redesign after committing the prior behavior-table cleanup.
- Added publication lifecycle, post-baseline change activity, and observed cadence contexts for feed markdown.
- Passed the engine `LibDir` into markdown generation so the static generation path can include the internal bootstrap changeset without making MCP/public serving read internal ledgers.
- Replaced the ambiguous Behavior section with explicit lifecycle, activity, and cadence markdown sections.
- Added compact interpretation text for critical infrastructure, ASN attribution, geographic attribution, bogons, retention, overlap, and technical specifications.
- Updated markdown rollup behavior so raw post-baseline changes are used when compact enough; larger activity windows roll up to buckets no smaller than configured or observed cadence.
- Updated website spec and MCP endpoint docs for the self-contained markdown analysis contract.

## Validation

Acceptance criteria evidence:

- Feed markdown renders `## Behavior` and no longer renders `## Publication Lifecycle`, `## Change Activity`, or `## Observed Cadence`.
- Feed markdown does not render `Initial publication` or `post-baseline`.
- `/opt/update-ipsets/web/blocklist_de.md` keeps `Tracked since 2015-06-07 20:56 UTC` in the header.
- `/opt/update-ipsets/web/blocklist_de.md` explains that behavior uses bounded public history and changeset artifacts and is not a full lifetime ledger.
- A generated-artifact scan found no remaining `## Publication Lifecycle`, `## Change Activity`, `## Observed Cadence`, `Initial publication`, or `post-baseline` strings in generated markdown artifacts.

Tests or equivalent validation:

- `go test ./pkg/markdown` passed.
- `go test ./pkg/engine ./pkg/mcp` passed.
- `git diff --check` passed.

Real-use evidence:

- `./install.sh` completed successfully after final template changes.
- Admin reprocess was triggered with `reprocess=true` and completed.
- `curl http://localhost:18888/healthz` returned `ok`.

Reviewer findings:

- No external reviewer was run; the user did not request one for this implementation.

Same-failure scan:

- `rg -n "## Publication Lifecycle|## Change Activity|## Observed Cadence|Initial publication|post-baseline" /opt/update-ipsets/web/*.md /opt/update-ipsets/web/countries/*.md /opt/update-ipsets/web/asns/*.md /opt/update-ipsets/web/maintainers/*.md` returned no matches after regeneration.

Sensitive data gate:

- Durable artifacts contain no raw secrets, credentials, bearer tokens, SNMP communities, community member names, customer names, personal data, non-private customer-identifying IPs, private endpoints, or proprietary incident details.

Artifact maintenance gate:

- AGENTS.md: no update needed; project workflow and guardrails are unchanged.
- Runtime project skills: no update needed; this work did not add a reusable agent workflow rule beyond existing content-surface discipline.
- Specs: updated `.agents/sow/specs/website.md`.
- End-user/operator docs: updated `docs/api/mcp-endpoint.md`.
- End-user/operator skills: no update needed; no exported operator skill changed.
- SOW lifecycle: this SOW is marked `completed` and will be moved to `.agents/sow/done/` with the implementation commit.

Specs update:

- Updated `.agents/sow/specs/website.md`.

Project skills update:

- No runtime project skill update needed.

End-user/operator docs update:

- Updated `docs/api/mcp-endpoint.md`.

End-user/operator skills update:

- No end-user/operator skill update needed.

Lessons:

- Agent-facing markdown must not simply mirror visual website layout. If the report is intended for model reasoning, every analytical section needs compact local interpretation and explicit limits.

Follow-up mapping:

- Future model-evaluation harness deferred until enrichment is integrated.

## Outcome

Implemented. Feed markdown is now a self-contained analysis report that keeps the website-aligned `## Behavior` section, explains retained artifact limits, and avoids unsupported lifecycle claims.

## Lessons Extracted

- Do not infer first publication from retained history or changeset ledgers. `Tracked since` is the monitoring-start metadata, while behavior artifacts are bounded movement evidence.
- Rollup resolution must be semantic and explained; row-count-only bucket choice can create misleading apparent precision.
- Model-evaluation harness work should wait until enrichment lands, then test one isolated question per fresh model context.

## Followup

Future evaluator harness after enrichment integration.

## Regression Log

### Regression - 2026-05-17

Issue:

- The implemented markdown introduced `## Publication Lifecycle`, `## Change Activity`, and `## Observed Cadence` sections that do not exist on the public feed-detail page.
- The generated `Publication Lifecycle` section labeled the first internal changeset row as `Initial publication`.
- That label is not generally true for old feeds or imported ledgers. Feed monitoring start is tracked by feed metadata (`StartedDate` / `Tracked since`), while feed-level `history.csv` and `changesets.csv` are evolution ledgers and do not prove the first-ever publication.

Evidence:

- `pkg/markdown/context_feed.go` labeled the first internal changeset row as `Initial publication`.
- `configs/templates/markdown/feed.md.tmpl` rendered the invented lifecycle/change/cadence section split.
- `ui/src/components/feed-detail/section-behavior.tsx` presents the website behavior surface as four charts over the last 500 recorded runs.
- `ui/src/components/feed-detail/section-specs.tsx` exposes monitoring start separately as `Tracked since`.
- `blocklist_de` local artifacts show the first `history.csv` row is earlier than the first `changesets.csv` row, proving the first changeset is not a safe initial-publication timestamp.

Corrected requirements:

- Feed markdown must keep `Tracked since` as the monitoring-start statement and must not infer first publication from history or changeset ledgers.
- Feed markdown must align its movement section with the website behavior semantics: last recorded-run window, size evolution, churn, cadence, and added/removed updates.
- Markdown may remain more textual than the website charts, but section names and interpretation must not invent a lifecycle concept absent from the website.
- Markdown generation must use the published bounded history and changeset artifacts for behavior reporting, not internal full ledgers.

Validation plan:

- Update markdown code and tests so no feed markdown renders `## Publication Lifecycle`, `Initial publication`, or `post-baseline`.
- Verify generated sample markdown shows `## Behavior` with bounded-artifact wording and still keeps `Tracked since` in the header.
- Run focused markdown tests and `git diff --check`.

Resolution:

- Removed markdown reading of internal changesets and removed the engine `LibDir` dependency from markdown generation.
- Restored the `## Behavior` section and aligned it with the website's retained behavior-window semantics.
- Added retained-window wording so models know public behavior artifacts are bounded and not a full lifetime ledger.
- Kept monitoring start in the existing header `Tracked since` field.

Regression validation:

- `go test ./pkg/markdown` passed.
- `go test ./pkg/engine ./pkg/mcp` passed.
- `git diff --check` passed.
- `./install.sh` completed successfully.
- Admin reprocess with `reprocess=true` completed.
- `rg -n "## Publication Lifecycle|## Change Activity|## Observed Cadence|Initial publication|post-baseline" /opt/update-ipsets/web/*.md /opt/update-ipsets/web/countries/*.md /opt/update-ipsets/web/asns/*.md /opt/update-ipsets/web/maintainers/*.md` returned no matches.
- `/opt/update-ipsets/web/blocklist_de.md` shows `Tracked since 2015-06-07 20:56 UTC` and a `## Behavior` section with bounded-artifact wording.

Regression artifact maintenance:

- AGENTS.md: no update needed; project workflow and guardrails are unchanged.
- Runtime project skills: no update needed; no reusable workflow rule changed.
- Specs: updated `.agents/sow/specs/website.md`.
- End-user/operator docs: updated `docs/api/mcp-endpoint.md`.
- End-user/operator skills: no update needed.
- SOW lifecycle: reopened regression fixed; this SOW is marked `completed` and moved back to `.agents/sow/done/` with the implementation commit.
