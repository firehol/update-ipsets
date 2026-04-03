#!/bin/bash
# Import bash-era update-ipsets data into the Go installation.
#
# Safety:
#   - pre-sync backup of the live Go installation
#   - full-tree staging import from a bash source host or localhost
#   - preserves legacy bash history snapshots as the canonical downloader history format
#   - batched stage-to-live promotion only after the import is complete
#
# Usage:
#   ./scripts/sync-from-bash-version.sh <source-host|localhost> [INSTALL_DIR]
#   default INSTALL_DIR: /opt/update-ipsets
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; GRAY='\033[0;90m'; NC='\033[0m'
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

run() {
  printf >&2 "${GRAY}$(pwd) >${NC} ${YELLOW}"
  printf >&2 "%q " "$@"
  printf >&2 "${NC}\n"
  local exit_code=0
  "$@" || exit_code=$?
  if [ "${exit_code}" -ne 0 ]; then
    echo -e >&2 "${RED}[ERROR]${NC} Exit code ${exit_code}: ${YELLOW}$*${NC} (in $(pwd))"
    return "${exit_code}"
  fi
}

run_to_file() {
  local out="$1"
  shift
  printf >&2 "${GRAY}$(pwd) >${NC} ${YELLOW}"
  printf >&2 "%q " "$@"
  printf >&2 "> %q${NC}\n" "${out}"
  local exit_code=0
  "$@" >"${out}" || exit_code=$?
  if [ "${exit_code}" -ne 0 ]; then
    echo -e >&2 "${RED}[ERROR]${NC} Exit code ${exit_code}: ${YELLOW}$* > ${out}${NC} (in $(pwd))"
    return "${exit_code}"
  fi
}

die() {
  echo -e >&2 "${RED}[ERROR]${NC} $*"
  exit 1
}

usage() {
  sed -n '1,10p' "$0"
}

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  usage
  exit 0
fi

SOURCE_HOST="${1:-}"
INSTALL_DIR="${2:-/opt/update-ipsets}"

