# SOW-0009 | 2026-04-26 | finalize-sow-specs

## Status

completed
repo specs moved under `.agents/sow/specs/` with no compatibility link for the old `specs/` path

## Requirements

Given the SOW skill expects project specs to live under `.agents/sow/specs/`, when this SOW is complete, then the existing project specs must be reconciled with that model.

Given the previous repo handbook said product specs lived under `specs/*.md` and `.agents/sow/specs/` was for process notes only, when changing spec ownership, then the conflict must be made explicit and resolved without losing content.

Given specs are used by future agents, when specs are moved or mirrored, then all references in `AGENTS.md`, `.agents/sow/specs/README.md`, SOWs, and project skills must remain accurate.

Given content loss is forbidden and Costa requested no compatibility link, when files are moved, then old-path references must be updated everywhere with audit evidence and the old `specs/` path must be absent.

## Analysis

Sources to consult when work starts:

- `~/.agents/skills/sow/SKILL.md`
- `~/.agents/skills/sow/sow-workflow.md`
- `~/.agents/skills/sow/sow-init.md`
- `AGENTS.md`
- `.agents/sow/specs/README.md`
- `.agents/sow/specs/*.md`

Current known conflict:

- SOW workflow step 8 says to update `.agents/sow/specs/`.
- This repo previously said product/application specs remained under `specs/*.md`.
- Costa requested moving existing specs to SOW specs.

Decision recorded immediately on 2026-04-26:

- Costa approved the SOW ownership model: repo specs should live under
  `.agents/sow/specs/`.
- Costa clarified that no compatibility link should be kept for the old
  `specs/` path.
- The canonical source of truth must move under the SOW tree.

## Implications and decisions

- This is a documentation and agent-contract migration, not a product feature.
- Moving specs can break references if done without a full path audit.
- No compatibility link should be used. References must be updated to
  `.agents/sow/specs/...`, and the old `specs/` path must be removed.

## Plan

Chunked SOW - reasoning: this changes the repo's instruction architecture.

1. `inventory-spec-references` - medium risk
   - Find every reference to `specs/` and `.agents/sow/specs/`.
2. `choose-ownership-model` - high risk
   - Present options: move, mirror, symlink, or revise project rule.
3. `migrate-specs` - high risk
   - Apply Costa-approved model without content loss.
4. `audit-agent-instructions` - medium risk
   - Update AGENTS/project skills/SOW references.
5. `validate-links` - medium risk
   - Verify paths, audit script, and repo searches.

## Execution log

2026-04-26:

- Recorded Costa's decision that specs must live under the SOW system.
- Recorded Costa's clarification that there must be no compatibility link for
  the old repo-root `specs/` path.
- Moved product/application specs into `.agents/sow/specs/`.
- Removed the old repo-root `specs/` directory.
- Updated `AGENTS.md`, project skills, docs, methodology references, SOW
  references, and internal spec cross-references to point at
  `.agents/sow/specs/`.
- Completed the superseded bootstrap spec SOW (`SOW-0001`) and moved it to
  `.agents/sow/done/` because its old ownership decision was replaced by this
  SOW.

## Validation

- [x] Acceptance criteria evidence

  Evidence:

  - `test ! -e specs` returned `OK: repo-root specs path absent`.
  - `.agents/sow/specs/` now contains the product spec set:
    `README.md`, `admin-ui.md`, `compatibility.md`, `config.md`, `design.md`,
    `downloader.md`, `feeds.md`, `files-layout.md`, `homepage.md`,
    `integrity.md`, `memory-management.md`, `operating-principles.md`,
    `pipeline.md`, `processing-engine.md`, and `website.md`.
  - `AGENTS.md` now says product/application contracts live under
    `.agents/sow/specs/*.md` and there is no repo-root `specs/` compatibility
    path.

- [x] Real-use validation evidence

  Evidence:

  - `bash ~/.agents/skills/sow/scripts/audit.sh` passed.
  - Search for old live references outside the SOW spec path found no stale
    `specs/...` references in `AGENTS.md`, project skills, README/docs, package
    docs, UI, config, Makefile, or install script.

- [x] Cross-model reviewer findings (logged + addressed)

  N/A - reason: this SOW changes documentation/spec ownership and repo
  instruction paths, not runtime code or product behavior. Deterministic path
  inventory and SOW audit were used as the review gate. External assistants were
  not run because Costa did not ask for external second opinions.

- [x] Lessons extracted (or "none, reasoning: ...")

  Evidence: see `## Lessons extracted`.

- [x] Same-failure-at-other-scales check

  Evidence:

  - Updated root instructions, project coding/reviewing skills, child SOW
    references, docs, methodology references, and internal spec references.
  - Completed `SOW-0001` because it carried the superseded old ownership model.

## Outcome

Specs are now canonical under `.agents/sow/specs/`.

The repo-root `specs/` path has been removed with no compatibility link.

Updated artifacts:

- `AGENTS.md`
- `.agents/skills/project-coding/SKILL.md`
- `.agents/skills/project-reviewing/SKILL.md`
- `.agents/sow/specs/*.md`
- `.agents/sow/done/SOW-0001-20260426-bootstrap-specs.md`
- release child SOW references
- docs/methodology references that pointed at the old spec path

## Lessons extracted

- Spec ownership should follow the SOW framework directly: product/application
  contracts live in `.agents/sow/specs/`.
- Do not recreate repo-root `specs/` as a compatibility path.
- Updated artifacts:
  - `AGENTS.md`
  - `.agents/skills/project-coding/SKILL.md`
  - `.agents/skills/project-reviewing/SKILL.md`
  - `.agents/sow/specs/README.md`
