# SOW-0100 - Generated Artifact Group-Readable Permissions

## Status

Status: completed

Sub-state: implemented, validated, installed, and ready for closure commit.

## Requirements

### Purpose

Make generated update-ipsets runtime and publication artifacts locally inspectable
by trusted members of the `iplists` group while preserving non-world-readable
install hygiene and daemon-owned writes.

### User Request

The user asked to fix generated permissions so all generated directories and
files are group-readable, and to make `install.sh` repair permissions on
existing generated directories and files.

### Assistant Understanding

Facts:

- The installed daemon runs as `User=iplists` and `Group=iplists`.
- The installed mutable trees are owned by `iplists:iplists`.
- Current generated files under `/opt/update-ipsets/web` are mode `0600`.
- Current generated directories are commonly mode `0700`.
- The user added their account to the `iplists` group so generated markdown can
  be inspected locally.
- Group membership alone does not permit reads while generated files are
  owner-only.

Inferences:

- The desired contract is `0640` for generated non-executable files and `0750`
  for generated directories.
- Group-write is not required and should not be introduced.
- World-readable access remains undesired for managed install/runtime trees.

Unknowns:

- No unresolved user decision blocks implementation.

### Acceptance Criteria

- Generated directories created by the daemon use group-readable/searchable
  permissions.
- Generated non-executable files created by the daemon use group-readable
  permissions.
- `install.sh` repairs existing generated mutable trees to group-readable file
  and directory modes.
- Existing focused tests are updated so the permission contract is enforced.
- Specs and operator docs no longer describe daemon-generated files as
  owner-private by default.

## Analysis

Sources checked:

- `pkg/markdown/generate.go`
- `pkg/engine/metadata.go`
- `internal/fileutil/fileutil.go`
- `pkg/engine/web_ipsets.go`
- `install.sh`
- `.agents/sow/specs/files-layout.md`
- `docs/installation/filesystem-layout.md`

Current state:

- Markdown artifacts are written with `os.WriteFile(..., 0o600)` and their
  parent directories with `os.MkdirAll(..., 0o700)`.
- Public index artifacts are written through atomic helpers with mode `0o600`.
- Raw published set mirrors are written through a temporary file opened with
  mode `0o600`.
- The installer sets mutable runtime directories to `0750` but does not repair
  generated files below them to group-readable modes.

Risks:

- Changing generated runtime modes broadens local read access to every member of
  the `iplists` group.
- Existing tests assert owner-private modes and must be updated deliberately so
  future regressions are caught.
- A one-time chmod outside the app would be overwritten by future generation
  runs; code paths and install repair both need changes.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The permission failure is caused by explicit file and directory modes in the
  application, not by systemd `UMask`.
- Generated artifacts are created owner-private. Members of `iplists` can enter
  top-level directories after account membership is updated, but cannot read
  files generated as `0600` or traverse subdirectories generated as `0700`.

Evidence reviewed:

- `pkg/markdown/generate.go` writes markdown files as `0600` and directories as
  `0700`.
- `pkg/engine/metadata.go` passes `0600` to the atomic writer for index files.
- `internal/fileutil/fileutil.go` creates parent directories as `0700` before
  atomic writes.
- `pkg/engine/web_ipsets.go` writes raw public mirror temp files as `0600`.
- `install.sh` currently fixes mutable runtime directories to `0750` but does
  not chmod existing generated files to `0640`.

Affected contracts and surfaces:

- Runtime file layout and local operator access.
- Installer repair behavior.
- Generated markdown, public JSON/CSV/XML/TXT artifacts, raw set mirrors,
  runtime ledgers, cache files, and staged artifacts.
- Specs and operator docs describing filesystem permissions.

Existing patterns to reuse:

- Existing explicit mode arguments instead of relying on process umask.
- Existing installer ownership/permission repair block.
- Existing focused tests that assert generated-file modes.

Risk and blast radius:

- Local access broadens only to the `iplists` group; no world-readable change is
  planned.
- Group members can read generated runtime/publication data, including
  non-redistributable feed artifacts. This is acceptable under the user's
  explicit local inspection requirement and the trusted-group model.
