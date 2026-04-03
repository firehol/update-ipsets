# SOW-0014 | 2026-04-26 | ai-in-the-loop

## Status

Status: completed

Sub-state: scope concretized 2026-05-13 after user discussion. Original SOW-0014 was a placeholder postponed pending discussion. This revision replaces the abstract content with a concrete enrichment plan, aligned with the refined "facts-not-labels" rule that permits clearly-labeled AI-researched sections.

Sub-state update (2026-05-24): enrichment agent artifacts exist in the working tree but are not committed by this SOW yet. Local validation evidence shows 337 latest third-party enrichment runs are clean and 20 static-generated FireHOL-maintained enrichments are clean. The previously queued 30 third-party runs were stale `validation-report.json` files; rerunning validation against current `output.json` files passed and the stale reports were regenerated. Integration work (engine reader, API, UI, markdown) is re-scoped under **Plan v2 (2026-05-24)**. Evaluation agent work is deferred to `.agents/sow/pending/SOW-0091-feed-evaluation-agent.md`. Several historical decisions are superseded by v2 — see annotations under each affected D-entry.

## Requirements

### Purpose

Enrich every public feed page on iplists.firehol.org with **researched facts** (who maintains it, how IPs are listed/unlisted, how to request unlist, maintainer's stated cadence, maintainer's stated detection method, historical community observations, strengths, weaknesses, and limitations). The system must be transparent, neutral in tone, respectful of maintainers, and provably free of feedback loops with iplists.firehol.org itself.

The separate AI Evaluation section from the original request is out of scope for SOW-0014 and is tracked by `.agents/sow/pending/SOW-0091-feed-evaluation-agent.md`.

### User Request (verbatim)

> Enrich feed metadata with ai web research to find:
> - who is the maintainer/owner
> - how IPs get listed into this feed
> - how IPs get unlisted from this feed
> - how people should request to unlist an IP
> - what is the optimal update frequency of the feed
> - community feedback (reddit, blogs, forums, github issues, etc)
> - classify detection as active/passive/whatever
> - Strengths/Weaknesses+Limitation
>
> The whole point is the above should be considered generally stable information. iplists.firehol.org should be excluded from this analysis - so the assistant should reject feedback loops - it should find all other information available online about the feed, excluding anything related/derived from iplists.firehol.org.
>
> The above should enrich the feed page, so the output of the assistant must be json.
>
> Then we need another agent, to take the feed markdown page (which should now include the above information - as the website does), and evaluate the feed itself. Like a new section about "AI Evaluation" in the feed page, which of course should not be given to the ai assistant that will redo the evaluation (different markdown template?).

Refinement (2026-05-13): "what other people say should be generalized as 'past complaints / pain points / weaknesses' — we will never know if these have been addressed or not, and we need to keep a neutral tone, but still surface what has actually happened, without directly criticizing/offending the maintainers for their hard work. So, everything should still remain transparent and objective and as neutral as possible."

### Assistant Understanding

Facts:

- `netdata/ai-agent @ c7356335` provides the agent-authoring reference at `.agents/skills/project-ai-agent-authoring/SKILL.md`.
- `netdata/ai-agent @ c7356335` has an existing deep-research agent at `neda/web-research.ai` using `nova/neda-thinker` plus web, GitHub, Reddit, and Sourcegraph search/fetch tools. It is invokable via MCP as `mcp__neda__web-research`.
- Precedent for JSON-schema research output: `netdata/ai-agent @ c7356335` files `neda/netdata-logs.ai`, `neda/company.ai`, and `neda/company-quick-check.ai`.
- update-ipsets feed catalog source of truth: per-feed YAML at `configs/firehol/sources/<category>/<feed>.yaml` (`maintainer`, `maintainer_url`, `license`, `info`, `attribution`, `frequency` already present).
- Engine exposes feeds via `pkg/engine/public_catalog.go:PublicFeedSummary`; markdown context via `pkg/markdown/context.go:FeedPageContext`; user-facing markdown template at `configs/templates/markdown/feed.md.tmpl` (187 lines, 11 sections + hero).
- Current spec `.agents/sow/specs/feeds.md` "What the product knows about a feed" enumerates the canonical fact families (catalog meaning, acquisition state, processing state, observed behavior, derived analysis, legal/publication policy). No enrichment family exists yet.
- Memory `feedback_facts_not_labels` (refined 2026-05-13) now permits clearly-labeled AI sections with neutral tone; community signal must be framed as historical observations with sources.
- The MCP tool `fetch_analysis(feed, name)` already serves `web/{feed}.md` (closed in SOW-0085 regression on 2026-05-13). Adding new sections to that markdown propagates to the MCP surface automatically.

Inferences:

- "Maintainer/owner" is partially a maintainer-level attribute (one record per maintainer, shared across feeds) and partially feed-level (the listing/unlisting policy varies per feed even from the same maintainer). Best modeled as TWO enrichment artifacts: per-maintainer + per-feed.
- Merged feeds (firehol_level1..4, ipsum_N) have no upstream maintainer/listing-policy/community of their own — their character is the union of their components. Enrichment of merges should be either skipped or auto-composed from component enrichments rather than re-researched.
- Refresh cadence: "generally stable" + reality of LLM cost + risk of frequent drift → operator-triggered, scope-bounded enrichment refresh is safer than cron. Evaluation cadence is out of scope for this SOW and belongs to SOW-0091.
- Feedback-loop guard: prompt-level rejection alone is not sufficient (LLMs sometimes ignore negative instructions); add a URL-level denylist (`iplists.firehol.org`, `firehol.org/ipsets`, `github.com/firehol/blocklist-ipsets`) applied at the web-fetch/web-search tool layer if the framework allows, otherwise enforced by the wrapper script that post-validates source URLs in the JSON output.
- Storage location is resolved by D11 and D19: SOW-0014 embeds a sanitized public enrichment projection inside the existing per-feed YAML, while full agent outputs remain local run artifacts.
- Evaluator input design is out of scope for SOW-0014 and tracked by SOW-0091.

Unknowns (resolved by 2026-05-24):

- ~~Which model/provider should the enrichment agent use?~~ — RESOLVED. Enrichment uses `nova/neda-thinker` (D7). Evaluation is deferred to SOW-0091 (D14).
- ~~Detection classification taxonomy?~~ — RESOLVED. Closed enum locked in D6 and embedded in schema v2.
- ~~Whether AI output goes live immediately or to an admin-UI review queue?~~ — RESOLVED. Plan v2 uses a PR-based gate (D13); admin queue not needed.

Plan v2 unknowns added 2026-05-24:

- Multi-source YAML files contain feeds with subtly-correlated configs (shared categories, similar `critical:` blocks). The split must preserve byte-for-byte engine output; the smoke-test (step 0 validation) is what proves it.
- The YAML round-trip library (ruamel.yaml) preserves comments and key order in most cases but not all. Step 2 conversion must be tested against a handful of files with comments before running on all 357.
- `<FeedRef>` tooltip discovery (step 4): need to inventory every UI surface that references a feed name to ensure none are missed.

### Acceptance Criteria

Original criteria (covering enrichment agent build and end-to-end pilot) are preserved below as **§A** after the 2026-05-24 repair. Plan v2 (integration phase) extends them with **§B**. Both sets must pass for the SOW to close.

#### §A — Original (enrichment agent build)

1. Enrichment ai-agent `.ai` file at `agents/feed-enrichment.ai` (in-repo per D8) producing schema-valid JSON output per feed. The evaluation agent is tracked separately by SOW-0091.
2. Agent includes explicit feedback-loop guards excluding `iplists.firehol.org` and related FireHOL-published domains; wrapper post-validates that no cited source URL matches the denylist.
3. Merged feeds and other FireHOL-maintained feeds either skip enrichment (wrapper refuses) or have enrichment generated deterministically by `tools/build-firehol-static-enrichment.py` (D5).
4. Wrapper script `agents/run-enrichment.sh` invokes the agent, validates JSON output against the schema, enforces the denylist, and writes the run artifact under `.local/agents/feed-enrichment/<feed>/<UTC>/`.
5. End-to-end run on every eligible third-party catalog feed; outputs schema-clean and validator-clean.

Status (2026-05-24): §A validation evidence is clean locally after stale report regeneration, but §A is not complete in repository terms until the agent artifacts are committed by this SOW. Local evidence:

- 337 latest third-party enrichment runs have clean `validation-report.json`.
- 20 static-generated FireHOL-maintained enrichments have clean `output.validation-report.json`; these are intentionally generated, not missing.
- `agents/run-enrichment-pool.sh --unenriched --retry-failed --dry-run` queues no feeds after stale report regeneration. The script reports the empty queue with exit code 2 and the message `no feeds queued`.

#### §B — Plan v2 (integration phase)

1. The 3 multi-source YAML files under `configs/firehol/sources/provider_infrastructure/` are split into 26 individual per-feed files; engine smoke-test confirms identical output before/after.
2. Manual config-correction pass complete: every per-feed YAML has its engine-config fields (`maintainer`, `maintainer_url`, `frequency`, `license`, `redistributable`, and any obvious wrongs) reconciled against enrichment-researched values. Per-feed delta report is the audit artifact.
3. Every per-feed source or merge YAML carries an `enrichment:` block populated from the latest clean agent or static-generator run, containing the public embedded enrichment projection only. The public embedded-enrichment schema validator passes.
4. Engine reads the `enrichment:` block at startup; exposes new fields:
   - `/api/v1/sets/{name}` returns the full enrichment payload + `current_status`.
   - `/api/v1/sets` list endpoint adds `short_description`, `official_name`, `current_status.state` per feed.
5. Markdown template extended with the new sections grouped by operator question:
   - **About this feed** (long_description, derivation with source_feeds[] cross-references, detection_classification, scope_and_intent, roles) — replaces the old one-line "About" section.
   - **Listing rules and how to request removal** (listing_policy, unlisting_policy, unlist_request).
   - **Reputation and community signals** (community).
   - **Sources consulted** footer + "Last researched" date + AI-research disclaimer.
6. UI feed page mirrors the markdown sections; collapsible cards for the long sections; reusable `<FeedRef>` tooltip component used wherever a feed name appears in lists/tables/cross-references; status banner above hero for `current_status != active`.
7. Feed-discovery markdown (MCP catalog) carries `short_description` inline for every feed entry; maintainer label uses the corrected source YAML.
8. `static/feed-descriptions/*.html` and the `/api/v1/sets/about/{name}` handler are removed; enrichment is the source of truth for descriptive content.
9. Internal-only enrichment fields (`maintainer_quotes`, `assistant_reasoning`, `confidence`, `evidence_ids`, `evidence[].description`) are stripped at conversion time (step 2) and again defensively at the reader; never reach the API or any rendered surface.
10. Wrapper script supports operator-scoped runs: `--feeds a,b,c`, `--category <name>`, `--all`. Successful runs write `enrichment:` blocks back to the per-feed YAMLs and create a local branch plus an auto-generated significant-change summary. When a remote and `gh` are configured, the wrapper also opens one PR per run via `gh pr create`, titled `Enrichment refresh: <scope> (<count> feeds)`.
11. Methodology page added explaining the AI-research process, refresh cadence, and what the disclaimer means.
12. `.agents/sow/specs/feeds.md` (knowledge model) and `.agents/sow/specs/files-layout.md` (per-feed YAML structure) updated to document the embedded enrichment.
13. `make build`, `make test`, `make race`, `make lint`, `pnpm --dir ui build`, `pnpm --dir ui lint` all pass.
14. End-to-end on a representative feed (e.g., `spamhaus_drop`): UI feed page renders all new sections; MCP `fetch_analysis` returns the enriched markdown; `<FeedRef>` tooltips work on at least three different surfaces (homepage tiles, overlap table rows, merge composition rows).
15. Public enrichment prose preserves markdown where fields are markdown-capable, and the initial embedded catalog passes a prose-hygiene review that flags wall-of-text fields before they are committed/rendered.

## Analysis

Sources checked:

- `netdata/ai-agent @ c7356335` — `.agents/skills/project-ai-agent-authoring/SKILL.md` (framework reference)
- `netdata/ai-agent @ c7356335` — `neda/web-research.ai` (existing deep-research agent pattern)
- `netdata/ai-agent @ c7356335` — `neda/company.ai`, `neda/company-quick-check.ai`, `neda/netdata-logs.ai` (JSON-schema research precedents)
- `pkg/engine/public_catalog.go:12-60` (PublicFeedSummary fields)
- `pkg/markdown/context.go:4-50` (FeedPageContext fields)
- `configs/templates/markdown/feed.md.tmpl` (current 11-section template)
- `configs/firehol/sources/policy_risk/spamhaus_drop.yaml` (sample feed config)
- `.agents/sow/specs/feeds.md:124-172` (knowledge model the engine maintains per feed)
- `.agents/sow/pending/SOW-0014-...md` (original abstract version of this SOW)
- Memory: `feedback_facts_not_labels` (refined 2026-05-13)

Current state:

- Enrichment agent artifacts exist in the working tree (`agents/feed-enrichment.ai`, `agents/schemas/feed-enrichment.schema.json`, wrappers, validators, and static generator), but they are untracked/uncommitted by this SOW.
- AI integration into the engine/public surfaces is still zero: nothing in update-ipsets reads `enrichment:` blocks from source YAMLs, no API or markdown context exposes them, and no UI component renders them.
- The MCP server in update-ipsets is read-only and inbound (serves `find_feeds` + `fetch_analysis` to external AI clients); this SOW adds an opposite direction: external AI agents producing enrichment for update-ipsets
- Latest local enrichment validation state on 2026-05-24: 337 clean latest third-party runs, 19 clean static-generated FireHOL-maintained runs, and 0 current validation failures.

Risks:

- **AI hallucination on listing/unlisting policy** — the agent might invent a process that doesn't exist. Mitigation: schema requires the agent to cite the source URL for every claim; if no source can be found, the field is `null` rather than fabricated.
- **Feedback loop with iplists.firehol.org** — even with prompt rejection, the agent might consume blog posts that summarize firehol_levelN. Mitigation: denylist enforced at TWO layers (prompt + wrapper post-validation).
- **Maintainer offense** — even neutral framing of past complaints can be received badly by maintainers. Mitigation: explicit prompt rule against speculation about maintainer competence; "complaints" rendered as quoted snippets with sources + dates only; visible AI-generated disclaimer.
- **Refresh cost** — full-catalog enrichment is expensive enough that refresh must be operator-triggered and scope-bounded, not cron-driven.
- **Public criticism of an active company** — surfacing past GitHub issues from 2019 about a still-active commercial feed could trigger a takedown request. Mitigation: include a per-feed `enrichment.suppress_complaints: true` opt-out in the feed YAML for cases where the user decides to hide complaints; document the policy.
- **Schema drift** — once the schema is locked, changing it requires re-running every enriched feed. Mitigation: version the schema (`enrichment_schema_version: 2` for current full agent output) and define a separate public embedded-enrichment schema for YAML storage.
- **Stale evaluation** — deferred to SOW-0091; this SOW must not design or ship evaluation output.

