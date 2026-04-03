# Soft-tier developer platforms and supply chain research (SOW-0017)

Research date: 2026-04-29  
Scope: soft-tier developer platforms, supply chain infrastructure, and related CI/CD, container, package registry, and hosting providers.  
This file is READ-ONLY research. No config files were modified.

---

## Summary table

| Provider | Role | Source URL | Format | Source grade | ASN(s) | Recommended tier | Notes |
|---|---|---|---|---|---|---|---|
| GitHub Meta API | Git host, CI, packages, webhooks, Codespaces, Copilot | https://api.github.com/meta | JSON | A | AS36459 | soft | 13 service fields; docs say list "not exhaustive"; GHCR not in packages field |
| GitLab.com | Git host, webhooks, registry | https://docs.gitlab.com/ee/user/gitlab_com/ | HTML (static) | C | GCP AS15169 / AWS AS16509 for runners | soft | Web/API: 2 GCP ranges; runners: no static IPs; incoming: Cloudflare edge |
| Atlassian Cloud / Bitbucket | Git host, Jira, Confluence, Trello, OpsGenie | https://ip-ranges.atlassian.com/ | JSON | A | AWS-hosted | soft | Covers all Atlassian products; syncToken for versioning; FedRAMP subset also published |
| Azure DevOps | Git host (Azure Repos), CI (Pipelines) | Azure service tag `AzureDevOps` + static page | JSON (Azure tags weekly) + HTML | A/B | Microsoft AS8075 | soft | Outbound: 6 IPv4 /24 ranges; inbound per-region table; Pipelines agents use `AzureCloud.<region>` |
| Terraform Cloud / HCP Terraform | IaC CI/CD | https://app.terraform.io/api/meta/ip-ranges | JSON | A | AWS anycast (AS16509) | soft | 4 service groups: api, notifications, sentinel, vcs; all /32 |
| CircleCI | CI/CD | https://circleci.com/docs/ip-ranges-list.json + DNS: `jobs.knownips.circleci.com` | JSON + DNS A records | A/B | AWS AS16509 | soft | Jobs, core, macOS separate; DNS preferred over static; not all executors covered |
| Travis CI | CI/CD | DNS: `nat.travisci.net` (dnsjson.com) | DNS A records | B | GCP AS15169 | soft | IPs change; DNS query preferred; legacy service |
| Buildkite | CI/CD | no official public IP list found | — | D | unknown | reject | Docs 404; no machine-readable feed verified |
| Snyk | Security scanning CI | no official public IP list found | — | D | unknown | reject | Docs pages 404 or not found; no IP list verified |
| SonarCloud | Code quality CI | no official public IP list found | — | D | unknown | reject | Docs redirect broken; no IP list verified |
| Codecov | Code coverage CI | no official public IP list found | — | D | unknown | reject | Docs 404; no machine-readable feed verified |
| Bitrise | Mobile CI | no official public IP list found | — | D | unknown | reject | Help page ECONNREFUSED; no machine-readable feed verified |
| Microsoft Container Registry (MCR) | Container registry | Azure service tags `MicrosoftContainerRegistry` + `AzureFrontDoor.FirstParty` | JSON (Azure tags weekly) | A | Microsoft AS8075 | soft | Covered by Azure service tags; no standalone feed |
| AWS ECR / ECR Public | Container registry | AWS ip-ranges.json (no dedicated ECR service tag) | — | D | AWS AS16509 | contextual | No ECR service tag in ip-ranges.json; falls under broad `EC2`/`AMAZON` |
| Google Container Registry (gcr.io) | Container registry | GCP cloud.json (no GCR-specific tag) | — | D | GCP AS15169 | contextual | No GCR-specific service tag; covered by broad GCP cloud ranges |
| Quay.io | Container registry | no official IP list found | — | D | Red Hat/IBM | reject | No official IP publication verified |
| DigitalOcean Container Registry | Container registry | no official IP list found | — | D | DO AS14061 | reject | No IP ranges publication found in docs |
| Cloudsmith | Container/package registry | no official IP list found | — | D | unknown | reject | No official IP publication verified |
| Docker Hub | Container registry | no official public IP list | — | D | various (CDN-fronted) | reject | No official IP list; see evidence below |
| GitHub Container Registry (GHCR) | Container registry | GitHub Meta API `packages` field (partial) | JSON | B | AS36459 | soft | GitHub docs say Packages IPs "not included" in Meta API; GHCR not separately documented |
| npm registry | Package registry | no official public IP list | — | D | Fastly-fronted | reject | CDN-fronted; no official IP list |
| PyPI | Package registry | no official public IP list | — | D | AWS+Fastly | reject | Fastly/AWS-fronted; no official IP list |
| RubyGems.org | Package registry | no official public IP list | — | D | AWS+Fastly | reject | Gems served from S3/Fastly; no official IP list |
| Maven Central / Sonatype | Package registry | no official public IP list | — | D | various | reject | No official IP list verified |
| NuGet.org | Package registry | no official public IP list | — | D | Azure-hosted | reject | Covered by general Azure; no dedicated NuGet IP list |
| crates.io | Package registry | no official public IP list | — | D | CloudFront-fronted | reject | CDN-fronted (CloudFront); no official IP list |
| Go module proxy (proxy.golang.org) | Package registry | no official public IP list | — | D | Google-hosted | reject | Google-hosted; no official IP list beyond GCP ranges |
| Hex.pm | Package registry | no official public IP list | — | D | unknown | reject | No official IP list verified |
| pub.dev | Package registry | no official public IP list | — | D | Google-hosted | reject | "Supported by Google"; no official IP list |
| Packagist | Package registry | https://packagist-org-network.s3-eu-west-1.amazonaws.com/ip-address-list | plain text | B | AWS (eu-west-1) | soft (later) | 8 individual IPs (4 IPv4, 4 IPv6); small list; not a clean CIDR feed |
| CocoaPods | Package registry | no official public IP list | — | D | unknown | reject | No official IP list verified |
| Conda / Anaconda | Package registry | no official public IP list | — | D | various | reject | No official IP list verified |
| ArtifactHub (Helm) | Package registry | no official public IP list | — | D | unknown | reject | No official IP list verified |
| Heroku | PaaS | Heroku docs page 404 | — | D | AWS AS16509 | reject | Docs 404 at time of research; no machine-readable feed verified |
| Render.com | PaaS | no static public IP list | — | D | various | reject | IPs per-service via dashboard, not a public static feed |
| Fly.io | PaaS | no official public IP list | — | D | various | reject | No official IP publication verified |
| Railway.app | PaaS | docs redirect 404 | — | D | various | reject | No official IP publication verified |
| Vercel hosting | PaaS/CDN | no official public IP list | — | D | various CDN | reject | 126 PoPs; no official static IP list published |
| Netlify hosting | PaaS/CDN | no official public IP list | — | D | various CDN | reject | Docs 404 at time of research; no machine-readable feed verified |
| Cloudflare Pages/Workers | PaaS/CDN | Cloudflare IP ranges (https://www.cloudflare.com/ips-v4) | text/JSON | A | AS13335 | soft (covered by Cloudflare edge) | Pages/Workers use same Cloudflare edge IPs as CDN |
| AWS App Runner | PaaS | AWS ip-ranges.json (no AppRunner service tag) | — | D | AWS AS16509 | contextual | No AppRunner service tag in ip-ranges.json |
| Google Cloud Run | PaaS | GCP cloud.json (no Cloud Run-specific tag) | — | D | GCP AS15169 | contextual | Covered by broad GCP cloud ranges only |
| AWS CodeBuild | CI/CD | AWS ip-ranges.json service tag `CODEBUILD` | JSON | A | AWS AS16509 | soft (later) | 60 prefixes across regions; could be useful for CI origin |
| Azure Pipelines (MS-hosted agents) | CI/CD | Azure weekly JSON (`AzureCloud.<region>`) | JSON | A/B | Microsoft AS8075 | soft | Must use all regions in geography; macOS uses GitHub Meta API `actions_macos` |
| SourceForge | Git host / dist | no official public IP list | — | D | various | reject | No official IP list verified |
| Codeberg | Git host | no official public IP list | — | D | unknown | reject | Hosted in Europe; no IP list verified |
| Gitea Cloud | Git host | no official public IP list | — | D | unknown | reject | No official IP list verified |
| AWS CodeCommit | Git host | no CODECOMMIT service tag in ip-ranges.json | — | D | AWS AS16509 | contextual | Not a separate service tag; falls under general AWS |
| Google Cloud Source Repositories | Git host | no official separate IP list | — | D | GCP AS15169 | contextual | Covered by GCP cloud ranges only |
| Sigstore / Fulcio / Rekor | Supply chain (code signing) | no official public IP list | — | D | various (Google/Linux Fdn) | later | No static IP feed published; hostname-based access |
| TUF / in-toto | Supply chain (attestation) | no official public IP list | — | D | various | later | Specification frameworks, not single IP endpoints |
| Read the Docs | Documentation hosting | no official public IP list | — | D | various | reject | No official IP list verified |
| GitBook | Documentation hosting | no official public IP list | — | D | various | reject | No official IP list verified |
| Linear | Issue tracker | no official public IP list | — | D | unknown | reject | No official IP list verified |
| Asana | Issue tracker / PM | no official public IP list | — | D | unknown | reject (or later) | No official IP list verified |
| ClickUp | Issue tracker / PM | no official public IP list | — | D | unknown | reject | No official IP list verified |

---

## Per-provider details

### GitHub Meta API

- **Role**: Git hosting, webhooks, web, API, Actions CI runners, Codespaces, Copilot, packages, Dependabot, Pages, GitHub Enterprise Importer.
- **Official source URL**: `https://api.github.com/meta`
- **Format**: JSON; no authentication required; no documented polling interval but GitHub advises monitoring regularly because IPs change.
- **Fields confirmed** (as of 2026-04-29):
  - `hooks` — webhook delivery
  - `web` — web interface
  - `api` — REST/GraphQL API
  - `git` — git protocol (push/pull)
  - `packages` — GitHub Packages
  - `pages` — GitHub Pages hosting
  - `importer` — repository import
  - `actions` — Linux/Windows runner IPs
  - `actions_macos` — macOS runner IPs (hosted in GitHub's macOS cloud; also used by Azure Pipelines macOS agents)
  - `codespaces` — Codespaces environment IPs
  - `dependabot` — Dependabot automation
  - `copilot` — GitHub Copilot
  - `github_enterprise_importer` — GHE migration tooling
  - `domains` — domain list (not IPs)
  - `ssh_key_fingerprints` and `ssh_keys` — not IPs
- **Exhaustiveness caveat**: Official docs state "The list of GitHub IP addresses returned by the Meta API is **not intended to be an exhaustive list**. IP addresses for some GitHub services might not be listed, such as LFS or GitHub Packages." GitHub explicitly advises against relying solely on IP-based allowlisting.
- **GHCR / GitHub Container Registry**: The `packages` field in the Meta API covers GitHub Packages but GitHub's docs confirm Packages/GHCR IPs may not be included. No separate GHCR IP feed is published.
- **ASNs**: GitHub is AS36459; macOS runners run in GitHub's macOS cloud (may be AS36459 or third-party infrastructure).
- **License / redistribution**: No explicit license on the data; these are factual infrastructure IP ranges with no stated redistribution restriction.
- **Update cadence**: Not specified; IPs change without a fixed interval; monitor the endpoint.
- **Recommended tier**: soft / `developer_platform`
- **Recommended fields to include**:
  - Include: `hooks`, `web`, `api`, `git`, `packages`, `pages`, `importer`, `actions`, `actions_macos`, `dependabot`
  - Include with caveat of expanded scope: `codespaces`, `copilot`, `github_enterprise_importer`
  - Label source as "not exhaustive" in all derived artifacts.

---

### GitLab.com

- **Role**: Git hosting, webhooks, repository mirroring, CI/CD (GitLab Runners), Container Registry, Pages.
- **Official source URL**: `https://docs.gitlab.com/ee/user/gitlab_com/` (HTML static page, not a machine-readable feed)
- **Format**: HTML static page.
- **Ranges published** (as of 2026-04-29):
  - **Web/API fleet (webhooks, repository mirroring)**: `34.74.90.64/28`, `34.74.226.0/24` — GCP-hosted, sole use by GitLab.
  - **Incoming connections (all traffic to gitlab.com)**: Cloudflare CIDR blocks — gitlab.com DNS resolves to `172.65.251.78` (AS13335 Cloudflare), confirming Cloudflare-fronted.
  - **CI/CD runners (outgoing)**: **No static IPs published.** GitLab states "We don't provide static IP addresses for outgoing connections from CI/CD runners." Runners are in GCP `us-east1`/`us-central1` (Linux) and AWS `us-east-1` (macOS).
  - **Container Registry**: `35.227.35.254` (registry), `34.149.22.116` (CDN).
- **ASNs**: Outbound web/API uses GCP AS15169 (those specific /28 and /24); incoming goes through AS13335 Cloudflare; runners overlap with broad GCP/AWS ranges.
- **Exhaustiveness caveat**: Runner IPs not published; mailing system uses Mailgun IPs that "are subject to change at any time."
- **License / redistribution**: No stated restriction.
- **Update cadence**: No automated feed; page updated by GitLab manually.
- **Source quality**: C (official static HTML page, not a machine-readable dynamic feed).
- **Previous config mistake**: AS35995 was incorrectly labelled as GitLab in the prior config. AS35995 is Twitter/X. This was corrected in the 2026-04-29 config cleanup. GitLab's outbound ranges come from GCP space, not AS35995.
- **Recommended tier**: soft / `developer_platform`
- **Caveats**: Runner IPs absent; static page only; Cloudflare-fronted for inbound (so incoming from internet hits Cloudflare first).

---

### Atlassian Cloud / Bitbucket Cloud

- **Role**: Jira, Confluence, Bitbucket (Git host), Trello, Halp, OpsGenie, StatusPage, Forge (dev platform).
- **Official source URL**: `https://ip-ranges.atlassian.com/`
- **Format**: JSON, includes `creationDate` and `syncToken` fields for versioning/change detection.
- **Coverage**: Covers all listed Atlassian Cloud products by category. Includes egress and ingress direction fields per range. Includes FedRAMP Moderate subset.
- **Ranges structure**:
  - Core collaboration (Bitbucket/Confluence/Jira/etc.): `104.192.136.0/21`, `185.166.140.0/22`, IPv6 ranges.
  - Email service: multiple /32 and /25 subnets.
  - FedRAMP subset: `44.220.40.160/28`, `18.246.188.32/28`.
  - Forge (developer platform): /32 IPs per region.
  - Rovo-Crawler: /28 subnets per region.
- **ASNs**: AWS-hosted (ranges fall in AWS space).
- **Update cadence**: `syncToken` is a Unix timestamp; check this value to detect updates.
- **License / redistribution**: No stated restriction; publicly accessible.
- **Source quality**: A (official machine-readable JSON feed with versioning).
- **Recommended tier**: soft / `developer_platform`
- **Notes**: Already present in MISP sources in local catalog; this is the authoritative upstream source.

---

### Azure DevOps (Repos + Pipelines)

- **Role**: Git hosting (Azure Repos), CI/CD (Azure Pipelines), webhooks, service hooks, artifact publishing.
- **Official source URL**: 
  - Azure service tag `AzureDevOps` in weekly JSON: `https://www.microsoft.com/download/details.aspx?id=56519`
  - Static docs page: `https://learn.microsoft.com/en-us/azure/devops/organizations/security/allow-list-ip-url`
- **Format**: JSON (Azure weekly service tags file) + HTML static ranges on docs page.
- **Ranges published** (outbound connections, port 443):
  - IPv4: `150.171.22.0/24`, `150.171.23.0/24`, `150.171.73.0/24`, `150.171.74.0/24`, `150.171.75.0/24`, `150.171.76.0/24`
  - IPv6: `2620:1ec:50::/48`, `2620:1ec:51::/48`, `2603:1061:10::/48`
  - Legacy: `13.107.6.183/32`, `13.107.9.183/32`
- **Inbound connections**: Per-region /24 ranges (see docs page); `AzureDevOps` service tag supported for inbound only.
- **Azure Pipelines Microsoft-hosted agents**: Use `AzureCloud.<region>` service tags from the same weekly JSON, not a dedicated `AzurePipelines` tag. macOS agents use GitHub Meta API `actions_macos` field (hosted in GitHub's macOS cloud).
- **Note on Service Tags**: The `AzureDevOps` service tag is only supported for *inbound* NSG rules within Azure; it cannot be used for outbound connection allowlisting from outside Azure.
- **ASNs**: Microsoft AS8075.
- **Source quality**: A (Azure weekly service tags JSON is official and machine-readable); B for the static HTML outbound ranges.
- **Recommended tier**: soft / `developer_platform`
- **Caveats**: Agent IPs are broad Azure geography (`AzureCloud.<region>`), not a tight AzureDevOps-specific set for Pipelines runners.

---

### Terraform Cloud / HCP Terraform

- **Role**: IaC CI/CD platform, plan/apply runners, VCS webhooks, notifications, Sentinel policy.
- **Official source URL**: `https://app.terraform.io/api/meta/ip-ranges`
- **Format**: JSON; four keys: `api`, `notifications`, `sentinel`, `vcs`.
- **Ranges** (as of 2026-04-29):
  - `api`: `75.2.98.97/32`, `99.83.150.238/32` (2 IPs — AWS Global Accelerator anycast)
  - `notifications`: 14 /32 IPs (AWS EC2)
  - `sentinel`: identical to notifications (14 IPs)
  - `vcs`: identical to notifications (14 IPs)
- **ASNs**: AWS AS16509 (EC2 us-east-1 / us-west-2 region IPs); API IPs use AWS Global Accelerator (anycast).
- **Update cadence**: No stated interval; poll the endpoint.
- **License / redistribution**: No stated restriction.
- **Source quality**: A (official machine-readable JSON endpoint).
- **Recommended tier**: soft / `developer_platform`
- **Notes**: The API endpoint uses AWS Global Accelerator anycast IPs (`75.2.0.0/15`, `99.83.128.0/17`), which are also used by other AWS customers; the /32 granularity here is very specific.

---

### CircleCI

- **Role**: CI/CD hosted runners (Linux, macOS), core services.
- **Official source URL**: 
  - JSON: `https://circleci.com/docs/ip-ranges-list.json`
  - DNS A records: `jobs.knownips.circleci.com`, `core.knownips.circleci.com`, `all.knownips.circleci.com`
- **Format**: JSON file + DNS A records; DNS is the preferred/authoritative method per CircleCI docs.
- **Ranges** (from JSON):
  - Jobs (Linux Docker executor): 20 individual /32 IPs on AWS EC2.
  - Core services: 10 individual /32 IPs.
  - macOS: separate /24 and /25 ranges (38.23.x, 100.27.x, 100.29.x, 98.80.x, 207.254.x, 18.97.x).
- **Important caveats**:
  - macOS IP ranges are **not** included in the DNS machine-consumable lists; requires the JSON file.
  - Only applies to Docker executor; Machine executor and `remote_docker` are not covered.
  - Docs last updated April 6, 2022 — freshness of the JSON should be verified before use.
  - "30 days notice" before changes, but this cannot be considered stable enough for a hard feed.
- **ASNs**: AWS AS16509 for Linux jobs; macOS ranges appear to be dedicated CircleCI infrastructure.
- **Source quality**: A for the DNS method; B for the JSON file (dated docs).
- **Recommended tier**: soft / `developer_platform`
- **Notes**: DNS-based source (`all.knownips.circleci.com`) is the closest to an authoritative live feed; the JSON file is a useful snapshot but requires freshness validation.

---

### Travis CI

- **Role**: CI/CD hosted runners (Linux, Windows, macOS).
- **Official source URL**: DNS: `nat.travisci.net`; JSON via third-party: `https://dnsjson.com/nat.travisci.net/A.json`
- **Format**: DNS A records (primary recommended method by Travis CI); dnsjson.com wraps in JSON.
- **Ranges**: ~50 individual IPs in GCP (Linux/Windows in `us-central1` and `us-east1`); distinct IPs for macOS.
- **Caveats**:
  - "These ranges can change in the future" — DNS is the only reliable method.
  - Travis CI has had significant business changes (acquired by Idera); service reliability concerns exist.
  - Not all executor types share IPs; macOS IPs differ from Linux.
  - IPs are "not guaranteed consistent per job" — may trigger security alerts.
- **ASNs**: GCP AS15169.
- **Source quality**: B (official DNS record but DNS is inherently dynamic; third-party JSON wrapper).
- **Recommended tier**: soft / `developer_platform` — but flag as low-priority / legacy service.
- **Notes**: Travis CI is in decline; many projects have migrated to GitHub Actions. Consider flagging as `later` pending service viability assessment.

---

### Microsoft Container Registry (MCR)

- **Role**: Container registry for Microsoft-published Docker images (Windows Server, .NET, Azure SDK images, etc.). Hostname: `mcr.microsoft.com`.
- **Official source URL**: Azure weekly service tags JSON (`https://www.microsoft.com/download/details.aspx?id=56519`), service tags `MicrosoftContainerRegistry` and `AzureFrontDoor.FirstParty`.
- **Format**: JSON (Azure weekly file).
- **Coverage**: MCR uses both `MicrosoftContainerRegistry` (REST/registry API) and `AzureFrontDoor.FirstParty` (CDN/data plane via Azure Front Door). Both service tags must be allowlisted per official MCR firewall rules.
- **ASNs**: Microsoft AS8075.
- **Source quality**: A (covered by Azure official service tags weekly JSON).
- **Recommended tier**: soft / `developer_platform`
- **Notes**: No standalone MCR-specific IP feed exists; use Azure service tags. MCR is the primary registry for all Microsoft-published images used in CI pipelines worldwide.

---

### AWS CodeBuild

- **Role**: CI/CD build service (managed build runners) within AWS.
- **Official source URL**: `https://ip-ranges.amazonaws.com/ip-ranges.json`, filter `service: CODEBUILD`.
- **Format**: JSON (AWS standard ip-ranges.json format).
- **Coverage**: 60 IPv4 prefixes across AWS regions, including China and GovCloud. No IPv6 CODEBUILD service tag found.
- **ASNs**: AWS AS16509.
- **Update cadence**: AWS updates the JSON file regularly; SNS notifications available.
- **Source quality**: A.
- **Recommended tier**: soft / `developer_platform` — include as `later` (lower priority than GitHub/GitLab/Atlassian; AWS CodeBuild is significant but less universally deployed).
- **Notes**: AWS does not publish service tags for CodeCommit, CodePipeline, CodeDeploy, CodeStar, or CodeArtifact; only CODEBUILD is present. ECR is not a separate service tag either.

---

### Packagist (PHP package registry)

- **Role**: Primary PHP package registry (packagist.org); Composer downloads packages from here.
- **Official source URL**: `https://packagist-org-network.s3-eu-west-1.amazonaws.com/ip-address-list`
- **Format**: Plain text, individual IPs (no CIDR notation).
- **Ranges** (as of 2026-04-29): 4 IPv4 IPs (`99.80.65.88`, `63.35.126.107`, `99.80.42.242`, `99.80.18.194`) + 4 IPv6 IPs (2a05:d018:e16::/48 space).
- **ASNs**: AWS AS16509 (eu-west-1 / Ireland region).
- **CDN**: Content served via Bunny.net CDN; only the Packagist worker/origin IPs are listed, not CDN edge IPs.
- **Source quality**: B (official but small list of individual IPs hosted on S3; not a stable structured feed).
- **Recommended tier**: soft / `developer_platform` — include as `later` (very small IP list; CDN IPs not covered; verify freshness before production use).

---

### GitLab CI/CD Runners (additional detail)

- **Outbound from runners**: No static IPs. GitLab explicitly states this. Consumers must allowlist:
  - GCP `us-east1` ranges for Linux runners (GCP cloud.json)
  - GCP `us-central1` ranges for Linux GPU/ARM64 runners (GCP cloud.json)
  - AWS `us-east-1` ranges for macOS runners (AWS ip-ranges.json)
- **This means GitLab runner traffic overlaps with the contextual cloud ranges, not the soft GitLab-specific ranges.**

---

### GitHub Container Registry (GHCR)

- **Role**: Container registry hosted at `ghcr.io`; part of GitHub Packages.
- **Official source**: GitHub Meta API `packages` field — but GitHub's official docs **explicitly state** "IP addresses for some GitHub services might not be listed, such as LFS or GitHub Packages."
- **Source quality**: B (partially covered; known gap).
- **Recommended tier**: soft — include as covered by the `packages` field in the Meta API, but document the known gap clearly.

---

### Azure Pipelines Microsoft-Hosted Agents

- **Role**: CI/CD hosted runner agents for Azure Pipelines.
- **IP source**: The Azure weekly JSON (`AzureCloud.<region>`) where `<region>` is the Azure geography of the organization. No dedicated `AzurePipelines` service tag.
- **macOS agents**: Hosted in GitHub's macOS cloud; use GitHub Meta API `actions_macos` field.
- **Caveat**: The geography-level ranges are very broad (includes all AzureCloud IPs in multi-region geographies). Microsoft explicitly notes "your exposure is rather large as the range of IP addresses is rather large and since machines in this range can belong to other customers as well."
- **Source quality**: A (Azure weekly JSON is authoritative) but the IP set is very broad.
- **Recommended tier**: soft for the outbound `AzureDevOps`-specific ranges; contextual for the broad `AzureCloud.<region>` agent ranges.

---

## Reject (no usable source) — with evidence

### Docker Hub

- **Evidence**: Docker Desktop networking docs, rate-limit docs, and Docker Hub main page contain no IP ranges. Docker Hub is fronted by CDNs and does not publish IP ranges.
- **Verdict**: Reject. No official IP list. Hub is CDN/cloud-fronted with dynamic IPs. HMAC/authentication is the intended security model, not IP allowlisting.

### npm registry (npmjs.com)

- **Evidence**: npm uses Fastly CDN (inferred from status page and general knowledge); registry.npmjs.org is Fastly-fronted. npm status page shows no IP ranges. No official IP publication found.
- **Verdict**: Reject. CDN-fronted (Fastly); no official IP list. Use Fastly IP ranges as a proxy if needed, but npm itself does not publish ranges.

### PyPI (pypi.org / files.pythonhosted.org)

- **Evidence**: PyPI security page identifies AWS and Fastly as sponsors/CDN. No IP ranges published on any PyPI page.
- **Verdict**: Reject. AWS+Fastly fronted; no official IP list. Fastly/AWS ranges cover PyPI CDN but PyPI itself does not publish dedicated ranges.

### RubyGems.org

- **Evidence**: About page confirms "Gems are hosted on Amazon S3, served by Fastly." No IP ranges published.
- **Verdict**: Reject. S3/Fastly fronted; no official IP list.

### Maven Central / Sonatype

- **Evidence**: sonatype.com/central page has no IP ranges. No dedicated IP list documented.
- **Verdict**: Reject. No official IP list found.

### NuGet.org

- **Evidence**: Azure-hosted; Azure Artifacts docs reference `Storage.<region>` tags for blobs but no NuGet-specific tag. No dedicated NuGet.org IP list.
- **Verdict**: Reject. Falls under general Azure; no dedicated NuGet IP list.

### crates.io

- **Evidence**: CloudFront-fronted (Amazon CloudFront CDN). No dedicated IP list.
- **Verdict**: Reject. CDN-fronted (CloudFront); no official IP list. AWS `CLOUDFRONT` service tag covers CDN IPs.

### Go module proxy (proxy.golang.org, sum.golang.org)

- **Evidence**: Google-hosted; falls under GCP ranges. No dedicated IP list published by Go team.
- **Verdict**: Reject. Covered by general GCP cloud.json; no dedicated IP list.

### Quay.io (Red Hat)

- **Evidence**: Quay.io page showed only error/status content; no IP ranges documentation.
- **Verdict**: Reject. No official IP list found. Red Hat / IBM Cloud hosted.

### AWS ECR / ECR Public

- **Evidence**: AWS ip-ranges.json has no `ECR` service tag. All ECR traffic falls under general `AMAZON` or `EC2` tags.
- **Verdict**: Reject for soft. Contextual only (covered by broad AWS ranges). No dedicated ECR service tag.

### Google Container Registry (gcr.io)

- **Evidence**: No GCR-specific service tag in GCP cloud.json. Falls under general GCP ranges.
- **Verdict**: Reject for soft. Contextual only (covered by broad GCP cloud.json ranges). No dedicated GCR IP list.

### DigitalOcean Container Registry

- **Evidence**: DigitalOcean Spaces docs mention internal VPC networking but no public IP list for the container registry.
- **Verdict**: Reject. No official IP list.

### Cloudsmith

- **Evidence**: No official IP list found on the Cloudsmith website.
- **Verdict**: Reject. No official IP list.

### Heroku

- **Evidence**: The Heroku IP ranges docs page (`https://devcenter.heroku.com/articles/heroku-ip-ranges`) returned HTTP 404 at time of research.
- **Verdict**: Reject pending re-verification. If the page exists at an alternate URL, re-check. Even when available, Heroku IP docs historically list Salesforce/Heroku cloud ranges that overlap with broad Salesforce infrastructure.

### Render.com

- **Evidence**: Render docs state outbound IPs are visible per-service in the dashboard, not as a static public list. The docs say "Outbound IP ranges are shared across all services in the same region" but do not publish the actual ranges publicly.
- **Verdict**: Reject. No public static IP list.

### Fly.io

- **Evidence**: Fly.io regions docs show deployment regions but no static outbound IP list.
- **Verdict**: Reject. No official public IP list.

### Railway.app

- **Evidence**: Docs URL returned 404.
- **Verdict**: Reject. No official public IP list verified.

### Vercel

- **Evidence**: Vercel docs page for IP allowlisting returned 404. Regions page shows 20 compute regions and 126 PoPs but no static IP list.
- **Verdict**: Reject. No official public static IP list. 126+ PoPs make a static list impractical.

### Netlify

- **Evidence**: Netlify platform/networks docs returned 404. Netlify's build infrastructure uses dynamic IPs.
- **Verdict**: Reject. No official public static IP list.

### Buildkite

- **Evidence**: Buildkite pipeline IP address docs page returned 404 twice (tried two URL patterns).
- **Verdict**: Reject. No official public IP list verified at time of research. Re-verify at `https://buildkite.com/docs/agent` if needed.

### Snyk

- **Evidence**: Three different Snyk docs URLs for IP allowlisting all returned 404 or the content was not found. Full llms-full.txt sitemap search found no IP allowlist section.
- **Verdict**: Reject. No official public IP list verified. Snyk may publish this in support documentation behind login.

### SonarCloud

- **Evidence**: Docs URL redirected then 404. No SonarCloud IP list found.
- **Verdict**: Reject. No official public IP list verified.

### Codecov

- **Evidence**: Codecov docs IP addresses page returned 404 at the primary and redirected URL.
- **Verdict**: Reject. No official public IP list verified.

### Bitrise (mobile CI)

- **Evidence**: Bitrise IP allowlist help page returned ECONNREFUSED.
- **Verdict**: Reject. No official public IP list verified. Re-verify if the site is accessible later.

### Travis CI (additional caution)

- **Evidence**: Service is DNS-based; Travis CI has had business continuity issues. Docs are dated April 2022.
- **Verdict**: Include as `later` / low-priority, not immediate soft-tier. DNS-based approach is valid but service viability needs assessment.

### Drone.io

- **Evidence**: Drone.io is now part of Harness; self-hosted model. The cloud-hosted CI offering was effectively discontinued.
- **Verdict**: Reject. No cloud-hosted IP list; self-hosted product.

### Codeship

- **Evidence**: Codeship is legacy/EoL (acquired by CloudBees, rarely updated). Help page returned ECONNREFUSED.
- **Verdict**: Reject. Legacy service; no IP list verified.

### TeamCity Cloud

- **Evidence**: TeamCity Cloud docs page returned 404.
- **Verdict**: Reject. No official public IP list verified.

### SourceForge

- **Evidence**: SourceForge main page has no infrastructure documentation. No IP list found.
- **Verdict**: Reject. No official IP list.

### Codeberg

- **Evidence**: About page lists hosting "in Europe" only. No IP list found.
- **Verdict**: Reject. No official IP list.

### Gitea Cloud

- **Evidence**: Gitea docs mention cloud hosting briefly but no IP list found.
- **Verdict**: Reject. No official IP list.

### AWS CodeCommit

- **Evidence**: AWS ip-ranges.json has no `CODECOMMIT` service tag. Falls under general `AMAZON`.
- **Verdict**: Reject for soft. Contextual only. Also: AWS CodeCommit is being retired (AWS announced end of service for new customers).

### Google Cloud Source Repositories

- **Evidence**: Google-hosted; no dedicated IP list. Falls under GCP cloud.json.
- **Verdict**: Reject for soft. Contextual only. Also: Google deprecated Cloud Source Repositories for most users.

### Sigstore / Fulcio / Rekor

- **Evidence**: Sigstore website showed loading state only; no IP list found. Sigstore infrastructure is hosted across multiple providers (Google infrastructure for the public good instance).
- **Verdict**: `later`. Important supply chain infrastructure but hostname-based; no static IP feed. Consider as a future candidate if a stable IP feed emerges.

### Read the Docs

- **Evidence**: readthedocs.org redirected to app.readthedocs.org; no IP list found.
- **Verdict**: Reject. No official IP list.

### GitBook

- **Evidence**: GitBook marketing page only; no networking docs. Security page referenced at `security.gitbook.com` not fetched.
- **Verdict**: Reject. No official IP list verified.

### Linear

- **Evidence**: Linear docs URL redirected but no IP list page found.
- **Verdict**: Reject. No official IP list.

### Hex.pm, pub.dev, CocoaPods, Conda/Anaconda, ArtifactHub

- **Evidence**: None of these publish official static IP ranges. They are either hosted on CDNs, use dynamic cloud IPs, or don't have publicly documented IP feeds.
- **Verdict**: All reject.

---

## Open questions / unverified

1. **Buildkite**: The documented URL `https://buildkite.com/docs/agent/v3/ip-addresses` returned 404 twice. Buildkite may have moved this documentation or renamed the page. Re-verify at `https://buildkite.com/docs/agent`.

2. **Bitrise**: The help page `https://help.bitrise.io` was unreachable (ECONNREFUSED). Bitrise publishes IP ranges for mobile CI. Re-verify when the site is accessible.

3. **Snyk**: Three different URL patterns for Snyk's IP allowlist page all returned 404. Snyk may publish this in customer support/docs behind login. UNVERIFIED: Snyk may have discontinued this documentation page.

4. **SonarCloud**: Docs redirect loop; IP list page not reachable. May require re-fetch.

5. **Codecov**: Both primary and redirected URL returned 404. Codecov (now part of Sentry) may have reorganized docs.

6. **Heroku**: The IP ranges page was 404. Heroku may have moved docs or discontinued publishing IP ranges after the Salesforce restructuring. UNVERIFIED.

7. **Packagist freshness**: The S3-hosted IP list has no documented update cadence. It should be polled and compared against a hash to detect changes.

8. **AWS CodeBuild IPv6**: No IPv6 CODEBUILD service tag found in ip-ranges.json. Verify whether CodeBuild ever has IPv6-only build environments.

9. **GitLab Container Registry CDN IP `34.149.22.116`**: This appears to be a GCP external IP; verify it is truly dedicated to GitLab (not multi-tenant GCP).

10. **GitHub Codespaces**: The Meta API has a `codespaces` field. Whether it is populated and up-to-date needs direct API validation beyond the docs page visited (which only mentions domain-based access).

11. **Travis CI service viability**: Travis CI's infrastructure and business status should be verified before shipping it as a soft-tier reference feed. The service has had stability concerns since 2021.

12. **MCR FrontDoor.FirstParty tag scope**: `AzureFrontDoor.FirstParty` covers all Microsoft-first-party services using Azure Front Door, not just MCR. Including this tag is broader than MCR alone.

13. **Packagist CDN**: Packagist uses Bunny.net CDN for content delivery. The published IP list covers only Packagist worker/origin IPs. Bunny.net publishes its own IP list at `https://bunnycdn.com/api/system/edgeserverlist` — this would need a separate entry if CDN coverage is required.

---

## AWS service tags relevant to developer platforms

From the authoritative `ip-ranges.amazonaws.com/ip-ranges.json` (fetched 2026-04-29):

IPv4 service tags present: AMAZON, AMAZON_APPFLOW, AMAZON_CONNECT, API_GATEWAY, AURORA_DSQL, CHIME_MEETINGS, CHIME_VOICECONNECTOR, CLOUD9, CLOUDFRONT, CLOUDFRONT_ORIGIN_FACING, **CODEBUILD**, DYNAMODB, EBS, EC2, EC2_INSTANCE_CONNECT, GLOBALACCELERATOR, IVS_LOW_LATENCY, IVS_REALTIME, KINESIS_VIDEO_STREAMS, MEDIA_PACKAGE_V2, ROUTE53, ROUTE53_HEALTHCHECKS, ROUTE53_HEALTHCHECKS_PUBLISHING, ROUTE53_RESOLVER, S3, WORKSPACES_GATEWAYS.

**Notable absences** (services with no dedicated service tag): ECR, CodeCommit, CodePipeline, CodeDeploy, CodeArtifact, CodeStar, App Runner, Elastic Beanstalk.

---

## Fastly IP list verification

Fetched `https://api.fastly.com/public-ip-list` (2026-04-29):
- 19 IPv4 ranges: 23.235.32.0/20, 43.249.72.0/22, 103.244.50.0/24, 103.245.222.0/23, 103.245.224.0/24, 104.156.80.0/20, 140.248.64.0/18, 140.248.128.0/17, 146.75.0.0/17, 151.101.0.0/16, 157.52.64.0/18, 167.82.0.0/17, 167.82.128.0/20, 167.82.160.0/20, 167.82.224.0/20, 172.111.64.0/18, 185.31.16.0/22, 199.27.72.0/21, 199.232.0.0/16
- 2 IPv6 ranges: 2a04:4e40::/32, 2a04:4e42::/32

Fastly fronts: npm, PyPI, RubyGems, Vimeo, Spotify (some services), and many others. Including Fastly as a soft CDN edge (already in scope from the CDN soft candidate list) transitively covers package registries that rely on Fastly without needing separate registry entries.

---

## Cloudflare IP verification

Fetched `https://api.cloudflare.com/client/v4/ips` (2026-04-29):
- IPv4 (15 ranges): 173.245.48.0/20, 103.21.244.0/22, 103.22.200.0/22, 103.31.4.0/22, 141.101.64.0/18, 108.162.192.0/18, 190.93.240.0/20, 188.114.96.0/20, 197.234.240.0/22, 198.41.128.0/17, 162.158.0.0/15, 104.16.0.0/13, 104.24.0.0/14, 172.64.0.0/13, 131.0.72.0/22
- IPv6 (7 ranges): 2400:cb00::/32, 2606:4700::/32, 2803:f800::/32, 2405:b500::/32, 2405:8100::/32, 2a06:98c0::/29, 2c0f:f248::/32
- Matches the plain-text feed at `https://www.cloudflare.com/ips-v4`; JSON API returns etag for change detection.
- Note: GitLab.com is fronted by Cloudflare (resolves to 172.65.251.78 in AS13335 Cloudflare space).

---

## Key cross-cutting findings

1. **Most package registries have no official IP list.** npm, PyPI, RubyGems, crates.io, Maven Central, NuGet, Go proxy, Hex.pm, pub.dev — none publish official IP ranges. They are all CDN-fronted (Fastly, CloudFront, or GCP). Including their respective CDNs transitively provides soft coverage.

2. **GitHub Meta API is the most comprehensive developer platform IP feed.** 13 fields covering all major GitHub services. The "not exhaustive" caveat must accompany all derived artifacts.

3. **GitLab.com web/API outbound ranges are small and dedicated** (2 GCP prefixes). Runner traffic cannot be bounded without allowlisting all of GCP/AWS.

4. **Azure DevOps and Azure Pipelines use different IP sources.** DevOps-specific outbound ranges are tight (6 /24 IPv4 + IPv6); Pipelines runner ranges are geographically broad (AzureCloud.region).

5. **Terraform Cloud uses AWS Global Accelerator anycast for its API** — very stable, small IPs. The notification/VCS/sentinel IPs are /32 AWS EC2 IPs.

6. **Cloudflare fronts many developer platforms** (GitLab.com inbound, Atlassian bits, Cloudflare Pages/Workers). Including Cloudflare edge as a soft-tier CDN entry provides transitive coverage for these.

7. **No CI/CD platforms in the "reject" list need to be revisited until their docs become accessible.** Buildkite, Snyk, Codecov, Bitrise, SonarCloud — all had inaccessible docs; re-verify before marking them permanently.

8. **MCR requires two Azure service tags** (MicrosoftContainerRegistry + AzureFrontDoor.FirstParty); both must be in the soft reference feed for correct coverage.

9. **AWS CODEBUILD is the only developer-platform-specific AWS service tag.** No ECR, CodeCommit, CodePipeline tags exist.

10. **Packagist is the only package registry with any official IP list** (8 individual IPs), but it covers only origin/worker IPs, not the Bunny.net CDN.

---

## Recommended implementation order for soft developer-platform reference feeds

Priority 1 (A-grade sources, high coverage, implement first):
- GitHub Meta API (all 13 fields, with "not exhaustive" caveat)
- Atlassian Cloud (`https://ip-ranges.atlassian.com/`)
- Terraform Cloud (`https://app.terraform.io/api/meta/ip-ranges`)
- Azure DevOps (outbound ranges from service tag `AzureDevOps` + static page)
- Microsoft Container Registry (service tags `MicrosoftContainerRegistry` + `AzureFrontDoor.FirstParty`)

Priority 2 (B-grade or A-grade with caveats):
- CircleCI (DNS method: `all.knownips.circleci.com`)
- GitLab.com web/API ranges (static page; source quality C but ranges are small and stated as solely allocated to GitLab)
- AWS CodeBuild (service tag `CODEBUILD` from ip-ranges.json)

Priority 3 (later / needs re-verification):
- Travis CI (DNS method; verify service viability first)
- Packagist (8-IP list; verify freshness)
- Buildkite, Bitrise, Snyk, Codecov, SonarCloud (re-verify docs accessibility)

---

## Sources consulted

Direct fetches (2026-04-29):
- `https://api.github.com/meta`
- `https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-githubs-ip-addresses`
- `https://docs.github.com/en/rest/meta/meta`
- `https://docs.github.com/en/actions/using-github-hosted-runners/about-github-hosted-runners`
- `https://ip-ranges.atlassian.com/`
- `https://app.terraform.io/api/meta/ip-ranges`
- `https://docs.gitlab.com/ee/user/gitlab_com/`
- `https://learn.microsoft.com/en-us/azure/devops/organizations/security/allow-list-ip-url`
- `https://learn.microsoft.com/en-us/azure/devops/pipelines/agents/hosted`
- `https://learn.microsoft.com/en-us/azure/container-registry/container-registry-firewall-access-rules`
- `https://raw.githubusercontent.com/microsoft/containerregistry/main/docs/client-firewall-rules.md`
- `https://circleci.com/docs/ip-ranges/`
- `https://circleci.com/docs/ip-ranges-list.json`
- `https://buildkite.com/docs/agent/v3/ip-addresses` (404)
- `https://buildkite.com/docs/pipelines/security/ip-addresses` (404)
- `https://docs.travis-ci.com/user/ip-addresses/`
- `https://devcenter.heroku.com/articles/heroku-ip-ranges` (404)
- `https://render.com/docs/static-outbound-ip-addresses`
- `https://vercel.com/docs/edge-network/regions`
- `https://docs.netlify.com/platform/networks/` (404)
- `https://docs.docker.com/docker-hub/download-rate-limit/`
- `https://docs.docker.com/desktop/networking/`
- `https://docs.npmjs.com/using-npm/registry`
- `https://pypi.org/security/`
- `https://rubygems.org/pages/about`
- `https://central.sonatype.com/`
- `https://pkg.go.dev/about`
- `https://quay.io/`
- `https://docs.digitalocean.com/products/spaces/`
- `https://pub.dev/`
- `https://packagist.org/about`
- `https://packagist-org-network.s3-eu-west-1.amazonaws.com/ip-address-list`
- `https://docs.snyk.io/sitemap.md` (IP section not found)
- `https://docs.snyk.io/llms-full.txt` (IP section not found)
- `https://docs.sonarcloud.io/...` (404)
- `https://docs.codecov.com/docs/codecov-ip-addresses` (404)
- `https://help.bitrise.io/...` (ECONNREFUSED)
- `https://fly.io/docs/reference/regions/`
- `https://docs.railway.com/reference/expose-your-app` (404)
- `https://codeberg.org/about`
- `https://www.sigstore.dev/`
- `https://docs.drone.io/`
- `https://artifacthub.io/`
- `https://anaconda.org/about` (404)
- `https://sourceforge.net/`
- `https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry`
- `https://aws.amazon.com/ecr/`
- `https://docs.linear.app/security` (redirect)
- `https://www.gitbook.com/`
- `https://api.fastly.com/public-ip-list`
- `https://www.cloudflare.com/ips-v4`
- `https://api.cloudflare.com/client/v4/ips`
- `https://ip-ranges.amazonaws.com/ip-ranges.json` (full service tag enumeration)
- Team Cymru DNS whois for GitLab.com IP (confirmed AS13335 Cloudflare)
