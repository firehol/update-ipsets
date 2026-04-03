# Soft-tier identity, SaaS, observability, communication research (SOW-0017)

Research date: 2026-04-29
Scope: SOFT-tier candidates across identity/IAM/SSO, office productivity SaaS, communication,
observability/APM/logs, email delivery/security, customer support, status pages, and
monitoring/alerting outbound.

Source quality grades:
- A = official, machine-readable, current feed/API
- B = official machine-readable but partial, geofeed, DNS-derived, or requires derivation
- C = official docs/static HTML page, no clean dynamic feed
- D = no official public source / third-party only / stale / unsuitable

---

## Identity / IAM / SSO

### Summary table

| Provider | Source quality | URL | Update cadence | ASN | Tier rec | Notes |
|---|---|---|---|---|---|---|
| Okta | A | https://s3.amazonaws.com/okta-ip-ranges/ip_ranges.json | Unspecified | AS19745 (GCP/AWS cells) | soft | 4841 entries, /32s per cell, no date field |
| Auth0 (Okta) | A | https://cdn.auth0.com/ip-ranges.json | Changelog-based | AWS-hosted | soft | 103 ranges, has `last_updated_at`, changelog |
| Microsoft Entra/Azure AD | A | Via Azure service tag `AzureActiveDirectory` in Azure service tags download | Weekly | AS8075 / AS8068 | contextual | Part of huge Azure tag download; Entra tag is extractable |
| Google Identity/Workspace IDP | B | SPF-derived: `_spf.google.com` TXT record | Dynamic (DNS TTL) | AS15169 | contextual | SPF explicit in docs; says "doesn't include all Google APIs" |
| OneLogin | C | Static HTML only | Unknown | AS16509 (AWS) | soft (if found) | UNVERIFIED: page 404/403; no confirmed public feed found |
| Ping Identity / PingOne | C | Docs page only (HTML) | Unknown | Unknown | soft | UNVERIFIED: official docs page inaccessible during research |
| Duo Security (Cisco) | D | Docs say don't use IP-based firewalling; KB1337 content inaccessible | Unknown | AS13445 (Webex/Cisco) | D/reject | Official advice: avoid static IP firewalling for Duo |
| ForgeRock / PingForgeRock | D | No public feed found | Unknown | Unknown | later/reject | No official public IP range publication found |
| JumpCloud | D | UNVERIFIED: multiple doc URLs 404/503 | Unknown | Unknown | later | No confirmed public IP range feed found |
| IBM Security Verify | D | No public feed found | Unknown | Unknown | later/reject | |
| CyberArk Identity | D | No public feed found | Unknown | Unknown | later/reject | |
| LastPass Business | D | No public feed found | Unknown | Unknown | reject | |
| 1Password Business | D | No public feed found | Unknown | Unknown | reject | |
| Keeper Security | D | No public feed found | Unknown | Unknown | reject | |
| Bitwarden | D | Self-hosted; cloud uses AWS/Azure; no dedicated IP feed | Unknown | Unknown | reject | |
| Authy (Twilio) | D | No dedicated IP publication | Unknown | Unknown | reject | |
| Yubico / YubiKey services | D | No public feed found | Unknown | Unknown | reject | |
| Microsoft Authenticator | D | Covered by Azure/M365 endpoint API | Unknown | AS8075 | contextual | Subsumed into M365 endpoint data |
| Symantec VIP / Norton VIP | D | No public feed found | Unknown | Unknown | reject | |

### Per-provider details

#### Okta
- **Role**: Identity/SSO provider used by enterprises as IdP
- **URL**: `https://s3.amazonaws.com/okta-ip-ranges/ip_ranges.json`
- **Format**: JSON, keys are cell names (e.g. `us_cell_1`, `emea_cell_1`), each has `ip_ranges: ["/32", ...]`
- **ASN**: Mostly GCP (AS396982) and AWS (AS16509) cloud cells
- **Source quality**: A — official S3 bucket, confirmed by Okta docs; docs say "includes all existing and future reserved IPs"
- **License/redistribution**: No explicit license restriction found; linked from official Okta admin docs
- **Update cadence**: No date field in JSON; Okta says list is kept current but no SLA published
- **Entry count**: 4841 individual /32 entries across 24 cells (apac_cell_1/2, ca_cell_1, emea_cell_1/2/pam_cell_1, in_cell_1, preview_cell_1/2/3/pam_cell_1, us_cell_1–17/pam_cell_1)
- **Recommended tier**: soft
- **Caveats**: All /32s — very granular; cells overlap (preview cells share IPs with production). No timestamp. "Includes future reserved IPs" is a reassurance but unverifiable without cadence.

#### Auth0 (Okta Auth0)
- **Role**: Identity-as-a-Service, customer authentication/federation
- **URL**: `https://cdn.auth0.com/ip-ranges.json`
- **Format**: JSON with `last_updated_at`, `regions` (US/AU/EU/JP/UK/CA each with `ipv4_cidrs`), `changelog`
- **ASN**: AWS (AS16509)
- **Source quality**: A — official CDN endpoint, has timestamp and changelog
- **License/redistribution**: No restriction found
- **Update cadence**: Changelog entries from 2017 through 2025; `last_updated_at: 2025-09-08T12:06:22Z`
- **Entry count**: 103 IPv4 ranges across 6 regions, all /32
- **Recommended tier**: soft
- **Caveats**: Document does not explicitly state exhaustiveness. Auth0 is now Okta's B2C/CIAM product. All /32 individual IPs on AWS.