[ -n "${SOURCE_HOST}" ] || die "SOURCE_HOST is required. Usage: ./scripts/sync-from-bash-version.sh <source-host|localhost> [INSTALL_DIR]"
INSTALL_DIR="${INSTALL_DIR%/}"
[ -n "${INSTALL_DIR}" ] || die "INSTALL_DIR is empty"
[[ "${INSTALL_DIR}" = /* ]] || die "INSTALL_DIR must be an absolute path"
[ "${INSTALL_DIR}" != "/" ] || die "INSTALL_DIR cannot be /"

BASH_BASE="${BASH_BASE:-/etc/firehol/ipsets}"
BASH_LIB="${BASH_LIB:-/var/lib/update-ipsets}"
BASH_CONFIG="${BASH_CONFIG:-/etc/firehol/update-ipsets.conf}"
BASH_WEB="${BASH_WEB:-/var/www/blocklists}"

LOCAL_DATA="${INSTALL_DIR}/data"
LOCAL_HISTORY="${LOCAL_DATA}/history"
LOCAL_LIB="${INSTALL_DIR}/lib"
LOCAL_WEB="${INSTALL_DIR}/web"
LOCAL_FILES_DIR="${LOCAL_WEB}/files"
LOCAL_BACKUP="${INSTALL_DIR}/backups"
ENV_FILE="${INSTALL_DIR}/.update-ipsets.env"
BIN_FILE="${INSTALL_DIR}/bin/update-ipsets"

STAGE_DIR="${INSTALL_DIR}/import-bash-version"
STAGE_DATA="${STAGE_DIR}/data"
STAGE_LIB="${STAGE_DIR}/lib"
STAGE_WEB="${STAGE_DIR}/web"
STAGE_FILES_DIR="${STAGE_WEB}/files"
STAGE_MANIFESTS="${STAGE_DIR}/manifests"
STAGE_CONFIG="${STAGE_DIR}/update-ipsets.conf"
STAGE_LOCAL_CACHE_JSON="${STAGE_DIR}/local-cache.json"
STAGE_MERGED_CACHE_JSON="${STAGE_DIR}/merged-cache.json"

MANAGED_ENV_KEYS=("AUTOSHUN_API_KEY" "BLUELIV_API_KEY" "XFORCE_API_KEY" "XFORCE_API_PASSWORD" "IP2LOCATION_API_KEY" "MAXMIND_LICENSE_KEY")

PRODUCTION_FEED_COUNT=0
LOCAL_ONLY_FEED_COUNT=0

assert_under_install() {
  local path="$1"
  local purpose="$2"
  case "${path}" in
    "${INSTALL_DIR}"|"${INSTALL_DIR}/"*) ;;
    *) die "${purpose} path is outside INSTALL_DIR: ${path}" ;;
  esac
}

assert_stage_path() {
  local path="$1"
  case "${path}" in
    "${STAGE_DIR}"|"${STAGE_DIR}/"*) ;;
    *) die "refusing to operate on non-stage path: ${path}" ;;
  esac
}

is_managed_env_key() {
  local key="$1"
  local managed
  for managed in "${MANAGED_ENV_KEYS[@]}"; do
    [ "${key}" != "${managed}" ] || return 0
  done
  return 1
}

is_local_source() {
  [ "${SOURCE_HOST}" = "localhost" ] || [ "${SOURCE_HOST}" = "127.0.0.1" ] || [ "${SOURCE_HOST}" = "::1" ]
}

detect_rsync_compression() {
  REMOTE_RSYNC_ARGS=(-a -z --info=progress2 --human-readable)
  LOCAL_RSYNC_ARGS=(-a --info=progress2 --human-readable)
  if is_local_source; then
    RSYNC_COMPRESSION="local"
    return 0
  fi
  if rsync --help 2>/dev/null | grep -q -- '--compress-choice' &&
    ssh -o ConnectTimeout=5 "${SOURCE_HOST}" "rsync --help 2>/dev/null | grep -q -- '--compress-choice'" 2>/dev/null; then
    REMOTE_RSYNC_ARGS+=("--compress-choice=zstd")
    RSYNC_COMPRESSION="zstd"
  else
    RSYNC_COMPRESSION="gzip"
  fi
}

remote_rsync() {
  run rsync "${REMOTE_RSYNC_ARGS[@]}" "$@"
}

local_rsync() {
  run sudo rsync "${LOCAL_RSYNC_ARGS[@]}" "$@"
}

source_rsync() {
  if is_local_source; then
    run sudo rsync "${LOCAL_RSYNC_ARGS[@]}" "$@"
    return 0
  fi
  run rsync "${REMOTE_RSYNC_ARGS[@]}" "$@"
}

source_file_exists() {
  local path="$1"
  if is_local_source; then
    sudo test -f "${path}"
    return $?
  fi
  ssh "${SOURCE_HOST}" "test -f '${path}'" 2>/dev/null
}

copy_source_file() {
  local src="$1"
  local dst="$2"
  if is_local_source; then
    run sudo cp "${src}" "${dst}"
    run sudo chown "$(id -u):$(id -g)" "${dst}"
    return 0
  fi
  remote_rsync "${SOURCE_HOST}:${src}" "${dst}"
}

run_update_ipsets() {
  if [ -f "${SCRIPT_DIR}/go.mod" ] && [ -d "${SCRIPT_DIR}/cmd/update-ipsets" ]; then
    (cd "${SCRIPT_DIR}" && run go run ./cmd/update-ipsets "$@")
    return 0
  fi
  if [ -x "${BIN_FILE}" ]; then
    run "${BIN_FILE}" "$@"
    return 0
  fi
  die "cannot run update-ipsets: neither ${BIN_FILE} nor Go source tree is available"
}

run_update_ipsets_to_file() {
  local out="$1"
  shift
  if [ -f "${SCRIPT_DIR}/go.mod" ] && [ -d "${SCRIPT_DIR}/cmd/update-ipsets" ]; then
    (cd "${SCRIPT_DIR}" && run_to_file "${out}" go run ./cmd/update-ipsets "$@")
    return 0
  fi
  if [ -x "${BIN_FILE}" ]; then
    run_to_file "${out}" "${BIN_FILE}" "$@"
    return 0
  fi
  die "cannot run update-ipsets: neither ${BIN_FILE} nor Go source tree is available"
}

stop_daemon_if_running() {
  WAS_RUNNING=0
  if systemctl is-active --quiet update-ipsets 2>/dev/null; then
    WAS_RUNNING=1
    echo -e "${YELLOW}Stopping update-ipsets daemon...${NC}"
    run sudo systemctl stop update-ipsets
  fi
  local state
  local i
  for i in {1..60}; do
    state="$(systemctl show -p ActiveState --value update-ipsets 2>/dev/null || true)"
    if [ "${state}" != "active" ] && [ "${state}" != "activating" ] && [ "${state}" != "deactivating" ]; then
      return 0
    fi
    sleep 1
  done
  die "update-ipsets did not become inactive within 60 seconds"
}

create_pre_sync_backup() {
  echo -e "${GREEN}Creating pre-sync backup...${NC}"
  run sudo mkdir -p "${LOCAL_BACKUP}"
  BACKUP_FILE="${LOCAL_BACKUP}/pre-bash-sync-$(date +%Y%m%d-%H%M%S).tar.gz"

  local backup_targets=()
  local backup_rel=()
  sudo test -d "${LOCAL_DATA}" && backup_targets+=("${LOCAL_DATA}")
  sudo test -d "${LOCAL_LIB}" && backup_targets+=("${LOCAL_LIB}")
  sudo test -d "${LOCAL_WEB}" && backup_targets+=("${LOCAL_WEB}")
  sudo test -f "${ENV_FILE}" && backup_targets+=("${ENV_FILE}")
  if [ "${#backup_targets[@]}" -eq 0 ]; then
    echo -e "${YELLOW}  Nothing to back up${NC}"
    BACKUP_FILE="(none)"
    return 0
  fi
  local path
  for path in "${backup_targets[@]}"; do
    backup_rel+=("${path#/}")
  done
  run sudo tar czf "${BACKUP_FILE}" \
    --warning=no-file-changed \
    --checkpoint=200000 \
    "--checkpoint-action=echo=tar checkpoint %u" \
    -C / "${backup_rel[@]}"
  echo -e "  Backup: ${BACKUP_FILE} ($(sudo du -h "${BACKUP_FILE}" | cut -f1))"
}

prepare_stage() {
  echo -e "${GREEN}Preparing staging directory...${NC}"
  assert_stage_path "${STAGE_DIR}"
  if sudo test -e "${STAGE_DIR}"; then
    run sudo rm -rf -- "${STAGE_DIR}"
  fi
  run sudo mkdir -p "${STAGE_DATA}" "${STAGE_LIB}" "${STAGE_WEB}" "${STAGE_FILES_DIR}" "${STAGE_MANIFESTS}"
  run sudo chown -R "$(id -u):$(id -g)" "${STAGE_DIR}"
}

import_bash_to_stage() {
  echo -e "${GREEN}Importing full bash data into staging...${NC}"
  echo -e "  Source host: ${SOURCE_HOST}"
  echo -e "  Rsync compression: ${RSYNC_COMPRESSION}"
  if is_local_source; then
    source_rsync --delete "${BASH_BASE}/" "${STAGE_DATA}/"
    source_rsync --delete "${BASH_LIB}/" "${STAGE_LIB}/"
    source_rsync --delete "${BASH_WEB}/" "${STAGE_WEB}/"
    if source_file_exists "${BASH_CONFIG}"; then
      copy_source_file "${BASH_CONFIG}" "${STAGE_CONFIG}"
    else
      echo -e "${YELLOW}  Could not read ${BASH_CONFIG} from localhost${NC}"
    fi
    run sudo chown -R "$(id -u):$(id -g)" "${STAGE_DIR}"
    return 0
  fi
  source_rsync --delete "${SOURCE_HOST}:${BASH_BASE}/" "${STAGE_DATA}/"
  source_rsync --delete "${SOURCE_HOST}:${BASH_LIB}/" "${STAGE_LIB}/"
  source_rsync --delete "${SOURCE_HOST}:${BASH_WEB}/" "${STAGE_WEB}/"
  if source_file_exists "${BASH_CONFIG}"; then
    copy_source_file "${BASH_CONFIG}" "${STAGE_CONFIG}"
  else
    echo -e "${YELLOW}  Could not read ${BASH_CONFIG} from ${SOURCE_HOST}${NC}"
  fi
}

collect_base_feed_names() {
  local dir="$1"
  [ -d "${dir}" ] || return 0
  find "${dir}" -maxdepth 1 -type f \
    \( -name '*.source' -o -name '*.ipset' -o -name '*.netset' -o -name '*.split' -o -name '*.setinfo' \) \
    -printf '%f\n' | sed -E 's/\.(source|ipset|netset|split|setinfo)$//'
}

collect_stage_feed_names() {
  find "${STAGE_LIB}" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' 2>/dev/null || true
  find "${STAGE_DATA}/history" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' 2>/dev/null || true
  collect_base_feed_names "${STAGE_DATA}"
  find "${STAGE_FILES_DIR}" -maxdepth 1 -type f \
    \( -name '*.ipset' -o -name '*.netset' \) \
    -printf '%f\n' 2>/dev/null | sed -E 's/\.(ipset|netset)$//'
}

collect_local_feed_names() {
  sudo find "${LOCAL_LIB}" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' 2>/dev/null || true
  sudo find "${LOCAL_HISTORY}" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' 2>/dev/null || true
  sudo find "${LOCAL_DATA}" -maxdepth 1 -type f \
    \( -name '*.source' -o -name '*.ipset' -o -name '*.netset' -o -name '*.split' -o -name '*.setinfo' \) \
    -printf '%f\n' 2>/dev/null | sed -E 's/\.(source|ipset|netset|split|setinfo)$//'
  sudo find "${LOCAL_FILES_DIR}" -maxdepth 1 -type f \
    \( -name '*.ipset' -o -name '*.netset' \) \
    -printf '%f\n' 2>/dev/null | sed -E 's/\.(ipset|netset)$//'
}

write_manifests() {
  echo -e "${GREEN}Building imported/local feed manifests...${NC}"
  collect_stage_feed_names | sort -u >"${STAGE_MANIFESTS}/production-feeds.txt"
  collect_local_feed_names | sort -u >"${STAGE_MANIFESTS}/local-feeds.txt"
  comm -23 "${STAGE_MANIFESTS}/local-feeds.txt" "${STAGE_MANIFESTS}/production-feeds.txt" >"${STAGE_MANIFESTS}/local-only-feeds.txt"
  PRODUCTION_FEED_COUNT="$(wc -l <"${STAGE_MANIFESTS}/production-feeds.txt" | tr -d ' ')"
  LOCAL_ONLY_FEED_COUNT="$(wc -l <"${STAGE_MANIFESTS}/local-only-feeds.txt" | tr -d ' ')"
  echo -e "  Feeds seen in source:      ${PRODUCTION_FEED_COUNT}"
  echo -e "  Local-only feeds kept:     ${LOCAL_ONLY_FEED_COUNT}"
  echo -e "  Manifest dir:              ${STAGE_MANIFESTS}"
}

snapshot_local_cache_json() {
  if sudo test -f "${LOCAL_DATA}/.cache.json"; then
    run sudo cp "${LOCAL_DATA}/.cache.json" "${STAGE_LOCAL_CACHE_JSON}"
    run sudo chown "$(id -u):$(id -g)" "${STAGE_LOCAL_CACHE_JSON}"
  fi
}

promote_live_trees() {
  echo -e "${GREEN}Copying staged data into live directories in batch...${NC}"
  run sudo mkdir -p "${LOCAL_DATA}" "${LOCAL_LIB}" "${LOCAL_WEB}" "${LOCAL_FILES_DIR}"
  echo -e "  Batch copy: ${STAGE_DATA} -> ${LOCAL_DATA}"
  local_rsync "${STAGE_DATA}/" "${LOCAL_DATA}/"
  echo -e "  Batch copy: ${STAGE_LIB} -> ${LOCAL_LIB}"
  local_rsync "${STAGE_LIB}/" "${LOCAL_LIB}/"
  echo -e "  Batch copy: ${STAGE_WEB} -> ${LOCAL_WEB}"
  local_rsync "${STAGE_WEB}/" "${LOCAL_WEB}/"
}

merge_cache_state() {
  [ -f "${STAGE_DATA}/.cache" ] || return 0
  echo -e "${GREEN}Merging bash cache with local-only cache entries...${NC}"
  run_update_ipsets cache-merge \
    --legacy "${STAGE_DATA}/.cache" \
    --local-json "${STAGE_LOCAL_CACHE_JSON}" \
    --local-only "${STAGE_MANIFESTS}/local-only-feeds.txt" \
    --out "${STAGE_MERGED_CACHE_JSON}"
}

promote_cache_files() {
  [ -f "${STAGE_MERGED_CACHE_JSON}" ] || return 0
  echo -e "${GREEN}Promoting merged cache JSON...${NC}"
  run sudo rsync -a "${STAGE_MERGED_CACHE_JSON}" "${LOCAL_DATA}/.cache.json"
}

extract_env_keys() {
  [ -f "${STAGE_CONFIG}" ] || return 0
  echo -e "${GREEN}Extracting API keys...${NC}"
  local old_env="${STAGE_DIR}/old-update-ipsets.env"
  local env_temp
  env_temp="$(mktemp "${STAGE_DIR}/.update-ipsets.env.XXXXXX")"
  if sudo test -f "${ENV_FILE}"; then
    run sudo cp "${ENV_FILE}" "${old_env}"
    run sudo chown "$(id -u):$(id -g)" "${old_env}"
  fi
  {
    echo "# update-ipsets API keys and credentials"
    echo "# This file is loaded by the Go update-ipsets engine at startup."
    echo "# Environment variables already set by systemd take precedence."
    echo "# DO NOT commit this file to version control."
    echo ""
    if [ -f "${old_env}" ]; then
      while IFS= read -r line; do
        local trimmed key
        trimmed="$(printf '%s\n' "${line}" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
        [ -n "${trimmed}" ] || continue
        [[ "${trimmed}" != \#* ]] || continue
        [[ "${trimmed}" == *=* ]] || continue
        key="${trimmed%%=*}"
        if ! is_managed_env_key "${key}"; then
          printf '%s\n' "${trimmed}"
        fi
      done <"${old_env}"
    fi
    while IFS= read -r line; do
      line="$(printf '%s\n' "${line}" | sed 's/^[[:space:]]*//')"
      [ -n "${line}" ] || continue
      [[ "${line}" != \#* ]] || continue
      if [[ "${line}" =~ ^(AUTOSHUN_API_KEY|BLUELIV_API_KEY|XFORCE_API_KEY|XFORCE_API_PASSWORD|IP2LOCATION_API_KEY|MAXMIND_LICENSE_KEY)= ]]; then
        local key value
        key="${BASH_REMATCH[1]}"
        value="${line#*=}"
        value="$(printf '%s\n' "${value}" | sed "s/^['\"]//;s/['\"]$//")"
        printf '%s=%s\n' "${key}" "${value}"
      fi
    done <"${STAGE_CONFIG}"
  } >"${env_temp}"
  run sudo cp "${env_temp}" "${ENV_FILE}"
  run sudo chmod 600 "${ENV_FILE}"
  echo -e "  API keys written to ${ENV_FILE}"
  echo -e "  Keys found:"
  sudo grep -v '^#' "${ENV_FILE}" | grep -v '^$' | sed 's/=.*//' | while IFS= read -r key; do
    echo -e "    - ${key}"
  done
}

cleanup_stale_old_go_files() {
  echo -e "${GREEN}Cleaning stale old-Go filenames after copy...${NC}"
  local removed=0
  local stale twin
  while IFS= read -r -d '' stale; do
    twin="${stale%.set}"
    if sudo test -f "${twin}"; then
      run sudo rm -f -- "${stale}"
      removed=$((removed + 1))
    fi
  done < <(sudo find "${LOCAL_LIB}" -mindepth 2 -maxdepth 2 -type f -name 'latest.set' -print0 2>/dev/null)
  while IFS= read -r -d '' stale; do
    twin="${stale%.set}"
    if sudo test -f "${twin}"; then
      run sudo rm -f -- "${stale}"
      removed=$((removed + 1))
    fi
  done < <(sudo find "${LOCAL_LIB}" -mindepth 3 -maxdepth 3 -type f -name '*.set' -path '*/new/*' -print0 2>/dev/null)
  echo -e "  Stale old-Go duplicates removed: ${removed}"
}

fix_ownership() {
  echo -e "${GREEN}Fixing ownership...${NC}"
  run sudo chown -R root:root "${INSTALL_DIR}"
}

print_summary() {
  local imported_history_count history_count lib_count source_count data_set_count web_set_count web_csv_count stale_count
  imported_history_count="$(find "${STAGE_DATA}/history" -name '*.set' 2>/dev/null | wc -l | tr -d ' ')"
  history_count="$(sudo find "${LOCAL_HISTORY}" -name '*.set' 2>/dev/null | wc -l | tr -d ' ')"
  lib_count="$(sudo find "${LOCAL_LIB}" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l | tr -d ' ')"
  source_count="$(sudo find "${LOCAL_DATA}" -maxdepth 1 -type f -name '*.source' 2>/dev/null | wc -l | tr -d ' ')"
  data_set_count="$(sudo find "${LOCAL_DATA}" -maxdepth 1 -type f \( -name '*.ipset' -o -name '*.netset' \) 2>/dev/null | wc -l | tr -d ' ')"
  web_set_count="$(sudo find "${LOCAL_FILES_DIR}" -maxdepth 1 -type f \( -name '*.ipset' -o -name '*.netset' \) 2>/dev/null | wc -l | tr -d ' ')"
  web_csv_count="$(sudo find "${LOCAL_WEB}" -maxdepth 1 -type f -name '*.csv' 2>/dev/null | wc -l | tr -d ' ')"
  stale_count="$(sudo find "${LOCAL_LIB}" -maxdepth 3 \( -name 'latest.set' -o -path '*/new/*.set' \) 2>/dev/null | wc -l | tr -d ' ')"
  echo ""
  echo -e "${GREEN}=== Sync complete ===${NC}"
  echo ""
  echo "  Source host:               ${SOURCE_HOST}"
  echo "  Rsync compression:         ${RSYNC_COMPRESSION}"
  echo "  Feeds seen in source:      ${PRODUCTION_FEED_COUNT}"
  echo "  Local-only feeds kept:     ${LOCAL_ONLY_FEED_COUNT}"
  echo "  Imported history snapshots:${imported_history_count}"
  echo "  Live history snapshots:    ${history_count}"
  echo "  Lib directories:           ${lib_count}"
  echo "  Source files:              ${source_count}"
  echo "  Data set files:            ${data_set_count}"
  echo "  Web set files:             ${web_set_count}"
  echo "  Web CSV files:             ${web_csv_count}"
  echo "  Remaining old-Go files:    ${stale_count}"
  echo "  Pre-sync backup:           ${BACKUP_FILE}"
  echo "  Manifests:                 ${STAGE_MANIFESTS}"
  echo "  Env file:                  ${ENV_FILE}"
  echo ""
  echo "To restore from the pre-sync backup:"
  echo "  sudo systemctl stop update-ipsets"
  echo "  sudo tar xzf ${BACKUP_FILE} -C /"
  echo "  sudo systemctl start update-ipsets"
  echo ""
}

restart_daemon_if_needed() {
  if [ "${WAS_RUNNING}" -eq 1 ]; then
    echo -e "${GREEN}Restarting update-ipsets daemon...${NC}"
    run sudo systemctl start update-ipsets
    sleep 2
    if systemctl is-active --quiet update-ipsets 2>/dev/null; then
      echo -e "${GREEN}Daemon restarted successfully${NC}"
    else
      echo -e "${RED}Daemon failed to start. Check: journalctl -u update-ipsets -n 50${NC}"
      exit 1
    fi
  fi
}

echo -e "${GREEN}=== Syncing bash-era data from ${SOURCE_HOST} to ${INSTALL_DIR} ===${NC}"
if is_local_source; then
  echo -e "${GREEN}Using local bash source directories${NC}"
else
  echo -e "${GREEN}Checking connectivity to ${SOURCE_HOST}...${NC}"
  if ! ssh -o ConnectTimeout=5 "${SOURCE_HOST}" "true" 2>/dev/null; then
    die "Cannot reach ${SOURCE_HOST} via SSH"
  fi
fi
assert_under_install "${LOCAL_DATA}" "data"
assert_under_install "${LOCAL_LIB}" "lib"
assert_under_install "${LOCAL_WEB}" "web"
assert_under_install "${LOCAL_BACKUP}" "backup"
assert_stage_path "${STAGE_DIR}"
detect_rsync_compression
stop_daemon_if_running
run sudo mkdir -p "${LOCAL_DATA}" "${LOCAL_HISTORY}" "${LOCAL_LIB}" "${LOCAL_WEB}" "${LOCAL_FILES_DIR}"
create_pre_sync_backup
prepare_stage
import_bash_to_stage
write_manifests
snapshot_local_cache_json
promote_live_trees
merge_cache_state
promote_cache_files
extract_env_keys
cleanup_stale_old_go_files
fix_ownership
print_summary
restart_daemon_if_needed
echo ""
echo -e "${GREEN}Done.${NC}"
echo ""
echo "Next steps:"
echo "  journalctl -u update-ipsets -f"
echo "  curl http://localhost:18888/healthz"
echo ""
