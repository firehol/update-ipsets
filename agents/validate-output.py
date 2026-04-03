#!/usr/bin/env python3
"""Post-run validator for feed-enrichment agent output (v2 schema).

Checks:
  1. JSON parses and validates against the v2 schema.
  2. Top-level evidence[].url and every URL anywhere in the document is
     denylist-clean (no firehol-self-references).
  3. PUBLIC fields contain no raw IPv4/IPv6 addresses. INTERNAL fields
     (maintainer_quotes[], assistant_reasoning, evidence[].description)
     are exempt — they are verification surfaces, not user-facing.

The set of PUBLIC field paths is defined explicitly below. Walking
arbitrary text fields and applying public rules to everything would
flag the internal verification fields incorrectly.

Exits 0 on clean output, non-zero with a JSON report on failure.

Usage:
    agents/validate-output.py path/to/output.json [--report report.json]
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

try:
    import jsonschema
except ImportError:
    print("ERROR: jsonschema not installed. Install with: pip install jsonschema", file=sys.stderr)
    sys.exit(2)


REPO_ROOT = Path(__file__).resolve().parent.parent
SCHEMA_PATH = REPO_ROOT / "agents" / "schemas" / "feed-enrichment.schema.json"

# Feedback-loop denylist — every URL the agent fetched or cites is checked.
DENYLIST_SUBSTRINGS = [
    "iplists.firehol.org",
    "firehol.org/ipsets",
    "github.com/firehol/",  # any repo under the firehol GitHub org
]

# IP-address detectors. The trailing `(?:/\d{1,3})?` catches CIDR notation.
IPV4_RE = re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}(?:/\d{1,2})?\b")
IPV6_RE = re.compile(r"\b(?:[0-9a-fA-F]{1,4}:){2,7}[0-9a-fA-F]{1,4}(?:/\d{1,3})?\b")

# Internal field names — IP rule does NOT apply to these. Anything else
# inside `value.*` or top-level text fields is treated as public.
INTERNAL_FIELD_KEYS = {"maintainer_quotes", "assistant_reasoning"}

# Evidence array's per-item `description` is also internal; URL is public-ish
# (exposed as Sources footer). We do not IP-scan evidence[].description.
EVIDENCE_INTERNAL_FIELDS = {"description"}


def denylist_violations(urls: list[str]) -> list[dict]:
    out = []
    for u in urls:
        for pat in DENYLIST_SUBSTRINGS:
            if pat in u:
                out.append({"url": u, "matched_pattern": pat})
                break
    return out


def collect_all_urls(obj, acc: list[str]) -> None:
    """Walk the document and gather every URL-like string we can find.
    Casts a wide net so denylist-checking covers evidence[], roles[].official_url,
    every URL field across sections, and any URL embedded in prose.
    """
    if isinstance(obj, str):
        # extract URLs from prose too (markdown links, raw URLs)
        for m in re.findall(r"https?://[^\s)<>\"']+", obj):
            acc.append(m.rstrip(".,;:)]}"))
    elif isinstance(obj, dict):
        for v in obj.values():
            collect_all_urls(v, acc)
    elif isinstance(obj, list):
        for item in obj:
            collect_all_urls(item, acc)


def walk_public_strings(obj, path: str, acc: list[tuple[str, str]], inside_evidence: bool = False) -> None:
    """Yield (jsonpath, string_value) pairs for every public string field.

    Excludes maintainer_quotes, assistant_reasoning, and evidence[].description
    because those are internal verification surfaces.

    Handles three node types:
      - string: record (path, value) and stop
      - dict: descend through each key, skipping internal-field keys
      - list: descend through each item; string items at list positions
        (e.g. listing_policy.criteria[]) are correctly captured because
        the recursion sees them as strings on the next call.
    """
    if isinstance(obj, str):
        acc.append((path, obj))
        return
    if isinstance(obj, dict):
        for k, v in obj.items():
            child_path = f"{path}.{k}"
            if k in INTERNAL_FIELD_KEYS:
                continue
            if inside_evidence and k in EVIDENCE_INTERNAL_FIELDS:
                continue
            walk_public_strings(v, child_path, acc, inside_evidence=(k == "evidence") or inside_evidence)
    elif isinstance(obj, list):
        for i, item in enumerate(obj):
            walk_public_strings(item, f"{path}[{i}]", acc, inside_evidence=inside_evidence)


def validate(output_path: Path) -> dict:
    report = {
        "output_path": str(output_path),
        "schema_valid": False,
        "schema_error": None,
        "denylist_violations": [],
        "ip_address_findings": [],
        "summary": {},
    }

    try:
        data = json.loads(output_path.read_text())
    except json.JSONDecodeError as e:
        report["schema_error"] = f"JSON parse error: {e}"
        return report

    try:
        schema = json.loads(SCHEMA_PATH.read_text())
        jsonschema.validate(data, schema)
        report["schema_valid"] = True
    except jsonschema.ValidationError as e:
        report["schema_error"] = {
            "message": e.message,
            "path": list(e.absolute_path),
        }
        # continue with other checks even if schema fails

    all_urls: list[str] = []
    collect_all_urls(data, all_urls)
    report["denylist_violations"] = denylist_violations(all_urls)

    text_findings: list[tuple[str, str]] = []
    walk_public_strings(data, "$", text_findings)
    for fpath, text in text_findings:
        for m in IPV4_RE.findall(text):
            report["ip_address_findings"].append({"path": fpath, "kind": "ipv4", "value": m})
        for m in IPV6_RE.findall(text):
            report["ip_address_findings"].append({"path": fpath, "kind": "ipv6", "value": m})

    # Content-presence: enough to claim a real research run
    evidence_count = len(data.get("evidence") or [])
    roles_count = len(((data.get("roles") or {}).get("value")) or [])

    report["summary"] = {
        "all_urls_count": len(all_urls),
        "evidence_count": evidence_count,
        "roles_count": roles_count,
        "denylist_violations_count": len(report["denylist_violations"]),
        "ip_address_findings_count": len(report["ip_address_findings"]),
    }
    return report


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("output_json", type=Path)
    p.add_argument("--report", type=Path, default=None)
    args = p.parse_args()

    if not args.output_json.exists():
        print(f"ERROR: {args.output_json} does not exist", file=sys.stderr)
        return 2

    report = validate(args.output_json)
    if args.report:
        args.report.write_text(json.dumps(report, indent=2))
    print(json.dumps(report, indent=2))

    ok = (
        report["schema_valid"]
        and not report["denylist_violations"]
        and not report["ip_address_findings"]
    )
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
