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
  printf >&2 "%b%s >%b %b" "$GRAY" "$(pwd)" "$NC" "$YELLOW"
  printf >&2 "%q " "$@"
  printf >&2 "%b\n" "$NC"
  if "$@"; then
    return 0
  else
    local exit_code=$?
    echo -e >&2 "${RED}[ERROR]${NC} Exit code ${exit_code}: ${YELLOW}$*${NC} (in $(pwd))"
    return "$exit_code"
  fi
}

repair_stale_publish_stages() {
  local install_dir="$1"
  local min_age_minutes=120
  local web_root="${install_dir}/web"
  local entity_root="${install_dir}/lib/entities"

  echo -e "${GREEN}Removing stale generated publish stage directories...${NC}"
  if [ -d "${web_root}" ]; then
    run sudo find "${web_root}" \
      -ignore_readdir_race \
      -mindepth 1 \
      -maxdepth 1 \
      -type d \
      -name '.update-ipsets-web-*' \
      -mmin "+${min_age_minutes}" \
      -exec rm -rf -- {} +
  fi
  if [ -d "${entity_root}" ]; then
    run sudo find "${entity_root}" \
      -ignore_readdir_race \
      -mindepth 1 \
      -maxdepth 1 \
      -type d \
      -name '.update-ipsets-entities-*' \
      -mmin "+${min_age_minutes}" \
      -exec rm -rf -- {} +
  fi
}

repair_git_object_stores() {
  local install_dir="$1"

  if ! command -v git >/dev/null 2>&1; then
    echo -e "${YELLOW}Skipping generated git repository maintenance; git is not installed.${NC}"
    return 0
  fi

  echo -e "${GREEN}Compacting generated git repository object stores...${NC}"
  for repo in "${install_dir}/data" "${install_dir}/web"; do
    if sudo test -d "${repo}/.git"; then
      run sudo -u iplists git -C "${repo}" gc --prune=now
    fi
  done
}

