#!/usr/bin/env python3
"""Scan completed enrichment outputs for content-quality issues.

Surfaces classes of mistakes in PUBLIC fields that the schema/validator
do not catch:

  - snapshot_size      : numeric feed-size claims that will rot
  - temporal_snapshot  : date-fixed claims ("as of Nov 2025", "currently")
  - firehol_mention    : firehol/iplists.firehol references in public prose
  - editorial_verdict  : comparative/judgmental claims about the feed
  - future_promise     : forward-looking claims about maintainer behavior
  - count_of_sources   : numeric counts of upstream sources (lower priority)

Public fields = everything under `value.*` paths EXCEPT
maintainer_quotes[], assistant_reasoning, and evidence[].description.

Output:
  - terminal report grouped by issue class
  - JSON detail file under .local/agents/scan-public-content/<ts>/findings.json
"""

from __future__ import annotations

import json
import re
import sys
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
ENRICH_DIR = REPO_ROOT / ".local" / "agents" / "feed-enrichment"

INTERNAL_FIELD_KEYS = {"maintainer_quotes", "assistant_reasoning"}
EVIDENCE_INTERNAL_FIELDS = {"description"}

# ---- pattern definitions ----------------------------------------------------

MONTHS = r"(?:January|February|March|April|May|June|July|August|September|October|November|December|Jan|Feb|Mar|Apr|Jun|Jul|Aug|Sep|Sept|Oct|Nov|Dec)"

PATTERNS = {
    "snapshot_size": [
        # "850,000 IPs", "1.2 million addresses", "47K entries"
        re.compile(r"\b\d{1,3}(?:[,.]\d{3})+\s*(?:IPs?|IP addresses|addresses|entries|ranges|networks?|CIDRs?|hosts?|records?)\b", re.IGNORECASE),
        re.compile(
            r"\b\d+(?:\.\d+)?\s*(?:million|thousand|billion|M|K)\b\s*"
            r"(?:IPs?|IP addresses|addresses|entries|ranges|networks?|CIDRs?|hosts?|records?)?\b",
            re.IGNORECASE,
        ),
        re.compile(r"\bcontains?\s+(?:approximately|about|around|roughly)?\s*\d[\d,.]*\s*(?:IPs?|addresses|entries|ranges)\b", re.IGNORECASE),
        re.compile(r"\b(?:currently|now|today)\s+(?:lists?|contains?|has|holds?)\s+\d[\d,.]*\b", re.IGNORECASE),
    ],
    "temporal_snapshot": [
        # "as of November 2025", "since October 2025"
        re.compile(rf"\bas of\s+(?:{MONTHS}\s+)?\d{{4}}\b", re.IGNORECASE),
        re.compile(rf"\b(?:in|during)\s+{MONTHS}\s+20\d\d\b", re.IGNORECASE),
        re.compile(r"\b(?:currently|presently|at the moment|right now|these days)\b", re.IGNORECASE),
        re.compile(r"\b(?:last|past|recent|recently)\s+(?:week|month|quarter|year|days?)\b", re.IGNORECASE),
        re.compile(r"\b(?:within the )?(?:last|past)\s+\d+\s+(?:days?|weeks?|months?|years?)\b", re.IGNORECASE),
        re.compile(rf"\b{MONTHS}\s+20\d\d\b"),
        # "the feed file was observed with a current timestamp"
        re.compile(r"\bcurrent timestamp\b", re.IGNORECASE),
    ],
    "firehol_mention": [
        re.compile(r"\bfirehol\b", re.IGNORECASE),
        re.compile(r"iplists\.firehol", re.IGNORECASE),
    ],
    "editorial_verdict": [
        re.compile(r"\b(?:more|less)\s+(?:accurate|reliable|effective|comprehensive|useful|noisy|aggressive|conservative)\s+than\b", re.IGNORECASE),
        re.compile(r"\bbetter than\b", re.IGNORECASE),
        re.compile(r"\bworse than\b", re.IGNORECASE),
        re.compile(r"\bcompared (?:to|with) other\b", re.IGNORECASE),
        re.compile(r"\b(?:high|low)-quality\s+(?:feed|list|source)\b", re.IGNORECASE),
        re.compile(r"\bone of the (?:best|most|top|leading)\b", re.IGNORECASE),
    ],
    "future_promise": [
        re.compile(r"\bplans? to\b", re.IGNORECASE),
        re.compile(r"\bwill (?:be|continue|keep|maintain|expand|add|introduce)\b", re.IGNORECASE),
        re.compile(r"\b(?:upcoming|forthcoming|future)\s+(?:release|update|feature|change)\b", re.IGNORECASE),
        re.compile(r"\bis expected to\b", re.IGNORECASE),
    ],
    "count_of_sources": [
        # "30+ public feeds", "approximately 50 sources"
        re.compile(r"\b\d{2,}\+?\s+(?:public\s+)?(?:feeds?|blacklists?|sources?|threat\s+(?:intel|intelligence)\s+(?:feeds?|sources?))\b", re.IGNORECASE),
        re.compile(r"\b(?:approximately|about|around|roughly|over)\s+\d+\s+(?:feeds?|blacklists?|sources?)\b", re.IGNORECASE),
    ],
}

