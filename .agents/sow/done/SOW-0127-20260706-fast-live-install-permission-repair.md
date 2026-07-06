# SOW-0127 - Fast Live Install Permission Repair

## Status

Status: completed

Sub-state: completed and ready for SOW move

## Requirements

### Purpose

Keep the public/admin web server available during managed installs, while preserving the managed filesystem ownership and private runtime-file permission contract.

### User Request

The user reported that `install.sh` stops the service for 30-60 seconds before starting it again, leaving the web server down. The user approved keeping the old daemon running while files are updated, with the caveat that runtime permission repair should not blindly run slow recursive `chown`/`chmod` work. The requested design is to find only files/directories whose owner/group/mode is wrong and change only those, using standard tools if practical or a small C helper if standard tools are insufficient.

### Assistant Understanding

Facts:

- The daemon does not watch installed files automatically; YAML catalog changes require `SIGHUP` or restart, and binary/unit/template changes require restart.
- The current installer stops the daemon before mutable runtime repair when restart is enabled.
- The current installer recursively touches mutable runtime trees with `chown -R` and broad `find ... chmod ...` operations.
- The current `find ... -exec ... {} +` form is batched, not one child process per file.
- The installed systemd unit sets `UMask=0077`, so daemon-created directories/files should already satisfy the managed private permission contract.

Inferences:

- Normal reinstall can safely update the binary, catalog, templates, and unit while the old daemon continues to serve because it will not pick up partial file changes.
- Full mutable-tree repair should be an explicit maintenance operation, not default deploy behavior.
- GNU `find` predicates can identify ownership and mode drift before invoking `chown`/`chmod`, so a C helper is not needed unless validation shows standard tools are still too slow or not expressive enough.

Unknowns:

- None blocking. Production tree size and exact drift rate are runtime facts, but the desired contract can be implemented without knowing them.

### Acceptance Criteria

- Default `./install.sh` does not stop the running service before build/copy/install work.
- Default `./install.sh` does not recursively `chown`/`chmod` mutable runtime trees.
- Default `./install.sh` still ensures the install root, `bin/`, `etc/`, and the bounded mutable directory set exist with correct owner/group/mode.
- An explicit repair mode exists for mutable runtime permission drift.
- Explicit repair mode scans mutable runtime trees and invokes `chown`/`chmod` only for paths whose owner/group/mode is wrong.
- Specs and operator docs describe the new normal-install versus explicit-repair behavior.
- Validation covers shell syntax, help behavior, and the selective `find` predicates on a temporary fixture.

## Analysis

Sources checked:

- `install.sh`
- `.agents/sow/specs/files-layout.md`
- `docs/installation/filesystem-layout.md`
- `docs/installation/installation.md`
- `.agents/skills/project-operations/SKILL.md`

Current state:

- `install.sh` stops the active service before mutable repair when restart is enabled.
- `install.sh` runs `chown -R iplists:iplists` over `data/`, `cache/`, `lib/`, `web/`, `run/`, and `tmp/`.
- `install.sh` runs broad directory and file mode repair over the same trees.
- The filesystem spec currently requires install and packaging flows to repair existing mutable runtime directories/files during reinstall.

Risks:

- Removing default recursive repair means existing permission drift is no longer silently fixed on every install.
- Running a repair scan while the daemon is active can race with live temp files.
- Changing restart timing affects deployment expectations and must keep the final restart path clear for binary/unit/template changes.

## Pre-Implementation Gate

Status: ready

Problem / root-cause model:

- The outage window exists because `install.sh` stops `update-ipsets` before bounded and unbounded install work, including mutable runtime repair.
- The slowest default work is unbounded with production state size: recursively changing ownership/modes under runtime trees containing years of generated/feed history.
- The runtime permission contract is better enforced at creation time by systemd `UMask=0077` and bounded directory creation, with explicit repair as a maintenance operation.

Evidence reviewed:

- `install.sh` active-service stop before repair and install work.
- `install.sh` recursive mutable runtime ownership/mode repair.
- `.agents/sow/specs/files-layout.md` managed ownership contract and existing reinstall repair requirement.
- `docs/installation/filesystem-layout.md` operator-facing filesystem behavior.
- `.agents/skills/project-operations/SKILL.md` install and service facts.

Affected contracts and surfaces:

- `./install.sh` CLI and behavior.
- Managed systemd install downtime behavior.
- Managed filesystem ownership and mode contract.
- Operator installation documentation.
- Project filesystem layout spec.
- Project operations skill.

Existing patterns to reuse:

- `install.sh` `run()` command wrapper and visible command output.
- Managed install root `/opt/update-ipsets`.
- Existing `iplists` service identity and `UMask=0077`.
- Existing spec/docs split for durable contracts versus operator instructions.

Risk and blast radius:

- Operational risk: a badly drifted runtime tree may require explicit repair before the daemon can write some paths.
- Availability risk: if final restart fails, old service may continue only until restart starts; this SOW does not implement socket activation or dual-instance zero-downtime handoff.
- Security risk: skipping broad default chmod must not create world-readable new runtime directories; bounded directory creation must use explicit `0700`.
- Data risk: no history/retention data is deleted, rewritten, compacted, migrated, or regenerated by this work.

Sensitive data handling plan:

- No production logs, credentials, private endpoints, customer data, personal data, or raw feed history are written to durable artifacts.
- SOW, specs, docs, and code comments will describe behavior using paths and command names only.

