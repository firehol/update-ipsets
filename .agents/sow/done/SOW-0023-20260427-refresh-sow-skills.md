# SOW-0023 | 2026-04-27 | refresh-sow-skills

## Status

completed — approved documentation/memory edits applied; Costa authorized closure and commit.

## Requirements

### Purpose

Apply Costa's updated SOW framework rules to this repository's agent handbook and review the project skills so future sessions use current, evidence-based operating memory.

### User request quoted verbatim

> $sow I have changed the sow framework, and I want the new rules applied. Also I want to review the project skills you have created.

### Assistant understanding

- Stated: Costa changed the global SOW framework and wants this initialized repository re-reviewed against the new rules.
- Stated: Costa wants the project skills reviewed.
- Inferred: This is a re-review path because `AGENTS.md` already ends with `Project SOW status: initialized`.
- Inferred: Existing SOWs, specs, and skills must be preserved unless Costa approves specific changes.

### Acceptance criteria

1. The current repository SOW setup is audited against the updated SOW skill.
   - Verification: `bash ~/.agents/skills/sow/scripts/audit.sh` output captured in Validation.
2. `AGENTS.md` deltas required by the updated framework are identified with file/line evidence.
   - Verification: numbered findings cite `AGENTS.md` line numbers.
3. Project skills are reviewed against the updated project-skill rules and current repo evidence.
   - Verification: numbered findings cite skill files and source evidence from manifests, CI, or code.
4. Costa approves which deltas to apply before implementation.
   - Verification: decisions recorded under `## Implications and decisions`.
5. Approved edits are implemented and validated.
   - Verification: audit plus targeted file checks; no generated assets touched.

## Analysis

### Sources checked

- `~/.agents/skills/sow/SKILL.md`
- `AGENTS.md`
- `.agents/skills/project-coding/SKILL.md`
- `.agents/skills/project-reviewing/SKILL.md`
- `.agents/skills/project-testing/SKILL.md`
- `.agents/skills/project-operations/SKILL.md`
- `.agents/sow/specs/README.md`
- `go.mod`
- `tools/dronebl2ipsets/go.mod`
- `ui/package.json`
- `ui/tsconfig.app.json`
- `ui/vite.config.ts`
- `ui/components.json`
- `Makefile`
- `.github/workflows/*`

### Facts

- The repo is initialized: `AGENTS.md` final line is `Project SOW status: initialized`.
- SOW audit reports one missing canonical AGENTS section: `### Project-specific overrides`.
- SOW audit reports all four SOW directories exist and all four project skills have valid `SKILL.md` files.
- `AGENTS.md` references `~/.agents/skills/sow/sow-workflow.md`, but that file is absent in the current global SOW skill directory.
- `go.mod` and `tools/dronebl2ipsets/go.mod` both declare Go `1.26.0`.
- `ui/package.json` declares React, Vite, TypeScript, Tailwind, Radix, TanStack, Recharts, D3/VisX, and Three dependencies.
- `ui/tsconfig.app.json` enables `strict`, `noUnusedLocals`, `noUnusedParameters`, and `noFallthroughCasesInSwitch`, and defines `@/*`.
- GitHub Actions currently runs Go build/test/race/vet/cross-build and coverage threshold checks.

### Working theory

The existing project skills are broadly useful but were written under the previous SOW framework. Under the updated rules they need small evidence and wording upgrades, not a destructive rewrite.

### Findings

1. `AGENTS.md` is missing one canonical section required by the updated SOW audit.
   - Evidence: audit reports `### Project-specific overrides (missing)`.
   - Evidence: updated global SOW template requires `### Project-specific overrides`.
2. `AGENTS.md` still contains stale local framework mechanics that should now live in the global SOW skill.
   - Evidence: `AGENTS.md` has an 11-step local pipeline and says per-step gates are in `~/.agents/skills/sow/sow-workflow.md`.
   - Evidence: `/home/costa/.agents/skills/sow/sow-workflow.md` does not exist.
   - Evidence: updated global SOW template says `Global SOW policy lives in ~/.AGENTS.md. This section is project-specific only.`
