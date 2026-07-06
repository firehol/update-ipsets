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
- Managed install memory defaults are `MemoryHigh=2.75G`, `MemoryMax=3G`, and
  `GOMEMLIMIT=2560MiB`; `GOMEMLIMIT` is intentionally below `MemoryHigh` because
  it is a Go runtime soft target and does not account for all memory charged to
  the service cgroup. It does not replace cgroup hard limits or bounded
  algorithms (evidence: `install.sh`,
  `.agents/sow/specs/memory-management.md`).
- Managed install ownership: install root, `bin/`, and `etc/` are
  `root:iplists`; binary and config are readable/executable to `iplists` by
  group permissions and are not world-readable. Mutable runtime directories are
  `iplists:iplists` and private to the service user (evidence: `install.sh`,
  `.agents/sow/specs/files-layout.md`).
- Daemon-created mutable runtime/publication artifacts are owner-private:
  managed installs use `0600` for non-executable files, `0700` for directories,
  and `UMask=0077`; normal reinstall ensures the bounded mutable directory
  roots exist with the right owner/group/mode and does not recursively rewrite
  existing runtime trees. Explicit repair is available with
  `./install.sh --repair-runtime-permissions` and must change only paths with
  owner/group/mode drift (evidence: `install.sh`,
  `.agents/sow/specs/files-layout.md`).
- Install command wrappers must preserve the real failing command status. Do
  not use `if ! "$@"; then exit_code=$?` because `$?` becomes the negated
  status inside the block. Use a positive `if "$@"; then return 0; else ...`
  form or another pattern that records the command status before negation.
- Explicit runtime permission repair must stop the daemon when it is active.
  Normal live installs must not scan or rewrite daemon-owned runtime trees.
- Managed installs may compact generated `data/` and `web/` Git object stores
  only during explicit mutable runtime repair, after ownership repair and only
  when the service is stopped or not running. This is private Git maintenance
  for generated publication trees; it must not rewrite feed files or public
  artifacts.

## Important environment

- Admin auth: `UPDATE_IPSETS_ADMIN_USER`, `UPDATE_IPSETS_ADMIN_PASSWORD` (evidence: `README.md`, `pkg/web/middleware.go`).
- GeoLite2 license: `MAXMIND_LICENSE_KEY` (evidence: `configs/firehol/sources/geolocation/geolite2_country.yaml`, `configs/firehol/sources/asn/maxmind_geolite2_asn.yaml`).
- DroneBL rsync secret: `DRONEBL_RSYNC_PASSWORD` or `RSYNC_PASSWORD` (evidence: `tools/dronebl2ipsets/fetch.go`, `tools/dronebl2ipsets/README.md`).
- OpenTelemetry metric export can be enabled with `UPDATE_IPSETS_OTEL=1` and
  local Netdata OTLP/gRPC endpoint `http://127.0.0.1:4317` (evidence:
  `install.sh`, `README.md`, `internal/observability/observability.go`).

## Smoke checks

- Public health: `curl http://localhost:18888/healthz`.
- In split-listener mode, `/healthz` must answer on both the public and admin
  listeners; the systemd watchdog treats both listeners as web-serving proof.
- Public status: `curl http://localhost:18888/api/v1/status`.
- Public sets: `curl http://localhost:18888/api/v1/sets`.
- Admin status/integrity require configured auth unless admin auth is explicitly disabled for local development.

## Operational rules

- Public serving must stay cache-first and cheap; do not trigger upstream downloads or broad recomputation from public requests (evidence: `.agents/sow/specs/operating-principles.md`).
- Watchdog, `/healthz`, and request-path telemetry are part of web-serving
  availability. They must not wait for OTel export, lazy metric creation,
  logging sinks, ingestion, integrity, or artifact work.
- Web-serving telemetry is still required, but metrics must be exact local
  atomic state. Export slowness must not change local metric values.
- OpenTelemetry startup is fail-open. Bad OTLP configuration, an unreachable
  collector, resource detector errors, or a telemetry setup timeout MUST log a
  warning and disable/degrade OTLP export instead of preventing daemon startup or
  web serving.
- Production metric recording uses project-owned `observability.Try*` helpers
  application-wide. Metrics are not exporter-owned samples and must not be
  dropped for exporter backpressure.
- Local admin/engine timing books used by web handlers must keep critical
  sections tiny and must not force broad snapshots on request paths.
- Admin status diagnostic reads of local telemetry books must be best-effort
  too. Busy current-run, lifetime, or scheduler telemetry sections should be
  omitted/degraded, not waited on.
- Public/admin HTTP middleware logs are request-path telemetry. They must use
  bounded serving-safe local logging, not a logger that can wait for export,
  disk, or a remote sink on the request path.
- Logs and traces are bounded local queues. Full buffers drop records before
  delaying engine, scheduler, admin, watchdog, or shutdown work, and the exact
  local counters `telemetry.logs.dropped` and `telemetry.traces.dropped` expose
  those losses.
- Local log/trace buffers are configured with
  `UPDATE_IPSETS_TELEMETRY_BUFFER_BYTES`, `UPDATE_IPSETS_LOG_BUFFER_BYTES`, and
  `UPDATE_IPSETS_TRACE_BUFFER_BYTES`. Values accept bytes or binary
  `KB`/`KiB`, `MB`/`MiB`, and `GB`/`GiB` suffixes.
- Daemon lifecycle control logs on the web-serving path, including pre-listen
  cleanup, startup integrity recovery, startup entity-artifact checks, ready,
  stopping, watchdog, daemon-control panic recovery, and delayed startup cleanup
  control logs, must also use serving-safe local logging instead of the
  export-backed or sink-blocked application logger.
- The admin-surface `/metrics` endpoint is also telemetry on the web-serving
  surface. It renders local metric snapshots and MUST return
  `503 Service Unavailable` for concurrent or timed out scrapes instead of
  stacking blocked scrape work.
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