## Pre-Implementation Gate

Status: ready after SOW repair (Plan v2 decisions D11-D21 resolved 2026-05-24; original D1-D10 either still valid or superseded as annotated). First execution step must confirm enrichment validation freshness before embedding enrichment into source YAML.

Problem / root-cause model:

- update-ipsets is a great factual catalog but has thin contextual information about each feed beyond what fits in the YAML `info:` field. Operators and downstream users can't easily learn how a feed is built, how to engage with the maintainer, or what others have reported. Web research can supply this once and refresh it slowly; the engine just needs a place to put the answers and a section in the markdown to render them.

Evidence reviewed:

- See `## Analysis → Sources checked` above
- ai-agent framework supports the required shape (JSON output, MCP tools, deep-research agent precedent, schema validation)
- update-ipsets already has the integration points (PublicFeedSummary, FeedPageContext, feed.md.tmpl) — these need extension, not invention

Affected contracts and surfaces (Plan v2):

- Refactored: `configs/firehol/sources/provider_infrastructure/critical_dns.yaml`, `critical_provider_ranges.yaml`, `critical_service_ranges.yaml` — split into 26 individual per-feed YAMLs (D15).
- Extended: every source or merge YAML entry under `configs/firehol/` — gains an `enrichment:` block under `sources: <feed>:` or `merges: <feed>:` (D11). 357 feed/merge entries touched after multi-source files are split.
- Corrected: every `configs/firehol/sources/<category>/<feed>.yaml` — engine-config fields reconciled against enrichment (D18). Touches a subset of feeds.
- New (Go): `pkg/enrichment/` (types + loader + validator for the public embedded-enrichment schema).
- Extended: `pkg/engine/public_catalog.go:PublicFeedSummary` — optional `Enrichment` + `CurrentStatus` fields.
- Extended: `pkg/markdown/context.go:FeedPageContext` — optional `Enrichment` field.
- Extended: catalog list endpoint (`/api/v1/sets`) — per-feed entries gain `short_description`, `official_name`, `current_status_state`.
- Extended: `configs/templates/markdown/feed.md.tmpl` — sections per §B-5; old "About" replaced.
- Extended: MCP feed-discovery markdown template — `short_description` + maintainer inline per feed reference.
- Extended: `ui/src/pages/feed-detail.tsx` and child components — new sections per §B-6; status banner; tooltips.
- New (UI): reusable `<FeedRef>` tooltip component used wherever a feed name is referenced (homepage tiles, overlap table, merge composition table, search results, cross-references inside enrichment fields).
- Removed: `static/feed-descriptions/*.html` files (D16).
- Removed: `/api/v1/sets/about/{name}` route + handler (D16).
- Extended: `agents/run-enrichment-pool.sh` — `--category` selector; after-run hook that writes back to per-feed YAMLs, creates a local branch and summary, and opens a PR only when a remote and `gh` are configured (D12, D13, D20).
- New (tool): conversion script (Plan v2 step 2) reading `.local/agents/feed-enrichment/<feed>/<UTC>/output.json` and writing the `enrichment:` block to the per-feed YAML.
- New (tool): config-correction delta-report generator (Plan v2 step 1).
- Present in the working tree and to be committed by this SOW: `agents/feed-enrichment.ai` + `agents/schemas/feed-enrichment.schema.json` + `agents/run-enrichment.sh` + `agents/run-enrichment-pool.sh` + `agents/locate-feed.py` + `agents/validate-output.py` + `tools/build-firehol-static-enrichment.py`.
- Extended: `pkg/web/static/methodology/` — new methodology page on AI research (per `feedback_methodology_transparency`).
- Extended spec: `.agents/sow/specs/feeds.md` — add "AI-enriched context" knowledge family.
- Extended spec: `.agents/sow/specs/files-layout.md` — document the per-feed source/merge YAML's `enrichment:` block.
- Deferred: evaluation agent + its UI surface (D14) — `.agents/sow/pending/SOW-0091-feed-evaluation-agent.md`.

Existing patterns to reuse:

- `pkg/markdown/context_feed.go:readFeedMetadata` — JSON-from-disk pattern for extending context from new artifact files
- staged publish and artifact registration patterns where generated public files are still produced; embedded enrichment itself is stored in source YAML per D11.
- `netdata/ai-agent @ c7356335` `neda/web-research.ai` agent structure — model fallback chain, tool wiring, reasoning level, cache
- `netdata/ai-agent @ c7356335` `neda/netdata-logs.ai` output schema structure
- `feedback_methodology_transparency` rule — every computed thing needs a published methodology page

Risk and blast radius:

- Public AI text on production site: medium-to-high reputational risk; mitigated by labeling + neutral framing + maintainer opt-out
- Per-feed YAML churn: every source file gains an `enrichment:` block; review diffs will be large.
- LLM cost: bounded by operator-triggered scoped runs.
- Performance: negligible — file reads at startup + on cycle, no LLM call during request handling
- Regression: zero if the new fields are additive (omitempty in JSON, optional in template)
- Migration: none — net-new artifact class

Sensitive data handling plan:

- Enrichment outputs are public-by-design (rendered on iplists.firehol.org). They MUST NOT contain anything sensitive — but the AI might quote a GitHub issue that includes an IP address belonging to a real user. Mitigation: agent prompt forbids quoting IPs found in linked discussions; validator rejects IPv4/IPv6 in public fields; conversion to YAML must refuse unclean output rather than silently stripping evidence.
- LLM provider API keys: live in `~/.ai-agent/ai-agent.env`, never in this repo.
- Source URLs may include private GitHub repos if the maintainer is on a private org — wrapper rejects non-200 URLs and any URL whose hostname matches private patterns (e.g., `*.internal`, `localhost`).
- SOWs, specs, docs, project skills, agent instructions, code comments must not paste raw enrichment payloads with real maintainer names in this SOW (they will appear in commits naturally via the YAML files). Use `<maintainer>` placeholders in examples within SOW/spec text.

Implementation plan:

See the **Plan v2 (2026-05-24 — current)** section above for the authoritative step-by-step. The original Plan v1 implementation list is preserved in the **Plan v1** section as historical record; agent artifacts and local validation evidence exist, but full backfill is not complete in repository terms until the artifacts are committed and embedded into source YAML. The remaining work is described in Plan v2 steps -1 through 8.

Validation plan (Plan v2):

- Engine smoke-test before/after Plan v2 step 0 (multi-source split): catalog feed count identical; per-feed `.ipset`/`.netset` outputs byte-identical; integrity checks unchanged.
- Public embedded-enrichment schema validator runs over every `enrichment:` block in the per-feed YAMLs after Plan v2 step 2.
- Public markdown/prose hygiene report runs over every public markdown-capable enrichment field after Plan v2 step 2; flagged wall-of-text fields are polished in YAML before the initial integration is considered review-ready.
- Go unit tests for the enrichment loader: feed with `enrichment:` present + valid, present + malformed, absent (nil fallback), present + containing forbidden internal fields (must be stripped or rejected).
- API contract tests: `/api/v1/sets` list endpoint carries `short_description`, `official_name`, `current_status_state`; `/api/v1/sets/{name}` carries the full enrichment payload; never carries internal fields.
- Markdown template golden-file tests covering: feed with full enrichment, feed with no enrichment (fallback), discontinued feed (status banner), merge with composed enrichment.
- UI behavioral tests: sections render only when data is present; status banner visible on discontinued feeds; `<FeedRef>` tooltip visible on hover across the three required surfaces (homepage tiles, overlap rows, merge composition rows); old `static/feed-descriptions/*.html` 404s gracefully.
- Wrapper branch/PR flow (step 6): dry-run produces correct branch name and summary; live run against a small `--feeds` selector creates a local branch and summary; PR creation is tested only when a remote and `gh` are configured.
- Real-use evidence: end-to-end on `spamhaus_drop` (well-documented, low-controversy feed); MCP `fetch_analysis` returns the enriched markdown; cross-feed `<FeedRef>` tooltips work on its overlap rows.

Artifact impact plan:

- AGENTS.md: no update (workflow unchanged; new files but no new rules)
- Project skills: update or create a project skill only if implementation surfaces durable rules worth codifying.
- Specs: `feeds.md` (new knowledge families), `files-layout.md` (new enrichment/evaluation paths)
- End-user/operator docs: new operator page on `docs/api/ai-research.md` or similar; operator-triggered refresh examples.
- End-user/operator skills: update or create only if implementation produces a durable operator workflow for refreshing enrichment.
- SOW lifecycle: this SOW replaces the abstract original SOW-0014; move from pending → current upon approval

Open-source reference evidence:

- `netdata/ai-agent @ c7356335` — `.agents/skills/project-ai-agent-authoring/SKILL.md`, `neda/web-research.ai`, `neda/company.ai`, `neda/netdata-logs.ai`.

Open decisions:

None. See `## Implications And Decisions` below — 21 numbered points (D1-D10 from the original plan; D11-D21 added in Plan v2 on 2026-05-24).

## Implications And Decisions

All decisions resolved (D1 on 2026-05-13; D2-D10 on 2026-05-14; D11-D21 on 2026-05-24). Decisions superseded by Plan v2 are annotated under their original entry and below in the **Plan v2 decisions** subsection.

**D1. Editorial scope — RESOLVED 2026-05-13: B with refinement.**
- AI sections allowed, clearly labeled, with disclaimer
- Community signal framed as "past complaints / pain points / weaknesses" — neutral, historical, with sources
- Never criticize/offend maintainers
- Recorded in memory `feedback_facts_not_labels` (updated)

**D2. Storage — RESOLVED 2026-05-14: option 1. SUPERSEDED 2026-05-24 by D11.**
- ~~Two committed YAML files per primary feed: `configs/firehol/enrichment/<feed>.yaml` + `configs/firehol/evaluation/<feed>.yaml`.~~
- Superseded reason: configuration duality between source-YAML and enrichment-YAML breeds discrepancies. Plan v2 embeds enrichment into the existing per-feed YAML.

**D3. Cadence and trigger — RESOLVED 2026-05-14: option 1. SUPERSEDED 2026-05-24 by D12.**
- ~~Quarterly enrichment + weekly evaluation, both via cron / systemd-timer; admin API endpoints for manual trigger.~~
- Superseded reason: cron-driven refresh is operationally rigid and produces a single unreviewable mega-PR. Plan v2 makes refresh **operator-triggered and scope-bounded** (per feed, per category, or all).

**D4. Go-live model — RESOLVED 2026-05-14: option 1. SUPERSEDED 2026-05-24 by D13.**
- ~~Wrapper commits AI output directly to the committed YAML files; next config reload picks it up; human override = edit/revert in a PR.~~
- Superseded reason: direct commit bypasses review; quality regressions ship before anyone sees them. Plan v2 makes the wrapper create a **branch + summary per scoped run**, with PR creation when a remote is configured; review is the gate.

**D5. Merged feeds — enrichment — RESOLVED 2026-05-14: option 1.**
- Auto-compose enrichment for merges from the enrichment of their constituent feeds
- No LLM call for merges; the engine builds the composed payload deterministically
- Composition rules to be specified in implementation (likely: list of components, each with their key facts; no aggregation of community feedback)

**D5b. Merged feeds — evaluation — RESOLVED 2026-05-14. DEFERRED 2026-05-24 with D14 (evaluation agent itself is deferred).**
- ~~Run the evaluation agent on merges too.~~
- The merge-coverage question is reopened in the follow-up evaluation SOW alongside the agent itself.

**D6. Detection classification enum — RESOLVED 2026-05-14: option 1.**
Closed enum, schema-validated:
- `honeypot` — passive observation of attackers reaching maintainer's traps
- `network_telescope` — passive observation of background internet scanning
- `active_scanning` — maintainer actively probes the internet
- `user_submission` — humans submit IPs via a portal/API
- `malware_analysis` — extracted from malware C2 infrastructure analysis
- `reputation_aggregation` — aggregates other feeds with policy
- `policy_assignment` — manually curated based on policy (e.g., bogons, geo blocks)
- `commercial_threat_intel` — vendor-curated proprietary methodology
- `mixed` — multiple methods combined
- `unknown` — research could not determine

Each feed gets exactly one value.

**D7. Enrichment model — RESOLVED 2026-05-14: option 1.**
- Primary: `nova/neda-thinker` (local, same model `web-research.ai` uses)
- Fallback chain is configured in agent frontmatter and changes only through normal review.

**D7b. Evaluation model — RESOLVED 2026-05-14. DEFERRED 2026-05-24 to SOW-0091 (see D14).**
- The evaluation agent is out of scope for this SOW. Plan v2 ships only the enrichment integration. The evaluation agent prompt, schema, model, and UI surface are tracked in `.agents/sow/pending/SOW-0091-feed-evaluation-agent.md`.

**D8. Agent location — RESOLVED 2026-05-14: user override, in-repo.**
- Enrichment agent files live inside this repo under `agents/`.
- The evaluation agent will also live inside this repo when SOW-0091 is implemented.
- Schema files and any shared includes co-located: `agents/schemas/`, `agents/shared/`
- Implication: ai-agent runtime is invoked by path or via an operator-configured agents directory; install/service integration must be handled only if needed during implementation.
- Rationale: domain-specific to update-ipsets; single source of truth; ai-agent.git stays generic

**D9. Feedback-loop guard — RESOLVED 2026-05-14: option 1, two-layer non-optional.**
- (a) Prompt-level: explicit forbid in the agent system prompt for these patterns:
  - `iplists.firehol.org`
  - `*.firehol.org/ipsets*`
  - `github.com/firehol/blocklist-ipsets`
  - any content that cites the above as its source
- (b) Wrapper post-validation: after JSON returned, regex-match every `source_url` against the denylist; if any match, reject the entire output and retry (max 2) with a stronger warning
- Both layers non-optional; both must be implemented and tested

**D10. Original SOW-0014 disposition — RESOLVED 2026-05-13.**
- This SOW replaces the abstract original in-place. No separate close-out.

### Plan v2 decisions (2026-05-24)