3. `AGENTS.md` contains a strict delegation rule that is stronger than the updated global SOW language.
   - Evidence: `AGENTS.md` says default is delegation and SOW initialization delegation is mandatory.
   - Evidence: updated global SOW skill says to parallelize where it pays off and that the model decides what investigation is needed; no fixed worker list.
4. The project skills are useful but do not fully satisfy the updated "observed evidence, never aspirations" standard.
   - Evidence: updated global SOW skill requires project-coding to cite example files and quote actual config values; current project-coding lists many conventions but mostly without source citations.
   - Evidence: updated global SOW skill requires project-reviewing project-specific concerns to cite SOWs; current project-reviewing lists concerns without SOW citations.
   - Evidence: updated global SOW skill requires test stack with versions from manifests; current project-testing lists commands and facts but does not cite manifest/CI sources.
5. No new project skill appears necessary from current evidence.
   - Evidence: existing skills cover code changes, reviews, test work, and operations; release/security hygiene is already represented in testing, operations, and reviewing.

## Implications and decisions

Costa approved the recommended path on 2026-04-27:

1. Decision 1: Option A — minimal `AGENTS.md` repair.
2. Decision 2: Option A — evidence-strengthen the existing four skills.
3. Decision 3: Option A — move SOW-0023 to `current/` and apply approved edits now.

### Decision 1 — AGENTS.md refresh scope

Option A — Minimal repair, recommended.
- Add `### Project-specific overrides`, add the global-policy sentence, remove the stale `sow-workflow.md` reference, and collapse duplicated global mechanics into concise project-specific rules.
- Pros: fixes audit failure, reduces stale local policy, low churn.
- Implications: future sessions rely on the global SOW skill for lifecycle details and `AGENTS.md` for project-specific contracts.
- Risks: less local detail in `AGENTS.md`; mitigated by explicit pointer to the global SOW policy.

Option B — Full rewrite to the updated template.
- Pros: strongest template compliance.
- Implications: rewrites a lot of local handbook text.
- Risks: higher chance of losing useful project-specific wording or creating review noise.

Option C — Review only, no AGENTS.md edit.
- Pros: zero file churn.
- Implications: audit remains partial; stale `sow-workflow.md` reference remains.
- Risks: future sessions may follow obsolete local instructions.

### Decision 2 — Project skill refresh scope

Option A — Evidence-strengthen the existing four skills, recommended.
- Keep `project-coding`, `project-reviewing`, `project-testing`, and `project-operations`.
- Add source citations, SOW references where lessons came from, and `(no convention observed; defer to general best practice)` only where needed.
- Pros: preserves the approved skill set and aligns it with the updated rules.
- Implications: skills become slightly denser but more trustworthy.
- Risks: small documentation churn; no behavior risk.

Option B — Full skill rewrite.
- Pros: cleanest structure under the new rules.
- Implications: changes all skill wording and organization.
- Risks: higher chance of losing useful lessons already captured.

Option C — Review only, no skill edit.
- Pros: zero file churn.
- Implications: skills remain useful but below the new evidence standard.
- Risks: future sessions may treat uncited conventions as equally strong facts.

### Decision 3 — Execution

Option A — Move SOW-0023 to `current/` and apply approved edits now, recommended.
- Pros: completes the requested framework refresh in one pass.
- Implications: approved changes will be implemented and audited.
- Risks: none beyond low-risk documentation churn.

Option B — Keep SOW-0023 pending after review.
- Pros: defers all edits.
- Implications: this turn ends as analysis only.
- Risks: the repo remains partially aligned with the updated SOW framework.

## Plan

Single low-risk documentation/memory refresh.