Implementation plan:

1. Add an explicit installer flag for mutable runtime repair and keep normal install live until the final restart.
2. Replace default recursive mutable-tree ownership/mode repair with bounded `install -d`/top-level ownership setup.
3. Implement selective repair using `find` predicates that call `chown`/`chmod` only for wrong paths.
4. Update specs, docs, and the operations skill to preserve the new contract.
5. Validate syntax/help/selective predicates.

Validation plan:

- `bash -n install.sh`
- `./install.sh --help`
- Temporary-fixture shell validation of selective `find` predicates.
- `git diff --check`

Artifact impact plan:

- AGENTS.md: no project-wide workflow rule change expected.
- Runtime project skills: update `.agents/skills/project-operations/SKILL.md`.
- Specs: update `.agents/sow/specs/files-layout.md`.
- End-user/operator docs: update installation/filesystem docs.
- End-user/operator skills: none expected.
- SOW lifecycle: current SOW completed and moved with implementation when accepted.

Open-source reference evidence:

- None. This is a project-local installer contract and standard GNU tool usage question.

Open decisions:

- Resolved by user: long-term-best install behavior is preferred over preserving automatic full repair on every reinstall.
- Resolved by user: find-and-change-only-drift is preferred; a C helper is only justified if standard tools are insufficient.

## Implications And Decisions

1. Default repair behavior:
   - Option A: Keep full recursive repair every install. Benefit: silent drift repair. Risk: production downtime and unbounded metadata writes.
   - Option B: Explicit repair mode only. Benefit: fast live install. Risk: drift requires operator action.
   - Selected: B, because availability during normal deploys is the purpose of this SOW.

2. Drift detection implementation:
   - Option A: GNU `find` predicates plus batched `chown`/`chmod`. Benefit: simple, available, visible, no compiled helper.
   - Option B: C helper. Benefit: potentially faster single-process traversal. Risk: more code, packaging, platform complexity.
   - Selected: A first; revisit B only if measured standard-tool repair is insufficient.

## Plan

1. Update `install.sh` CLI and default flow.
2. Add explicit selective mutable runtime repair path.
3. Update specs, docs, and project operations skill.
4. Validate and record outcomes.

## Execution Log

### 2026-07-06

- Created SOW after user approved the design direction and clarified the permission-repair caveat.
- Updated `install.sh` so normal installs do not stop the daemon until the final restart and do not recursively rewrite mutable runtime trees.
- Added explicit `--repair-runtime-permissions` mode that stops an active service, removes stale publish stages, scans mutable trees, and changes only paths with owner/group/mode drift.
- Updated filesystem spec, operator docs, and project operations skill.

## Validation

Acceptance criteria evidence:

- Default installer path no longer calls `systemctl stop update-ipsets` before normal install work; stopping is only reachable from `--repair-runtime-permissions`.
- Default installer path no longer runs recursive `chown -R iplists:iplists` or broad runtime-tree `chmod`.
- Bounded mutable directory roots are created with `install -d -o iplists -g iplists -m 0700`.
- Explicit repair mode uses `find` predicates for owner/group/mode drift before invoking `chown`/`chmod`.
- `.agents/sow/specs/files-layout.md`, `docs/installation/installation.md`, `docs/installation/filesystem-layout.md`, `docs/updating/updating-binary.md`, `docs/updating/updating-config.md`, and `.agents/skills/project-operations/SKILL.md` were updated.

Tests or equivalent validation:

- `bash -n install.sh` passed.
- `./install.sh --help` passed and shows `--repair-runtime-permissions`.
- `shellcheck install.sh` passed.
- `git diff --check` passed.
- Temporary fixture under `/tmp` validated that the selective `find` predicates identify only wrong-owner, wrong-mode directory, and wrong-mode file paths, then leave no drift after repair.
- Local `./install.sh` completed. The install output showed normal install skipped mutable runtime repair and proceeded to the final restart.
- `curl -fsS http://127.0.0.1:18888/healthz` returned `ok`.
- `systemctl is-active update-ipsets` returned `active`.

Real-use evidence:

- Full local `./install.sh` was run during validation and restarted the local service. The service came back healthy.

Sensitive data gate:

- Durable artifacts contain no raw secrets, credentials, bearer tokens, SNMP communities, community member names, customer names, personal data, non-private customer-identifying IPs, private endpoints, or proprietary incident details.

## Outcome

Completed. Normal managed installs now keep the daemon running during build/copy/install work and restart only at the end. Mutable runtime tree permission repair is explicit through `./install.sh --repair-runtime-permissions` and changes only paths with owner/group/mode drift.

## Lessons Extracted

Normal deployment and maintenance repair are different operational jobs. Keeping them in one default path created unnecessary downtime and unbounded metadata writes.

Artifact maintenance gate:

- AGENTS.md: no update needed; project-wide workflow did not change.
- Runtime project skills: updated `.agents/skills/project-operations/SKILL.md`.
- Specs: updated `.agents/sow/specs/files-layout.md`.
- End-user/operator docs: updated `docs/installation/installation.md`, `docs/installation/filesystem-layout.md`, `docs/updating/updating-binary.md`, and `docs/updating/updating-config.md`.
- End-user/operator skills: no exported operator skill affected.
- SOW lifecycle: this SOW is completed and will be moved to `.agents/sow/done/` with the implementation commit.

## Followup

None yet.

## Regression Log

None yet.