- No public HTTP behavior, upstream download, feed composition, or generated
  artifact content changes are intended.

Sensitive data handling plan:

- No raw secrets, tokens, credentials, customer data, private endpoints, or
  proprietary incident details are needed in durable artifacts.
- Evidence records only paths, modes, and generic permission behavior.

Implementation plan:

1. Introduce or use shared permission constants for generated files and
   directories where production writers create runtime/publication artifacts.
2. Patch markdown, atomic writer parent directories, raw mirror writer, and
   other production runtime writers so generated directories use `0750` and
   generated non-executable files use `0640`.
3. Patch `install.sh` to repair existing generated files to `0640` and
   generated directories to `0750` under mutable runtime trees.
4. Update specs/operator docs and focused tests.

Validation plan:

- Run focused Go tests for file utility, markdown generation, engine file
  contract, and copy-file mode assertions.
- Search for remaining production `0o600` / `0o700` writer modes and classify
  any intentional non-generated exceptions.
- Run a syntax check for `install.sh`.

Artifact impact plan:

- AGENTS.md: no update expected; project-wide workflow rules are unchanged.
- Runtime project skills: update only if validation finds a durable agent rule
  gap.
- Specs: update `.agents/sow/specs/files-layout.md`.
- End-user/operator docs: update installation filesystem layout docs.
- End-user/operator skills: no external operator skill is affected.
- SOW lifecycle: this SOW records the redirected priority and remains current
  until implemented and validated.

Open-source reference evidence:

- None checked; this is a project-local permission contract rather than an
  external protocol or library behavior.

Open decisions:

- Resolved by user: all generated directories and files must be group-readable,
  and `install.sh` must repair existing generated directories and files.

## Implications And Decisions

1. Generated permission contract

Selected: generated non-executable files use `0640`; generated directories use
`0750`.

Reasoning: this gives read/search access to trusted `iplists` group members,
keeps generated files non-world-readable, and avoids adding group-write.

2. Install repair

Selected: `install.sh` repairs existing mutable runtime trees after ownership is
set.

Reasoning: generated artifacts already present on disk would otherwise stay
owner-private until each file is regenerated.

## Plan

1. Patch shared writer and direct writer modes.
2. Patch installer repair commands.
3. Update specs/docs/tests.
4. Validate with focused tests and same-failure search.

## Execution Log

### 2026-06-02

- Recorded user decision and implementation gate.
- Added shared generated permission constants: `0750` for generated
  directories and `0640` for generated non-executable files.
- Updated runtime writers for generated markdown, engine artifacts, downloader
  and processor temp files, cache files, scheduler state, output sync files,
  lock files, and the nested DroneBL source helper.
- Updated `os.MkdirTemp` staging directory creators for web/entity publish
  batches and DroneBL artifact extraction outputs so temporary generated
  directories are also `0750`.
- Updated `install.sh` to set `UMask=0027` in the generated systemd unit and to
  chmod existing mutable runtime files to `0640` during install repair.
- Updated the filesystem spec, operator installation docs, systemd docs, and
  runtime operations skill to state the group-readable generated artifact
  contract.
- Repaired the current installed mutable trees under `/opt/update-ipsets` with
  `0750` directory modes and `0640` file modes.
- Installed the patched binary and generated systemd unit with `./install.sh`,
  then restarted `update-ipsets`.

## Validation

Acceptance criteria evidence:

- Generated directory/file writers now use the generated-mode contract in
  `internal/fileutil`, `pkg/engine`, `pkg/markdown`, `pkg/processor`,
  `pkg/downloader`, `pkg/cache`, `pkg/scheduler`, `pkg/output`, and
  `tools/dronebl2ipsets`.
- `install.sh` now repairs existing generated files under `data/`, `cache/`,
  `lib/`, `web/`, `run/`, and `tmp/` to `0640`, in addition to the existing
  `0750` directory repair.
- `install.sh` now writes `UMask=0027` into the generated systemd unit.
- The installed tree was repaired and verified:
  `/opt/update-ipsets/web` is `0750`, `/opt/update-ipsets/web/index.json` is
  `0640`, and `/opt/update-ipsets/web/apnic_ssh_bruteforce.md` is `0640`.