**D11. Storage of enrichment payloads (supersedes D2). — RESOLVED 2026-05-24.**
- Enrichment is embedded inside the existing per-feed source or merge YAML, under a top-level `enrichment:` block at the same level as engine config keys: `sources: { <feed>: { url: ..., processor: ..., ..., enrichment: { enrichment_schema_version: 2, run_at: ..., short_description: ..., long_description: ..., roles: [...], ... } } }` or `merges: { <feed>: { sources: [...], ..., enrichment: { ... } } }`.
- One file per feed or merge. No separate `configs/firehol/enrichment/` directory.
- Multi-source YAML files (today: `critical_dns.yaml`, `critical_provider_ranges.yaml`, `critical_service_ranges.yaml` — 26 sub-feeds across 3 files) are split into one file per feed as a precondition (Plan v2 step 0).
- The agent never writes engine config keys; the wrapper's contract is "may only write under `enrichment:`". Round-trip YAML editing uses a library that preserves comments/key order (ruamel.yaml).
- The `.local/agents/feed-enrichment/<feed>/<UTC>/output.json` artifacts remain as the per-run audit archive; they are not the source of truth for serving.
- Rationale: avoid configuration duality. One place to look per feed or merge. Engine-config and AI-research live next to each other but in clearly separated regions. Multi-source files are unworkable for embedded enrichment (would balloon to 2,000+ lines).

**D12. Refresh trigger (supersedes D3). — RESOLVED 2026-05-24.**
- Refresh is **operator-triggered and scope-bounded**, never on a cron.
- The wrapper accepts: `--feeds a,b,c`, `--category <name>`, `--all`, or stdin pipe of feed names.
- Each scoped run produces one local branch and summary; it produces a PR only when a remote and `gh` are configured.
- Rationale: cost discipline, review cadence under operator control, recovery isolation, targeted updates without forcing a full-catalog refresh.

**D13. Go-live model (supersedes D4). — RESOLVED 2026-05-24.**
- Wrapper creates a local branch and summary per scoped run. If a remote and `gh` are configured, it also opens a PR via `gh pr create`; PR title is `Enrichment refresh: <scope> (<count> feeds)`; PR body is an auto-generated summary listing "significant" changes (maintainer rename, status change, license change, redistribution change) above the bulk diff.
- Merge or local branch review is the go-live gate. No staging area, no admin queue.
- Human overrides happen inside the PR review (edit before merge) or as a direct commit to `main` between runs; the next run produces a diff the reviewer resolves.
- Rationale: git review is the natural quality gate; the diff is the change-set; no additional review-queue UI to build.

**D14. Evaluation agent — DEFERRED to SOW-0091. — RESOLVED 2026-05-24.**
- The originally-planned `feed-evaluation.ai` agent and its UI surface are removed from this SOW's scope.
- `.agents/sow/pending/SOW-0091-feed-evaluation-agent.md` will design the evaluation agent against the as-built enrichment + markdown surfaces, so its input contract is concrete instead of speculative.
- Rationale: enrichment alone is a complete, valuable surface; coupling delays operator-visible improvement. User guidance: "the evaluation should get the entire markdown and provide some recommendations and a second opinion (what is this good for, what to be careful about, etc.)" — best designed after the markdown is stable.

**D15. Multi-source YAML files (new in v2). — RESOLVED 2026-05-24.**
- The 3 multi-source files under `configs/firehol/sources/provider_infrastructure/` are split into 26 individual per-feed files; `agents/locate-feed.py`'s one-file-many-feeds branch can stay as a safety net but should become unreachable in practice.
- Rationale: one feed = one file, in line with D11's "no duality" principle. Embedded enrichment is unworkable in multi-source files.

**D16. Hand-crafted per-feed HTML files (new in v2). — RESOLVED 2026-05-24.**
- `static/feed-descriptions/*.html` and the `/api/v1/sets/about/{name}` handler are removed once Plan v2 steps 4-5 ship. Enrichment replaces them with broader and more uniform coverage.
- Rationale: enrichment now covers all 357 feeds/merges with comparable structure; per-feed hand-crafted HTML files were available only for a small subset and produced inconsistent voice.

**D17. `short_description` propagation (new in v2). — RESOLVED 2026-05-24.**
- `short_description` follows the feed everywhere it is referenced — homepage tiles, comparison rows, merge composition rows, search results, cross-feed links inside enrichment fields, MCP catalog markdown.
- Implementation: one new field on the catalog list endpoint; one reusable `<FeedRef>` UI component; markdown renders the sentence inline next to every feed link.
- Rationale: highest-leverage single field — answers "what is this feed" without a click.

**D18. Per-feed config-correction pass (new in v2). — RESOLVED 2026-05-24.**
- Before embedding enrichment, the operator does a one-time pass to reconcile engine-config fields (`maintainer`, `maintainer_url`, `frequency`, `license`, `redistributable`) with enrichment-researched values where they disagree.
- The wrapper produces a per-feed delta report; the operator reviews and applies fixes manually. Not automated.
- Rationale: fixes silent YAML drift accumulated since 2014; ensures source YAML and embedded enrichment agree on the small set of duplicated facts.

**D19. Embedded payload shape (new in v2 repair). — RESOLVED 2026-05-24.**
- Per-feed YAML stores a sanitized public embedded-enrichment projection, not the full agent output schema.
- The full agent output remains in `.local/agents/feed-enrichment/<feed>/<UTC>/output.json` as local audit evidence during the run. The committed YAML keeps public fields and source URLs needed for transparency, but omits internal fields such as `maintainer_quotes`, `assistant_reasoning`, `confidence`, `evidence_ids`, and `evidence[].description`.
- The implementation defines and tests a separate public embedded-enrichment schema. It must not claim the stripped YAML block validates against the full `agents/schemas/feed-enrichment.schema.json`, because that full schema requires internal audit fields.
- Rationale: public serving needs a stable, safe projection; the full agent output is an internal verification artifact and should not be rendered or committed wholesale.

**D20. No-remote git workflow (new in v2 repair). — RESOLVED 2026-05-24.**
- This clone has no configured remote, so `gh pr create` is conditional.
- The wrapper must always create a local branch and significant-change summary. It opens a PR only when a remote and authenticated `gh` are available.
- Validation covers both the no-remote dry-run/local-branch path and, when configured, the PR path.

**D21. Public markdown and prose hygiene (new in v2 repair). — RESOLVED 2026-05-24.**
- Public enrichment fields that are defined as markdown-capable remain markdown when moved into source YAML. Conversion must preserve paragraph breaks and inline markdown; it must not flatten markdown to plain text.
- The converter must not auto-rewrite public prose for style. Instead, it produces a prose-hygiene report over public markdown-capable fields and blocks the initial integration review until flagged wall-of-text fields are manually polished in YAML.
- Initial hygiene flags:
  - any single paragraph over 650 characters;
  - any single paragraph over 4 sentences;
  - `long_description` over 900 characters with only one paragraph;
  - raw HTML in any public string;
  - markdown headings in inline-markdown fields where the website supplies the section heading.
- Hard safety validation remains separate and mandatory: schema validity, denylisted source URLs, and public IP literal rejection are failures, not style flags.
- Evidence: the current local `spamhaus_drop.long_description` output is valid but bulky at 1296 characters, 6 sentences, and 1 paragraph. It should be split/polished before the initial embedded catalog is committed.

## Plan

### Plan v1 (original — superseded)

The original phased plan (schema + agent + wrapper + UI + backfill + cron + docs + close) is preserved here as a record. Agent artifacts and local validation evidence exist as of 2026-05-24, but full backfill is not complete in repository terms until the artifacts are committed and embedded into source YAML. Phases 3-7 (UI, backfill, schedule, docs, spec updates, close) are superseded by Plan v2 below.

### Plan v2 (2026-05-24 — current)

Integration of the as-built enrichment into the per-feed YAML configs, the engine, the API, the UI, and the markdown. Operator-triggered PR-based refresh for future runs. Evaluation agent deferred.

**Step -1. Confirm enrichment validation freshness (precondition)**
- Run `agents/run-enrichment-pool.sh --unenriched --retry-failed --dry-run`; the queue must be empty before embedding enrichment into source YAML. The script currently reports an empty queue with exit code 2 and the message `no feeds queued`.
- If stale `validation-report.json` files are found, regenerate reports from current `output.json` before treating outputs as failed.
- No output may be embedded unless the full agent schema passes and the validator reports zero denylist violations and zero public-field IP findings.
- Confirm the 20 static-generated FireHOL-maintained enrichments using their `output.validation-report.json` files; they are generated, not missing.

**Step 0. Split multi-source YAML files (precondition)**
- Split `critical_dns.yaml` (3 sub-feeds), `critical_provider_ranges.yaml` (8 sub-feeds), `critical_service_ranges.yaml` (15 sub-feeds) — 26 files total — into one file per feed under `configs/firehol/sources/provider_infrastructure/`.
- Smoke-test the engine before/after: catalog count identical, per-feed output bytes identical.
- Keep `agents/locate-feed.py` working but its multi-feed-per-file branch becomes unreachable in practice.

**Step 1. Manual config-correction pass (one-time)**
- Build/refresh the per-feed delta report (`agent says X, YAML says Y` for `maintainer`, `maintainer_url`, `frequency`, `license`, `redistributable`, plus obvious wrongs) across all 357 feeds/merges. Format: grouped by field, sortable.
- Operator reviews report and applies fixes manually to source YAMLs. Commit as one or more PRs at operator's discretion (typically grouped by category).
- Output: every per-feed YAML now agrees with the enrichment on the small duplicated-fact set.

**Step 2. Embed enrichment into per-feed YAML (one-time conversion)**
- Conversion script reads the latest clean `.local/agents/feed-enrichment/<feed>/<UTC>/output.json` or static-generated `output.json` for each feed, projects it to the public embedded-enrichment schema, and writes the result under `sources: <feed>: enrichment:` or `merges: <feed>: enrichment:` in the per-feed YAML using ruamel.yaml round-trip.
- Conversion preserves markdown-capable strings as markdown. It does not rewrite prose for style.
- After conversion, run the public prose-hygiene report from D21 and manually polish flagged YAML fields before committing the initial embedded catalog.
- All 357 feeds/merges in one pass after Step -1 is clean and Step 0 has split multi-source files. Static-generated payloads for FireHOL-maintained merges embed under their `merges.<feed>.enrichment` entries, wherever the merge YAML lives.
- Operator commits as one or more PRs at their discretion.

**Step 3. Engine reader + API + types**
- Define `Enrichment` Go struct in (new) `pkg/enrichment/` mirroring the public embedded-enrichment schema only.
- Loader reads the `enrichment:` block from every per-feed YAML at startup; rejects or strips any internal field that snuck through; validates against the public embedded-enrichment schema; never fails the engine if a feed has no `enrichment:` block (fallback: nil).
- Extend `PublicFeedSummary` (in `pkg/engine/public_catalog.go`) and `FeedPageContext` (in `pkg/markdown/context.go`) with an optional `Enrichment *Enrichment` field.
- Extend API:
  - `/api/v1/sets/{name}` adds the full enrichment payload + a top-level `current_status` field.
  - `/api/v1/sets` (list endpoint) adds `short_description`, `official_name`, `current_status_state` per feed entry.

**Step 4. UI work**
- `current_status != active` → banner above hero with visual severity (red/orange/yellow depending on state).
- `FeedHero` integrates `official_name` as subtitle; role tags pulled from `roles[primary]`.
- Replace `SectionAbout` (currently the YAML `info:` one-liner) with new `SectionAboutThisFeed`:
  - `long_description` (intro paragraphs)
  - "Built from" sub-block (`derivation` + clickable `source_feeds[]` cross-references)
  - "How detection works" sub-block (`detection_classification`)
  - "Intended for / not intended for" sub-block (`scope_and_intent`)
  - "Maintained by" panel (compact `roles[]`)
- Add `SectionListingRules`: `listing_policy`, `unlisting_policy`, `unlist_request` (contact channels prominent).
- Add `SectionReputation`: `community.awards`, `criticism`, `engagement` — collapsible by default.
- Modify `SectionBehavior` to show maintainer-stated cadence alongside engine-measured cadence.
- Modify `SectionSpecs` so license and redistribution come from the (now corrected) source YAML, with full `redistribution.terms` from enrichment surfaced when present.
- New `<FeedRef name={feed}>` component: renders the slug as a link; hover tooltip shows `official_name`, `short_description`, primary maintainer. Used in: homepage tiles, overlap table, merge composition table, search results, cross-references inside `derivation.source_feeds[]` and `current_status.successor`.
- New footer: collapsible "Sources consulted" (`evidence[]`) + "Last researched: <run_at>" + AI-research disclaimer.
- Remove `static/feed-descriptions/*.html` files and the corresponding `<feed-name>` fetch in the UI; remove the `/api/v1/sets/about/{name}` handler.
- New methodology page under `pkg/web/static/methodology/` explaining the AI-research process.

**Step 5. Markdown work**
- `configs/templates/markdown/feed.md.tmpl`: add the same sections as the UI (linear; no collapsibles); maintainer-stated cadence overlay in Behavior; full `community` in the Reputation section; footer.
- MCP feed-discovery markdown: `short_description` inline next to every feed reference; corrected maintainer label from source YAML.
- Cross-feed references across all markdown surfaces include `short_description` inline.

**Step 6. PR-based refresh wrapper (for future re-runs)**
- Extend `agents/run-enrichment-pool.sh` with `--category <name>` selector (resolves via `locate-feed.py` filtered by category).
- Add an after-run hook that:
  - Writes successful results' `enrichment:` blocks back into the per-feed YAMLs (ruamel.yaml round-trip).
  - Generates a "significant changes" summary (maintainer rename, status change, license change, redistribution change).
  - Creates a branch `enrichment/<scope>-<date>` and commits both the YAML changes and the summary file.
  - Opens a PR via `gh pr create` only when a remote and authenticated `gh` are configured; otherwise leaves the branch and summary for local review.
- Existing `--feeds`, stdin pipe, slot count, and `--dry-run` behaviors carry over.

**Step 7. Spec + skill updates**
- `.agents/sow/specs/feeds.md`: add an "AI-enriched context" knowledge family.
- `.agents/sow/specs/files-layout.md`: document the per-feed source/merge YAML's `enrichment:` block.
- Project skills updated if implementation surfaces new patterns worth codifying.

**Step 8. Validation + close**
- Full validation command set passes.
- Public prose-hygiene report has no remaining review-blocking wall-of-text findings in the initial embedded catalog.
- End-to-end smoke on `spamhaus_drop`: UI renders all new sections; MCP `fetch_analysis` returns enriched markdown; tooltips visible on the three required surfaces.
- Acceptance criteria §A and §B confirmed.
- Move SOW to done; confirm `.agents/sow/pending/SOW-0091-feed-evaluation-agent.md` exists for the evaluation agent.

Estimated total: integration phase ~6-10 working days, dominated by UI work (step 4) and steps 1-2 (manual review surface area).

