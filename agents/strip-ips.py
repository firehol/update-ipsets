#!/usr/bin/env python3
"""Defense-in-depth IP scrubber for feed-enrichment agent output.

Replaces every IPv4 and IPv6 address found inside summary/snippet string
fields with the literal marker `[IP-REDACTED]`. Walks the JSON recursively.

Other text fields (descriptions, rationales, etc.) are NOT touched — those
fields legitimately discuss IPs in technical contexts (CIDR blocks, ASN
prefixes, return-code conventions). The schema's `summary` and `snippet`
fields are the ones that capture community-reported allegations against
specific operators' IPs; those are the ones we redact.

Usage:
    agents/strip-ips.py INPUT.json OUTPUT.json
    agents/strip-ips.py FILE.json --in-place
"""

from __future__ import annotations

import argparse
import ipaddress
import json
import re
import sys
from pathlib import Path

IP_CANDIDATE_RE = re.compile(r"\b[0-9A-Fa-f:.]{2,}(?:/\d{1,3})?\b")
REDACTED = "[IP-REDACTED]"

# Fields we scrub. These are the only places where community-quoted IPs leak.
SENSITIVE_FIELDS = {"summary", "snippet"}


def scrub_text(s: str) -> tuple[str, int]:
    """Return (scrubbed_text, replacement_count)."""
    n = 0

    def repl(match: re.Match[str]) -> str:
        nonlocal n
        token = match.group(0)
        try:
            ipaddress.ip_network(token, strict=False)
        except ValueError:
            return token
        n += 1
        return REDACTED

    return IP_CANDIDATE_RE.sub(repl, s), n


def scrub(obj, total: list[int]) -> None:
    """Walk in place, scrubbing summary/snippet strings."""
    if isinstance(obj, dict):
        for k, v in obj.items():
            if k in SENSITIVE_FIELDS and isinstance(v, str):
                new, n = scrub_text(v)
                if n:
                    total[0] += n
                    obj[k] = new
            else:
                scrub(v, total)
    elif isinstance(obj, list):
        for item in obj:
            scrub(item, total)


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("input", type=Path)
    p.add_argument("output", type=Path, nargs="?")
    p.add_argument("--in-place", action="store_true")
    args = p.parse_args()

    if args.in_place:
        out_path = args.input
    elif args.output:
        out_path = args.output
    else:
        print("ERROR: either provide OUTPUT or use --in-place", file=sys.stderr)
        return 2

    data = json.loads(args.input.read_text())
    counter = [0]
    scrub(data, counter)
    out_path.write_text(json.dumps(data, indent=2))
    print(f"scrubbed {counter[0]} IP address(es) in {out_path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