- After service restart, a delayed tree-wide check returned no generated files
  missing group-read and no generated directories missing group read/search.
- `systemctl cat update-ipsets` shows `User=iplists`, `Group=iplists`, and
  `UMask=0027`; `systemctl is-active update-ipsets` returned `active`.

Tests or equivalent validation:

- `go test ./... && (cd tools/dronebl2ipsets && go test ./...)` passed.
- `go test ./internal/fileutil ./pkg/markdown ./pkg/processor ./pkg/cache ./pkg/scheduler ./pkg/downloader ./pkg/output ./pkg/engine`
  passed.
- `bash -n install.sh` passed.
- `git diff --check` passed.

Real-use evidence:

- `sg iplists -c 'test -r /opt/update-ipsets/web/apnic_ssh_bruteforce.md && test -r /opt/update-ipsets/web/index.json && echo group-readable'`
  returned `group-readable`.
- `sudo find /opt/update-ipsets/data /opt/update-ipsets/cache /opt/update-ipsets/lib /opt/update-ipsets/web /opt/update-ipsets/run /opt/update-ipsets/tmp \( -type f ! -perm -0040 -o -type d ! -perm -0050 \) -printf '%m %u %g %p\n' | head -20`
  returned no paths.

Reviewer findings:

- No external reviewers were run; this was a focused operational/code fix and
  the user did not ask for second-opinion agents.

Same-failure scan:

- `rg -n '0o600|0o700' internal pkg cmd tools --glob '*.go' --glob '!**/*_test.go'`
  returned no production Go matches.
- `rg -n 'want 0600|want 0700|owner-private|owner-only|private on disk|private modes' internal pkg tools docs .agents/skills/project-operations/SKILL.md .agents/sow/specs --glob '!node_modules/**'`
  returned no stale contract matches in updated contract surfaces.

Sensitive data gate:

- Durable artifacts record only paths, modes, commands, and generic permission
  behavior. No raw secrets, credentials, tokens, private endpoints, personal
  data, or proprietary incident details were added.

Artifact maintenance gate:

- AGENTS.md: no update needed; project-wide workflow rules did not change.
- Runtime project skills: updated
  `.agents/skills/project-operations/SKILL.md`.
- Specs: updated `.agents/sow/specs/files-layout.md`.
- End-user/operator docs: updated `docs/installation/filesystem-layout.md`,
  `docs/installation/installation.md`, and
  `docs/installation/systemd-setup.md`.
- End-user/operator skills: none affected beyond the runtime project operations
  skill.
- SOW lifecycle: this SOW is marked `Status: completed` and moved to
  `.agents/sow/done/` in the closure commit with the implementation.

Specs update:

- `.agents/sow/specs/files-layout.md` now states generated directories are
  `0750`, generated non-executable files are `0640`, managed systemd installs
  use `UMask=0027`, and install/packaging flows repair existing generated
  mutable trees.

Project skills update:

- `.agents/skills/project-operations/SKILL.md` now records the group-readable
  generated artifact contract.

End-user/operator docs update:

- Installation filesystem and systemd docs now explain `UMask=0027`, `0750`
  generated directories, and `0640` generated non-executable files.

End-user/operator skills update:

- None beyond `.agents/skills/project-operations/SKILL.md`.

Lessons:

- Existing generated artifact permissions must be repaired during install;
  runtime writer changes alone leave already-published files with old modes.

Follow-up mapping:

- No follow-up SOW is needed for this permission issue.

## Outcome

Completed. Generated runtime and publication artifacts are group-readable for
the trusted `iplists` group, not world-readable, and the installed service has
been updated and restarted with the new permission contract.

## Lessons Extracted

- Runtime writer changes are not enough when artifacts already exist on disk;
  installer repair must cover existing generated files as well as directories.
- `os.MkdirTemp` creates temporary directories with private modes by default,
  so staging directory creators need explicit chmod handling too.

## Followup

None yet.

## Regression Log

None yet.