# Whitelist phrases — strings that MATCH the patterns but are factual and
# should NOT be flagged. "Since 2018" (no month) is permanent fact.
WHITELIST_FIXED = [
    re.compile(r"\bsince \d{4}\b", re.IGNORECASE),     # "since 2018" is permanent
    re.compile(r"\bestablished in \d{4}\b", re.IGNORECASE),
    re.compile(r"\bfounded in \d{4}\b", re.IGNORECASE),
    re.compile(r"\boperating since \d{4}\b", re.IGNORECASE),
]


def is_whitelisted(text: str, match_text: str) -> bool:
    """Check if a match is part of a whitelisted fixed-fact phrase."""
    for w in WHITELIST_FIXED:
        for m in w.finditer(text):
            if m.start() <= text.find(match_text) <= m.end():
                return True
    return False


def walk_public_strings(obj, path: str, acc: list, inside_evidence: bool = False):
    """Collect (jsonpath, string) tuples for every public string field."""
    if isinstance(obj, dict):
        for k, v in obj.items():
            child_path = f"{path}.{k}"
            if k in INTERNAL_FIELD_KEYS:
                continue
            if inside_evidence and k in EVIDENCE_INTERNAL_FIELDS:
                continue
            if isinstance(v, str):
                acc.append((child_path, v))
            else:
                walk_public_strings(v, child_path, acc,
                                    inside_evidence=(k == "evidence") or inside_evidence)
    elif isinstance(obj, list):
        for i, item in enumerate(obj):
            walk_public_strings(item, f"{path}[{i}]", acc,
                                inside_evidence=inside_evidence)


def scan_feed(feed_dir: Path):
    out_path = feed_dir / "output.json"
    if not out_path.exists():
        return None
    try:
        data = json.loads(out_path.read_text())
    except json.JSONDecodeError:
        return None
    feed_name = data.get("feed_name") or feed_dir.parent.name

    fields: list = []
    walk_public_strings(data, "$", fields)

    findings = defaultdict(list)
    for field_path, text in fields:
        for cls, patterns in PATTERNS.items():
            for p in patterns:
                for m in p.finditer(text):
                    snippet = m.group(0)
                    if cls == "temporal_snapshot" and is_whitelisted(text, snippet):
                        continue
                    findings[cls].append({
                        "field": field_path,
                        "match": snippet,
                        "context": _context(text, m.start(), m.end()),
                    })
    return {"feed": feed_name, "feed_dir": str(feed_dir.relative_to(REPO_ROOT)), "findings": dict(findings)}


def _context(text: str, start: int, end: int, span: int = 60) -> str:
    a = max(0, start - span)
    b = min(len(text), end + span)
    prefix = "..." if a > 0 else ""
    suffix = "..." if b < len(text) else ""
    return prefix + text[a:b].replace("\n", " ") + suffix


def latest_run(feed_dir: Path) -> Path | None:
    runs = sorted([d for d in feed_dir.iterdir() if d.is_dir()])
    return runs[-1] if runs else None


def main():
    if not ENRICH_DIR.exists():
        print(f"No enrichment dir at {ENRICH_DIR}", file=sys.stderr)
        return 2

    all_findings = []
    by_class: defaultdict = defaultdict(list)

    for feed_dir in sorted(ENRICH_DIR.iterdir()):
        if not feed_dir.is_dir():
            continue
        run = latest_run(feed_dir)
        if run is None:
            continue
        result = scan_feed(run)
        if result is None:
            continue
        all_findings.append(result)
        for cls, items in result["findings"].items():
            for item in items:
                by_class[cls].append({"feed": result["feed"], **item})

    # write detail JSON
    now = datetime.now(timezone.utc)
    ts = (
        f"{now.year:04d}{now.month:02d}{now.day:02d}"
        f"T{now.hour:02d}{now.minute:02d}{now.second:02d}Z"
    )
    out_dir = REPO_ROOT / ".local" / "agents" / "scan-public-content" / ts
    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "findings.json").write_text(json.dumps(all_findings, indent=2))
    (out_dir / "by_class.json").write_text(json.dumps(dict(by_class), indent=2))

    # terminal report — counts per class
    total_feeds = len(all_findings)
    feeds_with_findings = sum(1 for r in all_findings if r["findings"])
    print(f"Scanned: {total_feeds} feeds with output.json")
    print(f"Feeds with at least one finding: {feeds_with_findings}")
    print()
    print(f"{'class':<22} {'matches':>8} {'feeds':>7}")
    print(f"{'-' * 22} {'-' * 8} {'-' * 7}")
    for cls in PATTERNS.keys():
        items = by_class[cls]
        feeds_set = {i["feed"] for i in items}
        print(f"{cls:<22} {len(items):>8} {len(feeds_set):>7}")
    print()
    print(f"Detail: {out_dir.relative_to(REPO_ROOT)}/")
    return 0


if __name__ == "__main__":
    sys.exit(main())