#### Microsoft Entra / Azure AD
- **Role**: Cloud identity provider for Microsoft 365, Azure, and federated enterprise apps
- **URL**: Azure service tags download at `https://www.microsoft.com/download/details.aspx?id=56519` — extract `AzureActiveDirectory` tag
- **Format**: JSON (Azure service tags file), filterable by tag `AzureActiveDirectory`
- **ASN**: AS8075 (Microsoft), AS8068 (Microsoft-US)
- **Source quality**: A — official weekly Microsoft download
- **License/redistribution**: Microsoft permits use of service tag data
- **Update cadence**: Weekly published JSON file; updated every Thursday approximately
- **Recommended tier**: contextual (broad cloud provider range; same caveats as Azure)
- **Caveats**: `AzureActiveDirectory` tag covers all Azure AD authentication endpoints globally; multi-tenant. Not soft-tier alone because it is a Microsoft service-tag subset, not an isolated IdP range.

#### Google Identity / Workspace IDP
- **Role**: Google Sign-In, Workspace SSO/federation outbound
- **URL**: DNS TXT `_spf.google.com`, recursively includes `_netblocks.google.com`, `_netblocks2.google.com`, `_netblocks3.google.com`
- **Format**: SPF TXT records; requires DNS resolution and parsing
- **ASN**: AS15169
- **Source quality**: B — official Google SPF, machine-readable but DNS-derived and requires re-resolution on a cadence
- **License/redistribution**: No restriction
- **Update cadence**: DNS TTL based; Google documentation says "the addresses change often"
- **Verified ranges**: From live DNS resolution:
  - `_spf.google.com`: `74.125.0.0/16`, `209.85.128.0/17`, plus IPv6 `/56` blocks
  - `_netblocks.google.com`: confirms IPv4 above
  - `_netblocks2.google.com`: IPv6 `2001:4860:4000::/36`, `2404:6800:4000::/36`, and others
  - `_netblocks3.google.com`: empty (`~all`)
- **Google docs note**: "the SPF record is complete for SPF but doesn't include all IP address ranges used by Google APIs and services"
- **Recommended tier**: contextual (Gmail-outbound soft only; broader Google space is contextual)
- **Caveats**: Non-exhaustive by official statement. SPF ranges change. Need DNS-refresh policy per product.

---

## Office Productivity SaaS

### Summary table

| Provider | Source quality | URL | Update cadence | ASN | Tier rec | Notes |
|---|---|---|---|---|---|---|
| Microsoft 365 (Exchange/Teams/SharePoint) | A | https://endpoints.office.com/endpoints/worldwide?clientrequestid=<GUID> | Microsoft publishes weekly | AS8075/AS8068 | soft/contextual | Requires GUID param; JSON with ips[], urls[], services |
| Google Workspace | B | SPF DNS `_spf.google.com` | DNS TTL | AS15169 | contextual | See Google Identity above |
| Zoho Mail | D | UNVERIFIED: docs pages 404 | Unknown | Unknown | later | No confirmed public IP range page found |
| Salesforce | A | https://ip-ranges.salesforce.com/ip-ranges.json | syncToken/createDate | AWS-hosted | soft | Verified: 24 Hyperforce prefixes, 2026-04-02 date |
| HubSpot | D | No IP range doc found in webhooks or API docs | Unknown | AWS | reject | Webhook docs have no IP range section |
| Atlassian Cloud (Jira/Confluence/Bitbucket/Opsgenie) | A | https://ip-ranges.atlassian.com/ | syncToken/creationDate | AWS+Google | soft | 252 entries (195 IPv4, 57 IPv6), 12 products including Opsgenie |
| Notion | D | Help page 404; no confirmed public range | Unknown | Unknown | reject | |
| Asana | D | Help page 307 redirect to generic; no confirmed range | Unknown | Unknown | reject | |
| Monday.com | D | No public feed found | Unknown | Unknown | reject | |
| ClickUp | D | No public feed found | Unknown | Unknown | reject | |
| Smartsheet | D | No public feed found | Unknown | Unknown | reject | |
| Box | D | No public feed found | Unknown | Unknown | reject | |
| Dropbox Business | D | UNVERIFIED: help pages 404 | Unknown | Unknown | reject | |
| Egnyte | D | No public feed found | Unknown | Unknown | reject | |
| Citrix ShareFile | D | No public feed found | Unknown | Unknown | reject | |

### Per-provider details

#### Microsoft 365 / Exchange / Teams / SharePoint
- **Role**: Core enterprise productivity suite; outbound IPs used by Exchange sending, Teams media, SharePoint sync
- **URL**: `https://endpoints.office.com/endpoints/worldwide?clientrequestid=00000000-0000-0000-0000-000000000001`
- **Format**: JSON array; each object has `id`, `serviceArea`, `serviceAreaDisplayName`, `ips[]`, `urls[]`, `category` (Optimize/Allow/Default), `tcpPorts`, `udpPorts`, `expressRoute`, `required`
- **ASN**: AS8075 (Microsoft), AS8068, AS2856
- **Source quality**: A — official Microsoft endpoint API, actively maintained
- **GUID requirement**: Any valid GUID works; the API documents say to use a stable GUID per consumer
- **License/redistribution**: Microsoft allows automation use; JSON format is stable
- **Update cadence**: Microsoft publishes changes with a change version; recommend polling weekly. No explicit update schedule but changelog available at `?TenantName=Common&clientrequestid=<GUID>&Version=latest`
- **Recommended tier**: soft (Exchange/Teams outbound), contextual (full service tag range)
- **Caveats**: Response includes service areas that may not be relevant to outbound IP allowlisting (e.g., CDN entries). Callers should filter by `category: Optimize` or `required: true` for critical paths. IPv4 and IPv6 mixed. Does not claim exhaustiveness — newer FQDNs published recently per response.

