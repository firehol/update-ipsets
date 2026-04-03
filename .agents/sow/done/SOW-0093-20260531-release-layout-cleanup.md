# SOW-0093 - Release Layout Cleanup

## Status

Status: completed

Sub-state: Release layout cleanup implemented, wiki mirror correction applied, validated, and pushed.

## Requirements

### Purpose

Prepare the first public release repository layout so research notes, operational scripts, documentation publishing, and CleanTalk feed semantics are clear and maintainable for public users and operators.

### User Request

The user requested:

- move `RESEARCH-asn-providers.md` under the research/knowledge area as `asn-providers.md`
- move all root `research/` material to `.agents/knowledge/`
- keep only the GLM enterprise-thread research and name it `thread-feed-for-enterprises.md`
- move `sync-*.sh` scripts under `scripts/`
- keep `docs/` as the source of truth and mirror it into the GitHub Wiki automatically; the wiki is destination-only
- fix `configs/firehol/merges/cleantalk*` because the suffixed CleanTalk feeds are history/retention windows, not ordinary authored merges

### Assistant Understanding

Facts:

- `RESEARCH-asn-providers.md` is currently at the repository root.
- `research/enterprise-glm.md`, `research/enterprise-m27.md`, and `research/enterprise-qwn.md` exist; the user wants only the GLM version kept.
- `sync-from-bash-version.sh` and `sync-from-d1.sh` are currently repository-root scripts.
- The user's intended pattern is the existing `ai-agent.git` workflow: mirror `docs/` into the GitHub Wiki on docs changes; the wiki is never edited directly.
- GitHub built-in Wikis are backed by `.wiki.git` repositories, but that repository is only the destination for the mirror. The source of truth remains `docs/` in this repository.
- GitHub Wiki uses `Home.md` and `_Sidebar.md` specially; `docs/Home.md` already exists and is the correct landing page source.
- `configs/firehol/merges/cleantalk_1d.yaml`, `_7d.yaml`, and `_30d.yaml` are authored as merges over `cleantalk_new_*` and `cleantalk_updated_*`.
- The config loader currently expands `history:` only on `sources:` entries before expanding `merges:`.
- `cleantalk_new` and `cleantalk_updated` already declare `history: [1440, 10080, 43200]`.

Inferences:

- The unsuffixed `cleantalk` feed is still a real merge because it unions the two current CleanTalk source views.
- The suffixed `cleantalk_1d`, `cleantalk_7d`, and `cleantalk_30d` identities should remain public feed identities but be generated as retention derivatives of the `cleantalk` merge.
- Supporting `history:` on merge declarations is the smallest model-aligned fix because merge processing already writes downloader-owned history snapshots when a merge has retention dependents.

Unknowns:

- None blocking implementation.

### Acceptance Criteria

- Root research files are gone; retained knowledge lives under `.agents/knowledge/` with the requested filenames.
- Only the GLM enterprise research survives as `.agents/knowledge/thread-feed-for-enterprises.md`.
- Sync scripts live under `scripts/`, with docs/spec references updated.
- `.github/workflows/wiki-sync.yml` mirrors `docs/` to the GitHub Wiki on pushes to `main` or `master` that touch docs or the workflow.
- `docs/Home.md` remains the wiki landing page; no duplicate `docs/index.md` entry is kept.
- GitHub Pages is disabled for this repository because docs are published through the GitHub Wiki mirror.
- CleanTalk suffixed window feeds are generated as retention derivatives of the `cleantalk` merge, not authored merge files.
- Config docs/specs/tests describe and verify merge-level history derivatives.
- Validation covers config/catalog tests, docs references, SOW audit, and Git status.

## Analysis

Sources checked:

- `RESEARCH-asn-providers.md`
- `research/enterprise-glm.md`
- `research/enterprise-m27.md`
- `research/enterprise-qwn.md`
- `sync-from-bash-version.sh`
- `sync-from-d1.sh`
- `docs/Home.md`
- `docs/migration-from-bash.md`
- `.agents/sow/specs/compatibility.md`
- `configs/firehol/merges/cleantalk*.yaml`
- `configs/firehol/sources/service_abuse/cleantalk_new.yaml`
- `configs/firehol/sources/service_abuse/cleantalk_updated.yaml`
- `pkg/config/config.go`
- `pkg/config/expand.go`
- `pkg/engine/download_stage.go`
- `pkg/engine/feed_body_stage.go`
- `~/src/ai-agent.git/.github/workflows/wiki-sync.yml`