1. Present re-review findings and numbered options to Costa.
2. Record Costa's decisions in this SOW.
3. Apply approved edits only.
4. Run SOW audit and targeted checks.
5. Close this SOW with lessons and skill/spec updates, or pause if Costa chooses review-only.

## Execution log

- 2026-04-27: Opened pending SOW from Costa's request.
- 2026-04-27: Ran initial SOW audit and project-skill evidence review.
- 2026-04-27: Recorded Costa's approval of options `1A 2A 3A` and moved SOW-0023 to `current/`.
- 2026-04-27: Updated `AGENTS.md` to point at global SOW policy, remove stale local pipeline mechanics, and add `### Project-specific overrides`.
- 2026-04-27: Evidence-strengthened the four existing project skills without adding new skills.
- 2026-04-27: Ran SOW audit and targeted stale-reference/whitespace checks.
- 2026-04-27: Costa requested `close it and commit`, authorizing the low-risk documentation-only closure gate and commit.

## Validation

### Acceptance criteria evidence

1. The current repository SOW setup was audited against the updated SOW skill.
   - Evidence: `bash ~/.agents/skills/sow/scripts/audit.sh` reports `SOW initialization complete and clean`.
2. `AGENTS.md` deltas required by the updated framework were identified with file/line evidence.
   - Evidence: findings above record the missing `### Project-specific overrides`, stale `sow-workflow.md` reference, and stricter-than-global delegation rule before implementation.
3. Project skills were reviewed against the updated project-skill rules and current repo evidence.
   - Evidence: skills now cite manifests, CI, source files, and SOW lessons where relevant.
4. Costa approved which deltas to apply before implementation.
   - Evidence: decisions above record approval of options `1A 2A 3A`.
5. Approved edits were implemented and validated.
   - Evidence: changed files are `AGENTS.md` and the four existing project skills; no generated assets touched.

### Real-use evidence

- `bash ~/.agents/skills/sow/scripts/audit.sh` passes cleanly.
- `git diff --check -- AGENTS.md .agents/skills/project-*.md .agents/sow/current/SOW-0023-20260427-refresh-sow-skills.md` passed with no output.
- `rg -n "sow-workflow|step 11|Mandatory SOW pipeline|SOW initialization specifically" AGENTS.md .agents/skills || true` returned no stale references.
- Skill line counts remain below the 200-line refactor threshold: 64, 42, 50, and 39 lines.

### Reviewer findings

- Costa authorized closing this low-risk documentation-only SOW without an independent reviewer by requesting `close it and commit`.
- No unresolved reviewer findings.

### Same-failure-at-other-scales scan

- Stale local-framework references were searched across `AGENTS.md` and `.agents/skills/`.
- The only remaining `sow-workflow` mentions are historical evidence inside this SOW.

### Specs updated

- N/A — reason: Costa authorized closure of this documentation-only SOW; no product/application behavior, API, file layout, pipeline, website/admin, integrity, memory, or compatibility contract changed.

### Skills updated

- Updated `.agents/skills/project-coding/SKILL.md`.
- Updated `.agents/skills/project-reviewing/SKILL.md`.
- Updated `.agents/skills/project-testing/SKILL.md`.
- Updated `.agents/skills/project-operations/SKILL.md`.

### Lessons captured

- Captured below.

## Outcome

Completed.

- `AGENTS.md` now has the missing canonical `### Project-specific overrides` section and points to the global SOW policy instead of duplicating stale framework mechanics.
- The four existing project skills now cite concrete evidence from manifests, CI, source files, specs, and prior SOW lessons.
- No new project skills were added.
- SOW-0023 closed after Costa authorized closure and commit.

## Lessons extracted

- Local `AGENTS.md` should not carry detailed global SOW workflow mechanics that can drift. Keep project-specific rules local and point at the global SOW skill for lifecycle details.
- Project skills remain most useful when every operational rule is tied to observed evidence: a manifest/config, code example, CI command, spec, or prior SOW lesson.

## Followup

None.