## Execution Log

### 2026-05-24

- User approved starting implementation; moved SOW from pending to current and changed status to in-progress.
- Completed Plan v2 Step -1 freshness validation:
  - initial local report count before the Akamai secondary merge static-generator repair was `latest_clean=337 latest_failed=0 latest_missing=0 static_clean=19 static_bad=0 total_clean=356`;
  - `agents/run-enrichment-pool.sh --unenriched --retry-failed --dry-run` queues no feeds and exits with the documented empty-queue status 2.
- Repaired SOW after review found stale lifecycle state, false completion claims, untracked deferrals, schema-shape contradictions, no-remote PR assumptions, wrong config field names, and personal-name leakage in durable text.
- Verified local enrichment run state:
  - 337 latest third-party runs have clean `validation-report.json`;
  - 19 static-generated FireHOL-maintained runs had clean `output.validation-report.json` before the Akamai secondary merge generator gap was found; after repairing the generator, 20 static-generated FireHOL-maintained runs are clean;
  - the previous 30 queued third-party runs were stale `validation-report.json` files; current `output.json` files passed validator reruns and the stale reports were regenerated.
- Added `.agents/sow/pending/SOW-0091-feed-evaluation-agent.md` as the concrete follow-up for the deferred evaluation agent.
- Recorded user guidance as D21: public markdown-capable enrichment strings remain markdown, and bulky/wall-of-text prose is flagged and manually polished after moving outputs into YAML.
- Tooling dry-run found the 15 static merge enrichments live under `configs/firehol/merges/*.yaml`, not `configs/firehol/sources/`; updated D11 and Step 2 to embed enrichment under either `sources.<feed>.enrichment` or `merges.<feed>.enrichment`.
- Added `agents/schemas/feed-enrichment-public.schema.json` and `agents/enrichment-public.py`:
  - `project` strips internal audit fields and validates the public projection;
  - `hygiene` reports D21 wall-of-text/markdown issues;
  - `embed` dry-runs or writes YAML `enrichment:` blocks under source or merge entries.
- Validated tooling on representative feeds:
  - `spamhaus_drop` projection strips internal reasoning, keeps 26 public source URLs, and reports 7 D21 hygiene findings;
  - `firehol_level1` static merge projection maps to `configs/firehol/merges/firehol_level1.yaml` with 0 D21 hygiene findings.
- Ran all-feed reports:
  - public projection/hygiene over all 356 latest outputs: 0 schema failures, 631 D21 hygiene findings;
  - YAML embed dry-run maps 330 entries (315 source entries, 15 merge entries) and blocks 26 entries until Step 0 splits the three multi-source YAML files.
- Completed Plan v2 Step 0 split:
  - split the 3 multi-entry provider-infrastructure YAML files into 26 single-source YAML files and 1 single-merge YAML file;
  - preserved the `critical_soft_akamai_edge_secondary` merge as a one-entry provider-infrastructure YAML after the split, because it was embedded in the original `critical_service_ranges.yaml`;
  - normalized raw config snapshots before/after match exactly: `sources=true`, `source_order=true`, `merges=true`, `merge_order=true` (`sources=341`, `merges=16`);
  - post-split enrichment embed dry-run resolves all 356 clean outputs with 0 blockers (`source_items=341`, `merge_items=15`, `hygiene_total=631`).
- Repaired the static-enrichment generator and public embed locator to handle `merges:` entries anywhere under `configs/firehol/`, not only under `configs/firehol/merges/`.
- Generated and validated static enrichment for the previously uncovered `critical_soft_akamai_edge_secondary` merge:
  - current clean-output count is `latest_clean=337 latest_failed=0 latest_missing=0 static_clean=20 static_bad=0 total_clean=357`;
  - post-repair enrichment embed dry-run resolves all 357 clean outputs with 0 blockers (`source_items=341`, `merge_items=16`, `hygiene_total=631`).
- Implemented and ran the Plan v2 Step 1 delta report:
  - `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.json --markdown .local/agents/config-correction-delta/all.md`;
  - scanned 357 outputs, found 1,062 review items, and reported 0 script errors;
  - findings by field: `frequency=76`, `license=301`, `maintainer=177`, `maintainer_url=193`, `redistributable=315`.
- User selected Step 1 option A: review and apply config corrections in batches, starting with conservative evidence-based groups instead of auto-applying the full 1,062-item delta.
- Step 1 batch 1 scope: apply only `redistributable: true` where YAML currently omits `redistributable`, enrichment says redistribution is allowed, and the same feed has no license delta. This is a 24-feed low-risk batch because it fills a missing policy flag without changing the license string.
- Step 1 batch 1 applied:
  - updated 24 YAML entries with `redistributable: true`;
  - final delta report moved from 1,062 to 1,038 findings, with `redistributable` findings reduced from 315 to 291;
  - validation passed with `go test ./pkg/config` and `git diff --check`.
- Step 1 batch 2 scope: apply only `redistributable: false` where YAML currently omits `redistributable`, enrichment says redistribution is not allowed, and the same feed has no license delta. This is a 24-feed restricted-use batch because it fills a missing policy flag without changing the license string.
- Step 1 batch 2 rejected and reverted:
  - attempted the 24-feed `redistributable: false` restricted-use batch;
  - `go test ./pkg/config` rejected at least `php_bad` and `stopforumspam` because project policy treats non-commercial, all-rights-reserved defaults, and similar wording as redistributable unless redistribution is explicitly forbidden;
  - reverted the full batch 2 so only batch 1 remains applied.
- Step 1 batch 3 scope: apply only license-string normalizations with unchanged legal meaning (`MIT` to `MIT License`, `GPL-3.0` to `GNU GPLv3`, and `Unlicense` to public-domain wording). This is a 23-feed batch with no redistribution, maintainer, URL, or cadence changes.
- Step 1 batch 3 applied:
  - updated 23 YAML license strings with equivalent canonical wording;
  - final delta report moved from 1,038 to 1,015 findings, with `license` findings reduced from 301 to 278;
  - validation passed with `go test ./pkg/config` and `git diff --check`.
- Step 1 batch 4 scope: normalize only the 48 MISP warninglist entries whose enrichment target is the same canonical CC0 string (`CC0 1.0 Universal (Public Domain Dedication)`). Leave the 10 MISP entries with variant enrichment phrasing for a later cleanup so catalog license wording stays consistent.
- Step 1 batch 4 applied:
  - updated 48 MISP warninglist license strings to `CC0 1.0 Universal (Public Domain Dedication)`;
  - final delta report moved from 1,015 to 967 findings, with `license` findings reduced from 278 to 230;
  - validation passed with `go test ./pkg/config` and `git diff --check`.

### 2026-05-25

- User confirmed the redistribution policy principle: do not mark feeds non-redistributable unless redistribution, republication, copying, or mirroring is explicitly prohibited. Non-commercial, all-rights-reserved, no-resale, warranty, or use-restriction wording is not enough by itself.
- Next Step 1 cleanup scope: improve the delta report so known equivalent license labels (for example CC0, MIT, GPLv3, Unlicense, and common Creative Commons names) do not appear as semantic conflicts.
- Step 1 license-delta cleanup applied:
  - updated `agents/enrichment-public.py` so the delta report normalizes known equivalent license labels before counting conflicts;
  - updated the shared classification rules and normative spec to record the direct-upstream/explicit-prohibition redistribution rule;
  - updated the project coding skill so future work does not default critical reference feeds to non-redistributable without direct-upstream evidence;
  - final delta report moved from 967 to 953 findings, with `license` findings reduced from 230 to 216.
- Step 1 batch 5 scope: apply the confirmed redistribution policy in the allow direction only:
  - set `redistributable: true` for 209 entries where YAML omits the field and enrichment says redistribution is allowed;
  - change the 4 existing `redistributable: false` entries to `true` where enrichment says redistribution is allowed (`critical_soft_auth0`, `critical_soft_cloudflare_edge`, `provider_context_gcp_cloud`, `provider_context_vultr_geofeed`);
  - leave the 78 missing-`false` findings untouched until direct-upstream evidence is reviewed under the explicit-prohibition rule.
- Step 1 batch 5 applied:
  - updated 213 YAML entries in the allow direction only;
  - repaired mechanical YAML writer formatting before validation so the batch is limited to the intended `redistributable` field changes and previously applied license/string changes;
  - final delta report moved from 953 to 740 findings, with `redistributable` findings reduced from 291 to 78.
- User approved proceeding with the remaining 78 `redistributable: false` findings as the next Step 1 task. Scope: review direct-upstream evidence by source family, set `redistributable: false` only when the direct upstream explicitly prohibits redistribution/republication/copying/mirroring/public sharing, and correct stale enrichment-derived claims when evidence is insufficient.
- Step 1 batch 6 direct-upstream redistribution review applied:
  - reviewed the 78 remaining `redistributable` findings by direct-upstream family and wrote the local evidence split to `.local/agents/config-correction-delta/redistributable-explicit-prohibition-review.json`;
  - set 69 entries to `redistributable: false` where direct-upstream evidence explicitly prohibits distribution, publication, public display, redistribution, or reproduction: iBlocklist (59), CAIDA (1), CleanTalk (2), IP2Location (1), IPIP (1), MaxMind (1), and Project Honey Pot (4);
  - corrected 9 enrichment false positives to `redistributable: true`: BotScout Last Bots Caught, drb-ra C2IntelFeeds, GriffinGuard, socks-proxy, and five StopForumSpam variants;
  - updated local enrichment `output.json` files for the 9 false positives so Step 2 embeds the corrected policy instead of the stale agent claim;
  - updated `pkg/config/catalog_verify_test.go` so catalog validation now protects the evidence-based non-redistribution list and the corrected redistributable examples;
  - updated the shared classification rules, normative spec, and project coding skill to include explicit public-display prohibitions as sufficient evidence for `redistributable: false`;
  - final delta report moved from 740 to 662 findings, with `redistributable` findings reduced from 78 to 0.
- Step 1 batch 7 scope: reduce `license` delta noise before editing catalog license values. Extend the delta normalizer only for known equivalent labels already verified in earlier batches or by direct-upstream family review: iBlocklist Terms of Service variants, DataPlane redistribution-prohibited variants, DroneBL community/software license wording, StopForumSpam modified CC BY-NC-ND wording, CAIDA AUA variants, AbuseIPDB Terms of Service short/URL variants, PDDL public-domain variants, and closely equivalent Creative Commons/Unlicense/default-public labels. Do not normalize `unknown` to public/default or hide real license-policy disagreements.
- Step 1 batch 7 applied:
  - updated `agents/enrichment-public.py` so license deltas normalize direct-upstream-family equivalents with feed context, including iBlocklist, DataPlane, DroneBL, StopForumSpam, AbuseIPDB, CAIDA, GeoLite, PDDL, Unlicense, and public-feed/default-public wording;
  - deliberately kept `unknown` and provider/merge policy disagreements visible;
  - final delta report moved from 662 to 587 findings, with `license` findings reduced from 216 to 141.
- Step 1 batch 8 scope: set source YAML `license: public feed` only for the 47 source entries whose enrichment says `Public, no stated license` and whose current catalog license is unknown/no-license wording or an upstream-of-upstream term that the direct-upstream rule supersedes. Exclude FireHOL merges, provider-published infrastructure descriptors, Blocklist.de as-is/no-warranty wording, and other cases that need separate family review.
- Step 1 batch 8 applied:
  - wrote the local review selection to `.local/agents/config-correction-delta/license-public-feed-review.json`;
  - updated 47 source YAML entries to `license: public feed` where the direct upstream is public and states no license or redistribution restriction;
  - left FireHOL merges, provider-published infrastructure descriptors, Blocklist.de as-is/no-warranty wording, and explicit-family cases for later focused review;
  - final delta report moved from 587 to 535 findings, with `license` findings reduced from 141 to 89.
- Step 1 batch 9 scope: apply only explicit direct-upstream license corrections where current source evidence is specific and low ambiguity:
  - set catalog license values for MIT-licensed GitHub/raw feeds, CyberCrime CC0, James Brine TLP:White custom terms, MaxMind GeoIP EULA, CleanTalk proprietary terms, CISA public-domain government publication, and the Threatview all-rights-reserved current site wording;
  - add concise normalizer aliases for verbose equivalent MaxMind, CleanTalk, and James Brine enrichment strings so YAML does not inherit bulky legal paragraphs;
  - correct local enrichment false positives for Threatview (old MIT claim no longer visible on the current official site) and IP Blacklist Cloud (plugin GPL does not govern the dead direct feed URL under the direct-upstream rule);
  - leave CoinBlockerLists, Emerging Threats, Blocklist.de, provider-infrastructure descriptors, and FireHOL merge-license semantics for separate family review.
- Step 1 batch 9 applied:
  - wrote the local review selection to `.local/agents/config-correction-delta/license-explicit-review.json`;
  - updated 11 catalog license fields for explicit direct-upstream evidence: 4 MIT/GitHub/raw feeds, datacenters MIT, CyberCrime CC0, James Brine TLP:White custom terms, MaxMind GeoIP EULA, 2 CleanTalk proprietary feeds, Threatview all-rights-reserved wording, and CISA public-domain government publication;
  - corrected 7 local enrichment `output.json` license values so future embed work keeps concise/stale-free license values instead of verbose legal paragraphs or unsupported source-of-source claims;
  - final delta report moved from 535 to 523 findings, with `license` findings reduced from 89 to 77.
- Step 1 batch 10 scope: perform a normalizer-only cleanup for remaining true wording equivalents: unknown/no-license wording, BotScout ToS variants, GeoLite EULA wording, CC0 short/long names, GPL-3.0 Hagezi wording, Turris CC BY-NC-SA wording, GreenSnow capitalization, GriffinGuard restricted-use wording, and VXVault copyleft wording. Do not hide version disagreements, source-family conflicts, provider policy differences, FireHOL merge gaps, or output values that include additional code-vs-data license detail.
- Step 1 batch 10 applied:
  - updated only the delta normalizer for the scoped wording equivalents;
  - final delta report moved from 523 to 513 findings, with `license` findings reduced from 77 to 67;
  - remaining `license` findings are intentionally visible because they are family-review items, version/policy disagreements, provider-descriptor semantics, FireHOL merge semantics, or code-vs-data distinctions.
