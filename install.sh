#!/bin/bash
# install.sh — Build and install update-ipsets to /opt/update-ipsets
#
# This script is the single entry point for a full deploy:
#   1. Build the React UI in ui/
#   2. Copy the fresh bundle into pkg/web/static/ (cleaning old assets)
#   3. Build the Go binary (with the UI embedded via go:embed)
#   4. Lay out the /opt/update-ipsets directory tree
#   5. Install the binary, config, description pages, systemd unit
#   6. systemctl daemon-reload
#   7. systemctl restart update-ipsets (unless --no-restart)
#
# Self-contained installation — all data, config, and state under one directory:
#   /opt/update-ipsets/
#   ├── bin/update-ipsets         # binary (UI embedded)
#   ├── etc/config/               # deployed source catalog directory (synced from repo on install)
#   ├── data/                     # ipset output files (.ipset, .netset, .source)
#   │   ├── history/              # history snapshots
#   │   └── errors/               # download error logs
#   ├── cache/                    # scheduler/runtime cache files (for example scheduler-state.json)
#   ├── lib/                      # binary sets, retention data, provider/artifact state
#   ├── web/                      # generated web files (JSON, CSV, sitemap)
#   │   └── files/                # downloadable ipset files
#   ├── run/                      # lock file
#   └── tmp/                      # temp download files
#
# Feed-state cache continuity remains in data/.cache.json for bash migration
# compatibility; cache/ is still used for scheduler/runtime state.
#
# The systemd service sets environment variables that override the YAML
# config's ${VAR-default} path templates, so the config file doesn't
# need editing — it works for both development (uses $HOME) and
# production (uses /opt/update-ipsets).
#
# Usage:
#   ./install.sh                          # full install to /opt/update-ipsets, restart service
#   ./install.sh --no-restart             # install but do not restart
#   ./install.sh /opt/custom              # install to custom directory
#   ./install.sh /opt/custom --no-restart
#

set -euo pipefail

RED='\033[0;31m' GREEN='\033[0;32m' YELLOW='\033[1;33m' GRAY='\033[0;90m' NC='\033[0m'
run() {
  printf >&2 "${GRAY}$(pwd) >${NC} ${YELLOW}"; printf >&2 "%q " "$@"; printf >&2 "${NC}\n"
  if ! "$@"; then
    local exit_code=$?
    echo -e >&2 "${RED}[ERROR]${NC} Exit code ${exit_code}: ${YELLOW}$*${NC} (in $(pwd))"
    return $exit_code
  fi
}

# Parse arguments: positional = install dir, --no-restart anywhere = skip restart.
INSTALL_DIR="/opt/update-ipsets"
RESTART=1
for arg in "$@"; do
  case "$arg" in
    --no-restart)
      RESTART=0
      ;;
    -h|--help)
      sed -n '2,34p' "$0"
      exit 0
      ;;
    -*)
      echo -e "${RED}[ERROR]${NC} Unknown flag: $arg" >&2
      exit 2
      ;;
    *)
      INSTALL_DIR="$arg"
      ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# ----------------------------------------------------------------------------
# Step 1: Build the React UI
# ----------------------------------------------------------------------------
# The Go binary embeds pkg/web/static/ at compile time via //go:embed.
# That means the UI bundle MUST land in pkg/web/static/ BEFORE `go build`.
# If the UI source is missing (e.g. someone wiped ui/ by accident) we bail
# out rather than silently shipping a stale or empty bundle.

if [ ! -d ui ]; then
  echo -e "${RED}[ERROR]${NC} ui/ directory is missing. The React source must live in $SCRIPT_DIR/ui." >&2
  exit 1
fi
if [ ! -f ui/package.json ]; then
  echo -e "${RED}[ERROR]${NC} ui/package.json is missing. ui/ does not look like a Vite project." >&2
  exit 1