SERVICE_STOPPED_FOR_INSTALL=0
MUTABLE_REPAIR_ALLOWED=0
stop_active_service_for_mutable_repair() {
  if [ "$RESTART" -ne 1 ]; then
    return 0
  fi
  if systemctl is-active --quiet update-ipsets; then
    echo -e "${GREEN}Stopping update-ipsets before repairing mutable runtime trees...${NC}"
    run sudo systemctl stop update-ipsets
    SERVICE_STOPPED_FOR_INSTALL=1
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

if [ "$RESTART" -eq 0 ] && systemctl is-active --quiet update-ipsets; then
    echo -e "${YELLOW}Skipping stale publish stage repair while update-ipsets is running with --no-restart.${NC}"
    echo "      Restart the service and run the installer without --no-restart to repair generated stage directories safely."
else
    stop_active_service_for_mutable_repair
    repair_stale_publish_stages "${INSTALL_DIR}"
    MUTABLE_REPAIR_ALLOWED=1
fi

# Create service identity if missing. The group is explicit because some
# useradd policies do not create a same-name group for system users.
if ! getent group iplists >/dev/null 2>&1; then
    echo -e "${GREEN}Creating iplists system group...${NC}"
    run sudo groupadd --system iplists
fi
if ! id -u iplists >/dev/null 2>&1; then
    echo -e "${GREEN}Creating iplists system user...${NC}"
    run sudo useradd --system --gid iplists --home-dir "${INSTALL_DIR}" --no-create-home --shell /usr/sbin/nologin iplists
fi

run sudo install -o root -g iplists -m 0750 update-ipsets "${INSTALL_DIR}/bin/update-ipsets"

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

# Keep shipped artifacts immutable to the daemon and writable runtime state
# owned by the service user. Do not chown the whole tree: bin/ and etc/ are
# part of the trusted install surface.
echo -e "${GREEN}Setting install ownership...${NC}"
run sudo chown root:iplists "${INSTALL_DIR}"
run sudo chown -R root:iplists "${INSTALL_DIR}/bin" "${INSTALL_DIR}/etc"
run sudo chmod 0750 "${INSTALL_DIR}" "${INSTALL_DIR}/bin" "${INSTALL_DIR}/etc"
run sudo chmod 0750 "${INSTALL_DIR}/bin/update-ipsets"
run sudo find "${INSTALL_DIR}/etc" -type d -exec chmod 0750 {} +
run sudo find "${INSTALL_DIR}/etc" -type f -exec chmod 0640 {} +
run sudo chown -R iplists:iplists \
    "${INSTALL_DIR}/data" \
    "${INSTALL_DIR}/cache" \
    "${INSTALL_DIR}/lib" \
    "${INSTALL_DIR}/web" \
    "${INSTALL_DIR}/run" \
    "${INSTALL_DIR}/tmp"
run sudo find \
    "${INSTALL_DIR}/data" \
    "${INSTALL_DIR}/cache" \
    "${INSTALL_DIR}/lib" \
    "${INSTALL_DIR}/web" \
    "${INSTALL_DIR}/run" \
    "${INSTALL_DIR}/tmp" \
    -ignore_readdir_race \
    -type d -exec chmod 0700 {} +
run sudo find \
    "${INSTALL_DIR}/data" \
    "${INSTALL_DIR}/cache" \
    "${INSTALL_DIR}/lib" \
    "${INSTALL_DIR}/web" \
    "${INSTALL_DIR}/run" \
    "${INSTALL_DIR}/tmp" \
    -ignore_readdir_race \
    -type f -exec chmod 0600 {} +

if [ "$MUTABLE_REPAIR_ALLOWED" -eq 1 ]; then
    repair_git_object_stores "${INSTALL_DIR}"
else
    echo -e "${YELLOW}Skipping generated git repository maintenance while update-ipsets may be running.${NC}"
fi

# Per-feed HTML description pages are embedded into the binary at
# build time (pkg/web/static/feed-descriptions/*.html via //go:embed).
# Nothing to copy at install time — the repo is self-contained.

# ----------------------------------------------------------------------------
# Step 5: Install systemd unit
# ----------------------------------------------------------------------------
# The unit file is always overwritten, which is intentional — it's part of
# the shipped artifacts. When Tailscale is available, the admin listener is
# automatically bound to the Tailscale IP with auth disabled. Site-local
# overrides (admin credentials, admin listener/auth flags, MaxMind license
# key, etc.) should go in a drop-in at
# /etc/systemd/system/update-ipsets.service.d/*.conf so they survive
# reinstalls without editing this file.

# Installer default: public on localhost, admin on localhost, auth disabled.
# When Tailscale is available, admin moves to the Tailscale IPv4 address.
# Override these environment variables in a systemd drop-in for another model.
LISTEN="127.0.0.1:18888"
ADMIN_LISTEN_ARG="--admin-listen=127.0.0.1:18889"
ADMIN_AUTH_ARG="--admin-auth-mode=disabled"
ALLOW_UNAUTH_ADMIN_ARG="--allow-unauthenticated-admin"
TAILSCALE_IP=""

# If Tailscale is present, move admin to the Tailscale IP
if command -v tailscale >/dev/null 2>&1; then
    TAILSCALE_IP=$(tailscale ip -4 2>/dev/null || true)
    if [ -n "$TAILSCALE_IP" ]; then
        echo -e "${GREEN}Tailscale detected at ${TAILSCALE_IP} — moving admin to Tailscale${NC}"
        ADMIN_LISTEN_ARG="--admin-listen=${TAILSCALE_IP}:18889"
    fi
fi

echo -e "${GREEN}[6/7] Installing systemd service…${NC}"
cat << UNIT | sudo tee /etc/systemd/system/update-ipsets.service > /dev/null
[Unit]
Description=FireHOL IP Lists Update Daemon
Documentation=https://github.com/firehol/firehol
After=network-online.target tailscaled.service
Wants=network-online.target tailscaled.service

[Service]
Type=notify
User=iplists
Group=iplists
UMask=0077
ExecStart=${INSTALL_DIR}/bin/update-ipsets daemon \\
    --config ${INSTALL_DIR}/etc/config \\
    --listen \${UPDATE_IPSETS_LISTEN} \\
    \${UPDATE_IPSETS_ADMIN_LISTEN_ARG} \\
    \${UPDATE_IPSETS_ADMIN_AUTH_ARG} \\
    \${UPDATE_IPSETS_ALLOW_UNAUTHENTICATED_ADMIN_ARG} \\
    --enable-all \\
    --verbose \\
    --web-dir ${INSTALL_DIR}/web \\
    --web-files-dir ${INSTALL_DIR}/web/files

Restart=on-failure
RestartSec=30
WatchdogSec=300
WorkingDirectory=${INSTALL_DIR}
LogNamespace=iplists

# Listener/auth defaults. The installer defaults to unauthenticated admin on
# localhost, or on the Tailscale IPv4 address when Tailscale is available.
# Override these in a drop-in without replacing the full ExecStart= line.
Environment=UPDATE_IPSETS_LISTEN=${LISTEN}
Environment=UPDATE_IPSETS_ADMIN_LISTEN_ARG=${ADMIN_LISTEN_ARG}
Environment=UPDATE_IPSETS_ADMIN_AUTH_ARG=${ADMIN_AUTH_ARG}
Environment=UPDATE_IPSETS_ALLOW_UNAUTHENTICATED_ADMIN_ARG=${ALLOW_UNAUTH_ADMIN_ARG}

# OpenTelemetry metric-export defaults for the local Netdata otel-plugin.
# Netdata exposes OTLP/gRPC on 127.0.0.1:4317.
Environment=UPDATE_IPSETS_OTEL=1
Environment=UPDATE_IPSETS_OTEL_PROTOCOL=grpc
Environment=OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317
Environment=OTEL_METRIC_EXPORT_INTERVAL=10000

# Path overrides — these env vars are expanded by the YAML config's
# \${VAR-default} templates, directing all data to ${INSTALL_DIR}.
Environment=HOME=${INSTALL_DIR}
Environment=BASE_DIR=${INSTALL_DIR}/data
Environment=RUN_PARENT_DIR=${INSTALL_DIR}/run
Environment=CACHE_DIR=${INSTALL_DIR}/cache
Environment=LIB_DIR=${INSTALL_DIR}/lib
Environment=HISTORY_DIR=${INSTALL_DIR}/data/history
Environment=ERRORS_DIR=${INSTALL_DIR}/data/errors
Environment=TMP_DIR=${INSTALL_DIR}/tmp
Environment=WEB_DIR=${INSTALL_DIR}/web
Environment=WEB_DIR_FOR_IPSETS=${INSTALL_DIR}/web/files

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=no
ReadWritePaths=${INSTALL_DIR}/data ${INSTALL_DIR}/cache ${INSTALL_DIR}/lib ${INSTALL_DIR}/web ${INSTALL_DIR}/run ${INSTALL_DIR}/tmp

# Resource limits
LimitNOFILE=65536
MemoryHigh=2.75G
MemoryMax=3G
Environment=GOMEMLIMIT=2560MiB

[Install]
WantedBy=multi-user.target
UNIT

echo -e "${GREEN}Reloading systemd…${NC}"
run sudo systemctl daemon-reload

# ----------------------------------------------------------------------------
# Step 6: Restart the running service (unless --no-restart)
# ----------------------------------------------------------------------------

if [ "$RESTART" -eq 1 ]; then
    if [ "$SERVICE_STOPPED_FOR_INSTALL" -eq 1 ]; then
        echo -e "${GREEN}[7/7] Starting update-ipsets…${NC}"
        run sudo systemctl start update-ipsets
    elif systemctl is-active --quiet update-ipsets; then
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
echo "  Web UI:   http://127.0.0.1:18888"
if [ -n "$TAILSCALE_IP" ]; then
    echo "  Admin:    http://${TAILSCALE_IP}:18889 (Tailscale, no auth)"
else
    echo "  Admin:    http://127.0.0.1:18889 (localhost, no auth)"
fi
echo "  Service:  update-ipsets.service"
echo ""
echo "Commands:"
echo "  sudo systemctl enable update-ipsets                   # start on boot"
echo "  sudo systemctl status update-ipsets                   # check status"
echo "  journalctl --namespace=iplists -u update-ipsets -f    # follow logs"
echo "  curl http://127.0.0.1:18888/healthz                  # health check"
echo ""
