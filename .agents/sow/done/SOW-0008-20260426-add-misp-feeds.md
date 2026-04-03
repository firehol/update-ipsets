# SOW-0008 | 2026-04-26 | add-misp-feeds

## Status

completed
Implementation, validation, install, runtime empty-feed investigation, and
closeout are complete for Costa's approved scope.

## Requirements

Given MISP publishes multiple feed catalogs and warninglists, when this SOW is complete, then update-ipsets must include every MISP feed that is reasonable and valuable to track.

Given not every MISP feed is necessarily appropriate, when feeds are evaluated, then the decision must use evidence: source type, update cadence, content quality, value to users, license, stability, format compatibility, and operational cost.

Given each accepted feed needs to blend into this project, when it is added, then it must have the right category, maintainer metadata, license metadata, cadence, parser settings, and public description.

Given rejected or deferred feeds may be revisited later, when a MISP feed is not added, then the reason must be documented.

Given Costa suspects some empty feeds may be parser or logic errors, when the
installed service is inspected, then all currently empty public feeds must be
listed and each likely cause must be classified as expected upstream-empty,
download/unavailable, parser/config bug, or needs deeper investigation.

## Analysis

Initial sources to consult when work starts:

- Official MISP feed metadata and warninglist repositories.
- Current `configs/firehol/sources/**/misp_*.yaml` entries.
- Current parser support in `pkg/processor/`.
- `.agents/sow/specs/feeds.md`, `.agents/sow/specs/config.md`, and `.agents/sow/specs/downloader.md`.

Current known context:

- The prior release tracker requested "add all MISP feeds".
- Current config already contains multiple MISP warninglist-derived feeds.
- This requires judgement, not blind import.

Discovery evidence gathered on 2026-04-27:

- Official MISP default feed catalog:
  - Source: `https://raw.githubusercontent.com/MISP/MISP/2.5/app/files/feed-metadata/defaults.json`.
  - Current catalog has 96 entries: 37 `freetext`, 46 `csv`, and 13 `misp`
    source-format feeds.
  - The catalog itself changed recently: GitHub commit API for
    `app/files/feed-metadata/defaults.json` on branch `2.5` reports
    `2026-04-21` as the latest change, adding Rectifyq MISP feeds.
  - MISP's public feed page states that default feeds can be MISP standard
    format, CSV, or freetext, and that the default feeds are described by the
    JSON file above:
    `https://misp.github.io/misp-website/feeds/`.
- Official MISP warninglists:
  - Source: `https://github.com/MISP/misp-warninglists`.
  - Latest GitHub release observed through the GitHub API:
    `2026040500 - MISP Warning Lists Updated`, published
    `2026-04-05T19:25:35Z`.
  - Current repository HEAD inspected locally:
    `59c9859`, committed `2026-04-17T22:58:47+02:00`.
  - The repository currently has 123 `lists/*/list.json` warninglists.
  - 73 warninglists contain IP material.
  - 24 IP warninglists are already configured under
    `configs/firehol/sources/**/misp_*.yaml`.
  - 42 additional warninglists contain IPv4 material and are not currently
    configured.
  - IPv6-only warninglists exist (`public-dns-v6`, `vpn-ipv6`,
    `tenable-cloud-ipv6`, `umbrella-blockpage-v6`, plus special-use IPv6
    lists), but the current authored catalog is IPv4-only; adding IPv6 feed
    publication should be a separate design decision.
  - MISP warninglists are CC0 unless a specific source requests another
    license; the repository README documents this.

Already configured MISP warninglist-derived feeds:

- `anonymizers`: `misp_vpn`.
- `provider_infrastructure`: `misp_cloudflare`, `misp_telegram`,
  `misp_tenable_cloud`, `misp_zscaler`.
- `scanners`: `misp_alphastrike_research_scanners`,
  `misp_bufferover_scanners`, `misp_coalition_intel_scanners`,
  `misp_cyberresilience_scanners`, `misp_cypex_scanners`, `misp_f6_scanners`,
  `misp_internet_census_scanners`, `misp_intrinsec_scanners`,
  `misp_ipinfo_scanners`, `misp_ipip_scanners`, `misp_modat_scanners`,
  `misp_onyphe_scanners`, `misp_probethenet_scanners`,
  `misp_rapid7_scanners`, `misp_research_scanners`,
  `misp_shadowforce_scanners`, `misp_shadowserver_scanners`,
  `misp_shodan_scanners`, `misp_stretchoid_scanners`.

