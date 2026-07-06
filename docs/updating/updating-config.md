# Updating Configuration

You will learn how to update the feed configuration catalog and how local edits interact with upgrades.

## Where configuration lives

The source catalog is in the repository at `configs/firehol/`. Each feed has its own YAML file:

- Source feeds: `configs/firehol/sources/<category>/<feed>.yaml`
- Merge feeds: `configs/firehol/merges/<name>.yaml`
- Artifact parents: `configs/firehol/artifacts/<name>.yaml`
- Shared registries: `configs/firehol/runtime.yaml`, `categories.yaml`, etc.

The installed active catalog is at `/opt/update-ipsets/etc/config/`.
Installed Markdown templates live under
`/opt/update-ipsets/etc/config/templates/markdown/`.

## Updating from the repository

Pull the latest changes and re-run install:

```bash
cd /path/to/update-ipsets
git pull
./install.sh
```

`install.sh` compares the repository catalog with the installed catalog. If files changed:

- Updated files are deployed to `/opt/update-ipsets/etc/config/`
- The previous config directory is preserved as a timestamped backup
- An active service keeps running while files are deployed, then is restarted at
  the end unless you pass `--no-restart`

Markdown templates are updated separately from the YAML catalog:

- Repository templates are copied from `configs/templates/markdown/` into `/opt/update-ipsets/etc/config/templates/markdown/`
- Identical installed templates are left untouched
- Differing repository template files are overwritten in place
- Extra local files under the installed template directory are not removed
- Template changes need a service restart; SIGHUP does not reload templates

## Applying the update

If `install.sh` restarted the active service, no extra action is needed. If you
used `--no-restart`, or if you edited the installed catalog manually, restart:

```bash
sudo systemctl restart update-ipsets
```

Or reload without restart when only YAML feed definitions changed, not runtime
settings or Markdown templates:

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

Template edits under `templates/markdown/` require a service restart. A SIGHUP
reload re-reads the YAML catalog, but it does not reload Markdown templates.

## How manual edits interact with upgrades

When you run `install.sh` again:

- If the installed config directory is identical to `configs/firehol/`, it is left untouched
- If the installed config directory differs from `configs/firehol/`, the whole active config directory is backed up and replaced with the repository catalog
- The backup directory preserves your previous version

To protect critical local edits, keep a copy outside the config directory or use a local patch file, then reapply it after the reinstall.

For Markdown template edits, do not rely on the catalog backup as your only
copy. Keep customized templates or patches outside
`/opt/update-ipsets/etc/config/templates/markdown/`, then reapply them after
running the installer.

## Migration from bash

If you are migrating from the legacy bash implementation, see [Migration from bash](../migration-from-bash.md) for the full migration procedure.