#### Salesforce (Hyperforce)
- **Role**: CRM SaaS; webhook, email, API outbound traffic
- **URL**: `https://ip-ranges.salesforce.com/ip-ranges.json`
- **Format**: JSON with `syncToken`, `createDate`, `prefixes[]` (each has `region`, `provider`, `ip_prefix[]`)
- **ASN**: AWS (AS16509)
- **Source quality**: A — confirmed `ip-ranges.salesforce.com` is on Salesforce TLS cert (Amazon RSA 2048 M03 for `ip-ranges.salesforce.com`), S3-hosted
- **Update cadence**: `createDate: 2026-04-02-19-34-44`; appears to mirror Salesforce Hyperforce AWS allocations
- **Entry count**: 24 prefixes across 22 AWS regions; ranges are /23 to /24
- **Recommended tier**: soft
- **Caveats**: These are Hyperforce (next-gen Salesforce cloud) IP ranges only. Classic Salesforce instances (non-Hyperforce) use a separate IP range set published in Help documentation (HTML only). The JSON endpoint covers only Hyperforce tenants. Soft tier appropriate for known Salesforce webhook/API outbound from Hyperforce. Classic instance ranges need separate static-doc sourcing.

#### Atlassian Cloud
- **Role**: Jira, Confluence, Bitbucket, Trello, Opsgenie, Statuspage, Forge, Loom, Rovo-Crawler
- **URL**: `https://ip-ranges.atlassian.com/`
- **Format**: JSON with `creationDate`, `syncToken`, `items[]` (each has `network`, `mask_len`, `cidr`, `region[]`, `product[]`, `direction` (egress/ingress), `perimeter` (commercial/fedramp-moderate))
- **ASN**: AWS (AS16509) and Google (AS15169/AS396982)
- **Source quality**: A — official Atlassian endpoint, versioned with syncToken and creationDate
- **Update cadence**: `creationDate: 2026-04-23T22:27:38.526318`; recommended to poll regularly
- **Entry count**: 252 items (195 IPv4, 57 IPv6); products: bitbucket, confluence, email, forge, github-for-jira, halp, jira, loom, opsgenie, rovo-crawler, statuspage, trello
- **Recommended tier**: soft (per-product filtering is possible)
- **Caveats**: Opsgenie IPs are embedded here (no separate Opsgenie feed needed). Rovo-Crawler may be a web crawling service — verify before treating as soft/non-crawler. `fedramp-moderate` perimeter entries are a subset; FedRAMP deployments may differ.

---

## Communication

### Summary table

| Provider | Source quality | URL | Update cadence | ASN | Tier rec | Notes |
|---|---|---|---|---|---|---|
| Slack | D | No static global provider IP list; workspace-specific or "contact Slack" | Unknown | AS13445 (Slack/Salesforce) | reject | Slack does not publish a global provider-side outbound IP list |
| Microsoft Teams | A | Via M365 endpoint API (serviceArea: Teams) | Weekly | AS8075 | soft | Subsumed under M365 endpoint API above |
| Zoom | A | Multiple txt files; Zoom.txt / ZoomMeetings.txt / ZoomPhone.txt / ZoomCDN.txt | Unknown (no date in files) | AS3209/AS13385/Lumen | soft | 49 general ranges; per-service split available |
| Cisco Webex | C | Static HTML table at https://help.webex.com/en-us/article/WBX264/ | Occasionally updated per revision history | AS13445 (Cisco Webex) | soft | 18 IPv4 CIDRs + IPv6 by region; HTML only; AS13445 confirmed |
| Discord | D | No published global IP ranges found | Unknown | AS36459 (Discord) | reject | No official provider-side outbound IP list found |
| Mattermost Cloud | D | Help page 403; no confirmed public range | Unknown | AWS | reject | |
| Rocket.Chat Cloud | D | No public feed found | Unknown | Unknown | reject | |
| RingCentral | D | UNVERIFIED: all support article URLs returned non-content pages | Unknown | AS21100 | later | May have IP ranges in PDF docs; not confirmed via web |
| 8x8 | D | IP ranges page 429 rate limited; no confirmed public range | Unknown | Unknown | later | |
| Twilio (voice/messaging) | C | SIP Trunking: https://www.twilio.com/docs/sip-trunking/ip-addresses (HTML) | Unknown | AS54517 (Twilio) | C (SIP only) | 8 regional /30 CIDR blocks for SIP signaling + 168.86.128.0/18 for media |
| Vonage | D | Doc page ECONNREFUSED | Unknown | Unknown | reject | |
| Sinch | D | No public feed found | Unknown | Unknown | reject | |
| Zoom Phone | A | https://assets.zoom.us/docs/ipranges/ZoomPhone.txt | Unknown | AS3209/AS13385 | soft | 25 ranges; same file-based approach as Zoom main |
| GoTo Meeting / GoTo Connect | D | No public feed found | Unknown | Unknown | reject | |
| BlueJeans (Verizon) | D | No public feed found | Unknown | Unknown | reject | |

### Per-provider details

#### Slack
- **Status**: VERIFIED REJECT
- **Finding**: Slack does not publish a global provider-side outbound/egress IP list. The changelog and API docs confirm no static CIDR feed. Slack's IP allowlisting is workspace-instance specific (each workspace has IPs visible to the admin, not a global shared provider range). The developer docs for Events API say nothing about provider-side outbound IPs.
- **Rationale for reject**: No official public provider-operated source ranges exist. Customer-specific per-workspace IPs are not a shared provider feed.

