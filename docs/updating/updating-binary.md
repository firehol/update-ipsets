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
- Installs or updates the systemd unit
- Creates a timestamped backup of the previous configuration if it changed

## Restart

```bash
sudo systemctl restart update-ipsets
```

The restart is fast. The daemon loads configuration, checks for recoverable staged state, and starts serving. Startup does not wait for full catalog processing or integrity scans.

## Zero-downtime considerations

The daemon restarts quickly — typically under a second. If you have a reverse proxy in front of update-ipsets, the proxy's health check against `/healthz` detects the brief unavailability and retries.

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
