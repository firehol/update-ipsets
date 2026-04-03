# Installation

You will learn how to build, install, and verify update-ipsets on a Linux server.

## Prerequisites

- Linux (amd64 or arm64)
- Go 1.22 or later
- pnpm (for building the web UI)
## Build from source

Clone the repository and build:

```bash
git clone https://github.com/firehol/firehol.git
cd firehol/update-ipsets
make build
```

This produces a single `update-ipsets` binary in the project root. The binary has the web UI embedded — no separate static files to deploy.

## Install

Run the installer:

```bash
./install.sh
```

The installer does these things in order:

1. Installs UI dependencies and builds the React app (`pnpm install` + `pnpm build` in `ui/`)
2. Copies the fresh UI bundle into the embedded static directory
3. Builds the Go binary with the UI baked in
4. Creates the directory tree under `/opt/update-ipsets/`
5. Installs the binary to `/opt/update-ipsets/bin/update-ipsets`
6. Deploys the feed catalog from `configs/firehol/` to `/opt/update-ipsets/etc/config/`
7. Installs the systemd unit at `/etc/systemd/system/update-ipsets.service`
8. Reloads systemd and restarts the service (if already running)

### Custom install directory

Pass a different path as an argument:

```bash
./install.sh /opt/custom-path
```

### Skip restart

Add `--no-restart` to install without restarting the running service:

```bash
./install.sh --no-restart
```

The new binary takes effect on the next manual restart.

## Configuration handling on reinstall

The installer compares the repository catalog (`configs/firehol/`) against the active config directory (`/opt/update-ipsets/etc/config/`). When the content changed, it:

- Creates a timestamped backup: `/opt/update-ipsets/etc/config.bak.YYYYMMDDHHMMSS`
- Deploys the fresh catalog from the repository

When the content is identical, the installer leaves the active config untouched. This avoids triggering unnecessary reprocessing.

Any local changes to the config directory survive reinstalls in the backup copy. Merge your customizations back after upgrading.

## Verify the installation

Check the binary version:

```bash
/opt/update-ipsets/bin/update-ipsets version
```

Check the service status:

```bash
systemctl status update-ipsets
```

Test the health endpoint:

```bash
curl http://localhost:18888/healthz
```

A working installation returns `ok`.

## Next steps

- [Systemd setup](systemd-setup.md) — customize the service with drop-in overrides
- [TLS configuration](tls-configuration.md) — enable HTTPS
- [Filesystem layout](filesystem-layout.md) — understand the installed directory tree