Recommended MISP warninglist additions:

- `provider_infrastructure`: `misp_akamai`, `misp_amazon_aws`, `misp_apple`,
  `misp_fastly`, `misp_github`, `misp_google_gcp`,
  `misp_google_gmail_sending_ips`, `misp_googlebot`,
  `misp_microsoft_azure`, `misp_microsoft_azure_china`,
  `misp_microsoft_azure_germany`, `misp_microsoft_azure_us_gov`,
  `misp_microsoft_office365_cn`, `misp_microsoft_office365_ip`,
  `misp_openai_gptbot`, `misp_ovh_cluster`, `misp_public_dns`,
  `misp_smtp_receiving_ips`, `misp_smtp_sending_ips`, `misp_stackpath`,
  `misp_umbrella_blockpage`.
- `scanners`: `misp_alphastrike_scanners`, `misp_censys_scanners`,
  `misp_check_host_net`, `misp_cybergreen_scanners`,
  `misp_modat_nt_scanners`, `misp_netsecscan_nt_scanners`,
  `misp_netsecscan_scanners`, `misp_onyphe_published_scanners`,
  `misp_palo_alto_cortex_xpanse`, `misp_shadowforce_published_scanners`,
  `misp_shadowserver_published_scanners`, `misp_shodan_published_scanners`,
  `misp_skipa_scanners`.
- `malware_infrastructure`: `misp_sinkholes`.

MISP warninglists recommended for skip/defer:

- Skip as duplicate/special-use coverage already exists locally:
  `multicast`, `rfc1918`, `rfc5735`, `rfc6598`.
- Defer because they are not clean IPv4 feed bodies:
  `microsoft-attack-simulator` and `public-dns-hostname` are hostname or
  substring warninglists with incidental IPv4 values.
- Defer pending policy classification: `parking-domain`.
- Defer IPv6-only warninglists until the project deliberately adds IPv6 public
  feed catalog entries.

MISP default feed catalog recommendation:

- Do not import the MISP default feed catalog mechanically. It is a catalog of
  third-party feeds, not a single MISP-owned IP source.
- Treat third-party feeds in the MISP catalog as ordinary upstream candidates:
  add them only when they are accessible, machine-readable, redistributable
  enough for this project, not already covered by a better canonical upstream,
  and clearly map to this project's IP categories.