Current state:

- `pkg/config/expand.go` expands source `history:` first, then expands merges.
- `pkg/config/config.go` has `History []int` only on `Source`, not on `Merge`.
- `pkg/engine/download_stage.go` appends history snapshots for merges when retention dependents exist.
- `pkg/engine/feed_body_stage.go` composes retention derivatives from a parent feed body plus downloader-owned history snapshots.
- `docs/Home.md` is already the Wiki-style landing page.
- `docs/_Sidebar.md` is maintained by hand and must point to `Home.md`.
- `gh repo view firehol/update-ipsets --json hasWikiEnabled,defaultBranchRef,url` reports `hasWikiEnabled=true` and default branch `main`.
- A `git ls-remote` check against the wiki remote initially failed because the wiki repository had not been initialized yet.

Risks:

- Moving scripts can break documented commands if `docs/` and specs are not updated.
- Mirroring docs into the GitHub Wiki makes docs public through the wiki; sensitive-data scans must include docs and knowledge files.
- Changing CleanTalk provenance can affect admin/API labels, public feed pages, and catalog tests; the public names must remain stable.
- Deleting two research files is intentional per user request but is irreversible after commit without history recovery.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- Repository layout has release-polish issues: knowledge artifacts are split across root and `research/`, migration scripts are in the root, and docs are prepared in a Wiki-like shape but need a repo-owned wiki mirror workflow.
- CleanTalk has a semantic modeling issue: `cleantalk_1d`, `_7d`, and `_30d` are currently authored as merges under `configs/firehol/merges/`, but their suffixes and input names represent history/retention windows. The product already has a retention derivative concept, but the loader only supports `history:` on sources. Adding merge-level `history:` lets the unsuffixed merge stay a merge while the suffixed variants become true retention derivatives.

Evidence reviewed:

- `pkg/config/expand.go` source-history expansion and merge expansion.
- `pkg/config/config.go` `Source.History` and `Merge` fields.
- `configs/firehol/merges/cleantalk_1d.yaml` references `cleantalk_new_1d` and `cleantalk_updated_1d`.
- `configs/firehol/sources/service_abuse/cleantalk_new.yaml` declares `history: [1440, 10080, 43200]`.
- `pkg/engine/download_stage.go` merge fetch path appends history snapshots and extends history derivative decisions.
- `pkg/engine/feed_body_stage.go` retention composition reads parent snapshots from `HistoryDir`.
- The local `ai-agent.git` workflow provides the desired GitHub Wiki mirror pattern.

Affected contracts and surfaces:

- Repository layout for durable knowledge artifacts.
- Operator scripts and docs that reference migration helper commands.
- GitHub repository Wiki mirror workflow and remote wiki initialization.
- Config schema/model for merge declarations.
- Catalog YAML for CleanTalk.
- Public/admin provenance semantics for CleanTalk suffixed feeds.
- Specs and operator docs for history derivatives and merge feeds.
- Tests validating catalog shape and merge/history expansion.

Existing patterns to reuse:

- `history:` derivative generation and `ProvenanceSecondaryRetention` from `pkg/config/expand.go`.
- `ProvenanceSecondaryMerge`, `MergeSources`, and `MergeExclude` from merge expansion.
- `docs/Home.md` as the docs entry content.
- Existing migration docs/spec wording around `sync-from-bash-version.sh`.
- Existing SOW lifecycle and validation gates.

Risk and blast radius:

- Low to medium. File moves and docs publishing are low-risk, but CleanTalk config semantics affect runtime catalog expansion. Public feed names must remain stable and the total catalog shape should remain predictable.

Sensitive data handling plan:

- Do not add raw secrets, credentials, tokens, private endpoints, personal data, customer names, customer identifiers, non-private customer-identifying IPs, or proprietary incident details to SOWs, specs, docs, knowledge files, scripts, or code comments.
- Keep evidence to file paths, command results, and official documentation URLs.
- Run the SOW audit sensitive-data guardrail and Gitleaks after changes.

Implementation plan:

