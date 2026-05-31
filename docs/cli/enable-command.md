# enable Command

You will learn how to enable and disable feed sources from the command line.

## Basic usage

Enable a specific feed:

```bash
update-ipsets enable firehol_level1
```

This creates the source enable marker for that feed. The daemon picks up the change on the next scheduler cycle or after a SIGHUP.

## Disable a feed

Add `--disable` to remove the enable marker:

```bash
update-ipsets enable firehol_level1 --disable
```

The feed stops being scheduled for download and processing. Existing committed outputs remain on disk.

## Enable or disable all feeds

Enable every known feed in the catalog:

```bash
update-ipsets enable --all
```

Disable every known feed:

```bash
update-ipsets enable --all --disable
```

Use this when you want a clean slate — either enable everything and selectively disable, or disable everything and selectively enable.

## Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Path to the configuration catalog |
| `--all` | Apply to all known feeds |
| `--disable` | Disable instead of enable |
| `--silent` | Suppress output |
| `--verbose` | Show what changed |

## Specifying the config path

Override the default config location:

```bash
update-ipsets enable --config /opt/update-ipsets/etc/config firehol_level1
```

## Combining with the daemon

The `enable` command modifies enable markers on disk. The running daemon detects the change when:

- The next scheduler evaluation runs (controlled by the daemon `--interval` duration)
- You send SIGHUP to reload configuration
- You restart the daemon

For immediate effect without restart:

```bash
update-ipsets enable firehol_level1
sudo systemctl kill -s HUP update-ipsets
```

Or use the admin API to enable feeds at runtime:

```bash
curl -X POST -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/feeds/firehol_level1/enable
```

## Checking current state

Use `--verbose` to see which feeds changed state:

```bash
update-ipsets enable --all --verbose
```

Output shows each feed and whether its enable marker was created, removed, or already in the requested state.
