#!/usr/bin/env bash
# Manual orchestrator for feed enrichment — independent streaming slots.
#
# Concurrency model: N parallel slots, each pulling the next feed from
# the queue as soon as its current feed completes. Not batch-of-N: a
# slot does not wait for its peers. When a worker finishes, the next
# queued feed starts immediately in that slot.
#
# Manual on purpose: enrichment calls paid models (web-search, web-fetch,
# extractor). Cost discipline is enforced at this layer — a human
# decides what gets enriched and when.
#
# Inputs (any of):
#   - --feeds a,b,c       : explicit comma-separated feed list
#   - --category NAME     : all eligible third-party feeds in one category
#   - --all               : all eligible third-party source feeds
#   - feed names as positional args
#   - feed names on stdin (one per line)
#   - --unenriched          : auto-discover feeds with YAML but no clean run
#   - --retry-failed        : also re-queue feeds whose last run did not pass
#
# Flags:
#   -j N                    : parallel slots (default 4)
#   --unenriched
#   --retry-failed
#   --limit N               : cap the queue at N feeds (safety net)
#   --no-finalize           : do not write successful outputs back to YAML
#   --dry-run               : print queue, exit
#
# Output:
#   - real-time per-feed STATE lines on stdout
#   - per-feed full log under .local/agents/feed-enrichment-pool/<ts>/<feed>.log
#   - results.tsv summary in the same dir

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

SLOTS=4
UNENRICHED=0
RETRY_FAILED=0
DRY_RUN=0
LIMIT=0
ALL=0
CATEGORY=""
FINALIZE=1
SCOPE_LABEL=""
FEEDS=()

append_csv_feeds() {
    local raw="$1"
    local item
    while [[ "$raw" == *,* ]]; do
        item="${raw%%,*}"
        FEEDS+=("$item")
        raw="${raw#*,}"
    done
    FEEDS+=("$raw")
}

join_csv() {
    local out=""
    local item
    for item in "$@"; do
        if [[ -n "$out" ]]; then
            out+=","
        fi
        out+="$item"
    done
    printf '%s' "$out"
}

usage() {
    sed -n '2,30p' "$0"
    exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -j)               SLOTS="$2"; shift 2 ;;
        --feeds)          append_csv_feeds "$2"; shift 2 ;;
        --category)       CATEGORY="$2"; shift 2 ;;
        --all)            ALL=1; shift ;;
        --unenriched)     UNENRICHED=1; shift ;;
        --retry-failed)   RETRY_FAILED=1; shift ;;
        --limit)          LIMIT="$2"; shift 2 ;;
        --scope)          SCOPE_LABEL="$2"; shift 2 ;;
        --no-finalize)    FINALIZE=0; shift ;;
        --dry-run)        DRY_RUN=1; shift ;;
        -h|--help)        usage 0 ;;
        -*)               echo "unknown flag: $1" >&2; usage 2 ;;
        *)                FEEDS+=("$1"); shift ;;
    esac
done

