# SOW-0005 | 2026-04-26 | bootstrap-repo

## Status

completed
implemented before SOW initialization and migrated with evidence

## Requirements

Given multiple agent tools need consistent repo instructions, when bootstrap is complete, then `AGENTS.md` must remain the instruction source of truth and supported tool files should point to it.

Given project skills need a stable location, when bootstrap is complete, then `.agents/skills/` must exist and `.claude/skills` must point to it.

## Analysis

Original source:

- `.agents/sow/.todo-backup/TODO-bootstrap-repo.md`

Evidence:

- Commit `772b1ff .agents` created `.agents/skills/.gitkeep` and `.claude/skills`.
- Commit `005f9ec Bootstrap repo agent compatibility` added `GEMINI.md -> AGENTS.md` and removed redundant `OPENCODE.md` and `QWEN.md` symlinks.
- Current target state has `CLAUDE.md -> AGENTS.md`, `GEMINI.md -> AGENTS.md`, `.agents/skills/`, and `.claude/skills -> ../.agents/skills`.

## Implications and decisions

- Costa approved removing redundant root symlinks for tools that read `AGENTS.md` directly.
- The current SOW initialization builds on the bootstrap-repo result.

## Plan

Single-unit implementation, no chunking - reasoning: this work was already completed before SOW initialization.

## Execution log

2026-04-26:

- Migrated completed bootstrap TODO into SOW history.
- Original TODO preserved at `.agents/sow/.todo-backup/TODO-bootstrap-repo.md`.

## Validation

- [x] Acceptance criteria evidence
  - Current symlink/directory state and commits listed in Analysis satisfy the target state.
- [x] Real-use validation evidence
  - Bootstrap audit previously passed after commit `005f9ec`.
- [x] Cross-model reviewer findings (logged + addressed)
  - N/A - completed before SOW system; no active SOW review cycle existed.
- [x] Lessons extracted (or "none, reasoning: ...")
  - Lesson captured by SOW initialization: project skills live under `.agents/skills/project-*`.
- [x] Same-failure-at-other-scales check
  - Root agent files were checked for redundant/orphan symlinks.

## Outcome

Repo agent compatibility was normalized around `AGENTS.md` and `.agents/skills/`.

## Lessons extracted

- Keep `AGENTS.md` compact and use project skills for detailed repeatable workflows.
