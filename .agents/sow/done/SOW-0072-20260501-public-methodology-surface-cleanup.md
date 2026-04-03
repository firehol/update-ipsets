# SOW-0072 - Public Methodology Surface Cleanup

## Status

Status: completed

Sub-state: validated

## Requirements

### Purpose

Keep public methodology UI copy focused on user interpretation, not implementation details, code paths, or internal artifact mechanics.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- `project-content-surfaces` forbids code paths in public methodology pages.
- `.agents/sow/specs/website.md` also says public methodology must not be an implementation guide.
- `ui/src/pages/methodology.tsx` said methodology pages explain "inputs,
  transformations, and source code paths behind each metric."
- A scan found additional public methodology markdown containing implementation-oriented phrases and paths, including some existing files under `pkg/web/static/methodology/`.

Inferences:

- Some public methodology copy likely drifted toward maintainer/spec content.
- Operator/config details may belong in `docs/` instead of methodology pages.

Unknowns:

- Which methodology pages intentionally include brief operator/API links versus misplaced implementation details.

### Acceptance Criteria

- Methodology index copy no longer promises source-code paths.
- Public methodology pages are scanned for config schemas, code paths, artifact filenames, migration history, internal validation mechanics, and SOW-style decisions.
- Misplaced implementation/operator details are either removed, rewritten for user interpretation, or moved/linked to operator docs.
- Tests or content checks cover the corrected index copy where practical.

## Analysis

Sources checked:

- `ui/src/pages/methodology.tsx`
- `pkg/web/static/methodology/*.md`
- `.agents/skills/project-content-surfaces/SKILL.md`
- `.agents/sow/specs/website.md`

Current state:

- The methodology index visibly promises source-code paths, which conflicts with the content-surface contract.

Risks:

- Public users may see internal implementation details instead of clear interpretation guidance.
- Removing too much detail can make methodology pages less useful; the cleanup must distinguish user-facing limits from maintainer implementation notes.

## Implications And Decisions

No implementation decision is taken in this pending SOW.

Recommended starting decision:

1. Cleanup depth
   - A. Fix only the methodology index copy.
     - Pros: tiny.
     - Cons: leaves likely markdown drift.
   - B. Fix index copy and audit all methodology markdown. Recommended.
     - Pros: addresses the surface as a whole.
     - Cons: requires careful content judgment.
   - C. Move all implementation detail to operator docs.
     - Pros: strong separation.
     - Cons: may over-strip useful interpretation context.

## Plan

1. Identify public methodology surface/audience/success criteria before editing.
2. Fix methodology index copy.
3. Audit methodology markdown for forbidden implementation details.
4. Rewrite or move misplaced content.
5. Run UI/content tests and update skills/specs if new durable rules appear.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Identified public methodology as a public user/operator interpretation
  surface: meaning, interpretation, strengths, limits, missing coverage, and
  false-positive/false-negative risks belong here; code paths, schemas,
  artifact filenames, migration history, and internal validation mechanics do
  not.
- Rewrote the methodology index copy in `ui/src/pages/methodology.tsx` so it
  promises interpretation and limits, not source-code paths.
- Audited all files under `pkg/web/static/methodology/`.
- Removed or rewrote implementation-oriented details from public methodology
  pages, including source paths, "Where in the code" sections, artifact
  filenames, schema examples, YAML/config/source-block wording, and "current
  implementation" sections.
- Added `ui/src/pages/methodology.test.tsx` to verify the index copy stays
  interpretation-focused and does not mention source-code paths.

## Validation

Acceptance criteria evidence:

- Methodology index copy no longer promises source-code paths.
- Public methodology markdown was scanned for forbidden implementation terms.
  Final scan command:
  `rg -n "source code|code path|pkg/|cmd/|internal/|configs/|\\.json|\\.csv|\\.ipset|artifact|filename|file name|mtime|SOW|migration|schema|YAML|Current implementation|Where in the code|source code paths" pkg/web/static/methodology ui/src/pages/methodology.tsx`
  returned no matches.
- Misplaced implementation/operator details were rewritten into user-facing
  interpretation text. No operator-only content needed relocation to `docs/`.
- Test coverage added for the methodology index copy and accessibility.

Tests or equivalent validation:

- `pnpm --dir ui test src/pages/methodology.test.tsx` passed: 1 test.
- `pnpm --dir ui lint` passed.
- `pnpm --dir ui test` passed: 10 files, 23 tests.
- `pnpm --dir ui build` passed with existing unresolved static font warnings.
- `pnpm --dir ui build:budget` passed.

Real-use evidence:

- Public methodology content now matches the existing public surface contract:
  interpretation and limits without internal storage or code references.

Reviewer findings:

- Frontend best-practices/content-surface review found public methodology copy promises source-code paths.

Same-failure scan:

- Same-failure scan passed with no matches for source paths, artifact
  filenames, schemas, YAML/config details, or implementation headings.

Artifact maintenance gate:

- AGENTS.md: no update needed; existing public methodology surface rule was
  sufficient.
- Runtime project skills: no update needed; `project-content-surfaces` already
  contains the durable rule this SOW applied.
- Specs: no update needed; website spec already says public methodology must
  not be an implementation guide.
- End-user/operator docs: updated public methodology pages and methodology
  index UI copy.
- End-user/operator skills: no update needed.
- SOW lifecycle: moved from pending to current; will move to done after
  validation.

Specs update:

- Not needed.

Project skills update:

- Not needed.

End-user/operator docs update:

- Updated `pkg/web/static/methodology/*.md` where implementation details leaked
  into public methodology content.

End-user/operator skills update:

- Not needed.

Lessons:

- Public methodology pages should describe what signals mean and where they can
  mislead. Storage formats, code paths, and maintainer implementation trails
  belong in specs, docs, or SOWs instead.

Follow-up mapping:

- None.

## Outcome

Completed.

## Lessons Extracted

Public methodology content needs an explicit surface check before copying from
specs or implementation notes. If a paragraph names a source path, storage
artifact, config schema, or internal validation path, it probably belongs
outside public methodology.

## Followup

None.