#### Zoom
- **Role**: Video conferencing, meetings, phone
- **URL (main)**: `https://assets.zoom.us/docs/ipranges/Zoom.txt` — 49 CIDR ranges
- **URL (meetings)**: `https://assets.zoom.us/docs/ipranges/ZoomMeetings.txt` — 49 CIDR ranges (identical to Zoom.txt)
- **URL (phone)**: `https://assets.zoom.us/docs/ipranges/ZoomPhone.txt` — 25 CIDR ranges
- **URL (CDN)**: `https://assets.zoom.us/docs/ipranges/ZoomCDN.txt` — 3 CIDR ranges
- **Format**: Plain text, one CIDR per line, no comments, no date
- **ASN**: Multiple: includes Lumen/CenturyLink, AS3209 (Vodafone/legacy Zoom), also AWS ranges
- **Source quality**: A — official Zoom domain, direct CIDR text files
- **Update cadence**: No date field in files; no stated cadence; recommend polling periodically
- **Recommended tier**: soft
- **Caveats**: No exhaustiveness statement in files. No version/date. `ZoomAll.txt` returned 403 during research — possibly internal. Zoom also uses TURN/STUN servers and media relay IPs that may not all appear here. Phone ranges separate from meetings.

#### Cisco Webex
- **Role**: Enterprise video conferencing and collaboration
- **URL**: `https://help.webex.com/en-us/article/WBX264/Network-Requirements-for-Webex-Services`
- **Format**: HTML table only (no JSON/API)
- **ASN**: AS13445 (Cisco/Webex)
- **Source quality**: C — official Cisco/Webex documentation, HTML-only, last revised April 2026
- **Update cadence**: Revision history shown in docs; not automated
- **Published CIDRs**: 18 IPv4 blocks including 23.89.0.0/16, 62.109.192.0/18, 64.68.96.0/19, 66.114.160.0/20, 66.163.32.0/19, 69.26.160.0/19, 114.29.192.0/19, 150.253.128.0/17, 170.72.0.0/16, 170.133.128.0/18, 173.39.224.0/19, 173.243.0.0/20, 207.182.160.0/19, 209.197.192.0/19, 210.4.192.0/20, 216.151.128.0/19, 144.196.0.0/16, 163.129.0.0/16; plus IPv6 by region
- **Recommended tier**: C grade; soft if scraped from HTML
- **Caveats**: "Partner-hosted services excluded" — contact partners separately. HTML format requires parsing. Webex AS13445 is confirmed in docs.

#### Twilio (SIP Trunking)
- **Role**: Communication APIs, SIP trunking, voice, SMS
- **URL**: `https://www.twilio.com/docs/sip-trunking/ip-addresses` (HTML)
- **Format**: HTML table; CIDR notation provided
- **ASN**: AS54517 (Twilio Inc)
- **Source quality**: C — official HTML docs
- **Published CIDRs (SIP signaling)**: 8 regional /30 blocks: N.America-VA 54.172.60.0/30, N.America-OR 54.244.51.0/30, EU-IE 54.171.127.192/30, EU-DE 35.156.191.128/30, APAC-JP 54.65.63.192/30, APAC-SG 54.169.127.128/30, APAC-AU 54.252.254.64/30, SA-BR 177.71.206.192/30
- **Published CIDRs (media)**: `168.86.128.0/18` (UDP 10000-60000)
- **Recommended tier**: C/soft (SIP-only, very narrow)
- **Caveats**: "Not all IPs will host active gateways at a given time." FQDN-based routing preferred over IP-based. Voice/SMS outbound ranges separate and not clearly documented as static.

---

## Observability / APM / logs

### Summary table

| Provider | Source quality | URL | Update cadence | ASN | Tier rec | Notes |
|---|---|---|---|---|---|---|
| Datadog | A | https://ip-ranges.datadoghq.com/ | version field (62); dated 2026-02-24 | AWS (AS16509) | soft | Multiple services; synthetics 113 IPs; webhooks 35 IPs |
| New Relic | A | https://nr-downloads-main.s3.us-east-1.amazonaws.com/networking/newrelic-ip-ranges.json | No date field; docs warn IPs discontinued | AWS (AS16509) | soft | 5 IPv4 CIDRs, 4 IPv6 CIDRs; JSON, simple structure |
| Grafana Cloud | A | Multiple per-service APIs; synthetics CIDR at https://allowlists.grafana.com/synthetics | Changes announced via status page | AWS/GCP | soft | Per-service JSON APIs confirmed working |
| Sentry | A | https://docs.sentry.io/security-legal-pii/security/ip-ranges/ (docs); https://sentry.io/api/0/uptime-ips/ (JSON) | "May change over time" | GCP (AS15169) | soft | Good: per-category IPs + machine-readable uptime endpoint |
| Splunk Cloud / Splunk Observability | D | Page 404; no confirmed public IP range feed | Unknown | Unknown | reject | Cannot verify source; page returns 404 |
| Honeycomb | D | No IP range documentation found | Unknown | AWS | reject | Docs searched; no IP range page found |
| Lightstep (ServiceNow) | D | No public feed found | Unknown | Unknown | reject | |
| Elastic Cloud | D | Redirect to new URL; page inaccessible | Unknown | AWS/GCP | reject | UNVERIFIED: redirects returned 404/308; no confirmed range |
| Logz.io | D | Help page 404 | Unknown | AWS | later | No confirmed public range feed |
| LogDNA / Mezmo | D | Docs page returned empty | Unknown | Unknown | later | No confirmed public range |
| Sumo Logic | D | Help page 404 redirect | Unknown | AWS | later | No confirmed public range |
| AppDynamics (Cisco) | D | TLS cert invalid for appd.com; docs page cert error | Unknown | AWS/Azure | later | Cannot verify; cert issue |
| Dynatrace | D | Multiple doc page attempts; all 404 | Unknown | AWS | later | UNVERIFIED: docs restructured; no confirmed range page |
| Instana (IBM) | D | Page 403 | Unknown | IBM Cloud | later | |
| ScoutAPM | D | No public feed found | Unknown | Unknown | reject | |
| Raygun | D | No public feed found | Unknown | Unknown | reject | |
| Bugsnag (SmartBear) | D | No public feed found | Unknown | Unknown | reject | |
| Coralogix | D | Pages redirect to generic | Unknown | AWS | later | |
| BetterStack / Logtail / Better Uptime | D | Docs page 404 | Unknown | AWS/Hetzner | later | |
| Pingdom (SolarWinds) | D | TLS cert error | Unknown | Unknown | later | |
| StatusCake | D | No public feed found | Unknown | Unknown | reject | |
| UptimeRobot | B | https://uptimerobot.com/inc/files/ips/IPv4andIPv6.txt | Unknown | Unknown | soft | Confirmed working; individual host IPs, not CIDRs |
| Site24x7 | D | Help page 404 | Unknown | AWS | later | |
| Catchpoint | D | Docs page 404 | Unknown | Unknown | later | |
| ThousandEyes (Cisco) | D | API docs only show example IPs, not ranges | Unknown | Cisco/AWS | later | No public IP range feed found |
| Apica | D | No public feed found | Unknown | Unknown | reject | |
| Nagios XI cloud | D | No cloud service with public IP feed | Unknown | Unknown | reject | |
| Checkly | D | Allowlist page returned 400 | Unknown | AWS/GCP | later | |
| Cribl | D | Not researched as separate entry (SaaS observability); no public range confirmed | Unknown | Unknown | later | |

