# SOW Initialization

## TL;DR

Purpose: initialize the SOW framework in this repository so future non-trivial
work is tracked through `.agents/sow/` instead of root TODO files.

Current request: `$sow initialize this repo`.

## Analysis

Verified facts from `~/.agents/skills/sow/SKILL.md` and `sow-init.md`:

- SOW initialization is required because `AGENTS.md` does not contain
  `Project SOW status: initialized`.
- Initialization must not begin until Costa answers:
  - full vs medium analysis
  - light scaffolding vs heavy bootstrap-specs
  - available for steering vs unattended
- Initialization requires delegated repo analysis.
- Initialization will eventually rewrite `AGENTS.md`, create `.agents/sow/`
  directories, create project skills, and migrate root `TODO-*.md` files.
- No content may be removed without explicit approval.

Verified repo state:

- `AGENTS.md` exists.
- Bootstrap-repo current working tree target state is complete after adding
  `GEMINI.md -> AGENTS.md` and removing approved `OPENCODE.md` / `QWEN.md`
  symlinks.
- Git has staged/uncommitted bootstrap changes:
  - `GEMINI.md` added
  - `OPENCODE.md` removed
  - `QWEN.md` removed
- SOW audit reports missing canonical SOW AGENTS sections, missing
  `.agents/sow/{specs,pending,current,done}/`, no `project-*` skills, and root
  TODO files not yet migrated.
- Root TODO files currently reported by SOW audit:
  - `TODO-admin-ui-workspace.md`
  - `TODO-bootstrap-repo.md`
  - `TODO-config-catalog-decomposition.md`
  - `TODO-release-master.md`

Phase A delegated analysis findings:

- The repository is a Go daemon/CLI with an embedded React/Vite SPA, a
  directory-based YAML feed catalog, and one nested Go helper module for
  DroneBL.
- The current `AGENTS.md` is mostly useful repo orientation and rules, but it
  lacks the canonical SOW sections and marker.
- The project role should be treated as maintainer/local-maintainer based on
  git evidence: all reachable commits are authored by Costa, and there is no
  configured remote or CODEOWNERS file to prove an external ownership model.
- Existing product/application specs already live under `specs/*.md`; SOW init
  must not silently move or duplicate their authority under `.agents/sow/specs/`.
- CI validates Go build/test/race/vet/cross-build and coverage only. UI
  build/lint scripts exist but are not in CI.
- `go test ./...` does not cover the nested `tools/dronebl2ipsets` module.
- Root `TODO-*.md` files are ignored by `.gitignore`, so they must be backed up
  before migration and not removed until explicitly approved.
- TODO migration recommendation:
  - `TODO-admin-ui-workspace.md`: implemented, migrate to `done`.
  - `TODO-bootstrap-repo.md`: implemented process TODO, migrate to `done` or
    backup-only.
  - `TODO-config-catalog-decomposition.md`: core implemented, migrate core to
    `done`; keep contributor-doc gap in the release SOW.
  - `TODO-release-master.md`: active mixed tracker, split into release-current
    and release-pending SOWs, preserving decisions and evidence.
  - `TODO-sow-initialization.md`: active init tracker, migrate to `current`.

## Decisions

Costa decisions:

- Analysis depth: `A` full analysis.
- Init weight: `B` heavy init, including a bootstrap-specs SOW.
- Steering mode: `A` Costa is available for Phase B decisions.
- Git state: Costa requested committing bootstrap-repo changes first; commit
  `005f9ec Bootstrap repo agent compatibility` completed before SOW init
  analysis.
- Phase B recommendation package approved with `all`:
  - Keep `specs/*.md` as product/application truth; use `.agents/sow/specs/`
    only for SOW/process notes.
  - Create `project-coding`, `project-reviewing`, `project-testing`, and
    `project-operations` skills.
  - Migrate completed TODOs to `done`, active release work to `current`, keep
    original TODOs backed up, and ask again before removing root files.
  - Create one active `release-readiness` SOW and keep future phases as grouped
    followups inside it.
  - Create `SOW-0001-bootstrap-specs` as current.
  - Preserve concise repo rules in `AGENTS.md`, move detailed workflow/navigation
    into project skills, and show the `AGENTS.md` diff before writing it.

## Plan

1. [x] Wait for Costa's decisions.
2. [x] Run mandatory delegated Phase A analysis according to chosen depth.
3. [x] Present Phase B recommendations.
4. [ ] After approval, execute SOW initialization.
5. [ ] Verify with SOW audit.

## Implied Decisions

- Do not migrate or remove any root TODO file until SOW init reaches the
  explicit migration approval gate.
- Do not rewrite `AGENTS.md` until Costa approves the Phase B recommendation
  package and the AGENTS diff.

## Testing Requirements

- `bash ~/.agents/skills/sow/scripts/audit.sh`
- Verify `AGENTS.md` marker is the final line only after all initialization
  steps succeed.

## Documentation Updates Required

- `AGENTS.md` SOW section and marker.
- Project skill files under `.agents/skills/project-*`.
- Migrated SOW files under `.agents/sow/`.
