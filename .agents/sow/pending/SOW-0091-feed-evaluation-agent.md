# SOW-0091 - Feed Evaluation Agent

## Status

Status: open

Sub-state: created as the concrete follow-up for the evaluation work deferred from SOW-0014.

## Requirements

### Purpose

Add a clearly-labeled AI Evaluation section for feed pages after SOW-0014 ships the enrichment-backed markdown/API/UI surfaces. The evaluator should read the full public feed markdown, excluding any previous AI Evaluation output, and provide a neutral second opinion about what the feed is good for, what to be careful about, and how to interpret its risks and limits.

### User Request

The original SOW-0014 request asked for a second agent that reads the feed markdown page and evaluates the feed itself. Later guidance: the evaluation should get the entire markdown and provide recommendations and a second opinion, including what the feed is good for and what to be careful about.

### Assistant Understanding

Facts:

- SOW-0014 now scopes only the enrichment integration.
- The evaluator input contract depends on the as-built SOW-0014 markdown template and public enrichment surface.
- The evaluator must not consume its own previous evaluation output.

Inferences:

- The evaluation agent should be designed after SOW-0014 is completed so its input contract is concrete.
- The evaluation schema, markdown template split, refresh trigger, UI placement, and feedback-loop guard need their own focused review.

Unknowns:

- Final evaluator schema.
- Final evaluation model/provider.
- Whether evaluation output is stored in source YAML, a separate committed artifact, or a generated/published artifact.

### Acceptance Criteria

- `feed-evaluation.ai` exists with a strict schema and neutral tone rules.
- Evaluator input uses feed markdown that includes enrichment but excludes prior AI Evaluation output.
- Evaluation output has a clearly-labeled public section and cannot be mistaken for maintainer-provided facts.
- Feedback-loop guards prevent consumption of iplists.firehol.org-derived evaluation output.
- Storage, refresh, review, and rollback model are documented and tested.
- Specs, docs, skills, and SOW follow-up mapping are updated.

## Analysis

Sources checked:

- SOW-0014 deferred evaluation work to this SOW.

Current state:

- No evaluation agent is implemented in this SOW.
- SOW-0014 enrichment surfaces are not completed yet.

Risks:

- Evaluation can sound like authoritative judgment rather than a labeled second opinion.
- Evaluation can become self-referential if it consumes previous AI Evaluation text.
- Evaluation may overfit transient feed metrics unless the prompt separates stable enrichment facts from live engine observations.

## Pre-Implementation Gate

Status: blocked

Problem / root-cause model:

- The evaluator cannot be specified safely until SOW-0014 finishes the enrichment-backed markdown/API/UI contract.

Evidence reviewed:

- `.agents/sow/pending/SOW-0014-20260426-ai-in-the-loop.md`

Affected contracts and surfaces:

- `agents/`
- feed markdown templates
- feed detail UI
- public API or published artifact that serves evaluation output
- MCP `fetch_analysis`
- public methodology pages
- specs under `.agents/sow/specs/`

Existing patterns to reuse:

- SOW-0014 enrichment agent and schema patterns once completed.

Risk and blast radius:

- Public AI judgment on maintainer work has reputational risk; labels, disclaimers, source separation, and neutral language are mandatory.

Sensitive data handling plan:

- Evaluation output is public-by-design. It must not include raw IP addresses from community reports, personal data, private endpoints, credentials, or internal reasoning.

Implementation plan:

1. Reopen after SOW-0014 is completed and review the final feed markdown/API/UI contract.
2. Define evaluator input markdown that excludes previous AI Evaluation output.
3. Design schema, prompt, storage, refresh, review, and rollback model.
4. Implement agent, wrapper, renderer, tests, specs, docs, and methodology updates.

Validation plan:

- Schema validation for evaluator output.
- Golden markdown tests proving previous AI Evaluation is excluded from input.
- UI/API/MCP tests proving labels and disclaimers render correctly.
- Feedback-loop scans over all cited and fetched URLs.

Artifact impact plan:

- AGENTS.md: no expected update unless project-wide AI rules change.
- Runtime project skills: likely update or create an AI-integration skill after implementation.
- Specs: update feed knowledge, file layout, public website, and MCP/markdown contracts as needed.
- End-user/operator docs: document evaluation meaning, limits, and refresh workflow.
- End-user/operator skills: update if an operator skill is created for refresh/evaluation.
- SOW lifecycle: this SOW remains pending until SOW-0014 completes.

Open-source reference evidence:

- None beyond SOW-0014 yet; this SOW is blocked on local contract completion.

Open decisions:

- Pending until SOW-0014 is completed.

## Implications And Decisions

Pending.

## Plan

Pending until SOW-0014 completes.

## Execution Log

### 2026-05-24

- Created as the concrete follow-up for the evaluation work deferred from SOW-0014.

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
