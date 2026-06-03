#!/usr/bin/env python3
"""
Locate a feed by name across the catalog YAML files.

The agent wrapper (run-enrichment.sh) and pool (run-enrichment-pool.sh)
historically iterated YAML files by filename and treated the stem as the
feed name. That works for single-source YAMLs (one feed per file) but
misses sub-feeds inside multi-source YAMLs like:

  configs/firehol/sources/provider_infrastructure/critical_provider_ranges.yaml
    sources:
      critical_soft_cloudflare_edge: {...}     # real feed
      critical_soft_microsoft365:    {...}     # real feed
      ...

This helper parses YAMLs and reports the real (feed_name → yaml_path,
url, maintainer, category) mapping.

Usage:
  agents/locate-feed.py <feed_name>           prints "yaml_path<TAB>url<TAB>maintainer<TAB>category"
                                              exit 0 found / 1 not found

  agents/locate-feed.py --all                 prints every (feed_name, yaml_path) from configs/firehol/sources/
                                              one per line: "feed_name<TAB>yaml_path"

  agents/locate-feed.py --all-with-meta       same as --all but with url/maintainer/category fields
                                              "feed_name<TAB>yaml_path<TAB>url<TAB>maintainer<TAB>category"
"""

from __future__ import annotations

import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML is required (pip install pyyaml)", file=sys.stderr)
    sys.exit(2)

REPO = Path(__file__).resolve().parent.parent
SOURCES_DIR = REPO / "configs" / "firehol" / "sources"


def _row(name: str, src: dict, yaml_path: Path) -> tuple[str, str, str, str, str]:
    """
    Return (feed_name, yaml_path, url, maintainer, category) — '-' for missing.

    Empty fields are emitted as a literal '-' instead of an empty string so
    shell `read -r ... ` doesn't collapse adjacent tabs and shift columns
    when a feed has no `url:` or other optional fields. Callers must treat
    '-' as the null marker.
    """
    return (
        name,
        str(yaml_path),
        str(src.get("url") or "-"),
        str(src.get("maintainer") or "-"),
        str(src.get("category") or "-"),
    )


def iter_feeds():
    """Yield (feed_name, src_dict, yaml_path) for every feed declared under sources:."""
    for y in sorted(SOURCES_DIR.rglob("*.yaml")):
        try:
            data = yaml.safe_load(y.read_text())
        except (OSError, yaml.YAMLError):
            continue
        if not isinstance(data, dict):
            continue
        srcs = data.get("sources")
        if not isinstance(srcs, dict):
            continue
        for name, src in srcs.items():
            if isinstance(src, dict):
                yield name, src, y


def find_one(feed_name: str):
    for name, src, y in iter_feeds():
        if name == feed_name:
            return _row(name, src, y)
    return None


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: locate-feed.py <feed_name> | --all | --all-with-meta", file=sys.stderr)
        return 2

    arg = sys.argv[1]

    if arg == "--all":
        for name, _src, y in iter_feeds():
            print(f"{name}\t{y}")
        return 0

    if arg == "--all-with-meta":
        for name, src, y in iter_feeds():
            row = _row(name, src, y)
            print("\t".join(row))
        return 0

    # Single-feed lookup
    row = find_one(arg)
    if row is None:
        return 1
    # Skip the leading name column — caller already knows the feed name
    print("\t".join(row[1:]))
    return 0


if __name__ == "__main__":
    sys.exit(main())