1. Move research/knowledge and script files with explicit paths; update docs/spec references.
2. Add `.github/workflows/wiki-sync.yml` so `docs/` is mirrored into the GitHub Wiki, and keep `docs/Home.md` as the wiki landing page.
3. Extend config expansion to support `history:` on merges, then change CleanTalk catalog YAML so `cleantalk` owns `history:` and the suffixed CleanTalk files are no longer authored merges.
4. Update docs/specs/tests/static-enrichment helper wording for merge-level history derivatives.
5. Disable the mistakenly configured GitHub Pages site, push the amended release, and verify the GitHub Wiki mirror.

Validation plan:

- `go test ./pkg/config ./pkg/engine`
- `make test` if targeted tests pass
- `.agents/sow/audit.sh`
- `git diff --check`
- Export the staged Git tree to a temporary directory and scan it with Gitleaks `dir` to avoid ignored local work logs.
- GitHub Wiki workflow and wiki URL verification after push.

Artifact impact plan:

- AGENTS.md: no expected update; repository-wide rules are unchanged.
- Runtime project skills: update release publishing guidance to the wiki mirror workflow.
- Specs: update config/compatibility specs for merge-level history and script path.
- End-user/operator docs: update docs for script path, wiki sidebar entry, history derivatives, and merge docs.
- End-user/operator skills: none expected.
- SOW lifecycle: complete this SOW and move it to `.agents/sow/done/` with the work.

Open-source reference evidence:

- No mirrored open-source reference check needed; this is repository layout, GitHub configuration, and existing local config model work.

Open decisions:

- Resolved by user correction: use `docs/` as the source of truth and mirror it into the GitHub Wiki; do not use GitHub Pages for this release.

## Implications And Decisions

1. Knowledge layout
   - Decision: Move root/research artifacts into `.agents/knowledge/`, keep only the GLM enterprise research as `thread-feed-for-enterprises.md`.
   - Implication: Two alternate enterprise drafts are removed from the public repository tree.

2. Script layout
   - Decision: Move `sync-from-bash-version.sh` and `sync-from-d1.sh` to `scripts/`.
   - Implication: Operator docs and specs must use `./scripts/...` paths.

3. Documentation publishing
   - Decision: Mirror `docs/` into the GitHub Wiki with `.github/workflows/wiki-sync.yml`; the wiki is destination-only and must not be edited directly.
   - Implication: GitHub Pages is disabled. The public docs URL is the GitHub Wiki, and `docs/Home.md` / `docs/_Sidebar.md` control the wiki landing page and navigation.

4. CleanTalk semantics
   - Decision: Keep unsuffixed `cleantalk` as a real merge and make `cleantalk_1d`, `cleantalk_7d`, and `cleantalk_30d` generated retention derivatives of that merge.
   - Implication: The public feed names remain, but their provenance becomes `secondary_retention` instead of `secondary_merge`.

5. Release dependency alert
   - Decision: Treat the post-push high Dependabot alert for `d3-color < 3.1.0` as part of first-release hygiene.
   - Implication: Remove the stale `react-simple-maps` dependency path and render the vendored TopoJSON with direct `d3-geo` / `topojson-client` dependencies, then validate the frontend before repushing the amended one-commit release.

## Plan

1. Move files and update references.
2. Implement merge-level history expansion and tests.
3. Update CleanTalk YAML, docs, specs, and static enrichment helper wording.
4. Validate locally.
5. Commit, push, disable Pages, verify the wiki mirror, and verify remote state.

## Execution Log

### 2026-05-31

- SOW opened for post-release layout cleanup.
- Moved root/research knowledge files into `.agents/knowledge/`.
- Removed the two non-GLM enterprise research drafts per user request.
- Moved sync helpers into `scripts/` and updated docs/spec references.
- Added merge-level `history:` expansion and CleanTalk catalog changes so `cleantalk_1d`, `cleantalk_7d`, and `cleantalk_30d` are retention derivatives of `cleantalk`.
- Added `.github/workflows/wiki-sync.yml`, removed the duplicate `docs/index.md`, and kept `docs/Home.md` as the wiki landing page.
- Post-push verification reported Dependabot alert `GHSA-36jr-mh4h-2g58` through the transitive path `react-simple-maps -> d3-zoom@2 -> d3-color@2.0.0`; a narrow `d3-zoom` override was rejected because `d3-color@2.0.0` remained through older D3 transition/interpolation packages.
- Replaced `react-simple-maps` with direct `d3-geo` / `topojson-client` map rendering, added browser coverage for the generated country path, and refreshed the dev-only `brace-expansion` lock entry to its patched version.
- Added `ui/pnpm-workspace.yaml` to explicitly allow the known `msw` install script for pnpm 11 CI installs.

