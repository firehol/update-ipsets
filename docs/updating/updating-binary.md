# Updating the Binary

You will learn how to build, install, and restart update-ipsets with minimal downtime.

## Build a new version

```bash
make build
```

This produces the `update-ipsets` binary in the project root.

## Install

```bash
./install.sh
```

The install script:

- Installs UI dependencies, rebuilds the UI bundle, and builds the Go binary
- Copies it to the installation directory (default `/opt/update-ipsets/bin/`)
- Deploys the configuration catalog
- Copies Markdown templates into `/opt/update-ipsets/etc/config/templates/markdown/`
- Installs or updates the systemd unit
- Creates a timestamped backup of the previous configuration if it changed

When the service is already running, the installer keeps the old daemon serving
while it performs those steps. The new binary and generated unit take effect at
the final restart.

The configuration backup covers the YAML catalog update. Markdown templates are
copied separately: identical templates are left alone, but differing repository
template files are overwritten in place under the installed template directory.
Keep customized templates or patches outside the install tree before updating.

## Restart

`./install.sh` restarts the service at the end when it is already active. If the
service is enabled but inactive, the installer starts it. If the service is not
enabled, or if you used `--no-restart`, restart manually when you are ready:

```bash
sudo systemctl restart update-ipsets
```

The daemon loads configuration, runs the startup feed-output integrity check,
queues any recovery work it can derive, and starts serving. It does not wait for
full catalog processing. Country and ASN entity-artifact repair continues in
background work after startup.

## Zero-downtime considerations

Normal installs do the build, copy, and systemd unit update while the old daemon
is still serving. The unavoidable downtime for a binary update is the final
process restart. If you have a reverse proxy in front of update-ipsets, the
proxy's health check against `/healthz` detects the brief unavailability and
retries.

For true zero-downtime, run two instances behind a load balancer and restart them one at a time.

## Verifying the update

Check that the new version is running:

```bash
update-ipsets version
```

The public status endpoint does not expose the build version. To check that the
daemon is running after restart:

```bash
curl -s http://localhost:18888/api/v1/status | jq '{running: .engine.running, sources: .engine.source_count, uptime: .system.uptime}'
```

## Configuration backup

`install.sh` creates a timestamped backup of the previous configuration directory when it detects changes. Backups are stored alongside the active configuration:

```
/opt/update-ipsets/etc/config.bak.20250501120000/
```

To roll back the configuration:

```bash
sudo rm -rf /opt/update-ipsets/etc/config
sudo mv /opt/update-ipsets/etc/config.bak.20250501120000 /opt/update-ipsets/etc/config
sudo systemctl restart update-ipsets
```

## Manual config edits

When the installed config directory differs from the repository catalog, `install.sh` backs up the whole active config directory and replaces it with `configs/firehol/`.

To preserve local edits across updates, keep a patch or a copy of your modified files outside the config directory, then reapply it after the reinstall.

The same rule applies to Markdown templates under
`/opt/update-ipsets/etc/config/templates/markdown/`, but without an automatic
template-specific backup.