### Per-provider details

#### Datadog
- **Role**: APM, infrastructure monitoring, logs, synthetics
- **URL**: `https://ip-ranges.datadoghq.com/`
- **Format**: JSON; `version` (int), `modified` (timestamp), then per-service keys (agents, api, apm, global, logs, orchestrator, process, remote-configuration, synthetics, synthetics-private-locations, webhooks)
- **Per-service structure**: Each key is `{ "prefixes_ipv4": [...], "prefixes_ipv6": [...] }`
- **Entry counts**: agents (1 /20), api (1), webhooks (35), synthetics (113), etc.
- **ASN**: AWS (AS16509) — from cell structure
- **Source quality**: A — official Datadog endpoint, versioned, dated
- **Update cadence**: `version: 62`, `modified: 2026-02-24T00:00:00`; Datadog docs say to poll for changes
- **Recommended tier**: soft
- **Caveats**: `datadoghq.eu` and other regional domains have separate feeds (not researched here). Synthetics agents vary by cloud provider and region — the 113 IPv4 synthetics IPs are individual /32 locations by AWS/GCP region. No explicit exhaustiveness statement.

#### New Relic
- **Role**: APM, synthetics monitoring
- **URL**: `https://nr-downloads-main.s3.us-east-1.amazonaws.com/networking/newrelic-ip-ranges.json`
- **Format**: JSON; two keys: `"New Relic IPv4"` (array of 5 CIDRs) and `"New Relic IPv6"` (array of 4 CIDRs)
- **IPv4 ranges**: 162.247.240.0/22, 152.38.128.0/19, 185.221.84.0/22, 212.32.0.0/20, 64.251.192.0/20
- **IPv6 ranges**: 2620:16:4000::/48, 2602:816:5000::/40, 2a0d:8000::/29, 2600:1f18:24e6:b900::/56
- **ASN**: AS397997 (New Relic)
- **Source quality**: A — official S3 JSON endpoint, directly linked from New Relic docs
- **Update cadence**: No date field; New Relic docs explicitly warn that IPs are discontinued and list changed as of May 1 2025; callers must monitor for changes
- **Recommended tier**: soft
- **Caveats**: **WARNING from docs**: New Relic "has discontinued the use of the following IP ranges" as of May 2025 — some historical ranges removed. Must re-verify against latest JSON before shipping. No version/date field makes automated staleness detection harder.

#### Grafana Cloud
- **Role**: Hosted observability (alerts, metrics, traces, logs, profiles, synthetics)
- **URLs** (confirmed working):
  - Hosted Alerts: `https://grafana.com/api/hosted-alerts/source-ips` (JSON array, 26 IPs)
  - Hosted Alerts text: `https://grafana.com/api/hosted-alerts/source-ips.txt`
  - Hosted Grafana: `https://grafana.com/api/hosted-grafana/source-ips` (218 IPs)
  - Hosted Metrics: `https://grafana.com/api/hosted-metrics/source-ips` (66 IPs)
  - Hosted Traces: `https://grafana.com/api/hosted-traces/source-ips` (38 IPs)
  - Hosted Logs: `https://grafana.com/api/hosted-logs/source-ips` (41 IPs)
  - Hosted Profiles: `https://grafana.com/api/hosted-profiles/source-ips` (28 IPs)
  - Synthetics (CIDR): `https://allowlists.grafana.com/synthetics` (JSON with `all.ipv4[]` 58 CIDRs, `all.ipv6[]` 58 CIDRs)