## Validation

Acceptance criteria evidence:

- `.agents/knowledge/asn-providers.md` exists; root `RESEARCH-asn-providers.md` no longer exists.
- `.agents/knowledge/thread-feed-for-enterprises.md` exists; `research/enterprise-m27.md` and `research/enterprise-qwn.md` are deleted.
- `scripts/sync-from-bash-version.sh` and `scripts/sync-from-d1.sh` exist; root `sync-*.sh` files no longer exist.
- `.github/workflows/wiki-sync.yml` exists and mirrors `docs/` into `${{ github.repository }}.wiki`.
- `docs/Home.md` exists as the GitHub Wiki landing page; `docs/index.md` is removed.
- `docs/_Sidebar.md` links Home to `Home.md`.
- GitHub Pages is disabled for this repository.
- `configs/firehol/merges/cleantalk.yaml` declares `history: [1440, 10080, 43200]`.
- `configs/firehol/merges/cleantalk_1d.yaml`, `cleantalk_7d.yaml`, and `cleantalk_30d.yaml` are deleted.
- `go test ./pkg/config -run 'TestCatalogExpectedCounts|TestCatalogSourcesComplete|TestLoadDirectoryExpandsHistoryOnMerge' -v` passes and verifies the new 13 merge / 67 retention split.
- `pnpm why d3-color` reports only `d3-color@3.1.0`; the vulnerable `d3-color@2.0.0` path is gone.
- `pnpm why brace-expansion` reports `brace-expansion@5.0.6`; the dev-only audit finding for `5.0.5` is gone.
- `ui/pnpm-workspace.yaml` lists `msw` under `allowBuilds`, matching pnpm's current build-script policy model.

Tests or equivalent validation:

- `go test ./pkg/config ./pkg/engine` passed.
- `make test` passed.
- `make lint` passed.
- `python3 tools/build-firehol-static-enrichment.py --dry-run` passed and discovered 13 FireHOL-maintained merge feeds.
- `.agents/sow/audit.sh` passed.
- `git diff --check` passed.
- `pnpm lint` passed.
- `pnpm build` passed.
- `pnpm test` passed with 13 files and 39 tests.
- `pnpm test:e2e` passed with 5 Chromium browser smoke tests, including the country-map SVG path assertion.
- `pnpm audit --registry=https://registry.npmjs.org/` passed with no known vulnerabilities.
- `pnpm --dir ui install --frozen-lockfile` passed locally.
- A fresh temporary checkout with `npx --yes pnpm@11.5.0 --dir <tmp>/ui install --frozen-lockfile` passed locally and reproduced the CI pnpm major version.
- `make coverage` passed locally after the post-push coverage failure was investigated; the failed GitHub run did not reproduce locally.
- `.github/workflows/wiki-sync.yml` was syntax-checked by YAML parsing.
- A local dry-run copy of `docs/.` into a temporary wiki directory confirmed `Home.md` and `_Sidebar.md` are present at the wiki root.
- GitHub Actions run `26721664638` completed successfully and pushed wiki commit `47b1567ead3338fd6d90737382cccff349900239`.
- A fresh clone of `git@github.com:firehol/update-ipsets.wiki.git` confirmed 134 mirrored files, including `Home.md`, `_Sidebar.md`, `admin-ui/accessing-admin.md`, and `api/api-overview.md`, with no `index.md`.
- Exported the staged Git tree with `git write-tree` / `git archive` and scanned the temporary tree with Gitleaks `dir`; no leaks were found.
- A broad untracked directory Gitleaks scan was not used as the release pass/fail gate because it scanned ignored `.local/agents/feed-enrichment/*/agent.log` files; the staged release content scan passed.

Real-use evidence:

- GitHub API reports `hasWikiEnabled=true` for `firehol/update-ipsets`.
- The wiki URL is `https://github.com/firehol/update-ipsets/wiki`.

Reviewer findings:

- No external reviewer was requested for this cleanup.

Same-failure scan:

