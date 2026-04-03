#!/bin/bash
# Compatibility wrapper. The canonical helper is scripts/sync-from-bash-version.sh.
#
# Legacy usage preserved:
#   ./scripts/sync-from-d1.sh [INSTALL_DIR]
# Optional override:
#   REMOTE=some-host ./scripts/sync-from-d1.sh [INSTALL_DIR]
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_HOST="${REMOTE:-d1}"

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  exec "${SCRIPT_DIR}/sync-from-bash-version.sh" --help
fi

exec "${SCRIPT_DIR}/sync-from-bash-version.sh" "${SOURCE_HOST}" "${1:-/opt/update-ipsets}"