- Recommended direct additions from the MISP default catalog:
  - `dataplane_proto41` (`scanners`)
  - `dataplane_smtpdata` (`messaging_abuse`)
  - `dataplane_smtpgreet` (`messaging_abuse`)
  - `dataplane_telnetlogin` (`intrusion`)
  - `apnic_ssh_bruteforce` (`intrusion`)
  - `apnic_telnet_bruteforce` (`intrusion`)
  - `jamesbrine_bruteforce` (`intrusion`, large feed: observed
    `Content-Length` about 19 MB)
  - `threatview_c2` (`malware_infrastructure`)
  - `threatview_ip` (`intrusion`)
  - `serpro_reputation` (`policy_risk`, per Costa's approved decision)
- Default catalog entries recommended for skip/defer:
  - Already covered by better canonical direct feeds: Tor exits, Feodo,
    Blocklist.de, GreenSnow, CINS/CIArmy, CyberCure, current DataPlane feeds.
  - Not IP feeds: SSLBL certificate hashes, URLhaus URLs, MalwareBazaar hashes,
    domain-only feeds, URL-only feeds, Bitcoin/hash feeds.
  - Requires a new MISP event-feed parser or mixed IOC extraction design:
    CIRCL OSINT, DigitalSide, Botvrij MISP, Threatfox MISP, MalwareBazaar MISP,
    URLhaus MISP, Infoblox MISP, Rosti, NOCACTI, Rectifyq.
  - Paid/licensed or not appropriate without terms review: Bambenek feeds.
  - Not currently machine-readable/stable from the catalog URL:
    ELLIO returned a web landing page, Snort returned a terms page,
    Phishing.Database IP feed returned an HTML redirect, hideNseek returned
    HTTP 522 during checks.

## Implications and decisions

- This work must not add feeds mechanically.
- Feed category choices affect user trust and public presentation.
- Feed cadence choices affect daemon load and public freshness.
- Costa must approve the recommendation list before implementation.
- Recommended approval gate:
  - add the recommended MISP warninglist feeds now, because they are MISP-owned
    CC0 warninglists, raw JSON, and fit the existing `misp_*` config pattern
  - add the recommended direct third-party default-catalog feeds only if Costa
    accepts treating MISP's default catalog as discovery evidence rather than
    as MISP-owned source authority
  - defer MISP-format event feeds until a separate parser/extraction design is
    approved
- Costa decision on 2026-04-27:
  - Approved `1A`: add the recommended IPv4 MISP warninglists now.
  - Approved `2A`: add only the clean, accessible third-party IP feeds found
    through the MISP default catalog: missing DataPlane feeds, APNIC
    brute-force feeds, Threatview IP/C2, James Brine, and SERPRO as
    `policy_risk`.
  - Keep MISP-event feeds, IPv6-only warninglists, hostname/substr
    warninglists, special-use duplicates, inaccessible HTML/TOS redirects, and
    paid/licensed feeds deferred.

## Plan

Chunked SOW - reasoning: discovery, judgement, config changes, and validation are separable.

1. `discover-misp-catalogs` - medium risk
   - Identify official MISP feed catalogs, warninglists, metadata, formats, licenses, and update cadence.
2. `compare-current-config` - medium risk
   - Compare discovered feeds against configured `misp_*` feeds.
3. `recommend-feed-actions` - high risk
   - Propose add/skip/defer decisions with evidence and category recommendations.
4. `implement-approved-feeds` - medium risk
   - Add only approved feed configs and metadata.
5. `validate-feeds` - high risk
   - Run config, download, parse, history, and public metadata validation.

## Execution log

2026-04-27:

- Moved SOW-0008 to `current/` after Costa requested MISP work.
- Scope guard: discovery and recommendation happen before implementation
  because this SOW requires judgement. No MISP feed config is added until the
  accepted/deferred/rejected list is supported by evidence.
- Consulted official MISP default feed catalog, MISP default feed public page,
  MISP warninglists repository, latest warninglists release metadata, current
  local `misp_*` configs, processor support, category definitions, and catalog
  verification tests.
- Built a local discovery table from 123 warninglists and compared it to the
  24 already-configured MISP warninglist feeds.
- Sampled selected MISP default catalog URLs with HTTP HEAD/GET checks to
  distinguish raw IP feeds from HTML/TOS redirects, mixed IOC feeds, stale
  feeds, and MISP-event feeds.
- Costa approved recommendation `1A 2A`; implementation may now add the
  approved feed configs and matching catalog tests.
- Added 35 MISP warninglist-derived IPv4 feed fragments:
  - 21 `provider_infrastructure` feeds covering Akamai, AWS, Apple, Fastly,
    GitHub, GCP, Gmail, Googlebot, Azure, Office 365, GPTBot, OVH, public DNS,
    SMTP infrastructure, StackPath, and Cisco Umbrella blockpage ranges.
  - 13 `scanners` feeds covering Alpha Strike, Censys, check-host.net,
    Cybergreen, Modat, Netsecscan, ONYPHE, Palo Alto Cortex Xpanse,
    Shadowforce, Shadowserver, Shodan, and Skipa scanner ranges.
  - 1 `malware_infrastructure` feed for known sinkhole ranges.
- Added 10 direct third-party feeds discovered through the MISP default catalog:
  `dataplane_proto41`, `dataplane_smtpdata`, `dataplane_smtpgreet`,
  `dataplane_telnetlogin`, `apnic_ssh_bruteforce`,
  `apnic_telnet_bruteforce`, `jamesbrine_bruteforce`, `threatview_c2`,
  `threatview_ip`, and `serpro_reputation`.
- Updated catalog source-count and source-completeness tests in
  `pkg/config` and `pkg/processor` from 351 runtime sources to 396 runtime
  sources.
- Updated DataPlane redistribution validation to include the four newly added
  DataPlane feeds. DataPlane remains explicitly non-redistributable because
  every DataPlane feed header prohibits redistribution.
- Updated the opt-in legacy bash comparison to treat the new MISP/default
  catalog feeds as curated post-bash additions.
- Review: applied the `project-reviewing` checklist in-session. No subagents or
  external assistants were run because the active tool/user rules only allow
  them when Costa explicitly asks. Findings: the changes are data-catalog and
  test updates only; no runtime code paths, public request paths, admin paths,
  scheduler paths, or startup paths changed. Remaining risk is upstream feed
  quality/license uncertainty for feeds marked `license: unknown`; this is
  visible in metadata and documented here.
- Real-use validation details:
  - all 35 new MISP warninglist raw JSON URLs returned HTTP 200 on
    2026-04-27
  - direct feed parser smoke counts on 2026-04-27:
    `dataplane_proto41` 59313, `dataplane_smtpdata` 444,
    `dataplane_smtpgreet` 8503, `dataplane_telnetlogin` 48764,
    `apnic_ssh_bruteforce` 3568, `apnic_telnet_bruteforce` 1296,
    `jamesbrine_bruteforce` 422537, `threatview_c2` 1046,
    `threatview_ip` 17583, `serpro_reputation` 8527
  - Threatview IP currently contains 4 addresses in `0/8`; the feed is kept
    because Costa approved it, but the feed description explicitly warns that
    consumers should judge quality through comparison and bogon-overlap signals.
- Validation commands run:
  - `go test ./pkg/config -count=1`
  - `make test`
  - `make lint`
  - `make build`
- Additional validation commands after empty-feed fixes:
  - `go test ./pkg/scheduler -run 'TestScheduledDownloadWithProcessingWorkWakesProcessLoop|TestRunQueuedProcessingPromotesSuccessfulItemsAndRequeuesFailures' -count=1`
  - `go test ./pkg/processor -run TestLegacyMaxmindProxyFraudParser -count=1`
  - `make test`
  - `make lint`
  - `make build`
  - `./install.sh`
  - `curl http://localhost:18888/healthz`
  - live admin rechecks for `datacenters`, `uscert_hidden_cobra`, and
    `maxmind_proxy_fraud`
- Install validation run after Costa requested install:
  - `./install.sh` completed successfully on 2026-04-27
  - installed binary reports `update-ipsets dd007b8-dirty`
  - active config was backed up to
    `/opt/update-ipsets/etc/config.bak.20260427064503`
  - `/opt/update-ipsets/etc/config/` contains new feed fragments including
    `apnic_ssh_bruteforce.yaml`, `serpro_reputation.yaml`, and
    `misp_akamai.yaml`
  - `systemctl is-active update-ipsets` returned `active`
  - `systemctl is-enabled update-ipsets` returned `enabled`
  - `curl http://localhost:18888/healthz` returned `ok`
  - `curl http://localhost:18888/api/v1/status` reported
    `engine.source_count=396`
  - `curl http://localhost:18888/api/v1/sets` reported 376 public sets and
    included `apnic_ssh_bruteforce`
  - journal after restart shows configuration loaded with `sources=396`,
    integrity check passed, public listener started on `:18888`, and a
    health-transition entity refresh queued for 45 feeds
- Costa follow-up request on 2026-04-27: "I need you to check all the feeds
  that are empty. I think that some of them are parsing/logic errors." This is
  being handled as SOW-0008 validation/regression work against the installed
  runtime.
- Runtime empty-feed investigation found that the new MISP/default-catalog
  feeds were not HTTP failures and were not parser-empty:
  - `/api/v1/admin/status` reported `processing_enqueued=51` and
    `processing_batches_started=0`
  - new feeds were in `waiting_process` with `download_failures=0`
  - downloaded source files and parsed `*.ipset.new`/`*.netset.new` files were
    present and non-empty under `/opt/update-ipsets/data/`
  - committed public files (`*.ipset`/`*.netset`, `.setinfo`, public JSON)
    were not present for those feeds
- Root cause in scheduler validation: `runDownload()` only woke the processing
  loop when downloader-stage work was marked `Immediate`. Scheduled-due
  downloads could enqueue processing work but then wait for the periodic
  processing interval instead of publishing promptly after startup/download.
  Plan: wake the processing loop for every non-empty
  `decision.ProcessingNames` result and add a regression test for scheduled
  download work.
- Fixed the scheduler wake-up bug and added
  `TestScheduledDownloadWithProcessingWorkWakesProcessLoop`.
- After reinstall, all new `misp_*`, APNIC, DataPlane, James Brine, SERPRO,
  and Threatview feeds published non-empty public sets. Examples from
  `/api/v1/sets`: `misp_akamai` 222 entries / 13,967,104 IPs,
  `misp_public_dns` 60,057 entries / 62,745 IPs,
  `dataplane_proto41` 58,450 entries / 59,313 IPs,
  `apnic_ssh_bruteforce` 3,514 entries / 3,563 IPs,
  `serpro_reputation` 4,370 entries / 8,527 IPs.
- Forced downloader-stage recheck healed two older feeds that had valid raw
  sources but empty committed canonical bodies:
  - `datacenters`: 3,194 entries / 95,959,476 IPs
  - `uscert_hidden_cobra`: 621 entries / 627 IPs
- Fixed one real parser bug in `parse_maxmind_proxy_fraud`: MaxMind now emits
  links shaped like `/en/high-risk-ip-sample/<ip>` and splits the anchor text
  across lines. The parser now extracts IPs from the `high-risk-ip-sample/`
  link path and deduplicates them. Live recheck published
  `maxmind_proxy_fraud` with 1,281 entries / 1,282 IPs.
- Final empty-feed inventory after fixes and live rechecks:
  - Public empty feeds: 22.
  - Admin empty rows: 26, because admin also includes hidden/internal
    `anonymous`, `satellite`, `cleantalk_new`, and `cleantalk_updated`.
  - Remaining public empties by likely cause:
    - `blueliv_crimeserver_last_{1d,2d,7d,30d}`: downloader/API-key or
      upstream availability problem; current sources are zero-byte and the
      configured source requires `BLUELIV_API_KEY`.
    - `proxylists_{1d,7d,30d}`, `proxz_{1d,7d,30d}`, `xroxy_{1d,7d,30d}`:
      archived/downloader-failed historical proxy RSS feeds.
    - `zeus`, `zeus_badips`: upstream DNS failure for
      `zeustracker.abuse.ch`.
    - `botvrij_src`: upstream-empty accepted feed (`accept_empty: true`).
    - `griffinguard`: upstream moved the free feed and now returns a
      "File Moved" notice instead of IP data.
    - `ipblacklistcloud_recent{,_1d,_7d,_30d}`: upstream URL currently returns
      an OVH "site under construction" page, not IP data.
    - `spamhaus_edrop`: expected empty; upstream states the list has been
      merged into Spamhaus DROP and publishes only comments/EOF.
  - Remaining admin-only empties:
    - `anonymous` and `satellite`: hidden synthetic GeoIP buckets currently
      empty.
    - `cleantalk_new` and `cleantalk_updated`: hidden parent feeds currently
      HTTP 403; their public history derivatives still publish non-empty
      retained sets.
- Documentation/spec step:
  - no product-contract spec change was needed; the generic config/feed
    contracts already require per-feed fragments, metadata, provenance, and
    redistribution policy
  - updated `.agents/skills/project-testing/SKILL.md` with the catalog-count
    assertion lesson

## Validation

- [x] Acceptance criteria evidence - approved feeds added with category,
  maintainer, license, cadence, parser, provenance where appropriate, and info
  metadata; skipped/deferred feeds documented in Analysis.
- [x] Real-use validation evidence - URL and parser smoke checks recorded in
  the execution log; `make test`, `make lint`, `make build`, `./install.sh`,
  service health/status, and public set inventory checks passed.
- [x] Cross-model reviewer findings (logged + addressed) - external/subagent
  review was not run because active tool/user rules require Costa to request it
  explicitly; Costa authorized proceeding on 2026-04-27. In-session
  `project-reviewing` self-review is recorded above.
- [x] Lessons extracted (or "none, reasoning: ...")
- [x] Same-failure-at-other-scales check - source-count assertions were updated
  in all packages found by searching the old count; legacy bash comparison was
  updated for curated post-bash sources.

## Outcome

Completed and shipped locally. The work adds the approved MISP warninglist and
MISP-discovered direct feeds, fixes the scheduler wake-up issue that delayed
publication after scheduled downloads, fixes the MaxMind proxy-fraud parser,
classifies the remaining empty feeds, and installs successfully.

## Lessons extracted

- Catalog source-count assertions are duplicated outside
  `pkg/config/catalog_verify_test.go`; future feed inventory changes must
  search for the old count across `pkg/config`, `pkg/processor`, and other test
  packages. Captured in `.agents/skills/project-testing/SKILL.md`.
- Feed-catalog validation must prove that installed feeds publish committed
  public sets, not only that URLs download and parsers return rows. Captured in
  `.agents/skills/project-testing/SKILL.md`.
- Empty-feed investigations must separate public feeds from hidden/internal
  admin-only rows, and must classify each remaining empty by upstream-empty,
  downloader/unavailable, parser/config bug, or synthetic-source semantics.
  Captured in `.agents/skills/project-testing/SKILL.md`.