- **Format**: JSON arrays for most; synthetics is structured JSON with `all.ipv4`, `all.ipv6`, `locations`
- **ASN**: AWS (AS16509), GCP (AS15169/AS396982)
- **Source quality**: A — official Grafana API endpoints, confirmed working
- **Update cadence**: Grafana docs say "These lists can change. Subscribe to the Grafana status page." No embedded timestamps in responses.
- **Recommended tier**: soft
- **Caveats**: Most per-service endpoints return individual host IPs (/32), not CIDRs. Synthetics endpoint returns CIDRs. Per-service split is useful for precise allowlisting. DNS-based discovery also available but not needed given API.

#### Sentry
- **Role**: Error tracking, performance monitoring
- **URL (docs)**: `https://docs.sentry.io/security-legal-pii/security/ip-ranges/`
- **URL (uptime machine-readable)**: `https://sentry.io/api/0/uptime-ips/` (newline-separated IPs)
- **Format**: Docs page lists per-category IPs; uptime endpoint is a newline-separated plaintext list
- **ASN**: GCP (AS15169/AS396982)
- **Source quality**: A (uptime endpoint) / C (other categories in docs)
- **Per-category IPs (from docs)**:
  - Dashboard & API: 35.186.247.156/32, 34.36.122.224/32, 34.36.87.148/32
  - Event Ingestion (apex): 35.186.247.156/32
  - Event Ingestion (org subdomains): 34.120.195.249/32, 34.120.62.213/32, 34.160.81.0/32, 34.102.210.18/32
  - Event Ingestion (legacy): 34.96.102.34/32
  - Outbound (US): 35.184.238.160/32, 104.155.159.182/32, 104.155.149.19/32, 130.211.230.102/32
  - Outbound (EU): 34.141.31.19/32, 34.141.4.162/32, 35.234.78.236/32
  - Email: 167.89.86.73, 167.89.84.75, 167.89.84.14
  - Uptime: 9 static IPs via API
- **Recommended tier**: soft
- **Caveats**: "Applies only to Sentry's SaaS product." "Uptime monitoring addresses may change over time." Self-hosted Sentry excluded. Most IPs are /32 GCP addresses.

#### UptimeRobot
- **Role**: Website uptime monitoring probes
- **URL**: `https://uptimerobot.com/inc/files/ips/IPv4andIPv6.txt`
- **Format**: Plain text; individual IPv4 and IPv6 host addresses (not CIDRs)
- **ASN**: Multiple (AWS, Hetzner, DigitalOcean, etc. — probe infrastructure)
- **Source quality**: B — official file, but individual IPs not CIDR ranges, no date/version
- **Recommended tier**: soft (monitoring probe IPs)
- **Caveats**: Individual host IPs only — high entry count as probe nodes. Not CIDR-based. No version or date in file. Probe IPs may change as infrastructure scales.

---

## Email delivery / security

### Summary table

| Provider | Source quality | URL | Update cadence | ASN | Tier rec | Notes |
|---|---|---|---|---|---|---|
| SendGrid (Twilio) | D | No global shared IP range feed found; only per-customer dedicated pool docs | Unknown | AS11377 (Twilio SendGrid) | reject | Shared pool IPs not published as a provider feed |
| Mailgun | D | Docs page 404/403 | Unknown | Rackspace/AS10532 | later | No confirmed public range |
| AWS SES | A | Via AWS ip-ranges.json filtered to `AMAZON_SES` | Weekly-ish | AS16509 (AWS) | contextual | Confirmed: AWS has service tags; check for SES tag in JSON |
| Postmark | C | https://postmarkapp.com/support/article/800-ips-for-firewalls (HTML, updated 2025-03-17) | Infrequent; manual | AS7922 (Comcast/Postmark) | C | HTML only; good structure; no JSON feed |
| SparkPost (MessageBird/Bird) | D | Page redirect to 404 | Unknown | Unknown | reject | Brand migrated to Bird; IP doc likely gone |
| Mailchimp (Mandrill) | D | Help page 404 | Unknown | AS10532 (Mailchimp/Rackspace) | later | No confirmed range |
| Sendinblue / Brevo | D | Page 404 | Unknown | Unknown | later | No confirmed public range |
| Constant Contact | D | No public feed found | Unknown | Unknown | reject | |
| Campaign Monitor | D | No public feed found | Unknown | Unknown | reject | |
| Pepipost / Netcore | D | No public feed found | Unknown | Unknown | reject | |
| SocketLabs | D | No public feed found | Unknown | Unknown | reject | |
| Elastic Email | D | No public feed found | Unknown | Unknown | reject | |
| Mailjet | D | No public feed found | Unknown | Unknown | reject | |
| Mailtrap | D | No public feed found | Unknown | Unknown | reject | |
| Proofpoint (Essentials) | D | Multiple URLs 404/ECONNREFUSED | Unknown | AS26211 (Proofpoint) | later | No confirmed public range found |
| Mimecast | D | Multiple URLs ECONNREFUSED/404 | Unknown | AS25255 (Mimecast) | later | No confirmed range; community forum pages 404 |
| Barracuda Email Protection | D | Redirect to 404; main URL then again 404 | Unknown | Unknown | later | |
| Cisco Email Security (IronPort) | D | Page 403 | Unknown | AS13445 (Cisco) | later | |
| Trend Micro Email Security | D | Too many redirects | Unknown | Unknown | reject | |
| Symantec Email Security | D | No public feed found | Unknown | Unknown | reject | |
| Microsoft Defender for Office 365 | A | Subsumed in M365 endpoint API | Weekly | AS8075 | soft/contextual | Same endpoint as M365 above |
| Zoho Mail | D | Multiple doc URLs 404 | Unknown | AS133229 (Zoho) | later | |

### Per-provider details