- Step 1 batch 11 scope: review the Blocklist.de family against current direct-upstream pages. Treat the current account/privacy terms, export/API page, and home page as the direct-upstream evidence: the service is public/free, publishes export lists and IP display windows, and states no explicit license or redistribution prohibition for the catalog-downloaded list files. Therefore normalize the 10 Blocklist.de source licenses to `public feed` and correct local enrichment outputs to the same concise value; leave maintainer and frequency deltas for later non-license cleanup.
- Step 1 batch 11 applied:
  - wrote the local review selection to `.local/agents/config-correction-delta/license-blocklist-de-review.json`;
  - updated 10 Blocklist.de source YAML license fields to `public feed`;
  - corrected the 10 matching local enrichment `output.json` license values to `public feed` so future embed work does not reintroduce the old as-is/no-warranty wording as a license value;
  - final delta report moved from 513 to 503 findings, with `license` findings reduced from 67 to 57.
- Step 1 batch 12 scope: apply direct-upstream source-license corrections only where current official evidence is explicit and not a merge/provider-policy decision:
  - DShield uses CC BY-NC-SA 2.5 on the direct feed header and current feed documentation;
  - Emerging Threats Open/fwrules sources use BSD-style/BSD-3-Clause terms from direct feed headers or the current ET Open FAQ;
  - Hagezi, Stratosphere AIP, and ThreatFox GitHub/raw sources carry explicit GPL-3.0 or BSD-3-Clause license files;
  - IP2Location country and IPIP country use explicit proprietary data-license/user-agreement terms;
  - Bitwire inbound's data license is CC BY-NC-SA 4.0; its MIT generator-code license is research context, not the feed license value;
  - MyIP.ms is attribution-required public data per the direct site wording.
- Step 1 batch 12 exclusions: leave IP2Proxy, Didsoft proxy lists, CoinBlockerLists, provider-infrastructure descriptors, and FireHOL merges visible. IP2Proxy has tension between the product page's attribution wording and the master license's distribution restrictions; FireHOL merges expose a separate implementation gap because the config spec says merge-derived sources inherit non-redistributability from parents, while the merge YAML/model currently does not carry a redistributable field.
- Step 1 batch 12 applied:
  - wrote the local review selection to `.local/agents/config-correction-delta/license-source-family-review.json`;
  - updated 13 catalog license fields and 14 local enrichment `output.json` license values for DShield, Emerging Threats, Hagezi, IP2Location country, IPIP country, MyIP, Stratosphere AIP, ThreatFox, and Bitwire inbound;
  - updated the catalog redistribution test's DShield comment from CC BY-NC-SA 4.0 to CC BY-NC-SA 2.5;
  - final delta report moved from 503 to 489 findings, with `license` findings reduced from 57 to 43.
- Step 1 batch 13 scope:
  - update the four CoinBlockerLists catalog and enrichment license values to `AGPL-3.0`; current official CoinBlockerLists pages state the lists are open source and AGPL-licensed;
  - update `ip2proxy_px1lite` to the applicable IP2Location/IP2Proxy master-license wording and set `redistributable: false`, because the direct upstream terms explicitly prohibit public storage, transfer, publication, distribution, display, or sublicensing of the licensed products without a redistribution license;
  - align `socks_proxy` and `sslproxies` local enrichment license values to the existing concise Didsoft/free-tier catalog wording without changing `redistributable`, because the current direct terms do not contain an explicit raw-list redistribution prohibition comparable to IP2Location/IP2Proxy.
- Step 1 batch 13 applied:
  - wrote the local review selection to `.local/agents/config-correction-delta/license-coinbl-ip2proxy-didsoft-review.json`;
  - updated the four CoinBlockerLists catalog and local enrichment license values to `AGPL-3.0`;
  - updated `ip2proxy_px1lite` catalog and local enrichment license values to `IP2Location Master License Agreement (IP2Proxy LITE)`, set the catalog `redistributable: false`, and corrected the local enrichment redistribution claim to `allowed: false`;
  - aligned the two Didsoft proxy-list enrichment license values to the existing concise catalog wording without changing catalog redistributability;
  - updated the catalog redistributability regression test so `ip2proxy_px1lite` is protected as non-redistributable;
  - final delta report moved from 489 to 482 findings, with `license` findings reduced from 43 to 36 and no `redistributable` findings remaining.
- Step 1 batch 14 scope: normalizer-only cleanup for provider-infrastructure license wording where both sides say the direct provider data is public and no separate data license is stated.
  - Treat `Provider-published public IP ranges`, `Provider-published RFC 8805 geofeed`, `Provider-published support documentation`, the Fastly no-separate-license wording, and the DigitalOcean geofeed no-stated-license paragraph as equivalent for delta reporting only.
  - Do not change catalog YAML or redistributability.
- Step 1 batch 14 applied:
  - updated only the delta normalizer in `agents/enrichment-public.py`;
  - final delta report moved from 482 to 469 findings, with `license` findings reduced from 36 to 23.
- Remaining Step 1 license findings after batch 14 are intentionally visible:
  - FireHOL merge/static entries: `cleantalk*`, `firehol_*`, and `critical_soft_akamai_edge_secondary`;
  - provider-infrastructure entries with explicit provider-policy or data-license disagreements: AWS, GitHub, GCP, Microsoft 365, Stripe, and Terraform Cloud;
  - the FireHOL merge group also exposes the implementation gap that merge-derived sources currently do not carry or compute `redistributable` despite the config spec's inherited non-redistributability rule.
- Step 1 batch 15 scope: resolve the explicit provider-policy license bucket by replacing generic provider-range catalog labels with concise direct-upstream terms and correcting stale or bulky local enrichment license values.
  - AWS entries use `AWS Intellectual Property License`;
  - GitHub hosted-compute ranges use GitHub Terms of Service and Acceptable Use Policies;
  - Microsoft 365 endpoint data uses Microsoft API Terms of Use with no dedicated endpoint-data license;
  - Stripe API/webhook ranges use Stripe Services Agreement;
  - HCP Terraform IP ranges use HashiCorp Website Terms of Service with no dedicated IP-ranges data license, replacing the stale BSL wording;
  - Google Cloud IP ranges use Creative Commons Attribution 4.0 International and remain redistributable.
- Step 1 batch 15 applied:
  - wrote the local review selection to `.local/agents/config-correction-delta/license-provider-explicit-review.json`;
  - updated 8 provider-infrastructure catalog license values;
  - corrected 6 matching local enrichment `output.json` license/redistribution values and regenerated their validation reports;
  - updated the catalog redistributability regression test to protect AWS, GitHub, Microsoft, Stripe, and HashiCorp as non-redistributable provider-data cases and Google Cloud as redistributable;
  - final delta report moved from 469 to 461 findings, with `license` findings reduced from 23 to 15.
- Remaining Step 1 license findings after batch 15 are only FireHOL merge/static entries: `cleantalk*`, `firehol_*`, and `critical_soft_akamai_edge_secondary`. They need merge-specific handling because the current expanded `Source` model does not compute inherited redistributability from merge parents.
- Step 1 batch 16 scope: resolve FireHOL merge/static license and redistributability semantics without hiding the merge inheritance gap.
  - Add `redistributable` parsing to `merges:` and compute expanded merge-source redistributability conservatively from every transitive additive and subtractive parent;
  - update static merge enrichment generation so generated `redistribution.allowed` mirrors the same inherited policy;
  - record FireHOL merge YAML license as `Composite of component feed licenses` where no more-specific merge license exists;
  - set explicit `redistributable: false` on the 5 FireHOL merges whose component policy makes raw redistribution false (`cleantalk`, `firehol_anonymous`, `firehol_level2`, `firehol_level4`, `firehol_proxies`).
- Step 1 batch 16 applied:
  - wrote the local review selection to `.local/agents/config-correction-delta/license-merge-inheritance-review.json`;
  - updated `pkg/config` merge expansion and added focused merge-redistributability tests covering additive, subtractive, explicit-false, explicit-true-with-blocked-parent, and transitive merge parents;
  - updated `tools/build-firehol-static-enrichment.py` to emit composite merge licenses and inherited redistributability;
  - regenerated and validated all 20 FireHOL-maintained static enrichment outputs;
  - split redistributability tests into focused files instead of growing already-large posture-guarded test files;
  - final delta report moved from 461 to 446 findings, removing all `license` and `redistributable` findings.
- Remaining Step 1 findings after batch 16 are non-license duplicated facts only: `frequency=76`, `maintainer=177`, and `maintainer_url=193`.
- Step 1 batch 17 scope: remove non-semantic maintainer and maintainer-URL noise from the delta report before considering catalog edits.
  - Treat `http`/`https`, `www`, provider documentation subdomains, and same-site source pages versus home pages as equivalent for `maintainer_url` comparison.
  - Treat GitHub/GitLab repository URLs and their owner namespace URLs as equivalent, while preserving owner identity so different GitHub/GitLab owners still report as disagreements.
  - Treat clear maintainer display-name variants as equivalent only where they are the same entity: legal suffixes, `FireHOL` versus `FireHOL catalog project`, `iBlocklist.com` versus `iblocklist, LLC`, `DroneBL.org` versus `The DroneBL Team`, `DataPlane.org` versus `DataPlane.org NFP`, and direct punctuation/case variants.
  - Do not change catalog YAML, local enrichment outputs, or public meaning; this is a report-quality cleanup so remaining findings represent real semantic choices.
- Step 1 batch 17 applied:
  - updated only `agents/enrichment-public.py`;
  - final delta report moved from 446 to 247 findings, with `maintainer_url` reduced from 193 to 61 and `maintainer` reduced from 177 to 110; `frequency` remains 76.
- Step 1 batch 18 scope: align merge-derived feed maintainers with the merge ownership model already used by the static enrichment generator.
  - A merge is catalog-maintained even when every component comes from one third-party provider; third-party source identity remains visible through the merge's component list, source feed links, license/attribution, and derivation text.
  - Update the 6 merge-derived entries still carrying component-provider maintainers (`cleantalk*`, `cymru_unassigned`, and `critical_soft_akamai_edge_secondary`) to `maintainer: FireHOL` with the existing FireHOL maintainer URL pattern.
- Step 1 batch 18 applied:
  - updated the 6 merge-derived YAML entries;
  - final delta report moved from 247 to 241 findings, with `maintainer` reduced from 110 to 104.
- Remaining Step 1 findings after batch 18 are semantic mismatches or policy choices, not obvious normalizer noise: `frequency=76`, `maintainer=104`, and `maintainer_url=61`.
- Step 1 batch 19 scope: reduce remaining report-only maintainer noise where the catalog value is evidence-equivalent to another identity already present in the same enrichment projection.
  - Accept YAML `maintainer` values that match the projection's official feed name, declared role names, URL-derived project/brand labels, GitHub/GitLab owner handles, slash/plus/`and`-separated identity parts, or parenthetical/legal-suffix variants.
  - Accept YAML `maintainer_url` values only through the existing URL normalizer; do not collapse different owner domains except the already-supported same-host/root-domain and repository-owner forms.
  - Do not change catalog YAML, local enrichment outputs, or public meaning; this keeps the delta report focused on real semantic owner/cadence choices.
- Step 1 batch 19 applied:
  - updated only `agents/enrichment-public.py`;
  - final delta report moved from 241 to 105 findings, with `maintainer` reduced from 104 to 20 and `maintainer_url` reduced from 61 to 9; `frequency` remains 76.
- Remaining Step 1 findings after batch 19:
  - `frequency=76` remains intentionally visible because enrichment `update_frequency` is a source-stated cadence while YAML `frequency` is the scheduler cadence. These are not safe to auto-apply without per-feed operational review.
  - `maintainer=20` and `maintainer_url=9` remain visible because they represent true semantic choices such as feed/project brand versus legal operator, renamed/acquired provider, repository URL versus owner URL, or person versus organization.
- User approved Step 1 batch 20: review the remaining 20 maintainer and 9 maintainer-URL findings against direct-upstream evidence, apply catalog edits only where current YAML is stale or misleading, keep feed/project brand names where they are more useful than legal-entity names, and leave the 76 frequency findings for a separate scheduler-cadence decision.
- Step 1 batch 20 scope: apply direct-upstream maintainer/URL metadata fixes where current evidence shows the catalog value is stale, less precise than the direct upstream, or only a spacing/renaming variant.
  - Current direct-upstream checks found: CINS Army pages title the service as `CINS Army`; Emerging Threats root redirects to Proofpoint ET Intelligence; Team Cymru `.org` redirects to `team-cymru.com`; OpenDBL's Fnutt Consulting URL does not resolve while OpenDBL itself serves the list; Stratosphere pages title the owner as `Stratosphere Laboratory`; GitHub API names `borestad` as Johan Borestad and `elliotwutingfeng` as Wu Tingfeng; USTC's official English site expands USTC to `University of Science and Technology of China`.
  - Kept several findings intentionally visible instead of editing YAML: Clean-MX feed brand versus legal operator, Data-Shield GitLab source repository versus GitHub owner profile, MISP Project versus individual list author, OpenDBL service brand versus legal provider, Google Cloud/Akamai/Vultr product brands versus legal entities, and Rutgers CS display name versus lab-level organization.
  - Added only one report alias for `CriticalPathSecurity` versus `Critical Path Security`; this is a spacing/brand-token variant, not a catalog metadata change.
- Step 1 batch 20 applied:
  - updated 15 source YAML files for maintained display names or stale maintainer URLs;
  - updated `agents/enrichment-public.py` for the `CriticalPathSecurity` spacing variant;
  - final delta report moved from 105 to 90 findings, with `maintainer` reduced from 20 to 9 and `maintainer_url` reduced from 9 to 5; `frequency` remains 76.
- Remaining Step 1 findings after batch 20:
  - `frequency=76` remains the next policy question because scheduler cadence and upstream-stated update cadence are different concepts.
  - `maintainer=9` and `maintainer_url=5` remain visible as real display/ownership choices, not obvious stale metadata.
- User approved the Step 1 frequency policy:
  - YAML `frequency` is the scheduler polling cadence.
  - Enrichment `update_frequency` is upstream-stated or assistant-inferred cadence.
  - Runtime cache/history statistics are the observed local cadence, but they are sampling-limited: if the configured poll interval is slower than upstream cadence, local observations cannot disprove faster upstream changes.
  - When configured polling is faster than upstream-stated cadence, observed local stats can prove likely over-polling.
  - When configured polling is slower than strongly evidenced upstream cadence, adopt the upstream cadence as a controlled experiment and use runtime stats after several days to confirm or relax it.
  - Do not auto-inherit weak/inferred cadence guesses; keep them visible for review.
- Step 1 batch 21 scope:
  - Fix the frequency review tooling before catalog edits: several enrichment outputs used `1m` prose to mean monthly, while the schema and delta parser define `m` as minutes. Treat monthly prose with `1m` as `30d` in the public projection/delta path so monthly feeds are not misclassified as one-minute polling candidates.
  - Generate a corrected frequency policy review from the delta report plus the local runtime cache at `/opt/update-ipsets/data/.cache.json`. The cache is local observational evidence only, not proof of upstream cadence.
  - Apply only the strongly evidenced under-polling candidates after the monthly-unit false positives are removed. Keep weak/inferred cadence guesses and over-polling cases visible for later review unless local observations clearly prove no faster updates.
