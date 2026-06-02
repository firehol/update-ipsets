#!/usr/bin/env python3
"""Generate deterministic static enrichment for FireHOL-maintained feeds.

Background
----------
FireHOL-maintained feeds (the catalog's own merges, static reference lists, and
internal-derived baselines) are NEVER researched by the enrichment agent: doing
so would either violate the no-firehol-self-reference rule or produce
hall-of-mirrors output (we'd be researching ourselves). The wrapper at
`agents/run-enrichment.sh` refuses them; this script generates their enrichment
deterministically from the catalog YAMLs.

Three shapes are handled:

  1. static-curated  — sources/.../<file>.yaml with `static:` blocks and no url
                       Example: critical_public_dns_core (DNS resolvers),
                       critical_dns_root_servers, critical_as112
  2. internal-baseline — sources/.../<file>.yaml with `url: internal://...`
                       Example: rfc_reserved (RFC-defined reserved ranges)
  3. merge           — any configs/firehol YAML with `merges: {name: {sources: [...]}}`
                       Example: firehol_level1, firehol_anonymous, cleantalk,
                       critical_soft_akamai_edge_secondary

For merges, listing/unlisting policy explicitly defers to the component feeds
— this catalog does NOT maintain its own whitelisting on merges. If a future
merge starts carrying exclusion logic (see SOW-0016: no-private variations,
critical-infra exclusions), this generator must be updated accordingly.

The per-feed editorial copy lives in EDITORIAL below. For feeds without an
explicit editorial entry, a sensible default per shape is used.

Run
---
    python3 tools/build-firehol-static-enrichment.py [--dry-run] [--validate]

Outputs to:
    .local/agents/feed-enrichment/<feed>/<UTC>-static/output.json
    .local/agents/feed-enrichment/<feed>/<UTC>-static/source.yaml      (copy)

Validation
----------
With --validate, runs agents/validate-output.py on each generated file.
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import shutil
import sys
from datetime import datetime, timezone
from functools import lru_cache
from pathlib import Path

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML is required. Install with: pip install pyyaml", file=sys.stderr)
    sys.exit(2)

REPO = Path(__file__).resolve().parent.parent
SOURCES_DIR = REPO / "configs" / "firehol" / "sources"
CONFIG_DIR = REPO / "configs" / "firehol"
OUT_BASE = REPO / ".local" / "agents" / "feed-enrichment"
VALIDATOR = REPO / "agents" / "validate-output.py"

GENERATOR_TAG = "build-firehol-static-enrichment.py"


# --- Editorial copy ----------------------------------------------------------
#
# Per-feed editorial content. Each entry overrides defaults for that feed.
# Fields that aren't set fall through to per-shape defaults. Keep this
# file the single source of truth for catalog editorial copy.

EDITORIAL: dict[str, dict] = {
    # ---- static-curated (critical_dns.yaml sub-feeds) ----
    "critical_public_dns_core": {
        "official_name": "Core public DNS resolvers",
        "short_description":
            "Curated reference list of widely-used public recursive DNS "
            "resolver service addresses (Cloudflare, Google Public DNS, Quad9, "
            "Cisco Umbrella/OpenDNS).",
        "long_description":
            "A curated reference list of IP addresses operated as public "
            "recursive DNS resolver services by major providers — Cloudflare, "
            "Google Public DNS, Quad9 recommended service, and Cisco "
            "Umbrella/OpenDNS. This is a reference feed, not a threat "
            "indicator. The intended use is whitelisting or exclusion: many "
            "networks intentionally configure these addresses, and blocking "
            "them can immediately break name resolution for users who rely "
            "on them.",
        "intended_for": [
            "Whitelisting from broader block policies",
            "Excluding from anti-bot rules and rate-limiting",
            "Operator awareness when investigating DNS-related traffic",
        ],
        "not_intended_for": [
            "Threat blocking — these are legitimate critical infrastructure",
            "Outbound deny lists",
        ],
    },
    "critical_dns_root_servers": {
        "official_name": "DNS root server service addresses",
        "short_description":
            "The IPv4 service addresses of the 13 named DNS root server "
            "authorities, per the IANA root server registry.",
        "long_description":
            "A curated reference list of the IPv4 service addresses for the "
            "13 named DNS root server authorities (A through M), as "
            "published by IANA. This is foundational DNS resolution "
            "infrastructure; blocking these addresses can break recursive "
            "resolver bootstrap and root-zone lookup paths. The list is a "
            "reference feed, not a threat indicator.",
        "intended_for": [
            "Whitelisting from edge block lists",
            "Excluding from anti-bot/rate-limit rules at recursive resolvers",
            "Audit of DNS traffic patterns",
        ],
        "not_intended_for": [
            "Threat blocking — these are legitimate critical infrastructure",
        ],
    },
    "critical_as112": {
        "official_name": "AS112 DNS sink infrastructure",
        "short_description":
            "The IPv4 service prefixes for AS112 nameserver operations, per "
            "RFC 7534 and RFC 7535.",
        "long_description":
            "A curated reference list of IPv4 service prefixes documented "
            "in RFC 7534 and RFC 7535 for AS112 nameserver operations. "
            "AS112 absorbs leaked reverse-DNS queries for private and "
            "special-use address space; blocking these prefixes can disrupt "
            "public DNS hygiene and diagnostics. This is a reference feed, "
            "not a threat indicator.",
        "intended_for": [
            "Whitelisting from edge block lists",
            "Excluding from anti-bot rules at DNS infrastructure",
        ],
        "not_intended_for": [
            "Threat blocking — this is legitimate DNS hygiene infrastructure",
        ],
    },

    # ---- internal-baseline (rfc_reserved) ----
    "rfc_reserved": {
        "official_name": "RFC-defined reserved IPv4 ranges (baseline bogons)",
        "short_description":
            "Hardcoded baseline of the 15 RFC-defined reserved IPv4 ranges "
            "(RFC 1918 private, loopback, link-local, multicast, TEST-NET, "
            "and others).",
        "long_description":
            "A locally-generated baseline of the IPv4 ranges that the "
            "IETF/IANA reserve for special use — RFC 1918 private networks, "
            "loopback, link-local autoconfiguration, multicast, the "
            "documentation/TEST-NET ranges, the benchmark range, the "
            "shared CGN range, and others. The list is bundled with the "
            "catalog (internal source) and is intentionally static; entries "
            "change only when the underlying RFCs do. It is intended as a "
            "stable bogons floor against which observed traffic can be "
            "audited.",
        "intended_for": [
            "Bogon filtering on internet-facing routers",
            "Auditing for misconfigured or spoofed source addresses",
            "Excluding RFC-reserved space from threat lookups",
        ],
        "not_intended_for": [
            "A complete bogon list (does not track allocated-but-unassigned RIR space — see fullbogons for that)",
        ],
    },

    # ---- merges ----
    "firehol_level1": {
        "official_name": "FireHOL Level 1",
        "short_description":
            "Conservative aggregate merge for edge blocking — low false-positive "
            "components only, suitable for internet-facing servers, routers, and "
            "firewalls.",
        "long_description":
            "The most conservative of the FireHOL composite merges. Combines "
            "well-established low-false-positive feeds (DShield, Feodo Tracker, "
            "Cymru Fullbogons, Spamhaus DROP) into a single additive union. "
            "Intended for environments where false positives are costly — "
            "edge firewalls protecting internet-facing services, where "
            "blocking a legitimate IP causes immediate business impact. "
            "The merge inherits all listing decisions from its component "
            "feeds and does not maintain its own whitelisting.",
        "intended_for": [
            "Edge firewall block lists for internet-facing services",
            "Conservative perimeter blocking where FP cost is high",
        ],
        "not_intended_for": [
            "Comprehensive coverage — see level3/level4 for broader aggregates",
            "Operator-specific whitelisting decisions (handled at the component-feed level)",
        ],
    },
    "firehol_level2": {
        "official_name": "FireHOL Level 2",
        "short_description":
            "Medium-confidence aggregate merge — adds short-window abuser data on "
            "top of level1's conservative coverage.",
        "long_description":
            "A medium-confidence FireHOL composite merge that builds on the "
            "level1 conservative core by adding short-window observation feeds "
            "(blocklist.de hour and day buckets, dshield_1d). The result is "
            "broader than level1 with somewhat higher false-positive exposure. "
            "Intended for environments that can absorb modest FP risk in "
            "exchange for fresher coverage of recently-active sources. "
            "Component feeds drive all listing decisions; this merge adds "
            "no logic of its own.",
        "intended_for": [
            "Detection enrichment (SIEM, IDS)",
            "Edge blocking where moderate FP exposure is acceptable",
        ],
        "not_intended_for": [
            "Inline blocking on services with zero FP tolerance — use level1",
        ],
    },
    "firehol_level3": {
        "official_name": "FireHOL Level 3",
        "short_description":
            "Broader-coverage aggregate merge — wider feed set with higher "
            "false-positive exposure.",
        "long_description":
            "A broader FireHOL composite merge that adds longer-window and "
            "more aggressive feeds on top of level2. Coverage increases, "
            "false-positive exposure increases proportionately. Intended for "
            "use cases that prioritize completeness over precision — threat "
            "intelligence enrichment, hunting queries, and detection "
            "environments where the analyst can adjudicate matches. "
            "Component feeds drive all listing decisions.",
        "intended_for": [
            "Threat-intelligence enrichment and hunting queries",
            "Detection environments with analyst-in-the-loop adjudication",
        ],
        "not_intended_for": [
            "Inline blocking — use level1 or level2 for that",
        ],
    },
    "firehol_level4": {
        "official_name": "FireHOL Level 4",
        "short_description":
            "Broadest FireHOL aggregate merge — highest coverage, highest FP exposure.",
        "long_description":
            "The broadest of the FireHOL composite merges. Combines the level3 "
            "set with additional broad-coverage feeds for maximum aggregate "
            "size. Expect non-trivial false-positive exposure: this is the "
            "aggregate-of-aggregates tier, not a curated low-FP list. "
            "Intended for research, retrospective analysis, and detection "
            "environments where breadth matters more than precision. "
            "Component feeds drive all listing decisions.",
        "intended_for": [
            "Retrospective forensic search (was this IP ever flagged anywhere?)",
            "Research and overlap analysis across the broader feed ecosystem",
        ],
        "not_intended_for": [
            "Inline blocking",
            "Operator decisions sensitive to false positives",
        ],
    },
    "firehol_abusers_1d": {
        "official_name": "FireHOL Abusers (1-day window)",
        "short_description":
            "Aggregate merge of recently-active abusive IP feeds across a 1-day window.",
        "long_description":
            "An aggregate merge of short-window abuser observations from "
            "multiple component feeds. Reflects IPs that have been observed "
            "engaging in abusive behavior within roughly the last day "
            "according to the component sources. Intended for fresh-data "
            "use cases (detection, near-real-time blocking) where stale "
            "history is less useful. Component feeds drive all listings.",
        "intended_for": [
            "Short-term blocklist refresh on edge filters",
            "Detection rule enrichment for recently-active sources",
        ],
        "not_intended_for": [
            "Historical investigation (see firehol_abusers_30d for the longer window)",
        ],
    },
    "firehol_abusers_30d": {
        "official_name": "FireHOL Abusers (30-day window)",
        "short_description":
            "Aggregate merge of abusive IP feeds across a 30-day rolling window.",
        "long_description":
            "An aggregate merge of abuser observations across a roughly "
            "30-day rolling window from multiple component feeds. Reflects "
            "IPs that have been observed engaging in abusive behavior "
            "within the last month. The longer window improves retrospective "
            "coverage at the cost of including IPs that may have rotated. "
            "Component feeds drive all listings.",
        "intended_for": [
            "Retrospective enrichment of older logs and alerts",
            "Threat-intel sweep across the last month",
        ],
        "not_intended_for": [
            "Inline blocking — older entries may be stale (see _1d for fresh data)",
        ],
    },
    "firehol_anonymous": {
        "official_name": "FireHOL Anonymous",
        "short_description":
            "Aggregate merge of anonymizer-class feeds (Tor exits, open proxies, "
            "anonymizer services).",
        "long_description":
            "An aggregate merge of feeds tracking anonymizer infrastructure — "
            "Tor exit nodes, anonymous-proxy services, and operator-published "
            "proxy aggregates. Reflects the union of anonymizing IPs covered "
            "by the component feeds. Useful for fraud prevention (account "
            "creation, payment) and policy enforcement where anonymized "
            "traffic is restricted. The merge inherits all classification "
            "decisions from its component feeds.",
        "intended_for": [
            "Fraud-prevention risk scoring",
            "Policy enforcement on anonymous-traffic restrictions",
        ],
        "not_intended_for": [
            "Distinguishing commercial VPN from residential-proxy from Tor — see component feeds for category granularity",
        ],
    },
    "firehol_proxies": {
        "official_name": "FireHOL Proxies",
        "short_description":
            "Aggregate merge of open-proxy, SOCKS, and proxy-service feeds.",
        "long_description":
            "An aggregate merge of feeds tracking open and commercial proxy "
            "infrastructure: SOCKS proxies, HTTP/S open proxies, and "
            "operator-published proxy aggregates. Reflects the union of "
            "proxy IPs covered by the component feeds. Distinct from "
            "firehol_anonymous in that proxies don't necessarily provide "
            "anonymity guarantees but do route traffic through intermediate "
            "hosts. The merge inherits all listings from its components.",
        "intended_for": [
            "Detecting proxy-routed traffic for fraud or policy enforcement",
            "Excluding proxy IPs from rate-limit or geo-policy decisions",
        ],
        "not_intended_for": [
            "Inline blocking without analyst-in-the-loop — open proxies often share IP space with legitimate services",
        ],
    },
    "firehol_webclient": {
        "official_name": "FireHOL Web Client",
        "short_description":
            "Aggregate of feeds covering IPs implicated in attacks targeting client-side "
            "browsers and web users.",
        "long_description":
            "An aggregate merge of feeds covering IP addresses implicated "
            "in attacks targeting client-side web users — malicious ad "
            "infrastructure, drive-by download hosts, and traffic-distribution "
            "systems serving exploit kits. Component feeds drive all listings; "
            "this merge adds no detection logic of its own.",
        "intended_for": [
            "DNS or proxy filtering on user-facing browsing",
            "Detection of malicious-ad / drive-by traffic patterns",
        ],
        "not_intended_for": [
            "Blocking inbound traffic to your own services — see firehol_webserver",
        ],
    },
    "firehol_webserver": {
        "official_name": "FireHOL Web Server",
        "short_description":
            "Aggregate of feeds covering IPs implicated in attacks against web servers.",
        "long_description":
            "An aggregate merge of feeds covering IP addresses implicated "
            "in attacks against web servers — scanners, automated exploit "
            "attempts, brute-force login sources, and WAF-flagged abusers "
            "targeting public web infrastructure. Component feeds drive all "
            "listings.",
        "intended_for": [
            "WAF and edge firewall protection of public web services",
            "Detection rules on inbound HTTP/HTTPS traffic",
        ],
        "not_intended_for": [
            "Blocking outbound user-browser traffic — see firehol_webclient",
        ],
    },
    "cleantalk": {
        "official_name": "CleanTalk merge",
        "short_description":
            "Aggregate merge of CleanTalk-based abuse feeds.",
        "long_description":
            "An aggregate merge of CleanTalk's abuse feeds. Reflects the "
            "union of all CleanTalk-derived listings the catalog tracks. "
            "Component feeds drive all listings; this merge adds no logic of "
            "its own.",
        "intended_for": [
            "Anti-spam and forum-abuse blocking",
        ],
    },
    "cymru_unassigned": {
        "official_name": "Team Cymru unassigned (bogons-class)",
        "short_description":
            "Bogons-class aggregate combining Team Cymru's IP allocation tracking.",
        "long_description":
            "A bogons-class aggregate combining Team Cymru's IP allocation "
            "tracking. Tracks IP space that is not yet allocated to a Regional "
            "Internet Registry (RIR) end-user. Useful for filtering "
            "fully-unallocated address space at internet-facing borders. "
            "Component feed drives all listings.",
        "intended_for": [
            "Bogon filtering at internet-facing routers",
            "Detection of spoofed source addresses",
        ],
    },
}


# --- Schema-shaped builders ---------------------------------------------------

ISO_NOW = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
UTC_STAMP = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")

INTERNAL_MAINTAINER_NOTE = (
    "Maintained by the FireHOL catalog project as a public reference / aggregate. "
    "This page is generated deterministically from the catalog YAML; the agent-"
    "based enrichment used for third-party feeds does not run on FireHOL-"
    "maintained feeds."
)


def make_role_firehol() -> dict:
    return {
        "role": "maintainer",
        "name": "FireHOL catalog project",
        "organization_type": "informal_collective",
        "official_url": None,
        "contact_email": None,
        "based_in": None,
        "active_since": None,
        "notes": INTERNAL_MAINTAINER_NOTE,
    }


import re as _re

# Regex matching IPv4 literals (with optional CIDR) — the same shape the
# validator uses. Used to scrub IPs out of YAML-derived prose before it
# lands in public fields.
_IPV4_RE = _re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}(?:/\d{1,2})?\b")


def scrub_ips(text: str) -> str:
    """Remove IPv4 literals from text intended for public fields.

    The YAML 'info' fields on FireHOL-maintained static feeds sometimes
    embed literal IPs (e.g. "Cloudflare 1.1.1.1, Google Public DNS, ...").
    Those IPs are operational data that belongs in the feed file itself,
    not in the enrichment prose — the same rule the agent follows.
    """
    return _IPV4_RE.sub("", text).replace("  ", " ").replace(" ,", ",").strip()


@lru_cache(maxsize=1)
def catalog_entries() -> dict[str, dict]:
    entries: dict[str, dict] = {}
    for y in sorted(CONFIG_DIR.rglob("*.yaml")):
        try:
            data = yaml.safe_load(y.read_text())
        except (OSError, yaml.YAMLError):
            continue
        if not isinstance(data, dict):
            continue
        for section_name in ("sources", "merges"):
            for name, entry in (data.get(section_name) or {}).items():
                if isinstance(entry, dict):
                    entries[name] = entry
    return entries


def entry_is_redistributable(name: str, visiting: set[str] | None = None) -> bool:
    entry = catalog_entries().get(name)
    if not isinstance(entry, dict):
        return True

    if "sources" not in entry and "exclude" not in entry:
        return entry.get("redistributable") is not False

    if visiting is None:
        visiting = set()
    if name in visiting:
        return False
    visiting.add(name)
    try:
        allowed = entry.get("redistributable") is not False
        for ref in list(entry.get("sources") or []) + list(entry.get("exclude") or []):
            if not entry_is_redistributable(str(ref), visiting):
                allowed = False
        return allowed
    finally:
        visiting.remove(name)


def section(value, reasoning: str, confidence: str = "explicitly-stated") -> dict:
    """Standard section shape — includes maintainer_quotes (always empty for
    static-generated content; we have no maintainer quotes since we ARE the
    maintainer)."""
    return {
        "value": value,
        "evidence_ids": [],
        "maintainer_quotes": [],
        "confidence": confidence,
        "assistant_reasoning": reasoning,
    }


def community_section(value, reasoning: str, confidence: str = "no-information") -> dict:
    """community section omits maintainer_quotes per schema."""
    return {
        "value": value,
        "evidence_ids": [],
        "confidence": confidence,
        "assistant_reasoning": reasoning,
    }


def humanize_feed_name(name: str) -> str:
    parts = name.replace("_", " ").split()
    return " ".join(p.capitalize() if not p.isupper() else p for p in parts)


# --- Builders per shape -------------------------------------------------------


def build_static_curated(feed_name: str, src: dict, yaml_path: Path) -> dict:
    """critical_public_dns_core / critical_dns_root_servers / critical_as112."""
    ed = EDITORIAL.get(feed_name, {})
    label = src.get("label") or ed.get("official_name") or humanize_feed_name(feed_name)
    # YAML info / rationale sometimes embed literal IPs (e.g. "Cloudflare
    # 1.1.1.1, Google Public DNS, ..."). Strip them before using in public
    # prose — same rule the agent follows for third-party feeds.
    info = scrub_ips((src.get("info") or "").strip())
    rationale = scrub_ips((src.get("critical", {}) or {}).get("rationale", "").strip())
    license_str = src.get("license") or "Public reference data curated by FireHOL"
    redistrib = bool(src.get("redistributable", False))

    out = {
        "enrichment_schema_version": 2,
        "feed_name": feed_name,
        "run_at": ISO_NOW,
        "evidence": [],
        "official_name": section(
            ed.get("official_name") or label,
            "Editorial copy from the static-enrichment generator; no external research.",
        ),
        "official_url": section(
            None,
            "Static reference data; the catalog itself is the publication surface.",
            confidence="no-information",
        ),
        "short_description": section(
            ed.get("short_description") or info,
            "Generator default from YAML 'info' field or editorial override.",
        ),
        "long_description": section(
            ed.get("long_description") or info,
            "Editorial copy; no external research applies.",
        ),
        "roles": section(
            [make_role_firehol()],
            "FireHOL-maintained static reference feed; the catalog project is the sole responsible party.",
        ),
        "derivation": section(
            {
                "type": "original",
                "description": (
                    "Static curated reference data published as part of the FireHOL "
                    "catalog. Entries are explicit, hand-curated, and change only "
                    "when the underlying authoritative source (provider documentation, "
                    "IANA registry, or cited RFC) changes."
                ),
                "source_feeds": [],
            },
            "Curated reference data; not derived from another IP feed.",
        ),
        "listing_policy": section(
            {
                "summary": (
                    "Curated reference. An entry appears in this list when the "
                    "underlying authoritative source (provider documentation, IANA "
                    "registry, or cited RFC) names it as part of the relevant "
                    "infrastructure class."
                ),
                "criteria": [
                    "Entry is published by the authoritative source as part of the "
                    "feed's stated scope.",
                ],
            },
            "Listing is editorial / curated, not detection-based.",
        ),
        "unlisting_policy": section(
            {
                "summary": (
                    "Entries are removed only when the underlying authoritative source "
                    "(provider documentation, IANA registry, RFC) removes them. The "
                    "catalog does not maintain its own per-entry unlist process."
                ),
                "criteria": [
                    "Authoritative source removes the entry from its published scope.",
                ],
            },
            "Static reference; unlisting tracks source changes.",
        ),
        "unlist_request": section(
            {
                "url": None,
                "email": None,
                "instructions": (
                    "This is a static curated reference feed. The catalog does not "
                    "operate a per-entry unlist channel. If an entry should not be "
                    "in this list, the change must occur at the authoritative "
                    "source (provider documentation, IANA registry, or relevant "
                    "RFC); the catalog will then reflect it on next refresh."
                ),
            },
            "Reference feeds have no unlist mechanism.",
            confidence="explicitly-stated",
        ),
        "update_frequency": section(
            None,
            "Static reference data; only refreshed when the underlying authoritative source changes.",
            confidence="no-information",
        ),
        "detection_classification": section(
            {
                "primary_method": "policy_assignment",
                "secondary_methods": [],
                "description": (
                    "Curated policy-defined reference data. Entries are not derived "
                    "from observation, scanning, or honeypot detection; they are the "
                    "explicit allocations or service addresses published by the "
                    "relevant authority."
                ),
            },
            "Policy-defined curated reference data.",
        ),
        "scope_and_intent": section(
            {
                "description": (
                    f"{info}\n\n{rationale}".strip()
                    if rationale
                    else info
                    or "See the maintainer's documentation for scope."
                ),
                "intended_for": ed.get("intended_for") or [],
                "not_intended_for": ed.get("not_intended_for") or [],
            },
            "Composed from YAML info and critical.rationale plus editorial.",
        ),
        "license": section(
            license_str,
            "From the YAML license field.",
        ),
        "redistribution": section(
            {
                "allowed": redistrib,
                "commercial_use_allowed": None,
                "attribution_required": None,
                "terms": None,
            },
            "From the YAML redistributable field.",
        ),
        "current_status": section(
            {
                "state": "active",
                "description": (
                    "Static curated reference; published as part of the FireHOL "
                    "catalog. Entries change only when the underlying authoritative "
                    "source publishes a change."
                ),
                "successor": None,
                "announcement_date": None,
            },
            "Static reference feeds are durably active; lifecycle changes are infrequent.",
        ),
        "community": community_section(
            {"awards": None, "criticism": None, "engagement": None},
            "Catalog-internal reference feed; community signals belong to the underlying authoritative source, not to the catalog's republication.",
            confidence="no-information",
        ),
    }
    return out


def build_internal_baseline(feed_name: str, src: dict, yaml_path: Path) -> dict:
    """rfc_reserved — locally-generated baseline keyed off RFCs/IANA."""
    ed = EDITORIAL.get(feed_name, {})
    label = src.get("label") or ed.get("official_name") or humanize_feed_name(feed_name)
    info = scrub_ips((src.get("info") or "").strip())
    license_str = src.get("license") or "Public Domain (IANA / RFC reference data)"

    out = {
        "enrichment_schema_version": 2,
        "feed_name": feed_name,
        "run_at": ISO_NOW,
        "evidence": [],
        "official_name": section(
            ed.get("official_name") or label,
            "Editorial copy from the static-enrichment generator; no external research.",
        ),
        "official_url": section(
            None,
            "Locally-generated baseline; the catalog itself is the publication surface.",
            confidence="no-information",
        ),
        "short_description": section(
            ed.get("short_description") or info,
            "Editorial copy.",
        ),
        "long_description": section(
            ed.get("long_description") or info,
            "Editorial copy.",
        ),
        "roles": section(
            [make_role_firehol()],
            "FireHOL-maintained internal baseline.",
        ),
        "derivation": section(
            {
                "type": "original",
                "description": (
                    "Locally-generated baseline maintained inside the catalog. "
                    "Entries are explicit IPv4 ranges drawn from cited IETF/IANA "
                    "documents. Not derived from another IP feed."
                ),
                "source_feeds": [],
            },
            "Internal baseline; no external upstream IP feed.",
        ),
        "listing_policy": section(
            {
                "summary": (
                    "Curated baseline. An entry appears here when an IETF/IANA "
                    "document reserves the range for special use (private, "
                    "loopback, link-local, documentation, benchmark, multicast, "
                    "TEST-NET, shared CGN, etc.)."
                ),
                "criteria": [
                    "Range is reserved by RFC or IANA registry for special use.",
                ],
            },
            "Listing tracks the authoritative reference documents.",
        ),
        "unlisting_policy": section(
            {
                "summary": (
                    "Entries are removed only when the IETF or IANA changes the "
                    "underlying reservation. The catalog does not unlist on a "
                    "per-entry basis."
                ),
                "criteria": [
                    "IETF or IANA revises the relevant reservation.",
                ],
            },
            "Authoritative source determines unlisting.",
        ),
        "unlist_request": section(
            {
                "url": None,
                "email": None,
                "instructions": (
                    "This is an internal baseline. The catalog does not operate "
                    "a per-entry unlist channel; the change must occur in the "
                    "underlying RFC or IANA registry."
                ),
            },
            "Internal baseline; no operator-facing unlist process.",
        ),
        "update_frequency": section(
            None,
            "Changes only when the underlying RFCs or IANA registries are updated.",
            confidence="no-information",
        ),
        "detection_classification": section(
            {
                "primary_method": "policy_assignment",
                "secondary_methods": [],
                "description": (
                    "Policy-defined reference data drawn from RFCs and IANA "
                    "registries. Not derived from observation."
                ),
            },
            "Policy-defined reference data.",
        ),
        "scope_and_intent": section(
            {
                "description": info or "RFC-defined IPv4 reservations.",
                "intended_for": ed.get("intended_for") or [],
                "not_intended_for": ed.get("not_intended_for") or [],
            },
            "From YAML info plus editorial.",
        ),
        "license": section(
            license_str,
            "From the YAML license field.",
        ),
        "redistribution": section(
            {
                "allowed": True,
                "commercial_use_allowed": None,
                "attribution_required": None,
                "terms": None,
            },
            "Underlying data is public-domain RFC/IANA reference; the baseline is freely redistributable.",
        ),
        "current_status": section(
            {
                "state": "active",
                "description": (
                    "Internal baseline maintained inside the catalog. Entries "
                    "change only when the underlying RFC or IANA reservation does."
                ),
                "successor": None,
                "announcement_date": None,
            },
            "Stable internal baseline.",
        ),
        "community": community_section(
            {"awards": None, "criticism": None, "engagement": None},
            "Internal baseline; no community signals apply at this layer.",
            confidence="no-information",
        ),
    }
    return out


def build_merge(feed_name: str, merge_cfg: dict, yaml_path: Path) -> dict:
    """firehol_level1..4, firehol_anonymous, etc."""
    ed = EDITORIAL.get(feed_name, {})
    info = scrub_ips((merge_cfg.get("info") or "").strip())
    sources = list(merge_cfg.get("sources") or [])
    excludes = list(merge_cfg.get("exclude") or [])
    n = len(sources)
    m = len(excludes)
    sources_csv = ", ".join(sources)
    excludes_csv = ", ".join(excludes)
    additive_phrase = f"{n} additive component feed(s) ({sources_csv})" if sources else "no additive component feeds"
    subtractive_phrase = (
        f", with {m} subtractive component feed(s) excluded from the result ({excludes_csv})"
        if excludes
        else ""
    )
    union_intro = (
        f"Additive union of {additive_phrase}{subtractive_phrase}."
        if excludes
        else f"Additive union of {n} component feed(s)."
    )
    redistributable = entry_is_redistributable(feed_name)
    license_str = merge_cfg.get("license") or "Composite of component feed licenses"
    if redistributable:
        redistribution_terms = (
            "Component-feed license terms continue to apply; users should "
            "consult each component's license before commercial or "
            "redistribution use."
        )
        redistribution_reasoning = (
            "Merge and component feeds are redistributable according to the "
            "catalog's current direct-upstream policy; component terms still "
            "apply downstream."
        )
    else:
        redistribution_terms = (
            "One or more additive or subtractive component feeds are marked "
            "non-redistributable in the catalog. Raw redistribution of this "
            "merge is disabled conservatively; component-feed license terms "
            "continue to apply."
        )
        redistribution_reasoning = (
            "Merge redistributability inherits from every additive and "
            "subtractive component feed. At least one component is not "
            "redistributable, so the merge is not redistributable."
        )

    # source_feeds entries — schema allows identifier, relationship, notes only.
    # Subtractive (excluded) components are recorded alongside the additive ones
    # with a `notes` marker so AI consumers and operators see the full picture;
    # the schema's relationship enum does not have a "subtracted_from" value, so
    # we keep the structural type and put the role in `notes`.
    source_feeds = [
        {
            "identifier": s,
            "relationship": "aggregate_component",
            "notes": None,
        }
        for s in sources
    ] + [
        {
            "identifier": s,
            "relationship": "filtered",
            "notes": "Subtracted from the merge result.",
        }
        for s in excludes
    ]

    if excludes:
        long_desc_default = (
            f"An aggregate merge of {n} additive component feed(s) minus "
            f"{m} subtractive component feed(s). The merge applies no "
            f"additional listing or whitelisting logic of its own; all "
            f"decisions are inherited from the included components, with "
            f"every IP that appears in any excluded component removed from "
            f"the result."
        )
        listing_summary = (
            f"An IP appears in this merge if it appears in any of the {n} "
            f"additive component feed(s) ({sources_csv}) AND does not appear "
            f"in any of the {m} subtractive component feed(s) ({excludes_csv}). "
            f"The merge does not add IPs of its own."
        )
        component_line = (
            f"Components: {sources_csv}. Excluded: {excludes_csv}."
        )
    else:
        long_desc_default = (
            f"An aggregate merge of {n} component feed(s). The merge applies "
            f"no additional listing or whitelisting logic of its own; all "
            f"decisions are inherited from the component feeds listed below."
        )
        listing_summary = (
            f"An IP appears in this merge if it appears in any of the {n} "
            f"additive component feed(s) listed below. The merge does not "
            f"add IPs of its own."
        )
        component_line = "Components: " + sources_csv + "."

    long_desc = ed.get("long_description") or info or long_desc_default

    out = {
        "enrichment_schema_version": 2,
        "feed_name": feed_name,
        "run_at": ISO_NOW,
        "evidence": [],
        "official_name": section(
            ed.get("official_name") or humanize_feed_name(feed_name),
            "Editorial copy from the static-enrichment generator.",
        ),
        "official_url": section(
            None,
            "Merge is catalog-internal; no separate maintainer URL applies.",
            confidence="no-information",
        ),
        "short_description": section(
            ed.get("short_description") or info,
            "Editorial copy or YAML info.",
        ),
        "long_description": section(
            f"{long_desc}\n\n{component_line}",
            "Editorial copy followed by deterministic component listing from YAML.",
        ),
        "roles": section(
            [make_role_firehol()],
            "FireHOL-maintained merge; no third-party maintainer.",
        ),
        "derivation": section(
            {
                "type": "aggregate_merge",
                "description": (
                    f"{union_intro} The merge applies no scoring, threshold, "
                    f"or filter beyond the union and the optional subtraction; "
                    f"all listing decisions are inherited from the component "
                    f"feeds. Component feeds are recomposed on each catalog "
                    f"refresh."
                ),
                "source_feeds": source_feeds,
            },
            "Catalog-defined merge; composition driven by YAML config.",
        ),
        "listing_policy": section(
            {
                "summary": listing_summary,
                "criteria": (
                    [f"IP appears in one or more of the additive component feeds: {sources_csv}."]
                    + (
                        [f"IP does not appear in any of the subtractive component feeds: {excludes_csv}."]
                        if excludes
                        else []
                    )
                ),
            },
            "Listing is inherited from components.",
        ),
        "unlisting_policy": section(
            {
                "summary": (
                    "Inherited from the component feeds. The merge does not "
                    "maintain its own whitelisting or per-entry unlist process. "
                    "When a component feed removes an IP, the next merge refresh "
                    "drops the IP unless another component still lists it."
                ),
                "criteria": [
                    "All component feeds that previously listed the IP have removed it.",
                ],
            },
            "Unlisting inherits from components.",
        ),
        "unlist_request": section(
            {
                "url": None,
                "email": None,
                "instructions": (
                    "This is an aggregate merge of component feeds; the merge "
                    "does not maintain its own unlist channel. To request that "
                    "an IP be removed, identify which component feed(s) list "
                    "it and submit the removal request directly to that "
                    f"maintainer. Component feeds: {sources_csv}."
                    + (
                        f" Subtractive feeds (whose entries are removed from the merge): {excludes_csv}."
                        if excludes
                        else ""
                    )
                ),
            },
            "Unlist requests route to components.",
        ),
        "update_frequency": section(
            None,
            "Recomposed on every catalog refresh from the live component feed states.",
            confidence="no-information",
        ),
        "detection_classification": section(
            {
                "primary_method": "mixed",
                "secondary_methods": [],
                "description": (
                    f"This merge inherits detection methodology from its "
                    f"{n} additive component feed(s). See each component "
                    f"for its specific detection method (honeypot, sandbox, "
                    f"reputation aggregation, etc.). Component feeds: "
                    f"{sources_csv}."
                    + (
                        f" {m} subtractive component feed(s) ({excludes_csv}) "
                        f"remove their entries from the result and therefore "
                        f"shape what this merge ultimately classifies."
                        if excludes
                        else ""
                    )
                ),
            },
            "Methodology inherited from components.",
        ),
        "scope_and_intent": section(
            {
                "description": ed.get("long_description") or info or "Aggregate merge; see component feeds for scope.",
                "intended_for": ed.get("intended_for") or [],
                "not_intended_for": ed.get("not_intended_for") or [],
            },
            "Editorial copy plus YAML info.",
        ),
        "license": section(
            license_str,
            "Catalog-defined merge; component-feed licenses continue to apply.",
        ),
        "redistribution": section(
            {
                "allowed": redistributable,
                "commercial_use_allowed": None,
                "attribution_required": None,
                "terms": redistribution_terms,
            },
            redistribution_reasoning,
        ),
        "current_status": section(
            {
                "state": "active",
                "description": (
                    "Maintained by FireHOL as a public catalog merge. "
                    "Recomposed automatically on each catalog refresh from the "
                    "live state of the component feeds. Component feeds that "
                    "go offline or are archived are dropped from the merge by "
                    "the catalog engine."
                ),
                "successor": None,
                "announcement_date": None,
            },
            "Catalog-managed merge; lifecycle tied to component availability.",
        ),
        "community": community_section(
            {"awards": None, "criticism": None, "engagement": None},
            "Community signals belong to the component feeds, not to the catalog's merge layer.",
            confidence="no-information",
        ),
    }
    return out


# --- Discovery + dispatch -----------------------------------------------------


def discover() -> list[tuple[str, str, Path, dict]]:
    """Return list of (shape, feed_name, yaml_path, feed_cfg)."""
    items: list[tuple[str, str, Path, dict]] = []

    # sources/.../<file>.yaml — filter to maintainer: FireHOL
    for y in sorted(SOURCES_DIR.rglob("*.yaml")):
        try:
            data = yaml.safe_load(y.read_text())
        except (OSError, yaml.YAMLError) as e:
            print(f"WARN: parse error on {y.relative_to(REPO)}: {e}", file=sys.stderr)
            continue
        if not isinstance(data, dict):
            continue
        for fname, src in (data.get("sources") or {}).items():
            if not isinstance(src, dict):
                continue
            if src.get("maintainer") != "FireHOL":
                continue
            url = src.get("url")
            if isinstance(url, str) and url.startswith("internal://"):
                shape = "internal-baseline"
            elif "static" in src:
                shape = "static-curated"
            else:
                shape = "static-curated"  # default for no-url FireHOL sources
            items.append((shape, fname, y, src))

    # merges can live in configs/firehol/merges/ or beside grouped reference
    # source files, as critical_soft_akamai_edge_secondary historically did.
    for y in sorted(CONFIG_DIR.rglob("*.yaml")):
        try:
            data = yaml.safe_load(y.read_text())
        except (OSError, yaml.YAMLError) as e:
            print(f"WARN: parse error on {y.relative_to(REPO)}: {e}", file=sys.stderr)
            continue
        if not isinstance(data, dict):
            continue
        for fname, m in (data.get("merges") or {}).items():
            if not isinstance(m, dict):
                continue
            items.append(("merge", fname, y, m))

    return items


def build(shape: str, feed_name: str, cfg: dict, yaml_path: Path) -> dict:
    if shape == "merge":
        return build_merge(feed_name, cfg, yaml_path)
    if shape == "internal-baseline":
        return build_internal_baseline(feed_name, cfg, yaml_path)
    return build_static_curated(feed_name, cfg, yaml_path)


def write_output(feed_name: str, doc: dict, yaml_path: Path) -> Path:
    run_dir = OUT_BASE / feed_name / f"{UTC_STAMP}-static"
    run_dir.mkdir(parents=True, exist_ok=True)
    out_path = run_dir / "output.json"
    out_path.write_text(json.dumps(doc, indent=2))
    shutil.copy(yaml_path, run_dir / "source.yaml")
    (run_dir / "GENERATOR").write_text(GENERATOR_TAG + "\n")
    return out_path


def validate_one(out_path: Path) -> tuple[bool, str]:
    report_path = out_path.with_suffix(".validation-report.json")
    try:
        validator = load_validator()
        report = validator.validate(out_path)
        report_path.write_text(json.dumps(report, indent=2))
    except (OSError, RuntimeError, TypeError, ValueError) as e:
        return False, f"validator failed: {e}"
    try:
        report = json.loads(report_path.read_text())
    except (OSError, json.JSONDecodeError) as e:
        return False, f"validator report parse error: {e}"
    summary = report.get("summary") or {}
    schema_ok = bool(report.get("schema_valid"))
    deny = summary.get("denylist_violations_count", -1)
    ips = summary.get("ip_address_findings_count", -1)
    if schema_ok and deny == 0 and ips == 0:
        return True, "ok"
    parts = []
    if not schema_ok:
        parts.append(f"schema_invalid({report.get('schema_error')})")
    if deny:
        parts.append(f"denylist={deny}")
    if ips:
        parts.append(f"ips={ips}")
    return False, ", ".join(parts)


@lru_cache(maxsize=1)
def load_validator():
    spec = importlib.util.spec_from_file_location("feed_enrichment_validator", VALIDATOR)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot import {VALIDATOR}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--dry-run", action="store_true", help="Discover and print plan, write nothing")
    p.add_argument("--validate", action="store_true", help="Run validate-output.py on each generated file")
    args = p.parse_args()

    items = discover()
    print(f"Discovered {len(items)} FireHOL-maintained feeds:")
    by_shape: dict[str, list[str]] = {}
    for shape, name, _, _ in items:
        by_shape.setdefault(shape, []).append(name)
    for shape, names in sorted(by_shape.items()):
        print(f"  {shape}: {len(names)}")
        for n in names:
            print(f"    - {n}")
    if args.dry_run:
        print("\n(dry-run; no output written)")
        return 0

    print(f"\nGenerating to {OUT_BASE.relative_to(REPO)}/...")
    failures: list[tuple[str, str]] = []
    written: list[Path] = []
    for shape, name, ypath, cfg in items:
        doc = build(shape, name, cfg, ypath)
        out_path = write_output(name, doc, ypath)
        written.append(out_path)
        ok, detail = (True, "skipped")
        if args.validate:
            ok, detail = validate_one(out_path)
            if not ok:
                failures.append((name, detail))
        marker = "OK" if ok else "FAIL"
        print(f"  [{marker}] {name}  -> {out_path.relative_to(REPO)}  {detail if not ok else ''}")

    print(f"\nWrote {len(written)} files.")
    if failures:
        print(f"\n{len(failures)} validation failures:")
        for n, d in failures:
            print(f"  - {n}: {d}")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