# Was the most recent run clean? (schema_valid && no denylist/IPs && content present)
is_run_clean() {
    local report="$1"
    [[ -f "$report" ]] || return 1
    jq -e '
        .schema_valid == true
        and (.summary.denylist_violations_count // 1) == 0
        and (.summary.ip_address_findings_count // 1) == 0
        and (.summary.evidence_count // 0) >= 3
        and (.summary.roles_count // 0) >= 1
    ' "$report" > /dev/null 2>&1
}

eligible_source_feed() {
    local url="$1"
    local maintainer="$2"
    [[ "$url" != internal://* ]] || return 1
    [[ "$maintainer" != "FireHOL" ]] || return 1
    return 0
}

queue_catalog_scope() {
    local wanted_category="$1"
    while IFS=$'\t' read -r feed _yaml_path url maintainer category; do
        if [[ -n "$wanted_category" ]] && [[ "$category" != "$wanted_category" ]]; then
            continue
        fi
        if eligible_source_feed "$url" "$maintainer"; then
            FEEDS+=("$feed")
        fi
    done < <(agents/locate-feed.py --all-with-meta | sort)
}

if [[ $ALL -eq 1 ]]; then
    queue_catalog_scope ""
fi

if [[ -n "$CATEGORY" ]]; then
    queue_catalog_scope "$CATEGORY"
fi

# Auto-discover unenriched / failed feeds.
#
# We enumerate REAL feed names from the YAML `sources:` keys, not file
# stems — multi-source YAMLs (e.g. critical_provider_ranges.yaml) declare
# several sub-feeds under one filename, none named after the file itself.
# agents/locate-feed.py --all-with-meta yields one row per feed with the
# url/maintainer/category we need to filter out internal:// and FireHOL-
# maintained feeds (which have their own static enrichment path).
if [[ $UNENRICHED -eq 1 ]]; then
    while IFS=$'\t' read -r feed _yaml_path url maintainer _category; do
        # Skip internal:// URLs (locally-generated derivatives) and
        # FireHOL-maintained feeds (handled by tools/build-firehol-static-enrichment.py;
        # the wrapper also refuses these).
        if ! eligible_source_feed "$url" "$maintainer"; then
            continue
        fi
        latest_dir=$(find ".local/agents/feed-enrichment/${feed}" -mindepth 1 -maxdepth 1 -type d -printf '%p/\n' 2>/dev/null | sort | tail -1 || true)
        if [[ -z "$latest_dir" ]]; then
            FEEDS+=("$feed")
            continue
        fi
        report="${latest_dir}validation-report.json"
        if ! is_run_clean "$report"; then
            if [[ $RETRY_FAILED -eq 1 ]] || [[ ! -f "$report" ]]; then
                FEEDS+=("$feed")
            fi
        fi
    done < <(agents/locate-feed.py --all-with-meta | sort)
fi

# Normalize feed names and drop duplicates after all selectors have contributed.
if [[ ${#FEEDS[@]} -gt 0 ]]; then
    mapfile -t FEEDS < <(printf '%s\n' "${FEEDS[@]}" | sed '/^[[:space:]]*$/d' | sort -u)
fi

# Drain stdin if it's a pipe and we still have nothing
if [[ ${#FEEDS[@]} -eq 0 ]] && [[ ! -t 0 ]]; then
    while IFS= read -r line; do
        [[ -n "$line" ]] && FEEDS+=("$line")
    done
fi

if [[ ${#FEEDS[@]} -eq 0 ]]; then
    echo "no feeds queued. provide names as args, on stdin, or use --unenriched" >&2
    exit 2
fi

# Apply --limit (queue cap)
if [[ $LIMIT -gt 0 ]] && [[ ${#FEEDS[@]} -gt $LIMIT ]]; then
    FEEDS=("${FEEDS[@]:0:$LIMIT}")
fi

POOL_TS=$(date -u +%Y%m%dT%H%M%SZ)
POOL_DIR=".local/agents/feed-enrichment-pool/${POOL_TS}"
mkdir -p "$POOL_DIR"

echo "[pool] queue=${#FEEDS[@]} slots=${SLOTS} log=${POOL_DIR}"

if [[ $DRY_RUN -eq 1 ]]; then
    printf '  %s\n' "${FEEDS[@]}"
    exit 0
fi

# Worker function — must be exported so the bash subshells xargs spawns
# can see it. One worker handles one feed end-to-end, then exits; xargs
# starts a fresh worker for the next queued feed.
worker() {
    local feed="$1"
    local pool_dir="$2"
    local start end dur rc tag summary
    start=$(date +%s)
    echo "[pool] $(date -u +%H:%M:%SZ) START   ${feed}"
    set +e
    agents/run-enrichment.sh "$feed" > "${pool_dir}/${feed}.log" 2>&1
    rc=$?
    set -e
    end=$(date +%s); dur=$((end - start))
    case $rc in
        0)   tag="SUCCESS" ;;
        3)   tag="AGENTERR" ;;   # ai-agent non-zero (model gave up / infra)
        4)   tag="REFUSED"  ;;   # retention derivative / merge / internal://
        124) tag="TIMEOUT"  ;;   # wall-clock cap hit
        1)   tag="VALIDATE" ;;   # ai-agent succeeded but validator rejected
        *)   tag="FAILED${rc}" ;;
    esac
    # Tail the last run-enrichment status line for the at-a-glance view
    summary=$(grep -hE "^\[run-enrichment\] (result|SUCCESS|FAILED|REFUSED)" \
        "${pool_dir}/${feed}.log" 2>/dev/null | tail -1 || true)
    echo "[pool] $(date -u +%H:%M:%SZ) ${tag} ${feed} (${dur}s) ${summary}"
    printf '%s\t%s\t%ss\trc=%d\n' "$feed" "$tag" "$dur" "$rc" >> "${pool_dir}/results.tsv"
}
export -f worker

# xargs -P SLOTS -n 1: each worker call gets exactly one feed; up to
# SLOTS workers run concurrently; as workers finish, xargs spawns new
# ones for the next queued feed. That is the streaming-slots model.
printf '%s\n' "${FEEDS[@]}" \
    | xargs -P "$SLOTS" -I {} bash -c "worker \"\$1\" \"\$2\"" _ {} "$POOL_DIR"

echo
echo "[pool] done. summary:"
if [[ -s "${POOL_DIR}/results.tsv" ]]; then
    sort "${POOL_DIR}/results.tsv" | column -t -s $'\t'
    echo
    echo "[pool] counts:"
    awk -F'\t' '{print $2}' "${POOL_DIR}/results.tsv" | sort | uniq -c | sort -rn
fi
echo
echo "[pool] per-feed logs: ${POOL_DIR}/"

if [[ $FINALIZE -eq 1 ]]; then
    mapfile -t SUCCESS_FEEDS < <(awk -F'\t' '$2 == "SUCCESS" {print $1}' "${POOL_DIR}/results.tsv" 2>/dev/null | sort -u)
    if [[ ${#SUCCESS_FEEDS[@]} -eq 0 ]]; then
        echo "[pool] no successful feeds to finalize"
    else
        if [[ -n "$SCOPE_LABEL" ]]; then
            scope="$SCOPE_LABEL"
        elif [[ -n "$CATEGORY" ]]; then
            scope="category-${CATEGORY}"
        elif [[ $ALL -eq 1 ]]; then
            scope="all"
        else
            scope="$(join_csv "${SUCCESS_FEEDS[@]}")"
        fi
        echo "[pool] finalizing ${#SUCCESS_FEEDS[@]} successful feed(s): ${scope}"
        python3 agents/enrichment-refresh.py \
            --feeds "$(join_csv "${SUCCESS_FEEDS[@]}")" \
            --scope "$scope" \
            --write \
            --branch \
            --commit \
            --open-pr
    fi
fi