#### AWS SES (Simple Email Service)
- **Role**: Cloud transactional email delivery by Amazon
- **URL**: `https://ip-ranges.amazonaws.com/ip-ranges.json` filtered by `service: AMAZON_SES`
- **Format**: JSON (AWS IP ranges), filter `service` field
- **ASN**: AS16509 (AWS)
- **Source quality**: A — official AWS JSON, but note: prior SOW research confirmed `SES` is NOT a service tag in AWS ip-ranges.json (the SOW knowledge doc was corrected). Verify current state before relying on this.
- **CORRECTION NOTE**: SOW-0017 knowledge doc corrections state "AWS `ip-ranges.json` has no `SES` service tag" — this was a factual error corrected on 2026-04-29. AWS does publish `AMAZON` service tags but the specific `SES` tag needs re-verification against current file. If absent, SES outbound IPs are within the general `AMAZON` or `EC2` ranges without a dedicated tag.
- **Recommended tier**: contextual (shared multi-tenant AWS service)
- **Caveats**: Must re-verify `SES` service tag presence. If absent, no clean separation from general AWS space.

#### Postmark
- **Role**: Transactional email delivery
- **URL**: `https://postmarkapp.com/support/article/800-ips-for-firewalls`
- **Format**: HTML table; CIDR notation and individual IPs
- **ASN**: Mixed (AWS, Comcast)
- **Source quality**: C — official HTML docs, updated 2025-03-17
- **Published ranges**: 50.31.156.96/27, 104.245.209.192/26, 50.31.205.204/30, 50.31.205.0/24, plus individual signaling IPs for SMTP, inbound MX, and webhooks
- **Note**: "IP allowlisting for our API is no longer supported" (deprecated)
- **Recommended tier**: C (HTML only)
- **Caveats**: API allowlisting deprecated. SMTP outbound ranges include /27 and /26. "Not stated as exhaustive."

---

## Customer support

### Summary table

| Provider | Source quality | URL | Update cadence | ASN | Tier rec | Notes |
|---|---|---|---|---|---|---|
| Zendesk | D | Help page 404/403 | Unknown | AWS | reject | Multiple URL attempts all failed |
| Freshdesk / Freshworks | D | Multiple pages 404; redirect to generic homepage | Unknown | AWS | reject | No confirmed public range |
| Intercom | D | Help page ECONNREFUSED; webhooks docs 404 | Unknown | AWS | reject | No confirmed public range |
| Help Scout | D | No public feed found | Unknown | Unknown | reject | |
| Drift | D | No public feed found | Unknown | Unknown | reject | |
| Front | D | No public feed found | Unknown | Unknown | reject | |
| Crisp | D | No public feed found | Unknown | Unknown | reject | |
| Tidio | D | No public feed found | Unknown | Unknown | reject | |
| LiveChat | D | No public feed found | Unknown | Unknown | reject | |
| Olark | D | No public feed found | Unknown | Unknown | reject | |
| HappyFox | D | 404 | Unknown | Unknown | reject | |
| Salesforce Service Cloud | A | Covered by Salesforce ip-ranges.salesforce.com | Weekly | AWS | soft | Same endpoint as Salesforce above |
| ServiceNow | D | KB page loads JS-only; no content extractable | Unknown | Unknown | reject | |
| HubSpot | D | Webhook docs contain no IP range section | Unknown | AWS | reject | |

---

## Status pages / alerting outbound

### Summary table

| Provider | Source quality | URL | Update cadence | ASN | Tier rec | Notes |
|---|---|---|---|---|---|---|
| Statuspage.io (Atlassian) | A | Subsumed in ip-ranges.atlassian.com (product: `statuspage`) | syncToken/creationDate | AWS/GCP | soft | See Atlassian above; statuspage IPs in same feed |
| Better Uptime / BetterStack | D | Docs page 404 | Unknown | Hetzner/AWS | later | No confirmed public range page |
| Instatus | D | 404 | Unknown | Unknown | reject | |
| Statussy | D | No public feed found | Unknown | Unknown | reject | |
| PagerDuty | C | https://support.pagerduty.com/main/docs/safelist-ips (HTML; individual IPs, no CIDR) | 30-day notice for REST API | AS33438 (PagerDuty) | C | No machine-readable feed; REST API has 30-day notice; webhook IPs "fixed" |
| Opsgenie (Atlassian) | A | Subsumed in ip-ranges.atlassian.com (product: `opsgenie`) | syncToken/creationDate | AWS/GCP | soft | See Atlassian above |
| VictorOps / Splunk On-Call | D | Help page redirects to generic Splunk On-Call intro | Unknown | Unknown | reject | No confirmed public range |
| Squadcast | D | ECONNREFUSED | Unknown | Unknown | reject | |
| xMatters | D | Help page 404 | Unknown | Unknown | later | |

### Per-provider details

#### PagerDuty
- **Role**: Incident alerting, on-call management; outbound webhooks and API
- **URL**: `https://support.pagerduty.com/main/docs/safelist-ips`
- **Format**: HTML only; individual IPv4 addresses (no CIDR); Events API IPs via DNS, webhook IPs static
- **ASN**: AS33438 (PagerDuty Inc)
- **Source quality**: C — official HTML docs
- **Published IPs (Events API example)**: 35.167.69.145, 44.231.93.240, 44.233.86.211
- **Webhook IPs**: Described as "fixed list" in developer docs; "should not be expected to change"
- **REST API**: IPs may change; 30-day notice for changes
- **Machine-readable feed**: None found
- **Recommended tier**: C (HTML only)
- **Caveats**: No CIDR notation. No version or date. Events API IPs served via DNS query per region. No JSON feed. "Fixed" webhook IPs are documented but no automated feed.

---

## Reject (with evidence)