- Step 1 batch 21 applied:
  - added a defensive public-projection normalizer so monthly `1m` prose is treated as `30d`, while real one-minute prose stays `1m`;
  - documented the public schema's compact frequency unit rule: `m` means minutes and monthly cadence should use `30d`;
  - generated `.local/agents/config-correction-delta/frequency-policy-review.json` and `.local/agents/config-correction-delta/frequency-policy-review.md` from the corrected delta and local runtime cache;
  - applied 18 strongly evidenced under-polling scheduler cadence changes: `abuseipdb_1d`, `abuseipdb_30d`, `abuseipdb_3d`, `abuseipdb_7d`, `feodo`, `fullbogons`, `gazpitchy_blacklist`, `geolite2_country`, `iblocklist_ciarmy_malicious`, `maxmind_geolite2_asn`, `netmountains_curated`, `romainmarcoux_malicious`, `rutgers_drop`, `sefinek_malicious`, `serpro_reputation`, `shadowwhisperer_bruteforce_extreme`, `shadowwhisperer_bruteforce_high`, and `uninvited_activity`;
  - applied 1 over-polling relaxation, `turris_greylist` from 1 hour to 1 day, because the upstream cadence is explicit and local observations showed no sub-daily update gaps;
  - final delta report moved from 90 to 69 findings: `frequency=55`, `maintainer=9`, and `maintainer_url=5`.
- Remaining Step 1 findings after batch 21:
  - `frequency=55` remains intentionally visible: 53 are over-polling cases without enough local proof to relax safely under the approved policy, and 2 are under-polling cases with only weak/inferred cadence confidence.
  - `maintainer=9` and `maintainer_url=5` remain semantic display/ownership choices from batch 20.
- Step 1 batch 22 scope:
  - Apply a narrow runtime-jitter interpretation to explicit daily over-polling cases only: if upstream cadence is explicitly daily, local observed average is daily-or-slower, and the observed minimum update gap is within 2% of daily, treat the few missing minutes as scheduler/runtime jitter rather than real sub-daily upstream updates.
  - Apply that rule only to the four current matches: `iblocklist_cidr_report_bogons`, `stratosphere_aip_24h`, `stratosphere_aip_alpha`, and `stratosphere_aip_prioritize`.
  - Do not relax monthly/weekly over-polling cases where local daily polling cannot prove the upstream is slower, and do not apply weak/inferred under-polling cases.
- Step 1 batch 23 scope:
  - Remove the batch-20 accepted maintainer and maintainer-URL display choices from the correction delta so the report no longer treats them as actionable metadata fixes.
  - Keep the catalog YAML unchanged: the accepted values are feed/project display names, source-repository URLs, or product brands that are more useful in the catalog than legal-entity names, individual authors, or alternate profile URLs.
  - Leave remaining frequency findings visible because they represent operational cadence review, not display metadata noise.
- Step 1 batch 22 applied:
  - updated `iblocklist_cidr_report_bogons` from 12h to 1d;
  - updated `stratosphere_aip_24h`, `stratosphere_aip_alpha`, and `stratosphere_aip_prioritize` from 1h to 1d;
  - final delta report moved from 69 to 65 findings: `frequency=51`, `maintainer=9`, and `maintainer_url=5`.
- Step 1 batch 23 applied:
  - updated only `agents/enrichment-public.py` to suppress the reviewed maintainer and maintainer-URL display-choice deltas;
  - final full delta report moved from 65 to 51 findings, all `frequency`;
  - non-frequency duplicated metadata delta is clean: `maintainer=0`, `maintainer_url=0`, `license=0`, and `redistributable=0`.
- Step 2 batch 1 scope:
  - embed public enrichment projections from the latest validator-clean local enrichment runs into all per-feed source and merge YAML files;
  - preserve markdown strings as authored by the projection script and do not perform prose polishing during the mechanical embed;
  - run prose hygiene after embedding so bulky/long markdown fields can be polished as a separate focused pass.
- Step 2 batch 2 scope:
  - add an embedded-YAML hygiene mode to `agents/enrichment-public.py` because the existing hygiene command scans local `output.json` projections, not manually polished YAML `enrichment:` blocks;
  - use the embedded hygiene mode to verify prose cleanup after YAML polishing without mutating the original local enrichment outputs.
- Step 2 batch 3 scope:
  - run a mechanical markdown-safe prose cleanup over embedded YAML only: split long paragraphs at sentence boundaries, preserving the original wording, field ownership, and markdown-capable strings;
  - do not rewrite claims, sources, licenses, redistribution terms, or feed semantics in this pass;
  - rerun embedded hygiene and then inspect the remaining findings for manual polishing.
- Step 2 batch 1 applied:
  - dry-run mapped all 357 latest clean enrichment outputs with 0 errors (`source_items=341`, `merge_items=16`) and reported 630 prose-hygiene findings before polishing;
  - wrote public `enrichment:` blocks to all 357 source/merge YAML entries, with 0 writer errors;
  - confirmed 357 embedded YAML `enrichment:` blocks after the write.
- Step 2 batch 2 applied:
  - added `agents/enrichment-public.py hygiene --embedded` to scan committed YAML `enrichment:` blocks instead of local output projections;
  - embedded hygiene baseline matched the projection baseline: 357 scanned, 0 schema failures, 630 hygiene findings.
- Step 2 batch 3 applied:
  - mechanically split long embedded markdown paragraphs at sentence boundaries, preserving wording and markdown-capable strings; touched 442 fields across 213 YAML files;
  - replaced the 6 DroneBL raw-HTML-looking `<reversed-IP>.dnsbl.dronebl.org` placeholders with Markdown-safe `reversed-IP.dnsbl.dronebl.org` text;
  - final embedded hygiene report scanned 357 YAML blocks with 0 schema failures and 0 hygiene findings.
- Step 2 spec maintenance applied:
  - updated `.agents/sow/specs/config.md` to define authored `enrichment:` blocks on source and merge entries;
  - updated `.agents/sow/specs/feeds.md` to define the public researched-context fields and distinguish upstream-stated cadence from scheduler polling cadence;
  - updated `.agents/sow/specs/files-layout.md` to require install/catalog round-trip tooling to preserve authored `enrichment:` metadata.
- Step 3 scope:
  - add a new `pkg/enrichment` package containing only the public embedded-enrichment schema types and lightweight validation;
  - extend `config.Source` and `config.Merge` with optional typed `enrichment:` fields, and copy those fields through directory merge, retention derivative expansion, and merge expansion;
  - reject malformed public enrichment metadata during config validation, but tolerate feeds without `enrichment:` blocks;
  - strip unexpected/internal YAML fields by decoding into typed structs and only serializing those typed public structs to JSON/API/markdown contexts;
  - expose enrichment additively through public feed summaries, per-feed metadata artifacts, and markdown feed context without adding request-time generation or upstream calls.
- Step 3 applied:
  - added `pkg/enrichment` public-schema types, enum/version/run-timestamp validation, clone helpers, and string pointer helpers;
  - extended `config.Source` and `config.Merge` with typed `Enrichment` fields and copied them through config directory merge, history derivative expansion, and merge expansion;
  - validated enrichment during config validation while allowing missing enrichment blocks;
  - exposed `official_name`, `short_description`, and `current_status_state` on `PublicFeedSummary`;
  - exposed top-level `official_name`, `short_description`, `current_status`, and full `enrichment` on generated per-feed metadata;
  - hydrated `markdown.FeedPageContext` from the generated metadata enrichment fields;
  - updated `.agents/sow/specs/config.md` and `.agents/sow/specs/website.md` to document the reader and public API contracts.
- Step 4 scope:
  - replace the UI's separate `/api/v1/sets/about/{name}` HTML fetch with the enriched per-feed metadata already returned by `/api/v1/sets/{name}`;
  - render the new About, listing/unlisting, reputation/community, and source-consulted sections from `feed.enrichment`, with the old YAML `info` text as fallback when no enrichment exists;
  - add a feed status banner and official-name subtitle from top-level metadata fields;
  - add maintainer-stated cadence and redistribution terms where available without hiding the existing observed/runtime fields;
  - remove the obsolete public about handler and corresponding frontend API/query helpers;
  - update the website spec and frontend tests to reflect the new artifact-backed source of descriptive content.
- Step 4 applied:
  - removed the old `/api/v1/sets/about/{name}` public handler, removed frontend `feedAbout` API/query helpers, and deleted the 27 tracked `pkg/web/static/feed-descriptions/*.html` files so descriptive content now comes from generated metadata/markdown;
  - rebuilt the feed page About section to render enriched long description, roles, derivation/source feeds, listing/unlisting/removal, scope/intent, community signals, and sources consulted, with the existing sanitized `feed.info` fallback for feeds without enrichment;
  - added the non-active status banner, official-name hero subtitle, maintainer-stated cadence panel, and enriched redistribution terms without replacing observed runtime cadence/health fields;
  - added the reusable `FeedRef` component and used it on the required feed-reference surfaces: homepage cards/table rows, overlap table and inclusion lists, merge composition rows, IP lookup/search results, enriched source-feed cross-references, and current-status successor links;
  - split enrichment DTOs into `ui/src/lib/enrichment-types.ts` so `api-types.ts` stayed within the project posture budget;
  - extended explorer free-text filtering to include `official_name` and `short_description`;
  - added the public methodology page `pkg/web/static/methodology/ai-researched-feed-context.md`;
  - updated `configs/templates/markdown/feed.md.tmpl` to render the same enriched context in generated feed markdown;
  - updated `.agents/sow/specs/website.md` to document the new metadata-backed enrichment contract and state that `/api/v1/sets/about/{name}` must not be reintroduced.
- Step 5 applied:
  - extended generated feed markdown with top-level `official_name` and `short_description`;
  - added derivation `source_feeds[]` cross-references and current-status successor references to the feed markdown template;
  - changed the enrichment footer so `Last researched` is rendered whenever enrichment has `run_at`, even if a source list is absent;
  - extended MCP `find_feeds` hits with `official_name` and `short_description`;
  - updated MCP search to include official names and short descriptions;
  - updated MCP discovery markdown to include an inline feed label with official name and a table description column backed by `short_description` before falling back to legacy `info`;
  - updated the MCP endpoint operator docs and website spec for the enriched discovery contract.

## Validation

### 2026-05-24 SOW Repair Validation

