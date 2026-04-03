---
name: project-content-surfaces
description: "Surface/audience discipline for update-ipsets docs, methodology pages, specs, SOWs, UI copy, and admin UI. MUST be followed when changing non-code user/operator/agent-facing content."
---

## Surface Contract

Before changing any non-code content, identify the surface, audience, job, success criteria, forbidden content, and where misplaced content belongs.

| Surface | Audience | Success criteria | Forbidden here |
|---|---|---|---|
| SOWs | future agents/maintainers | purpose, evidence, decisions, plan, validation, lessons are recoverable | polished public docs, UI copy, tutorials |
| Specs | future agents/maintainers | product behavior can be predicted without rereading all code | marketing copy, implementation walkthroughs, aspirational behavior |
| Public methodology pages | feed users and operators interpreting the site | explains what the signal means, why it matters, levels/taxonomy, how to interpret it, strengths, weaknesses, missing coverage, false-positive/false-negative risks | config schemas, code paths, artifact filenames, migration history, internal validation mechanics, SOW decisions |
| Operator docs / README | deployers and operators | users can configure, run, debug, and call supported APIs | broad product methodology unless linked |
| Public UI copy | feed users scanning a page | fast interpretation, local context, clear next action | hidden implementation details unless needed to avoid a wrong conclusion |
| Admin UI copy | service operators | current state, controls, consequences, and recovery steps are clear | end-user editorial methodology |
| Backend/frontend code comments | maintainers | non-obvious why/invariant is clear | user documentation |
| Project skills | future agents | durable working rules that prevent repeat mistakes | product explanations or one-off SOW history |

## Placement Rules

- "Why this signal matters, how to interpret it, and where it is incomplete" belongs in public methodology.
- "How to configure, operate, debug, or call the API" belongs in `docs/`, `README.md`, or an operator/admin surface.
- "What the product guarantees" belongs in `.agents/sow/specs/`.
- "What was decided, validated, or learned during one work item" belongs in the SOW.
- "How agents must work here next time" belongs in project skills or `AGENTS.md`.
- "Deferred" SOW work is not recoverable unless it is represented in the
  ledger. A SOW may record a deferral only with a concrete pending SOW path, or
  may record a rejection/non-goal with evidence. Do not leave valid work as
  prose-only future intent.

Never copy SOW/spec text into public docs without rewriting it for the target audience.

## Review Gate

For every content change, check:

- Does the artifact serve its own audience, not the audience of the source text?
- Would a careful user learn the practical meaning and limits of the signal without seeing internal implementation?
- Did operator/config/API details move to operator docs instead of methodology pages?
- Did durable behavioral contracts move to specs and durable process lessons move to skills?
- Are UI default ordering, labels, and emphasis aligned with the risk model the feature represents?

## SOW-0017 Regression Lesson

Critical-infrastructure methodology regressed when internal implementation details were promoted into a public explanation page. Critical-infrastructure UI also regressed when matched reference feeds were ordered by matched IP count instead of criticality. Future critical-infrastructure content must keep user interpretation first: hard risk before volume, then soft/contextual detail.