fi
if ! command -v pnpm >/dev/null 2>&1; then
  echo -e "${RED}[ERROR]${NC} pnpm is not installed but is required to build the UI." >&2
  echo "  Install with: npm install -g pnpm   (or your package manager of choice)" >&2
  exit 1
fi

echo -e "${GREEN}[1/7] Installing UI dependencies (ui/)…${NC}"
# `pnpm install --frozen-lockfile` is the CI-safe mode: fails if the lockfile
# is out of date instead of silently mutating it. For a local dev loop where
# package.json was just edited, re-run install.sh after updating the lockfile
# with `pnpm install` in ui/.
run pnpm --dir ui install --frozen-lockfile

echo -e "${GREEN}[2/7] Building UI bundle (ui/ → ui/dist/)…${NC}"
run pnpm --dir ui build

# ----------------------------------------------------------------------------
# Step 2: Refresh pkg/web/static/ with the fresh bundle
# ----------------------------------------------------------------------------
# Vite emits hashed filenames (index-<HASH>.js / .css). If we don't wipe the
# previous bundle first, the old hashed files accumulate in pkg/web/static/
# and get embedded into the Go binary as dead weight. So: clean, then copy.
# We only touch files vite actually emits; favicon.svg, fonts/, icons.svg,
# and methodology/ are source assets and must stay untouched.

echo -e "${GREEN}[3/7] Refreshing embedded static bundle (pkg/web/static/)…${NC}"
run rm -rf pkg/web/static/assets
run mkdir -p pkg/web/static/assets
run cp -r ui/dist/assets/. pkg/web/static/assets/
run cp ui/dist/index.html pkg/web/static/index.html
# Vite also emits top-level static assets referenced from index.html (e.g.
# favicon.svg copied from ui/public/). We intentionally do NOT overwrite
# pkg/web/static/favicon.svg / icons.svg / fonts/ / methodology/ because
# those are the canonical copies used by the backend — the vite build
# should not be trusted to produce them identically.

# ----------------------------------------------------------------------------
# Step 3: Build the Go binary
# ----------------------------------------------------------------------------
# With the fresh bundle in place, `go build` picks it up via //go:embed and
# bakes it into the binary. Version string is derived from `git describe`
# so admin/status can report the build you actually deployed.

echo -e "${GREEN}[4/7] Building update-ipsets binary…${NC}"
run go build \
  -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
  -o update-ipsets \
  ./cmd/update-ipsets

# ----------------------------------------------------------------------------
# Step 4: Lay out /opt/update-ipsets and install artifacts
# ----------------------------------------------------------------------------

echo -e "${GREEN}[5/7] Installing to ${INSTALL_DIR}…${NC}"
run sudo mkdir -p \
    "${INSTALL_DIR}/bin" \
    "${INSTALL_DIR}/etc" \
    "${INSTALL_DIR}/data/history" \
    "${INSTALL_DIR}/data/errors" \
    "${INSTALL_DIR}/cache" \
    "${INSTALL_DIR}/lib" \
    "${INSTALL_DIR}/web/files" \
    "${INSTALL_DIR}/run" \
    "${INSTALL_DIR}/tmp"

run sudo install -m 0755 update-ipsets "${INSTALL_DIR}/bin/update-ipsets"

# The repository catalog is the deployed source of truth. Reinstalls refresh
# the active config only when content changed; preserving the mtime on identical
# installs prevents entity-integrity from treating every reinstall as a catalog
# change and scheduling a full country/ASN artifact rebuild.
CONFIG_TARGET="${INSTALL_DIR}/etc/config"
LEGACY_CONFIG_TARGET="${INSTALL_DIR}/etc/config.yaml"
if [ -d "${CONFIG_TARGET}" ] && diff -qr configs/firehol "${CONFIG_TARGET}" >/dev/null; then
    echo -e "${GREEN}Active configuration already up to date.${NC}"
