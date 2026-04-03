# FireHOL Internal Configuration Terms - Research Result

## TL;DR

After thorough research across the FireHOL codebase and configuration, the 23 terms requested (**admin**, **base**, **cache**, **cidr**, **config**, **distribution**, **errors**, **gpf**, **history**, **ignore**, **ipset**, **ipsets**, **lib**, **local**, **max**, **min**, **parallel**, **push**, **rfc**, **rosi**, **run**, **skip**, **tmp**) are **NOT feed names**. They are internal runtime configuration variables, directory paths, and configuration keys.

---

## Findings

### What These Terms Actually Are

| Term | Type | Definition |
|------|-----|------------|
| **admin** | Runtime Variable | `ADMIN_SUPPLIED_IPSETS` — path to admin-provided ipsets directory (`${FIREHOL_CONFIG_DIR}/ipsets.d`) |
| **base** | Runtime Variable | `BASE_DIR` — base directory for all ipsets data (`${HOME}/ipsets` default) |
| **cache** | Runtime Variable | `CACHE_DIR` — path to cache directory (`${HOME}/.update-ipsets/cache`) |
| **cidr** | Config Key | `CIDR_REPORT_BOGONS` — is a **real feed** (see below) |
| **config** | Runtime Variable | `CONFIG_FILE` — path to config file (`${HOME}/.update-ipsets/update-ipsets.conf`) |
| **distribution** | Runtime Variable | `DISTRIBUTION_SUPPLIED_IPSETS` — path to distribution-provided ipsets (`${FIREHOL_SHARE_DIR}/ipsets.d`) |
| **errors** | Runtime Variable | `ERRORS_DIR` — path to errors directory (`${BASE_DIR}/errors`) |
| **gpf** | Feed Prefix | `GPF_COMICS` — is a **real feed** (see below) |
| **history** | Array Key | `history:` — array of retention periods for historical tracking (e.g., `[1440, 10080, 43200]` minutes) |
| **ignore** | Config Key | `ignore_repeating_download_errors` — number of consecutive errors before marking feed as failing |
| **ipset** | Config Keys | Multiple: `ipset_reduce_factor`, `ipset_reduce_entries`, `ipsets_apply` |
| **ipsets** | Runtime Variable | Related to ipset apply/save commands |
| **lib** | Runtime Variable | `LIB_DIR` — path to library directory for binary snapshots (`${HOME}/.update-ipsets/lib`) |
| **local** | Config Key | `local_copy_url` — URL prefix for local file serving |
| **max** | Config Keys | `max_connect_time`, `max_download_time`, `max_processing_workers` — parallelization limits |
| **min** | Config Key | `min_run_interval_seconds` — minimum seconds between scheduler runs (default 30) |
| **parallel** | Config Key | `parallel_dns_queries` — parallel DNS queries limit |
| **push** | Config Key | `push_to_git_merged` — whether to push merged sets to git |
| **rfc** | Feed Prefix | `RFC_RESERVED` — is a **real feed** (see below) |
| **rosi** | Renamed Prefix | `ROSI_*` feeds renamed to `RI_*` (see below) |
| **run** | Runtime Variable | `RUN_PARENT_DIR` — path to run parent directory |
| **skip** | Config Key | `skip_comparison_if_no_updates` — skip O(N²) comparisons when no updates occurred |
| **tmp** | Runtime Variable | `TMP_DIR` — temporary directory (`/tmp` default) |

---

## Real Feeds Found (Related to Requested Terms)

### CIDR-Related Feeds

| Feed Name | Maintainer | URL | License | Status |
|----------|-----------|-----|---------|--------|
| **cidr_report_bogons** | Team CYMRU | http://www.team-cymru.org/Services/Bogons/ | Unknown | Active |

### GPF-Related Feeds

| Feed Name | Maintainer | URL | License | Status |
|----------|-----------|-----|---------|--------|
| **gpf_comics** | GPF Comics | http://www.gpf-comics.info/ | Unknown | Active |

### RFC-Related Feeds

| Feed Name | Maintainer | URL | License | Status |
|----------|-----------|-----|---------|--------|
| **rfc_reserved** | IANA | https://www.iana.org/assignments/ipv4-address-space/ipv4-address-space.xhtml | Public Domain (IANA) | Active |

### ROSI-Related Feeds (Renamed)

The ROSI feeds have been renamed to RI prefix:

| Old Name | New Name | Status |
|---------|---------|--------|
| rosi_connect_proxies | ri_connect_proxies | Renamed |
| rosi_web_proxies | ri_web_proxies | Renamed |

See `renames:` section in `configs/firehol.yaml`.

---

## Runtime Directory Structure

FireHOL uses these directories (defined via runtime variables):

```
${BASE_DIR}/                          # Base directory
  ├── history/                      # Historical data (CSV files)
  ├── errors/                      # Error logs
  ├── web/                        # Published ipset files
  └── ...
${HOME}/.update-ipsets/
  ├── cache/                      # JSON cache
  ├── lib/                       # Binary snapshots
  └── run/                       # PID files, temporary state
${TMP_DIR:-/tmp}                  # Temporary files
${ADMIN_SUPPLIED_IPSETS}          # Admin-provided custom ipsets
${DISTRIBUTION_SUPPLIED_IPSETS}   # Distribution-provided ipsets
```

---

## Conclusion

None of the 23 requested terms are standalone feed names requiring license/research. They are:

- **20 runtime configuration variables** (paths, limits, toggles)
- **1 array key** (`history:` for retention periods)
- **3 legitimate feed prefixes that DO exist** (but with full names):
  - `cidr_report_bogons` → part of bogons category
  - `gpf_comics` → from gpf_comics feed
  - `rfc_reserved` → from rfc_reserved feed

The ROSI feeds have been renamed via the `renames:` configuration block.

---

## Sources Examined

1. `/home/costa/src/firehol/update-ipsets/configs/firehol.yaml` — Main configuration (2858 lines)
2. `/home/costa/src/firehol/update-ipsets/pkg/config/config.go` — Go config structs
3. `/home/costa/src/firehol/update-ipsets/pkg/config/legacy.go` — Legacy config loader
4. `/home/costa/src/firehol/blocklist-ipsets/` — Generated ipset files directory
5. Multiple grep searches across configs and Go packages

No additional feed definitions were found for any of the 23 requested terms beyond what is documented above.