- `find . -maxdepth 1 -type f \( -name 'RESEARCH-*' -o -name 'sync-*.sh' \)` returned no root files.
- `find research -maxdepth 1 -type f` returned no files because the root `research/` directory is gone.
- `rg 'cleantalk_1d|cleantalk_7d|cleantalk_30d' tools configs/firehol/merges configs/firehol/sources pkg/config docs .agents/sow/specs .agents/skills --hidden` found only tests and docs explaining the generated retention derivatives; no authored merge YAML remains.

Sensitive data gate:

- SOW audit scanned durable artifacts and found no sensitive-data patterns.
- Staged Gitleaks scan found no leaks in the release commit content.

Artifact maintenance gate:

- AGENTS.md: no update needed; repository-wide rules were unchanged.
- Runtime project skills: updated `.agents/skills/project-operations/SKILL.md` to record that public documentation publishing mirrors `docs/` into the GitHub Wiki.
- Specs: updated `.agents/sow/specs/config.md` for merge-level history derivatives and `.agents/sow/specs/compatibility.md` for the moved migration helper path.
- End-user/operator docs: updated docs for script paths, history derivatives, merge history windows, YAML fields, and wiki navigation.
- End-user/operator skills: none affected.
- SOW lifecycle: SOW status set to `completed` and moved to `.agents/sow/done/` with the implementation commit.

Specs update:

- Updated `.agents/sow/specs/config.md`.
- Updated `.agents/sow/specs/compatibility.md`.

Project skills update:

- Updated `.agents/skills/project-operations/SKILL.md`.

End-user/operator docs update:

- Updated `docs/_Sidebar.md`, `docs/migration-from-bash.md`, `docs/configuration/configuration-concepts.md`, `docs/feeds/feed-families.md`, `docs/feeds/history-derivatives.md`, `docs/feeds/merge-feeds.md`, and `docs/feeds/yaml-field-reference.md`.

End-user/operator skills update:

- None affected.

Lessons:

- GitHub built-in Wikis and GitHub Pages are different publishing mechanisms. The repository docs source of truth remains `docs/`; the public serving mechanism for this release is the GitHub Wiki mirror.
- Merge-level `history:` is the correct catalog model when a window should apply to the composed merge output rather than each input independently.

Follow-up mapping:

- None.

## Outcome

Completed.

## Lessons Extracted

- Keep documentation source in `docs/`; mirror it into the GitHub Wiki and treat the wiki as destination-only unless the user changes the target.
- Do not model retention-window feeds as authored merges when they are semantically one-parent history derivatives.

## Followup

None yet.

## Regression - 2026-05-31

The first implementation misread "wiki served from the `docs/` directory" as GitHub Pages from `main:/docs`. The user clarified that the intended model is the existing repo-owned wiki mirror pattern used by `ai-agent.git`: `docs/` is the source of truth, the GitHub Wiki is the destination, and the wiki is replaced from `docs/` by automation.

Correction:

- Removed the duplicate `docs/index.md`.
- Updated `docs/_Sidebar.md` to link Home to `Home.md`.
- Added `.github/workflows/wiki-sync.yml` to mirror `docs/` into `${{ github.repository }}.wiki`.
- Updated `.agents/skills/project-operations/SKILL.md` to record wiki mirror publishing.
- Disabled the mistakenly configured GitHub Pages site.
- Verified the mirror after the user initialized the first wiki page; GitHub Actions run `26721664638` pushed wiki commit `47b1567ead3338fd6d90737382cccff349900239`.

### CI embedded UI regression

The post-push CI failure was real release evidence, not a transient runner
issue. GitHub Actions run `26721910576` failed `make test` and `make coverage`
in `TestRunServesSplitAdminOnSeparateListeners` because `/admin` returned an
empty body instead of the embedded SPA shell.

Evidence:

- `pkg/web/run_lifecycle_test.go:142` requires the admin HTML to contain
  `/static/assets/`.
- `pkg/web/server.go:28` read `static/index.html` from the embedded filesystem
  and silently left `embeddedIndex` empty when the file was absent.
- `.gitignore:26` and `.gitignore:27` intentionally ignore
  `pkg/web/static/assets/` and `pkg/web/static/index.html`.
- `git ls-tree -r HEAD pkg/web/static/index.html pkg/web/static/assets`
  returned no tracked generated UI bundle files in the release commit.
