# SOW-0062 - Symlink-Resistant Public Artifact Serving

## Status

Status: completed

Sub-state: implemented and validated

## Requirements

### Purpose

Harden public artifact serving against symlink escape and traversal edge cases while preserving cache-first serving and `ServeContent` behavior.

### User Request

Review project quality against the four named project skills, identify gaps, and create SOWs for actionable improvements.

### Assistant Understanding

Facts:

- `safePath` uses `filepath.Join`, `filepath.Clean`, and a string-prefix check at `pkg/web/http.go:193`.
- Direct artifact routes resolve a public artifact path and serve the file through this lexical path check in `pkg/web/routes.go`.
- The check blocks normal `..` traversal after cleaning.
- The check does not by itself prevent symlink escape if a symlink exists under a served directory.
- Go has traversal-resistant filesystem APIs such as `os.Root` in modern releases; official context checked: https://go.dev/blog/osroot

Inferences:

- Risk depends on whether untrusted or operator-imported content can create symlinks under `WebDir`, `FilesDir`, or `BaseDir`.
- Even if production risk is low, explicit symlink tests would document the intended trust boundary.

Unknowns:

- Whether install, migration, import, or operator workflows can place symlinks in served trees.

### Acceptance Criteria

- Served-directory trust model is documented in specs or operator docs.
- Public file open paths are symlink-resistant where the served tree can contain untrusted entries.
- Tests cover `..` traversal, symlink escape, ordinary files, missing files, and `ServeContent` headers/ranges where relevant.
- Existing raw feed streaming and bounded artifact cache behavior are preserved.

## Analysis

Sources checked:

- `pkg/web/http.go`
- `pkg/web/routes.go`
- Go `os.Root` official blog.

Current state:

- Lexical path checks protect against simple traversal but not symlink escape.

Risks:

- A symlink under a served tree could point outside the intended root if such a symlink can be introduced.
- A hardening change can accidentally break legitimate artifact names, range requests, or static cache behavior.

## Implications And Decisions

No implementation decision is taken in this pending SOW.

Recommended starting decision:

1. Hardening approach
   - A. Use `os.Root`/rooted opens for file serving where available. Recommended.
     - Pros: path traversal resistance by construction.
     - Cons: requires careful integration with `ServeContent`.
   - B. Reject symlinks by checking `Lstat` on path components.
     - Pros: portable to older Go styles.
     - Cons: easier to get wrong and race-prone.
   - C. Document served directories as trusted and add tests only.
     - Pros: smallest change.
     - Cons: leaves hardening dependent on operator discipline.

## Plan

1. Inventory all public file-serving paths and served roots.
2. Determine whether symlinks can enter these roots through supported workflows.
3. Implement rooted-open or explicit symlink rejection.
4. Add real HTTP tests for symlink escape and normal serving behavior.
5. Update specs/operator docs with the served-tree trust model.

## Execution Log

### 2026-05-01

- Created from the four-skill quality review.
- Moved from pending to current for autonomous security hardening.
- Inventoried public serving paths using `safePath`, `ServeFile`,
  `ServeContent`, direct `os.Open`, and direct `os.ReadFile` under `pkg/web`.
- Added rooted file serving through Go `os.Root` while preserving the bounded
  file cache and `http.ServeContent` behavior.
- Migrated public published-artifact handlers, top-level metadata files,
  country/ASN entity artifacts, critical-infrastructure artifacts, direct
  artifacts, and raw feed downloads to rooted serving.
- Kept raw `.ipset`/`.netset` downloads streaming and outside the long-lived
  artifact cache.
- Added tests for symlink escape rejection and ordinary range serving.
- Updated specs and project coding guidance with served-root safety rules.

## Validation

Acceptance criteria evidence:

- Public published artifact serving now uses `ServeRootedFile` rooted at the
  configured `WebDir`/published output directory.
- Public raw feed download routes now stream with rooted opens from the
  configured raw mirror directory and base data directory.
- `ServeRootedFile` uses `os.Root` so symlinks may be followed only when the
  resolved target stays inside the served root.
- `rg -n "ServeFile\\(|ServeRootedFile\\(|os\\.ReadFile\\(|os\\.Open\\(|safePath\\(" pkg/web -g '*.go'`
  shows public route serving uses `ServeRootedFile`; direct `ServeFile` remains
  only as the legacy cache primitive and cache tests.
- Tests cover symlink escape for top-level public artifacts, raw feed
  downloads, rooted cache serving, ordinary file serving, and range responses.

Tests or equivalent validation:

- `go test ./pkg/web -run 'TestFileCacheRootedServingRejectsSymlinkEscapeAndKeepsServeContent|TestPublicTopLevelArtifactRejectsSymlinkEscape|TestRawFeedRouteRejectsSymlinkEscape'` passed.
- `go test ./pkg/web` passed.

Real-use evidence:

- Pending.

Reviewer findings:

- Go best-practices review found lexical-only path checks in public artifact serving.

Same-failure scan:

- Scanned all `safePath`, `ServeFile`, `ServeRootedFile`, `os.ReadFile`, and
  `os.Open` uses under `pkg/web`.

Artifact maintenance gate:

- AGENTS.md: not needed; no project-wide workflow rule changed.
- Runtime project skills: updated `.agents/skills/project-coding/SKILL.md`
  with the served-root rule.
- Specs: updated `.agents/sow/specs/website.md` and
  `.agents/sow/specs/files-layout.md` with the served-directory trust model.
- End-user/operator docs: not needed; the durable contract belongs in specs.
- End-user/operator skills: not needed; no exported operator skill changed.
- SOW lifecycle: moved from pending to current and then completed here.

Specs update:

- Updated `.agents/sow/specs/website.md` and
  `.agents/sow/specs/files-layout.md`.

Project skills update:

- Updated `.agents/skills/project-coding/SKILL.md`.

End-user/operator docs update:

- Not needed.

End-user/operator skills update:

- Not needed.

Lessons:

- Public file serving needs rooted opens, not just lexical path checks.
- The file cache can preserve `ServeContent` behavior and range requests while
  reading through a rooted filesystem boundary.

Follow-up mapping:

- None.

## Outcome

Completed. Public artifact and raw download routes reject traversal and
symlink escapes while preserving cache-first serving and streaming behavior.

## Lessons Extracted

When a route serves from a configurable directory, use rooted file opens and
test symlink escape explicitly.

## Followup

None yet.