else
    if [ -e "${CONFIG_TARGET}" ]; then
        CONFIG_BACKUP="${INSTALL_DIR}/etc/config.bak.$(date -u +%Y%m%d%H%M%S)"
        echo -e "${YELLOW}Backing up existing configuration to ${CONFIG_BACKUP}${NC}"
        run sudo cp -a "${CONFIG_TARGET}" "${CONFIG_BACKUP}"
        run sudo rm -rf "${CONFIG_TARGET}"
    fi
    if [ -f "${LEGACY_CONFIG_TARGET}" ]; then
        LEGACY_CONFIG_BACKUP="${INSTALL_DIR}/etc/config.yaml.bak.$(date -u +%Y%m%d%H%M%S)"
        echo -e "${YELLOW}Moving legacy monolithic configuration to ${LEGACY_CONFIG_BACKUP}${NC}"
        run sudo mv "${LEGACY_CONFIG_TARGET}" "${LEGACY_CONFIG_BACKUP}"
    fi
    echo -e "${GREEN}Installing active configuration…${NC}"
    run sudo mkdir -p "${CONFIG_TARGET}"
    run sudo cp -a --no-preserve=ownership configs/firehol/. "${CONFIG_TARGET}/"
fi

# Markdown templates are installed unconditionally and idempotently. They sit
# under the active config directory but are not part of `configs/firehol/`, so
# the per-source-of-truth diff above does not cover them. Replace only when
# content differs, preserving mtime on identical installs so reloads do not
# treat every reinstall as a template change.
MARKDOWN_TEMPLATES="${CONFIG_TARGET}/templates/markdown"
if [ -d "${MARKDOWN_TEMPLATES}" ] && diff -qr configs/templates/markdown "${MARKDOWN_TEMPLATES}" >/dev/null; then
    echo -e "${GREEN}Markdown templates already up to date.${NC}"
else
    echo -e "${GREEN}Installing markdown templates…${NC}"
    run sudo mkdir -p "${MARKDOWN_TEMPLATES}"
    run sudo cp -a --no-preserve=ownership configs/templates/markdown/. "${MARKDOWN_TEMPLATES}/"
fi

# Per-feed HTML description pages are embedded into the binary at
# build time (pkg/web/static/feed-descriptions/*.html via //go:embed).
# Nothing to copy at install time — the repo is self-contained.

# ----------------------------------------------------------------------------
# Step 5: Install systemd unit
# ----------------------------------------------------------------------------
# The unit file is always overwritten, which is intentional — it's part of
# the shipped artifacts. Site-local overrides (admin credentials, admin
# listener/auth flags, runtime.public_base_url in config, MaxMind license
# key, etc.) should go in a drop-in at
# /etc/systemd/system/update-ipsets.service.d/*.conf so they survive
# reinstalls without editing this file.

echo -e "${GREEN}[6/7] Installing systemd service…${NC}"
cat << 'UNIT' | sudo tee /etc/systemd/system/update-ipsets.service > /dev/null
[Unit]
Description=FireHOL IP Lists Update Daemon
Documentation=https://github.com/firehol/firehol
After=network-online.target
Wants=network-online.target

[Service]
Type=notify
ExecStart=/opt/update-ipsets/bin/update-ipsets daemon \
    --config /opt/update-ipsets/etc/config \
    --listen ${UPDATE_IPSETS_LISTEN} \
    ${UPDATE_IPSETS_ADMIN_LISTEN_ARG} \
    ${UPDATE_IPSETS_ADMIN_AUTH_ARG} \
    ${UPDATE_IPSETS_ALLOW_UNAUTHENTICATED_ADMIN_ARG} \
    --enable-all \
    --verbose \
    --web-dir /opt/update-ipsets/web \
    --web-files-dir /opt/update-ipsets/web/files

Restart=on-failure
RestartSec=30
WatchdogSec=300
WorkingDirectory=/opt/update-ipsets