- Re-ran the validator against the 30 queued third-party `output.json` files and regenerated their stale `.local/agents/feed-enrichment/<feed>/<UTC>/validation-report.json` files.
- Recounted latest local reports before the Akamai secondary merge static-generator repair: `latest_clean=337 latest_failed=0 latest_missing=0 static_clean=19 static_bad=0 total_clean=356`.
- Ran `agents/run-enrichment-pool.sh --unenriched --retry-failed --dry-run`; it queued no feeds. The script reports an empty queue with exit code 2 and the message `no feeds queued`.
- Searched SOW-0014 for stale 30-failure wording and replaced it with the current clean validation state.
- Re-ran Step -1 validation after moving the SOW to current; report counts remained clean and the dry-run still queued no feeds.
- Validated `agents/enrichment-public.py` with `python3 -m py_compile`.
- Validated `agents/schemas/feed-enrichment-public.schema.json` with `python3 -m json.tool`.
- Ran `agents/enrichment-public.py project .local/agents/feed-enrichment/spamhaus_drop/20260518T131942Z/output.json`; public projection validated and stripped internal audit fields.
- Ran `agents/enrichment-public.py hygiene --feeds spamhaus_drop`; reported 7 expected D21 prose findings.
- Ran `agents/enrichment-public.py embed --feeds firehol_level1`; dry-run mapped the static merge enrichment to `configs/firehol/merges/firehol_level1.yaml`.
- Ran `agents/enrichment-public.py hygiene --all`; scanned 356 outputs with 0 public-schema failures and 631 hygiene findings.
- Ran `agents/enrichment-public.py embed --all`; dry-run mapped 330 entries and reported the expected 26 multi-source YAML blockers for Step 0.
- Created `.local/agents/sow-0014/raw-config-before-split.json` and `.local/agents/sow-0014/raw-config-after-split.json`; after repairing the Akamai secondary merge placement, before/after raw config comparison passed for `sources`, `source_order`, `merges`, and `merge_order`.
- Ran `go test ./pkg/config` after Step 0 split; passed.
- Ran `agents/enrichment-public.py embed --all --report .local/agents/embed-enrichment/all.dry-run.after-split.v2.json`; mapped 356 entries with 0 errors (`source_items=341`, `merge_items=15`, `hygiene_total=631`).
- Ran `python3 tools/build-firehol-static-enrichment.py --dry-run`; discovered 20 FireHOL-maintained entries after the repair (`static-curated=3`, `internal-baseline=1`, `merge=16`).
- Ran `python3 tools/build-firehol-static-enrichment.py --validate`; wrote and validated 20 static outputs, including `critical_soft_akamai_edge_secondary`.
- Recounted latest local reports after the static-generator repair: `latest_clean=337 latest_failed=0 latest_missing=0 static_clean=20 static_bad=0 total_clean=357`.
- Ran `agents/enrichment-public.py embed --all --report .local/agents/embed-enrichment/all.dry-run.after-static-merge.json`; mapped 357 entries with 0 errors (`source_items=341`, `merge_items=16`, `hygiene_total=631`).
- Ran `agents/enrichment-public.py hygiene --all --json .local/agents/prose-hygiene/all.after-static-merge.json`; scanned 357 outputs with 0 public-schema failures and 631 hygiene findings.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.json --markdown .local/agents/config-correction-delta/all.md`; scanned 357 outputs, found 1,062 review items, and reported 0 script errors.
- Applied Step 1 batch 1: set `redistributable: true` on 24 entries where enrichment and YAML already agreed on the license field.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-batch1.final.json --markdown .local/agents/config-correction-delta/all.after-batch1.final.md`; scanned 357 outputs, found 1,038 remaining review items, and reported 0 script errors.
- Ran `go test ./pkg/config` after Step 1 batch 1; passed.
- Ran `git diff --check` on the touched SOW and batch-1 YAML paths; passed.
- Attempted Step 1 batch 2 (`redistributable: false` with no license delta), but `go test ./pkg/config` failed on project redistribution-policy expectations for `php_bad` and `stopforumspam`; reverted all 24 batch-2 edits.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-batch2-reverted.json --markdown .local/agents/config-correction-delta/all.after-batch2-reverted.md`; scanned 357 outputs, found 1,038 remaining review items, and reported 0 script errors.
- Ran `go test ./pkg/config` after reverting batch 2; passed.
- Ran `git diff --check` on the touched SOW and source YAML paths after reverting batch 2; passed.
- Applied Step 1 batch 3: normalized 23 equivalent license strings (`MIT`, `GPL-3.0`, `Unlicense`) without changing redistribution semantics.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-batch3.final.json --markdown .local/agents/config-correction-delta/all.after-batch3.final.md`; scanned 357 outputs, found 1,015 remaining review items, and reported 0 script errors.
- Ran `go test ./pkg/config` after Step 1 batch 3; passed.
- Ran `git diff --check` on the touched SOW and source YAML paths after Step 1 batch 3; passed.
- Applied Step 1 batch 4: normalized 48 MISP warninglist license strings to the same canonical CC0 wording.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-batch4.final.json --markdown .local/agents/config-correction-delta/all.after-batch4.final.md`; scanned 357 outputs, found 967 remaining review items, and reported 0 script errors.
- Ran `go test ./pkg/config` after Step 1 batch 4; passed.
- Ran `git diff --check` on the touched SOW and source YAML paths after Step 1 batch 4; passed.

### 2026-05-25 Step 1 Validation

- Ran `python3 -m py_compile agents/enrichment-public.py tools/build-firehol-static-enrichment.py` after the license-delta normalizer; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-license-normalizer.json --markdown .local/agents/config-correction-delta/all.after-license-normalizer.md`; scanned 357 outputs, found 953 remaining review items, and reported 0 script errors (`frequency=76`, `license=216`, `maintainer=177`, `maintainer_url=193`, `redistributable=291`).
- Ran `go test ./pkg/config` after recording the redistribution-policy rule updates; passed.
- Ran `git diff --check` on the touched SOW, spec, project skill, agent helper, generator helper, and catalog paths after recording the redistribution-policy rule updates; passed.
- Searched touched durable artifacts for the user's personal name; no matches.
- Searched touched durable artifacts and helper scripts for trailing whitespace; no matches.
- Applied Step 1 batch 5: set 213 YAML entries to `redistributable: true` in the allow direction only; the 78 missing-`false` findings remain untouched for explicit-prohibition review.
- Re-ran the batch-5 YAML writer with project-style indentation and wide output to remove unintended ruamel formatting churn before validation.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-batch5.final.json --markdown .local/agents/config-correction-delta/all.after-batch5.final.md`; scanned 357 outputs, found 740 remaining review items, and reported 0 script errors (`frequency=76`, `license=216`, `maintainer=177`, `maintainer_url=193`, `redistributable=78`).
- Ran `go test ./pkg/config` after Step 1 batch 5; passed.
- Ran `python3 -m py_compile agents/enrichment-public.py tools/build-firehol-static-enrichment.py` after Step 1 batch 5; passed.
- Ran `git diff --check` on the touched SOW, spec, project skill, agent helper, generator helper, and catalog paths after Step 1 batch 5; passed.
- Removed trailing whitespace from newly split provider-infrastructure YAML files found by direct scan, then re-ran `git diff --check`; passed.
- Re-ran `go test ./pkg/config` after the provider-infrastructure YAML whitespace cleanup; passed.
- Re-ran direct trailing-whitespace and personal-name scans across touched durable artifacts and source/catalog files after the cleanup; no matches.
- Applied Step 1 batch 6: reviewed the 78 remaining `redistributable` findings by direct-upstream family, set 69 confirmed-prohibited entries to `false`, corrected 9 false-positive enrichment outputs to `true`, and updated the catalog redistributability regression test.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-redistributable-review.json --markdown .local/agents/config-correction-delta/all.after-redistributable-review.md`; scanned 357 outputs, found 662 remaining review items, and reported 0 script errors (`frequency=76`, `license=216`, `maintainer=177`, `maintainer_url=193`).
- Ran `go test ./pkg/config` after Step 1 batch 6; passed.
- Ran `python3 -m py_compile agents/enrichment-public.py tools/build-firehol-static-enrichment.py` after Step 1 batch 6; passed.
- Ran `git diff --check` on the touched SOW, spec, project skill, agent helper, generator helper, catalog test, and catalog paths after Step 1 batch 6; passed.
- Ran `python3 -m py_compile agents/enrichment-public.py tools/build-firehol-static-enrichment.py` after Step 1 batch 7; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-license-normalizer-v2.json --markdown .local/agents/config-correction-delta/all.after-license-normalizer-v2.md`; scanned 357 outputs, found 587 remaining review items, and reported 0 script errors (`frequency=76`, `license=141`, `maintainer=177`, `maintainer_url=193`).
- Ran `python3 -m py_compile agents/enrichment-public.py tools/build-firehol-static-enrichment.py` after Step 1 batch 8; passed.
- Ran `go test ./pkg/config` after Step 1 batch 8; passed.
- Ran `git diff --check` on the touched SOW, spec, project skill, agent helper, generator helper, catalog test, and catalog paths after Step 1 batch 8; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-license-public-feed.json --markdown .local/agents/config-correction-delta/all.after-license-public-feed.md`; scanned 357 outputs, found 535 remaining review items, and reported 0 script errors (`frequency=76`, `license=89`, `maintainer=177`, `maintainer_url=193`).
- Ran `python3 -m py_compile agents/enrichment-public.py tools/build-firehol-static-enrichment.py` after Step 1 batch 9; passed.
- Ran `go test ./pkg/config` after Step 1 batch 9; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-license-explicit.json --markdown .local/agents/config-correction-delta/all.after-license-explicit.md`; scanned 357 outputs, found 523 remaining review items, and reported 0 script errors (`frequency=76`, `license=77`, `maintainer=177`, `maintainer_url=193`).
- Ran `python3 -m py_compile agents/enrichment-public.py tools/build-firehol-static-enrichment.py` after Step 1 batch 10; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-license-normalizer-v3.json --markdown .local/agents/config-correction-delta/all.after-license-normalizer-v3.md`; scanned 357 outputs, found 513 remaining review items, and reported 0 script errors (`frequency=76`, `license=67`, `maintainer=177`, `maintainer_url=193`).
- Ran `go test ./pkg/config` after Step 1 batch 10; passed.
- Ran `git diff --check` on touched SOW, spec, project skill, shared rules, agent helper, generator helper, catalog test, catalog paths, and locally corrected enrichment outputs after Step 1 batch 10; passed.
- Searched touched durable artifacts, source/catalog files, and locally corrected enrichment outputs for the user's personal name; no matches.
- Searched touched durable artifacts, source/catalog files, and locally corrected enrichment outputs for trailing whitespace; no matches.
- Ran `go test ./pkg/config` after Step 1 batch 11; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-license-blocklist-de.json --markdown .local/agents/config-correction-delta/all.after-license-blocklist-de.md`; scanned 357 outputs, found 503 remaining review items, and reported 0 script errors (`frequency=76`, `license=57`, `maintainer=177`, `maintainer_url=193`).
- Ran `git diff --check` on touched SOW, spec, project skill, shared rules, agent helper, generator helper, catalog test, catalog paths, and locally corrected enrichment outputs after Step 1 batch 11; passed.
- Re-ran targeted personal-name and trailing-whitespace scans on touched durable artifacts, source/catalog files, and the 17 locally corrected enrichment outputs; no matches.
- Ran `python3 -m py_compile agents/enrichment-public.py tools/build-firehol-static-enrichment.py` after Step 1 batch 12; passed.
- Ran `go test ./pkg/config` after Step 1 batch 12; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-license-source-family.json --markdown .local/agents/config-correction-delta/all.after-license-source-family.md`; scanned 357 outputs, found 489 remaining review items, and reported 0 script errors (`frequency=76`, `license=43`, `maintainer=177`, `maintainer_url=193`).
- Ran `python3 -m py_compile agents/enrichment-public.py tools/build-firehol-static-enrichment.py` after Step 1 batch 13; passed.
- Ran `go test ./pkg/config` after Step 1 batch 13; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-license-coinbl-ip2proxy-didsoft-v2.json --markdown .local/agents/config-correction-delta/all.after-license-coinbl-ip2proxy-didsoft-v2.md`; scanned 357 outputs, found 482 remaining review items, and reported 0 script errors (`frequency=76`, `license=36`, `maintainer=177`, `maintainer_url=193`).
- Ran `git diff --check` on the touched tracked SOW support, spec, project skill, helper, catalog test, and catalog paths after Step 1 batch 13; passed.
- Searched the touched durable artifacts, source/catalog files, and 38 locally corrected enrichment outputs for the user's personal name; no matches.
- Searched the touched durable artifacts, source/catalog files, and 38 locally corrected enrichment outputs for trailing whitespace; no matches.
- Ran `python3 -m py_compile agents/enrichment-public.py tools/build-firehol-static-enrichment.py` after Step 1 batch 14; passed.
- Ran `go test ./pkg/config` after Step 1 batch 14; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-license-provider-normalizer-v2.json --markdown .local/agents/config-correction-delta/all.after-license-provider-normalizer-v2.md`; scanned 357 outputs, found 469 remaining review items, and reported 0 script errors (`frequency=76`, `license=23`, `maintainer=177`, `maintainer_url=193`).
- Ran `git diff --check` on the touched tracked SOW support, spec, project skill, helper, catalog test, and catalog paths after Step 1 batch 14; passed.
- Re-ran targeted personal-name and trailing-whitespace scans on the touched durable artifacts, source/catalog files, and 38 locally corrected enrichment outputs; no matches.
- Regenerated `agents/validate-output.py --report` validation reports for the 6 locally corrected provider enrichment outputs after Step 1 batch 15; all passed.
- Ran `python3 -m py_compile agents/enrichment-public.py tools/build-firehol-static-enrichment.py agents/validate-output.py` after Step 1 batch 15; passed.
- Ran `go test ./pkg/config` after Step 1 batch 15; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-license-provider-explicit.json --markdown .local/agents/config-correction-delta/all.after-license-provider-explicit.md`; scanned 357 outputs, found 461 remaining review items, and reported 0 script errors (`frequency=76`, `license=15`, `maintainer=177`, `maintainer_url=193`).
- Ran `go test ./pkg/config` after the merge inheritance implementation; passed.
- Ran `python3 tools/build-firehol-static-enrichment.py --validate` after the merge inheritance implementation; discovered and regenerated 20 FireHOL-maintained feeds, all `[OK]`.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-merge-redistribution-v2.json --markdown .local/agents/config-correction-delta/all.after-merge-redistribution-v2.md`; scanned 357 outputs, found 446 remaining review items, and reported 0 script errors (`frequency=76`, `maintainer=177`, `maintainer_url=193`).
- Ran `go test ./...` after splitting the new redistributability checks out of large posture-guarded test files; passed, including `tools/archposture`.
- Ran `python3 -m py_compile agents/enrichment-public.py` after Step 1 batch 17; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-nonsemantic-delta-normalizers.json --markdown .local/agents/config-correction-delta/all.after-nonsemantic-delta-normalizers.md`; scanned 357 outputs, found 247 remaining review items, and reported 0 script errors (`frequency=76`, `maintainer=110`, `maintainer_url=61`).
- Ran `go test ./pkg/config` after Step 1 batch 18; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-merge-maintainer-config.json --markdown .local/agents/config-correction-delta/all.after-merge-maintainer-config.md`; scanned 357 outputs, found 241 remaining review items, and reported 0 script errors (`frequency=76`, `maintainer=104`, `maintainer_url=61`).
- Ran `git diff --check` on the Step 1 batch 17/18 touched SOW, helper, and catalog paths; passed.
- Searched the Step 1 batch 17/18 touched SOW, helper, and catalog paths for the user's personal name and trailing whitespace; no matches.

### 2026-05-26 Step 1 Validation