- `install.sh` already builds the UI and copies it into `pkg/web/static/`
  before `go build`; `Makefile` and the CI coverage job did not enforce that
  same precondition.

Correction plan:

- Add a `make ui-static` target that installs UI dependencies, builds the Vite
  app, and stages `ui/dist` into the ignored embedded static bundle location.
- Make root Go build/test/coverage/race/strict/cross targets depend on
  `ui-static` so clean CI checkouts build and test the same embedded UI shape
  that `install.sh` deploys.
- Update CI coverage to set up Node/pnpm before running `make coverage`.
- Change the Go embed contract so missing `static/index.html` is a compile-time
  failure instead of a runtime empty admin page.

Correction implemented:

- Added `make ui-static` and wired it into `make build`, `make test`,
  `make coverage`, `make race`, `make test-strict`, and `make cross`.
- Updated `.github/workflows/ci.yml` so both the build and coverage jobs prepare
  the embedded UI before root Go tests.
- Replaced the silent `embeddedIndex` init-time read with a compile-time
  `//go:embed static/index.html` binding.
- Updated runtime project skills with the new embedded UI build rule.

Validation:

- `make ui-static`: passed.
- `make test`: passed.
- `make coverage`: passed.
- `.agents/sow/audit.sh`: passed.
- `make build`: passed in the real checkout.
- `git diff --check`: passed.
- Clean archived checkout without generated `pkg/web/static/index.html` or
  `pkg/web/static/assets/`: `make ui-static` generated the bundle,
  `go test ./pkg/web -run TestRunServesSplitAdminOnSeparateListeners -count=1`
  passed, and `GOFLAGS=-buildvcs=false make build` passed. The
  `GOFLAGS=-buildvcs=false` override is archive-only because the temporary
  validation tree intentionally has no `.git` metadata.

### Code scanning release hygiene

After the CI fix, GitHub Actions was green but GitHub code scanning still
reported 7 open alerts on commit `0f3df04`:

- `.github/workflows/ci.yml:11` and `.github/workflows/ci.yml:89`:
  `actions/missing-workflow-permissions`.
- `pkg/engine/fileset_helpers.go:80` and `pkg/iprange/fileset.go:253`:
  `go/path-injection` from public compose query parameters
  `pkg/web/routes.go:77` and `pkg/web/routes.go:78`.
- `pkg/web/admin_manifest.go:305` and `pkg/web/admin_manifest.go:354`:
  `go/path-injection` from the admin manifest feed route segment
  `pkg/web/admin_manifest.go:103`.
- `agents/enrichment-public.py:644`:
  `py/incomplete-url-substring-sanitization` for a ThreatView license
  normalizer branch.

Correction plan:

- Set explicit read-only `GITHUB_TOKEN` permissions for CI jobs.
- Move CI actions to current major versions that support the upcoming GitHub
  Actions Node 24 runtime.
- Canonicalize public compose names by matching the request value against
  configured source and merge names, then passing the config-owned feed name
  into filesystem-opening paths.
- Canonicalize admin manifest route names through the loaded config source
  before enumerating or statting manifest paths.
- Replace the ThreatView license-prefix check with an exact compact-license
  match.

Correction implemented:

- Added explicit read-only workflow permissions in `.github/workflows/ci.yml`.
- Updated `actions/checkout`, `actions/setup-go`, and `actions/setup-node` to
  their current major versions.
- Split public compose validation into `pkg/engine/public_compose.go`, so
  public request strings are canonicalized before the compose path opens binary
  or text set files.
- Changed the admin manifest handler to pass `Source.Name` into manifest path
  enumeration instead of the route segment.
- Changed the ThreatView license normalizer to require an exact compact license
  match.

Validation:

- `go test ./pkg/engine ./pkg/web -count=1`: passed.
- `go test ./tools/archposture ./pkg/engine ./pkg/web -count=1`: passed.
- `make test`: passed.
- `make build`: passed.
- `make lint`: passed.
- `make staticcheck`: passed.
- `python3 -m py_compile agents/enrichment-public.py`: passed.
- `.agents/sow/audit.sh`: passed.
- `git diff --check`: passed.
- `go run github.com/zricethezav/gitleaks/v8@latest git --staged
  --no-banner --redact --log-level warn`: passed.