# Listener/auth defaults. Override these in a drop-in to switch
# between shared-listener development and split-listener production
# without replacing the full ExecStart= line.
Environment=UPDATE_IPSETS_LISTEN=:18888
Environment=UPDATE_IPSETS_ADMIN_LISTEN_ARG=
Environment=UPDATE_IPSETS_ADMIN_AUTH_ARG=--admin-auth-mode=required
Environment=UPDATE_IPSETS_ALLOW_UNAUTHENTICATED_ADMIN_ARG=

# OpenTelemetry defaults for the local Netdata otel-plugin. Netdata exposes
# OTLP/gRPC on 127.0.0.1:4317; traces are disabled here because the local
# plugin accepts metrics/logs, not trace spans.
Environment=UPDATE_IPSETS_OTEL=1
Environment=UPDATE_IPSETS_OTEL_PROTOCOL=grpc
Environment=OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317
Environment=OTEL_METRIC_EXPORT_INTERVAL=10000
Environment=OTEL_TRACES_EXPORTER=none

# Path overrides — these env vars are expanded by the YAML config's
# ${VAR-default} templates, directing all data to /opt/update-ipsets.
Environment=HOME=/opt/update-ipsets
Environment=BASE_DIR=/opt/update-ipsets/data
Environment=RUN_PARENT_DIR=/opt/update-ipsets/run
Environment=CACHE_DIR=/opt/update-ipsets/cache
Environment=LIB_DIR=/opt/update-ipsets/lib
Environment=HISTORY_DIR=/opt/update-ipsets/data/history
Environment=ERRORS_DIR=/opt/update-ipsets/data/errors
Environment=TMP_DIR=/opt/update-ipsets/tmp
Environment=WEB_DIR=/opt/update-ipsets/web
Environment=WEB_DIR_FOR_IPSETS=/opt/update-ipsets/web/files

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=no
ReadWritePaths=/opt/update-ipsets

# Resource limits
LimitNOFILE=65536
MemoryMax=2G

[Install]
WantedBy=multi-user.target
UNIT

echo -e "${GREEN}Reloading systemd…${NC}"
run sudo systemctl daemon-reload

# ----------------------------------------------------------------------------
# Step 6: Restart the running service (unless --no-restart)
# ----------------------------------------------------------------------------

if [ "$RESTART" -eq 1 ]; then
    if systemctl is-active --quiet update-ipsets; then
        echo -e "${GREEN}[7/7] Restarting update-ipsets…${NC}"
        run sudo systemctl restart update-ipsets
    elif systemctl is-enabled --quiet update-ipsets 2>/dev/null; then
        echo -e "${GREEN}[7/7] Starting update-ipsets…${NC}"
        run sudo systemctl start update-ipsets
    else
        echo -e "${YELLOW}[7/7] Service is not enabled — skipping restart.${NC}"
        echo "      Enable + start it with:"
        echo "        sudo systemctl enable update-ipsets"
        echo "        sudo systemctl start update-ipsets"
    fi
else
    echo -e "${YELLOW}[7/7] --no-restart specified, leaving current process running.${NC}"
    echo "      The new binary will be picked up on the next manual restart:"
    echo "        sudo systemctl restart update-ipsets"
fi

echo ""
echo -e "${GREEN}=== Installation complete ===${NC}"
echo ""
echo "  Binary:   ${INSTALL_DIR}/bin/update-ipsets"
echo "  Config:   ${INSTALL_DIR}/etc/config/"
echo "  Data:     ${INSTALL_DIR}/data/"
echo "  Cache:    ${INSTALL_DIR}/cache/"
echo "  Web UI:   http://localhost:18888"
echo "  Service:  update-ipsets.service"
echo ""
echo "Commands:"
echo "  sudo systemctl enable update-ipsets   # start on boot"
echo "  sudo systemctl status update-ipsets   # check status"
echo "  journalctl -u update-ipsets -f        # follow logs"
echo "  curl http://localhost:18888/healthz   # health check"
echo ""
