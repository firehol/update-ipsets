#!/usr/bin/env python3
# pyright: reportMissingImports=false, reportMissingModuleSource=false
"""
Project validated feed-enrichment runs into public YAML enrichment blocks.

This is the bridge between the full agent output under
`.local/agents/feed-enrichment/<feed>/<UTC>/output.json` and the committed
`sources.<feed>.enrichment` YAML block.

Commands:

  project OUTPUT.json
      Print the sanitized public projection for one full agent output.

  hygiene [--all | --feeds a,b,c | OUTPUT.json ...]
      Report public markdown/prose hygiene findings. This implements the
      D21 wall-of-text rules from SOW-0014.

  embed (--all | --feeds a,b,c) [--write]
      Dry-run or write `enrichment:` into per-feed YAML files. The command
      refuses multi-source YAMLs; split those files first per SOW-0014 Step 0.

  delta (--all | --feeds a,b,c) [--json REPORT.json] [--markdown REPORT.md]
      Compare selected public projections against duplicated engine-config
      fields in the per-feed YAML and write a manual correction report.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

try:
    import jsonschema
except ImportError:
    print("ERROR: jsonschema not installed. Install with: pip install jsonschema", file=sys.stderr)
    sys.exit(2)

try:
    from ruamel.yaml import YAML
except ImportError:
    print("ERROR: ruamel.yaml not installed. Install with: pip install ruamel.yaml", file=sys.stderr)
    sys.exit(2)


REPO_ROOT = Path(__file__).resolve().parent.parent
ENRICH_DIR = REPO_ROOT / ".local" / "agents" / "feed-enrichment"
CONFIG_DIR = REPO_ROOT / "configs" / "firehol"
PUBLIC_SCHEMA = REPO_ROOT / "agents" / "schemas" / "feed-enrichment-public.schema.json"

INLINE_MARKDOWN_FIELDS = {
    "short_description",
    "long_description",
    "roles[].notes",
    "derivation.description",
    "derivation.source_feeds[].notes",
    "listing_policy.summary",
    "listing_policy.criteria[]",
    "unlisting_policy.summary",
    "unlisting_policy.criteria[]",
    "unlist_request.instructions",
    "update_frequency.human_readable",
    "detection_classification.description",
    "scope_and_intent.description",
    "scope_and_intent.intended_for[]",
    "scope_and_intent.not_intended_for[]",
    "redistribution.terms",
    "redistribution.attribution_required",
    "current_status.description",
    "community.awards",
    "community.criticism",
    "community.engagement",
}

HTML_RE = re.compile(r"</?[a-zA-Z][^>]*>")
HEADING_RE = re.compile(r"(?m)^\s{0,3}#{1,6}\s+")
SENTENCE_RE = re.compile(r"(?<=[.!?])\s+")
MONTHLY_FREQUENCY_RE = re.compile(r"\b(monthly|calendar month|once a month|per month|first of each month)\b", re.IGNORECASE)


@dataclass(frozen=True)
class FeedRun:
    feed: str
    run_dir: Path
    output_path: Path
    report_path: Path
    static: bool


def load_json(path: Path) -> Any:
    return json.loads(path.read_text())


def claim_value(data: dict[str, Any], key: str) -> Any:
    claim = data.get(key)
    if claim is None:
        return None
    if not isinstance(claim, dict) or "value" not in claim:
        raise ValueError(f"{key}: expected claim object with value")
    return claim.get("value")


def normalize_public_update_frequency(value: Any) -> Any:
    if not isinstance(value, dict):
        return value
    if value.get("frequency") != "1m":
        return value
    human = str(value.get("human_readable") or "")
    if not MONTHLY_FREQUENCY_RE.search(human):
        return value
    normalized = dict(value)
    normalized["frequency"] = "30d"
    return normalized


def project_public(data: dict[str, Any]) -> dict[str, Any]:
    evidence = data.get("evidence") or []
    update_frequency = normalize_public_update_frequency(claim_value(data, "update_frequency"))
    return {
        "enrichment_schema_version": data.get("enrichment_schema_version"),
        "run_at": data.get("run_at"),
        "official_name": claim_value(data, "official_name"),
        "official_url": claim_value(data, "official_url"),
        "short_description": claim_value(data, "short_description"),
        "long_description": claim_value(data, "long_description"),
        "roles": claim_value(data, "roles") or [],
        "derivation": claim_value(data, "derivation") or {"type": "unknown", "description": ""},
        "listing_policy": claim_value(data, "listing_policy"),
        "unlisting_policy": claim_value(data, "unlisting_policy"),
        "unlist_request": claim_value(data, "unlist_request"),
        "update_frequency": update_frequency,
        "detection_classification": claim_value(data, "detection_classification")
        or {"primary_method": "unknown", "description": ""},
        "scope_and_intent": claim_value(data, "scope_and_intent"),
        "license": claim_value(data, "license"),
        "redistribution": claim_value(data, "redistribution") or {},
        "current_status": claim_value(data, "current_status") or {"state": "unknown", "description": ""},
        "community": claim_value(data, "community") or {"awards": None, "criticism": None, "engagement": None},
        "sources_consulted": [
            {
                "url": item.get("url"),
                "document_date": item.get("document_date"),
                "validation_date": item.get("validation_date"),
            }
            for item in evidence
            if isinstance(item, dict) and item.get("url")
        ],
    }


def validate_public(doc: dict[str, Any]) -> None:
    jsonschema.validate(doc, load_json(PUBLIC_SCHEMA))


RECOVERABLE_PUBLIC_ERRORS = (
    OSError,
    json.JSONDecodeError,
    ValueError,
    KeyError,
    TypeError,
    jsonschema.ValidationError,
    jsonschema.SchemaError,
)


def paragraph_count(text: str) -> list[str]:
    return [p.strip() for p in re.split(r"\n\s*\n", text.strip()) if p.strip()]


def sentence_count(text: str) -> int:
    stripped = text.strip()
    if not stripped:
        return 0
    return len([s for s in SENTENCE_RE.split(stripped) if s.strip()])


def add_markdown_string(out: list[tuple[str, str]], path: str, value: Any) -> None:
    if isinstance(value, str) and value.strip():
        out.append((path, value))


def add_roles_markdown(out: list[tuple[str, str]], roles: Any) -> None:
    for idx, role in enumerate(roles or []):
        add_markdown_string(out, f"roles[{idx}].notes", role.get("notes") if isinstance(role, dict) else None)


def add_derivation_markdown(out: list[tuple[str, str]], derivation: Any) -> None:
    if not isinstance(derivation, dict):
        return
    add_markdown_string(out, "derivation.description", derivation.get("description"))
    for idx, source in enumerate(derivation.get("source_feeds") or []):
        add_markdown_string(
            out,
            f"derivation.source_feeds[{idx}].notes",
            source.get("notes") if isinstance(source, dict) else None,
        )


def add_policy_markdown(out: list[tuple[str, str]], name: str, section: Any) -> None:
    if not isinstance(section, dict):
        return
    add_markdown_string(out, f"{name}.summary", section.get("summary"))
    for idx, item in enumerate(section.get("criteria") or []):
        add_markdown_string(out, f"{name}.criteria[{idx}]", item)


def add_scope_markdown(out: list[tuple[str, str]], scope: Any) -> None:
    if not isinstance(scope, dict):
        return
    add_markdown_string(out, "scope_and_intent.description", scope.get("description"))
    for key in ("intended_for", "not_intended_for"):
        for idx, item in enumerate(scope.get(key) or []):
            add_markdown_string(out, f"scope_and_intent.{key}[{idx}]", item)


def add_optional_object_fields(out: list[tuple[str, str]], prefix: str, value: Any, fields: tuple[str, ...]) -> None:
    if not isinstance(value, dict):
        return
    for field in fields:
        add_markdown_string(out, f"{prefix}.{field}", value.get(field))


def iter_markdown_strings(doc: dict[str, Any]) -> list[tuple[str, str]]:
    out: list[tuple[str, str]] = []
    add_markdown_string(out, "short_description", doc.get("short_description"))
    add_markdown_string(out, "long_description", doc.get("long_description"))
    add_roles_markdown(out, doc.get("roles"))
    add_derivation_markdown(out, doc.get("derivation"))
    add_policy_markdown(out, "listing_policy", doc.get("listing_policy"))
    add_policy_markdown(out, "unlisting_policy", doc.get("unlisting_policy"))
    add_optional_object_fields(out, "unlist_request", doc.get("unlist_request"), ("instructions",))
    add_optional_object_fields(out, "update_frequency", doc.get("update_frequency"), ("human_readable",))
    add_optional_object_fields(
        out,
        "detection_classification",
        doc.get("detection_classification"),
        ("description",),
    )
    add_scope_markdown(out, doc.get("scope_and_intent"))
    add_optional_object_fields(out, "redistribution", doc.get("redistribution"), ("terms", "attribution_required"))
    add_optional_object_fields(out, "current_status", doc.get("current_status"), ("description",))
    add_optional_object_fields(out, "community", doc.get("community"), ("awards", "criticism", "engagement"))
    return out


def prose_hygiene_findings(feed: str, doc: dict[str, Any], source: str) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []
    for path, text in iter_markdown_strings(doc):
        if HTML_RE.search(text):
            findings.append({"feed": feed, "source": source, "field": path, "rule": "raw_html", "detail": "raw HTML tag"})
        if HEADING_RE.search(text):
            findings.append({"feed": feed, "source": source, "field": path, "rule": "markdown_heading", "detail": "heading in inline-markdown field"})
        paragraphs = paragraph_count(text)
        for idx, paragraph in enumerate(paragraphs):
            chars = len(paragraph)
            sentences = sentence_count(paragraph)
            if chars > 650:
                findings.append({
                    "feed": feed,
                    "source": source,
                    "field": path,
                    "rule": "paragraph_too_long",
                    "paragraph": idx + 1,
                    "chars": chars,
                    "sentences": sentences,
                })
            if sentences > 4:
                findings.append({
                    "feed": feed,
                    "source": source,
                    "field": path,
                    "rule": "paragraph_too_many_sentences",
                    "paragraph": idx + 1,
                    "chars": chars,
                    "sentences": sentences,
                })
        if path == "long_description" and len(text) > 900 and len(paragraphs) == 1:
            findings.append({
                "feed": feed,
                "source": source,
                "field": path,
                "rule": "long_description_single_block",
                "chars": len(text),
                "sentences": sentence_count(text),
            })
    return findings


def clean_report(report_path: Path, static: bool) -> bool:
    try:
        report = load_json(report_path)
    except (OSError, json.JSONDecodeError):
        return False
    summary = report.get("summary") or {}
    if report.get("schema_valid") is not True:
        return False
    if summary.get("denylist_violations_count", 1) != 0:
        return False
    if summary.get("ip_address_findings_count", 1) != 0:
        return False
    if summary.get("roles_count", 0) < 1:
        return False
    if not static and summary.get("evidence_count", 0) < 3:
        return False
    return True


def latest_feed_run(feed: str) -> FeedRun:
    feed_dir = ENRICH_DIR / feed
    runs = sorted([p for p in feed_dir.iterdir() if p.is_dir()]) if feed_dir.exists() else []
    if not runs:
        raise FileNotFoundError(f"no enrichment runs for {feed}")
    run_dir = runs[-1]
    output_path = run_dir / "output.json"
    if not output_path.exists():
        raise FileNotFoundError(f"missing output.json for latest run of {feed}: {run_dir}")
    report_path = run_dir / "validation-report.json"
    static = False
    if not report_path.exists():
        report_path = run_dir / "output.validation-report.json"
        static = True
    if not report_path.exists():
        raise FileNotFoundError(f"missing validation report for latest run of {feed}: {run_dir}")
    if not clean_report(report_path, static=static):
        raise ValueError(f"latest run for {feed} is not validator-clean: {report_path}")
    return FeedRun(feed=feed, run_dir=run_dir, output_path=output_path, report_path=report_path, static=static)


def all_feed_names() -> list[str]:
    if not ENRICH_DIR.exists():
        return []
    return sorted([p.name for p in ENRICH_DIR.iterdir() if p.is_dir()])


def parse_feeds(raw: str | None) -> list[str]:
    if not raw:
        return []
    return sorted([part.strip() for part in raw.split(",") if part.strip()])


def locate_yaml(feed: str) -> tuple[Path, Any, Any, str]:
    yaml = YAML()
    yaml.preserve_quotes = True
    for path in sorted(CONFIG_DIR.rglob("*.yaml")):
        with path.open() as f:
            doc = yaml.load(f)
        for section in ("sources", "merges"):
            entries = (doc or {}).get(section)
            if isinstance(entries, dict) and feed in entries:
                return path, doc, yaml, section
    raise FileNotFoundError(f"feed {feed} not found under {CONFIG_DIR}")


def write_enrichment(feed: str, projection: dict[str, Any], write: bool) -> dict[str, Any]:
    path, doc, yaml, section = locate_yaml(feed)
    entries = (doc or {}).get(section)
    if not isinstance(entries, dict) or feed not in entries:
        raise ValueError(f"{path}: no {section}.{feed}")
    if len(entries) != 1:
        raise ValueError(f"{path}: contains {len(entries)} {section}; split multi-entry YAML before embedding")
    if write:
        entries[feed]["enrichment"] = projection
        with path.open("w") as f:
            yaml.dump(doc, f)
    return {"feed": feed, "yaml": str(path.relative_to(REPO_ROOT)), "section": section, "write": write}


DELTA_FIELDS = ("maintainer", "maintainer_url", "frequency", "license", "redistributable")
DURATION_RE = re.compile(r"^(\d+)([mhdw])$")
ACCEPTED_DISPLAY_CHOICE_DELTAS = {
    ("cleanmx_phishing", "maintainer"): {("clean mx de", "netpilot")},
    ("cleanmx_phishing", "maintainer_url"): {("clean-mx.de", "netpilot.de")},
    ("cleanmx_viruses", "maintainer"): {("clean mx de", "net4sec ug")},
    ("cleanmx_viruses", "maintainer_url"): {("clean-mx.de", "net4sec.com")},
    ("data_shield", "maintainer_url"): {("gitlab.com/duggytuxy", "github.com/duggytuxy")},
    ("data_shield_critical", "maintainer_url"): {("gitlab.com/duggytuxy", "github.com/duggytuxy")},
    ("misp_sinkholes", "maintainer"): {("misp project", "alexandre dulaunoy")},
    ("opendbl_bruteforce", "maintainer"): {("opendbl", "fnutt consulting")},
    ("opendbl_bruteforce", "maintainer_url"): {("opendbl.net", "fnuttconsulting.com")},
    ("provider_context_gcp_cloud", "maintainer"): {("google cloud", "google")},
    ("provider_context_linode_geofeed", "maintainer"): {("akamai cloud computing", "akamai connected cloud")},
    ("provider_context_vultr_geofeed", "maintainer"): {("vultr", "constant company")},
    ("rutgers_drop", "maintainer"): {
        ("rutgers university computer science", "rutgers laboratory for computer science research")
    },
    ("stratosphere_aip_prioritize", "maintainer"): {("stratosphere laboratory", "stratosphere research laboratory")},
}


def json_scalar(value: Any) -> Any:
    if value is None or isinstance(value, (str, int, float, bool)):
        return value
    return str(value)


def maintainer_role(projection: dict[str, Any]) -> dict[str, Any] | None:
    roles = projection.get("roles") or []
    for role in roles:
        if isinstance(role, dict) and role.get("role") == "maintainer":
            return role
    for role in roles:
        if isinstance(role, dict):
            return role
    return None


def duration_to_minutes(value: str | None) -> int | None:
    if not value:
        return None
    match = DURATION_RE.match(value)
    if not match:
        return None
    amount = int(match.group(1))
    unit = match.group(2)
    factor = {"m": 1, "h": 60, "d": 1440, "w": 10080}[unit]
    return amount * factor


def projection_config_values(projection: dict[str, Any]) -> dict[str, Any]:
    role = maintainer_role(projection)
    update_frequency = projection.get("update_frequency") or {}
    redistribution = projection.get("redistribution") or {}
    frequency = None
    if isinstance(update_frequency, dict):
        frequency = duration_to_minutes(update_frequency.get("frequency"))
    return {
        "maintainer": role.get("name") if role else None,
        "maintainer_url": role.get("official_url") if role else None,
        "frequency": frequency,
        "license": projection.get("license"),
        "redistributable": redistribution.get("allowed") if isinstance(redistribution, dict) else None,
    }


def projection_config_candidates(projection: dict[str, Any]) -> dict[str, list[Any]]:
    values = projection_config_values(projection)
    candidates = {field: [value] for field, value in values.items()}
    candidates["maintainer"].append(projection.get("official_name"))
    candidates["maintainer_url"].append(projection.get("official_url"))
    candidates["maintainer"].extend(url_identity_candidates(projection.get("official_url")))
    candidates["maintainer"].extend(maintainer_identity_parts(projection.get("official_name")))
    for role in projection.get("roles") or []:
        if not isinstance(role, dict):
            continue
        role_name = role.get("name")
        candidates["maintainer"].append(role_name)
        candidates["maintainer"].extend(maintainer_identity_parts(role_name))
        candidates["maintainer_url"].append(role.get("official_url"))
        candidates["maintainer"].extend(url_identity_candidates(role.get("official_url")))
    return candidates


def normalize_url(value: Any) -> str | None:
    if value is None:
        return None
    text = str(value).strip()
    if not text:
        return None
    parsed = urlparse(text)
    host = parsed.netloc.lower()
    path = parsed.path.rstrip("/")
    if host.startswith("www."):
        host = host[4:]
    if host in {"github.com", "gitlab.com"}:
        parts = [part for part in path.split("/") if part]
        path = "/" + parts[0].lower() if parts else ""
        if host:
            return host + path
    else:
        host = root_domain(host)
        if host:
            return host
    return text.rstrip("/").lower()


def normalize_string(value: Any) -> str | None:
    if value is None:
        return None
    text = re.sub(r"\s+", " ", str(value).strip())
    return text.casefold() if text else None


def root_domain(host: str) -> str:
    parts = [part for part in host.split(".") if part]
    if len(parts) < 2:
        return host
    return ".".join(parts[-2:])


def url_identity_candidates(value: Any) -> list[str]:
    if value is None:
        return []
    parsed = urlparse(str(value).strip())
    host = parsed.netloc.lower()
    if host.startswith("www."):
        host = host[4:]
    if host in {"github.com", "gitlab.com"}:
        parts = [part for part in parsed.path.split("/") if part]
        return [parts[0]] if parts else []
    root = root_domain(host)
    out = []
    for item in (host, root):
        if not item:
            continue
        out.append(item)
        parts = item.split(".")
        if len(parts) > 1:
            out.append(parts[0])
    return out


def maintainer_identity_parts(value: Any) -> list[str]:
    if value is None:
        return []
    return [
        part.strip()
        for part in re.split(r"\s*/\s*|\s+\+\s+|\s+\band\b\s+", str(value))
        if part.strip()
    ]


def normalize_maintainer(value: Any) -> str | None:
    text = normalize_string(value)
    if text is None:
        return None
    compact = text.lstrip("@")
    compact = compact.replace("&amp;", "&")
    compact = re.sub(r"\([^)]*\)", " ", compact)
    compact = re.sub(r"\b(the|inc|inc\.|llc|ltd|gmbh|nfp|group|systems)\b", " ", compact)
    compact = re.sub(r"\bcatalog project\b", " ", compact)
    compact = re.sub(r"[^a-z0-9]+", " ", compact).strip()
    aliases = {
        "bitwire it": "bitwire",
        "cleantalk": "cleantalk",
        "criticalpathsecurity": "critical path security",
        "dataplane org": "dataplane",
        "dronebl org": "dronebl",
        "dronebl team": "dronebl",
        "dronebl volunteer team": "dronebl",
        "firehol": "firehol",
        "github": "github",
        "i blocklist": "iblocklist",
        "iblocklist": "iblocklist",
        "iblocklist com": "iblocklist",
        "maxmind": "maxmind",
        "viriback": "viriback",
    }
    return aliases.get(compact, compact)


LICENSE_ALIASES = {
    "abuseipdb-terms-of-service": "abuseipdb-terms",
    "abuseipdb-terms-of-service-see-https-www.abuseipdb.com-legal": "abuseipdb-terms",
    "caida-acceptable-use-agreement": "caida-public-aua",
    "caida-acceptable-use-agreement-public-aua": "caida-public-aua",
    "cc-0-creative-commons-zero": "cc0-1.0",
    "cc-by-4.0": "cc-by-4.0",
    "cc-by-nc-sa-4.0": "cc-by-nc-sa-4.0",
    "cc-by-nc-sa-4.0-creative-commons-attribution-noncommercial-sharealike-4.0-international": (
        "cc-by-nc-sa-4.0"
    ),
    "cc0-1.0": "cc0-1.0",
    "cc0-1.0-misp-warninglists": "cc0-1.0",
    "cc0-1.0-universal": "cc0-1.0",
    "cc0-1.0-universal-cc0-1.0-public-domain-dedication": "cc0-1.0",
    "cc0-1.0-universal-public-domain-dedication": "cc0-1.0",
    "creative-commons-attribution-4.0-international-cc-by-4.0": "cc-by-4.0",
    "creative-commons-attribution-noncommercial-sharealike-4.0-international": "cc-by-nc-sa-4.0",
    "creative-commons-attribution-noncommercial-sharealike-4.0-international-cc-by-nc-sa-4.0": (
        "cc-by-nc-sa-4.0"
    ),
    "gnu-general-public-license-v3-gpl-3.0": "gpl-3.0",
    "gnu-general-public-license-v3.0-gpl-3.0": "gpl-3.0",
    "gnu-gplv3": "gpl-3.0",
    "gpl-3.0": "gpl-3.0",
    "gplv3": "gpl-3.0",
    "mit": "mit",
    "mit-license": "mit",
    "pddl-v1.0-public-domain": "pddl-1.0",
    "pddl-v1.0-public-domain-dedication-and-license": "pddl-1.0",
    "provider-published-public-ip-ranges": "public-feed",
    "provider-published-rfc-8805-geofeed": "public-feed",
    "provider-published-support-documentation": "public-feed",
    "public-feed": "public-feed",
    "public-no-stated-license": "public-feed",
    "public-no-stated-license-firehol-catalog-merge": "public-feed",
    "the-unlicense": "unlicense",
    "the-unlicense-derived-from-ipsum-bitwire-mirror-has-no-license-file": "unlicense",
    "the-unlicense-public-domain": "unlicense",
    "the-unlicense-public-domain-dedication": "unlicense",
    "unlicense": "unlicense",
    "unlicense-public-domain": "unlicense",
}

IBLOCKLIST_LICENSES = {
    "personal-non-commercial-only",
    "personal-non-commercial-use-only",
    "personal-use-only",
    "personal-use-only-non-commercial",
}


def compact_license(value: str) -> str:
    compact = value.replace("—", "-").replace("–", "-").replace("_", "-").replace("/", "-")
    compact = re.sub(r"[^a-z0-9.+-]+", "-", compact).strip("-")
    return re.sub(r"-+", "-", compact)


def matches_iblocklist_license(compact: str, feed_name: str) -> bool:
    return feed_name.startswith("iblocklist_") and (
        "iblocklist" in compact or "i-blocklist" in compact or compact in IBLOCKLIST_LICENSES
    )


def matches_dataplane_license(compact: str, feed_name: str) -> bool:
    return feed_name.startswith("dataplane_") and compact.startswith(
        ("dataplane.org-", "non-commercial-use-only")
    )


def matches_dronebl_license(compact: str, feed_name: str) -> bool:
    return feed_name.startswith("dronebl_") and (
        "dronebl" in compact
        or compact.startswith(("bsd-style", "free-for-commercial-and-non-commercial-use"))
        or compact in {"bsd-style-license", "mit-license", "public-no-stated-license"}
    )


def matches_stopforumspam_license(compact: str, feed_name: str) -> bool:
    return feed_name.startswith("stopforumspam") and (
        "cc-by-nc-nd-3.0" in compact or "attribution-noncommercial-noderivatives-3.0" in compact
    )


def matches_geolite_license(compact: str) -> bool:
    has_eula = "eula" in compact or "end-user-license-agreement" in compact
    has_cc = "cc-by-sa-4.0" in compact or "creative-commons-attribution-sharealike-4.0" in compact
    return compact.startswith("geolite") and has_eula and has_cc


def feed_specific_license_alias(compact: str, feed_name: str) -> str | None:
    rules = (
        (matches_iblocklist_license, "iblocklist-tos"),
        (matches_dataplane_license, "dataplane-noncommercial-redistribution-prohibited"),
        (matches_dronebl_license, "dronebl-community-data"),
        (matches_stopforumspam_license, "cc-by-nc-nd-3.0-modified"),
    )
    for matcher, alias in rules:
        if matcher(compact, feed_name):
            return alias
    return exact_feed_license_alias(compact, feed_name)


def exact_feed_license_alias(compact: str, feed_name: str) -> str | None:
    exact_rules = {
        "botscout": ("botscout-terms-of-service", "botscout-tos"),
        "jamesbrine_bruteforce": ("tlp-white", "jamesbrine-tlp-white-noncommercial"),
    }
    if feed_name in exact_rules and exact_rules[feed_name][0] in compact:
        return exact_rules[feed_name][1]
    if feed_name == "greensnow" and compact.startswith("reproduction-or-republication"):
        return "greensnow-reproduction-prohibited"
    if feed_name == "vxvault" and compact.startswith("copyleft") and "no-rights-reserved" in compact:
        return "vxvault-copyleft"
    if feed_name == "griffinguard" and compact.startswith(
        ("griffinguard-security-research-monitoring-only", "restricted-security-research-monitoring-only")
    ):
        return "griffinguard-restricted-security-research"
    return None


def public_pattern_license_alias(compact: str) -> str | None:
    if compact.startswith("unknown-no-license") or compact == "unknown":
        return "unknown"
    if compact.startswith(("free-no-restrictions-stated", "provider-published-public-ip-ranges-governed-by-fastly")):
        return "public-feed"
    if compact.startswith("no-stated-license") and "digitalocean.com-geo-google.csv" in compact:
        return "public-feed"
    return None


def vendor_pattern_license_alias(compact: str, feed_name: str) -> str | None:
    if matches_geolite_license(compact):
        return "geolite-eula-cc-by-sa-4.0"
    if compact.startswith("maxmind-geoip-end-user-license-agreement"):
        return "maxmind-geoip-eula"
    if feed_name.startswith("cleantalk") and (
        "cleantalk-license-agreement" in compact
        or "proprietary-commercial-license" in compact
        or "non-transferable-proprietary-license" in compact
    ):
        return "cleantalk-proprietary"
    if feed_name.startswith("threatview_") and compact == "all-rights-reserved-by-threatview.io":
        return "threatview-all-rights-reserved"
    return None


def pattern_license_alias(compact: str, feed_name: str) -> str | None:
    return public_pattern_license_alias(compact) or vendor_pattern_license_alias(compact, feed_name)


def normalize_license(value: Any, feed: str | None = None) -> str | None:
    text = normalize_string(value)
    if text is None:
        return None
    compact = compact_license(text)
    feed_name = feed or ""
    return (
        feed_specific_license_alias(compact, feed_name)
        or pattern_license_alias(compact, feed_name)
        or LICENSE_ALIASES.get(compact)
        or text
    )


def normalize_field(field: str, value: Any, feed: str | None = None) -> Any:
    if field == "license":
        return normalize_license(value, feed)
    if field == "maintainer":
        return normalize_maintainer(value)
    if field == "maintainer_url":
        return normalize_url(value)
    if field == "frequency":
        if value is None or value == "":
            return None
        try:
            return int(value)
        except (TypeError, ValueError):
            return str(value).strip()
    if field == "redistributable":
        if value is None or value == "":
            return None
        if isinstance(value, bool):
            return value
        text = str(value).strip().casefold()
        if text in {"true", "yes", "1"}:
            return True
        if text in {"false", "no", "0"}:
            return False
        return text
    return normalize_string(value)


def maintainer_matches_candidate(value: Any, candidate_values: list[Any], feed: str) -> bool:
    normalized = normalize_maintainer(value)
    candidates = {
        normalize_field("maintainer", candidate, feed)
        for candidate in candidate_values
        if candidate is not None
    }
    if normalized is not None and normalized in candidates:
        return True
    parts = [normalize_maintainer(part) for part in maintainer_identity_parts(value)]
    return len(parts) > 1 and all(part in candidates for part in parts if part is not None)


def accepted_display_choice_delta(feed: str, field: str, yaml_value: Any, enrichment_value: Any) -> bool:
    accepted = ACCEPTED_DISPLAY_CHOICE_DELTAS.get((feed, field))
    if not accepted:
        return False
    normalized_yaml = normalize_field(field, yaml_value, feed)
    normalized_enrichment = normalize_field(field, enrichment_value, feed)
    return (normalized_yaml, normalized_enrichment) in accepted


def delta_findings(run: FeedRun, projection: dict[str, Any], fields: list[str]) -> list[dict[str, Any]]:
    path, doc, _yaml, section = locate_yaml(run.feed)
    entries = (doc or {}).get(section)
    if not isinstance(entries, dict) or run.feed not in entries:
        raise ValueError(f"{path}: no {section}.{run.feed}")
    cfg = entries[run.feed] or {}
    projected = projection_config_values(projection)
    candidates = projection_config_candidates(projection)
    findings: list[dict[str, Any]] = []
    for field in fields:
        enrichment_value = projected.get(field)
        if enrichment_value is None:
            continue
        yaml_value = cfg.get(field) if isinstance(cfg, dict) else None
        normalized_yaml = normalize_field(field, yaml_value, run.feed)
        if normalized_yaml == normalize_field(field, enrichment_value, run.feed):
            continue
        if any(
            normalized_yaml is not None and normalized_yaml == normalize_field(field, candidate, run.feed)
            for candidate in candidates.get(field, [])
            if candidate is not None
        ):
            continue
        if field == "maintainer" and maintainer_matches_candidate(yaml_value, candidates.get(field, []), run.feed):
            continue
        if accepted_display_choice_delta(run.feed, field, yaml_value, enrichment_value):
            continue
        findings.append({
            "feed": run.feed,
            "yaml": str(path.relative_to(REPO_ROOT)),
            "section": section,
            "field": field,
            "yaml_value": json_scalar(yaml_value),
            "enrichment_value": json_scalar(enrichment_value),
            "run": str(run.run_dir.relative_to(REPO_ROOT)),
        })
    return findings


def markdown_cell(value: Any, limit: int = 160) -> str:
    if value is None:
        text = ""
    elif isinstance(value, bool):
        text = "true" if value else "false"
    else:
        text = str(value)
    text = re.sub(r"\s+", " ", text).strip()
    if len(text) > limit:
        text = text[: limit - 1].rstrip() + "..."
    return text.replace("|", "\\|")


def write_delta_markdown(path: Path, report: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    findings = report["findings"]
    by_field: dict[str, list[dict[str, Any]]] = {}
    for item in findings:
        by_field.setdefault(item["field"], []).append(item)

    lines = [
        "# Feed Enrichment Config Delta Report",
        "",
        f"Generated: {report['generated_at']}",
        f"Scanned runs: {report['scanned']}",
        f"Findings: {len(findings)}",
        f"Errors: {len(report['errors'])}",
        "",
        "## Summary",
        "",
        "| Field | Findings |",
        "| --- | ---: |",
    ]
    for field in sorted(by_field):
        lines.append(f"| `{field}` | {len(by_field[field])} |")
    lines.append("")

    for field in sorted(by_field):
        lines.extend([
            f"## {field}",
            "",
            "| Feed | YAML value | Enrichment value | File |",
            "| --- | --- | --- | --- |",
        ])
        for item in sorted(by_field[field], key=lambda x: (x["feed"], x["yaml"])):
            lines.append(
                f"| `{markdown_cell(item['feed'])}` | {markdown_cell(item['yaml_value'])} | "
                f"{markdown_cell(item['enrichment_value'])} | `{markdown_cell(item['yaml'])}` |"
            )
        lines.append("")

    if report["errors"]:
        lines.extend(["## Errors", "", "| Feed | Error |", "| --- | --- |"])
        for item in report["errors"]:
            lines.append(f"| `{markdown_cell(item['feed'])}` | {markdown_cell(item['error'], limit=260)} |")
        lines.append("")

    path.write_text("\n".join(lines))


def command_project(args: argparse.Namespace) -> int:
    data = load_json(args.output_json)
    projection = project_public(data)
    validate_public(projection)
    print(json.dumps(projection, indent=2, sort_keys=False))
    return 0


def selected_runs(args: argparse.Namespace) -> list[FeedRun]:
    if getattr(args, "all", False):
        feeds = all_feed_names()
    else:
        feeds = parse_feeds(getattr(args, "feeds", None))
    if feeds:
        return [latest_feed_run(feed) for feed in feeds]
    runs = []
    for path in getattr(args, "outputs", []) or []:
        data = load_json(path)
        feed = data.get("feed_name") or path.parent.parent.name
        runs.append(FeedRun(feed=feed, run_dir=path.parent, output_path=path, report_path=Path(), static=False))
    if not runs:
        raise ValueError("select --all, --feeds, or one or more output.json paths")
    return runs


def command_hygiene(args: argparse.Namespace) -> int:
    if args.embedded:
        return command_hygiene_embedded(args)

    runs = selected_runs(args)
    all_findings: list[dict[str, Any]] = []
    schema_failures: list[dict[str, str]] = []
    for run in runs:
        data = load_json(run.output_path)
        projection = project_public(data)
        try:
            validate_public(projection)
        except (jsonschema.ValidationError, jsonschema.SchemaError) as e:
            schema_failures.append({"feed": run.feed, "source": str(run.output_path), "error": str(e)})
            continue
        all_findings.extend(prose_hygiene_findings(run.feed, projection, str(run.output_path.relative_to(REPO_ROOT))))

    if args.json:
        args.json.parent.mkdir(parents=True, exist_ok=True)
        args.json.write_text(json.dumps({"schema_failures": schema_failures, "findings": all_findings}, indent=2))

    print(f"scanned={len(runs)} schema_failures={len(schema_failures)} hygiene_findings={len(all_findings)}")
    by_rule: dict[str, int] = {}
    for item in all_findings:
        by_rule[item["rule"]] = by_rule.get(item["rule"], 0) + 1
    for rule, count in sorted(by_rule.items()):
        print(f"{rule}\t{count}")
    if all_findings:
        print()
        for item in all_findings[:50]:
            print(f"{item['feed']}\t{item['field']}\t{item['rule']}\t{item.get('chars', '')}\t{item.get('sentences', '')}")
        if len(all_findings) > 50:
            print(f"... {len(all_findings) - 50} more finding(s)")
    if schema_failures:
        print()
        for item in schema_failures[:20]:
            print(f"schema_failure\t{item['feed']}\t{item['error']}", file=sys.stderr)
    return 1 if args.fail and (schema_failures or all_findings) else 0


def embedded_enrichments(feeds: list[str]) -> list[tuple[str, dict[str, Any], str]]:
    selected = set(feeds)
    yaml = YAML()
    yaml.preserve_quotes = True
    items: list[tuple[str, dict[str, Any], str]] = []
    for path in sorted(CONFIG_DIR.rglob("*.yaml")):
        with path.open() as f:
            doc = yaml.load(f)
        for section in ("sources", "merges"):
            entries = (doc or {}).get(section)
            if not isinstance(entries, dict):
                continue
            for feed, cfg in entries.items():
                if selected and feed not in selected:
                    continue
                if not isinstance(cfg, dict):
                    continue
                enrichment = cfg.get("enrichment")
                if isinstance(enrichment, dict):
                    items.append((feed, dict(enrichment), str(path.relative_to(REPO_ROOT))))
    return items


def command_hygiene_embedded(args: argparse.Namespace) -> int:
    feeds = parse_feeds(getattr(args, "feeds", None))
    if not args.all and not feeds:
        raise ValueError("embedded hygiene requires --all or --feeds")

    all_findings: list[dict[str, Any]] = []
    schema_failures: list[dict[str, str]] = []
    items = embedded_enrichments(feeds)
    for feed, projection, source in items:
        try:
            validate_public(projection)
        except (jsonschema.ValidationError, jsonschema.SchemaError) as e:
            schema_failures.append({"feed": feed, "source": source, "error": str(e)})
            continue
        all_findings.extend(prose_hygiene_findings(feed, projection, source))

    if args.json:
        args.json.parent.mkdir(parents=True, exist_ok=True)
        args.json.write_text(json.dumps({"schema_failures": schema_failures, "findings": all_findings}, indent=2))

    print(f"scanned={len(items)} schema_failures={len(schema_failures)} hygiene_findings={len(all_findings)}")
    by_rule: dict[str, int] = {}
    for item in all_findings:
        by_rule[item["rule"]] = by_rule.get(item["rule"], 0) + 1
    for rule, count in sorted(by_rule.items()):
        print(f"{rule}\t{count}")
    if all_findings:
        print()
        for item in all_findings[:50]:
            print(f"{item['feed']}\t{item['field']}\t{item['rule']}\t{item.get('chars', '')}\t{item.get('sentences', '')}")
        if len(all_findings) > 50:
            print(f"... {len(all_findings) - 50} more finding(s)")
    if schema_failures:
        print()
        for item in schema_failures[:20]:
            print(f"schema_failure\t{item['feed']}\t{item['error']}", file=sys.stderr)
    return 1 if args.fail and (schema_failures or all_findings) else 0


def command_embed(args: argparse.Namespace) -> int:
    runs = selected_runs(args)
    report = {"generated_at": datetime.now(timezone.utc).isoformat(), "write": args.write, "items": [], "errors": []}
    for run in runs:
        try:
            data = load_json(run.output_path)
            projection = project_public(data)
            validate_public(projection)
            hygiene = prose_hygiene_findings(run.feed, projection, str(run.output_path.relative_to(REPO_ROOT)))
            item = write_enrichment(run.feed, projection, write=args.write)
            item["run"] = str(run.run_dir.relative_to(REPO_ROOT))
            item["hygiene_findings"] = hygiene
            report["items"].append(item)
            print(("wrote" if args.write else "would_write") + f"\t{run.feed}\t{item['yaml']}\t{item['section']}\thygiene={len(hygiene)}")
        except RECOVERABLE_PUBLIC_ERRORS as e:
            report["errors"].append({"feed": run.feed, "error": str(e)})
            print(f"error\t{run.feed}\t{e}", file=sys.stderr)
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(report, indent=2))
    if report["errors"]:
        return 1
    if args.fail_on_hygiene and any(item["hygiene_findings"] for item in report["items"]):
        return 1
    return 0


def command_delta(args: argparse.Namespace) -> int:
    runs = selected_runs(args)
    fields = [field.strip() for field in (args.fields or ",".join(DELTA_FIELDS)).split(",") if field.strip()]
    unknown = sorted(set(fields) - set(DELTA_FIELDS))
    if unknown:
        raise ValueError(f"unknown delta field(s): {', '.join(unknown)}")

    report = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "fields": fields,
        "scanned": len(runs),
        "findings": [],
        "errors": [],
    }
    for run in runs:
        try:
            data = load_json(run.output_path)
            projection = project_public(data)
            validate_public(projection)
            report["findings"].extend(delta_findings(run, projection, fields))
        except RECOVERABLE_PUBLIC_ERRORS as e:
            report["errors"].append({"feed": run.feed, "error": str(e)})
            print(f"error\t{run.feed}\t{e}", file=sys.stderr)

    if args.json:
        args.json.parent.mkdir(parents=True, exist_ok=True)
        args.json.write_text(json.dumps(report, indent=2))
    if args.markdown:
        write_delta_markdown(args.markdown, report)

    print(f"scanned={report['scanned']} findings={len(report['findings'])} errors={len(report['errors'])}")
    by_field: dict[str, int] = {}
    for item in report["findings"]:
        by_field[item["field"]] = by_field.get(item["field"], 0) + 1
    for field, count in sorted(by_field.items()):
        print(f"{field}\t{count}")
    if report["findings"]:
        print()
        for item in report["findings"][:80]:
            print(
                f"{item['field']}\t{item['feed']}\t"
                f"{item.get('yaml_value')!r}\t{item.get('enrichment_value')!r}\t{item['yaml']}"
            )
        if len(report["findings"]) > 80:
            print(f"... {len(report['findings']) - 80} more finding(s)")
    return 1 if report["errors"] else 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="command", required=True)

    project = sub.add_parser("project", help="print public projection for one output.json")
    project.add_argument("output_json", type=Path)
    project.set_defaults(func=command_project)

    hygiene = sub.add_parser("hygiene", help="report public prose hygiene findings")
    hygiene.add_argument("outputs", nargs="*", type=Path)
    hygiene.add_argument("--all", action="store_true")
    hygiene.add_argument("--feeds")
    hygiene.add_argument("--embedded", action="store_true", help="scan committed YAML enrichment blocks instead of local output.json files")
    hygiene.add_argument("--json", type=Path)
    hygiene.add_argument("--fail", action="store_true")
    hygiene.set_defaults(func=command_hygiene)

    embed = sub.add_parser("embed", help="dry-run or write enrichment blocks to YAML")
    group = embed.add_mutually_exclusive_group(required=True)
    group.add_argument("--all", action="store_true")
    group.add_argument("--feeds")
    embed.add_argument("--write", action="store_true")
    embed.add_argument("--report", type=Path)
    embed.add_argument("--fail-on-hygiene", action="store_true")
    embed.set_defaults(func=command_embed)

    delta = sub.add_parser("delta", help="report YAML config fields that differ from enrichment")
    group = delta.add_mutually_exclusive_group(required=True)
    group.add_argument("--all", action="store_true")
    group.add_argument("--feeds")
    delta.add_argument("--fields", help=f"comma-separated fields (default: {','.join(DELTA_FIELDS)})")
    delta.add_argument("--json", type=Path)
    delta.add_argument("--markdown", type=Path)
    delta.set_defaults(func=command_delta)
    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    try:
        return args.func(args)
    except RECOVERABLE_PUBLIC_ERRORS as e:
        print(f"ERROR: {e}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