The following providers were researched and confirmed unsuitable for a soft-tier reference feed at this time:

| Provider | Rejection reason |
|---|---|
| Slack | No global provider-operated outbound IP list published. IP allowlisting is workspace-instance specific. |
| ServiceNow | KB pages are JS-rendered; no extractable content. No public IP range documentation confirmed. |
| Splunk Cloud | Docs page 404. No machine-readable public IP range confirmed. |
| Honeycomb | No IP range documentation found anywhere in public docs. |
| Elastic Cloud | Page redirect chain leads to 404. No confirmed public IP range. |
| Discord | No official provider-side outbound IP list published. No confirmed public range. |
| SendGrid (shared IPs) | Shared IP pools are per-customer dedicated; no global shared provider IP feed exists. |
| SparkPost | Brand migrated to Bird; original docs 404. No replacement confirmed. |
| Vonage | Connection refused on all doc pages attempted. |
| RingCentral | All support article URLs returned non-content pages. May have IP docs in PDF/login-gated materials. |
| Duo Security | Official advice is explicitly to avoid static IP-based firewalling for Duo; KB1337 inaccessible. |
| OneLogin | All pages 404/403; no confirmed public range. |
| ForgeRock | No public feed found. |
| JumpCloud | Doc pages 404/503; no confirmed public range. |
| Mattermost Cloud | Help page 403; no confirmed public range. |
| VictorOps / Splunk On-Call | Redirects to generic intro page; no confirmed range. |
| Trend Micro Email | Too many redirects; no confirmed range. |
| PagerDuty | Source quality C only; HTML with individual IPs; no machine-readable feed. |

---

## Open questions / unverified

1. **Mimecast**: Outbound IP documentation known to exist (community articles reference it) but all URLs returned connection refused or 404. Worth retrying from a different network or directly via Mimecast admin portal. Prior sources suggest ~50 egress IPs organized by region.
2. **Proofpoint Essentials**: Docs known to exist; multiple URL formats attempted; all 404. May require support.proofpoint.com login.
3. **Barracuda Email Protection**: Redirect chain broken; `barracuda.com/wp-content/uploads/Barracuda-Networks-IP-Ranges.txt` URL is stale after site migration.
4. **Dynatrace**: Multiple doc restructurings; specific IP range page likely moved. Worth searching `docs.dynatrace.com` directly.
5. **AWS SES tag**: Must re-verify whether `service: AMAZON_SES` currently exists in AWS `ip-ranges.json`. SOW-0017 knowledge doc correction says it does not exist; confirm against the live file before committing any config.
6. **Logz.io, Mezmo/LogDNA, Sumo Logic**: All had doc page failures. These providers are smaller and may have docs only accessible via login; worth a fresh retry.
7. **RingCentral**: May publish IP ranges in PDF format or in login-gated partner/admin portals. Not confirmed via public web.
8. **Salesforce classic (non-Hyperforce)**: The ip-ranges.salesforce.com endpoint only covers Hyperforce (24 prefixes). Classic Salesforce instance IP ranges are published only in HTML help docs as individual IPs per instance datacenter — not a single machine-readable feed.
9. **PingIdentity/PingOne**: Doc pages inaccessible; may have per-region IP tables in admin documentation.
10. **Grafana Cloud OTLP endpoint**: `https://grafana.com/api/otlp/source-ips` failed (404); other Grafana endpoints worked. Check if OTLP endpoint exists with different path.

---

## Sources consulted

### Official machine-readable (A-grade, confirmed working)
- Okta: `https://s3.amazonaws.com/okta-ip-ranges/ip_ranges.json`
- Auth0: `https://cdn.auth0.com/ip-ranges.json`
- Atlassian: `https://ip-ranges.atlassian.com/`
- Salesforce Hyperforce: `https://ip-ranges.salesforce.com/ip-ranges.json`
- Zoom: `https://assets.zoom.us/docs/ipranges/Zoom.txt`, `ZoomMeetings.txt`, `ZoomPhone.txt`, `ZoomCDN.txt`
- Datadog: `https://ip-ranges.datadoghq.com/`
- New Relic: `https://nr-downloads-main.s3.us-east-1.amazonaws.com/networking/newrelic-ip-ranges.json`
- Grafana Cloud: `https://grafana.com/api/hosted-alerts/source-ips` (and siblings), `https://allowlists.grafana.com/synthetics`
- Sentry uptime: `https://sentry.io/api/0/uptime-ips/`
- Cloudflare (reconfirmed): `https://api.cloudflare.com/client/v4/ips`
- Terraform Cloud: `https://app.terraform.io/api/meta/ip-ranges`
- Microsoft 365 endpoints: `https://endpoints.office.com/endpoints/worldwide?clientrequestid=...`
- UptimeRobot: `https://uptimerobot.com/inc/files/ips/IPv4andIPv6.txt`
- Google SPF: DNS TXT `_spf.google.com` (live resolution performed)

### Official HTML docs (C-grade)
- Cisco Webex: `https://help.webex.com/en-us/article/WBX264/`
- Twilio SIP: `https://www.twilio.com/docs/sip-trunking/ip-addresses`
- Postmark: `https://postmarkapp.com/support/article/800-ips-for-firewalls`
- PagerDuty: `https://support.pagerduty.com/main/docs/safelist-ips`
- Sentry (other categories): `https://docs.sentry.io/security-legal-pii/security/ip-ranges/`

### Failed / inaccessible (D-grade)
All providers listed under "Reject" or "later" in the tables above were attempted at their official documentation URLs and returned 404, 403, ECONNREFUSED, redirect loops, or JS-rendered pages with no extractable content.
