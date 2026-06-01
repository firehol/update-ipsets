# cache-merge Command

You will learn what the `cache-merge` migration helper does, when operators should use it, and which inputs it expects.

## Purpose

`cache-merge` imports a legacy bash-era `.cache` file into the Go JSON cache format and preserves selected local-only Go cache entries.

Most operators should not run this command directly. Use the canonical migration helper instead:

```bash
./scripts/sync-from-bash-version.sh <source-host|localhost> [INSTALL_DIR]
```

That helper stages files safely, builds the local-only feed manifest, runs `cache-merge`, and promotes the merged cache into the installed data directory.

## Direct usage

Use direct invocation only when you are repairing or testing a migration manually:

```bash
update-ipsets cache-merge \
  --legacy /path/to/imported/.cache \
  --local-json /opt/update-ipsets/data/.cache.json \
  --local-only /path/to/local-only-feeds.txt \
  --out /path/to/merged-cache.json
```

## Flags

| Flag | Required | Description |
|---|---|---|
| `--legacy <path>` | Yes | Legacy bash `.cache` file to import. |
| `--local-json <path>` | No | Existing Go JSON cache. When omitted, only the legacy cache contributes entries. |
| `--local-only <path>` | Yes | Newline-delimited feed names whose existing Go cache entries should be preserved. Blank lines and `#` comments are ignored. |
| `--out <path>` | Yes | Output path for the merged Go JSON cache. |

## Output

On success, the command writes the merged JSON cache and prints a short summary:

```text
merged 123 total entries; preserved 4 local-only entries
```

Exit code `0` means the merged cache was written. Exit code `1` means an input or output file failed to load or save. Exit code `2` means required flags were missing or invalid.

## Safety notes

- Write to a staging path first, not directly to `/opt/update-ipsets/data/.cache.json`.
- Stop the daemon or run the full migration helper before replacing the live cache.
- Keep a backup of the live installation before promoting migrated state.
- Prefer `scripts/sync-from-bash-version.sh` unless you have a specific manual migration reason.
