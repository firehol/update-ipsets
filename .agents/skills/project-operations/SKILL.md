---
name: project-operations
description: "Install, daemon, admin, and runtime operation guidance for update-ipsets. MUST be followed for operational changes."
---

## Install and service facts

- Authoritative local install path: `./install.sh` (evidence: `install.sh`).
- `make ui-static` refreshes the ignored embedded SPA bundle under
  `pkg/web/static/`; `make build` depends on it so release binaries do not
  silently embed an empty admin UI (evidence: `Makefile`, `pkg/web/server.go`).
- Installed binary: `/opt/update-ipsets/bin/update-ipsets` (evidence: `install.sh` systemd unit).
- Default local service name: `update-ipsets` (evidence: `install.sh`).
- Default local daemon listen in install flow: `:18888` (evidence: `install.sh`, `README.md`, `ui/vite.config.ts`).
- Runtime catalog install path: `/opt/update-ipsets/etc/config/` (evidence: `install.sh`, `README.md`).
- Managed install memory defaults are `MemoryHigh=1536M`, `MemoryMax=2G`, and
  `GOMEMLIMIT=1536MiB`; `GOMEMLIMIT` is a Go runtime soft target and does not
  replace cgroup hard limits or bounded algorithms (evidence: `install.sh`,
  `.agents/sow/specs/memory-management.md`).
- Managed install ownership: install root, `bin/`, and `etc/` are
  `root:iplists`; binary and config are readable/executable to `iplists` by
  group permissions and are not world-readable. Mutable runtime directories are
  `iplists:iplists` and private to the service user (evidence: `install.sh`,
  `.agents/sow/specs/files-layout.md`).
- Daemon-created mutable runtime/publication artifacts are owner-private:
  managed installs use `0600` for non-executable files, `0700` for directories,
  and `UMask=0077`; reinstall repairs existing mutable runtime trees to those
  modes (evidence: `install.sh`,
  `.agents/sow/specs/files-layout.md`).
- Install command wrappers must preserve the real failing command status. Do
  not use `if ! "$@"; then exit_code=$?` because `$?` becomes the negated
  status inside the block. Use a positive `if "$@"; then return 0; else ...`
  form or another pattern that records the command status before negation.
- Reinstall permission repair can race with a running daemon's mutable temp
  files. For managed live installs, use race-aware traversal such as GNU
  `find ... -ignore_readdir_race` on daemon-owned runtime/publication trees, or
  stop the daemon first if strict traversal is required.
- Managed installs may compact generated `data/` and `web/` Git object stores
  during mutable runtime repair, after ownership repair and only when the
  service is stopped or not running. This is private Git maintenance for
  generated publication trees; it must not rewrite feed files or public
  artifacts.

## Important environment

- Admin auth: `UPDATE_IPSETS_ADMIN_USER`, `UPDATE_IPSETS_ADMIN_PASSWORD` (evidence: `README.md`, `pkg/web/middleware.go`).
- GeoLite2 license: `MAXMIND_LICENSE_KEY` (evidence: `configs/firehol/sources/geolocation/geolite2_country.yaml`, `configs/firehol/sources/asn/maxmind_geolite2_asn.yaml`).
- DroneBL rsync secret: `DRONEBL_RSYNC_PASSWORD` or `RSYNC_PASSWORD` (evidence: `tools/dronebl2ipsets/fetch.go`, `tools/dronebl2ipsets/README.md`).
- OpenTelemetry can be enabled with `UPDATE_IPSETS_OTEL=1` and local Netdata OTLP/gRPC endpoint `http://127.0.0.1:4317` (evidence: `install.sh`, `README.md`, `internal/observability/observability.go`).

## Smoke checks

- Public health: `curl http://localhost:18888/healthz`.
- Public status: `curl http://localhost:18888/api/v1/status`.
- Public sets: `curl http://localhost:18888/api/v1/sets`.
- Admin status/integrity require configured auth unless admin auth is explicitly disabled for local development.

## Operational rules

- Public serving must stay cache-first and cheap; do not trigger upstream downloads or broad recomputation from public requests (evidence: `.agents/sow/specs/operating-principles.md`).
- Engine-lane work must be visible through admin status/UI. `max_engine_lane_workers`
  controls top-level processing/integrity/entity admission; `max_background_workers`
  controls bounded fan-out inside admitted background/entity work (from
  SOW-0117).
- Background concurrency must protect CPU and memory; default engine-lane and
  background workers should remain conservative.
- Do not use broad process-kill commands. Track and stop only specific PIDs started for the task.
- Do not push or touch production systems unless the user explicitly approves.

## Release hygiene

- Before public upstream release, run secret scans, hardcoded-path scans, generated artifact checks, license/notice review, and flattened-history audit (from SOW-0021).
- Keep docs/source of truth in the repo. Public operator documentation is mirrored from `docs/` into the GitHub Wiki by `.github/workflows/wiki-sync.yml`; do not edit the wiki directly.