- Ran `python3 -m py_compile agents/enrichment-public.py` after Step 1 batch 19; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-maintainer-brand-candidates.json --markdown .local/agents/config-correction-delta/all.after-maintainer-brand-candidates.md`; scanned 357 outputs, found 105 remaining review items, and reported 0 script errors (`frequency=76`, `maintainer=20`, `maintainer_url=9`).
- Re-ran `python3 -m py_compile agents/enrichment-public.py` after updating this SOW; passed.
- Searched the Step 1 batch 19 touched SOW and helper for the user's personal name and trailing whitespace; no matches.
- Ran direct-upstream metadata probes for Step 1 batch 20 against current source/project pages and redirects; no catalog edit was made where the direct source still supported the current display choice or where the enrichment output pointed at a legal owner instead of the feed/project display identity.
- Ran `python3 -m py_compile agents/enrichment-public.py` after Step 1 batch 20; passed.
- Ran `go test ./pkg/config` after Step 1 batch 20; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-maintainer-url-review.json --markdown .local/agents/config-correction-delta/all.after-maintainer-url-review.md`; scanned 357 outputs, found 90 remaining review items, and reported 0 script errors (`frequency=76`, `maintainer=9`, `maintainer_url=5`).
- Ran `git diff --check` on the Step 1 batch 20 touched tracked YAML files; passed.
- Searched the Step 1 batch 20 touched SOW, helper, and YAML files for the user's personal name and trailing whitespace; no matches.
- Ran `python3 -m py_compile agents/enrichment-public.py` after Step 1 batch 21; passed.
- Ran `python3 -m json.tool agents/schemas/feed-enrichment-public.schema.json`; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-frequency-month-normalizer.json --markdown .local/agents/config-correction-delta/all.after-frequency-month-normalizer.md`; scanned 357 outputs, found 88 remaining review items, and reported 0 script errors (`frequency=74`, `maintainer=9`, `maintainer_url=5`).
- Generated `.local/agents/config-correction-delta/frequency-policy-review.json` and `.local/agents/config-correction-delta/frequency-policy-review.md`; corrected frequency review classified 18 strong under-polling candidates, 1 observed over-polling relaxation candidate, and 55 keep-for-review cases.
- Ran `go test ./pkg/config` after Step 1 batch 21; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-frequency-policy-batch21.json --markdown .local/agents/config-correction-delta/all.after-frequency-policy-batch21.md`; scanned 357 outputs, found 69 remaining review items, and reported 0 script errors (`frequency=55`, `maintainer=9`, `maintainer_url=5`).
- Generated `.local/agents/config-correction-delta/frequency-policy-review.after-batch21.json` and `.local/agents/config-correction-delta/frequency-policy-review.after-batch21.md`; no remaining findings qualified for automatic frequency edits under the approved policy.
- Ran `agents/enrichment-public.py project` against `dbip_country`; projected monthly cadence normalized to `30d`.
- Ran `agents/enrichment-public.py project` against `dronebl_autorooting_worms`; projected true one-minute cadence remained `1m`.
- Ran `git diff --check` on the Step 1 batch 21 touched SOW, helper, schema, and YAML paths; passed.
- Searched the Step 1 batch 21 touched SOW, helper, schema, and YAML paths for the user's personal name; no matches.
- Ran `go test ./pkg/config` after Step 1 batch 22; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-frequency-policy-batch22.json --markdown .local/agents/config-correction-delta/all.after-frequency-policy-batch22.md`; scanned 357 outputs, found 65 remaining review items, and reported 0 script errors (`frequency=51`, `maintainer=9`, `maintainer_url=5`).
- Generated `.local/agents/config-correction-delta/frequency-policy-review.after-batch22.json` and `.local/agents/config-correction-delta/frequency-policy-review.after-batch22.md`; no remaining frequency findings qualified for automatic edits under the approved policy plus the daily-jitter rule.
- Ran `python3 -m py_compile agents/enrichment-public.py` after Step 1 batch 23; passed.
- Ran `agents/enrichment-public.py delta --all --json .local/agents/config-correction-delta/all.after-display-choice-normalizer.json --markdown .local/agents/config-correction-delta/all.after-display-choice-normalizer.md`; scanned 357 outputs, found 51 remaining review items, and reported 0 script errors (`frequency=51`).
- Ran `agents/enrichment-public.py delta --all --fields maintainer,maintainer_url,license,redistributable --json .local/agents/config-correction-delta/non-frequency.after-display-choice-normalizer.json --markdown .local/agents/config-correction-delta/non-frequency.after-display-choice-normalizer.md`; scanned 357 outputs, found 0 remaining review items, and reported 0 script errors.
- Ran `agents/enrichment-public.py embed --all --report .local/agents/embed-enrichment/all.dry-run.before-step2-write.json`; mapped 357 entries with 0 errors (`source_items=341`, `merge_items=16`, `hygiene_total=630`).
- Ran `agents/enrichment-public.py embed --all --write --report .local/agents/embed-enrichment/all.write.step2-initial.json`; wrote 357 entries with 0 errors (`source_items=341`, `merge_items=16`, `hygiene_total=630`).
- Counted embedded YAML blocks with `rg -n "^    enrichment:" configs/firehol | wc -l`; found 357.
- Ran `go test ./pkg/config` after the initial Step 2 embed; passed.
- Ran `agents/enrichment-public.py hygiene --embedded --all --json .local/agents/prose-hygiene/embedded.after-step2-embed.json`; scanned 357 embedded YAML blocks with 0 schema failures and 630 hygiene findings.
- Ran `python3 -m py_compile agents/enrichment-public.py` after adding embedded hygiene mode; passed.
- Ran `agents/enrichment-public.py hygiene --embedded --all --json .local/agents/prose-hygiene/embedded.after-mechanical-reflow.json`; scanned 357 embedded YAML blocks with 0 schema failures and 6 hygiene findings, all raw-HTML-looking DroneBL placeholders.
- Ran `go test ./pkg/config` after mechanical prose reflow; passed.
- Ran `agents/enrichment-public.py hygiene --embedded --all --json .local/agents/prose-hygiene/embedded.after-prose-polish.json`; scanned 357 embedded YAML blocks with 0 schema failures and 0 hygiene findings.
- Ran `go test ./pkg/config` after final prose polishing; passed.
- Ran `agents/enrichment-public.py hygiene --embedded --all --fail --json .local/agents/prose-hygiene/embedded.final-step2-after-trim.json`; scanned 357 embedded YAML blocks with 0 schema failures and 0 hygiene findings.
- Ran `go test ./pkg/config` after trimming generated trailing whitespace; passed.
- Ran `git diff --check` on the Step 1/2 touched SOW, specs, helper, schema, and `configs/firehol`; passed after trimming generated trailing whitespace from 259 files.
- Searched the Step 1/2 touched SOW, specs, helper, schema, and `configs/firehol` for the user's personal name; no matches.
- First Step 3 targeted package test run with strict optional date parsing failed on existing accepted enrichment values such as year-only `document_date`, date-only `validation_date`, and month-only `announcement_date`. The embedded Python schema validator already accepted these because `format` annotations are not enforced; Step 3 Go validation now keeps hard checks for schema version, run timestamp, enums, and required descriptions while treating those optional date-granularity fields as display text.
- Ran `go test ./pkg/config ./pkg/enrichment ./pkg/engine ./pkg/markdown` after the validation adjustment; passed.
- Ran `go test ./pkg/config ./pkg/enrichment ./pkg/engine ./pkg/markdown ./pkg/web`; passed.
- Ran `go test ./...`; passed, including `tools/archposture`.
- Ran `make lint`; passed (`go vet ./...`).
- Ran `make build`; passed (`CGO_ENABLED=0 go build ... ./cmd/update-ipsets`).
- Ran `git diff --check` on the Step 3 touched SOW, specs, and Go files; passed.
- Searched the Step 3 touched SOW, specs, and Go files for the user's personal name; no matches.
- Ran `go test ./pkg/web ./pkg/markdown` after removing the about endpoint and extending markdown rendering; passed.
- First focused `pnpm --dir ui test --run feed-detail.test.tsx` after adding the maintainer-stated cadence panel failed the axe `heading-order` rule. Replaced the reused notice component's heading with a local `h3` panel; rerun passed.
- Ran `pnpm --dir ui lint`; passed.
- Ran `pnpm --dir ui build`; passed. Vite still reports the existing runtime-resolved InterDisplay font warnings and Node `DEP0205`; the build completed successfully.
- Ran `pnpm --dir ui test`; passed at that point with 13 files and 38 tests.
- First broad `go test ./...` after Step 4 failed in `tools/archposture` because `ui/src/lib/api-types.ts` grew from 1045 to 1214 lines. Moved the enrichment DTOs to `ui/src/lib/enrichment-types.ts`; reran `go test ./...`; passed, including `tools/archposture`.
- Ran `make lint`; passed (`go vet ./...`).
- Ran `make build`; passed (`CGO_ENABLED=0 go build ... ./cmd/update-ipsets`).
- Added the final `FeedRef` rollout to homepage/table links, overlap rows/lists, merge composition rows, IP lookup/search results, and successor links; added focused behavioral coverage for tooltip rendering and enrichment-aware explorer search.
- First focused tooltip test run after the rollout failed because the feed-detail page has multiple tables and because multiple tooltip instances can contain the same official name. Updated the tests to target the overlap table by content and accept multiple tooltip instances; reran `pnpm --dir ui test --run feed-detail.test.tsx home-explorer.test.tsx explorer-state.test.ts`; passed with 3 files and 8 tests.
- Re-ran `pnpm --dir ui lint`; passed.
- Re-ran `pnpm --dir ui test`; passed with 13 files and 39 tests.
- Re-ran `pnpm --dir ui build`; passed with the same existing runtime-resolved InterDisplay font warnings and Node `DEP0205`.
- Re-ran `go test ./...`; passed, including `tools/archposture`.
- Re-ran `make lint`; passed (`go vet ./...`).
- Re-ran `make build`; passed (`CGO_ENABLED=0 go build ... ./cmd/update-ipsets`).
- Ran final `git diff --check` after the Step 5 SOW/spec/doc/template/code updates; passed.
- Re-ran the personal-name scan across touched SOW, spec, docs, agent, Go, markdown, methodology, UI, lib, page, and test artifacts. The only plain match was the legitimate country name `Costa Rica` in `pkg/markdown/names.go`; a PCRE scan excluding `Costa Rica` returned no matches.
- Ran final `git diff --check`; passed.
- The final personal-name scan initially found one pre-existing comment in `ui/src/components/feed-detail/geo-map.tsx`; sanitized it to refer to `the user`, reran the scan across touched SOW, agent, Go, markdown, methodology, UI, lib, page, and test artifacts; no matches.
- Ran `go test ./pkg/markdown ./pkg/mcp` after Step 5 MCP/feed-markdown changes; first run exposed that `Last researched` was hidden when `sources_consulted` was empty. Updated the template to render the footer when either `run_at` or sources exist; rerun passed.
- Ran `go test ./pkg/engine ./pkg/markdown ./pkg/mcp`; passed.
- Ran `go test ./...`; first Step 5 run failed `tools/archposture` because `TestFeedTemplateWithAllSections` grew beyond the large-function baseline. Moved the new enrichment fixture into a helper; reran `go test ./pkg/markdown ./pkg/mcp ./tools/archposture`; passed.
- Re-ran `go test ./...`; passed, including `tools/archposture`.
- Re-ran `make lint`; passed (`go vet ./...`).
- Re-ran `make build`; passed (`CGO_ENABLED=0 go build ... ./cmd/update-ipsets`).
- Ran Step 6 wrapper validation:
  - `bash -n agents/run-enrichment-pool.sh agents/run-enrichment.sh`; passed.
  - `python3 -m py_compile agents/enrichment-refresh.py agents/enrichment-public.py agents/locate-feed.py`; passed.
  - `agents/run-enrichment-pool.sh --feeds spamhaus_drop --dry-run`; queued exactly `spamhaus_drop`.
  - `agents/run-enrichment-pool.sh --category policy_risk --limit 2 --dry-run`; queued exactly `abuseipdb_1d` and `abuseipdb_30d`.
  - First `python3 agents/enrichment-refresh.py --feeds spamhaus_drop --scope validation --dry-run --summary .local/agents/enrichment-refresh/validation.md` exposed a Python 3.14 dynamic-import/dataclass failure; fixed by registering the loaded module in `sys.modules`.
  - Re-ran the refresh dry-run; passed and produced `.local/agents/enrichment-refresh/validation.md` with `would_write spamhaus_drop configs/firehol/sources/policy_risk/spamhaus_drop.yaml sources hygiene=7`.
  - Ran `agents/enrichment-public.py embed --feeds spamhaus_drop --report .local/agents/embed-enrichment/step6-spamhaus.dry-run.json`; passed with the same projected YAML target.
- Ran final close validation:
  - `make test`; passed.
  - `pnpm --dir ui test`; passed with 13 test files and 39 tests.
  - `make race`; passed, including `tools/dronebl2ipsets`.
  - `make lint`; passed.
  - `make build`; passed.
  - `pnpm --dir ui lint`; passed.
  - `pnpm --dir ui build`; passed, with the existing runtime-resolved InterDisplay font warnings and Node `DEP0205` warning.
  - `git diff --check`; passed.
  - Added-line personal-name scan excluding the country-name phrase; no matches.

## Outcome

Completed.

SOW-0014 now ships the researched feed context as committed, typed catalog metadata:

- 357 source/merge YAML entries carry public `enrichment:` blocks from the latest validator-clean local runs or deterministic FireHOL static generation.
- The three provider-infrastructure multi-source YAML files were split into per-feed files, preserving the normalized catalog source/merge snapshots.
- Direct-upstream license and redistributability rules are documented and enforced by specs, shared agent rules, catalog validation tests, and project skills.
- The engine reads typed enrichment metadata from config, validates it, strips internal-only fields by type, and exposes it through public feed summaries, per-feed metadata, markdown, MCP feed discovery, and the UI.
- The old `static/feed-descriptions/*.html` files and `/api/v1/sets/about/{name}` handler were removed; enrichment is now the descriptive source of truth.
- The UI renders about, listing/unlisting, reputation/community, maintainer cadence, source-consulted, and status-banner sections; feed references now use reusable enriched tooltips across the required surfaces.
- `agents/run-enrichment-pool.sh` now supports scoped refreshes via `--feeds`, `--category`, `--all`, stdin, `--unenriched`, and `--retry-failed`; successful non-dry-run refreshes write YAML, generate a significant-change summary, create a local branch/commit, and open a PR only when a remote and authenticated `gh` are available.

Deferred/rejected mapping:

- The evaluation agent remains intentionally out of scope and is tracked by `.agents/sow/pending/SOW-0091-feed-evaluation-agent.md`.
- The admin UI review queue is not required for SOW-0014 because the PR-based workflow is the approved gate; revisit only if review fatigue becomes an operational problem.
- The SOW-0085 markdown-disabled admin visibility item remains a carry-over outside this SOW.
- The remaining frequency delta findings are diagnostic only under the approved cadence policy; after batch 22 no remaining frequency item qualified for automatic edit.

Artifact maintenance gate:

- `AGENTS.md`: updated with the direct-upstream license/redistribution classification rule and links to the authoritative spec, shared agent include, and public methodology page.
- Runtime project skills: `.agents/skills/project-coding/SKILL.md` updated with the direct-upstream explicit-prohibition redistribution rule.
- Specs: updated `.agents/sow/specs/config.md`, `.agents/sow/specs/feeds.md`, `.agents/sow/specs/files-layout.md`, `.agents/sow/specs/website.md`, and added `.agents/sow/specs/ai-classification-rules.md`.
- End-user/operator docs and public methodology: updated `docs/api/mcp-endpoint.md` and added public methodology pages for researched feed context and license-rule interpretation.
- End-user/operator agent rules: added `agents/shared/classification-rules.md`; added the enrichment agent, schemas, validators, refresh tooling, and static generator.
- SOW lifecycle: this SOW is completed and will be moved from `.agents/sow/current/` to `.agents/sow/done/` in the same commit as the implementation.

## Lessons Extracted

- Optional date fields in public enrichment are display text with mixed real-world granularity; strict RFC3339 validation belongs only on `run_at`, while optional source dates stay human-readable.
- The public embedded schema is a separate contract from the full research-output schema. Full outputs stay local; committed YAML stores only the public projection.
- Public prose hygiene must run against embedded YAML, not only local research output, because manual polishing happens in the committed catalog.
- Feed reference context is more useful when centralized as a reusable UI component and API summary fields, instead of repeated one-off table/link rendering.
- Refresh tooling must separate expensive research execution from deterministic writeback/review. The wrapper owns worker orchestration; the refresh helper owns projection, summary, branch, commit, and optional PR creation.
- Scheduler polling cadence and upstream-stated cadence are different contracts. Local observations are sampling-limited and can prove over-polling only when the configured polling interval is faster than the stated cadence.

## Followup

- **Follow-up SOW (mandatory under Plan v2):** `.agents/sow/pending/SOW-0091-feed-evaluation-agent.md` — design `feed-evaluation.ai` against the as-built enrichment + markdown surfaces; define its UI surface ("AI Evaluation" section), refresh trigger, and feedback-loop guard. User guidance: "the evaluation should get the entire markdown and provide some recommendations and a second opinion (what is this good for, what to be careful about, etc.)."
- **Optional follow-up:** admin UI review queue for AI output (originally D4 option 2). Plan v2's PR-based model makes this less urgent; only revisit if review fatigue becomes a problem.
- **Carry-over from SOW-0085:** surface markdown-disabled state in admin API/UI.

## Regression Log

None yet.
