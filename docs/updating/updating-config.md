# Updating Configuration

You will learn how to update the feed configuration catalog and how local edits interact with upgrades.

## Where configuration lives

The source catalog is in the repository at `configs/firehol/`. Each feed has its own YAML file:

- Source feeds: `configs/firehol/sources/<category>/<feed>.yaml`
- Merge feeds: `configs/firehol/merges/<name>.yaml`
- Artifact parents: `configs/firehol/artifacts/<name>.yaml`
- Shared registries: `configs/firehol/runtime.yaml`, `categories.yaml`, etc.

The installed active catalog is at `/opt/update-ipsets/etc/config/`.

## Updating from the repository

Pull the latest changes and re-run install:

```bash
cd ~/src/firehol/update-ipsets
git pull
./install.sh
```

`install.sh` compares the repository catalog with the installed catalog. If files changed:

- Updated files are deployed to `/opt/update-ipsets/etc/config/`
- The previous config directory is preserved as a timestamped backup
- The daemon picks up changes on restart

## Applying the update

```bash
sudo systemctl restart update-ipsets
```

Or reload without restart (if only feed definitions changed, not runtime settings):

```bash
sudo systemctl kill -s HUP update-ipsets
```

## Automatic backups

Every time `install.sh` detects a configuration change, it creates a backup:

```
/opt/update-ipsets/etc/config.bak.20250501120000/
```

Multiple backups accumulate over time. Old backups are safe to delete manually.

## Manual edits

You can edit files directly in `/opt/update-ipsets/etc/config/`:

```bash
sudo vim /opt/update-ipsets/etc/config/sources/intrusion/myfeed.yaml
```

After editing, reload or restart:

```bash
sudo systemctl kill -s HUP update-ipsets
```

## How manual edits interact with upgrades

When you run `install.sh` again:

- If the installed config directory is identical to `configs/firehol/`, it is left untouched
- If the installed config directory differs from `configs/firehol/`, the whole active config directory is backed up and replaced with the repository catalog
- The backup directory preserves your previous version

To protect critical local edits, keep a copy outside the config directory or use a local patch file, then reapply it after the reinstall.

## Migration from bash

If you are migrating from the legacy bash implementation, see [Migration from bash](../migration-from-bash.md) for the full migration procedure.
