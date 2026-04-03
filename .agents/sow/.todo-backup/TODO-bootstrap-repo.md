# Bootstrap Repo Agent Compatibility

## TL;DR

Purpose: normalize this repository for cross-tool agent compatibility using the `bootstrap-repo` skill.

Requested final state:

- `AGENTS.md` remains the single instruction source of truth.
- `CLAUDE.md` is a relative symlink to `AGENTS.md`.
- `GEMINI.md` is a relative symlink to `AGENTS.md`.
- `.agents/skills/` is the single project skills directory.
- `.claude/skills` is a relative symlink to `../.agents/skills`.
- No other root agent instruction files or symlinks remain.

## Analysis

Pre-flight audit on 2026-04-26 with the earlier skill version found:

- Git repository detected.
- Working tree clean.
- `AGENTS.md` is a regular file.
- `CLAUDE.md` is already a relative symlink to `AGENTS.md`.
- `QWEN.md` is already a relative symlink to `AGENTS.md`; the skill says existing symlinks to `AGENTS.md` should be left alone.
- `.agents/` does not exist.
- `.claude/` exists.
- `.claude/skills` does not exist.
- Target state is 3/5 complete; only skills directory/symlink setup is needed.

Pre-flight audit on 2026-04-26 with the updated recursive skill found:

- Git repository detected.
- Working tree clean.
- `AGENTS.md` is a regular file.
- `CLAUDE.md` is already a relative symlink to `AGENTS.md`.
- `OPENCODE.md` is a symlink to `AGENTS.md`, but the updated skill treats it as an orphan because Opencode reads `AGENTS.md` natively.
- `QWEN.md` is a symlink to `AGENTS.md`, but the updated skill treats it as an orphan because Qwen-code reads `AGENTS.md` natively.
- `GEMINI.md` is missing; the updated skill requires `GEMINI.md -> AGENTS.md` because Gemini CLI reads `GEMINI.md` by default.
- `.agents/skills/` exists as a real directory.
- `.claude/skills` is already a relative symlink to `../.agents/skills`.
- No subdirectories have agent instruction files.
- Target state is 5/8 complete.

## Decisions

No merge conflicts exist. No content merge is required.

Removal decisions required by Costa under the updated skill:

- `OPENCODE.md` is a redundant symlink to `AGENTS.md`; removing it loses no authored content, but still requires explicit approval.
- `QWEN.md` is a redundant symlink to `AGENTS.md`; removing it loses no authored content, but still requires explicit approval.

Decision made by Costa:

- Remove all proposed orphan symlinks:
  - `OPENCODE.md`
  - `QWEN.md`

## Plan

1. [x] Create `.agents/skills/`.
2. [x] Ensure `.claude/` exists.
3. [x] Create relative symlink `.claude/skills -> ../.agents/skills`.
4. [x] Add `.agents/skills/.gitkeep` so Git preserves the empty skills directory.
5. [ ] Create relative symlink `GEMINI.md -> AGENTS.md`.
6. [ ] Remove approved redundant `OPENCODE.md` and `QWEN.md` symlinks.
7. [ ] Re-run the latest audit script.
8. [ ] Run manual symlink checks.
9. [ ] Commit explicit changed paths if verification passes.

## Implied Decisions

- The updated skill requires `GEMINI.md -> AGENTS.md`; this is no longer optional.
- The updated skill forbids `OPENCODE.md` and `QWEN.md` at the root, even as symlinks.
- `.agents/skills/.gitkeep` is required because Git cannot track an empty directory.

## Testing Requirements

- Re-run bootstrap audit.
- Manual checks from the skill.

## Documentation Updates Required

None expected unless verification shows a repo-specific note is needed.
