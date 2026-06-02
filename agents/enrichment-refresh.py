#!/usr/bin/env python3
"""Finalize a scoped enrichment refresh.

The pool runner is responsible for executing expensive research workers. This
script handles the deterministic post-run phase: project latest clean outputs to
the public embedded schema, write selected `enrichment:` blocks, generate a
review summary, and optionally create a local branch/commit plus PR.
"""

from __future__ import annotations

import argparse
import importlib.util
import re
import shutil
import subprocess  # nosec B404 - this script runs fixed git/gh commands.
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

REPO = Path(__file__).resolve().parent.parent
SCOPE_DIR = REPO / ".agents" / "sow" / "reports" / "enrichment-refresh"


def load_enrichment_public():
    path = REPO / "agents" / "enrichment-public.py"
    spec = importlib.util.spec_from_file_location("enrichment_public", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot import {path}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


EP = load_enrichment_public()


def command_path(name: str) -> str:
    path = shutil.which(name)
    if path is None:
        raise RuntimeError(f"{name} command not found in PATH")
    return path


def parse_feeds(raw: str) -> list[str]:
    feeds = sorted({part.strip() for part in raw.split(",") if part.strip()})
    if not feeds:
        raise ValueError("no feeds selected")
    return feeds


def embedded_projection(feed: str) -> dict[str, Any]:
    for name, projection, _source in EP.embedded_enrichments([feed]):
        if name == feed:
            return projection
    return {}


def latest_projection(feed: str) -> tuple[Any, dict[str, Any], list[dict[str, Any]]]:
    run = EP.latest_feed_run(feed)
    data = EP.load_json(run.output_path)
    projection = EP.project_public(data)
    EP.validate_public(projection)
    hygiene = EP.prose_hygiene_findings(
        run.feed,
        projection,
        str(run.output_path.relative_to(REPO)),
    )
    return run, projection, hygiene


def role_value(doc: dict[str, Any], key: str) -> str:
    roles = doc.get("roles") or []
    for role in roles:
        if isinstance(role, dict) and role.get("role") == "maintainer":
            return str(role.get(key) or "").strip()
    return ""


def nested_value(doc: dict[str, Any], path: tuple[str, ...]) -> str:
    value: Any = doc
    for part in path:
        if not isinstance(value, dict):
            return ""
        value = value.get(part)
    if value is None:
        return ""
    if isinstance(value, bool):
        return "true" if value else "false"
    return str(value).strip()


SIGNIFICANT_FIELDS = (
    ("maintainer", ("roles", "maintainer", "name")),
    ("maintainer_url", ("roles", "maintainer", "official_url")),
    ("status", ("current_status", "state")),
    ("license", ("license",)),
    ("redistributable", ("redistribution", "allowed")),
)


def significant_changes(feed: str, before: dict[str, Any], after: dict[str, Any]) -> list[dict[str, str]]:
    out: list[dict[str, str]] = []
    for label, path in SIGNIFICANT_FIELDS:
        if path[:2] == ("roles", "maintainer"):
            old = role_value(before, path[2])
            new = role_value(after, path[2])
        else:
            old = nested_value(before, path)
            new = nested_value(after, path)
        if old != new:
            out.append({"feed": feed, "field": label, "before": old or "-", "after": new or "-"})
    return out


def summary_path(scope: str, timestamp: str, override: Path | None) -> Path:
    if override is not None:
        return override
    return SCOPE_DIR / f"{slug(scope)}-{timestamp}.md"


def slug(value: str) -> str:
    text = re.sub(r"[^a-zA-Z0-9._-]+", "-", value.strip().lower()).strip("-")
    return text or "selected"


def branch_name(scope: str, timestamp: str) -> str:
    return f"enrichment/{slug(scope)}-{timestamp[:8]}"


def git(*args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(  # nosec B603 - args are fixed git subcommands assembled by this script.
        [command_path("git"), *args],
        cwd=REPO,
        check=check,
        stdin=subprocess.DEVNULL,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )


def require_clean_worktree() -> None:
    if git("diff", "--quiet", check=False).returncode != 0:
        raise RuntimeError("tracked worktree is dirty; commit or stash changes before branch finalization")
    if git("diff", "--cached", "--quiet", check=False).returncode != 0:
        raise RuntimeError("index is dirty; commit or unstage changes before branch finalization")


def has_remote_and_gh() -> bool:
    remotes = git("remote", check=False)
    if remotes.returncode != 0 or not remotes.stdout.strip():
        return False
    gh_path = shutil.which("gh")
    if gh_path is None:
        return False
    gh = subprocess.run(  # nosec B603 - fixed gh auth-status command.
        [gh_path, "auth", "status"],
        cwd=REPO,
        check=False,
        stdin=subprocess.DEVNULL,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    return gh.returncode == 0


def write_summary(
    path: Path,
    scope: str,
    feeds: list[str],
    items: list[dict[str, Any]],
    changes: list[dict[str, str]],
    dry_run: bool,
) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    lines = [
        f"# Enrichment refresh: {scope}",
        "",
        f"Generated: {datetime.now(timezone.utc).isoformat()}",
        f"Mode: {'dry-run' if dry_run else 'write'}",
        f"Feeds: {len(feeds)}",
        "",
        "## Selected feeds",
        "",
    ]
    lines.extend(f"- `{feed}`" for feed in feeds)
    lines.extend(["", "## Significant changes", ""])
    if changes:
        lines.extend([
            "| Feed | Field | Before | After |",
            "|:-----|:------|:-------|:------|",
        ])
        for item in changes:
            lines.append(
                f"| `{cell(item['feed'])}` | `{cell(item['field'])}` | {cell(item['before'])} | {cell(item['after'])} |"
            )
    else:
        lines.append("No significant maintainer, status, license, or redistribution changes detected.")
    lines.extend(["", "## Writeback details", ""])
    lines.extend([
        "| Feed | YAML | Section | Run | Hygiene findings |",
        "|:-----|:-----|:--------|:----|-----------------:|",
    ])
    for item in items:
        lines.append(
            f"| `{cell(item['feed'])}` | `{cell(item['yaml'])}` | `{cell(item['section'])}` | `{cell(item['run'])}` | {len(item['hygiene_findings'])} |"
        )
    path.write_text("\n".join(lines) + "\n")


def cell(value: str) -> str:
    return str(value).replace("|", r"\|").replace("\n", " ")


def run(args: argparse.Namespace) -> int:
    feeds = parse_feeds(args.feeds)
    scope = args.scope or ",".join(feeds)
    timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    branch = branch_name(scope, timestamp)
    if args.branch and not args.dry_run:
        require_clean_worktree()
        existing = git("show-ref", "--verify", "--quiet", f"refs/heads/{branch}", check=False)
        if existing.returncode == 0:
            branch = f"{branch}-{timestamp[9:15]}"
        git("switch", "-c", branch)

    items: list[dict[str, Any]] = []
    changes: list[dict[str, str]] = []
    yaml_paths: list[str] = []
    for feed in feeds:
        before = embedded_projection(feed)
        run_info, projection, hygiene = latest_projection(feed)
        item = EP.write_enrichment(feed, projection, write=args.write and not args.dry_run)
        item["run"] = str(run_info.run_dir.relative_to(REPO))
        item["hygiene_findings"] = hygiene
        items.append(item)
        yaml_paths.append(item["yaml"])
        changes.extend(significant_changes(feed, before, projection))
        status = "would_write" if args.dry_run or not args.write else "wrote"
        print(f"{status}\t{feed}\t{item['yaml']}\t{item['section']}\thygiene={len(hygiene)}")

    summary = summary_path(scope, timestamp, args.summary)
    write_summary(summary, scope, feeds, items, changes, dry_run=args.dry_run or not args.write)
    print(f"summary\t{summary.relative_to(REPO) if summary.is_relative_to(REPO) else summary}")

    if args.commit and not args.dry_run:
        if not args.write:
            raise RuntimeError("--commit requires --write")
        add_paths = sorted(set(yaml_paths + [str(summary.relative_to(REPO)) if summary.is_relative_to(REPO) else str(summary)]))
        git("add", "--", *add_paths)
        git("commit", "-m", f"Refresh enrichment metadata for {scope}")
        print(f"branch\t{branch}")
        if args.open_pr and has_remote_and_gh():
            title = f"Enrichment refresh: {scope} ({len(feeds)} feeds)"
            gh = subprocess.run(  # nosec B603 - fixed gh PR command with controlled arguments.
                [command_path("gh"), "pr", "create", "--title", title, "--body-file", str(summary)],
                cwd=REPO,
                check=False,
                stdin=subprocess.DEVNULL,
                text=True,
            )
            if gh.returncode != 0:
                raise RuntimeError("gh pr create failed")
        elif args.open_pr:
            print("pr\tskipped (no git remote or gh authentication)")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--feeds", required=True, help="comma-separated feed names")
    parser.add_argument("--scope", help="human-readable scope label for branch/summary")
    parser.add_argument("--summary", type=Path, help="summary path override")
    parser.add_argument("--write", action="store_true", help="write YAML enrichment blocks")
    parser.add_argument("--branch", action="store_true", help="create a local branch before writing")
    parser.add_argument("--commit", action="store_true", help="commit YAML writeback and summary")
    parser.add_argument("--open-pr", action="store_true", help="open a PR when remote and gh auth are available")
    parser.add_argument("--dry-run", action="store_true", help="project and summarize without writing, branching, or committing")
    args = parser.parse_args()
    return run(args)


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (RuntimeError, ValueError) as err:
        print(f"error: {err}", file=sys.stderr)
        sys.exit(1)
